'use client';

import { trpc } from '@/lib/trpc/client';

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
  const { data: health } = trpc.adminHealth.useQuery(undefined, { refetchInterval: 10000 });
  const { data: stations } = trpc.adminStations.useQuery();
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

  return (
    <div className="space-y-8">
      <h2 className="text-2xl font-bold">Admin</h2>

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
        </div>
      </section>
    </div>
  );
}
