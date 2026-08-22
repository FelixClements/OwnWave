# Research: Station switch stops playback

**Date:** 2026-08-22  
**Question:** When a song is playing and the user clicks another station in the sidebar, playback stops. Why?

Primary sources: `apps/t3-web-frontend` playback/station code, commit `f772de5`, and the CrossfadeEngine refactor (`lib/playback/`).

---

## Intended behavior

OwnWave deliberately separates **browsing** from **playback**:

| State | Meaning |
|-------|---------|
| `selectedStation` | Station shown in the UI / queue panel |
| `playingStation` | Station whose audio queue is actually playing |

Commit `f772de5` (*"Only switch playback when play is explicitly pressed"*) established that **clicking a station in the sidebar must not stop audio**. The sidebar handler only updates selection:

```49:49:apps/t3-web-frontend/components/Shell.tsx
                  onClick={() => setSelectedStation(station.id)}
```

Playback only switches when the user presses **Play** on the station page (`app/page.tsx:288`), which calls `setPlayingStation(selectedStation)`.

---

## What happens on a sidebar click

1. **`setSelectedStation(B)`** updates React state and rewrites the URL to `/?station=B` (`lib/station.tsx:45-54`).
2. **`StationProvider` fetches the queue for the selected station**, not the playing one:

```33:36:apps/t3-web-frontend/lib/station.tsx
  const { data: queue } = trpc.queue.useQuery(
    { id: selectedStation || '' },
    { enabled: !!selectedStation, refetchOnWindowFocus: false, refetchInterval: false }
  );
```

3. Until the new query resolves, context exposes `queue: []` (`queue || []` at line 74).
4. **`Shell` passes that context queue into `Player`** (`Shell.tsx:85`).
5. **`usePlayback` is supposed to pin the audible queue** while `playingStation !== selectedStation`:

```1:12:apps/t3-web-frontend/lib/playback/queue-pin.ts
/** Keep the audible queue pinned while browsing another station. */
export function pinQueue<T>(
  incoming: T[],
  selectedStation: string | null,
  playingStation: string | null,
  pinned: T[],
): T[] {
  if (playingStation === null || playingStation === selectedStation) {
    return incoming;
  }
  return pinned;
}
```

If pinning works, the playback effect should see the **same pinned array reference** as before the click and should not restart audio.

---

## Root cause: the playback effect restarts (or pauses) when it re-runs

All pause/restart paths go through `usePlayback`:

```72:95:apps/t3-web-frontend/lib/playback/use-playback.ts
  useEffect(() => {
    if (typeof window === 'undefined') return;
    if (!isPlaying) {
      engineRef.current?.pause();
      return;
    }
    if (loadingRef.current || !queue.length || !playingStation) return;

    if (playingStation === loadedStationRef.current && nowPlaying) {
      void engineRef.current?.play();
      return;
    }

    loadingRef.current = true;
    const engine = ensureEngine();
    void engine
      .load(queue, 0)
      .then(() => engine.play())
      .catch(() => {})
      .finally(() => {
        loadingRef.current = false;
        loadedStationRef.current = playingStation;
      });
  }, [isPlaying, playingStation, queue, ensureEngine, nowPlaying]);
```

### Mechanism A — full reload interrupts audio (most likely)

When the effect re-runs and the resume guard on **line 80** fails, it calls `engine.load(queue, 0)`.

`WebAudioCrossfadeEngine.load()` always tears down crossfade state and starts deck A at index 0:

```48:57:apps/t3-web-frontend/lib/playback/web-audio-engine.ts
  async load(queue: PlaybackTrack[], startIndex = 0) {
    this.queue = queue;
    this.currentIndex = startIndex;
    this.crossfading = false;
    if (this.crossfadeTimer) {
      clearTimeout(this.crossfadeTimer);
      this.crossfadeTimer = null;
    }
    await this.loadAndPlay(startIndex, 'A');
  }
```

That interrupts in-progress playback — perceived as “music stopped” or jumped to the beginning.

The guard fails when **`nowPlaying` is falsy** or **`loadedStationRef !== playingStation`**.

### Mechanism B — explicit pause

If `isPlaying` becomes `false`, line 75 calls `engine.pause()`. Nothing in the sidebar click path sets `isPlaying(false)`, so this is only relevant if something else toggles play state (station page Pause, Media Session pause, station delete in `StationManager.tsx:79`).

### Mechanism C — engine disposal on unmount

The hook disposes the engine on unmount (`use-playback.ts:65-69`). `Player` lives in `Shell` and does not remount on `?station=` changes, so this is unlikely for a normal sidebar click.

---

## Why the effect re-runs after a station click

The effect lists **`queue` and `nowPlaying` as React dependencies**. The pre-refactor `Player.tsx` (commit `f772de5`) used the same `queue` ref-pinning idea but **did not** include `nowPlaying` in the effect dependency array — it only read `nowPlaying` inside the effect body.

### 1. Queue pin bypass corrupts the pinned ref

`pinQueue` returns the **incoming** (selected-station) queue when `playingStation === null` (line 8).

If `playingStation` is ever `null` while audio is active:

- The first fetch for station B yields `incoming = []`.
- `pinnedQueueRef` is overwritten with `[]` (`use-playback.ts:44`).
- The effect hits `!queue.length` and returns early (line 78) without calling `play()`.
- When `playingStation` is later set, or when a subsequent render unlocks pinning, the effect may call `load()` with the wrong/empty queue.

