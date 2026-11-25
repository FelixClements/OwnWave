import { useState } from 'react';
import { Link } from 'react-router-dom';
import clsx from 'clsx';
import { Copy, Pencil, RefreshCw } from 'lucide-react';
import { api, callAs } from '@/api/client';
import type { GenreStation, GenreStationSettings, PipelineRun, ScanSummary, StationChange } from '@/api/types';
import { useAuth } from '@/lib/auth';
import { plural, timeAgo } from '@/lib/format';
import {
  isScanFinished,
  useDeleteUser,
  useGenreReview,
  useGenreStations,
  useHealth,
  usePipeline,
  useRefreshPipeline,
  useScans,
  useSetAdmin,
  useSetGenreStations,
  useStartScan,
  useUpdateGenreStation,
  useUsers,
} from '@/lib/queries';
import { scanPhase } from '@/lib/scan';
import { Block, Row, Section, Switch, Tag, table } from '@/components/settings-ui';
import { Button, ErrorText, IconButton, Spinner } from '@/components/ui';

// --- overview -------------------------------------------------------------

const HEALTH_LABELS: Record<string, [string, string]> = {
  go: ['API server', 'Go'],
  db: ['Database', 'PostgreSQL'],
  python: ['Audio analysis', 'Python worker'],
};

function Stat({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-xl border border-line bg-card px-4 py-3">
      <div className="text-[12px] text-faint">{label}</div>
      <div className="mt-0.5 truncate text-xl font-semibold tabular-nums">{value}</div>
    </div>
  );
}

export function OverviewPane() {
  const health = useHealth();
  const pipeline = usePipeline();
  const users = useUsers();
  const stations = useGenreStations();
  const live = stations.data?.genres.filter((g) => g.state === 'active').length;
  const lastScan = pipeline.data?.last_scan;
  return (
    <>
      <div className="mb-8 grid grid-cols-2 gap-3 lg:grid-cols-4">
        <Stat label="Songs" value={pipeline.data?.genres_scored ? pipeline.data.genres_scored.tracks.toLocaleString() : '–'} />
        <Stat label="People" value={users.data ? String(users.data.length) : '–'} />
        <Stat label="Genre stations live" value={live != null ? String(live) : '–'} />
        <Stat label="Last scan" value={lastScan ? timeAgo(lastScan.finished_at ?? lastScan.created_at) : 'Never'} />
      </div>
      <Section title="Health">
        {health.isLoading ? (
          <Block>
            <Spinner />
          </Block>
        ) : health.error ? (
          <Block>
            <ErrorText error={health.error} />
          </Block>
        ) : (
          Object.entries(health.data ?? {})
            .filter(([name]) => name !== 'version') // not a component
            .map(([name, status]) => {
              const [title, description] = HEALTH_LABELS[name] ?? [name.charAt(0).toUpperCase() + name.slice(1), undefined];
              return (
                <Row key={name} title={title} description={description}>
                  <Tag tone={status === 'ok' ? 'ok' : 'bad'}>{status === 'ok' ? 'Healthy' : status}</Tag>
                </Row>
              );
            })
        )}
      </Section>
    </>
  );
}

// --- library --------------------------------------------------------------

function Progress({ scan }: { scan: ScanSummary }) {
  const phase = scanPhase(scan);
  return (
    <Block>
      <div className="mb-2 flex flex-wrap justify-between gap-x-4 text-sm">
        <span className="font-medium">{phase.label}</span>
        <span className="text-muted">
          {plural(scan.stats.imported, 'track')} imported · {scan.stats.model_success} analysed
        </span>
      </div>
      <div className="h-1.5 overflow-hidden rounded-full bg-raised">
        <div
          className={clsx('h-full rounded-full bg-accent transition-all', phase.progress == null && 'w-1/3 animate-pulse')}
          style={phase.progress != null ? { width: `${phase.progress * 100}%` } : undefined}
        />
      </div>
    </Block>
  );
}

