"""
FMCSA Lead Generation and Filtering
Basic lead filtering and generation tools.
Advanced scoring functionality moved to lead_scoring.py
"""

import pandas as pd
from pathlib import Path
from typing import Dict, List, Optional
import logging

# Handle both relative and absolute imports
try:
    from .parsers import FMCSACensusParser
    from .utils import export_to_multiple_formats
except ImportError:
    from parsers import FMCSACensusParser
    from utils import export_to_multiple_formats

logger = logging.getLogger(__name__)


class FMCALeadGenerator:
    """Basic lead generation for carriers and brokers using FMCSA census data."""

    def __init__(self, census_file: Optional[Path] = None):
        """Initialize with census data file."""
        self.census_file = census_file
        self.census_parser = FMCSACensusParser(census_file) if census_file else None

    def filter_carriers(self, states: Optional[List[str]] = None,
                       equipment_types: Optional[List[str]] = None,
                       min_fleet_size: int = 1) -> pd.DataFrame:
        """Filter carriers by basic criteria."""
        if not self.census_parser:
            raise ValueError("Census data required")

        carriers = self.census_parser.filter_carriers()
        carriers = self.census_parser.filter_by_fleet_size(min_fleet_size)

        if states:
            carriers = carriers[carriers['phy_state'].str.upper().isin([s.upper() for s in states])]

        if equipment_types:
            carriers = self.census_parser.filter_equipment_types(equipment_types)

        return carriers

    def filter_brokers(self, states: Optional[List[str]] = None) -> pd.DataFrame:
        """Filter brokers by basic criteria."""
        if not self.census_parser:
            raise ValueError("Census data required")

        brokers = self.census_parser.filter_brokers()

        if states:
            brokers = brokers[brokers['phy_state'].str.upper().isin([s.upper() for s in states])]

        return brokers

    def get_contact_info(self) -> pd.DataFrame:
        """Get contact information for leads."""
        if not self.census_parser:
            raise ValueError("Census data required")
        return self.census_parser.get_contact_info()


# Convenience functions for common use cases
def generate_carrier_leads_by_region(census_file: Path,
                                   states: List[str],
                                   equipment_types: Optional[List[str]] = None,
                                   output_dir: Optional[Path] = None) -> Dict[str, Path]:
    """Convenience function to generate carrier leads for specific regions."""
    output_dir = output_dir or Path("./data/processed")
    output_dir.mkdir(parents=True, exist_ok=True)

    generator = FMCALeadGenerator(census_file)
    leads = generator.filter_carriers(states=states, equipment_types=equipment_types)

    # Add contact info
    contacts = generator.get_contact_info()
    leads_with_contacts = leads.merge(contacts, on='dot_number', how='left')

    # Export
    timestamp = pd.Timestamp.now().strftime("%Y%m%d_%H%M%S")
    output_path = output_dir / f"carrier_leads_{'_'.join(states).lower()}_{timestamp}.csv"
    leads_with_contacts.to_csv(output_path, index=False)

    return {"csv": output_path}


