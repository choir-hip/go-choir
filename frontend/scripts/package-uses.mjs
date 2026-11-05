// Static `uses` extraction (slice U1; docs/design/modular-architecture-2026-11-04.md
// section 2.5). `uses` is extracted, never authored: the SDK functions a package
// calls, the commands it contributes, and the grants those imply. The release
// record carries it, and the build fails when `uses` exceeds the grants the
// manifest declares (grant-implying uses only).
//
//   node scripts/package-uses.mjs [--json] [<package> ...]
//
// The extractor reads each package's source for imports of the SDK
// (src/lib/app-sdk) and the names they bring in. Only imports of the SDK count;
// the package boundary (package-boundary-plugin.mjs) already refuses every
// other reach, so a name cannot arrive by another route.
import { existsSync, readFileSync, readdirSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const FRONTEND = resolve(dirname(fileURLToPath(import.meta.url)), '..');

function sourceFiles(dir) {
  const out = [];
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const full = join(dir, entry.name);
    if (entry.isDirectory()) out.push(...sourceFiles(full));
    else if (/\.(svelte|js|mjs|ts)$/.test(entry.name)) out.push(full);
  }
  return out.sort();
}

/** The SDK names one source text imports. */
export function sdkImportsOf(source) {
  const names = new Set();
  const re = /import\s+(type\s+)?(?:\{([^}]*)\}|\*\s+as\s+\w+|\w+)\s+from\s+['"][^'"]*\/app-sdk(?:\.[a-z]+)?['"]/g;
  for (const m of source.matchAll(re)) {
    if (m[1]) continue; // import type: no runtime use
    if (!m[2]) {
      names.add('*');
      continue;
    }
    for (const part of m[2].split(',')) {
      const name = part.trim().replace(/^type\s+.*/, '').split(/\s+as\s+/)[0].trim();
      if (name) names.add(name);
    }
  }
  return names;
}

export function readSections(frontendRoot = FRONTEND) {
  return JSON.parse(readFileSync(join(frontendRoot, 'src/lib/sdk-surface/sections.json'), 'utf8'));
}

/** { sdk: [names], grants: [implied grants], commands: [ids] } for the package at `dir`. */
export function extractUses(dir, sections = readSections()) {
  const sdk = new Set();
  for (const file of sourceFiles(dir)) for (const n of sdkImportsOf(readFileSync(file, 'utf8'))) sdk.add(n);
  if (sdk.has('*')) throw new Error(`${dir}: a namespace import of the SDK hides which names are used; import them one by one`);
  const grants = new Set();
  for (const name of sdk) if (sections.grants?.[name]) grants.add(sections.grants[name]);
  let commands = [];
  const manifestPath = join(dir, 'choir-package.json');
  if (existsSync(manifestPath)) {
    const manifest = JSON.parse(readFileSync(manifestPath, 'utf8'));
    commands = (manifest.contributes?.commands ?? []).map((c) => c.id);
  }
  return { sdk: [...sdk].sort(), grants: [...grants].sort(), commands: commands.sort() };
}

/** Problems when a package's implied grants exceed what its manifest declares. */
export function usesProblems(uses, manifest) {
  const declared = new Set((manifest?.capabilities ?? []).map((c) => c.grant));
  return uses.grants.filter((g) => !declared.has(g)).map((g) => `uses imply grant "${g}" but the manifest does not declare it in capabilities`);
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const args = process.argv.slice(2);
  const json = args.includes('--json');
  const root = join(FRONTEND, 'src/packages');
  const names = args.filter((a) => !a.startsWith('--'));
  const out = {};
  let bad = 0;
  for (const name of names.length ? names : readdirSync(root)) {
    const dir = join(root, name);
    if (!existsSync(join(dir, 'choir-package.json'))) continue;
    const uses = extractUses(dir);
    out[name] = uses;
    for (const p of usesProblems(uses, JSON.parse(readFileSync(join(dir, 'choir-package.json'), 'utf8')))) {
      console.error(`${name}: ${p}`);
      bad++;
    }
  }
  if (json) console.log(JSON.stringify(out, null, 2));
  else for (const [name, u] of Object.entries(out)) console.log(`${name}: ${u.sdk.length} SDK names, grants [${u.grants.join(', ')}]`);
  process.exit(bad ? 1 : 0);
}
