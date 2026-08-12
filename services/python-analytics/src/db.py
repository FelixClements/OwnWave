import time
from datetime import datetime
from typing import List, Optional
from uuid import UUID

import psycopg
from psycopg.types.json import Jsonb

from config import DATABASE_URL
from models import AudioFeatures, GenrePrediction


def get_conn():
    conn = psycopg.connect(DATABASE_URL)
    try:
        import pgvector.psycopg

        pgvector.psycopg.register_vector(conn)
    except Exception:
        pass
    return conn


def wait_for_db(max_retries: int = 30, delay: float = 2.0) -> None:
    """Wait until the database is reachable."""
    last_exc = None
    for i in range(max_retries):
        try:
            with get_conn() as conn:
                with conn.cursor() as cur:
                    cur.execute("SELECT 1")
            return
        except Exception as exc:
            last_exc = exc
            print(f"waiting for db... ({i + 1}/{max_retries})")
            time.sleep(delay)
    raise last_exc


def get_or_create_artist(conn: psycopg.Connection, name: str) -> UUID:
    with conn.cursor() as cur:
        cur.execute(
            """
            INSERT INTO artists (name)
            VALUES (%s)
            ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name
            RETURNING id
            """,
            (name,),
        )
        return cur.fetchone()[0]


def get_or_create_album(
    conn: psycopg.Connection,
    title: str,
    artist_id: Optional[UUID] = None,
    year: Optional[int] = None,
) -> UUID:
    with conn.cursor() as cur:
        cur.execute(
            """
            INSERT INTO albums (artist_id, title, year)
            VALUES (%s, %s, %s)
            ON CONFLICT (artist_id, title) DO UPDATE
                SET year = EXCLUDED.year
            RETURNING id
            """,
            (artist_id, title, year),
        )
        return cur.fetchone()[0]


def upsert_track(
    conn: psycopg.Connection,
    path: str,
    title: str,
    artist_id: Optional[UUID],
    album_id: Optional[UUID],
    track_number: Optional[int],
    duration: Optional[float],
    sample_rate: Optional[int],
    channels: Optional[int],
    file_size: Optional[int] = None,
    file_mtime: Optional[datetime] = None,
) -> UUID:
    with conn.cursor() as cur:
        cur.execute(
            """
            INSERT INTO tracks
                (path, artist_id, album_id, title, track_number, duration_seconds,
                 sample_rate, channels, file_size, file_mtime, updated_at)
            VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, NOW())
            ON CONFLICT (path) DO UPDATE SET
                title = EXCLUDED.title,
                artist_id = EXCLUDED.artist_id,
                album_id = EXCLUDED.album_id,
                track_number = EXCLUDED.track_number,
                duration_seconds = EXCLUDED.duration_seconds,
                sample_rate = EXCLUDED.sample_rate,
                channels = EXCLUDED.channels,
                file_size = EXCLUDED.file_size,
                file_mtime = EXCLUDED.file_mtime,
                updated_at = NOW()
            RETURNING id
            """,
            (
                path,
                artist_id,
                album_id,
                title,
                track_number,
                duration,
                sample_rate,
                channels,
                file_size,
                file_mtime,
            ),
        )
        return cur.fetchone()[0]


def upsert_audio_features(
    conn: psycopg.Connection,
    track_id: UUID,
    features: AudioFeatures,
) -> None:
    with conn.cursor() as cur:
        cur.execute(
            """
            INSERT INTO audio_features
                (track_id, bpm, key, loudness, energy, valence,
                 outro_start_seconds, ideal_crossfade_seconds, intro_start_seconds,
                 outro_end_seconds, chroma, spectral_centroid, mfcc, feature_vector, analyzed_at)
            VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s::jsonb, %s, %s, %s, NOW())
            ON CONFLICT (track_id) DO UPDATE SET
                bpm = EXCLUDED.bpm,
                key = EXCLUDED.key,
                loudness = EXCLUDED.loudness,
                energy = EXCLUDED.energy,
                valence = EXCLUDED.valence,
                outro_start_seconds = EXCLUDED.outro_start_seconds,
                ideal_crossfade_seconds = EXCLUDED.ideal_crossfade_seconds,
                intro_start_seconds = EXCLUDED.intro_start_seconds,
                outro_end_seconds = EXCLUDED.outro_end_seconds,
                chroma = EXCLUDED.chroma,
                spectral_centroid = EXCLUDED.spectral_centroid,
                mfcc = EXCLUDED.mfcc,
                feature_vector = EXCLUDED.feature_vector,
                analyzed_at = NOW()
            """,
            (
                track_id,
                features.bpm,
                features.key,
                features.loudness,
                features.energy,
                features.valence,
                features.outro_start_seconds,
                features.ideal_crossfade_seconds,
                features.intro_start_seconds,
                features.outro_end_seconds,
                Jsonb(features.chroma) if features.chroma else None,
                features.spectral_centroid,
                features.mfcc,
                features.feature_vector,
            ),
        )


