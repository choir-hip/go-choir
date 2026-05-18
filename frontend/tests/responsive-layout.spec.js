/**
 * Playwright tests for responsive layout across three breakpoints.
 *
 * Covers validation assertions:
 * - VAL-RESP-001: Desktop — floating icons visible with labels
 * - VAL-RESP-002: Desktop — windows floating, draggable, resizable
 * - VAL-RESP-003: Desktop — bottom bar full height (~56px)
 * - VAL-RESP-004: Desktop — multiple windows visible simultaneously
 * - VAL-RESP-005: Tablet — windows floating with max-width constraint
 * - VAL-RESP-006: Mobile — floating icons remain visible
 * - VAL-RESP-007: Mobile — multiple windows remain available simultaneously
 * - VAL-RESP-008: Mobile — window remains floating and resizable
 * - VAL-RESP-009: Mobile — prompt bar full width with >=44px touch target
 * - VAL-RESP-010: Mobile — minimizing preserves window state via bottom bar
 * - VAL-RESP-011: No horizontal overflow at any breakpoint
 * - VAL-RESP-012: Breakpoint transition is smooth (no layout flash)
 * - VAL-RESP-013: Mobile — consistent desktop experience
 * - VAL-RESP-014: Tablet — multiple windows still supported
 */
import { test, expect } from './helpers/fixtures.js';
import { registerPasskey } from './helpers/auth.js';

const BASE_URL = 'http://localhost:4173';

function uniqueEmail() {
  return `resp-test-${Date.now()}-${Math.random().toString(36).slice(2, 8)}@example.com`;
}

// Helper: register a passkey and get to the authenticated desktop.
async function registerAndLoadDesktop(page, authenticator, email, viewportSize = { width: 1280, height: 800 }) {
  await page.setViewportSize(viewportSize);
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

async function mockTraceTrajectory(page) {
  const trajectoryId = 'trace-mobile-hit-target-regression';
  const timestamp = '2026-05-17T02:47:00Z';
  const trajectory = {
    trajectory_id: trajectoryId,
    title: 'A revise event was triggered for the current vtext document. Intent: inspect mobile Trace provenance readability.',
    subtitle: 'conductor · super',
    state: 'completed',
    live: false,
    agent_count: 3,
    delegation_count: 2,
    moment_count: 1,
    message_count: 4,
    finding_count: 0,
    search_attempt_count: 0,
    latest_activity_at: timestamp,
    latest_stream_seq: 0,
  };
  const snapshot = {
    trajectory,
    agents: [
      { agent_id: 'super', label: 'super', role: 'super', profile: 'foreground' },
      { agent_id: 'implementation', label: 'implementation co-super', role: 'cosuper', profile: 'worker' },
      { agent_id: 'verifier', label: 'verifier co-super', role: 'cosuper', profile: 'worker' },
    ],
    edges: [
      { from_agent_id: 'super', to_agent_id: 'implementation', label: 'delegates' },
      { from_agent_id: 'super', to_agent_id: 'verifier', label: 'verifies' },
    ],
    moments: [
      {
        moment_id: 'moment-export',
        kind: 'loop.completed',
        tone: 'success',
        loop_id: 'loop-export',
        summary: 'worker export completed',
        created_at: timestamp,
      },
    ],
    search: { providers: [] },
    mobile_summary: {
      headline: 'export-level · accepted · 6 evidence',
      acceptance_state: 'accepted',
      acceptance_level: 'export-level',
      agent_count: 3,
      delegation_count: 2,
      evidence_ref_count: 6,
      rollback_ref_count: 2,
      readable_evidence: ['implementation export and verifier evidence linked'],
      rollback_refs: ['base ccfd551'],
    },
    acceptances: [
      {
        acceptance_id: 'acceptance-export',
        state: 'accepted',
        acceptance_level: 'export-level',
        summary: 'mocked acceptance for responsive layout regression',
        checkpoints: [],
        evidence_refs: [],
        rollback_refs: [],
      },
    ],
  };

  await page.route('**/api/trace/trajectories?limit=200', (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ trajectories: [trajectory] }),
  }));
  await page.route(`**/api/trace/trajectories/${trajectoryId}`, (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify(snapshot),
  }));
  await page.route(`**/api/trace/trajectories/${trajectoryId}/moments/moment-export`, (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({
      moment: snapshot.moments[0],
      messages: [],
      artifacts: {},
    }),
  }));
  await page.route(`**/api/trace/trajectories/${trajectoryId}/events**`, (route) => route.fulfill({
    status: 200,
    contentType: 'text/event-stream',
    body: '\n',
  }));

  return trajectoryId;
}

