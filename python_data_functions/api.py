"""
FMCSA Data Portal API Functions
Handles downloading bulk data from FMCSA datasets and APIs.
"""

import requests
import logging
import time
from pathlib import Path
from typing import Dict, Optional, Tuple, List
from urllib.parse import urljoin
import zipfile
import gzip
from datetime import datetime, timedelta

# Handle both relative and absolute imports
try:
    from .consts import (
        FMCSA_ENDPOINTS, API_CONFIG, DOWNLOAD_CONFIG,
        RAW_DIR, PROCESSED_DIR, LOGGING_CONFIG
    )
except ImportError:
    from consts import (
        FMCSA_ENDPOINTS, API_CONFIG, DOWNLOAD_CONFIG,
        RAW_DIR, PROCESSED_DIR, LOGGING_CONFIG
    )

# Setup logging
logging.basicConfig(
    level=getattr(logging, LOGGING_CONFIG["level"]),
    format=LOGGING_CONFIG["format"],
    handlers=[
        logging.FileHandler(LOGGING_CONFIG["file"]),
        logging.StreamHandler()
    ]
)
logger = logging.getLogger(__name__)


class FMCSADataDownloader:
    """Main class for downloading FMCSA bulk data."""

    def __init__(self, api_key: Optional[str] = None):
        """
        Initialize the downloader.

        Args:
            api_key: FMCSA WebKey for API access (optional)
        """
        self.api_key = api_key
        self.session = requests.Session()
        self.session.headers.update(DOWNLOAD_CONFIG["headers"])

        if api_key:
            self.session.headers.update({"WebKey": api_key})

    def download_dataset(self, dataset_key: str, output_path: Optional[Path] = None) -> Path:
        """
        Download a specific FMCSA dataset.

        Args:
            dataset_key: Key from FMCSA_ENDPOINTS
            output_path: Custom output path (optional)

        Returns:
            Path to downloaded file
        """
        if dataset_key not in FMCSA_ENDPOINTS:
            raise ValueError(f"Unknown dataset: {dataset_key}")

        dataset = FMCSA_ENDPOINTS[dataset_key]
        url = dataset["url"]

        if not output_path:
            timestamp = datetime.now().strftime("%Y%m%d_%H%M%S")
            filename = f"{dataset_key.lower()}_{timestamp}.{dataset['format']}"
            output_path = RAW_DIR / filename

        logger.info(f"Downloading {dataset['name']} to {output_path}")

        try:
            response = self._download_with_retry(url, output_path)
            logger.info(f"Successfully downloaded {dataset_key}")
            return output_path

        except Exception as e:
            logger.error(f"Failed to download {dataset_key}: {e}")
            raise

    def download_all_datasets(self) -> Dict[str, Path]:
        """
        Download all available FMCSA datasets.

        Returns:
            Dictionary mapping dataset keys to file paths
        """
        results = {}
        timestamp = datetime.now().strftime("%Y%m%d_%H%M%S")

        for dataset_key, dataset in FMCSA_ENDPOINTS.items():
            try:
                filename = f"{dataset_key.lower()}_{timestamp}.{dataset['format']}"
                output_path = RAW_DIR / filename
                results[dataset_key] = self.download_dataset(dataset_key, output_path)

            except Exception as e:
                logger.error(f"Failed to download {dataset_key}: {e}")
                continue

        return results

    def _download_with_retry(self, url: str, output_path: Path) -> requests.Response:
        """Download with retry logic for large files."""
        last_exception = None

        for attempt in range(DOWNLOAD_CONFIG["max_retries"]):
            try:
                logger.debug(f"Download attempt {attempt + 1} for {url}")

                with self.session.get(
                    url,
                    stream=True,
                    timeout=DOWNLOAD_CONFIG["timeout"]
                ) as response:

                    response.raise_for_status()

                    # Handle different content encodings
                    if response.headers.get('Content-Encoding') == 'gzip':
                        content = gzip.decompress(response.content)
                        output_path.write_bytes(content)
                    else:
                        with output_path.open('wb') as f:
                            for chunk in response.iter_content(chunk_size=DOWNLOAD_CONFIG["chunk_size"]):
                                if chunk:
                                    f.write(chunk)

                    return response

            except Exception as e:
                last_exception = e
                if attempt < DOWNLOAD_CONFIG["max_retries"] - 1:
                    wait_time = DOWNLOAD_CONFIG["retry_delay"] * (2 ** attempt)
                    logger.warning(f"Download attempt {attempt + 1} failed, retrying in {wait_time}s: {e}")
                    time.sleep(wait_time)
                else:
                    logger.error(f"All download attempts failed for {url}")

        raise last_exception

    def extract_zip_file(self, zip_path: Path, extract_to: Optional[Path] = None) -> List[Path]:
        """
        Extract ZIP files containing multiple datasets.

        Args:
            zip_path: Path to ZIP file
            extract_to: Directory to extract to (default: same as zip_path parent)

        Returns:
            List of extracted file paths
        """
        if not extract_to:
            extract_to = zip_path.parent

        extracted_files = []

        try:
            with zipfile.ZipFile(zip_path, 'r') as zip_ref:
                for file_info in zip_ref.filelist:
                    # Only extract supported file types
                    if any(file_info.filename.endswith(ext) for ext in ['.csv', '.txt', '.xlsx', '.xls']):
                        extracted_path = extract_to / file_info.filename
                        zip_ref.extract(file_info, extract_to)
                        extracted_files.append(extracted_path)
                        logger.info(f"Extracted: {extracted_path}")

        except Exception as e:
            logger.error(f"Failed to extract {zip_path}: {e}")
            raise

        return extracted_files


