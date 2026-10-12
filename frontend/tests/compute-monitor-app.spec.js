import { test, expect } from './helpers/fixtures.js';
import { registerPasskey } from './helpers/auth.js';

const BASE_URL = process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:4173';

function uniqueEmail(prefix = 'compute-monitor') {
  return `${prefix}-${Date.now()}-${Math.random().toString(36).slice(2, 8)}@example.com`;
}

async function registerAndLoadDesktop(page, authenticator, email) {
  await page.goto(BASE_URL);
  await registerPasskey(page, email, BASE_URL);
  await page.reload();
  await page.locator('[data-desktop][data-desktop-ready="true"]').waitFor({ state: 'visible', timeout: 60000 });
}

async function openComputeMonitor(page) {
  await page.locator('[data-desk-menu-button]').click();
  await expect(page.locator('[data-desk-sheet]')).toBeVisible();
  await page.locator('[data-desk-sheet-app][data-desk-app-id="compute-monitor"]').click();
  await expect(page.locator('[data-compute-monitor-app]')).toBeVisible({ timeout: 10000 });
}

test('compute monitor opens through product UI with redacted status evidence', async ({ page, authenticator }) => {
  const email = uniqueEmail();
  await registerAndLoadDesktop(page, authenticator, email);
  await openComputeMonitor(page);

  await expect(page.locator('[data-compute-monitor-summary]')).toContainText('Computer health and recovery');
  await expect(page.locator('[data-compute-monitor-metrics]')).toBeVisible();
  await expect(page.locator('[data-compute-monitor-recovery]')).toContainText('Safe Recovery');
  await expect(page.locator('[data-compute-monitor-windows]')).toContainText('Compute Monitor');
  await expect(page.locator('[data-compute-monitor-app]')).not.toContainText('Memory available');
  await expect(page.locator('[data-compute-monitor-app]')).not.toContainText('Lifecycle Pressure');

  const api = await page.evaluate(async () => {
    const res = await fetch('/api/compute/status', { credentials: 'include' });
    const body = await res.json();
    return { status: res.status, text: JSON.stringify(body), body };
  });
  expect(api.status).toBe(200);
  expect(api.body.service).toBe('compute-monitor');
  expect(api.text).not.toContain(email);
  expect(api.text).not.toContain('computer_url');
  expect(api.text).not.toContain('vm_id');
  expect(api.text).not.toContain('vmctl');
  expect(api.text).not.toContain('active_vms');
  expect(api.text).not.toContain('total_ownerships');
  expect(api.text).not.toContain('memory_available_bytes');
  expect(api.text).not.toContain('active_provider');
  expect(api.text).not.toContain('build');
  expect(api.text).not.toContain('lifecycle');

  const metrics = await page.evaluate(() => {
    const app = document.querySelector('[data-compute-monitor-app]');
    const win = document.querySelector('[data-window][data-window-app-id="compute-monitor"]');
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

  await page.screenshot({ path: test.info().outputPath('compute-monitor-desktop.png'), fullPage: true });
});

test('compute monitor is usable in a 390x844 floating mobile desktop window', async ({ page, authenticator }) => {
  const email = uniqueEmail('compute-monitor-mobile');
  await page.setViewportSize({ width: 390, height: 844 });
  await registerAndLoadDesktop(page, authenticator, email);
  await openComputeMonitor(page);

  await expect(page.locator('[data-compute-monitor-app]')).toBeVisible();
  await expect(page.locator('[data-compute-monitor-computer]')).toBeVisible();
  await expect(page.locator('[data-compute-monitor-recovery]')).toBeVisible();

  const metrics = await page.evaluate(() => {
    const app = document.querySelector('[data-compute-monitor-app]');
    const win = document.querySelector('[data-window][data-window-app-id="compute-monitor"]');
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

  await page.screenshot({ path: test.info().outputPath('compute-monitor-mobile-390x844.png'), fullPage: true });
});

