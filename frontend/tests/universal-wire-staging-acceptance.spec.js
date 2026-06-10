import fs from 'fs';
import path from 'path';
import { fileURLToPath } from 'url';
import { test, expect } from '@playwright/test';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const FRONTEND_ROOT = path.resolve(__dirname, '..');
const BASE_URL = process.env.CHOIR_DEPLOYED_BASE_URL || 'https://choir.news';
const DEFAULT_AUTH_STATE = path.join(
  FRONTEND_ROOT,
  'playwright',
  '.auth',
  `${new URL(BASE_URL).hostname.replaceAll('.', '-')}.storage.json`,
);
const AUTH_STATE = path.resolve(process.env.CHOIR_AUTH_STATE || DEFAULT_AUTH_STATE);

test.use({ trace: 'on', screenshot: 'on' });
test.setTimeout(120_000);
test.skip(
  process.env.GO_CHOIR_RUN_UNIVERSAL_WIRE_STAGING !== '1',
  'set GO_CHOIR_RUN_UNIVERSAL_WIRE_STAGING=1 to verify deployed Universal Wire rename acceptance',
);
test.skip(
  !fs.existsSync(AUTH_STATE),
  `missing Playwright auth state at ${AUTH_STATE}; run node scripts/setup-auth-state.mjs --baseUrl ${BASE_URL}`,
);

async function fetchStories(page) {
  return page.evaluate(async () => {
    let res = await fetch('/api/universal-wire/stories', { credentials: 'include' });
    if (res.status === 401) {
      await fetch('/auth/session', { credentials: 'include' }).catch(() => null);
      res = await fetch('/api/universal-wire/stories', { credentials: 'include' });
    }
    const body = await res.text();
    if (!res.ok) {
      throw new Error(`/api/universal-wire/stories failed: ${res.status} ${body}`);
    }
    return body ? JSON.parse(body) : null;
  });
}

async function openDeskApp(page, appId) {
  await page.locator('[data-desk-menu-button]').click();
  await expect(page.locator('[data-desk-sheet]')).toBeVisible();
  await page.locator(`[data-desk-sheet-app][data-desk-app-id="${appId}"]`).click();
}

test('deployed Universal Wire rename: stories API and app surface', async ({ browser }) => {
  const context = await browser.newContext({ storageState: AUTH_STATE });
  const page = await context.newPage();

  try {
    await page.goto(BASE_URL);
    await page.locator('[data-desktop][data-authenticated="true"]').waitFor({ state: 'visible', timeout: 60_000 });

    const stories = await fetchStories(page);
    expect(stories.source).toMatch(/^universal-wire-/);
    expect(stories.edition).toBeTruthy();
    expect(stories.edition.source_path).toBe('universal-wire/Wire.vtext');
    expect(stories.edition.doc_id).toBeTruthy();
    expect(Array.isArray(stories.stories)).toBe(true);
    if (stories.source === 'universal-wire-edition-vtext') {
      expect(stories.stories.length).toBeGreaterThan(0);
      expect(stories.stories[0].headline).toBeTruthy();
    }

    await openDeskApp(page, 'universal-wire');
    const app = page.locator('[data-universal-wire-app]');
    await expect(app).toBeVisible();
    await expect(app.getByRole('heading', { name: 'Universal Wire' })).toBeVisible();
    await expect(app.locator('text=SourceMaxx newsroom')).toHaveCount(0);
    await expect(app.locator('text=Global Wire')).toHaveCount(0);

    if (stories.stories.length > 0) {
      await expect(app.locator('[data-universal-wire-story]').first()).toBeVisible({ timeout: 15_000 });
      await expect(app.locator('[data-universal-wire-empty-state]')).toHaveCount(0);
    }
  } finally {
    await context.close();
  }
});
