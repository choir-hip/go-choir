// Runtime install (S12; docs/design/s12-runtime-install-build-2026-11-04.md).
//
// No server: the spec runs the computer's own module build
// (internal/pkgruntime/build-module.mjs, the script the Go builder embeds)
// on a fixture package and on every in-tree first-party package, then loads
// the fixture's module in Chromium through the shell's real loader
// (src/lib/apps/packages.ts loadPackageModule) and mounts it with a ctx the
// way PackageAppFrame does. The installed-list and module routes are served
// by page.route from the built bytes.
//
// Failure modes, written before the code:
//   B1  a first-party package does not build to one module/v1 module;
//   B2  the module bundles the shell SDK instead of reading ctx.sdk;
//   L1  bytes whose SHA-256 is not the listed digest run;
//   L2  a missing module reads as anything but unavailable;
//   M1  the mounted app does not render, cannot reach the SDK, or loses its CSS;
//   M2  destroy leaves the app's DOM or its style element behind.
//
// Artifact: <output dir>/runtime-install-receipt.json (module digests and
// sizes, the checks) and runtime-install-mounted.png.
import { test, expect } from '@playwright/test';
import { spawnSync } from 'node:child_process';
import { createHash } from 'node:crypto';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { build } from 'vite';
import { svelte, vitePreprocess } from '@sveltejs/vite-plugin-svelte';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const frontendRoot = path.resolve(__dirname, '..');
const repoRoot = path.resolve(frontendRoot, '..');
const runtimeDir = path.join(repoRoot, 'internal/pkgruntime');
// https: WebCrypto (the digest check) exists only in a secure context, as on choir.news.
const ORIGIN = 'https://choir.test';

const FIXTURE_MANIFEST = {
  schema: 'choir.package/v2', name: 'fixture', version: '1.0.0',
  display: { title: 'Fixture', icon: 'F', description: 'Runtime install fixture' },
  app: {
    entry: 'App.svelte', abi: 'choir.app-frontend/svelte-inline/v1', aliases: [], requires_sdk: '^1.0',
    launcher: { menu: true, desktopIcon: false, order: 900 }, window: { singleton: false, heavy: false },
    auth: { preview: 'private' }, theme: { surface: 'standard', shellDataAttr: 'data-fixture-window', contentClass: 'fixture-content' },
  },
  opens: [], kinds: [], uses_kinds: [], contributes: { desks: [] }, capabilities: [],
};

const FIXTURE_APP = `<script>
  import { onMount } from 'svelte';
  import { appTitle } from '../../lib/app-sdk';
  import { label } from './label.js';
  export let windowId = '';
  let mounted = false;
  onMount(() => { mounted = true; });
</script>

<section data-fixture-app data-window={windowId}>
  <h2>{label}</h2>
  <p data-fixture-title>{appTitle('fixture')}</p>
  {#if mounted}<span data-fixture-mounted>mounted</span>{/if}
</section>

<style>
  section { padding: 17px; }
</style>
`;

// buildModule lays out a private frontend root as internal/pkgruntime does
// (the package, the locked package.json and node_modules, the two scripts)
// and runs the embedded build.
function buildModule(name, files) {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'choir-rt-'));
  const front = path.join(tmp, 'frontend');
  const pkg = path.join(front, 'src/packages', name);
  fs.mkdirSync(pkg, { recursive: true });
  for (const [rel, content] of Object.entries(files)) {
    fs.mkdirSync(path.dirname(path.join(pkg, rel)), { recursive: true });
    fs.writeFileSync(path.join(pkg, rel), content);
  }
  for (const script of ['build-module.mjs', 'package-boundary-plugin.mjs', 'legacy-events.json', 'css-fetch-functions.json']) fs.copyFileSync(path.join(runtimeDir, script), path.join(front, script));
  fs.copyFileSync(path.join(frontendRoot, 'package.json'), path.join(front, 'package.json'));
  fs.symlinkSync(path.join(frontendRoot, 'node_modules'), path.join(front, 'node_modules'));
  const out = path.join(tmp, 'module.js');
  const manifest = JSON.parse(files['choir-package.json']);
  const r = spawnSync(process.execPath, [path.join(front, 'build-module.mjs'), front, name, out, manifest.app.requires_sdk || '^1.0'], { encoding: 'utf8', cwd: front });
  const bytes = r.status === 0 ? fs.readFileSync(out) : null;
  fs.rmSync(tmp, { recursive: true, force: true });
  return { code: r.status, log: `${r.stdout}${r.stderr}`, bytes };
}

