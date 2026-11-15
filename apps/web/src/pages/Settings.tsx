import { useEffect, useRef, useState, type ReactNode } from 'react';
import { Link, Navigate, NavLink, Route, Routes, useLocation } from 'react-router-dom';
import clsx from 'clsx';
import {
  Activity,
  AudioLines,
  Library,
  ListChecks,
  Lock,
  LogOut,
  Play,
  Radio,
  RefreshCw,
  RotateCcw,
  Search,
  Smartphone,
  Tag as TagIcon,
  User,
  Users,
  X,
  type LucideIcon,
} from 'lucide-react';
import { api, call } from '@/api/client';
import type { Quality } from '@/api/types';
import { useAuth } from '@/lib/auth';
import { timeAgo } from '@/lib/format';
import { useDevices, useRevokeDevice, useUpdateProfile } from '@/lib/queries';
import { QUALITY_LABELS, defaultSettings, onCellular, updateSettings, useSettings } from '@/lib/settings';
import { Slider } from '@/components/player-ui';
import { Block, Note, Row, Section, SelectControl, Switch, Tag, table } from '@/components/settings-ui';
import { Button, ErrorText, Spinner } from '@/components/ui';
import { GenreReviewPane, GenresPane, GenreStationsPane, LibraryPane, OverviewPane, PeoplePane, StationRefreshPane, useAdminBadges } from './Admin';

const QUALITIES = (Object.keys(QUALITY_LABELS) as Quality[]).map((q) => ({ value: q, label: QUALITY_LABELS[q] }));

// --- your settings ----------------------------------------------------------

function PlaybackPane() {
  const s = useSettings();
  return (
    <>
      <Section title="Playback">
        <Row title="Crossfade" description="Blend the end of one song into the next.">
          <Slider label="Crossfade seconds" value={s.crossfade} max={12} step={1} onChange={(crossfade) => updateSettings({ crossfade })} className="w-40" />
          <span className="w-8 text-right text-sm tabular-nums text-muted">{s.crossfade ? `${s.crossfade} s` : 'Off'}</span>
        </Row>
        <Row title="Even out loudness" description="Play every song at about the same volume.">
          <Switch label="Even out loudness" checked={s.normalize} onChange={(normalize) => updateSettings({ normalize })} />
        </Row>
      </Section>
      <Note>Playback and quality settings are saved in this browser only.</Note>
    </>
  );
}

const QUALITY_COST: [Quality, string, string][] = [
  ['original', 'The file itself (FLAC, MP3…)', 'depends on the file'],
  ['high', 'Opus 160 kbps', '≈ 72 MB'],
  ['normal', 'Opus 96 kbps', '≈ 43 MB'],
  ['low', 'Opus 64 kbps', '≈ 29 MB'],
  ['aac', 'AAC 128 kbps, for older devices', '≈ 58 MB'],
];

function QualityPane() {
  const s = useSettings();
  const cellular = onCellular();
  const now = <Tag tone="ok">In use now</Tag>;
  return (
    <>
      <Section title="Streaming quality">
        <Row title={<>Wi-Fi and wired {!cellular && now}</>} description="Quality on a fast connection.">
          <SelectControl label="Wi-Fi and wired quality" value={s.quality} onChange={(quality) => updateSettings({ quality })} options={QUALITIES} />
        </Row>
        <Row title={<>Mobile data or data saver {cellular && now}</>} description="Used when the browser reports mobile data or data saver.">
          <SelectControl label="Mobile data quality" value={s.mobileQuality} onChange={(mobileQuality) => updateSettings({ mobileQuality })} options={QUALITIES} />
        </Row>
      </Section>
      <Note>Lower qualities are converted once on the server, then cached. Changes apply from the next song.</Note>
      <Section title="What each option uses">
        <table className={table.table}>
          <thead>
            <tr>
              <th className={table.th}>Option</th>
              <th className={table.th}>Format</th>
              <th className={clsx(table.th, 'text-right')}>Per hour</th>
            </tr>
          </thead>
          <tbody>
            {QUALITY_COST.map(([q, format, size]) => (
              <tr key={q}>
                <td className={table.td}>{QUALITY_LABELS[q].split(' · ')[0]}</td>
                <td className={clsx(table.td, 'text-muted')}>{format}</td>
                <td className={clsx(table.td, 'text-right tabular-nums text-muted')}>{size}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </Section>
    </>
  );
}

const fieldClass = 'h-9 w-56 max-w-full rounded-lg border border-line bg-raised px-2.5 text-sm outline-none placeholder:text-faint focus:border-ink';

function ProfilePane() {
  const { user } = useAuth();
  const update = useUpdateProfile();
  const [fullName, setFullName] = useState(user?.full_name ?? '');
  const [email, setEmail] = useState(user?.email ?? '');
  if (!user) return null;
  const changed = fullName !== (user.full_name ?? '') || email !== (user.email ?? '');
  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        update.mutate({ full_name: fullName.trim(), email: email.trim() });
      }}
    >
      <Section title="Profile">
        <div className="flex items-center gap-3 px-4 py-3">
          <span className="flex h-10 w-10 items-center justify-center rounded-full bg-accent text-base font-bold uppercase text-black">{user.username[0]}</span>
          <div>
            <div className="text-sm font-medium">{user.username}</div>
            <div className="text-[13px] text-muted">{user.is_admin ? 'Admin' : 'Listener'}</div>
          </div>
        </div>
        <Row title="Full name" description="Shown to the other people on this server.">
          <input aria-label="Full name" className={fieldClass} maxLength={100} value={fullName} onChange={(e) => setFullName(e.target.value)} />
        </Row>
        <Row title="Email" description="Optional.">
          <input aria-label="Email" type="email" className={fieldClass} value={email} onChange={(e) => setEmail(e.target.value)} />
        </Row>
        <Block className="flex items-center justify-end gap-3">
          <ErrorText error={update.error} />
          {update.isSuccess && !changed && <span className="text-sm text-accent">Saved.</span>}
          <Button size="sm" type="submit" variant="primary" disabled={!changed} loading={update.isPending}>
            Save
          </Button>
        </Block>
      </Section>
    </form>
  );
}

