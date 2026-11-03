// Context menus, Open with, launch files and upload helpers (owner, 2026-11-03:
// "all apps that depend on files should support file upload! ... everything
// needs context menu right click").
//
// Written before the code. The ways it could fail:
//   C1  a menu opened near an edge spills off screen, or one taller than the
//       viewport cannot be scrolled;
//   C2  the menu cannot be reached or driven without a mouse (Shift+F10, the
//       Menu key, arrows, Enter, Escape), or focus is lost when it closes;
//   C3  a long press on touch opens nothing, or opens the menu and then also
//       clicks the thing under the finger;
//   C4  Escape, an outside press or choosing an item leaves the menu open;
//   O1  a package that declares it opens .pptx is refused because the built-in
//       Slides already routes .pptx, or, once accepted, it silently becomes the
//       default for .pptx;
//   O2  Open with does not list such a package, or lists the default twice;
//   O3  the package window opens but cannot learn which file it was given;
//   U1  the upload helper invents a route instead of the one Files uses (PUT
//       /api/files/<folder>/<name> with the device header), loses the folder,
//       or reports paths that were not stored.
//
// The first block runs scripts/check-packages.mjs and needs no server. The
// rest needs the shell (vite dev or a preview) at PLAYWRIGHT_BASE_URL, default
// http://localhost:5173; the Files E2E needs vite dev, since it adds a package.
import { test, expect } from '@playwright/test';
import { spawnSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const frontendRoot = path.resolve(__dirname, '..');
const BASE_URL = process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:5173';
const PPTX = 'application/vnd.openxmlformats-officedocument.presentationml.presentation';

// ---- O1: the manifest's "opens" ----

function makeFixture() {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'choir-openwith-'));
  for (const rel of ['package.json', 'src/lib/apps/registry.ts', 'src/lib/media-utils.js']) {
    fs.mkdirSync(path.dirname(path.join(root, rel)), { recursive: true });
    fs.copyFileSync(path.join(frontendRoot, rel), path.join(root, rel));
  }
  fs.cpSync(path.join(frontendRoot, 'src/packages/image'), path.join(root, 'src/packages/image'), { recursive: true });
  return root;
}

function deckManifest(name, opens) {
  const m = JSON.parse(fs.readFileSync(path.join(frontendRoot, 'src/packages/image/choir-package.json'), 'utf8'));
  m.name = name;
  m.display = { title: 'Deck', icon: '🎞️', description: 'Deck player' };
  m.app.theme = { ...m.app.theme, shellDataAttr: `data-${name}-window` };
  m.opens = opens;
  return m;
}

function writePackage(root, name, manifest) {
  const dir = path.join(root, 'src/packages', name);
  fs.mkdirSync(dir, { recursive: true });
  fs.writeFileSync(path.join(dir, 'choir-package.json'), JSON.stringify(manifest));
  fs.writeFileSync(path.join(dir, 'App.svelte'), '<div>deck</div>\n');
}

function runCheck(root) {
  const r = spawnSync(process.execPath, [path.join(frontendRoot, 'scripts/check-packages.mjs'), '--root', root], { encoding: 'utf8' });
  return { code: r.status, out: `${r.stdout}${r.stderr}` };
}

test.describe('manifest "opens" with default', () => {
  test('a package may offer .pptx under Open with while Slides keeps the default', () => {
    const root = makeFixture();
    writePackage(root, 'deck', deckManifest('deck', [{ media_type: PPTX, extensions: ['pptx', 'pdf'], default: false }]));
    writePackage(root, 'deck2', deckManifest('deck2', [{ media_type: PPTX, extensions: ['pptx'], default: false }]));
    const r = runCheck(root);
    expect(r.out).toContain('3 package(s) ok');
    expect(r.code).toBe(0);
  });

  test('a default route on a statically routed extension is still refused', () => {
    const root = makeFixture();
    writePackage(root, 'deck', deckManifest('deck', [{ media_type: PPTX, extensions: ['pptx'] }]));
    const r = runCheck(root);
    expect(r.code).toBe(1);
    expect(r.out).toMatch(/extension "pptx" is still routed by the static/);
  });

  test('default must be a boolean, and an extension carries no dot', () => {
    const root = makeFixture();
    writePackage(root, 'deck', deckManifest('deck', [{ media_type: PPTX, extensions: ['pptx'], default: 'no' }]));
    writePackage(root, 'deck2', deckManifest('deck2', [{ media_type: PPTX, extensions: ['.pptx'], default: false }]));
    const r = runCheck(root);
    expect(r.code).toBe(1);
    expect(r.out).toMatch(/deck\/choir-package\.json opens\[0\]: default must be a boolean/);
    expect(r.out).toMatch(/deck2\/choir-package\.json opens\[0\]: extensions must be/);
  });
});

