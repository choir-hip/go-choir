import { test, expect } from './helpers/fixtures.js';
import { registerPasskey } from './helpers/auth.js';

const BASE_URL =
  process.env.GO_CHOIR_UX_BASE_URL ||
  process.env.PLAYWRIGHT_BASE_URL ||
  'http://localhost:4173';
const DESKTOP_BOOT_TIMEOUT_MS = Number(process.env.GO_CHOIR_DESKTOP_BOOT_TIMEOUT_MS || 300000);

function uniqueEmail(prefix = 'real-desktop') {
  return `${prefix}-${Date.now()}-${Math.random().toString(36).slice(2, 8)}@example.com`;
}

async function registerAndLoadDesktop(page, email, viewportSize) {
  await page.setViewportSize(viewportSize);
  await page.goto(BASE_URL);
  await registerPasskey(page, email, BASE_URL);
  await page.reload();
  await page
    .locator('[data-desktop][data-desktop-ready="true"]')
    .waitFor({ state: 'visible', timeout: DESKTOP_BOOT_TIMEOUT_MS });
}

async function launchFromDesk(page, appId) {
  await page.locator('[data-desk-button]').click();
  await page.locator(`[data-desk-app-id="${appId}"]`).click();
  const win = page.locator(`[data-window-app-id="${appId}"]`).last();
  await expect(win).toBeVisible({ timeout: 20000 });
  return win;
}

async function openRealDesktopSet(page) {
  for (const appId of ['files', 'vtext', 'trace', 'podcast']) {
    await launchFromDesk(page, appId);
    await page.waitForTimeout(250);
  }
  await expect(page.locator('[data-window]')).toHaveCount(4, { timeout: 20000 });
}

async function readDesktopMetrics(page) {
  return page.evaluate(() => {
    const desktop = document.querySelector('[data-desktop-windows]')?.getBoundingClientRect();
    const shelf = document.querySelector('[data-shelf]')?.getBoundingClientRect();
    const overviewSummary = document.querySelector('[data-overview-summary]');
    const previewWindows = Array.from(document.querySelectorAll('[data-window][data-overview-preview-state]'))
      .map((el) => el.getAttribute('data-overview-preview-state') || 'normal')
      .filter((state) => state !== 'normal');
    const previewStateCounts = previewWindows.reduce((counts, state) => {
      counts[state] = (counts[state] || 0) + 1;
      return counts;
    }, {});
    const windows = Array.from(document.querySelectorAll('[data-window]'))
      .filter((el) => {
        const box = el.getBoundingClientRect();
        return box.width > 0 && box.height > 0 && getComputedStyle(el).display !== 'none';
      })
      .map((el) => {
        const box = el.getBoundingClientRect();
        return {
          id: el.getAttribute('data-window-id'),
          appId: el.getAttribute('data-window-app-id'),
          mode: el.getAttribute('data-window-mode'),
          active: el.getAttribute('data-window-active') === 'true',
          x: Math.round(box.x),
          y: Math.round(box.y),
          width: Math.round(box.width),
          height: Math.round(box.height),
          zIndex: Number.parseInt(el.style.zIndex || '0', 10) || 0,
        };
      });
    const desktopArea = Math.max(1, (desktop?.width || 1) * (desktop?.height || 1));
    const maxAreaRatio = windows.reduce((max, win) => Math.max(max, (win.width * win.height) / desktopArea), 0);
    let overlapPairs = 0;
    for (let i = 0; i < windows.length; i += 1) {
      for (let j = i + 1; j < windows.length; j += 1) {
        const a = windows[i];
        const b = windows[j];
        const overlapW = Math.max(0, Math.min(a.x + a.width, b.x + b.width) - Math.max(a.x, b.x));
        const overlapH = Math.max(0, Math.min(a.y + a.height, b.y + b.height) - Math.max(a.y, b.y));
        if (overlapW * overlapH > 1200) overlapPairs += 1;
      }
    }
    return {
      viewport: { width: window.innerWidth, height: window.innerHeight },
      desktop: desktop ? { width: Math.round(desktop.width), height: Math.round(desktop.height) } : null,
      shelf: shelf ? { width: Math.round(shelf.width), height: Math.round(shelf.height) } : null,
      windows,
      maxAreaRatio: Number(maxAreaRatio.toFixed(3)),
      uniqueLefts: new Set(windows.map((win) => win.x)).size,
      uniqueTops: new Set(windows.map((win) => win.y)).size,
      overlapPairs,
      overview: overviewSummary
        ? {
            windowCount: Number(overviewSummary.getAttribute('data-overview-window-count') || '0'),
            livePreviewCount: Number(overviewSummary.getAttribute('data-overview-live-preview-count') || '0'),
            cardPreviewCount: Number(overviewSummary.getAttribute('data-overview-card-preview-count') || '0'),
            redactedPreviewCount: Number(overviewSummary.getAttribute('data-overview-redacted-preview-count') || '0'),
            suspendedPreviewCount: Number(overviewSummary.getAttribute('data-overview-suspended-preview-count') || '0'),
            domLivePreviewCount: previewStateCounts.live || 0,
            domCardPreviewCount: previewStateCounts.card || 0,
            domSuspendedPreviewCount: previewStateCounts.suspended || 0,
            domRedactedPreviewCount: previewStateCounts.redacted || 0,
          }
        : null,
    };
  });
}

