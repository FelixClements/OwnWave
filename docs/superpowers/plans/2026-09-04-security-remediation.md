# Security Remediation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remediate all 9 security audit findings across Python Analytics, Go API, Caddy, Next.js / tRPC, and PostgreSQL database.

**Architecture:** The fixes are implemented across isolated layers: Python Analytics handles scan path resolution and constant-time secret validation; Caddy ingress sanitizes `X-Forwarded-For` and strengthens CSP; Go API implements bounded cover extraction, rate limiting, and periodic expired session purging while removing dead streamauth code; and Next.js removes non-cookie-forwarding tRPC auth mutations.

**Tech Stack:** Go 1.25 (Chi, pgx/v5, httprate), Python 3.11 (FastAPI, secrets, pathlib), Next.js 14 / tRPC v10, Caddy 2, PostgreSQL 15.

## Global Constraints

- Preserve all existing public API and frontend authentication contracts (HttpOnly cookies with SameSite=Lax).
- Do not introduce new external dependencies or libraries.
- Ensure all tests in Go (`go test ./...`) and Frontend (`npm test`, `npm run build`) pass.
- All SQL operations must remain parameterized.

---

### Task 1: Fix Python Scan Path Resolution and Constant-Time Secret Check

**Files:**
- Modify: `services/python-analytics/src/api.py:20-50`
- Create: `services/python-analytics/tests/test_scan_path.py`

**Interfaces:**
- Consumes: `MUSIC_DIR` from `config.py`, `ANALYTICS_API_SECRET` from `config.py`
- Produces: Sanitized `_resolved_scan_path(path: str) -> str` and constant-time auth check in `require_internal_token`

- [ ] **Step 1: Write tests for `_resolved_scan_path` and `require_internal_token`**

In `services/python-analytics/tests/test_scan_path.py`:
```python
from pathlib import Path
import pytest
from fastapi.testclient import TestClient

def test_resolved_scan_path_default(monkeypatch, tmp_path):
    import config
    import api
    music_dir = tmp_path / "music"
    music_dir.mkdir()
    monkeypatch.setattr(config, "MUSIC_DIR", str(music_dir))
    monkeypatch.setattr(api, "MUSIC_DIR", str(music_dir))

    resolved = api._resolved_scan_path(str(music_dir))
    assert resolved == str(music_dir.resolve())

def test_resolved_scan_path_relative(monkeypatch, tmp_path):
    import config
    import api
    music_dir = tmp_path / "music"
    rock_dir = music_dir / "rock"
    rock_dir.mkdir(parents=True)
    monkeypatch.setattr(config, "MUSIC_DIR", str(music_dir))
    monkeypatch.setattr(api, "MUSIC_DIR", str(music_dir))

    resolved = api._resolved_scan_path("rock")
    assert resolved == str(rock_dir.resolve())

def test_resolved_scan_path_traversal(monkeypatch, tmp_path):
    import config
    import api
    music_dir = tmp_path / "music"
    music_dir.mkdir()
    monkeypatch.setattr(config, "MUSIC_DIR", str(music_dir))
    monkeypatch.setattr(api, "MUSIC_DIR", str(music_dir))

    with pytest.raises(Exception):
        api._resolved_scan_path("../../etc/passwd")
```

- [ ] **Step 2: Implement the fixes in `services/python-analytics/src/api.py`**

```python
import secrets

@app.middleware("http")
async def require_internal_token(request: Request, call_next):
    if request.url.path == "/health":
        return await call_next(request)
    token = request.headers.get("x-internal-token", "")
    if not ANALYTICS_API_SECRET or not secrets.compare_digest(token, ANALYTICS_API_SECRET):
        return JSONResponse({"detail": "unauthorized"}, status_code=401)
    return await call_next(request)

def _resolved_scan_path(path: str) -> str:
    music = Path(MUSIC_DIR).resolve()
    candidate = Path(path).resolve()
    if not candidate.is_relative_to(music):
        candidate = music.joinpath(path.lstrip("/")).resolve()
    if not candidate.is_relative_to(music) or not candidate.exists():
        raise HTTPException(status_code=400, detail="path outside music directory or does not exist")
    return str(candidate)
```

- [ ] **Step 3: Run the test suite**

Run: `PYTHONPATH=services/python-analytics/src pytest services/python-analytics/tests/test_scan_path.py`
Expected: PASS

---

### Task 2: Sanitize Ingress `X-Forwarded-For` and Strengthen CSP in Caddy

