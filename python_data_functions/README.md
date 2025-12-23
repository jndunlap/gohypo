# FMCSA Data Functions

A comprehensive Python toolkit for downloading and processing FMCSA (Federal Motor Carrier Safety Administration) bulk data for carriers and brokers.

## Overview

This package provides automated tools to download, parse, and analyze FMCSA datasets including:

- **MCMIS Census File**: Complete list of all registered carriers and brokers
- **SMS Safety Data**: Safety performance scores and violation data
- **L&I Data**: Licensing, insurance, and broker authority information
- **Inspection Data**: Vehicle inspection records with food safety compliance

## Features

- 🚀 **Bulk Downloads**: Automated downloading from FMCSA data portals
- 📊 **Data Parsing**: Support for CSV, fixed-width text, and ZIP files
- 🎯 **Lead Generation**: Specialized tools for carrier and broker lead identification
- 🔍 **Advanced Filtering**: Filter by region, equipment type, fleet size, and company age
- 📈 **Lead Scoring**: Comprehensive scoring system using multiple data sources
- 🔗 **API Integration**: FMCSA Developer API client for real-time data
- ✅ **Data Validation**: Comprehensive validation and error handling
- 📊 **Analytics**: Built-in analysis tools for fleet data and risk assessment

## Quick Start

### One-Click Lead Generation (Easiest!)
```bash
# Auto-detects data files and generates spreadsheets
python -m python_data_functions.run_leads
```

## Installation

```bash
# Run the automated setup script (recommended)
python -m python_data_functions.setup

# Or install manually:
pip install pandas requests

# Test the installation
python -m python_data_functions.test_imports
```

## Quick Start

### 1. Download All Datasets

```python
from python_data_functions import FMCSABulkProcessor

# Initialize processor
processor = FMCSABulkProcessor()

# Download all available datasets
downloaded_files = processor.download_all_datasets()
print(f"Downloaded {len(downloaded_files)} datasets")
```

### 2. Process Census Data

```python
# Process carrier/broker registry data
census_df = processor.process_census_data(downloaded_files['MCMIS_CENSUS'])

# Filter for brokers only
brokers = census_df[census_df['carrier_operation'].str.contains('broker', case=False, na=False)]

# Analyze by state
state_summary = census_df.groupby('phy_state').size().sort_values(ascending=False)
print("Top 10 states by carrier count:")
print(state_summary.head(10))
```

### 3. Analyze Safety Data

```python
# Process safety measurement system data
safety_df = processor.process_safety_data(downloaded_files['SMS_BULK'])

# Get high-risk carriers
high_risk = safety_df[safety_df['risk_level'] == 'Critical Risk']
print(f"Found {len(high_risk)} critical risk carriers")
```

### 4. Check Food Safety Compliance

```python
# Process inspection data with food safety focus
inspection_data = processor.process_inspection_data(downloaded_files['INSPECTIONS'])

# Get food safety violations
fsma_violations = inspection_data['food_safety_violations']
carrier_violations = inspection_data['carrier_violation_summary']

print(f"Found {len(fsma_violations)} food safety violations")
print(f"Affecting {len(carrier_violations)} carriers")
```

## Command Line Usage

```bash
# Comprehensive lead scoring analysis
python -m python_data_functions --census-file ./data/raw/mcmis_census.csv --sms-file ./data/raw/sms_data.csv --li-file ./data/raw/li_data.csv --score-leads

# Download all FMCSA datasets
python -m python_data_functions --download-all

# Download and process specific datasets
python -m python_data_functions --download-all --process-census --process-safety

# Use with API key for additional functionality
python -m python_data_functions --api-key YOUR_WEBKEY --download-all

### CLI Wrapper (Easiest)
```bash
# From anywhere - auto-detects files and generates spreadsheets!
python -m python_data_functions.run_leads

# Or use specific commands
python -m python_data_functions.fmcsa_cli --score-leads --census-file ./data/raw/census.csv
python -m python_data_functions.fmcsa_cli --download-all

# Get help and examples
python -m python_data_functions.fmcsa_cli --help
```

### Running Directly
```bash
# From the python_data_functions directory
cd python_data_functions
python main.py --score-leads --census-file ../data/raw/mcmis_census.csv
```

### Testing the Installation
```bash
# Run comprehensive import test (recommended)
python test_imports.py

# Test module execution
python -c "import python_data_functions; print('✅ Module import works!')"

# Test direct execution
cd python_data_functions
python -c "from lead_scoring import FMCALeadScorer; print('✅ Direct imports work!')"

