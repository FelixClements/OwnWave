/**
 * Station seed type literals — keep in sync with schemas/station-seed.schema.json
 */
export const STATION_SEED_TYPES = [
  'track',
  'artist',
  'album',
  'cluster',
  'mood',
  'genre',
  'sub_genre',
  'uncategorized',
] as const;

export type StationSeedType = (typeof STATION_SEED_TYPES)[number];
