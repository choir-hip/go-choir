import { test, expect } from './helpers/fixtures.js';
import { registerPasskey } from './helpers/auth.js';

const BASE_URL =
  process.env.GO_CHOIR_UX_BASE_URL ||
  process.env.PLAYWRIGHT_BASE_URL ||
  'http://localhost:4173';

test.use({ trace: 'on', video: 'on', screenshot: 'on' });

function uniqueEmail() {
  return `vtext-trace-${Date.now()}-${Math.random().toString(36).slice(2, 8)}@example.com`;
}

async function registerAndLoadDesktop(page, email) {
  await page.goto(BASE_URL);
  await registerPasskey(page, email, BASE_URL);
  await page.reload();
  await page.locator('[data-desktop]').waitFor({ state: 'visible', timeout: 15000 });
}

async function mockTraceTrajectory(page) {
  const trajectoryId = 'vtext-trace-coexistence';
  const timestamp = '2026-05-18T22:45:00Z';
  const trajectory = {
    trajectory_id: trajectoryId,
    title: 'VText and Trace coexistence proof',
    subtitle: 'editor focus · evidence inspection',
    state: 'completed',
    live: false,
    agent_count: 2,
    delegation_count: 1,
    moment_count: 2,
    message_count: 4,
    finding_count: 1,
    search_attempt_count: 0,
    latest_activity_at: timestamp,
    latest_stream_seq: 0,
  };
  const snapshot = {
    trajectory,
    agents: [
      { agent_id: 'vtext', label: 'VText', role: 'appagent', profile: 'vtext', run_count: 1, entry: true },
      { agent_id: 'trace', label: 'Trace verifier', role: 'verifier', profile: 'product-path', run_count: 1 },
    ],
    edges: [{ from_agent_id: 'vtext', to_agent_id: 'trace', label: 'inspect' }],
    moments: [
      {
        moment_id: 'moment-edit',
        kind: 'vtext.edit',
        tone: 'success',
        loop_id: 'loop-vtext-edit',
        summary: 'VText editor remains focused while Trace is open',
        created_at: timestamp,
        timestamp,
        agent_label: 'VText',
        stream_seq: 1,
      },
      {
        moment_id: 'moment-inspect',
        kind: 'trace.inspect',
        tone: 'success',
        loop_id: 'loop-trace-inspect',
        summary: 'Trace inspector remains reachable on mobile',
        created_at: timestamp,
        timestamp,
        agent_label: 'Trace verifier',
        stream_seq: 2,
      },
    ],
    search: {
      attempts: 18,
      successes: 12,
      providers: Array.from({ length: 12 }, (_, index) => ({
        provider: `provider-${index + 1}`,
        endpoint: `https://example.com/search/${index + 1}`,
        attempts: 2,
        successes: index % 3 === 0 ? 1 : 2,
        result_count: 20 + index,
        rate_limits: index % 4 === 0 ? 1 : 0,
        errors: 0,
        avg_latency_ms: 220 + index,
      })),
    },
    mobile_summary: {
      headline: 'staging-smoke-level · accepted · VText/Trace coexistence',
      acceptance_state: 'accepted',
      acceptance_level: 'staging-smoke-level',
      agent_count: 2,
      delegation_count: 1,
      evidence_ref_count: 2,
      rollback_ref_count: 1,
      readable_evidence: ['focused VText editor', 'Trace inspector drill-in'],
      rollback_refs: ['test rollback ref'],
    },
    acceptances: [],
  };

  await page.route('**/api/trace/**', async (route) => {
    const url = new URL(route.request().url());
    const pathname = url.pathname;
    if (pathname === '/api/trace/trajectories') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ trajectories: [trajectory] }),
      });
      return;
    }
    if (pathname === `/api/trace/trajectories/${trajectoryId}`) {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(snapshot),
      });
      return;
    }
    if (pathname.startsWith(`/api/trace/trajectories/${trajectoryId}/moments/`)) {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          moment: snapshot.moments[1],
          messages: [{ role: 'verifier', text: 'Trace detail remains readable while VText stays intact.' }],
          artifacts: {},
        }),
      });
      return;
    }
    if (pathname.startsWith(`/api/trace/trajectories/${trajectoryId}/events`)) {
      await route.fulfill({ status: 200, contentType: 'text/event-stream', body: '\n' });
      return;
    }
    await route.fallback();
  });
}

