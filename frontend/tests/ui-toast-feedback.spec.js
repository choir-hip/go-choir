import { test, expect } from '@playwright/test';

const BASE_URL = process.env.GO_CHOIR_UX_BASE_URL || 'http://127.0.0.1:5173';

test('conductor feedback is announced and can be dismissed', async ({ page }) => {
  await page.route('**/auth/**', async (route) => {
    const pathname = new URL(route.request().url()).pathname;
    const body = pathname === '/auth/session'
      ? { authenticated: true, user: { email: 'ux-review@example.com' } }
      : { ok: true };
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(body) });
  });
  await page.route('**/health', (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ status: 'ok' }),
  }));
  await page.route('**/api/**', async (route) => {
    const pathname = new URL(route.request().url()).pathname;
    let body = {};
    if (pathname === '/api/preferences/theme') body = { theme: {} };
    else if (pathname === '/api/shell/bootstrap') body = { sandbox_id: 'ux-test-computer' };
    else if (pathname === '/api/prompt-bar') body = { submission_id: 'ux-toast' };
    else if (pathname === '/api/prompt-bar/submissions/ux-toast') {
      body = { state: 'completed', decision: { action: 'toast', message: 'Request acknowledged' } };
    }
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(body) });
  });

  await page.goto(BASE_URL);
  const prompt = page.locator('[data-prompt-input]');
  await expect(prompt).toBeEnabled({ timeout: 15_000 });
  await prompt.fill('Summarize the current work');
  await prompt.press('Enter');

  const toast = page.getByRole('status').filter({ hasText: 'Request acknowledged' });
  await expect(toast).toBeVisible();
  await toast.getByRole('button', { name: 'Dismiss notification' }).click();
  await expect(toast).toHaveCount(0);
});
