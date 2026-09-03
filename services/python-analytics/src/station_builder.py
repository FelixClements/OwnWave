from typing import List, Optional
from uuid import UUID

import psycopg

from config import GENRE_MIN_CONFIDENCE, GENRE_MIN_TRACKS_PER_STATION
from db import (
    create_station,
    delete_orphaned_auto_stations,
    get_station_track_ids,
    get_tracks_by_genre,
    get_uncovered_tracks,
    insert_station_tracks,
    list_genres,
    list_user_ids,
    upsert_auto_station,
)
from station.compiler import compile_station_queue
from station.seed import parse_seed


def _title_case_name(name: str) -> str:
    return " ".join(part.capitalize() for part in name.split(" "))


def build_station(
    conn: psycopg.Connection,
    name: str,
    user_id: UUID,
    seed_filter: Optional[dict] = None,
    length: int = 50,
) -> UUID:
    """Build a smart station queue from a seed (filter, track, artist, album, mood, or cluster)."""
    parsed = parse_seed(seed_filter)
    track_ids = compile_station_queue(conn, parsed, length, user_id)
    station_id = create_station(conn, name, user_id, seed_features=parsed)
    insert_station_tracks(conn, station_id, track_ids)
    conn.commit()
    return station_id


def rebuild_genre_stations(
    conn: psycopg.Connection,
    user_id: UUID,
    min_confidence: float = None,
    min_tracks: int = None,
    station_length: int = 50,
) -> dict:
    if min_confidence is None:
        min_confidence = GENRE_MIN_CONFIDENCE
    if min_tracks is None:
        min_tracks = GENRE_MIN_TRACKS_PER_STATION

    genres = list_genres(conn, min_confidence=min_confidence)
    main_counts = {}
    for g in genres:
        main_counts.setdefault(g["main_genre"], {"total": 0, "subs": []})
        main_counts[g["main_genre"]]["total"] += g["track_count"]
        main_counts[g["main_genre"]]["subs"].append(g)

    valid_names = set()
    created = 0
    for main_genre, info in main_counts.items():
        if info["total"] < min_tracks:
            continue
        name = _title_case_name(main_genre)
        valid_names.add(name)
        seed = {"type": "genre", "main_genre": main_genre}
        upsert_auto_station(conn, name, user_id, seed, source="genre")
        _build_station_from_seed(conn, user_id, name, seed, station_length)
        created += 1

    for g in genres:
        if g["track_count"] < min_tracks or g["main_genre"] == g["sub_genre"]:
            continue
        name = f"{_title_case_name(g['main_genre'])} / {_title_case_name(g['sub_genre'])}"
        valid_names.add(name)
        seed = {"type": "sub_genre", "main_genre": g["main_genre"], "sub_genre": g["sub_genre"]}
        upsert_auto_station(conn, name, user_id, seed, source="genre")
        _build_station_from_seed(conn, user_id, name, seed, station_length)
        created += 1

    removed = delete_orphaned_auto_stations(conn, user_id, valid_names)
    conn.commit()
    return {"created_or_refreshed": created, "removed": removed}


def rebuild_genre_stations_for_users(
    conn: psycopg.Connection,
    user_id: Optional[UUID] = None,
) -> dict:
    if user_id is not None:
        return rebuild_genre_stations(conn, user_id)
    results = []
    for uid in list_user_ids(conn):
        results.append(rebuild_genre_stations(conn, uid))
    return {"users": len(results), "results": results}


def _build_station_from_seed(
    conn: psycopg.Connection,
    user_id: UUID,
    name: str,
    seed: dict,
    length: int,
) -> None:
    try:
        parsed = parse_seed(seed)
        track_ids = compile_station_queue(conn, parsed, length, user_id)
        with conn.cursor() as cur:
            cur.execute(
                "SELECT id FROM stations WHERE user_id = %s AND name = %s",
                (user_id, name),
            )
            row = cur.fetchone()
        if not row:
            return
        station_id = row[0]
        with conn.cursor() as cur:
            cur.execute("DELETE FROM station_tracks WHERE station_id = %s", (station_id,))
        insert_station_tracks(conn, station_id, track_ids)
    except Exception as exc:
        print(f"[rebuild_genre_stations] error building {name}: {exc}")


def setup_main_genre_stations(
    conn: psycopg.Connection,
    user_id: UUID,
    selected_main_genres: List[str],
    station_length: int = 50,
) -> dict:
    created = 0
    created_ids = []
    all_covered = set()

    for main_genre in selected_main_genres:
        track_ids = set(get_tracks_by_genre(conn, main_genre, min_confidence=GENRE_MIN_CONFIDENCE))
        if not track_ids:
            continue

        name = _title_case_name(main_genre)
        seed = {"type": "genre", "main_genre": main_genre}

        try:
            station_id = upsert_auto_station(conn, name, user_id, seed, source="setup")
            if len(track_ids) >= 2:
                _build_station_from_seed(conn, user_id, name, seed, station_length)
            else:
                with conn.cursor() as cur:
                    cur.execute("DELETE FROM station_tracks WHERE station_id = %s", (station_id,))
                insert_station_tracks(conn, station_id, list(track_ids))
            conn.commit()
            created += 1
            created_ids.append(str(station_id))
            all_covered |= set(get_station_track_ids(conn, station_id))
        except Exception as exc:
            print(f"[setup_main_genre_stations] error building {name}: {exc}")

    uncovered = [t for t in get_uncovered_tracks(conn) if t not in all_covered]
    if uncovered:
        uncategorized_name = "Uncategorized"
        uncategorized_id = upsert_auto_station(
            conn,
            uncategorized_name,
            user_id,
            seed_features={"type": "uncategorized"},
            source="setup",
        )
        with conn.cursor() as cur:
            cur.execute("DELETE FROM station_tracks WHERE station_id = %s", (uncategorized_id,))
        insert_station_tracks(conn, uncategorized_id, uncovered)
        conn.commit()
        created += 1
        created_ids.append(str(uncategorized_id))

    return {"created": created, "station_ids": created_ids, "uncovered": len(uncovered)}
