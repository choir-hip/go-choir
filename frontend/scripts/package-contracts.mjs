#!/usr/bin/env node
// The package contract runner (docs/design/modularity-check-build-2026-11-04.md).
//
// Each package under src/packages/<name>/ carries a smoke contract,
// choir-contract.json beside its manifest:
//
//   {
//     "schema": "choir.package-contract/v1",
//     "ready": "<selector>",            required: the app is up when this shows
//     "expect": [{ "selector": "<sel>", "text": "<substring>", "count": <n> }],
//     "api": [{ "method": "GET", "path": "/api/...", "status": 200, "json": {} }],
//     "steps": [<step>, ...],           optional: drive the app after `expect` (below)
//     "expect_requests": [{ "request": "PATCH /api/files/a", "body": "<substring>" }],
//                                       stubbed `api` requests the steps must have caused
//     "allow_console": ["<regex>"],     app console errors accepted, by name
//     "allow_unanswered": [{ "request": "GET /api/x/*", "reason": "<why>" }],
//                                       requests the app may make unstubbed
//     "note": "<what the contract shows>"
//   }
//
// A step is one of (selectors are Playwright locators inside the window):
//   { "click": sel }   { "fill": sel, "value": str }   { "press": sel, "key": "F2" }
//   { "upload": sel, "name": "a.txt", "mime": "text/plain", "content": "hi" }
//   { "see": sel, "text": "<substring>", "count": n }
// A failed step is a problem naming the step, never a skip.
//
// Every selector is resolved inside the app window's content
// ([data-window][data-window-app-id=<name>] [data-window-content]), never the
// shell chrome. The runner opens the package the way a person does (the app
// switcher, the launcher entry, the window) in a shell whose backend is a stub
// that answers the boot routes (/auth/session, /api/shell/bootstrap,
// /api/desktop/state, /api/preferences/theme, the desktop background setting
// (/api/preferences/pkg.settings.background*), the selection records
// (/api/preferences/packages.selection), /api/packages/installed and the
// update sync), the
// contract's `api` entries, and 404 for the rest. Any uncaught exception, a
// console error that is not the browser's own "Failed to load resource" line,
// or a request nothing answered fails the run: before the open click it is the
// stub's drift from the shell's boot, after it the package's, unless the
// contract lists it in allow_unanswered with a reason.
//
// Three modes (the first two run by default):
//   in-tree    the package as the base ships it: the shell build builds it into
//              a module/v1 module beside the shell and mounts it through the
//              loader (forced modularity M4; scripts/base-modules.mjs);
//   installed  the package built to one module/v1 module by the computer's own
//              build (internal/pkgruntime/build-module.mjs) and listed as an
//              installed package on the owner's own track, so the shell's real
//              loader checks its digest, requires_sdk and SDK names and mounts it;
//   sandboxed  the same module listed on a stranger's track (isolation
//              "frame"), so the shell runs it in the opaque-origin sandboxed
//              frame (MA6.1) and every SDK call crosses the bridge. Selectors
//              resolve inside the frame. A package that uses SDK names the
//              frame does not offer is refused there, by design: this mode is
//              for packages written for the frame (tests/fixtures/sandboxed-packages).
//
// The Playwright spec (tests/package-modularity.spec.js) is the CI gate and
// uses the exports. The CLI runs the same contracts against any built shell:
//
//   node scripts/package-contracts.mjs [--dist DIR] [--packages-dir DIR]
//        [--only a,b] [--modes in-tree,installed,sandboxed] [--receipt FILE]
//
// --dist is a built shell (default: build this tree into a temp directory).
// --packages-dir holds package trees with their contracts (default
// src/packages). Trees from elsewhere (a computer's installed releases) run in
// the installed or sandboxed modes only. Chromium: $CHOIR_BROWSER if set, else Playwright's.
// Exit 0 when every row passed, 1 otherwise; the receipt is written either way.
import { spawnSync } from 'node:child_process';
import { createHash } from 'node:crypto';
import fs from 'node:fs';
import http from 'node:http';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

export const CONTRACT_SCHEMA = 'choir.package-contract/v1';
export const CONTRACT_FILE = 'choir-contract.json';
const STEP_KINDS = ['click', 'fill', 'press', 'upload', 'see'];
export const RECEIPT_SCHEMA = 'choir.modularity-receipt/v1';
export const MODULE_ABI = 'choir.app-frontend/module/v1';
/** The stub computer's id; a contract's `api` paths that name a computer use it. */
export const STUB_COMPUTER_ID = 'contract-computer';
export const STUB_USER = 'contract-owner';
export const MODES = ['in-tree', 'installed', 'sandboxed'];
/** The modes a run takes when none are named: the sandboxed mode is for packages written for the frame. */
export const DEFAULT_MODES = ['in-tree', 'installed'];
/** The publisher of a sandboxed-mode release: any track but the stub owner's. */
export const STUB_STRANGER = 'contract-stranger';

const FRONTEND_ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const REPO_ROOT = path.resolve(FRONTEND_ROOT, '..');
const RUNTIME_DIR = path.join(REPO_ROOT, 'internal/pkgruntime');
const STEP_MS = 10_000;
const QUIET_MS = 750;
const NETWORK_NOISE = /^Failed to load resource\b/;

const sha256 = (bytes) => createHash('sha256').update(bytes).digest('hex');
const isObject = (v) => v !== null && typeof v === 'object' && !Array.isArray(v);

