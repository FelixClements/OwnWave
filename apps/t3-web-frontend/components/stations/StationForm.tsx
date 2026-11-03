'use client';

import { useState } from 'react';
import type { Playlist } from '@/lib/api';
import { useStationGenres } from '@/lib/queries';
import { Button } from '@/components/ui/Button';
import { Field, Input } from '@/components/ui/Field';

export type StationFormValue = { name: string; playlist_ids: string[]; genres: string[] };

/**
 * Create/edit form for a station: a name, the playlists it draws from and,
 * optionally, the station genres it keeps. Genre counts are for the picked
 * playlists. Mount it with a `key` per station so its state resets.
 */
export function StationForm({
  title,
  initial = { name: '', playlist_ids: [], genres: [] },
  playlists,
  submitLabel,
  submitting,
  error,
  onSubmit,
  onCancel,
}: {
  title: string;
  initial?: StationFormValue;
  playlists: Playlist[];
  submitLabel: string;
  submitting: boolean;
  error?: string | null;
  onSubmit: (value: StationFormValue) => void;
  onCancel: () => void;
}) {
  const [name, setName] = useState(initial.name);
  const [playlistIds, setPlaylistIds] = useState(initial.playlist_ids);
  const [genres, setGenres] = useState(initial.genres);
  const { data: pool } = useStationGenres(playlistIds);

  const toggle = (list: string[], id: string) => (list.includes(id) ? list.filter((x) => x !== id) : [...list, id]);
  // Matched tracks the station would play with the picked genres (a track
  // in two picked genres counts once on the server; this is an upper bound).
  const genreTotal = genres.length
    ? Math.min(pool?.tracks ?? 0, (pool?.genres ?? []).filter((g) => genres.includes(g.id)).reduce((n, g) => n + g.tracks, 0))
    : pool?.tracks ?? 0;

  return (
    <form
      aria-label={title}
      onSubmit={(e) => {
        e.preventDefault();
        if (name.trim() && playlistIds.length) onSubmit({ name: name.trim(), playlist_ids: playlistIds, genres });
      }}
      onKeyDown={(e) => e.key === 'Escape' && onCancel()}
      className="flex flex-col gap-3"
    >
      <div className="flex items-center justify-between">
        <h2 className="text-sm font-semibold">{title}</h2>
        <span className="text-muted text-[11.5px]">Esc to close</span>
      </div>

      <Field label="Name">
        {(p) => <Input {...p} value={name} onChange={(e) => setName(e.target.value)} required autoFocus maxLength={100} />}
      </Field>

      <fieldset>
        <legend className="text-[11.5px] font-semibold text-muted mb-1">Playlists</legend>
        {playlists.length === 0 ? (
          <p className="text-muted">No playlists yet. Add Spotify playlists on the Playlists page first.</p>
        ) : (
          <div className="max-h-56 overflow-y-auto space-y-1">
            {playlists.map((p) => (
              <label
                key={p.id}
                className="flex items-center gap-2.5 px-2.5 py-1.5 rounded-md border border-line bg-surface-2 cursor-pointer hover:bg-hover"
              >
                <input
                  type="checkbox"
                  checked={playlistIds.includes(p.id)}
                  onChange={() => setPlaylistIds(toggle(playlistIds, p.id))}
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
      </fieldset>

      {playlistIds.length > 0 && (
        <fieldset>
          <legend className="text-[11.5px] font-semibold text-muted mb-1">Genres (none picked = all genres)</legend>
          {pool && pool.genres.length === 0 ? (
            <p className="text-muted">These playlists have no genre-scored tracks yet.</p>
          ) : (
            <div className="flex flex-wrap gap-1.5">
              {(pool?.genres ?? []).map((g) => {
                const on = genres.includes(g.id);
                return (
                  <button
                    key={g.id}
                    type="button"
                    aria-pressed={on}
                    onClick={() => setGenres(toggle(genres, g.id))}
                    className={`px-2.5 py-1 rounded-full border text-[12px] ${
                      on ? 'bg-accent text-black border-accent' : 'border-line bg-surface-2 hover:bg-hover'
                    }`}
                  >
                    {g.name} <span className="tabular-nums opacity-70">{g.tracks}</span>
                  </button>
                );
              })}
            </div>
          )}
          <p className="text-[11.5px] text-muted mt-1">
            Plays up to {genreTotal} of {pool?.tracks ?? 0} matched tracks.
          </p>
        </fieldset>
      )}

      {error && (
        <p role="alert" className="text-danger">
          {error}
        </p>
      )}

      <div className="flex justify-end gap-2">
        <Button onClick={onCancel}>Cancel</Button>
        <Button type="submit" variant="primary" loading={submitting} disabled={!name.trim() || playlistIds.length === 0}>
          {submitLabel}
        </Button>
      </div>
    </form>
  );
}
