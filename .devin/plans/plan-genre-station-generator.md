# Plan: Genre/Sub-Genre Auto Station Generator

**One-sentence summary:** Add a CPU-conscious, optional genre analysis module using Essentia's pre-trained Discogs400 model, persist main/sub-genre labels per track, and automatically build/update genre-based stations after every library scan.

---

## 1. Goal

After a scan finishes, OwnWave will own and maintain a set of **auto-generated stations**:

- One station per detected **main genre** (e.g. `Rock`, `Electronic`, `Jazz`).
- One station per detected **sub-genre** that has enough tracks (e.g. `Rock / Post-Punk`, `Electronic / Deep House`).
- New, changed, or deleted tracks are reflected in these stations on the next scan.
- The system runs on an **i5-12500T** mini PC without choking it: genre analysis is done on a 30–60 s audio snippet, in background Celery tasks, and tracks are reanalyzed only when the file changes.

## 2. Non-goals (for this pass)

- No custom training of a genre model.
- No real-time genre prediction during playback.
- No manual taxonomy editor (use the existing Discogs400 hierarchy).
- No new cover-image or color generation tied to genre (the existing `.devin/notes/station-cover-image-ideas.md` can consume it later).

## 3. Key design decisions

1. **Use Essentia `genre_discogs400`** with the `discogs-effnet-bs64-1` embedding model and `genre_discogs400-discogs-effnet-1` classification head. This gives a ready-made hierarchical `Main---Sub` taxonomy (e.g. `Rock---Post-Punk`).
2. **Optional and fallback-first.** If `essentia-tensorflow` is not installed, the scanner falls back to the `genre` tag already read by `services/python-analytics/src/tags.py` and persists it.
3. **Snippet-first, full-track optional.** Analyze a 30–60 s clip per track (default 0:30–1:00). This keeps per-track CPU time to ~1–3 s on an i5-12500T. A full-track mode can be enabled via config for users who want higher accuracy and are willing to wait.
4. **Background only.** Genre analysis and station rebuild run inside Celery tasks, not in the HTTP request path. The file watcher (`services/python-analytics/src/watcher.py`) continues to queue `trigger_library_scan`, and `finalize_scan` triggers the station rebuild.
5. **Store top-k predictions** with confidence. The top `Main---Sub` pair is used for station grouping, but lower-confidence predictions are kept for future tuning.
6. **Do not touch the 33-dim feature vector.** Genre labels are stored separately; the feature vector remains the same so existing similarity/clustering keeps working.
7. **Auto-stations are marked `is_auto`.** They do not collide with user stations and are refreshed/re-created automatically.
8. **Confidence threshold and minimum track count.** A track is assigned to a genre station only if the top prediction is ≥ `GENRE_MIN_CONFIDENCE` (default 0.3). A sub-genre station is created only if it has ≥ `GENRE_MIN_TRACKS_PER_STATION` tracks (default 5).

## 4. Target performance on i5-12500T

- 30–45 s snippet per track: ~1–3 s CPU time.
- 500-track library (snippets): ~10–25 min background job.
- Full-track analysis (optional): ~1–3 h for 500 tracks.
- Peak memory per track: < 500 MB with `batchSize=64` and chunked patches; avoid `batchSize=-1` on long files to prevent multi-GB spikes.
- Set `TF_ENABLE_ONEDNN_OPTS=1` in the container so Essentia/TensorFlow can use AVX2/oneDNN.

## 5. Implementation steps

### 5.1 Dependencies and build

- `services/python-analytics/requirements.txt`: add `essentia-tensorflow` (Linux x86_64 wheel, ~291 MB). Keep a graceful import fallback so the rest of the stack still starts if the wheel is not present.
- `services/python-analytics/Dockerfile`:
  - Install the new requirement.
  - Download `discogs-effnet-bs64-1.pb` (~18 MB) and `genre_discogs400-discogs-effnet-1.pb` (~2 MB) into `/app/models`.
  - Add `ENV TF_ENABLE_ONEDNN_OPTS=1` and `TF_CPP_MIN_LOG_LEVEL=1`.
