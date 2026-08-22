"""Discogs400 ML genre predictions — no tag fallback."""

import json
import os
import re
from pathlib import Path
from typing import List, Optional

import numpy as np

from config import ENABLE_GENRE_ANALYSIS, GENRE_MODEL_DIR, GENRE_MIN_CONFIDENCE
from models import GenrePrediction

try:
    import essentia
    import essentia.standard as es

    HAS_ESSENTIA = True
    essentia.log.warningActive = False
    essentia.log.infoActive = False
except Exception:
    HAS_ESSENTIA = False

SNIPPET_START = float(os.environ.get("GENRE_SNIPPET_START", "30"))
SNIPPET_DURATION = float(os.environ.get("GENRE_SNIPPET_DURATION", "30"))
FULL_TRACK_MODE = os.environ.get("GENRE_FULL_TRACK_MODE", "false").lower() == "true"


class Discogs400GenreSource:
    source_id = "discogs400"

    def __init__(self):
        self._embed_model = None
        self._class_model = None
        self._labels: List[str] = []
        self._loader = None

    @classmethod
    def from_config(cls) -> Optional["Discogs400GenreSource"]:
        if not ENABLE_GENRE_ANALYSIS or not HAS_ESSENTIA:
            return None
        embed_path = os.path.join(GENRE_MODEL_DIR, "discogs-effnet-bs64-1.pb")
        class_path = os.path.join(GENRE_MODEL_DIR, "genre_discogs400-discogs-effnet-1.pb")
        labels_path = os.path.join(GENRE_MODEL_DIR, "genre_discogs400-discogs-effnet-1.json")
        if not Path(embed_path).exists() or not Path(class_path).exists():
            return None
        source = cls()
        source._embed_model = es.TensorflowPredictEffnetDiscogs(
            graphFilename=embed_path,
            output="PartitionedCall:1",
            patchHopSize=128,
        )
        source._class_model = es.TensorflowPredict2D(
            graphFilename=class_path,
            input="serving_default_model_Placeholder",
            output="PartitionedCall:0",
        )
        source._labels = source._load_labels(labels_path)
        source._loader = es.MonoLoader(sampleRate=16000, resampleQuality=4)
        return source

    def predict(self, path: str) -> List[GenrePrediction]:
        if not self._embed_model or not self._loader:
            return []
        try:
            self._loader.configure(filename=path)
            audio = self._loader()
        except Exception:
            return []

        try:
            duration = len(audio) / 16000.0
            if FULL_TRACK_MODE and duration > 0:
                snippet = audio
            else:
                start = min(SNIPPET_START, duration - 1) if duration > SNIPPET_START else 0.0
                end = min(start + SNIPPET_DURATION, duration)
                start_idx = int(start * 16000)
                end_idx = int(end * 16000)
                snippet = audio[start_idx:end_idx] or audio

            embeddings = self._embed_model(snippet)
            activations = self._class_model(embeddings)
            if isinstance(activations, (list, tuple)):
                activations = np.array(activations)
            avg = np.asarray(activations.mean(axis=0) if activations.ndim == 2 else activations).flatten()

            labels = self._labels or [f"class_{i}" for i in range(len(avg))]
            predictions: List[GenrePrediction] = []
            for idx in np.argsort(avg)[::-1][:10]:
                if idx >= len(labels):
                    continue
                confidence = float(avg[idx])
                if confidence < GENRE_MIN_CONFIDENCE:
                    continue
                main, sub = self._parse_label(labels[idx])
                predictions.append(
                    GenrePrediction(
                        main_genre=main,
                        sub_genre=sub,
                        confidence=confidence,
                        source=self.source_id,
                    )
                )
            return predictions
        except Exception as exc:
            print(f"[Discogs400GenreSource] error analyzing {path}: {exc}")
            return []

    @staticmethod
    def _load_labels(path: str) -> List[str]:
        try:
            with open(path, "r") as f:
                data = json.load(f)
            if isinstance(data, list):
                return data
            if isinstance(data, dict):
                return data.get("classes", data.get("labels", []))
        except Exception:
            pass
        return []

    @staticmethod
    def _parse_label(label: str) -> tuple:
        parts = re.split(r"\s*---\s*", label, maxsplit=1)
        if len(parts) == 2:
            return parts[0].strip(), parts[1].strip()
        return label.strip(), label.strip()
