import { test, expect } from './helpers/fixtures.js';
import { registerPasskey } from './helpers/auth.js';

const BASE_URL = 'http://localhost:4173';

function uniqueEmail() {
  return `vtext-stream-${Date.now()}-${Math.random().toString(36).slice(2, 8)}@example.com`;
}

async function registerAndLoadDesktop(page, authenticator, email) {
  await page.goto(BASE_URL);
  await registerPasskey(page, email, BASE_URL);
  await page.reload();
  await page.locator('[data-desktop]').waitFor({ state: 'visible', timeout: 10000 });
}

async function openFilesApp(page) {
  await page.locator('[data-desktop-icon-id="files"]').dblclick();
  const filesWindow = page.locator('[data-files-app]').last();
  await filesWindow.waitFor({ state: 'visible', timeout: 10000 });
  const rootBreadcrumb = filesWindow.locator('[data-breadcrumb-segment]').first();
  await rootBreadcrumb.click();
  await filesWindow.locator('[data-file-list]').waitFor({ state: 'visible', timeout: 10000 });
  return filesWindow;
}

async function openVText(page) {
  await page.locator('[data-desktop-icon-id="vtext"]').dblclick();
  await page.locator('[data-vtext-editor]').last().waitFor({ state: 'visible', timeout: 10000 });
  const recent = page.locator('[data-vtext-app] [data-vtext-recent]').last();
  if (await recent.isVisible().catch(() => false)) {
    await page.locator('[data-vtext-app] [data-vtext-new-document]').last().click();
  }
  const editor = page.locator('[data-vtext-app] [data-vtext-editor-area]').last();
  await editor.waitFor({ state: 'visible', timeout: 10000 });
  await expect(editor).toHaveAttribute('contenteditable', 'true', { timeout: 10000 });
}

async function seedTextFile(page, fileName, content) {
  await page.evaluate(async ({ fileName, content }) => {
    const res = await fetch(`/api/files/${encodeURIComponent(fileName)}`, {
      method: 'PUT',
      credentials: 'include',
      headers: { 'Content-Type': 'text/plain; charset=utf-8' },
      body: content,
    });
    if (!res.ok) {
      throw new Error(`failed to seed text file ${fileName}: ${res.status}`);
    }
  }, { fileName, content });
}

async function openFileInVText(page, fileName) {
  const filesWindow = await openFilesApp(page);
  const openResponse = page.waitForResponse((response) => {
    const url = new URL(response.url());
    return response.request().method() === 'POST' && url.pathname === '/api/vtext/files/open';
  });
  const fileItem = filesWindow.locator('[data-file-item]').filter({ hasText: fileName }).first();
  await expect(fileItem).toBeVisible({ timeout: 5000 });
  await fileItem.click();
  await page.locator('[data-vtext-app]').last().waitFor({ state: 'visible', timeout: 10000 });
  return (await openResponse).json();
}

async function createExternalRevision(page, docId, parentRevisionId, content) {
  return page.evaluate(async ({ docId, parentRevisionId, content }) => {
    const res = await fetch(`/api/vtext/documents/${encodeURIComponent(docId)}/revisions`, {
      method: 'POST',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        content,
        author_kind: 'user',
        author_label: 'browser-test',
        parent_revision_id: parentRevisionId,
      }),
    });
    if (!res.ok) {
      const body = await res.text();
      throw new Error(`failed to create external revision: ${res.status} ${body}`);
    }
    return res.json();
  }, { docId, parentRevisionId, content });
}

async function listRevisions(page, docId) {
  return page.evaluate(async (docIdValue) => {
    const res = await fetch(`/api/vtext/documents/${encodeURIComponent(docIdValue)}/revisions`, {
      method: 'GET',
      credentials: 'include',
    });
    if (!res.ok) {
      const body = await res.text();
      throw new Error(`failed to list revisions: ${res.status} ${body}`);
    }
    return res.json();
  }, docId);
}

async function waitForRevisionTotal(page, docId, want, timeout = 12000) {
  const deadline = Date.now() + timeout;
  while (Date.now() < deadline) {
    const revisions = await listRevisions(page, docId);
    if ((revisions.revisions || []).length >= want) {
      return revisions;
    }
    await page.waitForTimeout(200);
  }
  const revisions = await listRevisions(page, docId);
  throw new Error(`document ${docId} did not reach ${want} revisions, got ${(revisions.revisions || []).length}`);
}

