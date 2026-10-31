'use client';

import { useEffect, useMemo, useState } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import type { Station } from '@/lib/api';
import { useStation } from '@/lib/station';
import { useFeedbackMutation, usePlaylists, useSetupStatus } from '@/lib/queries';
import { describeStationSeed, parseStationSeed, SEED_TYPE_LABELS } from '@/lib/station-seed';
import { StationArt } from '@/components/stations/StationArt';
import { TrackTable } from '@/components/tracks/TrackTable';
import { Button, buttonClass } from '@/components/ui/Button';
import { Input } from '@/components/ui/Field';
import { PageHeader } from '@/components/ui/Layout';
import { PauseIcon, PlayIcon } from '@/components/ui/Icons';

export default function ListenPage() {
  const router = useRouter();
  const { selectedStation, stations } = useStation();

  const setupStatus = useSetupStatus();
  useEffect(() => {
    if (setupStatus.data && !setupStatus.data.setup_completed) {
      router.push('/setup');
    }
  }, [setupStatus.data, router]);

  const station = stations.find((s) => s.id === selectedStation);
  return station ? <StationView station={station} /> : <StationGrid />;
}

function StationGrid() {
  const { stations, playingStation, isPlaying } = useStation();
  const [filter, setFilter] = useState('');
  const shown = useMemo(() => {
    const q = filter.trim().toLowerCase();
    return q ? stations.filter((s) => s.name.toLowerCase().includes(q)) : stations;
  }, [stations, filter]);

  return (
    <>
      <PageHeader
        title="Stations"
        subtitle={`${stations.length} station${stations.length === 1 ? '' : 's'}`}
        actions={
          <>
            <Input
              type="search"
              value={filter}
              onChange={(e) => setFilter(e.target.value)}
              placeholder="Filter stations…"
              aria-label="Filter stations"
              className="w-56"
            />
            <Link href="/stations/manage" className={buttonClass('secondary')}>
              Manage
            </Link>
          </>
        }
      />
      {stations.length === 0 ? (
        <p className="text-muted">
          No stations yet.{' '}
          <Link href="/stations/manage" className="text-accent hover:underline">
            Create one
          </Link>{' '}
          to start listening.
        </p>
      ) : (
        <ul className="grid grid-cols-[repeat(auto-fill,minmax(180px,1fr))] gap-2">
          {shown.map((s) => (
            <li key={s.id}>
              <Link
                href={`/?station=${encodeURIComponent(s.id)}`}
                className="flex items-center gap-3 p-2 rounded-lg border border-line bg-surface hover:bg-hover transition"
              >
                <StationArt name={s.name} className="w-10 h-10 rounded-md" />
                <span className="min-w-0 flex-1">
                  <span className="block truncate font-medium">{s.name}</span>
                  <span className="block truncate text-[11.5px] text-muted">
                    {playingStation === s.id ? (isPlaying ? 'Playing' : 'Paused') : 'Station'}
                  </span>
                </span>
              </Link>
            </li>
          ))}
          {shown.length === 0 && <li className="text-muted">No stations match “{filter}”.</li>}
        </ul>
      )}
    </>
  );
}

function StationView({ station }: { station: Station }) {
  const { queue, nowPlaying, isPlaying, setIsPlaying, playingStation, setPlayingStation } = useStation();
  const feedback = useFeedbackMutation(station.id);

  const { data: playlists = [] } = usePlaylists();
  const playlistNames = Object.fromEntries(playlists.map((p) => [p.id, p.name]));
  const seed = parseStationSeed(station.seed_features as Parameters<typeof parseStationSeed>[0]);
  const seedType = seed?.type ?? seed?.seed_type;
  const isThisPlaying = playingStation === station.id;
  const active = isPlaying && isThisPlaying;

  const togglePlay = () => {
    if (active) {
      setIsPlaying(false);
    } else {
      setPlayingStation(station.id);
      setIsPlaying(true);
    }
  };

  return (
    <>
      <div className="flex flex-wrap items-center gap-3.5 mb-4">
        <StationArt name={station.name} className="w-16 h-16 rounded-xl" />
        <div className="flex-1 min-w-0">
          {seedType && (
            <div className="text-[11px] uppercase tracking-widest text-muted truncate">{SEED_TYPE_LABELS[seedType]} station</div>
          )}
          <h1 className="text-xl font-bold truncate">{station.name}</h1>
          <div className="text-muted truncate">
            {[describeStationSeed(seed, playlistNames), `${queue.length} queued`].filter(Boolean).join(' · ')}
          </div>
        </div>
        <Button variant="primary" onClick={togglePlay} disabled={queue.length === 0}>
          {active ? <PauseIcon className="w-3.5 h-3.5" /> : <PlayIcon className="w-3.5 h-3.5" />}
          {active ? 'Pause' : 'Play'}
        </Button>
        <Link href="/stations/manage" className={buttonClass('secondary', 'md', 'hidden sm:inline-flex')}>
          Edit stations
        </Link>
      </div>

      <TrackTable
        label={`${station.name} queue`}
        rows={queue}
        columns={['index', 'title', 'album', 'bpm', 'key', 'energy', 'valence', 'duration', 'like']}
        activeId={isThisPlaying ? nowPlaying?.id : null}
        paused={!isPlaying}
        onToggleLike={(t) => feedback.mutate({ id: t.id, feedback: 'like', remove: !!t.liked })}
        empty="This station has no tracks yet."
      />
    </>
  );
}
