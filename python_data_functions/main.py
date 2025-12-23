"""
FMCSA Bulk Data Downloader and Processor
Main orchestration script for downloading and processing FMCSA carrier/broker data.
"""

import argparse
import logging
from pathlib import Path
from typing import Dict, List, Optional
import pandas as pd
from datetime import datetime

# Handle both direct execution and module execution
try:
    from .api import FMCSADataDownloader, FMCSAAPIClient, list_available_datasets
    from .parsers import parse_file_to_dataframe, FMCSAInspectionParser
    from .lead_scoring import score_leads_template
    from .consts import RAW_DIR, PROCESSED_DIR, LOGGING_CONFIG
except ImportError:
    # Running directly, use absolute imports
    from api import FMCSADataDownloader, FMCSAAPIClient, list_available_datasets
    from parsers import parse_file_to_dataframe, FMCSAInspectionParser
    from lead_scoring import score_leads_template
    from consts import RAW_DIR, PROCESSED_DIR, LOGGING_CONFIG

# Setup logging
logging.basicConfig(
    level=getattr(logging, LOGGING_CONFIG["level"]),
    format=LOGGING_CONFIG["format"],
    handlers=[
        logging.FileHandler(LOGGING_CONFIG["file"]),
        logging.StreamHandler()
    ]
)
logger = logging.getLogger(__name__)


