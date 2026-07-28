import { createTRPCReact } from '@trpc/react-query';
import { AppRouter } from '@/server/routers/app';

export const trpc = createTRPCReact<AppRouter>();
