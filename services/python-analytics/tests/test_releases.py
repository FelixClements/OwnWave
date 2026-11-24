"""Releases, compilations, ISRCs and MusicBrainz ids from the scan (#134, ADR 0008)."""

from pathlib import Path

import numpy as np
import psycopg
import soundfile as sf
from mutagen.easyid3 import EasyID3
from mutagen.flac import FLAC

import db
from library_scan import scan_library
from library_scan.options import ScanOptions
from library_scan.plan import plan_scan
from tags import TAGS_VERSION, VARIOUS_ARTISTS_MBID, read_tags, tagged_compilation

RELEASE_A = "11111111-1111-1111-1111-111111111111"
RELEASE_B = "22222222-2222-2222-2222-222222222222"
GROUP = "33333333-3333-3333-3333-333333333333"
RECORDING = "44444444-4444-4444-4444-444444444444"
ARTIST_1 = "55555555-5555-5555-5555-555555555555"
ARTIST_2 = "66666666-6666-6666-6666-666666666666"


def _tone(path: Path, seconds: float, freq: float, fmt: str) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    t = np.linspace(0, seconds, int(8000 * seconds), endpoint=False)
    sf.write(path, 0.2 * np.sin(2 * np.pi * freq * t), 8000, format=fmt)


_freq = [300.0]


def write_song(path: Path, title: str, artist: str, album: str, **tags) -> Path:
    """A FLAC with the given tags. Each song gets its own tone, so no two
    files look like a moved copy of each other."""
    _freq[0] += 7.0
    _tone(path, 1.0, _freq[0], "FLAC")
    f = FLAC(path)
    f["title"] = title
    f["artist"] = artist
    f["album"] = album
    for key, value in tags.items():
        f[key] = value
    f.save()
    return path


def scan(root: Path):
    return scan_library(root, options=ScanOptions(analyze_audio=False))


def releases_by_path(url: str) -> dict:
    """path -> (release id, release title, is_compilation)."""
    with psycopg.connect(url) as conn:
        rows = conn.execute(
            """
            SELECT t.path, r.id, r.title, r.is_compilation
            FROM tracks t LEFT JOIN releases r ON r.id = t.release_id
            """
        ).fetchall()
    return {path: (rid, title, comp) for path, rid, title, comp in rows}


def release_count(url: str) -> int:
    with psycopg.connect(url) as conn:
        return conn.execute("SELECT count(*) FROM releases").fetchone()[0]


# Reading tags


def test_read_tags_reads_flac_ids(tmp_path):
    path = write_song(
        tmp_path / "a.flac",
        "Song",
        "Artist",
        "Album",
        albumartist="Album Artist",
        isrc="gb-aaa-24-00001",
        musicbrainz_trackid=RECORDING.upper(),
        musicbrainz_albumid=RELEASE_A,
        musicbrainz_releasegroupid=GROUP,
        musicbrainz_artistid=[ARTIST_1, ARTIST_2],
        releasetype=["album", "compilation"],
    )
    tags = read_tags(str(path))
    assert tags["albumartist"] == "Album Artist"
    assert tags["isrc"] == "GBAAA2400001"
    assert tags["mb_recording_id"] == RECORDING
    assert tags["mb_release_id"] == RELEASE_A
    assert tags["mb_release_group_id"] == GROUP
    assert tags["mb_artist_ids"] == [ARTIST_1, ARTIST_2]
    assert tags["releasetype"] == "album; compilation"
    assert tagged_compilation(tags)


def test_read_tags_reads_mp3_ids(tmp_path):
    path = tmp_path / "a.mp3"
    _tone(path, 1.0, 440.0, "MP3")
    f = EasyID3()
    f["title"] = "Song"
    f["artist"] = "Artist"
    f["album"] = "Album"
    f["albumartist"] = "Various Artists"
    f["isrc"] = "USAAA2400002"
    f["musicbrainz_trackid"] = RECORDING
    f["musicbrainz_albumid"] = RELEASE_A
    f["musicbrainz_releasegroupid"] = GROUP
    # ID3v2.3 style: several ids in one frame, joined by "/".
    f["musicbrainz_artistid"] = f"{ARTIST_1}/{ARTIST_2}"
    f["musicbrainz_albumtype"] = "album"
    f.save(path)

    tags = read_tags(str(path))
    assert tags["albumartist"] == "Various Artists"
    assert tags["isrc"] == "USAAA2400002"
    assert tags["mb_recording_id"] == RECORDING
    assert tags["mb_release_id"] == RELEASE_A
    assert tags["mb_release_group_id"] == GROUP
    assert tags["mb_artist_ids"] == [ARTIST_1, ARTIST_2]
    assert tags["releasetype"] == "album"


def test_read_tags_without_ids(tmp_path):
    tags = read_tags(str(write_song(tmp_path / "a.flac", "Song", "Artist", "Album", isrc="not an isrc")))
    for key in ("albumartist", "isrc", "mb_recording_id", "mb_release_id", "mb_release_group_id", "mb_artist_ids"):
        assert tags[key] is None
    assert not tagged_compilation(tags)


