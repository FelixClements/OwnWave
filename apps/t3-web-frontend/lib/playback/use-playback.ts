import { useCallback, useEffect, useRef } from 'react';
import { createPlaybackEngine } from './create-engine';
import { createApiStreamResolver } from './stream-resolver';
import type { PlaybackEngine, PlaybackTrack, StreamFormat } from './types';

type UsePlaybackOptions<T extends PlaybackTrack> = {
  /** Station being played; changing it loads that station's queue. */
  stationId: string | null;
  queue: T[];
  isPlaying: boolean;
  format: StreamFormat;
  bitrate: string;
  onTrackChange: (track: T, index: number) => void;
  /** Fetch the next page of the station's queue once the current one ends. */
  fetchNextQueue: () => Promise<T[]>;
};

/**
 * Drives a playback engine from React state. The engine owns its copy of the
 * queue: later queue updates (optimistic likes/bans) don't restart playback.
 */
export function usePlayback<T extends PlaybackTrack>({
  stationId,
  queue,
  isPlaying,
  format,
  bitrate,
  onTrackChange,
  fetchNextQueue,
}: UsePlaybackOptions<T>) {
  const audioARef = useRef<HTMLAudioElement | null>(null);
  const audioBRef = useRef<HTMLAudioElement | null>(null);
  const engineRef = useRef<PlaybackEngine | null>(null);
  const loadedStationRef = useRef<string | null>(null);
  const loadingRef = useRef(false);

  const onTrackChangeRef = useRef(onTrackChange);
  onTrackChangeRef.current = onTrackChange;
  const fetchNextQueueRef = useRef(fetchNextQueue);
  fetchNextQueueRef.current = fetchNextQueue;

  const getEngine = useCallback(() => {
    if (!engineRef.current && audioARef.current && audioBRef.current) {
      const engine: PlaybackEngine = createPlaybackEngine(
        createApiStreamResolver(),
        {
          onTrackChange: (track, index) => onTrackChangeRef.current(track as T, index),
          onQueueEnd: () => {
            void fetchNextQueueRef
              .current()
              .then((next) => (next.length ? engine.load(next, 0) : undefined))
              .catch(() => {});
          },
        },
        [audioARef.current, audioBRef.current],
      );
      engineRef.current = engine;
    }
    return engineRef.current;
  }, []);

  useEffect(() => {
    getEngine()?.setStreamOptions({ format, bitrate });
  }, [format, bitrate, getEngine]);

  useEffect(() => {
    return () => {
      engineRef.current?.dispose();
      engineRef.current = null;
    };
  }, []);

  useEffect(() => {
    const engine = getEngine();
    if (!engine) return;
    if (!isPlaying) {
      engine.pause();
      return;
    }
    if (loadingRef.current || !stationId || !queue.length) return;
    if (loadedStationRef.current === stationId) {
      void engine.play().catch(() => {});
      return;
    }

    loadingRef.current = true;
    loadedStationRef.current = stationId;
    void engine
      .load(queue, 0)
      .catch(() => {})
      .finally(() => {
        loadingRef.current = false;
      });
  }, [isPlaying, stationId, queue, getEngine]);

  const skipTo = useCallback((index: number) => void engineRef.current?.skipTo(index), []);

  const skipNext = useCallback(() => {
    const engine = engineRef.current;
    if (!engine || engine.isCrossfading()) return;
    void engine.skipTo(engine.getCurrentIndex() + 1);
  }, []);

  const skipPrev = useCallback(() => {
    const engine = engineRef.current;
    if (!engine || engine.isCrossfading()) return;
    void engine.skipTo(Math.max(0, engine.getCurrentIndex() - 1));
  }, []);

  const removeTrack = useCallback((id: string) => void engineRef.current?.removeTrack(id), []);

  return {
    audioARef,
    audioBRef,
    skipNext,
    skipPrev,
    skipTo,
    removeTrack,
    isCrossfading: () => engineRef.current?.isCrossfading() ?? false,
  };
}
