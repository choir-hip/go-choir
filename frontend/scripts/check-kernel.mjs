#!/usr/bin/env node
// Structural gates for the front-end kernel (docs/design/modular-architecture-plan-2026-11-04.md
// section 5.1, owner ruling R1: structural gates replace line limits).
//   1. Imports: kernel files import only other kernel files (no bare
//      specifier, no path leaving src/lib/kernel/, no dynamic import).
//   2. Names: kernel source names no app id, package name or desk. The list is
//      built at check time from apps/registry.ts, the base packages' choir-package.json
//      (src/packages/base-packages.json lists them)
//      and the legacy literals below.
//   3. Responsibilities: the exported symbols per file equal the checked-in
//      src/lib/kernel/responsibilities.json.
// Limit, by design: this is a review tripwire against drift, not a sandbox. It
// is regex based, so deliberate evasion passes (token splitting such as
// 'imp'+'ort', globalThis['require'], a banned name embedded in a longer
// token). Closing those would need a real parser and would still not stop a
// determined author; review and the leak differential cover the rest.
// `--root <frontend dir>` points it at a fixture tree (tests/kernel.spec.js).
import { existsSync, readdirSync, readFileSync } from 'node:fs';
import { dirname, join, resolve, relative } from 'node:path';
import { fileURLToPath } from 'node:url';

const argv = process.argv.slice(2);
const rootFlag = argv.indexOf('--root');
if (rootFlag >= 0 && !argv[rootFlag + 1]) {
  console.error('check-kernel: --root needs a directory argument');
  process.exit(2);
}
const frontendRoot = resolve(
  rootFlag >= 0 ? argv[rootFlag + 1] : join(dirname(fileURLToPath(import.meta.url)), '..'),
);
const kernelDir = join(frontendRoot, 'src/lib/kernel');

// Desks and legacy app literals that never appear in the kernel.
const LEGACY_NAMES = [
  'management', 'engineering', 'research', 'texture', 'conductor', 'mail', 'email',
  'calendar', 'slides', 'autopaper', 'desktop', 'desk',
];

const errors = [];
const fail = (where, msg) => errors.push(`${where}: ${msg}`);

function kernelFiles() {
  if (!existsSync(kernelDir)) return [];
  const out = [];
  const walk = (dir) => {
    for (const e of readdirSync(dir, { withFileTypes: true })) {
      const full = join(dir, e.name);
      if (e.isDirectory()) walk(full);
      else if (/\.(?:ts|js|mjs|cjs|tsx|jsx)$/.test(e.name)) out.push(relative(kernelDir, full));
    }
  };
  walk(kernelDir);
  return out.sort();
}

