import { test, expect } from './helpers/fixtures.js';
import { getSession, registerPasskey } from './helpers/auth.js';

const BASE_URL = process.env.GO_CHOIR_SECTION5_BASE_URL ||
  process.env.PLAYWRIGHT_BASE_URL ||
  'http://localhost:4173';

test.use({ trace: 'on', video: 'on', screenshot: 'on' });

function uniqueEmail(label) {
  return `section5-${label}-${Date.now()}-${Math.random().toString(36).slice(2, 8)}@example.com`;
}

async function registerAndLoadDesktop(page, email) {
  await page.goto(BASE_URL);
  await registerPasskey(page, email, BASE_URL);
  await page.reload();
  await page.locator('[data-desktop]').waitFor({ state: 'visible', timeout: 15000 });
}

async function openApp(page, appId) {
  await page.locator(`[data-desktop-icon-id="${appId}"]`).dblclick();
}

async function openStartApp(page, appId) {
  await page.locator('[data-start-button]').click();
  await page.locator(`[data-start-app-id="${appId}"]`).click();
}

async function attachScreenshot(page, testInfo, name) {
  const path = testInfo.outputPath(`${name}.png`);
  await page.screenshot({ path, fullPage: false });
  await testInfo.attach(name, { path, contentType: 'image/png' });
}

async function fetchJSON(page, path) {
  return page.evaluate(async (requestPath) => {
    const res = await fetch(requestPath, { credentials: 'include' });
    const body = await res.text();
    if (!res.ok) {
      throw new Error(`${requestPath} failed: ${res.status} ${body}`);
    }
    return body ? JSON.parse(body) : null;
  }, path);
}

async function waitForPromptDecision(page, submissionId, timeout = 30_000) {
  const deadline = Date.now() + timeout;
  while (Date.now() < deadline) {
    const status = await fetchJSON(page, `/api/prompt-bar/submissions/${encodeURIComponent(submissionId)}`);
    if (status.decision) return status.decision;
    if (['failed', 'blocked', 'cancelled'].includes(status.state)) {
      throw new Error(`prompt submission ${submissionId} ended as ${status.state}: ${status.error || ''}`);
    }
    await page.waitForTimeout(500);
  }
  throw new Error(`prompt submission ${submissionId} did not produce a decision`);
}

