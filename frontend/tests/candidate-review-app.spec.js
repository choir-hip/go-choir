import { test, expect } from './helpers/fixtures.js';

const INTAKE_ID = 'intake-review-ui-72';
const ADOPTION_ID = 'adoption-review-ui-72';
const REVIEW_SURFACE_PATH = `/api/candidate-package-intakes/${INTAKE_ID}/adoption-review/${ADOPTION_ID}/promotion-switch/review-surface`;

function reviewSurfaceFixture() {
  const acceptanceID = `candidate-package-local-acceptance-${ADOPTION_ID}`;
  return {
    artifact_kind: 'candidate_package_adoption_promotion_review_surface',
    state: 'reviewable',
    surface_scope: 'product_visible_non_deployed',
    deployment_state: 'non_deployed',
    product_visible: true,
    read_only: true,
    review_scope: 'non-deployed-candidate-package-source-lineage',
    intake_id: INTAKE_ID,
    adoption_id: ADOPTION_ID,
    package_id: 'candidate-package-review-ui',
    app_id: 'texture',
    candidate_package_id: 'candidate-package-review-ui',
    candidate_package_manifest_sha256: 'sha256-review-ui',
    source_computer_id: 'source-computer-review-ui',
    source_candidate_id: 'source-candidate-review-ui',
    target_computer_id: 'target-computer-review-ui',
    target_candidate_id: 'target-candidate-review-ui',
    target_active_source_ref_at_cutover: 'texture://active/source-before-review-ui',
    candidate_source_ref: `texture://evidence/${INTAKE_ID}/switch`,
    current_adoption_status: 'source_lineage_switched',
    local_acceptance_id: acceptanceID,
    local_acceptance_level: 'local-source-lineage-evidence',
    local_acceptance_state: 'accepted',
    package_publication: 'blocked',
    deployed_promotion: 'blocked',
    deployed_route_mutation: 'blocked',
    promotion_level: 'not_claimed',
    auth_session: 'unproven',
    staging: 'unproven',
    vm_lifecycle: 'blocked',
    run_acceptance_record: 'not_created',
    app_change_package_mutation: 'not_created',
    app_adoption_mutation: 'not_created',
    allowed_actions: ['review', 'inspect', 'prepare_activation_decision'],
    activation_decision_boundary: {
      state: 'owner_decision_preparable',
      owner_controlled: true,
      requires_authenticated_owner: true,
      prepared_action: 'prepare_activation_decision',
      no_mutation: true,
      uses_local_acceptance_id: acceptanceID,
      next_boundary: 'app_adoption_promotion_requires_separate_product_activation_contract',
      blocked_routes: [
        'POST /api/adoptions/{adoption_id}/verify',
        'POST /api/adoptions/{adoption_id}/approve',
        'POST /api/adoptions/{adoption_id}/promote',
        'POST /api/candidate-package-intakes',
        'POST /api/candidate-package-intakes/{intake_id}/review',
        'POST /api/candidate-package-intakes/{intake_id}/adoption-boundary',
        'POST /api/candidate-package-intakes/{intake_id}/publication-draft',
        'POST /api/candidate-package-intakes/{intake_id}/adoption-review',
        'POST /api/candidate-package-intakes/{intake_id}/adoption-review/{adoption_id}',
        'POST /api/candidate-package-intakes/{intake_id}/adoption-review/{adoption_id}/promotion-switch',
        'POST /api/candidate-package-intakes/{intake_id}/adoption-review/{adoption_id}/promotion-switch/rollback',
        'POST /api/candidate-package-intakes/{intake_id}/adoption-review/{adoption_id}/promotion-switch/roll-forward',
        'POST /api/run-acceptances/synthesize',
        'DELETE /auth/sessions/{session_id}',
        'POST /auth/logout',
        'POST /api/staging/claims',
        'POST /api/vm/lifecycle',
      ],
      required_contracts: [
        'authenticated owner decision contract',
        'package publication contract',
        'AppAdoption mutation contract',
        'deployed route mutation contract',
        'staging identity contract',
        'VM lifecycle contract',
        'run-acceptance contract',
      ],
    },
    blocked_actions: [
      'publish_package',
      'deploy_route',
      'promote_product',
      'create_run_acceptance_record',
      'mutate_auth_session',
      'mutate_vm_lifecycle',
      'claim_staging_acceptance',
    ],
    acceptance_evidence: {
      artifact_kind: 'candidate_package_promotion_switch_acceptance_evidence',
      acceptance_id: acceptanceID,
      acceptance_level: 'local-source-lineage-evidence',
      state: 'accepted',
      evidence_refs: [
        `texture://evidence/${INTAKE_ID}/switch`,
        `texture://evidence/${INTAKE_ID}/rollback`,
        `texture://evidence/${INTAKE_ID}/roll-forward`,
      ],
      residual_risks: [
        'local-source-lineage-evidence is not deployed promotion-level acceptance',
        'deployed route registration, auth/session, staging identity, package publication, and VM lifecycle semantics remain unproven',
      ],
    },
    boundary_assertions: {
      package_publication: 'blocked',
      deployed_promotion: 'blocked',
      deployed_route_mutation: 'blocked',
      promotion_level: 'not_claimed',
      run_acceptance_record: 'not_created',
      auth_session: 'unproven',
      staging: 'unproven',
      vm_lifecycle: 'blocked',
      app_change_package_write: 'not_created',
      app_adoption_write: 'not_created',
    },
    residual_risks: [
      'product-visible review surface is non-deployed and local-route-harness scoped',
      'reviewability does not authorize package publication, deployed promotion, route mutation, run acceptance, auth/session, staging, or VM lifecycle claims',
    ],
  };
}

