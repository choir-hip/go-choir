// vmctl Phase 0 step 1 acceptance (docs/vmctl-360-review-2026-10-10.md):
// every Firecracker kill on Node B leaves an attributable destruction receipt,
// and a destructive lifecycle call names its caller in vmctl's log.
//
// Runs on a fresh disposable owner (example.com, so retention treats it as
// ephemeral): boot its computer through the product path, close the browser
// (the desktop's automatic cold-recover is not disarmed yet and would add its
// own lifecycle calls), refresh then stop the computer through vmctl with a
// named caller, and read back both receipts and both log lines.
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
const CALLER = 'operator.vmctl-receipt-probe';
const RECEIPTS = '/var/lib/go-choir/vm-state/receipts/destructive.jsonl';

const result = {
  probe: 'vmctl_receipt_probe',
  base_url: BASE_URL,
  started_at: new Date().toISOString(),
  predicate:
    'A refresh and a stop of a disposable computer each leave a destruction receipt naming the VM, its cause, ' +
    'and a call chain through the vmctl lifecycle method, and vmctl logs each request with its named caller.',
  legs: { fresh_owner: false, computer_booted: false, refresh_receipt: false, refresh_caller_logged: false, stop_receipt: false, stop_caller_logged: false },
};

function nodeB(command, input) {
  return execFileSync('ssh', ['-o', 'BatchMode=yes', '-o', 'ConnectTimeout=20', 'node-b', command], {
    input, encoding: 'utf8', stdio: ['pipe', 'pipe', 'pipe'], maxBuffer: 8 * 1024 * 1024,
  }).trim();
}

function vmctlPost(path, body) {
  return nodeB(
    `curl -sS -o /dev/null -w '%{http_code}' -X POST -H 'Content-Type: application/json' -H 'X-Internal-Caller: true' -H 'X-Choir-Lifecycle-Caller: ${CALLER}' --data-binary @- http://127.0.0.1:8083${path}`,
    JSON.stringify(body),
  );
}

function ownership(userID) {
  const raw = nodeB(`curl -fsS -H 'X-Internal-Caller: true' http://127.0.0.1:8083/internal/vmctl/list | jq -c --arg u '${userID}' '.ownerships[] | select(.user_id==$u and .desktop_id=="primary" and .kind=="interactive")'`);
  return raw ? JSON.parse(raw) : null;
}

function receiptsFor(vmID) {
  const raw = nodeB(`sudo cat ${RECEIPTS} 2>/dev/null | grep -F '"vm_id":"${vmID}"' || true`);
  return raw ? raw.split('\n').map((line) => JSON.parse(line)) : [];
}

function callerLogged(path, since) {
  const raw = nodeB(`sudo journalctl -u go-choir-vmctl --since '${since}' --no-pager | grep -F 'lifecycle request POST ${path} caller="${CALLER}"' | tail -1 || true`);
  return raw;
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

try {
  const browser = await chromium.launch({ headless: true });
  let userID = '';
  try {
    const page = await (await browser.newContext()).newPage();
    await setupVirtualAuthenticator(page);
    await page.goto(BASE_URL, { waitUntil: 'domcontentloaded', timeout: 60_000 });
    result.email = `vmctl-receipt-${Date.now()}-${Math.random().toString(36).slice(2, 8)}@example.com`;
    await registerPasskey(page, result.email, BASE_URL);
    await page.reload({ waitUntil: 'domcontentloaded', timeout: 60_000 });
    await page.waitForSelector('[data-prompt-input]', { timeout: 300_000 });
    const session = await page.evaluate(async () => (await fetch('/auth/session', { credentials: 'same-origin' })).json());
    userID = session?.user?.id || '';
    if (!userID) throw new Error('no owner id from /auth/session');
    result.legs.fresh_owner = true;
  } finally {
    await browser.close();
  }

  const booted = await waitFor(() => {
    const own = ownership(userID);
    return own?.vm_id && own.state === 'active' ? own : null;
  }, 300_000);
  if (!booted) throw new Error('fresh computer never reached active');
  result.vm_id = booted.vm_id;
  result.computer_id = booted.computer_id;
  result.legs.computer_booted = true;

  const sinceRefresh = nodeB("date -u '+%Y-%m-%d %H:%M:%S'");
  result.refresh_http_status = vmctlPost('/internal/vmctl/refresh', { user_id: userID, desktop_id: 'primary' });
  const refreshReceipt = await waitFor(() => receiptsFor(booted.vm_id).find((r) => r.cause === 'refresh'), 60_000);
  result.refresh_receipt = refreshReceipt;
  result.legs.refresh_receipt = Boolean(refreshReceipt && (refreshReceipt.stack || []).some((frame) => frame.includes('RefreshVMForDesktop')));
  result.refresh_caller_log = callerLogged('/internal/vmctl/refresh', sinceRefresh);
  result.legs.refresh_caller_logged = Boolean(result.refresh_caller_log);

  await waitFor(() => ownership(userID)?.state === 'active', 300_000);
  const sinceStop = nodeB("date -u '+%Y-%m-%d %H:%M:%S'");
  result.stop_http_status = vmctlPost('/internal/vmctl/stop', { user_id: userID, desktop_id: 'primary' });
  const stopReceipt = await waitFor(() => receiptsFor(booted.vm_id).find((r) => r.cause === 'stop'), 60_000);
  result.stop_receipt = stopReceipt;
  result.legs.stop_receipt = Boolean(stopReceipt && (stopReceipt.stack || []).some((frame) => frame.includes('StopVMForDesktop')));
  result.stop_caller_log = callerLogged('/internal/vmctl/stop', sinceStop);
  result.legs.stop_caller_logged = Boolean(result.stop_caller_log);
} catch (error) {
  result.error = String(error?.stack || error);
}

result.finished_at = new Date().toISOString();
result.predicate_result = Object.values(result.legs).every(Boolean) ? 'satisfied' : 'not_satisfied';
console.log(JSON.stringify(result, null, 2));
if (result.predicate_result !== 'satisfied') runtimeProcess?.exit?.(1);
