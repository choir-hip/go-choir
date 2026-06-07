<script>
  import { createEventDispatcher, onMount } from 'svelte';

  export let currentUser = null;
  export let authenticated = false;

  const dispatch = createEventDispatcher();

  const previewStyleSources = [
    {
      id: 'wire-style',
      title: 'Style.vtext: Global Wire',
      label: 'Wire',
      summary: 'Fast public brief, direct sourcing, visible uncertainty, no oracle voice.',
      sourcePath: 'styles/global-wire.style.vtext',
    },
    {
      id: 'claim-audit-style',
      title: 'Style.vtext: Claim Audit',
      label: 'Audit',
      summary: 'Foregrounds dispute state, evidence gaps, counterclaims, track-record signals, and uncertainty.',
      sourcePath: 'styles/claim-audit.style.vtext',
    },
    {
      id: 'market-brief-style',
      title: 'Style.vtext: Market Brief',
      label: 'Market',
      summary: 'Emphasizes exposure, second-order effects, timing, and unresolved risks.',
      sourcePath: 'styles/market-brief.style.vtext',
    },
  ];

  const previewStories = [
    {
      id: 'story-supply-resilience',
      headline: 'Port backlog recedes as carriers warn of uneven inland recovery',
      dek: 'Lead port indicators improved this week, while rail dwell and warehouse reports still show regional stress.',
      freshness: 'updated 18 min ago',
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
      freshness: 'updated 41 min ago',
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
      freshness: 'updated 1 hr ago',
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
  let styleSources = previewStyleSources;
  let selectedStoryId = stories[0].id;
  let selectedStyleId = styleSources[0].id;
  let dataSource = 'preview-source-network';
  let loadError = '';
  let lastLoadKey = '';
  let sourceSearchQuery = '';
  let sourceSearchStatus = '';
  let sourceSearchMessage = '';
  let sourceSearchResults = [];
  let sourceSearchBusy = false;
  let fetchCycleStatus = '';
  let fetchCycleBusy = false;
  let fetchCycles = [];
  let sourceRegistryEntries = [];
  let sourceSchedulerRuns = [];
  let sourceServiceStatus = null;
  let styleSourceStatus = '';
  let styleSourceBusy = false;
  let storyActionStatus = '';
  let storyActionBusy = '';

  $: selectedStory = stories.find((story) => story.id === selectedStoryId) || stories[0];
  $: selectedStyle = styleSources.find((style) => style.id === selectedStyleId) || styleSources[0];
  $: projectionText = selectedStory?.projections?.[selectedStyle?.id] || selectedStory?.projections?.['wire-style'] || '';
  $: sourceChronology = buildSourceChronology(stories, sourceSearchResults);
  $: sourceClassCount = new Set(sourceChronology.map((source) => source.role || source.source_type || 'source')).size;
  $: sourceItemCount = sourceChronology.length;
  $: liveSourceCount = sourceServiceStatus?.fetch_count || 0;
  $: liveItemCount = sourceServiceStatus?.item_count || 0;
  $: displayedSourceCount = liveSourceCount || sourceItemCount;
  $: displayedSourceLabel = liveSourceCount ? 'live sources' : 'sources';
  $: displayedSourceSummary = sourceServiceStatus?.status === 'ok'
    ? `${formatCount(liveItemCount, 'source item')} · ${formatCount(sourceServiceStatus.processor_request_count || 0, 'processor')} · ${formatCount(sourceServiceStatus.reconciler_request_count || 0, 'reconciler')}`
    : `${sourceClassCount} source groups · ${stories.length} article VTexts · ${dataSource}`;
  $: chronologyCount = liveItemCount || sourceItemCount;
  $: ownerLabel = authenticated ? (currentUser?.email || 'owner computer') : 'public preview';

  function formatCount(count, singular, plural = `${singular}s`) {
    const value = Number(count || 0);
    return `${value.toLocaleString()} ${value === 1 ? singular : plural}`;
  }

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
      styleSources = previewStyleSources;
      dataSource = 'preview-source-network';
      fetchCycles = [];
      sourceRegistryEntries = [];
      sourceSchedulerRuns = [];
      sourceServiceStatus = null;
      return;
    }
    try {
      const response = await fetch('/api/global-wire/stories', { credentials: 'include' });
      if (!response.ok) throw new Error(`Global Wire load failed: ${response.status}`);
      const payload = await response.json();
      if (Array.isArray(payload.stories) && payload.stories.length) {
        stories = payload.stories;
        styleSources = Array.isArray(payload.style_sources) && payload.style_sources.length
          ? payload.style_sources
          : payload.stories[0].style_sources || previewStyleSources;
        dataSource = (payload.source || 'durable-source-network').replaceAll('source-maxx', 'source-network').replaceAll('sourcemaxx', 'source-network');
        if (!stories.some((story) => story.id === selectedStoryId)) selectedStoryId = stories[0].id;
        if (!styleSources.some((style) => style.id === selectedStyleId)) selectedStyleId = styleSources[0].id;
        await loadFetchCycles(selectedStoryId);
        await loadSourceStatus();
      }
    } catch (error) {
      loadError = error?.message || 'Global Wire load failed';
      stories = previewStories;
      styleSources = previewStyleSources;
      dataSource = 'preview-source-network';
      sourceServiceStatus = null;
    }
  }

  async function loadSourceStatus() {
    if (!authenticated) return;
    try {
      const response = await fetch('/api/global-wire/source-status', { credentials: 'include' });
      if (!response.ok) return;
      const payload = await response.json();
      if (payload?.status) sourceServiceStatus = payload;
    } catch {
      sourceServiceStatus = null;
    }
  }

  async function loadFetchCycles(storyId = selectedStoryId) {
    if (!authenticated) return;
    try {
      const response = await fetch(`/api/global-wire/fetch-cycles?story_id=${encodeURIComponent(storyId || '')}`, {
        credentials: 'include',
      });
      if (!response.ok) return;
      const payload = await response.json();
      fetchCycles = Array.isArray(payload.recent_cycles) ? payload.recent_cycles : [];
      sourceRegistryEntries = Array.isArray(payload.registry_entries) ? payload.registry_entries : [];
      sourceSchedulerRuns = Array.isArray(payload.scheduler_runs) ? payload.scheduler_runs : [];
    } catch {
      fetchCycles = [];
      sourceRegistryEntries = [];
      sourceSchedulerRuns = [];
    }
  }

  function buildSourceChronology(sourceStories, importedItems) {
    const rows = [];
    for (const story of sourceStories || []) {
      const manifest = story.manifest || {};
      for (const tier of ['lead', 'supporting', 'contrary', 'context']) {
        for (const source of manifest[tier] || []) {
          rows.push({
            id: source.content_id || source.id,
            title: source.title,
            standing: source.standing,
            role: source.role || tier,
            tier,
            storyId: story.id,
            storyHeadline: story.headline,
            freshness: story.freshness,
          });
        }
      }
    }
    for (const item of importedItems || []) {
      rows.unshift({
        id: item.content_id || item.id,
        title: item.title,
        standing: item.metadata?.schema || item.source_type || 'source artifact',
        role: item.source_type || 'source-service',
        tier: 'live',
        storyId: selectedStoryId,
        storyHeadline: selectedStory?.headline || '',
        freshness: 'live import',
      });
    }
    const seen = new Set();
    return rows.filter((row) => {
      const key = row.id || `${row.storyId}:${row.title}`;
      if (seen.has(key)) return false;
      seen.add(key);
      return true;
    });
  }

  function sourceEntityId(item = {}) {
    const base = String(item.id || item.content_id || item.title || '').toLowerCase();
    const cleaned = base.replace(/[^a-z0-9_-]+/g, '-').replace(/^-+|-+$/g, '');
    return cleaned ? `gw-src-${cleaned}` : '';
  }

  function sourceRef(item = {}, fallback = 'source') {
    const label = item.title || fallback;
    if (!item.content_id) return label;
    const entityId = sourceEntityId(item);
    return entityId ? `[${label}](source:${entityId})` : label;
  }

  function evidenceLines(kind, items = []) {
    return items.map((item) => `- ${kind}: ${item.title} (${item.id || item.content_id || 'source handle'})`).join('\n');
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
        if (!entityId || !item.content_id) return null;
        return {
          entity_id: entityId,
          kind: 'content_item',
          label: item.title,
          target: {
            target_kind: 'content_item',
            content_id: item.content_id,
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

  function storyVTextContent(story = selectedStory, style = selectedStyle) {
    const lead = story.manifest?.lead?.[0];
    const supporting = story.manifest?.supporting?.[0];
    const qualifying = story.manifest?.contrary?.[0];
    const context = story.manifest?.context?.[0];
    const sourceSentence = [
      lead ? `lead evidence from ${sourceRef(lead, 'lead source')}` : '',
      supporting ? `supporting context from ${sourceRef(supporting, 'supporting source')}` : '',
      qualifying ? `a qualifying account from ${sourceRef(qualifying, 'qualifying source')}` : '',
      context ? `background from ${sourceRef(context, 'context source')}` : '',
    ].filter(Boolean).join(', ');
    return [
      `# ${story.headline}`,
      '',
      story.dek,
      '',
      story.projections[style.id] || story.projections['wire-style'],
      '',
      sourceSentence ? `The current version keeps ${sourceSentence} in view.` : '',
    ].join('\n');
  }

  function styleVTextContent(style = selectedStyle) {
    return [
      `# ${style.title}`,
      '',
      style.summary,
      '',
      '## Applies To',
      '',
      '- Global Wire article VTexts',
      '- Source-grounded revisions',
      '- Publication projection reviews',
      '',
      '## Guardrails',
      '',
      '- Preserve source-neighborhood evidence and contrary accounts.',
      '- Change framing and salience without inventing evidence.',
      '- Keep uncertainty and corrections visible.',
      '- Cite this Style.vtext when it materially shapes a projection.',
    ].join('\n');
  }

  function launchVText({ title, content, createdFrom, sourcePath = '', docId = '', createInitialVersion = true, sourceEntities = [] }) {
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
        appHint: 'global-wire',
        allowMultiple: true,
      },
    });
  }

  function openStoryVText(story = selectedStory, style = selectedStyle) {
    selectedStoryId = story.id;
    selectedStyleId = style.id;
    const projectionDocId = story.projection_vtext_docs?.[style.id] || story.story_vtext_doc_id || '';
    const platformOwned = story.owner_id && story.owner_id !== currentUser?.id;
    const openDocId = platformOwned ? '' : projectionDocId;
    launchVText({
      title: story.headline,
      content: story.vtext_content || storyVTextContent(story, style),
      createdFrom: 'global_wire_story_projection',
      sourcePath: `global-wire/${story.id}.story.vtext`,
      docId: openDocId,
      createInitialVersion: !openDocId && !story.owner_id,
      sourceEntities: storySourceEntities(story),
    });
  }

  function openStyleVText(style = selectedStyle) {
    selectedStyleId = style.id;
    launchVText({
      title: style.title,
      content: styleVTextContent(style),
      createdFrom: 'global_wire_style_source',
      sourcePath: style.sourcePath,
      docId: style.doc_id || '',
      createInitialVersion: !style.doc_id,
    });
  }

  function storyPromptContext() {
    return [
      'Answer as Choir about this Global Wire article. Use only the evidence below, explain what changed, what remains uncertain, and what evidence should be checked next.',
      '',
      `Story id: ${selectedStory.id}`,
      `Headline: ${selectedStory.headline}`,
      `State: ${selectedStory.changeState}; ${selectedStory.tension}`,
      `Style.vtext source: ${selectedStyle.title}`,
      '',
      'Projection:',
      projectionText,
      '',
      'Claims:',
      ...selectedStory.claims.map((claim) => `- ${claim}`),
      '',
      'Source evidence handles:',
      evidenceLines('lead', selectedStory.manifest.lead),
      evidenceLines('supporting', selectedStory.manifest.supporting),
      evidenceLines('contrary or qualifying', selectedStory.manifest.contrary),
      evidenceLines('ambient context', selectedStory.manifest.context),
      '',
      'Guardrail: do not mutate the platform VText, do not invent facts, and make provenance visible.',
    ].join('\n');
  }

  async function submitStoryAction() {
    if (!authenticated) {
      storyActionStatus = 'Sign in to ask Choir from this article.';
      return;
    }
    storyActionBusy = 'ask';
    storyActionStatus = '';
    try {
      const response = await fetch('/api/prompt-bar', {
        method: 'POST',
        credentials: 'include',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ text: storyPromptContext() }),
      });
      const payload = await response.json();
      if (!response.ok || !payload.submission_id) {
        throw new Error(`Prompt submission failed: ${response.status}`);
      }
      storyActionStatus = `Ask submitted: ${payload.submission_id.slice(0, 8)}`;
    } catch (error) {
      storyActionStatus = error?.message || 'Prompt submission failed';
    } finally {
      storyActionBusy = '';
    }
  }

  async function searchSources() {
    const query = sourceSearchQuery.trim();
    if (!authenticated) {
      sourceSearchStatus = 'sign-in-required';
      sourceSearchMessage = 'Sign in to search source evidence.';
      sourceSearchResults = [];
      return;
    }
    if (!query) {
      sourceSearchStatus = 'query-required';
      sourceSearchMessage = 'Enter a source query.';
      sourceSearchResults = [];
      return;
    }
    sourceSearchBusy = true;
    sourceSearchStatus = '';
    sourceSearchMessage = '';
    try {
      const response = await fetch('/api/global-wire/source-search', {
        method: 'POST',
        credentials: 'include',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          query,
          max_results: 12,
          story_id: selectedStory.id,
          queue_top_result: false,
        }),
      });
      const payload = await response.json();
      sourceSearchStatus = payload.status || (response.ok ? 'ok' : 'unavailable');
      sourceSearchMessage = payload.message || `${payload.source || 'source-service'}: ${sourceSearchStatus}`;
      sourceSearchResults = Array.isArray(payload.content_items) ? payload.content_items : [];
    } catch (error) {
      sourceSearchStatus = 'unavailable';
      sourceSearchMessage = error?.message || 'Source search failed';
      sourceSearchResults = [];
    } finally {
      sourceSearchBusy = false;
    }
  }

  async function runFetchCycle(schedulerMode = false) {
    if (!authenticated) {
      fetchCycleStatus = 'Sign in to run a bounded source fetch cycle.';
      return;
    }
    fetchCycleBusy = true;
    fetchCycleStatus = '';
    try {
      const response = await fetch('/api/global-wire/fetch-cycles', {
        method: 'POST',
        credentials: 'include',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          story_ids: stories.slice(0, 3).map((story) => story.id),
          max_stories: 3,
          max_results: 8,
          trigger: schedulerMode ? 'global-wire-source-network-scheduled-cycle' : 'global-wire-source-network-manual-cycle',
          scheduler_mode: schedulerMode,
          cadence_seconds: schedulerMode ? 900 : undefined,
        }),
      });
      const payload = await response.json();
      fetchCycleStatus = payload.message || payload.status || `Fetch cycle ${response.status}`;
      if (payload.fetch_cycle?.id) fetchCycles = [payload.fetch_cycle, ...fetchCycles].slice(0, 20);
      if (payload.scheduler_run?.id) sourceSchedulerRuns = [payload.scheduler_run, ...sourceSchedulerRuns].slice(0, 20);
      if (Array.isArray(payload.registry_entries)) sourceRegistryEntries = [...payload.registry_entries, ...sourceRegistryEntries].slice(0, 30);
      if (Array.isArray(payload.content_items)) sourceSearchResults = [...payload.content_items, ...sourceSearchResults].slice(0, 30);
      await loadFetchCycles(selectedStory.id);
      await loadSourceStatus();
    } catch (error) {
      fetchCycleStatus = error?.message || 'Fetch cycle failed';
    } finally {
      fetchCycleBusy = false;
    }
  }

  async function updateStyleSource(action) {
    if (!authenticated || !selectedStory?.id || styleSourceBusy) return;
    styleSourceBusy = true;
    styleSourceStatus = '';
    const baseStyleIds = action === 'compose'
      ? styleSources.slice(0, 2).map((style) => style.id)
      : [selectedStyle.id];
    try {
      const response = await fetch('/api/global-wire/style-sources', {
        method: 'POST',
        credentials: 'include',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          story_id: selectedStory.id,
          action,
          base_style_ids: baseStyleIds,
          replace_style_id: action === 'replace' ? selectedStyle.id : '',
          title: action === 'compose'
            ? 'Style.vtext: Wire + Audit Hybrid'
            : `Style.vtext: Replacement ${selectedStyle.label || selectedStyle.title}`,
          label: action === 'compose' ? 'Hybrid' : 'Replace',
          summary: action === 'compose'
            ? 'Hybrid composed style preserving wire speed while foregrounding claim audit uncertainty.'
            : `Replacement style source derived from ${selectedStyle.title} with explicit provenance.`,
        }),
      });
      if (!response.ok) throw new Error(`Style ${action} failed: ${response.status}`);
      const payload = await response.json();
      if (payload.story?.id) stories = stories.map((story) => (story.id === payload.story.id ? payload.story : story));
      if (payload.style?.id) {
        styleSources = payload.story?.style_sources || [...styleSources, payload.style];
        selectedStyleId = payload.style.id;
      }
      styleSourceStatus = action === 'compose' ? 'Composed Style.vtext source created' : 'Replacement Style.vtext source created';
      lastLoadKey = '';
      await loadGlobalWireVTexts();
    } catch (error) {
      styleSourceStatus = error?.message || `Style ${action} failed`;
    } finally {
      styleSourceBusy = false;
    }
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
      <strong>{displayedSourceCount.toLocaleString()} {displayedSourceLabel}</strong>
      <small>{displayedSourceSummary}</small>
    </div>
  </header>

  {#if loadError}
    <p class="wire-load-error">{loadError}</p>
  {/if}

  <main class="wire-paper">
    <section class="wire-edition" data-global-wire-front-page aria-label="Front page">
      <div class="edition-head">
        <span>Front Page</span>
        <span>{fetchCycles[0]?.status || 'source ledger ready'}</span>
      </div>

      <div class="article-columns">
        {#each stories as story}
          {@const style = styleSources.find((item) => item.id === selectedStyleId) || story.style_sources?.[0] || selectedStyle}
          <article
            class="wire-article"
            data-global-wire-story
            data-selected={story.id === selectedStory.id ? 'true' : 'false'}
            data-story-id={story.id}
            on:mouseenter={() => (selectedStoryId = story.id)}
            on:focusin={() => (selectedStoryId = story.id)}
          >
            <div class="article-tools">
              <button type="button" aria-label="Open article VText" title="Open article VText" on:click={() => openStoryVText(story, style)} data-global-wire-open-vtext>V</button>
            </div>
            <p class="article-meta">{story.changeState} · {story.freshness} · {story.tension}</p>
            <h1>{story.headline}</h1>
            <p class="dek">{story.dek}</p>
            <p class="projection" data-global-wire-story-reader>{story.projections?.[style.id] || story.projections?.['wire-style']}</p>
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

    <aside class="source-column" data-global-wire-evidence aria-label="Source chronology">
      <div class="source-head">
        <div>
          <p class="kicker">Sources</p>
          <h3>Chronology</h3>
        </div>
        <span>{chronologyCount.toLocaleString()}</span>
      </div>

      <form class="source-search" data-global-wire-source-search on:submit|preventDefault={searchSources}>
        <input
          bind:value={sourceSearchQuery}
          type="search"
          placeholder="Search source ledger"
          data-global-wire-source-search-input
        />
        <button type="submit" disabled={sourceSearchBusy} data-global-wire-source-search-submit>
          {sourceSearchBusy ? '...' : 'Search'}
        </button>
      </form>
      {#if sourceSearchStatus}
        <p class="source-status" data-global-wire-source-search-status>{sourceSearchStatus}: {sourceSearchMessage}</p>
      {/if}

      <div class="source-actions">
        <button type="button" on:click={() => runFetchCycle(false)} disabled={fetchCycleBusy} data-global-wire-fetch-cycle>
          {fetchCycleBusy ? 'Running' : 'Fetch'}
        </button>
        <button type="button" on:click={() => runFetchCycle(true)} disabled={fetchCycleBusy} data-global-wire-scheduler-cycle>
          Schedule
        </button>
      </div>
      {#if fetchCycleStatus}
        <p class="source-status" data-global-wire-fetch-cycle-status>{fetchCycleStatus}</p>
      {/if}
      {#if sourceServiceStatus?.cycle_id}
        <p class="source-status" data-global-wire-source-status>
          {sourceServiceStatus.cycle_status || sourceServiceStatus.status} · {sourceServiceStatus.cycle_id}
        </p>
      {/if}

      {#if fetchCycles.length || sourceRegistryEntries.length || sourceSchedulerRuns.length}
        <div class="source-run-ledger" data-global-wire-fetch-cycle-runs>
          {#each sourceSchedulerRuns.slice(0, 2) as run}
            <p data-global-wire-source-scheduler-run>{run.status} · {run.trigger} · {run.message}</p>
          {/each}
          {#each fetchCycles.slice(0, 2) as cycle}
            <p>{cycle.status} · {(cycle.source_content_ids || []).length} source refs · {cycle.message}</p>
          {/each}
          {#each sourceRegistryEntries.slice(0, 3) as entry}
            <p data-global-wire-source-registry-entry>
              {entry.source_scope} · {entry.status} · {entry.query}
              {#if entry.cadence_seconds}
                <span data-global-wire-source-schedule-cadence>cadence {entry.cadence_seconds}s</span>
              {/if}
            </p>
          {/each}
        </div>
      {/if}

      <div class="source-list">
        {#each sourceChronology.slice(0, 24) as source}
          <button
            type="button"
            data-source-tier={source.tier}
            on:click={() => (selectedStoryId = source.storyId)}
          >
            <span>{source.tier}</span>
            <strong>{source.title}</strong>
            <small>{source.standing} · {source.storyHeadline}</small>
          </button>
        {/each}
      </div>

      {#if sourceSearchResults.length}
        <div class="source-search-results" data-global-wire-source-search-results>
          {#each sourceSearchResults.slice(0, 6) as result}
            <article>
              <strong>{result.title}</strong>
              <small>{result.source_type} · {result.metadata?.schema || 'source artifact'}</small>
            </article>
          {/each}
        </div>
      {/if}
    </aside>
  </main>

  <footer class="wire-disclosure">
    <section data-global-wire-style-switcher>
      <span>Style.vtext</span>
      {#each styleSources as style}
        <button
          type="button"
          class:active={style.id === selectedStyle.id}
          aria-label={`Use ${style.title}`}
          title={style.title}
          on:click={() => (selectedStyleId = style.id)}
        >
          {style.label}
        </button>
      {/each}
      <button type="button" aria-label="Open Style.vtext" title="Open Style.vtext" on:click={() => openStyleVText(selectedStyle)} data-global-wire-open-style>S</button>
      {#if authenticated}
        <button type="button" on:click={() => updateStyleSource('compose')} disabled={styleSourceBusy} data-global-wire-compose-style>Compose</button>
        <button type="button" on:click={() => updateStyleSource('replace')} disabled={styleSourceBusy} data-global-wire-replace-style>Replace</button>
      {/if}
      <small>Cites {selectedStyle.title}; source provenance stays with the VText version.</small>
      {#if styleSourceStatus}
        <small data-global-wire-style-source-status>{styleSourceStatus}</small>
      {/if}
    </section>
    <section>
      <button type="button" on:click={submitStoryAction} disabled={storyActionBusy !== ''} data-global-wire-ask-choir>Ask</button>
      {#if storyActionStatus}
        <small data-global-wire-story-action-status>{storyActionStatus}</small>
      {/if}
    </section>
  </footer>
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
  .wire-paper,
  .wire-disclosure {
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
  .source-list span,
  .wire-state span,
  .wire-state small,
  .edition-head,
  .wire-disclosure span,
  .wire-disclosure small,
  .source-status,
  .source-run-ledger,
  .source-search-results small {
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
  h3,
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
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(240px, 320px);
    gap: clamp(28px, 4vw, 56px);
    align-items: start;
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

  .article-tools button,
  .wire-disclosure button,
  .source-actions button,
  .source-search button {
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
  .article-tools button:focus-visible,
  .wire-disclosure button:hover,
  .wire-disclosure button:focus-visible,
  .source-actions button:hover,
  .source-actions button:focus-visible,
  .source-search button:hover,
  .source-search button:focus-visible,
  .wire-disclosure button.active {
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

  .source-column {
    position: sticky;
    top: 0;
  }

  .source-head {
    display: flex;
    justify-content: space-between;
    align-items: end;
    gap: 12px;
    margin-bottom: 18px;
  }

  .source-head h3 {
    font-size: 1.4rem;
  }

  .source-head > span {
    color: var(--choir-text-accent);
    font-weight: 800;
    font-size: 1.3rem;
  }

  .source-search {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    gap: 8px;
    margin-bottom: 10px;
  }

  .source-search input {
    min-width: 0;
    border: 0;
    border-radius: 999px;
    background: var(--choir-surface-input);
    color: var(--choir-text-primary);
    padding: 10px 14px;
    font: 700 0.9rem/1.2 var(--choir-font-ui);
  }

  .source-actions {
    display: flex;
    gap: 8px;
    margin-bottom: 14px;
  }

  .source-status,
  .source-run-ledger {
    margin: 0 0 12px;
  }

  .source-run-ledger {
    display: grid;
    gap: 7px;
    color: var(--choir-text-subtle);
  }

  .source-run-ledger span {
    display: block;
    color: var(--choir-text-accent);
  }

  .source-list {
    display: grid;
    gap: 14px;
  }

  .source-list button {
    display: grid;
    gap: 3px;
    padding: 0;
    text-align: left;
    border: 0;
    background: transparent !important;
    box-shadow: none !important;
    color: inherit;
    cursor: pointer;
  }

  .source-list button:hover strong,
  .source-list button:focus-visible strong {
    color: var(--choir-text-accent);
  }

  .source-list strong,
  .source-search-results strong {
    font-size: 0.98rem;
    line-height: 1.2;
  }

  .source-list small,
  .source-search-results small {
    color: var(--choir-text-muted);
    line-height: 1.25;
  }

  .source-search-results {
    display: grid;
    gap: 12px;
    margin-top: 18px;
  }

  .source-search-results article {
    display: grid;
    gap: 4px;
  }

  .wire-disclosure {
    display: flex;
    justify-content: space-between;
    gap: 18px;
    margin-top: clamp(22px, 4vw, 44px);
    padding-top: 16px;
  }

  .wire-disclosure section {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 8px;
  }

  .wire-disclosure small {
    max-width: 520px;
  }

  :global(:root[data-theme-id='london-salmon']) .global-wire {
    background: var(--choir-surface-document);
  }

  :global(:root[data-theme-id='london-salmon']) .wire-article h1,
  :global(:root[data-theme-id='london-salmon']) h2,
  :global(:root[data-theme-id='london-salmon']) h3,
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
    .wire-paper {
      grid-template-columns: minmax(0, 1fr) minmax(220px, 280px);
      gap: 28px;
    }

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

    .wire-paper {
      display: block;
    }

    .article-columns {
      display: grid;
      grid-template-columns: minmax(0, 1fr);
      row-gap: 30px;
    }

    .source-column {
      position: static;
      margin-top: 28px;
    }

    .wire-article h1 {
      font-size: 1.45rem;
      line-height: 1.1;
    }

    .wire-disclosure {
      display: grid;
    }
  }
</style>
