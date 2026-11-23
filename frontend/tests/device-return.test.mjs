// node --test tests/device-return.test.mjs
//
// FAILURE LIST (written before the code). The /device page may close the phone's
// sign-in sheet after an approval (docs/reference/exact/auth-session.md). It sends
// the app's own `return` (a scheme the SERVER allowlists) and `state` from its
// link, and follows the server's `return_to`, nothing else.
//
//  P1  No `return`/`state` in the link: the request body is exactly as before
//      (user_code, decision) and nothing is followed.
//  P2  Both in the link: they are sent in the body, as strings, unchanged.
//  P3  One without the other: neither is sent (a half link is a normal link).
//  P4  A `return_to` of the one shape `<scheme>:/signed-in?state=<token>` is
//      followed; anything else is not: http(s), javascript, data, a host, a
//      path, an extra parameter, a fragment, a missing state, a non-string.
//  P5  The target is followed only when it carries the state the page sent.
import test from 'node:test';
import assert from 'node:assert/strict';
import { approveRequestBody, followableReturn, hasReturnHint, returnTarget } from '../src/lib/device-return.js';

const STATE = 'k3VqS0m9Zt_xA1b2C3d4-E5f';

test('P1 a normal link sends the body it always sent', () => {
  assert.deepEqual(approveRequestBody(' ABCD-EFGH ', 'approve', ''), { user_code: 'ABCD-EFGH', decision: 'approve' });
  assert.deepEqual(approveRequestBody('ABCD-EFGH', 'deny', '?code=ABCD-EFGH'), { user_code: 'ABCD-EFGH', decision: 'deny' });
});

test('P2 the app link sends return and state', () => {
  const q = `?code=ABCD-EFGH&return=news.choir.autoradio.lab.one&state=${STATE}`;
  assert.deepEqual(approveRequestBody('ABCD-EFGH', 'approve', q), {
    user_code: 'ABCD-EFGH', decision: 'approve', return: 'news.choir.autoradio.lab.one', state: STATE,
  });
});

test('P3 half a link is a normal link', () => {
  assert.deepEqual(approveRequestBody('A', 'approve', `?return=news.choir.autoradio.lab`), { user_code: 'A', decision: 'approve' });
  assert.deepEqual(approveRequestBody('A', 'approve', `?state=${STATE}`), { user_code: 'A', decision: 'approve' });
});

test('P4 only the one shape is followed', () => {
  const ok = `news.choir.autoradio.lab:/signed-in?state=${STATE}`;
  assert.equal(followableReturn({ return_to: ok }, STATE, 'news.choir.autoradio.lab'), ok);
  assert.equal(followableReturn({ return_to: `news.choir.autoradio.lab.one:/signed-in?state=${STATE}` }, STATE, 'news.choir.autoradio.lab.one'), `news.choir.autoradio.lab.one:/signed-in?state=${STATE}`);
  const bad = [
    `https://evil.example/signed-in?state=${STATE}`, `http://x:/signed-in?state=${STATE}`, `javascript:/signed-in?state=${STATE}`,
    `data:/signed-in?state=${STATE}`, `file:/signed-in?state=${STATE}`, `news.choir.autoradio.lab://evil/signed-in?state=${STATE}`,
    `news.choir.autoradio.lab:/other?state=${STATE}`, `news.choir.autoradio.lab:/signed-in?state=${STATE}&code=x`,
    `news.choir.autoradio.lab:/signed-in?state=${STATE}#x`, 'news.choir.autoradio.lab:/signed-in', `news.choir.autoradio.lab:/signed-in?state=`,
    ` ${ok}`, '', null, undefined, 42, { x: 1 },
  ];
  for (const b of bad) assert.equal(followableReturn({ return_to: b }, STATE, 'news.choir.autoradio.lab'), null, String(b));
  assert.equal(followableReturn({}, STATE, 'news.choir.autoradio.lab'), null);
  assert.equal(followableReturn(null, STATE, 'news.choir.autoradio.lab'), null);
});

test('P5 only with the state the page sent', () => {
  const ok = `news.choir.autoradio.lab:/signed-in?state=${STATE}`;
  assert.equal(followableReturn({ return_to: ok }, 'another-state-aaaaaaaaaa', 'news.choir.autoradio.lab'), null);
  assert.equal(followableReturn({ return_to: ok }, '', 'news.choir.autoradio.lab'), null);
});

test('P6 only the scheme the page sent', () => {
  const sibling = `news.choir.autoradio.lab.one:/signed-in?state=${STATE}`;
  assert.equal(followableReturn({ return_to: sibling }, STATE, 'news.choir.autoradio.lab'), null);
  for (const s of ['tel', 'intent', 'mailto', 'shortcuts', 'market'])
    assert.equal(followableReturn({ return_to: `${s}:/signed-in?state=${STATE}` }, STATE, 'news.choir.autoradio.lab'), null, s);
  assert.equal(followableReturn({ return_to: `news.choir.autoradio.lab:/signed-in?state=${STATE}` }, STATE, ''), null);
  assert.equal(followableReturn({ return_to: `news.choir.autoradio.lab:/signed-in?state=${STATE}` }, STATE, undefined), null);
});

test('P7 a link from the app sheet is told from a normal one', () => {
  assert.equal(hasReturnHint(`?code=A&return=news.choir.autoradio.lab&state=${STATE}`), true);
  assert.equal(hasReturnHint('?code=A'), false);
  assert.equal(hasReturnHint(`?return=news.choir.autoradio.lab`), false);
  assert.equal(hasReturnHint(''), false);
});

test('P8 a hint that is not the server\'s shape is not sent (it could push the body over the 1 KiB cap)', () => {
  const plain = { user_code: 'A', decision: 'approve' };
  assert.deepEqual(approveRequestBody('A', 'approve', `?return=news.choir.autoradio.lab&state=${'a'.repeat(129)}`), plain);
  assert.deepEqual(approveRequestBody('A', 'approve', `?return=news.choir.autoradio.lab&state=short`), plain);
  assert.deepEqual(approveRequestBody('A', 'approve', `?return=${'a'.repeat(65)}&state=${STATE}`), plain);
  assert.deepEqual(approveRequestBody('A', 'approve', `?return=Bad Id!&state=${STATE}`), plain);
  assert.deepEqual(approveRequestBody('A', 'approve', `?return=news.choir.autoradio.lab&state=${'a'.repeat(5000)}`), plain);
  assert.equal(hasReturnHint(`?return=news.choir.autoradio.lab&state=${'a'.repeat(5000)}`), false);
  const max = 'a'.repeat(128);
  assert.equal(approveRequestBody('A', 'approve', `?return=news.choir.autoradio.lab&state=${max}`).state, max);
});

test('P9 only an approval is followed; a denial with a valid hint is not', () => {
  const q = `?code=A&return=news.choir.autoradio.lab&state=${STATE}`;
  const answer = { status: 'approved', return_to: `news.choir.autoradio.lab:/signed-in?state=${STATE}` };
  assert.equal(returnTarget('approve', answer, q), answer.return_to);
  assert.equal(returnTarget('deny', answer, q), null);
  assert.equal(returnTarget('deny', { status: 'denied' }, q), null);
  assert.equal(returnTarget('approve', answer, ''), null);
});
