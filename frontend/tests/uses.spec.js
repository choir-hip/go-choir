// Static `uses` extraction and the lock sections, slice U1 (yellow: proof
// tooling). The extractor reads imports of the SDK from package source; the
// build fails when the grants those imply exceed the manifest's. The sections
// (pipelines, services, contributes, ctx, grants) are locked and diffed.
import { test, expect } from '@playwright/test';
import { spawnSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { extractUses, usesProblems, readSections } from '../scripts/package-uses.mjs';
import { compareSurfaces, extractSurface } from '../scripts/sdk-surface.mjs';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');

function tmpPackage(files) {
  const dir = fs.mkdtempSync(path.join(fs.realpathSync(process.env.TMPDIR || '/tmp'), 'choir-uses-pkg-'));
  for (const [rel, body] of Object.entries(files)) {
    fs.mkdirSync(path.dirname(path.join(dir, rel)), { recursive: true });
    fs.writeFileSync(path.join(dir, rel), body);
  }
  return dir;
}
const svelteWith = (body) => `<script>\n${body}\n</script>\n<div>ok</div>\n`;
const SDK = "'../../lib/app-sdk'";

test('extractUses reads named, multi-line, aliased and minified imports, re-exports and type imports', async () => {
  const dir = tmpPackage({
    'App.svelte': svelteWith(`import { a, b as c,\n  d } from ${SDK};\nimport{e}from${SDK};\nimport/**/{f}/**/from/**/${SDK};\nimport './h.js';\nconsole.log(a, c, d, e, f);`),
    'h.ts': `export { g } from ${SDK};\nimport type { T } from ${SDK};\nexport type U = T;\n`,
    'local.js': "import { z } from './local2.js'; export { z };\n",
    'local2.js': 'export const z = 1;\n',
  });
  expect((await extractUses(dir)).sdk).toEqual(['a', 'b', 'd', 'e', 'f', 'g']);
  fs.rmSync(dir, { recursive: true, force: true });
});

test('B3: extraction refuses every way around the named import, the same as the installed build', async () => {
  const evasions = {
    'a namespace import': [`import * as sdk from ${SDK};\nconsole.log(sdk);`, /one by one/],
    'export *': [`import './h.js';`, /one by one/, { 'h.js': `export * from ${SDK};\n` }],
    'a dynamic import': [`import(${SDK}).then(console.log);`, /one by one/],
    'a dynamic import, no space': [`import(${SDK.replace(/'/g, '`')}).then(console.log);`, /one by one/],
    'a split-string global': ["console.log(globalThis['__choir' + 'AppSdk']);", /computed access to the global object/],
    'an aliased global': ['const g = globalThis;\nconsole.log(g);', /bare "globalThis"/],
  };
  for (const [label, [body, pattern, extra]] of Object.entries(evasions)) {
    const dir = tmpPackage({ 'App.svelte': svelteWith(body), ...extra });
    await expect(extractUses(dir), label).rejects.toThrow(pattern);
    fs.rmSync(dir, { recursive: true, force: true });
  }
});

test('review r2: every file the build analyzes is read for uses, tsx and jsx included', async () => {
  for (const [file, body] of [['helper.tsx', `import { submitPrompt } from ${SDK};\nexport const f = () => submitPrompt;\n`], ['helper.jsx', `import { logout } from ${SDK};\nexport const f = () => logout;\n`], ['helper.mts', `import { openApp } from ${SDK};\nexport const f = openApp;\n`], ['helper.cjs', `import { showOverview } from ${SDK};\nexport const f = showOverview;\n`]]) {
    const dir = tmpPackage({ 'App.svelte': svelteWith(''), [file]: body });
    const uses = await extractUses(dir);
    expect(uses.grants, file).toEqual(['ui.shell']);
    expect(usesProblems(uses, { capabilities: [] }).length, file).toBe(1);
    fs.rmSync(dir, { recursive: true, force: true });
  }
});

test('check-packages refuses a grant name that is a prototype property', () => {
  const dir = fs.mkdtempSync(path.join(fs.realpathSync(process.env.TMPDIR || '/tmp'), 'choir-grant2-'));
  fs.cpSync(root, dir, { recursive: true, filter: (src) => !/node_modules|dist|test-results|\.timestamp-/.test(src) });
  fs.symlinkSync(path.join(root, 'node_modules'), path.join(dir, 'node_modules'));
  const p = path.join(dir, 'src/packages/image/choir-package.json');
  const m = JSON.parse(fs.readFileSync(p, 'utf8'));
  m.capabilities = [{ grant: 'constructor' }];
  fs.writeFileSync(p, JSON.stringify(m, null, 2));
  const r = spawnSync(process.execPath, ['scripts/check-packages.mjs'], { cwd: dir, encoding: 'utf8' });
  fs.rmSync(dir, { recursive: true, force: true });
  expect(r.stdout + r.stderr).toMatch(/not a grant in the catalogue/);
});

test('B2: every shell-wide function implies ui.shell; the pure helpers imply nothing', async () => {
  const sections = readSections(root);
  for (const name of ['logout', 'windowAction', 'setAppContext', 'openApp', 'openFile', 'listWindows', 'setTheme', 'submitPrompt', 'showOverview', 'showDesktop', 'closeOtherWindows', 'resetDesktop', 'requireAuth', 'sessionExpired', 'focus', 'restore', 'suspendBackgroundHeavyWindows']) {
    expect(sections.grants[name], name).toBe('ui.shell');
  }
  for (const name of ['toast', 'copyText', 'buildInfo', 'getTheme', 'onThemeChange', 'registerCommand', 'fetchWithRenewal']) expect(sections.grants[name], name).toBeUndefined();
  // Every name in grants is a name the SDK exports and a grant in the catalogue.
  const catalogue = JSON.parse(fs.readFileSync(path.join(root, 'src/lib/sdk-surface/grants.json'), 'utf8')).grants;
  const surface = extractSurface({ frontendRoot: root });
  for (const [name, grant] of Object.entries(sections.grants)) {
    expect(catalogue[grant], `${name} -> ${grant}`).toBeTruthy();
    expect(surface.names, name).toHaveProperty(name);
  }
  const dir = tmpPackage({ 'App.svelte': svelteWith(`import { logout, windowAction } from ${SDK};\nconsole.log(logout, windowAction);`) });
  const uses = await extractUses(dir, sections);
  expect(uses.grants).toEqual(['ui.shell']);
  fs.rmSync(dir, { recursive: true, force: true });
});

test('every in-tree package has its uses extracted, and none exceeds its grants', async () => {
  const sections = readSections(root);
  for (const name of fs.readdirSync(path.join(root, 'src/packages'))) {
    const dir = path.join(root, 'src/packages', name);
    const uses = await extractUses(dir, sections);
    // A package that imports the SDK has its uses extracted; one that needs
    // nothing of it (the notification stack: props in, an event out) has none.
    const importsSdk = fs.readdirSync(dir).some((f) => /\.(svelte|[cm]?[jt]s)$/.test(f) && fs.readFileSync(path.join(dir, f), 'utf8').includes('lib/app-sdk'));
    if (importsSdk) expect(uses.sdk.length, name).toBeGreaterThan(0);
    else expect(uses.sdk, name).toEqual([]);
    expect(usesProblems(uses, JSON.parse(fs.readFileSync(path.join(dir, 'choir-package.json'), 'utf8'))), name).toEqual([]);
  }
});

test('B1: submitPrompt without ui.shell fails the build gate; with it passes check-packages (and ParseManifest, in the Go test, on the same fixture)', () => {
  const run = (declare) => {
    const dir = fs.mkdtempSync(path.join(fs.realpathSync(process.env.TMPDIR || '/tmp'), 'choir-uses-'));
    fs.cpSync(root, dir, { recursive: true, filter: (src) => !/node_modules|dist|test-results|\.timestamp-/.test(src) });
    fs.symlinkSync(path.join(root, 'node_modules'), path.join(dir, 'node_modules'));
    const app = path.join(dir, 'src/packages/image/App.svelte');
    fs.writeFileSync(app, fs.readFileSync(app, 'utf8').replace('<script', "<script context=\"module\">\n  import { submitPrompt } from '../../lib/app-sdk';\n  export const sendIt = submitPrompt;\n</script>\n<script"));
    if (declare) {
      const p = path.join(dir, 'src/packages/image/choir-package.json');
      const m = JSON.parse(fs.readFileSync(p, 'utf8'));
      m.capabilities = JSON.parse(fs.readFileSync(path.join(root, 'tests/fixtures/ui-shell-manifest.json'), 'utf8')).capabilities;
      fs.writeFileSync(p, JSON.stringify(m, null, 2));
    }
    const r = spawnSync(process.execPath, ['scripts/check-packages.mjs'], { cwd: dir, encoding: 'utf8' });
    fs.rmSync(dir, { recursive: true, force: true });
    return { status: r.status, out: r.stdout + r.stderr };
  };
  const without = run(false);
  expect(without.status).not.toBe(0);
  expect(without.out).toMatch(/uses imply grant "ui\.shell"/);
  const withGrant = run(true);
  expect(withGrant.status, withGrant.out).toBe(0);
});

test('check-packages validates a grant against the catalogue, not a shape', () => {
  const run = (grant) => {
    const dir = fs.mkdtempSync(path.join(fs.realpathSync(process.env.TMPDIR || '/tmp'), 'choir-grant-'));
    fs.cpSync(root, dir, { recursive: true, filter: (src) => !/node_modules|dist|test-results|\.timestamp-/.test(src) });
    fs.symlinkSync(path.join(root, 'node_modules'), path.join(dir, 'node_modules'));
    const p = path.join(dir, 'src/packages/image/choir-package.json');
    const m = JSON.parse(fs.readFileSync(p, 'utf8'));
    m.capabilities = [grant];
    fs.writeFileSync(p, JSON.stringify(m, null, 2));
    const r = spawnSync(process.execPath, ['scripts/check-packages.mjs'], { cwd: dir, encoding: 'utf8' });
    fs.rmSync(dir, { recursive: true, force: true });
    return { status: r.status, out: r.stdout + r.stderr };
  };
  expect(run({ grant: 'agent.prompt' }).out).toMatch(/not a grant in the catalogue/);
  expect(run({ grant: 'ui.invented' }).out).toMatch(/not a grant in the catalogue/);
  expect(run({ grant: 'ui.shell', args: ['x'] }).out).toMatch(/takes no argument/);
  expect(run({ grant: 'ui.net.egress' }).out).toMatch(/exactly one argument/);
  expect(run({ grant: 'ui.captures.read' }).status).toBe(0);
});

test('the lock diff names a section entry that left, moved or joined', () => {
  const surface = extractSurface({ frontendRoot: root });
  const clone = (o) => JSON.parse(JSON.stringify(o));
  const gone = clone(surface); delete gone.sections.pipelines['files.openWith'];
  expect(compareSurfaces(surface, gone, { frontendRoot: root })).toMatchObject({ required: 'major', removed: ['section:pipelines.files.openWith.payload'] });
  const moved = clone(surface); moved.sections.services.theme.abi = 'choir.service/theme/v2';
  expect(compareSurfaces(surface, moved, { frontendRoot: root }).required).toBe('major');
  const joined = clone(surface); joined.sections.ctx.push('zzz');
  expect(compareSurfaces(surface, joined, { frontendRoot: root })).toMatchObject({ required: 'minor', added: ['section:ctx.zzz'] });
  const ctxGone = clone(surface); ctxGone.sections.ctx = ctxGone.sections.ctx.filter((k) => k !== 'sdk');
  expect(compareSurfaces(surface, ctxGone, { frontendRoot: root }).required).toBe('major');
});
