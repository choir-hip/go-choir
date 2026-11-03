#!/usr/bin/env node
// choir-shot: the screenshot helper an engineer runs in its capsule.
//
// It is a tool the engineer runs, not a platform stage: nothing in Freeze,
// verify, judge or acceptance calls it, waits for it or refuses for its absence
// (docs/problems/selfdev-screens-are-not-files-2026-11-03.md: "no platform
// capture"). It exists because an engineer wrote its own shots.mjs, re-emitted
// it up to 14 times, and ran the whole set after every edit for 60 to 275 s a
// run (docs/evidence/selfdev-round-cycle-time-2026-11-03.md).
//
// What it does: builds the frontend once (cached by a hash of the sources, in
// /tmp), serves it, opens the store Chromium ($CHOIR_BROWSER) with the pinned
// Playwright from $CHOIR_FRONTEND_DEPS, and takes the named shots, waiting for
// network idle, fonts and each shot's selector (never a fixed sleep). PNGs go
// to /var/lib/artifact/screens/, which Freeze saves as files Texture embeds.
//
//   node /workspace/platform/scripts/choir-shot.mjs --shots /tmp/shots.mjs
//   node ... --shots /tmp/shots.mjs --only slides-home            # one shot
//   node ... --shots /tmp/shots.mjs --only a,b --preset mobile    # two, phone
//   node ... --shots /tmp/shots.mjs --size 1000x700               # custom size
//   node ... --shots /tmp/shots.mjs --list
//
// The shots file is an ES module whose default export is a list of
//   { name, path = '/', selector, routes(context, base), before(page, base),
//     act(page), fullPage = false }
// name: letters, digits, '.', '-', '_'. routes mocks the backend
// (context.route) for signed-in views. before runs before the page is opened
// (seed storage with context.addInitScript through page.context()); act runs
// after it is ready (click, type), and the helper waits for idle again.
// Files: <name>.png for the desktop preset, <name>-<preset>.png otherwise,
// <name>-<W>x<H>.png for --size. Presets: desktop 1280x800, tablet 820x1180,
// mobile 390x844. Run single shots while iterating; run the full set once
// before choir.Freeze (a change no one looks at needs no shot).
//
// Real content: when /computer/state.db exists (an implementation capsule's
// private copy of the computer's store), the helper starts an autoputer itself
// over /computer (RUNTIME_STORE_PATH=/computer/state AUTOPUTER_FILES_ROOT=
// /computer/files AUTOPUTER_PORT=8082 CHOIR_GUEST_TRUST_LOOPBACK=1) as a child
// of this process, so it lives exactly as long as the shots, and kills it at
// exit. The binary is /var/lib/artifact/release/bin/autoputer, else
// /tmp/autoputer, else built once with go build from /workspace/platform. The
// served frontend proxies /api/* to it with X-Authenticated-User: owner and
// answers GET /auth/session as that owner; a shot's routes() (context.route)
// still wins for any URL it handles. --no-computer keeps the mocked backend.
//
// Run it as a capsule job (choir.Start), not a choir.Exec: a first build and a
// browser start outlast 30 s. It does not touch /workspace/platform (the build runs in a private copy under /tmp).
import { spawn, spawnSync } from 'node:child_process';
import { createHash } from 'node:crypto';
import fs from 'node:fs';
import http from 'node:http';
import net from 'node:net';
import { createRequire } from 'node:module';
import path from 'node:path';
import { pathToFileURL } from 'node:url';

export const PRESETS = {
  desktop: { width: 1280, height: 800 },
  tablet: { width: 820, height: 1180 },
  mobile: { width: 390, height: 844, isMobile: true, hasTouch: true, deviceScaleFactor: 2 },
};
const NAME_RE = /^[A-Za-z0-9._-]+$/;
const STEP_MS = 15000;
export const COMPUTER_DIR = '/computer';
export const AUTOPUTER_PORT = 8082;
export const AUTOPUTER_USER = 'owner';
const AUTOPUTER_BINARIES = ['/var/lib/artifact/release/bin/autoputer', '/tmp/autoputer'];

