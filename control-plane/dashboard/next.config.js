/** @type {import('next').NextConfig} */
const nextConfig = {
  reactStrictMode: true,
  output: 'standalone', // For Docker deployment
  transpilePackages: [],
  eslint: {
    ignoreDuringBuilds: true
  },
  // Disable telemetry in production
  compiler: {
    removeConsole: process.env.NODE_ENV === 'production',
  },
};

module.exports = nextConfig;

