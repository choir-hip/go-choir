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
  await openStartApp(page, appId);
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
        url.pathname.startsWith('/api/texture/')) &&
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
      promptSurfaceSize: style.getPropertyValue('--choir-prompt-surface-size').trim(),
    };
  });
  expect(rootTheme.id).toBe('futuristic-noir');
  expect(rootTheme.bg).toBe('#050912');
  expect(rootTheme.panel).toBe('#0D1628');
  expect(rootTheme.border).toBe('rgba(133, 159, 211, 0.22)');
  expect(rootTheme.promptSurfaceSize).toMatch(/^\d+px$/);
  expect(Number.parseInt(rootTheme.promptSurfaceSize, 10)).toBeGreaterThanOrEqual(56);

  const expectedApps = [
    ['files', 'Files', '📁'],
    ['browser', 'Web Lens', '🌐'],
    ['super-console', 'Super Console', '⌘'],
    ['settings', 'Settings', '⚙️'],
    ['vtext', 'Texture', '📝'],
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
  await expect(settings.locator('[data-theme-preset="carbon-fiber-kintsugi"]')).toBeVisible();
  await settings.locator('[data-theme-preset="london-salmon"]').click();
  await expect(settings.locator('[data-settings-theme-validation]')).toContainText('London Salmon: valid config');
  const appliedTheme = await page.locator('.app-root').evaluate((node) => ({
    id: node.getAttribute('data-theme-id'),
    accent: getComputedStyle(node).getPropertyValue('--choir-accent').trim(),
  }));
  expect(appliedTheme.id).toBe('london-salmon');
  expect(appliedTheme.accent).toBe('#A44F38');
  const editorValue = await settings.locator('[data-theme-editor]').inputValue();
  expect(editorValue).toContain('"id": "london-salmon"');
  await expect(settings.locator('[data-settings-runtime-status]')).toBeVisible();
  await expect(settings.locator('[data-settings-promotions]')).toBeVisible();
  const promotionEvidence =
    (await settings.locator('[data-settings-promotions-empty]').count()) +
    (await settings.locator('[data-settings-promotions-list]').count());
  expect(promotionEvidence).toBeGreaterThan(0);
  await expect(settings).not.toContainText('Editable role prompt');
  await expect(settings).not.toContainText('/api/prompts');

  await expect(page.locator('[data-desktop-icon-id="trace"]')).toHaveCount(0);
  await openApp(page, 'super-console');
  const superConsole = page.locator('[data-super-console-app]').last();
  await expect(superConsole.locator('[data-super-console]')).toBeVisible({ timeout: 10000 });

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

test('Apps & Changes opens app adoption candidates without manual package or candidate IDs', async ({ page, authenticator }) => {
  const forbiddenRequests = [];
  page.on('request', (browserRequest) => {
    const url = new URL(browserRequest.url());
    if (url.pathname.startsWith('/internal')) {
      forbiddenRequests.push(`${browserRequest.method()} ${url.pathname}`);
    }
  });

  const seeded = {
    package: {
      package_id: '28433c19-5d02-416f-9368-de56390e1927',
      app_id: 'Chiron Shelf Observability',
      status: 'published_unlisted',
      visibility: 'unlisted',
      source_computer_id: 'primary',
      source_candidate_id: 'source-chiron',
      candidate_source_ref: 'refs/computers/source/candidates/chiron',
      package_manifest_sha256: 'manifest-chiron-ui-test',
    },
    adoption: {
      adoption_id: 'adoption-apps-changes-viewer',
      package_id: '28433c19-5d02-416f-9368-de56390e1927',
      app_id: 'Chiron Shelf Observability',
      target_computer_id: 'target-computer-candidate-viewer',
      target_candidate_id: 'vm-candidate-viewer-test',
      status: 'candidate_applied',
      candidate_source_ref: 'refs/computers/target-computer-candidate-viewer/candidates/vm-candidate-viewer-test',
      rollback_profile_json: '{}',
    },
  };
  await page.route('**/api/app-change-packages*', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ packages: [seeded.package] }),
    });
  });
  await page.route('**/api/adoptions*', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ adoptions: [seeded.adoption] }),
    });
  });
  await page.route('**/api/computers/primary/source-lineage*', async (route) => {
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
  await page.route('**/api/computers/primary/adoptions*', async (route) => {
    await route.fulfill({
      status: 201,
      contentType: 'application/json',
      body: JSON.stringify(seeded.adoption),
    });
  });

  const email = uniqueEmail('candidate-viewer');
  await registerAndLoadDesktop(page, email);
  const session = await getSession(page, BASE_URL);
  expect(session.authenticated).toBe(true);
  expect(session.user?.id).toBeTruthy();

  await page.locator('[data-start-button]').click();
  await expect(page.locator('[data-start-app-id="candidate-desktop"]')).toHaveCount(0);
  await page.locator('[data-start-app-id="apps-changes"]').click();
  const store = page.locator('[data-apps-changes-app]');
  await expect(store).toBeVisible({ timeout: 10_000 });
  await expect(store.locator('[data-change-card][data-change-id="chiron-shelf"]')).toBeVisible({ timeout: 10_000 });
  if (await store.locator('[data-change-try]').isEnabled()) {
    await store.locator('[data-change-try]').click();
  }

  const frame = store.locator('[data-change-preview-frame]');
  await expect(frame).toBeVisible({ timeout: 10_000 });
  await expect(frame).toHaveAttribute('data-change-preview-desktop-id', 'vm-candidate-viewer-test');
  const iframe = frame.locator('[data-change-preview-iframe]');
  await expect(iframe).toHaveAttribute('src', /desktop_id=vm-candidate-viewer-test/);
  await expect(iframe).toHaveAttribute('src', /embedded=1/);
  await expect(store.locator('[data-candidate-desktop-input]')).toHaveCount(0);
  await expect(store.locator(`[data-review-adoption-id="${seeded.adoption.adoption_id}"]`)).toBeVisible();
  expect(forbiddenRequests).toHaveLength(0);
});
