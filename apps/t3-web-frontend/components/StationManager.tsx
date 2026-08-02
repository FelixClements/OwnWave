'use client';

import { useState } from 'react';
import { trpc } from '@/lib/trpc/client';

export function StationManager() {
  const [name, setName] = useState('');
  const [editing, setEditing] = useState<string | null>(null);
  const [editName, setEditName] = useState('');
  const [previewId, setPreviewId] = useState<string | null>(null);

  const utils = trpc.useContext();
  const { data: stations } = trpc.stations.useQuery();
  const { data: queue } = trpc.queue.useQuery(
    { id: previewId || '' },
    { enabled: !!previewId }
  );

  const create = trpc.createStation.useMutation({
    onSuccess: () => {
      setName('');
      utils.stations.invalidate();
    },
  });

  const update = trpc.updateStation.useMutation({
    onSuccess: () => {
      setEditing(null);
      setEditName('');
      utils.stations.invalidate();
    },
  });

  const remove = trpc.deleteStation.useMutation({
    onSuccess: () => {
      utils.stations.invalidate();
    },
  });

  const handleCreate = (e: React.FormEvent) => {
    e.preventDefault();
    if (!name.trim()) return;
    create.mutate({ name: name.trim() });
  };

  const startEdit = (id: string, currentName: string) => {
    setEditing(id);
    setEditName(currentName);
  };

  const handleUpdate = (e: React.FormEvent) => {
    e.preventDefault();
    if (!editing || !editName.trim()) return;
    update.mutate({ id: editing, name: editName.trim() });
  };

  return (
    <div className="p-6 space-y-6">
      <h2 className="text-2xl font-bold text-spotify-text">Manage Stations</h2>

      <form onSubmit={handleCreate} className="flex gap-2">
        <input
          type="text"
          value={name}
          onChange={(e) => setName(e.target.value)}
          placeholder="New station name"
          className="flex-1 px-3 py-2 rounded bg-spotify-elevated text-spotify-text placeholder-spotify-subdued border border-spotify-border focus:outline-none focus:border-spotify-green"
        />
        <button
          type="submit"
          disabled={create.isLoading}
          className="px-4 py-2 rounded bg-spotify-green text-black font-semibold hover:bg-spotify-green-hover transition disabled:opacity-50"
        >
          Create
        </button>
      </form>

      <ul className="space-y-2">
        {stations?.map((station) => (
          <li
            key={station.id}
            className="flex items-center justify-between p-3 rounded bg-spotify-card"
          >
            {editing === station.id ? (
              <form onSubmit={handleUpdate} className="flex-1 flex gap-2">
                <input
                  type="text"
                  value={editName}
                  onChange={(e) => setEditName(e.target.value)}
                  className="flex-1 px-2 py-1 rounded bg-spotify-elevated text-spotify-text border border-spotify-border"
                />
                <button
                  type="submit"
                  className="px-3 py-1 rounded bg-spotify-green text-black text-sm font-semibold"
                >
                  Save
                </button>
                <button
                  type="button"
                  onClick={() => setEditing(null)}
                  className="px-3 py-1 rounded bg-spotify-elevated text-spotify-text text-sm"
                >
                  Cancel
                </button>
              </form>
            ) : (
              <>
                <span className="text-spotify-text font-medium flex-1">
                  {station.name}
                </span>
                <div className="flex gap-2">
                  <button
                    onClick={() => setPreviewId(station.id)}
                    className="px-3 py-1 rounded bg-spotify-elevated text-spotify-text text-sm hover:bg-spotify-card-hover transition"
                  >
                    Preview
                  </button>
                  <button
                    onClick={() => startEdit(station.id, station.name)}
                    className="px-3 py-1 rounded bg-spotify-elevated text-spotify-text text-sm hover:bg-spotify-card-hover transition"
                  >
                    Edit
                  </button>
                  <button
                    onClick={() => remove.mutate({ id: station.id })}
                    className="px-3 py-1 rounded bg-red-600 text-white text-sm hover:bg-red-700 transition"
                  >
                    Delete
                  </button>
                </div>
              </>
            )}
          </li>
        ))}
      </ul>

      {previewId && (
        <div className="rounded bg-spotify-card p-4">
          <div className="flex items-center justify-between mb-3">
            <h3 className="font-bold text-spotify-text">Preview Queue</h3>
            <button
              onClick={() => setPreviewId(null)}
              className="text-sm text-spotify-subdued hover:text-spotify-text"
            >
              Close
            </button>
          </div>
          {queue && queue.length > 0 ? (
            <ul className="space-y-1">
              {queue.map((track, i) => (
                <li
                  key={track.id}
                  className="flex items-center justify-between text-sm text-spotify-text"
                >
                  <span>
                    {i + 1}. {track.title}
                  </span>
                  <span className="text-spotify-subdued">
                    {track.artist || 'Unknown artist'}
                  </span>
                </li>
              ))}
            </ul>
          ) : (
            <p className="text-spotify-subdued text-sm">No tracks in queue.</p>
          )}
        </div>
      )}
    </div>
  );
}
