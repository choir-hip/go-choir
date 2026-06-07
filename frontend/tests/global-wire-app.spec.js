import { test, expect } from './helpers/fixtures.js';
import { registerPasskey } from './helpers/auth.js';

const BASE_URL = process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:5173';
const DESKTOP_BOOT_TIMEOUT_MS = Number(process.env.GO_CHOIR_DESKTOP_BOOT_TIMEOUT_MS || 120000);

function uniqueEmail() {
  return `global-wire-${Date.now()}-${Math.random().toString(36).slice(2, 8)}@example.com`;
}

async function openDeskApp(page, appId) {
  await page.locator('[data-desk-menu-button]').click();
  await expect(page.locator('[data-desk-sheet]')).toBeVisible();
  await page.locator(`[data-desk-sheet-app][data-desk-app-id="${appId}"]`).click();
}

async function applyTheme(page, id) {
  const names = {
    'futuristic-noir': 'Futuristic Noir',
    'carbon-fiber-kintsugi': 'Carbon Fiber Kintsugi',
    'london-salmon': 'London Salmon',
  };
  await page.evaluate(({ id, name }) => {
    window.dispatchEvent(new CustomEvent('choir-theme-change', {
      detail: {
        theme: {
          schema_version: 2,
          id,
          name,
        },
      },
    }));
  }, { id, name: names[id] });
}

test('Global Wire preserves StoryGraph, Style.vtext, VText fork, and contribution controls', async ({ page }) => {
  await page.goto(BASE_URL);
  await openDeskApp(page, 'global-wire');

  const app = page.locator('[data-global-wire-app]');
  await expect(app).toBeVisible();
  await expect(app.locator('[data-global-wire-story]')).toHaveCount(3);
  await expect(app.locator('[data-global-wire-story-reader]')).toContainText('Port backlog recedes');
  await expect(app.locator('[data-global-wire-evidence] [data-source-tier="lead"]')).toContainText('Port authority throughput bulletin');
  await expect(app.locator('[data-global-wire-story-graph]')).toContainText('Grid operators add reserve alerts');

  await app.locator('[data-global-wire-style-switcher] button').filter({ hasText: 'Audit' }).click();
  await expect(app.locator('[data-global-wire-style-switcher]')).toContainText('Cites Style.vtext: Claim Audit');
  await expect(app.locator('[data-global-wire-style-switcher]')).toContainText('evidence manifest unchanged');

  await app.locator('[data-global-wire-open-vtext]').click();
  const vtext = page.locator('[data-vtext-editor]').last();
  await expect(vtext).toBeVisible();
  await expect(vtext).toContainText('Source Manifest');
  await expect(vtext).toContainText('Style source: Style.vtext: Claim Audit');
});

