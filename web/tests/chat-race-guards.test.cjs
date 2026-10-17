'use strict';
const {test}=require('node:test');
const assert=require('node:assert/strict');
const {readFileSync}=require('node:fs');
const {join}=require('node:path');
const source=readFileSync(join(__dirname,'../app/ChatsClient.tsx'),'utf8');
// These lightweight source invariants protect against reintroducing a UI
// cross-account disclosure when a slow network response finishes late.
// End-to-end tests still need to exercise actual accounts and browser state.
test('late history errors cannot appear in another selected conversation',()=>{
  assert.match(source,/active && selectedIdRef\.current === selected\.chat_id/);
  assert.match(source,/if \(selectedIdRef\.current !== chatID \|\| accountRef\.current !== owner\) return/);
});
test('pending message delivery cannot render into switched user account',()=>{
  assert.match(source,/accountRef\.current === item\.user_id && selectedIdRef\.current === item\.chat_id/);
  assert.match(source,/if \(delivered > 0 && items\[0\]\?\.user_id === accountRef\.current\)/);
});

test('mutations ignore completions belonging to another conversation or account',()=>{
  assert.match(source,/accountRef\.current === item\.user_id && selectedIdRef\.current === item\.chat_id/);
  assert.match(source,/selectedIdRef\.current === originalChat && accountRef\.current === originalOwner/);
  assert.match(source,/selectedIdRef\.current !== originalChat \|\| accountRef\.current !== originalOwner/);
});
test('queued text is not cleared if the user changed chats during storage write',()=>{
  assert.match(source,/await enqueueOutbox\(item\)/);
  assert.match(source,/if \(accountRef\.current === item\.user_id && selectedIdRef\.current === item\.chat_id\) \{/);
});
