import type { Metadata, Viewport } from 'next';
import './globals.css';
import { Provider } from '@/lib/trpc/Provider';

export const metadata: Metadata = {
  title: 'OwnWave',
  description: 'AI smart radio for your local FLAC library',
};

export const viewport: Viewport = {
  width: 'device-width',
  initialScale: 1,
  maximumScale: 1,
  userScalable: false,
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="en">
      <body className="h-full overflow-hidden antialiased">
        <Provider>{children}</Provider>
      </body>
    </html>
  );
}
