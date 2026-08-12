from pathlib import Path
from typing import Callable, List, Optional
from uuid import UUID

import psycopg

from analyzers import analyze_file, get_analyzers
from config import ENABLE_GENRE_ANALYSIS, MUSIC_DIR
from db import (
    get_conn,
    get_or_create_album,
    get_or_create_artist,
    get_track_by_path,
    upsert_audio_features,
    upsert_track,
    upsert_track_genres,
)
from feature_vector import build_feature_vector
from models import AudioFeatures
from tags import read_tags

if ENABLE_GENRE_ANALYSIS:
    import genre_analyzer


SUPPORTED_EXTS = {".flac", ".mp3"}


def scan_path(
    path: str,
    force: bool = False,
    progress_callback: Optional[Callable[[str], None]] = None,
) -> List[UUID]:
    root = Path(path).expanduser().resolve()
    if not root.exists():
        raise FileNotFoundError(f"Music path does not exist: {root}")

    analyzers = get_analyzers()
    track_ids: List[UUID] = []

    files = sorted(p for p in root.rglob("*") if p.is_file() and p.suffix.lower() in SUPPORTED_EXTS)

    with get_conn() as conn:
        conn.autocommit = False
        try:
            for file_path in files:
                if progress_callback:
                    progress_callback(str(file_path))
                try:
                    track_id = _process_file(conn, file_path, analyzers, force=force)
                    if track_id:
                        track_ids.append(track_id)
                except Exception as e:
                    conn.rollback()
                    print(f"Error processing {file_path}: {e}")
                    continue
            conn.commit()
        except Exception:
            conn.rollback()
            raise

    return track_ids


def _process_file(
    conn: psycopg.Connection,
    file_path: Path,
    analyzers: list,
    force: bool,
) -> Optional[UUID]:
    path_str = str(file_path)
    existing = get_track_by_path(conn, path_str)
    if existing and not force:
        return existing

    tags = read_tags(path_str)

    artist_id = None
    if tags.get("artist"):
        artist_id = get_or_create_artist(conn, tags["artist"])

    album_id = None
    if tags.get("album"):
        year = _parse_int(tags.get("date"))
        album_id = get_or_create_album(conn, tags["album"], artist_id, year)

    track_number = _parse_int(tags.get("tracknumber"))

    features = analyze_file(path_str, analyzers=analyzers)
    features.feature_vector = build_feature_vector(features, conn)
    duration = _get_duration(path_str)

    sample_rate = None
    channels = None
    try:
        from mutagen import File as MutagenFile

        info = MutagenFile(path_str).info
        sample_rate = getattr(info, "sample_rate", None)
        channels = getattr(info, "channels", None)
    except Exception:
        pass

    from folder_importer import _get_file_stats

    file_size, file_mtime = _get_file_stats(file_path)
    track_id = upsert_track(
        conn=conn,
        path=path_str,
        title=tags.get("title") or file_path.stem,
        artist_id=artist_id,
        album_id=album_id,
        track_number=track_number,
        duration=duration,
        sample_rate=sample_rate,
        channels=channels,
        file_size=file_size,
        file_mtime=file_mtime,
    )

    upsert_audio_features(conn, track_id, features)

    if ENABLE_GENRE_ANALYSIS:
        try:
            predictions = genre_analyzer.analyze(path_str)
            if predictions:
                upsert_track_genres(conn, track_id, predictions)
        except Exception as e:
            print(f"[scanner] genre analysis failed for {path_str}: {e}")

    return track_id


def _parse_int(value) -> Optional[int]:
    if not value:
        return None
    try:
        return int(str(value).split("/")[0])
    except Exception:
        return None


def _get_duration(path: str) -> Optional[float]:
    try:
        from mutagen import File as MutagenFile

        return getattr(MutagenFile(path).info, "length", None)
    except Exception:
        return None
