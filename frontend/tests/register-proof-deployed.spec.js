/**
 * Deployed proof that an account is made only for an address whose holder proved it
 * (docs/problems/passkey-claims-an-unproved-address-2026-11-24.md), run against staging.
 *
 * It needs a mailbox the run controls. The product has no mail-service test hook; what it has is
 * the numbered test address (`maildctl reserve-test-number`, 10,000,000..99,999,999 at choir.news)
 * on the public receive policy, whose mail lands in the quarantine folder of the account the number is
 * bound to (`maildctl bind-number`), and a read:runtime key of that account reads it
 * (scripts/e2e-email-signin-staging.sh does the same for sign-in). So this needs, set up once by the operator
 * and never by this spec:
 *   1. an existing drill account (d@e, the one adversarial drills run on) with a read:runtime key;
 *   2. a reserved test number bound to THAT account: ssh node-b maildctl reserve-test-number, then
 *      ssh node-b maildctl bind-number <number> <drill user id>.
 * The new account is then made for the numbered address as its email, and the code the server mails to it
 * arrives in the drill account's quarantine folder, where the key reads it.
 *
 * Env: E2E_REGISTER_ADDRESS (the number@choir.news), E2E_INBOX_KEY (the drill account's read:runtime key),
 * BASE_URL (default https://choir.news). Nothing secret is printed.
 *
 * NOT RUN: written for the landing loop; the gap is the one-time operator setup above, and that
 * the numbered address becomes an account of its own (it is spent: reserve a new number per run).
 */
import { expect, request as pwRequest } from '@playwright/test';
import { test } from './helpers/fixtures.js';
import { registerPasskey, requestRegisterCode, getSession } from './helpers/auth.js';

const BASE_URL = process.env.BASE_URL || 'https://choir.news';
const ADDRESS = process.env.E2E_REGISTER_ADDRESS || '';
const KEY = process.env.E2E_INBOX_KEY || '';

async function newestCode(api, seen) {
  for (const folder of ['quarantine', 'inbox']) {
    const res = await api.get(`/api/email/messages?folder=${folder}&limit=50`);
    if (res.status() !== 200) continue;
    for (const m of (await res.json()).messages || []) {
      if (seen.has(m.id)) continue;
      const detail = await api.get(`/api/email/messages/${m.id}`);
      if (detail.status() !== 200) continue;
      const found = /Your code:\s*(\d{8})/.exec((await detail.json()).text_body || '');
      if (found) { seen.add(m.id); return found[1]; }
    }
  }
  return '';
}

test('an account is made only after the mailed code and the passkey, and nothing exists before', async ({ page, authenticator }) => {
  test.setTimeout(300_000);
  expect(/^\d{8}@choir\.news$/.test(ADDRESS), 'E2E_REGISTER_ADDRESS must be a reserved test number').toBeTruthy();
  expect(KEY, 'E2E_INBOX_KEY is required').toBeTruthy();
  const api = await pwRequest.newContext({ baseURL: BASE_URL, extraHTTPHeaders: { Authorization: `Bearer ${KEY}` } });
  await page.goto(BASE_URL);
  const seen = new Set();

  // 1. No proof, no registration, and nothing is made.
  const bare = await page.evaluate(async ({ email }) => {
    const r = await fetch('/auth/register/begin', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ email }) });
    return { status: r.status, body: await r.json().catch(() => ({})) };
  }, { email: ADDRESS });
  expect(bare.status).toBe(400);
  expect(bare.body.code).toBe('proof_required');

  // 2. Ask for the code, read it from the mailbox, and a wrong code is refused.
  const ask = await requestRegisterCode(page, ADDRESS, BASE_URL);
  let code = '';
  for (let i = 0; i < 30 && !code; i++) { code = await newestCode(api, seen); if (!code) await page.waitForTimeout(5000); }
  expect(code, 'the code mail reached the drill account\'s mailbox').toMatch(/^\d{8}$/);
  const wrong = code === '00000000' ? '11111111' : '00000000';
  await expect(registerPasskey(page, ADDRESS, BASE_URL, { ...ask, code: wrong })).rejects.toThrow();

  // 3. The right code makes the passkey and the account, once.
  const fresh = await requestRegisterCode(page, ADDRESS, BASE_URL);
  code = '';
  for (let i = 0; i < 30 && !code; i++) { code = await newestCode(api, seen); if (!code) await page.waitForTimeout(5000); }
  const made = await registerPasskey(page, ADDRESS, BASE_URL, { ...fresh, code });
  expect(made.ok).toBeTruthy();
  const session = await getSession(page, BASE_URL);
  expect(session.authenticated).toBeTruthy();

  // 4. The spent code makes nothing more, and the address now goes to recovery, not registration.
  await expect(registerPasskey(page, ADDRESS, BASE_URL, { ...fresh, code })).rejects.toThrow();
  const again = await requestRegisterCode(page, ADDRESS, BASE_URL);
  expect(again.attemptId).toBeTruthy();
});
