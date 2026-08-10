import type { Metadata, Viewport } from 'next';
import './globals.css';
import { Provider } from '@/lib/trpc/Provider';
import { PWA } from '@/components/PWA';
import { AuthProvider } from '@/lib/auth';
import { RequireAuth } from '@/components/RequireAuth';
import { StationProvider } from '@/lib/station';
import { Shell } from '@/components/Shell';

export const metadata: Metadata = {
  title: 'OwnWave',
  description: 'AI smart radio for your local FLAC library',
  manifest: '/manifest.json',
  icons: {
    icon: '/ownwave-192.svg',
    apple: '/ownwave-192.svg',
  },
};

export const viewport: Viewport = {
  width: 'device-width',
  initialScale: 1,
  themeColor: '#1db954',
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="en">
      <body className="min-h-screen overflow-hidden antialiased">
        <Provider>
          <AuthProvider>
            <RequireAuth>
              <StationProvider>
                <Shell>{children}</Shell>
              </StationProvider>
            </RequireAuth>
          </AuthProvider>
        </Provider>
        <PWA />
      </body>
    </html>
  );
}
