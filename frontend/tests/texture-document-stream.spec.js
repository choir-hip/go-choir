import { test, expect } from './helpers/fixtures.js';
import { registerPasskey } from './helpers/auth.js';

const BASE_URL = 'http://localhost:4173';

function uniqueEmail() {
  return `texture-stream-${Date.now()}-${Math.random().toString(36).slice(2, 8)}@example.com`;
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

async function openTexture(page) {
  await page.locator('[data-desktop-icon-id="texture"]').dblclick();
  await page.locator('[data-texture-editor]').last().waitFor({ state: 'visible', timeout: 10000 });
  const recent = page.locator('[data-texture-app] [data-texture-recent]').last();
  if (await recent.isVisible().catch(() => false)) {
    await page.locator('[data-texture-app] [data-texture-new-document]').last().click();
  }
  const editor = page.locator('[data-texture-app] [data-texture-editor-area]').last();
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

async function openFileInTexture(page, fileName) {
  const filesWindow = await openFilesApp(page);
  const openResponse = page.waitForResponse((response) => {
    const url = new URL(response.url());
    return response.request().method() === 'POST' && url.pathname === '/api/texture/files/open';
  });
  const fileItem = filesWindow.locator('[data-file-item]').filter({ hasText: fileName }).first();
  await expect(fileItem).toBeVisible({ timeout: 5000 });
  await fileItem.click();
  await page.locator('[data-texture-app]').last().waitFor({ state: 'visible', timeout: 10000 });
  return (await openResponse).json();
}

async function createExternalRevision(page, docId, parentRevisionId, content) {
  return page.evaluate(async ({ docId, parentRevisionId, content }) => {
    const res = await fetch(`/api/texture/documents/${encodeURIComponent(docId)}/revisions`, {
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
    const res = await fetch(`/api/texture/documents/${encodeURIComponent(docIdValue)}/revisions`, {
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

async function submitTestWorkerUpdate(page, payload) {
  return page.evaluate(async (body) => {
    const res = await fetch('/api/test/texture/worker-update', {
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

test('texture auto-follows latest head when the editor is clean', async ({ page, authenticator }) => {
  await registerAndLoadDesktop(page, authenticator, uniqueEmail());
  const fileName = `auto-follow-${Date.now()}.txt`;
  const initialContent = 'Initial version from file open';
  const externalContent = 'External clean-head update';

  await seedTextFile(page, fileName, initialContent);
  const opened = await openFileInTexture(page, fileName);

  const editor = page.locator('[data-texture-app] [data-texture-editor-area]').last();
  await expect(editor).toContainText(initialContent);

  await createExternalRevision(page, opened.doc_id, opened.current_revision_id, externalContent);

  await expect(editor).toContainText(externalContent, { timeout: 10000 });
  await expect(page.locator('[data-texture-new-version]')).toHaveCount(0);
});

test('texture autosaves dirty text without advancing versions when the head moves', async ({ page, authenticator }) => {
  await registerAndLoadDesktop(page, authenticator, uniqueEmail());
  const fileName = `dirty-rebase-${Date.now()}.txt`;
  const initialContent = 'Seed content from file open';
  const dirtyContent = 'Local dirty draft that must persist over the moved head';
  const externalContent = 'External moved-head update that must survive rebase';

  await seedTextFile(page, fileName, initialContent);
  const opened = await openFileInTexture(page, fileName);

  const editor = page.locator('[data-texture-app] [data-texture-editor-area]').last();
  await editor.fill(dirtyContent);

  await createExternalRevision(page, opened.doc_id, opened.current_revision_id, externalContent);

  await expect(editor).toContainText(dirtyContent);
  await page.waitForTimeout(1400);

  const revisions = await listRevisions(page, opened.doc_id);
  const currentDoc = await page.evaluate(async (docId) => {
    const res = await fetch(`/api/texture/documents/${encodeURIComponent(docId)}`, {
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
  await expect(page.locator('[data-texture-new-version]')).toHaveCount(1);
});

test('texture does not restore stale local draft over a newer canonical table head', async ({ page, authenticator }) => {
  await registerAndLoadDesktop(page, authenticator, uniqueEmail());
  const stamp = Date.now();
  const created = await page.evaluate(async (stampValue) => {
    const docRes = await fetch('/api/texture/documents', {
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
    const firstRes = await fetch(`/api/texture/documents/${encodeURIComponent(doc.doc_id)}/revisions`, {
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
    localStorage.setItem(`choir:texture:draft:${owner}:${doc.doc_id}`, JSON.stringify({
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
    const secondRes = await fetch(`/api/texture/documents/${encodeURIComponent(doc.doc_id)}/revisions`, {
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
  await page.locator('[data-desktop-icon-id="texture"]').dblclick();
  const textureWindow = page.locator('[data-texture-app]').last();
  await expect(textureWindow.locator('[data-texture-recent]')).toBeVisible({ timeout: 10000 });
  await textureWindow.locator('[data-texture-recent-document]').filter({ hasText: created.title }).click();

  const rendered = textureWindow.locator('[data-texture-rendered]');
  await expect(rendered.locator('.table-scroll table')).toBeVisible({ timeout: 10000 });
  await expect(rendered).toContainText('Source entity');
  await expect(textureWindow.locator('[data-texture-save-status]')).toContainText('Autosaved draft skipped; newer version loaded');
  await expect(textureWindow.locator('[data-texture-state]')).toContainText('Latest');
});

test('texture does not restore same-head local draft that lost canonical tables', async ({ page, authenticator }) => {
  await registerAndLoadDesktop(page, authenticator, uniqueEmail());
  const stamp = Date.now();
  const created = await page.evaluate(async (stampValue) => {
    const docRes = await fetch('/api/texture/documents', {
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
    const revisionRes = await fetch(`/api/texture/documents/${encodeURIComponent(doc.doc_id)}/revisions`, {
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
    localStorage.setItem(`choir:texture:draft:${owner}:${doc.doc_id}`, JSON.stringify({
      doc_id: doc.doc_id,
      parent_revision_id: revision.revision_id,
      content: collapsedContent,
      updated_at: new Date().toISOString(),
    }));
    return { doc, revision, title: doc.title };
  }, stamp);

  await page.reload();
  await page.locator('[data-desktop]').waitFor({ state: 'visible', timeout: 10000 });
  await page.locator('[data-desktop-icon-id="texture"]').dblclick();
  const textureWindow = page.locator('[data-texture-app]').last();
  await expect(textureWindow.locator('[data-texture-recent]')).toBeVisible({ timeout: 10000 });
  await textureWindow.locator('[data-texture-recent-document]').filter({ hasText: created.title }).click();

  const rendered = textureWindow.locator('[data-texture-rendered]');
  await expect(rendered.locator('.table-scroll table')).toBeVisible({ timeout: 10000 });
  await expect(rendered).toContainText('Source entity');
  await expect(textureWindow.locator('[data-texture-save-status]')).toContainText('Autosaved draft skipped; canonical table structure loaded');
  await expect(textureWindow.locator('[data-texture-state]')).toContainText('Latest');
});

test('texture does not auto-restore a differing local draft over a non-empty canonical head', async ({ page, authenticator }) => {
  await registerAndLoadDesktop(page, authenticator, uniqueEmail());
  const stamp = Date.now();
  const created = await page.evaluate(async (stampValue) => {
    const docRes = await fetch('/api/texture/documents', {
      method: 'POST',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ title: `Canonical Draft Skip ${stampValue}` }),
    });
    if (!docRes.ok) throw new Error(`create doc failed: ${docRes.status}`);
    const doc = await docRes.json();
    const currentContent = [
      '# Canonical Draft Skip',
      '',
      'This is the saved canonical paragraph.',
      '',
      'The browser cache has a different local draft.',
    ].join('\n');
    const revisionRes = await fetch(`/api/texture/documents/${encodeURIComponent(doc.doc_id)}/revisions`, {
      method: 'POST',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        content: currentContent,
        author_kind: 'user',
        author_label: 'browser-test',
        metadata: { source: 'canonical_draft_skip' },
      }),
    });
    if (!revisionRes.ok) throw new Error(`create current revision failed: ${revisionRes.status}`);
    const revision = await revisionRes.json();

    const sessionRes = await fetch('/auth/session', { credentials: 'include' });
    if (!sessionRes.ok) throw new Error(`session failed: ${sessionRes.status}`);
    const session = await sessionRes.json();
    const owner = session.user?.id || session.user?.email || 'guest';
    localStorage.setItem(`choir:texture:draft:${owner}:${doc.doc_id}`, JSON.stringify({
      doc_id: doc.doc_id,
      parent_revision_id: revision.revision_id,
      content: [
        '# Canonical Draft Skip',
        '',
        'This is the stale browser-local paragraph.',
        '',
        'It must not mask the saved canonical revision.',
      ].join('\n'),
      updated_at: new Date().toISOString(),
    }));
    return { doc, revision, title: doc.title };
  }, stamp);

  await page.reload();
  await page.locator('[data-desktop]').waitFor({ state: 'visible', timeout: 10000 });
  await page.locator('[data-desktop-icon-id="texture"]').dblclick();
  const textureWindow = page.locator('[data-texture-app]').last();
  await expect(textureWindow.locator('[data-texture-recent]')).toBeVisible({ timeout: 10000 });
  await textureWindow.locator('[data-texture-recent-document]').filter({ hasText: created.title }).click();

  const rendered = textureWindow.locator('[data-texture-rendered]');
  await expect(rendered).toContainText('This is the saved canonical paragraph.', { timeout: 10000 });
  await expect(rendered).not.toContainText('This is the stale browser-local paragraph.');
  await expect(textureWindow.locator('[data-texture-save-status]')).toContainText('Autosaved draft skipped; canonical version loaded');
  await expect(textureWindow.locator('[data-texture-state]')).toContainText('Latest');
});

test('texture compares historical version and accepts merge preview as next revision', async ({ page, authenticator }) => {
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
  const opened = await openFileInTexture(page, fileName);
  await createExternalRevision(page, opened.doc_id, opened.current_revision_id, latestContent);

  const editor = page.locator('[data-texture-app] [data-texture-editor-area]').last();
  await expect(editor).toContainText('Latest draft adds newer support', { timeout: 10000 });

  await page.locator('[data-texture-app] [data-texture-prev]').last().click();
  await expect(editor).toContainText('Earlier executive framing', { timeout: 10000 });
  await page.locator('[data-texture-app] [data-texture-compare]').last().click();
  await expect(page.locator('[data-texture-app] [data-texture-compare-panel]').last()).toContainText(/Compare|Model compare|changed/i, { timeout: 30000 });
  await expect(page.locator('[data-texture-app] [data-texture-merge-suggestion]').first()).toBeVisible({ timeout: 30000 });
  await page.locator('[data-texture-app] [data-texture-merge-preview]').last().click();
  await expect(page.locator('[data-texture-app] [data-texture-compare-panel]').last()).toContainText(/Merge preview|Model merge|Merged into/i, { timeout: 30000 });
  await expect(editor).toContainText('newest conclusion should remain', { timeout: 10000 });
  await expect(editor).not.toContainText('Texture merge preview provenance');
  await page.locator('[data-texture-app] [data-texture-accept-merge]').last().click();

  const revisions = await waitForRevisionTotal(page, opened.doc_id, 3, 12000);
  const currentDoc = await page.evaluate(async (docId) => {
    const res = await fetch(`/api/texture/documents/${encodeURIComponent(docId)}`, {
      method: 'GET',
      credentials: 'include',
    });
    if (!res.ok) {
      throw new Error(`failed to load current doc: ${res.status}`);
    }
    return res.json();
  }, opened.doc_id);
  const accepted = revisions.revisions.find((revision) => revision.revision_id === currentDoc.current_revision_id);
  expect(accepted.metadata?.source).toBe('texture_concept_merge');
  expect(accepted.metadata?.draft_line?.name).toBe('Primary draft');
  expect(accepted.content).not.toContain('Texture merge preview provenance');
});

test('reopening the same file path resolves to the same canonical texture doc', async ({ page, authenticator }) => {
  await registerAndLoadDesktop(page, authenticator, uniqueEmail());
  const fileName = `canonical-alias-${Date.now()}.txt`;
  const initialContent = 'Alias seed content';

  await seedTextFile(page, fileName, initialContent);

  const firstOpen = await openFileInTexture(page, fileName);
  expect(firstOpen.created).toBe(true);

  const secondOpen = await openFileInTexture(page, fileName);
  expect(secondOpen.created).toBe(false);
  expect(secondOpen.doc_id).toBe(firstOpen.doc_id);

  const revisions = await page.evaluate(async (docId) => {
    const res = await fetch(`/api/texture/documents/${encodeURIComponent(docId)}/revisions`, {
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

test('texture file-backed window restores on reload with the latest canonical head', async ({ page, authenticator }) => {
  await registerAndLoadDesktop(page, authenticator, uniqueEmail());
  const fileName = `restart-recovery-${Date.now()}.txt`;
  const initialContent = 'Initial restart content';
  const externalContent = 'Recovered latest head after reload';

  await seedTextFile(page, fileName, initialContent);
  const opened = await openFileInTexture(page, fileName);

  const editor = page.locator('[data-texture-app] [data-texture-editor-area]').last();
  await expect(editor).toContainText(initialContent);

  await createExternalRevision(page, opened.doc_id, opened.current_revision_id, externalContent);
  await expect(editor).toContainText(externalContent, { timeout: 10000 });

  await page.waitForTimeout(1000);
  await page.reload();
  await page.locator('[data-desktop]').waitFor({ state: 'visible', timeout: 10000 });
  await page.waitForTimeout(1500);

  const restoredEditor = page.locator('[data-texture-app] [data-texture-editor-area]').last();
  await expect(restoredEditor).toContainText(externalContent, { timeout: 10000 });
});

test('dry-run test endpoint: researcher source packets batch into one auto-advanced next version', async ({ page, authenticator }) => {
  test.skip(
    process.env.GO_CHOIR_RUN_TEXTURE_DRY_RUN_TESTS !== '1',
    'uses /api/test/texture/worker-update and is only a dry-run plumbing check'
  );
  await registerAndLoadDesktop(page, authenticator, uniqueEmail());
  const initialContent = 'Base draft that should get a findings-driven follow-up.';

  await openTexture(page);
  const editor = page.locator('[data-texture-app] [data-texture-editor-area]').last();
  await editor.fill(initialContent);
  await expect(editor).toContainText(initialContent);

  const revisionRequest = page.waitForResponse((response) => {
    return response.request().method() === 'POST' &&
      /\/api\/texture\/documents\/[^/]+\/revise$/.test(new URL(response.url()).pathname);
  });
  await page.locator('[data-texture-prompt]').last().click();
  const revisionResponse = await revisionRequest;
  expect(revisionResponse.status()).toBe(202);
  const revisionJSON = await revisionResponse.json();
  await expect(page.locator('[data-texture-save-status]').last()).toContainText(/First draft ready|Agent created next version/, { timeout: 10000 });

  const baselineRevisions = await listRevisions(page, revisionJSON.doc_id);
  const baselineCount = baselineRevisions.revisions.length;

  await submitTestWorkerUpdate(page, {
    doc_id: revisionJSON.doc_id,
    role: 'researcher',
    schema_version: 'coagent_source_packet.v1',
    kind: 'evidence_update',
    summary: `research-a-${Date.now()}`,
    claims: [{ text: 'Finding A: a new sourced detail arrived.', source_ids: ['src-finding-a'] }],
    sources: [{
      source_id: 'src-finding-a',
      kind: 'web_page',
      target: { uri: 'https://example.test/finding-a', title: 'Finding A' },
      selectors: [{ kind: 'text_quote', quote: 'Finding A' }],
    }],
    notes: ['Use a brief update.'],
  });
  await submitTestWorkerUpdate(page, {
    doc_id: revisionJSON.doc_id,
    role: 'researcher',
    schema_version: 'coagent_source_packet.v1',
    kind: 'evidence_update',
    summary: `research-b-${Date.now()}`,
    claims: [{ text: 'Finding B: another sourced detail arrived right after.', source_ids: ['src-finding-b'] }],
    sources: [{
      source_id: 'src-finding-b',
      kind: 'web_page',
      target: { uri: 'https://example.test/finding-b', title: 'Finding B' },
      selectors: [{ kind: 'text_quote', quote: 'Finding B' }],
    }],
    notes: ['Still one follow-up revision.'],
  });

  const afterWake = await waitForRevisionTotal(page, revisionJSON.doc_id, baselineCount + 1, 12000);
  expect(afterWake.revisions.length).toBe(baselineCount + 1);
  const currentDoc = await page.evaluate(async (docId) => {
    const res = await fetch(`/api/texture/documents/${encodeURIComponent(docId)}`, {
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
  await expect(page.locator('[data-texture-new-version]')).toHaveCount(0);
  await expect(page.locator('[data-texture-version]').last()).toHaveText(`v${afterWake.revisions.length - 1}`);

  await page.waitForTimeout(4000);
  const stableRevisions = await listRevisions(page, revisionJSON.doc_id);
  expect(stableRevisions.revisions.length).toBe(baselineCount + 1);
});

test('dry-run test endpoint: submit_worker_update records artifacts and tests before auto-advancing texture', async ({ page, authenticator }) => {
  test.skip(
    process.env.GO_CHOIR_RUN_TEXTURE_DRY_RUN_TESTS !== '1',
    'uses /api/test/texture/worker-update and is only a dry-run plumbing check'
  );
  await registerAndLoadDesktop(page, authenticator, uniqueEmail());

  await openTexture(page);
  const editor = page.locator('[data-texture-app] [data-texture-editor-area]').last();
  await editor.fill('Base draft that needs a verified simulation artifact.');

  const revisionRequest = page.waitForResponse((response) => {
    return response.request().method() === 'POST' &&
      /\/api\/texture\/documents\/[^/]+\/revise$/.test(new URL(response.url()).pathname);
  });
  await page.locator('[data-texture-prompt]').last().click();
  const revisionJSON = await (await revisionRequest).json();
  await expect(page.locator('[data-texture-save-status]').last()).toContainText(/First draft ready|Agent created next version/, { timeout: 10000 });

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
    const res = await fetch(`/api/texture/documents/${encodeURIComponent(docId)}`, {
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

  await expect(page.locator('[data-texture-version]').last()).toHaveText(`v${afterWake.revisions.length - 1}`);
});
