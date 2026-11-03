import type { ScanStatus, ScanSummary } from '@/api/types';

type Scan = ScanStatus | ScanSummary;

/** Where a scan is: folder imports drive the progress (0..1, null = unknown). */
export function scanPhase(scan: Scan): { label: string; progress: number | null } {
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
      if (finished < total) return { label: `Importing folders: ${finished} of ${total}`, progress: finished / total };
      return { label: 'Finishing scan…', progress: 1 };
  }
}
