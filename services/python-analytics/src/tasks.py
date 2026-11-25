import logging
from pathlib import Path
from typing import Optional
from uuid import UUID, uuid4

from psycopg.types.json import Jsonb

import db
import genre_audio
import task_queue
from folder_importer import import_folder
from library_scan.plan import plan_scan
from library_scan.prune import prune_library
from models import ScanResult

log = logging.getLogger(__name__)


# Serialises scan creation so two requests cannot both find no active scan.
_SCAN_LOCK = 4_242_002

# The app_state key score_genres writes (ADR 0005). Go reads it as
# settings.GenresScored in internal/settings.
GENRES_SCORED_KEY = "genres_scored"


def active_scan(conn, path: str) -> Optional[tuple]:
    """The (id, status) of the pending or running top-level scan of path, if any."""
    return conn.execute(
        """
        SELECT id, status FROM scan_jobs
        WHERE parent_id IS NULL AND path = %s AND status IN ('pending', 'running')
        ORDER BY created_at DESC LIMIT 1
        """,
        (path,),
    ).fetchone()


def queue_library_scan(conn, path: str, force: bool = False) -> UUID:
    """Create a scan job and queue it inside the caller's transaction.

    Requests are coalesced (ADR 0006):
    - a pending scan of the same path is returned as it is: it has not
      looked at the files yet;
    - a running one is returned with rerun_requested set, so it queues one
      follow-up scan when it ends, however many requests came in;
    - otherwise a new scan is created.

    The advisory lock serialises this with _end_scan, so a request either
    sets the flag before the scan ends or finds no active scan after.
    """
    conn.execute("SELECT pg_advisory_xact_lock(%s)", (_SCAN_LOCK,))
    existing = active_scan(conn, path)
    if existing:
        job_id, status = existing
        if status == "running":
            conn.execute("UPDATE scan_jobs SET rerun_requested = true WHERE id = %s", (job_id,))
        return job_id
    job_id = db.create_scan_job(conn, path)
    task_queue.enqueue(conn, "trigger_library_scan", master_job_id=str(job_id), path=path, force=force)
    return job_id


def _end_scan(conn, master: UUID, status: str, error: Optional[str] = None) -> Optional[UUID]:
    """Finish a scan (master job) inside the caller's transaction.

    Every place a scan ends calls this: the early stop and the bad root in
    trigger_library_scan, finalize_scan, rebuild_analysis, and the give-up
    handlers. It
    - marks the master completed or failed, only if it is still active, so
      a re-run ends nothing twice;
    - folds its folder jobs into its own progress row and deletes them;
    - deletes finished scans beyond the newest db.SCAN_JOBS_KEPT;
    - queues one follow-up scan if a request came in while it ran.

    The advisory lock is taken before the master row is touched: a request
    in queue_library_scan holds it while it waits for that row, so the other
    order would deadlock. Returns the follow-up scan's id, if one was queued.
    """
    conn.execute("SELECT pg_advisory_xact_lock(%s)", (_SCAN_LOCK,))
    row = conn.execute(
        """
        UPDATE scan_jobs SET status = %s, error = %s, finished_at = NOW()
        WHERE id = %s AND status IN ('pending', 'running')
        RETURNING path, rerun_requested
        """,
        (status, error, master),
    ).fetchone()
    if row is None:
        return None
    path, rerun = row
    db.roll_up_scan(conn, master)
    db.prune_scan_jobs(conn)
    if not rerun:
        return None
    follow_up = queue_library_scan(conn, path)
    log.info("scan %s ended with a rerun requested: queued scan %s", master, follow_up)
    return follow_up


def _scan_dispatched(conn, master: UUID) -> bool:
    """Whether trigger_library_scan already committed its work for this scan.

    Its last commit writes the master's own progress row (and, before scan
    plans, created the folder jobs), so a re-run after the worker died
    between committing and acknowledging finds one of them.
    """
    job = db.get_scan_job(conn, master)
    if job is None or job["status"] in ("completed", "failed"):
        return True
    return bool(
        conn.execute(
            """
            SELECT 1 FROM scan_job_progress WHERE job_id = %s
            UNION ALL
            SELECT 1 FROM scan_jobs WHERE parent_id = %s
            LIMIT 1
            """,
            (master, master),
        ).fetchone()
    )


