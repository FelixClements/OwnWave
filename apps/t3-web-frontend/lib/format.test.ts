import { describe, expect, it } from 'vitest';
import { formatChannels, formatDuration, formatSampleRate, formatUnit, titleCase } from './format';

describe('format', () => {
  it('formats durations', () => {
    expect(formatDuration(245)).toBe('4:05');
    expect(formatDuration(0)).toBe('--:--');
    expect(formatDuration(undefined)).toBe('--:--');
  });

  it('formats sample rates', () => {
    expect(formatSampleRate(44100)).toBe('44.1 kHz');
    expect(formatSampleRate(96000)).toBe('96 kHz');
    expect(formatSampleRate(undefined)).toBe('—');
  });

  it('formats channels', () => {
    expect(formatChannels(1)).toBe('Mono');
    expect(formatChannels(2)).toBe('Stereo');
    expect(formatChannels(6)).toBe('6 ch');
  });

  it('formats 0..1 features', () => {
    expect(formatUnit(0.784)).toBe('.78');
    expect(formatUnit(1)).toBe('1.00');
    expect(formatUnit(undefined)).toBe('—');
  });

  it('title-cases genre names', () => {
    expect(titleCase('deep house')).toBe('Deep House');
  });
});
