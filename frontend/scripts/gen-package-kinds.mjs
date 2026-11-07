#!/usr/bin/env node
// Generates internal/apppackage/firstparty_gen.go from the base package
// manifests (frontend/src/packages/<name>/choir-package.json for each name in
// frontend/src/packages/base-packages.json, the committed allowlist; a
// directory the list does not name is not built-in and registers no kinds,
// docs/design/package-operation-decision-2026-11-06.md 2.6): the table of data
// kinds each package declares, which the autoputer registers at boot through
// objectgraph.RegisterPackageKind (docs/design/app-packages-replan-2026-10-24.md
// section 2.2). With --check it writes nothing and exits 1 if the committed file
// is stale, as generate-source-contract.mjs --check does. Go validates the
// manifests in full (apppackage.ParseManifest, and a drift test over the same
// directory); this script refuses only what it cannot generate from.
import { existsSync, readFileSync, writeFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '../..');
const packagesDir = resolve(root, 'frontend/src/packages');
const outputPath = resolve(root, 'internal/apppackage/firstparty_gen.go');
const check = process.argv.includes('--check');

const fail = (message) => {
  console.error(`gen-package-kinds: ${message}`);
  process.exit(1);
};

// Sorted keys at every level, so the output does not depend on authoring order.
const canonical = (value) => {
  if (Array.isArray(value)) return value.map(canonical);
  if (value && typeof value === 'object') {
    return Object.fromEntries(Object.keys(value).sort().map((k) => [k, canonical(value[k])]));
  }
  return value;
};
const goString = (s) => JSON.stringify(s);

if (check && !existsSync(resolve(root, 'internal/apppackage'))) {
  // A deleted package with its generated file must not pass the drift check.
  fail('internal/apppackage is missing; --check cannot compare the generated table');
}

// Names a published package may not take: the reserved shell and system names
// of check-packages.mjs, and the retired app ids that now alias a first-party
// package (pdf, epub: reader; email: mail). The shell has no static app rows
// (slice T1 moved the last one, Texture, into its package); a published package
// of a first-party package's name is refused by builtinPackageNames instead.
// A published package of such a name would shadow or confuse the built-in app
// (review of MA2.3, B2).
const RESERVED_NAMES = [
  'desk', 'desktop', 'shell', 'prompt', 'system', 'owner', 'management', 'engineering', 'research',
];
const shellAppNames = new Set([...RESERVED_NAMES, 'pdf', 'epub', 'email']);

// The base packages are the allowlist, never a directory walk: a package a
// capsule adds under src/packages must not become built-in by being there.
const allowlistPath = resolve(packagesDir, 'base-packages.json');
const baseNames = (() => {
  if (!existsSync(allowlistPath)) {
    if (!existsSync(packagesDir)) return [];
    fail(`${allowlistPath} is missing; the base packages are the committed allowlist`);
  }
  let list;
  try {
    list = JSON.parse(readFileSync(allowlistPath, 'utf8'));
  } catch (err) {
    fail(`${allowlistPath}: ${err.message}`);
  }
  if (list.schema !== 'choir.base-packages/v1' || !Array.isArray(list.packages)) fail(`${allowlistPath}: want {"schema": "choir.base-packages/v1", "packages": [...]}`);
  for (const n of list.packages) if (typeof n !== 'string' || n === '') fail(`${allowlistPath}: every entry must be a package name`);
  if (new Set(list.packages).size !== list.packages.length) fail(`${allowlistPath}: a package is listed twice`);
  return [...list.packages].sort();
})();

