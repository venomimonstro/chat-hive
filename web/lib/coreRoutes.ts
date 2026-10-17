// Shared, auditable policy for the deliberately small first commercial release.
const pages = new Set([
 '/', '/login', '/welcome', '/onboarding', '/me', '/requests',
 '/settings/profile', '/settings/security', '/settings/language'
]);
export function corePageAllowed(path: string): boolean {
 if (pages.has(path)) return true;
 if (path.startsWith('/auth/')) return true;
 if (path.startsWith('/admin/')) return true; // Admin has independent passkey gate.
 if (path === '/manifest.webmanifest' || path === '/sw.js' || path === '/icon.svg') return true;
 return false;
}
