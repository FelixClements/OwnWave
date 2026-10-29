import { describe, expect, it } from 'vitest';
import { QueryClient } from '@tanstack/react-query';
import { keys } from './queries';

describe('query keys', () => {
  it('keeps the paged library list and the plain track list apart', () => {
    // Library caches { pages } and Similar caches Track[]; sharing a key
    // crashed whichever page was opened second.
    const qc = new QueryClient();
    qc.setQueryData(keys.tracks(''), { pages: [[]], pageParams: [0] });
    qc.setQueryData(keys.trackOptions, []);
    expect(qc.getQueryData(keys.tracks(''))).toEqual({ pages: [[]], pageParams: [0] });
    expect(qc.getQueryData(keys.trackOptions)).toEqual([]);
    expect(qc.getQueryCache().findAll(keys.trackOptions)).toHaveLength(1);
  });

  it('lets a rescan invalidate every track list', async () => {
    const qc = new QueryClient();
    qc.setQueryData(keys.tracks('abc'), { pages: [], pageParams: [] });
    qc.setQueryData(keys.trackOptions, []);
    await qc.invalidateQueries(['tracks']);
    expect(qc.getQueryState(keys.tracks('abc'))?.isInvalidated).toBe(true);
    expect(qc.getQueryState(keys.trackOptions)?.isInvalidated).toBe(true);
  });
});
