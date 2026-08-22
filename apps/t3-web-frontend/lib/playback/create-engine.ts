import { SequentialAudioEngine } from './sequential-engine';
import type { StreamResolver } from './stream-resolver';
import type { PlaybackCallbacks } from './types';
import { WebAudioCrossfadeEngine } from './web-audio-engine';

export type PlaybackEngine = WebAudioCrossfadeEngine | SequentialAudioEngine;

export function supportsWebAudio(): boolean {
  if (typeof window === 'undefined') return false;
  return Boolean(
    (window as Window & { AudioContext?: typeof AudioContext }).AudioContext ||
      (window as Window & { webkitAudioContext?: typeof AudioContext }).webkitAudioContext,
  );
}

export function createPlaybackEngine(
  resolver: StreamResolver,
  callbacks: PlaybackCallbacks,
): PlaybackEngine {
  if (typeof window === 'undefined') {
    return new SequentialAudioEngine(resolver, callbacks);
  }
  const AudioCtx =
    (window as Window & { AudioContext?: typeof AudioContext }).AudioContext ||
    (window as Window & { webkitAudioContext?: typeof AudioContext }).webkitAudioContext;
  if (AudioCtx) {
    return new WebAudioCrossfadeEngine(resolver, callbacks, AudioCtx);
  }
  return new SequentialAudioEngine(resolver, callbacks);
}