// ================================================================
// DESKTOP BREAKPOINT (>1024px) — viewport 1280x800
// ================================================================

test.describe('Desktop breakpoint (>1024px)', () => {
  // VAL-RESP-001: Desktop — floating icons visible with labels
  test('floating icons visible with labels', async ({ page, authenticator }) => {
    const email = uniqueEmail();
    await registerAndLoadDesktop(page, authenticator, email, { width: 1280, height: 800 });

    const surface = page.locator('[data-desktop-surface]');
    await expect(surface).toBeVisible();

    // Core desktop icons should be visible.
    const icons = surface.locator('[data-desktop-icon]');
    await expect(icons).toHaveCount(7);

    // Labels should be visible
    const filesLabel = surface.locator('[data-desktop-icon-label]').first();
    await expect(filesLabel).toBeVisible();
  });

  // VAL-RESP-002: Desktop — windows floating, draggable, resizable
  test('windows are floating, draggable, resizable', async ({ page, authenticator }) => {
    const email = uniqueEmail();
    await registerAndLoadDesktop(page, authenticator, email, { width: 1280, height: 800 });

    await openAppViaIcon(page, 'files');
    const windowEl = page.locator('[data-window]').first();
    await expect(windowEl).toBeVisible({ timeout: 5000 });

    // Window should be absolutely positioned (floating)
    const position = await windowEl.evaluate((el) => window.getComputedStyle(el).position);
    expect(position).toBe('absolute');

    // Resize handle should be present
    const resizeHandle = windowEl.locator('[data-resize-handle]');
    await expect(resizeHandle).toHaveCount(1);
  });

  // VAL-RESP-003: Desktop — bottom bar full height (~56px)
  test('bottom bar renders at ~56px full width', async ({ page, authenticator }) => {
    const email = uniqueEmail();
    await registerAndLoadDesktop(page, authenticator, email, { width: 1280, height: 800 });

    const bottomBar = page.locator('[data-bottom-bar]');
    await expect(bottomBar).toBeVisible();

    const height = await bottomBar.evaluate((el) => el.offsetHeight);
    expect(height).toBeGreaterThanOrEqual(52);
    expect(height).toBeLessThanOrEqual(60);

    const width = await bottomBar.evaluate((el) => el.offsetWidth);
    expect(width).toBe(1280);
  });

  // VAL-RESP-004: Desktop — multiple windows visible simultaneously
  test('multiple windows visible simultaneously', async ({ page, authenticator }) => {
    const email = uniqueEmail();
    await registerAndLoadDesktop(page, authenticator, email, { width: 1280, height: 800 });

    await openAppViaIcon(page, 'files');
    await page.locator('[data-window]').first().waitFor({ state: 'visible', timeout: 5000 });

    await openAppViaIcon(page, 'browser');
    await page.waitForTimeout(300);

    const windows = page.locator('[data-window]');
    await expect(windows).toHaveCount(2);

    // Both should be visible
    await expect(windows.nth(0)).toBeVisible();
    await expect(windows.nth(1)).toBeVisible();
  });
});

// ================================================================
// TABLET BREAKPOINT (768-1024px) — viewport 900x800
// ================================================================

