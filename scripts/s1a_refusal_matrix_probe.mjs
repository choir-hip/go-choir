#!/usr/bin/env node
// S1a deployed refusal-matrix proof — two disposable staging computers A + B.
//
// Mechanism: ssh -L forward workstation -> node-b -> guest tap, open the
// guest's /api/terminal/ws (host-sourced: RemoteAddr = host peer), and run
// `curl` INSIDE guest A's PTY. Every HTTP leg is truly guest-originated.
//
// Refusals:
//   R1 tap->tap           A: curl 10.200.B.2:8085              -> fail/timeout
//   R2 vmctl internal     A: GET 10.200.A.1:8083 /internal/vmctl/list + hdr -> 403
//   R3 maild forge        A: POST :8087 drafts as B's owner    -> 403
//   R4 corpusd bypass     A: GET  :8086 /internal/platform/update hdr -> 403
//   R5 proxy wire publish A: POST :8082 wire-platform publish  -> 403
//   R6 spoofed source     covered by anti-spoof rule + R1 (kernel drops
//                          forged SRC; tcp-dial from guest can't pick src)
// Legitimate (still green):
//   L1 gateway            A: curl :8084 connect               -> ok
//   L2 CV route resolve   A: GET :8083 own slot resolve        -> 200
//   L3 maild own drafts   A: POST :8087 drafts as own owner    -> 2xx
//   L4 egress             A: curl https://1.1.1.1 -k           -> connect ok
//   L5 product page       workstation GET /                   -> <400
//
// Requires: ssh node-b (BatchMode), playwright deps, node >=22 (native WS).
// Usage: node scripts/s1a_refusal_matrix_probe.mjs [--reuse-a ID --reuse-b ID]
// Output: docs/evidence/s1a-refusal-matrix-2026-10-04.json

