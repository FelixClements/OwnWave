import { useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { useAuth } from '@/lib/auth';
import { AuthLayout } from '@/components/Shell';
import { Button, ErrorText, Input } from '@/components/ui';

export function InvitePage() {
  const { token = '' } = useParams();
  const { register } = useAuth();
  const navigate = useNavigate();
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState<unknown>(null);
  const [busy, setBusy] = useState(false);
  return (
    <AuthLayout title="You're invited" subtitle="Create your account to start listening.">
      <form
        className="space-y-4"
        onSubmit={async (e) => {
          e.preventDefault();
          setBusy(true);
          setError(null);
          try {
            await register(username.trim(), password, token);
            navigate('/', { replace: true });
          } catch (err) {
            setError(err);
          } finally {
            setBusy(false);
          }
        }}
      >
        <Input label="Username" autoComplete="username" value={username} onChange={(e) => setUsername(e.target.value)} required autoFocus />
        <Input label="Password" type="password" autoComplete="new-password" minLength={8} hint="At least 8 characters." value={password} onChange={(e) => setPassword(e.target.value)} required />
        <ErrorText error={error} />
        <Button type="submit" variant="primary" className="w-full" loading={busy}>
          Create account
        </Button>
      </form>
    </AuthLayout>
  );
}
