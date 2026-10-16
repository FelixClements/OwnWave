from typing import Callable, Optional

from library_scan import scan_library
from library_scan.options import ScanOptions
from models import ScanResult


def scan_path(
    path: str,
    force: bool = False,
    progress_callback: Optional[Callable[[str], None]] = None,
) -> ScanResult:
    """Scan a library path using the unified scan pipeline."""
    return scan_library(path, options=ScanOptions(force=force), on_folder_done=progress_callback)
