import type { StationSeed } from './station-seed.generated';
import { STATION_SEED_TYPES, type StationSeedType } from './station-seed-types';

export type { StationSeed, StationSeedType };
export { STATION_SEED_TYPES };

export type StationFilterForm = {
  min_bpm: string;
  max_bpm: string;
  min_energy: string;
  max_energy: string;
  min_valence: string;
  max_valence: string;
  seed_type: string;
  main_genre: string;
  sub_genre: string;
};

export const emptyStationFilterForm = (): StationFilterForm => ({
  min_bpm: '',
  max_bpm: '',
  min_energy: '',
  max_energy: '',
  min_valence: '',
  max_valence: '',
  seed_type: '',
  main_genre: '',
  sub_genre: '',
});

export function parseStationSeed(
  raw: string | StationSeed | null | undefined,
): StationSeed | null {
  if (!raw) return null;
  if (typeof raw === 'string') {
    try {
      return JSON.parse(raw) as StationSeed;
    } catch {
      return null;
    }
  }
  return raw;
}

export function seedToFilterForm(seed: StationSeed | null): StationFilterForm {
  if (!seed) return emptyStationFilterForm();
  const seedType = seed.type ?? seed.seed_type ?? '';
  return {
    min_bpm: seed.min_bpm != null ? String(seed.min_bpm) : '',
    max_bpm: seed.max_bpm != null ? String(seed.max_bpm) : '',
    min_energy: seed.min_energy != null ? String(seed.min_energy) : '',
    max_energy: seed.max_energy != null ? String(seed.max_energy) : '',
    min_valence: seed.min_valence != null ? String(seed.min_valence) : '',
    max_valence: seed.max_valence != null ? String(seed.max_valence) : '',
    seed_type: seedType,
    main_genre: seed.main_genre ?? '',
    sub_genre: seed.sub_genre ?? '',
  };
}
