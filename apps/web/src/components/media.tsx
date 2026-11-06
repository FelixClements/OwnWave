import { useEffect, useState, type ReactNode } from 'react';
import { Link } from 'react-router-dom';
import clsx from 'clsx';
import { Ban, EyeOff, Heart } from 'lucide-react';
import type { Playlist, QueueTrack, Station } from '@/api/types';
import { FALLBACK, imageColor, nameColor, rgb, type Rgb } from '@/lib/color';
import { formatDuration, plural } from '@/lib/format';
import { useGenreNames } from '@/lib/queries';
import { coverUrl, usePlayer } from '@/player/PlayerProvider';
import { Art, Equalizer, IconButton, PlayButton } from './ui';

export const stationCover = (id: string) => `/api/stations/${encodeURIComponent(id)}/cover`;

/** The playlist's Spotify cover, once a sync has downloaded it. */
export const playlistCover = (p: Pick<Playlist, 'id' | 'has_cover'>) =>
  p.has_cover ? `/api/playlists/${encodeURIComponent(p.id)}/cover` : undefined;

/** "Techno, House" or "All genres". */
export function useStationGenreLabel() {
  const name = useGenreNames();
  return (s: Pick<Station, 'genres'>) => (s.genres.length ? s.genres.map(name).join(', ') : 'All genres');
}

/** Square station card with a play button that appears on hover. */
export function StationCard({ station, subtitle }: { station: Station; subtitle?: string }) {
  const player = usePlayer();
  const label = useStationGenreLabel();
  const active = player.station?.id === station.id;
  const playing = active && player.playing;
  return (
    <Link
      to={`/station/${station.id}`}
      className="group relative flex flex-col gap-3 rounded-lg bg-card/0 p-3 transition hover:bg-card focus-visible:bg-card focus-visible:outline-none"
    >
      <div className="relative">
        <Art src={stationCover(station.id)} name={station.name} className="aspect-square w-full shadow-lift" />
        <PlayButton
          size="md"
          playing={playing}
          label={playing ? `Pause ${station.name}` : `Play ${station.name}`}
          onClick={() => (active ? player.toggle() : void player.playStation(station))}
          className={clsx(
            'absolute bottom-2 right-2 transition duration-200',
            playing ? 'translate-y-0 opacity-100' : 'translate-y-2 opacity-0 group-hover:translate-y-0 group-hover:opacity-100 group-focus-visible:opacity-100',
          )}
        />
      </div>
      <div className="min-w-0">
        <div className={clsx('truncate font-semibold', active && 'text-accent')}>{station.name}</div>
        <div className="line-clamp-2 text-[13px] text-muted">{subtitle ?? `${label(station)} · ${plural(station.track_count, 'track')}`}</div>
      </div>
    </Link>
  );
}

/** Responsive grid of cards. */
export function CardGrid({ children }: { children: ReactNode }) {
  return <div className="-mx-3 grid grid-cols-2 gap-1 sm:grid-cols-3 md:grid-cols-4 xl:grid-cols-5 2xl:grid-cols-6">{children}</div>;
}

export function Section({ title, action, children }: { title: string; action?: ReactNode; children: ReactNode }) {
  return (
    <section className="mb-10">
      <div className="mb-3 flex items-end justify-between gap-4">
        <h2 className="text-2xl font-bold tracking-tight">{title}</h2>
        {action}
      </div>
      {children}
    </section>
  );
}

/** Colour sampled from artwork, for hero gradients. */
export function useArtColor(src: string | undefined, name: string): Rgb {
  const [c, setC] = useState<Rgb>(() => nameColor(name));
  useEffect(() => {
    let live = true;
    if (src) void imageColor(src).then((col) => live && setC(col === FALLBACK ? nameColor(name) : col));
    return () => {
      live = false;
    };
  }, [src, name]);
  return c;
}

/** Big page header with artwork and a gradient taken from it. */
export function HeroHeader({ color, art, kind, title, meta }: { color: Rgb; art: ReactNode; kind: string; title: string; meta?: ReactNode }) {
  return (
    <header
      className="-mx-4 -mt-16 px-4 pb-6 pt-24 md:-mx-8 md:px-8"
      style={{ backgroundImage: `linear-gradient(to bottom, ${rgb(color)}, ${rgb(color, 0.35)} 70%, transparent)` }}
    >
      <div className="flex flex-col items-center gap-6 sm:flex-row sm:items-end">
        <div className="w-44 shrink-0 shadow-[0_8px_40px_rgba(0,0,0,.6)] sm:w-56">{art}</div>
        <div className="min-w-0 text-center sm:text-left">
          <div className="text-[13px] font-semibold">{kind}</div>
          <h1 className="mt-1 break-words text-4xl font-black leading-tight tracking-tight sm:text-5xl lg:text-6xl">{title}</h1>
          {meta && <div className="mt-3 text-sm text-ink/80">{meta}</div>}
        </div>
      </div>
    </header>
  );
}

