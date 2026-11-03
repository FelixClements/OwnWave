import { useState, type ReactNode } from 'react';
import clsx from 'clsx';
import { AlertTriangle, CheckCircle2, Copy, RefreshCw, Trash2, XCircle } from 'lucide-react';
import { useQueryClient } from '@tanstack/react-query';
import { api, call, callAs } from '@/api/client';
import type { ScanSummary } from '@/api/types';
import { useAuth } from '@/lib/auth';
import { plural, timeAgo } from '@/lib/format';
import { isScanFinished, keys, useGenreReport, useGenres, useHealth, useScans, useStartScan, useUsers } from '@/lib/queries';
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
        <Users />
      </div>
    </div>
  );
}
