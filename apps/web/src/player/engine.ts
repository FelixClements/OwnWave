/**
 * Plays a queue of tracks through two audio elements: the active one plays,
 * the standby one preloads the next track (which also warms the server's
 * transcode cache), so tracks start without a gap. With a crossfade set,
 * the next track starts that many seconds before the current one ends and
 * the volumes cross.
 */

export type EngineTrack = { id: string; duration_seconds?: number };

/** The parts of HTMLAudioElement the engine uses (a fake in tests). */
export interface AudioLike {
  src: string;
  currentTime: number;
  readonly duration: number;
  readonly paused: boolean;
  volume: number;
  preload: '' | 'none' | 'metadata' | 'auto';
  play(): Promise<void>;
  pause(): void;
  load(): void;
  addEventListener(type: string, fn: () => void): void;
  removeEventListener(type: string, fn: () => void): void;
}

export type EngineEvents<T> = {
  /** A track became the current one (index into the engine's queue). */
  onTrack?: (track: T, index: number) => void;
  onPlaying?: (playing: boolean) => void;
  onTime?: (position: number, duration: number) => void;
  /** Fewer than LOOKAHEAD tracks are left: append more. */
  onNeedMore?: () => void;
  onError?: (track: T, error: unknown) => void;
};

/** Ask for more tracks when this many (or fewer) are left after the current one. */
export const LOOKAHEAD = 2;
const RAMP_STEP_MS = 50;
/** append() skips tracks among this many last played. */
const RECENT_SKIP = 50;
/** previous() restarts the track instead when it has played this long. */
const RESTART_AFTER = 3;

export class PlayerEngine<T extends EngineTrack> {
  private queue: T[] = [];
  private index = -1;
  private active: AudioLike;
  private standby: AudioLike;
  private standbyFor = -1; // queue index loaded in standby
  private volume = 1;
  private crossfade = 0;
  private fading: ReturnType<typeof setInterval> | null = null;
  private askedMore = false;
  private wantPlaying = false;
  private readonly detach: (() => void)[] = [];

  constructor(
    private readonly urlFor: (track: T) => string,
    private readonly events: EngineEvents<T> = {},
    audio: [AudioLike, AudioLike] = [new Audio(), new Audio()],
  ) {
    [this.active, this.standby] = audio;
    for (const el of audio) {
      el.preload = 'auto';
      this.listen(el, 'timeupdate', () => el === this.active && this.onTimeUpdate());
      this.listen(el, 'ended', () => el === this.active && this.advance());
      this.listen(el, 'play', () => el === this.active && this.events.onPlaying?.(true));
      this.listen(el, 'pause', () => el === this.active && !this.fading && this.events.onPlaying?.(false));
      this.listen(el, 'error', () => {
        if (el !== this.active) return;
        const t = this.queue[this.index];
        if (t) this.events.onError?.(t, new Error('playback error'));
        this.advance();
      });
    }
  }

  private listen(el: AudioLike, type: string, fn: () => void) {
    el.addEventListener(type, fn);
    this.detach.push(() => el.removeEventListener(type, fn));
  }

  get current(): T | null {
    return this.queue[this.index] ?? null;
  }
  get currentIndex(): number {
    return this.index;
  }
  get tracks(): readonly T[] {
    return this.queue;
  }
  get upcoming(): readonly T[] {
    return this.queue.slice(this.index + 1);
  }
  get position(): number {
    return this.active.currentTime || 0;
  }
  get duration(): number {
    const d = this.active.duration;
    return isFinite(d) && d > 0 ? d : (this.current?.duration_seconds ?? 0);
  }
  get playing(): boolean {
    return this.wantPlaying;
  }

  /** Replaces the queue and starts at index. */
  load(tracks: T[], index = 0, autoplay = true) {
    this.stopFade();
    this.queue = [...tracks];
    this.askedMore = false;
    this.standbyFor = -1;
    this.standby.src = '';
    this.start(index, autoplay);
  }

  /**
   * Adds tracks to the end, skipping ones still queued and the last
   * RECENT_SKIP played (the server's next page may repeat them).
   */
  append(tracks: T[]) {
    const skip = new Set(this.queue.slice(Math.max(0, this.index - RECENT_SKIP)).map((t) => t.id));
    const waiting = this.index >= this.queue.length; // ran out of tracks
    this.queue.push(...tracks.filter((t) => !skip.has(t.id)));
    this.askedMore = false;
    if (waiting && this.index < this.queue.length) this.start(this.index, this.wantPlaying);
    else this.preloadNext();
  }

