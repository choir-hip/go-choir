import { test, expect } from './helpers/fixtures.js';

async function openStartApp(page, appId) {
  const existing = page.locator(`[data-window-app-id="${appId}"]`);
  for (let count = await existing.count(); count > 0; count -= 1) {
    await existing.last().locator('[data-window-close]').click();
  }
  await page.locator('[data-start-button]').click();
  await page.locator(`[data-start-app-id="${appId}"]`).click();
}

function packageRecord(overrides = {}) {
  const packageId = overrides.package_id || `pkg-${Math.random().toString(36).slice(2)}`;
  const title = overrides.title || 'Chiron Shelf Observability';
  return {
    package_id: packageId,
    owner_id: overrides.owner_id || 'source-owner',
    app_id: overrides.app_id || title,
    status: overrides.status || 'published_unlisted',
    visibility: overrides.visibility || 'unlisted',
    source_computer_id: overrides.source_computer_id || 'primary',
    source_candidate_id: overrides.source_candidate_id || `candidate-${packageId}`,
    source_active_ref: 'refs/computers/primary/active',
    candidate_source_ref: `refs/computers/primary/candidates/candidate-${packageId}`,
    runtime_source_delta_sha256: 'runtime-delta-sha',
    ui_source_delta_sha256: 'ui-delta-sha',
    package_manifest_sha256: `manifest-${packageId}`,
    app_protocol_contract: 'recipient rebuild required',
    manifest_json: {
      title,
      family: overrides.family || 'Shell',
      summary: overrides.summary || 'Streams live agent progress through the Shelf while controls keep working.',
    },
    provenance_refs_json: overrides.provenance_refs_json ?? {
      human_summary: 'Owner-readable narrative describing what changed and what was verified.',
      recommendation: 'Try in a candidate before install.',
      texture_doc_id: 'doc-chiron',
      texture_revision_id: 'rev-chiron',
      screenshot_refs: ['test-results/chiron-shelf.png'],
      video_refs: ['test-results/chiron-shelf.webm'],
      artifact_refs: ['runacc-chiron'],
    },
    trace_id: overrides.trace_id || 'trace-chiron',
    created_at: '2026-05-22T12:00:00Z',
    updated_at: '2026-05-22T12:00:00Z',
    ...overrides,
  };
}

function humanProofBody(pkg) {
  const refs = pkg?.provenance_refs_json || {};
  const hasNarrative = Boolean(refs.human_summary || refs.texture_doc_id || refs.texture_revision_id);
  const hasMedia = Boolean(refs.screenshot_refs?.length || refs.video_refs?.length || refs.benchmark_refs?.length);
  return {
    state: hasNarrative && hasMedia ? 'human_reviewable' : refs.artifact_refs?.length ? 'machine_receipt_only' : 'evidence_pending',
    summary: refs.human_summary || '',
    recommendation: refs.recommendation || '',
    narrative_refs: refs.texture_doc_id ? [refs.texture_doc_id] : [],
    screenshot_refs: refs.screenshot_refs || [],
    video_refs: refs.video_refs || [],
    benchmark_refs: refs.benchmark_refs || [],
    artifact_refs: refs.artifact_refs || [],
    missing: [
      ...(hasNarrative ? [] : ['causal Texture narrative']),
      ...(hasMedia ? [] : ['screenshot, video, or benchmark evidence']),
    ],
  };
}

