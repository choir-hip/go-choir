import { test, expect } from './helpers/fixtures.js';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import {
  normalizeReaderArtifactState,
  normalizeSourceSelectorKind,
  normalizeSourceEvidenceState,
  readerArtifactStateLabel,
  sourceSelectorList,
  sourceOpenPlan,
  sourceEntityID,
  sourceEntityInlineExcerptText,
  sourceEntityReaderSnapshotText,
  sourceEntityReaderFallbackText,
  sourceEntityOpenPlan,
  renderSourceTransclusionBody,
  renderInlineMarkdown,
  publicationSourceEntityToLocal,
  parseTextureRelatedRef,
  selectorTextQuote,
  sourceEvidenceState,
  sourceEvidenceStateLabel,
  textureRelatedMarkdownTarget,
} from '../src/lib/texture-source-renderer.ts';
import { sourceEntityLaunchPayload } from '../src/lib/texture-source-launcher.ts';
import { revisionSourceEntities } from '../src/lib/texture-source-state.ts';
import { browserOpenableSourceURL } from '../src/lib/source-url.ts';

const sourceContractMatrix = JSON.parse(readFileSync(fileURLToPath(
  new URL('../../internal/sourcecontract/testdata/source_contract_matrix.json', import.meta.url),
), 'utf8'));

test('frontend source contract stays aligned with shared matrix', () => {
  for (const item of sourceContractMatrix.evidence_states) {
    expect(normalizeSourceEvidenceState(item.raw), `evidence state ${item.raw}`).toBe(item.want);
  }
  for (const item of sourceContractMatrix.reader_artifact_states) {
    expect(normalizeReaderArtifactState(item.raw), `reader artifact state ${item.raw}`).toBe(item.want);
  }
  for (const item of sourceContractMatrix.selector_kinds) {
    expect(normalizeSourceSelectorKind(item.raw), `selector kind ${item.raw}`).toBe(item.want);
  }
  for (const item of sourceContractMatrix.open_surfaces) {
    expect(sourceOpenPlan({ requestedOpenSurface: item.raw }).openSurface, `open surface ${item.raw}`).toBe(item.want || 'source');
  }
  for (const item of sourceContractMatrix.frontend_open_plans) {
    expect(sourceOpenPlan(item.input), item.name).toMatchObject(item.want);
  }
});

test('source reader exposes only web-safe original links to the browser', () => {
  expect(browserOpenableSourceURL('https://example.com/source-reader-fixture')).toBe('https://example.com/source-reader-fixture');
  expect(browserOpenableSourceURL('http://example.com/source-reader-fixture')).toBe('http://example.com/source-reader-fixture');
  expect(browserOpenableSourceURL('choir://universal-wire/source/source-port-authority')).toBe('');
  expect(browserOpenableSourceURL('source_service_item:srcitem_123')).toBe('');
  expect(browserOpenableSourceURL('/api/content/items/item_123')).toBe('');
});

test('URL-only source entities open as reader fallback, not missing content items', () => {
  const entity = {
    source_entity_id: 'src-url-only',
    kind: 'web_url',
    target: {
      kind: 'web_url',
      uri: 'https://example.com/newsroom',
      id: 'src-url-only',
    },
    display: {
      title: 'Newsroom - Example',
      mode: 'numbered_ref',
    },
    evidence: {
      state: 'available',
      open_surface: 'source',
    },
  };

  const payload = sourceEntityLaunchPayload(entity);

  expect(payload.appId).toBe('content');
  expect(payload.appContext.contentId).toBe('');
  expect(payload.appContext.sourceUrl).toBe('https://example.com/newsroom');
  expect(sourceEntityReaderFallbackText(entity)).toContain('Newsroom - Example');
  expect(sourceEntityReaderFallbackText(entity)).toContain('Original source: https://example.com/newsroom');
});

test('legacy URL-only content_item source ids do not trigger content-item fetches', () => {
  const entity = {
    source_entity_id: 'src-synthetic-url',
    kind: 'content_item',
    target: {
      kind: 'content_item',
      id: 'src-synthetic-url',
      uri: 'https://example.com/legacy-newsroom',
    },
    display: {
      title: 'Legacy newsroom source',
    },
    evidence: {
      state: 'available',
      open_surface: 'source',
    },
  };

  const payload = sourceEntityLaunchPayload(entity);

  expect(payload.appContext.contentId).toBe('');
  expect(payload.appContext.sourceUrl).toBe('https://example.com/legacy-newsroom');
  expect(sourceEntityReaderFallbackText(entity)).toContain('Legacy newsroom source');
});

