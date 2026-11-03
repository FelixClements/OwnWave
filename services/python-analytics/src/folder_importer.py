import logging
from pathlib import Path
from typing import Any, Dict, List, Optional
from uuid import UUID

import psycopg

from analyzers import analyze_file, get_analyzers
import genre_audio
from tags import read_tags
from db import (
    audio_genre_versions_by_paths,
    get_conn,
    get_or_create_album,
    get_or_create_artist,
    get_track_stats_by_paths,
    upsert_audio_features_batch,
    upsert_scan_job_progress,
    upsert_track_audio_genres,
    upsert_tracks_batch,
)
from models import FailedPath, ScanResult
from audio_metadata import (
    get_duration,
    get_file_stats,
    get_sample_info,
    is_supported_audio_file,
    parse_int_tag,
)

log = logging.getLogger(__name__)

def _file_changed(
    file_path: Path,
    stats: dict,
    force: bool,
) -> bool:
    if force:
        return True
    size, mtime = get_file_stats(file_path)
    return (
        stats.get("file_size") != size
        or stats.get("file_mtime") != mtime
    )


def import_folder(
    folder_path: str,
    job_id: Optional[UUID] = None,
    master_job_id: Optional[UUID] = None,
    analyze: bool = True,
    force: bool = False,
) -> ScanResult:
    """Import the audio files directly inside one folder (not its subfolders).

    Never deletes tracks: removal of vanished files happens once per scan in
    library_scan.prune, against a full listing of the library.
    """
    folder = Path(folder_path).expanduser().resolve()
    if not folder.exists():
        raise FileNotFoundError(f"Music path does not exist: {folder}")

    files = sorted(p for p in folder.iterdir() if is_supported_audio_file(p))
    result = ScanResult(total_files=len(files))

    if not files:
        if job_id:
            with get_conn() as conn:
                upsert_scan_job_progress(conn, job_id, master_job_id, result)
                conn.commit()
        return result

    analyzers = get_analyzers() if analyze else None
    genre_model = genre_audio.get_model() if analyze else None
    processed_ids: List[UUID] = []
    artist_cache: Dict[str, UUID] = {}
    album_cache: Dict[tuple, UUID] = {}

    with get_conn() as conn:
        paths = [str(f) for f in files]
        existing_stats = get_track_stats_by_paths(conn, paths)
        genre_versions = audio_genre_versions_by_paths(conn, paths) if genre_model else {}

        if job_id:
            upsert_scan_job_progress(conn, job_id, master_job_id, result)
            conn.commit()

        try:
            for file_path in files:
                path_str = str(file_path)
                stats = existing_stats.get(path_str, {})
                if _import_file(
                    conn, file_path, stats, analyzers, genre_model, genre_versions.get(path_str),
                    artist_cache, album_cache, force, result, processed_ids,
                ):
                    if job_id:
                        upsert_scan_job_progress(conn, job_id, master_job_id, result)
                    conn.commit()

            result.track_ids = processed_ids

        except Exception:
            conn.rollback()
            raise

    return result


def _import_file(
    conn: psycopg.Connection,
    file_path: Path,
    stats: dict,
    analyzers: Optional[List[Any]],
    genre_model,
    genre_version: Optional[str],
    artist_cache: Dict[str, UUID],
    album_cache: Dict[tuple, UUID],
    force: bool,
    result: ScanResult,
    processed_ids: List[UUID],
) -> bool:
    """Import, analyse and store one file. Returns whether anything changed."""
    path_str = str(file_path)
    changed = False
    try:
        track_id = stats.get("id")
        if _file_changed(file_path, stats, force):
            track_rec = _build_track_record(conn, file_path, artist_cache, album_cache)
            upsert_tracks_batch(conn, [track_rec])
            track_id = get_track_stats_by_paths(conn, [path_str])[path_str]["id"]
            processed_ids.append(track_id)
            result.imported += 1
            changed = True

        needs_features = bool(analyzers) and (force or not stats.get("has_features"))
        needs_genre = genre_model is not None and (force or genre_version != genre_audio.MODEL_VERSION)
        if needs_features or needs_genre:
            result.scanned += 1
            changed = True

        if needs_features:
            feature_rec, error = _build_feature_record(conn, file_path, path_str, analyzers)
            if feature_rec:
                upsert_audio_features_batch(conn, [feature_rec], {path_str: track_id})
                result.features += 1
                if track_id not in processed_ids:
                    processed_ids.append(track_id)
            if error:
                result.failed_paths.append(FailedPath(path_str, error))

        if needs_genre:
            genres = genre_model.analyze(path_str)
            if genres:
                upsert_track_audio_genres(conn, track_id, genres)
                result.model_success += 1
            else:
                result.model_failed += 1
                result.failed_paths.append(FailedPath(path_str, "genre analysis failed"))
    except Exception as e:
        log.warning("error processing %s: %s", file_path, e)
        result.failed_paths.append(FailedPath(path_str, str(e)))
        changed = True
    return changed


def _build_track_record(
    conn: psycopg.Connection,
    file_path: Path,
    artist_cache: Dict[str, UUID],
    album_cache: Dict[tuple, UUID],
) -> dict:
    path_str = str(file_path)
    tags = read_tags(path_str)

    artist_name = tags.get("artist") or "Unknown Artist"
    if artist_name not in artist_cache:
        artist_cache[artist_name] = get_or_create_artist(conn, artist_name)
    artist_id = artist_cache[artist_name]

    album_title = tags.get("album") or "Unknown Album"
    album_key = (artist_id, album_title)
    if album_key not in album_cache:
        year = parse_int_tag(tags.get("date"))
        album_cache[album_key] = get_or_create_album(conn, album_title, artist_id, year)
    album_id = album_cache[album_key]

    sample_rate, channels = get_sample_info(path_str)
    file_size, file_mtime = get_file_stats(file_path)
    return {
        "path": path_str,
        "artist_id": artist_id,
        "album_id": album_id,
        "title": tags.get("title") or file_path.stem,
        "track_number": parse_int_tag(tags.get("tracknumber")),
        "duration": get_duration(path_str),
        "sample_rate": sample_rate,
        "channels": channels,
        "file_size": file_size,
        "file_mtime": file_mtime,
        "genre_tag": tags.get("genre"),
    }


def _build_feature_record(conn: psycopg.Connection, file_path: Path, path_str: str, analyzers: List[Any]) -> tuple:
    try:
        features = analyze_file(path_str, analyzers=analyzers)
    except Exception as exc:
        log.warning("error analyzing %s: %s", file_path, exc)
        return None, f"audio analysis failed: {exc}"
    return {
        "path": path_str,
        "bpm": features.bpm,
        "key": features.key,
        "loudness": features.loudness,
        "energy": features.energy,
        "valence": features.valence,
        "outro_start_seconds": features.outro_start_seconds,
        "ideal_crossfade_seconds": features.ideal_crossfade_seconds,
        "intro_start_seconds": features.intro_start_seconds,
        "outro_end_seconds": features.outro_end_seconds,
        "chroma": features.chroma,
        "spectral_centroid": features.spectral_centroid,
        "mfcc": features.mfcc,
    }, None
