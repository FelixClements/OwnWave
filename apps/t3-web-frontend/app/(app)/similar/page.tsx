'use client';

import { useState } from 'react';
import { useSearchParams } from 'next/navigation';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '@/lib/api';
import { keys } from '@/lib/queries';
import { useStation } from '@/lib/station';
import { TrackTable } from '@/components/tracks/TrackTable';
import { Button } from '@/components/ui/Button';
import { Field, Input, Select } from '@/components/ui/Field';
import { PageHeader, Panel } from '@/components/ui/Layout';

export default function SimilarPage() {
  const search = useSearchParams();
  const trackId = search?.get('track');
  const qc = useQueryClient();
  const { setSelectedStation } = useStation();
  const [selected, setSelected] = useState(trackId ?? '');
  const [stationName, setStationName] = useState('');

  const { data: tracks } = useQuery(keys.tracks(''), () => api.listTracks({ limit: 200 }));
  const { data: selectedTrack } = useQuery(['track', selected], () => api.getTrack(selected), {
    enabled: !!selected && !tracks?.some((t) => t.id === selected),
  });
  const { data: similar, isLoading } = useQuery(keys.similar(selected), () => api.getSimilarTracks(selected), {
    enabled: !!selected,
  });
  const create = useMutation(
    (vars: { name: string; trackId: string }) =>
      api.createStation({ name: vars.name, seed_type: 'track', track_id: vars.trackId }),
    {
      onSuccess: (data) => {
        setStationName('');
        void qc.invalidateQueries(keys.stations);
        setSelectedStation(data.station_id);
      },
    },
  );
  const options =
    selectedTrack && !tracks?.some((t) => t.id === selectedTrack.id) ? [selectedTrack, ...(tracks ?? [])] : tracks;

  const handleSelect = (id: string) => {
    setSelected(id);
    const url = new URL(window.location.href);
    url.searchParams.set('track', id);
    window.history.replaceState({}, '', url.toString());
  };

  const handleCreateStation = (e: React.FormEvent) => {
    e.preventDefault();
    if (!selected || !stationName.trim()) return;
    create.mutate({ name: stationName.trim(), trackId: selected });
  };

  const rows = (similar ?? []).map((t) => ({
    id: t.track_id,
    title: t.title,
    artist: t.artist,
    album: t.album,
    distance: t.distance,
  }));

  return (
    <>
      <PageHeader title="Similar tracks" subtitle="Nearest neighbours by audio features." />

      <div className="grid gap-3 lg:grid-cols-[1fr_340px] mb-4 items-start">
        <Panel>
          <Field label="Seed track" hint="Shows the first 200 library tracks; use “Similar” in the library for any other.">
            {(p) => (
              <Select {...p} value={selected} onChange={(e) => handleSelect(e.target.value)}>
                <option value="">Choose a track</option>
                {options?.map((track) => (
                  <option key={track.id} value={track.id}>
                    {track.title}
                    {track.artist ? ` — ${track.artist}` : ''}
                  </option>
                ))}
              </Select>
            )}
          </Field>
        </Panel>

        {selected && (
          <Panel title="Create station from this track">
            <form onSubmit={handleCreateStation} className="flex flex-col gap-2.5">
              <Field label="Station name" error={create.error instanceof Error ? create.error.message : null}>
                {(p) => (
                  <Input
                    {...p}
                    value={stationName}
                    onChange={(e) => setStationName(e.target.value)}
                    placeholder="e.g. Like this track"
                    required
                  />
                )}
              </Field>
              <Button type="submit" variant="primary" loading={create.isLoading} disabled={!stationName.trim()}>
                {create.isLoading ? 'Creating…' : 'Create station'}
              </Button>
            </form>
          </Panel>
        )}
      </div>

      {selected && (
        <TrackTable
          label="Similar tracks"
          rows={rows}
          columns={['title', 'album', 'distance']}
          sortable
          empty={isLoading ? 'Loading…' : 'No similar tracks found.'}
        />
      )}
    </>
  );
}
