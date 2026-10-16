import { describe, expect, it } from 'vitest';
import type { ScanStatus } from './api';
import { failedFileCount, scanDuration, scanPhase } from './scan';

function scan(overrides: Partial<ScanStatus> = {}): ScanStatus {
  return {
    id: 'j',
    path: '/music',
    status: 'running',
    folders: { total: 4, done: 1, failed: 1 },
    stats: {
      total_files: 0,
      imported: 0,
      scanned: 0,
      features: 0,
      model_success: 0,
      model_failed: 0,
      failed_paths: [{ path: '/music/x.flac', error: 'bad' }],
    },
    ...overrides,
  };
}

describe('scanPhase', () => {
  it('reports folder progress while importing', () => {
    expect(scanPhase(scan())).toEqual({ label: 'Importing folders: 2 of 4', progress: 0.5 });
  });

  it('reports the analysis step once every folder is done', () => {
    const phase = scanPhase(scan({ folders: { total: 4, done: 4, failed: 0 } }));
    expect(phase.label).toMatch(/Analyzing/);
    expect(phase.progress).toBe(1);
  });

  it('has no progress before folders are known', () => {
    expect(scanPhase(scan({ status: 'pending' })).progress).toBeNull();
    expect(scanPhase(scan({ folders: { total: 0, done: 0, failed: 0 } })).progress).toBeNull();
  });

  it('shows the error of a failed scan', () => {
    expect(scanPhase(scan({ status: 'failed', error: '1 / 4 folder tasks failed' })).label).toBe(
      '1 / 4 folder tasks failed',
    );
  });
});

describe('failedFileCount', () => {
  it('counts the full list or uses the summary count', () => {
    expect(failedFileCount(scan())).toBe(1);
    const summary = { ...scan(), stats: { ...scan().stats, failed_paths: undefined, failed_files: 7 } };
    delete (summary.stats as { failed_paths?: unknown }).failed_paths;
    expect(failedFileCount(summary as never)).toBe(7);
  });
});

describe('scanDuration', () => {
  it('formats finished and running scans', () => {
    const started_at = '2026-01-01T00:00:00Z';
    expect(scanDuration(scan({ started_at, finished_at: '2026-01-01T00:00:42Z' }))).toBe('42s');
    expect(scanDuration(scan({ started_at, finished_at: '2026-01-01T00:03:05Z' }))).toBe('3m 05s');
    expect(scanDuration(scan({ started_at }), Date.parse('2026-01-01T01:02:00Z'))).toBe('1h 02m');
    expect(scanDuration(scan())).toBeNull();
  });
});
