/**
 * Deployed approve step for the `choir login` acceptance
 * (scripts/e2e-cli-login-staging.sh, docs/design/cli-agent-onboarding-2026-11-03.md).
 *
 * Registers a fresh test account on example.com with Playwright's CDP virtual
 * WebAuthn authenticator, signs out (the approve link is opened signed out,
 * so the page is the host's current platform shell, not a computer's own
 * frontend), waits for the script to write the link `choir login` printed,
 * opens it, signs in with the passkey on the page, checks the requesting
 * device's label and presses Approve.
 *
 * Env: CLI_LOGIN_LINK_FILE, CLI_LOGIN_LABEL, CLI_LOGIN_RESULT_FILE (required);
 * BASE_URL (default https://choir.news).
 */
import { expect } from '@playwright/test';
import { test } from './helpers/fixtures.js';
import { registerPasskey, getSession } from './helpers/auth.js';
import fs from 'node:fs';

const BASE_URL = process.env.BASE_URL || 'https://choir.news';
const LINK_FILE = process.env.CLI_LOGIN_LINK_FILE || '';
const LABEL = process.env.CLI_LOGIN_LABEL || '';
const RESULT_FILE = process.env.CLI_LOGIN_RESULT_FILE || '';

async function waitForLink() {
  const deadline = Date.now() + 120_000;
  while (Date.now() < deadline) {
    try {
      const link = fs.readFileSync(LINK_FILE, 'utf8').trim();
      if (link) return link;
    } catch (_e) {
      /* not yet */
    }
    await new Promise((r) => setTimeout(r, 500));
  }
  throw new Error('choir login printed no approve link within 120s');
}

test('approve a choir login from a passkey session', async ({ page, authenticator }) => {
  test.setTimeout(300_000);
  expect(LINK_FILE && LABEL && RESULT_FILE, 'CLI_LOGIN_LINK_FILE, CLI_LOGIN_LABEL and CLI_LOGIN_RESULT_FILE are required').toBeTruthy();
  void authenticator;
  const email = `cli-login-e2e-${Date.now()}-${Math.random().toString(36).slice(2, 8)}@example.com`;

  await page.goto(BASE_URL);
  const reg = await registerPasskey(page, email, BASE_URL);
  expect(reg && reg.ok !== false, 'registration did not complete').toBeTruthy();
  const session = await getSession(page, BASE_URL);
  const userID = session.user && session.user.id;
  expect(userID, 'registration returned no user id').toBeTruthy();
  fs.writeFileSync(RESULT_FILE, JSON.stringify({ stage: 'registered', account_id: userID, email }) + '\n');

  // Open the link signed out; the passkey stays in the virtual authenticator.
  await page.context().clearCookies();
  const link = await waitForLink();
  expect(link.startsWith(`${BASE_URL}/device?code=`), `link ${link} is not this backend's approve page`).toBeTruthy();
  await page.goto(link);
  const route = page.locator('[data-device-route]');
  await expect(route).toHaveAttribute('data-device-state', 'signin', { timeout: 30_000 });
  await page.locator('[data-device-signin-email]').fill(email);
  await page.locator('[data-device-signin-btn]').click();
  await expect(route).toHaveAttribute('data-device-state', 'review', { timeout: 60_000 });
  await expect(page.locator('[data-device-label]')).toHaveText(LABEL);
  await expect(page.locator('[data-device-from]')).toContainText('the choir CLI');
  const shownCode = (await page.locator('[data-device-code]').textContent()).trim();
  expect(link.endsWith(shownCode), 'the page shows the code from the link').toBeTruthy();
  await page.locator('[data-device-approve]').click();
  await expect(route).toHaveAttribute('data-device-state', 'approved', { timeout: 30_000 });
  fs.writeFileSync(RESULT_FILE, JSON.stringify({ stage: 'approved', account_id: userID, email }) + '\n');
});
