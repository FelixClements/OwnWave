import type { ScanStatus, ScanSummary } from '@/api/types';

type Scan = ScanStatus | ScanSummary;

/**
 * Where a scan is: folder imports drive the progress (0..1, null = unknown).
 * Only folders the scan plan picked get a job, so a scan may have 0 or 1.
 */
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
      // No folder jobs yet: planning, or pruning a scan that only lost files.
      if (total === 0) return { label: 'Checking for changes…', progress: null };
      if (finished < total) return { label: `Importing folders: ${finished} of ${total}`, progress: finished / total };
      return { label: 'Finishing scan…', progress: 1 };
  }
}
