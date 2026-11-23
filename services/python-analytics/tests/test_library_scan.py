import shutil
from pathlib import Path

import numpy as np
import psycopg
import pytest
import soundfile as sf
from mutagen.flac import FLAC

from library_scan import scan_library
from library_scan.options import ScanOptions
from library_scan.prune import match_moved_tracks


def write_flac(path: Path, title: str, seconds: float = 1.0, freq: float = 440.0) -> Path:
    path.parent.mkdir(parents=True, exist_ok=True)
    t = np.linspace(0, seconds, int(8000 * seconds), endpoint=False)
    sf.write(path, 0.2 * np.sin(2 * np.pi * freq * t), 8000, format="FLAC")
    tags = FLAC(path)
    tags["title"] = title
    tags["artist"] = "Test Artist"
    tags.save()
    return path


def scan(root: Path):
    return scan_library(root, options=ScanOptions(analyze_audio=False))


def tracks(url: str) -> dict:
    with psycopg.connect(url) as conn:
        return {path: tid for tid, path in conn.execute("SELECT id, path FROM tracks").fetchall()}


def test_match_moved_tracks_requires_unambiguous_match():
    old = {"id": 1, "file_size": 100, "duration_seconds": 10.0}
    new = {"id": 2, "file_size": 100, "duration_seconds": 10.2}
    assert match_moved_tracks([old], [new]) == [(old, new)]

    copy = {"id": 3, "file_size": 100, "duration_seconds": 10.1}
    assert match_moved_tracks([old], [new, copy]) == []

    different = {"id": 4, "file_size": 101, "duration_seconds": 10.0}
    assert match_moved_tracks([old], [different]) == []


def test_scan_imports_loose_root_files_and_subfolders(db_url, tmp_path):
    files = [
        write_flac(tmp_path / "loose.flac", "Loose"),
        write_flac(tmp_path / "Artist" / "single.flac", "Single"),
        write_flac(tmp_path / "Artist" / "Album" / "song.flac", "Song"),
    ]
    scan(tmp_path)
    assert set(tracks(db_url)) == {str(f) for f in files}


def test_underscore_folders_do_not_delete_each_other(db_url, tmp_path):
    underscored = write_flac(tmp_path / "The_Band" / "a.flac", "A")
    spaced = write_flac(tmp_path / "The Band" / "b.flac", "B")
    scan(tmp_path)
    assert set(tracks(db_url)) == {str(underscored), str(spaced)}

    underscored.unlink()
    write_flac(tmp_path / "The_Band" / "c.flac", "C", freq=880.0, seconds=2.0)
    scan(tmp_path)
    assert str(spaced) in tracks(db_url)
    assert str(underscored) not in tracks(db_url)


def test_empty_library_deletes_nothing(db_url, tmp_path):
    write_flac(tmp_path / "Artist" / "song.flac", "Song")
    scan(tmp_path)
    shutil.rmtree(tmp_path / "Artist")

    scan(tmp_path)
    assert len(tracks(db_url)) == 1


@pytest.mark.parametrize("new_relpath", ["Artist/renamed.flac", "Elsewhere/song.flac"])
def test_moved_file_keeps_track_id_and_likes(db_url, tmp_path, new_relpath):
    original = write_flac(tmp_path / "Artist" / "song.flac", "Song")
    write_flac(tmp_path / "Artist" / "other.flac", "Other", freq=660.0, seconds=2.0)
    scan(tmp_path)
    track_id = tracks(db_url)[str(original)]

    with psycopg.connect(db_url) as conn:
        user_id = conn.execute(
            "INSERT INTO users (username, password_hash) VALUES ('u', 'x') RETURNING id"
        ).fetchone()[0]
        conn.execute(
            "INSERT INTO track_feedback (user_id, track_id, feedback) VALUES (%s, %s, 'like')",
            (user_id, track_id),
        )

    moved = tmp_path / new_relpath
    moved.parent.mkdir(parents=True, exist_ok=True)
    original.rename(moved)
    scan(tmp_path)

    after = tracks(db_url)
    assert after[str(moved)] == track_id
    assert str(original) not in after
    with psycopg.connect(db_url) as conn:
        likes = conn.execute("SELECT track_id FROM track_feedback WHERE feedback = 'like'").fetchall()
    assert likes == [(track_id,)]


