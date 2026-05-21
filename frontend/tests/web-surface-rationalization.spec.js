import { test, expect } from './helpers/fixtures.js';

async function openStartApp(page, appId) {
  const existing = page.locator(`[data-window-app-id="${appId}"]`);
  for (let count = await existing.count(); count > 0; count -= 1) {
    await existing.last().locator('[data-window-close]').click();
  }
  await page.locator('[data-start-button]').click();
  await page.locator(`[data-start-app-id="${appId}"]`).click();
}

test('Apps & Changes replaces manual candidate desktop entry points', async ({ desktopSession }) => {
  const { page } = desktopSession;
  const forbiddenRemoteDisplayRequests = [];
  page.on('request', (request) => {
    const url = request.url().toLowerCase();
    if (url.includes('vnc') || url.includes('webrtc') || url.includes('mjpeg') || url.includes('framebuffer')) {
      forbiddenRemoteDisplayRequests.push(url);
    }
  });

  await page.route('**/api/app-change-packages*', async (route) => {
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ packages: [] }) });
  });
  await page.route('**/api/adoptions*', async (route) => {
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ adoptions: [] }) });
  });

  await page.locator('[data-start-button]').click();
  await expect(page.locator('[data-start-app-id="candidate-desktop"]')).toHaveCount(0);
  await page.locator('[data-start-app-id="apps-changes"]').click();
  const store = page.locator('[data-apps-changes-app]');
  await expect(store).toBeVisible({ timeout: 10_000 });
  await expect(store.locator('[data-change-card]')).toHaveCount(4);
  await expect(store.locator('[data-change-open-vtext-report]')).toBeVisible();
  await expect(store.locator('[data-open-mission-vtext]')).toBeVisible();
  await expect(store.locator('[data-change-preview-empty]')).toBeVisible();
  await expect(store.locator('[data-candidate-desktop-input]')).toHaveCount(0);
  expect(forbiddenRemoteDisplayRequests).toEqual([]);
});

test('Apps & Changes compact catalog remains clickable beside the detail pane', async ({ desktopSession }) => {
  const { page } = desktopSession;
  await page.setViewportSize({ width: 390, height: 844 });
  await page.route('**/api/app-change-packages*', async (route) => {
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ packages: [] }) });
  });
  await page.route('**/api/adoptions*', async (route) => {
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ adoptions: [] }) });
  });

  await openStartApp(page, 'apps-changes');
  const store = page.locator('[data-apps-changes-app]');
  await expect(store).toBeVisible({ timeout: 10_000 });
  await store.locator('[data-change-card][data-change-id="motion-language"]').click();
  await expect(store.locator('[data-change-detail] h3')).toContainText('Process Animation Language');
  await store.locator('[data-change-card][data-change-id="python-code-mode"]').click();
  await expect(store.locator('[data-change-detail] h3')).toContainText('Python Code Mode');
});

