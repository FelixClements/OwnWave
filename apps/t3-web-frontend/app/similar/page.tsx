'use client';

import { useState } from 'react';
import { useSearchParams } from 'next/navigation';
import { trpc } from '@/lib/trpc/client';
import { useStation } from '@/lib/station';
import { getCoverUrl } from '@/lib/api';

function Cover({ id, title, className }: { id: string; title: string; className?: string }) {
  const [error, setError] = useState(false);
  if (error) {
    return (
      <div className={className}>
        <span className="font-bold">{title.charAt(0).toUpperCase()}</span>
      </div>
    );
  }
  return <img src={getCoverUrl(id)} alt="" className={className} onError={() => setError(true)} />;
}

export default function SimilarPage() {
  const search = useSearchParams() ?? undefined;
  const trackId = search?.get('track');
  const { data: tracks } = trpc.tracks.useQuery({ limit: 1000 });
  const { data: similar } = trpc.similar.useQuery(
    { id: trackId ?? '' },
    { enabled: !!trackId }
  );
  const utils = trpc.useContext();
  const { setSelectedStation } = useStation();
  const create = trpc.createStation.useMutation({
    onSuccess: (data) => {
      setStationName('');
      utils.stations.invalidate();
      setSelectedStation(data.station_id);
    },
    onError: (err) => {
      console.error('create station failed', err);
    },
  });

  const [selected, setSelected] = useState(trackId ?? '');
  const [stationName, setStationName] = useState('');

  const handleSelect = (id: string) => {
    setSelected(id);
    const url = new URL(window.location.href);
    url.searchParams.set('track', id);
    window.history.replaceState({}, '', url.toString());
  };

  const handleCreateStation = (e: React.FormEvent) => {
    e.preventDefault();
    if (!selected || !stationName.trim()) return;
    create.mutate({
      name: stationName.trim(),
      seed_type: 'track',
      track_id: selected,
    });
  };

  return (
    <div className="space-y-6">
      <h2 className="text-2xl font-bold">Similar Tracks</h2>

      <div className="flex flex-col md:flex-row md:items-end md:justify-between gap-4">
        <div className="max-w-xl flex-1">
          <label className="block text-sm font-semibold mb-2">Seed track</label>
        <select
          value={selected}
          onChange={(e) => handleSelect(e.target.value)}
          className="w-full px-3 py-2 rounded bg-spotify-elevated text-spotify-text border border-spotify-border focus:outline-none focus:border-spotify-green"
        >
          <option value="">Choose a track</option>
          {tracks?.map((track) => (
            <option key={track.id} value={track.id}>
              {track.title} {track.artist ? `— ${track.artist}` : ''}
            </option>
          ))}
        </select>
      </div>

      {selected && (
        <form onSubmit={handleCreateStation} className="w-full md:max-w-sm space-y-3 p-4 rounded bg-spotify-card">
          <h3 className="text-lg font-bold">Create Station</h3>
          <div className="space-y-2">
            <label htmlFor="station-name" className="text-sm font-semibold">
              Station Name
            </label>
            <input
              id="station-name"
              type="text"
              value={stationName}
              onChange={(e) => setStationName(e.target.value)}
              placeholder="e.g. Like this track"
              className="w-full px-3 py-2 rounded bg-spotify-elevated text-spotify-text placeholder-spotify-subdued border border-spotify-border focus:outline-none focus:border-spotify-green"
              required
            />
          </div>
          {create.error && (
            <p className="text-sm text-red-500">{create.error.message}</p>
          )}
          <button
            type="submit"
            disabled={create.isLoading || !stationName.trim()}
            className="w-full px-4 py-2 rounded bg-spotify-green text-black font-semibold hover:bg-spotify-green-hover transition disabled:opacity-50"
          >
            {create.isLoading ? 'Creating...' : 'Create Station'}
          </button>
        </form>
      )}
      </div>

      {similar && similar.length > 0 && (
        <ul className="space-y-1 bg-spotify-card rounded-lg p-4">
          {similar.map((t) => {
            const track = tracks?.find((tr) => tr.id === t.track_id);
            return (
              <li
                key={t.track_id}
                className="flex items-center gap-3 py-2 border-b border-spotify-border last:border-0"
              >
                <Cover
                  id={t.track_id}
                  title={track?.title ?? t.track_id}
                  className="w-10 h-10 rounded bg-spotify-elevated object-cover flex items-center justify-center text-xs text-spotify-subdued"
                />
                <div className="min-w-0 flex-1">
                  <div className="text-sm font-bold truncate">
                    {track?.title ?? t.track_id}
                  </div>
                  <div className="text-xs text-spotify-subdued truncate">
                    {track?.artist ?? 'Unknown artist'} {track?.album ? `· ${track.album}` : ''}
                  </div>
                </div>
                <div className="text-xs text-spotify-subdued shrink-0">
                  {t.distance.toFixed(4)}
                </div>
              </li>
            );
          })}
        </ul>
      )}

      {selected && (!similar || similar.length === 0) && (
        <p className="text-spotify-subdued text-sm">No similar tracks found.</p>
      )}
    </div>
  );
}
