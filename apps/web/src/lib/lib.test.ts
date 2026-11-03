import { describe, expect, it } from 'vitest';
import { darken, dominantColor, nameColor } from './color';
import { formatDuration, greeting, plural, timeAgo } from './format';
import { scanPhase } from './scan';
import { defaultSettings, onCellular, streamQuality } from './settings';
import type { ScanSummary } from '@/api/types';

describe('format', () => {
  it('formats durations', () => {
    expect(formatDuration(187)).toBe('3:07');
    expect(formatDuration(3723)).toBe('1:02:03');
    expect(formatDuration(undefined)).toBe('–:––');
  });
  it('says how long ago', () => {
    const now = Date.parse('2026-11-03T12:00:00Z');
    expect(timeAgo('2026-11-03T11:59:30Z', now)).toBe('just now');
    expect(timeAgo('2026-11-03T11:55:00Z', now)).toBe('5 min ago');
    expect(timeAgo('2026-11-02T11:00:00Z', now)).toBe('yesterday');
  });
  it('pluralizes and greets', () => {
    expect(plural(1, 'track')).toBe('1 track');
    expect(plural(1200, 'track')).toBe('1,200 tracks');
    expect(greeting(new Date('2026-11-03T08:00:00'))).toBe('Good morning');
    expect(greeting(new Date('2026-11-03T21:00:00'))).toBe('Good evening');
  });
});

describe('color', () => {
  it('prefers a saturated colour over grey', () => {
    const w = 8;
    const px = new Uint8ClampedArray(w * w * 4);
    for (let i = 0; i < w * w; i++) {
      const red = i % w < 2 && Math.floor(i / w) < 2; // one cell of the 4x4 grid
      px.set(red ? [220, 30, 40, 255] : [120, 120, 120, 255], i * 4);
    }
    const [r, g, b] = dominantColor(px, w, w);
    expect(r).toBeGreaterThan(g * 3);
    expect(Math.max(r, g, b)).toBeLessThanOrEqual(150);
  });
  it('darkens and names colours stably', () => {
    expect(darken([255, 128, 0])).toEqual([150, 75, 0]);
    expect(nameColor('Techno')).toEqual(nameColor('Techno'));
    expect(nameColor('Techno')).not.toEqual(nameColor('House'));
  });
});

describe('settings', () => {
  const nav = (connection?: object) => ({ connection }) as unknown as Navigator;
  it('uses the mobile quality on cellular or data saver', () => {
    const s = { ...defaultSettings, quality: 'original' as const, mobileQuality: 'low' as const };
    expect(streamQuality(s, nav({ type: 'wifi' }))).toBe('original');
    expect(streamQuality(s, nav({ type: 'cellular' }))).toBe('low');
    expect(streamQuality(s, nav({ saveData: true }))).toBe('low');
    expect(streamQuality(s, nav())).toBe('original');
    expect(onCellular(nav())).toBe(false);
  });
});

describe('scanPhase', () => {
  const scan = (status: ScanSummary['status'], total: number, done: number): ScanSummary => ({
    id: '1', path: '/music', status, folders: { total, done, failed: 0 },
    stats: { total_files: 0, imported: 0, scanned: 0, features: 0, model_success: 0, model_failed: 0, failed_files: 0 },
  });
  it('reports progress through the folders', () => {
    expect(scanPhase(scan('running', 0, 0)).progress).toBeNull();
    expect(scanPhase(scan('running', 4, 1))).toEqual({ label: 'Importing folders: 1 of 4', progress: 0.25 });
    expect(scanPhase(scan('running', 4, 4)).label).toBe('Finishing scan…');
    expect(scanPhase(scan('completed', 4, 4)).progress).toBe(1);
  });
});
