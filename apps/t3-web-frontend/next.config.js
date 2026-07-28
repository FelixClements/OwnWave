/** @type {import('next').NextConfig} */
const nextConfig = {
  output: 'standalone',
  env: {
    GO_API_URL: process.env.GO_API_URL || 'http://go:8080',
  },
};

module.exports = nextConfig;