// ---- browser helpers ----

async function stubLoggedOutBackend(page) {
  await page.route(/\/(auth|api)\//, (route) =>
    route.fulfill({ status: 401, contentType: 'application/json', body: '{"error":"unauthenticated"}' }),
  );
}

async function openShell(page) {
  await stubLoggedOutBackend(page);
  await page.goto(BASE_URL);
  await expect(page.locator('[data-desk-menu-button]')).toBeVisible();
}

// A focusable target wired with the context-menu action, at (x, y).
async function mountTarget(page, { x = 200, y = 200, longPress = true } = {}) {
  await page.evaluate(async ({ x, y, longPress }) => {
    const sdk = await import('/src/lib/app-sdk.ts');
    document.querySelector('#cm-target')?.remove();
    const el = document.createElement('button');
    el.id = 'cm-target';
    el.textContent = 'target';
    Object.assign(el.style, { position: 'fixed', left: `${x}px`, top: `${y}px`, width: '120px', height: '40px', zIndex: 5 });
    document.body.appendChild(el);
    window.__picked = [];
    window.__clicks = 0;
    el.addEventListener('click', () => { window.__clicks += 1; });
    sdk.contextMenu(el, {
      label: 'Target actions',
      longPress,
      items: () => [
        { id: 'first', label: 'First', onSelect: () => window.__picked.push('first') },
        { id: 'off', label: 'Disabled', disabled: true },
        { separator: true },
        { id: 'more', label: 'More', items: [
          { id: 'sub-a', label: 'Sub A', onSelect: () => window.__picked.push('sub-a') },
          { id: 'sub-b', label: 'Sub B', onSelect: () => window.__picked.push('sub-b') },
        ] },
        { id: 'last', label: 'Last', danger: true, onSelect: () => window.__picked.push('last') },
      ],
    });
  }, { x, y, longPress });
}

const menu = (page) => page.locator('[data-context-menu]:not([data-context-submenu])');
const submenu = (page) => page.locator('[data-context-submenu]');

async function expectInsideViewport(page, locator) {
  const box = await locator.boundingBox();
  const vp = page.viewportSize();
  expect(box).not.toBeNull();
  expect(box.x).toBeGreaterThanOrEqual(0);
  expect(box.y).toBeGreaterThanOrEqual(0);
  expect(box.x + box.width).toBeLessThanOrEqual(vp.width);
  expect(box.y + box.height).toBeLessThanOrEqual(vp.height);
}

// ---- C1-C4: the context menu primitive ----

