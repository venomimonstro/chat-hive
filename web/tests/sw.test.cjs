'use strict';

const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const { join } = require('node:path');
const vm = require('node:vm');

function worker() {
  const handlers = new Map();
  const context = vm.createContext({
    URL,
    self: {
      location: { origin: 'https://chat.example' },
      addEventListener(type, handler) { handlers.set(type, handler); }
    },
    caches: {},
    fetch() { throw new Error('Unexpected network request'); }
  });
  const source = readFileSync(join(__dirname, '..', 'public', 'sw.js'), 'utf8');
  vm.runInContext(source, context, { filename: 'sw.js' });
  return { handlers, context };
}

test('private pages never enter the service-worker response cache', () => {
  const { handlers } = worker();
  for (const path of ['/', '/me', '/notifications', '/?chat=private-id', '/login', '/api/v1/chats']) {
    let intercepted = false;
    handlers.get('fetch')({
      request: { method: 'GET', mode: 'navigate', url: 'https://chat.example' + path },
      respondWith() { intercepted = true; }
    });
    assert.equal(intercepted, false, path);
  }
});

test('API and cross-origin requests are never cached even as subresources', () => {
  const { handlers } = worker();
  for (const path of ['https://chat.example/api/v1/me/profile', 'https://foreign.example/_next/static/a.js']) {
    let intercepted = false;
    handlers.get('fetch')({
      request: { method: 'GET', mode: 'cors', url: path },
      respondWith() { intercepted = true; }
    });
    assert.equal(intercepted, false, path);
  }
});

test('no future push notifications can disclose private chat details in core', () => {
  const { handlers } = worker();
  assert.equal(handlers.has('push'), false);
  assert.equal(handlers.has('notificationclick'), false);
});
