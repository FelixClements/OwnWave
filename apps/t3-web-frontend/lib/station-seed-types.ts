/**
 * Station seed type literals — keep in sync with schemas/station-seed.schema.json
 */
export const STATION_SEED_TYPES = [
  'track',
  'artist',
  'album',
  'cluster',
  'mood',
  'playlists',
] as const;

export type StationSeedType = (typeof STATION_SEED_TYPES)[number];
