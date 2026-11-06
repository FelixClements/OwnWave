import { useEffect } from 'react';
import { Link } from 'react-router-dom';
import clsx from 'clsx';
import { Ban, EyeOff, ChevronDown, Heart, ListMusic, SkipBack, SkipForward, Volume1, Volume2, VolumeX } from 'lucide-react';
import { formatDuration } from '@/lib/format';
import { QUALITY_LABELS, streamQuality, updateSettings, useSettings } from '@/lib/settings';
import { rgb } from '@/lib/color';
import { coverUrl, usePlaybackTime, usePlayer } from '@/player/PlayerProvider';
import { useArtColor } from './media';
import { Art, IconButton, PauseGlyph, PlayGlyph } from './ui';

/** A range input styled as a thin bar that fills with the accent colour. */
export function Slider({ value, max, onChange, label, className, step = 'any' }: { value: number; max: number; onChange: (v: number) => void; label: string; className?: string; step?: number | 'any' }) {
  const pct = max > 0 ? Math.min(100, (value / max) * 100) : 0;
  return (
    <input
      type="range"
      aria-label={label}
      min={0}
      max={max || 1}
      step={step}
      value={Math.min(value, max || 1)}
      onChange={(e) => onChange(Number(e.target.value))}
      className={clsx('ow-slider', className)}
      style={{ '--pct': `${pct}%` } as React.CSSProperties}
    />
  );
}

function Seek({ compact }: { compact?: boolean }) {
  const { position, duration } = usePlaybackTime();
  const player = usePlayer();
  return (
    <div className={clsx('flex w-full items-center gap-2 text-[11px] tabular-nums text-muted', compact && 'gap-3')}>
      <span className="w-10 text-right">{formatDuration(position)}</span>
      <Slider label="Seek" value={position} max={duration} onChange={player.seek} className="flex-1" />
      <span className="w-10">{formatDuration(duration)}</span>
    </div>
  );
}

function Controls({ big }: { big?: boolean }) {
  const p = usePlayer();
  const disabled = !p.track && !p.station;
  return (
    <div className={clsx('flex items-center justify-center', big ? 'gap-8' : 'gap-4')}>
      <IconButton label="Previous" onClick={p.previous} disabled={!p.track}>
        <SkipBack className={big ? 'h-7 w-7' : 'h-5 w-5'} fill="currentColor" />
      </IconButton>
      <button
        type="button"
        onClick={p.toggle}
        disabled={disabled}
        aria-label={p.playing ? 'Pause' : 'Play'}
        className={clsx(
          'inline-flex items-center justify-center rounded-full bg-ink text-black transition hover:scale-105 disabled:opacity-40',
          big ? 'h-16 w-16' : 'h-9 w-9',
        )}
      >
        {p.playing ? <PauseGlyph className={big ? 'h-7 w-7' : 'h-4 w-4'} /> : <PlayGlyph className={big ? 'h-7 w-7' : 'h-4 w-4'} />}
      </button>
      <IconButton label="Next" onClick={p.next} disabled={!p.track}>
        <SkipForward className={big ? 'h-7 w-7' : 'h-5 w-5'} fill="currentColor" />
      </IconButton>
    </div>
  );
}

function Volume() {
  const s = useSettings();
  const Icon = s.volume === 0 ? VolumeX : s.volume < 0.5 ? Volume1 : Volume2;
  return (
    <div className="flex w-36 items-center gap-1">
      <IconButton label={s.volume === 0 ? 'Unmute' : 'Mute'} size="sm" onClick={() => updateSettings({ volume: s.volume === 0 ? 0.8 : 0 })}>
        <Icon className="h-4 w-4" />
      </IconButton>
      <Slider label="Volume" value={s.volume} max={1} onChange={(v) => updateSettings({ volume: v })} className="flex-1" />
    </div>
  );
}

function LikeButton({ size = 'md' }: { size?: 'sm' | 'md' }) {
  const p = usePlayer();
  if (!p.track) return null;
  const liked = p.liked(p.track.id);
  return (
    <IconButton label={liked ? 'Unlike' : 'Like'} active={liked} size={size === 'sm' ? 'sm' : 'md'} onClick={() => p.track && p.like(p.track)}>
      <Heart className="h-[18px] w-[18px]" fill={liked ? 'currentColor' : 'none'} />
    </IconButton>
  );
}

