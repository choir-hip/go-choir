// First-party app packages, slice P1
// (docs/design/app-packages-replan-2026-10-24.md §3.1 and §3.3).
//
// The first block runs scripts/check-packages.mjs against fixture trees and
// needs no server. The second block needs the shell (vite dev or a built
// preview) at PLAYWRIGHT_BASE_URL, default http://localhost:5173.
import { test, expect } from '@playwright/test';
import { spawnSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { build } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';
import packageBoundary from '../scripts/package-boundary-plugin.mjs';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const frontendRoot = path.resolve(__dirname, '..');
const BASE_URL = process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:5173';

// ---- the check script, against fixture trees ----

function makeFixture() {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'choir-pkg-'));
  for (const rel of ['package.json', 'src/lib/apps/registry.ts', 'src/lib/media-utils.js']) {
    fs.mkdirSync(path.dirname(path.join(root, rel)), { recursive: true });
    fs.copyFileSync(path.join(frontendRoot, rel), path.join(root, rel));
  }
  fs.mkdirSync(path.join(root, 'src/lib'), { recursive: true });
  fs.writeFileSync(path.join(root, 'src/lib/app-sdk.ts'), 'export {};\n');
  fs.cpSync(path.join(frontendRoot, 'src/packages/image'), path.join(root, 'src/packages/image'), { recursive: true });
  return root;
}

function imageManifest(root) {
  return JSON.parse(fs.readFileSync(path.join(root, 'src/packages/image/choir-package.json'), 'utf8'));
}

function writeManifest(root, dir, manifest) {
  fs.mkdirSync(path.join(root, 'src/packages', dir), { recursive: true });
  fs.writeFileSync(path.join(root, 'src/packages', dir, 'choir-package.json'), typeof manifest === 'string' ? manifest : JSON.stringify(manifest));
  const app = path.join(root, 'src/packages', dir, 'App.svelte');
  if (!fs.existsSync(app)) fs.writeFileSync(app, '<div>hi</div>\n');
}

function runCheck(root) {
  const r = spawnSync(process.execPath, [path.join(frontendRoot, 'scripts/check-packages.mjs'), '--root', root], { encoding: 'utf8' });
  return { code: r.status, out: `${r.stdout}${r.stderr}` };
}

