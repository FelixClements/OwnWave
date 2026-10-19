'use client';

import { useEffect, useMemo, useState } from 'react';
import { useRouter } from 'next/navigation';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useAuth } from '@/lib/auth';
import { api } from '@/lib/api';
import { isScanFinished, keys, useScanStatus, useScans, useSetupStatus } from '@/lib/queries';
import { ScanProgress } from '@/components/scan/ScanProgress';
import { Button } from '@/components/ui/Button';
import { Field, Input } from '@/components/ui/Field';
import { Panel } from '@/components/ui/Layout';
import { titleCase } from '@/lib/format';

const MIN_TRACKS_PER_STATION = 5;
const GENRES_PER_PAGE = 10;
const STEPS = ['Account', 'Scan', 'Review', 'Stations', 'Done'];

function Steps({ step }: { step: number }) {
  return (
    <ol className="flex flex-wrap gap-x-4 gap-y-1 mb-4 text-[12px]" aria-label="Setup progress">
      {STEPS.map((label, i) => {
        const n = i + 1;
        const state = n < step ? 'done' : n === step ? 'current' : 'todo';
        return (
          <li key={label} aria-current={state === 'current' ? 'step' : undefined} className="flex items-center gap-1.5">
            <span
              className={`grid place-items-center w-5 h-5 rounded-full text-[11px] font-semibold tabular-nums ${
                state === 'current' ? 'bg-accent text-accent-fg' : state === 'done' ? 'bg-accent/20 text-accent' : 'bg-surface-2 text-muted'
              }`}
            >
              {n}
            </span>
            <span className={state === 'todo' ? 'text-muted' : ''}>{label}</span>
          </li>
        );
      })}
    </ol>
  );
}

function Stat({ value, label }: { value: number; label: string }) {
  return (
    <div className="rounded-md border border-line bg-surface-2 p-3 text-center">
      <dd className="text-xl font-bold tabular-nums">{value.toLocaleString()}</dd>
      <dt className="text-[11.5px] text-muted">{label}</dt>
    </div>
  );
}

