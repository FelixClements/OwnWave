import { describe, expect, it } from 'vitest';
import { parseStationSeed, seedToFilterForm } from './station-seed';

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
