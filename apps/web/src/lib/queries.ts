import { keepPreviousData, useInfiniteQuery, useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api, call, callAs } from '@/api/client';
import type {
  AdminHealth,
  Feedback,
  GenreCorrection,
  GenreReport,
  GenreStationSettings,
  QueueTrack,
  ScanStatus,
  ScanSummary,
  SetupStatus,
  Station,
  StationRequest,
  StationSuggestion,
} from '@/api/types';

/** Query keys in one place so invalidations stay consistent. */
export const keys = {
  me: ['me'] as const,
  setup: ['setup'] as const,
  stations: ['stations'] as const,
  station: (id: string) => ['stations', id] as const,
  queue: (id: string) => ['queue', id] as const,
  hidden: (id: string) => ['hidden', id] as const,
  poolGenres: (ids: string[] | 'all') => ['poolGenres', ids === 'all' ? 'all' : [...ids].sort().join(',')] as const,
  suggestions: ['suggestions'] as const,
  browse: (node: string) => ['browse', node] as const,
  playlists: ['playlists'] as const,
  playlist: (id: string, items?: string) => ['playlists', id, items ?? 'all'] as const,
  picks: (id: string) => ['playlists', id, 'picks'] as const,
  genres: ['genres'] as const,
  tracks: (q: string) => ['tracks', q] as const,
  search: (q: string) => ['search', q] as const,
  stationSearch: (q: string) => ['stationSearch', q] as const,
  history: ['history'] as const,
  feedback: (f: Feedback) => ['feedback', f] as const,
  devices: ['devices'] as const,
  scans: ['scans'] as const,
  scan: (id: string) => ['scans', id] as const,
  health: ['health'] as const,
  genreReport: ['genreReport'] as const,
  users: ['users'] as const,
  genreStations: ['genreStations'] as const,
  genreReview: ['genreReview'] as const,
  genreCorrections: ['genreCorrections'] as const,
  pipeline: ['pipeline'] as const,
};

// --- stations -----------------------------------------------------------

export const useStations = () =>
  useQuery({ queryKey: keys.stations, queryFn: () => call(api.GET('/api/stations')).then((r) => r.stations) });

export const useStation = (id: string) =>
  useQuery({
    queryKey: keys.station(id),
    queryFn: () => call(api.GET('/api/stations/{id}', { params: { path: { id } } })),
  });

/** The server reshuffles on every request: only refetch on purpose. */
export const useQueue = (id: string | null) =>
  useQuery({
    queryKey: keys.queue(id ?? ''),
    queryFn: () => call(api.GET('/api/stations/{id}/queue', { params: { path: { id: id! } } })).then((r) => r.queue),
    enabled: !!id,
    staleTime: Infinity,
    refetchOnWindowFocus: false,
  });

export const fetchQueue = (id: string): Promise<QueueTrack[]> =>
  call(api.GET('/api/stations/{id}/queue', { params: { path: { id } } })).then((r) => r.queue);

/** Genre counts of the picked playlists, or of every playlist ('all'). */
export const usePoolGenres = (playlistIds: string[] | 'all') =>
  useQuery({
    queryKey: keys.poolGenres(playlistIds),
    queryFn: () =>
      call(
        api.GET('/api/stations/genres', {
          params: { query: playlistIds === 'all' ? { all_playlists: true } : { playlist_ids: playlistIds.join(',') } },
        }),
      ),
    enabled: playlistIds === 'all' || playlistIds.length > 0,
    placeholderData: keepPreviousData,
  });

export const useSuggestions = (enabled: boolean) =>
  useQuery({
    queryKey: keys.suggestions,
    queryFn: () => call(api.GET('/api/admin/station-suggestions')),
    enabled,
  });

function useInvalidateStations() {
  const qc = useQueryClient();
  return () => {
    void qc.invalidateQueries({ queryKey: keys.stations });
    void qc.invalidateQueries({ queryKey: keys.suggestions });
    void qc.invalidateQueries({ queryKey: ['browse'] });
    void qc.invalidateQueries({ queryKey: keys.genreStations });
  };
}