def trigger_library_scan(master_job_id: str, path: str, force: bool = False) -> dict:
    """Plan the scan, then queue one import task per folder that needs work.

    The last folder task to finish queues finalize_scan. A plan with nothing
    to import and no missing track completes the scan here (ADR 0006).
    """
    master = UUID(master_job_id)
    nothing = {"master_job_id": master_job_id, "dispatched": 0}
    with db.get_conn() as conn:
        if _scan_dispatched(conn, master):
            return nothing
        db.update_scan_job(conn, master, "running")
        conn.commit()

        root = Path(path).expanduser().resolve()
        if not root.is_dir():
            _end_scan(conn, master, "failed", error=f"not a directory: {root}")
            conn.commit()
            return nothing

        genre_version = genre_audio.MODEL_VERSION if genre_audio.get_model() else None
        plan = plan_scan(conn, root, force=force, genre_version=genre_version)
        conn.commit()

        if plan.empty:
            # _end_scan's lock order: the scan lock before the master row.
            conn.execute("SELECT pg_advisory_xact_lock(%s)", (_SCAN_LOCK,))
        conn.execute("SELECT 1 FROM scan_jobs WHERE id = %s FOR UPDATE", (master,))
        if _scan_dispatched(conn, master):
            conn.rollback()
            return nothing
        db.upsert_scan_job_progress(conn, master, None, ScanResult(total_files=len(plan.files)))

        if plan.empty:
            _end_scan(conn, master, "completed")
        elif not plan.folders:
            # Only vanished files: prune them, nothing to import.
            task_queue.enqueue(conn, "finalize_scan", master_job_id=master_job_id, root_path=str(root))
        else:
            # Folder jobs and their tasks are created together so progress has
            # a fixed denominator and every folder job has a task to finish it.
            folder_args = []
            for folder, files in plan.folders.items():
                job_id = db.create_scan_job(conn, folder, job_id=uuid4(), parent_id=master)
                folder_args.append(
                    {
                        "folder_path": folder,
                        "job_id": str(job_id),
                        "master_job_id": master_job_id,
                        "root_path": str(root),
                        "force": force,
                        "files": [list(f) for f in files],
                    }
                )
            task_queue.enqueue_many(conn, "process_audio_folder", folder_args)
        conn.commit()

    return {"master_job_id": master_job_id, "dispatched": len(plan.folders), "missing": plan.missing}


def process_audio_folder(
    folder_path: str,
    job_id: str,
    master_job_id: str,
    root_path: str,
    force: bool = False,
    files: Optional[list] = None,
) -> dict:
    """Import the planned files of one folder. Raising lets the queue retry it."""
    jid = UUID(job_id)
    with db.get_conn() as conn:
        job = db.get_scan_job(conn, jid)
        if job is None or job["status"] in ("completed", "failed"):
            # Re-run after the worker died between finishing and acknowledging
            # (the job is gone once its scan ended and was rolled up).
            return {"folder": folder_path, "job_id": job_id, "skipped": True}
        db.update_scan_job(conn, jid, "running")
        conn.commit()

    result = import_folder(
        folder_path, job_id=jid, master_job_id=UUID(master_job_id), analyze=True, force=force, files=files
    )
    _finish_folder(jid, UUID(master_job_id), root_path, "completed")
    return {"folder": folder_path, "tracks": len(result.track_ids), "job_id": job_id}


def _process_audio_folder_failed(error: str, job_id: str, master_job_id: str, root_path: str, **_) -> None:
    _finish_folder(UUID(job_id), UUID(master_job_id), root_path, "failed", error=error)


def _finish_folder(job_id: UUID, master: UUID, root_path: str, status: str, error: Optional[str] = None) -> None:
    """Record a folder's final status; the last folder of the scan queues finalize_scan.

    Each finisher locks the master job before counting, so under READ
    COMMITTED exactly one of them sees no unfinished folders left.
    """
    with db.get_conn() as conn:
        changed = conn.execute(
            """
            UPDATE scan_jobs SET status = %s, error = %s, finished_at = NOW()
            WHERE id = %s AND status IN ('pending', 'running')
            """,
            (status, error, job_id),
        ).rowcount
        if not changed:
            return
        conn.execute("SELECT 1 FROM scan_jobs WHERE id = %s FOR UPDATE", (master,))
        unfinished = conn.execute(
            "SELECT COUNT(*) FROM scan_jobs WHERE parent_id = %s AND status IN ('pending', 'running')",
            (master,),
        ).fetchone()[0]
        if unfinished == 0:
            task_queue.enqueue(conn, "finalize_scan", master_job_id=str(master), root_path=root_path)
        conn.commit()


