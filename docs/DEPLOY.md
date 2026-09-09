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
ALLOWED_ORIGINS=https://ownwave.example.com
PUBLIC_APP_URL=https://ownwave.example.com
MUSIC_PATH=/path/to/music
```

Generate a strong `POSTGRES_PASSWORD` and unique `ANALYTICS_API_SECRET` before starting.

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

Prometheus metrics are available to authenticated admins at `/api/admin/metrics`. They are not mounted on the public Caddy routes.

## 4. Security model

- The browser talks to a single origin. Caddy proxies `/api/*` to Go (except `/api/trpc`). Go sets an HttpOnly `ownwave_session` cookie; JSON APIs never return the raw session token.
- Library, station, history, feedback, and stream endpoints require that cookie (or `Authorization: Bearer` for scripts).
- Stream URLs are cookie-gated (`/api/stream/{id}`). Unauthenticated requests return 401.
- The music catalog is shared. Stations, listening history, and likes/bans/skips are scoped to the signed-in user.
- Python analytics is an internal service. Go sends `X-Internal-Token` (`ANALYTICS_API_SECRET`) and `X-OwnWave-User-Id` on station writes.
- Registration is invite-only after the first admin account is created during setup.
- Admin maintenance endpoints require `is_admin`.
- Caddy adds standard security headers (HSTS, CSP, frame denial, etc.). CSP still includes `'unsafe-inline'` for Next.js.

### Post-deploy verification checklist

```bash
DOMAIN=https://ownwave.example.com

# Should return 401
curl -s -o /dev/null -w "%{http_code}" "$DOMAIN/api/tracks"

# Stream without a session cookie should return 401
curl -s -o /dev/null -w "%{http_code}" "$DOMAIN/api/stream/00000000-0000-0000-0000-000000000001"

# Should return 401 or 403
curl -s -o /dev/null -w "%{http_code}" -X POST "$DOMAIN/api/admin/scan"

# Open registration should be forbidden once setup has users
curl -s -o /dev/null -w "%{http_code}" -X POST "$DOMAIN/api/register" \
  -H 'Content-Type: application/json' \
  -d '{"username":"intruder","password":"password123"}'

# Public metrics should not be reachable
curl -s -o /dev/null -w "%{http_code}" "$DOMAIN/api/metrics"
```

Expected status codes: `401`, `401`, `401` or `403`, `403`, and `404` respectively.

Set `OWNWAVE_COOKIE_SECURE=true` in production (the prod compose overlay does this). Generate a unique `ANALYTICS_API_SECRET` and `POSTGRES_PASSWORD`.

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
