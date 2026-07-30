from contextlib import contextmanager
from typing import Optional
from uuid import UUID

from fastapi import BackgroundTasks, FastAPI, HTTPException
from pydantic import BaseModel

import db
from celery_app import celery_app
from config import CELERY_BROKER_URL, MUSIC_DIR
from scanner import scan_path
from similarity import get_similar_tracks
from station_builder import build_station

app = FastAPI(title="OwnWave Analytics")


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
    # New seed options
    seed_type: Optional[str] = None
    track_id: Optional[UUID] = None
    artist_id: Optional[UUID] = None
    album_id: Optional[UUID] = None
    cluster_id: Optional[int] = None
    min_valence: Optional[float] = None
    max_valence: Optional[float] = None


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
    with _db_conn() as conn:
        job_id = db.create_scan_job(conn, req.path)
        conn.commit()

    if _celery_available():
        try:
            from tasks import trigger_library_scan

            trigger_library_scan.delay(str(job_id), req.path, req.force)
            return {"job_id": str(job_id), "status": "queued"}
        except Exception:
            # Redis may be configured but not reachable; fall through.
            pass

    background_tasks.add_task(_run_scan, job_id, req.path, req.force)
    return {"job_id": str(job_id), "status": "pending"}


@app.get("/jobs/{job_id}")
async def get_job(job_id: UUID):
    with _db_conn() as conn:
        with conn.cursor() as cur:
            cur.execute(
                "SELECT id, path, status, error, started_at, finished_at, created_at "
                "FROM scan_jobs WHERE id = %s",
                (job_id,),
            )
            row = cur.fetchone()
            if not row:
                raise HTTPException(status_code=404, detail="not found")
            return {
                "id": str(row[0]),
                "path": row[1],
                "status": row[2],
                "error": row[3],
                "started_at": row[4],
                "finished_at": row[5],
                "created_at": row[6],
            }


@app.post("/stations")
async def create_station(req: StationRequest):
    filters = _seed_filter(req)

    with _db_conn() as conn:
        station_id = build_station(conn, req.name, seed_filter=filters or None, length=req.length)
        return {"station_id": str(station_id)}


def _seed_filter(req: StationRequest) -> dict:
    """Convert API request into the seed filter used by station_builder."""
    if req.seed_type == "track" and req.track_id:
        return {"type": "track", "track_id": str(req.track_id)}
    if req.seed_type == "artist" and req.artist_id:
        return {"type": "artist", "artist_id": str(req.artist_id)}
    if req.seed_type == "album" and req.album_id:
        return {"type": "album", "album_id": str(req.album_id)}
    if req.seed_type == "cluster" and req.cluster_id is not None:
        return {"type": "cluster", "cluster_id": req.cluster_id}
    if req.seed_type == "mood":
        return {
            "type": "mood",
            "min_energy": req.min_energy,
            "max_energy": req.max_energy,
            "min_valence": req.min_valence,
            "max_valence": req.max_valence,
        }

    filters = {}
    if req.min_bpm is not None:
        filters["min_bpm"] = req.min_bpm
    if req.max_bpm is not None:
        filters["max_bpm"] = req.max_bpm
    if req.min_energy is not None:
        filters["min_energy"] = req.min_energy
    if req.max_energy is not None:
        filters["max_energy"] = req.max_energy
    return filters


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


@app.get("/stations/{station_id}/queue")
async def station_queue(station_id: UUID):
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


def _run_rebuild_vectors():
    from feature_vector import backfill_library_feature_vectors

    backfill_library_feature_vectors()


def _run_scan(job_id: UUID, path: str, force: bool):
    with _db_conn() as conn:
        db.update_scan_job(conn, job_id, "running")
        conn.commit()
        try:
            scan_path(path, force=force)
            db.update_scan_job(conn, job_id, "completed")
            conn.commit()
        except Exception as e:
            db.update_scan_job(conn, job_id, "failed", error=str(e))
            conn.commit()