test.describe('context menu primitive', () => {
  test('right-click opens at the pointer; choosing an item runs it and closes', async ({ page }) => {
    await openShell(page);
    await mountTarget(page);
    await page.locator('#cm-target').click({ button: 'right' });
    await expect(menu(page)).toBeVisible();
    await expect(menu(page)).toHaveAttribute('role', 'menu');
    await expect(menu(page)).toHaveAttribute('aria-label', 'Target actions');
    const box = await menu(page).boundingBox();
    expect(Math.abs(box.x - 260)).toBeLessThan(4); // target center x = 200 + 60
    await expect(menu(page).locator('[data-context-menu-item="off"]')).toHaveAttribute('aria-disabled', 'true');
    await menu(page).locator('[data-context-menu-item="first"]').click();
    await expect(menu(page)).toHaveCount(0);
    expect(await page.evaluate(() => window.__picked)).toEqual(['first']);
  });

  test('Escape and an outside press close it; focus returns to the target', async ({ page }) => {
    await openShell(page);
    await mountTarget(page);
    await page.locator('#cm-target').focus();
    await page.keyboard.press('Shift+F10');
    await expect(menu(page)).toBeVisible();
    await page.keyboard.press('Escape');
    await expect(menu(page)).toHaveCount(0);
    await expect(page.locator('#cm-target')).toBeFocused();
    await page.locator('#cm-target').click({ button: 'right' });
    await expect(menu(page)).toBeVisible();
    await page.mouse.click(5, 5);
    await expect(menu(page)).toHaveCount(0);
  });

  test('keyboard: Shift+F10 and the Menu key open it; arrows skip disabled items; Enter runs; submenus open with ArrowRight', async ({ page }) => {
    await openShell(page);
    await mountTarget(page);
    await page.locator('#cm-target').focus();
    await page.keyboard.press('ContextMenu');
    await expect(menu(page)).toBeVisible();
    await expect(menu(page).locator('[data-context-menu-item="first"]')).toBeFocused();
    await page.keyboard.press('ArrowDown'); // skips the disabled item and the separator
    await expect(menu(page).locator('[data-context-menu-item="more"]')).toBeFocused();
    await expect(menu(page).locator('[data-context-menu-item="more"]')).toHaveAttribute('aria-haspopup', 'menu');
    await page.keyboard.press('ArrowRight');
    await expect(submenu(page)).toBeVisible();
    await expect(submenu(page).locator('[data-context-menu-item="sub-a"]')).toBeFocused();
    await page.keyboard.press('ArrowDown');
    await expect(submenu(page).locator('[data-context-menu-item="sub-b"]')).toBeFocused();
    await page.keyboard.press('ArrowLeft');
    await expect(submenu(page)).toHaveCount(0);
    await expect(menu(page).locator('[data-context-menu-item="more"]')).toBeFocused();
    await page.keyboard.press('End');
    await expect(menu(page).locator('[data-context-menu-item="last"]')).toBeFocused();
    await page.keyboard.press('ArrowDown'); // wraps
    await expect(menu(page).locator('[data-context-menu-item="first"]')).toBeFocused();
    await page.keyboard.press('ArrowUp');
    await page.keyboard.press('Enter');
    await expect(menu(page)).toHaveCount(0);
    expect(await page.evaluate(() => window.__picked)).toEqual(['last']);
    await expect(page.locator('#cm-target')).toBeFocused();

    await page.keyboard.press('Shift+F10');
    await expect(menu(page)).toBeVisible();
    await page.keyboard.press('ArrowDown');
    await page.keyboard.press('Enter'); // Enter on a submenu item opens it
    await expect(submenu(page)).toBeVisible();
    await page.keyboard.press('Enter');
    await expect(menu(page)).toHaveCount(0);
    expect(await page.evaluate(() => window.__picked)).toEqual(['last', 'sub-a']);
  });

  test('a menu opened at the corners stays inside the viewport, and so does its submenu', async ({ page }) => {
    await page.setViewportSize({ width: 800, height: 600 });
    await openShell(page);
    const vp = page.viewportSize();
    await mountTarget(page, { x: vp.width - 124, y: vp.height - 44 });
    await page.mouse.click(vp.width - 8, vp.height - 8, { button: 'right' });
    await expect(menu(page)).toBeVisible();
    await expectInsideViewport(page, menu(page));
    await menu(page).locator('[data-context-menu-item="more"]').hover();
    await expect(submenu(page)).toBeVisible();
    await expectInsideViewport(page, submenu(page));
    await page.keyboard.press('Escape');
    await page.keyboard.press('Escape');
    await expect(menu(page)).toHaveCount(0);

    // A menu taller than the viewport is clamped and scrolls.
    await page.setViewportSize({ width: 400, height: 220 });
    await page.evaluate(async () => {
      const sdk = await import('/src/lib/app-sdk.ts');
      const items = Array.from({ length: 30 }, (_, i) => ({ id: `row-${i}`, label: `Row ${i}` }));
      sdk.openContextMenu({ x: 390, y: 210, items });
    });
    await expect(menu(page)).toBeVisible();
    await expectInsideViewport(page, menu(page));
    const scrolls = await menu(page).evaluate((el) => el.scrollHeight > el.clientHeight);
    expect(scrolls).toBe(true);
  });

  test('placeMenu flips and clamps', async ({ page }) => {
    await openShell(page);
    const placed = await page.evaluate(async () => {
      const sdk = await import('/src/lib/app-sdk.ts');
      const vp = { width: 800, height: 600 };
      return [
        sdk.placeMenu({ x: 100, y: 100 }, { width: 200, height: 300 }, vp),
        sdk.placeMenu({ x: 790, y: 590 }, { width: 200, height: 300 }, vp),
        sdk.placeMenu({ x: 50, y: 50 }, { width: 900, height: 900 }, vp),
      ];
    });
    expect(placed[0]).toEqual({ x: 100, y: 100, maxHeight: 584 });
    expect(placed[1]).toEqual({ x: 590, y: 290, maxHeight: 584 });
    expect(placed[2]).toEqual({ x: 8, y: 8, maxHeight: 584 });
  });

  test('a long press on touch opens it and does not also click the target', async ({ page }) => {
    await openShell(page);
    await mountTarget(page);
    await page.evaluate(() => {
      const el = document.querySelector('#cm-target');
      const at = { clientX: 230, clientY: 215, bubbles: true, pointerType: 'touch', pointerId: 7, isPrimary: true };
      el.dispatchEvent(new PointerEvent('pointerdown', at));
      return new Promise((resolve) => setTimeout(() => {
        el.dispatchEvent(new PointerEvent('pointerup', at));
        el.dispatchEvent(new MouseEvent('click', { bubbles: true, clientX: 230, clientY: 215 }));
        resolve();
      }, 700));
    });
    await expect(menu(page)).toBeVisible();
    expect(await page.evaluate(() => window.__clicks)).toBe(0);
    // A press that moves is a drag or a scroll, not a long press.
    await page.keyboard.press('Escape');
    await page.evaluate(() => {
      const el = document.querySelector('#cm-target');
      const at = (x) => ({ clientX: x, clientY: 215, bubbles: true, pointerType: 'touch', pointerId: 8, isPrimary: true });
      el.dispatchEvent(new PointerEvent('pointerdown', at(230)));
      el.dispatchEvent(new PointerEvent('pointermove', at(260)));
      return new Promise((resolve) => setTimeout(resolve, 700));
    });
    await expect(menu(page)).toHaveCount(0);
  });

  test('a long press inside a nested target opens the innermost menu, and text fields keep the native menu', async ({ page }) => {
    await openShell(page);
    const result = await page.evaluate(async () => {
      const sdk = await import('/src/lib/app-sdk.ts');
      const outer = document.createElement('div');
      Object.assign(outer.style, { position: 'fixed', left: '20px', top: '300px', width: '300px', height: '120px' });
      const inner = document.createElement('div');
      Object.assign(inner.style, { width: '200px', height: '40px' });
      const field = document.createElement('input');
      inner.appendChild(field);
      outer.appendChild(inner);
      document.body.appendChild(outer);
      sdk.contextMenu(outer, [{ id: 'outer', label: 'Outer' }]);
      sdk.contextMenu(inner, [{ id: 'inner', label: 'Inner' }]);
      const at = { clientX: 40, clientY: 310, bubbles: true, pointerType: 'touch', pointerId: 9, isPrimary: true };
      inner.dispatchEvent(new PointerEvent('pointerdown', at));
      await new Promise((r) => setTimeout(r, 700));
      inner.dispatchEvent(new PointerEvent('pointerup', at));
      const ids = Array.from(document.querySelectorAll('[data-context-menu-item]')).map((el) => el.getAttribute('data-context-menu-item'));
      sdk.closeContextMenu();
      await new Promise((r) => setTimeout(r, 50));
      const ev = new MouseEvent('contextmenu', { bubbles: true, cancelable: true, clientX: 40, clientY: 310 });
      field.dispatchEvent(ev);
      await new Promise((r) => setTimeout(r, 50));
      return { ids, fieldPrevented: ev.defaultPrevented, menus: document.querySelectorAll('[data-context-menu]').length };
    });
    expect(result.ids).toEqual(['inner']);
    expect(result.fieldPrevented).toBe(false);
    expect(result.menus).toBe(0);
  });
});

