import math
from typing import List, Optional, Tuple

import numpy as np
import psycopg
from sklearn.cluster import KMeans


def _count_items(cur, sql, args=()):
    cur.execute(sql, args)
    return cur.fetchone()[0]


def _load_vectors(cur, min_cluster_size: int = 3) -> List[Tuple[str, List[float]]]:
    cur.execute(
        """
        SELECT track_id, feature_vector
        FROM audio_features
        WHERE feature_vector IS NOT NULL
        ORDER BY track_id
        """
    )
    rows = cur.fetchall()
    return [(str(r[0]), r[1].to_list()) for r in rows]


def _choose_k(n: int, max_k: int = 20) -> int:
    if n < 10:
        return max(2, n // 2)
    return min(max_k, max(2, int(math.sqrt(n / 2))))


def rebuild_clusters(conn: psycopg.Connection, n_clusters: Optional[int] = None) -> dict:
    """Run K-Means over feature vectors and store the cluster assignment."""
    with conn.cursor() as cur:
        rows = _load_vectors(cur)

    if not rows:
        return {"kmeans_clusters": 0, "tracks": 0}

    track_ids = [r[0] for r in rows]
    vectors = np.array([r[1] for r in rows])
    n = len(track_ids)

    k = min(n_clusters or _choose_k(n), n)
    kmeans = KMeans(n_clusters=k, random_state=42, n_init=10)
    kmeans_labels = kmeans.fit_predict(vectors)

    with conn.cursor() as cur:
        # Clear previous clusters
        cur.execute("DELETE FROM track_clusters")
        cur.execute("DELETE FROM cluster_centers")

        # Store track -> cluster mapping
        for tid, label in zip(track_ids, kmeans_labels):
            cur.execute(
                "INSERT INTO track_clusters (track_id, cluster_id) VALUES (%s, %s) ON CONFLICT (track_id) DO UPDATE SET cluster_id = EXCLUDED.cluster_id",
                (tid, int(label)),
            )

        # Store cluster centers and counts
        for i, center in enumerate(kmeans.cluster_centers_):
            count = int(np.sum(kmeans_labels == i))
            cur.execute(
                "INSERT INTO cluster_centers (cluster_id, center_vector, track_count) VALUES (%s, %s, %s) ON CONFLICT (cluster_id) DO UPDATE SET center_vector = EXCLUDED.center_vector, track_count = EXCLUDED.track_count",
                (int(i), center.tolist(), count),
            )

    return {
        "kmeans_clusters": int(k),
        "tracks": n,
    }


def backfill_library_clusters(n_clusters: Optional[int] = None) -> dict:
    """Backfill clusters for the whole library."""
    from db import get_conn

    with get_conn() as conn:
        result = rebuild_clusters(conn, n_clusters=n_clusters)
        conn.commit()
        return result
