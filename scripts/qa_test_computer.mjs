// qa_test_computer.mjs — create a fresh test owner and computer on staging and
// mint an API key scoped to that one computer, so an agent can drive it through
// the choir CLI and a browser harness can drive it through the GUI.
//
// The key and the browser session are written only to 0600 files under
// ~/.config/choir-qa/test-computers/; nothing secret is printed. stdout is one
// JSON line of non-secret identifiers and file paths.
//
//   node scripts/qa_test_computer.mjs [label]
import { createRequire } from 'node:module';
import { execFileSync } from 'node:child_process';
import { chmodSync, mkdirSync, writeFileSync } from 'node:fs';
import { homedir } from 'node:os';
import { join } from 'node:path';

const requireFrontend = createRequire(new URL('../frontend/package.json', import.meta.url));
const { chromium } = requireFrontend('@playwright/test');
import { registerPasskey } from '../frontend/tests/helpers/auth.js';
import { setupVirtualAuthenticator } from '../frontend/tests/helpers/webauthn.js';

const BASE_URL = process.env.CHOIR_DEPLOYED_BASE_URL || 'https://choir.news';
const label = process.argv[2] || `test-computer-${Date.now()}`;
const dir = join(homedir(), '.config', 'choir-qa', 'test-computers');
const SCOPES = [
  'read:texture', 'write:texture', 'read:runtime', 'write:runtime', 'read:base',
  'computer:lifecycle',
  'computer:self_development:read', 'computer:self_development:propose',
  'computer:self_development:approve', 'computer:self_development:rollback',
  'computer:self_development:mode',
];

function vmctlOwnership(userID) {
  const raw = execFileSync('ssh', ['-o', 'BatchMode=yes', '-o', 'ConnectTimeout=8', 'node-b',
    `curl -fsS -H "X-Internal-Caller: true" http://127.0.0.1:8083/internal/vmctl/list | jq -c --arg u '${userID}' '.ownerships[] | select(.user_id==$u and .desktop_id=="primary" and .kind=="interactive")'`,
  ], { encoding: 'utf8' }).trim();
  return raw ? JSON.parse(raw) : null;
}

function writePrivate(path, content) {
  writeFileSync(path, content, { mode: 0o600 });
  chmodSync(path, 0o600);
}

mkdirSync(dir, { recursive: true, mode: 0o700 });
chmodSync(dir, 0o700);
const browser = await chromium.launch({ headless: true });
try {
  const context = await browser.newContext();
  const page = await context.newPage();
  await setupVirtualAuthenticator(page);
  await page.goto(BASE_URL, { waitUntil: 'domcontentloaded', timeout: 60_000 });
  const email = `${label}@example.com`;
  await registerPasskey(page, email, BASE_URL);
  await page.reload({ waitUntil: 'domcontentloaded', timeout: 60_000 });
  await page.waitForSelector('[data-prompt-input]', { timeout: 300_000 });

  const session = await page.evaluate(async () => (await fetch('/auth/session', { credentials: 'same-origin' })).json());
  const userID = session?.user?.id;
  if (!userID) throw new Error('no owner id from /auth/session');
  let ownership = null;
  for (let i = 0; i < 30 && !ownership?.computer_id; i++) {
    ownership = vmctlOwnership(userID);
    if (!ownership?.computer_id) await new Promise((resolve) => setTimeout(resolve, 5_000));
  }
  if (!ownership?.computer_id) throw new Error('fresh owner has no computer');

  const expiresAt = new Date(Date.now() + 7 * 24 * 3600 * 1000).toISOString();
  const minted = await page.evaluate(async (body) => {
    const response = await fetch('/auth/api-keys', {
      method: 'POST', credentials: 'same-origin',
      headers: { 'content-type': 'application/json' }, body: JSON.stringify(body),
    });
    return { status: response.status, json: await response.json().catch(() => null) };
  }, { label: `${label}-agent`, scopes: SCOPES, computer_id: ownership.computer_id, expires_at: expiresAt });
  const secret = minted.json?.key || minted.json?.secret || minted.json?.api_key;
  if (minted.status !== 201 && minted.status !== 200) throw new Error(`api key create refused: ${minted.status} ${minted.json?.error || ''}`);
  if (!secret) throw new Error(`api key create returned no secret field (keys: ${Object.keys(minted.json || {}).join(',')})`);

  const credentialPath = join(dir, `${label}.json`);
  const storagePath = join(dir, `${label}.storage.json`);
  writePrivate(credentialPath, JSON.stringify({
    base_url: BASE_URL, email, user_id: userID, computer_id: ownership.computer_id, vm_id: ownership.vm_id,
    api_key: secret, api_key_id: minted.json?.id, scopes: SCOPES, expires_at: expiresAt, created_at: new Date().toISOString(),
  }, null, 2));
  writePrivate(storagePath, JSON.stringify(await context.storageState()));
  console.log(JSON.stringify({
    label, email, user_id: userID, computer_id: ownership.computer_id, vm_id: ownership.vm_id,
    api_key_id: minted.json?.id, scopes: SCOPES, expires_at: expiresAt,
    credential_file: credentialPath, browser_session_file: storagePath,
  }));
} finally {
  await browser.close();
}
