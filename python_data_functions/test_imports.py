#!/usr/bin/env python3
"""
Test script to verify FMCSA data functions imports work correctly.
Tests both module execution and direct execution scenarios.
"""

import sys

def test_module_imports():
    """Test imports when running as a module."""
    print("Testing module imports (python -m python_data_functions)...")
    try:
        # Test package imports
        import python_data_functions
        print("✅ Package import successful")

        from python_data_functions.consts import FMCSA_ENDPOINTS
        print("✅ consts import successful")

        from python_data_functions.api import FMCSADataDownloader
        print("✅ api import successful")

        from python_data_functions.parsers import FMCSACensusParser
        print("✅ parsers import successful")

        from python_data_functions.leads import FMCALeadGenerator
        print("✅ leads import successful")

        from python_data_functions.lead_scoring import FMCALeadScorer
        print("✅ lead_scoring import successful")

        return True

    except ImportError as e:
        print(f"❌ Module import error: {e}")
        return False
    except Exception as e:
        print(f"❌ Unexpected module error: {e}")
        return False

def test_direct_imports():
    """Test imports when running files directly."""
    print("\nTesting direct imports (python main.py)...")
    try:
        # Test direct imports (like when running main.py directly)
        from .consts import FMCSA_ENDPOINTS
        print("✅ Direct consts import successful")

        from .api import FMCSADataDownloader
        print("✅ Direct api import successful")

        from .parsers import FMCSACensusParser, FMCSAFixedWidthParser
        print("✅ Direct parsers import successful")

        from .leads import FMCALeadGenerator
        print("✅ Direct leads import successful")

        from .lead_scoring import FMCALeadScorer
        print("✅ Direct lead_scoring import successful")

        # Test main.py imports
        from .main import main
        print("✅ Direct main import successful")

        return True

    except ImportError as e:
        print(f"❌ Direct import error: {e}")
        import traceback
        traceback.print_exc()
        return False
    except Exception as e:
        print(f"❌ Unexpected direct error: {e}")
        import traceback
        traceback.print_exc()
        return False

def main():
    """Run all import tests."""
    print("🚛 FMCSA Data Functions - Import Test Suite")
    print("=" * 50)

    module_success = test_module_imports()
    direct_success = test_direct_imports()

    print("\n" + "=" * 50)

    if module_success and direct_success:
        print("🎉 All import tests passed! The package is ready to use.")
        print("\nUsage examples:")
        print("  # As a module:")
        print("  python -m python_data_functions --download-all")
        print("  python -m python_data_functions --score-leads --census-file ./data/census.csv")
        print()
        print("  # Auto-run everything:")
        print("  python -m python_data_functions.run_leads")
        return True
    else:
        print("❌ Some import tests failed. Check the error messages above.")
        if not module_success:
            print("  - Module imports failed")
        if not direct_success:
            print("  - Direct imports failed")
        return False

if __name__ == "__main__":
    success = main()
    sys.exit(0 if success else 1)
