# Go API deepening implementation plan

Repository: `services/go-api-server`. Research: `.devin/notes/go-api-playback-streaming-auth-research.md`.

## Global constraints

- Module path: `ownwave/api` (see `go.mod`).
- Do not merge session Bearer auth with stream JWT query tokens.
- Do not adopt Authelia, jwtauth for sessions, or OAuth servers.
- Integration tests use `//go:build integration` and must not run under default `go test ./...`.
- Use testcontainers-go with image `pgvector/pgvector:pg15` for playback integration tests.
- Each task: `go test ./...` must pass before commit; integration task also runs `go test -tags=integration ./internal/playback/...`.
- Follow existing code style; run `gofmt` on changed Go files.
- Commit each task separately with a clear message.
- Do not change HTTP API contracts (paths, status codes, JSON shapes).

---

## Task 1: Playback integration tests

Add pgx-backed integration tests for `internal/playback` using testcontainers-go.

### Dependencies

```bash
cd services/go-api-server
go get github.com/testcontainers/testcontainers-go/modules/postgres
go get github.com/stretchr/testify
```

### Files to create

1. `internal/playback/testutil_integration_test.go` with `//go:build integration`
   - `setupTestDB(t *testing.T) *pgxpool.Pool`
   - Start container: `postgres.Run(ctx, "pgvector/pgvector:pg15", postgres.WithSQLDriver("pgx"))`
   - Apply migrations: set `MIGRATIONS_PATH` to repo `database/migrations` (relative path from test: `../../../database/migrations`) and call migration logic. Either export a test helper from main package or duplicate minimal migrate-up using same golang-migrate pattern as `migrations.go`.
   - `t.Cleanup` terminates container.

2. `internal/playback/service_integration_test.go` with `//go:build integration`, package `playback_test`
   - Seed helpers for artist, album, track, audio_features, station, station_tracks (all columns required by `queryStationQueue` scan in `service.go`).

### Exact test cases (all required)

| Test name | Assert |
|-----------|--------|
| `TestRecord_WritesListeningHistory` | After `Record(ctx, trackID, "")`: row in `listening_history`, `station_id IS NULL` |
| `TestRecord_UpdatesStationTracksPlayedAt` | After `Record` with station: `listening_history.station_id` set; `station_tracks.played_at` not null |
| `TestRecord_TransactionAtomic` | Invalid station update path leaves no `listening_history` row (use non-matching station_id/track pair or valid approach) |
| `TestBuildQueue_ExcludesRecentlyPlayed` | Track played within window excluded when `recentHours=24`; unplayed track present |
| `TestBuildQueue_FallbackWhenAllPlayed` | All tracks recently played → `BuildQueue` still non-empty |
| `TestBuildQueue_ExcludesBanned` | Track with `track_feedback.feedback='ban'` absent |
| `TestBuildQueue_DeduplicatesTitleArtist` | Two station_tracks same title+artist → one queue entry |

### CI

Update `.github/workflows/ci.yml`:

```yaml
- name: Test Go (unit)
  run: cd services/go-api-server && go test ./...

- name: Test Go (integration)
  run: cd services/go-api-server && go test -tags=integration ./internal/playback/...
```

### Documentation

Add short note to `services/go-api-server/README.md` (create if missing): local command `go test -tags=integration ./internal/playback/...` requires Docker.

---

## Task 2: Extract `internal/streaming`

Move ffmpeg audio serving from `main` to `internal/streaming`. JWT stream tokens stay in `handlers_streaming.go`.

### Create `internal/streaming`

- `gain.go`: `NormalizeBitrate`, `VolumeGainDb` (move from `streaming.go`)
- `server.go`: `Config`, `Server`, `New`, `ResolvePath`, `ServeFLAC`, `ServeTranscoded`, `ServeCrossfaded`
- `gain_test.go`: move tests from `streaming_test.go`

### Update `main`

- Add `stream *streaming.Server` to `Handler`; initialize in `NewHandler` from `musicDir`, `ffmpegPath`.
- `handlers_streaming.go`: replace `h.serveFLAC`, `h.serveTranscoded`, `h.serveCrossfaded` with `h.stream.*`
- Delete `streaming.go` and `streaming_test.go` from `main`.
- Remove `TrackWithFeatures` alias from `db.go` if unused; import `playback` where needed.

### Tests

- `go test ./...` passes including `internal/streaming`.

---

## Task 3: Extract `internal/auth` and `internal/streamauth`

### `internal/auth`

- `tokens.go`: `GenerateToken`, `HashToken` (from `handler.go`)
- `service.go`: `Service` with `Register`, `Login`, `Logout`, `UserFromRequest`; session DB ops moved from `db.go` or wrapped
- `tokens_test.go`: move `TestHashToken` from `handlers_test.go`

### `internal/streamauth`

- `streamtoken.go`: `StreamTokens` with `SignTrack`, `ValidateTrack`, `SignStation`, `ValidateStation`
- Move `getString` helper here (only used for JWT claims)

### Update `main`

- `Handler` holds `auth *auth.Service` and `streamTokens *streamauth.StreamTokens` instead of scattered methods
- Slim `handlers_auth.go` to HTTP only
- `handlers_streaming.go` uses `h.streamTokens` for sign/validate
- `handlers_test.go`: keep `TestGetString` moved to streamauth tests or delete if covered

### Tests

- `go test ./...` passes.
