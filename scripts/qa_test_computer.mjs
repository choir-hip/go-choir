// qa_test_computer.mjs — create a fresh test owner and computer on staging and
// mint an API key scoped to that one computer, so an agent can drive it through
// the choir CLI and a browser harness can drive it through the GUI.
//
// The key and the browser session are written only to 0600 files under
// ~/.config/choir-qa/test-computers/; nothing secret is printed. stdout is one
// JSON line of non-secret identifiers, file paths and per-phase timings.
//
// The computer id comes from the product API under the new owner's session
// (GET /api/runtime/observability, cross-checked against the proxy's
// GET /api/compute/status), never from SSH or /internal/* (standing question
// 9). No product route exposes the VM id, so the output carries none.
//
//   node scripts/qa_test_computer.mjs [--label <label> | <label>]
import { createRequire } from 'node:module';
import { chmodSync, existsSync, mkdirSync, writeFileSync } from 'node:fs';
import { homedir } from 'node:os';
import { join } from 'node:path';

const requireFrontend = createRequire(new URL('../frontend/package.json', import.meta.url));
const { chromium } = requireFrontend('@playwright/test');
import { registerPasskey } from '../frontend/tests/helpers/auth.js';
import { setupVirtualAuthenticator } from '../frontend/tests/helpers/webauthn.js';

const BASE_URL = process.env.CHOIR_DEPLOYED_BASE_URL || 'https://choir.news';
const argv = process.argv.slice(2);
const labelFlag = argv.indexOf('--label');
const label = (labelFlag >= 0 ? argv[labelFlag + 1] : argv.find((a) => !a.startsWith('--'))) || `test-computer-${Date.now()}`;
if (!/^[A-Za-z0-9._-]+$/.test(label)) throw new Error(`label must be [A-Za-z0-9._-]+: ${label}`);
const dir = join(homedir(), '.config', 'choir-qa', 'test-computers');
const SCOPES = [
  'read:texture', 'write:texture', 'read:runtime', 'write:runtime', 'read:base',
  'computer:lifecycle',
  'computer:self_development:read', 'computer:self_development:propose',
  'computer:self_development:approve', 'computer:self_development:rollback',
  'computer:self_development:mode',
];

// computerFromProductAPI reads the logged-in owner's computer id through
// same-origin product routes. The guest's own observability answer is the
// authority for which computer the session reaches; the proxy's status view
// (vmctl ownership as the product API shows it) must agree when it names one.
async function computerFromProductAPI(page) {
  return page.evaluate(async () => {
    const read = async (path) => {
      try {
        const response = await fetch(path, { credentials: 'same-origin' });
        return { status: response.status, json: response.ok ? await response.json().catch(() => null) : null };
      } catch (err) {
        return { status: 0, json: null, error: String(err) };
      }
    };
    const observability = await read('/api/runtime/observability');
    const status = await read('/api/compute/status');
    return {
      observability_status: observability.status,
      guest_computer_id: observability.json?.computer_id || '',
      guest_build_commit: observability.json?.build?.commit || '',
      status_status: status.status,
      proxy_computer_id: status.json?.current_computer?.computer_id || '',
      proxy_state: status.json?.current_computer?.state || '',
    };
  });
}

function writePrivate(path, content) {
  writeFileSync(path, content, { mode: 0o600 });
  chmodSync(path, 0o600);
}

mkdirSync(dir, { recursive: true, mode: 0o700 });
chmodSync(dir, 0o700);
// A label names exactly one minted computer: never overwrite an earlier
// credential (a caller that reuses a label would otherwise act on a computer
// it did not mint in this invocation).
if (existsSync(join(dir, `${label}.json`))) throw new Error(`label ${label} already has a credential file; pick a new label`);
const browser = await chromium.launch({ headless: true });
try {
  const context = await browser.newContext();
  const page = await context.newPage();
  await setupVirtualAuthenticator(page);
  await page.goto(BASE_URL, { waitUntil: 'domcontentloaded', timeout: 60_000 });
  const email = `${label}@example.com`;
  const t0 = Date.now();
  const timings = { started_at: new Date(t0).toISOString() };
  const mark = (name) => { timings[`${name}_ms`] = Date.now() - t0; };
  await registerPasskey(page, email, BASE_URL);
  mark('registered');
  await page.reload({ waitUntil: 'domcontentloaded', timeout: 60_000 });

  const session = await page.evaluate(async () => (await fetch('/auth/session', { credentials: 'same-origin' })).json());
  const userID = session?.user?.id;
  if (!userID) throw new Error('no owner id from /auth/session');
  // A fresh computer can take minutes to boot when no warm one is free. Wait
  // for both the guest's answer (the computer id) and the prompt bar, up to
  // MINT_READY_TIMEOUT_MIN; the page's own bootstrap retries forever, so a
  // reload is only a fallback for a page that stopped rendering.
  const readyDeadline = Date.now() + Number(process.env.MINT_READY_TIMEOUT_MIN || 20) * 60_000;
  let observed = null;
  let lastReload = Date.now();
  while (Date.now() < readyDeadline) {
    observed = await computerFromProductAPI(page).catch(() => observed);
    if (observed?.guest_computer_id && timings.computer_answered_ms === undefined) mark('computer_answered');
    const promptBar = await page.locator('[data-prompt-input]').first().isVisible().catch(() => false);
    if (promptBar && timings.prompt_bar_visible_ms === undefined) mark('prompt_bar_visible');
    if (observed?.guest_computer_id && promptBar) break;
    if (Date.now() - lastReload > 5 * 60_000) {
      await page.reload({ waitUntil: 'domcontentloaded', timeout: 60_000 }).catch(() => {});
      lastReload = Date.now();
    }
    await new Promise((resolve) => setTimeout(resolve, 3_000));
  }
  if (!observed?.guest_computer_id || timings.prompt_bar_visible_ms === undefined) {
    throw new Error(`fresh computer not ready after ${Math.round((Date.now() - t0) / 1000)}s: user ${userID}, observability ${observed?.observability_status}, compute/status ${observed?.status_status} computer ${observed?.proxy_computer_id || '-'} state ${observed?.proxy_state || '-'}, prompt bar ${timings.prompt_bar_visible_ms === undefined ? 'not visible' : 'visible'}`);
  }
  if (observed.proxy_computer_id && observed.proxy_computer_id !== observed.guest_computer_id) {
    throw new Error(`computer id disagreement: guest ${observed.guest_computer_id} vs proxy ${observed.proxy_computer_id}`);
  }
  const ownership = { computer_id: observed.guest_computer_id };
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
  mark('key_minted');

  const credentialPath = join(dir, `${label}.json`);
  const storagePath = join(dir, `${label}.storage.json`);
  writePrivate(credentialPath, JSON.stringify({
    base_url: BASE_URL, email, user_id: userID, computer_id: ownership.computer_id,
    api_key: secret, api_key_id: minted.json?.id, scopes: SCOPES, expires_at: expiresAt, created_at: new Date().toISOString(),
  }, null, 2));
  writePrivate(storagePath, JSON.stringify(await context.storageState()));
  console.log(JSON.stringify({
    label, email, user_id: userID, computer_id: ownership.computer_id,
    guest_build_commit: observed.guest_build_commit || null,
    api_key_id: minted.json?.id, scopes: SCOPES, expires_at: expiresAt,
    credential_file: credentialPath, browser_session_file: storagePath, timings,
  }));
} finally {
  await browser.close();
}
