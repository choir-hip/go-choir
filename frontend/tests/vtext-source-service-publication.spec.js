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

test('publishes source-service source entities as expandable transclusions and canonical exports', async ({ desktopSession }) => {
  const { page, baseURL } = desktopSession;
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
                selector_kind: 'text_quote',
                text_quote: excerpt,
                content_hash: 'sha256-fixture-source-service-excerpt',
              },
            ],
            display: {
              inline_mode: 'embedded_excerpt',
              expanded_mode: 'source_card',
              open_surface: 'source',
              default_collapsed: false,
            },
            evidence: {
              state: 'available',
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
  expect(resolved.policy.export.formats).toEqual(expect.arrayContaining(['txt', 'md', 'html']));

  const exported = await fetchJSON(page, `/api/platform/publications/export?route=${encodeURIComponent(publish.route_path)}&format=md`);
  expect(exported.format).toBe('md');
  expect(exported.content).toContain(`# ${title}`);
  expect(exported.content_hash).toBeTruthy();
  expect(exported.filename).toMatch(/\.md$/);

  await page.goto(`${baseURL}${publish.route_path}`);
  const publishedReader = page.locator('[data-vtext-published-reader]').last();
  await expect(publishedReader).toBeVisible({ timeout: 15_000 });
  await expect(publishedReader).toHaveAttribute('data-publication-version-id', publish.publication_version_id);
  const citation = publishedReader.locator('[data-vtext-source-ref]').first();
  await expect(citation).toHaveAttribute('data-vtext-citation-transclusion', '');
  await citation.click();
  await expect(citation).toHaveAttribute('data-expanded', 'true');
  await expect(citation.locator('[data-vtext-inline-transclusion]')).toContainText(sourceLabel);
  await expect(citation.locator('[data-vtext-inline-transclusion]')).toContainText(excerpt);
  await expect(citation.locator('[data-vtext-inline-transclusion]')).not.toContainText(itemID);
  const openSource = citation.locator('[data-vtext-open-source]');
  await expect(openSource).toBeVisible();

  const initialSourceWindows = await page.locator('[data-content-viewer]').count();
  await openSource.click();
  await expect(page.locator('[data-content-viewer]')).toHaveCount(initialSourceWindows + 1, { timeout: 10000 });
  const sourceWindow = page.locator('[data-content-viewer]').last();
  await expect(sourceWindow).toContainText(sourceLabel);
  await expect(sourceWindow).toContainText(excerpt);
  await expect(sourceWindow.locator('[data-source-entity]')).toContainText('src-service-economy');
  await expect(sourceWindow.locator('[data-source-entity]')).toContainText(itemID);
});

test('publishes public content-item sources with cleaned reader snapshots', async ({ desktopSession, browser }) => {
  const { page, baseURL } = desktopSession;
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
      app_hint: 'browser',
      title: 'ABA Formal Opinion 512 cleaned source',
      source_url: 'https://www.americanbar.org/groups/professional_responsibility/publications/ethics_opinions/aba-formal-opinion-512/',
      canonical_url: 'https://www.americanbar.org/groups/professional_responsibility/publications/ethics_opinions/aba-formal-opinion-512/',
      text_content: fullSourceText,
      provenance: {
        rights_scope: 'public_source',
        created_by: 'browser-test',
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
  const sourceWindow = page.locator('[data-browser-app]').last();
  await expect(sourceWindow).toBeVisible({ timeout: 10000 });
  await expect(sourceWindow).toContainText('Source reader snapshot');
  await expect(sourceWindow.locator('[data-browser-reader-markdown]')).toContainText('Full cleaned reader source detail');
  await expect(sourceWindow.locator('[data-browser-iframe]')).toHaveCount(0);

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
    const guestSourceWindow = guestPage.locator('[data-browser-app]').last();
    await expect(guestSourceWindow).toBeVisible({ timeout: 10000 });
    await expect(guestSourceWindow).toContainText('Source reader snapshot');
    await expect(guestSourceWindow.locator('[data-browser-reader-markdown]')).toContainText('Full cleaned reader source detail');
    await expect(guestSourceWindow.locator('[data-browser-iframe]')).toHaveCount(0);
  } finally {
    await guestContext.close();
  }
});
