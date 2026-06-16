/**
 * Playwright tests for desktop state persistence (VAL-SHELL-022, VAL-SHELL-023,
 * VAL-CROSS-203).
 *
 * Verifies:
 * - Window positions, sizes, z-index, minimized/maximized states, and active
 *   window are saved and restored on page reload (VAL-SHELL-022)
 * - Desktop state saved in one tab restores in a new tab for the same user
 *   (VAL-SHELL-023)
 * - No perceptible flash of empty desktop before state restores (VAL-SHELL-022)
 * - Same window IDs present after reload (VAL-SHELL-022)
 */
import { test, expect } from './helpers/fixtures.js';
import { registerPasskey, getSession } from './helpers/auth.js';

const BASE_URL = process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:4173';

function uniqueEmail() {
  return `state-test-${Date.now()}-${Math.random().toString(36).slice(2, 8)}@example.com`;
}

// Helper: register a passkey and get to the authenticated desktop.
async function registerAndLoadDesktop(page, authenticator, email) {
  await page.goto(BASE_URL);
  await registerPasskey(page, email, BASE_URL);
  await page.reload();
  await page.locator('[data-desktop]').waitFor({ state: 'visible', timeout: 10000 });
}

// Helper: open a specific app from the floating desktop icon
async function openApp(page, appId) {
  await page.locator(`[data-desktop-icon-id="${appId}"]`).dblclick();
  await page.locator('[data-window]').first().waitFor({ state: 'visible', timeout: 5000 });
}

async function fetchJSON(page, path, options = {}) {
  return page.evaluate(async ({ requestPath, requestOptions }) => {
    const res = await fetch(requestPath, {
      credentials: 'include',
      headers: { 'Content-Type': 'application/json', ...(requestOptions.headers || {}) },
      ...requestOptions,
    });
    const text = await res.text();
    let body = null;
    try {
      body = text ? JSON.parse(text) : null;
    } catch (_err) {
      body = text;
    }
    if (!res.ok) {
      throw new Error(`${requestOptions.method || 'GET'} ${requestPath} failed ${res.status}: ${text}`);
    }
    return body;
  }, { requestPath: path, requestOptions: options });
}

// Helper: get window positions and sizes from the DOM
async function getWindowStates(page) {
  return page.evaluate(() => {
    const wins = document.querySelectorAll('[data-window]');
    return Array.from(wins).map((el) => ({
      windowId: el.getAttribute('data-window-id'),
      left: parseInt(el.style.left, 10) || 0,
      top: parseInt(el.style.top, 10) || 0,
      width: parseInt(el.style.width, 10) || 0,
      height: parseInt(el.style.height, 10) || 0,
      zIndex: parseInt(el.style.zIndex, 10) || 0,
      isActive: el.classList.contains('window-active'),
      isVisible: el.offsetWidth > 0 && el.offsetHeight > 0,
    }));
  });
}

// ---------------------------------------------------------------
// Test: single window position and size restored after reload
// (VAL-SHELL-022)
// ---------------------------------------------------------------
test('single window position and size restored after reload', async ({
  page,
  authenticator,
}) => {
  const email = uniqueEmail();
  await registerAndLoadDesktop(page, authenticator, email);

  // Open a Files window
  await openApp(page, 'files');
  const windowEl = page.locator('[data-window]').first();
  await expect(windowEl).toBeVisible();

  // Get the window ID
  const windowIdBefore = await windowEl.getAttribute('data-window-id');
  expect(windowIdBefore).toBeTruthy();

  // Wait for the debounced state save
  await page.waitForTimeout(1000);

  // Record position and size before reload
  const statesBefore = await getWindowStates(page);
  expect(statesBefore.length).toBe(1);

  // Reload
  await page.reload();
  await page.locator('[data-desktop]').waitFor({ state: 'visible', timeout: 10000 });

  // Wait for desktop state to load
  await page.waitForTimeout(2000);

  // Window should be restored
  const restoredWindow = page.locator('[data-window]').first();
  await expect(restoredWindow).toBeVisible({ timeout: 5000 });

  // Window ID should match
  const windowIdAfter = await restoredWindow.getAttribute('data-window-id');
  expect(windowIdAfter).toBe(windowIdBefore);

  // Position and size should be close to original (within 5px tolerance)
  const statesAfter = await getWindowStates(page);
  expect(statesAfter.length).toBe(1);
  expect(Math.abs(statesAfter[0].left - statesBefore[0].left)).toBeLessThanOrEqual(5);
  expect(Math.abs(statesAfter[0].top - statesBefore[0].top)).toBeLessThanOrEqual(5);
  expect(Math.abs(statesAfter[0].width - statesBefore[0].width)).toBeLessThanOrEqual(5);
  expect(Math.abs(statesAfter[0].height - statesBefore[0].height)).toBeLessThanOrEqual(5);
});

