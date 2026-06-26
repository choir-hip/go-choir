<script>
  import { createEventDispatcher, onMount } from 'svelte';
  import { sourceEntityLaunchPayload } from './texture-source-launcher';

  export let currentUser = null;
  export let authenticated = false;

  const dispatch = createEventDispatcher();

  let stories = [];
  let selectedStoryId = '';
  let dataSource = 'universal-wire-texture-index';
  let feedDiagnostics = null;
  let visibleDiagnostics = [];
  let loadError = '';
  let lastSuccessfulLoadKey = '';
  let loadInFlight = false;
  let retryTimer = null;
  let refreshTimer = null;

  $: selectedStory = stories.find((story) => story.id === selectedStoryId) || stories[0] || null;
  $: visibleDiagnostics = diagnosticRows(feedDiagnostics);

  onMount(() => {
    loadUniversalWireTextures({ force: true });
    refreshTimer = setInterval(() => {
      if (authenticated) loadUniversalWireTextures({ force: true, silent: true });
    }, 30000);
    const handleFocus = () => {
      if (authenticated) loadUniversalWireTextures({ force: true, silent: true });
    };
    const handleVisibility = () => {
      if (!document.hidden && authenticated) loadUniversalWireTextures({ force: true, silent: true });
    };
    window.addEventListener('focus', handleFocus);
    document.addEventListener('visibilitychange', handleVisibility);
    return () => {
      clearTimeout(retryTimer);
      clearInterval(refreshTimer);
      window.removeEventListener('focus', handleFocus);
      document.removeEventListener('visibilitychange', handleVisibility);
    };
  });

  $: if (authenticated) loadUniversalWireTextures();

  function scheduleAuthenticatedRetry() {
    clearTimeout(retryTimer);
    retryTimer = setTimeout(() => {
      if (authenticated) loadUniversalWireTextures({ force: true, silent: true });
    }, 3000);
  }

  async function loadUniversalWireTextures({ force = false, silent = false } = {}) {
    const loadKey = authenticated ? 'authenticated' : 'preview';
    if (loadInFlight) return;
    if (!force && lastSuccessfulLoadKey === loadKey) return;
    if (!authenticated) {
      loadError = '';
      stories = [];
      selectedStoryId = '';
      dataSource = 'universal-wire-texture-index';
      feedDiagnostics = null;
      lastSuccessfulLoadKey = loadKey;
      return;
    }
    loadInFlight = true;
    if (!silent) loadError = '';
    try {
      const response = await fetch('/api/universal-wire/stories', { credentials: 'include' });
      if (!response.ok) throw new Error(`Universal Wire load failed: ${response.status}`);
      const payload = await response.json();
      if (Array.isArray(payload.stories)) {
        stories = payload.stories;
        dataSource = payload.source || 'universal-wire-texture-index';
        feedDiagnostics = payload.diagnostics || null;
        if (stories.length && !stories.some((story) => story.id === selectedStoryId)) selectedStoryId = stories[0].id;
        if (!stories.length) selectedStoryId = '';
      }
      clearTimeout(retryTimer);
      loadError = '';
      lastSuccessfulLoadKey = loadKey;
    } catch (error) {
      if (!silent) loadError = error?.message || 'Universal Wire load failed';
      scheduleAuthenticatedRetry();
    } finally {
      loadInFlight = false;
    }
  }

  function storyRelatedTextures(story = selectedStory) {
    return (story.related || [])
      .map((storyId) => {
        const related = stories.find((item) => item.id === storyId);
        if (!related) return null;
        return {
          entity_id: `gw-texture-${storyId}`,
          label: related.headline,
          title: related.headline,
          target: {
            target_kind: 'texture_document',
            doc_id: related.story_texture_doc_id || related.story_texture_doc_id || storyId,
            story_id: storyId,
          },
          transclusion: {
            snapshot_text: related.dek || related.projections?.['wire-style'] || '',
            relation: 'related_story',
          },
          provenance: {
            created_from: 'universal_wire',
            source: 'universal_wire_related_story_index',
          },
        };
      })
      .filter(Boolean);
  }

  function stringValue(value) {
    return String(value || '').trim();
  }

  function firstString(...values) {
    for (const value of values) {
      const normalized = stringValue(value);
      if (normalized) return normalized;
    }
    return '';
  }

  function firstOpenableManifestItem(story) {
    const lead = Array.isArray(story?.manifest?.lead) ? story.manifest.lead : [];
    return lead.find((item) => firstString(item?.canonical_url, item?.content_id, item?.source_id, item?.id)) || null;
  }

  function storySourceControls(story) {
    const item = firstOpenableManifestItem(story);
    if (!item) return [];
    const openSurface = firstString(item.open_surface, 'source');
    const controls = [{
      kind: 'source',
      label: 'Open source',
      sourceItem: item,
      openSurface,
    }];
    const liveOpenSurface = firstString(item.live_open_surface);
    if (liveOpenSurface && liveOpenSurface !== openSurface) {
      controls.push({
        kind: 'live',
        label: 'Web Lens',
        sourceItem: item,
        openSurface: liveOpenSurface,
      });
    }
    return controls;
  }

  function wireSourceItemEntity(item, openSurface = '') {
    if (!item) return null;
    const canonicalURL = firstString(item.canonical_url);
    const sourceKind = firstString(item.source_kind, canonicalURL ? 'web_url' : '', item.object_kind, 'source');
    const targetKind = firstString(
      item.target_kind,
      item.content_id ? 'content_item' : '',
      item.source_id ? 'source_service_item' : '',
      canonicalURL ? 'web_url' : '',
      sourceKind,
    );
    const entityID = firstString(item.id, item.canonical_id, item.content_id, item.source_id, canonicalURL);
    if (!entityID) return null;
    const title = firstString(item.title, canonicalURL, entityID);
    const target = {
      kind: targetKind,
      target_kind: targetKind,
      id: firstString(item.content_id, item.source_id, item.id),
      content_id: firstString(item.content_id),
      item_id: firstString(item.source_id),
      canonical_url: canonicalURL,
      url: canonicalURL,
      uri: canonicalURL,
    };
    const readerArtifactState = firstString(item.reader_artifact_state);
    const entity = {
      entity_id: entityID,
      source_entity_id: entityID,
      kind: sourceKind,
      label: title,
      target,
      display: {
        title,
        label: title,
        open_surface: firstString(openSurface, item.open_surface, 'source'),
      },
      evidence: {
        state: 'available',
        open_surface: firstString(openSurface, item.open_surface, 'source'),
        reader_artifact_state: readerArtifactState,
      },
      provenance: {
        created_from: 'universal_wire',
        source: 'universal_wire_manifest',
      },
      graph: {
        object_kind: firstString(item.object_kind),
        canonical_id: firstString(item.canonical_id),
        version_id: firstString(item.version_id),
        content_hash: firstString(item.content_hash),
      },
    };
    if (readerArtifactState) {
      entity.reader_snapshot_status = { state: readerArtifactState };
    }
    return entity;
  }

  function openStorySource(story, sourceItem, openSurface = '') {
    if (!story || !sourceItem) return;
    selectedStoryId = story.id;
    const entity = wireSourceItemEntity(sourceItem, openSurface);
    const payload = sourceEntityLaunchPayload(entity);
    if (payload) dispatch('launchapp', payload);
  }

  function launchTexture({ title, content, createdFrom, sourcePath = '', docId = '', createInitialVersion = false, relatedTextures = [] }) {
    dispatch('launchapp', {
      appId: 'texture',
      appName: 'Texture',
      icon: '📝',
      appContext: {
        windowTitle: title,
        initialContent: content,
        docId,
        createInitialVersion,
        createdFrom,
        sourcePath,
        relatedTextures,
        appHint: 'universal-wire',
        platformRead: true,
        allowMultiple: true,
      },
    });
  }

  function openStoryTexture(story = selectedStory) {
    if (!story) return;
    selectedStoryId = story.id;
    const docId = story.story_texture_doc_id || story.story_texture_doc_id || '';
    if (!docId) return;
    launchTexture({
      title: story.headline,
      content: '',
      createdFrom: 'universal_wire_article',
      sourcePath: `universal-wire/${story.id}.story.texture`,
      docId,
      createInitialVersion: false,
      relatedTextures: storyRelatedTextures(story),
    });
  }

  function diagnosticRows(diagnostics) {
    const rows = Array.isArray(diagnostics?.substrates) ? diagnostics.substrates : [];
    return rows.filter((item) => firstString(item?.substrate, item?.state, item?.reason));
  }

  function diagnosticLabel(substrate) {
    switch (substrate) {
      case 'texture_edition':
        return 'Texture edition';
      case 'web_capture_graph':
        return 'Graph captures';
      case 'source_provenance':
        return 'Source provenance';
      default:
        return 'Feed substrate';
    }
  }

  function diagnosticText(item) {
    const reason = firstString(item?.reason);
    const counts = [];
    if (Number.isFinite(item?.candidate_count)) counts.push(`${item.candidate_count} candidates`);
    if (Number.isFinite(item?.story_count)) counts.push(`${item.story_count} stories`);
    if (Number.isFinite(item?.filtered_count) && item.filtered_count > 0) counts.push(`${item.filtered_count} filtered`);
    return counts.length ? `${reason} (${counts.join(', ')})` : reason;
  }