- Add `services/python-analytics/src/genre_analyzer.py`:
  - Load models lazily.
  - `analyze(path, snippet_start=30.0, snippet_duration=30.0)`:
    - Load audio at 16000 Hz with `MonoLoader`.
    - Slice the snippet; fall back to the full track if it is shorter.
    - Run `TensorflowPredictEffnetDiscogs` with `patchHopSize=128` (less overlap for speed) and `batchSize=64`.
    - Run `TensorflowPredict2D` and average patch activations.
    - Parse labels like `"Rock---Post-Punk"` into `main_genre` and `sub_genre`.
    - Return a list of `GenrePrediction` objects with confidence.
  - If the model is unavailable, fall back to parsing the `genre` tag from `read_tags()` and split on `;`, `/`, `|`.

### 5.2 Database schema

New migration `database/migrations/009_track_genres.up.sql`:

```sql
CREATE TABLE track_genres (
    track_id UUID REFERENCES tracks(id) ON DELETE CASCADE,
    main_genre TEXT NOT NULL,
    sub_genre TEXT NOT NULL,
    confidence FLOAT NOT NULL,
    source TEXT NOT NULL, -- 'discogs400', 'file_tag'
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (track_id, sub_genre, source)
);

CREATE INDEX idx_track_genres_track ON track_genres(track_id);
CREATE INDEX idx_track_genres_main ON track_genres(main_genre, sub_genre);
CREATE INDEX idx_track_genres_confidence ON track_genres(confidence);

ALTER TABLE stations
    ADD COLUMN IF NOT EXISTS is_auto BOOLEAN DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS source TEXT,
    ADD COLUMN IF NOT EXISTS last_refreshed_at TIMESTAMPTZ;
```

Add the corresponding `.down.sql` migration.

### 5.3 Data access

In `services/python-analytics/src/db.py`:

- `upsert_track_genres(conn, track_id, predictions)`
- `get_track_genres(conn, track_id)`
- `list_genres(conn, min_confidence=0.3)` → distinct `(main_genre, sub_genre, count)`
- `get_tracks_by_genre(conn, main_genre, sub_genre, limit=200)`
- `upsert_auto_station(conn, name, seed_features, source)` → returns `station_id` with `is_auto=true`
- `delete_orphaned_auto_stations(conn)` → remove auto stations whose genre no longer has tracks

### 5.4 Analyzer integration

- `services/python-analytics/src/scanner.py` and `services/python-analytics/src/folder_importer.py`:
  - After `analyze_file()` and `build_feature_vector()`, call `genre_analyzer.analyze()` if the file has changed or has no genre predictions.
  - Pass the predictions to `upsert_track_genres()`.
- `services/python-analytics/src/models.py`:
  - Add a `GenrePrediction` dataclass.

### 5.5 Station builder

In `services/python-analytics/src/station_builder.py`:

- Add `"genre"` and `"sub_genre"` to `SEED_TYPES`.
- In `_apply_filter()`:
  - `seed_type == "genre"`: select tracks whose top `main_genre` matches.
  - `seed_type == "sub_genre"`: select tracks whose `main_genre` and `sub_genre` both match.
- Add `rebuild_genre_stations(conn, min_confidence=0.3, min_tracks=5, station_length=50)`:
  - Query `list_genres()`.
  - For each main genre, `build_station()` with a `genre` filter.
  - For each sub-genre with enough tracks, `build_station()` with a `sub_genre` filter.
  - Mark created stations `is_auto = true`, `source = 'genre'`.
  - Delete auto stations whose genre now has fewer than `min_tracks` tracks or no longer exists.

### 5.6 Background tasks

In `services/python-analytics/src/tasks.py`:

- Add `@celery_app.task` `rebuild_track_genres(path, force=False)` for batch genre backfill.
- Add `@celery_app.task` `rebuild_genre_stations()`.
- In `finalize_scan()`:
  - After pruning deleted tracks, call `rebuild_genre_stations.delay()`.

### 5.7 Python API

In `services/python-analytics/src/api.py`:

- Extend `StationRequest` with `genre` and `sub_genre`.
- `GET /genres` → list main/sub counts.
- `GET /tracks/{track_id}/genres` → top predictions.
- `POST /rebuild-genres` → queue full backfill.
- `POST /rebuild-genre-stations` → queue station rebuild.
- `POST /stations` already exists; `build_station()` now handles genre filters.

