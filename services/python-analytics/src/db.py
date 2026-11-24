import logging
import time
from typing import List, Optional
from uuid import UUID

import psycopg
from psycopg.adapt import AdaptersMap
from psycopg.types.json import Jsonb

from config import DATABASE_URL
from models import AudioFeatures, ScanResult

log = logging.getLogger(__name__)


# Adapters with the pgvector types, shared by every connection of this process.
# register_vector builds new classes on each call and psycopg caches them for
# good, so calling it per connection leaks about 24 KB each time. The type OIDs
# are kept too: restart the process if the vector extension is ever recreated.
_vector_adapters: Optional[AdaptersMap] = None


def get_conn():
    global _vector_adapters
    if _vector_adapters is not None:
        return psycopg.connect(DATABASE_URL, context=_vector_adapters)

    conn = psycopg.connect(DATABASE_URL)
    try:
        import pgvector.psycopg

        pgvector.psycopg.register_vector(conn)
        _vector_adapters = conn.adapters
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



def get_or_create_release(
    conn: psycopg.Connection,
    *,
    mb_release_id: Optional[str],
    folder: str,
    title: str,
    album_artist: Optional[str] = None,
    mb_release_group_id: Optional[str] = None,
    mb_release_type: Optional[str] = None,
    tagged_compilation: bool = False,
) -> UUID:
    """The release of a file: its MusicBrainz release when it names one,
    otherwise the folder and album title. Updates the release's other tags.

    is_compilation is left alone: refresh_releases sets it once the
    release's tracks are stored.
    """
    if mb_release_id:
        conflict = "(mb_release_id) WHERE mb_release_id IS NOT NULL"
    else:
        conflict = "(folder, title) WHERE mb_release_id IS NULL"
    row = conn.execute(
        f"""
        INSERT INTO releases
            (mb_release_id, folder, title, album_artist, mb_release_group_id,
             mb_release_type, tagged_compilation)
        VALUES (%s, %s, %s, %s, %s, %s, %s)
        ON CONFLICT {conflict} DO UPDATE SET
            title = EXCLUDED.title,
            album_artist = EXCLUDED.album_artist,
            mb_release_group_id = EXCLUDED.mb_release_group_id,
            mb_release_type = EXCLUDED.mb_release_type,
            tagged_compilation = EXCLUDED.tagged_compilation,
            updated_at = NOW()
        RETURNING id
        """,
        (
            mb_release_id,
            folder,
            title,
            album_artist,
            mb_release_group_id,
            mb_release_type,
            tagged_compilation,
        ),
    ).fetchone()
    return row[0]


# A release with more distinct track artists than this is a compilation.
COMPILATION_MIN_ARTISTS = 3


def refresh_releases(conn: psycopg.Connection, release_ids) -> None:
    """After tracks joined or left these releases: delete the ones left with
    no tracks and work out is_compilation for the rest (tagged as one, or
    more than COMPILATION_MIN_ARTISTS distinct track artists)."""
    ids = [r for r in set(release_ids) if r]
    if not ids:
        return
    conn.execute(
        """
        DELETE FROM releases r
        WHERE r.id = ANY(%s)
          AND NOT EXISTS (SELECT 1 FROM tracks t WHERE t.release_id = r.id)
        """,
        (ids,),
    )
    conn.execute(
        """
        UPDATE releases r
        SET is_compilation = r.tagged_compilation
                OR (SELECT count(DISTINCT t.artist_id) FROM tracks t WHERE t.release_id = r.id) > %s
        WHERE r.id = ANY(%s)
        """,
        (COMPILATION_MIN_ARTISTS, ids),
    )


def get_track_by_path(conn: psycopg.Connection, path: str) -> Optional[UUID]:
    with conn.cursor() as cur:
        cur.execute("SELECT id FROM tracks WHERE path = %s", (path,))
        row = cur.fetchone()
        return row[0] if row else None


