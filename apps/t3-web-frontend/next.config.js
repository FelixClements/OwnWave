const { PHASE_DEVELOPMENT_SERVER } = require('next/constants');

const port = process.env.PORT || '3000';
const proxyUri = process.env.VSCODE_PROXY_URI;

const basePath = process.env.NEXT_PUBLIC_BASE_PATH || '';
const assetPrefix = proxyUri
  ? proxyUri.replace(/\{\{port\}\}/g, port).replace(/\/$/, '')
  : process.env.NEXT_PUBLIC_ASSET_PREFIX || '';

/** @type {(phase: string) => import('next').NextConfig} */
module.exports = (phase) => {
  // Built as static files that the Go server serves next to its /api routes.
  const nextConfig = {
    output: 'export',
    basePath,
    assetPrefix,
  };

  if (phase === PHASE_DEVELOPMENT_SERVER) {
    // `next dev` proxies API calls to a running Go server.
    const go = process.env.GO_API_URL || 'http://localhost:8080';
    nextConfig.rewrites = async () => [
      { source: '/api/:path*', destination: `${go}/api/:path*` },
      // Invite links are /invite/<token>; one page reads the token from the URL.
      { source: '/invite/:token', destination: '/invite' },
    ];
  }

  return nextConfig;
};