async function submitTestResearchFindings(page, payload) {
  return page.evaluate(async (body) => {
    const res = await fetch('/api/test/vtext/research-findings', {
      method: 'POST',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    });
    if (!res.ok) {
      const err = await res.text();
      throw new Error(`failed to submit research findings: ${res.status} ${err}`);
    }
    return res.json();
  }, payload);
}

async function submitTestWorkerUpdate(page, payload) {
  return page.evaluate(async (body) => {
    const res = await fetch('/api/test/vtext/worker-update', {
      method: 'POST',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    });
    if (!res.ok) {
      const err = await res.text();
      throw new Error(`failed to submit worker update: ${res.status} ${err}`);
    }
    return res.json();
  }, payload);
}

test('vtext auto-follows latest head when the editor is clean', async ({ page, authenticator }) => {
  await registerAndLoadDesktop(page, authenticator, uniqueEmail());
  const fileName = `auto-follow-${Date.now()}.txt`;
  const initialContent = 'Initial version from file open';
  const externalContent = 'External clean-head update';

  await seedTextFile(page, fileName, initialContent);
  const opened = await openFileInVText(page, fileName);

  const editor = page.locator('[data-vtext-app] [data-vtext-editor-area]').last();
  await expect(editor).toContainText(initialContent);

  await createExternalRevision(page, opened.doc_id, opened.current_revision_id, externalContent);

  await expect(editor).toContainText(externalContent, { timeout: 10000 });
  await expect(page.locator('[data-vtext-new-version]')).toHaveCount(0);
});

test('vtext autosaves dirty text without advancing versions when the head moves', async ({ page, authenticator }) => {
  await registerAndLoadDesktop(page, authenticator, uniqueEmail());
  const fileName = `dirty-rebase-${Date.now()}.txt`;
  const initialContent = 'Seed content from file open';
  const dirtyContent = 'Local dirty draft that must persist over the moved head';
  const externalContent = 'External moved-head update that must survive rebase';

  await seedTextFile(page, fileName, initialContent);
  const opened = await openFileInVText(page, fileName);

  const editor = page.locator('[data-vtext-app] [data-vtext-editor-area]').last();
  await editor.fill(dirtyContent);

  await createExternalRevision(page, opened.doc_id, opened.current_revision_id, externalContent);

  await expect(editor).toContainText(dirtyContent);
  await page.waitForTimeout(1400);

  const revisions = await listRevisions(page, opened.doc_id);
  const currentDoc = await page.evaluate(async (docId) => {
    const res = await fetch(`/api/vtext/documents/${encodeURIComponent(docId)}`, {
      method: 'GET',
      credentials: 'include',
    });
    if (!res.ok) {
      throw new Error(`failed to get document: ${res.status}`);
    }
    return res.json();
  }, opened.doc_id);
  expect(revisions.revisions).toHaveLength(2);
  const latestRevision = revisions.revisions.find((revision) => revision.revision_id === currentDoc.current_revision_id);
  expect(latestRevision?.content || '').toContain(externalContent);
  expect(latestRevision?.content || '').not.toContain(dirtyContent);
  await expect(editor).toContainText(dirtyContent, { timeout: 10000 });
  await expect(page.locator('[data-vtext-new-version]')).toHaveCount(1);
});

