#!/usr/bin/env node
// The shell SDK surface and its lock (docs/design/modularity-check-build-2026-11-04.md).
//
// The surface is every name src/lib/app-sdk.ts exports, with its type as the
// TypeScript checker prints it, plus every local type those types reference
// (expanded, so a change inside `ChoirAppDefinition` is seen even though the
// SDK only returns it). It is locked per SDK_VERSION in
// src/lib/sdk-surface/<version>.json. A change needs a bump of the right kind:
//
//   major  a name leaves; a value's new type does not fit where the old one
//          was used; a type changes and is not equivalent to the old one
//   minor  a name is added; a value's type widens (the new type fits every
//          old use)
//   patch  nothing a package can see changed (types that differ only in how
//          the printer wrote them, checked equivalent both ways, need nothing)
//
// Usage:
//   node scripts/sdk-surface.mjs            check (exit 1 on a problem)
//   node scripts/sdk-surface.mjs --base REF also refuse any lock that existed
//                                           at merge-base(REF, HEAD) and was
//                                           changed, deleted or renamed (CI)
//   node scripts/sdk-surface.mjs --write    write the lock for SDK_VERSION,
//                                           only when the bump is enough
//   node scripts/sdk-surface.mjs --json     print the surface
import { spawnSync } from 'node:child_process';
import { createHash } from 'node:crypto';
import { existsSync, mkdirSync, readdirSync, readFileSync, realpathSync, writeFileSync } from 'node:fs';
import path from 'node:path';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';

export const SURFACE_SCHEMA = 'choir.sdk-surface/v1';
const DEFAULT_ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const SDK_REL = 'src/lib/app-sdk.ts';
const LOCK_REL = 'src/lib/sdk-surface';
const MIN_NAMES = 30; // L6: a broken extractor must not produce an empty surface that "matches"
const MIN_TYPES = 1; // L6: nor lose every referenced type (a changed import("...") print)
const RANK = { none: 0, patch: 1, minor: 2, major: 3 };

function loadTs(frontendRoot) {
  return createRequire(path.join(frontendRoot, 'package.json'))('typescript');
}

const compilerOptions = (ts) => ({
  allowJs: true,
  checkJs: false,
  noEmit: true,
  strict: true,
  skipLibCheck: true,
  target: ts.ScriptTarget.ES2022,
  module: ts.ModuleKind.ESNext,
  moduleResolution: ts.ModuleResolutionKind.Bundler,
  allowImportingTsExtensions: true,
  types: [],
});

const FLAGS = (ts) => ts.TypeFormatFlags.NoTruncation | ts.TypeFormatFlags.UseFullyQualifiedType;

// ---- extraction ----

