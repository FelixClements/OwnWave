import math
import random
from typing import List, Optional
from uuid import UUID

import psycopg

from db import create_station, get_all_tracks_with_features, insert_station_tracks


def build_station(
    conn: psycopg.Connection,
    name: str,
    seed_filter: Optional[dict] = None,
    length: int = 50,
) -> UUID:
    """Build a station queue by greedily walking the track feature space."""
    tracks = get_all_tracks_with_features(conn)
    if not tracks:
        raise ValueError("No analyzed tracks found")

    if seed_filter:
        tracks = _apply_filter(tracks, seed_filter)

    if len(tracks) < 2:
        raise ValueError("Not enough tracks match the filter")

    sequence = _greedy_queue(tracks, length)
    track_ids = [t["id"] for t in sequence]

    station_id = create_station(conn, name, seed_features=seed_filter)
    insert_station_tracks(conn, station_id, track_ids)
    conn.commit()
    return station_id


def _apply_filter(tracks: List[dict], filters: dict) -> List[dict]:
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


def _greedy_queue(tracks: List[dict], length: int) -> List[dict]:
    # Sort by energy to get a sensible starting point, then pick closest neighbours.
    pool = sorted(tracks, key=lambda t: (t["energy"] or 0, t["valence"] or 0))
    current = pool[0]
    queue = [current]
    remaining = pool[1:]

    while len(queue) < length and remaining:
        next_track = min(remaining, key=lambda t: _distance(current, t))
        queue.append(next_track)
        remaining.remove(next_track)
        current = next_track

    return queue


def _distance(a: dict, b: dict) -> float:
    bpm_a = a.get("bpm") or 120.0
    bpm_b = b.get("bpm") or 120.0
    energy_a = a.get("energy") or 0.5
    energy_b = b.get("energy") or 0.5
    valence_a = a.get("valence") or 0.5
    valence_b = b.get("valence") or 0.5

    # Tempo is relative; energy/valence are 0-1.
    bpm_diff = abs(bpm_a - bpm_b) / max((bpm_a + bpm_b) / 2, 1.0)
    energy_diff = abs(energy_a - energy_b)
    valence_diff = abs(valence_a - valence_b)

    return math.sqrt(bpm_diff ** 2 + energy_diff ** 2 + valence_diff ** 2)