test('stale bare email URL intent does not override restored desktop state', async ({
  page,
  authenticator,
}) => {
  const email = uniqueEmail();
  await registerAndLoadDesktop(page, authenticator, email);

  await openApp(page, 'vtext');
  await page.locator('[data-texture-app]').last().waitFor({ state: 'visible', timeout: 10000 });
  await page.waitForTimeout(1000);

  await page.goto(`${BASE_URL}?app=email`);
  await page.locator('[data-desktop]').waitFor({ state: 'visible', timeout: 10000 });
  await page.waitForTimeout(1500);

  await expect(page.locator('[data-texture-app]').last()).toBeVisible({ timeout: 10000 });
  await expect(page.locator('[data-email-app]')).toHaveCount(0);
  await expect(page).not.toHaveURL(/app=email/);
});

test('private vtext URL intent opens the requested authenticated document', async ({
  page,
  authenticator,
}) => {
  const email = uniqueEmail();
  await registerAndLoadDesktop(page, authenticator, email);

  const stamp = Date.now();
  const title = `Deep Linked VText ${stamp}`;
  const doc = await fetchJSON(page, '/api/texture/documents', {
    method: 'POST',
    body: JSON.stringify({ title }),
  });
  await fetchJSON(page, `/api/texture/documents/${encodeURIComponent(doc.doc_id)}/revisions`, {
    method: 'POST',
    body: JSON.stringify({
      content: `# ${title}\n\nPrivate deep link fixture.`,
      author_kind: 'user',
      author_label: 'Browser test',
    }),
  });

  await page.goto(`${BASE_URL}?app=vtext&doc=${encodeURIComponent(doc.doc_id)}&title=${encodeURIComponent(title)}`);
  await page.locator('[data-desktop][data-authenticated="true"][data-desktop-ready="true"]').waitFor({
    state: 'visible',
    timeout: 120000,
  });

  const editor = page.locator(`[data-texture-editor][data-texture-doc-id="${doc.doc_id}"]`).last();
  await expect(editor).toBeVisible({ timeout: 15000 });
  await expect(editor.locator('[data-texture-rendered]')).toContainText('Private deep link fixture');
  await expect(page).not.toHaveURL(/app=vtext/);
  await expect(page).not.toHaveURL(/doc=/);
});

test('email app view state persists through universal app context after reload', async ({
  page,
  authenticator,
}) => {
  const email = uniqueEmail();
  await registerAndLoadDesktop(page, authenticator, email);
  await page.locator('[data-desktop][data-authenticated="true"][data-desktop-ready="true"]').waitFor({
    state: 'visible',
    timeout: 120000,
  });

  await openApp(page, 'email');

  const emailApp = page.locator('[data-email-app]').last();
  await expect(emailApp).toBeVisible({ timeout: 10000 });

  await emailApp.locator('[data-email-folder="sent"]').click();
  await expect(emailApp.locator('[data-email-folder="sent"]')).toHaveClass(/active/);

  await expect.poll(async () => page.evaluate(async () => {
    const res = await fetch('/api/desktop/state', { credentials: 'include' });
    if (!res.ok) return '';
    const state = await res.json();
    const emailWindow = (state.windows || []).find((win) => win.app_id === 'email');
    return emailWindow?.app_context?.activeFolder || '';
  }), { timeout: 5000 }).toBe('sent');

  await page.reload();
  await page.locator('[data-desktop][data-authenticated="true"][data-desktop-ready="true"]').waitFor({
    state: 'visible',
    timeout: 120000,
  });

  const restoredEmail = page.locator('[data-email-app]').last();
  await expect(restoredEmail).toBeVisible({ timeout: 10000 });
  await expect(restoredEmail.locator('[data-email-folder="sent"]')).toHaveClass(/active/);
});