export function extractSurface({ frontendRoot = DEFAULT_ROOT, sdkFile = path.join(frontendRoot, SDK_REL) } = {}) {
  const ts = loadTs(frontendRoot);
  const libDir = path.join(frontendRoot, 'src/lib');
  const program = ts.createProgram([sdkFile], compilerOptions(ts));
  const checker = program.getTypeChecker();
  const sf = program.getSourceFile(sdkFile);
  if (!sf) throw new Error(`cannot read the SDK at ${sdkFile}`);
  const moduleSymbol = checker.getSymbolAtLocation(sf);
  if (!moduleSymbol) throw new Error(`the SDK at ${sdkFile} is not a module`);

  // Absolute import("...") paths become relative to src/lib; node_modules
  // paths become their package specifier. No machine path reaches a lock (L7).
  const normalize = (text) => text
    .replace(/import\("([^"]+)"\)/g, (_m, spec) => {
      if (!path.isAbsolute(spec)) return `import("${spec}")`;
      const nm = spec.lastIndexOf('/node_modules/');
      if (nm >= 0) {
        const rest = spec.slice(nm + '/node_modules/'.length).replace(/\/(index)?(\.d)?\.(ts|js|mts|mjs)$/, '').replace(/\.d$/, '');
        return `import("${rest}")`;
      }
      let rel = path.relative(libDir, spec).split(path.sep).join('/');
      if (!rel.startsWith('.')) rel = `./${rel}`;
      return `import("${rel}")`;
    });

  const objectText = (type) => {
    const parts = [];
    for (const sig of type.getConstructSignatures()) parts.push(`new ${checker.signatureToString(sig, undefined, FLAGS(ts))}`);
    for (const sig of type.getCallSignatures()) parts.push(checker.signatureToString(sig, undefined, FLAGS(ts)));
    for (const prop of checker.getPropertiesOfType(type)) {
      const decl = prop.valueDeclaration || prop.declarations?.[0];
      if (decl && ts.getCombinedModifierFlags(decl) & (ts.ModifierFlags.Private | ts.ModifierFlags.Protected)) continue;
      if (prop.name.startsWith('#') || prop.name.startsWith('__@')) continue;
      const optional = prop.flags & ts.SymbolFlags.Optional ? '?' : '';
      const readonly = decl && ts.getCombinedModifierFlags(decl) & ts.ModifierFlags.Readonly ? 'readonly ' : '';
      const t = checker.typeToString(checker.getTypeOfSymbolAtLocation(prop, decl || sf), undefined, FLAGS(ts));
      parts.push(`${readonly}${prop.name}${optional}: ${t}`);
    }
    return `{ ${parts.join('; ')}${parts.length ? '; ' : ''}}`;
  };

  // describe returns { kind, type } for one symbol: values by the type of the
  // value, type aliases expanded, interfaces and classes as object literals.
  const describe = (symbol) => {
    let target = symbol;
    if (target.flags & ts.SymbolFlags.Alias) target = checker.getAliasedSymbol(target);
    const decl = target.valueDeclaration || target.declarations?.[0];
    if (target.flags & ts.SymbolFlags.Value) {
      const type = checker.getTypeOfSymbolAtLocation(target, decl || sf);
      if (target.flags & ts.SymbolFlags.Class) {
        const instance = checker.getDeclaredTypeOfSymbol(target);
        return { kind: 'class', type: normalize(`${objectText(type)} & ${objectText(instance)}`) };
      }
      return { kind: 'value', type: normalize(checker.typeToString(type, undefined, FLAGS(ts))) };
    }
    const declared = checker.getDeclaredTypeOfSymbol(target);
    if (target.flags & ts.SymbolFlags.TypeAlias) {
      return { kind: 'type', type: normalize(checker.typeToString(declared, undefined, FLAGS(ts) | ts.TypeFormatFlags.InTypeAlias)) };
    }
    return { kind: 'type', type: normalize(objectText(declared)) };
  };

  const names = {};
  for (const symbol of checker.getExportsOfModule(moduleSymbol)) {
    const d = describe(symbol);
    // L7: SDK_VERSION's literal type is the version itself; record what it is, not which.
    if (symbol.name === 'SDK_VERSION') d.type = 'string';
    names[symbol.name] = d;
  }

  // L8: expand the local types the surface references, to a fixed point.
  const types = {};
  const refRe = /import\("(\.[^"]+)"\)\.([A-Za-z_$][\w$]*)/g;
  const resolveRef = (spec, name) => {
    const base = path.resolve(libDir, spec);
    for (const candidate of [`${base}.ts`, `${base}.js`, `${base}.d.ts`, base, path.join(base, 'index.ts'), path.join(base, 'index.js')]) {
      const file = program.getSourceFile(candidate);
      if (!file) continue;
      const mod = checker.getSymbolAtLocation(file);
      const sym = mod && checker.getExportsOfModule(mod).find((s) => s.name === name);
      if (sym) return sym;
    }
    return null;
  };
  const queue = Object.values(names).map((d) => d.type);
  while (queue.length) {
    const text = queue.shift();
    for (const m of text.matchAll(refRe)) {
      const key = `import("${m[1]}").${m[2]}`;
      if (key in types) continue;
      const sym = resolveRef(m[1], m[2]);
      if (!sym) {
        types[key] = { kind: 'unresolved', type: '' };
        continue;
      }
      const d = describe(sym);
      types[key] = d;
      queue.push(d.type);
    }
  }

  const sdkVersion = readSdkVersion(frontendRoot);
  const moduleAbi = /export const MODULE_ABI = '([^']+)'/.exec(readFileSync(path.join(libDir, 'apps/packages.ts'), 'utf8'))?.[1] || '';
  const surface = {
    schema: SURFACE_SCHEMA,
    sdk_version: sdkVersion,
    typescript: ts.version,
    module_abi: moduleAbi,
    names: sortObject(names),
    types: sortObject(types),
  };
  const legacy = readLegacyEvents(frontendRoot);
  if (legacy) surface.legacy_events = legacy;
  surface.digest = surfaceDigest(surface);
  return surface;
}

