import { useParams } from 'react-router-dom';
import { useBrowse, useStations } from '@/lib/queries';
import { CardGrid, StationCard } from '@/components/media';
import { Empty, Spinner } from '@/components/ui';
import { nameColor, rgb } from '@/lib/color';
import type { Station } from '@/api/types';

/**
 * A genre's page (from the browse tree): its stations, then the stations of
 * its styles, e.g. House, then Deep House and Tech House.
 */
export function GenrePage() {
  const { id = '' } = useParams();
  const node = useBrowse(`genre:${id}`);
  const { data: stations = [] } = useStations();
  if (node.isLoading) return <Spinner />;
  if (!node.data) return <Empty title="Genre not found" />;
  const byId = new Map(stations.map((s) => [s.id, s]));
  // The browse node keeps the order: the genre's stations first, then its styles'.
  const list = node.data.items.map((i) => byId.get(i.station_id ?? '')).filter((s): s is Station => !!s);
  const own = list.filter((s) => s.genres.includes(id));
  const styles = list.filter((s) => !s.genres.includes(id));
  const c = nameColor(node.data.title);
  return (
    <>
      <header className="-mx-4 -mt-16 mb-8 px-4 pb-8 pt-28 md:-mx-8 md:px-8" style={{ backgroundImage: `linear-gradient(to bottom, ${rgb(c)}, transparent)` }}>
        <div className="text-[13px] font-semibold">Genre</div>
        <h1 className="text-5xl font-black tracking-tight">{node.data.title}</h1>
      </header>
      {list.length ? (
        <>
          {own.length > 0 && (
            <CardGrid>
              {own.map((s) => (
                <StationCard key={s.id} station={s} />
              ))}
            </CardGrid>
          )}
          {styles.length > 0 && (
            <section className={own.length ? 'mt-10' : ''}>
              <h2 className="mb-4 text-2xl font-bold tracking-tight">Styles</h2>
              <CardGrid>
                {styles.map((s) => (
                  <StationCard key={s.id} station={s} />
                ))}
              </CardGrid>
            </section>
          )}
        </>
      ) : (
        <Empty title={`No ${node.data.title} stations`} />
      )}
    </>
  );
}
