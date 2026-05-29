/**
 * Playwright tests for the desktop shell core components (VAL-SHELL-001
 * through VAL-SHELL-032).
 *
 * These tests verify the desktop shell rewrite:
 * - No top bar rendered (VAL-SHELL-001)
 * - Floating desktop icons render with emoji and labels (VAL-SHELL-002)
 * - Double-click icon opens single-instance window (VAL-SHELL-003)
 * - Active window indicator on desktop icon (VAL-SHELL-004)
 * - Bottom bar always visible (VAL-SHELL-006)
 * - Bottom bar prompt input (VAL-SHELL-007)
 * - Minimized window indicators in prompt surface (VAL-SHELL-008)
 * - User info and logout in desktop/account menu (VAL-SHELL-009)
 * - Live connection status dot (VAL-SHELL-010)
 * - No bootstrap accordion or runtime panel (VAL-SHELL-024)
 * - No left rail, no hamburger button, no backdrop (VAL-SHELL-026)
 */
import { test, expect } from './helpers/fixtures.js';
import { registerPasskey, getSession } from './helpers/auth.js';

const BASE_URL =
  process.env.GO_CHOIR_UX_BASE_URL ||
  process.env.PLAYWRIGHT_BASE_URL ||
  process.env.BASE_URL ||
  'http://localhost:4173';

function uniqueEmail() {
  return `shell-test-${Date.now()}-${Math.random().toString(36).slice(2, 8)}@example.com`;
}

// Helper: register a passkey and get to the authenticated desktop.
async function registerAndLoadDesktop(page, authenticator, email) {
  await page.goto(BASE_URL);
  await registerPasskey(page, email, BASE_URL);
  await page.reload();
  await page.locator('[data-desktop]').waitFor({ state: 'visible', timeout: 10000 });
}

