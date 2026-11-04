import { useState } from 'react';
import { Navigate, useNavigate } from 'react-router-dom';
import { useQueryClient } from '@tanstack/react-query';
import { Check } from 'lucide-react';
import clsx from 'clsx';
import { api, call } from '@/api/client';
import { useAuth } from '@/lib/auth';
import { plural } from '@/lib/format';
import { isScanFinished, keys, usePlaylistActions, useScan, useScans, useSetupStatus, useStartScan } from '@/lib/queries';
import { scanPhase } from '@/lib/scan';
import { AuthLayout } from '@/components/Shell';
import { Button, ErrorText, Input, Spinner } from '@/components/ui';

const STEPS = ['Admin account', 'Scan library', 'Add a playlist'];

/** First run: create the admin, scan the music folder, add a Spotify playlist. */
export function SetupPage() {
  const { user } = useAuth();
  const status = useSetupStatus();
  if (status.isLoading) return <Spinner />;
  if (status.data?.setup_completed) return <Navigate to="/" replace />;
  if (status.data?.has_users && !user) return <Navigate to="/login" replace />;
  if (user && !user.is_admin) return <Navigate to="/" replace />;
  const step = user ? 1 : 0;
  return <SetupSteps initial={step} />;
}

function Steps({ current }: { current: number }) {
  return (
    <ol className="mb-8 flex items-center justify-center gap-2 text-[12px]">
      {STEPS.map((s, i) => (
        <li key={s} className="flex items-center gap-2">
          <span
            className={clsx(
              'flex h-6 w-6 items-center justify-center rounded-full font-bold',
              i < current ? 'bg-accent text-black' : i === current ? 'bg-ink text-black' : 'bg-raised text-muted',
            )}
          >
            {i < current ? <Check className="h-3.5 w-3.5" /> : i + 1}
          </span>
          <span className={clsx('hidden sm:inline', i === current ? 'text-ink' : 'text-muted')}>{s}</span>
          {i < STEPS.length - 1 && <span className="mx-1 h-px w-6 bg-line" />}
        </li>
      ))}
    </ol>
  );
}

function SetupSteps({ initial }: { initial: number }) {
  const [step, setStep] = useState(initial);
  return (
    <AuthLayout title="Set up OwnWave" subtitle="Three quick steps and the radio is on.">
      <Steps current={step} />
      {step === 0 && <AccountStep onDone={() => setStep(1)} />}
      {step === 1 && <ScanStep onDone={() => setStep(2)} />}
      {step === 2 && <PlaylistStep />}
    </AuthLayout>
  );
}

function AccountStep({ onDone }: { onDone: () => void }) {
  const { register } = useAuth();
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState<unknown>(null);
  const [busy, setBusy] = useState(false);
  return (
    <form
      className="space-y-4"
      onSubmit={async (e) => {
        e.preventDefault();
        setBusy(true);
        setError(null);
        try {
          await register(username.trim(), password);
          onDone();
        } catch (err) {
          setError(err);
        } finally {
          setBusy(false);
        }
      }}
    >
      <p className="text-sm text-muted">The first account is the admin: it adds playlists and manages users.</p>
      <Input label="Username" autoComplete="username" value={username} onChange={(e) => setUsername(e.target.value)} required autoFocus />
      <Input label="Password" type="password" autoComplete="new-password" minLength={8} hint="At least 8 characters." value={password} onChange={(e) => setPassword(e.target.value)} required />
      <ErrorText error={error} />
      <Button type="submit" variant="primary" className="w-full" loading={busy}>
        Create admin account
      </Button>
    </form>
  );
}

function ScanStep({ onDone }: { onDone: () => void }) {
  const start = useStartScan();
  const status = useSetupStatus();
  const scans = useScans();
  const active = scans.data?.find((s) => !isScanFinished(s.status));
  const [started, setStarted] = useState<string | null>(null);
  // A scan already running (e.g. after a reload) is followed, never repeated.
  const jobId = started ?? active?.id ?? null;
  const scan = useScan(jobId);
  const tracks = status.data?.track_count ?? 0;
  const phase = scan.data ? scanPhase(scan.data) : null;
  const done = scan.data?.status === 'completed' || scan.data?.status === 'failed';
  return (
    <div className="space-y-5">
      <p className="text-sm text-muted">
        OwnWave reads your music folder, imports tags and analyses every track's genre. A large library takes a few hours; you can
        continue while it runs.
      </p>
      {!jobId && tracks > 0 && (
        <p className="rounded-md bg-card p-3 text-sm">
          Your library already has {plural(tracks, 'track')}. Scan again only to pick up changes.
        </p>
      )}
      {!jobId ? (
        <div className="space-y-2">
          <Button
            variant={tracks > 0 ? 'secondary' : 'primary'}
            className="w-full"
            loading={start.isPending || scans.isLoading}
            onClick={() => start.mutate(false, { onSuccess: (r) => setStarted(r.job_id) })}
          >
            {tracks > 0 ? 'Scan for changes' : 'Scan my library'}
          </Button>
          {tracks > 0 && (
            <Button variant="primary" className="w-full" onClick={onDone}>
              Next
            </Button>
          )}
        </div>
      ) : (
        <div className="rounded-lg bg-card p-4">
          <div className="mb-2 flex justify-between text-sm">
            <span>{phase?.label ?? 'Starting…'}</span>
            {scan.data && <span className="text-muted">{plural(scan.data.stats.imported, 'track')}</span>}
          </div>
          <div className="h-1.5 overflow-hidden rounded-full bg-raised">
            <div
              className={clsx('h-full rounded-full bg-accent transition-all', phase?.progress == null && 'w-1/3 animate-pulse')}
              style={phase?.progress != null ? { width: `${phase.progress * 100}%` } : undefined}
            />
          </div>
        </div>
      )}
      <ErrorText error={start.error} />
      {jobId && (
        <Button variant={done ? 'primary' : 'secondary'} className="w-full" onClick={onDone}>
          {done ? 'Next' : 'Continue while it scans'}
        </Button>
      )}
    </div>
  );
}

function PlaylistStep() {
  const { add } = usePlaylistActions();
  const qc = useQueryClient();
  const navigate = useNavigate();
  const [url, setUrl] = useState('');
  const [finishing, setFinishing] = useState(false);
  const finish = async () => {
    setFinishing(true);
    await call(api.POST('/api/setup/complete')).catch(() => {});
    await qc.invalidateQueries({ queryKey: keys.setup });
    navigate('/', { replace: true });
  };
  return (
    <div className="space-y-5">
      <p className="text-sm text-muted">
        Paste a public Spotify playlist. OwnWave syncs it daily, finds its songs in your library and suggests stations from it.
      </p>
      <form
        className="space-y-3"
        onSubmit={(e) => {
          e.preventDefault();
          if (url.trim()) add.mutate(url.trim(), { onSuccess: () => setUrl('') });
        }}
      >
        <Input label="Spotify playlist link" placeholder="https://open.spotify.com/playlist/…" value={url} onChange={(e) => setUrl(e.target.value)} />
        <Button type="submit" className="w-full" loading={add.isPending} disabled={!url.trim()}>
          Add playlist
        </Button>
      </form>
      {add.data && (
        <p className="rounded-md bg-accent-soft p-3 text-sm">
          Added “{add.data.name}”: {add.data.matched_count} of {add.data.track_count} tracks found so far.
        </p>
      )}
      <ErrorText error={add.error} />
      <Button variant="primary" className="w-full" onClick={finish} loading={finishing}>
        {add.data ? 'Start listening' : 'Skip for now'}
      </Button>
    </div>
  );
}
