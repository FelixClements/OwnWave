# Go API research: playback tests, streaming extraction, auth extraction

Research date: 2026-08-22. Repository: `services/go-api-server` in OwnWave.

---

## 1. Playback integration tests

### Current state

`internal/playback/service.go` holds the rotation logic.

- `Record` (lines 21–47) opens a transaction, inserts into `listening_history`, and when `stationID` is non-empty updates `station_tracks.played_at` to `NOW()`.
- `BuildQueue` (lines 49–63) calls `queryStationQueue` with `recentHours`, shuffles the result, and if the filtered queue is empty retries with `recentHours = 0` (full pool fallback).
- `queryStationQueue` (lines 65–121) joins `station_tracks`, `tracks`, `audio_features`, artists, albums; excludes banned tracks via `track_feedback`; optionally filters `st.played_at IS NULL OR st.played_at < NOW() - INTERVAL '1 hour' * $2`; deduplicates by lowercase `title|artist`.

HTTP wiring:

| Handler | File | Lines | Calls |
|---------|------|-------|-------|
| `RecordPlay` | `handlers_tracks.go` | 65–76 | `h.playback.Record(ctx, trackID, req.StationID)` |
| `GetQueue` | `handlers_stations.go` | 47–56 | `h.playback.BuildQueue(ctx, id, h.recentHours)` |
| `StationCrossfadeStream` | `handlers_streaming.go` | 133–169 | `h.playback.BuildQueue` then `h.serveCrossfaded` |

Schema (from `database/migrations/`):

- `station_tracks` (`001_schema.up.sql` lines 58–64): `station_id`, `track_id`, `position`, `played_at`.
- `listening_history` (`006_add_interactions.up.sql` lines 1–6): `track_id`, `station_id`, `played_at`.

Tests today:

- `handlers_test.go`: `TestHashToken`, `TestGetString` (JWT claim helper, not playback).
- `streaming_test.go`: `TestNormalizeBitrate`, `TestVolumeGainDb` (audio helpers, not playback).
- `internal/analytics/client_test.go`: HTTP stub tests with `httptest`, no database.

CI (`.github/workflows/ci.yml` lines 42–45): `cd services/go-api-server && go test ./...`. No Postgres service, no Docker-in-test step. Default `go test` skips anything behind a build tag unless invoked explicitly.

`recentHours` defaults to 24 via `Handler` construction (`handler.go` lines 29–32) and `STATION_RECENT_HOURS` env (`main.go` lines 78–83).

### Gap / problem

`playback.Service` is the only code that ties `listening_history` inserts to `station_tracks.played_at` updates and implements the recent-play filter plus fallback. None of that is covered by tests. A regression in the transaction, the SQL filter, or the empty-queue retry would ship unnoticed. Unit tests with mocks would not catch SQL mistakes against the real schema (joins, `INTERVAL` arithmetic, ban exclusion).

### Options table

| Option | Pros | Cons |
|--------|------|------|
| **testcontainers-go** (`modules/postgres`) | Real Postgres per test run; programmatic lifecycle; snapshot/restore for fast resets; works locally and in CI with Docker | Requires Docker; slower than pure unit tests; pgvector image needed (`pgvector/pgvector:pg15` matches `docker-compose.yml`) |
| **GitHub Actions `services: postgres`** | No testcontainers dependency; native GHA pattern | Must wire `DATABASE_URL` manually; no snapshot API; harder to match pgvector extensions; tests less portable off CI |
| **`-tags=integration` + env `TEST_DATABASE_URL`** | Simple; devs can point at local compose DB | Flaky without isolated DB; shared state between tests; CI still needs a DB |
| **pgxmock** (`github.com/pashagolub/pgxmock`) | Fast; no Docker | Mocks SQL strings, not schema; poor fit for multi-statement transactions and complex joins |
| **testify only (no DB)** | Already a transitive dep via testcontainers | Cannot test `Record`/`BuildQueue` without a database adapter behind an interface (extra refactor, still needs real DB somewhere) |

### Recommended approach + migration steps

Use **testcontainers-go** with the **pgvector** image, **`//go:build integration`** build tags, and **testify** for assertions and cleanup helpers.

1. Add dev dependencies to `services/go-api-server`:
   ```
   go get github.com/testcontainers/testcontainers-go/modules/postgres
   go get github.com/stretchr/testify
   ```
