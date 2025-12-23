#!/usr/bin/env python3
"""
Demo: Commands That Generate Spreadsheets
Shows exactly which commands create Excel/CSV output files.
"""

import sys
from pathlib import Path

# Add the package to Python path
package_dir = Path(__file__).parent.parent / 'python_data_functions'
sys.path.insert(0, str(package_dir))

def main():
    """Show which commands generate spreadsheets."""
    print("📊 FMCSA Lead Generation - Spreadsheet Generation Guide")
    print("=" * 60)

    print("❌ Commands that do NOT generate spreadsheets:")
    print("   python main.py                           # No arguments")
    print("   python -m python_data_functions         # No arguments")
    print("   python fmcsa_cli.py                     # No arguments")
    print("   python -m python_data_functions --download-all  # Only downloads")
    print()

    print("✅ Commands that DO generate spreadsheets:")
    print()

    print("1️⃣  BASIC LEAD SCORING:")
    print("   python -m python_data_functions --score-leads --census-file ./data/census.csv")
    print("   📁 Output files:")
    print("      • ./data/processed/top_leads_[date].csv")
    print("      • ./data/processed/lead_scoring_report_[date].csv")
    print()

    print("2️⃣  FULL ANALYSIS (with safety data):")
    print("   python -m python_data_functions --score-leads --census-file ./data/census.csv --sms-file ./data/safety.csv")
    print("   📁 Same output files as above, plus enhanced scoring")
    print()

    print("3️⃣  CLI WRAPPER (easiest):")
    print("   python fmcsa_cli.py --score-leads --census-file ./data/census.csv")
    print("   📁 Same output files")
    print()

    print("4️⃣  DIRECT EXECUTION:")
    print("   cd python_data_functions")
    print("   python main.py --score-leads --census-file ../data/census.csv")
    print("   📁 Same output files")
    print()

    print("🔍 What the spreadsheets contain:")
    print("   • top_leads_[date].csv - Prioritized leads ready for outreach")
    print("     - Columns: dot_number, legal_name, composite_score, lead_tier, priority")
    print("     - Sorted by score (highest first)")
    print("     - Top 50 leads by default")
    print()

    print("   • lead_scoring_report_[date].csv - Detailed analysis")
    print("     - All scored leads with component scores")
    print("     - Growth, safety, legitimacy, contact scores")
    print("     - Lead tier classifications")
    print()

    print("🎯 Lead Tiers in spreadsheets:")
    print("   • Top Tier (4.0-5.0): Immediate outreach priority")
    print("   • High Priority (3.5-4.0): Strong prospects")
    print("   • Medium Priority (2.0-3.5): Nurture candidates")
    print("   • Low Priority (0-2.0): Monitor for qualification")
    print()

    print("📋 Prerequisites:")
    print("   1. Census data: Download from FMCSA or use existing file")
    print("   2. Optional: SMS safety data for enhanced scoring")
    print("   3. Run: python test_imports.py (should pass all tests)")
    print()

    print("🚀 To generate your first spreadsheet:")
    print("   1. Get census data: python -m python_data_functions --download-all")
    print("   2. Score leads: python -m python_data_functions --score-leads --census-file ./data/raw/mcmis_census_*.csv")
    print("   3. Check ./data/processed/ for the generated files!")
    print()

    # Test if the scoring function is available
    try:
        from lead_scoring import FMCALeadScorer
        print("✅ Lead scoring system is ready!")
    except ImportError as e:
        print(f"❌ Import error: {e}")
        print("   Run: python test_imports.py")
        return False

    return True

if __name__ == "__main__":
    success = main()
    if success:
        print("\n🎉 Ready to generate spreadsheets!")
        print("Run one of the commands above to create your first lead scoring spreadsheet.")
    else:
        print("\n❌ Setup issue detected.")
