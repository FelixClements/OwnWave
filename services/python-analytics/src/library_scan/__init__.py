"""Unified library scan entry point."""

from pathlib import Path
from typing import TYPE_CHECKING, Optional
from uuid import UUID

from folder_importer import import_folder
from library_scan.options import ScanJobContext, ScanOptions
from library_scan.progress import CallbackProgress, DbJobProgress, NullProgress, ProgressReporter
from models import ScanResult

if TYPE_CHECKING:
    import psycopg
from library_scan.options import ScanJobContext, ScanOptions
from library_scan.progress import CallbackProgress, DbJobProgress, NullProgress, ProgressReporter
from models import ScanResult


def scan_library(
    path: str | Path,
    *,
    options: Optional[ScanOptions] = None,
    progress: Optional[ProgressReporter] = None,
    job: Optional[ScanJobContext] = None,
    conn: Optional["psycopg.Connection"] = None,
) -> ScanResult:
    """Scan a path using the canonical import_folder pipeline."""
    opts = options or ScanOptions()
    path_str = str(Path(path).expanduser().resolve())

    job_id = job.job_id if job else None
    master_job_id = job.master_job_id if job else None

    if job_id and conn is not None and progress is None:
        progress = DbJobProgress(conn, job_id, master_job_id)
    elif progress is None:
        progress = NullProgress()

    callback = None
    if isinstance(progress, CallbackProgress):
        callback = progress._callback

    result = import_folder(
        path_str,
        job_id=job_id,
        master_job_id=master_job_id,
        analyze=opts.analyze_audio,
        force=opts.force,
    )

    if callback:
        progress.on_scan_complete(result)
    elif not isinstance(progress, NullProgress) and not isinstance(progress, DbJobProgress):
        progress.on_scan_complete(result)

    return result
