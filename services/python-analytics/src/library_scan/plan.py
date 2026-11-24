"""Decide what a library scan has to do before it queues anything (ADR 0006).

One walk of the root gives every audio file with its size and mtime, and one
query gives every track under the root. Comparing the two tells which folders
need importing and whether any track's file is gone. A scan whose plan is
empty does nothing else.
"""

import logging
import os
import time
from dataclasses import dataclass, field
from pathlib import Path
from typing import Dict, List, Optional, Tuple

import psycopg

from audio_metadata import SUPPORTED_EXTS, file_mtime
from tags import TAGS_VERSION

log = logging.getLogger(__name__)

# (size in bytes, st_mtime). st_mtime stays a float so it survives the JSON
# task args exactly; file_mtime() turns it into the stored timestamp.
FileStat = Tuple[int, float]


@dataclass
class ScanPlan:
    root: str
    # Every supported audio file under root.
    files: Dict[str, FileStat] = field(default_factory=dict)
    # Folder -> the files in it that need importing or analysis, sorted.
    folders: Dict[str, List[Tuple[str, int, float]]] = field(default_factory=dict)
    # Tracks under root whose file is not on disk (moved or deleted).
    missing: int = 0
    walk_ms: int = 0
    query_ms: int = 0

    @property
    def empty(self) -> bool:
        return not self.folders and not self.missing


def walk_audio_files(root: Path) -> Dict[str, FileStat]:
    """Every supported audio file under root, with size and mtime, in one walk.

    Symlinks (to files or folders) are skipped, as the per-folder import
    always did.
    """
    found: Dict[str, FileStat] = {}
    stack = [str(root)]
    while stack:
        try:
            entries = os.scandir(stack.pop())
        except OSError as exc:
            log.warning("cannot list %s: %s", exc.filename, exc)
            continue
        with entries:
            for entry in entries:
                try:
                    if entry.is_dir(follow_symlinks=False):
                        stack.append(entry.path)
                    elif (
                        entry.is_file(follow_symlinks=False)
                        and os.path.splitext(entry.name)[1].lower() in SUPPORTED_EXTS
                    ):
                        st = entry.stat(follow_symlinks=False)
                        found[entry.path] = (st.st_size, st.st_mtime)
                except OSError as exc:
                    log.warning("cannot stat %s: %s", entry.path, exc)
    return found


def load_tracks(conn: psycopg.Connection, root: Path) -> Dict[str, dict]:
    """path -> size, mtime, tag reader version, features, genre model version
    and analysis failures of every track under root."""
    rows = conn.execute(
        """
        SELECT t.path, t.file_size, t.file_mtime,
               af.track_id IS NOT NULL AS has_features, g.model_version,
               ff.track_id IS NOT NULL AS features_failed, gf.model_version,
               t.tags_version
        FROM tracks t
        LEFT JOIN audio_features af ON af.track_id = t.id
        LEFT JOIN track_audio_genres g ON g.track_id = t.id
        LEFT JOIN track_analysis_failures ff ON ff.track_id = t.id AND ff.kind = 'features'
        LEFT JOIN track_analysis_failures gf ON gf.track_id = t.id AND gf.kind = 'genre'
        WHERE starts_with(t.path, %s)
        """,
        (str(root) + "/",),
    ).fetchall()
    return {
        r[0]: {
            "file_size": r[1],
            "file_mtime": r[2],
            "has_features": r[3],
            "genre_version": r[4],
            "features_failed": r[5],
            "genre_failed_version": r[6],
            "tags_version": r[7],
        }
        for r in rows
    }


def needs_import(
    stat: FileStat,
    track: Optional[dict],
    *,
    force: bool,
    analyze: bool,
    genre_version: Optional[str],
) -> bool:
    """Whether one file needs importing or analysis.

    genre_version is the current audio genre model version, or None when the
    model is off (then stored genre versions don't matter).

    Analysis that failed before is not retried (#119): features with any
    failure on record, genre with a failure for the current model version.
    A changed file (its failures are cleared on import) or force retries both.

    A track whose tags were read by an older tag reader (tags.TAGS_VERSION)
    is imported again, so new tags reach files that did not change.
    """
    if force or track is None:
        return True
    size, mtime = stat
    if track["file_size"] != size or track["file_mtime"] != file_mtime(mtime):
        return True
    if track.get("tags_version") != TAGS_VERSION:
        return True
    if not analyze:
        return False
    if not track["has_features"] and not track.get("features_failed"):
        return True
    return (
        genre_version is not None
        and track["genre_version"] != genre_version
        and track.get("genre_failed_version") != genre_version
    )


def plan_scan(
    conn: psycopg.Connection,
    root: Path,
    *,
    force: bool = False,
    analyze: bool = True,
    genre_version: Optional[str] = None,
) -> ScanPlan:
    root = Path(root).expanduser().resolve()
    plan = ScanPlan(root=str(root))

    started = time.monotonic()
    plan.files = walk_audio_files(root)
    plan.walk_ms = round((time.monotonic() - started) * 1000)

    started = time.monotonic()
    tracks = load_tracks(conn, root)
    plan.query_ms = round((time.monotonic() - started) * 1000)

    for path in sorted(plan.files):
        size, mtime = plan.files[path]
        if needs_import((size, mtime), tracks.get(path), force=force, analyze=analyze, genre_version=genre_version):
            plan.folders.setdefault(os.path.dirname(path), []).append((path, size, mtime))

    if plan.files:
        plan.missing = sum(1 for path in tracks if path not in plan.files)
    elif tracks:
        # Same guard as prune: an unmounted or empty library never counts as
        # every track being gone.
        log.warning("no audio files under %s; not treating %d tracks as missing", root, len(tracks))

    log.info(
        "scan plan %s: %d files, %d tracks, %d folders to import (%d files), %d missing, walk %d ms, query %d ms",
        root,
        len(plan.files),
        len(tracks),
        len(plan.folders),
        sum(len(f) for f in plan.folders.values()),
        plan.missing,
        plan.walk_ms,
        plan.query_ms,
    )
    return plan
