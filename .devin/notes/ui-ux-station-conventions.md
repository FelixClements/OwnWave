# Station UI/UX Conventions for OwnWave

Research memo on how major music/radio apps and open-source players handle smart radio, generated playlists, artist/album radios, and the player surfaces that support them. Primary sources only: Apple HIG, Material/Android docs, official help pages, and open-source source code.

---

## 1. Card/Cover click vs. Play icon click

### Convention
There are two dominant patterns, but the safer default is to **separate navigation from playback**.

- **Apple Music (web)**: in the Radio tab you “move the pointer over any station, show, or playlist, then click the Play button.” The play button is an explicit control on the card; the rest of the card is for discovery/selection [1].
- **Pandora**: search results lead to a station “backstage page” where you explicitly “Start Station.” Station tuning and modes are then controlled from the Now Playing screen [2, 3].
- **Spotify**: Radio is created from an artist/album/song via an overflow menu (`Go to Radio`) or a “Start Artist Radio” action; the resulting station appears as a playlist-like page with its own play/ shuffle controls [4, 5].
- **YouTube Music**: “Start radio” lives in the track overflow menu or behind a long-press; the Now Playing art also exposes a quick “Start radio” button [6].
- **Music Assistant (open source)**: deliberately separates the two actions — on desktop a play icon reveals on hover, on touch it is always visible on the right of the row; tapping the cover or title always navigates, never plays [7].
- **Bandcamp player (open source)**: the whole radio card is clickable to play; the play overlay has no `onClick` handler, only the card root does [8].
- **Google Play Music (historical)**: added per-card play buttons to half-size album and radio-station cards that “start some tunes” [9].

### Takeaway for OwnWave
- **Default**: card/cover opens the station page or detail; a dedicated play icon/button starts playback.
- **Touch-first**: always show the play button on touch devices; do not rely on hover.
- **Radio-specific safety**: accidental playback is worse for generated stations than for albums, because it changes the current queue. Splitting the affordances is the safest choice [7].

---

## 2. Play/pause state when switching stations

### Convention
Switching a station is a **new playback context**, not a pause/resume of the old one.

- **Apple HIG**: interruptions can be “resumable, like an incoming phone call, or nonresumable, like when people start a new music playlist.” A media playback app should only resume automatically after a resumable interruption [10]. This implies a station change should not auto-resume the previous station.
- **Apple audio session docs**: after an interruption ends, restore state and context and “reactivate audio session, if appropriate for the app.” Don’t resume unless the app was doing so prior to the interruption [11].
- **YouTube Music**: “Starting radio” updates the “Up Next” tab to show the new radio tracks [6].
- **Spotify**: station/playlist changes push a new `spotify:station:{id}` context; AutoPlay is a user preference, not a cross-station resume [4, 12].
- **Jellyfin `PlayQueueManager`**: `SetPlaylist` resets the playing item; the queue is replaced by the new content [13].

### Takeaway for OwnWave
- Selecting a new station should **replace the queue** and transition to the new station; do not keep the old station paused in the background.
- Preserve a “recently played stations” list, not a pause state.
- If the user is already listening and clicks a new station, start playing the new station immediately (or give a clear “Play this station” confirmation).

---

## 3. Where and when a play button should appear

### Convention
Play buttons should be **visible for touch, revealed on hover for pointer, and always keyboard-focusable**.

- **Music Assistant**: on desktop a play icon reveals on hover; on touch `(hover: none)` an explicit play button sits on the right before the ⋮ menu. Tapping cover/title still navigates [7].
- **Angular Spotify (open-source clone of Spotify card)**: `.play-button-overlay` is `opacity: 0` by default and becomes visible on `:hover`; when the card is playing it is always visible [14].
- **Material 3 / Android Compose**: buttons expose hover, focus, pressed, and disabled states. Material components should have hover states for stylus/mouse but those states must not be the only interactive cue [15, 16].
- **WCAG 2.2 / WAI-ARIA**: content shown on hover or focus must be hoverable, dismissible, and persistent. Keyboard focus must always be visible (Focus Visible, SC 2.4.7) [17, 18].
- **Apple HIG (Playing Audio)**: custom audio player controls should only be created if you need commands the system doesn’t support; controls should still be discoverable and reachable [10].

