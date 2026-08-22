from typing import Callable, Optional, Protocol
from uuid import UUID

import psycopg

from db import upsert_scan_job_progress
from models import ScanResult


class ProgressReporter(Protocol):
    def on_scan_start(self, total_files: int) -> None: ...

    def on_file_done(self, path: str, index: int, cumulative: ScanResult) -> None: ...

    def on_scan_complete(self, result: ScanResult) -> None: ...


class NullProgress:
    def on_scan_start(self, total_files: int) -> None:
        pass

    def on_file_done(self, path: str, index: int, cumulative: ScanResult) -> None:
        pass

    def on_scan_complete(self, result: ScanResult) -> None:
        pass


class CallbackProgress:
    def __init__(self, callback: Optional[Callable[[str], None]] = None):
        self._callback = callback

    def on_scan_start(self, total_files: int) -> None:
        pass

    def on_file_done(self, path: str, index: int, cumulative: ScanResult) -> None:
        if self._callback:
            self._callback(path)

    def on_scan_complete(self, result: ScanResult) -> None:
        pass


class DbJobProgress:
    def __init__(
        self,
        conn: psycopg.Connection,
        job_id: UUID,
        master_job_id: Optional[UUID] = None,
    ):
        self._conn = conn
        self._job_id = job_id
        self._master_job_id = master_job_id

    def on_scan_start(self, total_files: int) -> None:
        pass

    def on_file_done(self, path: str, index: int, cumulative: ScanResult) -> None:
        upsert_scan_job_progress(
            self._conn, self._job_id, self._master_job_id, cumulative
        )

    def on_scan_complete(self, result: ScanResult) -> None:
        upsert_scan_job_progress(
            self._conn, self._job_id, self._master_job_id, result
        )
