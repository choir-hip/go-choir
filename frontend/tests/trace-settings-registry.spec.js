import { test, expect } from './helpers/fixtures.js';
import { registerPasskey } from './helpers/auth.js';

const BASE_URL = process.env.GO_CHOIR_SECTION5_BASE_URL ||
  process.env.PLAYWRIGHT_BASE_URL ||
  'http://localhost:4173';

test.use({ trace: 'on', video: 'on', screenshot: 'on' });

function uniqueEmail(label) {
  return `section5-${label}-${Date.now()}-${Math.random().toString(36).slice(2, 8)}@example.com`;
}

async function registerAndLoadDesktop(page, email) {
  await page.goto(BASE_URL);
  await registerPasskey(page, email, BASE_URL);
  await page.reload();
  await page.locator('[data-desktop]').waitFor({ state: 'visible', timeout: 15000 });
}

async function openApp(page, appId) {
  await page.locator(`[data-desktop-icon-id="${appId}"]`).dblclick();
}

async function attachScreenshot(page, testInfo, name) {
  const path = testInfo.outputPath(`${name}.png`);
  await page.screenshot({ path, fullPage: false });
  await testInfo.attach(name, { path, contentType: 'image/png' });
}

test('Trace and Settings stay product-safe while app and theme metadata come from product config', async ({ page, authenticator }, testInfo) => {
  const forbiddenRequests = [];
  const failedTraceRequests = [];
  const failedProductRequests = [];

  page.on('request', (request) => {
    const url = new URL(request.url());
    if (
      url.pathname.startsWith('/api/agent/') ||
      url.pathname.startsWith('/api/prompts') ||
      url.pathname.startsWith('/api/test/') ||
      url.pathname.startsWith('/internal') ||
      url.pathname === '/api/events'
    ) {
      forbiddenRequests.push(`${request.method()} ${url.pathname}`);
    }
  });

  page.on('response', (response) => {
    const url = new URL(response.url());
    if (url.pathname.startsWith('/api/trace/') && response.status() >= 400) {
      failedTraceRequests.push(`${url.pathname}:${response.status()}`);
    }
    if (
      (url.pathname === '/health' ||
        url.pathname.startsWith('/api/shell/') ||
        url.pathname.startsWith('/api/desktop/') ||
        url.pathname.startsWith('/api/vtext/')) &&
      response.status() >= 400
    ) {
      failedProductRequests.push(`${url.pathname}:${response.status()}`);
    }
  });

  const email = uniqueEmail('product-safe');
  await registerAndLoadDesktop(page, email);

  const rootTheme = await page.locator('.app-root').evaluate((node) => {
    const style = getComputedStyle(node);
    return {
      id: node.getAttribute('data-theme-id'),
      bg: style.getPropertyValue('--choir-bg').trim(),
      panel: style.getPropertyValue('--choir-panel').trim(),
      border: style.getPropertyValue('--choir-border').trim(),
      bottomBarHeight: style.getPropertyValue('--choir-bottom-bar-height').trim(),
    };
  });
  expect(rootTheme).toEqual({
    id: 'system-noir',
    bg: '#0b0d10',
    panel: '#171827',
    border: 'rgba(148, 163, 184, 0.18)',
    bottomBarHeight: '56px',
  });

  const expectedApps = [
    ['files', 'Files', '📁'],
    ['browser', 'Browser', '🌐'],
    ['terminal', 'Terminal', '💻'],
    ['settings', 'Settings', '⚙️'],
    ['vtext', 'VText', '📝'],
    ['trace', 'Trace', '🔎'],
  ];
  for (const [appId, label, icon] of expectedApps) {
    const appIcon = page.locator(`[data-desktop-icon-id="${appId}"]`);
    await expect(appIcon).toBeVisible();
    await expect(appIcon.locator('[data-desktop-icon-label]')).toContainText(label);
    await expect(appIcon.locator('[data-desktop-icon-emoji]')).toContainText(icon);
  }

  await openApp(page, 'settings');
  const settings = page.locator('[data-settings-app]').last();
  await expect(settings).toBeVisible({ timeout: 10000 });
  await expect(settings.locator('[data-settings-account]')).toContainText(email);
  await expect(settings.locator('[data-settings-theme-validation]')).toContainText('valid config');
  await expect(settings.locator('[data-settings-runtime-status]')).toBeVisible();
  await expect(settings).not.toContainText('Editable role prompt');
  await expect(settings).not.toContainText('/api/prompts');

  await openApp(page, 'trace');
  const trace = page.locator('[data-trace-window]').last();
  await expect(trace.locator('[data-trace-app]')).toBeVisible({ timeout: 10000 });
  await expect(trace.locator('[data-trace-trajectory-list]')).toBeVisible();
  await expect(trace.locator('[data-trace-app]')).toContainText(/Trace|No trajectories|Select a trajectory/);

  await page.waitForTimeout(500);
  expect(forbiddenRequests).toHaveLength(0);
  expect(failedTraceRequests).toHaveLength(0);
  expect(failedProductRequests).toHaveLength(0);

  await attachScreenshot(page, testInfo, 'trace-settings-registry');
});