def test_tags_that_mark_a_compilation():
    assert tagged_compilation({"albumartist": "Various Artists"})
    assert tagged_compilation({"albumartist": " various artists "})
    assert tagged_compilation({"albumartist": "VA"})
    assert tagged_compilation({"mb_album_artist_ids": [VARIOUS_ARTISTS_MBID]})
    assert tagged_compilation({"releasetype": "album; compilation"})
    assert tagged_compilation({"releasetype": "Compilation"})
    assert tagged_compilation({"compilation": "1"})

    assert not tagged_compilation({})
    assert not tagged_compilation({"albumartist": "Various Artists Tribute Band"})
    assert not tagged_compilation({"releasetype": "album"})
    assert not tagged_compilation({"compilation": "0"})


# Release grouping without MusicBrainz ids: same folder and album tag


def test_release_without_mb_id_is_folder_and_album(db_url, tmp_path):
    a1 = write_song(tmp_path / "A" / "1.flac", "One", "Artist", "Album")
    a2 = write_song(tmp_path / "A" / "2.flac", "Two", "Artist", "Album")
    other_album = write_song(tmp_path / "A" / "3.flac", "Three", "Artist", "Other Album")
    other_folder = write_song(tmp_path / "B" / "1.flac", "One", "Artist", "Album")
    scan(tmp_path)

    rel = releases_by_path(db_url)
    assert rel[str(a1)][0] == rel[str(a2)][0]
    assert rel[str(a1)][1] == "Album"
    assert rel[str(other_album)][0] != rel[str(a1)][0]
    assert rel[str(other_folder)][0] != rel[str(a1)][0]
    assert release_count(db_url) == 3
    assert not any(comp for _, _, comp in rel.values())


def test_compilation_without_mb_id_stays_one_release(db_url, tmp_path):
    """Albums are (track artist, album title), so a compilation splits into
    one album per artist; its release stays whole."""
    songs = [
        write_song(tmp_path / "Mix" / f"{i}.flac", f"Song {i}", f"Artist {i}", "Summer Hits")
        for i in range(4)
    ]
    scan(tmp_path)

    rel = releases_by_path(db_url)
    assert len({rel[str(s)][0] for s in songs}) == 1
    assert rel[str(songs[0])][2] is True
    with psycopg.connect(db_url) as conn:
        albums = conn.execute("SELECT count(DISTINCT album_id) FROM tracks").fetchone()[0]
    assert albums == 4


def test_three_track_artists_are_not_a_compilation(db_url, tmp_path):
    songs = [
        write_song(tmp_path / "Split" / f"{i}.flac", f"Song {i}", f"Artist {i % 3}", "Split EP")
        for i in range(5)
    ]
    scan(tmp_path)
    assert releases_by_path(db_url)[str(songs[0])][2] is False


def test_various_artists_album_artist_is_a_compilation(db_url, tmp_path):
    song = write_song(tmp_path / "VA" / "1.flac", "Song", "Artist", "Hits", albumartist="Various Artists")
    write_song(tmp_path / "VA" / "2.flac", "Song 2", "Other", "Hits", albumartist="Various Artists")
    scan(tmp_path)
    assert releases_by_path(db_url)[str(song)][2] is True


# Release grouping with MusicBrainz ids


def test_release_with_mb_id_spans_folders(db_url, tmp_path):
    cd1 = write_song(tmp_path / "Album" / "CD1" / "1.flac", "One", "Artist", "Album", musicbrainz_albumid=RELEASE_A)
    cd2 = write_song(
        tmp_path / "Album" / "CD2" / "1.flac", "Two", "Artist", "Album (disc 2)", musicbrainz_albumid=RELEASE_A
    )
    scan(tmp_path)

    rel = releases_by_path(db_url)
    assert rel[str(cd1)][0] == rel[str(cd2)][0]
    assert release_count(db_url) == 1
    with psycopg.connect(db_url) as conn:
        assert conn.execute("SELECT mb_release_id FROM releases").fetchone()[0] == RELEASE_A


def test_mb_ids_split_one_folder_into_releases(db_url, tmp_path):
    a = write_song(tmp_path / "Dump" / "1.flac", "One", "Artist", "Greatest Hits", musicbrainz_albumid=RELEASE_A)
    b = write_song(tmp_path / "Dump" / "2.flac", "Two", "Artist", "Greatest Hits", musicbrainz_albumid=RELEASE_B)
    untagged = write_song(tmp_path / "Dump" / "3.flac", "Three", "Artist", "Greatest Hits")
    scan(tmp_path)

    rel = releases_by_path(db_url)
    assert len({rel[str(p)][0] for p in (a, b, untagged)}) == 3


