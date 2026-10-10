// vmctl Phase 0 step 2 acceptance (docs/vmctl-360-review-2026-10-10.md):
// uncertainty never destroys a live computer. A disposable computer whose
// guest stops answering is not restarted by the open desktop, by the owner's
// wake, or by a vmctl resolve; once it answers again it is the same
// realization.
//
// The probe freezes the guest's Firecracker process with SIGSTOP (a pause, not
// a kill), waits past vmctl's 45s unhealthy grace, then resolves and wakes it.
// Neither may leave a destruction receipt for the VM. SIGCONT thaws it and the
// probe checks the guest answers at the same epoch.
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
const CALLER = 'operator.vmctl-no-escalation-probe';
const RECEIPTS = '/var/lib/go-choir/vm-state/receipts/destructive.jsonl';
const UNHEALTHY_GRACE_MS = 45_000;

const result = {
  probe: 'vmctl_no_escalation_probe',
  base_url: BASE_URL,
  started_at: new Date().toISOString(),
  predicate:
    'A disposable computer whose guest is frozen past the unhealthy grace is not destroyed by a vmctl resolve or ' +
    'the owner\'s wake (no destruction receipt), and after thawing it answers at the same epoch. The owner\'s ' +
    'explicit restart then ends it with an attributed stop receipt and boots a new epoch.',
  legs: {
    fresh_owner: false,
    computer_booted: false,
    guest_frozen: false,
    resolve_left_it_running: false,
    wake_reported_not_restarted: false,
    no_destruction_receipt: false,
    thawed_same_epoch: false,
    owner_restart_new_epoch: false,
    owner_restart_receipted: false,
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
  result.email = `vmctl-no-escalation-${Date.now()}-${Math.random().toString(36).slice(2, 8)}@example.com`;
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

  frozenPID = firecrackerPID(booted.vm_id);
  if (!frozenPID) throw new Error('no Firecracker process found for the VM');
  nodeB(`sudo kill -STOP ${frozenPID}`);
  result.frozen_at = new Date().toISOString();
  await sleep(5_000);
  result.legs.guest_frozen = !guestHealthy(booted.computer_url);

  await sleep(UNHEALTHY_GRACE_MS + 15_000);
  result.resolve_http_status = nodeB(
    `curl -sS -m 120 -o /dev/null -w '%{http_code}' -X POST -H 'Content-Type: application/json' -H 'X-Internal-Caller: true' -H 'X-Choir-Lifecycle-Caller: ${CALLER}' --data-binary @- http://127.0.0.1:8083/internal/vmctl/resolve`,
    JSON.stringify({ user_id: userID, desktop_id: 'primary' }),
  );
  const afterResolve = ownership(userID);
  result.legs.resolve_left_it_running = afterResolve?.vm_id === booted.vm_id && afterResolve?.epoch === booted.epoch &&
    firecrackerPID(booted.vm_id) === frozenPID;

  const wake = await page.evaluate(async () => {
    const res = await fetch('/api/compute/recovery', {
      method: 'POST', credentials: 'same-origin',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ action: 'wake_current_computer' }),
    });
    const status = await (await fetch('/api/compute/status', { credentials: 'same-origin' })).json().catch(() => null);
    return { http: res.status, recovery: status?.recovery || null };
  });
  result.wake = { http: wake.http, code: wake.recovery?.code || '', status: wake.recovery?.status || '' };
  result.legs.wake_reported_not_restarted = wake.http === 502 && wake.recovery?.code === 'not_restarted';

  result.receipts_before = receiptsBefore;
  result.receipts_while_frozen = receiptCount(booted.vm_id);
  result.legs.no_destruction_receipt = result.receipts_while_frozen === receiptsBefore && firecrackerPID(booted.vm_id) === frozenPID;

  nodeB(`sudo kill -CONT ${frozenPID}`);
  frozenPID = '';
  const thawed = await waitFor(() => guestHealthy(booted.computer_url), 120_000);
  const afterThaw = ownership(userID);
  result.epoch_after = afterThaw?.epoch;
  result.legs.thawed_same_epoch = Boolean(thawed) && afterThaw?.vm_id === booted.vm_id && afterThaw?.epoch === booted.epoch;

  const sinceRestart = nodeB("date -u '+%Y-%m-%d %H:%M:%S'");
  const stopsBefore = nodeB(`sudo cat ${RECEIPTS} 2>/dev/null | grep -F '"vm_id":"${booted.vm_id}"' | grep -cF '"cause":"stop"' || true`);
  result.restart_http = await page.evaluate(async (computerID) => {
    const res = await fetch(`/api/computers/${encodeURIComponent(computerID)}/lifecycle/restart`, {
      method: 'POST', credentials: 'same-origin',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ idempotency_key: `probe-restart-${Date.now()}` }),
    });
    return res.status;
  }, booted.computer_id);
  const restarted = await waitFor(() => {
    const own = ownership(userID);
    return own?.state === 'active' && own.epoch > booted.epoch ? own : null;
  }, 300_000);
  result.epoch_after_restart = restarted?.epoch;
  result.legs.owner_restart_new_epoch = Boolean(restarted);
  const stopsAfter = nodeB(`sudo cat ${RECEIPTS} 2>/dev/null | grep -F '"vm_id":"${booted.vm_id}"' | grep -cF '"cause":"stop"' || true`);
  result.restart_caller_log = nodeB(`sudo journalctl -u go-choir-vmctl --since '${sinceRestart}' --no-pager | grep -F 'lifecycle request POST /internal/vmctl/stop caller="proxy.lifecycle.restart"' | tail -1 || true`);
  result.legs.owner_restart_receipted = Number(stopsAfter) === Number(stopsBefore) + 1 && Boolean(result.restart_caller_log);

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
