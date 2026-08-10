'use client';

import { useState } from 'react';
import { useRouter } from 'next/navigation';
import { useAuth } from '@/lib/auth';

export default function LoginPage() {
  const { login, register, user } = useAuth();
  const router = useRouter();
  const [mode, setMode] = useState<'login' | 'register'>('login');
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  if (user) {
    router.push('/');
    return null;
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setLoading(true);
    try {
      if (mode === 'login') {
        await login(username, password);
      } else {
        await register(username, password);
      }
      router.push('/');
    } catch (err: any) {
      setError(err?.message || 'Something went wrong');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="min-h-screen flex flex-col md:flex-row bg-black text-spotify-text">
      <div className="w-full md:w-3/4 min-h-[50vh] md:min-h-screen bg-white flex items-center justify-center text-black p-8">
        <h1 className="text-7xl md:text-8xl font-extrabold tracking-tight">OwnWave</h1>
      </div>

      <div className="w-full md:w-1/4 min-h-[50vh] md:min-h-screen bg-black flex flex-col justify-center items-center p-4 md:p-6">
        <div className="w-full max-w-sm">
          <h2 className="text-2xl font-bold mb-6 text-center md:text-left">
            {mode === 'login' ? 'Sign in' : 'Create account'}
          </h2>

          <div className="flex rounded bg-spotify-elevated p-1 mb-6">
            <button
              type="button"
              onClick={() => setMode('login')}
              className={`flex-1 py-1.5 text-sm font-semibold rounded transition ${
                mode === 'login' ? 'bg-spotify-card text-spotify-text' : 'text-spotify-subdued'
              }`}
            >
              Login
            </button>
            <button
              type="button"
              onClick={() => setMode('register')}
              className={`flex-1 py-1.5 text-sm font-semibold rounded transition ${
                mode === 'register' ? 'bg-spotify-card text-spotify-text' : 'text-spotify-subdued'
              }`}
            >
              Register
            </button>
          </div>

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
              placeholder="Password"
              className="w-full px-4 py-2 rounded bg-spotify-elevated text-spotify-text placeholder-spotify-subdued border border-spotify-border focus:outline-none focus:border-spotify-green"
              required
            />

            {error && <p className="text-red-500 text-sm">{error}</p>}

            <button
              type="submit"
              disabled={loading}
              className="w-full px-4 py-2 rounded bg-spotify-green text-black font-semibold hover:bg-spotify-green-hover transition disabled:opacity-50"
            >
              {loading ? 'Please wait...' : mode === 'login' ? 'Sign in' : 'Create account'}
            </button>
          </form>

        </div>
      </div>
    </div>
  );
}