async function routeAppsChanges(page, { packages = [], adoptions = [], acceptances = [], packageEvidence = {} } = {}) {
  await page.route(/\/api\/app-change-packages(?:\/.*)?(?:\?.*)?$/, async (route) => {
    const url = new URL(route.request().url());
    const method = route.request().method();
    const reviewMatch = url.pathname.match(/^\/api\/app-change-packages\/([^/]+)\/review-evidence$/);
    if (reviewMatch) {
      const packageId = decodeURIComponent(reviewMatch[1]);
      const pkg = packages.find((item) => item.package_id === packageId);
      const body = packageEvidence[packageId] || {
        package_id: packageId,
        human_proof: pkg ? humanProofBody(pkg) : { state: 'evidence_pending', missing: ['human proof'] },
        acceptances: acceptances.filter((acceptance) => JSON.stringify(acceptance).includes(packageId)),
      };
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(body) });
      return;
    }
    if (url.pathname === '/api/app-change-packages' && method === 'GET') {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ packages }) });
      return;
    }
    if (url.pathname === '/api/app-change-packages/pull' && method === 'POST') {
      const body = JSON.parse(route.request().postData() || '{}');
      const pkg = packages.find((item) => item.package_id === body.package_id) || packages[0] || {};
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(pkg) });
      return;
    }
    await route.fulfill({ status: 404, contentType: 'application/json', body: JSON.stringify({ error: 'unexpected package route' }) });
  });

  await page.route(/\/api\/adoptions(?:\/.*)?(?:\?.*)?$/, async (route) => {
    const url = new URL(route.request().url());
    const method = route.request().method();
    if (url.pathname === '/api/adoptions' && method === 'GET') {
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ adoptions }) });
      return;
    }
    const actionMatch = url.pathname.match(/^\/api\/adoptions\/([^/]+)\/(verify|promote|rollback)$/);
    if (actionMatch) {
      const adoption = adoptions.find((item) => item.adoption_id === decodeURIComponent(actionMatch[1]));
      if (!adoption) {
        await route.fulfill({ status: 404, contentType: 'application/json', body: JSON.stringify({ error: 'unknown adoption' }) });
        return;
      }
      const action = actionMatch[2];
      const next = {
        ...adoption,
        status: action === 'verify' ? 'verified' : action === 'promote' ? 'adopted' : 'rolled_back',
        runtime_artifact_digest: adoption.runtime_artifact_digest || 'sha256:recipient-runtime',
        ui_artifact_digest: adoption.ui_artifact_digest || 'sha256:recipient-ui',
        rollback_profile_json: adoption.rollback_profile_json || {
          previous_active_source_ref: 'refs/computers/primary/active',
          previous_route_profile: 'route:primary',
        },
      };
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(next) });
      return;
    }
    await route.fulfill({ status: 404, contentType: 'application/json', body: JSON.stringify({ error: 'unexpected adoption route' }) });
  });

  await page.route(/\/api\/run-acceptances(?:\/.*)?(?:\?.*)?$/, async (route) => {
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ acceptances }) });
  });
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
  const humanPackage = packageRecord({ package_id: 'pkg-human-proof' });
  const pendingPackage = packageRecord({
    package_id: 'pkg-machine-only',
    title: 'Machine Receipt Only',
    provenance_refs_json: { artifact_refs: ['runacc-machine-only'] },
  });

  await routeAppsChanges(page, { packages: [humanPackage, pendingPackage] });

  await openStartApp(page, 'apps-changes');
  await expect(page.locator('[data-start-app-id="candidate-desktop"]')).toHaveCount(0);
  const store = page.locator('[data-apps-changes-app]').last();
  await expect(store).toBeVisible({ timeout: 10_000 });
  await expect(store.locator('[data-change-card]')).toHaveCount(2);
  await expect(store.locator('[data-change-open-texture-report]')).toBeVisible();
  await expect(store.locator('[data-change-preview-empty]')).toBeVisible();
  await expect(store.locator('[data-candidate-desktop-input]')).toHaveCount(0);
  await expect(store.locator('[data-change-card][data-change-id="pkg-human-proof"]')).toHaveAttribute('data-human-proof-state', 'human_reviewable');
  await expect(store.locator('[data-change-card][data-change-id="pkg-machine-only"]')).toHaveAttribute('data-human-proof-state', 'machine_receipt_only');
  expect(forbiddenRemoteDisplayRequests).toEqual([]);
});

test('Apps & Changes compact catalog remains clickable beside the detail pane', async ({ desktopSession }) => {
  const { page } = desktopSession;
  const motionPackage = packageRecord({ package_id: 'pkg-motion', title: 'Process Animation Language', family: 'Motion' });
  const pythonPackage = packageRecord({ package_id: 'pkg-python', title: 'Python Code Mode', family: 'Code Execution' });
  await page.setViewportSize({ width: 390, height: 844 });
  await routeAppsChanges(page, { packages: [motionPackage, pythonPackage] });

  await openStartApp(page, 'apps-changes');
  const store = page.locator('[data-apps-changes-app]').last();
  await expect(store).toBeVisible({ timeout: 10_000 });
  await store.locator('[data-change-card][data-change-id="pkg-motion"]').click();
  await expect(store.locator('[data-change-detail] h3')).toContainText('Process Animation Language');
  await store.locator('[data-change-card][data-change-id="pkg-python"]').click();
  await expect(store.locator('[data-change-detail] h3')).toContainText('Python Code Mode');
});

