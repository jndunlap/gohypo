"""
FMCSA Data File Parsers
Handles parsing of different FMCSA data file formats (CSV, fixed-width TXT, etc.)
"""

import pandas as pd
import logging
from pathlib import Path
from typing import Dict, List, Optional, Iterator, Any, Tuple
import csv
import re
from datetime import datetime

# Handle both relative and absolute imports
try:
    from .consts import FILE_FORMATS, VALIDATION_RULES, FSMA_CONFIG
except ImportError:
    from consts import FILE_FORMATS, VALIDATION_RULES, FSMA_CONFIG

logger = logging.getLogger(__name__)


class FMCSADataParser:
    """Base class for parsing FMCSA data files."""

    def __init__(self, file_path: Path):
        self.file_path = file_path
        self.encoding = FILE_FORMATS["csv"]["encoding"]  # Default encoding

    def parse_to_dataframe(self) -> pd.DataFrame:
        """Parse file to pandas DataFrame."""
        raise NotImplementedError

    def validate_data(self, df: pd.DataFrame) -> Dict[str, Any]:
        """Validate parsed data against rules."""
        validation_results = {
            "total_rows": len(df),
            "errors": [],
            "warnings": []
        }

        # Check required fields
        for field, rules in VALIDATION_RULES.items():
            if rules.get("required", False) and field not in df.columns:
                validation_results["errors"].append(f"Required field missing: {field}")
            elif field in df.columns:
                # Validate field values
                validation_result = self._validate_field(df[field], rules)
                validation_results["errors"].extend(validation_result["errors"])
                validation_results["warnings"].extend(validation_result["warnings"])

        return validation_results

    def _validate_field(self, series: pd.Series, rules: Dict) -> Dict[str, List[str]]:
        """Validate a single field against rules."""
        errors = []
        warnings = []

        # Pattern validation
        if "pattern" in rules:
            pattern = re.compile(rules["pattern"])
            invalid_count = (~series.astype(str).str.match(pattern, na=False)).sum()
            if invalid_count > 0:
                errors.append(f"{invalid_count} values don't match pattern for {series.name}")

        # Length validation
        if "max_length" in rules:
            too_long = (series.astype(str).str.len() > rules["max_length"]).sum()
            if too_long > 0:
                errors.append(f"{too_long} values exceed max length for {series.name}")

        # Null check for required fields
        if rules.get("required", False):
            null_count = series.isnull().sum()
            if null_count > 0:
                errors.append(f"{null_count} null values in required field {series.name}")

        return {"errors": errors, "warnings": warnings}


class FMCSACSVParser(FMCSADataParser):
    """Parser for FMCSA CSV files."""

    def __init__(self, file_path: Path, delimiter: str = ","):
        super().__init__(file_path)
        self.delimiter = delimiter

    def parse_to_dataframe(self) -> pd.DataFrame:
        """Parse CSV file to DataFrame with automatic type detection."""
        try:
            logger.info(f"Parsing CSV file: {self.file_path}")

            # First, try to detect the delimiter and encoding
            with open(self.file_path, 'r', encoding=self.encoding) as f:
                sample = f.read(1024)
                detected_delimiter = csv.Sniffer().sniff(sample).delimiter

            # Use detected delimiter, but fall back to specified one
            delimiter = detected_delimiter if detected_delimiter else self.delimiter

            # Read CSV with pandas
            df = pd.read_csv(
                self.file_path,
                delimiter=delimiter,
                encoding=self.encoding,
                low_memory=False,  # Handle large files
                dtype=str  # Read as strings first to avoid type issues
            )

            # Clean column names
            df.columns = df.columns.str.strip().str.lower().str.replace(' ', '_')

            logger.info(f"Parsed {len(df)} rows with {len(df.columns)} columns")
            return df

        except Exception as e:
            logger.error(f"Failed to parse CSV {self.file_path}: {e}")
            raise


