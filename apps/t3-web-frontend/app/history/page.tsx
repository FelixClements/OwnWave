'use client';

import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { api } from '@/lib/api';
import { Cover } from '@/components/Cover';
import { keys } from '@/lib/queries';

type Tab = 'history' | 'liked' | 'banned';

export default function HistoryPage() {
  const [tab, setTab] = useState<Tab>('history');
  const { data: history } = useQuery(keys.history, () => api.listHistory());
  const { data: liked } = useQuery(keys.feedback('like'), () => api.listFeedback('like'));
  const { data: banned } = useQuery(keys.feedback('ban'), () => api.listFeedback('ban'));
  const feedbackTracks = tab === 'liked' ? liked : tab === 'banned' ? banned : undefined;

  return (
    <div className="space-y-6">
      <h2 className="text-2xl font-bold">History & Feedback</h2>

      <div className="flex gap-4 border-b border-spotify-border mb-6 overflow-x-auto">
        {(['history', 'liked', 'banned'] as Tab[]).map((t) => (
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
                className="w-10 h-10 rounded bg-spotify-elevated object-cover text-xs text-spotify-subdued shrink-0 overflow-hidden"
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
          {feedbackTracks?.map((track) => (
            <li key={track.id} className="flex items-center gap-3 py-2 border-b border-spotify-border last:border-0">
              <Cover
                id={track.id}
                title={track.title}
                className="w-10 h-10 rounded bg-spotify-elevated object-cover text-xs text-spotify-subdued shrink-0 overflow-hidden"
              />
              <div className="min-w-0 flex-1">
                <div className="text-sm font-bold truncate">{track.title}</div>
                <div className="text-xs text-spotify-subdued truncate">
                  {track.artist || 'Unknown artist'} {track.album ? `· ${track.album}` : ''}
                </div>
              </div>
            </li>
          ))}
          {!feedbackTracks?.length && <p className="text-spotify-subdued text-sm">No {tab} tracks yet.</p>}
        </ul>
      )}
    </div>
  );
}