test('Apps & Changes exposes rollback-only removal honestly', async ({ desktopSession }) => {
  const { page } = desktopSession;
  const pkg = packageRecord({ package_id: 'pkg-chiron-rollback-only' });
  const adoption = {
    adoption_id: 'adoption-chiron-rollback-only',
    package_id: pkg.package_id,
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
    package_id: pkg.package_id,
    supports_human_review: true,
    evidence_refs: [
      { ref_id: 'narrative-proof', kind: 'texture', summary: 'owner-readable narrative' },
      { ref_id: 'screenshot-proof', kind: 'screenshot', summary: 'candidate behavior screenshot.png' },
    ],
    rollback_refs: [
      { kind: 'source_ref', ref: 'refs/computers/primary/active', summary: 'previous active ref' },
    ],
    checkpoints: [],
    verifier_contracts: [],
    invariant_checks: [],
  };

  await routeAppsChanges(page, { packages: [pkg], adoptions: [adoption], acceptances: [acceptance] });
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
            headline: 'promotion-level / accepted / Apps & Changes',
            acceptance_state: 'accepted',
            acceptance_level: 'promotion-level',
            readable_evidence: ['owner-readable narrative', 'candidate behavior screenshot'],
            rollback_refs: ['refs/computers/primary/active'],
          },
          acceptances: [acceptance],
        }),
      });
      return;
    }
    if (url.pathname === `/api/trace/trajectories/${adoption.trace_id}/events`) {
      await route.fulfill({ status: 200, headers: { 'Content-Type': 'text/event-stream' }, body: '' });
      return;
    }
    await route.fulfill({ status: 404, contentType: 'application/json', body: JSON.stringify({ error: 'unexpected trace route' }) });
  });

  await openStartApp(page, 'apps-changes');
  const store = page.locator('[data-apps-changes-app]').last();
  await expect(store).toBeVisible({ timeout: 10_000 });
  const removal = store.locator('[data-change-removal-model]');
  await expect(removal).toHaveAttribute('data-removal-mode', 'Rollback-only');
  await expect(removal).toContainText('no verified inverse source patch');
  await expect(removal).toContainText('no declared feature flag');
  await expect(store.locator('[data-change-uninstall]')).toBeDisabled();
  await expect(store.locator('[data-change-disable]')).toBeDisabled();
  await expect(store.locator('[data-change-rollback]')).toBeEnabled();
  await expect(store.locator('[data-change-candidate-summary]')).toContainText('recorded');
  await expect(store.locator('[data-change-trace-review]')).toHaveAttribute('data-change-trace-ready', 'trace-only');
  await expect(store.locator('[data-change-open-trace]')).toBeEnabled();
});

test('Apps & Changes does not invent a static experiment portfolio', async ({ desktopSession }) => {
  const { page } = desktopSession;
  await routeAppsChanges(page, { packages: [] });

  await openStartApp(page, 'apps-changes');
  const store = page.locator('[data-apps-changes-app]').last();
  await expect(store).toBeVisible({ timeout: 10_000 });
  await expect(store.locator('[data-portfolio-review]')).toHaveCount(0);
  await expect(store.locator('[data-change-card]')).toHaveCount(0);
  await expect(store.locator('[data-apps-changes-empty]')).toContainText('No reviewable changes');
  await expect(store).not.toContainText('Choir Liquid Material Engine');
});

