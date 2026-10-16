import type { StationSeedType } from './station-seed-types';

/** Same-origin API prefix: the Next rewrite in dev, Caddy in production. */
export const API_BASE = '/api';

export function coverUrl(trackId: string) {
  return `${API_BASE}/tracks/${encodeURIComponent(trackId)}/cover`;
}

/** HTTP error from the Go API, carrying the response status. */
export class ApiError extends Error {
  constructor(
    readonly status: number,
    message: string,
  ) {
    super(message || `HTTP ${status}`);
    this.name = 'ApiError';
  }
}

export type Track = {
  id: string;
  title: string;
  artist?: string;
  album?: string;
  track_number?: number;
  duration_seconds?: number;
  sample_rate?: number;
  channels?: number;
};

export type Station = {
  id: string;
  name: string;
  seed_features?: string | Record<string, unknown> | null;
};

export type QueueTrack = Track & {
  bpm: number;
  key: string;
  energy: number;
  valence: number;
  intro_start_seconds: number;
  outro_start_seconds: number;
  outro_end_seconds: number;
  ideal_crossfade_seconds: number;
  position: number;
  liked?: boolean;
};

export type Feedback = 'like' | 'ban';


export type Artist = {
  id: string;
  name: string;
};

export type Album = {
  id: string;
  title: string;
};

export type SearchResults = {
  tracks: Track[];
  albums: Album[];
  artists: Artist[];
};

export type User = {
  id: string;
  username: string;
  email?: string;
  full_name?: string;
  is_admin: boolean;
};

export type AuthResponse = {
  user: User;
};

export type SimilarTrack = {
  track_id: string;
  title: string;
  artist?: string;
  album?: string;
  distance: number;
};

export type HistoryEntry = {
  track_id: string;
  title: string;
  artist?: string;
  album?: string;
  station_id?: string;
  played_at: string;
};

export type ScanResponse = {
  job_id: string;
  status: string;
};

export type StationRequest = {
  name: string;
  min_bpm?: number;
  max_bpm?: number;
  min_energy?: number;
  max_energy?: number;
  min_valence?: number;
  max_valence?: number;
  seed_type?: StationSeedType;
  track_id?: string;
  artist_id?: string;
  album_id?: string;
  cluster_id?: number;
  main_genre?: string;
  sub_genre?: string;
};

export type Genre = {
  main_genre: string;
  sub_genre: string;
  track_count: number;
};

export type FailedPath = { path: string; error: string };

export type ScanStatus = {
  id: string;
  path: string;
  status: string;
  error?: string;
  started_at?: string;
  finished_at?: string;
  created_at?: string;
  stats: {
    total_files: number;
    imported: number;
    scanned: number;
    features: number;
    model_success: number;
    model_failed: number;
    failed_paths: FailedPath[];
  };
};

export type CreateStationResponse = { station_id: string };

export type AdminStation = {
  id: string;
  name: string;
  track_count: number;
  played_count: number;
};

export type SetupStatus = {
  setup_completed: boolean;
  has_users: boolean;
  track_count?: number;
};

export type SetupSummary = {
  total_tracks: number;
  main_genres: { main_genre: string; track_count: number; sub_count: number }[];
  sub_genres: { main_genre: string; sub_genre: string; track_count: number }[];
  uncovered: number;
};

export type SetupStationsResponse = {
  created: number;
  station_ids: string[];
  uncovered: number;
};

class OwnWaveAPI {
  private async request<T>(path: string, opts: RequestInit & { json?: unknown } = {}): Promise<T> {
    const { json, ...init } = opts;
    const headers = new Headers(init.headers);
    if (json !== undefined) headers.set('Content-Type', 'application/json');
    const res = await fetch(`${API_BASE}${path}`, {
      ...init,
      headers,
      body: json !== undefined ? JSON.stringify(json) : init.body,
      credentials: 'include',
    });
    if (!res.ok) {
      throw new ApiError(res.status, (await res.text()).trim());
    }
    if (res.status === 204) {
      return undefined as T;
    }
    return (await res.json()) as T;
  }

  listTracks(opts?: { limit?: number; offset?: number; q?: string }) {
    const params = new URLSearchParams();
    if (opts?.limit !== undefined) params.set('limit', String(opts.limit));
    if (opts?.offset !== undefined) params.set('offset', String(opts.offset));
    if (opts?.q) params.set('q', opts.q);
    const query = params.toString();
    return this.request<{ tracks: Track[] }>(`/tracks${query ? `?${query}` : ''}`).then((r) => r.tracks);
  }

  listAlbums() {
    return this.request<{ albums: Album[] }>('/albums').then((r) => r.albums);
  }

  listArtists() {
    return this.request<{ artists: Artist[] }>('/artists').then((r) => r.artists);
  }

  getTrack(id: string) {
    return this.request<Track>(`/tracks/${encodeURIComponent(id)}`);
  }

  search(q: string) {
    return this.request<SearchResults>(`/search?q=${encodeURIComponent(q)}`);
  }

  triggerScan(force = false) {
    return this.request<ScanResponse>('/admin/scan', { method: 'POST', json: { force } });
  }

