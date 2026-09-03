import { createNextApiHandler } from '@trpc/server/adapters/next';
import type { NextApiRequest } from 'next';
import { OwnWaveAPI } from '@/lib/api';
import { appRouter } from '@/server/routers/app';

const GO_API_URL = process.env.GO_API_URL || 'http://localhost:8080';

export function createTRPCContext({ req }: { req: NextApiRequest }) {
  const cookie = req.headers.cookie;
  const authorization = req.headers.authorization;
  return {
    api: new OwnWaveAPI(GO_API_URL, {
      cookie,
      authorization,
    }),
  };
}

export default createNextApiHandler({
  router: appRouter,
  createContext: createTRPCContext,
  onError: ({ error }) => {
    if (error.code === 'INTERNAL_SERVER_ERROR') {
      console.error('tRPC error:', error);
    }
  },
});