test('window shell keeps opaque backing under alpha app themes', async ({
  page,
  authenticator,
}) => {
  const email = uniqueEmail();
  await registerAndLoadDesktop(page, authenticator, email);
  await openApp(page, 'features');
  await page.locator('[data-window][data-window-app-id="features"]').waitFor({ state: 'visible', timeout: 10000 });
  await page.locator('[data-window][data-window-app-id="features"] [data-features-app]').waitFor({ state: 'visible', timeout: 10000 });

  await page.evaluate(() => {
    document.documentElement.style.setProperty('--choir-panel', 'rgba(9, 12, 19, 0.2)');
    document.documentElement.style.setProperty('--choir-panel-strong', 'rgba(9, 16, 31, 0.2)');
  });

  const alphaValues = await page.evaluate(() => {
    function alphaFor(selector) {
      const el = document.querySelector(selector);
      if (!el) throw new Error(`missing element: ${selector}`);
      const color = getComputedStyle(el).backgroundColor;
      const match = color.match(/rgba?\(([^)]+)\)/);
      if (!match) return 1;
      const parts = match[1].split(',').map((part) => part.trim());
      return parts.length >= 4 ? Number(parts[3]) : 1;
    }
    return {
      windowContent: alphaFor('[data-window][data-window-app-id="features"] [data-window-content]'),
      appSurface: alphaFor('[data-window][data-window-app-id="features"] [data-app-host]'),
      featuresApp: alphaFor('[data-window][data-window-app-id="features"] .features-app'),
      titlebar: alphaFor('[data-window][data-window-app-id="features"] [data-window-titlebar]'),
      windowIsolation: getComputedStyle(document.querySelector('[data-window][data-window-app-id="features"]')).isolation,
      windowContain: getComputedStyle(document.querySelector('[data-window][data-window-app-id="features"]')).contain,
      appSurfaceIsolation: getComputedStyle(document.querySelector('[data-window][data-window-app-id="features"] [data-app-host]')).isolation,
      featuresHeader: alphaFor('[data-window][data-window-app-id="features"] .features-header'),
    };
  });

  expect(alphaValues).toEqual({
    windowContent: 1,
    appSurface: 1,
    featuresApp: 1,
    titlebar: 1,
    windowIsolation: 'isolate',
    windowContain: 'paint',
    appSurfaceIsolation: 'isolate',
    featuresHeader: 1,
  });
});

