import os
import secrets
from contextlib import contextmanager
from typing import List, NoReturn, Optional
from uuid import UUID

from fastapi import BackgroundTasks, FastAPI, HTTPException, Request
from fastapi.responses import JSONResponse
from pydantic import BaseModel

import db
from celery_app import celery_app
from config import ANALYTICS_API_SECRET, CELERY_BROKER_URL, MUSIC_DIR
from library_scan import scan_library
from library_scan.options import ScanJobContext, ScanOptions
from similarity import get_similar_tracks
from station_builder import build_station, setup_main_genre_stations
from station.seed import seed_from_request
from station.service import recompile_station

app = FastAPI(title="OwnWave Analytics")


@app.middleware("http")
async def require_internal_token(request: Request, call_next):
    if request.url.path == "/health":
        return await call_next(request)
    token = request.headers.get("x-internal-token", "")
    if not ANALYTICS_API_SECRET or not secrets.compare_digest(token, ANALYTICS_API_SECRET):
        return JSONResponse({"detail": "unauthorized"}, status_code=401)
    return await call_next(request)


def _user_id_from_request(request: Request) -> UUID:
    raw = request.headers.get("x-ownwave-user-id")
    if not raw:
        raise HTTPException(status_code=400, detail="missing user id")
    try:
        return UUID(raw)
    except ValueError:
        raise HTTPException(status_code=400, detail="invalid user id")


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


class StationRequest(BaseModel):
    name: str
    length: int = 50
    min_bpm: Optional[float] = None
    max_bpm: Optional[float] = None
    min_energy: Optional[float] = None
    max_energy: Optional[float] = None
    min_valence: Optional[float] = None
    max_valence: Optional[float] = None
    # New seed options
    seed_type: Optional[str] = None
    track_id: Optional[UUID] = None
    artist_id: Optional[UUID] = None
    album_id: Optional[UUID] = None
    cluster_id: Optional[int] = None
    main_genre: Optional[str] = None
    sub_genre: Optional[str] = None


class SetupStationsRequest(BaseModel):
    selected_main_genres: List[str]


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


@app.post("/stations")
async def create_station(request: Request, req: StationRequest):
    user_id = _user_id_from_request(request)
    filters = seed_from_request(req)

    with _db_conn() as conn:
        station_id = build_station(
            conn, req.name, user_id, seed_filter=filters or None, length=req.length
        )
        return {"station_id": str(station_id)}


@app.patch("/stations/{station_id}")
async def update_station(request: Request, station_id: UUID, req: StationRequest):
    user_id = _user_id_from_request(request)
    filters = seed_from_request(req)
    has_seed = bool(req.seed_type) or any(
        getattr(req, field) is not None
        for field in (
            "min_bpm",
            "max_bpm",
            "min_energy",
            "max_energy",
            "min_valence",
            "max_valence",
        )
    )
    try:
        with _db_conn() as conn:
            result = recompile_station(
                conn,
                station_id,
                user_id,
                name=req.name,
                seed=filters if has_seed else None,
                length=req.length,
            )
            return result
    except ValueError as exc:
        raise HTTPException(status_code=404 if "not found" in str(exc) else 400, detail=str(exc))


@app.get("/tracks/{track_id}/similar")
async def similar_tracks(track_id: UUID, limit: int = 20):
    with _db_conn() as conn:
        tracks = get_similar_tracks(conn, track_id, limit=limit)
        return {
            "track_id": str(track_id),
            "similar": [
                {"track_id": str(tid), "distance": round(dist, 4)} for tid, dist in tracks
            ],
        }


@app.get("/stations/{station_id}/tracklist")
async def station_tracklist(station_id: UUID):
    with _db_conn() as conn:
        queue = db.get_queue(conn, station_id)
        return {
            "station_id": str(station_id),
            "tracks": [
                {
                    "position": t["position"],
                    "track_id": str(t["id"]),
                    "title": t["title"],
                    "path": t["path"],
                    "bpm": t["bpm"],
                    "key": t["key"],
                    "energy": t["energy"],
                    "valence": t["valence"],
                }
                for t in queue
            ],
        }


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


@app.post("/rebuild-genre-stations")
async def rebuild_genre_stations(request: Request, background_tasks: BackgroundTasks):
    user_id = _user_id_from_request(request)
    if _celery_available():
        from tasks import rebuild_genre_stations

        task = rebuild_genre_stations.delay(str(user_id))
        return {"task_id": task.id, "status": "queued"}
    background_tasks.add_task(_run_rebuild_genre_stations, user_id)
    return {"status": "pending"}


def _run_rebuild_vectors():
    from feature_vector import backfill_library_feature_vectors

    backfill_library_feature_vectors()


def _run_rebuild_genres():
    from tasks import rebuild_track_genres

    rebuild_track_genres(MUSIC_DIR)


def _run_rebuild_genre_stations(user_id: UUID):
    from station_builder import rebuild_genre_stations

    with _db_conn() as conn:
        rebuild_genre_stations(conn, user_id)


def _run_scan(job_id: UUID, path: str, force: bool):
    with _db_conn() as conn:
        db.update_scan_job(conn, job_id, "running")
        conn.commit()
        try:
            result = scan_library(
                path,
                options=ScanOptions(force=force),
                job=ScanJobContext(job_id=job_id),
                conn=conn,
            )
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
        main_genres = db.get_main_genre_counts(conn)
        sub_genres = db.get_sub_genre_counts(conn)
        uncovered = db.get_uncovered_tracks(conn)
        return {
            "total_tracks": total_tracks,
            "main_genres": main_genres,
            "sub_genres": sub_genres,
            "uncovered": len(uncovered),
        }


@app.post("/setup/stations")
async def setup_stations(request: Request, req: SetupStationsRequest):
    user_id = _user_id_from_request(request)
    with _db_conn() as conn:
        result = setup_main_genre_stations(conn, user_id, req.selected_main_genres)
        return result
