'use client';

import { useMemo, useState } from 'react';
import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { useAuth } from '@/lib/auth';
import { useStation } from '@/lib/station';
import { StationArt } from '@/components/stations/StationArt';
import {
  Equalizer,
  HistoryIcon,
  LibraryIcon,
  PlaylistIcon,
  RadioIcon,
  SettingsIcon,
  ShieldIcon,
  SimilarIcon,
  StationsIcon,
} from '@/components/ui/Icons';

const NAV = [
  { href: '/', label: 'Listen', icon: RadioIcon },
  { href: '/library', label: 'Library', icon: LibraryIcon },
  { href: '/history', label: 'History', icon: HistoryIcon },
  { href: '/similar', label: 'Similar', icon: SimilarIcon },
  { href: '/playlists', label: 'Playlists', icon: PlaylistIcon },
  { href: '/stations/manage', label: 'Stations', icon: StationsIcon },
  { href: '/admin', label: 'Admin', icon: ShieldIcon, admin: true },
  { href: '/settings', label: 'Settings', icon: SettingsIcon },
];

export function Logo() {
  return (
    <span className="flex items-center gap-2 font-bold text-[15px]">
      <span className="w-[22px] h-[22px] rounded-md bg-gradient-to-br from-accent to-blue-500" aria-hidden="true" />
      OwnWave
    </span>
  );
}

/** Primary navigation and the station list. A drawer below md. */
export function NavRail({ open, onClose }: { open: boolean; onClose: () => void }) {
  const path = usePathname() ?? '/';
  const { user } = useAuth();
  const { stations, selectedStation, playingStation, isPlaying } = useStation();
  const [filter, setFilter] = useState('');

  const shown = useMemo(() => {
    const q = filter.trim().toLowerCase();
    return q ? stations.filter((s) => s.name.toLowerCase().includes(q)) : stations;
  }, [stations, filter]);

  return (
    <>
      {open && <div className="fixed inset-0 z-30 bg-black/50 md:hidden" onClick={onClose} aria-hidden="true" />}
      <aside
        id="nav-rail"
        aria-label="Sidebar"
        className={`fixed inset-y-0 left-0 z-40 w-[232px] md:static md:z-auto md:w-[212px] shrink-0 flex flex-col gap-3.5 border-r border-line bg-surface px-2.5 py-3 transition-transform md:translate-x-0 ${
          // Off-screen drawer stays out of the tab order until opened.
          open ? 'translate-x-0' : '-translate-x-full invisible md:visible'
        }`}
      >
        <div className="px-1.5">
          <Logo />
        </div>

        <nav aria-label="Main">
          <ul className="flex flex-col gap-0.5">
            {NAV.filter((n) => !n.admin || user?.is_admin).map(({ href, label, icon: Icon }) => {
              const active = href === '/' ? path === '/' : path.startsWith(href);
              return (
                <li key={href}>
                  <Link
                    href={href}
                    onClick={onClose}
                    aria-current={active ? 'page' : undefined}
                    className={`flex items-center gap-2.5 px-2 py-1.5 rounded-md font-medium transition ${
                      active ? 'bg-surface-2 text-fg shadow-[inset_2px_0_0_rgb(var(--accent))]' : 'text-muted hover:text-fg hover:bg-hover'
                    }`}
                  >
                    <Icon className="w-[15px] h-[15px]" />
                    {label}
                  </Link>
                </li>
              );
            })}
          </ul>
        </nav>

        <section aria-labelledby="rail-stations" className="flex flex-col min-h-0 flex-1">
          <div className="flex justify-between px-2 text-[10.5px] uppercase tracking-widest text-muted">
            <h2 id="rail-stations">Stations</h2>
            <span className="tabular-nums">{stations.length}</span>
          </div>
          <input
            type="search"
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
            placeholder="Filter stations"
            aria-label="Filter stations"
            className="mt-1.5 mb-1 h-[26px] rounded-md border border-line bg-bg px-2 text-xs placeholder:text-muted focus:outline-none focus:border-accent"
          />
          <ul className="flex-1 overflow-y-auto -mx-0.5 px-0.5">
            {shown.map((station) => {
              const selected = path === '/' && selectedStation === station.id;
              const playing = playingStation === station.id;
              return (
                <li key={station.id}>
                  <Link
                    href={`/?station=${encodeURIComponent(station.id)}`}
                    onClick={onClose}
                    aria-current={selected ? 'page' : undefined}
                    className={`flex items-center gap-2 px-2 py-1.5 rounded-md transition ${
                      selected ? 'bg-surface-2 text-fg' : 'text-muted hover:text-fg hover:bg-hover'
                    }`}
                  >
                    <StationArt name={station.name} />
                    <span className="flex-1 truncate">{station.name}</span>
                    {playing && (
                      <>
                        <Equalizer paused={!isPlaying} />
                        <span className="sr-only">{isPlaying ? '(playing)' : '(paused)'}</span>
                      </>
                    )}
                  </Link>
                </li>
              );
            })}
            {shown.length === 0 && <li className="px-2 py-1.5 text-muted">No stations.</li>}
          </ul>
        </section>
      </aside>
    </>
  );
}
