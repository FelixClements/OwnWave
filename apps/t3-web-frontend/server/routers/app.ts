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
  tracks: t.procedure
    .input(
      z.object({
        limit: z.number().optional(),
        q: z.string().optional(),
        cursor: z.number().optional(),
      })
    )
    .query(async ({ input }) =>
      api.listTracks({
        limit: input.limit,
        offset: input.cursor,
        q: input.q,
      })
    ),

  albums: t.procedure.query(async () => api.listAlbums()),

  artists: t.procedure.query(async () => api.listArtists()),

  rescan: t.procedure.mutation(async () => api.rescan()),

  register: t.procedure
    .input(z.object({ username: z.string(), password: z.string() }))
    .mutation(async ({ input }) => api.register(input.username, input.password)),

  login: t.procedure
    .input(z.object({ username: z.string(), password: z.string() }))
    .mutation(async ({ input }) => api.login(input.username, input.password)),

  me: t.procedure.query(async () => api.me()),

  updateProfile: t.procedure
    .input(z.object({ email: z.string(), fullName: z.string() }))
    .mutation(async ({ input }) => api.updateProfile(input.email, input.fullName)),

  logout: t.procedure.mutation(async () => api.logout()),

  changePassword: t.procedure
    .input(z.object({ currentPassword: z.string(), newPassword: z.string() }))
    .mutation(async ({ input }) => api.changePassword(input.currentPassword, input.newPassword)),

  similar: t.procedure
    .input(z.object({ id: z.string(), limit: z.number().optional() }))
    .query(async ({ input }) => api.getSimilarTracks(input.id, input.limit ?? 20)),

  adminHealth: t.procedure.query(async () => api.adminHealth()),
  adminStations: t.procedure.query(async () => api.adminStations()),
  adminScan: t.procedure
    .input(z.object({ path: z.string().optional(), force: z.boolean().optional() }))
    .mutation(async ({ input }) => api.adminScan(input.path, input.force)),
  adminRebuildVectors: t.procedure.mutation(async () => api.adminRebuildVectors()),
  adminRebuildClusters: t.procedure.mutation(async () => api.adminRebuildClusters()),
  adminRebuildGenres: t.procedure.mutation(async () => api.adminRebuildGenres()),
  adminRebuildGenreStations: t.procedure.mutation(async () => api.adminRebuildGenreStations()),

  genres: t.procedure.query(async () => api.getGenres()),
  trackGenres: t.procedure.input(z.object({ id: z.string() })).query(async ({ input }) => api.getTrackGenres(input.id)),

  recordPlay: t.procedure
    .input(z.object({ id: z.string(), stationId: z.string().optional() }))
    .mutation(async ({ input }) => api.recordPlay(input.id, input.stationId)),

  recordFeedback: t.procedure
    .input(z.object({ id: z.string(), feedback: z.enum(['like', 'skip', 'ban']) }))
    .mutation(async ({ input }) => api.recordFeedback(input.id, input.feedback)),

  removeFeedback: t.procedure
    .input(z.object({ id: z.string(), feedback: z.enum(['like', 'skip', 'ban']) }))
    .mutation(async ({ input }) => api.deleteFeedback(input.id, input.feedback)),

  history: t.procedure.query(async () => api.listHistory()),

  liked: t.procedure.query(async () => api.listFeedback('like')),
  skipped: t.procedure.query(async () => api.listFeedback('skip')),
  banned: t.procedure.query(async () => api.listFeedback('ban')),

  track: t.procedure
    .input(z.object({ id: z.string() }))
    .query(async ({ input }) => api.getTrack(input.id)),

  stations: t.procedure.query(async () => api.listStations()),

  search: t.procedure
    .input(z.object({ q: z.string() }))
    .query(async ({ input }) => api.search(input.q)),

  queue: t.procedure
    .input(z.object({ id: z.string() }))
    .query(async ({ input }) => api.getQueue(input.id)),

  station: t.procedure
    .input(z.object({ id: z.string() }))
    .query(async ({ input }) => api.getStation(input.id)),

  createStation: t.procedure
    .input(
      z.object({
        name: z.string(),
        length: z.number().optional(),
        min_bpm: z.number().optional(),
        max_bpm: z.number().optional(),
        min_energy: z.number().optional(),
        max_energy: z.number().optional(),
        min_valence: z.number().optional(),
        max_valence: z.number().optional(),
        seed_type: z.enum(['track', 'artist', 'album', 'cluster', 'mood', 'genre', 'sub_genre']).optional(),
        track_id: z.string().optional(),
        artist_id: z.string().optional(),
        album_id: z.string().optional(),
        cluster_id: z.number().optional(),
        main_genre: z.string().optional(),
        sub_genre: z.string().optional(),
      })
    )
    .mutation(async ({ input }) => api.createStation(input)),

  updateStation: t.procedure
    .input(
      z.object({
        id: z.string(),
        name: z.string(),
        length: z.number().optional(),
        min_bpm: z.number().optional(),
        max_bpm: z.number().optional(),
        min_energy: z.number().optional(),
        max_energy: z.number().optional(),
        min_valence: z.number().optional(),
        max_valence: z.number().optional(),
        seed_type: z.enum(['track', 'artist', 'album', 'cluster', 'mood', 'genre', 'sub_genre']).optional(),
        track_id: z.string().optional(),
        artist_id: z.string().optional(),
        album_id: z.string().optional(),
        cluster_id: z.number().optional(),
        main_genre: z.string().optional(),
        sub_genre: z.string().optional(),
      })
    )
    .mutation(async ({ input }) => api.updateStation(input.id, input)),

  deleteStation: t.procedure
    .input(z.object({ id: z.string() }))
    .mutation(async ({ input }) => api.deleteStation(input.id)),

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
