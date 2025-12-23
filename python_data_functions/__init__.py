"""
FMCSA Data Functions Package
A comprehensive toolkit for downloading and processing FMCSA carrier and broker data.

This package provides:
- Bulk download capabilities from FMCSA data portals
- Parsing and processing of various data formats (CSV, fixed-width, ZIP)
- Specialized analysis for carriers, brokers, and food safety compliance
- Integration with FMCSA Developer API

Main Components:
- api.py: Download functionality and API client
- parsers.py: File parsing and data processing
- main.py: Command-line interface and orchestration
- consts.py: Configuration and constants
"""

from .api import FMCSADataDownloader, FMCSAAPIClient, list_available_datasets
from .parsers import (
    parse_file_to_dataframe,
    FMCSACSVParser,
    FMCSAInspectionParser,
    FMCSACensusParser,
    FMCSASMSParser
)
from .leads import FMCALeadGenerator, generate_carrier_leads_by_region, generate_broker_leads_by_region
from .lead_scoring import FMCALeadScorer, score_leads_template
from .main import FMCSABulkProcessor
from .consts import FMCSA_ENDPOINTS, API_CONFIG, DOWNLOAD_CONFIG

# CLI modules
from . import fmcsa_cli
from . import run_leads
from . import test_imports
from . import setup

__version__ = "1.0.0"
__author__ = "FMCSA Data Tools"
__description__ = "Bulk data processing toolkit for FMCSA carrier and broker information"

__all__ = [
    # Main classes
    "FMCSADataDownloader",
    "FMCSAAPIClient",
    "FMCSABulkProcessor",

    # Parsers
    "parse_file_to_dataframe",
    "FMCSACSVParser",
    "FMCSAInspectionParser",
    "FMCSACensusParser",
    "FMCSASMSParser",

    # Lead Generation
    "FMCALeadGenerator",
    "generate_carrier_leads_by_region",
    "generate_broker_leads_by_region",

    # Lead Scoring
    "FMCALeadScorer",
    "score_leads_template",

    # Constants and utilities
    "FMCSA_ENDPOINTS",
    "API_CONFIG",
    "DOWNLOAD_CONFIG",
    "list_available_datasets"
]
