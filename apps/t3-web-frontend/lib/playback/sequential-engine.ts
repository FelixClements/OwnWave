import type { StreamResolver } from './stream-resolver';
import type { PlaybackCallbacks, PlaybackEngine, PlaybackTrack, StreamOptions } from './types';

/** Fallback when Web Audio is unavailable — sequential single-deck playback. */
export class SequentialAudioEngine implements PlaybackEngine {
  private queue: PlaybackTrack[] = [];
  private streamOptions: StreamOptions = { format: 'flac', bitrate: '320' };
  private currentIndex = 0;

  constructor(
    private readonly resolver: StreamResolver,
    private readonly callbacks: PlaybackCallbacks,
    private readonly audio: HTMLAudioElement,
  ) {}

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
    this.audio.pause();
  }

  async play() {
    await this.audio.play();
  }

  async load(queue: PlaybackTrack[], startIndex = 0) {
    this.queue = [...queue];
    await this.playIndex(startIndex);
  }

  async skipTo(index: number) {
    if (index < 0) return;
    await this.playIndex(index);
  }

  async removeTrack(id: string) {
    const idx = this.queue.findIndex((t) => t.id === id);
    if (idx < 0) return;
    this.queue.splice(idx, 1);
    if (idx < this.currentIndex) {
      this.currentIndex--;
    } else if (idx === this.currentIndex) {
      await this.playIndex(idx);
    }
  }

  dispose() {
    this.pause();
    this.audio.onended = null;
    this.audio.removeAttribute('src');
  }

  private async playIndex(index: number) {
    const track = this.queue[index];
    this.audio.onended = null;
    if (!track) {
      this.audio.pause();
      this.callbacks.onQueueEnd();
      return;
    }
    this.currentIndex = index;
    this.audio.src = await this.resolver.resolve(track.id, this.streamOptions);
    this.audio.load();
    this.callbacks.onTrackChange(track, index);
    this.audio.onended = () => void this.playIndex(this.currentIndex + 1);
    await this.audio.play();
  }
}