async function assertRealDesktopGeometry(metrics) {
  expect(metrics.windows.length).toBeGreaterThanOrEqual(4);
  expect(metrics.uniqueLefts).toBeGreaterThanOrEqual(2);
  expect(metrics.uniqueTops).toBeGreaterThanOrEqual(2);
  expect(metrics.overlapPairs).toBeGreaterThanOrEqual(2);
  expect(metrics.maxAreaRatio).toBeLessThan(0.9);
  for (const win of metrics.windows) {
    expect(win.width).toBeLessThan(metrics.viewport.width);
    expect(win.height).toBeLessThan(metrics.viewport.height);
  }
}

test('390x844 mobile keeps a real overlapping desktop with Desktop Overview actions', async ({
  page,
  authenticator,
}, testInfo) => {
  const email = uniqueEmail('mobile-real-desktop');
  await registerAndLoadDesktop(page, email, { width: 390, height: 844 });
  await openRealDesktopSet(page);

  const beforeMetrics = await readDesktopMetrics(page);
  await assertRealDesktopGeometry(beforeMetrics);
  await page.screenshot({ path: testInfo.outputPath('mobile-overlapping-windows.png'), fullPage: false });

  const activeWindow = page.locator('[data-window-active="true"]').last();
  const activeBefore = await activeWindow.boundingBox();
  const titlebar = activeWindow.locator('[data-window-titlebar]');
  await titlebar.dragTo(page.locator('[data-desktop-windows]'), {
    sourcePosition: { x: 70, y: 20 },
    targetPosition: { x: Math.max(18, activeBefore.x - 30), y: Math.min(activeBefore.y + 95, 170) },
  });
  const activeAfterMove = await activeWindow.boundingBox();
  expect(Math.abs(activeAfterMove.x - activeBefore.x) + Math.abs(activeAfterMove.y - activeBefore.y)).toBeGreaterThan(20);

  const resizeHandle = activeWindow.locator('[data-resize-handle]');
  const beforeResize = await activeWindow.boundingBox();
  await page.mouse.move(beforeResize.x + beforeResize.width - 4, beforeResize.y + beforeResize.height - 4);
  await page.mouse.down();
  await page.mouse.move(beforeResize.x + beforeResize.width - 58, beforeResize.y + beforeResize.height - 72, { steps: 8 });
  await page.mouse.up();
  await expect(resizeHandle).toBeVisible();
  const afterResize = await activeWindow.boundingBox();
  expect(beforeResize.width - afterResize.width).toBeGreaterThan(20);
  expect(beforeResize.height - afterResize.height).toBeGreaterThan(20);

  const activeId = await activeWindow.getAttribute('data-window-id');
  const minimizedWindow = page.locator(`[data-window][data-window-id="${activeId}"]`);
  await activeWindow.locator('[data-window-minimize]').click();
  await expect(minimizedWindow).not.toBeVisible();
  await page.locator(`[data-window-indicator][data-window-id="${activeId}"]`).click();
  await expect(minimizedWindow).toBeVisible();
  await expect(minimizedWindow).toHaveAttribute('data-window-active', 'true');

  await page.locator('[data-desk-button]').click();
  await page.locator('[data-desk-overview]').click();
  const overview = page.locator('[data-desktop-overview]');
  await expect(overview).toBeVisible();
  await expect(page.locator('[data-overview-card]')).toHaveCount(4);
  await expect(page.locator('[data-overview-live-hint]')).toBeVisible();
  await page.screenshot({ path: testInfo.outputPath('mobile-desktop-overview.png'), fullPage: false });

  const overviewMetrics = await readDesktopMetrics(page);
  expect(overviewMetrics.overview.windowCount).toBe(4);
  expect(overviewMetrics.overview.livePreviewCount).toBeGreaterThanOrEqual(2);
  expect(overviewMetrics.overview.livePreviewCount).toBeLessThanOrEqual(3);
  expect(overviewMetrics.overview.livePreviewCount).toBe(overviewMetrics.overview.domLivePreviewCount);
  expect(overviewMetrics.overview.livePreviewCount + overviewMetrics.overview.cardPreviewCount).toBe(4);

  await page.locator('[data-overview-suspend-background]').click();
  await expect(page.locator('[data-overview-card].suspended')).toHaveCount(2, { timeout: 10000 });
  const afterSuspendOverviewMetrics = await readDesktopMetrics(page);
  expect(afterSuspendOverviewMetrics.overview.suspendedPreviewCount).toBeGreaterThanOrEqual(2);
  expect(afterSuspendOverviewMetrics.overview.livePreviewCount).toBeGreaterThanOrEqual(1);

  const filesCard = page.locator('[data-overview-card-app-id="files"]').first();
  const filesWindowId = await filesCard.getAttribute('data-overview-card-window-id');
  await filesCard.locator('[data-overview-focus-window]').click();
  await expect(overview).not.toBeVisible();
  await expect(page.locator(`[data-window][data-window-id="${filesWindowId}"]`)).toHaveAttribute('data-window-active', 'true');

  const afterMetrics = await readDesktopMetrics(page);
  await assertRealDesktopGeometry(afterMetrics);
  console.log(JSON.stringify({ phase: 'mobile-real-desktop', beforeMetrics, afterMetrics }, null, 2));
});