### Troubleshooting Import Issues

**Error: "ImportError: attempted relative import with no known parent package"**

**Solution 1 - Use Module Execution (Recommended):**
```bash
# Always use this method
python -m python_data_functions [arguments]
```

**Solution 2 - Run from Package Directory:**
```bash
cd python_data_functions
python main.py [arguments]
```

**Solution 3 - Run the Test Script:**
```bash
python test_imports.py
# This will verify all imports work correctly in both scenarios
```

**Common Issues:**
- **Running `python main.py` from project root**: Use `python -m python_data_functions` instead
- **Missing dependencies**: Run `pip install pandas requests`
- **Python path issues**: The package handles both relative and absolute imports automatically
```

## Lead Generation

The package includes specialized tools for identifying and scoring carrier and broker leads:

### Generate Carrier Leads

```python
from python_data_functions.leads import FMCALeadGenerator

# Initialize with census data
lead_gen = FMCALeadGenerator(census_file="./data/raw/mcmis_census.csv")

# Generate leads for refrigerated carriers in Western states
carriers = lead_gen.generate_carrier_leads(
    region_states=['CA', 'WA', 'OR', 'NV'],
    equipment_types=['Reefer', 'Refrigerated'],
    min_fleet_size=5,
    include_new_companies=True,
    days_new=180
)

# Score leads comprehensively
scored_leads = lead_gen.score_leads_comprehensive(carriers)

# Export for outreach
lead_gen.export_leads_for_outreach(scored_leads, "./data/processed/western_reefer_carriers")
```

### Generate Broker Leads

```python
# Generate broker leads in major freight corridors
brokers = lead_gen.generate_broker_leads(
    region_states=['CA', 'TX', 'FL', 'IL'],
    include_new_companies=True,
    days_new=90
)

# Score and export
scored_brokers = lead_gen.score_leads_comprehensive(brokers)
lead_gen.export_leads_for_outreach(scored_brokers, "./data/processed/major_brokers")
```

### Lead Scoring Factors

Leads are scored based on:
- **Fleet Size**: Larger fleets get higher scores
- **Company Age**: Newer companies prioritized for lead generation
- **Safety Rating**: Lower risk carriers get higher scores
- **Geographic Relevance**: Regional proximity (future enhancement)

## Advanced Lead Scoring

The package includes a sophisticated lead scoring system that analyzes carriers and brokers across multiple dimensions to identify your best prospects:

### Scoring Components (Weighted)

| Component | Weight | Description |
| --- | --- | --- |
| **Growth Potential** | 25% | Fleet size, utilization rates, expansion indicators |
| **Business Legitimacy** | 20% | Authority status, bonding, insurance verification |
| **Safety/Compliance** | 20% | BASIC scores, violation history, inspection frequency |
| **Contact Quality** | 15% | Email/phone validity, address completeness |
| **Cargo Specialization** | 10% | High-value cargo types, specialized operations |
| **Company Recency** | 10% | New company identification, recent registrations |

### Lead Tiers

- **Top Tier (4.0-5.0)**: Exceptional leads - prioritize immediate outreach
- **High Priority (3.5-4.0)**: Strong prospects with multiple positive indicators
- **Medium Priority (2.0-3.5)**: Viable leads for nurturing campaigns
- **Low Priority (0-2.0)**: Limited potential or poor data quality

### Example Lead Scoring Analysis

```python
from python_data_functions.lead_scoring import FMCALeadScorer

# Initialize scorer
scorer = FMCALeadScorer()

# Load your FMCSA data
census_df = pd.read_csv('./data/raw/mcmis_census.csv')
sms_df = pd.read_csv('./data/raw/sms_safety.csv')  # Optional
li_df = pd.read_csv('./data/raw/li_licensing.csv')  # Optional

# Calculate comprehensive scores
scored_leads = scorer.calculate_comprehensive_score(census_df, sms_df, li_df)

# Get top leads for outreach
top_prospects = scorer.get_top_leads(scored_leads, max_leads=50)

print(f"Found {len(top_prospects)} high-priority leads ready for outreach")
print(top_prospects[['legal_name', 'composite_score', 'lead_tier', 'priority']].head())
```

### Quick Lead Generation

```python
from python_data_functions.leads import FMCALeadGenerator

# Basic lead filtering
generator = FMCALeadGenerator('./data/raw/mcmis_census.csv')

# Get carriers in specific states
california_carriers = generator.filter_carriers(states=['CA'], min_fleet_size=5)

# Get brokers in multiple states
regional_brokers = generator.filter_brokers(states=['CA', 'TX', 'FL'])