class FMCSABulkProcessor:
    """Main processor for FMCSA bulk data operations."""

    def __init__(self, api_key: Optional[str] = None):
        """
        Initialize the bulk processor.

        Args:
            api_key: FMCSA WebKey for API access (optional)
        """
        self.downloader = FMCSADataDownloader(api_key)
        self.api_client = FMCSAAPIClient(api_key) if api_key else None

    def download_all_datasets(self) -> Dict[str, Path]:
        """
        Download all available FMCSA datasets.

        Returns:
            Dictionary mapping dataset keys to downloaded file paths
        """
        logger.info("Starting bulk download of all FMCSA datasets")

        try:
            downloaded_files = self.downloader.download_all_datasets()

            logger.info(f"Successfully downloaded {len(downloaded_files)} datasets:")
            for dataset_key, file_path in downloaded_files.items():
                logger.info(f"  - {dataset_key}: {file_path}")

            return downloaded_files

        except Exception as e:
            logger.error(f"Bulk download failed: {e}")
            raise

    def process_census_data(self, census_file: Path) -> pd.DataFrame:
        """
        Process MCMIS Census data with broker/carrier analysis.

        Args:
            census_file: Path to census CSV file

        Returns:
            Processed DataFrame with analysis
        """
        logger.info("Processing census data...")

        try:
            df = parse_file_to_dataframe(census_file)

            # Basic data cleaning
            df = self._clean_census_data(df)

            # Add derived columns
            df = self._add_census_derived_columns(df)

            # Generate summary statistics
            self._generate_census_summary(df)

            output_path = PROCESSED_DIR / f"census_processed_{datetime.now().strftime('%Y%m%d')}.csv"
            df.to_csv(output_path, index=False)

            logger.info(f"Census data processed and saved to: {output_path}")
            return df

        except Exception as e:
            logger.error(f"Failed to process census data: {e}")
            raise

    def process_safety_data(self, sms_file: Path) -> pd.DataFrame:
        """
        Process SMS safety data with risk analysis.

        Args:
            sms_file: Path to SMS data file

        Returns:
            Processed DataFrame with safety analysis
        """
        logger.info("Processing safety data...")

        try:
            # Handle ZIP files - SMS data often comes in ZIP format
            if sms_file.suffix.lower() == '.zip':
                extracted_files = self.downloader.extract_zip_file(sms_file)
                # Use the first CSV file found
                csv_files = [f for f in extracted_files if f.suffix.lower() == '.csv']
                if csv_files:
                    sms_file = csv_files[0]
                else:
                    raise ValueError("No CSV file found in SMS ZIP archive")

            df = parse_file_to_dataframe(sms_file)

            # Clean and process safety data
            df = self._clean_safety_data(df)

            # Calculate safety scores and risk levels
            df = self._calculate_safety_scores(df)

            output_path = PROCESSED_DIR / f"safety_processed_{datetime.now().strftime('%Y%m%d')}.csv"
            df.to_csv(output_path, index=False)

            logger.info(f"Safety data processed and saved to: {output_path}")
            return df

        except Exception as e:
            logger.error(f"Failed to process safety data: {e}")
            raise

    def process_inspection_data(self, inspection_file: Path) -> Dict[str, pd.DataFrame]:
        """
        Process inspection data with food safety focus.

        Args:
            inspection_file: Path to inspection CSV file

        Returns:
            Dictionary with different processed datasets
        """
        logger.info("Processing inspection data...")

        try:
            parser = FMCSAInspectionParser(inspection_file)

            # Get different views of the data
            results = {
                'all_inspections': parser.parse_to_dataframe(),
                'food_safety_violations': parser.parse_food_safety_violations(),
                'carrier_violation_summary': parser.get_carrier_violation_summary()
            }

            # Save processed datasets
            timestamp = datetime.now().strftime('%Y%m%d')
            for name, df in results.items():
                output_path = PROCESSED_DIR / f"{name}_{timestamp}.csv"
                df.to_csv(output_path, index=False)
                logger.info(f"Saved {name} to: {output_path}")

            return results

        except Exception as e:
            logger.error(f"Failed to process inspection data: {e}")
            raise

    def merge_datasets(self, datasets: Dict[str, pd.DataFrame]) -> pd.DataFrame:
        """
        Merge census, safety, and inspection data for comprehensive analysis.

        Args:
            datasets: Dictionary of processed datasets

        Returns:
            Merged comprehensive dataset
        """
        logger.info("Merging datasets for comprehensive analysis...")

        try:
            # Start with census data as the base
            if 'census' not in datasets:
                raise ValueError("Census data required for merging")

            merged_df = datasets['census'].copy()

            # Merge safety data
            if 'safety' in datasets:
                merged_df = merged_df.merge(
                    datasets['safety'],
                    on='dot_number',
                    how='left',
                    suffixes=('', '_safety')
                )

            # Merge inspection summary
            if 'inspections' in datasets and 'carrier_violation_summary' in datasets['inspections']:
                violation_summary = datasets['inspections']['carrier_violation_summary']
                merged_df = merged_df.merge(
                    violation_summary,
                    on='dot_number',
                    how='left',
                    suffixes=('', '_violations')
                )

            # Add food safety compliance indicator
            if 'inspections' in datasets and 'food_safety_violations' in datasets['inspections']:
                fsma_violations = datasets['inspections']['food_safety_violations']
                fsma_carriers = fsma_violations['dot_number'].unique()
                merged_df['has_fsma_violations'] = merged_df['dot_number'].isin(fsma_carriers)

            output_path = PROCESSED_DIR / f"comprehensive_merged_{datetime.now().strftime('%Y%m%d')}.csv"
            merged_df.to_csv(output_path, index=False)

            logger.info(f"Comprehensive dataset created with {len(merged_df)} carriers")
            logger.info(f"Merged data saved to: {output_path}")

            return merged_df

        except Exception as e:
            logger.error(f"Failed to merge datasets: {e}")
            raise

    def _clean_census_data(self, df: pd.DataFrame) -> pd.DataFrame:
        """Clean and standardize census data."""
        # Standardize column names
        df.columns = df.columns.str.lower().str.replace(' ', '_')

        # Clean DOT numbers
        if 'dot_number' in df.columns:
            df['dot_number'] = df['dot_number'].astype(str).str.strip()

        # Clean carrier names
        if 'legal_name' in df.columns:
            df['legal_name'] = df['legal_name'].str.strip()

        # Convert fleet size to numeric
        if 'fleet_size' in df.columns:
            df['fleet_size'] = pd.to_numeric(df['fleet_size'], errors='coerce')

        return df

    def _add_census_derived_columns(self, df: pd.DataFrame) -> pd.DataFrame:
        """Add derived columns to census data."""
        # Fleet size categories
        if 'fleet_size' in df.columns:
            df['fleet_category'] = pd.cut(
                df['fleet_size'],
                bins=[0, 1, 5, 20, 100, float('inf')],
                labels=['1 vehicle', '2-5 vehicles', '6-20 vehicles', '21-100 vehicles', '100+ vehicles']
            )

        # Extract state from address if available
        if 'phy_state' in df.columns:
            df['phy_state'] = df['phy_state'].str.upper()

        return df

    def _generate_census_summary(self, df: pd.DataFrame) -> None:
        """Generate summary statistics for census data."""
        logger.info("=== Census Data Summary ===")
        logger.info(f"Total carriers/brokers: {len(df)}")

        if 'carrier_operation' in df.columns:
            operation_counts = df['carrier_operation'].value_counts()
            logger.info(f"Operation types: {operation_counts.to_dict()}")

        if 'fleet_size' in df.columns:
            logger.info(f"Average fleet size: {df['fleet_size'].mean():.1f}")
            logger.info(f"Median fleet size: {df['fleet_size'].median():.1f}")

        if 'phy_state' in df.columns:
            state_counts = df['phy_state'].value_counts().head(10)
            logger.info(f"Top 10 states by carrier count: {state_counts.to_dict()}")

    def _clean_safety_data(self, df: pd.DataFrame) -> pd.DataFrame:
        """Clean and standardize safety data."""
        # Standardize column names
        df.columns = df.columns.str.lower().str.replace(' ', '_')

        # Clean DOT numbers
        if 'dot_number' in df.columns:
            df['dot_number'] = df['dot_number'].astype(str).str.strip()

        # Convert score columns to numeric
        score_columns = [col for col in df.columns if 'score' in col.lower()]
        for col in score_columns:
            df[col] = pd.to_numeric(df[col], errors='coerce')

        return df

    def _calculate_safety_scores(self, df: pd.DataFrame) -> pd.DataFrame:
        """Calculate overall safety risk scores."""
        score_columns = [col for col in df.columns if 'score' in col.lower()]

        if score_columns:
            # Calculate average safety score
            df['avg_safety_score'] = df[score_columns].mean(axis=1)

            # Categorize risk levels
            df['risk_level'] = pd.cut(
                df['avg_safety_score'],
                bins=[0, 2, 3, 4, float('inf')],
                labels=['Low Risk', 'Moderate Risk', 'High Risk', 'Critical Risk']
            )

        return df


