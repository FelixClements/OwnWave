from dataclasses import dataclass
from typing import Optional, Protocol


@dataclass
class AudioFeatures:
    bpm: float
    key: str
    loudness: float
    energy: float
    valence: float
    outro_start_seconds: float
    ideal_crossfade_seconds: float
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
        chroma=pick(base.chroma, extra.chroma),
        spectral_centroid=pick(base.spectral_centroid, extra.spectral_centroid),
        mfcc=pick(base.mfcc, extra.mfcc),
        feature_vector=pick(base.feature_vector, extra.feature_vector),
    )
