"""Unified library scan entry point (CLI and in-process API scans)."""

from pathlib import Path
from typing import Callable, Optional

from db import get_conn
from folder_importer import import_folder
from library_scan.options import ScanOptions
from library_scan.prune import db_now, music_dirs, prune_library
from models import ScanResult


def scan_library(
    path: str | Path,
    *,
    options: Optional[ScanOptions] = None,
    on_folder_done: Optional[Callable[[str], None]] = None,
) -> ScanResult:
    """Import every music folder under path, then prune moved/deleted tracks."""
    opts = options or ScanOptions()
    root = Path(path).expanduser().resolve()
    if not root.is_dir():
        raise FileNotFoundError(f"Music path does not exist: {root}")

    with get_conn() as conn:
        started_at = db_now(conn)

    total = ScanResult()
    for folder in music_dirs(root):
        total.merge(import_folder(folder, analyze=opts.analyze_audio, force=opts.force))
        if on_folder_done:
            on_folder_done(folder)

    with get_conn() as conn:
        prune_library(conn, root, started_at)
    return total