export function readSdkVersion(frontendRoot = DEFAULT_ROOT) {
  const m = /export const SDK_VERSION = '(\d+\.\d+\.\d+)'/.exec(readFileSync(path.join(frontendRoot, 'src/lib/sdk-version.ts'), 'utf8'));
  if (!m) throw new Error('src/lib/sdk-version.ts has no SDK_VERSION = \'MAJOR.MINOR.PATCH\'');
  return m[1];
}

/** The events an app may still dispatch (src/lib/sdk-surface/legacy-events.json), or null before 1.1.0. */
export function readLegacyEvents(frontendRoot = DEFAULT_ROOT) {
  const file = path.join(frontendRoot, LOCK_REL, 'legacy-events.json');
  if (!existsSync(file)) return null;
  const events = JSON.parse(readFileSync(file, 'utf8')).events;
  if (!Array.isArray(events) || events.some((e) => !/^[a-z]{3,40}$/.test(e)) || new Set(events).size !== events.length) {
    throw new Error(`${path.relative(frontendRoot, file)}: events must be a list of distinct lowercase names`);
  }
  return [...events].sort();
}

function sortObject(obj) {
  return Object.fromEntries(Object.keys(obj).sort().map((k) => [k, obj[k]]));
}

/** The digest of what a package can rely on: names, types and the module ABI (not the version or the TypeScript that printed it). */
export function surfaceDigest(surface) {
  const body = JSON.stringify({
    module_abi: surface.module_abi,
    names: sortObject(surface.names),
    types: sortObject(surface.types),
    ...(surface.legacy_events ? { legacy_events: surface.legacy_events } : {}),
  });
  return `sha256:${createHash('sha256').update(body).digest('hex')}`;
}

// ---- comparison ----

// compatibility decides, with the checker, whether texts that differ are a
// widening (minor) or a break (major). One virtual file per pair, so one pair
// that does not compile (a referenced type that left) marks only itself major.
function compatibility(pairs, frontendRoot) {
  if (!pairs.length) return {};
  const ts = loadTs(frontendRoot);
  const libDir = path.join(frontendRoot, 'src/lib');
  const files = new Map();
  pairs.forEach((p, i) => {
    const file = path.join(libDir, `__sdk_surface_compare_${i}__.ts`);
    files.set(file, `export {};\ntype Old = ${p.old};\ntype New = ${p.new};\ndeclare const o: Old;\ndeclare const n: New;\nexport const __old = o;\nexport const __new = n;\n`);
  });
  const options = compilerOptions(ts);
  const host = ts.createCompilerHost(options);
  const getSourceFile = host.getSourceFile.bind(host);
  host.getSourceFile = (name, lang, ...rest) => (files.has(name) ? ts.createSourceFile(name, files.get(name), lang, true) : getSourceFile(name, lang, ...rest));
  const fileExists = host.fileExists.bind(host);
  host.fileExists = (name) => files.has(name) || fileExists(name);
  const readFile = host.readFile.bind(host);
  host.readFile = (name) => (files.has(name) ? files.get(name) : readFile(name));
  const program = ts.createProgram([...files.keys()], options, host);
  const checker = program.getTypeChecker();
  const out = {};
  pairs.forEach((p, i) => {
    const file = [...files.keys()][i];
    const sf = program.getSourceFile(file);
    // Diagnostics in the pair's own two lines mean a text no longer compiles: a break.
    const broken = program.getSemanticDiagnostics(sf).concat(program.getSyntacticDiagnostics(sf)).some((d) => d.file === sf);
    if (broken) {
      out[p.key] = 'major';
      return;
    }
    const typeOf = (name) => {
      const mod = checker.getSymbolAtLocation(sf);
      const sym = checker.getExportsOfModule(mod).find((s) => s.name === name);
      return checker.getTypeOfSymbolAtLocation(sym, sym.valueDeclaration);
    };
    const oldT = typeOf('__old');
    const newT = typeOf('__new');
    const fwd = checker.isTypeAssignableTo(newT, oldT);
    const back = checker.isTypeAssignableTo(oldT, newT);
    // Equivalent both ways: the printer reworded it (TypeScript orders union
    // members by creation, so an unrelated file can reorder one). No bump.
    if (fwd && back) out[p.key] = 'none';
    // A package uses a value: a new one that fits every old use widened it.
    else if (p.kind === 'value' && fwd) out[p.key] = 'minor';
    // A type is read and written by packages: only an equivalent one is safe.
    else out[p.key] = 'major';
  });
  return out;
}

