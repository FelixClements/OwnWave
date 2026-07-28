from contextlib import contextmanager
from typing import Optional
from uuid import UUID

from fastapi import BackgroundTasks, FastAPI, HTTPException
from pydantic import BaseModel

import db
from config import MUSIC_DIR
from scanner import scan_path
from station_builder import build_station

app = FastAPI(title="OwnWave Analytics")


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


@app.post("/scan")
async def scan(req: ScanRequest, background_tasks: BackgroundTasks):
    with _db_conn() as conn:
        job_id = db.create_scan_job(conn, req.path)
        conn.commit()

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
    filters = {}
    if req.min_bpm is not None:
        filters["min_bpm"] = req.min_bpm
    if req.max_bpm is not None:
        filters["max_bpm"] = req.max_bpm
    if req.min_energy is not None:
        filters["min_energy"] = req.min_energy
    if req.max_energy is not None:
        filters["max_energy"] = req.max_energy

    with _db_conn() as conn:
        station_id = build_station(conn, req.name, seed_filter=filters or None, length=req.length)
        return {"station_id": str(station_id)}


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