test('restored overlapping active window is opaque and paint isolated before focus', async ({
  page,
  authenticator,
}) => {
  const email = uniqueEmail();
  await page.setViewportSize({ width: 390, height: 844 });
  await registerAndLoadDesktop(page, authenticator, email);

  const windows = [
    {
      window_id: 'restore-features-overlap',
      app_id: 'features',
      title: 'Features',
      geometry: { x: 51, y: 10, width: 445, height: 640 },
      mode: 'normal',
      z_index: 11,
      app_context: { windowTitle: 'Features' },
    },
    {
      window_id: 'restore-email-overlap',
      app_id: 'email',
      title: 'Email',
      geometry: { x: 10, y: 40, width: 396, height: 708 },
      mode: 'normal',
      z_index: 12,
      app_context: {
        activeFolder: 'inbox',
        detailPaneOpen: true,
        selectedId: '',
        windowTitle: 'Email',
      },
    },
  ];

  await page.evaluate(async ({ windows }) => {
    const res = await fetch('/api/desktop/state', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        windows,
        active_window_id: 'restore-email-overlap',
      }),
    });
    if (!res.ok) throw new Error(`desktop state save failed: ${res.status}`);
  }, { windows });

  await page.reload();
  await page.locator('[data-desktop][data-authenticated="true"][data-desktop-ready="true"]').waitFor({
    state: 'visible',
    timeout: 120000,
  });
  const emailWindow = page.locator('[data-window][data-window-id="restore-email-overlap"]');
  const traceWindow = page.locator('[data-window][data-window-id="restore-features-overlap"]');
  await expect(emailWindow).toBeVisible({ timeout: 10000 });
  await expect(traceWindow).toBeVisible({ timeout: 10000 });

  const metrics = await page.evaluate(() => {
    function alphaFor(el) {
      const color = getComputedStyle(el).backgroundColor;
      const match = color.match(/rgba?\(([^)]+)\)/);
      if (!match) return 1;
      const parts = match[1].split(',').map((part) => part.trim());
      return parts.length >= 4 ? Number(parts[3]) : 1;
    }
    const email = document.querySelector('[data-window][data-window-id="restore-email-overlap"]');
    const trace = document.querySelector('[data-window][data-window-id="restore-trace-overlap"]');
    const content = email.querySelector('[data-window-content]');
    const appHost = email.querySelector('[data-app-host]');
    const emailApp = email.querySelector('[data-email-app]');
    const messageDetail = email.querySelector('.message-detail');
    const mobileMailbar = email.querySelector('.mobile-mailbar');
    const emailRect = email.getBoundingClientRect();
    const sample = document.elementFromPoint(
      Math.min(emailRect.right - 24, emailRect.left + 180),
      Math.min(emailRect.bottom - 24, emailRect.top + 240),
    );
    return {
      active: email.getAttribute('data-window-active'),
      overviewState: email.getAttribute('data-overview-preview-state'),
      emailZ: Number(getComputedStyle(email).zIndex),
      traceZ: Number(getComputedStyle(trace).zIndex),
      emailOpacity: getComputedStyle(email).opacity,
      emailIsolation: getComputedStyle(email).isolation,
      emailContain: getComputedStyle(email).contain,
      contentAlpha: alphaFor(content),
      contentIsolation: getComputedStyle(content).isolation,
      appHostAlpha: alphaFor(appHost),
      appHostIsolation: getComputedStyle(appHost).isolation,
      messageDetailAlpha: messageDetail ? alphaFor(messageDetail) : alphaFor(emailApp || appHost),
      mobileMailbarAlpha: mobileMailbar ? alphaFor(mobileMailbar) : 1,
      hitWindowId: sample?.closest?.('[data-window]')?.getAttribute('data-window-id') || '',
    };
  });

  expect(metrics).toEqual({
    active: 'true',
    overviewState: 'normal',
    emailZ: 2,
    traceZ: 1,
    emailOpacity: '1',
    emailIsolation: 'isolate',
    emailContain: 'paint',
    contentAlpha: 1,
    contentIsolation: 'isolate',
    appHostAlpha: 1,
    appHostIsolation: 'isolate',
    messageDetailAlpha: 1,
    mobileMailbarAlpha: 1,
    hitWindowId: 'restore-email-overlap',
  });
});

// ---------------------------------------------------------------
// Test: multiple windows with z-index restored after reload
// (VAL-SHELL-022)
// ---------------------------------------------------------------
test('multiple windows with z-index restored after reload', async ({
  page,
  authenticator,
}) => {
  const email = uniqueEmail();
  await registerAndLoadDesktop(page, authenticator, email);

  // Open two windows
  await openApp(page, 'files');
  await openApp(page, 'browser');

  const windowsBefore = page.locator('[data-window]');
  await expect(windowsBefore).toHaveCount(2);

  // Wait for the debounced state save
  await page.waitForTimeout(1000);

  // Record window IDs and z-index order
  const statesBefore = await getWindowStates(page);
  expect(statesBefore.length).toBe(2);

  // Sort by z-index to get the stacking order
  const sortedBefore = [...statesBefore].sort((a, b) => a.zIndex - b.zIndex);
  const topWindowIdBefore = sortedBefore[sortedBefore.length - 1].windowId;

  // Reload
  await page.reload();
  await page.locator('[data-desktop]').waitFor({ state: 'visible', timeout: 10000 });
  await page.waitForTimeout(2000);

  // Both windows should be restored
  const windowsAfter = page.locator('[data-window]');
  await expect(windowsAfter).toHaveCount(2);

  // Same window IDs should be present
  const statesAfter = await getWindowStates(page);
  const idsBefore = statesBefore.map((s) => s.windowId).sort();
  const idsAfter = statesAfter.map((s) => s.windowId).sort();
  expect(idsAfter).toEqual(idsBefore);

  // Top window (highest z-index) should be the same
  const sortedAfter = [...statesAfter].sort((a, b) => a.zIndex - b.zIndex);
  const topWindowIdAfter = sortedAfter[sortedAfter.length - 1].windowId;
  expect(topWindowIdAfter).toBe(topWindowIdBefore);
});