// Helper: open app via double-click on floating desktop icon
async function openAppViaIcon(page, appId) {
  const icon = page.locator(`[data-desktop-icon-id="${appId}"]`);
  await icon.dblclick();
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

async function waitForPromptSubmissionDecision(page, submissionId, timeout = 10000) {
  const deadline = Date.now() + timeout;
  while (Date.now() < deadline) {
    const status = await fetchJSON(page, `/api/prompt-bar/submissions/${encodeURIComponent(submissionId)}`);
    if (status.decision) {
      return status.decision;
    }
    if (['failed', 'blocked', 'cancelled'].includes(status.state)) {
      throw new Error(status.error || `prompt submission ${submissionId} ended as ${status.state}`);
    }
    await page.waitForTimeout(200);
  }
  throw new Error(`prompt submission ${submissionId} did not produce a decision`);
}

// ---------------------------------------------------------------
// Test: no top bar present after rewrite (VAL-SHELL-001)
// ---------------------------------------------------------------
test('no top bar present after rewrite', async ({ page, authenticator }) => {
  const email = uniqueEmail();
  await registerAndLoadDesktop(page, authenticator, email);

  // data-desktop-bar must be absent from DOM
  const topBar = page.locator('[data-desktop-bar]');
  await expect(topBar).toHaveCount(0);
});

// ---------------------------------------------------------------
// Test: no left rail, no hamburger, no backdrop (VAL-SHELL-026)
// ---------------------------------------------------------------
test('no left rail, no hamburger button, no backdrop', async ({ page, authenticator }) => {
  const email = uniqueEmail();
  await registerAndLoadDesktop(page, authenticator, email);

  // data-desktop-rail must be absent
  await expect(page.locator('[data-desktop-rail]')).toHaveCount(0);

  // data-hamburger-btn must be absent
  await expect(page.locator('[data-hamburger-btn]')).toHaveCount(0);

  // data-rail-backdrop must be absent
  await expect(page.locator('[data-rail-backdrop]')).toHaveCount(0);
});

// ---------------------------------------------------------------
// Test: floating desktop icons render with emoji and labels (VAL-SHELL-002)
// ---------------------------------------------------------------
test('floating desktop icons render with emoji and labels', async ({ page, authenticator }) => {
  const email = uniqueEmail();
  await registerAndLoadDesktop(page, authenticator, email);

  const surface = page.locator('[data-desktop-surface]');
  await expect(surface).toBeVisible();

  // Should have exactly 7 desktop icons (Files, Browser, Terminal, Settings, VText, Trace, Podcast)
  const icons = surface.locator('[data-desktop-icon]');
  await expect(icons).toHaveCount(7);

  // Verify each app icon is present
  await expect(surface.locator('[data-desktop-icon-id="files"]')).toBeVisible();
  await expect(surface.locator('[data-desktop-icon-id="browser"]')).toBeVisible();
  await expect(surface.locator('[data-desktop-icon-id="terminal"]')).toBeVisible();
  await expect(surface.locator('[data-desktop-icon-id="settings"]')).toBeVisible();
  await expect(surface.locator('[data-desktop-icon-id="vtext"]')).toBeVisible();
  await expect(surface.locator('[data-desktop-icon-id="trace"]')).toBeVisible();
  await expect(surface.locator('[data-desktop-icon-id="podcast"]')).toBeVisible();

  // Each icon should have an emoji and a label
  const filesEmoji = surface.locator('[data-desktop-icon-id="files"] [data-desktop-icon-emoji]');
  await expect(filesEmoji).toContainText('📁');

  const filesLabel = surface.locator('[data-desktop-icon-id="files"] [data-desktop-icon-label]');
  await expect(filesLabel).toContainText('Files');
});

// ---------------------------------------------------------------
// Test: VText appears as a first-class desktop app
// ---------------------------------------------------------------
test('VText appears as a first-class desktop app', async ({ page, authenticator }) => {
  const email = uniqueEmail();
  await registerAndLoadDesktop(page, authenticator, email);

  const vtextIcon = page.locator('[data-desktop-icon-id="vtext"]');
  await expect(vtextIcon).toBeVisible();
  await expect(vtextIcon).toContainText('VText');

  await vtextIcon.dblclick();

  const vtextWindow = page.locator('[data-vtext-app]');
  await expect(vtextWindow).toBeVisible({ timeout: 5000 });
  await expect(vtextWindow.locator('[data-vtext-recent]')).toBeVisible();

  const titleText = page.locator('[data-window-titlebar] .titlvtext');
  await expect(titleText.first()).toContainText('VText');
});

test('bottom-left Start menu launches registered apps', async ({ page, authenticator }) => {
  const email = uniqueEmail();
  await registerAndLoadDesktop(page, authenticator, email);

  await page.locator('[data-start-button]').click();
  const startMenu = page.locator('[data-start-menu]');
  await expect(startMenu).toBeVisible();
  await expect(startMenu.locator('[data-start-app-id="files"]')).toBeVisible();
  await expect(startMenu.locator('[data-start-app-id="settings"]')).toBeVisible();
  await expect(startMenu.locator('[data-start-app-id="podcast"]')).toBeVisible();
  await expect(startMenu.locator('[data-start-app-id="image"]')).toBeVisible();
  await expect(startMenu.locator('[data-start-app-id="audio"]')).toBeVisible();
  await expect(startMenu.locator('[data-start-app-id="video"]')).toBeVisible();
  await expect(startMenu.locator('[data-start-app-id="pdf"]')).toBeVisible();
  await expect(startMenu.locator('[data-start-app-id="epub"]')).toBeVisible();

  await startMenu.locator('[data-start-app-id="files"]').click();
  await expect(page.locator('[data-files-app]').last()).toBeVisible({ timeout: 10000 });
});

test('logged-out desktop can open read-only Podcast without the auth wall', async ({ page }) => {
  await page.goto(BASE_URL);
  await page.locator('[data-desktop]').waitFor({ state: 'visible', timeout: 10000 });
  await expect(page.locator('[data-desktop]')).toHaveAttribute('data-authenticated', 'false');

  await page.locator('[data-start-button]').click();
  await page.locator('[data-start-app-id="podcast"]').click();

  const podcastWindow = page.locator('[data-podcast-app]').last();
  await expect(podcastWindow).toBeVisible({ timeout: 5000 });
  await expect(podcastWindow.locator('[data-podcast-library-recommended]')).toBeVisible();
  await expect(page.locator('[data-auth-entry]')).toHaveCount(0);
});

test('logged-out desktop opens Browser and Trace as read shells before auth', async ({ page }) => {
  const protectedAppRequests = [];
  page.on('request', (request) => {
    const pathname = new URL(request.url()).pathname;
    if (pathname.startsWith('/api/browser') || pathname.startsWith('/api/trace')) {
      protectedAppRequests.push(pathname);
    }
  });

  await page.goto(BASE_URL);
  await page.locator('[data-desktop]').waitFor({ state: 'visible', timeout: 10000 });
  await expect(page.locator('[data-desktop]')).toHaveAttribute('data-authenticated', 'false');

  await page.locator('[data-start-button]').click();
  await page.locator('[data-start-app-id="browser"]').click();

  const browserApp = page.locator('[data-browser-app]').last();
  await expect(browserApp).toBeVisible({ timeout: 5000 });
  await expect(browserApp.locator('[data-browser-backend-status]')).toHaveAttribute(
    'data-browser-backend-mode',
    'guest_iframe',
  );

  const artifactUrl = 'data:text/html;charset=utf-8,%3Ch1%3EGuest%20browser%3C%2Fh1%3E';
  await browserApp.locator('[data-browser-url-input]').fill(artifactUrl);
  await browserApp.locator('[data-browser-go-btn]').click();
  await expect(browserApp.locator('[data-browser-iframe]')).toHaveAttribute('src', artifactUrl);
  expect(protectedAppRequests).toEqual([]);
  await expect(page.locator('[data-auth-entry]')).toHaveCount(0);

  await page.locator('[data-start-button]').click();
  await page.locator('[data-start-app-id="trace"]').click();

  const traceApp = page.locator('[data-trace-app]').last();
  await expect(traceApp).toBeVisible({ timeout: 5000 });
  await expect(traceApp.locator('[data-trace-guest]')).toBeVisible();
  await expect(traceApp.locator('[data-trace-guest-sidebar]')).toBeVisible();
  expect(protectedAppRequests).toEqual([]);
  await expect(page.locator('[data-auth-entry]')).toHaveCount(0);

  await traceApp.locator('[data-trace-guest-sign-in]').click();
  await expect(page.locator('[data-auth-overlay]')).toBeVisible();
  await expect(page.locator('[data-auth-entry]')).toBeVisible();
});

test('mobile prompt-surface restore raises the selected app above the current window', async ({ page, authenticator }) => {
  const email = uniqueEmail();
  await page.setViewportSize({ width: 390, height: 844 });
  await registerAndLoadDesktop(page, authenticator, email);

  await openAppViaIcon(page, 'files');
  const filesWindow = page.locator('[data-window]').filter({ has: page.locator('[data-files-app]') }).last();
  await expect(filesWindow).toBeVisible({ timeout: 5000 });
  await filesWindow.locator('[data-window-minimize]').click();
  await expect(filesWindow).not.toBeVisible();

  await openAppViaIcon(page, 'browser');
  const browserWindow = page.locator('[data-window]').filter({ has: page.locator('[data-browser-app]') }).last();
  await expect(browserWindow).toBeVisible({ timeout: 5000 });

  const filesIndicator = page.locator('[data-window-indicator]').filter({ hasText: 'Files' }).last();
  await expect(filesIndicator).toBeVisible();
  await filesIndicator.click();

  await expect(filesWindow).toBeVisible();
  await expect(filesIndicator).toHaveAttribute('data-window-indicator-active', 'true');

  const z = await Promise.all([
    filesWindow.evaluate((el) => Number(getComputedStyle(el).zIndex) || 0),
    browserWindow.evaluate((el) => Number(getComputedStyle(el).zIndex) || 0),
  ]);
  expect(z[0]).toBeGreaterThan(z[1]);
});

test('VText recent landing can open a Markdown document without control overlap', async ({ page, authenticator }) => {
  const email = uniqueEmail();
  await registerAndLoadDesktop(page, authenticator, email);

  const created = await page.evaluate(async () => {
    const docRes = await fetch('/api/vtext/documents', {
      method: 'POST',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ title: 'Markdown UX Fixture' }),
    });
    if (!docRes.ok) throw new Error(`create doc failed: ${docRes.status}`);
    const doc = await docRes.json();
    const revRes = await fetch(`/api/vtext/documents/${encodeURIComponent(doc.doc_id)}/revisions`, {
      method: 'POST',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        content: '# Markdown UX Fixture\n\nThis has **bold** text, *emphasis*, a [link](https://example.com), and a list.\n\n- First item\n- Second item',
        author_kind: 'user',
        author_label: 'browser-test',
      }),
    });
    if (!revRes.ok) throw new Error(`create rev failed: ${revRes.status}`);
    return doc;
  });

  await page.locator('[data-desktop-icon-id="vtext"]').dblclick();
  const vtextWindow = page.locator('[data-vtext-app]').last();
  await expect(vtextWindow.locator('[data-vtext-recent]')).toBeVisible({ timeout: 5000 });
  await vtextWindow.locator('[data-vtext-recent-document]').filter({ hasText: created.title }).click();

  const toolbar = vtextWindow.locator('[data-vtext-toolbar]');
  const body = vtextWindow.locator('[data-vtext-document-body]');
  await expect(toolbar).toBeVisible();
  await expect(body).toBeVisible();
  const [toolbarBox, bodyBox] = await Promise.all([toolbar.boundingBox(), body.boundingBox()]);
  expect(toolbarBox.y + toolbarBox.height).toBeLessThanOrEqual(bodyBox.y + 1);

  const rendered = vtextWindow.locator('[data-vtext-rendered]');
  await expect(rendered.locator('h1')).toContainText('Markdown UX Fixture');
  await expect(rendered.locator('strong')).toContainText('bold');
  await expect(rendered.locator('em')).toContainText('emphasis');
  await expect(rendered.locator('a')).toHaveAttribute('href', 'https://example.com');
  await expect(rendered.locator('li')).toHaveCount(2);
  await expect(vtextWindow.locator('[data-vtext-editor-area]')).toContainText(/Markdown UX Fixture/);
});

