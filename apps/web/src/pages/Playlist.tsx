import { useState, type ReactNode } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import clsx from 'clsx';
import { Eye, EyeOff, ExternalLink, Link2, Radio, RefreshCw, RotateCcw, Trash2, Unlink, X } from 'lucide-react';
import type { PlaylistItem, PlaylistPick } from '@/api/types';
import { useAuth } from '@/lib/auth';
import { formatDuration, plural, timeAgo } from '@/lib/format';
import { usePlaylist, usePlaylistActions, usePlaylistItemActions, usePlaylistPicks, useRemovePick, useTrackPicker } from '@/lib/queries';
import { coverUrl } from '@/player/PlayerProvider';
import { HeroBody, HeroHeader, playlistCover, useArtColor } from '@/components/media';
import { StationEditor } from '@/components/StationEditor';
import { Art, Button, Empty, ErrorText, IconButton, Input, Modal, Spinner, Tabs } from '@/components/ui';
import { matchPct } from './Playlists';

const NEW_DAYS = 7;

type ItemTab = 'matched' | 'picks' | 'missing' | 'hidden';

/** Admin buttons on a playlist track. Matches are shared by every playlist with the track; hiding is per playlist. */
function ItemActions({ item, playlistId, onPick }: { item: PlaylistItem; playlistId: string; onPick: () => void }) {
  const { setMatch, resetMatch, setHidden } = usePlaylistItemActions();
  const ids = { playlistId, spotifyTrackId: item.spotify_track_id };
  const busy = setMatch.isPending || resetMatch.isPending || setHidden.isPending;
  const error = setMatch.error ?? resetMatch.error ?? setHidden.error;
  if (item.hidden) {
    return (
      <span className="flex items-center justify-end gap-1">
        <IconButton label={`Show ${item.title} again`} size="sm" disabled={busy} onClick={() => setHidden.mutate({ ...ids, hidden: false })}>
          <Eye className="h-4 w-4" />
        </IconButton>
      </span>
    );
  }
  return (
    <span className="flex items-center justify-end gap-1" title={error instanceof Error ? error.message : undefined}>
      <IconButton label={item.track_id ? `Change the file for ${item.title}` : `Pick the file for ${item.title}`} size="sm" disabled={busy} onClick={onPick}>
        <Link2 className="h-4 w-4" />
      </IconButton>
      {item.track_id && (
        <IconButton label={`Wrong match: mark ${item.title} as not in your library`} size="sm" disabled={busy} onClick={() => setMatch.mutate({ ...ids, trackId: null })}>
          <Unlink className="h-4 w-4" />
        </IconButton>
      )}
      {item.manual && (
        <IconButton label={`Match ${item.title} automatically again`} size="sm" disabled={busy} onClick={() => resetMatch.mutate(ids)}>
          <RotateCcw className="h-4 w-4" />
        </IconButton>
      )}
      <IconButton label={`Hide ${item.title} from this playlist's stations`} size="sm" disabled={busy} onClick={() => setHidden.mutate({ ...ids, hidden: true })}>
        <EyeOff className="h-4 w-4" />
      </IconButton>
    </span>
  );
}

