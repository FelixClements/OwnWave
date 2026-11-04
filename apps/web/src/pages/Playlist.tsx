import { useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import clsx from 'clsx';
import { ExternalLink, Radio, RefreshCw, Trash2 } from 'lucide-react';
import type { PlaylistItem } from '@/api/types';
import { useAuth } from '@/lib/auth';
import { formatDuration, plural, timeAgo } from '@/lib/format';
import { usePlaylist, usePlaylistActions } from '@/lib/queries';
import { coverUrl } from '@/player/PlayerProvider';
import { HeroBody, HeroHeader, playlistCover, useArtColor } from '@/components/media';
import { StationEditor } from '@/components/StationEditor';
import { Art, Button, Empty, ErrorText, IconButton, Spinner, Tabs } from '@/components/ui';
import { matchPct } from './Playlists';

const NEW_DAYS = 7;

function ItemRow({ item, index, lastSync }: { item: PlaylistItem; index: number; lastSync?: string }) {
  const isNew = Date.now() - Date.parse(item.first_seen_at) < NEW_DAYS * 86400_000;
  // Every sync refreshes last_seen_at of the tracks Spotify still lists.
  const rotatedOut = !!lastSync && Date.parse(item.last_seen_at) < Date.parse(lastSync) - 60_000;
  return (
    <div className="grid grid-cols-[2rem_minmax(0,1fr)_auto] items-center gap-4 rounded-md px-4 py-2 text-sm hover:bg-raised/70 md:grid-cols-[2rem_minmax(0,3fr)_minmax(0,2fr)_6rem_4rem]">
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
      </span>
      <span className="text-right tabular-nums text-muted">{item.duration_ms ? formatDuration(item.duration_ms / 1000) : ''}</span>
    </div>
  );
}

export function PlaylistPage() {
  const { id = '' } = useParams();
  const { user } = useAuth();
  const [tab, setTab] = useState<'matched' | 'missing'>('matched');
  const all = usePlaylist(id);
  const { sync, remove } = usePlaylistActions();
  const loaded = all.data?.playlist;
  const color = useArtColor(loaded && playlistCover(loaded), loaded?.name ?? id);
  const navigate = useNavigate();
  const [creating, setCreating] = useState(false);

  if (all.isLoading) return <Spinner />;
  if (!all.data) return <Empty title="Playlist not found" />;
  const p = all.data.playlist;
  const items = all.data.items.filter((i) => (tab === 'matched' ? !!i.track_id : !i.track_id));

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
              { id: 'missing', label: 'Missing', count: p.track_count - p.matched_count },
            ]}
          />
        </div>
        {tab === 'missing' && items.length > 0 && (
          <p className="mb-3 text-[13px] text-muted">Songs this playlist listed that are not in your library. Add the files and the next scan picks them up.</p>
        )}
        {items.length ? (
          items.map((it, i) => <ItemRow key={it.spotify_track_id} item={it} index={i + 1} lastSync={p.last_synced_at} />)
        ) : (
          <Empty title={tab === 'matched' ? 'No matches yet' : 'Nothing missing'}>
            {tab === 'matched' ? 'None of these songs were found in your library (yet).' : 'Every song of this playlist is in your library.'}
          </Empty>
        )}
      </HeroBody>
      <StationEditor
        open={creating}
        onClose={() => setCreating(false)}
        initial={{ name: p.name, playlist_ids: [p.id] }}
        onSaved={(s) => navigate(`/station/${s.id}`)}
      />
    </>
  );
}
