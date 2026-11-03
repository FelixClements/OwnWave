import { useEffect, useId, useRef, useState, type ReactNode } from 'react';
import { NavLink, Outlet, useLocation, useNavigate } from 'react-router-dom';
import clsx from 'clsx';
import { ChevronLeft, ChevronRight, Home, Library, ListMusic, LogOut, Plus, Search, Settings, Shield, Volume2 } from 'lucide-react';
import { useAuth } from '@/lib/auth';
import { plural } from '@/lib/format';
import { useStations } from '@/lib/queries';
import { usePlayer } from '@/player/PlayerProvider';
import { stationCover } from './media';
import { NowPlaying, PlayerBar } from './player-ui';
import { StationEditor } from './StationEditor';
import { Art, IconButton } from './ui';

export function Logo({ className }: { className?: string }) {
  // Per-instance id: a gradient referenced from a hidden (display:none) copy does not render.
  const gradient = `ow-g-${useId().replace(/:/g, '')}`;
  return (
    <span className={clsx('inline-flex items-center gap-2 font-black tracking-tight', className)}>
      <svg viewBox="0 0 64 64" className="h-8 w-8" aria-hidden>
        <defs>
          <linearGradient id={gradient} x1="0" y1="0" x2="1" y2="1">
            <stop offset="0" stopColor="#2dd4bf" />
            <stop offset="1" stopColor="#38bdf8" />
          </linearGradient>
        </defs>
        <path d="M6 34c5 0 6-14 11-14s6 24 11 24 6-32 11-32 6 24 11 24 7-8 8-8" fill="none" stroke={`url(#${gradient})`} strokeWidth="6" strokeLinecap="round" strokeLinejoin="round" />
      </svg>
      <span className="text-xl">OwnWave</span>
    </span>
  );
}

const NAV = [
  { to: '/', label: 'Home', icon: Home, end: true },
  { to: '/search', label: 'Search', icon: Search },
  { to: '/library', label: 'Library', icon: Library },
  { to: '/playlists', label: 'Playlists', icon: ListMusic },
];

function NavItem({ to, label, icon: Icon, end }: (typeof NAV)[number]) {
  return (
    <NavLink
      to={to}
      end={end}
      className={({ isActive }) =>
        clsx('flex items-center gap-4 rounded-md px-3 py-2 font-bold transition', isActive ? 'text-ink' : 'text-muted hover:text-ink')
      }
    >
      <Icon className="h-6 w-6" aria-hidden />
      {label}
    </NavLink>
  );
}

function Sidebar() {
  const { user } = useAuth();
  const { data: stations } = useStations();
  const player = usePlayer();
  const [creating, setCreating] = useState(false);
  const navigate = useNavigate();
  return (
    <aside className="hidden min-h-0 w-[280px] flex-col gap-2 md:flex" aria-label="Sidebar">
      <nav className="rounded-lg bg-panel px-3 py-4">
        <div className="mb-4 px-3">
          <Logo />
        </div>
        {NAV.map((n) => (
          <NavItem key={n.to} {...n} />
        ))}
        {user?.is_admin && <NavItem to="/admin" label="Admin" icon={Shield} />}
      </nav>
      <section className="flex min-h-0 flex-1 flex-col rounded-lg bg-panel" aria-label="Your stations">
        <div className="flex items-center justify-between px-5 pb-2 pt-4">
          <NavLink to="/stations" className="font-bold text-muted hover:text-ink">
            Stations
          </NavLink>
          <IconButton label="New station" size="sm" onClick={() => setCreating(true)}>
            <Plus className="h-5 w-5" />
          </IconButton>
        </div>
        <ul className="min-h-0 flex-1 overflow-y-auto px-2 pb-2">
          {stations?.map((s) => {
            const active = player.station?.id === s.id;
            return (
              <li key={s.id}>
                <NavLink
                  to={`/station/${s.id}`}
                  className={({ isActive }) => clsx('flex items-center gap-3 rounded-md p-2 transition hover:bg-raised', isActive && 'bg-raised')}
                >
                  <Art src={stationCover(s.id)} name={s.name} className="h-12 w-12" />
                  <span className="min-w-0 flex-1">
                    <span className={clsx('block truncate font-medium', active && 'text-accent')}>{s.name}</span>
                    <span className="block truncate text-[13px] text-muted">
                      {s.shared ? 'Station' : 'Your station'} · {plural(s.track_count, 'track')}
                    </span>
                  </span>
                  {active && player.playing && <Volume2 className="h-4 w-4 shrink-0 text-accent" aria-label="Playing" />}
                </NavLink>
              </li>
            );
          })}
          {stations?.length === 0 && (
            <li className="m-2 rounded-lg bg-raised p-4">
              <p className="font-bold">Create your first station</p>
              <p className="mt-1 text-[13px] text-muted">Pick playlists and genres; OwnWave plays the matches from your library.</p>
              <button type="button" onClick={() => setCreating(true)} className="mt-4 rounded-full bg-ink px-4 py-1.5 text-sm font-bold text-black">
                Create station
              </button>
            </li>
          )}
        </ul>
      </section>
      <StationEditor open={creating} onClose={() => setCreating(false)} onSaved={(s) => navigate(`/station/${s.id}`)} />
    </aside>
  );
}

