'use client';

import { useState } from 'react';
import type { Genre } from '@/lib/api';
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
 * its state resets when switching stations.
 */
export function StationForm({
  title,
  initialName = '',
  initialForm,
  genres,
  submitLabel,
  submitting,
  error,
  onSubmit,
  onCancel,
}: {
  title: string;
  initialName?: string;
  initialForm: StationFilterForm;
  genres: Genre[];
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
  const seedTypes = (Object.keys(SEED_TYPE_LABELS) as StationSeedType[]).filter(
    (t) => !ID_SEED_TYPES.includes(t) || t === initialForm.seed_type,
  );
  const mainGenres = Array.from(new Set(genres.map((g) => g.main_genre)));
  const subGenres = genres.filter((g) => g.main_genre === form.main_genre && g.sub_genre !== g.main_genre);
  const needsGenre = form.seed_type === 'genre' || form.seed_type === 'sub_genre';

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
          <Select {...p} value={form.seed_type} onChange={(e) => set({ seed_type: e.target.value })}>
            <option value="">Any (filters only)</option>
            {seedTypes.map((t) => (
              <option key={t} value={t}>
                {SEED_TYPE_LABELS[t]}
              </option>
            ))}
          </Select>
        )}
      </Field>

      {needsGenre && (
        <div className="grid grid-cols-2 gap-2">
          <Field label="Main genre">
            {(p) => (
              <Select {...p} value={form.main_genre} onChange={(e) => set({ main_genre: e.target.value, sub_genre: '' })} required>
                <option value="">Select genre</option>
                {mainGenres.map((g) => (
                  <option key={g} value={g}>
                    {g}
                  </option>
                ))}
              </Select>
            )}
          </Field>
          {form.seed_type === 'sub_genre' && (
            <Field label="Sub-genre">
              {(p) => (
                <Select {...p} value={form.sub_genre} onChange={(e) => set({ sub_genre: e.target.value })} required>
                  <option value="">Select sub-genre</option>
                  {subGenres.map((g) => (
                    <option key={g.sub_genre} value={g.sub_genre}>
                      {g.sub_genre}
                    </option>
                  ))}
                </Select>
              )}
            </Field>
          )}
        </div>
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
        <Button type="submit" variant="primary" loading={submitting} disabled={!name.trim()}>
          {submitLabel}
        </Button>
      </div>
    </form>
  );
}