test('VText opens near full mobile workspace and clears the prompt bar', async ({ page, authenticator }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  const email = uniqueEmail();
  await registerAndLoadDesktop(page, authenticator, email);

  await page.locator('[data-desktop-icon-id="vtext"]').dblclick();
  const windowEl = page.locator('[data-window]').filter({ has: page.locator('[data-vtext-app]') }).last();
  await expect(windowEl).toBeVisible({ timeout: 5000 });

  const box = await windowEl.boundingBox();
  expect(box.width).toBeGreaterThanOrEqual(350);
  expect(box.height).toBeGreaterThanOrEqual(720);
  expect(box.x).toBeGreaterThanOrEqual(8);
  expect(box.y).toBeGreaterThanOrEqual(8);
  expect(box.y + box.height).toBeLessThanOrEqual(844 - 56 + 2);
});

// ---------------------------------------------------------------
// Test: Trace appears as a debugging desktop app
// ---------------------------------------------------------------
test('Trace appears as a debugging desktop app', async ({ page, authenticator }) => {
  const email = uniqueEmail();
  await registerAndLoadDesktop(page, authenticator, email);

  const traceIcon = page.locator('[data-desktop-icon-id="trace"]');
  await expect(traceIcon).toBeVisible();
  await expect(traceIcon).toContainText('Trace');

  await traceIcon.dblclick();

  const traceWindow = page.locator('[data-trace-window]');
  await expect(traceWindow).toBeVisible({ timeout: 5000 });
  await expect(traceWindow.locator('[data-trace-app]')).toBeVisible();
});