// Each case mutates a clean fixture and must fail with the stated text.
const badCases = [
  ['bad-json', (root) => writeManifest(root, 'broken', '{not json'), /not valid JSON/],
  ['unknown-field', (root) => writeManifest(root, 'extra', { ...imageManifest(root), name: 'extra', surprise: 1 }), /unknown field "surprise"/],
  ['wrong-schema', (root) => writeManifest(root, 'extra', { ...imageManifest(root), name: 'extra', schema: 'nope/v0', opens: [] }), /schema must be/],
  ['bad-name', (root) => writeManifest(root, 'Bad_Name', { ...imageManifest(root), name: 'Bad_Name', opens: [] }), /name must match/],
  ['dir-mismatch', (root) => writeManifest(root, 'other', { ...imageManifest(root), name: 'another', opens: [] }), /must equal its directory/],
  ['static-id', (root) => writeManifest(root, 'audio', { ...imageManifest(root), name: 'audio', opens: [] }), /"audio" is a static registry id/],
  ['static-id-alias', (root) => {
    const m = imageManifest(root);
    m.app.aliases = ['pdf'];
    fs.writeFileSync(path.join(root, 'src/packages/image/choir-package.json'), JSON.stringify(m));
  }, /"pdf" is a static registry id/],
  ['texture-static-id', (root) => writeManifest(root, 'texture', { ...imageManifest(root), name: 'texture', opens: [] }), /"texture" is a static registry id/],
  ['reserved', (root) => writeManifest(root, 'management', { ...imageManifest(root), name: 'management', opens: [] }), /reserved shell or system-desk name/],
  ['package-alias-clash', (root) => {
    const m = { ...imageManifest(root), name: 'viewer', opens: [] };
    m.app = { ...m.app, aliases: ['image'] };
    writeManifest(root, 'viewer', m);
  }, /"image" is already held by package "image"/],
  ['extension-clash-package', (root) => writeManifest(root, 'viewer', { ...imageManifest(root), name: 'viewer' }), /extension "png" is already opened by package "image"/],
  ['extension-clash-static', (root) => writeManifest(root, 'viewer', { ...imageManifest(root), name: 'viewer', opens: [{ media_type: 'application/pdf', extensions: ['pdf'] }] }), /extension "pdf" is still routed by the static/],
  ['missing-entry', (root) => fs.rmSync(path.join(root, 'src/packages/image/App.svelte')), /missing App\.svelte/],
  ['bad-abi', (root) => {
    const m = imageManifest(root);
    m.app.abi = 'something/else';
    fs.writeFileSync(path.join(root, 'src/packages/image/choir-package.json'), JSON.stringify(m));
  }, /abi must be/],
  ['kinds', (root) => {
    const m = imageManifest(root);
    m.kinds = [{ kind: 'choir.app.image.thing' }];
    fs.writeFileSync(path.join(root, 'src/packages/image/choir-package.json'), JSON.stringify(m));
  }, /kinds must be empty in P1/],
  ['desk', (root) => {
    const m = imageManifest(root);
    m.desk = { name: 'image' };
    fs.writeFileSync(path.join(root, 'src/packages/image/choir-package.json'), JSON.stringify(m));
  }, /desk must be null/],
  // F3: only real directories under src/packages.
  ['symlinked-package-dir', (root) => {
    fs.cpSync(path.join(root, 'src/packages/image'), path.join(root, 'elsewhere'), { recursive: true });
    fs.symlinkSync(path.join(root, 'elsewhere'), path.join(root, 'src/packages/linked'));
  }, /linked: must be a real directory/],
  ['stray-file-in-packages', (root) => fs.writeFileSync(path.join(root, 'src/packages/README.md'), 'hi\n'), /README\.md: must be a real directory/],
  // F5: the required-package floor.
  ['required-package-missing', (root) => {
    fs.cpSync(path.join(root, 'src/packages/image'), path.join(root, 'src/packages/viewer'), { recursive: true });
    const m = imageManifest(root);
    m.name = 'viewer';
    m.opens = [];
    fs.writeFileSync(path.join(root, 'src/packages/viewer/choir-package.json'), JSON.stringify(m));
    fs.rmSync(path.join(root, 'src/packages/image'), { recursive: true });
  }, /required package "image" is missing/],
  ['dir-without-manifest', (root) => {
    fs.mkdirSync(path.join(root, 'src/packages/ghost'), { recursive: true });
    fs.writeFileSync(path.join(root, 'src/packages/ghost/App.svelte'), '<div/>\n');
  }, /ghost: missing choir-package\.json/],
  ['manifest-filtered-away', (root) => fs.rmSync(path.join(root, 'src/packages/image/choir-package.json')), /image: missing choir-package\.json/],
  ['zero-packages', (root) => fs.rmSync(path.join(root, 'src/packages/image'), { recursive: true }), /holds no package/],
  ['empty-registry', (root) => fs.writeFileSync(path.join(root, 'src/lib/apps/registry.ts'), 'export const x = 1;\n'), /static app ids \(floor/],
];

test.describe('check-packages.mjs', () => {
  test('passes on the real tree', () => {
    const r = runCheck(frontendRoot);
    expect(r.out).toContain('check-packages:');
    expect(r.code).toBe(0);
    expect(r.out).toContain('image');
  });

  test('passes on a clean fixture and on a second well-formed package', () => {
    const root = makeFixture();
    expect(runCheck(root).code).toBe(0);
    const m = imageManifest(root);
    m.name = 'viewer2';
    m.opens = [{ media_type: 'image/x-test', extensions: ['xtest'] }];
    m.app.theme = { ...m.app.theme, shellDataAttr: 'data-viewer2-window' };
    writeManifest(root, 'viewer2', m);
    const r = runCheck(root);
    expect(r.out).toContain('2 package(s) ok');
    expect(r.code).toBe(0);
  });

  for (const [label, mutate, pattern] of badCases) {
    test(`refuses ${label}`, () => {
      const root = makeFixture();
      mutate(root);
      const r = runCheck(root);
      expect(r.code, r.out).toBe(1);
      expect(r.out).toMatch(pattern);
    });
  }
});


// ---- the package boundary plugin, on the resolved module graph ----
//
// Each row builds a tiny project with the real plugin and must fail (or, for the
// controls, pass). Rows are the bypasses found in review (bunch201, bunch203).

const PNG = Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==', 'base64');

function makeProject(files) {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'choir-boundary-'));
  fs.symlinkSync(path.join(frontendRoot, 'node_modules'), path.join(root, 'node_modules'));
  fs.copyFileSync(path.join(frontendRoot, 'package.json'), path.join(root, 'package.json'));
  const all = {
    'main.js': "import('./src/packages/p/App.svelte').then((m) => console.log(m));\n",
    'src/lib/app-sdk.ts': 'export const sdk = 1;\n',
    'src/lib/secret.js': 'export const secret = 42;\n',
    'src/lib/secret.css': '.secret { color: red; }\n',
    'src/packages/q/helper.js': 'export const other = 1;\n',
    'src/packages/p/local.js': 'export const local = 1;\n',
    'src/packages/p/App.svelte': '<div>ok</div>\n',
    ...files,
  };
  for (const [rel, body] of Object.entries(all)) {
    fs.mkdirSync(path.dirname(path.join(root, rel)), { recursive: true });
    fs.writeFileSync(path.join(root, rel), body);
  }
  fs.writeFileSync(path.join(root, 'src/lib/secret.png'), PNG);
  fs.writeFileSync(path.join(root, 'src/packages/p/pic.png'), PNG);
  return root;
}

