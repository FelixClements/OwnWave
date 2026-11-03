import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';
import { AuthProvider } from '@/lib/auth';
import { AppRoutes } from './App';
import type { Playlist, Station } from '@/api/types';

type Handler = (body: unknown, url: URL) => unknown;

/** A fake OwnWave API: "METHOD /path" -> handler (status 401 via a thrown number). */
function fakeApi(routes: Record<string, Handler>) {
  const calls: { method: string; path: string; body: unknown }[] = [];
  vi.stubGlobal(
    'fetch',
    vi.fn(async (input: Request | string, init?: RequestInit) => {
      const req = input instanceof Request ? input : new Request(new URL(input, 'http://localhost'), init);
      const url = new URL(req.url);
      const text = req.method === 'GET' ? '' : await req.text();
      const body = text ? JSON.parse(text) : undefined;
      calls.push({ method: req.method, path: url.pathname, body });
      const key = `${req.method} ${url.pathname}`;
      const handler = routes[key] ?? Object.entries(routes).find(([k]) => new RegExp(`^${k.replace(/\{[^}]+\}/g, '[^/]+')}$`).test(key))?.[1];
      if (!handler) return new Response('not found', { status: 404 });
      try {
        const data = handler(body, url);
        return data === undefined ? new Response(null, { status: 204 }) : Response.json(data);
      } catch (status) {
        return new Response('unauthorized', { status: status as number });
      }
    }),
  );
  return calls;
}

function renderAt(path: string) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <AuthProvider>
        <MemoryRouter initialEntries={[path]}>
          <AppRoutes />
        </MemoryRouter>
      </AuthProvider>
    </QueryClientProvider>,
  );
}

const admin = { id: 'u1', username: 'felix', is_admin: true };
const playlist: Playlist = { id: 'p1', spotify_id: 'sp1', name: 'Techno Bunker', track_count: 100, matched_count: 40 };
const station: Station = { id: 's1', name: 'Bunker Techno', shared: true, playlist_ids: ['p1'], genres: ['techno'], track_count: 40, updated_at: '2026-11-03T12:00:00Z' };
const genres = {
  scored_tracks: 40,
  warnings: [],
  genres: [
    { genre: 'techno', name: 'Techno', primary_tracks: 30, tracks: 35, station: true },
    { genre: 'house', name: 'House', primary_tracks: 10, tracks: 12, station: true },
  ],
};

const signedIn = (extra: Record<string, Handler> = {}) =>
  fakeApi({
    'GET /api/setup/status': () => ({ setup_completed: true, has_users: true }),
    'GET /api/me': () => admin,
    'GET /api/stations': () => ({ stations: [station] }),
    'GET /api/playlists': () => ({ playlists: [playlist] }),
    'GET /api/genres': () => genres,
    'GET /api/browse': () => ({ id: 'recent', title: 'Recently played', items: [] }),
    'GET /api/admin/station-suggestions': () => ({ suggestions: [], min_tracks: 25 }),
    ...extra,
  });

