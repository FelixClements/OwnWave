# 0004: Read every track of a Spotify playlist, like SoulLite

Status: accepted, 2026-11-13. No migration.

## Context

OwnWave read a playlist only from Spotify's public embed page, which lists
about the first 100 tracks. SoulLite (ADR 0003 makes it follow OwnWave's
playlists) reads the whole playlist through SpotipyFree, and falls back to the
embed page only when that fails. So for a playlist over 100 tracks, OwnWave
matched the first 100 while SoulLite downloaded all of them.

SpotipyFree gets an anonymous web-player token (a TOTP whose secret it
downloads from a third-party repo), a client token, and the hash of the web
player's `fetchPlaylist` query, then pages through
`api-partner.spotify.com/pathfinder/v1/query` 343 items at a time.

## Decision

OwnWave does the same read, with less: the embed page already carries an
anonymous access token, and that token alone is enough for the playlist
query. So OwnWave needs no TOTP secret and no client token.

1. Fetch the embed page as before; keep its tracks, name and cover, and read
   `settings.session.accessToken`.
2. Find the `fetchPlaylist` hash in the desktop web player's main script
   (`open.spotify.com` with a desktop Chrome User-Agent; other agents get the
   mobile player), or else in its script chunks. Cache it for 6 hours; when a
   query fails with a cached hash, scrape it again once.
3. Page through the query (343 per page) until `totalCount`. Keep the same
   items the embed read keeps: `spotify:track:` items with a title.
4. If any step fails, keep the embed page's tracks, mark the read partial and
   log a warning, as SoulLite does.

## Consequences

- Playlists over 100 tracks now match in full, and OwnWave sees the same
  tracks SoulLite downloads.
- This relies on undocumented Spotify internals (the token in the embed page,
  the query and its hash, the script layout). When they change, reads quietly
  drop back to the first 100 tracks; the warning in the log is the signal.
- One sync after a web-player release downloads the main script (about 5 MB)
  again.
- OwnWave skips local files (`spotify:local:`) and podcast episodes, as it
  always has. SoulLite may keep a local file if SpotipyFree returns one (not
  checked); such a file has no Spotify track to download anyway.
