import logging
from pathlib import Path
from typing import Optional
from uuid import UUID, uuid4

from psycopg.types.json import Jsonb

import db
import task_queue
from folder_importer import import_folder
from library_scan.prune import music_dirs, prune_library
from models import ScanResult

log = logging.getLogger(__name__)


def queue_library_scan(conn, path: str, force: bool = False) -> UUID:
    """Create a scan job and queue it inside the caller's transaction."""
    job_id = db.create_scan_job(conn, path)
    task_queue.enqueue(conn, "trigger_library_scan", master_job_id=str(job_id), path=path, force=force)
    return job_id


def trigger_library_scan(master_job_id: str, path: str, force: bool = False) -> dict:
    """Queue one import task per music folder; the last one to finish queues finalize_scan."""
    master = UUID(master_job_id)
    with db.get_conn() as conn:
        if conn.execute("SELECT 1 FROM scan_jobs WHERE parent_id = %s LIMIT 1", (master,)).fetchone():
            # Re-run after the worker died between dispatching and acknowledging.
            return {"master_job_id": master_job_id, "dispatched": 0}
        db.update_scan_job(conn, master, "running")

        root = Path(path).expanduser().resolve()
        if not root.is_dir():
            db.update_scan_job(conn, master, "failed", error=f"not a directory: {root}")
            conn.commit()
            return {"master_job_id": master_job_id, "dispatched": 0}

        folders = music_dirs(root)
        if not folders:
            db.upsert_scan_job_progress(conn, master, None, ScanResult())
            db.update_scan_job(conn, master, "completed")
            conn.commit()
            return {"master_job_id": master_job_id, "dispatched": 0}

        # Folder jobs and their tasks are created together so progress has a
        # fixed denominator and every folder job has a task to finish it.
        folder_args = []
        for folder in folders:
            job_id = db.create_scan_job(conn, folder, job_id=uuid4(), parent_id=master)
            folder_args.append(
                {
                    "folder_path": folder,
                    "job_id": str(job_id),
                    "master_job_id": master_job_id,
                    "root_path": str(root),
                    "force": force,
                }
            )
        task_queue.enqueue_many(conn, "process_audio_folder", folder_args)
        conn.commit()

    return {"master_job_id": master_job_id, "dispatched": len(folders)}


def process_audio_folder(folder_path: str, job_id: str, master_job_id: str, root_path: str, force: bool = False) -> dict:
    """Import one music folder. Raising lets the queue retry it."""
    jid = UUID(job_id)
    with db.get_conn() as conn:
        job = db.get_scan_job(conn, jid)
        if job and job["status"] in ("completed", "failed"):
            # Re-run after the worker died between finishing and acknowledging.
            return {"folder": folder_path, "job_id": job_id, "skipped": True}
        db.update_scan_job(conn, jid, "running")
        conn.commit()

    result = import_folder(folder_path, job_id=jid, master_job_id=UUID(master_job_id), analyze=True, force=force)
    _finish_folder(jid, UUID(master_job_id), root_path, "completed")
    return {"folder": folder_path, "tracks": len(result.track_ids), "job_id": job_id}


def _process_audio_folder_failed(error: str, job_id: str, master_job_id: str, root_path: str, **_) -> None:
    _finish_folder(UUID(job_id), UUID(master_job_id), root_path, "failed", error=error)


def _finish_folder(job_id: UUID, master: UUID, root_path: str, status: str, error: Optional[str] = None) -> None:
    """Record a folder's final status; the last folder of the scan queues finalize_scan.

    Each finisher locks the master job before counting, so under READ
    COMMITTED exactly one of them sees no unfinished folders left.
    """
    with db.get_conn() as conn:
        changed = conn.execute(
            """
            UPDATE scan_jobs SET status = %s, error = %s, finished_at = NOW()
            WHERE id = %s AND status IN ('pending', 'running')
            """,
            (status, error, job_id),
        ).rowcount
        if not changed:
            return
        conn.execute("SELECT 1 FROM scan_jobs WHERE id = %s FOR UPDATE", (master,))
        unfinished = conn.execute(
            "SELECT COUNT(*) FROM scan_jobs WHERE parent_id = %s AND status IN ('pending', 'running')",
            (master,),
        ).fetchone()[0]
        if unfinished == 0:
            task_queue.enqueue(conn, "finalize_scan", master_job_id=str(master), root_path=root_path)
        conn.commit()


def finalize_scan(master_job_id: str, root_path: str) -> dict:
    """Prune moved/deleted tracks, then refresh analysis.

    The master job stays running until rebuild_analysis completes it.
    """
    job_id = UUID(master_job_id)

    with db.get_conn() as conn:
        total, failed = conn.execute(
            "SELECT COUNT(*), COUNT(*) FILTER (WHERE status = 'failed') FROM scan_jobs WHERE parent_id = %s",
            (job_id,),
        ).fetchone()
        summary = {"master_job_id": master_job_id, "total": total, "failed": failed}

        if failed:
            # Skip pruning: a moved file whose new folder failed to import
            # would otherwise lose its history.
            db.update_scan_job(conn, job_id, "failed", error=f"{failed} / {total} folder tasks failed")
        else:
            try:
                summary.update(prune_library(conn, Path(root_path), _job_started_at(conn, job_id)))
            except Exception as exc:
                conn.rollback()
                db.update_scan_job(conn, job_id, "failed", error=f"prune failed: {exc}")
                conn.commit()
                raise

        # A failed scan is already finished; still refresh analysis for the
        # folders that did import.
        task_queue.enqueue(conn, "rebuild_analysis", scan_job_id=None if failed else master_job_id)
        conn.commit()
    return summary