/** Search the library and pick the local file a playlist track plays. */
function MatchPicker({ item, playlistId, onClose }: { item: PlaylistItem | null; playlistId: string; onClose: () => void }) {
  const [q, setQ] = useState('');
  const [seeded, setSeeded] = useState<string | null>(null);
  if (item && seeded !== item.spotify_track_id) {
    setSeeded(item.spotify_track_id);
    setQ(item.title);
  }
  const results = useTrackPicker(item ? q : '');
  const { setMatch } = usePlaylistItemActions();
  if (!item) return null;
  const pick = (trackId: string | null) =>
    setMatch.mutate({ playlistId, spotifyTrackId: item.spotify_track_id, trackId }, { onSuccess: onClose });
  return (
    <Modal title="Pick the local file" open onClose={onClose} wide>
      <p className="mb-3 text-sm text-muted">
        For <span className="font-semibold text-ink">{item.title}</span> · {item.artists.join(', ')}. Every playlist with this song uses the file
        you pick, and syncs keep it.
      </p>
      <Input label="Search your library" value={q} onChange={(e) => setQ(e.target.value)} autoFocus />
      <ErrorText error={results.error ?? setMatch.error} />
      <div className="mt-3 max-h-80 overflow-y-auto">
        {results.isLoading && <Spinner />}
        {results.data?.map((t) => (
          <button
            key={t.id}
            type="button"
            disabled={setMatch.isPending}
            onClick={() => pick(t.id)}
            className={clsx(
              'flex w-full items-center gap-3 rounded-md px-2 py-2 text-left text-sm hover:bg-base/60',
              t.id === item.track_id && 'bg-accent-soft',
            )}
          >
            <Art src={coverUrl(t.id)} name={t.title} className="h-9 w-9" />
            <span className="min-w-0 flex-1">
              <span className="block truncate font-medium">{t.title}</span>
              <span className="block truncate text-[13px] text-muted">
                {t.artist ?? 'Unknown artist'}
                {t.album && <span className="text-faint"> · {t.album}</span>}
              </span>
            </span>
            <span className="tabular-nums text-muted">{t.duration_seconds ? formatDuration(t.duration_seconds) : ''}</span>
          </button>
        ))}
        {results.data?.length === 0 && <p className="py-4 text-center text-sm text-muted">No songs match “{q}”.</p>}
      </div>
      <div className="mt-4 flex justify-end">
        <Button disabled={setMatch.isPending} onClick={() => pick(null)}>
          Not in my library
        </Button>
      </div>
    </Modal>
  );
}

function ItemRow({
  item,
  index,
  lastSync,
  actions,
}: {
  item: PlaylistItem;
  index: number;
  lastSync?: string;
  actions?: ReactNode;
}) {
  const isNew = Date.now() - Date.parse(item.first_seen_at) < NEW_DAYS * 86400_000;
  // Every sync refreshes last_seen_at of the tracks Spotify still lists.
  const rotatedOut = !!lastSync && Date.parse(item.last_seen_at) < Date.parse(lastSync) - 60_000;
  return (
    <div
      className={clsx(
        'grid items-center gap-4 rounded-md px-4 py-2 text-sm hover:bg-raised/70',
        actions
          ? 'grid-cols-[2rem_minmax(0,1fr)_auto_auto] md:grid-cols-[2rem_minmax(0,3fr)_minmax(0,2fr)_6rem_4rem_auto]'
          : 'grid-cols-[2rem_minmax(0,1fr)_auto] md:grid-cols-[2rem_minmax(0,3fr)_minmax(0,2fr)_6rem_4rem]',
      )}
    >
      <span className="tabular-nums text-muted">{index}</span>
      <span className="flex min-w-0 items-center gap-3">
        {item.track_id ? (
          <Art src={coverUrl(item.track_id)} name={item.title} className="h-10 w-10" />
        ) : (
          <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-md border border-dashed border-line text-faint">?</span>
        )}
        <span className="min-w-0">
          <span className="block truncate font-medium">{item.title}</span>
          <span className="block truncate text-[13px] text-muted">{item.artists.join(', ')}</span>
        </span>
      </span>
      <span className="hidden min-w-0 truncate text-[13px] text-muted md:block">
        {item.track_id ? (
          <>
            {item.local_title} <span className="text-faint">· {item.local_artist}</span>
          </>
        ) : (
          'Not in your library'
        )}
      </span>
      <span className="hidden flex-wrap gap-1 md:flex">
        {isNew && <span className="rounded-full bg-accent-soft px-2 py-0.5 text-[11px] font-bold text-accent">New</span>}
        {rotatedOut && <span className="rounded-full bg-raised px-2 py-0.5 text-[11px] text-muted" title="Spotify no longer lists it; OwnWave keeps it">Rotated out</span>}
        {item.match_method === 'fuzzy' && <span className="rounded-full bg-raised px-2 py-0.5 text-[11px] text-muted" title="Matched on a similar title">Fuzzy</span>}
        {item.manual && (
          <span className="rounded-full bg-raised px-2 py-0.5 text-[11px] text-muted" title="An admin set this; syncs keep it">
            {item.track_id ? 'Set by hand' : 'Marked missing'}
          </span>
        )}
      </span>
      <span className="text-right tabular-nums text-muted">{item.duration_ms ? formatDuration(item.duration_ms / 1000) : ''}</span>
      {actions}
    </div>
  );
}