export function useSaveStation() {
  const invalidate = useInvalidateStations();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, body }: { id?: string; body: StationRequest }) =>
      id
        ? call(api.PUT('/api/stations/{id}', { params: { path: { id } }, body }))
        : call(api.POST('/api/stations', { body })),
    onSuccess: (station: Station) => {
      qc.setQueryData(keys.station(station.id), station);
      void qc.invalidateQueries({ queryKey: keys.queue(station.id) });
      invalidate();
    },
  });
}

export function useDeleteStation() {
  const invalidate = useInvalidateStations();
  return useMutation({
    mutationFn: (id: string) => call(api.DELETE('/api/stations/{id}', { params: { path: { id } } })),
    onSuccess: invalidate,
  });
}

export const useHiddenTracks = (stationId: string) =>
  useQuery({
    queryKey: keys.hidden(stationId),
    queryFn: () => call(api.GET('/api/stations/{id}/hidden-tracks', { params: { path: { id: stationId } } })).then((r) => r.tracks),
  });

/** Hide a song on a station, or bring it back (`restore`). */
export function useHideTrack() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ stationId, trackId, restore }: { stationId: string; trackId: string; restore?: boolean }) =>
      restore
        ? call(api.DELETE('/api/stations/{id}/hidden-tracks/{trackId}', { params: { path: { id: stationId, trackId } } }))
        : call(api.POST('/api/stations/{id}/hidden-tracks', { params: { path: { id: stationId } }, body: { track_id: trackId } })),
    onSettled: (_d, _e, { stationId }) => {
      void qc.invalidateQueries({ queryKey: keys.hidden(stationId) });
      void qc.invalidateQueries({ queryKey: keys.queue(stationId) });
      void qc.invalidateQueries({ queryKey: keys.stations });
    },
  });
}

export function useDismissSuggestion() {
  const invalidate = useInvalidateStations();
  return useMutation({
    mutationFn: (s: StationSuggestion) =>
      call(api.POST('/api/admin/station-suggestions/dismiss', { body: { playlist_ids: s.playlist_ids, genre: s.genre } })),
    onSuccess: invalidate,
  });
}

export const useBrowse = (node: string) =>
  useQuery({ queryKey: keys.browse(node), queryFn: () => call(api.GET('/api/browse', { params: { query: { node } } })) });

// --- playlists ----------------------------------------------------------

export const usePlaylists = () =>
  useQuery({ queryKey: keys.playlists, queryFn: () => call(api.GET('/api/playlists')).then((r) => r.playlists) });

export const usePlaylist = (id: string, items?: 'matched' | 'missing') =>
  useQuery({
    queryKey: keys.playlist(id, items),
    queryFn: () => call(api.GET('/api/playlists/{id}', { params: { path: { id }, query: items ? { items } : {} } })),
  });

export const usePlaylistPicks = (id: string, enabled: boolean) =>
  useQuery({
    queryKey: keys.picks(id),
    enabled,
    queryFn: () => call(api.GET('/api/playlists/{id}/picks', { params: { path: { id } } })).then((r) => r.picks),
  });

export function useRemovePick() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ playlistId, trackId }: { playlistId: string; trackId: string }) =>
      call(api.DELETE('/api/admin/playlists/{id}/picks/{trackId}', { params: { path: { id: playlistId, trackId } } })),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: keys.playlists });
      void qc.invalidateQueries({ queryKey: keys.stations });
    },
  });
}

export function usePlaylistActions() {
  const qc = useQueryClient();
  const invalidate = () => {
    void qc.invalidateQueries({ queryKey: keys.playlists });
    // A playlist change refreshes the genre stations on the server.
    void qc.invalidateQueries({ queryKey: keys.suggestions });
    void qc.invalidateQueries({ queryKey: keys.stations });
    void qc.invalidateQueries({ queryKey: ['browse'] });
    void qc.invalidateQueries({ queryKey: keys.genreStations });
    void qc.invalidateQueries({ queryKey: keys.pipeline });
  };
  return {
    add: useMutation({ mutationFn: (url: string) => call(api.POST('/api/admin/playlists', { body: { url } })), onSuccess: invalidate }),
    sync: useMutation({
      mutationFn: (id: string) => call(api.POST('/api/admin/playlists/{id}/sync', { params: { path: { id } } })),
      onSuccess: invalidate,
    }),
    remove: useMutation({
      mutationFn: (id: string) => call(api.DELETE('/api/admin/playlists/{id}', { params: { path: { id } } })),
      onSuccess: invalidate,
    }),
  };
}