test('vtext does not restore stale local draft over a newer canonical table head', async ({ page, authenticator }) => {
  await registerAndLoadDesktop(page, authenticator, uniqueEmail());
  const stamp = Date.now();
  const created = await page.evaluate(async (stampValue) => {
    const docRes = await fetch('/api/vtext/documents', {
      method: 'POST',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ title: `Stale Draft Table ${stampValue}` }),
    });
    if (!docRes.ok) throw new Error(`create doc failed: ${docRes.status}`);
    const doc = await docRes.json();

    const staleContent = [
      '# Stale Draft Table',
      '',
      'Intro paragraph.',
      '',
      'Term',
      'Definition',
      'Vector database',
      'Stores embeddings for retrieval.',
    ].join('\n');
    const firstRes = await fetch(`/api/vtext/documents/${encodeURIComponent(doc.doc_id)}/revisions`, {
      method: 'POST',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        content: staleContent,
        author_kind: 'user',
        author_label: 'browser-test',
        metadata: { source: 'stale_draft_parent' },
      }),
    });
    if (!firstRes.ok) throw new Error(`create first revision failed: ${firstRes.status}`);
    const first = await firstRes.json();

    const sessionRes = await fetch('/auth/session', { credentials: 'include' });
    if (!sessionRes.ok) throw new Error(`session failed: ${sessionRes.status}`);
    const session = await sessionRes.json();
    const owner = session.user?.id || session.user?.email || 'guest';
    localStorage.setItem(`choir:vtext:draft:${owner}:${doc.doc_id}`, JSON.stringify({
      doc_id: doc.doc_id,
      parent_revision_id: first.revision_id,
      content: staleContent,
      updated_at: new Date().toISOString(),
    }));

    const currentContent = [
      '# Stale Draft Table',
      '',
      'Intro paragraph.',
      '',
      '| Term | Definition |',
      '| --- | --- |',
      '| Vector database | Stores embeddings for retrieval. |',
      '| Source entity | A citation-backed source object. |',
    ].join('\n');
    const secondRes = await fetch(`/api/vtext/documents/${encodeURIComponent(doc.doc_id)}/revisions`, {
      method: 'POST',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        content: currentContent,
        author_kind: 'user',
        author_label: 'browser-test',
        parent_revision_id: first.revision_id,
        metadata: { source: 'current_table_head' },
      }),
    });
    if (!secondRes.ok) throw new Error(`create current revision failed: ${secondRes.status}`);
    const second = await secondRes.json();
    return { doc, first, second, title: doc.title };
  }, stamp);

  await page.reload();
  await page.locator('[data-desktop]').waitFor({ state: 'visible', timeout: 10000 });
  await page.locator('[data-desktop-icon-id="vtext"]').dblclick();
  const vtextWindow = page.locator('[data-vtext-app]').last();
  await expect(vtextWindow.locator('[data-vtext-recent]')).toBeVisible({ timeout: 10000 });
  await vtextWindow.locator('[data-vtext-recent-document]').filter({ hasText: created.title }).click();

  const rendered = vtextWindow.locator('[data-vtext-rendered]');
  await expect(rendered.locator('.table-scroll table')).toBeVisible({ timeout: 10000 });
  await expect(rendered).toContainText('Source entity');
  await expect(vtextWindow.locator('[data-vtext-save-status]')).toContainText('Autosaved draft skipped; newer version loaded');
  await expect(vtextWindow.locator('[data-vtext-state]')).toContainText('Latest');
});

test('vtext does not restore same-head local draft that lost canonical tables', async ({ page, authenticator }) => {
  await registerAndLoadDesktop(page, authenticator, uniqueEmail());
  const stamp = Date.now();
  const created = await page.evaluate(async (stampValue) => {
    const docRes = await fetch('/api/vtext/documents', {
      method: 'POST',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ title: `Same Head Table Draft ${stampValue}` }),
    });
    if (!docRes.ok) throw new Error(`create doc failed: ${docRes.status}`);
    const doc = await docRes.json();
    const currentContent = [
      '# Same Head Table Draft',
      '',
      'Intro paragraph.',
      '',
      '| Term | Definition |',
      '| --- | --- |',
      '| Vector database | Stores embeddings for retrieval. |',
      '| Source entity | A citation-backed source object. |',
    ].join('\n');
    const revisionRes = await fetch(`/api/vtext/documents/${encodeURIComponent(doc.doc_id)}/revisions`, {
      method: 'POST',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        content: currentContent,
        author_kind: 'user',
        author_label: 'browser-test',
        metadata: { source: 'same_head_table_revision' },
      }),
    });
    if (!revisionRes.ok) throw new Error(`create current revision failed: ${revisionRes.status}`);
    const revision = await revisionRes.json();

    const collapsedContent = [
      '# Same Head Table Draft',
      '',
      'Intro paragraph.',
      '',
      'Term',
      'Definition',
      'Vector database',
      'Stores embeddings for retrieval.',
      'Source entity',
      'A citation-backed source object.',
    ].join('\n');
    const sessionRes = await fetch('/auth/session', { credentials: 'include' });
    if (!sessionRes.ok) throw new Error(`session failed: ${sessionRes.status}`);
    const session = await sessionRes.json();
    const owner = session.user?.id || session.user?.email || 'guest';
    localStorage.setItem(`choir:vtext:draft:${owner}:${doc.doc_id}`, JSON.stringify({
      doc_id: doc.doc_id,
      parent_revision_id: revision.revision_id,
      content: collapsedContent,
      updated_at: new Date().toISOString(),
    }));
    return { doc, revision, title: doc.title };
  }, stamp);

  await page.reload();
  await page.locator('[data-desktop]').waitFor({ state: 'visible', timeout: 10000 });
  await page.locator('[data-desktop-icon-id="vtext"]').dblclick();
  const vtextWindow = page.locator('[data-vtext-app]').last();
  await expect(vtextWindow.locator('[data-vtext-recent]')).toBeVisible({ timeout: 10000 });
  await vtextWindow.locator('[data-vtext-recent-document]').filter({ hasText: created.title }).click();

  const rendered = vtextWindow.locator('[data-vtext-rendered]');
  await expect(rendered.locator('.table-scroll table')).toBeVisible({ timeout: 10000 });
  await expect(rendered).toContainText('Source entity');
  await expect(vtextWindow.locator('[data-vtext-save-status]')).toContainText('Autosaved draft skipped; canonical table structure loaded');
  await expect(vtextWindow.locator('[data-vtext-state]')).toContainText('Latest');
});

