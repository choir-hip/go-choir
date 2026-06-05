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

test('VText opens content-item text sources as reader-mode markdown', async ({ desktopSession }) => {
  const { page } = desktopSession;
  await page.evaluate(async () => {
    await fetch('/api/desktop/state', {
      method: 'PUT',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ windows: [], active_window_id: '' }),
    });
  });
  await page.reload();
  await expect(page.locator('[data-desktop]')).toBeVisible({ timeout: 10000 });
  await expect(page.locator('[data-window]')).toHaveCount(0);

  const created = await page.evaluate(async () => {
    const title = `Content Source Reader Fixture ${Date.now()}`;
    const contentRes = await fetch('/api/content/items', {
      method: 'POST',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        source_type: 'extracted_url',
        media_type: 'text/markdown',
        app_hint: 'content',
        title: 'Reader-mode source fixture',
        source_url: 'https://example.com/source-reader-fixture',
        canonical_url: 'https://example.com/source-reader-fixture',
        text_content: [
          '# Reader-mode source fixture',
          '',
          'Full cleaned reader source detail supports the cited claim.',
          '',
          '- First supporting point',
          '- Second supporting point',
          '',
          '| Field | Value |',
          '| --- | --- |',
          '| Evidence | Cleaned markdown |',
        ].join('\n'),
        provenance: {
          rights_scope: 'public_source',
          created_by: 'browser-test',
        },
      }),
    });
    if (!contentRes.ok) throw new Error(`create content item failed: ${contentRes.status}`);
    const item = await contentRes.json();
    const docRes = await fetch('/api/vtext/documents', {
      method: 'POST',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ title }),
    });
    if (!docRes.ok) throw new Error(`create doc failed: ${docRes.status}`);
    const doc = await docRes.json();
    const revRes = await fetch(`/api/vtext/documents/${encodeURIComponent(doc.doc_id)}/revisions`, {
      method: 'POST',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        content: '# Content Source Reader Fixture\n\nThis claim has a cleaned source [1](source:src-reader-mode).',
        author_kind: 'user',
        author_label: 'browser-test',
        metadata: {
          source_entities: [
            {
              entity_id: 'src-reader-mode',
              kind: 'content_item',
              label: 'Reader-mode source fixture',
              target: {
                target_kind: 'content_item',
                content_id: item.content_id,
                url: item.source_url,
                canonical_url: item.canonical_url,
              },
              selectors: [
                {
                  selector_kind: 'text_quote',
                  text_quote: 'Full cleaned reader source detail supports the cited claim.',
                  content_hash: item.content_hash,
                },
              ],
              display: {
                inline_mode: 'embedded_excerpt',
                expanded_mode: 'source_card',
                open_surface: 'content',
                default_collapsed: true,
              },
              evidence: {
                state: 'available',
                research_state: 'confirmed',
              },
              provenance: {
                created_by: 'browser-test',
                rights_scope: 'public_source',
                untrusted_source_text: true,
              },
            },
          ],
        },
      }),
    });
    if (!revRes.ok) throw new Error(`create revision failed: ${revRes.status}`);
    return { title, contentID: item.content_id };
  });

  await page.locator('[data-desktop-icon-id="vtext"]').dblclick();
  const vtextWindow = page.locator('[data-vtext-app]').last();
  await expect(vtextWindow.locator('[data-vtext-recent]')).toBeVisible({ timeout: 5000 });
  await vtextWindow.locator('[data-vtext-recent-document]').filter({ hasText: created.title }).click();

  const rendered = vtextWindow.locator('[data-vtext-rendered]');
  const citation = rendered.locator('[data-vtext-source-ref][data-source-entity-id="src-reader-mode"]');
  await expect(citation).toBeVisible({ timeout: 10000 });
  await citation.click();
  const flowNote = rendered.locator('[data-vtext-source-flow-note]');
  await expect(flowNote).toBeVisible();
  await flowNote.locator('[data-vtext-open-source][data-source-entity-id="src-reader-mode"]').click();

  const sourceWindow = page.locator('[data-content-viewer]').last();
  await expect(sourceWindow).toBeVisible({ timeout: 10000 });
  const reader = sourceWindow.locator('[data-content-reader-markdown]');
  await expect(reader).toBeVisible();
  await expect(reader.locator('h2')).toContainText('Reader-mode source fixture');
  await expect(reader.locator('li')).toHaveCount(2);
  await expect(reader.locator('table')).toContainText('Cleaned markdown');
  await expect(reader).toContainText('Full cleaned reader source detail');
  await expect(reader).not.toContainText(created.contentID);
  await expect(sourceWindow.locator('[data-content-evidence]')).toContainText('SHA-256');
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
              supports: 'ethics guidance',
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
      'Third paragraph gives the layout enough prose to continue below the source note after the narrow line region ends, using ordinary article text that should not become a separate card or metadata block.',
      'Fourth paragraph proves the article continues in the normal full measure once the source apparatus no longer occupies the right column.',
      'Fifth paragraph gives the verifier another full-width line after the note so the test cannot pass merely because one paragraph happened to wrap narrowly beside the source.',
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
  const note = flow.locator('[data-vtext-source-flow-note]');
  expect(await note.evaluate((node) => getComputedStyle(node).position)).toBe('absolute');
  await expect(flow).toHaveAttribute('data-vtext-source-flow-routed-lines', /^[3-9]\d*$/);
  const journalGeometry = await flow.evaluate((node) => {
    const note = node.querySelector('[data-vtext-source-flow-note]');
    const flowBox = node.getBoundingClientRect();
    const noteBox = note.getBoundingClientRect();
    const noteBottom = note.getBoundingClientRect().bottom - node.getBoundingClientRect().top;
    const besideLines = Array.from(node.querySelectorAll('[data-vtext-source-flow-line-beside-note]'));
    const besideLineCount = besideLines.length;
    const sideColumnIsClear = besideLines.every((line) => {
      const lineBox = line.getBoundingClientRect();
      return lineBox.right <= noteBox.left - 10;
    });
    const lowerWrappedLine = Array.from(node.querySelectorAll('.vtext-source-journal-line')).some((line) => {
      const top = line.getBoundingClientRect().top - node.getBoundingClientRect().top;
      return top > noteBottom * 0.45 && top < noteBottom && line.textContent.includes('Second paragraph');
    });
    return { besideLineCount, sideColumnIsClear, lowerWrappedLine };
  });
  const continuedBelowFlow = await rendered.evaluate((node) => {
    const flow = node.querySelector('[data-vtext-source-flow]');
    const followingParagraph = Array.from(node.querySelectorAll('p')).find((paragraph) => paragraph.textContent.includes('Third paragraph'));
    if (!flow || !followingParagraph) return false;
    const flowBox = flow.getBoundingClientRect();
    const paragraphBox = followingParagraph.getBoundingClientRect();
    return paragraphBox.top >= flowBox.bottom - 1;
  });
  expect(journalGeometry.besideLineCount).toBeGreaterThanOrEqual(3);
  expect(journalGeometry.sideColumnIsClear).toBe(true);
  expect(journalGeometry.lowerWrappedLine).toBe(true);
  expect(continuedBelowFlow).toBe(true);
  const noteFactStyle = await note.locator('.vtext-source-facts span').first().evaluate((node) => {
    const style = getComputedStyle(node);
    return {
      borderStyle: style.borderStyle,
      borderRadius: style.borderRadius,
      backgroundColor: style.backgroundColor,
    };
  });
  expect(noteFactStyle.borderStyle).toBe('none');
  expect(noteFactStyle.borderRadius).toBe('0px');
  expect(noteFactStyle.backgroundColor).toBe('rgba(0, 0, 0, 0)');
  const nestedCitation = flow.locator('[data-vtext-source-ref][data-source-entity-id="src-fixture-nested"]');
  await expect(nestedCitation).toBeVisible();
  await expect(nestedCitation.locator('[data-vtext-inline-transclusion]')).toBeHidden();
  await nestedCitation.click();
  const remountedFlow = rendered.locator('[data-vtext-source-flow]');
  await expect(remountedFlow).toHaveCount(1);
  await expect(remountedFlow.locator('[data-vtext-source-flow-note]')).toContainText('ABA Model Rule 1.6 fixture');
  await expect(remountedFlow.locator('[data-vtext-source-flow-note]')).not.toContainText('ABA Formal Opinion 512 fixture');
  const remountedState = await rendered.evaluate((node) => {
    const flow = node.querySelector('[data-vtext-source-flow]');
    const mounted = node.querySelector('[data-vtext-source-ref][data-source-entity-id="src-fixture-nested"][data-source-flow-mounted="true"]');
    const expandedInsideFlow = flow?.querySelector('[data-vtext-source-ref][data-expanded="true"]');
    return {
      owner: flow?.getAttribute('data-source-flow-owner-id') || '',
      hasMountedOriginal: !!mounted && !mounted.closest('[data-vtext-source-flow]'),
      hasExpandedInsideFlow: !!expandedInsideFlow,
    };
  });
  expect(remountedState.owner).toBe('src-fixture-nested');
  expect(remountedState.hasMountedOriginal).toBe(true);
  expect(remountedState.hasExpandedInsideFlow).toBe(false);

  await remountedFlow.locator('[data-vtext-open-source][data-source-entity-id="src-fixture-nested"]').click();
  const sourceWindow = page.locator('[data-window]').filter({ hasText: 'ABA Model Rule 1.6 fixture' }).last();
  await expect(sourceWindow).toBeVisible({ timeout: 10000 });
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