2. Create `internal/playback/testutil_test.go` (build tag `integration`) with `setupTestDB(t)`:
   - Start `postgres.Run(ctx, "pgvector/pgvector:pg15", ...)` with `BasicWaitStrategies()` and `WithSQLDriver("pgx")` per [testcontainers Postgres docs](https://golang.testcontainers.org/modules/postgres/).
   - Apply migrations via `postgres.WithOrderedInitScripts` pointing at `../../../database/migrations/*.up.sql` (ordered), or call existing `runMigrations(dsn)` from a test helper with `MIGRATIONS_PATH` set.
   - Optionally `Snapshot`/`Restore` after seeding base fixtures for per-test isolation.
   - Return `*pgxpool.Pool`; register `t.Cleanup` to terminate the container.
3. Create `internal/playback/service_integration_test.go`:
   ```go
   //go:build integration

   package playback_test
   ```
4. Add seed helpers: insert `stations`, `tracks`, `audio_features`, `station_tracks` rows (minimal columns required by `queryStationQueue` scan list).
5. Update CI `.github/workflows/ci.yml`:
   ```yaml
   - name: Test Go (unit)
     run: cd services/go-api-server && go test ./...
   - name: Test Go (integration)
     run: cd services/go-api-server && go test -tags=integration ./internal/playback/...
   ```
   Integration step needs Docker (ubuntu-latest runners provide it). No separate `services: postgres` block required when using testcontainers.
6. Document in `services/go-api-server/README` or `AGENTS.md`: `go test -tags=integration ./internal/playback/...` for local runs (Docker required).

#### Exact test cases

| Test | Setup | Action | Assert |
|------|-------|--------|--------|
| `TestRecord_WritesListeningHistory` | Seed track | `Record(ctx, trackID, "")` | Row in `listening_history` with `track_id`, `station_id IS NULL` |
| `TestRecord_UpdatesStationTracksPlayedAt` | Seed station + track + `station_tracks` row with `played_at NULL` | `Record(ctx, trackID, stationID)` | `station_tracks.played_at` not null; `listening_history.station_id` matches |
| `TestRecord_TransactionAtomic` | Seed station + track | Simulate failure (e.g. invalid `station_id` on update) | No `listening_history` row if transaction rolls back (optional hardening test) |
| `TestBuildQueue_ExcludesRecentlyPlayed` | Station with tracks A (played 1h ago) and B (never played); `recentHours=24` | `BuildQueue` | Result contains B, not A |
| `TestBuildQueue_FallbackWhenAllPlayed` | Station with only recently played tracks | `BuildQueue` with `recentHours=24` | Non-empty queue (fallback to `recentHours=0`) |
| `TestBuildQueue_ExcludesBanned` | Track with `track_feedback.feedback='ban'` | `BuildQueue` | Banned track absent |
| `TestBuildQueue_DeduplicatesTitleArtist` | Two `station_tracks` rows same title+artist | `BuildQueue` | One entry in result |

Handler-level integration tests (`RecordPlay` HTTP 204, `GetQueue` JSON) can follow in `handlers_integration_test.go` once `playback` tests pass. Start at the service layer; it is the deep module with the SQL.

### Tests that survive

Unchanged: `handlers_test.go`, `streaming_test.go`, `internal/analytics/client_test.go`. New integration files are additive behind the build tag.

### Sources

- OwnWave `internal/playback/service.go`: repository file
- OwnWave `database/migrations/001_schema.up.sql`, `006_add_interactions.up.sql`: repository files
- OwnWave `.github/workflows/ci.yml`: repository file
- Go testing tutorial: https://go.dev/doc/tutorial/add-a-test
- Go build constraints: https://go.dev/doc/go1.17#build-tags
- testcontainers-go Postgres module: https://golang.testcontainers.org/modules/postgres/
- testcontainers-go `postgres.Run` API: https://pkg.go.dev/github.com/testcontainers/testcontainers-go/modules/postgres
- pgx contributing (env-based test DB pattern): https://github.com/jackc/pgx/blob/master/CONTRIBUTING.md
- testify: https://pkg.go.dev/github.com/stretchr/testify

---

## 2. `internal/streaming` extraction

### Current state

| File | Lines | Responsibility |
|------|-------|----------------|
| `streaming.go` | 1–320 | `serveFLAC` (14–30), `serveTranscoded` (32–108), `normalizeBitrate` (110–118), `volumeGainDb` (126–138), `serveCrossfaded` (140–320) |
| `handlers_streaming.go` | 1–193 | HTTP handlers; JWT sign/validate for track (`signStreamToken` 82–89, `validateStreamToken` 90–105) and station streams (`signStationStreamToken` 170–177, `validateStationStreamToken` 178–193) |
| `handler.go` | 19–27, 69–77 | `Handler` fields `musicDir`, `ffmpegPath`; `getString` for JWT claims |
| `db.go` | 21–23 | `type TrackWithFeatures = playback.TrackWithFeatures` alias |

`serveCrossfaded` takes `[]TrackWithFeatures` (main package alias). It resolves relative paths with `h.musicDir`, shells out to `h.ffmpegPath`, and uses `volumeGainDb` / `normalizeBitrate`.

`StationCrossfadeStream` (`handlers_streaming.go` 149) calls `playback.BuildQueue` then `h.serveCrossfaded`. Streaming handlers depend on playback for queue data but not vice versa.

Tests: `streaming_test.go` covers `normalizeBitrate` and `volumeGainDb` only. No tests for `serveFLAC`, ffmpeg pipelines, or crossfade filter graph.

### Gap / problem

`streaming.go` and `handlers_streaming.go` mix three concerns: HTTP routing, short-lived JWT stream authorization, and ffmpeg audio serving. The `Handler` struct in `main` is a shallow module: it exposes many unrelated methods and hides no streaming complexity. Tests for pure audio math live in `main` beside unrelated handler tests. Extracting a `streaming` package would create a seam where ffmpeg and path resolution are testable without HTTP, and would mirror the existing `internal/playback` and `internal/analytics` layout.

### Options table

| Option | Pros | Cons |
|--------|------|------|
| **Extract `internal/streaming` package** | Clear module boundary; pure helpers testable in-package; matches `internal/playback` pattern | One-time move; `Handler` needs a delegate field |
| **Keep in `main`, add interfaces** | Smaller diff | Interfaces in `main` without a second adapter are hypothetical seams (codebase-design: one adapter = no real seam) |
| **Merge streaming into `internal/playback`** | Single "audio playback" package | Playback is DB rotation; streaming is ffmpeg I/O. Violates locality. Large mixed module. |
| **Extract only pure functions** | Minimal move | `serve*` methods stay on `Handler`; boundary stays fuzzy |

### Recommended approach + migration steps

Extract a **deep `internal/streaming` module** with a small interface and ffmpeg behind it. Keep HTTP handlers and JWT stream tokens in `handlers_streaming.go` for now (auth topic covers JWT consolidation later).

**Proposed module API** (interface = what callers learn; implementation = ffmpeg + filesystem):

```go
package streaming

type Config struct {
    MusicDir   string
    FFmpegPath string
}

type Server struct { cfg Config }

func New(cfg Config) *Server

// ResolvePath joins musicDir when path is relative.
func (s *Server) ResolvePath(path string) string

func (s *Server) ServeFLAC(w http.ResponseWriter, r *http.Request, absPath string)
func (s *Server) ServeTranscoded(w http.ResponseWriter, r *http.Request, absPath, format string, loudness *float64, normalize bool, bitrate string)
func (s *Server) ServeCrossfaded(w http.ResponseWriter, r *http.Request, queue []playback.TrackWithFeatures, format, bitrate string, gapless, normalize bool)

// Pure functions, exported for tests:
func NormalizeBitrate(input, defaultRate string) string
func VolumeGainDb(loudness *float64, normalize bool) float64
```

**Seam placement:** HTTP handlers in `main` are adapters. They validate JWT, load track metadata from `db`, then call `streaming.Server` methods. `playback.TrackWithFeatures` is the shared DTO at the seam (already defined in `internal/playback/types.go`). No new type needed.

**Migration steps:**

1. Create `internal/streaming/server.go`; move `serveFLAC`, `serveTranscoded`, `serveCrossfaded` as methods on `Server`.
2. Move `normalizeBitrate` → `NormalizeBitrate`, `volumeGainDb` → `VolumeGainDb` into `internal/streaming/gain.go` (or same file).
3. Move `streaming_test.go` → `internal/streaming/gain_test.go`; update package and imports.
4. Add `stream *streaming.Server` to `Handler` (`handler.go`); initialize in `NewHandler` from `musicDir`/`ffmpegPath`.
5. Replace `h.serveFLAC(...)` etc. with `h.stream.ServeFLAC(...)` in handlers.
6. Remove `TrackWithFeatures` alias from `db.go` if nothing else in `main` needs it; handlers import `playback` directly where required.
7. Optional later: introduce `FFmpegRunner` interface inside `streaming` if you need to mock `exec.Command` in tests (internal seam, not exported).

**Vocabulary check:**

- **Module:** `internal/streaming` (package boundary).
- **Interface:** `Server` methods + `Config` (small surface, ~4 methods + 2 pure functions).
- **Seam:** between HTTP handlers (`main`) and audio output (`streaming.Server`).
- **Adapter:** `Handler` methods in `handlers_streaming.go` adapt HTTP + JWT to `streaming.Server`.
- **Depth:** crossfade filter graph (~180 lines) hidden behind `ServeCrossfaded`.

### Tests that survive

| File | Fate |
|------|------|
| `streaming_test.go` | Moves to `internal/streaming/gain_test.go` unchanged |
| `handlers_test.go` | Stays in `main` (`getString` used by stream JWT validation) |
| New tests (optional) | `internal/streaming/server_test.go` with fake `http.ResponseWriter` for `ServeFLAC` on a temp file; skip ffmpeg integration unless CI has ffmpeg |

Do not move JWT tests yet. Stream token sign/validate stays in `handlers_streaming.go` until auth extraction (section 3).

### Sources

- OwnWave `streaming.go`, `handlers_streaming.go`, `handler.go`, `db.go`: repository files
- Codebase-design skill (module, interface, seam, adapter, depth): `.agents/skills/codebase-design/SKILL.md`
- Go internal packages: https://go.dev/doc/go1.4#internalpackages
- `net/http` `ServeContent`: https://pkg.go.dev/net/http#ServeContent

---

## 3. `internal/auth` extraction

### Current state

| File | Lines | Responsibility |
|------|-------|----------------|
| `handlers_auth.go` | 12–222 | `Register`, `Login`, `Logout`, `Me`, `UpdateProfile`, `ChangePassword`; `authUser`, `createSession` |
| `handler.go` | 56–67 | `generateSessionToken` (32 random bytes, hex), `hashToken` (SHA-256 hex) |
| `db.go` | 209–291 | `User` struct; `CreateUser`, `GetUserByUsername`, `CreateSession`, `GetUserByTokenHash`, `DeleteSession`, password/profile updates |
| `handlers_streaming.go` | 82–105, 170–193 | Separate JWT HS256 stream tokens (`golang-jwt/jwt/v5`), 10-minute expiry, query param `token` |

**Session auth (Bearer):** Opaque token returned at login/register. Stored as SHA-256 hash in `sessions.token_hash` (`005_add_users.up.sql` lines 8–14). `authUser` (`handlers_auth.go` 197–211) reads `Authorization: Bearer`, hashes token, loads user via `GetUserByTokenHash` with expiry check. Used only by `/me`, `/logout`, `/me/profile`, `/me/password`.

**Stream auth (JWT query param):** Signed with `h.jwtSecret` (`handler.go` 23). Not stored in DB. Validated in `StreamTrack` and `StationCrossfadeStream` before serving audio. Different purpose: capability URL for `<audio src>` without Bearer headers.

**Router:** `main.go` registers all routes without auth middleware. Most endpoints (tracks, stations, stream URLs) are unauthenticated at the router level. Session auth is opt-in per handler.

Tests: `handlers_test.go` `TestHashToken` only.

### Gap / problem

Auth logic is split across `handlers_auth.go`, `handler.go`, `db.go`, and `handlers_streaming.go`. Two token systems share `jwtSecret` / `hashToken` naming proximity but behave differently. There is no `internal/auth` module while `internal/playback` and `internal/analytics` already exist. Adopting a heavy external auth stack (Authelia, OIDC) would mismatch OwnWave's self-hosted, single-user deployment model.

### Options table

| Option | Fits session (Bearer DB) | Fits stream (JWT URL) | Fit for OwnWave |
|--------|--------------------------|----------------------|-----------------|
| **Custom `internal/auth`** | Yes: extract token + session service | Stream JWT can be `auth/streamtoken` subpackage or stay in handlers | Best match; minimal deps; full control |
| **go-chi/jwtauth** | No: designed for JWT in `Authorization` header via middleware | Partial: could sign stream JWTs, but Verifier expects header/cookie not query param | Adds middleware model OwnWave does not use for sessions |
| **golang-jwt/jwt only (status quo)** | N/A | Already used | Fine for stream tokens; keep |
| **Authelia / external SSO** | Replaces login flow with proxy auth | Stream URLs still need app-level tokens | High ops burden for single-user; wrong layer for `<audio>` URLs |
| **OAuth2 / OIDC provider** | Possible | Still need stream capability tokens | Overkill unless multi-user cloud |

### Recommended approach + migration steps

**Extract custom `internal/auth`. Do not adopt Authelia or jwtauth for session management.**

Rationale: OwnWave session auth is ~100 lines of bcrypt + opaque tokens + Postgres. jwtauth solves "verify JWT on every request via middleware," which is not the current design. Authelia solves organizational SSO at the reverse proxy. Neither replaces DB-backed sessions or query-param stream JWTs without a large product shift.

**Proposed module API:**

```go
package auth

type Service struct { /* pool or SessionStore interface */ }

func NewService(pool *pgxpool.Pool) *Service

func (s *Service) Register(ctx, username, password) (token string, user User, err error)
func (s *Service) Login(ctx, username, password) (token string, user User, err error)
func (s *Service) Logout(ctx, rawToken string) error
func (s *Service) UserFromRequest(r *http.Request) (User, bool)

func GenerateToken() (string, error)   // moved from generateSessionToken
func HashToken(raw string) string      // moved from hashToken
```

**Stream JWT** (keep separate, smaller surface):

```go
package streamauth  // or auth/streamtoken

type StreamTokens struct { secret []byte }

func (t *StreamTokens) SignTrack(trackID, format string) (string, error)
func (t *StreamTokens) ValidateTrack(tokenString string) (trackID, format string, err error)
func (t *StreamTokens) SignStation(stationID, format string) (string, error)
func (t *StreamTokens) ValidateStation(tokenString string) (stationID, format string, err error)
```

Move `getString` into `streamauth` (only JWT claim parsing). `Handler` holds `*auth.Service` and `*streamauth.StreamTokens` instead of raw `jwtSecret` + scattered methods.

**Migration steps:**

1. Create `internal/auth/service.go`; move session DB methods from `db.go` (`CreateSession`, `GetUserByTokenHash`, `DeleteSession`, user lookups used by auth) into `auth` package or an `auth/store.go` adapter on `pgxpool`.
2. Move `generateSessionToken`, `hashToken` to `internal/auth/tokens.go`.
3. Move `authUser`, `createSession` logic into `auth.Service.UserFromRequest` / `CreateSession`.
4. Slim `handlers_auth.go` to HTTP decode/encode calling `h.auth.*`.
5. Create `internal/auth/streamtoken.go`; move sign/validate from `handlers_streaming.go`.
6. Move `TestHashToken` to `internal/auth/tokens_test.go`.
7. Leave router unchanged initially. Optional follow-up: chi middleware wrapping only `/me` routes.

Do not add Authelia unless the product moves to multi-user hosted deployments with a reverse proxy owning login.

### Tests that survive

| Test | Fate |
|------|------|
| `TestHashToken` | Moves to `internal/auth/tokens_test.go` |
| `TestGetString` | Moves to `internal/auth/streamtoken_test.go` with stream token helpers |
| Session integration tests (future) | `internal/auth/service_integration_test.go` with testcontainers; cases: login returns valid token, expired session rejected, logout deletes row |

### Sources

- OwnWave `handlers_auth.go`, `handler.go`, `db.go`, `handlers_streaming.go`, `main.go`: repository files
- OwnWave `database/migrations/005_add_users.up.sql`: repository file
- go-chi jwtauth README: https://github.com/go-chi/jwtauth/blob/master/README.md
- golang-jwt/jwt v5: https://pkg.go.dev/github.com/golang-jwt/jwt/v5
- Authelia introduction: https://www.authelia.com/overview/prologue/introduction/
- golang.org/x/crypto/bcrypt (used in handlers): https://pkg.go.dev/golang.org/x/crypto/bcrypt
