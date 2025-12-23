#!/usr/bin/env python3
"""
Quick Start Example: FMCSA Lead Generation
Demonstrates the simplest way to get started with FMCSA lead scoring.
"""

import sys
from pathlib import Path

# Add the package to Python path (for direct execution)
sys.path.insert(0, str(Path(__file__).parent.parent / 'python_data_functions'))

def main():
    """Quick start example for FMCSA lead generation."""
    print("🚛 FMCSA Lead Generation - Quick Start")
    print("=" * 50)

    # Example data files (you would replace these with your actual files)
    example_census = Path("./data/raw/mcmis_census_example.csv")
    example_sms = Path("./data/raw/sms_example.csv")

    print("📋 This example shows how to:")
    print("   1. Download FMCSA datasets")
    print("   2. Score leads using comprehensive analysis")
    print("   3. Generate prioritized outreach lists")
    print()

    print("💡 Key Commands (that generate spreadsheets):")
    print()

    print("1. 🧪 Test that everything works:")
    print("   python test_imports.py")
    print()

    print("2. 📥 Download FMCSA datasets:")
    print("   python -m python_data_functions --download-all")
    print()

    print("3. 🚀 LEAD SCORING (Generates Excel/CSV files!):")
    print(f"   python -m python_data_functions --score-leads --census-file {example_census}")
    print("   # Output: ./data/processed/top_leads_[date].csv")
    print()

    print("4. 🎯 Full analysis with all data sources:")
    print(f"   python -m python_data_functions --score-leads --census-file {example_census} --sms-file {example_sms}")
    print()

    print("5. 💻 CLI wrapper (easiest):")
    print(f"   python fmcsa_cli.py --score-leads --census-file {example_census}")
    print()

    print("📁 Output will be saved to:")
    print("   ./data/processed/top_leads_[timestamp].csv")
    print("   ./data/processed/lead_scoring_report_[timestamp].csv")
    print()

    print("🎯 Lead Scoring Components:")
    print("   • Growth Potential (25%): Fleet size & utilization")
    print("   • Business Legitimacy (20%): Authority & bonding")
    print("   • Safety/Compliance (20%): BASIC scores & inspections")
    print("   • Contact Quality (15%): Email & phone completeness")
    print("   • Cargo Specialization (10%): High-value commodities")
    print("   • Company Recency (10%): New business identification")
    print()

    print("🏆 Lead Tiers:")
    print("   • Top Tier (4.0-5.0): Immediate outreach priority")
    print("   • High Priority (3.5-4.0): Strong prospects")
    print("   • Medium Priority (2.0-3.5): Nurture candidates")
    print("   • Low Priority (0-2.0): Monitor for qualification")
    print()

    print("🚀 Ready to get started? Run:")
    print("   python -m python_data_functions --download-all")
    print()

    # Test that imports work
    try:
        from python_data_functions.lead_scoring import FMCALeadScorer
        from python_data_functions.api import FMCSADataDownloader
        from python_data_functions.leads import FMCALeadGenerator
        print("✅ Package imports successful!")
    except ImportError as e:
        print(f"❌ Import error: {e}")
        print("Make sure you're running this from the correct directory.")
        print("Try running: python test_imports.py")
        return False

    return True

if __name__ == "__main__":
    success = main()
    if success:
        print("\n🎉 Ready to generate FMCSA leads!")
    else:
        print("\n❌ Setup issue detected. Check the README for installation instructions.")
