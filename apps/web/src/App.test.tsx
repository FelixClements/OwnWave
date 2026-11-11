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
const playlist: Playlist = { id: 'p1', spotify_id: 'sp1', name: 'Techno Bunker', track_count: 100, matched_count: 40, hidden_count: 0, pick_count: 12, has_cover: false };
const station: Station = {
  id: 's1',
  name: 'Bunker Techno',
  shared: true,
  kind: 'custom',
  all_playlists: false,
  playlist_ids: ['p1'],
  genres: ['techno'],
  enhance: false,
  track_count: 40,
  updated_at: '2026-11-03T12:00:00Z',
};
const genreStation: Station = { ...station, id: 'g1', name: 'Techno', kind: 'genre', all_playlists: false, playlist_ids: [], track_count: 30 };
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
    await user.click(within(dialog).getByRole('checkbox', { name: /Techno Bunker/ }));
    await user.click(await within(dialog).findByRole('button', { name: /Techno 30/ }));
    expect(within(dialog).getByText(/Plays up to 30 tracks of 40 tracks/)).toBeInTheDocument();
    await user.click(within(dialog).getByRole('button', { name: 'Create station' }));
    await waitFor(() => expect(created).toEqual({ name: 'Late Techno', all_playlists: false, playlist_ids: ['p1'], genres: ['techno'], enhance: false }));
  });

  it('creates a station from every playlist, including later ones', async () => {
    let created: unknown;
    const counted: string[] = [];
    signedIn({
      'GET /api/stations/genres': (_b, url) => {
        counted.push(url.search);
        return { tracks: 90, genres: [{ id: 'house', name: 'House', tracks: 50 }] };
      },
      'POST /api/stations': (body) => {
        created = body;
        return { ...station, id: 's2', name: 'All House', all_playlists: true };
      },
    });
    renderAt('/stations');
    const user = userEvent.setup();
    await user.click(await within(await screen.findByRole('main')).findByRole('button', { name: /New station/ }));
    const dialog = await screen.findByRole('dialog', { name: 'New station' });
    await user.type(within(dialog).getByLabelText('Name'), 'All House');
    await user.click(within(dialog).getByRole('checkbox', { name: /All playlists/ }));
    expect(within(dialog).queryByRole('checkbox', { name: /Techno Bunker/ })).not.toBeInTheDocument();
    await user.click(await within(dialog).findByRole('button', { name: /House 50/ }));
    expect(counted).toContain('?all_playlists=true');
    await user.click(within(dialog).getByRole('button', { name: 'Create station' }));
    await waitFor(() => expect(created).toEqual({ name: 'All House', all_playlists: true, playlist_ids: [], genres: ['house'], enhance: false }));
  });

  it('shows a genre station that can be renamed but not deleted', async () => {
    let renamed: unknown;
    signedIn({
      'GET /api/stations': () => ({ stations: [station, genreStation] }),
      'GET /api/stations/{id}': () => genreStation,
      'GET /api/stations/{id}/queue': () => ({ queue: [] }),
      'PUT /api/stations/{id}': (body) => {
        renamed = body;
        return { ...genreStation, name: 'Peak Techno' };
      },
    });
    renderAt('/station/g1');
    const user = userEvent.setup();
    expect(await screen.findByText('Genre station')).toBeInTheDocument();
    expect(screen.getByText('from your whole library')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Delete station' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /Enhance/ })).not.toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Rename station' }));
    const dialog = await screen.findByRole('dialog', { name: 'Edit station' });
    expect(within(dialog).queryByRole('checkbox')).not.toBeInTheDocument();
    const name = within(dialog).getByLabelText('Name');
    await user.clear(name);
    await user.type(name, 'Peak Techno');
    await user.click(within(dialog).getByRole('button', { name: 'Save' }));
    await waitFor(() => expect(renamed).toEqual({ name: 'Peak Techno' }));
  });

  it('turns Enhance on for a playlist station', async () => {
    let sent: unknown;
    let current = station;
    signedIn({
      'GET /api/playlists': () => ({ playlists: [playlist] }),
      'GET /api/stations/{id}': () => current,
      'GET /api/stations/{id}/queue': () => ({ queue: [] }),
      'PUT /api/stations/{id}': (body) => {
        sent = body;
        current = { ...station, enhance: true, track_count: 52 };
        return current;
      },
    });
    renderAt('/station/s1');
    const user = userEvent.setup();
    await user.click(await screen.findByRole('button', { name: /^Enhance$/ }));
    await waitFor(() => expect(sent).toEqual({ enhance: true }));
    expect(await screen.findByRole('button', { name: /Enhanced/ })).toHaveAttribute('aria-pressed', 'true');
  });

  it('lists genre stations by kind', async () => {
    signedIn({ 'GET /api/stations': () => ({ stations: [station, genreStation] }) });
    renderAt('/stations');
    const user = userEvent.setup();
    await user.click(await screen.findByRole('tab', { name: /Genres/ }));
    const main = screen.getByRole('main');
    expect(within(main).getByText('Techno')).toBeInTheDocument();
    expect(within(main).queryByText('Bunker Techno')).not.toBeInTheDocument();
  });

  it('runs the genre stations and the pipeline from admin', async () => {
    let settings = {
      enabled: true,
      show_at: 25,
      hide_below: 20,
      genres: [
        { genre: 'techno', genre_name: 'Techno', station_id: 'g1', name: 'Techno', name_is_custom: false, tracks: 30, state: 'active' },
        { genre: 'house', genre_name: 'House', name: 'House', name_is_custom: false, tracks: 12, state: 'below_threshold' },
      ],
    };
    const calls = signedIn({
      'GET /api/admin/health': () => ({ database: 'ok' }),
      'GET /api/admin/scans': () => ({ jobs: [] }),
      'GET /api/admin/users': () => ({ users: [admin] }),
      'GET /api/admin/genre-stations': () => settings,
      'PUT /api/admin/genre-stations': (body) => {
        settings = { ...settings, ...(body as object), genres: settings.genres.map((g) => ({ ...g, state: 'off' })) };
        return settings;
      },
      'PUT /api/admin/genre-stations/{genre}': (body) => {
        settings = { ...settings, genres: settings.genres.map((g) => (g.genre === 'techno' && (body as { excluded?: boolean }).excluded ? { ...g, state: 'excluded' } : g)) };
        return settings;
      },
      'GET /api/admin/pipeline': () => ({
        last_scan: { id: 'scan1', status: 'completed', created_at: '2026-11-03T10:00:00Z', finished_at: '2026-11-03T11:00:00Z' },
        genres_scored: { at: '2026-11-03T11:05:00Z', tracks: 600, scored: 580 },
        runs: [{ id: 2, trigger: 'genres_scored', started_at: '2026-11-03T11:05:00Z', finished_at: '2026-11-03T11:05:01Z', matched: 3, changes: [{ genre: 'techno', name: 'Techno', change: 'created' }] }],
      }),
      'POST /api/admin/pipeline/refresh': () => ({ id: 3, trigger: 'admin', started_at: '2026-11-03T12:00:00Z', changes: [] }),
    });
    renderAt('/admin');
    const user = userEvent.setup();
    expect(await screen.findByText('Needs 13 more tracks')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Techno' })).toHaveAttribute('href', '/station/g1');
    expect(await screen.findByText('3 tracks matched · Techno created')).toBeInTheDocument();
    expect(screen.getByText('Genres scored', { selector: 'span' })).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Hide Techno' }));
    expect(await screen.findByText('Hidden by you')).toBeInTheDocument();
    expect(calls.find((c) => c.method === 'PUT' && c.path === '/api/admin/genre-stations/techno')?.body).toEqual({ excluded: true });

    await user.click(screen.getByRole('checkbox', { name: 'A station for every main genre' }));
    await waitFor(() => expect(calls.find((c) => c.method === 'PUT' && c.path === '/api/admin/genre-stations')?.body).toEqual({ enabled: false }));
    expect(await screen.findAllByText('Off')).toHaveLength(2);

    await user.click(screen.getByRole('button', { name: /Refresh now/ }));
    await waitFor(() => expect(calls.some((c) => c.method === 'POST' && c.path === '/api/admin/pipeline/refresh')).toBe(true));
  });

  it('hides a song on a station and brings it back', async () => {
    let hidden: { track_id: string; title: string; artist?: string }[] = [];
    const song = { id: 't1', title: 'Domino', artist: 'Oxia', liked: false };
    const calls = signedIn({
      'GET /api/stations/{id}': () => genreStation,
      'GET /api/stations/{id}/queue': () => ({ queue: hidden.length ? [] : [song] }),
      'GET /api/stations/{id}/hidden-tracks': () => ({ tracks: hidden }),
      'POST /api/stations/{id}/hidden-tracks': () => {
        hidden = [{ track_id: 't1', title: 'Domino', artist: 'Oxia' }];
        return undefined;
      },
      'DELETE /api/stations/{id}/hidden-tracks/{trackId}': () => {
        hidden = [];
        return undefined;
      },
    });
    renderAt('/station/g1');
    const user = userEvent.setup();
    await user.click(await screen.findByRole('button', { name: 'Not in this station: Domino' }));
    await waitFor(() => expect(calls.find((c) => c.method === 'POST' && c.path === '/api/stations/g1/hidden-tracks')?.body).toEqual({ track_id: 't1' }));
    await user.click(await screen.findByText('Hidden on this station (1)'));
    await user.click(screen.getByRole('button', { name: 'Bring back' }));
    await waitFor(() => expect(calls.some((c) => c.method === 'DELETE' && c.path === '/api/stations/g1/hidden-tracks/t1')).toBe(true));
    await waitFor(() => expect(screen.queryByText('Hidden on this station (1)')).not.toBeInTheDocument());
  });

  it('shows a playlist picks and lets an admin remove one', async () => {
    let picks = [{ track_id: 't9', title: 'Sounds Alike', artist: 'Someone', hits: 3, similarity: 0.9 }];
    const calls = signedIn({
      'GET /api/playlists/{id}': () => ({ playlist, items: [] }),
      'GET /api/playlists/{id}/picks': () => ({ picks }),
      'DELETE /api/admin/playlists/{id}/picks/{trackId}': () => {
        picks = [];
        return undefined;
      },
    });
    renderAt('/playlist/p1');
    const user = userEvent.setup();
    await user.click(await screen.findByRole('tab', { name: /From your library/ }));
    expect(await screen.findByText('Sounds Alike')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: /Remove Sounds Alike/ }));
    await waitFor(() => expect(calls.some((c) => c.method === 'DELETE' && c.path === '/api/admin/playlists/p1/picks/t9')).toBe(true));
    expect(await screen.findByText('No picks yet')).toBeInTheDocument();
  });

  it('lets an admin fix a playlist match and hide a song', async () => {
    const base = { artists: ['Oxia'], first_seen_at: '2026-01-01T00:00:00Z', last_seen_at: '2099-01-01T00:00:00Z', manual: false, hidden: false };
    let items = [
      { ...base, spotify_track_id: 'sp1', title: 'Domino', track_id: 't1', local_title: 'Domino (Wrong Mix)', local_artist: 'Oxia', match_method: 'fuzzy' },
      { ...base, spotify_track_id: 'sp2', title: 'Obsidian', artists: ['Kevin de Vries'] },
    ];
    const calls = signedIn({
      'GET /api/playlists/{id}': () => ({
        playlist: { ...playlist, track_count: items.filter((i) => !i.hidden).length, hidden_count: items.filter((i) => i.hidden).length },
        items,
      }),
      'GET /api/playlists/{id}/picks': () => ({ picks: [] }),
      'GET /api/tracks': () => ({ tracks: [{ id: 't7', title: 'Obsidian (Extended Mix)', artist: 'Kevin de Vries' }] }),
      'PUT /api/admin/playlists/{id}/items/{spotifyTrackId}/match': (body, url) => {
        const sp = url.pathname.split('/')[6];
        const trackId = (body as { track_id: string | null }).track_id;
        items = items.map((i) =>
          i.spotify_track_id === sp
            ? { ...i, manual: true, track_id: trackId ?? undefined, local_title: trackId ? 'Obsidian (Extended Mix)' : undefined, match_method: trackId ? 'manual' : undefined }
            : i,
        );
        return undefined;
      },
      'PUT /api/admin/playlists/{id}/items/{spotifyTrackId}/hidden': (body, url) => {
        const sp = url.pathname.split('/')[6];
        items = items.map((i) => (i.spotify_track_id === sp ? { ...i, hidden: (body as { hidden: boolean }).hidden } : i));
        return undefined;
      },
    });
    renderAt('/playlist/p1');
    const user = userEvent.setup();

    // A wrong match is marked as not in the library.
    await user.click(await screen.findByRole('button', { name: /Wrong match: mark Domino/ }));
    await waitFor(() =>
      expect(calls.find((c) => c.method === 'PUT' && c.path === '/api/admin/playlists/p1/items/sp1/match')?.body).toEqual({ track_id: null }),
    );

    // A missing song gets its file picked by hand.
    await user.click(screen.getByRole('tab', { name: /Missing/ }));
    expect(await screen.findByText('Marked missing')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Pick the file for Obsidian' }));
    const dialog = await screen.findByRole('dialog', { name: 'Pick the local file' });
    await user.click(await within(dialog).findByText('Obsidian (Extended Mix)'));
    await waitFor(() =>
      expect(calls.find((c) => c.method === 'PUT' && c.path === '/api/admin/playlists/p1/items/sp2/match')?.body).toEqual({ track_id: 't7' }),
    );
    expect(calls.some((c) => c.path === '/api/tracks')).toBe(true);

    // Hide it, then bring it back from the Hidden tab.
    await user.click(screen.getByRole('tab', { name: /In your library/ }));
    await user.click(await screen.findByRole('button', { name: /Hide Obsidian/ }));
    await waitFor(() =>
      expect(calls.find((c) => c.method === 'PUT' && c.path === '/api/admin/playlists/p1/items/sp2/hidden')?.body).toEqual({ hidden: true }),
    );
    await user.click(screen.getByRole('tab', { name: /Hidden/ }));
    await user.click(await screen.findByRole('button', { name: 'Show Obsidian again' }));
    await waitFor(() => expect(calls.filter((c) => c.path === '/api/admin/playlists/p1/items/sp2/hidden').at(-1)?.body).toEqual({ hidden: false }));
  });

  it('reviews and corrects genres from admin', async () => {
    let corrections: unknown[] = [];
    const calls = signedIn({
      'GET /api/admin/health': () => ({ database: 'ok' }),
      'GET /api/admin/scans': () => ({ jobs: [] }),
      'GET /api/admin/users': () => ({ users: [admin] }),
      'GET /api/admin/genre-stations': () => ({ enabled: true, show_at: 25, hide_below: 20, genres: [] }),
      'GET /api/admin/pipeline': () => ({ runs: [] }),
      'GET /api/admin/genre-review': () => ({
        items: [{ track_id: 't1', title: 'Domino', artist: 'Oxia', genre: 'techno', score: 0.3, hidden_by: 2, copies_disagree: true }],
      }),
      'GET /api/admin/genre-corrections': () => ({ corrections }),
      'POST /api/admin/genre-corrections': (body) => {
        corrections = [{ id: 'c1', artist: 'Oxia', genre: (body as { genre: string }).genre, created_at: '2026-11-05T00:00:00Z' }];
        return { corrections };
      },
    });
    renderAt('/admin');
    const user = userEvent.setup();
    expect(await screen.findByText('Hidden by 2 listeners')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Looks right' })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Set for this artist' })).not.toBeInTheDocument();
    await user.selectOptions(await screen.findByRole('combobox', { name: 'Genre of Domino' }), 'house');
    await user.click(screen.getByRole('button', { name: 'Set for this artist' }));
    await waitFor(() => expect(calls.find((c) => c.method === 'POST' && c.path === '/api/admin/genre-corrections')?.body).toEqual({ artist: 'Oxia', genre: 'house' }));
    expect(await screen.findByText('Everything by Oxia')).toBeInTheDocument();
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

  it('shows a playlist Spotify cover once it is downloaded', async () => {
    signedIn({ 'GET /api/playlists': () => ({ playlists: [playlist, { ...playlist, id: 'p2', name: 'Deep House', has_cover: true }] }) });
    const { container } = renderAt('/playlists');
    expect(await screen.findByText('Deep House')).toBeInTheDocument();
    const covers = [...container.querySelectorAll('img')].map((img) => img.getAttribute('src'));
    expect(covers).toContain('/api/playlists/p2/cover');
    expect(covers).not.toContain('/api/playlists/p1/cover');
  });

  it('says how much of a station the preview shows', async () => {
    const queue = Array.from({ length: 120 }, (_, i) => ({ id: `t${i}`, title: `Song ${i}`, position: i, liked: false }));
    signedIn({
      'GET /api/stations/{id}': () => ({ ...station, track_count: 257 }),
      'GET /api/stations/{id}/queue': () => ({ queue }),
    });
    renderAt('/station/s1');
    expect(await screen.findByText(/showing 100 of 257 tracks/)).toBeInTheDocument();
    expect(screen.getByText('Song 99')).toBeInTheDocument();
    expect(screen.queryByText('Song 100')).not.toBeInTheDocument();
  });

  it('follows a running scan on the setup page instead of starting another', async () => {
    const running = {
      id: 'scan1', path: '/music', status: 'running', folders: { total: 10, done: 4, failed: 0 },
      stats: { total_files: 0, imported: 600, scanned: 0, features: 0, model_success: 0, model_failed: 0, failed_files: 0, failed_paths: [] },
    };
    const calls = fakeApi({
      'GET /api/setup/status': () => ({ setup_completed: false, has_users: true, track_count: 600 }),
      'GET /api/me': () => admin,
      'GET /api/admin/scans': () => ({ jobs: [running] }),
      'GET /api/admin/scan/{id}': () => running,
    });
    renderAt('/');
    expect(await screen.findByText('Importing folders: 4 of 10')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /Scan my library|Scan for changes/ })).not.toBeInTheDocument();
    expect(calls.some((c) => c.method === 'POST' && c.path === '/api/admin/scan')).toBe(false);
  });

  it('keeps non-admins out of admin', async () => {
    signedIn({ 'GET /api/me': () => ({ ...admin, is_admin: false }) });
    renderAt('/admin');
    expect(await screen.findByText(/Good (morning|afternoon|evening)/)).toBeInTheDocument();
  });
});
