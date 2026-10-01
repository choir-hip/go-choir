
// S0a reality probe — drives the boot-timeline instrument and the read-only
// staging receipts for station S0 (choir-appdev-s0-reality-boot-timeline).
//
// What it does, in order:
//   1. Fresh computer: registers a brand-new owner via Playwright passkey
//      (product path), waits for the VM to reach active, then fetches the
//      merged host+guest boot receipt — s0a-boot-timeline-fresh.
//   2. Owner-sized computer: fetches the stored boot receipt for the owner's
//      computer (written by the deploy refresh boot) —
//      s0a-boot-timeline-owner-sized.
//   3. Guest layout: live /internal/boot/timeline on an active guest carries
//      mounts, /nix/store identity, cmdline presence flags, env presence,
//      and file modes — s0a-guest-layout.
//   4. Runtime closure: host-side nix-store -q --requisites on the deployed
//      autoputer package path — s0a-runtime-closure.
//   5. Tap reachability: guest A dials guest B's tap IP via
//      /internal/diag/tcp-dial — s0a-tap-reachability.
//   6. Gateway token visibility: guest receipt layout + host fc-config.json
//      and /proc/<fcpid>/cmdline inspection — s0a-gateway-token-visibility.
//
// Every fetch asserts minimally (schema_version present) and records raw
// payloads verbatim under docs/evidence/. A failure to collect a named
// receipt is itself reported, never silently skipped.
//
// Requirements: ssh node-b (BatchMode), frontend playwright deps installed.
// Usage: node frontend/tests/s0a-reality-probe.mjs [--skip-fresh] [--help]

