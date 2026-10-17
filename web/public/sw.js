// Static assets only: never persist authenticated navigations or user data.
const CACHE_NAME = 'chat-static-v2';
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

function safeNotificationPath(value) {
  if (typeof value !== 'string' || !value.startsWith('/') || value.startsWith('//') || value.includes('\\')) {
    return '/notifications';
  }
  try {
    const url = new URL(value, self.location.origin);
    return url.origin === self.location.origin ? url.pathname + url.search + url.hash : '/notifications';
  } catch {
    return '/notifications';
  }
}

self.addEventListener('push', (event) => {
  let data = {};
  try { data = event.data ? event.data.json() : {}; } catch { data = {}; }
  const title = typeof data.title === 'string' && data.title ? data.title : 'CHAT';
  event.waitUntil(self.registration.showNotification(title, {
    body: typeof data.body === 'string' ? data.body : 'Новое событие в CHAT',
    icon: '/icon.svg',
    badge: '/icon.svg',
    tag: typeof data.tag === 'string' ? data.tag : undefined,
    data: { url: safeNotificationPath(data.url) }
  }));
});

self.addEventListener('notificationclick', (event) => {
  event.notification.close();
  const target = safeNotificationPath(event.notification.data?.url);
  event.waitUntil(
    self.clients.matchAll({ type: 'window', includeUncontrolled: true }).then((clients) => {
      for (const client of clients) {
        if ('focus' in client) {
          return client.navigate(target).then(() => client.focus());
        }
      }
      return self.clients.openWindow(target);
    })
  );
});
