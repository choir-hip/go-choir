import { test, expect } from './helpers/fixtures.js';
import { registerPasskey } from './helpers/auth.js';

const BASE_URL =
  process.env.GO_CHOIR_UX_BASE_URL ||
  process.env.PLAYWRIGHT_BASE_URL ||
  'http://localhost:4173';
const DESKTOP_BOOT_TIMEOUT_MS = Number(process.env.GO_CHOIR_DESKTOP_BOOT_TIMEOUT_MS || 300000);

const HEAVY_APP_IDS = new Set([
  'browser',
  'apps-changes',
  'terminal',
  'vtext',
  'trace',
  'podcast',
  'image',
  'audio',
  'video',
  'pdf',
  'epub',
]);

const HEAVY_SESSION_APPS = [
  'files',
  'vtext',
  'trace',
  'podcast',
  'image',
  'audio',
  'video',
  'pdf',
  'epub',
  'podcast',
  'vtext',
  'image',
];

function uniqueEmail(prefix = 'heavy-overview') {
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
  await expect(win).toBeVisible({ timeout: 25000 });
  return win;
}

async function openHeavySession(page) {
  for (const appId of HEAVY_SESSION_APPS) {
    await launchFromDesk(page, appId);
    await page.waitForTimeout(140);
  }
  await expect(page.locator('[data-window]')).toHaveCount(HEAVY_SESSION_APPS.length, { timeout: 30000 });
}

async function reloadIntoRestoredSession(page, testInfo) {
  await page.waitForTimeout(1700);
  await page.reload();
  await page
    .locator('[data-desktop][data-desktop-ready="true"]')
    .waitFor({ state: 'visible', timeout: DESKTOP_BOOT_TIMEOUT_MS });

  const recovery = page.locator('[data-desktop-recovery]');
  if (await recovery.isVisible({ timeout: 5000 }).catch(() => false)) {
    await page.screenshot({ path: testInfo.outputPath('restore-recovery-gate.png'), fullPage: false });
    await page.locator('[data-desktop-recovery-restore-all]').click();
    await expect(recovery).not.toBeVisible({ timeout: 15000 });
  }

  await expect(page.locator('[data-window]')).toHaveCount(HEAVY_SESSION_APPS.length, { timeout: 30000 });
}

async function openOverview(page) {
  await page.locator('[data-desk-button]').click();
  await page.locator('[data-desk-overview]').click();
  const overview = page.locator('[data-desktop-overview]');
  await expect(overview).toBeVisible({ timeout: 15000 });
  return overview;
}

