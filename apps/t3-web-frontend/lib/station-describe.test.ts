import { describe, expect, it } from 'vitest';
import { describeStation } from './station-describe';

describe('describeStation', () => {
  it('names playlists and genres', () => {
    expect(
      describeStation({ playlist_ids: ['a', 'b', 'gone'], genres: ['techno', 'house'] }, { a: 'Bunker', b: 'Hard' }, { techno: 'Techno', house: 'House' }),
    ).toBe('Bunker, Hard · Techno, House');
  });

  it('reads an empty genre list as all genres and falls back to genre ids', () => {
    expect(describeStation({ playlist_ids: ['a'], genres: [] }, { a: 'Bunker' })).toBe('Bunker · all genres');
    expect(describeStation({ playlist_ids: [], genres: ['drum_and_bass'] })).toBe('drum_and_bass');
  });
});
