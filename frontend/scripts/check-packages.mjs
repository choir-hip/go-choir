#!/usr/bin/env node
// Refuses a bad first-party app package at build time
// (docs/design/app-packages-replan-2026-10-24.md §3.1 and §3.3): manifest
// shape, name/alias/extension clashes, directory rules and the required floor.
// Import confinement is not here; see scripts/package-boundary-plugin.mjs.
//
// Run by `npm run build`. `--root <frontend dir>` points it at a fixture tree
// (tests/app-packages.spec.js does this to watch each bad case be refused).
import { existsSync, lstatSync, readdirSync, readFileSync, realpathSync } from 'node:fs';
import { dirname, join, resolve, sep } from 'node:path';
import { fileURLToPath } from 'node:url';
import { extractUses, usesProblems } from './package-uses.mjs';

const argv = process.argv.slice(2);
const rootFlag = argv.indexOf('--root');
const frontendRoot = resolve(
  rootFlag >= 0 ? argv[rootFlag + 1] : join(dirname(fileURLToPath(import.meta.url)), '..'),
);
const packagesDir = join(frontendRoot, 'src/packages');

const SCHEMA = 'choir.package/v2';
const ABI = 'choir.app-frontend/svelte-inline/v1';
const NAME_RE = /^[a-z][a-z0-9-]{1,39}$/;
const EXT_RE = /^[a-z0-9]{1,16}$/;
const RESERVED_NAMES = new Set([
  'desk', 'desktop', 'shell', 'prompt', 'system', 'owner', 'management', 'engineering', 'research',
]);
// Facts only the platform sets (internal/apppackage/strict.go publisherDeclared):
// a manifest that states one is refused apart from an unknown key. The Go
// validator (apppackage.ParseManifest, run over every in-tree manifest by
// TestInTreeManifestsParseAndHash) is the full check; this script is the
// build-time shape gate.
const PUBLISHER_DECLARED = new Set([
  'origin', 'kind', 'class', 'level', 'tier', 'publisher', 'owner', 'owner_id', 'track', 'trust',
  'review', 'reviewed', 'approved', 'behaviour_surface', 'behavior_surface', 'hash', 'digest',
  'sha256', 'source_digest', 'artifact', 'artifact_digest', 'toolchain', 'binary', 'wasm',
  'import_set', 'imports', 'first_party',
]);
const GRANT_RE = /^((ui|desk)(\.[a-z][a-z0-9_]*)+|agent\.prompt)$/;
const MIN_STATIC_IDS = 10;
// First-party packages that must exist: deleting one (or filtering it out of a
// build) fails the build instead of shipping a shell without the app.
const REQUIRED_PACKAGES = ['image', 'files'];

const errors = [];
const fail = (where, message) => errors.push(`${where}: ${message}`);

const isObject = (v) => v !== null && typeof v === 'object' && !Array.isArray(v);

function strictKeys(obj, where, required, optional = []) {
  if (!isObject(obj)) {
    fail(where, 'must be an object');
    return false;
  }
  const allowed = new Set([...required, ...optional]);
  for (const key of Object.keys(obj)) {
    if (allowed.has(key)) continue;
    if (PUBLISHER_DECLARED.has(key.toLowerCase())) fail(where, `"${key}" is a platform fact; a manifest cannot state it`);
    else fail(where, `unknown field "${key}"`);
  }
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
  // The one file-route table is data (src/lib/file-routes.json), read as data.
  const exts = new Set();
  let table = null;
  try {
    table = JSON.parse(readFileSync(join(frontendRoot, 'src/lib/file-routes.json'), 'utf8'));
  } catch (err) {
    fail('file-routes.json', `cannot be read: ${err.message}`);
  }
  for (const [i, route] of (Array.isArray(table?.routes) ? table.routes : []).entries()) {
    const where = `file-routes.json routes[${i}]`;
    if (!route || typeof route.appId !== 'string' || typeof route.mediaType !== 'string' || !route.appId || !route.mediaType) {
      fail(where, 'needs a non-empty appId and mediaType');
      continue;
    }
    if (!Array.isArray(route.extensions) || route.extensions.length === 0 || !route.extensions.every((e) => /^[a-z0-9]+$/.test(e))) {
      fail(where, 'extensions must be a non-empty array of lowercase extensions without a dot');
      continue;
    }
    if (route.default !== undefined && typeof route.default !== 'boolean') fail(where, 'default must be a boolean');
    if (route.default !== false) route.extensions.forEach((e) => exts.add(e));
  }
  if (ids.size < MIN_STATIC_IDS) {
    fail('registry.ts', `parsed ${ids.size} static app ids (floor ${MIN_STATIC_IDS}); the layout changed, so the clash check cannot be trusted`);
  }
  if (exts.size === 0) fail('file-routes.json', 'holds no click-to-open routes; the clash check cannot be trusted');
  return { ids, exts };
}