def generate_broker_leads_by_region(census_file: Path,
                                  states: List[str],
                                  output_dir: Optional[Path] = None) -> Dict[str, Path]:
    """Convenience function to generate broker leads for specific regions."""
    output_dir = output_dir or Path("./data/processed")
    output_dir.mkdir(parents=True, exist_ok=True)

    generator = FMCALeadGenerator(census_file)
    leads = generator.filter_brokers(states=states)

    # Add contact info
    contacts = generator.get_contact_info()
    leads_with_contacts = leads.merge(contacts, on='dot_number', how='left')

    # Export
    timestamp = pd.Timestamp.now().strftime("%Y%m%d_%H%M%S")
    output_path = output_dir / f"broker_leads_{'_'.join(states).lower()}_{timestamp}.csv"
    leads_with_contacts.to_csv(output_path, index=False)

    return {"csv": output_path}
                                leads_df: pd.DataFrame,
                                weights: Optional[Dict[str, float]] = None) -> pd.DataFrame:
        """
        Score leads using comprehensive criteria including safety data.

        Args:
            leads_df: Base leads dataframe
            weights: Custom weights for scoring factors

        Returns:
            Leads with comprehensive scoring
        """
        scored_leads = leads_df.copy()

        # Default weights
        default_weights = {
            'fleet_size': 0.3,
            'company_age': 0.2,
            'safety_score': 0.3,
            'geographic_proximity': 0.2  # Placeholder for future implementation
        }

        weights = weights or default_weights

        # Fleet size score (larger fleets = higher score)
        if 'fleet_size' in scored_leads.columns:
            scored_leads['fleet_size_score'] = pd.cut(
                pd.to_numeric(scored_leads['fleet_size'], errors='coerce').fillna(0),
                bins=[0, 1, 5, 20, 100, float('inf')],
                labels=[1, 2, 3, 4, 5]
            ).astype(float)
        else:
            scored_leads['fleet_size_score'] = 3  # Neutral score

        # Company age score (newer companies = higher score for lead gen)
        if 'add_date' in scored_leads.columns:
            try:
                scored_leads['add_date'] = pd.to_datetime(scored_leads['add_date'], errors='coerce')
                max_date = scored_leads['add_date'].max()
                scored_leads['company_age_years'] = (max_date - scored_leads['add_date']).dt.days / 365.25
                scored_leads['company_age_score'] = scored_leads['company_age_years'].apply(
                    lambda x: max(1, 6 - x) if pd.notna(x) else 3
                )
            except:
                scored_leads['company_age_score'] = 3
        else:
            scored_leads['company_age_score'] = 3

        # Safety score (lower risk = higher score)
        if self.sms_parser:
            try:
                safety_data = self.sms_parser.get_safety_ratings()
                scored_leads = scored_leads.merge(
                    safety_data[['dot_number', 'avg_safety_score']],
                    on='dot_number',
                    how='left'
                )

                # Convert safety score to lead score (lower safety score = higher lead score)
                scored_leads['safety_lead_score'] = pd.cut(
                    scored_leads['avg_safety_score'].fillna(2.5),  # Neutral score for missing data
                    bins=[0, 1, 2, 3, 4, float('inf')],
                    labels=[5, 4, 3, 2, 1]
                ).astype(float)
            except Exception as e:
                logger.warning(f"Could not incorporate safety data: {e}")
                scored_leads['safety_lead_score'] = 3
        else:
            scored_leads['safety_lead_score'] = 3

        # Calculate composite score
        scored_leads['composite_score'] = (
            scored_leads['fleet_size_score'] * weights['fleet_size'] +
            scored_leads['company_age_score'] * weights['company_age'] +
            scored_leads['safety_lead_score'] * weights['safety_score']
        )

        # Sort by composite score
        scored_leads = scored_leads.sort_values('composite_score', ascending=False)

        return scored_leads

    def find_regional_leads(self,
                          target_states: List[str],
                          lead_type: str = 'carrier',
                          equipment_types: Optional[List[str]] = None,
                          min_fleet_size: int = 1) -> pd.DataFrame:
        """
        Find leads in specific geographic regions.

        Args:
            target_states: List of state codes
            lead_type: 'carrier' or 'broker'
            equipment_types: Equipment types to filter
            min_fleet_size: Minimum fleet size

        Returns:
            Regional leads sorted by relevance
        """
        logger.info(f"Finding {lead_type} leads in states: {target_states}")

        if lead_type == 'carrier':
            leads = self.generate_carrier_leads(
                region_states=target_states,
                equipment_types=equipment_types,
                min_fleet_size=min_fleet_size
            )
        elif lead_type == 'broker':
            leads = self.generate_broker_leads(region_states=target_states)
        else:
            raise ValueError("lead_type must be 'carrier' or 'broker'")

        # Score the leads
        scored_leads = self.score_leads_comprehensive(leads)

        return scored_leads

    def find_equipment_specific_leads(self,
                                    equipment_types: List[str],
                                    region_states: Optional[List[str]] = None,
                                    min_fleet_size: int = 1) -> pd.DataFrame:
        """
        Find carriers with specific equipment types.

        Args:
            equipment_types: Equipment types (e.g., ['Reefer', 'Flatbed'])
            region_states: Optional state filter
            min_fleet_size: Minimum fleet size

        Returns:
            Equipment-specific leads
        """
        logger.info(f"Finding carriers with equipment: {equipment_types}")

        leads = self.generate_carrier_leads(
            region_states=region_states,
            equipment_types=equipment_types,
            min_fleet_size=min_fleet_size
        )

        # Score the leads
        scored_leads = self.score_leads_comprehensive(leads)

        return scored_leads

    def export_leads_for_outreach(self,
                                leads_df: pd.DataFrame,
                                output_path: Path,
                                include_contact_info: bool = True) -> Dict[str, Path]:
        """
        Export leads in outreach-ready format.

        Args:
            leads_df: Scored leads dataframe
            output_path: Base output path
            include_contact_info: Whether to include contact details

        Returns:
            Dictionary of exported file paths
        """
        if include_contact_info:
            # Get contact information
            if self.census_parser:
                contact_df = self.census_parser.get_contact_info()
                leads_df = leads_df.merge(
                    contact_df,
                    on='dot_number',
                    how='left',
                    suffixes=('', '_contact')
                )

        # Select columns for outreach
        outreach_columns = [
            'dot_number', 'legal_name', 'dba_name', 'composite_score',
            'phy_city', 'phy_state', 'phy_zip', 'fleet_size',
            'email', 'clean_phone', 'contact_name', 'carrier_operation'
        ]

        # Only include columns that exist
        available_columns = [col for col in outreach_columns if col in leads_df.columns]
        outreach_df = leads_df[available_columns].copy()

        # Sort by score for priority outreach
        outreach_df = outreach_df.sort_values('composite_score', ascending=False)

        # Export in multiple formats
        return export_to_multiple_formats(outreach_df, output_path, ['csv', 'xlsx'])

    def get_lead_generation_report(self, leads_df: pd.DataFrame) -> Dict:
        """
        Generate a summary report of lead generation results.

        Args:
            leads_df: Generated leads dataframe

        Returns:
            Summary statistics dictionary
        """
        report = {
            'total_leads': len(leads_df),
            'avg_fleet_size': leads_df.get('fleet_size', pd.Series()).mean(),
            'states_covered': leads_df.get('phy_state', pd.Series()).nunique(),
            'top_states': leads_df.get('phy_state', pd.Series()).value_counts().head(5).to_dict(),
            'score_distribution': None,
            'contact_info_available': 0
        }

        # Score distribution
        if 'composite_score' in leads_df.columns:
            score_dist = pd.cut(
                leads_df['composite_score'],
                bins=[0, 2, 3, 4, 5, 6],
                labels=['Poor', 'Below Average', 'Average', 'Good', 'Excellent']
            ).value_counts().to_dict()
            report['score_distribution'] = score_dist

        # Contact info availability
        contact_fields = ['email', 'phone', 'contact_name']
        available_contacts = 0
        for field in contact_fields:
            if field in leads_df.columns:
                available_contacts += leads_df[field].notna().sum()

        report['contact_info_available'] = available_contacts

        return report