test('Apps & Changes marks package-scoped machine receipts as insufficient for human review', async ({ desktopSession }) => {
  const { page } = desktopSession;
  const pkg = packageRecord({
    package_id: 'pkg-machine-receipt',
    title: 'Liquid Material Receipt',
    provenance_refs_json: { artifact_refs: ['runacc-machine-receipt'] },
  });
  const acceptance = {
    acceptance_id: 'runacc-machine-receipt',
    target_mission_id: 'mission-machine-receipt',
    trajectory_id: 'trajectory-machine-receipt',
    state: 'accepted',
    acceptance_level: 'export-level',
    authority_profile: 'product-path',
    package_id: pkg.package_id,
    review_scope: 'package-referenced',
    trace_visible: false,
    evidence_ref_count: 3,
    rollback_ref_count: 0,
    human_proof_state: 'machine_receipt_only',
    supports_human_review: false,
    machine_receipt_only: true,
    checkpoint_kinds: ['app_change_package_published'],
  };
  await routeAppsChanges(page, {
    packages: [pkg],
    acceptances: [acceptance],
    packageEvidence: {
      [pkg.package_id]: {
        package_id: pkg.package_id,
        human_proof: {
          state: 'machine_receipt_only',
          artifact_refs: ['runacc-machine-receipt'],
          missing: ['causal Texture narrative', 'screenshot, video, or benchmark evidence'],
        },
        acceptances: [acceptance],
      },
    },
  });

  await openStartApp(page, 'apps-changes');
  const store = page.locator('[data-apps-changes-app]').last();
  await expect(store.locator('[data-change-card][data-change-id="pkg-machine-receipt"]')).toHaveAttribute('data-human-proof-state', 'machine_receipt_only');
  await expect(store.locator('[data-human-proof-panel]')).toHaveAttribute('data-human-proof-state', 'machine_receipt_only');
  await expect(store.locator('[data-human-proof-missing]')).toContainText('causal Texture narrative');
  await expect(store.locator('[data-change-try]')).toBeDisabled();
  await expect(store.locator('[data-change-acceptance-summary]')).toContainText('Try this change before Trace');
});

test('Apps & Changes opens existing Texture narratives instead of generating claim reports', async ({ desktopSession }) => {
  const { page } = desktopSession;
  const pkg = packageRecord({ package_id: 'pkg-texture-open' });
  const now = '2026-05-21T00:00:00.000Z';
  let createdRevision = false;
  await routeAppsChanges(page, { packages: [pkg] });
  await page.route('**/api/texture/**', async (route) => {
    const url = new URL(route.request().url());
    const method = route.request().method();
    if (method === 'POST' && (url.pathname === '/api/texture/documents' || url.pathname.includes('/revisions'))) {
      createdRevision = true;
    }
    if (url.pathname === '/api/texture/documents/doc-chiron') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          doc_id: 'doc-chiron',
          owner_id: 'test-owner',
          title: 'Chiron Shelf narrative',
          current_revision_id: 'rev-chiron',
          created_at: now,
          updated_at: now,
          revision_count: 1,
        }),
      });
      return;
    }
    if (url.pathname === '/api/texture/revisions/rev-chiron') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          revision_id: 'rev-chiron',
          doc_id: 'doc-chiron',
          owner_id: 'test-owner',
          author_kind: 'agent',
          author_label: 'Choir experiment worker',
          content: '# Chiron Shelf Observability\n\nReal narrative, not generated from static seed data.',
          citations: [],
          metadata: {},
          parent_revision_id: '',
          created_at: now,
        }),
      });
      return;
    }
    if (url.pathname === '/api/texture/documents/doc-chiron/revisions') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          revisions: [{
            revision_id: 'rev-chiron',
            doc_id: 'doc-chiron',
            author_kind: 'agent',
            author_label: 'Choir experiment worker',
            content_excerpt: 'Real narrative',
            created_at: now,
          }],
        }),
      });
      return;
    }
    if (url.pathname === '/api/texture/documents/doc-chiron/stream') {
      await route.fulfill({
        status: 200,
        headers: { 'Content-Type': 'text/event-stream' },
        body: `data: ${JSON.stringify({ kind: 'snapshot', doc_id: 'doc-chiron', current_revision_id: 'rev-chiron' })}\n\n`,
      });
      return;
    }
    if (url.pathname === '/api/texture/documents/doc-chiron/manifest' && method === 'POST') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ doc_id: 'doc-chiron', source_path: 'chiron-report.texture' }),
      });
      return;
    }
    await route.fulfill({ status: 404, contentType: 'application/json', body: JSON.stringify({ error: 'unexpected texture route' }) });
  });

  await openStartApp(page, 'apps-changes');
  const store = page.locator('[data-apps-changes-app]').last();
  await expect(store).toBeVisible({ timeout: 10_000 });
  await store.locator('[data-change-open-texture-report]').click();

  const texture = page.locator('[data-texture-editor]').last();
  await expect(texture).toBeVisible({ timeout: 10_000 });
  await expect(texture.locator('[data-texture-editor-area]')).toContainText('Real narrative', { timeout: 20_000 });
  expect(createdRevision).toBe(false);
});

