'use client';

import { useMemo, useState } from 'react';
import { EmptyRow, Table, Td, Th, TitleCell } from '@/components/ui/Table';
import { Equalizer, HeartIcon } from '@/components/ui/Icons';
import { formatChannels, formatDateTime, formatDuration, formatSampleRate } from '@/lib/format';
import { KeyBadge, Meter, formatBpm } from './AudioStats';

/** Any track-like row; each table shows the fields its columns need. */
export type TrackRow = {
  id: string;
  title: string;
  artist?: string;
  album?: string;
  track_number?: number;
  duration_seconds?: number;
  sample_rate?: number;
  channels?: number;
  bpm?: number;
  key?: string;
  energy?: number;
  valence?: number;
  liked?: boolean;
  played_at?: string;
  station?: string;
  distance?: number;
};

export type TrackColumn =
  | 'index'
  | 'title'
  | 'album'
  | 'track_number'
  | 'bpm'
  | 'key'
  | 'energy'
  | 'valence'
  | 'duration'
  | 'sample_rate'
  | 'channels'
  | 'station'
  | 'played_at'
  | 'distance'
  | 'like'
  | 'actions';

type ColumnDef = {
  header: string;
  width?: string;
  align?: 'left' | 'right';
  /** Responsive visibility, e.g. 'hidden md:table-cell' to drop the column on phones. */
  hide?: string;
  sortValue?: (row: TrackRow) => string | number | undefined;
  render: (row: TrackRow, index: number, ctx: RenderCtx) => React.ReactNode;
};

type RenderCtx = {
  activeId?: string | null;
  paused?: boolean;
  onToggleLike?: (row: TrackRow) => void;
  actions?: (row: TrackRow) => React.ReactNode;
};

const COLUMNS: Record<TrackColumn, ColumnDef> = {
  index: {
    header: '#',
    width: '36px',
    align: 'right',
    render: (row, i, { activeId, paused }) =>
      row.id === activeId ? <Equalizer paused={paused} /> : <span className="text-muted">{i + 1}</span>,
  },
  title: {
    header: 'Title',
    sortValue: (r) => r.title.toLowerCase(),
    render: (row, _i, { activeId }) => (
      <TitleCell title={row.title} subtitle={row.artist || 'Unknown artist'} active={row.id === activeId} />
    ),
  },
  album: {
    header: 'Album',
    width: '24%',
    hide: 'hidden lg:table-cell',
    sortValue: (r) => r.album?.toLowerCase(),
    render: (row) => <span className="text-muted">{row.album || '—'}</span>,
  },
  track_number: {
    header: 'No.',
    width: '48px',
    hide: 'hidden md:table-cell',
    align: 'right',
    sortValue: (r) => r.track_number,
    render: (row) => <span className="text-muted">{row.track_number || '—'}</span>,
  },
  bpm: { header: 'BPM', width: '56px', align: 'right', sortValue: (r) => r.bpm, render: (row) => formatBpm(row.bpm) },
  key: { header: 'Key', width: '60px', hide: 'hidden sm:table-cell', sortValue: (r) => r.key, render: (row) => <KeyBadge musicalKey={row.key} /> },
  energy: {
    header: 'Energy',
    width: '104px',
    hide: 'hidden md:table-cell',
    sortValue: (r) => r.energy,
    render: (row) => <Meter value={row.energy} label={`Energy of ${row.title}`} />,
  },
  valence: {
    header: 'Valence',
    width: '104px',
    hide: 'hidden md:table-cell',
    sortValue: (r) => r.valence,
    render: (row) => <Meter value={row.valence} label={`Valence of ${row.title}`} tone="valence" />,
  },
  duration: {
    header: 'Time',
    width: '60px',
    align: 'right',
    sortValue: (r) => r.duration_seconds,
    render: (row) => <span className="text-muted">{formatDuration(row.duration_seconds)}</span>,
  },
  sample_rate: {
    header: 'Rate',
    width: '84px',
    hide: 'hidden sm:table-cell',
    align: 'right',
    sortValue: (r) => r.sample_rate,
    render: (row) => (
      <span className={row.sample_rate && row.sample_rate > 48000 ? 'text-accent' : ''}>{formatSampleRate(row.sample_rate)}</span>
    ),
  },
  channels: {
    header: 'Ch',
    width: '64px',
    hide: 'hidden md:table-cell',
    sortValue: (r) => r.channels,
    render: (row) => <span className="text-muted">{formatChannels(row.channels)}</span>,
  },
  station: {
    header: 'Station',
    width: '20%',
    hide: 'hidden md:table-cell',
    sortValue: (r) => r.station?.toLowerCase(),
    render: (row) => <span className="text-muted">{row.station || '—'}</span>,
  },
  played_at: {
    header: 'Played',
    width: '168px',
    hide: 'hidden sm:table-cell',
    align: 'right',
    sortValue: (r) => r.played_at,
    render: (row) => <span className="text-muted">{formatDateTime(row.played_at)}</span>,
  },
  distance: {
    header: 'Distance',
    width: '84px',
    align: 'right',
    sortValue: (r) => r.distance,
    render: (row) => (row.distance === undefined ? '—' : row.distance.toFixed(4)),
  },
  like: {
    header: 'Like',
    width: '40px',
    render: (row, _i, { onToggleLike }) =>
      onToggleLike ? (
        <button
          type="button"
          onClick={() => onToggleLike(row)}
          aria-pressed={!!row.liked}
          aria-label={`${row.liked ? 'Unlike' : 'Like'} ${row.title}`}
          className={`grid place-items-center w-7 h-7 rounded hover:bg-hover ${row.liked ? 'text-accent' : 'text-muted hover:text-fg'}`}
        >
          <HeartIcon filled={row.liked} className="w-3.5 h-3.5" />
        </button>
      ) : null,
  },
  actions: {
    header: 'Actions',
    width: '96px',
    align: 'right',
    render: (row, _i, { actions }) => actions?.(row),
  },
};

