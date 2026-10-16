import logging
import time
from typing import List, Optional
from uuid import UUID

import psycopg
from psycopg.types.json import Jsonb

from config import DATABASE_URL
from models import AudioFeatures, GenrePrediction, ScanResult

log = logging.getLogger(__name__)


def get_conn():
    conn = psycopg.connect(DATABASE_URL)
    try:
        import pgvector.psycopg

        pgvector.psycopg.register_vector(conn)
    except Exception:
        pass
    return conn


def wait_for_db(max_retries: int = 30, delay: float = 2.0) -> None:
    """Wait until the database is reachable."""
    last_exc = None
    for i in range(max_retries):
        try:
            with get_conn() as conn:
                with conn.cursor() as cur:
                    cur.execute("SELECT 1")
            return
        except Exception as exc:
            last_exc = exc
            log.info("waiting for db... (%d/%d)", i + 1, max_retries)
            time.sleep(delay)
    raise last_exc


def get_or_create_artist(conn: psycopg.Connection, name: str) -> UUID:
    with conn.cursor() as cur:
        cur.execute(
            """
            INSERT INTO artists (name)
            VALUES (%s)
            ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name
            RETURNING id
            """,
            (name,),
        )
        return cur.fetchone()[0]


def get_or_create_album(
    conn: psycopg.Connection,
    title: str,
    artist_id: Optional[UUID] = None,
    year: Optional[int] = None,
) -> UUID:
    with conn.cursor() as cur:
        cur.execute(
            """
            INSERT INTO albums (artist_id, title, year)
            VALUES (%s, %s, %s)
            ON CONFLICT (artist_id, title) DO UPDATE
                SET year = EXCLUDED.year
            RETURNING id
            """,
            (artist_id, title, year),
        )
        return cur.fetchone()[0]




def get_track_by_path(conn: psycopg.Connection, path: str) -> Optional[UUID]:
    with conn.cursor() as cur:
        cur.execute("SELECT id FROM tracks WHERE path = %s", (path,))
        row = cur.fetchone()
        return row[0] if row else None


def create_scan_job(
    conn: psycopg.Connection,
    path: str,
    job_id: Optional[UUID] = None,
) -> UUID:
    with conn.cursor() as cur:
        if job_id:
            cur.execute(
                """
                INSERT INTO scan_jobs (id, path) VALUES (%s, %s)
                ON CONFLICT (id) DO UPDATE SET path = EXCLUDED.path
                RETURNING id
                """,
                (job_id, path),
            )
        else:
            cur.execute(
                "INSERT INTO scan_jobs (path) VALUES (%s) RETURNING id",
                (path,),
            )
        return cur.fetchone()[0]


def update_scan_job(
    conn: psycopg.Connection,
    job_id: UUID,
    status: str,
    error: Optional[str] = None,
) -> None:
    with conn.cursor() as cur:
        cur.execute(
            """
            UPDATE scan_jobs
            SET status = %s, error = %s,
                started_at = CASE WHEN %s = 'running' AND started_at IS NULL THEN NOW() ELSE started_at END,
                finished_at = CASE WHEN %s IN ('completed', 'failed') THEN NOW() ELSE finished_at END
            WHERE id = %s
            """,
            (status, error, status, status, job_id),
        )


def upsert_scan_job_progress(
    conn: psycopg.Connection,
    job_id: UUID,
    master_job_id: Optional[UUID],
    result: ScanResult,
) -> None:
    with conn.cursor() as cur:
        cur.execute(
            """
            INSERT INTO scan_job_progress (job_id, master_job_id, stats, updated_at)
            VALUES (%s, %s, %s, NOW())
            ON CONFLICT (job_id) DO UPDATE
            SET master_job_id = EXCLUDED.master_job_id,
                stats = EXCLUDED.stats,
                updated_at = NOW()
            """,
            (job_id, master_job_id, Jsonb(result.to_dict())),
        )


def get_scan_job(conn: psycopg.Connection, job_id: UUID) -> Optional[dict]:
    with conn.cursor() as cur:
        cur.execute(
            "SELECT id, path, status, error, started_at, finished_at, created_at "
            "FROM scan_jobs WHERE id = %s",
            (job_id,),
        )
        row = cur.fetchone()
        if not row:
            return None
        return {
            "id": str(row[0]),
            "path": row[1],
            "status": row[2],
            "error": row[3],
            "started_at": row[4].isoformat() if row[4] else None,
            "finished_at": row[5].isoformat() if row[5] else None,
            "created_at": row[6].isoformat() if row[6] else None,
        }


