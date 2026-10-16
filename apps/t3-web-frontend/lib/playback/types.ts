export type PlaybackTrack = {
  id: string;
  duration_seconds?: number;
  outro_start_seconds: number;
  outro_end_seconds?: number;
  ideal_crossfade_seconds: number;
};

export type StreamFormat = 'flac' | 'mp3' | 'opus' | 'aac';

export type StreamOptions = {
  format: StreamFormat;
  bitrate: string;
};

export type PlaybackCallbacks = {
  onTrackChange: (track: PlaybackTrack, index: number) => void;
  /** Called after the last track of the queue finishes. */
  onQueueEnd: () => void;
};

export type Deck = 'A' | 'B';

/** Shared surface of both playback engines. */
export interface PlaybackEngine {
  setStreamOptions(options: StreamOptions): void;
  getCurrentIndex(): number;
  isCrossfading(): boolean;
  pause(): void;
  play(): Promise<void>;
  load(queue: PlaybackTrack[], startIndex?: number): Promise<void>;
  skipTo(index: number): Promise<void>;
  /** Drop a track from the queue; skips ahead if it is the current one. */
  removeTrack(id: string): Promise<void>;
  dispose(): void;
}
