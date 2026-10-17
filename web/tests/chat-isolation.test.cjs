'use strict';
const {test}=require('node:test');const assert=require('node:assert/strict');
const fs=require('node:fs');const ts=require('typescript');const vm=require('node:vm');
const src=ts.transpileModule(fs.readFileSync(require('node:path').join(__dirname,'../lib/chatIsolation.ts'),'utf8'),{compilerOptions:{module:ts.ModuleKind.CommonJS}}).outputText;
const exports={};vm.runInNewContext(src,{exports});
test('private chat isolation during selection changes',()=>{
 const a=[{chat_id:'A',body:'secretA'}],b=[{chat_id:'B',body:'secretB'}];
 assert.equal(exports.scopedChatMessages(a,'B').length,0);
 assert.equal(exports.scopedChatMessages([...a,...b],'B').length,1);
 assert.equal(exports.canCacheChatMessages([...a,...b],'B'),false);
 assert.equal(exports.canCacheChatMessages(b,'B'),true);
 assert.equal(exports.scopedChatMessages(a,undefined).length,0);
});