async function buildProject(root) {
  await build({
    root,
    configFile: false,
    logLevel: 'silent',
    plugins: [packageBoundary({ root }), svelte({ configFile: false, compilerOptions: { css: 'injected' } })],
    build: { write: false, minify: false, rollupOptions: { input: path.join(root, 'main.js') } },
  });
}

const script = (body) => ({ 'src/packages/p/App.svelte': `<script>\n${body}\n</script>\n<div>ok</div>\n` });
const style = (css) => ({ 'src/packages/p/App.svelte': `<div class="a">ok</div>\n<style>\n${css}\n</style>\n` });
const outside = /may not import/;

const boundaryControls = [
  ['own files, sdk, svelte, svelte/store, a declared dependency, own asset',
    script("import { local } from './local.js';\nimport { sdk } from '../../lib/app-sdk';\nimport { onMount } from 'svelte';\nimport { writable } from 'svelte/store';\nimport JSZip from 'jszip';\nimport pic from './pic.png';\nimport s from './local.js?raw';\nconsole.log(local, sdk, onMount, writable, JSZip, pic, s, import.meta.env);\nimport('./local.js').then(console.log);")],
  ['own css, data: urls and fragments', style('.a { color: red; background: url(data:image/png;base64,AAAA); fill: url(#grad); }')],
];

