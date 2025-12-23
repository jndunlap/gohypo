#!/usr/bin/env python3
"""
Entry point for running FMCSA data functions as a module.
Usage: python -m python_data_functions [arguments]
"""

# Handle both module execution and direct execution
try:
    from .main import main
except ImportError:
    from main import main

if __name__ == "__main__":
    main()
