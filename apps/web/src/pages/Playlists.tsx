import { useState } from 'react';
import { Link } from 'react-router-dom';
import { ListMusic, Plus } from 'lucide-react';
import type { Playlist } from '@/api/types';
import { useAuth } from '@/lib/auth';
import { timeAgo } from '@/lib/format';
import { usePlaylistActions, usePlaylists } from '@/lib/queries';
import { CardGrid, playlistCover } from '@/components/media';
import { Art, Button, Empty, ErrorText, Input, Spinner } from '@/components/ui';

export const matchPct = (p: Playlist) => (p.track_count ? Math.round((p.matched_count / p.track_count) * 100) : 0);

function PlaylistCard({ p }: { p: Playlist }) {
  const pct = matchPct(p);
  return (
    <Link to={`/playlist/${p.id}`} className="group flex flex-col gap-3 rounded-lg p-3 transition hover:bg-card">
      <div className="relative">
        <Art src={playlistCover(p)} name={p.name} className="aspect-square w-full shadow-lift" />
        <span className="absolute bottom-2 left-2 rounded-full bg-black/70 px-2 py-0.5 text-[11px] font-bold tabular-nums">{pct}% in library</span>
      </div>
      <div className="min-w-0">
        <div className="truncate font-semibold">{p.name}</div>
        <div className="text-[13px] text-muted">
          {p.matched_count} of {p.track_count} tracks
          {p.last_sync_error ? <span className="text-danger"> · sync failed</span> : p.last_synced_at && ` · synced ${timeAgo(p.last_synced_at)}`}
        </div>
      </div>
    </Link>
  );
}

export function PlaylistsPage() {
  const { user } = useAuth();
  const { data, isLoading } = usePlaylists();
  const { add } = usePlaylistActions();
  const [url, setUrl] = useState('');
  const [adding, setAdding] = useState(false);
  if (isLoading) return <Spinner />;
  return (
    <div className="pt-2">
      <div className="mb-6 flex flex-wrap items-center justify-between gap-4">
        <div>
          <h1 className="text-3xl font-black tracking-tight">Playlists</h1>
          <p className="mt-1 text-sm text-muted">Public Spotify playlists, synced daily. They only grow: songs Spotify drops stay.</p>
        </div>
        {user?.is_admin && !adding && (
          <Button variant="primary" onClick={() => setAdding(true)}>
            <Plus className="h-4 w-4" /> Add playlist
          </Button>
        )}
      </div>
      {adding && (
        <form
          className="mb-8 flex max-w-2xl flex-col gap-3 rounded-lg bg-card p-4 sm:flex-row sm:items-end"
          onSubmit={(e) => {
            e.preventDefault();
            if (url.trim()) add.mutate(url.trim(), { onSuccess: () => (setUrl(''), setAdding(false)) });
          }}
        >
          <div className="flex-1">
            <Input label="Spotify playlist link" placeholder="https://open.spotify.com/playlist/37i9dQZF1DX1spT6G94GFC" value={url} onChange={(e) => setUrl(e.target.value)} autoFocus />
          </div>
          <Button type="submit" variant="primary" loading={add.isPending} disabled={!url.trim()}>
            Add and sync
          </Button>
          <Button variant="ghost" onClick={() => setAdding(false)}>
            Cancel
          </Button>
        </form>
      )}
      <ErrorText error={add.error} />
      {data?.length ? (
        <CardGrid>
          {data.map((p) => (
            <PlaylistCard key={p.id} p={p} />
          ))}
        </CardGrid>
      ) : (
        <Empty icon={<ListMusic className="h-12 w-12" />} title="No playlists yet">
          {user?.is_admin ? 'Paste a public Spotify playlist link to start.' : 'An admin adds Spotify playlists here.'}
        </Empty>
      )}
    </div>
  );
}
