/** Keep the audible queue pinned while browsing another station. */
export function pinQueue<T>(
  incoming: T[],
  selectedStation: string | null,
  playingStation: string | null,
  pinned: T[],
): T[] {
  if (playingStation === null || playingStation === selectedStation) {
    return incoming;
  }
  // Keep the audible queue while the browsed station's fetch is still loading.
  if (pinned.length > 0) {
    return pinned;
  }
  return incoming;
}
