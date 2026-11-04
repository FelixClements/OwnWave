"""Integrated loudness (EBU R128 / LUFS) of a track.

Loudness is the one audio feature the app uses: the stream server evens
out volume with it. The earlier librosa and Essentia feature analyzers took
about 9 s per track for values nothing reads (BPM, key, energy, MFCC…).
"""

import logging
import subprocess
from typing import Tuple

import numpy as np
import pyloudnorm
import soundfile as sf

from models import AudioFeatures

log = logging.getLogger(__name__)

FFMPEG_RATE = 48000


def _decode(path: str) -> Tuple[np.ndarray, int]:
    """(frames, sample rate), via libsndfile or, for anything it rejects, ffmpeg."""
    try:
        data, rate = sf.read(path, dtype="float32", always_2d=True)
        return data, rate
    except Exception:
        out = subprocess.run(
            ["ffmpeg", "-v", "error", "-i", path, "-ac", "2", "-ar", str(FFMPEG_RATE), "-f", "f32le", "-"],
            capture_output=True,
            check=True,
            timeout=300,
        ).stdout
        return np.frombuffer(out, dtype=np.float32).reshape(-1, 2), FFMPEG_RATE


def integrated_loudness(data: np.ndarray, rate: int) -> float | None:
    """LUFS, or None for silence and clips too short to measure (< 0.4 s)."""
    if data.shape[0] < int(0.4 * rate):
        return None
    lufs = pyloudnorm.Meter(rate).integrated_loudness(data)
    return round(float(lufs), 2) if np.isfinite(lufs) else None


class LoudnessAnalyzer:
    def analyze(self, path: str) -> AudioFeatures:
        data, rate = _decode(path)
        return AudioFeatures(loudness=integrated_loudness(data, rate))