def main():
    """Main function for command-line usage."""
    parser = argparse.ArgumentParser(
        description="FMCSA Bulk Data Downloader and Lead Generation Engine",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  # Download all FMCSA datasets
  python -m python_data_functions --download-all

  # Score leads using census data
  python -m python_data_functions --score-leads --census-file ./data/raw/census.csv

  # Full lead analysis with all data sources
  python -m python_data_functions --score-leads --census-file ./data/census.csv --sms-file ./data/safety.csv

  # Test that everything works
  python test_imports.py
        """
    )
    parser.add_argument("--api-key", help="FMCSA WebKey for API access")
    parser.add_argument("--download-all", action="store_true", help="Download all available datasets")
    parser.add_argument("--process-census", help="Process specific census file")
    parser.add_argument("--process-safety", help="Process specific safety file")
    parser.add_argument("--process-inspections", help="Process specific inspection file")
    parser.add_argument("--merge-all", action="store_true", help="Merge all processed datasets")

    # Lead scoring arguments
    parser.add_argument("--score-leads", action="store_true", help="Score leads using comprehensive FMCSA data analysis")
    parser.add_argument("--census-file", help="Path to census CSV file for lead scoring")
    parser.add_argument("--sms-file", help="Path to SMS safety CSV file for lead scoring")
    parser.add_argument("--li-file", help="Path to L&I licensing CSV file for lead scoring")

    args = parser.parse_args()

    # Check if any action was requested
    actions_requested = any([
        args.download_all,
        args.process_census,
        args.process_safety,
        args.process_inspections,
        args.merge_all,
        args.score_leads
    ])

    if not actions_requested:
        parser.print_help()
        print("\n❌ No action specified!")
        print("💡 Try one of these commands:")
        print("   python -m python_data_functions --download-all")
        print("   python -m python_data_functions --score-leads --census-file ./data/census.csv")
        print("   python test_imports.py")
        return

    try:
        processor = FMCSABulkProcessor(args.api_key)

        datasets = {}
        downloaded_files = {}

        # Download phase
        if args.download_all:
            downloaded_files = processor.download_all_datasets()

        # Processing phase
        if args.process_census:
            census_file = Path(args.process_census)
            datasets['census'] = processor.process_census_data(census_file)

        if args.process_safety:
            safety_file = Path(args.process_safety)
            datasets['safety'] = processor.process_safety_data(safety_file)

        if args.process_inspections:
            inspection_file = Path(args.process_inspections)
            datasets['inspections'] = processor.process_inspection_data(inspection_file)

        # Auto-process downloaded files
        if downloaded_files:
            for dataset_key, file_path in downloaded_files.items():
                try:
                    if dataset_key == 'MCMIS_CENSUS':
                        datasets['census'] = processor.process_census_data(file_path)
                    elif dataset_key == 'SMS_BULK':
                        datasets['safety'] = processor.process_safety_data(file_path)
                    elif dataset_key == 'INSPECTIONS':
                        datasets['inspections'] = processor.process_inspection_data(file_path)
                except Exception as e:
                    logger.error(f"Failed to process {dataset_key}: {e}")

        # Merge phase
        if args.merge_all and len(datasets) > 1:
            merged_df = processor.merge_datasets(datasets)
            logger.info("Dataset merging completed successfully")

        # Lead scoring phase
        if args.score_leads:
            census_file_path = args.census_file or downloaded_files.get('MCMIS_CENSUS')
            if not census_file_path:
                logger.error("Census file required for lead scoring. Use --census-file or --download-all")
            else:
                logger.info("Running comprehensive lead scoring analysis...")

                sms_file_path = args.sms_file or downloaded_files.get('SMS_BULK')
                li_file_path = args.li_file

                scored_leads, report = score_leads_template(
                    census_path=Path(census_file_path),
                    sms_path=Path(sms_file_path) if sms_file_path else None,
                    li_path=Path(li_file_path) if li_file_path else None,
                    output_dir=PROCESSED_DIR
                )

                # Print scoring summary
                print("\n🏆 Lead Scoring Summary:")
                print("=" * 50)
                print(f"Total leads scored: {report['total_leads']}")
                print(f"Average composite score: {report['avg_composite_score']:.2f}")
                print(f"Top tier leads: {report['top_tier_count']}")

                print(f"Tier breakdown: {report['tier_breakdown']}")
                print(f"High growth leads: {report['high_growth_leads']}")
                print(f"Leads needing safety help: {report['poor_safety_leads']}")

                logger.info("Lead scoring analysis completed. Check processed directory for detailed reports.")

        logger.info("FMCSA data processing completed")

    except Exception as e:
        logger.error(f"Processing failed: {e}")
        raise


if __name__ == "__main__":
    main() 