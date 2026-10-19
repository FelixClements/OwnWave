'use client';

import { useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import { useAuth } from '@/lib/auth';
import { Button } from '@/components/ui/Button';
import { Field, Input } from '@/components/ui/Field';
import { Panel } from '@/components/ui/Layout';

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
    return <p className="text-muted">Invalid invite link.</p>;
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
    <div className="max-w-sm">
      <h1 className="text-xl font-bold mb-1">Accept invite</h1>
      <p className="text-muted mb-5">Create your account to join this OwnWave server.</p>
      <Panel>
        <form onSubmit={handleSubmit} className="flex flex-col gap-3">
          <Field label="Username">
            {(p) => <Input {...p} value={username} onChange={(e) => setUsername(e.target.value)} autoComplete="username" required autoFocus />}
          </Field>
          <Field label="Password" hint="At least 8 characters." error={error || null}>
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
          <Button type="submit" variant="primary" loading={loading}>
            Create account
          </Button>
        </form>
      </Panel>
    </div>
  );
}