**Files:**
- Modify: `Caddyfile:1-35`

**Interfaces:**
- Consumes: Caddy HTTP request headers
- Produces: Sanitized `X-Forwarded-For` passed upstream to Go API and Next.js; hardened CSP

- [ ] **Step 1: Update `Caddyfile`**

In `Caddyfile`:
```caddy
{$OWNWAVE_DOMAIN:localhost} {
    header {
        Strict-Transport-Security "max-age=31536000; includeSubDomains"
        X-Content-Type-Options "nosniff"
        X-Frame-Options "DENY"
        Referrer-Policy "strict-origin-when-cross-origin"
        Permissions-Policy "camera=(), microphone=(), geolocation=()"
        Content-Security-Policy "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; media-src 'self' blob:; connect-src 'self'; font-src 'self' data:; object-src 'none'; base-uri 'self'; frame-ancestors 'none'"
    }

    log {
        format filter {
            wrap json
            fields {
                request>uri regexp `/invite/[^/?]+` `/invite/REDACTED`
            }
        }
    }

    # tRPC requests go to the Next.js frontend server
    handle /api/trpc* {
        reverse_proxy web:3000 {
            header_up X-Forwarded-For {http.request.remote.host}
        }
    }

    # All other /api/* routes go to the Go API (strip the /api prefix)
    handle_path /api/* {
        reverse_proxy go:8080 {
            header_up X-Forwarded-For {http.request.remote.host}
        }
    }

    # Everything else is served by the Next.js frontend
    handle {
        reverse_proxy web:3000 {
            header_up X-Forwarded-For {http.request.remote.host}
        }
    }
}
```

- [ ] **Step 2: Verify Caddy configuration syntax**

Ensure directives follow standard Caddy v2 syntax.

---

### Task 3: Cover Art Extraction Protection & Concurrency Semaphore

**Files:**
- Modify: `services/go-api-server/handlers_tracks.go:160-195`
- Modify: `services/go-api-server/main.go:165-195`
- Create: `services/go-api-server/internal/streaming/cover_cache.go`
- Create: `services/go-api-server/internal/streaming/cover_cache_test.go`

**Interfaces:**
- Consumes: Track audio file path
- Produces: Thread-safe in-memory cover art cache and bounded ffmpeg concurrency semaphore

- [ ] **Step 1: Write cover cache with concurrency limiter**

In `services/go-api-server/internal/streaming/cover_cache.go`:
```go
package streaming

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"sync"
)

type CoverCache struct {
	mu      sync.RWMutex
	cache   map[string][]byte
	sem     chan struct{}
	maxSize int
}

func NewCoverCache(maxEntries int, maxConcurrent int) *CoverCache {
	return &CoverCache{
		cache:   make(map[string][]byte),
		sem:     make(chan struct{}, maxConcurrent),
		maxSize: maxEntries,
	}
}

func (c *CoverCache) GetOrExtract(ctx context.Context, ffmpegPath, filePath string) ([]byte, error) {
	c.mu.RLock()
	if data, ok := c.cache[filePath]; ok {
		c.mu.RUnlock()
		return data, nil
	}
	c.mu.RUnlock()

	select {
	case c.sem <- struct{}{}:
		defer func() { <-c.sem }()
	case <-ctx.Context().Done():
		return nil, ctx.Err()
	}

	// Double check
	c.mu.RLock()
	if data, ok := c.cache[filePath]; ok {
		c.mu.RUnlock()
		return data, nil
	}
	c.mu.RUnlock()

	cmd := exec.CommandContext(ctx, ffmpegPath, "-i", filePath, "-an", "-vcodec", "mjpeg", "-f", "image2", "-", "-v", "0")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil || out.Len() == 0 {
		return nil, errors.New("no cover art")
	}

	data := out.Bytes()
	c.mu.Lock()
	if len(c.cache) < c.maxSize {
		c.cache[filePath] = data
	}
	c.mu.Unlock()

	return data, nil
}
```

- [ ] **Step 2: Add unit tests for `CoverCache`**

Verify cache hit does not re-extract and semaphore bounds concurrency.

- [ ] **Step 3: Wire `CoverCache` and rate limit in `handlers_tracks.go` and `main.go`**

In `main.go`:
Wrap `r.Get("/tracks/{id}/cover", h.GetTrackCover)` in `httprate.LimitByIP(60, time.Minute)`.

- [ ] **Step 4: Run Go unit tests**

Run: `cd services/go-api-server && go test ./...`
Expected: PASS

