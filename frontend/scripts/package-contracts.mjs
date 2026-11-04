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
//     "allow_console": ["<regex>"],     app console errors accepted, by name
//     "note": "<what the contract shows>"
//   }
//
// Every selector is resolved inside the app window's content
// ([data-window][data-window-app-id=<name>] [data-window-content]), never the
// shell chrome. The runner opens the package the way a person does (the app
// switcher, the launcher entry, the window) in a shell whose backend is a stub
// that answers the four boot routes (/auth/session, /api/shell/bootstrap,
// /api/desktop/state, /api/packages/installed), the contract's `api` entries,
// and 404 for the rest. Any uncaught exception, or a console error that is not
// the browser's own "Failed to load resource" line, fails the run.
//
// Two modes:
//   in-tree    the package as the shell build bundles it (svelte-inline/v1);
//   installed  the package built to one module/v1 module by the computer's own
//              build (internal/pkgruntime/build-module.mjs) and listed as an
//              installed package, so the shell's real loader checks its
//              digest, requires_sdk and SDK names and mounts it.
//
// The Playwright spec (tests/package-modularity.spec.js) is the CI gate and
// uses the exports. The CLI runs the same contracts against any built shell:
//
//   node scripts/package-contracts.mjs [--dist DIR] [--packages-dir DIR]
//        [--only a,b] [--modes in-tree,installed] [--receipt FILE]
//
// --dist is a built shell (default: build this tree into a temp directory).
// --packages-dir holds package trees with their contracts (default
// src/packages). Trees from elsewhere (a computer's installed releases) run in
// the installed mode only. Chromium: $CHOIR_BROWSER if set, else Playwright's.
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
export const RECEIPT_SCHEMA = 'choir.modularity-receipt/v1';
export const MODULE_ABI = 'choir.app-frontend/module/v1';
/** The stub computer's id; a contract's `api` paths that name a computer use it. */
export const STUB_COMPUTER_ID = 'contract-computer';
export const STUB_USER = 'contract-owner';
export const MODES = ['in-tree', 'installed'];

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
  const allowed = new Set(['schema', 'ready', 'expect', 'api', 'allow_console', 'note']);
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
  if (c.note !== undefined && typeof c.note !== 'string') problems.push('note must be a string');
  return problems;
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
    for (const script of ['build-module.mjs', 'package-boundary-plugin.mjs']) fs.copyFileSync(path.join(runtimeDir, script), path.join(front, script));
    fs.copyFileSync(path.join(frontendRoot, 'package.json'), path.join(front, 'package.json'));
    fs.symlinkSync(path.join(frontendRoot, 'node_modules'), path.join(front, 'node_modules'));
    const manifest = JSON.parse(fs.readFileSync(path.join(dir, 'choir-package.json'), 'utf8'));
    const requiresSdk = manifest?.app?.requires_sdk || '^1.0';
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

/** installedRecord is the GET /api/packages/installed row for a built module. */
export function installedRecord(manifest, built) {
  return {
    name: manifest.name,
    release_id: `contract-${built.sha256.slice(0, 16)}`,
    install_id: `contract-install-${manifest.name}`,
    installed_at: '2026-11-04T00:00:00Z',
    requires_sdk: built.requires_sdk,
    manifest,
    module: { abi: MODULE_ABI, sha256: built.sha256, url: `/api/packages/modules/${built.sha256}.js` },
    available: true,
  };
}

/**
 * stubBackend routes /auth and /api for one page (C13): the boot routes, the
 * installed list and its modules, the contract's api entries, 404 otherwise.
 * It returns the list of unanswered requests it saw.
 */
export async function stubBackend(page, { installed = [], modules = new Map(), api = [] } = {}) {
  const unanswered = [];
  await page.routeWebSocket(/\/api\/ws\b/, () => {}); // the live channel opens and stays quiet
  await page.route(/\/(auth|api)\//, (route) => {
    const req = route.request();
    const url = new URL(req.url());
    const p = url.pathname;
    const own = api.find((a) => a.method === req.method() && a.path === p);
    if (own) return route.fulfill({ status: own.status || 200, contentType: 'application/json', body: JSON.stringify(own.json ?? {}) });
    if (p === '/auth/session') return route.fulfill({ json: { authenticated: true, user: { id: STUB_USER, username: STUB_USER } } });
    if (p === '/api/shell/bootstrap') return route.fulfill({ json: { computer_id: STUB_COMPUTER_ID } });
    if (p === '/api/desktop/state') return route.fulfill({ json: req.method() === 'GET' ? { windows: [], active_window_id: '' } : { ok: true } });
    if (p === '/api/packages/installed') return route.fulfill({ json: { packages: installed } });
    const m = /^\/api\/packages\/modules\/([0-9a-f]{64})\.js$/.exec(p);
    if (m && modules.has(m[1])) return route.fulfill({ status: 200, contentType: 'text/javascript', body: modules.get(m[1]) });
    unanswered.push(`${req.method()} ${p}`);
    return route.fulfill({ status: 404, contentType: 'application/json', body: '{"error":"not_stubbed"}' });
  });
  return unanswered;
}

