const DEFAULT_BASE_URL =
  process.env.GO_API_URL ||
  process.env.NEXT_PUBLIC_GO_API_URL ||
  'http://localhost:8080';

export function getCoverUrl(id: string) {
  return `${DEFAULT_BASE_URL}/tracks/${id}/cover`;
}

export function getStreamBaseUrl() {
  return DEFAULT_BASE_URL;
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
};

export type CreateStationResponse = { station_id: string };

export class OwnWaveAPI {
  constructor(private baseURL: string = DEFAULT_BASE_URL) {}

  private async request<T>(path: string, opts?: RequestInit): Promise<T> {
    const res = await fetch(`${this.baseURL}${path}`, opts);
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
