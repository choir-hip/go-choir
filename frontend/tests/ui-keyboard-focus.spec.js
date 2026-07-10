import { test, expect } from '@playwright/test';

const BASE_URL = process.env.GO_CHOIR_UX_BASE_URL || 'http://127.0.0.1:5173';

test.beforeEach(async ({ page }) => {
  await page.route('**/auth/session', (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ authenticated: false }),
  }));
  await page.goto(BASE_URL);
  await expect(page.locator('[data-desktop]')).toBeVisible();
});

test('the command prompt receives focus once desktop startup finishes', async ({ page }) => {
  await expect(page.locator('[data-texture-editor]')).toBeVisible();
  await expect(page.locator('[data-prompt-input]')).toBeFocused();
});

test('Control+K opens Desk, Escape closes it, and focus returns', async ({ page }) => {
  const deskButton = page.locator('[data-desk-menu-button]');

  await page.keyboard.press('Control+K');
  await expect(page.getByRole('dialog', { name: 'Desk' })).toBeVisible();
  await expect(page.locator('[data-desk-sheet-close]')).toBeFocused();

  await page.keyboard.press('Escape');
  await expect(page.getByRole('dialog', { name: 'Desk' })).toHaveCount(0);
  await expect(deskButton).toBeFocused();
});

test('desktop app icons launch from the keyboard', async ({ page }) => {
  const settingsIcon = page.locator('[data-desktop-icon-id="settings"]');
  await settingsIcon.focus();
  await page.keyboard.press('Enter');

  await expect(page.locator('[data-settings-app]')).toBeVisible();
});
