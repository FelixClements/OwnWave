const DEFAULT_BASE_URL =
  process.env.NEXT_PUBLIC_GO_API_URL ||
  process.env.GO_API_URL ||
  'http://localhost:8080';

let authToken: string | null = null;

export function setAuthToken(token: string | null) {
  authToken = token;
}

export function getAuthToken() {
  return authToken;
}

export function getStreamBaseUrl() {
  return DEFAULT_BASE_URL;
}

export function getCoverUrl(id: string) {
  return `${DEFAULT_BASE_URL}/tracks/${id}/cover`;
}

export type Track = {
  id: string;
  title: string;
  artist?: string;
  album?: string;
  path: string;
  track_number?: number;
  duration_seconds?: number;
  sample_rate?: number;
  channels?: number;
};

export type Station = {
  id: string;
  name: string;
};

export type QueueTrack = Track & {
  bpm: number;
  key: string;
  energy: number;
  valence: number;
  outro_start_seconds: number;
  ideal_crossfade_seconds: number;
  position: number;
  liked?: boolean;
};

export type HealthResponse = { status: string };
export type StationStatusResponse = { id: string; status: string };

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
};

export type AuthResponse = {
  token: string;
  user: User;
};

export type SimilarTrack = {
  track_id: string;
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

export type StreamUrlResponse = { url: string };
export type StreamUrlOptions = {
  format?: 'flac' | 'mp3' | 'opus' | 'aac';
  bitrate?: string;
};

export type ScanRequest = {
  path?: string;
  force?: boolean;
};

export type ScanResponse = {
  job_id: string;
  status: string;
};

export type CreateStationRequest = {
  name: string;
  length?: number;
  min_bpm?: number;
  max_bpm?: number;
  min_energy?: number;
  max_energy?: number;
  min_valence?: number;
  max_valence?: number;
  seed_type?: 'track' | 'artist' | 'album' | 'cluster' | 'mood';
  track_id?: string;
  artist_id?: string;
  album_id?: string;
  cluster_id?: number;
};

export type UpdateStationRequest = {
  name: string;
  length?: number;
  min_bpm?: number;
  max_bpm?: number;
  min_energy?: number;
  max_energy?: number;
  min_valence?: number;
  max_valence?: number;
  seed_type?: 'track' | 'artist' | 'album' | 'cluster' | 'mood';
  track_id?: string;
  artist_id?: string;
  album_id?: string;
  cluster_id?: number;
};

export type CreateStationResponse = { station_id: string };

export class OwnWaveAPI {
  constructor(private baseURL: string = DEFAULT_BASE_URL) {}

  private async request<T>(path: string, opts?: RequestInit): Promise<T> {
    const headers: HeadersInit = {
      ...(opts?.headers || {}),
    };
    if (authToken) {
      (headers as Record<string, string>)['Authorization'] = `Bearer ${authToken}`;
    }
    const res = await fetch(`${this.baseURL}${path}`, {
      ...opts,
      headers,
    });
    if (!res.ok) {
      throw new Error(`HTTP ${res.status}: ${await res.text()}`);
    }
    if (res.status === 204) {
      return undefined as T;
    }
    return (await res.json()) as T;
  }

  health() {
    return this.request<HealthResponse>('/health');
  }

  listTracks() {
    return this.request<{ tracks: Track[] }>('/tracks').then((r) => r.tracks);
  }

  listAlbums() {
    return this.request<{ albums: Album[] }>('/albums').then((r) => r.albums);
  }

  listArtists() {
    return this.request<{ artists: Artist[] }>('/artists').then((r) => r.artists);
  }

  rescan() {
    return this.request<{ job_id?: string }>('/rescan', { method: 'POST' });
  }

  getTrack(id: string) {
    return this.request<Track>(`/tracks/${encodeURIComponent(id)}`);
  }

  listStations() {
    return this.request<{ stations: Station[] }>('/stations').then((r) => r.stations);
  }

  search(q: string) {
    return this.request<SearchResults>(`/search?q=${encodeURIComponent(q)}`);
  }

  getCoverUrl(id: string) {
    return `${this.baseURL}/tracks/${id}/cover`;
  }

  recordPlay(id: string, stationId?: string) {
    return this.request(`/tracks/${id}/played`, {
      method: 'POST',
      body: JSON.stringify({ station_id: stationId ?? '' }),
    });
  }

  recordFeedback(id: string, feedback: 'like' | 'skip' | 'ban') {
    return this.request(`/tracks/${id}/feedback`, {
      method: 'POST',
      body: JSON.stringify({ feedback }),
    });
  }

  listHistory() {
    return this.request<{ history: HistoryEntry[] }>('/history').then((r) => r.history);
  }

  listFeedback(feedback: 'like' | 'skip' | 'ban') {
    return this.request<{ tracks: Track[] }>(`/feedback?feedback=${feedback}`).then((r) => r.tracks);
  }

  getStation(id: string) {
    return this.request<Station>(`/stations/${encodeURIComponent(id)}`);
  }

  updateStation(id: string, body: UpdateStationRequest) {
    return this.request<void>(`/stations/${encodeURIComponent(id)}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    });
  }

  deleteStation(id: string) {
    return this.request<void>(`/stations/${encodeURIComponent(id)}`, {
      method: 'DELETE',
    });
  }

  getQueue(id: string) {
    return this.request<{ queue: QueueTrack[] }>(
      `/stations/${encodeURIComponent(id)}/queue`
    ).then((r) => r.queue);
  }

  getSimilarTracks(id: string, limit = 20) {
    return this.request<{ track_id: string; similar: SimilarTrack[] }>(
      `/tracks/${encodeURIComponent(id)}/similar?limit=${limit}`
    ).then((r) => r.similar);
  }

  register(username: string, password: string) {
    return this.request<AuthResponse>('/register', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username, password }),
    });
  }

  login(username: string, password: string) {
    return this.request<AuthResponse>('/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username, password }),
    });
  }

  me(): Promise<User | null> {
    return this.request<User>('/me').catch((err) => {
      if (err instanceof Error && err.message.includes('401')) {
        return null;
      }
      throw err;
    });
  }

  logout() {
    return this.request<void>('/logout', { method: 'POST' });
  }

  adminHealth() {
    return this.request<Record<string, string>>('/admin/health');
  }

  adminStations() {
    return this.request<{ stations: { id: string; name: string; track_count: number; played_count: number }[] }>(
      '/admin/stations'
    ).then((r) => r.stations);
  }

  adminScan(path?: string, force?: boolean) {
    return this.request<{ status: string }>('/admin/scan', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ path: path ?? '', force: force ?? false }),
    });
  }

  adminRebuildVectors() {
    return this.request<{ status: string }>('/admin/rebuild-vectors', { method: 'POST' });
  }

  adminRebuildClusters() {
    return this.request<{ status: string }>('/admin/rebuild-clusters', { method: 'POST' });
  }

  getStreamUrl(id: string, opts?: StreamUrlOptions) {
    const params = new URLSearchParams();
    if (opts?.format) params.set('format', opts.format);
    if (opts?.bitrate) params.set('bitrate', opts.bitrate);
    const query = params.toString();
    const suffix = query ? `?${query}` : '';
    return this.request<StreamUrlResponse>(
      `/tracks/${encodeURIComponent(id)}/stream-url${suffix}`
    );
  }

  createStation(body: CreateStationRequest) {
    return this.request<CreateStationResponse>('/stations', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    });
  }

  triggerScan(body: ScanRequest = {}) {
    return this.request<ScanResponse>('/admin/scan', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    });
  }
}

export const api = new OwnWaveAPI();
