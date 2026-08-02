import json
import math
from typing import Any, Dict, List, Optional, Tuple

import numpy as np
import psycopg
from sklearn.preprocessing import StandardScaler

from db import get_conn
from models import AudioFeatures


DIM = 33


def _parse_key(key: Optional[str]) -> Tuple[float, float, float]:
    """Return (sin, cos, is_minor) for a key string like 'C major' or 'A minor'."""
    if not key:
        return 0.0, 1.0, 0.0

    names = ["C", "C#", "D", "D#", "E", "F", "F#", "G", "G#", "A", "A#", "B"]
    idx = -1
    for i, name in enumerate(names):
        if key.startswith(name):
            idx = i
            break
    if idx < 0:
        return 0.0, 1.0, 0.0

    angle = 2 * math.pi * idx / 12
    is_minor = 1.0 if "minor" in key.lower() else 0.0
    return math.sin(angle), math.cos(angle), is_minor


def build_raw_feature_vector(features: AudioFeatures) -> List[float]:
    """Build a raw 31-dimensional feature vector from AudioFeatures."""
    bpm = float(features.bpm) if features.bpm is not None else 120.0
    loudness = float(features.loudness) if features.loudness is not None else -14.0
    energy = float(features.energy) if features.energy is not None else 0.5
    valence = float(features.valence) if features.valence is not None else 0.5
    spectral = float(features.spectral_centroid) if features.spectral_centroid is not None else 5512.5
    chroma = list(features.chroma) if features.chroma else [1.0 / 12] * 12
    mfcc = list(features.mfcc) if features.mfcc else [0.0] * 13

    key_sin, key_cos, is_minor = _parse_key(features.key)

    return [
        math.log1p(bpm) / math.log1p(300),
        key_sin,
        key_cos,
        is_minor,
        (loudness + 30.0) / 30.0,
        energy,
        valence,
        spectral / 11025.0,
        *chroma[:12],
        *mfcc[:13],
    ]


def get_feature_stats(conn: psycopg.Connection) -> Optional[Tuple[List[float], List[float]]]:
    with conn.cursor() as cur:
        cur.execute(
            "SELECT data FROM feature_stats WHERE name = 'standard_scaler'"
        )
        row = cur.fetchone()
        if not row:
            return None
        data = row[0]
        return data.get("mean"), data.get("scale")


def set_feature_stats(conn: psycopg.Connection, mean: List[float], scale: List[float]) -> None:
    with conn.cursor() as cur:
        cur.execute(
            """
            INSERT INTO feature_stats (name, data, updated_at)
            VALUES ('standard_scaler', %s, NOW())
            ON CONFLICT (name) DO UPDATE SET
                data = EXCLUDED.data,
                updated_at = NOW()
            """,
            (json.dumps({"mean": mean, "scale": scale}),),
        )


def normalize_feature_vector(raw: List[float], mean: List[float], scale: List[float]) -> List[float]:
    out = []
    for i, v in enumerate(raw[:DIM]):
        s = scale[i] if i < len(scale) and scale[i] != 0 else 1.0
        m = mean[i] if i < len(mean) else 0.0
        out.append(float((v - m) / s))
    while len(out) < DIM:
        out.append(0.0)
    return out


def build_feature_vector(features: AudioFeatures, conn: Optional[psycopg.Connection] = None) -> List[float]:
    """Build a normalized feature vector, using stored StandardScaler stats if available."""
    raw = np.array(build_raw_feature_vector(features))
    if conn is not None:
        stats = get_feature_stats(conn)
        if stats is not None:
            mean, scale = stats
            return normalize_feature_vector(raw.tolist(), mean, scale)
    return raw.tolist()


def rebuild_feature_vectors(conn: psycopg.Connection) -> int:
    """Recompute normalized feature vectors for all tracks with audio_features."""
    with conn.cursor() as cur:
        cur.execute(
            """
            SELECT t.id, af.bpm, af.key, af.loudness, af.energy, af.valence,
                   af.spectral_centroid, af.chroma, af.mfcc
            FROM tracks t
            JOIN audio_features af ON t.id = af.track_id
            """
        )
        rows = cur.fetchall()

    raw_vectors = []
    track_ids = []
    for row in rows:
        features = AudioFeatures(
            bpm=row[1],
            key=row[2],
            loudness=row[3],
            energy=row[4],
            valence=row[5],
            outro_start_seconds=0.0,
            ideal_crossfade_seconds=0.0,
            intro_start_seconds=0.0,
            outro_end_seconds=0.0,
            spectral_centroid=row[6],
            chroma=row[7],
            mfcc=row[8],
        )
        raw = build_raw_feature_vector(features)
        raw_vectors.append(raw)
        track_ids.append(row[0])

    if not raw_vectors:
        return 0

    matrix = np.array(raw_vectors)
    scaler = StandardScaler()
    normalized = scaler.fit_transform(matrix)

    with conn.cursor() as cur:
        for tid, vec in zip(track_ids, normalized.tolist()):
            cur.execute(
                "UPDATE audio_features SET feature_vector = %s WHERE track_id = %s",
                (vec, tid),
            )

    set_feature_stats(conn, scaler.mean_.tolist(), scaler.scale_.tolist())
    return len(track_ids)


def backfill_library_feature_vectors() -> int:
    """Backfill feature vectors for the whole library."""
    with get_conn() as conn:
        count = rebuild_feature_vectors(conn)
        conn.commit()
        return count
