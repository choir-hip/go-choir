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

async function focusDeskApp(page, appId, title) {
  await page.locator(`[data-window-tray-item][title="${title}"]`).click();
  await expect(page.locator(`[data-window][data-window-app-id="${appId}"]`)).toHaveAttribute('data-window-active', 'true');
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
  await expect(app.locator('[data-global-wire-source-refresh]')).toBeVisible();
  await expect(app.locator('[data-global-wire-ask-choir]')).toBeVisible();
  await expect(app.locator('[data-global-wire-autoradio]')).toBeVisible();

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

  const composeStyleResponsePromise = page.waitForResponse((response) =>
    new URL(response.url()).pathname === '/api/global-wire/style-sources' && response.request().method() === 'POST'
  );
  await app.locator('[data-global-wire-compose-style]').click();
  const composeStyleResponse = await composeStyleResponsePromise;
  expect(composeStyleResponse.status()).toBe(201);
  const composedStyle = await composeStyleResponse.json();
  expect(composedStyle.style?.doc_id).toBe(composedStyle.document?.doc_id);
  expect(composedStyle.projection?.style_id).toBe(composedStyle.style?.id);
  await expect(app.locator('[data-global-wire-style-source-status]')).toContainText('Composed Style.vtext source created');
  await expect(app.locator('[data-global-wire-style-switcher]')).toContainText('Hybrid');
  const composedStyleDoc = await page.evaluate(async (docId) => {
    const res = await fetch(`/api/vtext/documents/${docId}`, { credentials: 'include' });
    if (!res.ok) throw new Error(`load composed Style.vtext failed: ${res.status}`);
    return res.json();
  }, composedStyle.document.doc_id);
  expect(composedStyleDoc.revisions?.[0]?.content || composedStyle.revision?.content || '').toContain('Parent Style.vtext Sources');
  const composedStoryGraph = await page.evaluate(async (styleId) => {
    const res = await fetch('/api/global-wire/stories', { credentials: 'include' });
    if (!res.ok) throw new Error(`load composed StoryGraph failed: ${res.status}`);
    const graph = await res.json();
    const story = (graph.stories || []).find((item) => item.id === 'story-supply-resilience');
    return {
      style: (story?.style_sources || []).find((item) => item.id === styleId),
      projectionDocId: story?.projection_vtext_docs?.[styleId],
      projectionText: story?.projections?.[styleId],
    };
  }, composedStyle.style.id);
  expect(composedStoryGraph.style?.doc_id).toBe(composedStyle.document.doc_id);
  expect(composedStoryGraph.projectionDocId).toBeTruthy();
  expect(composedStoryGraph.projectionText).toContain('composed/replacement Style.vtext source');

  await app.locator('[data-global-wire-fork-story]').click();
  await expect(page.locator('[data-vtext-editor]').last()).toContainText('My Edit');
  await expect(page.locator('[data-vtext-editor]').last()).toContainText('User edits create user-owned versions');

  await openDeskApp(page, 'global-wire');
  const contributionText = `Add a local utility filing as a qualifying source before reconciliation (${email}).`;
  await app.locator('[data-global-wire-contribution] textarea').fill(contributionText);
  const contributionResponsePromise = page.waitForResponse((response) =>
    new URL(response.url()).pathname === '/api/global-wire/contributions' && response.request().method() === 'POST'
  );
  await app.locator('[data-global-wire-submit-contribution]').click();
  const contributionResponse = await contributionResponsePromise;
  expect(contributionResponse.status()).toBe(201);
  const queuedContribution = await contributionResponse.json();
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
  const listedContribution = (queue.contributions || []).find((item) => item.id === queuedContribution.id);
  expect(listedContribution).toBeTruthy();
  expect(queuedContribution.source_content_id).toBeTruthy();
  const contributionSource = await page.evaluate(async (contentId) => {
    const res = await fetch(`/api/content/items/${contentId}`, { credentials: 'include' });
    if (!res.ok) throw new Error(`load contribution SourceItem failed: ${res.status}`);
    return res.json();
  }, queuedContribution.source_content_id);
  expect(contributionSource.metadata?.schema).toBe('choir.global_wire_user_source_contribution.v1');

  await openDeskApp(page, 'global-wire');
  const contributionCard = app.locator('[data-global-wire-reconciliation-item]').filter({ hasText: contributionText });
  await expect(contributionCard.locator('[data-global-wire-reconciliation-source]')).toContainText('Contribution source');
  await contributionCard.locator('[data-global-wire-reconcile-accept]').evaluate((button) => button.click());
  await expect(contributionCard.locator('[data-global-wire-reconciliation-decision]')).toContainText('accepted');
  await expect(contributionCard.locator('[data-global-wire-graph-candidate]')).toContainText('source-manifest-update');
  await expect(contributionCard.locator('[data-global-wire-graph-candidate]')).toContainText('shared-source-neighborhood');

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

  const sourceRefresh = await page.evaluate(async () => {
    const res = await fetch('/api/global-wire/source-refresh', {
      method: 'POST',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        story_id: 'story-supply-resilience',
        query: 'port congestion',
        max_results: 2,
      }),
    });
    const body = await res.json();
    return { statusCode: res.status, body };
  });
  expect([201, 200, 502, 503]).toContain(sourceRefresh.statusCode);
  expect(['candidate-review', 'no-visible-change', 'no-evidence', 'unavailable']).toContain(sourceRefresh.body.status);
  expect(sourceRefresh.body.refresh_run?.story_id).toBe('story-supply-resilience');
  if (sourceRefresh.statusCode === 201) {
    expect(sourceRefresh.body.content_item?.source_type).toBe('source_service_item');
    expect(sourceRefresh.body.contribution?.research_state).toBe('accepted-for-graph-review');
    expect(sourceRefresh.body.decision?.decision).toBe('accepted');
    expect(sourceRefresh.body.candidate?.status).toBe('candidate-review');
    expect(sourceRefresh.body.claim_record?.status).toBe('research-review-required');
    expect(sourceRefresh.body.claim_record?.candidate_id).toBe(sourceRefresh.body.candidate?.id);
    expect(sourceRefresh.body.claim_record?.uncertainty_state).toBeTruthy();
    expect(sourceRefresh.body.research_task?.claim_id).toBe(sourceRefresh.body.claim_record?.id);
    expect(sourceRefresh.body.research_task?.status).toBe('open');
    expect(sourceRefresh.body.refresh_run?.update_classification).toBeTruthy();
    expect(sourceRefresh.body.refresh_run?.storygraph_action).toBeTruthy();
    expect(sourceRefresh.body.refresh_run?.projection_action).toBeTruthy();
    expect(sourceRefresh.body.refresh_run?.candidate_id).toBe(sourceRefresh.body.candidate?.id);
    const refreshQueue = await page.evaluate(async (candidateId) => {
      const res = await fetch('/api/global-wire/reconciliation?story_id=story-supply-resilience', { credentials: 'include' });
      if (!res.ok) throw new Error(`load refresh claim queue failed: ${res.status}`);
      const list = await res.json();
      const claimRecords = (list.claim_records || []).filter((item) => item.candidate_id === candidateId);
      const researchTasks = (list.research_tasks || []).filter((item) => claimRecords.some((claim) => claim.id === item.claim_id));
      return { claimRecords, researchTasks };
    }, sourceRefresh.body.candidate.id);
    expect(refreshQueue.claimRecords.length).toBeGreaterThanOrEqual(1);
    expect(refreshQueue.claimRecords[0].status).toBe('research-review-required');
    expect(refreshQueue.researchTasks.length).toBeGreaterThanOrEqual(1);
    expect(refreshQueue.researchTasks[0].status).toBe('open');
  } else if (sourceRefresh.body.status === 'no-visible-change') {
    expect(sourceRefresh.body.content_item?.source_type).toBe('source_service_item');
    expect(sourceRefresh.body.refresh_run?.update_classification).toBe('no-visible-change');
    expect(sourceRefresh.body.refresh_run?.storygraph_action).toBe('no-storygraph-change');
    expect(sourceRefresh.body.refresh_run?.candidate_id || '').toBe('');
  } else {
    expect(sourceRefresh.body.message).toBeTruthy();
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

  const candidateCard = app.locator(
    `[data-global-wire-graph-candidate][data-global-wire-candidate-id="${reconciliation.candidate.id}"]`
  );
  await candidateCard.locator('[data-global-wire-promote-candidate]').evaluate((button) => button.click());
  await expect(candidateCard.locator('[data-global-wire-graph-promotion]')).toContainText('promoted');
  await expect(candidateCard.locator('[data-global-wire-graph-promotion]')).toContainText('appended source_content_id');
  await expect(candidateCard.locator('[data-global-wire-projection-review]').first()).toContainText('projection-review-required');
  const draftResponsePromise = page.waitForResponse((response) =>
    new URL(response.url()).pathname === '/api/global-wire/projection-reviews' && response.request().method() === 'POST'
  );
  const draftButton = candidateCard.locator('[data-global-wire-create-projection-draft]').first();
  await expect(draftButton).toBeVisible();
  await draftButton.click();
  const draftResponse = await draftResponsePromise;
  expect(draftResponse.status()).toBe(201);
  const draftPayload = await draftResponse.json();
  await expect(page.locator(`[data-vtext-doc-id="${draftPayload.document.doc_id}"]`)).toContainText(
    'Draft state: review draft, not platform publication'
  );

  await focusDeskApp(page, 'global-wire', 'Global Wire');
  const approvalResponsePromise = page.waitForResponse((response) => {
    if (new URL(response.url()).pathname !== '/api/global-wire/projection-reviews' || response.request().method() !== 'POST') {
      return false;
    }
    return response.request().postDataJSON()?.action === 'approve';
  });
  const approveButton = candidateCard.locator(
    `[data-global-wire-approve-projection-draft][data-global-wire-projection-review-id="${draftPayload.review.id}"]`
  );
  await expect(approveButton).toBeVisible();
  await approveButton.click();
  const approvalResponse = await approvalResponsePromise;
  expect(approvalResponse.status()).toBe(200);
  const approvalPayload = await approvalResponse.json();
  expect(approvalPayload.review.status).toBe('approved');
  expect(approvalPayload.review.approved_revision_id).toBe(approvalPayload.revision.revision_id);
  await expect(page.locator(`[data-vtext-doc-id="${approvalPayload.document.doc_id}"]`)).toContainText(
    'Review status: approved'
  );

  const promoted = await page.evaluate(async ({ candidateId, sourceContentId }) => {
    const listRes = await fetch('/api/global-wire/reconciliation?story_id=story-supply-resilience', {
      credentials: 'include',
    });
    if (!listRes.ok) throw new Error(`list promoted reconciliation failed: ${listRes.status}`);
    const list = await listRes.json();
    const candidate = (list.candidates || []).find((item) => item.id === candidateId);
    const promotion = (list.promotions || []).find((item) => item.candidate_id === candidateId);
    const projectionReviews = (list.projection_reviews || []).filter((item) => item.candidate_id === candidateId);
    const storyRes = await fetch('/api/global-wire/stories', { credentials: 'include' });
    if (!storyRes.ok) throw new Error(`load promoted StoryGraph failed: ${storyRes.status}`);
    const storyGraphAfter = await storyRes.json();
    const story = (storyGraphAfter.stories || []).find((item) => item.id === 'story-supply-resilience');
    const source = (story?.manifest?.supporting || []).find((item) => item.content_id === sourceContentId);
    const approvedProjection = story?.projections?.[projectionReviews.find((item) => item.status === 'approved')?.style_id || ''];
    return { candidate, promotion, projectionReviews, source, approvedProjection };
  }, { candidateId: reconciliation.candidate.id, sourceContentId: queuedContribution.source_content_id });
  expect(promoted.candidate.status).toBe('promoted-to-storygraph');
  expect(promoted.promotion.decision).toBe('promoted');
  expect(promoted.projectionReviews.length).toBeGreaterThanOrEqual(1);
  const approvedReview = promoted.projectionReviews.find((item) => item.status === 'approved');
  expect(approvedReview?.draft_story_doc_id).toBeTruthy();
  expect(approvedReview?.approved_revision_id).toBeTruthy();
  expect(promoted.approvedProjection).toContain('Review status: approved');
  expect(promoted.source.content_id).toBe(queuedContribution.source_content_id);

  await focusDeskApp(page, 'global-wire', 'Global Wire');
  const askResponsePromise = page.waitForResponse((response) =>
    new URL(response.url()).pathname === '/api/prompt-bar' && response.request().method() === 'POST'
  );
  await app.locator('[data-global-wire-ask-choir]').click();
  const askResponse = await askResponsePromise;
  expect(askResponse.status()).toBe(202);
  const askPayload = askResponse.request().postDataJSON();
  expect(askPayload.text).toContain('StoryGraph id: story-supply-resilience');
  expect(askPayload.text).toContain('Source Manifest:');
  expect(askPayload.text).toContain('Style.vtext source:');
  await expect(app.locator('[data-global-wire-story-action-status]')).toContainText('Ask Choir submitted');
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
