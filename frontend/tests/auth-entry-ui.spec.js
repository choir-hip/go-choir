/**
 * Playwright tests for the signed-out public desktop and auth overlay
 * (VAL-FRONTEND-001, VAL-FRONTEND-002).
 *
 * These tests verify that:
 * - Signed-out root renders the public desktop instead of blocking on auth
 * - Users can reach distinct register and login views from the auth overlay
 * - Each guest auth view has a clear primary action to begin the passkey flow
 * - Signed-out initial render does not spam failing protected bootstrap/live-channel calls
 *
 * No virtual authenticator needed — these test the signed-out guest UI only.
 */
import { test, expect } from './helpers/fixtures.js';

const BASE_URL = 'http://localhost:4173';

function uniqueEmail() {
  return `public-desktop-${Date.now()}-${Math.random().toString(36).slice(2, 8)}@example.com`;
}

// ---------------------------------------------------------------
async function openAuthOverlay(page) {
  await page.locator('[data-show-desktop-btn]').click();
  await page.locator('[data-shell-login]').click();
  await page.locator('[data-auth-overlay]').waitFor({ state: 'visible' });
}

// ---------------------------------------------------------------
// Test: signed-out root shows public desktop, not the old placeholder
// ---------------------------------------------------------------
test('signed-out root shows public desktop instead of placeholder', async ({
  page,
}) => {
  // Navigate to root with no auth cookies.
  await page.goto(BASE_URL);

  const desktop = page.locator('[data-desktop]');
  await expect(desktop).toBeVisible();
  await expect(page.locator('[data-auth-entry]')).toHaveCount(0);

  await openAuthOverlay(page);
  const registerToggle = page.locator('[data-register-toggle]');
  const loginToggle = page.locator('[data-login-toggle]');
  await expect(registerToggle).toBeVisible();
  await expect(loginToggle).toBeVisible();
});

// ---------------------------------------------------------------
// Test: guest users can reach distinct register and login views
// ---------------------------------------------------------------
test('guest users can reach both register and login views', async ({ page }) => {
  await page.goto(BASE_URL);
  await openAuthOverlay(page);

  // There should be controls to switch between register and login views.
  const registerToggle = page.locator('[data-register-toggle]');
  const loginToggle = page.locator('[data-login-toggle]');

  // Both toggles should be present.
  await expect(registerToggle).toBeVisible();
  await expect(loginToggle).toBeVisible();

  // Register view should be visible by default.
  const registerView = page.locator('[data-register-view]');
  await expect(registerView).toBeVisible();

  // Switch to login view.
  await loginToggle.click();
  const loginView = page.locator('[data-login-view]');
  await expect(loginView).toBeVisible();

  // Register view should no longer be visible when login is active.
  await expect(registerView).not.toBeVisible();

  // Switch back to register view.
  await registerToggle.click();
  await expect(registerView).toBeVisible();
  await expect(loginView).not.toBeVisible();
});

// ---------------------------------------------------------------
// Test: each guest auth view has a clear primary action
// ---------------------------------------------------------------
test('register view has a clear primary action to begin passkey flow', async ({
  page,
}) => {
  await page.goto(BASE_URL);
  await openAuthOverlay(page);

  // Register view is visible by default.
  const registerView = page.locator('[data-register-view]');

  // There should be a primary action (button) to begin passkey registration.
  const registerAction = registerView.locator('button[type="submit"]');
  await expect(registerAction).toBeVisible();
  await expect(registerAction).toBeEnabled();
  await expect(registerAction).toContainText('Passkey');
});

test('login view has a clear primary action to begin passkey flow', async ({
  page,
}) => {
  await page.goto(BASE_URL);
  await openAuthOverlay(page);

  // Switch to login view.
  const loginToggle = page.locator('[data-login-toggle]');
  await loginToggle.click();

  const loginView = page.locator('[data-login-view]');

  // There should be a primary action (button) to begin passkey login.
  const loginAction = loginView.locator('button[type="submit"]');
  await expect(loginAction).toBeVisible();
  await expect(loginAction).toBeEnabled();
  await expect(loginAction).toContainText('Passkey');
});

// ---------------------------------------------------------------
// Test: signed-out root shows public shell without signed-in controls
// ---------------------------------------------------------------
test('signed-out root shows public shell without signed-in controls', async ({
  page,
}) => {
  await page.goto(BASE_URL);

  const shell = page.locator('[data-shell]');
  await expect(shell).toBeVisible();

  await page.locator('[data-show-desktop-btn]').click();
  await expect(page.locator('[data-shell-login]')).toBeVisible();
  await expect(page.locator('[data-shell-logout]')).toHaveCount(0);
});

test('signed-out prompt intent opens auth overlay without prompt-bar mutation', async ({
  page,
}) => {
  const promptRequests = [];
  page.on('request', (req) => {
    const url = new URL(req.url());
    if (url.pathname === '/api/prompt-bar') {
      promptRequests.push({ method: req.method(), url: req.url() });
    }
  });

  await page.goto(BASE_URL);
  await page.locator('[data-desktop]').waitFor({ state: 'visible' });
  await page.locator('[data-prompt-input]').fill('Draft a public desktop proof note');
  await page.locator('[data-prompt-input]').press('Enter');

  await expect(page.locator('[data-auth-overlay]')).toBeVisible();
  await expect(page.locator('[data-auth-intent]')).toContainText('Draft a public desktop proof note');
  expect(promptRequests).toHaveLength(0);
});

test('signed-out prompt survives registration and resumes through product prompt-bar', async ({
  page,
  authenticator,
}) => {
  const email = uniqueEmail();
  const prompt = `Draft public desktop replay proof ${Date.now()}`;
  const responsePromise = page.waitForResponse((response) => {
    const url = new URL(response.url());
    return url.pathname === '/api/prompt-bar' && response.request().method() === 'POST';
  }, { timeout: 30000 });

  await page.goto(BASE_URL);
  await page.locator('[data-desktop]').waitFor({ state: 'visible' });
  await page.locator('[data-prompt-input]').fill(prompt);
  await page.locator('[data-prompt-input]').press('Enter');

  await expect(page.locator('[data-auth-overlay]')).toBeVisible();
  await page.locator('[data-register-view] input[type="email"]').fill(email);
  await page.locator('[data-register-view] [data-auth-submit]').click();

  await expect(page.locator('[data-auth-overlay]')).toHaveCount(0, { timeout: 30000 });
  await expect(page.locator('[data-prompt-status]')).toContainText(/Routing|Waiting|Opening/, { timeout: 30000 });

  const response = await responsePromise;
  expect(response.status()).toBe(202);
  expect(response.request().postDataJSON()).toEqual({ text: prompt });
  await expect(page.locator('[data-vtext-app]').last()).toBeVisible({ timeout: 15000 });
});

// ---------------------------------------------------------------
// Test: signed-out initial render does not spam failing protected calls
// ---------------------------------------------------------------
test('signed-out render does not repeatedly fire failing protected requests', async ({
  page,
}) => {
  const failingProtectedRequests = [];

  // Listen for requests to protected routes.
  page.on('request', (req) => {
    const url = new URL(req.url());
    if (
      url.pathname === '/api/shell/bootstrap' ||
      url.pathname === '/api/ws'
    ) {
      failingProtectedRequests.push({
        url: req.url(),
        method: req.method(),
      });
    }
  });

  // Navigate to root with no auth cookies.
  await page.goto(BASE_URL);

  // Wait a moment for any deferred/eager requests.
  await page.waitForTimeout(1500);

  // No protected requests should have been made while signed out.
  expect(failingProtectedRequests).toHaveLength(0);
});
