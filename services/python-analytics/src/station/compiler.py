import math
import random
from typing import Callable, List, Optional, Tuple
from uuid import UUID

import psycopg

from db import get_all_tracks_with_features
from similarity import get_similar_tracks
from station.filter import apply_seed_filter, exclude_banned
from station.seed import parse_seed, track_seed_id

SimilarFn = Callable[[UUID, int], List[Tuple[UUID, float]]]


def compile_station_queue(
    conn: psycopg.Connection,
    seed: Optional[dict],
    length: int,
    user_id: UUID,
) -> List[UUID]:
    """Build an ordered track list for a station from a seed filter."""
    tracks = get_all_tracks_with_features(conn, user_id)
    if not tracks:
        raise ValueError("No analyzed tracks found")

    parsed = parse_seed(seed)
    if parsed:
        tracks = apply_seed_filter(conn, tracks, parsed)

    tracks = exclude_banned(tracks)
    if len(tracks) < 2:
        raise ValueError("Not enough tracks match the seed")

    sequence = smart_queue(conn, tracks, length, seed_id=track_seed_id(parsed))
    return [t["id"] for t in sequence]


def smart_queue(
    conn: psycopg.Connection,
    tracks: List[dict],
    length: int,
    novelty: float = 0.05,
    seed_id: Optional[UUID] = None,
    get_similar: Optional[SimilarFn] = None,
) -> List[dict]:
    """Greedy k-NN walk that occasionally jumps for novelty."""
    similar_fn = get_similar or (lambda track_id, limit: get_similar_tracks(conn, track_id, limit=limit))

    current = None
    if seed_id is not None:
        current = next((t for t in tracks if t["id"] == seed_id), None)
    if current is None:
        liked = [t for t in tracks if "like" in (t.get("feedback") or [])]
        current = random.choice(liked) if liked else random.choice(tracks)

    queue = [current]
    remaining = [t for t in tracks if t["id"] != current["id"]]
    sim_cache: dict = {}

    if current.get("feature_vector") is not None:
        sim_cache[current["id"]] = similar_fn(current["id"], min(50, len(tracks)))

    while len(queue) < length and remaining:
        if random.random() < novelty and remaining:
            current_cluster = current.get("cluster_id")
            other_clusters = [t for t in remaining if t.get("cluster_id") != current_cluster]
            next_track = random.choice(other_clusters) if other_clusters else random.choice(remaining)
        else:
            next_track = _nearest_remaining(current, remaining, similar_fn, sim_cache)

        queue.append(next_track)
        remaining.remove(next_track)
        current = next_track

    return queue


def _nearest_remaining(
    current: dict,
    remaining: List[dict],
    similar_fn: SimilarFn,
    sim_cache: dict,
) -> dict:
    current_id = current["id"]
    current_vec = current.get("feature_vector")

    if current_vec is not None:
        if current_id not in sim_cache:
            sim_cache[current_id] = similar_fn(current_id, 50)
        sims = {tid: dist for tid, dist in sim_cache[current_id]}

        def _score(t: dict) -> float:
            base = sims.get(t["id"], float("inf"))
            smooth = distance(current, t)
            key_pen = key_penalty(current.get("key"), t.get("key"))
            feedback = t.get("feedback") or []
            if "like" in feedback:
                return base + 0.3 * smooth + key_pen - 0.05
            if "skip" in feedback:
                return base + 0.3 * smooth + key_pen + 0.15
            return base + 0.3 * smooth + key_pen

        return min(remaining, key=_score)

    return min(remaining, key=lambda t: distance(current, t))


def distance(a: dict, b: dict) -> float:
    bpm_a = a.get("bpm") or 120.0
    bpm_b = b.get("bpm") or 120.0
    energy_a = a.get("energy") or 0.5
    energy_b = b.get("energy") or 0.5
    valence_a = a.get("valence") or 0.5
    valence_b = b.get("valence") or 0.5

    bpm_diff = abs(bpm_a - bpm_b) / max((bpm_a + bpm_b) / 2, 1.0)
    energy_diff = abs(energy_a - energy_b)
    valence_diff = abs(valence_a - valence_b)

    return math.sqrt(bpm_diff**2 + energy_diff**2 + valence_diff**2)


def key_penalty(a: Optional[str], b: Optional[str]) -> float:
    if not a or not b:
        return 0.2
    if a == b:
        return 0.0
    return 0.6
