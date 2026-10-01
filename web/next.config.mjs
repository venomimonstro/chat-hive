function origins() {
  const raw = process.env.NEXT_PUBLIC_API_BASE_URL || 'http://localhost:8080';
  try {
    const api = new URL(raw);
    const ws = new URL(raw);
    ws.protocol = api.protocol === 'https:' ? 'wss:' : 'ws:';
    return { api: api.origin, ws: ws.origin };
  } catch {
    return { api: 'http://localhost:8080', ws: 'ws://localhost:8080' };
  }
}

const { api: apiOrigin, ws: wsOrigin } = origins();
const csp = [
  "default-src 'self'",
  "base-uri 'self'",
  "frame-ancestors 'none'",
  "object-src 'none'",
  "form-action 'self'",
  `connect-src 'self' ${apiOrigin} ${wsOrigin}`,
  `img-src 'self' data: blob: ${apiOrigin}`,
  "font-src 'self' data:",
  "manifest-src 'self'",
  "worker-src 'self' blob:",
  "script-src 'self' 'unsafe-inline'",
  "style-src 'self' 'unsafe-inline'"
].join('; ');

/** @type {import('next').NextConfig} */
const nextConfig = {
  output: 'standalone',
  poweredByHeader: false,
  reactStrictMode: true,
  compress: true,
  productionBrowserSourceMaps: false,
  async headers() {
    const securityHeaders = [
      { key: 'Content-Security-Policy', value: csp },
      { key: 'X-Content-Type-Options', value: 'nosniff' },
      { key: 'Referrer-Policy', value: 'no-referrer' },
      { key: 'Permissions-Policy', value: 'camera=(), microphone=(), geolocation=()' },
      { key: 'X-Frame-Options', value: 'DENY' },
      { key: 'Cross-Origin-Opener-Policy', value: 'same-origin' },
      { key: 'Cross-Origin-Resource-Policy', value: 'same-origin' }
    ];
    if (process.env.NODE_ENV === 'production') {
      securityHeaders.push({ key: 'Strict-Transport-Security', value: 'max-age=31536000; includeSubDomains' });
    }
    return [{ source: '/:path*', headers: securityHeaders }];
  }
};

export default nextConfig;
