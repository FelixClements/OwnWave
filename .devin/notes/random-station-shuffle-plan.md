# Plan: Random station queue (default and only behaviour)

## Scope change

The user now wants only one behaviour: every OwnWave station must play tracks in a random order from the matching pool, every time the queue is requested. This is the default and only mode. No `smart` or `shuffle` modes, no `mode` column, and the smallest possible change.

## Current queue-building and playback flow

### Queue building

- `services/python-analytics/src/station_builder.py` `build_station` (lines 30–59) loads all analysed tracks, applies the seed filter, excludes banned tracks, and calls `_smart_queue`.
- `_apply_filter` (lines 66–121) and `_exclude_banned` (lines 62–63) define the matching track pool.
- `_smart_queue` (lines 124–165) performs a deterministic greedy k-NN walk through that pool, occasionally using novelty jumps, and produces a fixed order.
- `insert_station_tracks` is called on line 57 to persist the resulting track ids in `station_tracks`.

### Storage

- `database/migrations/001_schema.up.sql` (lines 58–64) defines `station_tracks` with `station_id`, `track_id`, `position`, and `played_at`. The primary key is `(station_id, position)`. `played_at` is currently populated but not used to order or rotate playback.

### Queue serving

- `services/go-api-server/db.go` `GetStationQueue` (lines 230–280) joins `station_tracks` to `tracks` and `audio_features`, excludes `ban` feedback, and orders the result first by a `skip` penalty and then by `st.position`.
- `services/go-api-server/handlers.go` `GetQueue` (lines 363–372) exposes this as `GET /stations/{id}/queue`. `StationCrossfadeStream` (lines 646–682) also calls `GetStationQueue`.

### Frontend playback

- `apps/t3-web-frontend/lib/station.tsx` (lines 33–36) fetches the queue via `trpc.queue.useQuery` with a 5-second refetch interval.
- `apps/t3-web-frontend/components/Player.tsx` copies `queueProp` into `queueRef.current` (lines 52–56) whenever the station matches, and starts playback at `currentIndexRef.current = 0` (lines 224–226), then advances sequentially. Because the queue order is currently fixed, the same song starts every time.

## Proposed change

Make `GetStationQueue` return a new random order on every call. The player starts at index 0 of this already-shuffled queue, so both the starting track and the full order are random each time. No modes, no schema columns, and no new API parameters are introduced.

## Implementation steps

1. **Python analytics — keep the pool, drop reliance on order**
   - `services/python-analytics/src/station_builder.py` can still populate `station_tracks` on line 57, but the output of `_smart_queue` (lines 124–165) is now only a pool of eligible track ids. The stored `position` values no longer determine playback order.
   - The smallest option is to leave the queue-building code as-is and let the Go API randomise at request time. A later cleanup could replace `_smart_queue` with a simple random sample of the filtered pool, but that is not required for this scope.

2. **Go API — randomise at queue request**
   - In `services/go-api-server/db.go` `GetStationQueue` (lines 230–280), remove the deterministic `ORDER BY` on `st.position`.
   - After the rows are scanned into the `queue` slice, shuffle the slice in place before returning. This makes every `GET /stations/{id}/queue` call return a different order.
   - Keep the existing `ban` exclusion so banned tracks remain out of the pool.

3. **Avoiding recently played tracks (optional)**
   - `database/migrations/001_schema.up.sql` already provides `station_tracks.played_at` (line 62), and `MarkTrackPlayed` in `services/go-api-server/db.go` (lines 283–290) already updates it.
   - Before shuffling, either:
     - Exclude tracks whose `played_at` is within the last N hours, or
     - Sort by least-recently-played first, then shuffle only the tracks that are not very recent.
   - If the pool becomes empty after filtering, fall back to the full matching pool.

4. **Frontend — player starting point and queue refresh**
   - `apps/t3-web-frontend/components/Player.tsx` already starts at `currentIndexRef.current = 0` (lines 224–226). With a shuffled queue, index 0 is a random starting track.
   - The queue currently refetches every 5 seconds (`apps/t3-web-frontend/lib/station.tsx`, lines 33–36) and `Player.tsx` updates `queueRef.current` from `queueProp` (lines 52–56). This can overwrite the in-memory queue with a new random order while a track is playing, causing a jump or an invalid index.
   - Add a guard so `queueRef` is not overwritten while the same station is already playing. Alternatively, disable the 5-second refetch while playing and only refetch when the selected station changes.

5. **Database / schema**
   - No migration is required. Do **not** add a `mode` column.
   - `station_tracks.position` can remain because it is part of the primary key; it simply no longer drives playback order.
   - `station_tracks.played_at` is reused for the optional recently-played exclusion.

6. **API surface**
   - `GET /stations/{id}/queue` (`services/go-api-server/handlers.go`, lines 363–372) keeps the same request and response shape; only the returned order changes.
   - No new query parameters are added.

## Rationale

- Fulfils the request with the smallest footprint: the core change is in `GetStationQueue`, plus a small frontend guard.
- Reuses the existing pool-building and feedback filtering pipeline.
- Uses the existing `played_at` column rather than adding schema.
- No modes to support, test, or migrate; random is the only and default behaviour.
