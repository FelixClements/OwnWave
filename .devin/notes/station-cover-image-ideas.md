# Station / Radio Playlist Cover-Image Ideas for OwnWave

Research memo on replacing the plain green station placeholders with covers that match music-app conventions. Primary sources: official platform artist docs, design systems (Material 3, Apple HIG/Human Interface guidelines), open-source component libraries, and the Apple Music API format.

---

## 1. Recommended cover sizes, aspect ratios and formats

Music platforms treat playlist/station cover art as a **square 1:1 image**.

- **Spotify**: cover art must be 1:1, between 640 px and 10 000 px, in TIFF/PNG/JPG, sRGB color space, 24 bits per pixel ([Spotify Cover Art Requirements](https://support.spotify.com/us/artists/article/cover-art-requirements/)). The playlist API returns images up to 300×300 px in practice ([Spotify Get Playlist Cover](https://developer.spotify.com/documentation/web-api/reference/get-playlist-cover)).
- **Apple Music (Curator)**: 3000×3000 px minimum, 72 dpi, RGB PNG with no transparency ([Apple Music Curator Best Practices](https://help.apple.com/itc/musiccuratorbestpractices/en.lproj/static.html)).
- **Apple Music (Album)**: perfect square, at least 4000×4000 px, JPG/PNG/GIF ([Apple Music for Artists — Cover Art](https://artists.apple.com/support/1120-cover-art)).
- **YouTube Music / Art Tracks**: square PNG or JPEG, max 4098×4098 px, recommended minimum 1400×1400 px at 300 DPI ([YouTube Help — File Format for Resources](https://support.google.com/youtube/answer/3506725?hl=en-GB)).
- **Material 3 Image List**: the open-source `mdc-image-list` constrains standard images to 1:1 by default, with an `aspect` mixin to override it ([`@material/image-list` on npm](https://www.npmjs.com/package/@material/image-list)). MUI’s `ImageList` also defaults to a uniform size/ratio grid ([MUI Image List](https://mui.com/material-ui/react-image-list/)).

---

## 2. Default / fallback patterns when no cover exists

- **Apple Music**: if a curator playlist has no uploaded cover, Apple displays a 2×2 grid of album covers from the playlist; the docs explicitly ask curators not to manually recreate that 2×2 grid ([Apple Music Curator Best Practices](https://help.apple.com/itc/musiccuratorbestpractices/en.lproj/static.html)).
- **MusicKit / Apple Music API**: `Artwork` objects carry an average `bgColor` (background color) plus `textColor1…textColor4`, and `ArtworkImage` uses `backgroundColor` as a placeholder while the image loads ([Exploring MusicKit — ArtworkImage](https://exploringmusickit.com/musickit-artwork-image)). Station payloads expose the same color fields, e.g. `bgColor: "300a78"` for a Discovery Station ([Exploring MusicKit — Discovery Station](https://exploringmusickit.com/musickit-discovery-station)).
- **Spotify user playlists**: the default cover is commonly a 2×2 collage of the first four unique album arts (observed behavior, consistent with the platform’s playlist image API returning the existing cover or grid). The Web API lets clients upload a custom JPEG as a base64 payload ≤ 256 KB ([Spotify Upload Custom Playlist Cover](https://developer.spotify.com/documentation/web-api/reference/upload-custom-playlist-cover)).
- **YouTube Music UI** (reported in tech press): defaults to the first track’s album art or a 2×2 grid, and has added AI-generated theme covers and custom thumbnail uploads ([9to5Google](https://9to5google.com/2023/10/24/youtube-music-ai-art-playlist/), [Android Police](https://www.androidpolice.com/youtube-music-custom-playlist-thumbnails/)).
- **Material UI fallback chain**: for avatars/images, MUI falls back through `children` → first letter of `alt` → a generic icon. This is a useful pattern for any placeholder cover that may have a name but no art ([MUI Avatar](https://mui.com/material-ui/react-avatar/)).

---

## 3. Color, typography and iconography for generated / placeholder covers

- **Represent the station in a single glance**: Apple asks curators to make cover art representative of the playlist’s title/description and the listening experience, with legible text at all the sizes the art will appear ([Apple Music Curator Best Practices](https://help.apple.com/itc/musiccuratorbestpractices/en.lproj/static.html)).
- **Simple, single-color or gradient background**: Apple’s Brand Logo guidelines call for a simple, single-color background and one main graphic element; avoid excessive effects ([Apple Music Curator Best Practices — Brand Logo](https://help.apple.com/itc/musiccuratorbestpractices/en.lproj/static.html)).
- **Material 3 color roles**: use `surface` and `on-surface` tokens (or `primaryContainer`/`onPrimaryContainer`) so foreground text/icon automatically contrasts with the background. M3 derives color roles from a source color with HCT (hue/chroma/tone) to keep pairs accessible ([Android Color for Mobile Design](https://developer.android.com/design/ui/mobile/guides/styles/color)).
- **Deterministic color from station name/ID**: hash the station name or genre to a seed color, then sample a container color and its `on-container` text color from a theme. This keeps the same station visually stable across sessions.
- **Icon + initials**: for a text-free placeholder use a single centered icon from [Material Symbols](https://fonts.google.com/icons?icon.set=Material+Symbols) such as `art_track`, `radio`, `album`, or `music_note`; if no icon is meaningful, show one or two initials of the station name on the colored background. Keep the glyph large (30–40% of the thumbnail) and use a contrasting `on-surface` color.
- **Typography**: use a bold sans-serif (e.g. Inter, Roboto or Noto) and keep text to a very short label/initials so it remains legible at 48–64 px. Avoid overly thin weights and rely on `on-surface` tokens rather than overlaying arbitrary photos with text.

---

## 4. Accessibility and contrast

- **WCAG 2.2**: normal text needs a contrast ratio of at least 4.5:1 against its background; large text (≥18 pt or 14 pt bold) needs 3:1; non-text UI elements need 3:1 ([W3C — Contrast Minimum](https://www.w3.org/WAI/WCAG22/Understanding/contrast-minimum)).
- **Apple HIG / App Store Connect**: recommends the same 4.5:1 minimum for foreground/background text and also 3:1 for non-text state contrasts, with testing in both Light and Dark Modes and with Increase Contrast on ([Apple — Sufficient Contrast](https://developer.apple.com/help/app-store-connect/manage-app-accessibility/sufficient-contrast-accessibility-evaluation-criteria)).
- **Material 3**: color roles are designed so container/on-container combinations meet contrast expectations, and the HCT tonal model can be used to check accessibility ([Material 3 Color Contrast Codelab](https://codelabs.developers.google.com/color-contrast-accessibility)).
- **Practical cover rule**: do not place station titles directly on busy photographic backgrounds; if text must sit on an image, apply a gradient or solid scrim and verify contrast with a checker such as the Material Theme Builder or `apca-w3`.

---

## 5. How the major apps handle playlist / station covers

| Platform | Cover handling | Source |
|---|---|---|
| Apple Music (Curator/Playlist) | Custom 3000×3000 PNG; if missing, a 2×2 album grid | [Curator Best Practices](https://help.apple.com/itc/musiccuratorbestpractices/en.lproj/static.html) |
| Apple Music (Radio/Station API) | `artwork` with `bgColor` + `textColor1-4` for color-matched UI and placeholders | [MusicKit Discovery Station example](https://exploringmusickit.com/musickit-discovery-station) |
| Spotify | 1:1 playlist image via API; default 2×2 collage; custom upload as base64 JPEG ≤ 256 KB | [Get Playlist Cover](https://developer.spotify.com/documentation/web-api/reference/get-playlist-cover), [Upload Custom Cover](https://developer.spotify.com/documentation/web-api/reference/upload-custom-playlist-cover) |
| YouTube Music | Partner Art Track specs are 1:1 1400–4098 px. In the consumer UI the default is a 2×2 track grid and recent builds add AI-generated and user-uploaded custom covers. | [YouTube Help — File Format](https://support.google.com/youtube/answer/3506725?hl=en-GB), [9to5Google](https://9to5google.com/2023/10/24/youtube-music-ai-art-playlist/), [Android Police](https://www.androidpolice.com/youtube-music-custom-playlist-thumbnails/) |

---

## 6. Tools and libraries for generating placeholder or cover art

| Approach | Options | Notes |
|---|---|---|
| **CSS / SVG generative patterns** | `geopattern` ([GitHub](https://github.com/btmills/geopattern)), `trianglify` ([GitHub](https://github.com/qrohling/Trianglify)), CSS `conic-gradient`/`linear-gradient` | Deterministic, no network dependency, can be seeded from station name |
| **Dominant-color extraction** | `node-vibrant` ([GitHub](https://github.com/Vibrant-Colors/node-vibrant)), `color-thief` ([GitHub](https://github.com/lokesh/color-thief)) | Pulls palette from the first track’s album art to tint the placeholder |
| **AI image generation APIs** | OpenAI Images API / DALL·E 3 ([docs](https://platform.openai.com/docs/api-reference/images)), Stability AI ([docs](https://platform.stability.ai/)), Replicate ([site](https://replicate.com/)) | Good for user-generated or "vibe" covers; adds latency and cost |
| **Placeholder image services** | Picsum ([picsum.photos](https://picsum.photos/)) | Useful for development/demo grids; not a brand-safe music-art strategy |
| **Color / theme tooling** | Material Theme Builder ([site](https://material-foundation.github.io/material-theme-builder/)) | Generates accessible M3 tokens from a seed color |

---

## 7. Recommendations for OwnWave

1. **Adopt a 1:1 square cover for stations in lists and a larger hero for the detail view.** Store source art at **1400×1400 px or higher** so it is crisp at thumbnail (48–64 px), list (120–200 px) and detail (300–640 px) sizes.
2. **Replace the solid green placeholder with a layered fallback chain:**
   1. User-uploaded / curator-provided cover
   2. Auto-generated 2×2 album-art grid from the station’s first tracks
   3. Deterministic **gradient or pattern + icon + station initials**, colored by genre/station seed
   4. Neutral `surface` color + generic music icon
3. **Use Material 3 color tokens** (`surface`/`on-surface`, `primaryContainer`/`onPrimaryContainer`) or extracted `bgColor`/`textColor` fields so every placeholder is contrast-safe in light and dark themes.
4. **Do not burn text into cover images**; overlay station labels as live text with a subtle scrim if needed, so text remains accessible and localizable.
5. **Add `alt` text** for every cover (e.g. "Cover for Focus Flow station") and ensure the fallback icon is not the only way to identify a station.
6. **Consider a wide banner crop for station hero surfaces** (Apple Music station artworks can be 4320×1080). Keep a 1:1 thumbnail for grids and a separate 4:1–16:9 hero asset if the design allows.
7. **For a quick first pass**, implement the deterministic gradient/initial/icon placeholder. It is cheap, no-network, accessible and immediately more visually informative than a uniform green square.
