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


def test_queue_library_scan_reuses_an_active_scan_of_the_same_path(db_url):
    import psycopg

    import tasks

    with psycopg.connect(db_url) as conn:
        first = tasks.queue_library_scan(conn, "/music")
        again = tasks.queue_library_scan(conn, "/music", force=True)
        other = tasks.queue_library_scan(conn, "/music/Some Artist")
        conn.commit()
        assert again == first
        assert other != first
        triggers = conn.execute("SELECT count(*) FROM task_queue WHERE task = 'trigger_library_scan'").fetchone()[0]
        assert triggers == 2

        conn.execute("UPDATE scan_jobs SET status = 'completed' WHERE id = %s", (first,))
        assert tasks.queue_library_scan(conn, "/music") != first


def test_queue_library_scan_flags_a_running_scan_for_one_rerun(db_url):
    import tasks

    with psycopg.connect(db_url) as conn:
        first = tasks.queue_library_scan(conn, "/music")
        flag = "SELECT rerun_requested FROM scan_jobs WHERE id = %s"
        # Pending: it has not read the files yet, so no rerun is needed.
        assert tasks.queue_library_scan(conn, "/music") == first
        assert conn.execute(flag, (first,)).fetchone()[0] is False

        db.update_scan_job(conn, first, "running")
        for _ in range(3):
            assert tasks.queue_library_scan(conn, "/music") == first
        assert conn.execute(flag, (first,)).fetchone()[0] is True

        follow_up = tasks._end_scan(conn, first, "completed")
        assert follow_up not in (None, first)
        # Ending again (a re-run task) changes nothing and queues nothing.
        assert tasks._end_scan(conn, first, "completed") is None
        scans = conn.execute("SELECT id, status FROM scan_jobs ORDER BY created_at").fetchall()
        assert scans == [(first, "completed"), (follow_up, "pending")]
        triggers = conn.execute("SELECT count(*) FROM task_queue WHERE task = 'trigger_library_scan'").fetchone()[0]
        assert triggers == 2


def _finished_scan(conn, created_days_ago: float, status: str = "completed") -> uuid.UUID:
    job = db.create_scan_job(conn, "/music")
    conn.execute(
        "UPDATE scan_jobs SET status = %s, created_at = NOW() - make_interval(secs => %s) WHERE id = %s",
        (status, created_days_ago * 86400, job),
    )
    return job


def test_only_the_newest_100_scans_are_kept(db_url):
    import tasks

    with psycopg.connect(db_url) as conn:
        old = [_finished_scan(conn, 200 - i) for i in range(120)]
        stuck = _finished_scan(conn, 300, status="running")  # active scans are never deleted
        conn.execute("UPDATE scan_jobs SET path = '/other' WHERE id = %s", (stuck,))
        newest = db.create_scan_job(conn, "/music")
        db.update_scan_job(conn, newest, "running")
        tasks._end_scan(conn, newest, "completed")

        kept = {r[0] for r in conn.execute("SELECT id FROM scan_jobs WHERE parent_id IS NULL").fetchall()}
        assert len(kept) == 101
        assert newest in kept and stuck in kept
        assert kept - {newest, stuck} == set(old[-99:])


def _legacy_scan(conn, created_days_ago: float, folders: dict, status: str = "completed") -> uuid.UUID:
    """A scan as written before #120: folder jobs and their progress rows kept."""
    master = _finished_scan(conn, created_days_ago, status)
    db.upsert_scan_job_progress(conn, master, None, ScanResult(total_files=999))
    for name, (job_status, result) in folders.items():
        child = db.create_scan_job(conn, f"/music/{name}", job_id=uuid.uuid4(), parent_id=master)
        db.update_scan_job(conn, child, job_status)
        db.upsert_scan_job_progress(conn, child, master, result)
    return master


def test_cleanup_migration_rolls_up_folder_jobs_and_keeps_100_scans(db_url):
    from pathlib import Path

    sql = (Path(__file__).resolve().parents[3] / "database/migrations/031_scan_job_retention.up.sql").read_text()
    with psycopg.connect(db_url) as conn:
        bad = ScanResult(total_files=1, failed_paths=[FailedPath("/music/b/x.flac", "bad header")])
        scan = _legacy_scan(
            conn,
            1,
            {
                "a": ("completed", ScanResult(total_files=2, imported=2, scanned=2, features=2, model_success=1)),
                "b": ("failed", bad),
                "c": ("completed", ScanResult(total_files=3, scanned=3)),
            },
        )
        before = db.get_scan_status(conn, scan)
        listed_before = db.list_scan_jobs(conn, 1)[0]
        running = _legacy_scan(conn, 0.5, {"d": ("running", ScanResult(total_files=1))}, status="running")
        older = [_legacy_scan(conn, 10 + i, {"x": ("completed", ScanResult(total_files=1))}) for i in range(105)]
        conn.commit()

        conn.execute(sql)
        conn.execute(sql)  # running it twice changes nothing
        conn.commit()

        after = db.get_scan_status(conn, scan)
        assert after["stats"] == before["stats"]
        assert after["stats"]["total_files"] == 6
        assert after["stats"]["imported"] == 2
        assert after["folders"] == before["folders"] == {"total": 3, "done": 2, "failed": 1}
        listed = {j["id"]: j for j in db.list_scan_jobs(conn, 50)}
        assert listed[str(scan)]["stats"] == listed_before["stats"]
        assert listed[str(scan)]["folders"] == listed_before["folders"]

        count = lambda q, *a: conn.execute(q, a).fetchone()[0]
        # Only the running scan keeps its folder job.
        assert count("SELECT count(*) FROM scan_jobs WHERE parent_id IS NOT NULL") == 1
        assert count("SELECT count(*) FROM scan_jobs WHERE parent_id = %s", running) == 1
        assert count("SELECT count(*) FROM scan_jobs WHERE parent_id IS NULL") == 100
        kept = {r[0] for r in conn.execute("SELECT id FROM scan_jobs WHERE parent_id IS NULL").fetchall()}
        assert {scan, running} <= kept
        assert kept - {scan, running} == set(older[:98])
        # A kept old scan's single folder is rolled up too.
        assert db.get_scan_status(conn, older[0])["folders"] == {"total": 1, "done": 1, "failed": 0}
        assert count("SELECT count(*) FROM scan_job_progress WHERE master_job_id IS NOT NULL") == 1


def test_a_scan_task_out_of_attempts_fails_the_scan_and_honours_a_rerun(db_url):
    import tasks

    with psycopg.connect(db_url) as conn:
        first = tasks.queue_library_scan(conn, "/music")
        db.update_scan_job(conn, first, "running")
        tasks.queue_library_scan(conn, "/music")
        conn.commit()

    tasks.ON_FAILURE["finalize_scan"](error="disk gone", master_job_id=str(first), root_path="/music")

    with psycopg.connect(db_url) as conn:
        job = db.get_scan_job(conn, first)
        assert (job["status"], job["error"]) == ("failed", "finalize failed: disk gone")
        scans = conn.execute("SELECT status FROM scan_jobs WHERE id <> %s", (first,)).fetchall()
        assert scans == [("pending",)]
