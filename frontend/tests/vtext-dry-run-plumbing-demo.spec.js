import { test, expect } from './helpers/fixtures.js';
import { registerPasskey } from './helpers/auth.js';

const BASE_URL = 'http://localhost:4173';

test.use({ trace: 'on', video: 'on', screenshot: 'on' });
test.setTimeout(75_000);
test.skip(
  process.env.GO_CHOIR_RUN_STUB_DRY_RUN_DEMO !== '1',
  'dry-run plumbing demo uses /api/test endpoints and seeded artifacts; it is not product proof'
);

function uniqueEmail() {
  return `vtext-demo-${Date.now()}-${Math.random().toString(36).slice(2, 8)}@example.com`;
}

async function registerAndLoadDesktop(page, authenticator, email) {
  await page.goto(BASE_URL);
  await registerPasskey(page, email, BASE_URL);
  await page.reload();
  await page.locator('[data-desktop]').waitFor({ state: 'visible', timeout: 10000 });
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

async function postJSON(page, path, body) {
  return page.evaluate(async ({ requestPath, requestBody }) => {
    const res = await fetch(requestPath, {
      method: 'POST',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(requestBody),
    });
    if (!res.ok) {
      const text = await res.text();
      throw new Error(`${requestPath} failed: ${res.status} ${text}`);
    }
    return res.json();
  }, { requestPath: path, requestBody: body });
}

async function putTextFile(page, path, content) {
  return page.evaluate(async ({ filePath, fileContent }) => {
    const res = await fetch(`/api/files/${encodeURIComponent(filePath)}`, {
      method: 'PUT',
      credentials: 'include',
      headers: { 'Content-Type': 'text/plain; charset=utf-8' },
      body: fileContent,
    });
    if (!res.ok) {
      const text = await res.text();
      throw new Error(`write ${filePath} failed: ${res.status} ${text}`);
    }
    return res.json();
  }, { filePath: path, fileContent: content });
}

async function ensureDirectory(page, path) {
  await page.evaluate(async (dirPath) => {
    const res = await fetch(`/api/files/${encodeURIComponent(dirPath)}`, {
      method: 'POST',
      credentials: 'include',
    });
    if (!res.ok && res.status !== 409) {
      const text = await res.text();
      throw new Error(`mkdir ${dirPath} failed: ${res.status} ${text}`);
    }
  }, path);
}

async function waitForPromptDecision(page, submissionId, timeout = 12000) {
  const deadline = Date.now() + timeout;
  while (Date.now() < deadline) {
    const status = await fetchJSON(page, `/api/prompt-bar/submissions/${encodeURIComponent(submissionId)}`);
    if (status.decision) return status.decision;
    if (['failed', 'blocked', 'cancelled'].includes(status.state)) {
      throw new Error(status.error || `prompt submission ${submissionId} ended as ${status.state}`);
    }
    await page.waitForTimeout(200);
  }
  throw new Error(`prompt submission ${submissionId} did not produce a decision`);
}

async function listRevisions(page, docId) {
  return fetchJSON(page, `/api/vtext/documents/${encodeURIComponent(docId)}/revisions`);
}

async function waitForRevisionTotal(page, docId, want, timeout = 15000) {
  const deadline = Date.now() + timeout;
  while (Date.now() < deadline) {
    const revisions = await listRevisions(page, docId);
    if ((revisions.revisions || []).length >= want) return revisions;
    await page.waitForTimeout(250);
  }
  const revisions = await listRevisions(page, docId);
  throw new Error(`document ${docId} did not reach ${want} revisions, got ${(revisions.revisions || []).length}`);
}

async function waitForConsumedWorkerRoles(page, docId, roles, timeout = 20000) {
  const deadline = Date.now() + timeout;
  while (Date.now() < deadline) {
    const doc = await fetchJSON(page, `/api/vtext/documents/${encodeURIComponent(docId)}`);
    const revisions = await listRevisions(page, docId);
    const head = (revisions.revisions || []).find((revision) => revision.revision_id === doc.current_revision_id);
    const consumed = (revisions.revisions || []).flatMap((revision) =>
      revision?.metadata?.worker_updates_consumed || []
    );
    if (roles.every((role) => consumed.some((item) => item.role === role))) {
      return { doc, revisions, head, consumed };
    }
    await page.waitForTimeout(300);
  }
  throw new Error(`document ${docId} did not consume worker roles: ${roles.join(', ')}`);
}

async function submitDryRunResearchFindings(page, payload) {
  return postJSON(page, '/api/test/vtext/research-findings', payload);
}

async function submitDryRunWorkerUpdate(page, payload) {
  return postJSON(page, '/api/test/vtext/worker-update', payload);
}

async function seedEvolutionArtifact(page) {
  const html = `<!doctype html>
<html>
<head>
  <meta charset="utf-8">
  <title>Evolution CA Demo</title>
  <style>
    body { margin: 0; font-family: system-ui, sans-serif; background: #101418; color: #e5edf5; }
    main { padding: 24px; }
    canvas { width: 520px; height: 260px; background: #0b0e11; border: 1px solid #3b4652; }
    .stats { margin-top: 12px; display: flex; gap: 16px; }
  </style>
</head>
<body>
  <main>
    <h1>Evolution CA</h1>
    <canvas id="grid" width="520" height="260"></canvas>
    <div class="stats"><span>seed: demo-42</span><span>generations: 64</span><span>verified: deterministic</span></div>
    <script>
      const c = document.getElementById('grid');
      const ctx = c.getContext('2d');
      for (let y = 0; y < 26; y++) {
        for (let x = 0; x < 52; x++) {
          const n = (x * 17 + y * 31 + (x ^ y) * 7) % 9;
          ctx.fillStyle = n > 5 ? '#7dd3fc' : n > 2 ? '#86efac' : '#334155';
          ctx.fillRect(x * 10, y * 10, 9, 9);
        }
      }
    </script>
  </main>
</body>
</html>`;
  const verify = `const assert = require('assert');
const generations = 64;
const seeded = 'demo-42';
assert.strictEqual(generations, 64);
assert.strictEqual(seeded, 'demo-42');
console.log('evolution-ca.verify.js passed');`;
  await ensureDirectory(page, 'artifacts');
  await putTextFile(page, 'artifacts/evolution-ca.html', html);
  await putTextFile(page, 'artifacts/evolution-ca.verify.js', verify);
  return html;
}

test('dry-run vtext plumbing demo uses seeded worker updates and artifacts', async ({ page, authenticator }) => {
  await registerAndLoadDesktop(page, authenticator, uniqueEmail());

  const prompt = 'Research cellular automata models of biological evolution, then build and verify a small interactive visualization.';
  const conductorResponse = page.waitForResponse((response) =>
    response.url().includes('/api/prompt-bar') && response.request().method() === 'POST'
  );
  await page.locator('[data-prompt-input]').fill(prompt);
  await page.locator('[data-prompt-input]').press('Enter');

  const conductorSubmitted = await (await conductorResponse).json();
  const conductorDecision = await waitForPromptDecision(page, conductorSubmitted.submission_id);
  expect(conductorDecision.action).toBe('open_app');
  expect(conductorDecision.app).toBe('vtext');
  expect(conductorDecision.initial_loop_id || '').toBeTruthy();

  const vtextWindow = page.locator('[data-vtext-app]').last();
  await expect(vtextWindow).toBeVisible({ timeout: 10000 });
  await expect(vtextWindow.locator('[data-vtext-version]')).toHaveText('v1');
  await expect(vtextWindow.locator('[data-vtext-editor-area]')).toContainText(/cellular automata/);
  await expect(vtextWindow.locator('[data-vtext-editor-area]')).not.toContainText(/Conductor framing|Use this vtext|User request:|Current requirements:|Grounding status:/);

  const beforeManualRevision = await listRevisions(page, conductorDecision.doc_id);
  const editor = vtextWindow.locator('[data-vtext-editor-area]');
  await editor.fill(`${await editor.textContent()}

User edit: keep the model deterministic, cite the research assumptions, and require a generated artifact plus a verification script.`);

  const manualRevisionResponse = page.waitForResponse((response) =>
    response.request().method() === 'POST' &&
    /\/api\/vtext\/documents\/[^/]+\/revise$/.test(new URL(response.url()).pathname)
  );
  await vtextWindow.locator('[data-vtext-prompt]').click();
  const manualRevision = await (await manualRevisionResponse).json();
  await waitForRevisionTotal(page, conductorDecision.doc_id, beforeManualRevision.revisions.length + 2, 15000);
  await expect(vtextWindow.locator('[data-vtext-version]')).toContainText(/^v[3-9]/, { timeout: 10000 });

  const artifactHTML = await seedEvolutionArtifact(page);
  const researchResp = await submitDryRunResearchFindings(page, {
    doc_id: conductorDecision.doc_id,
    finding_id: `biology-ca-${Date.now()}`,
    findings: [
      'Cellular automata are useful toy models when local rules, inherited state, mutation, and selection pressure are explicit.',
      'The demo should label assumptions instead of claiming biological fidelity.',
    ],
    evidence: [
      {
        kind: 'web_summary',
        source_uri: 'https://plato.stanford.edu/entries/cellular-automata/',
        title: 'Cellular Automata overview',
        content: 'Background source for local-rule cellular automata framing.',
      },
    ],
    notes: ['Fold the source and assumption limits into the next canonical version.'],
  });
  expect(researchResp.status).toBe('submitted');
  expect(researchResp.trajectory_id).toBe(conductorSubmitted.submission_id);

  const superResp = await submitDryRunWorkerUpdate(page, {
    doc_id: conductorDecision.doc_id,
    update_id: `super-evolution-artifact-${Date.now()}`,
    role: 'super',
    artifacts: ['artifacts/evolution-ca.html', 'artifacts/evolution-ca.verify.js'],
    tests: ['node artifacts/evolution-ca.verify.js passed'],
    proposals: ['Mention the generated artifact and deterministic verification result in the next version.'],
  });
  expect(superResp.status).toBe('submitted');
  expect(superResp.trajectory_id).toBe(conductorSubmitted.submission_id);

  const coSuperResp = await submitDryRunWorkerUpdate(page, {
    doc_id: conductorDecision.doc_id,
    update_id: `cosuper-evolution-review-${Date.now()}`,
    role: 'co-super',
    findings: ['The artifact exposes seed, generation count, and a stable verification claim.'],
    refs: ['artifacts/evolution-ca.html#grid'],
    tests: ['Manual visual smoke check: canvas renders non-empty colored cells.'],
    notes: ['Keep the final text bounded: this is a toy model, not a biological simulator.'],
  });
  expect(coSuperResp.status).toBe('submitted');
  expect(coSuperResp.trajectory_id).toBe(conductorSubmitted.submission_id);

  const finalState = await waitForConsumedWorkerRoles(page, conductorDecision.doc_id, ['researcher', 'super', 'co-super']);
  expect(finalState.revisions.revisions.length).toBeGreaterThanOrEqual(5);
  const finalText = finalState.head?.content || '';
  expect(finalText).toContain('User edit: keep the model deterministic');
  expect(finalText).toMatch(/Cellular automata|local rules|selection pressure/i);
  expect(finalText).toContain('artifacts/evolution-ca.html');
  expect(finalText).toContain('node artifacts/evolution-ca.verify.js passed');
  expect(finalText).not.toMatch(/Task completed successfully|stub provider|Worker update ready\.|Research findings ready\./i);
  await expect(vtextWindow.locator('[data-vtext-version]')).toContainText(/^v[4-9]/, { timeout: 10000 });

  await page.locator('[data-desktop-icon-id="files"]').dblclick();
  const filesApp = page.locator('[data-files-app]').last();
  await expect(filesApp).toBeVisible({ timeout: 10000 });
  await filesApp.locator('[data-file-item]').filter({ hasText: 'artifacts' }).first().click();
  await expect(filesApp.locator('[data-file-item]').filter({ hasText: 'evolution-ca.html' })).toBeVisible({ timeout: 10000 });
  await expect(filesApp.locator('[data-file-item]').filter({ hasText: 'evolution-ca.verify.js' })).toBeVisible();

  await page.locator('[data-desktop-icon-id="browser"]').dblclick();
  const browserApp = page.locator('[data-browser-app]').last();
  await expect(browserApp).toBeVisible({ timeout: 10000 });
  await browserApp.locator('[data-browser-url-input]').fill(`data:text/html;charset=utf-8,${encodeURIComponent(artifactHTML)}`);
  await browserApp.locator('[data-browser-go-btn]').click();
  await expect(browserApp.locator('[data-browser-iframe]')).toHaveAttribute('src', /data:text\/html/, { timeout: 10000 });

  const traceSnapshot = await fetchJSON(page, `/api/trace/trajectories/${encodeURIComponent(conductorSubmitted.submission_id)}`);
  const roles = traceSnapshot.agents.map((agent) => agent.role || agent.profile || agent.label);
  expect(roles).toEqual(expect.arrayContaining(['conductor', 'vtext', 'researcher', 'super', 'co-super']));
  expect(traceSnapshot.moments.some((moment) => /Worker update ready|Research findings ready/i.test(moment.summary))).toBe(true);

  await page.locator('[data-desktop-icon-id="trace"]').dblclick();
  const traceApp = page.locator('[data-trace-app]').last();
  await expect(traceApp).toBeVisible({ timeout: 10000 });
  const trajectory = traceApp.locator(`[data-trace-trajectory-id="${conductorSubmitted.submission_id}"]`);
  await expect(trajectory).toBeVisible({ timeout: 10000 });
  await trajectory.click();
  await expect(traceApp.locator('[data-trace-agent-node]').filter({ hasText: /conductor/i })).toBeVisible();
  await expect(traceApp.locator('[data-trace-agent-node]').filter({ hasText: /vtext/i })).toBeVisible();
  await expect(traceApp.locator('[data-trace-agent-node]').filter({ hasText: /researcher/i })).toBeVisible();
  await expect(traceApp.locator('[data-trace-agent-node]').filter({ hasText: /^super\b/i })).toBeVisible();
  await expect(traceApp.locator('[data-trace-agent-node]').filter({ hasText: /^co-super\b/i })).toBeVisible();
  await expect(traceApp.locator('[data-trace-moment]').filter({ hasText: /Worker update ready|Research findings/i }).first()).toBeVisible();

  await page.waitForTimeout(1000);
  expect(manualRevision.loop_id).toBeTruthy();
});
