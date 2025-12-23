#!/usr/bin/env python3
"""
Example: Generate Broker Leads by Region

This script demonstrates how to identify active freight brokers
in specific geographic regions using FMCSA data.
"""

import sys
from pathlib import Path

# Add the package to Python path
package_dir = Path(__file__).parent.parent / 'python_data_functions'
sys.path.insert(0, str(package_dir))

from python_data_functions.leads import FMCALeadGenerator, generate_broker_leads_by_region
from python_data_functions.api import FMCSADataDownloader


def main():
    """Generate broker leads for major freight corridors."""

    # Configuration
    CENSUS_FILE = Path("./data/raw/mcmis_census_20241223_120000.csv")  # Update with your file
    OUTPUT_DIR = Path("./data/processed")
    OUTPUT_DIR.mkdir(parents=True, exist_ok=True)

    # Define target criteria - Major freight corridors
    TARGET_STATES = ['CA', 'TX', 'FL', 'IL', 'GA', 'NJ', 'PA']  # Key freight states

    print("🚛 FMCSA Broker Lead Generation Example")
    print("=" * 50)
    print(f"Target Region: {', '.join(TARGET_STATES)}")
    print()

    # Check if census file exists, download if needed
    if not CENSUS_FILE.exists():
        print("📥 Census file not found, downloading...")
        downloader = FMCSADataDownloader()
        try:
            downloaded = downloader.download_dataset('MCMIS_CENSUS')
            CENSUS_FILE = downloaded
            print(f"✅ Downloaded census data to: {CENSUS_FILE}")
        except Exception as e:
            print(f"❌ Failed to download census data: {e}")
            return
    else:
        print(f"✅ Using existing census file: {CENSUS_FILE}")

    # Use convenience function for broker leads
    print("\n🔍 Generating broker leads...")
    lead_files = generate_broker_leads_by_region(
        census_file=CENSUS_FILE,
        states=TARGET_STATES,
        output_dir=OUTPUT_DIR
    )

    # Load the generated leads for analysis
    import pandas as pd
    leads_file = lead_files['csv']
    leads = pd.read_csv(leads_file)

    print(f"📊 Found {len(leads)} potential broker leads")

    # Score leads comprehensively
    print("\n⚖️ Scoring leads...")
    scored_leads = lead_gen.score_leads_comprehensive(leads)

    # Display top 10 leads
    print("\n🏆 Top 10 Broker Leads:")
    print("-" * 80)

    top_leads = scored_leads.head(10)
    for i, (_, lead) in enumerate(top_leads.iterrows(), 1):
        name = lead.get('legal_name', lead.get('dba_name', 'Unknown'))
        city = lead.get('phy_city', 'Unknown')
        state = lead.get('phy_state', 'Unknown')
        entity_type = lead.get('entity_type', lead.get('carrier_operation', 'Unknown'))
        score = lead.get('composite_score', 0)

        print(f"{i:2d}. {name[:40]:<40} | {city}, {state} | Type: {entity_type[:15]} | Score: {score:.1f}")

    # Export leads for outreach
    print(f"\n💾 Exporting leads to: {OUTPUT_DIR}")
    exported_files = lead_gen.export_leads_for_outreach(
        scored_leads,
        OUTPUT_DIR / "major_states_brokers"
    )

    print("📁 Exported files:")
    for fmt, path in exported_files.items():
        print(f"   {fmt.upper()}: {path}")

    # Generate report
    report = lead_gen.get_lead_generation_report(scored_leads)
    print("
📈 Broker Lead Generation Summary:"    print(f"   Total Leads: {report['total_leads']}")
    print(f"   States Covered: {report['states_covered']}")
    print(f"   Top States: {report['top_states']}")

    if report['score_distribution']:
        print(f"   Score Distribution: {report['score_distribution']}")

    print("
✅ Broker lead generation completed!"    print(f"Check {OUTPUT_DIR} for exported lead lists.")


def find_new_brokers_last_30_days():
    """Example: Find brokers registered in the last 30 days."""
    print("\n🔥 Hot Lead: Brokers Registered in Last 30 Days")
    print("-" * 50)

    CENSUS_FILE = Path("./data/raw/mcmis_census_20241223_120000.csv")

    if not CENSUS_FILE.exists():
        print("❌ Census file not found. Run main example first.")
        return

    lead_gen = FMCALeadGenerator(census_file=CENSUS_FILE)

    # Find new brokers
    new_brokers = lead_gen.census_parser.find_new_companies(days_back=30)
    broker_mask = (
        new_brokers['carrier_operation'].str.contains('broker', case=False, na=False) |
        new_brokers['entity_type'].str.contains('broker', case=False, na=False)
    )
    new_brokers_only = new_brokers[broker_mask]

    if len(new_brokers_only) > 0:
        print(f"Found {len(new_brokers_only)} new brokers registered in last 30 days:")

        for _, broker in new_brokers_only.head(5).iterrows():
            name = broker.get('legal_name', broker.get('dba_name', 'Unknown'))
            city = broker.get('phy_city', 'Unknown')
            state = broker.get('phy_state', 'Unknown')
            reg_date = broker.get('add_date', 'Unknown')

            print(f"   • {name} - {city}, {state} (Registered: {reg_date})")
    else:
        print("No new brokers found in the last 30 days.")


if __name__ == "__main__":
    main()
    find_new_brokers_last_30_days()