test('Apps & Changes exposes rollback-only removal honestly', async ({ desktopSession }) => {
  const { page } = desktopSession;
  const adoption = {
    adoption_id: 'adoption-chiron-rollback-only',
    package_id: '28433c19-5d02-416f-9368-de56390e1927',
    app_id: 'Chiron Shelf Observability',
    target_computer_id: 'primary',
    target_candidate_id: 'candidate-chiron-rollback-only',
    status: 'adopted',
    target_active_source_ref_at_candidate_start: 'refs/computers/primary/active',
    target_active_source_ref_at_cutover: 'refs/computers/primary/active',
    candidate_source_ref: 'refs/computers/primary/candidates/candidate-chiron-rollback-only',
    foreground_tail_merge_result: 'no-conflict',
    merge_strategy: 'rebase',
    merge_conflicts_json: [],
    runtime_artifact_digest: 'sha256:recipient-runtime',
    ui_artifact_digest: 'sha256:recipient-ui',
    verifier_results_json: [],
    trace_id: 'apps-changes-chiron-shelf',
    rollback_profile_json: {
      previous_active_source_ref: 'refs/computers/primary/active',
      previous_route_profile: 'route:primary',
    },
  };
  const acceptance = {
    acceptance_id: 'runacc-chiron-removal-test',
    target_mission_id: 'mission-apps-and-changes-store-sweep-v0',
    trajectory_id: adoption.trace_id,
    state: 'accepted',
    acceptance_level: 'promotion-level',
    authority_profile: 'product-path',
    evidence_refs: [
      { ref_id: 'adoption-proof', kind: 'app_adoption', summary: 'recipient build and promote evidence' },
    ],
    rollback_refs: [
      { kind: 'source_ref', ref: 'refs/computers/primary/active', summary: 'previous active ref' },
    ],
    checkpoints: [],
    verifier_contracts: [],
    invariant_checks: [],
  };

  await page.route('**/api/app-change-packages*', async (route) => {
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ packages: [] }) });
  });
  await page.route('**/api/adoptions*', async (route) => {
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ adoptions: [adoption] }) });
  });
  let acceptanceRouteHits = 0;
  await page.route('**/api/run-acceptances**', async (route) => {
    acceptanceRouteHits += 1;
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ acceptances: [acceptance] }) });
  });
  await page.route('**/api/trace/trajectories**', async (route) => {
    const url = new URL(route.request().url());
    if (url.pathname === '/api/trace/trajectories') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          trajectories: [{
            trajectory_id: adoption.trace_id,
            title: 'Apps & Changes Chiron Shelf',
            subtitle: 'Product-path adoption evidence',
            state: 'completed',
            live: false,
            agent_count: 1,
            delegation_count: 0,
            moment_count: 1,
            message_count: 0,
            finding_count: 0,
            search_attempt_count: 0,
            latest_activity_at: '2026-05-21T02:17:21.563Z',
          }],
        }),
      });
      return;
    }
    if (url.pathname === `/api/trace/trajectories/${adoption.trace_id}`) {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          trajectory: {
            trajectory_id: adoption.trace_id,
            title: 'Apps & Changes Chiron Shelf',
            subtitle: 'Product-path adoption evidence',
            state: 'completed',
            latest_stream_seq: 1,
            agent_count: 1,
            delegation_count: 0,
            moment_count: 1,
            message_count: 0,
            finding_count: 0,
            search_attempt_count: 0,
            latest_activity_at: '2026-05-21T02:17:21.563Z',
          },
          agents: [],
          edges: [],
          moments: [],
          search: { attempts: 0, providers: [] },
          mobile_summary: {
            headline: 'promotion-level · accepted · Apps & Changes',
            acceptance_state: 'accepted',
            acceptance_level: 'promotion-level',
            readable_evidence: ['recipient build and promote evidence'],
            rollback_refs: ['refs/computers/primary/active'],
          },
          acceptances: [acceptance],
        }),
      });
      return;
    }
    if (url.pathname === `/api/trace/trajectories/${adoption.trace_id}/events`) {
      await route.fulfill({
        status: 200,
        headers: { 'Content-Type': 'text/event-stream' },
        body: '',
      });
      return;
    }
    await route.fulfill({ status: 404, contentType: 'application/json', body: JSON.stringify({ error: 'unexpected trace route' }) });
  });

  await openStartApp(page, 'apps-changes');
  const store = page.locator('[data-apps-changes-app]');
  await expect(store).toBeVisible({ timeout: 10_000 });
  const removal = store.locator('[data-change-removal-model]');
  await expect(removal).toHaveAttribute('data-removal-mode', 'Rollback-only');
  await expect(removal).toContainText('no verified inverse source patch');
  await expect(removal).toContainText('no declared feature flag');
  await expect(store.locator('[data-change-uninstall]')).toBeDisabled();
  await expect(store.locator('[data-change-disable]')).toBeDisabled();
  await expect(store.locator('[data-change-rollback]')).toBeEnabled();
  await expect(store.locator('[data-change-candidate-summary]')).toContainText('recorded');
  await expect.poll(() => acceptanceRouteHits).toBeGreaterThan(0);
  await expect(store.locator('[data-change-trace-review]')).toHaveAttribute('data-change-trace-ready', 'accepted');
  await expect(store.locator('[data-change-trace-review]')).toContainText('promotion-level');
  await store.locator('[data-change-open-trace]').click();
  const trace = page.locator('[data-trace-window]').last();
  await expect(trace.locator(`[data-trace-trajectory-id="${adoption.trace_id}"]`)).toBeVisible({ timeout: 10_000 });
  await expect(trace.locator('[data-trace-run-acceptance]')).toContainText('promotion-level', { timeout: 10_000 });
});

