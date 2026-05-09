/**
 * Gateway end-to-end test for VAL-GATEWAY-001
 * Tests: Authenticated requests receive a provider response through the gateway
 *
 * Target: https://draft.choir-ip.com (deployed origin)
 * Path: login -> proxy -> user runtime/VM -> gateway -> provider -> UI response
 */
import fs from 'node:fs';
import { test, expect } from '@playwright/test';
import { setupVirtualAuthenticator, removeVirtualAuthenticator } from './helpers/webauthn.js';
import { registerPasskey, getSession } from './helpers/auth.js';

const BASE_URL = 'https://draft.choir-ip.com';
const EVIDENCE_DIR = process.env.CHOIR_EVIDENCE_DIR || 'test-results/gateway-e2e-deployed';
fs.mkdirSync(EVIDENCE_DIR, { recursive: true });

function uniqueEmail() {
  return `gateway-e2e-${Date.now()}-${Math.random().toString(36).slice(2, 8)}@example.com`;
}

async function fetchJSON(page, path) {
  return page.evaluate(async (requestPath) => {
    const res = await fetch(requestPath, { credentials: 'include' });
    const body = await res.json().catch(() => null);
    if (!res.ok) {
      throw new Error(`${requestPath} failed: ${res.status} ${JSON.stringify(body)}`);
    }
    return body;
  }, path);
}

async function waitForPromptSubmissionDecision(page, submissionId, timeout = 120000) {
  const deadline = Date.now() + timeout;
  while (Date.now() < deadline) {
    const status = await fetchJSON(page, `/api/prompt-bar/submissions/${encodeURIComponent(submissionId)}`);
    if (status.decision) {
      return { status, decision: status.decision };
    }
    if (['failed', 'blocked', 'cancelled'].includes(status.state)) {
      throw new Error(status.error || `prompt submission ${submissionId} ended as ${status.state}`);
    }
    await page.waitForTimeout(1000);
  }
  throw new Error(`prompt submission ${submissionId} did not produce a decision`);
}

const testResults = {
  assertionId: 'VAL-GATEWAY-001',
  title: 'Authenticated requests receive a provider response through the gateway',
  status: 'pending',
  steps: [],
  evidence: {
    screenshots: [],
    consoleErrors: 'none',
    network: []
  },
  issues: null
};