def get_track_by_path(conn: psycopg.Connection, path: str) -> Optional[UUID]:
    with conn.cursor() as cur:
        cur.execute("SELECT id FROM tracks WHERE path = %s", (path,))
        row = cur.fetchone()
        return row[0] if row else None


def create_scan_job(conn: psycopg.Connection, path: str) -> UUID:
    with conn.cursor() as cur:
        cur.execute(
            "INSERT INTO scan_jobs (path) VALUES (%s) RETURNING id",
            (path,),
        )
        return cur.fetchone()[0]


def update_scan_job(
    conn: psycopg.Connection,
    job_id: UUID,
    status: str,
    error: Optional[str] = None,
) -> None:
    with conn.cursor() as cur:
        cur.execute(
            """
            UPDATE scan_jobs
            SET status = %s, error = %s,
                started_at = CASE WHEN %s = 'running' AND started_at IS NULL THEN NOW() ELSE started_at END,
                finished_at = CASE WHEN %s IN ('completed', 'failed') THEN NOW() ELSE finished_at END
            WHERE id = %s
            """,
            (status, error, status, status, job_id),
        )


def create_station(
    conn: psycopg.Connection,
    name: str,
    seed_features: Optional[dict] = None,
) -> UUID:
    with conn.cursor() as cur:
        cur.execute(
            "INSERT INTO stations (name, seed_features) VALUES (%s, %s::jsonb) RETURNING id",
            (name, Jsonb(seed_features) if seed_features else None),
        )
        return cur.fetchone()[0]


def insert_station_tracks(
    conn: psycopg.Connection,
    station_id: UUID,
    track_ids: List[UUID],
) -> None:
    with conn.cursor() as cur:
        for pos, track_id in enumerate(track_ids, start=1):
            cur.execute(
                "INSERT INTO station_tracks (station_id, track_id, position) VALUES (%s, %s, %s)",
                (station_id, track_id, pos),
            )


def get_all_tracks_with_features(conn: psycopg.Connection) -> List[dict]:
    with conn.cursor() as cur:
        cur.execute(
            """
            SELECT t.id, t.title, t.artist_id, t.album_id, t.path,
                   af.bpm, af.key, af.energy, af.valence, af.feature_vector,
                   tc.cluster_id,
                   array_agg(f.feedback) as feedback
            FROM tracks t
            JOIN audio_features af ON t.id = af.track_id
            LEFT JOIN track_clusters tc ON t.id = tc.track_id
            LEFT JOIN track_feedback f ON t.id = f.track_id
            WHERE af.feature_vector IS NOT NULL
            GROUP BY t.id, af.bpm, af.key, af.energy, af.valence, af.feature_vector, tc.cluster_id
            """
        )
        columns = [desc[0] for desc in cur.description]
        rows = cur.fetchall()
        out = []
        for row in rows:
            d = dict(zip(columns, row))
            if d.get("feature_vector") is not None:
                d["feature_vector"] = d["feature_vector"].to_list()
            out.append(d)
        return out


def get_queue(conn: psycopg.Connection, station_id: UUID) -> List[dict]:
    with conn.cursor() as cur:
        cur.execute(
            """
            SELECT t.id, t.title, t.path, t.artist_id, t.album_id,
                   af.bpm, af.key, af.energy, af.valence,
                   af.outro_start_seconds, af.ideal_crossfade_seconds,
                   af.intro_start_seconds, af.outro_end_seconds,
                   st.position
            FROM station_tracks st
            JOIN tracks t ON st.track_id = t.id
            JOIN audio_features af ON t.id = af.track_id
            WHERE st.station_id = %s
            ORDER BY st.position
            """,
            (station_id,),
        )
        columns = [desc[0] for desc in cur.description]
        return [dict(zip(columns, row)) for row in cur.fetchall()]


