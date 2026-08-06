import type { Metadata, Viewport } from 'next';
import './globals.css';
import { Provider } from '@/lib/trpc/Provider';
import { PWA } from '@/components/PWA';
import { AuthProvider } from '@/lib/auth';
import { RequireAuth } from '@/components/RequireAuth';

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
  maximumScale: 1,
  userScalable: false,
  themeColor: '#1db954',
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="en">
      <body className="h-full overflow-hidden antialiased">
        <Provider>
          <AuthProvider>
            <RequireAuth>{children}</RequireAuth>
          </AuthProvider>
        </Provider>
        <PWA />
      </body>
    </html>
  );
}
