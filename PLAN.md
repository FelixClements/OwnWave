# OwnWave Implementation Plan

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

## Milestone 2: v2 — Smart radio & production hardening

### 2.1 Auto-indexing & storage

- [ ] File-system watcher (`watchdog` or `inotify`) on `MUSIC_DIR` for real-time reindexing.
- [ ] Incremental scanning: only process new/changed/deleted files.
- [ ] S3-compatible storage support (MinIO / AWS S3) for audio archives.
- [ ] Configurable audio format ladder: FLAC source, MP3/Opus/AAC transcoding targets.
- [ ] Background job queue (Redis/RabbitMQ/Celery) for large library scans.

### 2.2 AI-driven station generation

- [ ] Compute a normalized feature vector for every track (BPM, key, loudness, spectral centroid, chroma, energy, valence, MFCCs).
- [ ] K-means / DBSCAN clustering over feature vectors; store `track_clusters` and `cluster_centers`.
- [ ] Similarity search with k-NN (pgvector or Faiss) for "play more like this".
- [ ] Station seeds from track, artist, album, mood, or cluster.
- [ ] "Smart shuffle" that balances similarity and novelty.
- [ ] Optional musicnn/essentia embedding-based similarity.

### 2.3 Streaming & crossfade

- [ ] Server-side crossfaded MP3 stream for Safari / mobile / clients without Web Audio.
- [ ] Format and bitrate selection per client (`format=mp3|flac|opus&bitrate=192`).
- [ ] Gapless playback and precise cue-point handling.
- [ ] Volume normalization via ReplayGain or EBU R128 integrated loudness.

### 2.4 Users, feedback, & personalization

- [ ] User accounts and sessions (OAuth or local auth).
- [ ] Listening history, likes, skips, and bans.
- [ ] Feedback loop that updates station weights and queue scoring.
- [ ] Per-station controls: min/max BPM, energy, valence, preferred clusters.

### 2.5 Web frontend & clients

- [ ] Station management UI: create, edit, delete, preview stations.
- [ ] Real-time queue display with upcoming tracks.
- [ ] Full-text search for tracks, albums, and artists.
- [ ] Album art and metadata display.
- [ ] PWA support, Media Session API, and offline queue cache.
- [ ] Mobile-first responsive design.

### 2.6 Admin, observability, & operations

- [ ] Admin dashboard for scan jobs, queues, and system health.
- [ ] Prometheus/OpenTelemetry metrics and structured logging.
- [ ] Comprehensive test suites (unit, integration, E2E).
- [ ] GitHub Actions CI/CD: build, lint, test, and Docker image publishing.
- [ ] Production deployment guide (reverse proxy, SSL, backups).

## v2 Verification Checklist

- [ ] Adding a file to `music/` triggers re-analysis automatically.
- [ ] Server-side crossfade stream works in Safari without Web Audio.
- [ ] AI stations generate coherent queues from a seed track.
- [ ] Similar tracks are retrievable via API/frontend.
- [ ] MP3/Opus/AAC transcoding selectable per client.
- [ ] Likes/skips influence station recommendations.
- [ ] Web player works as PWA and shows album art.
- [ ] CI runs tests and builds on every PR.
- [ ] Deployment guide runs a production-like setup.

## Future Work (v3+)

- [ ] Native mobile and desktop clients.
- [ ] Multi-room synchronized playback.
- [ ] Live DJ / scheduled programming blocks.
- [ ] Podcast and spoken-word station support.
- [ ] Federated / shared stations between instances.