### Takeaway for OwnWave
- **Pointer devices**: show play on hover and on focus; animate it in (e.g. `opacity`/`transform`) and ensure the card also gains a hover/focus outline.
- **Touch devices**: show the play button all the time or on the trailing edge of the card; do not require hover.
- **Currently playing station**: always show a pause/equalizer state on that card, regardless of hover.
- **Keyboard**: the play button must be in the tab order with a visible focus ring; pressing Enter or Space starts/stops the station.

---

## 4. Now Playing / Up Next view vs. the station list

### Convention
The Now Playing view is the source of truth for the current context; the station list is a browsing surface.

- **Apple Music (iOS)**: open the MiniPlayer to see the Now Playing screen, then tap the queue icon to see “Now Playing,” “Up Next,” and “Play Later” sections; AutoPlay can be toggled from the top of the queue [19, 20].
- **Apple Music (web)**: the queue is part of the player UI; “Move the pointer over an item, click the More button, then choose Create Station” and the new station appears under Recently Played [1].
- **YouTube Music**: Up Next lives inside the Now Playing panel; a “Playing from [playlist/album/radio]” header is shown at the top of the queue. The queue is remembered across relaunches and can be synced from mobile [21, 22].
- **Spotify**: the Now Playing view is a context panel on desktop that shows the current track, queue, related info, and Jam/Share controls [23].
- **Pandora**: the Now Playing screen is reached by tapping the current song title in the bottom player; station modes and thumb feedback are managed there [2, 3].
- **Funkwhale**: radios build dynamic playlists and playback is managed through the Queue [24].

### Takeaway for OwnWave
- Use a **persistent Now Playing / Up Next panel** (sidebar or bottom sheet) that shows the current track and a “Playing from [Station Name]” header.
- The station list should show the active station in a selected state.
- Give the queue sections: **Now Playing**, **Up Next**, and (if generated on the fly) **Recommended** / **AutoPlay**.
- Let users tune the station from the queue/Now Playing view rather than from the list alone (e.g. remove/like an upcoming track).

---

## 5. Persistent player bar behavior on station change or stop

### Convention
The player bar is the anchor of the playback context; it updates to the new content and remains visible while a source is active.

- **Apple Music (iOS)**: a MiniPlayer bar at the bottom expands to Now Playing; it persists while anything is queued [19].
- **YouTube Music (web)**: a docked miniplayer stays at the bottom when the full player is closed; it now remembers the last song and queue on relaunch, though playback position resets to the start of the song [21, 22].
- **Android Media3 / Jetpack Compose**: `MiniController` is a compact persistent playback control; the `PlayPauseButton` toggles based on `playWhenReady` and `STATE_READY`/`STATE_BUFFERING`/`STATE_ENDED` [25, 26]. The bar reflects the current `Player` state automatically.
- **Android media controls (system)**: Slot 1 shows Play when `playWhenReady` is false or ended, a loading spinner when buffering, and Pause when ready and playing [27].
- **Pandora**: the currently playing song title at the bottom opens Now Playing; the bar is present whenever a station is active [2].
- **Navidrome (open source)**: `setTrack(await songFromRadio(record))` loads a radio into the shared player; the player bar updates to the new radio stream [28].

### Takeaway for OwnWave
- When a station changes, the player bar should **immediately reflect the new station name/artwork/controls** and start buffering.
- When playback is **paused**, keep the bar visible showing the paused station and track.
- When playback is **stopped** (no active station), either collapse the bar to a dormant “Nothing playing” state or remove it; do not leave stale station metadata.
- On desktop, use a persistent bottom bar; on mobile, use a bottom miniplayer that expands to the full Now Playing view.

