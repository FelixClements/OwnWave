# OwnWave redesign plan

Status: proposal, 2026-11-03. Decisions taken with the owner:

- Keep the backend core: Go API, Postgres, streaming/transcoding, auth/invites, Spotify embed scraper, playlist matcher.
- Rewrite the genre engine and stations. Build a new web UI. Add a native Android app.
- Android: Kotlin + Jetpack Compose + Media3 (`MediaLibraryService`) for Android Auto.
- Genre stations: OwnWave auto-suggests them, the user accepts, renames or drops.
- Android plays by streaming only (no offline downloads for now).

## Findings that drive this plan (live data, 2026-11-03)

### Genre analysis is broken by a sample-rate bug

- `discogs400` labels 5818 of 6290 tracks "Electronic". Its subgenres are Experimental (4234), Vaporwave (741), Industrial (684), Ambient (660), with mean confidence about 0.3.
- Cause: `genre_discogs400.py` builds `MonoLoader(sampleRate=16000, …)` and later calls `self._loader.configure(filename=path)`. Essentia's `configure()` resets every parameter that is not passed, so the loader decodes at 44100 Hz. Verified in the worker: same file, correct loader 3,276,801 samples, buggy loader 9,031,634 samples (ratio 2.756), `sampleRate` reads back 44100.
- The model therefore hears every track about 2.75x slower and lower. That reads as vaporwave/ambient/experimental, which are Electronic subclasses.
- Second issue: only the top-1 label's parent genre is used. Discogs400 has many more Electronic subclasses than any other parent, so even correct audio over-weights Electronic.
- File tags exist for 2667 tracks and look sane (Pop, Alternative Rock, Hip Hop, Rock, House…). They can serve as a ground-truth set to measure the audio model.

### Playlist matching works; the library lacks the songs

- 2 playlists, 201 items, 16 matched. Spot checks show unmatched tracks are not in the library (e.g. Amelie Lens "drift away"; the library has other Amelie Lens tracks).
- Gap: `(Mixed)` is not version noise, so "Affection (Mixed)" does not match "Affection".
- The library holds many duplicate tracks (same artist, title, duration). They need deduping or the matcher picks one at random.

## Target architecture

| Part | Tech | Change |
|------|------|--------|
| `services/go-api-server` | Go, Chi, pgx, FFmpeg | Keep. Add device tokens, quality profiles, station pools, genre suggestions, browse API for Android Auto. |
| `services/python-analytics` | Python, Essentia | Rewrite the genre pipeline. Drop the librosa/k-means features stations do not use. |
| `apps/web` | Vite + React + TanStack Query/Router + Tailwind | New Spotify-style SPA, served by Go as today. Replaces `apps/t3-web-frontend`. |
| `apps/android` | Kotlin, Compose, Media3 ExoPlayer, `MediaLibraryService` | New. Phone app plus Android Auto. |
| `database` | Postgres + pgvector | New migrations for genres, pools, suggestions, devices. |

The Next.js static export is replaced by Vite because the UI is a pure client-side SPA behind auth; Next adds build complexity and no server features are used. (Open to keeping Next if preferred.)

## Domain terms

- **Playlist**: a public Spotify playlist added by URL. Each sync adds new items and never removes old ones.
- **Playlist item**: one Spotify track seen in a playlist, with first/last seen dates.
- **Match**: the local library track for a Spotify track, or none. Unmatched items form the playlist's **missing list**.
- **Station genre**: one of about 20 broad genres used for stations (see taxonomy below). Not the same as a Discogs label or a file tag.
- **Pool**: the set of playlists a station draws from.
- **Station**: a pool plus an optional station-genre filter. It plays the matched local tracks in the pool, filtered by genre.
- **Suggestion**: a station OwnWave proposes after a sync: one per station genre with enough tracks in a pool.

## Phase 1: genre engine

Status: implemented on branch `phase1/genre-engine` (migration 018).


1. Fix loading: pass `filename`, `sampleRate=16000` and `resampleQuality` together on every `configure()`, or build a new `MonoLoader` per file. Add a test that asserts the loaded length equals `duration * 16000` within 1%.
2. Store the full Discogs400 activation vector per track (`vector(400)`) and the EffNet embedding mean (`vector(1280)`). Storing the vector lets us re-map genres without re-analysing audio.
3. Analyse 3 windows (at 20%, 50%, 75% of the track, 20 s each, `GENRE_WINDOW_SECONDS`) and average them. This avoids judging a 7-minute techno track by its intro. Cost is about 3 s per track (EffNet inference about 1.4 s per 30 s of audio on the home server).
4. Taxonomy: a checked-in table maps every Discogs400 label and common tag spellings to one station genre. Draft station genres: Techno, House, Trance, Drum & Bass, Dubstep/Bass, Ambient/Chill, Other Electronic, Hip Hop, R&B/Soul, Pop, Rock, Indie/Alternative, Metal, Punk, Jazz, Blues, Classical, Folk/Country, Latin, Reggae/Dancehall, World, Soundtrack.
   - Mapping is by full label, not by parent. `Electronic---Techno` maps to Techno; `Electronic---Vaporwave` to Other Electronic; `Hip Hop---Trap` to Hip Hop.
5. Scoring per track: per station genre take the strongest Discogs activation among its labels (max, not sum, so a genre with many labels cannot win by adding up small activations), then blend with tag genres and MusicBrainz artist genres. Starting weights: audio 0.5, file tags 0.35, MusicBrainz 0.15. When a source is missing, its weight is spread over the others.
   - Primary genre = top score. Secondary genres = any genre scoring at least 60% of the top. A track can sit in more than one station.
