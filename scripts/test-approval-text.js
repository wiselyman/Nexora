const assert = require('node:assert/strict');
const displayApprovalText = require('../internal/ui/assets/approval-text.js');

assert.deepEqual(displayApprovalText('echo hello'), { text: 'echo hello', escaped: false });
assert.deepEqual(displayApprovalText('echo\tfirst\nsecond'), { text: 'echo\tfirst\nsecond', escaped: false });
assert.deepEqual(displayApprovalText('echo safe\u202Egpj.sh'), { text: 'echo safe\\u{202E}gpj.sh', escaped: true });
assert.deepEqual(displayApprovalText('echo\u001B[2J\u0000'), { text: 'echo\\u{001B}[2J\\u{0000}', escaped: true });
console.log('Approval text visibility checks passed.');
