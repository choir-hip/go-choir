import { test, expect } from '@playwright/test';

const BASE_URL = process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:5173';
const DESK_APP_IDS = [
  'files',
  'browser',
  'email',
  'compute-monitor',
  'vtext',
  'trace',
  'podcast',
  'image',
  'audio',
  'video',
  'pdf',
  'epub',
  'features',
  'terminal',
  'settings',
];

async function openDeskApp(page, appId) {
  await page.locator('[data-desk-menu-button]').click();
  await expect(page.locator('[data-desk-sheet]')).toBeVisible();
  await page.locator(`[data-desk-sheet-app][data-desk-app-id="${appId}"]`).click();
}

test('logged-out shell uses PromptSurface, DeskSheet, and fixture previews', async ({ page }) => {
  await page.goto(BASE_URL);
  await expect(page.locator('[data-prompt-surface]')).toBeVisible();
  await expect(page.locator('[data-bottom-bar]')).toHaveCount(0);
  await expect(page.locator('[data-desk-menu-button]')).toBeVisible();
  await expect(page.locator('[data-window-tray-item]')).toHaveCount(3);
  await expect(page.locator('[data-vtext-editor]')).toContainText('Node A redesign morning review');
  await expect(page.locator('[data-trace-app]')).toContainText('Preview fixture');
  const favicon = await page.locator('link[rel="icon"][data-tetramark-favicon]').getAttribute('href');
  expect(decodeURIComponent(favicon || '')).toContain('M 269.72 36.86');
  expect(decodeURIComponent(favicon || '')).toContain('M 476.43 455.41');

  const surfaceHeight = await page.locator('[data-prompt-surface]').evaluate((el) => el.getBoundingClientRect().height);
  expect(surfaceHeight).toBeLessThanOrEqual(78);

  await page.locator('[data-desk-menu-button]').click();
  await expect(page.locator('[data-desk-sheet].placement-bottom')).toBeVisible();
  await expect(page.locator('[data-desk-sheet-app][data-desk-app-id="email"]')).toBeVisible();
});

test('PromptSurface supports top placement without old geometry variables', async ({ page }) => {
  await page.goto(BASE_URL);
  await page.evaluate(() => {
    window.dispatchEvent(new CustomEvent('choir-theme-change', {
      detail: {
        theme: {
          schema_version: 2,
          id: 'futuristic-noir',
          name: 'Futuristic Noir',
          layout: { promptSurfacePlacement: 'top' },
        },
      },
    }));
  });

  await expect(page.locator('[data-prompt-surface][data-placement="top"]')).toBeVisible();
  await page.locator('[data-desk-menu-button]').click();
  await expect(page.locator('[data-desk-sheet].placement-top')).toBeVisible();

  const boxes = await page.evaluate(() => {
    const surface = document.querySelector('[data-prompt-surface]').getBoundingClientRect();
    const sheet = document.querySelector('[data-desk-sheet]').getBoundingClientRect();
    const root = getComputedStyle(document.documentElement);
    return {
      surfaceBottom: surface.bottom,
      sheetTop: sheet.top,
      promptTop: root.getPropertyValue('--choir-prompt-surface-top-offset').trim(),
      promptBottom: root.getPropertyValue('--choir-prompt-surface-bottom-offset').trim(),
      legacyBottom: root.getPropertyValue('--choir-prompt-surface-height').trim(),
    };
  });
  expect(boxes.sheetTop).toBeGreaterThanOrEqual(boxes.surfaceBottom - 1);
  expect(boxes.promptTop).toMatch(/px$/);
  expect(boxes.promptBottom).toBe('0px');
  expect(boxes.legacyBottom).toBe('');
});

test('desktop icons reflow inside the prompt-safe viewport', async ({ page }) => {
  await page.setViewportSize({ width: 420, height: 540 });
  await page.goto(BASE_URL);
  await expect(page.locator('[data-prompt-surface]')).toBeVisible();

  const layout = await page.evaluate(() => {
    const prompt = document.querySelector('[data-prompt-surface]').getBoundingClientRect();
    const icons = [...document.querySelectorAll('[data-desktop-icon]')].map((icon) => {
      const rect = icon.getBoundingClientRect();
      return {
        id: icon.getAttribute('data-desktop-icon-id'),
        left: rect.left,
        top: rect.top,
        right: rect.right,
        bottom: rect.bottom,
      };
    });
    return {
      viewportWidth: window.innerWidth,
      viewportHeight: window.innerHeight,
      promptTop: prompt.top,
      icons,
    };
  });

  expect(layout.icons.length).toBeGreaterThan(0);
  for (const icon of layout.icons) {
    expect(icon.left, `${icon.id} left`).toBeGreaterThanOrEqual(0);
    expect(icon.top, `${icon.id} top`).toBeGreaterThanOrEqual(0);
    expect(icon.right, `${icon.id} right`).toBeLessThanOrEqual(layout.viewportWidth);
    expect(icon.bottom, `${icon.id} bottom`).toBeLessThanOrEqual(layout.promptTop);
    expect(icon.bottom, `${icon.id} viewport bottom`).toBeLessThanOrEqual(layout.viewportHeight);
  }
});

