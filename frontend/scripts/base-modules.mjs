// The base packages as shipped modules (forced modularity M4;
// docs/design/forced-modularity-build-2026-11-04.md section 3).
//
// Every first-party app package under src/packages/<name>/ is built by the
// computer's own module build (build-module.mjs, a byte copy of
// internal/pkgruntime/build-module.mjs) into one module/v1 ES module, emitted
// beside the shell at assets/packages/<sha256>.js. The shell never bundles a
// package's Svelte: it imports the virtual module `virtual:choir-base-packages`
// (name, requires_sdk, sha256, url per package) and mounts each through the
// same loader an installed package goes through (PackageAppFrame: digest,
// requires_sdk and SDK names checked, then mount(target, ctx)). One load path,
// one set of failure modes: a base package proves on every build the contract
// an installed one does.
//
// Failure modes:
//  B1 a package's module build fails: the shell build fails, naming it and
//     the build's log (a base without one of its apps is not shipped);
//  B2 a stale cached module is served for a changed tree: the cache key is
//     the digest of the package tree, the build scripts, the SDK lock inputs
//     and the dependency lock, so any change rebuilds;
//  B3 the dev server and the build disagree: both answer from the same build
//     results (the dev server serves the bytes at the same URL);
//  B4 a surface-only package (no `app`) has no window to mount: it is not a
//     module here; shell-slots.ts registers its surfaces.
import { spawn } from 'node:child_process';
import { createHash } from 'node:crypto';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

export const VIRTUAL_ID = 'virtual:choir-base-packages';
const RESOLVED = `\0${VIRTUAL_ID}`;
export const MODULE_DIR = 'assets/packages';
const FRONTEND_ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');

const sha256 = (bytes) => createHash('sha256').update(bytes).digest('hex');

function run(cmd, args, cwd) {
  return new Promise((resolve) => {
    const child = spawn(cmd, args, { cwd, stdio: ['ignore', 'pipe', 'pipe'] });
    let log = '';
    child.stdout.on('data', (d) => { log += d; });
    child.stderr.on('data', (d) => { log += d; });
    child.on('error', (err) => resolve({ status: 1, log: `${log}${err.message}` }));
    child.on('close', (status) => resolve({ status, log }));
  });
}

/** The build's inputs beside the package tree: the script, the boundary plugin and its data. */
function scriptFiles(frontendRoot) {
  const files = {
    'build-module.mjs': path.join(frontendRoot, 'scripts/build-module.mjs'),
    'package-boundary-plugin.mjs': path.join(frontendRoot, 'scripts/package-boundary-plugin.mjs'),
    'css-fetch-functions.json': path.join(frontendRoot, 'scripts/css-fetch-functions.json'),
  };
  return files;
}

function treeDigest(dir) {
  const h = createHash('sha256');
  const walk = (d, rel) => {
    for (const e of fs.readdirSync(d, { withFileTypes: true }).sort((a, b) => a.name.localeCompare(b.name))) {
      const full = path.join(d, e.name);
      const r = rel ? `${rel}/${e.name}` : e.name;
      if (e.isDirectory()) walk(full, r);
      else if (e.isFile()) h.update(`${r}\0${sha256(fs.readFileSync(full))}\n`);
    }
  };
  walk(dir, '');
  return h.digest('hex');
}

/** The app packages of this tree: [{ name, dir, manifest }] (B4: surface-only packages are left out). */
export function basePackages(frontendRoot = FRONTEND_ROOT) {
  const dir = path.join(frontendRoot, 'src/packages');
  if (!fs.existsSync(dir)) return [];
  const out = [];
  for (const name of fs.readdirSync(dir).sort()) {
    const manifestFile = path.join(dir, name, 'choir-package.json');
    if (!fs.existsSync(manifestFile)) continue;
    let manifest;
    try {
      manifest = JSON.parse(fs.readFileSync(manifestFile, 'utf8'));
    } catch {
      continue; // check-packages refuses it in the build; the shell skips it
    }
    if (manifest && typeof manifest.app === 'object' && manifest.app) out.push({ name, dir: path.join(dir, name), manifest });
  }
  return out;
}

/**
 * buildBaseModule builds one package as the computer does (a private frontend
 * root: the package tree, package.json, node_modules, the scripts) and returns
 * { name, requires_sdk, sha256, bytes }. A cached result is reused when every
 * input is the same (B2).
 */
