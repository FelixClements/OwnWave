'use client';

import { useState } from 'react';
import { useRouter } from 'next/navigation';
import { useTheme } from '@/lib/theme';
import { IconButton } from '@/components/ui/Button';
import { MenuIcon, MoonIcon, SearchIcon, SunIcon } from '@/components/ui/Icons';
import { ProfileMenu } from './ProfileMenu';

/** Library search (opens /library?q=…), theme toggle and account menu. */
export function TopBar({ navOpen, onMenu }: { navOpen: boolean; onMenu: () => void }) {
  const router = useRouter();
  const { isLight, toggleTheme } = useTheme();
  const [term, setTerm] = useState('');

  const submit = (e: React.FormEvent) => {
    e.preventDefault();
    const q = term.trim();
    router.push(q ? `/library?q=${encodeURIComponent(q)}` : '/library');
  };

  return (
    <header className="h-12 shrink-0 flex items-center gap-2.5 px-3 md:px-4 border-b border-line bg-bg">
      <IconButton
        label="Open navigation"
        onClick={onMenu}
        aria-expanded={navOpen}
        aria-controls="nav-rail"
        className="md:hidden border border-line"
      >
        <MenuIcon />
      </IconButton>
      <form role="search" onSubmit={submit} className="flex-1 max-w-[420px]">
        <label className="flex items-center gap-2 h-[30px] rounded-md border border-line bg-surface px-2.5 text-muted focus-within:border-accent">
          <SearchIcon className="w-[15px] h-[15px] shrink-0" />
          <span className="sr-only">Search library</span>
          <input
            type="search"
            value={term}
            onChange={(e) => setTerm(e.target.value)}
            placeholder="Search tracks, artists, albums"
            className="flex-1 min-w-0 bg-transparent text-fg placeholder:text-muted focus:outline-none"
          />
        </label>
      </form>
      <div className="flex-1" />
      <IconButton label={isLight ? 'Switch to dark theme' : 'Switch to light theme'} onClick={toggleTheme} className="border border-line">
        {isLight ? <SunIcon /> : <MoonIcon />}
      </IconButton>
      <ProfileMenu />
    </header>
  );
}
