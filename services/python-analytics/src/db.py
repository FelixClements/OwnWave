from datetime import datetime
from typing import List, Optional
from uuid import UUID

import psycopg
from psycopg.types.json import Jsonb

from config import DATABASE_URL
from models import AudioFeatures


def get_conn():
    return psycopg.connect(DATABASE_URL)


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
) -> UUID:
    with conn.cursor() as cur:
        cur.execute(
            """
            INSERT INTO tracks
                (path, artist_id, album_id, title, track_number, duration_seconds,
                 sample_rate, channels, updated_at)
            VALUES (%s, %s, %s, %s, %s, %s, %s, %s, NOW())
            ON CONFLICT (path) DO UPDATE SET
                title = EXCLUDED.title,
                artist_id = EXCLUDED.artist_id,
                album_id = EXCLUDED.album_id,
                track_number = EXCLUDED.track_number,
                duration_seconds = EXCLUDED.duration_seconds,
                sample_rate = EXCLUDED.sample_rate,
                channels = EXCLUDED.channels,
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
                 outro_start_seconds, ideal_crossfade_seconds, chroma, analyzed_at)
            VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s::jsonb, NOW())
            ON CONFLICT (track_id) DO UPDATE SET
                bpm = EXCLUDED.bpm,
                key = EXCLUDED.key,
                loudness = EXCLUDED.loudness,
                energy = EXCLUDED.energy,
                valence = EXCLUDED.valence,
                outro_start_seconds = EXCLUDED.outro_start_seconds,
                ideal_crossfade_seconds = EXCLUDED.ideal_crossfade_seconds,
                chroma = EXCLUDED.chroma,
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
                Jsonb(features.chroma) if features.chroma else None,
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
                   af.bpm, af.energy, af.valence
            FROM tracks t
            JOIN audio_features af ON t.id = af.track_id
            """
        )
        columns = [desc[0] for desc in cur.description]
        return [dict(zip(columns, row)) for row in cur.fetchall()]


def get_queue(conn: psycopg.Connection, station_id: UUID) -> List[dict]:
    with conn.cursor() as cur:
        cur.execute(
            """
            SELECT t.id, t.title, t.path, t.artist_id, t.album_id,
                   af.bpm, af.key, af.energy, af.valence,
                   af.outro_start_seconds, af.ideal_crossfade_seconds,
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
