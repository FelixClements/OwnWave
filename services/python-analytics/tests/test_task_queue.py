import threading
import uuid

import psycopg
import pytest

import db
import task_queue
import tasks


def queue_rows(conn):
    return conn.execute("SELECT task, status, attempts, error FROM task_queue ORDER BY id").fetchall()


def make_runnable(conn):
    conn.execute("UPDATE task_queue SET run_after = NOW()")
    conn.commit()


def test_claim_skips_rows_locked_by_another_worker(db_url):
    with db.get_conn() as conn:
        first = task_queue.enqueue(conn, "a")
        second = task_queue.enqueue(conn, "b")
        conn.commit()

    with db.get_conn() as holder, db.get_conn() as other:
        holder.execute("SELECT id FROM task_queue WHERE id = %s FOR UPDATE", (first,))
        assert task_queue.claim(other)[0] == second
        assert task_queue.claim(other) is None
        holder.rollback()
        assert task_queue.claim(other)[0] == first


def test_failing_task_retries_then_gives_up(db_url, monkeypatch):
    calls, gave_up = [], []

    def boom(**args):
        calls.append(args)
        raise RuntimeError("bad file")

    monkeypatch.setitem(tasks.TASKS, "boom", boom)
    monkeypatch.setitem(tasks.ON_FAILURE, "boom", lambda **kw: gave_up.append(kw))

    with db.get_conn() as conn:
        task_queue.enqueue(conn, "boom", x=1)
        conn.commit()

        assert task_queue.run_one(conn)
        assert queue_rows(conn) == [("boom", "pending", 1, "bad file")]
        # Retries wait RETRY_DELAY_SECONDS before they are runnable again.
        assert not task_queue.run_one(conn)

        for _ in range(task_queue.MAX_ATTEMPTS - 1):
            make_runnable(conn)
            assert task_queue.run_one(conn)

        assert queue_rows(conn) == [("boom", "failed", task_queue.MAX_ATTEMPTS, "bad file")]
        assert not task_queue.run_one(conn)

    assert calls == [{"x": 1}] * task_queue.MAX_ATTEMPTS
    assert gave_up == [{"error": "bad file", "x": 1}]


def test_successful_task_is_removed(db_url, monkeypatch):
    monkeypatch.setitem(tasks.TASKS, "ok", lambda **args: None)
    with db.get_conn() as conn:
        task_queue.enqueue(conn, "ok")
        conn.commit()
        assert task_queue.run_one(conn)
        assert queue_rows(conn) == []


def test_requeue_running_recovers_interrupted_tasks(db_url, monkeypatch):
    gave_up = []
    monkeypatch.setitem(tasks.ON_FAILURE, "stuck", lambda **kw: gave_up.append(kw))

    with db.get_conn() as conn:
        task_queue.enqueue(conn, "interrupted")
        task_queue.enqueue(conn, "stuck", n=2)
        conn.commit()
        task_queue.claim(conn)
        task_queue.claim(conn)
        conn.execute("UPDATE task_queue SET attempts = %s WHERE task = 'stuck'", (task_queue.MAX_ATTEMPTS,))
        conn.commit()

        assert task_queue.requeue_running(conn) == 2
        assert queue_rows(conn) == [
            ("interrupted", "pending", 1, None),
            ("stuck", "failed", task_queue.MAX_ATTEMPTS, "worker stopped while running the task"),
        ]
    assert gave_up == [{"error": "worker stopped while running the task", "n": 2}]


def scan_with_folders(conn, n):
    master = db.create_scan_job(conn, "/music")
    db.update_scan_job(conn, master, "running")
    children = [db.create_scan_job(conn, f"/music/{i}", job_id=uuid.uuid4(), parent_id=master) for i in range(n)]
    for child in children:
        db.update_scan_job(conn, child, "running")
    conn.commit()
    return master, children


def finalize_rows(conn, master):
    return conn.execute(
        "SELECT COUNT(*) FROM task_queue WHERE task = 'finalize_scan' AND args->>'master_job_id' = %s",
        (str(master),),
    ).fetchone()[0]


def test_last_folder_queues_finalize_once(db_url):
    with db.get_conn() as conn:
        master, (a, b) = scan_with_folders(conn, 2)

        tasks._finish_folder(a, master, "/music", "completed")
        assert finalize_rows(conn, master) == 0
        tasks._finish_folder(b, master, "/music", "failed", error="boom")
        assert finalize_rows(conn, master) == 1
        # A repeated finish (task re-run after a crash) changes nothing.
        tasks._finish_folder(b, master, "/music", "completed")
        assert finalize_rows(conn, master) == 1


@pytest.mark.parametrize("round_", range(10))
def test_concurrent_finishers_queue_finalize_once(db_url, round_):
    with db.get_conn() as conn:
        master, children = scan_with_folders(conn, 4)

    barrier = threading.Barrier(len(children))

    def finish(child):
        barrier.wait()
        tasks._finish_folder(child, master, "/music", "completed")

    threads = [threading.Thread(target=finish, args=(c,)) for c in children]
    for t in threads:
        t.start()
    for t in threads:
        t.join()

    with psycopg.connect(db_url) as conn:
        assert finalize_rows(conn, master) == 1


def test_trigger_rerun_does_not_duplicate_folders(db_url, tmp_path):
    (tmp_path / "a").mkdir()
    (tmp_path / "a" / "x.flac").write_bytes(b"")
    with db.get_conn() as conn:
        master = db.create_scan_job(conn, str(tmp_path))
        conn.commit()

    assert tasks.trigger_library_scan(str(master), str(tmp_path))["dispatched"] == 1
    assert tasks.trigger_library_scan(str(master), str(tmp_path))["dispatched"] == 0

    with db.get_conn() as conn:
        assert conn.execute("SELECT COUNT(*) FROM task_queue WHERE task = 'process_audio_folder'").fetchone()[0] == 1