test('Apps & Changes preserves a just-created adoption but waits for healthy preview before verification', async ({ desktopSession }) => {
  const { page } = desktopSession;
  const pkg = packageRecord({ package_id: 'pkg-stale-read-preserved' });
  const adoption = {
    adoption_id: 'adoption-stale-read-preserved',
    package_id: pkg.package_id,
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

  await routeAppsChanges(page, { packages: [pkg], adoptions: [] });
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
  const store = page.locator('[data-apps-changes-app]').last();
  await expect(store).toBeVisible({ timeout: 10_000 });
  await store.locator('[data-change-card][data-change-id="pkg-stale-read-preserved"]').click();
  await store.locator('[data-change-try]').click();

  await expect(store.locator('[data-change-preview-iframe]')).toHaveAttribute(
    'src',
    /desktop_id=candidate-stale-read-preserved/,
    { timeout: 10_000 }
  );
  await expect(store.locator('[data-change-verify]')).toBeDisabled();
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

test('Web Lens imports Obscura semantic snapshot into Texture without iframe rendering', async ({ desktopSession }) => {
  const { page } = desktopSession;
  const browserRequests = [];
  const contentCreates = [];
  const sessionID = `web-lens-session-${Date.now()}`;
  const contentID = `web-lens-content-${Date.now()}`;

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
  await page.route('**/api/content/items', async (route) => {
    const payload = JSON.parse(route.request().postData() || '{}');
    contentCreates.push(payload);
    await route.fulfill({
      status: 201,
      contentType: 'application/json',
      body: JSON.stringify({
        content_id: contentID,
        owner_id: 'test-owner',
        source_type: payload.source_type,
        media_type: payload.media_type,
        app_hint: payload.app_hint,
        title: payload.title,
        source_url: payload.source_url,
        canonical_url: payload.canonical_url,
        text_content: payload.text_content,
        metadata: payload.metadata,
        provenance: payload.provenance,
        content_hash: 'sha256:web-lens-fixture',
      }),
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
  await expect(webLens.locator('[data-browser-import-texture]')).toBeVisible();

  await webLens.locator('[data-browser-import-texture]').click();
  const texture = page.locator('[data-texture-editor]').last();
  await expect(texture).toBeVisible({ timeout: 10_000 });
  await expect(texture.locator('[data-texture-editor-area]')).toContainText('Web Lens import', {
    timeout: 20_000,
  });
  await expect(texture.locator('[data-texture-editor-area]')).toContainText('Example Domain', {
    timeout: 20_000,
  });
  await expect(texture.locator('[data-texture-editor-area]')).toContainText(`Content item: ${contentID}`, {
    timeout: 20_000,
  });

  expect(browserRequests.some((entry) => entry.includes('/api/browser/sessions'))).toBe(true);
  expect(contentCreates).toHaveLength(1);
  expect(contentCreates[0].source_type).toBe('text');
  expect(contentCreates[0].media_type).toBe('text/markdown');
  expect(contentCreates[0].app_hint).toBe('content');
  expect(contentCreates[0].text_content).toContain('This domain is for use in illustrative examples.');
  expect(contentCreates[0].metadata.reader_artifact_kind).toBe('web_lens_reader_markdown');
  expect(contentCreates[0].metadata.browser_session_id).toBe(sessionID);
  expect(contentCreates[0].provenance.publish_source_snapshot).toBe(true);
  expect(contentCreates[0].provenance.browser_session_id).toBe(sessionID);
});