test('URL-backed source entities prefer preserved source text over title-only fallback', () => {
  const entity = {
    source_entity_id: 'src-url-preserved-text',
    kind: 'web_url',
    target: {
      kind: 'web_url',
      uri: 'https://example.com/policy-source',
    },
    display: {
      title: 'Policy source',
      mode: 'numbered_ref',
    },
    selectors: [{
      kind: 'text_quote',
      data: { exact: 'Bounded source excerpt shown in the Texture stub.' },
    }],
    reader_snapshot: {
      text_content: 'Bounded source excerpt shown in the Texture stub.\n\nFuller researcher-read source text appears in the Source Viewer.',
      source_url: 'https://example.com/policy-source',
      snapshot_kind: 'cleaned_reader_markdown',
      media_type: 'text/markdown',
    },
    reader_snapshot_status: {
      state: 'reader_snapshot_ready',
    },
  };

  expect(sourceEntityInlineExcerptText(entity)).toContain('Bounded source excerpt');
  expect(sourceEntityReaderSnapshotText(entity)).toContain('Fuller researcher-read source text');
  const body = renderSourceTransclusionBody(entity);
  expect(body).toContain('Bounded source excerpt shown in the Texture stub.');
  expect(body).not.toContain('Original source: https://example.com/policy-source');
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

test('related Texture inline refs render as native transclusion refs', () => {
  const html = renderInlineMarkdown(
    'Read the related [grid update](texture:doc-grid-story).',
    [],
    [{
      label: 'grid update',
      title: 'Grid operators add reserve alerts as heat forecast shifts north',
      target: {
        target_kind: 'texture_document',
        doc_id: 'doc-grid-story',
        current_revision_id: 'rev-grid-v2',
        current_version_number: 2,
      },
      transclusion: {
        revision_id: 'rev-grid-v1',
        version_number: 1,
        snapshot_text: 'Forecast changes moved stress toward northern reserve margins.',
      },
    }],
  );

  expect(html).toContain('data-texture-related-ref');
  expect(html).toContain('data-texture-doc-id="doc-grid-story"');
  expect(html).toContain('data-texture-related-revision-id="rev-grid-v1"');
  expect(html).toContain('data-texture-related-version-number="1"');
  expect(html).toContain('data-texture-related-current-revision-id="rev-grid-v2"');
  expect(html).toContain('data-texture-related-current-version-number="2"');
  expect(html).toContain('data-texture-related-has-newer-version="true"');
  expect(html).toContain('data-texture-related-newer-version');
  expect(html).toContain('Forecast changes moved stress toward northern reserve margins.');
});

test('legacy texture inline refs still render as Texture transclusion refs', () => {
  const html = renderInlineMarkdown(
    'Read the related [grid update](texture:doc-grid-story@rev-grid-v1).',
    [],
    [{
      label: 'grid update',
      title: 'Grid operators add reserve alerts as heat forecast shifts north',
      target: {
        target_kind: 'texture_document',
        doc_id: 'doc-grid-story',
      },
    }],
  );

  expect(html).toContain('data-texture-related-ref');
  expect(html).toContain('data-texture-related-revision-id="rev-grid-v1"');
});

test('raw markdown source links do not render as native Texture source refs', () => {
  const html = renderInlineMarkdown(
    'This claim has old source syntax [source label](source:src-raw-link).',
    [{ entity_id: 'src-raw-link', label: 'Source label' }],
    [],
  );

  expect(html).not.toContain('data-texture-source-ref');
  expect(html).toContain('[source label](source:src-raw-link)');
});

test('Texture inline markdown keeps ordinary web links inert unless source-reader mode opts in', () => {
  const raw = 'This claim links to [ordinary context](https://example.com/context).';
  const textureHTML = renderInlineMarkdown(raw, [], []);
  const readerHTML = renderInlineMarkdown(raw, [], [], { linkMode: 'anchor' });

  expect(textureHTML).not.toContain('<a ');
  expect(textureHTML).toContain('[ordinary context](https://example.com/context)');
  expect(readerHTML).toContain('<a href="https://example.com/context"');
  expect(readerHTML).toContain('target="_blank"');
});

test('related Texture refs parse and format pinned revision targets', () => {
  expect(parseTextureRelatedRef('doc-grid-story@rev-grid-v1')).toEqual({
    docID: 'doc-grid-story',
    revisionID: 'rev-grid-v1',
  });
  expect(parseTextureRelatedRef('doc-grid-story')).toEqual({
    docID: 'doc-grid-story',
    revisionID: '',
  });
  expect(textureRelatedMarkdownTarget('doc-grid-story', 'rev-grid-v1')).toBe('doc-grid-story@rev-grid-v1');
  expect(textureRelatedMarkdownTarget('doc-grid-story', '')).toBe('doc-grid-story');
});

test('source evidence states normalize to typed reader labels', () => {
  expect(normalizeSourceEvidenceState('pending')).toBe('candidate');
  expect(normalizeSourceEvidenceState('no-source-needed')).toBe('no_source_needed');
  expect(normalizeSourceEvidenceState('access-blocked')).toBe('blocked_by_access');
  expect(normalizeSourceEvidenceState('error')).toBe('unavailable');
  expect(normalizeSourceEvidenceState('fetch_failed')).toBe('unavailable');
  expect(normalizeSourceEvidenceState('unknown-state')).toBe('');
  expect(sourceEvidenceState({ evidence: { state: 'represented' } })).toBe('confirms');
  expect(sourceEvidenceStateLabel('confirms')).toBe('Confirms claim');
  expect(sourceEvidenceStateLabel('blocked_by_access')).toBe('Blocked by access');
  expect(sourceEvidenceStateLabel('fetch_failed')).toBe('Unavailable source');
  expect(sourceEvidenceStateLabel('reader_snapshot_ready')).toBe('Evidence unclassified');
  expect(normalizeReaderArtifactState('reader_snapshot_ready')).toBe('reader_snapshot_ready');
  expect(readerArtifactStateLabel('reader_snapshot_ready')).toBe('Reader snapshot ready');
  expect(readerArtifactStateLabel('bounded_excerpt_only')).toBe('Bounded excerpt only');
});

test('revisions do not synthesize source entities from legacy media refs', () => {
  const legacyRef = { kind: 'image', url: 'https://example.com/source.png' };
  expect(revisionSourceEntities({ revision: { metadata: { media_source_refs: [legacyRef] } } })).toEqual([]);
  expect(revisionSourceEntities({
    revision: {
      body_doc: { schema: 'choir.texture.doc.v1', doc: { type: 'doc', content: [] } },
      metadata: { media_source_refs: [legacyRef] },
    },
  })).toEqual([]);
});

test('revision source entities prefer publication bundle sources over revision wrappers', () => {
  const entities = revisionSourceEntities({
    revision: {
      source_entities: [{ source_entity_id: 'src-legacy-revision', label: 'Legacy revision source' }],
      source_entity_objects: [{
        canonical_id: 'choir.source_entity:user-1:graph-wrapper',
        version_id: 'ver-graph-wrapper',
        legacy_source_entity_id: 'src-graph-wrapper',
        metadata: {
          source_kind: 'web_url',
          target: { kind: 'web_url', identity: 'https://example.com/graph-wrapper', uri: 'https://example.com/graph-wrapper' },
          display: { title: 'Graph wrapper source' },
          evidence: { state: 'available', open_surface: 'source' },
        },
      }],
    },
    bundle: {
      route: { path: '/pub/texture/source-priority' },
      source_entities: [{
        source_entity_id: 'src-published-priority',
        kind: 'source_service_item',
        target_kind: 'source_service_item',
        target_id: 'source-item-published-priority',
        entity: {
          entity_id: 'src-published-priority',
          kind: 'source_service_item',
          label: 'Published priority source',
          target: { target_kind: 'source_service_item', item_id: 'source-item-published-priority' },
          display: { title: 'Published priority source' },
        },
      }],
    },
  });

  expect(entities.map((entity) => sourceEntityID(entity))).toEqual(['src-published-priority']);
  expect(entities[0].publication_route_path).toBe('/pub/texture/source-priority');
});

test('revision source entities preserve legacy revision fallback before graph wrappers', () => {
  const legacyEntity = {
    source_entity_id: 'src-legacy-fallback',
    target: { kind: 'web_url', uri: 'https://example.com/legacy-fallback' },
    display: { title: 'Legacy fallback source' },
    evidence: { state: 'available', open_surface: 'source' },
  };
  const entities = revisionSourceEntities({
    revision: {
      source_entities: [legacyEntity],
      source_entity_objects: [{
        canonical_id: 'choir.source_entity:user-1:graph-fallback',
        version_id: 'ver-graph-fallback',
        legacy_source_entity_id: 'src-graph-fallback',
        metadata: {
          source_kind: 'web_url',
          target: { kind: 'web_url', identity: 'https://example.com/graph-fallback', uri: 'https://example.com/graph-fallback' },
          display: { title: 'Graph fallback source' },
          evidence: { state: 'available', open_surface: 'source' },
        },
      }],
    },
  });

  expect(entities).toEqual([legacyEntity]);
});

test('revision source entities derive openable local entities from graph wrappers', () => {
  const entities = revisionSourceEntities({
    revision: {
      source_entity_objects: [{
        object_kind: 'choir.source_entity',
        canonical_id: 'choir.source_entity:user-1:graph-open',
        version_id: 'ver-graph-open',
        content_hash: 'sha256:graph-open',
        owner_id: 'user-1',
        body: 'Graph-backed reader snapshot supports the cited claim.',
        legacy_source_entity_id: 'src-graph-open',
        metadata: {
          schema_version: 'choir.source_entity.v1',
          legacy_entity_id: 'src-graph-open',
          source_kind: 'web_url',
          target: {
            kind: 'web_url',
            identity: 'https://example.com/graph-open',
            uri: 'https://example.com/graph-open',
          },
          display: {
            title: 'Graph-backed source',
            label: '1',
            display_mode: 'excerpt',
          },
          evidence: {
            state: 'available',
            open_surface: 'source',
          },
          selectors: [{
            kind: 'text_quote',
            data: { text_quote: 'Graph-backed reader snapshot supports the cited claim.' },
          }],
        },
      }],
      source_refs: [{
        object_kind: 'choir.source_ref',
        canonical_id: 'choir.source_ref:user-1:graph-ref-open',
        version_id: 'ver-graph-ref-open',
        legacy_source_entity_id: 'src-graph-open',
        source_entity_canonical_id: 'choir.source_entity:user-1:graph-open',
        source_entity_version_id: 'ver-graph-open',
        body_node_id: 'ref-graph-open',
        display_mode: 'numbered_ref',
        citation_state: 'cited',
      }],
    },
  });

  expect(entities).toHaveLength(1);
  expect(sourceEntityID(entities[0])).toBe('src-graph-open');
  expect(entities[0]).toMatchObject({
    kind: 'web_url',
    target: {
      kind: 'web_url',
      target_kind: 'web_url',
      uri: 'https://example.com/graph-open',
      canonical_url: 'https://example.com/graph-open',
    },
    reader_snapshot: {
      text_content: 'Graph-backed reader snapshot supports the cited claim.',
      source_url: 'https://example.com/graph-open',
    },
    graph: {
      canonical_id: 'choir.source_entity:user-1:graph-open',
      version_id: 'ver-graph-open',
    },
    source_ref: {
      body_node_id: 'ref-graph-open',
      display_mode: 'numbered_ref',
    },
  });
  expect(sourceEntityOpenPlan(entities[0])).toMatchObject({
    appId: 'content',
    mode: 'source_reader',
    readerMode: true,
  });
  const payload = sourceEntityLaunchPayload(entities[0]);
  expect(payload.appContext.sourceUrl).toBe('https://example.com/graph-open');
  expect(payload.appContext.contentId).toBe('');
  expect(payload.appContext.sourceReaderMode).toBe(true);
  expect(payload.appContext.singletonKey).toBe('source:src-graph-open');
});

test('revision source entities use source_refs to preserve body source ids for shared graph entities', () => {
  const revision = {
    source_entity_objects: [{
      object_kind: 'choir.source_entity',
      canonical_id: 'choir.source_entity:user-1:graph-shared',
      version_id: 'ver-graph-shared',
      content_hash: 'sha256:graph-shared',
      owner_id: 'user-1',
      body: 'Shared graph source body.',
      legacy_source_entity_id: 'src-a',
      metadata: {
        schema_version: 'choir.source_entity.v1',
        legacy_entity_id: 'src-a',
        source_kind: 'web_url',
        target: {
          kind: 'web_url',
          identity: 'https://example.com/shared-graph-source',
          uri: 'https://example.com/shared-graph-source',
        },
        display: { title: 'Shared graph source', label: 'shared' },
        evidence: { state: 'available', open_surface: 'source' },
      },
    }],
    source_refs: [
      {
        object_kind: 'choir.source_ref',
        canonical_id: 'choir.source_ref:user-1:graph-ref-a',
        version_id: 'ver-graph-ref-a',
        legacy_source_entity_id: 'src-a',
        source_entity_canonical_id: 'choir.source_entity:user-1:graph-shared',
        source_entity_version_id: 'ver-graph-shared',
        body_node_id: 'ref-a',
        display_mode: 'numbered_ref',
        citation_state: 'cited',
      },
      {
        object_kind: 'choir.source_ref',
        canonical_id: 'choir.source_ref:user-1:graph-ref-b',
        version_id: 'ver-graph-ref-b',
        legacy_source_entity_id: 'src-b',
        source_entity_canonical_id: 'choir.source_entity:user-1:graph-shared',
        source_entity_version_id: 'ver-graph-shared',
        body_node_id: 'ref-b',
        display_mode: 'expanded_ref',
        citation_state: 'cited',
      },
    ],
  };

  const entities = revisionSourceEntities({ revision });
  expect(entities.map((entity) => sourceEntityID(entity))).toEqual(['src-a', 'src-b']);
  expect(entities.map((entity) => entity.graph.canonical_id)).toEqual([
    'choir.source_entity:user-1:graph-shared',
    'choir.source_entity:user-1:graph-shared',
  ]);
  expect(entities[1]).toMatchObject({
    entity_id: 'src-b',
    source_entity_id: 'src-b',
    source_ref: {
      body_node_id: 'ref-b',
      display_mode: 'expanded_ref',
      source_entity_canonical_id: 'choir.source_entity:user-1:graph-shared',
    },
  });
  expect(sourceEntityLaunchPayload(entities[1]).appContext.singletonKey).toBe('source:src-b');
});

test('multimedia source entities render transcluded media without clickable links', () => {
  const audioHTML = renderSourceTransclusionBody({
    entity_id: 'src-audio',
    kind: 'audio',
    label: 'Audio clip',
    target: { target_kind: 'content_item', canonical_url: 'https://example.com/clip.mp3' },
    evidence: { state: 'available' },
  });
  expect(audioHTML).toContain('<audio');
  expect(audioHTML).not.toContain('<a ');

  const pdfHTML = renderSourceTransclusionBody({
    entity_id: 'src-pdf',
    kind: 'pdf',
    label: 'Report PDF',
    target: { target_kind: 'content_item', canonical_url: 'https://example.com/report.pdf' },
    evidence: { state: 'available' },
  });
  expect(pdfHTML).toContain('type="application/pdf"');
  expect(pdfHTML).not.toContain('<a ');
});

test('source selectors normalize and flatten selector sets', () => {
  expect(normalizeSourceSelectorKind('text quote')).toBe('text_quote');
  expect(normalizeSourceSelectorKind('table-range')).toBe('table_range');
  expect(normalizeSourceSelectorKind('page range')).toBe('page_range');
  expect(normalizeSourceSelectorKind('')).toBe('whole_resource');

  const selectors = sourceSelectorList({
    selector_kind: 'selector-set',
    selectors: [
      {
        selector_kind: 'text quote',
        text_quote: 'Publication selector-set quote.',
      },
      {
        selector_kind: 'table-range',
        table_id: 'appendix-a',
      },
    ],
  });
  expect(selectors).toHaveLength(2);
  expect(selectors[0]).toMatchObject({
    selector_kind: 'text_quote',
    text_quote: 'Publication selector-set quote.',
  });
  expect(selectors[1]).toMatchObject({
    selector_kind: 'table_range',
    table_id: 'appendix-a',
  });
});

test('published source entity quote falls back to transclusion source selector set', () => {
  const entity = publicationSourceEntityToLocal({
    source_entity_id: 'src-published-selector-set',
    kind: 'source_service_item',
    target_kind: 'source_service_item',
    target_id: 'source-item-selector-set',
    display_policy: 'collapsed_citation',
    entity: {
      entity_id: 'src-published-selector-set',
      kind: 'source_service_item',
      label: 'Published selector set source',
      target: {
        target_kind: 'source_service_item',
        item_id: 'source-item-selector-set',
      },
    },
  }, {
    bundle: {
      route: { path: '/pub/texture/selector-set-source' },
      transclusions: [{
        source_entity_id: 'src-published-selector-set',
        source_selector: {
          selector_kind: 'selector_set',
          selectors: [
            {
              selector_kind: 'text_quote',
              text_quote: 'Selector-set quote survives publication reconstruction.',
            },
            {
              selector_kind: 'page_range',
              start_page: 3,
              end_page: 4,
            },
          ],
        },
      }],
    },
  });

  expect(selectorTextQuote(entity)).toBe('Selector-set quote survives publication reconstruction.');
  expect(sourceEntityInlineExcerptText(entity)).toBe('Selector-set quote survives publication reconstruction.');
});

test('Source Viewer renders publication transclusion selector-set quote without flat selectors', async ({ desktopSession }) => {
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

  const quote = 'Publication transclusion selector-set quote renders in Source Viewer.';
  await page.evaluate(async ({ selectorQuote }) => {
    const sourceEntity = {
      entity_id: 'src-transclusion-selector-set',
      kind: 'source_service_item',
      label: 'Selector-set publication source',
      target: {
        target_kind: 'source_service_item',
        item_id: 'source-item-transclusion-selector-set',
      },
      transclusion: {
        source_entity_id: 'src-transclusion-selector-set',
        source_selector: {
          selector_kind: 'selector_set',
          selectors: [
            {
              selector_kind: 'text_quote',
              text_quote: selectorQuote,
            },
            {
              selector_kind: 'page_range',
              start_page: 9,
              end_page: 10,
            },
          ],
        },
      },
      provenance: {
        rights_scope: 'publication_reader',
        created_by: 'browser-test',
      },
    };
    const stateRes = await fetch('/api/desktop/state', {
      method: 'PUT',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        windows: [{
          window_id: 'source-viewer-transclusion-selector-set',
          app_id: 'content',
          title: 'Selector-set publication source',
          geometry: { x: 80, y: 80, width: 820, height: 620 },
          mode: 'normal',
          z_index: 20,
          app_context: {
            windowTitle: 'Selector-set publication source',
            title: 'Selector-set publication source',
            mediaType: 'text/markdown',
            appHint: 'content',
            sourceEntity,
            sourceEntityId: sourceEntity.entity_id,
            publishedRoutePath: '/pub/texture/transclusion-selector-set-fixture',
            publishedGuest: true,
          },
        }],
        active_window_id: 'source-viewer-transclusion-selector-set',
      }),
    });
    if (!stateRes.ok) throw new Error(`desktop state save failed: ${stateRes.status}`);
  }, { selectorQuote: quote });

  await page.reload();
  const viewer = page.locator('[data-window][data-window-id="source-viewer-transclusion-selector-set"] [data-content-viewer]');
  await expect(viewer).toBeVisible({ timeout: 10000 });
  await expect(viewer).toHaveAttribute('data-source-reader-mode', 'true');
  await expect(viewer.locator('[data-content-reader-markdown]')).toContainText(quote);
  await expect(viewer.locator('[data-content-reader-markdown]')).not.toContainText('source-item-transclusion-selector-set');
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
      openSurface: 'web_lens',
      mode: 'live_original',
      liveOriginal: true,
      readerMode: false,
    });
  }

  for (const open_surface of ['source-viewer', 'source_viewer', 'reader', 'content', 'source']) {
    expect(sourceEntityOpenPlan({ ...urlSource, display: { open_surface } })).toMatchObject({
      appId: 'content',
      openSurface: 'source',
      mode: 'source_reader',
      liveOriginal: false,
      readerMode: true,
    });
  }

  expect(sourceEntityOpenPlan(urlSource)).toMatchObject({
    appId: 'content',
    openSurface: 'source',
    mode: 'source_reader',
    readerMode: true,
  });
  expect(sourceOpenPlan({ targetKind: 'source_service_item' })).toMatchObject({
    appId: 'content',
    openSurface: 'source',
    mode: 'source_reader',
    readerMode: true,
  });
  expect(sourceOpenPlan({ requestedOpenSurface: 'browser', hasURL: true })).toMatchObject({
    appId: 'browser',
    openSurface: 'web_lens',
    mode: 'live_original',
    liveOriginal: true,
  });
  expect(sourceOpenPlan({ targetKind: 'publication_version' })).toMatchObject({
    appId: 'texture',
    openSurface: 'texture',
    mode: 'published_texture',
  });
  expect(sourceOpenPlan({ targetKind: 'published_texture_span' })).toMatchObject({
    appId: 'texture',
    openSurface: 'texture',
    mode: 'published_texture',
  });
  expect(sourceEntityOpenPlan({ ...urlSource, kind: 'youtube_video', display: { open_surface: 'video' } })).toMatchObject({
    appId: 'video',
    mode: 'media',
  });
});

