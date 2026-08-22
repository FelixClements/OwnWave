import { useCallback, useEffect, useRef } from 'react';
import { createPlaybackEngine } from './create-engine';
import { pinQueue } from './queue-pin';
import { createApiStreamResolver } from './stream-resolver';
import type { PlaybackTrack, StreamFormat } from './types';
import { WebAudioCrossfadeEngine } from './web-audio-engine';
import { SequentialAudioEngine } from './sequential-engine';

type UsePlaybackOptions<T extends PlaybackTrack> = {
  queue: T[];
  selectedStation: string | null;
  playingStation: string | null;
  isPlaying: boolean;
  format: StreamFormat;
  bitrate: string;
  nowPlaying: T | null;
  onTrackChange: (track: T, index: number) => void;
};

export function usePlayback<T extends PlaybackTrack>({
  queue: queueProp,
  selectedStation,
  playingStation,
  isPlaying,
  format,
  bitrate,
  nowPlaying,
  onTrackChange,
}: UsePlaybackOptions<T>) {
  const audioARef = useRef<HTMLAudioElement | null>(null);
  const audioBRef = useRef<HTMLAudioElement | null>(null);
  const pinnedQueueRef = useRef(queueProp);
  const loadedStationRef = useRef<string | null>(null);
  const loadingRef = useRef(false);
  const engineRef = useRef<ReturnType<typeof createPlaybackEngine> | null>(null);
  const resolverRef = useRef(createApiStreamResolver());
  const audibleQueueRef = useRef(queueProp);
  const nowPlayingRef = useRef(nowPlaying);
  nowPlayingRef.current = nowPlaying;

  const queue = pinQueue(
    queueProp,
    selectedStation,
    playingStation,
    pinnedQueueRef.current,
  );
  pinnedQueueRef.current = queue;
  audibleQueueRef.current = queue;

  const onTrackChangeRef = useRef(onTrackChange);
  onTrackChangeRef.current = onTrackChange;

  const ensureEngine = useCallback(() => {
    if (!engineRef.current) {
      engineRef.current = createPlaybackEngine(resolverRef.current, {
        onTrackChange: (track, index) => {
          const audible = audibleQueueRef.current;
          const fullTrack =
            audible.find((q) => q.id === track.id) ?? audible[index];
          if (!fullTrack) return;
          onTrackChangeRef.current(fullTrack, index);
        },
      });
    }
    const engine = engineRef.current;
    engine.setStreamOptions({ format, bitrate });
    if (engine instanceof WebAudioCrossfadeEngine && audioARef.current && audioBRef.current) {
      engine.attach([audioARef.current, audioBRef.current]);
    } else if (engine instanceof SequentialAudioEngine && audioARef.current) {
      engine.attach(audioARef.current);
    }
    return engine;
  }, [format, bitrate]);

  useEffect(() => {
    return () => {
      engineRef.current?.dispose();
      engineRef.current = null;
    };
  }, []);

  useEffect(() => {
    if (typeof window === 'undefined') return;
    if (!isPlaying) {
      engineRef.current?.pause();
      return;
    }
    if (loadingRef.current || !queue.length || !playingStation) return;

    if (playingStation === loadedStationRef.current && nowPlayingRef.current) {
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
  }, [isPlaying, playingStation, queue, ensureEngine]);

  useEffect(() => {
    ensureEngine();
  }, [format, bitrate, ensureEngine]);

  const skipTo = useCallback(
    (index: number) => {
      void ensureEngine().skipTo(index);
    },
    [ensureEngine],
  );

  const skipNext = useCallback(() => {
    const engine = engineRef.current;
    if (!engine || engine.isCrossfading()) return;
    void engine.skipTo(engine.getCurrentIndex() + 1);
  }, []);

  const skipPrev = useCallback(() => {
    const engine = engineRef.current;
    if (!engine || engine.isCrossfading()) return;
    void engine.skipTo(engine.getCurrentIndex() - 1);
  }, []);

  return {
    audioARef,
    audioBRef,
    queue,
    skipNext,
    skipPrev,
    skipTo,
    getCurrentIndex: () => engineRef.current?.getCurrentIndex() ?? 0,
    isCrossfading: () => engineRef.current?.isCrossfading() ?? false,
  };
}