`playingStation` is normally set by the station page Play button (`page.tsx:288`) before playback starts, but `handleTrackChange` can also set it from `playingStation ?? selectedStation` (`Player.tsx:67-73`). If `playingStation` is still `null` when a track-change fires after the user has already selected another station, it can set `playingStation` to the **browsed** station instead of the audible one.

### 2. `handleTrackChange` resolves tracks from the selected-station queue, not the pinned queue

```61:76:apps/t3-web-frontend/components/Player.tsx
  const handleTrackChange = useCallback(
    (track: { id: string }, index: number) => {
      const fullTrack =
        queueProp.find((q) => q.id === track.id) ??
        queueProp[index];
      if (!fullTrack) return;
      ...
      setNowPlaying(fullTrack);
```

After switching to station B while station A is playing:

- `queueProp` is B’s queue.
- The engine still plays a track from A’s pinned queue.
- `queueProp.find(track.id)` misses → fallback `queueProp[index]` is wrong or `undefined`.
- If `!fullTrack`, the callback returns **without calling `setNowPlaying`**.
- `nowPlaying` stays `null` even though audio is playing.
- Any later effect re-run (from `queue` or `ensureEngine` changing) fails the line-80 guard and calls `engine.load(queue, 0)` → **playback stops/restarts**.

This is especially likely if the user switches station quickly after pressing Play, or on the first crossfade after switching.

### 3. URL ↔ state sync can briefly unlock queue pinning

`StationProvider` mirrors `?station=` from the URL:

```38:43:apps/t3-web-frontend/lib/station.tsx
  useEffect(() => {
    const id = searchParams?.get('station') ?? null;
    if (id !== selectedStation) {
      setSelectedState(id);
    }
  }, [searchParams, selectedStation]);
```

Sidebar click order: `setSelectedState(B)` then `router.replace(?station=B)`.

Until the router commits, `searchParams` may still read `A` while `selectedStation` is `B`. The effect above can **snap selection back to A**, then forward to B when the URL updates. During a render where `playingStation === selectedStation`, `pinQueue` follows the **incoming** context queue instead of the pinned one. That can change the `queue` dependency reference and re-enter the playback effect.

### 4. Context queue flashes empty (no `placeholderData`)

There is no `keepPreviousData` / `placeholderData` on the queue query. On station switch the context queue becomes `[]` until B loads (`station.tsx:74`).

Pinning should mask this inside `usePlayback`, but combined with (1) or (3) the empty array can still poison `pinnedQueueRef`.

---

## Regression introduced by the CrossfadeEngine refactor

| Before (`Player.tsx` + `queueRef`) | After (`usePlayback` + `pinQueue`) |
|-----------------------------------|-------------------------------------|
| Queue pin updated in `useEffect` after render | Queue pin computed synchronously each render |
| Playback effect deps: `[isPlaying, playingStation, queue]` | Adds **`nowPlaying`** and **`ensureEngine`** |
| Track metadata resolved from `queueRef.current` inside `loadTrack` | `handleTrackChange` resolves from **`queueProp`** (selected station) |

The pin logic is equivalent in the happy path, but the new effect dependencies and the `queueProp`/`nowPlaying` split make it easier for a station switch to trigger a reload or leave `nowPlaying` unset while audio runs.

---

## Summary

**Observed:** Clicking another station stops playback.  
**Expected:** Sidebar click only changes `selectedStation`; audio keeps playing until Play is pressed on the new station (`f772de5`, `Shell.tsx:49`).  
**Reason:** The `usePlayback` effect treats a station switch as a signal to re-sync playback. When its resume guard fails — most often because `nowPlaying` was not updated (track lookup uses the browsed station’s `queueProp`) or because queue pinning is briefly bypassed — it calls `engine.load(queue, 0)`, which interrupts the crossfade engine and stops the current song.

---

## References

| Source | Location |
|--------|----------|
| Sidebar only selects station | `apps/t3-web-frontend/components/Shell.tsx:49` |
| Queue fetched for selected station | `apps/t3-web-frontend/lib/station.tsx:33-36` |
| Queue pin while browsing | `apps/t3-web-frontend/lib/playback/queue-pin.ts` |
| Playback effect + reload | `apps/t3-web-frontend/lib/playback/use-playback.ts:72-95` |
| Engine load restarts at index 0 | `apps/t3-web-frontend/lib/playback/web-audio-engine.ts:48-57` |
| Track change uses selected queue | `apps/t3-web-frontend/components/Player.tsx:61-76` |
| Explicit play switches playing station | `apps/t3-web-frontend/app/page.tsx:284-290` |
| Original browse-without-stop behavior | git commit `f772de55` |

---

## Suggested fix direction (not implemented)

1. Resolve `fullTrack` from the **pinned** queue inside `usePlayback` / `Player`, not `queueProp`.
2. Remove `nowPlaying` from the playback effect deps; use a ref for the resume guard (match pre-refactor behavior).
3. Harden `pinQueue`: never overwrite the pinned ref with an empty incoming queue while `isPlaying && playingStation`.
4. Add `placeholderData: (prev) => prev` (or pin the playing-station queue in context) so the selected-station fetch cannot flash `[]` into consumers.
5. Optionally fetch/cache the **playing** station queue separately from the **selected** station queue.
