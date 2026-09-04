# OwnWave Security Remediation Design

**Date:** 2026-09-04  
**Status:** Approved  
**Scope:** Remediation of 9 security audit findings across Python Analytics, Go API, Caddy, Next.js / tRPC, and Database.

---

## 1. Context & Objectives

A comprehensive repository audit identified 9 vulnerabilities and technical debt items across the stack:
1. **Critical:** Python library scan path resolution bug breaking directory scans (`services/python-analytics/src/api.py:44`).
2. **High:** Caddy `X-Forwarded-For` appending allows clients to bypass Chi `httprate.LimitByIP` (`Caddyfile:28`).
3. **High:** Uncapped `ffmpeg` cover art extraction allows CPU/memory denial-of-service (`services/go-api-server/handlers_tracks.go:177`).
4. **Medium:** Non-constant-time token check for `ANALYTICS_API_SECRET` (`services/python-analytics/src/api.py:27`).
5. **Medium:** Expired database sessions and user invites are never purged from Postgres (`services/go-api-server/internal/auth/service.go`).
6. **Medium:** Dead `internal/streamauth` package and discarded `loadJWTSecret()` check in `main.go`.
7. **Low:** Broken tRPC `login` and `register` procedures that fail to forward `Set-Cookie` (`apps/t3-web-frontend/server/routers/app.ts`).
8. **Low:** Information disclosure of total track catalog count on public `/setup/status` (`services/go-api-server/handlers_setup.go`).
9. **Low:** Content Security Policy missing `object-src 'none'` and `base-uri 'self'` (`Caddyfile:8`).

---

## 2. Architecture & Subsystem Specifications

### 2.1 Python Analytics Service
- **Path Resolution:** In `services/python-analytics/src/api.py`, fix `_resolved_scan_path(path: str) -> str`:
  - When given an absolute path that is already within `MUSIC_DIR` (such as `/music`), resolve it directly and verify `is_relative_to(music)`.
  - When given a relative path or subpath, join it against `music` without duplicating leading segments.
  - Require that the resolved candidate exists and is relative to `music`.
- **Constant-Time Secret Verification:** In `require_internal_token`, use `secrets.compare_digest(request.headers.get("x-internal-token", ""), ANALYTICS_API_SECRET)` to eliminate timing side-channels.

### 2.2 Edge & Ingress Networking (Caddy)
- **`X-Forwarded-For` Sanitization:** In `Caddyfile`, add `header_up X-Forwarded-For {http.request.remote.host}` to all `reverse_proxy` directives (`web:3000` and `go:8080`). This replaces any untrusted incoming client header with the genuine socket remote address, securing Chi's `middleware.RealIP` and `httprate.LimitByIP`.
- **Content Security Policy:** Update the CSP in `Caddyfile` to include `object-src 'none'; base-uri 'self';`.

### 2.3 Go API Server: Media, Throttling & Maintenance
- **Cover Art Protection:**
  - In `services/go-api-server/handlers_tracks.go`, implement a bounded concurrency semaphore (maximum 4 concurrent `ffmpeg` image extractions).
  - Add an in-memory thread-safe LRU/bounded cache for extracted cover art bytes.
  - In `services/go-api-server/main.go`, add `httprate.LimitByIP(60, time.Minute)` to `GET /tracks/{id}/cover`.
- **Database Housekeeping:**
  - In `services/go-api-server/internal/auth/service.go`, add `PurgeExpired(ctx context.Context) (int64, error)` executing `DELETE FROM sessions WHERE expires_at < NOW()` and `DELETE FROM user_invites WHERE expires_at < NOW()`.
  - In `services/go-api-server/main.go`, run `PurgeExpired` periodically (hourly) in a non-blocking background goroutine.
- **Dead Code Cleanup:**
  - Remove the unused `services/go-api-server/internal/streamauth` package and tests.
  - Remove the unused `loadJWTSecret()` call from `main.go` and clean up dead references.

### 2.4 Frontend & API Metadata
- **tRPC Procedures Cleanup:**
  - In `apps/t3-web-frontend/server/routers/app.ts`, delete the `login` and `register` procedures. Authentication is handled cleanly via direct `/api/login` and `/api/register` HTTP calls.
- **Setup Metadata Privacy:**
  - In `services/go-api-server/handlers_setup.go`, when `setup_completed` is true, omit `track_count` unless an authenticated admin is making the request.

---

## 3. Testing Strategy

1. **Python Analytics Unit Tests:**
   - Test `_resolved_scan_path` with `/music`, `/music/rock`, `rock`, and path traversal attempts (`/etc/passwd`, `../../escape`).
   - Test `require_internal_token` with valid and invalid tokens using `secrets.compare_digest`.
2. **Go API Unit & Integration Tests:**
   - Test `PurgeExpired` in `internal/auth` service tests.
   - Test cover cache concurrency and semaphore limiting.
   - Verify all existing Go test suites pass (`go test ./...`).
3. **Frontend Tests:**
   - Verify `npm test` and `npm run build` pass cleanly with `appRouter` updated.
4. **Caddyfile Validation:**
   - Verify `caddy validate` if Caddy is present, and check header syntax.
