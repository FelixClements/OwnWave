import { useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import clsx from 'clsx';
import { Radio, Sparkles, X } from 'lucide-react';
import type { Station, StationSuggestion } from '@/api/types';
import { useAuth } from '@/lib/auth';
import { greeting, plural } from '@/lib/format';
import { useBrowse, useDismissSuggestion, useSaveStation, useStations, useSuggestions } from '@/lib/queries';
import { usePlayer } from '@/player/PlayerProvider';
import { CardGrid, Section, StationCard, stationCover } from '@/components/media';
import { StationEditor } from '@/components/StationEditor';
import { Art, Button, Empty, ErrorText, IconButton, PlayButton, Spinner } from '@/components/ui';

/** Wide tile for the quick-pick grid at the top, like a shortcut. */
function QuickTile({ station }: { station: Station }) {
  const player = usePlayer();
  const active = player.station?.id === station.id;
  return (
    <Link to={`/station/${station.id}`} className="group relative flex h-16 items-center gap-3 overflow-hidden rounded-md bg-white/[0.07] pr-3 transition hover:bg-white/[0.14]">
      <Art src={stationCover(station.id)} name={station.name} className="h-16 w-16" rounded="none" />
      <span className={clsx('min-w-0 flex-1 truncate font-bold transition-[padding] group-hover:pr-12', active && 'pr-12 text-accent')}>{station.name}</span>
      <PlayButton
        size="md"
        playing={active && player.playing}
        label={`Play ${station.name}`}
        onClick={() => (active ? player.toggle() : void player.playStation(station))}
        className={clsx('absolute right-3 h-10 w-10 transition', active && player.playing ? 'opacity-100' : 'opacity-0 group-hover:opacity-100')}
      />
    </Link>
  );
}

function SuggestionCard({ s }: { s: StationSuggestion }) {
  const save = useSaveStation();
  const dismiss = useDismissSuggestion();
  const navigate = useNavigate();
  return (
    <div className="group relative flex flex-col gap-3 rounded-lg p-3 transition hover:bg-card">
      <Art src={`/api/playlists/${encodeURIComponent(s.playlist_ids[0])}/cover`} name={s.name} className="aspect-square w-full shadow-lift" />
      <IconButton
        label={`Dismiss ${s.name}`}
        size="sm"
        className="absolute right-4 top-4 bg-black/60 text-ink opacity-0 group-hover:opacity-100 focus:opacity-100"
        onClick={() => dismiss.mutate(s)}
      >
        <X className="h-4 w-4" />
      </IconButton>
      <div className="min-w-0">
        <div className="truncate font-semibold">{s.name}</div>
        <div className="line-clamp-2 text-[13px] text-muted">
          Playlist · {plural(s.track_count, 'track')}
        </div>
      </div>
      <Button
        size="sm"
        variant="primary"
        loading={save.isPending}
        onClick={() =>
          save.mutate(
            { body: { name: s.name, playlist_ids: s.playlist_ids, genres: [] } },
            { onSuccess: (st) => navigate(`/station/${st.id}`) },
          )
        }
      >
        Create station
      </Button>
      <ErrorText error={save.error} />
    </div>
  );
}

export function HomePage() {
  const { user } = useAuth();
  const stations = useStations();
  const recent = useBrowse('recent');
  const suggestions = useSuggestions(!!user?.is_admin);
  const [creating, setCreating] = useState(false);

  if (stations.isLoading) return <Spinner />;
  const list = stations.data ?? [];
  const byId = new Map(list.map((s) => [s.id, s]));
  const recentStations = (recent.data?.items ?? []).map((i) => byId.get(i.station_id ?? '')).filter((s): s is Station => !!s);
  const quick = [...recentStations, ...list.filter((s) => !recentStations.includes(s))].slice(0, 8);
  const sugg = suggestions.data?.suggestions ?? [];

  return (
    <div className="-mx-4 -mt-16 bg-gradient-to-b from-[#123a37]/80 via-transparent to-transparent px-4 pt-20 md:-mx-8 md:px-8">
      <h1 className="mb-6 text-3xl font-black tracking-tight">{greeting()}</h1>

      {list.length === 0 ? (
        <Empty
          icon={<Radio className="h-12 w-12" />}
          title="No stations yet"
          action={
            <Button variant="primary" onClick={() => setCreating(true)}>
              Create a station
            </Button>
          }
        >
          {user?.is_admin
            ? 'Add Spotify playlists on the Playlists page. Each main genre gets its own station once enough of their songs are in your library; you can also create stations or accept a suggestion below.'
            : 'Create a station from the synced playlists. Genre stations appear by themselves once enough playlist songs are in the library.'}
        </Empty>
      ) : (
        <div className="mb-10 grid grid-cols-1 gap-2 sm:grid-cols-2 xl:grid-cols-4">
          {quick.map((s) => (
            <QuickTile key={s.id} station={s} />
          ))}
        </div>
      )}

      {recentStations.length > 0 && (
        <Section title="Jump back in">
          <CardGrid>
            {recentStations.slice(0, 6).map((s) => (
              <StationCard key={s.id} station={s} />
            ))}
          </CardGrid>
        </Section>
      )}

      {sugg.length > 0 && (
        <Section
          title="Suggested stations"
          action={
            <span className="flex items-center gap-1 text-[13px] text-muted">
              <Sparkles className="h-4 w-4" /> One per playlist
            </span>
          }
        >
          <CardGrid>
            {sugg.slice(0, 12).map((s) => (
              <SuggestionCard key={s.playlist_ids.join(',')} s={s} />
            ))}
          </CardGrid>
        </Section>
      )}

      {list.length > 0 && (
        <Section
          title="Your stations"
          action={
            <Link to="/stations" className="text-[13px] font-bold text-muted hover:text-ink hover:underline">
              Show all
            </Link>
          }
        >
          <CardGrid>
            {list.slice(0, 12).map((s) => (
              <StationCard key={s.id} station={s} />
            ))}
          </CardGrid>
        </Section>
      )}
      <StationEditor open={creating} onClose={() => setCreating(false)} />
    </div>
  );
}