function PickRow({ pick, index, onRemove, removing }: { pick: PlaylistPick; index: number; onRemove?: () => void; removing: boolean }) {
  return (
    <div className="grid grid-cols-[2rem_minmax(0,1fr)_auto_auto] items-center gap-4 rounded-md px-4 py-2 text-sm hover:bg-raised/70">
      <span className="tabular-nums text-muted">{index}</span>
      <span className="flex min-w-0 items-center gap-3">
        <Art src={coverUrl(pick.track_id)} name={pick.title} className="h-10 w-10" />
        <span className="min-w-0">
          <span className="block truncate font-medium">{pick.title}</span>
          <span className="block truncate text-[13px] text-muted">{pick.artist ?? 'Unknown artist'}</span>
        </span>
      </span>
      <span className="text-right tabular-nums text-muted">{pick.duration_seconds ? formatDuration(pick.duration_seconds) : ''}</span>
      {onRemove ? (
        <IconButton label={`Remove ${pick.title} from this playlist's picks`} size="sm" disabled={removing} onClick={onRemove}>
          <X className="h-4 w-4" />
        </IconButton>
      ) : (
        <span />
      )}
    </div>
  );
}

function Picks({ playlistId, canRemove }: { playlistId: string; canRemove: boolean }) {
  const picks = usePlaylistPicks(playlistId, true);
  const remove = useRemovePick();
  if (picks.isLoading) return <Spinner />;
  return (
    <>
      <ErrorText error={picks.error ?? remove.error} />
      {picks.data?.length ? (
        <>
          <p className="mb-3 text-[13px] text-muted">Songs from your library that sound like this playlist. Its stations play them alongside the matched tracks.</p>
          {picks.data.map((pick, i) => (
            <PickRow
              key={pick.track_id}
              pick={pick}
              index={i + 1}
              removing={remove.isPending}
              onRemove={canRemove ? () => remove.mutate({ playlistId, trackId: pick.track_id }) : undefined}
            />
          ))}
        </>
      ) : (
        !picks.error && <Empty title="No picks yet">Picks follow the playlist's matched tracks once those are analysed.</Empty>
      )}
    </>
  );
}

