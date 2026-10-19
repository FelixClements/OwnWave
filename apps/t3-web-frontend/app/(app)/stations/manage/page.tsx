'use client';

import { useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { api, type StationRequest } from '@/lib/api';
import {
  describeStationSeed,
  emptyStationFilterForm,
  filterFormToRequest,
  parseStationSeed,
  SEED_TYPE_LABELS,
  seedToFilterForm,
} from '@/lib/station-seed';
import { keys, useGenres, useQueue, useStationQuery, useStations } from '@/lib/queries';
import { useStation } from '@/lib/station';
import { StationArt } from '@/components/stations/StationArt';
import { StationForm } from '@/components/stations/StationForm';
import { TrackTable } from '@/components/tracks/TrackTable';
import { Button } from '@/components/ui/Button';
import { Badge, Notice, PageHeader, Panel, type NoticeValue } from '@/components/ui/Layout';
import { EmptyRow, Table, Td, Th } from '@/components/ui/Table';

type Editor = { mode: 'create' } | { mode: 'edit'; id: string } | null;

export default function ManageStationsPage() {
  const qc = useQueryClient();
  const { selectedStation, setSelectedStation, playingStation, setIsPlaying } = useStation();
  const { data: stations } = useStations();
  const { data: genres = [] } = useGenres();
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
    onSuccess: (_data, body) => {
      closeEditor();
      setNotice({ type: 'success', text: `Created “${body.name}”.` });
      void qc.invalidateQueries(keys.stations);
    },
    onError: (err: Error) => setFormError(err.message),
  });

  const update = useMutation(({ id, body }: { id: string; body: StationRequest }) => api.updateStation(id, body), {
    onSuccess: (data, { id }) => {
      closeEditor();
      setNotice({ type: 'success', text: `Saved “${data.name}” (${data.track_count} tracks).` });
      void qc.invalidateQueries(keys.stations);
      void qc.invalidateQueries(keys.station(id));
      void qc.invalidateQueries(keys.queue(id));
    },
    onError: (err: Error) => setFormError(err.message),
  });

  const remove = useMutation((id: string) => api.deleteStation(id), {
    onSuccess: (_, id) => {
      void qc.invalidateQueries(keys.stations);
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
        subtitle="A station holds every library track matching its seed and filters."
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
                <Th style={{ width: '96px' }} className="hidden sm:table-cell">
                  Seed
                </Th>
                <Th className="hidden lg:table-cell w-[40%]">Filters</Th>
                <Th align="right" style={{ width: '196px' }}>
                  <span className="sr-only">Actions</span>
                </Th>
              </tr>
            </thead>
            <tbody>
              {stations?.map((s) => {
                const seed = parseStationSeed(s.seed_features as Parameters<typeof parseStationSeed>[0]);
                const seedType = seed?.type ?? seed?.seed_type;
                const editing = editingId === s.id;
                return (
                  <tr key={s.id} className={editing || previewId === s.id ? 'bg-accent/[0.08]' : 'hover:bg-hover/50'}>
                    <Td>
                      <StationArt name={s.name} />
                    </Td>
                    <Td className="font-medium">{s.name}</Td>
                    <Td className="hidden sm:table-cell">
                      {seedType ? <Badge>{SEED_TYPE_LABELS[seedType]}</Badge> : <span className="text-muted">—</span>}
                    </Td>
                    <Td className="hidden lg:table-cell text-muted" title={describeStationSeed(seed)}>
                      {describeStationSeed(seed) || '—'}
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
                            if (confirm(`Delete station “${s.name}”?`)) remove.mutate(s.id);
                          }}
                        >
                          Delete
                        </Button>
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
                columns={['index', 'title', 'bpm', 'key', 'energy', 'valence', 'duration']}
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
                initialForm={emptyStationFilterForm()}
                genres={genres}
                submitLabel="Create"
                submitting={create.isLoading}
                error={formError}
                onSubmit={(name, form) => create.mutate(filterFormToRequest(name, form))}
                onCancel={closeEditor}
              />
            ) : editingReady ? (
              <StationForm
                key={editingStation.id}
                title={`Edit ${editingStation.name}`}
                initialName={editingStation.name}
                initialForm={seedToFilterForm(
                  parseStationSeed(editingStation.seed_features as Parameters<typeof parseStationSeed>[0]),
                )}
                genres={genres}
                submitLabel="Save"
                submitting={update.isLoading}
                error={formError}
                onSubmit={(name, form) => update.mutate({ id: editingStation.id, body: filterFormToRequest(name, form) })}
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
