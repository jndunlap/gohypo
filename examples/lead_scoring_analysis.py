#!/usr/bin/env python3
"""
Example: Comprehensive Lead Scoring Analysis
Demonstrates advanced lead prioritization using all FMCSA data sources.
"""

import sys
from pathlib import Path

# Add the package to Python path
package_dir = Path(__file__).parent.parent / 'python_data_functions'
sys.path.insert(0, str(package_dir))

from python_data_functions.lead_scoring import FMCALeadScorer, score_leads_template
from python_data_functions.api import FMCSADataDownloader
import pandas as pd


def main():
    """Run comprehensive lead scoring analysis on FMCSA data."""

    # Configuration
    DATA_DIR = Path("./data")
    RAW_DIR = DATA_DIR / "raw"
    PROCESSED_DIR = DATA_DIR / "processed"
    PROCESSED_DIR.mkdir(parents=True, exist_ok=True)

    # File paths
    CENSUS_FILE = RAW_DIR / "mcmis_census_20241223_120000.csv"
    SMS_FILE = RAW_DIR / "sms_bulk_20241223_120000.csv"
    LI_FILE = RAW_DIR / "li_bulk_20241223_120000.csv"

    print("🎯 FMCSA Lead Scoring Analysis")
    print("=" * 50)
    print("Analyzing carriers and brokers for:")
    print("• Growth potential and fleet size")
    print("• Safety compliance needs")
    print("• Business legitimacy and authority")
    print("• Contact quality for outreach")
    print("• Specialization and cargo value")
    print("• Company recency and new opportunities")
    print()

    # Download/check data availability
    downloader = FMCSADataDownloader()
    files_to_check = [
        ('MCMIS_CENSUS', CENSUS_FILE, "Census data (required)"),
        ('SMS_BULK', SMS_FILE, "Safety data (recommended)"),
        ('LI_PUBLIC', LI_FILE, "Licensing data (optional)")
    ]

    available_files = {}
    for dataset_key, file_path, description in files_to_check:
        if file_path.exists():
            print(f"✅ {description}: {file_path}")
            available_files[dataset_key] = file_path
        else:
            print(f"❌ {description}: Not found")
            try:
                print(f"📥 Attempting to download {dataset_key}...")
                downloaded = downloader.download_dataset(dataset_key)
                available_files[dataset_key] = downloaded
                print(f"✅ Downloaded: {downloaded}")
            except Exception as e:
                print(f"❌ Download failed: {e}")

    if 'MCMIS_CENSUS' not in available_files:
        print("\n❌ Census data is required for lead scoring. Exiting.")
        return

    print("
🔍 Starting comprehensive lead scoring analysis..."    # Run the scoring template
    scored_leads, report = score_leads_template(
        census_path=available_files['MCMIS_CENSUS'],
        sms_path=available_files.get('SMS_BULK'),
        li_path=available_files.get('LI_PUBLIC'),
        output_dir=PROCESSED_DIR
    )

    # Display detailed analysis
    display_scoring_analysis(scored_leads, report)

    # Show top opportunities
    display_top_opportunities(scored_leads)

    print("
📁 Analysis complete! Check the processed directory for:"    print(f"• Detailed scoring reports: {PROCESSED_DIR}")
    print("• Outreach priority lists"
    print("• Lead quality metrics"
    print("\n💡 Pro tip: Focus your sales efforts on 'Top Tier' leads first!")


def display_scoring_analysis(scored_leads: pd.DataFrame, report: dict):
    """Display comprehensive scoring analysis."""

    print("\n📊 LEAD SCORING ANALYSIS RESULTS")
    print("=" * 50)

    # Overall statistics
    print(f"Total Leads Analyzed: {report['total_leads']:,}")
    print(".1f")
    print(f"Top Tier Leads: {len(report['top_performers'].get('Top Tier', []))}")
    print(f"High Priority Leads: {len(report['top_performers'].get('High Priority', []))}")
    print()

    # Tier breakdown
    print("🏆 Lead Tier Distribution:")
    tiers = report['tier_breakdown']
    for tier, count in tiers.items():
        percentage = (count / report['total_leads']) * 100
        bar = "█" * int(percentage / 5)  # Simple bar chart
        print("15"        print()

    # Scoring component averages
    print("📈 Average Scores by Component:")
    insights = report['scoring_insights']
    components = [
        ('Growth Potential', insights['avg_growth_score']),
        ('Safety/Compliance', insights['avg_safety_score']),
        ('Contact Quality', insights['avg_contact_score']),
        ('Business Legitimacy', 'N/A'),  # Not in insights
        ('Cargo Specialization', 'N/A'),  # Not in insights
        ('Company Recency', 'N/A')  # Not in insights
    ]

    for component, score in components:
        if score != 'N/A':
            print("15.2f"            print("15"        else:
            print("15"    print()

    # Key insights
    print("🔍 Key Insights:")
    print(f"• High-growth carriers (fleet ≥20 trucks): {insights['high_growth_leads']}")
    print(f"• Carriers needing safety improvement: {insights['poor_safety_leads']}")

    growth = report['growth_opportunities']
    print(f"• New companies (<30 days): {growth['new_companies_30_days']}")
    print(f"• Large fleet operators: {growth['large_fleets']}")
    print(f"• Specialized carriers: {growth['specialized_carriers']}")
    print(f"• High-value cargo carriers: {growth['high_value_cargo']}")
    print()

    # Contact quality
    print("📞 Contact Information Quality:")
    contact = report['contact_quality']
    for field, completeness in contact.items():
        print(f"• {field.replace('_', ' ').title()}: {completeness}")
    print()


def display_top_opportunities(scored_leads: pd.DataFrame):
    """Display top lead opportunities for outreach."""

    print("🎯 TOP LEAD OPPORTUNITIES")
    print("=" * 50)

    # Get top tier leads
    top_tier = scored_leads[scored_leads['lead_tier'] == 'Top Tier'].head(10)

    if len(top_tier) == 0:
        print("No Top Tier leads found. Checking High Priority...")
        top_tier = scored_leads[scored_leads['lead_tier'] == 'High Priority'].head(10)

    if len(top_tier) == 0:
        print("No high-priority leads found. Check your data quality.")
        return

    print(f"Showing top {len(top_tier)} highest-scoring leads:")
    print("-" * 100)

    for i, (_, lead) in enumerate(top_tier.iterrows(), 1):
        name = str(lead.get('legal_name', lead.get('dba_name', 'Unknown'))).strip()
        city = str(lead.get('phy_city', 'Unknown')).strip()
        state = str(lead.get('phy_state', 'Unknown')).strip()
        dot = str(lead.get('dot_number', 'N/A')).strip()
        score = lead.get('composite_score', 0)
        tier = lead.get('lead_tier', 'Unknown')
        fleet = lead.get('fleet_size', 'Unknown')
        email = str(lead.get('email', '')).strip()

        # Generate outreach notes
        notes = generate_outreach_notes(lead)

        print("2d"        print("10"        print("15"        if email:
            print("15"        print("15"        if notes:
            print("15"        print()


def generate_outreach_notes(lead) -> str:
    """Generate intelligent outreach notes based on lead characteristics."""
    notes = []

    # Growth indicators
    growth_score = lead.get('growth_score', 0)
    if growth_score >= 4:
        fleet = lead.get('fleet_size', 0)
        if pd.notna(fleet) and fleet >= 20:
            notes.append("Large, growing fleet")
        elif pd.notna(fleet) and fleet >= 10:
            notes.append("Mid-sized growing carrier")

    # Safety/compliance needs
    safety_score = lead.get('safety_score', 0)
    if safety_score >= 4:
        notes.append("May need compliance solutions")

    # Specialization
    spec_score = lead.get('specialization_score', 0)
    if spec_score >= 4:
        cargo = str(lead.get('cargo_carried', '')).lower()
        if 'reefer' in cargo or 'refrigerated' in cargo:
            notes.append("Refrigerated specialist")
        elif 'hazmat' in cargo:
            notes.append("Hazardous materials expert")
        else:
            notes.append("Specialized operations")

    # Recency
    recency_score = lead.get('recency_score', 0)
    if recency_score >= 4:
        notes.append("New company - high potential")

    # Contact quality
    contact_score = lead.get('contact_score', 0)
    if contact_score >= 4:
        notes.append("Strong contact information")
    elif contact_score <= 2:
        notes.append("Limited contact info")

    return "; ".join(notes) if notes else "General carrier/broker"


if __name__ == "__main__":
    main()