export function parseArgs(argv) {
  const o = { shots: '', only: [], presets: [], size: '', out: '/var/lib/artifact/screens', list: false, computer: true };
  for (let i = 0; i < argv.length; i++) {
    const a = argv[i];
    const val = () => {
      if (i + 1 >= argv.length) throw new Error(`${a} needs a value`);
      return argv[++i];
    };
    if (a === '--shots') o.shots = val();
    else if (a === '--only') o.only = val().split(',').filter(Boolean);
    else if (a === '--preset') o.presets = val().split(',').filter(Boolean);
    else if (a === '--size') o.size = val();
    else if (a === '--out') o.out = val();
    else if (a === '--list') o.list = true;
    else if (a === '--no-computer') o.computer = false;
    else throw new Error(`unknown flag ${a}`);
  }
  if (!o.shots) throw new Error('--shots FILE is required (an ES module whose default export is a list of shots)');
  if (o.size && o.presets.length) throw new Error('--size and --preset are exclusive');
  if (o.size && !/^\d{2,5}x\d{2,5}$/.test(o.size)) throw new Error(`--size ${o.size}: want WIDTHxHEIGHT, such as 1000x700`);
  for (const p of o.presets) if (!PRESETS[p]) throw new Error(`--preset ${p}: want one of ${Object.keys(PRESETS).join(', ')}`);
  return o;
}

// plan expands the selected shots and presets into the files to write.
export function plan(shots, o) {
  if (!Array.isArray(shots) || shots.length === 0) throw new Error('the shots file must default-export a non-empty list');
  const seen = new Set();
  for (const s of shots) {
    if (!s || !NAME_RE.test(s.name || '')) throw new Error(`shot name ${JSON.stringify(s && s.name)}: letters, digits, '.', '-' and '_' only`);
    if (seen.has(s.name)) throw new Error(`duplicate shot name ${s.name}`);
    seen.add(s.name);
  }
  for (const n of o.only) if (!seen.has(n)) throw new Error(`--only ${n}: no such shot (have ${[...seen].join(', ')})`);
  const picked = o.only.length ? shots.filter((s) => o.only.includes(s.name)) : shots;
  let views;
  if (o.size) {
    const [width, height] = o.size.split('x').map(Number);
    views = [{ label: o.size, viewport: { width, height } }];
  } else {
    const names = o.presets.length ? o.presets : ['desktop'];
    views = names.map((n) => ({ label: n, preset: PRESETS[n] }));
  }
  const out = [];
  for (const shot of picked)
    for (const v of views) {
      const { width, height, ...device } = v.preset || { ...v.viewport };
      out.push({ shot, viewport: { width, height }, device, file: `${shot.name}${v.label === 'desktop' ? '' : '-' + v.label}.png` });
    }
  return out;
}

// pickAutoputerBinary returns the first existing binary in the fixed order, or
// '' when none exists and the caller must build one.
export function pickAutoputerBinary(exists = fs.existsSync) {
  return AUTOPUTER_BINARIES.find((b) => exists(b)) || '';
}

// isProxied is the rule for what the served frontend sends to the autoputer:
// every /api/ URL. /auth/session is answered locally; other /auth URLs 404.
export function isProxied(pathname) {
  return pathname === '/api' || pathname.startsWith('/api/');
}

export function autoputerEnv(base = process.env) {
  return { ...base, RUNTIME_STORE_PATH: `${COMPUTER_DIR}/state`, AUTOPUTER_FILES_ROOT: `${COMPUTER_DIR}/files`, AUTOPUTER_PORT: String(AUTOPUTER_PORT), CHOIR_GUEST_TRUST_LOOPBACK: '1' };
}

function portOpen(port) {
  return new Promise((resolve) => {
    const sock = net.connect(port, '127.0.0.1');
    sock.once('connect', () => { sock.destroy(); resolve(true); });
    sock.once('error', () => { sock.destroy(); resolve(false); });
  });
}