test('Trace opens a prompt-bar trajectory without stale route calls', async ({ page, authenticator }) => {
  const email = uniqueEmail();
  await registerAndLoadDesktop(page, authenticator, email);

  const staleRequests = [];
  const failedTraceRequests = [];
  page.on('response', (response) => {
    const url = new URL(response.url());
    if (['/api/agent/topology', '/api/prompts', '/api/events'].includes(url.pathname)) {
      staleRequests.push(url.pathname);
    }
    if (url.pathname.startsWith('/api/trace/') && response.status() >= 400) {
      failedTraceRequests.push(`${url.pathname}:${response.status()}`);
    }
  });

  const prompt = `Trace smoke ${Date.now()}`;
  await page.locator('[data-prompt-input]').fill(prompt);
  await page.locator('[data-prompt-input]').press('Enter');
  await expect(page.locator('[data-vtext-app]').last()).toBeVisible({ timeout: 10000 });

  await openAppViaIcon(page, 'trace');
  const traceWindow = page.locator('[data-trace-window]').last();
  await expect(traceWindow.locator('[data-trace-trajectory]').filter({ hasText: prompt })).toBeVisible({ timeout: 10000 });
  expect(staleRequests).toHaveLength(0);
  expect(failedTraceRequests).toHaveLength(0);
});

// ---------------------------------------------------------------
// Test: double-click icon opens single-instance window (VAL-SHELL-003)
// ---------------------------------------------------------------
test('double-click icon opens single-instance window', async ({ page, authenticator }) => {
  const email = uniqueEmail();
  await registerAndLoadDesktop(page, authenticator, email);

  // Double-click the Files icon
  await openAppViaIcon(page, 'files');

  // A window should appear
  const windowEl = page.locator('[data-window]');
  await expect(windowEl).toHaveCount(1);
  await expect(windowEl.first()).toBeVisible({ timeout: 5000 });

  // Double-click the same icon again — should NOT open a second window
  await openAppViaIcon(page, 'files');
  await expect(page.locator('[data-window]')).toHaveCount(1);

  // The window title should match
  const titleText = page.locator('[data-window-titlebar] .titlvtext');
  await expect(titleText.first()).toContainText('Files');
});

// ---------------------------------------------------------------
// Test: floating icon active indicator (VAL-SHELL-004)
// ---------------------------------------------------------------
test('floating icon active indicator highlights open app', async ({ page, authenticator }) => {
  const email = uniqueEmail();
  await registerAndLoadDesktop(page, authenticator, email);

  // Open Files app
  await openAppViaIcon(page, 'files');
  await page.locator('[data-window]').first().waitFor({ state: 'visible', timeout: 5000 });

  // Files icon should have icon-active class
  const filesIcon = page.locator('[data-desktop-icon-id="files"].icon-active');
  await expect(filesIcon).toBeVisible();

  // Open another app — Browser
  await openAppViaIcon(page, 'browser');
  await page.waitForTimeout(300);

  // Browser should now be active
  const browserIcon = page.locator('[data-desktop-icon-id="browser"].icon-active');
  await expect(browserIcon).toBeVisible();
});

// ---------------------------------------------------------------
// Test: prompt surface always visible (VAL-SHELL-006)
// ---------------------------------------------------------------
test('prompt surface always visible', async ({ page, authenticator }) => {
  const email = uniqueEmail();
  await registerAndLoadDesktop(page, authenticator, email);

  const promptSurface = page.locator('[data-prompt-surface]');
  await expect(promptSurface).toBeVisible();

  // Bottom bar should have a fixed height approximately 56px
  const height = await promptSurface.evaluate((el) => el.offsetHeight);
  expect(height).toBeGreaterThanOrEqual(52);
  expect(height).toBeLessThanOrEqual(60);

  // Open a window and check prompt surface is still visible
  await openAppViaIcon(page, 'files');
  await page.locator('[data-window]').first().waitFor({ state: 'visible', timeout: 5000 });
  await expect(promptSurface).toBeVisible();
});

// ---------------------------------------------------------------
// Test: prompt surface prompt input (VAL-SHELL-007)
// ---------------------------------------------------------------
test('prompt surface prompt input with placeholder', async ({ page, authenticator }) => {
  const email = uniqueEmail();
  await registerAndLoadDesktop(page, authenticator, email);

  const promptInput = page.locator('[data-prompt-input]');
  await expect(promptInput).toBeVisible();
  await expect(promptInput).toBeEnabled();
  await expect(promptInput).toHaveJSProperty('tagName', 'TEXTAREA');

  // Check placeholder text
  const placeholder = await promptInput.getAttribute('placeholder');
  expect(placeholder).toBe('Ask anything...');

  const initialBox = await promptInput.boundingBox();
  await promptInput.fill('This prompt is intentionally long enough to wrap across more than one visual line on the desktop prompt bar. '.repeat(6));
  await expect.poll(async () => (await promptInput.boundingBox())?.height || 0).toBeGreaterThan((initialBox?.height || 0) + 8);
  await promptInput.fill('');
  await expect.poll(async () => (await promptInput.boundingBox())?.height || 0).toBeLessThanOrEqual((initialBox?.height || 0) + 2);

  // Type text and submit with Enter
  await promptInput.fill('Hello world');
  await promptInput.press('Enter');

  // Input should be cleared after submit
  await expect(promptInput).toHaveValue('');
});

