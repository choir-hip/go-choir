/**
 * Deployed bootstrap for the email sign-in acceptance
 * (scripts/e2e-email-signin-staging.sh, docs/design/email-sign-in-2026-10-31.md,
 * "Automated test (no person)"). Run once; the script skips it when the state
 * file says it ran.
 *
 * Registers a dedicated test account whose address is a reserved numbered
 * TEST address (E2E_ADDRESS, from maildctl reserve-test-number; 10,000,000..
 * 99,999,999) on the public receive policy, which needs no sender trust. The
 * number is bound to the account with maildctl bind-number on node-b between
 * register/begin (the user row exists) and register/finish (the credential
 * appears, which is what the auto-assign reconciler waits for), so the account
 * never receives a user number. Public-policy mail lands in quarantine; the
 * account's own read:runtime key reads the quarantine folder. Playwright's CDP
 * virtual WebAuthn authenticator makes the one passkey. From that session the
 * spec mints the key, mails and finishes verify-address, and waits for the
 * account's computer.
 *
 * Secrets: the account id, address and key go only to the state file
 * (E2E_STATE_FILE, mode 0600, outside the repo). Nothing secret is printed or
 * placed in an assertion message.
 *
 * Env: E2E_ADDRESS, E2E_STATE_FILE (required); BASE_URL (default
 * https://choir.news); E2E_NODE_B_SSH (default node-b). The key: E2E_KEY_SCOPES
 * (comma list, default read:runtime), E2E_KEY_LABEL, E2E_KEY_FIELD (the state
 * field that holds the secret, default inbox_key). scripts/test-computer-create
 * passes every registry scope and api_key.
 */
import { expect, request as pwRequest } from '@playwright/test';
import { test } from './helpers/fixtures.js';
import { registerPasskey, getSession } from './helpers/auth.js';
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';

const BASE_URL = process.env.BASE_URL || 'https://choir.news';
const ADDRESS = process.env.E2E_ADDRESS || '';
const STATE_FILE = process.env.E2E_STATE_FILE || '';
const NODE_B = process.env.E2E_NODE_B_SSH || 'node-b';
const KEY_SCOPES = (process.env.E2E_KEY_SCOPES || 'read:runtime')
  .split(',')
  .map((s) => s.trim())
  .filter(Boolean);
const KEY_LABEL = process.env.E2E_KEY_LABEL || 'e2e-email-signin inbox reader';
const KEY_FIELD = process.env.E2E_KEY_FIELD || 'inbox_key';

function readState() {
  try {
    return JSON.parse(fs.readFileSync(STATE_FILE, 'utf8'));
  } catch (_e) {
    return {};
  }
}

function writeState(patch) {
  const next = { ...readState(), ...patch };
  fs.mkdirSync(path.dirname(STATE_FILE), { recursive: true, mode: 0o700 });
  const tmp = `${STATE_FILE}.tmp`;
  fs.writeFileSync(tmp, JSON.stringify(next, null, 2) + '\n', { mode: 0o600 });
  fs.renameSync(tmp, STATE_FILE);
  fs.chmodSync(STATE_FILE, 0o600);
}

async function pageFetch(page, p, options = {}) {
  return page.evaluate(
    async ({ p, options }) => {
      const res = await fetch(p, { credentials: 'include', ...options });
      const text = await res.text();
      let body = text;
      try {
        body = JSON.parse(text);
      } catch (_e) {
        /* raw */
      }
      return { status: res.status, body };
    },
    { p, options },
  );
}

// Newest message whose subject contains `subject`, in inbox or quarantine,
// that was not in `seen`. Returns {id, code} or null.
async function newestCode(api, subject, seen) {
  let best = null;
  for (const folder of ['inbox', 'quarantine']) {
    const res = await api.get(`/api/email/messages?folder=${folder}&limit=50`);
    if (res.status() !== 200) continue;
    const body = await res.json();
    for (const m of body.messages || []) {
      if (seen.has(m.id) || !(m.subject || '').includes(subject)) continue;
      if (!best || (m.created_at || '') > (best.created_at || '')) best = m;
    }
  }
  if (!best) return null;
  const detail = await api.get(`/api/email/messages/${best.id}`);
  if (detail.status() !== 200) return null;
  const text = (await detail.json()).text_body || '';
  const m = /Your code:\s*(\d{8})/.exec(text);
  return m ? { id: best.id, code: m[1] } : null;
}

