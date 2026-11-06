import { useState, type ReactNode } from 'react';
import { Link } from 'react-router-dom';
import clsx from 'clsx';
import { AlertTriangle, CheckCircle2, Copy, Pencil, RefreshCw, Trash2, XCircle } from 'lucide-react';
import { useQueryClient } from '@tanstack/react-query';
import { api, call, callAs } from '@/api/client';
import type { GenreReviewItem, GenreStation, GenreStationSettings, PipelineRun, ScanSummary, StationChange } from '@/api/types';
import { useAuth } from '@/lib/auth';
import { plural, timeAgo } from '@/lib/format';
import {
  isScanFinished,
  keys,
  useGenreCorrectionActions,
  useGenreCorrections,
  useGenreNames,
  useGenreReport,
  useGenreReview,
  useGenres,
  useGenreStations,
  useHealth,
  usePipeline,
  useRefreshPipeline,
  useScans,
  useSetGenreStations,
  useStartScan,
  useUpdateGenreStation,
  useUsers,
} from '@/lib/queries';
import { scanPhase } from '@/lib/scan';
import { GenreChart } from '@/components/GenreChart';
import { Button, ErrorText, IconButton, Input, Spinner } from '@/components/ui';

function Card({ title, action, children }: { title: string; action?: ReactNode; children: ReactNode }) {
  return (
    <section className="rounded-xl bg-card p-6">
      <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
        <h2 className="text-lg font-bold">{title}</h2>
        {action}
      </div>
      {children}
    </section>
  );
}

function Health() {
  const h = useHealth();
  if (h.isLoading) return <Spinner />;
  return (
    <ul className="flex flex-wrap gap-3">
      {Object.entries(h.data ?? {})
        .filter(([name]) => name !== 'version') // not a component
        .map(([name, status]) => {
        const ok = status === 'ok';
        return (
          <li key={name} className="flex items-center gap-2 rounded-full bg-raised px-3 py-1.5 text-sm" title={status}>
            {ok ? <CheckCircle2 className="h-4 w-4 text-accent" aria-hidden /> : <XCircle className="h-4 w-4 text-danger" aria-hidden />}
            <span className="font-semibold capitalize">{name}</span>
            <span className="text-muted">{ok ? 'OK' : status}</span>
          </li>
        );
      })}
    </ul>
  );
}

function Progress({ scan }: { scan: ScanSummary }) {
  const phase = scanPhase(scan);
  return (
    <div className="rounded-lg bg-raised p-4">
      <div className="mb-2 flex justify-between gap-4 text-sm">
        <span className="font-semibold">{phase.label}</span>
        <span className="text-muted">
          {plural(scan.stats.imported, 'track')} imported · {scan.stats.model_success} analysed
        </span>
      </div>
      <div className="h-1.5 overflow-hidden rounded-full bg-base">
        <div
          className={clsx('h-full rounded-full bg-accent transition-all', phase.progress == null && 'w-1/3 animate-pulse')}
          style={phase.progress != null ? { width: `${phase.progress * 100}%` } : undefined}
        />
      </div>
    </div>
  );
}

