'use client';

import { useTheme } from '@/lib/theme';
import { StreamSelect } from '@/components/player/StreamSelect';
import { Field, Select } from '@/components/ui/Field';
import { PageHeader, Panel } from '@/components/ui/Layout';

export default function SettingsPage() {
  const { theme, setTheme } = useTheme();

  return (
    <div className="max-w-xl">
      <PageHeader title="Settings" subtitle="Preferences are saved in this browser." />
      <div className="space-y-4">
        <Panel title="Appearance">
          <Field label="Theme">
            {(p) => (
              <Select {...p} value={theme} onChange={(e) => setTheme(e.target.value as 'dark' | 'light')}>
                <option value="dark">Dark</option>
                <option value="light">Light</option>
              </Select>
            )}
          </Field>
        </Panel>
        <Panel title="Streaming">
          <p className="text-muted mb-2.5">
            FLAC streams the original lossless file. MP3, Opus and AAC are transcoded on the server and use less bandwidth.
          </p>
          <StreamSelect selectClassName="h-[30px] text-[13px] px-2" />
        </Panel>
      </div>
    </div>
  );
}