# Progress counters that mean a folder task changed the database.
_CHANGE_COUNTERS = ("imported", "features", "model_success")


def finalize_scan(master_job_id: str, root_path: str) -> dict:
    """Prune moved/deleted tracks, then refresh analysis if anything changed.

    When something changed the master job stays running until
    rebuild_analysis completes it. When nothing did (every planned file
    failed or was already up to date), the scan completes here with no
    analysis refresh and no NOTIFY. A failed prune raises so the queue
    retries it; the give-up handler fails the scan.
    """
    job_id = UUID(master_job_id)

    with db.get_conn() as conn:
        job = db.get_scan_job(conn, job_id)
        if job is None or job["status"] in ("completed", "failed"):
            # Re-run after the worker died between finishing and acknowledging.
            return {"master_job_id": master_job_id, "skipped": True}

        total, failed = conn.execute(
            "SELECT COUNT(*), COUNT(*) FILTER (WHERE status = 'failed') FROM scan_jobs WHERE parent_id = %s",
            (job_id,),
        ).fetchone()
        stats = db.scan_stats(conn, job_id)
        summary = {"master_job_id": master_job_id, "total": total, "failed": failed}

        if not failed:
            # Skipped when a folder failed: a moved file whose new folder
            # failed to import would otherwise lose its history.
            try:
                summary.update(prune_library(conn, Path(root_path), _job_started_at(conn, job_id)))
            except Exception:
                conn.rollback()
                raise

        changed = any(stats[k] for k in _CHANGE_COUNTERS) or summary.get("moved") or summary.get("deleted")
        summary["changed"] = bool(changed)
        if changed:
            # A failed scan ends here; still refresh analysis for the folders
            # that did import.
            task_queue.enqueue(conn, "rebuild_analysis", scan_job_id=None if failed else master_job_id)
        if failed:
            _end_scan(conn, job_id, "failed", error=f"{failed} / {total} folder tasks failed")
        elif not changed:
            _end_scan(conn, job_id, "completed")
        conn.commit()
    log.info("scan %s finalized: %s", master_job_id, summary)
    return summary


def _job_started_at(conn, job_id: UUID):
    with conn.cursor() as cur:
        cur.execute("SELECT started_at FROM scan_jobs WHERE id = %s", (job_id,))
        row = cur.fetchone()
        return row[0] if row else None


# Postgres channel the Go API listens on. It matches playlist tracks and
# refreshes the genre stations whenever a step posts here.
PIPELINE_CHANNEL = "ownwave_pipeline"


def notify_pipeline(conn, step: str) -> None:
    """Tell the API a pipeline step finished. Sent on commit, with the work."""
    conn.execute("SELECT pg_notify(%s, %s)", (PIPELINE_CHANNEL, step))


def rebuild_analysis(scan_job_id: Optional[str] = None) -> dict:
    """Last step of a scan: complete its job, then queue the genre refresh.

    The API matches playlist tracks to the new files right away; genre
    stations follow once the genres are scored.
    """
    with db.get_conn() as conn:
        if scan_job_id:
            _end_scan(conn, UUID(scan_job_id), "completed")
        queue_genre_refresh(conn)
        notify_pipeline(conn, "scan_finished")
        conn.commit()
    return {"scan_job_id": scan_job_id}


# Artist names looked up per fetch_artist_genres run (about 1.1 s each).
MB_BATCH = 150
# Stop a run (and score anyway) after this many failed lookups.
MB_MAX_FAILURES = 5
# Advisory lock id: only one MusicBrainz fetcher may run (shared rate limit).
_MB_LOCK = 4_242_001
# One fetcher per outside genre source; MusicBrainz shares the artist tag lock.
_SOURCE_LOCKS = {"musicbrainz": _MB_LOCK, "discogs": 4_242_002, "lastfm": 4_242_003}
# A fetch that finds its source's lock taken tries again after this long.
LOCK_RETRY_SECONDS = 60