test('logged-out Desk opens every app and keeps Settings themes available', async ({ page }) => {
  await page.goto(BASE_URL);
  await expect(page.locator('[data-prompt-surface]')).toBeVisible();

  await page.locator('[data-desk-menu-button]').click();
  await expect(page.locator('[data-desk-sheet]')).toBeVisible();
  const deskOverflow = await page.locator('[data-desk-sheet]').evaluate((el) => el.scrollHeight - el.clientHeight);
  expect(deskOverflow).toBeLessThanOrEqual(1);
  for (const appId of DESK_APP_IDS) {
    await expect(page.locator(`[data-desk-sheet-app][data-desk-app-id="${appId}"]`), appId).toBeVisible();
  }
  await page.locator('[data-desk-sheet-close]').click();

  for (const appId of DESK_APP_IDS) {
    await openDeskApp(page, appId);
  }

  await expect(page.locator('[data-files-app]')).toHaveCount(1);
  await expect(page.locator('[data-browser-app-container]')).toHaveCount(1);
  await expect(page.locator('[data-email-window]')).toHaveCount(1);
  await expect(page.locator('[data-compute-monitor-window]')).toHaveCount(1);
  expect(await page.locator('[data-vtext-app]').count()).toBeGreaterThanOrEqual(1);
  await expect(page.locator('[data-trace-window]')).toHaveCount(1);
  await expect(page.locator('[data-podcast-window]')).toHaveCount(1);
  await expect(page.locator('[data-image-window]')).toHaveCount(1);
  await expect(page.locator('[data-audio-window]')).toHaveCount(1);
  await expect(page.locator('[data-video-window]')).toHaveCount(1);
  await expect(page.locator('[data-pdf-window]')).toHaveCount(1);
  await expect(page.locator('[data-epub-window]')).toHaveCount(1);
  await expect(page.locator('[data-features-window]')).toHaveCount(1);
  await expect(page.locator('[data-terminal-preview]')).toBeVisible();
  await expect(page.locator('[data-settings-window]')).toHaveCount(1);

  await expect(page.locator('[data-settings-window] [data-theme-preset]')).toHaveCount(3);
  await expect(page.locator('[data-settings-window] [data-theme-preset="futuristic-noir"]')).toBeVisible();
  await expect(page.locator('[data-settings-window] [data-theme-preset="carbon-fiber-kintsugi"]')).toBeVisible();
  await expect(page.locator('[data-settings-window] [data-theme-preset="london-salmon"]')).toBeVisible();
  await expect(page.locator('[data-settings-window] [data-theme-editor]')).toBeHidden();
});

test('Trace renders swimlanes and mobile TetraMark switches open apps', async ({ page, browser }) => {
  await page.goto(BASE_URL);
  await openDeskApp(page, 'trace');
  const traceWindow = page.locator('[data-trace-window]').last();
  await expect(traceWindow.locator('[data-trace-swimlane-chart]')).toBeVisible();
  await expect(traceWindow.locator('[data-trace-swimlane]')).toHaveCount(4);
  await expect(traceWindow.locator('[data-trace-swimlane-chart] [data-trace-moment]')).toHaveCount(7);

  const mobile = await browser.newPage({ viewport: { width: 390, height: 844 } });
  await mobile.goto(BASE_URL);
  await expect(mobile.locator('[data-prompt-surface]')).toBeVisible();
  await mobile.locator('[data-desk-menu-button]').click();
  await expect(mobile.locator('[data-desk-sheet]')).toBeVisible();
  await expect(mobile.locator('[data-mobile-app-switcher]')).toBeVisible();
  await expect(mobile.locator('[data-mobile-switcher-open="true"] [data-prompt-input]')).toHaveCount(0);
  await expect(mobile.locator('[data-mobile-app-switcher] button')).not.toHaveCount(0);
  await mobile.close();
});