def get_track_stats_by_paths(conn: psycopg.Connection, paths: List[str]) -> dict:
    with conn.cursor() as cur:
        cur.execute(
            """
            SELECT t.id, t.path, t.file_size, t.file_mtime, af.track_id IS NOT NULL AS has_features
            FROM tracks t
            LEFT JOIN audio_features af ON t.id = af.track_id
            WHERE t.path = ANY(%s)
            """,
            (paths,),
        )
        return {
            row[1]: {
                "id": row[0],
                "file_size": row[2],
                "file_mtime": row[3],
                "has_features": row[4],
            }
            for row in cur.fetchall()
        }


def upsert_tracks_batch(conn: psycopg.Connection, records: List[dict]) -> None:
    if not records:
        return
    values = [
        (
            rec["path"],
            rec.get("artist_id"),
            rec.get("album_id"),
            rec["title"],
            rec.get("track_number"),
            rec.get("duration"),
            rec.get("sample_rate"),
            rec.get("channels"),
            rec.get("file_size"),
            rec.get("file_mtime"),
        )
        for rec in records
    ]
    with conn.cursor() as cur:
        cur.executemany(
            """
            INSERT INTO tracks
                (path, artist_id, album_id, title, track_number, duration_seconds,
                 sample_rate, channels, file_size, file_mtime, updated_at)
            VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, NOW())
            ON CONFLICT (path) DO UPDATE SET
                artist_id = EXCLUDED.artist_id,
                album_id = EXCLUDED.album_id,
                title = EXCLUDED.title,
                track_number = EXCLUDED.track_number,
                duration_seconds = EXCLUDED.duration_seconds,
                sample_rate = EXCLUDED.sample_rate,
                channels = EXCLUDED.channels,
                file_size = EXCLUDED.file_size,
                file_mtime = EXCLUDED.file_mtime,
                updated_at = NOW()
            """,
            values,
        )


def upsert_audio_features_batch(
    conn: psycopg.Connection, records: List[AudioFeatures], path_to_id: dict
) -> None:
    if not records:
        return
    values = []
    for rec in records:
        track_id = path_to_id.get(rec["path"])
        if not track_id:
            continue
        values.append(
            (
                track_id,
                rec["bpm"],
                rec["key"],
                rec["loudness"],
                rec["energy"],
                rec["valence"],
                rec["outro_start_seconds"],
                rec["ideal_crossfade_seconds"],
                rec["intro_start_seconds"],
                rec["outro_end_seconds"],
                Jsonb(rec["chroma"]) if rec.get("chroma") else None,
                rec.get("spectral_centroid"),
                rec.get("mfcc"),
                rec.get("feature_vector"),
            )
        )
    if not values:
        return
    with conn.cursor() as cur:
        cur.executemany(
            """
            INSERT INTO audio_features
                (track_id, bpm, key, loudness, energy, valence,
                 outro_start_seconds, ideal_crossfade_seconds, intro_start_seconds,
                 outro_end_seconds, chroma, spectral_centroid, mfcc, feature_vector, analyzed_at)
            VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, NOW())
            ON CONFLICT (track_id) DO UPDATE SET
                bpm = EXCLUDED.bpm,
                key = EXCLUDED.key,
                loudness = EXCLUDED.loudness,
                energy = EXCLUDED.energy,
                valence = EXCLUDED.valence,
                outro_start_seconds = EXCLUDED.outro_start_seconds,
                ideal_crossfade_seconds = EXCLUDED.ideal_crossfade_seconds,
                intro_start_seconds = EXCLUDED.intro_start_seconds,
                outro_end_seconds = EXCLUDED.outro_end_seconds,
                chroma = EXCLUDED.chroma,
                spectral_centroid = EXCLUDED.spectral_centroid,
                mfcc = EXCLUDED.mfcc,
                feature_vector = EXCLUDED.feature_vector,
                analyzed_at = NOW()
            """,
            values,
        )


def delete_tracks(conn: psycopg.Connection, track_ids: List[UUID]) -> None:
    if not track_ids:
        return
    with conn.cursor() as cur:
        cur.execute(
            "DELETE FROM tracks WHERE id = ANY(%s)",
            (track_ids,),
        )


