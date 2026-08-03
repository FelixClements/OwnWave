'use client';

import { useEffect, useState } from 'react';
import Link from 'next/link';
import { trpc } from '@/lib/trpc/client';
import { Player } from '@/components/Player';
import { Station, QueueTrack } from '@/server/routers/app';

const DEMO_STATIONS: Station[] = [
  { id: 'demo-station-1', name: 'Focus Flow' },
  { id: 'demo-station-2', name: 'High Energy' },
  { id: 'demo-station-3', name: 'Late Night Jazz' },
  { id: 'demo-station-4', name: 'Indie Mix' },
  { id: 'demo-station-5', name: 'Classical Mornings' },
  { id: 'demo-station-6', name: 'Electronic Dreams' },
];

const DEMO_QUEUE: QueueTrack[] = [
  {
    id: 'demo-track-1',
    title: 'Midnight City',
    artist: 'M83',
    album: 'Hurry Up, We\'re Dreaming',
    path: '/music/demo1.flac',
    bpm: 120,
    key: 'A min',
    energy: 0.8,
    valence: 0.6,
    outro_start_seconds: 240,
    ideal_crossfade_seconds: 8,
    position: 1,
  },
  {
    id: 'demo-track-2',
    title: 'Starboy',
    artist: 'The Weeknd',
    album: 'Starboy',
    path: '/music/demo2.flac',
    bpm: 124,
    key: 'D maj',
    energy: 0.7,
    valence: 0.5,
    outro_start_seconds: 200,
    ideal_crossfade_seconds: 6,
    position: 2,
  },
  {
    id: 'demo-track-3',
    title: 'Electric Feel',
    artist: 'MGMT',
    album: 'Oracular Spectacular',
    path: '/music/demo3.flac',
    bpm: 95,
    key: 'F maj',
    energy: 0.6,
    valence: 0.7,
    outro_start_seconds: 220,
    ideal_crossfade_seconds: 7,
    position: 3,
  },
  {
    id: 'demo-track-4',
    title: 'Get Lucky',
    artist: 'Daft Punk',
    album: 'Random Access Memories',
    path: '/music/demo4.flac',
    bpm: 116,
    key: 'F# min',
    energy: 0.75,
    valence: 0.8,
    outro_start_seconds: 210,
    ideal_crossfade_seconds: 5,
    position: 4,
  },
  {
    id: 'demo-track-5',
    title: 'Nightcall',
    artist: 'Kavinsky',
    album: 'OutRun',
    path: '/music/demo5.flac',
    bpm: 110,
    key: 'C min',
    energy: 0.65,
    valence: 0.4,
    outro_start_seconds: 230,
    ideal_crossfade_seconds: 8,
    position: 5,
  },
];

function PlayIcon({ className }: { className?: string }) {
  return (
    <svg
      className={className}
      viewBox="0 0 24 24"
      fill="currentColor"
      xmlns="http://www.w3.org/2000/svg"
    >
      <path d="M8 5v14l11-7L8 5z" />
    </svg>
  );
}

function SunIcon({ className }: { className?: string }) {
  return (
    <svg
      className={className}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <circle cx="12" cy="12" r="5" />
      <path d="M12 1v2M12 21v2M4.22 4.22l1.42 1.42M18.36 18.36l1.42 1.42M1 12h2M21 12h2M4.22 19.78l1.42-1.42M18.36 5.64l1.42-1.42" />
    </svg>
  );
}

function MoonIcon({ className }: { className?: string }) {
  return (
    <svg
      className={className}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z" />
    </svg>
  );
}

