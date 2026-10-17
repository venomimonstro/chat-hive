'use strict';
const {test}=require('node:test');
const assert=require('node:assert/strict');
const {readFileSync}=require('node:fs');
const {join}=require('node:path');
const read=(p)=>readFileSync(join(__dirname,'..',p),'utf8');

test('chat switches do not persist message history or drafts in storage',()=>{
 const source=read('app/ChatsClient.tsx');
 assert.match(source,/historyCacheRef/);
 assert.match(source,/draftsRef/);
 assert.match(source,/historyCacheRef\.current\.clear\(\)/);
 assert.match(source,/draftsRef\.current\.clear\(\)/);
 assert.doesNotMatch(source,/localStorage\.setItem\(|sessionStorage\.setItem\(/);
});

test('message send uses durable outbox before delivery and an IME-safe submit',()=>{
 const source=read('app/ChatsClient.tsx');
 const queue=source.indexOf('await enqueueOutbox(item)');
 const deliver=source.indexOf('if (navigator.onLine) await deliver(item)');
 assert.ok(queue>=0 && deliver>queue, 'message must be persisted before sending');
 assert.match(source,/!e\.nativeEvent\.isComposing/);
 assert.match(source,/createPendingRef\.current/);
 assert.match(source,/ref=\{messageListRef\}/);
});

test('release scope excludes push and new social endpoints',()=>{
 const worker=read('public/sw.js');
 const shell=read('components/AppShell.tsx');
 assert.doesNotMatch(worker,/addEventListener\(['"]push['"]/);
 for(const section of ['channels','groups','discover','feed']){
   assert.doesNotMatch(shell,new RegExp('href=["\\x27]/'+section));
 }
});

test('cached language preferences are invalidated on auth account change',()=>{
 const source=read('lib/language.ts');
 assert.match(source,/const onAccountChange = \(\) =>/);
 assert.match(source,/accountLanguagePromise = null/);
 assert.match(source,/window\.addEventListener\('chat:auth-changed', onAccountChange\)/);
});