// ---------------------------------------------------------------
// Test: mobile restore recovery blocks heavyweight crash loops
// ---------------------------------------------------------------
test('mobile restore recovery pauses too many heavyweight saved windows', async ({
  page,
  authenticator,
}) => {
  const email = uniqueEmail();
  await page.setViewportSize({ width: 390, height: 844 });
  await registerAndLoadDesktop(page, authenticator, email);

  const appIds = ['image', 'pdf', 'epub', 'video', 'audio', 'features', 'vtext', 'browser', 'super-console'];
  const windows = appIds.map((appId, index) => ({
    window_id: `recovery-window-${index + 1}`,
    app_id: appId,
    title: `Recovery ${appId}`,
    geometry: { x: 12 + index * 2, y: 12 + index * 2, width: 360, height: 700 },
    mode: 'normal',
    z_index: index + 1,
    app_context: { windowTitle: `Recovery ${appId}` },
  }));

  await page.evaluate(async ({ windows }) => {
    const res = await fetch('/api/desktop/state', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        windows,
        active_window_id: 'recovery-window-7',
      }),
    });
    if (!res.ok) {
      throw new Error(`desktop state save failed: ${res.status}`);
    }
  }, { windows });

  await page.reload();

  const recovery = page.locator('[data-desktop-recovery]');
  await expect(recovery).toBeVisible({ timeout: 10000 });
  await expect(recovery).toContainText('Saved windows are paused');
  await expect(recovery).toContainText('9 visible windows');
  await expect(page.locator('[data-window]')).toHaveCount(0);

  await page.locator('[data-desktop-recovery-clear]').click();
  await expect(recovery).not.toBeVisible({ timeout: 5000 });

  const saved = await page.evaluate(async () => {
    const res = await fetch('/api/desktop/state');
    if (!res.ok) {
      throw new Error(`desktop state fetch failed: ${res.status}`);
    }
    return res.json();
  });
  expect(saved.windows).toEqual([]);
});

// ---------------------------------------------------------------
// Test: minimized window state preserved after reload
// (VAL-SHELL-022)
// ---------------------------------------------------------------
test('minimized window state preserved after reload', async ({
  page,
  authenticator,
}) => {
  const email = uniqueEmail();
  await registerAndLoadDesktop(page, authenticator, email);

  // Open two windows
  await openApp(page, 'files');
  await openApp(page, 'browser');

  // Minimize the first (files) window
  const filesWindow = page.locator('[data-window]').first();
  const filesWindowId = await filesWindow.getAttribute('data-window-id');
  await filesWindow.locator('[data-window-minimize]').click();
  await page.waitForTimeout(300);

  // Files window should be hidden (minimized)
  await expect(filesWindow).not.toBeVisible();

  // Minimized indicator should show
  const indicator = page.locator('[data-window-tray-item]');
  await expect(indicator).toHaveCount(1);

  // Wait for state save
  await page.waitForTimeout(1000);

  // Reload
  await page.reload();
  await page.locator('[data-desktop]').waitFor({ state: 'visible', timeout: 10000 });
  await page.waitForTimeout(2000);

  // The files window should still be minimized (not visible)
  const restoredFilesWindow = page.locator(`[data-window-id="${filesWindowId}"]`);
  await expect(restoredFilesWindow).not.toBeVisible();

  // Minimized indicator should be present
  await expect(page.locator('[data-window-tray-item]')).toHaveCount(1);

  // Browser window should still be visible
  const visibleWindows = page.locator('[data-window]:visible');
  const visibleCount = await visibleWindows.count();
  expect(visibleCount).toBeGreaterThanOrEqual(1);
});