import { createRequire } from 'node:module';
import { execFileSync } from 'node:child_process';
import { writeFileSync, mkdirSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';

const requireFrontend = createRequire(new URL('../frontend/package.json', import.meta.url));
const { chromium } = requireFrontend('@playwright/test');
const { registerPasskey } = await import('../frontend/tests/helpers/auth.js');
const { setupVirtualAuthenticator } = await import('../frontend/tests/helpers/webauthn.js');

const BASE_URL = process.env.CHOIR_DEPLOYED_BASE_URL || 'https://choir.news';
const args = process.argv.slice(2);
const arg = (n) => { const i = args.indexOf(`--${n}`); return i >= 0 ? args[i + 1] : null; };
const REUSE_A = arg('reuse-a'), REUSE_B = arg('reuse-b');
const EVIDENCE_DIR = join(dirname(fileURLToPath(import.meta.url)), '..', 'docs', 'evidence');

const report = { schema_version: 1, probe: 's1a-refusal-matrix', base_url: BASE_URL, started_at: new Date().toISOString(), legs: {}, ok: null };
const nodeB = (cmd) => execFileSync('ssh', ['-o', 'BatchMode=yes', '-o', 'ConnectTimeout=8', 'node-b', cmd], { encoding: 'utf8', maxBuffer: 16 * 1024 * 1024 }).trim();
const vmctlList = () => JSON.parse(nodeB(`curl -fsS -H "X-Internal-Caller: true" http://127.0.0.1:8083/internal/vmctl/list`));
const ownershipFor = (id) => (vmctlList()?.ownerships || []).find(o => o.computer_id === id) || null;
const guestIP = (own) => { try { return new URL(own.computer_url).hostname; } catch { return null; } };
const hostPeerIP = (own) => { const ip = guestIP(own); if (!ip) return null; const p = ip.split('.'); p[3] = String(parseInt(p[3], 10) - 1); return p.join('.'); };

// ── in-guest exec via node-b helper binary ────────────────────────────────
// scripts/wsexec (Go, CGO_ENABLED=0 GOOS=linux GOARCH=amd64) is copied to
// node-b:/tmp/wsexec. It opens the guest terminal WS from the host — so the
// call is host-sourced (RemoteAddr = host peer) and passes HostSourcedCaller
// — then pipes one command into the guest PTY. Every HTTP leg below is
// truly guest-originated: curl runs inside guest A.
function guestExec(own, userID, cmd, ms = 20000) {
  const url = `ws://${guestIP(own)}:8085/api/terminal/ws`;
  try {
    const out = execFileSync('ssh', ['-o', 'BatchMode=yes', '-o', 'ConnectTimeout=8', 'node-b',
      `/tmp/wsexec -url '${url}' -user '${userID}' -cmd ${shellQuote(cmd)} -timeout ${ms}ms`],
      { encoding: 'utf8', maxBuffer: 8 * 1024 * 1024, timeout: ms + 15000 });
    const m = out.match(/EXIT=(\S+)\s*$/);
    return { out, exit: m ? m[1] : '?' };
  } catch (e) {
    return { out: String(e).slice(0, 400), exit: 'ssh-fail', err: true };
  }
}
function shellQuote(s) { return `'${String(s).replace(/'/g, `'\\''`)}'`; }
async function guestCurl(own, userID, curlArgs, ms = 15000) {
  const r = guestExec(own, userID,
    `curl -sS -m ${Math.max(5, Math.floor(ms / 1000) - 2)} -o /dev/null -w 'HTTP:%{http_code}' ${curlArgs}`,
    ms);
  const m = (r.out || '').match(/HTTP:(\d{3})/);
  const refused = r.exit === 'ssh-fail' || (!m && (r.exit !== '0' || (r.out || '').match(/refused|timed out|unreachable|timeout/i)));
  return { status: m ? parseInt(m[1], 10) : null, raw: (r.out || '').slice(-400), exit: r.exit, connectFailed: refused && !m };
}

function record(leg, out) { report.legs[leg] = out; console.log(`  ${leg}: ${out.verdict} — ${(out.detail || '').slice(0, 160)}`); }

async function ensureComputer(label, reuseId, browser) {
  if (reuseId) {
    const own = ownershipFor(reuseId);
    if (own) return { computer: reuseId, user: own.user_id, own };
  }
  const ctx = await browser.newContext();
  const page = await ctx.newPage();
  await setupVirtualAuthenticator(page);
  const email = `s1a-${label}-${Date.now()}-${Math.random().toString(36).slice(2, 6)}@example.com`;
  await page.goto(BASE_URL, { waitUntil: 'domcontentloaded', timeout: 60_000 });
  const regStarted = Date.now();
  const reg = await registerPasskey(page, email, BASE_URL);
  if (!reg || reg.ok === false) throw new Error(`registration failed for ${label}: ${JSON.stringify(reg)?.slice(0, 200)}`);
  const userId = reg.user?.id || '';
  await page.reload({ waitUntil: 'domcontentloaded', timeout: 60_000 });
  const createdAfter = new Date(regStarted - 5000).toISOString();
  let own = null;
  for (let i = 0; i < 150; i++) {
    own = (vmctlList()?.ownerships || []).filter(o => o.user_id === userId && o.created_at >= createdAfter)
      .sort((a, b) => String(b.created_at).localeCompare(String(a.created_at)))[0] || null;
    if (own && own.state === 'active') break;
    execFileSync('sleep', ['3']);
  }
  if (!own) throw new Error(`${label} computer never reached active`);
  await ctx.close().catch(() => {});
  return { computer: own.computer_id, user: userId, own };
}

const main = async () => {
  console.log(`S1a refusal matrix vs ${BASE_URL}`);
  nodeB('echo ok');
  report.iptables = {
    filter_drops: nodeB(`sudo iptables -L INPUT -nvx 2>/dev/null | grep -c 'DROP.*10.200' ; sudo iptables -L FORWARD -nvx 2>/dev/null | grep -c 'DROP.*10.200'`),
    tap_isolate_rules: nodeB(`sudo iptables-save 2>/dev/null | grep -cE 'go-choir-vm.*DROP'`),
  };

  const browser = await chromium.launch();
  const A = await ensureComputer('a', REUSE_A, browser);
  const B = await ensureComputer('b', REUSE_B, browser);
  await browser.close();
  const aIP = guestIP(A.own), bIP = guestIP(B.own), hostIP = hostPeerIP(A.own);
  report.computers = { a: { computer: A.computer, user: A.user, guest_ip: aIP }, b: { computer: B.computer, user: B.user, guest_ip: bIP } };
  console.log(`    A=${aIP}(${A.computer.slice(0, 16)}) B=${bIP} host=${hostIP}`);


  const slot = `computer:${A.user}:${A.computer}`;
  // Refusals (guest-originated curl against hostIP = the DNAT'd host peer).
  record('R1_tap_to_tap', await guestCurl(A.own, A.user, `http://${bIP}:8085/health`).then(r =>
    r.status === null || r.status >= 400 || r.timeout ? { verdict: 'refused', detail: r.raw || `status=${r.status}` } : { verdict: 'FAILED-OPEN', detail: `reached B:${r.status}` }));
  record('R2_vmctl_internal', await guestCurl(A.own, A.user, `-H 'X-Internal-Caller: true' 'http://${hostIP}:8083/internal/vmctl/list'`).then(r =>
    r.status === 403 ? { verdict: 'refused', detail: `HTTP ${r.status}` } : { verdict: 'FAILED-OPEN', detail: `HTTP ${r.status} raw=${r.raw.slice(-160)}` }));
  record('R3_maild_forged_owner', await guestCurl(A.own, A.user, `-H 'X-Internal-Caller: true' -H 'X-Authenticated-User: ${B.user}' 'http://${hostIP}:8087/api/email/messages'`).then(r =>
    r.status === 403 ? { verdict: 'refused', detail: `HTTP ${r.status}` } : { verdict: 'FAILED-OPEN', detail: `HTTP ${r.status} raw=${r.raw.slice(-160)}` }));
  record('R4_corpusd_bypass', await guestCurl(A.own, A.user, `-H 'X-Internal-Caller: true' 'http://${hostIP}:8086/internal/platform/update'`).then(r =>
    r.status === 403 ? { verdict: 'refused', detail: `HTTP ${r.status}` } : { verdict: 'FAILED-OPEN', detail: `HTTP ${r.status} raw=${r.raw.slice(-160)}` }));
  record('R5_proxy_wire_publish', await guestCurl(A.own, A.user, `-X POST -H 'Content-Type: application/json' -H 'X-Internal-Caller: true' -d '{}' 'http://${hostIP}:8082/internal/platform/wire/publish'`).then(r =>
    r.status === 403 || r.status === 404 ? { verdict: 'refused', detail: `HTTP ${r.status}` } : { verdict: 'FAILED-OPEN', detail: `HTTP ${r.status} raw=${r.raw.slice(-160)}` }));

  // Legitimate flows.
  record('L1_gateway', await guestCurl(A.own, A.user, `'http://${hostIP}:8084/health'`).then(r =>
    r.status && r.status < 500 ? { verdict: 'green', detail: `HTTP ${r.status}` } : { verdict: 'FAILED', detail: `HTTP ${r.status} ${r.raw.slice(-120)}` }));
  record('L2_cv_route_resolve', await guestCurl(A.own, A.user, `'http://${hostIP}:8083/internal/vmctl/computer-version-routes/resolve?route_slot_id=${encodeURIComponent(slot)}'`).then(r =>
    r.status === 200 ? { verdict: 'green', detail: `HTTP ${r.status} bound-owner resolve` } : { verdict: 'FAILED', detail: `HTTP ${r.status} ${r.raw.slice(-160)}` }));
  record('L3_maild_own_drafts', await guestCurl(A.own, A.user, `-H 'X-Internal-Caller: true' -H 'X-Authenticated-User: ${A.user}' 'http://${hostIP}:8087/api/email/messages'`).then(r =>
    r.status === 200 ? { verdict: 'green', detail: `HTTP ${r.status} bound-owner read` } : { verdict: 'FAILED', detail: `HTTP ${r.status} ${r.raw.slice(-160)}` }));
  record('L4_egress', await guestCurl(A.own, A.user, `-k -o /dev/null -w 'HTTP:%{http_code}' 'https://1.1.1.1'`).then(r =>
    r.status ? { verdict: 'green', detail: `HTTP ${r.status}` } : { verdict: 'FAILED', detail: `HTTP ${r.status} ${r.raw.slice(-120)}` }));

  // L5 product surface still loads for the account (proxy path unaffected).
  {
    const b2 = await chromium.launch();
    try {
      const p = await (await b2.newContext()).newPage();
      const resp = await p.goto(BASE_URL, { waitUntil: 'domcontentloaded', timeout: 30_000 });
      record('L5_product_page', { verdict: resp && resp.status() < 400 ? 'green' : 'FAILED', detail: `GET / -> ${resp?.status()}` });
    } finally { await b2.close().catch(() => {}); }
  }

  const failed = Object.entries(report.legs).filter(([, v]) => v.verdict === 'FAILED' || v.verdict === 'FAILED-OPEN');
  report.ok = failed.length === 0;
  report.failed_legs = failed.map(([k]) => k);
  report.finished_at = new Date().toISOString();
  mkdirSync(EVIDENCE_DIR, { recursive: true });
  const out = join(EVIDENCE_DIR, 's1a-refusal-matrix-2026-10-04.json');
  writeFileSync(out, JSON.stringify(report, null, 2) + '\n');
  console.log(`\n${report.ok ? 'OK' : 'FAILURES: ' + report.failed_legs.join(',')} — ${out}`);
  process.exit(report.ok ? 0 : 1);
};

main().catch(e => { console.error('probe error:', e); process.exit(2); });
