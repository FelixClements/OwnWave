'use client';

import { useState } from 'react';
import { trpc } from '@/lib/trpc/client';

export function StationManager() {
  const [name, setName] = useState('');
  const [editing, setEditing] = useState<string | null>(null);
  const [editName, setEditName] = useState('');
  const [previewId, setPreviewId] = useState<string | null>(null);
  const [showCreate, setShowCreate] = useState(false);
  const [createError, setCreateError] = useState('');

  const [createFilters, setCreateFilters] = useState({
    min_bpm: '',
    max_bpm: '',
    min_energy: '',
    max_energy: '',
    min_valence: '',
    max_valence: '',
    seed_type: '',
  });
  const [editFilters, setEditFilters] = useState({
    min_bpm: '',
    max_bpm: '',
    min_energy: '',
    max_energy: '',
    min_valence: '',
    max_valence: '',
    seed_type: '',
  });

  const utils = trpc.useContext();
  const { data: stations } = trpc.stations.useQuery();
  const { data: queue } = trpc.queue.useQuery(
    { id: previewId || '' },
    { enabled: !!previewId }
  );

  const create = trpc.createStation.useMutation({
    onSuccess: () => {
      setName('');
      setCreateFilters({
        min_bpm: '',
        max_bpm: '',
        min_energy: '',
        max_energy: '',
        min_valence: '',
        max_valence: '',
        seed_type: '',
      });
      setShowCreate(false);
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
    onError: (err) => {
      console.error('delete station failed', err);
      alert('Failed to delete station: ' + err.message);
    },
  });

  const handleCreate = (e: React.FormEvent) => {
    e.preventDefault();
    if (!name.trim()) {
      setCreateError('Please enter a station name.');
      return;
    }
    setCreateError('');
    create.mutate({
      name: name.trim(),
      min_bpm: toNum(createFilters.min_bpm),
      max_bpm: toNum(createFilters.max_bpm),
      min_energy: toNum(createFilters.min_energy),
      max_energy: toNum(createFilters.max_energy),
      min_valence: toNum(createFilters.min_valence),
      max_valence: toNum(createFilters.max_valence),
      seed_type: createFilters.seed_type as any || undefined,
    });
  };

  const handleUpdate = (e: React.FormEvent) => {
    e.preventDefault();
    if (!editing || !editName.trim()) return;
    update.mutate({
      id: editing,
      name: editName.trim(),
      min_bpm: toNum(editFilters.min_bpm),
      max_bpm: toNum(editFilters.max_bpm),
      min_energy: toNum(editFilters.min_energy),
      max_energy: toNum(editFilters.max_energy),
      min_valence: toNum(editFilters.min_valence),
      max_valence: toNum(editFilters.max_valence),
      seed_type: editFilters.seed_type as any || undefined,
    });
  };

  const startEdit = (id: string, currentName: string) => {
    setEditing(id);
    setEditName(currentName);
    setEditFilters({
      min_bpm: '',
      max_bpm: '',
      min_energy: '',
      max_energy: '',
      min_valence: '',
      max_valence: '',
      seed_type: '',
    });
  };

  function toNum(value: string) {
    const n = parseFloat(value);
    return isNaN(n) ? undefined : n;
  }

  return (
    <div className="p-4 md:p-6 space-y-6">
      <div className="flex items-center justify-between">
        <h2 className="text-2xl font-bold text-spotify-text">Manage Stations</h2>
        <button
          onClick={() => setShowCreate(!showCreate)}
          className="px-4 py-2 rounded bg-spotify-green text-black font-semibold hover:bg-spotify-green-hover transition"
        >
          {showCreate ? 'Cancel' : 'New Station'}
        </button>
      </div>

      {showCreate && (
        <form onSubmit={handleCreate} className="space-y-3 p-4 rounded bg-spotify-card">
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <div className="space-y-1">
              <label htmlFor="create-name" className="text-xs text-spotify-subdued">Station Name</label>
              <input
                id="create-name"
                type="text"
                required
                value={name}
                onChange={(e) => {
                  setName(e.target.value);
                  if (createError) setCreateError('');
                }}
                placeholder="Name"
                className="w-full px-3 py-2 rounded bg-spotify-elevated text-spotify-text placeholder-spotify-subdued border border-spotify-border focus:outline-none focus:border-spotify-green"
              />
            </div>
            <div className="space-y-1">
              <label htmlFor="create-seed-type" className="text-xs text-spotify-subdued">Seed Type</label>
              <select
                id="create-seed-type"
                required
                value={createFilters.seed_type}
                onChange={(e) => setCreateFilters({ ...createFilters, seed_type: e.target.value })}
                className="w-full px-3 py-2 rounded bg-spotify-elevated text-spotify-text border border-spotify-border focus:outline-none focus:border-spotify-green"
              >
                <option value="">Any seed</option>
                <option value="track">Track</option>
                <option value="artist">Artist</option>
                <option value="album">Album</option>
                <option value="cluster">Cluster</option>
                <option value="mood">Mood</option>
              </select>
            </div>
          </div>
          {createError && (
            <p className="text-red-500 text-sm">{createError}</p>
          )}
          <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
            <div className="space-y-2">
              <div className="space-y-1">
                <label htmlFor="create-min-bpm" className="text-xs text-spotify-subdued">Min BPM</label>
                <input
                  id="create-min-bpm"
                  type="number"
                  required
                  value={createFilters.min_bpm}
                  onChange={(e) => setCreateFilters({ ...createFilters, min_bpm: e.target.value })}
                  placeholder="0"
                  className="w-full px-3 py-2 rounded bg-spotify-elevated text-spotify-text placeholder-spotify-subdued border border-spotify-border focus:outline-none focus:border-spotify-green"
                />
              </div>
              <div className="space-y-1">
                <label htmlFor="create-max-bpm" className="text-xs text-spotify-subdued">Max BPM</label>
                <input
                  id="create-max-bpm"
                  type="number"
                  required
                  value={createFilters.max_bpm}
                  onChange={(e) => setCreateFilters({ ...createFilters, max_bpm: e.target.value })}
                  placeholder="300"
                  className="w-full px-3 py-2 rounded bg-spotify-elevated text-spotify-text placeholder-spotify-subdued border border-spotify-border focus:outline-none focus:border-spotify-green"
                />
              </div>
            </div>
            <div className="space-y-2">
              <div className="space-y-1">
                <label htmlFor="create-min-energy" className="text-xs text-spotify-subdued">Min Energy</label>
                <input
                  id="create-min-energy"
                  type="number"
                  step="0.01"
                  required
                  min="0"
                  max="1"
                  value={createFilters.min_energy}
                  onChange={(e) => setCreateFilters({ ...createFilters, min_energy: e.target.value })}
                  placeholder="0"
                  className="w-full px-3 py-2 rounded bg-spotify-elevated text-spotify-text placeholder-spotify-subdued border border-spotify-border focus:outline-none focus:border-spotify-green"
                />
              </div>
              <div className="space-y-1">
                <label htmlFor="create-max-energy" className="text-xs text-spotify-subdued">Max Energy</label>
                <input
                  id="create-max-energy"
                  type="number"
                  step="0.01"
                  required
                  min="0"
                  max="1"
                  value={createFilters.max_energy}
                  onChange={(e) => setCreateFilters({ ...createFilters, max_energy: e.target.value })}
                  placeholder="1"
                  className="w-full px-3 py-2 rounded bg-spotify-elevated text-spotify-text placeholder-spotify-subdued border border-spotify-border focus:outline-none focus:border-spotify-green"
                />
              </div>
            </div>
            <div className="space-y-2">
              <div className="space-y-1">
                <label htmlFor="create-min-valence" className="text-xs text-spotify-subdued">Min Valence</label>
                <input
                  id="create-min-valence"
                  type="number"
                  step="0.01"
                  required
                  min="0"
                  max="1"
                  value={createFilters.min_valence}
                  onChange={(e) => setCreateFilters({ ...createFilters, min_valence: e.target.value })}
                  placeholder="0"
                  className="w-full px-3 py-2 rounded bg-spotify-elevated text-spotify-text placeholder-spotify-subdued border border-spotify-border focus:outline-none focus:border-spotify-green"
                />
              </div>
              <div className="space-y-1">
                <label htmlFor="create-max-valence" className="text-xs text-spotify-subdued">Max Valence</label>
                <input
                  id="create-max-valence"
                  type="number"
                  step="0.01"
                  required
                  min="0"
                  max="1"
                  value={createFilters.max_valence}
                  onChange={(e) => setCreateFilters({ ...createFilters, max_valence: e.target.value })}
                  placeholder="1"
                  className="w-full px-3 py-2 rounded bg-spotify-elevated text-spotify-text placeholder-spotify-subdued border border-spotify-border focus:outline-none focus:border-spotify-green"
                />
              </div>
            </div>
          </div>
          <div className="flex justify-end">
            <button
              type="submit"
              disabled={create.isLoading}
              className="px-4 py-2 rounded bg-spotify-green text-black font-semibold hover:bg-spotify-green-hover transition disabled:opacity-50"
            >
              Create
            </button>
          </div>
        </form>
      )}

      <ul className="space-y-2">
        {stations?.map((station) => (
          <li
            key={station.id}
            className="flex flex-col sm:flex-row items-start sm:items-center justify-between p-3 rounded bg-spotify-card gap-2"
          >
            {editing === station.id ? (
              <form onSubmit={handleUpdate} className="flex-1 flex flex-col gap-2">
                <input
                  type="text"
                  value={editName}
                  onChange={(e) => setEditName(e.target.value)}
                  className="w-full px-2 py-1 rounded bg-spotify-elevated text-spotify-text border border-spotify-border"
                />
                <div className="grid grid-cols-2 sm:grid-cols-3 gap-2">
                  <div className="space-y-1">
                    <label htmlFor="edit-min-bpm" className="text-xs text-spotify-subdued">Min BPM</label>
                    <input
                      id="edit-min-bpm"
                      type="number"
                      value={editFilters.min_bpm}
                      onChange={(e) => setEditFilters({ ...editFilters, min_bpm: e.target.value })}
                      placeholder="Min BPM"
                      className="w-full px-2 py-1 rounded bg-spotify-elevated text-spotify-text border border-spotify-border"
                    />
                  </div>
                  <div className="space-y-1">
                    <label htmlFor="edit-max-bpm" className="text-xs text-spotify-subdued">Max BPM</label>
                    <input
                      id="edit-max-bpm"
                      type="number"
                      value={editFilters.max_bpm}
                      onChange={(e) => setEditFilters({ ...editFilters, max_bpm: e.target.value })}
                      placeholder="Max BPM"
                      className="w-full px-2 py-1 rounded bg-spotify-elevated text-spotify-text border border-spotify-border"
                    />
                  </div>
                  <div className="space-y-1">
                    <label htmlFor="edit-min-energy" className="text-xs text-spotify-subdued">Min Energy</label>
                    <input
                      id="edit-min-energy"
                      type="number"
                      step="0.01"
                      min="0"
                      max="1"
                      value={editFilters.min_energy}
                      onChange={(e) => setEditFilters({ ...editFilters, min_energy: e.target.value })}
                      placeholder="Min energy"
                      className="w-full px-2 py-1 rounded bg-spotify-elevated text-spotify-text border border-spotify-border"
                    />
                  </div>
                  <div className="space-y-1">
                    <label htmlFor="edit-max-energy" className="text-xs text-spotify-subdued">Max Energy</label>
                    <input
                      id="edit-max-energy"
                      type="number"
                      step="0.01"
                      min="0"
                      max="1"
                      value={editFilters.max_energy}
                      onChange={(e) => setEditFilters({ ...editFilters, max_energy: e.target.value })}
                      placeholder="Max energy"
                      className="w-full px-2 py-1 rounded bg-spotify-elevated text-spotify-text border border-spotify-border"
                    />
                  </div>
                  <div className="space-y-1">
                    <label htmlFor="edit-min-valence" className="text-xs text-spotify-subdued">Min Valence</label>
                    <input
                      id="edit-min-valence"
                      type="number"
                      step="0.01"
                      min="0"
                      max="1"
                      value={editFilters.min_valence}
                      onChange={(e) => setEditFilters({ ...editFilters, min_valence: e.target.value })}
                      placeholder="Min valence"
                      className="w-full px-2 py-1 rounded bg-spotify-elevated text-spotify-text border border-spotify-border"
                    />
                  </div>
                  <div className="space-y-1">
                    <label htmlFor="edit-max-valence" className="text-xs text-spotify-subdued">Max Valence</label>
                    <input
                      id="edit-max-valence"
                      type="number"
                      step="0.01"
                      min="0"
                      max="1"
                      value={editFilters.max_valence}
                      onChange={(e) => setEditFilters({ ...editFilters, max_valence: e.target.value })}
                      placeholder="Max valence"
                      className="w-full px-2 py-1 rounded bg-spotify-elevated text-spotify-text border border-spotify-border"
                    />
                  </div>
                  <div className="space-y-1">
                    <label htmlFor="edit-seed-type" className="text-xs text-spotify-subdued">Seed Type</label>
                    <select
                      id="edit-seed-type"
                      value={editFilters.seed_type}
                      onChange={(e) => setEditFilters({ ...editFilters, seed_type: e.target.value })}
                      className="w-full px-2 py-1 rounded bg-spotify-elevated text-spotify-text border border-spotify-border"
                    >
                      <option value="">Any seed</option>
                      <option value="track">Track</option>
                      <option value="artist">Artist</option>
                      <option value="album">Album</option>
                      <option value="cluster">Cluster</option>
                      <option value="mood">Mood</option>
                    </select>
                  </div>
                </div>
                <div className="flex gap-2 justify-end">
                  <button
                    type="submit"
                    className="px-3 py-1 rounded bg-spotify-green text-black text-sm font-semibold"
                  >
                    Save
                  </button>
                  <button
                    type="button"
                    onClick={() => setEditing(null)}
                    className="px-3 py-1 rounded bg-spotify-elevated text-spotify-text text-sm w-full sm:w-auto"
                  >
                    Cancel
                  </button>
                </div>
              </form>
            ) : (
              <>
                <span className="text-spotify-text font-medium flex-1 break-all">
                  {station.name}
                </span>
                <div className="flex flex-wrap gap-2">
                  <button
                    onClick={() => setPreviewId(previewId === station.id ? null : station.id)}
                    className={`px-3 py-1 rounded text-sm transition ${
                      previewId === station.id
                        ? 'bg-spotify-green text-black'
                        : 'bg-spotify-elevated text-spotify-text hover:bg-spotify-card-hover'
                    }`}
                  >
                    {previewId === station.id ? 'Close Preview' : 'Preview'}
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
