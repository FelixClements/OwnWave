'use client';

import { useState } from 'react';
import Link from 'next/link';
import { useAuth } from '@/lib/auth';

export function ProfileButton() {
  const { user, logout } = useAuth();
  const [open, setOpen] = useState(false);

  if (!user) {
    return (
      <Link
        href="/login"
        className="text-sm font-semibold px-3 py-1.5 rounded-full bg-spotify-green text-black hover:bg-spotify-green-hover transition"
      >
        Login
      </Link>
    );
  }

  return (
    <div className="relative">
      <button
        onClick={() => setOpen(!open)}
        className="w-9 h-9 rounded-full bg-spotify-elevated text-spotify-text flex items-center justify-center font-bold hover:bg-spotify-card-hover transition"
        aria-label="Profile"
      >
        {user.username.charAt(0).toUpperCase()}
      </button>

      {open && (
        <>
          <div
            className="fixed inset-0 z-10"
            onClick={() => setOpen(false)}
          />
          <div className="absolute right-0 mt-2 w-44 bg-spotify-elevated rounded-lg shadow-lg py-2 z-20 border border-spotify-border">
            <div className="px-4 py-2 text-sm font-bold text-spotify-text border-b border-spotify-border">
              {user.username}
            </div>
            <Link
              href="/settings"
              className="block px-4 py-2 text-sm text-spotify-text hover:bg-spotify-card-hover transition"
              onClick={() => setOpen(false)}
            >
              Settings
            </Link>
            <Link
              href="/account"
              className="block px-4 py-2 text-sm text-spotify-text hover:bg-spotify-card-hover transition"
              onClick={() => setOpen(false)}
            >
              Account
            </Link>
            <button
              onClick={() => {
                setOpen(false);
                logout();
              }}
              className="w-full text-left px-4 py-2 text-sm text-spotify-text hover:bg-spotify-card-hover transition"
            >
              Logout
            </button>
          </div>
        </>
      )}
    </div>
  );
}
