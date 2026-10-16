import logging
from pathlib import Path
from typing import List, Optional
from uuid import UUID, uuid4

from celery import chord, group

import db
from celery_app import celery_app
from folder_importer import import_folder
from library_scan.prune import music_dirs, prune_library
from models import ScanResult

log = logging.getLogger(__name__)


@celery_app.task(bind=True, max_retries=3, default_retry_delay=5)
def process_audio_folder(self, folder_path: str, master_job_id: str, force: bool = False) -> dict:
    """Import one music folder and update its scan job.

    Returns a failure marker instead of raising once retries are exhausted, so
    the scan chord always reaches finalize_scan.
    """
    job_id = UUID(self.request.id)

    with db.get_conn() as conn:
        db.create_scan_job(conn, folder_path, job_id=job_id)
        db.update_scan_job(conn, job_id, "running")
        conn.commit()

    try:
        result = import_folder(
            folder_path,
            job_id=job_id,
            master_job_id=UUID(master_job_id),
            analyze=True,
            force=force,
        )

        with db.get_conn() as conn:
            db.update_scan_job(conn, job_id, "completed")
            conn.commit()

        return {
            "folder": folder_path,
            "tracks": len(result.track_ids),
            "stats": result.to_dict(),
            "job_id": str(job_id),
        }
    except Exception as exc:
        with db.get_conn() as conn:
            db.update_scan_job(conn, job_id, "failed", error=str(exc))
            conn.commit()
        if self.request.retries < self.max_retries:
            raise self.retry(exc=exc)
        return {"folder": folder_path, "failed": True, "error": str(exc), "job_id": str(job_id)}


@celery_app.task
def trigger_library_scan(master_job_id: Optional[str], path: str, force: bool = False) -> dict:
    """Dispatch one import task per music folder, then finalize via a chord."""
    db.wait_for_db()

    with db.get_conn() as conn:
        if master_job_id:
            db.update_scan_job(conn, UUID(master_job_id), "running")
        else:
            master_job_id = str(db.create_scan_job(conn, path))
            db.update_scan_job(conn, UUID(master_job_id), "running")
        conn.commit()

    root = Path(path).expanduser().resolve()
    if not root.is_dir():
        with db.get_conn() as conn:
            db.update_scan_job(conn, UUID(master_job_id), "failed", error=f"not a directory: {root}")
            conn.commit()
        raise FileNotFoundError(f"Music path does not exist: {root}")

    folders = music_dirs(root)
    if not folders:
        with db.get_conn() as conn:
            db.upsert_scan_job_progress(conn, UUID(master_job_id), None, ScanResult())
            db.update_scan_job(conn, UUID(master_job_id), "completed")
            conn.commit()
        return {"master_job_id": master_job_id, "dispatched": 0}

    # Create the folder jobs up front so progress has a fixed denominator.
    master = UUID(master_job_id)
    folder_jobs = [(folder, uuid4()) for folder in folders]
    with db.get_conn() as conn:
        for folder, job_id in folder_jobs:
            db.create_scan_job(conn, folder, job_id=job_id, parent_id=master)
        conn.commit()

    header = group(
        process_audio_folder.s(folder, master_job_id, force).set(task_id=str(job_id))
        for folder, job_id in folder_jobs
    )
    finalize = chord(header)(finalize_scan.s(master_job_id, str(root)))

    return {
        "master_job_id": master_job_id,
        "dispatched": len(folders),
        "finalize_id": finalize.id,
    }


@celery_app.task
def finalize_scan(results: List[dict], master_job_id: str, root_path: str) -> dict:
    """Prune moved/deleted tracks, then refresh analysis.

    The master job stays running until rebuild_analysis completes it.
    """
    job_id = UUID(master_job_id)
    failures = [r for r in results if not r or r.get("failed")]
    summary = {"master_job_id": master_job_id, "total": len(results), "failed": len(failures)}

    with db.get_conn() as conn:
        if failures:
            # Skip pruning: a moved file whose new folder failed to import
            # would otherwise lose its history.
            db.update_scan_job(
                conn, job_id, "failed",
                error=f"{len(failures)} / {len(results)} folder tasks failed",
            )
            conn.commit()
        else:
            try:
                summary.update(prune_library(conn, Path(root_path), _job_started_at(conn, job_id)))
                conn.commit()
            except Exception as exc:
                conn.rollback()
                db.update_scan_job(conn, job_id, "failed", error=f"prune failed: {exc}")
                conn.commit()
                raise

    # A failed scan is already finished; still refresh analysis for the
    # folders that did import.
    rebuild_analysis.delay(None if failures else master_job_id)
    return summary


def _job_started_at(conn, job_id: UUID):
    with conn.cursor() as cur:
        cur.execute("SELECT started_at FROM scan_jobs WHERE id = %s", (job_id,))
        row = cur.fetchone()
        return row[0] if row else None


@celery_app.task
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


@celery_app.task
def rebuild_feature_vectors() -> int:
    """Rebuild normalized feature vectors for all tracks."""
    from feature_vector import backfill_library_feature_vectors

    return backfill_library_feature_vectors()


@celery_app.task
def rebuild_clusters(n_clusters: Optional[int] = None) -> dict:
    """Rebuild track clusters."""
    from clustering import backfill_library_clusters

    return backfill_library_clusters(n_clusters=n_clusters)


@celery_app.task
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