test('vtext compares historical version and accepts merge preview as next revision', async ({ page, authenticator }) => {
  await registerAndLoadDesktop(page, authenticator, uniqueEmail());
  const fileName = `semantic-merge-${Date.now()}.md`;
  const initialContent = [
    '# Legal Cloud Proposal',
    '',
    'Earlier executive framing has the clearest problem statement.',
    '',
    '## Glossary',
    '',
    '- Matter workspace: a durable client work surface.',
  ].join('\n');
  const latestContent = [
    '# Legal Cloud Proposal',
    '',
    'Latest draft adds newer support and a source marker [1].',
    '',
    '## Conclusion',
    '',
    'The newest conclusion should remain in the primary draft.',
  ].join('\n');

  await seedTextFile(page, fileName, initialContent);
  const opened = await openFileInVText(page, fileName);
  await createExternalRevision(page, opened.doc_id, opened.current_revision_id, latestContent);

  const editor = page.locator('[data-vtext-app] [data-vtext-editor-area]').last();
  await expect(editor).toContainText('Latest draft adds newer support', { timeout: 10000 });

  await page.locator('[data-vtext-app] [data-vtext-prev]').last().click();
  await expect(editor).toContainText('Earlier executive framing', { timeout: 10000 });
  await page.locator('[data-vtext-app] [data-vtext-compare]').last().click();
  await expect(page.locator('[data-vtext-app] [data-vtext-compare-panel]').last()).toContainText(/Compare|Model compare|changed/i, { timeout: 30000 });
  await expect(page.locator('[data-vtext-app] [data-vtext-merge-suggestion]').first()).toBeVisible({ timeout: 30000 });
  await page.locator('[data-vtext-app] [data-vtext-merge-preview]').last().click();
  await expect(page.locator('[data-vtext-app] [data-vtext-compare-panel]').last()).toContainText(/Merge preview|Model merge|Merged into/i, { timeout: 30000 });
  await expect(editor).toContainText('newest conclusion should remain', { timeout: 10000 });
  await expect(editor).not.toContainText('VText merge preview provenance');
  await page.locator('[data-vtext-app] [data-vtext-accept-merge]').last().click();

  const revisions = await waitForRevisionTotal(page, opened.doc_id, 3, 12000);
  const currentDoc = await page.evaluate(async (docId) => {
    const res = await fetch(`/api/vtext/documents/${encodeURIComponent(docId)}`, {
      method: 'GET',
      credentials: 'include',
    });
    if (!res.ok) {
      throw new Error(`failed to load current doc: ${res.status}`);
    }
    return res.json();
  }, opened.doc_id);
  const accepted = revisions.revisions.find((revision) => revision.revision_id === currentDoc.current_revision_id);
  expect(accepted.metadata?.source).toBe('vtext_concept_merge');
  expect(accepted.metadata?.draft_line?.name).toBe('Primary draft');
  expect(accepted.content).not.toContain('VText merge preview provenance');
});