test('prompt bar routes normal input through conductor and opens vtext', async ({ page, authenticator }) => {
  const email = uniqueEmail();
  await registerAndLoadDesktop(page, authenticator, email);

  const prompt = 'Draft a project outline';
  const promptInput = page.locator('[data-prompt-input]');
  const initialVTextCount = await page.locator('[data-vtext-app]').count();
  const responsePromise = page.waitForResponse((response) =>
    response.url().includes('/api/prompt-bar') && response.request().method() === 'POST'
  );

  await promptInput.fill(prompt);
  await promptInput.press('Enter');

  const response = await responsePromise;
  expect(response.status()).toBe(202);
  const submitted = await response.json();

  const payload = response.request().postDataJSON();
  expect(payload).toEqual({ text: prompt });

  const decision = await waitForPromptSubmissionDecision(page, submitted.submission_id);
  expect(decision.action).toBe('open_app');
  expect(decision.app).toBe('vtext');

  const vtextWindow = page.locator('[data-vtext-app]').last();
  await expect(vtextWindow).toBeVisible({ timeout: 5000 });
  await expect(page.locator('[data-vtext-app]')).toHaveCount(initialVTextCount + 1);

  if (!decision.doc_id) {
    // The local stub provider can only prove the public prompt-bar route and
    // app launch path. Real/live conductor materialization includes the
    // durable doc/revision IDs asserted below.
    await expect(vtextWindow.locator('[data-vtext-editor-area]')).toContainText(/Draft a project outline/);
    return;
  }

  expect(decision.doc_id).toBeTruthy();
  expect(decision.user_revision_id).toBeTruthy();
  expect(decision.framing_revision_id).toBeTruthy();
  expect(decision.initial_revision_id).toBe(decision.framing_revision_id);
  expect(decision.initial_loop_id || '').toBeTruthy();

  const doc = await fetchJSON(page, `/api/vtext/documents/${encodeURIComponent(decision.doc_id)}`);
  expect(doc.current_revision_id).toBeTruthy();

  const revisionsResponse = await fetchJSON(page, `/api/vtext/documents/${encodeURIComponent(decision.doc_id)}/revisions`);
  const revisions = revisionsResponse.revisions;
  expect(revisions.length).toBeGreaterThanOrEqual(2);

  const userRevision = revisions.find((revision) => revision.revision_id === decision.user_revision_id);
  const framingRevision = revisions.find((revision) => revision.revision_id === decision.framing_revision_id);
  expect(userRevision).toBeTruthy();
  expect(framingRevision).toBeTruthy();
  expect(userRevision.author_kind).toBe('user');
  expect(userRevision.content).toBe(prompt);
  expect(userRevision.metadata.source).toBe('user_prompt');
  expect(userRevision.metadata.vtext_version).toBe('v0');

  expect(framingRevision.author_kind).toBe('appagent');
  expect(framingRevision.author_label).toBe('conductor');
  expect(framingRevision.parent_revision_id).toBe(userRevision.revision_id);
  expect(framingRevision.content).toContain(prompt);
  expect(framingRevision.content).not.toContain('Conductor framing');
  expect(framingRevision.content).not.toContain('Use this vtext');
  expect(framingRevision.content).not.toContain('User request:');
  expect(framingRevision.content).not.toContain('Current requirements:');
  expect(framingRevision.content).not.toContain('Grounding status:');
  expect(framingRevision.metadata.source).toBe('initial_vtext_seed');
  expect(framingRevision.metadata.vtext_version).toBe('v1');
  expect(framingRevision.metadata.user_revision_id).toBe(userRevision.revision_id);

  const trace = await fetchJSON(page, `/api/trace/trajectories/${encodeURIComponent(submitted.submission_id)}`);
  expect((trace.agents || []).some((agent) => agent.profile === 'vtext' && agent.agent_id === `vtext:${decision.doc_id}`)).toBe(true);

  await expect(vtextWindow.locator('[data-vtext-editor-area]')).toContainText(/Draft a project outline/);
  await expect(vtextWindow.locator('[data-vtext-editor-area]')).not.toContainText(/Conductor framing|Use this vtext|User request:|Current requirements:|Grounding status:/);
  await expect(vtextWindow.locator('[data-vtext-version]')).toHaveText(/^v[1-9][0-9]*$/);
  await expect(vtextWindow.locator('[data-vtext-prev]')).toBeEnabled();
  await expect(vtextWindow.locator('[data-vtext-next]')).toBeDisabled();
});