async function mockSession(page, authenticated) {
  await page.route('**/auth/session', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(authenticated ? {
        authenticated: true,
        user: {
          id: 'candidate-review-user',
          email: 'candidate-review-ui@example.com',
          created_at: '2026-07-04T00:00:00Z',
        },
      } : { authenticated: false }),
    });
  });
}

async function mockAuthenticatedShell(page, requests = []) {
  await page.route('**/api/shell/bootstrap**', async (route) => {
    const url = new URL(route.request().url());
    requests.push(`${route.request().method()} ${url.pathname}`);
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        sandbox_id: 'candidate-review-sandbox',
        computer_id: 'candidate-review-computer',
        owner_id: 'candidate-review-user',
      }),
    });
  });

  await page.route('**/api/desktop/state**', async (route) => {
    const url = new URL(route.request().url());
    requests.push(`${route.request().method()} ${url.pathname}`);
    if (route.request().method() === 'GET') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          owner_id: 'candidate-review-user',
          windows: [],
          active_window_id: '',
          icon_positions: {},
          updated_at: '2026-07-04T00:00:00Z',
        }),
      });
      return;
    }
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ ok: true, updated_at: '2026-07-04T00:00:00Z' }),
    });
  });
}

async function captureCandidatePackageRequests(page) {
  const requests = [];
  await page.route('**/api/candidate-package-intakes/**', async (route) => {
    const url = new URL(route.request().url());
    requests.push(`${route.request().method()} ${url.pathname}`);
    if (url.pathname === REVIEW_SURFACE_PATH && route.request().method() === 'GET') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(reviewSurfaceFixture()),
      });
      return;
    }
    await route.fulfill({
      status: 404,
      contentType: 'application/json',
      body: JSON.stringify({ error: 'unexpected candidate package API request' }),
    });
  });
  return requests;
}

async function captureActivationBoundaryRequests(page) {
  const requests = [];
  page.on('request', (request) => {
    const url = new URL(request.url());
    const method = request.method();
    const path = url.pathname;
    if (
      path.startsWith('/api/adoptions') ||
      path.startsWith('/api/run-acceptances') ||
      path.startsWith('/api/staging') ||
      path.startsWith('/api/vm') ||
      (path.startsWith('/auth/') && method !== 'GET') ||
      (path.startsWith('/api/candidate-package-intakes/') && method !== 'GET')
    ) {
      requests.push(`${method} ${path}`);
    }
  });
  for (const pattern of ['**/api/adoptions**', '**/api/run-acceptances**', '**/api/staging**', '**/api/vm**']) {
    await page.route(pattern, async (route) => {
      const url = new URL(route.request().url());
      requests.push(`${route.request().method()} ${url.pathname}`);
      await route.fulfill({
        status: 500,
        contentType: 'application/json',
        body: JSON.stringify({ error: 'activation boundary must not call protected route' }),
      });
    });
  }
  return requests;
}


async function openCandidateReviewFromDesk(page) {
  await page.locator('[data-desk-menu-button]').click();
  await page.locator('[data-desk-app-id="candidate-review"]').click();
  const app = page.locator('[data-candidate-review-app]').last();
  await expect(app).toBeVisible({ timeout: 10_000 });
  return app;
}