def get_scan_status(conn: psycopg.Connection, job_id: UUID) -> Optional[dict]:
    job = get_scan_job(conn, job_id)
    if not job:
        return None

    stats = {
        "total_files": 0,
        "imported": 0,
        "scanned": 0,
        "features": 0,
        "model_success": 0,
        "model_failed": 0,
        "failed_paths": [],
    }

    with conn.cursor() as cur:
        cur.execute(
            "SELECT stats FROM scan_job_progress WHERE master_job_id = %s",
            (job_id,),
        )
        for row in cur.fetchall():
            s = row[0] or {}
            stats["total_files"] += s.get("total_files", 0) or 0
            stats["imported"] += s.get("imported", 0) or 0
            stats["scanned"] += s.get("scanned", 0) or 0
            stats["features"] += s.get("features", 0) or 0
            stats["model_success"] += s.get("model_success", 0) or 0
            stats["model_failed"] += s.get("model_failed", 0) or 0
            for fp in s.get("failed_paths", []) or []:
                stats["failed_paths"].append(fp)

    # Also include the master job's own progress row if no sub-jobs yet.
    with conn.cursor() as cur:
        cur.execute(
            "SELECT stats FROM scan_job_progress WHERE job_id = %s",
            (job_id,),
        )
        row = cur.fetchone()
        if row and not stats["total_files"]:
            s = row[0] or {}
            stats["total_files"] = s.get("total_files", 0) or 0
            stats["imported"] = s.get("imported", 0) or 0
            stats["scanned"] = s.get("scanned", 0) or 0
            stats["features"] = s.get("features", 0) or 0
            stats["model_success"] = s.get("model_success", 0) or 0
            stats["model_failed"] = s.get("model_failed", 0) or 0
            stats["failed_paths"] = list(s.get("failed_paths", []) or [])

    job["stats"] = stats
    return job






def get_track_stats_by_paths(conn: psycopg.Connection, paths: List[str]) -> dict:
    with conn.cursor() as cur:
        cur.execute(
            """
            SELECT t.id, t.path, t.file_size, t.file_mtime, af.track_id IS NOT NULL AS has_features
            FROM tracks t
            LEFT JOIN audio_features af ON t.id = af.track_id
            WHERE t.path = ANY(%s)
            """,
            (paths,),
        )
        return {
            row[1]: {
                "id": row[0],
                "file_size": row[2],
                "file_mtime": row[3],
                "has_features": row[4],
            }
            for row in cur.fetchall()
        }


def upsert_tracks_batch(conn: psycopg.Connection, records: List[dict]) -> None:
    if not records:
        return
    values = [
        (
            rec["path"],
            rec.get("artist_id"),
            rec.get("album_id"),
            rec["title"],
            rec.get("track_number"),
            rec.get("duration"),
            rec.get("sample_rate"),
            rec.get("channels"),
            rec.get("file_size"),
            rec.get("file_mtime"),
        )
        for rec in records
    ]
    with conn.cursor() as cur:
        cur.executemany(
            """
            INSERT INTO tracks
                (path, artist_id, album_id, title, track_number, duration_seconds,
                 sample_rate, channels, file_size, file_mtime, updated_at)
            VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, NOW())
            ON CONFLICT (path) DO UPDATE SET
                artist_id = EXCLUDED.artist_id,
                album_id = EXCLUDED.album_id,
                title = EXCLUDED.title,
                track_number = EXCLUDED.track_number,
                duration_seconds = EXCLUDED.duration_seconds,
                sample_rate = EXCLUDED.sample_rate,
                channels = EXCLUDED.channels,
                file_size = EXCLUDED.file_size,
                file_mtime = EXCLUDED.file_mtime,
                updated_at = NOW()
            """,
            values,
        )


def upsert_audio_features_batch(
    conn: psycopg.Connection, records: List[AudioFeatures], path_to_id: dict
) -> None:
    if not records:
        return
    values = []
    for rec in records:
        track_id = path_to_id.get(rec["path"])
        if not track_id:
            continue
        values.append(
            (
                track_id,
                rec["bpm"],
                rec["key"],
                rec["loudness"],
                rec["energy"],
                rec["valence"],
                rec["outro_start_seconds"],
                rec["ideal_crossfade_seconds"],
                rec["intro_start_seconds"],
                rec["outro_end_seconds"],
                Jsonb(rec["chroma"]) if rec.get("chroma") else None,
                rec.get("spectral_centroid"),
                rec.get("mfcc"),
                rec.get("feature_vector"),
            )
        )
    if not values:
        return
    with conn.cursor() as cur:
        cur.executemany(
            """
            INSERT INTO audio_features
                (track_id, bpm, key, loudness, energy, valence,
                 outro_start_seconds, ideal_crossfade_seconds, intro_start_seconds,
                 outro_end_seconds, chroma, spectral_centroid, mfcc, feature_vector, analyzed_at)
            VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, NOW())
            ON CONFLICT (track_id) DO UPDATE SET
                bpm = EXCLUDED.bpm,
                key = EXCLUDED.key,
                loudness = EXCLUDED.loudness,
                energy = EXCLUDED.energy,
                valence = EXCLUDED.valence,
                outro_start_seconds = EXCLUDED.outro_start_seconds,
                ideal_crossfade_seconds = EXCLUDED.ideal_crossfade_seconds,
                intro_start_seconds = EXCLUDED.intro_start_seconds,
                outro_end_seconds = EXCLUDED.outro_end_seconds,
                chroma = EXCLUDED.chroma,
                spectral_centroid = EXCLUDED.spectral_centroid,
                mfcc = EXCLUDED.mfcc,
                feature_vector = EXCLUDED.feature_vector,
                analyzed_at = NOW()
            """,
            values,
        )