class FMCSAAPIClient:
    """Client for FMCSA Developer API."""

    def __init__(self, api_key: str):
        """
        Initialize API client.

        Args:
            api_key: FMCSA WebKey
        """
        if not api_key:
            raise ValueError("API key (WebKey) is required for FMCSA API access")

        self.api_key = api_key
        self.base_url = API_CONFIG["base_url"]
        self.session = requests.Session()
        self.session.headers.update({
            "WebKey": api_key,
            "Accept": "application/json",
            "Content-Type": "application/json"
        })

    def get_carrier_data(self, dot_number: str) -> Dict:
        """
        Get carrier data by DOT number.

        Args:
            dot_number: Carrier DOT number

        Returns:
            Carrier data dictionary
        """
        endpoint = urljoin(self.base_url, API_CONFIG["carrier_data_endpoint"])

        params = {"dot": dot_number}

        try:
            response = self.session.get(
                endpoint,
                params=params,
                timeout=API_CONFIG["timeout"]
            )
            response.raise_for_status()
            return response.json()

        except requests.exceptions.RequestException as e:
            logger.error(f"Failed to get carrier data for DOT {dot_number}: {e}")
            raise

    def get_multiple_carriers(self, dot_numbers: List[str]) -> Dict[str, Dict]:
        """
        Get data for multiple carriers with rate limiting.

        Args:
            dot_numbers: List of DOT numbers

        Returns:
            Dictionary mapping DOT numbers to carrier data
        """
        results = {}

        for i, dot_number in enumerate(dot_numbers):
            try:
                results[dot_number] = self.get_carrier_data(dot_number)

                # Rate limiting - avoid overwhelming the API
                if i < len(dot_numbers) - 1:
                    time.sleep(0.1)  # 100ms delay between requests

            except Exception as e:
                logger.error(f"Failed to get data for DOT {dot_number}: {e}")
                results[dot_number] = {"error": str(e)}

        return results


def get_dataset_info(dataset_key: str) -> Dict:
    """
    Get information about a specific dataset.

    Args:
        dataset_key: Dataset key from FMCSA_ENDPOINTS

    Returns:
        Dataset information dictionary
    """
    if dataset_key not in FMCSA_ENDPOINTS:
        raise ValueError(f"Unknown dataset: {dataset_key}")

    return FMCSA_ENDPOINTS[dataset_key].copy()


def list_available_datasets() -> Dict[str, Dict]:
    """
    List all available FMCSA datasets.

    Returns:
        Dictionary of all available datasets
    """
    return FMCSA_ENDPOINTS.copy() 