import type { PlaybackTrack } from './types';

const DEFAULT_CROSSFADE_SECONDS = 5;
const OUTRO_LEAD_SECONDS = 0.1;

/** Outro end, falling back to the track duration (0 when unknown). */
export function effectiveOutroEnd(track: PlaybackTrack): number {
  const duration = track.duration_seconds ?? 0;
  const end = track.outro_end_seconds ?? 0;
  return end > 0 && (duration <= 0 || end <= duration) ? end : duration;
}

/**
 * When to start fading out: the analysed outro start, or a default crossfade
 * before the outro end when analysis didn't find one (0 = never).
 */
export function effectiveOutroStart(track: PlaybackTrack): number {
  const end = effectiveOutroEnd(track);
  const start = track.outro_start_seconds;
  if (start > 0 && (end <= 0 || start < end)) return start;
  return end > DEFAULT_CROSSFADE_SECONDS ? end - DEFAULT_CROSSFADE_SECONDS : 0;
}

export function shouldBeginCrossfade(currentTime: number, outroStartSeconds: number): boolean {
  if (outroStartSeconds <= 0) return false;
  return currentTime >= outroStartSeconds - OUTRO_LEAD_SECONDS;
}

/** Crossfade length for leaving `current` into `next`, clamped to the outro and the next track. */
export function crossfadeDuration(current: PlaybackTrack, next: PlaybackTrack): number {
  let duration =
    current.ideal_crossfade_seconds > 0 ? current.ideal_crossfade_seconds : DEFAULT_CROSSFADE_SECONDS;
  const outro = effectiveOutroEnd(current) - effectiveOutroStart(current);
  if (outro > 0 && duration > outro) duration = outro;
  const nextDuration = next.duration_seconds ?? 0;
  if (nextDuration > 0 && duration > nextDuration) duration = nextDuration;
  return duration;
}

export { DEFAULT_CROSSFADE_SECONDS, OUTRO_LEAD_SECONDS };