test('Texture renders source entities as expandable sources and opens owning media surface', async ({ desktopSession }) => {
  const { page } = desktopSession;
  const created = await page.evaluate(async () => {
    const title = `Source Entity Fixture ${Date.now()}`;
    const docRes = await fetch('/api/texture/documents', {
      method: 'POST',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ title }),
    });
    if (!docRes.ok) throw new Error(`create doc failed: ${docRes.status}`);
    const doc = await docRes.json();
    const sourceURL = 'https://www.youtube.com/watch?v=dQw4w9WgXcQ';
    const sourceEntities = [
      {
        source_entity_id: 'src-fixture-youtube',
        target: { kind: 'video', uri: sourceURL },
        selectors: [{ kind: 'whole_resource' }],
        display: { mode: 'player', title: 'YouTube source fixture', label: 'source' },
        evidence: { state: 'available', open_surface: 'video' },
        provenance: { created_by: 'browser-test', source_system: 'playwright' },
      },
    ];
    const bodyDoc = {
      schema: 'choir.texture_doc.v1',
      doc: {
        type: 'doc',
        attrs: { id: `doc-${doc.doc_id}` },
        content: [
          { type: 'heading', attrs: { id: 'heading-source-entity-fixture', level: 1 }, content: [{ type: 'text', text: 'Source Entity Fixture' }] },
          {
            type: 'paragraph',
            attrs: { id: 'p-source-entity-fixture' },
            content: [
              { type: 'text', text: 'Review this ' },
              { type: 'source_ref', attrs: { id: 'ref-fixture-youtube', source_entity_id: 'src-fixture-youtube', display_mode: 'numbered_ref', label: 'source' } },
              { type: 'text', text: `: ${sourceURL}` },
            ],
          },
        ],
      },
    };
    const revRes = await fetch(`/api/texture/documents/${encodeURIComponent(doc.doc_id)}/revisions`, {
      method: 'POST',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        content: `# Source Entity Fixture\n\nReview this [1]: ${sourceURL}`,
        body_doc: bodyDoc,
        source_entities: sourceEntities,
        author_kind: 'user',
        author_label: 'browser-test',
      }),
    });
    if (!revRes.ok) throw new Error(`create rev failed: ${revRes.status}`);
    return doc;
  });

  await page.locator('[data-desktop-icon-id="texture"]').dblclick();
  const textureWindow = page.locator('[data-texture-app]').last();
  await expect(textureWindow.locator('[data-texture-recent]')).toBeVisible({ timeout: 5000 });
  await textureWindow.locator('[data-texture-recent-document]').filter({ hasText: created.title }).click();

  const rendered = textureWindow.locator('[data-texture-rendered]');
  await expect(rendered.locator('[data-texture-source-ref]')).toBeVisible({ timeout: 10000 });
  await expect(rendered.locator('[data-texture-source-ref]')).toHaveAttribute('data-texture-citation-transclusion', '');
  await rendered.locator('[data-texture-source-ref]').click();
  const citation = rendered.locator('[data-texture-source-ref]');
  await expect(citation).toHaveAttribute('data-expanded', 'true');
  await expect(citation).toHaveAttribute('data-source-expansion-surface', 'media');
  await expect(citation.locator('[data-texture-inline-transclusion]')).toContainText('YouTube source fixture');
  await expect(citation.locator('[data-texture-inline-transclusion] iframe')).toHaveAttribute('src', /youtube\.com\/embed\/dQw4w9WgXcQ/);

  await citation.locator('[data-texture-open-source]').click();
  await expect(page.locator('[data-window]').filter({ hasText: 'YouTube source fixture' }).last()).toBeVisible({ timeout: 10000 });
});

