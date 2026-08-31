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
JWT_SECRET=<generate-a-strong-secret-at-least-32-chars>
ALLOWED_ORIGINS=https://ownwave.example.com
PUBLIC_APP_URL=https://ownwave.example.com
MUSIC_PATH=/path/to/music
```

Generate a strong `JWT_SECRET` (at least 32 characters) and `POSTGRES_PASSWORD` before starting. The Go API refuses to start with missing or default JWT secrets.

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

Prometheus metrics are exposed internally on the Go container at port `9090` (`/metrics`) and to authenticated admins at `/api/admin/metrics`. They are not mounted on the public Caddy routes.

## 4. Security model

- All library, station, and streaming URL endpoints require a logged-in session (Bearer token).
- Stream playback uses short-lived JWT capability tokens minted by authenticated users.
- Registration is invite-only after the first admin account is created during setup.
- Admin maintenance endpoints require `is_admin`.
- Caddy adds standard security headers (HSTS, CSP, frame denial, etc.).

### Post-deploy verification checklist

```bash
DOMAIN=https://ownwave.example.com

# Should return 401
curl -s -o /dev/null -w "%{http_code}" "$DOMAIN/api/tracks"

# Should return 401 or 403
curl -s -o /dev/null -w "%{http_code}" -X POST "$DOMAIN/api/admin/scan"

# Open registration should be forbidden once setup has users
curl -s -o /dev/null -w "%{http_code}" -X POST "$DOMAIN/api/register" \
  -H 'Content-Type: application/json' \
  -d '{"username":"intruder","password":"password123"}'

# Public metrics should not be reachable
curl -s -o /dev/null -w "%{http_code}" "$DOMAIN/api/metrics"
```

Expected status codes: `401`, `401` or `403`, `403`, and `404` respectively.

Invite flow:

1. Sign in as admin → Admin → Generate invite link.
2. Open the invite URL in a private window and create an account.
3. Confirm the new user can sign in and browse the library.

## 5. Backups

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

## 6. Restore

Stop the app consumers, then restore from a backup:

```bash
./scripts/restore.sh /var/backups/ownwave/ownwave_backup_YYYYMMDD_HHMMSS.sql
```

## 7. Updates

Pull the latest code, then rebuild:

```bash
git pull
docker compose -f docker-compose.yml -f docker-compose.prod.yml up -d --build
```
