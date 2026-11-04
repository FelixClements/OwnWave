import type { components } from './schema';

type S = components['schemas'];

export type User = S['User'];
export type Device = S['Device'];
export type Track = S['Track'];
export type QueueTrack = S['QueueTrack'];
export type HistoryEntry = S['HistoryEntry'];
export type Album = S['Album'];
export type Artist = S['Artist'];
export type Station = S['Station'];
export type StationRequest = S['StationRequest'];
export type PoolGenres = S['PoolGenres'];
export type StationSuggestion = S['StationSuggestion'];
export type GenreStation = S['GenreStation'];
export type GenreStationSettings = S['GenreStationSettings'];
export type StationChange = S['StationChange'];
export type PipelineRun = S['PipelineRun'];
export type PipelineStatus = S['PipelineStatus'];
export type BrowseItem = S['BrowseItem'];
export type BrowseNode = S['BrowseNode'];
export type SearchHit = S['SearchHit'];
export type Playlist = S['Playlist'];
export type PlaylistItem = S['PlaylistItem'];
export type GenreDistribution = S['GenreDistribution'];
export type Quality = S['Quality'];
export type Feedback = 'like' | 'ban';

// Shapes the analytics service owns (the spec types them as plain objects).
export type ScanState = 'pending' | 'running' | 'completed' | 'failed';
type ScanCounts = {
  total_files: number;
  imported: number;
  scanned: number;
  features: number;
  model_success: number;
  model_failed: number;
};
type ScanJob = {
  id: string;
  path: string;
  status: ScanState;
  error?: string | null;
  started_at?: string | null;
  finished_at?: string | null;
  created_at?: string | null;
  folders: { total: number; done: number; failed: number };
};
export type ScanStatus = ScanJob & { stats: ScanCounts & { failed_paths: { path: string; error: string }[] } };
export type ScanSummary = ScanJob & { stats: ScanCounts & { failed_files: number } };

export type SetupStatus = { setup_completed: boolean; has_users: boolean; track_count?: number };
export type AdminHealth = Record<string, string>;
export type GenreReport = {
  tracks: number;
  audio_agreement: number | null;
  per_genre: Record<string, { tracks: number; agree: number; agreement: number }>;
  top_confusions: { tag: string; audio: string | null; tracks: number }[];
};