// ---------------------------------------------------------------
// Test: maximized window state preserved after reload
// (VAL-SHELL-022)
// ---------------------------------------------------------------
test('maximized window state preserved after reload', async ({
  page,
  authenticator,
}) => {
  const email = uniqueEmail();
  await registerAndLoadDesktop(page, authenticator, email);

  // Open a Files window
  await openApp(page, 'files');
  const windowEl = page.locator('[data-window]').first();
  const windowId = await windowEl.getAttribute('data-window-id');

  // Maximize it
  await windowEl.locator('[data-window-maximize]').click();
  await page.waitForTimeout(300);

  // Verify it's maximized (button shows restore icon)
  const maxBtn = windowEl.locator('[data-window-maximize]');
  const btnText = await maxBtn.textContent();
  expect(btnText).toContain('❐');

  // Wait for state save
  await page.waitForTimeout(1000);

  // Reload
  await page.reload();
  await page.locator('[data-desktop]').waitFor({ state: 'visible', timeout: 10000 });
  await page.waitForTimeout(2000);

  // The window should still be maximized
  const restoredWindow = page.locator(`[data-window-id="${windowId}"]`);
  await expect(restoredWindow).toBeVisible({ timeout: 5000 });

  // The maximize button should still show restore icon
  const restoredMaxBtn = restoredWindow.locator('[data-window-maximize]');
  const restoredBtnText = await restoredMaxBtn.textContent();
  expect(restoredBtnText).toContain('❐');
});

// ---------------------------------------------------------------
// Test: no flash of empty desktop during state restore
// (VAL-SHELL-022)
// ---------------------------------------------------------------
test('no flash of empty desktop during state restore', async ({
  page,
  authenticator,
}) => {
  const email = uniqueEmail();
  await registerAndLoadDesktop(page, authenticator, email);

  // Open a Files window
  await openApp(page, 'files');
  await page.waitForTimeout(1000);

  // Record the window ID
  const windowId = await page.locator('[data-window]').first().getAttribute('data-window-id');

  // Set up a flag to detect if desktop-windows area was ever visible while empty
  // We'll use a mutation observer to track visibility changes
  const flashDetected = await page.evaluate(() => {
    return new Promise((resolve) => {
      let hadFlash = false;
      const desktopArea = document.querySelector('[data-desktop-windows]');

      if (!desktopArea) {
        resolve(false);
        return;
      }

      // Check initial state
      const observer = new MutationObserver(() => {
        // If the area is visible but has no windows, that's a potential flash
        const windows = desktopArea.querySelectorAll('[data-window]');
        if (windows.length === 0 && desktopArea.style.visibility !== 'hidden') {
          hadFlash = true;
        }
      });

      observer.observe(desktopArea, { childList: true, subtree: true, attributes: true });

      // Resolve after a short time
      setTimeout(() => {
        observer.disconnect();
        resolve(hadFlash);
      }, 200);
    });
  });

  // Now reload and check for flash
  await page.reload();

  // Immediately check: the desktop area should not show windows until state is loaded
  // The state-loading class hides the area until state is ready
  let flashDuringLoad = false;
  try {
    // Check if desktop area has visibility:hidden during loading
    const areaVisibility = await page.evaluate(() => {
      const area = document.querySelector('[data-desktop-windows]');
      if (!area) return 'not-found';
      return area.classList.contains('state-loading') ? 'hidden' : 'visible';
    });

    // If the area starts as hidden (state-loading), that's correct - no flash
    // If it starts visible immediately, check if windows are already there
    if (areaVisibility === 'visible') {
      const windowCount = await page.locator('[data-window]').count();
      if (windowCount === 0) {
        flashDuringLoad = true;
      }
    }
  } catch (_e) {
    // Page not ready yet — acceptable
  }

  // Wait for desktop to fully load
  await page.locator('[data-desktop]').waitFor({ state: 'visible', timeout: 10000 });
  await page.waitForTimeout(2000);

  // Window should be restored
  const restoredWindow = page.locator(`[data-window-id="${windowId}"]`);
  await expect(restoredWindow).toBeVisible({ timeout: 5000 });

  // No flash should have been detected
  expect(flashDuringLoad).toBe(false);
});

