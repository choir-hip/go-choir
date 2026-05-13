import { test, expect } from './helpers/fixtures.js';
import { registerPasskey } from './helpers/auth.js';

const BASE_URL =
  process.env.GO_CHOIR_CONTENT_BASE_URL ||
  process.env.PLAYWRIGHT_BASE_URL ||
  'http://localhost:4173';

test.use({ trace: 'on', video: 'on', screenshot: 'on' });
test.setTimeout(120_000);
test.skip(
  process.env.GO_CHOIR_RUN_OBSCURA_CDP_BROWSER !== '1',
  'set GO_CHOIR_RUN_OBSCURA_CDP_BROWSER=1 with CHOIR_OBSCURA_CDP_SCREENSHOTS=1 to verify Browser CDP screenshots'
);

function uniqueEmail() {
  return `browser-cdp-${Date.now()}-${Math.random().toString(36).slice(2, 8)}@example.com`;
}

async function registerAndLoadDesktop(page, email) {
  await page.goto(BASE_URL);
  await registerPasskey(page, email, BASE_URL);
  await page.reload();
  await page.locator('[data-desktop]').waitFor({ state: 'visible', timeout: 30_000 });
}

test('Browser app persists a backend Obscura CDP screenshot when enabled', async ({ page, authenticator }) => {
  expect(authenticator.authenticatorId).toBeTruthy();
  await registerAndLoadDesktop(page, uniqueEmail());

  await page.locator('[data-desktop-icon-id="browser"]').dblclick();
  const browserApp = page.locator('[data-browser-app]').last();
  await expect(browserApp).toBeVisible({ timeout: 10_000 });

  const backendStatus = browserApp.locator('[data-browser-backend-status]');
  await expect(backendStatus).toHaveAttribute('data-browser-backend-available', 'true', { timeout: 20_000 });
  await expect(backendStatus).toHaveAttribute(
    'data-browser-backend-substrate',
    'obscura_cli_fetch+obscura_cdp_screenshot'
  );
  await expect(backendStatus).toHaveAttribute('data-browser-supports-screenshot', 'true');
  await expect(backendStatus).toHaveAttribute('data-browser-supports-cdp-screenshot', 'true');
  await expect(backendStatus).toHaveAttribute('data-browser-supports-bounded-input', 'true');
  await expect(backendStatus).toHaveAttribute('data-browser-supports-cdp', 'false');
  await expect(backendStatus).toHaveAttribute('data-browser-world-kind', 'foreground');

  await browserApp.locator('[data-browser-url-input]').fill('https://example.com');
  await browserApp.locator('[data-browser-go-btn]').click();

  const snapshot = browserApp.locator('[data-browser-backend-snapshot]');
  await expect(snapshot).toContainText('Example Domain', { timeout: 30_000 });

  const screenshot = browserApp.locator('[data-browser-backend-screenshot]');
  await expect(screenshot).toBeVisible({ timeout: 30_000 });
  const screenshotBytes = Number(await screenshot.getAttribute('data-browser-backend-screenshot-bytes'));
  expect(screenshotBytes).toBeGreaterThan(1000);
  await expect(screenshot.locator('img')).toHaveAttribute('src', /^data:image\/png;base64,/);
  await expect(backendStatus).toHaveAttribute('data-browser-execution-scope', 'host_process');
  const backendSessionId = await backendStatus.getAttribute('data-browser-backend-session-id');
  expect(backendSessionId).toBeTruthy();

  await browserApp.locator('[data-browser-url-input]').fill('https://example.com/?choir=1');
  await browserApp.locator('[data-browser-go-btn]').click();
  await expect(snapshot).toContainText('Example Domain', { timeout: 30_000 });
  await expect(backendStatus).toHaveAttribute('data-browser-backend-session-id', backendSessionId, {
    timeout: 10_000,
  });

  const sessionId = await backendStatus.getAttribute('data-browser-session-id');
  expect(sessionId).toBeTruthy();

  await page.locator('[data-desktop-icon-id="trace"]').dblclick();
  const trace = page.locator('[data-trace-app]').last();
  await expect(trace).toBeVisible({ timeout: 10_000 });
  const browserTrajectory = trace.locator(`[data-trace-trajectory-id="browser:${sessionId}"]`);
  await expect(browserTrajectory).toBeVisible({ timeout: 20_000 });
  await browserTrajectory.click();
  await expect(trace.locator('[data-trace-moment-strip]')).toContainText('browser screenshot', {
    timeout: 20_000,
  });
});

test('Browser app applies bounded backend control through the persistent CDP session', async ({
  page,
  authenticator,
}) => {
  expect(authenticator.authenticatorId).toBeTruthy();
  await registerAndLoadDesktop(page, uniqueEmail());

  await page.locator('[data-desktop-icon-id="browser"]').dblclick();
  const browserApp = page.locator('[data-browser-app]').last();
  await expect(browserApp).toBeVisible({ timeout: 10_000 });

  const backendStatus = browserApp.locator('[data-browser-backend-status]');
  await expect(backendStatus).toHaveAttribute('data-browser-supports-bounded-input', 'true', {
    timeout: 20_000,
  });
  await expect(backendStatus).toHaveAttribute('data-browser-world-kind', 'foreground');

  await browserApp.locator('[data-browser-url-input]').fill('https://httpbin.org/forms/post');
  await browserApp.locator('[data-browser-go-btn]').click();
  await expect(browserApp.locator('[data-browser-backend-snapshot]')).toContainText('Customer name', {
    timeout: 30_000,
  });
  const backendSessionId = await backendStatus.getAttribute('data-browser-backend-session-id');
  expect(backendSessionId).toBeTruthy();

  await browserApp.locator('[data-browser-control-selector]').fill('input[name=custname]');
  await browserApp.locator('[data-browser-control-value]').fill('choir-control-ok');
  await browserApp.locator('[data-browser-control-fill]').click();
  await expect(browserApp.locator('[data-browser-backend-control]')).toHaveAttribute(
    'data-browser-control-status',
    'choir-control-ok',
    { timeout: 20_000 }
  );

  await browserApp.locator('[data-browser-control-selector]').fill('input[name=topping]');
  await browserApp.locator('[data-browser-control-click]').click();
  await expect(browserApp.locator('[data-browser-backend-control]')).toHaveAttribute(
    'data-browser-control-status',
    /Customer name/,
    { timeout: 20_000 }
  );
  await expect(backendStatus).toHaveAttribute('data-browser-backend-session-id', backendSessionId);

  const sessionId = await backendStatus.getAttribute('data-browser-session-id');
  await page.locator('[data-desktop-icon-id="trace"]').dblclick();
  const trace = page.locator('[data-trace-app]').last();
  await expect(trace).toBeVisible({ timeout: 10_000 });
  const browserTrajectory = trace.locator(`[data-trace-trajectory-id="browser:${sessionId}"]`);
  await expect(browserTrajectory).toBeVisible({ timeout: 20_000 });
  await browserTrajectory.click();
  const momentStrip = trace.locator('[data-trace-moment-strip]');
  await expect(momentStrip).toContainText('browser fill', { timeout: 20_000 });
  await expect(momentStrip).toContainText('browser click', { timeout: 20_000 });
});
