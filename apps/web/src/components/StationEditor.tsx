import { useEffect, useState } from 'react';
import { Sparkles } from 'lucide-react';
import type { Station } from '@/api/types';
import { plural } from '@/lib/format';
import { usePlaylists, usePoolGenres, useSaveStation } from '@/lib/queries';
import { Button, Chip, ErrorText, Input, Modal } from './ui';

/**
 * Create or edit a station: a name, the playlists it draws from (picked
 * ones, or every playlist including ones added later), and optionally the
 * genres it keeps (counts are for those playlists), and Enhance, which mixes
 * in library songs that sound like the playlists. A genre station keeps
 * itself up to date, so only its name changes here.
 */
export function StationEditor({
  open,
  onClose,
  station,
  initial,
  onSaved,
}: {
  open: boolean;
  onClose: () => void;
  /** The station to edit; without it the editor creates one. */
  station?: Station;
  /** Prefill for a new station. */
  initial?: { name?: string; playlist_ids?: string[]; genres?: string[]; all_playlists?: boolean; enhance?: boolean };
  onSaved?: (s: Station) => void;
}) {
  const { data: playlists = [] } = usePlaylists();
  const save = useSaveStation();
  const [name, setName] = useState('');
  const [playlistIds, setPlaylistIds] = useState<string[]>([]);
  const [allPlaylists, setAllPlaylists] = useState(false);
  const [genres, setGenres] = useState<string[]>([]);
  const [enhance, setEnhance] = useState(false);
  const genreStation = station?.kind === 'genre';
  const pool = usePoolGenres(genreStation ? [] : allPlaylists ? 'all' : playlistIds);

  useEffect(() => {
    if (!open) return;
    const from = station ?? initial;
    setName(from?.name ?? '');
    setAllPlaylists(from?.all_playlists ?? false);
    setPlaylistIds(from?.all_playlists ? [] : (from?.playlist_ids ?? []));
    setGenres(from?.genres ?? []);
    setEnhance(from?.enhance ?? false);
    save.reset();
  }, [open, station?.id]);

  const toggle = (list: string[], id: string) => (list.includes(id) ? list.filter((x) => x !== id) : [...list, id]);
  const poolGenres = pool.data?.genres ?? [];
  // A track in two picked genres counts twice here: an upper bound.
  const plays = genres.length
    ? Math.min(pool.data?.tracks ?? 0, poolGenres.filter((g) => genres.includes(g.id)).reduce((n, g) => n + g.tracks, 0))
    : (pool.data?.tracks ?? 0);
  const hasPool = allPlaylists || playlistIds.length > 0;
  const canSave = name.trim() && (genreStation || hasPool);
  const body = genreStation
    ? { name: name.trim() }
    : allPlaylists
      ? { name: name.trim(), all_playlists: true, playlist_ids: [], genres, enhance }
      : { name: name.trim(), all_playlists: false, playlist_ids: playlistIds, genres, enhance };

  const submit = () =>
    save.mutate(
      { id: station?.id, body },
      {
        onSuccess: (s) => {
          onSaved?.(s);
          onClose();
        },
      },
    );

  return (
    <Modal
      title={station ? 'Edit station' : 'New station'}
      open={open}
      onClose={onClose}
      wide
      footer={
        <>
          <Button variant="ghost" onClick={onClose}>
            Cancel
          </Button>
          <Button variant="primary" onClick={submit} disabled={!canSave} loading={save.isPending}>
            {station ? 'Save' : 'Create station'}
          </Button>
        </>
      }
    >
      <form
        className="space-y-6"
        onSubmit={(e) => {
          e.preventDefault();
          if (canSave) submit();
        }}
      >
        <Input label="Name" value={name} onChange={(e) => setName(e.target.value)} maxLength={100} autoFocus required placeholder="Late night techno" />

        {genreStation ? (
          <p className="text-sm text-muted">
            A genre station plays every song in your library with this main genre and keeps itself up to date, so only its name changes. Admins can hide it
            or give it back its genre's name on the Admin page.
          </p>
        ) : (
          <>
            <fieldset>
              <legend className="mb-2 text-[13px] font-semibold">Playlists</legend>
              {playlists.length > 0 && (
                <label className="mb-1 flex cursor-pointer items-center gap-3 rounded-md px-3 py-2 hover:bg-card">
                  <input
                    type="checkbox"
                    checked={allPlaylists}
                    onChange={() => setAllPlaylists(!allPlaylists)}
                    className="h-4 w-4 accent-[#2dd4bf]"
                  />
                  <span className="min-w-0 flex-1">
                    <span className="block font-medium">All playlists</span>
                    <span className="block text-[12px] text-muted">Including ones added later</span>
                  </span>
                </label>
              )}
              {playlists.length === 0 ? (
                <p className="text-sm text-muted">No playlists yet. An admin adds Spotify playlists on the Playlists page.</p>
              ) : allPlaylists ? null : (
                <div className="grid max-h-60 gap-1 overflow-y-auto sm:grid-cols-2">
                  {playlists.map((p) => (
                    <label key={p.id} className="flex cursor-pointer items-center gap-3 rounded-md px-3 py-2 hover:bg-card">
                      <input
                        type="checkbox"
                        checked={playlistIds.includes(p.id)}
                        onChange={() => setPlaylistIds(toggle(playlistIds, p.id))}
                        className="h-4 w-4 accent-[#2dd4bf]"
                      />
                      <span className="min-w-0 flex-1">
                        <span className="block truncate font-medium">{p.name}</span>
                        <span className="block text-[12px] text-muted">
                          {p.matched_count} of {p.track_count} in your library
                        </span>
                      </span>
                    </label>
                  ))}
                </div>
              )}
            </fieldset>

            {hasPool && (
              <fieldset>
                <legend className="mb-2 text-[13px] font-semibold">
                  Genres <span className="font-normal text-muted">· none picked plays every genre</span>
                </legend>
                {pool.data && poolGenres.length === 0 ? (
                  <p className="text-sm text-muted">No genre-scored tracks in these playlists yet.</p>
                ) : (
                  <div className="flex flex-wrap gap-2">
                    {poolGenres.map((g) => (
                      <Chip key={g.id} selected={genres.includes(g.id)} onClick={() => setGenres(toggle(genres, g.id))}>
                        {g.name} <span className="tabular-nums opacity-60">{g.tracks}</span>
                      </Chip>
                    ))}
                  </div>
                )}
                <p className="mt-3 text-[13px] text-muted">
                  Plays {genres.length ? 'up to ' : ''}
                  {plural(plays, 'track')} of {plural(pool.data?.tracks ?? 0, 'track')}
                  {enhance ? ', plus similar songs from your library' : ''}.
                </p>
              </fieldset>
            )}

            {hasPool && (
              <label className="flex cursor-pointer items-center gap-3 rounded-md px-3 py-2 hover:bg-card">
                <input type="checkbox" checked={enhance} onChange={() => setEnhance(!enhance)} className="h-4 w-4 accent-[#2dd4bf]" />
                <span className="min-w-0 flex-1">
                  <span className="flex items-center gap-1.5 font-medium">
                    <Sparkles className="h-4 w-4" /> Enhance
                  </span>
                  <span className="block text-[12px] text-muted">Mix in songs from your library that sound like these playlists</span>
                </span>
              </label>
            )}
          </>
        )}
        <ErrorText error={save.error} />
      </form>
    </Modal>
  );
}
