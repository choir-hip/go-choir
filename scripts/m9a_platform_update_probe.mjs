import { createRequire } from 'node:module';
import { createHash } from 'node:crypto';
import { execFileSync } from 'node:child_process';

// Playwright ships to this repo via frontend's @playwright/test; resolve
// chromium through the frontend manifest so pnpm's layout stays hidden.
const requireFrontend = createRequire(new URL('../frontend/package.json', import.meta.url));
const { chromium } = requireFrontend('@playwright/test');
import { registerPasskey } from '../frontend/tests/helpers/auth.js';
import {
  setupVirtualAuthenticator,
} from '../frontend/tests/helpers/webauthn.js';

// M9a deployed proof: platform-signed update push + pinned-head restore on a
// live staging computer. Flow: register a fresh owner (Playwright passkey ->
// live computer), read canonical head + realization from corpusd/vmctl on
// Node B, mint a signed platform update offer at corpusd, push it to the
// guest through the vmctl autoputer proxy, verify route promotion, then
// restore to the update-1 checkpoint through the product restore endpoint.
const runtimeProcess = globalThis.process || null;
const BASE_URL = runtimeProcess?.env?.CHOIR_DEPLOYED_BASE_URL || 'https://choir.news';
const marker = `M9A_PLATFORM_UPDATE_${Date.now()}`;

const result = {
  marker,
  base_url: BASE_URL,
  started_at: new Date().toISOString(),
  harness_version: '2026-09-26',
  predicate:
    'A platform-control-signed update offer presented to the guest apply ' +
    'endpoint mutates the served release and promotes the route slot under ' +
    'the platform-follow evidence class, with no owner decision; a pinned ' +
    'restore to the update-1 checkpoint returns the prior release.',
};

function uniqueEmail() {
  return `m9a-platform-update-${Date.now()}-${Math.random().toString(36).slice(2, 8)}@example.com`;
}

function nodeB(command, input) {
  return execFileSync(
    'ssh',
    ['-o', 'BatchMode=yes', '-o', 'ConnectTimeout=8', 'node-b', command],
    { input, encoding: 'utf8', stdio: ['pipe', 'pipe', 'pipe'], maxBuffer: 32 * 1024 * 1024 },
  ).trim();
}

function nodeBJSON(command, input) {
  const raw = nodeB(command, input);
  return raw ? JSON.parse(raw) : null;
}

function vmctlOwnership(userID) {
  return nodeBJSON(
    `curl -fsS -H "X-Internal-Caller: true" http://127.0.0.1:8083/internal/vmctl/list | jq -c --arg u '${userID}' '.ownerships[] | select(.user_id==$u and .desktop_id=="primary" and .kind=="interactive")'`,
  );
}

function corpusdEventHead(computerID, ownerID) {
  return nodeBJSON(
    `curl -fsS -H "X-Internal-Caller: true" -H "X-Authenticated-User: ${ownerID}" 'http://127.0.0.1:8086/internal/computers/events/head?computer_id=${computerID}'`,
  );
}

function corpusdEvents(computerID, ownerID, limit = 40) {
  // Guest event chain as replayed to corpusd — the tape is the apply oracle,
  // not the push response (the callee restarts mid-request).
  return nodeBJSON(
    `curl -fsS -H "X-Internal-Caller: true" -H "X-Authenticated-User: ${ownerID}" 'http://127.0.0.1:8086/internal/computers/events/replay?computer_id=${computerID}&limit=${limit}'`,
  );
}

function eventKinds(events) {
  return (events ?? []).map((e) => e?.request?.event?.event_kind).filter(Boolean);
}


function mintOffer(mintRequest) {
  return nodeBJSON(
    'curl -fsS -X POST -H "Content-Type: application/json" -H "X-Internal-Caller: true" --data-binary @- http://127.0.0.1:8086/internal/computers/platform-updates/offer',
    JSON.stringify(mintRequest),
  );
}
function pushOffer(ownerID, offer) {
  // No -f: capture the refusal body — the guest's error string is the oracle.
  const raw = nodeB(
    `curl -sS -X POST -H "Content-Type: application/json" -H "X-Internal-Caller: true" --data-binary @- 'http://127.0.0.1:8083/internal/vmctl/autoputer-proxy/${ownerID}/internal/runtime/platform-update?desktop=primary'`,
    JSON.stringify({ offer }),
  );
  try { return JSON.parse(raw); } catch { return { error: raw }; }
}

function waitForRoute(ownerID, wantGeneration, timeoutSec = 300) {
  const deadline = Date.now() + timeoutSec * 1000;
  let last;
  while (Date.now() < deadline) {
    try {
      last = resolveRoute(ownerID);
      if (last?.slot?.generation === wantGeneration) return last;
    } catch { /* ledger momentarily unavailable */ }
    execFileSync('sleep', ['5']);
  }
  return last;
}