function waitForPort(port, child, deadlineMs) {
  const end = Date.now() + deadlineMs;
  return new Promise((resolve, reject) => {
    const tryOnce = () => {
      if (child.exitCode !== null) return reject(new Error(`the autoputer exited (code ${child.exitCode}) before port ${port} accepted connections`));
      const sock = net.connect(port, '127.0.0.1');
      sock.once('connect', () => { sock.destroy(); resolve(); });
      sock.once('error', () => {
        sock.destroy();
        if (Date.now() > end) reject(new Error(`the autoputer did not accept connections on 127.0.0.1:${port} within ${deadlineMs / 1000} s`));
        else setTimeout(tryOnce, 200);
      });
    };
    tryOnce();
  });
}

// startAutoputer starts the autoputer over /computer as a child of this
// process and returns it once its port accepts connections.
async function startAutoputer(root, holder) {
  if (await portOpen(AUTOPUTER_PORT)) throw new Error(`127.0.0.1:${AUTOPUTER_PORT} already accepts connections (an autoputer started by hand?); stop it, since the shots would read its content`);
  let bin = pickAutoputerBinary();
  if (!bin) {
    bin = AUTOPUTER_BINARIES[1];
    console.log('choir-shot: no autoputer binary found; the first run builds it (go build -o /tmp/autoputer ./cmd/autoputer)');
    const r = spawnSync('go', ['build', '-o', bin, './cmd/autoputer'], { cwd: root, stdio: 'inherit' });
    if (r.status !== 0) throw new Error(`go build of the autoputer failed (exit ${r.status}); pass --no-computer to mock the backend instead`);
  }
  const child = spawn(bin, [], { env: autoputerEnv(), stdio: ['ignore', 'inherit', 'inherit'] });
  holder.child = child;
  child.on('error', () => {});
  try {
    await waitForPort(AUTOPUTER_PORT, child, 30000);
  } catch (e) {
    child.kill();
    throw e;
  }
  console.log(`choir-shot: autoputer (${bin}) is up on 127.0.0.1:${AUTOPUTER_PORT}`);
  return child;
}

function sourceHash(frontend) {
  const h = createHash('sha256');
  const walk = (p) => {
    for (const e of fs.readdirSync(p, { withFileTypes: true }).sort((a, b) => (a.name < b.name ? -1 : 1))) {
      if (['node_modules', 'dist', 'tests', '.vite'].includes(e.name)) continue;
      const f = path.join(p, e.name);
      if (e.isDirectory()) walk(f);
      else h.update(f).update(fs.readFileSync(f));
    }
  };
  walk(frontend);
  return h.digest('hex').slice(0, 16);
}

// build runs vite once per source hash, in a private copy of the frontend under
// /tmp whose node_modules is a link to $CHOIR_FRONTEND_DEPS/node_modules. It
// never reads or writes /workspace/platform/frontend/node_modules: a directory
// an earlier run or the engineer left there (a stray .vite cache, a half
// install) hid the locked dependencies from the build ("could not resolve
// @chenglou/pretext", problem 18), and a link made there could freeze into the
// candidate. Vite follows the link (resolve.preserveSymlinks is off), so a
// dependency resolves through the read-only Nix tree and its real path.
function build(frontend, deps) {
  const cacheRoot = '/tmp/choir-shot';
  const key = path.join(cacheRoot, sourceHash(frontend));
  const dist = path.join(key, 'dist');
  if (fs.existsSync(path.join(key, 'done'))) {
    console.log(`choir-shot: build cached (${dist})`);
    return dist;
  }
  const depsModules = path.join(deps, 'node_modules');
  const missing = missingDependencies(frontend, depsModules);
  if (missing.length) {
    throw new Error(`the locked dependency tree ${depsModules} lacks ${missing.join(', ')}: package.json changed since the tree was built, so the shots need an npm ci round with network "ecosystem_proxy" (a build job does that); nothing here can install them`);
  }
  const work = path.join(key, 'work');
  fs.rmSync(key, { recursive: true, force: true });
  fs.mkdirSync(work, { recursive: true });
  for (const e of fs.readdirSync(frontend)) {
    if (['node_modules', 'dist', 'tests', '.vite'].includes(e)) continue;
    fs.cpSync(path.join(frontend, e), path.join(work, e), { recursive: true });
  }
  fs.symlinkSync(depsModules, path.join(work, 'node_modules'));
  const t = Date.now();
  const r = spawnSync(path.join(work, 'node_modules', '.bin', 'vite'), ['build', '--outDir', dist, '--emptyOutDir'], { cwd: work, stdio: 'inherit' });
  fs.rmSync(work, { recursive: true, force: true });
  if (r.status !== 0) throw new Error(`vite build failed (exit ${r.status}); the full npm run build prints the same error`);
  console.log(`choir-shot: built in ${((Date.now() - t) / 1000).toFixed(1)} s`);
  fs.writeFileSync(path.join(key, 'done'), ''); // a build killed midway has no marker and is redone
  for (const d of fs.readdirSync(cacheRoot)) if (path.join(cacheRoot, d) !== key) fs.rmSync(path.join(cacheRoot, d), { recursive: true, force: true });
  return dist;
}

