# OwnWave v1 Implementation Plan

## Goal

Deliver a working end-to-end smart radio for a local FLAC library:
- Scan files, extract features, and persist them.
- Serve a generated station queue.
- Stream audio (FLAC or MP3) to the browser.
- Crossfade tracks in the browser with Web Audio.
- Provide a simple station UI in Next.js.

## Milestone 1: Foundations

### 1.1 Repository & infrastructure

- [ ] Create `docker-compose.yml` with PostgreSQL, Python worker, Go API, and Next.js.
- [ ] Create `database/migrations/001_schema.sql`.
- [ ] Add `Makefile` or `justfile` for common commands.

### 1.2 Database schema

Tables:
- `artists`
- `albums`
- `tracks` (`id`, `artist_id`, `album_id`, `path`, `title`, `track_number`, `duration`)
- `audio_features` (`track_id`, `bpm`, `key`, `loudness`, `energy`, `valence`, `outro_start_seconds`, `ideal_crossfade_seconds`, `sample_rate`, `channels`)
- `stations` (`id`, `name`, `seed_features`)
- `queue` (`station_id`, `track_id`, `position`, `played_at`)
- `scan_jobs` (`id`, `status`, `started_at`, `finished_at`, `error`)

### 1.3 Python analytics engine

- [ ] `src/analyzer.py` — base `Analyzer` protocol.
- [ ] `src/analyzer_librosa.py` — default librosa analyzer.
- [ ] `src/analyzer_essentia.py` — optional essentia analyzer.
- [ ] `src/db.py` — Postgres read/write.
- [ ] `src/scanner.py` — FLAC tag reader + analyzer invocation.
- [ ] `src/station_builder.py` — rule-based station profiles.
- [ ] `src/api.py` — FastAPI app with `POST /scan`.
- [ ] `src/main.py` — CLI `python -m scanner scan <path>`.

### 1.4 Go API server

- [ ] `main.go` — server setup, CORS, routes.
- [ ] `db.go` — database connection and queries.
- [ ] `handlers.go` — `GET /tracks`, `GET /stations`, `GET /queue`, `POST /admin/scan`.
- [ ] `streaming.go` — `GET /stream/{id}?format=mp3|flac&token=...`.
- [ ] `queue.go` — generate next track with crossfade markers.
- [ ] `auth.go` — validate signed stream tokens.

### 1.5 Next.js web frontend

- [ ] `package.json` with Next.js, Tailwind, tRPC, Zod.
- [ ] `app/layout.tsx` and `app/page.tsx`.
- [ ] `server/routers/app.ts` — tRPC router proxying to Go.
- [ ] `components/Player.tsx` — two `<audio>` elements + Web Audio crossfade.
- [ ] `lib/api.ts` — Go REST client.

### 1.6 Integration & verification

- [ ] `docker compose up --build`.
- [ ] Place a small FLAC library in the mounted music directory.
- [ ] Run `python -m scanner scan /music`.
- [ ] Verify tracks and features in Postgres.
- [ ] Create a station and fetch `/queue`.
- [ ] Open the web player and confirm playback and crossfade.

## Verification Checklist

- [ ] Postgres is reachable from Go and Python.
- [ ] Scanner creates rows for all FLAC files.
- [ ] `audio_features` contains `bpm`, `outro_start_seconds`, and `ideal_crossfade_seconds`.
- [ ] `GET /queue` returns a valid next track with transition markers.
- [ ] `GET /stream/{id}` streams FLAC.
- [ ] `GET /stream/{id}?format=mp3` returns MP3.
- [ ] Web player starts playback.
- [ ] When a track reaches its outro, the next track fades in and the current track fades out.
- [ ] On Safari/mobile fallback, the player still plays sequential MP3 tracks.

## Future Work (v2)

- [ ] File-system watcher for auto-reindexing.
- [ ] Server-side crossfaded MP3 stream for clients without Web Audio.
- [ ] AI clustering (K-means / embeddings) for station generation.
- [ ] Native mobile/desktop clients.
- [ ] S3-compatible storage support.
