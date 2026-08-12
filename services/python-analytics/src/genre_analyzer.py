import os
import re

from pathlib import Path
from typing import List, Optional

import numpy as np

from models import GenrePrediction
from tags import read_tags


try:
    import essentia
    import essentia.standard as es

    HAS_ESSENTIA = True
    essentia.log.warningActive = False
    essentia.log.infoActive = False
except Exception:
    HAS_ESSENTIA = False


MODEL_DIR = os.environ.get("GENRE_MODEL_DIR", "/app/models")
ENABLE_GENRE_ANALYSIS = os.environ.get("ENABLE_GENRE_ANALYSIS", "false").lower() == "true"
SNIPPET_START = float(os.environ.get("GENRE_SNIPPET_START", "30"))
SNIPPET_DURATION = float(os.environ.get("GENRE_SNIPPET_DURATION", "30"))
MIN_CONFIDENCE = float(os.environ.get("GENRE_MIN_CONFIDENCE", "0.3"))
FULL_TRACK_MODE = os.environ.get("GENRE_FULL_TRACK_MODE", "false").lower() == "true"

_EMBED_MODEL = None
_CLASS_MODEL = None
_LABELS = None
_MONO_LOADER = None


def _load_models():
    global _EMBED_MODEL, _CLASS_MODEL, _LABELS, _MONO_LOADER
    if not HAS_ESSENTIA or not ENABLE_GENRE_ANALYSIS:
        return None, None, None
    if _EMBED_MODEL is not None and _MONO_LOADER is not None:
        return _EMBED_MODEL, _CLASS_MODEL, _LABELS

    embed_path = os.path.join(MODEL_DIR, "discogs-effnet-bs64-1.pb")
    class_path = os.path.join(MODEL_DIR, "genre_discogs400-discogs-effnet-1.pb")
    labels_path = os.path.join(MODEL_DIR, "genre_discogs400-discogs-effnet-1.json")

    if not Path(embed_path).exists() or not Path(class_path).exists():
        return None, None, None

    _EMBED_MODEL = es.TensorflowPredictEffnetDiscogs(
        graphFilename=embed_path,
        output="PartitionedCall:1",
        patchHopSize=128,
    )
    _CLASS_MODEL = es.TensorflowPredict2D(
        graphFilename=class_path,
        input="serving_default_model_Placeholder",
        output="PartitionedCall:0",
    )
    _LABELS = _load_labels(labels_path)
    _MONO_LOADER = es.MonoLoader(sampleRate=16000, resampleQuality=4)
    return _EMBED_MODEL, _CLASS_MODEL, _LABELS


def _load_labels(path: str) -> List[str]:
    try:
        import json
        with open(path, "r") as f:
            data = json.load(f)
        if isinstance(data, list):
            return data
        if isinstance(data, dict):
            return data.get("classes", data.get("labels", []))
    except Exception:
        pass
    return []


def _parse_label(label: str) -> tuple:
    parts = re.split(r"\s*---\s*", label, maxsplit=1)
    if len(parts) == 2:
        return parts[0].strip(), parts[1].strip()
    return label.strip(), label.strip()


def _split_genre_tag(tag: str) -> List[str]:
    if not tag:
        return []
    return [g.strip() for g in re.split(r"[;/|]", tag) if g.strip()]


def analyze(path: str, snippet_start: Optional[float] = None, snippet_duration: Optional[float] = None) -> List[GenrePrediction]:
    """Predict genres for an audio file, falling back to file tags if models are unavailable."""
    if snippet_start is None:
        snippet_start = SNIPPET_START
    if snippet_duration is None:
        snippet_duration = SNIPPET_DURATION

    models = _load_models()
    if models[0] is None or _MONO_LOADER is None:
        return _fallback_from_tags(path)

    try:
        _MONO_LOADER.configure(filename=path)
        audio = _MONO_LOADER()
    except Exception:
        return _fallback_from_tags(path)

    try:
        duration = len(audio) / 16000.0
        if FULL_TRACK_MODE and duration > 0:
            snippet = audio
        else:
            start = min(snippet_start, duration - 1) if duration > snippet_start else 0.0
            end = min(start + snippet_duration, duration)
            start_idx = int(start * 16000)
            end_idx = int(end * 16000)
            snippet = audio[start_idx:end_idx]
            if len(snippet) == 0:
                snippet = audio

        embed_model, class_model, labels = models
        embeddings = embed_model(snippet)
        activations = class_model(embeddings)

        if isinstance(activations, (list, tuple)):
            activations = np.array(activations)

        if activations.ndim == 2:
            avg = activations.mean(axis=0)
        else:
            avg = activations

        avg = np.asarray(avg).flatten()

        if not labels and len(avg) > 0:
            labels = [f"class_{i}" for i in range(len(avg))]

        sorted_idx = np.argsort(avg)[::-1]
        predictions = []
        for idx in sorted_idx[:10]:
            if idx >= len(labels):
                continue
            label = labels[idx]
            main, sub = _parse_label(label)
            confidence = float(avg[idx])
            predictions.append(GenrePrediction(main_genre=main, sub_genre=sub, confidence=confidence, source="discogs400"))

        if not predictions:
            return _fallback_from_tags(path)
        return predictions
    except Exception as exc:
        print(f"[genre_analyzer] error analyzing {path}: {exc}")
        return _fallback_from_tags(path)


def _fallback_from_tags(path: str) -> List[GenrePrediction]:
    tags = read_tags(path)
    genre = tags.get("genre")
    if not genre:
        return []

    parts = _split_genre_tag(genre)
    if not parts:
        return []

    predictions = []
    confidence = 1.0 / max(len(parts), 1)
    for part in parts:
        main, sub = _parse_label(part)
        if not sub:
            sub = main
        predictions.append(GenrePrediction(main_genre=main, sub_genre=sub, confidence=confidence, source="file_tag"))
    return predictions
