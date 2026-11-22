"""The scan plan: one walk and one query decide which folders a scan imports (ADR 0006)."""

import os
from pathlib import Path

import psycopg

import db
from audio_metadata import file_mtime
from library_scan import scan_library
from library_scan.options import ScanOptions
from library_scan.plan import needs_import, plan_scan, walk_audio_files
from test_library_scan import write_flac

GENRE = "genre-model-2"
STAT = (100, 1_700_000_000.25)


def track(**overrides) -> dict:
    t = {"file_size": 100, "file_mtime": file_mtime(STAT[1]), "has_features": True, "genre_version": GENRE}
    t.update(overrides)
    return t


def needs(stat=STAT, existing="default", force=False, analyze=True, genre_version=GENRE) -> bool:
    return needs_import(
        stat, track() if existing == "default" else existing, force=force, analyze=analyze, genre_version=genre_version
    )


def test_unchanged_file_needs_nothing():
    assert not needs()


def test_new_file_needs_import():
    assert needs(existing=None)


def test_changed_size_or_mtime_needs_import():
    assert needs(stat=(101, STAT[1]))
    assert needs(stat=(100, STAT[1] + 1))


def test_missing_features_need_import_only_when_analysing():
    assert needs(existing=track(has_features=False))
    assert not needs(existing=track(has_features=False), analyze=False)


def test_old_genre_version_needs_import_only_with_the_model_on():
    assert needs(existing=track(genre_version="genre-model-1"))
    assert needs(existing=track(genre_version=None))
    assert not needs(existing=track(genre_version="genre-model-1"), genre_version=None)


def test_failed_features_are_not_retried_until_the_file_changes():
    failed = track(has_features=False, features_failed=True)
    assert not needs(existing=failed)
    assert needs(stat=(101, STAT[1]), existing=failed)
    assert needs(existing=failed, force=True)


def test_failed_genre_is_retried_only_for_a_new_model_version():
    failed = track(genre_version=None, genre_failed_version=GENRE)
    assert not needs(existing=failed)
    assert needs(existing=track(genre_version=None, genre_failed_version="genre-model-1"))
    assert needs(stat=(100, STAT[1] + 1), existing=failed)
    assert needs(existing=failed, force=True)


def test_force_imports_everything():
    assert needs(force=True)


def test_walk_finds_audio_with_its_stats_and_skips_symlinks(tmp_path):
    loose = write_flac(tmp_path / "loose.flac", "Loose")
    nested = write_flac(tmp_path / "Artist" / "Album" / "song.flac", "Song")
    (tmp_path / "Artist" / "cover.jpg").write_bytes(b"jpg")
    (tmp_path / "link.flac").symlink_to(loose)
    (tmp_path / "linked-dir").symlink_to(tmp_path / "Artist")

    found = walk_audio_files(tmp_path)

    assert set(found) == {str(loose), str(nested)}
    st = nested.stat()
    assert found[str(nested)] == (st.st_size, st.st_mtime)


def _scan_without_analysis(root: Path) -> None:
    scan_library(root, options=ScanOptions(analyze_audio=False))


def _plan(root: Path, **kwargs):
    with db.get_conn() as conn:
        return plan_scan(conn, root, **kwargs)


def test_plan_diffs_disk_against_tracks(db_url, tmp_path):
    a = write_flac(tmp_path / "A" / "a.flac", "A")
    b = write_flac(tmp_path / "B" / "b.flac", "B", freq=550.0)
    c = write_flac(tmp_path / "C" / "c.flac", "C", freq=660.0)
    _scan_without_analysis(tmp_path)

    unchanged = _plan(tmp_path, analyze=False)
    assert unchanged.empty
    assert len(unchanged.files) == 3

    new = write_flac(tmp_path / "D" / "d.flac", "D", freq=770.0)
    st = b.stat()
    os.utime(b, (st.st_atime, st.st_mtime + 10))  # changed mtime
    with c.open("ab") as f:  # changed size
        f.write(b"\0")

    plan = _plan(tmp_path, analyze=False)
    assert {f: [p for p, _, _ in files] for f, files in plan.folders.items()} == {
        str(tmp_path / "B"): [str(b)],
        str(tmp_path / "C"): [str(c)],
        str(tmp_path / "D"): [str(new)],
    }
    assert plan.missing == 0
    # The task args carry the walk's stats, so the import doesn't stat again.
    assert plan.folders[str(tmp_path / "D")][0][1:] == plan.files[str(new)]

    a.unlink()
    assert _plan(tmp_path, analyze=False).missing == 1
    assert _plan(tmp_path, analyze=False, force=True).folders.keys() == {
        str(tmp_path / d) for d in ("B", "C", "D")
    }


def test_plan_picks_tracks_missing_features_or_with_an_old_genre_version(db_url, tmp_path):
    a = write_flac(tmp_path / "A" / "a.flac", "A")
    b = write_flac(tmp_path / "B" / "b.flac", "B", freq=550.0)
    _scan_without_analysis(tmp_path)

    with psycopg.connect(db_url) as conn:
        ids = dict(conn.execute("SELECT path, id FROM tracks").fetchall())
        for tid in ids.values():
            conn.execute("INSERT INTO audio_features (track_id, bpm) VALUES (%s, 120)", (tid,))
            conn.execute(
                "INSERT INTO track_audio_genres (track_id, model_version, activations) VALUES (%s, %s, '{0.5}')",
                (tid, GENRE),
            )

    assert _plan(tmp_path, genre_version=GENRE).empty

    with psycopg.connect(db_url) as conn:
        conn.execute("DELETE FROM audio_features WHERE track_id = %s", (ids[str(a)],))
        conn.execute("UPDATE track_audio_genres SET model_version = 'old' WHERE track_id = %s", (ids[str(b)],))

    assert _plan(tmp_path, genre_version=GENRE).folders.keys() == {str(tmp_path / "A"), str(tmp_path / "B")}
    # With the genre model off, stored genre versions don't matter.
    assert _plan(tmp_path, genre_version=None).folders.keys() == {str(tmp_path / "A")}


def test_empty_disk_is_not_every_track_missing(db_url, tmp_path):
    song = write_flac(tmp_path / "A" / "song.flac", "Song")
    _scan_without_analysis(tmp_path)
    song.unlink()

    assert _plan(tmp_path, analyze=False).empty
