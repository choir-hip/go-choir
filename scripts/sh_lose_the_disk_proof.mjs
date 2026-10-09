#!/usr/bin/env node
// SH lose-the-disk proof (operational invariant O21, Gate 1 exit test).
//
// On a fresh disposable staging computer: write a private owner file, sync
// it to the file root chain, remove the realization (its data image is
// gone), and require that the next realization of the same computer serves
// the same file. The key half is proven by the delivery log line; the files
// half by reading the file back through the product API.
//
// Steps:
//   1. register a disposable account (Playwright passkey, product path) and
//      wait for its computer to be active;
//   2. PUT /api/files/<proof file>, then POST /api/files/sync;
//   3. vmctl remove (the realization and its volume are discarded);
//   4. poll GET /api/files/<proof file> until it returns the written content
//      (each request resolves the computer, which boots a fresh realization);
//   5. collect the new realization's console lines for key delivery and file
//      hydration.
//
// Writes a JSON receipt (default docs/evidence/sh-lose-the-disk-<stamp>.json)
// and exits 0 only on pass.
// Usage: node scripts/sh_lose_the_disk_proof.mjs [--out path] [--timeout-min 20]
// Requires: ssh node-b (BatchMode), frontend playwright deps.

import { createRequire } from 'node:module';
import { execFileSync } from 'node:child_process';
import { writeFileSync } from 'node:fs';

const requireFrontend = createRequire(new URL('../frontend/package.json', import.meta.url));
const { chromium } = requireFrontend('@playwright/test');
const { registerPasskey } = await import('../frontend/tests/helpers/auth.js');
const { setupVirtualAuthenticator } = await import('../frontend/tests/helpers/webauthn.js');

const BASE_URL = process.env.CHOIR_DEPLOYED_BASE_URL || 'https://choir.news';
const args = process.argv.slice(2);
const arg = (name, dflt) => { const i = args.indexOf(`--${name}`); return i >= 0 ? args[i + 1] : dflt; };
const stamp = new Date().toISOString().replace(/[:.]/g, '-');
const OUT = arg('out', `docs/evidence/sh-lose-the-disk-${stamp}.json`);
const TIMEOUT_MS = Number(arg('timeout-min', '20')) * 60_000;
const PROOF_FILE = 'sh-o21-proof.txt';
const PROOF_TEXT = `lose-the-disk proof ${stamp} ${Math.random().toString(36).slice(2)}`;

const receipt = { started_at: new Date().toISOString(), base_url: BASE_URL, proof_file: PROOF_FILE, steps: [] };
const step = (name, data) => {
  const entry = { name, at: new Date().toISOString(), ...data };
  receipt.steps.push(entry);
  console.error(`[${entry.at}] ${name}: ${JSON.stringify(data).slice(0, 400)}`);
};
const save = () => writeFileSync(OUT, JSON.stringify(receipt, null, 2) + '\n');

function nodeB(command) {
  return execFileSync('ssh', ['-o', 'BatchMode=yes', '-o', 'ConnectTimeout=8', 'node-b', command], { encoding: 'utf8', maxBuffer: 16 * 1024 * 1024 }).trim();
}
function ownershipsFor(userId) {
  const list = JSON.parse(nodeB('curl -fsS -H "X-Internal-Caller: true" http://127.0.0.1:8083/internal/vmctl/list'));
  return (list?.ownerships || [])
    .filter(o => o.user_id === userId)
    .map(o => ({ state: o.state, vm_id: o.vm_id, computer_id: o.computer_id, epoch: o.epoch, computer_url: o.computer_url }));
}
const sleep = (ms) => new Promise(r => setTimeout(r, ms));

