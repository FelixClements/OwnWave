import { initTRPC } from '@trpc/server';
import { z } from 'zod';
import superjson from 'superjson';

const GO_API_URL = process.env.GO_API_URL || 'http://localhost:8080';

const t = initTRPC.create({
  transformer: superjson,
});

async function fetchJson<T>(url: string, opts?: RequestInit): Promise<T> {
  const res = await fetch(url, opts);
  if (!res.ok) {
    throw new Error(`HTTP ${res.status}: ${await res.text()}`);
  }
  return (await res.json()) as T;
}

export const appRouter = t.router({
  tracks: t.procedure.query(async () => {
    const data = await fetchJson<{ tracks: Track[] }>(`${GO_API_URL}/tracks`);
    return data.tracks;
  }),

  track: t.procedure
    .input(z.object({ id: z.string() }))
    .query(async ({ input }) => {
      return fetchJson<Track>(`${GO_API_URL}/tracks/${input.id}`);
    }),

  stations: t.procedure.query(async () => {
    const data = await fetchJson<{ stations: Station[] }>(`${GO_API_URL}/stations`);
    return data.stations;
  }),

  queue: t.procedure
    .input(z.object({ id: z.string() }))
    .query(async ({ input }) => {
      const data = await fetchJson<{ queue: QueueTrack[] }>(
        `${GO_API_URL}/stations/${input.id}/queue`
      );
      return data.queue;
    }),

  streamUrl: t.procedure
    .input(
      z.object({
        id: z.string(),
        format: z.enum(['flac', 'mp3']).default('flac'),
      })
    )
    .query(async ({ input }) => {
      const goBase = process.env.NEXT_PUBLIC_GO_API_URL || 'http://localhost:8080';
      const data = await fetchJson<{ url: string }>(
        `${GO_API_URL}/tracks/${input.id}/stream-url?format=${input.format}`
      );
      return `${goBase}${data.url}`;
    }),
});

export type AppRouter = typeof appRouter;

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
