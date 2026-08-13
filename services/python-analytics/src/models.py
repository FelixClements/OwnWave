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
class GenrePrediction:
    main_genre: str
    sub_genre: str
    confidence: float
    source: str


@dataclass
class AudioFeatures:
    bpm: float
    key: str
    loudness: float
    energy: float
    valence: float
    outro_start_seconds: float
    ideal_crossfade_seconds: float
    intro_start_seconds: Optional[float] = None
    outro_end_seconds: Optional[float] = None
    chroma: Optional[list] = None
    spectral_centroid: Optional[float] = None
    mfcc: Optional[list] = None
    feature_vector: Optional[list] = None


class Analyzer(Protocol):
    """Pluggable audio analyzer."""

    def analyze(self, path: str) -> AudioFeatures:
        ...


def merge_features(base: AudioFeatures, extra: Optional[AudioFeatures]) -> AudioFeatures:
    """Use extra (e.g. essentia) to fill any gaps, otherwise keep base."""
    if not extra:
        return base

    def pick(base_val, extra_val):
        return extra_val if extra_val is not None else base_val

    return AudioFeatures(
        bpm=pick(base.bpm, extra.bpm),
        key=pick(base.key, extra.key),
        loudness=pick(base.loudness, extra.loudness),
        energy=pick(base.energy, extra.energy),
        valence=pick(base.valence, extra.valence),
        outro_start_seconds=pick(base.outro_start_seconds, extra.outro_start_seconds),
        ideal_crossfade_seconds=pick(
            base.ideal_crossfade_seconds, extra.ideal_crossfade_seconds
        ),
        intro_start_seconds=pick(base.intro_start_seconds, extra.intro_start_seconds),
        outro_end_seconds=pick(base.outro_end_seconds, extra.outro_end_seconds),
        chroma=pick(base.chroma, extra.chroma),
        spectral_centroid=pick(base.spectral_centroid, extra.spectral_centroid),
        mfcc=pick(base.mfcc, extra.mfcc),
        feature_vector=pick(base.feature_vector, extra.feature_vector),
    )
