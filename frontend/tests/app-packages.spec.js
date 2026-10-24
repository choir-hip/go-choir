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
  ['import-escape', (root) => fs.appendFileSync(path.join(root, 'src/packages/image/App.svelte'), "\n<script>import x from '../../lib/auth.js';</script>\n"), /leaves the package/],
  ['import-sdk-wrong-path', (root) => fs.appendFileSync(path.join(root, 'src/packages/image/App.svelte'), "\n<script>import x from '../../lib/media-utils.js';</script>\n"), /leaves the package/],
  ['import-bare-module', (root) => fs.appendFileSync(path.join(root, 'src/packages/image/App.svelte'), "\n<script>import x from 'lodash';</script>\n"), /not svelte, a declared dependency/],
  ['import-dynamic', (root) => fs.writeFileSync(path.join(root, 'src/packages/image/helper.js'), "export const f = () => import('../../lib/auth.js');\n"), /leaves the package/],
  ['import-reexport', (root) => fs.writeFileSync(path.join(root, 'src/packages/image/helper.ts'), "export * from '../../lib/auth.js';\n"), /leaves the package/],
  ['symlink-out', (root) => fs.symlinkSync(path.join(root, 'src/lib/app-sdk.ts'), path.join(root, 'src/packages/image/link.js')), /symlinks are not allowed/],
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
});
