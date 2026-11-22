"""Unified library scan entry point (CLI and in-process API scans)."""

from pathlib import Path
from typing import Callable, Optional

import genre_audio
from db import get_conn
from folder_importer import import_folder
from library_scan.options import ScanOptions
from library_scan.plan import plan_scan
from library_scan.prune import db_now, prune_library
from models import ScanResult


def scan_library(
    path: str | Path,
    *,
    options: Optional[ScanOptions] = None,
    on_folder_done: Optional[Callable[[str], None]] = None,
) -> ScanResult:
    """Import the folders the scan plan picks under path, then prune moved/deleted tracks."""
    opts = options or ScanOptions()
    root = Path(path).expanduser().resolve()
    if not root.is_dir():
        raise FileNotFoundError(f"Music path does not exist: {root}")

    genre_version = genre_audio.MODEL_VERSION if opts.analyze_audio and genre_audio.get_model() else None
    with get_conn() as conn:
        started_at = db_now(conn)
        plan = plan_scan(conn, root, force=opts.force, analyze=opts.analyze_audio, genre_version=genre_version)

    total = ScanResult()
    for folder, files in plan.folders.items():
        total.merge(import_folder(folder, analyze=opts.analyze_audio, force=opts.force, files=files))
        if on_folder_done:
            on_folder_done(folder)

    if plan.missing or plan.folders:
        with get_conn() as conn:
            prune_library(conn, root, started_at)
    return total