---

## 6. Official design guidelines

### Apple
- **HIG — Playing Audio**: choose the right audio category, respond to remote controls sensibly, and decide whether to resume after interruptions [10].
- **Audio Session Programming Guide**: save state, update UI on interruption, and only resume when the interruption type is resumable [11].
- **Apple Music User Guides**: Radio lives in a dedicated tab; queue has sections for Now Playing, Up Next, Play Later, and AutoPlay [1, 19, 20].

### Google / Material / Android
- **Material 3 — Cards & Buttons**: cards present content and actions; buttons expose hover, focus, and pressed states; do not rely on hover as the sole cue [15, 29].
- **Jetpack Media3 Compose UI**: provides `PlayPauseButton`, `MiniController`, `Player`, and state-bound controls [25, 26].
- **Android Media Controls (mobile)**: system surfaces derive Play/Pause/Previous/Next from the `Player` state and `PlaybackState` actions [27].

### Accessibility
- **W3C WCAG 2.2**: focus must be visible; hover/focus content must be hoverable, dismissible, and persistent [17, 18].

---

## Open-source implementation patterns

| Project | Relevant pattern | Source |
|---|---|---|
| **Music Assistant** | Play icon on hover (desktop) / always visible on touch; cover/title navigates only | [7] |
| **Bandcamp player** | Whole radio card root `onClick` calls `playRadioStation()` | [8] |
| **Angular Spotify clone** | `.play-button-overlay` revealed on `:hover`; visible when `is-playing` | [14] |
| **Jellyfin** | `PlayQueueManager.SetPlaylist` resets playing item; `PlaybackManager` routes queue to active player | [13] |
| **Navidrome** | `handleRowClick` dispatches `setTrack(await songFromRadio(record))` for radio rows | [28] |
| **Funkwhale** | Radios are dynamic playlists; playback uses the Queue | [24] |
| **Home Assistant (media browser)** | Play button in media browser reveals on hover and is styled as a FAB | [30] |

---

## Actionable recommendations for OwnWave

1. **Station cards** open a detail page; the play icon starts playback. On touch, always show the play icon; on pointer, show it on hover/focus and for the currently playing station.
2. **Clicking a new station replaces the queue** and begins playback of that station. Don’t maintain a pause state for the previously active station.
3. **Use a persistent player bar** that updates with the new track/artwork/station name when switching; keep it visible while paused, collapse it on explicit stop.
4. **Expose Now Playing / Up Next** as a dedicated panel. Show a “Playing from [Station]” header and allow tuning/removing upcoming tracks.
5. **Make play controls keyboard-accessible** and never rely solely on hover; respect `prefers-reduced-motion` and `(hover: none)` media queries.
6. **Follow Material 3 state guidance** and Android Media3 `Player`/`MiniController` idioms for the player bar; follow Apple HIG for interruption handling and remote-control behavior.

---

## References