def upsert_track_genres(
    conn: psycopg.Connection,
    track_id: UUID,
    predictions: List[GenrePrediction],
) -> None:
    if not predictions:
        return
    values = [
        (track_id, p.main_genre, p.sub_genre, p.confidence, p.source)
        for p in predictions
    ]
    with conn.cursor() as cur:
        cur.executemany(
            """
            INSERT INTO track_genres
                (track_id, main_genre, sub_genre, confidence, source, updated_at)
            VALUES (%s, %s, %s, %s, %s, NOW())
            ON CONFLICT (track_id, sub_genre, source) DO UPDATE SET
                main_genre = EXCLUDED.main_genre,
                confidence = EXCLUDED.confidence,
                updated_at = NOW()
            """,
            values,
        )


def get_track_genres(conn: psycopg.Connection, track_id: UUID) -> List[dict]:
    with conn.cursor() as cur:
        cur.execute(
            """
            SELECT main_genre, sub_genre, confidence, source
            FROM track_genres
            WHERE track_id = %s
            ORDER BY confidence DESC
            """,
            (track_id,),
        )
        return [
            {"main_genre": r[0], "sub_genre": r[1], "confidence": r[2], "source": r[3]}
            for r in cur.fetchall()
        ]


def list_genres(conn: psycopg.Connection, min_confidence: float = 0.3) -> List[dict]:
    with conn.cursor() as cur:
        cur.execute(
            """
            SELECT main_genre, sub_genre, COUNT(DISTINCT track_id) as track_count
            FROM track_genres
            WHERE confidence >= %s
            GROUP BY main_genre, sub_genre
            ORDER BY main_genre, sub_genre
            """,
            (min_confidence,),
        )
        return [
            {"main_genre": r[0], "sub_genre": r[1], "track_count": r[2]}
            for r in cur.fetchall()
        ]


def get_tracks_by_genre(
    conn: psycopg.Connection,
    main_genre: str,
    sub_genre: Optional[str] = None,
    min_confidence: float = 0.0,
    limit: int = 200,
) -> List[UUID]:
    with conn.cursor() as cur:
        if sub_genre:
            cur.execute(
                """
                SELECT track_id
                FROM track_genres
                WHERE main_genre = %s AND sub_genre = %s AND confidence >= %s
                ORDER BY confidence DESC
                LIMIT %s
                """,
                (main_genre, sub_genre, min_confidence, limit),
            )
        else:
            cur.execute(
                """
                SELECT track_id
                FROM track_genres
                WHERE main_genre = %s AND confidence >= %s
                ORDER BY confidence DESC
                LIMIT %s
                """,
                (main_genre, min_confidence, limit),
            )
        return [r[0] for r in cur.fetchall()]


def create_station(
    conn: psycopg.Connection,
    name: str,
    seed_features: Optional[dict] = None,
    is_auto: bool = False,
    source: Optional[str] = None,
) -> UUID:
    with conn.cursor() as cur:
        cur.execute(
            """
            INSERT INTO stations (name, seed_features, is_auto, source, last_refreshed_at)
            VALUES (%s, %s::jsonb, %s, %s, NOW())
            RETURNING id
            """,
            (name, Jsonb(seed_features) if seed_features else None, is_auto, source),
        )
        return cur.fetchone()[0]


def upsert_auto_station(
    conn: psycopg.Connection,
    name: str,
    seed_features: Optional[dict],
    source: str = "genre",
) -> UUID:
    with conn.cursor() as cur:
        cur.execute(
            """
            INSERT INTO stations (name, seed_features, is_auto, source, last_refreshed_at)
            VALUES (%s, %s::jsonb, TRUE, %s, NOW())
            ON CONFLICT (name) WHERE is_auto = TRUE DO UPDATE SET
                seed_features = EXCLUDED.seed_features,
                is_auto = TRUE,
                source = EXCLUDED.source,
                last_refreshed_at = NOW()
            RETURNING id
            """,
            (name, Jsonb(seed_features) if seed_features else None, source),
        )
        return cur.fetchone()[0]


def delete_orphaned_auto_stations(conn: psycopg.Connection, valid_names: List[str]) -> int:
    if not valid_names:
        with conn.cursor() as cur:
            cur.execute(
                """
                DELETE FROM stations
                WHERE is_auto = TRUE AND source = 'genre'
                """
            )
            return cur.rowcount
    with conn.cursor() as cur:
        cur.execute(
            """
            DELETE FROM stations
            WHERE is_auto = TRUE
              AND source = 'genre'
              AND name <> ALL(%s)
            """,
            (list(valid_names),),
        )
        return cur.rowcount