/** Area under a hero: continues its colour faintly. */
export function HeroBody({ color, children }: { color: Rgb; children: ReactNode }) {
  return (
    <div className="-mx-4 px-4 md:-mx-8 md:px-8" style={{ backgroundImage: `linear-gradient(to bottom, ${rgb(color, 0.18)}, transparent 240px)` }}>
      {children}
    </div>
  );
}

/** A station's queue or any list of tracks. */
export function TrackList({
  tracks,
  startIndex = 1,
  showActions = true,
  empty,
  onHide,
}: {
  tracks: readonly QueueTrack[];
  startIndex?: number;
  showActions?: boolean;
  empty?: ReactNode;
  /** Offers "Not in this station" on each row. */
  onHide?: (track: QueueTrack) => void;
}) {
  const player = usePlayer();
  if (!tracks.length) return <>{empty}</>;
  return (
    <div role="table" aria-label="Tracks" className="text-sm">
      <div role="row" className="grid grid-cols-[2rem_1fr_auto] items-center gap-4 border-b border-line px-4 pb-2 text-[12px] uppercase tracking-wider text-muted md:grid-cols-[2rem_minmax(0,4fr)_minmax(0,3fr)_5rem_auto]">
        <span role="columnheader">#</span>
        <span role="columnheader">Title</span>
        <span role="columnheader" className="hidden md:block">Album</span>
        <span role="columnheader" className="hidden text-right md:block">Time</span>
        <span role="columnheader" className="sr-only">Actions</span>
      </div>
      <div className="mt-2">
        {tracks.map((t, i) => {
          const current = player.track?.id === t.id;
          return (
            <div
              role="row"
              key={`${t.id}-${i}`}
              className={clsx(
                'group grid grid-cols-[2rem_1fr_auto] items-center gap-4 rounded-md px-4 py-2 hover:bg-raised/70 md:grid-cols-[2rem_minmax(0,4fr)_minmax(0,3fr)_5rem_auto]',
                current && 'bg-raised/50',
              )}
            >
              <span role="cell" className="tabular-nums text-muted">
                {current ? <Equalizer paused={!player.playing} /> : i + startIndex}
              </span>
              <span role="cell" className="flex min-w-0 items-center gap-3">
                <Art src={coverUrl(t.id)} name={t.album || t.title} className="h-10 w-10" rounded="md" />
                <span className="min-w-0">
                  <span className={clsx('block truncate font-medium', current ? 'text-accent' : 'text-ink')}>{t.title}</span>
                  <span className="block truncate text-[13px] text-muted">{t.artist ?? 'Unknown artist'}</span>
                </span>
              </span>
              <span role="cell" className="hidden truncate text-muted md:block">
                {t.album}
              </span>
              <span role="cell" className="hidden text-right tabular-nums text-muted md:block">
                {formatDuration(t.duration_seconds)}
              </span>
              <span role="cell" className="flex items-center gap-0.5">
                {showActions && (
                  <>
                    <IconButton label={player.liked(t.id) ? `Unlike ${t.title}` : `Like ${t.title}`} active={player.liked(t.id)} size="sm" onClick={() => player.like(t)}>
                      <Heart className="h-4 w-4" fill={player.liked(t.id) ? 'currentColor' : 'none'} />
                    </IconButton>
                    {onHide && (
                      <IconButton
                        label={`Not in this station: ${t.title}`}
                        size="sm"
                        className="opacity-0 focus:opacity-100 group-hover:opacity-100"
                        onClick={() => onHide(t)}
                      >
                        <EyeOff className="h-4 w-4" />
                      </IconButton>
                    )}
                    <IconButton
                      label={`Never play ${t.title}`}
                      size="sm"
                      className="opacity-0 focus:opacity-100 group-hover:opacity-100"
                      onClick={() => player.ban(t)}
                    >
                      <Ban className="h-4 w-4" />
                    </IconButton>
                  </>
                )}
              </span>
            </div>
          );
        })}
      </div>
    </div>
  );
}
