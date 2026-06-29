import { expect, test } from './helpers/fixtures.js';

test('public legal routes render clean policy documents', async ({ page }) => {
  await page.goto('/privacy');
  await expect(page.locator('[data-legal-reader][data-legal-kind="privacy"]')).toBeVisible();
  await expect(page.getByRole('heading', { name: 'Choir Privacy Policy' })).toBeVisible();
  await expect(page.getByText('privacy@choir.news')).toBeVisible();
  await expect(page.getByText('Conjecture Verdict')).toHaveCount(0);

  await page.goto('/terms');
  await expect(page.locator('[data-legal-reader][data-legal-kind="terms"]')).toBeVisible();
  await expect(page.getByRole('heading', { name: 'Choir Terms of Service' })).toBeVisible();
  await expect(page.getByText('legal@choir.news')).toBeVisible();
  await expect(page.getByText('Conjecture Verdict')).toHaveCount(0);
});

test('auth entry exposes privacy and terms links before registration', async ({ page }) => {
  await page.route('**/auth/session', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ authenticated: false }),
    });
  });

  await page.goto('/');
  await page.locator('[data-desk-menu-button]').click();
  await page.locator('[data-prompt-surface-login]').click();

  await expect(page.locator('[data-auth-entry]')).toBeVisible();
  await expect(page.locator('[data-auth-entry] a[href="/privacy"]')).toBeVisible();
  await expect(page.locator('[data-auth-entry] a[href="/terms"]')).toBeVisible();
});
