import { test, expect } from './helpers/fixtures.js';
import {
  mediaRefToSourceEntity,
  normalizeSourceEvidenceState,
  sourceEntityInlineExcerptText,
  sourceEntityOpenPlan,
  sourceEvidenceState,
  sourceEvidenceStateLabel,
} from '../src/lib/vtext-source-renderer.ts';
import { buildSourceReviewPayload } from '../src/lib/vtext-source-review.js';

test('source review URL repairs default to Source Viewer open surface', () => {
  const payload = buildSourceReviewPayload({
    marker: '[1]',
    title: 'Source review URL fixture',
    excerpt: 'The cited source confirms the claim.',
    url: 'https://example.com/source-review-url-fixture',
    revisionID: 'rev-url-source-viewer',
    relation: 'confirms',
  });

  expect(payload.source_entities).toHaveLength(1);
  expect(payload.source_entities[0]).toMatchObject({
    kind: 'web_source',
    target: {
      target_kind: 'url',
      url: 'https://example.com/source-review-url-fixture',
    },
    display: {
      open_surface: 'source',
    },
  });
});

test('source inline excerpts prefer selected transclusion over full reader snapshot', () => {
  const entity = {
    entity_id: 'src-inline-reader-snapshot',
    label: 'Inline reader snapshot fixture',
    transclusion: {
      snapshot_text: 'Selected bounded citation excerpt.',
    },
    reader_snapshot: {
      text_content: [
        'Selected bounded citation excerpt.',
        'Full cleaned reader source detail should remain in the source window instead of the inline note.',
      ].join('\n\n'),
    },
  };

  expect(sourceEntityInlineExcerptText(entity)).toBe('Selected bounded citation excerpt.');
});

test('source evidence states normalize to typed reader labels', () => {
  expect(normalizeSourceEvidenceState('pending')).toBe('candidate');
  expect(normalizeSourceEvidenceState('no-source-needed')).toBe('no_source_needed');
  expect(normalizeSourceEvidenceState('access-blocked')).toBe('blocked_by_access');
  expect(sourceEvidenceState({ evidence: { state: 'represented' } })).toBe('confirms');
  expect(sourceEvidenceStateLabel('confirms')).toBe('Confirms claim');
  expect(sourceEvidenceStateLabel('blocked_by_access')).toBe('Blocked by access');

  const mediaEntity = mediaRefToSourceEntity({ kind: 'image', url: 'https://example.com/source.png' });
  expect(mediaEntity?.evidence?.state).toBe('candidate');
});

