import { useState } from 'react';
import { Plus, Radio } from 'lucide-react';
import { useStations } from '@/lib/queries';
import { CardGrid, StationCard } from '@/components/media';
import { StationEditor } from '@/components/StationEditor';
import { Button, Empty, Spinner, Tabs } from '@/components/ui';

type Filter = 'all' | 'shared' | 'mine';

export function StationsPage() {
  const { data, isLoading } = useStations();
  const [filter, setFilter] = useState<Filter>('all');
  const [creating, setCreating] = useState(false);
  if (isLoading) return <Spinner />;
  const list = (data ?? []).filter((s) => filter === 'all' || (filter === 'shared' ? s.shared : !s.shared));
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
            { id: 'all', label: 'All', count: data?.length },
            { id: 'shared', label: 'Shared', count: data?.filter((s) => s.shared).length },
            { id: 'mine', label: 'Yours', count: data?.filter((s) => !s.shared).length },
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
          A station plays the songs of one or more playlists that are in your library, optionally only some genres.
        </Empty>
      )}
      <StationEditor open={creating} onClose={() => setCreating(false)} />
    </div>
  );
}
