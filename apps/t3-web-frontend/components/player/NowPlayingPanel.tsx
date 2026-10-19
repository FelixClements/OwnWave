'use client';

import { useQueue } from '@/lib/queries';
import { useStation } from '@/lib/station';
import { formatChannels, formatDuration, formatSampleRate } from '@/lib/format';
import { Cover } from '@/components/tracks/Cover';
import { KeyBadge, Meter, formatBpm } from '@/components/tracks/AudioStats';
import { Badge } from '@/components/ui/Layout';

const UP_NEXT = 6;

function Stat({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="rounded-lg border border-line bg-surface-2 px-2.5 py-2">
      <dt className="text-[10.5px] uppercase tracking-wider text-muted">{label}</dt>
      <dd className="mt-0.5 text-base font-semibold tabular-nums">{children}</dd>
    </div>
  );
}

/** Wide-screen inspector: details and audio features of the playing track, and what follows. */
export function NowPlayingPanel() {
  const { nowPlaying, playingStation } = useStation();
  const { data: queue = [] } = useQueue(playingStation);

  if (!nowPlaying) {
    return (
      <aside aria-label="Now playing" className="hidden xl:flex w-[292px] shrink-0 border-l border-line bg-surface p-4 text-muted">
        Pick a station and press play.
      </aside>
    );
  }

  const index = queue.findIndex((t) => t.id === nowPlaying.id);
  const upNext = index >= 0 ? queue.slice(index + 1, index + 1 + UP_NEXT) : [];

  return (
    <aside
      aria-label="Now playing"
      className="hidden xl:flex w-[292px] shrink-0 flex-col gap-3.5 overflow-y-auto border-l border-line bg-surface p-4"
    >
      <div className="flex items-center justify-between">
        <h2 className="text-[10.5px] uppercase tracking-widest text-muted">Now playing</h2>
        {nowPlaying.ideal_crossfade_seconds > 0 && (
          <Badge tone="accent">Crossfade {Math.round(nowPlaying.ideal_crossfade_seconds)}s</Badge>
        )}
      </div>
      <Cover id={nowPlaying.id} title={nowPlaying.title} className="w-full aspect-square rounded-lg text-5xl" />
      <div className="min-w-0">
        <div className="text-[15px] font-semibold truncate">{nowPlaying.title}</div>
        <div className="text-muted truncate">
          {nowPlaying.artist || 'Unknown artist'}
          {nowPlaying.album && ` · ${nowPlaying.album}`}
        </div>
      </div>
      <div className="flex flex-wrap gap-1.5">
        {nowPlaying.sample_rate ? <Badge tone="accent">{formatSampleRate(nowPlaying.sample_rate)}</Badge> : null}
        {nowPlaying.channels ? <Badge>{formatChannels(nowPlaying.channels)}</Badge> : null}
        <Badge>{formatDuration(nowPlaying.duration_seconds)}</Badge>
      </div>
      <dl className="grid grid-cols-2 gap-2">
        <Stat label="BPM">{formatBpm(nowPlaying.bpm)}</Stat>
        <Stat label="Key">
          <KeyBadge musicalKey={nowPlaying.key} />
        </Stat>
        <Stat label="Energy">
          <Meter value={nowPlaying.energy} label="Energy" />
        </Stat>
        <Stat label="Valence">
          <Meter value={nowPlaying.valence} label="Valence" tone="valence" />
        </Stat>
      </dl>
      {upNext.length > 0 && (
        <section aria-label="Up next">
          <h2 className="text-[10.5px] uppercase tracking-widest text-muted mb-1">Up next</h2>
          <ol>
            {upNext.map((t, i) => (
              <li key={t.id} className="grid grid-cols-[16px_1fr_auto] items-center gap-2 py-1.5 border-b border-line/60">
                <span className="text-muted tabular-nums text-right">{i + 1}</span>
                <div className="min-w-0 leading-tight">
                  <div className="truncate">{t.title}</div>
                  <div className="truncate text-[11.5px] text-muted">{t.artist || 'Unknown artist'}</div>
                </div>
                <span className="text-muted tabular-nums text-[11.5px]">{formatBpm(t.bpm)}</span>
              </li>
            ))}
          </ol>
        </section>
      )}
    </aside>
  );
}
