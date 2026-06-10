<script>
  import { createEventDispatcher, onMount } from 'svelte';

  export let currentUser = null;
  export let authenticated = false;

  const dispatch = createEventDispatcher();

  let stories = [];
  let selectedStoryId = '';
  let dataSource = 'community-wire-vtext-index';
  let loadError = '';
  let lastSuccessfulLoadKey = '';
  let loadInFlight = false;
  let retryTimer = null;
  let refreshTimer = null;

  $: selectedStory = stories.find((story) => story.id === selectedStoryId) || stories[0] || null;
  $: ownerLabel = authenticated ? (currentUser?.email || 'owner computer') : 'public reader';

  onMount(() => {
    loadGlobalWireVTexts({ force: true });
    refreshTimer = setInterval(() => {
      if (authenticated) loadGlobalWireVTexts({ force: true, silent: true });
    }, 30000);
    const handleFocus = () => {
      if (authenticated) loadGlobalWireVTexts({ force: true, silent: true });
    };
    const handleVisibility = () => {
      if (!document.hidden && authenticated) loadGlobalWireVTexts({ force: true, silent: true });
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

  $: if (authenticated) loadGlobalWireVTexts();

  function scheduleAuthenticatedRetry() {
    clearTimeout(retryTimer);
    retryTimer = setTimeout(() => {
      if (authenticated) loadGlobalWireVTexts({ force: true, silent: true });
    }, 3000);
  }

  async function loadGlobalWireVTexts({ force = false, silent = false } = {}) {
    const loadKey = authenticated ? 'authenticated' : 'preview';
    if (loadInFlight) return;
    if (!force && lastSuccessfulLoadKey === loadKey) return;
    if (!authenticated) {
      loadError = '';
      stories = [];
      selectedStoryId = '';
      dataSource = 'community-wire-vtext-index';
      lastSuccessfulLoadKey = loadKey;
      return;
    }
    loadInFlight = true;
    if (!silent) loadError = '';
    try {
      const response = await fetch('/api/global-wire/stories', { credentials: 'include' });
      if (!response.ok) throw new Error(`Global Wire load failed: ${response.status}`);
      const payload = await response.json();
      if (Array.isArray(payload.stories)) {
        stories = payload.stories;
        dataSource = payload.source || 'community-wire-vtext-index';
        if (stories.length && !stories.some((story) => story.id === selectedStoryId)) selectedStoryId = stories[0].id;
        if (!stories.length) selectedStoryId = '';
      }
      clearTimeout(retryTimer);
      loadError = '';
      lastSuccessfulLoadKey = loadKey;
    } catch (error) {
      if (!silent) loadError = error?.message || 'Global Wire load failed';
      scheduleAuthenticatedRetry();
    } finally {
      loadInFlight = false;
    }
  }

  function sourceEntityId(item = {}) {
    const base = String(item.id || item.content_id || item.title || '').toLowerCase();
    const cleaned = base.replace(/[^a-z0-9_-]+/g, '-').replace(/^-+|-+$/g, '');
    return cleaned ? `gw-src-${cleaned}` : '';
  }

  function sourceRef(item = {}, fallback = 'source') {
    const label = item.title || fallback;
    const entityId = sourceEntityId(item);
    return entityId ? `[${label}](source:${entityId})` : label;
  }

  function manifestItems(story = selectedStory) {
    return [
      ...(story.manifest?.lead || []),
      ...(story.manifest?.supporting || []),
      ...(story.manifest?.contrary || []),
      ...(story.manifest?.context || []),
    ];
  }

  function storySourceEntities(story = selectedStory) {
    return manifestItems(story)
      .map((item) => {
        const entityId = sourceEntityId(item);
        if (!entityId) return null;
        return {
          entity_id: entityId,
          kind: 'content_item',
          label: item.title,
          target: {
            target_kind: 'content_item',
            content_id: item.content_id || item.id || entityId,
            canonical_url: item.canonical_url || '',
          },
          selectors: [{ selector_kind: 'whole_resource' }],
          display: {
            inline_mode: 'collapsed_citation',
            expanded_mode: 'source_card',
            open_surface: 'source',
            default_collapsed: true,
          },
          evidence: {
            state: 'available',
            research_state: 'represented',
            relation: item.role || 'context',
          },
          provenance: {
            created_by: 'global_wire',
            rights_scope: 'private_user_source',
            untrusted_source_text: true,
          },
        };
      })
      .filter(Boolean);
  }

  function storyRelatedVTexts(story = selectedStory) {
    return (story.related || [])
      .map((storyId) => {
        const related = stories.find((item) => item.id === storyId);
        if (!related) return null;
        return {
          entity_id: `gw-vtext-${storyId}`,
          label: related.headline,
          title: related.headline,
          target: {
            target_kind: 'vtext_document',
            doc_id: related.story_vtext_doc_id || storyId,
            story_id: storyId,
          },
          transclusion: {
            snapshot_text: related.dek || related.projections?.['wire-style'] || '',
            relation: 'related_story',
          },
          provenance: {
            created_by: 'global_wire',
            source: 'global_wire_related_story_index',
          },
        };
      })
      .filter(Boolean);
  }

  function storyVTextContent(story = selectedStory) {
    const lead = story.manifest?.lead?.[0];
    const secondLead = story.manifest?.lead?.[1];
    const supporting = story.manifest?.supporting?.[0];
    const qualifying = story.manifest?.contrary?.[0];
    const context = story.manifest?.context?.[0];
    const related = (story.related || [])
      .map((storyId) => {
        const relatedStory = stories.find((item) => item.id === storyId);
        if (!relatedStory) return '';
        return `[${relatedStory.headline}](vtext:${relatedStory.story_vtext_doc_id || storyId})`;
      })
      .filter(Boolean);
    return [
      `# ${story.headline}`,
      '',
      story.dek,
      '',
      lead ? `The lead signal is still the narrowest one: ${sourceRef(lead, 'lead source')} supports the update without turning it into an all-clear.${secondLead ? ` ${sourceRef(secondLead, 'operator source')} keeps the operator view attached to the story.` : ''}` : '',
      '',
      story.projections['wire-style'],
      '',
      supporting || qualifying
        ? `The source neighborhood keeps the story open rather than flattening it into a verdict.${supporting ? ` ${sourceRef(supporting, 'supporting source')} adds context for the headline improvement.` : ''}${qualifying ? ` ${sourceRef(qualifying, 'qualifying source')} remains visible as qualifying evidence.` : ''}`
        : '',
      '',
      context ? `Background remains part of the article rather than a hidden appendix. ${sourceRef(context, 'context source')} supplies the context future revisions can walk when the story updates.` : '',
      '',
      related.length
        ? `This article transcludes the related ${related.join(related.length === 2 ? ' and ' : ', ')} VTexts so reconcilers can review cross-story updates without flattening the relationship into a list.`
        : '',
      '',
      'This is a living Global Wire VText. Later processor and reconciler updates should revise this article as ordinary VText versions, with corrections treated as progress rather than as a separate product surface.',
    ].join('\n');
  }

  function launchVText({ title, content, createdFrom, sourcePath = '', docId = '', createInitialVersion = true, sourceEntities = [], relatedVTexts = [] }) {
    dispatch('launchapp', {
      appId: 'vtext',
      appName: 'VText',
      icon: '📝',
      appContext: {
        windowTitle: title,
        initialContent: content,
        docId,
        createInitialVersion,
        createdFrom,
        sourcePath,
        sourceEntities,
        relatedVTexts,
        appHint: 'global-wire',
        allowMultiple: true,
      },
    });
  }

  function openStoryVText(story = selectedStory) {
    if (!story) return;
    selectedStoryId = story.id;
    const projectionDocId = story.story_vtext_doc_id || '';
    const platformOwned = story.owner_id && story.owner_id !== currentUser?.id;
    const openDocId = platformOwned ? '' : projectionDocId;
    launchVText({
      title: story.headline,
      content: story.vtext_content || storyVTextContent(story),
      createdFrom: 'global_wire_story_projection',
      sourcePath: `global-wire/${story.id}.story.vtext`,
      docId: openDocId,
      createInitialVersion: !openDocId && !story.owner_id,
      sourceEntities: storySourceEntities(story),
      relatedVTexts: storyRelatedVTexts(story),
    });
  }
</script>

<section class="global-wire" data-global-wire-app data-global-wire-data-source={dataSource}>
  <header class="wire-masthead">
    <div>
      <p class="kicker">Living source network</p>
      <h2>Global Wire</h2>
    </div>
    <div class="wire-state" data-global-wire-state>
      <span>{ownerLabel}</span>
      <strong>{stories.length.toLocaleString()} article{stories.length === 1 ? '' : 's'}</strong>
      <small>{dataSource}</small>
    </div>
  </header>

  {#if loadError}
    <p class="wire-load-error">{loadError}</p>
  {/if}

  <main class="wire-paper">
    <section class="wire-edition" data-global-wire-front-page aria-label="Front page">
      <div class="edition-head">
        <span>Front Page</span>
        <span>awaiting edition VTexts</span>
      </div>

      {#if stories.length}
        <div class="article-columns">
          {#each stories as story}
            <article
              class="wire-article"
              data-global-wire-story
              data-selected={story.id === selectedStory?.id ? 'true' : 'false'}
              data-story-id={story.id}
              on:mouseenter={() => (selectedStoryId = story.id)}
              on:focusin={() => (selectedStoryId = story.id)}
            >
              <div class="article-tools">
                <button type="button" aria-label="Open article VText" title="Open article VText" on:click={() => openStoryVText(story)} data-global-wire-open-vtext>V</button>
              </div>
              <p class="article-meta">{story.changeState} · {story.freshness} · {story.tension}</p>
              <h1>{story.headline}</h1>
              <p class="dek">{story.dek}</p>
              <p class="projection" data-global-wire-story-reader>{story.projections?.['wire-style']}</p>
              <p class="source-line">
                {(story.manifest?.lead || []).length} lead · {(story.manifest?.supporting || []).length} supporting · {(story.manifest?.contrary || []).length} qualifying
              </p>
              <div class="claims" data-global-wire-claims>
                {#each (story.claims || []).slice(0, 2) as claim}
                  <p>{claim}</p>
                {/each}
              </div>
            </article>
          {/each}
        </div>
      {:else}
        <section class="wire-empty-state" data-global-wire-empty-state>
          <h1>No Wire edition articles yet</h1>
          <p>Community Wire will show VText-owned articles here after platform source processing and VText authoring publish an edition.</p>
        </section>
      {/if}
    </section>
  </main>
</section>

<style>
  :global(.global-wire-content) {
    overflow: auto;
  }

  .global-wire {
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
  .source-line,
  .wire-state span,
  .wire-state small,
  .edition-head {
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

  .edition-head {
    display: flex;
    justify-content: space-between;
    gap: 16px;
    margin-bottom: 20px;
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

  .wire-article {
    position: relative;
    margin: 0 0 clamp(28px, 4vw, 48px);
    padding: 0 0 2px;
  }

  .wire-article[data-selected='true'] h1 {
    color: var(--choir-text-accent);
  }

  .article-tools {
    position: absolute;
    top: 0;
    right: 0;
    display: flex;
    gap: 4px;
    opacity: 0.58;
  }

  .wire-article:hover .article-tools,
  .wire-article:focus-within .article-tools {
    opacity: 1;
  }

  .article-tools button {
    min-width: 36px;
    min-height: 32px;
    border: 0;
    border-radius: 999px;
    background: var(--choir-surface-control);
    color: var(--choir-text-primary);
    box-shadow: var(--choir-control-shadow);
    font: 700 0.82rem/1 var(--choir-font-ui);
    cursor: pointer;
  }

  .article-tools button {
    min-width: 28px;
    min-height: 28px;
    border-radius: 50%;
    background: transparent;
    box-shadow: none;
    color: var(--choir-text-muted);
  }

  .article-tools button:hover,
  .article-tools button:focus-visible {
    background: var(--choir-state-selected);
    color: var(--choir-text-accent);
    outline: none;
  }

  .article-meta {
    padding-right: 68px;
    margin-bottom: 8px;
    text-transform: uppercase;
  }

  .wire-article h1 {
    font-family: Georgia, 'Times New Roman', ui-serif, serif;
    font-size: clamp(1.55rem, 2.2vw, 2.05rem);
    line-height: 1.08;
    margin-bottom: 12px;
  }

  .dek,
  .projection,
  .claims p {
    font-family: Georgia, 'Times New Roman', ui-serif, serif;
    font-size: 1.05rem;
    line-height: 1.48;
    color: var(--choir-text-primary);
  }

  .dek {
    color: var(--choir-text-muted);
    margin-bottom: 13px;
  }

  .projection {
    margin-bottom: 12px;
  }

  .source-line {
    color: var(--choir-text-accent);
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

  :global(:root[data-theme-id='london-salmon']) .global-wire {
    background: var(--choir-surface-document);
  }

  :global(:root[data-theme-id='london-salmon']) .wire-article h1,
  :global(:root[data-theme-id='london-salmon']) h2,
  :global(:root[data-theme-id='london-salmon']) .dek,
  :global(:root[data-theme-id='london-salmon']) .projection,
  :global(:root[data-theme-id='london-salmon']) .claims p {
    font-family: Georgia, 'Times New Roman', ui-serif, serif;
  }

  :global(:root[data-theme-id='carbon-fiber-kintsugi']) .wire-article h1 {
    color: var(--choir-accent-2);
  }

  :global(:root[data-theme-id='carbon-fiber-kintsugi']) .wire-article[data-selected='true'] h1 {
    color: var(--choir-accent);
  }

  @media (max-width: 1100px) {
    .article-columns {
      grid-template-columns: repeat(2, minmax(0, 1fr));
      column-gap: 28px;
    }
  }

  @media (max-width: 820px) {
    .global-wire {
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

    .article-meta,
    .edition-head {
      font-size: 0.7rem;
    }

    .article-columns {
      display: grid;
      grid-template-columns: minmax(0, 1fr);
      row-gap: 30px;
    }

    .wire-article h1 {
      font-size: 1.45rem;
      line-height: 1.1;
    }
  }
</style>
