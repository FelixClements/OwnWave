'use client';

import { useEffect, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '@/lib/api';
import { useAuth } from '@/lib/auth';
import { isScanFinished, keys, useScanStatus, useScans } from '@/lib/queries';
import { failedFileCount, scanDuration } from '@/lib/scan';
import { ScanProgress, ScanStateBadge } from '@/components/ScanProgress';

type Notice = { type: 'success' | 'error'; text: string } | null;

function NoticeBar({ notice }: { notice: Notice }) {
  if (!notice) return null;
  return (
    <p
      role={notice.type === 'error' ? 'alert' : 'status'}
      className={`rounded px-3 py-2 text-sm ${
        notice.type === 'error'
          ? 'bg-red-900/40 text-red-400 border border-red-400/30'
          : 'bg-green-900/40 text-spotify-green border border-spotify-green/30'
      }`}
    >
      {notice.text}
    </p>
  );
}

const errorNotice = (prefix: string) => (err: Error): Notice => ({ type: 'error', text: `${prefix}: ${err.message}` });

function StatusDot({ ok }: { ok: boolean }) {
  return (
    <span
      className={`inline-block w-2 h-2 rounded-full mr-2 ${
        ok ? 'bg-spotify-green' : 'bg-red-500'
      }`}
    />
  );
}

export default function AdminPage() {
  const { user } = useAuth();
  const qc = useQueryClient();
  const { data: health } = useQuery(keys.adminHealth, () => api.adminHealth(), { refetchInterval: 10000 });
  const { data: stations } = useQuery(keys.adminStations, () => api.adminStations());
  const { data: users } = useQuery(keys.users, () => api.listUsers());
  const [inviteUsername, setInviteUsername] = useState('');
  const [inviteUrl, setInviteUrl] = useState('');
  const [newUsername, setNewUsername] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [userNotice, setUserNotice] = useState<Notice>(null);
  const [maintenanceNotice, setMaintenanceNotice] = useState<Notice>(null);

  const createInvite = useMutation((vars: { username?: string }) => api.createInvite(vars.username), {
    onMutate: () => setUserNotice(null),
    onSuccess: async (data) => {
      setInviteUrl(data.invite_url);
      try {
        await navigator.clipboard.writeText(data.invite_url);
        setUserNotice({ type: 'success', text: 'Invite link created and copied to the clipboard.' });
      } catch {
        // Clipboard access needs HTTPS and permission; the link is shown below.
        setUserNotice({ type: 'success', text: 'Invite link created.' });
      }
    },
    onError: (err: Error) => setUserNotice(errorNotice('Invite failed')(err)),
  });
  const createUser = useMutation((vars: { username: string; password: string }) => api.createUser(vars.username, vars.password), {
    onMutate: () => setUserNotice(null),
    onSuccess: (_, vars) => {
      setNewUsername('');
      setNewPassword('');
      void qc.invalidateQueries(keys.users);
      setUserNotice({ type: 'success', text: `User ${vars.username} created.` });
    },
    onError: (err: Error) => setUserNotice(errorNotice('Create user failed')(err)),
  });
  const deleteUser = useMutation((vars: { id: string; username: string }) => api.deleteUser(vars.id), {
    onMutate: () => setUserNotice(null),
    onSuccess: (_, vars) => {
      void qc.invalidateQueries(keys.users);
      setUserNotice({ type: 'success', text: `User ${vars.username} removed.` });
    },
    onError: (err: Error) => setUserNotice(errorNotice('Remove user failed')(err)),
  });

  const rebuildAnalysis = useMutation(() => api.adminRebuildAnalysis(), {
    onMutate: () => setMaintenanceNotice(null),
    onSuccess: () => setMaintenanceNotice({ type: 'success', text: 'Analysis rebuild queued.' }),
    onError: (err: Error) => setMaintenanceNotice(errorNotice('Analysis rebuild failed')(err)),
  });
  const rebuildGenres = useMutation(() => api.adminRebuildGenres(), {
    onMutate: () => setMaintenanceNotice(null),
    onSuccess: () => setMaintenanceNotice({ type: 'success', text: 'Genre rebuild queued.' }),
    onError: (err: Error) => setMaintenanceNotice(errorNotice('Genre rebuild failed')(err)),
  });
  const rebuildGenreStations = useMutation(() => api.adminRebuildGenreStations(), {
    onMutate: () => setMaintenanceNotice(null),
    onSuccess: (data) => {
      void qc.invalidateQueries(keys.stations);
      void qc.invalidateQueries(keys.adminStations);
      setMaintenanceNotice({
        type: 'success',
        text: `Created ${data.created} genre station${data.created === 1 ? '' : 's'}.`,
      });
    },
    onError: (err: Error) => setMaintenanceNotice(errorNotice('Genre station rebuild failed')(err)),
  });

  if (user && !user.is_admin) {
    return null;
  }

  return (
    <div className="space-y-8">
      <h2 className="text-2xl font-bold">Admin</h2>

      <section>
        <h3 className="text-xl font-bold mb-4">Users</h3>
        <div className="bg-spotify-card rounded-lg p-4 space-y-4 border border-spotify-border">
          <div className="flex flex-wrap gap-3 items-end">
            <div>
              <label className="text-xs text-spotify-subdued block mb-1">Reserved username (optional)</label>
              <input
                type="text"
                value={inviteUsername}
                onChange={(e) => setInviteUsername(e.target.value)}
                placeholder="username"
                className="px-3 py-2 rounded bg-spotify-elevated border border-spotify-border"
              />
            </div>
            <button
              onClick={() =>
                createInvite.mutate({
                  username: inviteUsername.trim() || undefined,
                })
              }
              disabled={createInvite.isLoading}
              className="px-4 py-2 rounded bg-spotify-green text-black font-semibold hover:bg-spotify-green-hover transition disabled:opacity-50"
            >
              {createInvite.isLoading ? 'Generating...' : 'Generate invite link'}
            </button>
          </div>
          <NoticeBar notice={userNotice} />
          {inviteUrl && (
            <p className="text-sm text-spotify-subdued break-all">
              Latest invite: <span className="text-spotify-text">{inviteUrl}</span>
            </p>
          )}

          <form
            className="flex flex-wrap gap-3 items-end pt-2 border-t border-spotify-border"
            onSubmit={(e) => {
              e.preventDefault();
              createUser.mutate({ username: newUsername.trim(), password: newPassword });
            }}
          >
            <div>
              <label className="text-xs text-spotify-subdued block mb-1">Username</label>
              <input
                type="text"
                value={newUsername}
                onChange={(e) => setNewUsername(e.target.value)}
                required
                className="px-3 py-2 rounded bg-spotify-elevated border border-spotify-border"
              />
            </div>
            <div>
              <label className="text-xs text-spotify-subdued block mb-1">Password</label>
              <input
                type="password"
                value={newPassword}
                onChange={(e) => setNewPassword(e.target.value)}
                minLength={8}
                required
                className="px-3 py-2 rounded bg-spotify-elevated border border-spotify-border"
              />
            </div>
            <button
              type="submit"
              disabled={createUser.isLoading}
              className="px-4 py-2 rounded bg-spotify-elevated text-spotify-text font-semibold hover:bg-spotify-card-hover transition disabled:opacity-50"
            >
              {createUser.isLoading ? 'Creating...' : 'Create user'}
            </button>
          </form>

          <div className="overflow-hidden rounded border border-spotify-border">
            <table className="w-full text-sm">
              <thead className="bg-spotify-elevated text-spotify-subdued">
                <tr>
                  <th className="text-left p-3">Username</th>
                  <th className="text-left p-3">Role</th>
                  <th className="text-right p-3">Actions</th>
                </tr>
              </thead>
              <tbody>
                {users?.map((u) => (
                  <tr key={u.id} className="border-t border-spotify-border">
                    <td className="p-3">{u.username}</td>
                    <td className="p-3">{u.is_admin ? 'Admin' : 'User'}</td>
                    <td className="p-3 text-right">
                      {u.id !== user?.id && (
                        <button
                          onClick={() => {
                            if (confirm(`Remove user ${u.username}?`)) {
                              deleteUser.mutate({ id: u.id, username: u.username });
                            }
                          }}
                          className="text-red-400 hover:text-red-300 text-sm"
                        >
                          Remove
                        </button>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      </section>

      <section>
        <h3 className="text-xl font-bold mb-4">System Health</h3>
        <div className="grid grid-cols-2 sm:grid-cols-4 gap-4">
          {health &&
            Object.entries(health).map(([name, status]) => (
              <div
                key={name}
                className="bg-spotify-card rounded-lg p-4 flex items-center"
              >
                <StatusDot ok={status === 'ok'} />
                <div>
                  <div className="text-sm font-semibold uppercase">{name}</div>
                  <div className="text-xs text-spotify-subdued">{status}</div>
                </div>
              </div>
            ))}
          {!health && <p className="text-spotify-subdued col-span-full">Loading health...</p>}
        </div>
      </section>

      <section>
        <h3 className="text-xl font-bold mb-4">Stations & Queues</h3>
        <div className="bg-spotify-card rounded-lg overflow-hidden">
          <table className="w-full text-sm">
            <thead className="bg-spotify-elevated text-spotify-subdued">
              <tr>
                <th className="text-left p-3">Name</th>
                <th className="text-right p-3">Queue tracks</th>
                <th className="text-right p-3">Played</th>
                <th className="text-right p-3">Remaining</th>
              </tr>
            </thead>
            <tbody>
              {stations?.map((s) => (
                <tr key={s.id} className="border-b border-spotify-border last:border-0">
                  <td className="p-3">{s.name}</td>
                  <td className="text-right p-3">{s.track_count}</td>
                  <td className="text-right p-3">{s.played_count}</td>
                  <td className="text-right p-3">{s.track_count - s.played_count}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>

      <ScanSection />

      <section>
        <h3 className="text-xl font-bold mb-4">Maintenance</h3>
        <div className="space-y-3">
          <div className="flex flex-wrap gap-3">
            <button
              onClick={() => rebuildAnalysis.mutate()}
              disabled={rebuildAnalysis.isLoading}
              title="Recompute similarity vectors and clusters (also runs after every scan)"
              className="px-4 py-2 rounded bg-spotify-elevated text-spotify-text font-semibold hover:bg-spotify-card-hover transition disabled:opacity-50"
            >
              {rebuildAnalysis.isLoading ? 'Queueing...' : 'Rebuild analysis'}
            </button>
            <button
              onClick={() => { if (confirm('Rebuild all genre predictions?')) rebuildGenres.mutate(); }}
              disabled={rebuildGenres.isLoading}
              className="px-4 py-2 rounded bg-spotify-elevated text-spotify-text font-semibold hover:bg-spotify-card-hover transition disabled:opacity-50"
            >
              {rebuildGenres.isLoading ? 'Queueing...' : 'Rebuild genres'}
            </button>
            <button
              onClick={() => { if (confirm('Create missing genre stations?')) rebuildGenreStations.mutate(); }}
              disabled={rebuildGenreStations.isLoading}
              className="px-4 py-2 rounded bg-spotify-elevated text-spotify-text font-semibold hover:bg-spotify-card-hover transition disabled:opacity-50"
            >
              {rebuildGenreStations.isLoading ? 'Creating...' : 'Create genre stations'}
            </button>
          </div>
          <NoticeBar notice={maintenanceNotice} />
        </div>
      </section>
    </div>
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
  const [notice, setNotice] = useState<Notice>(null);

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
    <section>
      <h3 className="text-xl font-bold mb-4">Library Scans</h3>
      <div className="space-y-4">
        <div className="flex flex-wrap items-center gap-4">
          <button
            onClick={() => startScan.mutate()}
            disabled={startScan.isLoading || !!scanInProgress}
            className="px-4 py-2 rounded bg-spotify-green text-black font-semibold hover:bg-spotify-green-hover transition disabled:opacity-50"
          >
            {startScan.isLoading ? 'Starting...' : scanInProgress ? 'Scan in progress' : 'Rescan library'}
          </button>
          <label className="flex items-center gap-2 text-sm text-spotify-subdued">
            <input type="checkbox" checked={force} onChange={(e) => setForce(e.target.checked)} />
            Re-analyze every file (slow)
          </label>
        </div>
        <NoticeBar notice={notice} />

        {shown.data && <ScanProgress scan={shown.data} />}

        {scans.data && scans.data.length > 0 && (
          <div className="bg-spotify-card rounded-lg overflow-x-auto border border-spotify-border">
            <table className="w-full text-sm">
              <thead className="bg-spotify-elevated text-spotify-subdued">
                <tr>
                  <th className="text-left p-3">Started</th>
                  <th className="text-left p-3">Status</th>
                  <th className="text-right p-3">Duration</th>
                  <th className="text-right p-3">Found</th>
                  <th className="text-right p-3">Imported</th>
                  <th className="text-right p-3">Failed</th>
                </tr>
              </thead>
              <tbody>
                {scans.data.map((job) => (
                  <tr
                    key={job.id}
                    onClick={() => setSelectedId(job.id)}
                    className={`border-t border-spotify-border cursor-pointer hover:bg-spotify-card-hover ${
                      job.id === shownId ? 'bg-spotify-elevated' : ''
                    }`}
                  >
                    <td className="p-3">
                      <button className="text-left" aria-pressed={job.id === shownId}>
                        {job.created_at ? new Date(job.created_at).toLocaleString() : '—'}
                      </button>
                    </td>
                    <td className="p-3"><ScanStateBadge status={job.status} /></td>
                    <td className="p-3 text-right">{scanDuration(job) ?? '—'}</td>
                    <td className="p-3 text-right">{job.stats.total_files}</td>
                    <td className="p-3 text-right">{job.stats.imported}</td>
                    <td className="p-3 text-right">{failedFileCount(job)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
        {scans.data?.length === 0 && <p className="text-spotify-subdued text-sm">No scans yet.</p>}
      </div>
    </section>
  );
}
