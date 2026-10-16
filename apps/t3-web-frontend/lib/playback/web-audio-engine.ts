import { crossfadeDuration, effectiveOutroStart, shouldBeginCrossfade } from './crossfade-math';
import type { StreamResolver } from './stream-resolver';
import type { Deck, PlaybackCallbacks, PlaybackEngine, PlaybackTrack, StreamOptions } from './types';

type AudioContextCtor = typeof AudioContext;

/** Two-deck engine that crossfades into the next track during the outro. */
export class WebAudioCrossfadeEngine implements PlaybackEngine {
  private queue: PlaybackTrack[] = [];
  private streamOptions: StreamOptions = { format: 'flac', bitrate: '320' };
  private ctx: AudioContext | null = null;
  private gains: Record<Deck, GainNode | null> = { A: null, B: null };
  private active: Deck = 'A';
  private current: PlaybackTrack | null = null;
  private crossfading = false;
  private crossfadeRaf: number | null = null;

  constructor(
    private readonly resolver: StreamResolver,
    private readonly callbacks: PlaybackCallbacks,
    private readonly AudioCtx: AudioContextCtor,
    private readonly decks: [HTMLAudioElement, HTMLAudioElement],
  ) {}

  setStreamOptions(options: StreamOptions) {
    this.streamOptions = options;
  }

  getCurrentIndex() {
    return this.current ? this.queue.indexOf(this.current) : 0;
  }

  isCrossfading() {
    return this.crossfading;
  }

  pause() {
    this.decks.forEach((d) => d.pause());
  }

  async play() {
    await this.deck(this.active).play();
    if (this.ctx?.state === 'suspended') await this.ctx.resume();
  }

  async load(queue: PlaybackTrack[], startIndex = 0) {
    this.queue = [...queue];
    this.cancelCrossfadeSchedule();
    this.crossfading = false;
    await this.loadAndPlay(startIndex, this.active);
  }

  async skipTo(index: number) {
    if (this.crossfading || index < 0) return;
    await this.loadAndPlay(index, this.active);
  }

  async removeTrack(id: string) {
    const track = this.queue.find((t) => t.id === id);
    if (!track) return;
    const index = this.queue.indexOf(track);
    this.queue.splice(index, 1);
    // While crossfading the current track is already on its way out.
    if (track === this.current && !this.crossfading) {
      await this.loadAndPlay(index, this.active);
    }
  }

  dispose() {
    this.pause();
    this.cancelCrossfadeSchedule();
    void this.ctx?.close();
    this.ctx = null;
  }

  private deck(target: Deck) {
    return target === 'A' ? this.decks[0] : this.decks[1];
  }

  private ensureContext() {
    if (this.ctx) return;
    const ctx = new this.AudioCtx();
    const master = ctx.createGain();
    master.gain.value = 0.9;
    master.connect(ctx.destination);
    for (const target of ['A', 'B'] as Deck[]) {
      const gain = ctx.createGain();
      gain.gain.value = 0;
      gain.connect(master);
      ctx.createMediaElementSource(this.deck(target)).connect(gain);
      this.gains[target] = gain;
    }
    this.ctx = ctx;
  }

  private detach(audio: HTMLAudioElement) {
    audio.ontimeupdate = null;
    audio.onended = null;
  }

  private async loadAndPlay(index: number, target: Deck) {
    const track = this.queue[index];
    const audio = this.deck(target);
    const other = this.deck(target === 'A' ? 'B' : 'A');
    this.detach(audio);
    this.detach(other);
    other.pause();
    other.removeAttribute('src');

    if (!track) {
      audio.pause();
      this.current = null;
      this.callbacks.onQueueEnd();
      return;
    }

    this.ensureContext();
    this.current = track;
    this.active = target;
    this.crossfading = false;
    audio.src = await this.resolver.resolve(track.id, this.streamOptions);
    audio.load();
    this.callbacks.onTrackChange(track, index);

    const ctx = this.ctx!;
    const now = ctx.currentTime;
    const otherGain = this.gains[target === 'A' ? 'B' : 'A']!;
    const gain = this.gains[target]!;
    otherGain.gain.cancelScheduledValues(now);
    otherGain.gain.setValueAtTime(0, now);
    gain.gain.cancelScheduledValues(now);
    gain.gain.setValueAtTime(0, now);
    gain.gain.linearRampToValueAtTime(1, now + 0.1);

    this.attach(audio, target);
    await audio.play();
  }

  private attach(audio: HTMLAudioElement, target: Deck) {
    audio.ontimeupdate = () => {
      const track = this.current;
      if (this.crossfading || !track) return;
      if (shouldBeginCrossfade(audio.currentTime, effectiveOutroStart(track))) {
        const next = this.queue[this.queue.indexOf(track) + 1];
        if (next) void this.beginCrossfade(target, track, next);
      }
    };
    audio.onended = () => {
      if (this.crossfading || !this.current) return;
      void this.loadAndPlay(this.queue.indexOf(this.current) + 1, target);
    };
  }

  private async beginCrossfade(currentTarget: Deck, current: PlaybackTrack, next: PlaybackTrack) {
    if (this.crossfading) return;
    this.crossfading = true;

    const nextTarget: Deck = currentTarget === 'A' ? 'B' : 'A';
    const currentAudio = this.deck(currentTarget);
    const nextAudio = this.deck(nextTarget);
    const ctx = this.ctx;
    const currentGain = this.gains[currentTarget];
    const nextGain = this.gains[nextTarget];
    if (!ctx || !currentGain || !nextGain) {
      this.crossfading = false;
      return;
    }

    try {
      nextAudio.src = await this.resolver.resolve(next.id, this.streamOptions);
      nextAudio.load();
      await nextAudio.play();
    } catch {
      // Couldn't start the next deck: let the current track end normally,
      // and onended advances without a crossfade.
      nextAudio.pause();
      nextAudio.removeAttribute('src');
      this.crossfading = false;
      return;
    }

    const duration = crossfadeDuration(current, next);
    const now = ctx.currentTime;
    currentGain.gain.cancelScheduledValues(now);
    currentGain.gain.setValueAtTime(currentGain.gain.value, now);
    currentGain.gain.linearRampToValueAtTime(0, now + duration);
    nextGain.gain.cancelScheduledValues(now);
    nextGain.gain.setValueAtTime(0, now);
    nextGain.gain.linearRampToValueAtTime(1, now + duration);

    this.scheduleAtContextTime(ctx, now + duration, () => {
      this.detach(currentAudio);
      currentAudio.pause();
      currentAudio.removeAttribute('src');

      this.active = nextTarget;
      this.current = next;
      this.crossfading = false;
      this.attach(nextAudio, nextTarget);
      this.callbacks.onTrackChange(next, this.queue.indexOf(next));
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
