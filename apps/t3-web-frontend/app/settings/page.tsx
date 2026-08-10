'use client';

import { useTheme } from '@/lib/theme';

export default function SettingsPage() {
  const { theme, setTheme } = useTheme();

  return (
    <div className="max-w-2xl mx-auto space-y-6">
      <h1 className="text-2xl font-bold">Settings</h1>
      <p className="text-spotify-subdued">Configure your OwnWave preferences.</p>

      <div className="bg-spotify-elevated rounded-lg p-6 space-y-4">
        <div>
          <h2 className="text-lg font-semibold">Appearance</h2>
          <p className="text-sm text-spotify-subdued">Choose the default theme for OwnWave.</p>
        </div>

        <div className="space-y-2">
          <label htmlFor="theme" className="text-sm font-medium text-spotify-text">
            Theme
          </label>
          <select
            id="theme"
            value={theme}
            onChange={(e) => setTheme(e.target.value as 'dark' | 'light')}
            className="w-full bg-spotify-black border border-spotify-border text-spotify-text rounded-md px-3 py-2 focus:outline-none focus:ring-2 focus:ring-spotify-green"
          >
            <option value="dark">Dark</option>
            <option value="light">Light</option>
          </select>
        </div>
      </div>
    </div>
  );
}