---

### Task 4: Expired Sessions & Invites Periodic Purge

**Files:**
- Modify: `services/go-api-server/internal/auth/service.go:360-395`
- Modify: `services/go-api-server/main.go:100-130`
- Modify: `services/go-api-server/internal/auth/service_integration_test.go`

**Interfaces:**
- Consumes: PostgreSQL connection pool
- Produces: `PurgeExpired(ctx context.Context) (int64, error)` method and background cleanup loop

- [ ] **Step 1: Write `PurgeExpired` in `internal/auth/service.go`**

```go
func (s *Service) PurgeExpired(ctx context.Context) (int64, error) {
	tag1, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE expires_at < NOW()`)
	if err != nil {
		return 0, err
	}
	tag2, err := s.pool.Exec(ctx, `DELETE FROM user_invites WHERE expires_at < NOW()`)
	if err != nil {
		return tag1.RowsAffected(), err
	}
	return tag1.RowsAffected() + tag2.RowsAffected(), nil
}
```

- [ ] **Step 2: Launch periodic background purge in `main.go`**

```go
go func() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()
	for range ticker.C {
		if _, err := h.auth.PurgeExpired(context.Background()); err != nil {
			slog.Error("failed to purge expired sessions", "error", err)
		}
	}
}()
```

- [ ] **Step 3: Run auth unit and integration tests**

Run: `cd services/go-api-server && go test ./internal/auth/...`
Expected: PASS

---

### Task 5: Remove Dead `streamauth` Package and Unused `JWT_SECRET` Validation

**Files:**
- Delete: `services/go-api-server/internal/streamauth/streamtoken.go`
- Delete: `services/go-api-server/internal/streamauth/streamtoken_test.go`
- Modify: `services/go-api-server/main.go:47-60, 104`

**Interfaces:**
- Cleans up dead code removed when streaming transitioned to session cookies in PR #51

- [ ] **Step 1: Remove `internal/streamauth` files**

Remove `streamtoken.go` and `streamtoken_test.go`.

- [ ] **Step 2: Remove discarded `loadJWTSecret()` call from `main.go`**

Remove line 104 `loadJWTSecret()` and the helper function since JWT stream tokens are no longer used.

- [ ] **Step 3: Run Go tests to confirm clean build**

Run: `cd services/go-api-server && go test ./...`
Expected: PASS

---

### Task 6: Clean Up tRPC Auth Procedures and Restrict `/setup/status` Disclosure

**Files:**
- Modify: `apps/t3-web-frontend/server/routers/app.ts:40-60`
- Modify: `services/go-api-server/handlers_setup.go:10-40`

**Interfaces:**
- Consumes: Next.js tRPC procedures and Go API setup endpoint
- Produces: Cleaner API boundary without broken tRPC cookies, and sanitized public setup status

- [ ] **Step 1: Remove `login` and `register` procedures in `app.ts`**

In `apps/t3-web-frontend/server/routers/app.ts`, delete the `register` and `login` tRPC mutations.

- [ ] **Step 2: Sanitize `/setup/status` in `handlers_setup.go`**

```go
func (h *Handler) SetupStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	hasUsers, err := h.db.CountUsers(ctx)
	if err != nil {
		writeInternalError(w, r, "setup status users", err)
		return
	}
	state, _ := h.db.GetAppState(ctx, "setup_completed")
	completed := false
	if state != nil {
		if v, ok := state["completed"].(bool); ok {
			completed = v
		}
	}

	resp := map[string]interface{}{
		"setup_completed": completed,
		"has_users":       hasUsers > 0,
	}

	// Only disclose track count during initial setup before completion
	if !completed {
		trackCount, err := h.db.CountTracks(ctx)
		if err == nil {
			resp["track_count"] = trackCount
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
```

- [ ] **Step 3: Verify frontend tests and build**

Run: `cd apps/t3-web-frontend && npm test && npm run build`
Expected: PASS

---

### Task 7: Full Verification Across All Services

- [ ] **Step 1: Run all Go unit and integration tests**
Run: `cd services/go-api-server && go test ./...`
Expected: PASS

- [ ] **Step 2: Run frontend tests**
Run: `cd apps/t3-web-frontend && npm test`
Expected: PASS

- [ ] **Step 3: Verify Canvas state and audit findings**
Open [security-audit](/Users/maarten/.cursor/projects/Users-maarten-Documents-Github-OwnWave/canvases/security-audit.canvas.tsx) and update all items to reflect resolved status.