test('Texture opens content-item text sources as reader-mode markdown', async ({ desktopSession }) => {
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
    const docRes = await fetch('/api/texture/documents', {
      method: 'POST',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ title }),
    });
    if (!docRes.ok) throw new Error(`create doc failed: ${docRes.status}`);
    const doc = await docRes.json();
    const sourceEntities = [
      {
        source_entity_id: 'src-reader-mode',
        target: { kind: 'content_item', id: item.content_id, uri: item.source_url },
        selectors: [{
          kind: 'text_quote',
          data: {
            text_quote: 'Full cleaned reader source detail supports the cited claim.',
            content_hash: item.content_hash,
          },
        }],
        display: { mode: 'excerpt', title: 'Reader-mode source fixture', label: '1' },
        evidence: { state: 'available', open_surface: 'source' },
        provenance: { created_by: 'browser-test', source_system: 'playwright' },
      },
    ];
    const bodyDoc = {
      schema: 'choir.texture_doc.v1',
      doc: {
        type: 'doc',
        attrs: { id: `doc-${doc.doc_id}` },
        content: [
          { type: 'heading', attrs: { id: 'heading-content-source-reader', level: 1 }, content: [{ type: 'text', text: 'Content Source Reader Fixture' }] },
          {
            type: 'paragraph',
            attrs: { id: 'p-content-source-reader' },
            content: [
              { type: 'text', text: 'This claim has a cleaned source ' },
              { type: 'source_ref', attrs: { id: 'ref-reader-mode', source_entity_id: 'src-reader-mode', display_mode: 'numbered_ref', label: '1' } },
              { type: 'text', text: '.' },
            ],
          },
        ],
      },
    };
    const revRes = await fetch(`/api/texture/documents/${encodeURIComponent(doc.doc_id)}/revisions`, {
      method: 'POST',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        content: '# Content Source Reader Fixture\n\nThis claim has a cleaned source [1].',
        body_doc: bodyDoc,
        source_entities: sourceEntities,
        author_kind: 'user',
        author_label: 'browser-test',
      }),
    });
    if (!revRes.ok) throw new Error(`create revision failed: ${revRes.status}`);
    return { title, contentID: item.content_id };
  });

  await page.locator('[data-desktop-icon-id="texture"]').dblclick();
  const textureWindow = page.locator('[data-texture-app]').last();
  await expect(textureWindow.locator('[data-texture-recent]')).toBeVisible({ timeout: 5000 });
  await textureWindow.locator('[data-texture-recent-document]').filter({ hasText: created.title }).click();

  const rendered = textureWindow.locator('[data-texture-rendered]');
  const citation = rendered.locator('[data-texture-source-ref][data-source-entity-id="src-reader-mode"]');
  await expect(citation).toBeVisible({ timeout: 10000 });
  await citation.click();
  const flowNote = rendered.locator('[data-texture-source-flow-note]');
  await expect(flowNote).toBeVisible();
  await expect(flowNote).toContainText('Full cleaned reader source detail supports the cited claim');
  await flowNote.locator('[data-texture-open-source][data-source-entity-id="src-reader-mode"]').click();

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
  await page.locator('[data-window-app-id="texture"]').last().click({ position: { x: 24, y: 24 } });
  await flowNote.locator('[data-texture-open-source][data-source-entity-id="src-reader-mode"]').click();
  await expect(page.locator('[data-content-viewer][data-source-reader-mode="true"]')).toHaveCount(1);
});

