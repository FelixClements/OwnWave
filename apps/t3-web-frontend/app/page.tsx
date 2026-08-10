'use client';

import { useState, useEffect, useMemo } from 'react';
import Link from 'next/link';
import { useStation } from '@/lib/station';
import { getCoverUrl } from '@/lib/api';

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
  useEffect(() => {
    setError(false);
  }, [id]);
  return (
    <div className={`${className} overflow-hidden`}>
      {error ? (
        <div className="w-full h-full flex items-center justify-center bg-spotify-green text-black font-bold text-xl">
          {title.charAt(0).toUpperCase()}
        </div>
      ) : (
        <img
          src={getCoverUrl(id)}
          alt={title}
          className="w-full h-full object-cover"
          onError={() => setError(true)}
        />
      )}
    </div>
  );
}

export default function Home() {
  const { selectedStation, setSelectedStation, stations, queue, nowPlaying } = useStation();
  const [searchQuery, setSearchQuery] = useState('');

  const filteredStations = useMemo(() => {
    const q = searchQuery.trim().toLowerCase();
    if (!q) return stations || [];
    return (stations || []).filter((s) => s.name.toLowerCase().includes(q));
  }, [stations, searchQuery]);

  return (
    <>
      <div className="flex-1 max-w-md hidden sm:block mb-6">
        <input
          type="text"
          value={searchQuery}
          onChange={(e) => setSearchQuery(e.target.value)}
          placeholder="Search stations..."
          className="w-full px-4 py-1.5 rounded-full bg-spotify-elevated text-spotify-text placeholder-spotify-subdued text-sm border border-spotify-border focus:outline-none focus:border-spotify-green"
        />
      </div>

      {(stations || []).length === 0 && !searchQuery && (
        <p className="text-spotify-subdued">
          No stations yet. <Link href="/stations/manage" className="text-spotify-green hover:underline">Create one</Link> to start listening.
        </p>
      )}

      {searchQuery && filteredStations.length === 0 && (
        <p className="text-spotify-subdued mb-6">No stations found.</p>
      )}

      {filteredStations.length > 0 && (
        <section className="mb-10">
          <h2 className="text-2xl font-bold text-spotify-text mb-5">Stations</h2>
          <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-4 lg:grid-cols-5 gap-6">
            {filteredStations.map((station) => (
              <button
                key={station.id}
                onClick={() => setSelectedStation(station.id)}
                className="group relative bg-spotify-card rounded-lg p-4 hover:bg-spotify-card-hover transition text-left"
              >
                <div className="w-full aspect-square rounded-md bg-gradient-to-br from-spotify-green to-spotify-green-hover shadow-lg mb-4 flex items-center justify-center text-black font-bold text-2xl">
                  {station.name.charAt(0).toUpperCase()}
                </div>
                <h3 className="font-bold text-spotify-text truncate">{station.name}</h3>
                <p className="text-sm text-spotify-subdued">AI Radio</p>
                <div className="absolute bottom-16 right-4 w-12 h-12 rounded-full bg-spotify-green shadow-lg opacity-0 group-hover:opacity-100 transition transform translate-y-2 group-hover:translate-y-0 flex items-center justify-center text-black">
                  <PlayIcon className="w-5 h-5 ml-0.5" />
                </div>
              </button>
            ))}
          </div>
        </section>
      )}

      {selectedStation && ((queue || []).length > 0 || nowPlaying) && (
        <section>
          <h2 className="text-2xl font-bold text-spotify-text mb-5">Now Playing</h2>
          {(() => {
            const track = nowPlaying || (queue || [])[0];
            if (!track) return null;
            return (
              <div className="bg-spotify-card rounded-lg p-4 mb-6">
                <div className="flex items-center gap-4 mb-4">
                  <Cover
                    id={track.id}
                    title={track.title}
                    className="w-16 h-16 rounded shadow object-cover bg-spotify-green flex items-center justify-center text-black text-xl font-bold"
                  />
                  <div className="min-w-0">
                    <div className="text-lg font-bold text-spotify-text truncate">
                      {track.title}
                    </div>
                    <div className="text-sm text-spotify-subdued truncate">
                      {track.artist || 'Unknown artist'}
                    </div>
                    {track.album && (
                      <div className="text-xs text-spotify-subdued truncate">
                        {track.album}
                      </div>
                    )}
                  </div>
                </div>
                <div className="flex flex-wrap gap-3 text-xs text-spotify-subdued">
                  {track.bpm && (
                    <span>{track.bpm.toFixed(0)} BPM</span>
                  )}
                  {track.key && (
                    <span>{track.key}</span>
                  )}
                  {track.energy !== undefined && (
                    <span>Energy {track.energy.toFixed(2)}</span>
                  )}
                </div>
              </div>
            );
          })()}

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
                      className="grid grid-cols-[1fr,auto] items-center gap-4 py-2 px-3 rounded hover:bg-spotify-elevated text-sm"
                    >
                      <div className="min-w-0">
                        <span className="text-spotify-text truncate">
                          <span className="text-spotify-subdued w-6 inline-block">
                            {i + 1}
                          </span>
                          {track.title}
                        </span>
                        <span className="block text-spotify-subdued truncate text-xs">
                          {track.artist || 'Unknown artist'}
                          {track.album ? ` · ${track.album}` : ''}
                        </span>
                      </div>
                      <div className="w-6 flex justify-center shrink-0">
                        {track.liked && (
                          <span className="text-spotify-green" title="Liked">👍</span>
                        )}
                      </div>
                    </li>
                  ))}
                </ul>
              </div>
            </>
          )}
        </section>
      )}
    </>
  );
}
