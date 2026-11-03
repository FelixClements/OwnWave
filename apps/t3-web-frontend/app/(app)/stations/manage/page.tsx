'use client';

import { useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { api, type StationRequest, type StationSuggestion } from '@/lib/api';
import { useAuth } from '@/lib/auth';
import { describeStation } from '@/lib/station-describe';
import { keys, useGenreNames, usePlaylists, useQueue, useStationQuery, useStations, useStationSuggestions } from '@/lib/queries';
import { useStation } from '@/lib/station';
import { StationArt } from '@/components/stations/StationArt';
import { StationForm, type StationFormValue } from '@/components/stations/StationForm';
import { TrackTable } from '@/components/tracks/TrackTable';
import { Button } from '@/components/ui/Button';
import { Badge, Notice, PageHeader, Panel, type NoticeValue } from '@/components/ui/Layout';
import { EmptyRow, Table, Td, Th } from '@/components/ui/Table';

type Editor = { mode: 'create' } | { mode: 'edit'; id: string } | null;

export default function ManageStationsPage() {
  const qc = useQueryClient();
  const { selectedStation, setSelectedStation, playingStation, setIsPlaying } = useStation();
  const { user } = useAuth();
  const isAdmin = !!user?.is_admin;
  const { data: stations } = useStations();
  const { data: playlists = [] } = usePlaylists();
  const playlistNames = Object.fromEntries(playlists.map((p) => [p.id, p.name]));
  const genreNames = useGenreNames();
  const [editor, setEditor] = useState<Editor>(null);
  const [previewId, setPreviewId] = useState<string | null>(null);
  const [notice, setNotice] = useState<NoticeValue>(null);
  const [formError, setFormError] = useState<string | null>(null);

  const editingId = editor?.mode === 'edit' ? editor.id : null;
  const { data: editingStation } = useStationQuery(editingId);
  const { data: previewQueue } = useQueue(previewId);

  const closeEditor = () => {
    setEditor(null);
    setFormError(null);
  };

  const create = useMutation((body: StationRequest) => api.createStation(body), {
    onSuccess: (station) => {
      closeEditor();
      setNotice({ type: 'success', text: `Created “${station.name}” (${station.track_count} tracks).` });
      void qc.invalidateQueries(keys.stations);
      void qc.invalidateQueries(keys.stationSuggestions);
    },
    onError: (err: Error) => setFormError(err.message),
  });

  const update = useMutation(({ id, body }: { id: string; body: StationRequest }) => api.updateStation(id, body), {
    onSuccess: (data, { id }) => {
      closeEditor();
      setNotice({ type: 'success', text: `Saved “${data.name}” (${data.track_count} tracks).` });
      void qc.invalidateQueries(keys.stations);
      void qc.invalidateQueries(keys.stationSuggestions);
      void qc.invalidateQueries(keys.station(id));
      void qc.invalidateQueries(keys.queue(id));
    },
    onError: (err: Error) => setFormError(err.message),
  });

  const remove = useMutation((id: string) => api.deleteStation(id), {
    onSuccess: (_, id) => {
      void qc.invalidateQueries(keys.stations);
      void qc.invalidateQueries(keys.stationSuggestions);
      if (previewId === id) setPreviewId(null);
      if (editingId === id) closeEditor();
      if (id === selectedStation) setSelectedStation(null);
      if (id === playingStation) setIsPlaying(false);
    },
    onError: (err: Error) => setNotice({ type: 'error', text: `Failed to delete station: ${err.message}` }),
  });

  const editingReady = editingId && editingStation?.id === editingId;
  const previewName = stations?.find((s) => s.id === previewId)?.name;

  return (
    <>
      <PageHeader
        title="Stations"
        subtitle={
          isAdmin
            ? 'A station plays the local matches of its playlists, optionally only some genres. Your stations are shared with everyone.'
            : 'A station plays the local matches of its playlists, optionally only some genres.'
        }
        actions={
          <Button
            variant="primary"
            onClick={() => {
              setFormError(null);
              setEditor({ mode: 'create' });
            }}
          >
            + New station
          </Button>
        }
      />
      <div className="mb-3">
        <Notice notice={notice} />
      </div>

      {isAdmin && <Suggestions onAccepted={(name) => setNotice({ type: 'success', text: `Created “${name}”.` })} />}

      {/* Side-by-side only when the table keeps room for its filter and action columns. */}
      <div className={`grid gap-4 items-start ${editor ? '2xl:grid-cols-[1fr_380px]' : ''}`}>
        <div className="min-w-0 space-y-4">
          <Table label="Stations">
            <thead>
              <tr>
                <Th style={{ width: '34px' }}>
                  <span className="sr-only">Art</span>
                </Th>
                <Th>Name</Th>
                <Th style={{ width: '80px' }} align="right" className="hidden sm:table-cell">
                  Tracks
                </Th>
                <Th className="hidden lg:table-cell w-[40%]">Plays</Th>
                <Th align="right" style={{ width: '196px' }}>
                  <span className="sr-only">Actions</span>
                </Th>
              </tr>
            </thead>
            <tbody>
              {stations?.map((s) => {
                const description = describeStation(s, playlistNames, genreNames);
                const editing = editingId === s.id;
                const canChange = !s.shared || isAdmin;
                return (
                  <tr key={s.id} className={editing || previewId === s.id ? 'bg-accent/[0.08]' : 'hover:bg-hover/50'}>
                    <Td>
                      <StationArt name={s.name} />
                    </Td>
                    <Td className="font-medium">
                      {s.name}
                      {s.shared && (
                        <Badge tone="accent" className="ml-2">
                          Shared
                        </Badge>
                      )}
                    </Td>
                    <Td align="right" className="hidden sm:table-cell tabular-nums">
                      {s.track_count}
                    </Td>
                    <Td className="hidden lg:table-cell text-muted" title={description}>
                      {description || '—'}
                    </Td>
                    <Td align="right">
                      <div className="inline-flex gap-1">
                        <Button
                          size="sm"
                          aria-pressed={previewId === s.id}
                          onClick={() => setPreviewId(previewId === s.id ? null : s.id)}
                        >
                          Preview
                        </Button>
                        {canChange && (
                          <>
                            <Button
                              size="sm"
                              onClick={() => {
                                setFormError(null);
                                setEditor({ mode: 'edit', id: s.id });
                              }}
                              aria-label={`Edit ${s.name}`}
                            >
                              Edit
                            </Button>
                            <Button
                              size="sm"
                              variant="danger"
                              aria-label={`Delete ${s.name}`}
                              onClick={() => {
                                if (confirm(`Delete station “${s.name}”${s.shared ? ' for everyone' : ''}?`)) remove.mutate(s.id);
                              }}
                            >
                              Delete
                            </Button>
                          </>
                        )}
                      </div>
                    </Td>
                  </tr>
                );
              })}
              {stations?.length === 0 && <EmptyRow colSpan={5}>No stations yet.</EmptyRow>}
            </tbody>
          </Table>

          {previewId && (
            <Panel
              title={`Preview: ${previewName ?? 'station'}`}
              actions={
                <Button size="sm" variant="ghost" onClick={() => setPreviewId(null)}>
                  Close
                </Button>
              }
            >
              <TrackTable
                label="Preview queue"
                rows={previewQueue ?? []}
                columns={['index', 'title', 'album', 'duration']}
                empty="No tracks in queue."
              />
            </Panel>
          )}
        </div>

        {editor && (
          <Panel className="order-first 2xl:order-none 2xl:sticky 2xl:top-0 max-w-2xl 2xl:max-w-none">
            {editor.mode === 'create' ? (
              <StationForm
                key="create"
                title="New station"
                playlists={playlists}
                submitLabel="Create"
                submitting={create.isLoading}
                error={formError}
                onSubmit={(v: StationFormValue) => create.mutate(v)}
                onCancel={closeEditor}
              />
            ) : editingReady ? (
              <StationForm
                key={editingStation.id}
                title={`Edit ${editingStation.name}`}
                initial={{ name: editingStation.name, playlist_ids: editingStation.playlist_ids, genres: editingStation.genres }}
                playlists={playlists}
                submitLabel="Save"
                submitting={update.isLoading}
                error={formError}
                onSubmit={(v: StationFormValue) => update.mutate({ id: editingStation.id, body: v })}
                onCancel={closeEditor}
              />
            ) : (
              <p className="text-muted">Loading…</p>
            )}
          </Panel>
        )}
      </div>
    </>
  );
}

/** Stations OwnWave proposes from the synced playlists (admins only). */
function Suggestions({ onAccepted }: { onAccepted: (name: string) => void }) {
  const qc = useQueryClient();
  const { data } = useStationSuggestions(true);
  const [error, setError] = useState<string | null>(null);
  const refresh = () => {
    void qc.invalidateQueries(keys.stationSuggestions);
    void qc.invalidateQueries(keys.stations);
  };
  const accept = useMutation(
    (s: StationSuggestion) =>
      api.createStation({ name: s.name, playlist_ids: s.playlist_ids, genres: s.genre ? [s.genre] : [] }),
    {
      onMutate: () => setError(null),
      onSuccess: (station) => {
        onAccepted(station.name);
        refresh();
      },
      onError: (err: Error) => setError(err.message),
    },
  );
  const dismiss = useMutation((s: StationSuggestion) => api.dismissStationSuggestion(s), {
    onMutate: () => setError(null),
    onSuccess: refresh,
    onError: (err: Error) => setError(err.message),
  });

  const list = data?.suggestions ?? [];
  if (list.length === 0) return null;
  return (
    <Panel title="Suggested stations" className="mb-4">
      <p className="text-muted mb-2 text-[12.5px]">
        Built from your playlists: each one alone and all together, whole or per genre with at least {data?.min_tracks}{' '}
        tracks. Accept creates the station (rename it with Edit); Dismiss hides the suggestion for good.
      </p>
      {error && (
        <p role="alert" className="text-danger mb-2">
          {error}
        </p>
      )}
      <ul className="divide-y divide-line">
        {list.map((s) => (
          <li key={`${s.playlist_ids.join(',')}|${s.genre}`} className="flex items-center gap-3 py-2">
            <StationArt name={s.name} />
            <div className="flex-1 min-w-0">
              <div className="font-medium truncate">{s.name}</div>
              <div className="text-muted text-[12px] truncate">
                {s.playlist_names.join(', ')} · {s.genre_name ?? 'all genres'} · {s.track_count} tracks
              </div>
            </div>
            <Button size="sm" variant="primary" onClick={() => accept.mutate(s)} loading={accept.isLoading && accept.variables === s}>
              Accept
            </Button>
            <Button size="sm" variant="ghost" onClick={() => dismiss.mutate(s)} aria-label={`Dismiss ${s.name}`}>
              Dismiss
            </Button>
          </li>
        ))}
      </ul>
    </Panel>
  );
}