test('prompt-created vtext gets a .vtext shortcut and keeps state canonical in vtext', async ({ page, authenticator }) => {
  const email = uniqueEmail();
  await registerAndLoadDesktop(page, authenticator, email);

  const promptInput = page.locator('[data-prompt-input]');
  await promptInput.fill('Ahaha');
  await promptInput.press('Enter');

  const vtextWindow = page.locator('[data-vtext-app]').last();
  await expect(vtextWindow).toBeVisible({ timeout: 5000 });
  const editor = vtextWindow.locator('[data-vtext-editor-area]');
  await expect(editor).toContainText(/Ahaha/);
  await expect(editor).not.toContainText(/Conductor framing|Use this vtext|User request:/);

  const manifestDocId = await vtextWindow.locator('[data-vtext-editor]').getAttribute('data-vtext-doc-id');
  expect(manifestDocId).toBeTruthy();

  const fileNameHandle = await page.waitForFunction(async (docId) => {
    const res = await fetch('/api/files', { credentials: 'include' });
    if (!res.ok) return null;
    const entries = await res.json();
    if (!Array.isArray(entries)) return null;
    for (const entry of entries) {
      if (!entry?.name?.endsWith('.vtext')) continue;
      const fileRes = await fetch('/api/files/' + encodeURIComponent(entry.name), { credentials: 'include' });
      if (!fileRes.ok) continue;
      const text = await fileRes.text();
      try {
        const shortcut = JSON.parse(text);
        if (shortcut?.kind === 'vtext' && shortcut?.doc_id === docId) return entry.name;
      } catch {
        // Keep scanning; malformed files are not the shortcut for this doc.
      }
    }
    return null;
  }, manifestDocId, { timeout: 10000 });
  const fileName = await fileNameHandle.jsonValue();
  expect(fileName.endsWith('.vtext')).toBe(true);

  const filePath = '/api/files/' + encodeURIComponent(fileName);

  const shortcutBefore = await page.evaluate(async (path) => {
    const res = await fetch(path, { credentials: 'include' });
    return res.text();
  }, filePath);
  const parsedBefore = JSON.parse(shortcutBefore);
  expect(parsedBefore.kind).toBe('vtext');
  expect(parsedBefore.doc_id).toBe(manifestDocId);

  const revisedContent = 'Ahaha with a real file alias';
  await editor.fill(revisedContent);
  await vtextWindow.locator('[data-vtext-prompt]').click();

  const shortcutAfter = await page.evaluate(async (path) => {
    const res = await fetch(path, { credentials: 'include' });
    return res.text();
  }, filePath);
  const parsedAfter = JSON.parse(shortcutAfter);
  expect(parsedAfter.kind).toBe('vtext');
  expect(parsedAfter.doc_id).toBe(manifestDocId);

  await openAppViaIcon(page, 'files');
  const fileItem = page.locator('[data-file-item]').filter({ hasText: fileName }).first();
  await expect(fileItem).toBeVisible({ timeout: 5000 });

  const openResponsePromise = page.waitForResponse((response) => {
    const url = new URL(response.url());
    return response.request().method() === 'POST' && url.pathname === '/api/vtext/files/open';
  });
  await fileItem.click();
  const openResponse = await openResponsePromise;
  const openJSON = await openResponse.json();
  expect(openJSON.doc_id).toBe(manifestDocId);
});

test('prompt bar sends greetings through conductor instead of frontend pattern matching', async ({ page, authenticator }) => {
  const email = uniqueEmail();
  await registerAndLoadDesktop(page, authenticator, email);

  const promptInput = page.locator('[data-prompt-input]');
  const responsePromise = page.waitForResponse((response) =>
    response.url().includes('/api/prompt-bar') && response.request().method() === 'POST'
  );
  await promptInput.fill('hi');
  await promptInput.press('Enter');

  const response = await responsePromise;
  expect(response.status()).toBe(202);

  const payload = response.request().postDataJSON();
  expect(payload).toEqual({ text: 'hi' });

  const vtextWindow = page.locator('[data-vtext-app]').last();
  await expect(vtextWindow).toBeVisible({ timeout: 5000 });
  await expect(vtextWindow.locator('[data-vtext-editor-area]')).toContainText(/hi/);
  await expect(vtextWindow.locator('[data-vtext-editor-area]')).not.toContainText(/Conductor framing|Use this vtext|User request:/);
});

// ---------------------------------------------------------------
// Test: minimized window indicators in prompt surface (VAL-SHELL-008)
// ---------------------------------------------------------------
test('minimized window indicators in prompt surface', async ({ page, authenticator }) => {
  const email = uniqueEmail();
  await registerAndLoadDesktop(page, authenticator, email);

  // Open Files window
  await openAppViaIcon(page, 'files');
  const windowEl = page.locator('[data-window]').first();
  await expect(windowEl).toBeVisible({ timeout: 5000 });

  // Minimize it
  await windowEl.locator('[data-window-minimize]').click();
  await page.waitForTimeout(200);

  // Window should be hidden
  await expect(windowEl).not.toBeVisible();

  // A minimized indicator should appear in prompt surface
  const indicator = page.locator('[data-window-tray-item]');
  await expect(indicator).toHaveCount(1);
  await expect(indicator.first()).toBeVisible();

  // Click the indicator to restore
  await indicator.first().click();
  await page.waitForTimeout(200);

  // Window should be visible again
  await expect(windowEl).toBeVisible();
  // Indicator should be gone
  await expect(page.locator('[data-window-tray-item]')).toHaveCount(0);
});

