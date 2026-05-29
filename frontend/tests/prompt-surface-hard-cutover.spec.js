import { test, expect } from '@playwright/test';

const BASE_URL = process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:5173';

test('logged-out shell uses PromptSurface, DeskSheet, and fixture previews', async ({ page }) => {
  await page.goto(BASE_URL);
  await expect(page.locator('[data-prompt-surface]')).toBeVisible();
  await expect(page.locator('[data-bottom-bar]')).toHaveCount(0);
  await expect(page.locator('[data-desk-menu-button]')).toBeVisible();
  await expect(page.locator('[data-window-tray-item]')).toHaveCount(3);
  await expect(page.locator('[data-vtext-editor]')).toContainText('Node A redesign morning review');
  await expect(page.locator('[data-trace-app]')).toContainText('Preview fixture');

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
