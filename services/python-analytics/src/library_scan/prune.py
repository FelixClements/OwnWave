"""Reconcile the tracks table with the audio files on disk after a scan.

Pruning runs once per scan, against a full listing of the scanned root, so a
folder task can never delete tracks that belong to another folder. Files that
were moved or renamed are matched to their old track row and keep its id, so
listening history and likes/bans survive.
"""

from datetime import datetime
from pathlib import Path
from typing import Dict, List, Optional, Set, Tuple

import psycopg

from audio_metadata import is_supported_audio_file
from db import delete_tracks

# Durations of the same file read twice can differ slightly between decoders.
DURATION_TOLERANCE_SECONDS = 0.5


def list_audio_files(root: Path) -> Set[str]:
    """All supported audio files under root (recursive)."""
    return {str(p) for p in root.rglob("*") if is_supported_audio_file(p)}


def music_dirs(root: Path) -> List[str]:
    """Every directory under root (including root) that directly contains audio."""
    return sorted({str(Path(p).parent) for p in list_audio_files(root)})


def match_moved_tracks(missing: List[dict], new: List[dict]) -> List[Tuple[dict, dict]]:
    """Pair tracks whose file vanished with tracks first seen in this scan.

    A pair needs the same byte size and a duration within tolerance, and must
    be unambiguous on both sides (copies of the same file are left alone).
    """

    def candidates(track: dict, pool: List[dict]) -> List[dict]:
        if track.get("file_size") is None or track.get("duration_seconds") is None:
            return []
        return [
            other
            for other in pool
            if other.get("file_size") == track["file_size"]
            and other.get("duration_seconds") is not None
            and abs(other["duration_seconds"] - track["duration_seconds"]) <= DURATION_TOLERANCE_SECONDS
        ]

    pairs = []
    for old in missing:
        matches = candidates(old, new)
        if len(matches) != 1:
            continue
        if len(candidates(matches[0], missing)) != 1:
            continue
        pairs.append((old, matches[0]))
    return pairs


def prune_library(
    conn: psycopg.Connection,
    root: Path,
    scan_started_at: Optional[datetime],
) -> Dict[str, int]:
    """Re-home moved tracks and delete tracks whose files are gone.

    Does nothing when no audio is found on disk, so an unmounted or empty
    library never wipes the database.
    """
    root = Path(root).expanduser().resolve()
    if not root.is_dir():
        return {"moved": 0, "deleted": 0}
    on_disk = list_audio_files(root)
    if not on_disk:
        print(f"[prune] no audio files under {root}; skipping prune")
        return {"moved": 0, "deleted": 0}

    with conn.cursor() as cur:
        cur.execute(
            """
            SELECT id, path, file_size, duration_seconds, created_at
            FROM tracks
            WHERE starts_with(path, %s)
            """,
            (str(root) + "/",),
        )
        columns = [d[0] for d in cur.description]
        rows = [dict(zip(columns, r)) for r in cur.fetchall()]

    missing = [r for r in rows if r["path"] not in on_disk]
    if not missing:
        return {"moved": 0, "deleted": 0}

    new = [
        r
        for r in rows
        if r["path"] in on_disk and scan_started_at is not None and r["created_at"] >= scan_started_at
    ]
    pairs = match_moved_tracks(missing, new)

    with conn.cursor() as cur:
        for old, moved in pairs:
            # Drop the freshly imported duplicate, then point the old row
            # (which owns history and feedback) at the new location.
            cur.execute(
                "DELETE FROM tracks WHERE id = %s RETURNING path, file_size, file_mtime",
                (moved["id"],),
            )
            path, file_size, file_mtime = cur.fetchone()
            cur.execute(
                """
                UPDATE tracks
                SET path = %s, file_size = %s, file_mtime = %s, updated_at = NOW()
                WHERE id = %s
                """,
                (path, file_size, file_mtime, old["id"]),
            )

    moved_ids = {old["id"] for old, _ in pairs}
    to_delete = [r["id"] for r in missing if r["id"] not in moved_ids]
    delete_tracks(conn, to_delete)
    conn.commit()

    if pairs or to_delete:
        print(f"[prune] {root}: moved {len(pairs)}, deleted {len(to_delete)}")
    return {"moved": len(pairs), "deleted": len(to_delete)}


def db_now(conn: psycopg.Connection) -> datetime:
    with conn.cursor() as cur:
        cur.execute("SELECT NOW()")
        return cur.fetchone()[0]