test('Apps & Changes aggregates portfolio reports and acceptance coverage', async ({ desktopSession }) => {
  const { page } = desktopSession;
  const acceptanceIDs = {
    'chiron-shelf': 'runacc-c3d70f753b81fd591442',
    'motion-language': 'runacc-3b54c9ae8dac2337184a',
    'liquid-material': 'runacc-d144087c5ffacad2e147',
    'python-code-mode': 'runacc-45495b8caebc3e1b82c5',
  };
  const acceptances = Object.entries(acceptanceIDs).map(([changeID, acceptanceID]) => ({
    acceptance_id: acceptanceID,
    target_mission_id: `mission-${changeID}`,
    trajectory_id: `trajectory-${changeID}`,
    state: 'accepted',
    acceptance_level: changeID === 'chiron-shelf' ? 'promotion-level' : 'export-level',
    authority_profile: 'product-path',
    evidence_refs: [
      { ref_id: `${changeID}-report`, kind: 'vtext', summary: 'owner-readable change report' },
    ],
    rollback_refs: changeID === 'chiron-shelf'
      ? [{ kind: 'source_ref', ref: 'refs/computers/primary/active', summary: 'previous active ref' }]
      : [],
    checkpoints: [],
    verifier_contracts: [],
    invariant_checks: [],
  }));

  await page.route('**/api/app-change-packages*', async (route) => {
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ packages: [] }) });
  });
  await page.route('**/api/adoptions*', async (route) => {
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ adoptions: [] }) });
  });
  await page.route('**/api/run-acceptances**', async (route) => {
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ acceptances }) });
  });
  await page.route('**/api/trace/trajectories**', async (route) => {
    const url = new URL(route.request().url());
    const target = acceptances.find((acceptance) => url.pathname.endsWith(`/${acceptance.trajectory_id}`));
    if (url.pathname === '/api/trace/trajectories') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          trajectories: acceptances.map((acceptance) => ({
            trajectory_id: acceptance.trajectory_id,
            title: acceptance.target_mission_id,
            subtitle: 'Portfolio acceptance evidence',
            state: 'completed',
            live: false,
            agent_count: 1,
            delegation_count: 0,
            moment_count: 1,
            message_count: 0,
            finding_count: 0,
            search_attempt_count: 0,
            latest_activity_at: '2026-05-21T02:58:41.000Z',
          })),
        }),
      });
      return;
    }
    if (target) {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          trajectory: {
            trajectory_id: target.trajectory_id,
            title: target.target_mission_id,
            subtitle: 'Portfolio acceptance evidence',
            state: 'completed',
            latest_stream_seq: 1,
            agent_count: 1,
            delegation_count: 0,
            moment_count: 1,
            message_count: 0,
            finding_count: 0,
            search_attempt_count: 0,
            latest_activity_at: '2026-05-21T02:58:41.000Z',
          },
          agents: [],
          edges: [],
          moments: [],
          search: { attempts: 0, providers: [] },
          mobile_summary: {
            headline: `${target.acceptance_level} · ${target.state}`,
            acceptance_state: target.state,
            acceptance_level: target.acceptance_level,
            readable_evidence: target.evidence_refs.map((ref) => ref.summary),
            rollback_refs: target.rollback_refs.map((ref) => ref.ref),
          },
          acceptances: [target],
        }),
      });
      return;
    }
    if (url.pathname.endsWith('/events')) {
      await route.fulfill({
        status: 200,
        headers: { 'Content-Type': 'text/event-stream' },
        body: '',
      });
      return;
    }
    await route.fulfill({ status: 404, contentType: 'application/json', body: JSON.stringify({ error: 'unexpected trace route' }) });
  });

  await openStartApp(page, 'apps-changes');
  const store = page.locator('[data-apps-changes-app]');
  await expect(store).toBeVisible({ timeout: 10_000 });
  const portfolio = store.locator('[data-portfolio-review]');
  await expect(portfolio).toHaveAttribute('data-portfolio-change-count', '4');
  await expect(portfolio).toHaveAttribute('data-portfolio-report-count', '4');
  await expect(portfolio).toHaveAttribute('data-portfolio-accepted-count', '4');
  await expect(portfolio.locator('[data-portfolio-change]')).toHaveCount(4);
  await expect(portfolio).toContainText('Choir Liquid Material Engine');
  await expect(portfolio).not.toContainText('28433c19-5d02-416f-9368-de56390e1927');

  const liquidRow = portfolio.locator('[data-portfolio-change][data-change-id="liquid-material"]');
  await expect(liquidRow).toHaveAttribute('data-portfolio-acceptance-state', 'accepted');
  await liquidRow.locator('[data-portfolio-open-trace]').click();
  const trace = page.locator('[data-trace-window]').last();
  await expect(trace.locator('[data-trace-trajectory-id="trajectory-liquid-material"]')).toBeVisible({
    timeout: 10_000,
  });
  await expect(trace.locator('[data-trace-run-acceptance]')).toContainText('export-level', { timeout: 10_000 });
});

