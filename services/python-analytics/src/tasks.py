from pathlib import Path
from typing import List, Optional
from uuid import UUID

from celery import shared_task
from celery.result import AsyncResult

import db
from celery_app import celery_app
from config import MUSIC_DIR
from folder_importer import import_folder
from scanner import SUPPORTED_EXTS


@celery_app.task(bind=True, max_retries=3, default_retry_delay=5)
def process_audio_folder(self, folder_path: str, master_job_id: str) -> dict:
    """Import one music folder and update its scan job."""
    with db.get_conn() as conn:
        job_id = db.create_scan_job(conn, folder_path)
        conn.commit()

    try:
        db.wait_for_db()
        with db.get_conn() as conn:
            db.update_scan_job(conn, job_id, "running")
            conn.commit()

        track_ids = import_folder(folder_path, analyze=True, force=False)

        with db.get_conn() as conn:
            db.update_scan_job(conn, job_id, "completed")
            conn.commit()

        return {"folder": folder_path, "tracks": len(track_ids), "job_id": str(job_id)}
    except Exception as exc:
        with db.get_conn() as conn:
            db.update_scan_job(conn, job_id, "failed", error=str(exc))
            conn.commit()
        raise self.retry(exc=exc)


@celery_app.task
def trigger_library_scan(master_job_id: Optional[str], path: str, force: bool = False) -> dict:
    """Split a music library into folder jobs and dispatch Celery workers."""
    db.wait_for_db()

    with db.get_conn() as conn:
        if master_job_id:
            db.update_scan_job(conn, UUID(master_job_id), "running")
        else:
            master_job_id = str(db.create_scan_job(conn, path))
        conn.commit()

    root = Path(path).expanduser().resolve()
    if not root.exists():
        raise FileNotFoundError(f"Music path does not exist: {root}")

    if not root.is_dir():
        # A single file or leaf folder was passed; process it directly.
        folders = [str(root)]
    else:
        # Find all directories under root that contain music files and are not
        # ancestors of any other music-containing directory. This avoids
        # duplicate work when a parent and its children are both scanned.
        music_dirs: List[Path] = []
        for p in root.rglob("*"):
            if p.is_file() and p.suffix.lower() in SUPPORTED_EXTS:
                music_dirs.append(p.parent)

        by_depth = sorted(set(music_dirs), key=lambda d: len(d.parts), reverse=True)
        selected: List[str] = []
        selected_paths: set = set()
        for d in by_depth:
            d_str = str(d)
            if any(sel.startswith(d_str + "/") for sel in selected_paths):
                continue
            selected_paths.add(d_str)
            selected.append(d_str)
        folders = selected

    if not folders:
        with db.get_conn() as conn:
            db.update_scan_job(conn, UUID(master_job_id), "completed")
            conn.commit()
        return {"master_job_id": master_job_id, "dispatched": 0}

    subtasks = [
        process_audio_folder.s(folder, master_job_id) for folder in folders
    ]
    results = [task.delay() for task in subtasks]
    task_ids = [r.id for r in results]

    finalize_result = finalize_scan.delay(master_job_id, task_ids)

    return {
        "master_job_id": master_job_id,
        "dispatched": len(folders),
        "finalize_id": finalize_result.id,
    }


@celery_app.task(bind=True, max_retries=2000, default_retry_delay=2)
def finalize_scan(self, master_job_id: str, task_ids: List[str]) -> dict:
    """Poll subtask results and finalize the master scan job."""
    results = [AsyncResult(tid) for tid in task_ids]

    if not all(r.ready() for r in results):
        raise self.retry(countdown=2)

    failures = [r for r in results if not r.successful()]

    with db.get_conn() as conn:
        if failures:
            db.update_scan_job(
                conn,
                UUID(master_job_id),
                "failed",
                error=f"{len(failures)} / {len(results)} folder tasks failed",
            )
        else:
            db.update_scan_job(conn, UUID(master_job_id), "completed")
        conn.commit()

    return {
        "master_job_id": master_job_id,
        "total": len(results),
        "failed": len(failures),
    }