export default function Home() {
  const [isLight, setIsLight] = useState(false);
  const [demo, setDemo] = useState(true);
  const [selectedStation, setSelectedStation] = useState<string | null>(null);

  const { data: stations } = trpc.stations.useQuery();
  const { data: queue } = trpc.queue.useQuery(
    { id: selectedStation || '' },
    { enabled: !!selectedStation && !demo, refetchInterval: 5000 }
  );

  const displayStations = demo ? DEMO_STATIONS : (stations || []);
  const displayQueue = demo ? DEMO_QUEUE : (queue || []);

  useEffect(() => {
    document.documentElement.classList.toggle('light', isLight);
  }, [isLight]);

  useEffect(() => {
    if (!selectedStation || !displayStations.some((s) => s.id === selectedStation)) {
      setSelectedStation(displayStations[0]?.id ?? null);
    }
  }, [displayStations, selectedStation]);

  return (
    <div className="h-screen flex flex-col bg-spotify-bg text-spotify-text overflow-hidden">
      <div className="flex-1 flex flex-col md:flex-row overflow-hidden">
        <aside className="w-full md:w-64 h-56 md:h-auto shrink-0 flex flex-col bg-spotify-black p-6 gap-6">
          <div className="text-2xl font-bold tracking-tight text-spotify-text flex items-center gap-3">
            <span className="w-8 h-8 rounded-full bg-spotify-green flex items-center justify-center text-black text-xs">
              OW
            </span>
            OwnWave
          </div>

          <nav className="flex flex-col gap-3 text-sm font-semibold text-spotify-subdued">
            <Link href="/" className="text-spotify-text transition">
              Home
            </Link>
            <Link href="/stations" className="hover:text-spotify-text transition">
              Manage Stations
            </Link>
          </nav>

          <div className="flex-1 overflow-y-auto">
            <h3 className="text-xs uppercase tracking-widest text-spotify-subdued mb-3">
              Stations
            </h3>
            {displayStations.map((station) => (
              <button
                key={station.id}
                onClick={() => setSelectedStation(station.id)}
                className={`block w-full text-left py-2 px-3 rounded-md text-sm transition ${
                  selectedStation === station.id
                    ? 'bg-spotify-elevated text-spotify-text'
                    : 'text-spotify-subdued hover:text-spotify-text'
                }`}
              >
                {station.name}
              </button>
            ))}
          </div>
        </aside>

        <main className="flex-1 flex flex-col min-w-0 overflow-hidden">
          <header className="h-16 flex items-center justify-between px-4 md:px-8 bg-spotify-bg/95 sticky top-0 z-10">
            <h2 className="text-sm font-bold text-spotify-text">Browse</h2>
            <div className="flex items-center gap-3">
              <button
                onClick={() => setDemo(!demo)}
                className="text-xs font-semibold px-3 py-1.5 rounded-full bg-spotify-elevated text-spotify-text hover:bg-spotify-card-hover transition"
              >
                {demo ? 'Demo data: on' : 'Demo data: off'}
              </button>
              <button
                onClick={() => setIsLight(!isLight)}
                className="w-9 h-9 rounded-full bg-spotify-elevated text-spotify-text flex items-center justify-center hover:bg-spotify-card-hover transition"
                aria-label="Toggle theme"
              >
                {isLight ? <SunIcon className="w-4 h-4" /> : <MoonIcon className="w-4 h-4" />}
              </button>
            </div>
          </header>

          <div className="flex-1 overflow-y-auto p-4 md:p-8 bg-gradient-to-b from-spotify-elevated to-spotify-bg">
            {displayStations.length === 0 && (
              <p className="text-spotify-subdued">
                No stations yet. Scan your library and build a station first.
              </p>
            )}

            <section className="mb-10">
              <h2 className="text-2xl font-bold text-spotify-text mb-5">Stations</h2>
              <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-4 lg:grid-cols-5 gap-6">
                {displayStations.map((station) => (
                  <button
                    key={station.id}
                    onClick={() => setSelectedStation(station.id)}
                    className="group relative bg-spotify-card rounded-lg p-4 hover:bg-spotify-card-hover transition text-left"
                  >
                    <div className="w-full aspect-square rounded-md bg-gradient-to-br from-spotify-green to-spotify-green-hover shadow-lg mb-4 flex items-center justify-center text-black font-bold text-2xl">
                      {station.name.charAt(0).toUpperCase()}
                    </div>
                    <h3 className="font-bold text-spotify-text truncate">
                      {station.name}
                    </h3>
                    <p className="text-sm text-spotify-subdued">AI Radio</p>
                    <div className="absolute bottom-16 right-4 w-12 h-12 rounded-full bg-spotify-green shadow-lg opacity-0 group-hover:opacity-100 transition transform translate-y-2 group-hover:translate-y-0 flex items-center justify-center text-black">
                      <PlayIcon className="w-5 h-5 ml-0.5" />
                    </div>
                  </button>
                ))}
              </div>
            </section>

            {displayQueue.length > 0 && (
              <section>
                <h2 className="text-2xl font-bold text-spotify-text mb-5">
                  Now Playing
                </h2>
                <div className="bg-spotify-card rounded-lg p-4 mb-6">
                  <div className="flex items-center gap-4 mb-4">
                    <div className="w-16 h-16 bg-spotify-green rounded shadow flex items-center justify-center text-black text-xl font-bold">
                      {displayQueue[0].title.charAt(0).toUpperCase()}
                    </div>
                    <div className="min-w-0">
                      <div className="text-lg font-bold text-spotify-text truncate">
                        {displayQueue[0].title}
                      </div>
                      <div className="text-sm text-spotify-subdued truncate">
                        {displayQueue[0].artist || 'Unknown artist'}
                      </div>
                    </div>
                  </div>
                </div>

                {displayQueue.length > 1 && (
                  <>
                    <h3 className="text-lg font-bold text-spotify-text mb-3">
                      Up Next
                    </h3>
                    <div className="bg-spotify-card rounded-lg p-4">
                      <ul className="space-y-1">
                        {displayQueue.slice(1).map((track, i) => (
                          <li
                            key={track.id}
                            className="flex items-center justify-between py-2 px-3 rounded hover:bg-spotify-elevated text-sm"
                          >
                            <span className="text-spotify-text truncate pr-4">
                              <span className="text-spotify-subdued w-6 inline-block">
                                {i + 1}
                              </span>
                              {track.title}
                            </span>
                            <span className="text-spotify-subdued truncate">
                              {track.artist || 'Unknown artist'}
                            </span>
                          </li>
                        ))}
                      </ul>
                    </div>
                  </>
                )}
              </section>
            )}
          </div>
        </main>
      </div>

      <div className="h-24 bg-spotify-black border-t border-spotify-border px-4 flex items-center">
        {displayQueue.length > 0 ? (
          <Player queue={displayQueue} />
        ) : (
          <div className="w-full text-center text-spotify-subdued text-sm">
            Select a station to start listening
          </div>
        )}
      </div>
    </div>
  );
}
