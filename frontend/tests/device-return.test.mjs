// node --test tests/device-return.test.mjs
//
// FAILURE LIST (written before the code). The /device page may close the phone's
// sign-in sheet after a decision (docs/reference/exact/auth-session.md). It sends
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
import { approveRequestBody, followableReturn } from '../src/lib/device-return.js';

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
  assert.equal(followableReturn({ return_to: ok }, STATE), ok);
  assert.equal(followableReturn({ return_to: `news.choir.autoradio.lab.one:/signed-in?state=${STATE}` }, STATE), `news.choir.autoradio.lab.one:/signed-in?state=${STATE}`);
  const bad = [
    `https://evil.example/signed-in?state=${STATE}`, `http://x:/signed-in?state=${STATE}`, `javascript:/signed-in?state=${STATE}`,
    `data:/signed-in?state=${STATE}`, `file:/signed-in?state=${STATE}`, `news.choir.autoradio.lab://evil/signed-in?state=${STATE}`,
    `news.choir.autoradio.lab:/other?state=${STATE}`, `news.choir.autoradio.lab:/signed-in?state=${STATE}&code=x`,
    `news.choir.autoradio.lab:/signed-in?state=${STATE}#x`, 'news.choir.autoradio.lab:/signed-in', `news.choir.autoradio.lab:/signed-in?state=`,
    ` ${ok}`, '', null, undefined, 42, { x: 1 },
  ];
  for (const b of bad) assert.equal(followableReturn({ return_to: b }, STATE), null, String(b));
  assert.equal(followableReturn({}, STATE), null);
  assert.equal(followableReturn(null, STATE), null);
});

test('P5 only with the state the page sent', () => {
  const ok = `news.choir.autoradio.lab:/signed-in?state=${STATE}`;
  assert.equal(followableReturn({ return_to: ok }, 'another-state-aaaaaaaaaa'), null);
  assert.equal(followableReturn({ return_to: ok }, ''), null);
});
