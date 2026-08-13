from datetime import datetime, timezone
from pathlib import Path
from typing import Any, Dict, List, Optional, Set
from uuid import UUID

import psycopg

from analyzers import analyze_file, get_analyzers
from config import ENABLE_GENRE_ANALYSIS
from db import (
    delete_tracks,
    get_conn,
    get_or_create_album,
    get_or_create_artist,
    get_track_stats_by_paths,
    upsert_audio_features_batch,
    upsert_tracks_batch,
    upsert_track_genres,
)
from feature_vector import build_feature_vector
from models import AudioFeatures, FailedPath, GenrePrediction, ScanResult
from scanner import SUPPORTED_EXTS, _get_duration, _parse_int
from tags import extract_genres_from_tags, read_tags

if ENABLE_GENRE_ANALYSIS:
    import genre_analyzer


def _get_file_stats(path: Path) -> tuple:
    stat = path.stat()
    mtime = datetime.fromtimestamp(stat.st_mtime, tz=timezone.utc)
    return stat.st_size, mtime


def _file_changed(
    file_path: Path,
    stats: dict,
    force: bool,
) -> bool:
    if force:
        return True
    size, mtime = _get_file_stats(file_path)
    return (
        stats.get("file_size") != size
        or stats.get("file_mtime") != mtime
    )


def import_folder(folder_path: str, analyze: bool = True, force: bool = False) -> ScanResult:
    """Import a single music folder with batch database upserts.

    Performs incremental updates: files whose size/mtime have not changed
    since the last scan are skipped unless `force=True`. Missing audio
    features are filled in for unchanged files. Tracks that have disappeared
    from the folder are deleted from the database.
    """
    folder = Path(folder_path).expanduser().resolve()
    if not folder.exists():
        raise FileNotFoundError(f"Music path does not exist: {folder}")

    files = sorted(
        p for p in folder.rglob("*") if p.is_file() and p.suffix.lower() in SUPPORTED_EXTS
    )
    result = ScanResult(total_files=len(files))

    if not files:
        with get_conn() as conn:
            _remove_deleted_tracks(conn, folder, set())
            conn.commit()
        return result

    analyzers = get_analyzers() if analyze else None
    track_records: List[dict] = []
    feature_records: List[dict] = []
    genre_records: List[List[GenrePrediction]] = []
    processed_ids: List[UUID] = []
    artist_cache: Dict[str, UUID] = {}
    album_cache: Dict[tuple, UUID] = {}

    with get_conn() as conn:
        existing_stats = get_track_stats_by_paths(conn, [str(f) for f in files])

        try:
            for file_path in files:
                try:
                    path_str = str(file_path)
                    stats = existing_stats.get(path_str, {})

                    if not _file_changed(file_path, stats, force):
                        if analyzers and not stats.get("has_features"):
                            # Metadata is up to date but features are missing;
                            # analyze and backfill without rebuilding the track row.
                            result.scanned += 1
                            feature_rec, predictions, error = _build_feature_record(
                                conn, file_path, path_str, analyzers
                            )
                            if feature_rec:
                                feature_records.append(feature_rec)
                                processed_ids.append(stats["id"])
                                result.features += 1
                            if predictions:
                                genre_records.append((path_str, predictions))
                                result.model_success += 1
                            if error:
                                if feature_rec:
                                    result.model_failed += 1
                                result.failed_paths.append(FailedPath(path_str, error))
                        # Always backfill tag-based genres for existing tracks.
                        tag_genres = extract_genres_from_tags(read_tags(path_str))
                        if tag_genres:
                            upsert_track_genres(conn, stats["id"], tag_genres)
                        continue

                    track_rec, feature_rec, predictions, tag_genres, feature_error = _build_record(
                        conn,
                        file_path,
                        analyzers,
                        artist_cache,
                        album_cache,
                        force=force,
                    )
                    if track_rec:
                        track_records.append(track_rec)
                        result.imported += 1
                        if tag_genres:
                            genre_records.append((track_rec["path"], tag_genres))
                    if analyzers:
                        result.scanned += 1
                    if feature_rec:
                        feature_records.append(feature_rec)
                        result.features += 1
                    if predictions:
                        genre_records.append((track_rec["path"], predictions))
                        result.model_success += 1
                    elif ENABLE_GENRE_ANALYSIS and not feature_error:
                        # Audio features succeeded but model produced no predictions.
                        result.model_failed += 1
                        result.failed_paths.append(FailedPath(path_str, "genre analysis produced no predictions"))

                    if feature_error:
                        result.failed_paths.append(FailedPath(path_str, feature_error))
                except Exception as e:
                    reason = str(e)
                    print(f"Error processing {file_path}: {e}")
                    result.failed_paths.append(FailedPath(str(file_path), reason))
                    continue

            if track_records:
                upsert_tracks_batch(conn, track_records)

            # Build a path -> track_id map for feature upserts, combining
            # newly upserted tracks with tracks that already existed.
            new_paths = [r["path"] for r in track_records]
            new_stats = get_track_stats_by_paths(conn, new_paths) if new_paths else {}
            path_to_id = {**{p: s["id"] for p, s in existing_stats.items()}, **{p: s["id"] for p, s in new_stats.items()}}

            if feature_records:
                upsert_audio_features_batch(conn, feature_records, path_to_id)

            for path_str, predictions in genre_records:
                track_id = path_to_id.get(path_str)
                if track_id:
                    upsert_track_genres(conn, track_id, predictions)

            # Delete database rows for files that no longer exist in this folder.
            _remove_deleted_tracks(conn, folder, {str(f) for f in files})

            conn.commit()

            for rec in track_records:
                track_id = path_to_id.get(rec["path"])
                if track_id:
                    processed_ids.append(track_id)

            result.track_ids = processed_ids

        except Exception:
            conn.rollback()
            raise

    return result


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
    file_size, file_mtime = _get_file_stats(file_path)

    track_rec = {
        "path": path_str,
        "artist_id": artist_id,
        "album_id": album_id,
        "title": title,
        "track_number": track_number,
        "duration": duration,
        "sample_rate": sample_rate,
        "channels": channels,
        "file_size": file_size,
        "file_mtime": file_mtime,
    }
    tag_genres = extract_genres_from_tags(tags)

    feature_error = None
    if analyzers and (force or not _has_features(conn, path_str)):
        feature_rec, predictions, feature_error = _build_feature_record(conn, file_path, path_str, analyzers)
    else:
        feature_rec, predictions = None, []

    return track_rec, feature_rec, predictions, tag_genres, feature_error


