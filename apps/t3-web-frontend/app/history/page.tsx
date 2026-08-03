'use client';

import { useState } from 'react';
import Link from 'next/link';
import { trpc } from '@/lib/trpc/client';
import { getCoverUrl } from '@/lib/api';

type Tab = 'history' | 'liked' | 'skipped' | 'banned';

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

export default function HistoryPage() {
  const [tab, setTab] = useState<Tab>('history');
  const { data: history } = trpc.history.useQuery();
  const { data: liked } = trpc.liked.useQuery();
  const { data: skipped } = trpc.skipped.useQuery();
  const { data: banned } = trpc.banned.useQuery();

  return (
    <div className="min-h-screen bg-spotify-bg text-spotify-text">
      <header className="h-16 flex items-center justify-between px-4 md:px-8 bg-spotify-bg/95 sticky top-0 z-10">
        <div className="flex items-center gap-4">
          <Link href="/" className="text-sm font-bold hover:text-spotify-green transition">
            ← Home
          </Link>
          <h2 className="text-sm font-bold">History & Feedback</h2>
        </div>
      </header>

      <main className="p-4 md:p-8">
        <div className="flex gap-4 border-b border-spotify-border mb-6 overflow-x-auto">
          {(['history', 'liked', 'skipped', 'banned'] as Tab[]).map((t) => (
            <button
              key={t}
              onClick={() => setTab(t)}
              className={`pb-2 text-sm font-semibold capitalize transition whitespace-nowrap ${
                tab === t ? 'text-spotify-green border-b-2 border-spotify-green' : 'text-spotify-subdued hover:text-spotify-text'
              }`}
            >
              {t}
            </button>
          ))}
        </div>

        {tab === 'history' && (
          <ul className="space-y-1 bg-spotify-card rounded-lg p-4">
            {history?.map((entry) => (
              <li key={`${entry.track_id}-${entry.played_at}`} className="flex items-center gap-3 py-2 border-b border-spotify-border last:border-0">
                <Cover
                  id={entry.track_id}
                  title={entry.title}
                  className="w-10 h-10 rounded bg-spotify-elevated object-cover flex items-center justify-center text-xs text-spotify-subdued"
                />
                <div className="min-w-0 flex-1">
                  <div className="text-sm font-bold truncate">{entry.title}</div>
                  <div className="text-xs text-spotify-subdued truncate">
                    {entry.artist || 'Unknown artist'} {entry.album ? `· ${entry.album}` : ''}
                  </div>
                </div>
                <div className="text-xs text-spotify-subdued shrink-0">
                  {new Date(entry.played_at).toLocaleString()}
                </div>
              </li>
            ))}
            {(!history || history.length === 0) && <p className="text-spotify-subdued text-sm">No history yet.</p>}
          </ul>
        )}

        {tab !== 'history' && (
          <ul className="space-y-1 bg-spotify-card rounded-lg p-4">
            {{
              liked,
              skipped,
              banned,
            }[tab]?.map((track) => (
              <li key={track.id} className="flex items-center gap-3 py-2 border-b border-spotify-border last:border-0">
                <Cover
                  id={track.id}
                  title={track.title}
                  className="w-10 h-10 rounded bg-spotify-elevated object-cover flex items-center justify-center text-xs text-spotify-subdued"
                />
                <div className="min-w-0 flex-1">
                  <div className="text-sm font-bold truncate">{track.title}</div>
                  <div className="text-xs text-spotify-subdued truncate">
                    {track.artist || 'Unknown artist'} {track.album ? `· ${track.album}` : ''}
                  </div>
                </div>
              </li>
            ))}
            {({
              liked,
              skipped,
              banned,
            }[tab]?.length === 0 || !{
              liked,
              skipped,
              banned,
            }[tab]) && <p className="text-spotify-subdued text-sm">No {tab} tracks yet.</p>}
          </ul>
        )}
      </main>
    </div>
  );
}