test('candidate-review URL workflow fetches the review surface and renders non-deployed local evidence', async ({ page }) => {
  await mockSession(page, true);
  await mockAuthenticatedShell(page);
  const candidateRequests = await captureCandidatePackageRequests(page);
  const activationRequests = await captureActivationBoundaryRequests(page);

  await page.goto(`/?app=candidate-review&intake=${INTAKE_ID}&adoption=${ADOPTION_ID}`);

  await expect(page.locator('[data-window][data-window-app-id="candidate-review"]')).toBeVisible({ timeout: 30_000 });
  const app = page.locator('[data-candidate-review-app]').last();
  await expect(app).toBeVisible();
  await expect(app.locator('[data-candidate-review-surface]')).toBeVisible({ timeout: 10_000 });

  expect(candidateRequests).toEqual([`GET ${REVIEW_SURFACE_PATH}`]);

  await expect(app.locator('[data-candidate-review-mode]')).toContainText('Read-only');
  await expect(app.locator('[data-candidate-review-mode]')).toContainText('Product-visible, non-deployed');
  await expect(app.locator('[data-candidate-review-deployment]')).toHaveText('non_deployed');
  await expect(app.locator('[data-candidate-review-surface]')).toContainText('candidate_package_adoption_promotion_review_surface');
  await expect(app.locator('[data-candidate-review-surface]')).toContainText('non-deployed-candidate-package-source-lineage');

  await expect(app.locator('.provenance-grid')).toContainText('candidate-package-review-ui');
  await expect(app.locator('.provenance-grid')).toContainText('texture');
  await expect(app.locator('.provenance-grid')).toContainText('target-computer-review-ui');
  await expect(app.locator('.provenance-grid')).toContainText(`texture://evidence/${INTAKE_ID}/switch`);
  await expect(app.locator('.provenance-grid')).toContainText(`candidate-package-local-acceptance-${ADOPTION_ID}`);
  await expect(app.locator('.provenance-grid')).toContainText('local-source-lineage-evidence');

  const actions = app.locator('[data-candidate-review-actions]');
  await expect(actions.locator('li')).toHaveText(['review', 'inspect', 'prepare_activation_decision']);
  await expect(actions).not.toContainText('publish_package');
  await expect(actions).not.toContainText('deploy_route');
  await expect(actions).not.toContainText('promote_product');
  await expect(actions).not.toContainText('create_run_acceptance_record');

  const activationBoundary = app.locator('[data-candidate-review-activation-boundary]');
  await expect(activationBoundary).toContainText('Owner activation decision');
  await expect(activationBoundary).toContainText('does not activate, publish, promote, create run acceptance, or call AppAdoption mutation routes');
  await expect(app.locator('[data-candidate-review-activation-summary]')).toHaveCount(0);
  await app.locator('[data-candidate-review-prepare-activation]').click();
  const activationSummary = app.locator('[data-candidate-review-activation-summary]');
  await expect(activationSummary).toBeVisible();
  await expect(activationSummary).toContainText('owner_decision_preparable');
  await expect(activationSummary).toContainText(`candidate-package-local-acceptance-${ADOPTION_ID}`);
  await expect(activationSummary).toContainText('app_adoption_promotion_requires_separate_product_activation_contract');
  await expect(activationSummary).toContainText('No mutation was performed.');
  await expect(activationSummary).toContainText('authenticated owner decision contract');
  await expect(activationSummary).toContainText('package publication contract');
  await expect(activationSummary).toContainText('AppAdoption mutation contract');
  await expect(activationSummary).toContainText('staging identity contract');
  await expect(activationSummary).toContainText('run-acceptance contract');
  await expect(activationSummary).toContainText('POST /api/adoptions/{adoption_id}/verify');
  await expect(activationSummary).toContainText('POST /api/adoptions/{adoption_id}/approve');
  await expect(activationSummary).toContainText('POST /api/adoptions/{adoption_id}/promote');
  await expect(activationSummary).toContainText('POST /api/candidate-package-intakes/{intake_id}/adoption-review/{adoption_id}/promotion-switch');
  await expect(activationSummary).toContainText('POST /api/run-acceptances/synthesize');
  await expect(activationSummary).toContainText('DELETE /auth/sessions/{session_id}');
  expect(activationRequests).toEqual([]);

  const boundaries = app.locator('[data-candidate-review-boundaries]');
  await expect(boundaries).toContainText('Package publication');
  await expect(boundaries).toContainText('blocked');
  await expect(boundaries).toContainText('Deployed promotion');
  await expect(boundaries).toContainText('Deployed route mutation');
  await expect(boundaries).toContainText('Promotion-level acceptance');
  await expect(boundaries).toContainText('not_claimed');
  await expect(boundaries).toContainText('Run acceptance record');
  await expect(boundaries).toContainText('not_created');
  await expect(boundaries).toContainText('Auth/session');
  await expect(boundaries).toContainText('unproven');
  await expect(boundaries).toContainText('Staging');
  await expect(boundaries).toContainText('VM lifecycle');
  await expect(boundaries).toContainText('AppChangePackage mutation');
  await expect(boundaries).toContainText('AppAdoption mutation');

  const evidence = app.locator('[data-candidate-review-evidence]');
  await expect(evidence).toContainText('Accepted local evidence');
  await expect(evidence).toContainText(`candidate-package-local-acceptance-${ADOPTION_ID}`);
  await expect(evidence).toContainText('local-source-lineage-evidence is not deployed promotion-level acceptance');
  await expect(evidence).toContainText('deployed route registration, auth/session, staging identity, package publication, and VM lifecycle semantics remain unproven');
});

