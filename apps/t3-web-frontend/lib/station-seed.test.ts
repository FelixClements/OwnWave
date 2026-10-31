import { describe, expect, it } from 'vitest';
import {
  describeStationSeed,
  emptyStationFilterForm,
  filterFormToRequest,
  parseStationSeed,
  seedToFilterForm,
} from './station-seed';

describe('parseStationSeed', () => {
  it('parses JSON string seeds', () => {
    expect(parseStationSeed('{"type":"playlists","playlist_ids":["a"]}')).toEqual({
      type: 'playlists',
      playlist_ids: ['a'],
    });
  });

  it('normalizes legacy seed_type in form state', () => {
    expect(seedToFilterForm(parseStationSeed('{"seed_type":"mood","min_energy":0.5}'))).toMatchObject({
      seed_type: 'mood',
      min_energy: '0.5',
    });
  });
});

describe('filterFormToRequest', () => {
  it('sends playlist ids only for playlist stations', () => {
    const form = { ...emptyStationFilterForm(), playlist_ids: ['a', 'b'] };
    expect(filterFormToRequest('Rock', { ...form, seed_type: 'playlists' }).playlist_ids).toEqual(['a', 'b']);
    expect(filterFormToRequest('Mood', { ...form, seed_type: 'mood' }).playlist_ids).toBeUndefined();
  });
});

describe('describeStationSeed', () => {
  it('summarizes playlists and ranges', () => {
    expect(
      describeStationSeed(
        { type: 'playlists', playlist_ids: ['a', 'gone', 'b'], min_bpm: 118, max_bpm: 126, min_energy: 0.5 },
        { a: 'Rock Classics', b: '90s Rock' },
      ),
    ).toBe('Rock Classics, 90s Rock · BPM 118–126 · energy ≥ 0.50');
  });

  it('is empty without a seed', () => {
    expect(describeStationSeed(null)).toBe('');
  });
});