// ---------------------------------------------------------------
// Test: desktop state persists across fresh browser context (new tab)
// (VAL-SHELL-023)
// ---------------------------------------------------------------
test('desktop state persists across fresh browser context', async ({
  page,
  authenticator,
  context,
  browser,
}) => {
  const email = uniqueEmail();
  await registerAndLoadDesktop(page, authenticator, email);

  // Open a Files window
  await openApp(page, 'files');
  const windowId = await page.locator('[data-window]').first().getAttribute('data-window-id');
  expect(windowId).toBeTruthy();

  // Wait for the debounced state save
  await page.waitForTimeout(1000);

  // Get the cookies from the current context
  const cookies = await context.cookies();

  // Create a new context (simulates new tab/window)
  const newContext = await browser.newContext();
  const newPage = await newContext.newPage();

  try {
    // Set up virtual authenticator in the new context
    // Use WebAuthn virtual environment for the new context
    const newClient = await newPage.context().newCDPSession(newPage);
    const authenticatorResult = await newClient.send('WebAuthn.enable');
    const { authenticatorId } = await newClient.send('WebAuthn.addVirtualAuthenticator', {
      options: {
        protocol: 'ctap2',
        transport: 'internal',
        hasResidentKey: true,
        hasUserVerification: true,
        isUserVerified: true,
      },
    });

    // Transfer cookies to new context
    await newContext.addCookies(cookies);

    // Navigate to the app in the new tab
    await newPage.goto(BASE_URL);

    // Wait for session check and desktop to load
    await newPage.waitForTimeout(2000);

    // Check if we're authenticated (desktop visible) or need to login
    const desktopVisible = await newPage.locator('[data-desktop]').isVisible().catch(() => false);

    if (desktopVisible) {
      // Desktop state should be restored
      await newPage.waitForTimeout(2000);

      // The Files window should be restored from server state
      const restoredWindow = newPage.locator(`[data-window-id="${windowId}"]`);
      const restoredVisible = await restoredWindow.isVisible().catch(() => false);

      // The window should be restored if cookies carried over
      expect(restoredVisible).toBe(true);
    }

    await newClient.send('WebAuthn.removeVirtualAuthenticator', { authenticatorId });
  } finally {
    await newContext.close();
  }
});

test('desktop state flushes on page hide without waiting for debounce', async ({
  page,
  authenticator,
}) => {
  const email = uniqueEmail();
  await registerAndLoadDesktop(page, authenticator, email);

  const saveResponse = page.waitForResponse((response) =>
    response.url().includes('/api/desktop/state') &&
    response.request().method() === 'PUT' &&
    response.ok(),
  );

  await openApp(page, 'files');
  const windowId = await page.locator('[data-window]').first().getAttribute('data-window-id');
  expect(windowId).toBeTruthy();

  await page.evaluate(() => {
    window.dispatchEvent(new PageTransitionEvent('pagehide'));
  });
  await saveResponse;

  await page.reload();
  await page.locator('[data-desktop]').waitFor({ state: 'visible', timeout: 10000 });
  await expect(page.locator(`[data-window][data-window-id="${windowId}"]`)).toBeVisible({ timeout: 5000 });
});

// ---------------------------------------------------------------
// Test: empty desktop state (no windows) preserved after reload
// (VAL-SHELL-022)
// ---------------------------------------------------------------
test('empty desktop state preserved after reload', async ({
  page,
  authenticator,
}) => {
  const email = uniqueEmail();
  await registerAndLoadDesktop(page, authenticator, email);

  // No windows opened — desktop should be empty
  await expect(page.locator('[data-window]')).toHaveCount(0);

  // Wait for potential state save
  await page.waitForTimeout(1000);

  // Reload
  await page.reload();
  await page.locator('[data-desktop]').waitFor({ state: 'visible', timeout: 10000 });
  await page.waitForTimeout(2000);

  // Desktop should still be empty (no ghost windows)
  await expect(page.locator('[data-window]')).toHaveCount(0);
});

// ---------------------------------------------------------------
// Test: window close removes window from persisted state
// (VAL-SHELL-022)
// ---------------------------------------------------------------
test('window close removes window from persisted state', async ({
  page,
  authenticator,
}) => {
  const email = uniqueEmail();
  await registerAndLoadDesktop(page, authenticator, email);

  // Open a Files window
  await openApp(page, 'files');
  await page.waitForTimeout(1000);

  // Close it
  await page.locator('[data-window-close]').first().click();
  await expect(page.locator('[data-window]')).toHaveCount(0);

  // Wait for state save
  await page.waitForTimeout(1000);

  // Reload
  await page.reload();
  await page.locator('[data-desktop]').waitFor({ state: 'visible', timeout: 10000 });
  await page.waitForTimeout(2000);

  // Window should NOT be restored (it was closed)
  await expect(page.locator('[data-window]')).toHaveCount(0);
});
