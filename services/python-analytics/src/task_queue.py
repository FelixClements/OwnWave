"""Postgres-backed task queue and the worker that drains it.

Tasks are rows in task_queue. Worker processes claim pending rows with
FOR UPDATE SKIP LOCKED, so N processes share one queue without a broker.
A row is 'running' while a worker holds it; rows left 'running' by a killed
worker go back to 'pending' when the worker service starts again. This
assumes a single worker service (any number of processes inside it).
"""

import logging
import multiprocessing
import multiprocessing.connection
import signal
import sys
import threading
import time
from typing import Iterable, Optional

from psycopg.types.json import Jsonb

import db
from config import MUSIC_DIR

log = logging.getLogger(__name__)

# Runs per task, counting retries after an exception and restarts after a
# worker was killed mid-task.
MAX_ATTEMPTS = 3
RETRY_DELAY_SECONDS = 5
IDLE_POLL_SECONDS = 1.0


def enqueue(conn, task: str, **args) -> int:
    """Queue a task inside the caller's transaction; the caller commits."""
    row = conn.execute(
        "INSERT INTO task_queue (task, args) VALUES (%s, %s) RETURNING id",
        (task, Jsonb(args)),
    ).fetchone()
    return row[0]


def enqueue_many(conn, task: str, args_list: Iterable[dict]) -> None:
    with conn.cursor() as cur:
        cur.executemany(
            "INSERT INTO task_queue (task, args) VALUES (%s, %s)",
            [(task, Jsonb(args)) for args in args_list],
        )


def claim(conn) -> Optional[tuple]:
    """Mark the oldest runnable task as running and return (id, task, args, attempts)."""
    row = conn.execute(
        """
        UPDATE task_queue SET status = 'running', attempts = attempts + 1
        WHERE id = (
            SELECT id FROM task_queue
            WHERE status = 'pending' AND run_after <= NOW()
            ORDER BY id
            FOR UPDATE SKIP LOCKED
            LIMIT 1
        )
        RETURNING id, task, args, attempts
        """
    ).fetchone()
    conn.commit()
    return row


def complete(conn, task_id: int) -> None:
    conn.execute("DELETE FROM task_queue WHERE id = %s", (task_id,))
    conn.commit()


def fail(conn, task_id: int, task: str, args: dict, attempts: int, error: str) -> None:
    """Retry a failed task later, or give up once it is out of attempts."""
    if attempts < MAX_ATTEMPTS:
        conn.execute(
            """
            UPDATE task_queue
            SET status = 'pending', error = %s, run_after = NOW() + make_interval(secs => %s)
            WHERE id = %s
            """,
            (error, RETRY_DELAY_SECONDS, task_id),
        )
        conn.commit()
        return
    _give_up(conn, task_id, task, args, error)


def _give_up(conn, task_id: int, task: str, args: dict, error: str) -> None:
    conn.execute("UPDATE task_queue SET status = 'failed', error = %s WHERE id = %s", (error, task_id))
    conn.commit()
    from tasks import ON_FAILURE

    handler = ON_FAILURE.get(task)
    if handler:
        handler(error=error, **args)


def requeue_running(conn) -> int:
    """Return tasks orphaned by a killed worker to the queue. Call before workers start."""
    rows = conn.execute(
        "SELECT id, task, args, attempts FROM task_queue WHERE status = 'running' FOR UPDATE"
    ).fetchall()
    for task_id, task, args, attempts in rows:
        if attempts >= MAX_ATTEMPTS:
            _give_up(conn, task_id, task, args, "worker stopped while running the task")
        else:
            conn.execute("UPDATE task_queue SET status = 'pending' WHERE id = %s", (task_id,))
    conn.commit()
    return len(rows)


def run_one(conn) -> bool:
    """Claim and run one task. Returns False when nothing is runnable."""
    from tasks import TASKS

    row = claim(conn)
    if row is None:
        return False
    task_id, task, args, attempts = row
    log.info("running %s #%d (attempt %d)", task, task_id, attempts)
    try:
        TASKS[task](**args)
    except Exception as exc:
        log.exception("%s #%d failed", task, task_id)
        fail(conn, task_id, task, args, attempts, str(exc) or type(exc).__name__)
    else:
        complete(conn, task_id)
    return True


def _work_loop() -> None:
    stopping = threading.Event()
    signal.signal(signal.SIGTERM, lambda *_: stopping.set())
    signal.signal(signal.SIGINT, lambda *_: stopping.set())

    # A database error outside a task ends this process; the supervisor then
    # exits so the container restarts and requeues whatever was running.
    with db.get_conn() as conn:
        while not stopping.is_set():
            if not run_one(conn):
                stopping.wait(IDLE_POLL_SECONDS)


def run_worker(concurrency: int) -> None:
    """Run `concurrency` worker processes, plus the music folder watcher."""
    db.wait_for_db()
    with db.get_conn() as conn:
        requeued = requeue_running(conn)
    if requeued:
        log.info("requeued %d interrupted tasks", requeued)

    # Fork before starting any threads.
    procs = [multiprocessing.Process(target=_work_loop, name=f"worker-{i}") for i in range(concurrency)]
    for proc in procs:
        proc.start()

    stopping = threading.Event()

    def stop(*_):
        stopping.set()
        for proc in procs:
            if proc.is_alive():
                proc.terminate()  # SIGTERM: finish the current task, then exit

    signal.signal(signal.SIGTERM, stop)
    signal.signal(signal.SIGINT, stop)

    from watcher import start_watching

    try:
        observer = start_watching(MUSIC_DIR)
    except FileNotFoundError as exc:
        log.warning("not watching for changes: %s", exc)
        observer = None

    log.info("worker started with %d processes", concurrency)
    multiprocessing.connection.wait([p.sentinel for p in procs])
    exit_code = 0
    if not stopping.is_set():
        log.error("a worker process exited unexpectedly; stopping")
        exit_code = 1
        stop()
    for proc in procs:
        proc.join()
    if observer:
        observer.stop()
    sys.exit(exit_code)