// ---- shell surfaces wired to it ----

test.describe('shell context menus', () => {
  test('window title bar and taskbar chip offer Minimize, Maximize and Close', async ({ page }) => {
    await openShell(page);
    await page.locator('[data-desktop-icon][data-desktop-icon-id="files"]').click();
    const win = page.locator('[data-window][data-window-app-id="files"]');
    await expect(win).toBeVisible();
    await win.locator('[data-window-titlebar]').click({ button: 'right', position: { x: 40, y: 10 } });
    await expect(menu(page).locator('[data-context-menu-item="maximize"]')).toBeVisible();
    await menu(page).locator('[data-context-menu-item="maximize"]').click();
    await expect(win).toHaveAttribute('data-window-mode', 'maximized');
    const windowId = await win.getAttribute('data-window-id');
    const chip = page.locator(`[data-window-tray-item][data-window-id="${windowId}"]`);
    await chip.click({ button: 'right' });
    await expect(menu(page).locator('[data-context-menu-item="minimize"]')).toBeVisible();
    await menu(page).locator('[data-context-menu-item="minimize"]').click();
    await expect(win).toHaveAttribute('data-window-mode', 'minimized');
    await chip.click({ button: 'right' });
    await menu(page).locator('[data-context-menu-item="close"]').click();
    await expect(win).toHaveCount(0);
  });

  test('a desktop icon offers Open', async ({ page }) => {
    await openShell(page);
    await page.locator('[data-desktop-icon][data-desktop-icon-id="files"]').click({ button: 'right' });
    await menu(page).locator('[data-context-menu-item="open"]').click();
    await expect(page.locator('[data-window][data-window-app-id="files"]')).toBeVisible();
  });
});