// missingDependencies names each package.json dependency and dev that the
// locked tree does not hold, so a stale tree fails here with its cause.
export function missingDependencies(frontend, depsModules) {
  const pkg = JSON.parse(fs.readFileSync(path.join(frontend, 'package.json'), 'utf8'));
  return Object.keys({ ...pkg.dependencies, ...pkg.devDependencies }).filter((n) => !fs.existsSync(path.join(depsModules, n, 'package.json')));
}

const TYPES = { '.html': 'text/html', '.js': 'text/javascript', '.mjs': 'text/javascript', '.css': 'text/css', '.json': 'application/json', '.svg': 'image/svg+xml', '.png': 'image/png', '.woff2': 'font/woff2', '.ico': 'image/x-icon', '.webmanifest': 'application/manifest+json' };

function proxy(req, res) {
  const headers = { ...req.headers, host: `127.0.0.1:${AUTOPUTER_PORT}`, 'x-authenticated-user': AUTOPUTER_USER };
  const up = http.request({ host: '127.0.0.1', port: AUTOPUTER_PORT, method: req.method, path: req.url, headers }, (r) => {
    res.writeHead(r.statusCode, r.headers);
    r.pipe(res);
  });
  up.on('error', (e) => { if (!res.headersSent) res.writeHead(502, { 'content-type': 'text/plain' }); res.end(`autoputer: ${e.message}`); });
  res.on('close', () => up.destroy());
  req.pipe(up);
}

// tunnel carries a WebSocket upgrade under /api (the live stream) to the
// autoputer as the owner; anything else is refused.
function tunnel(req, socket, head) {
  const u = new URL(req.url, 'http://x').pathname;
  if (!isProxied(u)) { socket.destroy(); return; }
  const up = net.connect(AUTOPUTER_PORT, '127.0.0.1', () => {
    const lines = [`${req.method} ${req.url} HTTP/1.1`];
    const h = { ...req.headers, host: `127.0.0.1:${AUTOPUTER_PORT}`, 'x-authenticated-user': AUTOPUTER_USER };
    for (const [k, v] of Object.entries(h)) lines.push(`${k}: ${v}`);
    up.write(lines.join('\r\n') + '\r\n\r\n');
    if (head && head.length) up.write(head);
    up.pipe(socket);
    socket.pipe(up);
  });
  up.on('error', () => socket.destroy());
  socket.on('error', () => up.destroy());
  socket.on('close', () => up.destroy());
}

function serve(dist, computer = false) {
  const server = http.createServer((req, res) => {
    const u = decodeURIComponent(new URL(req.url, 'http://x').pathname);
    if (computer && isProxied(u)) return proxy(req, res);
    if (computer && u === '/auth/session') {
      res.writeHead(200, { 'content-type': 'application/json' }).end(JSON.stringify({ authenticated: true, user: { id: AUTOPUTER_USER, username: AUTOPUTER_USER } }));
      return;
    }
    if (computer && u.startsWith('/auth/')) { res.writeHead(404).end('not found'); return; }
    let f = path.join(dist, path.normalize(u));
    if (!f.startsWith(dist)) { res.writeHead(403).end(); return; }
    if (!fs.existsSync(f) || fs.statSync(f).isDirectory()) {
      if (path.extname(u)) { res.writeHead(404).end('not found'); return; } // a missing asset or an unmocked /api call
      f = path.join(dist, 'index.html'); // the SPA shell
    }
    res.writeHead(200, { 'content-type': TYPES[path.extname(f)] || 'application/octet-stream' });
    fs.createReadStream(f).pipe(res);
  });
  if (computer) server.on('upgrade', (req, socket, head) => tunnel(req, socket, head));
  return new Promise((resolve) => server.listen(0, '127.0.0.1', () => resolve({ server, base: `http://127.0.0.1:${server.address().port}` })));
}

