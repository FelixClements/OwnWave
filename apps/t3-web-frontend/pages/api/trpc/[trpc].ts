import { createNextApiHandler } from '@trpc/server/adapters/next';
import type { NextApiRequest } from 'next';
import { setAuthToken } from '@/lib/api';
import { appRouter } from '@/server/routers/app';

function createContext({ req }: { req: NextApiRequest }) {
  const auth = req.headers.authorization;
  if (auth && auth.startsWith('Bearer ')) {
    setAuthToken(auth.slice(7));
  } else {
    setAuthToken(null);
  }
  return {};
}

export default createNextApiHandler({
  router: appRouter,
  createContext,
  onError: ({ error }) => {
    if (error.code === 'INTERNAL_SERVER_ERROR') {
      console.error('tRPC error:', error);
    }
  },
});
