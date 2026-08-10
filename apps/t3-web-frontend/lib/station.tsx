'use client';

import { createContext, useContext, useMemo, useState, useEffect } from 'react';
import { useSearchParams, useRouter } from 'next/navigation';
import { trpc } from '@/lib/trpc/client';
import { QueueTrack } from '@/server/routers/app';

interface StationContextValue {
  selectedStation: string | null;
  setSelectedStation: (id: string | null) => void;
  stations: { id: string; name: string }[];
  queue: QueueTrack[];
  nowPlaying: QueueTrack | null;
  setNowPlaying: (track: QueueTrack | null) => void;
}

const StationContext = createContext<StationContextValue | null>(null);

export function StationProvider({ children }: { children: React.ReactNode }) {
  const searchParams = useSearchParams();
  const router = useRouter();
  const stationParam = searchParams?.get('station') ?? null;

  const [selectedStation, setSelectedState] = useState<string | null>(stationParam);
  const [nowPlaying, setNowPlayingState] = useState<QueueTrack | null>(null);
  const { data: stations } = trpc.stations.useQuery();
  const { data: queue } = trpc.queue.useQuery(
    { id: selectedStation || '' },
    { enabled: !!selectedStation, refetchInterval: 5000 }
  );

  useEffect(() => {
    const id = searchParams?.get('station') ?? null;
    if (id !== selectedStation) {
      setSelectedState(id);
    }
  }, [searchParams, selectedStation]);

  const setSelectedStation = (id: string | null) => {
    setSelectedState(id);
    const url = new URL(window.location.href);
    url.pathname = '/';
    if (id) {
      url.searchParams.set('station', id);
    } else {
      url.searchParams.delete('station');
    }
    router.replace(url.toString());
  };

  const setNowPlaying = (track: QueueTrack | null) => {
    setNowPlayingState(track);
  };

  const value = useMemo(
    () => ({
      selectedStation,
      setSelectedStation,
      stations: stations || [],
      queue: queue || [],
      nowPlaying,
      setNowPlaying,
    }),
    [selectedStation, stations, queue, nowPlaying]
  );

  return <StationContext.Provider value={value}>{children}</StationContext.Provider>;
}

export function useStation() {
  const ctx = useContext(StationContext);
  if (!ctx) throw new Error('useStation must be used within a StationProvider');
  return ctx;
}
