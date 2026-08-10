# OwnWave

An open-source, self-hosted smart radio. Turn a local library of lossless audio into personalized, continuous radio stations.

## What it does

- Scans a local music folder and extracts rich metadata and audio features (BPM, key, energy, valence).
- Builds AI-driven stations that queue cohesive, similar tracks from a seed.
- Streams raw FLAC or on-the-fly transcodes to MP3, Opus, or AAC per client.
- Tracks listening history, likes, skips, and bans to improve recommendations.
- Provides a responsive web player and an admin dashboard for monitoring and maintenance.

## Architecture

| Layer | Technology | Role |
|-------|------------|------|
| `apps/t3-web-frontend` | Next.js, Tailwind, tRPC | Web UI and player |
| `services/go-api-server` | Go, Chi, pgx, FFmpeg | API, auth, streaming, queues |
| `services/python-analytics` | Python, FastAPI, librosa, scikit-learn | Scanning, feature extraction, AI clustering, station generation |
| `database` | Postgres + pgvector, Redis | Data and task broker |

## Quick start

Requires Docker and a folder of audio files.

```bash
cp .env.example .env
# Edit .env with your MUSIC_PATH and secrets

docker compose up --build -d
# Once healthy, open http://localhost:3000
```

## Development

```bash
# Go API
cd services/go-api-server
go test ./...
go build .

# Frontend
cd apps/t3-web-frontend
npm install
npm run build

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
