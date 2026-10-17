'use strict';

const { test } = require('node:test');
const assert = require('node:assert/strict');
const { readFileSync } = require('node:fs');
const { join } = require('node:path');
const vm = require('node:vm');
const ts = require('typescript');

const source = ts.transpileModule(
  readFileSync(join(__dirname, '..', 'lib', 'push.ts'), 'utf8'),
  { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 } }
).outputText;

function harness({ token, response }) {
  let unsubscribeCalls = 0;
  let deleteCalls = 0;
  const subscription = {
    endpoint: 'https://push.example.org/send/123',
    async unsubscribe() { unsubscribeCalls++; return true; }
  };
  const navigator = { serviceWorker: { ready: Promise.resolve({
    pushManager: { async getSubscription() { return subscription; } }
  }) } };
  const exports = {};
  const Headers = globalThis.Headers;
  vm.runInNewContext(source, {
    exports, navigator, Headers,
    process: { env: { NEXT_PUBLIC_API_BASE_URL: '' } },
    require(name) {
      if (name === './api') {
        return { getAccessToken: () => token ?? null, refreshSession: async () => null };
      }
      throw new Error('unexpected dependency: ' + name);
    },
    fetch: async () => { deleteCalls++; return response; }
  }, { filename: 'push.js' });
  return {
    disablePush: exports.disablePush,
    get unsubscribeCalls() { return unsubscribeCalls; },
    get deleteCalls() { return deleteCalls; }
  };
}

test('expired login does not falsely claim to disable server-side push', async () => {
  const env = harness({ token: null });
  await assert.rejects(env.disablePush(), /необходимо войти/);
  assert.equal(env.unsubscribeCalls, 0);
  assert.equal(env.deleteCalls, 0);
});

test('server outage leaves browser subscription intact for a safe retry', async () => {
  const env = harness({ token: 'ok', response: { status: 503, ok: false } });
  await assert.rejects(env.disablePush(), /Не удалось отключить/);
  assert.equal(env.deleteCalls, 1);
  assert.equal(env.unsubscribeCalls, 0);
});

test('browser unsubscribe runs only after server revocation succeeds', async () => {
  const env = harness({ token: 'ok', response: { status: 204, ok: true } });
  await env.disablePush();
  assert.equal(env.deleteCalls, 1);
  assert.equal(env.unsubscribeCalls, 1);
});
