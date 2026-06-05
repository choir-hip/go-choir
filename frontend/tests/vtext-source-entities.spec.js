import { test, expect } from './helpers/fixtures.js';

test('VText renders source entities as expandable sources and opens owning media surface', async ({ desktopSession }) => {
  const { page } = desktopSession;
  const created = await page.evaluate(async () => {
    const title = `Source Entity Fixture ${Date.now()}`;
    const docRes = await fetch('/api/vtext/documents', {
      method: 'POST',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ title }),
    });
    if (!docRes.ok) throw new Error(`create doc failed: ${docRes.status}`);
    const doc = await docRes.json();
    const sourceURL = 'https://www.youtube.com/watch?v=dQw4w9WgXcQ';
    const metadata = {
      source_entities: [
        {
          entity_id: 'src-fixture-youtube',
          kind: 'youtube_video',
          label: 'YouTube source fixture',
          target: {
            target_kind: 'content_item',
            url: sourceURL,
            canonical_url: sourceURL,
          },
          selectors: [{ selector_kind: 'whole_resource' }],
          display: {
            inline_mode: 'embedded_preview',
            expanded_mode: 'media_player',
            open_surface: 'video',
            default_collapsed: true,
          },
          evidence: {
            state: 'available',
            research_state: 'pending',
            transcript_availability: 'unavailable',
          },
          provenance: {
            created_by: 'importer',
            rights_scope: 'private_user_source',
            untrusted_source_text: true,
          },
        },
      ],
    };
    const revRes = await fetch(`/api/vtext/documents/${encodeURIComponent(doc.doc_id)}/revisions`, {
      method: 'POST',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        content: `# Source Entity Fixture\n\nReview this [source](source:src-fixture-youtube): ${sourceURL}`,
        author_kind: 'user',
        author_label: 'browser-test',
        metadata,
      }),
    });
    if (!revRes.ok) throw new Error(`create rev failed: ${revRes.status}`);
    return doc;
  });

  await page.locator('[data-desktop-icon-id="vtext"]').dblclick();
  const vtextWindow = page.locator('[data-vtext-app]').last();
  await expect(vtextWindow.locator('[data-vtext-recent]')).toBeVisible({ timeout: 5000 });
  await vtextWindow.locator('[data-vtext-recent-document]').filter({ hasText: created.title }).click();

  const rendered = vtextWindow.locator('[data-vtext-rendered]');
  await expect(rendered.locator('[data-vtext-source-ref]')).toBeVisible({ timeout: 10000 });
  await expect(rendered.locator('[data-vtext-source-ref]')).toHaveAttribute('data-vtext-citation-transclusion', '');
  await rendered.locator('[data-vtext-source-ref]').click();
  const citation = rendered.locator('[data-vtext-source-ref]');
  await expect(citation).toHaveAttribute('data-expanded', 'true');
  await expect(citation.locator('[data-vtext-inline-transclusion]')).toContainText('YouTube source fixture');
  await expect(citation.locator('[data-vtext-inline-transclusion] iframe')).toHaveAttribute('src', /youtube\.com\/embed\/dQw4w9WgXcQ/);
  await expect(citation.locator('[data-vtext-inline-transclusion]')).toContainText('transcript unavailable');
  await expect(rendered.locator('[data-vtext-source-inline]')).toHaveAttribute('data-vtext-display-policy', 'embedded_preview');

  const initialVideoWindows = await page.locator('[data-video-app]').count();
  await citation.locator('[data-vtext-open-source]').click();
  await expect(page.locator('[data-video-app]')).toHaveCount(initialVideoWindows + 1, { timeout: 10000 });
  await expect(page.locator('[data-video-frame]').last()).toHaveAttribute('src', /youtube\.com\/embed\/dQw4w9WgXcQ/);
});

test('VText autosave roundtrips rendered markdown tables without flattening cells', async ({ desktopSession }) => {
  const { page } = desktopSession;
  const created = await page.evaluate(async () => {
    const title = `Table Roundtrip Fixture ${Date.now()}`;
    const docRes = await fetch('/api/vtext/documents', {
      method: 'POST',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ title }),
    });
    if (!docRes.ok) throw new Error(`create doc failed: ${docRes.status}`);
    const doc = await docRes.json();
    const content = [
      '# Table Roundtrip Fixture',
      '',
      '| Term | Definition |',
      '| --- | --- |',
      '| Tokens per second | A measure of inference speed. |',
      '| Vector database | A database optimized for vector search. |',
      '',
      'Edit this paragraph to trigger serialization.',
    ].join('\n');
    const revRes = await fetch(`/api/vtext/documents/${encodeURIComponent(doc.doc_id)}/revisions`, {
      method: 'POST',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        content,
        author_kind: 'user',
        author_label: 'browser-test',
        metadata: { source_path: 'fixtures/table-roundtrip.md', created_from: 'browser-test' },
      }),
    });
    if (!revRes.ok) throw new Error(`create rev failed: ${revRes.status}`);
    return doc;
  });

  await page.locator('[data-desktop-icon-id="vtext"]').dblclick();
  const vtextWindow = page.locator('[data-vtext-app]').last();
  await expect(vtextWindow.locator('[data-vtext-recent]')).toBeVisible({ timeout: 5000 });
  await vtextWindow.locator('[data-vtext-recent-document]').filter({ hasText: created.title }).click();

  const rendered = vtextWindow.locator('[data-vtext-rendered]');
  await expect(rendered.locator('.table-scroll table')).toBeVisible({ timeout: 10000 });
  await expect(rendered).toContainText('Edit this paragraph to trigger serialization.');
  await rendered.click();
  await page.keyboard.press('End');
  await page.keyboard.type(' ');
  await expect(rendered.locator('.table-scroll table')).toBeVisible();
  await page.waitForTimeout(1300);

  const draft = await page.evaluate((docId) => {
    for (let i = 0; i < localStorage.length; i += 1) {
      const key = localStorage.key(i) || '';
      if (!key.includes(`:${docId}`)) continue;
      const value = JSON.parse(localStorage.getItem(key) || '{}');
      if (value?.doc_id === docId) return value;
    }
    return null;
  }, created.doc_id);
  expect(draft?.content).toContain('| Term | Definition |');
  expect(draft?.content).toContain('| Tokens per second | A measure of inference speed. |');
  expect(draft?.content).not.toContain('TermDefinition');
});