export function LibraryPane() {
  const scans = useScans();
  const start = useStartScan();
  const latest = scans.data?.[0];
  const running = latest && !isScanFinished(latest.status);
  return (
    <>
      <Section title="Scan">
        {running && <Progress scan={latest} />}
        <Row title="Scan for changes" description="Add new files and remove deleted ones.">
          <Button size="sm" variant="primary" disabled={running} loading={start.isPending && start.variables === false} onClick={() => start.mutate(false)}>
            Scan for changes
          </Button>
        </Row>
        <Row title="Full rescan" description="Import and analyse every file again. Takes hours on a large library.">
          <Button
            size="sm"
            disabled={running}
            loading={start.isPending && start.variables === true}
            onClick={() => confirm('Re-import and re-analyse every file? This takes hours on a large library.') && start.mutate(true)}
          >
            Full rescan
          </Button>
        </Row>
        {start.error != null && (
          <Block>
            <ErrorText error={start.error} />
          </Block>
        )}
      </Section>
      <Section title="Recent scans">
        {scans.isLoading ? (
          <Block>
            <Spinner />
          </Block>
        ) : (
          <table className={table.table}>
            <thead>
              <tr>
                <th className={table.th}>Started</th>
                <th className={table.th}>Status</th>
                <th className={clsx(table.th, 'text-right')}>Imported</th>
                <th className={clsx(table.th, 'text-right')}>Failed files</th>
              </tr>
            </thead>
            <tbody>
              {(scans.data ?? []).slice(0, 8).map((s) => (
                <tr key={s.id}>
                  <td className={table.td}>{s.started_at ? timeAgo(s.started_at) : '–'}</td>
                  <td className={table.td}>
                    <Tag tone={s.status === 'failed' ? 'bad' : s.status === 'completed' ? 'neutral' : 'ok'}>{scanPhase(s).label}</Tag>
                  </td>
                  <td className={clsx(table.td, 'text-right tabular-nums')}>{s.stats.imported}</td>
                  <td className={clsx(table.td, 'text-right tabular-nums')}>{s.stats.failed_files}</td>
                </tr>
              ))}
              {scans.data?.length === 0 && (
                <tr>
                  <td colSpan={4} className={clsx(table.td, 'text-muted')}>
                    No scans yet.
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        )}
      </Section>
    </>
  );
}

// --- people ---------------------------------------------------------------

export function PeoplePane() {
  const { user: me } = useAuth();
  const users = useUsers();
  const setAdmin = useSetAdmin();
  const remove = useDeleteUser();
  const [username, setUsername] = useState('');
  const [invite, setInvite] = useState<{ invite_url: string; expires_at: string } | null>(null);
  const [error, setError] = useState<unknown>(null);
  const [copied, setCopied] = useState(false);
  return (
    <>
      <Section title="Invite someone">
        <form
          onSubmit={async (e) => {
            e.preventDefault();
            setError(null);
            try {
              setInvite(await callAs<NonNullable<typeof invite>>(api.POST('/api/admin/invites', { body: username.trim() ? { username: username.trim() } : {} })));
              setCopied(false);
            } catch (err) {
              setError(err);
            }
          }}
        >
          <Row title="Invite link" description="Works once and expires after a week. The username is optional.">
            <input
              aria-label="Username (optional)"
              placeholder="Username (optional)"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              className="h-9 w-44 rounded-lg border border-line bg-raised px-2.5 text-sm outline-none placeholder:text-faint focus:border-ink"
            />
            <Button size="sm" type="submit" variant="primary">
              Create invite link
            </Button>
          </Row>
        </form>
        {invite && (
          <Row title={<code className="block truncate font-mono text-[12px] text-muted">{invite.invite_url}</code>} description={`Expires ${new Date(invite.expires_at).toLocaleDateString()}`}>
            <Button size="sm" onClick={() => void navigator.clipboard?.writeText(invite.invite_url).then(() => setCopied(true))}>
              <Copy className="h-4 w-4" /> {copied ? 'Copied' : 'Copy link'}
            </Button>
          </Row>
        )}
        {error != null && (
          <Block>
            <ErrorText error={error} />
          </Block>
        )}
      </Section>
      <Section title="People">
        {users.isLoading && (
          <Block>
            <Spinner />
          </Block>
        )}
        {(users.data ?? []).map((u) => (
          <div key={u.id} className="flex flex-wrap items-center gap-3 px-4 py-3">
            <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-raised text-sm font-bold uppercase">{u.username[0]}</span>
            <span className="min-w-0 flex-1">
              <span className="block truncate text-sm font-medium">
                {u.full_name || u.username} {u.id === me?.id && <Tag>You</Tag>}
              </span>
              <span className="block truncate text-[13px] text-muted">
                {u.full_name ? `${u.username} · ` : ''}
                {u.is_admin ? 'Admin' : 'Listener'}
              </span>
            </span>
            {u.id !== me?.id && (
              <>
                <Button
                  size="sm"
                  variant="ghost"
                  loading={setAdmin.isPending && setAdmin.variables?.id === u.id}
                  onClick={() => setAdmin.mutate({ id: u.id, is_admin: !u.is_admin })}
                >
                  {u.is_admin ? 'Remove admin' : 'Make admin'}
                </Button>
                <Button
                  size="sm"
                  variant="danger"
                  loading={remove.isPending && remove.variables === u.id}
                  onClick={() => confirm(`Delete ${u.username} and their history?`) && remove.mutate(u.id)}
                >
                  Remove
                </Button>
              </>
            )}
          </div>
        ))}
        {(setAdmin.error ?? remove.error) != null && (
          <Block>
            <ErrorText error={setAdmin.error ?? remove.error} />
          </Block>
        )}
      </Section>
    </>
  );
}

// --- genre stations -------------------------------------------------------

function genreState(g: GenreStation, settings: GenreStationSettings): string {
  switch (g.state) {
    case 'active':
      return 'Live';
    case 'excluded':
      return 'Hidden by you';
    case 'off':
      return 'Off';
    default:
      return `Needs ${plural(settings.show_at - g.tracks, 'more track')}`;
  }
}

function GenreStationRow({ g, settings }: { g: GenreStation; settings: GenreStationSettings }) {
  const update = useUpdateGenreStation();
  const [editing, setEditing] = useState(false);
  const [name, setName] = useState(g.name);
  const rename = (value: string) => update.mutate({ genre: g.genre, name: value }, { onSuccess: () => setEditing(false) });
  return (
    <tr className="align-top">
      <td className={table.td}>
        {editing ? (
          <form
            className="flex flex-wrap items-center gap-2"
            onSubmit={(e) => {
              e.preventDefault();
              rename(name.trim());
            }}
          >
            <input
              aria-label={`Name of the ${g.genre_name} station`}
              className="min-w-0 flex-1 rounded-md border border-line bg-raised px-2 py-1"
              value={name}
              maxLength={100}
              autoFocus
              onChange={(e) => setName(e.target.value)}
            />
            <Button size="sm" type="submit" variant="primary" loading={update.isPending} disabled={!name.trim()}>
              Save
            </Button>
            <Button size="sm" variant="ghost" onClick={() => setEditing(false)}>
              Cancel
            </Button>
          </form>
        ) : (
          <div className="flex items-center gap-1">
            {g.station_id && g.state === 'active' ? (
              <Link to={`/station/${g.station_id}`} className="font-medium hover:underline">
                {g.name}
              </Link>
            ) : (
              <span className="font-medium">{g.name}</span>
            )}
            <IconButton
              size="sm"
              label={`Rename the ${g.genre_name} station`}
              onClick={() => {
                setName(g.name);
                setEditing(true);
              }}
            >
              <Pencil className="h-3.5 w-3.5" />
            </IconButton>
          </div>
        )}
        {g.name_is_custom && !editing && (
          <button type="button" className="text-[12px] text-muted hover:text-ink hover:underline" onClick={() => rename('')}>
            Rename back to {g.genre_name}
          </button>
        )}
        <ErrorText error={update.error} />
      </td>
      <td className={clsx(table.td, 'text-right tabular-nums')}>{g.tracks}</td>
      <td className={table.td}>
        <Tag tone={g.state === 'active' ? 'ok' : 'neutral'}>{genreState(g, settings)}</Tag>
      </td>
      <td className={clsx(table.td, 'text-right')}>
        <Button
          size="sm"
          variant="ghost"
          loading={update.isPending && update.variables?.excluded !== undefined}
          onClick={() => update.mutate({ genre: g.genre, excluded: g.state !== 'excluded' })}
        >
          {g.state === 'excluded' ? `Show ${g.genre_name}` : `Hide ${g.genre_name}`}
        </Button>
      </td>
    </tr>
  );
}

export function GenreStationsPane() {
  const q = useGenreStations();
  const setEnabled = useSetGenreStations();
  const settings = q.data;
  if (q.isLoading) return <Spinner />;
  if (!settings) return <ErrorText error={q.error} />;
  return (
    <>
      <Section>
        <Row
          title="A station for every main genre"
          description={`Each genre station plays every song with that main genre, shared with everyone. It appears once ${plural(settings.show_at, 'song')} have the genre and hides again below ${settings.hide_below}. Hidden stations keep their listening history.`}
        >
          <Switch label="A station for every main genre" checked={settings.enabled} disabled={setEnabled.isPending} onChange={(v) => setEnabled.mutate(v)} />
        </Row>
        {setEnabled.error != null && (
          <Block>
            <ErrorText error={setEnabled.error} />
          </Block>
        )}
      </Section>
      <Section title="Stations">
        <table className={table.table}>
          <thead>
            <tr>
              <th className={table.th}>Station</th>
              <th className={clsx(table.th, 'text-right')}>Tracks</th>
              <th className={table.th}>State</th>
              <th className={table.th} />
            </tr>
          </thead>
          <tbody>
            {settings.genres.map((g) => (
              <GenreStationRow key={g.genre} g={g} settings={settings} />
            ))}
            {settings.genres.length === 0 && (
              <tr>
                <td colSpan={4} className={clsx(table.td, 'text-muted')}>
                  No genres yet.
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </Section>
    </>
  );
}

// --- station refresh (pipeline) -------------------------------------------

const triggers: Record<string, string> = {
  startup: 'API started',
  reconnect: 'Reconnected',
  hourly: 'Hourly check',
  admin: 'Refresh now',
  settings: 'Settings changed',
  playlist_added: 'Playlist added',
  playlist_synced: 'Playlist synced',
  playlist_deleted: 'Playlist removed',
  scan_finished: 'Scan finished',
  genres_scored: 'Genres scored',
};

const changeVerbs: Record<StationChange['change'], string> = {
  created: 'created',
  shown: 'shown again',
  hidden: 'hidden',
  renamed: 'renamed',
};

function RunRow({ run }: { run: PipelineRun }) {
  const parts = [...(run.matched ? [`${plural(run.matched, 'track')} matched`] : []), ...run.changes.map((c) => `${c.name} ${changeVerbs[c.change]}`)];
  return (
    <div className="flex flex-wrap gap-x-3 px-4 py-2.5 text-sm">
      <span className="w-24 shrink-0 text-muted">{timeAgo(run.finished_at ?? run.started_at)}</span>
      <span className="w-36 shrink-0 font-medium">{triggers[run.trigger] ?? run.trigger.replace(/_/g, ' ')}</span>
      <span className={clsx('min-w-0 flex-1', run.error ? 'text-danger' : !parts.length && 'text-muted')}>
        {run.error ?? (parts.length ? parts.join(' · ') : 'Nothing changed')}
      </span>
    </div>
  );
}

export function StationRefreshPane() {
  const q = usePipeline();
  const refresh = useRefreshPipeline();
  const status = q.data;
  return (
    <>
      <p className="mb-6 text-[13px] text-muted">
        After a scan and genre scoring, after playlist changes, at startup and every hour, OwnWave matches playlist songs to your files and brings the
        genre stations up to date.
      </p>
      <Section title="Status">
        {q.isLoading ? (
          <Block>
            <Spinner />
          </Block>
        ) : !status ? (
          <Block>
            <ErrorText error={q.error} />
          </Block>
        ) : (
          <>
            <Row title="Last library scan">
              <span className="text-sm text-muted">
                {status.last_scan ? `${status.last_scan.status}, ${timeAgo(status.last_scan.finished_at ?? status.last_scan.created_at)}` : 'Never'}
              </span>
            </Row>
            <Row title="Last genre scoring">
              <span className="text-sm text-muted">
                {status.genres_scored
                  ? `${timeAgo(status.genres_scored.at)}, ${plural(status.genres_scored.scored, 'track')} of ${status.genres_scored.tracks.toLocaleString()}`
                  : 'Not yet'}
              </span>
            </Row>
            <Row title="Last station refresh">
              <span className="text-sm text-muted">{status.runs[0] ? timeAgo(status.runs[0].finished_at ?? status.runs[0].started_at) : 'Not yet'}</span>
            </Row>
          </>
        )}
        <Row title="Refresh stations now" description="Match playlist songs and update the genre stations right away.">
          <Button size="sm" variant="primary" loading={refresh.isPending} onClick={() => refresh.mutate()}>
            <RefreshCw className="h-4 w-4" /> Refresh now
          </Button>
        </Row>
        {refresh.error != null && (
          <Block>
            <ErrorText error={refresh.error} />
          </Block>
        )}
      </Section>
      {status && status.runs.length > 0 && (
        <Section title="Recent refreshes">
          {status.runs.slice(0, 12).map((run) => (
            <RunRow key={run.id} run={run} />
          ))}
        </Section>
      )}
    </>
  );
}

/** Sidebar dots: a scan running, songs waiting for review. Only queried for admins. */
export function useAdminBadges(enabled: boolean) {
  const scans = useScans(enabled);
  const review = useGenreReview(0, enabled, 1);
  return {
    library: !!scans.data?.[0] && !isScanFinished(scans.data[0].status),
    review: (review.data?.total ?? 0) > 0,
  };
}
