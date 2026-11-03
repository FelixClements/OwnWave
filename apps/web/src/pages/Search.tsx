import { useEffect, useState } from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import { Search as SearchIcon, X } from 'lucide-react';
import type { Station } from '@/api/types';
import { nameColor, rgb } from '@/lib/color';
import { useBrowse, useSearch, useStationSearch, useStations } from '@/lib/queries';
import { coverUrl, usePlayer } from '@/player/PlayerProvider';
import { CardGrid, Section, StationCard, stationCover } from '@/components/media';
import { Art, Empty, IconButton, PlayButton, Spinner } from '@/components/ui';

function useDebounced<T>(value: T, ms = 250): T {
  const [v, setV] = useState(value);
  useEffect(() => {
    const t = setTimeout(() => setV(value), ms);
    return () => clearTimeout(t);
  }, [value, ms]);
  return v;
}

const REASONS: Record<string, string> = {
  station: 'Station',
  genre: 'Genre match',
  playlist: 'From a matching playlist',
  artist: 'Has this artist',
};

function TopResult({ station, reason }: { station: Station; reason: string }) {
  const player = usePlayer();
  const active = player.station?.id === station.id;
  return (
    <Link to={`/station/${station.id}`} className="group relative flex flex-col gap-5 rounded-lg bg-card p-5 transition hover:bg-raised">
      <Art src={stationCover(station.id)} name={station.name} className="h-24 w-24 shadow-lift" />
      <div>
        <div className="text-3xl font-black tracking-tight">{station.name}</div>
        <div className="mt-1 text-sm text-muted">
          <span className="rounded-full bg-base px-2.5 py-1 text-[12px] font-bold text-ink">{REASONS[reason] ?? 'Station'}</span>
        </div>
      </div>
      <PlayButton
        size="md"
        playing={active && player.playing}
        label={`Play ${station.name}`}
        onClick={() => (active ? player.toggle() : void player.playStation(station))}
        className="absolute bottom-5 right-5 opacity-0 transition group-hover:opacity-100"
      />
    </Link>
  );
}

function BrowseGenres() {
  const { data, isLoading } = useBrowse('genres');
  if (isLoading) return <Spinner />;
  if (!data?.items.length) return <Empty title="Nothing to browse yet">Genres appear here once stations use them.</Empty>;
  return (
    <Section title="Browse genres">
      <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-4 2xl:grid-cols-6">
        {data.items.map((g) => {
          const c = nameColor(g.title);
          return (
            <Link
              key={g.id}
              to={`/genre/${g.id.replace(/^genre:/, '')}`}
              className="relative aspect-[16/10] overflow-hidden rounded-lg p-4 text-xl font-black transition hover:brightness-110"
              style={{ backgroundColor: rgb(c) }}
            >
              {g.title}
              <span className="absolute bottom-3 left-4 text-[12px] font-semibold text-white/70">{g.subtitle}</span>
            </Link>
          );
        })}
      </div>
    </Section>
  );
}

export function SearchPage() {
  const [params, setParams] = useSearchParams();
  const [q, setQ] = useState(params.get('q') ?? '');
  const query = useDebounced(q.trim());
  const stations = useStationSearch(query);
  const library = useSearch(query);
  const { data: all = [] } = useStations();
  const byId = new Map(all.map((s) => [s.id, s]));

  useEffect(() => {
    setParams(query ? { q: query } : {}, { replace: true });
  }, [query, setParams]);

  const hits = (stations.data?.results ?? []).map((h) => ({ hit: h, station: byId.get(h.station_id ?? '') })).filter((x) => x.station);
  const tracks = library.data?.tracks ?? [];
  const artists = library.data?.artists ?? [];
  const searching = query.length > 1;

  return (
    <div className="pt-1">
      <div className="sticky top-3 z-30 -mt-14 mb-8 ml-0 max-w-md md:ml-24">
        <label className="flex h-12 items-center gap-3 rounded-full bg-raised px-4 ring-ink focus-within:ring-2">
          <SearchIcon className="h-5 w-5 text-muted" aria-hidden />
          <input
            autoFocus
            value={q}
            onChange={(e) => setQ(e.target.value)}
            placeholder="Stations, genres, playlists, artists"
            aria-label="Search"
            className="min-w-0 flex-1 bg-transparent text-[15px] outline-none placeholder:text-faint"
          />
          {q && (
            <IconButton label="Clear search" size="sm" onClick={() => setQ('')}>
              <X className="h-4 w-4" />
            </IconButton>
          )}
        </label>
      </div>

      {!searching ? (
        <BrowseGenres />
      ) : stations.isLoading && library.isLoading ? (
        <Spinner />
      ) : hits.length === 0 && tracks.length === 0 && artists.length === 0 ? (
        <Empty title={`No results for “${query}”`}>Try a genre, a playlist name or an artist.</Empty>
      ) : (
        <>
          {hits.length > 0 && (
            <div className="mb-10 grid gap-6 lg:grid-cols-[minmax(0,2fr)_minmax(0,3fr)]">
              <div>
                <h2 className="mb-3 text-2xl font-bold">Top result</h2>
                <TopResult station={hits[0].station!} reason={hits[0].hit.reason} />
              </div>
              <div>
                <h2 className="mb-3 text-2xl font-bold">Tracks</h2>
                {tracks.length === 0 ? (
                  <p className="text-sm text-muted">No tracks match.</p>
                ) : (
                  <ul>
                    {tracks.slice(0, 4).map((t) => (
                      <li key={t.id} className="flex items-center gap-3 rounded-md p-2 hover:bg-raised">
                        <Art src={coverUrl(t.id)} name={t.album || t.title} className="h-10 w-10" />
                        <div className="min-w-0">
                          <div className="truncate font-medium">{t.title}</div>
                          <div className="truncate text-[13px] text-muted">{[t.artist, t.album].filter(Boolean).join(' · ')}</div>
                        </div>
                      </li>
                    ))}
                  </ul>
                )}
              </div>
            </div>
          )}
          {hits.length > 1 && (
            <Section title="Stations">
              <CardGrid>
                {hits.slice(1).map(({ hit, station }) => (
                  <StationCard key={station!.id} station={station!} subtitle={REASONS[hit.reason]} />
                ))}
              </CardGrid>
            </Section>
          )}
          {hits.length === 0 && tracks.length > 0 && (
            <Section title="Tracks in your library">
              <p className="mb-3 text-sm text-muted">No station plays these yet: add a playlist that has them.</p>
              <ul className="grid gap-1 md:grid-cols-2">
                {tracks.slice(0, 20).map((t) => (
                  <li key={t.id} className="flex items-center gap-3 rounded-md p-2 hover:bg-raised">
                    <Art src={coverUrl(t.id)} name={t.album || t.title} className="h-10 w-10" />
                    <div className="min-w-0">
                      <div className="truncate font-medium">{t.title}</div>
                      <div className="truncate text-[13px] text-muted">{t.artist}</div>
                    </div>
                  </li>
                ))}
              </ul>
            </Section>
          )}
          {artists.length > 0 && (
            <Section title="Artists">
              <div className="flex flex-wrap gap-2">
                {artists.slice(0, 20).map((a) => (
                  <button key={a.id} type="button" onClick={() => setQ(a.name)} className="rounded-full bg-raised px-4 py-2 text-sm font-semibold hover:bg-line">
                    {a.name}
                  </button>
                ))}
              </div>
            </Section>
          )}
        </>
      )}
    </div>
  );
}
