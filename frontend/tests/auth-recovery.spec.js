/**
 * Account recovery (lost passkey), station LA slice 1.
 *
 * Mocked-route proof of the frontend path:
 *   sign-in -> "Lost your passkey?" -> email form -> "check your email"
 *   /recover#token=... landing page -> "Add a new passkey" -> passkey
 *   registration bound to the account (virtual authenticator) -> signed in.
 *
 * /auth/* routes are mocked; no auth service, maild or email is involved.
 * The Go tests in internal/auth and internal/maild prove the server side.
 * Failure modes: docs/problems/account-recovery-token-discarded-2026-10-14.md
 */
import { test, expect } from './helpers/fixtures.js';

const PHONE = { width: 390, height: 844 };
const TOKEN = 'choir_rt_' + 'A1b2C3d4E5f6G7h8I9j0K1l2M3n4O5p6Q7r8S9t0U1v';

function b64url(bytes) {
  return Buffer.from(bytes).toString('base64').replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
}

/** Creation options shaped like go-webauthn's BeginRegistration JSON. */
function creationOptions() {
  return {
    publicKey: {
      rp: { name: 'go-choir', id: 'localhost' },
      user: { name: 'owner@example.com', displayName: 'owner@example.com', id: b64url(Buffer.from('user-recover-1')) },
      challenge: b64url(Buffer.from('recovery-challenge-0123456789abcdef')),
      pubKeyCredParams: [{ type: 'public-key', alg: -7 }, { type: 'public-key', alg: -257 }],
      timeout: 60000,
      excludeCredentials: [],
      authenticatorSelection: { authenticatorAttachment: 'platform', userVerification: 'required' },
      attestation: 'none',
    },
  };
}

async function mockSignedOut(page) {
  await page.route('**/auth/session', (route) =>
    route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ authenticated: false }) }),
  );
}

async function expectNoHorizontalScroll(page) {
  const overflow = await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth);
  expect(overflow).toBeLessThanOrEqual(0);
}

async function openRecoveryFromSignIn(page) {
  await page.goto('/');
  await page.locator('[data-desk-menu-button]').click();
  await page.locator('[data-prompt-surface-login]').click();
  await expect(page.locator('[data-auth-entry]')).toBeVisible();
  await page.locator('[data-login-toggle]').click();
  await page.locator('[data-recovery-toggle]').click();
  await expect(page.locator('[data-recovery-view]')).toBeVisible();
}

test('sign-in offers "Lost your passkey?" and sends a recovery email request', async ({ page }) => {
  await page.setViewportSize(PHONE);
  await mockSignedOut(page);
  const requests = [];
  await page.route('**/auth/recovery/request', async (route) => {
    requests.push(route.request().postDataJSON());
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ ok: true }) });
  });

  await openRecoveryFromSignIn(page);
  await expect(page.locator('[data-recovery-toggle]')).toHaveCount(0);
  await expectNoHorizontalScroll(page);

  await page.locator('[data-recovery-submit]').click();
  await expect(page.locator('[data-recovery-error]')).toContainText('enter an email');
  expect(requests).toHaveLength(0);

  await page.locator('[data-recovery-email]').fill('owner@example.com');
  await page.locator('[data-recovery-submit]').click();

  const sent = page.locator('[data-recovery-sent]');
  await expect(sent).toBeVisible();
  await expect(sent).toContainText('Check your email');
  await expect(sent).toContainText('owner@example.com');
  // Enumeration-safe copy: never claims the account exists.
  await expect(sent).toContainText('If owner@example.com belongs to a Choir account');
  expect(requests).toEqual([{ email: 'owner@example.com' }]);
  await expectNoHorizontalScroll(page);

  await page.locator('[data-recovery-sent] [data-recovery-back]').click();
  await expect(page.locator('[data-login-view]')).toBeVisible();
});

test('recovery email request shows the rate-limit message on 429', async ({ page }) => {
  await page.setViewportSize(PHONE);
  await mockSignedOut(page);
  await page.route('**/auth/recovery/request', (route) =>
    route.fulfill({ status: 429, contentType: 'application/json', body: JSON.stringify({ error: 'too many recovery requests for this email' }) }),
  );

  await openRecoveryFromSignIn(page);
  await page.locator('[data-recovery-email]').fill('owner@example.com');
  await page.locator('[data-recovery-submit]').click();
  await expect(page.locator('[data-recovery-error]')).toContainText('Wait an hour');
  await expect(page.locator('[data-recovery-sent]')).toHaveCount(0);
});