test('Trace and Settings stay product-safe while app and theme metadata come from product config', async ({ page, authenticator }, testInfo) => {
  const forbiddenRequests = [];
  const failedTraceRequests = [];
  const failedProductRequests = [];

  page.on('request', (request) => {
    const url = new URL(request.url());
    if (
      url.pathname.startsWith('/api/agent/') ||
      url.pathname.startsWith('/api/prompts') ||
      url.pathname.startsWith('/api/test/') ||
      url.pathname.startsWith('/internal') ||
      url.pathname === '/api/events'
    ) {
      forbiddenRequests.push(`${request.method()} ${url.pathname}`);
    }
  });

  page.on('response', (response) => {
    const url = new URL(response.url());
    if (url.pathname.startsWith('/api/trace/') && response.status() >= 400) {
      failedTraceRequests.push(`${url.pathname}:${response.status()}`);
    }
    if (
      (url.pathname === '/health' ||
        url.pathname === '/api/promotions' ||
        url.pathname.startsWith('/api/shell/') ||
        url.pathname.startsWith('/api/desktop/') ||
        url.pathname.startsWith('/api/vtext/')) &&
      response.status() >= 400
    ) {
      failedProductRequests.push(`${url.pathname}:${response.status()}`);
    }
  });

  const email = uniqueEmail('product-safe');
  await registerAndLoadDesktop(page, email);

  const rootTheme = await page.locator('.app-root').evaluate((node) => {
    const style = getComputedStyle(node);
    return {
      id: node.getAttribute('data-theme-id'),
      bg: style.getPropertyValue('--choir-bg').trim(),
      panel: style.getPropertyValue('--choir-panel').trim(),
      border: style.getPropertyValue('--choir-border').trim(),
      bottomBarHeight: style.getPropertyValue('--choir-bottom-bar-height').trim(),
    };
  });
  expect(rootTheme.id).toBe('system-noir');
  expect(rootTheme.bg).toBe('#0b0d10');
  expect(rootTheme.panel).toBe('#171827');
  expect(rootTheme.border).toBe('rgba(148, 163, 184, 0.18)');
  expect(rootTheme.bottomBarHeight).toMatch(/^\d+px$/);
  expect(Number.parseInt(rootTheme.bottomBarHeight, 10)).toBeGreaterThanOrEqual(56);

  const expectedApps = [
    ['files', 'Files', '📁'],
    ['browser', 'Web Lens', '🌐'],
    ['terminal', 'Terminal', '💻'],
    ['settings', 'Settings', '⚙️'],
    ['vtext', 'VText', '📝'],
    ['trace', 'Trace', '🔎'],
  ];
  for (const [appId, label, icon] of expectedApps) {
    const appIcon = page.locator(`[data-desktop-icon-id="${appId}"]`);
    await expect(appIcon).toBeVisible();
    await expect(appIcon.locator('[data-desktop-icon-label]')).toContainText(label);
    await expect(appIcon.locator('[data-desktop-icon-emoji]')).toContainText(icon);
  }

  await openApp(page, 'settings');
  const settings = page.locator('[data-settings-app]').last();
  await expect(settings).toBeVisible({ timeout: 10000 });
  await expect(settings.locator('[data-settings-account]')).toContainText(email);
  await expect(settings.locator('[data-settings-theme-validation]')).toContainText('valid config');
  await expect(settings.locator('[data-theme-presets]')).toBeVisible();
  await expect(settings.locator('[data-theme-preset="next-workstation"]')).toBeVisible();
  await settings.locator('[data-theme-preset="frutiger-aero"]').click();
  await expect(settings.locator('[data-settings-theme-validation]')).toContainText('Frutiger Aero: valid config');
  const appliedTheme = await page.locator('.app-root').evaluate((node) => ({
    id: node.getAttribute('data-theme-id'),
    accent: getComputedStyle(node).getPropertyValue('--choir-accent').trim(),
  }));
  expect(appliedTheme.id).toBe('frutiger-aero');
  expect(appliedTheme.accent).toBe('#7bd923');
  const editorValue = await settings.locator('[data-theme-editor]').inputValue();
  expect(editorValue).toContain('"id": "frutiger-aero"');
  await expect(settings.locator('[data-settings-runtime-status]')).toBeVisible();
  await expect(settings.locator('[data-settings-promotions]')).toBeVisible();
  await expect(settings.locator('[data-settings-promotions-empty]')).toContainText('No candidate patchsets queued.');
  await expect(settings).not.toContainText('Editable role prompt');
  await expect(settings).not.toContainText('/api/prompts');

  await openApp(page, 'trace');
  const trace = page.locator('[data-trace-window]').last();
  await expect(trace.locator('[data-trace-app]')).toBeVisible({ timeout: 10000 });
  await expect(trace.locator('[data-trace-trajectory-list]')).toBeVisible();
  await expect(trace.locator('[data-trace-app]')).toContainText(/Trace|No trajectories|Select a trajectory/);

  await page.waitForTimeout(500);
  expect(forbiddenRequests).toHaveLength(0);
  expect(failedTraceRequests).toHaveLength(0);
  expect(failedProductRequests).toHaveLength(0);

  await attachScreenshot(page, testInfo, 'trace-settings-registry');
});

test('Settings renders queued promotion candidates without browser-internal routes', async ({ page, authenticator, request }) => {
  const forbiddenRequests = [];
  page.on('request', (browserRequest) => {
    const url = new URL(browserRequest.url());
    if (url.pathname.startsWith('/internal')) {
      forbiddenRequests.push(`${browserRequest.method()} ${url.pathname}`);
    }
  });

  const email = uniqueEmail('promotion-queue');
  await registerAndLoadDesktop(page, email);
  const session = await getSession(page, BASE_URL);
  expect(session.authenticated).toBe(true);
  expect(session.user?.id).toBeTruthy();

  const candidateID = `candidate-ui-${Date.now()}`;
  const seed = await request.post('http://127.0.0.1:8085/internal/promotions', {
    headers: {
      'Content-Type': 'application/json',
      'X-Internal-Caller': 'true',
    },
    data: {
      candidate_id: candidateID,
      owner_id: session.user.id,
      status: 'queued',
      source_loop_id: 'seeded-product-test',
      trace_id: 'trace-seeded-product-test',
      vm_id: 'vm-product-test',
      snapshot_id: 'snapshot-product-test',
      base_sha: 'base-product-test',
      worker_head_sha: 'worker-product-test',
      manifest_path: '/tmp/manifest.json',
      patchset_path: '/tmp/patch.diff',
      integration_branch: 'agent/seeded-product-test/candidate',
      destination_branch: 'main',
      summary: 'Seeded promotion queue candidate',
    },
  });
  expect(seed.status()).toBe(202);

  await openApp(page, 'settings');
  const settings = page.locator('[data-settings-app]').last();
  await expect(settings.locator('[data-settings-promotions-list]')).toBeVisible({ timeout: 10000 });
  const candidate = settings.locator(`[data-settings-promotion-id="${candidateID}"]`);
  await expect(candidate).toContainText('Seeded promotion queue candidate');
  await expect(candidate).toContainText('vm-product-test');
  await expect(candidate.locator('[data-settings-promotion-status]')).toContainText('queued');
  await expect(candidate.locator('[data-settings-promotion-verify]')).toBeVisible();
  expect(forbiddenRequests).toHaveLength(0);
});

