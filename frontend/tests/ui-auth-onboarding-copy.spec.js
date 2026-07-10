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

test('generic sign-in explains the passkey and the next step in plain language', async ({ page }) => {
  await page.locator('[data-desk-menu-button]').click();
  await page.locator('[data-prompt-surface-login]').click();

  const auth = page.locator('[data-auth-entry]');
  await expect(auth).toContainText('Sign in. Pick up where you left off.');
  await expect(auth).toContainText('A passkey unlocks your saved work without a password.');
  await expect(page.locator('[data-auth-intent]')).toContainText('After sign-in');
  await expect(page.locator('[data-auth-intent]')).toContainText('Open your private computer and keep working.');
  await expect(auth).not.toContainText(/durable|spend-bearing|owner-scoped/i);
});

test('prompt sign-in names the interrupted action without flooding the dialog', async ({ page }) => {
  const promptText = `Draft a briefing ${'with supporting context '.repeat(12)}`;
  const prompt = page.locator('[data-prompt-input]');
  await prompt.fill(promptText);
  await prompt.press('Enter');

  const intent = page.locator('[data-auth-intent]');
  await expect(intent).toContainText('Run your prompt after sign-in');
  await expect(intent).toContainText('Draft a briefing');
  expect((await intent.textContent()).length).toBeLessThan(180);
});
