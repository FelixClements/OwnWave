from typing import List, Optional, Tuple
from uuid import UUID

import psycopg


def get_similar_tracks(conn: psycopg.Connection, track_id: UUID, limit: int = 20) -> List[Tuple[UUID, float]]:
    """Return (track_id, distance) for tracks most similar to track_id."""
    with conn.cursor() as cur:
        cur.execute(
            """
            SELECT af2.track_id, af1.feature_vector <=> af2.feature_vector AS dist
            FROM audio_features af1
            JOIN audio_features af2 ON af1.track_id != af2.track_id
            WHERE af1.track_id = %s AND af2.feature_vector IS NOT NULL
            ORDER BY af1.feature_vector <=> af2.feature_vector
            LIMIT %s
            """,
            (track_id, limit),
        )
        return [(row[0], float(row[1])) for row in cur.fetchall()]


def get_track_cluster(conn: psycopg.Connection, track_id: UUID) -> Optional[int]:
    with conn.cursor() as cur:
        cur.execute("SELECT cluster_id FROM track_clusters WHERE track_id = %s", (track_id,))
        row = cur.fetchone()
        return row[0] if row else None


def get_tracks_in_cluster(conn: psycopg.Connection, cluster_id: int) -> List[UUID]:
    with conn.cursor() as cur:
        cur.execute(
            """
            SELECT t.id
            FROM tracks t
            JOIN track_clusters tc ON t.id = tc.track_id
            WHERE tc.cluster_id = %s
            """,
            (cluster_id,),
        )
        return [row[0] for row in cur.fetchall()]
