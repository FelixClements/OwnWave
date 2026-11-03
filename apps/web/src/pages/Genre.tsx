import { useParams } from 'react-router-dom';
import { useBrowse, useStations } from '@/lib/queries';
import { CardGrid, StationCard } from '@/components/media';
import { Empty, Spinner } from '@/components/ui';
import { nameColor, rgb } from '@/lib/color';

/** Stations with one genre (from the browse tree). */
export function GenrePage() {
  const { id = '' } = useParams();
  const node = useBrowse(`genre:${id}`);
  const { data: stations = [] } = useStations();
  if (node.isLoading) return <Spinner />;
  if (!node.data) return <Empty title="Genre not found" />;
  const ids = new Set(node.data.items.map((i) => i.station_id));
  const list = stations.filter((s) => ids.has(s.id));
  const c = nameColor(node.data.title);
  return (
    <>
      <header className="-mx-4 -mt-16 mb-8 px-4 pb-8 pt-28 md:-mx-8 md:px-8" style={{ backgroundImage: `linear-gradient(to bottom, ${rgb(c)}, transparent)` }}>
        <div className="text-[13px] font-semibold">Genre</div>
        <h1 className="text-5xl font-black tracking-tight">{node.data.title}</h1>
      </header>
      {list.length ? (
        <CardGrid>
          {list.map((s) => (
            <StationCard key={s.id} station={s} />
          ))}
        </CardGrid>
      ) : (
        <Empty title={`No ${node.data.title} stations`} />
      )}
    </>
  );
}
