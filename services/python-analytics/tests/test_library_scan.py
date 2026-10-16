import shutil
from pathlib import Path

import numpy as np
import psycopg
import pytest
import soundfile as sf
from mutagen.flac import FLAC

from library_scan import scan_library
from library_scan.options import ScanOptions
from library_scan.prune import match_moved_tracks, music_dirs


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


def test_music_dirs_includes_folders_with_loose_files(tmp_path):
    write_flac(tmp_path / "loose.flac", "Loose")
    write_flac(tmp_path / "Artist" / "single.flac", "Single")
    write_flac(tmp_path / "Artist" / "Album" / "song.flac", "Song")

    assert music_dirs(tmp_path) == sorted(
        [str(tmp_path), str(tmp_path / "Artist"), str(tmp_path / "Artist" / "Album")]
    )


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