class FMCSAFixedWidthParser(FMCSADataParser):
    """Parser for FMCSA fixed-width text files."""

    def __init__(self, file_path: Path, field_specs: List[Tuple[str, int, int]]):
        """
        Initialize fixed-width parser.

        Args:
            file_path: Path to the fixed-width file
            field_specs: List of tuples (field_name, start_pos, length)
        """
        super().__init__(file_path)
        self.field_specs = field_specs

    def parse_to_dataframe(self, chunk_size: int = 10000) -> pd.DataFrame:
        """
        Parse fixed-width file to DataFrame.

        Args:
            chunk_size: Number of rows to process at once
        """
        try:
            logger.info(f"Parsing fixed-width file: {self.file_path}")

            # Read file in chunks to handle large files
            chunks = []
            with open(self.file_path, 'r', encoding=self.encoding) as f:
                chunk_lines = []
                for line_num, line in enumerate(f, 1):
                    chunk_lines.append(line.rstrip('\n\r'))

                    if len(chunk_lines) >= chunk_size:
                        chunk_df = self._parse_chunk(chunk_lines)
                        chunks.append(chunk_df)
                        chunk_lines = []

                # Process remaining lines
                if chunk_lines:
                    chunk_df = self._parse_chunk(chunk_lines)
                    chunks.append(chunk_df)

            # Combine all chunks
            if chunks:
                df = pd.concat(chunks, ignore_index=True)
                logger.info(f"Parsed {len(df)} rows with {len(df.columns)} columns")
                return df
            else:
                return pd.DataFrame()

        except Exception as e:
            logger.error(f"Failed to parse fixed-width file {self.file_path}: {e}")
            raise

    def _parse_chunk(self, lines: List[str]) -> pd.DataFrame:
        """Parse a chunk of lines into a DataFrame."""
        data = []
        for line in lines:
            if len(line) == 0:
                continue

            row = {}
            for field_name, start_pos, length in self.field_specs:
                # Extract field value (handle lines shorter than expected)
                end_pos = min(start_pos + length, len(line))
                value = line[start_pos:end_pos].strip() if start_pos < len(line) else ""
                row[field_name] = value

            data.append(row)

        return pd.DataFrame(data)


class FMCSAInspectionParser(FMCSACSVParser):
    """Specialized parser for inspection data with food safety filtering."""

    def parse_food_safety_violations(self) -> pd.DataFrame:
        """Parse and filter for food safety related violations."""
        df = self.parse_to_dataframe()

        # Filter for food safety violations
        food_safety_mask = df['violation_code'].str.contains(
            '|'.join(FSMA_CONFIG["violation_codes"]),
            case=False,
            na=False
        )

        # Also check description for food safety keywords
        desc_mask = df['violation_description'].str.contains(
            '|'.join(FSMA_CONFIG["food_safety_keywords"]),
            case=False,
            na=False
        )

        # Combine filters
        fsma_df = df[food_safety_mask | desc_mask].copy()

        logger.info(f"Found {len(fsma_df)} food safety related violations")
        return fsma_df

    def get_carrier_violation_summary(self) -> pd.DataFrame:
        """Get summary of violations by carrier."""
        df = self.parse_to_dataframe()

        # Group by carrier and violation type
        summary = df.groupby(['dot_number', 'violation_code']).agg({
            'violation_description': 'first',
            'inspection_date': 'count'
        }).rename(columns={'inspection_date': 'violation_count'}).reset_index()

        return summary.sort_values('violation_count', ascending=False)