/**
 * compareSurfaces says what changed between two surfaces and which bump that
 * needs: { removed, added, changed: [{ name, kind, need }], required }.
 */
export function compareSurfaces(before, after, { frontendRoot = DEFAULT_ROOT } = {}) {
  const removed = [];
  const added = [];
  const changed = [];
  const pairs = [];
  const visit = (section, label) => {
    const a = before[section] || {};
    const b = after[section] || {};
    for (const name of Object.keys(a)) {
      if (!(name in b)) {
        // A referenced type that is no longer referenced is not a removal; the names that used it changed.
        if (section === 'names') removed.push(name);
        continue;
      }
      if (a[name].type === b[name].type && a[name].kind === b[name].kind) continue;
      const key = `${label}${name}`;
      if (a[name].kind !== b[name].kind || a[name].kind === 'unresolved' || b[name].kind === 'unresolved') {
        changed.push({ name: key, kind: `${a[name].kind}->${b[name].kind}`, need: 'major', before: a[name].type, after: b[name].type });
        continue;
      }
      pairs.push({ key, kind: a[name].kind === 'value' ? 'value' : 'type', old: a[name].type, new: b[name].type });
    }
    if (section === 'names') for (const name of Object.keys(b)) if (!(name in a)) added.push(name);
  };
  visit('names', '');
  visit('types', 'type ');
  const verdicts = compatibility(pairs, frontendRoot);
  for (const p of pairs) changed.push({ name: p.key, kind: p.kind, need: verdicts[p.key], before: p.old, after: p.new });
  // The events an app may dispatch: one leaving breaks the apps that send it (major); one joining is new (minor).
  if (before.legacy_events || after.legacy_events) {
    const a = new Set(before.legacy_events || []);
    const b = new Set(after.legacy_events || []);
    for (const e of a) if (!b.has(e)) removed.push(`event:${e}`);
    for (const e of b) if (!a.has(e) && before.legacy_events) added.push(`event:${e}`);
  }
  if ((before.module_abi || '') !== (after.module_abi || '')) changed.push({ name: 'module_abi', kind: 'abi', need: 'major', before: before.module_abi, after: after.module_abi });

  let required = 'none';
  if (added.length) required = 'minor';
  for (const c of changed) if (RANK[c.need] > RANK[required]) required = c.need;
  if (removed.length) required = 'major';
  return { removed, added, changed, required };
}

// ---- versions and locks ----

const parse = (v) => {
  const m = /^(\d+)\.(\d+)\.(\d+)$/.exec(String(v));
  if (!m) throw new Error(`not a MAJOR.MINOR.PATCH version: ${v}`);
  return m.slice(1).map(Number);
};

export function compareVersions(a, b) {
  const x = parse(a);
  const y = parse(b);
  for (let i = 0; i < 3; i++) if (x[i] !== y[i]) return x[i] < y[i] ? -1 : 1;
  return 0;
}

/** bumpKind(from, to): none, patch, minor, major, or backwards. */
export function bumpKind(from, to) {
  const c = compareVersions(from, to);
  if (c === 0) return 'none';
  if (c > 0) return 'backwards';
  const [a, b] = [parse(from), parse(to)];
  if (b[0] !== a[0]) return 'major';
  if (b[1] !== a[1]) return 'minor';
  return 'patch';
}

export function lockDir(frontendRoot = DEFAULT_ROOT) {
  return path.join(frontendRoot, LOCK_REL);
}