async function readHeavyMetrics(page) {
  return page.evaluate((heavyAppIds) => {
    const heavySet = new Set(heavyAppIds);
    const desktop = document.querySelector('[data-desktop-windows]')?.getBoundingClientRect();
    const windows = Array.from(document.querySelectorAll('[data-window]'))
      .map((el) => {
        const box = el.getBoundingClientRect();
        const appId = el.getAttribute('data-window-app-id') || '';
        const mode = el.getAttribute('data-window-mode') || '';
        const display = getComputedStyle(el).display;
        const suspended = Boolean(el.querySelector('[data-suspended-app]'));
        return {
          id: el.getAttribute('data-window-id'),
          appId,
          mode,
          active: el.getAttribute('data-window-active') === 'true',
          heavy: heavySet.has(appId),
          suspended,
          mountedHeavy: heavySet.has(appId) && !suspended && mode !== 'minimized' && display !== 'none',
          visible: box.width > 0 && box.height > 0 && display !== 'none',
          x: Math.round(box.x),
          y: Math.round(box.y),
          width: Math.round(box.width),
          height: Math.round(box.height),
          zIndex: Number.parseInt(el.style.zIndex || '0', 10) || 0,
        };
      });
    const visibleWindows = windows.filter((win) => win.visible);
    let overlapPairs = 0;
    for (let i = 0; i < visibleWindows.length; i += 1) {
      for (let j = i + 1; j < visibleWindows.length; j += 1) {
        const a = visibleWindows[i];
        const b = visibleWindows[j];
        const overlapW = Math.max(0, Math.min(a.x + a.width, b.x + b.width) - Math.max(a.x, b.x));
        const overlapH = Math.max(0, Math.min(a.y + a.height, b.y + b.height) - Math.max(a.y, b.y));
        if (overlapW * overlapH > 1200) overlapPairs += 1;
      }
    }
    const summary = document.querySelector('[data-overview-summary]');
    const overviewCards = Array.from(document.querySelectorAll('[data-overview-card]'));
    const previewWindows = Array.from(document.querySelectorAll('[data-window][data-overview-preview-state]'))
      .map((el) => el.getAttribute('data-overview-preview-state') || 'normal')
      .filter((state) => state !== 'normal');
    const previewStateCounts = previewWindows.reduce((counts, state) => {
      counts[state] = (counts[state] || 0) + 1;
      return counts;
    }, {});
    return {
      viewport: { width: window.innerWidth, height: window.innerHeight },
      desktop: desktop ? { width: Math.round(desktop.width), height: Math.round(desktop.height) } : null,
      windows,
      visibleWindowCount: visibleWindows.length,
      heavyWindowCount: windows.filter((win) => win.heavy).length,
      suspendedWindowCount: windows.filter((win) => win.suspended).length,
      mountedHeavyCount: windows.filter((win) => win.mountedHeavy).length,
      overlapPairs,
      overview: summary
        ? {
            windowCount: Number(summary.getAttribute('data-overview-window-count') || '0'),
            visibleCount: Number(summary.getAttribute('data-overview-visible-count') || '0'),
            heavyCount: Number(summary.getAttribute('data-overview-heavy-count') || '0'),
            mountedHeavyCount: Number(summary.getAttribute('data-overview-mounted-heavy-count') || '0'),
            suspendedCount: Number(summary.getAttribute('data-overview-suspended-count') || '0'),
            minimizedCount: Number(summary.getAttribute('data-overview-minimized-count') || '0'),
            livePreviewCount: Number(summary.getAttribute('data-overview-live-preview-count') || '0'),
            cardPreviewCount: Number(summary.getAttribute('data-overview-card-preview-count') || '0'),
            redactedPreviewCount: Number(summary.getAttribute('data-overview-redacted-preview-count') || '0'),
            suspendedPreviewCount: Number(summary.getAttribute('data-overview-suspended-preview-count') || '0'),
            pressure: summary.getAttribute('data-overview-pressure') || '',
            cardCount: overviewCards.length,
            suspendedCardCount: overviewCards.filter((card) => card.getAttribute('data-overview-card-suspended') === 'true').length,
            heavyCardCount: overviewCards.filter((card) => card.getAttribute('data-overview-card-heavy') === 'true').length,
            mapCount: document.querySelectorAll('[data-overview-map-window]').length,
            domLivePreviewCount: previewStateCounts.live || 0,
            domCardPreviewCount: previewStateCounts.card || 0,
            domSuspendedPreviewCount: previewStateCounts.suspended || 0,
            domRedactedPreviewCount: previewStateCounts.redacted || 0,
          }
        : null,
    };
  }, [...HEAVY_APP_IDS]);
}

function assertHeavyWindowMetrics(metrics) {
  expect(metrics.windows.length).toBe(HEAVY_SESSION_APPS.length);
  expect(metrics.visibleWindowCount).toBe(HEAVY_SESSION_APPS.length);
  expect(metrics.heavyWindowCount).toBeGreaterThanOrEqual(10);
  expect(metrics.suspendedWindowCount).toBeGreaterThanOrEqual(8);
  expect(metrics.mountedHeavyCount).toBeLessThanOrEqual(2);
  expect(metrics.overlapPairs).toBeGreaterThanOrEqual(10);
}

