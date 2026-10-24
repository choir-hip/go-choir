#!/usr/bin/env node
// Refuses a bad first-party app package at build time
// (docs/design/app-packages-replan-2026-10-24.md §3.1 and §3.3).
//
// Run by `npm run build`. `--root <frontend dir>` points it at a fixture tree
// (tests/app-packages.spec.js does this to watch each bad case be refused).
import { existsSync, lstatSync, readdirSync, readFileSync, realpathSync } from 'node:fs';
import { dirname, join, relative, resolve, sep } from 'node:path';
import { fileURLToPath } from 'node:url';

const argv = process.argv.slice(2);
const rootFlag = argv.indexOf('--root');
const frontendRoot = resolve(
  rootFlag >= 0 ? argv[rootFlag + 1] : join(dirname(fileURLToPath(import.meta.url)), '..'),
);
const packagesDir = join(frontendRoot, 'src/packages');
const sdkBase = join(frontendRoot, 'src/lib/app-sdk');

const SCHEMA = 'choir.app-package/v1';
const ABI = 'choir.app-frontend/svelte-inline/v1';
const NAME_RE = /^[a-z][a-z0-9-]{1,39}$/;
const EXT_RE = /^[a-z0-9]{1,16}$/;
const RESERVED_NAMES = new Set([
  'desk', 'desktop', 'shell', 'prompt', 'system', 'owner', 'management', 'engineering', 'research',
]);
const MIN_STATIC_IDS = 10;
// First-party packages that must exist: deleting one (or filtering it out of a
// build) fails the build instead of shipping a shell without the app.
const REQUIRED_PACKAGES = ['image'];
// Every file in a package is scanned for imports except known binary assets,
// so a new extension (.tsx, .jsx, .mts, .cts, ...) cannot hide an import.
const BINARY_EXT = new Set([
  '.png', '.jpg', '.jpeg', '.gif', '.webp', '.avif', '.bmp', '.ico',
  '.woff', '.woff2', '.ttf', '.otf', '.mp3', '.mp4', '.webm', '.wav', '.ogg', '.pdf', '.json',
]);

const errors = [];
const fail = (where, message) => errors.push(`${where}: ${message}`);

const isObject = (v) => v !== null && typeof v === 'object' && !Array.isArray(v);

function strictKeys(obj, where, required, optional = []) {
  if (!isObject(obj)) {
    fail(where, 'must be an object');
    return false;
  }
  const allowed = new Set([...required, ...optional]);
  for (const key of Object.keys(obj)) if (!allowed.has(key)) fail(where, `unknown field "${key}"`);
  for (const key of required) if (!(key in obj)) fail(where, `missing field "${key}"`);
  return true;
}

const str = (v) => typeof v === 'string' && v.trim() !== '';

function geometry(obj, where) {
  if (obj === undefined) return;
  if (!strictKeys(obj, where, [], ['width', 'height', 'minWidth', 'minHeight'])) return;
  for (const [k, v] of Object.entries(obj)) {
    if (!Number.isFinite(v) || v <= 0) fail(where, `"${k}" must be a positive number`);
  }
}

// ---- static registry facts (ids and file routes the packages must not take) ----

function staticFacts() {
  const registry = readFileSync(join(frontendRoot, 'src/lib/apps/registry.ts'), 'utf8');
  const start = registry.indexOf('const STATIC_APPS = [');
  const end = registry.indexOf('] satisfies ChoirAppDefinition[]');
  const block = start >= 0 && end > start ? registry.slice(start, end) : '';
  const ids = new Set();
  for (const m of block.matchAll(/^ {4}id: (?:'([a-z0-9-]+)'|TEXTURE_APP_ID),/gm)) {
    ids.add(m[1] || 'texture');
  }
  const media = readFileSync(join(frontendRoot, 'src/lib/media-utils.js'), 'utf8');
  const rStart = media.indexOf('export const MEDIA_FILE_ROUTES = [');
  const rEnd = rStart >= 0 ? media.indexOf('\n];', rStart) : -1;
  const routeBlock = rEnd > rStart ? media.slice(rStart, rEnd) : '';
  const exts = new Set();
  for (const m of routeBlock.matchAll(/extensions:\s*\[([^\]]*)\]/g)) {
    for (const e of m[1].matchAll(/'([a-z0-9]+)'/g)) exts.add(e[1]);
  }
  if (ids.size < MIN_STATIC_IDS) {
    fail('registry.ts', `parsed ${ids.size} static app ids (floor ${MIN_STATIC_IDS}); the layout changed, so the clash check cannot be trusted`);
  }
  if (exts.size === 0) fail('media-utils.js', 'parsed no static file routes; the clash check cannot be trusted');
  return { ids, exts };
}