describe('App', () => {
  beforeEach(() => {
    localStorage.clear();
  });
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('sends a fresh install to setup', async () => {
    fakeApi({
      'GET /api/setup/status': () => ({ setup_completed: false, has_users: false }),
      'GET /api/me': () => {
        throw 401;
      },
    });
    renderAt('/');
    expect(await screen.findByRole('heading', { name: 'Set up OwnWave' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Create admin account' })).toBeInTheDocument();
  });

  it('sends a signed-out visitor to login and logs in', async () => {
    let signed = false;
    const calls = fakeApi({
      'GET /api/setup/status': () => ({ setup_completed: true, has_users: true }),
      'GET /api/me': () => {
        if (!signed) throw 401;
        return admin;
      },
      'POST /api/login': () => {
        signed = true;
        return { user: admin };
      },
      'GET /api/stations': () => ({ stations: [] }),
      'GET /api/browse': () => ({ id: 'recent', title: '', items: [] }),
      'GET /api/admin/station-suggestions': () => ({ suggestions: [], min_tracks: 25 }),
      'GET /api/playlists': () => ({ playlists: [] }),
      'GET /api/genres': () => genres,
    });
    renderAt('/');
    const user = userEvent.setup();
    await user.type(await screen.findByLabelText('Username'), 'felix');
    await user.type(screen.getByLabelText('Password'), 'secret-password');
    await user.click(screen.getByRole('button', { name: 'Log in' }));
    expect(await screen.findByText('No stations yet')).toBeInTheDocument();
    expect(calls.find((c) => c.path === '/api/login')?.body).toEqual({ username: 'felix', password: 'secret-password' });
  });

  it('shows stations on home and in the sidebar', async () => {
    signedIn();
    renderAt('/');
    const sidebar = await screen.findByRole('complementary', { name: 'Sidebar' });
    expect(await within(sidebar).findByText('Bunker Techno')).toBeInTheDocument();
    expect(screen.getAllByRole('button', { name: 'Play Bunker Techno' }).length).toBeGreaterThan(0);
  });

  it('creates a station from playlists and genre chips', async () => {
    let created: unknown;
    signedIn({
      'GET /api/stations/genres': () => ({ tracks: 40, genres: [{ id: 'techno', name: 'Techno', tracks: 30 }, { id: 'house', name: 'House', tracks: 10 }] }),
      'POST /api/stations': (body) => {
        created = body;
        return { ...station, id: 's2', name: 'Late Techno' };
      },
      'GET /api/stations/{id}': () => ({ ...station, id: 's2', name: 'Late Techno' }),
      'GET /api/stations/{id}/queue': () => ({ queue: [] }),
    });
    renderAt('/stations');
    const user = userEvent.setup();
    const main = await screen.findByRole('main');
    await user.click(await within(main).findByRole('button', { name: /New station/ }));
    const dialog = await screen.findByRole('dialog', { name: 'New station' });
    await user.type(within(dialog).getByLabelText('Name'), 'Late Techno');
    await user.click(within(dialog).getByRole('checkbox'));
    await user.click(await within(dialog).findByRole('button', { name: /Techno 30/ }));
    expect(within(dialog).getByText(/Plays up to 30 tracks of 40 matched tracks/)).toBeInTheDocument();
    await user.click(within(dialog).getByRole('button', { name: 'Create station' }));
    await waitFor(() => expect(created).toEqual({ name: 'Late Techno', playlist_ids: ['p1'], genres: ['techno'] }));
  });

  it('lists the missing songs of a playlist', async () => {
    signedIn({
      'GET /api/playlists/{id}': () => ({
        playlist,
        items: [
          { spotify_track_id: 'a', title: 'Hymn', artists: ['Charlotte de Witte'], first_seen_at: '2026-10-01T00:00:00Z', last_seen_at: '2026-11-03T00:00:00Z', track_id: 't1', local_title: 'Hymn', local_artist: 'Charlotte de Witte', match_method: 'exact' },
          { spotify_track_id: 'b', title: 'drift away', artists: ['Amelie Lens'], first_seen_at: '2026-10-01T00:00:00Z', last_seen_at: '2026-11-03T00:00:00Z' },
        ],
      }),
    });
    renderAt('/playlist/p1');
    expect(await screen.findByRole('heading', { name: 'Techno Bunker' })).toBeInTheDocument();
    expect(screen.getAllByText('Hymn')).toHaveLength(2); // Spotify title and the local file
    expect(screen.queryByText('drift away')).not.toBeInTheDocument();
    await userEvent.setup().click(screen.getByRole('tab', { name: /Missing/ }));
    expect(screen.getByText('drift away')).toBeInTheDocument();
    expect(screen.getByText('Not in your library')).toBeInTheDocument();
    expect(screen.queryAllByText('Hymn')).toHaveLength(0);
  });

  it('keeps non-admins out of admin', async () => {
    signedIn({ 'GET /api/me': () => ({ ...admin, is_admin: false }) });
    renderAt('/admin');
    expect(await screen.findByText(/Good (morning|afternoon|evening)/)).toBeInTheDocument();
  });
});
