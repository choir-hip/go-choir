// vmctl wedge recovery acceptance (owner ruling 2026-10-10,
// docs/problems/owner-computer-stranded-after-vmctl-restart-2026-10-10.md):
// a computer that stops answering recovers by itself, with no owner action.
//
// The probe freezes a disposable guest's Firecracker process (SIGSTOP), so the
// guest refuses even a TCP connection. vmctl must leave it alone inside the
// five-minute wedge window, then stop it by itself (stopped_by wedged, with a
// destruction receipt). A plain page reload then boots it at a new epoch.
//
// Output: one JSON receipt on stdout; exit 1 unless every leg holds.
import { execFileSync } from 'node:child_process';
import { createRequire } from 'node:module';
import { registerPasskey } from '../frontend/tests/helpers/auth.js';
import { setupVirtualAuthenticator } from '../frontend/tests/helpers/webauthn.js';

const requireFrontend = createRequire(new URL('../frontend/package.json', import.meta.url));
const { chromium } = requireFrontend('@playwright/test');

const runtimeProcess = globalThis.process || null;
const BASE_URL = runtimeProcess?.env?.CHOIR_DEPLOYED_BASE_URL || 'https://choir.news';
const CALLER = 'operator.vmctl-wedge-probe';
const RECEIPTS = '/var/lib/go-choir/vm-state/receipts/destructive.jsonl';

const result = {
  probe: 'vmctl_wedge_recovery_probe',
  base_url: BASE_URL,
  started_at: new Date().toISOString(),
  predicate:
    'A disposable computer frozen so it refuses connections is left alone for the wedge window, then stopped by ' +
    'vmctl itself as wedged with a destruction receipt, and a plain page reload boots it at a new epoch.',
  legs: {
    fresh_owner: false,
    computer_booted: false,
    guest_frozen: false,
    untouched_inside_window: false,
    stopped_as_wedged: false,
    wedge_receipted: false,
    reload_boots_new_epoch: false,
  },
};

function nodeB(command, input) {
  return execFileSync('ssh', ['-o', 'BatchMode=yes', '-o', 'ConnectTimeout=20', 'node-b', command], {
    input, encoding: 'utf8', stdio: ['pipe', 'pipe', 'pipe'], maxBuffer: 8 * 1024 * 1024,
  }).trim();
}

function ownership(userID) {
  const raw = nodeB(`curl -fsS -H 'X-Internal-Caller: true' http://127.0.0.1:8083/internal/vmctl/list | jq -c --arg u '${userID}' '.ownerships[] | select(.user_id==$u and .desktop_id=="primary" and .kind=="interactive")'`);
  return raw ? JSON.parse(raw) : null;
}

function receiptCount(vmID) {
  return Number(nodeB(`sudo cat ${RECEIPTS} ${RECEIPTS}.1 2>/dev/null | grep -cF '"vm_id":"${vmID}"' || true`) || '0');
}

function firecrackerPID(vmID) {
  // The guest's Firecracker process names its VM id on its command line.
  return nodeB(`pgrep -f -- '[f]irecracker .*--id ${vmID}( |$)' | head -1 || true`);
}

function guestHealthy(computerURL) {
  return nodeB(`curl -s -m 5 -o /dev/null -w '%{http_code}' ${computerURL}/health || true`) === '200';
}

const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

async function waitFor(check, timeoutMs, stepMs = 3000) {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    const value = check();
    if (value) return value;
    await sleep(stepMs);
  }
  return null;
}

let browser;
let frozenPID = '';
try {
  browser = await chromium.launch({ headless: true });
  const page = await (await browser.newContext()).newPage();
  await setupVirtualAuthenticator(page);
  await page.goto(BASE_URL, { waitUntil: 'domcontentloaded', timeout: 60_000 });
  result.email = `vmctl-wedge-${Date.now()}-${Math.random().toString(36).slice(2, 8)}@example.com`;
  await registerPasskey(page, result.email, BASE_URL);
  await page.reload({ waitUntil: 'domcontentloaded', timeout: 60_000 });
  await page.waitForSelector('[data-prompt-input]', { timeout: 300_000 });
  const session = await page.evaluate(async () => (await fetch('/auth/session', { credentials: 'same-origin' })).json());
  const userID = session?.user?.id || '';
  if (!userID) throw new Error('no owner id from /auth/session');
  result.legs.fresh_owner = true;

  const booted = await waitFor(() => {
    const own = ownership(userID);
    return own?.vm_id && own.state === 'active' && guestHealthy(own.computer_url) ? own : null;
  }, 300_000);
  if (!booted) throw new Error('fresh computer never reached active and healthy');
  result.vm_id = booted.vm_id;
  result.epoch_before = booted.epoch;
  result.legs.computer_booted = true;
  const receiptsBefore = receiptCount(booted.vm_id);
  // Leave the desktop so its probes do not drive resolves during the freeze.
  await page.goto('about:blank');

  frozenPID = firecrackerPID(booted.vm_id);
  if (!frozenPID) throw new Error('no Firecracker process found for the VM');
  nodeB(`sudo kill -STOP ${frozenPID}`);
  result.frozen_at = new Date().toISOString();
  await sleep(5_000);
  result.legs.guest_frozen = !guestHealthy(booted.computer_url);

  await sleep(3 * 60_000);
  result.legs.untouched_inside_window = firecrackerPID(booted.vm_id) === frozenPID && receiptCount(booted.vm_id) === receiptsBefore;

  const stopped = await waitFor(() => {
    const own = ownership(userID);
    return own?.state === 'stopped' && own.stopped_by === 'wedged' ? own : null;
  }, 9 * 60_000, 10_000);
  result.stopped_at = new Date().toISOString();
  result.legs.stopped_as_wedged = Boolean(stopped);
  if (stopped) frozenPID = '';
  result.receipts_after = receiptCount(booted.vm_id);
  result.legs.wedge_receipted = result.receipts_after === receiptsBefore + 1 && !firecrackerPID(booted.vm_id);

  await page.goto(BASE_URL, { waitUntil: 'domcontentloaded', timeout: 120_000 }).catch(() => {});
  const rebooted = await waitFor(() => {
    const own = ownership(userID);
    return own?.state === 'active' && own.epoch > booted.epoch && guestHealthy(own.computer_url) ? own : null;
  }, 300_000);
  result.epoch_after = rebooted?.epoch;
  result.legs.reload_boots_new_epoch = Boolean(rebooted);

  nodeB(
    `curl -sS -o /dev/null -X POST -H 'Content-Type: application/json' -H 'X-Internal-Caller: true' -H 'X-Choir-Lifecycle-Caller: ${CALLER}' --data-binary @- http://127.0.0.1:8083/internal/vmctl/stop`,
    JSON.stringify({ user_id: userID, desktop_id: 'primary' }),
  );
} catch (error) {
  result.error = String(error?.stack || error);
} finally {
  if (frozenPID) {
    try { nodeB(`sudo kill -CONT ${frozenPID}`); } catch { /* best effort thaw */ }
  }
  await browser?.close();
}

result.finished_at = new Date().toISOString();
result.predicate_result = Object.values(result.legs).every(Boolean) ? 'satisfied' : 'not_satisfied';
console.log(JSON.stringify(result, null, 2));
if (result.predicate_result !== 'satisfied') runtimeProcess?.exit?.(1);