test('reopening the same file path resolves to the same canonical vtext doc', async ({ page, authenticator }) => {
  await registerAndLoadDesktop(page, authenticator, uniqueEmail());
  const fileName = `canonical-alias-${Date.now()}.txt`;
  const initialContent = 'Alias seed content';

  await seedTextFile(page, fileName, initialContent);

  const firstOpen = await openFileInVText(page, fileName);
  expect(firstOpen.created).toBe(true);

  const secondOpen = await openFileInVText(page, fileName);
  expect(secondOpen.created).toBe(false);
  expect(secondOpen.doc_id).toBe(firstOpen.doc_id);

  const revisions = await page.evaluate(async (docId) => {
    const res = await fetch(`/api/vtext/documents/${encodeURIComponent(docId)}/revisions`, {
      method: 'GET',
      credentials: 'include',
    });
    if (!res.ok) {
      throw new Error(`failed to list revisions: ${res.status}`);
    }
    return res.json();
  }, firstOpen.doc_id);
  expect(revisions.revisions).toHaveLength(1);
  expect(revisions.revisions[0].content).toBe(initialContent);
});

test('vtext file-backed window restores on reload with the latest canonical head', async ({ page, authenticator }) => {
  await registerAndLoadDesktop(page, authenticator, uniqueEmail());
  const fileName = `restart-recovery-${Date.now()}.txt`;
  const initialContent = 'Initial restart content';
  const externalContent = 'Recovered latest head after reload';

  await seedTextFile(page, fileName, initialContent);
  const opened = await openFileInVText(page, fileName);

  const editor = page.locator('[data-vtext-app] [data-vtext-editor-area]').last();
  await expect(editor).toContainText(initialContent);

  await createExternalRevision(page, opened.doc_id, opened.current_revision_id, externalContent);
  await expect(editor).toContainText(externalContent, { timeout: 10000 });

  await page.waitForTimeout(1000);
  await page.reload();
  await page.locator('[data-desktop]').waitFor({ state: 'visible', timeout: 10000 });
  await page.waitForTimeout(1500);

  const restoredEditor = page.locator('[data-vtext-app] [data-vtext-editor-area]').last();
  await expect(restoredEditor).toContainText(externalContent, { timeout: 10000 });
});

test('dry-run test endpoint: submit_research_findings batches rapid worker updates into one auto-advanced next version', async ({ page, authenticator }) => {
  test.skip(
    process.env.GO_CHOIR_RUN_VTEXT_DRY_RUN_TESTS !== '1',
    'uses /api/test/vtext/research-findings and is only a dry-run plumbing check'
  );
  await registerAndLoadDesktop(page, authenticator, uniqueEmail());
  const initialContent = 'Base draft that should get a findings-driven follow-up.';

  await openVText(page);
  const editor = page.locator('[data-vtext-app] [data-vtext-editor-area]').last();
  await editor.fill(initialContent);
  await expect(editor).toContainText(initialContent);

  const revisionRequest = page.waitForResponse((response) => {
    return response.request().method() === 'POST' &&
      /\/api\/vtext\/documents\/[^/]+\/revise$/.test(new URL(response.url()).pathname);
  });
  await page.locator('[data-vtext-prompt]').last().click();
  const revisionResponse = await revisionRequest;
  expect(revisionResponse.status()).toBe(202);
  const revisionJSON = await revisionResponse.json();
  await expect(page.locator('[data-vtext-save-status]').last()).toContainText(/First draft ready|Agent created next version/, { timeout: 10000 });

  const baselineRevisions = await listRevisions(page, revisionJSON.doc_id);
  const baselineCount = baselineRevisions.revisions.length;

  await submitTestResearchFindings(page, {
    doc_id: revisionJSON.doc_id,
    finding_id: `finding-a-${Date.now()}`,
    findings: ['Finding A: a new sourced detail arrived.'],
    notes: ['Use a brief update.'],
  });
  await submitTestResearchFindings(page, {
    doc_id: revisionJSON.doc_id,
    finding_id: `finding-b-${Date.now()}`,
    findings: ['Finding B: another sourced detail arrived right after.'],
    notes: ['Still one follow-up revision.'],
  });

  const afterWake = await waitForRevisionTotal(page, revisionJSON.doc_id, baselineCount + 1, 12000);
  expect(afterWake.revisions.length).toBe(baselineCount + 1);
  const currentDoc = await page.evaluate(async (docId) => {
    const res = await fetch(`/api/vtext/documents/${encodeURIComponent(docId)}`, {
      credentials: 'include',
    });
    if (!res.ok) {
      throw new Error(`failed to load document: ${res.status}`);
    }
    return res.json();
  }, revisionJSON.doc_id);
  const latestRevision = afterWake.revisions.find((revision) => revision.revision_id === currentDoc.current_revision_id);
  expect(latestRevision?.content || '').toContain('Finding A');
  expect(latestRevision?.content || '').toContain('Finding B');
  expect(latestRevision?.content || '').not.toMatch(/Research findings ready\.|Task completed successfully|stub provider/i);
  await expect(page.locator('[data-vtext-new-version]')).toHaveCount(0);
  await expect(page.locator('[data-vtext-version]').last()).toHaveText(`v${afterWake.revisions.length - 1}`);

  await page.waitForTimeout(4000);
  const stableRevisions = await listRevisions(page, revisionJSON.doc_id);
  expect(stableRevisions.revisions.length).toBe(baselineCount + 1);
});

