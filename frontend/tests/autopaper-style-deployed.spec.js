/**
 * Style on the rendered Autopaper front, on the deployed origin (handoff
 * 4.G, internal/styleguide, frontend/src/packages/autopaper/style-design.js).
 * The API answer of /api/texture/style is proved by key; this proves what an
 * API key cannot: the signed-in browser draws the reader's own Style Guide on
 * the front page root, and a signed-out reader gets the compiled default.
 *
 * A fresh passkey account (tests/helpers/fixtures.js desktopSession) writes a
 * Texture titled "Style Guide" with serif modern, headline-weight 800 and
 * nameplate-rule single, then opens the front page. Synthetic text only.
 *
 *   cd frontend && BASE_URL=https://choir.news npx playwright test tests/autopaper-style-deployed.spec.js
 *   AUTOPAPER_STYLE_SHOTS   directory for screenshots (default test-results)
 */
import path from 'node:path';
import { request as pwRequest } from '@playwright/test';
import { test, expect } from './helpers/fixtures.js';

const BASE_URL = (process.env.BASE_URL || process.env.CHOIR_DEPLOYED_BASE_URL || 'https://choir.news').replace(/\/+$/, '');
const SHOTS = process.env.AUTOPAPER_STYLE_SHOTS || 'test-results';

const GUIDE = `# Style Guide

## Design values

- serif: modern
- headline-weight: 800
- dek-style: normal
- nameplate-rule: single
`;

async function api(page, method, apiPath, body) {
  return page.evaluate(async ({ method, apiPath, body }) => {
    const send = () => fetch(apiPath, {
      method, credentials: 'include',
      headers: body ? { 'Content-Type': 'application/json' } : undefined,
      body: body ? JSON.stringify(body) : undefined,
    });
    let res = await send();
    if (res.status === 401) {
      await fetch('/auth/session', { credentials: 'include' });
      res = await send();
    }
    return { status: res.status, body: await res.json().catch(() => null) };
  }, { method, apiPath, body });
}

test.describe.configure({ mode: 'serial', timeout: 300_000 });

test('the front page draws the signed-in reader\'s Style Guide and a signed-out reader gets the default', async ({ desktopSession }) => {
  const { page, context } = desktopSession;

  const created = await api(page, 'POST', '/api/texture/lifecycle-documents', {
    title: 'Style Guide',
    initial_content: GUIDE,
    client_request_id: `style-guide-${Date.now()}`,
  });
  expect(created.status, JSON.stringify(created.body)).toBe(201);

  // The read surface answers the guide's closed names.
  let style;
  await expect.poll(async () => {
    style = await api(page, 'GET', '/api/texture/style');
    return style.body?.design?.headline_weight;
  }, { timeout: 60_000 }).toBe('800');
  expect(style.body.design).toMatchObject({ serif: 'modern', nameplate_rule: 'single' });

  // Signed in: the Autopaper app root carries the variables.
  // The signed-in home is the desktop; the Autopaper app opens from its icon.
  const front = page;
  await front.locator('[data-desktop-icon-id="autopaper"]').click({ timeout: 30_000 });
  const root = front.locator('[data-autopaper]').first();
  await expect(root).toBeVisible({ timeout: 60_000 });
  await expect.poll(async () => (await root.getAttribute('style')) || '', { timeout: 30_000 }).toContain('--ap-headline-weight: 800');
  const inline = await root.getAttribute('style');
  expect(inline).toContain('--ap-nameplate-rule: 2px solid');
  expect(inline).toContain('--ap-serif: \'Charter\'');
  await front.screenshot({ path: path.join(SHOTS, 'autopaper-style-signed-in.png'), fullPage: false });

  // Signed out: the compiled default, and no console error but the expected 401.
  const reader = await context.browser().newContext({ storageState: { cookies: [], origins: [] } });
  try {
    const out = await reader.newPage();
    const errors = [];
    out.on('console', (m) => { if (m.type() === 'error') errors.push(m.text()); });
    out.on('pageerror', (e) => errors.push(`pageerror: ${e.message}`));
    await out.goto(`${BASE_URL}/`, { waitUntil: 'domcontentloaded' });
    const outRoot = out.locator('[data-autopaper]').first();
    await expect(outRoot).toBeVisible({ timeout: 60_000 });
    await expect.poll(async () => (await outRoot.getAttribute('style')) || '', { timeout: 30_000 }).toContain('--ap-headline-weight');
    const def = await outRoot.getAttribute('style');
    expect(def).toContain('--ap-headline-weight: 700');
    expect(def).toContain('--ap-nameplate-rule: 3px double');
    await out.waitForTimeout(3000);
    await out.screenshot({ path: path.join(SHOTS, 'autopaper-style-signed-out.png'), fullPage: false });
    const unexpected = errors.filter((e) => !/401|Unauthorized/i.test(e));
    expect(unexpected, unexpected.join('\n')).toEqual([]);
  } finally {
    await reader.close();
  }
  await (await pwRequest.newContext({ baseURL: BASE_URL })).dispose();
});
