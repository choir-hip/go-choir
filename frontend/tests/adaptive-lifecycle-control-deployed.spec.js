import { expect, test } from '@playwright/test';
import { logout } from './helpers/auth.js';
import { removeVirtualAuthenticator, setupVirtualAuthenticator } from './helpers/webauthn.js';

const BASE_URL = process.env.CHOIR_DEPLOYED_BASE_URL || 'https://choir.news';
const RUN_DEPLOYED = process.env.GO_CHOIR_RUN_DEPLOYED_LIFECYCLE === '1';

test.skip(!RUN_DEPLOYED, 'set GO_CHOIR_RUN_DEPLOYED_LIFECYCLE=1 to run deployed lifecycle proof');
test.use({ trace: 'on', video: 'on', screenshot: 'on' });

function uniqueEmail(prefix = 'adaptive-lifecycle') {
  return `${prefix}-${Date.now()}-${Math.random().toString(36).slice(2, 8)}@example.com`;
}

async function fetchJSON(page, path) {
  return page.evaluate(async (url) => {
    const res = await fetch(url, { credentials: 'include' });
    const body = await res.json().catch(() => null);
    return { status: res.status, body };
  }, path);
}

async function bootstrap(page) {
  const result = await fetchJSON(page, '/api/shell/bootstrap');
  expect(result.status).toBe(200);
  expect(result.body?.sandbox_id).toBeTruthy();
  return result.body;
}

test('adaptive lifecycle control deployed product path', async ({ page }, testInfo) => {
  test.setTimeout(480_000);
  const email = uniqueEmail();
  const timings = {};
  const bootstrapRequests = [];
  const prewarmRequests = [];

  page.on('request', (request) => {
    const url = new URL(request.url());
    if (url.pathname === '/api/shell/bootstrap') {
      bootstrapRequests.push({
        lifecycleStage: request.headers()['x-choir-client-lifecycle-stage'] || '',
      });
      if (request.headers()['x-choir-client-lifecycle-stage'] === 'post-auth-prewarm') {
        prewarmRequests.push(request.url());
      }
    }
  });

  const { client, authenticatorId } = await setupVirtualAuthenticator(page);

  const publicStart = Date.now();
  await page.goto(BASE_URL, { waitUntil: 'domcontentloaded', timeout: 45_000 });
  await expect(page.locator('[data-desktop]')).toBeVisible({ timeout: 30_000 });
  timings.public_desktop_ready_ms = Date.now() - publicStart;
  expect(bootstrapRequests).toHaveLength(0);

  const prompt = `adaptive lifecycle proof ${Date.now()}`;
  const firstPromptResponse = page.waitForResponse((response) => {
    const url = new URL(response.url());
    return url.pathname === '/api/prompt-bar' && response.request().method() === 'POST';
  }, { timeout: 300_000 });

  await page.locator('[data-prompt-input]').fill(prompt);
  await page.locator('[data-prompt-input]').press('Enter');
  await expect(page.locator('[data-auth-overlay]')).toBeVisible({ timeout: 10_000 });

  const registerStart = Date.now();
  await page.locator('[data-register-view] input[type="email"]').fill(email);
  await page.locator('[data-register-view] [data-auth-submit]').click();
  timings.register_ms = Date.now() - registerStart;
  await expect.poll(() => prewarmRequests.length, { timeout: 30_000 }).toBeGreaterThan(0);

  const authenticatedDesktop = page.locator('[data-desktop][data-authenticated="true"]');
  await expect(authenticatedDesktop).toHaveCount(1, { timeout: 30_000 });
  if (await authenticatedDesktop.getAttribute('data-desktop-ready') !== 'true') {
    await expect(page.locator('[data-boot-console]')).toBeVisible({ timeout: 30_000 });
  }
  await expect(page.locator('[data-desktop][data-authenticated="true"][data-desktop-ready="true"]')).toBeVisible({ timeout: 300_000 });
  const firstPromptRes = await firstPromptResponse;
  expect(firstPromptRes.status()).toBe(202);
  const firstBootstrap = await bootstrap(page);

  const secondPrompt = `adaptive lifecycle returning proof ${Date.now()}`;
  const secondPromptResponse = page.waitForResponse((response) => {
    const url = new URL(response.url());
    return url.pathname === '/api/prompt-bar' && response.request().method() === 'POST';
  }, { timeout: 45_000 });
  await page.locator('[data-prompt-input]').fill(secondPrompt);
  await page.locator('[data-prompt-input]').press('Enter');
  const promptRes = await secondPromptResponse;
  expect(promptRes.status()).toBe(202);

  await logout(page, BASE_URL);
  await page.goto(BASE_URL, { waitUntil: 'domcontentloaded', timeout: 45_000 });
  await page.locator('[data-prompt-input]').fill(`returning login ${Date.now()}`);
  await page.locator('[data-prompt-input]').press('Enter');
  await expect(page.locator('[data-auth-overlay]')).toBeVisible({ timeout: 10_000 });
  await page.locator('[data-login-toggle]').click();
  await page.locator('[data-login-view] input[type="email"]').fill(email);
  await page.locator('[data-login-view] [data-auth-submit]').click();
  const returningAuthenticatedDesktop = page.locator('[data-desktop][data-authenticated="true"]');
  await expect(returningAuthenticatedDesktop).toHaveCount(1, { timeout: 30_000 });
  if (await returningAuthenticatedDesktop.getAttribute('data-desktop-ready') !== 'true') {
    await expect(page.locator('[data-boot-console]')).toBeVisible({ timeout: 30_000 });
  }
  await expect(page.locator('[data-desktop][data-authenticated="true"][data-desktop-ready="true"]')).toBeVisible({ timeout: 300_000 });
  const returningBootstrap = await bootstrap(page);
  expect(returningBootstrap.sandbox_id).toBe(firstBootstrap.sandbox_id);

  const health = await fetchJSON(page, '/health');
  expect(health.status).toBe(200);
  expect(health.body?.vmctl_routing).toBe('enabled');
  expect(health.body?.vmctl_status).toBe('ok');
  const healthText = JSON.stringify(health.body);
  expect(healthText).not.toContain('vmctl_health');
  expect(healthText).not.toContain('vmctl_url');
  expect(healthText).not.toContain('active_vms');
  expect(healthText).not.toContain('total_ownerships');
  expect(healthText).not.toContain('memory_available_bytes');
  expect(health.body?.lifecycle?.stages || []).toEqual(expect.arrayContaining([
    expect.objectContaining({ stage: 'bootstrap.resolve' }),
    expect.objectContaining({ stage: 'bootstrap.total' }),
    expect.objectContaining({ stage: 'prompt_bar.total' }),
  ]));

  await testInfo.attach('adaptive-lifecycle-proof.json', {
    contentType: 'application/json',
    body: JSON.stringify({
      email,
      timings,
      first_vm: firstBootstrap.sandbox_id,
      returning_vm: returningBootstrap.sandbox_id,
      prewarm_requests: prewarmRequests.length,
      lifecycle_stages: health.body.lifecycle?.stages || [],
      vmctl_status: health.body.vmctl_status || null,
    }, null, 2),
  });

  await removeVirtualAuthenticator(client, authenticatorId).catch(() => {});
});
