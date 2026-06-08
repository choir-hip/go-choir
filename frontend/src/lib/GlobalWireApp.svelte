<script>
  import { createEventDispatcher, onMount } from 'svelte';

  export let currentUser = null;
  export let authenticated = false;

  const dispatch = createEventDispatcher();

  const previewStories = [
    {
      id: 'story-supply-resilience',
      headline: 'Port backlog recedes as carriers warn of uneven inland recovery',
      dek: 'Lead port indicators improved this week, while rail dwell and warehouse reports still show regional stress.',
      freshness: 'seed source neighborhood',
      prominence: 82,
      tension: 'qualifying evidence',
      changeState: 'claim narrowed',
      nodeTone: 'live',
      related: ['story-energy-grid', 'story-retail-margins'],
      manifest: {
        lead: [
          { id: 'source-port-authority', title: 'Port authority throughput bulletin', standing: 'official operations bulletin', role: 'lead' },
          { id: 'source-carrier-note', title: 'Carrier service advisory', standing: 'operator disclosure', role: 'lead' },
        ],
        supporting: [
          { id: 'source-rail-dwell', title: 'Rail dwell dashboard', standing: 'public logistics metric', role: 'supporting' },
          { id: 'source-warehouse-index', title: 'Warehouse vacancy index', standing: 'industry data', role: 'supporting' },
        ],
        contrary: [
          { id: 'source-regional-exporters', title: 'Regional exporters report delays', standing: 'trade association survey', role: 'contrary' },
        ],
        context: [
          { id: 'source-ambient-brief', title: 'Ambient corpus: shipping and retail filings', standing: 'bounded context packet', role: 'context' },
        ],
      },
      claims: [
        'Container queue times have improved at the port complex.',
        'Inland recovery remains uneven and should not be summarized as resolved.',
        'Retail margin impact depends on regional warehouse exposure.',
      ],
      projections: {
        'wire-style': 'Port congestion indicators eased this week, but the recovery remains uneven once inland rail dwell and warehouse data are included. The current platform story treats the port bulletin as lead evidence and keeps the exporter delay survey visible as qualifying evidence.',
        'claim-audit-style': 'The strongest supported claim is narrower than the headline risk suggests: vessel queues have improved. A broader claim that supply chains are normal again is not supported because rail dwell, warehouse vacancy, and exporter surveys still show regional delays.',
        'market-brief-style': 'The market read is mixed. Port improvement lowers near-term shipping pressure, but inland bottlenecks leave margin risk concentrated in retailers with regionally exposed inventories and limited warehouse flexibility.',
      },
    },
    {
      id: 'story-energy-grid',
      headline: 'Grid operators add reserve alerts as heat forecast shifts north',
      dek: 'Forecast changes moved stress from the southern peak window toward northern reserve margins.',
      freshness: 'seed source neighborhood',
      prominence: 74,
      tension: 'forecast changed',
      changeState: 'timeline updated',
      nodeTone: 'changed',
      related: ['story-supply-resilience', 'story-city-air'],
      manifest: {
        lead: [
          { id: 'source-grid-notice', title: 'Regional grid operator reserve notice', standing: 'official grid notice', role: 'lead' },
          { id: 'source-weather-update', title: 'National forecast update', standing: 'meteorological update', role: 'lead' },
        ],
        supporting: [
          { id: 'source-demand-model', title: 'Demand forecast model', standing: 'operator model packet', role: 'supporting' },
        ],
        contrary: [
          { id: 'source-utility-comment', title: 'Utility says local capacity is adequate', standing: 'utility statement', role: 'contrary' },
        ],
        context: [
          { id: 'source-grid-history', title: 'Prior reserve-alert history', standing: 'timeline context', role: 'context' },
        ],
      },
      claims: [
        'Reserve concern shifted north with the updated heat forecast.',
        'The alert is operational risk, not proof of shortage.',
        'Local utility statements should be read against regional reserve margins.',
      ],
      projections: {
        'wire-style': 'Grid operators issued reserve alerts after the heat forecast moved north. The story is not a shortage call; it is an operational watch with utility statements and prior alert history kept in the evidence neighborhood.',
        'claim-audit-style': 'The alert supports a risk claim, not a failure claim. The contrary utility statement does not negate the regional notice, but it narrows the geography and should stay attached to the story.',
        'market-brief-style': 'The exposure is timing-sensitive: reserve alerts can move power prices before any outage occurs. The practical signal is regional load stress and hedging pressure rather than confirmed infrastructure failure.',
      },
    },
    {
      id: 'story-city-air',
      headline: 'City air monitors show sharp overnight improvement after smoke plume disperses',
      dek: 'Monitors improved by morning, but health agencies kept cautions for sensitive groups while plume models update.',
      freshness: 'seed source neighborhood',
      prominence: 63,
      tension: 'public guidance lag',
      changeState: 'status improved',
      nodeTone: 'cooling',
      related: ['story-energy-grid'],
      manifest: {
        lead: [
          { id: 'source-air-monitors', title: 'City air-quality monitor readings', standing: 'public sensor network', role: 'lead' },
          { id: 'source-health-agency', title: 'Health agency advisory', standing: 'public health guidance', role: 'lead' },
        ],
        supporting: [
          { id: 'source-plume-model', title: 'Smoke plume model update', standing: 'forecast model', role: 'supporting' },
        ],
        contrary: [
          { id: 'source-community-reports', title: 'Community reports of local haze', standing: 'local observations', role: 'contrary' },
        ],
        context: [
          { id: 'source-prior-air-event', title: 'Prior air-quality event timeline', standing: 'historical context', role: 'context' },
        ],
      },
      claims: [
        'Sensor readings improved materially overnight.',
        'Sensitive-group caution remains because public-health guidance lags and local haze reports persist.',
        'The story should track monitor changes over time instead of freezing the morning state.',
      ],
      projections: {
        'wire-style': 'Air-quality readings improved sharply after the smoke plume dispersed overnight. Health guidance remains more cautious for sensitive groups, so the story keeps monitor data, plume models, and local reports in view.',
        'claim-audit-style': 'The evidence supports improvement, not all-clear. The health advisory and community haze reports qualify the monitor trend and prevent the platform story from flattening a changing condition into a single verdict.',
        'market-brief-style': 'The operational effect is localized but real: school, transit, and outdoor-work decisions may lag sensor improvement because public guidance and local observations update on different cadences.',
      },
    },
  ];

  let stories = previewStories;
  let selectedStoryId = stories[0].id;
  let dataSource = 'preview-source-network';
  let loadError = '';
  let lastLoadKey = '';

  $: selectedStory = stories.find((story) => story.id === selectedStoryId) || stories[0];
  $: ownerLabel = authenticated ? (currentUser?.email || 'owner computer') : 'public preview';

  onMount(() => {
    loadGlobalWireVTexts();
  });

  $: if (authenticated) {
    loadGlobalWireVTexts();
  }

  async function loadGlobalWireVTexts() {
    const loadKey = authenticated ? 'authenticated' : 'preview';
    if (lastLoadKey === loadKey) return;
    lastLoadKey = loadKey;
    loadError = '';
    if (!authenticated) {
      stories = previewStories;
      dataSource = 'preview-source-network';
      return;
    }
    try {
      const response = await fetch('/api/global-wire/stories', { credentials: 'include' });
      if (!response.ok) throw new Error(`Global Wire load failed: ${response.status}`);
      const payload = await response.json();
      if (Array.isArray(payload.stories) && payload.stories.length) {
        stories = payload.stories;
        dataSource = (payload.source || 'durable-source-network').replaceAll('source-maxx', 'source-network').replaceAll('sourcemaxx', 'source-network');
        if (!stories.some((story) => story.id === selectedStoryId)) selectedStoryId = stories[0].id;
      }
    } catch (error) {
      loadError = error?.message || 'Global Wire load failed';
      stories = previewStories;
      dataSource = 'preview-source-network';
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
        <span>source network ready</span>
      </div>

      <div class="article-columns">
        {#each stories as story}
          <article
            class="wire-article"
            data-global-wire-story
            data-selected={story.id === selectedStory.id ? 'true' : 'false'}
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