// ---- O2, O3: Open with and the launch file ----

test.describe('open with', () => {
  test('lists a package that declares .pptx after the default, once each', async ({ page }) => {
    await openShell(page);
    const result = await page.evaluate(async (PPTX) => {
      const ow = await import('/src/lib/open-with.ts');
      const reg = await import('/src/lib/apps/registry.ts');
      const routes = [
        ...reg.PACKAGE_MEDIA_ROUTES,
        { appId: 'deckpkg', mediaType: PPTX, extensions: ['pptx'], default: false },
        { appId: 'deckpkg', mediaType: 'application/pdf', extensions: ['pdf'], default: false },
      ];
      const pick = (list) => list.map(({ appId, isDefault, via }) => ({ appId, isDefault, via }));
      return {
        pptx: pick(ow.openersForFile('Q3 review.PPTX', { packageRoutes: routes })),
        pdf: pick(ow.openersForFile('brief.pdf', { packageRoutes: routes })).map((o) => o.appId),
        md: pick(ow.openersForFile('notes.md', { packageRoutes: routes })),
        unknown: ow.openersForFile('blob.bin', { packageRoutes: routes }),
        realTree: ow.openersForFile('deck.pptx').map((o) => o.appId),
      };
    }, PPTX);
    expect(result.pptx).toEqual([
      { appId: 'slides', isDefault: true, via: 'media' },
      { appId: 'deckpkg', isDefault: false, via: 'media' },
    ]);
    expect(result.pdf[0]).toBe('reader');
    expect(result.pdf).toContain('deckpkg');
    expect(new Set(result.pdf).size).toBe(result.pdf.length);
    expect(result.md).toEqual([{ appId: 'texture', isDefault: true, via: 'text' }]);
    expect(result.unknown).toEqual([]);
    expect(result.realTree[0]).toBe('slides');
  });

  test('getLaunchFile reads the file the shell passed', async ({ page }) => {
    await openShell(page);
    const files = await page.evaluate(async (PPTX) => {
      const sdk = await import('/src/lib/app-sdk.ts');
      return [
        sdk.getLaunchFile({ filePath: 'Decks/Q3 review.pptx', fileName: 'Q3 review.pptx', mediaType: PPTX, openedWith: 'choice' }),
        sdk.getLaunchFile({ windowTitle: 'nothing' }),
        sdk.getLaunchFile(),
      ];
    }, PPTX);
    expect(files[0]).toEqual({
      path: 'Decks/Q3 review.pptx',
      name: 'Q3 review.pptx',
      mediaType: PPTX,
      url: '/api/files/Decks/Q3%20review.pptx?disposition=inline',
      sourceUrl: '',
      openedWith: 'choice',
    });
    expect(files[1]).toBeNull();
    expect(files[2]).toBeNull(); // outside a component there is no launch context
  });
});

// ---- U1: the upload helpers ----