async function openBlankVText(page) {
  await page.locator('[data-desktop-icon-id="vtext"]').dblclick();
  const vtextWindow = page.locator('[data-window]').filter({ has: page.locator('[data-vtext-app]') }).last();
  await expect(vtextWindow).toBeVisible({ timeout: 15000 });
  await vtextWindow.locator('[data-vtext-recent]').waitFor({ state: 'visible', timeout: 15000 });
  await vtextWindow.locator('[data-vtext-new-document]').click();
  const editor = vtextWindow.locator('[data-vtext-editor-area]');
  await editor.waitFor({ state: 'visible', timeout: 15000 });
  await expect(editor).toHaveAttribute('contenteditable', 'true', { timeout: 15000 });
  return { vtextWindow, editor };
}

async function taskButton(page, titlePattern) {
  return page.locator('[data-window-switcher] [data-window-indicator]').filter({ hasText: titlePattern }).last();
}

test('VText keeps focused editing stable while Trace is open on mobile desktop', async ({ page, authenticator }) => {
  expect(authenticator.authenticatorId).toBeTruthy();
  await page.setViewportSize({ width: 390, height: 844 });
  await mockTraceTrajectory(page);
  await registerAndLoadDesktop(page, uniqueEmail());

  const { vtextWindow, editor } = await openBlankVText(page);
  const draft = [
    '# Coexistence Draft',
    '',
    ...Array.from({ length: 26 }, (_, index) =>
      `Paragraph ${index + 1}: the editor should not flicker or lose focus when Trace is open.`
    ),
  ].join('\n\n');
  await editor.fill(draft);
  await editor.click();
  await editor.evaluate((node) => {
    node.scrollTop = 220;
    node.dispatchEvent(new Event('scroll', { bubbles: true }));
  });

  await expect(vtextWindow.locator('[data-vtext-toolbar]')).toHaveCSS('opacity', '1');
  await expect.poll(() => page.evaluate(() => document.activeElement?.matches('[data-vtext-editor-area]'))).toBe(true);

  await page.locator('[data-start-button]').click();
  await page.locator('[data-start-app-id="trace"]').click();
  const traceWindow = page.locator('[data-window]').filter({ has: page.locator('[data-trace-app]') }).last();
  await expect(traceWindow).toBeVisible({ timeout: 15000 });
  await expect(traceWindow.locator('[data-trace-mobile-tabs]')).toBeVisible();
  const traceScrollOwners = await traceWindow.evaluate((root) => {
    const nodes = [root, ...root.querySelectorAll('*')];
    return nodes
      .filter((node) => {
        const style = getComputedStyle(node);
        return /(auto|scroll)/.test(style.overflowY) && node.scrollHeight > node.clientHeight + 8;
      })
      .map((node) => ({
        className: String(node.className || ''),
        testId: node.getAttribute('data-trace-scroll-owner') != null ? 'trace-app' : node.getAttribute('data-window-content') != null ? 'window-content' : '',
        scrollHeight: node.scrollHeight,
        clientHeight: node.clientHeight,
      }));
  });
  expect(traceScrollOwners).toHaveLength(1);
  expect(traceScrollOwners[0].testId).toBe('trace-app');
  await traceWindow.locator('[data-trace-mobile-tabs] button', { hasText: 'Inspector' }).click();
  await expect(traceWindow.locator('[data-trace-inspector]')).toBeVisible();

  await (await taskButton(page, /VText/)).click();
  await expect(vtextWindow).toBeVisible();
  await expect(vtextWindow.locator('[data-vtext-editor-area]')).toContainText('Coexistence Draft');
  await editor.click();
  await editor.press('End');
  await editor.type('\n\nTrace stayed open while VText stayed editable.');
  await expect(editor).toContainText('Trace stayed open while VText stayed editable.');
  await expect(vtextWindow.locator('[data-vtext-toolbar]')).toHaveCSS('opacity', '1');

  const metrics = await page.evaluate(() => ({
    horizontalOverflow: document.documentElement.scrollWidth - document.documentElement.clientWidth,
    openWindowButtons: document.querySelectorAll('[data-window-switcher] [data-window-indicator]').length,
    activeApp: document.querySelector('[data-window].window-active [data-vtext-app]') ? 'vtext' : 'other',
  }));
  expect(metrics.horizontalOverflow).toBe(0);
  expect(metrics.openWindowButtons).toBeGreaterThanOrEqual(2);
  expect(metrics.activeApp).toBe('vtext');
});
