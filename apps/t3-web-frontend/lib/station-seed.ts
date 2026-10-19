import type { StationRequest } from './api';
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
  // Seed ids have no form inputs; they are carried over from the stored seed.
  track_id: string;
  artist_id: string;
  album_id: string;
  cluster_id: string;
};

/** Seed types whose target id must come from an existing seed (no picker in the form). */
export const ID_SEED_TYPES: StationSeedType[] = ['track', 'artist', 'album', 'cluster'];

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
  track_id: '',
  artist_id: '',
  album_id: '',
  cluster_id: '',
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
    track_id: seed.track_id ?? '',
    artist_id: seed.artist_id ?? '',
    album_id: seed.album_id ?? '',
    cluster_id: seed.cluster_id != null ? String(seed.cluster_id) : '',
  };
}

function toNum(value: string) {
  const n = parseFloat(value);
  return isNaN(n) ? undefined : n;
}

/** Build the create/update request body from the filter form. */
export function filterFormToRequest(name: string, form: StationFilterForm): StationRequest {
  return {
    name,
    seed_type: (form.seed_type || undefined) as StationSeedType | undefined,
    min_bpm: toNum(form.min_bpm),
    max_bpm: toNum(form.max_bpm),
    min_energy: toNum(form.min_energy),
    max_energy: toNum(form.max_energy),
    min_valence: toNum(form.min_valence),
    max_valence: toNum(form.max_valence),
    main_genre: form.main_genre || undefined,
    sub_genre: form.sub_genre || undefined,
    track_id: form.track_id || undefined,
    artist_id: form.artist_id || undefined,
    album_id: form.album_id || undefined,
    cluster_id: form.cluster_id ? Number(form.cluster_id) : undefined,
  };
}

export const SEED_TYPE_LABELS: Record<StationSeedType, string> = {
  track: 'Track',
  artist: 'Artist',
  album: 'Album',
  cluster: 'Cluster',
  mood: 'Mood',
  genre: 'Genre',
  sub_genre: 'Sub-genre',
  uncategorized: 'Uncategorized',
};

function range(label: string, min?: number, max?: number, digits = 0) {
  if (min == null && max == null) return null;
  const f = (n: number) => n.toFixed(digits);
  if (min != null && max != null) return `${label} ${f(min)}–${f(max)}`;
  return min != null ? `${label} ≥ ${f(min)}` : `${label} ≤ ${f(max!)}`;
}

/** One-line summary of a seed: "Electronic › Deep House · BPM 118–126 · energy 0.50–0.90". */
export function describeStationSeed(seed: StationSeed | null): string {
  if (!seed) return '';
  const parts = [
    seed.main_genre && (seed.sub_genre ? `${seed.main_genre} › ${seed.sub_genre}` : seed.main_genre),
    range('BPM', seed.min_bpm, seed.max_bpm),
    range('energy', seed.min_energy, seed.max_energy, 2),
    range('valence', seed.min_valence, seed.max_valence, 2),
  ];
  return parts.filter(Boolean).join(' · ');
}