test.describe('upload helpers', () => {
  test('pickAndUploadFiles opens the picker with accept and PUTs each file under the folder', async ({ page }) => {
    const puts = [];
    await page.route(/\/(auth|api)\//, async (route) => {
      const req = route.request();
      if (req.method() === 'PUT' && /\/api\/files\//.test(req.url())) {
        puts.push({ url: new URL(req.url()).pathname, type: req.headers()['content-type'] || '', device: Boolean(req.headers()['x-choir-device']), size: (req.postDataBuffer() || Buffer.alloc(0)).length });
        return route.fulfill({ status: 200, contentType: 'application/json', body: '{"status":"updated"}' });
      }
      return route.fulfill({ status: 401, contentType: 'application/json', body: '{"error":"unauthenticated"}' });
    });
    await page.goto(BASE_URL);
    await expect(page.locator('[data-desk-menu-button]')).toBeVisible();
    await page.evaluate(async () => {
      const sdk = await import('/src/lib/app-sdk.ts');
      const b = document.createElement('button');
      b.id = 'up';
      b.textContent = 'up';
      Object.assign(b.style, { position: 'fixed', left: '10px', top: '10px', zIndex: 99999 });
      b.addEventListener('click', () => {
        window.__up = sdk.pickAndUploadFiles({ accept: ['pptx', '.pdf', 'text/html'], folder: 'Decks/2026' });
      });
      document.body.appendChild(b);
    });
    const [chooser] = await Promise.all([page.waitForEvent('filechooser'), page.click('#up')]);
    expect(chooser.isMultiple()).toBe(true);
    expect(await chooser.element().getAttribute('accept')).toBe('.pptx,.pdf,text/html');
    await chooser.setFiles([
      { name: 'Q3 review.pptx', mimeType: PPTX, buffer: Buffer.from('deck-bytes') },
      { name: 'brief.pdf', mimeType: 'application/pdf', buffer: Buffer.from('%PDF-1.4') },
    ]);
    const paths = await page.evaluate(() => window.__up);
    expect(paths).toEqual(['Decks/2026/Q3 review.pptx', 'Decks/2026/brief.pdf']);
    expect(puts).toEqual([
      { url: '/api/files/Decks/2026/Q3%20review.pptx', type: PPTX, device: true, size: 10 },
      { url: '/api/files/Decks/2026/brief.pdf', type: 'application/pdf', device: true, size: 8 },
    ]);
  });

  test('a failed upload rejects with the stored paths and the failures; dropZone uploads dropped files', async ({ page }) => {
    const puts = [];
    await page.route(/\/(auth|api)\//, async (route) => {
      const req = route.request();
      if (req.method() === 'PUT') {
        puts.push(new URL(req.url()).pathname);
        if (/bad/.test(req.url())) return route.fulfill({ status: 507, contentType: 'application/json', body: '{"error":"disk full"}' });
        return route.fulfill({ status: 200, contentType: 'application/json', body: '{}' });
      }
      return route.fulfill({ status: 401, contentType: 'application/json', body: '{"error":"unauthenticated"}' });
    });
    await page.goto(BASE_URL);
    await expect(page.locator('[data-desk-menu-button]')).toBeVisible();
    const result = await page.evaluate(async () => {
      const sdk = await import('/src/lib/app-sdk.ts');
      let failure = null;
      try {
        await sdk.uploadFiles([new File(['a'], 'good.pptx'), new File(['b'], 'bad.pptx')], { folder: '' });
      } catch (err) {
        failure = { name: err.name, paths: err.paths, failures: err.failures.map((f) => [f.name, f.status, f.error]) };
      }
      const zone = document.createElement('div');
      Object.assign(zone.style, { position: 'fixed', left: '0', top: '0', width: '100px', height: '100px' });
      document.body.appendChild(zone);
      const dropped = new Promise((resolve) => {
        sdk.dropZone(zone, { folder: 'Inbox', accept: ['pptx'], onUploaded: resolve });
      });
      const dt = new DataTransfer();
      dt.items.add(new File(['c'], 'talk.pptx'));
      dt.items.add(new File(['d'], 'skip.txt'));
      zone.dispatchEvent(new DragEvent('dragenter', { bubbles: true, cancelable: true, dataTransfer: dt }));
      const active = zone.hasAttribute('data-drop-active');
      zone.dispatchEvent(new DragEvent('drop', { bubbles: true, cancelable: true, dataTransfer: dt }));
      return { failure, active, dropped: await dropped, after: zone.hasAttribute('data-drop-active') };
    });
    expect(result.failure).toEqual({ name: 'UploadError', paths: ['good.pptx'], failures: [['bad.pptx', 507, 'disk full']] });
    expect(result.active).toBe(true);
    expect(result.after).toBe(false);
    expect(result.dropped).toEqual(['Inbox/talk.pptx']);
    expect(puts).toEqual(['/api/files/good.pptx', '/api/files/bad.pptx', '/api/files/Inbox/talk.pptx']);
  });

  test('a disabled drop zone still swallows the drop, so the browser does not navigate to the file', async ({ page }) => {
    await openShell(page);
    const result = await page.evaluate(async () => {
      const sdk = await import('/src/lib/app-sdk.ts');
      const zone = document.createElement('div');
      document.body.appendChild(zone);
      let disabledDrops = 0;
      sdk.dropZone(zone, { enabled: () => false, onDisabledDrop: () => { disabledDrops += 1; }, onUploaded: () => { throw new Error('uploaded'); } });
      const dt = new DataTransfer();
      dt.items.add(new File(['x'], 'x.pptx'));
      const over = new DragEvent('dragover', { bubbles: true, cancelable: true, dataTransfer: dt });
      zone.dispatchEvent(over);
      const drop = new DragEvent('drop', { bubbles: true, cancelable: true, dataTransfer: dt });
      zone.dispatchEvent(drop);
      return { over: over.defaultPrevented, drop: drop.defaultPrevented, disabledDrops };
    });
    expect(result).toEqual({ over: true, drop: true, disabledDrops: 1 });
  });
});

