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

async function closeDesktopWindows(page, appIds = ['content', 'vtext']) {
  for (const appId of appIds) {
    const windows = page.locator(`[data-window-app-id="${appId}"]`);
    let count = await windows.count();
    while (count > 0) {
      await windows.last().locator('[data-window-close]').click({ force: true });
      await expect(windows).toHaveCount(count - 1, { timeout: 5000 });
      count -= 1;
    }
  }
}

async function openRecentVTextDocument(page, recentLabel, openedTitle = recentLabel) {
  await closeDesktopWindows(page);
  await page.locator('[data-desktop-icon-id="vtext"]').dblclick();
  const recentWindow = page.locator('[data-window-app-id="vtext"]').filter({ has: page.locator('[data-texture-recent]') }).last();
  await expect(recentWindow.locator('[data-texture-recent]')).toBeVisible({ timeout: 5000 });
  await recentWindow.locator('[data-texture-recent-document]').filter({ hasText: recentLabel }).click();
  const documentWindow = page.locator('[data-window-app-id="vtext"]').filter({ hasText: openedTitle }).last();
  await expect(documentWindow.locator('[data-texture-app]')).toBeVisible({ timeout: 10000 });
  return documentWindow.locator('[data-texture-app]');
}

test('Markdown lineage import resolves known citation markers into expandable source transclusions', async ({ desktopSession }) => {
  const { page } = desktopSession;
  const stamp = Date.now();
  const sourceEntityID = `src-lineage-rule-16-${stamp}`;
  const sourceLabel = 'ABA Model Rule 1.6';
  const excerpt = 'A lawyer shall not reveal information relating to the representation of a client.';

  const imported = await fetchJSON(page, '/api/texture/markdown-lineage/import', {
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

  const revisions = await fetchJSON(page, `/api/texture/documents/${encodeURIComponent(imported.doc_id)}/revisions?limit=10000`);
  expect(revisions.revisions).toHaveLength(1);
  const revision = revisions.revisions[0];
  expect(revision.content).toContain(`[1](source:${sourceEntityID})`);
  expect(revision.content).toContain('One claim still needs source repair [2].');
  expect(revision.content).not.toContain('Confidentiality matters for private legal-cloud work [1].');
  expect(revision.metadata?.source_entities).toHaveLength(1);
  expect(revision.metadata?.source_gaps).toHaveLength(1);
  expect(revision.metadata?.source_gaps?.[0]?.marker).toBe('[2]');
  expect(revision.metadata?.migration_manifest?.citation_resolutions).toEqual([
    { marker: '[1]', action: 'link_source', entity_id: sourceEntityID },
  ]);

  const vtextWindow = await openRecentVTextDocument(page, `Legal Cloud Sourced Lineage ${stamp}`);

  const rendered = vtextWindow.locator('[data-texture-rendered]');
  const citation = rendered.locator('[data-texture-source-ref]').first();
  await expect(citation).toBeVisible({ timeout: 10000 });
  await expect(citation).toHaveAttribute('data-texture-citation-transclusion', '');
  await citation.click();
  await expect(citation).toHaveAttribute('data-expanded', 'true');
  await expect(citation.locator('[data-texture-inline-transclusion]')).toContainText(sourceLabel);
  await expect(citation.locator('[data-texture-inline-transclusion]')).toContainText(excerpt);
  const journalOpenSource = rendered.locator('[data-texture-source-flow-note] [data-texture-open-source]');
  await expect(journalOpenSource).toBeVisible();
  const initialSourceWindows = await page.locator('[data-content-viewer]').count();
  await journalOpenSource.click();
  await expect(page.locator('[data-content-viewer]')).toHaveCount(initialSourceWindows + 1, { timeout: 10000 });
  const sourceWindow = page.locator('[data-content-viewer]').last();
  await expect(sourceWindow).toContainText(sourceLabel);
  await expect(sourceWindow).toContainText(excerpt);
  await expect(sourceWindow.locator('[data-source-entity]')).toContainText(sourceEntityID);
  await expect(rendered).toContainText('One claim still needs source repair [2].');
});

test('Imported Markdown advances from v0 source artifact to canonical .vtext with Markdown export', async ({ desktopSession }) => {
  test.setTimeout(60_000);
  const { page } = desktopSession;
  const stamp = Date.now();
  const sourcePath = `proposals/imported-md-vtext-${stamp}.md`;
  const initialContent = [
    '# Imported Markdown VText Identity',
    '',
    '| Term | Definition |',
    '| --- | --- |',
    '| Work product | Durable professional output. |',
    '',
    'Seeded from Markdown source bytes.',
  ].join('\n');

  const opened = await fetchJSON(page, '/api/texture/files/open', {
    method: 'POST',
    body: JSON.stringify({
      source_path: sourcePath,
      title: `imported-md-vtext-${stamp}.md`,
      initial_content: initialContent,
    }),
  });

  expect(opened.created).toBe(true);
  expect(opened.doc_id).toBeTruthy();
  expect(opened.original_content_id).toBeTruthy();

  const v0Doc = await fetchJSON(page, `/api/texture/documents/${encodeURIComponent(opened.doc_id)}`);
  expect(v0Doc.title).toBe(`imported-md-vtext-${stamp}.vtext`);
  expect(v0Doc.current_version_number).toBe(0);

  const v1Content = [
    '# Imported Markdown VText Identity',
    '',
    '| Term | Definition |',
    '| --- | --- |',
    '| Work product | Durable, reviewable professional output. |',
    '| VText | Canonical editable document identity. |',
    '',
    'Seeded from Markdown source bytes and revised as VText.',
  ].join('\n');

  const v1 = await fetchJSON(page, `/api/texture/documents/${encodeURIComponent(opened.doc_id)}/revisions`, {
    method: 'POST',
    body: JSON.stringify({
      content: v1Content,
      author_kind: 'user',
      author_label: 'browser-test',
      parent_revision_id: v0Doc.current_revision_id,
      metadata: {
        source_path: sourcePath,
        created_from: 'browser_product_path_markdown_v1_proof',
      },
    }),
  });

  expect(v1.version_number).toBe(1);
  expect(v1.parent_revision_id).toBe(v0Doc.current_revision_id);
  expect(v1.metadata?.canonical_vtext_source_path).toMatch(/\.vtext$/);
  expect(v1.metadata?.import_manifest?.source_media_type).toBe('text/markdown');
  expect(v1.metadata?.migration_manifest?.migration_adapter).toBe('markdown_to_vtext_projection');
  expect(v1.content).toContain('| VText | Canonical editable document identity. |');

  const v1Doc = await fetchJSON(page, `/api/texture/documents/${encodeURIComponent(opened.doc_id)}`);
  expect(v1Doc.title).toBe(`imported-md-vtext-${stamp}.vtext`);
  expect(v1Doc.current_revision_id).toBe(v1.revision_id);
  expect(v1Doc.current_version_number).toBe(1);

  const reopenedAlias = await fetchJSON(page, '/api/texture/files/open', {
    method: 'POST',
    body: JSON.stringify({
      source_path: sourcePath,
      title: `imported-md-vtext-${stamp}.md`,
      initial_content: 'Changed original Markdown bytes must not fork canonical VText.',
    }),
  });
  expect(reopenedAlias.created).toBe(false);
  expect(reopenedAlias.doc_id).toBe(opened.doc_id);

  const manifest = await fetchJSON(page, `/api/texture/documents/${encodeURIComponent(opened.doc_id)}/manifest`, {
    method: 'POST',
  });
  expect(manifest.source_path).toMatch(/\.vtext$/);

  const exported = await fetchJSON(page, `/api/texture/documents/${encodeURIComponent(opened.doc_id)}/export?format=md`);
  expect(exported.format).toBe('md');
  expect(exported.filename).toBe(`imported-md-vtext-${stamp}.md`);
  expect(exported.revision_id).toBe(v1.revision_id);
  expect(exported.content).toBe(v1Content);
  expect(exported.content_hash).toBeTruthy();

  const vtextWindow = await openRecentVTextDocument(page, `imported-md-vtext-${stamp}.vtext`);
  await expect(vtextWindow.locator('[data-texture-version]')).toHaveText('v1');
  await expect(vtextWindow.locator('[data-texture-editor-area]')).toContainText('Canonical editable document identity.');
});

test('Imported plain text advances to canonical .vtext with migration metadata and Markdown export', async ({ desktopSession }) => {
  test.setTimeout(60_000);
  const { page } = desktopSession;
  const stamp = Date.now();
  const sourcePath = `notes/imported-text-vtext-${stamp}.txt`;
  const initialContent = [
    'Imported text VText identity',
    '',
    'Plain text source bytes should become canonical VText.',
  ].join('\n');

  const opened = await fetchJSON(page, '/api/texture/files/open', {
    method: 'POST',
    body: JSON.stringify({
      source_path: sourcePath,
      title: `imported-text-vtext-${stamp}.txt`,
      initial_content: initialContent,
    }),
  });

  expect(opened.created).toBe(true);
  expect(opened.doc_id).toBeTruthy();
  expect(opened.original_content_id).toBeTruthy();

  const v0Doc = await fetchJSON(page, `/api/texture/documents/${encodeURIComponent(opened.doc_id)}`);
  expect(v0Doc.title).toBe(`imported-text-vtext-${stamp}.vtext`);
  expect(v0Doc.current_version_number).toBe(0);

  const v1Content = [
    'Imported text VText identity',
    '',
    'Plain text source bytes should become canonical VText.',
    '',
    'The first durable revision preserves the source migration manifest.',
  ].join('\n');

  const v1 = await fetchJSON(page, `/api/texture/documents/${encodeURIComponent(opened.doc_id)}/revisions`, {
    method: 'POST',
    body: JSON.stringify({
      content: v1Content,
      author_kind: 'user',
      author_label: 'browser-test',
      parent_revision_id: v0Doc.current_revision_id,
      metadata: {
        created_from: 'browser_product_path_plain_text_v1_proof',
      },
    }),
  });

  expect(v1.version_number).toBe(1);
  expect(v1.parent_revision_id).toBe(v0Doc.current_revision_id);
  expect(v1.metadata?.canonical_vtext_source_path).toMatch(/\.vtext$/);
  expect(v1.metadata?.import_manifest?.source_media_type).toBe('text/plain');
  expect(v1.metadata?.migration_manifest).toMatchObject({
    source_kind: 'text',
    source_media_type: 'text/plain',
    projection_kind: 'vtext',
    migration_adapter: 'plain_text_to_vtext_projection',
  });

  const reopenedAlias = await fetchJSON(page, '/api/texture/files/open', {
    method: 'POST',
    body: JSON.stringify({
      source_path: sourcePath,
      title: `imported-text-vtext-${stamp}.txt`,
      initial_content: 'Changed original text bytes must not fork canonical VText.',
    }),
  });
  expect(reopenedAlias.created).toBe(false);
  expect(reopenedAlias.doc_id).toBe(opened.doc_id);

  const exported = await fetchJSON(page, `/api/texture/documents/${encodeURIComponent(opened.doc_id)}/export?format=md`);
  expect(exported.format).toBe('md');
  expect(exported.filename).toBe(`imported-text-vtext-${stamp}.md`);
  expect(exported.revision_id).toBe(v1.revision_id);
  expect(exported.content).toBe(v1Content);
  expect(exported.content_hash).toBeTruthy();
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

  const imported = await fetchJSON(page, '/api/texture/markdown-lineage/import', {
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

  const revisions = await fetchJSON(page, `/api/texture/documents/${encodeURIComponent(imported.doc_id)}/revisions?limit=10000`);
  expect(revisions.revisions).toHaveLength(2);
  const historical = revisions.revisions.find((revision) => revision.version_number === 0);
  const latest = revisions.revisions.find((revision) => revision.version_number === 1);
  expect(historical.content).toContain(`[1](source:${sourceEntityID})`);
  expect(latest.content).toContain('| Work product | Durable professional output |');
  expect(historical.metadata?.migration_manifest?.original_content_id).toBe(oldItem.content_id);
  expect(historical.metadata?.migration_manifest?.source_content_item_id).toBe(oldItem.content_id);
  expect(historical.metadata?.migration_manifest?.original_content_source).toBe('content_item');
  expect(historical.metadata?.migration_manifest?.original_content_path).toBe(oldItem.file_path);

  const vtextWindow = await openRecentVTextDocument(page, `Content-backed Legal Cloud ${stamp}`);

  const rendered = vtextWindow.locator('[data-texture-rendered]');
  await expect(rendered.locator('.table-scroll table')).toBeVisible({ timeout: 10000 });
  await expect(rendered).toContainText('Work product');

  const restored = await fetchJSON(page, `/api/texture/documents/${encodeURIComponent(imported.doc_id)}/restore`, {
    method: 'POST',
    body: JSON.stringify({ revision_id: historical.revision_id }),
  });
  expect(restored.revision_id).toBeTruthy();
  await page.reload();
  const restoredWindow = await openRecentVTextDocument(page, `Content-backed Legal Cloud ${stamp}`);
  const restoredRendered = restoredWindow.locator('[data-texture-rendered]');
  const citation = restoredRendered.locator('[data-texture-source-ref]').first();
  await expect(citation).toBeVisible({ timeout: 10000 });
  await citation.click();
  await expect(citation).toHaveAttribute('data-expanded', 'true');
  await expect(citation.locator('[data-texture-inline-transclusion]')).toContainText(sourceLabel);
  await expect(citation.locator('[data-texture-inline-transclusion]')).toContainText(excerpt);
});

test('Migrated source gaps can be repaired as canonical VText revisions', async ({ desktopSession }) => {
  const { page } = desktopSession;
  const stamp = Date.now();
  const sourceEntityID = `src-repaired-gap-${stamp}`;
  const sourceLabel = 'Repaired Legal Source';
  const excerpt = 'Repaired source evidence supports the migrated citation.';

  const imported = await fetchJSON(page, '/api/texture/markdown-lineage/import', {
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

  const revisionsBefore = await fetchJSON(page, `/api/texture/documents/${encodeURIComponent(imported.doc_id)}/revisions?limit=10000`);
  expect(revisionsBefore.revisions).toHaveLength(1);
  expect(revisionsBefore.revisions[0].metadata?.source_gaps?.[0]?.marker).toBe('[2]');

  const repaired = await fetchJSON(page, `/api/texture/documents/${encodeURIComponent(imported.doc_id)}/source-repairs`, {
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
  expect(repaired.metadata?.source_entities?.[0]?.evidence?.state).toBe('confirms');
  expect(repaired.metadata?.source_entities?.[0]?.evidence?.relation).toBe('confirms');
  expect(repaired.metadata?.source_repair_resolutions).toEqual([{
    marker: '[2]',
    action: 'link_source',
    entity_id: sourceEntityID,
    evidence_state: {
      state: 'confirms',
      target_id: sourceEntityID,
    },
  }]);

  const vtextWindow = await openRecentVTextDocument(page, `Source Gap Repair ${stamp}`);

  const rendered = vtextWindow.locator('[data-texture-rendered]');
  const citation = rendered.locator('[data-texture-source-ref]').first();
  await expect(citation).toBeVisible({ timeout: 10000 });
  await expect(citation).toHaveAttribute('data-source-label', '2');
  await citation.click();
  await expect(citation).toHaveAttribute('data-expanded', 'true');
  await expect(citation.locator('[data-texture-inline-transclusion]')).toContainText(sourceLabel);
  await expect(citation.locator('[data-texture-inline-transclusion]')).toContainText(excerpt);
  await expect(rendered.locator('[data-texture-source-flow-note] [data-texture-open-source]')).toBeVisible();
});

test('VText Sources panel applies source-gap repair and opens repaired source window', async ({ desktopSession }) => {
  const { page } = desktopSession;
  const stamp = Date.now();
  const sourceLabel = 'Panel Repair Source';
  const excerpt = 'Panel repair source evidence supports the citation.';

  const imported = await fetchJSON(page, '/api/texture/markdown-lineage/import', {
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

  const vtextWindow = await openRecentVTextDocument(page, `Panel Source Repair ${stamp}`);

  await vtextWindow.locator('[data-texture-source-panel]').click();
  const sourcePanel = vtextWindow.locator('[data-texture-source-diagnostics]');
  await expect(sourcePanel).toBeVisible({ timeout: 10000 });
  await expect(sourcePanel.locator('[data-texture-source-gaps]')).toContainText('[2]');
  await expect(sourcePanel.locator('[data-texture-source-review-panel]')).toBeVisible();
  await expect(sourcePanel.locator('[data-texture-source-review-marker].selected')).toContainText('[2]');
  await expect(sourcePanel.locator('[data-texture-source-repair-payload]')).toHaveCount(0);
  await sourcePanel.locator('[data-texture-source-review-title]').fill(sourceLabel);
  await sourcePanel.locator('[data-texture-source-review-excerpt]').fill(excerpt);
  const repairRequestPromise = page.waitForRequest((request) => request.url().includes('/source-repairs'));
  const repairResponsePromise = page.waitForResponse((response) => response.url().includes('/source-repairs'));
  await sourcePanel.locator('[data-texture-apply-source-review]').click();
  const repairRequest = await repairRequestPromise;
  expect(repairRequest.method()).toBe('POST');
  const repairPayload = JSON.parse(repairRequest.postData() || '{}');
  expect(repairPayload.source_entities?.[0]?.evidence?.state).toBe('confirms');
  expect(repairPayload.source_entities?.[0]?.evidence?.research_state).toBe('owner_supplied');
  expect(repairPayload.source_entities?.[0]?.evidence?.relation).toBe('confirms');
  expect(repairPayload.citation_resolutions?.[0]?.action).toBe('link_source');
  const repairResponse = await repairResponsePromise;
  expect(repairResponse.status()).toBe(201);

  const rendered = vtextWindow.locator('[data-texture-rendered]');
  const citation = rendered.locator('[data-texture-source-ref]').first();
  await expect(citation).toBeVisible({ timeout: 15000 });
  await expect(citation).toHaveAttribute('data-texture-citation-transclusion', '');
  await citation.click();
  await expect(citation).toHaveAttribute('data-expanded', 'true');
  await expect(citation.locator('[data-texture-inline-transclusion]')).toContainText(sourceLabel);
  await expect(citation.locator('[data-texture-inline-transclusion]')).toContainText(excerpt);

  const initialSourceWindows = await page.locator('[data-content-viewer]').count();
  await rendered.locator('[data-texture-source-flow-note] [data-texture-open-source]').click();
  await expect(page.locator('[data-content-viewer]')).toHaveCount(initialSourceWindows + 1, { timeout: 10000 });
  const sourceWindow = page.locator('[data-content-viewer]').last();
  await expect(sourceWindow).toContainText(sourceLabel);
  await expect(sourceWindow).toContainText(excerpt);
  await expect(sourceWindow.locator('[data-source-entity]')).toContainText('Confirms claim / Owner supplied');
  await expect(sourceWindow.locator('[data-source-entity]')).toContainText(/src_review_2_panel_repair_source/);
  await page.locator('[data-window-app-id="content"]').last().locator('[data-window-close]').click();
  await expect(page.locator('[data-content-viewer]')).toHaveCount(initialSourceWindows, { timeout: 10000 });

  await expect(sourcePanel.locator('[data-texture-source-entities]')).toContainText(sourceLabel);
  await expect(sourcePanel.locator('[data-texture-source-entity-chip]').filter({ hasText: sourceLabel })).toContainText('Confirms claim');
  await sourcePanel.locator('[data-texture-source-entity-chip]').filter({ hasText: sourceLabel }).click();
  await expect(page.locator('[data-content-viewer]')).toHaveCount(initialSourceWindows + 1, { timeout: 10000 });
  const panelSourceWindow = page.locator('[data-content-viewer]').last();
  await expect(panelSourceWindow).toContainText(sourceLabel);
  await expect(panelSourceWindow).toContainText(excerpt);
  await expect(panelSourceWindow.locator('[data-source-entity]')).toContainText('Confirms claim / Owner supplied');
  await expect(panelSourceWindow.locator('[data-source-entity]')).toContainText(/src_review_2_panel_repair_source/);
});

test('VText Sources panel can mark a citation gap as no source needed', async ({ desktopSession }) => {
  const { page } = desktopSession;
  const stamp = Date.now();
  const reason = 'This is a framing sentence rather than a factual claim requiring citation.';

  const imported = await fetchJSON(page, '/api/texture/markdown-lineage/import', {
    method: 'POST',
    body: JSON.stringify({
      source_path: `proposals/panel-no-source-needed-${stamp}.md`,
      title: `Panel No Source Needed ${stamp}`,
      versions: [
        {
          label: 'v44',
          source_revision_id: `legacy-panel-no-source-v44-${stamp}`,
          content: [
            '# Panel No Source Needed',
            '',
            'This ordinary framing sentence should not keep a citation marker [2].',
          ].join('\n'),
        },
      ],
    }),
  });

  const vtextWindow = await openRecentVTextDocument(page, `Panel No Source Needed ${stamp}`);

  await vtextWindow.locator('[data-texture-source-panel]').click();
  const sourcePanel = vtextWindow.locator('[data-texture-source-diagnostics]');
  await expect(sourcePanel).toBeVisible({ timeout: 10000 });
  await expect(sourcePanel.locator('[data-texture-source-review-panel]')).toBeVisible();
  await expect(sourcePanel.locator('[data-texture-source-gaps]')).toContainText('[2]');
  await expect(sourcePanel.locator('[data-texture-source-review-marker].selected')).toContainText('[2]');

  await sourcePanel.locator('[data-texture-source-review-relation]').selectOption('no_source_needed');
  await expect(sourcePanel.locator('[data-texture-source-review-title]')).toHaveCount(0);
  await expect(sourcePanel.locator('[data-texture-source-review-excerpt]')).toHaveCount(0);
  await expect(sourcePanel.locator('[data-texture-apply-source-review]')).toBeDisabled();
  await sourcePanel.locator('[data-texture-source-review-reason]').fill(reason);

  const repairRequestPromise = page.waitForRequest((request) => request.url().includes('/source-repairs'));
  const repairResponsePromise = page.waitForResponse((response) => response.url().includes('/source-repairs'));
  await sourcePanel.locator('[data-texture-apply-source-review]').click();
  const repairRequest = await repairRequestPromise;
  expect(repairRequest.method()).toBe('POST');
  const repairPayload = JSON.parse(repairRequest.postData() || '{}');
  expect(repairPayload.source_entities || []).toHaveLength(0);
  expect(repairPayload.citation_resolutions).toEqual([
    {
      marker: '[2]',
      action: 'no_source_needed',
      reason,
    },
  ]);
  const repairResponse = await repairResponsePromise;
  expect(repairResponse.status()).toBe(201);

  const revisions = await fetchJSON(page, `/api/texture/documents/${encodeURIComponent(imported.doc_id)}/revisions?limit=10000`);
  const latest = revisions.revisions.find((revision) => revision.version_number === 1);
  expect(latest.content).toContain('This ordinary framing sentence should not keep a citation marker.');
  expect(latest.content).not.toContain('[2]');
  expect(latest.metadata?.source_entities).toBeUndefined();
  expect(latest.metadata?.source_gaps).toBeUndefined();
  expect(latest.metadata?.source_repair_resolutions).toEqual([
    {
      marker: '[2]',
      action: 'no_source_needed',
      reason,
      evidence_state: {
        state: 'no_source_needed',
        reason,
      },
    },
  ]);

  const rendered = vtextWindow.locator('[data-texture-rendered]');
  await expect(rendered).toContainText('This ordinary framing sentence should not keep a citation marker.');
  await expect(rendered).not.toContainText('[2]');
  await expect(rendered.locator('[data-texture-source-ref]')).toHaveCount(0);
  await expect(sourcePanel.locator('[data-texture-source-gaps]')).toHaveCount(0, { timeout: 15000 });
});

test('VText Sources panel can cancel diagnosis without blocking source review', async ({ desktopSession }) => {
  const { page } = desktopSession;
  const stamp = Date.now();
  let releaseDiagnosis = null;
  const diagnosisRoute = '**/api/texture/documents/*/diagnosis?*';

  const imported = await fetchJSON(page, '/api/texture/markdown-lineage/import', {
    method: 'POST',
    body: JSON.stringify({
      source_path: `proposals/cancel-source-diagnosis-${stamp}.md`,
      title: `Cancel Source Diagnosis ${stamp}`,
      versions: [
        {
          label: 'v44',
          source_revision_id: `legacy-cancel-diagnosis-v44-${stamp}`,
          content: [
            '# Cancel Source Diagnosis',
            '',
            'This claim keeps source review available while diagnosis is pending [2].',
          ].join('\n'),
        },
      ],
    }),
  });
  expect(imported.doc_id).toBeTruthy();

  await page.route(diagnosisRoute, async (route) => {
    await new Promise((resolve) => {
      releaseDiagnosis = resolve;
    });
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ revisions: [], runs: [] }),
    }).catch(() => {});
  });

  try {
    const vtextWindow = await openRecentVTextDocument(page, `Cancel Source Diagnosis ${stamp}`);
    await vtextWindow.locator('[data-texture-source-panel]').click();
    const sourcePanel = vtextWindow.locator('[data-texture-source-diagnostics]');
    const diagnosisButton = sourcePanel.locator('[data-texture-load-diagnosis]');
    await expect(sourcePanel).toBeVisible({ timeout: 10000 });
    await expect(sourcePanel.locator('[data-texture-source-review-panel]')).toBeVisible();
    await expect(sourcePanel.locator('[data-texture-source-gaps]')).toContainText('[2]');

    const diagnosisRequest = page.waitForRequest((request) => request.url().includes('/diagnosis'));
    await diagnosisButton.click();
    await diagnosisRequest;
    await expect(diagnosisButton).toHaveText('Cancel diagnosis');
    await expect(sourcePanel.locator('[data-texture-apply-source-review]')).toBeDisabled();

    await diagnosisButton.click();
    await expect(diagnosisButton).toHaveText('Diagnosis', { timeout: 5000 });
    await expect(vtextWindow.locator('[data-texture-save-status]')).toContainText('Source diagnosis cancelled');
    await expect(sourcePanel.locator('[data-texture-source-review-panel]')).toBeVisible();
    await expect(sourcePanel.locator('[data-texture-source-gaps]')).toContainText('[2]');
  } finally {
    if (releaseDiagnosis) releaseDiagnosis();
    await page.unroute(diagnosisRoute);
  }
});

test('VText Sources panel shows structured edit evidence without raw prompts', async ({ desktopSession }) => {
  const { page } = desktopSession;
  const stamp = Date.now();
  const doc = await fetchJSON(page, '/api/texture/documents', {
    method: 'POST',
    body: JSON.stringify({
      title: `Edit Evidence Fixture ${stamp}`,
    }),
  });

  await fetchJSON(page, `/api/texture/documents/${encodeURIComponent(doc.doc_id)}/revisions`, {
    method: 'POST',
    body: JSON.stringify({
      content: [
        '# Edit Evidence Fixture',
        '',
        'This revision carries structured edit metadata for diagnosis.',
      ].join('\n'),
      metadata: {
        source: 'patch_texture',
        vtext_context_mode: 'focused_user_edit_diff',
        vtext_edit_operation: 'apply_edits',
        vtext_edit_count: 2,
        vtext_run_prompt_chars: 9382,
        vtext_edit_delta_chars: -41,
        vtext_run_latency_ms: 1275,
        original_prompt: 'raw prompt text must stay out of the diagnosis panel',
      },
    }),
  });

  const vtextWindow = await openRecentVTextDocument(page, `Edit Evidence Fixture ${stamp}`);

  await vtextWindow.locator('[data-texture-source-panel]').click();
  const editEvidence = vtextWindow.locator('[data-texture-edit-evidence]');
  await expect(editEvidence).toBeVisible({ timeout: 10000 });
  await expect(editEvidence.locator('[data-texture-edit-context-mode]')).toContainText('focused_user_edit_diff');
  await expect(editEvidence.locator('[data-texture-edit-operation]')).toContainText('apply_edits');
  await expect(editEvidence.locator('[data-texture-edit-prompt-chars]')).toContainText('9382');
  await expect(editEvidence.locator('[data-texture-edit-count]')).toContainText('2');
  await expect(editEvidence.locator('[data-texture-edit-delta-chars]')).toContainText('-41');
  await expect(editEvidence.locator('[data-texture-edit-latency-ms]')).toContainText('1275');
  await expect(editEvidence).not.toContainText('raw prompt text must stay out');
  await expect(vtextWindow.locator('[data-texture-rendered]')).not.toContainText('focused_user_edit_diff');
});

test('VText Sources panel shows bounded revision structure without body text', async ({ desktopSession }) => {
  const { page } = desktopSession;
  const stamp = Date.now();
  const doc = await fetchJSON(page, '/api/texture/documents', {
    method: 'POST',
    body: JSON.stringify({
      title: `Structure Evidence Fixture ${stamp}`,
    }),
  });

  await fetchJSON(page, `/api/texture/documents/${encodeURIComponent(doc.doc_id)}/revisions`, {
    method: 'POST',
    body: JSON.stringify({
      content: [
        '# Structure Evidence Fixture',
        '',
        '| Term | Meaning |',
        '| --- | --- |',
        '| Work product | Durable output [source](source:src-structure-evidence) |',
        '',
      ].join('\n'),
      metadata: {
        source: 'owner_structure_probe',
      },
    }),
  });

  const diagnosisRoute = `**/api/texture/documents/${encodeURIComponent(doc.doc_id)}/diagnosis?*`;
  await page.route(diagnosisRoute, async (route) => {
    expect(route.request().url()).toContain('include_content=false');
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        owner_id: 'user-1',
        doc_id: doc.doc_id,
        store_path: '',
        vtext_path: '',
        document: doc,
        revisions: [],
        revision_structures: Array.from({ length: 18 }, (_, index) => {
          const version = 87 - index;
          return {
            revision_id: `rev-structure-ui-v${version}`,
            doc_id: doc.doc_id,
            version_number: version,
            author_kind: 'user',
            author_label: 'browser-test',
            created_at: '2026-06-06T11:40:00.000Z',
            content_hash: `sha256:${String(version).padStart(64, 'a')}`,
            line_count: 6,
            non_empty_line_count: 4,
            heading_count: 1,
            source_marker_count: version >= 83 ? 1 : 0,
            table_count: 1,
            table_row_count: 3,
            tables: [{
              index: 0,
              start_line: 3,
              end_line: 5,
              column_count: 2,
              row_count: 3,
              has_separator: true,
              signature: `sha256:${String(version).padStart(64, 'b')}`,
            }],
          };
        }),
        runs: [],
        events: [],
        messages: [],
        evidence: [],
      }),
    });
  });

  const vtextWindow = await openRecentVTextDocument(page, `Structure Evidence Fixture ${stamp}`);
  try {
    await vtextWindow.locator('[data-texture-source-panel]').click();
    await vtextWindow.locator('[data-texture-load-diagnosis]').click();

    const structureSummary = vtextWindow.locator('[data-texture-structure-summary]');
    await expect(structureSummary).toBeVisible({ timeout: 10000 });
    await expect(structureSummary).toContainText('bounded summaries');
    await expect(structureSummary).toContainText('tables');
    await expect(structureSummary).toContainText('sources');
    await expect(structureSummary).toContainText('v78');
    await expect(structureSummary).toContainText('v70');
    const structureRevision = structureSummary.locator('[data-texture-structure-revision]').first();
    await expect(structureRevision).toContainText('table 1');
    await expect(structureRevision).toContainText('2c/3r');
    const signature = await structureRevision.locator('[data-texture-table-signature]').first().getAttribute('data-table-signature');
    expect(signature || '').toMatch(/^sha256:/);
    await expect(structureSummary).not.toContainText('Work product');
    await expect(structureSummary).not.toContainText('Durable output');
  } finally {
    await page.unroute(diagnosisRoute);
  }
});

test('VText Sources panel shows off-document decision notes separately', async ({ desktopSession }) => {
  const { page } = desktopSession;
  const stamp = Date.now();
  const doc = await fetchJSON(page, '/api/texture/documents', {
    method: 'POST',
    body: JSON.stringify({
      title: `Decision Evidence Fixture ${stamp}`,
    }),
  });

  await fetchJSON(page, `/api/texture/documents/${encodeURIComponent(doc.doc_id)}/revisions`, {
    method: 'POST',
    body: JSON.stringify({
      content: [
        '# Decision Evidence Fixture',
        '',
        'The reader-facing document does not contain agent process rationale.',
      ].join('\n'),
      metadata: {
        source: 'owner_decision_probe',
      },
    }),
  });

  const decisionReason = 'Owner supplied source excerpt, so VText skipped researcher for this revision.';
  const diagnosisRoute = `**/api/texture/documents/${encodeURIComponent(doc.doc_id)}/diagnosis?*`;
  await page.route(diagnosisRoute, async (route) => {
    expect(route.request().url()).toContain('include_content=false');
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        owner_id: 'user-1',
        doc_id: doc.doc_id,
        store_path: '',
        vtext_path: '',
        document: doc,
        revisions: [],
        revision_structures: [],
        runs: [],
        events: [],
        messages: [],
        decisions: [{
          decision_id: 'decision-ui-1',
          owner_id: 'user-1',
          doc_id: doc.doc_id,
          loop_id: 'run-vtext-decision-ui',
          trajectory_id: 'trajectory-vtext-decision-ui',
          actor_id: 'vtext:' + doc.doc_id,
          decision_kind: 'delegation_skipped',
          reason: decisionReason,
          evidence_refs: ['rev-owner-source', 'source:owner-excerpt'],
          next_action: 'Use patch_texture for the reader-facing revision.',
          created_at: '2026-06-14T20:00:00.000Z',
        }],
        evidence: [],
      }),
    });
  });

  const vtextWindow = await openRecentVTextDocument(page, `Decision Evidence Fixture ${stamp}`);
  try {
    await vtextWindow.locator('[data-texture-source-panel]').click();
    await vtextWindow.locator('[data-texture-load-diagnosis]').click();

    const decisions = vtextWindow.locator('[data-texture-decisions]');
    await expect(decisions).toBeVisible({ timeout: 10000 });
    await expect(decisions).toContainText('VText decisions');
    await expect(decisions.locator('[data-texture-decision]')).toHaveAttribute('data-decision-kind', 'delegation_skipped');
    await expect(decisions).toContainText(decisionReason);
    await expect(decisions).toContainText('Use patch_texture for the reader-facing revision.');
    await expect(decisions.locator('[data-texture-decision-refs]')).toContainText('source:owner-excerpt');
    await expect(vtextWindow.locator('[data-texture-rendered]')).not.toContainText(decisionReason);
  } finally {
    await page.unroute(diagnosisRoute);
  }
});
