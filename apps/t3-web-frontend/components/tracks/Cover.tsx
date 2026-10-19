'use client';

import { useEffect, useState } from 'react';
import { coverUrl } from '@/lib/api';

/** Track cover art, falling back to the title's first letter. */
export function Cover({ id, title, className = '' }: { id: string; title: string; className?: string }) {
  const [failed, setFailed] = useState(false);
  useEffect(() => setFailed(false), [id]);

  const base = `shrink-0 overflow-hidden rounded-md bg-gradient-to-br from-teal-700 to-blue-900 text-white ${className}`;
  if (failed) {
    return (
      <div className={`${base} flex items-center justify-center font-bold`} aria-hidden="true">
        {title.charAt(0).toUpperCase()}
      </div>
    );
  }
  // eslint-disable-next-line @next/next/no-img-element -- auth'd same-origin API image, not a static asset
  return <img src={coverUrl(id)} alt="" className={`${base} object-cover`} onError={() => setFailed(true)} />;
}
