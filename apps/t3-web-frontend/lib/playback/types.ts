export type PlaybackTrack = {
  id: string;
  outro_start_seconds: number;
  ideal_crossfade_seconds: number;
};

export type StreamFormat = 'flac' | 'mp3' | 'opus' | 'aac';

export type StreamOptions = {
  format: StreamFormat;
  bitrate: string;
};

export type PlaybackCallbacks = {
  onTrackChange: (track: PlaybackTrack, index: number) => void;
};

export type Deck = 'A' | 'B';
