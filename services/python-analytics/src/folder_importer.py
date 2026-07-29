from pathlib import Path
from typing import Any, Dict, List, Optional
from uuid import UUID

import psycopg

from analyzers import analyze_file, get_analyzers
from db import (
    get_conn,
    get_or_create_album,
    get_or_create_artist,
    get_track_ids_by_paths,
    upsert_audio_features_batch,
    upsert_tracks_batch,
)
from models import AudioFeatures
from scanner import SUPPORTED_EXTS, _get_duration, _parse_int
from tags import read_tags


def import_folder(folder_path: str, analyze: bool = True, force: bool = False) -> List[UUID]:
    """Import a single music folder with batch database upserts."""
    folder = Path(folder_path).expanduser().resolve()
    if not folder.exists():
        raise FileNotFoundError(f"Music path does not exist: {folder}")

    files = sorted(
        p for p in folder.rglob("*") if p.is_file() and p.suffix.lower() in SUPPORTED_EXTS
    )
    if not files:
        return []

    analyzers = get_analyzers() if analyze else None
    track_records: List[dict] = []
    feature_records: List[dict] = []
    artist_cache: Dict[str, UUID] = {}
    album_cache: Dict[tuple, UUID] = {}

    with get_conn() as conn:
        conn.autocommit = False
        try:
            for file_path in files:
                try:
                    track_rec, feature_rec = _build_record(
                        conn,
                        file_path,
                        analyzers,
                        artist_cache,
                        album_cache,
                        force=force,
                    )
                    if track_rec:
                        track_records.append(track_rec)
                    if feature_rec:
                        feature_records.append(feature_rec)
                except Exception as e:
                    print(f"Error processing {file_path}: {e}")
                    continue

            if track_records:
                upsert_tracks_batch(conn, track_records)
                path_to_id = get_track_ids_by_paths(conn, [r["path"] for r in track_records])
                for rec in track_records:
                    rec["id"] = path_to_id.get(rec["path"])
                if feature_records:
                    upsert_audio_features_batch(conn, feature_records, path_to_id)
                conn.commit()

        except Exception:
            conn.rollback()
            raise

    return [r["id"] for r in track_records]


def _build_record(
    conn: psycopg.Connection,
    file_path: Path,
    analyzers: Optional[List[Any]],
    artist_cache: Dict[str, UUID],
    album_cache: Dict[tuple, UUID],
    force: bool,
) -> tuple:
    path_str = str(file_path)
    tags = read_tags(path_str)

    artist_name = tags.get("artist") or "Unknown Artist"
    if artist_name not in artist_cache:
        artist_cache[artist_name] = get_or_create_artist(conn, artist_name)
    artist_id = artist_cache[artist_name]

    album_title = tags.get("album") or "Unknown Album"
    album_key = (artist_id, album_title)
    if album_key not in album_cache:
        year = _parse_int(tags.get("date"))
        album_cache[album_key] = get_or_create_album(conn, album_title, artist_id, year)
    album_id = album_cache[album_key]

    track_number = _parse_int(tags.get("tracknumber"))
    duration = _get_duration(path_str)
    sample_rate, channels = _get_sample_info(path_str)
    title = tags.get("title") or file_path.stem

    track_rec = {
        "path": path_str,
        "artist_id": artist_id,
        "album_id": album_id,
        "title": title,
        "track_number": track_number,
        "duration": duration,
        "sample_rate": sample_rate,
        "channels": channels,
    }

    feature_rec = None
    if analyzers is not None:
        features = analyze_file(path_str, analyzers=analyzers)
        feature_rec = {
            "path": path_str,
            "bpm": features.bpm,
            "key": features.key,
            "loudness": features.loudness,
            "energy": features.energy,
            "valence": features.valence,
            "outro_start_seconds": features.outro_start_seconds,
            "ideal_crossfade_seconds": features.ideal_crossfade_seconds,
            "chroma": features.chroma,
        }

    return track_rec, feature_rec


def _get_sample_info(path: str) -> tuple:
    try:
        from mutagen import File as MutagenFile

        info = MutagenFile(path).info
        return getattr(info, "sample_rate", None), getattr(info, "channels", None)
    except Exception:
        return None, None
