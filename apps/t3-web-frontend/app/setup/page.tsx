'use client';

import { useEffect, useMemo, useState } from 'react';
import { useRouter } from 'next/navigation';
import { useAuth } from '@/lib/auth';
import { trpc } from '@/lib/trpc/client';
import { ScanStatus } from '@/lib/api';

const MIN_TRACKS_PER_STATION = 5;

export default function SetupPage() {
  const router = useRouter();
  const { user, register } = useAuth();

  const [step, setStep] = useState(0);
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [accountError, setAccountError] = useState('');
  const [scanJobId, setScanJobId] = useState<string | null>(null);
  const [selectedGenres, setSelectedGenres] = useState<Set<string>>(new Set());
  const [createStationError, setCreateStationError] = useState('');

  const utils = trpc.useContext();
  const setupStatus = trpc.setupStatus.useQuery();
  const setupSummary = trpc.setupSummary.useQuery(undefined, {
    enabled: step === 3,
  });
  const rescan = trpc.rescan.useMutation();
  const scanStatus = trpc.scanStatus.useQuery(
    { jobId: scanJobId || '' },
    {
      enabled: !!scanJobId,
      refetchInterval: (data) =>
        data && (data.status === 'completed' || data.status === 'failed')
          ? false
          : 1000,
    }
  );
  const setupStations = trpc.setupStations.useMutation({
    onSuccess: () => {
      utils.setupSummary.invalidate();
      setStep(5);
    },
    onError: (err) => {
      setCreateStationError(err.message);
    },
  });
  const setupComplete = trpc.setupComplete.useMutation({
    onSuccess: () => {
      window.location.href = '/';
    },
  });

  useEffect(() => {
    if (setupStatus.data?.setup_completed) {
      router.push('/');
    } else if (setupStatus.data) {
      const { has_users, track_count } = setupStatus.data;
      if (step === 0) {
        if (!has_users) setStep(1);
        else if (track_count === 0) setStep(2);
        else setStep(3);
      }
    }
  }, [setupStatus.data, step, router]);

  useEffect(() => {
    if (scanStatus.data?.status === 'completed') {
      utils.setupStatus.invalidate();
      utils.setupSummary.invalidate();
      setStep(3);
    } else if (scanStatus.data?.status === 'failed') {
      setStep(2);
    }
  }, [scanStatus.data, utils]);

  useEffect(() => {
    if (setupSummary.data && step === 3) {
      const preselected = new Set<string>();
      for (const g of setupSummary.data.main_genres) {
        if (g.track_count >= MIN_TRACKS_PER_STATION) {
          preselected.add(g.main_genre);
        }
      }
      setSelectedGenres(preselected);
    }
  }, [setupSummary.data, step]);

  const handleCreateAccount = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!username.trim() || !password.trim()) return;
    try {
      setAccountError('');
      await register(username.trim(), password.trim());
      utils.setupStatus.invalidate();
      setStep(2);
    } catch (err) {
      setAccountError(err instanceof Error ? err.message : 'Failed to create account');
    }
  };

  const handleStartScan = async () => {
    try {
      const data = await rescan.mutateAsync();
      if (data.job_id) {
        setScanJobId(data.job_id);
        setStep(2);
      }
    } catch (err) {
      console.error('scan failed', err);
    }
  };

  const toggleGenre = (main: string) => {
    setSelectedGenres((prev) => {
      const next = new Set(prev);
      if (next.has(main)) next.delete(main);
      else next.add(main);
      return next;
    });
  };

  const handleCreateStations = async () => {
    if (selectedGenres.size === 0) return;
    setCreateStationError('');
    setupStations.mutate({ selectedMainGenres: Array.from(selectedGenres) });
  };

  const handleComplete = async () => {
    await setupComplete.mutateAsync();
  };

  const failedPaths = (scanStatus.data?.stats?.failed_paths || []) as ScanStatus['stats']['failed_paths'];

  if (setupStatus.isLoading) {
    return <div className="p-6 text-spotify-text">Loading...</div>;
  }

  return (
    <div className="h-screen overflow-y-auto bg-spotify-bg text-spotify-text p-4 md:p-6">
      <div className="max-w-2xl mx-auto space-y-6 pb-20">
        <h1 className="text-2xl font-bold">OwnWave Setup</h1>

        {step === 1 && (
          <div className="p-4 rounded bg-spotify-card space-y-4">
            <h2 className="text-lg font-semibold">Step 1: Create admin account</h2>
            <form onSubmit={handleCreateAccount} className="space-y-3">
              <div>
                <label className="text-xs text-spotify-subdued block mb-1">Username</label>
                <input
                  type="text"
                  value={username}
                  onChange={(e) => setUsername(e.target.value)}
                  className="w-full px-3 py-2 rounded bg-spotify-elevated border border-spotify-border focus:border-spotify-green focus:outline-none"
                />
              </div>
              <div>
                <label className="text-xs text-spotify-subdued block mb-1">Password</label>
                <input
                  type="password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  className="w-full px-3 py-2 rounded bg-spotify-elevated border border-spotify-border focus:border-spotify-green focus:outline-none"
                />
              </div>
              {accountError && <p className="text-red-500 text-sm">{accountError}</p>}
              <button
                type="submit"
                className="px-4 py-2 rounded bg-spotify-green text-black font-semibold hover:bg-spotify-green-hover transition"
              >
                Create account
              </button>
            </form>
          </div>
        )}

        {step === 2 && (
          <div className="p-4 rounded bg-spotify-card space-y-4">
            <h2 className="text-lg font-semibold">Step 2: Scan your library</h2>
            {!scanJobId ? (
              <div className="space-y-3">
                <p className="text-spotify-subdued">
                  OwnWave will scan your music folder for FLAC/MP3 files, extract audio features,
                  and analyse genres. This may take a few minutes.
                </p>
                <button
                  onClick={handleStartScan}
                  disabled={rescan.isLoading}
                  className="px-4 py-2 rounded bg-spotify-green text-black font-semibold hover:bg-spotify-green-hover transition disabled:opacity-50"
                >
                  {rescan.isLoading ? 'Starting...' : 'Start scan'}
                </button>
              </div>
            ) : scanStatus.data ? (
              <div className="space-y-2">
                <p className="text-spotify-subdued">Status: <span className="text-spotify-text capitalize">{scanStatus.data.status}</span></p>
                <div className="grid grid-cols-2 sm:grid-cols-4 gap-2 text-sm">
                  <div className="bg-spotify-elevated p-2 rounded">
                    <div className="text-spotify-subdued text-xs">Found</div>
                    <div className="font-semibold">{scanStatus.data.stats.total_files}</div>
                  </div>
                  <div className="bg-spotify-elevated p-2 rounded">
                    <div className="text-spotify-subdued text-xs">Imported</div>
                    <div className="font-semibold">{scanStatus.data.stats.imported}</div>
                  </div>
                  <div className="bg-spotify-elevated p-2 rounded">
                    <div className="text-spotify-subdued text-xs">Scanned</div>
                    <div className="font-semibold">{scanStatus.data.stats.scanned}</div>
                  </div>
                  <div className="bg-spotify-elevated p-2 rounded">
                    <div className="text-spotify-subdued text-xs">Model OK</div>
                    <div className="font-semibold">{scanStatus.data.stats.model_success}</div>
                  </div>
                </div>
                {scanStatus.data.status === 'failed' && (
                  <p className="text-red-500 text-sm">Scan failed. Please try again.</p>
                )}
              </div>
            ) : (
              <p className="text-spotify-subdued">Waiting for scan status...</p>
            )}
          </div>
        )}

        {step === 3 && setupSummary.data && (
          <div className="p-4 rounded bg-spotify-card space-y-4">
            <h2 className="text-lg font-semibold">Step 3: What we found</h2>
            <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 text-center">
              <div className="bg-spotify-elevated p-3 rounded">
                <div className="text-2xl font-bold">{setupSummary.data.total_tracks}</div>
                <div className="text-xs text-spotify-subdued">Tracks</div>
              </div>
              <div className="bg-spotify-elevated p-3 rounded">
                <div className="text-2xl font-bold">{setupSummary.data.main_genres.length}</div>
                <div className="text-xs text-spotify-subdued">Main genres</div>
              </div>
              <div className="bg-spotify-elevated p-3 rounded">
                <div className="text-2xl font-bold">{setupSummary.data.sub_genres.length}</div>
                <div className="text-xs text-spotify-subdued">Sub-genres</div>
              </div>
              <div className="bg-spotify-elevated p-3 rounded">
                <div className="text-2xl font-bold">{setupSummary.data.uncovered}</div>
                <div className="text-xs text-spotify-subdued">Not in a station</div>
              </div>
            </div>
            {failedPaths.length > 0 && (
              <div>
                <h3 className="font-semibold text-sm mb-2">Failed files</h3>
                <ul className="text-xs text-red-400 max-h-32 overflow-y-auto space-y-1">
                  {failedPaths.map((fp, i) => (
                    <li key={i}>
                      <span className="text-spotify-subdued">{fp.path}</span>: {fp.error}
                    </li>
                  ))}
                </ul>
              </div>
            )}
            <button
              onClick={() => setStep(4)}
              className="px-4 py-2 rounded bg-spotify-green text-black font-semibold hover:bg-spotify-green-hover transition"
            >
              Continue
            </button>
          </div>
        )}

        {step === 4 && setupSummary.data && (
          <div className="p-4 rounded bg-spotify-card space-y-4">
            <h2 className="text-lg font-semibold">Step 4: Create main-genre stations</h2>
            <p className="text-spotify-subdued text-sm">
              Select the main genres you want as stations. Genres with at least {MIN_TRACKS_PER_STATION} tracks are pre-selected.
            </p>
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
              {setupSummary.data.main_genres.map((g) => (
                <label
                  key={g.main_genre}
                  className="flex items-center gap-3 p-3 rounded bg-spotify-elevated cursor-pointer hover:bg-spotify-elevated-hover transition"
                >
                  <input
                    type="checkbox"
                    checked={selectedGenres.has(g.main_genre)}
                    onChange={() => toggleGenre(g.main_genre)}
                    className="w-4 h-4 accent-spotify-green"
                  />
                  <div className="flex-1">
                    <div className="font-semibold">{g.main_genre.replace(/\w\S*/g, (w) => w.charAt(0).toUpperCase() + w.substr(1))}</div>
                    <div className="text-xs text-spotify-subdued">{g.track_count} tracks</div>
                  </div>
                </label>
              ))}
            </div>
            {createStationError && <p className="text-red-500 text-sm">{createStationError}</p>}
            <div className="flex gap-3">
              <button
                onClick={() => setStep(3)}
                className="px-4 py-2 rounded bg-spotify-elevated text-spotify-text font-semibold hover:bg-spotify-elevated-hover transition"
              >
                Back
              </button>
              <button
                onClick={handleCreateStations}
                disabled={selectedGenres.size === 0 || setupStations.isLoading}
                className="px-4 py-2 rounded bg-spotify-green text-black font-semibold hover:bg-spotify-green-hover transition disabled:opacity-50"
              >
                {setupStations.isLoading ? 'Creating...' : 'Create stations'}
              </button>
            </div>
          </div>
        )}

        {step === 5 && (
          <div className="p-4 rounded bg-spotify-card space-y-4 text-center">
            <h2 className="text-lg font-semibold">Setup complete</h2>
            <p className="text-spotify-subdued">
              Your stations have been created. Every track is now in at least one station.
            </p>
            <button
              onClick={handleComplete}
              disabled={setupComplete.isLoading}
              className="px-4 py-2 rounded bg-spotify-green text-black font-semibold hover:bg-spotify-green-hover transition disabled:opacity-50"
            >
              {setupComplete.isLoading ? 'Finishing...' : 'Go to stations'}
            </button>
          </div>
        )}
      </div>
    </div>
  );
}