test('VAL-GATEWAY-001: Gateway end-to-end flow', async ({ browser }) => {
  test.setTimeout(120000);
  const context = await browser.newContext({
    viewport: { width: 430, height: 932 },
    isMobile: true,
    hasTouch: true
  });
  const page = await context.newPage();

  // Capture network requests
  const networkRequests = [];
  page.on('request', (req) => {
    networkRequests.push({
      method: req.method(),
      url: req.url(),
      timestamp: new Date().toISOString()
    });
  });

  page.on('response', async (res) => {
    const url = new URL(res.url());
    if (url.pathname.includes('/api/') || url.pathname.includes('/auth/')) {
      networkRequests.push({
        method: res.request().method(),
        url: res.url(),
        status: res.status(),
        timestamp: new Date().toISOString()
      });
    }
  });

  // Capture console errors
  const consoleErrors = [];
  page.on('console', (msg) => {
    if (msg.type() === 'error') {
      consoleErrors.push(msg.text());
    }
  });

  try {
    // Step 1: Navigate to deployed origin
    testResults.steps.push({
      action: 'Navigate to deployed origin',
      expected: 'Page loads with auth UI',
      observed: 'In progress...'
    });

    await page.goto(BASE_URL);
    await page.waitForLoadState('networkidle');

    await page.screenshot({ path: `${EVIDENCE_DIR}/07-playwright-initial.png` });
    testResults.evidence.screenshots.push('gateway-vm/gateway-e2e/07-playwright-initial.png');

    testResults.steps[0].observed = `Page loaded: ${page.url()}, Title: ${await page.title()}`;

    // Step 2: Set up virtual authenticator for WebAuthn
    testResults.steps.push({
      action: 'Set up virtual WebAuthn authenticator',
      expected: 'Virtual authenticator ready for passkey registration',
      observed: 'In progress...'
    });

    const { client, authenticatorId } = await setupVirtualAuthenticator(page);
    testResults.steps[1].observed = `Virtual authenticator created: ${authenticatorId}`;

    // Step 3: Register a new user with passkey
    const email = uniqueEmail();
    testResults.steps.push({
      action: `Register new user: ${email}`,
      expected: 'Passkey registration completes successfully',
      observed: 'In progress...'
    });

    const registerResult = await registerPasskey(page, email, BASE_URL);

    if (!registerResult.ok) {
      throw new Error(`Registration failed: ${JSON.stringify(registerResult)}`);
    }

    testResults.steps[2].observed = `Registration successful, user: ${registerResult.user?.email || email}`;

    await page.screenshot({ path: `${EVIDENCE_DIR}/08-playwright-registered.png` });
    testResults.evidence.screenshots.push('gateway-vm/gateway-e2e/08-playwright-registered.png');

    // Step 4: Verify authenticated session
    testResults.steps.push({
      action: 'Verify authenticated session',
      expected: 'Session shows authenticated: true with user identity',
      observed: 'In progress...'
    });

    const session = await getSession(page, BASE_URL);

    if (!session.authenticated) {
      throw new Error('Session not authenticated after registration');
    }

    testResults.steps[3].observed = `Session authenticated: ${session.authenticated}, user: ${session.user?.email}`;

    // Step 5: Reload to reach the authenticated shell
    testResults.steps.push({
      action: 'Reload to reach authenticated shell',
      expected: 'Shell UI loads with prompt input',
      observed: 'In progress...'
    });

    await page.reload();
    await page.waitForLoadState('networkidle');

    // Wait for shell to be visible
    try {
      await page.locator('[data-shell]').waitFor({ state: 'visible', timeout: 10000 });
      testResults.steps[4].observed = 'Shell UI visible after reload';
    } catch (e) {
      // Try alternative selectors
      const bodyText = await page.locator('body').textContent();
      testResults.steps[4].observed = `Shell check: ${bodyText?.substring(0, 200)}...`;
    }

    await page.screenshot({ path: `${EVIDENCE_DIR}/09-playwright-shell.png` });
    testResults.evidence.screenshots.push('gateway-vm/gateway-e2e/09-playwright-shell.png');

    // Step 6: Submit through the visible prompt bar and verify VText opens.
    testResults.steps.push({
      action: 'Submit prompt through visible prompt bar',
      expected: 'Prompt bar posts /api/prompt-bar, conductor opens VText, and VText materializes v0/v1',
      observed: 'In progress...'
    });

    const marker = `deployed-prompt-bar-${Date.now()}`;
    const prompt = `Draft a vtext abstract for ${marker}`;
    const promptInput = page.locator('[data-prompt-input]');
    await expect(promptInput).toBeVisible({ timeout: 10000 });
    await expect(promptInput).toBeEnabled();

    const promptBarResponse = page.waitForResponse((response) =>
      new URL(response.url()).pathname === '/api/prompt-bar' && response.request().method() === 'POST'
    );
    await promptInput.fill(prompt);
    await promptInput.press('Enter');

    const response = await promptBarResponse;
    const responseBody = await response.json().catch(() => null);
    if (response.status() !== 202 || !responseBody?.submission_id) {
      throw new Error(`Prompt bar submission failed: ${response.status()} ${JSON.stringify(responseBody)}`);
    }
    const posted = response.request().postDataJSON();
    expect(posted).toEqual({ text: prompt });

    testResults.steps[5].observed = `Prompt bar returned submission ${responseBody.submission_id}`;

    testResults.steps.push({
      action: 'Wait for conductor decision and VText window',
      expected: 'Conductor decision opens VText with a durable document id',
      observed: 'In progress...'
    });

    const { status: finalStatus, decision } = await waitForPromptSubmissionDecision(page, responseBody.submission_id);
    expect(decision.action).toBe('open_app');
    expect(decision.app).toBe('vtext');
    expect(decision.doc_id).toBeTruthy();
    expect(decision.user_revision_id).toBeTruthy();
    expect(decision.framing_revision_id).toBeTruthy();
    expect(decision.initial_loop_id).toBeTruthy();

    const vtextWindow = page.locator('[data-vtext-app]').last();
    await expect(vtextWindow).toBeVisible({ timeout: 30000 });
    await expect(vtextWindow.locator('[data-vtext-editor-area]')).toContainText(new RegExp(marker.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')), { timeout: 30000 });
    await expect(vtextWindow.locator('[data-vtext-editor-area]')).not.toContainText(/Conductor framing|Use this vtext|User request:|Current requirements:|Grounding status:/);

    testResults.steps[6].observed = `Conductor decision: ${JSON.stringify(finalStatus.decision)?.substring(0, 300)}`;

    testResults.steps.push({
      action: 'Verify durable VText revisions and trace projection',
      expected: 'Document has user v0, conductor v1, and Trace sees the VText trajectory',
      observed: 'In progress...'
    });

    const revisionsResponse = await fetchJSON(page, `/api/vtext/documents/${encodeURIComponent(decision.doc_id)}/revisions`);
    const revisions = revisionsResponse.revisions || [];
    const userRevision = revisions.find((revision) => revision.revision_id === decision.user_revision_id);
    const framingRevision = revisions.find((revision) => revision.revision_id === decision.framing_revision_id);
    expect(userRevision?.author_kind).toBe('user');
    expect(userRevision?.content).toBe(prompt);
    expect(framingRevision?.author_kind).toBe('appagent');
    expect(framingRevision?.author_label).toBe('conductor');
    expect(framingRevision?.content || '').toContain(marker);
    expect(framingRevision?.content || '').not.toMatch(/Conductor framing|Use this vtext|User request:|Current requirements:|Grounding status:/);

    const trace = await fetchJSON(page, `/api/trace/trajectories/${encodeURIComponent(responseBody.submission_id)}`);
    expect((trace.agents || []).some((agent) => agent.profile === 'vtext' && agent.agent_id === `vtext:${decision.doc_id}`)).toBe(true);

    testResults.steps[7].observed = `VText revisions=${revisions.length}, trace agents=${(trace.agents || []).length}`;
    testResults.status = 'pass';

    await page.screenshot({ path: `${EVIDENCE_DIR}/10-playwright-completed.png` });
    testResults.evidence.screenshots.push('gateway-vm/gateway-e2e/10-playwright-completed.png');

    // Cleanup
    await removeVirtualAuthenticator(client, authenticatorId);
    await context.close();

  } catch (error) {
    testResults.status = 'fail';
    testResults.issues = error.message;
    await page.screenshot({ path: `${EVIDENCE_DIR}/10-playwright-error.png` });
    testResults.evidence.screenshots.push('gateway-vm/gateway-e2e/10-playwright-error.png');
    throw error;
  }

  testResults.evidence.consoleErrors = consoleErrors.length > 0 ? consoleErrors.join(', ') : 'none';
  testResults.evidence.network = networkRequests.slice(0, 20); // Limit to first 20

  // Write results to file for the test report
  fs.writeFileSync(`${EVIDENCE_DIR}/test-results.json`, JSON.stringify(testResults, null, 2));

  expect(testResults.status).toBe('pass');
});