// ---- the whole path: Files right-click -> Open with -> package gets the file ----

test.describe('Files Open with (E2E)', () => {
  test('right-click a .pptx in Files, Open with a package, and the package receives the file', async ({ page, request }) => {
    const dev = await request.get(`${BASE_URL}/src/lib/apps/packages.ts`);
    test.skip(!dev.ok(), 'needs the vite dev server (the glob must see a new package directory)');
    const name = 'zz-deck-fixture';
    const dir = path.join(frontendRoot, 'src/packages', name);
    fs.mkdirSync(dir, { recursive: true });
    try {
      const manifest = deckManifest(name, [{ media_type: PPTX, extensions: ['pptx'], default: false }]);
      manifest.display.title = 'Deck Fixture';
      manifest.app.launcher = { ...manifest.app.launcher, menu: false };
      fs.writeFileSync(path.join(dir, 'choir-package.json'), JSON.stringify(manifest));
      fs.writeFileSync(path.join(dir, 'App.svelte'), [
        '<script>',
        "  import { getLaunchFile } from '../../lib/app-sdk';",
        '  export let appContext = {};',
        '  export let windowId = "";',
        '  const file = getLaunchFile();',
        '</script>',
        '<p data-deck-fixture-file data-window={windowId}>{file ? `${file.path}|${file.mediaType}|${file.openedWith}` : "none"}</p>',
        '<p data-deck-fixture-context>{appContext.filePath || ""}</p>',
        '',
      ].join('\n'));
      await page.waitForTimeout(1500); // let vite's watcher see the new directory

      await openShell(page);
      await page.locator('[data-desktop-icon][data-desktop-icon-id="files"]').click();
      const files = page.locator('[data-window][data-window-app-id="files"]');
      const row = files.locator('[data-file-item]', { hasText: 'quarterly-review.pptx' });
      await expect(row).toBeVisible();
      await row.click({ button: 'right' });
      await expect(menu(page).locator('[data-context-menu-item="open"]')).toBeVisible();
      await expect(menu(page).locator('[data-context-menu-item="rename"]')).toBeVisible();
      await expect(menu(page).locator('[data-context-menu-item="delete"]')).toBeVisible();
      await menu(page).locator('[data-context-menu-item="open-with"]').click();
      await expect(submenu(page)).toBeVisible();
      await expect(submenu(page).locator('[data-context-menu-item="open-with:slides"]')).toContainText('Slides');
      await submenu(page).locator(`[data-context-menu-item="open-with:${name}"]`).click();

      const win = page.locator(`[data-window][data-window-app-id="${name}"]`);
      await expect(win).toBeVisible();
      await expect(win.locator('[data-deck-fixture-file]')).toHaveText(`quarterly-review.pptx|${PPTX}|choice`);
      await expect(win.locator('[data-deck-fixture-context]')).toHaveText('quarterly-review.pptx');

      // Plain Open keeps the default (from the keyboard: the deck window now
      // covers the row).
      await row.focus();
      await page.keyboard.press('Shift+F10');
      await expect(menu(page).locator('[data-context-menu-item="open"]')).toBeFocused();
      await page.keyboard.press('Enter');
      await expect(page.locator('[data-window][data-window-app-id="slides"]')).toBeVisible();
    } finally {
      fs.rmSync(dir, { recursive: true, force: true });
    }
  });
});