[1] Apple, *Listen to radio stations in Music on the web*, https://support.apple.com/en-ca/guide/music-web/apdmdb465399/web  
[2] Pandora, *Create, edit and delete stations*, https://help.pandora.com/s/article/Create-a-Station-1519949296261  
[3] Pandora, *Pandora Modes*, https://help.pandora.com/s/article/Pandora-Modes?language=en_US  
[4] Spotify, *Spotify Radio*, https://support.spotify.com/us/article/spotify-radio/  
[5] CNET, *Spotify Radio conducts music with Pandora-like app*, https://www.cnet.com/culture/spotify-radio-conducts-music-with-pandora-like-app/  
[6] Android Police, *YouTube Music makes it easier to “start radio” from your currently playing song*, https://www.androidpolice.com/2021/02/02/youtube-music-makes-it-easier-to-start-radio-from-your-currently-playing-song/  
[7] Music Assistant, *“Unify list row layout and refine play affordances” PR #1862*, https://github.com/music-assistant/frontend/pull/1862  
[8] eremef/bandcamp-player, *GEMINI.md — Radio Card Playback*, https://github.com/eremef/bandcamp-player/blob/main/GEMINI.md  
[9] Android Police, *Play Music v6.11 adds play buttons to album and radio station cards*, https://www.androidpolice.com/2016/07/15/play-music-v6-11-adds-play-buttons-album-radio-station-cards-prepares-add-sleep-timer-apk-teardown-download/  
[10] Apple, *HIG — Playing audio*, https://developer.apple.com/design/human-interface-guidelines/playing-audio  
[11] Apple, *Audio Session Programming Guide — Responding to Interruptions*, https://developer.apple.com/library/archive/documentation/Audio/Conceptual/AudioSessionProgrammingGuide/HandlingAudioInterruptions/HandlingAudioInterruptions.html  
[12] Spotify, *Autoplay tracks*, https://support.spotify.com/uk/article/autoplay/  
[13] Jellyfin, *PlayQueueManager.cs*, https://github.com/jellyfin/jellyfin/blob/9239b121/MediaBrowser.Controller/SyncPlay/Queue/PlayQueueManager.cs  
[14] trungvose/angular-spotify, *card.component.scss*, https://github.com/trungvose/angular-spotify/blob/f8721f70/libs/web/shared/ui/media/src/lib/card.component.scss  
[15] Material 3, *States — Applying states*, https://m3.material.io/foundations/interaction/states/applying-states  
[16] Android Developers, *Enrich stylus and mouse experiences with hover*, https://medium.com/androiddevelopers/enrich-stylus-and-mouse-experiences-with-hover-9db19320bf56  
[17] W3C, *WCAG 2.2 — Focus Visible (SC 2.4.7)*, https://www.w3.org/WAI/WCAG22/Understanding/focus-visible  
[18] W3C, *SCR39: Making content on focus or hover hoverable, dismissible, and persistent*, https://www.w3.org/WAI/WCAG22/Techniques/client-side-script/SCR39.html  
[19] Apple, *iPhone User Guide — Queue up your music*, https://support.apple.com/guide/iphone/queue-up-your-music-ipha4521ef7d/ios  
[20] Apple, *iPhone User Guide — Listen to radio stations*, https://support.apple.com/en-gb/guide/iphone/iph0fc7cdac1/ios  
[21] 9to5Google, *YouTube Music web app now remembers your last song and queue on relaunch*, https://9to5google.com/2024/05/31/youtube-music-web-remembers/  
[22] YouTube Music Help, *Listen to personal or custom mix*, https://support.google.com/youtubemusic/answer/15165061?hl=en  
[23] TechBloat, *Major UI Makeover for Spotify on Desktop*, https://www.techbloat.com/major-ui-makeover-for-spotify-on-desktop.html  
[24] Funkwhale, *Radios*, https://docs.funkwhale.audio/stable/user/radios/index.html  
[25] Android Developers, *Getting started with Compose-based UI — Media3*, https://developer.android.com/media/media3/ui/compose  
[26] Android Developers, *PlayerDefaults API reference*, https://developer.android.com/reference/kotlin/androidx/media3/ui/compose/material3/PlayerDefaults  
[27] Android Developers, *Media controls — Mobile*, https://developer.android.com/media/implement/surfaces/mobile  
[28] Navidrome, *RadioList.jsx*, https://github.com/navidrome/navidrome/blob/220019a9/ui/src/radio/RadioList.jsx  
[29] Material 3, *Buttons — Guidelines*, https://m3.material.io/components/buttons/guidelines  
[30] Home Assistant, *“Make media play button in media browser more visible when hovering” PR #23760*, https://github.com/home-assistant/frontend/pull/23760  