test('source open plans normalize Web Lens and Source Viewer aliases', () => {
  const urlSource = {
    entity_id: 'src-open-alias-url',
    kind: 'web_source',
    target: {
      target_kind: 'url',
      url: 'https://example.com/open-alias',
      canonical_url: 'https://example.com/open-alias',
    },
  };

  for (const open_surface of ['web-lens', 'web_lens', 'live-original', 'live_original']) {
    expect(sourceEntityOpenPlan({ ...urlSource, display: { open_surface } })).toMatchObject({
      appId: 'browser',
      mode: 'live_original',
      liveOriginal: true,
      readerMode: false,
    });
  }

  for (const open_surface of ['source-viewer', 'source_viewer', 'reader', 'content', 'source']) {
    expect(sourceEntityOpenPlan({ ...urlSource, display: { open_surface } })).toMatchObject({
      appId: 'content',
      mode: 'source_reader',
      liveOriginal: false,
      readerMode: true,
    });
  }

  expect(sourceEntityOpenPlan(urlSource)).toMatchObject({
    appId: 'content',
    mode: 'source_reader',
    readerMode: true,
  });
  expect(sourceEntityOpenPlan({ ...urlSource, kind: 'youtube_video', display: { open_surface: 'video' } })).toMatchObject({
    appId: 'video',
    mode: 'media',
  });
});

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
  await expect(citation).toHaveAttribute('data-source-expansion-surface', 'media');
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
              reader_snapshot: {
                text_content: [
                  'Full cleaned reader source detail supports the cited claim.',
                  'Second source sentence explains why the cleaned markdown is useful before opening the full source window.',
                  'Third source sentence gives the inline citation enough context to be read in flow without turning the note into a complete source dump.',
                  'Fourth source sentence should remain bounded by the excerpt helper when the inline note is compact.',
                ].join(' '),
              },
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
  await expect(flowNote).toContainText('Second source sentence explains why the cleaned markdown is useful');
  await expect(flowNote).toContainText('Third source sentence gives the inline citation enough context');
  await expect(flowNote).not.toContainText('Fourth source sentence should remain bounded');
  await flowNote.locator('[data-vtext-open-source][data-source-entity-id="src-reader-mode"]').click();

  const sourceWindow = page.locator('[data-content-viewer]').last();
  await expect(sourceWindow).toBeVisible({ timeout: 10000 });
  await expect(sourceWindow).toHaveAttribute('data-source-reader-mode', 'true');
  const reader = sourceWindow.locator('[data-content-reader-markdown]');
  await expect(reader).toBeVisible();
  await expect(reader.locator('h2')).toContainText('Reader-mode source fixture');
  await expect(reader.locator('li')).toHaveCount(2);
  await expect(reader.locator('table')).toContainText('Cleaned markdown');
  await expect(reader).toContainText('Full cleaned reader source detail');
  await expect(reader).not.toContainText(created.contentID);
  await expect(sourceWindow.locator('[data-content-evidence]')).toContainText('SHA-256');
  await expect(sourceWindow.locator('.eyebrow')).toHaveCount(0);
  const closedGeometry = await sourceWindow.evaluate((node) => {
    const reader = node.querySelector('[data-content-reader-markdown]');
    const apparatus = node.querySelector('.source-apparatus');
    const title = node.querySelector('h2');
    const openLink = node.querySelector('.source-link');
    const lastReaderChild = reader?.lastElementChild;
    const readerBox = lastReaderChild?.getBoundingClientRect();
    const apparatusBox = apparatus?.getBoundingClientRect();
    const titleBox = title?.getBoundingClientRect();
    const linkBox = openLink?.getBoundingClientRect();
    return {
      apparatusAfterReader: !!readerBox && !!apparatusBox && apparatusBox.top >= readerBox.bottom - 1,
      titleAndLinkDoNotOverlap: !!titleBox && !!linkBox && (titleBox.right <= linkBox.left - 8 || linkBox.top >= titleBox.bottom - 1),
    };
  });
  expect(closedGeometry.apparatusAfterReader).toBe(true);
  expect(closedGeometry.titleAndLinkDoNotOverlap).toBe(true);
  await sourceWindow.locator('[data-content-evidence] summary').click();
  const expandedGeometry = await sourceWindow.evaluate((node) => {
    const reader = node.querySelector('[data-content-reader-markdown]');
    const shell = reader?.parentElement;
    const evidence = node.querySelector('[data-content-evidence]');
    const lastReaderChild = reader?.lastElementChild;
    const readerBox = lastReaderChild?.getBoundingClientRect();
    const shellBox = shell?.getBoundingClientRect();
    const evidenceBox = evidence?.getBoundingClientRect();
    return !!readerBox && !!shellBox && !!evidenceBox && evidenceBox.top >= readerBox.bottom - 1 && evidenceBox.top >= shellBox.bottom - 1;
  });
  expect(expandedGeometry).toBe(true);
  await page.locator('[data-window-app-id="vtext"]').last().click({ position: { x: 24, y: 24 } });
  await flowNote.locator('[data-vtext-open-source][data-source-entity-id="src-reader-mode"]').click();
  await expect(page.locator('[data-content-viewer][data-source-reader-mode="true"]')).toHaveCount(1);
});

