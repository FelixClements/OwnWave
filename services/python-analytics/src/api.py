import os
import secrets
from contextlib import asynccontextmanager, contextmanager
from typing import Any, NoReturn
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
    # None scans the whole library: this service's own MUSIC_DIR.
    path: str | None = None
    force: bool = False


# Response models. Go proxies these routes unchanged; openapi.yaml documents
# them, and tests/test_openapi_contract.py checks the two agree.
class ScanQueued(BaseModel):
    job_id: str
    status: str


class TaskQueued(BaseModel):
    task_id: str
    status: str


class Queued(BaseModel):
    status: str


class GenreCount(BaseModel):
    genre: str
    name: str
    primary_tracks: int
    tracks: int
    station: bool


class GenreDistribution(BaseModel):
    scored_tracks: int
    genres: list[GenreCount]
    warnings: list[str]


class TrackGenre(BaseModel):
    genre: str
    score: float
    is_primary: bool


class TrackGenres(BaseModel):
    track_id: str
    genres: list[TrackGenre]


class SetupSummary(BaseModel):
    total_tracks: int


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


@app.post("/scan", response_model=ScanQueued)
def scan(req: ScanRequest | None = None):
    req = req or ScanRequest()
    scan_path = _resolved_scan_path(req.path or MUSIC_DIR)
    with _db_conn() as conn:
        job_id = tasks.queue_library_scan(conn, scan_path, req.force)
        conn.commit()
    return {"job_id": str(job_id), "status": "queued"}


@app.get("/jobs", response_model=dict[str, Any])
def list_jobs(limit: int = Query(10, ge=1, le=50)):
    with _db_conn() as conn:
        return {"jobs": db.list_scan_jobs(conn, limit)}


@app.get("/jobs/{job_id}", response_model=dict[str, Any])
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


@app.post("/rebuild-analysis", response_model=TaskQueued)
def rebuild_analysis():
    return _enqueue("rebuild_analysis")


@app.post("/rebuild-genres", response_model=Queued)
def rebuild_genres():
    """Refresh MusicBrainz artist tags, then re-blend every track's genres.

    Audio is re-analysed by a library scan whenever the model version changes.
    """
    with _db_conn() as conn:
        tasks.queue_genre_refresh(conn)
        conn.commit()
    return {"status": "queued"}


@app.post("/outside-genres/refetch", response_model=Queued)
def refetch_outside_genres():
    """Fetch every outside genre (MusicBrainz, Discogs, Last.fm) again, in the background.

    Lookups are otherwise repeated only once they are about 6 months old.
    """
    from outside_genres import mark_all_due

    with _db_conn() as conn:
        mark_all_due(conn)
        tasks.queue_outside_genres(conn)
        conn.commit()
    return {"status": "queued"}


@app.get("/genres", response_model=GenreDistribution)
def list_genres():
    """Station genres with track counts, plus a warning when one genre dominates."""
    from genre_scoring import distribution

    with _db_conn() as conn:
        return distribution(conn)


@app.get("/genres/report", response_model=dict[str, Any])
def genre_report():
    """How well the audio model alone agrees with file genre tags."""
    from genre_audio import load_labels
    from genre_scoring import agreement_report

    with _db_conn() as conn:
        return agreement_report(conn, load_labels(GENRE_MODEL_DIR))


@app.get("/tracks/{track_id}/genres", response_model=TrackGenres)
def track_genres(track_id: UUID):
    with _db_conn() as conn:
        return {"track_id": str(track_id), "genres": db.get_track_station_genres(conn, track_id)}


@app.get("/setup/summary", response_model=SetupSummary)
def setup_summary():
    with _db_conn() as conn:
        return {"total_tracks": db.count_tracks(conn)}