6. MusicBrainz lookup: one artist search per name (no key, 1 request/s), using the artist's community tags weighted by votes, cached per name in Postgres. A credit like "A, B feat. C" is looked up whole first, then as its primary artist.
7. Validation: use the 2667 tag-labelled tracks to report audio-only accuracy per station genre. Tune weights on this set.
8. Guardrail: an admin "genre distribution" view. A warning shows if one station genre holds more than 50% of tracks while tags disagree. Analysis runs record the model and pipeline version so stale results can be re-run.
9. Re-analyse the whole library (about 6.3k tracks) as a background job.

## Phase 2: playlists and stations

Status: implemented on branch `phase2/stations` (migration 019), stacked on Phase 1.


1. Matcher: add `mixed` to version noise; dedupe library duplicates (prefer the highest quality file); keep exact/fuzzy logic.
2. Missing list per playlist: unmatched items with title, artist, first seen (`GET /api/playlists/{id}?items=missing`), view only. Unmatched items are re-matched by the hourly sync, so files added by a scan are found within the hour.
3. Sync: keep the embed scraper (`open.spotify.com/embed/playlist/<id>`). The user pastes the normal public URL (`open.spotify.com/playlist/<id>`); OwnWave extracts the id and builds the embed URL itself. This already works today (`spotify.ParsePlaylistID`, covered by `TestParsePlaylistID`) and must stay that way. The embed shows up to about 100 tracks. Daily sync plus "sync now". Growth-only semantics stay as today.
4. Pools: a station's pool is 1..n playlists. "Combine playlists into a station" = create a station whose pool has several playlists.
5. Genre split: after a sync or analysis, compute matched tracks per station genre per pool. Suggest a station when a genre has at least 25 tracks (configurable). The suggestion list shows track counts and sample covers; accepting creates a normal station.
6. Within a station: shuffle with recently played tracks last (as today), with no repeat of the same artist within 3 tracks. Likes and bans are respected.

## Phase 3: API for clients

1. Device tokens: the Android app logs in once and gets a long-lived revocable token (`Authorization: Bearer`). The web keeps cookie sessions.
2. Quality profiles: `original` (FLAC), `high` (Opus 160k), `normal` (Opus 96k), `low` (Opus 64k), `aac128` for clients without Opus. The client picks one per request; Android stores one value for Wi-Fi and one for mobile data.
3. Transcode cache: cache transcoded files on disk keyed by track + profile, so seeking and replays do not re-encode.
4. Browse API matching the Android Auto tree: `/api/browse/root`, `/api/browse/{node}` returning stations, playlists, genres, recent.
5. Search endpoint for voice queries: station, genre, artist, track.
6. Publish an OpenAPI spec; generate the TypeScript and Kotlin clients from it.

## Phase 4: web UI

Spotify-inspired, dark-first, responsive:

- Left sidebar: Home, Search, Stations, Playlists, Library; admin section for admins.
- Home: "Jump back in" stations, suggested genre stations, recently synced playlists.
- Station and playlist pages: large header with mosaic cover art, accent colour sampled from the art, track list, match status per item, missing list tab.
- Bottom player bar: cover, title/artist, like/ban, progress, volume, quality selector. Full-screen now-playing view with large art and up-next queue.
- Media Session API so phone lock screens and headsets control playback; installable PWA.
- Admin: library scan status, genre distribution, sync log, users/invites.

## Phase 5: Android app

- Kotlin, Jetpack Compose, Material 3 with a dark theme matching the web UI.
- Media3 `MediaLibraryService` + ExoPlayer. The same service powers the phone notification, lock screen, Bluetooth controls and Android Auto.
- Android Auto: browsable tree (Stations, Genres, Playlists, Recent), voice search ("play techno on OwnWave"), thumbs up/down mapped to like/ban, resume last station on connect.
- Network-aware quality: Wi-Fi vs mobile profile; small read-ahead buffer for smooth playback in the car.
- Server URL plus login on first launch; token in EncryptedSharedPreferences.
- Distribution: signed APK from CI (GitHub Releases). Play Store is optional and needed only for Android Auto on head units that block sideloaded apps; the in-car developer setting "unknown sources" covers personal use.

## Order and rough size

1. Phase 1 genre engine: high value, about 1 PR for the loader fix + re-analysis, 1–2 PRs for taxonomy, blending, validation.
2. Phase 2 stations: 2 PRs.
3. Phase 3 API: 2 PRs.
4. Phase 4 web UI: 3–4 PRs (shell + player, stations/playlists, admin).
5. Phase 5 Android: 3–4 PRs (player service + login, browse UI, Android Auto, CI release).

Phases 4 and 5 can run in parallel once Phase 3 is merged.

## Decisions on open questions (2026-11-03)

- MusicBrainz lookups are allowed (artist names are sent to musicbrainz.org).
- The missing list is view-only for now. No wishlist or download integration.
- The old seed stations (artist, album, similar track, mood) are removed with all their code: `stations/seed.go`, `filter.go`, `schemas/station-seed.schema.json`, the `/similar` page and the station seed form. The k-means clustering and librosa feature pipeline go too, since nothing else uses them.
- Start from zero: the new version ships with a fresh database schema (one new baseline migration replacing migrations 001–017). All current data is deleted: users, sessions, invites, stations, playlists, matches, listening history, feedback, tracks and analysis results. The music files on disk are never touched; the first scan rebuilds the library, and the admin is created again through the setup page.