export function PlaylistPage() {
  const { id = '' } = useParams();
  const { user } = useAuth();
  const [tab, setTab] = useState<ItemTab>('matched');
  const [picking, setPicking] = useState<PlaylistItem | null>(null);
  const all = usePlaylist(id);
  const { sync, remove } = usePlaylistActions();
  const loaded = all.data?.playlist;
  const color = useArtColor(loaded && playlistCover(loaded), loaded?.name ?? id);
  const navigate = useNavigate();
  const [creating, setCreating] = useState(false);

  if (all.isLoading) return <Spinner />;
  if (!all.data) return <Empty title="Playlist not found" />;
  const p = all.data.playlist;
  const isAdmin = !!user?.is_admin;
  const items = all.data.items.filter((i) =>
    tab === 'hidden' ? i.hidden : !i.hidden && (tab === 'matched' ? !!i.track_id : !i.track_id),
  );

  return (
    <>
      <HeroHeader
        color={color}
        art={<Art src={playlistCover(p)} name={p.name} className="aspect-square w-full" />}
        kind="Spotify playlist"
        title={p.name}
        meta={
          <>
            <span className="font-semibold">{matchPct(p)}% in your library</span>
            <span className="mx-1.5">•</span>
            {p.matched_count} of {plural(p.track_count, 'track')}
            {p.last_synced_at && (
              <>
                <span className="mx-1.5">•</span>synced {timeAgo(p.last_synced_at)}
              </>
            )}
          </>
        }
      />
      <HeroBody color={color}>
        <div className="flex flex-wrap items-center gap-3 py-6">
          <Button variant="primary" onClick={() => setCreating(true)}>
            <Radio className="h-4 w-4" /> Make a station
          </Button>
          <a
            href={`https://open.spotify.com/playlist/${p.spotify_id}`}
            target="_blank"
            rel="noreferrer noopener"
            className="inline-flex h-10 items-center gap-2 rounded-full border border-line px-4 text-sm font-semibold hover:border-ink"
          >
            <ExternalLink className="h-4 w-4" /> Open on Spotify
          </a>
          {user?.is_admin && (
            <>
              <IconButton label="Sync now" onClick={() => sync.mutate(p.id)} disabled={sync.isPending}>
                <RefreshCw className={clsx('h-5 w-5', sync.isPending && 'animate-spin')} />
              </IconButton>
              <IconButton
                label="Delete playlist"
                onClick={() => {
                  if (confirm(`Delete “${p.name}” and every track it collected?`)) remove.mutate(p.id, { onSuccess: () => navigate('/playlists') });
                }}
              >
                <Trash2 className="h-5 w-5" />
              </IconButton>
            </>
          )}
        </div>
        {p.last_sync_error && <p className="mb-4 text-sm text-danger">Last sync failed: {p.last_sync_error}</p>}
        <ErrorText error={sync.error ?? remove.error} />
        <div className="mb-4">
          <Tabs
            label="Playlist tracks"
            value={tab}
            onChange={setTab}
            tabs={[
              { id: 'matched', label: 'In your library', count: p.matched_count },
              { id: 'picks', label: 'From your library', count: p.pick_count },
              { id: 'missing', label: 'Missing', count: p.track_count - p.matched_count },
              ...(p.hidden_count > 0 || isAdmin ? [{ id: 'hidden' as const, label: 'Hidden', count: p.hidden_count }] : []),
            ]}
          />
        </div>
        {tab === 'missing' && items.length > 0 && (
          <p className="mb-3 text-[13px] text-muted">Songs this playlist listed that are not in your library. Add the files and the next scan picks them up.</p>
        )}
        {tab === 'hidden' && items.length > 0 && (
          <p className="mb-3 text-[13px] text-muted">Songs left out of this playlist's stations. Syncs keep them hidden.</p>
        )}
        {tab === 'picks' ? (
          <Picks playlistId={p.id} canRemove={!!user?.is_admin} />
        ) : items.length ? (
          items.map((it, i) => (
            <ItemRow
              key={it.spotify_track_id}
              item={it}
              index={i + 1}
              lastSync={p.last_synced_at}
              actions={isAdmin ? <ItemActions item={it} playlistId={p.id} onPick={() => setPicking(it)} /> : undefined}
            />
          ))
        ) : (
          <Empty title={tab === 'matched' ? 'No matches yet' : tab === 'hidden' ? 'Nothing hidden' : 'Nothing missing'}>
            {tab === 'matched'
              ? 'None of these songs were found in your library (yet).'
              : tab === 'hidden'
                ? 'Hide a song to keep it out of this playlist\'s stations.'
                : 'Every song of this playlist is in your library.'}
          </Empty>
        )}
      </HeroBody>
      {isAdmin && <MatchPicker item={picking} playlistId={p.id} onClose={() => setPicking(null)} />}
      <StationEditor
        open={creating}
        onClose={() => setCreating(false)}
        initial={{ name: p.name, playlist_ids: [p.id] }}
        onSaved={(s) => navigate(`/station/${s.id}`)}
      />
    </>
  );
}