function Scans() {
  const scans = useScans();
  const start = useStartScan();
  const latest = scans.data?.[0];
  const running = latest && !isScanFinished(latest.status);
  return (
    <Card
      title="Library"
      action={
        <div className="flex gap-2">
          <Button size="sm" variant="primary" disabled={running} loading={start.isPending && start.variables === false} onClick={() => start.mutate(false)}>
            Scan for changes
          </Button>
          <Button
            size="sm"
            disabled={running}
            loading={start.isPending && start.variables === true}
            onClick={() => confirm('Re-import and re-analyse every file? This takes hours on a large library.') && start.mutate(true)}
          >
            Full rescan
          </Button>
        </div>
      }
    >
      <ErrorText error={start.error} />
      {running && <Progress scan={latest} />}
      {scans.isLoading ? (
        <Spinner />
      ) : (
        <table className="mt-4 w-full text-left text-sm">
          <thead className="text-[12px] uppercase tracking-wider text-muted">
            <tr>
              <th className="py-2 font-medium">Started</th>
              <th className="py-2 font-medium">Status</th>
              <th className="py-2 text-right font-medium">Imported</th>
              <th className="py-2 text-right font-medium">Failed files</th>
            </tr>
          </thead>
          <tbody>
            {(scans.data ?? []).slice(0, 8).map((s) => (
              <tr key={s.id} className="border-t border-line">
                <td className="py-2">{s.started_at ? timeAgo(s.started_at) : '–'}</td>
                <td className={clsx('py-2', s.status === 'failed' && 'text-danger')}>{scanPhase(s).label}</td>
                <td className="py-2 text-right tabular-nums">{s.stats.imported}</td>
                <td className="py-2 text-right tabular-nums">{s.stats.failed_files}</td>
              </tr>
            ))}
            {scans.data?.length === 0 && (
              <tr>
                <td colSpan={4} className="py-4 text-muted">
                  No scans yet.
                </td>
              </tr>
            )}
          </tbody>
        </table>
      )}
    </Card>
  );
}