// ---- imports ----

function importSpecifiers(source) {
  const specs = [];
  const patterns = [
    /\b(?:import|export)\s+(?:[\w*${}\s,]+?\s+from\s+)?['"]([^'"]+)['"]/g,
    /\bimport\s*\(\s*['"]([^'"]+)['"]\s*\)/g,
    /@import\s+(?:url\()?\s*['"]([^'"]+)['"]/g,
  ];
  for (const re of patterns) for (const m of source.matchAll(re)) specs.push(m[1]);
  return specs;
}

// import.meta.glob/resolve reach files by pattern and bypass the specifier
// scan; a dynamic import whose argument is not a quoted literal cannot be
// resolved at check time. Both are refused outright.
function checkDynamicReach(source, where) {
  if (/\bimport\s*\.\s*meta\s*\.\s*(glob|globEager|resolve)\b/.test(source)) {
    fail(where, 'import.meta.glob/resolve is not allowed in a package');
  }
  const calls = (source.match(/\bimport\s*\(/g) || []).length;
  const literal = (source.match(/\bimport\s*\(\s*(?:'[^'\n]*'|"[^"\n]*")\s*\)/g) || []).length;
  if (calls !== literal) fail(where, 'import() must take a quoted string literal');
}

function listFiles(dir, out = []) {
  for (const name of readdirSync(dir)) {
    const full = join(dir, name);
    const st = lstatSync(full);
    if (st.isSymbolicLink()) fail(relative(frontendRoot, full), 'symlinks are not allowed inside a package');
    else if (st.isDirectory()) listFiles(full, out);
    else out.push(full);
  }
  return out;
}

const inside = (dir, target) => target === dir || target.startsWith(dir + sep);
const stripExt = (p) => p.replace(/\.(ts|js|mjs)$/, '');

function checkImports(pkgDir, declaredDeps) {
  for (const file of listFiles(pkgDir)) {
    const dot = file.lastIndexOf('.');
    if (dot >= 0 && BINARY_EXT.has(file.slice(dot).toLowerCase())) continue;
    const where = relative(frontendRoot, file);
    const source = readFileSync(file, 'utf8');
    checkDynamicReach(source, where);
    for (const spec of importSpecifiers(source)) {
      if (spec.startsWith('.')) {
        const target = resolve(dirname(file), spec);
        if (inside(pkgDir, target)) continue;
        if (stripExt(target) === sdkBase) continue;
        fail(where, `import "${spec}" leaves the package (only src/lib/app-sdk.ts is allowed)`);
      } else if (spec.startsWith('/') || /^[a-z]+:/i.test(spec) && !spec.startsWith('@')) {
        fail(where, `import "${spec}" is an absolute or URL import`);
      } else {
        const bare = spec.startsWith('@') ? spec.split('/').slice(0, 2).join('/') : spec.split('/')[0];
        if (bare === 'svelte' || declaredDeps.has(bare)) continue;
        fail(where, `import "${spec}" is not svelte, a declared dependency, or app-sdk.ts`);
      }
    }
  }
}

// ---- manifest ----

function checkManifest(manifest, dirName, where) {
  const ok = strictKeys(
    manifest, where,
    ['schema', 'name', 'version', 'origin', 'display', 'app', 'opens'],
    ['kinds', 'uses_kinds', 'desk', 'backend', 'grants'],
  );
  if (!ok) return;
  if (manifest.schema !== SCHEMA) fail(where, `schema must be "${SCHEMA}"`);
  if (typeof manifest.name !== 'string' || !NAME_RE.test(manifest.name)) fail(where, `name must match ${NAME_RE}`);
  else if (manifest.name !== dirName) fail(where, `name "${manifest.name}" must equal its directory "${dirName}"`);
  if (typeof manifest.version !== 'string' || !/^\d+\.\d+\.\d+$/.test(manifest.version)) fail(where, 'version must be major.minor.patch');
  if (manifest.origin !== 'first_party') fail(where, 'origin must be "first_party" (installed packages are a later slice)');

  if (strictKeys(manifest.display, `${where} display`, ['title', 'icon', 'description'])) {
    for (const k of ['title', 'icon', 'description']) if (!str(manifest.display[k])) fail(`${where} display`, `"${k}" must be a non-empty string`);
  }

  const app = manifest.app;
  if (strictKeys(app, `${where} app`, ['entry', 'abi', 'launcher', 'window', 'auth', 'theme'], ['aliases'])) {
    if (app.entry !== 'App.svelte') fail(`${where} app`, 'entry must be "App.svelte" inside the package');
    if (app.abi !== ABI) fail(`${where} app`, `abi must be "${ABI}"`);
    if (app.aliases !== undefined && !Array.isArray(app.aliases)) fail(`${where} app`, 'aliases must be an array');
    for (const a of Array.isArray(app.aliases) ? app.aliases : []) {
      if (typeof a !== 'string' || !NAME_RE.test(a)) fail(`${where} app`, `alias ${JSON.stringify(a)} must match ${NAME_RE}`);
    }
    if (strictKeys(app.launcher, `${where} launcher`, ['menu', 'desktopIcon', 'order'])) {
      if (typeof app.launcher.menu !== 'boolean') fail(`${where} launcher`, '"menu" must be boolean');
      if (typeof app.launcher.desktopIcon !== 'boolean') fail(`${where} launcher`, '"desktopIcon" must be boolean');
      if (!Number.isFinite(app.launcher.order)) fail(`${where} launcher`, '"order" must be a number');
    }
    if (strictKeys(app.window, `${where} window`, ['singleton', 'heavy'], ['desktop', 'compact'])) {
      if (typeof app.window.singleton !== 'boolean') fail(`${where} window`, '"singleton" must be boolean');
      if (typeof app.window.heavy !== 'boolean') fail(`${where} window`, '"heavy" must be boolean');
      geometry(app.window.desktop, `${where} window.desktop`);
      geometry(app.window.compact, `${where} window.compact`);
    }
    if (strictKeys(app.auth, `${where} auth`, ['preview'])) {
      if (!['public-preview', 'public-readonly', 'private'].includes(app.auth.preview)) fail(`${where} auth`, 'preview must be public-preview, public-readonly or private');
    }
    if (strictKeys(app.theme, `${where} theme`, ['surface', 'shellDataAttr', 'contentClass'])) {
      if (!['standard', 'document', 'media'].includes(app.theme.surface)) fail(`${where} theme`, 'surface must be standard, document or media');
      if (typeof app.theme.shellDataAttr !== 'string' || !/^data-[a-z0-9-]+$/.test(app.theme.shellDataAttr)) fail(`${where} theme`, 'shellDataAttr must be a data-* attribute name');
      if (typeof app.theme.contentClass !== 'string' || !/^[a-z][a-z0-9-]*$/.test(app.theme.contentClass)) fail(`${where} theme`, 'contentClass must be a css class name');
    }
  }

  if (!Array.isArray(manifest.opens)) fail(where, 'opens must be an array');
  else {
    manifest.opens.forEach((o, i) => {
      if (!strictKeys(o, `${where} opens[${i}]`, ['media_type', 'extensions'])) return;
      if (!str(o.media_type)) fail(`${where} opens[${i}]`, 'media_type must be a non-empty string');
      if (!Array.isArray(o.extensions) || o.extensions.length === 0 || o.extensions.some((e) => typeof e !== 'string' || !EXT_RE.test(e))) {
        fail(`${where} opens[${i}]`, 'extensions must be a non-empty array of lowercase extensions without a dot');
      }
    });
  }

  // P1 refuses what later slices own (P2: kinds, P4: desk, P5: backend and grants).
  if (Array.isArray(manifest.kinds) ? manifest.kinds.length > 0 : manifest.kinds !== undefined) fail(where, 'kinds must be empty in P1 (package-declared kinds land with P2)');
  if (manifest.uses_kinds !== undefined && !(Array.isArray(manifest.uses_kinds) && manifest.uses_kinds.length === 0)) fail(where, 'uses_kinds must be empty in P1');
  if (manifest.desk !== undefined && manifest.desk !== null) fail(where, 'desk must be null in P1');
  if (manifest.backend !== undefined && manifest.backend !== null) fail(where, 'backend must be null in P1');
  if (manifest.grants !== undefined && !(Array.isArray(manifest.grants) && manifest.grants.length === 0)) fail(where, 'grants must be empty in P1');
}

// ---- run ----

function main() {
  const { ids: staticIds, exts: staticExts } = staticFacts();
  let declaredDeps = new Set();
  try {
    declaredDeps = new Set(Object.keys(JSON.parse(readFileSync(join(frontendRoot, 'package.json'), 'utf8')).dependencies || {}));
  } catch (e) {
    fail('package.json', `unreadable: ${e.message}`);
  }

  // Every entry under src/packages must be a real directory that stays under it:
  // a symlink or stray file would be skipped here while Vite's glob can still match it.
  const dirs = [];
  if (existsSync(packagesDir)) {
    const realPackages = realpathSync(packagesDir);
    for (const n of readdirSync(packagesDir).sort()) {
      const full = join(packagesDir, n);
      if (!lstatSync(full).isDirectory() || !inside(realPackages, realpathSync(full))) {
        fail(`src/packages/${n}`, 'must be a real directory inside src/packages (no symlinks or stray files)');
        continue;
      }
      dirs.push(n);
    }
  }
  for (const required of REQUIRED_PACKAGES) {
    if (!dirs.includes(required)) fail('src/packages', `required package "${required}" is missing`);
  }

  // A source filter that drops choir-package.json (the Nix build once could) must fail loudly.
  if (existsSync(packagesDir) && dirs.length === 0) {
    fail('src/packages', 'the directory exists but holds no package; refusing a build that would ship none');
  }

  const claimedIds = new Map(); // id -> package that holds it
  const claimedExts = new Map(); // ext -> package that holds it
  const claim = (id, owner, where) => {
    if (staticIds.has(id)) fail(where, `"${id}" is a static registry id (delete the static entry in the same change to take it)`);
    else if (RESERVED_NAMES.has(id)) fail(where, `"${id}" is a reserved shell or system-desk name`);
    else if (claimedIds.has(id)) fail(where, `"${id}" is already held by package "${claimedIds.get(id)}"`);
    claimedIds.set(id, claimedIds.get(id) || owner);
  };

  for (const dir of dirs) {
    const pkgDir = join(packagesDir, dir);
    const where = `src/packages/${dir}/choir-package.json`;
    const manifestPath = join(pkgDir, 'choir-package.json');
    if (!existsSync(manifestPath)) {
      fail(`src/packages/${dir}`, 'missing choir-package.json');
      continue;
    }
    let manifest;
    try {
      manifest = JSON.parse(readFileSync(manifestPath, 'utf8'));
    } catch (e) {
      fail(where, `not valid JSON: ${e.message}`);
      continue;
    }
    const before = errors.length;
    checkManifest(manifest, dir, where);
    if (!existsSync(join(pkgDir, 'App.svelte'))) fail(`src/packages/${dir}`, 'missing App.svelte');
    checkImports(pkgDir, declaredDeps);
    if (errors.length > before && !(typeof manifest?.name === 'string' && NAME_RE.test(manifest.name))) continue;

    // name, alias and extension clashes
    const name = typeof manifest?.name === 'string' ? manifest.name : dir;
    const aliases = Array.isArray(manifest?.app?.aliases) ? manifest.app.aliases.filter((a) => typeof a === 'string') : [];
    if (new Set([name, ...aliases]).size !== aliases.length + 1) fail(where, 'name and aliases must be distinct');
    for (const id of new Set([name, ...aliases])) claim(id, name, where);
    for (const o of Array.isArray(manifest?.opens) ? manifest.opens : []) {
      for (const e of Array.isArray(o?.extensions) ? o.extensions : []) {
        if (staticExts.has(e)) fail(where, `extension "${e}" is still routed by the static MEDIA_FILE_ROUTES (delete that row in the same change)`);
        else if (claimedExts.has(e) && claimedExts.get(e) !== name) fail(where, `extension "${e}" is already opened by package "${claimedExts.get(e)}"`);
        else if (claimedExts.get(e) === name) fail(where, `extension "${e}" is listed twice`);
        claimedExts.set(e, claimedExts.get(e) || name);
      }
    }
  }

  if (errors.length) {
    console.error(`check-packages: ${errors.length} problem(s)`);
    for (const e of errors) console.error(`  - ${e}`);
    process.exit(1);
  }
  console.log(`check-packages: ${dirs.length} package(s) ok (${dirs.join(', ') || 'none'})`);
}

main();