test('Global Wire fork and contribution create owner-scoped VTexts when signed in', async ({ page, authenticator }) => {
  test.skip(process.env.GLOBAL_WIRE_AUTH_PROOF !== '1', 'Set GLOBAL_WIRE_AUTH_PROOF=1 for deployed auth-backed ownership proof.');
  const email = uniqueEmail();
  await page.goto(BASE_URL);
  await registerPasskey(page, email, BASE_URL);
  await page.reload();
  await page.locator('[data-desktop]').waitFor({ state: 'visible', timeout: DESKTOP_BOOT_TIMEOUT_MS });

  await openDeskApp(page, 'global-wire');
  const app = page.locator('[data-global-wire-app]');
  await expect(app).toBeVisible();
  await expect(app).toHaveAttribute('data-global-wire-data-source', 'durable-storygraph');
  await expect(app.locator('[data-global-wire-state]')).toContainText('durable-storygraph');

  const storyGraph = await page.evaluate(async () => {
    const res = await fetch('/api/global-wire/stories', { credentials: 'include' });
    if (!res.ok) throw new Error(`load durable StoryGraph failed: ${res.status}`);
    return res.json();
  });
  const leadSource = storyGraph.stories?.[0]?.manifest?.lead?.[0];
  expect(leadSource?.content_id).toBeTruthy();
  const sourceItem = await page.evaluate(async (contentId) => {
    const res = await fetch(`/api/content/items/${contentId}`, { credentials: 'include' });
    if (!res.ok) throw new Error(`load SourceItem failed: ${res.status}`);
    return res.json();
  }, leadSource.content_id);
  expect(sourceItem.app_hint).toBe('global-wire');
  expect(sourceItem.metadata?.schema).toBe('choir.global_wire_source_item.v1');

  await app.locator('[data-global-wire-fork-story]').click();
  await expect(page.locator('[data-vtext-editor]').last()).toContainText('My Edit');
  await expect(page.locator('[data-vtext-editor]').last()).toContainText('User edits create user-owned versions');

  await openDeskApp(page, 'global-wire');
  await app.locator('[data-global-wire-contribution] textarea').fill('Add a local utility filing as a qualifying source before reconciliation.');
  await app.locator('[data-global-wire-submit-contribution]').click();
  await expect(page.locator('[data-vtext-editor]').last()).toContainText('Research/Reconciliation State');
  await expect(app.locator('[data-global-wire-contribution-list]')).toContainText('pending-researcher-review');

  const docs = await page.evaluate(async () => {
    const res = await fetch('/api/vtext/documents', { credentials: 'include' });
    if (!res.ok) throw new Error(`list documents failed: ${res.status}`);
    return res.json();
  });
  const titles = (docs.documents || []).map((doc) => doc.title || '');
  expect(titles.some((title) => title.startsWith('My version of Port backlog recedes'))).toBeTruthy();
  expect(titles.some((title) => title.startsWith('Contribution: Port backlog recedes'))).toBeTruthy();

  const queue = await page.evaluate(async () => {
    const res = await fetch('/api/global-wire/contributions?story_id=story-supply-resilience', { credentials: 'include' });
    if (!res.ok) throw new Error(`list contributions failed: ${res.status}`);
    return res.json();
  });
  const queuedContribution = (queue.contributions || []).find((item) => item.research_state === 'pending-researcher-review');
  expect(queuedContribution).toBeTruthy();
  expect(queuedContribution.source_content_id).toBeTruthy();
  const contributionSource = await page.evaluate(async (contentId) => {
    const res = await fetch(`/api/content/items/${contentId}`, { credentials: 'include' });
    if (!res.ok) throw new Error(`load contribution SourceItem failed: ${res.status}`);
    return res.json();
  }, queuedContribution.source_content_id);
  expect(contributionSource.metadata?.schema).toBe('choir.global_wire_user_source_contribution.v1');
});

for (const themeId of ['futuristic-noir', 'carbon-fiber-kintsugi', 'london-salmon']) {
  test(`Global Wire renders core views in ${themeId}`, async ({ page }) => {
    await page.goto(BASE_URL);
    await applyTheme(page, themeId);
    await openDeskApp(page, 'global-wire');

    const app = page.locator('[data-global-wire-app]');
    await expect(app.locator('[data-global-wire-front-page]')).toBeVisible();
    await expect(app.locator('[data-global-wire-story-reader]')).toBeVisible();
    await expect(app.locator('[data-global-wire-evidence]')).toBeVisible();
    await expect(app.locator('[data-global-wire-story-graph]')).toBeVisible();
    await expect(app.locator('[data-global-wire-contribution]')).toBeVisible();

    const overflow = await app.evaluate((node) => {
      const rect = node.getBoundingClientRect();
      return {
        width: rect.width,
        scrollWidth: node.scrollWidth,
        height: rect.height,
        scrollHeight: node.scrollHeight,
      };
    });
    expect(overflow.scrollWidth).toBeLessThanOrEqual(overflow.width + 2);
    expect(overflow.scrollHeight).toBeGreaterThan(0);
  });
}
