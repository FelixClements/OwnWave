# OwnWave

An open-source, self-hosted smart radio. Turn a local library of lossless audio into personalized, continuous radio stations.

## What it does

- Scans a local music folder and extracts rich metadata and audio features (BPM, key, energy, valence).
- Builds stations from Spotify playlists: admins add public playlist URLs (the normal `open.spotify.com/playlist/…` link), OwnWave syncs them daily and finds each track in the local library. A station plays the local matches of one or more playlists plus library songs that sound like them, optionally only some genres. Playlists only grow: tracks Spotify rotates out are kept, and each playlist lists the tracks missing from the library.
- Tags every track with station genres by blending an audio model (Discogs400), file genre tags and MusicBrainz artist tags, and suggests stations per playlist and genre. Every main genre with enough songs gets its own station that plays the whole library's songs of that genre.
- A station is live: it holds every library song matching its playlists and genres, plays them shuffled with songs you have not heard in the longest time first and the same artist at most once every four tracks, and reshuffles when the queue runs out. Several files of one song play as one, and no file is ever changed or deleted. A listener can take a song off one station without banning it everywhere.
- Streams the original file or a cached transcode per client: Opus at 160, 96 or 64 kbps (phones on mobile data) or AAC 128 kbps, with seeking.
- Apps sign in with long-lived device tokens; a browse tree and voice search serve Android Auto. The API is described in [`services/go-api-server/openapi.yaml`](services/go-api-server/openapi.yaml).
- Tracks listening history and likes; banned tracks are never queued again.
- Provides a Spotify-style responsive web player (desktop and phone) and an admin dashboard for scans, genres and users.

## Architecture

| Layer | Technology | Role |
|-------|------------|------|
| `apps/web` | Vite, React, Tailwind, React Query, typed client from the OpenAPI spec | Web UI and player (gapless, optional crossfade, lock-screen controls), served by the Go API |
| `services/go-api-server` | Go, Chi, pgx, FFmpeg | API (under `/api`), auth, streaming, stations and queues; serves the web UI |
| `services/python-analytics` | Python, FastAPI, Essentia, librosa | Internal API plus a worker that scans the library, extracts audio features and scores station genres |
| `database` | Postgres + pgvector | Data and the worker's task queue |

## Android app

A native app for phones and Android Auto lives in [`apps/android`](apps/android). Download the signed APK from the [Releases](https://github.com/FelixClements/OwnWave/releases) page (tags `android-v…`); install steps, including Android Auto, are in [`apps/android/README.md`](apps/android/README.md#install).

## Quick start

Requires Docker and a folder of audio files.

```bash
cp .env.example .env
# Edit .env with your MUSIC_PATH and secrets

docker compose up --build -d
# Once healthy, open http://localhost:8080
```

## Development

```bash
# Go API
cd services/go-api-server
go test ./...
go build .

# Web UI (Vite + React; builds to dist/, which the Go image serves)
cd apps/web
npm install
npm test
npm run build

# Web UI with hot reload: start the stack, then run Vite's dev server,
# which proxies /api to Go at GO_API_URL (default http://localhost:8080)
npm run dev

# After changing services/go-api-server/openapi.yaml, regenerate the typed client
npm run generate:api

# Full smoke test (builds Docker images and checks /health)
just test
```

## Production

See [`docs/DEPLOY.md`](docs/DEPLOY.md) for:

- Caddy reverse proxy with automatic HTTPS
- Docker Compose override (`docker-compose.prod.yml`)
- Postgres backup and restore scripts

## CI/CD

A GitHub Actions workflow in `.github/workflows/ci.yml` lints and tests the Go code, builds the frontend, builds Docker images, and publishes them to GHCR on pushes to `main`.

## License

See `LICENSE`.