// ---- contributes.commands and contributes.keybindings (services/commands.ts has the runtime rules) ----

const COMMAND_ID = /^[a-z][a-z0-9-]*\.[A-Za-z][A-Za-z0-9._-]*$/;
const COMMAND_SCOPE = /^(global|app|target:[a-z][a-z0-9_-]*)$/;
const COMMAND_ACTIONS = new Set(['openApp', 'openFile', 'copyText', 'setTheme', 'toast']);
const LOCKED_KEYS = new Set(['Mod+K']);

function normalizeKey(key) {
  const mods = new Set();
  let main = '';
  for (const part of String(key).split('+').map((p) => p.trim()).filter(Boolean)) {
    const lower = part.toLowerCase();
    if (['mod', 'cmd', 'ctrl', 'meta'].includes(lower)) mods.add('Mod');
    else if (['alt', 'option'].includes(lower)) mods.add('Alt');
    else if (lower === 'shift') mods.add('Shift');
    else main = part.length === 1 ? part.toUpperCase() : part[0].toUpperCase() + part.slice(1);
  }
  return main ? [...['Mod', 'Alt', 'Shift'].filter((m) => mods.has(m)), main].join('+') : '';
}

// contributes.links: the rules of services/links.ts (L1 to L6), as data.
const LINK_NAME = /^[a-z][a-z0-9_]{0,31}$/;
const LINK_CONTEXT_KEY = /^[a-z][A-Za-z0-9_]{0,31}$/;
const LINK_RESERVED = new Set(['app', 'title', 'doc', 'doc_id']);
function checkLinks(contributes, owner, where) {
  if (contributes.links === undefined) return;
  if (!Array.isArray(contributes.links)) {
    fail(where, 'links must be an array');
    return;
  }
  const apps = new Set();
  contributes.links.forEach((link, i) => {
    const at = `${where} links[${i}]`;
    if (!strictKeys(link, at, ['app', 'params'], ['launch'])) return;
    if (link.app !== owner) fail(at, `app must be the package's own ("${owner}")`);
    if (apps.has(link.app)) fail(at, `app "${link.app}" is listed twice`);
    apps.add(link.app);
    const names = new Set();
    if (!Array.isArray(link.params) || link.params.length === 0) fail(at, 'params must list at least one parameter');
    (Array.isArray(link.params) ? link.params : []).forEach((p, j) => {
      if (!strictKeys(p, `${at} params[${j}]`, ['name'], ['required', 'context'])) return;
      if (typeof p.name !== 'string' || !LINK_NAME.test(p.name)) fail(`${at} params[${j}]`, 'name must be a lowercase word');
      else if (LINK_RESERVED.has(p.name)) fail(`${at} params[${j}]`, `"${p.name}" is a parameter the shell owns`);
      else if (names.has(p.name)) fail(`${at} params[${j}]`, `"${p.name}" is listed twice`);
      else names.add(p.name);
      if (p.required !== undefined && typeof p.required !== 'boolean') fail(`${at} params[${j}]`, 'required must be a boolean');
      if (p.context !== undefined && !(typeof p.context === 'string' && LINK_CONTEXT_KEY.test(p.context))) fail(`${at} params[${j}]`, 'context must be a key word');
    });
    if (link.launch === undefined) return;
    if (!strictKeys(link.launch, `${at} launch`, [], ['windowTitle', 'requireAny', 'context'])) return;
    const l = link.launch;
    if (l.windowTitle !== undefined && !str(l.windowTitle)) fail(`${at} launch`, 'windowTitle must be a non-empty string');
    if (l.requireAny !== undefined && !(Array.isArray(l.requireAny) && l.requireAny.every((n) => names.has(n)))) fail(`${at} launch`, 'requireAny must name declared parameters');
    if (l.context !== undefined) {
      if (!isObject(l.context)) fail(`${at} launch`, 'context must be an object');
      else {
        for (const [key, value] of Object.entries(l.context)) {
          if (!LINK_CONTEXT_KEY.test(key)) fail(`${at} launch`, `context key "${key}" is not a key word`);
          if (typeof value === 'string' && value.startsWith('param:')) {
            if (!names.has(value.slice(6))) fail(`${at} launch`, `context ${key} reads parameter "${value.slice(6)}", which is not declared`);
          } else if (!['string', 'number', 'boolean'].includes(typeof value)) fail(`${at} launch`, `context ${key} must be a literal or "param:<name>"`);
        }
      }
    }
  });
}