def queue_genre_refresh(conn) -> None:
    """Queue the genre steps that follow new analysis.

    Artist tags, then scoring. The outside genre fetch runs beside them;
    scoring does not wait for it, and it queues a rescore when it brings new
    data.
    """
    from config import MUSICBRAINZ_ENABLED

    if MUSICBRAINZ_ENABLED:
        task_queue.enqueue(conn, "fetch_artist_genres")
    else:
        task_queue.enqueue(conn, "score_genres")
    queue_outside_genres(conn)


def queue_outside_genres(conn, sources: Optional[list] = None) -> list:
    """Queue a fetch for each switched-on source that has none queued or running."""
    from outside_genres import enabled_sources

    queued = []
    for source in enabled_sources():
        if sources is not None and source not in sources:
            continue
        busy = conn.execute(
            """
            SELECT 1 FROM task_queue
            WHERE task = 'fetch_outside_genres' AND args->>'source' = %s
              AND status IN ('pending', 'running')
            LIMIT 1
            """,
            (source,),
        ).fetchone()
        if not busy:
            task_queue.enqueue(conn, "fetch_outside_genres", source=source)
            queued.append(source)
    return queued


def fetch_outside_genres(source: str, new_data: int = 0) -> dict:
    """Fetch genres from one outside source by ID, one request budget per run.

    A run queues the next run while lookups are due, and a rescore once the
    source is done if any run brought new genres (new_data carries the count
    along the chain). New Discogs links found on MusicBrainz releases start
    the Discogs fetch.
    """
    import outside_genres as og
    from config import OUTSIDE_GENRES_MAX_AGE_DAYS

    if source not in og.enabled_sources():
        return {"source": source, "skipped": "switched off"}
    lock = _SOURCE_LOCKS[source]
    with db.get_conn() as conn:
        if not conn.execute("SELECT pg_try_advisory_lock(%s)", (lock,)).fetchone()[0]:
            # Another fetch holds the source's rate limit (for MusicBrainz,
            # possibly the artist tag fetch): try again shortly.
            task_queue.enqueue_later(conn, LOCK_RETRY_SECONDS, "fetch_outside_genres", source=source, new_data=new_data)
            conn.commit()
            return {"source": source, "skipped": "another fetch is running"}
        try:
            summary = og.fetch_due(conn, source, og.make_client(source), OUTSIDE_GENRES_MAX_AGE_DAYS)
            total = new_data + summary.new_data
            if summary.more:
                task_queue.enqueue(conn, "fetch_outside_genres", source=source, new_data=total)
            elif total:
                task_queue.enqueue(conn, "score_genres")
            if summary.discogs_links:
                queue_outside_genres(conn, [og.DISCOGS])
            conn.commit()
        finally:
            conn.execute("SELECT pg_advisory_unlock(%s)", (lock,))
            conn.commit()
    log.info("outside genres from %s: %s", source, summary.as_dict())
    return {"source": source, **summary.as_dict()}


