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
  await expect(app.locator('[data-global-wire-source-search]')).toBeVisible();

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
  await app.locator('[data-global-wire-source-search-input]').fill('port congestion');
  await app.locator('[data-global-wire-source-search-submit]').click();
  const searchStatus = app.locator('[data-global-wire-source-search-status]');
  await expect(searchStatus).toContainText(/ok|no-evidence|unavailable/);
  const searchResultCount = await app.locator('[data-global-wire-source-search-results] article').count();
  if (searchResultCount > 0) {
    await expect(app.locator('[data-global-wire-source-search-results] article').first()).toContainText(/source_service_item|source artifact/);
  }

  const storyGraph = await page.evaluate(async () => {
    const res = await fetch('/api/global-wire/stories', { credentials: 'include' });
    if (!res.ok) throw new Error(`load durable StoryGraph failed: ${res.status}`);
    return res.json();
  });
  const leadSource = storyGraph.stories?.[0]?.manifest?.lead?.[0];
  const auditProjectionDoc = storyGraph.stories?.[0]?.projection_vtext_docs?.['claim-audit-style'];
  expect(leadSource?.content_id).toBeTruthy();
  expect(auditProjectionDoc).toBeTruthy();
  const sourceItem = await page.evaluate(async (contentId) => {
    const res = await fetch(`/api/content/items/${contentId}`, { credentials: 'include' });
    if (!res.ok) throw new Error(`load SourceItem failed: ${res.status}`);
    return res.json();
  }, leadSource.content_id);
  expect(sourceItem.app_hint).toBe('global-wire');
  expect(sourceItem.metadata?.schema).toBe('choir.global_wire_source_item.v1');
  const projectionDoc = await page.evaluate(async (docId) => {
    const res = await fetch(`/api/vtext/documents/${docId}`, { credentials: 'include' });
    if (!res.ok) throw new Error(`load projection VText failed: ${res.status}`);
    return res.json();
  }, auditProjectionDoc);
  expect(projectionDoc.document?.title || projectionDoc.title || '').toContain('Audit');

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

  await openDeskApp(page, 'global-wire');
  await expect(app.locator('[data-global-wire-reconciliation-source]').first()).toContainText('Contribution source');
  await app.locator('[data-global-wire-reconcile-accept]').first().evaluate((button) => button.click());
  await expect(app.locator('[data-global-wire-reconciliation-decision]').first()).toContainText('accepted');
  await expect(app.locator('[data-global-wire-graph-candidate]').first()).toContainText('source-manifest-update');
  await expect(app.locator('[data-global-wire-graph-candidate]').first()).toContainText('shared-source-neighborhood');

  const importedSourceQueue = await page.evaluate(async () => {
    const itemRes = await fetch('/api/content/items', {
      method: 'POST',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        source_type: 'text',
        media_type: 'text/markdown',
        app_hint: 'global-wire',
        title: 'Imported source for Global Wire proof',
        text_content: 'Imported source text queued as a Global Wire source contribution.',
        metadata: { schema: 'test.global_wire_imported_source' },
        provenance: { created_from: 'global_wire_playwright_proof' },
      }),
    });
    if (!itemRes.ok) throw new Error(`create imported source failed: ${itemRes.status}`);
    const item = await itemRes.json();
    const contributionRes = await fetch('/api/global-wire/contributions', {
      method: 'POST',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        story_id: 'story-supply-resilience',
        kind: 'source',
        headline: 'Port backlog recedes',
        text: 'Queue imported source content item for review.',
        source_content_id: item.content_id,
      }),
    });
    if (!contributionRes.ok) throw new Error(`queue imported source failed: ${contributionRes.status}`);
    return contributionRes.json();
  });
  expect(importedSourceQueue.source_content_id).toBeTruthy();

  const sourceServiceBridge = await page.evaluate(async () => {
    const res = await fetch('/api/global-wire/source-search', {
      method: 'POST',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        query: 'port congestion',
        max_results: 2,
        story_id: 'story-supply-resilience',
        queue_top_result: true,
      }),
    });
    const body = await res.json();
    return { statusCode: res.status, body };
  });
  expect([200, 502, 503]).toContain(sourceServiceBridge.statusCode);
  expect(['ok', 'no-evidence', 'unavailable']).toContain(sourceServiceBridge.body.status);
  if (sourceServiceBridge.body.status === 'ok') {
    expect(sourceServiceBridge.body.content_items?.[0]?.source_type).toBe('source_service_item');
    expect(sourceServiceBridge.body.content_items?.[0]?.metadata?.schema).toBe('choir.global_wire_source_service_item.v1');
    expect(sourceServiceBridge.body.contribution?.source_content_id).toBe(sourceServiceBridge.body.content_items?.[0]?.content_id);
  } else {
    expect(sourceServiceBridge.body.source).toBeTruthy();
    expect(sourceServiceBridge.body.message).toBeTruthy();
  }

  const reconciliation = await page.evaluate(async ({ contributionId, sourceContentId }) => {
    const listRes = await fetch('/api/global-wire/reconciliation?story_id=story-supply-resilience', {
      credentials: 'include',
    });
    if (!listRes.ok) throw new Error(`list reconciliation failed: ${listRes.status}`);
    const list = await listRes.json();
    const contribution = (list.contributions || []).find((item) => item.id === contributionId);
    const decision = (list.decisions || []).find((item) => item.contribution_id === contributionId);
    const candidate = (list.candidates || []).find((item) => item.contribution_id === contributionId);
    const sourceItem = list.source_items?.[sourceContentId];
    return { contribution, decision, candidate, sourceItem };
  }, { contributionId: queuedContribution.id, sourceContentId: queuedContribution.source_content_id });
  expect(reconciliation.decision.decision).toBe('accepted');
  expect(reconciliation.contribution.research_state).toBe('accepted-for-graph-review');
  expect(reconciliation.candidate.status).toBe('candidate-review');
  expect(reconciliation.candidate.source_tier).toBe('supporting');
  expect(reconciliation.candidate.edge_kind).toBe('shared-source-neighborhood');
  expect(reconciliation.candidate.source_content_id).toBe(queuedContribution.source_content_id);
  expect(reconciliation.sourceItem?.content_id).toBe(queuedContribution.source_content_id);
  expect(reconciliation.sourceItem?.metadata?.schema).toBe('choir.global_wire_user_source_contribution.v1');
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
