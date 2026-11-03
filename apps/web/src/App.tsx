import type { ReactNode } from 'react';
import { BrowserRouter, Navigate, Route, Routes, useLocation } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { ApiError } from '@/api/client';
import { AuthProvider, useAuth } from '@/lib/auth';
import { useSetupStatus } from '@/lib/queries';
import { PlayerProvider } from '@/player/PlayerProvider';
import { Shell } from '@/components/Shell';
import { Spinner } from '@/components/ui';
import { AdminPage } from '@/pages/Admin';
import { GenrePage } from '@/pages/Genre';
import { HomePage } from '@/pages/Home';
import { InvitePage } from '@/pages/Invite';
import { LibraryPage } from '@/pages/Library';
import { LoginPage } from '@/pages/Login';
import { NotFoundPage } from '@/pages/NotFound';
import { PlaylistPage } from '@/pages/Playlist';
import { PlaylistsPage } from '@/pages/Playlists';
import { SearchPage } from '@/pages/Search';
import { SettingsPage } from '@/pages/Settings';
import { SetupPage } from '@/pages/Setup';
import { StationPage } from '@/pages/Station';
import { StationsPage } from '@/pages/Stations';

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30_000,
      retry: (n, e) => !(e instanceof ApiError && e.status < 500) && n < 2,
      refetchOnWindowFocus: false,
    },
  },
});

/** Sends visitors to setup (first run) or login; admins stay in setup until it is done. */
function RequireUser({ children }: { children: ReactNode }) {
  const { user, loading } = useAuth();
  const setup = useSetupStatus();
  const location = useLocation();
  if (loading || setup.isLoading) return <Spinner />;
  if (setup.data && !setup.data.has_users) return <Navigate to="/setup" replace />;
  if (!user) return <Navigate to="/login" replace state={{ from: location.pathname }} />;
  if (setup.data && !setup.data.setup_completed && user.is_admin) return <Navigate to="/setup" replace />;
  return <>{children}</>;
}

function RequireAdmin({ children }: { children: ReactNode }) {
  const { user } = useAuth();
  return user?.is_admin ? <>{children}</> : <Navigate to="/" replace />;
}

export function AppRoutes() {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route path="/setup" element={<SetupPage />} />
      <Route path="/invite/:token" element={<InvitePage />} />
      <Route
        element={
          <RequireUser>
            <PlayerProvider>
              <Shell />
            </PlayerProvider>
          </RequireUser>
        }
      >
        <Route index element={<HomePage />} />
        <Route path="search" element={<SearchPage />} />
        <Route path="stations" element={<StationsPage />} />
        <Route path="station/:id" element={<StationPage />} />
        <Route path="genre/:id" element={<GenrePage />} />
        <Route path="playlists" element={<PlaylistsPage />} />
        <Route path="playlist/:id" element={<PlaylistPage />} />
        <Route path="library" element={<LibraryPage />} />
        <Route path="settings" element={<SettingsPage />} />
        <Route
          path="admin"
          element={
            <RequireAdmin>
              <AdminPage />
            </RequireAdmin>
          }
        />
        <Route path="*" element={<NotFoundPage />} />
      </Route>
    </Routes>
  );
}

export function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <AuthProvider>
        <BrowserRouter>
          <AppRoutes />
        </BrowserRouter>
      </AuthProvider>
    </QueryClientProvider>
  );
}
