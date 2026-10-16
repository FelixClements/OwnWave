'use client';

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api, type Feedback, type QueueTrack, type ScanState, type ScanStatus, type ScanSummary } from '@/lib/api';

/** React Query keys, one place so invalidations stay consistent. */
export const keys = {
  me: ['me'] as const,
  setupStatus: ['setupStatus'] as const,
  setupSummary: ['setupSummary'] as const,
  stations: ['stations'] as const,
  station: (id: string) => ['station', id] as const,
  queue: (id: string) => ['queue', id] as const,
  genres: ['genres'] as const,
  tracks: (q: string) => ['tracks', q] as const,
  albums: ['albums'] as const,
  artists: ['artists'] as const,
  similar: (id: string) => ['similar', id] as const,
  history: ['history'] as const,
  feedback: (kind: Feedback) => ['feedback', kind] as const,
  scanStatus: (jobId: string) => ['scanStatus', jobId] as const,
  scans: ['scans'] as const,
  adminHealth: ['adminHealth'] as const,
  adminStations: ['adminStations'] as const,
  users: ['users'] as const,
};

export const useSetupStatus = () => useQuery(keys.setupStatus, () => api.setupStatus());
export const useStations = () => useQuery(keys.stations, () => api.listStations());
export const useGenres = () => useQuery(keys.genres, () => api.getGenres());

export function useStationQuery(id: string | null) {
  return useQuery(keys.station(id ?? ''), () => api.getStation(id!), { enabled: !!id });
}

export function useQueue(id: string | null) {
  return useQuery(keys.queue(id ?? ''), () => api.getQueue(id!), {
    enabled: !!id,
    // The server reshuffles on every request; only refetch on purpose so the
    // visible queue keeps matching what the player is playing.
    staleTime: Infinity,
    refetchOnWindowFocus: false,
  });
}

export const isScanFinished = (status: ScanState) => status === 'completed' || status === 'failed';

/** Polls a scan job every second until it completes or fails. */
export function useScanStatus(jobId: string | null) {
  return useQuery(keys.scanStatus(jobId ?? ''), () => api.getScanStatus(jobId!), {
    enabled: !!jobId,
    refetchInterval: (data?: ScanStatus) => (data && isScanFinished(data.status) ? false : 1000),
  });
}

/** Recent scans, polled while one is still in progress. */
export function useScans(enabled = true) {
  return useQuery(keys.scans, () => api.listScans(), {
    enabled,
    refetchInterval: (jobs?: ScanSummary[]) => (jobs?.some((j) => !isScanFinished(j.status)) ? 5000 : false),
  });
}

/**
 * Like/unlike/ban a track with an optimistic update of the station's cached
 * queue. The queue is not refetched afterwards: that would reshuffle it.
 */
export function useFeedbackMutation(stationId: string | null) {
  const qc = useQueryClient();
  return useMutation(
    ({ id, feedback, remove }: { id: string; feedback: Feedback; remove?: boolean }) =>
      remove ? api.deleteFeedback(id, feedback) : api.recordFeedback(id, feedback),
    {
      onMutate: async ({ id, feedback, remove }) => {
        if (!stationId) return {};
        const key = keys.queue(stationId);
        await qc.cancelQueries(key);
        const previous = qc.getQueryData<QueueTrack[]>(key);
        qc.setQueryData<QueueTrack[]>(key, (old) => {
          if (!old) return old;
          if (feedback === 'ban') return remove ? old : old.filter((t) => t.id !== id);
          return old.map((t) => (t.id === id ? { ...t, liked: !remove } : t));
        });
        return { previous };
      },
      onError: (_err, _vars, context) => {
        if (stationId && context?.previous) {
          qc.setQueryData(keys.queue(stationId), context.previous);
        }
      },
      onSettled: (_data, _err, { feedback }) => {
        qc.invalidateQueries(keys.feedback(feedback));
      },
    },
  );
}