def delete_tracks(conn: psycopg.Connection, track_ids: List[UUID]) -> None:
    if not track_ids:
        return
    with conn.cursor() as cur:
        cur.execute(
            "DELETE FROM tracks WHERE id = ANY(%s)",
            (track_ids,),
        )


def upsert_track_genres(
    conn: psycopg.Connection,
    track_id: UUID,
    predictions: List[GenrePrediction],
) -> None:
    if not predictions:
        return
    values = [
        (track_id, p.main_genre, p.sub_genre, p.confidence, p.source)
        for p in predictions
    ]
    with conn.cursor() as cur:
        cur.executemany(
            """
            INSERT INTO track_genres
                (track_id, main_genre, sub_genre, confidence, source, updated_at)
            VALUES (%s, %s, %s, %s, %s, NOW())
            ON CONFLICT (track_id, sub_genre, source) DO UPDATE SET
                main_genre = EXCLUDED.main_genre,
                confidence = EXCLUDED.confidence,
                updated_at = NOW()
            """,
            values,
        )


def get_track_genres(conn: psycopg.Connection, track_id: UUID) -> List[dict]:
    with conn.cursor() as cur:
        cur.execute(
            """
            SELECT main_genre, sub_genre, confidence, source
            FROM track_genres
            WHERE track_id = %s
            ORDER BY confidence DESC
            """,
            (track_id,),
        )
        return [
            {"main_genre": r[0], "sub_genre": r[1], "confidence": r[2], "source": r[3]}
            for r in cur.fetchall()
        ]


def list_genres(conn: psycopg.Connection, min_confidence: float = 0.3) -> List[dict]:
    with conn.cursor() as cur:
        cur.execute(
            """
            SELECT main_genre, sub_genre, COUNT(DISTINCT track_id) as track_count
            FROM track_genres
            WHERE confidence >= %s
            GROUP BY main_genre, sub_genre
            ORDER BY main_genre, sub_genre
            """,
            (min_confidence,),
        )
        return [
            {"main_genre": r[0], "sub_genre": r[1], "track_count": r[2]}
            for r in cur.fetchall()
        ]









def count_tracks(conn: psycopg.Connection) -> int:
    with conn.cursor() as cur:
        cur.execute("SELECT COUNT(*) FROM tracks")
        return cur.fetchone()[0]



def get_main_genre_counts(conn: psycopg.Connection, min_confidence: float) -> List[dict]:
    """Return main genres with how many tracks and sub-genres each has."""
    with conn.cursor() as cur:
        cur.execute(
            """
            SELECT main_genre,
                   COUNT(DISTINCT track_id) as track_count,
                   COUNT(DISTINCT sub_genre) as sub_count
            FROM track_genres
            WHERE confidence >= %s
            GROUP BY main_genre
            ORDER BY track_count DESC
            """,
            (min_confidence,),
        )
        return [
            {"main_genre": r[0], "track_count": r[1], "sub_count": r[2]}
            for r in cur.fetchall()
        ]


def get_sub_genre_counts(conn: psycopg.Connection, min_confidence: float) -> List[dict]:
    with conn.cursor() as cur:
        cur.execute(
            """
            SELECT main_genre, sub_genre,
                   COUNT(DISTINCT track_id) as track_count
            FROM track_genres
            WHERE confidence >= %s
            GROUP BY main_genre, sub_genre
            ORDER BY main_genre, sub_genre
            """,
            (min_confidence,),
        )
        return [
            {"main_genre": r[0], "sub_genre": r[1], "track_count": r[2]}
            for r in cur.fetchall()
        ]


def count_uncategorized_tracks(conn: psycopg.Connection, min_confidence: float) -> int:
    """Analysed tracks without a genre prediction at or above min_confidence."""
    with conn.cursor() as cur:
        cur.execute(
            """
            SELECT COUNT(*)
            FROM tracks t
            JOIN audio_features af ON af.track_id = t.id
            WHERE NOT EXISTS (
                SELECT 1 FROM track_genres g
                WHERE g.track_id = t.id AND g.confidence >= %s
            )
            """,
            (min_confidence,),
        )
        return cur.fetchone()[0]
