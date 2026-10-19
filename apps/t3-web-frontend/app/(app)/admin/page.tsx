'use client';

import { useEffect, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '@/lib/api';
import { useAuth } from '@/lib/auth';
import { isScanFinished, keys, useScanStatus, useScans } from '@/lib/queries';
import { failedFileCount, scanDuration } from '@/lib/scan';
import { formatDateTime } from '@/lib/format';
import { ScanProgress, ScanStateBadge } from '@/components/scan/ScanProgress';
import { Button } from '@/components/ui/Button';
import { Field, Input } from '@/components/ui/Field';
import { Badge, Notice, PageHeader, Panel, type NoticeValue } from '@/components/ui/Layout';
import { EmptyRow, Table, Td, Th } from '@/components/ui/Table';

const errorNotice = (prefix: string) => (err: Error): NoticeValue => ({ type: 'error', text: `${prefix}: ${err.message}` });

export default function AdminPage() {
  const { user } = useAuth();
  if (user && !user.is_admin) return null;

  return (
    <>
      <PageHeader title="Admin" subtitle="Users, system health, scans and maintenance." />
      <div className="grid gap-4 xl:grid-cols-2 items-start">
        <UsersSection />
        <div className="space-y-4">
          <HealthSection />
          <MaintenanceSection />
        </div>
        <ScanSection />
        <StationsSection />
      </div>
    </>
  );
}

function UsersSection() {
  const { user } = useAuth();
  const qc = useQueryClient();
  const { data: users } = useQuery(keys.users, () => api.listUsers());
  const [inviteUsername, setInviteUsername] = useState('');
  const [inviteUrl, setInviteUrl] = useState('');
  const [newUsername, setNewUsername] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [notice, setNotice] = useState<NoticeValue>(null);

  const createInvite = useMutation((vars: { username?: string }) => api.createInvite(vars.username), {
    onMutate: () => setNotice(null),
    onSuccess: async (data) => {
      setInviteUrl(data.invite_url);
      try {
        await navigator.clipboard.writeText(data.invite_url);
        setNotice({ type: 'success', text: 'Invite link created and copied to the clipboard.' });
      } catch {
        // Clipboard access needs HTTPS and permission; the link is shown below.
        setNotice({ type: 'success', text: 'Invite link created.' });
      }
    },
    onError: (err: Error) => setNotice(errorNotice('Invite failed')(err)),
  });
  const createUser = useMutation(
    (vars: { username: string; password: string }) => api.createUser(vars.username, vars.password),
    {
      onMutate: () => setNotice(null),
      onSuccess: (_, vars) => {
        setNewUsername('');
        setNewPassword('');
        void qc.invalidateQueries(keys.users);
        setNotice({ type: 'success', text: `User ${vars.username} created.` });
      },
      onError: (err: Error) => setNotice(errorNotice('Create user failed')(err)),
    },
  );
  const deleteUser = useMutation((vars: { id: string; username: string }) => api.deleteUser(vars.id), {
    onMutate: () => setNotice(null),
    onSuccess: (_, vars) => {
      void qc.invalidateQueries(keys.users);
      setNotice({ type: 'success', text: `User ${vars.username} removed.` });
    },
    onError: (err: Error) => setNotice(errorNotice('Remove user failed')(err)),
  });

  return (
    <Panel title="Users">
      <div className="space-y-3">
        <form
          className="flex flex-wrap items-end gap-2"
          onSubmit={(e) => {
            e.preventDefault();
            createInvite.mutate({ username: inviteUsername.trim() || undefined });
          }}
        >
          <Field label="Reserved username (optional)" className="flex-1 min-w-[160px]">
            {(p) => <Input {...p} value={inviteUsername} onChange={(e) => setInviteUsername(e.target.value)} />}
          </Field>
          <Button type="submit" variant="primary" loading={createInvite.isLoading}>
            Generate invite link
          </Button>
        </form>
        {inviteUrl && (
          <p className="break-all text-muted">
            Latest invite: <span className="text-fg select-all">{inviteUrl}</span>
          </p>
        )}

        <form
          className="flex flex-wrap items-end gap-2 pt-3 border-t border-line"
          onSubmit={(e) => {
            e.preventDefault();
            createUser.mutate({ username: newUsername.trim(), password: newPassword });
          }}
        >
          <Field label="Username" className="flex-1 min-w-[120px]">
            {(p) => <Input {...p} value={newUsername} onChange={(e) => setNewUsername(e.target.value)} required />}
          </Field>
          <Field label="Password (min 8)" className="flex-1 min-w-[120px]">
            {(p) => (
              <Input
                {...p}
                type="password"
                value={newPassword}
                onChange={(e) => setNewPassword(e.target.value)}
                minLength={8}
                required
                autoComplete="new-password"
              />
            )}
          </Field>
          <Button type="submit" loading={createUser.isLoading}>
            Create user
          </Button>
        </form>

        <Notice notice={notice} />

        <Table label="Users">
          <thead>
            <tr>
              <Th>Username</Th>
              <Th>Role</Th>
              <Th align="right">
                <span className="sr-only">Actions</span>
              </Th>
            </tr>
          </thead>
          <tbody>
            {users?.map((u) => (
              <tr key={u.id}>
                <Td className="font-medium">{u.username}</Td>
                <Td>{u.is_admin ? <Badge tone="accent">Admin</Badge> : <Badge>User</Badge>}</Td>
                <Td align="right">
                  {u.id !== user?.id && (
                    <Button
                      size="sm"
                      variant="danger"
                      aria-label={`Remove user ${u.username}`}
                      onClick={() => {
                        if (confirm(`Remove user ${u.username}?`)) deleteUser.mutate({ id: u.id, username: u.username });
                      }}
                    >
                      Remove
                    </Button>
                  )}
                </Td>
              </tr>
            ))}
          </tbody>
        </Table>
      </div>
    </Panel>
  );
}

function HealthSection() {
  const { data: health } = useQuery(keys.adminHealth, () => api.adminHealth(), { refetchInterval: 10000 });
  return (
    <Panel title="System health">
      {health ? (
        <ul className="grid grid-cols-2 sm:grid-cols-4 gap-2">
          {Object.entries(health).map(([name, status]) => {
            const ok = status === 'ok';
            return (
              <li key={name} className="flex items-center gap-2 rounded-md border border-line bg-surface-2 px-2.5 py-2">
                <span className={`w-2 h-2 rounded-full shrink-0 ${ok ? 'bg-ok' : 'bg-danger'}`} aria-hidden="true" />
                <div className="min-w-0">
                  <div className="font-semibold uppercase text-[11px] tracking-wider">{name}</div>
                  <div className={`truncate text-[11.5px] ${ok ? 'text-muted' : 'text-danger'}`}>{status}</div>
                </div>
              </li>
            );
          })}
        </ul>
      ) : (
        <p className="text-muted">Loading health…</p>
      )}
    </Panel>
  );
}

function MaintenanceSection() {
  const qc = useQueryClient();
  const [notice, setNotice] = useState<NoticeValue>(null);
  const rebuildAnalysis = useMutation(() => api.adminRebuildAnalysis(), {
    onMutate: () => setNotice(null),
    onSuccess: () => setNotice({ type: 'success', text: 'Analysis rebuild queued.' }),
    onError: (err: Error) => setNotice(errorNotice('Analysis rebuild failed')(err)),
  });
  const rebuildGenres = useMutation(() => api.adminRebuildGenres(), {
    onMutate: () => setNotice(null),
    onSuccess: () => setNotice({ type: 'success', text: 'Genre rebuild queued.' }),
    onError: (err: Error) => setNotice(errorNotice('Genre rebuild failed')(err)),
  });
  const rebuildGenreStations = useMutation(() => api.adminRebuildGenreStations(), {
    onMutate: () => setNotice(null),
    onSuccess: (data) => {
      void qc.invalidateQueries(keys.stations);
      void qc.invalidateQueries(keys.adminStations);
      setNotice({ type: 'success', text: `Created ${data.created} genre station${data.created === 1 ? '' : 's'}.` });
    },
    onError: (err: Error) => setNotice(errorNotice('Genre station rebuild failed')(err)),
  });

  return (
    <Panel title="Maintenance">
      <div className="flex flex-wrap gap-2 mb-3">
        <Button
          onClick={() => rebuildAnalysis.mutate()}
          loading={rebuildAnalysis.isLoading}
          title="Recompute similarity vectors and clusters (also runs after every scan)"
        >
          Rebuild analysis
        </Button>
        <Button
          onClick={() => {
            if (confirm('Rebuild all genre predictions?')) rebuildGenres.mutate();
          }}
          loading={rebuildGenres.isLoading}
        >
          Rebuild genres
        </Button>
        <Button
          onClick={() => {
            if (confirm('Create missing genre stations?')) rebuildGenreStations.mutate();
          }}
          loading={rebuildGenreStations.isLoading}
        >
          Create genre stations
        </Button>
      </div>
      <Notice notice={notice} />
    </Panel>
  );
}

function StationsSection() {
  const { data: stations } = useQuery(keys.adminStations, () => api.adminStations());
  return (
    <Panel title="Stations & queues">
      <Table label="Stations and queues">
        <thead>
          <tr>
            <Th>Name</Th>
            <Th align="right">Tracks</Th>
            <Th align="right">Played</Th>
            <Th align="right">Remaining</Th>
          </tr>
        </thead>
        <tbody>
          {stations?.map((s) => (
            <tr key={s.id}>
              <Td>{s.name}</Td>
              <Td align="right">{s.track_count}</Td>
              <Td align="right">{s.played_count}</Td>
              <Td align="right">{s.track_count - s.played_count}</Td>
            </tr>
          ))}
          {stations?.length === 0 && <EmptyRow colSpan={4}>No stations.</EmptyRow>}
        </tbody>
      </Table>
    </Panel>
  );
}

/**
 * Start a library scan, follow it live, and browse recent scans. The newest
 * scan is shown by default, so a running scan is picked up again after a
 * reload.
 */
function ScanSection() {
  const qc = useQueryClient();
  const scans = useScans();
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [force, setForce] = useState(false);
  const [notice, setNotice] = useState<NoticeValue>(null);

  const shownId = selectedId ?? scans.data?.[0]?.id ?? null;
  const shown = useScanStatus(shownId);
  const scanInProgress =
    scans.data?.some((j) => !isScanFinished(j.status)) || (shown.data && !isScanFinished(shown.data.status));

  const startScan = useMutation(() => api.triggerScan(force), {
    onMutate: () => setNotice(null),
    onSuccess: (data) => {
      setSelectedId(data.job_id);
      void qc.invalidateQueries(keys.scans);
    },
    onError: (err: Error) => setNotice(errorNotice('Scan could not start')(err)),
  });

  // Refresh the history (and the library) when the followed scan finishes.
  const shownStatus = shown.data?.status;
  useEffect(() => {
    if (shownStatus && isScanFinished(shownStatus)) {
      void qc.invalidateQueries(keys.scans);
      void qc.invalidateQueries(['tracks']);
      void qc.invalidateQueries(keys.albums);
      void qc.invalidateQueries(keys.artists);
    }
  }, [shownStatus, qc]);

  return (
    <Panel
      title="Library scans"
      className="xl:col-span-2"
      actions={
        <>
          <label className="flex items-center gap-1.5 text-muted">
            <input type="checkbox" checked={force} onChange={(e) => setForce(e.target.checked)} className="accent-accent" />
            Re-analyze every file (slow)
          </label>
          <Button
            variant="primary"
            onClick={() => startScan.mutate()}
            loading={startScan.isLoading}
            disabled={!!scanInProgress}
          >
            {scanInProgress ? 'Scan in progress' : 'Rescan library'}
          </Button>
        </>
      }
    >
      <div className="space-y-3">
        <Notice notice={notice} />
        {shown.data && <ScanProgress scan={shown.data} />}
        <Table label="Recent scans">
          <thead>
            <tr>
              <Th>Started</Th>
              <Th>Status</Th>
              <Th align="right">Duration</Th>
              <Th align="right">Found</Th>
              <Th align="right">Imported</Th>
              <Th align="right">Failed</Th>
            </tr>
          </thead>
          <tbody>
            {scans.data?.map((job) => (
              <tr key={job.id} className={job.id === shownId ? 'bg-accent/[0.08]' : 'hover:bg-hover/50'}>
                <Td>
                  <button
                    type="button"
                    className="hover:underline text-left"
                    aria-pressed={job.id === shownId}
                    onClick={() => setSelectedId(job.id)}
                  >
                    {formatDateTime(job.created_at)}
                  </button>
                </Td>
                <Td>
                  <ScanStateBadge status={job.status} />
                </Td>
                <Td align="right">{scanDuration(job) ?? '—'}</Td>
                <Td align="right">{job.stats.total_files}</Td>
                <Td align="right">{job.stats.imported}</Td>
                <Td align="right">{failedFileCount(job)}</Td>
              </tr>
            ))}
            {scans.data?.length === 0 && <EmptyRow colSpan={6}>No scans yet.</EmptyRow>}
          </tbody>
        </Table>
      </div>
    </Panel>
  );
}
