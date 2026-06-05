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
  await expect(publishedReader.locator('[data-vtext-source-ref]')).toHaveAttribute('data-vtext-citation-transclusion', '');
  await expect(publishedReader.locator('[data-vtext-source-inline]')).toHaveAttribute('data-vtext-display-policy', 'embedded_excerpt');
  await expect(publishedReader.locator('[data-vtext-source-inline]')).toHaveAttribute('open', '');
  await expect(publishedReader.locator('[data-vtext-source-inline]')).toContainText(sourceLabel);
  await expect(publishedReader.locator('[data-vtext-source-inline]')).not.toContainText(itemID);
  await expect(publishedReader.locator('[data-vtext-source-inline] [data-vtext-open-source]')).toBeVisible();
});