const boundaryBad = [
  // static, whitespace and comment forms (bunch203 N1)
  ['static import', script("import { secret } from '../../lib/secret.js';\nconsole.log(secret);"), outside],
  ['import with no spaces', script("import{secret}from'../../lib/secret.js';console.log(secret);"), outside],
  ['bare side-effect import', script("import'../../lib/secret.js';"), outside],
  ['comment-split import', { ...script("import/**/a/**/from/**/'../../lib/secret.js';console.log(a);") }, outside],
  ['unicode binding', script("import π from'../../lib/secret.js';console.log(π);"), outside],
  ['re-export star in a helper', { ...script("import './h.js';"), 'src/packages/p/h.js': "export*from'../../lib/secret.js';\n" }, outside],
  ['re-export star in a .tsx helper', { ...script("import './h.tsx';"), 'src/packages/p/h.tsx': "export*from'../../lib/secret.js';\n" }, outside],
  ['dynamic import, comment before paren', script("import/*c*/('../../lib/secret.js').then(console.log);"), outside],
  ['dynamic import with a variable', script("const p = '../../lib/secret.js';\nimport(/* @vite-ignore */ p).then(console.log);"), /must take a string literal/],
  ['dynamic import with a template', script("export let n = 'secret';\nimport(`../../lib/${n}.js`).then(console.log);"), outside],
  ['import.meta.glob eager', script("const m = import.meta.glob('../../lib/*.js', { eager: true });\nconsole.log(m);"), outside],
  ['import.meta.glob with ?raw', script("const m = import.meta.glob('../../lib/*.js', { query: '?raw', eager: true });\nconsole.log(m);"), outside],
  ['import.meta.resolve', script("console.log(import.meta.resolve('../../lib/secret.js'));"), /import\.meta is only allowed/],
  ['import.meta computed member', script("console.log(import.meta['resolve']('../../lib/secret.js'));"), /import\.meta is only allowed/],
  ['import.meta destructured', script("const { glob } = import.meta;\nconsole.log(glob);"), /import\.meta is only allowed/],
  ['import.meta aliased', script("const m = import.meta;\nconsole.log(m.resolve('x'));"), /import\.meta is only allowed/],
  // the other query forms, absolute and bare traversal
  ['?raw of an outside file', script("import s from '../../lib/secret.js?raw';console.log(s);"), outside],
  ['?url of an outside file', script("import u from '../../lib/secret.png?url';console.log(u);"), outside],
  ['root-absolute path', script("import { secret } from '/src/lib/secret.js';console.log(secret);"), outside],
  ['bare path traversal', script("import { secret } from 'svelte/../../src/lib/secret.js';console.log(secret);"), /.+/],
  ['another package', script("import { other } from '../q/helper.js';console.log(other);"), outside],
  ['an undeclared dependency', script("import { build } from 'vite';console.log(build);"), outside],
  // runtime reach
  ['eval', script("eval('1 + 1');"), /"eval" is not allowed/],
  ['eval of raw text', { ...script("import raw from './local.js?raw';\neval(raw);") }, /"eval" is not allowed/],
  ['new Function', script("new Function('return 1')();"), /"Function" is not allowed/],
  ['Function call', script("Function('return 1')();"), /"Function" is not allowed/],
  ['setTimeout with a string', script("setTimeout('console.log(1)', 0);"), /setTimeout with a string/],
  ['new URL with import.meta.url', script("console.log(new URL('../../lib/secret.js', import.meta.url).href);"), /import\.meta is only allowed|outside/],
  ['new Worker with import.meta.url', script("new Worker(new URL('../../lib/secret.js', import.meta.url), { type: 'module' });"), /"Worker" is not allowed|import\.meta is only allowed/],
  // CSS (bunch203 N4)
  ['css @import quoted', style("@import '../../lib/secret.css';\n.a { color: red; }"), /@import is not allowed/],
  ['css @import url unquoted', style("@import url(../../lib/secret.css);\n.a { color: red; }"), /@import is not allowed/],
  ['css @IMPORT upper case', style("@IMPORT '../../lib/secret.css';\n.a { color: red; }"), /@import is not allowed/],
  ['css @import no space', style("@import'../../lib/secret.css';\n.a { color: red; }"), /@import is not allowed/],
  ['css escaped @import', style("@\\69mport '../../lib/secret.css';\n.a { color: red; }"), /@import is not allowed/],
  ['css url() outside', style('.a { background: url(../../lib/secret.png); }'), /url\(\) in package CSS/],
  ['css url() quoted, upper case', style(".a { background: URL('../../lib/secret.png'); }"), /url\(\) in package CSS/],
  ['css .css file import outside', { ...script("import '../../lib/secret.css';") }, outside],
  ['css file in the package with @import', { ...script("import './x.css';"), 'src/packages/p/x.css': "@import '../../lib/secret.css';\n" }, /@import is not allowed/],
];

