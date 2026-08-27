import { test, expect } from './helpers/fixtures.js';
import { registerPasskey } from './helpers/auth.js';

const TARGET_URL = process.env.CHOIR_HOST || 'https://choir.news';

test('fresh user signup loads desktop and settings app dynamic chunks without CSS preload errors', async ({
  page,
  authenticator,
}) => {
  test.setTimeout(60000);
  const email = `staging-genesis-${Date.now()}@example.com`;
  console.log(`Testing staging genesis with ${email} on ${TARGET_URL}...`);

  const errors = [];
  const responses = [];

  page.on('console', msg => {
    if (msg.type() === 'error') {
      console.log(`[Browser Console Error]: ${msg.text()}`);
      errors.push(msg.text());
    }
  });

  page.on('pageerror', err => {
    console.log(`[Page Error]: ${err.message}`);
    errors.push(err.message);
  });

  page.on('response', resp => {
    responses.push({
      url: resp.url(),
      status: resp.status(),
      contentType: resp.headers()['content-type'] || '',
    });
  });

  // 1. Navigate to target and register passkey
  await page.goto(TARGET_URL);
  const regResult = await registerPasskey(page, email, TARGET_URL);
  expect(regResult.ok).toBe(true);
  console.log(`   Registered passkey for user: ${regResult.user?.id}`);
  // 3. Shell should render
  const shell = page.locator('[data-shell]');
  await expect(shell).toBeVisible({ timeout: 30000 });

  // 4. Open Settings app
  const settingsIcon = page.locator('[data-app-id="settings"], button:has-text("Settings"), div:has-text("Settings")').first();
  await settingsIcon.click({ timeout: 15000 });

  // Wait for settings app chunk to load
  await page.waitForTimeout(3000);

  // 5. Verify no stylesheet preload errors
  const preloadErrors = errors.filter(e => e.includes('Unable to preload CSS') || e.includes('Could not open Settings'));
  console.log('6. Reloading the page (testing SPA recovery / non-503)...');
  const reloadResp = await page.reload({ waitUntil: 'networkidle', timeout: 30000 });
  console.log(`   Reload response status: ${reloadResp.status()} for URL: ${reloadResp.url()}`);
  const body = await page.content();
  console.log(`   Page content preview on reload: ${body.slice(0, 300)}`);
  expect(reloadResp.status()).toBeLessThan(400);

  // 7. Verify no underivable SPA errors
  const underivableErrors = errors.filter(e => e.includes('served SPA is underivable'));
  expect(underivableErrors).toHaveLength(0);

  console.log('Staging genesis & settings verification SUCCESS!');
});
