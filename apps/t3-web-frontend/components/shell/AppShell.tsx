'use client';

import { useEffect, useState } from 'react';
import { usePathname } from 'next/navigation';
import { PlayerBar } from '@/components/player/PlayerBar';
import { NowPlayingPanel } from '@/components/player/NowPlayingPanel';
import { NavRail } from './NavRail';
import { TopBar } from './TopBar';

/** Signed-in layout: rail | top bar + page + now-playing panel, player along the bottom. */
export function AppShell({ children }: { children: React.ReactNode }) {
  const path = usePathname();
  const [navOpen, setNavOpen] = useState(false);

  useEffect(() => setNavOpen(false), [path]);

  useEffect(() => {
    if (!navOpen) return;
    const onKey = (e: KeyboardEvent) => e.key === 'Escape' && setNavOpen(false);
    document.addEventListener('keydown', onKey);
    return () => document.removeEventListener('keydown', onKey);
  }, [navOpen]);

  return (
    <div className="h-screen flex flex-col bg-bg text-fg overflow-hidden">
      <a
        href="#main"
        className="sr-only focus:not-sr-only focus:fixed focus:top-2 focus:left-2 focus:z-[60] focus:rounded-md focus:bg-accent focus:px-3 focus:py-1.5 focus:font-semibold focus:text-accent-fg"
      >
        Skip to content
      </a>
      <div className="flex-1 flex min-h-0">
        <NavRail open={navOpen} onClose={() => setNavOpen(false)} />
        <div className="flex-1 flex flex-col min-w-0">
          <TopBar navOpen={navOpen} onMenu={() => setNavOpen(true)} />
          <div className="flex-1 flex min-h-0">
            <main id="main" tabIndex={-1} className="flex-1 min-w-0 overflow-y-auto px-4 py-4 md:px-5 focus:outline-none">
              {children}
            </main>
            <NowPlayingPanel />
          </div>
        </div>
      </div>
      <PlayerBar />
    </div>
  );
}