test('recovery link adds a new passkey only after the person asks, then signs in', async ({ page, authenticator }) => {
  void authenticator;
  await page.setViewportSize(PHONE);
  const verifyBodies = [];
  const finishBodies = [];
  await page.route('**/auth/recovery/verify', async (route) => {
    verifyBodies.push(route.request().postDataJSON());
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(creationOptions()) });
  });
  await page.route('**/auth/register/finish', async (route) => {
    finishBodies.push(route.request().postDataJSON());
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      headers: { 'Set-Cookie': 'choir_access=mock; Path=/; HttpOnly' },
      body: JSON.stringify({ ok: true, user: { id: 'user-recover-1', email: 'owner@example.com' } }),
    });
  });

  await page.goto(`/recover#token=${TOKEN}`);
  const route = page.locator('[data-recover-route]');
  await expect(route).toHaveAttribute('data-recover-state', 'ready');

  // The token leaves the address bar at once and nothing is redeemed on load
  // (a mail scanner opening the link must not burn it).
  await expect.poll(() => page.evaluate(() => window.location.hash)).toBe('');
  expect(page.url()).not.toContain('choir_rt_');
  await page.waitForTimeout(300);
  expect(verifyBodies).toHaveLength(0);
  await expectNoHorizontalScroll(page);

  await page.locator('[data-recover-start]').click();
  await expect(page.locator('[data-recover-done]')).toBeVisible();
  await expect(page.locator('[data-recover-done]')).toContainText('You are back in');

  expect(verifyBodies).toEqual([{ token: TOKEN }]);
  expect(finishBodies).toHaveLength(1);
  expect(finishBodies[0].type).toBe('public-key');
  expect(finishBodies[0].response.attestationObject).toBeTruthy();
  const clientData = JSON.parse(Buffer.from(finishBodies[0].response.clientDataJSON, 'base64url').toString('utf8'));
  expect(clientData.type).toBe('webauthn.create');
  expect(clientData.challenge).toBe(creationOptions().publicKey.challenge);
  expect(await page.evaluate(() => window.localStorage.getItem('choir.auth.returning'))).toBe('true');
  await expectNoHorizontalScroll(page);
});

test('a cancelled passkey prompt retries without a new email', async ({ page, authenticator }) => {
  void authenticator;
  await page.setViewportSize(PHONE);
  await page.addInitScript(() => {
    const realCreate = navigator.credentials.create.bind(navigator.credentials);
    let calls = 0;
    navigator.credentials.create = (options) => {
      calls += 1;
      window.__recoveryCreateCalls = calls;
      if (calls === 1) {
        return Promise.reject(new DOMException('The operation either timed out or was not allowed.', 'NotAllowedError'));
      }
      return realCreate(options);
    };
  });
  let verifyCalls = 0;
  let finishCalls = 0;
  await page.route('**/auth/recovery/verify', async (route) => {
    verifyCalls += 1;
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(creationOptions()) });
  });
  await page.route('**/auth/register/finish', async (route) => {
    finishCalls += 1;
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ ok: true, user: { id: 'user-recover-1' } }) });
  });

  await page.goto(`/recover#token=${TOKEN}`);
  await page.locator('[data-recover-start]').click();
  await expect(page.locator('[data-recover-route]')).toHaveAttribute('data-recover-state', 'retry');
  await expect(page.locator('[data-recover-error]')).toContainText('cancelled');
  expect(finishCalls).toBe(0);

  await page.locator('[data-recover-retry]').click();
  await expect(page.locator('[data-recover-done]')).toBeVisible();
  expect(verifyCalls).toBe(1);
  expect(finishCalls).toBe(1);
  expect(await page.evaluate(() => window.__recoveryCreateCalls)).toBe(2);
});

test('a used or expired link explains itself and offers a new one', async ({ page }) => {
  await page.setViewportSize(PHONE);
  await page.route('**/auth/recovery/verify', (route) =>
    route.fulfill({ status: 400, contentType: 'application/json', body: JSON.stringify({ error: 'invalid or expired recovery token' }) }),
  );
  const requests = [];
  await page.route('**/auth/recovery/request', async (route) => {
    requests.push(route.request().postDataJSON());
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ ok: true }) });
  });

  await page.goto(`/recover#token=${TOKEN}`);
  await page.locator('[data-recover-start]').click();
  await expect(page.locator('[data-recover-route]')).toHaveAttribute('data-recover-state', 'invalid');
  await expect(page.getByRole('heading', { name: 'This link no longer works' })).toBeVisible();
  await expect(page.locator('[data-recover-error]')).toContainText('expire after 15 minutes');

  await page.locator('[data-recovery-email]').fill('owner@example.com');
  await page.locator('[data-recovery-submit]').click();
  await expect(page.locator('[data-recovery-sent]')).toBeVisible();
  expect(requests).toEqual([{ email: 'owner@example.com' }]);
  await expectNoHorizontalScroll(page);
});

test('/recover without a token shows the request form and redeems nothing', async ({ page }) => {
  await page.setViewportSize(PHONE);
  let verifyCalls = 0;
  await page.route('**/auth/recovery/verify', async (route) => {
    verifyCalls += 1;
    await route.fulfill({ status: 400, body: '{}' });
  });

  await page.goto('/recover');
  await expect(page.locator('[data-recover-route]')).toHaveAttribute('data-recover-state', 'request');
  await expect(page.locator('[data-recovery-form]')).toBeVisible();
  await page.goto('about:blank');
  await page.goto('/recover#token=not-a-choir-token');
  await expect(page.locator('[data-recover-route]')).toHaveAttribute('data-recover-state', 'request');
  expect(verifyCalls).toBe(0);
});