export async function buildBaseModule({ name, dir, manifest }, { frontendRoot = FRONTEND_ROOT, cacheDir } = {}) {
  const requiresSdk = manifest?.app?.requires_sdk || '^2.0';
  const scripts = scriptFiles(frontendRoot);
  const key = createHash('sha256');
  key.update(`${name}\0${requiresSdk}\0${treeDigest(dir)}\n`);
  for (const [n, f] of Object.entries(scripts)) key.update(`${n}\0${sha256(fs.readFileSync(f))}\n`);
  for (const lock of ['package.json', 'package-lock.json']) {
    const f = path.join(frontendRoot, lock);
    if (fs.existsSync(f)) key.update(`${lock}\0${sha256(fs.readFileSync(f))}\n`);
  }
  const cacheFile = cacheDir ? path.join(cacheDir, `${name}-${key.digest('hex')}.js`) : '';
  if (cacheFile && fs.existsSync(cacheFile)) {
    const bytes = fs.readFileSync(cacheFile);
    return { name, requires_sdk: requiresSdk, sha256: sha256(bytes), bytes };
  }
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'choir-base-module-'));
  try {
    const front = path.join(tmp, 'frontend');
    fs.mkdirSync(path.join(front, 'src/packages'), { recursive: true });
    fs.cpSync(dir, path.join(front, 'src/packages', name), { recursive: true, dereference: false });
    for (const [n, f] of Object.entries(scripts)) fs.copyFileSync(f, path.join(front, n));
    fs.copyFileSync(path.join(frontendRoot, 'package.json'), path.join(front, 'package.json'));
    fs.symlinkSync(path.join(frontendRoot, 'node_modules'), path.join(front, 'node_modules'));
    const out = path.join(tmp, 'module.js');
    const r = await run(process.execPath, [path.join(front, 'build-module.mjs'), front, name, out, requiresSdk], front);
    if (r.status !== 0) {
      throw new Error(`the base package ${name} did not build as a module/v1 module (B1):\n${r.log.trim().slice(-4000)}`);
    }
    const bytes = fs.readFileSync(out);
    if (cacheFile) {
      // The cache is best-effort: in a capsule, node_modules is a link to the
      // read-only locked dependency tree, and an unwritable cache must not
      // fail the build (every capsule shot failed on it, 2026-11-08).
      // The rename is atomic, so a failed write never leaves a partial file
      // under the input-keyed name (Devin r1).
      const tmpFile = `${cacheFile}.tmp-${process.pid}`;
      try {
        fs.mkdirSync(cacheDir, { recursive: true });
        fs.writeFileSync(tmpFile, bytes);
        fs.renameSync(tmpFile, cacheFile);
      } catch {
        fs.rmSync(tmpFile, { force: true });
      }
    }
    return { name, requires_sdk: requiresSdk, sha256: sha256(bytes), bytes };
  } finally {
    fs.rmSync(tmp, { recursive: true, force: true });
  }
}

/**
 * buildBaseModules builds every app package, a few at a time. `tolerant` (the
 * dev server) leaves a package that fails out with a console note, so one
 * broken package being edited never takes the dev shell down; the build is
 * never tolerant (B1).
 */
export async function buildBaseModules({ frontendRoot = FRONTEND_ROOT, cacheDir, tolerant = false } = {}) {
  const pkgs = basePackages(frontendRoot);
  const width = Math.max(1, Math.min(6, Math.floor(os.cpus().length / 2)));
  const results = new Array(pkgs.length);
  let next = 0;
  // Each build is its own node process, so a few run side by side.
  const worker = async () => {
    while (next < pkgs.length) {
      const i = next++;
      try {
        results[i] = await buildBaseModule(pkgs[i], { frontendRoot, cacheDir });
      } catch (err) {
        if (!tolerant) throw err;
        console.warn(`[base-modules] ${pkgs[i].name} left out: ${String(err.message).split('\n')[0]}`);
      }
    }
  };
  await Promise.all(Array.from({ length: width }, worker));
  return results.filter(Boolean);
}

/** The Vite plugin: builds the base modules, emits them, and answers the virtual module. */
export default function baseModules({ root = FRONTEND_ROOT } = {}) {
  let built = null;
  let command = 'build';
  const cacheDir = path.join(root, 'node_modules/.cache/choir-base-modules');
  const ensure = () => {
    if (!built) built = buildBaseModules({ frontendRoot: root, cacheDir, tolerant: command === 'serve' });
    return built;
  };
  return {
    name: 'choir-base-modules',
    configResolved(config) {
      command = config.command;
    },
    resolveId(id) {
      return id === VIRTUAL_ID ? RESOLVED : null;
    },
    async load(id) {
      if (id !== RESOLVED) return null;
      const modules = await ensure();
      if (command === 'build') {
        for (const m of modules) this.emitFile({ type: 'asset', fileName: `${MODULE_DIR}/${m.sha256}.js`, source: m.bytes });
      }
      const list = modules.map((m) => ({ name: m.name, requires_sdk: m.requires_sdk, sha256: m.sha256, url: `/${MODULE_DIR}/${m.sha256}.js` }));
      return `export default ${JSON.stringify(list)};\n`;
    },
    configureServer(server) {
      // A package added, edited or removed: build again and reload the shell.
      const packagesDir = path.join(root, 'src/packages') + path.sep;
      let timer = null;
      server.watcher.on('all', (_event, file) => {
        if (!String(file).startsWith(packagesDir)) return;
        clearTimeout(timer);
        timer = setTimeout(() => {
          built = null;
          const mod = server.moduleGraph.getModuleById(RESOLVED);
          if (mod) server.moduleGraph.invalidateModule(mod);
          server.ws.send({ type: 'full-reload' });
        }, 200);
      });
      // B3: the dev server serves the same bytes at the same URL.
      server.middlewares.use(async (req, res, next) => {
        const m = /^\/assets\/packages\/([0-9a-f]{64})\.js$/.exec((req.url || '').split('?')[0]);
        if (!m) return next();
        const hit = (await ensure()).find((x) => x.sha256 === m[1]);
        if (!hit) return next();
        res.setHeader('Content-Type', 'text/javascript');
        res.end(hit.bytes);
      });
    },
  };
}
