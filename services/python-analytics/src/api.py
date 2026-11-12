import os
import secrets
from contextlib import asynccontextmanager, contextmanager
from typing import NoReturn
from uuid import UUID

from fastapi import FastAPI, HTTPException, Query, Request
from fastapi.responses import JSONResponse
from pydantic import BaseModel

import db
import task_queue
import tasks
from config import ANALYTICS_API_SECRET, GENRE_MODEL_DIR, MUSIC_DIR, analytics_secret_problem

@asynccontextmanager
async def lifespan(app: FastAPI):
    problem = analytics_secret_problem(ANALYTICS_API_SECRET)
    if problem:
        raise RuntimeError(problem)
    db.wait_for_db()
    from genre_scoring import sync_catalog

    with db.get_conn() as conn:
        sync_catalog(conn)
        conn.commit()
    yield


app = FastAPI(title="OwnWave Analytics", lifespan=lifespan)


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


# Routes are plain `def` so FastAPI runs them in its threadpool: they do
# blocking database I/O that would otherwise stall the event loop.


@app.get("/health")
def health():
    return {"status": "ok"}


@app.post("/scan")
def scan(req: ScanRequest):
    scan_path = _resolved_scan_path(req.path)
    with _db_conn() as conn:
        job_id = tasks.queue_library_scan(conn, scan_path, req.force)
        conn.commit()
    return {"job_id": str(job_id), "status": "queued"}


@app.get("/jobs")
def list_jobs(limit: int = Query(10, ge=1, le=50)):
    with _db_conn() as conn:
        return {"jobs": db.list_scan_jobs(conn, limit)}


@app.get("/jobs/{job_id}")
def get_job(job_id: UUID):
    with _db_conn() as conn:
        status = db.get_scan_status(conn, job_id)
        if not status:
            raise HTTPException(status_code=404, detail="not found")
        return status


def _enqueue(task: str, **args):
    with _db_conn() as conn:
        task_id = task_queue.enqueue(conn, task, **args)
        conn.commit()
    return {"task_id": str(task_id), "status": "queued"}


@app.post("/rebuild-analysis")
def rebuild_analysis():
    return _enqueue("rebuild_analysis")


@app.post("/rebuild-genres")
def rebuild_genres():
    """Refresh MusicBrainz artist tags, then re-blend every track's genres.

    Audio is re-analysed by a library scan whenever the model version changes.
    """
    with _db_conn() as conn:
        tasks.queue_genre_refresh(conn)
        conn.commit()
    return {"status": "queued"}


@app.get("/genres")
def list_genres():
    """Station genres with track counts, plus a warning when one genre dominates."""
    from genre_scoring import distribution

    with _db_conn() as conn:
        return distribution(conn)


@app.get("/genres/report")
def genre_report():
    """How well the audio model alone agrees with file genre tags."""
    from genre_audio import load_labels
    from genre_scoring import agreement_report

    with _db_conn() as conn:
        return agreement_report(conn, load_labels(GENRE_MODEL_DIR))


@app.get("/tracks/{track_id}/genres")
def track_genres(track_id: UUID):
    with _db_conn() as conn:
        return {"track_id": str(track_id), "genres": db.get_track_station_genres(conn, track_id)}


@app.get("/setup/summary")
def setup_summary():
    with _db_conn() as conn:
        return {"total_tracks": db.count_tracks(conn)}
