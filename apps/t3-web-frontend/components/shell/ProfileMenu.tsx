'use client';

import { useEffect, useRef, useState } from 'react';
import Link from 'next/link';
import { useAuth } from '@/lib/auth';

/** Avatar button disclosing account links and logout. Esc or an outside click closes it. */
export function ProfileMenu() {
  const { user, logout } = useAuth();
  const [open, setOpen] = useState(false);
  const rootRef = useRef<HTMLDivElement>(null);
  const buttonRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        setOpen(false);
        buttonRef.current?.focus();
      }
    };
    const onClick = (e: MouseEvent) => {
      if (!rootRef.current?.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener('keydown', onKey);
    document.addEventListener('mousedown', onClick);
    return () => {
      document.removeEventListener('keydown', onKey);
      document.removeEventListener('mousedown', onClick);
    };
  }, [open]);

  if (!user) return null;

  const item = 'block w-full text-left px-3 py-1.5 hover:bg-hover';
  return (
    <div ref={rootRef} className="relative">
      <button
        ref={buttonRef}
        type="button"
        onClick={() => setOpen(!open)}
        aria-expanded={open}
        aria-controls="profile-menu"
        aria-label={`Account menu for ${user.username}`}
        className="grid place-items-center w-7 h-7 rounded-full border border-line bg-surface-2 text-[11px] font-semibold hover:bg-hover"
      >
        {user.username.slice(0, 2).toUpperCase()}
      </button>
      {open && (
        <div id="profile-menu" className="absolute right-0 mt-2 w-48 z-50 rounded-lg border border-line bg-surface py-1 shadow-xl">
          <div className="px-3 py-1.5 font-semibold border-b border-line truncate">{user.username}</div>
          <ul>
            <li>
              <Link href="/account" className={item} onClick={() => setOpen(false)}>
                Account
              </Link>
            </li>
            <li>
              <Link href="/settings" className={item} onClick={() => setOpen(false)}>
                Settings
              </Link>
            </li>
            {user.is_admin && (
              <li>
                <Link href="/admin" className={item} onClick={() => setOpen(false)}>
                  Admin
                </Link>
              </li>
            )}
            <li>
              <button
                type="button"
                className={`${item} text-danger`}
                onClick={() => {
                  setOpen(false);
                  void logout();
                }}
              >
                Log out
              </button>
            </li>
          </ul>
        </div>
      )}
    </div>
  );
}
