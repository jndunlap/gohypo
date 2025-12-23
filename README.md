# FMCSA Lead Generation Toolkit

A comprehensive Python toolkit for downloading, processing, and generating carrier and broker leads from FMCSA bulk data.

## 🚀 Quick Start

**One command to generate lead spreadsheets:**
```bash
python -m python_data_functions.run_leads
```

This automatically:
- Downloads FMCSA datasets (if needed)
- Scores all carriers and brokers
- Generates Excel-ready spreadsheets in `./data/processed/`

## 📖 Documentation

See `python_data_functions/README.md` for complete documentation, installation instructions, and advanced usage examples.

## 🏗️ Project Structure

```
├── python_data_functions/     # Main package
│   ├── lead_scoring.py       # Advanced scoring engine
│   ├── api.py               # Data downloading
│   ├── parsers.py           # File parsing
│   ├── leads.py             # Basic filtering
│   ├── main.py              # CLI interface
│   ├── run_leads.py         # One-click runner
│   ├── fmcsa_cli.py         # CLI wrapper
│   ├── test_imports.py      # Import testing
│   ├── setup.py             # Setup script
│   └── README.md            # Full documentation
├── examples/                # Usage examples
├── FMCSA_Data_Pipeline_TRD.md  # Technical requirements
└── README.md                # This file
```

## 🆘 Need Help?

```bash
# Test everything works
python -m python_data_functions.test_imports

# Setup your environment
python -m python_data_functions.setup

# Get help
python -m python_data_functions --help
```
