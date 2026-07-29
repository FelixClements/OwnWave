/** @type {import('next').NextConfig} */

const port = process.env.PORT || '3000';
const proxyUri = process.env.VSCODE_PROXY_URI;

let basePath = process.env.NEXT_PUBLIC_BASE_PATH || '';
let assetPrefix = process.env.NEXT_PUBLIC_ASSET_PREFIX || '';

if (proxyUri) {
  const resolved = proxyUri.replace(/\{\{port\}\}/g, port).replace(/\/$/, '');
  if (resolved.startsWith('http')) {
    assetPrefix = resolved;
  } else {
    assetPrefix = resolved;
  }
}

const nextConfig = {
  output: 'standalone',
  basePath,
  assetPrefix,
  env: {
    GO_API_URL: process.env.GO_API_URL || 'http://go:8080',
  },
};

module.exports = nextConfig;
