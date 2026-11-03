from dataclasses import dataclass, field
from typing import List, Optional, Protocol


@dataclass
class FailedPath:
    path: str
    error: str


@dataclass
class ScanResult:
    total_files: int = 0
    imported: int = 0
    scanned: int = 0
    features: int = 0
    model_success: int = 0
    model_failed: int = 0
    failed_paths: List[FailedPath] = field(default_factory=list)
    track_ids: List = field(default_factory=list)

    def merge(self, other: "ScanResult") -> None:
        self.total_files += other.total_files
        self.imported += other.imported
        self.scanned += other.scanned
        self.features += other.features
        self.model_success += other.model_success
        self.model_failed += other.model_failed
        self.failed_paths.extend(other.failed_paths)
        self.track_ids.extend(other.track_ids)

    def to_dict(self):
        return {
            "total_files": self.total_files,
            "imported": self.imported,
            "scanned": self.scanned,
            "features": self.features,
            "model_success": self.model_success,
            "model_failed": self.model_failed,
            "failed_paths": [{"path": f.path, "error": f.error} for f in self.failed_paths],
        }


@dataclass
class AudioFeatures:
    """Per-track audio features. Only loudness is measured now; the other
    columns stay for clients that read them and are empty."""

    loudness: Optional[float] = None
    bpm: Optional[float] = None
    key: Optional[str] = None
    energy: Optional[float] = None
    valence: Optional[float] = None
    outro_start_seconds: Optional[float] = None
    ideal_crossfade_seconds: Optional[float] = None
    intro_start_seconds: Optional[float] = None
    outro_end_seconds: Optional[float] = None
    chroma: Optional[list] = None
    spectral_centroid: Optional[float] = None
    mfcc: Optional[list] = None


class Analyzer(Protocol):
    """Pluggable audio analyzer."""

    def analyze(self, path: str) -> AudioFeatures:
        ...