export function readLocks(dir = lockDir()) {
  if (!existsSync(dir)) return [];
  return readdirSync(dir)
    .filter((f) => /^\d+\.\d+\.\d+\.json$/.test(f))
    .map((f) => ({ version: f.replace(/\.json$/, ''), file: path.join(dir, f), surface: JSON.parse(readFileSync(path.join(dir, f), 'utf8')) }))
    .sort((a, b) => compareVersions(a.version, b.version));
}

/**
 * checkLock: the surface of the tree against its locks. ok is true when the
 * lock for SDK_VERSION exists and equals the surface, and the step from the
 * previous lock is bumped enough for what changed.
 */
export function checkLock({ frontendRoot = DEFAULT_ROOT, dir = lockDir(frontendRoot), surface = extractSurface({ frontendRoot }), base = '' } = {}) {
  const problems = [];
  const history = base ? lockHistory({ base, dir }) : null;
  if (history) problems.push(...history.problems);
  const version = surface.sdk_version;
  const count = Object.keys(surface.names).length;
  if (count < MIN_NAMES) problems.push(`the SDK surface has ${count} names (floor ${MIN_NAMES}); the extractor or app-sdk.ts is broken`);
  if (!surface.names.SDK_VERSION) problems.push('the SDK surface does not export SDK_VERSION');
  const typeCount = Object.keys(surface.types || {}).length;
  if (typeCount < MIN_TYPES) problems.push(`the SDK surface references ${typeCount} local types (floor ${MIN_TYPES}); the extractor no longer reads import("...") references`);
  const unresolved = Object.entries(surface.types).filter(([, d]) => d.kind === 'unresolved').map(([k]) => k);
  if (unresolved.length) problems.push(`referenced types the extractor could not resolve: ${unresolved.join(', ')}`);
  const locks = readLocks(dir);
  const current = locks.find((l) => l.version === version);
  const previous = [...locks].reverse().find((l) => compareVersions(l.version, version) < 0) || null;
  const newest = locks[locks.length - 1];
  if (newest && compareVersions(newest.version, version) > 0) {
    problems.push(`SDK_VERSION ${version} is older than the newest lock ${newest.version}: versions do not go backwards`);
  }
  let diff = null;
  let bump = 'none';
  if (current) {
    diff = compareSurfaces(current.surface, surface, { frontendRoot });
    if (diff.required !== 'none') {
      const base = previous ? previous.version : 'none';
      problems.push(`the SDK surface differs from the lock for ${version} (needs a ${diff.required} bump): ${describeDiff(diff)}. Bump SDK_VERSION in src/lib/sdk-version.ts (${diff.required}) and run node scripts/sdk-surface.mjs --write (previous lock: ${base})`);
    }
  } else if (!previous) {
    problems.push(`no lock for SDK_VERSION ${version} in ${path.relative(frontendRoot, dir)}; run node scripts/sdk-surface.mjs --write`);
  } else {
    problems.push(`no lock for SDK_VERSION ${version}; run node scripts/sdk-surface.mjs --write`);
  }
  // M1: every adjacent pair of locks is a step too, so a fabricated
  // intermediate lock that already holds a break cannot launder it into a
  // patch: v_i -> v_{i+1} must bump at least what its own diff needs.
  for (let i = 0; i + 1 < locks.length; i++) {
    const a = locks[i];
    const b = locks[i + 1];
    const pair = compareSurfaces(a.surface, b.surface, { frontendRoot });
    const kind = bumpKind(a.version, b.version);
    if (RANK[kind] < RANK[pair.required]) {
      problems.push(`locks ${a.version} -> ${b.version} are a ${kind} bump, but their surfaces differ by a ${pair.required} change: ${describeDiff(pair)}`);
    }
  }
  if (previous) {
    const stepDiff = compareSurfaces(previous.surface, surface, { frontendRoot });
    bump = bumpKind(previous.version, version);
    if (RANK[bump] < RANK[stepDiff.required]) {
      problems.push(`SDK ${previous.version} -> ${version} is a ${bump} bump, but the surface change needs ${stepDiff.required}: ${describeDiff(stepDiff)}`);
    }
    if (!diff || diff.required === 'none') diff = stepDiff;
  }
  return {
    ok: problems.length === 0,
    version,
    previous: previous ? previous.version : null,
    bump,
    digest: surface.digest,
    lock: current ? path.relative(frontendRoot, current.file) : null,
    history,
    names: count,
    problems,
    diff,
    surface,
  };
}

