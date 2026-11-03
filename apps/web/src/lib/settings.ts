import { useSyncExternalStore } from 'react';
import type { Quality } from '@/api/types';

/** Player settings, kept per browser. */
export type Settings = {
  /** Quality on Wi-Fi/wired or unknown connections. */
  quality: Quality;
  /** Quality when the browser reports a cellular connection. */
  mobileQuality: Quality;
  /** Crossfade between tracks, in seconds (0 = off). */
  crossfade: number;
  normalize: boolean;
  volume: number;
};

export const defaultSettings: Settings = { quality: 'original', mobileQuality: 'low', crossfade: 0, normalize: true, volume: 0.8 };

const KEY = 'ownwave.settings';
const listeners = new Set<() => void>();
let current: Settings = load();

function load(): Settings {
  try {
    return { ...defaultSettings, ...JSON.parse(localStorage.getItem(KEY) ?? '{}') };
  } catch {
    return defaultSettings;
  }
}

export function getSettings(): Settings {
  return current;
}

export function updateSettings(patch: Partial<Settings>) {
  current = { ...current, ...patch };
  try {
    localStorage.setItem(KEY, JSON.stringify(current));
  } catch {
    // private mode: keep the settings for this session only
  }
  listeners.forEach((l) => l());
}

export function useSettings(): Settings {
  return useSyncExternalStore(
    (l) => {
      listeners.add(l);
      return () => listeners.delete(l);
    },
    () => current,
  );
}

type NetworkInformation = { type?: string; saveData?: boolean };

/** Whether the browser says it is on mobile data (Chrome on Android). */
export function onCellular(nav: Navigator = navigator): boolean {
  const c = (nav as Navigator & { connection?: NetworkInformation }).connection;
  return c?.type === 'cellular' || c?.saveData === true;
}

/** The quality to stream with right now. */
export function streamQuality(s: Settings = current, nav?: Navigator): Quality {
  return onCellular(nav) ? s.mobileQuality : s.quality;
}

export const QUALITY_LABELS: Record<Quality, string> = {
  original: 'Original (lossless)',
  high: 'High · Opus 160 kbps',
  normal: 'Normal · Opus 96 kbps',
  low: 'Data saver · Opus 64 kbps',
  aac: 'Compatible · AAC 128 kbps',
};
