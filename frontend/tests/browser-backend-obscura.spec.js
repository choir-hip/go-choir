import { test, expect } from './helpers/fixtures.js';
import { registerPasskey } from './helpers/auth.js';

const BASE_URL =
  process.env.GO_CHOIR_CONTENT_BASE_URL ||
  process.env.PLAYWRIGHT_BASE_URL ||
  'http://localhost:4173';

test.use({ trace: 'on', video: 'on', screenshot: 'on' });
test.setTimeout(120_000);
test.skip(
  process.env.GO_CHOIR_RUN_OBSCURA_BROWSER !== '1',
  'set GO_CHOIR_RUN_OBSCURA_BROWSER=1 with CHOIR_OBSCURA_BIN to verify backend browser snapshots'
);

function uniqueEmail() {
  return `browser-backend-${Date.now()}-${Math.random().toString(36).slice(2, 8)}@example.com`;
}

async function registerAndLoadDesktop(page, email) {
  await page.goto(BASE_URL);
  await registerPasskey(page, email, BASE_URL);
  await page.reload();
  await page.locator('[data-desktop]').waitFor({ state: 'visible', timeout: 30_000 });
}

test('Browser app renders a backend Obscura text snapshot when configured', async ({ page, authenticator }) => {
  expect(authenticator.authenticatorId).toBeTruthy();
  await registerAndLoadDesktop(page, uniqueEmail());

  await page.locator('[data-desktop-icon-id="browser"]').dblclick();
  const browserApp = page.locator('[data-browser-app]').last();
  await expect(browserApp).toBeVisible({ timeout: 10_000 });

  const backendStatus = browserApp.locator('[data-browser-backend-status]');
  await expect(backendStatus).toHaveAttribute('data-browser-backend-available', 'true', { timeout: 20_000 });
  await expect(backendStatus).toHaveAttribute('data-browser-backend-substrate', 'obscura_cli_fetch');
  await expect(backendStatus).toHaveAttribute('data-browser-supports-text', 'true');
  await expect(backendStatus).toHaveAttribute('data-browser-supports-html', 'true');
  await expect(backendStatus).toHaveAttribute('data-browser-supports-links', 'true');
  await expect(backendStatus).toHaveAttribute('data-browser-supports-screenshot', 'false');
  await expect(backendStatus).toHaveAttribute('data-browser-supports-cdp-screenshot', 'false');
  await expect(backendStatus).toHaveAttribute('data-browser-supports-input', 'false');
  await expect(backendStatus).toHaveAttribute('data-browser-supports-cdp', 'false');

  await browserApp.locator('[data-browser-url-input]').fill('https://example.com');
  await browserApp.locator('[data-browser-go-btn]').click();

  const snapshot = browserApp.locator('[data-browser-backend-snapshot]');
  await expect(snapshot).toBeVisible({ timeout: 20_000 });
  await expect(snapshot).toContainText('Example Domain', { timeout: 30_000 });
  await expect(browserApp.locator('[data-browser-backend-links]')).toContainText('Learn more', {
    timeout: 30_000,
  });
  const htmlSource = browserApp.locator('[data-browser-backend-html]');
  await expect(htmlSource).toBeVisible({ timeout: 30_000 });
  await expect(htmlSource).toContainText('<title>Example Domain</title>', { timeout: 30_000 });
  await expect(browserApp.locator('[data-browser-iframe]')).toHaveCount(0);

  const sessionId = await backendStatus.getAttribute('data-browser-session-id');
  expect(sessionId).toBeTruthy();
  await browserApp.locator('[data-browser-close-session]').click();
  await expect(backendStatus).toHaveAttribute('data-browser-session-state', 'closed', { timeout: 10_000 });
  await expect(browserApp.locator('[data-browser-backend-snapshot]')).toHaveCount(0);

  await expect(page.locator('[data-desktop-icon-id="trace"]')).toHaveCount(0);
});