export default function SetupPage() {
  const router = useRouter();
  const { user, loading: authLoading, register } = useAuth();

  const [step, setStep] = useState(0);
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [accountError, setAccountError] = useState('');
  const [scanJobId, setScanJobId] = useState<string | null>(null);
  const [selectedGenres, setSelectedGenres] = useState<Set<string>>(new Set());
  const [createStationError, setCreateStationError] = useState('');
  const [genrePage, setGenrePage] = useState(0);

  const qc = useQueryClient();
  const setupStatus = useSetupStatus();
  const isAdmin = !!user?.is_admin;
  const scans = useScans(isAdmin);
  const setupSummary = useQuery(keys.setupSummary, () => api.setupSummary(), { enabled: step === 3 });
  const rescan = useMutation(() => api.triggerScan());
  const scanStatus = useScanStatus(scanJobId);
  const setupStations = useMutation((genres: string[]) => api.setupStations(genres), {
    onSuccess: () => {
      void qc.invalidateQueries(keys.setupSummary);
      void qc.invalidateQueries(keys.stations);
      setStep(5);
    },
    onError: (err: Error) => setCreateStationError(err.message),
  });
  const setupComplete = useMutation(() => api.setupComplete(), {
    onSuccess: () => {
      window.location.href = '/';
    },
  });

  // Pick the starting step. A scan still running (e.g. after a page reload)
  // is resumed so its live progress stays visible.
  useEffect(() => {
    if (setupStatus.data?.setup_completed) {
      router.push('/');
      return;
    }
    if (!setupStatus.data || step !== 0) return;
    const { has_users, track_count } = setupStatus.data;
    if (!has_users) {
      setStep(1);
      return;
    }
    if (authLoading || (isAdmin && scans.isLoading)) return;
    const running = scans.data?.find((j) => !isScanFinished(j.status));
    if (running) {
      setScanJobId(running.id);
      setStep(2);
    } else {
      setStep(track_count === 0 ? 2 : 3);
    }
  }, [setupStatus.data, step, router, authLoading, isAdmin, scans.isLoading, scans.data]);

  useEffect(() => {
    if (scanStatus.data?.status === 'completed') {
      void qc.invalidateQueries(keys.setupStatus);
      void qc.invalidateQueries(keys.setupSummary);
      setStep(3);
    } else if (scanStatus.data?.status === 'failed') {
      setStep(2);
    }
  }, [scanStatus.data, qc]);

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
    if (!username.trim() || !password) return;
    try {
      setAccountError('');
      await register(username.trim(), password);
      void qc.invalidateQueries(keys.setupStatus);
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
    setupStations.mutate(Array.from(selectedGenres));
  };

  const handleComplete = async () => {
    await setupComplete.mutateAsync();
  };

  const failedPaths = scanStatus.data?.stats.failed_paths ?? [];

  const mainGenres = setupSummary.data?.main_genres ?? [];
  const genrePageCount = Math.max(1, Math.ceil(mainGenres.length / GENRES_PER_PAGE));
  const currentGenrePage = Math.min(genrePage, genrePageCount - 1);
  const pageGenres = mainGenres.slice(
    currentGenrePage * GENRES_PER_PAGE,
    (currentGenrePage + 1) * GENRES_PER_PAGE,
  );

  if (setupStatus.isLoading) {
    return <p className="text-muted">Loading…</p>;
  }

  return (
    <div className="space-y-4">
      <h1 className="text-xl font-bold">Set up OwnWave</h1>
      {step > 0 && <Steps step={step} />}

      {step === 1 && (
        <Panel title="Create the admin account">
          <form onSubmit={handleCreateAccount} className="flex flex-col gap-3 max-w-sm">
            <Field label="Username">
              {(p) => <Input {...p} value={username} onChange={(e) => setUsername(e.target.value)} autoComplete="username" required autoFocus />}
            </Field>
            <Field label="Password" hint="At least 8 characters." error={accountError || null}>
              {(p) => (
                <Input
                  {...p}
                  type="password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  minLength={8}
                  autoComplete="new-password"
                  required
                />
              )}
            </Field>
            <div>
              <Button type="submit" variant="primary">
                Create account
              </Button>
            </div>
          </form>
        </Panel>
      )}

      {step === 2 && (
        <Panel title="Scan your library">
          {!scanJobId ? (
            <div className="space-y-3">
              <p className="text-muted">
                OwnWave will scan your music folder for FLAC/MP3 files, extract audio features, and analyse genres. This
                may take a few minutes.
              </p>
              <Button variant="primary" onClick={handleStartScan} loading={rescan.isLoading}>
                Start scan
              </Button>
            </div>
          ) : scanStatus.data ? (
            <div className="space-y-2">
              <ScanProgress scan={scanStatus.data} />
              {scanStatus.data.status === 'failed' && (
                <p role="alert" className="text-danger">
                  Scan failed. Please try again.
                </p>
              )}
            </div>
          ) : (
            <p className="text-muted">Waiting for scan status…</p>
          )}
        </Panel>
      )}

      {step === 3 && setupSummary.data && (
        <Panel title="What we found">
          <dl className="grid grid-cols-2 sm:grid-cols-4 gap-2 mb-3">
            <Stat value={setupSummary.data.total_tracks} label="Tracks" />
            <Stat value={setupSummary.data.main_genres.length} label="Main genres" />
            <Stat value={setupSummary.data.sub_genres.length} label="Sub-genres" />
            <Stat value={setupSummary.data.uncovered} label="Not in a station" />
          </dl>
          {failedPaths.length > 0 && (
            <details className="mb-3">
              <summary className="cursor-pointer font-semibold text-accent">
                {failedPaths.length} failed file{failedPaths.length === 1 ? '' : 's'}
              </summary>
              <ul className="mt-2 max-h-32 overflow-y-auto space-y-1 text-[11.5px]">
                {failedPaths.map((fp, i) => (
                  <li key={i} className="break-all">
                    <span className="text-muted">{fp.path}</span>: <span className="text-danger">{fp.error}</span>
                  </li>
                ))}
              </ul>
            </details>
          )}
          <Button variant="primary" onClick={() => setStep(4)}>
            Continue
          </Button>
        </Panel>
      )}

      {step === 4 && setupSummary.data && (
        <Panel title="Create main-genre stations">
          <p className="text-muted mb-3">
            Select the main genres you want as stations. Genres with at least {MIN_TRACKS_PER_STATION} tracks are
            pre-selected.
          </p>
          <fieldset>
            <legend className="sr-only">Main genres</legend>
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-1.5">
              {pageGenres.map((g) => (
                <label
                  key={g.main_genre}
                  className="flex items-center gap-2.5 px-2.5 py-2 rounded-md border border-line bg-surface-2 cursor-pointer hover:bg-hover"
                >
                  <input
                    type="checkbox"
                    checked={selectedGenres.has(g.main_genre)}
                    onChange={() => toggleGenre(g.main_genre)}
                    className="w-4 h-4 accent-accent"
                  />
                  <span className="flex-1 min-w-0 truncate font-medium">{titleCase(g.main_genre)}</span>
                  <span className="text-muted tabular-nums text-[11.5px]">{g.track_count} tracks</span>
                </label>
              ))}
            </div>
          </fieldset>
          {genrePageCount > 1 && (
            <div className="flex items-center justify-between gap-3 mt-3">
              <Button size="sm" onClick={() => setGenrePage(currentGenrePage - 1)} disabled={currentGenrePage === 0}>
                Prev
              </Button>
              <span className="text-muted tabular-nums" aria-live="polite">
                Page {currentGenrePage + 1} of {genrePageCount} · {selectedGenres.size} selected
              </span>
              <Button
                size="sm"
                onClick={() => setGenrePage(currentGenrePage + 1)}
                disabled={currentGenrePage >= genrePageCount - 1}
              >
                Next
              </Button>
            </div>
          )}
          {createStationError && (
            <p role="alert" className="text-danger mt-3">
              {createStationError}
            </p>
          )}
          <div className="flex gap-2 mt-4">
            <Button onClick={() => setStep(3)}>Back</Button>
            <Button
              variant="primary"
              onClick={handleCreateStations}
              loading={setupStations.isLoading}
              disabled={selectedGenres.size === 0}
            >
              Create stations
            </Button>
          </div>
        </Panel>
      )}

      {step === 5 && (
        <Panel title="Setup complete">
          <p className="text-muted mb-3">Your stations are ready. They update automatically as your library changes.</p>
          <Button variant="primary" onClick={handleComplete} loading={setupComplete.isLoading}>
            Go to stations
          </Button>
        </Panel>
      )}
    </div>
  );
}