function readPackage(name) {
  const dir = path.join(frontendRoot, 'src/packages', name);
  const files = {};
  for (const rel of fs.readdirSync(dir, { recursive: true }).map(String)) {
    if (fs.statSync(path.join(dir, rel)).isFile()) files[rel] = fs.readFileSync(path.join(dir, rel), 'utf8');
  }
  return files;
}

const sha256 = (bytes) => createHash('sha256').update(bytes).digest('hex');

test.describe('runtime install: module/v1 build and verified load', () => {
  test.skip(!fs.existsSync(path.join(frontendRoot, 'node_modules/vite')), 'frontend/node_modules is not installed');

  const receipt = { builds: {}, checks: [] };
  let fixture;
  let harness;

  test.beforeAll(async () => {
    fixture = buildModule('fixture', {
      'choir-package.json': JSON.stringify(FIXTURE_MANIFEST),
      'App.svelte': FIXTURE_APP,
      'label.js': "export const label = 'Fixture package';\n",
    });
    // The harness is the shell's real loader, bundled for the page.
    const entry = path.join(os.tmpdir(), `choir-rt-harness-${process.pid}.js`);
    fs.writeFileSync(entry, `
      import { loadPackageModule, PackageLoadError } from ${JSON.stringify(path.join(frontendRoot, 'src/lib/apps/packages.ts'))};
      import { SDK_VERSION, sdkSatisfies } from ${JSON.stringify(path.join(frontendRoot, 'src/lib/sdk-version.ts'))};
      window.__choirLoad = async (record) => {
        try {
          const mod = await loadPackageModule(record);
          return { ok: true, abi: mod.abi, requires_sdk: mod.requires_sdk, sdk_imports: mod.sdk_imports, inRange: sdkSatisfies(mod.requires_sdk), sdk: SDK_VERSION };
        } catch (err) {
          return { ok: false, state: err instanceof PackageLoadError ? err.state : 'thrown', message: String(err && err.message) };
        }
      };
      window.__choirMount = async (record) => {
        const mod = await loadPackageModule(record);
        const target = document.getElementById('target');
        const sdk = Object.freeze({ appTitle: (s) => 'SDK says ' + s });
        window.__choirHandle = mod.mount(target, { sdk, package: record.name, windowId: 'w-1', props: { windowId: 'w-1' }, dispatch: () => {} });
        return true;
      };
    `);
    const out = await build({
      root: frontendRoot, configFile: false, logLevel: 'error', publicDir: false,
      plugins: [svelte({ configFile: false, preprocess: vitePreprocess() })],
      build: { write: false, minify: false, rollupOptions: { input: entry, output: { format: 'es', entryFileNames: 'harness.js', chunkFileNames: '[name]-[hash].js' } } },
    });
    fs.rmSync(entry, { force: true });
    harness = new Map();
    for (const item of [out].flat().flatMap((o) => o.output)) harness.set(item.fileName, item.type === 'chunk' ? item.code : item.source);
  });

  test.afterAll(async ({}, testInfo) => {
    fs.writeFileSync(testInfo.outputPath('runtime-install-receipt.json'), JSON.stringify(receipt, null, 2));
  });

  test('every in-tree first-party package builds to one module/v1 module (B1, B2)', async () => {
    for (const name of fs.readdirSync(path.join(frontendRoot, 'src/packages'))) {
      const r = buildModule(name, readPackage(name));
      expect(r.code, `${name}: ${r.log}`).toBe(0);
      const code = r.bytes.toString('utf8');
      expect(code).toContain('"choir.app-frontend/module/v1"');
      expect(code).toMatch(/as mount\b/);
      expect(code).toMatch(/as sdk_imports\b/);
      // B2: the shell SDK file is never in the build (only the shim is).
      expect(code).not.toContain('MEDIA_FILE_ROUTES=');
      receipt.builds[name] = { sha256: sha256(r.bytes), bytes: r.bytes.length };
    }
    receipt.checks.push('B1', 'B2');
  });

  test('the shell loads only the verified bytes, mounts and destroys (L1, L2, M1, M2)', async ({ page }, testInfo) => {
    expect(fixture.code, fixture.log).toBe(0);
    const digest = sha256(fixture.bytes);
    receipt.builds.fixture = { sha256: digest, bytes: fixture.bytes.length };
    let served = fixture.bytes;
    await page.route(`${ORIGIN}/**`, async (route) => {
      const url = new URL(route.request().url());
      if (url.pathname === '/') return route.fulfill({ contentType: 'text/html', body: '<!doctype html><div id="target"></div><script type="module" src="/harness.js"></script>' });
      if (url.pathname === `/api/packages/modules/${digest}.js`) {
        return served ? route.fulfill({ contentType: 'text/javascript', body: served }) : route.fulfill({ status: 404, body: '{}' });
      }
      const file = url.pathname.slice(1);
      if (harness.has(file)) return route.fulfill({ contentType: 'text/javascript', body: harness.get(file) });
      return route.fulfill({ status: 404, body: '' });
    });
    await page.goto(`${ORIGIN}/`);
    await page.waitForFunction(() => typeof window.__choirLoad === 'function');
    const record = { name: 'fixture', install_id: 'i-1', release_id: 'r', requires_sdk: '^1.0', manifest: FIXTURE_MANIFEST, available: true, module: { abi: 'choir.app-frontend/module/v1', sha256: digest, url: `/api/packages/modules/${digest}.js` } };

    const loaded = await page.evaluate((r) => window.__choirLoad(r), record);
    expect(loaded, JSON.stringify(loaded)).toMatchObject({ ok: true, abi: 'choir.app-frontend/module/v1', requires_sdk: '^1.0', sdk_imports: ['appTitle'], inRange: true });

    // L1: one byte changed is refused, never run.
    served = Buffer.concat([fixture.bytes, Buffer.from('\nwindow.__tampered = true;\n')]);
    const tampered = await page.evaluate((r) => window.__choirLoad(r), record);
    expect(tampered).toMatchObject({ ok: false, state: 'refused' });
    expect(await page.evaluate(() => window.__tampered === true)).toBe(false);
    // L2
    served = null;
    expect(await page.evaluate((r) => window.__choirLoad(r), record)).toMatchObject({ ok: false, state: 'unavailable' });
    served = fixture.bytes;

    // M1
    await page.evaluate((r) => window.__choirMount(r), record);
    await expect(page.locator('[data-fixture-app] h2')).toHaveText('Fixture package');
    await expect(page.locator('[data-fixture-title]')).toHaveText('SDK says fixture');
    await expect(page.locator('[data-fixture-mounted]')).toBeVisible();
    await expect(page.locator('style[data-choir-package-style="fixture"]')).toHaveCount(1);
    expect(await page.locator('[data-fixture-app]').evaluate((el) => getComputedStyle(el).paddingTop)).toBe('17px');
    await page.screenshot({ path: testInfo.outputPath('runtime-install-mounted.png') });

    // M2
    await page.evaluate(() => window.__choirHandle.destroy());
    await expect(page.locator('[data-fixture-app]')).toHaveCount(0);
    await expect(page.locator('style[data-choir-package-style]')).toHaveCount(0);
    receipt.checks.push('L1', 'L2', 'M1', 'M2');
  });
});