test.describe('package boundary plugin', () => {
  for (const [label, files] of boundaryControls) {
    test(`allows: ${label}`, async () => {
      const root = makeProject(files);
      await buildProject(root);
    });
  }
  for (const [label, files, pattern] of boundaryBad) {
    test(`refuses: ${label}`, async () => {
      const root = makeProject(files);
      let message = '';
      try {
        await buildProject(root);
      } catch (e) {
        message = String(e?.message || e);
      }
      expect(message, 'the build should have failed').not.toBe('');
      if (process.env.SHOW_BOUNDARY) console.log(`${label} => ${message.split('\n')[0].slice(0, 230)}`);
      expect(message).toMatch(pattern);
    });
  }

  test('a module outside any package may import anything', async () => {
    const root = makeProject({ 'main.js': "import { secret } from './src/lib/secret.js';\nconsole.log(secret);\n" });
    await buildProject(root);
  });
});

// ---- the shell: the image viewer as a package ----

// The static entry the image viewer had before it became a package (registry.ts at origin/main e2d8fca61).
const IMAGE_BEFORE = {
  id: 'image',
  name: 'Image',
  icon: '🖼️',
  description: 'Image viewer',
  launcher: { menu: true, desktopIcon: false, order: 80 },
  window: { singleton: false, heavy: true, desktop: { width: 900, height: 680 }, compact: { minWidth: 280, minHeight: 420 } },
  auth: { preview: 'public-preview' },
  theme: { surface: 'media', shellDataAttr: 'data-image-window', contentClass: 'image-content' },
};

// No local service stack: answer every auth and api call as "not signed in",
// which is the logged-out shell with local previews.
async function stubLoggedOutBackend(page) {
  await page.route(/\/(auth|api)\//, (route) =>
    route.fulfill({ status: 401, contentType: 'application/json', body: '{"error":"unauthenticated"}' }),
  );
}

