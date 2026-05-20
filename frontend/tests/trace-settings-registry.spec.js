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

async function postJSON(page, path, data) {
  return page.evaluate(async ({ requestPath, requestBody }) => {
    const res = await fetch(requestPath, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'include',
      body: JSON.stringify(requestBody),
    });
    const body = await res.text();
    if (!res.ok) {
      throw new Error(`${requestPath} failed: ${res.status} ${body}`);
    }
    return body ? JSON.parse(body) : null;
  }, { requestPath: path, requestBody: data });
}

async function createAppPackageAndAdoption(page, label, overrides = {}) {
  const stamp = `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
  const packageID = overrides.packageID || `package-${label}-${stamp}`;
  const targetComputerID = overrides.targetComputerID || `target-computer-${label}`;
  const targetCandidateID = overrides.targetCandidateID || `target-candidate-${label}-${stamp}`;
  const appID = overrides.appID || `${label}-app`;
  const pkg = await postJSON(page, '/api/app-change-packages', {
    package_id: packageID,
    app_id: appID,
    visibility: 'unlisted',
    source_computer_id: `source-computer-${label}`,
    source_candidate_id: `source-candidate-${label}-${stamp}`,
    candidate_source_ref: `refs/computers/source-${label}/candidates/${stamp}`,
    source_ledger_base_ref: `base-${label}`,
    source_ledger_candidate_ref: `refs/computers/source-${label}/candidates/${stamp}`,
    source_ledger_commit_sha: `commit-${label}-${stamp}`,
    runtime_source_delta: `diff --git a/${label}-runtime.txt b/${label}-runtime.txt\nnew file mode 100644\n--- /dev/null\n+++ b/${label}-runtime.txt\n@@ -0,0 +1 @@\n+${label} runtime\n`,
    ui_source_delta: `diff --git a/frontend/${label}-ui.txt b/frontend/${label}-ui.txt\nnew file mode 100644\n--- /dev/null\n+++ b/frontend/${label}-ui.txt\n@@ -0,0 +1 @@\n+${label} ui\n`,
    app_protocol_contract: `${label} protocol contract requires recipient runtime/ui build`,
    trace_id: overrides.traceID || '',
  });
  const adoption = await postJSON(page, `/api/computers/${encodeURIComponent(targetComputerID)}/adoptions`, {
    adoption_id: overrides.adoptionID || `adoption-${label}-${stamp}`,
    package_id: packageID,
    target_candidate_id: targetCandidateID,
    candidate_source_ref: `refs/computers/${targetComputerID}/candidates/${targetCandidateID}`,
    foreground_tail_merge_result: 'not_tested_product_ui_fixture',
    merge_strategy: 'fixture-no-conflict',
    trace_id: overrides.traceID || '',
  });
  return { pkg, adoption, packageID, targetComputerID, targetCandidateID, appID };
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
        url.pathname === '/api/app-change-packages' ||
        url.pathname === '/api/adoptions' ||
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
  await expect(settings.locator('[data-settings-promotions-empty]')).toContainText('No AppChangePackages or recipient adoptions yet.');
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

test('Settings renders AppChangePackages and adoptions without browser-internal routes', async ({ page, authenticator }) => {
  const forbiddenRequests = [];
  page.on('request', (browserRequest) => {
    const url = new URL(browserRequest.url());
    if (url.pathname.startsWith('/internal')) {
      forbiddenRequests.push(`${browserRequest.method()} ${url.pathname}`);
    }
  });

  const email = uniqueEmail('app-package-settings');
  await registerAndLoadDesktop(page, email);
  const session = await getSession(page, BASE_URL);
  expect(session.authenticated).toBe(true);
  expect(session.user?.id).toBeTruthy();

  const legacyPromotionsRoute = await page.evaluate(async () => {
    const res = await fetch('/api/promotions', { credentials: 'include' });
    return { status: res.status, body: await res.text() };
  });
  expect(legacyPromotionsRoute.status).toBe(404);
  expect(legacyPromotionsRoute.body).not.toContain('PromotionCandidate');

  const seeded = await createAppPackageAndAdoption(page, 'settings');

  await openApp(page, 'settings');
  const settings = page.locator('[data-settings-app]').last();
  await expect(settings.locator('[data-settings-promotions-list]')).toBeVisible({ timeout: 10000 });
  const pkg = settings.locator(`[data-settings-package-id="${seeded.packageID}"]`);
  await expect(pkg).toContainText(seeded.appID);
  await expect(pkg).toContainText('unlisted');
  const adoption = settings.locator(`[data-settings-adoption-id="${seeded.adoption.adoption_id}"]`);
  await expect(adoption).toContainText(seeded.appID);
  await expect(adoption).toContainText(seeded.targetComputerID);
  await expect(adoption.locator('[data-settings-promotion-status]')).toContainText('candidate_applied');
  await expect(adoption.locator('[data-settings-adoption-verify]')).toBeVisible();
  expect(forbiddenRequests).toHaveLength(0);
});

test('Candidate desktop viewer opens app adoption candidates without manual IDs', async ({ page, authenticator }) => {
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

  const seeded = await createAppPackageAndAdoption(page, 'candidate-viewer', {
    targetComputerID: 'target-computer-candidate-viewer',
    targetCandidateID: 'vm-candidate-viewer-test',
    appID: 'Candidate viewer package',
  });

  await openStartApp(page, 'candidate-desktop');
  const viewer = page.locator('[data-candidate-desktop-viewer]');
  await expect(viewer).toBeVisible({ timeout: 10_000 });
  await expect(viewer.locator('[data-candidate-desktop-list]')).toBeVisible({ timeout: 10_000 });

  const candidate = viewer.locator(`[data-candidate-desktop-candidate-id="${seeded.adoption.adoption_id}"]`);
  await expect(candidate).toContainText('Candidate viewer package');
  await expect(candidate).toContainText('target-computer-candidate-viewer');
  await expect(candidate.locator('[data-candidate-desktop-status]')).toContainText('proposed');
  await candidate.locator('[data-candidate-desktop-open-candidate]').click();

  await expect(viewer).toHaveAttribute('data-candidate-desktop-id', 'vm-candidate-viewer-test');
  const frame = viewer.locator('[data-candidate-desktop-frame]');
  await expect(frame).toBeVisible({ timeout: 10_000 });
  await expect(frame).toHaveAttribute('src', /desktop_id=vm-candidate-viewer-test/);
  await expect(frame).toHaveAttribute('src', /embedded=1/);
  expect(forbiddenRequests).toHaveLength(0);
});
