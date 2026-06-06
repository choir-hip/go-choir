import { test, expect } from './helpers/fixtures.js';

async function fetchJSON(page, path, options = {}) {
  return page.evaluate(async ({ requestPath, requestOptions }) => {
    const res = await fetch(requestPath, {
      credentials: 'include',
      headers: { 'Content-Type': 'application/json', ...(requestOptions.headers || {}) },
      ...requestOptions,
    });
    const text = await res.text();
    let body = null;
    try {
      body = text ? JSON.parse(text) : null;
    } catch (_err) {
      body = text;
    }
    if (!res.ok) {
      throw new Error(`${requestOptions.method || 'GET'} ${requestPath} failed ${res.status}: ${text}`);
    }
    return body;
  }, { requestPath: path, requestOptions: options });
}

async function clearDesktopWindows(page) {
  await fetchJSON(page, '/api/desktop/state', {
    method: 'PUT',
    body: JSON.stringify({ windows: [], active_window_id: '' }),
  });
}

test('publishes source-service source entities as expandable transclusions and canonical exports', async ({ desktopSession, browser }) => {
  const { page, baseURL } = desktopSession;
  await clearDesktopWindows(page);
  const itemID = process.env.SOURCE_SERVICE_ITEM_ID || 'srcitem_fixture_economy';
  const sourceID = process.env.SOURCE_SERVICE_SOURCE_ID || 'gdelt:15min';
  const fetchID = process.env.SOURCE_SERVICE_FETCH_ID || 'fetch_fixture';
  const title = `Source Service Publication ${Date.now()}`;
  const sourceLabel = 'Current economy source packet';
  const excerpt = 'The source-service item is represented as a VText transclusion, not flattened prose.';

  const doc = await fetchJSON(page, '/api/vtext/documents', {
    method: 'POST',
    body: JSON.stringify({ title }),
  });
  await fetchJSON(page, `/api/vtext/documents/${encodeURIComponent(doc.doc_id)}/revisions`, {
    method: 'POST',
    body: JSON.stringify({
      content: `# ${title}\n\n${excerpt} [source](source:src-service-economy)\n\nA second sentence keeps the citation marker compact while metadata owns the source identity.`,
      author_kind: 'user',
      author_label: 'browser-test',
      metadata: {
        source_entities: [
          {
            entity_id: 'src-service-economy',
            kind: 'source_service_item',
            label: sourceLabel,
            target: {
              target_kind: 'source_service_item',
              item_id: itemID,
              source_id: sourceID,
              fetch_id: fetchID,
            },
            selectors: [
              {
                selector_kind: 'text quote',
                text_quote: excerpt,
                content_hash: 'sha256-fixture-source-service-excerpt',
              },
              {
                selector_kind: 'table-range',
                table_id: 'source-service-fixture-table',
                start_row: 1,
                end_row: 2,
              },
              {
                selector_kind: 'page range',
                start_page: 3,
                end_page: 4,
              },
            ],
            display: {
              inline_mode: 'embedded_excerpt',
              expanded_mode: 'source_card',
              open_surface: 'source',
              default_collapsed: false,
            },
            evidence: {
              state: 'confirms',
              relation: 'confirms',
              research_state: 'represented',
            },
            provenance: {
              created_by: 'vtext',
              rights_scope: 'source_service_projection',
              untrusted_source_text: true,
            },
          },
        ],
        export_policy: {
          copy_allowed: true,
          download_allowed: true,
          formats: ['txt', 'md', 'html'],
        },
      },
    }),
  });

  const publish = await fetchJSON(page, '/api/platform/vtext/publications', {
    method: 'POST',
    body: JSON.stringify({
      doc_id: doc.doc_id,
      slug: `source-service-publication-${Date.now()}`,
      access_policy: {
        visibility: 'public',
        route: 'public',
      },
      export_policy: {
        copy_allowed: true,
        download_allowed: true,
        formats: ['txt', 'md', 'html'],
      },
    }),
  });
  expect(publish.route_path).toMatch(/^\/pub\/vtext\//);
  expect(publish.publication_id).toBeTruthy();
  expect(publish.publication_version_id).toBeTruthy();

  const resolved = await fetchJSON(page, `/api/platform/publications/resolve?route=${encodeURIComponent(publish.route_path)}`);
  expect(resolved.source_entities).toHaveLength(1);
  expect(resolved.source_entities[0]).toMatchObject({
    source_entity_id: 'src-service-economy',
    kind: 'source_service_item',
    target_kind: 'source_service_item',
    target_id: itemID,
    display_policy: 'embedded_excerpt',
    open_surface: 'source',
  });
  expect(resolved.transclusions).toHaveLength(1);
  expect(resolved.transclusions[0]).toMatchObject({
    source_entity_id: 'src-service-economy',
    default_display_mode: 'embedded_excerpt',
    snapshot_text: excerpt,
  });
  expect(resolved.transclusions[0].source_selector).toMatchObject({
    selector_kind: 'selector_set',
  });
  expect(resolved.transclusions[0].source_selector.selectors).toHaveLength(3);
  expect(resolved.transclusions[0].source_selector.selectors[0]).toMatchObject({
    selector_kind: 'text_quote',
  });
  expect(resolved.transclusions[0].source_selector.selectors[1]).toMatchObject({
    selector_kind: 'table_range',
    table_id: 'source-service-fixture-table',
  });
  expect(resolved.transclusions[0].source_selector.evidence_state).toMatchObject({
    state: 'confirms',
    relation: 'confirms',
    research_state: 'represented',
  });
  expect(resolved.policy.export.formats).toEqual(expect.arrayContaining(['txt', 'md', 'html']));

  const exported = await fetchJSON(page, `/api/platform/publications/export?route=${encodeURIComponent(publish.route_path)}&format=md`);
  expect(exported.format).toBe('md');
  expect(exported.content).toContain(`# ${title}`);
  expect(exported.content_hash).toBeTruthy();
  expect(exported.filename).toMatch(/\.md$/);
  expect(exported.metadata.source_entities[0].source_entity_id).toBe('src-service-economy');
  expect(exported.metadata.access_policy).toMatchObject({
    route: 'public',
    visibility: 'public',
  });
  expect(exported.metadata.export_policy).toMatchObject({
    download_allowed: true,
  });
  expect(exported.metadata.retrieval.source_id).toBeTruthy();
  expect(exported.metadata.retrieval.spans).toHaveLength(1);
  expect(exported.metadata.retrieval.spans[0].id).toBeTruthy();
  expect(exported.metadata.transclusions[0].source_selector).toMatchObject({
    selector_kind: 'selector_set',
  });
  expect(exported.metadata.transclusions[0].source_selector.selectors).toHaveLength(3);
  expect(exported.metadata.transclusions[0].source_selector.selectors[2]).toMatchObject({
    selector_kind: 'page_range',
    start_page: 3,
    end_page: 4,
  });
  expect(exported.metadata.transclusions[0].source_selector.evidence_state).toMatchObject({
    state: 'confirms',
    relation: 'confirms',
    research_state: 'represented',
  });
  const textExport = await fetchJSON(page, `/api/platform/publications/export?route=${encodeURIComponent(publish.route_path)}&format=txt`);
  expect(textExport.content).toContain(excerpt);
  expect(textExport.content).not.toContain('Open source');
  expect(textExport.content).not.toContain('Close');

  await page.goto(`${baseURL}${publish.route_path}`);
  const publishedReader = page.locator('[data-vtext-published-reader]').last();
  await expect(publishedReader).toBeVisible({ timeout: 15_000 });
  await expect(publishedReader).toHaveAttribute('data-publication-version-id', publish.publication_version_id);
  await expect(publishedReader).toHaveAttribute('contenteditable', 'false');
  await expect(publishedReader).toHaveAttribute('aria-label', 'Published VText document');
  await expect(publishedReader).not.toHaveAttribute('aria-multiline', 'true');
  const publishedSurfaceSemantics = await publishedReader.evaluate((node) => ({
    tagName: node.tagName.toLowerCase(),
    role: node.getAttribute('role') || '',
    tabIndexAttribute: node.getAttribute('tabindex') || '',
  }));
  expect(publishedSurfaceSemantics).toEqual({
    tagName: 'article',
    role: '',
    tabIndexAttribute: '',
  });
  const citation = publishedReader.locator('[data-vtext-source-ref]').first();
  await expect(citation).toHaveAttribute('data-vtext-citation-transclusion', '');
  await citation.click();
  await expect(citation).toHaveAttribute('data-expanded', 'true');
  await expect(citation.locator('[data-vtext-inline-transclusion]')).toContainText(sourceLabel);
  await expect(citation.locator('[data-vtext-inline-transclusion]')).toContainText(excerpt);
  await expect(citation.locator('[data-vtext-inline-transclusion]')).not.toContainText(itemID);
  const journalOpenSource = publishedReader.locator('[data-vtext-source-flow-note] [data-vtext-open-source]').first();
  const inlineOpenSource = publishedReader.locator('[data-vtext-open-source][data-source-entity-id="src-service-economy"]:visible').first();
  const openSource = await journalOpenSource.isVisible().catch(() => false) ? journalOpenSource : inlineOpenSource;
  await expect(openSource).toBeVisible();

  const initialSourceWindows = await page.locator('[data-content-viewer]').count();
  await openSource.click();
  await expect(page.locator('[data-content-viewer]')).toHaveCount(initialSourceWindows + 1, { timeout: 10000 });
  const sourceWindow = page.locator('[data-content-viewer]').last();
  await expect(sourceWindow).toContainText(sourceLabel);
  await expect(sourceWindow).toContainText(excerpt);
  await expect(sourceWindow.locator('[data-source-entity]')).toContainText('src-service-economy');
  await expect(sourceWindow.locator('[data-source-entity]')).toContainText(itemID);

  const guestContext = await browser.newContext();
  try {
    const guestPage = await guestContext.newPage();
    await guestPage.goto(`${baseURL}${publish.route_path}`);
    const guestReader = guestPage.locator('[data-vtext-published-reader]').last();
    await expect(guestReader).toBeVisible({ timeout: 15_000 });
    const guestCitation = guestReader.locator('[data-vtext-source-ref][data-source-entity-id="src-service-economy"]').first();
    await guestCitation.click();
    const guestOpenSource = guestReader.locator('[data-vtext-source-flow-note] [data-vtext-open-source]').first();
    await expect(guestOpenSource).toBeVisible({ timeout: 10_000 });
    const initialGuestSourceWindows = await guestPage.locator('[data-content-viewer]').count();
    await guestOpenSource.click();
    await expect(guestPage.locator('[data-content-viewer]')).toHaveCount(initialGuestSourceWindows + 1, { timeout: 10000 });
    const guestSourceWindow = guestPage.locator('[data-content-viewer]').last();
    await expect(guestSourceWindow).toHaveAttribute('data-source-reader-mode', 'true');
    await expect(guestSourceWindow).toContainText(sourceLabel);
    await expect(guestSourceWindow).toContainText(excerpt);
    await expect(guestSourceWindow.locator('[data-source-entity]')).toContainText('src-service-economy');
    await expect(guestPage.locator('[data-browser-app]')).toHaveCount(0);
  } finally {
    await guestContext.close();
  }
});

test('publishes public content-item sources with cleaned reader snapshots', async ({ desktopSession, browser }) => {
  const { page, baseURL } = desktopSession;
  await clearDesktopWindows(page);
  const stamp = Date.now();
  const title = `Public Source Snapshot Publication ${stamp}`;
  const excerpt = 'ABA guidance says lawyers using generative AI must consider competence and confidentiality.';
  const fullSourceText = [
    excerpt,
    'Full cleaned reader source detail: lawyers should evaluate model limitations, protect client information, supervise subordinate use, communicate relevant risks, and ensure fees remain reasonable.',
    'This sentence is intentionally outside the selected citation excerpt so the source window proves publication carried the cleaned reader snapshot, not only the bounded quote.',
  ].join('\n\n');

  const contentItem = await fetchJSON(page, '/api/content/items', {
    method: 'POST',
    body: JSON.stringify({
      source_type: 'extracted_url',
      media_type: 'text/html; charset=utf-8',
      app_hint: 'content',
      title: 'ABA Formal Opinion 512 cleaned source',
      source_url: 'https://www.americanbar.org/groups/professional_responsibility/publications/ethics_opinions/aba-formal-opinion-512/',
      canonical_url: 'https://www.americanbar.org/groups/professional_responsibility/publications/ethics_opinions/aba-formal-opinion-512/',
      text_content: fullSourceText,
      provenance: {
        rights_scope: 'public_source',
        created_by: 'browser-test',
        warnings: ['extracted text is low-content'],
      },
    }),
  });

  const doc = await fetchJSON(page, '/api/vtext/documents', {
    method: 'POST',
    body: JSON.stringify({ title }),
  });
  await fetchJSON(page, `/api/vtext/documents/${encodeURIComponent(doc.doc_id)}/revisions`, {
    method: 'POST',
    body: JSON.stringify({
      content: `# ${title}\n\nThe proposal cites ethics guidance as an inspectable source [1](source:src-public-content). A following sentence keeps normal article flow beside the source note.`,
      author_kind: 'user',
      author_label: 'browser-test',
      metadata: {
        source_entities: [
          {
            entity_id: 'src-public-content',
            kind: 'ethics_opinion',
            label: 'ABA Formal Opinion 512 cleaned source',
            target: {
              target_kind: 'content_item',
              content_id: contentItem.content_id,
              url: contentItem.source_url,
              canonical_url: contentItem.canonical_url,
            },
            selectors: [
              {
                selector_kind: 'text_quote',
                text_quote: excerpt,
                content_hash: contentItem.content_hash,
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

  const publish = await fetchJSON(page, '/api/platform/vtext/publications', {
    method: 'POST',
    body: JSON.stringify({
      doc_id: doc.doc_id,
      slug: `public-source-snapshot-${stamp}`,
    }),
  });
  const resolved = await fetchJSON(page, `/api/platform/publications/resolve?route=${encodeURIComponent(publish.route_path)}`);
  const entity = resolved.source_entities?.[0]?.entity || {};
  expect(entity.reader_snapshot?.text_content).toContain('Full cleaned reader source detail');
  expect(entity.reader_snapshot?.snapshot_kind).toBe('cleaned_reader_markdown');
  expect(entity.reader_snapshot?.media_type).toBe('text/markdown');
  expect(entity.reader_snapshot?.original_media_type).toBe('text/html');
  expect(entity.reader_snapshot_status?.quality).toBe('warning');
  expect(entity.reader_snapshot_status?.warnings).toContain('extracted text is low-content');
  expect(resolved.transclusions?.[0]?.snapshot_text).toBe(excerpt);

  await page.goto(`${baseURL}${publish.route_path}`);
  const publishedReader = page.locator('[data-vtext-published-reader]').last();
  await expect(publishedReader).toBeVisible({ timeout: 15_000 });
  const citation = publishedReader.locator('[data-vtext-source-ref][data-source-entity-id="src-public-content"]').first();
  await citation.click();
  const sourceNote = publishedReader.locator('[data-vtext-source-flow-note]').filter({ hasText: 'ABA Formal Opinion 512 cleaned source' }).last();
  await expect(sourceNote).toBeVisible({ timeout: 10_000 });
  await expect(sourceNote).toContainText(excerpt);
  await expect(sourceNote).not.toContainText('Full cleaned reader source detail');

  await sourceNote.locator('[data-vtext-open-source]').click();
  const sourceWindow = page.locator('[data-content-viewer]').last();
  await expect(sourceWindow).toBeVisible({ timeout: 10000 });
  await expect(sourceWindow).toHaveAttribute('data-source-reader-mode', 'true');
  await expect(sourceWindow.locator('[data-content-reader-markdown]')).toContainText('Full cleaned reader source detail');
  await expect(page.locator('[data-browser-app]')).toHaveCount(0);

  const guestContext = await browser.newContext();
  try {
    const guestPage = await guestContext.newPage();
    await guestPage.goto(`${baseURL}${publish.route_path}`);
    const guestReader = guestPage.locator('[data-vtext-published-reader]').last();
    await expect(guestReader).toBeVisible({ timeout: 15_000 });
    const guestCitation = guestReader.locator('[data-vtext-source-ref][data-source-entity-id="src-public-content"]').first();
    await guestCitation.click();
    const guestSourceNote = guestReader.locator('[data-vtext-source-flow-note]').filter({ hasText: 'ABA Formal Opinion 512 cleaned source' }).last();
    await expect(guestSourceNote).toBeVisible({ timeout: 10_000 });
    await expect(guestSourceNote).toContainText(excerpt);
    await guestSourceNote.locator('[data-vtext-open-source]').click();
    const guestSourceWindow = guestPage.locator('[data-content-viewer]').last();
    await expect(guestSourceWindow).toBeVisible({ timeout: 10000 });
    await expect(guestSourceWindow).toHaveAttribute('data-source-reader-mode', 'true');
    await expect(guestSourceWindow.locator('[data-content-reader-markdown]')).toContainText('Full cleaned reader source detail');
    await expect(guestPage.locator('[data-browser-app]')).toHaveCount(0);
  } finally {
    await guestContext.close();
  }
});

test('publishes public URL-backed sources with reader snapshots for guests', async ({ desktopSession, browser }) => {
  const { page, baseURL } = desktopSession;
  await clearDesktopWindows(page);
  const stamp = Date.now();
  const title = `Public URL Source Snapshot ${stamp}`;
  const sourceURL = 'https://example.com/';
  const sourceLabel = 'Example Domain public URL source';
  const excerpt = 'This domain is for use in illustrative examples in documents.';

  const doc = await fetchJSON(page, '/api/vtext/documents', {
    method: 'POST',
    body: JSON.stringify({ title }),
  });
  await fetchJSON(page, `/api/vtext/documents/${encodeURIComponent(doc.doc_id)}/revisions`, {
    method: 'POST',
    body: JSON.stringify({
      content: `# ${title}\n\nThe publication path imports a public URL-backed source snapshot [1](source:src-public-url).`,
      author_kind: 'user',
      author_label: 'browser-test',
      metadata: {
        source_entities: [
          {
            entity_id: 'src-public-url',
            kind: 'web_source',
            label: sourceLabel,
            target: {
              target_kind: 'url',
              url: sourceURL,
              canonical_url: sourceURL,
            },
            selectors: [
              {
                selector_kind: 'text_quote',
                text_quote: excerpt,
              },
            ],
            display: {
              inline_mode: 'embedded_excerpt',
              expanded_mode: 'source_card',
              open_surface: 'source',
              default_collapsed: true,
            },
            evidence: {
              state: 'confirms',
              relation: 'confirms',
              research_state: 'owner_supplied',
            },
            provenance: {
              created_by: 'browser-test',
              rights_scope: 'public_url_snapshot',
              untrusted_source_text: true,
            },
          },
        ],
        export_policy: {
          copy_allowed: true,
          download_allowed: true,
          formats: ['md', 'html', 'txt'],
        },
      },
    }),
  });

  const publish = await fetchJSON(page, '/api/platform/vtext/publications', {
    method: 'POST',
    body: JSON.stringify({
      doc_id: doc.doc_id,
      slug: `public-url-source-snapshot-${stamp}`,
    }),
  });
  const resolved = await fetchJSON(page, `/api/platform/publications/resolve?route=${encodeURIComponent(publish.route_path)}`);
  const entity = resolved.source_entities?.[0]?.entity || {};
  expect(resolved.source_entities?.[0]).toMatchObject({
    source_entity_id: 'src-public-url',
    kind: 'web_source',
    target_kind: 'url',
    target_id: sourceURL,
    open_surface: 'source',
  });
  expect(resolved.transclusions?.[0]?.snapshot_text).toBe(excerpt);
  expect(resolved.transclusions?.[0]?.source_selector.evidence_state).toMatchObject({
    state: 'confirms',
    relation: 'confirms',
    research_state: 'owner_supplied',
  });
  expect(entity.reader_snapshot?.text_content).toContain('Example Domain');
  expect(entity.reader_snapshot?.source_url).toBe(sourceURL);
  expect(entity.reader_snapshot?.access_scope).toBe('publication_reader');
  expect(entity.reader_snapshot_status?.state).toBe('reader_snapshot_ready');

  const exported = await fetchJSON(page, `/api/platform/publications/export?route=${encodeURIComponent(publish.route_path)}&format=md`);
  expect(exported.metadata.source_entities[0].source_entity_id).toBe('src-public-url');
  expect(exported.metadata.access_policy).toMatchObject({
    route: 'public',
    visibility: 'public',
  });
  expect(exported.metadata.export_policy).toMatchObject({
    download_allowed: true,
  });
  expect(exported.metadata.retrieval.source_id).toBeTruthy();
  expect(exported.metadata.retrieval.spans).toHaveLength(1);
  expect(exported.metadata.transclusions[0].snapshot_text).toBe(excerpt);
  expect(exported.metadata.transclusions[0].source_selector.evidence_state).toMatchObject({
    state: 'confirms',
    relation: 'confirms',
    research_state: 'owner_supplied',
  });

  await page.goto(`${baseURL}${publish.route_path}`);
  const publishedReader = page.locator('[data-vtext-published-reader]').last();
  await expect(publishedReader).toBeVisible({ timeout: 15_000 });
  const citation = publishedReader.locator('[data-vtext-source-ref][data-source-entity-id="src-public-url"]').first();
  await citation.click();
  const sourceNote = publishedReader.locator('[data-vtext-source-flow-note]').filter({ hasText: sourceLabel }).last();
  await expect(sourceNote).toBeVisible({ timeout: 10_000 });
  await expect(sourceNote).toContainText(excerpt);
  await expect(sourceNote).not.toContainText('More information');

  const initialSourceWindows = await page.locator('[data-content-viewer]').count();
  await sourceNote.locator('[data-vtext-open-source]').click();
  await expect(page.locator('[data-content-viewer]')).toHaveCount(initialSourceWindows + 1, { timeout: 10000 });
  const sourceWindow = page.locator('[data-content-viewer]').last();
  await expect(sourceWindow).toBeVisible({ timeout: 10000 });
  await expect(sourceWindow).toHaveAttribute('data-source-reader-mode', 'true');
  await expect(sourceWindow.locator('[data-content-reader-markdown]')).toContainText('Example Domain');
  await expect(page.locator('[data-browser-app]')).toHaveCount(0);

  const guestContext = await browser.newContext();
  try {
    const guestPage = await guestContext.newPage();
    await guestPage.goto(`${baseURL}${publish.route_path}`);
    const guestReader = guestPage.locator('[data-vtext-published-reader]').last();
    await expect(guestReader).toBeVisible({ timeout: 15_000 });
    const guestCitation = guestReader.locator('[data-vtext-source-ref][data-source-entity-id="src-public-url"]').first();
    await guestCitation.click();
    const guestSourceNote = guestReader.locator('[data-vtext-source-flow-note]').filter({ hasText: sourceLabel }).last();
    await expect(guestSourceNote).toBeVisible({ timeout: 10_000 });
    await expect(guestSourceNote).toContainText(excerpt);
    await expect(guestSourceNote).not.toContainText('More information');
    const initialGuestSourceWindows = await guestPage.locator('[data-content-viewer]').count();
    await guestSourceNote.locator('[data-vtext-open-source]').click();
    await expect(guestPage.locator('[data-content-viewer]')).toHaveCount(initialGuestSourceWindows + 1, { timeout: 10000 });
    const guestSourceWindow = guestPage.locator('[data-content-viewer]').last();
    await expect(guestSourceWindow).toBeVisible({ timeout: 10000 });
    await expect(guestSourceWindow).toHaveAttribute('data-source-reader-mode', 'true');
    await expect(guestSourceWindow.locator('[data-content-reader-markdown]')).toContainText('Example Domain');
    await expect(guestPage.locator('[data-browser-app]')).toHaveCount(0);
  } finally {
    await guestContext.close();
  }
});