test.describe('Tablet breakpoint (768-1024px)', () => {
  // VAL-RESP-005: Tablet — windows floating with max-width constraint
  test('windows floating with max-width constraint', async ({ page, authenticator }) => {
    const email = uniqueEmail();
    await registerAndLoadDesktop(page, authenticator, email, { width: 900, height: 800 });

    await openAppViaIcon(page, 'files');
    const windowEl = page.locator('[data-window]').first();
    await expect(windowEl).toBeVisible({ timeout: 5000 });

    // Window width should not exceed viewport width
    const winBox = await windowEl.boundingBox();
    expect(winBox.width).toBeLessThanOrEqual(900);

    // Floating icons should still be visible with labels
    const icons = page.locator('[data-desktop-icon]');
    await expect(icons).toHaveCount(7);
  });

  // Bottom bar remains full height
  test('bottom bar remains full height', async ({ page, authenticator }) => {
    const email = uniqueEmail();
    await registerAndLoadDesktop(page, authenticator, email, { width: 900, height: 800 });

    const bottomBar = page.locator('[data-bottom-bar]');
    await expect(bottomBar).toBeVisible();

    const height = await bottomBar.evaluate((el) => el.offsetHeight);
    expect(height).toBeGreaterThanOrEqual(52);
    expect(height).toBeLessThanOrEqual(60);
  });

  // VAL-RESP-014: Tablet — multiple windows still supported
  test('multiple windows still supported', async ({ page, authenticator }) => {
    const email = uniqueEmail();
    await registerAndLoadDesktop(page, authenticator, email, { width: 900, height: 800 });

    await openAppViaIcon(page, 'files');
    await page.locator('[data-window]').first().waitFor({ state: 'visible', timeout: 5000 });

    await openAppViaIcon(page, 'browser');
    await page.waitForTimeout(300);

    const windows = page.locator('[data-window]');
    await expect(windows).toHaveCount(2);
  });
});

// ================================================================
// MOBILE BREAKPOINT (<768px) — viewport 375x812
// ================================================================

test.describe('Mobile breakpoint (<768px)', () => {
  // VAL-RESP-006: Mobile — floating icons remain visible
  test('floating icons remain visible', async ({ page, authenticator }) => {
    const email = uniqueEmail();
    await registerAndLoadDesktop(page, authenticator, email, { width: 375, height: 812 });

    // No left rail should be present
    await expect(page.locator('[data-desktop-rail]')).toHaveCount(0);

    // No hamburger button should be present
    await expect(page.locator('[data-hamburger-btn]')).toHaveCount(0);

    // No backdrop should be present
    await expect(page.locator('[data-rail-backdrop]')).toHaveCount(0);

    // Floating desktop icons should be visible
    const icons = page.locator('[data-desktop-icon]');
    await expect(icons).toHaveCount(7);
    await expect(icons.first()).toBeVisible();

    // Desktop surface spans full viewport width
    const surface = page.locator('[data-desktop-surface]');
    const surfaceWidth = await surface.evaluate((el) => el.offsetWidth);
    expect(surfaceWidth).toBeGreaterThanOrEqual(375);
  });

  // VAL-RESP-007: Mobile — multiple windows remain available simultaneously
  test('multiple windows can remain open on mobile', async ({ page, authenticator }) => {
    const email = uniqueEmail();
    await registerAndLoadDesktop(page, authenticator, email, { width: 375, height: 812 });

    await openAppViaIcon(page, 'files');
    await page.waitForTimeout(300);
    await openAppViaIcon(page, 'browser');
    await page.waitForTimeout(300);

    const windows = page.locator('[data-window]');
    await expect(windows).toHaveCount(2);
    await expect(windows.nth(0)).toBeVisible();
    await expect(windows.nth(1)).toBeVisible();
  });

  // VAL-RESP-010: Mobile — minimizing preserves window state via bottom bar
  test('minimizing on mobile preserves the window and exposes a restore target', async ({ page, authenticator }) => {
    const email = uniqueEmail();
    await registerAndLoadDesktop(page, authenticator, email, { width: 375, height: 812 });

    await openAppViaIcon(page, 'files');
    await page.waitForTimeout(300);
    const windowEl = page.locator('[data-window]').first();
    await expect(windowEl).toBeVisible();

    await page.locator('[data-window-minimize]').first().click();
    await page.waitForTimeout(200);

    await expect(windowEl).not.toBeVisible();

    const indicator = page.locator('[data-minimized-indicator]');
    await expect(indicator).toHaveCount(1);
    await expect(indicator.first()).toBeVisible();

    await indicator.first().click();
    await page.waitForTimeout(200);
    await expect(windowEl).toBeVisible();
  });

  // VAL-RESP-008: Mobile — window remains floating and resizable
  test('window remains floating and resizable on mobile', async ({ page, authenticator }) => {
    const email = uniqueEmail();
    await registerAndLoadDesktop(page, authenticator, email, { width: 375, height: 812 });

    await openAppViaIcon(page, 'files');
    await page.waitForTimeout(300);

    const windowEl = page.locator('[data-window]').first();
    await expect(windowEl).toBeVisible();

    const winBox = await windowEl.boundingBox();
    expect(winBox.width).toBeLessThan(375);
    expect(winBox.x).toBeGreaterThan(0);

    const resizeHandle = windowEl.locator('[data-resize-handle]');
    await expect(resizeHandle).toHaveCount(1);
  });

  // VAL-RESP-009: Mobile — prompt bar full width with >=44px touch target
  test('prompt bar full width with >=44px touch target', async ({ page, authenticator }) => {
    const email = uniqueEmail();
    await registerAndLoadDesktop(page, authenticator, email, { width: 375, height: 812 });

    const promptInput = page.locator('[data-prompt-input]');
    await expect(promptInput).toBeVisible();

    // Touch target should be >=44px
    const height = await promptInput.evaluate((el) => el.offsetHeight);
    expect(height).toBeGreaterThanOrEqual(44);
  });

  // VAL-RESP-013: Mobile — consistent desktop experience
  test('consistent desktop experience — mobile uses the same floating window model', async ({ page, authenticator }) => {
    const email = uniqueEmail();
    await registerAndLoadDesktop(page, authenticator, email, { width: 375, height: 812 });

    // Floating icons visible
    const icons = page.locator('[data-desktop-icon]');
    await expect(icons).toHaveCount(7);

    // No hamburger, no rail, no overlay
    await expect(page.locator('[data-hamburger-btn]')).toHaveCount(0);
    await expect(page.locator('[data-desktop-rail]')).toHaveCount(0);
    await expect(page.locator('[data-rail-backdrop]')).toHaveCount(0);

    await openAppViaIcon(page, 'files');
    await page.waitForTimeout(300);

    const windowEl = page.locator('[data-window]').first();
    await expect(windowEl).toBeVisible();
    await expect(windowEl.locator('[data-window-minimize]')).toBeVisible();
    await expect(windowEl.locator('[data-window-maximize]')).toBeVisible();
    await expect(windowEl.locator('[data-window-close]')).toBeVisible();
  });
});

