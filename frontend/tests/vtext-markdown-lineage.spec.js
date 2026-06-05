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
  const initialSourceWindows = await page.locator('[data-content-viewer]').count();
  await citation.locator('[data-vtext-open-source]').click();
  await expect(page.locator('[data-content-viewer]')).toHaveCount(initialSourceWindows + 1, { timeout: 10000 });
  const sourceWindow = page.locator('[data-content-viewer]').last();
  await expect(sourceWindow).toContainText(sourceLabel);
  await expect(sourceWindow).toContainText(excerpt);
  await expect(sourceWindow.locator('[data-source-entity]')).toContainText(sourceEntityID);
  await expect(rendered).toContainText('One claim still needs source repair [2].');
});

test('Markdown lineage import can migrate from stored ContentItem versions', async ({ desktopSession }) => {
  const { page } = desktopSession;
  const stamp = Date.now();
  const sourceEntityID = `src-content-backed-rule-${stamp}`;
  const sourceLabel = 'Stored Legal Source';
  const excerpt = 'Stored source evidence supports the migrated historical claim.';

  const oldItem = await fetchJSON(page, '/api/content/items', {
    method: 'POST',
    body: JSON.stringify({
      source_type: 'file',
      media_type: 'text/markdown',
      app_hint: 'vtext',
      title: `Content-backed Legal Cloud v44 ${stamp}`,
      file_path: `proposals/content-backed-legal-cloud-${stamp}.md#v44`,
      text_content: [
        '# Content-backed Legal Cloud',
        '',
        'Stored historical source-backed claim [1].',
      ].join('\n'),
      metadata: { source_revision_id: `legacy-content-v44-${stamp}` },
      provenance: { created_from: 'browser-product-path-fixture' },
    }),
  });
  const latestItem = await fetchJSON(page, '/api/content/items', {
    method: 'POST',
    body: JSON.stringify({
      source_type: 'file',
      media_type: 'text/markdown',
      app_hint: 'vtext',
      title: `Content-backed Legal Cloud v49 ${stamp}`,
      file_path: `proposals/content-backed-legal-cloud-${stamp}.md#v49`,
      text_content: [
        '# Content-backed Legal Cloud',
        '',
        '| Term | Definition |',
        '| --- | --- |',
        '| Work product | Durable professional output |',
      ].join('\n'),
      metadata: { source_revision_id: `legacy-content-v49-${stamp}` },
      provenance: { created_from: 'browser-product-path-fixture' },
    }),
  });

  const imported = await fetchJSON(page, '/api/vtext/markdown-lineage/import', {
    method: 'POST',
    body: JSON.stringify({
      source_path: `proposals/content-backed-legal-cloud-${stamp}.md`,
      title: `Content-backed Legal Cloud ${stamp}`,
      source_entities: [
        {
          entity_id: sourceEntityID,
          kind: 'content_item',
          label: sourceLabel,
          target: {
            target_kind: 'content_item',
            content_id: oldItem.content_id,
          },
          selectors: [
            {
              selector_kind: 'text_quote',
              text_quote: excerpt,
              content_hash: `sha256-content-backed-${stamp}`,
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
            rights_scope: 'private_user_source',
            untrusted_source_text: true,
          },
        },
      ],
      versions: [
        {
          label: 'v44',
          source_revision_id: `legacy-content-v44-${stamp}`,
          content_item_id: oldItem.content_id,
          citation_resolutions: [
            {
              marker: '[1]',
              entity_id: sourceEntityID,
            },
          ],
        },
        {
          label: 'v49',
          source_revision_id: `legacy-content-v49-${stamp}`,
          content_item_id: latestItem.content_id,
        },
      ],
    }),
  });

  expect(imported.revision_count).toBe(2);
  expect(imported.original_content_ids).toEqual([oldItem.content_id, latestItem.content_id]);

  const revisions = await fetchJSON(page, `/api/vtext/documents/${encodeURIComponent(imported.doc_id)}/revisions?limit=10000`);
  expect(revisions.revisions).toHaveLength(2);
  const historical = revisions.revisions.find((revision) => revision.version_number === 0);
  const latest = revisions.revisions.find((revision) => revision.version_number === 1);
  expect(historical.content).toContain(`[1](source:${sourceEntityID})`);
  expect(latest.content).toContain('| Work product | Durable professional output |');
  expect(historical.metadata?.migration_manifest?.original_content_id).toBe(oldItem.content_id);
  expect(historical.metadata?.migration_manifest?.source_content_item_id).toBe(oldItem.content_id);
  expect(historical.metadata?.migration_manifest?.original_content_source).toBe('content_item');
  expect(historical.metadata?.migration_manifest?.original_content_path).toBe(oldItem.file_path);

  await page.locator('[data-desktop-icon-id="vtext"]').dblclick();
  const vtextWindow = page.locator('[data-vtext-app]').last();
  await expect(vtextWindow.locator('[data-vtext-recent]')).toBeVisible({ timeout: 5000 });
  await vtextWindow.locator('[data-vtext-recent-document]').filter({ hasText: `Content-backed Legal Cloud ${stamp}` }).click();

  const rendered = vtextWindow.locator('[data-vtext-rendered]');
  await expect(rendered.locator('.table-scroll table')).toBeVisible({ timeout: 10000 });
  await expect(rendered).toContainText('Work product');

  const restored = await fetchJSON(page, `/api/vtext/documents/${encodeURIComponent(imported.doc_id)}/restore`, {
    method: 'POST',
    body: JSON.stringify({ revision_id: historical.revision_id }),
  });
  expect(restored.revision_id).toBeTruthy();
  await page.reload();
  await page.locator('[data-desktop-icon-id="vtext"]').dblclick();
  const restoredWindow = page.locator('[data-vtext-app]').last();
  await expect(restoredWindow.locator('[data-vtext-recent]')).toBeVisible({ timeout: 5000 });
  await restoredWindow.locator('[data-vtext-recent-document]').filter({ hasText: `Content-backed Legal Cloud ${stamp}` }).click();
  const restoredRendered = restoredWindow.locator('[data-vtext-rendered]');
  const citation = restoredRendered.locator('[data-vtext-source-ref]').first();
  await expect(citation).toBeVisible({ timeout: 10000 });
  await citation.click();
  await expect(citation).toHaveAttribute('data-expanded', 'true');
  await expect(citation.locator('[data-vtext-inline-transclusion]')).toContainText(sourceLabel);
  await expect(citation.locator('[data-vtext-inline-transclusion]')).toContainText(excerpt);
});

test('Migrated source gaps can be repaired as canonical VText revisions', async ({ desktopSession }) => {
  const { page } = desktopSession;
  const stamp = Date.now();
  const sourceEntityID = `src-repaired-gap-${stamp}`;
  const sourceLabel = 'Repaired Legal Source';
  const excerpt = 'Repaired source evidence supports the migrated citation.';

  const imported = await fetchJSON(page, '/api/vtext/markdown-lineage/import', {
    method: 'POST',
    body: JSON.stringify({
      source_path: `proposals/source-gap-repair-${stamp}.md`,
      title: `Source Gap Repair ${stamp}`,
      versions: [
        {
          label: 'v44',
          source_revision_id: `legacy-gap-v44-${stamp}`,
          content: [
            '# Source Gap Repair',
            '',
            'This migrated claim starts with a repairable citation gap [2].',
          ].join('\n'),
        },
      ],
    }),
  });

  const revisionsBefore = await fetchJSON(page, `/api/vtext/documents/${encodeURIComponent(imported.doc_id)}/revisions?limit=10000`);
  expect(revisionsBefore.revisions).toHaveLength(1);
  expect(revisionsBefore.revisions[0].metadata?.source_gaps?.[0]?.marker).toBe('[2]');

  const repaired = await fetchJSON(page, `/api/vtext/documents/${encodeURIComponent(imported.doc_id)}/source-repairs`, {
    method: 'POST',
    body: JSON.stringify({
      base_revision_id: imported.current_revision_id,
      source_entities: [
        {
          entity_id: sourceEntityID,
          kind: 'source_service_item',
          label: sourceLabel,
          target: {
            target_kind: 'source_service_item',
            item_id: `srcitem-repaired-gap-${stamp}`,
          },
          selectors: [
            {
              selector_kind: 'text_quote',
              text_quote: excerpt,
              content_hash: `sha256-repaired-gap-${stamp}`,
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
            created_by: 'source_gap_repair',
            rights_scope: 'source_service_projection',
            untrusted_source_text: true,
          },
        },
      ],
      citation_resolutions: [
        {
          marker: '[2]',
          entity_id: sourceEntityID,
        },
      ],
    }),
  });

  expect(repaired.version_number).toBe(1);
  expect(repaired.parent_revision_id).toBe(imported.current_revision_id);
  expect(repaired.content).toContain(`[2](source:${sourceEntityID})`);
  expect(repaired.content).not.toContain(`[2](source:${sourceEntityID})(source:`);
  expect(repaired.metadata?.source).toBe('vtext_source_gap_repair');
  expect(repaired.metadata?.source_gaps).toBeUndefined();
  expect(repaired.metadata?.source_entities).toHaveLength(1);
  expect(repaired.metadata?.source_repair_resolutions).toEqual([{ marker: '[2]', entity_id: sourceEntityID }]);

  await page.locator('[data-desktop-icon-id="vtext"]').dblclick();
  const vtextWindow = page.locator('[data-vtext-app]').last();
  await expect(vtextWindow.locator('[data-vtext-recent]')).toBeVisible({ timeout: 5000 });
  await vtextWindow.locator('[data-vtext-recent-document]').filter({ hasText: `Source Gap Repair ${stamp}` }).click();

  const rendered = vtextWindow.locator('[data-vtext-rendered]');
  const citation = rendered.locator('[data-vtext-source-ref]').first();
  await expect(citation).toBeVisible({ timeout: 10000 });
  await expect(citation).toHaveAttribute('data-source-label', '2');
  await citation.click();
  await expect(citation).toHaveAttribute('data-expanded', 'true');
  await expect(citation.locator('[data-vtext-inline-transclusion]')).toContainText(sourceLabel);
  await expect(citation.locator('[data-vtext-inline-transclusion]')).toContainText(excerpt);
  await expect(citation.locator('[data-vtext-open-source]')).toBeVisible();
});

test('VText Sources panel applies source-gap repair and opens repaired source window', async ({ desktopSession }) => {
  const { page } = desktopSession;
  const stamp = Date.now();
  const sourceEntityID = `src-panel-repair-${stamp}`;
  const sourceLabel = 'Panel Repair Source';
  const excerpt = 'Panel repair source evidence supports the citation.';

  const imported = await fetchJSON(page, '/api/vtext/markdown-lineage/import', {
    method: 'POST',
    body: JSON.stringify({
      source_path: `proposals/panel-source-gap-repair-${stamp}.md`,
      title: `Panel Source Repair ${stamp}`,
      versions: [
        {
          label: 'v44',
          source_revision_id: `legacy-panel-gap-v44-${stamp}`,
          content: [
            '# Panel Source Repair',
            '',
            'This owner-visible source panel claim starts with a citation gap [2].',
          ].join('\n'),
        },
      ],
    }),
  });

  await page.locator('[data-desktop-icon-id="vtext"]').dblclick();
  const vtextWindow = page.locator('[data-vtext-app]').last();
  await expect(vtextWindow.locator('[data-vtext-recent]')).toBeVisible({ timeout: 5000 });
  await vtextWindow.locator('[data-vtext-recent-document]').filter({ hasText: `Panel Source Repair ${stamp}` }).click();

  await vtextWindow.locator('[data-vtext-source-panel]').click();
  const sourcePanel = vtextWindow.locator('[data-vtext-source-diagnostics]');
  await expect(sourcePanel).toBeVisible({ timeout: 10000 });
  await expect(sourcePanel.locator('[data-vtext-source-gaps]')).toContainText('[2]');

  const repairPayload = {
    base_revision_id: imported.current_revision_id,
    source_entities: [
      {
        entity_id: sourceEntityID,
        kind: 'source_service_item',
        label: sourceLabel,
        target: {
          target_kind: 'source_service_item',
          item_id: `srcitem-panel-repair-${stamp}`,
        },
        selectors: [
          {
            selector_kind: 'text_quote',
            text_quote: excerpt,
            content_hash: `sha256-panel-repair-${stamp}`,
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
          created_by: 'source_panel_repair_test',
          rights_scope: 'source_service_projection',
          untrusted_source_text: true,
        },
      },
    ],
    citation_resolutions: [
      {
        marker: '[2]',
        entity_id: sourceEntityID,
      },
    ],
  };
  await sourcePanel.locator('[data-vtext-source-repair-payload]').fill(JSON.stringify(repairPayload, null, 2));
  await sourcePanel.locator('[data-vtext-apply-source-repair]').click();

  const rendered = vtextWindow.locator('[data-vtext-rendered]');
  const citation = rendered.locator('[data-vtext-source-ref]').first();
  await expect(citation).toBeVisible({ timeout: 15000 });
  await expect(citation).toHaveAttribute('data-vtext-citation-transclusion', '');
  await citation.click();
  await expect(citation).toHaveAttribute('data-expanded', 'true');
  await expect(citation.locator('[data-vtext-inline-transclusion]')).toContainText(sourceLabel);
  await expect(citation.locator('[data-vtext-inline-transclusion]')).toContainText(excerpt);

  const initialSourceWindows = await page.locator('[data-content-viewer]').count();
  await citation.locator('[data-vtext-open-source]').click();
  await expect(page.locator('[data-content-viewer]')).toHaveCount(initialSourceWindows + 1, { timeout: 10000 });
  const sourceWindow = page.locator('[data-content-viewer]').last();
  await expect(sourceWindow).toContainText(sourceLabel);
  await expect(sourceWindow).toContainText(excerpt);
  await expect(sourceWindow.locator('[data-source-entity]')).toContainText(sourceEntityID);
});