test('Texture source URL opens Source Viewer unless browser is explicitly requested', async ({ desktopSession }) => {
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
    const docRes = await fetch('/api/texture/documents', {
      method: 'POST',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ title }),
    });
    if (!docRes.ok) throw new Error(`create doc failed: ${docRes.status}`);
    const doc = await docRes.json();
    const sourceEntities = [
      {
        source_entity_id: 'src-url-source-viewer',
        target: { kind: 'web_url', uri: sourceURL },
        selectors: [{ kind: 'text_quote', data: { text_quote: 'Reader snapshot text proves Source Viewer opened instead of Web Lens.' } }],
        display: { mode: 'excerpt', title: 'Source URL routing fixture', label: '1' },
        evidence: { state: 'available', open_surface: 'source', reader_artifact_state: 'reader_snapshot_ready' },
        provenance: { created_by: 'browser-test', source_system: 'playwright' },
      },
      {
        source_entity_id: 'src-url-web-lens',
        target: { kind: 'web_url', uri: sourceURL },
        selectors: [{ kind: 'text_quote', data: { text_quote: 'Explicit browser routing fixture.' } }],
        display: { mode: 'excerpt', title: 'Source URL explicit Web Lens fixture', label: '2' },
        evidence: { state: 'available', open_surface: 'web_lens' },
        provenance: { created_by: 'browser-test', source_system: 'playwright' },
      },
    ];
    const bodyDoc = {
      schema: 'choir.texture_doc.v1',
      doc: {
        type: 'doc',
        attrs: { id: `doc-${doc.doc_id}` },
        content: [
          { type: 'heading', attrs: { id: 'heading-source-url-routing', level: 1 }, content: [{ type: 'text', text: 'Source URL Routing Fixture' }] },
          {
            type: 'paragraph',
            attrs: { id: 'p-source-url-routing-source' },
            content: [
              { type: 'text', text: 'This claim opens a source URL ' },
              { type: 'source_ref', attrs: { id: 'ref-url-source-viewer', source_entity_id: 'src-url-source-viewer', display_mode: 'numbered_ref', label: '1' } },
              { type: 'text', text: '.' },
            ],
          },
          {
            type: 'paragraph',
            attrs: { id: 'p-source-url-routing-web-lens' },
            content: [
              { type: 'text', text: 'This claim explicitly inspects the live original ' },
              { type: 'source_ref', attrs: { id: 'ref-url-web-lens', source_entity_id: 'src-url-web-lens', display_mode: 'numbered_ref', label: '2' } },
              { type: 'text', text: '.' },
            ],
          },
        ],
      },
    };
    const revRes = await fetch(`/api/texture/documents/${encodeURIComponent(doc.doc_id)}/revisions`, {
      method: 'POST',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        content: '# Source URL Routing Fixture\n\nThis claim opens a source URL [1].\n\nThis claim explicitly inspects the live original [2].',
        body_doc: bodyDoc,
        source_entities: sourceEntities,
        author_kind: 'user',
        author_label: 'browser-test',
      }),
    });
    if (!revRes.ok) throw new Error(`create revision failed: ${revRes.status}`);
    return { title };
  });

  await page.locator('[data-desktop-icon-id="texture"]').dblclick();
  const textureWindow = page.locator('[data-texture-app]').last();
  await expect(textureWindow.locator('[data-texture-recent]')).toBeVisible({ timeout: 5000 });
  await textureWindow.locator('[data-texture-recent-document]').filter({ hasText: created.title }).click();

  const rendered = textureWindow.locator('[data-texture-rendered]');
  const citation = rendered.locator('[data-texture-source-ref][data-source-entity-id="src-url-source-viewer"]');
  await expect(citation).toBeVisible({ timeout: 10000 });
  await citation.click();
  await rendered.locator('[data-texture-source-flow-note] [data-texture-open-source]').click();

  const sourceWindow = page.locator('[data-content-viewer]').last();
  await expect(sourceWindow).toBeVisible({ timeout: 10000 });
  await expect(sourceWindow).toHaveAttribute('data-source-reader-mode', 'true');
  await expect(sourceWindow.locator('[data-content-reader-markdown]')).toContainText('Reader snapshot text proves Source Viewer opened');
  await expect(page.locator('[data-browser-app]')).toHaveCount(0);
  await page.locator('[data-window-app-id="texture"]').last().click({ position: { x: 24, y: 24 } });

  const explicitBrowserCitation = rendered.locator('[data-texture-source-ref][data-source-entity-id="src-url-web-lens"]').first();
  await expect(explicitBrowserCitation).toBeVisible({ timeout: 10000 });
  await explicitBrowserCitation.click();
  await rendered.locator('[data-texture-source-flow-note] [data-texture-open-source][data-source-entity-id="src-url-web-lens"]').click();
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
      publication_route_path: '/pub/texture/published-snapshot-fixture',
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
      reader_snapshot_status: {
        state: 'reader_snapshot_ready',
        truncated: false,
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
  await expect(viewer.locator('.source-kicker')).toContainText('Reader snapshot ready');
  await expect(viewer.locator('.source-kicker')).not.toContainText('Evidence unclassified');
  await expect(viewer.locator('[data-content-evidence]')).toContainText('Reader artifact');
  await expect(viewer.locator('[data-content-evidence]')).toContainText('Reader snapshot ready');
  await expect(viewer.locator('[data-content-evidence]')).not.toContainText('SHA-256');
  const reader = viewer.locator('[data-content-reader-markdown]');
  await expect(reader.locator('h2')).toContainText('Publication reader snapshot');
  await expect(reader).toContainText('publication-carried reader snapshot must remain');
  await expect(reader).not.toContainText('Mutable content item body');
  await expect(reader).not.toContainText('Selector quote is only a final fallback');
  await expect(reader).not.toContainText(created.contentID);
});

test('Texture source panel omits retired artifact attachment UI while structured sources still expand', async ({ desktopSession }) => {
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
    const docRes = await fetch('/api/texture/documents', {
      method: 'POST',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ title }),
    });
    if (!docRes.ok) throw new Error(`create doc failed: ${docRes.status}`);
    const doc = await docRes.json();
    const sourceEntities = [
      {
        source_entity_id: 'src-attach-text',
        target: { kind: 'web_url', uri: 'https://example.com/attachable-source-fixture' },
        selectors: [{ kind: 'text_quote', data: { text_quote: 'Readable attachment confirms the cited claim.' } }],
        display: { mode: 'excerpt', title: 'Attachable source fixture', label: '1' },
        evidence: { state: 'available', open_surface: 'source' },
        provenance: { created_by: 'browser-test', source_system: 'playwright' },
      },
    ];
    const bodyDoc = {
      schema: 'choir.texture_doc.v1',
      doc: {
        type: 'doc',
        attrs: { id: `doc-${doc.doc_id}` },
        content: [
          { type: 'heading', attrs: { id: 'heading-source-artifact-attach', level: 1 }, content: [{ type: 'text', text: 'Source Artifact Attach Fixture' }] },
          {
            type: 'paragraph',
            attrs: { id: 'p-source-artifact-attach' },
            content: [
              { type: 'text', text: 'This claim will receive a readable source artifact ' },
              { type: 'source_ref', attrs: { id: 'ref-attach-text', source_entity_id: 'src-attach-text', display_mode: 'numbered_ref', label: '1' } },
              { type: 'text', text: '.' },
            ],
          },
        ],
      },
    };
    const revRes = await fetch(`/api/texture/documents/${encodeURIComponent(doc.doc_id)}/revisions`, {
      method: 'POST',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        content: '# Source Artifact Attach Fixture\n\nThis claim will receive a readable source artifact [1].',
        body_doc: bodyDoc,
        source_entities: sourceEntities,
        author_kind: 'user',
        author_label: 'browser-test',
      }),
    });
    if (!revRes.ok) throw new Error(`create revision failed: ${revRes.status}`);
    return doc;
  });

  await page.locator('[data-desktop-icon-id="texture"]').dblclick();
  const textureWindow = page.locator('[data-texture-app]').last();
  await expect(textureWindow.locator('[data-texture-recent]')).toBeVisible({ timeout: 5000 });
  await textureWindow.locator('[data-texture-recent-document]').filter({ hasText: created.title }).click();

  await textureWindow.locator('[data-texture-source-panel]').click();
  const sourcePanel = textureWindow.locator('[data-texture-source-diagnostics]');
  await expect(sourcePanel).toBeVisible({ timeout: 10000 });
  await expect(sourcePanel.locator('[data-texture-source-artifact-panel]')).toHaveCount(0);
  await expect(sourcePanel.locator('[data-texture-source-artifact-text]')).toHaveCount(0);
  await expect(sourcePanel.locator('[data-texture-attach-source-artifact]')).toHaveCount(0);
  await expect(sourcePanel.locator('[data-texture-import-source-artifact]')).toHaveCount(0);
  await expect(sourcePanel.locator('[data-texture-source-entity-chip]').filter({ hasText: 'Attachable source fixture' })).toBeVisible();

  const rendered = textureWindow.locator('[data-texture-rendered]');
  const citation = rendered.locator('[data-texture-source-ref][data-source-entity-id="src-attach-text"]').first();
  await expect(citation).toBeVisible({ timeout: 10000 });
  await citation.click();
  await expect(citation).toHaveAttribute('data-expanded', 'true');
  await expect(citation.locator('[data-texture-inline-transclusion]')).toContainText('Attachable source fixture');
  await expect(citation.locator('[data-texture-inline-transclusion]')).toContainText('Readable attachment confirms the cited claim.');
});

