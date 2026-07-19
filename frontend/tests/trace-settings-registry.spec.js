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
  await openStartApp(page, appId);
}

async function openStartApp(page, appId) {
  await page.locator('[data-start-button]').click();
  await page.locator(`[data-start-app-id="${appId}"]`).click();
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
        url.pathname.startsWith('/api/texture/')) &&
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
      promptSurfaceSize: style.getPropertyValue('--choir-prompt-surface-size').trim(),
    };
  });
  expect(rootTheme.id).toBe('futuristic-noir');
  expect(rootTheme.bg).toBe('#050912');
  expect(rootTheme.panel).toBe('#0D1628');
  expect(rootTheme.border).toBe('rgba(133, 159, 211, 0.22)');
  expect(rootTheme.promptSurfaceSize).toMatch(/^\d+px$/);
  expect(Number.parseInt(rootTheme.promptSurfaceSize, 10)).toBeGreaterThanOrEqual(56);

  const expectedApps = [
    ['files', 'Files', '📁'],
    ['browser', 'Web Lens', '🌐'],
    ['super-console', 'Super Console', '⌘'],
    ['settings', 'Settings', '⚙️'],
    ['texture', 'Texture', '📝'],
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
  await expect(settings.locator('[data-theme-presets]')).toBeVisible();
  await expect(settings.locator('[data-theme-preset="carbon-fiber-kintsugi"]')).toBeVisible();
  await settings.locator('[data-theme-preset="london-salmon"]').click();
  await expect(settings.locator('[data-settings-theme-validation]')).toContainText('London Salmon: valid config');
  const appliedTheme = await page.locator('.app-root').evaluate((node) => ({
    id: node.getAttribute('data-theme-id'),
    accent: getComputedStyle(node).getPropertyValue('--choir-accent').trim(),
  }));
  expect(appliedTheme.id).toBe('london-salmon');
  expect(appliedTheme.accent).toBe('#A44F38');
  const editorValue = await settings.locator('[data-theme-editor]').inputValue();
  expect(editorValue).toContain('"id": "london-salmon"');
  await expect(settings).not.toContainText('Editable role prompt');
  await expect(settings).not.toContainText('/api/prompts');

  await expect(page.locator('[data-desktop-icon-id="trace"]')).toHaveCount(0);
  await openApp(page, 'super-console');
  const superConsole = page.locator('[data-super-console-app]').last();
  await expect(superConsole.locator('[data-super-console]')).toBeVisible({ timeout: 10000 });

  await page.waitForTimeout(500);
  expect(forbiddenRequests).toHaveLength(0);
  expect(failedTraceRequests).toHaveLength(0);
  expect(failedProductRequests).toHaveLength(0);

  await attachScreenshot(page, testInfo, 'trace-settings-registry');
});

