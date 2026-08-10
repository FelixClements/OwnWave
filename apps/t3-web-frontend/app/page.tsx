'use client';

import { useEffect, useState } from 'react';
import Link from 'next/link';
import { useSearchParams, useRouter } from 'next/navigation';
import { trpc } from '@/lib/trpc/client';
import { Player } from '@/components/Player';
import { ProfileButton } from '@/components/ProfileButton';
import { getCoverUrl } from '@/lib/api';

function Cover({
  id,
  title,
  className,
}: {
  id: string;
  title: string;
  className?: string;
}) {
  const [error, setError] = useState(false);
  if (error) {
    return (
      <div className={className}>
        <span className="font-bold">{title.charAt(0).toUpperCase()}</span>
      </div>
    );
  }
  return (
    <img
      src={getCoverUrl(id)}
      alt=""
      className={className}
      onError={() => setError(true)}
    />
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
  const searchParams = useSearchParams();
  const router = useRouter();
  const stationParam = searchParams?.get('station') ?? null;

  const [isLight, setIsLight] = useState(false);
  const [selectedStation, setSelectedStation] = useState<string | null>(stationParam);
  const [searchQuery, setSearchQuery] = useState('');

  const { data: search } = trpc.search.useQuery(
    { q: searchQuery },
    { enabled: searchQuery.length > 0 }
  );

  const { data: stations } = trpc.stations.useQuery();
  const { data: queue } = trpc.queue.useQuery(
    { id: selectedStation || '' },
    { enabled: !!selectedStation, refetchInterval: 5000 }
  );

  useEffect(() => {
    document.documentElement.classList.toggle('light', isLight);
  }, [isLight]);

  useEffect(() => {
    const id = searchParams?.get('station') ?? null;
    if (id !== selectedStation) {
      setSelectedStation(id);
    }
  }, [searchParams, selectedStation]);

  const handleSelectStation = (id: string) => {
    setSelectedStation(id);
    const url = new URL(window.location.href);
    url.searchParams.set('station', id);
    router.replace(url.toString());
  }

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
            <Link href="/library" className="hover:text-spotify-text transition">
              Library
            </Link>
            <Link href="/history" className="hover:text-spotify-text transition">
              History
            </Link>
            <Link href="/stations" className="hover:text-spotify-text transition">
              Stations
            </Link>
            <Link href="/stations/manage" className="hover:text-spotify-text transition">
              Manage Stations
            </Link>
            <Link href="/admin" className="hover:text-spotify-text transition">
              Admin
            </Link>
          </nav>

          <div className="flex-1 overflow-y-auto">
            <h3 className="text-xs uppercase tracking-widest text-spotify-subdued mb-3">
              Stations
            </h3>
            {(stations || []).map((station) => (
              <button
                key={station.id}
                onClick={() => handleSelectStation(station.id)}
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
          <header className="h-16 flex items-center justify-between px-4 md:px-8 bg-spotify-bg/95 sticky top-0 z-10 gap-4">
            <h2 className="text-sm font-bold text-spotify-text shrink-0">Browse</h2>
            <div className="flex-1 max-w-md hidden sm:block">
              <input
                type="text"
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                placeholder="Search tracks, albums, artists..."
                className="w-full px-4 py-1.5 rounded-full bg-spotify-elevated text-spotify-text placeholder-spotify-subdued text-sm border border-spotify-border focus:outline-none focus:border-spotify-green"
              />
            </div>
            <div className="flex items-center gap-3 shrink-0">
              <button
                onClick={() => setIsLight(!isLight)}
                className="w-9 h-9 rounded-full bg-spotify-elevated text-spotify-text flex items-center justify-center hover:bg-spotify-card-hover transition"
                aria-label="Toggle theme"
              >
                {isLight ? <SunIcon className="w-4 h-4" /> : <MoonIcon className="w-4 h-4" />}
              </button>
              <ProfileButton />
            </div>
          </header>

          <div className="flex-1 overflow-y-auto p-4 md:p-8 bg-gradient-to-b from-spotify-elevated to-spotify-bg">
            {(stations || []).length === 0 && (
              <p className="text-spotify-subdued">
                No stations yet. <Link href="/stations/manage" className="text-spotify-green hover:underline">Create one</Link> to start listening.
              </p>
            )}

            {!selectedStation && (stations || []).length > 0 && (
              <div className="text-center py-20">
                <h2 className="text-2xl font-bold text-spotify-text mb-4">Select a station to start listening</h2>
                <Link
                  href="/stations"
                  className="inline-block px-6 py-3 rounded-full bg-spotify-green text-black font-semibold hover:bg-spotify-green-hover transition"
                >
                  Browse stations
                </Link>
              </div>
            )}

            {searchQuery && (
              <section className="mb-10">
                <h2 className="text-2xl font-bold text-spotify-text mb-5">Search Results</h2>
                {search ? (
                  <div className="space-y-6">
                    {search.tracks.length > 0 && (
                      <div>
                        <h3 className="text-sm font-bold text-spotify-subdued uppercase tracking-widest mb-3">Tracks</h3>
                        <ul className="space-y-1 bg-spotify-card rounded-lg p-4">
                          {search.tracks.map((track) => (
                            <li
                              key={track.id}
                              className="flex items-center justify-between text-sm text-spotify-text py-1"
                            >
                              <span className="truncate pr-4">{track.title}</span>
                              <span className="text-spotify-subdued truncate">
                                {track.artist || 'Unknown artist'}
                              </span>
                            </li>
                          ))}
                        </ul>
                      </div>
                    )}
                    {search.albums.length > 0 && (
                      <div>
                        <h3 className="text-sm font-bold text-spotify-subdued uppercase tracking-widest mb-3">Albums</h3>
                        <ul className="space-y-1 bg-spotify-card rounded-lg p-4">
                          {search.albums.map((album) => (
                            <li key={album.id} className="text-sm text-spotify-text py-1">
                              {album.title}
                            </li>
                          ))}
                        </ul>
                      </div>
                    )}
                    {search.artists.length > 0 && (
                      <div>
                        <h3 className="text-sm font-bold text-spotify-subdued uppercase tracking-widest mb-3">Artists</h3>
                        <ul className="space-y-1 bg-spotify-card rounded-lg p-4">
                          {search.artists.map((artist) => (
                            <li key={artist.id} className="text-sm text-spotify-text py-1">
                              {artist.name}
                            </li>
                          ))}
                        </ul>
                      </div>
                    )}
                    {search.tracks.length === 0 && search.albums.length === 0 && search.artists.length === 0 && (
                      <p className="text-spotify-subdued">No results found.</p>
                    )}
                  </div>
                ) : (
                  <p className="text-spotify-subdued">Searching...</p>
                )}
              </section>
            )}

            {selectedStation && (queue || []).length > 0 && (
              <section>
                <h2 className="text-2xl font-bold text-spotify-text mb-5">
                  Now Playing
                </h2>
                <div className="bg-spotify-card rounded-lg p-4 mb-6">
                  <div className="flex items-center gap-4 mb-4">
                    <Cover
                      id={(queue || [])[0].id}
                      title={(queue || [])[0].title}
                      className="w-16 h-16 rounded shadow object-cover bg-spotify-green flex items-center justify-center text-black text-xl font-bold"
                    />
                    <div className="min-w-0">
                      <div className="text-lg font-bold text-spotify-text truncate">
                        {(queue || [])[0].title}
                      </div>
                      <div className="text-sm text-spotify-subdued truncate">
                        {(queue || [])[0].artist || 'Unknown artist'}
                      </div>
                      {(queue || [])[0].album && (
                        <div className="text-xs text-spotify-subdued truncate">
                          {(queue || [])[0].album}
                        </div>
                      )}
                    </div>
                  </div>
                  <div className="flex flex-wrap gap-3 text-xs text-spotify-subdued">
                    {(queue || [])[0].bpm && (
                      <span>{(queue || [])[0].bpm.toFixed(0)} BPM</span>
                    )}
                    {(queue || [])[0].key && (
                      <span>{(queue || [])[0].key}</span>
                    )}
                    {(queue || [])[0].energy !== undefined && (
                      <span>Energy {(queue || [])[0].energy.toFixed(2)}</span>
                    )}
                  </div>
                </div>

                {(queue || []).length > 1 && (
                  <>
                    <h3 className="text-lg font-bold text-spotify-text mb-3">
                      Up Next
                    </h3>
                    <div className="bg-spotify-card rounded-lg p-4">
                      <ul className="space-y-1">
                        {(queue || []).slice(1).map((track, i) => (
                          <li
                            key={track.id}
                            className="flex items-center justify-between py-2 px-3 rounded hover:bg-spotify-elevated text-sm"
                          >
                            <span className="text-spotify-text truncate pr-4 flex-1">
                              <span className="text-spotify-subdued w-6 inline-block">
                                {i + 1}
                              </span>
                              {track.title}
                            </span>
                            <span className="text-spotify-subdued truncate text-right max-w-[40%]">
                              {track.artist || 'Unknown artist'}
                              {track.album ? ` · ${track.album}` : ''}
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
        {(queue || []).length > 0 ? (
          <Player queue={(queue || [])} />
        ) : (
          <div className="w-full text-center text-spotify-subdued text-sm">
            Select a station to start listening
          </div>
        )}
      </div>
    </div>
  );
}
