"""
Utility functions for FMCSA data processing.
Includes error handling, validation, and helper functions.
"""

import logging
import pandas as pd
from pathlib import Path
from typing import Dict, List, Any, Optional, Tuple
from datetime import datetime, timedelta
import re

# Handle both relative and absolute imports
try:
    from .consts import VALIDATION_RULES
except ImportError:
    from consts import VALIDATION_RULES

logger = logging.getLogger(__name__)


class FMCSADataError(Exception):
    """Base exception for FMCSA data processing errors."""
    pass


class ValidationError(FMCSADataError):
    """Raised when data validation fails."""
    pass


class DownloadError(FMCSADataError):
    """Raised when data download fails."""
    pass


class ParseError(FMCSADataError):
    """Raised when data parsing fails."""
    pass


def validate_dot_number(dot_number: str) -> bool:
    """
    Validate FMCSA DOT number format.

    Args:
        dot_number: DOT number as string

    Returns:
        True if valid, False otherwise
    """
    if not dot_number or not isinstance(dot_number, str):
        return False

    # DOT numbers are 1-8 digits
    pattern = VALIDATION_RULES["dot_number"]["pattern"]
    return bool(re.match(pattern, dot_number.strip()))


def validate_carrier_data(df: pd.DataFrame, required_fields: Optional[List[str]] = None) -> Dict[str, Any]:
    """
    Comprehensive validation of carrier data.

    Args:
        df: DataFrame to validate
        required_fields: List of fields that must be present

    Returns:
        Validation results dictionary

    Raises:
        ValidationError: If critical validation fails
    """
    results = {
        "valid": True,
        "errors": [],
        "warnings": [],
        "stats": {
            "total_rows": len(df),
            "total_columns": len(df.columns)
        }
    }

    # Check required fields
    if required_fields:
        missing_fields = [field for field in required_fields if field not in df.columns]
        if missing_fields:
            results["errors"].append(f"Missing required fields: {missing_fields}")
            results["valid"] = False

    # Validate DOT numbers if present
    if "dot_number" in df.columns:
        invalid_dots = df["dot_number"].astype(str).apply(lambda x: not validate_dot_number(x))
        invalid_count = invalid_dots.sum()

        if invalid_count > 0:
            results["warnings"].append(f"{invalid_count} invalid DOT numbers found")

        # Remove rows with invalid DOT numbers for critical validation
        if invalid_count > len(df) * 0.1:  # More than 10% invalid
            results["errors"].append(f"Too many invalid DOT numbers: {invalid_count}/{len(df)}")
            results["valid"] = False

    # Check for duplicate DOT numbers
    if "dot_number" in df.columns:
        duplicates = df["dot_number"].duplicated().sum()
        if duplicates > 0:
            results["warnings"].append(f"{duplicates} duplicate DOT numbers found")

    # Check data completeness
    null_percentages = (df.isnull().sum() / len(df) * 100).round(2)
    high_null_fields = null_percentages[null_percentages > 50]

    if not high_null_fields.empty:
        results["warnings"].append(f"Fields with >50% null values: {high_null_fields.to_dict()}")

    # Validate data types
    type_issues = _validate_data_types(df)
    if type_issues:
        results["warnings"].extend(type_issues)

    if results["errors"]:
        raise ValidationError(f"Data validation failed: {results['errors']}")

    return results


def _validate_data_types(df: pd.DataFrame) -> List[str]:
    """Validate data types for common fields."""
    issues = []

    # Check numeric fields
    numeric_fields = ["fleet_size", "score", "violation_count"]
    for field in numeric_fields:
        if field in df.columns:
            try:
                pd.to_numeric(df[field], errors='coerce')
            except Exception as e:
                issues.append(f"Type validation failed for {field}: {e}")

    # Check date fields
    date_fields = ["inspection_date", "created_date", "updated_date"]
    for field in date_fields:
        if field in df.columns:
            try:
                pd.to_datetime(df[field], errors='coerce')
            except Exception as e:
                issues.append(f"Date validation failed for {field}: {e}")

    return issues


def clean_column_names(df: pd.DataFrame) -> pd.DataFrame:
    """
    Clean and standardize column names.

    Args:
        df: DataFrame with potentially messy column names

    Returns:
        DataFrame with cleaned column names
    """
    df = df.copy()

    # Convert to lowercase and replace common separators
    df.columns = (
        df.columns
        .str.lower()
        .str.replace(r'[\s\-\_\.]+', '_', regex=True)  # Replace separators with underscore
        .str.replace(r'[^\w]', '', regex=True)  # Remove special characters
        .str.strip('_')  # Remove leading/trailing underscores
    )

    # Handle duplicate column names
    seen = {}
    new_columns = []

    for col in df.columns:
        if col in seen:
            seen[col] += 1
            new_columns.append(f"{col}_{seen[col]}")
        else:
            seen[col] = 0
            new_columns.append(col)

    df.columns = new_columns
    return df


