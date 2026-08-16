'use client';

import { usePathname } from 'next/navigation';
import { StationProvider } from '@/lib/station';
import { Shell } from '@/components/Shell';

export function RootShell({ children }: { children: React.ReactNode }) {
  const path = usePathname() ?? '';
  const publicPaths = ['/login', '/setup'];

  if (publicPaths.includes(path)) {
    return <>{children}</>;
  }

  return (
    <StationProvider>
      <Shell>{children}</Shell>
    </StationProvider>
  );
}