function PasswordPane() {
  const [current, setCurrent] = useState('');
  const [next, setNext] = useState('');
  const [repeat, setRepeat] = useState('');
  const [state, setState] = useState<{ busy?: boolean; error?: unknown; done?: boolean }>({});
  const mismatch = repeat !== '' && repeat !== next;
  return (
    <form
      onSubmit={async (e) => {
        e.preventDefault();
        if (mismatch) return;
        setState({ busy: true });
        try {
          await call(api.POST('/api/me/password', { body: { current_password: current, new_password: next } }));
          setCurrent('');
          setNext('');
          setRepeat('');
          setState({ done: true });
        } catch (err) {
          setState({ error: err });
        }
      }}
    >
      <Section title="Change password">
        <Row title="Current password">
          <input aria-label="Current password" type="password" autoComplete="current-password" className={fieldClass} value={current} onChange={(e) => setCurrent(e.target.value)} required />
        </Row>
        <Row title="New password" description="At least 8 characters. Signs out every other browser and app.">
          <input aria-label="New password" type="password" autoComplete="new-password" minLength={8} className={fieldClass} value={next} onChange={(e) => setNext(e.target.value)} required />
        </Row>
        <Row title="Repeat new password" description={mismatch ? <span className="text-danger">The passwords do not match.</span> : undefined}>
          <input aria-label="Repeat new password" type="password" autoComplete="new-password" className={fieldClass} value={repeat} onChange={(e) => setRepeat(e.target.value)} required />
        </Row>
        <Block className="flex items-center justify-end gap-3">
          <ErrorText error={state.error} />
          {state.done && <span className="text-sm text-accent">Password changed.</span>}
          <Button type="submit" size="sm" variant="primary" loading={state.busy} disabled={mismatch}>
            Change password
          </Button>
        </Block>
      </Section>
    </form>
  );
}

function DevicesPane() {
  const devices = useDevices();
  const revoke = useRevokeDevice();
  const [signingOut, setSigningOut] = useState(false);
  const others = devices.data?.filter((d) => !d.current) ?? [];
  return (
    <>
      <Section title="Signed-in apps">
        {devices.isLoading ? (
          <Block>
            <Spinner />
          </Block>
        ) : !devices.data?.length ? (
          <Block className="text-sm text-muted">No apps signed in. Sign in from the OwnWave Android app to see it here.</Block>
        ) : (
          devices.data.map((d) => (
            <Row
              key={d.id}
              title={
                <span className="flex items-center gap-2">
                  <Smartphone className="h-4 w-4 text-muted" aria-hidden />
                  {d.name} {d.current && <Tag tone="ok">This device</Tag>}
                </span>
              }
              description={`${d.last_used_at ? `Last used ${timeAgo(d.last_used_at)}` : 'Never used'} · signed in ${timeAgo(d.created_at)}`}
            >
              {!d.current && (
                <Button size="sm" variant="danger" loading={revoke.isPending && revoke.variables === d.id} onClick={() => revoke.mutate(d.id)}>
                  Sign out
                </Button>
              )}
            </Row>
          ))
        )}
        {revoke.error != null && (
          <Block>
            <ErrorText error={revoke.error} />
          </Block>
        )}
      </Section>
      <Note>Android and Android Auto stay signed in until you sign them out here.</Note>
      {others.length > 1 && (
        <div className="flex justify-end">
          <Button
            size="sm"
            variant="danger"
            loading={signingOut}
            onClick={async () => {
              setSigningOut(true);
              for (const d of others) await revoke.mutateAsync(d.id).catch(() => {});
              setSigningOut(false);
            }}
          >
            Sign out of all other apps
          </Button>
        </div>
      )}
    </>
  );
}

