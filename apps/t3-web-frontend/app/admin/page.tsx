'use client';

import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '@/lib/api';
import { useAuth } from '@/lib/auth';
import { keys } from '@/lib/queries';

/** Mutation that reports its outcome with alert(); replaced by inline status later. */
function useAdminAction<TVars = void>(fn: (vars: TVars) => Promise<unknown>, success: string, failure: string) {
  return useMutation(fn, {
    onSuccess: () => alert(success),
    onError: (err: Error) => alert(`${failure}: ${err.message}`),
  });
}

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

  const scan = useAdminAction(() => api.triggerScan(), 'Scan started.', 'Scan failed');
  const rebuildVectors = useAdminAction(() => api.adminRebuildVectors(), 'Vector rebuild queued.', 'Vector rebuild failed');
  const rebuildClusters = useAdminAction(() => api.adminRebuildClusters(), 'Cluster rebuild queued.', 'Cluster rebuild failed');
  const rebuildGenres = useAdminAction(() => api.adminRebuildGenres(), 'Genre rebuild queued.', 'Genre rebuild failed');
  const rebuildGenreStations = useMutation(() => api.adminRebuildGenreStations(), {
    onSuccess: (data) => {
      void qc.invalidateQueries(keys.stations);
      void qc.invalidateQueries(keys.adminStations);
      alert(`Created ${data.created} genre station${data.created === 1 ? '' : 's'}.`);
    },
    onError: (err: Error) => alert('Genre station rebuild failed: ' + err.message),
  });
  const createInvite = useMutation((vars: { username?: string }) => api.createInvite(vars.username), {
    onSuccess: (data) => {
      setInviteUrl(data.invite_url);
      void navigator.clipboard.writeText(data.invite_url);
      alert('Invite link copied to clipboard.');
    },
    onError: (err: Error) => alert('Invite failed: ' + err.message),
  });
  const createUser = useMutation((vars: { username: string; password: string }) => api.createUser(vars.username, vars.password), {
    onSuccess: () => {
      setNewUsername('');
      setNewPassword('');
      void qc.invalidateQueries(keys.users);
      alert('User created.');
    },
    onError: (err: Error) => alert('Create user failed: ' + err.message),
  });
  const deleteUser = useMutation((vars: { id: string }) => api.deleteUser(vars.id), {
    onSuccess: () => void qc.invalidateQueries(keys.users),
    onError: (err: Error) => alert('Delete user failed: ' + err.message),
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
                              deleteUser.mutate({ id: u.id });
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

      <section>
        <h3 className="text-xl font-bold mb-4">Scan Jobs & Maintenance</h3>
        <div className="flex flex-wrap gap-3">
          <button
            onClick={() => scan.mutate()}
            disabled={scan.isLoading}
            className="px-4 py-2 rounded bg-spotify-green text-black font-semibold hover:bg-spotify-green-hover transition disabled:opacity-50"
          >
            {scan.isLoading ? 'Scanning...' : 'Rescan library'}
          </button>
          <button
            onClick={() => rebuildVectors.mutate()}
            disabled={rebuildVectors.isLoading}
            className="px-4 py-2 rounded bg-spotify-elevated text-spotify-text font-semibold hover:bg-spotify-card-hover transition disabled:opacity-50"
          >
            {rebuildVectors.isLoading ? 'Queueing...' : 'Rebuild vectors'}
          </button>
          <button
            onClick={() => rebuildClusters.mutate()}
            disabled={rebuildClusters.isLoading}
            className="px-4 py-2 rounded bg-spotify-elevated text-spotify-text font-semibold hover:bg-spotify-card-hover transition disabled:opacity-50"
          >
            {rebuildClusters.isLoading ? 'Queueing...' : 'Rebuild clusters'}
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
      </section>
    </div>
  );
}
