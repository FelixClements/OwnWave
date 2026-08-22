import { describe, expect, it } from 'vitest';
import {
  clampCrossfadeDuration,
  shouldBeginCrossfade,
} from './crossfade-math';
import { pinQueue } from './queue-pin';

describe('shouldBeginCrossfade', () => {
  it('returns false when outro is unset', () => {
    expect(shouldBeginCrossfade(100, 0)).toBe(false);
  });

  it('returns true at outro threshold', () => {
    expect(shouldBeginCrossfade(89.9, 90)).toBe(true);
  });

  it('returns false before threshold', () => {
    expect(shouldBeginCrossfade(89.8, 90)).toBe(false);
  });
});

describe('clampCrossfadeDuration', () => {
  it('uses default when ideal is zero', () => {
    expect(clampCrossfadeDuration(0, 100, 120)).toBe(5);
  });

  it('clamps to outro length', () => {
    expect(clampCrossfadeDuration(10, 100, 105)).toBe(5);
  });
});

describe('pinQueue', () => {
  it('returns incoming when stations match', () => {
    expect(pinQueue([1, 2], 'a', 'a', [9])).toEqual([1, 2]);
  });

  it('keeps pinned queue while browsing another station', () => {
    expect(pinQueue([1, 2], 'b', 'a', [9, 8])).toEqual([9, 8]);
  });

  it('keeps pinned queue when browsed station fetch is still empty', () => {
    expect(pinQueue([], 'b', 'a', [9, 8])).toEqual([9, 8]);
  });
});
