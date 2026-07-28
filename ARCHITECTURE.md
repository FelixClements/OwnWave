# OwnWave v1 Architecture

## Overview

OwnWave is a self-hosted, AI-driven smart radio for local FLAC libraries.
The v1 architecture is split into four layers:

1. **Python analytics engine** — scans the music folder, extracts audio features (BPM, key, loudness, outro boundaries, mood), and writes structured data to PostgreSQL.
2. **Go API server** — serves REST endpoints for tracks, stations, and queue; streams audio (raw FLAC or transcoded MP3) directly to the browser.
3. **Next.js web frontend** — presents stations and a player. Uses tRPC for typed data and Web Audio for client-side crossfading.
4. **PostgreSQL** — shared state for tracks, albums, artists, audio features, stations, and the playback queue.

```text
┌─────────────────────────────────────┐
│  Browser (Next.js + Web Audio)     │
│  tRPC → Next.js API → Go REST      │
│  <audio>  ────────────────────────▶│
└─────────────────────────────────────┘
           │                  │
           ▼                  ▼
┌─────────────────┐   ┌─────────────────────┐
│  Next.js (tRPC) │   │  Go API server      │
└────────┬────────┘   │  REST + streaming   │
         │            └──────────┬──────────┘
         │                       │
         └───────────┬───────────┘
                     │
         ┌───────────▼───────────┐
         │    PostgreSQL         │
         └───────────┬───────────┘
                     │
         ┌───────────▼───────────┐
         │ Python analytics      │
         │ librosa / essentia    │
         └───────────────────────┘
```

## Architectural Decisions

### ADR 1: Direct Go REST for audio, tRPC only for data

- **Context**: The frontend needs both JSON data and binary audio streams. tRPC is TypeScript-native and cannot be hosted by Go. Streaming audio through Next.js adds latency and memory cost.
- **Decision**: Use **tRPC in Next.js** for typed JSON data; tRPC procedures proxy to the Go REST API. Audio is requested directly from Go via `<audio>` URLs (`/stream/{id}?format=...&token=...`).
- **Consequences**: Type-safe data; minimal audio latency; audio URLs require signed-token auth because `<audio>` cannot send custom headers.

### ADR 2: Client-side Web Audio crossfade with server fallback

- **Context**: The README envisions beat-aligned, gapless crossfading. This must be low-latency. Some clients (older mobile, Safari) may not support Web Audio well.
- **Decision**: Implement the crossfade in the browser with two `<audio>` elements connected to Web Audio gain nodes. Fallback clients receive individual MP3 tracks played sequentially.
- **Consequences**: Best crossfade quality and instant track changes; fallback is simpler but not mixed.

### ADR 3: Pluggable DSP with librosa default and essentia optional

- **Context**: More metadata improves station quality. `librosa` is lightweight and widely available; `essentia` is more powerful but harder to install.
- **Decision**: Define a single `Analyzer` protocol. The default analyzer uses `librosa`; if `essentia` is installed, a second analyzer is registered and its extra features are merged.
- **Consequences**: The container stays buildable even when essentia is missing; richer data when available.

### ADR 4: Short-lived signed URLs for audio auth

- **Context**: `<audio>` cannot send `Authorization` headers, so secret tokens in headers are not possible if we stream through the native audio element.
- **Decision**: Use short-lived, signed URLs (`?token=<jwt-or-signed-string>`) for audio streams. Data endpoints use normal session/cookie or token auth.
- **Consequences**: Tokens may appear in server logs and browser history, so expiry must be short (minutes) and tokens single-purpose.

### ADR 5: PostgreSQL as shared database

- **Context**: The system needs durable, queryable state shared between Python, Go, and Next.js.
- **Decision**: Use PostgreSQL in `docker-compose`. SQLite is not suitable for concurrent worker + API access.
- **Consequences**: Requires a database container but gives concurrent, reliable storage for large libraries.

### ADR 6: Indexing by API + CLI, optional file watcher

- **Context**: Music libraries can be large and change over time. A manual CLI is useful for initial import; an API trigger is useful for the UI; a watcher keeps data fresh.
- **Decision**: The Python worker exposes a scan endpoint and a CLI command. A filesystem watcher can be enabled via config but is off by default in v1.
- **Consequences**: Fast initial import via CLI; remote trigger for admin UI; watcher added later to avoid Docker volume/inotify edge cases.

## Components

### Python analytics engine (`services/python-analytics`)

- `src/analyzer_librosa.py` — default analyzer.
- `src/analyzer_essentia.py` — optional analyzer.
- `src/scanner.py` — directory walker and tag reader.
- `src/station_builder.py` — groups tracks into station profiles.
- `src/db.py` — PostgreSQL persistence.
- `src/api.py` — small HTTP API for scan jobs.
- `src/main.py` — CLI entry point.

### Go API server (`services/go-api-server`)

- `main.go` — HTTP server setup.
- `handlers/` — REST handlers for tracks, stations, queue, admin.
- `streaming.go` — FLAC/MP3 streaming with optional `ffmpeg`.
- `queue.go` — queue scheduler and crossfade marker logic.
- `db.go` — database access.
- `auth.go` — signed URL validation.

### Next.js web frontend (`apps/t3-web-frontend`)

- `app/page.tsx` — station browser.
- `app/player/page.tsx` or embedded player.
- `server/routers/app.ts` — tRPC router.
- `components/Player.tsx` — Web Audio crossfade player.
- `lib/api.ts` — Go REST client for the tRPC proxy.

## Technology Stack

- **Python 3.11+**, `librosa`, `mutagen`, `pyloudnorm`, `essentia` (optional), `fastapi`, `uvicorn`, `psycopg`.
- **Go 1.22+**, `chi` or stdlib, `jackc/pgx`.
- **Next.js 14+**, React, Tailwind CSS, tRPC, TypeScript.
- **PostgreSQL 15+**.
- **Docker / docker-compose** for local development and deployment.
