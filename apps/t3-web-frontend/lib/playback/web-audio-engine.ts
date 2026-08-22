import { clampCrossfadeDuration, shouldBeginCrossfade } from './crossfade-math';
import type { StreamResolver } from './stream-resolver';
import type { Deck, PlaybackCallbacks, PlaybackTrack, StreamOptions } from './types';

type AudioContextCtor = typeof AudioContext;

export class WebAudioCrossfadeEngine {
  private queue: PlaybackTrack[] = [];
  private streamOptions: StreamOptions = { format: 'flac', bitrate: '320' };
  private audioA: HTMLAudioElement | null = null;
  private audioB: HTMLAudioElement | null = null;
  private ctx: AudioContext | null = null;
  private gainA: GainNode | null = null;
  private gainB: GainNode | null = null;
  private active: Deck = 'A';
  private currentIndex = 0;
  private crossfading = false;
  private crossfadeRaf: number | null = null;

  constructor(
    private readonly resolver: StreamResolver,
    private readonly callbacks: PlaybackCallbacks,
    private readonly AudioCtx: AudioContextCtor,
  ) {}

  attach(elements: [HTMLAudioElement, HTMLAudioElement]) {
    this.audioA = elements[0];
    this.audioB = elements[1];
  }

  setStreamOptions(options: StreamOptions) {
    this.streamOptions = options;
  }

  getCurrentIndex() {
    return this.currentIndex;
  }

  isCrossfading() {
    return this.crossfading;
  }

  pause() {
    this.audioA?.pause();
    this.audioB?.pause();
  }

  async load(queue: PlaybackTrack[], startIndex = 0) {
    this.queue = queue;
    this.currentIndex = startIndex;
    this.crossfading = false;
    this.cancelCrossfadeSchedule();
    await this.loadAndPlay(startIndex, 'A');
  }

  async play() {
    const audio = this.activeAudio();
    if (audio) await audio.play();
    if (this.ctx?.state === 'suspended') {
      await this.ctx.resume();
    }
  }

  async skipTo(index: number) {
    if (this.crossfading || index < 0 || index >= this.queue.length) return;
    const audio = this.activeAudio();
    if (audio) {
      audio.ontimeupdate = null;
      audio.onended = null;
    }
    await this.loadAndPlay(index, this.active);
  }

  dispose() {
    this.pause();
    this.cancelCrossfadeSchedule();
    this.ctx?.close();
    this.ctx = null;
  }

  private activeAudio() {
    return this.active === 'A' ? this.audioA : this.audioB;
  }

  private deckAudio(target: Deck) {
    return target === 'A' ? this.audioA : this.audioB;
  }

  private deckGain(target: Deck) {
    return target === 'A' ? this.gainA : this.gainB;
  }

  private ensureContext() {
    if (this.ctx) return;
    const ctx = new this.AudioCtx();
    this.ctx = ctx;

    const master = ctx.createGain();
    master.gain.value = 0.9;
    master.connect(ctx.destination);

    const gainA = ctx.createGain();
    gainA.connect(master);
    gainA.gain.value = 0;
    this.gainA = gainA;

    const gainB = ctx.createGain();
    gainB.connect(master);
    gainB.gain.value = 0;
    this.gainB = gainB;

    if (this.audioA) {
      ctx.createMediaElementSource(this.audioA).connect(gainA);
    }
    if (this.audioB) {
      ctx.createMediaElementSource(this.audioB).connect(gainB);
    }
  }

  private async resolveUrl(trackId: string) {
    return this.resolver.resolve(trackId, this.streamOptions);
  }

  private async loadTrack(audio: HTMLAudioElement, index: number) {
    const track = this.queue[index];
    if (!track) return;
    const url = await this.resolveUrl(track.id);
    audio.src = url;
    audio.load();
    this.callbacks.onTrackChange(track, index);
  }