test('Apps & Changes creates owner-readable VText reports', async ({ desktopSession }) => {
  const { page } = desktopSession;
  const docs = new Map();
  const revisions = new Map();
  const reportContents = new Map();
  const now = '2026-05-21T00:00:00.000Z';

  function docIdForTitle(title) {
    return `doc-${title.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '')}`;
  }

  function createMockDocument(title) {
    const doc = {
      doc_id: docIdForTitle(title),
      owner_id: 'test-owner',
      title,
      current_revision_id: '',
      created_at: now,
      updated_at: now,
      revision_count: 0,
    };
    docs.set(doc.doc_id, doc);
    return doc;
  }

  function revisionBody(revision) {
    return {
      revision_id: revision.revision_id,
      doc_id: revision.doc_id,
      owner_id: 'test-owner',
      author_kind: 'user',
      author_label: 'Apps & Changes',
      content: revision.content,
      citations: [],
      metadata: revision.metadata || {},
      parent_revision_id: revision.parent_revision_id || '',
      created_at: now,
    };
  }

  await page.route('**/api/app-change-packages*', async (route) => {
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ packages: [] }) });
  });
  await page.route('**/api/adoptions*', async (route) => {
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ adoptions: [] }) });
  });
  await page.route('**/api/vtext/**', async (route) => {
    const url = new URL(route.request().url());
    const method = route.request().method();

    if (url.pathname === '/api/vtext/documents' && method === 'GET') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ documents: Array.from(docs.values()) }),
      });
      return;
    }
    if (url.pathname === '/api/vtext/documents' && method === 'POST') {
      const body = JSON.parse(route.request().postData() || '{}');
      const doc = createMockDocument(body.title || 'Untitled VText');
      await route.fulfill({ status: 201, contentType: 'application/json', body: JSON.stringify(doc) });
      return;
    }
    const documentRevisionMatch = url.pathname.match(/^\/api\/vtext\/documents\/([^/]+)\/revisions$/);
    if (documentRevisionMatch && method === 'POST') {
      const docId = decodeURIComponent(documentRevisionMatch[1]);
      const doc = docs.get(docId);
      if (!doc) {
        await route.fulfill({ status: 404, contentType: 'application/json', body: JSON.stringify({ error: 'unknown doc' }) });
        return;
      }
      const body = JSON.parse(route.request().postData() || '{}');
      const revision = {
        revision_id: `rev-${docId}-${doc.revision_count + 1}`,
        doc_id: docId,
        content: body.content || '',
        metadata: body.metadata || {},
        parent_revision_id: body.parent_revision_id || '',
      };
      revisions.set(revision.revision_id, revision);
      reportContents.set(doc.title, revision.content);
      doc.current_revision_id = revision.revision_id;
      doc.revision_count += 1;
      doc.updated_at = now;
      await route.fulfill({
        status: 201,
        contentType: 'application/json',
        body: JSON.stringify(revisionBody(revision)),
      });
      return;
    }
    const documentMatch = url.pathname.match(/^\/api\/vtext\/documents\/([^/]+)$/);
    if (documentMatch && method === 'GET') {
      const doc = docs.get(decodeURIComponent(documentMatch[1]));
      await route.fulfill({
        status: doc ? 200 : 404,
        contentType: 'application/json',
        body: JSON.stringify(doc || { error: 'unknown doc' }),
      });
      return;
    }
    if (documentRevisionMatch && method === 'GET') {
      const docId = decodeURIComponent(documentRevisionMatch[1]);
      const docRevisions = Array.from(revisions.values()).filter((revision) => revision.doc_id === docId);
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ revisions: docRevisions.map(revisionBody) }),
      });
      return;
    }
    const revisionMatch = url.pathname.match(/^\/api\/vtext\/revisions\/([^/]+)$/);
    if (revisionMatch && method === 'GET') {
      const revision = revisions.get(decodeURIComponent(revisionMatch[1]));
      await route.fulfill({
        status: revision ? 200 : 404,
        contentType: 'application/json',
        body: JSON.stringify(revision ? revisionBody(revision) : { error: 'unknown revision' }),
      });
      return;
    }
    const manifestMatch = url.pathname.match(/^\/api\/vtext\/documents\/([^/]+)\/manifest$/);
    if (manifestMatch && method === 'POST') {
      const docId = decodeURIComponent(manifestMatch[1]);
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ doc_id: docId, source_path: 'apps-changes-report.vtext' }),
      });
      return;
    }
    const streamMatch = url.pathname.match(/^\/api\/vtext\/documents\/([^/]+)\/stream$/);
    if (streamMatch) {
      const docId = decodeURIComponent(streamMatch[1]);
      const doc = docs.get(docId);
      await route.fulfill({
        status: 200,
        headers: { 'Content-Type': 'text/event-stream' },
        body: `data: ${JSON.stringify({ kind: 'snapshot', doc_id: docId, current_revision_id: doc?.current_revision_id || '' })}\n\n`,
      });
      return;
    }
    await route.fulfill({ status: 404, contentType: 'application/json', body: JSON.stringify({ error: 'unexpected vtext route' }) });
  });

  await openStartApp(page, 'apps-changes');
  const store = page.locator('[data-apps-changes-app]');
  await expect(store).toBeVisible({ timeout: 10_000 });
  await store.locator('[data-open-mission-vtext]').click();
  await expect(store.locator('[data-apps-changes-report-status]')).toContainText('Mission VText dashboard ready', {
    timeout: 10_000,
  });

  let vtext = page.locator('[data-vtext-editor]').last();
  await expect(vtext).toBeVisible({ timeout: 10_000 });
  await expect(vtext.locator('[data-vtext-editor-area]')).toContainText('Apps & Changes Store Sweep v0', {
    timeout: 20_000,
  });
  await expect(vtext.locator('[data-vtext-editor-area]')).toContainText('Current Checkpoint', {
    timeout: 20_000,
  });
  expect(reportContents.get('Apps & Changes Store Sweep v0')).toContain('Chiron proof');

  await page.locator('[data-window-app-id="vtext"]').last().locator('[data-window-close]').click();
  await store.locator('[data-change-card][data-change-id="chiron-shelf"]').click();
  await store.locator('[data-change-open-vtext-report]').click();
  await expect(store.locator('[data-apps-changes-report-status]')).toContainText('VText report ready', {
    timeout: 10_000,
  });

  vtext = page.locator('[data-vtext-editor]').last();
  await expect(vtext).toBeVisible({ timeout: 10_000 });
  await expect(vtext.locator('[data-vtext-editor-area]')).toContainText('Chiron Shelf Observability', {
    timeout: 20_000,
  });
  await expect(vtext.locator('[data-vtext-editor-area]')).toContainText('Technical Refs', {
    timeout: 20_000,
  });
  expect(reportContents.get('Apps & Changes report: Chiron Shelf Observability')).toContain('Source acceptance');
  expect(reportContents.get('Apps & Changes report: Chiron Shelf Observability')).toContain('Package: `28433c19-5d02-416f-9368-de56390e1927`');
});

