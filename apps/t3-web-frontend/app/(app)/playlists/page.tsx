'use client';

import { useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { api, type Playlist, type PlaylistItem } from '@/lib/api';
import { useAuth } from '@/lib/auth';
import { formatDateTime, formatDuration } from '@/lib/format';
import { keys, usePlaylist, usePlaylists } from '@/lib/queries';
import { StationArt } from '@/components/stations/StationArt';
import { Button } from '@/components/ui/Button';
import { Input } from '@/components/ui/Field';
import { Badge, Notice, PageHeader, Panel, type NoticeValue } from '@/components/ui/Layout';
import { EmptyRow, Table, Td, Th, TitleCell } from '@/components/ui/Table';
import { TabPanel, Tabs } from '@/components/ui/Tabs';

const NEW_DAYS = 7;

export default function PlaylistsPage() {
  const qc = useQueryClient();
  const { user } = useAuth();
  const isAdmin = !!user?.is_admin;
  const { data: playlists } = usePlaylists();
  const [url, setUrl] = useState('');
  const [openId, setOpenId] = useState<string | null>(null);
  const [notice, setNotice] = useState<NoticeValue>(null);

  const refresh = (id?: string) => {
    void qc.invalidateQueries(keys.playlists);
    if (id) void qc.invalidateQueries(keys.playlist(id));
  };
  const summary = (p: Playlist) => `“${p.name}”: ${p.matched_count} of ${p.track_count} tracks found locally.`;

  const add = useMutation((u: string) => api.addPlaylist(u), {
    onMutate: () => setNotice(null),
    onSuccess: (p) => {
      setUrl('');
      setOpenId(p.id);
      setNotice({ type: 'success', text: `Added ${summary(p)}` });
      refresh(p.id);
    },
    onError: (err: Error) => setNotice({ type: 'error', text: `Could not add playlist: ${err.message}` }),
  });

  const sync = useMutation((id: string) => api.syncPlaylist(id), {
    onMutate: () => setNotice(null),
    onSuccess: (p) => {
      setNotice({ type: 'success', text: `Synced ${summary(p)}` });
      refresh(p.id);
    },
    onError: (err: Error, id) => {
      setNotice({ type: 'error', text: `Sync failed: ${err.message}` });
      refresh(id);
    },
  });

  const remove = useMutation((id: string) => api.deletePlaylist(id), {
    onMutate: () => setNotice(null),
    onSuccess: (_, id) => {
      if (openId === id) setOpenId(null);
      refresh();
    },
    onError: (err: Error) => setNotice({ type: 'error', text: `Could not delete playlist: ${err.message}` }),
  });

  const open = playlists?.find((p) => p.id === openId);

  return (
    <>
      <PageHeader
        title="Playlists"
        subtitle="Spotify playlists, synced daily. Playlists only grow: tracks Spotify rotates out are kept."
      />
      {isAdmin && (
        <form
          className="flex flex-wrap gap-2 mb-3"
          onSubmit={(e) => {
            e.preventDefault();
            if (url.trim()) add.mutate(url.trim());
          }}
        >
          <Input
            value={url}
            onChange={(e) => setUrl(e.target.value)}
            placeholder="https://open.spotify.com/playlist/…"
            aria-label="Spotify playlist URL"
            className="flex-1 min-w-[260px]"
          />
          <Button type="submit" variant="primary" loading={add.isLoading} disabled={!url.trim()}>
            Add playlist
          </Button>
        </form>
      )}
      <div className="mb-3">
        <Notice notice={notice} />
      </div>

      <div className="space-y-4">
        <Table label="Playlists">
          <thead>
            <tr>
              <Th style={{ width: '34px' }}>
                <span className="sr-only">Art</span>
              </Th>
              <Th>Name</Th>
              <Th align="right" style={{ width: '110px' }}>
                Found locally
              </Th>
              <Th className="hidden md:table-cell" style={{ width: '190px' }}>
                Last synced
              </Th>
              <Th align="right" style={{ width: isAdmin ? '196px' : '80px' }}>
                <span className="sr-only">Actions</span>
              </Th>
            </tr>
          </thead>
          <tbody>
            {playlists?.map((p) => (
              <tr key={p.id} className={openId === p.id ? 'bg-accent/[0.08]' : 'hover:bg-hover/50'}>
                <Td>
                  <StationArt name={p.name} />
                </Td>
                <Td className="font-medium">
                  {p.name}
                  {p.last_sync_error && (
                    <Badge tone="danger" className="ml-2">
                      <span title={p.last_sync_error}>Sync failed</span>
                    </Badge>
                  )}
                </Td>
                <Td align="right" className="tabular-nums">
                  {p.matched_count} / {p.track_count}
                </Td>
                <Td className="hidden md:table-cell text-muted">{formatDateTime(p.last_synced_at)}</Td>
                <Td align="right">
                  <div className="inline-flex gap-1">
                    <Button size="sm" aria-pressed={openId === p.id} onClick={() => setOpenId(openId === p.id ? null : p.id)}>
                      Tracks
                    </Button>
                    {isAdmin && (
                      <>
                        <Button
                          size="sm"
                          loading={sync.isLoading && sync.variables === p.id}
                          onClick={() => sync.mutate(p.id)}
                          aria-label={`Sync ${p.name}`}
                        >
                          Sync
                        </Button>
                        <Button
                          size="sm"
                          variant="danger"
                          aria-label={`Delete ${p.name}`}
                          onClick={() => {
                            if (confirm(`Delete playlist “${p.name}” and every track it collected?`)) remove.mutate(p.id);
                          }}
                        >
                          Delete
                        </Button>
                      </>
                    )}
                  </div>
                </Td>
              </tr>
            ))}
            {playlists?.length === 0 && (
              <EmptyRow colSpan={5}>
                No playlists yet.{isAdmin ? ' Paste a Spotify playlist URL above.' : ' An admin can add them.'}
              </EmptyRow>
            )}
          </tbody>
        </Table>

        {open && <PlaylistTracks playlist={open} onClose={() => setOpenId(null)} />}
      </div>
    </>
  );
}

type TabId = 'found' | 'missing';

function PlaylistTracks({ playlist, onClose }: { playlist: Playlist; onClose: () => void }) {
  const { data } = usePlaylist(playlist.id);
  const [tab, setTab] = useState<TabId>('found');
  const items = data?.items ?? [];
  const found = items.filter((i) => i.track_id);
  const missing = items.filter((i) => !i.track_id);
  const rows = tab === 'found' ? found : missing;

  return (
    <Panel
      title={playlist.name}
      actions={
        <Button size="sm" variant="ghost" onClick={onClose}>
          Close
        </Button>
      }
    >
      {playlist.last_sync_error && (
        <p role="alert" className="text-danger mb-3">
          Last sync failed: {playlist.last_sync_error}
        </p>
      )}
      <Tabs<TabId>
        idPrefix="playlist-tracks"
        label="Playlist tracks"
        value={tab}
        onChange={setTab}
        tabs={[
          { id: 'found', label: 'Found locally', count: found.length },
          { id: 'missing', label: 'Missing', count: missing.length },
        ]}
      />
      <TabPanel idPrefix="playlist-tracks" id={tab}>
        <Table label={tab === 'found' ? 'Tracks found locally' : 'Tracks missing locally'}>
          <thead>
            <tr>
              <Th>Spotify track</Th>
              {tab === 'found' && <Th className="hidden md:table-cell">Local file</Th>}
              <Th align="right" style={{ width: '64px' }}>
                Length
              </Th>
              <Th className="hidden sm:table-cell" style={{ width: '150px' }}>
                Status
              </Th>
            </tr>
          </thead>
          <tbody>
            {rows.map((item) => (
              <ItemRow key={item.spotify_track_id} item={item} playlist={playlist} showLocal={tab === 'found'} />
            ))}
            {data && rows.length === 0 && (
              <EmptyRow colSpan={tab === 'found' ? 4 : 3}>
                {tab === 'found' ? 'No tracks found in the library yet.' : 'Every track is in the library.'}
              </EmptyRow>
            )}
          </tbody>
        </Table>
      </TabPanel>
    </Panel>
  );
}

function ItemRow({ item, playlist, showLocal }: { item: PlaylistItem; playlist: Playlist; showLocal: boolean }) {
  const isNew = Date.now() - new Date(item.first_seen_at).getTime() < NEW_DAYS * 24 * 3600 * 1000;
  // Every sync refreshes last_seen_at of the tracks Spotify still lists.
  const rotatedOut =
    !!playlist.last_synced_at && new Date(item.last_seen_at).getTime() < new Date(playlist.last_synced_at).getTime() - 60_000;
  return (
    <tr className="hover:bg-hover/50">
      <Td>
        <TitleCell title={item.title} subtitle={item.artists.join(', ')} />
      </Td>
      {showLocal && (
        <Td className="hidden md:table-cell">
          <TitleCell title={item.local_title ?? ''} subtitle={item.local_artist} />
        </Td>
      )}
      <Td align="right" className="tabular-nums text-muted">
        {item.duration_ms ? formatDuration(item.duration_ms / 1000) : '—'}
      </Td>
      <Td className="hidden sm:table-cell">
        <div className="flex flex-wrap gap-1">
          {isNew && <Badge tone="accent">New</Badge>}
          {rotatedOut && <Badge>Off Spotify list</Badge>}
          {item.match_method === 'fuzzy' && (
            <Badge tone="ok">
              <span title="Matched on a similar title">Fuzzy match</span>
            </Badge>
          )}
        </div>
      </Td>
    </tr>
  );
}
