# 0003: SoulLite follows OwnWave's playlists

Status: accepted, 2026-11-12. Issue #102. No migration.

## Context

OwnWave matches Spotify playlists to local files. Some tracks have no file,
and they show up on the playlist's missing list. SoulLite is a separate
self-hosted app. It follows Spotify playlists, downloads the tracks the
library lacks into the shared music folder, and then calls OwnWave's
`POST /api/admin/scan`. That call already worked. What was missing was the
other direction: an admin had to add each playlist in both apps.

SoulLite has an HTTP API that takes an admin-scope API Key. Its relevant
endpoints:

- `POST /playlists {url, name, schedule}` returns 201. It returns 409 if the
  playlist is already followed.
- `POST /playlists/{id}/sync`
- `GET /playlists` returns a bare list.
- `DELETE /playlists/{id}` keeps the files.

SoulLite knows a playlist only by its own integer id and by its URL.

## Decision

1. **Off unless configured.** The integration is off by default and turns on
   only when both `SOULLITE_URL` and `SOULLITE_API_KEY` are set. Setting just
   one of them is a startup error, so a half-configured install fails
   loudly rather than silently doing nothing.
2. **Push on add and delete, in the background.**
   - After `AddPlaylist` succeeds, OwnWave sends
     `https://open.spotify.com/playlist/<spotify_id>` with schedule `daily`,
     the same rate as OwnWave's own playlist sync.
   - On 201, OwnWave also starts a Sync so downloads begin at once.
   - After `DeletePlaylist` succeeds, OwnWave finds SoulLite's id by comparing
     Spotify ids in `GET /playlists`, then unfollows it.
   - Each call runs in its own goroutine with a 30 s limit. Failures are only
     logged, so SoulLite never fails or slows an admin request.
3. **Catch up on every start.** On startup, OwnWave sends every playlist
   again. SoulLite's 409 "already followed" counts as success and leaves
   those playlists alone. This covers follows lost while SoulLite was down,
   and playlists added before the integration was turned on, without a retry
   queue or any new state.
4. **No stored SoulLite ids.** Unfollowing looks the id up by URL instead of
   adding a column. SoulLite's list is small and has no pagination.

## Consequences

- OwnWave is the source of truth for which playlists SoulLite follows. A
  playlist unfollowed only in SoulLite is followed again on the next OwnWave
  restart. To stop it for good, delete it in OwnWave.
- A follow lost while SoulLite is down waits for the next OwnWave restart.
  An hourly catch-up would close that gap sooner, but it would also undo
  SoulLite-side unfollows within the hour, so it was not chosen.
- A playlist an admin had already added by hand in SoulLite keeps its own
  schedule, because a 409 changes nothing.
- The SoulLite API Key needs `admin` scope. A `read` key fails every follow
  with 403, which shows up only in the logs.
