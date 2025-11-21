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
  // Expose ENVIRONMENT as NEXT_PUBLIC_ENVIRONMENT for client-side access
  env: {
    NEXT_PUBLIC_ENVIRONMENT: process.env.ENVIRONMENT || process.env.NODE_ENV,
  },
};

module.exports = nextConfig;