// ---- contracts ----

/** validateContract returns the problems with one contract (C2); [] when valid. */
export function validateContract(c) {
  const problems = [];
  if (!isObject(c)) return ['the contract must be a JSON object'];
  const allowed = new Set(['schema', 'ready', 'expect', 'api', 'steps', 'expect_requests', 'allow_console', 'allow_unanswered', 'note']);
  for (const k of Object.keys(c)) if (!allowed.has(k)) problems.push(`unknown field "${k}"`);
  if (c.schema !== CONTRACT_SCHEMA) problems.push(`schema must be "${CONTRACT_SCHEMA}"`);
  if (typeof c.ready !== 'string' || !c.ready.trim()) problems.push('ready must be a non-empty selector');
  if (c.expect !== undefined) {
    if (!Array.isArray(c.expect)) problems.push('expect must be an array');
    else c.expect.forEach((e, i) => {
      if (!isObject(e)) return problems.push(`expect[${i}] must be an object`);
      for (const k of Object.keys(e)) if (!['selector', 'text', 'count'].includes(k)) problems.push(`expect[${i}]: unknown field "${k}"`);
      if (typeof e.selector !== 'string' || !e.selector.trim()) problems.push(`expect[${i}].selector must be a non-empty selector`);
      if (e.text !== undefined && (typeof e.text !== 'string' || !e.text)) problems.push(`expect[${i}].text must be a non-empty string`);
      if (e.count !== undefined && !(Number.isInteger(e.count) && e.count >= 0)) problems.push(`expect[${i}].count must be a whole number`);
    });
  }
  if (c.api !== undefined) {
    if (!Array.isArray(c.api)) problems.push('api must be an array');
    else c.api.forEach((a, i) => {
      if (!isObject(a)) return problems.push(`api[${i}] must be an object`);
      for (const k of Object.keys(a)) if (!['method', 'path', 'status', 'json'].includes(k)) problems.push(`api[${i}]: unknown field "${k}"`);
      if (!['GET', 'POST', 'PUT', 'PATCH', 'DELETE'].includes(a.method)) problems.push(`api[${i}].method must be GET, POST, PUT, PATCH or DELETE`);
      if (typeof a.path !== 'string' || !a.path.startsWith('/api/')) problems.push(`api[${i}].path must start with /api/`);
      if (a.status !== undefined && !(Number.isInteger(a.status) && a.status >= 100 && a.status < 600)) problems.push(`api[${i}].status must be an HTTP status`);
    });
  }
  if (c.steps !== undefined) {
    if (!Array.isArray(c.steps)) problems.push('steps must be an array');
    else c.steps.forEach((st, i) => {
      if (!isObject(st)) return problems.push(`steps[${i}] must be an object`);
      const kind = STEP_KINDS.filter((k) => k in st);
      if (kind.length !== 1) return problems.push(`steps[${i}] must have exactly one of ${STEP_KINDS.join(', ')}`);
      const k = kind[0];
      const extra = { click: [], fill: ['value'], press: ['key'], upload: ['name', 'mime', 'content'], see: ['text', 'count'] }[k];
      for (const f of Object.keys(st)) if (f !== k && !extra.includes(f)) problems.push(`steps[${i}]: unknown field "${f}"`);
      if (typeof st[k] !== 'string' || !st[k].trim()) problems.push(`steps[${i}].${k} must be a non-empty selector`);
      if (k === 'fill' && typeof st.value !== 'string') problems.push(`steps[${i}].value must be a string`);
      if (k === 'press' && (typeof st.key !== 'string' || !st.key)) problems.push(`steps[${i}].key must be a key name`);
      if (k === 'upload' && (typeof st.name !== 'string' || !st.name || typeof st.content !== 'string')) problems.push(`steps[${i}] upload needs name and content strings`);
      if (k === 'see' && st.count !== undefined && !(Number.isInteger(st.count) && st.count >= 0)) problems.push(`steps[${i}].count must be a whole number`);
    });
  }
  if (c.expect_requests !== undefined) {
    if (!Array.isArray(c.expect_requests)) problems.push('expect_requests must be an array');
    else c.expect_requests.forEach((r, i) => {
      if (!isObject(r)) return problems.push(`expect_requests[${i}] must be an object`);
      for (const k of Object.keys(r)) if (!['request', 'body'].includes(k)) problems.push(`expect_requests[${i}]: unknown field "${k}"`);
      if (typeof r.request !== 'string' || !/^(GET|POST|PUT|PATCH|DELETE) \/api\/\S*$/.test(r.request)) problems.push(`expect_requests[${i}].request must be "METHOD /api/path"`);
      if (r.body !== undefined && typeof r.body !== 'string') problems.push(`expect_requests[${i}].body must be a string`);
    });
  }
  if (c.allow_console !== undefined) {
    if (!Array.isArray(c.allow_console)) problems.push('allow_console must be an array');
    else c.allow_console.forEach((r, i) => {
      try {
        if (typeof r !== 'string' || !r) throw new Error('empty');
        new RegExp(r); // eslint-disable-line no-new
      } catch {
        problems.push(`allow_console[${i}] must be a regular expression`);
      }
    });
  }
  if (c.allow_unanswered !== undefined) {
    if (!Array.isArray(c.allow_unanswered)) problems.push('allow_unanswered must be an array');
    else c.allow_unanswered.forEach((a, i) => {
      if (!isObject(a)) return problems.push(`allow_unanswered[${i}] must be an object`);
      for (const k of Object.keys(a)) if (!['request', 'reason'].includes(k)) problems.push(`allow_unanswered[${i}]: unknown field "${k}"`);
      if (typeof a.request !== 'string' || !/^(GET|POST|PUT|PATCH|DELETE) \/api\/\S*$/.test(a.request)) problems.push(`allow_unanswered[${i}].request must be "METHOD /api/path", with * for any run of characters`);
      else if (!allowEntryNarrow(a.request)) problems.push(`allow_unanswered[${i}].request ${JSON.stringify(a.request)} is too broad: name at least two path segments after /api/ before any *, and use * only as a whole last segment`);
      if (typeof a.reason !== 'string' || a.reason.trim().length < 10) problems.push(`allow_unanswered[${i}].reason must say why (10 characters or more)`);
    });
  }
  if (c.note !== undefined && typeof c.note !== 'string') problems.push('note must be a string');
  return problems;
}

