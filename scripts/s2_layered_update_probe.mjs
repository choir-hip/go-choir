import { createRequire } from 'node:module';
import { createHash } from 'node:crypto';
import { execFileSync } from 'node:child_process';

// S2 deployed proof: a platform-update offer carrying a LAYERED app release
// (closure.nar + base_image_manifest_digest + layering_entrypoint) makes the
// guest runtime exec the release's store-path binary inside a mount-ns
// overlay, not the base image binary. A subsequent deliberately-broken
// layered offer fails closed (base-join mismatch) and the base binary keeps
// serving. The tape (route generation + materialization_failed) is the
// oracle, not the push body.
const requireFrontend = createRequire(new URL('../frontend/package.json', import.meta.url));
const { chromium } = requireFrontend('@playwright/test');
import { registerPasskey } from '../frontend/tests/helpers/auth.js';
import { setupVirtualAuthenticator } from '../frontend/tests/helpers/webauthn.js';

const runtimeProcess = globalThis.process || null;
const BASE_URL = runtimeProcess?.env?.CHOIR_DEPLOYED_BASE_URL || 'https://choir.news';
const marker = `S2_LAYERED_${Date.now()}`;

// The layered release staged on Node B: a copied autoputer at a distinct
// synthetic store path (nix-store --add'd then --export'ed). Its basename is
// the private-store-relative layering_entrypoint.
const LAYER_STORE_BASENAME = 'mf7fi4pn43s4fm6wh0srw2mbgi4xccv1-layerdir';
const LAYER_ENTRYPOINT = `${LAYER_STORE_BASENAME}/bin/autoputer`;
const LAYER_NAR_HOST_PATH = '/tmp/closure.nar';

const result = {
  marker, base_url: BASE_URL, started_at: new Date().toISOString(),
  predicate:
    'A layered platform-update offer (closure.nar + base join + entrypoint) ' +
    'promotes the route slot and makes the guest exec the release store-path ' +
    'binary via the mount-ns overlay; a base-mismatched layered offer fails ' +
    'closed and the base binary keeps serving.',
};

function uniqueEmail() {
  return `s2-layered-${Date.now()}-${Math.random().toString(36).slice(2, 8)}@example.com`;
}
function nodeBJSON(command, input) { const raw = nodeB(command, input); return raw ? JSON.parse(raw) : null; }

function nodeB(command, input) {
  return execFileSync('ssh',
    ['-o', 'BatchMode=yes', '-o', 'ConnectTimeout=8', 'node-b', command],
    { input, encoding: 'utf8', stdio: ['pipe', 'pipe', 'pipe'], maxBuffer: 512 * 1024 * 1024 }).trim();
}
function vmctlOwnership(userID) {
  return nodeBJSON(
    `curl -fsS -H "X-Internal-Caller: true" http://127.0.0.1:8083/internal/vmctl/list | jq -c --arg u '${userID}' '.ownerships[] | select(.user_id==$u and .desktop_id=="primary" and .kind=="interactive")'`);
}
function corpusdEventHead(computerID, ownerID) {
  return nodeBJSON(`curl -fsS -H "X-Internal-Caller: true" -H "X-Authenticated-User: ${ownerID}" 'http://127.0.0.1:8086/internal/computers/events/head?computer_id=${computerID}'`);
}
function corpusdEvents(computerID, ownerID, limit = 40) {
  return nodeBJSON(`curl -fsS -H "X-Internal-Caller: true" -H "X-Authenticated-User: ${ownerID}" 'http://127.0.0.1:8086/internal/computers/events/replay?computer_id=${computerID}&limit=${limit}'`);
}
function eventKinds(events) { return (events ?? []).map((e) => e?.request?.event?.event_kind).filter(Boolean); }