test('Texture lays out expanded text sources as noncanonical journal flow', async ({ desktopSession }) => {
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
    const docRes = await fetch('/api/texture/documents', {
      method: 'POST',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ title }),
    });
    if (!docRes.ok) throw new Error(`create doc failed: ${docRes.status}`);
    const doc = await docRes.json();
    const sourceEntities = [
      {
        source_entity_id: 'src-fixture-flow',
        target: {
          kind: 'web_url',
          uri: 'https://www.americanbar.org/groups/professional_responsibility/publications/ethics_opinions/aba-formal-opinion-512/',
        },
        selectors: [{
          kind: 'text_quote',
          data: {
            text_quote: 'Lawyers using generative artificial intelligence tools must consider duties including competence, confidentiality, communication, supervision, candor, and reasonable fees.',
            supports: 'ethics guidance',
          },
        }],
        display: { mode: 'excerpt', title: 'ABA Formal Opinion 512 fixture', label: 'ethics guidance' },
        evidence: { state: 'available', open_surface: 'source' },
        provenance: { created_by: 'browser-test', source_system: 'playwright' },
      },
      {
        source_entity_id: 'src-fixture-nested',
        target: {
          kind: 'web_url',
          uri: 'https://www.americanbar.org/groups/professional_responsibility/publications/model_rules_of_professional_conduct/rule_1_6_confidentiality_of_information/',
        },
        selectors: [{
          kind: 'text_quote',
          data: { text_quote: 'A lawyer shall not reveal information relating to the representation of a client unless the client gives informed consent.' },
        }],
        display: { mode: 'excerpt', title: 'ABA Model Rule 1.6 fixture', label: 'confidentiality' },
        evidence: { state: 'available', open_surface: 'source' },
        provenance: { created_by: 'browser-test', source_system: 'playwright' },
      },
    ];
    const paragraphs = [
      'Legal practice now depends on durable work product, governed source memory, and reliable citation review across long client documents. [1]',
      'Second paragraph keeps using the reading measure beside the expanded evidence while preserving [2] as its own citation marker rather than flattening it into prose.',
      'Third paragraph gives the layout enough prose to continue below the source note after the narrow line region ends, using ordinary article text that should not become a separate card or metadata block.',
      'Fourth paragraph proves the article continues in the normal full measure once the source apparatus no longer occupies the right column.',
      'Fifth paragraph gives the verifier another full-width line after the note so the test cannot pass merely because one paragraph happened to wrap narrowly beside the source.',
    ];
    const bodyDoc = {
      schema: 'choir.texture_doc.v1',
      doc: {
        type: 'doc',
        attrs: { id: `doc-${doc.doc_id}` },
        content: [
          { type: 'heading', attrs: { id: 'heading-source-flow', level: 1 }, content: [{ type: 'text', text: 'Source Flow Fixture' }] },
          {
            type: 'paragraph',
            attrs: { id: 'p-source-flow-1' },
            content: [
              { type: 'text', text: 'Legal practice now depends on durable work product, governed source memory, and reliable citation review across long client documents. ' },
              { type: 'source_ref', attrs: { id: 'ref-fixture-flow', source_entity_id: 'src-fixture-flow', display_mode: 'numbered_ref', label: 'ethics guidance' } },
            ],
          },
          {
            type: 'paragraph',
            attrs: { id: 'p-source-flow-2' },
            content: [
              { type: 'text', text: 'Second paragraph keeps using the reading measure beside the expanded evidence while preserving ' },
              { type: 'source_ref', attrs: { id: 'ref-fixture-nested', source_entity_id: 'src-fixture-nested', display_mode: 'numbered_ref', label: 'confidentiality' } },
              { type: 'text', text: ' as its own citation marker rather than flattening it into prose.' },
            ],
          },
          { type: 'paragraph', attrs: { id: 'p-source-flow-3' }, content: [{ type: 'text', text: paragraphs[2] }] },
          { type: 'paragraph', attrs: { id: 'p-source-flow-4' }, content: [{ type: 'text', text: paragraphs[3] }] },
          { type: 'paragraph', attrs: { id: 'p-source-flow-5' }, content: [{ type: 'text', text: paragraphs[4] }] },
        ],
      },
    };
    const revRes = await fetch(`/api/texture/documents/${encodeURIComponent(doc.doc_id)}/revisions`, {
      method: 'POST',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        content: `# Source Flow Fixture\n\n${paragraphs.join('\n\n')}`,
        body_doc: bodyDoc,
        source_entities: sourceEntities,
        author_kind: 'user',
        author_label: 'browser-test',
      }),
    });
    if (!revRes.ok) throw new Error(`create rev failed: ${revRes.status}`);
    return doc;
  });

  await page.locator('[data-desktop-icon-id="texture"]').dblclick();
  const textureWindow = page.locator('[data-texture-app]').last();
  await expect(textureWindow.locator('[data-texture-recent]')).toBeVisible({ timeout: 5000 });
  await textureWindow.locator('[data-texture-recent-document]').filter({ hasText: created.title }).click();

  const rendered = textureWindow.locator('[data-texture-rendered]');
  const citation = rendered.locator('[data-texture-source-ref][data-source-entity-id="src-fixture-flow"]').first();
  await expect(citation).toBeVisible({ timeout: 10000 });
  await expect(citation).toHaveAttribute('data-source-expansion-surface', 'journal');
  await citation.click();
  const flow = rendered.locator('[data-texture-source-flow]');
  await expect(flow).toBeVisible({ timeout: 5000 });
  await expect(flow).toContainText('ABA Formal Opinion 512 fixture');
  await expect(flow).not.toContainText('source available');
  await expect(flow).not.toContainText('public source');
  await expect(citation).toHaveAttribute('data-source-flow-mounted', 'true');
  expect(await rendered.locator('p[data-texture-source-flow-hidden]').count()).toBeGreaterThanOrEqual(2);
  expect(await flow.evaluate((node) => getComputedStyle(node).position)).toBe('relative');
  const note = flow.locator('[data-texture-source-flow-note]');
  expect(await note.evaluate((node) => getComputedStyle(node).position)).toBe('absolute');
  await expect(note.locator('[data-texture-source-flow-note-title]')).toContainText('ABA Formal Opinion 512 fixture');
  await expect(note.locator('[data-texture-source-ref-popover]')).toHaveCount(0);
  await expect(note.locator('[data-texture-source-flow-note-actions] [data-texture-open-source]')).toBeVisible();
  await expect(flow).toHaveAttribute('data-texture-source-flow-routed-lines', /^[3-9]\d*$/);
  const journalGeometry = await flow.evaluate((node) => {
    const note = node.querySelector('[data-texture-source-flow-note]');
    const flowBox = node.getBoundingClientRect();
    const noteBox = note.getBoundingClientRect();
    const noteBottom = note.getBoundingClientRect().bottom - node.getBoundingClientRect().top;
    const besideLines = Array.from(node.querySelectorAll('[data-texture-source-flow-line-beside-note]'));
    const besideLineCount = besideLines.length;
    const sideColumnIsClear = besideLines.every((line) => {
      const lineBox = line.getBoundingClientRect();
      return lineBox.right <= noteBox.left - 10;
    });
    const secondParagraphBesideNote = Array.from(node.querySelectorAll('.texture-source-journal-line')).some((line) => {
      const top = line.getBoundingClientRect().top - node.getBoundingClientRect().top;
      const lineBox = line.getBoundingClientRect();
      return top >= 0 && top < noteBottom && lineBox.right <= noteBox.left - 10 && line.textContent.includes('Second paragraph');
    });
    return { besideLineCount, sideColumnIsClear, secondParagraphBesideNote };
  });
  const continuedBelowFlow = await rendered.evaluate((node) => {
    const flow = node.querySelector('[data-texture-source-flow]');
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
  await expect(note.locator('.texture-source-facts')).toHaveCount(0);
  const nestedCitation = flow.locator('[data-texture-source-ref][data-source-entity-id="src-fixture-nested"]');
  await expect(nestedCitation).toBeVisible();
  await expect(nestedCitation.locator('[data-texture-inline-transclusion]')).toBeHidden();
  await nestedCitation.click();
  const remountedFlow = rendered.locator('[data-texture-source-flow]');
  await expect(remountedFlow).toHaveCount(1);
  await expect(remountedFlow.locator('[data-texture-source-flow-note]')).toContainText('ABA Model Rule 1.6 fixture');
  await expect(remountedFlow.locator('[data-texture-source-flow-note]')).not.toContainText('ABA Formal Opinion 512 fixture');
  const remountedState = await rendered.evaluate((node) => {
    const flow = node.querySelector('[data-texture-source-flow]');
    const mounted = node.querySelector('[data-texture-source-ref][data-source-entity-id="src-fixture-nested"][data-source-flow-mounted="true"]');
    const expandedInsideFlow = flow?.querySelector('[data-texture-source-ref][data-expanded="true"]');
    return {
      owner: flow?.getAttribute('data-source-flow-owner-id') || '',
      hasMountedOriginal: !!mounted && !mounted.closest('[data-texture-source-flow]'),
      hasExpandedInsideFlow: !!expandedInsideFlow,
    };
  });
  expect(remountedState.owner).toBe('src-fixture-nested');
  expect(remountedState.hasMountedOriginal).toBe(true);
  expect(remountedState.hasExpandedInsideFlow).toBe(false);

  await remountedFlow.locator('[data-texture-open-source][data-source-entity-id="src-fixture-nested"]').click();
  const sourceWindow = page.locator('[data-window]').filter({ hasText: 'ABA Model Rule 1.6 fixture' }).last();
  await expect(sourceWindow).toBeVisible({ timeout: 10000 });
});