function Genres() {
  const genres = useGenres();
  const [showReport, setShowReport] = useState(false);
  const report = useGenreReport(showReport);
  const qc = useQueryClient();
  const [refreshing, setRefreshing] = useState(false);
  return (
    <Card
      title="Genres"
      action={
        <Button
          size="sm"
          loading={refreshing}
          onClick={async () => {
            setRefreshing(true);
            await call(api.POST('/api/admin/rebuild-genres')).catch(() => {});
            setRefreshing(false);
            setTimeout(() => void qc.invalidateQueries({ queryKey: keys.genres }), 5000);
          }}
        >
          <RefreshCw className="h-4 w-4" /> Refresh genres
        </Button>
      }
    >
      {genres.isLoading ? (
        <Spinner />
      ) : genres.error ? (
        <ErrorText error={genres.error} />
      ) : (
        genres.data && (
          <>
            {genres.data.warnings.map((w) => (
              <p key={w} role="alert" className="mb-4 flex items-start gap-2 rounded-md bg-danger/10 p-3 text-sm">
                <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0 text-danger" aria-hidden />
                <span>
                  <strong>Warning: </strong>
                  {w}
                </span>
              </p>
            ))}
            <GenreChart data={genres.data} />
          </>
        )
      )}
      <div className="mt-6 border-t border-line pt-4">
        {!showReport ? (
          <Button size="sm" variant="ghost" onClick={() => setShowReport(true)}>
            Check the audio model against file tags
          </Button>
        ) : report.isLoading ? (
          <Spinner />
        ) : report.data ? (
          <div className="text-sm">
            <p>
              The audio model alone agrees with the file tag on{' '}
              <strong>{report.data.audio_agreement != null ? `${Math.round(report.data.audio_agreement * 100)}%` : '–'}</strong> of{' '}
              {plural(report.data.tracks, 'track')} with a single-genre tag. The final genres also use tags and MusicBrainz.
            </p>
            {report.data.top_confusions.length > 0 && (
              <>
                <h3 className="mb-1 mt-4 font-semibold">Most common disagreements</h3>
                <ul className="text-muted">
                  {report.data.top_confusions.slice(0, 8).map((c) => (
                    <li key={`${c.tag}-${c.audio}`}>
                      tagged {c.tag}, audio says {c.audio ?? '–'}: {c.tracks}
                    </li>
                  ))}
                </ul>
              </>
            )}
          </div>
        ) : (
          <ErrorText error={report.error} />
        )}
      </div>
    </Card>
  );
}

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
    <tr className="border-t border-line align-top">
      <td className="py-2 pr-3">
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
              className="min-w-0 flex-1 rounded-md bg-raised px-2 py-1"
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
              <Link to={`/station/${g.station_id}`} className="font-semibold hover:underline">
                {g.name}
              </Link>
            ) : (
              <span className="font-semibold">{g.name}</span>
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
      <td className="py-2 pr-3 text-right tabular-nums">{g.tracks}</td>
      <td className={clsx('py-2 pr-3', g.state === 'active' ? 'text-accent' : 'text-muted')}>{genreState(g, settings)}</td>
      <td className="py-2 text-right">
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

function GenreStations() {
  const q = useGenreStations();
  const setEnabled = useSetGenreStations();
  const settings = q.data;
  return (
    <Card
      title="Genre stations"
      action={
        settings && (
          <label className="flex cursor-pointer items-center gap-2 text-sm">
            <input
              type="checkbox"
              checked={settings.enabled}
              disabled={setEnabled.isPending}
              onChange={() => setEnabled.mutate(!settings.enabled)}
              className="h-4 w-4 accent-[#2dd4bf]"
            />
            A station for every main genre
          </label>
        )
      }
    >
      {q.isLoading ? (
        <Spinner />
      ) : !settings ? (
        <ErrorText error={q.error} />
      ) : (
        <>
          <p className="text-sm text-muted">
            Each genre station plays every song in your library with that main genre, shared with everyone. It appears once{' '}
            {plural(settings.show_at, 'song')} have the genre and hides again below {settings.hide_below}. Hidden stations keep their listening
            history.
          </p>
          <ErrorText error={setEnabled.error} />
          <table className="mt-4 w-full text-left text-sm">
            <thead className="text-[12px] uppercase tracking-wider text-muted">
              <tr>
                <th className="py-2 font-medium">Station</th>
                <th className="py-2 pr-3 text-right font-medium">Tracks</th>
                <th className="py-2 font-medium">State</th>
                <th className="py-2" />
              </tr>
            </thead>
            <tbody>
              {settings.genres.map((g) => (
                <GenreStationRow key={g.genre} g={g} settings={settings} />
              ))}
              {settings.genres.length === 0 && (
                <tr>
                  <td colSpan={4} className="py-4 text-muted">
                    No genres yet.
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </>
      )}
    </Card>
  );
}

/** One song to check: pick its genre, for the song or for its whole artist. */
function GenreReviewRow({ item, genres }: { item: GenreReviewItem; genres: { genre: string; name: string }[] }) {
  const { save } = useGenreCorrectionActions();
  const [genre, setGenre] = useState(item.genre);
  return (
    <li className="py-3 text-sm">
      <div className="flex flex-wrap items-baseline gap-x-2">
        <span className="min-w-0 truncate font-medium">{item.title}</span>
        <span className="min-w-0 truncate text-muted">{item.artist ?? 'Unknown artist'}</span>
        {item.hidden_by > 0 && <span className="rounded-full bg-raised px-2 py-0.5 text-[11px] text-muted">Hidden by {plural(item.hidden_by, 'listener')}</span>}
        {item.copies_disagree && <span className="rounded-full bg-raised px-2 py-0.5 text-[11px] text-muted">Copies disagree</span>}
      </div>
      <div className="mt-2 flex flex-wrap items-center gap-2">
        <select
          aria-label={`Genre of ${item.title}`}
          value={genre}
          onChange={(e) => setGenre(e.target.value)}
          className="h-9 rounded-md border border-line bg-card px-2 text-sm text-ink outline-none focus:border-ink"
        >
          {genres.map((g) => (
            <option key={g.genre} value={g.genre}>
              {g.name}
            </option>
          ))}
        </select>
        <Button size="sm" loading={save.isPending} onClick={() => save.mutate({ track_id: item.track_id, genre })}>
          {genre === item.genre ? 'Looks right' : 'Set for this song'}
        </Button>
        {item.artist && genre !== item.genre && (
          <Button size="sm" loading={save.isPending} onClick={() => save.mutate({ artist: item.artist, genre })}>
            Set for this artist
          </Button>
        )}
      </div>
      <ErrorText error={save.error} />
    </li>
  );
}

function GenreReview() {
  const review = useGenreReview();
  const corrections = useGenreCorrections();
  const genreData = useGenres();
  const genreName = useGenreNames();
  const { remove } = useGenreCorrectionActions();
  const genres = (genreData.data?.genres ?? []).filter((g) => g.station).map((g) => ({ genre: g.genre, name: g.name }));
  return (
    <Card title="Genre review">
      <p className="text-sm text-muted">
        Songs most worth checking: those people hid on a genre station, then those whose copies disagree, then the least certain. A correction
        replaces the scored genre at the next scoring, which starts right away.
      </p>
      {review.isLoading ? (
        <Spinner />
      ) : (
        <>
          <ErrorText error={review.error} />
          <ul className="mt-2 max-h-96 divide-y divide-line overflow-y-auto">
            {review.data?.map((item) => <GenreReviewRow key={`${item.track_id}-${item.genre}`} item={item} genres={genres} />)}
            {review.data?.length === 0 && <li className="py-4 text-sm text-muted">Nothing to check.</li>}
          </ul>
        </>
      )}
      <div className="mt-4 border-t border-line pt-4">
        <h3 className="mb-2 text-sm font-semibold">Corrections{corrections.data?.length ? ` (${corrections.data.length})` : ''}</h3>
        <ErrorText error={corrections.error ?? remove.error} />
        {corrections.data?.length === 0 && <p className="text-sm text-muted">None yet.</p>}
        <ul className="max-h-64 divide-y divide-line overflow-y-auto">
          {corrections.data?.map((c) => {
            const what = c.artist ? `Everything by ${c.artist}` : `${c.track_title} · ${c.track_artist ?? 'Unknown artist'}`;
            return (
              <li key={c.id} className="flex items-center gap-3 py-2 text-sm">
                <span className="min-w-0 flex-1 truncate">{what}</span>
                <span className="shrink-0 font-semibold">{genreName(c.genre)}</span>
                <IconButton label={`Remove the correction of ${what}`} size="sm" disabled={remove.isPending} onClick={() => remove.mutate(c.id)}>
                  <Trash2 className="h-4 w-4" />
                </IconButton>
              </li>
            );
          })}
        </ul>
      </div>
    </Card>
  );
}

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
  const parts = [
    ...(run.matched ? [`${plural(run.matched, 'track')} matched`] : []),
    ...run.changes.map((c) => `${c.name} ${changeVerbs[c.change]}`),
  ];
  return (
    <li className="flex flex-wrap gap-x-3 py-2 text-sm">
      <span className="w-24 shrink-0 text-muted">{timeAgo(run.finished_at ?? run.started_at)}</span>
      <span className="w-36 shrink-0 font-semibold">{triggers[run.trigger] ?? run.trigger.replace(/_/g, ' ')}</span>
      <span className={clsx('min-w-0 flex-1', run.error ? 'text-danger' : !parts.length && 'text-muted')}>
        {run.error ?? (parts.length ? parts.join(' · ') : 'Nothing changed')}
      </span>
    </li>
  );
}

function Pipeline() {
  const q = usePipeline();
  const refresh = useRefreshPipeline();
  const status = q.data;
  return (
    <Card
      title="Keeping stations up to date"
      action={
        <Button size="sm" loading={refresh.isPending} onClick={() => refresh.mutate()}>
          <RefreshCw className="h-4 w-4" /> Refresh now
        </Button>
      }
    >
      <p className="text-sm text-muted">
        After a scan and genre scoring, after playlist changes, at startup and every hour, OwnWave matches playlist songs to your files and
        brings the genre stations up to date.
      </p>
      <ErrorText error={refresh.error} />
      {q.isLoading ? (
        <Spinner />
      ) : !status ? (
        <ErrorText error={q.error} />
      ) : (
        <>
          <dl className="mt-4 grid grid-cols-[auto_1fr] gap-x-4 gap-y-1 text-sm">
            <dt className="text-muted">Last scan</dt>
            <dd>
              {status.last_scan
                ? `${status.last_scan.status}, ${timeAgo(status.last_scan.finished_at ?? status.last_scan.created_at)}`
                : 'Never'}
            </dd>
            <dt className="text-muted">Genres scored</dt>
            <dd>
              {status.genres_scored
                ? `${timeAgo(status.genres_scored.at)}, ${plural(status.genres_scored.scored, 'track')} of ${status.genres_scored.tracks.toLocaleString()}`
                : 'Not yet'}
            </dd>
            <dt className="text-muted">Stations refreshed</dt>
            <dd>{status.runs[0] ? timeAgo(status.runs[0].finished_at ?? status.runs[0].started_at) : 'Not yet'}</dd>
          </dl>
          {status.runs.length > 0 && (
            <>
              <h3 className="mb-1 mt-6 text-sm font-semibold">Recent refreshes</h3>
              <ul className="divide-y divide-line">
                {status.runs.slice(0, 12).map((run) => (
                  <RunRow key={run.id} run={run} />
                ))}
              </ul>
            </>
          )}
        </>
      )}
    </Card>
  );
}

function Users() {
  const { user: me } = useAuth();
  const users = useUsers();
  const qc = useQueryClient();
  const [username, setUsername] = useState('');
  const [invite, setInvite] = useState<{ invite_url: string; expires_at: string } | null>(null);
  const [error, setError] = useState<unknown>(null);
  const [copied, setCopied] = useState(false);
  return (
    <Card title="People">
      <ul className="mb-6 divide-y divide-line">
        {(users.data ?? []).map((u) => (
          <li key={u.id} className="flex items-center gap-3 py-2.5">
            <span className="flex h-8 w-8 items-center justify-center rounded-full bg-raised text-sm font-bold uppercase">{u.username[0]}</span>
            <span className="flex-1">
              {u.username} {u.is_admin && <span className="text-[12px] text-accent">· admin</span>}
            </span>
            {u.id !== me?.id && (
              <IconButton
                label={`Delete ${u.username}`}
                size="sm"
                onClick={async () => {
                  if (!confirm(`Delete ${u.username} and their history?`)) return;
                  await call(api.DELETE('/api/admin/users/{id}', { params: { path: { id: u.id } } })).catch(setError);
                  void qc.invalidateQueries({ queryKey: keys.users });
                }}
              >
                <Trash2 className="h-4 w-4" />
              </IconButton>
            )}
          </li>
        ))}
      </ul>
      <form
        className="flex flex-col gap-3 sm:flex-row sm:items-end"
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
        <div className="flex-1">
          <Input label="Invite someone" placeholder="Username (optional)" value={username} onChange={(e) => setUsername(e.target.value)} />
        </div>
        <Button type="submit" variant="primary">
          Create invite link
        </Button>
      </form>
      <ErrorText error={error} />
      {invite && (
        <div className="mt-4 flex items-center gap-2 rounded-md bg-raised p-3 text-sm">
          <code className="min-w-0 flex-1 truncate">{invite.invite_url}</code>
          <IconButton
            label={copied ? 'Copied' : 'Copy link'}
            active={copied}
            size="sm"
            onClick={() => void navigator.clipboard?.writeText(invite.invite_url).then(() => setCopied(true))}
          >
            <Copy className="h-4 w-4" />
          </IconButton>
          <span className="text-[12px] text-muted">expires {new Date(invite.expires_at).toLocaleDateString()}</span>
        </div>
      )}
    </Card>
  );
}

export function AdminPage() {
  return (
    <div className="pt-2">
      <h1 className="mb-6 text-3xl font-black tracking-tight">Admin</h1>
      <div className="mb-6">
        <Health />
      </div>
      <div className="grid gap-6 xl:grid-cols-2">
        <Scans />
        <Genres />
        <GenreStations />
        <GenreReview />
        <Pipeline />
        <Users />
      </div>
    </div>
  );
}
