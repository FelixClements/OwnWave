'use client';

import { createContext, useCallback, useContext, useMemo, useState } from 'react';
import { usePathname, useRouter, useSearchParams } from 'next/navigation';
import type { QueueTrack } from '@/lib/api';
import { useQueue, useStations } from '@/lib/queries';

interface StationContextValue {
  selectedStation: string | null;
  setSelectedStation: (id: string | null) => void;
  stations: { id: string; name: string }[];
  queue: QueueTrack[];
  nowPlaying: QueueTrack | null;
  setNowPlaying: (track: QueueTrack | null) => void;
  playingStation: string | null;
  setPlayingStation: (id: string | null) => void;
  isPlaying: boolean;
  setIsPlaying: (value: boolean) => void;
}

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
  const { data: stations } = useStations();
  const { data: queue } = useQueue(selectedStation);

  const setSelectedStation = useCallback(
    (id: string | null) => {
      router.replace(id ? `/?station=${encodeURIComponent(id)}` : '/');
    },
    [router],
  );

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
    }),
    [selectedStation, setSelectedStation, stations, queue, nowPlaying, playingStation, isPlaying],
  );

  return <StationContext.Provider value={value}>{children}</StationContext.Provider>;
}

export function useStation() {
  const ctx = useContext(StationContext);
  if (!ctx) throw new Error('useStation must be used within a StationProvider');
  return ctx;
}
