from typing import List
from uuid import UUID

import psycopg

from config import GENRE_MIN_CONFIDENCE
from db import get_tracks_by_genre
from similarity import get_similar_tracks


def exclude_banned(tracks: List[dict]) -> List[dict]:
    return [t for t in tracks if "ban" not in (t.get("feedback") or [])]


def apply_numeric_filter(tracks: List[dict], filters: dict) -> List[dict]:
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
        if "min_valence" in filters and (t["valence"] or 0) < filters["min_valence"]:
            ok = False
        if "max_valence" in filters and (t["valence"] or 0) > filters["max_valence"]:
            ok = False
        if ok:
            out.append(t)
    return out


def apply_seed_filter(conn: psycopg.Connection, tracks: List[dict], filters: dict) -> List[dict]:
    seed_type = filters.get("type") or filters.get("seed_type")
    if seed_type == "track":
        target = UUID(filters["track_id"])
        similar = get_similar_tracks(conn, target, limit=200)
        similar_ids = {target} | {
            (tid if isinstance(tid, UUID) else UUID(tid)) for tid, _ in similar
        }
        tracks = [t for t in tracks if t["id"] in similar_ids]
    elif seed_type == "artist":
        artist_id = UUID(filters["artist_id"])
        tracks = [t for t in tracks if t["artist_id"] == artist_id]
    elif seed_type == "album":
        album_id = UUID(filters["album_id"])
        tracks = [t for t in tracks if t["album_id"] == album_id]
    elif seed_type == "cluster":
        cluster_id = int(filters["cluster_id"])
        tracks = [t for t in tracks if t.get("cluster_id") == cluster_id]
    elif seed_type == "mood":
        min_energy = filters.get("min_energy", 0.0)
        max_energy = filters.get("max_energy", 1.0)
        min_valence = filters.get("min_valence", 0.0)
        max_valence = filters.get("max_valence", 1.0)
        tracks = [
            t
            for t in tracks
            if min_energy <= (t["energy"] or 0) <= max_energy
            and min_valence <= (t["valence"] or 0) <= max_valence
        ]
    elif seed_type == "genre":
        main_genre = filters.get("main_genre")
        if not main_genre:
            return []
        genre_track_ids = set(
            get_tracks_by_genre(conn, main_genre, min_confidence=GENRE_MIN_CONFIDENCE)
        )
        tracks = [t for t in tracks if t["id"] in genre_track_ids]
    elif seed_type == "sub_genre":
        main_genre = filters.get("main_genre")
        sub_genre = filters.get("sub_genre")
        if not main_genre or not sub_genre:
            return []
        genre_track_ids = set(
            get_tracks_by_genre(
                conn, main_genre, sub_genre, min_confidence=GENRE_MIN_CONFIDENCE
            )
        )
        tracks = [t for t in tracks if t["id"] in genre_track_ids]
    elif seed_type == "uncategorized":
        tracks = tracks
    elif seed_type:
        return []

    return apply_numeric_filter(tracks, filters)
