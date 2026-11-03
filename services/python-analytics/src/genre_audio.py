"""Discogs400 audio genre model (Essentia, EffNet-Discogs embeddings).

The model expects mono audio at 16 kHz. Each call builds a new MonoLoader
with every parameter set: Essentia's configure() resets any parameter it is
not given, so reconfiguring a loader with only a filename silently decodes
at 44.1 kHz and the model hears the track about 2.75x too slow.
"""

import json
import logging
import subprocess
from dataclasses import dataclass
from pathlib import Path
from typing import List, Optional

import numpy as np

log = logging.getLogger(__name__)

try:
    import essentia
    import essentia.standard as es

    HAS_ESSENTIA = True
    essentia.log.warningActive = False
    essentia.log.infoActive = False
except Exception:  # pragma: no cover - depends on the image
    HAS_ESSENTIA = False

SAMPLE_RATE = 16000
MODEL_VERSION = "discogs400-effnet-1/w3"

EMBED_FILE = "discogs-effnet-bs64-1.pb"
CLASS_FILE = "genre_discogs400-discogs-effnet-1.pb"
LABELS_FILE = "genre_discogs400-discogs-effnet-1.json"


@dataclass
class AudioGenreResult:
    activations: List[float]  # one per label, same order as labels
    embedding: List[float]  # mean EffNet embedding (1280)
    model_version: str


def analysis_windows(duration: float, window: float, positions=(0.2, 0.5, 0.75)) -> List[tuple]:
    """Start/end seconds of the windows to analyse.

    Windows sit at fixed fractions of the track so a long intro or outro
    does not decide the genre. Short tracks are analysed whole.
    """
    if duration <= 0:
        return []
    if duration <= window * len(positions):
        return [(0.0, duration)]
    out = []
    for p in positions:
        start = min(max(0.0, duration * p - window / 2), duration - window)
        out.append((start, start + window))
    return out


def load_labels(model_dir: str) -> Optional[List[str]]:
    """The model's label names (no TensorFlow needed), or None if missing."""
    try:
        return json.loads((Path(model_dir) / LABELS_FILE).read_text())["classes"]
    except (OSError, ValueError, KeyError):
        return None


def load_mono_16k(path: str) -> np.ndarray:
    """Mono 16 kHz audio. Falls back to ffmpeg for files Essentia rejects
    (e.g. more than 2 channels)."""
    try:
        return es.MonoLoader(filename=path, sampleRate=SAMPLE_RATE, resampleQuality=4)()
    except Exception:
        return _ffmpeg_mono_16k(path)


def _ffmpeg_mono_16k(path: str) -> np.ndarray:
    out = subprocess.run(
        ["ffmpeg", "-v", "error", "-i", path, "-ac", "1", "-ar", str(SAMPLE_RATE), "-f", "f32le", "-"],
        capture_output=True,
        check=True,
        timeout=300,
    ).stdout
    return np.frombuffer(out, dtype=np.float32)


class AudioGenreModel:
    def __init__(self, embed_model, class_model, labels: List[str], window_seconds: float):
        self._embed = embed_model
        self._classify = class_model
        self.labels = labels
        self.window_seconds = window_seconds

    @classmethod
    def load(cls, model_dir: str, window_seconds: float = 30.0) -> Optional["AudioGenreModel"]:
        """The model, or None when Essentia or the model files are missing."""
        if not HAS_ESSENTIA:
            return None
        d = Path(model_dir)
        if not all((d / f).exists() for f in (EMBED_FILE, CLASS_FILE, LABELS_FILE)):
            log.warning("genre model files missing in %s", model_dir)
            return None
        labels = load_labels(model_dir)
        embed = es.TensorflowPredictEffnetDiscogs(
            graphFilename=str(d / EMBED_FILE), output="PartitionedCall:1", patchHopSize=128
        )
        classify = es.TensorflowPredict2D(
            graphFilename=str(d / CLASS_FILE),
            input="serving_default_model_Placeholder",
            output="PartitionedCall:0",
        )
        return cls(embed, classify, labels, window_seconds)

    def analyze(self, path: str) -> Optional[AudioGenreResult]:
        try:
            audio = load_mono_16k(path)
        except Exception as exc:
            log.warning("cannot load %s: %s", path, exc)
            return None
        duration = len(audio) / SAMPLE_RATE
        embeddings = []
        for start, end in analysis_windows(duration, self.window_seconds):
            chunk = audio[int(start * SAMPLE_RATE) : int(end * SAMPLE_RATE)]
            if chunk.size < SAMPLE_RATE * 3:
                continue
            emb = np.asarray(self._embed(chunk))
            if emb.ndim == 2 and emb.shape[0] > 0:
                embeddings.append(emb)
        if not embeddings:
            return None
        emb = np.concatenate(embeddings, axis=0)
        acts = np.asarray(self._classify(emb))
        mean_acts = acts.mean(axis=0) if acts.ndim == 2 else acts.flatten()
        return AudioGenreResult(
            activations=[float(x) for x in mean_acts],
            embedding=[float(x) for x in emb.mean(axis=0)],
            model_version=MODEL_VERSION,
        )


_model: Optional[AudioGenreModel] = None
_loaded = False


def get_model() -> Optional[AudioGenreModel]:
    """The process-wide model, loaded on first use (None when disabled)."""
    global _model, _loaded
    if not _loaded:
        from config import ENABLE_GENRE_ANALYSIS, GENRE_MODEL_DIR, GENRE_WINDOW_SECONDS

        _loaded = True
        if ENABLE_GENRE_ANALYSIS:
            _model = AudioGenreModel.load(GENRE_MODEL_DIR, GENRE_WINDOW_SECONDS)
    return _model
