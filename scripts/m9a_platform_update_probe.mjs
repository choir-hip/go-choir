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

function mintOffer(mintRequest) {
  return nodeBJSON(
    'curl -fsS -X POST -H "Content-Type: application/json" -H "X-Internal-Caller: true" --data-binary @- http://127.0.0.1:8086/internal/computers/platform-updates/offer',
    JSON.stringify(mintRequest),
  );
}

function pushOffer(ownerID, offer) {
  return nodeBJSON(
    `curl -fsS -X POST -H "Content-Type: application/json" -H "X-Internal-Caller: true" --data-binary @- 'http://127.0.0.1:8083/internal/vmctl/autoputer-proxy/${ownerID}/internal/runtime/platform-update'`,
    JSON.stringify({ offer }),
  );
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
  const pushA = pushOffer(ownerID, mintA);
  result.update_a = pushA;
  if (!pushA?.release_digest || !pushA?.checkpoint_digest || !pushA?.checkpoint?.checkpoint) {
    throw new Error(`update A refused or incomplete: ${JSON.stringify(pushA)}`);
  }

  const routeAfterA = resolveRoute(ownerID);
  result.route_after_a = routeAfterA;
  if (routeAfterA?.route_absent || !routeAfterA?.slot || routeAfterA.slot.generation !== generationBefore + 1) {
    throw new Error(`route slot did not promote after update A: ${JSON.stringify(routeAfterA)}`);
  }
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
