import psycopg


def test_scan_creates_job_and_queues_trigger(db_url, monkeypatch, tmp_path):
    import api

    monkeypatch.setattr(api, "MUSIC_DIR", str(tmp_path))

    resp = api.scan(api.ScanRequest(path=str(tmp_path), force=True))

    assert resp["status"] == "queued"
    with psycopg.connect(db_url) as conn:
        job = conn.execute("SELECT path, status FROM scan_jobs WHERE id = %s", (resp["job_id"],)).fetchone()
        queued = conn.execute("SELECT task, args, status FROM task_queue").fetchall()
    assert job == (str(tmp_path.resolve()), "pending")
    assert queued == [
        (
            "trigger_library_scan",
            {"master_job_id": resp["job_id"], "path": str(tmp_path.resolve()), "force": True},
            "pending",
        )
    ]


def test_rebuild_queues_task(db_url):
    import api

    resp = api.rebuild_analysis()

    with psycopg.connect(db_url) as conn:
        row = conn.execute("SELECT id, task FROM task_queue").fetchone()
    assert resp == {"task_id": str(row[0]), "status": "queued"}
    assert row[1] == "rebuild_analysis"
