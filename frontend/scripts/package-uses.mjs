// Static `uses` extraction (slice U1; docs/design/modular-architecture-2026-11-04.md
// section 2.5). `uses` is extracted, never authored: the SDK functions a package
// calls, the commands it contributes, and the grants those imply. The release
// record carries it, and the build fails when `uses` exceeds the grants the
// manifest declares (grant-implying uses only).
//
//   node scripts/package-uses.mjs [--json] [<package> ...]
//
// The extractor reads each package's source as the build does: Svelte and
// TypeScript are compiled, then the module AST goes through the boundary
// plugin's analyzeModule (imports, re-exports, import(), the global object), so
// the in-tree gate and the installed build see the same names and refuse the
// same evasions. Only imports of the SDK count; the package boundary already
// refuses every other reach, so a name cannot arrive by another route.
import { existsSync, readFileSync, readdirSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { parseAst, transformWithEsbuild } from 'vite';
import { compile, preprocess } from 'svelte/compiler';
import { vitePreprocess } from '@sveltejs/vite-plugin-svelte';
import { SCRIPT_EXT, analyzeModule } from './package-boundary-plugin.mjs';

const FRONTEND = resolve(dirname(fileURLToPath(import.meta.url)), '..');

function sourceFiles(dir) {
  const out = [];
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const full = join(dir, entry.name);
    if (entry.isDirectory()) out.push(...sourceFiles(full));
    else if (SCRIPT_EXT.test(entry.name)) out.push(full); // the boundary's own set: what the build analyzes, extraction reads
  }
  return out.sort();
}

/** The compiled JavaScript of one package source file. */
async function compiled(file) {
  const source = readFileSync(file, 'utf8');
  if (file.endsWith('.svelte')) {
    const pre = await preprocess(source, vitePreprocess(), { filename: file });
    return compile(pre.code, { filename: file, generate: 'dom', css: 'external' }).js.code;
  }
  const loader = /\.(ts|mts|cts)$/i.test(file) ? 'ts' : /\.tsx$/i.test(file) ? 'tsx' : /\.jsx$/i.test(file) ? 'jsx' : '';
  if (loader) return (await transformWithEsbuild(source, file, { loader, target: 'es2022' })).code;
  return source;
}

/** The SDK names one package file uses, or a thrown refusal. */
export async function sdkReachOf(file) {
  let ast;
  try {
    ast = parseAst(await compiled(file));
  } catch (err) {
    throw new Error(`${file}: cannot be read to extract its uses: ${err.message}`);
  }
  const { names, problems } = analyzeModule(ast, { checkMeta: true });
  if (problems.length) throw new Error(`${file}: ${problems.join('; ')}`);
  return names;
}

export function readSections(frontendRoot = FRONTEND) {
  return JSON.parse(readFileSync(join(frontendRoot, 'src/lib/sdk-surface/sections.json'), 'utf8'));
}

/** { sdk: [names], grants: [implied grants], commands: [ids] } for the package at `dir`. */
export async function extractUses(dir, sections = readSections()) {
  const sdk = new Set();
  for (const file of sourceFiles(dir)) for (const n of await sdkReachOf(file)) sdk.add(n);
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
    const uses = await extractUses(dir);
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
