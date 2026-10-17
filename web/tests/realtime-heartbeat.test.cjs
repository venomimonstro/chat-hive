'use strict';

const { test } = require('node:test');
const assert = require('node:assert/strict');
const { readFileSync } = require('node:fs');
const { join } = require('node:path');
const vm = require('node:vm');
const ts = require('typescript');

const source = ts.transpileModule(
  readFileSync(join(__dirname, '..', 'lib', 'realtime.ts'), 'utf8'),
  { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 } }
).outputText;

function harness() {
  const sockets = [], events = [], timers = new Map();
  let nextTimer = 1, now = 0;
  class FakeWebSocket {
    static CONNECTING = 0;
    static OPEN = 1;
    readyState = 0;
    sent = [];
    closes = [];
    constructor(url) { this.url = url; sockets.push(this); }
    send(message) { this.sent.push(JSON.parse(message)); }
    close(code, reason) {
      this.closes.push({ code, reason });
      this.readyState = 3;
      this.onclose?.();
    }
  }
  const moduleExports = {};
  const window = {
    location: { origin: 'https://chat.example' },
    setInterval(callback) { const id = nextTimer++; timers.set(id, { callback, kind: 'interval' }); return id; },
    clearInterval(id) { timers.delete(id); },
    setTimeout(callback) { const id = nextTimer++; timers.set(id, { callback, kind: 'timeout' }); return id; },
    clearTimeout(id) { timers.delete(id); },
    dispatchEvent(event) { events.push(event); }
  };
  vm.runInNewContext(source, {
    exports: moduleExports,
    require(name) {
      if (name === './api') return { getAccessToken: () => 'token', refreshSession: async () => null };
      throw new Error('Unexpected import: ' + name);
    },
    process: { env: { NEXT_PUBLIC_API_BASE_URL: '' } },
    window,
    WebSocket: FakeWebSocket,
    URL,
    URLSearchParams,
    Date: class extends Date { static now() { return now; } },
    CustomEvent: class { constructor(type, options) { this.type = type; this.detail = options.detail; } },
    fetch: async () => ({ status: 201, ok: true, json: async () => ({ ticket: 'abc' }) })
  }, { filename: 'realtime.js' });
  return {
    client: new moduleExports.RealtimeClient(), sockets, events,
    advance(ms) { now += ms; for (const item of [...timers.values()]) if (item.kind === 'interval') item.callback(); },
    timers
  };
}

test('missing pongs force a reconnect instead of leaving a stale connected socket', async () => {
  const env = harness();
  env.client.start();
  await new Promise((resolve) => setImmediate(resolve));
  assert.equal(env.sockets.length, 1);
  const socket = env.sockets[0];
  socket.readyState = 1;
  socket.onopen();
  env.advance(25_000);
  assert.equal(socket.sent.length, 1);
  assert.equal(socket.closes.length, 0);
  env.advance(80_000);
  assert.equal(socket.closes[0].reason, 'heartbeat timeout');
  assert.ok(env.events.some((event) => event.type === 'chat:connection' && event.detail === false));
  env.client.stop();
});

test('pongs and ready packets are not broadcast as chat events', async () => {
  const env = harness();
  env.client.start();
  await new Promise((resolve) => setImmediate(resolve));
  const socket = env.sockets[0];
  socket.readyState = 1;
  socket.onopen();
  socket.onmessage({ data: JSON.stringify({ type: 'ready' }) });
  env.advance(30_000);
  socket.onmessage({ data: JSON.stringify({ type: 'pong' }) });
  env.advance(60_000);
  assert.equal(socket.closes.length, 0);
  socket.onmessage({ data: JSON.stringify({ type: 'message.created', chat_id: 'x' }) });
  assert.equal(env.events.filter((event) => event.type === 'chat:realtime').length, 1);
  env.client.stop();
});

test('a stuck WebSocket handshake is closed and reconnect scheduled', async () => {
  const env = harness();
  env.client.start();
  await new Promise((resolve) => setImmediate(resolve));
  assert.equal(env.sockets.length, 1);
  const socket = env.sockets[0];
  assert.equal(socket.readyState, 0);
  const timer = Array.from(env.timers.values()).find((entry) => entry.kind === 'timeout');
  assert.ok(timer, 'handshake watchdog must be scheduled');
  timer.callback();
  assert.equal(socket.closes[0]?.reason, 'handshake timeout');
  assert.ok(Array.from(env.timers.values()).some((entry) => entry.kind === 'timeout'),
    'reconnect must be scheduled after a timed-out handshake');
  env.client.stop();
});