// contributes.themes: { id, file } with the file in the package, JSON, schema 2, the same id.
// The token, safe-value and contrast rules run when the theme registers (theme.ts) and in tests/theme-contract.spec.js.
function checkThemes(contributes, dir, where) {
  if (contributes.themes === undefined) return;
  if (!Array.isArray(contributes.themes)) {
    fail(where, 'themes must be an array');
    return;
  }
  const ids = new Set();
  contributes.themes.forEach((t, i) => {
    const at = `${where} themes[${i}]`;
    if (!strictKeys(t, at, ['id', 'file'])) return;
    if (typeof t.id !== 'string' || !/^[a-z][a-z0-9-]*(\.[a-z0-9-]+)*$/.test(t.id)) fail(at, 'id must be lowercase letters, digits and dashes');
    else if (ids.has(t.id)) fail(at, `id "${t.id}" is listed twice`);
    else ids.add(t.id);
    if (typeof t.file !== 'string' || !/^[A-Za-z0-9._-]+(\/[A-Za-z0-9._-]+)*\.json$/.test(t.file) || t.file.split('/').includes('..')) {
      fail(at, 'file must be a relative .json path inside the package');
      return;
    }
    let theme;
    try {
      theme = JSON.parse(readFileSync(join(dir, t.file), 'utf8'));
    } catch (err) {
      fail(at, `file ${t.file} cannot be read as JSON: ${err.message}`);
      return;
    }
    if (!isObject(theme) || theme.id !== t.id) fail(at, `file ${t.file} must be a theme with id "${t.id}"`);
    else if (theme.schema_version !== 2) fail(at, `file ${t.file} must have schema_version 2`);
  });
}