// --- layout -------------------------------------------------------------------

type Page = {
  path: string;
  label: string;
  icon: LucideIcon;
  group: string;
  /** Extra words the search box matches. */
  keywords: string;
  element: ReactNode;
  admin?: boolean;
  badge?: 'library' | 'review';
  /** Pages whose settings "Restore defaults" resets. */
  restorable?: boolean;
};

export const SETTINGS_PAGES: Page[] = [
  { path: 'playback', label: 'Playback', icon: Play, group: '', keywords: 'crossfade loudness normalize volume', element: <PlaybackPane />, restorable: true },
  { path: 'quality', label: 'Audio quality', icon: AudioLines, group: '', keywords: 'streaming bitrate opus aac lossless mobile data wifi', element: <QualityPane />, restorable: true },
  { path: 'profile', label: 'Profile', icon: User, group: 'Account', keywords: 'name email', element: <ProfilePane /> },
  { path: 'password', label: 'Password', icon: Lock, group: 'Account', keywords: 'security change', element: <PasswordPane /> },
  { path: 'devices', label: 'Signed-in apps', icon: Smartphone, group: 'Account', keywords: 'devices android sign out sessions', element: <DevicesPane /> },
  { path: 'overview', label: 'Overview', icon: Activity, group: 'Server', keywords: 'health status stats', element: <OverviewPane />, admin: true },
  { path: 'library', label: 'Library', icon: Library, group: 'Server', keywords: 'scan rescan import files', element: <LibraryPane />, admin: true, badge: 'library' },
  { path: 'people', label: 'People', icon: Users, group: 'Server', keywords: 'users invite admin accounts', element: <PeoplePane />, admin: true },
  { path: 'genres', label: 'Genres', icon: TagIcon, group: 'Stations & genres', keywords: 'refresh report model tags', element: <GenresPane />, admin: true },
  { path: 'genre-stations', label: 'Genre stations', icon: Radio, group: 'Stations & genres', keywords: 'hide show rename', element: <GenreStationsPane />, admin: true },
  { path: 'genre-review', label: 'Genre review', icon: ListChecks, group: 'Stations & genres', keywords: 'corrections check songs', element: <GenreReviewPane />, admin: true, badge: 'review' },
  { path: 'station-refresh', label: 'Station refresh', icon: RefreshCw, group: 'Stations & genres', keywords: 'pipeline update playlists match', element: <StationRefreshPane />, admin: true },
];

