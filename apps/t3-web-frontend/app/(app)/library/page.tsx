'use client';

import { useEffect, useMemo, useRef, useState } from 'react';
import Link from 'next/link';
import { useSearchParams } from 'next/navigation';
import { useInfiniteQuery, useMutation, useQuery } from '@tanstack/react-query';
import { api } from '@/lib/api';
import { keys, useScanStatus } from '@/lib/queries';
import { useAuth } from '@/lib/auth';
import { ScanProgress } from '@/components/scan/ScanProgress';
import { TrackTable } from '@/components/tracks/TrackTable';
import { Button, buttonClass } from '@/components/ui/Button';
import { Input } from '@/components/ui/Field';
import { Notice, PageHeader, type NoticeValue } from '@/components/ui/Layout';
import { TabPanel, Tabs } from '@/components/ui/Tabs';

type Tab = 'tracks' | 'albums' | 'artists';

const PAGE_LIMIT = 100;

export default function LibraryPage() {
  const { user } = useAuth();
  const params = useSearchParams();
  const urlQuery = params?.get('q') ?? '';
  const [tab, setTab] = useState<Tab>('tracks');
  const [q, setQ] = useState(urlQuery);
  const [nameFilter, setNameFilter] = useState('');
  const [notice, setNotice] = useState<NoticeValue>(null);
  const [scanJobId, setScanJobId] = useState<string | null>(null);
  const sentinelRef = useRef<HTMLDivElement | null>(null);

  // The top bar search lands here as ?q=…
  useEffect(() => {
    setQ(urlQuery);
    setTab('tracks');
  }, [urlQuery]);

  const tracksQuery = useInfiniteQuery(
    keys.tracks(q),
    ({ pageParam = 0 }) => api.listTracks({ limit: PAGE_LIMIT, offset: pageParam, q }),
    {
      enabled: tab === 'tracks',
      getNextPageParam: (lastPage, allPages) =>
        lastPage.length < PAGE_LIMIT ? undefined : allPages.length * PAGE_LIMIT,
    },
  );
  const { data: albums } = useQuery(keys.albums, () => api.listAlbums());
  const { data: artists } = useQuery(keys.artists, () => api.listArtists());

  const scanStatus = useScanStatus(scanJobId);
  const rescan = useMutation(() => api.triggerScan(), {
    onMutate: () => setNotice(null),
    onSuccess: (data) => {
      setScanJobId(data.job_id ?? null);
      setNotice({ type: 'success', text: 'Library rescan started.' });
    },
    onError: (err: Error) => setNotice({ type: 'error', text: `Rescan failed: ${err.message}` }),
  });

  const allTracks = useMemo(() => tracksQuery.data?.pages.flat() ?? [], [tracksQuery.data]);
  const { hasNextPage, isFetchingNextPage, fetchNextPage } = tracksQuery;

  // Load the next page when the end of the table scrolls into view.
  useEffect(() => {
    if (!sentinelRef.current || isFetchingNextPage || tab !== 'tracks') return;
    const observer = new IntersectionObserver(
      (entries) => {
        if (entries[0].isIntersecting && hasNextPage) void fetchNextPage();
      },
      { rootMargin: '200px' },
    );
    observer.observe(sentinelRef.current);
    return () => observer.disconnect();
  }, [hasNextPage, isFetchingNextPage, fetchNextPage, tab, q]);

  const filterNames = <T,>(items: T[] | undefined, name: (t: T) => string) => {
    const f = nameFilter.trim().toLowerCase();
    return (items ?? []).filter((i) => name(i).toLowerCase().includes(f));
  };

  return (
    <>
      <PageHeader
        title="Library"
        actions={
          user?.is_admin && (
            <Button onClick={() => rescan.mutate()} loading={rescan.isLoading}>
              {rescan.isLoading ? 'Starting…' : 'Rescan library'}
            </Button>
          )
        }
      />

      <div className="space-y-3 mb-3">
        <Notice notice={notice} />
        {scanStatus.data && <ScanProgress scan={scanStatus.data} />}
      </div>

      <Tabs
        label="Library sections"
        idPrefix="library"
        value={tab}
        onChange={setTab}
        tabs={[
          { id: 'tracks', label: 'Tracks' },
          { id: 'albums', label: 'Albums', count: albums?.length },
          { id: 'artists', label: 'Artists', count: artists?.length },
        ]}
      />

      {tab === 'tracks' && (
        <TabPanel idPrefix="library" id="tracks">
          <Input
            type="search"
            value={q}
            onChange={(e) => setQ(e.target.value)}
            placeholder="Search tracks, artists, albums…"
            aria-label="Search tracks"
            className="mb-3 max-w-md"
          />
          <TrackTable
            label="Tracks"
            rows={allTracks}
            columns={['title', 'album', 'track_number', 'duration', 'sample_rate', 'channels', 'actions']}
            actions={(t) => (
              <Link
                href={`/similar?track=${encodeURIComponent(t.id)}`}
                className={buttonClass('secondary', 'sm')}
                aria-label={`Find tracks similar to ${t.title}`}
              >
                Similar
              </Link>
            )}
            empty={tracksQuery.isLoading ? 'Loading…' : q ? `No tracks match “${q}”.` : 'No tracks yet.'}
          />
          <div className="flex items-center justify-between mt-3 text-muted">
            <span className="tabular-nums" aria-live="polite">
              {allTracks.length} track{allTracks.length === 1 ? '' : 's'} loaded
            </span>
            {hasNextPage && (
              <Button size="sm" onClick={() => void fetchNextPage()} loading={isFetchingNextPage}>
                Load more
              </Button>
            )}
          </div>
          <div ref={sentinelRef} className="h-1" />
        </TabPanel>
      )}

      {tab !== 'tracks' && (
        <TabPanel idPrefix="library" id={tab}>
          <Input
            type="search"
            value={nameFilter}
            onChange={(e) => setNameFilter(e.target.value)}
            placeholder={`Filter ${tab}…`}
            aria-label={`Filter ${tab}`}
            className="mb-3 max-w-md"
          />
          <ul className="grid grid-cols-[repeat(auto-fill,minmax(220px,1fr))] gap-x-4">
            {(tab === 'albums'
              ? filterNames(albums, (a) => a.title).map((a) => ({ id: a.id, name: a.title }))
              : filterNames(artists, (a) => a.name)
            ).map((item) => (
              <li key={item.id} className="truncate py-1.5 border-b border-line/60">
                {item.name}
              </li>
            ))}
          </ul>
        </TabPanel>
      )}
    </>
  );
}