test('VText source URL opens Source Viewer unless browser is explicitly requested', async ({ desktopSession }) => {
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

  const created = await page.evaluate(async () => {
    const title = `Source URL Routing Fixture ${Date.now()}`;
    const sourceURL = 'https://example.com/source-url-routing-fixture';
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
        content: '# Source URL Routing Fixture\n\nThis claim opens a source URL [1](source:src-url-source-viewer).\n\nThis claim explicitly inspects the live original [2](source:src-url-web-lens).',
        author_kind: 'user',
        author_label: 'browser-test',
        metadata: {
          source_entities: [
            {
              entity_id: 'src-url-source-viewer',
              kind: 'web_source',
              label: 'Source URL routing fixture',
              target: {
                target_kind: 'url',
                url: sourceURL,
                canonical_url: sourceURL,
              },
              selectors: [
                {
                  selector_kind: 'text_quote',
                  text_quote: 'Reader snapshot text proves Source Viewer opened instead of Web Lens.',
                },
              ],
              reader_snapshot: {
                text_content: '# Source URL routing fixture\n\nReader snapshot text proves Source Viewer opened instead of Web Lens.',
              },
              display: {
                inline_mode: 'embedded_excerpt',
                expanded_mode: 'source_card',
                open_surface: 'source',
                default_collapsed: true,
              },
              evidence: {
                state: 'available',
                research_state: 'confirmed',
              },
              provenance: {
                created_by: 'browser-test',
                rights_scope: 'public_url_snapshot',
                untrusted_source_text: true,
              },
            },
            {
              entity_id: 'src-url-web-lens',
              kind: 'web_source',
              label: 'Source URL explicit Web Lens fixture',
              target: {
                target_kind: 'url',
                url: sourceURL,
                canonical_url: sourceURL,
              },
              selectors: [
                {
                  selector_kind: 'text_quote',
                  text_quote: 'Explicit browser routing fixture.',
                },
              ],
              display: {
                inline_mode: 'embedded_excerpt',
                expanded_mode: 'source_card',
                open_surface: 'web-lens',
                default_collapsed: true,
              },
              evidence: {
                state: 'available',
                research_state: 'confirmed',
              },
              provenance: {
                created_by: 'browser-test',
                rights_scope: 'public_url_snapshot',
                untrusted_source_text: true,
              },
            },
          ],
        },
      }),
    });
    if (!revRes.ok) throw new Error(`create revision failed: ${revRes.status}`);
    return { title };
  });

  await page.locator('[data-desktop-icon-id="vtext"]').dblclick();
  const vtextWindow = page.locator('[data-vtext-app]').last();
  await expect(vtextWindow.locator('[data-vtext-recent]')).toBeVisible({ timeout: 5000 });
  await vtextWindow.locator('[data-vtext-recent-document]').filter({ hasText: created.title }).click();

  const rendered = vtextWindow.locator('[data-vtext-rendered]');
  const citation = rendered.locator('[data-vtext-source-ref][data-source-entity-id="src-url-source-viewer"]');
  await expect(citation).toBeVisible({ timeout: 10000 });
  await citation.click();
  await rendered.locator('[data-vtext-source-flow-note] [data-vtext-open-source]').click();

  const sourceWindow = page.locator('[data-content-viewer]').last();
  await expect(sourceWindow).toBeVisible({ timeout: 10000 });
  await expect(sourceWindow).toHaveAttribute('data-source-reader-mode', 'true');
  await expect(sourceWindow.locator('[data-content-reader-markdown]')).toContainText('Reader snapshot text proves Source Viewer opened');
  await expect(page.locator('[data-browser-app]')).toHaveCount(0);
  await page.locator('[data-window-app-id="vtext"]').last().click({ position: { x: 24, y: 24 } });

  const explicitBrowserCitation = rendered.locator('[data-vtext-source-ref][data-source-entity-id="src-url-web-lens"]').first();
  await expect(explicitBrowserCitation).toBeVisible({ timeout: 10000 });
  await explicitBrowserCitation.click();
  await rendered.locator('[data-vtext-source-flow-note] [data-vtext-open-source][data-source-entity-id="src-url-web-lens"]').click();
  await expect(page.locator('[data-browser-app]')).toHaveCount(1);
});

