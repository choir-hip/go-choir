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

async function ensureDeskApp(page, appId, title) {
  if (await page.locator(`[data-window-tray-item][title="${title}"]`).count()) {
    await focusDeskApp(page, appId, title);
    return;
  }
  await openDeskApp(page, appId);
}

async function fetchJSON(page, path) {
  return page.evaluate(async (requestPath) => {
    const res = await fetch(requestPath, { credentials: 'include' });
    if (!res.ok) {
      const body = await res.text();
      throw new Error(`${requestPath} failed: ${res.status} ${body}`);
    }
    return res.json();
  }, path);
}

async function postJSON(page, path, body) {
  return page.evaluate(async ({ requestPath, payload }) => {
    const res = await fetch(requestPath, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'include',
      body: JSON.stringify(payload),
    });
    if (!res.ok) {
      const responseBody = await res.text();
      throw new Error(`${requestPath} failed: ${res.status} ${responseBody}`);
    }
    return res.json();
  }, { requestPath: path, payload: body });
}

async function waitForPromptDecision(page, submissionId, timeout = 120_000) {
  const deadline = Date.now() + timeout;
  while (Date.now() < deadline) {
    const status = await fetchJSON(page, `/api/prompt-bar/submissions/${encodeURIComponent(submissionId)}`);
    if (status.decision) return status.decision;
    if (['failed', 'blocked', 'cancelled'].includes(status.state)) {
      throw new Error(`prompt submission ${submissionId} ended as ${status.state}: ${status.error || ''}`);
    }
    await page.waitForTimeout(1000);
  }
  throw new Error(`prompt submission ${submissionId} did not produce a decision`);
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
  const app = page.locator('[data-window][data-window-app-id="global-wire"]').first().locator('[data-global-wire-app]');
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
  let publicationArtifactId = '';
  if (sourceRefresh.statusCode === 201) {
    expect(sourceRefresh.body.content_item?.source_type).toBe('source_service_item');
    expect(sourceRefresh.body.contribution?.research_state).toBe('accepted-for-graph-review');
    expect(sourceRefresh.body.decision?.decision).toBe('accepted');
    expect(sourceRefresh.body.candidate?.status).toBe('candidate-review');
    expect(sourceRefresh.body.claim_record?.status).toBe('research-review-required');
    expect(sourceRefresh.body.claim_record?.candidate_id).toBe(sourceRefresh.body.candidate?.id);
    expect(sourceRefresh.body.claim_record?.uncertainty_state).toBeTruthy();
    expect(sourceRefresh.body.source_review_signal?.claim_id).toBe(sourceRefresh.body.claim_record?.id);
    expect(sourceRefresh.body.source_review_signal?.candidate_id).toBe(sourceRefresh.body.candidate?.id);
    expect(sourceRefresh.body.source_review_signal?.update_classification).toBe(sourceRefresh.body.refresh_run?.update_classification);
    expect(sourceRefresh.body.source_review_signal?.overlap_state).toBeTruthy();
    expect(sourceRefresh.body.source_review_signal?.contradiction_state).toBeTruthy();
    expect(sourceRefresh.body.source_review_signal?.evidence_refs || []).toContain(`claim:${sourceRefresh.body.claim_record?.id}`);
    expect(sourceRefresh.body.research_task?.claim_id).toBe(sourceRefresh.body.claim_record?.id);
    expect(sourceRefresh.body.research_task?.status).toBe('open');
    expect(sourceRefresh.body.extraction_artifact?.claim_id).toBe(sourceRefresh.body.claim_record?.id);
    expect(sourceRefresh.body.extraction_artifact?.source_content_id).toBe(sourceRefresh.body.content_item?.content_id);
    expect(sourceRefresh.body.extraction_artifact?.status).toBe('provisional-review');
    expect(sourceRefresh.body.extraction_artifact?.entities?.length || 0).toBeGreaterThan(0);
    expect(sourceRefresh.body.extraction_artifact?.events?.length || 0).toBeGreaterThan(0);
    expect(sourceRefresh.body.extraction_artifact?.timeline?.length || 0).toBeGreaterThan(0);
    expect(sourceRefresh.body.refresh_run?.update_classification).toBeTruthy();
    expect(sourceRefresh.body.refresh_run?.storygraph_action).toBeTruthy();
    expect(sourceRefresh.body.refresh_run?.projection_action).toBeTruthy();
    expect(sourceRefresh.body.refresh_run?.candidate_id).toBe(sourceRefresh.body.candidate?.id);
    const refreshQueue = await page.evaluate(async (candidateId) => {
      const res = await fetch('/api/global-wire/reconciliation?story_id=story-supply-resilience', { credentials: 'include' });
      if (!res.ok) throw new Error(`load refresh claim queue failed: ${res.status}`);
      const list = await res.json();
      const claimRecords = (list.claim_records || []).filter((item) => item.candidate_id === candidateId);
      const sourceReviewSignals = (list.source_review_signals || []).filter((item) => claimRecords.some((claim) => claim.id === item.claim_id));
      const researchTasks = (list.research_tasks || []).filter((item) => claimRecords.some((claim) => claim.id === item.claim_id));
      const extractionArtifacts = (list.extraction_artifacts || []).filter((item) => claimRecords.some((claim) => claim.id === item.claim_id));
      const dossier = (list.source_dossiers || []).find((item) => item.story_id === 'story-supply-resilience');
      return { claimRecords, sourceReviewSignals, researchTasks, extractionArtifacts, dossier };
    }, sourceRefresh.body.candidate.id);
    expect(refreshQueue.claimRecords.length).toBeGreaterThanOrEqual(1);
    expect(refreshQueue.claimRecords[0].status).toBe('research-review-required');
    expect(refreshQueue.sourceReviewSignals.length).toBeGreaterThanOrEqual(1);
    expect(refreshQueue.sourceReviewSignals[0].status).toBe('review-signal-open');
    expect(refreshQueue.sourceReviewSignals[0].overlap_state).toBeTruthy();
    expect(refreshQueue.dossier?.source_review_signals?.length || 0).toBeGreaterThanOrEqual(1);
    expect(refreshQueue.dossier?.claim_dossiers?.[0]?.source_review_signal_ids || []).toContain(refreshQueue.sourceReviewSignals[0].id);
    expect(refreshQueue.researchTasks.length).toBeGreaterThanOrEqual(1);
    expect(refreshQueue.researchTasks[0].status).toBe('open');
    expect(refreshQueue.extractionArtifacts.length).toBeGreaterThanOrEqual(1);
    expect(refreshQueue.extractionArtifacts[0].status).toBe('provisional-review');
    await app.locator('[data-global-wire-source-search-input]').fill('port congestion');
    const uiRefreshPromise = page.waitForResponse((response) =>
      new URL(response.url()).pathname === '/api/global-wire/source-refresh' && response.request().method() === 'POST'
    );
    await app.locator('[data-global-wire-source-refresh]').click();
    const uiRefreshResponse = await uiRefreshPromise;
    expect([201, 200, 502, 503]).toContain(uiRefreshResponse.status());
    if (uiRefreshResponse.status() === 201) {
      await expect(app.locator('[data-global-wire-source-dossier-claims]')).toContainText('signals:');
      await expect(app.locator('[data-global-wire-source-review-signal]').first()).toBeVisible();
    }
    const researchTaskLifecycle = await page.evaluate(async (taskId) => {
      const res = await fetch('/api/global-wire/research-tasks', {
        method: 'POST',
        credentials: 'include',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          task_id: taskId,
          action: 'complete',
          evidence_level: 'reconciliation-level',
          evidence_summary: 'Playwright completed source-review evidence; reconciliation can use it without mutating the platform StoryGraph.',
          reviewer_note: 'playwright-auth-proof',
        }),
      });
      const body = await res.json();
      return { statusCode: res.status, body };
    }, refreshQueue.researchTasks[0].id);
    expect(researchTaskLifecycle.statusCode).toBe(201);
    expect(researchTaskLifecycle.body.task.status).toBe('completed');
    expect(researchTaskLifecycle.body.evidence.task_id).toBe(refreshQueue.researchTasks[0].id);
    expect(researchTaskLifecycle.body.evidence.status).toBe('completed');
    const completedQueue = await page.evaluate(async (taskId) => {
      const res = await fetch('/api/global-wire/reconciliation?story_id=story-supply-resilience', { credentials: 'include' });
      if (!res.ok) throw new Error(`load completed research queue failed: ${res.status}`);
      const list = await res.json();
      return {
        task: (list.research_tasks || []).find((item) => item.id === taskId),
        evidence: (list.research_evidence || []).filter((item) => item.task_id === taskId),
      };
    }, refreshQueue.researchTasks[0].id);
    expect(completedQueue.task.status).toBe('completed');
    expect(completedQueue.evidence.length).toBeGreaterThanOrEqual(1);
    expect(completedQueue.evidence[0].summary).toContain('without mutating');
    const researchEvidenceHandoff = await page.evaluate(async (evidenceId) => {
      const res = await fetch('/api/global-wire/research-evidence', {
        method: 'POST',
        credentials: 'include',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          evidence_id: evidenceId,
          decision: 'accept',
          note: 'playwright-auth-proof accepted completed evidence for review',
        }),
      });
      const body = await res.json();
      return { statusCode: res.status, body };
    }, completedQueue.evidence[0].id);
    expect(researchEvidenceHandoff.statusCode).toBe(201);
    expect(researchEvidenceHandoff.body.decision.decision).toBe('accepted-for-review');
    expect(researchEvidenceHandoff.body.decision.result_state).toBe('ready-for-platform-review');
    expect(researchEvidenceHandoff.body.candidate.status).toBe('research-evidence-accepted');
    const handoffQueue = await page.evaluate(async (evidenceId) => {
      const res = await fetch('/api/global-wire/reconciliation?story_id=story-supply-resilience', { credentials: 'include' });
      if (!res.ok) throw new Error(`load research handoff queue failed: ${res.status}`);
      const list = await res.json();
      return {
        decision: (list.research_decisions || []).find((item) => item.evidence_id === evidenceId),
        candidate: (list.candidates || []).find((item) => item.id),
      };
    }, completedQueue.evidence[0].id);
    expect(handoffQueue.decision.result_state).toBe('ready-for-platform-review');
    expect(handoffQueue.candidate.status).toBeTruthy();
    const publicationUpdate = await page.evaluate(async (researchDecisionId) => {
      const res = await fetch('/api/global-wire/publication-updates', {
        method: 'POST',
        credentials: 'include',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          research_decision_id: researchDecisionId,
        }),
      });
      const body = await res.json();
      return { statusCode: res.status, body };
    }, researchEvidenceHandoff.body.decision.id);
    expect(publicationUpdate.statusCode).toBe(201);
    expect(publicationUpdate.body.update.status).toBe('packaged-for-publication-review');
    expect(publicationUpdate.body.update.research_decision_id).toBe(researchEvidenceHandoff.body.decision.id);
    expect(publicationUpdate.body.update.extraction_ids?.length || 0).toBeGreaterThanOrEqual(1);
    expect(publicationUpdate.body.update.rollback_refs.length).toBeGreaterThanOrEqual(4);
    const publicationQueue = await page.evaluate(async (updateId) => {
      const res = await fetch('/api/global-wire/reconciliation?story_id=story-supply-resilience', { credentials: 'include' });
      if (!res.ok) throw new Error(`load publication update queue failed: ${res.status}`);
      const list = await res.json();
      return (list.publication_updates || []).find((item) => item.id === updateId);
    }, publicationUpdate.body.update.id);
    expect(publicationQueue.status).toBe('packaged-for-publication-review');
    expect(publicationQueue.summary).toContain('does not publish or mutate');
    expect(publicationQueue.extraction_ids?.length || 0).toBeGreaterThanOrEqual(1);
    const publicationArtifact = await page.evaluate(async (updateId) => {
      const res = await fetch('/api/global-wire/publication-artifacts', {
        method: 'POST',
        credentials: 'include',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          update_id: updateId,
          channel: 'newsletter',
        }),
      });
      const body = await res.json();
      return { statusCode: res.status, body };
    }, publicationUpdate.body.update.id);
    expect(publicationArtifact.statusCode).toBe(201);
    expect(publicationArtifact.body.artifact.status).toBe('publication-review-ready');
    expect(publicationArtifact.body.artifact.update_id).toBe(publicationUpdate.body.update.id);
    expect(publicationArtifact.body.artifact.channel).toBe('newsletter');
    expect(publicationArtifact.body.artifact.citation_refs.length).toBeGreaterThanOrEqual(5);
    expect(publicationArtifact.body.artifact.extraction_ids?.length || 0).toBeGreaterThanOrEqual(1);
    expect(publicationArtifact.body.artifact.body).toContain('does not mutate the platform story');
    publicationArtifactId = publicationArtifact.body.artifact.id;
    const artifactQueue = await page.evaluate(async (artifactId) => {
      const res = await fetch('/api/global-wire/reconciliation?story_id=story-supply-resilience', { credentials: 'include' });
      if (!res.ok) throw new Error(`load publication artifact queue failed: ${res.status}`);
      const list = await res.json();
      return (list.publication_artifacts || []).find((item) => item.id === artifactId);
    }, publicationArtifact.body.artifact.id);
    expect(artifactQueue.status).toBe('publication-review-ready');
    expect(artifactQueue.citation_refs.length).toBeGreaterThanOrEqual(5);
    const publicationFeed = await page.evaluate(async (artifactId) => {
      const res = await fetch('/api/global-wire/publication-feed?story_id=story-supply-resilience&channel=newsletter', { credentials: 'include' });
      if (!res.ok) throw new Error(`load publication feed failed: ${res.status}`);
      const list = await res.json();
      return {
        status: list.status,
        item: (list.feed_items || []).find((feedItem) => feedItem.artifact?.id === artifactId),
      };
    }, publicationArtifact.body.artifact.id);
    expect(publicationFeed.status).toBe('ready');
    expect(publicationFeed.item.artifact.status).toBe('publication-review-ready');
    expect(publicationFeed.item.story.id).toBe('story-supply-resilience');
    expect(publicationFeed.item.source_item?.content_id).toBeTruthy();
    expect(publicationFeed.item.citation_count).toBeGreaterThanOrEqual(5);
    expect(publicationFeed.item.rollback_count).toBeGreaterThanOrEqual(5);
    await page.reload();
    await page.locator('[data-desktop]').waitFor({ state: 'visible', timeout: DESKTOP_BOOT_TIMEOUT_MS });
    await openDeskApp(page, 'global-wire');
    await expect(app.locator('[data-global-wire-publication-feed-item]').first()).toBeVisible();
    await expect(app.locator('[data-global-wire-publication-feed-provenance]').first()).toContainText('citations:');
    const approveResponsePromise = page.waitForResponse((response) =>
      new URL(response.url()).pathname === '/api/global-wire/publication-artifact-reviews' && response.request().method() === 'POST'
    );
    await app.locator('[data-global-wire-approve-publication-artifact]').first().click();
    const approveResponse = await approveResponsePromise;
    expect(approveResponse.status()).toBe(201);
    const approvePayload = await approveResponse.json();
    expect(approvePayload.artifact.id).toBe(publicationArtifact.body.artifact.id);
    expect(approvePayload.artifact.status).toBe('publication-approved');
    await expect(app.locator('[data-global-wire-publication-feed-item]').first()).toContainText('publication-approved');
    const deliveryResponsePromise = page.waitForResponse((response) =>
      new URL(response.url()).pathname === '/api/global-wire/publication-deliveries' && response.request().method() === 'POST'
    );
    await app.locator('[data-global-wire-create-publication-delivery]').first().click();
    const deliveryResponse = await deliveryResponsePromise;
    expect(deliveryResponse.status()).toBe(201);
    const deliveryPayload = await deliveryResponse.json();
    expect(deliveryPayload.delivery.artifact_id).toBe(publicationArtifact.body.artifact.id);
    expect(deliveryPayload.delivery.status).toBe('delivery-ready');
    expect(deliveryPayload.delivery.delivery_ref).toContain('global-wire/story-supply-resilience/publications/');
    expect(deliveryPayload.delivery.citation_count).toBeGreaterThanOrEqual(5);
    await expect(app.locator('[data-global-wire-publication-delivery]').first()).toContainText('delivery-ready');
    await expect(app.locator('[data-global-wire-publication-delivery-provenance]').first()).toContainText('delivery citations:');
    const deliveryDetailResponsePromise = page.waitForResponse((response) =>
      new URL(response.url()).pathname === `/api/global-wire/publication-deliveries/${deliveryPayload.delivery.id}` && response.request().method() === 'GET'
    );
    await app.locator('[data-global-wire-open-publication-delivery]').first().click();
    const deliveryDetailResponse = await deliveryDetailResponsePromise;
    expect(deliveryDetailResponse.status()).toBe(200);
    const deliveryDetail = await deliveryDetailResponse.json();
    expect(deliveryDetail.delivery.id).toBe(deliveryPayload.delivery.id);
    expect(deliveryDetail.artifact.id).toBe(publicationArtifact.body.artifact.id);
    expect(deliveryDetail.source_item?.content_id).toBeTruthy();
    await expect(app.locator('[data-global-wire-publication-delivery-detail]')).toContainText('delivery-ready');
    await expect(app.locator('[data-global-wire-publication-delivery-detail-citations]')).toContainText('citations:');
    await expect(app.locator('[data-global-wire-publication-delivery-detail-rollback]')).toContainText('rollback refs:');
    const autoradioScriptResponsePromise = page.waitForResponse((response) =>
      new URL(response.url()).pathname === '/api/global-wire/autoradio-scripts' && response.request().method() === 'POST'
    );
    await app.locator('[data-global-wire-create-autoradio-script]').first().click();
    const autoradioScriptResponse = await autoradioScriptResponsePromise;
    expect(autoradioScriptResponse.status()).toBe(201);
    const autoradioScriptPayload = await autoradioScriptResponse.json();
    expect(autoradioScriptPayload.script.artifact_id).toBe(publicationArtifact.body.artifact.id);
    expect(autoradioScriptPayload.script.status).toBe('script-ready');
    expect(autoradioScriptPayload.script.script_body).toContain(publicationArtifact.body.artifact.body);
    expect(autoradioScriptPayload.script.citation_count).toBeGreaterThanOrEqual(5);
    expect(autoradioScriptPayload.source_item?.content_id).toBeTruthy();
    await expect(app.locator('[data-global-wire-autoradio-script]').first()).toContainText('script-ready');
    await expect(app.locator('[data-global-wire-autoradio-script]').first()).toContainText('Autoradio script for');
    await expect(app.locator('[data-global-wire-autoradio-script-provenance]').first()).toContainText('script citations:');
    const autoradioScriptQueue = await page.evaluate(async (scriptId) => {
      const res = await fetch('/api/global-wire/reconciliation?story_id=story-supply-resilience', { credentials: 'include' });
      if (!res.ok) throw new Error(`load autoradio script queue failed: ${res.status}`);
      const list = await res.json();
      return (list.autoradio_scripts || []).find((item) => item.id === scriptId);
    }, autoradioScriptPayload.script.id);
    expect(autoradioScriptQueue.artifact_id).toBe(publicationArtifact.body.artifact.id);
    expect(autoradioScriptQueue.rollback_refs).toContain(`publication_artifact:${publicationArtifact.body.artifact.id}`);
    const autoradioEpisodeResponsePromise = page.waitForResponse((response) =>
      new URL(response.url()).pathname === '/api/global-wire/autoradio-episodes' && response.request().method() === 'POST'
    );
    await app.locator('[data-global-wire-create-autoradio-episode]').first().click();
    const autoradioEpisodeResponse = await autoradioEpisodeResponsePromise;
    expect(autoradioEpisodeResponse.status()).toBe(201);
    const autoradioEpisodePayload = await autoradioEpisodeResponse.json();
    expect(autoradioEpisodePayload.episode.script_id).toBe(autoradioScriptPayload.script.id);
    expect(autoradioEpisodePayload.episode.artifact_id).toBe(publicationArtifact.body.artifact.id);
    expect(autoradioEpisodePayload.episode.status).toBe('episode-ready');
    expect(autoradioEpisodePayload.episode.playback_mode).toBe('browser-speech');
    expect(autoradioEpisodePayload.episode.transcript).toContain(autoradioScriptPayload.script.script_body);
    expect(autoradioEpisodePayload.episode.rollback_refs).toContain(`autoradio_script:${autoradioScriptPayload.script.id}`);
    await expect(app.locator('[data-global-wire-autoradio-episode]').first()).toContainText('episode-ready');
    await expect(app.locator('[data-global-wire-autoradio-episode-provenance]').first()).toContainText('browser-speech');
    const autoradioEpisodeQueue = await page.evaluate(async (episodeId) => {
      const res = await fetch('/api/global-wire/reconciliation?story_id=story-supply-resilience', { credentials: 'include' });
      if (!res.ok) throw new Error(`load autoradio episode queue failed: ${res.status}`);
      const list = await res.json();
      const episode = (list.autoradio_episodes || []).find((item) => item.id === episodeId);
      const dossier = (list.source_dossiers || []).find((item) => item.story_id === 'story-supply-resilience');
      return { episode, dossier };
    }, autoradioEpisodePayload.episode.id);
    expect(autoradioEpisodeQueue.episode.script_id).toBe(autoradioScriptPayload.script.id);
    expect(autoradioEpisodeQueue.dossier.publication_refs.autoradio_episode_ids).toContain(autoradioEpisodePayload.episode.id);
    await page.evaluate(() => {
      window.__globalWireSpoken = [];
      const TestSpeechSynthesisUtterance = function SpeechSynthesisUtterance(text) {
        this.text = text;
      };
      const testSpeechSynthesis = {
        cancel() {},
        speak(utterance) {
          window.__globalWireSpoken.push(utterance.text);
          if (utterance.onstart) utterance.onstart();
          if (utterance.onend) utterance.onend();
        },
      };
      Object.defineProperty(window, 'SpeechSynthesisUtterance', {
        configurable: true,
        value: TestSpeechSynthesisUtterance,
      });
      Object.defineProperty(window, 'speechSynthesis', {
        configurable: true,
        value: testSpeechSynthesis,
      });
    });
    await app.locator('[data-global-wire-play-autoradio-episode]').first().click();
    await expect(app.locator('[data-global-wire-autoradio-playback-state]').first()).toContainText('Played');
    const spoken = await page.evaluate(() => window.__globalWireSpoken || []);
    expect(spoken[0]).toContain(autoradioScriptPayload.script.script_body);
    const deliveryExportResponsePromise = page.waitForResponse((response) =>
      new URL(response.url()).pathname === '/api/global-wire/publication-delivery-exports' && response.request().method() === 'POST'
    );
    await app.locator('[data-global-wire-create-delivery-export]').first().click();
    const deliveryExportResponse = await deliveryExportResponsePromise;
    expect(deliveryExportResponse.status()).toBe(201);
    const deliveryExportPayload = await deliveryExportResponse.json();
    expect(deliveryExportPayload.export.delivery_id).toBe(deliveryPayload.delivery.id);
    expect(deliveryExportPayload.export.artifact_id).toBe(publicationArtifact.body.artifact.id);
    expect(deliveryExportPayload.export.script_id).toBe(autoradioScriptPayload.script.id);
    expect(deliveryExportPayload.export.status).toBe('export-ready');
    expect(deliveryExportPayload.export.export_body).toContain(publicationArtifact.body.artifact.body);
    expect(deliveryExportPayload.export.export_body).toContain(autoradioScriptPayload.script.script_body);
    expect(deliveryExportPayload.script?.id).toBe(autoradioScriptPayload.script.id);
    await expect(app.locator('[data-global-wire-delivery-export]').first()).toContainText('export-ready');
    await expect(app.locator('[data-global-wire-delivery-export]').first()).toContainText('Publication Artifact');
    await expect(app.locator('[data-global-wire-delivery-export-provenance]').first()).toContainText('export format: md');
    const deliveryExportQueue = await page.evaluate(async (exportId) => {
      const res = await fetch('/api/global-wire/reconciliation?story_id=story-supply-resilience', { credentials: 'include' });
      if (!res.ok) throw new Error(`load delivery export queue failed: ${res.status}`);
      const list = await res.json();
      return (list.delivery_exports || []).find((item) => item.id === exportId);
    }, deliveryExportPayload.export.id);
    expect(deliveryExportQueue.delivery_id).toBe(deliveryPayload.delivery.id);
    expect(deliveryExportQueue.rollback_refs).toContain(`publication_delivery:${deliveryPayload.delivery.id}`);
    const publicLinkResponsePromise = page.waitForResponse((response) =>
      new URL(response.url()).pathname === '/api/global-wire/publication-public-links' && response.request().method() === 'POST'
    );
    await app.locator('[data-global-wire-create-public-link]').first().click();
    const publicLinkResponse = await publicLinkResponsePromise;
    expect(publicLinkResponse.status()).toBe(201);
    const publicLinkPayload = await publicLinkResponse.json();
    expect(publicLinkPayload.public_link.export_id).toBe(deliveryExportPayload.export.id);
    expect(publicLinkPayload.public_link.status).toBe('public-unlisted');
    expect(publicLinkPayload.public_link.route_path).toContain('/global-wire/publications/');
    expect(publicLinkPayload.public_link.feed_path).toContain('/api/global-wire/publication-public-links/');
    await expect(app.locator('[data-global-wire-public-link]').first()).toContainText('public-unlisted');
    const publicRead = await page.evaluate(async (token) => {
      const res = await fetch(`/api/global-wire/publication-public-links/${encodeURIComponent(token)}`);
      const body = await res.json();
      return { statusCode: res.status, body };
    }, publicLinkPayload.public_link.token);
    expect(publicRead.statusCode).toBe(200);
    expect(publicRead.body.public_link.owner_id || '').toBe('');
    expect(publicRead.body.public_link.export_id).toBe(deliveryExportPayload.export.id);
    expect(publicRead.body.public_link.export_body).toContain(publicationArtifact.body.artifact.body);
    expect(publicRead.body.public_link.feed_path).toBe(publicLinkPayload.public_link.feed_path);
    const publicFeed = await page.evaluate(async (feedPath) => {
      const res = await fetch(feedPath);
      return {
        statusCode: res.status,
        contentType: res.headers.get('content-type') || '',
        body: await res.text(),
      };
    }, publicLinkPayload.public_link.feed_path);
    expect(publicFeed.statusCode).toBe(200);
    expect(publicFeed.contentType).toContain('application/rss+xml');
    expect(publicFeed.body).toContain('<rss');
    expect(publicFeed.body).toContain(publicLinkPayload.public_link.title);
    expect(publicFeed.body).toContain('Citation refs:');
    expect(publicFeed.body).toContain('Rollback refs:');
    const newsletterSubscriberResponsePromise = page.waitForResponse((response) =>
      new URL(response.url()).pathname === '/api/global-wire/newsletter-subscribers' && response.request().method() === 'POST'
    );
    const newsletterIssueResponsePromise = page.waitForResponse((response) =>
      new URL(response.url()).pathname === '/api/global-wire/newsletter-issues' && response.request().method() === 'POST'
    );
    await app.locator('[data-global-wire-create-newsletter-issue]').first().click();
    const newsletterSubscriberResponse = await newsletterSubscriberResponsePromise;
    expect(newsletterSubscriberResponse.status()).toBe(201);
    const newsletterSubscriberPayload = await newsletterSubscriberResponse.json();
    expect(newsletterSubscriberPayload.subscriber.status).toBe('active');
    const newsletterIssueResponse = await newsletterIssueResponsePromise;
    expect(newsletterIssueResponse.status()).toBe(201);
    const newsletterIssuePayload = await newsletterIssueResponse.json();
    expect(newsletterIssuePayload.issue.status).toBe('issue-ready');
    expect(newsletterIssuePayload.issue.public_link_ids).toContain(publicLinkPayload.public_link.id);
    expect(newsletterIssuePayload.issue.subscriber_count).toBeGreaterThanOrEqual(1);
    expect(newsletterIssuePayload.deliveries?.[0]?.status).toBe('delivery-ready');
    expect(newsletterIssuePayload.deliveries?.[0]?.subscriber_id).toBe(newsletterSubscriberPayload.subscriber.id);
    await expect(app.locator('[data-global-wire-newsletter-issue]').first()).toContainText('issue-ready');
    await expect(app.locator('[data-global-wire-newsletter-issue-provenance]').first()).toContainText('subscribers:');
    await expect(app.locator('[data-global-wire-newsletter-delivery]').first()).toContainText('delivery-ready');
    const newsletterQueue = await page.evaluate(async (issueId) => {
      const res = await fetch('/api/global-wire/reconciliation?story_id=story-supply-resilience', { credentials: 'include' });
      if (!res.ok) throw new Error(`load newsletter queue failed: ${res.status}`);
      const list = await res.json();
      return {
        issue: (list.newsletter_issues || []).find((item) => item.id === issueId),
        delivery: (list.newsletter_deliveries || []).find((item) => item.issue_id === issueId),
        dossier: (list.source_dossiers || []).find((item) => item.story_id === 'story-supply-resilience'),
      };
    }, newsletterIssuePayload.issue.id);
    expect(newsletterQueue.issue.public_link_ids).toContain(publicLinkPayload.public_link.id);
    expect(newsletterQueue.issue.rollback_refs).toContain(`public_link:${publicLinkPayload.public_link.id}`);
    expect(newsletterQueue.delivery.status).toBe('delivery-ready');
    expect(newsletterQueue.dossier.review_state).toBe('source-dossier-ready');
    expect(newsletterQueue.dossier.claim_dossiers.length).toBeGreaterThanOrEqual(1);
    expect(newsletterQueue.dossier.extraction_ids.length).toBeGreaterThanOrEqual(1);
    expect(newsletterQueue.dossier.research_task_ids.length).toBeGreaterThanOrEqual(1);
    expect(newsletterQueue.dossier.publication_refs.newsletter_issue_ids).toContain(newsletterIssuePayload.issue.id);
    expect(newsletterQueue.dossier.publication_refs.public_link_ids).toContain(publicLinkPayload.public_link.id);
    expect(newsletterQueue.dossier.publication_refs.citation_refs.length).toBeGreaterThanOrEqual(5);
    expect(newsletterQueue.dossier.missing_fields).not.toContain('claim_dossiers');
    const sourceDossierResponse = await page.evaluate(async (issueId) => {
      const res = await fetch('/api/global-wire/source-dossiers?story_id=story-supply-resilience', { credentials: 'include' });
      if (!res.ok) throw new Error(`load source dossiers failed: ${res.status}`);
      const list = await res.json();
      const dossier = (list.dossiers || []).find((item) => item.story_id === 'story-supply-resilience');
      return { status: list.status, source: list.source, dossier, issueId };
    }, newsletterIssuePayload.issue.id);
    expect(sourceDossierResponse.status).toBe('ready');
    expect(sourceDossierResponse.source).toBe('derived-reconciliation-dossier');
    expect(sourceDossierResponse.dossier.publication_refs.newsletter_issue_ids).toContain(sourceDossierResponse.issueId);
    expect(sourceDossierResponse.dossier.manifest_tiers.find((tier) => tier.tier === 'lead').count).toBeGreaterThanOrEqual(1);
    await expect(app.locator('[data-global-wire-source-dossier]').first()).toContainText('source-dossier-ready');
    await expect(app.locator('[data-global-wire-source-dossier-claims]').first()).toContainText('claims:');
    await expect(app.locator('[data-global-wire-source-dossier-publication]').first()).toContainText('newsletter issues:');
    await expect(app.locator('[data-global-wire-source-dossier-provenance]').first()).toContainText('missing: none');
    const publicReaderURL = new URL(publicLinkPayload.public_link.route_path, page.url()).toString();
    await page.goto(publicReaderURL);
    await expect(page.locator('[data-global-wire-public-reader]')).toBeVisible();
    await expect(page.locator('[data-global-wire-public-publication]')).toContainText(publicLinkPayload.public_link.title);
    await expect(page.locator('[data-global-wire-public-publication]')).toContainText(publicationArtifact.body.artifact.body);
    await expect(page.locator('[data-global-wire-public-feed]')).toHaveAttribute('href', publicLinkPayload.public_link.feed_path);
    await expect(page.locator('[data-global-wire-public-provenance]')).toContainText('citations:');
    await expect(page.locator('[data-global-wire-public-citations]')).toContainText('story:story-supply-resilience');
    await expect(page.locator('[data-global-wire-public-rollback]')).toContainText('delivery_export:');
    await page.goto(new URL('/', publicReaderURL).toString());
    await page.locator('[data-desktop]').waitFor({ state: 'visible', timeout: DESKTOP_BOOT_TIMEOUT_MS });
    await ensureDeskApp(page, 'global-wire', 'Global Wire');
    await expect(app).toBeVisible();
  } else if (sourceRefresh.body.status === 'no-visible-change') {
    expect(sourceRefresh.body.content_item?.source_type).toBe('source_service_item');
    expect(sourceRefresh.body.refresh_run?.update_classification).toBe('no-visible-change');
    expect(sourceRefresh.body.refresh_run?.storygraph_action).toBe('no-storygraph-change');
    expect(sourceRefresh.body.refresh_run?.candidate_id || '').toBe('');
  } else {
    expect(sourceRefresh.body.message).toBeTruthy();
  }

  await focusDeskApp(page, 'global-wire', 'Global Wire');
  const fetchCycleResponsePromise = page.waitForResponse((response) =>
    new URL(response.url()).pathname === '/api/global-wire/fetch-cycles' && response.request().method() === 'POST'
  );
  await app.locator('[data-global-wire-fetch-cycle]').click();
  const fetchCycleResponse = await fetchCycleResponsePromise;
  expect([201, 503]).toContain(fetchCycleResponse.status());
  const fetchCycle = await fetchCycleResponse.json();
  expect(fetchCycle.fetch_cycle?.id).toBeTruthy();
  expect(fetchCycle.fetch_cycle?.story_ids).toContain('story-supply-resilience');
  expect(fetchCycle.registry_entries?.[0]?.story_id).toBe('story-supply-resilience');
  expect(fetchCycle.refresh_runs?.length || 0).toBeGreaterThanOrEqual(1);
  await expect(app.locator('[data-global-wire-fetch-cycle-status]')).toBeVisible();
  await expect(app.locator('[data-global-wire-fetch-cycle-runs]')).toContainText('story-supply-resilience');
  await expect(app.locator('[data-global-wire-source-standing-policy]').first()).toBeVisible();

  const schedulerResponsePromise = page.waitForResponse((response) =>
    new URL(response.url()).pathname === '/api/global-wire/fetch-cycles' && response.request().method() === 'POST'
  );
  await app.locator('[data-global-wire-scheduler-cycle]').click();
  const schedulerResponse = await schedulerResponsePromise;
  expect([201, 503]).toContain(schedulerResponse.status());
  const schedulerCycle = await schedulerResponse.json();
  expect(schedulerCycle.fetch_cycle?.id).toBeTruthy();
  expect(schedulerCycle.scheduler_run?.fetch_cycle_id).toBe(schedulerCycle.fetch_cycle?.id);
  expect(schedulerCycle.scheduler_run?.standing_policies?.length || 0).toBeGreaterThanOrEqual(1);
  expect(schedulerCycle.registry_entries?.[0]?.source_standing_policy).toBeTruthy();
  expect(schedulerCycle.registry_entries?.[0]?.cadence_seconds).toBeGreaterThanOrEqual(900);
  await expect(app.locator('[data-global-wire-source-scheduler-run]').first()).toContainText('scheduled-cycle');
  await expect(app.locator('[data-global-wire-source-schedule-cadence]').first()).toContainText('cadence');

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
  if (publicationArtifactId) {
    const autoradioResponsePromise = page.waitForResponse((response) =>
      new URL(response.url()).pathname === '/api/prompt-bar' && response.request().method() === 'POST'
    );
    await app.locator('[data-global-wire-autoradio]').click();
    const autoradioResponse = await autoradioResponsePromise;
    expect(autoradioResponse.status()).toBe(202);
    const autoradioSubmitted = await autoradioResponse.json();
    expect(autoradioSubmitted.submission_id).toBeTruthy();
    const autoradioPayload = autoradioResponse.request().postDataJSON();
    expect(autoradioPayload.text).toContain('Create an Autoradio-ready spoken brief from the selected Global Wire publication artifact');
    expect(autoradioPayload.text).toContain(`Artifact id: ${publicationArtifactId}`);
    expect(autoradioPayload.text).toContain('Citation count:');
    expect(autoradioPayload.text).toContain('Rollback count:');
    expect(autoradioPayload.text).toContain('Citation Refs:');
    expect(autoradioPayload.text).toContain('Guardrail: speak from this citeable publication artifact');
    await expect(app.locator('[data-global-wire-story-action-status]')).toContainText('Autoradio brief submitted');
    await waitForPromptDecision(page, autoradioSubmitted.submission_id);
    const acceptance = await postJSON(page, '/api/run-acceptances/synthesize', {
      target_mission_id: 'mission-global-wire-style-vtext-collaborative-storygraph-v0',
      source_prompt_or_objective: autoradioPayload.text,
      trajectory_id: autoradioSubmitted.submission_id,
      staging_url: new URL(BASE_URL).origin,
    });
    expect(acceptance.acceptance_id).toBeTruthy();
    expect(acceptance.target_mission_id).toBe('mission-global-wire-style-vtext-collaborative-storygraph-v0');
    expect(acceptance.trajectory_id).toBe(autoradioSubmitted.submission_id);
    expect(acceptance.acceptance_level).not.toBe('promotion-level');
    expect(acceptance.checkpoints?.length || 0).toBeGreaterThan(0);
    expect(acceptance.evidence_refs?.length || 0).toBeGreaterThan(0);
    expect(acceptance.verifier_contracts?.length || 0).toBeGreaterThan(0);
    const storedAcceptance = await fetchJSON(page, `/api/run-acceptances/${encodeURIComponent(acceptance.acceptance_id)}`);
    expect(storedAcceptance.acceptance_id).toBe(acceptance.acceptance_id);
  }

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
