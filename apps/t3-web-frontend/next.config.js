/** @type {import('next').NextConfig} */

const port = process.env.PORT || '3000';
const proxyUri = process.env.VSCODE_PROXY_URI;

const basePath = process.env.NEXT_PUBLIC_BASE_PATH || '';
const assetPrefix = proxyUri
  ? proxyUri.replace(/\{\{port\}\}/g, port).replace(/\/$/, '')
  : process.env.NEXT_PUBLIC_ASSET_PREFIX || '';

const nextConfig = {
  output: 'standalone',
  basePath,
  assetPrefix,
  async rewrites() {
    // Dev only: in production Caddy routes /api/* to Go before Next sees it.
    const go = process.env.GO_API_URL || 'http://localhost:8080';
    return [
      {
        source: '/api/:path*',
        destination: `${go}/:path*`,
      },
    ];
  },
};

module.exports = nextConfig;