test('prompt surface switches all open windows and exits show-desktop state', async ({ page, authenticator }) => {
  const email = uniqueEmail();
  await registerAndLoadDesktop(page, authenticator, email);

  await openAppViaIcon(page, 'files');
  await openAppViaIcon(page, 'settings');

  const windows = page.locator('[data-window]');
  await expect(windows).toHaveCount(2);
  await expect(page.locator('[data-window-switcher] [data-window-indicator]')).toHaveCount(2);

  const filesWindow = windows.filter({ has: page.locator('[data-files-app]') }).first();
  const settingsWindow = windows.filter({ has: page.locator('[data-settings-window]') }).first();

  await settingsWindow.locator('[data-window-minimize]').click();
  await expect(settingsWindow).not.toBeVisible();
  await expect(page.locator('[data-window-tray-item]')).toHaveCount(1);

  const settingsSwitch = page.locator('[data-window-switcher] [data-window-indicator]').filter({ hasText: 'Settings' }).first();
  await settingsSwitch.click();
  await expect(settingsWindow).toBeVisible();
  await expect(page.locator('[data-window-tray-item]')).toHaveCount(0);

  await page.locator('[data-show-desktop-btn]').click();
  await page.locator('[data-start-show-desktop]').click();
  await expect(filesWindow).not.toBeVisible();
  await expect(settingsWindow).not.toBeVisible();
  await expect(page.locator('[data-window-tray-item]')).toHaveCount(2);

  const filesSwitch = page.locator('[data-window-switcher] [data-window-indicator]').filter({ hasText: 'Files' }).first();
  await filesSwitch.click();
  await expect(filesWindow).toBeVisible();

  await page.locator('[data-show-desktop-btn]').click();
  await page.locator('[data-start-show-desktop]').click();
  await expect(filesWindow).not.toBeVisible();
});

// ---------------------------------------------------------------
// Test: user info and logout in desktop/account menu (VAL-SHELL-009)
// ---------------------------------------------------------------
test('user info and logout in desktop menu', async ({ page, authenticator }) => {
  const email = uniqueEmail();
  await registerAndLoadDesktop(page, authenticator, email);

  await expect(page.locator('[data-bottom-logout]')).toHaveCount(0);
  await page.locator('[data-show-desktop-btn]').click();

  const menu = page.locator('[data-desktop-menu]');
  await expect(menu).toBeVisible();

  // User info should show email
  const userInfo = page.locator('[data-bottom-user]');
  await expect(userInfo).toBeVisible();
  await expect(userInfo).toContainText(email);

  // Logout button should be present in the menu.
  const logoutBtn = page.locator('[data-bottom-logout]');
  await expect(logoutBtn).toBeVisible();

  // Click logout
  await logoutBtn.click();

  // Should return to public desktop, with auth available from the menu.
  await expect(page.locator('[data-desktop]')).toBeVisible();
  await expect(page.locator('[data-auth-entry]')).toHaveCount(0);
  await page.locator('[data-show-desktop-btn]').click();
  await expect(page.locator('[data-shell-login]')).toBeVisible();
  await expect(page.locator('[data-bottom-logout]')).toHaveCount(0);
});

test('logout remains reachable when desktop bootstrap fails', async ({ page, authenticator }) => {
  const email = uniqueEmail();
  await registerAndLoadDesktop(page, authenticator, email);

  await page.route('**/api/shell/bootstrap', async (route) => {
    await route.fulfill({
      status: 502,
      contentType: 'application/json',
      body: JSON.stringify({ error: 'stale desktop route' }),
    });
  });
  await page.reload();

  await expect(page.locator('[data-desktop]')).not.toBeVisible();
  await expect(page.locator('[data-boot-console]')).toBeVisible();
  await expect(page.locator('[data-boot-line]').first()).toContainText(/Powering|Resolving|returned 502/);
  await expect(page.locator('[data-prompt-surface]')).toBeVisible();
  await page.locator('[data-start-button]').click();
  await expect(page.locator('[data-shell-logout]')).toBeVisible();

  await page.locator('[data-shell-logout]').click();
  await expect(page.locator('[data-desktop]')).toBeVisible();
  await expect(page.locator('[data-auth-entry]')).toHaveCount(0);
});

test('Settings opens as safe product settings without prompt APIs', async ({ page, authenticator }) => {
  const email = uniqueEmail();
  await registerAndLoadDesktop(page, authenticator, email);

  const forbiddenRequests = [];
  page.on('request', (req) => {
    const url = new URL(req.url());
    if (url.pathname.startsWith('/api/prompts')) {
      forbiddenRequests.push(url.pathname);
    }
  });

  await openAppViaIcon(page, 'settings');
  const settings = page.locator('[data-settings-app]');
  await expect(settings).toBeVisible({ timeout: 5000 });
  await expect(settings.locator('[data-settings-account]')).toContainText(email);
  await expect(settings.locator('[data-settings-runtime-status]')).toBeVisible();
  await expect(settings.locator('[data-settings-theme-validation]')).toContainText('valid config');
  await expect(settings).not.toContainText('Editable role prompt');
  await page.waitForTimeout(300);
  expect(forbiddenRequests).toHaveLength(0);
});