test('desktop overview preserves the same shell model on desktop viewport', async ({
  page,
  authenticator,
}, testInfo) => {
  const email = uniqueEmail('desktop-overview');
  await registerAndLoadDesktop(page, email, { width: 1280, height: 900 });
  await openRealDesktopSet(page);

  const metrics = await readDesktopMetrics(page);
  await assertRealDesktopGeometry(metrics);
  await page.screenshot({ path: testInfo.outputPath('desktop-overlapping-windows.png'), fullPage: false });

  await page.locator('[data-desk-button]').click();
  await page.locator('[data-desk-overview]').click();
  await expect(page.locator('[data-desktop-overview]')).toBeVisible();
  await expect(page.locator('[data-overview-card]')).toHaveCount(4);
  await expect(page.locator('[data-overview-map-window]')).toHaveCount(4);
  await page.screenshot({ path: testInfo.outputPath('desktop-overview.png'), fullPage: false });

  const overviewMetrics = await readDesktopMetrics(page);
  expect(overviewMetrics.overview.windowCount).toBe(4);
  expect(overviewMetrics.overview.livePreviewCount).toBe(4);
  expect(overviewMetrics.overview.livePreviewCount).toBe(overviewMetrics.overview.domLivePreviewCount);

  console.log(JSON.stringify({ phase: 'desktop-real-desktop', metrics }, null, 2));
});
