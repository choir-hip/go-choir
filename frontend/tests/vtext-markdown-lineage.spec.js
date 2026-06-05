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

test('Markdown lineage import resolves known citation markers into expandable source transclusions', async ({ desktopSession }) => {
  const { page } = desktopSession;
  const stamp = Date.now();
  const sourceEntityID = `src-lineage-rule-16-${stamp}`;
  const sourceLabel = 'ABA Model Rule 1.6';
  const excerpt = 'A lawyer shall not reveal information relating to the representation of a client.';

  const imported = await fetchJSON(page, '/api/vtext/markdown-lineage/import', {
    method: 'POST',
    body: JSON.stringify({
      source_path: `proposals/legal-cloud-sourced-${stamp}.md`,
      title: `Legal Cloud Sourced Lineage ${stamp}`,
      source_entities: [
        {
          entity_id: sourceEntityID,
          kind: 'source_service_item',
          label: sourceLabel,
          target: {
            target_kind: 'source_service_item',
            item_id: `srcitem-rule-16-${stamp}`,
            source_id: 'fixture:legal-sources',
            fetch_id: `fetch-${stamp}`,
          },
          selectors: [
            {
              selector_kind: 'text_quote',
              text_quote: excerpt,
              content_hash: `sha256-fixture-${stamp}`,
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
            research_state: 'represented',
          },
          provenance: {
            created_by: 'migration',
            rights_scope: 'source_service_projection',
            untrusted_source_text: true,
          },
        },
      ],
      versions: [
        {
          label: 'v44',
          source_revision_id: `legacy-v44-${stamp}`,
          content: [
            '# Legal Cloud Sourced Lineage',
            '',
            'Confidentiality matters for private legal-cloud work [1].',
            '',
            'One claim still needs source repair [2].',
          ].join('\n'),
          citation_resolutions: [
            {
              marker: '[1]',
              entity_id: sourceEntityID,
            },
          ],
        },
      ],
    }),
  });

  expect(imported.doc_id).toBeTruthy();
  expect(imported.revisions).toHaveLength(1);

  const revisions = await fetchJSON(page, `/api/vtext/documents/${encodeURIComponent(imported.doc_id)}/revisions?limit=10000`);
  expect(revisions.revisions).toHaveLength(1);
  const revision = revisions.revisions[0];
  expect(revision.content).toContain(`[1](source:${sourceEntityID})`);
  expect(revision.content).toContain('One claim still needs source repair [2].');
  expect(revision.content).not.toContain('Confidentiality matters for private legal-cloud work [1].');
  expect(revision.metadata?.source_entities).toHaveLength(1);
  expect(revision.metadata?.source_gaps).toHaveLength(1);
  expect(revision.metadata?.source_gaps?.[0]?.marker).toBe('[2]');
  expect(revision.metadata?.migration_manifest?.citation_resolutions).toEqual([
    { marker: '[1]', entity_id: sourceEntityID },
  ]);

  await page.locator('[data-desktop-icon-id="vtext"]').dblclick();
  const vtextWindow = page.locator('[data-vtext-app]').last();
  await expect(vtextWindow.locator('[data-vtext-recent]')).toBeVisible({ timeout: 5000 });
  await vtextWindow.locator('[data-vtext-recent-document]').filter({ hasText: `Legal Cloud Sourced Lineage ${stamp}` }).click();

  const rendered = vtextWindow.locator('[data-vtext-rendered]');
  const citation = rendered.locator('[data-vtext-source-ref]').first();
  await expect(citation).toBeVisible({ timeout: 10000 });
  await expect(citation).toHaveAttribute('data-vtext-citation-transclusion', '');
  await citation.click();
  await expect(citation).toHaveAttribute('data-expanded', 'true');
  await expect(citation.locator('[data-vtext-inline-transclusion]')).toContainText(sourceLabel);
  await expect(citation.locator('[data-vtext-inline-transclusion]')).toContainText(excerpt);
  await expect(citation.locator('[data-vtext-open-source]')).toBeVisible();
  await expect(rendered).toContainText('One claim still needs source repair [2].');
});
