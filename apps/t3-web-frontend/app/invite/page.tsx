'use client';

import { useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import { useAuth } from '@/lib/auth';

export default function InvitePage() {
  // Invite links are /invite/<token>; every one of them is served this page.
  const [token, setToken] = useState<string | null>(null);
  const { register, user } = useAuth();
  const router = useRouter();
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    setToken(decodeURIComponent(window.location.pathname.split('/')[2] ?? ''));
  }, []);

  useEffect(() => {
    if (user) router.push('/');
  }, [user, router]);

  if (user || token === null) return null;

  if (!token) {
    return (
      <div className="min-h-screen bg-spotify-bg flex items-center justify-center text-spotify-subdued">
        Invalid invite link.
      </div>
    );
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setLoading(true);
    try {
      await register(username, password, token);
      router.push('/');
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Something went wrong');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="min-h-screen bg-spotify-bg flex items-center justify-center p-4">
      <div className="w-full max-w-sm bg-spotify-card rounded-lg p-6 border border-spotify-border">
        <h1 className="text-xl font-bold mb-2">Accept invite</h1>
        <p className="text-sm text-spotify-subdued mb-6">Create your account to join this OwnWave server.</p>

        <form onSubmit={handleSubmit} className="space-y-4">
          <input
            type="text"
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            placeholder="Username"
            className="w-full px-4 py-2 rounded bg-spotify-elevated text-spotify-text placeholder-spotify-subdued border border-spotify-border focus:outline-none focus:border-spotify-green"
            required
          />
          <input
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            placeholder="Password (min 8 characters)"
            minLength={8}
            className="w-full px-4 py-2 rounded bg-spotify-elevated text-spotify-text placeholder-spotify-subdued border border-spotify-border focus:outline-none focus:border-spotify-green"
            required
          />

          {error && <p className="text-red-500 text-sm">{error}</p>}

          <button
            type="submit"
            disabled={loading}
            className="w-full px-4 py-2 rounded bg-spotify-green text-black font-semibold hover:bg-spotify-green-hover transition disabled:opacity-50"
          >
            {loading ? 'Creating account...' : 'Create account'}
          </button>
        </form>
      </div>
    </div>
  );
}
