import { SequentialAudioEngine } from './sequential-engine';
import type { StreamResolver } from './stream-resolver';
import type { PlaybackCallbacks, PlaybackEngine } from './types';
import { WebAudioCrossfadeEngine } from './web-audio-engine';

export function createPlaybackEngine(
  resolver: StreamResolver,
  callbacks: PlaybackCallbacks,
  audio: [HTMLAudioElement, HTMLAudioElement],
): PlaybackEngine {
  const AudioCtx =
    typeof window === 'undefined'
      ? undefined
      : (window as Window & { AudioContext?: typeof AudioContext }).AudioContext ||
        (window as Window & { webkitAudioContext?: typeof AudioContext }).webkitAudioContext;
  return AudioCtx
    ? new WebAudioCrossfadeEngine(resolver, callbacks, AudioCtx, audio)
    : new SequentialAudioEngine(resolver, callbacks, audio[0]);
}