test('candidate-review missing IDs keeps the input state empty and does not call the review API', async ({ page }) => {
  await mockSession(page, true);
  await mockAuthenticatedShell(page);
  const candidateRequests = await captureCandidatePackageRequests(page);

  await page.goto('/?app=candidate-review');

  await expect(page.locator('[data-window][data-window-app-id="candidate-review"]')).toBeVisible({ timeout: 30_000 });
  const app = page.locator('[data-candidate-review-app]').last();
  await expect(app).toBeVisible();
  await expect(app.locator('[data-candidate-review-intake]')).toHaveValue('');
  await expect(app.locator('[data-candidate-review-adoption]')).toHaveValue('');
  await expect(app.locator('[data-candidate-review-load]')).toBeDisabled();
  await expect(app.locator('[data-candidate-review-empty]')).toContainText('Enter an intake/adoption pair');
  await expect(app.locator('[data-candidate-review-surface]')).toHaveCount(0);
  await expect(app.locator('[data-candidate-review-error]')).toHaveCount(0);
  expect(candidateRequests).toEqual([]);
});

test('candidate-review signed-out load requests auth without protected API or state mutation', async ({ page }) => {
  await mockSession(page, false);
  const protectedRequests = [];
  await page.route('**/api/shell/bootstrap**', async (route) => {
    const url = new URL(route.request().url());
    protectedRequests.push(`${route.request().method()} ${url.pathname}`);
    await route.fulfill({ status: 500, contentType: 'application/json', body: JSON.stringify({ error: 'signed-out shell bootstrap should not run' }) });
  });
  await page.route('**/api/desktop/state**', async (route) => {
    const url = new URL(route.request().url());
    protectedRequests.push(`${route.request().method()} ${url.pathname}`);
    await route.fulfill({ status: 500, contentType: 'application/json', body: JSON.stringify({ error: 'signed-out desktop state should not run' }) });
  });
  const candidateRequests = await captureCandidatePackageRequests(page);

  await page.goto('/');
  await expect(page.locator('[data-desktop][data-authenticated="false"]')).toBeVisible({ timeout: 10_000 });

  const app = await openCandidateReviewFromDesk(page);
  await app.locator('[data-candidate-review-intake]').fill(INTAKE_ID);
  await app.locator('[data-candidate-review-adoption]').fill(ADOPTION_ID);
  await expect(app.locator('[data-candidate-review-load]')).toBeEnabled();
  await app.locator('[data-candidate-review-load]').click();

  await expect(page.locator('[data-auth-overlay][data-auth-intent-kind="candidate_review"]')).toBeVisible();
  await expect(page.locator('[data-auth-entry]')).toBeVisible();
  await expect(page.locator('[data-auth-entry]')).toContainText('Continue with private computer state');
  await expect(app.locator('[data-candidate-review-surface]')).toHaveCount(0);
  await expect(app.locator('[data-candidate-review-error]')).toHaveCount(0);
  expect(candidateRequests).toEqual([]);
  expect(protectedRequests).toEqual([]);
});