def _build_feature_record(conn: psycopg.Connection, file_path: Path, path_str: str, analyzers: List[Any]) -> tuple:
    error = None
    try:
        features = analyze_file(path_str, analyzers=analyzers)
        record = {
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
            "feature_vector": build_feature_vector(features, conn),
        }
    except Exception as exc:
        error = f"audio analysis failed: {exc}"
        print(f"Error analyzing {file_path}: {exc}")
        return None, [], error

    predictions = []
    if ENABLE_GENRE_ANALYSIS:
        try:
            predictions = genre_analyzer.analyze(path_str)
        except Exception as exc:
            error = f"genre analysis failed: {exc}"
            print(f"[folder_importer] genre analysis failed for {path_str}: {exc}")

    return record, predictions, error


def _has_features(conn: psycopg.Connection, path_str: str) -> bool:
    with conn.cursor() as cur:
        cur.execute(
            """
            SELECT 1 FROM audio_features af
            JOIN tracks t ON t.id = af.track_id
            WHERE t.path = %s
            """,
            (path_str,),
        )
        return cur.fetchone() is not None


def _remove_deleted_tracks(
    conn: psycopg.Connection,
    folder: Path,
    current_files: Set[str],
) -> None:
    with conn.cursor() as cur:
        cur.execute(
            "SELECT id, path FROM tracks WHERE path LIKE %s",
            (str(folder) + "/%",),
        )
        rows = cur.fetchall()

    to_delete = [row[0] for row in rows if row[1] not in current_files]
    if to_delete:
        delete_tracks(conn, to_delete)


def _get_sample_info(path: str) -> tuple:
    try:
        from mutagen import File as MutagenFile

        info = MutagenFile(path).info
        return getattr(info, "sample_rate", None), getattr(info, "channels", None)
    except Exception:
        return None, None
