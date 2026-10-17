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
