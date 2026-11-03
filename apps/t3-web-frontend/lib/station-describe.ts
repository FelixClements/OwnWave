import type { Station } from './api';

/**
 * One-line summary of what a station plays: "Techno Bunker, Hard Techno · Techno, House".
 * Unknown playlist/genre ids are skipped; no genres reads "all genres".
 */
export function describeStation(
  station: Pick<Station, 'playlist_ids' | 'genres'>,
  playlistNames: Record<string, string> = {},
  genreNames: Record<string, string> = {},
): string {
  const lists = station.playlist_ids.map((id) => playlistNames[id]).filter(Boolean).join(', ');
  const genres = station.genres.length
    ? station.genres.map((g) => genreNames[g] ?? g).join(', ')
    : 'all genres';
  return [lists, genres].filter(Boolean).join(' · ');
}
