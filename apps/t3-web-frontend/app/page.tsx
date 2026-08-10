'use client';

import { useState } from 'react';
import Link from 'next/link';
import { trpc } from '@/lib/trpc/client';
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
  const { selectedStation, setSelectedStation, stations, queue } = useStation();
  const [searchQuery, setSearchQuery] = useState('');

  const { data: search } = trpc.search.useQuery(
    { q: searchQuery },
    { enabled: searchQuery.length > 0 }
  );

  return (
    <>
      <div className="flex-1 max-w-md hidden sm:block mb-6">
        <input
          type="text"
          value={searchQuery}
          onChange={(e) => setSearchQuery(e.target.value)}
          placeholder="Search tracks, albums, artists..."
          className="w-full px-4 py-1.5 rounded-full bg-spotify-elevated text-spotify-text placeholder-spotify-subdued text-sm border border-spotify-border focus:outline-none focus:border-spotify-green"
        />
      </div>

      {(stations || []).length === 0 && (
        <p className="text-spotify-subdued">
          No stations yet. <Link href="/stations/manage" className="text-spotify-green hover:underline">Create one</Link> to start listening.
        </p>
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

      {!selectedStation && (stations || []).length > 0 && (
        <section>
          <h2 className="text-2xl font-bold text-spotify-text mb-5">Stations</h2>
          <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-4 lg:grid-cols-5 gap-6">
            {(stations || []).map((station) => (
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

      {selectedStation && (queue || []).length > 0 && (
        <section>
          <h2 className="text-2xl font-bold text-spotify-text mb-5">Now Playing</h2>
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
    </>
  );
}