// --- genres -------------------------------------------------------------

export const useGenres = () =>
  useQuery({ queryKey: keys.genres, queryFn: () => call(api.GET('/api/genres')), staleTime: 5 * 60_000, retry: 1 });

/** Station genre id -> display name (falls back to the id). */
export function useGenreNames(): (id: string) => string {
  const { data } = useGenres();
  const names = new Map((data?.genres ?? []).map((g) => [g.genre, g.name]));
  return (id) => names.get(id) ?? id.replace(/_/g, ' ');
}

// --- library ------------------------------------------------------------

const PAGE = 100;

export const useTracks = (q: string) =>
  useInfiniteQuery({
    queryKey: keys.tracks(q),
    initialPageParam: 0,
    queryFn: ({ pageParam }) =>
      call(api.GET('/api/tracks', { params: { query: { limit: PAGE, offset: pageParam, q: q || undefined } } })).then(
        (r) => r.tracks,
      ),
    getNextPageParam: (last, pages) => (last.length === PAGE ? pages.length * PAGE : undefined),
  });

export const useSearch = (q: string) =>
  useQuery({
    queryKey: keys.search(q),
    queryFn: () => call(api.GET('/api/search', { params: { query: { q } } })),
    enabled: q.trim().length > 1,
    placeholderData: keepPreviousData,
  });

export const useStationSearch = (q: string) =>
  useQuery({
    queryKey: keys.stationSearch(q),
    queryFn: () => call(api.GET('/api/search/stations', { params: { query: { q } } })),
    enabled: q.trim().length > 1,
    placeholderData: keepPreviousData,
  });

export const useHistory = () =>
  useQuery({ queryKey: keys.history, queryFn: () => call(api.GET('/api/history')).then((r) => r.history ?? []) });

export const useFeedbackList = (feedback: Feedback) =>
  useQuery({
    queryKey: keys.feedback(feedback),
    queryFn: () => call(api.GET('/api/feedback', { params: { query: { feedback } } })).then((r) => r.tracks),
  });

export function useFeedback() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, feedback, remove }: { id: string; feedback: Feedback; remove?: boolean }) =>
      remove
        ? call(api.DELETE('/api/tracks/{id}/feedback', { params: { path: { id } }, body: { feedback } }))
        : call(api.POST('/api/tracks/{id}/feedback', { params: { path: { id } }, body: { feedback } })),
    onSettled: (_d, _e, { feedback }) => void qc.invalidateQueries({ queryKey: keys.feedback(feedback) }),
  });
}

// --- account ------------------------------------------------------------

export const useDevices = () =>
  useQuery({ queryKey: keys.devices, queryFn: () => call(api.GET('/api/me/devices')).then((r) => r.devices) });

export function useRevokeDevice() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => call(api.DELETE('/api/me/devices/{id}', { params: { path: { id } } })),
    onSuccess: () => void qc.invalidateQueries({ queryKey: keys.devices }),
  });
}

// --- setup and admin ----------------------------------------------------

export const useSetupStatus = () =>
  useQuery({ queryKey: keys.setup, queryFn: () => callAs<SetupStatus>(api.GET('/api/setup/status')) });

export const isScanFinished = (s: ScanStatus['status']) => s === 'completed' || s === 'failed';

export const useScans = (enabled = true) =>
  useQuery({
    queryKey: keys.scans,
    queryFn: () => call(api.GET('/api/admin/scans')).then((r) => ((r as { jobs?: ScanSummary[] }).jobs ?? [])),
    enabled,
    refetchInterval: (q) => (q.state.data?.some((j) => !isScanFinished(j.status)) ? 3000 : false),
  });

export const useScan = (id: string | null) =>
  useQuery({
    queryKey: keys.scan(id ?? ''),
    queryFn: () => callAs<ScanStatus>(api.GET('/api/admin/scan/{id}', { params: { path: { id: id! } } })),
    enabled: !!id,
    refetchInterval: (q) => (q.state.data && isScanFinished(q.state.data.status) ? false : 1500),
  });

