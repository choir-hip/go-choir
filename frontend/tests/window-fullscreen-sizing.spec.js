/**
 * Window sizing and interaction across the whole free area.
 *
 * The owner's direction: a maximized window must be the entire screen between
 * the top edge and the prompt bar, windows must drag and resize all the way to
 * the edges, a new window must open large enough for its content, and on a
 * phone every window opens (nearly) full screen.
 *
 * Free area = [safe-area inset left, safe-area inset top + prompt-bar top] to
 * [viewport width - inset right, prompt bar top]. On the two viewports this
 * spec uses (390x844 and 1280x860) the safe-area insets are 0.
 *
 * Screenshots of each case are written to test-results/window-sizing/.
 *
 * UNRUN IN THIS CAPSULE: this spec's harness needs the local service stack
 * (auth on :8081, proxy on :8082), a Vite server on :4173 and the Playwright
 * Chromium download. The implementation capsule that wrote it had only the Go
 * module proxy, so no Playwright run happened there. The checks that did run
 * there are the frontend build (which runs the package/source-contract checks)
 * and capsule syntax checks; the visual evidence is the capsule screenshot of
 * the signed-out root page.
 */
import fs from 'node:fs';
import path from 'node:path';
import { test, expect } from './helpers/fixtures.js';
import { registerPasskey } from './helpers/auth.js';

const BASE_URL =
  process.env.GO_CHOIR_UX_BASE_URL ||
  process.env.PLAYWRIGHT_BASE_URL ||
  'http://localhost:4173';
const DESKTOP_BOOT_TIMEOUT_MS = Number(process.env.GO_CHOIR_DESKTOP_BOOT_TIMEOUT_MS || 120000);
const SHOT_DIR = path.join(process.cwd(), 'test-results', 'window-sizing');
const PHONE = { width: 390, height: 844 };
const DESKTOP = { width: 1280, height: 860 };

function uniqueEmail() {
  return 'sizing-test-' + Date.now() + '-' + Math.random().toString(36).slice(2, 8) + '@example.com';
}

async function shot(page, name) {
  fs.mkdirSync(SHOT_DIR, { recursive: true });
  await page.screenshot({ path: path.join(SHOT_DIR, name), fullPage: false });
}

async function loadDesktop(page, authenticator, email, viewportSize) {
  await page.setViewportSize(viewportSize);
  await page.goto(BASE_URL);
  await registerPasskey(page, email, BASE_URL);
  await page.reload();
  await page.locator('[data-desktop]').waitFor({ state: 'visible', timeout: DESKTOP_BOOT_TIMEOUT_MS });
}

async function openApp(page, appId) {
  await page.locator('[data-desktop-icon-id="' + appId + '"]').first().click();
  await page.waitForTimeout(350);
}

function windowFor(page, appId) {
  return page.locator('[data-window][data-window-app-id="' + appId + '"]').first();
}

/** The area a maximized window must fill, measured from the live page. */
async function freeArea(page) {
  return page.evaluate(() => {
    const prompt = document.querySelector('[data-prompt-surface]');
    const rect = prompt.getBoundingClientRect();
    const placement = prompt.getAttribute('data-placement') || 'bottom';
    const rootStyle = getComputedStyle(document.documentElement);
    const px = (name) => {
      const value = Number.parseFloat(rootStyle.getPropertyValue(name));
      return Number.isFinite(value) ? value : 0;
    };
    const top = placement === 'top' ? rect.bottom : px('--choir-safe-area-inset-top');
    const bottom = placement === 'bottom' ? rect.top : window.innerHeight;
    return {
      x: 0,
      y: top,
      width: window.innerWidth,
      height: bottom - top,
      promptTop: rect.top,
      promptLeft: rect.left,
      promptRight: rect.right,
      promptBottom: rect.bottom,
    };
  });
}

async function rect(page, locator) {
  const box = await locator.boundingBox();
  expect(box, 'element has a bounding box').not.toBeNull();
  return box;
}

/** Drag from one point to another with real pointer moves; returns the release. */
async function dragBy(page, from, dx, dy, steps = 12) {
  await page.mouse.move(from.x, from.y);
  await page.mouse.down();
  for (let i = 1; i <= steps; i++) {
    await page.mouse.move(from.x + (dx * i) / steps, from.y + (dy * i) / steps);
  }
  return async () => {
    await page.mouse.up();
    await page.waitForTimeout(150);
  };
}

