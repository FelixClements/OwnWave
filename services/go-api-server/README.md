# Go API Server

## Integration tests

Integration tests use testcontainers and require Docker:

```bash
go test -tags=integration ./internal/stations/... ./internal/listening/...
```

Unit tests (default):

```bash
go test ./...
```