def _job_started_at(conn, job_id: UUID):
    with conn.cursor() as cur:
        cur.execute("SELECT started_at FROM scan_jobs WHERE id = %s", (job_id,))
        row = cur.fetchone()
        return row[0] if row else None


def rebuild_analysis(scan_job_id: Optional[str] = None) -> dict:
    """Rebuild feature vectors, then clusters, then queue the genre refresh.

    With scan_job_id, this is the last step of that scan: the job completes
    or fails with it.
    """
    try:
        from clustering import backfill_library_clusters
        from feature_vector import backfill_library_feature_vectors

        vectors = backfill_library_feature_vectors()
        clusters = backfill_library_clusters()
    except Exception as exc:
        if scan_job_id:
            with db.get_conn() as conn:
                db.update_scan_job(conn, UUID(scan_job_id), "failed", error=f"analysis failed: {exc}")
                conn.commit()
        raise
    with db.get_conn() as conn:
        if scan_job_id:
            db.update_scan_job(conn, UUID(scan_job_id), "completed")
        queue_genre_refresh(conn)
        conn.commit()
    return {"vectors": vectors, "clusters": clusters}


def rebuild_feature_vectors() -> int:
    """Rebuild normalized feature vectors for all tracks."""
    from feature_vector import backfill_library_feature_vectors

    return backfill_library_feature_vectors()


def rebuild_clusters(n_clusters: Optional[int] = None) -> dict:
    """Rebuild track clusters."""
    from clustering import backfill_library_clusters

    return backfill_library_clusters(n_clusters=n_clusters)


# Artist names looked up per fetch_artist_genres run (about 1.1 s each).
MB_BATCH = 150
# Stop a run (and score anyway) after this many failed lookups.
MB_MAX_FAILURES = 5
# Advisory lock id: only one MusicBrainz fetcher may run (shared rate limit).
_MB_LOCK = 4_242_001


def queue_genre_refresh(conn) -> None:
    """Queue the genre steps that follow new analysis: artist tags, then scoring."""
    from config import MUSICBRAINZ_ENABLED

    if MUSICBRAINZ_ENABLED:
        task_queue.enqueue(conn, "fetch_artist_genres")
    else:
        task_queue.enqueue(conn, "score_genres")


def fetch_artist_genres() -> dict:
    """Look up MusicBrainz tags for artists not looked up yet, in batches.

    A run handles MB_BATCH names, then queues the next run, or score_genres
    when no names are left.
    """
    from config import MUSICBRAINZ_URL, MUSICBRAINZ_USER_AGENT
    from musicbrainz import MusicBrainzClient, name_key, primary_artist

    client = MusicBrainzClient(MUSICBRAINZ_URL, MUSICBRAINZ_USER_AGENT)
    with db.get_conn() as conn:
        if not conn.execute("SELECT pg_try_advisory_lock(%s)", (_MB_LOCK,)).fetchone()[0]:
            return {"skipped": "another fetch is running"}
        try:
            done = {r[0] for r in conn.execute("SELECT name_key FROM musicbrainz_artists").fetchall()}
            credits = [r[0] for r in conn.execute(
                "SELECT DISTINCT a.name FROM artists a JOIN tracks t ON t.artist_id = a.id ORDER BY 1"
            ).fetchall()]
            pending = [c for c in credits if name_key(c) and name_key(c) not in done
                       and name_key(c) != "unknown artist"]
            looked_up = failures = 0
            for credit in pending[:MB_BATCH]:
                if failures >= MB_MAX_FAILURES:
                    break
                names = [credit]
                primary = primary_artist(credit)
                if name_key(primary) != name_key(credit):
                    names.append(primary)
                for name in names:
                    key = name_key(name)
                    if key in done:
                        continue
                    try:
                        found = client.lookup_artist(name)
                    except Exception as exc:
                        failures += 1
                        log.warning("MusicBrainz lookup failed for %r: %s", name, exc)
                        break
                    looked_up += 1
                    conn.execute(
                        """
                        INSERT INTO musicbrainz_artists (name_key, mbid, tags, fetched_at)
                        VALUES (%s, %s, %s, NOW())
                        ON CONFLICT (name_key) DO UPDATE SET
                            mbid = EXCLUDED.mbid, tags = EXCLUDED.tags, fetched_at = NOW()
                        """,
                        (key, found.mbid, Jsonb(found.tags)),
                    )
                    conn.commit()
                    done.add(key)
                    if found.mbid:
                        break  # the whole credit matched; no need for the primary artist
            remaining = max(0, len(pending) - MB_BATCH)
            # MusicBrainz unreachable: score with what we have; failed names
            # are retried on the next refresh.
            more = remaining and failures < MB_MAX_FAILURES
            task_queue.enqueue(conn, "fetch_artist_genres" if more else "score_genres")
            conn.commit()
        finally:
            conn.execute("SELECT pg_advisory_unlock(%s)", (_MB_LOCK,))
            conn.commit()
    return {"looked_up": looked_up, "failures": failures, "remaining": remaining}


def score_genres() -> dict:
    """Blend all genre sources into track_station_genres for the whole library."""
    from config import GENRE_MODEL_DIR
    from genre_audio import load_labels
    from genre_scoring import score_library

    with db.get_conn() as conn:
        summary = score_library(conn, load_labels(GENRE_MODEL_DIR))
        conn.commit()
    return summary


TASKS = {
    fn.__name__: fn
    for fn in (
        trigger_library_scan,
        process_audio_folder,
        finalize_scan,
        rebuild_analysis,
        rebuild_feature_vectors,
        rebuild_clusters,
        fetch_artist_genres,
        score_genres,
    )
}

# Called with the error and the task's args once a task is out of attempts.
ON_FAILURE = {"process_audio_folder": _process_audio_folder_failed}