def create_scan_job(
    conn: psycopg.Connection,
    path: str,
    job_id: Optional[UUID] = None,
    parent_id: Optional[UUID] = None,
) -> UUID:
    with conn.cursor() as cur:
        if job_id:
            cur.execute(
                """
                INSERT INTO scan_jobs (id, path, parent_id) VALUES (%s, %s, %s)
                ON CONFLICT (id) DO UPDATE SET path = EXCLUDED.path
                RETURNING id
                """,
                (job_id, path, parent_id),
            )
        else:
            cur.execute(
                "INSERT INTO scan_jobs (path, parent_id) VALUES (%s, %s) RETURNING id",
                (path, parent_id),
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


_SCAN_JOB_COLUMNS = "id, path, status, error, started_at, finished_at, created_at"


def _scan_job_row(row) -> dict:
    return {
        "id": str(row[0]),
        "path": row[1],
        "status": row[2],
        "error": row[3],
        "started_at": row[4].isoformat() if row[4] else None,
        "finished_at": row[5].isoformat() if row[5] else None,
        "created_at": row[6].isoformat() if row[6] else None,
    }


def get_scan_job(conn: psycopg.Connection, job_id: UUID) -> Optional[dict]:
    with conn.cursor() as cur:
        cur.execute(f"SELECT {_SCAN_JOB_COLUMNS} FROM scan_jobs WHERE id = %s", (job_id,))
        row = cur.fetchone()
        return _scan_job_row(row) if row else None


_STAT_COUNTERS = ("total_files", "imported", "scanned", "features", "model_success", "model_failed")


def scan_stats(conn: psycopg.Connection, job_id: UUID) -> dict:
    """Sum the per-folder progress rows of a master job.

    A scan with no music folders only has the master job's own row.
    """
    stats = {k: 0 for k in _STAT_COUNTERS}
    stats["failed_paths"] = []
    with conn.cursor() as cur:
        cur.execute(
            """
            SELECT stats FROM scan_job_progress WHERE master_job_id = %s
            UNION ALL
            SELECT stats FROM scan_job_progress
            WHERE job_id = %s
              AND NOT EXISTS (SELECT 1 FROM scan_job_progress WHERE master_job_id = %s)
            """,
            (job_id, job_id, job_id),
        )
        for (s,) in cur.fetchall():
            s = s or {}
            for k in _STAT_COUNTERS:
                stats[k] += s.get(k, 0) or 0
            stats["failed_paths"].extend(s.get("failed_paths", []) or [])
    return stats


def _count_folder_jobs(conn: psycopg.Connection, job_id: UUID) -> dict:
    with conn.cursor() as cur:
        cur.execute(
            """
            SELECT COUNT(*),
                   COUNT(*) FILTER (WHERE status = 'completed'),
                   COUNT(*) FILTER (WHERE status = 'failed')
            FROM scan_jobs WHERE parent_id = %s
            """,
            (job_id,),
        )
        total, done, failed = cur.fetchone()
    return {"total": total, "done": done, "failed": failed}


def _scan_folders(conn: psycopg.Connection, job_id: UUID) -> dict:
    """Count a master job's folder jobs by outcome.

    A finished scan's folder jobs are deleted (roll_up_scan); its counts are
    then read from the master's own progress row.
    """
    folders = _count_folder_jobs(conn, job_id)
    if folders["total"]:
        return folders
    with conn.cursor() as cur:
        cur.execute("SELECT stats->'folders' FROM scan_job_progress WHERE job_id = %s", (job_id,))
        row = cur.fetchone()
    saved = (row[0] if row else None) or {}
    return {k: int(saved.get(k, 0) or 0) for k in ("total", "done", "failed")}


def roll_up_scan(conn: psycopg.Connection, job_id: UUID) -> None:
    """Fold a finished scan's folder jobs into its master's progress row.

    The master row then holds the summed folder progress and the folder
    counts, so the scan reports the same totals, and the folder jobs are
    deleted (their progress rows go with them through the cascade).
    """
    folders = _count_folder_jobs(conn, job_id)
    if not folders["total"]:
        return
    stats = scan_stats(conn, job_id)
    stats["folders"] = folders
    with conn.cursor() as cur:
        cur.execute(
            """
            INSERT INTO scan_job_progress (job_id, master_job_id, stats, updated_at)
            VALUES (%s, NULL, %s, NOW())
            ON CONFLICT (job_id) DO UPDATE
            SET master_job_id = NULL, stats = EXCLUDED.stats, updated_at = NOW()
            """,
            (job_id, Jsonb(stats)),
        )
        cur.execute("DELETE FROM scan_jobs WHERE parent_id = %s", (job_id,))


# Library scans kept in scan_jobs; older finished ones are deleted.
SCAN_JOBS_KEPT = 100


def prune_scan_jobs(conn: psycopg.Connection, keep: int = SCAN_JOBS_KEPT) -> int:
    """Delete finished scans beyond the newest `keep` scans."""
    with conn.cursor() as cur:
        cur.execute(
            """
            DELETE FROM scan_jobs
            WHERE parent_id IS NULL
              AND status IN ('completed', 'failed')
              AND id NOT IN (
                  SELECT id FROM scan_jobs WHERE parent_id IS NULL
                  ORDER BY created_at DESC LIMIT %s
              )
            """,
            (keep,),
        )
        return cur.rowcount


def get_scan_status(conn: psycopg.Connection, job_id: UUID) -> Optional[dict]:
    job = get_scan_job(conn, job_id)
    if not job:
        return None
    job["stats"] = scan_stats(conn, job_id)
    job["folders"] = _scan_folders(conn, job_id)
    return job


def list_scan_jobs(conn: psycopg.Connection, limit: int) -> List[dict]:
    """Most recent library scans (master jobs), newest first.

    Failed files are reduced to a count; fetch one job for the full list.
    """
    with conn.cursor() as cur:
        cur.execute(
            f"SELECT {_SCAN_JOB_COLUMNS} FROM scan_jobs "
            "WHERE parent_id IS NULL ORDER BY created_at DESC LIMIT %s",
            (limit,),
        )
        jobs = [_scan_job_row(row) for row in cur.fetchall()]
    for job in jobs:
        job_id = UUID(job["id"])
        stats = scan_stats(conn, job_id)
        stats["failed_files"] = len(stats.pop("failed_paths"))
        job["stats"] = stats
        job["folders"] = _scan_folders(conn, job_id)
    return jobs


def get_track_stats_by_paths(conn: psycopg.Connection, paths: List[str]) -> dict:
    with conn.cursor() as cur:
        cur.execute(
            """
            SELECT t.id, t.path, t.file_size, t.file_mtime, af.track_id IS NOT NULL AS has_features,
                   ff.track_id IS NOT NULL AS features_failed, gf.model_version,
                   t.tags_version, t.release_id
            FROM tracks t
            LEFT JOIN audio_features af ON t.id = af.track_id
            LEFT JOIN track_analysis_failures ff ON ff.track_id = t.id AND ff.kind = 'features'
            LEFT JOIN track_analysis_failures gf ON gf.track_id = t.id AND gf.kind = 'genre'
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
                "features_failed": row[5],
                "genre_failed_version": row[6],
                "tags_version": row[7],
                "release_id": row[8],
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
            rec.get("genre_tag"),
            rec.get("release_id"),
            rec.get("isrc"),
            rec.get("mb_recording_id"),
            rec.get("mb_artist_ids"),
            rec.get("tags_version", 0),
        )
        for rec in records
    ]
    with conn.cursor() as cur:
        cur.executemany(
            """
            INSERT INTO tracks
                (path, artist_id, album_id, title, track_number, duration_seconds,
                 sample_rate, channels, file_size, file_mtime, genre_tag,
                 release_id, isrc, mb_recording_id, mb_artist_ids, tags_version, updated_at)
            VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, NOW())
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
                genre_tag = EXCLUDED.genre_tag,
                release_id = EXCLUDED.release_id,
                isrc = EXCLUDED.isrc,
                mb_recording_id = EXCLUDED.mb_recording_id,
                mb_artist_ids = EXCLUDED.mb_artist_ids,
                tags_version = EXCLUDED.tags_version,
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
                 outro_end_seconds, chroma, spectral_centroid, mfcc, analyzed_at)
            VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, NOW())
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
                analyzed_at = NOW()
            """,
            values,
        )


def delete_tracks(conn: psycopg.Connection, track_ids: List[UUID]) -> None:
    if not track_ids:
        return
    with conn.cursor() as cur:
        cur.execute(
            "DELETE FROM tracks WHERE id = ANY(%s) RETURNING release_id",
            (track_ids,),
        )
        release_ids = {row[0] for row in cur.fetchall() if row[0]}
    refresh_releases(conn, release_ids)


def upsert_track_audio_genres(conn: psycopg.Connection, track_id: UUID, result) -> None:
    """Store a genre_audio.AudioGenreResult for a track."""
    conn.execute(
        """
        INSERT INTO track_audio_genres (track_id, model_version, activations, embedding, analyzed_at)
        VALUES (%s, %s, %s, %s::vector, NOW())
        ON CONFLICT (track_id) DO UPDATE SET
            model_version = EXCLUDED.model_version,
            activations = EXCLUDED.activations,
            embedding = EXCLUDED.embedding,
            analyzed_at = NOW()
        """,
        (track_id, result.model_version, result.activations, str(result.embedding)),
    )


def record_analysis_failure(
    conn: psycopg.Connection, track_id: UUID, kind: str, error: str, model_version: Optional[str] = None
) -> None:
    """Remember that analysis of kind ('features' or 'genre') failed for a track (ADR 0006)."""
    conn.execute(
        """
        INSERT INTO track_analysis_failures (track_id, kind, model_version, error, failed_at)
        VALUES (%s, %s, %s, %s, NOW())
        ON CONFLICT (track_id, kind) DO UPDATE SET
            model_version = EXCLUDED.model_version,
            error = EXCLUDED.error,
            failed_at = NOW()
        """,
        (track_id, kind, model_version, error),
    )


def clear_analysis_failures(conn: psycopg.Connection, track_id: UUID, kind: Optional[str] = None) -> None:
    """Forget a track's analysis failures: one kind, or all of them when kind is None."""
    if kind is None:
        conn.execute("DELETE FROM track_analysis_failures WHERE track_id = %s", (track_id,))
    else:
        conn.execute("DELETE FROM track_analysis_failures WHERE track_id = %s AND kind = %s", (track_id, kind))


def audio_genre_versions_by_paths(conn: psycopg.Connection, paths: List[str]) -> dict:
    """path -> model_version of the stored audio genre analysis."""
    if not paths:
        return {}
    rows = conn.execute(
        """
        SELECT t.path, g.model_version FROM tracks t
        JOIN track_audio_genres g ON g.track_id = t.id
        WHERE t.path = ANY(%s)
        """,
        (paths,),
    ).fetchall()
    return {r[0]: r[1] for r in rows}


def get_track_station_genres(conn: psycopg.Connection, track_id: UUID) -> List[dict]:
    rows = conn.execute(
        """
        SELECT genre, score, is_primary FROM track_station_genres
        WHERE track_id = %s ORDER BY is_primary DESC, score DESC
        """,
        (track_id,),
    ).fetchall()
    return [{"genre": r[0], "score": round(r[1], 3), "is_primary": r[2]} for r in rows]


def count_tracks(conn: psycopg.Connection) -> int:
    with conn.cursor() as cur:
        cur.execute("SELECT COUNT(*) FROM tracks")
        return cur.fetchone()[0]