def test_mb_release_type_marks_a_compilation(db_url, tmp_path):
    song = write_song(
        tmp_path / "C" / "1.flac", "Song", "Artist", "Best Of", musicbrainz_albumid=RELEASE_A,
        releasetype=["album", "compilation"],
    )
    scan(tmp_path)
    assert releases_by_path(db_url)[str(song)][2] is True


def test_scan_stores_ids_by_recording_and_release(db_url, tmp_path):
    song = write_song(
        tmp_path / "A" / "1.flac",
        "Song",
        "Artist",
        "Album",
        albumartist="Album Artist",
        isrc="GBAAA2400001",
        musicbrainz_trackid=RECORDING,
        musicbrainz_albumid=RELEASE_A,
        musicbrainz_releasegroupid=GROUP,
        musicbrainz_artistid=[ARTIST_1, ARTIST_2],
        releasetype="album",
    )
    scan(tmp_path)

    with psycopg.connect(db_url) as conn:
        row = conn.execute(
            """
            SELECT t.isrc, t.mb_recording_id, t.mb_artist_ids, t.tags_version,
                   r.mb_release_id, r.mb_release_group_id, r.album_artist, r.mb_release_type, r.folder
            FROM tracks t JOIN releases r ON r.id = t.release_id
            WHERE t.path = %s
            """,
            (str(song),),
        ).fetchone()
    assert row == (
        "GBAAA2400001",
        RECORDING,
        [ARTIST_1, ARTIST_2],
        TAGS_VERSION,
        RELEASE_A,
        GROUP,
        "Album Artist",
        "album",
        str(tmp_path / "A"),
    )


# Keeping releases up to date


def test_rescan_fills_ids_for_files_read_by_an_older_tag_reader(db_url, tmp_path):
    song = write_song(tmp_path / "A" / "1.flac", "Song", "Artist", "Album", isrc="GBAAA2400001")
    scan(tmp_path)
    # Rows as a scan before #134 left them.
    with psycopg.connect(db_url) as conn:
        conn.execute("UPDATE tracks SET release_id = NULL, isrc = NULL, tags_version = 0")
        conn.execute("DELETE FROM releases")

    with db.get_conn() as conn:
        plan = plan_scan(conn, tmp_path, analyze=False)
    assert [p for p, _, _ in plan.folders[str(tmp_path / "A")]] == [str(song)]

    scan(tmp_path)
    with psycopg.connect(db_url) as conn:
        row = conn.execute("SELECT isrc, release_id IS NOT NULL, tags_version FROM tracks").fetchone()
    assert row == ("GBAAA2400001", True, TAGS_VERSION)
    with db.get_conn() as conn:
        assert plan_scan(conn, tmp_path, analyze=False).empty


def test_retagged_song_moves_release_and_empty_release_goes(db_url, tmp_path):
    song = write_song(tmp_path / "A" / "1.flac", "Song", "Artist", "Old Title")
    scan(tmp_path)
    old_release = releases_by_path(db_url)[str(song)][0]

    f = FLAC(song)
    f["album"] = "New Title"
    f.save()
    scan(tmp_path)

    rid, title, _ = releases_by_path(db_url)[str(song)]
    assert rid != old_release and title == "New Title"
    assert release_count(db_url) == 1


def test_deleted_songs_update_compilation_and_empty_releases(db_url, tmp_path):
    songs = [
        write_song(tmp_path / "Mix" / f"{i}.flac", f"Song {i}", f"Artist {i}", "Mix")
        for i in range(4)
    ]
    keep = write_song(tmp_path / "Keep" / "1.flac", "Kept", "Artist", "Kept")
    scan(tmp_path)
    assert releases_by_path(db_url)[str(songs[0])][2] is True

    songs[3].unlink()
    scan(tmp_path)
    assert releases_by_path(db_url)[str(songs[0])][2] is False

    for s in songs[:3]:
        s.unlink()
    scan(tmp_path)
    assert set(releases_by_path(db_url)) == {str(keep)}
    assert release_count(db_url) == 1


def test_moved_song_takes_the_release_of_its_new_folder(db_url, tmp_path):
    song = write_song(tmp_path / "Old" / "1.flac", "Song", "Artist", "Album")
    write_song(tmp_path / "Old" / "2.flac", "Other", "Artist", "Album")
    scan(tmp_path)
    with psycopg.connect(db_url) as conn:
        track_id = conn.execute("SELECT id FROM tracks WHERE path = %s", (str(song),)).fetchone()[0]

    moved = tmp_path / "New" / "1.flac"
    moved.parent.mkdir()
    song.rename(moved)
    scan(tmp_path)

    with psycopg.connect(db_url) as conn:
        tid, folder = conn.execute(
            "SELECT t.id, r.folder FROM tracks t JOIN releases r ON r.id = t.release_id WHERE t.path = %s",
            (str(moved),),
        ).fetchone()
    assert tid == track_id
    assert folder == str(tmp_path / "New")
    assert release_count(db_url) == 2
