'use client';

import { createContext, useCallback, useContext, useEffect, useMemo, useState } from 'react';
import { usePathname, useRouter, useSearchParams } from 'next/navigation';
import type { QueueTrack, Station } from '@/lib/api';
import type { StreamFormat, StreamOptions } from '@/lib/playback/types';
import { useQueue, useStations } from '@/lib/queries';

interface StationContextValue {
  selectedStation: string | null;
  setSelectedStation: (id: string | null) => void;
  stations: Station[];
  queue: QueueTrack[];
  nowPlaying: QueueTrack | null;
  setNowPlaying: (track: QueueTrack | null) => void;
  playingStation: string | null;
  setPlayingStation: (id: string | null) => void;
  isPlaying: boolean;
  setIsPlaying: (value: boolean) => void;
  /** Stream format/bitrate, persisted in localStorage. */
  stream: StreamOptions;
  setStream: (stream: StreamOptions) => void;
}

const FORMATS: StreamFormat[] = ['flac', 'mp3', 'opus', 'aac'];

const StationContext = createContext<StationContextValue | null>(null);

export function StationProvider({ children }: { children: React.ReactNode }) {
  const searchParams = useSearchParams();
  const pathname = usePathname();
  const router = useRouter();
  // The URL (?station=…) is the single source of truth for the browsed station.
  const selectedStation = pathname === '/' ? searchParams?.get('station') ?? null : null;

  const [nowPlaying, setNowPlaying] = useState<QueueTrack | null>(null);
  const [playingStation, setPlayingStation] = useState<string | null>(null);
  const [isPlaying, setIsPlaying] = useState(false);
  const [stream, setStreamState] = useState<StreamOptions>({ format: 'flac', bitrate: '320' });
  const { data: stations } = useStations();
  const { data: queue } = useQueue(selectedStation);

  const setSelectedStation = useCallback(
    (id: string | null) => {
      router.replace(id ? `/?station=${encodeURIComponent(id)}` : '/');
    },
    [router],
  );

  useEffect(() => {
    const format = localStorage.getItem('ownwave:format');
    const bitrate = localStorage.getItem('ownwave:bitrate');
    setStreamState((s) => ({
      format: format && FORMATS.includes(format as StreamFormat) ? (format as StreamFormat) : s.format,
      bitrate: bitrate || s.bitrate,
    }));
  }, []);

  const setStream = useCallback((next: StreamOptions) => {
    setStreamState(next);
    localStorage.setItem('ownwave:format', next.format);
    localStorage.setItem('ownwave:bitrate', next.bitrate);
  }, []);

  const value = useMemo(
    () => ({
      selectedStation,
      setSelectedStation,
      stations: stations || [],
      queue: queue || [],
      nowPlaying,
      setNowPlaying,
      playingStation,
      setPlayingStation,
      isPlaying,
      setIsPlaying,
      stream,
      setStream,
    }),
    [selectedStation, setSelectedStation, stations, queue, nowPlaying, playingStation, isPlaying, stream, setStream],
  );

  return <StationContext.Provider value={value}>{children}</StationContext.Provider>;
}

export function useStation() {
  const ctx = useContext(StationContext);
  if (!ctx) throw new Error('useStation must be used within a StationProvider');
  return ctx;
}
