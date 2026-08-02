# OwnWave command runner
# Requires: `just` (https://just.systems) and `docker`

# Show available recipes by default
default:
    @just --list

# Build all Docker images
build:
    docker compose build

# Start the development stack (add `-d` to detach, e.g. `just dev -d`)
dev *args:
    docker compose up --build {{args}}

# Scan the music library (defaults to /music inside the container)
scan path="/music" *args:
    docker compose run --rm python python -m main scan {{path}} {{args}}

# Smoke test: build, start core services, and health-check the Go API
test:
    #!/usr/bin/env bash
    set -euo pipefail
    docker compose build python go
    docker compose up -d db redis go python
    trap 'docker compose down' EXIT
    for i in $(seq 1 60); do
        if curl -fsS http://localhost:8080/health >/dev/null 2>&1; then
            echo "Health check passed"
            exit 0
        fi
        sleep 1
    done
    echo "Health check failed" >&2
    exit 1

# Apply database migrations with golang-migrate
db-migrate:
    docker compose run --rm migrate up
