import { getStreamBaseUrl } from '@/lib/api';
import type { StreamOptions } from './types';

export interface StreamResolver {
  resolve(trackId: string, options: StreamOptions): Promise<string>;
}

export function createApiStreamResolver(): StreamResolver {
  return {
    async resolve(trackId, options) {
      const params = new URLSearchParams();
      params.set('format', options.format);
      if (options.bitrate) params.set('bitrate', options.bitrate);
      return `${getStreamBaseUrl()}/stream/${encodeURIComponent(trackId)}?${params.toString()}`;
    },
  };
}