const HIDDEN_HEADERS: TrackColumn[] = ['like', 'actions'];

type SortState = { column: TrackColumn; dir: 'asc' | 'desc' } | null;

/**
 * Dense track table. Sorting is local and off by default: queues must keep
 * play order, and paged lists would only sort what is loaded.
 */
export function TrackTable({
  label,
  rows,
  columns,
  sortable = false,
  activeId,
  paused,
  onToggleLike,
  actions,
  empty = 'No tracks.',
}: {
  label: string;
  rows: TrackRow[];
  columns: TrackColumn[];
  sortable?: boolean;
  activeId?: string | null;
  paused?: boolean;
  onToggleLike?: (row: TrackRow) => void;
  actions?: (row: TrackRow) => React.ReactNode;
  empty?: React.ReactNode;
}) {
  const [sort, setSort] = useState<SortState>(null);

  const sorted = useMemo(() => {
    if (!sort) return rows;
    const value = COLUMNS[sort.column].sortValue!;
    const factor = sort.dir === 'asc' ? 1 : -1;
    return [...rows].sort((a, b) => {
      const va = value(a);
      const vb = value(b);
      if (va === vb) return 0;
      if (va === undefined) return 1;
      if (vb === undefined) return -1;
      return (va < vb ? -1 : 1) * factor;
    });
  }, [rows, sort]);

  const toggleSort = (column: TrackColumn) =>
    setSort((s) => (s?.column !== column ? { column, dir: 'asc' } : s.dir === 'asc' ? { column, dir: 'desc' } : null));

  const ctx: RenderCtx = { activeId, paused, onToggleLike, actions };

  return (
    <Table label={label}>
      <thead>
        <tr>
          {columns.map((c) => {
            const def = COLUMNS[c];
            const hidden = HIDDEN_HEADERS.includes(c);
            const canSort = sortable && !!def.sortValue;
            const dir = sort?.column === c ? sort.dir : null;
            return (
              <Th
                key={c}
                align={def.align}
                // Widths live on the header cells (not <col>) so hidden columns drop out cleanly.
                style={{ width: def.width }}
                className={def.hide}
                aria-sort={canSort ? (dir === 'asc' ? 'ascending' : dir === 'desc' ? 'descending' : 'none') : undefined}
              >
                {hidden ? (
                  <span className="sr-only">{def.header}</span>
                ) : canSort ? (
                  <button
                    type="button"
                    onClick={() => toggleSort(c)}
                    className={`inline-flex items-center gap-1 uppercase tracking-wider hover:text-fg ${dir ? 'text-fg' : ''}`}
                  >
                    {def.header}
                    <span aria-hidden="true">{dir === 'asc' ? '▲' : dir === 'desc' ? '▼' : ''}</span>
                  </button>
                ) : (
                  def.header
                )}
              </Th>
            );
          })}
        </tr>
      </thead>
      <tbody>
        {sorted.map((row, i) => {
          const active = row.id === activeId;
          return (
            <tr
              key={`${row.id}-${row.played_at ?? i}`}
              aria-current={active ? 'true' : undefined}
              className={active ? 'bg-accent/[0.08]' : 'hover:bg-hover/50'}
            >
              {columns.map((c) => (
                <Td key={c} align={COLUMNS[c].align} className={COLUMNS[c].hide}>
                  {COLUMNS[c].render(row, i, ctx)}
                </Td>
              ))}
            </tr>
          );
        })}
        {sorted.length === 0 && <EmptyRow colSpan={columns.length}>{empty}</EmptyRow>}
      </tbody>
    </Table>
  );
}
