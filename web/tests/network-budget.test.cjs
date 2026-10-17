'use strict';
const { test } = require('node:test');
const assert = require('node:assert/strict');
const { readFileSync } = require('node:fs');
const { join } = require('node:path');
const vm = require('node:vm');
const ts = require('typescript');
const source = ts.transpileModule(
  readFileSync(join(__dirname, '../lib/networkBudget.ts'), 'utf8'),
  { compilerOptions: {module:ts.ModuleKind.CommonJS,target:ts.ScriptTarget.ES2022} }
).outputText;
const moduleExports={};
vm.runInNewContext(source,{exports:moduleExports,navigator:{connection:{saveData:true}}});
const {messagePageSize,currentMessagePageSize}=moduleExports;
test('limits history on metered or 2G connections without dropping pagination',()=>{
  assert.equal(messagePageSize(undefined),40);
  assert.equal(messagePageSize({saveData:true}),24);
  assert.equal(messagePageSize({effectiveType:'2g'}),24);
  assert.equal(messagePageSize({effectiveType:'slow-2g'}),24);
  assert.equal(messagePageSize({effectiveType:'4g'}),40);
  assert.equal(currentMessagePageSize(),24);
});
