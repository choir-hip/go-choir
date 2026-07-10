import { test, expect } from '@playwright/test';

const BASE_URL = process.env.GO_CHOIR_UX_BASE_URL || 'http://127.0.0.1:5173';

test.beforeEach(async ({ page }) => {
  await page.route('**/auth/session', (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ authenticated: false }),
  }));
  await page.goto(BASE_URL);
  await page.locator('[data-desktop-icon-id="settings"]').focus();
  await page.keyboard.press('Enter');
  await expect(page.locator('[data-settings-app]')).toBeVisible();
  await page.getByText('Advanced theme JSON').click();
});

test('validated theme overrides apply explicitly and remain visible in the editor', async ({ page }) => {
  const editor = page.locator('[data-theme-editor]');
  const draft = JSON.parse(await editor.inputValue());
  draft.name = 'Noir with magenta';
  draft.colors.accent = '#ff00ff';

  await editor.fill(JSON.stringify(draft, null, 2));
  await expect(page.locator('html')).not.toHaveCSS('--choir-accent', '#ff00ff');
  await page.locator('[data-theme-apply]').click();

  await expect(page.locator('[data-theme-notice]')).toContainText('Noir with magenta applied');
  expect(await page.locator('html').evaluate((node) => node.style.getPropertyValue('--choir-accent'))).toBe('#ff00ff');
  await expect(editor).toHaveValue(/#ff00ff/);
});

test('unsafe CSS values are rejected and a draft can be reverted', async ({ page }) => {
  const editor = page.locator('[data-theme-editor]');
  const original = await editor.inputValue();
  const unsafeDraft = JSON.parse(original);
  unsafeDraft.colors.accent = 'url(https://example.com/tracker)';

  await editor.fill(JSON.stringify(unsafeDraft, null, 2));
  await expect(page.locator('[data-theme-error]')).toContainText('colors.accent must be a safe CSS value');
  await expect(page.locator('[data-theme-apply]')).toBeDisabled();

  await page.locator('[data-theme-revert]').click();
  await expect(editor).toHaveValue(original);
  await expect(page.locator('[data-theme-error]')).toHaveCount(0);
});