  getScanStatus(jobId: string) {
    return this.request<ScanStatus>(`/admin/scan/${encodeURIComponent(jobId)}`);
  }

  recordPlay(id: string, stationId?: string) {
    return this.request<void>(`/tracks/${encodeURIComponent(id)}/played`, {
      method: 'POST',
      json: { station_id: stationId ?? '' },
    });
  }

  recordFeedback(id: string, feedback: Feedback) {
    return this.request<void>(`/tracks/${encodeURIComponent(id)}/feedback`, { method: 'POST', json: { feedback } });
  }

  deleteFeedback(id: string, feedback: Feedback) {
    return this.request<void>(`/tracks/${encodeURIComponent(id)}/feedback`, { method: 'DELETE', json: { feedback } });
  }

  listHistory() {
    return this.request<{ history: HistoryEntry[] }>('/history').then((r) => r.history);
  }

  listFeedback(feedback: Feedback) {
    return this.request<{ tracks: Track[] }>(`/feedback?feedback=${feedback}`).then((r) => r.tracks);
  }

  listStations() {
    return this.request<{ stations: Station[] }>('/stations').then((r) => r.stations);
  }

  getStation(id: string) {
    return this.request<Station>(`/stations/${encodeURIComponent(id)}`);
  }

  createStation(body: StationRequest) {
    return this.request<CreateStationResponse>('/stations', { method: 'POST', json: body });
  }

  updateStation(id: string, body: StationRequest) {
    return this.request<{ station_id: string; name: string; track_count: number }>(
      `/stations/${encodeURIComponent(id)}`,
      { method: 'PUT', json: body },
    );
  }

  deleteStation(id: string) {
    return this.request<void>(`/stations/${encodeURIComponent(id)}`, { method: 'DELETE' });
  }

  getQueue(id: string) {
    return this.request<{ queue: QueueTrack[] }>(`/stations/${encodeURIComponent(id)}/queue`).then((r) => r.queue);
  }

  getSimilarTracks(id: string, limit = 20) {
    return this.request<{ track_id: string; similar: SimilarTrack[] }>(
      `/tracks/${encodeURIComponent(id)}/similar?limit=${limit}`,
    ).then((r) => r.similar);
  }

  getGenres() {
    return this.request<{ genres: Genre[] }>('/genres').then((r) => r.genres);
  }

  register(username: string, password: string, inviteToken?: string) {
    return this.request<AuthResponse>('/register', {
      method: 'POST',
      json: { username, password, invite_token: inviteToken ?? '' },
    });
  }

  login(username: string, password: string) {
    return this.request<AuthResponse>('/login', { method: 'POST', json: { username, password } });
  }

  me(): Promise<User | null> {
    return this.request<User>('/me').catch((err) => {
      if (err instanceof ApiError && err.status === 401) return null;
      throw err;
    });
  }

  updateProfile(email: string, fullName: string) {
    return this.request<{ status: string }>('/me/profile', {
      method: 'PUT',
      json: { email, full_name: fullName },
    });
  }

  logout() {
    return this.request<void>('/logout', { method: 'POST' });
  }

  changePassword(currentPassword: string, newPassword: string) {
    return this.request<{ status: string }>('/me/password', {
      method: 'POST',
      json: { current_password: currentPassword, new_password: newPassword },
    });
  }

  adminHealth() {
    return this.request<Record<string, string>>('/admin/health');
  }

  adminStations() {
    return this.request<{ stations: AdminStation[] }>('/admin/stations').then((r) => r.stations);
  }

  adminRebuildVectors() {
    return this.request<{ status: string }>('/admin/rebuild-vectors', { method: 'POST' });
  }

  adminRebuildClusters() {
    return this.request<{ status: string }>('/admin/rebuild-clusters', { method: 'POST' });
  }

  adminRebuildGenres() {
    return this.request<{ status: string }>('/admin/rebuild-genres', { method: 'POST' });
  }

  adminRebuildGenreStations() {
    return this.request<{ status: string; created: number }>('/admin/rebuild-genre-stations', { method: 'POST' });
  }

  setupStatus() {
    return this.request<SetupStatus>('/setup/status');
  }

  setupSummary() {
    return this.request<SetupSummary>('/setup/summary');
  }

  setupStations(selectedMainGenres: string[]) {
    return this.request<SetupStationsResponse>('/setup/stations', {
      method: 'POST',
      json: { selected_main_genres: selectedMainGenres },
    });
  }

  setupComplete() {
    return this.request<{ setup_completed: boolean }>('/setup/complete', { method: 'POST' });
  }

  createInvite(username?: string) {
    return this.request<{ invite_url: string; expires_at: string }>('/admin/invites', {
      method: 'POST',
      json: { username: username || null },
    });
  }

  createUser(username: string, password: string) {
    return this.request<{ user: User }>('/admin/users', { method: 'POST', json: { username, password } });
  }

  listUsers() {
    return this.request<{ users: User[] }>('/admin/users').then((r) => r.users);
  }

  deleteUser(id: string) {
    return this.request<void>(`/admin/users/${encodeURIComponent(id)}`, { method: 'DELETE' });
  }
}

export const api = new OwnWaveAPI();
