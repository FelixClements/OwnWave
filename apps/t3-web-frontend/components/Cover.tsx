'use client';

import { useEffect, useState } from 'react';
import { coverUrl } from '@/lib/api';

/** Track cover art, falling back to the title's first letter. */
export function Cover({ id, title, className }: { id: string; title: string; className?: string }) {
  const [failed, setFailed] = useState(false);
  useEffect(() => setFailed(false), [id]);

  if (failed) {
    return (
      <div className={`${className ?? ''} flex items-center justify-center`}>
        <span className="font-bold">{title.charAt(0).toUpperCase()}</span>
      </div>
    );
  }
  // eslint-disable-next-line @next/next/no-img-element -- auth'd same-origin API image, not a static asset
  return <img src={coverUrl(id)} alt="" className={className} onError={() => setFailed(true)} />;
}