function expectRectClose(actual, expected, tolerance, label) {
  expect(Math.abs(actual.x - expected.x), label + ': left').toBeLessThanOrEqual(tolerance);
  expect(Math.abs(actual.y - expected.y), label + ': top').toBeLessThanOrEqual(tolerance);
  expect(Math.abs(actual.width - expected.width), label + ': width').toBeLessThanOrEqual(tolerance);
  expect(Math.abs(actual.height - expected.height), label + ': height').toBeLessThanOrEqual(tolerance);
}

function intersects(a, b) {
  return (
    a.x < b.x + b.width &&
    b.x < a.x + a.width &&
    a.y < b.y + b.height &&
    b.y < a.y + a.height
  );
}

// =====================================================================
// Desktop 1280x860
// =====================================================================

test.describe('desktop 1280x860', () => {
  test('a maximized window fills the free area and clears the prompt bar', async ({ page, authenticator }) => {
    await loadDesktop(page, authenticator, uniqueEmail(), DESKTOP);

    await openApp(page, 'files');
    const files = windowFor(page, 'files');
    await expect(files).toBeVisible({ timeout: 5000 });

    const normal = await rect(page, files);
    // A desktop window opens at the larger of its default and 70% of the free
    // area, instead of as a small card.
    const area = await freeArea(page);
    expect(normal.width).toBeGreaterThanOrEqual(Math.round(area.width * 0.7) - 1);

    await files.locator('[data-window-maximize]').click();
    await page.waitForTimeout(250);

    const maximized = await rect(page, files);
    expectRectClose(maximized, area, 1, 'maximized Files');
    expect(await files.getAttribute('data-window-mode')).toBe('maximized');
    await shot(page, 'desktop-01-maximized-files.png');

    // Mail maximized is the same rectangle, and its default fits a list and a
    // reading column.
    await openApp(page, 'email');
    const mail = windowFor(page, 'email');
    await expect(mail).toBeVisible({ timeout: 5000 });
    const mailNormal = await rect(page, mail);
    expect(mailNormal.width).toBeGreaterThanOrEqual(1100);
    expect(mailNormal.height).toBeGreaterThanOrEqual(720);
    await mail.locator('[data-window-maximize]').click();
    await page.waitForTimeout(250);
    expectRectClose(await rect(page, mail), area, 1, 'maximized Mail');
    await shot(page, 'desktop-02-maximized-mail.png');

    // Square corners and no shadow on a maximized window.
    const chrome = await mail.evaluate((el) => {
      const style = getComputedStyle(el);
      return { radius: style.borderTopLeftRadius, shadow: style.boxShadow };
    });
    expect(chrome.radius).toBe('0px');
    expect(chrome.shadow).toBe('none');

    // No window covers the prompt bar.
    const prompt = await rect(page, page.locator('[data-prompt-surface]'));
    for (const win of await page.locator('[data-window]').all()) {
      expect(intersects(await rect(page, win), prompt), 'window intersects prompt bar').toBe(false);
    }
  });

  test('a window dragged to the right edge reaches it', async ({ page, authenticator }) => {
    await loadDesktop(page, authenticator, uniqueEmail(), DESKTOP);

    await openApp(page, 'files');
    const files = windowFor(page, 'files');
    const before = await rect(page, files);
    const grab = { x: before.x + 60, y: before.y + 18 };

    const release = await dragBy(page, grab, DESKTOP.width, 0);
    await shot(page, 'desktop-03-drag-right-held.png');
    await release();

    const after = await rect(page, files);
    expect(after.x + after.width).toBeGreaterThanOrEqual(DESKTOP.width - 1);
    expect(after.x).toBeGreaterThan(before.x);
    await shot(page, 'desktop-04-dragged-right-edge.png');

    // And back again: the left edge and the top edge are reachable too.
    const grab2 = { x: after.x + 20, y: after.y + 18 };
    const release2 = await dragBy(page, grab2, -DESKTOP.width, -300);
    await release2();
    const left = await rect(page, files);
    expect(left.x).toBeLessThanOrEqual(1);
    expect(left.y).toBeLessThanOrEqual(1);
    await shot(page, 'desktop-05-dragged-top-left-edge.png');
  });

  test('resizing from the left edge widens the window to the edge', async ({ page, authenticator }) => {
    await loadDesktop(page, authenticator, uniqueEmail(), DESKTOP);

    await openApp(page, 'files');
    const files = windowFor(page, 'files');
    const before = await rect(page, files);
    const rightEdge = before.x + before.width;

    const handle = files.locator('[data-resize-handle].resize-w');
    await expect(handle).toHaveCount(1);
    const handleBox = await rect(page, handle);
    const grab = { x: handleBox.x + handleBox.width / 2, y: before.y + before.height / 2 };
    const release = await dragBy(page, grab, -before.x, 0);
    await release();

    const after = await rect(page, files);
    expect(after.x).toBeLessThanOrEqual(1);
    expect(Math.abs(after.x + after.width - rightEdge)).toBeLessThanOrEqual(2);
    expect(after.width).toBeGreaterThan(before.width);
    await shot(page, 'desktop-06-resized-from-left.png');
  });

  test('resizing can reach exactly the maximized area', async ({ page, authenticator }) => {
    await loadDesktop(page, authenticator, uniqueEmail(), DESKTOP);

    await openApp(page, 'files');
    const files = windowFor(page, 'files');
    const area = await freeArea(page);
    const before = await rect(page, files);

    // Drive the north-west corner to the top-left of the free area.
    const nw = await rect(page, files.locator('[data-resize-handle].resize-nw'));
    const releaseNw = await dragBy(page, { x: nw.x + 4, y: nw.y + 4 }, -before.x - 20, -before.y - 20);
    await releaseNw();
    const grown = await rect(page, files);
    expect(grown.x).toBeLessThanOrEqual(1);
    expect(grown.y).toBeLessThanOrEqual(1);

    // Drive the south-east corner out past the prompt bar; it stops at the
    // free area's bottom.
    const se = await rect(page, files.locator('[data-resize-handle].resize-se'));
    const releaseSe = await dragBy(page, { x: se.x + 4, y: se.y + 4 }, area.width, area.height);
    await releaseSe();

    const reached = await rect(page, files);
    expectRectClose(reached, area, 1, 'resized to free area');
    await shot(page, 'desktop-07-resized-to-free-area.png');
  });

  test('dragging a title bar to the top edge previews and then maximizes', async ({ page, authenticator }) => {
    await loadDesktop(page, authenticator, uniqueEmail(), DESKTOP);

    await openApp(page, 'files');
    const files = windowFor(page, 'files');
    const before = await rect(page, files);
    const grab = { x: before.x + 60, y: before.y + 18 };
    const release = await dragBy(page, grab, 0, -before.y - 40);
    await page.waitForTimeout(100);
    await expect(page.locator('[data-snap-preview]')).toBeVisible();
    await shot(page, 'desktop-08-snap-preview.png');
    await release();
    await page.waitForTimeout(250);

    expect(await files.getAttribute('data-window-mode')).toBe('maximized');
    expectRectClose(await rect(page, files), await freeArea(page), 1, 'snap maximized');
    await shot(page, 'desktop-09-snap-maximized.png');
  });

  test('the prompt bar rect never intersects a window rect', async ({ page, authenticator }) => {
    await loadDesktop(page, authenticator, uniqueEmail(), DESKTOP);

    await openApp(page, 'files');
    await openApp(page, 'email');
    await windowFor(page, 'email').locator('[data-window-maximize]').click();
    await page.waitForTimeout(250);

    const prompt = await rect(page, page.locator('[data-prompt-surface]'));
    for (const win of await page.locator('[data-window]').all()) {
      const box = await rect(page, win);
      expect(intersects(box, prompt), 'window intersects the prompt bar').toBe(false);
      expect(box.y + box.height).toBeLessThanOrEqual(prompt.y + 1);
    }
    await shot(page, 'desktop-10-prompt-bar-clear.png');
  });
});

