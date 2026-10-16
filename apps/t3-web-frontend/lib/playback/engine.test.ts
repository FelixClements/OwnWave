import { describe, expect, it, vi } from 'vitest';
import { SequentialAudioEngine } from './sequential-engine';
import type { StreamResolver } from './stream-resolver';
import type { PlaybackTrack } from './types';
import { WebAudioCrossfadeEngine } from './web-audio-engine';

class FakeAudio {
  src = '';
  currentTime = 0;
  ontimeupdate: (() => void) | null = null;
  onended: (() => void) | null = null;
  playImpl: () => Promise<void> = () => Promise.resolve();
  load() {}
  pause() {}
  play() {
    return this.playImpl();
  }
  removeAttribute(name: string) {
    if (name === 'src') this.src = '';
  }
}

const resolver: StreamResolver = { resolve: async (id) => `/stream/${id}` };
const tracks = (...ids: string[]): PlaybackTrack[] =>
  ids.map((id) => ({ id, duration_seconds: 100, outro_start_seconds: 90, outro_end_seconds: 98, ideal_crossfade_seconds: 4 }));
const flush = () => new Promise((r) => setTimeout(r, 0));

function callbacks() {
  return { onTrackChange: vi.fn(), onQueueEnd: vi.fn() };
}

describe('SequentialAudioEngine', () => {
  it('advances on end and reports the queue end', async () => {
    const audio = new FakeAudio();
    const cb = callbacks();
    const engine = new SequentialAudioEngine(resolver, cb, audio as unknown as HTMLAudioElement);
    await engine.load(tracks('a', 'b'));
    expect(audio.src).toBe('/stream/a');

    audio.onended?.();
    await flush();
    expect(audio.src).toBe('/stream/b');

    audio.onended?.();
    await flush();
    expect(cb.onQueueEnd).toHaveBeenCalledOnce();
  });

  it('removing the current track plays the next one', async () => {
    const audio = new FakeAudio();
    const cb = callbacks();
    const engine = new SequentialAudioEngine(resolver, cb, audio as unknown as HTMLAudioElement);
    await engine.load(tracks('a', 'b', 'c'));
    await engine.removeTrack('a');
    expect(audio.src).toBe('/stream/b');
    expect(engine.getCurrentIndex()).toBe(0);

    await engine.removeTrack('c');
    audio.onended?.();
    await flush();
    expect(cb.onQueueEnd).toHaveBeenCalledOnce();
  });
});

function fakeContext() {
  const param = () => ({ value: 0, cancelScheduledValues() {}, setValueAtTime() {}, linearRampToValueAtTime() {} });
  return class {
    currentTime = 0;
    state = 'running';
    destination = {};
    createGain() {
      return { gain: param(), connect() {} };
    }
    createMediaElementSource() {
      return { connect() {} };
    }
    resume() {
      return Promise.resolve();
    }
    close() {
      return Promise.resolve();
    }
  } as unknown as typeof AudioContext;
}

describe('WebAudioCrossfadeEngine', () => {
  function setup() {
    const a = new FakeAudio();
    const b = new FakeAudio();
    const cb = callbacks();
    const engine = new WebAudioCrossfadeEngine(resolver, cb, fakeContext(), [
      a as unknown as HTMLAudioElement,
      b as unknown as HTMLAudioElement,
    ]);
    return { a, b, cb, engine };
  }

  it('recovers when the next deck fails to start, then advances on end', async () => {
    const { a, b, engine } = setup();
    await engine.load(tracks('a', 'b'));
    b.playImpl = () => Promise.reject(new Error('blocked'));

    a.currentTime = 95;
    a.ontimeupdate?.();
    await flush();
    expect(engine.isCrossfading()).toBe(false);

    b.playImpl = () => Promise.resolve();
    a.onended?.();
    await flush();
    expect(a.src).toBe('/stream/b');
    expect(engine.getCurrentIndex()).toBe(1);
  });

  it('removing the current track skips ahead; queue end is reported', async () => {
    const { a, cb, engine } = setup();
    await engine.load(tracks('a', 'b'));
    await engine.removeTrack('a');
    expect(a.src).toBe('/stream/b');
    expect(cb.onTrackChange).toHaveBeenLastCalledWith(expect.objectContaining({ id: 'b' }), 0);

    a.onended?.();
    await flush();
    expect(cb.onQueueEnd).toHaveBeenCalledOnce();
  });
});
