#!/usr/bin/env python3
"""
Setup script for FMCSA Data Functions
Helps users get started with the FMCSA lead generation toolkit.
"""

import os
import sys
import subprocess

def check_python_version():
    """Check if Python version is compatible."""
    if sys.version_info < (3, 8):
        print("❌ Python 3.8+ is required")
        print(f"   Current version: {sys.version}")
        return False
    print(f"✅ Python {sys.version.split()[0]} detected")
    return True

def check_dependencies():
    """Check if required dependencies are installed."""
    required_packages = ['pandas', 'requests']

    missing_packages = []
    for package in required_packages:
        try:
            __import__(package)
            print(f"✅ {package} is installed")
        except ImportError:
            missing_packages.append(package)
            print(f"❌ {package} is missing")

    if missing_packages:
        print("
📦 Installing missing dependencies..."        try:
            subprocess.check_call([sys.executable, '-m', 'pip', 'install'] + missing_packages)
            print("✅ Dependencies installed successfully")
            return True
        except subprocess.CalledProcessError:
            print("❌ Failed to install dependencies. Try: pip install pandas requests")
            return False

    return True

def test_imports():
    """Test that all imports work correctly."""
    print("
🔍 Testing imports..."    try:
        # Test key imports
        from .lead_scoring import FMCALeadScorer
        from .api import FMCSADataDownloader
        from .leads import FMCALeadGenerator

        print("✅ All imports successful")
        return True

    except ImportError as e:
        print(f"❌ Import error: {e}")
        print("💡 Try: python -m python_data_functions.test_imports")
        return False

def create_directories():
    """Create necessary directories."""
    directories = [
        'data',
        'data/raw',
        'data/processed'
    ]

    for directory in directories:
        if not os.path.exists(directory):
            os.makedirs(directory)
            print(f"📁 Created directory: {directory}")
        else:
            print(f"✅ Directory exists: {directory}")

def main():
    """Run the setup process."""
    print("🚛 FMCSA Data Functions - Setup Script")
    print("=" * 50)

    # Check Python version
    if not check_python_version():
        return False

    # Create directories
    print("\n📁 Creating directories...")
    create_directories()

    # Check dependencies
    print("\n📦 Checking dependencies...")
    if not check_dependencies():
        return False

    # Test imports
    if not test_imports():
        return False

    print("\n" + "=" * 50)
    print("🎉 Setup complete! You're ready to use FMCSA Data Functions.")
    print()
    print("🚀 Quick start commands:")
    print("   python -m python_data_functions --download-all")
    print("   python -m python_data_functions --score-leads --census-file ./data/census.csv")
    print()
    print("📖 For more examples, see examples/quick_start.py")
    print("📚 For full documentation, see python_data_functions/README.md")
    print()
    print("🆘 Need help? Run: python -m python_data_functions.test_imports")

    return True

if __name__ == "__main__":
    success = main()
    sys.exit(0 if success else 1)
