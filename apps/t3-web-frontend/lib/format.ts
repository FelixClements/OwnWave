/** "4:05"; "--:--" when the duration is unknown. */
export function formatDuration(seconds?: number): string {
  if (!seconds || seconds <= 0) return '--:--';
  const m = Math.floor(seconds / 60);
  const s = Math.floor(seconds % 60);
  return `${m}:${s.toString().padStart(2, '0')}`;
}

/** 44100 → "44.1 kHz", 96000 → "96 kHz". */
export function formatSampleRate(hz?: number): string {
  if (!hz) return '—';
  return `${Number((hz / 1000).toFixed(1))} kHz`;
}

export function formatChannels(channels?: number): string {
  if (!channels) return '—';
  if (channels === 1) return 'Mono';
  if (channels === 2) return 'Stereo';
  return `${channels} ch`;
}

/** 0..1 feature value as ".78"; "—" when missing. */
export function formatUnit(value?: number): string {
  if (value === undefined || value === null || Number.isNaN(value)) return '—';
  return value.toFixed(2).replace(/^0/, '');
}

export function formatDateTime(iso?: string | null): string {
  if (!iso) return '—';
  const d = new Date(iso);
  return Number.isNaN(d.getTime()) ? '—' : d.toLocaleString();
}

/** "deep house" → "Deep House". */
export function titleCase(s: string): string {
  return s.replace(/\w\S*/g, (w) => w.charAt(0).toUpperCase() + w.slice(1));
}