export function useStartScan() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (force: boolean) =>
      callAs<{ job_id: string; status: string }>(api.POST('/api/admin/scan', { body: { force } })),
    onSuccess: () => void qc.invalidateQueries({ queryKey: keys.scans }),
  });
}

export const useHealth = () =>
  useQuery({ queryKey: keys.health, queryFn: () => callAs<AdminHealth>(api.GET('/api/admin/health')) });

export const useGenreReport = (enabled: boolean) =>
  useQuery({
    queryKey: keys.genreReport,
    queryFn: () => callAs<GenreReport>(api.GET('/api/admin/genres/report')),
    enabled,
  });

export const useUsers = () =>
  useQuery({ queryKey: keys.users, queryFn: () => call(api.GET('/api/admin/users')).then((r) => r.users ?? []) });

// --- genre stations and the pipeline (admin) ----------------------------

export const useGenreStations = () =>
  useQuery({ queryKey: keys.genreStations, queryFn: () => call(api.GET('/api/admin/genre-stations')) });

/** After a settings change the server refreshes: stations may appear or go. */
function useGenreStationsSaved() {
  const qc = useQueryClient();
  return (settings: GenreStationSettings) => {
    qc.setQueryData(keys.genreStations, settings);
    void qc.invalidateQueries({ queryKey: keys.stations });
    void qc.invalidateQueries({ queryKey: keys.suggestions });
    void qc.invalidateQueries({ queryKey: ['browse'] });
    void qc.invalidateQueries({ queryKey: keys.pipeline });
  };
}

export function useSetGenreStations() {
  const onSuccess = useGenreStationsSaved();
  return useMutation({
    mutationFn: (enabled: boolean) => call(api.PUT('/api/admin/genre-stations', { body: { enabled } })),
    onSuccess,
  });
}

export function useUpdateGenreStation() {
  const onSuccess = useGenreStationsSaved();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ genre, ...body }: { genre: string; name?: string; excluded?: boolean }) =>
      call(api.PUT('/api/admin/genre-stations/{genre}', { params: { path: { genre } }, body })),
    onSuccess: (settings, { genre }) => {
      onSuccess(settings);
      const id = settings.genres.find((g) => g.genre === genre)?.station_id;
      if (id) void qc.invalidateQueries({ queryKey: keys.station(id) });
    },
  });
}

export const useGenreReview = () =>
  useQuery({ queryKey: keys.genreReview, queryFn: () => call(api.GET('/api/admin/genre-review')).then((r) => r.items) });

export const useGenreCorrections = () =>
  useQuery({ queryKey: keys.genreCorrections, queryFn: () => call(api.GET('/api/admin/genre-corrections')).then((r) => r.corrections) });

/** Saves or removes a correction. Scoring runs in the worker, so genre data changes a little later. */
export function useGenreCorrectionActions() {
  const qc = useQueryClient();
  const done = (corrections: { corrections: GenreCorrection[] }) => {
    qc.setQueryData(keys.genreCorrections, corrections.corrections);
    void qc.invalidateQueries({ queryKey: keys.genreReview });
    setTimeout(() => {
      for (const key of [keys.genreReview, keys.genres, keys.genreStations, keys.stations]) void qc.invalidateQueries({ queryKey: key });
    }, 5000);
  };
  return {
    save: useMutation({
      mutationFn: (body: { track_id?: string; artist?: string; genre: string }) => call(api.POST('/api/admin/genre-corrections', { body })),
      onSuccess: done,
    }),
    remove: useMutation({
      mutationFn: (id: string) => call(api.DELETE('/api/admin/genre-corrections/{id}', { params: { path: { id } } })),
      onSuccess: done,
    }),
  };
}

export const usePipeline = () =>
  useQuery({ queryKey: keys.pipeline, queryFn: () => call(api.GET('/api/admin/pipeline')), refetchInterval: 30_000 });

export function useRefreshPipeline() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => call(api.POST('/api/admin/pipeline/refresh')),
    onSuccess: () => {
      for (const key of [keys.pipeline, keys.genreStations, keys.stations, keys.suggestions, keys.playlists, ['browse']])
        void qc.invalidateQueries({ queryKey: key });
    },
  });
}
