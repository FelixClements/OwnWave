import { useState } from 'react';
import { Plus, Radio } from 'lucide-react';
import { useStations } from '@/lib/queries';
import { CardGrid, StationCard } from '@/components/media';
import { StationEditor } from '@/components/StationEditor';
import { Button, Empty, Spinner, Tabs } from '@/components/ui';

type Filter = 'all' | 'genre' | 'style' | 'shared' | 'mine';

export function StationsPage() {
  const { data, isLoading } = useStations();
  const [filter, setFilter] = useState<Filter>('all');
  const [creating, setCreating] = useState(false);
  if (isLoading) return <Spinner />;
  const all = data ?? [];
  const matches: Record<Filter, (s: (typeof all)[number]) => boolean> = {
    all: () => true,
    genre: (s) => s.kind === 'genre' && !s.parent_genre,
    style: (s) => !!s.parent_genre,
    shared: (s) => s.shared && s.kind !== 'genre',
    mine: (s) => !s.shared,
  };
  const list = all.filter(matches[filter]);
  return (
    <div className="pt-2">
      <div className="mb-6 flex flex-wrap items-center justify-between gap-4">
        <h1 className="text-3xl font-black tracking-tight">Stations</h1>
        <Button variant="primary" onClick={() => setCreating(true)}>
          <Plus className="h-4 w-4" /> New station
        </Button>
      </div>
      <div className="mb-6">
        <Tabs
          label="Filter stations"
          value={filter}
          onChange={setFilter}
          tabs={[
            { id: 'all', label: 'All', count: all.length },
            { id: 'genre', label: 'Genres', count: all.filter(matches.genre).length },
            { id: 'style', label: 'Styles', count: all.filter(matches.style).length },
            { id: 'shared', label: 'Shared', count: all.filter(matches.shared).length },
            { id: 'mine', label: 'Yours', count: all.filter(matches.mine).length },
          ]}
        />
      </div>
      {list.length ? (
        <CardGrid>
          {list.map((s) => (
            <StationCard key={s.id} station={s} />
          ))}
        </CardGrid>
      ) : (
        <Empty icon={<Radio className="h-12 w-12" />} title="Nothing here yet">
          A station plays the songs of one or more playlists that are in your library, optionally only some genres. Each genre and
          style gets its own station once enough of your library's songs are in it.
        </Empty>
      )}
      <StationEditor open={creating} onClose={() => setCreating(false)} />
    </div>
  );
}