function checkCommands(contributes, owner, where) {
  const ids = new Set();
  if (contributes.commands !== undefined && !Array.isArray(contributes.commands)) fail(where, 'commands must be an array');
  (Array.isArray(contributes.commands) ? contributes.commands : []).forEach((cmd, i) => {
    const at = `${where} commands[${i}]`;
    if (!strictKeys(cmd, at, ['id', 'title', 'scope', 'run'], ['when'])) return;
    if (typeof cmd.id !== 'string' || !COMMAND_ID.test(cmd.id)) fail(at, 'id must be <package>.<name>');
    else if (cmd.id.split('.')[0] !== owner) fail(at, `id "${cmd.id}" is outside the namespace of "${owner}"`);
    else if (ids.has(cmd.id)) fail(at, `id "${cmd.id}" is listed twice`);
    else ids.add(cmd.id);
    if (!str(cmd.title)) fail(at, 'title must be a non-empty string');
    if (typeof cmd.scope !== 'string' || !COMMAND_SCOPE.test(cmd.scope)) fail(at, 'scope must be global, app or target:<kind>');
    if (cmd.when !== undefined && typeof cmd.when !== 'string') fail(at, 'when must be a string');
    if (!isObject(cmd.run)) fail(at, 'run must be an action or { "handler": true }');
    else if (cmd.run.handler !== undefined) {
      if (cmd.run.handler !== true || Object.keys(cmd.run).length !== 1) fail(at, 'run.handler must be exactly { "handler": true }');
    } else if (!COMMAND_ACTIONS.has(cmd.run.action)) fail(at, `run.action must be one of ${[...COMMAND_ACTIONS].join(', ')}`);
  });
  if (contributes.keybindings !== undefined && !Array.isArray(contributes.keybindings)) fail(where, 'keybindings must be an array');
  (Array.isArray(contributes.keybindings) ? contributes.keybindings : []).forEach((kb, i) => {
    const at = `${where} keybindings[${i}]`;
    if (!strictKeys(kb, at, ['key', 'command'], ['when'])) return;
    const key = typeof kb.key === 'string' ? normalizeKey(kb.key) : '';
    if (!key) fail(at, 'key must name a key, for example "Mod+Shift+P"');
    else if (LOCKED_KEYS.has(key)) fail(at, `key ${key} is locked to the shell`);
    if (typeof kb.command !== 'string' || !kb.command.startsWith(`${owner}.`)) fail(at, `command must be one of the package's own (${owner}.<name>)`);
    else if (!ids.has(kb.command)) fail(at, `command "${kb.command}" is not in this package's commands`);
    if (kb.when !== undefined && typeof kb.when !== 'string') fail(at, 'when must be a string');
  });
}

// ---- imports ----
// Not checked here. What a package may import is enforced on the resolved
// module graph by scripts/package-boundary-plugin.mjs, run by vite build.

const inside = (dir, target) => target === dir || target.startsWith(dir + sep);

// ---- manifest ----

function checkManifest(manifest, dirName, where, pkgDir) {
  const ok = strictKeys(
    manifest, where,
    ['schema', 'name', 'version', 'display', 'app', 'opens'],
    ['kinds', 'uses_kinds', 'contributes', 'capabilities'],
  );
  if (!ok) return;
  if (manifest.schema !== SCHEMA) fail(where, `schema must be "${SCHEMA}"`);
  if (typeof manifest.name !== 'string' || !NAME_RE.test(manifest.name)) fail(where, `name must match ${NAME_RE}`);
  else if (manifest.name !== dirName) fail(where, `name "${manifest.name}" must equal its directory "${dirName}"`);
  if (typeof manifest.version !== 'string' || !/^\d+\.\d+\.\d+$/.test(manifest.version)) fail(where, 'version must be major.minor.patch');

  if (strictKeys(manifest.display, `${where} display`, ['title', 'icon', 'description'])) {
    for (const k of ['title', 'icon', 'description']) if (!str(manifest.display[k])) fail(`${where} display`, `"${k}" must be a non-empty string`);
  }

  const app = manifest.app;
  if (strictKeys(app, `${where} app`, ['entry', 'abi', 'launcher', 'window', 'auth', 'theme'], ['aliases', 'requires_sdk'])) {
    if (app.requires_sdk !== undefined && (typeof app.requires_sdk !== 'string' || !/^\^(0|[1-9]\d{0,3})\.(0|[1-9]\d{0,3})$/.test(app.requires_sdk))) {
      fail(`${where} app`, 'requires_sdk must be "^MAJOR.MINOR"');
    }
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
      // default: false offers the package under Open with only; the extension
      // keeps its default opener, so it may be one another app routes.
      if (!strictKeys(o, `${where} opens[${i}]`, ['media_type', 'extensions'], ['default'])) return;
      if (o.default !== undefined && typeof o.default !== 'boolean') fail(`${where} opens[${i}]`, 'default must be a boolean');
      if (!str(o.media_type)) fail(`${where} opens[${i}]`, 'media_type must be a non-empty string');
      if (!Array.isArray(o.extensions) || o.extensions.length === 0 || o.extensions.some((e) => typeof e !== 'string' || !EXT_RE.test(e))) {
        fail(`${where} opens[${i}]`, 'extensions must be a non-empty array of lowercase extensions without a dot');
      }
    });
  }

  // Kinds are generated into Go (gen-package-kinds.mjs); the other lists carry no code yet.
  if (Array.isArray(manifest.kinds) ? manifest.kinds.length > 0 : manifest.kinds !== undefined) fail(where, 'kinds must be empty in P1 (package-declared kinds land with P2)');
  if (manifest.uses_kinds !== undefined && !(Array.isArray(manifest.uses_kinds) && manifest.uses_kinds.length === 0)) fail(where, 'uses_kinds must be empty in P1');
  // Desks arrive first-party in MA4.5 (contributes.desks is decoded by the Go validator now).
  if (manifest.contributes !== undefined && strictKeys(manifest.contributes, `${where} contributes`, [], ['desks', 'commands', 'keybindings', 'themes', 'links'])) {
    const c = manifest.contributes;
    if (c.desks !== undefined && (!Array.isArray(c.desks) || c.desks.length > 0)) fail(`${where} contributes`, 'desks must be an empty list until first-party desks register (MA4.5)');
    checkCommands(c, manifest.name, `${where} contributes`);
    checkThemes(c, pkgDir, `${where} contributes`);
    checkLinks(c, manifest.name, `${where} contributes`);
  }
  if (manifest.capabilities !== undefined) {
    if (!Array.isArray(manifest.capabilities)) fail(where, 'capabilities must be an array');
    else {
      manifest.capabilities.forEach((c, i) => {
        if (!strictKeys(c, `${where} capabilities[${i}]`, ['grant'], ['args'])) return;
        if (typeof c.grant !== 'string' || !GRANT_RE.test(c.grant)) fail(`${where} capabilities[${i}]`, 'grant must be a ui.* or desk.* name, or agent.prompt');
        if (c.args !== undefined && (!Array.isArray(c.args) || c.args.some((a) => typeof a !== 'string'))) fail(`${where} capabilities[${i}]`, 'args must be an array of strings');
      });
    }
  }
}

