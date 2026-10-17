'use strict';
const { test } = require('node:test');
const assert = require('node:assert/strict');
const { readFileSync } = require('node:fs');
const { join } = require('node:path');
// Lightweight source-contract assertions complement the full TypeScript
// regression suite and catch accidental preference waterfalls.
const source = readFileSync(join(__dirname, '../lib/language.ts'), 'utf8');
test('language settings cache requests per account, not across users', () => {
  assert.match(source, /accountLanguageUser === userID/);
  assert.match(source, /loadAccountLanguage\(session\.user_id\)/);
  assert.match(source, /accountLanguageUser = session\.user_id/);
  assert.match(source, /await saveLanguagePreference\(next\)/);
});
