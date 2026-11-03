import { describe, expect, it } from 'vitest';
import { QueryClient } from '@tanstack/react-query';
import { keys } from './queries';

describe('query keys', () => {
  it('lets a rescan invalidate every track list', async () => {
    const qc = new QueryClient();
    qc.setQueryData(keys.tracks('abc'), { pages: [], pageParams: [] });
    qc.setQueryData(keys.tracks(''), { pages: [], pageParams: [] });
    await qc.invalidateQueries(['tracks']);
    expect(qc.getQueryState(keys.tracks('abc'))?.isInvalidated).toBe(true);
    expect(qc.getQueryState(keys.tracks(''))?.isInvalidated).toBe(true);
  });

  it('keys a pool genre breakdown by its playlists, in any order', () => {
    expect(keys.stationGenres(['b', 'a'])).toEqual(keys.stationGenres(['a', 'b']));
  });
});