/**
 * allowEntryNarrow: an allow_unanswered request may not be a blanket excuse.
 * At least two literal segments after /api/, and * only as a whole last
 * segment ("GET /api/catalog/releases/*", never "GET /api/*" or "GET /api/catalog*").
 */
export function allowEntryNarrow(request) {
  const segments = request.slice(request.indexOf(' ') + 1).split('/').slice(2); // after "", "api"
  const star = segments.findIndex((seg) => seg.includes('*'));
  if (star >= 0 && (star !== segments.length - 1 || segments[star] !== '*')) return false;
  const literal = (star >= 0 ? segments.slice(0, star) : segments).filter(Boolean);
  return literal.length >= 2;
}

/** treeDigest: sha256 over every regular file's relative path and bytes, sorted. */
export function treeDigest(dir) {
  const h = createHash('sha256');
  const files = fs.readdirSync(dir, { recursive: true }).map(String).filter((rel) => fs.lstatSync(path.join(dir, rel)).isFile()).sort();
  for (const rel of files) {
    const bytes = fs.readFileSync(path.join(dir, rel));
    h.update(`${rel.split(path.sep).join('/')}\0${bytes.length}\0`);
    h.update(bytes);
  }
  return `sha256:${h.digest('hex')}`;
}

/**
 * loadPackages reads every package directory: its manifest, its contract and
 * the problems with either (C1: a package with no contract is a problem).
 */
export function loadPackages(packagesDir = path.join(FRONTEND_ROOT, 'src/packages')) {
  const out = [];
  for (const name of fs.readdirSync(packagesDir).sort()) {
    const dir = path.join(packagesDir, name);
    if (!fs.lstatSync(dir).isDirectory()) continue;
    const row = { name, dir, manifest: null, contract: null, problems: [], tree_digest: treeDigest(dir), contract_digest: null };
    try {
      row.manifest = JSON.parse(fs.readFileSync(path.join(dir, 'choir-package.json'), 'utf8'));
    } catch (e) {
      row.problems.push(`choir-package.json: ${e.message}`);
    }
    // A shell-surface package (provides, no app) draws no window, so it has no
    // window contract; its SDK names are checked by surfacePackages() below.
    // A pack (contributes, no app, no provides) draws nothing either.
    if (row.manifest && row.manifest.app === undefined && (Array.isArray(row.manifest.provides) || (row.manifest.contributes && typeof row.manifest.contributes === 'object'))) continue;
    const contractFile = path.join(dir, CONTRACT_FILE);
    if (!fs.existsSync(contractFile)) {
      row.problems.push(`no ${CONTRACT_FILE}: every package carries a smoke contract (docs/design/modularity-check-build-2026-11-04.md)`);
    } else {
      const raw = fs.readFileSync(contractFile);
      row.contract_digest = `sha256:${sha256(raw)}`;
      try {
        row.contract = JSON.parse(raw.toString('utf8'));
        for (const p of validateContract(row.contract)) row.problems.push(`${CONTRACT_FILE}: ${p}`);
      } catch (e) {
        row.problems.push(`${CONTRACT_FILE}: ${e.message}`);
      }
    }
    out.push(row);
  }
  return out;
}

// ---- builds ----

/**
 * buildPackageModule lays out a private frontend root as internal/pkgruntime
 * does (the package tree, the locked package.json and node_modules, the two
 * embedded scripts) and runs the computer's own module build. It returns
 * { ok, log, bytes, sha256, sdk_imports, requires_sdk }.
 */