def fetch_artist_genres() -> dict:
    """Look up MusicBrainz tags for artists not looked up yet, in batches.

    A run handles MB_BATCH names, then queues the next run, or score_genres
    when no names are left.
    """
    from config import MUSICBRAINZ_URL, MUSICBRAINZ_USER_AGENT
    from musicbrainz import MusicBrainzClient, name_key, primary_artist

    client = MusicBrainzClient(MUSICBRAINZ_URL, MUSICBRAINZ_USER_AGENT)
    with db.get_conn() as conn:
        if not conn.execute("SELECT pg_try_advisory_lock(%s)", (_MB_LOCK,)).fetchone()[0]:
            # The outside genre fetch holds MusicBrainz's rate limit: try
            # again shortly, so the chain still ends in score_genres.
            task_queue.enqueue_later(conn, LOCK_RETRY_SECONDS, "fetch_artist_genres")
            conn.commit()
            return {"skipped": "another fetch is running"}
        try:
            done = {r[0] for r in conn.execute("SELECT name_key FROM musicbrainz_artists").fetchall()}
            credits = [r[0] for r in conn.execute(
                "SELECT DISTINCT a.name FROM artists a JOIN tracks t ON t.artist_id = a.id ORDER BY 1"
            ).fetchall()]
            pending = [c for c in credits if name_key(c) and name_key(c) not in done
                       and name_key(c) != "unknown artist"]
            looked_up = failures = 0
            for credit in pending[:MB_BATCH]:
                if failures >= MB_MAX_FAILURES:
                    break
                names = [credit]
                primary = primary_artist(credit)
                if name_key(primary) != name_key(credit):
                    names.append(primary)
                for name in names:
                    key = name_key(name)
                    if key in done:
                        continue
                    try:
                        found = client.lookup_artist(name)
                    except Exception as exc:
                        failures += 1
                        log.warning("MusicBrainz lookup failed for %r: %s", name, exc)
                        break
                    looked_up += 1
                    conn.execute(
                        """
                        INSERT INTO musicbrainz_artists (name_key, mbid, tags, fetched_at)
                        VALUES (%s, %s, %s, NOW())
                        ON CONFLICT (name_key) DO UPDATE SET
                            mbid = EXCLUDED.mbid, tags = EXCLUDED.tags, fetched_at = NOW()
                        """,
                        (key, found.mbid, Jsonb(found.tags)),
                    )
                    conn.commit()
                    done.add(key)
                    if found.mbid:
                        break  # the whole credit matched; no need for the primary artist
            remaining = max(0, len(pending) - MB_BATCH)
            # MusicBrainz unreachable: score with what we have; failed names
            # are retried on the next refresh.
            more = remaining and failures < MB_MAX_FAILURES
            task_queue.enqueue(conn, "fetch_artist_genres" if more else "score_genres")
            conn.commit()
        finally:
            conn.execute("SELECT pg_advisory_unlock(%s)", (_MB_LOCK,))
            conn.commit()
    return {"looked_up": looked_up, "failures": failures, "remaining": remaining}


def score_genres() -> dict:
    """Decide every track's genre into track_station_genres for the whole library.

    Uses the genre engine v2 (genre_engine) once the genre v2 switch
    migration is in, the v1 blend (genre_scoring) until then: the new engine
    replaces the old one in one release (ADR 0008 decision 6).

    Records when scoring ran (admin pipeline view) and tells the API, which
    then refreshes the genre stations.
    """
    import genre_engine
    import genre_scoring
    from config import GENRE_MODEL_DIR
    from genre_audio import load_labels

    with db.get_conn() as conn:
        engine = genre_engine if genre_engine.switched(conn) else genre_scoring
        summary = engine.score_library(conn, load_labels(GENRE_MODEL_DIR))
        conn.execute(
            """
            INSERT INTO app_state (key, value, updated_at) VALUES (%s, %s, NOW())
            ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW()
            """,
            (GENRES_SCORED_KEY, Jsonb({"tracks": summary["tracks"], "scored": summary["scored"]}),),
        )
        notify_pipeline(conn, "genres_scored")
        conn.commit()
    return summary


TASKS = {
    fn.__name__: fn
    for fn in (
        trigger_library_scan,
        process_audio_folder,
        finalize_scan,
        rebuild_analysis,
        fetch_artist_genres,
        fetch_outside_genres,
        score_genres,
    )
}

def _scan_task_failed(error: str, master_job_id: str, **_) -> None:
    """A scan's own task is out of attempts: fail the scan so it ends.

    Without this a scan whose trigger or finalize task gave up stays
    running, and every later request would only flag it for a rerun.
    """
    with db.get_conn() as conn:
        _end_scan(conn, UUID(master_job_id), "failed", error=error)
        conn.commit()


def _rebuild_analysis_failed(error: str, scan_job_id: Optional[str] = None, **_) -> None:
    if scan_job_id:
        _scan_task_failed(f"analysis refresh failed: {error}", scan_job_id)


# Called with the error and the task's args once a task is out of attempts.
ON_FAILURE = {
    "process_audio_folder": _process_audio_folder_failed,
    "trigger_library_scan": _scan_task_failed,
    "finalize_scan": lambda error, **args: _scan_task_failed(f"finalize failed: {error}", **args),
    "rebuild_analysis": _rebuild_analysis_failed,
}
