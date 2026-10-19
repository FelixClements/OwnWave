'use client';

import { useEffect, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '@/lib/api';
import { useAuth } from '@/lib/auth';
import { keys } from '@/lib/queries';
import { Button } from '@/components/ui/Button';
import { Field, Input } from '@/components/ui/Field';
import { Notice, PageHeader, Panel, type NoticeValue } from '@/components/ui/Layout';

export default function AccountPage() {
  const { user } = useAuth();
  const qc = useQueryClient();
  const { data: me, isLoading: meLoading } = useQuery(keys.me, () => api.me());
  const updateProfile = useMutation(
    (vars: { email: string; fullName: string }) => api.updateProfile(vars.email, vars.fullName),
    { onSuccess: () => qc.invalidateQueries(keys.me) },
  );
  // Direct REST call so the browser receives the rotated session cookie.
  const changePassword = useMutation((vars: { currentPassword: string; newPassword: string }) =>
    api.changePassword(vars.currentPassword, vars.newPassword),
  );

  const [email, setEmail] = useState('');
  const [fullName, setFullName] = useState('');
  const [profileNotice, setProfileNotice] = useState<NoticeValue>(null);

  const [currentPassword, setCurrentPassword] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [passwordNotice, setPasswordNotice] = useState<NoticeValue>(null);

  useEffect(() => {
    if (me) {
      setEmail(me.email ?? '');
      setFullName(me.full_name ?? '');
    }
  }, [me]);

  const handleProfileSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setProfileNotice(null);
    try {
      await updateProfile.mutateAsync({ email, fullName });
      setProfileNotice({ type: 'success', text: 'Profile saved.' });
    } catch (err) {
      setProfileNotice({ type: 'error', text: err instanceof Error ? err.message : 'Failed to save profile.' });
    }
  };

  const handlePasswordSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setPasswordNotice(null);
    if (newPassword !== confirmPassword) {
      setPasswordNotice({ type: 'error', text: 'New passwords do not match.' });
      return;
    }
    if (newPassword.length < 8) {
      setPasswordNotice({ type: 'error', text: 'New password must be at least 8 characters.' });
      return;
    }
    try {
      await changePassword.mutateAsync({ currentPassword, newPassword });
      setPasswordNotice({ type: 'success', text: 'Password updated.' });
      setCurrentPassword('');
      setNewPassword('');
      setConfirmPassword('');
    } catch (err) {
      setPasswordNotice({ type: 'error', text: err instanceof Error ? err.message : 'Failed to update password.' });
    }
  };

  return (
    <div className="max-w-xl">
      <PageHeader title="Account" subtitle="Manage your account details." />
      <div className="space-y-4">
        <Panel title="Profile">
          <form onSubmit={handleProfileSubmit} className="flex flex-col gap-3">
            <Field label="Username">{(p) => <Input {...p} value={user?.username ?? ''} disabled />}</Field>
            <Field label="Full name">
              {(p) => <Input {...p} value={fullName} onChange={(e) => setFullName(e.target.value)} disabled={meLoading} autoComplete="name" />}
            </Field>
            <Field label="Email">
              {(p) => (
                <Input {...p} type="email" value={email} onChange={(e) => setEmail(e.target.value)} disabled={meLoading} autoComplete="email" />
              )}
            </Field>
            <Notice notice={profileNotice} />
            <div>
              <Button type="submit" variant="primary" loading={updateProfile.isLoading} disabled={meLoading}>
                Save profile
              </Button>
            </div>
          </form>
        </Panel>

        <Panel title="Change password">
          <form onSubmit={handlePasswordSubmit} className="flex flex-col gap-3">
            <Field label="Current password">
              {(p) => (
                <Input
                  {...p}
                  type="password"
                  value={currentPassword}
                  onChange={(e) => setCurrentPassword(e.target.value)}
                  autoComplete="current-password"
                  required
                />
              )}
            </Field>
            <Field label="New password" hint="At least 8 characters.">
              {(p) => (
                <Input
                  {...p}
                  type="password"
                  value={newPassword}
                  onChange={(e) => setNewPassword(e.target.value)}
                  autoComplete="new-password"
                  required
                />
              )}
            </Field>
            <Field label="Confirm new password">
              {(p) => (
                <Input
                  {...p}
                  type="password"
                  value={confirmPassword}
                  onChange={(e) => setConfirmPassword(e.target.value)}
                  autoComplete="new-password"
                  required
                />
              )}
            </Field>
            <Notice notice={passwordNotice} />
            <div>
              <Button type="submit" variant="primary" loading={changePassword.isLoading}>
                Update password
              </Button>
            </div>
          </form>
        </Panel>
      </div>
    </div>
  );
}