async function inboxIds(api) {
  const ids = new Set();
  for (const folder of ['inbox', 'quarantine']) {
    const res = await api.get(`/api/email/messages?folder=${folder}&limit=200`);
    if (res.status() !== 200) continue;
    for (const m of (await res.json()).messages || []) ids.add(m.id);
  }
  return ids;
}

let pendingUserID = '';
let bindError = '';

test('bootstrap the email sign-in test account', async ({ page, authenticator }) => {
  test.setTimeout(900_000);
  expect(ADDRESS, 'E2E_ADDRESS is required').toBeTruthy();
  expect(STATE_FILE, 'E2E_STATE_FILE is required').toBeTruthy();
  const prior = readState();
  if (prior.stage === 'done') {
    console.log('bootstrap already done; nothing to do');
    return;
  }
  expect(
    !prior.stage,
    'state file holds a half-finished bootstrap: its passkey lived only in the browser that registered it; move the state file aside and run again (a new address is chosen)',
  ).toBeTruthy();
  expect(/^\d{8}@choir\.news$/.test(ADDRESS), 'E2E_ADDRESS must be a reserved 8-digit test number at choir.news').toBeTruthy();

  // 1. Register the passkey. Between register/begin and register/finish the
  // reserved number is bound to the new user (id from the begin response).
  let boundID = '';
  // The auth limiter (internal/auth/rate_limit.go, 10 requests per IP per
  // minute) answers 429 before the handler runs, so replaying the same request
  // after the window is safe; back-to-back test computers hit it.
  const continueRetrying429 = async (route) => {
    for (let attempt = 0; ; attempt++) {
      const res = await route.fetch();
      if (res.status() !== 429 || attempt >= 3) {
        await route.fulfill({ response: res });
        return;
      }
      console.log('auth rate limit (429); waiting for the next window');
      await new Promise((r) => setTimeout(r, 61_000));
    }
  };
  await page.route('**/auth/**', continueRetrying429);
  await page.route('**/auth/register/finish', async (route) => {
    if (!boundID) {
      try {
        execFileSync(
          'ssh',
          ['-o', 'ConnectTimeout=15', '-o', 'BatchMode=yes', NODE_B, `maildctl bind-number --address ${ADDRESS} --owner ${pendingUserID}`],
          { stdio: 'pipe', timeout: 60_000 },
        );
        boundID = pendingUserID;
      } catch (e) {
        bindError = String((e.stderr || e.message || e)).split(ADDRESS).join('<address>').slice(0, 300);
      }
    }
    await continueRetrying429(route);
  });
  page.on('response', async (res) => {
    if (!res.url().endsWith('/auth/register/begin')) return;
    try {
      const j = await res.json();
      const idb64 = (j.publicKey || j).user.id;
      pendingUserID = Buffer.from(idb64, 'base64url').toString('utf8');
    } catch (_e) {
      /* surfaced by the bind check below */
    }
  });
  // A cap or waitlist answers 403 registration_waitlist.
  await page.goto(BASE_URL);
  let reg;
  try {
    reg = await registerPasskey(page, ADDRESS, BASE_URL);
  } catch (e) {
    const msg = String(e.message || e);
    if (msg.includes('registration_waitlist')) {
      throw new Error('registration is capped (AUTH_REGISTRATION_CAP reached, registration_waitlist): the operator must raise or unset the cap in nix/node-b.nix, deploy, and run again. This attempt put the test address on the waitlist.');
    }
    throw new Error(`register failed: ${msg.split(ADDRESS).join('<address>')}`);
  }
  expect(reg && reg.ok !== false, 'registration did not complete').toBeTruthy();
  const session = await getSession(page, BASE_URL);
  const userID = session.user && session.user.id;
  expect(userID, 'registration returned no user id').toBeTruthy();
  writeState({ stage: 'registered', account_id: userID, address: ADDRESS, backend: BASE_URL });
  console.log('registered the test account (id and address in the state file)');

  // 2. The bind ran in the finish hook; refuse to go on without it.
  expect(boundID && boundID === userID && !bindError, `bind-number did not complete: ${bindError || 'user id mismatch'}`).toBeTruthy();
  writeState({ stage: 'aliased' });
  console.log('numbered address bound to the account');

  // 3. Mint the key from the passkey session (it reads the inbox: every
  // scope set passed here includes read:runtime). A refusal is reported with
  // the auth service's reason, never retried with fewer scopes.
  const keyRes = await pageFetch(page, '/auth/api-keys', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ label: KEY_LABEL, scopes: KEY_SCOPES }),
  });
  const keyRefusal = keyRes.body && typeof keyRes.body === 'object' ? keyRes.body.error : String(keyRes.body).slice(0, 200);
  expect([200, 201], `mint api key with scopes ${KEY_SCOPES.join(',')}: ${keyRes.status} ${keyRefusal || ''}`).toContain(keyRes.status);
  const secret = keyRes.body && keyRes.body.secret;
  expect(secret, 'api key secret missing').toBeTruthy();
  const granted = (keyRes.body.scopes || []).slice().sort();
  const missing = KEY_SCOPES.filter((s) => !granted.includes(s));
  expect(missing, 'the minted key lacks requested scopes').toEqual([]);
  writeState({ stage: 'keyed', [KEY_FIELD]: secret, [`${KEY_FIELD}_id`]: keyRes.body.id, [`${KEY_FIELD}_scopes`]: granted });
  console.log(`key minted with ${granted.length} scopes (secret in the state file only)`);

  const api = await pwRequest.newContext({
    baseURL: BASE_URL,
    extraHTTPHeaders: { Authorization: `Bearer ${secret}` },
  });
  const probe = await api.get('/api/email/messages?folder=inbox&limit=1');
  expect(probe.status(), 'the key reads the inbox').toBe(200);

  // 4. verify-address: begin, read the code from the inbox, finish.
  const seen = await inboxIds(api);
  const begin = await pageFetch(page, '/auth/email/verify-address/begin', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: '{}',
  });
  expect(begin.status, 'verify-address/begin').toBe(200);
  if (begin.body && begin.body.verified) {
    console.log('address already verified');
  } else {
    const attemptID = begin.body && begin.body.attempt_id;
    expect(attemptID, 'verify-address/begin returned no attempt').toBeTruthy();
    let found = null;
    const until = Date.now() + 240_000;
    while (Date.now() < until && !found) {
      found = await newestCode(api, 'Verify your email address', seen);
      if (!found) await page.waitForTimeout(5000);
    }
    expect(found, 'verify-address mail never reached the test inbox (alias, receive policy, or Resend inbound); check inbox and quarantine of the test account').toBeTruthy();
    const fin = await pageFetch(page, '/auth/email/verify-address/finish', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ attempt_id: attemptID, code: found.code }),
    });
    expect(fin.status, 'verify-address/finish').toBe(200);
  }
  writeState({ stage: 'verified' });
  console.log('address verified');

  // 5. The account's computer cold-boots on first use; wait for it and keep
  // its id for the follow step.
  await page.goto(BASE_URL);
  await page.locator('[data-desktop][data-authenticated="true"]').waitFor({ state: 'visible', timeout: 120_000 });
  const until = Date.now() + 300_000;
  let files = 0;
  while (Date.now() < until) {
    files = (await pageFetch(page, '/api/files')).status;
    if (files === 200) break;
    await page.waitForTimeout(5000);
  }
  expect(files, "the account's computer never became ready").toBe(200);
  const status = await pageFetch(page, '/api/compute/status');
  const computerID = status.body && status.body.current_computer && status.body.current_computer.computer_id;
  expect(computerID, 'no current computer').toBeTruthy();
  writeState({ stage: 'done', computer_id: computerID, bootstrapped_at: new Date().toISOString() });
  console.log('bootstrap done');
  await api.dispose();
});
