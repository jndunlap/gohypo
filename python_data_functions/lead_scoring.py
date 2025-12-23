"""
FMCSA Lead Scoring Template
Advanced lead prioritization system based on high-value FMCSA data points.
Scores carriers and brokers to identify active operators and high-growth prospects.
"""

import pandas as pd
import numpy as np
from typing import Dict, List, Optional, Tuple, Any
from pathlib import Path
import logging

logger = logging.getLogger(__name__)


class FMCALeadScorer:
    """
    Advanced lead scoring system for FMCSA carrier and broker data.
    Prioritizes leads based on growth potential, legitimacy, and outreach readiness.
    """

    def __init__(self):
        # Define scoring weights for different factors
        self.scoring_weights = {
            'growth_potential': 0.25,
            'legitimacy': 0.20,
            'safety_compliance': 0.20,
            'contact_quality': 0.15,
            'specialization_value': 0.10,
            'company_recency': 0.10
        }

        # High-value cargo types (worth more for lead scoring)
        self.high_value_cargo = {
            'hazardous_materials': ['hazmat', 'hazardous', 'chemical', 'explosive', 'radioactive'],
            'specialized': ['refrigerated', 'reefer', 'temperature_controlled', 'oversized', 'heavy_haul'],
            'valuable': ['household_goods', 'electronics', 'pharmaceutical', 'automotive', 'machinery'],
            'time_sensitive': ['perishable', 'fresh_produce', 'dairy', 'meat', 'bakery']
        }

        # Safety BASIC categories and their importance for lead scoring
        self.safety_basics = {
            'Unsafe Driving': 0.25,
            'Hours of Service Compliance': 0.20,
            'Vehicle Maintenance': 0.20,
            'Controlled Substances/Alcohol': 0.15,
            'Hazardous Materials Compliance': 0.10,
            'Driver Fitness': 0.10
        }

    def calculate_comprehensive_score(self,
                                   census_df: pd.DataFrame,
                                   sms_df: Optional[pd.DataFrame] = None,
                                   li_df: Optional[pd.DataFrame] = None) -> pd.DataFrame:
        """Calculate comprehensive lead scores using all available FMCSA data sources."""
        logger.info("Calculating comprehensive lead scores...")

        scored_df = census_df.copy()

        # Calculate all scoring components in one pass
        component_scores = self._calculate_all_components(scored_df, sms_df, li_df)

        # Calculate composite score using vectorized operations
        scored_df['composite_score'] = sum(
            component_scores[component] * weight
            for component, weight in self.scoring_weights.items()
        )

        # Add scoring tier and sort
        scored_df['lead_tier'] = pd.cut(
            scored_df['composite_score'],
            bins=[0, 2, 3.5, 5, float('inf')],
            labels=['Low Priority', 'Medium Priority', 'High Priority', 'Top Tier']
        )

        scored_df = scored_df.sort_values('composite_score', ascending=False)

        logger.info(f"Scored {len(scored_df)} leads. Top tier: {len(scored_df[scored_df['lead_tier'] == 'Top Tier'])}")

        return scored_df

    def _calculate_all_components(self, df: pd.DataFrame, sms_df: Optional[pd.DataFrame],
                                li_df: Optional[pd.DataFrame]) -> Dict[str, pd.Series]:
        """Calculate all scoring components efficiently in one pass."""
        return {
            'growth_potential': self._calculate_growth_score(df, sms_df),
            'legitimacy': self._calculate_legitimacy_score(df, li_df),
            'safety_compliance': self._calculate_safety_score(df, sms_df),
            'contact_quality': self._calculate_contact_score(df),
            'specialization_value': self._calculate_specialization_score(df),
            'company_recency': self._calculate_recency_score(df)
        }

    def _calculate_growth_score(self, census_df: pd.DataFrame, sms_df: Optional[pd.DataFrame]) -> pd.Series:
        """Calculate growth potential score based on fleet size and utilization."""
        # Fleet size scoring (vectorized)
        fleet_size = pd.to_numeric(census_df.get('fleet_size', 0), errors='coerce').fillna(0)
        fleet_scores = pd.cut(fleet_size, bins=[0, 1, 5, 10, 20, 50, float('inf')],
                            labels=[0, 1, 2, 3, 4, 5], include_lowest=True).astype(int)

        if sms_df is None or 'dot_number' not in sms_df.columns:
            return fleet_scores

        # Merge SMS data for utilization scoring
        merged = census_df[['dot_number']].merge(
            sms_df[['dot_number', 'power_units', 'drivers', 'vmt']],
            on='dot_number', how='left'
        )

        # Utilization ratio scoring
        power_units = pd.to_numeric(merged['power_units'], errors='coerce').fillna(0)
        drivers = pd.to_numeric(merged['drivers'], errors='coerce').fillna(1)  # Avoid division by zero
        utilization_ratio = power_units / drivers

        utilization_scores = pd.cut(utilization_ratio, bins=[0, 1.5, 2, float('inf')],
                                  labels=[0, 1, 2], include_lowest=True).astype(int)

        # VMT scoring (high mileage = high utilization)
        vmt = pd.to_numeric(merged['vmt'], errors='coerce').fillna(0)
        avg_vmt = vmt / power_units.replace(0, 1)  # Avoid division by zero
        vmt_scores = (avg_vmt >= 50000).astype(int)

        return (fleet_scores + utilization_scores + vmt_scores).clip(upper=5)

    def _calculate_legitimacy_score(self, census_df: pd.DataFrame, li_df: Optional[pd.DataFrame]) -> pd.Series:
        """Calculate legitimacy score based on authority status and bonding."""
        # Base score from entity type
        entity_type = census_df.get('entity_type', '').str.lower().fillna('')
        operation = census_df.get('carrier_operation', '').str.lower().fillna('')

        base_scores = pd.Series([3] * len(census_df), index=census_df.index)
        base_scores += (entity_type.str.contains('carrier') | operation.str.contains('carrier')).astype(int)
        base_scores += (entity_type.str.contains('broker') | operation.str.contains('broker')).astype(int)

        if li_df is None or 'dot_number' not in li_df.columns:
            return base_scores.clip(0, 5)

        # Merge L&I data
        merged = census_df[['dot_number']].merge(
            li_df[['dot_number', 'authority_status', 'bond_status', 'bond_amount']],
            on='dot_number', how='left'
        )

        # Authority status scoring
        authority_scores = pd.Series([0] * len(merged), index=merged.index)
        authority_status = merged.get('authority_status', '').str.lower().fillna('')
        authority_scores += authority_status.str.contains('active').astype(int)
        authority_scores -= (authority_status.str.contains('revoked|suspended')).astype(int) * 2

        # Bond status scoring
        bond_scores = pd.Series([0] * len(merged), index=merged.index)
        bond_status = merged.get('bond_status', '').str.lower().fillna('')
        bond_scores += bond_status.str.contains('active|valid').astype(int)

        # Bond amount scoring
        bond_amount = pd.to_numeric(merged.get('bond_amount', 0), errors='coerce').fillna(0)
        bond_scores += (bond_amount >= 75000).astype(int)

        return (base_scores + authority_scores + bond_scores).clip(0, 5)

    def _calculate_safety_score(self, census_df: pd.DataFrame, sms_df: Optional[pd.DataFrame]) -> pd.Series:
        """Calculate safety/compliance score - higher scores for carriers needing improvement."""
        base_scores = pd.Series([3] * len(census_df), index=census_df.index)

        if sms_df is None or 'dot_number' not in sms_df.columns:
            return base_scores

        # Merge SMS data
        merged = census_df[['dot_number']].merge(
            sms_df[['dot_number', 'safety_rating', 'total_inspections'] +
                   [f"{basic.lower().replace(' ', '_')}_score" for basic in self.safety_basics.keys()]],
            on='dot_number', how='left'
        )

        # Safety rating scoring
        safety_scores = pd.Series([0] * len(merged), index=merged.index)
        safety_rating = merged.get('safety_rating', '').str.lower().fillna('')
        safety_scores += safety_rating.str.contains('unsatisfactory').astype(int) * 2
        safety_scores += safety_rating.str.contains('conditional').astype(int)

        # BASIC scores (average of all available BASIC categories)
        basic_columns = [col for col in merged.columns if '_score' in col]
        if basic_columns:
            basic_scores = merged[basic_columns].apply(pd.to_numeric, errors='coerce').mean(axis=1).fillna(0)
            safety_scores += (basic_scores >= 3).astype(int)
            safety_scores += ((basic_scores >= 2) & (basic_scores < 3)).astype(int) * 0.5

        # Inspection activity scoring
        inspections = pd.to_numeric(merged.get('total_inspections', 0), errors='coerce').fillna(0)
        inspection_scores = pd.cut(inspections, bins=[0, 20, 50, float('inf')],
                                 labels=[0, 0.5, 1], include_lowest=True).astype(float)

        return (base_scores + safety_scores + inspection_scores).clip(upper=5)

    def _calculate_contact_score(self, census_df: pd.DataFrame) -> pd.Series:
        """Calculate contact quality score for outreach readiness."""
        score = pd.Series([0] * len(census_df), index=census_df.index)

        # Email scoring (vectorized)
        email = census_df.get('email', '').str.strip().fillna('')
        email_valid = email.str.contains(r'^[^@]+@[^@]+\.[^@]+$', regex=True)
        score += email_valid.astype(int) * 2
        score += (email_valid & email.str.endswith('.com')).astype(int) * 0.5

        # Phone scoring (vectorized)
        phone_digits = census_df.get('phone', '').str.replace(r'\D', '', regex=True).fillna('')
        phone_scores = pd.cut(phone_digits.str.len(), bins=[0, 7, 10, float('inf')],
                             labels=[0, 1, 2], include_lowest=True).astype(int)
        score += phone_scores

        # Address completeness (vectorized)
        address_fields = ['phy_city', 'phy_state', 'phy_zip']
        address_complete = census_df[address_fields].notna() & (census_df[address_fields].astype(str).str.strip() != '')
        score += address_complete.sum(axis=1)

        return score.clip(upper=5)

    def _calculate_specialization_score(self, census_df: pd.DataFrame) -> pd.Series:
        """Calculate specialization value score based on cargo types."""
        cargo_carried = census_df.get('cargo_carried', '').str.lower().fillna('')
        scores = pd.Series([1] * len(census_df), index=census_df.index)  # Base score

        # Vectorized scoring for high-value cargo
        for category, keywords in self.high_value_cargo.items():
            pattern = '|'.join(keywords)
            matches = cargo_carried.str.contains(pattern, regex=True)

            if category == 'hazardous_materials':
                scores += matches.astype(int) * 2
            elif category == 'specialized':
                scores += matches.astype(int) * 1.5
            elif category in ['valuable', 'time_sensitive']:
                scores += matches.astype(int)

        # Additional specialization bonuses
        scores += cargo_carried.str.contains('intermodal').astype(int) * 0.5
        scores += cargo_carried.str.contains('international').astype(int) * 0.5

        return scores.clip(upper=5)

    def _calculate_recency_score(self, census_df: pd.DataFrame) -> pd.Series:
        """Calculate recency score - newer companies get higher scores for lead gen."""
        add_date = pd.to_datetime(census_df.get('add_date'), errors='coerce')
        days_since_registration = (pd.Timestamp.now() - add_date).dt.days

        # Vectorized scoring based on registration recency
        scores = pd.cut(days_since_registration, bins=[0, 30, 90, 365, 730, float('inf')],
                       labels=[5, 4, 3, 2, 1], include_lowest=True).astype(float)

        # Fill NaN values (missing dates) with neutral score
        return scores.fillna(3)

    def get_scoring_summary(self, scored_df: pd.DataFrame) -> Dict[str, Any]:
        """Generate a concise scoring summary."""
        return {
            'total_leads': len(scored_df),
            'tier_breakdown': scored_df['lead_tier'].value_counts().to_dict(),
            'avg_composite_score': scored_df['composite_score'].mean(),
            'top_tier_count': len(scored_df[scored_df['lead_tier'] == 'Top Tier']),
            'high_growth_leads': len(scored_df[scored_df['growth_score'] >= 4]),
            'poor_safety_leads': len(scored_df[scored_df['safety_score'] >= 4])
        }

    def get_top_leads(self, scored_df: pd.DataFrame, max_leads: int = 50) -> pd.DataFrame:
        """Get top-scoring leads ready for outreach."""
        top_leads = scored_df[
            scored_df['lead_tier'].isin(['Top Tier', 'High Priority'])
        ].head(max_leads).copy()

        # Add simple priority notes
        top_leads['priority'] = top_leads['lead_tier'].map({
            'Top Tier': 'Immediate Outreach',
            'High Priority': 'High Priority'
        })

        return top_leads


