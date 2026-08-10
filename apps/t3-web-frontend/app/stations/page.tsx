'use client';

import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { trpc } from '@/lib/trpc/client';

function PlayIcon({ className }: { className?: string }) {
  return (
    <svg
      className={className}
      viewBox="0 0 24 24"
      fill="currentColor"
      xmlns="http://www.w3.org/2000/svg"
    >
      <path d="M8 5v14l11-7L8 5z" />
    </svg>
  );
}

export default function StationsPage() {
  const router = useRouter();
  const { data: stations } = trpc.stations.useQuery();

  const playStation = (id: string) => {
    router.push(`/?station=${encodeURIComponent(id)}`);
  };

  return (
    <div className="min-h-screen bg-spotify-bg text-spotify-text p-4 md:p-8">
      <header className="flex items-center justify-between mb-8">
        <h1 className="text-3xl font-bold">Stations</h1>
        <Link
          href="/stations/manage"
          className="px-4 py-2 rounded bg-spotify-elevated text-sm font-semibold hover:bg-spotify-card-hover transition"
        >
          Manage
        </Link>
      </header>

      {(stations || []).length === 0 && (
        <p className="text-spotify-subdued">
          No stations yet. Go to <Link href="/stations/manage" className="text-spotify-green hover:underline">Manage</Link> to create one.
        </p>
      )}

      <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-4 lg:grid-cols-5 gap-6">
        {(stations || []).map((station) => (
          <button
            key={station.id}
            onClick={() => playStation(station.id)}
            className="group relative bg-spotify-card rounded-lg p-4 hover:bg-spotify-card-hover transition text-left"
          >
            <div className="w-full aspect-square rounded-md bg-gradient-to-br from-spotify-green to-spotify-green-hover shadow-lg mb-4 flex items-center justify-center text-black font-bold text-2xl">
              {station.name.charAt(0).toUpperCase()}
            </div>
            <h3 className="font-bold text-spotify-text truncate">{station.name}</h3>
            <p className="text-sm text-spotify-subdued">AI Radio</p>
            <div className="absolute bottom-16 right-4 w-12 h-12 rounded-full bg-spotify-green shadow-lg opacity-0 group-hover:opacity-100 transition transform translate-y-2 group-hover:translate-y-0 flex items-center justify-center text-black">
              <PlayIcon className="w-5 h-5 ml-0.5" />
            </div>
          </button>
        ))}
      </div>
    </div>
  );
}
