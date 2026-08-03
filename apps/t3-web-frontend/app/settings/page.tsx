import Link from 'next/link';

export default function SettingsPage() {
  return (
    <div className="min-h-screen bg-spotify-bg text-spotify-text p-4 md:p-8">
      <div className="max-w-2xl mx-auto">
        <div className="flex items-center gap-4 mb-6">
          <Link href="/" className="text-sm font-bold hover:text-spotify-green transition">
            ← Home
          </Link>
          <h1 className="text-2xl font-bold">Settings</h1>
        </div>
        <p className="text-spotify-subdued">Settings will be configured here.</p>
      </div>
    </div>
  );
}
