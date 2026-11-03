import { useEffect, useRef, useState } from 'react';
import { Ban, Heart, History, Library as LibraryIcon, Search } from 'lucide-react';
import type { Track } from '@/api/types';
import { formatDuration, timeAgo } from '@/lib/format';
import { useFeedback, useFeedbackList, useHistory, useTracks } from '@/lib/queries';
import { coverUrl } from '@/player/PlayerProvider';
import { Art, Button, Empty, Spinner, Tabs } from '@/components/ui';

type Tab = 'tracks' | 'liked' | 'banned' | 'history';

function Row({ t, right }: { t: Pick<Track, 'title' | 'artist' | 'album'> & { id: string }; right?: React.ReactNode }) {
  return (
    <div className="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-4 rounded-md px-3 py-2 text-sm hover:bg-raised/70 md:grid-cols-[minmax(0,3fr)_minmax(0,2fr)_auto]">
      <span className="flex min-w-0 items-center gap-3">
        <Art src={coverUrl(t.id)} name={t.album || t.title} className="h-10 w-10" />
        <span className="min-w-0">
          <span className="block truncate font-medium">{t.title}</span>
          <span className="block truncate text-[13px] text-muted">{t.artist ?? 'Unknown artist'}</span>
        </span>
      </span>
      <span className="hidden truncate text-muted md:block">{t.album}</span>
      <span className="text-right text-[13px] tabular-nums text-muted">{right}</span>
    </div>
  );
}

function AllTracks() {
  const [q, setQ] = useState('');
  const [query, setQuery] = useState('');
  useEffect(() => {
    const t = setTimeout(() => setQuery(q.trim()), 250);
    return () => clearTimeout(t);
  }, [q]);
  const tracks = useTracks(query);
  const sentinel = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const el = sentinel.current;
    if (!el) return;
    const io = new IntersectionObserver((e) => e[0]?.isIntersecting && tracks.hasNextPage && !tracks.isFetchingNextPage && void tracks.fetchNextPage());
    io.observe(el);
    return () => io.disconnect();
  }, [tracks]);
  const rows = tracks.data?.pages.flat() ?? [];
  return (
    <>
      <label className="mb-4 flex h-10 max-w-sm items-center gap-2 rounded-md bg-raised px-3">
        <Search className="h-4 w-4 text-muted" aria-hidden />
        <input value={q} onChange={(e) => setQ(e.target.value)} placeholder="Filter by title, artist or album" aria-label="Filter tracks" className="flex-1 bg-transparent text-sm outline-none placeholder:text-faint" />
      </label>
      {tracks.isLoading ? (
        <Spinner />
      ) : rows.length ? (
        rows.map((t) => <Row key={t.id} t={t} right={formatDuration(t.duration_seconds)} />)
      ) : (
        <Empty icon={<LibraryIcon className="h-12 w-12" />} title={query ? `No tracks match “${query}”` : 'Your library is empty'}>
          {!query && 'An admin scans the music folder from the Admin page.'}
        </Empty>
      )}
      <div ref={sentinel} className="h-8" />
      {tracks.isFetchingNextPage && <Spinner />}
    </>
  );
}

function FeedbackTracks({ kind }: { kind: 'like' | 'ban' }) {
  const list = useFeedbackList(kind);
  const feedback = useFeedback();
  if (list.isLoading) return <Spinner />;
  if (!list.data?.length)
    return kind === 'like' ? (
      <Empty icon={<Heart className="h-12 w-12" />} title="No liked songs yet">Tap the heart while a song plays.</Empty>
    ) : (
      <Empty icon={<Ban className="h-12 w-12" />} title="Nothing banned">Songs you ban never play again on any station.</Empty>
    );
  return (
    <>
      {list.data.map((t) => (
        <Row
          key={t.id}
          t={t}
          right={
            <Button size="sm" variant="ghost" onClick={() => feedback.mutate({ id: t.id, feedback: kind, remove: true })}>
              {kind === 'like' ? 'Unlike' : 'Allow again'}
            </Button>
          }
        />
      ))}
    </>
  );
}

function HistoryTab() {
  const h = useHistory();
  if (h.isLoading) return <Spinner />;
  if (!h.data?.length) return <Empty icon={<History className="h-12 w-12" />} title="Nothing played yet" />;
  return (
    <>
      {h.data.map((e, i) => (
        <Row key={`${e.track_id}-${i}`} t={{ id: e.track_id, title: e.title, artist: e.artist ?? undefined, album: e.album ?? undefined }} right={timeAgo(e.played_at)} />
      ))}
    </>
  );
}

export function LibraryPage() {
  const [tab, setTab] = useState<Tab>('tracks');
  return (
    <div className="pt-2">
      <h1 className="mb-6 text-3xl font-black tracking-tight">Library</h1>
      <div className="mb-6">
        <Tabs
          label="Library"
          value={tab}
          onChange={setTab}
          tabs={[
            { id: 'tracks', label: 'All tracks' },
            { id: 'liked', label: 'Liked' },
            { id: 'history', label: 'Recently played' },
            { id: 'banned', label: 'Banned' },
          ]}
        />
      </div>
      {tab === 'tracks' && <AllTracks />}
      {tab === 'liked' && <FeedbackTracks kind="like" />}
      {tab === 'banned' && <FeedbackTracks kind="ban" />}
      {tab === 'history' && <HistoryTab />}
    </div>
  );
}
