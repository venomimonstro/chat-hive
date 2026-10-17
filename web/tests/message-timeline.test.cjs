'use strict';
const { test } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const ts = require('typescript');
const source = ts.transpileModule(fs.readFileSync(path.join(__dirname,'../lib/messageTimeline.ts'),'utf8'),
  {compilerOptions:{module:ts.ModuleKind.CommonJS,target:ts.ScriptTarget.ES2022}}).outputText;
const exports = {};
vm.runInNewContext(source,{exports});
const merge = exports.reconcileTimeline;
const uid='alice', cid='chat-a';
const msg=(id,sequence,chat_id=cid)=>({id,chat_id,client_message_id:id,sequence,type:'text',body:id,created_at:'2026-10-17T12:00:00Z'});
const pending=(id,chat_id=cid,user_id=uid)=>({chat_id,user_id,client_message_id:id,body:'draft',created_at:'2026-10-17T12:00:00Z'});
const optimistic=(id,status='sending')=>({...msg('local-'+id,Number.MAX_SAFE_INTEGER),client_message_id:id,local_status:status});
test('concurrent optimistic message survives delayed IndexedDB snapshot',()=>{
  const out=merge([optimistic('m')],[msg('persisted',1)],[],uid,cid);
  assert.deepEqual(Array.from(out.map(x=>x.client_message_id)),['persisted','m']);
});
test('failed message status survives queued retry reconciliation',()=>{
  const out=merge([optimistic('m','failed')],[msg('persisted',1)],[pending('m')],uid,cid);
  assert.equal(out[1].local_status,'failed');
});
test('server acknowledgment replaces pending message with no duplicate',()=>{
  const out=merge([optimistic('m')],[msg('m',3)],[pending('m')],uid,cid);
  assert.equal(out.length,1);assert.equal(out[0].sequence,3);assert.equal(out[0].local_status,undefined);
});
test('another chat and another account cannot enter the timeline',()=>{
  const out=merge([msg('foreign',3,'chat-b')],[msg('ok',1)],
    [pending('mine'),pending('theirs',cid,'bob'),pending('other','chat-b')],uid,cid);
  assert.deepEqual(Array.from(out.map(x=>x.client_message_id)),['ok','mine']);
});
test('recovery retains older pages and newer acknowledged messages',()=>{
  const out=merge([msg('old',1),msg('recent',20)],[msg('middle',11),msg('prior',10)],[],uid,cid,true);
  assert.deepEqual(Array.from(out.map(x=>x.sequence)),[1,10,11,20]);
});
