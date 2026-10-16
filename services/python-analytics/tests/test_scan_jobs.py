import uuid

import psycopg

import db
from models import FailedPath, ScanResult


def test_scan_status_counts_folders_and_history_lists_only_scans(db_url):
    with psycopg.connect(db_url) as conn:
        master = db.create_scan_job(conn, "/music")
        db.update_scan_job(conn, master, "running")
        folders = {name: uuid.uuid4() for name in ("a", "b", "c")}
        for name, job_id in folders.items():
            db.create_scan_job(conn, f"/music/{name}", job_id=job_id, parent_id=master)

        db.update_scan_job(conn, folders["a"], "completed")
        db.upsert_scan_job_progress(conn, folders["a"], master, ScanResult(total_files=2, imported=2))
        db.update_scan_job(conn, folders["b"], "failed", error="boom")
        db.upsert_scan_job_progress(
            conn,
            folders["b"],
            master,
            ScanResult(total_files=1, failed_paths=[FailedPath("/music/b/x.flac", "bad header")]),
        )
        # A folder task re-registering itself (retry) keeps its parent.
        db.create_scan_job(conn, "/music/c", job_id=folders["c"])

        status = db.get_scan_status(conn, master)
        assert status["status"] == "running"
        assert status["folders"] == {"total": 3, "done": 1, "failed": 1}
        assert status["stats"]["total_files"] == 3
        assert status["stats"]["imported"] == 2
        assert status["stats"]["failed_paths"] == [{"path": "/music/b/x.flac", "error": "bad header"}]

        older = db.create_scan_job(conn, "/music")
        conn.execute("UPDATE scan_jobs SET created_at = NOW() - INTERVAL '1 day' WHERE id = %s", (older,))

        jobs = db.list_scan_jobs(conn, 10)
        assert [j["id"] for j in jobs] == [str(master), str(older)]
        assert jobs[0]["stats"]["failed_files"] == 1
        assert "failed_paths" not in jobs[0]["stats"]
        assert jobs[0]["folders"]["total"] == 3
        assert db.list_scan_jobs(conn, 1)[0]["id"] == str(master)


def test_scan_without_folders_reports_its_own_progress(db_url):
    with psycopg.connect(db_url) as conn:
        master = db.create_scan_job(conn, "/music")
        db.upsert_scan_job_progress(conn, master, None, ScanResult(total_files=0))
        db.update_scan_job(conn, master, "completed")

        status = db.get_scan_status(conn, master)
        assert status["folders"] == {"total": 0, "done": 0, "failed": 0}
        assert status["stats"]["total_files"] == 0