const browser = await chromium.launch();
let pass = false;
try {
  const ctx = await browser.newContext();
  const page = await ctx.newPage();
  await setupVirtualAuthenticator(page);
  const email = `sh-lose-disk-${Date.now()}-${Math.random().toString(36).slice(2, 8)}@example.com`;
  await page.goto(BASE_URL, { waitUntil: 'domcontentloaded', timeout: 60_000 });
  const reg = await registerPasskey(page, email, BASE_URL);
  if (!reg || reg.ok === false) throw new Error(`passkey registration rejected: ${JSON.stringify(reg)?.slice(0, 300)}`);
  const userId = reg.user?.id || '';
  receipt.disposable = { email, user_id: userId };

  let own = null;
  for (let i = 0; i < 200 && !(own && own.state === 'active'); i++) {
    own = ownershipsFor(userId)[0] || null;
    if (!(own && own.state === 'active')) await sleep(3000);
  }
  if (!own || own.state !== 'active') throw new Error(`computer did not reach active: ${JSON.stringify(own)}`);
  receipt.disposable.computer_id = own.computer_id;
  receipt.disposable.initial_vm_id = own.vm_id;
  const health = JSON.parse(nodeB(`curl -fsS --max-time 10 ${own.computer_url}/health`));
  step('precondition', { ownership: [own], build: health?.build?.commit });

  const apiFetch = (path, init = {}) => page.evaluate(async ({ path, init }) => {
    const res = await fetch(path, { credentials: 'include', ...init });
    return { status: res.status, text: (await res.text()).slice(0, 2000) };
  }, { path, init });

  const written = await apiFetch(`/api/files/${PROOF_FILE}`, { method: 'PUT', body: PROOF_TEXT });
  const synced = await apiFetch('/api/files/sync', { method: 'POST' });
  step('private_file_written', { written, synced });
  if (written.status !== 200 || synced.status !== 200) throw new Error('file write or sync failed');

  const removed = nodeB(`curl -sS -X POST -H "X-Internal-Caller: true" -H "Content-Type: application/json" -d '{"user_id":"${userId}","desktop_id":"primary"}' http://127.0.0.1:8083/internal/vmctl/remove`);
  step('ownership_removed', { response: removed, after: ownershipsFor(userId) });

  const deadline = Date.now() + TIMEOUT_MS;
  const last = [];
  let attempts = 0;
  let readBack = null;
  while (Date.now() < deadline) {
    attempts++;
    const res = await apiFetch(`/api/files/${PROOF_FILE}`);
    last.push({ status: res.status, at_ms: TIMEOUT_MS - (deadline - Date.now()), text: res.text.slice(0, 120) });
    if (last.length > 5) last.shift();
    if (res.status === 200 && res.text === PROOF_TEXT) { readBack = res; break; }
    await sleep(10_000);
  }
  const after = ownershipsFor(userId);
  step('file_read_back', { pass: Boolean(readBack), attempts, last, ownership: after });

  const vm = after[0]?.vm_id;
  if (vm) {
    let lines = '';
    try {
      lines = nodeB(`sudo grep -a -h -E "privacy key delivered|fresh-volume|file tree hydrated|file sync" /var/lib/go-choir/vm-state/${vm}/console.log 2>/dev/null | sed -E "s/^.*runtime\\[[0-9]+\\]: //" | tail -20`);
    } catch (err) {
      lines = `console unavailable: ${err.message}`;
    }
    let build = null;
    try { build = JSON.parse(nodeB(`curl -fsS --max-time 10 ${after[0].computer_url}/health`))?.build?.commit; } catch {}
    step('fresh_realization_console', { vm_id: vm, build, lines: lines.split('\n') });
  }
  pass = Boolean(readBack) && vm && vm !== own.vm_id;
} catch (err) {
  step('error', { message: String(err?.message || err) });
} finally {
  await browser.close();
  receipt.result = pass ? 'pass' : 'fail';
  receipt.finished_at = new Date().toISOString();
  save();
  console.log(JSON.stringify({ result: receipt.result, receipt: OUT }));
  process.exit(pass ? 0 : 1);
}