</script>

<section class="universal-wire" data-universal-wire-app data-universal-wire-data-source={dataSource}>
  <header class="wire-masthead">
    <div>
      <p class="kicker">Living source network</p>
      <h2>Universal Wire</h2>
    </div>
    <div class="wire-state" data-universal-wire-state>
      <strong>{stories.length.toLocaleString()} article{stories.length === 1 ? '' : 's'}</strong>
    </div>
  </header>

  {#if loadError}
    <p class="wire-load-error">{loadError}</p>
  {/if}

  <main class="wire-paper">
    <section class="wire-edition" data-universal-wire-front-page aria-label="Universal Wire articles">
      {#if stories.length}
        <div class="article-columns">
          {#each stories as story}
            <article
              class="wire-article"
              data-universal-wire-story
              data-selected={story.id === selectedStory?.id ? 'true' : 'false'}
              data-story-id={story.id}
              on:mouseenter={() => (selectedStoryId = story.id)}
              on:focusin={() => (selectedStoryId = story.id)}
            >
              <p class="article-meta">{story.freshness}</p>
              <button
                type="button"
                class="headline-button"
                data-universal-wire-open-texture
                on:click={() => openStoryTexture(story)}
              >
                {story.headline}
              </button>
              <p class="dek">{story.dek}</p>
              {#if storySourceControls(story).length}
                <div class="source-actions" data-universal-wire-source-actions>
                  {#each storySourceControls(story) as control}
                    <button
                      type="button"
                      class="source-action"
                      data-universal-wire-open-source={control.kind === 'source' ? '' : undefined}
                      data-universal-wire-open-live-source={control.kind === 'live' ? '' : undefined}
                      on:click={() => openStorySource(story, control.sourceItem, control.openSurface)}
                    >
                      {control.label}
                    </button>
                  {/each}
                </div>
              {/if}
              <p class="projection" data-universal-wire-story-reader>{story.projections?.['wire-style']}</p>
              <div class="claims" data-universal-wire-claims>
                {#each (story.claims || []).slice(0, 2) as claim}
                  <p>{claim}</p>
                {/each}
              </div>
            </article>
          {/each}
        </div>
      {:else}
        <section class="wire-empty-state" data-universal-wire-empty-state>
          <h1>No Wire edition articles yet</h1>
          <p>Universal Wire will show Texture-owned articles here after platform source processing and Texture authoring publish an edition.</p>
          {#if visibleDiagnostics.length}
            <div class="empty-diagnostics" data-universal-wire-empty-diagnostics aria-label="Feed diagnostics">
              {#each visibleDiagnostics as item}
                <p data-universal-wire-empty-diagnostic={item.substrate}>
                  <strong>{diagnosticLabel(item.substrate)}</strong>
                  <span>{diagnosticText(item)}</span>
                </p>
              {/each}
            </div>
          {/if}
        </section>
      {/if}
    </section>
  </main>
</section>

<style>
  :global(.universal-wire-content) {
    overflow: auto;
  }

  .universal-wire {
    min-height: 100%;
    color: var(--choir-text-primary);
    background: var(--choir-surface-app);
    font-family: var(--choir-font-ui);
    padding: clamp(18px, 3vw, 34px);
  }

  .wire-masthead,
  .wire-paper {
    width: min(1320px, 100%);
    margin: 0 auto;
  }

  .wire-masthead {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    gap: 24px;
    margin-bottom: clamp(22px, 4vw, 44px);
  }

  .kicker,
  .article-meta,
  .wire-state span,
  .wire-state small {
    color: var(--choir-text-muted);
    font-family: var(--choir-font-ui);
    font-size: 0.78rem;
    font-weight: 700;
    letter-spacing: 0;
  }

  .kicker {
    margin: 0 0 4px;
    color: var(--choir-text-accent);
    text-transform: uppercase;
  }

  h2,
  h1,
  p {
    margin: 0;
  }

  h2 {
    font-family: var(--choir-font-display);
    font-size: 3.15rem;
    line-height: 0.95;
  }

  .wire-state {
    text-align: right;
    display: grid;
    gap: 3px;
  }

  .wire-state strong {
    font-size: 1.2rem;
  }

  .wire-load-error {
    width: min(1320px, 100%);
    margin: 0 auto 18px;
    color: var(--choir-status-danger);
  }

  .wire-paper {
    display: block;
  }

  .article-columns {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    column-gap: clamp(28px, 4vw, 48px);
    row-gap: 36px;
  }

  .wire-empty-state {
    max-width: 760px;
    padding: clamp(28px, 5vw, 56px) 0;
    border-top: 1px solid var(--choir-border-subtle);
  }

  .wire-empty-state h1 {
    max-width: 680px;
    font-family: var(--choir-font-display);
    font-size: clamp(2rem, 5vw, 4rem);
    line-height: 0.98;
    margin-bottom: 16px;
  }

  .wire-empty-state p {
    max-width: 620px;
    color: var(--choir-text-secondary);
    font-size: 1rem;
    line-height: 1.55;
  }

  .empty-diagnostics {
    display: grid;
    gap: 10px;
    max-width: 760px;
    margin-top: 26px;
    padding-top: 18px;
    border-top: 1px solid var(--choir-border-subtle);
  }

  .empty-diagnostics p {
    display: grid;
    grid-template-columns: minmax(128px, 0.25fr) minmax(0, 1fr);
    gap: 14px;
    max-width: none;
    color: var(--choir-text-muted);
    font-size: 0.92rem;
  }

  .empty-diagnostics strong {
    color: var(--choir-text-primary);
    font-size: 0.78rem;
    text-transform: uppercase;
  }

  .wire-article {
    position: relative;
    margin: 0 0 clamp(28px, 4vw, 48px);
    padding: 0 0 2px;
  }

  .wire-article[data-selected='true'] .headline-button {
    color: var(--choir-text-accent);
  }

  .article-meta {
    margin-bottom: 8px;
    text-transform: uppercase;
  }

  .headline-button {
    display: block;
    width: 100%;
    margin: 0 0 12px;
    padding: 0;
    border: 0;
    background: transparent;
    text-align: left;
    cursor: pointer;
    font-family: Georgia, 'Times New Roman', ui-serif, serif;
    font-size: clamp(1.05rem, 1.45vw, 1.28rem);
    font-variant: small-caps;
    font-weight: 700;
    line-height: 1.22;
    letter-spacing: 0.03em;
    color: var(--choir-text-primary);
  }

  .headline-button:hover,
  .headline-button:focus-visible {
    color: var(--choir-text-accent);
    outline: none;
  }

  .dek,
  .projection,
  .claims p {
    font-family: Georgia, 'Times New Roman', ui-serif, serif;
    font-size: 1.05rem;
    line-height: 1.48;
    color: var(--choir-text-primary);
    text-align: justify;
    hyphens: auto;
  }

  .dek {
    color: var(--choir-text-muted);
    margin-bottom: 13px;
  }

  .source-actions {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
    margin: 0 0 12px;
  }

  .source-action {
    min-height: 30px;
    padding: 4px 10px;
    border: 1px solid var(--choir-border);
    border-radius: 6px;
    background: transparent;
    color: var(--choir-text-secondary);
    cursor: pointer;
    font: 700 0.72rem/1 var(--choir-font-ui);
    letter-spacing: 0;
    text-transform: uppercase;
  }

  .source-action:hover,
  .source-action:focus-visible {
    border-color: var(--choir-border-strong);
    color: var(--choir-text-accent);
    outline: none;
  }

  .projection {
    margin-bottom: 12px;
  }

  .claims {
    display: grid;
    gap: 8px;
  }

  .claims p {
    color: var(--choir-text-muted);
    font-size: 1rem;
  }

  :global(:root[data-theme-id='london-salmon']) .universal-wire {
    background: var(--choir-surface-document);
  }

  :global(:root[data-theme-id='london-salmon']) .headline-button,
  :global(:root[data-theme-id='london-salmon']) h2,
  :global(:root[data-theme-id='london-salmon']) .dek,
  :global(:root[data-theme-id='london-salmon']) .projection,
  :global(:root[data-theme-id='london-salmon']) .claims p {
    font-family: Georgia, 'Times New Roman', ui-serif, serif;
  }

  :global(:root[data-theme-id='carbon-fiber-kintsugi']) .headline-button {
    color: var(--choir-accent-2);
  }

  :global(:root[data-theme-id='carbon-fiber-kintsugi']) .wire-article[data-selected='true'] .headline-button {
    color: var(--choir-accent);
  }

  @media (max-width: 1100px) {
    .article-columns {
      grid-template-columns: repeat(2, minmax(0, 1fr));
      column-gap: 28px;
    }
  }

  @media (max-width: 820px) {
    .universal-wire {
      padding: 18px;
    }

    .wire-masthead {
      display: grid;
      gap: 12px;
      margin-bottom: 26px;
    }

    .wire-state {
      text-align: left;
    }

    h2 {
      font-size: 2rem;
    }

    .wire-state strong {
      font-size: 1.05rem;
    }

    .empty-diagnostics p {
      grid-template-columns: minmax(0, 1fr);
      gap: 3px;
    }

    .article-meta {
      font-size: 0.7rem;
    }

    .article-columns {
      display: grid;
      grid-template-columns: minmax(0, 1fr);
      row-gap: 30px;
    }

    .headline-button {
      font-size: 1.08rem;
      line-height: 1.18;
    }
  }
</style>
