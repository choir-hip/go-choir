// Deployed acceptance for the account switcher
// (docs/design/account-switcher-2026-11-09.md). One browser signs up two
// accounts with a virtual authenticator, then lists, switches, signs one out
// and signs out of all, against the real auth service. API level only: no
// desktop is opened, so no computer is woken.
//
//   BASE_URL=https://choir.news npx playwright test tests/account-switcher-deployed.spec.js
//
// Writes a JSON receipt to ACCOUNT_SWITCHER_RECEIPT when set.
import fs from 'node:fs';
import { expect } from '@playwright/test';
import { test } from './helpers/fixtures.js';
import { registerPasskey } from './helpers/auth.js';

const BASE_URL = process.env.BASE_URL || 'https://choir.news';
const RECEIPT = process.env.ACCOUNT_SWITCHER_RECEIPT || '';

// The auth service allows 10 requests a minute per address
// (internal/auth/rate_limit.go); a 429 waits out the window and retries.
async function api(page, method, path, body) {
  for (let i = 0; i < 4; i++) {
    const r = await apiOnce(page, method, path, body);
    if (r.status !== 429) return r;
    await page.waitForTimeout(61_000);
  }
  throw new Error(`${method} ${path}: still rate limited`);
}

async function paced(fn) {
  for (let i = 0; i < 4; i++) {
    try {
      return await fn();
    } catch (err) {
      if (!/429|too many auth requests/.test(String(err))) throw err;
      await new Promise((r) => setTimeout(r, 61_000));
    }
  }
  throw new Error('still rate limited');
}

async function sessionUser(page) {
  const r = await api(page, 'GET', '/auth/session');
  return r.body || {};
}

async function apiOnce(page, method, path, body) {
  return page.evaluate(async ({ method, url, body }) => {
    const init = { method, credentials: 'include', headers: { Accept: 'application/json' } };
    if (body !== undefined) {
      init.headers['Content-Type'] = 'application/json';
      init.body = JSON.stringify(body);
    }
    const res = await fetch(url, init);
    let json = null;
    try { json = await res.json(); } catch (_err) { json = null; }
    return { status: res.status, body: json };
  }, { method, url: `${BASE_URL}${path}`, body });
}

const members = (list) => (Array.isArray(list) ? list : list?.accounts || []);
const activeOf = (list) => members(list).find((m) => m.active)?.user_id || '';

test('two accounts in one browser: list, switch, sign one out, sign out of all', async ({ page, authenticator }) => {
  test.setTimeout(900_000);
  void authenticator;
  const tag = `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
  const emailA = `acct-switch-a-${tag}@example.com`;
  const emailB = `acct-switch-b-${tag}@example.com`;
  const receipt = { base_url: BASE_URL, steps: [] };
  const step = (name, data) => receipt.steps.push({ name, at: new Date().toISOString(), ...data });

  await page.goto(BASE_URL);

  // A signs up: a set with one active member.
  expect(await paced(() => registerPasskey(page, emailA, BASE_URL))).toBeTruthy();
  const a = (await sessionUser(page)).user.id;
  let list = await api(page, 'GET', '/auth/accounts');
  expect(list.status).toBe(200);
  expect(members(list.body).map((m) => m.user_id)).toEqual([a]);
  expect(activeOf(list.body)).toBe(a);
  step('signed_up_a', { user_a: a, members: members(list.body).length });

  // B signs up in the same browser: both are members, B is active.
  expect(await paced(() => registerPasskey(page, emailB, BASE_URL))).toBeTruthy();
  const b = (await sessionUser(page)).user.id;
  expect(b).not.toBe(a);
  list = await api(page, 'GET', '/auth/accounts');
  expect(members(list.body).map((m) => m.user_id).sort()).toEqual([a, b].sort());
  expect(activeOf(list.body)).toBe(b);
  step('signed_up_b', { user_b: b, members: members(list.body).length });

  // Switch to A, then back to B: the session follows each switch.
  let sw = await api(page, 'POST', '/auth/accounts/switch', { user_id: a });
  expect(sw.status, JSON.stringify(sw.body)).toBe(200);
  expect((await sessionUser(page)).user.id).toBe(a);
  expect(activeOf((await api(page, 'GET', '/auth/accounts')).body)).toBe(a);
  sw = await api(page, 'POST', '/auth/accounts/switch', { user_id: b });
  expect(sw.status).toBe(200);
  expect((await sessionUser(page)).user.id).toBe(b);
  // A switch to the member already active rotates and still resolves it.
  sw = await api(page, 'POST', '/auth/accounts/switch', { user_id: b });
  expect(sw.status).toBe(200);
  expect((await sessionUser(page)).user.id).toBe(b);
  step('switched', { a_then_b_then_b: true });

  // A user id that is not a member is refused.
  const stranger = await api(page, 'POST', '/auth/accounts/switch', { user_id: 'not-a-member' });
  expect(stranger.status).toBe(403);
  // A POST without a JSON content type is refused.
  await page.waitForTimeout(61_000);
  const plain = await page.evaluate(async (url) => (await fetch(url, { method: 'POST', credentials: 'include', body: 'user_id=x' })).status, `${BASE_URL}/auth/accounts/switch`);
  expect(plain).toBe(415);
  step('refusals', { non_member: stranger.status, non_json: plain });

  // Sign out the inactive member A: B stays signed in, the list has only B.
  const so = await api(page, 'POST', '/auth/accounts/signout', { user_id: a });
  expect(so.status, JSON.stringify(so.body)).toBe(200);
  expect((await sessionUser(page)).user.id).toBe(b);
  list = await api(page, 'GET', '/auth/accounts');
  expect(members(list.body).filter((m) => m.signed_in !== false).map((m) => m.user_id)).toEqual([b]);
  step('signed_out_a', { remaining: members(list.body).length });

  // Sign out of all: the browser is signed out.
  const all = await api(page, 'POST', '/auth/accounts/logout-all', {});
  expect(all.status).toBe(200);
  const after = await sessionUser(page);
  expect(after.authenticated).toBe(false);
  step('logout_all', { authenticated: after.authenticated });

  receipt.result = 'pass';
  if (RECEIPT) fs.writeFileSync(RECEIPT, JSON.stringify(receipt, null, 2) + '\n');
});