class FMCSACensusParser(FMCSACSVParser):
    """Specialized parser for MCMIS Census data."""

    def filter_by_state(self, state_code: str) -> pd.DataFrame:
        """Filter carriers by state."""
        df = self.parse_to_dataframe()
        return df[df['phy_state'].str.upper() == state_code.upper()]

    def filter_brokers(self) -> pd.DataFrame:
        """Filter for brokers only."""
        df = self.parse_to_dataframe()
        # Brokers typically have specific authority indicators
        return df[df['carrier_operation'].str.contains('broker', case=False, na=False)]

    def filter_by_fleet_size(self, min_size: int = 0, max_size: Optional[int] = None) -> pd.DataFrame:
        """Filter by fleet size."""
        df = self.parse_to_dataframe()

        # Convert fleet size to numeric, handling non-numeric values
        df['fleet_size'] = pd.to_numeric(df.get('fleet_size', 0), errors='coerce').fillna(0)

        mask = df['fleet_size'] >= min_size
        if max_size:
            mask &= (df['fleet_size'] <= max_size)

        return df[mask]

    def filter_by_region(self, states: List[str]) -> pd.DataFrame:
        """Filter carriers by multiple states (region)."""
        df = self.parse_to_dataframe()
        return df[df['phy_state'].str.upper().isin([s.upper() for s in states])]

    def filter_carriers(self) -> pd.DataFrame:
        """Filter for carriers only (exclude brokers)."""
        df = self.parse_to_dataframe()
        broker_mask = (
            df['carrier_operation'].str.contains('broker', case=False, na=False) |
            df['entity_type'].str.contains('broker', case=False, na=False)
        )
        return df[~broker_mask]

    def filter_equipment_types(self, equipment_types: List[str]) -> pd.DataFrame:
        """Filter by equipment/cargo types (Reefers, Flatbeds, etc.)."""
        df = self.parse_to_dataframe()

        # Equipment types might be in various columns
        equipment_columns = ['cargo_type', 'equipment_type', 'commodity_type', 'vehicle_type']

        combined_mask = pd.Series([False] * len(df), index=df.index)

        for col in equipment_columns:
            if col in df.columns:
                for eq_type in equipment_types:
                    mask = df[col].str.contains(eq_type, case=False, na=False)
                    combined_mask |= mask

        return df[combined_mask]

    def find_new_companies(self, days_back: int = 30) -> pd.DataFrame:
        """Find newly registered companies."""
        df = self.parse_to_dataframe()

        # Look for add date or registration date columns
        date_columns = ['add_date', 'registration_date', 'created_date', 'start_date']

        for col in date_columns:
            if col in df.columns:
                try:
                    # Convert to datetime
                    df[col] = pd.to_datetime(df[col], errors='coerce')

                    # Filter for recent additions
                    cutoff_date = pd.Timestamp.now() - pd.Timedelta(days=days_back)
                    new_companies = df[df[col] >= cutoff_date]

                    logger.info(f"Found {len(new_companies)} new companies registered in last {days_back} days")
                    return new_companies.sort_values(col, ascending=False)

                except Exception as e:
                    logger.warning(f"Could not parse dates in column {col}: {e}")
                    continue

        logger.warning("No date column found for identifying new companies")
        return pd.DataFrame()

    def filter_by_city(self, city_name: str, state_code: Optional[str] = None) -> pd.DataFrame:
        """Filter carriers by city (and optionally state)."""
        df = self.parse_to_dataframe()

        city_mask = df['phy_city'].str.contains(city_name, case=False, na=False)

        if state_code:
            state_mask = df['phy_state'].str.upper() == state_code.upper()
            return df[city_mask & state_mask]

        return df[city_mask]

    def get_lead_scoring_data(self) -> pd.DataFrame:
        """Get data formatted for lead scoring and prioritization."""
        df = self.parse_to_dataframe()

        # Select key columns for lead scoring
        lead_columns = [
            'dot_number', 'legal_name', 'dba_name', 'phy_city', 'phy_state', 'phy_zip',
            'fleet_size', 'carrier_operation', 'entity_type', 'add_date',
            'email', 'phone', 'contact_name'
        ]

        # Only include columns that exist
        available_columns = [col for col in lead_columns if col in df.columns]

        leads_df = df[available_columns].copy()

        # Add derived scoring columns
        if 'fleet_size' in leads_df.columns:
            leads_df['fleet_size_score'] = pd.cut(
                pd.to_numeric(leads_df['fleet_size'], errors='coerce').fillna(0),
                bins=[0, 1, 5, 20, 100, float('inf')],
                labels=[1, 2, 3, 4, 5]  # Higher score = larger fleet
            ).astype(float)

        # Add recency score for new companies
        if 'add_date' in leads_df.columns:
            try:
                leads_df['add_date'] = pd.to_datetime(leads_df['add_date'], errors='coerce')
                max_date = leads_df['add_date'].max()
                leads_df['recency_score'] = (
                    (max_date - leads_df['add_date']).dt.days / 365.25
                ).apply(lambda x: max(0, 5 - x))  # Newer = higher score
            except:
                leads_df['recency_score'] = 0

        # Calculate overall lead score
        score_columns = [col for col in ['fleet_size_score', 'recency_score'] if col in leads_df.columns]
        if score_columns:
            leads_df['lead_score'] = leads_df[score_columns].mean(axis=1)
            leads_df = leads_df.sort_values('lead_score', ascending=False)

        return leads_df

    def get_contact_info(self) -> pd.DataFrame:
        """Extract contact information for lead outreach."""
        df = self.parse_to_dataframe()

        contact_columns = [
            'dot_number', 'legal_name', 'dba_name',
            'phy_city', 'phy_state', 'phy_zip', 'phy_country',
            'mail_city', 'mail_state', 'mail_zip',
            'email', 'phone', 'fax', 'contact_name'
        ]

        # Only include columns that exist
        available_columns = [col for col in contact_columns if col in df.columns]

        contacts_df = df[available_columns].copy()

        # Clean contact data
        if 'email' in contacts_df.columns:
            contacts_df['email'] = contacts_df['email'].str.lower().str.strip()
            # Filter out invalid emails (basic check)
            contacts_df['valid_email'] = contacts_df['email'].str.contains(
                r'^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$',
                regex=True, na=False
            )

        if 'phone' in contacts_df.columns:
            # Clean phone numbers (remove non-digits)
            contacts_df['clean_phone'] = contacts_df['phone'].str.replace(r'\D', '', regex=True)

        return contacts_df


