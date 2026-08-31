'use client';

import { useEffect } from 'react';
import { usePathname, useRouter } from 'next/navigation';
import { useAuth } from '@/lib/auth';
import { trpc } from '@/lib/trpc/client';

function isPublicPath(path: string) {
  return path === '/login' || path === '/setup' || path.startsWith('/invite/');
}

export function RequireAuth({ children }: { children: React.ReactNode }) {
  const { user, loading } = useAuth();
  const router = useRouter();
  const path = usePathname() ?? '';

  const setupStatus = trpc.setupStatus.useQuery();
  const isLoading = loading || setupStatus.isLoading;

  useEffect(() => {
    if (isLoading) return;

    if (!setupStatus.data?.has_users) {
      if (path !== '/setup') {
        router.push('/setup');
      }
      return;
    }

    if (!user && !isPublicPath(path)) {
      router.push('/login');
      return;
    }
    if (user && path === '/login') {
      router.push('/');
      return;
    }
    if (path === '/admin' && user && !user.is_admin) {
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

  if (!user && !isPublicPath(path)) {
    return null;
  }

  if (path === '/admin' && user && !user.is_admin) {
    return null;
  }

  return <>{children}</>;
}
