#!/usr/bin/env python3
"""
Example: Generate Carrier Leads by Region and Equipment Type

This script demonstrates how to use the FMCSA data toolkit to generate
targeted carrier leads based on geographic location and equipment types.
"""

import sys
from pathlib import Path

# Add the package to Python path
package_dir = Path(__file__).parent.parent / 'python_data_functions'
sys.path.insert(0, str(package_dir))

from python_data_functions.leads import FMCALeadGenerator, generate_carrier_leads_by_region
from python_data_functions.api import FMCSADataDownloader


def main():
    """Generate carrier leads for refrigerated transport in Western US."""

    # Configuration
    CENSUS_FILE = Path("./data/raw/mcmis_census_20241223_120000.csv")  # Update with your file
    OUTPUT_DIR = Path("./data/processed")
    OUTPUT_DIR.mkdir(parents=True, exist_ok=True)

    # Define target criteria
    TARGET_STATES = ['CA', 'WA', 'OR', 'NV', 'AZ']  # Western US
    EQUIPMENT_TYPES = ['Reefer', 'Refrigerated', 'Temperature Controlled']
    MIN_FLEET_SIZE = 5

    print("🚛 FMCSA Carrier Lead Generation Example")
    print("=" * 50)
    print(f"Target Region: {', '.join(TARGET_STATES)}")
    print(f"Equipment Types: {', '.join(EQUIPMENT_TYPES)}")
    print(f"Minimum Fleet Size: {MIN_FLEET_SIZE}")
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

    # Use convenience function for carrier leads
    print("\n🔍 Generating carrier leads...")
    lead_files = generate_carrier_leads_by_region(
        census_file=CENSUS_FILE,
        states=TARGET_STATES,
        equipment_types=EQUIPMENT_TYPES,
        output_dir=OUTPUT_DIR
    )

    # Load the generated leads for analysis
    import pandas as pd
    leads_file = lead_files['csv']
    leads = pd.read_csv(leads_file)

    print(f"📊 Found {len(leads)} potential carrier leads")

    # Score leads comprehensively
    print("\n⚖️ Scoring leads...")
    scored_leads = lead_gen.score_leads_comprehensive(leads)

    # Display top 10 leads
    print("\n🏆 Top 10 Carrier Leads:")
    print("-" * 80)

    top_leads = scored_leads.head(10)
    for i, (_, lead) in enumerate(top_leads.iterrows(), 1):
        name = lead.get('legal_name', lead.get('dba_name', 'Unknown'))
        city = lead.get('phy_city', 'Unknown')
        state = lead.get('phy_state', 'Unknown')
        fleet = lead.get('fleet_size', 'Unknown')
        score = lead.get('composite_score', 0)

        print(f"{i:2d}. {name[:40]:<40} | {city}, {state} | Fleet: {fleet} | Score: {score:.1f}")

    # Export leads for outreach
    print(f"\n💾 Exporting leads to: {OUTPUT_DIR}")
    exported_files = lead_gen.export_leads_for_outreach(
        scored_leads,
        OUTPUT_DIR / "western_us_reefer_carriers"
    )

    print("📁 Exported files:")
    for fmt, path in exported_files.items():
        print(f"   {fmt.upper()}: {path}")

    # Generate report
    report = lead_gen.get_lead_generation_report(scored_leads)
    print("
📈 Lead Generation Summary:"    print(f"   Total Leads: {report['total_leads']}")
    print(f"   States Covered: {report['states_covered']}")
    print(f"   Average Fleet Size: {report['avg_fleet_size']:.1f}")

    if report['score_distribution']:
        print(f"   Score Distribution: {report['score_distribution']}")

    print("
✅ Lead generation completed!"    print(f"Check {OUTPUT_DIR} for exported lead lists.")


if __name__ == "__main__":
    main()
