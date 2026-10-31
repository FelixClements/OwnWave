'use client';

import { useState } from 'react';
import type { Playlist } from '@/lib/api';
import { ID_SEED_TYPES, SEED_TYPE_LABELS, type StationFilterForm } from '@/lib/station-seed';
import type { StationSeedType } from '@/lib/station-seed-types';
import { Button } from '@/components/ui/Button';
import { Field, Input, Select } from '@/components/ui/Field';

type RangeKey = 'bpm' | 'energy' | 'valence';

const RANGES: { key: RangeKey; label: string; step: string; max?: string; placeholder: [string, string] }[] = [
  { key: 'bpm', label: 'BPM', step: '1', placeholder: ['0', '300'] },
  { key: 'energy', label: 'Energy', step: '0.01', max: '1', placeholder: ['0', '1'] },
  { key: 'valence', label: 'Valence', step: '0.01', max: '1', placeholder: ['0', '1'] },
];

/**
 * Create/edit form for a station seed. Mount it with a `key` per station so
 * its state resets when switching stations. seedTypes lists the seed types
 * this station may use (playlist stations are shared and admin-only).
 */
export function StationForm({
  title,
  initialName = '',
  initialForm,
  seedTypes: allowedSeedTypes,
  playlists,
  submitLabel,
  submitting,
  error,
  onSubmit,
  onCancel,
}: {
  title: string;
  initialName?: string;
  initialForm: StationFilterForm;
  seedTypes: StationSeedType[];
  playlists: Playlist[];
  submitLabel: string;
  submitting: boolean;
  error?: string | null;
  onSubmit: (name: string, form: StationFilterForm) => void;
  onCancel: () => void;
}) {
  const [name, setName] = useState(initialName);
  const [form, setForm] = useState(initialForm);
  const set = (patch: Partial<StationFilterForm>) => setForm((f) => ({ ...f, ...patch }));

  // Id-based seeds have no picker here; offer them only when the station already uses one.
  const seedTypes = allowedSeedTypes.filter((t) => !ID_SEED_TYPES.includes(t) || t === initialForm.seed_type);
  const onlyPlaylists = seedTypes.length === 1 && seedTypes[0] === 'playlists';
  const togglePlaylist = (id: string) =>
    set({
      playlist_ids: form.playlist_ids.includes(id)
        ? form.playlist_ids.filter((p) => p !== id)
        : [...form.playlist_ids, id],
    });
  const missingPlaylists = form.seed_type === 'playlists' && form.playlist_ids.length === 0;

  return (
    <form
      aria-label={title}
      onSubmit={(e) => {
        e.preventDefault();
        if (name.trim()) onSubmit(name.trim(), form);
      }}
      onKeyDown={(e) => e.key === 'Escape' && onCancel()}
      className="flex flex-col gap-3"
    >
      <div className="flex items-center justify-between">
        <h2 className="text-sm font-semibold">{title}</h2>
        <span className="text-muted text-[11.5px]">Esc to close</span>
      </div>

      <Field label="Name">
        {(p) => <Input {...p} value={name} onChange={(e) => setName(e.target.value)} required autoFocus />}
      </Field>

      <Field label="Seed">
        {(p) => (
          <Select {...p} value={form.seed_type} onChange={(e) => set({ seed_type: e.target.value })} disabled={onlyPlaylists}>
            {!onlyPlaylists && <option value="">Any (filters only)</option>}
            {seedTypes.map((t) => (
              <option key={t} value={t}>
                {SEED_TYPE_LABELS[t]}
              </option>
            ))}
          </Select>
        )}
      </Field>

      {form.seed_type === 'playlists' && (
        <fieldset>
          <legend className="text-[11.5px] font-semibold text-muted mb-1">Playlists</legend>
          {playlists.length === 0 ? (
            <p className="text-muted">No playlists yet. Add Spotify playlists on the Playlists page first.</p>
          ) : (
            <div className="max-h-64 overflow-y-auto space-y-1">
              {playlists.map((p) => (
                <label
                  key={p.id}
                  className="flex items-center gap-2.5 px-2.5 py-1.5 rounded-md border border-line bg-surface-2 cursor-pointer hover:bg-hover"
                >
                  <input
                    type="checkbox"
                    checked={form.playlist_ids.includes(p.id)}
                    onChange={() => togglePlaylist(p.id)}
                    className="w-4 h-4 accent-accent"
                  />
                  <span className="flex-1 min-w-0 truncate font-medium">{p.name}</span>
                  <span className="text-muted tabular-nums text-[11.5px]">
                    {p.matched_count}/{p.track_count}
                  </span>
                </label>
              ))}
            </div>
          )}
          <p className="text-[11.5px] text-muted mt-1">The station plays every local match of the picked playlists.</p>
        </fieldset>
      )}

      {RANGES.map(({ key, label, step, max, placeholder }) => {
        const minKey = `min_${key}` as const;
        const maxKey = `max_${key}` as const;
        return (
          <fieldset key={key} className="grid grid-cols-[64px_1fr_auto_1fr] items-center gap-2">
            <legend className="sr-only">{label} range</legend>
            <span aria-hidden="true" className="text-[11.5px] font-semibold text-muted">
              {label}
            </span>
            <Input
              type="number"
              step={step}
              min="0"
              max={max}
              value={form[minKey]}
              onChange={(e) => set({ [minKey]: e.target.value })}
              placeholder={placeholder[0]}
              aria-label={`Minimum ${label}`}
              className="tabular-nums"
            />
            <span className="text-muted" aria-hidden="true">
              –
            </span>
            <Input
              type="number"
              step={step}
              min="0"
              max={max}
              value={form[maxKey]}
              onChange={(e) => set({ [maxKey]: e.target.value })}
              placeholder={placeholder[1]}
              aria-label={`Maximum ${label}`}
              className="tabular-nums"
            />
          </fieldset>
        );
      })}
      <p className="text-[11.5px] text-muted">Leave a range empty to not filter on it.</p>

      {error && (
        <p role="alert" className="text-danger">
          {error}
        </p>
      )}

      <div className="flex justify-end gap-2">
        <Button onClick={onCancel}>Cancel</Button>
        <Button type="submit" variant="primary" loading={submitting} disabled={!name.trim() || missingPlaylists}>
          {submitLabel}
        </Button>
      </div>
    </form>
  );
}
