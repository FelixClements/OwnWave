import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { LOOKAHEAD, PlayerEngine, type AudioLike } from './engine';

class FakeAudio implements AudioLike {
  src = '';
  currentTime = 0;
  duration = NaN;
  paused = true;
  volume = 1;
  preload: '' | 'none' | 'metadata' | 'auto' = '';
  loads = 0;
  private handlers = new Map<string, Set<() => void>>();
  play = vi.fn(async () => {
    this.paused = false;
    this.emit('play');
  });
  pause = vi.fn(() => {
    if (!this.paused) {
      this.paused = true;
      this.emit('pause');
    }
  });
  load = vi.fn(() => {
    this.loads++;
  });
  addEventListener(type: string, fn: () => void) {
    if (!this.handlers.has(type)) this.handlers.set(type, new Set());
    this.handlers.get(type)!.add(fn);
  }
  removeEventListener(type: string, fn: () => void) {
    this.handlers.get(type)?.delete(fn);
  }
  emit(type: string) {
    this.handlers.get(type)?.forEach((fn) => fn());
  }
  /** Plays to `t` seconds of a `d`-second track. */
  at(t: number, d = 200) {
    this.duration = d;
    this.currentTime = t;
    this.emit('timeupdate');
  }
}

type T = { id: string; duration_seconds?: number };
const tracks = (...ids: string[]): T[] => ids.map((id) => ({ id, duration_seconds: 200 }));

function setup() {
  const a = new FakeAudio();
  const b = new FakeAudio();
  const events = { onTrack: vi.fn(), onNeedMore: vi.fn(), onPlaying: vi.fn(), onError: vi.fn() };
  const engine = new PlayerEngine<T>((t) => `/api/stream/${t.id}`, events, [a, b]);
  const playing = () => (a.src.endsWith(engine.current?.id ?? '?') && !a.paused ? a : b);
  return { a, b, engine, events, playing };
}

describe('PlayerEngine', () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => vi.useRealTimers());

  it('plays the first track and preloads the next one in the other element', () => {
    const { a, b, engine, events } = setup();
    engine.load(tracks('1', '2', '3', '4', '5'));
    expect(a.src).toBe('/api/stream/1');
    expect(a.play).toHaveBeenCalled();
    expect(b.src).toBe('/api/stream/2');
    expect(b.loads).toBe(1);
    expect(events.onTrack).toHaveBeenLastCalledWith({ id: '1', duration_seconds: 200 }, 0);
    expect(events.onNeedMore).not.toHaveBeenCalled();
  });

  it('switches to the preloaded element when a track ends (no reload)', () => {
    const { a, b, engine } = setup();
    engine.load(tracks('1', '2', '3', '4', '5'));
    a.emit('ended');
    expect(engine.current?.id).toBe('2');
    expect(b.play).toHaveBeenCalled();
    expect(b.loads).toBe(1);
    expect(a.src).toBe('/api/stream/3'); // a now preloads the one after
  });

  it('asks for more tracks when the queue runs low, and continues once they arrive', () => {
    const { a, b, engine, events } = setup();
    engine.load(tracks('1', '2', '3', '4'));
    expect(events.onNeedMore).not.toHaveBeenCalled();
    a.emit('ended'); // now at 2: 2 left = LOOKAHEAD
    expect(LOOKAHEAD).toBe(2);
    expect(events.onNeedMore).toHaveBeenCalledTimes(1);

    b.emit('ended');
    a.emit('ended'); // at 4, the last
    const last = engine.current?.id === '4' ? b : a;
    expect(engine.current?.id).toBe('4');
    last.emit('ended'); // ran out
    expect(engine.current).toBeNull();

    engine.append(tracks('4', '5', '6'));
    expect(engine.current?.id).toBe('5'); // 4 just played: not again
  });

  it('skips tracks still ahead or just played when appending', () => {
    const { engine } = setup();
    engine.load(tracks('1', '2', '3'));
    engine.append(tracks('1', '2', '3', '9'));
    expect(engine.upcoming.map((t) => t.id)).toEqual(['2', '3', '9']);
  });

  it('removes a banned upcoming track and skips a banned current one', () => {
    const { engine, b } = setup();
    engine.load(tracks('1', '2', '3', '4'));
    engine.remove('2');
    expect(engine.upcoming.map((t) => t.id)).toEqual(['3', '4']);
    expect(b.src).toBe('/api/stream/3');
    engine.remove('1');
    expect(engine.current?.id).toBe('3');
  });

  it('previous restarts after 3 seconds, otherwise goes back', () => {
    const { a, engine } = setup();
    engine.load(tracks('1', '2', '3', '4'));
    engine.next();
    const now = engine.current!;
    expect(now.id).toBe('2');
    const el = a.src.endsWith('/2') ? a : (engine as unknown as { active: FakeAudio }).active;
    el.at(10);
    engine.previous();
    expect(engine.current?.id).toBe('2');
    expect(el.currentTime).toBe(0);
    engine.previous();
    expect(engine.current?.id).toBe('1');
  });

  it('crossfades into the next track before the current one ends', () => {
    const { a, b, engine, events } = setup();
    engine.setVolume(0.8);
    engine.setCrossfade(4);
    engine.load(tracks('1', '2', '3', '4'));
    a.at(150);
    expect(b.play).not.toHaveBeenCalled();
    a.at(196.5);
    expect(b.play).toHaveBeenCalled();
    expect(engine.current?.id).toBe('2');
    expect(events.onTrack).toHaveBeenLastCalledWith(expect.objectContaining({ id: '2' }), 1);
    vi.advanceTimersByTime(2000);
    expect(b.volume).toBeCloseTo(0.4, 1);
    expect(a.volume).toBeCloseTo(0.4, 1);
    vi.advanceTimersByTime(2100);
    expect(b.volume).toBeCloseTo(0.8);
    expect(a.paused).toBe(true);
    expect(a.src).toBe('/api/stream/3'); // a preloads the next one again
  });

  it('pause stops a crossfade and keeps the current track', () => {
    const { a, b, engine } = setup();
    engine.setCrossfade(4);
    engine.load(tracks('1', '2', '3'));
    a.at(197);
    engine.pause();
    expect(b.paused).toBe(true);
    expect(a.paused).toBe(true);
    expect(engine.playing).toBe(false);
  });

  it('skips a track that fails to load', () => {
    const { a, engine, events } = setup();
    engine.load(tracks('1', '2', '3'));
    a.emit('error');
    expect(events.onError).toHaveBeenCalledWith(expect.objectContaining({ id: '1' }), expect.any(Error));
    expect(engine.current?.id).toBe('2');
  });
});
