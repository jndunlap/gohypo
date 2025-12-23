"""
FMCSA Data Portal Constants and Configuration
Contains URLs, API endpoints, and configuration for downloading bulk carrier/broker data.
"""

import os
from pathlib import Path

# Base directories
BASE_DIR = Path(__file__).parent.parent
DATA_DIR = BASE_DIR / "data"
RAW_DIR = DATA_DIR / "raw"
PROCESSED_DIR = DATA_DIR / "processed"

# Ensure directories exist
for dir_path in [DATA_DIR, RAW_DIR, PROCESSED_DIR]:
    dir_path.mkdir(parents=True, exist_ok=True)

# FMCSA Dataset URLs and Endpoints
FMCSA_ENDPOINTS = {
    # MCMIS Census File - Master list of all carriers and brokers
    "MCMIS_CENSUS": {
        "name": "Motor Carrier Registrations Census Files",
        "url": "https://data.transportation.gov/api/views/4a2k-zf79/rows.csv?accessType=DOWNLOAD",
        "catalog_url": "https://catalog.data.gov/dataset/motor-carrier-registrations-census-files",
        "documentation": "https://www.fmcsa.dot.gov/registration/fmcsa-data-dissemination-program",
        "format": "csv",
        "update_frequency": "daily_monthly",
        "description": "Complete list of registered carriers and brokers with DOT numbers, legal names, addresses, and fleet sizes"
    },

    # SMS Safety Measurement System - Safety scores and violation data
    "SMS_BULK": {
        "name": "Safety Measurement System Bulk Data",
        "url": "https://ai.fmcsa.dot.gov/SMS/Tools/Index.aspx",
        "documentation": "https://ai.fmcsa.dot.gov/SMS/Information/Resources.aspx",
        "format": "zip",
        "update_frequency": "monthly",
        "description": "Safety performance data including BASICs scores, crash counts, and inspection totals"
    },

    # Licensing & Insurance Data - Authority and insurance status
    "LI_PUBLIC": {
        "name": "Licensing & Insurance Public Data",
        "search_url": "https://li-public.fmcsa.dot.gov/",
        "bulk_url": "https://catalog.data.gov/dataset/authhist",
        "documentation": "https://li-public.fmcsa.dot.gov/LIVIEW/pkg_li_std_routines.prc_help?pn_pageid=17",
        "format": "txt",
        "update_frequency": "daily",
        "description": "Insurance status, broker bonds, and authority information (BMC-84/85 forms)"
    },

    # Vehicle Inspection File - For food safety compliance (FSMA)
    "INSPECTIONS": {
        "name": "Motor Carrier Inspections",
        "url": "https://data.transportation.gov/api/views/3fs9-47s9/rows.csv?accessType=DOWNLOAD",
        "catalog_url": "https://catalog.data.gov/dataset/motor-carrier-inspections",
        "documentation": "https://data.transportation.gov/Trucking-and-Motorcoaches/Motor-Carrier-Inspections/3fs9-47s9",
        "format": "csv",
        "update_frequency": "daily",
        "description": "Vehicle inspection data including food safety violations (49 CFR Part 390.3)"
    }
}

# FMCSA Developer API Configuration
API_CONFIG = {
    "base_url": "https://mobile.fmcsa.dot.gov",
    "developer_portal": "https://mobile.fmcsa.dot.gov/QCDevsite/",
    "api_docs": "https://mobile.fmcsa.dot.gov/QCDevsite/docs/apiAccess",
    "carrier_data_endpoint": "/QCDevsite/api/CarrierData",
    "timeout": 30,  # seconds
    "retry_attempts": 3,
    "retry_delay": 1  # seconds
}

# File format specifications
FILE_FORMATS = {
    "csv": {
        "delimiter": ",",
        "encoding": "utf-8",
        "has_header": True
    },
    "txt_fixed_width": {
        "encoding": "utf-8",
        "has_header": False,
        "line_terminator": "\n"
    },
    "zip": {
        "extract_to_raw": True,
        "supported_extensions": [".csv", ".txt", ".xlsx", ".xls"]
    }
}

# Data validation rules
VALIDATION_RULES = {
    "dot_number": {
        "pattern": r"^\d{1,8}$",  # 1-8 digits
        "required": True,
        "description": "DOT Number must be 1-8 digits"
    },
    "carrier_name": {
        "max_length": 255,
        "required": True,
        "description": "Carrier legal name"
    },
    "state": {
        "pattern": r"^[A-Z]{2}$",
        "required": False,
        "description": "Two-letter state abbreviation"
    }
}

# Food Safety (FSMA) specific constants
FSMA_CONFIG = {
    "violation_codes": [
        "390.3",  # Sanitary Transportation of Food
    ],
    "inspection_types": [
        "Vehicle Inspection",
        "Driver Inspection"
    ],
    "food_safety_keywords": [
        "food", "perishable", "refrigerated", "temperature",
        "sanitary", "contamination", "pest"
    ]
}

# Download configuration
DOWNLOAD_CONFIG = {
    "chunk_size": 8192,  # 8KB chunks for streaming downloads
    "timeout": 300,  # 5 minutes timeout for large files
    "max_retries": 3,
    "retry_delay": 5,  # seconds
    "user_agent": "FMCSA-Bulk-Data-Downloader/1.0",
    "headers": {
        "User-Agent": "FMCSA-Bulk-Data-Downloader/1.0",
        "Accept": "text/csv,application/zip,text/plain,*/*",
        "Accept-Encoding": "gzip, deflate, br"
    }
}

# Database/output configuration
OUTPUT_CONFIG = {
    "supported_formats": ["csv", "json", "parquet", "sqlite"],
    "default_format": "csv",
    "compression": "gzip",  # for large files
    "date_format": "%Y-%m-%d",
    "datetime_format": "%Y-%m-%d %H:%M:%S"
}

# Logging configuration
LOGGING_CONFIG = {
    "level": "INFO",
    "format": "%(asctime)s - %(name)s - %(levelname)s - %(message)s",
    "file": "fmcsa_downloader.log"
} 