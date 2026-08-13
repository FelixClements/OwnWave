# Plan: Random station start and shuffle

## User request
Make OwnWave stations start on a random song and not play the same songs in the same order every time. No code changes yet — this is the plan only.

## Current behaviour (research findings)

### How the queue is built
- `services/python-analytics/src/station_builder.py` `_smart_queue` builds a deterministic greedy k-NN walk.
- It starts from a seed track if one is supplied; otherwise it picks a random track only at queue creation time.
- Once built, the queue is written to `station_tracks` and does not change between listens.

### How the queue is stored
- `database/migrations/001_schema.up.sql` defines `station_tracks` with `position` and `played_at`.
- `played_at` is currently set but not used for rotation or shuffle.

### How the queue is played
- `services/go-api-server/db.go` `GetStationQueue` returns tracks by feedback priority (`like`, neutral, `skip`) then by `position`.
- `apps/t3-web-frontend/components/Player.tsx` plays the queue sequentially from the start, so the same song starts every time.

## Recommended approach: Option 1 — persistent station mode

Add a `mode` column to the `stations` table with three values:

- `smart` — current k-NN walk (default, unchanged)
- `shuffle` — random order of the same filtered track pool
- `random` — completely random selection each time, respecting filters

This is the recommended option because the preference belongs to the station, not to each playback request.

### Backend changes

1. **Database migration**
   - Add `mode TEXT NOT NULL DEFAULT 'smart'` to the `stations` table.

2. **Python analytics queue building**
   - Update `build_station` and `_smart_queue` in `services/python-analytics/src/station_builder.py` to respect the station `mode`.
   - For `shuffle`: generate the filtered track pool, then shuffle it and store it.
   - For `random`: pick tracks randomly from the filtered pool (with or without replacement depending on desired behaviour).

3. **Go API queue serving**
   - Update `GetStationQueue` in `services/go-api-server/db.go` to optionally return a shuffled order.
   - Keep the existing feedback/ban logic.
   - Use `played_at` to optionally de-prioritize recently played tracks.

4. **Queue endpoint**
   - Allow `GET /stations/{id}/queue?mode=shuffle` to override the stored mode for one request.

### Frontend changes

1. **Station creation / editing**
   - Add a `Mode` selector in `StationManager.tsx` when creating or editing a station.
   - Persist `mode` in `CreateStationRequest` / `UpdateStationRequest`.

2. **Player**
   - Add a shuffle toggle that calls the queue endpoint with `mode=shuffle` or `mode=random`.
   - Keep the toggle state in the player store if it is per-session.

### Avoiding the same songs in a row

- `played_at` can be used to skip or de-prioritize tracks played in the last N hours.
- For `shuffle` mode, the queue should be re-shuffled when it ends or on demand, not repeated from the same starting point.
- `banned` tracks must remain excluded in all modes.

## Alternative (not recommended as the main path)

### Option 2: query parameter only
- Add `?mode=shuffle` to `GET /stations/{id}/queue`.
- Simpler, but the preference is not stored with the station and must be set every time.

### Option 3: hybrid
- Store `mode` on the station and also allow the query parameter to override it.
- This is the most flexible and is a good future step after Option 1.

## Suggested implementation order

1. Add `mode` migration.
2. Support `mode` in `build_station` and `_smart_queue`.
3. Update `GetStationQueue` in Go to respect stored `mode` and support `?mode=...` override.
4. Add tRPC/types support.
5. Add `StationManager` mode selector.
6. Add shuffle button/toggle in the player.
7. Use `played_at` to avoid recently played tracks.