function assertOverviewMetrics(metrics) {
  expect(metrics.overview).toBeTruthy();
  expect(metrics.overview.windowCount).toBe(HEAVY_SESSION_APPS.length);
  expect(metrics.overview.cardCount).toBe(HEAVY_SESSION_APPS.length);
  expect(metrics.overview.mapCount).toBe(HEAVY_SESSION_APPS.length);
  expect(metrics.overview.heavyCount).toBeGreaterThanOrEqual(10);
  expect(metrics.overview.suspendedCount).toBeGreaterThanOrEqual(8);
  expect(metrics.overview.mountedHeavyCount).toBeLessThanOrEqual(2);
  expect(metrics.overview.livePreviewCount).toBeGreaterThanOrEqual(1);
  expect(metrics.overview.livePreviewCount).toBeLessThanOrEqual(metrics.viewport.width < 768 ? 3 : 6);
  expect(metrics.overview.livePreviewCount).toBe(metrics.overview.domLivePreviewCount);
  expect(metrics.overview.suspendedPreviewCount).toBeGreaterThanOrEqual(8);
  expect(metrics.overview.suspendedPreviewCount).toBe(metrics.overview.domSuspendedPreviewCount);
  expect(['steady', 'elevated', 'high']).toContain(metrics.overview.pressure);
}

async function exerciseOverviewRecovery(page, testInfo, label) {
  const beforeOverview = await readHeavyMetrics(page);
  assertHeavyWindowMetrics(beforeOverview);
  await page.screenshot({ path: testInfo.outputPath(`${label}-restored-heavy-windows.png`), fullPage: false });

  const overview = await openOverview(page);
  await expect(page.locator('[data-overview-card]')).toHaveCount(HEAVY_SESSION_APPS.length);
  await expect(page.locator('[data-overview-map-window]')).toHaveCount(HEAVY_SESSION_APPS.length);
  await expect(page.locator('[data-overview-live-hint]')).toBeVisible();
  await expect(page.locator('[data-overview-keep-active-only]')).toBeVisible();
  await page.screenshot({ path: testInfo.outputPath(`${label}-heavy-overview.png`), fullPage: false });

  const overviewMetrics = await readHeavyMetrics(page);
  assertOverviewMetrics(overviewMetrics);

  await page.locator('[data-overview-suspend-background]').click();
  const afterSuspendMetrics = await readHeavyMetrics(page);
  assertOverviewMetrics(afterSuspendMetrics);

  const suspendedCard = page.locator('[data-overview-card][data-overview-card-suspended="true"]').first();
  const suspendedWindowId = await suspendedCard.getAttribute('data-overview-card-window-id');
  await suspendedCard.locator('[data-overview-focus-window]').click();
  await expect(overview).not.toBeVisible({ timeout: 15000 });
  await expect(page.locator(`[data-window][data-window-id="${suspendedWindowId}"]`)).toHaveAttribute('data-window-active', 'true');
  await expect(page.locator(`[data-window][data-window-id="${suspendedWindowId}"] [data-suspended-app]`)).toHaveCount(0);

  await openOverview(page);
  await page.locator('[data-overview-open-compute-monitor]').click();
  await expect(page.locator('[data-compute-monitor-window]')).toBeVisible({ timeout: 20000 });
  await page.screenshot({ path: testInfo.outputPath(`${label}-compute-monitor-handoff.png`), fullPage: false });

  await openOverview(page);
  await page.locator('[data-overview-keep-active-only]').click();
  await expect(page.locator('[data-window]')).toHaveCount(1, { timeout: 20000 });

  const finalMetrics = await readHeavyMetrics(page);
  console.log(JSON.stringify({
    phase: label,
    beforeOverview,
    overviewMetrics,
    afterSuspendMetrics,
    finalMetrics,
  }, null, 2));
}

test('390x844 mobile Desktop Overview manages a restored 12-window heavy session', async ({
  page,
  authenticator,
}, testInfo) => {
  const email = uniqueEmail('mobile-heavy-overview');
  await registerAndLoadDesktop(page, email, { width: 390, height: 844 });
  await openHeavySession(page);
  await reloadIntoRestoredSession(page, testInfo);
  await exerciseOverviewRecovery(page, testInfo, 'mobile-heavy-session');
});

test('desktop Desktop Overview manages a restored 12-window heavy session', async ({
  page,
  authenticator,
}, testInfo) => {
  const email = uniqueEmail('desktop-heavy-overview');
  await registerAndLoadDesktop(page, email, { width: 1280, height: 900 });
  await openHeavySession(page);
  await reloadIntoRestoredSession(page, testInfo);
  await exerciseOverviewRecovery(page, testInfo, 'desktop-heavy-session');
});
