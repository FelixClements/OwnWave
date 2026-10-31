# OwnWave

An open-source, self-hosted smart radio. Turn a local library of lossless audio into personalized, continuous radio stations.

## What it does

- Scans a local music folder and extracts rich metadata and audio features (BPM, key, energy, valence).
- Builds stations from Spotify playlists: admins add public playlist URLs, OwnWave syncs them daily and finds each track in the local library, and a station plays the local matches of one or more playlists. Playlists only grow: tracks Spotify rotates out are kept. Personal stations from a seed (artist, album, similar track, or mood) work too.
- A station is live: it holds every library track matching its seed, plays them shuffled with tracks you have not heard recently first, and reshuffles when the queue runs out.
- Streams raw FLAC or on-the-fly transcodes to MP3, Opus, or AAC per client.
- Tracks listening history and likes; banned tracks are never queued again.
- Provides a responsive web player and an admin dashboard for monitoring and maintenance.

## Architecture

| Layer | Technology | Role |
|-------|------------|------|
| `apps/t3-web-frontend` | Next.js (static export), Tailwind, React Query | Web UI and player (crossfade in the browser), served by the Go API |
| `services/go-api-server` | Go, Chi, pgx, FFmpeg | API (under `/api`), auth, streaming, stations and queues; serves the web UI |
| `services/python-analytics` | Python, FastAPI, librosa, scikit-learn | Internal API plus a worker that scans the library, extracts audio features, genres and clusters |
| `database` | Postgres + pgvector | Data and the worker's task queue |

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

# Frontend (static export to out/)
cd apps/t3-web-frontend
npm install
npm run build

# Frontend with hot reload: start the stack, then run Next's dev server,
# which proxies /api to Go at GO_API_URL (default http://localhost:8080)
npm run dev

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
