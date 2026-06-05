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

  await citation.locator('[data-vtext-open-source]').click();
  await expect(page.locator('[data-window]').filter({ hasText: 'YouTube source fixture' }).last()).toBeVisible({ timeout: 10000 });
});

test('VText lays out expanded text sources as noncanonical journal flow', async ({ desktopSession }) => {
  const { page } = desktopSession;
  await page.setViewportSize({ width: 1440, height: 980 });
  const created = await page.evaluate(async () => {
    const title = `Source Flow Fixture ${Date.now()}`;
    const docRes = await fetch('/api/vtext/documents', {
      method: 'POST',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ title }),
    });
    if (!docRes.ok) throw new Error(`create doc failed: ${docRes.status}`);
    const doc = await docRes.json();
    const metadata = {
      source_entities: [
        {
          entity_id: 'src-fixture-flow',
          kind: 'ethics_opinion',
          label: 'ABA Formal Opinion 512 fixture',
          target: {
            target_kind: 'url',
            url: 'https://www.americanbar.org/groups/professional_responsibility/publications/ethics_opinions/aba-formal-opinion-512/',
            canonical_url: 'https://www.americanbar.org/groups/professional_responsibility/publications/ethics_opinions/aba-formal-opinion-512/',
          },
          selectors: [
            {
              selector_kind: 'text_quote',
              text_quote: 'Lawyers using generative artificial intelligence tools must consider duties including competence, confidentiality, communication, supervision, candor, and reasonable fees.',
            },
          ],
          display: {
            inline_mode: 'embedded_excerpt',
            expanded_mode: 'source_card',
            open_surface: 'source',
            default_collapsed: true,
          },
          evidence: { state: 'available', research_state: 'confirmed' },
          provenance: { created_by: 'browser-test', rights_scope: 'public_source' },
        },
        {
          entity_id: 'src-fixture-nested',
          kind: 'ethics_rule',
          label: 'ABA Model Rule 1.6 fixture',
          target: {
            target_kind: 'url',
            url: 'https://www.americanbar.org/groups/professional_responsibility/publications/model_rules_of_professional_conduct/rule_1_6_confidentiality_of_information/',
            canonical_url: 'https://www.americanbar.org/groups/professional_responsibility/publications/model_rules_of_professional_conduct/rule_1_6_confidentiality_of_information/',
          },
          selectors: [
            {
              selector_kind: 'text_quote',
              text_quote: 'A lawyer shall not reveal information relating to the representation of a client unless the client gives informed consent.',
            },
          ],
          display: {
            inline_mode: 'embedded_excerpt',
            expanded_mode: 'source_card',
            open_surface: 'source',
            default_collapsed: true,
          },
          evidence: { state: 'available', research_state: 'confirmed' },
          provenance: { created_by: 'browser-test', rights_scope: 'public_source' },
        },
      ],
    };
    const paragraphs = [
      [
        'Legal practice now depends on durable work product, governed source memory, and reliable citation review across long client documents.',
        '[ethics guidance](source:src-fixture-flow)',
      ].join(' '),
      'Second paragraph keeps using the reading measure beside the expanded evidence while preserving [confidentiality](source:src-fixture-nested) as its own citation marker rather than flattening it into prose.',
      'Third paragraph gives the layout enough prose to continue below the source note after the narrow line region ends.',
    ];
    const revRes = await fetch(`/api/vtext/documents/${encodeURIComponent(doc.doc_id)}/revisions`, {
      method: 'POST',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        content: `# Source Flow Fixture\n\n${paragraphs.join('\n\n')}`,
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
  const citation = rendered.locator('[data-vtext-source-ref][data-source-entity-id="src-fixture-flow"]').first();
  await expect(citation).toBeVisible({ timeout: 10000 });
  await citation.click();
  const flow = rendered.locator('[data-vtext-source-flow]');
  await expect(flow).toBeVisible({ timeout: 5000 });
  await expect(flow).toContainText('ABA Formal Opinion 512 fixture');
  await expect(flow).not.toContainText('source available');
  await expect(flow).not.toContainText('public source');
  await expect(citation).toHaveAttribute('data-source-flow-mounted', 'true');
  expect(await rendered.locator('p[data-vtext-source-flow-hidden]').count()).toBeGreaterThanOrEqual(2);
  expect(await flow.evaluate((node) => getComputedStyle(node).position)).toBe('relative');
  expect(await flow.locator('[data-vtext-source-flow-note]').evaluate((node) => getComputedStyle(node).position)).toBe('absolute');
  const lowerWrappedLine = await flow.evaluate((node) => {
    const note = node.querySelector('[data-vtext-source-flow-note]');
    const noteBottom = note.getBoundingClientRect().bottom - node.getBoundingClientRect().top;
    return Array.from(node.querySelectorAll('.vtext-source-journal-line')).some((line) => {
      const top = line.getBoundingClientRect().top - node.getBoundingClientRect().top;
      return top > noteBottom * 0.45 && top < noteBottom && line.textContent.includes('Second paragraph');
    });
  });
  expect(lowerWrappedLine).toBe(true);
  const nestedCitation = flow.locator('[data-vtext-source-ref][data-source-entity-id="src-fixture-nested"]');
  await expect(nestedCitation).toBeVisible();
  await nestedCitation.click();
  await expect(nestedCitation).toHaveAttribute('data-expanded', 'true');
  await expect(nestedCitation.locator('[data-vtext-inline-transclusion]')).toContainText('ABA Model Rule 1.6 fixture');
  await expect(flow).toBeVisible();

  await flow.locator('[data-vtext-open-source][data-source-entity-id="src-fixture-flow"]').click();
  const sourceWindow = page.locator('[data-window]').filter({ hasText: 'ABA Formal Opinion 512 fixture' }).last();
  await expect(sourceWindow).toBeVisible({ timeout: 10000 });
  await expect(sourceWindow.locator('[data-browser-reader-markdown]')).toContainText(
    'Lawyers using generative artificial intelligence tools must consider duties',
    { timeout: 10000 }
  );
  await expect(sourceWindow.locator('[data-browser-iframe]')).toHaveCount(0);
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

test('VText autosave preserves table structure when a bounded cell edit is made', async ({ desktopSession }) => {
  const { page } = desktopSession;
  const created = await page.evaluate(async () => {
    const title = `Bounded Table Edit Fixture ${Date.now()}`;
    const docRes = await fetch('/api/vtext/documents', {
      method: 'POST',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ title }),
    });
    if (!docRes.ok) throw new Error(`create doc failed: ${docRes.status}`);
    const doc = await docRes.json();
    const content = [
      '# Bounded Table Edit Fixture',
      '',
      '| Term | Definition |',
      '| --- | --- |',
      '| Work product | Durable professional output. |',
      '| Source entity | A citation-backed source object. |',
      '',
      'Only one table cell should change.',
    ].join('\n');
    const revRes = await fetch(`/api/vtext/documents/${encodeURIComponent(doc.doc_id)}/revisions`, {
      method: 'POST',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        content,
        author_kind: 'user',
        author_label: 'browser-test',
        metadata: { source_path: 'fixtures/bounded-table-edit.md', created_from: 'browser-test' },
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
  const editedDefinition = 'Durable, reviewable professional output with source memory.';
  await rendered.locator('tbody tr').first().locator('td').nth(1).evaluate((cell, text) => {
    cell.textContent = text;
    cell.closest('[data-vtext-rendered]')?.dispatchEvent(new InputEvent('input', {
      bubbles: true,
      inputType: 'insertText',
      data: text,
    }));
  }, editedDefinition);
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
  expect(draft?.content).toContain(`| Work product | ${editedDefinition} |`);
  expect(draft?.content).toContain('| Source entity | A citation-backed source object. |');
  expect(draft?.content).toContain('| --- | --- |');
  expect(draft?.content).not.toContain('TermDefinition');
});
