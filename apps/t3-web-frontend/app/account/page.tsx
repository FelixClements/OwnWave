'use client';

import { useEffect, useState } from 'react';
import { useAuth } from '@/lib/auth';
import { trpc } from '@/lib/trpc/client';

export default function AccountPage() {
  const { user } = useAuth();
  const utils = trpc.useContext();
  const { data: me, isLoading: meLoading } = trpc.me.useQuery();
  const updateProfile = trpc.updateProfile.useMutation({
    onSuccess: () => {
      utils.me.invalidate();
    },
  });
  const changePassword = trpc.changePassword.useMutation();

  const [email, setEmail] = useState('');
  const [fullName, setFullName] = useState('');
  const [profileMessage, setProfileMessage] = useState<string | null>(null);
  const [profileError, setProfileError] = useState<string | null>(null);

  const [currentPassword, setCurrentPassword] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [passwordMessage, setPasswordMessage] = useState<string | null>(null);
  const [passwordError, setPasswordError] = useState<string | null>(null);

  useEffect(() => {
    if (me) {
      setEmail(me.email ?? '');
      setFullName(me.full_name ?? '');
    }
  }, [me]);

  const handleProfileSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setProfileMessage(null);
    setProfileError(null);

    try {
      await updateProfile.mutateAsync({ email, fullName });
      setProfileMessage('Profile saved.');
    } catch (err) {
      setProfileError(err instanceof Error ? err.message : 'Failed to save profile.');
    }
  };

  const handlePasswordSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setPasswordMessage(null);
    setPasswordError(null);

    if (newPassword !== confirmPassword) {
      setPasswordError('New passwords do not match.');
      return;
    }
    if (newPassword.length < 6) {
      setPasswordError('New password must be at least 6 characters.');
      return;
    }

    try {
      await changePassword.mutateAsync({ currentPassword, newPassword });
      setPasswordMessage('Password updated successfully.');
      setCurrentPassword('');
      setNewPassword('');
      setConfirmPassword('');
    } catch (err) {
      setPasswordError(err instanceof Error ? err.message : 'Failed to update password.');
    }
  };

  return (
    <div className="max-w-2xl mx-auto space-y-6">
      <h1 className="text-2xl font-bold">Account</h1>
      <p className="text-spotify-subdued">Manage your account details.</p>

      <form
        onSubmit={handleProfileSubmit}
        className="bg-spotify-elevated rounded-lg p-6 space-y-4"
      >
        <div>
          <h2 className="text-lg font-semibold">Profile</h2>
          <p className="text-sm text-spotify-subdued">Update your account information.</p>
        </div>

        <div className="space-y-2">
          <label htmlFor="username" className="text-sm font-medium text-spotify-text">
            Username
          </label>
          <input
            id="username"
            type="text"
            value={user?.username ?? ''}
            disabled
            className="w-full bg-spotify-black border border-spotify-border text-spotify-subdued rounded-md px-3 py-2 cursor-not-allowed"
          />
        </div>

        <div className="space-y-2">
          <label htmlFor="fullName" className="text-sm font-medium text-spotify-text">
            Full Name
          </label>
          <input
            id="fullName"
            type="text"
            value={fullName}
            onChange={(e) => setFullName(e.target.value)}
            disabled={meLoading}
            className="w-full bg-spotify-black border border-spotify-border text-spotify-text rounded-md px-3 py-2 focus:outline-none focus:ring-2 focus:ring-spotify-green"
          />
        </div>

        <div className="space-y-2">
          <label htmlFor="email" className="text-sm font-medium text-spotify-text">
            Email
          </label>
          <input
            id="email"
            type="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            disabled={meLoading}
            className="w-full bg-spotify-black border border-spotify-border text-spotify-text rounded-md px-3 py-2 focus:outline-none focus:ring-2 focus:ring-spotify-green"
          />
        </div>

        {profileError && <p className="text-sm text-red-500">{profileError}</p>}
        {profileMessage && <p className="text-sm text-spotify-green">{profileMessage}</p>}

        <button
          type="submit"
          disabled={updateProfile.isLoading || meLoading}
          className="px-4 py-2 bg-spotify-green text-black font-semibold rounded-md hover:bg-spotify-green-hover transition disabled:opacity-50"
        >
          {updateProfile.isLoading ? 'Saving...' : 'Save Profile'}
        </button>
      </form>

      <form
        onSubmit={handlePasswordSubmit}
        className="bg-spotify-elevated rounded-lg p-6 space-y-4"
      >
        <div>
          <h2 className="text-lg font-semibold">Change Password</h2>
          <p className="text-sm text-spotify-subdued">Update your account password.</p>
        </div>

        <div className="space-y-2">
          <label htmlFor="current" className="text-sm font-medium text-spotify-text">
            Current Password
          </label>
          <input
            id="current"
            type="password"
            value={currentPassword}
            onChange={(e) => setCurrentPassword(e.target.value)}
            className="w-full bg-spotify-black border border-spotify-border text-spotify-text rounded-md px-3 py-2 focus:outline-none focus:ring-2 focus:ring-spotify-green"
            required
          />
        </div>

        <div className="space-y-2">
          <label htmlFor="new" className="text-sm font-medium text-spotify-text">
            New Password
          </label>
          <input
            id="new"
            type="password"
            value={newPassword}
            onChange={(e) => setNewPassword(e.target.value)}
            className="w-full bg-spotify-black border border-spotify-border text-spotify-text rounded-md px-3 py-2 focus:outline-none focus:ring-2 focus:ring-spotify-green"
            required
          />
        </div>

        <div className="space-y-2">
          <label htmlFor="confirm" className="text-sm font-medium text-spotify-text">
            Confirm New Password
          </label>
          <input
            id="confirm"
            type="password"
            value={confirmPassword}
            onChange={(e) => setConfirmPassword(e.target.value)}
            className="w-full bg-spotify-black border border-spotify-border text-spotify-text rounded-md px-3 py-2 focus:outline-none focus:ring-2 focus:ring-spotify-green"
            required
          />
        </div>

        {passwordError && <p className="text-sm text-red-500">{passwordError}</p>}
        {passwordMessage && <p className="text-sm text-spotify-green">{passwordMessage}</p>}

        <button
          type="submit"
          disabled={changePassword.isLoading}
          className="px-4 py-2 bg-spotify-green text-black font-semibold rounded-md hover:bg-spotify-green-hover transition disabled:opacity-50"
        >
          {changePassword.isLoading ? 'Updating...' : 'Update Password'}
        </button>
      </form>
    </div>
  );
}
