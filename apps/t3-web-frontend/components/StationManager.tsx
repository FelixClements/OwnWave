'use client';

import { useEffect, useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { api, type Station, type StationRequest } from '@/lib/api';
import {
  emptyStationFilterForm,
  filterFormToRequest,
  ID_SEED_TYPES,
  parseStationSeed,
  seedToFilterForm,
  type StationFilterForm,
} from '@/lib/station-seed';
import type { StationSeedType } from '@/lib/station-seed-types';
import { keys, useGenres, useQueue, useStationQuery, useStations } from '@/lib/queries';
import { useStation } from '@/lib/station';

const SEED_TYPE_LABELS: Record<StationSeedType, string> = {
  track: 'Track',
  artist: 'Artist',
  album: 'Album',
  cluster: 'Cluster',
  mood: 'Mood',
  genre: 'Genre',
  sub_genre: 'Sub-genre',
  uncategorized: 'Uncategorized',
};

/** Seed type options; id-based types only when the station already uses one. */
function SeedTypeOptions({ current }: { current: string }) {
  const types = (Object.keys(SEED_TYPE_LABELS) as StationSeedType[]).filter(
    (t) => !ID_SEED_TYPES.includes(t) || t === current,
  );
  return (
    <>
      <option value="">Any seed</option>
      {types.map((t) => (
        <option key={t} value={t}>
          {SEED_TYPE_LABELS[t]}
        </option>
      ))}
    </>
  );
}

export function StationManager() {
  const [name, setName] = useState('');
  const [editing, setEditing] = useState<string | null>(null);
  const [editName, setEditName] = useState('');
  const [previewId, setPreviewId] = useState<string | null>(null);
  const [showCreate, setShowCreate] = useState(false);
  const [createError, setCreateError] = useState('');

  const [createFilters, setCreateFilters] = useState<StationFilterForm>(emptyStationFilterForm());
  const [editFilters, setEditFilters] = useState<StationFilterForm>(emptyStationFilterForm());

  const [updateMessage, setUpdateMessage] = useState<string | null>(null);

  const qc = useQueryClient();
  const { selectedStation, setSelectedStation, setIsPlaying } = useStation();
  const { data: stations } = useStations();
  const { data: genres } = useGenres();
  const { data: editingStation } = useStationQuery(editing);
  const { data: queue } = useQueue(previewId);

  const create = useMutation((body: StationRequest) => api.createStation(body), {
    onSuccess: () => {
      setName('');
      setCreateFilters(emptyStationFilterForm());
      setShowCreate(false);
      void qc.invalidateQueries(keys.stations);
    },
    onError: (err: Error) => setCreateError(err.message),
  });

  const update = useMutation(({ id, body }: { id: string; body: StationRequest }) => api.updateStation(id, body), {
    onSuccess: (data, { id }) => {
      setEditing(null);
      setEditName('');
      setUpdateMessage(`Saved "${data.name}" (${data.track_count} tracks)`);
      void qc.invalidateQueries(keys.stations);
      void qc.invalidateQueries(keys.station(id));
      void qc.invalidateQueries(keys.queue(id));
    },
    onError: (err: Error) => alert('Failed to update station: ' + err.message),
  });

  const remove = useMutation((id: string) => api.deleteStation(id), {
    onSuccess: (_, id) => {
      void qc.invalidateQueries(keys.stations);
      if (id === selectedStation) {
        setSelectedStation(null);
        setIsPlaying(false);
      }
    },
    onError: (err: Error) => alert('Failed to delete station: ' + err.message),
  });

  const handleCreate = (e: React.FormEvent) => {
    e.preventDefault();
    if (!name.trim()) {
      setCreateError('Please enter a station name.');
      return;
    }
    setCreateError('');
    create.mutate(filterFormToRequest(name.trim(), createFilters));
  };

  const handleUpdate = (e: React.FormEvent) => {
    e.preventDefault();
    if (!editing || !editName.trim()) return;
    update.mutate({ id: editing, body: filterFormToRequest(editName.trim(), editFilters) });
  };

  const startEdit = (station: Station) => {
    setUpdateMessage(null);
    setEditing(station.id);
  };

  useEffect(() => {
    if (!editing || !editingStation || editingStation.id !== editing) return;
    setEditName(editingStation.name);
    setEditFilters(seedToFilterForm(parseStationSeed(editingStation.seed_features)));
  }, [editing, editingStation]);


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

      {updateMessage && (
        <p className="text-sm text-spotify-green">{updateMessage}</p>
      )}

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
                <SeedTypeOptions current="" />
              </select>
            </div>
            {['genre', 'sub_genre'].includes(createFilters.seed_type) && (
              <>
                <div className="space-y-1">
                  <label htmlFor="create-main-genre" className="text-xs text-spotify-subdued">Main Genre</label>
                  <select
                    id="create-main-genre"
                    value={createFilters.main_genre}
                    onChange={(e) => setCreateFilters({ ...createFilters, main_genre: e.target.value })}
                    className="w-full px-3 py-2 rounded bg-spotify-elevated text-spotify-text border border-spotify-border focus:outline-none focus:border-spotify-green"
                  >
                    <option value="">Select genre</option>
                    {Array.from(new Set((genres || []).map((g) => g.main_genre))).map((g) => (
                      <option key={g} value={g}>{g}</option>
                    ))}
                  </select>
                </div>
                {createFilters.seed_type === 'sub_genre' && (
                  <div className="space-y-1">
                    <label htmlFor="create-sub-genre" className="text-xs text-spotify-subdued">Sub Genre</label>
                    <select
                      id="create-sub-genre"
                      value={createFilters.sub_genre}
                      onChange={(e) => setCreateFilters({ ...createFilters, sub_genre: e.target.value })}
                      className="w-full px-3 py-2 rounded bg-spotify-elevated text-spotify-text border border-spotify-border focus:outline-none focus:border-spotify-green"
                    >
                      <option value="">Select sub-genre</option>
                      {(genres || [])
                        .filter((g) => g.main_genre === createFilters.main_genre && g.sub_genre !== g.main_genre)
                        .map((g) => (
                          <option key={`${g.main_genre}-${g.sub_genre}`} value={g.sub_genre}>{g.sub_genre}</option>
                        ))}
                    </select>
                  </div>
                )}
              </>
            )}
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
                      <SeedTypeOptions current={seedToFilterForm(parseStationSeed(editingStation?.seed_features)).seed_type} />
                    </select>
                  </div>
                  {['genre', 'sub_genre'].includes(editFilters.seed_type) && (
                    <>
                      <div className="space-y-1">
                        <label htmlFor="edit-main-genre" className="text-xs text-spotify-subdued">Main Genre</label>
                        <select
                          id="edit-main-genre"
                          value={editFilters.main_genre}
                          onChange={(e) => setEditFilters({ ...editFilters, main_genre: e.target.value })}
                          className="w-full px-2 py-1 rounded bg-spotify-elevated text-spotify-text border border-spotify-border"
                        >
                          <option value="">Select genre</option>
                          {Array.from(new Set((genres || []).map((g) => g.main_genre))).map((g) => (
                            <option key={g} value={g}>{g}</option>
                          ))}
                        </select>
                      </div>
                      {editFilters.seed_type === 'sub_genre' && (
                        <div className="space-y-1">
                          <label htmlFor="edit-sub-genre" className="text-xs text-spotify-subdued">Sub Genre</label>
                          <select
                            id="edit-sub-genre"
                            value={editFilters.sub_genre}
                            onChange={(e) => setEditFilters({ ...editFilters, sub_genre: e.target.value })}
                            className="w-full px-2 py-1 rounded bg-spotify-elevated text-spotify-text border border-spotify-border"
                          >
                            <option value="">Select sub-genre</option>
                            {(genres || [])
                              .filter((g) => g.main_genre === editFilters.main_genre && g.sub_genre !== g.main_genre)
                              .map((g) => (
                                <option key={`${g.main_genre}-${g.sub_genre}`} value={g.sub_genre}>{g.sub_genre}</option>
                              ))}
                          </select>
                        </div>
                      )}
                    </>
                  )}
                </div>
                <div className="flex gap-2 justify-end">
                    <button
                      type="submit"
                      disabled={update.isLoading}
                      className="px-3 py-1 rounded bg-spotify-green text-black text-sm font-semibold disabled:opacity-50"
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
                        ? 'bg-spotify-elevated text-spotify-text'
                        : 'bg-spotify-elevated text-spotify-text hover:bg-spotify-card-hover'
                    }`}
                  >
                    {previewId === station.id ? 'Close Preview' : 'Preview'}
                  </button>
                  <button
                    onClick={() => startEdit(station)}
                    className="px-3 py-1 rounded bg-spotify-elevated text-spotify-text text-sm hover:bg-spotify-card-hover transition"
                  >
                    Edit
                  </button>
                  <button
                    onClick={() => remove.mutate(station.id)}
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
