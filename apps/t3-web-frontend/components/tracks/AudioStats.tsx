import { formatUnit } from '@/lib/format';

/** 0..1 audio feature as a bar; exposed to assistive tech as a meter. */
export function Meter({ value, label, tone = 'energy' }: { value?: number; label: string; tone?: 'energy' | 'valence' }) {
  if (value === undefined || value === null) return <span className="text-muted">—</span>;
  const pct = Math.max(0, Math.min(1, value)) * 100;
  return (
    <div
      role="meter"
      aria-label={label}
      aria-valuemin={0}
      aria-valuemax={1}
      aria-valuenow={value}
      aria-valuetext={formatUnit(value)}
      className="flex items-center gap-1.5 w-full"
    >
      <div className="flex-1 h-1 rounded-sm bg-hover overflow-hidden">
        <div className={`h-full rounded-sm ${tone === 'valence' ? 'bg-valence' : 'bg-accent'}`} style={{ width: `${pct}%` }} />
      </div>
      <span className="w-7 text-right text-[11.5px] text-muted tabular-nums" aria-hidden="true">
        {formatUnit(value)}
      </span>
    </div>
  );
}

export function KeyBadge({ musicalKey }: { musicalKey?: string }) {
  if (!musicalKey) return <span className="text-muted">—</span>;
  return (
    <span className="font-mono text-[11.5px] px-1.5 py-px rounded border border-line bg-surface-2">{musicalKey}</span>
  );
}

export function formatBpm(bpm?: number) {
  return bpm && bpm > 0 ? bpm.toFixed(0) : '—';
}