function stripComments(src) {
  return src.replace(/\/\*[\s\S]*?\*\//g, '').replace(/(^|[^:'"`])\/\/.*$/gm, '$1');
}

function builtNames() {
  const names = new Set(LEGACY_NAMES);
  const reg = join(frontendRoot, 'src/lib/apps/registry.ts');
  if (existsSync(reg)) {
    const src = readFileSync(reg, 'utf8');
    for (const m of src.matchAll(/\bid:\s*['"`]([a-z][a-z0-9-]*)['"`]/g)) names.add(m[1]);
    // ids written as constants: export const X_ID = 'texture'
    for (const m of src.matchAll(/\b[A-Z][A-Z0-9_]*_ID\b\s*(?::[^=]+)?=\s*['"`]([a-z][a-z0-9-]*)['"`]/g)) names.add(m[1]);
  }
  // Built-in names are the base packages, the committed allowlist; a package
  // a capsule adds under src/packages is not a kernel built-in.
  const pk = join(frontendRoot, 'src/packages');
  const allowlist = join(pk, 'base-packages.json');
  if (existsSync(pk) && !existsSync(allowlist)) {
    fail('src/packages/base-packages.json', 'missing: the kernel names list is built from the base package allowlist');
  } else if (existsSync(allowlist)) {
    for (const d of JSON.parse(readFileSync(allowlist, 'utf8')).packages ?? []) {
      const f = join(pk, d, 'choir-package.json');
      if (!existsSync(f)) continue;
      try {
        const j = JSON.parse(readFileSync(f, 'utf8'));
        if (typeof j.name === 'string') names.add(j.name);
        for (const a of j.app?.aliases ?? []) if (typeof a === 'string') names.add(a);
      } catch { /* check-packages reports a bad manifest */ }
    }
  }
  return names;
}

function importSpecifiers(code) {
  const out = [];
  for (const m of code.matchAll(/\b(?:import|export)\b[^'"`;]*?\bfrom\s*['"]([^'"]+)['"]/g)) out.push(m[1]);
  for (const m of code.matchAll(/\bimport\s*['"]([^'"]+)['"]/g)) out.push(m[1]);
  if (/\bimport\s*\(/.test(code)) out.push('<dynamic import>');
  if (/\brequire\s*\(/.test(code)) out.push('<require>');
  return out;
}

function exportedSymbols(code) {
  const out = new Set();
  for (const m of code.matchAll(/\bexport\s+(?:declare\s+)?(?:abstract\s+)?(?:async\s+)?(?:class|function\*?|const|let|var|interface|type|enum)\s+([A-Za-z_$][\w$]*)/g)) out.add(m[1]);
  for (const m of code.matchAll(/\bexport\s+(?:type\s+)?\{([^}]*)\}/g)) {
    for (const part of m[1].split(',')) {
      const s = part.trim();
      if (!s) continue;
      const as = s.split(/\s+as\s+/);
      out.add((as[1] ?? as[0]).replace(/^type\s+/, '').trim());
    }
  }
  if (/\bexport\s+default\b/.test(code)) out.add('default');
  if (/\bexport\s*\*/.test(code)) out.add('*');
  return out;
}

const files = kernelFiles();
if (files.length === 0) fail('kernel', `no kernel source under ${relative(frontendRoot, kernelDir)}`);

const names = builtNames();
const surface = {};
for (const f of files) {
  const code = stripComments(readFileSync(join(kernelDir, f), 'utf8'));
  // 1. imports
  for (const spec of importSpecifiers(code)) {
    if (spec === '<dynamic import>' || spec === '<require>') {
      fail(f, `${spec} is not allowed in the kernel`);
    } else if (!spec.startsWith('./') || spec.slice(2).includes('/')) {
      fail(f, `imports "${spec}"; the kernel imports only its own files`);
    } else if (!files.some((x) => x === spec.slice(2) || x.replace(/\.[a-z]+$/, '') === spec.slice(2))) {
      fail(f, `imports "${spec}", which is not a kernel file`);
    }
  }
  // 2. names
  const seen = new Set();
  for (const tok of code.match(/[A-Za-z0-9_-]+/g) ?? []) {
    const lower = tok.toLowerCase();
    if (names.has(lower) && !seen.has(lower)) {
      seen.add(lower);
      fail(f, `names "${lower}"; the kernel knows no package, app or desk`);
    }
  }
  surface[f] = [...exportedSymbols(code)].sort();
}

// 3. responsibilities
const listFile = join(kernelDir, 'responsibilities.json');
if (!existsSync(listFile)) {
  fail('responsibilities.json', 'missing');
} else {
  let listed = {};
  try {
    const j = JSON.parse(readFileSync(listFile, 'utf8'));
    for (const [resp, byFile] of Object.entries(j.responsibilities ?? {})) {
      for (const [f, syms] of Object.entries(byFile)) {
        if (listed[f]) fail('responsibilities.json', `${f} is listed under two responsibilities (${resp})`);
        listed[f] = [...syms].sort();
      }
    }
  } catch (e) {
    fail('responsibilities.json', `unreadable: ${e.message}`);
  }
  for (const f of new Set([...Object.keys(listed), ...files])) {
    if (!(f in surface)) { fail(f, 'listed in responsibilities.json but not a kernel file'); continue; }
    if (!(f in listed)) { fail(f, 'kernel file is in no responsibility'); continue; }
    const have = new Set(surface[f]);
    const want = new Set(listed[f]);
    for (const s of have) if (!want.has(s)) fail(f, `exports "${s}", which responsibilities.json does not list`);
    for (const s of want) if (!have.has(s)) fail(f, `responsibilities.json lists "${s}", which is not exported`);
  }
}

if (errors.length > 0) {
  console.error('check-kernel: FAILED');
  for (const e of errors) console.error(`  - ${e}`);
  process.exit(1);
}
console.log(`check-kernel: ok (${files.length} files, ${names.size} names checked)`);
