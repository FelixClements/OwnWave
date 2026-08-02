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

- [x] Create `docker-compose.yml` with PostgreSQL, Python worker, Go API, and Next.js.
- [x] Create `database/migrations/001_schema.sql`.
- [ ] Add `justfile` for common commands (`build`, `scan`, `test`, `dev`, `db-migrate`).
- [ ] Add environment/secrets handling and `golang-migrate` migration tooling.

### 1.2 Database schema

- [x] `artists`
- [x] `albums`
- [x] `tracks` (`id`, `artist_id`, `album_id`, `path`, `title`, `track_number`, `duration`)
- [x] `audio_features` (`track_id`, `bpm`, `key`, `loudness`, `energy`, `valence`, `outro_start_seconds`, `ideal_crossfade_seconds`)
- [x] `stations` (`id`, `name`, `seed_features`)
- [x] `station_tracks` (`station_id`, `track_id`, `position`, `played_at`)
- [x] `scan_jobs` (`id`, `status`, `started_at`, `finished_at`, `error`)

### 1.3 Python analytics engine

- [x] `src/analyzer.py` / `src/models.py` — base `Analyzer` protocol.
- [x] `src/analyzer_librosa.py` — default librosa analyzer.
- [x] `src/analyzer_essentia.py` — optional essentia analyzer.
- [x] `src/db.py` — Postgres read/write.
- [x] `src/scanner.py` — FLAC tag reader + analyzer invocation.
- [x] `src/station_builder.py` — rule-based station profiles.
- [x] `src/api.py` — FastAPI app with `POST /scan`.
- [x] `src/main.py` — CLI `python -m main scan <path>`.

### 1.4 Go API server

- [x] `main.go` — server setup, CORS, routes.
- [x] `db.go` — database connection and queries.
- [x] `handlers.go` — `GET /tracks`, `GET /stations`, `GET /queue`, `POST /admin/scan`.
- [x] `streaming.go` — `GET /stream/{id}?format=mp3|flac&token=...`.
- [x] `handlers.go` / `station_builder.py` — queue generation with crossfade markers.
- [x] `handlers.go` / `streaming.go` — signed stream token validation.

### 1.5 Next.js web frontend

- [x] `package.json` with Next.js, Tailwind, tRPC, Zod.
- [x] `app/layout.tsx` and `app/page.tsx`.
- [x] `server/routers/app.ts` — tRPC router proxying to Go.
- [x] `components/Player.tsx` — two `<audio>` elements + Web Audio crossfade.
- [ ] `lib/api.ts` — typed Go REST client covering all Go endpoints.

### 1.6 Integration & verification

- [x] `docker compose up --build`.
- [x] Place a small FLAC library in the mounted music directory.
- [x] Run `python -m main scan /music`.
- [x] Verify tracks and features in Postgres.
- [x] Create a station and fetch `/queue`.
- [x] Open the web player and confirm playback and crossfade.

## Verification Checklist

- [x] Postgres is reachable from Go and Python.
- [x] Scanner creates rows for all FLAC files.
- [x] `audio_features` contains `bpm`, `outro_start_seconds`, and `ideal_crossfade_seconds`.
- [x] `GET /queue` returns a valid next track with transition markers.
- [x] `GET /stream/{id}` streams FLAC.
- [x] `GET /stream/{id}?format=mp3` returns MP3.
- [x] Web player starts playback.
- [x] When a track reaches its outro, the next track fades in and the current track fades out.
- [x] On Safari/mobile fallback, the player still plays sequential MP3 tracks.

## Milestone 2: v2 — Smart radio & production hardening

### 2.1 Auto-indexing & storage

- [x] File-system watcher (`watchdog` or `inotify`) on `MUSIC_DIR` for real-time reindexing.
- [x] Incremental scanning: only process new/changed/deleted files.
- [x] Configurable audio format ladder: FLAC source, MP3/Opus/AAC transcoding targets.
- [x] Background job queue (Redis/RabbitMQ/Celery) for large library scans.

### 2.2 AI-driven station generation

- [x] Compute a normalized feature vector for every track (BPM, key, loudness, spectral centroid, chroma, energy, valence, MFCCs).
- [x] K-means / DBSCAN clustering over feature vectors; store `track_clusters` and `cluster_centers`.
- [x] Similarity search with k-NN (pgvector or Faiss) for "play more like this".
- [x] Station seeds from track, artist, album, mood, or cluster.
- [x] "Smart shuffle" that balances similarity and novelty using a weighted score (similarity, cluster distance, novelty/decay).
- [ ] Optional musicnn/essentia embedding-based similarity. (post-v2 / out of v2)

### 2.3 Streaming & crossfade

- [ ] Server-side crossfaded FLAC stream (MP3 fallback) for Safari / mobile / clients without Web Audio.
- [x] Format and bitrate selection per client (`format=mp3|flac|opus&bitrate=192`).
- [ ] Gapless playback and precise cue-point handling.
- [ ] Volume normalization via EBU R128 integrated loudness (ReplayGain tags may be used as a fallback source).

### 2.4 Users, feedback, & personalization

- [ ] User accounts and sessions (local username/password to start; OAuth post-v2).
- [ ] Listening history, likes, skips, and bans.
- [ ] Feedback loop that updates station weights and queue scoring.
- [ ] Per-station controls: min/max BPM, energy, valence, preferred clusters.

### 2.5 Web frontend & clients

- [ ] Station management UI: create, edit, delete, preview stations.
- [ ] Real-time queue display with upcoming tracks.
- [ ] Full-text search for tracks, albums, and artists.
- [ ] Album art and metadata display.
- [ ] Music library page: browse/search tracks, albums, and artists; show cover art; admin rescan action.
- [ ] PWA support, Media Session API, and offline queue cache. (post-v2)
- [ ] Mobile-first responsive design.

### 2.6 Admin, observability, & operations

- [ ] Admin dashboard for scan jobs, queues, and system health.
- [ ] Prometheus/OpenTelemetry metrics and structured logging.
- [ ] Comprehensive test suites (unit, integration, E2E).
- [ ] GitHub Actions CI/CD: build, lint, test, and Docker image publishing.
- [ ] Production deployment guide (reverse proxy, SSL, backups).

## v2 Verification Checklist

- [x] Adding a file to `music/` triggers re-analysis automatically.
- [ ] Server-side crossfade stream works in Safari without Web Audio.
- [ ] AI stations generate coherent queues from a seed track.
- [ ] Similar tracks are retrievable via API/frontend.
- [ ] MP3/Opus/AAC transcoding selectable per client.
- [ ] Likes/skips influence station recommendations.
- [ ] Web player works as PWA and shows album art. (PWA post-v2; album art in v2)
- [ ] CI runs tests and builds on every PR.
- [ ] Deployment guide runs a production-like setup.

## Future Work (v3+)

- [ ] Native mobile and desktop clients.
- [ ] Multi-room synchronized playback.
- [ ] Live DJ / scheduled programming blocks.
- [ ] Podcast and spoken-word station support.
- [ ] Federated / shared stations between instances.
- [ ] Embedding-based similarity (musicnn/essentia).