function SettingsNav({ pages }: { pages: Page[] }) {
  const { user, logout } = useAuth();
  const badges = useAdminBadges(!!user?.is_admin);
  const [query, setQuery] = useState('');
  const searchRef = useRef<HTMLInputElement>(null);
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const t = e.target as HTMLElement;
      if (e.key === '/' && !['INPUT', 'TEXTAREA', 'SELECT'].includes(t.tagName) && !t.isContentEditable) {
        e.preventDefault();
        searchRef.current?.focus();
      }
    };
    document.addEventListener('keydown', onKey);
    return () => document.removeEventListener('keydown', onKey);
  }, []);
  const q = query.trim().toLowerCase();
  const shown = q ? pages.filter((p) => `${p.label} ${p.group} ${p.keywords}`.toLowerCase().includes(q)) : pages;
  const groups = [...new Set(shown.map((p) => p.group))];
  return (
    <aside className="shrink-0 md:sticky md:top-20 md:w-56">
      <div className="rounded-xl border border-line bg-base/60 p-2">
        <label className="mb-1 hidden items-center gap-2 rounded-lg px-2.5 py-2 text-sm text-faint focus-within:bg-card md:flex">
          <Search className="h-4 w-4 shrink-0" aria-hidden />
          <input
            ref={searchRef}
            aria-label="Search settings"
            placeholder="Search"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            onKeyDown={(e) => e.key === 'Escape' && setQuery('')}
            className="min-w-0 flex-1 bg-transparent text-ink outline-none placeholder:text-faint"
          />
          <kbd className="rounded border border-line bg-raised px-1.5 text-[11px] text-muted">/</kbd>
        </label>
        <nav aria-label="Settings" className="flex gap-1 overflow-x-auto md:flex-col md:gap-0 md:overflow-visible">
          {groups.map((group) => (
            <div key={group || 'top'} className="contents md:block">
              {group && (
                <div className="hidden items-center gap-1.5 px-2.5 pb-1 pt-3 text-[11px] text-faint md:flex">
                  {group}
                  {shown.some((p) => p.group === group && p.admin) && (
                    <span className="rounded bg-accent-soft px-1 text-[9.5px] font-semibold uppercase tracking-wide text-accent">Admin</span>
                  )}
                </div>
              )}
              {shown
                .filter((p) => p.group === group)
                .map(({ path, label, icon: Icon, badge }) => (
                  <NavLink
                    key={path}
                    to={`/settings/${path}`}
                    className={({ isActive }) =>
                      clsx(
                        'flex shrink-0 items-center gap-2.5 whitespace-nowrap rounded-lg px-2.5 py-1.5 text-sm transition',
                        isActive ? 'bg-accent-soft text-ink [&>svg]:text-accent' : 'text-muted hover:bg-card hover:text-ink',
                      )
                    }
                  >
                    <Icon className="h-4 w-4 shrink-0" aria-hidden />
                    {label}
                    {badge && badges[badge] && (
                      <span className={clsx('ml-auto h-1.5 w-1.5 rounded-full', badge === 'review' ? 'bg-amber-400' : 'bg-accent')} aria-label={badge === 'review' ? 'Songs to check' : 'Scan running'} />
                    )}
                  </NavLink>
                ))}
            </div>
          ))}
          {shown.length === 0 && <p className="px-2.5 py-2 text-sm text-faint">Nothing matches.</p>}
        </nav>
        {user && (
          <div className="mt-2 hidden items-center gap-2 border-t border-line px-2 pt-2.5 md:flex">
            <span className="flex h-7 w-7 items-center justify-center rounded-full bg-accent text-xs font-bold uppercase text-black">{user.username[0]}</span>
            <span className="min-w-0 flex-1">
              <span className="block truncate text-sm">{user.username}</span>
              <span className="block text-[11px] text-faint">{user.is_admin ? 'Admin' : 'Listener'}</span>
            </span>
            <button type="button" aria-label="Log out" title="Log out" onClick={() => void logout()} className="rounded p-1 text-faint hover:bg-card hover:text-ink">
              <LogOut className="h-4 w-4" />
            </button>
          </div>
        )}
      </div>
    </aside>
  );
}

export function SettingsPage() {
  const { user } = useAuth();
  const location = useLocation();
  const pages = SETTINGS_PAGES.filter((p) => !p.admin || user?.is_admin);
  const current = SETTINGS_PAGES.find((p) => location.pathname.startsWith(`/settings/${p.path}`));
  return (
    <div className="flex flex-col gap-6 pt-2 md:flex-row md:items-start">
      <SettingsNav pages={pages} />
      <div className="min-w-0 max-w-3xl flex-1">
        <header className="mb-6 flex items-center gap-2 border-b border-line pb-3 text-sm text-muted">
          <span>Settings</span>
          {current && (
            <>
              <span aria-hidden>/</span>
              <h1 className="font-medium text-ink">{current.label}</h1>
            </>
          )}
          <span className="ml-auto flex items-center gap-1">
            {current?.restorable && (
              <Button
                size="sm"
                variant="ghost"
                onClick={() => {
                  const { volume: _keep, ...defaults } = defaultSettings;
                  updateSettings(defaults);
                }}
              >
                <RotateCcw className="h-3.5 w-3.5" /> Restore defaults
              </Button>
            )}
            <Link to="/" aria-label="Close settings" className="rounded p-1.5 text-faint hover:bg-card hover:text-ink">
              <X className="h-4 w-4" />
            </Link>
          </span>
        </header>
        <Routes>
          <Route index element={<Navigate to="/settings/playback" replace />} />
          {SETTINGS_PAGES.map((p) => (
            <Route key={p.path} path={p.path} element={p.admin && !user?.is_admin ? <Navigate to="/" replace /> : p.element} />
          ))}
          <Route path="*" element={<Navigate to="/settings/playback" replace />} />
        </Routes>
      </div>
    </div>
  );
}