test('Texture uses stacked journal flow instead of old source card when side routing is unavailable', async ({ desktopSession }) => {
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
    const docRes = await fetch('/api/texture/documents', {
      method: 'POST',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ title }),
    });
    if (!docRes.ok) throw new Error(`create doc failed: ${docRes.status}`);
    const doc = await docRes.json();
    const sourceEntities = [
      {
        source_entity_id: 'src-stacked-flow',
        target: {
          kind: 'web_url',
          uri: 'https://example.com/stacked-source-flow',
        },
        selectors: [{
          kind: 'text_quote',
          data: { text_quote: 'Stacked source notes should remain content-first without reusing the old expanded card surface.' },
        }],
        display: { mode: 'excerpt', title: 'Stacked journal source fixture', label: '1' },
        evidence: { state: 'available', open_surface: 'source' },
        provenance: { created_by: 'browser-test', source_system: 'playwright' },
      },
    ];
    const paragraphs = [
      'This constrained measure still needs the source note to read like journal evidence rather than an expanded card [1], while the article text remains the main object being read.',
      'A second paragraph proves the original source card path is not needed just because a side column is unavailable.',
    ];
    const bodyDoc = {
      schema: 'choir.texture_doc.v1',
      doc: {
        type: 'doc',
        attrs: { id: `doc-${doc.doc_id}` },
        content: [
          { type: 'heading', attrs: { id: 'heading-stacked-source-flow', level: 1 }, content: [{ type: 'text', text: 'Stacked Source Flow Fixture' }] },
          {
            type: 'paragraph',
            attrs: { id: 'p-stacked-source-flow-1' },
            content: [
              { type: 'text', text: 'This constrained measure still needs the source note to read like journal evidence rather than an expanded card ' },
              { type: 'source_ref', attrs: { id: 'ref-stacked-flow', source_entity_id: 'src-stacked-flow', display_mode: 'numbered_ref', label: '1' } },
              { type: 'text', text: ', while the article text remains the main object being read.' },
            ],
          },
          { type: 'paragraph', attrs: { id: 'p-stacked-source-flow-2' }, content: [{ type: 'text', text: paragraphs[1] }] },
        ],
      },
    };
    const revRes = await fetch(`/api/texture/documents/${encodeURIComponent(doc.doc_id)}/revisions`, {
      method: 'POST',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        content: `# Stacked Source Flow Fixture\n\n${paragraphs.join('\n\n')}`,
        body_doc: bodyDoc,
        source_entities: sourceEntities,
        author_kind: 'user',
        author_label: 'browser-test',
      }),
    });
    if (!revRes.ok) throw new Error(`create revision failed: ${revRes.status}`);
    return doc;
  });

  await page.locator('[data-desktop-icon-id="texture"]').dblclick();
  const textureWindow = page.locator('[data-texture-app]').last();
  await expect(textureWindow.locator('[data-texture-recent]')).toBeVisible({ timeout: 5000 });
  await textureWindow.locator('[data-texture-recent-document]').filter({ hasText: created.title }).click();

  const rendered = textureWindow.locator('[data-texture-rendered]');
  await rendered.evaluate((node) => {
    const element = /** @type {HTMLElement} */ (node);
    element.style.width = '560px';
    element.style.maxWidth = '560px';
  });
  const citation = rendered.locator('[data-texture-source-ref][data-source-entity-id="src-stacked-flow"]').first();
  await expect(citation).toBeVisible({ timeout: 10000 });
  await expect(citation).toHaveAttribute('data-source-expansion-surface', 'journal');
  await citation.click();

  const flow = rendered.locator('[data-texture-source-flow]');
  await expect(flow).toBeVisible({ timeout: 5000 });
  await expect(flow).toHaveAttribute('data-texture-source-flow-mode', 'stacked');
  await expect(flow).toHaveAttribute('data-texture-source-flow-routed-lines', '0');
  await expect(citation).toHaveAttribute('data-source-flow-mounted', 'true');
  await expect(flow.locator('[data-texture-source-ref-popover]')).toHaveCount(0);
  await expect(flow.locator('[data-texture-source-flow-note]')).toContainText('Stacked journal source fixture');
  await expect(flow.locator('[data-texture-source-flow-note]')).not.toContainText('source available');

  const geometry = await flow.evaluate((node) => {
    const flowBox = node.getBoundingClientRect();
    const note = node.querySelector('[data-texture-source-flow-note]');
    const lines = Array.from(node.querySelectorAll('.texture-source-journal-line'));
    const noteBox = note?.getBoundingClientRect();
    const lastLineBottom = Math.max(...lines.map((line) => line.getBoundingClientRect().bottom - flowBox.top));
    const lineLayerHasOldCard = !!node.querySelector('.texture-source-ref[data-expanded="true"] .texture-source-ref-popover');
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

test('Texture mobile source journal flow stays within the reader width', async ({ desktopSession }) => {
  const { page } = desktopSession;
  await page.setViewportSize({ width: 390, height: 844 });
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
    const title = `Mobile Source Flow Fixture ${Date.now()}`;
    const docRes = await fetch('/api/texture/documents', {
      method: 'POST',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ title }),
    });
    if (!docRes.ok) throw new Error(`create doc failed: ${docRes.status}`);
    const doc = await docRes.json();
    const sourceEntities = [
      {
        source_entity_id: 'src-mobile-flow',
        target: {
          kind: 'web_url',
          uri: 'https://example.com/mobile-source-flow-with-a-long-url-that-must-not-expand-the-reader-width',
        },
        selectors: [{
          kind: 'text_quote',
          data: { text_quote: 'NixOS declarative configuration helps reproduce system configuration and supports rollback.' },
        }],
        display: { mode: 'excerpt', title: 'NixOS reproducible configuration and rollback', label: '1' },
        evidence: { state: 'available', open_surface: 'source' },
        provenance: { created_by: 'browser-test', source_system: 'playwright' },
      },
    ];
    const paragraphs = [
      'Operating system. The server should run a Linux distribution that supports reproducible, version-controlled system configurations with rollback capability [1]. This normal paragraph must stay aligned with the phone viewport after the source is opened.',
      'Base architecture. The architecture is European host, encrypted storage, vector database, open-weight embeddings, and private inference routing.',
    ];
    const bodyDoc = {
      schema: 'choir.texture_doc.v1',
      doc: {
        type: 'doc',
        attrs: { id: `doc-${doc.doc_id}` },
        content: [
          { type: 'heading', attrs: { id: 'heading-mobile-source-flow', level: 1 }, content: [{ type: 'text', text: 'Mobile Source Flow Fixture' }] },
          {
            type: 'paragraph',
            attrs: { id: 'p-mobile-source-flow-1' },
            content: [
              { type: 'text', text: 'Operating system. The server should run a Linux distribution that supports reproducible, version-controlled system configurations with rollback capability ' },
              { type: 'source_ref', attrs: { id: 'ref-mobile-flow', source_entity_id: 'src-mobile-flow', display_mode: 'numbered_ref', label: '1' } },
              { type: 'text', text: '. This normal paragraph must stay aligned with the phone viewport after the source is opened.' },
            ],
          },
          { type: 'paragraph', attrs: { id: 'p-mobile-source-flow-2' }, content: [{ type: 'text', text: paragraphs[1] }] },
        ],
      },
    };
    const revRes = await fetch(`/api/texture/documents/${encodeURIComponent(doc.doc_id)}/revisions`, {
      method: 'POST',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        content: `# Mobile Source Flow Fixture\n\n${paragraphs.join('\n\n')}`,
        body_doc: bodyDoc,
        source_entities: sourceEntities,
        author_kind: 'user',
        author_label: 'browser-test',
      }),
    });
    if (!revRes.ok) throw new Error(`create revision failed: ${revRes.status}`);
    return doc;
  });

  await page.locator('[data-desktop-icon-id="texture"]').dblclick();
  const textureWindow = page.locator('[data-texture-app]').last();
  await expect(textureWindow.locator('[data-texture-recent]')).toBeVisible({ timeout: 5000 });
  await textureWindow.locator('[data-texture-recent-document]').filter({ hasText: created.title }).click();

  const rendered = textureWindow.locator('[data-texture-rendered]');
  const citation = rendered.locator('[data-texture-source-ref][data-source-entity-id="src-mobile-flow"]').first();
  await expect(citation).toBeVisible({ timeout: 10000 });
  await citation.click();

  const flow = rendered.locator('[data-texture-source-flow]');
  await expect(flow).toBeVisible({ timeout: 5000 });
  await expect(flow.locator('[data-texture-source-flow-note]')).toContainText('NixOS reproducible configuration and rollback');
  await page.screenshot({ path: test.info().outputPath('texture-mobile-source-flow.png'), fullPage: true });

  const geometry = await rendered.evaluate((node) => {
    const rendered = /** @type {HTMLElement} */ (node);
    const flow = rendered.querySelector('[data-texture-source-flow]');
    const note = rendered.querySelector('[data-texture-source-flow-note]');
    const paragraph = Array.from(rendered.querySelectorAll('p')).find((item) => item.textContent?.includes('Base architecture'));
    const renderedBox = rendered.getBoundingClientRect();
    const flowBox = flow?.getBoundingClientRect();
    const noteBox = note?.getBoundingClientRect();
    const paragraphBox = paragraph?.getBoundingClientRect();
    return {
      renderedClientWidth: rendered.clientWidth,
      renderedScrollWidth: rendered.scrollWidth,
      documentClientWidth: document.documentElement.clientWidth,
      documentScrollWidth: document.documentElement.scrollWidth,
      flowLeft: flowBox?.left ?? 0,
      flowRight: flowBox?.right ?? 0,
      noteLeft: noteBox?.left ?? 0,
      noteRight: noteBox?.right ?? 0,
      paragraphLeft: paragraphBox?.left ?? 0,
      renderedLeft: renderedBox.left,
    };
  });

  expect(geometry.renderedScrollWidth - geometry.renderedClientWidth).toBeLessThanOrEqual(2);
  expect(geometry.documentScrollWidth - geometry.documentClientWidth).toBeLessThanOrEqual(2);
  expect(geometry.flowLeft).toBeGreaterThanOrEqual(geometry.renderedLeft - 1);
  expect(geometry.flowRight).toBeLessThanOrEqual(geometry.renderedLeft + geometry.renderedClientWidth + 1);
  expect(geometry.noteLeft).toBeGreaterThanOrEqual(geometry.renderedLeft - 1);
  expect(geometry.noteRight).toBeLessThanOrEqual(geometry.renderedLeft + geometry.renderedClientWidth + 1);
  expect(geometry.paragraphLeft).toBeGreaterThanOrEqual(geometry.renderedLeft - 1);
});