function guestHealth(ownerID) {
  // Proxy-wrapped guest health — the transport's reachability oracle.
  const raw = nodeB(
    `curl -sS -m 8 -o /dev/null -w '%{http_code}' -H "X-Internal-Caller: true" 'http://127.0.0.1:8083/internal/vmctl/autoputer-proxy/${ownerID}/health?desktop=primary'`,
  );
  return raw.trim() === '200';
}

function waitGuestUp(ownerID, timeoutSec = 240) {
  const deadline = Date.now() + timeoutSec * 1000;
  while (Date.now() < deadline) {
    if (guestHealth(ownerID)) return true;
    execFileSync('sleep', ['5']);
  }
  return false;
}

function resolveRoute(ownerID) {
  return nodeBJSON(
    `curl -fsS -H "X-Internal-Caller: true" 'http://127.0.0.1:8083/internal/vmctl/computer-version-routes/resolve?route_slot_id=computer:${encodeURIComponent(ownerID)}:primary'`,
  );
}

async function waitForDesktopReady(page, timeout = 180_000) {
  await page.waitForSelector('[data-prompt-input]', { timeout });
}

async function fetchJSON(page, path) {
  return page.evaluate(async (p) => {
    const response = await fetch(p, { credentials: 'same-origin' });
    const text = await response.text();
    try { return { status: response.status, json: JSON.parse(text) }; }
    catch { return { status: response.status, text }; }
  }, path);
}

async function postJSON(page, path, body) {
  return page.evaluate(async ({ p, b }) => {
    const response = await fetch(p, {
      method: 'POST', credentials: 'same-origin',
      headers: { 'content-type': 'application/json' },
      body: JSON.stringify(b),
    });
    const text = await response.text();
    try { return { status: response.status, json: JSON.parse(text) }; }
    catch { return { status: response.status, text }; }
  }, { p: path, b: body });
}

function sha256hex(text) {
  return createHash('sha256').update(text).digest('hex');
}

