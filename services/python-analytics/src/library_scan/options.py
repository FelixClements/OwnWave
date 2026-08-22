from dataclasses import dataclass
from typing import List, Optional
from uuid import UUID

from models import ScanResult


@dataclass(frozen=True)
class ScanOptions:
    force: bool = False
    analyze_audio: bool = True
    analyze_genre: bool = True
    prune_deleted: bool = True


@dataclass(frozen=True)
class ScanJobContext:
    job_id: UUID
    master_job_id: Optional[UUID] = None