const builtinNames = [];
const packages = [];
// contributes.settings of each in-tree package, as canonical JSON: the settings
// route (internal/mediastate) takes only a key a manifest declares.
const settings = [];
if (existsSync(packagesDir)) {
  for (const dir of baseNames) {
    const manifestPath = resolve(packagesDir, dir, 'choir-package.json');
    if (!existsSync(manifestPath)) fail(`${allowlistPath} lists ${dir}, which has no ${manifestPath}`);
    let manifest;
    try {
      manifest = JSON.parse(readFileSync(manifestPath, 'utf8'));
    } catch (err) {
      fail(`${manifestPath}: ${err.message}`);
    }
    if (manifest.schema !== 'choir.package/v2') fail(`${manifestPath}: schema must be "choir.package/v2" (the v1 reader is deleted)`);
    if (manifest.name !== dir) fail(`${manifestPath}: name ${JSON.stringify(manifest.name)} must equal the directory name ${dir}`);
    builtinNames.push(manifest.name);
    if (manifest.contributes?.settings !== undefined) {
      if (!Array.isArray(manifest.contributes.settings)) fail(`${manifestPath}: contributes.settings must be a list`);
      if (manifest.contributes.settings.length > 0) settings.push({ name: manifest.name, json: JSON.stringify(canonical(manifest.contributes.settings)) });
    }
    const kinds = manifest.kinds ?? [];
    if (!Array.isArray(kinds)) fail(`${manifestPath}: kinds must be a list`);
    if (kinds.length === 0) continue;
    packages.push({
      name: manifest.name,
      kinds: kinds.map((k, i) => {
        for (const field of ['kind', 'identity', 'schema_version', 'schema']) {
          if (k[field] === undefined) fail(`${manifestPath}: kinds[${i}].${field} is required`);
        }
        if (!Number.isInteger(k.schema_version)) fail(`${manifestPath}: kinds[${i}].schema_version must be an integer`);
        return {
          kind: k.kind,
          identity: k.identity,
          versioned: k.versioned === true,
          ownerEdits: k.owner_edits === true,
          schemaVersion: k.schema_version,
          reads: k.reads_schema_versions ?? [],
          schema: JSON.stringify(canonical(k.schema)),
        };
      }),
    });
  }
}

const lines = [
  '// Code generated by frontend/scripts/gen-package-kinds.mjs; DO NOT EDIT.',
  '',
  'package apppackage',
  '',
];
if (packages.length === 0) {
  lines.push('var firstPartyPackages = []firstPartyPackage{}');
} else {
  lines.push('var firstPartyPackages = []firstPartyPackage{');
  for (const p of packages) {
    lines.push(`\t{Name: ${goString(p.name)}, Kinds: []firstPartyKind{`);
    for (const k of p.kinds) {
      lines.push(
        `\t\t{Kind: ${goString(k.kind)}, Identity: ${goString(k.identity)}, Versioned: ${k.versioned}, OwnerEdits: ${k.ownerEdits}, SchemaVersion: ${k.schemaVersion}, ReadsSchemaVersions: []int{${k.reads.join(', ')}}, Schema: ${goString(k.schema)}},`,
      );
    }
    lines.push('\t}},');
  }
  lines.push('}');
}
lines.push('');
lines.push('// firstPartySettings is each in-tree package\'s contributes.settings, as canonical JSON.');
if (settings.length === 0) {
  lines.push('var firstPartySettings = map[string]string{}');
} else {
  lines.push('var firstPartySettings = map[string]string{');
  for (const e of settings) lines.push(`\t${goString(e.name)}: ${goString(e.json)},`);
  lines.push('}');
}
lines.push('');
lines.push('// builtinPackageNames are the in-tree first-party packages: an installed package of such a name replaces the built-in app, so only the owner\'s own track may install one.');
lines.push('var builtinPackageNames = []string{');
for (const n of builtinNames.sort()) lines.push(`\t${goString(n)},`);
lines.push('}');
lines.push('');
lines.push('// shellAppNames are the names a published package may not take (see gen-package-kinds.mjs).');
lines.push('var shellAppNames = []string{');
for (const n of [...shellAppNames].sort()) lines.push(`\t${goString(n)},`);
lines.push('}');
const generated = `${lines.join('\n')}\n`;

if (check) {
  const current = existsSync(outputPath) ? readFileSync(outputPath, 'utf8') : '';
  if (current !== generated) {
    console.error('internal/apppackage/firstparty_gen.go is stale; run node frontend/scripts/gen-package-kinds.mjs');
    process.exit(1);
  }
  process.exit(0);
}
writeFileSync(outputPath, generated);
