import type { ScanStatus, ScanSummary } from '@/lib/api';

type Scan = ScanStatus | ScanSummary;

export type ScanPhase = {
  label: string;
  /** 0..1, or null while there is nothing to measure yet. */
  progress: number | null;
};

/**
 * Where a scan is. Folder imports drive the progress bar; once every folder
 * is done the job stays running while vectors and clusters are rebuilt.
 */
export function scanPhase(scan: Scan): ScanPhase {
  const { total, done, failed } = scan.folders;
  const finished = done + failed;
  switch (scan.status) {
    case 'pending':
      return { label: 'Waiting for a worker…', progress: null };
    case 'completed':
      return { label: 'Completed', progress: 1 };
    case 'failed':
      return { label: scan.error || 'Failed', progress: total ? finished / total : null };
    case 'running':
      if (total === 0) return { label: 'Looking for music folders…', progress: null };
      if (finished < total) {
        return { label: `Importing folders: ${finished} of ${total}`, progress: finished / total };
      }
      return { label: 'Analyzing library (similarity and clusters)…', progress: 1 };
  }
}

export function failedFileCount(scan: Scan): number {
  return 'failed_paths' in scan.stats ? scan.stats.failed_paths.length : scan.stats.failed_files;
}

/** "42s", "3m 05s", "1h 02m"; measured to now while the scan is still running. */
export function scanDuration(scan: Scan, now = Date.now()): string | null {
  if (!scan.started_at) return null;
  const end = scan.finished_at ? Date.parse(scan.finished_at) : now;
  const secs = Math.max(0, Math.round((end - Date.parse(scan.started_at)) / 1000));
  const pad = (n: number) => String(n).padStart(2, '0');
  if (secs < 60) return `${secs}s`;
  if (secs < 3600) return `${Math.floor(secs / 60)}m ${pad(secs % 60)}s`;
  return `${Math.floor(secs / 3600)}h ${pad(Math.floor((secs % 3600) / 60))}m`;
}
