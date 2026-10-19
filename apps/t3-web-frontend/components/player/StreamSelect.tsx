'use client';

import { useStation } from '@/lib/station';
import type { StreamFormat } from '@/lib/playback/types';

const FORMATS: { value: StreamFormat; label: string }[] = [
  { value: 'flac', label: 'FLAC' },
  { value: 'mp3', label: 'MP3' },
  { value: 'opus', label: 'Opus' },
  { value: 'aac', label: 'AAC' },
];
const BITRATES = ['128', '192', '256', '320'];

/** Stream format and (for lossy formats) bitrate pickers. */
export function StreamSelect({ selectClassName = 'h-[26px] px-1.5 text-xs' }: { selectClassName?: string }) {
  const { stream, setStream } = useStation();
  const cls = `rounded-md border border-line bg-bg text-fg focus:outline-none focus:border-accent ${selectClassName}`;
  return (
    <div className="flex items-center gap-1.5">
      <select
        value={stream.format}
        onChange={(e) => setStream({ ...stream, format: e.target.value as StreamFormat })}
        className={cls}
        aria-label="Stream format"
      >
        {FORMATS.map((f) => (
          <option key={f.value} value={f.value}>
            {f.label}
          </option>
        ))}
      </select>
      {stream.format !== 'flac' && (
        <select
          value={stream.bitrate}
          onChange={(e) => setStream({ ...stream, bitrate: e.target.value })}
          className={cls}
          aria-label="Bitrate in kbps"
        >
          {BITRATES.map((b) => (
            <option key={b} value={b}>
              {b} kbps
            </option>
          ))}
        </select>
      )}
    </div>
  );
}
