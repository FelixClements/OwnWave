import math
import random
from typing import List, Optional
from uuid import UUID

import psycopg

from db import create_station, get_all_tracks_with_features, insert_station_tracks
from similarity import get_similar_tracks


SEED_TYPES = {"track", "artist", "album", "mood", "cluster"}


def build_station(
    conn: psycopg.Connection,
    name: str,
    seed_filter: Optional[dict] = None,
    length: int = 50,
) -> UUID:
    """Build a smart station queue from a seed (filter, track, artist, album, mood, or cluster)."""
    tracks = get_all_tracks_with_features(conn)
    if not tracks:
        raise ValueError("No analyzed tracks found")

    if seed_filter:
        tracks = _apply_filter(conn, tracks, seed_filter)

    tracks = _exclude_banned(tracks)

    if len(tracks) < 2:
        raise ValueError("Not enough tracks match the seed")

    seed_track_id = None
    if seed_filter and seed_filter.get("type") == "track":
        seed_track_id = UUID(seed_filter["track_id"])

    sequence = _smart_queue(conn, tracks, length, seed_id=seed_track_id)
    track_ids = [t["id"] for t in sequence]

    station_id = create_station(conn, name, seed_features=seed_filter)
    insert_station_tracks(conn, station_id, track_ids)
    conn.commit()
    return station_id


def _exclude_banned(tracks: List[dict]) -> List[dict]:
    return [t for t in tracks if "ban" not in (t.get("feedback") or [])]


def _apply_filter(conn: psycopg.Connection, tracks: List[dict], filters: dict) -> List[dict]:
    seed_type = filters.get("type")
    if seed_type == "track":
        target = UUID(filters["track_id"])
        similar = get_similar_tracks(conn, target, limit=200)
        similar_ids = {target} | {(tid if isinstance(tid, UUID) else UUID(tid)) for tid, _ in similar}
        return [t for t in tracks if t["id"] in similar_ids]
    if seed_type == "artist":
        artist_id = UUID(filters["artist_id"])
        return [t for t in tracks if t["artist_id"] == artist_id]
    if seed_type == "album":
        album_id = UUID(filters["album_id"])
        return [t for t in tracks if t["album_id"] == album_id]
    if seed_type == "cluster":
        cluster_id = int(filters["cluster_id"])
        return [t for t in tracks if t.get("cluster_id") == cluster_id]
    if seed_type == "mood":
        min_energy = filters.get("min_energy", 0.0)
        max_energy = filters.get("max_energy", 1.0)
        min_valence = filters.get("min_valence", 0.0)
        max_valence = filters.get("max_valence", 1.0)
        return [
            t
            for t in tracks
            if min_energy <= (t["energy"] or 0) <= max_energy
            and min_valence <= (t["valence"] or 0) <= max_valence
        ]

    # Legacy numeric filters
    out = []
    for t in tracks:
        ok = True
        if "min_bpm" in filters and (t["bpm"] or 0) < filters["min_bpm"]:
            ok = False
        if "max_bpm" in filters and (t["bpm"] or 0) > filters["max_bpm"]:
            ok = False
        if "min_energy" in filters and (t["energy"] or 0) < filters["min_energy"]:
            ok = False
        if "max_energy" in filters and (t["energy"] or 0) > filters["max_energy"]:
            ok = False
        if ok:
            out.append(t)
    return out


def _smart_queue(
    conn: psycopg.Connection,
    tracks: List[dict],
    length: int,
    novelty: float = 0.05,
    seed_id: Optional[UUID] = None,
) -> List[dict]:
    """Greedy k-NN walk that occasionally jumps for novelty."""
    # Start from the seed track if provided and in the pool, otherwise a liked/random track
    current = None
    if seed_id is not None:
        current = next((t for t in tracks if t["id"] == seed_id), None)
    if current is None:
        liked = [t for t in tracks if "like" in (t.get("feedback") or [])]
        current = random.choice(liked) if liked else random.choice(tracks)
    queue = [current]
    remaining = [t for t in tracks if t["id"] != current["id"]]

    # Pre-compute k-NN cache for the current track set
    track_ids_in_pool = {t["id"] for t in tracks}
    sim_cache = {}

    if current.get("feature_vector") is not None:
        sim_cache[current["id"]] = get_similar_tracks(conn, current["id"], limit=min(50, len(tracks)))

    while len(queue) < length and remaining:
        if random.random() < novelty and remaining:
            # Novelty jump: pick a track from a different cluster if possible
            current_cluster = current.get("cluster_id")
            other_clusters = [t for t in remaining if t.get("cluster_id") != current_cluster]
            if other_clusters:
                next_track = random.choice(other_clusters)
            else:
                next_track = random.choice(remaining)
        else:
            next_track = _nearest_remaining(current, remaining, conn, sim_cache)

        queue.append(next_track)
        remaining.remove(next_track)
        current = next_track

    return queue


def _nearest_remaining(
    current: dict,
    remaining: List[dict],
    conn: psycopg.Connection,
    sim_cache: dict,
) -> dict:
    """Pick the closest track using pgvector distance when possible."""
    current_id = current["id"]
    current_vec = current.get("feature_vector")

    if current_vec is not None:
        if current_id not in sim_cache:
            sim_cache[current_id] = get_similar_tracks(conn, current_id, limit=50)
        sims = {tid: dist for tid, dist in sim_cache[current_id]}

        def _score(t: dict) -> float:
            base = sims.get(t["id"], float("inf"))
            smooth = _distance(current, t)
            key_pen = _key_penalty(current.get("key"), t.get("key"))
            feedback = t.get("feedback") or []
            if "like" in feedback:
                return base + 0.3 * smooth + key_pen - 0.05
            if "skip" in feedback:
                return base + 0.3 * smooth + key_pen + 0.15
            return base + 0.3 * smooth + key_pen

        best = min(remaining, key=_score)
        return best

    # Fallback to the old BPM/energy/valence distance
    return min(remaining, key=lambda t: _distance(current, t))


def _distance(a: dict, b: dict) -> float:
    bpm_a = a.get("bpm") or 120.0
    bpm_b = b.get("bpm") or 120.0
    energy_a = a.get("energy") or 0.5
    energy_b = b.get("energy") or 0.5
    valence_a = a.get("valence") or 0.5
    valence_b = b.get("valence") or 0.5

    bpm_diff = abs(bpm_a - bpm_b) / max((bpm_a + bpm_b) / 2, 1.0)
    energy_diff = abs(energy_a - energy_b)
    valence_diff = abs(valence_a - valence_b)

    return math.sqrt(bpm_diff ** 2 + energy_diff ** 2 + valence_diff ** 2)


def _key_penalty(a: Optional[str], b: Optional[str]) -> float:
    if not a or not b:
        return 0.2
    if a == b:
        return 0.0
    return 0.6
