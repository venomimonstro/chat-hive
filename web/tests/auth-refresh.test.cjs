'use strict';

const { test } = require('node:test');
const assert = require('node:assert/strict');
const { readFileSync } = require('node:fs');
const { join } = require('node:path');
const vm = require('node:vm');
const ts = require('typescript');

const source = ts.transpileModule(
  readFileSync(join(__dirname, '..', 'lib', 'api.ts'), 'utf8'),
  { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 } }
).outputText;

function createApp(fetchHandler, locks) {
  const stored = new Map();
  const exports = {};
  const context = {
    exports,
    process: { env: { NEXT_PUBLIC_API_BASE_URL: '' } },
    fetch: fetchHandler,
    Headers: globalThis.Headers,
    navigator: locks ? { locks } : {},
    window: {
      sessionStorage: {
        getItem: (key) => stored.get(key) ?? null,
        setItem: (key, value) => stored.set(key, value),
        removeItem: (key) => stored.delete(key)
      }
    }
  };
  vm.runInNewContext(source, context, { filename: 'api.js' });
  return exports;
}

function lockQueue() {
  let tail = Promise.resolve();
  return {
    request(_name, _options, callback) {
      const before = tail;
      let release;
      tail = new Promise((resolve) => { release = resolve; });
      return before.then(async () => {
        try { return await callback(); } finally { release(); }
      });
    }
  };
}

test('same-tab refreshes share one cookie-rotation request', async () => {
  let calls = 0;
  const app = createApp(async () => {
    calls++;
    await new Promise((resolve) => setTimeout(resolve, 10));
    return { status: 200, ok: true, json: async () => ({ access_token: 'fresh' }) };
  });
  const [a, b] = await Promise.all([app.refreshSession(), app.refreshSession()]);
  assert.equal(calls, 1);
  assert.equal(a.access_token, 'fresh');
  assert.equal(b.access_token, 'fresh');
});

test('tabs serialize rotating refresh-token calls with Web Locks', async () => {
  let active = 0;
  let peak = 0;
  let calls = 0;
  const locks = lockQueue();
  const request = async () => {
    calls++;
    active++;
    peak = Math.max(peak, active);
    await new Promise((resolve) => setTimeout(resolve, 10));
    active--;
    return { status: 200, ok: true, json: async () => ({ access_token: 'fresh' }) };
  };
  const first = createApp(request, locks);
  const second = createApp(request, locks);
  await Promise.all([first.refreshSession(), second.refreshSession()]);
  assert.equal(calls, 2);
  assert.equal(peak, 1);
});

test('temporary server outages preserve the last access token', async () => {
  const app = createApp(async () => ({ status: 503, ok: false }));
  app.setAccessToken('existing');
  await assert.rejects(app.refreshSession(), /временно недоступен/);
  assert.equal(app.getAccessToken(), 'existing');
});

test('profile service outage never returns a null onboarding profile', async () => {
  const app = createApp(async () => ({
    status: 503, ok: false,
    json: async () => ({ error: { code: 'temporarily_unavailable', message: 'Try again later' } })
  }));
  await assert.rejects(app.getProfile(), /Try again later/);
});

test('incomplete onboarding profile remains a valid API response', async () => {
  const profile = { user_id: 'u', username: '', display_name: '', interests: [], bio: '', completed: false };
  const app = createApp(async () => ({ status: 200, ok: true, json: async () => profile }));
  const result = await app.getProfile();
  assert.equal(result.completed, false);
  assert.equal(result.user_id, profile.user_id);
});

test('logout while refresh is in flight never restores an old access token', async () => {
  let finish;
  const app = createApp(() => new Promise((resolve) => { finish = resolve; }));
  app.setAccessToken('old');
  const pending = app.refreshSession();
  app.setAccessToken(null);
  finish({ status: 200, ok: true, json: async () => ({ access_token: 'stale' }) });
  assert.equal(await pending, null);
  assert.equal(app.getAccessToken(), null);
});

test('account switch during refresh is not overwritten by an old account', async () => {
  let finish;
  const app = createApp(() => new Promise((resolve) => { finish = resolve; }));
  app.setAccessToken('old-account');
  const pending = app.refreshSession();
  app.setAccessToken('new-account');
  finish({ status: 200, ok: true, json: async () => ({ access_token: 'old-account-rotated' }) });
  assert.equal(await pending, null);
  assert.equal(app.getAccessToken(), 'new-account');
});

test('stale unauthorized refresh cannot clear a newer session', async () => {
  let finish;
  const app = createApp(() => new Promise((resolve) => { finish = resolve; }));
  app.setAccessToken('old-account');
  const pending = app.refreshSession();
  app.setAccessToken('new-account');
  finish({ status: 401, ok: false });
  assert.equal(await pending, null);
  assert.equal(app.getAccessToken(), 'new-account');
});

test('a temporarily unavailable session endpoint never logs the user out', async () => {
  const app = createApp(async () => ({
    status: 503, ok: false,
    json: async () => ({ error: { message: 'Try again later' } })
  }));
  app.setAccessToken('still-valid');
  await assert.rejects(app.getCurrentSession(), /временно недоступен/);
  assert.equal(app.getAccessToken(), 'still-valid');
});

test('an unauthorized session response is still treated as logged out', async () => {
  const app = createApp(async () => ({ status: 401, ok: false }));
  assert.equal(await app.getCurrentSession(), null);
});

test('access tokens are memory-only and never persisted in browser storage', () => {
  const app = createApp(async () => ({ status: 200, ok: true }));
  app.setAccessToken('private-token');
  assert.equal(app.getAccessToken(), 'private-token');
  // A fresh app instance cannot read the old tab's in-memory bearer token.
  // The HttpOnly refresh cookie is the browser's only session persistence.
  const fresh = createApp(async () => ({ status: 401, ok: false }));
  assert.equal(fresh.getAccessToken(), null);
});

test('same-page clients reuse short-lived session lookup instead of issuing redundant API calls', async () => {
  let calls = 0;
  const app = createApp(async () => {
    calls++;
    return { status: 200, ok: true, json: async () => ({ user_id: 'alice', session_id: 's' }) };
  });
  app.setAccessToken('token');
  const [a, b, c] = await Promise.all([app.getCurrentSession(), app.getCurrentSession(), app.getCurrentSession()]);
  assert.equal(calls, 1);
  assert.equal(a.user_id, 'alice');
  assert.equal(b.session_id, 's');
  assert.equal(c.user_id, 'alice');
});

test('account switch invalidates session lookup before a new account is read', async () => {
  let calls = 0;
  const app = createApp(async () => {
    calls++;
    return { status: 200, ok: true, json: async () => ({ user_id: String(calls), session_id: String(calls) }) };
  });
  app.setAccessToken('alice');
  assert.equal((await app.getCurrentSession()).user_id, '1');
  app.setAccessToken('bob');
  assert.equal((await app.getCurrentSession()).user_id, '2');
  assert.equal(calls, 2);
});

test('failed session request is not cached and succeeds on a retry', async () => {
  let calls = 0;
  const app = createApp(async () => {
    calls++;
    if (calls === 1) return { status: 503, ok: false };
    return { status: 200, ok: true, json: async () => ({ user_id: 'healthy' }) };
  });
  await assert.rejects(app.getCurrentSession(), /временно недоступен/);
  // Catch handler clears a rejected cached Promise in a microtask.
  await Promise.resolve();
  assert.equal((await app.getCurrentSession()).user_id, 'healthy');
});
