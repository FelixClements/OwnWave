import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState, useSyncExternalStore, type ReactNode } from 'react';
import { api, call } from '@/api/client';
import type { QueueTrack, Station } from '@/api/types';
import { fetchQueue, useFeedback, useHideTrack } from '@/lib/queries';
import { getSettings, streamQuality, useSettings } from '@/lib/settings';
import { PlayerEngine } from './engine';

/** A play counts (history, "recently played last") after this many seconds or half the track. */
const PLAY_COUNTS_AFTER = 30;

export type NowPlaying = {
  station: Pick<Station, 'id' | 'name'> | null;
  track: QueueTrack | null;
  upcoming: readonly QueueTrack[];
  playing: boolean;
  loading: boolean;
  error: string | null;
};

type PlayerValue = NowPlaying & {
  playStation: (station: Pick<Station, 'id' | 'name'>) => Promise<void>;
  toggle: () => void;
  next: () => void;
  previous: () => void;
  seek: (seconds: number) => void;
  like: (track: QueueTrack) => void;
  ban: (track: QueueTrack) => void;
  /** Keep the song off the station that is playing, for this user. */
  hide: (track: QueueTrack) => void;
  liked: (id: string) => boolean;
  expanded: boolean;
  setExpanded: (open: boolean) => void;
};

const PlayerContext = createContext<PlayerValue | null>(null);

/** Position updates (several per second) live outside React state. */
const time = {
  position: 0,
  duration: 0,
  listeners: new Set<() => void>(),
  set(p: number, d: number) {
    this.position = p;
    this.duration = d;
    this.listeners.forEach((l) => l());
  },
};
let timeSnapshot = { position: 0, duration: 0 };
export function usePlaybackTime() {
  return useSyncExternalStore(
    (l) => {
      time.listeners.add(l);
      return () => time.listeners.delete(l);
    },
    () => {
      if (timeSnapshot.position !== time.position || timeSnapshot.duration !== time.duration) {
        timeSnapshot = { position: time.position, duration: time.duration };
      }
      return timeSnapshot;
    },
  );
}

export function streamUrl(trackId: string): string {
  const s = getSettings();
  const q = new URLSearchParams({ quality: streamQuality(s) });
  if (!s.normalize) q.set('normalize', 'false');
  return `/api/stream/${encodeURIComponent(trackId)}?${q}`;
}

export const coverUrl = (trackId: string) => `/api/tracks/${encodeURIComponent(trackId)}/cover`;

