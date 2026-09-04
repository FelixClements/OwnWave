import { initTRPC } from '@trpc/server';
import { z } from 'zod';
import superjson from 'superjson';
import { OwnWaveAPI } from '@/lib/api';
import { STATION_SEED_TYPES } from '@/lib/station-seed-types';

const stationSeedTypeSchema = z.enum(STATION_SEED_TYPES);

const t = initTRPC.context<{ api: OwnWaveAPI }>().create({
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
    .query(async ({ ctx, input }) =>
      ctx.api.listTracks({
        limit: input.limit,
        offset: input.cursor,
        q: input.q,
      })
    ),

  albums: t.procedure.query(async ({ ctx }) => ctx.api.listAlbums()),

  artists: t.procedure.query(async ({ ctx }) => ctx.api.listArtists()),

  rescan: t.procedure.mutation(async ({ ctx }) => ctx.api.rescan()),

  scanStatus: t.procedure
    .input(z.object({ jobId: z.string() }))
    .query(async ({ ctx, input }) => ctx.api.getScanStatus(input.jobId)),

  me: t.procedure.query(async ({ ctx }) => ctx.api.me()),

  updateProfile: t.procedure
    .input(z.object({ email: z.string(), fullName: z.string() }))
    .mutation(async ({ ctx, input }) => ctx.api.updateProfile(input.email, input.fullName)),

  logout: t.procedure.mutation(async ({ ctx }) => ctx.api.logout()),

  changePassword: t.procedure
    .input(z.object({ currentPassword: z.string(), newPassword: z.string() }))
    .mutation(async ({ ctx, input }) => ctx.api.changePassword(input.currentPassword, input.newPassword)),

  similar: t.procedure
    .input(z.object({ id: z.string(), limit: z.number().optional() }))
    .query(async ({ ctx, input }) => ctx.api.getSimilarTracks(input.id, input.limit ?? 20)),

  adminHealth: t.procedure.query(async ({ ctx }) => ctx.api.adminHealth()),
  adminStations: t.procedure.query(async ({ ctx }) => ctx.api.adminStations()),
  adminScan: t.procedure
    .input(z.object({ path: z.string().optional(), force: z.boolean().optional() }))
    .mutation(async ({ ctx, input }) => ctx.api.adminScan(input.path, input.force)),
  adminRebuildVectors: t.procedure.mutation(async ({ ctx }) => ctx.api.adminRebuildVectors()),
  adminRebuildClusters: t.procedure.mutation(async ({ ctx }) => ctx.api.adminRebuildClusters()),
  adminRebuildGenres: t.procedure.mutation(async ({ ctx }) => ctx.api.adminRebuildGenres()),
  adminRebuildGenreStations: t.procedure.mutation(async ({ ctx }) => ctx.api.adminRebuildGenreStations()),

  genres: t.procedure.query(async ({ ctx }) => ctx.api.getGenres()),
  trackGenres: t.procedure.input(z.object({ id: z.string() })).query(async ({ ctx, input }) => ctx.api.getTrackGenres(input.id)),

  recordPlay: t.procedure
    .input(z.object({ id: z.string(), stationId: z.string().optional() }))
    .mutation(async ({ ctx, input }) => ctx.api.recordPlay(input.id, input.stationId)),

  recordFeedback: t.procedure
    .input(z.object({ id: z.string(), feedback: z.enum(['like', 'skip', 'ban']) }))
    .mutation(async ({ ctx, input }) => ctx.api.recordFeedback(input.id, input.feedback)),

  removeFeedback: t.procedure
    .input(z.object({ id: z.string(), feedback: z.enum(['like', 'skip', 'ban']) }))
    .mutation(async ({ ctx, input }) => ctx.api.deleteFeedback(input.id, input.feedback)),

  history: t.procedure.query(async ({ ctx }) => ctx.api.listHistory()),

  liked: t.procedure.query(async ({ ctx }) => ctx.api.listFeedback('like')),
  skipped: t.procedure.query(async ({ ctx }) => ctx.api.listFeedback('skip')),
  banned: t.procedure.query(async ({ ctx }) => ctx.api.listFeedback('ban')),

  track: t.procedure
    .input(z.object({ id: z.string() }))
    .query(async ({ ctx, input }) => ctx.api.getTrack(input.id)),

  stations: t.procedure.query(async ({ ctx }) => ctx.api.listStations()),

  search: t.procedure
    .input(z.object({ q: z.string() }))
    .query(async ({ ctx, input }) => ctx.api.search(input.q)),

  queue: t.procedure
    .input(z.object({ id: z.string() }))
    .query(async ({ ctx, input }) => ctx.api.getQueue(input.id)),

  station: t.procedure
    .input(z.object({ id: z.string() }))
    .query(async ({ ctx, input }) => ctx.api.getStation(input.id)),

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
        seed_type: stationSeedTypeSchema.optional(),
        track_id: z.string().optional(),
        artist_id: z.string().optional(),
        album_id: z.string().optional(),
        cluster_id: z.number().optional(),
        main_genre: z.string().optional(),
        sub_genre: z.string().optional(),
      })
    )
    .mutation(async ({ ctx, input }) => ctx.api.createStation(input)),

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
        seed_type: stationSeedTypeSchema.optional(),
        track_id: z.string().optional(),
        artist_id: z.string().optional(),
        album_id: z.string().optional(),
        cluster_id: z.number().optional(),
        main_genre: z.string().optional(),
        sub_genre: z.string().optional(),
      })
    )
    .mutation(async ({ ctx, input }) => {
      const { id, ...body } = input;
      return ctx.api.updateStation(id, body);
    }),

  deleteStation: t.procedure
    .input(z.object({ id: z.string() }))
    .mutation(async ({ ctx, input }) => ctx.api.deleteStation(input.id)),

  streamUrl: t.procedure
    .input(
      z.object({
        id: z.string(),
        format: z.enum(['flac', 'mp3']).default('flac'),
      })
    )
    .query(async ({ input }) => {
      return `/api/stream/${encodeURIComponent(input.id)}?format=${input.format}`;
    }),

  setupStatus: t.procedure.query(async ({ ctx }) => ctx.api.setupStatus()),
  setupSummary: t.procedure.query(async ({ ctx }) => ctx.api.setupSummary()),
  setupStations: t.procedure
    .input(z.object({ selectedMainGenres: z.array(z.string()) }))
    .mutation(async ({ ctx, input }) => ctx.api.setupStations(input.selectedMainGenres)),
  setupComplete: t.procedure.mutation(async ({ ctx }) => ctx.api.setupComplete()),

  createInvite: t.procedure
    .input(z.object({ username: z.string().optional(), ttlHours: z.number().optional() }))
    .mutation(async ({ ctx, input }) => ctx.api.createInvite(input.username, input.ttlHours)),

  createUser: t.procedure
    .input(z.object({ username: z.string(), password: z.string() }))
    .mutation(async ({ ctx, input }) => ctx.api.createUser(input.username, input.password)),

  listUsers: t.procedure.query(async ({ ctx }) => ctx.api.listUsers()),

  deleteUser: t.procedure
    .input(z.object({ id: z.string() }))
    .mutation(async ({ ctx, input }) => ctx.api.deleteUser(input.id)),
});

export type AppRouter = typeof appRouter;
export type { Track, Station, QueueTrack } from '@/lib/api';
