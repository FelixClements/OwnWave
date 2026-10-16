'use client';

import type { ScanState, ScanStatus } from '@/lib/api';
import { scanDuration, scanPhase } from '@/lib/scan';

export function ScanStateBadge({ status }: { status: ScanState }) {
  const tone =
    status === 'completed'
      ? 'bg-green-900/40 text-spotify-green'
      : status === 'failed'
        ? 'bg-red-900/40 text-red-400'
        : 'bg-spotify-elevated text-spotify-text';
  return <span className={`text-xs font-semibold px-2 py-0.5 rounded capitalize ${tone}`}>{status}</span>;
}

function Stat({ label, value }: { label: string; value: number }) {
  return (
    <div className="rounded bg-spotify-elevated p-2 text-center">
      <div className="text-lg font-bold">{value}</div>
      <div className="text-[10px] uppercase tracking-wide text-spotify-subdued">{label}</div>
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
    <div className="rounded-lg border border-spotify-border bg-spotify-card p-4 space-y-3">
      <div className="flex items-center justify-between gap-3">
        <div className="flex items-center gap-2">
          <h3 className="text-sm font-bold uppercase tracking-wide">Library scan</h3>
          <ScanStateBadge status={scan.status} />
        </div>
        {duration && <span className="text-xs text-spotify-subdued">{duration}</span>}
      </div>

      <div className="space-y-1">
        <p className={`text-sm ${scan.status === 'failed' ? 'text-red-400' : 'text-spotify-subdued'}`}>{phase.label}</p>
        <div
          role="progressbar"
          aria-label="Scan progress"
          aria-valuemin={0}
          aria-valuemax={100}
          aria-valuenow={phase.progress === null ? undefined : Math.round(phase.progress * 100)}
          className="h-2 rounded-full bg-spotify-elevated overflow-hidden"
        >
          <div
            className={`h-full transition-all ${scan.status === 'failed' ? 'bg-red-500' : 'bg-spotify-green'} ${
              running && (phase.progress === null || phase.progress === 1) ? 'animate-pulse' : ''
            }`}
            style={{ width: `${(phase.progress ?? (running ? 1 : 0)) * 100}%` }}
          />
        </div>
      </div>

      <div className="grid grid-cols-2 sm:grid-cols-4 gap-2">
        <Stat label="Found" value={stats.total_files} />
        <Stat label="Imported" value={stats.imported} />
        <Stat label="Analyzed" value={stats.features} />
        <Stat label="Failed" value={failed.length} />
      </div>

      {failed.length > 0 && (
        <details className="text-xs">
          <summary className="cursor-pointer font-semibold text-spotify-green">
            {failed.length} failed file{failed.length === 1 ? '' : 's'}
          </summary>
          <ul className="mt-2 max-h-48 overflow-y-auto rounded border border-spotify-border bg-black p-2 space-y-1">
            {failed.map((fp, i) => (
              <li key={i} className="text-spotify-subdued break-all">
                <span className="text-spotify-text font-medium">{fp.path}</span>
                <span className="mx-1">—</span>
                {fp.error}
              </li>
            ))}
          </ul>
        </details>
      )}
    </div>
  );
}
