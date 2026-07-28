import type { Metadata } from 'next';
import './globals.css';
import { Provider } from '@/lib/trpc/Provider';

export const metadata: Metadata = {
  title: 'OwnWave',
  description: 'AI smart radio for your local FLAC library',
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="en">
      <body className="antialiased">
        <Provider>{children}</Provider>
      </body>
    </html>
  );
}