def generate_carrier_leads_by_region(census_file: Path,
                                   states: List[str],
                                   equipment_types: Optional[List[str]] = None,
                                   output_dir: Optional[Path] = None) -> Dict[str, Path]:
    """
    Convenience function to generate carrier leads for specific regions.

    Args:
        census_file: Path to census CSV
        states: List of state codes
        equipment_types: Optional equipment type filter
        output_dir: Output directory (defaults to ./data/processed)

    Returns:
        Dictionary of generated file paths
    """
    output_dir = output_dir or Path("./data/processed")
    output_dir.mkdir(parents=True, exist_ok=True)

    # Initialize lead generator
    lead_gen = FMCALeadGenerator(census_file=census_file)

    # Generate leads
    leads = lead_gen.find_regional_leads(
        target_states=states,
        lead_type='carrier',
        equipment_types=equipment_types
    )

    # Export for outreach
    timestamp = datetime.now().strftime("%Y%m%d_%H%M%S")
    output_base = output_dir / f"carrier_leads_{'_'.join(states).lower()}_{timestamp}"

    exported_files = lead_gen.export_leads_for_outreach(leads, output_base)

    # Generate report
    report = lead_gen.get_lead_generation_report(leads)
    logger.info("Lead Generation Report:")
    for key, value in report.items():
        logger.info(f"  {key}: {value}")

    return exported_files


def generate_broker_leads_by_region(census_file: Path,
                                  states: List[str],
                                  output_dir: Optional[Path] = None) -> Dict[str, Path]:
    """
    Convenience function to generate broker leads for specific regions.

    Args:
        census_file: Path to census CSV
        states: List of state codes
        output_dir: Output directory (defaults to ./data/processed)

    Returns:
        Dictionary of generated file paths
    """
    output_dir = output_dir or Path("./data/processed")
    output_dir.mkdir(parents=True, exist_ok=True)

    # Initialize lead generator
    lead_gen = FMCALeadGenerator(census_file=census_file)

    # Generate leads
    leads = lead_gen.find_regional_leads(
        target_states=states,
        lead_type='broker'
    )

    # Export for outreach
    timestamp = datetime.now().strftime("%Y%m%d_%H%M%S")
    output_base = output_dir / f"broker_leads_{'_'.join(states).lower()}_{timestamp}"

    exported_files = lead_gen.export_leads_for_outreach(leads, output_base)

    # Generate report
    report = lead_gen.get_lead_generation_report(leads)
    logger.info("Broker Lead Generation Report:")
    for key, value in report.items():
        logger.info(f"  {key}: {value}")

    return exported_files