export async function buildPackageModule({ name, dir, frontendRoot = FRONTEND_ROOT, runtimeDir = RUNTIME_DIR }) {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'choir-contract-module-'));
  try {
    const front = path.join(tmp, 'frontend');
    fs.mkdirSync(path.join(front, 'src/packages'), { recursive: true });
    fs.cpSync(dir, path.join(front, 'src/packages', name), { recursive: true, dereference: false });
    for (const script of ['build-module.mjs', 'package-boundary-plugin.mjs', 'css-fetch-functions.json']) fs.copyFileSync(path.join(runtimeDir, script), path.join(front, script));
    fs.copyFileSync(path.join(frontendRoot, 'package.json'), path.join(front, 'package.json'));
    fs.symlinkSync(path.join(frontendRoot, 'node_modules'), path.join(front, 'node_modules'));
    const manifest = JSON.parse(fs.readFileSync(path.join(dir, 'choir-package.json'), 'utf8'));
    const requiresSdk = manifest?.app?.requires_sdk || '^2.0';
    const out = path.join(tmp, 'module.js');
    const r = spawnSync(process.execPath, [path.join(front, 'build-module.mjs'), front, name, out, requiresSdk], { encoding: 'utf8', cwd: front });
    const log = `${r.stdout || ''}${r.stderr || ''}`.trim();
    if (r.status !== 0) return { ok: false, log, requires_sdk: requiresSdk };
    const bytes = fs.readFileSync(out);
    return { ok: true, log, bytes, sha256: sha256(bytes), requires_sdk: requiresSdk, sdk_imports: await moduleSdkImports(bytes) };
  } finally {
    fs.rmSync(tmp, { recursive: true, force: true });
  }
}

/** moduleSdkImports reads a module/v1 module's sdk_imports export by importing it (its top level touches no DOM). */
export async function moduleSdkImports(bytes) {
  // A bundled dependency may log at evaluation (pdf.js asks for its legacy
  // build under Node); that is not the package's output, so it is muted here.
  const saved = [console.log, console.warn, console.info];
  console.log = console.warn = console.info = () => {};
  let mod;
  try {
    mod = await import(`data:text/javascript;base64,${Buffer.from(bytes).toString('base64')}`);
  } finally {
    [console.log, console.warn, console.info] = saved;
  }
  if (mod.abi !== MODULE_ABI) throw new Error(`the module's abi is ${mod.abi}, not ${MODULE_ABI}`);
  return Array.isArray(mod.sdk_imports) ? [...mod.sdk_imports] : [];
}

/** buildShell builds this tree's shell (the build gate first, then vite build) into outDir (C7). */
export async function buildShell({ frontendRoot = FRONTEND_ROOT, outDir }) {
  const check = spawnSync(process.execPath, [path.join(frontendRoot, 'scripts/check-packages.mjs')], { encoding: 'utf8', cwd: frontendRoot });
  if (check.status !== 0) throw new Error(`check-packages refused the tree:\n${check.stdout}${check.stderr}`);
  const { build } = await import(pathToFileURL(path.join(frontendRoot, 'node_modules/vite/dist/node/index.js')).href);
  await build({ root: frontendRoot, configFile: path.join(frontendRoot, 'vite.config.js'), logLevel: 'error', build: { outDir, emptyOutDir: true } });
  return outDir;
}

/** distDigest: what shell was checked (C12). */
export function distDigest(dist) {
  return treeDigest(dist);
}

// ---- the served shell and its stub backend ----

const TYPES = { '.html': 'text/html', '.js': 'text/javascript', '.mjs': 'text/javascript', '.css': 'text/css', '.json': 'application/json', '.svg': 'image/svg+xml', '.png': 'image/png', '.woff2': 'font/woff2', '.wasm': 'application/wasm', '.ico': 'image/x-icon', '.webmanifest': 'application/manifest+json' };

/** serveDist serves a built shell on 127.0.0.1 (a secure context, so the loader's WebCrypto digest runs). */
export function serveDist(dist) {
  const root = path.resolve(dist);
  const server = http.createServer((req, res) => {
    const u = decodeURIComponent(new URL(req.url, 'http://x').pathname);
    let f = path.join(root, path.normalize(u));
    if (!f.startsWith(root)) return res.writeHead(403).end();
    if (!fs.existsSync(f) || fs.statSync(f).isDirectory()) {
      if (path.extname(u) || u.startsWith('/api/') || u.startsWith('/auth/')) return res.writeHead(404).end();
      f = path.join(root, 'index.html');
    }
    res.writeHead(200, { 'content-type': TYPES[path.extname(f)] || 'application/octet-stream' });
    fs.createReadStream(f).pipe(res);
  });
  return new Promise((resolve) => server.listen(0, '127.0.0.1', () => resolve({ base: `http://127.0.0.1:${server.address().port}`, close: () => new Promise((r) => server.close(r)) })));
}

/**
 * installedRecord is the GET /api/packages/installed row for a built module:
 * on the owner's own track (isolation "shell"), or a stranger's (isolation
 * "frame") with `stranger`.
 */
export function installedRecord(manifest, built, { stranger = false } = {}) {
  return {
    name: manifest.name,
    release_id: `contract-${built.sha256.slice(0, 16)}`,
    install_id: `contract-install-${manifest.name}`,
    installed_at: '2026-11-04T00:00:00Z',
    track: stranger ? STUB_STRANGER : STUB_USER,
    isolation: stranger ? 'frame' : 'shell',
    uses: [...(built.sdk_imports || [])],
    requires_sdk: built.requires_sdk,
    manifest,
    module: { abi: MODULE_ABI, sha256: built.sha256, url: `/api/packages/modules/${built.sha256}.js` },
    available: true,
  };
}

/**
 * stubBackend routes /auth and /api for one page (C13): the boot routes, the
 * installed list and its modules, the contract's api entries, 404 otherwise.
 * Each request nothing answered is passed to onUnanswered("METHOD /path").
 */
