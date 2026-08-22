import { api, getStreamBaseUrl } from '@/lib/api';
import type { StreamOptions } from './types';

export interface StreamResolver {
  resolve(trackId: string, options: StreamOptions): Promise<string>;
}

export function createApiStreamResolver(): StreamResolver {
  return {
    async resolve(trackId, options) {
      const { url } = await api.getStreamUrl(trackId, {
        format: options.format,
        bitrate: options.bitrate,
      });
      return `${getStreamBaseUrl()}${url}`;
    },
  };
}
