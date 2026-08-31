'use client';

import { useState } from 'react';
import { trpc } from '@/lib/trpc/client';
import { useAuth } from '@/lib/auth';

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
  const utils = trpc.useUtils();
  const { data: health } = trpc.adminHealth.useQuery(undefined, { refetchInterval: 10000 });
  const { data: stations } = trpc.adminStations.useQuery();
  const { data: users } = trpc.listUsers.useQuery();
  const [inviteUsername, setInviteUsername] = useState('');
  const [inviteUrl, setInviteUrl] = useState('');
  const [newUsername, setNewUsername] = useState('');
  const [newPassword, setNewPassword] = useState('');

  const scan = trpc.adminScan.useMutation({
    onSuccess: () => alert('Scan started.'),
    onError: (err) => alert('Scan failed: ' + err.message),
  });
  const rebuildVectors = trpc.adminRebuildVectors.useMutation({
    onSuccess: () => alert('Vector rebuild queued.'),
    onError: (err) => alert('Vector rebuild failed: ' + err.message),
  });
  const rebuildClusters = trpc.adminRebuildClusters.useMutation({
    onSuccess: () => alert('Cluster rebuild queued.'),
    onError: (err) => alert('Cluster rebuild failed: ' + err.message),
  });
  const rebuildGenres = trpc.adminRebuildGenres.useMutation({
    onSuccess: () => alert('Genre rebuild queued.'),
    onError: (err) => alert('Genre rebuild failed: ' + err.message),
  });
  const rebuildGenreStations = trpc.adminRebuildGenreStations.useMutation({
    onSuccess: () => alert('Genre station rebuild queued.'),
    onError: (err) => alert('Genre station rebuild failed: ' + err.message),
  });
  const createInvite = trpc.createInvite.useMutation({
    onSuccess: (data) => {
      setInviteUrl(data.invite_url);
      void navigator.clipboard.writeText(data.invite_url);
      alert('Invite link copied to clipboard.');
    },
    onError: (err) => alert('Invite failed: ' + err.message),
  });
  const createUser = trpc.createUser.useMutation({
    onSuccess: () => {
      setNewUsername('');
      setNewPassword('');
      void utils.listUsers.invalidate();
      alert('User created.');
    },
    onError: (err) => alert('Create user failed: ' + err.message),
  });
  const deleteUser = trpc.deleteUser.useMutation({
    onSuccess: () => void utils.listUsers.invalidate(),
    onError: (err) => alert('Delete user failed: ' + err.message),
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
            onClick={() => scan.mutate({})}
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
            onClick={() => { if (confirm('Rebuild all genre stations?')) rebuildGenreStations.mutate(); }}
            disabled={rebuildGenreStations.isLoading}
            className="px-4 py-2 rounded bg-spotify-elevated text-spotify-text font-semibold hover:bg-spotify-card-hover transition disabled:opacity-50"
          >
            {rebuildGenreStations.isLoading ? 'Queueing...' : 'Rebuild genre stations'}
          </button>
        </div>
      </section>
    </div>
  );
}