  /** Drops a track (e.g. banned). Removing the current one skips it. */
  remove(id: string) {
    const i = this.queue.findIndex((t, k) => t.id === id && k >= this.index);
    if (i < 0) return;
    if (i === this.index) {
      this.queue.splice(i, 1);
      this.standbyFor = -1;
      this.start(i, this.wantPlaying);
      return;
    }
    this.queue.splice(i, 1);
    if (this.standbyFor === i) this.standbyFor = -1;
    else if (this.standbyFor > i) this.standbyFor--;
    this.preloadNext();
  }

  play() {
    if (!this.current) return;
    this.wantPlaying = true;
    void Promise.resolve(this.active.play()).catch((e) => this.handlePlayError(e));
    this.events.onPlaying?.(true);
  }

  pause() {
    this.wantPlaying = false;
    this.stopFade();
    this.active.pause();
    this.events.onPlaying?.(false);
  }

  toggle() {
    if (this.wantPlaying) this.pause();
    else this.play();
  }

  next() {
    this.stopFade();
    this.advance();
  }

  previous() {
    if (this.position > RESTART_AFTER || this.index <= 0) {
      this.seek(0);
      return;
    }
    this.stopFade();
    this.start(this.index - 1, this.wantPlaying);
  }

  seek(seconds: number) {
    this.active.currentTime = Math.max(0, Math.min(seconds, this.duration || seconds));
  }

  setVolume(v: number) {
    this.volume = Math.max(0, Math.min(1, v));
    if (!this.fading) this.active.volume = this.volume;
  }

  setCrossfade(seconds: number) {
    this.crossfade = Math.max(0, seconds);
  }

  dispose() {
    this.stopFade();
    this.active.pause();
    this.standby.pause();
    this.detach.forEach((d) => d());
  }

  private handlePlayError(e: unknown) {
    // Autoplay blocked: the user must press play.
    if (e instanceof DOMException && e.name === 'NotAllowedError') {
      this.wantPlaying = false;
      this.events.onPlaying?.(false);
    }
  }

  private start(index: number, autoplay: boolean) {
    if (index >= this.queue.length) {
      this.index = this.queue.length;
      this.wantPlaying = autoplay;
      this.events.onNeedMore?.();
      return;
    }
    this.index = Math.max(0, index);
    const track = this.queue[this.index];
    if (this.standbyFor === this.index) {
      this.swap();
    } else {
      this.active.pause();
      this.active.src = this.urlFor(track);
      this.active.currentTime = 0;
    }
    this.active.volume = this.volume;
    this.standbyFor = -1;
    this.events.onTrack?.(track, this.index);
    if (autoplay) this.play();
    this.preloadNext();
    this.checkMore();
  }

  private swap() {
    [this.active, this.standby] = [this.standby, this.active];
    this.standby.pause();
  }

  private advance() {
    if (this.index < this.queue.length) this.start(this.index + 1, this.wantPlaying);
  }

  private preloadNext() {
    const n = this.index + 1;
    if (n >= this.queue.length || this.standbyFor === n || this.fading) return;
    this.standby.pause();
    this.standby.src = this.urlFor(this.queue[n]);
    this.standby.load();
    this.standbyFor = n;
  }

  private checkMore() {
    if (!this.askedMore && this.queue.length - this.index - 1 <= LOOKAHEAD) {
      this.askedMore = true;
      this.events.onNeedMore?.();
    }
  }

  private onTimeUpdate() {
    const pos = this.position;
    const dur = this.duration;
    this.events.onTime?.(pos, dur);
    if (this.crossfade > 0 && !this.fading && dur > 0 && this.wantPlaying) {
      const n = this.index + 1;
      if (dur - pos <= this.crossfade && n < this.queue.length && dur > this.crossfade * 2) {
        this.startFade(n);
      }
    }
  }

  private startFade(n: number) {
    if (this.standbyFor !== n) this.preloadNext();
    const from = this.active;
    const to = this.standby;
    const steps = Math.max(1, Math.round((this.crossfade * 1000) / RAMP_STEP_MS));
    let step = 0;
    to.volume = 0;
    void Promise.resolve(to.play()).catch((e) => this.handlePlayError(e));
    // The incoming track is now the current one.
    [this.active, this.standby] = [to, from];
    this.index = n;
    this.standbyFor = -1;
    this.events.onTrack?.(this.queue[n], n);
    this.fading = setInterval(() => {
      step++;
      const k = Math.min(1, step / steps);
      to.volume = this.volume * k;
      from.volume = this.volume * (1 - k);
      if (k >= 1) {
        this.stopFade();
        from.pause();
        this.preloadNext();
      }
    }, RAMP_STEP_MS);
    this.checkMore();
  }

  private stopFade() {
    if (this.fading) {
      clearInterval(this.fading);
      this.fading = null;
      this.standby.pause();
      this.active.volume = this.volume;
    }
  }
}
