'use client';

import { useEffect } from 'react';
import { usePathname, useRouter } from 'next/navigation';
import { useAuth } from '@/lib/auth';
import { trpc } from '@/lib/trpc/client';

export function RequireAuth({ children }: { children: React.ReactNode }) {
  const { user, loading } = useAuth();
  const router = useRouter();
  const path = usePathname() ?? '';
  const publicPaths = ['/login', '/setup'];

  const setupStatus = trpc.setupStatus.useQuery();
  const isLoading = loading || setupStatus.isLoading;

  useEffect(() => {
    if (isLoading) return;

    // First run: no admin account exists yet, send everyone to /setup.
    if (!setupStatus.data?.has_users) {
      if (path !== '/setup') {
        router.push('/setup');
      }
      return;
    }

    // Normal post-setup auth rules.
    if (!user && !publicPaths.includes(path)) {
      router.push('/login');
    }
    if (user && path === '/login') {
      router.push('/');
    }
  }, [isLoading, user, path, router, setupStatus.data]);

  if (isLoading) {
    return (
      <div className="min-h-screen bg-spotify-bg flex items-center justify-center">
        <div className="text-spotify-subdued text-sm animate-pulse">Loading...</div>
      </div>
    );
  }

  if (!user && !publicPaths.includes(path)) {
    return null;
  }

  return <>{children}</>;
}