test('dry-run test endpoint: submit_worker_update records artifacts and tests before auto-advancing vtext', async ({ page, authenticator }) => {
  test.skip(
    process.env.GO_CHOIR_RUN_VTEXT_DRY_RUN_TESTS !== '1',
    'uses /api/test/vtext/worker-update and is only a dry-run plumbing check'
  );
  await registerAndLoadDesktop(page, authenticator, uniqueEmail());

  await openVText(page);
  const editor = page.locator('[data-vtext-app] [data-vtext-editor-area]').last();
  await editor.fill('Base draft that needs a verified simulation artifact.');

  const revisionRequest = page.waitForResponse((response) => {
    return response.request().method() === 'POST' &&
      /\/api\/vtext\/documents\/[^/]+\/revise$/.test(new URL(response.url()).pathname);
  });
  await page.locator('[data-vtext-prompt]').last().click();
  const revisionJSON = await (await revisionRequest).json();
  await expect(page.locator('[data-vtext-save-status]').last()).toContainText(/First draft ready|Agent created next version/, { timeout: 10000 });

  const baselineRevisions = await listRevisions(page, revisionJSON.doc_id);
  const baselineCount = baselineRevisions.revisions.length;
  const updateId = `super-artifact-${Date.now()}`;

  const workerResp = await submitTestWorkerUpdate(page, {
    doc_id: revisionJSON.doc_id,
    update_id: updateId,
    role: 'super',
    artifacts: ['artifacts/evolution-ca.html'],
    tests: ['node artifacts/evolution-ca.verify.js passed'],
    proposals: ['Mention the verified cellular automata visualization in the next version.'],
  });
  expect(workerResp.status).toBe('submitted');
  expect(workerResp.loop_id).toBeTruthy();

  const afterWake = await waitForRevisionTotal(page, revisionJSON.doc_id, baselineCount + 1, 12000);
  const currentDoc = await page.evaluate(async (docId) => {
    const res = await fetch(`/api/vtext/documents/${encodeURIComponent(docId)}`, {
      credentials: 'include',
    });
    if (!res.ok) {
      throw new Error(`failed to load document: ${res.status}`);
    }
    return res.json();
  }, revisionJSON.doc_id);
  const latestRevision = afterWake.revisions.find((revision) => revision.revision_id === currentDoc.current_revision_id);
  expect(latestRevision).toBeTruthy();
  expect(latestRevision.content || '').toContain('artifacts/evolution-ca.html');
  expect(latestRevision.content || '').toContain('node artifacts/evolution-ca.verify.js passed');
  expect(latestRevision.content || '').not.toMatch(/Worker update ready\.|Task completed successfully|stub provider/i);
  const consumed = latestRevision.metadata.worker_updates_consumed || [];
  expect(consumed.some((item) =>
    item.seq === workerResp.cursor &&
    item.role === 'super' &&
    item.content_preview.includes('artifacts/evolution-ca.html') &&
    item.content_preview.includes('evolution-ca.verify.js passed')
  )).toBe(true);

  await expect(page.locator('[data-vtext-version]').last()).toHaveText(`v${afterWake.revisions.length - 1}`);
});