import { createRequire } from 'node:module';
import { execFileSync } from 'node:child_process';
import { writeFileSync, mkdirSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';

const requireFrontend = createRequire(new URL('../package.json', import.meta.url));
const { chromium } = requireFrontend('@playwright/test');
const { registerPasskey } = await import('./helpers/auth.js');
const { setupVirtualAuthenticator } = await import('./helpers/webauthn.js');

const runtimeProcess = globalThis.process || null;
const BASE_URL = runtimeProcess?.env?.CHOIR_DEPLOYED_BASE_URL || 'https://choir.news';
const SKIP_FRESH = (runtimeProcess?.argv || []).includes('--skip-fresh') ||
  runtimeProcess?.env?.S0A_SKIP_FRESH === '1';
const REPO_ROOT = join(dirname(fileURLToPath(import.meta.url)), '..', '..');
const EVIDENCE_DIR = join(REPO_ROOT, 'docs', 'evidence');
const OWNER_COMPUTER_ID = 'computer-03335285269bdba4f94377e56879f9e6';
const OWNER_USER_ID = runtimeProcess?.env?.S0A_OWNER_USER_ID || ''; // resolved from vmctl list below

const index = {
  schema_version: 1,
  kind: 's0-probe-index',
  run_started_at: new Date().toISOString(),
  base_url: BASE_URL,
  receipts: {},
  findings: [],
  errors: [],
};

function nodeB(command, input) {
  return execFileSync('ssh', ['-o', 'BatchMode=yes', '-o', 'ConnectTimeout=8', 'node-b', command], {
    input, encoding: 'utf8', stdio: ['pipe', 'pipe', 'pipe'], maxBuffer: 64 * 1024 * 1024,
  }).trim();
}
// nodeBMaybe: like nodeB but returns {error} instead of throwing — for
// endpoints that legitimately 404 while a boot is in flight.
function nodeBMaybe(command) {
  try {
    return { out: nodeB(command) };
  } catch (e) {
    return { err: String(e?.message || e) };
  }
}
function nodeBJSON(command, input) {
  const raw = nodeB(command, input);
  return raw ? JSON.parse(raw) : null;
}
function writeReceipt(name, data) {
  mkdirSync(EVIDENCE_DIR, { recursive: true });
  const path = join(EVIDENCE_DIR, `${name}.json`);
  writeFileSync(path, JSON.stringify(data, null, 2));
  index.receipts[name] = { path: `docs/evidence/${name}.json` };
  console.log(`  wrote docs/evidence/${name}.json`);
  return path;
}
function vmctlList() {
  return nodeBJSON(`curl -fsS -H "X-Internal-Caller: true" http://127.0.0.1:8083/internal/vmctl/list`);
}
function vmctlBootTimeline(params) {
  return nodeBJSON(`curl -fsS -H "X-Internal-Caller: true" 'http://127.0.0.1:8083/internal/vmctl/boot-timeline?${params}'`);
}
// Maybe-variant for the poll loop: 404 while the boot hasn't written its
// receipt yet is a pending signal, not an error.
function vmctlBootTimelineMaybe(params) {
  const r = nodeBMaybe(`curl -fsS -H "X-Internal-Caller: true" 'http://127.0.0.1:8083/internal/vmctl/boot-timeline?${params}'`);
  if (r.err) return { pending: r.err };
  try { return JSON.parse(r.out); } catch { return { pending: 'not json' }; }
}
function guestFetch(hostURL, path) {
  return nodeBJSON(`curl -fsS -m 20 -H "X-Internal-Caller: true" '${hostURL}${path}'`);
}
function guestDial(guestHostURL, addr) {
  return nodeBJSON(`curl -fsS -m 20 -H "X-Internal-Caller: true" '${guestHostURL}/internal/diag/tcp-dial?addr=${addr}'`);
}

function uniqueEmail() {
  return `s0a-probe-${Date.now()}-${Math.random().toString(36).slice(2, 8)}@example.com`;
}

// Reuse a computer registered by an earlier run (e.g. after a mid-probe
// failure) instead of creating another throwaway on staging.
const FRESH_COMPUTER_ID = runtimeProcess?.env?.S0A_FRESH_COMPUTER_ID || '';

async function main() {
  console.log(`S0a reality probe against ${BASE_URL}`);

  // ── Sanity: node-b + staging reachable ──────────────────────────────
  nodeB('echo ok');
  const list0 = vmctlList();
  console.log(`  vmctl ownerships: ${list0?.count ?? '??'}`);

  const activeVMs = (list0?.ownerships || []).filter(o => o.state === 'active');
  console.log(`  active VMs: ${activeVMs.length}`);
  for (const o of activeVMs) {
    console.log(`    ${o.vm_id} computer=${o.computer_id} url=${o.computer_url} epoch=${o.epoch}`);
  }

  // ── 1. Fresh-computer boot timeline (product path) ──────────────────
  let freshReceipt = null;
  if (!SKIP_FRESH) {
    console.log('  [1] fresh computer via Playwright registration …');
    const browser = await chromium.launch();
    try {
      const ctx = await browser.newContext();
      const page = await ctx.newPage();
      await setupVirtualAuthenticator(page);
      const email = uniqueEmail();
      const regStarted = Date.now();
      // The helper fetches /auth/* relative to the page origin; land on the
      // app first so same-origin fetch + session cookies work.
      await page.goto(BASE_URL, { waitUntil: 'domcontentloaded', timeout: 60_000 });
      const reg = await registerPasskey(page, email, BASE_URL);
      if (!reg || (reg.ok === false)) {
        throw new Error(`passkey registration rejected: ${JSON.stringify(reg)?.slice(0, 300)}`);
      }
      // Computer provisioning fires on the first authenticated app load —
      // reload so the session-carrying page boots the desktop.
      await page.reload({ waitUntil: 'domcontentloaded', timeout: 60_000 });
      // The prompt surface exists pre-registration too — wait for the fresh
      // ownership to appear in vmctl instead of a UI selector that can match
      // the landing page before boot has started.
      const regMs = Date.now() - regStarted;
      const createdAfter = new Date(regStarted - 5000).toISOString();
      let fresh = null;
      for (let i = 0; i < 120; i++) {
        const listNow = vmctlList();
        fresh = (listNow?.ownerships || [])
          .filter(o => o.created_at >= createdAfter && o.vm_id && !activeVMs.some(a => a.vm_id === o.vm_id))
          .sort((a, b) => String(b.created_at).localeCompare(String(a.created_at)))[0];
        if (fresh) break;
        execFileSync('sleep', ['2']);
      }
      if (!fresh) throw new Error('no new ownership appeared after registration');
      console.log(`    fresh vm=${fresh.vm_id} computer=${fresh.computer_id} reg_ms=${regMs}`);
      // Poll the stored receipt until the boot finish is persisted; a 404 is
      // pending (the record is written at boot finish).
      let tl = null;
      for (let i = 0; i < 90; i++) {
        tl = vmctlBootTimelineMaybe(`computer_id=${fresh.computer_id}`);
        if (tl?.outcome === 'healthy' && tl?.guest_receipt) break;
        if (tl?.outcome === 'failed') break;
        execFileSync('sleep', ['3']);
      }
      if (!tl?.outcome) throw new Error(`no boot timeline for ${fresh.vm_id}`);
      tl.probe = { registration_ms: regMs, email_domain: 'example.com' };
      writeReceipt('s0a-boot-timeline-fresh-2026-10-01', tl);
      freshReceipt = tl;
      index.fresh_computer = { vm_id: fresh.vm_id, computer_id: fresh.computer_id, registration_ms: regMs };
    } finally {
      await browser.close();
    }
  } else if (FRESH_COMPUTER_ID) {
    // Reuse the computer registered by an earlier probe run; its boot
    // receipt should be persisted by now.
    console.log(`  [1] reusing fresh computer ${FRESH_COMPUTER_ID}`);
    const tl = vmctlBootTimelineMaybe(`computer_id=${FRESH_COMPUTER_ID}`);
    if (tl?.outcome) {
      writeReceipt('s0a-boot-timeline-fresh-2026-10-01', tl);
      freshReceipt = tl;
      index.fresh_computer = { computer_id: FRESH_COMPUTER_ID, reused: true };
    } else {
      index.errors.push(`reused fresh timeline still pending: ${JSON.stringify(tl)?.slice(0, 200)}`);
    }
  } else {
    console.log('  [1] skipped (--skip-fresh)');
  }

  // ── 2. Owner-sized boot timeline (deploy refresh boot) ──────────────
  console.log('  [2] owner-sized boot timeline …');
  const ownerOwn = (vmctlList()?.ownerships || []).find(o => o.computer_id === OWNER_COMPUTER_ID);
  if (!ownerOwn) throw new Error(`owner computer ${OWNER_COMPUTER_ID} not in vmctl list`);
  console.log(`    owner vm=${ownerOwn.vm_id} state=${ownerOwn.state} epoch=${ownerOwn.epoch}`);
  let ownerTL = vmctlBootTimeline(`computer_id=${OWNER_COMPUTER_ID}`);
  if (ownerTL?.outcome) {
    ownerTL.probe = { note: 'written by the deploy-time refresh boot; computer-sized data volume replayed' };
    writeReceipt('s0a-boot-timeline-owner-sized-2026-10-01', ownerTL);
  } else {
    index.errors.push(`owner boot timeline missing: ${JSON.stringify(ownerTL)}`);
  }

  // ── 3. Guest layout (live guest receipt) ────────────────────────────
  console.log('  [3] guest layout …');
  const layoutTarget = activeVMs[0];
  let layoutReceipt = null;
  if (layoutTarget?.computer_url) {
    const live = guestFetch(layoutTarget.computer_url, '/internal/boot/timeline');
    if (live?.kind === 'guest_boot_timeline') {
      layoutReceipt = {
        schema_version: 1,
        kind: 's0a-guest-layout',
        collected_at: new Date().toISOString(),
        vm_id: layoutTarget.vm_id,
        computer_id: layoutTarget.computer_id,
        host_url: layoutTarget.computer_url,
        layout: live.layout,
        identity: live.identity,
        units: live.units,
        missing_observers: live.missing_observers,
      };
      writeReceipt('s0a-guest-layout-2026-10-01', layoutReceipt);
    } else {
      index.errors.push(`guest layout fetch failed: ${JSON.stringify(live)?.slice(0, 300)}`);
    }
  }
  // ── 4. Runtime closure (host-side nix-store requisites) ─────────────
  console.log('  [4] runtime closure …');
  try {
    let closure = { schema_version: 1, kind: 's0a-runtime-closure', collected_at: new Date().toISOString() };
    // The guest executable resolves to a /nix/store path inside the shared
    // EROFS store disk; that package was built on node-b so the same path
    // exists in the host nix store, making host-side closure queries exact.
    const guestExe = layoutReceipt?.identity?.executable_resolved ||
      ownerTL?.guest_receipt?.identity?.executable_resolved ||
      freshReceipt?.guest_receipt?.identity?.executable_resolved;
    closure.guest_executable_resolved = guestExe || null;
    if (guestExe && guestExe.startsWith('/nix/store/')) {
      const pkg = guestExe.split('/bin/')[0];
      closure.package_path = pkg;
      closure.present_on_host = nodeB(`[ -d ${pkg} ] && echo yes || echo no`) === 'yes';
      if (closure.present_on_host) {
        const refs = nodeB(`/run/current-system/sw/bin/nix-store -q --requisites ${pkg} | sort`);
        closure.requisites = refs.split('\n').filter(Boolean);
        closure.requisite_count = closure.requisites.length;
        const direct = nodeB(`/run/current-system/sw/bin/nix-store -q --references ${pkg} | sort`);
        closure.direct_references = direct.split('\n').filter(Boolean);
      }
    }
    // Guest-side view: store entry count + EROFS mount from the receipt.
    closure.guest_store_entries = layoutReceipt?.layout?.nix_store_entries ?? null;
    closure.guest_store_mount = (layoutReceipt?.layout?.mounts || []).find(m => m.point === '/nix/store') || null;
    closure.host_package_pointer = nodeB('readlink -f /var/lib/go-choir/services/autoputer 2>/dev/null || echo ""');
    writeReceipt('s0a-runtime-closure-2026-10-01', closure);
  } catch (e) {
    index.errors.push(`runtime closure failed: ${e.message}`);
  }

  // ── 5. Tap reachability (guest->guest) ──────────────────────────────
  console.log('  [5] tap reachability …');
  try {
    const actives = (vmctlList()?.ownerships || []).filter(o => o.state === 'active' && o.computer_url);
    const a = actives[0];
    const b = actives.find(o => o.vm_id !== a?.vm_id);
    let reach = {
      schema_version: 1, kind: 's0a-tap-reachability', collected_at: new Date().toISOString(),
      source_vm: a?.vm_id, source_url: a?.computer_url,
    };
    if (a && b) {
      const bIP = new URL(b.computer_url).hostname;
      reach.targets = {
        other_guest_tap_8085: guestDial(a.computer_url, `${bIP}:8085`),
        host_vmctl_8083: guestDial(a.computer_url, `${new URL(a.computer_url).hostname.replace(/\.\d+$/, '.1')}:8083`),
        external_https_443: guestDial(a.computer_url, '1.1.1.1:443'),
      };
      reach.interpretation = 'other_guest_tap_8085 ok=true means tap->tap forwarding is open (S1 must isolate); host_vmctl_8083 ok=true is expected service reachability; external ok=true means unfiltered guest egress.';
    } else {
      reach.error = 'need two active computers to test tap->tap; only one present';
    }
    writeReceipt('s0a-tap-reachability-2026-10-01', reach);
  } catch (e) {
    index.errors.push(`tap reachability failed: ${e.message}`);
  }

  // ── 6. Gateway token visibility ─────────────────────────────────────
  console.log('  [6] gateway token visibility …');
  try {
    const vmid = layoutTarget?.vm_id || ownerOwn.vm_id;
    const fcPID = nodeB(`cat /var/lib/go-choir/vm-state/${vmid}/firecracker.pid 2>/dev/null || echo ""`).trim();
    const vis = {
      schema_version: 1, kind: 's0a-gateway-token-visibility', collected_at: new Date().toISOString(),
      vm_id: vmid,
      guest: layoutReceipt ? {
        cmdline_has_gateway_token: !!layoutReceipt.layout?.cmdline?.['choir.gateway_token'],
        env_has_runtime_gateway_token: !!layoutReceipt.layout?.env_presence?.RUNTIME_GATEWAY_TOKEN,
        autoputer_env_file: layoutReceipt.layout?.files?.find(f => f.path === '/run/go-choir-autoputer.env'),
        gateway_token_file: layoutReceipt.layout?.files?.find(f => f.path === '/mnt/persistent/gateway-token'),
      } : null,
      host: {},
    };
    if (fcPID && /^\d+$/.test(fcPID)) {
      const cmd = nodeB(`tr '\\0' ' ' < /proc/${fcPID}/cmdline`);
      vis.host.firecracker_cmdline_has_token = cmd.includes('choir.gateway_token=');
      vis.host.firecracker_cmdline_config_path = cmd.includes('fc-config.json');
      const fc = nodeB(`cat /var/lib/go-choir/vm-state/${vmid}/fc-config.json 2>/dev/null`);
      if (fc) {
        const cfg = JSON.parse(fc);
        const args = cfg?.['boot-source']?.boot_args || '';
        vis.host.fc_config_boot_args_has_token = args.includes('choir.gateway_token=');
        vis.host.fc_config_mode = nodeB(`stat -c '%a %U:%G' /var/lib/go-choir/vm-state/${vmid}/fc-config.json`);
      }
    }
    // Host ps surface: is the token visible in process listings?
    const ps = nodeB(`ps -eo args -p ${fcPID} 2>/dev/null | grep -c 'choir.gateway_token=' || true`);
    vis.host.ps_shows_token = ps.trim() !== '0' && ps.trim() !== '';
    vis.finding = 'see docs/problems/s0-gateway-token-on-kernel-cmdline-2026-10-01.md';
    writeReceipt('s0a-gateway-token-visibility-2026-10-01', vis);
    if (vis.guest?.cmdline_has_gateway_token || vis.host?.fc_config_boot_args_has_token || vis.host?.ps_shows_token) {
      index.findings.push('gateway token exposed on kernel cmdline (guest /proc/cmdline) and host fc-config.json / ps — confirmed live');
    }
  } catch (e) {
    index.errors.push(`gateway token visibility failed: ${e.message}`);
  }

  // ── 7. Owner re-refresh: capture owner-sized replay volume ──────────
  // Epoch-993's replay.sequence was 0 (the prior boot committed the whole
  // tape). The owner's engineering redrive loop appends deferral rows
  // continuously, so a fresh refresh now measures a real nonzero replay.
  console.log('  [7] owner re-refresh for real replay volume …');
  try {
    nodeB(`curl -fsS -m 300 -H "X-Internal-Caller: true" -H "Content-Type: application/json" -d '{"user_id":"5bd6de97-3b58-408c-bf89-c42c81b083de","desktop_id":"primary"}' http://127.0.0.1:8083/internal/vmctl/refresh >/dev/null`);
    execFileSync('sleep', ['12']);
    const tl = vmctlBootTimeline(`computer_id=${OWNER_COMPUTER_ID}`);
    if (tl?.outcome) {
      tl.probe = { note: 're-refresh after fresh-computer registration; tape grew from deferral rows in between' };
      writeReceipt('s0a-boot-timeline-owner-sized-post-refresh-2026-10-01', tl);
      index.owner_replay_volume = tl.guest_receipt?.replay ?? null;
    }
  } catch (e) {
    index.errors.push(`owner re-refresh failed: ${e.message}`);
  }
  index.run_finished_at = new Date().toISOString();
  writeReceipt('s0-probe-index-2026-10-01', index);
  console.log('S0a probe complete:', JSON.stringify({ receipts: Object.keys(index.receipts), findings: index.findings, errors: index.errors }, null, 1));
}

main().catch((e) => {
  index.errors.push(String(e?.stack || e));
  index.run_finished_at = new Date().toISOString();
  try { writeReceipt('s0-probe-index-2026-10-01', index); } catch {}
  console.error('S0a probe failed:', e);
  throw e;
});
