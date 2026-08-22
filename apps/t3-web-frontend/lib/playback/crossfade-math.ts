const DEFAULT_CROSSFADE_SECONDS = 5;
const OUTRO_LEAD_SECONDS = 0.1;

export function shouldBeginCrossfade(
  currentTime: number,
  outroStartSeconds: number,
): boolean {
  if (outroStartSeconds <= 0) return false;
  return currentTime >= outroStartSeconds - OUTRO_LEAD_SECONDS;
}

/** Clamp crossfade duration to available outro (mirrors server streaming.go defaults). */
export function clampCrossfadeDuration(
  idealSeconds: number,
  outroStartSeconds: number,
  outroEndSeconds: number,
  nextTrackDurationSeconds?: number,
): number {
  let duration = idealSeconds > 0 ? idealSeconds : DEFAULT_CROSSFADE_SECONDS;
  const outroLength =
    outroEndSeconds > outroStartSeconds
      ? outroEndSeconds - outroStartSeconds
      : 0;
  if (outroLength > 0 && duration > outroLength) {
    duration = outroLength;
  }
  if (
    nextTrackDurationSeconds !== undefined &&
    nextTrackDurationSeconds > 0 &&
    duration > nextTrackDurationSeconds
  ) {
    duration = nextTrackDurationSeconds;
  }
  return duration;
}

export { DEFAULT_CROSSFADE_SECONDS, OUTRO_LEAD_SECONDS };