// ---- run ----

function main() {
  const { ids: staticIds, exts: staticExts } = staticFacts();
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
    checkManifest(manifest, dir, where, pkgDir);
    // `uses` is extracted, never authored: the grants the SDK names a package imports imply must be declared.
    try {
      for (const problem of usesProblems(extractUses(pkgDir), manifest)) fail(where, problem);
    } catch (err) {
      fail(where, err.message);
    }
    if (!existsSync(join(pkgDir, 'App.svelte'))) fail(`src/packages/${dir}`, 'missing App.svelte');
    if (errors.length > before && !(typeof manifest?.name === 'string' && NAME_RE.test(manifest.name))) continue;

    // name, alias and extension clashes
    const name = typeof manifest?.name === 'string' ? manifest.name : dir;
    const aliases = Array.isArray(manifest?.app?.aliases) ? manifest.app.aliases.filter((a) => typeof a === 'string') : [];
    if (new Set([name, ...aliases]).size !== aliases.length + 1) fail(where, 'name and aliases must be distinct');
    for (const id of new Set([name, ...aliases])) claim(id, name, where);
    const ownExts = new Set();
    for (const o of Array.isArray(manifest?.opens) ? manifest.opens : []) {
      for (const e of Array.isArray(o?.extensions) ? o.extensions : []) {
        if (o?.default === false) {
          // Open with only: no clash with other apps, but not twice in one package.
          if (ownExts.has(e)) fail(where, `extension "${e}" is listed twice`);
          ownExts.add(e);
          continue;
        }
        if (ownExts.has(e)) fail(where, `extension "${e}" is listed twice`);
        ownExts.add(e);
        if (staticExts.has(e)) fail(where, `extension "${e}" is still routed by the static file-routes.json (delete that row in the same change)`);
        else if (claimedExts.has(e) && claimedExts.get(e) !== name) fail(where, `extension "${e}" is already opened by package "${claimedExts.get(e)}"`);
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
