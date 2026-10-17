import { NextRequest, NextResponse } from 'next/server';

// The API has its own independent core-launch gate. This navigation gate is
// only for clarity: users must not discover unlaunched screens via deep links.
const allowed = new Set([
  '/', '/login', '/welcome', '/onboarding', '/me', '/requests',
  '/settings/profile', '/settings/security', '/settings/language'
]);
function isAllowed(pathname: string) {
  if (allowed.has(pathname)) return true;
  if (pathname.startsWith('/auth/')) return true;
  if (pathname.startsWith('/admin/')) return true; // Owner-only API checks still apply.
  return false;
}
export function middleware(request: NextRequest) {
  if (process.env.CHAT_LAUNCH_MODE === 'full') return NextResponse.next();
  const path = request.nextUrl.pathname;
  if (isAllowed(path)) return NextResponse.next();
  if (path.startsWith('/api/')) return NextResponse.next(); // Caddy routes API.
  if (path === '/manifest.webmanifest' || path === '/sw.js' || path === '/icon.svg') {
    return NextResponse.next();
  }
  const target = request.nextUrl.clone();
  target.pathname = '/';
  target.search = '';
  return NextResponse.redirect(target, 307);
}
export const config = {
  matcher: ['/((?!_next/static|_next/image|favicon.ico|icon.svg|manifest.webmanifest|sw.js).*)']
};