test.describe('image viewer package in the shell', () => {
  test('registry definition equals the former static entry; file routes keep their ids', async ({ page }) => {
    await page.goto(BASE_URL);
    const result = await page.evaluate(async () => {
      const reg = await import('/src/lib/apps/registry.ts');
      const media = await import('/src/lib/media-utils.js');
      const def = reg.getAppDefinition('image');
      const { component, ...rest } = def;
      return {
        def: rest,
        hasComponent: typeof component === 'function',
        ids: reg.APP_REGISTRY.map((a) => a.id),
        launcher: reg.LAUNCHER_APPS.map((a) => a.id),
        desktopIcons: reg.DESKTOP_ICON_APPS.map((a) => a.id),
        heavy: reg.isHeavyAppId('image'),
        icon: reg.getAppIcon('image'),
        routes: ['a.png', 'b.JPG', 'c.svg', 'd.webp', 'e.pdf', 'f.epub', 'g.mp3', 'h.ics'].map((n) => media.mediaRouteForFileName(n)),
      };
    });
    expect(result.hasComponent).toBe(true);
    expect(result.def).toEqual(IMAGE_BEFORE);
    expect(result.icon).toBe('🖼️');
    expect(result.heavy).toBe(true);
    expect(result.ids.filter((id) => id === 'image')).toHaveLength(1);
    expect(result.launcher).toContain('image');
    expect(result.desktopIcons).not.toContain('image');
    // Launcher order is by `order`: image (80) sits between podcast (70) and audio (90).
    expect(result.launcher.slice(result.launcher.indexOf('podcast'), result.launcher.indexOf('audio') + 1)).toEqual(['podcast', 'image', 'audio']);
    expect(result.routes).toEqual([
      { appId: 'image', mediaType: 'image/png' },
      { appId: 'image', mediaType: 'image/png' },
      { appId: 'image', mediaType: 'image/svg+xml' },
      { appId: 'image', mediaType: 'image/png' },
      { appId: 'pdf', mediaType: 'application/pdf' },
      { appId: 'epub', mediaType: 'application/epub+zip' },
      { appId: 'audio', mediaType: 'audio/mpeg' },
      { appId: 'calendar', mediaType: 'text/calendar' },
    ]);
  });

  test('opens from the launcher with the same window kind', async ({ page }) => {
    const errors = [];
    page.on('console', (m) => {
      if (m.type() === 'error' || /\[packages\]/.test(m.text())) errors.push(m.text());
    });
    page.on('pageerror', (e) => errors.push(String(e)));
    await stubLoggedOutBackend(page);
    await page.goto(BASE_URL);
    await page.locator('[data-desk-menu-button]').click();
    await expect(page.locator('[data-desk-sheet]')).toBeVisible();
    const entry = page.locator('[data-desk-sheet-app][data-desk-app-id="image"]');
    await expect(entry).toBeVisible();
    await expect(entry).toContainText('Image');
    await entry.click();
    const win = page.locator('[data-window][data-window-app-id="image"]');
    await expect(win).toBeVisible();
    await expect(win.locator('[data-image-window], .image-content').first()).toBeVisible();
    expect(errors.filter((e) => /\[packages\]/.test(e))).toEqual([]);
  });

  test('opens from a file route (the path the Files app takes) with the same ids', async ({ page }) => {
    await stubLoggedOutBackend(page);
    const png = Buffer.from(
      'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==',
      'base64',
    );
    await page.route('https://example.com/**', (route) => route.fulfill({ status: 200, contentType: 'image/png', body: png }));
    await page.goto(BASE_URL);
    await expect(page.locator('[data-desk-menu-button]')).toBeVisible();
    const opened = await page.evaluate(async () => {
      const media = await import('/src/lib/media-utils.js');
      const desktop = await import('/src/lib/stores/desktop.js');
      const route = media.mediaRouteForFileName('holiday.png');
      desktop.openApp(route.appId, 'Image', desktop.getAppIcon(route.appId), {
        mediaType: route.mediaType,
        sourceUrl: 'https://example.com/holiday.png',
        fileName: 'holiday.png',
      });
      return route.appId;
    });
    expect(opened).toBe('image');
    const win = page.locator('[data-window][data-window-app-id="image"]');
    await expect(win).toBeVisible();
    await expect(win.locator('[data-image-viewer]')).toHaveAttribute('src', 'https://example.com/holiday.png');
  });

  // F4: the loader must never crash the shell. Needs `vite dev`: the test writes
  // a malformed package into src/packages for its duration and removes it after.
  test('a malformed package is skipped with a warning and the shell still works', async ({ page, request }) => {
    const dev = await request.get(`${BASE_URL}/src/lib/apps/packages.ts`);
    test.skip(!dev.ok(), 'needs the vite dev server (the glob must see a new package directory)');
    const badDir = path.join(frontendRoot, 'src/packages/zz-bad-fixture');
    const bad = {
      ...JSON.parse(fs.readFileSync(path.join(frontendRoot, 'src/packages/image/choir-package.json'), 'utf8')),
      name: 'zz-bad-fixture',
      app: { entry: 'App.svelte', aliases: 'not-an-array', launcher: 7 },
      opens: [{ media_type: 'x/y', extensions: 'zzz' }],
    };
    fs.mkdirSync(badDir, { recursive: true });
    try {
      fs.writeFileSync(path.join(badDir, 'choir-package.json'), JSON.stringify(bad));
      fs.writeFileSync(path.join(badDir, 'App.svelte'), '<div>bad</div>\n');
      await page.waitForTimeout(1500); // let vite's watcher see the new directory
      const warnings = [];
      const crashes = [];
      page.on('console', (m) => {
        if (/\[packages\]/.test(m.text())) warnings.push(m.text());
      });
      page.on('pageerror', (e) => crashes.push(String(e)));
      await stubLoggedOutBackend(page);
      await page.goto(BASE_URL);
      await page.locator('[data-desk-menu-button]').click();
      await expect(page.locator('[data-desk-sheet-app][data-desk-app-id="image"]')).toBeVisible();
      await expect(page.locator('[data-desk-sheet-app][data-desk-app-id="zz-bad-fixture"]')).toHaveCount(0);
      expect(warnings.join('\n')).toContain('zz-bad-fixture');
      expect(crashes).toEqual([]);
    } finally {
      fs.rmSync(badDir, { recursive: true, force: true });
    }
  });
});
