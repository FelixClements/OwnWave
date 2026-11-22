# 0005: One writer per table, Go or Python

Status: accepted, 2026-11-22. Issue #110, map #109. No migration.

## Context

Go (`services/go-api-server`) and Python (`services/python-analytics`) share
one Postgres database. Nothing said which service may change which table, so
a rule about a table could end up split across both services. For example,
Python builds `tracks` from the music folder, but Go also wrote
`tracks.copy_of` until #112.

This ADR records who writes what today. Each owner below was checked against
the `INSERT`, `UPDATE` and `DELETE` statements in both services' non-test
code on 2026-11-22. Paths are relative to each service's root (`src/` for
Python).

## Decision

1. **Every table has exactly one writing service.** Only that service runs
   `INSERT`, `UPDATE` or `DELETE` on the table.
2. **"Ownership" means writing.** Either service may read any table in SQL.
   For example, Go reads Python's `genres_scored` key and Python reads Go's
   `genre_corrections`.
3. **A service that needs another owner's table changed asks that owner**
   over HTTP or through a table it owns itself. It does not write the table.
   This is why #109 rejected Go writing `task_queue` (Draft B).
4. **`ON DELETE` actions declared in a migration are not a breach.** When
   Python deletes a track, the database removes or clears the rows that
   reference it. That follows from the schema, not from a second writer.
5. **`app_state` is split by key.** Each key has one writing service (see
   below).
6. **A new table names its owner** in the ticket that adds it, and the ticket
   updates this ADR.

### Python owns: the library, analysis and scans

| Table | Written in |
| --- | --- |
| `tracks` | `db.py` (upsert by path, delete), `library_scan/prune.py` (moved files) |
| `artists` | `db.py` |
| `albums` | `db.py` |
| `audio_features` | `db.py` |
| `track_audio_genres` | `db.py` |
| `musicbrainz_artists` | `tasks.py` |
| `track_station_genres` | `genre_scoring.py` (including `corrected`, which it derives from Go's `genre_corrections`) |
| `station_genres` | `genre_scoring.py` |
| `scan_jobs` | `db.py`, `tasks.py` |
| `scan_job_progress` | `db.py` |
| `task_queue` | `task_queue.py` |

### Go owns: users, listening, taste, stations and playlists

| Table | Written in |
| --- | --- |
| `users` | `internal/auth/service.go` |
| `sessions` | `internal/auth/service.go` |
| `user_invites` | `internal/auth/service.go` |
| `listening_history` | `internal/listening/service.go` |
| `track_feedback` | `internal/listening/service.go` |
| `stations` | `internal/stations/service.go`, `internal/stations/genre_stations.go` |
| `station_playlists` | `internal/stations/service.go` |
| `station_suggestion_dismissals` | `internal/stations/service.go` |
| `station_refresh_runs` | the genre-station refresh |
| `station_hidden_tracks` | `internal/listening/service.go` |
| `genre_corrections` | `internal/stations/genre_corrections.go` |
| `playlists` | `internal/playlists/service.go`, `internal/playlists/picks.go` |
| `playlist_items` | `internal/playlists/service.go` |
| `spotify_track_matches` | `internal/playlists/service.go` |
| `playlist_library_picks` | `internal/playlists/picks.go` |
| `playlist_pick_removals` | `internal/playlists/picks.go` |
| `track_copies` | `internal/playlists/copies.go` (#112, migration 028) |

### `app_state`, split by key

| Key | Writer | Written in | Read by |
| --- | --- | --- | --- |
| `setup_completed` | Go | `handlers_setup.go`, through `settings.Service.Set` in `internal/settings` | Go |
| `auto_stations` | Go | `SetGenreStationsEnabled` in `internal/stations/genre_stations.go` | Go |
| `genres_scored` | Python | `score_genres` in `tasks.py` | Go (`internal/pipeline/pipeline.go`) |

`settings.Service.Set` writes any key it is given, but today only
`setup_completed` goes through it. Every key is a `settings.Key` constant in
`internal/settings/settings.go` (Python's `genres_scored` is also
`GENRES_SCORED_KEY` in `tasks.py`). A new key gets a constant there and a row
in this table. (Paths updated by #114, which split the root `db.go`.)

### Dropped tables: no owner

These tables are created in migrations and dropped by later ones. No code
writes them, so they need no owner. This settles the ⚠ unverified entries in
#110 and the open question in #109.

| Table | Created | Dropped |
| --- | --- | --- |
| `station_tracks` | 001 | 014 (live stations) |
| `track_genres` | 009 | 018 (genre engine) |
| `track_clusters` | 001, 003 | 019 |
| `cluster_centers` | 001, 003 | 019 |
| `feature_stats` | 001, 003 | 019 |

### Contract: Python keeps a track's id when its file moves

Go keys listening history, likes and bans, playlist matches, hidden tracks
and copies by `tracks.id`. So when a file moves or is renamed, Python must
keep the old row and update its path. It must not delete the track and
import it again under a new id.

`library_scan/prune.py` does this today. `match_moved_tracks` pairs a track
whose file vanished with a track first seen in this scan (same byte size,
duration within tolerance, unambiguous on both sides). Prune then deletes
the fresh row and moves its path, size and mtime onto the old row. An
ambiguous pair, such as two copies of one file, is not matched, and the
vanished track is deleted as before.

Any change to scans keeps this contract. ADR 0006 (#116, incremental scans)
relies on it: prune still sees the full listing.

## Rules the code broke

- **Go wrote `tracks.copy_of`.** Fixed by #112: migration 028 moved
  `copy_of` into the Go-owned `track_copies` table, and
  `internal/playlists/copies.go` writes that instead. No Go code writes
  `tracks` now. When prune re-homes a moved file it keeps the old track's
  id, so that track's `track_copies` rows stay. The fresh row it deletes
  loses its rows through `ON DELETE CASCADE`, and the next grouping run
  rebuilds them.

No table has two writers now.

## Planned

Tables and columns that approved tickets will add, with their owner:

| Table or column | Owner | Ticket |
| --- | --- | --- |
| `track_analysis_failures` | Python | #119 |
| `scan_jobs.rerun_requested` | Python (a column of a Python table) | #120 |

ADR 0006 (`0006-incremental-scans.md`, #117) adds both and relies on them
being Python's. When a ticket lands, its row leaves this table, and
`track_analysis_failures` joins the Python table above.

## Consequences

- A rule about a table lives in one service, so it is changed and tested in
  one place.
- Go cannot fix a Python table directly, even for a one-line change. It goes
  through Python's API or a Go-owned table, as `genre_corrections` does for
  genre scores.
- The contract above ties Go's per-track data to Python's scan. A scan
  change that loses track ids would quietly detach history and likes.
- Nothing in the database enforces this yet. Ownership is checked in review
  against this ADR.
