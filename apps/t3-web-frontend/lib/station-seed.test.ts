import { describe, expect, it } from 'vitest';
import { describeStationSeed, parseStationSeed, seedToFilterForm } from './station-seed';

describe('parseStationSeed', () => {
  it('parses JSON string seeds', () => {
    expect(parseStationSeed('{"type":"genre","main_genre":"rock"}')).toEqual({
      type: 'genre',
      main_genre: 'rock',
    });
  });

  it('normalizes legacy seed_type in form state', () => {
    expect(
      seedToFilterForm(parseStationSeed('{"seed_type":"genre","main_genre":"jazz"}')),
    ).toMatchObject({
      seed_type: 'genre',
      main_genre: 'jazz',
    });
  });
});

describe('describeStationSeed', () => {
  it('summarizes genre and ranges', () => {
    expect(
      describeStationSeed({ type: 'sub_genre', main_genre: 'Electronic', sub_genre: 'Deep House', min_bpm: 118, max_bpm: 126, min_energy: 0.5 }),
    ).toBe('Electronic › Deep House · BPM 118–126 · energy ≥ 0.50');
  });

  it('is empty without a seed', () => {
    expect(describeStationSeed(null)).toBe('');
  });
});
