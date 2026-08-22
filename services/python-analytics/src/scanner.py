from pathlib import Path
from typing import Callable, Optional

from library_scan import scan_library
from library_scan.options import ScanOptions
from library_scan.progress import CallbackProgress
from models import ScanResult


def scan_path(
    path: str,
    force: bool = False,
    progress_callback: Optional[Callable[[str], None]] = None,
) -> ScanResult:
    """Scan a library path using the unified scan pipeline."""
    progress = CallbackProgress(progress_callback) if progress_callback else None
    return scan_library(
        path,
        options=ScanOptions(force=force),
        progress=progress,
    )


# Re-export for backwards compatibility
from audio_metadata import SUPPORTED_EXTS, get_duration, parse_int_tag

_parse_int = parse_int_tag
_get_duration = get_duration
