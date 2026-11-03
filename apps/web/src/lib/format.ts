/** 3:07, or 1:02:03 past an hour. */
export function formatDuration(seconds?: number | null): string {
  if (seconds == null || !isFinite(seconds) || seconds < 0) return '–:––';
  const s = Math.floor(seconds);
  const h = Math.floor(s / 3600);
  const m = Math.floor((s % 3600) / 60);
  const sec = String(s % 60).padStart(2, '0');
  return h ? `${h}:${String(m).padStart(2, '0')}:${sec}` : `${m}:${sec}`;
}

/** "just now", "5 min ago", "3 h ago", "2 days ago", or a date. */
export function timeAgo(iso: string, now = Date.now()): string {
  const secs = Math.round((now - Date.parse(iso)) / 1000);
  if (secs < 60) return 'just now';
  if (secs < 3600) return `${Math.floor(secs / 60)} min ago`;
  if (secs < 86400) return `${Math.floor(secs / 3600)} h ago`;
  if (secs < 7 * 86400) {
    const d = Math.floor(secs / 86400);
    return d === 1 ? 'yesterday' : `${d} days ago`;
  }
  return new Date(iso).toLocaleDateString();
}

export const plural = (n: number, word: string) => `${n.toLocaleString()} ${word}${n === 1 ? '' : 's'}`;

/** "Good morning" / "Good afternoon" / "Good evening". */
export function greeting(date = new Date()): string {
  const h = date.getHours();
  return h < 5 || h >= 18 ? 'Good evening' : h < 12 ? 'Good morning' : 'Good afternoon';
}
