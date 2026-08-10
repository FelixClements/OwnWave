'use client';

import Link from 'next/link';
import { useState, useEffect } from 'react';
import { useStation } from '@/lib/station';
import { Player } from '@/components/Player';
import { ProfileButton } from '@/components/ProfileButton';
import { SunIcon, MoonIcon } from '@/components/Icons';

export function Shell({ children }: { children: React.ReactNode }) {
  const { selectedStation, setSelectedStation, stations, queue } = useStation();
  const [isLight, setIsLight] = useState(false);

  useEffect(() => {
    document.documentElement.classList.toggle('light', isLight);
  }, [isLight]);

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
          <header className="h-16 flex items-center justify-between px-4 md:px-8 bg-spotify-bg/95 sticky top-0 z-10 gap-4">
            <h2 className="text-sm font-bold text-spotify-text shrink-0">OwnWave</h2>
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
            {children}
          </div>
        </main>
      </div>

      <div className="h-24 bg-spotify-black border-t border-spotify-border px-4 flex items-center">
        <Player queue={queue} />
      </div>
    </div>
  );
}