def score_leads_template(census_path: Path,
                        sms_path: Optional[Path] = None,
                        li_path: Optional[Path] = None,
                        output_dir: Path = None) -> Tuple[pd.DataFrame, Dict[str, Any]]:
    """
    Complete lead scoring workflow template.

    Args:
        census_path: Path to census CSV
        sms_path: Path to SMS safety CSV (optional)
        li_path: Path to L&I CSV (optional)
        output_dir: Output directory

    Returns:
        Tuple of (scored_leads_df, report_dict)
    """
    if output_dir is None:
        output_dir = Path("./data/processed")

    output_dir.mkdir(parents=True, exist_ok=True)

    # Load data
    logger.info("Loading FMCSA datasets for lead scoring...")
    census_df = pd.read_csv(census_path, low_memory=False)

    sms_df = None
    if sms_path and sms_path.exists():
        sms_df = pd.read_csv(sms_path, low_memory=False)
        logger.info(f"Loaded SMS data: {len(sms_df)} records")

    li_df = None
    if li_path and li_path.exists():
        li_df = pd.read_csv(li_path, low_memory=False)
        logger.info(f"Loaded L&I data: {len(li_df)} records")

    # Initialize scorer and calculate scores
    scorer = FMCALeadScorer()
    scored_leads = scorer.calculate_comprehensive_score(census_df, sms_df, li_df)

    # Generate summary report
    report = scorer.get_scoring_summary(scored_leads)

    # Get top leads
    top_leads = scorer.get_top_leads(scored_leads)
    timestamp = pd.Timestamp.now().strftime("%Y%m%d_%H%M%S")
    leads_path = output_dir / f"top_leads_{timestamp}.csv"
    top_leads.to_csv(leads_path, index=False)

    logger.info("Lead scoring complete!")
    logger.info(f"Total leads scored: {len(scored_leads)}")
    logger.info(f"Top leads saved: {len(top_leads)}")
    logger.info(f"Results saved to: {output_dir}")

    return scored_leads, report
