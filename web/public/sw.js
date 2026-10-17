// Static assets only: never persist authenticated navigations or user data.
const CACHE_NAME = 'chat-static-v3';
const ASSETS = ['/icon.svg', '/manifest.webmanifest'];

self.addEventListener('install', (event) => {
  event.waitUntil(
    caches.open(CACHE_NAME)
      .then((cache) => cache.addAll(ASSETS))
      .then(() => self.skipWaiting())
  );
});

self.addEventListener('activate', (event) => {
  event.waitUntil(
    caches.keys()
      .then((keys) => Promise.all(keys.filter((key) => key.startsWith('chat-') && key !== CACHE_NAME).map((key) => caches.delete(key))))
      .then(() => self.clients.claim())
  );
});

function isStaticAsset(request, url) {
  if (request.method !== 'GET' || request.mode === 'navigate' || url.origin !== self.location.origin) return false;
  return url.pathname.startsWith('/_next/static/') || ASSETS.includes(url.pathname);
}

self.addEventListener('fetch', (event) => {
  const request = event.request;
  const url = new URL(request.url);
  // Let navigation use the network. Caching HTML for / or /me can leak a prior
  // account's SSR payload after sign-out or when accounts share a device.
  if (!isStaticAsset(request, url)) return;
  event.respondWith(
    caches.match(request).then((cached) => cached || fetch(request).then((response) => {
      if (response.ok && response.type === 'basic' && !response.headers.has('set-cookie') &&
          !/no-store|private/i.test(response.headers.get('cache-control') || '')) {
        const copy = response.clone();
        event.waitUntil(caches.open(CACHE_NAME).then((cache) => cache.put(request, copy)));
      }
      return response;
    }))
  );
});

// Push alerts and notification deep links are intentionally not registered in
// the private-text-only core release. Reintroduce them only with explicit
// consent, message-content redaction and fresh privacy/security review.
