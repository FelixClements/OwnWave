from typing import Optional
from uuid import UUID

import psycopg
from psycopg.types.json import Jsonb

from db import get_station_by_id, insert_station_tracks
from station.compiler import compile_station_queue
from station.seed import parse_seed


def recompile_station(
    conn: psycopg.Connection,
    station_id: UUID,
    *,
    name: Optional[str] = None,
    seed: Optional[dict] = None,
    length: int = 50,
) -> dict:
    """Update station metadata and rebuild its track queue from the seed."""
    existing = get_station_by_id(conn, station_id)
    if not existing:
        raise ValueError("station not found")

    effective_name = name or existing["name"]
    if seed is not None:
        effective_seed = parse_seed(seed)
    else:
        effective_seed = parse_seed(existing.get("seed_features"))

    track_ids = compile_station_queue(conn, effective_seed, length)

    with conn.cursor() as cur:
        cur.execute(
            """
            UPDATE stations
            SET name = %s,
                seed_features = %s::jsonb
            WHERE id = %s
            """,
            (effective_name, Jsonb(effective_seed) if effective_seed else None, station_id),
        )
        cur.execute("DELETE FROM station_tracks WHERE station_id = %s", (station_id,))

    insert_station_tracks(conn, station_id, track_ids)
    conn.commit()

    return {
        "station_id": str(station_id),
        "name": effective_name,
        "track_count": len(track_ids),
    }