export function describeDiff(diff) {
  const parts = [];
  if (diff.removed.length) parts.push(`removed ${diff.removed.join(', ')}`);
  if (diff.added.length) parts.push(`added ${diff.added.join(', ')}`);
  for (const c of diff.changed) if (c.need !== 'none') parts.push(`changed ${c.name} (${c.need})`);
  return parts.join('; ') || 'no change';
}

/**
 * lockHistory (B1): a lock is never changed once it exists. Against
 * merge-base(base, HEAD), every lock file in `dir` that existed there must be
 * byte-for-byte unchanged in the working tree (no modification, deletion or
 * rename); only new lock files may be added. Without this, the only lock
 * could be deleted and written again with a smaller surface, and with one
 * lock there is no previous step to catch it. A base that cannot be read
 * fails: a check that passes when it cannot see is open exactly then.
 * Returns { base, merge_base, problems, changed }.
 */
export function lockHistory({ base, dir = lockDir() }) {
  const git = (...args) => spawnSync('git', args, { cwd: dir, encoding: 'utf8' });
  const out = { base, merge_base: null, problems: [], changed: [] };
  const top = git('rev-parse', '--show-toplevel');
  if (top.status !== 0) {
    out.problems.push(`lock history: ${dir} is not in a git work tree`);
    return out;
  }
  const mb = git('merge-base', base, 'HEAD');
  if (mb.status !== 0 || !mb.stdout.trim()) {
    out.problems.push(`lock history: cannot find the merge base of ${base} and HEAD (${(mb.stderr || '').trim() || 'no output'}); fetch the base's history`);
    return out;
  }
  out.merge_base = mb.stdout.trim();
  const rel = path.relative(top.stdout.trim(), realpathSync(dir)).split(path.sep).join('/');
  // The working tree, not HEAD: an uncommitted rewrite is refused too.
  // Pathspecs are relative to the working directory: name the lock directory from the top.
  const diff = git('diff', '--no-renames', '--name-status', out.merge_base, '--', `:(top)${rel}`);
  if (diff.status !== 0) {
    out.problems.push(`lock history: git diff failed: ${diff.stderr.trim()}`);
    return out;
  }
  for (const line of diff.stdout.split('\n').filter(Boolean)) {
    const [status, file] = line.split('\t');
    if (!/\.json$/.test(file)) continue;
    out.changed.push(`${status} ${file}`);
    if (status !== 'A') {
      const what = status === 'D' ? 'deleted (or renamed)' : status === 'M' ? 'modified' : `changed (${status})`;
      out.problems.push(`lock history: ${file} existed at ${out.merge_base.slice(0, 12)} and is ${what}; a lock is never changed once it exists. Bump SDK_VERSION and write a new lock instead`);
    }
  }
  // M1: a new lock sorts above every lock the merge base holds; a lock slotted
  // in below a released one would rewrite the history the steps are judged on.
  const listed = git('ls-tree', '--name-only', out.merge_base, '--', `:(top)${rel}/`);
  const versionOf = (f) => (/(\d+\.\d+\.\d+)\.json$/.exec(f) || [])[1];
  const atBase = listed.status === 0 ? listed.stdout.split('\n').map(versionOf).filter(Boolean) : [];
  const now = existsSync(dir) ? readdirSync(dir).map(versionOf).filter(Boolean) : [];
  const newestAtBase = atBase.sort(compareVersions).at(-1);
  if (newestAtBase) {
    for (const v of now.filter((x) => !atBase.includes(x))) {
      if (compareVersions(v, newestAtBase) <= 0) {
        out.problems.push(`lock history: the new lock ${v}.json sorts below ${newestAtBase}.json, which the merge base already holds; a new lock goes above every released one`);
      }
    }
  }
  return out;
}