test('published source readers prefer publication snapshots over loaded content items', async ({ desktopSession }) => {
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

  const created = await page.evaluate(async () => {
    const contentRes = await fetch('/api/content/items', {
      method: 'POST',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        source_type: 'extracted_url',
        media_type: 'text/markdown',
        app_hint: 'content',
        title: 'Published snapshot fallback source',
        source_url: 'https://example.com/published-snapshot-fallback',
        canonical_url: 'https://example.com/published-snapshot-fallback',
        text_content: [
          '# Mutable content item body',
          '',
          'This loaded content item body is deliberately different from the publication snapshot.',
        ].join('\n'),
        provenance: {
          rights_scope: 'public_source',
          created_by: 'browser-test',
        },
      }),
    });
    if (!contentRes.ok) throw new Error(`create content item failed: ${contentRes.status}`);
    const item = await contentRes.json();
    const sourceEntity = {
      entity_id: 'src-published-snapshot-fallback',
      kind: 'content_item',
      label: 'Published snapshot fallback source',
      publication_route_path: '/pub/vtext/published-snapshot-fixture',
      target: {
        target_kind: 'content_item',
        content_id: item.content_id,
        url: item.source_url,
        canonical_url: item.canonical_url,
      },
      selectors: [
        {
          selector_kind: 'text_quote',
          text_quote: 'Selector quote is only a final fallback.',
          content_hash: item.content_hash,
        },
      ],
      reader_snapshot: {
        text_content: [
          '# Publication reader snapshot',
          '',
          'This publication-carried reader snapshot must remain the source-window body for published readers.',
          '',
          'It is more stable than the target content item and is the public source contract.',
        ].join('\n'),
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
    };
    const windows = [
      {
        window_id: 'published-source-reader-fixture',
        app_id: 'content',
        title: 'Published snapshot fallback source',
        geometry: { x: 80, y: 80, width: 820, height: 620 },
        mode: 'normal',
        z_index: 20,
        app_context: {
          windowTitle: 'Published snapshot fallback source',
          title: 'Published snapshot fallback source',
          sourceUrl: item.source_url,
          contentId: item.content_id,
          content_id: item.content_id,
          mediaType: 'text/markdown',
          appHint: 'content',
          sourceEntity,
          sourceEntityId: sourceEntity.entity_id,
          publishedRoutePath: sourceEntity.publication_route_path,
          publishedGuest: true,
          singletonKey: `source:${sourceEntity.entity_id}`,
        },
      },
    ];
    const stateRes = await fetch('/api/desktop/state', {
      method: 'PUT',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ windows, active_window_id: 'published-source-reader-fixture' }),
    });
    if (!stateRes.ok) throw new Error(`desktop state save failed: ${stateRes.status}`);
    return { contentID: item.content_id };
  });

  await page.reload();
  const sourceWindow = page.locator('[data-window][data-window-id="published-source-reader-fixture"]');
  await expect(sourceWindow).toBeVisible({ timeout: 10000 });
  const viewer = sourceWindow.locator('[data-content-viewer][data-source-reader-mode="true"]');
  await expect(viewer).toBeVisible({ timeout: 10000 });
  await expect(viewer.locator('[data-content-evidence]')).toContainText('Reference');
  await expect(viewer.locator('[data-content-evidence]')).not.toContainText('SHA-256');
  const reader = viewer.locator('[data-content-reader-markdown]');
  await expect(reader.locator('h2')).toContainText('Publication reader snapshot');
  await expect(reader).toContainText('publication-carried reader snapshot must remain');
  await expect(reader).not.toContainText('Mutable content item body');
  await expect(reader).not.toContainText('Selector quote is only a final fallback');
  await expect(reader).not.toContainText(created.contentID);
});

