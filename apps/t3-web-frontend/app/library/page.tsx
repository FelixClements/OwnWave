'use client';

import { useState, useEffect, useRef } from 'react';
import Link from 'next/link';
import { trpc } from '@/lib/trpc/client';
import { getCoverUrl } from '@/lib/api';

type Tab = 'tracks' | 'albums' | 'artists';

function Cover({ id, title, className }: { id: string; title: string; className?: string }) {
  const [error, setError] = useState(false);
  if (error) {
    return (
      <div className={className}>
        <span className="font-bold">{title.charAt(0).toUpperCase()}</span>
      </div>
    );
  }
  return (
    <img
      src={getCoverUrl(id)}
      alt=""
      className={className}
      onError={() => setError(true)}
    />
  );
}

const PAGE_LIMIT = 20;

export default function LibraryPage() {
  const [tab, setTab] = useState<Tab>('tracks');
  const [q, setQ] = useState('');
  const [search, setSearch] = useState('');
  const sentinelRef = useRef<HTMLDivElement | null>(null);

  const tracksQuery = trpc.tracks.useInfiniteQuery(
    { limit: PAGE_LIMIT, q },
    {
      enabled: tab === 'tracks',
      getNextPageParam: (lastPage, allPages) =>
        !lastPage || lastPage.length < PAGE_LIMIT ? undefined : allPages.length * PAGE_LIMIT,
    }
  );

  const { data: albums } = trpc.albums.useQuery();
  const { data: artists } = trpc.artists.useQuery();
  const rescan = trpc.rescan.useMutation({
    onSuccess: () => alert('Library rescan started.'),
    onError: () => alert('Rescan failed.'),
  });

  const allTracks = tracksQuery.data?.pages.flat() ?? [];

  useEffect(() => {
    if (!sentinelRef.current || tracksQuery.isFetchingNextPage) return;
    const observer = new IntersectionObserver(
      (entries) => {
        if (entries[0].isIntersecting && tracksQuery.hasNextPage) {
          tracksQuery.fetchNextPage();
        }
      },
      { rootMargin: '200px' }
    );
    observer.observe(sentinelRef.current);
    return () => observer.disconnect();
  }, [tracksQuery.hasNextPage, tracksQuery.isFetchingNextPage, tracksQuery.fetchNextPage, q]);

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h2 className="text-2xl font-bold">Library</h2>
        <button
          onClick={() => rescan.mutate()}
          disabled={rescan.isLoading}
          className="text-xs font-semibold px-3 py-1.5 rounded-full bg-spotify-elevated text-spotify-text hover:bg-spotify-card-hover transition disabled:opacity-50"
        >
          {rescan.isLoading ? 'Rescanning...' : 'Rescan library'}
        </button>
      </div>

      <div className="flex gap-4 border-b border-spotify-border mb-6">
        {(['tracks', 'albums', 'artists'] as Tab[]).map((t) => (
          <button
            key={t}
            onClick={() => setTab(t)}
            className={`pb-2 text-sm font-semibold capitalize transition ${
              tab === t ? 'text-spotify-green border-b-2 border-spotify-green' : 'text-spotify-subdued hover:text-spotify-text'
            }`}
          >
            {t}
          </button>
        ))}
      </div>

      {tab === 'tracks' && (
        <div className="space-y-4">
          <input
            type="text"
            value={q}
            onChange={(e) => setQ(e.target.value)}
            placeholder="Search tracks..."
            className="w-full px-4 py-2 rounded-full bg-spotify-elevated text-spotify-text placeholder-spotify-subdued border border-spotify-border focus:outline-none focus:border-spotify-green"
          />
          <section className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5 gap-6">
            {allTracks.map((track) => (
              <div
                key={track.id}
                className="bg-spotify-card rounded-lg p-4 text-left"
              >
                <Cover
                  id={track.id}
                  title={track.title}
                  className="w-full aspect-square rounded-md bg-spotify-elevated object-cover mb-4 flex items-center justify-center"
                />
                <h3 className="font-bold text-spotify-text truncate">{track.title}</h3>
                <p className="text-sm text-spotify-subdued truncate">{track.artist || 'Unknown artist'}</p>
                {track.album && <p className="text-xs text-spotify-subdued truncate">{track.album}</p>}
                <Link
                  href={`/similar?track=${encodeURIComponent(track.id)}`}
                  className="mt-2 inline-block text-xs font-semibold text-spotify-green hover:underline"
                >
                  Find similar
                </Link>
              </div>
            ))}
          </section>
          {tracksQuery.isFetchingNextPage && (
            <p className="text-center text-spotify-subdued text-sm">Loading more...</p>
          )}
          <div ref={sentinelRef} className="h-1" />
        </div>
      )}

      {tab === 'albums' && (
        <div className="space-y-4">
          <input
            type="text"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Search albums..."
            className="w-full px-4 py-2 rounded-full bg-spotify-elevated text-spotify-text placeholder-spotify-subdued border border-spotify-border focus:outline-none focus:border-spotify-green"
          />
          <section className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 gap-6">
            {albums?.filter((album) => album.title.toLowerCase().includes(search.toLowerCase())).map((album) => (
            <div key={album.id} className="bg-spotify-card rounded-lg p-4 text-left">
              <div className="w-full aspect-square rounded-md bg-spotify-elevated mb-4 flex items-center justify-center text-2xl font-bold text-spotify-subdued">
                {album.title.charAt(0).toUpperCase()}
              </div>
              <h3 className="font-bold text-spotify-text truncate">{album.title}</h3>
            </div>
          ))}
          </section>
        </div>
      )}

      {tab === 'artists' && (
        <div className="space-y-4">
          <input
            type="text"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Search artists..."
            className="w-full px-4 py-2 rounded-full bg-spotify-elevated text-spotify-text placeholder-spotify-subdued border border-spotify-border focus:outline-none focus:border-spotify-green"
          />
          <section className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 gap-6">
            {artists?.filter((artist) => artist.name.toLowerCase().includes(search.toLowerCase())).map((artist) => (
            <div key={artist.id} className="bg-spotify-card rounded-lg p-4 text-left">
              <div className="w-full aspect-square rounded-md bg-spotify-elevated mb-4 flex items-center justify-center text-2xl font-bold text-spotify-subdued">
                {artist.name.charAt(0).toUpperCase()}
              </div>
              <h3 className="font-bold text-spotify-text truncate">{artist.name}</h3>
            </div>
          ))}
          </section>
        </div>
      )}
    </div>
  );
}
