#!/usr/bin/env node
// S0b disposable-computer keydriver: register a fresh staging account, wait
// for its computer to reach active, POST lifecycle/bootstrap-chain (idempotent
// repair path — computers provisioned after SA slice 0 self-mint genesis at
// first boot and this returns already_bootstrapped; see
// docs/problems/s0b-registration-computer-missing-genesis-2026-10-04.md),
// mint a scoped API key, print {api_key, user_id, computer_id, vm_id,
// computer_url} JSON on stdout.
//
// Usage: node scripts/s0b_disposable_keydriver.mjs [--label NAME]
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
const LABEL = arg('label', `s0b-probe-${Date.now()}`);

function nodeB(command) {
  return execFileSync('ssh', ['-o', 'BatchMode=yes', '-o', 'ConnectTimeout=8', 'node-b', command], { encoding: 'utf8', maxBuffer: 16 * 1024 * 1024 }).trim();
}
function vmctlList() {
  return JSON.parse(nodeB(`curl -fsS -H "X-Internal-Caller: true" http://127.0.0.1:8083/internal/vmctl/list`));
}
const uniqueEmail = () => `s0b-dispo-${Date.now()}-${Math.random().toString(36).slice(2, 8)}@example.com`;

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

  // Post-SA-slice-0 (9f6f369c): computers mint genesis_imported in-guest at
  // first boot, so bootstrap-chain is a repair path only. --skip-bootstrap
  // asserts that property: the fresh computer is left untouched and the
  // caller's first write must succeed without it (fails pre-slice-0).
  if (process.argv.includes('--skip-bootstrap')) {
    console.error('skipping bootstrap-chain (SA slice 0 acceptance)');
  } else {
    const boot = await page.evaluate(async ({ computerID }) => {
      const res = await fetch(`/api/computers/${encodeURIComponent(computerID)}/lifecycle/bootstrap-chain`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: '{}',
      });
      return { status: res.status, body: await res.json().catch(() => null) };
    }, { computerID: own.computer_id });
    if (boot.status !== 200 && boot.status !== 201) {
      throw new Error(`bootstrap-chain refused: ${JSON.stringify(boot)}`);
    }
    console.error(`bootstrap-chain -> ${boot.status}`);
  }

  const key = await page.evaluate(async ({ label, computer }) => {
    const res = await fetch('/auth/api-keys', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ label, scopes: ['read:base', 'write:base', 'read:runtime', 'write:runtime', 'read:texture', 'write:texture', 'computer:lifecycle', 'computer:self_development:read', 'computer:self_development:propose', 'computer:self_development:mode', 'manage:keys'], computer_id: computer }),
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