test('VText source panel attaches readable text to an existing source entity', async ({ desktopSession }) => {
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
    const title = `Source Artifact Attach Fixture ${Date.now()}`;
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
        content: '# Source Artifact Attach Fixture\n\nThis claim will receive a readable source artifact [1](source:src-attach-text).',
        author_kind: 'user',
        author_label: 'browser-test',
        metadata: {
          source_entities: [
            {
              entity_id: 'src-attach-text',
              kind: 'web_source',
              label: 'Attachable source fixture',
              target: {
                target_kind: 'url',
                url: 'https://example.com/attachable-source-fixture',
                canonical_url: 'https://example.com/attachable-source-fixture',
              },
              selectors: [
                {
                  selector_kind: 'text_quote',
                  text_quote: 'Readable attachment confirms the cited claim.',
                },
              ],
              display: {
                inline_mode: 'embedded_excerpt',
                expanded_mode: 'source_card',
                open_surface: 'source',
                default_collapsed: true,
              },
              evidence: {
                state: 'available',
                research_state: 'pending',
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
    return doc;
  });

  await page.locator('[data-desktop-icon-id="vtext"]').dblclick();
  const vtextWindow = page.locator('[data-vtext-app]').last();
  await expect(vtextWindow.locator('[data-vtext-recent]')).toBeVisible({ timeout: 5000 });
  await vtextWindow.locator('[data-vtext-recent-document]').filter({ hasText: created.title }).click();

  await vtextWindow.locator('[data-vtext-source-panel]').click();
  const sourcePanel = vtextWindow.locator('[data-vtext-source-diagnostics]');
  await expect(sourcePanel).toBeVisible({ timeout: 10000 });
  await sourcePanel.locator('[data-vtext-source-artifact-text]').fill([
    '# Attached readable source',
    '',
    'Readable attachment confirms the cited claim.',
    '',
    '- The attachment is a reader artifact.',
  ].join('\n'));

  const createRequest = page.waitForRequest((request) => request.url().includes('/api/content/items'));
  const createResponse = page.waitForResponse((response) => response.url().includes('/api/content/items'));
  const attachRequest = page.waitForRequest((request) => request.url().includes('/source-attachments'));
  const attachResponse = page.waitForResponse((response) => response.url().includes('/source-attachments'));
  await sourcePanel.locator('[data-vtext-attach-source-artifact]').click();
  expect((await createRequest).method()).toBe('POST');
  const contentResponse = await createResponse;
  expect(contentResponse.status()).toBe(201);
  const attachment = await attachRequest;
  expect(attachment.method()).toBe('POST');
  const attachmentPayload = JSON.parse(attachment.postData() || '{}');
  expect(attachmentPayload.attachments?.[0]?.entity_id).toBe('src-attach-text');
  const sourceAttachmentResponse = await attachResponse;
  expect(sourceAttachmentResponse.status()).toBe(201);

  const rendered = vtextWindow.locator('[data-vtext-rendered]');
  const citation = rendered.locator('[data-vtext-source-ref][data-source-entity-id="src-attach-text"]').first();
  await expect(citation).toBeVisible({ timeout: 10000 });
  await citation.click();
  await rendered.locator('[data-vtext-source-flow-note] [data-vtext-open-source]').click();
  const sourceWindow = page.locator('[data-content-viewer]').last();
  await expect(sourceWindow).toBeVisible({ timeout: 10000 });
  await expect(sourceWindow.locator('[data-content-reader-markdown]')).toContainText('Attached readable source');
  await expect(sourceWindow.locator('[data-content-reader-markdown]')).toContainText('Readable attachment confirms the cited claim.');
});

test('VText lays out expanded text sources as noncanonical journal flow', async ({ desktopSession }) => {
  const { page } = desktopSession;
  await page.setViewportSize({ width: 1440, height: 980 });
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
  await expect(citation).toHaveAttribute('data-source-expansion-surface', 'journal');
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
  await expect(note.locator('[data-vtext-source-flow-note-title]')).toContainText('ABA Formal Opinion 512 fixture');
  await expect(note.locator('[data-vtext-source-ref-popover]')).toHaveCount(0);
  await expect(note.locator('[data-vtext-source-flow-note-actions] [data-vtext-open-source]')).toBeVisible();
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
    const secondParagraphBesideNote = Array.from(node.querySelectorAll('.vtext-source-journal-line')).some((line) => {
      const top = line.getBoundingClientRect().top - node.getBoundingClientRect().top;
      const lineBox = line.getBoundingClientRect();
      return top >= 0 && top < noteBottom && lineBox.right <= noteBox.left - 10 && line.textContent.includes('Second paragraph');
    });
    return { besideLineCount, sideColumnIsClear, secondParagraphBesideNote };
  });
  const continuedBelowFlow = await rendered.evaluate((node) => {
    const flow = node.querySelector('[data-vtext-source-flow]');
    const followingParagraph = Array.from(node.querySelectorAll('p')).find((paragraph) => paragraph.textContent.includes('Fourth paragraph'));
    if (!flow || !followingParagraph) return false;
    const flowBox = flow.getBoundingClientRect();
    const paragraphBox = followingParagraph.getBoundingClientRect();
    return paragraphBox.top >= flowBox.bottom - 1;
  });
  expect(journalGeometry.besideLineCount).toBeGreaterThanOrEqual(3);
  expect(journalGeometry.sideColumnIsClear).toBe(true);
  expect(journalGeometry.secondParagraphBesideNote).toBe(true);
  expect(continuedBelowFlow).toBe(true);
  await expect(note.locator('.vtext-source-facts')).toHaveCount(0);
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

test('VText uses stacked journal flow instead of old source card when side routing is unavailable', async ({ desktopSession }) => {
  const { page } = desktopSession;
  await page.setViewportSize({ width: 1280, height: 900 });
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
    const title = `Stacked Source Flow Fixture ${Date.now()}`;
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
        content: [
          '# Stacked Source Flow Fixture',
          '',
          'This constrained measure still needs the source note to read like journal evidence rather than an expanded card [1](source:src-stacked-flow), while the article text remains the main object being read.',
          '',
          'A second paragraph proves the original source card path is not needed just because a side column is unavailable.',
        ].join('\n'),
        author_kind: 'user',
        author_label: 'browser-test',
        metadata: {
          source_entities: [
            {
              entity_id: 'src-stacked-flow',
              kind: 'content_item',
              label: 'Stacked journal source fixture',
              target: {
                target_kind: 'url',
                url: 'https://example.com/stacked-source-flow',
                canonical_url: 'https://example.com/stacked-source-flow',
              },
              selectors: [
                {
                  selector_kind: 'text_quote',
                  text_quote: 'Stacked source notes should remain content-first without reusing the old expanded card surface.',
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
        },
      }),
    });
    if (!revRes.ok) throw new Error(`create revision failed: ${revRes.status}`);
    return doc;
  });

  await page.locator('[data-desktop-icon-id="vtext"]').dblclick();
  const vtextWindow = page.locator('[data-vtext-app]').last();
  await expect(vtextWindow.locator('[data-vtext-recent]')).toBeVisible({ timeout: 5000 });
  await vtextWindow.locator('[data-vtext-recent-document]').filter({ hasText: created.title }).click();

  const rendered = vtextWindow.locator('[data-vtext-rendered]');
  await rendered.evaluate((node) => {
    const element = /** @type {HTMLElement} */ (node);
    element.style.width = '560px';
    element.style.maxWidth = '560px';
  });
  const citation = rendered.locator('[data-vtext-source-ref][data-source-entity-id="src-stacked-flow"]').first();
  await expect(citation).toBeVisible({ timeout: 10000 });
  await expect(citation).toHaveAttribute('data-source-expansion-surface', 'journal');
  await citation.click();

  const flow = rendered.locator('[data-vtext-source-flow]');
  await expect(flow).toBeVisible({ timeout: 5000 });
  await expect(flow).toHaveAttribute('data-vtext-source-flow-mode', 'stacked');
  await expect(flow).toHaveAttribute('data-vtext-source-flow-routed-lines', '0');
  await expect(citation).toHaveAttribute('data-source-flow-mounted', 'true');
  await expect(flow.locator('[data-vtext-source-ref-popover]')).toHaveCount(0);
  await expect(flow.locator('[data-vtext-source-flow-note]')).toContainText('Stacked journal source fixture');
  await expect(flow.locator('[data-vtext-source-flow-note]')).not.toContainText('source available');

  const geometry = await flow.evaluate((node) => {
    const flowBox = node.getBoundingClientRect();
    const note = node.querySelector('[data-vtext-source-flow-note]');
    const lines = Array.from(node.querySelectorAll('.vtext-source-journal-line'));
    const noteBox = note?.getBoundingClientRect();
    const lastLineBottom = Math.max(...lines.map((line) => line.getBoundingClientRect().bottom - flowBox.top));
    const lineLayerHasOldCard = !!node.querySelector('.vtext-source-ref[data-expanded="true"] .vtext-source-ref-popover');
    return {
      lineCount: lines.length,
      noteAfterLines: !!noteBox && noteBox.top - flowBox.top >= lastLineBottom - 1,
      lineLayerHasOldCard,
    };
  });
  expect(geometry.lineCount).toBeGreaterThan(0);
  expect(geometry.noteAfterLines).toBe(true);
  expect(geometry.lineLayerHasOldCard).toBe(false);
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
