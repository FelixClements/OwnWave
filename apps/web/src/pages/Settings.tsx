import { useState, type ReactNode } from 'react';
import { Laptop, Smartphone } from 'lucide-react';
import { api, call } from '@/api/client';
import type { Quality } from '@/api/types';
import { useAuth } from '@/lib/auth';
import { timeAgo } from '@/lib/format';
import { useDevices, useRevokeDevice } from '@/lib/queries';
import { QUALITY_LABELS, onCellular, updateSettings, useSettings } from '@/lib/settings';
import { Slider } from '@/components/player-ui';
import { Button, ErrorText, Input, Select, Spinner } from '@/components/ui';

const QUALITIES = (Object.keys(QUALITY_LABELS) as Quality[]).map((q) => ({ value: q, label: QUALITY_LABELS[q] }));

function Card({ title, children, hint }: { title: string; hint?: string; children: ReactNode }) {
  return (
    <section className="mb-6 rounded-xl bg-card p-6">
      <h2 className="text-lg font-bold">{title}</h2>
      {hint && <p className="mt-1 text-sm text-muted">{hint}</p>}
      <div className="mt-5">{children}</div>
    </section>
  );
}

function Toggle({ label, checked, onChange, hint }: { label: string; checked: boolean; onChange: (v: boolean) => void; hint?: string }) {
  return (
    <label className="flex cursor-pointer items-center justify-between gap-4">
      <span>
        <span className="block text-sm font-semibold">{label}</span>
        {hint && <span className="block text-[13px] text-muted">{hint}</span>}
      </span>
      <input type="checkbox" role="switch" checked={checked} onChange={(e) => onChange(e.target.checked)} className="peer sr-only" />
      <span className="relative h-6 w-11 shrink-0 rounded-full bg-line transition after:absolute after:left-1 after:top-1 after:h-4 after:w-4 after:rounded-full after:bg-ink after:transition peer-checked:bg-accent peer-checked:after:translate-x-5 peer-focus-visible:outline peer-focus-visible:outline-2 peer-focus-visible:outline-accent" />
    </label>
  );
}

function Devices() {
  const devices = useDevices();
  const revoke = useRevokeDevice();
  if (devices.isLoading) return <Spinner />;
  if (!devices.data?.length) return <p className="text-sm text-muted">No apps signed in. Sign in from the OwnWave Android app to see it here.</p>;
  return (
    <ul className="divide-y divide-line">
      {devices.data.map((d) => (
        <li key={d.id} className="flex items-center gap-4 py-3">
          <Smartphone className="h-5 w-5 text-muted" aria-hidden />
          <div className="min-w-0 flex-1">
            <div className="truncate font-medium">
              {d.name} {d.current && <span className="text-[12px] text-accent">· this device</span>}
            </div>
            <div className="text-[13px] text-muted">
              {d.last_used_at ? `Last used ${timeAgo(d.last_used_at)}` : 'Never used'} · signed in {timeAgo(d.created_at)}
            </div>
          </div>
          <Button size="sm" variant="danger" loading={revoke.isPending && revoke.variables === d.id} onClick={() => revoke.mutate(d.id)}>
            Sign out
          </Button>
        </li>
      ))}
    </ul>
  );
}

function Password() {
  const [current, setCurrent] = useState('');
  const [next, setNext] = useState('');
  const [state, setState] = useState<{ busy?: boolean; error?: unknown; done?: boolean }>({});
  return (
    <form
      className="max-w-sm space-y-4"
      onSubmit={async (e) => {
        e.preventDefault();
        setState({ busy: true });
        try {
          await call(api.POST('/api/me/password', { body: { current_password: current, new_password: next } }));
          setCurrent('');
          setNext('');
          setState({ done: true });
        } catch (err) {
          setState({ error: err });
        }
      }}
    >
      <Input label="Current password" type="password" autoComplete="current-password" value={current} onChange={(e) => setCurrent(e.target.value)} required />
      <Input label="New password" type="password" autoComplete="new-password" minLength={8} value={next} onChange={(e) => setNext(e.target.value)} required hint="Signs out every other browser and app." />
      <ErrorText error={state.error} />
      {state.done && <p className="text-sm text-accent">Password changed.</p>}
      <Button type="submit" variant="primary" loading={state.busy}>
        Change password
      </Button>
    </form>
  );
}

export function SettingsPage() {
  const s = useSettings();
  const { user } = useAuth();
  const cellular = onCellular();
  return (
    <div className="max-w-3xl pt-2">
      <h1 className="mb-6 text-3xl font-black tracking-tight">Settings</h1>
      <Card title="Streaming quality" hint="Lower qualities are converted once on the server, then cached. Changes apply from the next track.">
        <div className="grid gap-5 sm:grid-cols-2">
          <div>
            <div className="mb-2 flex items-center gap-2 text-[13px] text-muted">
              <Laptop className="h-4 w-4" /> Wi-Fi and wired {!cellular && <span className="text-accent">· now</span>}
            </div>
            <Select label="Quality" value={s.quality} onChange={(quality) => updateSettings({ quality })} options={QUALITIES} />
          </div>
          <div>
            <div className="mb-2 flex items-center gap-2 text-[13px] text-muted">
              <Smartphone className="h-4 w-4" /> Mobile data or data saver {cellular && <span className="text-accent">· now</span>}
            </div>
            <Select label="Quality" value={s.mobileQuality} onChange={(mobileQuality) => updateSettings({ mobileQuality })} options={QUALITIES} />
          </div>
        </div>
      </Card>
      <Card title="Playback">
        <div className="space-y-6">
          <div>
            <div className="mb-2 flex justify-between text-sm">
              <span className="font-semibold">Crossfade</span>
              <span className="tabular-nums text-muted">{s.crossfade ? `${s.crossfade} s` : 'Off'}</span>
            </div>
            <Slider label="Crossfade seconds" value={s.crossfade} max={12} step={1} onChange={(crossfade) => updateSettings({ crossfade })} className="w-full" />
          </div>
          <Toggle label="Even out loudness" hint="Plays every track at a similar volume." checked={s.normalize} onChange={(normalize) => updateSettings({ normalize })} />
        </div>
      </Card>
      <Card title="Signed-in apps" hint="Android and Android Auto stay signed in until you sign them out here.">
        <Devices />
      </Card>
      <Card title="Account" hint={`Signed in as ${user?.username}.`}>
        <Password />
      </Card>
    </div>
  );
}
