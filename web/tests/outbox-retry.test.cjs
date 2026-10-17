'use strict';
const { test } = require('node:test');
const assert = require('node:assert/strict');
const { readFileSync } = require('node:fs');
const { join } = require('node:path');
const vm = require('node:vm');
const ts = require('typescript');
const source = ts.transpileModule(
  readFileSync(join(__dirname, '../lib/outboxRetry.ts'), 'utf8'),
  { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 } }
).outputText;
const exports = {};
vm.runInNewContext(source, { exports });
const { nextOutboxRetry, readyForOutboxRetry } = exports;

test('automatic failures back off exponentially instead of every polling tick', () => {
  let retry;
  let now = 1000;
  const intervals = [];
  for (let i = 0; i < 10; i++) {
    retry = nextOutboxRetry(retry, now, 1);
    intervals.push(retry.nextRetryAt - now);
    assert.equal(readyForOutboxRetry(retry, now + 499), false);
    assert.equal(readyForOutboxRetry(retry, retry.nextRetryAt), true);
    now = retry.nextRetryAt;
  }
  assert.deepEqual(Array.from(intervals), [5000, 10000, 20000, 40000, 80000, 160000, 300000, 300000, 300000, 300000]);
});

test('retry jitter is bounded and never yields invalid scheduling values', () => {
  const minimum = nextOutboxRetry(undefined, 0, -999);
  const maximum = nextOutboxRetry(undefined, 0, 999);
  const invalid = nextOutboxRetry(undefined, 0, NaN);
  assert.equal(minimum.nextRetryAt, 4000);
  assert.equal(maximum.nextRetryAt, 5000);
  assert.equal(invalid.nextRetryAt, 4000);
});

test('a missing state is immediately eligible after reconnection or manual retry', () => {
  assert.equal(readyForOutboxRetry(undefined, 0), true);
});
