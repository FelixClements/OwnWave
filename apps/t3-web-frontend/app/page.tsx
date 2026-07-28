'use client';

import { useState } from 'react';
import { trpc } from '@/lib/trpc/client';
import { Player } from '@/components/Player';

export default function Home() {
  const { data: stations } = trpc.stations.useQuery();
  const [selectedStation, setSelectedStation] = useState<string | null>(null);
  const { data: queue } = trpc.queue.useQuery(
    { id: selectedStation || '' },
    { enabled: !!selectedStation }
  );

  return (
    <main className="min-h-screen p-8">
      <h1 className="text-4xl font-bold mb-8 text-center">OwnWave</h1>

      <section className="max-w-2xl mx-auto mb-8">
        <h2 className="text-xl font-semibold mb-4">Stations</h2>
        {stations && stations.length === 0 && (
          <p className="text-gray-400">No stations yet. Scan your library and build a station first.</p>
        )}
        <div className="grid grid-cols-1 gap-3">
          {stations?.map((station) => (
            <button
              key={station.id}
              onClick={() => setSelectedStation(station.id)}
              className={`p-4 rounded-lg text-left transition ${
                selectedStation === station.id
                  ? 'bg-blue-600'
                  : 'bg-zinc-800 hover:bg-zinc-700'
              }`}
            >
              {station.name}
            </button>
          ))}
        </div>
      </section>

      {queue && queue.length > 0 && (
        <section className="max-w-3xl mx-auto">
          <h2 className="text-xl font-semibold mb-4">Now Playing</h2>
          <Player queue={queue} />

          <ol className="mt-6 space-y-2">
            {queue.map((track, i) => (
              <li
                key={track.id}
                className="flex justify-between p-3 bg-zinc-900 rounded"
              >
                <span>{i + 1}. {track.title}</span>
                <span className="text-zinc-500 text-sm">
                  {track.bpm.toFixed(0)} BPM · {track.key}
                </span>
              </li>
            ))}
          </ol>
        </section>
      )}
    </main>
  );
}
