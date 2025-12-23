#!/usr/bin/env python3
"""
FMCSA Data Functions CLI Wrapper
Run FMCSA lead generation commands from the package directory.

Usage:
    python -m python_data_functions.fmcsa_cli --download-all
    python -m python_data_functions.fmcsa_cli --score-leads --census-file ./data/raw/census.csv
"""

import sys
from pathlib import Path

def main():
    """Run FMCSA CLI commands."""
    # Import and run the main function
    try:
        from .main import main as run_main
        run_main()
    except ImportError as e:
        print(f"❌ Import error: {e}")
        print("\n💡 Troubleshooting:")
        print("   1. Run: python -m python_data_functions.test_imports")
        print("   2. Run: python -m python_data_functions.setup")
        print("   3. Check that pandas is installed: pip install pandas requests")
        sys.exit(1)

if __name__ == "__main__":
    main()