class FMCSASMSParser(FMCSACSVParser):
    """Specialized parser for Safety Measurement System data."""

    def get_safety_ratings(self) -> pd.DataFrame:
        """Extract carrier safety ratings."""
        df = self.parse_to_dataframe()

        # SMS data typically includes BASIC scores
        basic_columns = [col for col in df.columns if 'basic' in col.lower() or 'score' in col.lower()]

        if basic_columns:
            return df[['dot_number', 'carrier_name'] + basic_columns].copy()
        else:
            logger.warning("No BASIC score columns found in SMS data")
            return df

    def get_high_risk_carriers(self, threshold: float = 3.0) -> pd.DataFrame:
        """Get carriers with high safety scores (indicating higher risk)."""
        df = self.parse_to_dataframe()

        # Look for total score or overall rating
        score_columns = [col for col in df.columns if 'score' in col.lower() or 'rating' in col.lower()]

        if not score_columns:
            logger.warning("No score columns found for risk assessment")
            return pd.DataFrame()

        # Use the first score column found
        score_col = score_columns[0]
        high_risk = df[pd.to_numeric(df[score_col], errors='coerce') >= threshold].copy()

        return high_risk.sort_values(score_col, ascending=False)


def get_parser_for_file(file_path: Path, **kwargs) -> FMCSADataParser:
    """
    Factory function to get appropriate parser for a file.

    Args:
        file_path: Path to the data file
        **kwargs: Additional parser-specific arguments

    Returns:
        Appropriate parser instance
    """
    file_ext = file_path.suffix.lower()

    if file_ext == '.csv':
        # Determine if it's a specialized parser based on filename
        filename = file_path.name.lower()

        if 'census' in filename or 'registration' in filename:
            return FMCSACensusParser(file_path)
        elif 'sms' in filename or 'safety' in filename:
            return FMCSASMSParser(file_path)
        elif 'inspection' in filename:
            return FMCSAInspectionParser(file_path)
        else:
            return FMCSACSVParser(file_path)

    elif file_ext == '.txt':
        # For fixed-width files, field specs need to be provided
        if 'field_specs' in kwargs:
            return FMCSAFixedWidthParser(file_path, kwargs['field_specs'])
        else:
            raise ValueError(f"Field specifications required for fixed-width file: {file_path}")

    else:
        raise ValueError(f"Unsupported file format: {file_ext}")


def parse_file_to_dataframe(file_path: Path, **kwargs) -> pd.DataFrame:
    """
    Convenience function to parse any supported file to DataFrame.

    Args:
        file_path: Path to the data file
        **kwargs: Parser-specific arguments

    Returns:
        Parsed DataFrame
    """
    parser = get_parser_for_file(file_path, **kwargs)
    return parser.parse_to_dataframe()
