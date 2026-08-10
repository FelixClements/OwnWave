# Production Deployment Guide

This guide covers deploying OwnWave on a single server using Docker Compose with a Caddy reverse proxy, automatic HTTPS, and scheduled backups.

## Requirements

- A Linux server with Docker and Docker Compose installed.
- A DNS `A` or `CNAME` record pointing to the server.
- Ports `80` and `443` available.
- A `music/` directory containing your audio files.

## 1. Environment

Copy the example file and edit it:

```bash
cp .env.example .env
```

Key variables:

```env
OWNWAVE_DOMAIN=ownwave.example.com
POSTGRES_PASSWORD=<generate-a-strong-password>
JWT_SECRET=<generate-a-strong-secret>
MUSIC_PATH=/path/to/music
```

Generate a strong `JWT_SECRET` and `POSTGRES_PASSWORD` before starting.

## 2. Start the stack

```bash
docker compose -f docker-compose.yml -f docker-compose.prod.yml up -d
```

Caddy will automatically obtain and renew a TLS certificate for `OWNWAVE_DOMAIN` using Let's Encrypt.

## 3. Reverse proxy and routing

Caddy is configured in `Caddyfile`:

- `https://<OWNWAVE_DOMAIN>/` → Next.js frontend
- `https://<OWNWAVE_DOMAIN>/api/trpc/*` → Next.js tRPC endpoint
- `https://<OWNWAVE_DOMAIN>/api/*` → Go API (the `/api` prefix is stripped)

The `web` service uses `NEXT_PUBLIC_GO_API_URL=https://<OWNWAVE_DOMAIN>/api` so the browser calls the same host, and `GO_API_URL=http://go:8080` for server-side requests.

## 4. Backups

### Manual backup

```bash
./scripts/backup.sh /path/to/backup/dir
```

This produces a timestamped `pg_dump` SQL file.

### Scheduled backups

Add a cron job on the host:

```cron
0 3 * * * cd /opt/ownwave && ./scripts/backup.sh /var/backups/ownwave
```

## 5. Restore

Stop the app consumers, then restore from a backup:

```bash
./scripts/restore.sh /var/backups/ownwave/ownwave_backup_YYYYMMDD_HHMMSS.sql
```

## 6. Updates

Pull the latest code, then rebuild:

```bash
git pull
docker compose -f docker-compose.yml -f docker-compose.prod.yml up -d --build
```
