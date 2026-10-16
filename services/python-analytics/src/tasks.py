import logging
from pathlib import Path
from typing import Optional
from uuid import UUID, uuid4

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
    """Rebuild feature vectors, then clusters.

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
    if scan_job_id:
        with db.get_conn() as conn:
            db.update_scan_job(conn, UUID(scan_job_id), "completed")
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


def rebuild_track_genres(path: str, force: bool = False) -> dict:
    """Batch backfill genre predictions for a path."""
    from audio_metadata import is_supported_audio_file
    from config import ENABLE_GENRE_ANALYSIS
    from genre_sources import get_genre_sources

    if not ENABLE_GENRE_ANALYSIS:
        return {"updated": 0, "skipped": 0}

    ml_sources = [s for s in get_genre_sources() if s.source_id == "discogs400"]
    if not ml_sources:
        return {"updated": 0, "skipped": 0}

    root = Path(path).expanduser().resolve()
    if not root.exists():
        raise FileNotFoundError(f"Music path does not exist: {root}")

    files = sorted(p for p in root.rglob("*") if is_supported_audio_file(p))
    updated = 0
    skipped = 0

    with db.get_conn() as conn:
        for file_path in files:
            path_str = str(file_path)
            track_id = db.get_track_by_path(conn, path_str)
            if not track_id:
                skipped += 1
                continue
            if not force:
                existing = db.get_track_genres(conn, track_id)
                if existing:
                    skipped += 1
                    continue
            try:
                predictions = []
                for source in ml_sources:
                    predictions.extend(source.predict(path_str))
                if predictions:
                    db.upsert_track_genres(conn, track_id, predictions)
                    updated += 1
            except Exception as exc:
                log.warning("genre rebuild failed for %s: %s", path_str, exc)
                skipped += 1
        conn.commit()

    return {"updated": updated, "skipped": skipped}


TASKS = {
    fn.__name__: fn
    for fn in (
        trigger_library_scan,
        process_audio_folder,
        finalize_scan,
        rebuild_analysis,
        rebuild_feature_vectors,
        rebuild_clusters,
        rebuild_track_genres,
    )
}

# Called with the error and the task's args once a task is out of attempts.
ON_FAILURE = {"process_audio_folder": _process_audio_folder_failed}