export async function stubBackend(page, { installed = [], modules = new Map(), api = [], seedWindow = null, onUnanswered = () => {}, onApi = () => {} } = {}) {
  await page.routeWebSocket(/\/api\/ws\b/, () => {}); // the live channel opens and stays quiet
  await page.route(/\/(auth|api)\//, (route) => {
    const req = route.request();
    const url = new URL(req.url());
    const p = url.pathname;
    const own = api.find((a) => a.method === req.method() && a.path === p);
    if (own) {
      onApi({ request: `${req.method()} ${p}`, body: req.postData() || '' });
      return route.fulfill({ status: own.status || 200, contentType: 'application/json', body: JSON.stringify(own.json ?? {}) });
    }
    if (p === '/auth/session') return route.fulfill({ json: { authenticated: true, user: { id: STUB_USER, username: STUB_USER } } });
    if (p === '/auth/accounts') return route.fulfill({ json: [] });
    if (p === `/api/computers/${STUB_COMPUTER_ID}/update` && req.method() === 'GET') return route.fulfill({ json: { update_available: null } }); // no update pending: the chip stays hidden
    if (p === '/api/shell/bootstrap') return route.fulfill({ json: { computer_id: STUB_COMPUTER_ID } });
    if (p === '/api/desktop/state') return route.fulfill({ json: req.method() === 'GET' ? (seedWindow ? { windows: [seedWindow], active_window_id: seedWindow.window_id } : { windows: [], active_window_id: '' }) : { ok: true } });
    if (p === '/api/packages/installed') return route.fulfill({ json: { packages: installed } });
    // F1: the shell (once per sign-in) and the Library sync updates; the stub computer has none.
    if (p === `/api/computers/${STUB_COMPUTER_ID}/packages/updates` && req.method() === 'POST') return route.fulfill({ json: { updates: [], policy: { default: 'manual', apps: {} } } });
    if (p === '/api/preferences/theme' && req.method() === 'GET') return route.fulfill({ json: { theme: {} } }); // no stored preference
    if (p === '/api/preferences/packages.selection' && req.method() === 'GET') return route.fulfill({ json: { key: 'packages.selection', value: {}, default: true } }); // no selection record (L2)
    if (p.startsWith('/api/preferences/pkg.settings.background') && req.method() === 'GET') return route.fulfill({ json: { key: p.slice('/api/preferences/'.length), value: '', default: true } }); // no desktop background chosen: the theme colour
    // Autopaper reads the reader's own Style Guide design at read time; the stub computer has no guide, so the default stands.
    if (p === '/api/texture/style' && req.method() === 'GET') return route.fulfill({ json: { source: 'default', design: { serif: 'classic', headline_weight: '700', dek_style: 'normal', nameplate_rule: 'double' } } });
    // The shell's changelog watch polls for unseen events while signed in; the stub computer has none.
    if (p === '/api/changelog' && req.method() === 'GET') return route.fulfill({ json: { events: [], next_before: 0, head: 0, seen_seq: 0, unseen_major: 0, unseen_more: false } });
    const m = /^\/api\/packages\/modules\/([0-9a-f]{64})\.js$/.exec(p);
    if (m && modules.has(m[1])) return route.fulfill({ status: 200, contentType: 'text/javascript', body: modules.get(m[1]) });
    onUnanswered(`${req.method()} ${p}`);
    return route.fulfill({ status: 404, contentType: 'application/json', body: '{"error":"not_stubbed"}' });
  });
}

/** requestPattern turns an allow_unanswered request ("GET /api/x/*") into a whole-string regex. */
export function requestPattern(request) {
  return new RegExp(`^${request.split('*').map((part) => part.replace(/[.+?^${}()|[\]\\]/g, '\\$&')).join('.*')}$`);
}

// ---- one contract run ----

const windowSel = (name) => `[data-window][data-window-app-id="${name}"]`;
const contentSel = (name) => `${windowSel(name)} [data-window-content]`;

async function until(fn, timeout) {
  const end = Date.now() + timeout;
  for (;;) {
    if (await fn().catch(() => false)) return;
    if (Date.now() > end) throw new Error('timed out');
    await new Promise((r) => setTimeout(r, 100));
  }
}

async function step(fn, what) {
  try {
    return await fn();
  } catch (e) {
    throw new Error(`${what}: ${String(e.message || e).split('\n')[0]}`);
  }
}

/**
 * runContract opens one package in a fresh page of `context` and checks its
 * contract. It never throws for a failed contract: it returns a row
 * { mode, pass, ms, problems, page_errors, console_errors, network, unanswered }.
 */