function UserMenu() {
  const { user, logout } = useAuth();
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);
  const navigate = useNavigate();
  useEffect(() => {
    if (!open) return;
    const close = (e: MouseEvent) => !ref.current?.contains(e.target as Node) && setOpen(false);
    document.addEventListener('mousedown', close);
    return () => document.removeEventListener('mousedown', close);
  }, [open]);
  if (!user) return null;
  const item = 'flex w-full items-center gap-3 rounded px-3 py-2.5 text-left text-sm hover:bg-line';
  return (
    <div ref={ref} className="relative">
      <button
        type="button"
        aria-haspopup="menu"
        aria-expanded={open}
        aria-label={`Account: ${user.username}`}
        onClick={() => setOpen(!open)}
        className="flex h-8 w-8 items-center justify-center rounded-full bg-accent/90 text-sm font-bold uppercase text-black ring-4 ring-black/40 transition hover:scale-105"
      >
        {user.username[0]}
      </button>
      {open && (
        <div role="menu" className="absolute right-0 top-10 z-40 w-52 rounded-md bg-raised p-1 shadow-lift">
          <div className="px-3 py-2 text-[12px] text-muted">Signed in as {user.username}</div>
          <button role="menuitem" type="button" className={item} onClick={() => (setOpen(false), navigate('/settings'))}>
            <Settings className="h-4 w-4" /> Settings
          </button>
          {user.is_admin && (
            <button role="menuitem" type="button" className={item} onClick={() => (setOpen(false), navigate('/admin'))}>
              <Shield className="h-4 w-4" /> Admin
            </button>
          )}
          <button role="menuitem" type="button" className={clsx(item, 'border-t border-line')} onClick={() => void logout()}>
            <LogOut className="h-4 w-4" /> Log out
          </button>
        </div>
      )}
    </div>
  );
}

/** Sticky bar over the page: back/forward and the account menu; darkens on scroll. */
function TopBar({ scrolled }: { scrolled: boolean }) {
  const navigate = useNavigate();
  return (
    <div className={clsx('sticky top-0 z-30 flex h-16 items-center justify-between px-4 transition-colors md:px-8', scrolled && 'bg-panel/95 backdrop-blur')}>
      <div className="flex items-center gap-2">
        <span className="md:hidden">
          <Logo className="scale-90" />
        </span>
        <IconButton label="Back" className="hidden bg-black/50 text-ink md:inline-flex" onClick={() => navigate(-1)}>
          <ChevronLeft className="h-5 w-5" />
        </IconButton>
        <IconButton label="Forward" className="hidden bg-black/50 text-ink md:inline-flex" onClick={() => navigate(1)}>
          <ChevronRight className="h-5 w-5" />
        </IconButton>
      </div>
      <UserMenu />
    </div>
  );
}

function MobileNav() {
  const { user } = useAuth();
  const items = [...NAV, ...(user?.is_admin ? [{ to: '/admin', label: 'Admin', icon: Shield }] : [])];
  return (
    <nav aria-label="Main" className="grid h-14 md:hidden" style={{ gridTemplateColumns: `repeat(${items.length}, minmax(0, 1fr))` }}>
      {items.map(({ to, label, icon: Icon, ...rest }) => (
        <NavLink
          key={to}
          to={to}
          end={'end' in rest}
          className={({ isActive }) => clsx('flex flex-col items-center justify-center gap-0.5 text-[10px]', isActive ? 'text-ink' : 'text-muted')}
        >
          <Icon className="h-5 w-5" aria-hidden />
          {label}
        </NavLink>
      ))}
    </nav>
  );
}

/** The signed-in app: sidebar, scrolling main panel, player bar. */
export function Shell() {
  const mainRef = useRef<HTMLElement>(null);
  const [scrolled, setScrolled] = useState(false);
  const location = useLocation();
  useEffect(() => {
    if (mainRef.current) mainRef.current.scrollTop = 0;
  }, [location.pathname]);
  return (
    <div className="flex h-dvh flex-col gap-2 bg-base p-0 md:p-2">
      <div className="flex min-h-0 flex-1 gap-2">
        <Sidebar />
        <main
          ref={mainRef}
          onScroll={(e) => setScrolled(e.currentTarget.scrollTop > 40)}
          className="relative min-h-0 flex-1 overflow-y-auto bg-panel md:rounded-lg"
        >
          <TopBar scrolled={scrolled} />
          <div className="px-4 pb-10 md:px-8">
            <Outlet />
          </div>
        </main>
      </div>
      <PlayerBar />
      <MobileNav />
      <NowPlaying />
    </div>
  );
}

/** Centered card for the signed-out pages. */
export function AuthLayout({ title, subtitle, children }: { title: string; subtitle?: string; children: ReactNode }) {
  return (
    <div className="min-h-dvh bg-gradient-to-b from-[#123a37] via-base to-base px-4 py-10">
      <div className="mx-auto flex max-w-md flex-col items-center">
        <Logo className="mb-10 text-ink" />
        <div className="w-full rounded-xl bg-panel/90 p-8 shadow-lift md:p-10">
          <h1 className="text-center text-3xl font-black tracking-tight">{title}</h1>
          {subtitle && <p className="mt-2 text-center text-sm text-muted">{subtitle}</p>}
          <div className="mt-8">{children}</div>
        </div>
      </div>
    </div>
  );
}
