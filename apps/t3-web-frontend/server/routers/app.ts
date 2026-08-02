import { initTRPC } from '@trpc/server';
import { z } from 'zod';
import superjson from 'superjson';
import { OwnWaveAPI } from '@/lib/api';

const GO_API_URL = process.env.GO_API_URL || 'http://localhost:8080';
const api = new OwnWaveAPI(GO_API_URL);

const t = initTRPC.create({
  transformer: superjson,
});

export const appRouter = t.router({
  tracks: t.procedure.query(async () => api.listTracks()),

  track: t.procedure
    .input(z.object({ id: z.string() }))
    .query(async ({ input }) => api.getTrack(input.id)),

  stations: t.procedure.query(async () => api.listStations()),

  queue: t.procedure
    .input(z.object({ id: z.string() }))
    .query(async ({ input }) => api.getQueue(input.id)),

  streamUrl: t.procedure
    .input(
      z.object({
        id: z.string(),
        format: z.enum(['flac', 'mp3']).default('flac'),
      })
    )
    .query(async ({ input }) => {
      const goBase = process.env.NEXT_PUBLIC_GO_API_URL || 'http://localhost:8080';
      const { url } = await api.getStreamUrl(input.id, { format: input.format });
      return `${goBase}${url}`;
    }),
});

export type AppRouter = typeof appRouter;
export type { Track, Station, QueueTrack } from '@/lib/api';