async function settle(page, selector) {
  await page.waitForLoadState('networkidle', { timeout: STEP_MS });
  if (selector) await page.waitForSelector(selector, { state: 'visible', timeout: STEP_MS });
  await Promise.race([page.evaluate(() => document.fonts && document.fonts.ready), new Promise((r) => setTimeout(r, STEP_MS))]);
}

async function main() {
  const o = parseArgs(process.argv.slice(2));
  const mod = await import(pathToFileURL(path.resolve(o.shots)).href);
  const items = plan(mod.default, o);
  if (o.list) {
    for (const it of items) console.log(it.file);
    return 0;
  }
  const root = process.env.CHOIR_PLATFORM_ROOT || path.resolve(path.dirname(new URL(import.meta.url).pathname), '..');
  const frontend = path.join(root, 'frontend');
  const deps = process.env.CHOIR_FRONTEND_DEPS;
  if (!deps) throw new Error('CHOIR_FRONTEND_DEPS is not set (a capsule job has it)');
  const exe = process.env.CHOIR_BROWSER;
  if (!exe) throw new Error('CHOIR_BROWSER is not set (a capsule job has the store Chromium)');
  const req = createRequire(path.join(deps, 'package.json'));
  let pw;
  try { pw = req('playwright'); } catch { pw = req('@playwright/test'); } // npm hoists playwright; either exports chromium
  const { chromium } = pw;
  const dist = build(frontend, deps);
  const useComputer = o.computer && fs.existsSync(path.join(COMPUTER_DIR, 'state.db'));
  const holder = { child: null }; // handlers first: a signal during the build or the wait must not orphan the child
  const stopAutoputer = () => { if (holder.child && holder.child.exitCode === null) holder.child.kill(); };
  process.on('exit', stopAutoputer);
  for (const sig of ['SIGINT', 'SIGTERM']) process.on(sig, () => { stopAutoputer(); process.exit(130); });
  if (useComputer) await startAutoputer(root, holder);
  const { server, base } = await serve(dist, useComputer);
  fs.mkdirSync(o.out, { recursive: true });
  const browser = await chromium.launch({ executablePath: exe, args: ['--no-sandbox', '--disable-dev-shm-usage'] });
  const failed = [];
  try {
    for (const it of items) {
      const t = Date.now();
      const context = await browser.newContext({ viewport: it.viewport, ...it.device });
      try {
        const s = it.shot;
        if (s.routes) await s.routes(context, base);
        const page = await context.newPage();
        if (s.before) await s.before(page, base);
        await page.goto(base + (s.path || '/'), { waitUntil: 'load', timeout: STEP_MS });
        await settle(page, s.selector);
        if (s.act) {
          await s.act(page);
          await settle(page, s.selector);
        }
        await page.screenshot({ path: path.join(o.out, it.file), fullPage: !!s.fullPage });
        console.log(`choir-shot: ${it.file} ${((Date.now() - t) / 1000).toFixed(1)} s`);
      } catch (e) {
        failed.push(it.file);
        console.error(`choir-shot: ${it.file} FAILED: ${String(e.message || e).split('\n')[0]}`);
      } finally {
        await context.close();
      }
    }
  } finally {
    await browser.close();
    server.close();
    stopAutoputer();
  }
  const have = fs.readdirSync(o.out).filter((f) => f.endsWith('.png')).length;
  if (have > 12) console.warn(`choir-shot: ${have} PNGs in ${o.out}; Freeze takes at most 12`);
  console.log(JSON.stringify({ shots: items.length - failed.length, failed, out: o.out }));
  return failed.length ? 1 : 0;
}

if (import.meta.url === pathToFileURL(process.argv[1] || '').href) {
  main().then((c) => process.exit(c), (e) => { console.error(`choir-shot: ${e.message}`); process.exit(2); });
}