/**
 * minSdkFor: the lowest locked version (within the major of `range`) whose
 * surface holds every name in `used`, or null when none does.
 */
export function minSdkFor(used, dir = lockDir()) {
  for (const lock of readLocks(dir)) {
    if (used.every((n) => n in lock.surface.names)) return lock.version;
  }
  return null;
}

/** The lowest version `^MAJOR.MINOR` admits, as MAJOR.MINOR.0. */
export function rangeFloor(range) {
  const m = /^\^(\d+)\.(\d+)$/.exec(String(range || ''));
  return m ? `${Number(m[1])}.${Number(m[2])}.0` : null;
}

/**
 * requiresProblem: '' when every name in `used` exists in the lowest SDK
 * version `range` admits (L10), else why not.
 */
export function requiresProblem(range, used, dir = lockDir()) {
  const floor = rangeFloor(range);
  if (!floor) return `requires_sdk ${JSON.stringify(range)} is not ^MAJOR.MINOR`;
  const locks = readLocks(dir);
  // The surface at the floor is the newest lock at or below it within its major.
  const atFloor = [...locks].reverse().find((l) => compareVersions(l.version, floor) <= 0 && parse(l.version)[0] === parse(floor)[0])
    || locks.find((l) => parse(l.version)[0] === parse(floor)[0] && compareVersions(l.version, floor) >= 0);
  if (!atFloor) return `no SDK lock in the major of ${range}`;
  const missing = used.filter((n) => !(n in atFloor.surface.names));
  if (missing.length) {
    return `requires_sdk ${range} admits SDK ${atFloor.version}, which lacks ${missing.join(', ')}; the package needs at least ${minSdkFor(used, dir) || 'a version that has them'}`;
  }
  return '';
}

// ---- CLI ----

function main(argv) {
  const frontendRoot = DEFAULT_ROOT;
  const surface = extractSurface({ frontendRoot });
  if (argv.includes('--json')) {
    process.stdout.write(`${JSON.stringify(surface, null, 2)}\n`);
    return 0;
  }
  const bi = argv.indexOf('--base');
  const base = bi >= 0 ? argv[bi + 1] || '' : '';
  if (bi >= 0 && !base) {
    console.error('sdk-surface: --base needs a ref');
    return 2;
  }
  const result = checkLock({ frontendRoot, surface, base });
  if (argv.includes('--write')) {
    const dir = lockDir(frontendRoot);
    const target = path.join(dir, `${surface.sdk_version}.json`);
    // Writing is allowed when the only problems are the missing or stale lock
    // for this version and the step from the previous lock is bumped enough.
    const blocking = result.problems.filter((p) => !/^no lock for SDK_VERSION|^the SDK surface differs from the lock for/.test(p));
    if (existsSync(target) && result.problems.some((p) => p.startsWith('the SDK surface differs from the lock for'))) {
      blocking.push(`the lock for ${surface.sdk_version} exists; a lock is never rewritten. Bump SDK_VERSION (${result.diff ? result.diff.required : 'patch'} or more) first. (Deleting it and writing it again is safe only while it has never reached main; CI refuses any change to a lock that exists at the merge base.)`);
    }
    if (blocking.length) {
      for (const p of blocking) console.error(`sdk-surface: ${p}`);
      return 1;
    }
    if (existsSync(target)) {
      console.log(`sdk-surface: the lock for ${surface.sdk_version} already holds this surface; nothing written`);
      return 0;
    }
    mkdirSync(dir, { recursive: true });
    writeFileSync(target, `${JSON.stringify(surface, null, 2)}\n`);
    console.log(`sdk-surface: wrote ${path.relative(frontendRoot, target)} (${Object.keys(surface.names).length} names, ${surface.digest})`);
    return 0;
  }
  for (const p of result.problems) console.error(`sdk-surface: ${p}`);
  const seen = result.history ? `, locks unchanged since ${result.history.merge_base ? result.history.merge_base.slice(0, 12) : base}` : '';
  console.log(`sdk-surface: SDK ${result.version}, ${result.names} names, ${result.digest}${seen}: ${result.ok ? 'ok' : 'FAILED'}`);
  return result.ok ? 0 : 1;
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  process.exit(main(process.argv.slice(2)));
}
