'use strict';
const { test } = require('node:test');
const assert = require('node:assert/strict');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const ts = require('typescript');
const source = ts.transpileModule(
 readFileSync(path.join(__dirname,'../lib/coreRoutes.ts'),'utf8'),
 {compilerOptions:{module:ts.ModuleKind.CommonJS,target:ts.ScriptTarget.ES2022}}
).outputText;
const moduleExports={};
vm.runInNewContext(source,{exports:moduleExports});
const {corePageAllowed}=moduleExports;
test('only first-release pages are reachable',()=>{
 for (const page of ['/','/login','/onboarding','/welcome','/me','/requests',
                      '/settings/language','/settings/profile','/settings/security',
                      '/auth/callback','/auth/yandex-complete','/admin/security']) {
  assert.equal(corePageAllowed(page),true,page);
 }
 for (const page of ['/discover','/create','/groups/new','/channels',
                      '/communities/new','/post/123','/notifications','/search',
                      '/settings/traffic','/u/user','/join/token','/report']) {
  assert.equal(corePageAllowed(page),false,page);
 }
});
test('public app shell does not show unfinished features',()=>{
 const shell=readFileSync(path.join(__dirname,'../components/AppShell.tsx'),'utf8');
 for(const banned of ['/discover','/channels','/groups','/create','/notifications','/feed']){
  assert.equal(shell.includes('href="'+banned),false,banned);
 }
});
