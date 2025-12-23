#!/usr/bin/env python3
"""
FMCSA Lead Generation - One-Click Runner
Automatically finds data files and generates lead spreadsheets.

Usage: python -m python_data_functions.run_leads
"""

import sys
from pathlib import Path

def find_latest_file(pattern, directory="./data/raw"):
    """Find the most recent file matching a pattern."""
    path = Path(directory)
    if not path.exists():
        return None

    files = list(path.glob(pattern))
    if not files:
        return None

    # Return the most recently modified file
    return max(files, key=lambda f: f.stat().st_mtime)

def main():
    """Run lead generation with auto-detected files."""
    print("🚛 FMCSA Lead Generation - Auto Runner")
    print("=" * 45)

    # Auto-detect data files
    print("🔍 Auto-detecting data files...")

    census_file = find_latest_file("mcmis_census*.csv")
    sms_file = find_latest_file("sms*.csv")
    li_file = find_latest_file("li*.csv")

    files_found = []
    if census_file:
        files_found.append(f"✅ Census: {census_file.name}")
    if sms_file:
        files_found.append(f"✅ Safety: {sms_file.name}")
    if li_file:
        files_found.append(f"✅ Licensing: {li_file.name}")

    if not files_found:
        print("❌ No data files found in ./data/raw/")
        print()
        print("📥 Downloading FMCSA datasets...")
        print("   (This may take several minutes)")
        print()

        # Download datasets
        try:
            from .api import FMCSADataDownloader
            downloader = FMCSADataDownloader()
            downloaded = downloader.download_all_datasets()

            census_file = downloaded.get('MCMIS_CENSUS')
            sms_file = downloaded.get('SMS_BULK')

            if census_file:
                print(f"✅ Downloaded census data: {census_file.name}")
            if sms_file:
                print(f"✅ Downloaded safety data: {sms_file.name}")

        except Exception as e:
            print(f"❌ Download failed: {e}")
            print("💡 Try: pip install pandas requests")
            return False
    else:
        print("Found data files:")
        for file_info in files_found:
            print(f"   {file_info}")
        print()

    # Require at least census data
    if not census_file:
        print("❌ Census data required for lead generation")
        print("💡 Run: python -m python_data_functions --download-all")
        return False

    # Run lead scoring
    print("🎯 Running comprehensive lead scoring...")
    print(f"   Census file: {census_file}")
    if sms_file:
        print(f"   Safety file: {sms_file}")
    if li_file:
        print(f"   License file: {li_file}")
    print()

    try:
        from .lead_scoring import score_leads_template

        scored_leads, report = score_leads_template(
            census_path=census_file,
            sms_path=sms_file,
            li_path=li_file,
            output_dir=Path("./data/processed")
        )

        # Show results
        print("🏆 LEAD SCORING COMPLETE!")
        print("=" * 30)
        print(f"📊 Total leads scored: {report['total_leads']:,}")
        print(".2f")
        print(f"🎯 Top tier leads: {report['top_tier_count']}")

        if report['tier_breakdown']:
            print("🏅 Lead tiers:"            for tier, count in report['tier_breakdown'].items():
                print(f"   • {tier}: {count}")

        print(f"📈 High growth leads: {report['high_growth_leads']}")
        print(f"🛡️  Leads needing safety help: {report['poor_safety_leads']}")
        print()

        # Show output files
        output_dir = Path("./data/processed")
        if output_dir.exists():
            csv_files = list(output_dir.glob("*.csv"))
            if csv_files:
                print("📁 Generated files:")
                for csv_file in sorted(csv_files, key=lambda x: x.stat().st_mtime, reverse=True)[:3]:
                    size_mb = csv_file.stat().st_size / (1024 * 1024)
                    print(".1f"                print()

        print("🎉 Success! Check ./data/processed/ for your lead spreadsheets.")
        print("💡 Top leads are ready for outreach in top_leads_*.csv")

        return True

    except ImportError as e:
        print(f"❌ Import error: {e}")
        print("💡 Try: python -m python_data_functions.test_imports")
        return False
    except Exception as e:
        print(f"❌ Processing error: {e}")
        print("💡 Check that your data files are valid CSV files")
        return False

if __name__ == "__main__":
    success = main()
    sys.exit(0 if success else 1)