def drain_queue():
    import db
    import task_queue

    with db.get_conn() as conn:
        while task_queue.run_one(conn):
            pass


def test_queued_scan_finalizes_job_and_prunes(db_url, tmp_path):
    import db
    import tasks

    loose = write_flac(tmp_path / "loose.flac", "Loose")
    nested = write_flac(tmp_path / "Artist" / "song.flac", "Song", freq=550.0, seconds=1.5)
    with db.get_conn() as conn:
        job_id = tasks.queue_library_scan(conn, str(tmp_path))
        conn.commit()

    drain_queue()

    assert set(tracks(db_url)) == {str(loose), str(nested)}
    with db.get_conn() as conn:
        status = db.get_scan_status(conn, job_id)
        leftover = conn.execute("SELECT task, status, error FROM task_queue").fetchall()
    assert status["status"] == "completed"
    assert status["folders"] == {"total": 2, "done": 2, "failed": 0}
    assert status["stats"]["total_files"] == 2
    assert leftover == []

    loose.unlink()
    with db.get_conn() as conn:
        tasks.queue_library_scan(conn, str(tmp_path))
        conn.commit()
    drain_queue()
    assert set(tracks(db_url)) == {str(nested)}


def queue_scan(path: Path):
    import db
    import tasks

    with db.get_conn() as conn:
        job_id = tasks.queue_library_scan(conn, str(path))
        conn.commit()
    return job_id


def folder_jobs(url: str, master) -> list:
    with psycopg.connect(url) as conn:
        return [r[0] for r in conn.execute("SELECT path FROM scan_jobs WHERE parent_id = %s", (master,)).fetchall()]


def scan_planned_folders(url: str, path: Path):
    """Queue and run a scan; return its id and the folders it planned.

    Folder jobs are deleted when the scan ends (ADR 0006), so they are read
    right after trigger_library_scan has run.
    """
    import db
    import task_queue

    job_id = queue_scan(path)
    with db.get_conn() as conn:
        assert task_queue.run_one(conn)  # trigger_library_scan
    planned = folder_jobs(url, job_id)
    drain_queue()
    return job_id, planned


def test_scan_with_no_changes_queues_nothing_and_sends_no_notify(db_url, tmp_path, monkeypatch):
    import db
    import task_queue
    import tasks

    monkeypatch.setattr("config.MUSICBRAINZ_ENABLED", False)
    write_flac(tmp_path / "A" / "a.flac", "A")
    write_flac(tmp_path / "B" / "b.flac", "B", freq=550.0)
    queue_scan(tmp_path)
    drain_queue()

    with psycopg.connect(db_url, autocommit=True) as listener:
        listener.execute(f"LISTEN {tasks.PIPELINE_CHANNEL}")
        job_id = queue_scan(tmp_path)
        with db.get_conn() as conn:
            assert task_queue.run_one(conn)  # trigger_library_scan
            queued = conn.execute("SELECT task FROM task_queue").fetchall()
            status = db.get_scan_status(conn, job_id)
        notices = [n.payload for n in listener.notifies(timeout=1, stop_after=1)]

    assert queued == []
    assert status["status"] == "completed"
    assert status["folders"] == {"total": 0, "done": 0, "failed": 0}
    assert status["stats"]["total_files"] == 2
    assert notices == []


def test_queued_scan_of_one_new_file_imports_one_folder(db_url, tmp_path, monkeypatch):
    monkeypatch.setattr("config.MUSICBRAINZ_ENABLED", False)
    write_flac(tmp_path / "A" / "a.flac", "A")
    write_flac(tmp_path / "B" / "b.flac", "B", freq=550.0)
    queue_scan(tmp_path)
    drain_queue()

    new = write_flac(tmp_path / "C" / "c.flac", "C", freq=660.0)
    _, planned = scan_planned_folders(db_url, tmp_path)

    assert planned == [str(tmp_path / "C")]
    assert str(new) in tracks(db_url)


