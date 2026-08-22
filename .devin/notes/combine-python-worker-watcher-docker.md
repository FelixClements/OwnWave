# Combining `python`, `worker`, and `watcher` Docker Services

**Date:** 2026-08-22

## Question

Can OwnWave's three Docker Compose services — `python` (API), `worker` (Celery), and `watcher` (filesystem monitor) — be merged into a single container?

## Summary

Yes, it is technically possible to run uvicorn, a Celery gevent worker, and the watchdog-based file watcher in one container using a process supervisor, wrapper script, or a new CLI entrypoint. Docker and Celery do not prohibit this pattern. For OwnWave, the current three-service split is intentional: ADR 6 cites filesystem watcher / inotify edge cases with Docker bind mounts, and the worker's gevent pool (`-c 50`) plus optional TensorFlow/Essentia genre models create memory and CPU contention risks when colocated with the API. **Recommendation: keep separate containers for production; consider an optional dev-only combined service or Compose profile if local startup simplicity matters.**

## Current architecture

All three services build from the same image (`./services/python-analytics`), share env vars, mount `${MUSIC_PATH}:/music:ro`, and depend on `db`, `redis`, and `go`.

| Service | Container name | Command | Role |
|---------|----------------|---------|------|
| `python` | `python-1` | Dockerfile default: `uvicorn api:app --host 0.0.0.0 --port 8000` | FastAPI HTTP API on port 8000 |
| `worker` | `worker-1` | `celery -A celery_app worker -P gevent -c 50 --loglevel=info -n worker@%h` | Celery task execution (scans, genre rebuild, clustering) |
| `watcher` | `watcher-1` | `python -m main watch` | Filesystem watcher → queues `tasks.trigger_library_scan` via Redis |

Go calls the API at `PYTHON_API_URL: http://python:8000` (`docker-compose.yml` lines 120, 31–57).

**Dockerfile** (`services/python-analytics/Dockerfile`):

- Python 3.11-slim, ffmpeg, Essentia TensorFlow models baked into `/app/models`
- Default `CMD`: uvicorn on port 8000 (lines 37–37)

**Process responsibilities in code:**

- **API** (`api.py`): FastAPI app; `startup()` waits for DB (lines 21–24). Scan endpoint queues Celery tasks when Redis is available, otherwise runs scan in `BackgroundTasks` in-process (lines 81–98).
- **Worker** (`celery_app.py`): Celery app with `worker_process_init` hook calling `db.wait_for_db()` (lines 26–30). Gevent pool with 50 concurrent greenlets (compose command).
- **Watcher** (`watcher.py`): `watchdog.observers.Observer` on `MUSIC_DIR`; debounced events send `tasks.trigger_library_scan` via `celery_app.send_task` (lines 61–66, 71–87). Does **not** load ML models.
- **CLI** (`main.py`): `watch` and `serve` subcommands already exist (lines 68–71, 89–93) but are run as separate containers today.

**ADR 6** (`ARCHITECTURE.md` lines 70–74): indexing via API + CLI; watcher optional and "off by default in v1" to avoid Docker volume / inotify edge cases. Compose currently runs `watcher` as a permanent third service anyway.

## Technical feasibility

### Docker: multi-process containers are supported but not the default recommendation

Docker's official guidance states that a container's main process is `ENTRYPOINT`/`CMD`, and **best practice is one service per container**. Multiple processes are acceptable when a single service forks (e.g. Apache workers), but Docker recommends connecting containers via networks and shared volumes rather than bundling unrelated application roles.

When multiple processes are needed, Docker documents three approaches: wrapper shell script, Bash job control, or a process manager (e.g. supervisord). It also recommends `--init` or Compose `init: true` so PID 1 can forward signals and reap zombies — especially important for shell-based wrappers.

**Sources:**