export async function runContract(context, { base, pkg, mode, installed = [], modules = new Map(), screenshot = '' }) {
  const started = Date.now();
  const row = { mode, pass: false, ms: 0, problems: [], boot_errors: [], page_errors: [], console_errors: [], allowed_console: [], network: [], unanswered: [], allowed_unanswered: [] };
  const page = await context.newPage();
  let opened = pkg.manifest?.app?.launcher?.menu === false; // a saved-window open boots with the shell
  const allow = (pkg.contract.allow_console || []).map((r) => new RegExp(r));
  page.on('pageerror', (e) => (opened ? row.page_errors : row.boot_errors).push(String(e && e.stack ? e.stack.split('\n').slice(0, 3).join(' | ') : e)));
  page.on('console', (m) => {
    if (m.type() !== 'error') return;
    const text = m.text();
    if (NETWORK_NOISE.test(text)) return row.network.push(text);
    if (!opened) return row.boot_errors.push(`console.error: ${text}`);
    if (allow.some((r) => r.test(text))) return row.allowed_console.push(text);
    row.console_errors.push(text);
  });
  try {
    const apiHits = [];
    const noLauncher = pkg.manifest?.app?.launcher?.menu === false;
    const allowUnanswered = (pkg.contract.allow_unanswered || []).map((a) => requestPattern(a.request));
    await stubBackend(page, {
      installed,
      modules,
      api: pkg.contract.api || [],
      // A package with no launcher entry (Source) opens from a saved window, as the shell opens it for a person.
      seedWindow: noLauncher ? { window_id: `contract-${pkg.name}`, app_id: pkg.name, title: pkg.manifest?.display?.title || pkg.name, geometry: { x: 40, y: 40, width: 900, height: 640 }, mode: 'normal', z_index: 1, app_context: {} } : null,
      onApi: (hit) => apiHits.push(hit),
      onUnanswered: (request) => {
        if (!opened) return row.boot_errors.push(`unanswered ${request} (the stub no longer covers the shell's boot)`);
        if (allowUnanswered.some((r) => r.test(request))) return row.allowed_unanswered.push(request);
        row.unanswered.push(request);
      },
    });
    const name = pkg.name;
    await step(() => page.goto(base, { waitUntil: 'domcontentloaded' }), 'the shell did not load');
    await step(() => page.locator('[data-desktop][data-desktop-ready="true"]').waitFor({ timeout: STEP_MS }), 'the desktop never became ready (the stub backend or the shell boot is broken)');
    if (noLauncher) {
      opened = true; // from here, errors are the package's (C8)
    } else {
      await step(() => page.locator('[data-desk-menu-button]').first().click({ timeout: STEP_MS }), 'no app switcher button');
      await step(() => page.locator('[data-desk-sheet]').first().waitFor({ timeout: STEP_MS }), 'the launcher sheet did not open');
      const entry = page.locator(`[data-desk-sheet-app][data-desk-app-id="${name}"]`).first();
      if (!(await entry.count())) {
        const ids = await page.locator('[data-desk-sheet-app]').evaluateAll((els) => els.map((e) => e.getAttribute('data-desk-app-id')));
        throw new Error(`no launcher entry for ${name}; the launcher has ${ids.join(', ') || 'no apps'}`);
      }
      opened = true; // from here, errors are the package's (C8)
      await entry.click();
    }
    await step(() => page.locator(windowSel(name)).first().waitFor({ timeout: STEP_MS }), `no ${name} window opened`);
    let content = page.locator(contentSel(name)).first();
    // Both modes mount a module through the shell's loader (M4): the base copy
    // in-tree, the installed release in installed mode, never the other one.
    {
      const frame = content.locator('[data-package-frame]').first();
      await step(() => frame.waitFor({ timeout: STEP_MS }), 'the window does not host a package module (no PackageAppFrame)');
      const settled = await step(() => page.waitForFunction((sel) => {
        const el = document.querySelector(sel);
        const s = el && el.getAttribute('data-package-state');
        return s && s !== 'loading' ? s : false;
      }, `${contentSel(name)} [data-package-frame]`, { timeout: STEP_MS }), 'the module never left loading');
      const state = await settled.jsonValue();
      const install = (await frame.getAttribute('data-package-install')) || '';
      const isolation = (await frame.getAttribute('data-package-isolation')) || '';
      if (mode !== 'in-tree' && install.startsWith('base:')) throw new Error('the window mounts the base copy: the installed release did not replace it');
      if (mode === 'in-tree' && !install.startsWith('base:')) throw new Error(`the window mounts ${install || 'no record'}, not the base copy`);
      const realm = mode === 'sandboxed' ? 'frame' : 'shell';
      if (isolation !== realm) throw new Error(`the ${mode} module runs in the ${isolation || 'unnamed'} realm, not the ${realm}`);
      if (state !== 'mounted') {
        const why = await frame.innerText().catch(() => '');
        throw new Error(`the shell did not mount the ${mode === 'in-tree' ? 'base' : mode} module (state ${state}): ${why.replace(/\s+/g, ' ').trim()}`);
      }
      // Sandboxed: the app's document is the frame's, so selectors resolve there.
      if (mode === 'sandboxed') content = page.frameLocator(`${contentSel(name)} iframe[data-package-sandbox]`).first().locator('body');
    }
    try {
      await content.locator(pkg.contract.ready).first().waitFor({ state: 'visible', timeout: STEP_MS });
    } catch {
      const text = (await content.innerText().catch(() => '')).replace(/\s+/g, ' ').trim().slice(0, 200);
      throw new Error(`ready selector ${JSON.stringify(pkg.contract.ready)} did not show inside the ${name} window; the window shows: ${text || '(no text)'}`);
    }
    for (const e of pkg.contract.expect || []) {
      const loc = content.locator(e.selector);
      if (e.count !== undefined) {
        const n = await loc.count();
        if (n !== e.count) row.problems.push(`expected ${e.count} of ${JSON.stringify(e.selector)}, found ${n}`);
        continue;
      }
      try {
        await loc.first().waitFor({ state: 'visible', timeout: STEP_MS });
      } catch {
        row.problems.push(`${JSON.stringify(e.selector)} did not show inside the window`);
        continue;
      }
      if (e.text !== undefined) {
        const text = await loc.first().innerText();
        if (!text.includes(e.text)) row.problems.push(`${JSON.stringify(e.selector)} shows ${JSON.stringify(text.slice(0, 120))}, not ${JSON.stringify(e.text)}`);
      }
    }
    for (const [i, st] of (pkg.contract.steps || []).entries()) {
      const kind = STEP_KINDS.find((k) => k in st);
      const what = `step ${i + 1} (${kind} ${JSON.stringify(st[kind])})`;
      const loc = content.locator(st[kind]).first();
      if (kind === 'click') await step(() => loc.click({ timeout: STEP_MS }), what);
      else if (kind === 'fill') await step(() => loc.fill(st.value, { timeout: STEP_MS }), what);
      else if (kind === 'press') await step(async () => { await loc.focus({ timeout: STEP_MS }); await page.keyboard.press(st.key); }, what);
      else if (kind === 'upload') await step(() => loc.setInputFiles({ name: st.name, mimeType: st.mime || 'application/octet-stream', buffer: Buffer.from(st.content) }, { timeout: STEP_MS }), what);
      else {
        try {
          if (st.count !== undefined) {
            await until(async () => (await content.locator(st.see).count()) === st.count, STEP_MS);
          } else {
            await loc.waitFor({ state: 'visible', timeout: STEP_MS });
          }
          if (st.text !== undefined) await until(async () => (await loc.innerText({ timeout: 500 })).includes(st.text), STEP_MS);
        } catch {
          row.problems.push(`${what}${st.text !== undefined ? ` text ${JSON.stringify(st.text)}` : ''}${st.count !== undefined ? ` count ${st.count}` : ''} did not hold`);
          break;
        }
      }
    }
    for (const r of pkg.contract.expect_requests || []) {
      const hit = apiHits.find((h) => h.request === r.request && (r.body === undefined || h.body.includes(r.body)));
      if (!hit) row.problems.push(`the steps never sent ${r.request}${r.body !== undefined ? ` with a body holding ${JSON.stringify(r.body)}` : ''}`);
    }
    // Errors thrown after mount (a failed fetch handled badly, a timer) land in the quiet window.
    await page.waitForLoadState('networkidle', { timeout: 3000 }).catch(() => {});
    await page.waitForTimeout(QUIET_MS);
    if (screenshot) await page.screenshot({ path: screenshot }).catch(() => {});
  } catch (e) {
    row.problems.push(String(e.message || e));
  } finally {
    await page.close().catch(() => {});
  }
  for (const e of row.boot_errors) row.problems.push(`shell boot: ${e}`);
  for (const e of row.page_errors) row.problems.push(`uncaught: ${e}`);
  for (const e of row.console_errors) row.problems.push(`console.error: ${e}`);
  for (const r of [...new Set(row.unanswered)]) row.problems.push(`unanswered: ${r} (stub it in the contract's api, or list it in allow_unanswered with a reason)`);
  row.pass = row.problems.length === 0;
  row.ms = Date.now() - started;
  return row;
}