def test_queued_scan_keeps_track_id_of_file_moved_between_folders(db_url, tmp_path, monkeypatch):
    import db

    monkeypatch.setattr("config.MUSICBRAINZ_ENABLED", False)
    original = write_flac(tmp_path / "A" / "song.flac", "Song")
    write_flac(tmp_path / "A" / "other.flac", "Other", freq=660.0, seconds=2.0)
    queue_scan(tmp_path)
    drain_queue()
    track_id = tracks(db_url)[str(original)]

    moved = tmp_path / "B" / "song.flac"
    moved.parent.mkdir()
    original.rename(moved)
    job_id, planned = scan_planned_folders(db_url, tmp_path)

    after = tracks(db_url)
    assert after[str(moved)] == track_id
    assert str(original) not in after
    # Only the destination is imported; prune sees the source from the full listing.
    assert planned == [str(tmp_path / "B")]
    with db.get_conn() as conn:
        assert db.get_scan_status(conn, job_id)["status"] == "completed"


def test_queued_scan_of_only_deleted_files_prunes_without_folder_tasks(db_url, tmp_path, monkeypatch):
    import db

    monkeypatch.setattr("config.MUSICBRAINZ_ENABLED", False)
    gone = write_flac(tmp_path / "A" / "a.flac", "A")
    kept = write_flac(tmp_path / "B" / "b.flac", "B", freq=550.0)
    queue_scan(tmp_path)
    drain_queue()

    gone.unlink()
    job_id, planned = scan_planned_folders(db_url, tmp_path)

    assert set(tracks(db_url)) == {str(kept)}
    assert planned == []
    with db.get_conn() as conn:
        assert db.get_scan_status(conn, job_id)["status"] == "completed"


def test_scan_whose_planned_folder_changes_nothing_skips_downstream(db_url, tmp_path, monkeypatch):
    """A planned folder whose analysis fails again changes nothing, so no
    analysis refresh or NOTIFY follows."""
    import db
    import task_queue
    import tasks

    monkeypatch.setattr("config.MUSICBRAINZ_ENABLED", False)
    broken = tmp_path / "A" / "broken.flac"
    broken.parent.mkdir()
    broken.write_bytes(b"not audio")
    queue_scan(tmp_path)
    drain_queue()
    # The failure is on record now (#119), so the next scan would skip the
    # file. Forget it, as for a failure recorded before #119, to plan it again.
    with psycopg.connect(db_url) as conn:
        assert conn.execute("SELECT count(*) FROM track_analysis_failures").fetchone()[0] > 0
        conn.execute("DELETE FROM track_analysis_failures")

    with psycopg.connect(db_url, autocommit=True) as listener:
        listener.execute(f"LISTEN {tasks.PIPELINE_CHANNEL}")
        job_id = queue_scan(tmp_path)
        ran = []
        with db.get_conn() as conn:
            while True:
                task = conn.execute("SELECT task FROM task_queue ORDER BY id LIMIT 1").fetchone()
                if not task or not task_queue.run_one(conn):
                    break
                ran.append(task[0])
            status = db.get_scan_status(conn, job_id)
        notices = [n.payload for n in listener.notifies(timeout=1, stop_after=1)]

    assert ran == ["trigger_library_scan", "process_audio_folder", "finalize_scan"]
    assert status["status"] == "completed"
    assert status["stats"]["imported"] == 0
    assert status["stats"]["failed_paths"]
    assert notices == []

    # With the failure on record again, the next scan has nothing to do.
    _, planned = scan_planned_folders(db_url, tmp_path)
    assert planned == []


def run_task(name: str):
    """Run the next queued task, which must be `name`."""
    import db
    import task_queue

    with db.get_conn() as conn:
        task = conn.execute("SELECT task FROM task_queue ORDER BY id LIMIT 1").fetchone()
        assert task and task[0] == name, task
        assert task_queue.run_one(conn)


def masters(url: str) -> list:
    with psycopg.connect(url) as conn:
        return conn.execute(
            "SELECT id, status FROM scan_jobs WHERE parent_id IS NULL ORDER BY created_at"
        ).fetchall()


def test_requests_during_a_running_scan_give_exactly_one_rerun(db_url, tmp_path, monkeypatch):
    import db

    monkeypatch.setattr("config.MUSICBRAINZ_ENABLED", False)
    write_flac(tmp_path / "A" / "a.flac", "A")
    first = queue_scan(tmp_path)
    run_task("trigger_library_scan")

    # A file lands after its folder was planned; three requests come in.
    late = write_flac(tmp_path / "B" / "b.flac", "B", freq=550.0)
    assert {queue_scan(tmp_path) for _ in range(3)} == {first}
    drain_queue()

    jobs = masters(db_url)
    assert [status for _, status in jobs] == ["completed", "completed"]
    assert jobs[0][0] == first
    assert str(late) in tracks(db_url)
    with db.get_conn() as conn:
        assert conn.execute("SELECT count(*) FROM task_queue").fetchone()[0] == 0
        # The rerun ran without asking for another one.
        assert conn.execute("SELECT rerun_requested FROM scan_jobs WHERE id = %s", (jobs[1][0],)).fetchone()[0] is False


