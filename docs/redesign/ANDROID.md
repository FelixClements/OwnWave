# Android app: implementation plan

Detail for [Phase 5 of the redesign plan](PLAN.md#phase-5-android-app).

## Status (2026-11-11)

| Step | State |
| --- | --- |
| Plan and decisions | ✅ Done (this file). |
| Authentik gate bypass for app API calls (Caddy) | ✅ Done and checked live. See [the blocker section](#blocker-getting-past-the-authentik-gate). |
| PR 1: scaffold, client, sign-in, station list | ✅ Merged: [PR #87](https://github.com/FelixClements/OwnWave/pull/87). |
| PR 2: playback | 🔄 Built on branch `android/playback` (from `main`). Build, 38 unit tests and lint pass. On the emulator with a real account: stations play, the queue and covers load, and a play was recorded (the station moved to the top of "Jump back in"). Still to check on a phone: an hour with the screen off, lock screen and Bluetooth controls, the notification's like and ban buttons. |
| PR 3: Android Auto | ⬜ Not started. |
| PR 4: signed release | ⬜ Not started. |

### Picking this up later

- **Build:** `cd apps/android && ./gradlew :app:assembleDebug :app:testDebugUnitTest :app:lintDebug`. You need JDK 21 and the SDK platform `android-37.0`. On the dev box, the toolchain is in `~/Android`: JDK, Gradle and an SDK with platforms 36 and 37.0.
- **Emulator:** the AVD `t3_pixel` (Android 16), opened through the T3 Device panel. Install with `ANDROID_SERIAL=emulator-5554 ./gradlew :app:installDebug`. The DHU for Android Auto is not installed yet (SDK extras `extras;google;auto`); PR 3 needs it.
- **Versions:** they are pinned in `apps/android/gradle/libs.versions.toml`. Notes from the scaffold:
  - `kotlinx-coroutines` is 1.10.2 because 1.12.0 could not be resolved from Maven Central.
  - openapi-generator is 7.26.0 because 7.27.0 was too new.
  - Current Coil and AndroidX releases need compileSdk 37.
- **Open infra items** (wiki `todo/backlog`):
  - A CrowdSec parser for OwnWave app traffic (see Risks).
  - An OwnWave fix so it trusts only `X-Real-IP`. Caddy strips `True-Client-IP` in the meantime.

## Goal

A native app that plays OwnWave stations on the phone and in Android Auto, using the client API from Phase 3 (device tokens, quality profiles, browse tree, voice search).

In v1:

- Sign in with server URL, username and password. Sign out.
- List stations (shared and own, recent first) and play one.
- Background playback with notification, lock screen and Bluetooth controls.
- Like and ban the current song. A ban skips it.
- Wi-Fi and mobile-data quality settings.
- Android Auto: browse tree, voice search, thumbs up/down, resume the last station.

Not in v1: offline downloads, registration and invites, admin pages, creating or editing stations, crossfade, playlist pages.

## Decisions

| Topic | Decision |
| --- | --- |
| Application id | `io.github.felixclements.ownwave`, which is also the root package. It is permanent, because a new id is a separate install. |
| Language and UI | Kotlin, Jetpack Compose, Material 3, dark theme matching the web UI. |
| Playback | Media3 ExoPlayer and `MediaLibraryService`. One service serves the phone UI, the notification, Bluetooth and Android Auto. |
| API client | Generated at build time from `services/go-api-server/openapi.yaml` by the openapi-generator Gradle plugin. Generator `kotlin`, library `jvm-retrofit2`, `kotlinx_serialization`, coroutines. Output goes to `build/` and is never checked in. A spec change that breaks the app breaks the compile, so the client cannot drift. |
| HTTP | One OkHttp client with an auth interceptor, shared by Retrofit, ExoPlayer (`OkHttpDataSource`), Coil and the artwork provider. |
| Token storage | DataStore, with the token encrypted by an AES-GCM key held in the Android Keystore. This replaces EncryptedSharedPreferences from PLAN.md, because `androidx.security:security-crypto` is deprecated. |
| Dependency injection | A small hand-written `AppContainer`, with no Hilt. The app has about ten collaborators. |
| SDK levels | `minSdk` 28, `compileSdk` and `targetSdk` 37. Current AndroidX, Coil and Media3 releases require compileSdk 37. |
| Build | Gradle wrapper committed in `apps/android`, version catalog `gradle/libs.versions.toml`, and the newest stable AGP, Kotlin and Media3 when the project is scaffolded. |

## Blocker: getting past the Authentik gate

**✅ Resolved on 2026-11-10 with option 1.** The `ownwave.silver-sand.com` block in the edge VPS's `/etc/caddy/Caddyfile` skips `forward_auth` in two cases:
- the path starts with `/api/` (an exact, case-sensitive check) and the request has an `Authorization: Bearer …` header;
- the request is `POST /api/auth/device`.

Those requests still go through `edge` (AppSec). Client-sent `X-Authentik-*` and `True-Client-IP` headers are stripped from them. `True-Client-IP` is stripped because chi's `RealIP` trusts it, and that would make the sign-in rate limit spoofable. Caddy's `path()` matcher was not used, because it ignores case: `/API/me` slipped past the gate.

Checked live:
- Without a token, `/` and `/api/me` get 302 to Authentik.
- `/api/me` with `Bearer x` gets OwnWave's own 401.
- A real device token works.

Details, backups and the CrowdSec analysis are on the wiki (`infrastructure/vps`, `apps/ownwave`, changelog). The original analysis follows.

`https://ownwave.silver-sand.com` is behind Caddy `forward_auth` to Authentik (`silver-sand-admin`, MFA), and the API is not on the LAN (only `10.200.200.2:8090` on wg0). An app request with `Authorization: Bearer` gets a 302 to the Authentik login. Android Auto runs on mobile data, so the app has to use the public URL.

This has to be fixed before PR 1 can be tested on a real phone. Options:

1. **Recommended.** In Caddy, skip `forward_auth` for `/api/*` requests that carry an `Authorization: Bearer` header, and for `POST /api/auth/device`. OwnWave checks the token itself: tokens are hashed and revocable, and login is limited to 10 per minute per IP. The browser keeps the gate, because it sends cookies, not Bearer headers. Anyone can send the header, but they only reach OwnWave's own auth.
2. Add a second host (for example `ownwave-api.silver-sand.com`) with no gate that serves `/api/*` only. Same exposure as option 1, clearer logs, and one more DNS name and certificate.
3. Authentik outpost tokens or a VPN on the phone. This is heavy for the car, so it is not recommended.

CrowdSec bans bursts of 4xx responses (`http-probing`; this happened on 2026-11-04 with cover 404s). The app has to avoid such bursts. On the first 401 it stops every request and goes back to login. It never retries 4xx responses, and it caches covers on disk. Covers without art already return 204.

Once the Caddy change is made, it goes in the infra wiki (`apps/ownwave`, `infrastructure/vps`, changelog).

## Project layout

✅ = exists since PR 1. Everything else is planned.

```
apps/android/
  ✅ settings.gradle.kts, build.gradle.kts, gradle/libs.versions.toml, gradlew
  ✅ api/                    generated client only (kotlin library module)
  ✅   build.gradle.kts      openApiGenerate from ../../../services/go-api-server/openapi.yaml
  app/src/main/kotlin/io/github/felixclements/ownwave/
  ✅ OwnWaveApp.kt           Application, builds AppContainer, Coil ImageLoader on the shared OkHttp
  ✅ AppContainer.kt         OkHttp (no redirects, 50 MB cache), APIs, stores, repositories
  ✅ MainActivity.kt         Loading / login / home by SessionState
  ✅ auth/                   Session, SessionStore (DataStore + TokenCipher/Keystore), AuthInterceptor,
                             AccountRepository (login/logout), ServerUrl (parse + same-origin check)
  ✅ data/                   ApiFactory (Retrofit per server), StationRepository
  ✅ data/                   PlaybackRepository (queue, plays, like, ban), SettingsStore (PR 2)
  ✅ ui/                     theme, login, home, SignedInApp, player (mini player, now playing), settings
  ✅ playback/
  ✅   PlaybackService.kt     MediaLibraryService + MediaLibrarySession.Callback
  ✅   MediaIds.kt            station:/track: media ids, ownwave://track/<id> URIs
  ✅   StationQueue.kt        refill, dedupe; pure Kotlin
  ✅   PlayReporter.kt        30 s or half-track rule -> POST /played
  ✅   QualityPolicy.kt       network -> Quality
  ✅   StreamResolver.kt      ownwave://track/<id> -> https URL with current quality
  ✅   Commands.kt            LIKE / BAN session commands
      BrowseTree.kt          /api/browse -> MediaItems (Auto)
    artwork/ArtworkProvider.kt  content:// proxy for covers (Auto)
```

There are two modules because generated code is large and compiles slowly. Keeping it in `:api` keeps `:app` builds incremental.

## How it works

### Sign in (✅ PR 1)

The login screen asks for a server URL, a username and a password.

- **Server URL:** reduced to its origin. `https://` is added when no scheme is given. `http://` is allowed only in debug builds, for a dev server such as `http://10.0.2.2:8080`. The release network security config is HTTPS only.
- **Sign-in:** calls `POST /api/auth/device` with `device_name = Build.MODEL` and stores `{server, token, user}`.
- **Bearer header:** `AuthInterceptor` adds it only to requests with the stored scheme, host and port.
- **401:** the app clears the session and shows the login screen. PR 2 also has to stop the player.
- **Sign out:** calls `POST /api/logout`, then clears the session.
- **Redirects:** OkHttp does not follow them. A 3xx, or a non-JSON 200, shows "The server did not answer like OwnWave" (a login gate in front of the server).
- **Backups:** off (`allowBackup=false`, data extraction rules exclude everything), because the token is bound to this device's Keystore key.

### Playing a station (✅ PR 2)

How it was built, where it differs from the plan below:
- The UI plays a station by setting one MediaItem with id `station:<id>`. `onSetMediaItems` in the service swaps it for the station queue, so Auto (PR 3) can use the same path.
- **Like and ban:** the session publishes the current track's like state in its session extras, and the phone UI reads it from there. In the notification, like sits in the back-secondary slot and ban in the overflow.
- **Unlike:** the generated `PlaybackApi.deleteFeedback` cannot work, because Retrofit refuses `@Body` on `@DELETE`. `ApiFactory.FeedbackApi` makes that one call with `@HTTP(method = "DELETE", hasBody = true)`. A unit test covers it.
- **No retries on 4xx:** ExoPlayer's default load policy retries failed loads. `NoRetryOnClientErrors` stops that for 4xx responses.
- **Errors:** a track the server cannot serve (4xx other than 401/429, or a file that cannot be parsed or decoded) is removed and skipped, at most 3 in a row. Network errors, 401 and 429 pause playback instead.
- **Sign-out:** the service watches the session. Signing out, or a 401 (which signs out), stops the player and clears the queue.
- **Length while transcoding:** the first play of a track in a new quality has no known length. The UI then shows the queue's `duration_seconds` and disables seeking.
- **Settings:** Wi-Fi offers original, high, normal and low. Mobile data offers high, normal and low. `aac` is not offered.

The plan:

- Playing a station calls `GET /api/stations/{id}/queue` (up to 200 tracks) and sets them as the player's MediaItems.
- A MediaItem's id is `track:<trackId>|station:<stationId>` and its URI is `ownwave://track/<trackId>`. A `ResolvingDataSource` turns that URI into `<server>/api/stream/<id>?quality=<q>` when the track loads. The quality therefore follows the network at load time, and saved items never contain the host or the token.
- **Refill:** when the player is 3 items from the end, the app fetches the queue again and appends tracks that are not among the last 50 played. A station keeps playing without end.
- **Quality:** `QualityPolicy` checks `NET_CAPABILITY_NOT_METERED`. Defaults are `high` (Opus 160) on Wi-Fi and `normal` (Opus 96) on mobile data. Both can be changed in Settings. `original` is only offered for Wi-Fi.
- **Plays:** `PlayReporter` posts `POST /api/tracks/{id}/played {station_id}` once per item, after 30 s or half the track (the same rule as the web player).
- **Like and ban:** custom `SessionCommand`s, shown through `setMediaButtonPreferences` (heart and block icons) in the notification and in Auto. Ban calls `POST /feedback {ban}`, removes the item and skips. Like toggles between `POST` and `DELETE`.
- **Buffering:** `DefaultLoadControl` with a modest buffer (about 30 s minimum, 120 s maximum). It is enough for tunnels in the car without fetching a whole track over mobile data. The first play of a track in a new quality streams while the server transcodes it (no Content-Length), so the seek bar may be unknown until the transcode is cached.
- **Audio focus and noisy audio:** `setAudioAttributes(..., handleAudioFocus = true)` and `setHandleAudioBecomingNoisy(true)`.

### Android Auto

- `onGetLibraryRoot` and `onGetChildren` map `GET /api/browse?node=…` one to one. A `BrowseItem` becomes a MediaItem with `isBrowsable` and `isPlayable` set. The root has 4 tabs at most (Stations, Recent, Genres, Playlists), which fits Auto's tab limit.
- Selecting a playable `station:<id>` goes through `onSetMediaItems`, which loads the station queue as above.
- **Voice:** in `onSetMediaItems`, a request with `requestMetadata.searchQuery` calls `GET /api/search/stations?q=` and plays the first hit. `onSearch` and `onGetSearchResult` serve Auto's search screen with the same endpoint.
- **Resume:** the last station id and track are saved on every transition. `onPlaybackResumption` rebuilds the queue from them, so Auto and the system media controls resume where playback stopped.
- **Artwork:** Auto loads artwork URIs itself and cannot send the Bearer header. `ArtworkProvider` (authority `io.github.felixclements.ownwave.artwork`, exported, read-only, images only) serves `content://…/track/<id>` and `…/station/<id>`. It fetches through the shared OkHttp client, which has a 50 MB disk cache. On a 204 it serves a bundled placeholder.
- `automotive_app_desc.xml` declares `media`, and the manifest has the `MediaLibraryService` intent filters.

### Phone UI

- **Login.** ✅
- **Home:** Jump back in (`browse?node=recent`), Genre stations, Your stations. Each card shows the cover and the track count, and a tap plays it. ✅ Partly done in PR 1:
  - Done: a grid with "Stations" (custom) and "Genre stations", covers from `/api/stations/{id}/cover`, song counts, pull-to-refresh, and sign out in the menu.
  - ✅ PR 2: "Jump back in" (up to 6 recent stations) and tap to play.
- **Mini player and now playing:** cover, title and artist, like and ban, progress, the next items in the queue. ✅ PR 2 (the next 5 items).
- **Settings:** quality on Wi-Fi and on mobile data, signed-in devices (`GET/DELETE /api/me/devices`, with the current one marked), sign out. ✅ PR 2: quality and sign out. The devices list is in PR 4.
- The UI talks to the service through `MediaController`. It never holds its own player.

## Testing

- **JVM unit tests:** `StationQueue` (refill, dedupe), `PlayReporter` (30 s and half-track thresholds, once per item), `QualityPolicy`, `StreamResolver`, `AuthInterceptor` (host check, 401 handling), and the repositories against OkHttp `MockWebServer` with recorded JSON shaped by the spec.
  - ✅ PR 1 has 13 tests:
    - `ServerUrlTest`: parsing and the same-origin check.
    - `AccountRepositoryTest`: success, 401, 429, a 302 gate, an HTML page, 503, and Bearer only for the signed-in server with sign-out on 401.
  - ✅ PR 2 adds 25 tests, for 38 in all: `MediaIdsTest`, `StationQueueTest`, `PlayReporterTest`, `QualityPolicyTest`, `StreamResolverTest`, and `PlaybackRepositoryTest` (MockWebServer: queue, a failure that is not retried, ids that are not UUIDs are never sent, play body, like POST and unlike DELETE with body, ban).
- **Manual:** emulator or phone through the T3 device panel. Android Auto through the Desktop Head Unit (DHU, in the SDK extras) with "unknown sources" turned on in Auto's developer settings.
- `openapi_test.go` already keeps the spec complete. The Android build checks that the spec still generates and compiles against the app.

## CI and release

- A new `android` job in `.github/workflows/ci.yml`: Temurin 21, `gradle/actions/setup-gradle`, then in `apps/android` run `./gradlew :app:assembleDebug :app:testDebugUnitTest :app:lintDebug`.
  - ✅ Done in PR 1. It runs on every push and PR (no path filter; about 3.5 minutes) and uploads the debug APK as the `ownwave-debug-apk` artifact (kept 14 days).
- **Release** (last PR): on a tag `android-v*`, build `assembleRelease`, signed with a keystore from repository secrets (base64 keystore plus passwords; it never touches the repo or the wiki), and attach the APK to the GitHub Release. `versionName` comes from the tag and `versionCode` from the run number.

## PR sequence

1. ✅ **Scaffold, client and login.** Gradle project, `:api` generation (spike this first; see Risks), theme, `AppContainer`, `SessionStore`, the login screen, a Home that lists stations (with no playback), and the CI build job. Done when a debug APK signs in against the real server and lists stations. This needs the Caddy fix. **Done:** [PR #87](https://github.com/FelixClements/OwnWave/pull/87). The acceptance check passed on 2026-11-10 (emulator, real account).
2. 🔄 **Playback.** Built on `android/playback`; the phone checks are still open (see Status). `PlaybackService`, `StreamResolver`, `StationQueue` refill, `QualityPolicy`, `PlayReporter`, like and ban, the mini player and now playing screen, and the Settings quality options. Done when a station plays for an hour with the screen off, controls work from the lock screen and Bluetooth, and plays show up in web history.
3. **Android Auto.** `BrowseTree`, `ArtworkProvider`, voice search, resume, and the Auto manifest bits. Done when the DHU browses all 4 tabs with covers, "play techno on OwnWave" starts a station, and thumbs work.
4. **Release.** Signed release build, the GitHub Release on tag, the devices screen, README install steps, and an infra wiki entry.

## Risks and open questions

- **Generator and OpenAPI 3.1.** Checked on 2026-11-10: openapi-generator 7.26.0 (`kotlin`, `jvm-retrofit2`, `kotlinx_serialization`) generates the whole spec as it is, including the nullable unions, `QueueTrack` and the inline `loginDevice` body, and it compiles. The only addition was OkHttp's logging interceptor. If a later spec change breaks generation, the fix order is `openapiNormalizer` rules first, then named components in the spec, and hand-written models last.
- **CrowdSec and app traffic.** Requests that skip Authentik have no `authentik_user` tag, so the `local/authentik-user-ok` whitelist does not cover them.
  - Fast browsing (Auto's grids, cover loads, skipping) could trip `http-crawl-non_statics` (about 40 distinct paths in a burst), and 404s add to `http-probing`.
  - Failed sign-ins trip `http-generic-401-bf` after about 6 quick failures, which is intended.
  - The proposed fix, not applied yet, is a parser `local/ownwave-app-ok` that skips 2xx/3xx responses on `/api/` when the request has an `Authorization` header. Watch for bans during PR 2 and PR 3 testing.
- **Rate limits.** Streams are limited to 60 per minute per IP. Seeks on a cached file are new Range requests, and skipping fast in the car adds more. This is probably fine (⚠ not measured). If it is not, raise the limit for Bearer clients.
- **Exported artwork provider.** Any app on the phone could read cover images through it. This is acceptable for personal use. The stricter option is `grantUriPermission` to the Auto packages only.
- **Play Store.** It is only needed if a head unit blocks sideloaded apps. It needs a privacy policy and Auto review, and is deferred.