- [Run multiple processes in a container](https://docs.docker.com/engine/containers/multi-service_container/)
- [Compose `init` option](https://docs.docker.com/reference/compose-file/services/#init)

### Celery worker alongside other processes

Celery workers are designed as standalone processes. There is no documented prohibition on running a worker next to other processes in the same container. OwnWave's worker uses the **gevent pool** (`-P gevent -c 50`), which runs up to 50 concurrent greenlets in **one OS process** — cooperative I/O concurrency, not 50 separate processes.

Celery Beat (`celery beat` or `worker -B`) is a separate scheduler role; OwnWave does not use Beat or periodic tasks today, so Beat-combined approaches are irrelevant.

**Sources:**

- [Celery gevent concurrency](https://docs.celeryq.dev/en/stable/userguide/concurrency/gevent.html)
- [Celery periodic tasks / Beat](https://docs.celeryq.dev/en/stable/userguide/periodic-tasks.html)

### Uvicorn deployment

Uvicorn is intended as an ASGI server, typically one process per worker. Production docs recommend Gunicorn with `uvicorn.workers.UvicornWorker` and multiple worker processes (`-w 4`) for throughput. OwnWave runs a **single uvicorn process** in Compose — no Gunicorn multi-worker setup.

Colocating uvicorn with Celery means two (or more) Python interpreter processes in one container unless uvicorn is embedded in a thread inside a custom entrypoint (not recommended for production ASGI).

**Sources:**

- [Uvicorn deployment](https://www.uvicorn.dev/deployment/)

### Watchdog / inotify in Docker

`watcher.py` uses the default `watchdog.observers.Observer`, which on Linux resolves to `InotifyObserver` (inotify). Inotify events propagate correctly when files live on the **host's native Linux filesystem** and are bind-mounted into the container. They are unreliable when:

- The mount is a remote share (NFS/CIFS)
- Docker Desktop on Windows/macOS maps a non-Linux filesystem into the container
- Some virtualization / shared-folder setups (Vagrant, older Docker Desktop versions)

Fallback: `watchdog.observers.polling.PollingObserver` — works everywhere but polls periodically (higher CPU, slower detection).

ADR 6 explicitly flags watcher / inotify as a reason for cautious rollout. Combining containers does not fix inotify; it only reduces image pull / service count.

**Sources:**

- [watchdog API — Observer platforms](https://python-watchdog.readthedocs.io/en/stable/api.html)
- [watchdog issue #283 — Docker/shared folders](https://github.com/gorakhargosh/watchdog/issues/283)
- [Docker Desktop WSL2 — inotify on Linux filesystem paths](https://docs.docker.com/docker-for-windows/wsl/#best-practices)

## Approaches compared

| Approach | Mechanism | Pros | Cons | OwnWave fit |
|----------|-----------|------|------|-------------|
| **Keep 3 Compose services** (current) | One container per role | Independent scaling, restart, and memory limits; clear logs; matches ADR 6; worker OOM does not kill API | 3× base Python memory; duplicated env/volume blocks in compose | **Best for prod** |
| **supervisord / s6-overlay** | PID 1 supervisor spawns uvicorn, celery, watcher | Docker-documented pattern; per-process restart; structured logging config | Extra image complexity; health checks need custom script; supervisor becomes PID 1 | Reasonable if combining is required |
| **Shell wrapper + `wait`** | `uvicorn & celery ... & python -m main watch & wait -n` | Minimal deps; quick to prototype | Poor signal handling without `init: true`; first exit kills container; no per-process restart | Dev-only |
| **Custom `main all` CLI** | Thread/subprocess for watcher + uvicorn; subprocess for celery | No supervisord; uses existing `main.py` seams | Still multi-process; celery must stay subprocess; custom maintenance | Possible dev convenience |
| **Honcho / foreman** | Procfile runner | Familiar to Rails/Heroku users | Another dependency; same multi-process downsides | Low value vs supervisord |
| **Embed watcher in API process** | Run `Observer` in FastAPI `startup` background thread | Single Python process for API + watcher | Celery worker still separate or embedded differently; mixes HTTP and FS monitoring lifecycles | Watcher-only partial merge; limited benefit |
| **Celery `worker -B`** | Beat inside worker | — | OwnWave has no periodic tasks | **Not applicable** |
| **Compose init / sidecar** | Separate init container | — | Does not combine processes; only ordering | Not a merge strategy |

## OwnWave-specific considerations

### Memory: ML models (not truly ×3 today)

Genre models (Essentia TensorFlow graphs) are loaded **lazily** when genre analysis runs:

- `genre_sources.py` caches sources per process (`_cached_sources`, lines 8–22).
- `Discogs400GenreSource.from_config()` loads embed + classification graphs (`genre_discogs400.py` lines 48–59).
- `genre_analyzer.py` uses module-level singletons (`_EMBED_MODEL`, etc., lines 31–63).

**Per current container layout:**

| Container | Loads ML models? |
|-----------|------------------|
| `watcher` | No — only imports `celery_app` and sends tasks |
| `python` (API) | Only if API code path calls `get_genre_sources()` / genre endpoints or in-process scan with genre analysis |
| `worker` | Yes — during `import_folder` / `rebuild_track_genres` when `ENABLE_GENRE_ANALYSIS=true` |

Combining into one container reduces **interpreter overhead** (3 Python processes → ~2 if watcher threads into API, or 3 if all separate OS processes) but does **not** eliminate duplicate model memory if uvicorn and celery worker are separate processes both analyzing audio. Essentia/TensorFlow graphs can be hundreds of MB per process.

Default compose sets `ENABLE_GENRE_ANALYSIS: false`, so model memory is often irrelevant until enabled.

### CPU and gevent worker contention

Worker command: `celery ... -P gevent -c 50` allows 50 concurrent I/O-bound tasks in one process. Audio analysis (librosa, Essentia) is **CPU-heavy**. Under load, a combined container can starve uvicorn's event loop process and increase API latency for Go (`PYTHON_API_URL` calls for scans, stations, setup).

### Resource isolation and OOM

Docker applies memory limits per container. A runaway scan in the worker can trigger cgroup OOM and kill **all** colocated processes (API + watcher) in a merged container. With separate services, only `worker-1` dies; Go may still reach `/health` on `python:8000` (though scan jobs would stall).

### Health checks and restart behavior

`python` has **no** `healthcheck` in `docker-compose.yml` (unlike `go` and `db`). `watcher` and `worker` have none either. A merged service would need a composite health check (e.g. HTTP `/health` + celery inspect ping) to be production-grade. Supervisord can restart a crashed child without restarting siblings; a plain shell wrapper cannot.

### Scaling

Separate `worker` replicas (`docker compose up --scale worker=3`) are straightforward. A merged container cannot scale workers independently of API or watcher.

### File watcher / bind mounts (ADR 6)

`ARCHITECTURE.md` ADR 6: watcher deferred to avoid Docker volume / inotify edge cases. `watcher.py` uses inotify by default. On a Linux server with `MUSIC_PATH` on local disk, this usually works. On Docker Desktop dev machines with host paths outside WSL2 Linux filesystem, events may not fire — **independent of container merge**. Fix is `PollingObserver`, not service consolidation.

### Code already supports partial unification

`main.py` exposes `serve` and `watch` as subcommands; Dockerfile CMD is uvicorn directly, not `python -m main serve`. A combined entrypoint could orchestrate existing commands without architectural code changes.

### Production compose

`docker-compose.prod.yml` only strips published ports for `python`; it does not change the three-service layout. Caddy routes to `web` and `go`, not directly to Python.

## Recommendation

| Environment | Recommendation |
|-------------|----------------|
| **Production (single-server Compose)** | **Keep three services.** Aligns with ADR 6, Docker one-service-per-container guidance, independent worker scaling, OOM isolation, and avoids API latency under scan load. |
| **Local dev** | **Optional:** add a Compose **profile** (e.g. `combined-python`) with one service running supervisord or a thin wrapper, **or** document `docker compose up python worker watcher` as the norm. Only worth it if devs consistently complain about service count. |
| **If merging anyway** | Use **supervisord** (or s6-overlay) + Compose `init: true`; run uvicorn, celery worker, and `python -m main watch` as separate supervised programs; add composite healthcheck; do **not** use a naive `&` shell script without init. |

**Do not merge** solely to save memory from ML models — watcher already does not load models; the main savings are duplicate Python interpreter baseline (~tens of MB each) and one fewer image instance, which is minor compared to audio analysis RAM/CPU.

**Future improvement** (orthogonal to container merge): if genre analysis is enabled, consider a shared model-loading strategy or dedicated `worker-genre` service with higher memory limit, keeping API lightweight.

## Sources

### OwnWave repository

- `docker-compose.yml` — service definitions (lines 31–111)
- `docker-compose.prod.yml` — production port overrides
- `services/python-analytics/Dockerfile`
- `services/python-analytics/src/api.py`
- `services/python-analytics/src/celery_app.py`
- `services/python-analytics/src/watcher.py`
- `services/python-analytics/src/main.py`
- `services/python-analytics/src/tasks.py`
- `services/python-analytics/src/genre_discogs400.py`
- `services/python-analytics/src/genre_sources.py`
- `ARCHITECTURE.md` — ADR 6 (lines 70–74)
- `docs/DEPLOY.md`

### Official / primary external documentation

- Docker — [Run multiple processes in a container](https://docs.docker.com/engine/containers/multi-service_container/)
- Docker — [Compose `init` option](https://docs.docker.com/reference/compose-file/services/#init)
- Docker — [Docker Desktop WSL2 best practices (inotify)](https://docs.docker.com/docker-for-windows/wsl/#best-practices)
- Celery — [Gevent concurrency](https://docs.celeryq.dev/en/stable/userguide/concurrency/gevent.html)
- Celery — [Periodic tasks / Beat](https://docs.celeryq.dev/en/stable/userguide/periodic-tasks.html)
- Uvicorn — [Deployment](https://www.uvicorn.dev/deployment/)
- watchdog — [API reference (Observer / PollingObserver)](https://python-watchdog.readthedocs.io/en/stable/api.html)
- watchdog — [GitHub issue #283 — Docker/shared folders](https://github.com/gorakhargosh/watchdog/issues/283)