### 5.8 Go API

- `services/go-api-server/handlers.go`:
  - Add `GET /genres`, `GET /tracks/{id}/genres`, `POST /admin/rebuild-genres`, `POST /admin/rebuild-genre-stations`.
  - Either proxy to the Python service or query Postgres directly (direct query is fine for `/genres` and track genres).
  - Extend the `CreateStation` / `UpdateStation` request structs to accept `genre` and `sub_genre`.
- `services/go-api-server/main.go`: register the new routes.

### 5.9 Frontend

- `apps/t3-web-frontend/lib/api.ts`: add `getGenres`, `getTrackGenres`, `adminRebuildGenres`, `adminRebuildGenreStations`.
- `apps/t3-web-frontend/server/routers/app.ts`: add tRPC procedures for the new endpoints.
- `apps/t3-web-frontend/components/StationManager.tsx`: add `genre` / `sub_genre` seed options and dropdowns.
- New or existing page (`/genres` or the home station grid) to browse auto-generated stations grouped by main genre.
- `apps/t3-web-frontend/app/admin/page.tsx`: add "Rebuild Genres" and "Rebuild Genre Stations" buttons with confirmation.

### 5.10 Configuration

Add to `services/python-analytics/src/config.py` and `.env`:

```env
ENABLE_GENRE_ANALYSIS=true
GENRE_MODEL_DIR=/app/models
GENRE_SNIPPET_START=30
GENRE_SNIPPET_DURATION=30
GENRE_MIN_CONFIDENCE=0.3
GENRE_MIN_TRACKS_PER_STATION=5
GENRE_FULL_TRACK_MODE=false
```

## 6. File changes

- `services/python-analytics/requirements.txt`
- `services/python-analytics/Dockerfile`
- `services/python-analytics/src/config.py`
- `services/python-analytics/src/models.py`
- `services/python-analytics/src/genre_analyzer.py` (new)
- `services/python-analytics/src/db.py`
- `services/python-analytics/src/scanner.py`
- `services/python-analytics/src/folder_importer.py`
- `services/python-analytics/src/station_builder.py`
- `services/python-analytics/src/tasks.py`
- `services/python-analytics/src/api.py`
- `database/migrations/009_track_genres.up.sql` (new)
- `database/migrations/009_track_genres.down.sql` (new)
- `services/go-api-server/handlers.go`
- `services/go-api-server/main.go`
- `apps/t3-web-frontend/lib/api.ts`
- `apps/t3-web-frontend/server/routers/app.ts`
- `apps/t3-web-frontend/components/StationManager.tsx`
- `apps/t3-web-frontend/app/admin/page.tsx`
- `apps/t3-web-frontend/app/page.tsx` or new `app/genres/page.tsx`

## 7. Verification plan

1. Build with `just build` and run `just db-migrate`.
2. Place a small test library with clear genre diversity and run `just scan`.
3. Verify `track_genres` is populated and `GET /genres` returns expected main/sub counts.
4. Verify auto stations appear in the UI after the scan.
5. Add a new track and confirm the watcher/Celery flow adds it to the correct station.
6. Delete a track and confirm it is removed from the station.
7. On the i5-12500T target, measure per-track snippet time and a 500-track scan time.
8. Test fallback: remove `essentia-tensorflow` and confirm the scanner still works using file tags.

## 8. Open questions

1. Should genre analysis be **enabled by default** or require `ENABLE_GENRE_ANALYSIS=true`? (Recommended: off until the first scan is tested.)
2. Which default snippet window? 0:30–1:00 is a safe starting point.
3. Should the top `sub_genre` prediction be used even when confidence is low, or should low-confidence tracks go into an `Uncategorized` station? (Recommended: threshold 0.3.)
4. Should we also store the **1280-dim genre embedding** for future similarity improvements? (Recommended: no in v1; only store labels.)
5. What license model is acceptable? Essentia/MTG models are released under **CC BY-NC-SA 4.0** by default, with a proprietary license available on request.
