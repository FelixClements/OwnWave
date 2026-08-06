'use client';

import { useEffect } from 'react';
import { usePathname, useRouter } from 'next/navigation';
import { useAuth } from '@/lib/auth';

export function RequireAuth({ children }: { children: React.ReactNode }) {
  const { user, loading } = useAuth();
  const router = useRouter();
  const path = usePathname() ?? '';
  const publicPaths = ['/login'];

  useEffect(() => {
    if (!loading && !user && !publicPaths.includes(path)) {
      router.push('/login');
    }
    if (!loading && user && path === '/login') {
      router.push('/');
    }
  }, [loading, user, path, router]);

  if (loading) {
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