  private async loadAndPlay(index: number, target: Deck) {
    const audio = this.deckAudio(target);
    const otherAudio = this.deckAudio(target === 'A' ? 'B' : 'A');
    if (!audio || !otherAudio) return;

    this.ensureContext();
    await this.loadTrack(audio, index);

    otherAudio.pause();
    otherAudio.src = '';

    await audio.play();

    const gain = this.deckGain(target);
    const otherGain = this.deckGain(target === 'A' ? 'B' : 'A');
    const ctx = this.ctx;
    if (gain && otherGain && ctx) {
      const now = ctx.currentTime;
      otherGain.gain.cancelScheduledValues(now);
      otherGain.gain.setValueAtTime(0, now);
      gain.gain.cancelScheduledValues(now);
      gain.gain.setValueAtTime(0, now);
      gain.gain.linearRampToValueAtTime(1, now + 0.1);
    }

    this.active = target;
    this.currentIndex = index;
    this.crossfading = false;

    audio.ontimeupdate = () => this.handleTimeUpdate(audio, target);
    audio.onended = () => this.handleEnded(target);
  }

  private handleTimeUpdate(audio: HTMLAudioElement, target: Deck) {
    if (this.crossfading) return;
    const track = this.queue[this.currentIndex];
    if (!track) return;
    if (shouldBeginCrossfade(audio.currentTime, track.outro_start_seconds)) {
      const next = this.currentIndex + 1;
      if (next < this.queue.length) {
        void this.beginCrossfade(target, next);
      }
    }
  }

  private handleEnded(target: Deck) {
    if (this.crossfading) return;
    const next = this.currentIndex + 1;
    if (next < this.queue.length) {
      void this.loadAndPlay(next, target);
    }
  }

  private async beginCrossfade(currentTarget: Deck, nextIndex: number) {
    if (this.crossfading) return;
    this.crossfading = true;

    const nextTarget: Deck = currentTarget === 'A' ? 'B' : 'A';
    const currentAudio = this.deckAudio(currentTarget);
    const nextAudio = this.deckAudio(nextTarget);
    const currentGain = this.deckGain(currentTarget);
    const nextGain = this.deckGain(nextTarget);
    const ctx = this.ctx;
    const currentTrack = this.queue[this.currentIndex];
    const nextTrack = this.queue[nextIndex];

    if (!currentAudio || !nextAudio || !currentGain || !nextGain || !ctx || !nextTrack) {
      this.crossfading = false;
      return;
    }

    const url = await this.resolveUrl(nextTrack.id);
    nextAudio.src = url;
    nextAudio.load();
    await nextAudio.play();

    const crossfade = clampCrossfadeDuration(
      nextTrack.ideal_crossfade_seconds,
      currentTrack?.outro_start_seconds ?? 0,
      currentTrack?.outro_start_seconds
        ? currentTrack.outro_start_seconds + nextTrack.ideal_crossfade_seconds
        : 0,
    );
    const now = ctx.currentTime;

    currentGain.gain.cancelScheduledValues(now);
    currentGain.gain.setValueAtTime(currentGain.gain.value, now);
    currentGain.gain.linearRampToValueAtTime(0, now + crossfade);

    nextGain.gain.cancelScheduledValues(now);
    nextGain.gain.setValueAtTime(0, now);
    nextGain.gain.linearRampToValueAtTime(1, now + crossfade);

    const endTime = now + crossfade;
    this.scheduleAtContextTime(ctx, endTime, () => {
      currentAudio.pause();
      currentAudio.src = '';
      currentAudio.ontimeupdate = null;
      currentAudio.onended = null;

      this.callbacks.onTrackChange(nextTrack, nextIndex);

      nextAudio.ontimeupdate = () => this.handleTimeUpdate(nextAudio, nextTarget);
      nextAudio.onended = () => this.handleEnded(nextTarget);

      this.active = nextTarget;
      this.currentIndex = nextIndex;
      this.crossfading = false;
    });
  }

  private cancelCrossfadeSchedule() {
    if (this.crossfadeRaf != null) {
      cancelAnimationFrame(this.crossfadeRaf);
      this.crossfadeRaf = null;
    }
  }

  private scheduleAtContextTime(ctx: AudioContext, endTime: number, onComplete: () => void) {
    this.cancelCrossfadeSchedule();
    const tick = () => {
      if (ctx.currentTime >= endTime) {
        this.crossfadeRaf = null;
        onComplete();
        return;
      }
      this.crossfadeRaf = requestAnimationFrame(tick);
    };
    this.crossfadeRaf = requestAnimationFrame(tick);
  }
}