// ---- one contract run ----

const windowSel = (name) => `[data-window][data-window-app-id="${name}"]`;
const contentSel = (name) => `${windowSel(name)} [data-window-content]`;

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
  const row = { mode, pass: false, ms: 0, problems: [], boot_errors: [], page_errors: [], console_errors: [], allowed_console: [], network: [], unanswered: [] };
  const page = await context.newPage();
  let opened = false;
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
    row.unanswered = await stubBackend(page, { installed, modules, api: pkg.contract.api || [] });
    const name = pkg.name;
    await step(() => page.goto(base, { waitUntil: 'domcontentloaded' }), 'the shell did not load');
    await step(() => page.locator('[data-desktop][data-desktop-ready="true"]').waitFor({ timeout: STEP_MS }), 'the desktop never became ready (the stub backend or the shell boot is broken)');
    await step(() => page.locator('[data-desk-menu-button]').first().click({ timeout: STEP_MS }), 'no app switcher button');
    await step(() => page.locator('[data-desk-sheet]').first().waitFor({ timeout: STEP_MS }), 'the launcher sheet did not open');
    const entry = page.locator(`[data-desk-sheet-app][data-desk-app-id="${name}"]`).first();
    if (!(await entry.count())) {
      const ids = await page.locator('[data-desk-sheet-app]').evaluateAll((els) => els.map((e) => e.getAttribute('data-desk-app-id')));
      throw new Error(`no launcher entry for ${name}; the launcher has ${ids.join(', ') || 'no apps'}`);
    }
    opened = true; // from here, errors are the package's (C8)
    await entry.click();
    await step(() => page.locator(windowSel(name)).first().waitFor({ timeout: STEP_MS }), `no ${name} window opened`);
    const content = page.locator(contentSel(name)).first();
    if (mode === 'installed') {
      const frame = content.locator('[data-package-frame]').first();
      await step(() => frame.waitFor({ timeout: STEP_MS }), 'the window does not host the installed module (no PackageAppFrame): the in-tree package was not replaced');
      const settled = await step(() => page.waitForFunction((sel) => {
        const el = document.querySelector(sel);
        const s = el && el.getAttribute('data-package-state');
        return s && s !== 'loading' ? s : false;
      }, `${contentSel(name)} [data-package-frame]`, { timeout: STEP_MS }), 'the installed module never left loading');
      const state = await settled.jsonValue();
      if (state !== 'mounted') {
        const why = await frame.innerText().catch(() => '');
        throw new Error(`the shell did not mount the installed module (state ${state}): ${why.replace(/\s+/g, ' ').trim()}`);
      }
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
export async function checkPackages({ browser, base, packages, modes = MODES, sdkCheck = null, screenshotDir = '' }) {
  const rows = [];
  const built = new Map();
  for (const pkg of packages) {
    const row = {
      name: pkg.name,
      version: pkg.manifest?.version || null,
      tree_digest: pkg.tree_digest,
      contract_digest: pkg.contract_digest,
      requires_sdk: pkg.manifest?.app?.requires_sdk || '^1.0',
      sdk_imports: null,
      min_sdk: null,
      module: null,
      problems: [...pkg.problems],
      modes: {},
      pass: false,
    };
    rows.push(row);
    if (!pkg.manifest) continue;
    if (modes.includes('installed') || sdkCheck) {
      const b = await buildPackageModule({ name: pkg.name, dir: pkg.dir });
      if (!b.ok) row.problems.push(`the module/v1 build failed (the package boundary or the build): ${b.log.split('\n').slice(-4).join(' | ')}`);
      else {
        row.module = { abi: MODULE_ABI, sha256: b.sha256, bytes: b.bytes.length };
        row.sdk_imports = b.sdk_imports;
        built.set(pkg.name, b);
        if (sdkCheck) sdkCheck(row, b);
      }
    }
  }
  const installedAll = [...built.entries()].map(([name, b]) => installedRecord(packages.find((p) => p.name === name).manifest, b));
  const modules = new Map([...built.values()].map((b) => [b.sha256, b.bytes]));
  for (const row of rows) {
    const pkg = packages.find((p) => p.name === row.name);
    for (const mode of modes) {
      if (!pkg.contract || validateContract(pkg.contract).length) {
        row.modes[mode] = { mode, pass: false, ms: 0, problems: ['not run: no valid contract'] };
        continue;
      }
      if (mode === 'installed' && !built.has(row.name)) {
        row.modes[mode] = { mode, pass: false, ms: 0, problems: ['not run: no module'] };
        continue;
      }
      // Installed: only this package is listed, so a failure is its own.
      const installed = mode === 'installed' ? installedAll.filter((r) => r.name === row.name) : [];
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
  const modes = (argValue(argv, '--modes') || (foreign ? 'installed' : MODES.join(','))).split(',').filter(Boolean);
  for (const m of modes) if (!MODES.includes(m)) throw new Error(`unknown mode ${m}; modes are ${MODES.join(', ')}`);
  if (foreign && modes.includes('in-tree')) throw new Error('--packages-dir trees are not in the shell build; run them in the installed mode only');
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