test('legacy content-only markdown tables render as plain structured paragraph text', async ({ desktopSession }) => {
  const { page } = desktopSession;
  const created = await page.evaluate(async () => {
    const title = `Table Roundtrip Fixture ${Date.now()}`;
    const docRes = await fetch('/api/texture/documents', {
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
    const revRes = await fetch(`/api/texture/documents/${encodeURIComponent(doc.doc_id)}/revisions`, {
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
    const revision = await revRes.json();
    return { ...doc, revision };
  });

  await page.locator('[data-desktop-icon-id="texture"]').dblclick();
  const textureWindow = page.locator('[data-texture-app]').last();
  await expect(textureWindow.locator('[data-texture-recent]')).toBeVisible({ timeout: 5000 });
  await textureWindow.locator('[data-texture-recent-document]').filter({ hasText: created.title }).click();

  const rendered = textureWindow.locator('[data-texture-rendered]');
  await expect(rendered.locator('.table-scroll table')).toHaveCount(0);
  await expect(rendered).toContainText('| Term | Definition |');
  await expect(rendered).toContainText('Edit this paragraph to trigger serialization.');
  expect(created.revision?.content).toContain('| Term | Definition |');
  expect(created.revision?.content).toContain('| Tokens per second | A measure of inference speed. |');
  expect(created.revision?.body_doc?.schema).toBe('choir.texture_doc.v1');
  expect(JSON.stringify(created.revision?.body_doc || {})).not.toContain('"table"');
});

test('legacy content-only markdown tables do not expose bounded cell edit controls', async ({ desktopSession }) => {
  const { page } = desktopSession;
  const created = await page.evaluate(async () => {
    const title = `Bounded Table Edit Fixture ${Date.now()}`;
    const docRes = await fetch('/api/texture/documents', {
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
    const revRes = await fetch(`/api/texture/documents/${encodeURIComponent(doc.doc_id)}/revisions`, {
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
    const revision = await revRes.json();
    return { ...doc, revision };
  });

  await page.locator('[data-desktop-icon-id="texture"]').dblclick();
  const textureWindow = page.locator('[data-texture-app]').last();
  await expect(textureWindow.locator('[data-texture-recent]')).toBeVisible({ timeout: 5000 });
  await textureWindow.locator('[data-texture-recent-document]').filter({ hasText: created.title }).click();

  const rendered = textureWindow.locator('[data-texture-rendered]');
  await expect(rendered.locator('.table-scroll table')).toHaveCount(0);
  await expect(rendered).toContainText('| Work product | Durable professional output. |');
  await expect(rendered.locator('tbody td')).toHaveCount(0);
  expect(created.revision?.content).toContain('| Term | Definition |');
  expect(created.revision?.content).toContain('| Work product | Durable professional output. |');
  expect(created.revision?.content).toContain('| Source entity | A citation-backed source object. |');
  expect(created.revision?.content).toContain('| --- | --- |');
  expect(created.revision?.body_doc?.schema).toBe('choir.texture_doc.v1');
  expect(JSON.stringify(created.revision?.body_doc || {})).not.toContain('"table"');
});