# Add contact information
contacts = generator.get_contact_info()
carriers_with_contacts = california_carriers.merge(contacts, on='dot_number', how='left')
```

### Lead Scoring Insights

The scoring system identifies:

**High-Growth Prospects:**
- Carriers adding trucks (fleet expansion)
- High utilization rates (active operations)
- Recent company registrations

**Compliance-Challenged Leads:**
- Poor BASIC scores indicating safety issues
- Frequent violations in key areas
- High inspection frequency (active but struggling)

**Specialized Operations:**
- Hazardous materials carriers
- Refrigerated/temperature-controlled transport
- Oversized/heavy haul specialists
- Time-sensitive delivery services

**Business-Ready Contacts:**
- Verified authority status (active carriers/brokers)
- Proper bonding and insurance
- Complete contact information for outreach

## API Usage

For real-time carrier data, register for a WebKey at [FMCSA Developer Portal](https://mobile.fmcsa.dot.gov/QCDevsite/).

```python
from python_data_functions import FMCSAAPIClient

# Initialize API client
api_client = FMCSAAPIClient(api_key="your_webkey_here")

# Get carrier data by DOT number
carrier_data = api_client.get_carrier_data("1234567")
print(carrier_data)

# Get multiple carriers
dot_numbers = ["1234567", "7654321", "1111111"]
multiple_carriers = api_client.get_multiple_carriers(dot_numbers)
```

## Data Dictionary

### MCMIS Census File
- `dot_number`: Unique carrier identifier
- `legal_name`: Registered business name
- `phy_city`, `phy_state`, `phy_zip`: Physical address
- `fleet_size`: Number of vehicles
- `carrier_operation`: Type of operation (carrier, broker, etc.)

### SMS Safety Data
- `dot_number`: Carrier DOT number
- `carrier_name`: Business name
- `basic_score`: Safety measurement scores
- `risk_level`: Calculated risk category

### Inspection Data
- `dot_number`: Carrier DOT number
- `violation_code`: FMCSA violation code
- `inspection_date`: Date of inspection
- `violation_description`: Description of violation

## Food Safety (FSMA) Analysis

The package includes specialized tools for analyzing food safety compliance under FSMA:

```python
from python_data_functions.parsers import FMCSAInspectionParser

parser = FMCSAInspectionParser(inspection_file_path)

# Get all food safety violations
fsma_violations = parser.parse_food_safety_violations()

# Get carriers with FSMA violations
fsma_carriers = fsma_violations['dot_number'].unique()

# Analyze violation patterns
violation_summary = parser.get_carrier_violation_summary()
```

## Configuration

Edit `consts.py` to modify:

- Dataset URLs and endpoints
- Download timeouts and retry settings
- Data validation rules
- Output formats and compression

## Error Handling

The package includes comprehensive error handling:

```python
from python_data_functions.utils import validate_carrier_data, ValidationError

try:
    df = parse_file_to_dataframe(data_file)
    validation_results = validate_carrier_data(df)

    if not validation_results['valid']:
        print("Validation warnings:", validation_results['warnings'])
        print("Validation errors:", validation_results['errors'])

except ValidationError as e:
    print(f"Data validation failed: {e}")
except Exception as e:
    print(f"Processing failed: {e}")
```

## Logging

All operations are logged to `fmcsa_downloader.log`. Configure logging in `consts.py`:

```python
LOGGING_CONFIG = {
    "level": "INFO",  # DEBUG, INFO, WARNING, ERROR
    "format": "%(asctime)s - %(name)s - %(levelname)s - %(message)s",
    "file": "fmcsa_downloader.log"
}
```

## Data Volume Notes

- **MCMIS Census**: ~500MB, ~2M carriers/brokers
- **SMS Data**: ~100MB ZIP, monthly updates
- **Inspection Data**: ~2GB, daily updates
- **L&I Data**: ~200MB, daily updates

Ensure adequate disk space and processing power for bulk operations.

## Contributing

1. Fork the repository
2. Create a feature branch
3. Add tests for new functionality
4. Submit a pull request

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Support

For issues and questions:

1. Check the logs in `fmcsa_downloader.log`
2. Review FMCSA data documentation
3. Open an issue with sample data and error messages

## Related Links

- [FMCSA Data Portal](https://www.fmcsa.dot.gov/registration/fmcsa-data-dissemination-program)
- [Data.gov Transportation](https://catalog.data.gov/dataset?groups=transportation-marad)
- [FMCSA Developer API](https://mobile.fmcsa.dot.gov/QCDevsite/)