export function PlayerProvider({ children }: { children: ReactNode }) {
  const settings = useSettings();
  const feedback = useFeedback();
  const hideTrack = useHideTrack();
  const [state, setState] = useState<NowPlaying>({ station: null, track: null, upcoming: [], playing: false, loading: false, error: null });
  const [likes, setLikes] = useState<Record<string, boolean>>({});
  const [expanded, setExpanded] = useState(false);
  const stationRef = useRef<NowPlaying['station']>(null);
  const counted = useRef<string | null>(null);
  const fetching = useRef(false);

  const engineRef = useRef<PlayerEngine<QueueTrack> | null>(null);
  if (!engineRef.current && typeof Audio !== 'undefined') {
    engineRef.current = new PlayerEngine<QueueTrack>((t) => streamUrl(t.id), {
      onTrack: (track) => {
        counted.current = null;
        time.set(0, track.duration_seconds ?? 0);
        setLikes((l) => (track.id in l ? l : { ...l, [track.id]: track.liked }));
        sync();
      },
      onPlaying: (playing) => setState((s) => (s.playing === playing ? s : { ...s, playing })),
      onTime: (position, duration) => {
        time.set(position, duration);
        const track = engineRef.current?.current;
        if (track && counted.current !== track.id && position >= Math.min(PLAY_COUNTS_AFTER, duration / 2 || PLAY_COUNTS_AFTER)) {
          counted.current = track.id;
          void call(
            api.POST('/api/tracks/{id}/played', { params: { path: { id: track.id } }, body: { station_id: stationRef.current?.id ?? '' } }),
          ).catch(() => {});
        }
      },
      onNeedMore: () => void loadMore(),
      onError: () => {},
    });
  }
  const engine = engineRef.current;

  function sync() {
    const e = engineRef.current;
    if (!e) return;
    setState((s) => ({ ...s, track: e.current, upcoming: e.upcoming.slice(0, 50), playing: e.playing }));
  }

  async function loadMore() {
    const station = stationRef.current;
    if (!station || fetching.current) return;
    fetching.current = true;
    try {
      engineRef.current?.append(await fetchQueue(station.id));
      sync();
    } catch {
      // Retried at the next track change.
    } finally {
      fetching.current = false;
    }
  }

  useEffect(() => {
    engine?.setVolume(settings.volume);
    engine?.setCrossfade(settings.crossfade);
  }, [engine, settings.volume, settings.crossfade]);

  useEffect(() => () => engine?.dispose(), [engine]);

  const playStation = useCallback(
    async (station: Pick<Station, 'id' | 'name'>) => {
      if (!engine) return;
      if (stationRef.current?.id === station.id && engine.current) {
        engine.play();
        return;
      }
      stationRef.current = station;
      setState((s) => ({ ...s, station, loading: true, error: null }));
      try {
        const queue = await fetchQueue(station.id);
        if (stationRef.current?.id !== station.id) return;
        if (!queue.length) {
          setState((s) => ({ ...s, loading: false, error: 'This station has no tracks yet.' }));
          return;
        }
        engine.load(queue, 0, true);
        setState((s) => ({ ...s, loading: false }));
      } catch (e) {
        setState((s) => ({ ...s, loading: false, error: e instanceof Error ? e.message : 'Could not load the station' }));
      }
    },
    [engine],
  );

  const like = useCallback(
    (track: QueueTrack) => {
      const remove = !!likes[track.id];
      setLikes((l) => ({ ...l, [track.id]: !remove }));
      feedback.mutate(
        { id: track.id, feedback: 'like', remove },
        { onError: () => setLikes((l) => ({ ...l, [track.id]: remove })) },
      );
    },
    [likes, feedback],
  );

  const ban = useCallback(
    (track: QueueTrack) => {
      feedback.mutate({ id: track.id, feedback: 'ban' });
      engine?.remove(track.id);
      sync();
    },
    [engine, feedback],
  );

  const stationId = state.station?.id;
  const hide = useCallback(
    (track: QueueTrack) => {
      if (!stationId) return;
      hideTrack.mutate({ stationId, trackId: track.id });
      engine?.remove(track.id);
      sync();
    },
    [engine, hideTrack, stationId],
  );

  // Lock screen, notification and headset controls.
  useEffect(() => {
    if (!('mediaSession' in navigator)) return;
    const ms = navigator.mediaSession;
    const t = state.track;
    if (t) {
      ms.metadata = new MediaMetadata({
        title: t.title,
        artist: t.artist ?? '',
        album: state.station?.name ?? t.album ?? '',
        artwork: [{ src: coverUrl(t.id), sizes: '512x512', type: 'image/jpeg' }],
      });
      document.title = `${t.title}${t.artist ? ` · ${t.artist}` : ''} — OwnWave`;
    } else {
      document.title = 'OwnWave';
    }
    ms.playbackState = state.playing ? 'playing' : 'paused';
  }, [state.track, state.playing, state.station]);

  useEffect(() => {
    if (!engine || !('mediaSession' in navigator)) return;
    const ms = navigator.mediaSession;
    const handlers: [MediaSessionAction, MediaSessionActionHandler][] = [
      ['play', () => engine.play()],
      ['pause', () => engine.pause()],
      ['nexttrack', () => engine.next()],
      ['previoustrack', () => engine.previous()],
      ['seekto', (d) => d.seekTime != null && engine.seek(d.seekTime)],
    ];
    for (const [action, fn] of handlers) {
      try {
        ms.setActionHandler(action, fn);
      } catch {
        // unsupported action
      }
    }
  }, [engine]);

  const value = useMemo<PlayerValue>(
    () => ({
      ...state,
      playStation,
      toggle: () => (engine?.current ? engine.toggle() : state.station && void playStation(state.station)),
      next: () => engine?.next(),
      previous: () => engine?.previous(),
      seek: (s) => engine?.seek(s),
      like,
      ban,
      hide,
      liked: (id) => !!likes[id],
      expanded,
      setExpanded,
    }),
    [state, playStation, engine, like, ban, hide, likes, expanded],
  );

  return <PlayerContext.Provider value={value}>{children}</PlayerContext.Provider>;
}

export function usePlayer(): PlayerValue {
  const v = useContext(PlayerContext);
  if (!v) throw new Error('usePlayer outside PlayerProvider');
  return v;
}
