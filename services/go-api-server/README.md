# Go API Server

## Integration tests

Playback integration tests use testcontainers and require Docker:

```bash
go test -tags=integration ./internal/playback/...
```

Unit tests (default):

```bash
go test ./...
```