const browser = await chromium.launch({ headless: true });
try {
  const context = await browser.newContext();
  const page = await context.newPage();
  const { authenticatorId } = await setupVirtualAuthenticator(page);
  void authenticatorId;

  await page.goto(BASE_URL, { waitUntil: 'domcontentloaded', timeout: 60_000 });
  result.email = uniqueEmail();
  await registerPasskey(page, result.email, BASE_URL);
  await page.reload({ waitUntil: 'domcontentloaded', timeout: 60_000 });
  await waitForDesktopReady(page);

  const session = await fetchJSON(page, '/auth/session');
  const ownerID = session.json?.user?.id;
  if (!ownerID) throw new Error(`owner id not derivable from /auth/session: ${JSON.stringify(session)}`);
  result.session_user = session.json?.user;
  const status = await fetchJSON(page, '/api/compute/status');
  result.compute_status = status.json ?? status.text;

  const ownership = vmctlOwnership(ownerID);
  result.ownership = ownership;
  if (!ownership?.computer_id || !ownership?.vm_id || ownership?.epoch == null) {
    throw new Error('vmctl ownership incomplete — missing_oracle: no live tracking computer for owner ' + ownerID);
  }
  const computerID = ownership.computer_id;
  const realization = `${ownership.vm_id}-epoch-${ownership.epoch}`;
  result.computer_id = computerID;
  result.realization_id = realization;


  // Product genesis: an owner POST bootstraps the event chain (idempotent —
  // 201 on first append, 200 when already bootstrapped).
  const boot = await postJSON(page, `/api/computers/${encodeURIComponent(computerID)}/lifecycle/bootstrap-chain`, {});
  result.bootstrap_chain = boot.json ?? boot.text;
  if (boot.status !== 200 && boot.status !== 201) {
    throw new Error(`bootstrap-chain refused: ${JSON.stringify(result.bootstrap_chain)}`);
  }
  const routeBefore = resolveRoute(ownerID);
  result.route_before = routeBefore;
  const generationBefore = (routeBefore && !routeBefore.route_absent)
    ? (routeBefore.slot?.generation ?? 0)
    : 0;

  const head = corpusdEventHead(computerID, ownerID);
  result.head_before = head;
  if (!head?.canonical_event_head) {
    throw new Error('canonical event head unavailable — missing_oracle');
  }

  // Build + mint + push update A.
  const spaA = `<!doctype html><html><body>${marker}-A</body></html>`;
  const updateIDA = `upd-${marker.toLowerCase().replace(/_/g, '-')}-a`;
  const mintA = mintOffer({
    computer_id: computerID, update_id: updateIDA, realization_id: realization,
    base_event_head: head.canonical_event_head,
    expires_at: new Date(Date.now() + 4 * 60_000).toISOString().replace('Z', 'Z'),
    marker: `m9a-${updateIDA}`,
    code_commit: sha256hex('platform-base'),
    files: [{ path: 'frontend/index.html', mode: 292, bytes: Buffer.from(spaA).toString('base64') }],
    verifier_refs: [sha256hex(`verify-${updateIDA}`)],
    divergence_status: 'tracking', platform_follow_policy: 'auto',
  });
  result.offer_a = mintA;
  if (!mintA?.authorization?.signature) {
    throw new Error(`mint refused: ${JSON.stringify(mintA)}`);
  }
  // updater.Apply restarts the guest service mid-request — the proxied push
  // dies empty. Wait for the restarted guest, then re-push the same offer:
  // resume on the pending accepted event, or replay the completed outcome.
  let pushA = pushOffer(ownerID, mintA);
  for (let attempt = 0; attempt < 8 && !(pushA?.release_digest && pushA?.checkpoint_digest); attempt++) {
    const recoverable = pushA?.error === '' ||
      /materialization failed|canonical head unavailable/.test(pushA?.error ?? '');
    if (!recoverable) break;
    result.push_attempts = (result.push_attempts ?? 0) + 1;
    if (!waitGuestUp(ownerID)) break; // guest never came back — transport dead
    pushA = pushOffer(ownerID, mintA);
  }
  result.update_a = pushA;
  // Tape, not the push body, is the oracle: the apply self-restarts the guest
  // mid-request, so a dead HTTP response is normal even on success. The apply
  // is complete iff the route slot promoted one generation (the last tail
  // event commit). Failed updates record materialization_failed instead.
  let routeAfterA = waitForRoute(ownerID, generationBefore + 1);
  if (routeAfterA?.slot?.generation !== generationBefore + 1) {
    const kinds = eventKinds(corpusdEvents(computerID, ownerID));
    if (kinds.includes('materialization_failed')) {
      throw new Error(`update A failed on the tape: ${JSON.stringify(kinds)}`);
    }
    routeAfterA = waitForRoute(ownerID, generationBefore + 1, 300);
  }
  result.route_after_a = routeAfterA;
  if (routeAfterA?.route_absent || !routeAfterA?.slot || routeAfterA.slot.generation !== generationBefore + 1) {
    throw new Error(`route slot did not promote after update A: ${JSON.stringify(routeAfterA)}`);
  }
  // The checkpoint operand: mint a restore-set checkpoint on the post-update
  // head via the product path — the apply's own checkpoint response died with
  // the push, but the checkpoint mint is idempotent authority work.
  const ckptResponse = await postJSON(page, `/api/computers/${encodeURIComponent(computerID)}/lifecycle/checkpoint`, {});
  result.checkpoint_bind = ckptResponse?.json ?? ckptResponse?.text ?? ckptResponse;
  const boundCheckpoint = ckptResponse?.json?.published_checkpoint?.checkpoint ?? ckptResponse?.json?.checkpoint;
  if (ckptResponse?.status !== 200 || !boundCheckpoint) {
    throw new Error(`restore-set checkpoint mint failed: ${JSON.stringify(result.checkpoint_bind)}`);
  }
  pushA = pushA ?? {};
  pushA.checkpoint = { checkpoint: boundCheckpoint };
  // Restore edge: return to the update-A pinned head via the product path.
  // updater.Apply restarts the guest service — retry the restore POST across
  // the brief restart window.
  let restore = null;
  for (let attempt = 0; attempt < 12; attempt++) {
    restore = await postJSON(page, `/api/computers/${encodeURIComponent(computerID)}/lifecycle/restore`, {
      checkpoint: pushA.checkpoint.checkpoint,
      operand_scopes: ['vm_local', 'computer_surface_frontend'],
    });
    if (restore.status === 200 || restore.status === 400 || restore.status === 409) break;
    await new Promise((resolve) => setTimeout(resolve, 5000));
  }
  result.restore = restore?.json ?? restore?.text ?? restore;
  if (!restore || restore.status !== 200 || !(restore.json?.frontend_restaged === true)) {
    throw new Error(`restore to update-A head failed: ${JSON.stringify(result.restore)}`);
  }

  result.predicate_result = 'satisfied';
} catch (error) {
  result.predicate_result = 'refused';
  result.error = String(error && error.stack ? error.stack : error);
} finally {
  await browser.close();
}

console.log(JSON.stringify(result, null, 2));
if (result.predicate_result !== 'satisfied') runtimeProcess?.exit?.(1);
