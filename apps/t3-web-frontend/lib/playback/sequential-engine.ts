import type { StreamResolver } from './stream-resolver';
import type { PlaybackCallbacks, PlaybackTrack, StreamOptions } from './types';

/** Fallback when Web Audio is unavailable — sequential single-deck playback. */
export class SequentialAudioEngine {
  private queue: PlaybackTrack[] = [];
  private streamOptions: StreamOptions = { format: 'flac', bitrate: '320' };
  private audio: HTMLAudioElement | null = null;
  private currentIndex = 0;

  constructor(
    private readonly resolver: StreamResolver,
    private readonly callbacks: PlaybackCallbacks,
  ) {}

  attach(audio: HTMLAudioElement) {
    this.audio = audio;
  }

  setStreamOptions(options: StreamOptions) {
    this.streamOptions = options;
  }

  getCurrentIndex() {
    return this.currentIndex;
  }

  isCrossfading() {
    return false;
  }

  pause() {
    this.audio?.pause();
  }

  async load(queue: PlaybackTrack[], startIndex = 0) {
    this.queue = queue;
    this.currentIndex = startIndex;
    if (!this.audio) return;
    await this.playIndex(startIndex);
  }

  async play() {
    if (this.audio) await this.audio.play();
  }

  async skipTo(index: number) {
    if (index < 0 || index >= this.queue.length || !this.audio) return;
    this.audio.onended = null;
    await this.playIndex(index);
  }

  dispose() {
    this.pause();
    if (this.audio) {
      this.audio.onended = null;
      this.audio.src = '';
    }
  }

  private async playIndex(index: number) {
    const audio = this.audio;
    const track = this.queue[index];
    if (!audio || !track) return;

    this.currentIndex = index;
    const url = await this.resolver.resolve(track.id, this.streamOptions);
    audio.src = url;
    audio.load();
    this.callbacks.onTrackChange(track, index);
    await audio.play();

    audio.onended = () => {
      const next = this.currentIndex + 1;
      if (next < this.queue.length) {
        void this.playIndex(next);
      }
    };
  }
}
