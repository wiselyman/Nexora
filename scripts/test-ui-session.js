const assert = require('node:assert/strict');
const { initializeUISession } = require('../internal/ui/assets/ui-session.js');

const history = { replaceState() {} };
const directVisit = initializeUISession(
  { hash: '', pathname: '/', search: '' },
  history,
  'server-injected-token',
);
assert.equal(directVisit.token, 'server-injected-token');
directVisit.clear();
assert.equal(directVisit.token, '');

let replaced;
const legacyLaunch = initializeUISession(
  { hash: '#ui_session=legacy-token', pathname: '/', search: '' },
  { replaceState(...args) { replaced = args; } },
  'server-injected-token',
);
assert.equal(legacyLaunch.token, 'legacy-token');
assert.deepEqual(replaced, [null, '', '/']);

const noCredential = initializeUISession(
  { hash: '', pathname: '/', search: '' },
  history,
  '',
);
assert.equal(noCredential.token, '');
console.log('UI session bootstrap checks passed.');
