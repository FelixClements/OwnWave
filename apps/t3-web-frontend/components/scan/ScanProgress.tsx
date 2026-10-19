'use client';

import type { ScanState, ScanStatus } from '@/lib/api';
import { scanDuration, scanPhase } from '@/lib/scan';
import { Badge } from '@/components/ui/Layout';

export function ScanStateBadge({ status }: { status: ScanState }) {
  const tone = status === 'completed' ? 'ok' : status === 'failed' ? 'danger' : 'accent';
  return (
    <Badge tone={tone} className="capitalize">
      {status}
    </Badge>
  );
}

function Stat({ label, value }: { label: string; value: number }) {
  return (
    <div className="flex items-baseline gap-1.5">
      <dt className="text-muted">{label}</dt>
      <dd className="font-semibold tabular-nums">{value.toLocaleString()}</dd>
    </div>
  );
}

/** Live view of one library scan: phase, progress bar, counts and failed files. */
export function ScanProgress({ scan }: { scan: ScanStatus }) {
  const phase = scanPhase(scan);
  const duration = scanDuration(scan);
  const { stats } = scan;
  const failed = stats.failed_paths;
  const running = scan.status === 'pending' || scan.status === 'running';

  return (
    <section aria-label="Library scan" className="rounded-lg border border-line bg-surface p-3 space-y-2.5">
      <div className="flex flex-wrap items-center gap-3">
        <ScanStateBadge status={scan.status} />
        <p className={`flex-1 min-w-0 truncate ${scan.status === 'failed' ? 'text-danger' : ''}`} aria-live="polite">
          {phase.label}
        </p>
        {duration && <span className="text-muted tabular-nums">{duration}</span>}
      </div>

      <div
        role="progressbar"
        aria-label="Scan progress"
        aria-valuemin={0}
        aria-valuemax={100}
        aria-valuenow={phase.progress === null ? undefined : Math.round(phase.progress * 100)}
        className="h-1.5 rounded-full bg-hover overflow-hidden"
      >
        <div
          className={`h-full transition-all ${scan.status === 'failed' ? 'bg-danger' : 'bg-accent'} ${
            running && (phase.progress === null || phase.progress === 1) ? 'animate-pulse' : ''
          }`}
          style={{ width: `${(phase.progress ?? (running ? 1 : 0)) * 100}%` }}
        />
      </div>

      <dl className="flex flex-wrap gap-x-5 gap-y-1">
        <Stat label="Found" value={stats.total_files} />
        <Stat label="Imported" value={stats.imported} />
        <Stat label="Analyzed" value={stats.features} />
        <Stat label="Failed" value={failed.length} />
      </dl>

      {failed.length > 0 && (
        <details>
          <summary className="cursor-pointer font-semibold text-accent">
            {failed.length} failed file{failed.length === 1 ? '' : 's'}
          </summary>
          <ul className="mt-2 max-h-48 overflow-y-auto rounded border border-line bg-bg p-2 space-y-1 text-[11.5px]">
            {failed.map((fp, i) => (
              <li key={i} className="text-muted break-all">
                <span className="text-fg font-medium">{fp.path}</span>
                <span className="mx-1">—</span>
                {fp.error}
              </li>
            ))}
          </ul>
        </details>
      )}
    </section>
  );
}