def handle_large_file(file_path: Path, chunk_size: int = 100000) -> pd.DataFrame:
    """
    Handle large files by processing in chunks.

    Args:
        file_path: Path to large file
        chunk_size: Number of rows per chunk

    Returns:
        Processed DataFrame
    """
    logger.info(f"Processing large file in chunks: {file_path}")

    if file_path.suffix.lower() == '.csv':
        # Process CSV in chunks
        chunks = []
        for chunk in pd.read_csv(file_path, chunksize=chunk_size):
            # Apply basic cleaning to each chunk
            chunk = clean_column_names(chunk)
            chunks.append(chunk)

        # Combine chunks
        df = pd.concat(chunks, ignore_index=True)

    else:
        # For other formats, load normally (assuming they fit in memory)
        df = pd.read_csv(file_path) if file_path.suffix.lower() == '.csv' else pd.DataFrame()
        df = clean_column_names(df)

    logger.info(f"Processed {len(df)} rows from large file")
    return df


def create_backup(file_path: Path) -> Path:
    """
    Create a backup of a file before processing.

    Args:
        file_path: Original file path

    Returns:
        Backup file path
    """
    timestamp = datetime.now().strftime("%Y%m%d_%H%M%S")
    backup_path = file_path.parent / f"{file_path.stem}_backup_{timestamp}{file_path.suffix}"

    try:
        import shutil
        shutil.copy2(file_path, backup_path)
        logger.info(f"Backup created: {backup_path}")
        return backup_path
    except Exception as e:
        logger.warning(f"Failed to create backup: {e}")
        return file_path  # Return original if backup fails


def get_file_info(file_path: Path) -> Dict[str, Any]:
    """
    Get detailed information about a data file.

    Args:
        file_path: Path to the file

    Returns:
        Dictionary with file information
    """
    info = {
        "path": str(file_path),
        "size_mb": round(file_path.stat().st_size / (1024 * 1024), 2),
        "modified": datetime.fromtimestamp(file_path.stat().st_mtime),
        "exists": file_path.exists()
    }

    if file_path.exists() and file_path.suffix.lower() == '.csv':
        try:
            # Get basic CSV info without loading full file
            with open(file_path, 'r', encoding='utf-8') as f:
                sample = f.read(1024)
                lines = sample.split('\n')
                info["estimated_rows"] = max(1, len(lines) - 1)  # Rough estimate
                if lines:
                    info["columns"] = len(lines[0].split(','))
        except Exception as e:
            logger.warning(f"Could not read file info: {e}")

    return info


def log_processing_stats(operation: str, start_time: datetime, records_processed: int,
                        errors: int = 0, warnings: int = 0) -> None:
    """
    Log processing statistics.

    Args:
        operation: Name of the operation
        start_time: When processing started
        records_processed: Number of records processed
        errors: Number of errors encountered
        warnings: Number of warnings generated
    """
    duration = datetime.now() - start_time
    records_per_second = records_processed / duration.total_seconds() if duration.total_seconds() > 0 else 0

    logger.info(f"=== {operation} Statistics ===")
    logger.info(f"Duration: {duration}")
    logger.info(f"Records processed: {records_processed:,}")
    logger.info(f"Processing rate: {records_per_second:.1f} records/second")
    if errors > 0:
        logger.warning(f"Errors: {errors}")
    if warnings > 0:
        logger.warning(f"Warnings: {warnings}")


def safe_merge_dataframes(left: pd.DataFrame, right: pd.DataFrame,
                         on: str, how: str = 'left') -> pd.DataFrame:
    """
    Safely merge DataFrames with error handling.

    Args:
        left: Left DataFrame
        right: Right DataFrame
        on: Column to merge on
        how: Type of merge

    Returns:
        Merged DataFrame
    """
    try:
        # Check if merge column exists in both DataFrames
        if on not in left.columns:
            raise ValidationError(f"Merge column '{on}' not found in left DataFrame")
        if on not in right.columns:
            raise ValidationError(f"Merge column '{on}' not found in right DataFrame")

        # Perform merge
        merged = left.merge(right, on=on, how=how, suffixes=('_left', '_right'))

        logger.info(f"Merged {len(left)} + {len(right)} rows -> {len(merged)} rows")
        return merged

    except Exception as e:
        logger.error(f"Merge failed: {e}")
        raise


def export_to_multiple_formats(df: pd.DataFrame, base_path: Path,
                              formats: List[str] = None) -> Dict[str, Path]:
    """
    Export DataFrame to multiple formats.

    Args:
        df: DataFrame to export
        base_path: Base path for output files
        formats: List of formats to export to

    Returns:
        Dictionary mapping format to file path
    """
    if formats is None:
        formats = ["csv"]

    exported_files = {}

    for fmt in formats:
        try:
            output_path = base_path.parent / f"{base_path.stem}.{fmt}"

            if fmt == "csv":
                df.to_csv(output_path, index=False)
            elif fmt == "json":
                df.to_json(output_path, orient="records", date_format="iso")
            elif fmt == "parquet":
                df.to_parquet(output_path, index=False)
            elif fmt == "xlsx":
                df.to_excel(output_path, index=False)
            else:
                logger.warning(f"Unsupported export format: {fmt}")
                continue

            exported_files[fmt] = output_path
            logger.info(f"Exported to {fmt}: {output_path}")

        except Exception as e:
            logger.error(f"Failed to export to {fmt}: {e}")

    return exported_files
