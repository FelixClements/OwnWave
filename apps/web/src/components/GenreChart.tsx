import clsx from 'clsx';
import type { GenreDistribution } from '@/api/types';
import { plural } from '@/lib/format';

/** Bar colour: the accent stepped into the dark-mode lightness band (validated on the card surface). */
const BAR = '#11a897';

/**
 * Tracks per station genre as a ranked bar list (one series, no legend).
 * Hover or focus a row for primary/secondary counts.
 */
export function GenreChart({ data }: { data: GenreDistribution }) {
  const rows = data.genres.filter((g) => g.tracks > 0).sort((a, b) => b.primary_tracks - a.primary_tracks || b.tracks - a.tracks);
  const max = Math.max(1, ...rows.map((g) => g.primary_tracks));
  if (!rows.length) return <p className="text-sm text-muted">No genres scored yet. They appear after the first scan.</p>;
  return (
    <figure>
      <figcaption className="mb-3 text-[13px] text-muted">Primary genre of {plural(data.scored_tracks, 'scored track')}</figcaption>
      <ul className="space-y-1">
        {rows.map((g) => {
          const share = data.scored_tracks ? Math.round((g.primary_tracks / data.scored_tracks) * 100) : 0;
          return (
            <li
              key={g.genre}
              tabIndex={0}
              className="group relative grid grid-cols-[9rem_minmax(0,1fr)_4.5rem] items-center gap-3 rounded px-1 py-1 text-[13px] outline-none hover:bg-raised focus-visible:bg-raised"
            >
              <span className={clsx('truncate', g.station ? 'text-ink' : 'text-faint')}>{g.name}</span>
              <span className="h-3">
                <span className="block h-full rounded-r-[4px]" style={{ width: `${(g.primary_tracks / max) * 100}%`, backgroundColor: BAR }} />
              </span>
              <span className="text-right tabular-nums text-muted">{g.primary_tracks ? `${share}%` : 'secondary'}</span>
              <span
                role="tooltip"
                className="pointer-events-none absolute left-40 top-full z-10 mt-1 hidden whitespace-nowrap rounded-md bg-ink px-3 py-2 text-[12px] text-black shadow-lift group-hover:block group-focus-visible:block"
              >
                <strong>{g.name}</strong>: {plural(g.primary_tracks, 'track')} primary, {g.tracks} incl. secondary
                {!g.station && ' · not offered as a station'}
              </span>
            </li>
          );
        })}
      </ul>
    </figure>
  );
}