test('Apps & Changes preserves a just-created adoption when the catalog read is stale', async ({ desktopSession }) => {
  const { page } = desktopSession;
  const adoption = {
    adoption_id: 'adoption-stale-read-preserved',
    package_id: '28433c19-5d02-416f-9368-de56390e1927',
    app_id: 'Chiron Shelf Observability',
    target_computer_id: 'primary',
    target_candidate_id: 'candidate-stale-read-preserved',
    status: 'candidate_applied',
    target_active_source_ref_at_candidate_start: 'refs/computers/primary/active',
    candidate_source_ref: 'refs/computers/primary/candidates/candidate-stale-read-preserved',
    foreground_tail_merge_result: 'pending-recipient-review',
    merge_strategy: 'rebase',
    merge_conflicts_json: [],
    verifier_results_json: [],
    rollback_profile_json: {},
  };

  await page.route('**/api/app-change-packages**', async (route) => {
    const url = new URL(route.request().url());
    if (url.pathname === '/api/app-change-packages/pull' && route.request().method() === 'POST') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          package_id: adoption.package_id,
          app_id: adoption.app_id,
          visibility: 'unlisted',
          source_computer_id: 'primary',
        }),
      });
      return;
    }
    if (url.pathname === '/api/app-change-packages' && route.request().method() === 'GET') {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ packages: [] }) });
      return;
    }
    await route.fulfill({ status: 404, contentType: 'application/json', body: JSON.stringify({ error: 'unexpected package route' }) });
  });
  await page.route('**/api/adoptions**', async (route) => {
    const url = new URL(route.request().url());
    if (url.pathname === '/api/adoptions' && route.request().method() === 'GET') {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ adoptions: [] }) });
      return;
    }
    await route.fulfill({ status: 404, contentType: 'application/json', body: JSON.stringify({ error: 'unexpected adoption route' }) });
  });
  await page.route('**/api/computers/primary/source-lineage**', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        owner_id: 'test-owner',
        computer_id: 'primary',
        computer_kind: 'primary',
        active_source_ref: 'refs/computers/primary/active',
        route_profile: 'route:primary',
      }),
    });
  });
  await page.route('**/api/computers/primary/adoptions**', async (route) => {
    await route.fulfill({ status: 201, contentType: 'application/json', body: JSON.stringify(adoption) });
  });

  await openStartApp(page, 'apps-changes');
  const store = page.locator('[data-apps-changes-app]');
  await expect(store).toBeVisible({ timeout: 10_000 });
  await store.locator('[data-change-card][data-change-id="chiron-shelf"]').click();
  await store.locator('[data-change-try]').click();

  await expect(store.locator('[data-change-preview-iframe]')).toHaveAttribute(
    'src',
    /desktop_id=candidate-stale-read-preserved/,
    { timeout: 10_000 }
  );
  await expect(store.locator('[data-change-verify]')).toBeEnabled();
  await expect(store.locator('[data-review-adoption-id="adoption-stale-read-preserved"]')).toBeVisible();
});

