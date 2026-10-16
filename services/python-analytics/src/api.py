import os
import secrets
from contextlib import contextmanager
from typing import NoReturn
from uuid import UUID

from fastapi import BackgroundTasks, FastAPI, HTTPException, Request
from fastapi.responses import JSONResponse
from pydantic import BaseModel

import db
from config import ANALYTICS_API_SECRET, CELERY_BROKER_URL, GENRE_MIN_CONFIDENCE, MUSIC_DIR
from library_scan import scan_library
from library_scan.options import ScanOptions

app = FastAPI(title="OwnWave Analytics")


@app.middleware("http")
async def require_internal_token(request: Request, call_next):
    if request.url.path == "/health":
        return await call_next(request)
    token = request.headers.get("x-internal-token", "")
    if not ANALYTICS_API_SECRET or not secrets.compare_digest(token, ANALYTICS_API_SECRET):
        return JSONResponse({"detail": "unauthorized"}, status_code=401)
    return await call_next(request)


def _raise_invalid_scan_path() -> NoReturn:
    raise HTTPException(status_code=400, detail="path outside music directory or does not exist")


def _resolved_scan_path(path: str) -> str:
    if "\x00" in path:
        _raise_invalid_scan_path()

    # Compare absolute requests against both the configured MUSIC_DIR string and
    # its realpath so symlink prefixes (e.g. /var vs /private/var) still match
    # without resolving the user-supplied path on disk first.
    configured = os.path.normpath(MUSIC_DIR)
    base = os.path.realpath(MUSIC_DIR)

    if os.path.isabs(path):
        normalized = os.path.normpath(path)
        if normalized == configured or normalized == base:
            rel = ""
        elif normalized.startswith(configured + os.sep):
            rel = normalized[len(configured) + len(os.sep) :]
        elif normalized.startswith(base + os.sep):
            rel = normalized[len(base) + len(os.sep) :]
        else:
            _raise_invalid_scan_path()
    else:
        rel = path

    parts = [p for p in rel.replace("\\", "/").split("/") if p and p != "."]
    if any(p == ".." for p in parts):
        _raise_invalid_scan_path()

    fullpath = os.path.normpath(os.path.join(base, *parts) if parts else base)
    # Containment check before any further filesystem use of the user-derived path.
    if os.path.commonpath([base, fullpath]) != base:
        _raise_invalid_scan_path()
    if not (fullpath == base or fullpath.startswith(base + os.sep)):
        _raise_invalid_scan_path()

    real = os.path.realpath(fullpath)
    if os.path.commonpath([base, real]) != base:
        _raise_invalid_scan_path()
    if not os.path.exists(real):
        _raise_invalid_scan_path()
    return real


@app.on_event("startup")
def startup():
    db.wait_for_db()


class ScanRequest(BaseModel):
    path: str = MUSIC_DIR
    force: bool = False


@contextmanager
def _db_conn():
    conn = db.get_conn()
    try:
        yield conn
    finally:
        conn.close()


@app.get("/health")
async def health():
    return {"status": "ok"}


def _celery_available() -> bool:
    if not CELERY_BROKER_URL:
        return False
    try:
        import redis

        r = redis.from_url(CELERY_BROKER_URL, socket_connect_timeout=1)
        r.ping()
        return True
    except Exception:
        return False


@app.post("/scan")
async def scan(req: ScanRequest, background_tasks: BackgroundTasks):
    scan_path = _resolved_scan_path(req.path)
    with _db_conn() as conn:
        job_id = db.create_scan_job(conn, scan_path)
        conn.commit()

    if _celery_available():
        try:
            from tasks import trigger_library_scan

            trigger_library_scan.delay(str(job_id), scan_path, req.force)
            return {"job_id": str(job_id), "status": "queued"}
        except Exception:
            # Redis may be configured but not reachable; fall through.
            pass

    background_tasks.add_task(_run_scan, job_id, scan_path, req.force)
    return {"job_id": str(job_id), "status": "pending"}


@app.get("/jobs/{job_id}")
async def get_job(job_id: UUID):
    with _db_conn() as conn:
        status = db.get_scan_status(conn, job_id)
        if not status:
            raise HTTPException(status_code=404, detail="not found")
        return status


@app.post("/rebuild-vectors")
async def rebuild_vectors(background_tasks: BackgroundTasks):
    if _celery_available():
        from tasks import rebuild_feature_vectors

        task = rebuild_feature_vectors.delay()
        return {"task_id": task.id, "status": "queued"}
    background_tasks.add_task(_run_rebuild_vectors)
    return {"status": "pending"}


@app.post("/rebuild-clusters")
async def rebuild_clusters():
    if _celery_available():
        from tasks import rebuild_clusters

        task = rebuild_clusters.delay()
        return {"task_id": task.id, "status": "queued"}
    return {"status": "not implemented"}


@app.get("/genres")
async def list_genres():
    with _db_conn() as conn:
        return {"genres": db.list_genres(conn)}


@app.get("/tracks/{track_id}/genres")
async def track_genres(track_id: UUID):
    with _db_conn() as conn:
        return {"track_id": str(track_id), "genres": db.get_track_genres(conn, track_id)}


@app.post("/rebuild-genres")
async def rebuild_genres(background_tasks: BackgroundTasks):
    if _celery_available():
        from tasks import rebuild_track_genres

        task = rebuild_track_genres.delay(MUSIC_DIR)
        return {"task_id": task.id, "status": "queued"}
    background_tasks.add_task(_run_rebuild_genres)
    return {"status": "pending"}


def _run_rebuild_vectors():
    from feature_vector import backfill_library_feature_vectors

    backfill_library_feature_vectors()


def _run_rebuild_genres():
    from tasks import rebuild_track_genres

    rebuild_track_genres(MUSIC_DIR)


def _run_scan(job_id: UUID, path: str, force: bool):
    with _db_conn() as conn:
        db.update_scan_job(conn, job_id, "running")
        conn.commit()
        try:
            result = scan_library(path, options=ScanOptions(force=force))
            db.upsert_scan_job_progress(conn, job_id, None, result)
            db.update_scan_job(conn, job_id, "completed")
            conn.commit()
        except Exception as e:
            db.update_scan_job(conn, job_id, "failed", error=str(e))
            conn.commit()


@app.get("/setup/summary")
async def setup_summary():
    with _db_conn() as conn:
        total_tracks = db.count_tracks(conn)
        main_genres = db.get_main_genre_counts(conn, GENRE_MIN_CONFIDENCE)
        sub_genres = db.get_sub_genre_counts(conn, GENRE_MIN_CONFIDENCE)
        return {
            "total_tracks": total_tracks,
            "main_genres": main_genres,
            "sub_genres": sub_genres,
            # Tracks the "Uncategorized" station will hold.
            "uncovered": db.count_uncategorized_tracks(conn, GENRE_MIN_CONFIDENCE),
        }