def test_request_during_a_scan_that_stops_early_gives_one_rerun(db_url, tmp_path, monkeypatch):
    import db
    import task_queue
    import tasks

    monkeypatch.setattr("config.MUSICBRAINZ_ENABLED", False)
    write_flac(tmp_path / "A" / "a.flac", "A")
    queue_scan(tmp_path)
    drain_queue()

    # Nothing changed, so the scan stops in trigger_library_scan. The
    # requests come in while it is planning (status running, no plan yet).
    first = queue_scan(tmp_path)
    real_plan = tasks.plan_scan

    def plan_with_requests(*args, **kwargs):
        for _ in range(2):
            assert queue_scan(tmp_path) == first
        return real_plan(*args, **kwargs)

    monkeypatch.setattr(tasks, "plan_scan", plan_with_requests)
    run_task("trigger_library_scan")
    monkeypatch.setattr(tasks, "plan_scan", real_plan)

    jobs = masters(db_url)
    assert [status for _, status in jobs][-2:] == ["completed", "pending"]
    assert jobs[-2][0] == first
    with db.get_conn() as conn:
        queued = [r[0] for r in conn.execute("SELECT task FROM task_queue").fetchall()]
    assert queued == ["trigger_library_scan"]

    drain_queue()
    assert [status for _, status in masters(db_url)][-1] == "completed"
    with db.get_conn() as conn:
        assert not task_queue.run_one(conn)


def test_request_during_a_failed_scan_gives_one_rerun(db_url, tmp_path):
    import db

    missing = tmp_path / "gone"
    first = queue_scan(missing)
    with db.get_conn() as conn:
        # As if trigger_library_scan had started: the request flags the scan.
        db.update_scan_job(conn, first, "running")
        conn.commit()
    assert queue_scan(missing) == first
    run_task("trigger_library_scan")

    jobs = masters(db_url)
    assert [status for _, status in jobs] == ["failed", "pending"]
    with db.get_conn() as conn:
        assert db.get_scan_job(conn, first)["error"].startswith("not a directory")
        queued = [r[0] for r in conn.execute("SELECT task FROM task_queue").fetchall()]
    assert queued == ["trigger_library_scan"]


def test_finished_scan_keeps_its_totals_without_its_folder_jobs(db_url, tmp_path, monkeypatch):
    import db

    monkeypatch.setattr("config.MUSICBRAINZ_ENABLED", False)
    write_flac(tmp_path / "A" / "a.flac", "A")
    write_flac(tmp_path / "B" / "b.flac", "B", freq=550.0)
    broken = tmp_path / "C" / "broken.flac"
    broken.parent.mkdir()
    broken.write_bytes(b"not audio")

    job_id = queue_scan(tmp_path)
    run_task("trigger_library_scan")
    with db.get_conn() as conn:
        for _ in range(3):
            run_task("process_audio_folder")
        # Every folder ran; finalize_scan has not. These are the children's totals.
        running = db.get_scan_status(conn, job_id)
    drain_queue()

    with db.get_conn() as conn:
        done = db.get_scan_status(conn, job_id)
        listed = db.list_scan_jobs(conn, 10)[0]
        children = conn.execute("SELECT count(*) FROM scan_jobs WHERE parent_id = %s", (job_id,)).fetchone()[0]
        progress = conn.execute("SELECT count(*) FROM scan_job_progress").fetchone()[0]

    assert children == 0
    assert progress == 1  # only the master's own row is left
    assert done["status"] == "completed"
    assert done["stats"] == running["stats"]
    assert done["stats"]["total_files"] == 3
    assert len(done["stats"]["failed_paths"]) == 1
    assert done["folders"] == running["folders"] == {"total": 3, "done": 3, "failed": 0}
    assert listed["folders"] == done["folders"]
    assert listed["stats"]["failed_files"] == 1
    assert listed["stats"]["imported"] == done["stats"]["imported"] > 0
