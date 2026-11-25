import { expect, test } from '@playwright/test';

for (const [name, session, state, visible] of [
  ['no account', { authenticated: false }, 'signin', '[data-delete-signin-btn]'],
  ['reviewer', { authenticated: true, user: { reviewer: true } }, 'reviewer', '[data-delete-reviewer]'],
  ['ordinary account', { authenticated: true, user: { reviewer: false } }, 'info', '[data-delete-ask]'],
]) {
  test(`account deletion route: ${name}`, async ({ page }) => {
    await page.route('**/auth/session', route => route.fulfill({ json: session }));
    await page.goto('/account/delete');
    await expect(page.locator('[data-delete-route]')).toHaveAttribute('data-delete-state', state);
    await expect(page.locator(visible)).toBeVisible();
    await expect(page.locator('[data-delete-confirm]')).toHaveCount(0);
    if (state !== 'info') await expect(page.locator('[data-delete-ask]')).toHaveCount(0);
    if (state === 'reviewer') await expect(page.locator(visible)).toContainText('separate account with a passkey');
  });
}
