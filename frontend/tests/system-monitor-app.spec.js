import { test, expect } from './helpers/fixtures.js';
import { registerPasskey } from './helpers/auth.js';

const BASE_URL = process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:4173';

function uniqueEmail(prefix = 'system-monitor') {
  return `${prefix}-${Date.now()}-${Math.random().toString(36).slice(2, 8)}@example.com`;
}

async function registerAndLoadDesktop(page, authenticator, email) {
  await page.goto(BASE_URL);
  await registerPasskey(page, email, BASE_URL);
  await page.reload();
  await page.locator('[data-desktop][data-desktop-ready="true"]').waitFor({ state: 'visible', timeout: 60000 });
}

async function openSystemMonitor(page) {
  await page.locator('[data-start-button]').click();
  await page.locator('[data-start-app-id="system-monitor"]').click();
  await expect(page.locator('[data-system-monitor-app]')).toBeVisible({ timeout: 10000 });
}

test('system monitor opens through product UI with redacted status evidence', async ({ page, authenticator }) => {
  const email = uniqueEmail();
  await registerAndLoadDesktop(page, authenticator, email);
  await openSystemMonitor(page);

  await expect(page.locator('[data-system-monitor-summary]')).toContainText('Computer health and recovery');
  await expect(page.locator('[data-system-monitor-metrics]')).toBeVisible();
  await expect(page.locator('[data-system-monitor-recovery]')).toContainText('Safe Recovery');
  await expect(page.locator('[data-system-monitor-windows]')).toContainText('System Monitor');

  const api = await page.evaluate(async () => {
    const res = await fetch('/api/system/status', { credentials: 'include' });
    const body = await res.json();
    return { status: res.status, text: JSON.stringify(body), body };
  });
  expect(api.status).toBe(200);
  expect(api.body.service).toBe('system-monitor');
  expect(api.text).not.toContain(email);
  expect(api.text).not.toContain('sandbox_url');
  expect(api.text).not.toContain('vm_id');

  const metrics = await page.evaluate(() => {
    const app = document.querySelector('[data-system-monitor-app]');
    const win = document.querySelector('[data-window][data-window-app-id="system-monitor"]');
    const appBox = app.getBoundingClientRect();
    const winBox = win.getBoundingClientRect();
    return {
      appArea: appBox.width * appBox.height,
      winArea: winBox.width * winBox.height,
      appWidth: appBox.width,
      appHeight: appBox.height,
      winWidth: winBox.width,
      winHeight: winBox.height,
      horizontalOverflow: document.documentElement.scrollWidth - document.documentElement.clientWidth,
    };
  });
  expect(metrics.appArea / metrics.winArea).toBeGreaterThan(0.82);
  expect(metrics.horizontalOverflow).toBeLessThanOrEqual(2);

  await page.screenshot({ path: test.info().outputPath('system-monitor-desktop.png'), fullPage: true });
});

test('system monitor is usable in a 390x844 floating mobile desktop window', async ({ page, authenticator }) => {
  const email = uniqueEmail('system-monitor-mobile');
  await page.setViewportSize({ width: 390, height: 844 });
  await registerAndLoadDesktop(page, authenticator, email);
  await openSystemMonitor(page);

  await expect(page.locator('[data-system-monitor-app]')).toBeVisible();
  await expect(page.locator('[data-system-monitor-computer]')).toBeVisible();
  await expect(page.locator('[data-system-monitor-recovery]')).toBeVisible();

  const metrics = await page.evaluate(() => {
    const app = document.querySelector('[data-system-monitor-app]');
    const win = document.querySelector('[data-window][data-window-app-id="system-monitor"]');
    const appBox = app.getBoundingClientRect();
    const winBox = win.getBoundingClientRect();
    return {
      appArea: appBox.width * appBox.height,
      winArea: winBox.width * winBox.height,
      windowRight: winBox.right,
      viewportWidth: window.innerWidth,
      horizontalOverflow: document.documentElement.scrollWidth - document.documentElement.clientWidth,
    };
  });
  expect(metrics.appArea / metrics.winArea).toBeGreaterThan(0.78);
  expect(metrics.windowRight).toBeLessThanOrEqual(metrics.viewportWidth + 2);
  expect(metrics.horizontalOverflow).toBeLessThanOrEqual(2);

  await page.screenshot({ path: test.info().outputPath('system-monitor-mobile-390x844.png'), fullPage: true });
});

test('heavy restored background apps are lazily suspended behind the active window', async ({ page, authenticator }) => {
  const email = uniqueEmail('system-monitor-lazy');
  await page.setViewportSize({ width: 1280, height: 900 });
  await registerAndLoadDesktop(page, authenticator, email);

  const windows = [
    {
      window_id: 'restore-monitor',
      app_id: 'system-monitor',
      title: 'System Monitor',
      geometry: { x: 120, y: 70, width: 980, height: 700 },
      mode: 'normal',
      z_index: 10,
      app_context: { windowTitle: 'System Monitor' },
    },
    {
      window_id: 'restore-trace',
      app_id: 'trace',
      title: 'Trace',
      geometry: { x: 70, y: 48, width: 900, height: 640 },
      mode: 'normal',
      z_index: 2,
      app_context: {},
    },
    {
      window_id: 'restore-vtext',
      app_id: 'vtext',
      title: 'VText',
      geometry: { x: 88, y: 62, width: 900, height: 640 },
      mode: 'normal',
      z_index: 3,
      app_context: { windowTitle: 'VText' },
    },
    {
      window_id: 'restore-pdf',
      app_id: 'pdf',
      title: 'PDF',
      geometry: { x: 104, y: 84, width: 900, height: 640 },
      mode: 'normal',
      z_index: 4,
      app_context: { windowTitle: 'PDF' },
    },
  ];

  await page.evaluate(async ({ windows }) => {
    const res = await fetch('/api/desktop/state', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ windows, active_window_id: 'restore-monitor' }),
    });
    if (!res.ok) throw new Error(`desktop state save failed: ${res.status}`);
  }, { windows });

  await page.reload();
  await page.locator('[data-desktop][data-desktop-ready="true"]').waitFor({ state: 'visible', timeout: 60000 });
  await expect(page.locator('[data-window][data-window-app-id="system-monitor"]')).toBeVisible();
  await expect(page.locator('[data-suspended-app]')).toHaveCount(3);
  await expect(page.locator('[data-system-monitor-windows]')).toContainText('suspended');

  await page.locator('[data-window-switcher] [data-window-indicator]').filter({ hasText: 'Trace' }).click();
  await expect(page.locator('[data-trace-window]')).toBeVisible({ timeout: 10000 });
});