/** Bottom bar: desktop player, or a mini player on phones. */
export function PlayerBar() {
  const p = usePlayer();
  const s = useSettings();
  const t = p.track;
  return (
    <>
      {/* desktop */}
      <div className="hidden h-[76px] grid-cols-[minmax(0,1fr)_minmax(0,1.4fr)_minmax(0,1fr)] items-center gap-4 px-2 md:grid" aria-label="Player">
        <div className="flex min-w-0 items-center gap-3">
          {t ? (
            <>
              <button type="button" onClick={() => p.setExpanded(true)} aria-label="Open now playing" className="shrink-0">
                <Art src={coverUrl(t.id)} name={t.album || t.title} className="h-14 w-14" />
              </button>
              <div className="min-w-0">
                <div className="truncate text-sm font-semibold">{t.title}</div>
                <div className="truncate text-[12px] text-muted">{t.artist ?? 'Unknown artist'}</div>
              </div>
              <LikeButton />
            </>
          ) : (
            <div className="text-sm text-muted">{p.loading ? 'Loading station…' : (p.error ?? 'Pick a station to start listening')}</div>
          )}
        </div>
        <div className="flex flex-col items-center gap-1">
          <Controls />
          <Seek />
        </div>
        <div className="flex items-center justify-end gap-2">
          {p.station && (
            <Link to={`/station/${p.station.id}`} className="hidden max-w-[10rem] truncate text-[12px] text-muted hover:text-ink lg:block">
              {p.station.name}
            </Link>
          )}
          <span className="hidden rounded border border-line px-1.5 py-0.5 text-[10px] font-semibold uppercase text-muted xl:inline" title={QUALITY_LABELS[streamQuality(s)]}>
            {streamQuality(s)}
          </span>
          <IconButton label="Now playing and queue" size="sm" onClick={() => p.setExpanded(true)} disabled={!t}>
            <ListMusic className="h-4 w-4" />
          </IconButton>
          <Volume />
        </div>
      </div>

      {/* phone mini player */}
      {t && (
        <div className="relative mx-1 flex items-center gap-3 overflow-hidden rounded-md bg-raised p-2 md:hidden" aria-label="Player">
          <button type="button" className="flex min-w-0 flex-1 items-center gap-3 text-left" onClick={() => p.setExpanded(true)} aria-label="Open now playing">
            <Art src={coverUrl(t.id)} name={t.album || t.title} className="h-10 w-10" />
            <span className="min-w-0">
              <span className="block truncate text-sm font-semibold">{t.title}</span>
              <span className="block truncate text-[12px] text-muted">{t.artist}</span>
            </span>
          </button>
          <LikeButton size="sm" />
          <IconButton label={p.playing ? 'Pause' : 'Play'} onClick={p.toggle} className="text-ink">
            {p.playing ? <PauseGlyph /> : <PlayGlyph />}
          </IconButton>
          <MiniProgress />
        </div>
      )}
    </>
  );
}

function MiniProgress() {
  const { position, duration } = usePlaybackTime();
  return (
    <div className="absolute inset-x-2 bottom-0 h-[2px] bg-white/20">
      <div className="h-full bg-ink" style={{ width: `${duration ? (position / duration) * 100 : 0}%` }} />
    </div>
  );
}

/** Full-screen now playing with the up-next queue. */
export function NowPlaying() {
  const p = usePlayer();
  const t = p.track;
  const color = useArtColor(t ? coverUrl(t.id) : undefined, t?.album || t?.title || 'OwnWave');

  useEffect(() => {
    if (!p.expanded) return;
    const onKey = (e: KeyboardEvent) => e.key === 'Escape' && p.setExpanded(false);
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [p]);

  if (!p.expanded || !t) return null;
  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-label="Now playing"
      className="fixed inset-0 z-50 flex flex-col overflow-y-auto bg-base"
      style={{ backgroundImage: `linear-gradient(to bottom, ${rgb(color)}, ${rgb(color, 0.4)} 45%, #0d0d0f 90%)` }}
    >
      <div className="flex items-center justify-between px-4 py-3 md:px-8">
        <IconButton label="Close now playing" onClick={() => p.setExpanded(false)} className="text-ink">
          <ChevronDown className="h-6 w-6" />
        </IconButton>
        <div className="min-w-0 text-center">
          <div className="text-[11px] uppercase tracking-widest text-ink/70">Playing from station</div>
          <div className="truncate text-sm font-bold">{p.station?.name}</div>
        </div>
        <span className="w-9" />
      </div>
      <div className="mx-auto grid w-full max-w-6xl flex-1 gap-10 px-6 pb-10 md:grid-cols-[minmax(0,1fr)_minmax(0,1fr)] md:items-center md:px-10">
        <div className="mx-auto w-full max-w-[min(70vh,520px)]">
          <Art src={coverUrl(t.id)} name={t.album || t.title} className="aspect-square w-full rounded-lg shadow-[0_16px_60px_rgba(0,0,0,.6)]" />
          <div className="mt-6 flex items-center gap-3">
            <div className="min-w-0 flex-1">
              <h2 className="truncate text-2xl font-bold">{t.title}</h2>
              <p className="truncate text-ink/70">{[t.artist, t.album].filter(Boolean).join(' · ')}</p>
            </div>
            <IconButton label={`Not in this station: ${t.title}`} onClick={() => p.hide(t)}>
              <EyeOff className="h-5 w-5" />
            </IconButton>
            <IconButton label={`Never play ${t.title}`} onClick={() => p.ban(t)}>
              <Ban className="h-5 w-5" />
            </IconButton>
            <LikeButton />
          </div>
          <div className="mt-4">
            <Seek compact />
          </div>
          <div className="mt-4">
            <Controls big />
          </div>
        </div>
        <div className="min-w-0 self-stretch rounded-xl bg-black/25 p-4 md:max-h-[75vh] md:overflow-y-auto">
          <h3 className="mb-3 font-bold">Up next</h3>
          {p.upcoming.length === 0 && <p className="text-sm text-muted">More tracks load as you listen.</p>}
          <ol className="space-y-1">
            {p.upcoming.slice(0, 30).map((u, i) => (
              <li key={`${u.id}-${i}`} className="flex items-center gap-3 rounded-md p-2 hover:bg-white/5">
                <Art src={coverUrl(u.id)} name={u.album || u.title} className="h-10 w-10" />
                <div className="min-w-0 flex-1">
                  <div className="truncate text-sm font-medium">{u.title}</div>
                  <div className="truncate text-[12px] text-muted">{u.artist}</div>
                </div>
                <span className="text-[12px] tabular-nums text-muted">{formatDuration(u.duration_seconds)}</span>
              </li>
            ))}
          </ol>
        </div>
      </div>
    </div>
  );
}