test('Candidate desktop viewer opens queued promotion candidates without manual IDs', async ({ page, authenticator, request }) => {
  const forbiddenRequests = [];
  page.on('request', (browserRequest) => {
    const url = new URL(browserRequest.url());
    if (url.pathname.startsWith('/internal')) {
      forbiddenRequests.push(`${browserRequest.method()} ${url.pathname}`);
    }
  });

  const email = uniqueEmail('candidate-viewer');
  await registerAndLoadDesktop(page, email);
  const session = await getSession(page, BASE_URL);
  expect(session.authenticated).toBe(true);
  expect(session.user?.id).toBeTruthy();

  const candidateID = `candidate-viewer-${Date.now()}`;
  const seed = await request.post('http://127.0.0.1:8085/internal/promotions', {
    headers: {
      'Content-Type': 'application/json',
      'X-Internal-Caller': 'true',
    },
    data: {
      candidate_id: candidateID,
      owner_id: session.user.id,
      status: 'queued',
      source_loop_id: 'seeded-candidate-viewer-test',
      trace_id: 'trace-seeded-candidate-viewer-test',
      vm_id: 'vm-candidate-viewer-test',
      snapshot_id: 'snapshot-candidate-viewer-test',
      base_sha: 'base-candidate-viewer-test',
      worker_head_sha: 'worker-candidate-viewer-test',
      manifest_path: '/tmp/candidate-viewer-manifest.json',
      patchset_path: '/tmp/candidate-viewer.patch',
      integration_branch: 'agent/seeded-candidate-viewer/candidate',
      destination_branch: 'main',
      summary: 'Candidate desktop contextual viewer test',
    },
  });
  expect(seed.status()).toBe(202);

  await openStartApp(page, 'candidate-desktop');
  const viewer = page.locator('[data-candidate-desktop-viewer]');
  await expect(viewer).toBeVisible({ timeout: 10_000 });
  await expect(viewer.locator('[data-candidate-desktop-list]')).toBeVisible({ timeout: 10_000 });

  const candidate = viewer.locator(`[data-candidate-desktop-candidate-id="${candidateID}"]`);
  await expect(candidate).toContainText('Candidate desktop contextual viewer test');
  await expect(candidate).toContainText('vm-candidate-viewer-test');
  await expect(candidate.locator('[data-candidate-desktop-status]')).toContainText('queued');
  await candidate.locator('[data-candidate-desktop-open-candidate]').click();

  await expect(viewer).toHaveAttribute('data-candidate-desktop-id', 'vm-candidate-viewer-test');
  const frame = viewer.locator('[data-candidate-desktop-frame]');
  await expect(frame).toBeVisible({ timeout: 10_000 });
  await expect(frame).toHaveAttribute('src', /desktop_id=vm-candidate-viewer-test/);
  await expect(frame).toHaveAttribute('src', /embedded=1/);
  expect(forbiddenRequests).toHaveLength(0);
});