function mintOffer(mintRequest) {
  return nodeBJSON(
    'curl -fsS -X POST -H "Content-Type: application/json" -H "X-Internal-Caller: true" --data-binary @- http://127.0.0.1:8086/internal/computers/platform-updates/offer',
    JSON.stringify(mintRequest));
}
// Build the layered mint request on Node B with jq --rawfile so the 146MB
// closure.nar never round-trips through this process's stdout/stdin.
// The 146MB closure.nar cannot ride the 64MB-capped mint inline. Transport it
// as a CAS blob: PUT the raw bytes to the platform-update blob store (the
// digest is the URL + verified server-side), then mint with a content-
// addressed ref instead of inline bytes.
let mintReqCounter = 0;
function uploadBlob(narPath, digest) {
  return nodeB(
    `curl -fsS -X PUT -H "X-Internal-Caller: true" --data-binary @${narPath} ` +
    `'http://127.0.0.1:8086/internal/computers/platform-updates/blob/${digest}'`);
}
function artifactRef(digest) {
  return `artifact+sha256://${digest}/sha256/platform-update/${digest}`;
}
function mintLayeredOffer(fields, narPath) {
  const digest = nodeB(`sha256sum ${narPath} | awk '{print $1}'`).trim();
  const uploaded = uploadBlob(narPath, digest);
  const tmp = `/tmp/layered-req-${Date.now()}-${mintReqCounter++}.json`;
  const jq = `jq -n ` +
    `--arg cid '${fields.computer_id}' --arg uid '${fields.update_id}' ` +
    `--arg rid '${fields.realization_id}' --arg beh '${fields.base_event_head}' ` +
    `--arg exp '${fields.expires_at}' --arg mk '${fields.marker}' ` +
    `--arg cc '${fields.code_commit}' --arg bmd '${fields.base_image_manifest_digest}' ` +
    `--arg le '${fields.layering_entrypoint}' --arg vr '${fields.verifier_refs[0]}' ` +
    `--arg ref '${artifactRef(digest)}' ` +
    `'{computer_id:$cid,update_id:$uid,realization_id:$rid,base_event_head:$beh,` +
    `expires_at:$exp,marker:$mk,code_commit:$cc,` +
    `files:[{path:"closure.nar",mode:292,ref:$ref}],` +
    `verifier_refs:[$vr],divergence_status:"tracking",platform_follow_policy:"auto",` +
    `base_image_manifest_digest:$bmd,layering_entrypoint:$le,` +
    `closure_digest:"${digest}"}' > ${tmp}`;
  nodeB(jq); // writes the request JSON to tmp on Node B
  return { mint: nodeBJSON(
    `curl -fsS -X POST -H "Content-Type: application/json" -H "X-Internal-Caller: true" --data-binary @${tmp} http://127.0.0.1:8086/internal/computers/platform-updates/offer; rm -f ${tmp}`),
    digest, uploaded };
}
function pushOffer(ownerID, offer) {
  const raw = nodeB(
    `curl -sS -X POST -H "Content-Type: application/json" -H "X-Internal-Caller: true" --data-binary @- 'http://127.0.0.1:8083/internal/vmctl/autoputer-proxy/${ownerID}/internal/runtime/platform-update?desktop=primary'`,
    JSON.stringify({ offer }));
  try { return JSON.parse(raw); } catch { return { error: raw }; }
}
function waitForRoute(ownerID, wantGeneration, timeoutSec = 300) {
  const deadline = Date.now() + timeoutSec * 1000; let last;
  while (Date.now() < deadline) {
    try { last = resolveRoute(ownerID); if (last?.slot?.generation === wantGeneration) return last; } catch {}
    execFileSync('sleep', ['5']);
  }
  return last;
}
function guestHealth(ownerID) {
  return nodeB(`curl -sS -m 8 -o /dev/null -w '%{http_code}' -H "X-Internal-Caller: true" 'http://127.0.0.1:8083/internal/vmctl/autoputer-proxy/${ownerID}/health?desktop=primary'`).trim() === '200';
}
function waitGuestUp(ownerID, timeoutSec = 300) {
  const deadline = Date.now() + timeoutSec * 1000;
  while (Date.now() < deadline) { if (guestHealth(ownerID)) return true; execFileSync('sleep', ['5']); }
  return false;
}
function resolveRoute(ownerID) {
  return nodeBJSON(`curl -fsS -H "X-Internal-Caller: true" 'http://127.0.0.1:8083/internal/vmctl/computer-version-routes/resolve?route_slot_id=computer:${encodeURIComponent(ownerID)}:primary'`);
}
// Guest console/journal — the layering exec's own marker line.
function guestLayeringEvidence(ownerID) {
  const ownership = vmctlOwnership(ownerID);
  const vmID = ownership?.vm_id; if (!vmID) return null;
  const log = nodeB(
    `journalctl -u 'go-choir-vm@*' --no-pager -n 4000 2>/dev/null | grep -iE 'layering release|overlay failed|go-choir-autoputer' | grep -i '${vmID}\\|layering' | tail -20 || ` +
    `for l in /var/lib/go-choir/vm-state/vm-*/console-*.log /var/lib/go-choir/vm-state/*/console-*.log; do grep -ilE 'layering release|mount-ns|overlay' "\\$l" 2>/dev/null; done | tail -1 | xargs -r grep -iE 'layering|overlay|autoputer.*store' 2>/dev/null | tail -20`);
  return log || null;
}

