import { describe, expect, it } from 'vitest';
import {
  crossfadeDuration,
  effectiveOutroEnd,
  effectiveOutroStart,
  shouldBeginCrossfade,
} from './crossfade-math';
import type { PlaybackTrack } from './types';

const track = (overrides: Partial<PlaybackTrack> = {}): PlaybackTrack => ({
  id: 't',
  duration_seconds: 200,
  outro_start_seconds: 180,
  outro_end_seconds: 195,
  ideal_crossfade_seconds: 8,
  ...overrides,
});

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

describe('effective outro points', () => {
  it('uses analysed values when valid', () => {
    expect(effectiveOutroStart(track())).toBe(180);
    expect(effectiveOutroEnd(track())).toBe(195);
  });

  it('falls back to duration minus the default crossfade', () => {
    const t = track({ outro_start_seconds: 0, outro_end_seconds: 0 });
    expect(effectiveOutroEnd(t)).toBe(200);
    expect(effectiveOutroStart(t)).toBe(195);
  });

  it('ignores an outro start past the outro end', () => {
    expect(effectiveOutroStart(track({ outro_start_seconds: 199 }))).toBe(190);
  });

  it('never crossfades when nothing is known', () => {
    expect(effectiveOutroStart(track({ duration_seconds: 0, outro_start_seconds: 0, outro_end_seconds: 0 }))).toBe(0);
  });
});

describe('crossfadeDuration', () => {
  it("uses the current track's ideal duration", () => {
    expect(crossfadeDuration(track(), track({ ideal_crossfade_seconds: 1 }))).toBe(8);
  });

  it('uses the default when ideal is unset', () => {
    expect(crossfadeDuration(track({ ideal_crossfade_seconds: 0 }), track())).toBe(5);
  });

  it('clamps to the outro length', () => {
    expect(crossfadeDuration(track({ outro_start_seconds: 192 }), track())).toBe(3);
  });

  it('clamps to the next track duration', () => {
    expect(crossfadeDuration(track(), track({ duration_seconds: 4 }))).toBe(4);
  });
});
