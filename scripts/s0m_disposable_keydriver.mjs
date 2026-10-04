#!/usr/bin/env node
// S0m boundary-close driver: mint a scoped API key for a fresh disposable
// staging account so the stranded-bound probe (action 2) can run on a
// disposable computer, as the finish contract requires.
//
// Steps: Playwright passkey registration (product path) -> wait for the new
// computer to reach active via node-b vmctl -> POST /auth/api-keys under the
// session cookie -> print {api_key, user_id, computer_id} JSON on stdout.
//
// Usage: node scripts/s0m_disposable_keydriver.mjs [--resume-computer id]
//   --resume-computer: skip registration; log in is impossible for the old
//   account (virtual authenticator is per-run), so this instead registers
//   another fresh account. Kept for interface stability only.
// Requires: ssh node-b (BatchMode), frontend playwright deps.

import { createRequire } from 'node:module';
import { execFileSync } from 'node:child_process';

const requireFrontend = createRequire(new URL('../frontend/package.json', import.meta.url));
const { chromium } = requireFrontend('@playwright/test');
const { registerPasskey } = await import('../frontend/tests/helpers/auth.js');
const { setupVirtualAuthenticator } = await import('../frontend/tests/helpers/webauthn.js');

const BASE_URL = process.env.CHOIR_DEPLOYED_BASE_URL || 'https://choir.news';
const args = process.argv.slice(2);
const arg = (name, dflt) => { const i = args.indexOf(`--${name}`); return i >= 0 ? args[i + 1] : dflt; };
const LABEL = arg('label', `s0m-stranded-probe-${Date.now()}`);

function nodeB(command) {
  return execFileSync('ssh', ['-o', 'BatchMode=yes', '-o', 'ConnectTimeout=8', 'node-b', command], { encoding: 'utf8', maxBuffer: 16 * 1024 * 1024 }).trim();
}
function vmctlList() {
  return JSON.parse(nodeB(`curl -fsS -H "X-Internal-Caller: true" http://127.0.0.1:8083/internal/vmctl/list`));
}
const uniqueEmail = () => `s0m-dispo-${Date.now()}-${Math.random().toString(36).slice(2, 8)}@example.com`;

const browser = await chromium.launch();
try {
  const ctx = await browser.newContext();
  const page = await ctx.newPage();
  await setupVirtualAuthenticator(page);
  const email = uniqueEmail();
  const regStarted = Date.now();
  await page.goto(BASE_URL, { waitUntil: 'domcontentloaded', timeout: 60_000 });
  const reg = await registerPasskey(page, email, BASE_URL);
  if (!reg || reg.ok === false) throw new Error(`passkey registration rejected: ${JSON.stringify(reg)?.slice(0, 300)}`);
  const userId = reg.user?.id || '';
  await page.reload({ waitUntil: 'domcontentloaded', timeout: 60_000 });
  console.error(`registered ${email} user=${userId}`);

  // Wait for the disposable computer to provision and reach active.
  const createdAfter = new Date(regStarted - 5000).toISOString();
  let own = null;
  for (let i = 0; i < 180; i++) {
    const list = vmctlList();
    own = (list?.ownerships || [])
      .filter(o => o.user_id === userId && o.created_at >= createdAfter)
      .sort((a, b) => String(b.created_at).localeCompare(String(a.created_at)))[0]
      || (list?.ownerships || []).find(o => o.user_id === userId);
    if (own && own.state === 'active') break;
    await new Promise(r => setTimeout(r, 3000));
  }
  if (!own) throw new Error(`no computer provisioned for ${userId}`);
  console.error(`computer=${own.computer_id} vm=${own.vm_id} state=${own.state}`);
  if (own.state !== 'active') throw new Error(`computer did not reach active: ${own.state}`);

  // Mint a scoped API key under the session cookie.
  const key = await page.evaluate(async ({ label, computer }) => {
    const res = await fetch('/auth/api-keys', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ label, scopes: ['read:base', 'write:base', 'read:runtime', 'write:runtime', 'read:texture', 'write:texture', 'computer:lifecycle'], computer_id: computer }),
    });
    const body = await res.json().catch(() => null);
    return { status: res.status, body };
  }, { label: LABEL, computer: own.computer_id });
  if (key.status !== 201 || !key.body?.secret) {
    const retry = await page.evaluate(async ({ label }) => {
      const res = await fetch('/auth/api-keys', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ label }),
      });
      const body = await res.json().catch(() => null);
      return { status: res.status, body };
    }, { label: LABEL });
    if (retry.status !== 201 || !retry.body?.secret) {
      throw new Error(`api key mint failed: ${JSON.stringify({ first: key, retry })}`);
    }
    key.body = retry.body;
  }
  console.log(JSON.stringify({ api_key: key.body.secret, key_id: key.body.id, user_id: userId, email, computer_id: own.computer_id, vm_id: own.vm_id, computer_url: own.computer_url }));
} finally {
  await browser.close();
}