// ---------------------------------------------------------------
// Test: live connection status dot (VAL-SHELL-010)
// ---------------------------------------------------------------
test('live connection status dot in prompt surface', async ({ page, authenticator }) => {
  const email = uniqueEmail();
  await registerAndLoadDesktop(page, authenticator, email);

  const statusEl = page.locator('[data-connection-status]');
  await expect(statusEl).toBeVisible();

  // Should have a status dot inside
  const dot = statusEl.locator('.status-dot');
  await expect(dot).toBeVisible();

  // Check it has aria-live for accessibility
  const ariaLive = await statusEl.getAttribute('aria-live');
  expect(ariaLive).toBe('polite');
});

// ---------------------------------------------------------------
// Test: no bootstrap accordion or runtime panel (VAL-SHELL-024)
// ---------------------------------------------------------------
test('no bootstrap accordion or runtime panel', async ({ page, authenticator }) => {
  const email = uniqueEmail();
  await registerAndLoadDesktop(page, authenticator, email);

  // No bootstrap element should be present
  await expect(page.locator('[data-shell-bootstrap]')).toHaveCount(0);

  // No task runner should be visible
  await expect(page.locator('[data-task-runner]')).toHaveCount(0);

  // No launcher toggle should be present
  await expect(page.locator('[data-launcher-toggle]')).toHaveCount(0);
});

// ---------------------------------------------------------------
// Test: floating window close removes from DOM (VAL-SHELL-012)
// ---------------------------------------------------------------
test('floating window close removes from DOM', async ({ page, authenticator }) => {
  const email = uniqueEmail();
  await registerAndLoadDesktop(page, authenticator, email);

  // Open Files window
  await openAppViaIcon(page, 'files');
  await page.locator('[data-window]').first().waitFor({ state: 'visible', timeout: 5000 });

  // Close it
  await page.locator('[data-window-close]').first().click();

  // Window should be removed
  await expect(page.locator('[data-window]')).toHaveCount(0);
});

// ---------------------------------------------------------------
// Test: floating window minimize and restore (VAL-SHELL-013)
// ---------------------------------------------------------------
test('floating window minimize hides and shows indicator', async ({ page, authenticator }) => {
  const email = uniqueEmail();
  await registerAndLoadDesktop(page, authenticator, email);

  // Open Files window
  await openAppViaIcon(page, 'files');
  await page.locator('[data-window]').first().waitFor({ state: 'visible', timeout: 5000 });

  // Minimize
  await page.locator('[data-window-minimize]').first().click();
  await page.waitForTimeout(200);

  // Window hidden (still in DOM but display:none), indicator shown
  await expect(page.locator('[data-window]').first()).not.toBeVisible();
  await expect(page.locator('[data-window-tray-item]')).toHaveCount(1);
});

// ---------------------------------------------------------------
// Test: floating window maximize and restore (VAL-SHELL-014)
// ---------------------------------------------------------------
test('floating window maximize fills desktop area', async ({ page, authenticator }) => {
  const email = uniqueEmail();
  await registerAndLoadDesktop(page, authenticator, email);

  // Open Files window
  await openAppViaIcon(page, 'files');
  const windowEl = page.locator('[data-window]').first();
  await expect(windowEl).toBeVisible({ timeout: 5000 });

  // Maximize
  await page.locator('[data-window-maximize]').first().click();
  await page.waitForTimeout(200);

  // Window should still be visible
  await expect(windowEl).toBeVisible();

  // Maximize button should now show restore icon
  const maxBtn = page.locator('[data-window-maximize]').first();
  const btnText = await maxBtn.textContent();
  expect(btnText).toContain('❐');

  // Click again to restore
  await maxBtn.click();
  await page.waitForTimeout(200);

  // Restore icon should change back
  const restoredText = await maxBtn.textContent();
  expect(restoredText).toContain('☐');
});

// ---------------------------------------------------------------
// Test: aria labels on desktop icons and window controls (VAL-SHELL-031)
// ---------------------------------------------------------------
test('aria labels on desktop icons and window controls', async ({ page, authenticator }) => {
  const email = uniqueEmail();
  await registerAndLoadDesktop(page, authenticator, email);

  // Check desktop icons have aria-labels
  const filesIcon = page.locator('[data-desktop-icon-id="files"]');
  const filesAria = await filesIcon.getAttribute('aria-label');
  expect(filesAria).toBe('Files');

  // Open a window and check its controls have aria-labels
  await filesIcon.dblclick();
  await page.locator('[data-window]').first().waitFor({ state: 'visible', timeout: 5000 });

  const closeBtn = page.locator('[data-window-close]').first();
  const closeAria = await closeBtn.getAttribute('aria-label');
  expect(closeAria).toBe('Close');

  const minBtn = page.locator('[data-window-minimize]').first();
  const minAria = await minBtn.getAttribute('aria-label');
  expect(minAria).toBe('Minimize');

  const maxBtn = page.locator('[data-window-maximize]').first();
  const maxAria = await maxBtn.getAttribute('aria-label');
  expect(maxAria).toBe('Maximize');
});