test('Trace selects a synthesized next objective from a queued promotion candidate without internal browser routes', async ({ page, authenticator, request }) => {
  const forbiddenRequests = [];
  const failedContinuationRequests = [];

  page.on('request', (browserRequest) => {
    const url = new URL(browserRequest.url());
    if (url.pathname.startsWith('/internal')) {
      forbiddenRequests.push(`${browserRequest.method()} ${url.pathname}`);
    }
  });

  page.on('response', (response) => {
    const url = new URL(response.url());
    if (url.pathname.startsWith('/api/continuations') && response.status() >= 400) {
      failedContinuationRequests.push(`${url.pathname}:${response.status()}`);
    }
  });

  const email = uniqueEmail('trace-continuation');
  await registerAndLoadDesktop(page, email);
  const session = await getSession(page, BASE_URL);
  expect(session.authenticated).toBe(true);
  expect(session.user?.id).toBeTruthy();

  const promptURL = `https://example.com/trace-continuation-${Date.now()}.pdf`;
  const promptBarResponse = page.waitForResponse((response) =>
    new URL(response.url()).pathname === '/api/prompt-bar' && response.request().method() === 'POST'
  );
  await page.locator('[data-prompt-input]').fill(promptURL);
  await page.locator('[data-prompt-input]').press('Enter');
  const submitted = await (await promptBarResponse).json();
  const decision = await waitForPromptDecision(page, submitted.submission_id);
  expect(decision.action).toBe('open_app');
  expect(decision.app).toBe('pdf');

  const candidateID = `candidate-trace-continuation-${Date.now()}`;
  const seed = await request.post('http://127.0.0.1:8085/internal/promotions', {
    headers: {
      'Content-Type': 'application/json',
      'X-Internal-Caller': 'true',
    },
    data: {
      candidate_id: candidateID,
      owner_id: session.user.id,
      status: 'queued',
      source_loop_id: submitted.submission_id,
      trace_id: submitted.submission_id,
      vm_id: 'vm-trace-continuation',
      snapshot_id: 'snapshot-trace-continuation',
      base_sha: 'base-trace-continuation',
      worker_head_sha: 'worker-trace-continuation',
      manifest_path: '/tmp/trace-continuation-manifest.json',
      patchset_path: '/tmp/trace-continuation.patch',
      integration_branch: 'agent/trace-continuation/candidate',
      destination_branch: 'main',
      summary: 'Trace selected continuation candidate',
    },
  });
  expect(seed.status()).toBe(202);

  await openApp(page, 'trace');
  const trace = page.locator('[data-trace-app]').last();
  await expect(trace).toBeVisible({ timeout: 10_000 });
  const trajectory = trace.locator(`[data-trace-trajectory-id="${submitted.submission_id}"]`);
  await expect(trajectory).toBeVisible({ timeout: 20_000 });
  await trajectory.click();

  await expect(trace.locator('[data-trace-select-continuation]')).toBeVisible({ timeout: 10_000 });
  const continuationResponse = page.waitForResponse((response) =>
    new URL(response.url()).pathname === '/api/continuations' && response.request().method() === 'POST'
  );
  await trace.locator('[data-trace-select-continuation]').click();
  const continuation = await (await continuationResponse).json();
  expect(continuation.status).toBe('selected');
  expect(continuation.details?.candidate_id).toBe(candidateID);

  const proposal = trace.locator('[data-trace-continuation-proposal]');
  await expect(proposal).toBeVisible({ timeout: 10_000 });
  await expect(proposal).toContainText('Verify queued promotion candidate');
  await expect(proposal).toContainText(candidateID);
  await expect(proposal.locator('[data-trace-start-continuation]')).toBeVisible();

  expect(forbiddenRequests).toHaveLength(0);
  expect(failedContinuationRequests).toHaveLength(0);
});

test('Settings records owner approval for verified promotion candidates without internal browser routes', async ({ page, authenticator, request }) => {
  const forbiddenRequests = [];
  page.on('request', (browserRequest) => {
    const url = new URL(browserRequest.url());
    if (url.pathname.startsWith('/internal')) {
      forbiddenRequests.push(`${browserRequest.method()} ${url.pathname}`);
    }
  });

  const email = uniqueEmail('promotion-approve');
  await registerAndLoadDesktop(page, email);
  const session = await getSession(page, BASE_URL);
  expect(session.authenticated).toBe(true);
  expect(session.user?.id).toBeTruthy();

  const candidateID = `candidate-approve-${Date.now()}`;
  const seed = await request.post('http://127.0.0.1:8085/internal/promotions', {
    headers: {
      'Content-Type': 'application/json',
      'X-Internal-Caller': 'true',
    },
    data: {
      candidate_id: candidateID,
      owner_id: session.user.id,
      status: 'verified',
      source_loop_id: 'seeded-approval-test',
      trace_id: 'trace-seeded-approval-test',
      vm_id: 'vm-approval-test',
      snapshot_id: 'snapshot-approval-test',
      base_sha: 'base-approval-test',
      worker_head_sha: 'worker-approval-test',
      manifest_path: '/tmp/approval-manifest.json',
      patchset_path: '/tmp/approval.patch',
      integration_branch: 'agent/seeded-approval-test/candidate',
      destination_branch: 'main',
      summary: 'Verified candidate awaiting owner approval',
      report_json: {
        status: 'verified',
        promotion_approved: false,
      },
    },
  });
  expect(seed.status()).toBe(202);

  await openApp(page, 'settings');
  const settings = page.locator('[data-settings-app]').last();
  await expect(settings.locator('[data-settings-promotions-list]')).toBeVisible({ timeout: 10000 });
  const candidate = settings.locator(`[data-settings-promotion-id="${candidateID}"]`);
  await expect(candidate).toContainText('Verified candidate awaiting owner approval');
  await expect(candidate.locator('[data-settings-promotion-status]')).toContainText('verified');
  await candidate.locator('[data-settings-promotion-approve]').click();
  await expect(candidate.locator('[data-settings-promotion-approved]')).toContainText('Owner approved');
  await expect(candidate.locator('[data-settings-promotion-approve]')).toHaveCount(0);
  expect(forbiddenRequests).toHaveLength(0);
});
