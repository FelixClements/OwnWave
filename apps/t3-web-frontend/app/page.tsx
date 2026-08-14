'use client';

import { useState, useEffect, useMemo } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { useStation } from '@/lib/station';
import { getCoverUrl } from '@/lib/api';
import { trpc } from '@/lib/trpc/client';

function formatTime(seconds?: number) {
  if (!seconds || seconds <= 0) return '--:--';
  const m = Math.floor(seconds / 60);
  const s = Math.floor(seconds % 60);
  return `${m}:${s.toString().padStart(2, '0')}`;
}

function LikeButton({
  liked,
  onClick,
}: {
  liked: boolean;
  onClick: () => void;
}) {
  return (
    <button
      onClick={onClick}
      className={`w-8 h-8 flex items-center justify-center rounded-full transition ${
        liked
          ? 'text-spotify-green'
          : 'text-spotify-subdued hover:text-spotify-text'
      }`}
      title={liked ? 'Remove like' : 'Like'}
    >
      <svg
        className="w-4 h-4"
        viewBox="0 0 24 24"
        fill="currentColor"
        xmlns="http://www.w3.org/2000/svg"
      >
        <path d="M14 9V5a3 3 0 0 0-3-3l-4 9v11h11.28a2 2 0 0 0 2-1.7l1.38-9a2 2 0 0 0-2-2.3H14zM7 22H4a2 2 0 0 1-2-2v-9a2 2 0 0 1 2-2h3v13z" />
      </svg>
    </button>
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

const STATION_PATTERNS = [
  {
    backgroundImage:
      'radial-gradient(circle at 30% 30%, rgba(255,255,255,0.4) 0 8%, transparent 9%), radial-gradient(circle at 70% 70%, rgba(255,255,255,0.25) 0 12%, transparent 13%)',
    backgroundSize: '48px 48px',
  },
  {
    backgroundImage:
      'repeating-linear-gradient(45deg, rgba(255,255,255,0.15) 0, rgba(255,255,255,0.15) 8px, transparent 8px, transparent 16px)',
  },
  {
    backgroundImage:
      'conic-gradient(from 0deg, rgba(255,255,255,0.12), transparent 30%, rgba(255,255,255,0.12) 50%, transparent 80%)',
  },
  {
    backgroundImage:
      'linear-gradient(rgba(255,255,255,0.1) 1px, transparent 1px), linear-gradient(90deg, rgba(255,255,255,0.1) 1px, transparent 1px)',
    backgroundSize: '24px 24px',
  },
];

function hashString(s: string) {
  return s.split('').reduce((a, c) => a + c.charCodeAt(0) * 31, 0);
}

function StationCover({ name }: { name: string }) {
  const style = useMemo(() => {
    const seed = hashString(name);
    const hue1 = Math.abs(seed % 360);
    const hue2 = Math.abs((seed * 17 + 120) % 360);
    const angle = Math.abs((seed * 13) % 360);
    const patternIdx = Math.abs(seed % STATION_PATTERNS.length);
    const patternOpacity = 0.08 + (Math.abs(seed) % 6) * 0.05;
    const fontSize = 2.2 + (Math.abs(seed) % 5) * 0.4;
    const letterSpacing = 0.02 + (Math.abs(seed) % 7) / 100;
    const display = name.replace(/ Radio$/i, '');
    const pattern = STATION_PATTERNS[patternIdx];
    return { hue1, hue2, angle, patternIdx, patternOpacity, fontSize, letterSpacing, display, pattern };
  }, [name]);

  return (
    <div
      className="w-full aspect-square rounded-md shadow-lg mb-4 relative overflow-hidden flex items-center justify-center text-white font-black uppercase break-words leading-none z-0"
      style={{
        background: `linear-gradient(${style.angle}deg, hsl(${style.hue1} 75% 30%), hsl(${style.hue2} 75% 50%))`,
      }}
    >
      <div
        className="absolute inset-0"
        style={{
          backgroundImage: style.pattern.backgroundImage,
          backgroundSize: style.pattern.backgroundSize,
          opacity: style.patternOpacity,
        }}
      />
      <span
        className="relative z-10 px-4"
        style={{
          fontSize: `${style.fontSize}rem`,
          letterSpacing: `${style.letterSpacing}rem`,
        }}
      >
        {style.display}
      </span>
    </div>
  );
}

export default function Home() {
  const router = useRouter();
  const { selectedStation, setSelectedStation, stations, queue, nowPlaying, isPlaying, setIsPlaying, setPlayingStation, playingStation } = useStation();
  const [searchQuery, setSearchQuery] = useState('');

  const setupStatus = trpc.setupStatus.useQuery();
  useEffect(() => {
    if (setupStatus.data && !setupStatus.data.setup_completed) {
      router.push('/setup');
    }
  }, [setupStatus.data, router]);

  const utils = trpc.useContext();
  const recordFeedback = trpc.recordFeedback.useMutation({
    onSuccess: () => {
      if (selectedStation) utils.queue.invalidate({ id: selectedStation });
    },
  });
  const removeFeedback = trpc.removeFeedback.useMutation({
    onSuccess: () => {
      if (selectedStation) utils.queue.invalidate({ id: selectedStation });
    },
  });

  const filteredStations = useMemo(() => {
    const q = searchQuery.trim().toLowerCase();
    if (!q) return stations || [];
    return (stations || []).filter((s) => s.name.toLowerCase().includes(q));
  }, [stations, searchQuery]);

  return (
    <>
      <div className="flex-1 max-w-md hidden sm:block mb-6 flex items-center gap-3">
        <input
          type="text"
          value={searchQuery}
          onChange={(e) => setSearchQuery(e.target.value)}
          placeholder="Search stations..."
          className="flex-1 px-4 py-1.5 rounded-full bg-spotify-elevated text-spotify-text placeholder-spotify-subdued text-sm border border-spotify-border focus:outline-none focus:border-spotify-green"
        />
        <Link
          href="/stations/manage"
          className="px-4 py-1.5 rounded-full bg-spotify-elevated text-spotify-text text-sm border border-spotify-border hover:border-spotify-green transition"
        >
          Manage
        </Link>
      </div>

      {(stations || []).length === 0 && !searchQuery && (
        <p className="text-spotify-subdued">
          No stations yet. <Link href="/stations/manage" className="text-spotify-green hover:underline">Create one</Link> to start listening.
        </p>
      )}

      {searchQuery && filteredStations.length === 0 && !selectedStation && (
        <p className="text-spotify-subdued mb-6">No stations found.</p>
      )}

      {filteredStations.length > 0 && (!selectedStation || searchQuery) && (
        <section className="mb-10">
          <h2 className="text-2xl font-bold text-spotify-text mb-5">Stations</h2>
          <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-4 lg:grid-cols-5 gap-6">
            {filteredStations.map((station) => (
              <button
                key={station.id}
                onClick={() => setSelectedStation(station.id)}
                className={`group relative bg-spotify-card rounded-lg p-4 hover:bg-spotify-card-hover transition text-left w-full ${
                  selectedStation === station.id ? 'ring-2 ring-spotify-green' : ''
                }`}
              >
                <StationCover name={station.name} />
                <h3 className="font-bold text-spotify-text truncate">{station.name}</h3>
                <p className="text-sm text-spotify-subdued">AI Radio</p>
              </button>
            ))}
          </div>
        </section>
      )}

      {selectedStation && (
        <section>
          {(() => {
            const station = stations.find((s) => s.id === selectedStation);
            if (!station) return null;
            const active = isPlaying && playingStation === selectedStation;
            return (
              <div className="flex items-center gap-4 mb-5">
                <h2 className="text-2xl font-bold text-spotify-text">{station.name}</h2>
                <button
                  type="button"
                  onClick={() => {
                    if (active) {
                      setIsPlaying(false);
                    } else {
                      setPlayingStation(selectedStation);
                      setIsPlaying(true);
                    }
                  }}
                  className="px-4 py-1.5 rounded-full bg-spotify-green text-black text-sm font-semibold hover:bg-spotify-green-hover focus:outline-none focus:ring-2 focus:ring-spotify-green"
                >
                  {active ? 'Pause' : 'Play'}
                </button>
              </div>
            );
          })()}
          {(() => {
            const track = playingStation === selectedStation ? (nowPlaying || (queue || [])[0]) : (queue || [])[0];
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
                Queue
              </h3>
              <div className="bg-spotify-card rounded-lg p-4">
                <ul className="space-y-1">
                  {(queue || []).slice(1).map((track, i) => (
                    <li
                      key={track.id}
                      className="grid grid-cols-[1fr,auto,auto] items-center gap-4 py-2 px-3 rounded hover:bg-spotify-elevated text-sm"
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
                      <div className="w-8 flex justify-center shrink-0">
                        <LikeButton
                          liked={!!track.liked}
                          onClick={() =>
                            track.liked
                              ? removeFeedback.mutate({ id: track.id, feedback: 'like' })
                              : recordFeedback.mutate({ id: track.id, feedback: 'like' })
                          }
                        />
                      </div>
                      <div className="w-12 text-right text-spotify-subdued text-xs shrink-0">
                        {formatTime(track.duration_seconds)}
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