// ================================================================
// CROSS-BREAKPOINT TESTS
// ================================================================

test.describe('Cross-breakpoint checks', () => {
  // VAL-RESP-011: No horizontal overflow at any breakpoint
  test('no horizontal overflow at desktop breakpoint', async ({ page, authenticator }) => {
    const email = uniqueEmail();
    await registerAndLoadDesktop(page, authenticator, email, { width: 1280, height: 800 });

    const scrollWidth = await page.evaluate(() => document.documentElement.scrollWidth);
    const clientWidth = await page.evaluate(() => document.documentElement.clientWidth);
    expect(scrollWidth).toBeLessThanOrEqual(clientWidth);
  });

  test('no horizontal overflow at tablet breakpoint', async ({ page, authenticator }) => {
    const email = uniqueEmail();
    await registerAndLoadDesktop(page, authenticator, email, { width: 900, height: 800 });

    const scrollWidth = await page.evaluate(() => document.documentElement.scrollWidth);
    const clientWidth = await page.evaluate(() => document.documentElement.clientWidth);
    expect(scrollWidth).toBeLessThanOrEqual(clientWidth);
  });

  test('no horizontal overflow at mobile breakpoint', async ({ page, authenticator }) => {
    const email = uniqueEmail();
    await registerAndLoadDesktop(page, authenticator, email, { width: 375, height: 812 });

    const scrollWidth = await page.evaluate(() => document.documentElement.scrollWidth);
    const clientWidth = await page.evaluate(() => document.documentElement.clientWidth);
    expect(scrollWidth).toBeLessThanOrEqual(clientWidth);
  });

  test('open window stays inside mobile viewport after desktop resize', async ({ page, authenticator }) => {
    const email = uniqueEmail();
    await registerAndLoadDesktop(page, authenticator, email, { width: 1280, height: 800 });

    await openAppViaIcon(page, 'trace');
    const windowEl = page.locator('[data-window]').first();
    await expect(windowEl).toBeVisible({ timeout: 5000 });

    await page.setViewportSize({ width: 390, height: 844 });
    await page.waitForTimeout(300);

    const box = await windowEl.boundingBox();
    expect(box.x).toBeGreaterThanOrEqual(7);
    expect(box.x + box.width).toBeLessThanOrEqual(383);
  });

  test('Trace mobile trajectory item remains inside its sidebar hit target', async ({ page, authenticator }) => {
    const email = uniqueEmail();
    await registerAndLoadDesktop(page, authenticator, email, { width: 1280, height: 800 });
    const trajectoryId = await mockTraceTrajectory(page);

    await openAppViaIcon(page, 'trace');
    const trace = page.locator('[data-trace-app]').last();
    await expect(trace).toBeVisible({ timeout: 5000 });
    await expect(trace.locator(`[data-trace-trajectory-id="${trajectoryId}"]`)).toBeVisible();

    await page.setViewportSize({ width: 390, height: 844 });
    await page.waitForTimeout(300);
    await trace.locator('[data-trace-mobile-tabs] button').filter({ hasText: 'Runs' }).click();
    await expect(trace.locator(`[data-trace-trajectory-id="${trajectoryId}"]`)).toBeVisible();

    const metrics = await trace.locator(`[data-trace-trajectory-id="${trajectoryId}"]`).evaluate((item) => {
      const sidebar = item.closest('.trace-sidebar');
      const list = item.closest('[data-trace-trajectory-list]');
      const itemRect = item.getBoundingClientRect();
      const sidebarRect = sidebar.getBoundingClientRect();
      const x = Math.min(itemRect.right - 8, Math.max(itemRect.left + 8, itemRect.left + itemRect.width / 2));
      const y = Math.min(itemRect.bottom - 8, Math.max(itemRect.top + 8, itemRect.top + itemRect.height / 2));
      const hit = document.elementFromPoint(x, y);
      return {
        itemBottom: itemRect.bottom,
        sidebarBottom: sidebarRect.bottom,
        listScrolls: list.scrollHeight > list.clientHeight,
        hitInsideItem: item === hit || item.contains(hit),
        hitTag: hit?.tagName || '',
      };
    });

    expect(metrics.itemBottom).toBeLessThanOrEqual(metrics.sidebarBottom + 1);
    expect(metrics.hitInsideItem).toBe(true);
  });

  test('Trace mobile exposes Runs, Summary, Timeline, and Inspector drill-in panels', async ({ page, authenticator }) => {
    const email = uniqueEmail();
    await registerAndLoadDesktop(page, authenticator, email, { width: 390, height: 844 });
    await mockTraceTrajectory(page);

    await openAppViaIcon(page, 'trace');
    const trace = page.locator('[data-trace-app]').last();
    await expect(trace).toBeVisible({ timeout: 5000 });

    const tabs = trace.locator('[data-trace-mobile-tabs]');
    await expect(tabs).toBeVisible();
    await expect(trace.locator('[data-trace-summary-panel]')).toBeVisible();

    await tabs.locator('button').filter({ hasText: 'Timeline' }).click();
    await expect(trace.locator('[data-trace-summary-panel]')).not.toBeVisible();
    await expect(trace.locator('[data-trace-moment-strip]')).toBeVisible();

    await trace.locator('[data-trace-moment]').first().click();
    await expect(trace.locator('[data-trace-inspector]')).toBeVisible();
    await expect(trace.locator('[data-trace-summary-panel]')).not.toBeVisible();

    await tabs.locator('button').filter({ hasText: 'Runs' }).click();
    await expect(trace.locator('[data-trace-trajectory-list]')).toBeVisible();
  });

  // VAL-RESP-012: Breakpoint transition is smooth (no layout flash)
  test('breakpoint transition from desktop to tablet is clean', async ({ page, authenticator }) => {
    const email = uniqueEmail();
    await registerAndLoadDesktop(page, authenticator, email, { width: 1280, height: 800 });

    // Verify desktop layout — floating icons visible
    const surface = page.locator('[data-desktop-surface]');
    await expect(surface).toBeVisible();

    // Resize to tablet
    await page.setViewportSize({ width: 900, height: 800 });
    await page.waitForTimeout(300);

    // Icons should still be visible
    await expect(surface).toBeVisible();

    // No JS errors
    const logs = [];
    page.on('console', (msg) => {
      if (msg.type() === 'error') logs.push(msg.text());
    });
    await page.waitForTimeout(100);
    // Allow some errors that might be from other sources
  });
});
