import { Suspense } from 'react';
import { StationProvider } from '@/lib/station';
import { AppShell } from '@/components/shell/AppShell';

/** Signed-in pages share the station/playback state and the app shell. */
export default function AppLayout({ children }: { children: React.ReactNode }) {
  return (
    // StationProvider reads ?station= via useSearchParams, which needs a
    // Suspense boundary in a static export.
    <Suspense>
      <StationProvider>
        <AppShell>{children}</AppShell>
      </StationProvider>
    </Suspense>
  );
}
