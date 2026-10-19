'use client';

import { useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import { useAuth } from '@/lib/auth';
import { Button } from '@/components/ui/Button';
import { Field, Input } from '@/components/ui/Field';
import { Panel } from '@/components/ui/Layout';

export default function LoginPage() {
  const { login, user } = useAuth();
  const router = useRouter();
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (user) router.push('/');
  }, [user, router]);

  if (user) return null;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setLoading(true);
    try {
      await login(username, password);
      router.push('/');
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Something went wrong');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="max-w-sm">
      <h1 className="text-xl font-bold mb-1">Sign in</h1>
      <p className="text-muted mb-5">Your self-hosted radio, from your own lossless library.</p>
      <Panel>
        <form onSubmit={handleSubmit} className="flex flex-col gap-3">
          <Field label="Username">
            {(p) => <Input {...p} value={username} onChange={(e) => setUsername(e.target.value)} autoComplete="username" required autoFocus />}
          </Field>
          <Field label="Password" error={error || null}>
            {(p) => (
              <Input
                {...p}
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                autoComplete="current-password"
                required
              />
            )}
          </Field>
          <Button type="submit" variant="primary" loading={loading}>
            Sign in
          </Button>
        </form>
      </Panel>
      <p className="mt-4 text-muted">New accounts require an invite from an admin.</p>
    </div>
  );
}
