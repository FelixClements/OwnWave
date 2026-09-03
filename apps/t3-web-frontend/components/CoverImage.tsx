'use client';

import { useEffect, useState } from 'react';
import { getStreamBaseUrl } from '@/lib/api';

type CoverImageProps = {
  id: string;
  alt?: string;
  className?: string;
  onError?: () => void;
};

export function CoverImage({ id, alt = '', className, onError }: CoverImageProps) {
  const [src, setSrc] = useState<string | null>(null);

  useEffect(() => {
    let objectUrl: string | undefined;
    let cancelled = false;

    const load = async () => {
      const res = await fetch(`${getStreamBaseUrl()}/tracks/${encodeURIComponent(id)}/cover`, {
        credentials: 'include',
      });
      if (!res.ok) {
        onError?.();
        return;
      }
      const blob = await res.blob();
      if (cancelled) return;
      objectUrl = URL.createObjectURL(blob);
      setSrc(objectUrl);
    };

    void load().catch(() => onError?.());

    return () => {
      cancelled = true;
      if (objectUrl) URL.revokeObjectURL(objectUrl);
    };
  }, [id, onError]);

  if (!src) {
    return <div className={className} aria-hidden />;
  }

  return (
    <img
      src={src}
      alt={alt}
      className={className}
      onError={() => onError?.()}
    />
  );
}

export function useCoverUrl(id: string | null | undefined) {
  const [url, setUrl] = useState<string | null>(null);

  useEffect(() => {
    if (!id) {
      setUrl(null);
      return;
    }

    let objectUrl: string | undefined;
    let cancelled = false;

    const load = async () => {
      const res = await fetch(`${getStreamBaseUrl()}/tracks/${encodeURIComponent(id)}/cover`, {
        credentials: 'include',
      });
      if (!res.ok) return;
      const blob = await res.blob();
      if (cancelled) return;
      objectUrl = URL.createObjectURL(blob);
      setUrl(objectUrl);
    };

    void load();

    return () => {
      cancelled = true;
      if (objectUrl) URL.revokeObjectURL(objectUrl);
    };
  }, [id]);

  return url;
}