// =====================================================================
// Phone 390x844
// =====================================================================

test.describe('phone 390x844', () => {
  test('a new phone window opens maximized and fills the free area', async ({ page, authenticator }) => {
    await loadDesktop(page, authenticator, uniqueEmail(), PHONE);

    await openApp(page, 'files');
    const files = windowFor(page, 'files');
    await expect(files).toBeVisible({ timeout: 5000 });
    expect(await files.getAttribute('data-window-mode')).toBe('maximized');
    expectRectClose(await rect(page, files), await freeArea(page), 1, 'phone Files');
    await shot(page, 'phone-01-files-maximized.png');

    await openApp(page, 'email');
    const mail = windowFor(page, 'email');
    expect(await mail.getAttribute('data-window-mode')).toBe('maximized');
    expectRectClose(await rect(page, mail), await freeArea(page), 1, 'phone Mail');
    await shot(page, 'phone-02-mail-maximized.png');

    const prompt = await rect(page, page.locator('[data-prompt-surface]'));
    for (const win of await page.locator('[data-window]').all()) {
      expect(intersects(await rect(page, win), prompt), 'window intersects the prompt bar').toBe(false);
    }
  });

  test('restoring from maximized keeps a near full-screen window', async ({ page, authenticator }) => {
    await loadDesktop(page, authenticator, uniqueEmail(), PHONE);

    await openApp(page, 'files');
    const files = windowFor(page, 'files');
    await files.locator('[data-window-maximize]').click();
    await page.waitForTimeout(250);

    expect(await files.getAttribute('data-window-mode')).toBe('normal');
    const restored = await rect(page, files);
    const area = await freeArea(page);
    expect(restored.width).toBeGreaterThanOrEqual(Math.round(area.width * 0.92) - 1);
    expect(restored.height).toBeGreaterThanOrEqual(Math.round(area.height * 0.85) - 1);
    await shot(page, 'phone-03-restored.png');
  });

  test('a window dragged to the right edge reaches it', async ({ page, authenticator }) => {
    await loadDesktop(page, authenticator, uniqueEmail(), PHONE);

    await openApp(page, 'files');
    const files = windowFor(page, 'files');
    // Restore first: a maximized window drags into a normal one under the
    // pointer, which is the phone's way back to a floating window.
    await files.locator('[data-window-maximize]').click();
    await page.waitForTimeout(250);
    const before = await rect(page, files);
    const grab = { x: before.x + 50, y: before.y + 18 };
    const release = await dragBy(page, grab, PHONE.width, 60);
    await release();

    const after = await rect(page, files);
    expect(after.x + after.width).toBeGreaterThanOrEqual(PHONE.width - 1);
    await shot(page, 'phone-04-dragged-right-edge.png');
  });

  test('resizing from the left edge works on a phone-sized window', async ({ page, authenticator }) => {
    await loadDesktop(page, authenticator, uniqueEmail(), PHONE);

    await openApp(page, 'files');
    const files = windowFor(page, 'files');
    await files.locator('[data-window-maximize]').click();
    await page.waitForTimeout(250);
    const before = await rect(page, files);
    const rightEdge = before.x + before.width;

    const w = await rect(page, files.locator('[data-resize-handle].resize-w'));
    const release = await dragBy(page, { x: w.x + w.width / 2, y: before.y + before.height / 2 }, -before.x, 0);
    await release();

    const after = await rect(page, files);
    expect(after.x).toBeLessThanOrEqual(1);
    expect(Math.abs(after.x + after.width - rightEdge)).toBeLessThanOrEqual(2);
    await shot(page, 'phone-05-resized-from-left.png');

    const prompt = await rect(page, page.locator('[data-prompt-surface]'));
    expect(intersects(after, prompt), 'resized window intersects the prompt bar').toBe(false);
  });

  test('the prompt bar rect never intersects a window rect', async ({ page, authenticator }) => {
    await loadDesktop(page, authenticator, uniqueEmail(), PHONE);

    await openApp(page, 'files');
    await openApp(page, 'email');
    const prompt = await rect(page, page.locator('[data-prompt-surface]'));
    for (const win of await page.locator('[data-window]').all()) {
      const box = await rect(page, win);
      expect(intersects(box, prompt), 'window intersects the prompt bar').toBe(false);
    }
    await shot(page, 'phone-06-prompt-bar-clear.png');
  });
});
