// qa_renew_key.mjs — renew the API key of a test owner made by
// qa_test_computer.mjs, from that owner's saved browser session.
//
// The test owners have no person behind them: qa_test_computer.mjs registered
// them with a virtual passkey and saved the session to
// ~/.config/choir-qa/test-computers/<label>.storage.json. Their keys expire in
// seven days. This mints a new key with the same scopes and the same computer,
// checks it, rewrites <label>.json (0600), refreshes the saved session, then
// revokes the old key. Nothing secret is printed; stdout is one JSON line of
// non-secret identifiers.
//
//   node scripts/qa_renew_key.mjs <label>
//
// CHOIR_FRONTEND_DIR names a frontend checkout with node_modules when this
// tree has none (default: ../frontend beside this script).
import { createRequire } from 'node:module';
import { chmodSync, existsSync, readFileSync, renameSync, writeFileSync } from 'node:fs';
import { homedir } from 'node:os';
import { join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = fileURLToPath(new URL('.', import.meta.url));
const frontendDir = process.env.CHOIR_FRONTEND_DIR || resolve(here, '../frontend');
const { chromium } = createRequire(join(frontendDir, 'package.json'))('@playwright/test');

const label = process.argv[2] || '';
if (!/^[A-Za-z0-9._-]+$/.test(label)) throw new Error('usage: node scripts/qa_renew_key.mjs <label>');
const dir = join(homedir(), '.config', 'choir-qa', 'test-computers');
const credentialPath = join(dir, `${label}.json`);
const storagePath = join(dir, `${label}.storage.json`);
if (!existsSync(credentialPath) || !existsSync(storagePath)) throw new Error(`no credential or session file for ${label}`);
const old = JSON.parse(readFileSync(credentialPath, 'utf8'));
const BASE_URL = old.base_url || process.env.CHOIR_DEPLOYED_BASE_URL || 'https://choir.news';
if (!old.user_id || !old.computer_id || !Array.isArray(old.scopes) || old.scopes.length === 0) {
  throw new Error(`${label}.json lacks user_id, computer_id or scopes`);
}

function writePrivate(path, content) {
  const tmp = `${path}.tmp`;
  writeFileSync(tmp, content, { mode: 0o600 });
  chmodSync(tmp, 0o600);
  renameSync(tmp, path);
}

const browser = await chromium.launch();
try {
  const context = await browser.newContext({ storageState: storagePath });
  const page = await context.newPage();
  await page.goto(BASE_URL, { waitUntil: 'domcontentloaded', timeout: 60_000 });
  const call = (path, init) => page.evaluate(async ([p, i]) => {
    const response = await fetch(p, { credentials: 'same-origin', ...i });
    return { status: response.status, json: await response.json().catch(() => null) };
  }, [path, init || {}]);

  // The saved session must still be this owner's; a lapsed session would
  // otherwise mint nothing, or a fresh anonymous one would mint for someone else.
  const session = await call('/auth/session');
  const userID = session.json?.user?.id || '';
  if (userID !== old.user_id) throw new Error(`saved session is not ${label}'s owner (session user ${userID || 'none'}, status ${session.status})`);

  const expiresAt = new Date(Date.now() + 7 * 24 * 3600 * 1000).toISOString();
  const minted = await call('/auth/api-keys', {
    method: 'POST', headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ label: `${label}-agent-renewed`, scopes: old.scopes, computer_id: old.computer_id, expires_at: expiresAt }),
  });
  if (minted.status !== 201 && minted.status !== 200) throw new Error(`api key create refused: ${minted.status} ${minted.json?.error || ''}`);
  const secret = minted.json?.key || minted.json?.secret || minted.json?.api_key;
  if (!secret) throw new Error(`api key create returned no secret field (keys: ${Object.keys(minted.json || {}).join(',')})`);

  // The new key must work before the old one goes: read the computer's status
  // with it as a bearer, outside the browser's cookies.
  const check = await fetch(`${BASE_URL}/api/compute/status`, { headers: { Authorization: `Bearer ${secret}` } });
  if (!check.ok) throw new Error(`new key refused by ${BASE_URL}/api/compute/status: ${check.status}; old key kept, new key ${minted.json?.id} left for revocation`);

  writePrivate(credentialPath, JSON.stringify({
    ...old, api_key: secret, api_key_id: minted.json?.id, expires_at: expiresAt, created_at: new Date().toISOString(),
    renewed_from_api_key_id: old.api_key_id || null,
  }, null, 2));
  writePrivate(storagePath, JSON.stringify(await context.storageState()));

  let revoked = null;
  if (old.api_key_id && old.api_key_id !== minted.json?.id) {
    revoked = (await call(`/auth/api-keys/${encodeURIComponent(old.api_key_id)}`, { method: 'DELETE' })).status;
  }
  console.log(JSON.stringify({
    label, user_id: userID, computer_id: old.computer_id, api_key_id: minted.json?.id,
    old_api_key_id: old.api_key_id || null, old_key_revoke_status: revoked,
    scopes: old.scopes, expires_at: expiresAt, credential_file: credentialPath,
  }));
} finally {
  await browser.close();
}
