# 0001: Genre stations that build and maintain themselves

Status: accepted, 2026-11-04. Migration 022. Partly superseded by
[ADR 0002](0002-library-genre-stations.md), 2026-11-05: genre stations now
play the whole library by main genre, so `all_playlists` no longer applies
to them (decisions 1 and 2), the thresholds in decision 4 count library songs,
and decision 6 is gone.

## Context

Stations were made by hand or by accepting a suggestion. Keeping a "House"
station current meant noticing new playlists and new files and editing the
station. The "All playlists" suggestion stored the playlist ids it saw when
accepted, so a later playlist was missing from it, and the suggestion came
back as a near-duplicate (dismissals keyed on the id list had the same bug).

We want one shared station per station genre, built from every playlist and
kept current with no manual steps: new files → scan → analysis → genre
scoring → playlist matching → genre stations refreshed.

## Decision

1. **Genre stations are ordinary rows in `stations`.** A genre station has
   `auto_genre` set (unique, partial index), `user_id NULL` (shared),
   `all_playlists = TRUE` and `genres = {auto_genre}`. Queue, cover,
   history, feedback, browse, search and Android Auto work unchanged. A
   check constraint keeps genre stations shared and all-playlists.

2. **"All playlists" is a live pool.** `stations.all_playlists` means "every
   playlist, now and later"; the queue resolves it when it plays. Any
   station can use it. The old snapshot-of-ids behaviour is gone, which
   fixes the duplicate suggestion bug.

3. **Refresh is idempotent and never deletes.** `stations.Service.Refresh`
   runs in one transaction under an advisory lock. Per station genre it
   inserts the station with `ON CONFLICT (auto_genre) DO NOTHING`, then
   shows or hides it. A refresh that has nothing to do writes nothing, and
   a second run reports no changes.

4. **Hysteresis.** A station is shown from 25 matched tracks of its genre
   (across all playlists) and hidden only below 20, so a genre near the line
   does not flicker. A hidden station keeps its id, name, history and
   feedback and comes back with the same id. Hidden stations answer 404 to
   users and are left out of lists, browse and search.

5. **Admin controls** (`/api/admin/genre-stations`):
   - on/off for all genre stations (`app_state` key `auto_stations`, on by
     default); off hides them, it deletes nothing;
   - rename (`name_is_custom` keeps the name across refreshes; an empty name
     goes back to the genre's display name, which then follows renames in
     the taxonomy);
   - exclude a genre (hidden and kept hidden; can be set before the genre
     ever reaches the threshold).
   Genre stations cannot be deleted or have their pool or genres changed;
   the API answers 409 or 400 and points to Admin.

6. **No collisions with custom stations.** A custom station that is shared,
   all-playlists and exactly one genre would duplicate a genre station: it is
   refused with 409 while that genre's station is not excluded, and an
   exclusion cannot be lifted while such a custom station exists.
   Suggestions now cover single playlists only and skip a genre whose
   playlist tracks are exactly what a live genre station already plays.

7. **Hand-off by NOTIFY, state in the database.** The analytics worker sends
   `pg_notify('ownwave_pipeline', 'scan_finished' | 'genres_scored')` inside
   the transaction that did the work, so the notice arrives only after
   commit. The API listens on a dedicated connection and runs the pipeline
   (match pending playlist tracks, then refresh) right away. NOTIFY only
   wakes the runner: everything it acts on is in the database, so a missed
   notice is caught up at startup, on every reconnect and by the hourly
   tick. Triggers that arrive while a run waits fold into it. Playlist add,
   sync and delete refresh in the request.

8. **Run log.** Each run is written to `station_refresh_runs` (trigger,
   time, tracks matched, changes, error). Runs that changed nothing are
   dropped except the newest, and the log keeps 50 rows. The worker records
   its last genre scoring in `app_state` key `genres_scored`. The Admin page
   shows the last scan, the last genre scoring and recent runs, with a
   "Refresh now" button.

9. **Clean start.** Migration 022 deletes every station (shared and
   personal) and every suggestion dismissal. Listening history and feedback
   stay (their `station_id` becomes NULL). The first refresh after deploy
   builds the genre stations. There is no automatic "Everything" station.

## Consequences

- Deploying 022 removes all existing stations; users re-create custom ones.
- Genre stations appear without anyone accepting anything; an admin who
  does not want one excludes it.
- A new file reaches its genre station after the scan, analysis and genre
  scoring finish, without waiting for the hourly sync.
- If the listener is down, stations still catch up within the hour.