test('Web Lens API calls preserve candidate desktop selector', async ({ desktopSession }) => {
  const { page } = desktopSession;
  const capabilityRequests = [];

  await page.route('**/api/browser/capabilities**', async (route) => {
    capabilityRequests.push(new URL(route.request().url()));
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        provider: 'obscura',
        mode: 'legacy_iframe',
        substrate: 'frontend_iframe',
        available: false,
        configured: false,
        status: 'not_configured',
        supports: {
          navigate: false,
          text: false,
          html: false,
          links: false,
          screenshot: false,
          cdp_screenshot: false,
          bounded_input: false,
          input: false,
          cdp: false,
        },
        legacy_iframe_available: true,
      }),
    });
  });

  await page.evaluate(() => {
    window.history.pushState({}, '', '/?desktop_id=branch-preview');
  });
  await page.locator('[data-desktop-icon-id="browser"]').dblclick();

  const status = page.locator('[data-browser-backend-status]');
  await expect(status).toHaveAttribute('data-browser-backend-available', 'false', { timeout: 10_000 });
  expect(capabilityRequests).toHaveLength(1);
  expect(capabilityRequests[0].searchParams.get('desktop_id')).toBe('branch-preview');
});

test('Web Lens imports Obscura semantic snapshot into VText without iframe rendering', async ({ desktopSession }) => {
  const { page } = desktopSession;
  const browserRequests = [];
  const sessionID = `web-lens-session-${Date.now()}`;

  await page.route('**/api/browser/**', async (route) => {
    const url = new URL(route.request().url());
    browserRequests.push(`${route.request().method()} ${url.pathname}${url.search}`);

    if (url.pathname === '/api/browser/capabilities') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          provider: 'obscura',
          mode: 'backend',
          substrate: 'obscura_cli_fetch',
          available: true,
          configured: true,
          status: 'ready',
          supports: {
            navigate: true,
            text: true,
            html: true,
            links: true,
            screenshot: false,
            cdp_screenshot: false,
            bounded_input: false,
            input: false,
            cdp: false,
          },
          legacy_iframe_available: true,
        }),
      });
      return;
    }

    if (url.pathname === '/api/browser/sessions') {
      await route.fulfill({
        status: 201,
        contentType: 'application/json',
        body: JSON.stringify({
          session_id: sessionID,
          owner_id: 'test-owner',
          provider: 'obscura',
          mode: 'backend',
          execution_scope: 'host_process',
          world_kind: 'foreground',
          state: 'idle',
          current_url: '',
        }),
      });
      return;
    }

    if (url.pathname === `/api/browser/sessions/${sessionID}/navigate`) {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          session_id: sessionID,
          owner_id: 'test-owner',
          provider: 'obscura',
          mode: 'backend',
          execution_scope: 'host_process',
          world_kind: 'foreground',
          state: 'ready',
          current_url: 'https://example.com',
          title: 'Example Domain',
          text_snapshot: 'Example Domain\nThis domain is for use in illustrative examples.',
          html_snapshot: '<title>Example Domain</title>',
          links: [{ url: 'https://www.iana.org/domains/example', text: 'More information' }],
        }),
      });
      return;
    }

    await route.fulfill({
      status: 404,
      contentType: 'application/json',
      body: JSON.stringify({ error: 'unexpected browser route' }),
    });
  });

  await page.locator('[data-desktop-icon-id="browser"]').dblclick();
  const webLens = page.locator('[data-browser-app]').last();
  await expect(webLens).toBeVisible({ timeout: 10_000 });
  await expect(webLens.locator('[data-browser-backend-status]')).toHaveAttribute(
    'data-browser-backend-substrate',
    'obscura_cli_fetch',
    { timeout: 10_000 }
  );

  await webLens.locator('[data-browser-url-input]').fill('https://example.com');
  await webLens.locator('[data-browser-go-btn]').click();

  await expect(webLens.locator('[data-browser-backend-snapshot]')).toContainText('Example Domain', {
    timeout: 10_000,
  });
  await expect(webLens.locator('[data-browser-iframe]')).toHaveCount(0);
  await expect(webLens.locator('[data-browser-import-vtext]')).toBeVisible();

  await webLens.locator('[data-browser-import-vtext]').click();
  const vtext = page.locator('[data-vtext-editor]').last();
  await expect(vtext).toBeVisible({ timeout: 10_000 });
  await expect(vtext.locator('[data-vtext-editor-area]')).toContainText('Web Lens import', {
    timeout: 20_000,
  });
  await expect(vtext.locator('[data-vtext-editor-area]')).toContainText('Example Domain', {
    timeout: 20_000,
  });

  expect(browserRequests.some((entry) => entry.includes('/api/browser/sessions'))).toBe(true);
});