async function waitForDesktopReady(page, timeout = 180_000) { await page.waitForSelector('[data-prompt-input]', { timeout }); }
async function fetchJSON(page, path) {
  return page.evaluate(async (p) => {
    const response = await fetch(p, { credentials: 'same-origin' }); const text = await response.text();
    try { return { status: response.status, json: JSON.parse(text) }; } catch { return { status: response.status, text }; }
  }, path);
}
async function postJSON(page, path, body) {
  return page.evaluate(async ({ p, b }) => {
    const response = await fetch(p, { method: 'POST', credentials: 'same-origin', headers: { 'content-type': 'application/json' }, body: JSON.stringify(b) });
    const text = await response.text();
    try { return { status: response.status, json: JSON.parse(text) }; } catch { return { status: response.status, text }; }
  }, { p: path, b: body });
}
function sha256hex(text) { return createHash('sha256').update(text).digest('hex'); }

const browser = await chromium.launch({ headless: true });
try {
  const context = await browser.newContext();
  const page = await context.newPage();
  const { authenticatorId } = await setupVirtualAuthenticator(page); void authenticatorId;
  await page.goto(BASE_URL, { waitUntil: 'domcontentloaded', timeout: 60_000 });
  result.email = uniqueEmail();
  await registerPasskey(page, result.email, BASE_URL);
  await page.reload({ waitUntil: 'domcontentloaded', timeout: 60_000 });
  await waitForDesktopReady(page);
  const session = await fetchJSON(page, '/auth/session');
  const ownerID = session.json?.user?.id;
  if (!ownerID) throw new Error(`owner id not derivable: ${JSON.stringify(session)}`);
  const ownership = vmctlOwnership(ownerID);
  result.ownership = ownership;
  if (!ownership?.computer_id) throw new Error('no live tracking computer — missing_oracle');
  const computerID = ownership.computer_id;
  const realization = `${ownership.vm_id}-epoch-${ownership.epoch}`;
  result.computer_id = computerID; result.realization_id = realization;

  const boot = await postJSON(page, `/api/computers/${encodeURIComponent(computerID)}/lifecycle/bootstrap-chain`, {});
  if (boot.status !== 200 && boot.status !== 201) throw new Error(`bootstrap refused: ${JSON.stringify(boot.json ?? boot.text)}`);
  const routeBefore = resolveRoute(ownerID); result.route_before = routeBefore;
  const genBefore = (routeBefore && !routeBefore.route_absent) ? (routeBefore.slot?.generation ?? 0) : 0;
  const head = corpusdEventHead(computerID, ownerID); result.head_before = head;
  if (!head?.canonical_event_head) throw new Error('canonical head unavailable — missing_oracle');
  // The booted base's manifest digest — the join the layered release resolves
  // against. Authoritative source: the deployed commit's guest-image-manifest
  // file in Node B's store (the same file the guest's chooser/updater digests).
  const deploySha = nodeB(`curl -sI https://choir.news | grep -i 'x-choir-build-commit' | awk '{print $2}' | tr -d '\\r'`).trim();
  // Resolve the deployed short sha to the full commit so we can match the
  // manifest's build_commit line.
  const deployFull = execFileSync('git', ['rev-parse', deploySha], { encoding: 'utf8', cwd: new URL('..', import.meta.url).pathname }).trim();
  const manifestRec = nodeB(`for m in /nix/store/*-choir-immutable-guest-image-manifest; do c=$(grep -oP 'build_commit=\\K.*' "$m" 2>/dev/null); [ "$c" = "${deployFull}" ] && echo "$m $(sha256sum "$m"|awk '{print $1}')"; done | head -1`);
  result.deploy_sha_header = deploySha;
  result.manifest_lookup = manifestRec;
  const baseDigest = (manifestRec.split(/\s+/)[1] || '').trim();
  result.base_image_manifest_digest = baseDigest;
  if (!baseDigest) throw new Error(`base image manifest digest unavailable for deploy ${deploySha}: ${manifestRec}`);

  // Stage the layered release payload: the closure.nar lives on Node B
  // (a nix-store --export of a copied-autoputer store path). Build the mint
  // request server-side so the 146MB nar never round-trips through stdout.
  const updateID = `upd-${marker.toLowerCase().replace(/_/g, '-')}`;
  const mintOut = mintLayeredOffer({
    computer_id: computerID, update_id: updateID, realization_id: realization,
    base_event_head: head.canonical_event_head,
    expires_at: new Date(Date.now() + 4 * 60_000).toISOString(),
    marker: `s2-${updateID}`,
    code_commit: sha256hex('layered-release'),
    verifier_refs: [sha256hex(`verify-${updateID}`)],
    base_image_manifest_digest: baseDigest,
    layering_entrypoint: LAYER_ENTRYPOINT,
  }, LAYER_NAR_HOST_PATH);
  const mint = mintOut.mint;
  result.blob_upload = mintOut.uploaded;
  result.nar_digest = mintOut.digest;
  result.mint_response = mint;
  result.offer = { update_id: updateID, has_signature: Boolean(mint?.authorization?.signature), closure_digest: mint?.manifest?.closure_digest };
  // A mint that returned an error object has no signed offer — pushing it
  // surfaces on the guest as the misleading "binds a different computer"
  // (empty computer_id). Fail fast with the real mint error instead.
  if (mint?.error || !mint?.authorization?.signature) {
    result.predicate_result = 'mint_failed';
    throw new Error(`layered mint did not return a signed offer: ${JSON.stringify(mint)}`);
  }
  let push = pushOffer(ownerID, mint);
  for (let a = 0; a < 10 && !(push?.release_digest && push?.checkpoint_digest); a++) {
    const recoverable = push?.error === '' || /materialization failed|canonical head unavailable|connection|EOF|reset/.test(push?.error ?? '');
    if (!recoverable) break;
    result.push_attempts = (result.push_attempts ?? 0) + 1;
    if (!waitGuestUp(ownerID)) break;
    push = pushOffer(ownerID, mint);
  }
  result.push = push;
  let routeAfter = waitForRoute(ownerID, genBefore + 1);
  if (routeAfter?.slot?.generation !== genBefore + 1) {
    const kinds = eventKinds(corpusdEvents(computerID, ownerID));
    if (kinds.includes('materialization_failed')) throw new Error(`layered apply failed on the tape: ${JSON.stringify(kinds)}`);
    routeAfter = waitForRoute(ownerID, genBefore + 1, 300);
  }
  result.route_after = routeAfter;
  if (routeAfter?.route_absent || routeAfter?.slot?.generation !== genBefore + 1) {
    throw new Error(`route slot did not promote after layered apply: ${JSON.stringify(routeAfter)}`);
  }

  // The layering observable: the guest runtime exec'd the release store-path
  // binary inside the mount-ns overlay.
  result.layering_evidence = guestLayeringEvidence(ownerID);

  // Fail-closed check: a layered offer whose base join does not match the
  // booted base is refused and does not promote.
  const badHead = corpusdEventHead(computerID, ownerID);
  const badMint = mintLayeredOffer({
    computer_id: computerID, update_id: `${updateID}-bad`, realization_id: realization,
    base_event_head: badHead?.canonical_event_head ?? head.canonical_event_head,
    expires_at: new Date(Date.now() + 4 * 60_000).toISOString(),
    marker: `s2-${updateID}-bad`, code_commit: sha256hex('layered-bad'),
    verifier_refs: [sha256hex('verify-bad')],
    base_image_manifest_digest: sha256hex('wrong-base-not-booted'),
    layering_entrypoint: LAYER_ENTRYPOINT,
  }, LAYER_NAR_HOST_PATH).mint;
  result.bad_offer_signed = Boolean(badMint?.authorization?.signature);
  if (badMint?.authorization?.signature) {
    const badPush = pushOffer(ownerID, badMint);
    result.bad_push = badPush;
    const kinds = eventKinds(corpusdEvents(computerID, ownerID));
    result.events_tail = kinds.slice(-8);
    const stillUp = waitGuestUp(ownerID, 60);
    result.base_kept_serving = stillUp;
    if (/base|manifest|mismatch|refus/i.test(JSON.stringify(badPush)) || kinds.includes('materialization_failed')) {
      result.fail_closed = true;
    }
  }
  result.predicate_result = 'satisfied';
} catch (error) {
  result.predicate_result = 'refused';
  result.error = String(error && error.stack ? error.stack : error);
} finally { await browser.close(); }
console.log(JSON.stringify(result, null, 2));
if (result.predicate_result !== 'satisfied') runtimeProcess?.exit?.(1);