// ---- the whole run ----

/**
 * checkPackages runs every package's contract in every mode against the
 * served shell and returns the package rows of the receipt. sdkCheck(row,
 * built) may add SDK problems (the spec passes the lock checks).
 */
export async function checkPackages({ browser, base, packages, modes = DEFAULT_MODES, sdkCheck = null, screenshotDir = '' }) {
  const rows = [];
  const built = new Map();
  for (const pkg of packages) {
    const row = {
      name: pkg.name,
      version: pkg.manifest?.version || null,
      tree_digest: pkg.tree_digest,
      contract_digest: pkg.contract_digest,
      requires_sdk: pkg.manifest?.app?.requires_sdk || '^2.0',
      sdk_imports: null,
      min_sdk: null,
      module: null,
      problems: [...pkg.problems],
      modes: {},
      pass: false,
    };
    rows.push(row);
    if (!pkg.manifest) continue;
    if (modes.includes('installed') || modes.includes('sandboxed') || sdkCheck) {
      const b = await buildPackageModule({ name: pkg.name, dir: pkg.dir });
      if (!b.ok) {
        // Name the refusal itself (the boundary plugin's or the build's own line), not Node's stack dump after it.
        const lines = b.log.split('\n').map((l) => l.trim()).filter(Boolean);
        const why = lines.find((l) => /package "|module\/v1|Error:/.test(l)) || lines.slice(-4).join(' | ');
        row.problems.push(`the module/v1 build failed (the package boundary or the build): ${why}`);
      }
      else {
        row.module = { abi: MODULE_ABI, sha256: b.sha256, bytes: b.bytes.length };
        row.sdk_imports = b.sdk_imports;
        built.set(pkg.name, b);
        if (sdkCheck) sdkCheck(row, b);
      }
    }
  }
  const manifestOf = (name) => packages.find((p) => p.name === name).manifest;
  const modules = new Map([...built.values()].map((b) => [b.sha256, b.bytes]));
  for (const row of rows) {
    const pkg = packages.find((p) => p.name === row.name);
    for (const mode of modes) {
      if (!pkg.contract || validateContract(pkg.contract).length) {
        row.modes[mode] = { mode, pass: false, ms: 0, problems: ['not run: no valid contract'] };
        continue;
      }
      if (mode !== 'in-tree' && !built.has(row.name)) {
        row.modes[mode] = { mode, pass: false, ms: 0, problems: ['not run: no module'] };
        continue;
      }
      // Installed and sandboxed: only this package is listed, so a failure is its own.
      const installed = mode === 'in-tree' ? [] : [installedRecord(manifestOf(row.name), built.get(row.name), { stranger: mode === 'sandboxed' })];
      const context = await browser.newContext({ viewport: { width: 1280, height: 800 } });
      try {
        row.modes[mode] = await runContract(context, {
          base,
          pkg,
          mode,
          installed,
          modules,
          screenshot: screenshotDir ? path.join(screenshotDir, `${row.name}.${mode}.png`) : '',
        });
      } finally {
        await context.close();
      }
    }
    row.pass = row.problems.length === 0 && modes.every((m) => row.modes[m]?.pass);
  }
  return rows;
}

export function gitCommit(repoRoot = REPO_ROOT) {
  if (process.env.GITHUB_SHA) return process.env.GITHUB_SHA;
  const r = spawnSync('git', ['-C', repoRoot, 'rev-parse', 'HEAD'], { encoding: 'utf8' });
  const sha = r.status === 0 ? r.stdout.trim() : '';
  const dirty = spawnSync('git', ['-C', repoRoot, 'status', '--porcelain', '--', 'frontend', 'internal/pkgruntime'], { encoding: 'utf8' });
  return sha && dirty.stdout && dirty.stdout.trim() ? `${sha}+dirty` : sha || null;
}

/** makeReceipt assembles the receipt (C11, C12); pass is the AND of every part given. */
export function makeReceipt({ sdk = null, shell = null, packages = [], checks = {} }) {
  const parts = [...packages.map((p) => p.pass), ...(sdk ? [sdk.ok] : []), ...Object.values(checks).map((c) => c.pass)];
  return {
    schema: RECEIPT_SCHEMA,
    commit: gitCommit(),
    created_at: new Date().toISOString(),
    sdk,
    shell,
    packages,
    checks,
    pass: parts.length > 0 && parts.every(Boolean),
  };
}

export function writeReceipt(file, receipt) {
  fs.mkdirSync(path.dirname(path.resolve(file)), { recursive: true });
  fs.writeFileSync(file, `${JSON.stringify(receipt, null, 2)}\n`);
}

// ---- CLI ----

function argValue(argv, flag) {
  const i = argv.indexOf(flag);
  return i >= 0 ? argv[i + 1] : '';
}

async function main(argv) {
  const ownPackages = path.join(FRONTEND_ROOT, 'src/packages');
  const packagesDir = path.resolve(argValue(argv, '--packages-dir') || ownPackages);
  const foreign = packagesDir !== ownPackages;
  const modes = (argValue(argv, '--modes') || (foreign ? 'installed' : DEFAULT_MODES.join(','))).split(',').filter(Boolean);
  for (const m of modes) if (!MODES.includes(m)) throw new Error(`unknown mode ${m}; modes are ${MODES.join(', ')}`);
  if (foreign && modes.includes('in-tree')) throw new Error('--packages-dir trees are not in the shell build; run them in the installed or sandboxed mode');
  const only = (argValue(argv, '--only') || '').split(',').filter(Boolean);
  const receiptFile = path.resolve(argValue(argv, '--receipt') || path.join(FRONTEND_ROOT, 'test-results/modularity-receipt.json'));
  let dist = argValue(argv, '--dist');
  let tmp = '';
  if (!dist) {
    tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'choir-contract-shell-'));
    dist = await buildShell({ outDir: path.join(tmp, 'dist') });
  }
  const packages = loadPackages(packagesDir).filter((p) => !only.length || only.includes(p.name));
  const { chromium } = await import(pathToFileURL(path.join(FRONTEND_ROOT, 'node_modules/@playwright/test/index.mjs')).href);
  const browser = await chromium.launch({ ...(process.env.CHOIR_BROWSER ? { executablePath: process.env.CHOIR_BROWSER } : {}), args: ['--no-sandbox', '--disable-dev-shm-usage'] });
  const server = await serveDist(dist);
  let receipt;
  try {
    const rows = await checkPackages({ browser, base: server.base, packages, modes });
    receipt = makeReceipt({ shell: { dist: path.resolve(dist), digest: distDigest(dist) }, packages: rows });
  } finally {
    await browser.close();
    await server.close();
    if (tmp) fs.rmSync(tmp, { recursive: true, force: true });
  }
  writeReceipt(receiptFile, receipt);
  for (const p of receipt.packages) {
    const modesText = Object.values(p.modes).map((m) => `${m.mode} ${m.pass ? 'pass' : 'FAIL'}`).join(', ');
    console.log(`package-contracts: ${p.name} ${p.pass ? 'pass' : 'FAIL'} (${modesText})`);
    for (const m of Object.values(p.modes)) for (const problem of m.problems || []) console.log(`  ${m.mode}: ${problem}`);
    for (const problem of p.problems) console.log(`  ${problem}`);
  }
  console.log(`package-contracts: receipt ${receiptFile}: ${receipt.pass ? 'pass' : 'FAILED'}`);
  return receipt.pass ? 0 : 1;
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  main(process.argv.slice(2)).then((c) => process.exit(c), (e) => {
    console.error(`package-contracts: ${e.message}`);
    process.exit(2);
  });
}
