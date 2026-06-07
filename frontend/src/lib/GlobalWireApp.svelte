<script>
  import { createEventDispatcher } from 'svelte';
  import { onMount } from 'svelte';

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
      summary: 'Foregrounds dispute state, evidence gaps, counterclaims, and source standing.',
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
        'claim-audit-style': 'The alert supports a risk claim, not a failure claim. The contrary utility statement does not negate the regional notice, but it narrows the geography and should stay attached to the StoryGraph.',
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
  let contributionKind = 'source';
  let contributionText = '';
  let contributionStatus = '';
  let sourceSearchQuery = '';
  let sourceSearchStatus = '';
  let sourceSearchMessage = '';
  let sourceSearchResults = [];
  let sourceSearchBusy = false;
  let sourceRefreshStatus = '';
  let sourceRefreshBusy = false;
  let fetchCycleStatus = '';
  let fetchCycleBusy = false;
  let fetchCycles = [];
  let sourceRegistryEntries = [];
  let styleSourceStatus = '';
  let styleSourceBusy = false;
  let queueTopSourceResult = true;
  let storyActionStatus = '';
  let storyActionBusy = '';
  let contributions = [];
  let reconciliationSourceItems = {};
  let reconciliationDecisions = [];
  let graphUpdateCandidates = [];
  let graphPromotionDecisions = [];
  let sourceRefreshes = [];
  let claimRecords = [];
  let researchTasks = [];
  let projectionReviews = [];
  let reconciliationBusyId = '';
  let dataSource = 'preview-storygraph';
  let loadError = '';
  let lastLoadKey = '';

  $: selectedStory = stories.find((story) => story.id === selectedStoryId) || stories[0];
  $: selectedStyle = styleSources.find((style) => style.id === selectedStyleId) || styleSources[0];
  $: projectionText = selectedStory.projections[selectedStyle.id] || selectedStory.projections['wire-style'];
  $: allSources = [
    ...selectedStory.manifest.lead,
    ...selectedStory.manifest.supporting,
    ...selectedStory.manifest.contrary,
    ...selectedStory.manifest.context,
  ];

  onMount(() => {
    loadDurableStoryGraph();
  });

  $: if (authenticated) {
    loadDurableStoryGraph();
  }

  async function loadDurableStoryGraph() {
    const loadKey = authenticated ? 'authenticated' : 'preview';
    if (lastLoadKey === loadKey) return;
    lastLoadKey = loadKey;
    loadError = '';
    if (!authenticated) {
      stories = previewStories;
      styleSources = previewStyleSources;
      dataSource = 'preview-storygraph';
      contributions = [];
      reconciliationSourceItems = {};
      reconciliationDecisions = [];
      graphUpdateCandidates = [];
      graphPromotionDecisions = [];
      sourceRefreshes = [];
      fetchCycles = [];
      sourceRegistryEntries = [];
      claimRecords = [];
      researchTasks = [];
      projectionReviews = [];
      sourceSearchResults = [];
      sourceSearchStatus = '';
      sourceSearchMessage = '';
      sourceRefreshStatus = '';
      fetchCycleStatus = '';
      styleSourceStatus = '';
      return;
    }
    try {
      const response = await fetch('/api/global-wire/stories', { credentials: 'include' });
      if (!response.ok) throw new Error(`StoryGraph load failed: ${response.status}`);
      const payload = await response.json();
      if (Array.isArray(payload.stories) && payload.stories.length) {
        stories = payload.stories;
        styleSources = Array.isArray(payload.style_sources) && payload.style_sources.length
          ? payload.style_sources
          : payload.stories[0].style_sources || previewStyleSources;
        dataSource = payload.source || 'durable-storygraph';
        if (!stories.some((story) => story.id === selectedStoryId)) selectedStoryId = stories[0].id;
        if (!styleSources.some((style) => style.id === selectedStyleId)) selectedStyleId = styleSources[0].id;
        await loadContributions();
      }
    } catch (error) {
      loadError = error?.message || 'StoryGraph load failed';
      stories = previewStories;
      styleSources = previewStyleSources;
      dataSource = 'preview-storygraph';
    }
  }

  async function loadContributions(storyId = selectedStoryId) {
    if (!authenticated || !storyId) return;
    try {
      const response = await fetch(`/api/global-wire/reconciliation?story_id=${encodeURIComponent(storyId)}`, {
        credentials: 'include',
      });
      if (!response.ok) throw new Error(`Reconciliation load failed: ${response.status}`);
      const payload = await response.json();
      contributions = Array.isArray(payload.contributions) ? payload.contributions.slice(0, 6) : [];
      reconciliationSourceItems = payload.source_items || {};
      reconciliationDecisions = Array.isArray(payload.decisions) ? payload.decisions : [];
      graphUpdateCandidates = Array.isArray(payload.candidates) ? payload.candidates : [];
      graphPromotionDecisions = Array.isArray(payload.promotions) ? payload.promotions : [];
      sourceRefreshes = Array.isArray(payload.refreshes) ? payload.refreshes : [];
      claimRecords = Array.isArray(payload.claim_records) ? payload.claim_records : [];
      researchTasks = Array.isArray(payload.research_tasks) ? payload.research_tasks : [];
      projectionReviews = Array.isArray(payload.projection_reviews) ? payload.projection_reviews : [];
      await loadFetchCycles(storyId);
    } catch {
      contributions = [];
      reconciliationSourceItems = {};
      reconciliationDecisions = [];
      graphUpdateCandidates = [];
      graphPromotionDecisions = [];
      sourceRefreshes = [];
      fetchCycles = [];
      sourceRegistryEntries = [];
      claimRecords = [];
      researchTasks = [];
      projectionReviews = [];
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
    } catch {
      fetchCycles = [];
      sourceRegistryEntries = [];
    }
  }

  $: if (authenticated && dataSource === 'durable-storygraph' && selectedStoryId) {
    loadContributions(selectedStoryId);
  }

  function sourceLines(kind, items) {
    return items.map((item) => `- ${kind}: ${item.title} (${item.standing}; ${item.id})`).join('\n');
  }

  function storyVTextContent(story = selectedStory, style = selectedStyle) {
    return [
      `# ${story.headline}`,
      '',
      story.dek,
      '',
      `Style source: ${style.title}`,
      `StoryGraph id: ${story.id}`,
      `State: ${story.changeState}; ${story.tension}`,
      '',
      '## Projection',
      '',
      story.projections[style.id] || story.projections['wire-style'],
      '',
      '## Claims',
      '',
      ...story.claims.map((claim) => `- ${claim}`),
      '',
      '## Source Manifest',
      '',
      sourceLines('lead', story.manifest.lead),
      sourceLines('supporting', story.manifest.supporting),
      sourceLines('contrary or qualifying', story.manifest.contrary),
      sourceLines('ambient context', story.manifest.context),
      '',
      '## Related Story VTexts',
      '',
      ...story.related.map((id) => {
        const related = stories.find((item) => item.id === id);
        return `- ${related?.headline || id} (${id})`;
      }),
      '',
      '## Non-oracle note',
      '',
      'This story is a source-grounded VText projection. User edits create user-owned versions and do not mutate the platform story.',
    ].join('\n');
  }

  function storyPromptContext(mode) {
    const sourceManifest = [
      sourceLines('lead', selectedStory.manifest.lead),
      sourceLines('supporting', selectedStory.manifest.supporting),
      sourceLines('contrary or qualifying', selectedStory.manifest.contrary),
      sourceLines('ambient context', selectedStory.manifest.context),
    ].filter(Boolean).join('\n');
    const related = selectedStory.related.map((id) => {
      const story = stories.find((item) => item.id === id);
      return `- ${story?.headline || id} (${id})`;
    }).join('\n');
    const task = mode === 'autoradio'
      ? 'Create an Autoradio-ready spoken brief for this story projection. Use only the evidence below, keep uncertainty audible, cite source tiers, and name evidence gaps instead of filling them.'
      : 'Answer as Choir about this Global Wire story. Use only the evidence below, explain what changed, what remains uncertain, and what evidence should be checked next.';
    return [
      task,
      '',
      `StoryGraph id: ${selectedStory.id}`,
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
      'Source Manifest:',
      sourceManifest,
      '',
      'Related Story VTexts:',
      related,
      '',
      'Guardrail: do not mutate the platform StoryGraph, do not invent facts, and make provenance visible.',
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
      '- StoryGraph projections',
      '- Story VText revision prompts',
      '- News reader and Autoradio traversal',
      '',
      '## Guardrails',
      '',
      '- Preserve lead, supporting, contrary, and ambient source tiers.',
      '- Change framing and salience without inventing evidence.',
      '- Keep uncertainty and corrections visible.',
      '- Cite this Style.vtext when it materially shapes a projection.',
    ].join('\n');
  }

  function contributionContent() {
    const kind = contributionKind.replaceAll('-', ' ');
    return [
      `# Contribution: ${selectedStory.headline}`,
      '',
      `StoryGraph id: ${selectedStory.id}`,
      `Contribution kind: ${kind}`,
      `Owner: ${currentUser?.email || 'public preview user'}`,
      '',
      '## User Contribution',
      '',
      contributionText.trim() || 'Draft contribution awaiting detail.',
      '',
      '## Research/Reconciliation State',
      '',
      '- user-owned artifact created from the News app contribution surface',
      '- pending researcher review before any platform story reconciliation',
      '- does not mutate the platform Story VText',
    ].join('\n');
  }

  function launchVText({ title, content, createdFrom, sourcePath = '', docId = '', createInitialVersion = true }) {
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
        appHint: 'global-wire',
        allowMultiple: true,
      },
    });
  }

  function openStoryVText() {
    const projectionDocId = selectedStory.projection_vtext_docs?.[selectedStyle.id] || selectedStory.story_vtext_doc_id || '';
    launchVText({
      title: selectedStory.headline,
      content: storyVTextContent(),
      createdFrom: 'global_wire_story_projection',
      sourcePath: `global-wire/${selectedStory.id}.story.vtext`,
      docId: projectionDocId,
      createInitialVersion: !projectionDocId,
    });
  }

  function forkStory() {
    launchVText({
      title: `My version of ${selectedStory.headline}`,
      content: `${storyVTextContent()}\n\n## My Edit\n\n`,
      createdFrom: 'global_wire_user_story_fork',
      sourcePath: `user-forks/${selectedStory.id}.story.vtext`,
    });
  }

  function openStyleVText() {
    launchVText({
      title: selectedStyle.title,
      content: styleVTextContent(),
      createdFrom: 'global_wire_style_source',
      sourcePath: selectedStyle.sourcePath,
      docId: selectedStyle.doc_id || '',
      createInitialVersion: !selectedStyle.doc_id,
    });
  }

  async function submitStoryAction(mode) {
    if (!authenticated) {
      storyActionStatus = 'Sign in to ask Choir from this StoryGraph.';
      return;
    }
    storyActionBusy = mode;
    storyActionStatus = '';
    try {
      const response = await fetch('/api/prompt-bar', {
        method: 'POST',
        credentials: 'include',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ text: storyPromptContext(mode) }),
      });
      const payload = await response.json();
      if (!response.ok || !payload.submission_id) {
        throw new Error(`Prompt submission failed: ${response.status}`);
      }
      storyActionStatus = `${mode === 'autoradio' ? 'Autoradio brief' : 'Ask Choir'} submitted: ${payload.submission_id.slice(0, 8)}`;
    } catch (error) {
      storyActionStatus = error?.message || 'Prompt submission failed';
    } finally {
      storyActionBusy = '';
    }
  }

  async function submitContribution() {
    const text = contributionText.trim();
    let record = {
      id: `contribution-${Date.now()}`,
      kind: contributionKind,
      storyId: selectedStory.id,
      headline: selectedStory.headline,
      text: text || 'Draft contribution awaiting detail.',
      owner: currentUser?.email || 'public-preview',
      research_state: authenticated ? 'pending-researcher-review' : 'preview-only',
    };
    if (authenticated) {
      try {
        const response = await fetch('/api/global-wire/contributions', {
          method: 'POST',
          credentials: 'include',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            story_id: selectedStory.id,
            kind: contributionKind,
            headline: selectedStory.headline,
            text: record.text,
          }),
        });
        if (!response.ok) throw new Error(`Contribution queue failed: ${response.status}`);
        const saved = await response.json();
        record = {
          ...record,
          id: saved.id || record.id,
          storyId: saved.storyId || selectedStory.id,
          text: saved.text || record.text,
          source_content_id: saved.source_content_id || '',
          research_state: saved.research_state || record.research_state,
        };
      } catch (error) {
        contributionStatus = error?.message || 'Contribution queue failed';
        return;
      }
    }
    contributions = [record, ...contributions].slice(0, 6);
    contributionStatus = authenticated
      ? 'Contribution queued for research/reconciliation'
      : 'Local contribution preview - sign in to save';
    if (authenticated) {
      await loadContributions(selectedStory.id);
    }
    launchVText({
      title: `Contribution: ${selectedStory.headline}`,
      content: contributionContent(),
      createdFrom: 'global_wire_user_contribution',
      sourcePath: `contributions/${selectedStory.id}-${record.kind}.vtext`,
    });
    contributionText = '';
  }

  async function searchSources() {
    const query = sourceSearchQuery.trim();
    if (!authenticated) {
      sourceSearchStatus = 'sign-in-required';
      sourceSearchMessage = 'Sign in to search and import source evidence.';
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
          max_results: 5,
          story_id: selectedStory.id,
          queue_top_result: queueTopSourceResult,
        }),
      });
      const payload = await response.json();
      sourceSearchStatus = payload.status || (response.ok ? 'ok' : 'unavailable');
      sourceSearchMessage = payload.message || `${payload.source || 'source-service'}: ${sourceSearchStatus}`;
      sourceSearchResults = Array.isArray(payload.content_items) ? payload.content_items : [];
      if (payload.contribution?.id) {
        contributionStatus = 'Source evidence imported and queued for reconciliation';
        await loadContributions(selectedStory.id);
      } else if (sourceSearchStatus === 'ok') {
        contributionStatus = 'Source evidence imported';
      }
    } catch (error) {
      sourceSearchStatus = 'unavailable';
      sourceSearchMessage = error?.message || 'Source search failed';
      sourceSearchResults = [];
    } finally {
      sourceSearchBusy = false;
    }
  }

  async function refreshStorySources() {
    if (!authenticated) {
      sourceRefreshStatus = 'Sign in to refresh source evidence.';
      return;
    }
    if (!selectedStory?.id) return;
    sourceRefreshBusy = true;
    sourceRefreshStatus = '';
    try {
      const response = await fetch('/api/global-wire/source-refresh', {
        method: 'POST',
        credentials: 'include',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          story_id: selectedStory.id,
          query: sourceSearchQuery.trim() || selectedStory.headline,
          max_results: 3,
        }),
      });
      const payload = await response.json();
      sourceRefreshStatus = payload.message || payload.status || `Source refresh ${response.status}`;
      if (payload.content_item?.content_id) {
        reconciliationSourceItems = {
          ...reconciliationSourceItems,
          [payload.content_item.content_id]: payload.content_item,
        };
      }
      if (payload.contribution?.id) {
        contributions = [payload.contribution, ...contributions]
          .filter(Boolean)
          .slice(0, 6);
      }
      if (payload.decision?.id) {
        reconciliationDecisions = [payload.decision, ...reconciliationDecisions]
          .filter(Boolean)
          .slice(0, 20);
      }
      if (payload.candidate?.id) {
        graphUpdateCandidates = [payload.candidate, ...graphUpdateCandidates]
          .filter(Boolean)
          .slice(0, 20);
      }
      if (payload.refresh_run?.id) {
        sourceRefreshes = [payload.refresh_run, ...sourceRefreshes]
          .filter(Boolean)
          .slice(0, 20);
      }
      if (payload.claim_record?.id) {
        claimRecords = [payload.claim_record, ...claimRecords]
          .filter(Boolean)
          .slice(0, 30);
      }
      if (payload.research_task?.id) {
        researchTasks = [payload.research_task, ...researchTasks]
          .filter(Boolean)
          .slice(0, 30);
      }
      if (response.status === 201) {
        contributionStatus = `Source refresh classified ${payload.refresh_run?.update_classification || 'candidate evidence'} for review`;
        await loadContributions(selectedStory.id);
      }
    } catch (error) {
      sourceRefreshStatus = error?.message || 'Source refresh failed';
    } finally {
      sourceRefreshBusy = false;
    }
  }

  async function runFetchCycle() {
    if (!authenticated) {
      fetchCycleStatus = 'Sign in to run a bounded source-registry fetch cycle.';
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
          story_ids: [selectedStory.id],
          max_stories: 1,
          max_results: 2,
          trigger: 'global-wire-app-bounded-cycle',
        }),
      });
      const payload = await response.json();
      fetchCycleStatus = payload.message || payload.status || `Fetch cycle ${response.status}`;
      if (payload.fetch_cycle?.id) {
        fetchCycles = [payload.fetch_cycle, ...fetchCycles]
          .filter(Boolean)
          .slice(0, 20);
      }
      if (Array.isArray(payload.registry_entries)) {
        sourceRegistryEntries = [...payload.registry_entries, ...sourceRegistryEntries]
          .filter(Boolean)
          .slice(0, 20);
      }
      if (Array.isArray(payload.refresh_runs)) {
        sourceRefreshes = [...payload.refresh_runs, ...sourceRefreshes]
          .filter(Boolean)
          .slice(0, 20);
      }
      if (Array.isArray(payload.content_items)) {
        const nextItems = {};
        for (const item of payload.content_items) {
          if (item?.content_id) nextItems[item.content_id] = item;
        }
        reconciliationSourceItems = {
          ...reconciliationSourceItems,
          ...nextItems,
        };
      }
      if (Array.isArray(payload.contributions) && payload.contributions.length) {
        contributions = [...payload.contributions, ...contributions]
          .filter(Boolean)
          .slice(0, 6);
      }
      if (Array.isArray(payload.candidates) && payload.candidates.length) {
        graphUpdateCandidates = [...payload.candidates, ...graphUpdateCandidates]
          .filter(Boolean)
          .slice(0, 20);
      }
      if (Array.isArray(payload.claim_records) && payload.claim_records.length) {
        claimRecords = [...payload.claim_records, ...claimRecords]
          .filter(Boolean)
          .slice(0, 30);
      }
      if (Array.isArray(payload.research_tasks) && payload.research_tasks.length) {
        researchTasks = [...payload.research_tasks, ...researchTasks]
          .filter(Boolean)
          .slice(0, 30);
      }
      await loadContributions(selectedStory.id);
    } catch (error) {
      fetchCycleStatus = error?.message || 'Fetch cycle failed';
    } finally {
      fetchCycleBusy = false;
    }
  }

  function contributionSource(item) {
    const contentId = item?.source_content_id || item?.sourceContentId || '';
    return contentId ? reconciliationSourceItems[contentId] : null;
  }

  function contributionDecision(item) {
    const id = item?.id || '';
    return reconciliationDecisions.find((decision) => decision.contribution_id === id);
  }

  function contributionCandidate(item) {
    const id = item?.id || '';
    return graphUpdateCandidates.find((candidate) => candidate.contribution_id === id);
  }

  function candidatePromotion(candidate) {
    const id = candidate?.id || '';
    return graphPromotionDecisions.find((promotion) => promotion.candidate_id === id);
  }

  function candidateProjectionReviews(candidate) {
    const id = candidate?.id || '';
    return projectionReviews.filter((review) => review.candidate_id === id);
  }

  function candidateClaimRecords(candidate) {
    const id = candidate?.id || '';
    return claimRecords.filter((claim) => claim.candidate_id === id);
  }

  function claimResearchTasks(claim) {
    const id = claim?.id || '';
    return researchTasks.filter((task) => task.claim_id === id);
  }

  async function createProjectionDraft(review) {
    if (!authenticated || !review?.id) return;
    reconciliationBusyId = `${review.id}:draft`;
    contributionStatus = '';
    try {
      const response = await fetch('/api/global-wire/projection-reviews', {
        method: 'POST',
        credentials: 'include',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ review_id: review.id }),
      });
      if (!response.ok) throw new Error(`Projection draft failed: ${response.status}`);
      const payload = await response.json();
      projectionReviews = projectionReviews.map((item) => (
        item.id === review.id ? payload.review : item
      ));
      contributionStatus = 'Projection draft VText created';
      if (payload.document?.doc_id) {
        launchVText({
          title: payload.document.title || `Draft projection: ${selectedStory.headline}`,
          content: payload.revision?.content || '',
          createdFrom: 'global_wire_projection_review_draft',
          sourcePath: `global-wire/projection-drafts/${review.id}.vtext`,
          docId: payload.document.doc_id,
          createInitialVersion: false,
        });
      }
      await loadContributions(selectedStory.id);
    } catch (error) {
      contributionStatus = error?.message || 'Projection draft failed';
    } finally {
      reconciliationBusyId = '';
    }
  }

  async function reviewProjectionDraft(review, action) {
    if (!authenticated || !review?.id) return;
    reconciliationBusyId = `${review.id}:${action}`;
    contributionStatus = '';
    try {
      const response = await fetch('/api/global-wire/projection-reviews', {
        method: 'POST',
        credentials: 'include',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ review_id: review.id, action }),
      });
      if (!response.ok) throw new Error(`Projection review ${action} failed: ${response.status}`);
      const payload = await response.json();
      projectionReviews = projectionReviews.map((item) => (
        item.id === review.id ? payload.review : item
      ));
      contributionStatus = action === 'approve'
        ? 'Projection draft approved into Story VText revision'
        : 'Projection draft rejected';
      if (action === 'approve' && payload.document?.doc_id) {
        launchVText({
          title: payload.document.title || `Projection: ${selectedStory.headline}`,
          content: payload.revision?.content || '',
          createdFrom: 'global_wire_projection_review_approval',
          sourcePath: `global-wire/${selectedStory.id}.${review.style_id}.story.vtext`,
          docId: payload.document.doc_id,
          createInitialVersion: false,
        });
        lastLoadKey = '';
        await loadDurableStoryGraph();
      }
      await loadContributions(selectedStory.id);
    } catch (error) {
      contributionStatus = error?.message || `Projection review ${action} failed`;
    } finally {
      reconciliationBusyId = '';
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
      if (payload.story?.id) {
        stories = stories.map((story) => (story.id === payload.story.id ? payload.story : story));
      }
      if (payload.style?.id) {
        styleSources = payload.story?.style_sources || [...styleSources, payload.style];
        selectedStyleId = payload.style.id;
      }
      styleSourceStatus = action === 'compose'
        ? 'Composed Style.vtext source created'
        : 'Replacement Style.vtext source created';
      lastLoadKey = '';
      await loadDurableStoryGraph();
    } catch (error) {
      styleSourceStatus = error?.message || `Style ${action} failed`;
    } finally {
      styleSourceBusy = false;
    }
  }

  async function reconcileContribution(item, decision) {
    if (!authenticated || !item?.id) return;
    reconciliationBusyId = `${item.id}:${decision}`;
    contributionStatus = '';
    try {
      const response = await fetch('/api/global-wire/reconciliation', {
        method: 'POST',
        credentials: 'include',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          contribution_id: item.id,
          decision,
          note: decision === 'accepted'
            ? 'Accepted for graph review from the Global Wire desk.'
            : 'Rejected for this StoryGraph neighborhood from the Global Wire desk.',
        }),
      });
      if (!response.ok) throw new Error(`Reconciliation decision failed: ${response.status}`);
      const payload = await response.json();
      contributions = contributions.map((contribution) => (
        contribution.id === item.id ? payload.contribution : contribution
      ));
      if (payload.source_item?.content_id) {
        reconciliationSourceItems = {
          ...reconciliationSourceItems,
          [payload.source_item.content_id]: payload.source_item,
        };
      }
      reconciliationDecisions = [payload.decision, ...reconciliationDecisions]
        .filter(Boolean)
        .slice(0, 20);
      if (payload.candidate?.id) {
        graphUpdateCandidates = [payload.candidate, ...graphUpdateCandidates]
          .filter(Boolean)
          .slice(0, 20);
      }
      contributionStatus = decision === 'accepted'
        ? 'Contribution accepted for graph review'
        : 'Contribution rejected for this story neighborhood';
      await loadContributions(selectedStory.id);
    } catch (error) {
      contributionStatus = error?.message || 'Reconciliation decision failed';
    } finally {
      reconciliationBusyId = '';
    }
  }

  async function reviewGraphCandidate(candidate, decision) {
    if (!authenticated || !candidate?.id) return;
    reconciliationBusyId = `${candidate.id}:${decision}`;
    contributionStatus = '';
    try {
      const response = await fetch('/api/global-wire/graph-candidates', {
        method: 'POST',
        credentials: 'include',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          candidate_id: candidate.id,
          decision,
          note: decision === 'promoted'
            ? 'Platform review promoted this candidate as a bounded StoryGraph source-manifest update.'
            : 'Platform review rejected this candidate for the current StoryGraph.',
        }),
      });
      if (!response.ok) throw new Error(`Graph candidate review failed: ${response.status}`);
      const payload = await response.json();
      graphUpdateCandidates = graphUpdateCandidates.map((item) => (
        item.id === candidate.id ? payload.candidate : item
      ));
      graphPromotionDecisions = [payload.promotion, ...graphPromotionDecisions]
        .filter(Boolean)
        .slice(0, 20);
      if (Array.isArray(payload.projection_reviews) && payload.projection_reviews.length) {
        projectionReviews = [...payload.projection_reviews, ...projectionReviews]
          .filter(Boolean)
          .slice(0, 30);
      }
      if (payload.story?.id) {
        stories = stories.map((story) => (story.id === payload.story.id ? payload.story : story));
      }
      contributionStatus = decision === 'promoted'
        ? 'Graph candidate promoted through platform review'
        : 'Graph candidate rejected by platform review';
      await loadContributions(selectedStory.id);
    } catch (error) {
      contributionStatus = error?.message || 'Graph candidate review failed';
    } finally {
      reconciliationBusyId = '';
    }
  }
</script>

<section class="global-wire" data-global-wire-app data-global-wire-data-source={dataSource}>
  <header class="wire-header">
    <div>
      <p class="eyebrow">Global Wire</p>
      <h2>StoryGraph desk</h2>
    </div>
    <div class="wire-state" data-global-wire-state>
      <span>{authenticated ? 'owner computer' : 'public preview'}</span>
      <strong>{stories.length} story nodes</strong>
      <small>{dataSource}</small>
    </div>
  </header>
  {#if loadError}
    <p class="wire-load-error">{loadError}</p>
  {/if}

  <main class="wire-layout">
    <section class="front-page" data-global-wire-front-page aria-label="Front page stories">
      <div class="section-title">
        <h3>Front Page</h3>
        <span>headline nodes</span>
      </div>
      <div class="story-list">
        {#each stories as story}
          <button
            type="button"
            class:selected={story.id === selectedStory.id}
            class="story-row"
            data-global-wire-story
            data-story-id={story.id}
            on:click={() => (selectedStoryId = story.id)}
          >
            <span class={`story-dot ${story.nodeTone}`}></span>
            <span class="story-copy">
              <strong>{story.headline}</strong>
              <small>{story.freshness} · {story.tension}</small>
            </span>
            <span class="prominence">{story.prominence}</span>
          </button>
        {/each}
      </div>
    </section>

    <section class="story-reader" data-global-wire-story-reader aria-label="Story reader">
      <div class="story-reader-header">
        <div>
          <p class="eyebrow">{selectedStory.changeState}</p>
          <h1>{selectedStory.headline}</h1>
        </div>
        <div class="reader-actions">
          <button
            type="button"
            on:click={() => submitStoryAction('ask')}
            disabled={storyActionBusy !== ''}
            data-global-wire-ask-choir
          >
            Ask Choir
          </button>
          <button
            type="button"
            on:click={() => submitStoryAction('autoradio')}
            disabled={storyActionBusy !== ''}
            data-global-wire-autoradio
          >
            Autoradio
          </button>
          <button type="button" on:click={openStoryVText} data-global-wire-open-vtext>Open VText</button>
          <button type="button" on:click={forkStory} data-global-wire-fork-story>Fork/Edit</button>
        </div>
      </div>
      {#if storyActionStatus}
        <p class="story-action-status" data-global-wire-story-action-status>{storyActionStatus}</p>
      {/if}

      <p class="dek">{selectedStory.dek}</p>

      <div class="style-switcher" data-global-wire-style-switcher>
        <div class="section-title">
          <h3>Style.vtext Projection</h3>
          <div class="style-actions">
            <button type="button" on:click={openStyleVText} data-global-wire-open-style>Open style source</button>
            {#if authenticated}
              <button type="button" on:click={() => updateStyleSource('compose')} disabled={styleSourceBusy} data-global-wire-compose-style>Compose</button>
              <button type="button" on:click={() => updateStyleSource('replace')} disabled={styleSourceBusy} data-global-wire-replace-style>Replace</button>
            {/if}
          </div>
        </div>
        <div class="style-tabs" role="tablist" aria-label="Style source">
          {#each styleSources as style}
            <button
              type="button"
              role="tab"
              class:active={style.id === selectedStyle.id}
              aria-selected={style.id === selectedStyle.id}
              on:click={() => (selectedStyleId = style.id)}
            >
              {style.label}
            </button>
          {/each}
        </div>
        <article class="projection">
          <p>{projectionText}</p>
          <small>Cites {selectedStyle.title}; evidence manifest unchanged.</small>
        </article>
        {#if styleSourceStatus}
          <p class="style-source-status" data-global-wire-style-source-status>{styleSourceStatus}</p>
        {/if}
      </div>

      <div class="claims" data-global-wire-claims>
        <h3>Claims</h3>
        <ul>
          {#each selectedStory.claims as claim}
            <li>{claim}</li>
          {/each}
        </ul>
      </div>
    </section>

    <aside class="right-rail" aria-label="Evidence and graph">
      <section class="evidence" data-global-wire-evidence>
        <div class="section-title">
          <h3>Evidence</h3>
          <span>{allSources.length} sources</span>
        </div>
        {#each ['lead', 'supporting', 'contrary', 'context'] as tier}
          <div class="source-tier" data-source-tier={tier}>
            <h4>{tier}</h4>
            {#each selectedStory.manifest[tier] as source}
              <div class="source-item">
                <strong>{source.title}</strong>
                <small>{source.standing}</small>
              </div>
            {/each}
          </div>
        {/each}
      </section>

      <section class="graph" data-global-wire-story-graph>
        <div class="section-title">
          <h3>StoryGraph</h3>
          <span>source neighborhood</span>
        </div>
        <div class="graph-canvas">
          {#each stories as story}
            <button
              type="button"
              class:selected={story.id === selectedStory.id}
              class={`graph-node ${story.nodeTone}`}
              style={`--node-size: ${Math.max(58, Math.min(112, story.prominence + 20))}px`}
              on:click={() => (selectedStoryId = story.id)}
            >
              <span>{story.headline}</span>
            </button>
          {/each}
        </div>
      </section>

      <section class="contribution" data-global-wire-contribution>
        <div class="section-title">
          <h3>Contribute</h3>
          <span>research queue</span>
        </div>
        <div class="source-search" data-global-wire-source-search>
          <label>
            <span>Source search</span>
            <input
              bind:value={sourceSearchQuery}
              type="search"
              placeholder="Search live/source-service evidence"
              data-global-wire-source-search-input
            />
          </label>
          <label class="queue-toggle">
            <input type="checkbox" bind:checked={queueTopSourceResult} />
            <span>Queue top result</span>
          </label>
          <button
            type="button"
            class="source-search-button"
            on:click={searchSources}
            disabled={sourceSearchBusy}
            data-global-wire-source-search-submit
          >
            {sourceSearchBusy ? 'Searching...' : 'Search sources'}
          </button>
          <button
            type="button"
            class="source-search-button"
            on:click={refreshStorySources}
            disabled={sourceRefreshBusy}
            data-global-wire-source-refresh
          >
            {sourceRefreshBusy ? 'Refreshing...' : 'Refresh story evidence'}
          </button>
          <button
            type="button"
            class="source-search-button"
            on:click={runFetchCycle}
            disabled={fetchCycleBusy}
            data-global-wire-fetch-cycle
          >
            {fetchCycleBusy ? 'Running...' : 'Run fetch cycle'}
          </button>
          {#if sourceSearchStatus}
            <p class="source-search-status" data-global-wire-source-search-status>
              {sourceSearchStatus}: {sourceSearchMessage}
            </p>
          {/if}
          {#if sourceRefreshStatus}
            <p class="source-search-status" data-global-wire-source-refresh-status>
              {sourceRefreshStatus}
            </p>
          {/if}
          {#if fetchCycleStatus}
            <p class="source-search-status" data-global-wire-fetch-cycle-status>
              {fetchCycleStatus}
            </p>
          {/if}
          {#if fetchCycles.length || sourceRegistryEntries.length}
            <div class="source-search-results" data-global-wire-fetch-cycle-runs>
              {#each fetchCycles.slice(0, 2) as cycle}
                <article>
                  <strong>{cycle.status}</strong>
                  <small>{cycle.trigger} · {cycle.refresh_run_ids?.length || 0} refreshes · {cycle.source_content_ids?.length || 0} sources</small>
                  <span>{cycle.message}</span>
                </article>
              {/each}
              {#each sourceRegistryEntries.slice(0, 2) as entry}
                <article data-global-wire-source-registry-entry>
                  <strong>{entry.source_scope}</strong>
                  <small>{entry.status} · {entry.story_id}</small>
                  <span>{entry.query}</span>
                </article>
              {/each}
            </div>
          {/if}
          {#if sourceRefreshes.length}
            <div class="source-search-results" data-global-wire-source-refresh-runs>
              {#each sourceRefreshes.slice(0, 2) as run}
                <article>
                  <strong>{run.update_classification || run.status}</strong>
                  <small>{run.storygraph_action || run.status} · {run.projection_action || 'projection pending'} · {run.provider}</small>
                  <span>{run.query}</span>
                </article>
              {/each}
            </div>
          {/if}
          {#if sourceSearchResults.length}
            <div class="source-search-results" data-global-wire-source-search-results>
              {#each sourceSearchResults.slice(0, 3) as result}
                <article>
                  <strong>{result.title}</strong>
                  <small>{result.source_type} · {result.metadata?.schema || 'source artifact'}</small>
                </article>
              {/each}
            </div>
          {/if}
        </div>
        <label>
          <span>Kind</span>
          <select bind:value={contributionKind}>
            <option value="source">Add source</option>
            <option value="counter-source">Counter-source</option>
            <option value="claim-dispute">Dispute claim</option>
            <option value="argument">Make argument</option>
            <option value="research-request">Request research</option>
          </select>
        </label>
        <label>
          <span>Contribution</span>
          <textarea bind:value={contributionText} rows="4" placeholder="Add source URL, claim note, counter-evidence, or research request"></textarea>
        </label>
        <button type="button" class="submit-contribution" on:click={submitContribution} data-global-wire-submit-contribution>
          Create user-owned contribution
        </button>
        {#if contributionStatus}
          <p class="contribution-status">{contributionStatus}</p>
        {/if}
        {#if contributions.length}
          <div class="contribution-list" data-global-wire-contribution-list>
            {#each contributions as item}
              {@const source = contributionSource(item)}
              {@const decision = contributionDecision(item)}
              {@const candidate = contributionCandidate(item)}
              {@const promotion = candidatePromotion(candidate)}
              {@const reviews = candidateProjectionReviews(candidate)}
              {@const claims = candidateClaimRecords(candidate)}
              <article class="contribution-card" data-global-wire-reconciliation-item>
                <p><strong>{item.kind.replaceAll('-', ' ')}</strong> · {item.text}</p>
                <small>{item.research_state || 'pending-researcher-review'}</small>
                {#if source}
                  <div class="reconciliation-source" data-global-wire-reconciliation-source>
                    <strong>{source.title}</strong>
                    <small>{source.source_type} · {source.metadata?.schema || 'source artifact'}</small>
                  </div>
                {/if}
                {#if decision}
                  <small data-global-wire-reconciliation-decision>{decision.decision}: {decision.note}</small>
                  {#if candidate}
                    <div
                      class="graph-candidate"
                      data-global-wire-graph-candidate
                      data-global-wire-candidate-id={candidate.id}
                    >
                      <strong>{candidate.candidate_kind}</strong>
                      <small>{candidate.source_tier} · {candidate.edge_kind} · {candidate.status}</small>
                      <span>{candidate.projection_action}</span>
                      {#if claims.length}
                        <div class="claim-research-list" data-global-wire-claim-records>
                          {#each claims.slice(0, 2) as claim}
                            {@const tasks = claimResearchTasks(claim)}
                            <article data-global-wire-claim-record data-global-wire-claim-id={claim.id}>
                              <strong>{claim.claim_kind}</strong>
                              <small>{claim.uncertainty_state} · {claim.dispute_state} · {claim.status}</small>
                              <span>{claim.claim_text}</span>
                              <em>{claim.evidence_gap}</em>
                              {#each tasks.slice(0, 2) as task}
                                <small data-global-wire-research-task>
                                  {task.task_kind}: {task.status} · {task.priority}
                                </small>
                              {/each}
                            </article>
                          {/each}
                        </div>
                      {/if}
                      {#if promotion}
                        <small data-global-wire-graph-promotion>
                          {promotion.decision}: {promotion.applied_change}
                        </small>
                        {#if reviews.length}
                          <div class="projection-review-list" data-global-wire-projection-reviews>
                            {#each reviews.slice(0, 3) as review}
                              <small data-global-wire-projection-review>
                                {review.status}: {review.style_title || review.style_id}
                              </small>
                              {#if review.draft_story_doc_id}
                                <button
                                  type="button"
                                  on:click={() => launchVText({
                                    title: `Draft projection: ${selectedStory.headline}`,
                                    content: '',
                                    createdFrom: 'global_wire_projection_review_draft',
                                    sourcePath: `global-wire/projection-drafts/${review.id}.vtext`,
                                    docId: review.draft_story_doc_id,
                                    createInitialVersion: false,
                                  })}
                                  data-global-wire-open-projection-draft
                                  data-global-wire-projection-review-id={review.id}
                                >
                                  Open draft
                                </button>
                                {#if authenticated && review.status === 'draft-created'}
                                  <button
                                    type="button"
                                    on:click={() => reviewProjectionDraft(review, 'approve')}
                                    disabled={reconciliationBusyId !== ''}
                                    data-global-wire-approve-projection-draft
                                    data-global-wire-projection-review-id={review.id}
                                  >
                                    Approve draft
                                  </button>
                                  <button
                                    type="button"
                                    on:click={() => reviewProjectionDraft(review, 'reject')}
                                    disabled={reconciliationBusyId !== ''}
                                    data-global-wire-reject-projection-draft
                                    data-global-wire-projection-review-id={review.id}
                                  >
                                    Reject draft
                                  </button>
                                {/if}
                              {:else if authenticated}
                                <button
                                  type="button"
                                  on:click={() => createProjectionDraft(review)}
                                  disabled={reconciliationBusyId !== ''}
                                  data-global-wire-create-projection-draft
                                  data-global-wire-projection-review-id={review.id}
                                >
                                  Draft VText
                                </button>
                              {/if}
                            {/each}
                          </div>
                        {/if}
                      {:else if authenticated && candidate.status === 'candidate-review'}
                        <div class="reconciliation-actions">
                          <button
                            type="button"
                            on:click={() => reviewGraphCandidate(candidate, 'promoted')}
                            disabled={reconciliationBusyId !== ''}
                            data-global-wire-promote-candidate
                          >
                            Promote
                          </button>
                          <button
                            type="button"
                            on:click={() => reviewGraphCandidate(candidate, 'rejected')}
                            disabled={reconciliationBusyId !== ''}
                            data-global-wire-reject-candidate
                          >
                            Reject
                          </button>
                        </div>
                      {/if}
                    </div>
                  {/if}
                {:else if authenticated}
                  <div class="reconciliation-actions">
                    <button
                      type="button"
                      on:click={() => reconcileContribution(item, 'accepted')}
                      disabled={reconciliationBusyId !== ''}
                      data-global-wire-reconcile-accept
                    >
                      Accept
                    </button>
                    <button
                      type="button"
                      on:click={() => reconcileContribution(item, 'rejected')}
                      disabled={reconciliationBusyId !== ''}
                      data-global-wire-reconcile-reject
                    >
                      Reject
                    </button>
                  </div>
                {/if}
              </article>
            {/each}
          </div>
        {/if}
      </section>
    </aside>
  </main>
</section>

<style>
  .global-wire {
    display: flex;
    flex-direction: column;
    min-height: 100%;
    gap: 0.75rem;
    color: var(--choir-text-primary);
  }

  .wire-header,
  .wire-layout,
  .front-page,
  .story-reader,
  .right-rail,
  .evidence,
  .graph,
  .contribution,
  .style-switcher,
  .claims {
    min-width: 0;
  }

  .wire-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 1rem;
    padding: 0.9rem 1rem;
    border: 1px solid var(--choir-border);
    border-radius: 8px;
    background: var(--choir-surface-pane);
  }

  .eyebrow,
  .section-title span,
  .wire-state span,
  small {
    color: var(--choir-text-muted);
  }

  .eyebrow {
    margin: 0 0 0.25rem;
    font-size: 0.72rem;
    font-weight: 760;
    text-transform: uppercase;
    letter-spacing: 0;
  }

  h1,
  h2,
  h3,
  h4,
  p {
    margin: 0;
  }

  h1 {
    font-family: var(--choir-font-display);
    font-size: clamp(1.45rem, 3vw, 2.45rem);
    line-height: 1.05;
    letter-spacing: 0;
  }

  h2 {
    font-size: 1.25rem;
    letter-spacing: 0;
  }

  h3 {
    font-size: 0.95rem;
    letter-spacing: 0;
  }

  h4 {
    color: var(--choir-text-muted);
    font-size: 0.72rem;
    text-transform: uppercase;
    letter-spacing: 0;
  }

  .wire-state {
    display: grid;
    gap: 0.15rem;
    justify-items: end;
    font-size: 0.82rem;
  }

  .wire-load-error {
    padding: 0 1rem;
    color: var(--choir-text-muted);
    font-size: 0.82rem;
  }

  .wire-layout {
    display: grid;
    grid-template-columns: minmax(220px, 0.72fr) minmax(360px, 1.42fr) minmax(280px, 0.9fr);
    gap: 0.75rem;
    flex: 1;
    overflow: hidden;
  }

  .front-page,
  .story-reader,
  .right-rail > section {
    border: 1px solid var(--choir-border);
    border-radius: 8px;
    background: var(--choir-surface-app);
    overflow: auto;
  }

  .front-page,
  .story-reader {
    overflow: auto;
  }

  .front-page,
  .story-reader,
  .right-rail > section {
    padding: 0.85rem;
  }

  .right-rail {
    display: grid;
    grid-template-rows: minmax(220px, 1fr) minmax(220px, 0.9fr) minmax(240px, 0.9fr);
    gap: 0.75rem;
    overflow: auto;
  }

  .section-title {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.75rem;
    margin-bottom: 0.65rem;
  }

  .story-list {
    display: grid;
    gap: 0.5rem;
  }

  .story-row,
  .reader-actions button,
  .style-tabs button,
  .section-title button,
  .submit-contribution,
  .source-search-button,
  .reconciliation-actions button,
  .graph-node {
    min-height: 2.35rem;
    border: 1px solid var(--choir-border);
    border-radius: 8px;
    background: var(--choir-surface-control);
    color: var(--choir-text-primary);
    font: inherit;
    cursor: pointer;
  }

  .story-row {
    display: grid;
    grid-template-columns: 0.8rem minmax(0, 1fr) auto;
    align-items: center;
    gap: 0.55rem;
    width: 100%;
    padding: 0.7rem;
    text-align: left;
  }

  .story-row.selected,
  .style-tabs button.active,
  .graph-node.selected {
    border-color: var(--choir-border-strong);
    background: var(--choir-state-selected);
  }

  .story-copy {
    display: grid;
    gap: 0.25rem;
    min-width: 0;
  }

  .story-copy strong {
    overflow-wrap: anywhere;
    line-height: 1.2;
  }

  .prominence {
    color: var(--choir-text-accent);
    font-weight: 760;
  }

  .story-dot {
    width: 0.7rem;
    height: 0.7rem;
    border-radius: 999px;
    background: var(--choir-chart-1);
  }

  .story-dot.changed,
  .graph-node.changed {
    background: var(--choir-status-warning-soft);
  }

  .story-dot.cooling,
  .graph-node.cooling {
    background: var(--choir-status-success-soft);
  }

  .story-reader {
    display: grid;
    align-content: start;
    gap: 0.9rem;
  }

  .story-reader-header {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    gap: 0.85rem;
    align-items: start;
  }

  .reader-actions {
    display: flex;
    flex-wrap: wrap;
    gap: 0.45rem;
    justify-content: flex-end;
  }

  .reader-actions button,
  .section-title button,
  .style-tabs button,
  .submit-contribution,
  .source-search-button,
  .reconciliation-actions button {
    padding: 0.45rem 0.7rem;
    font-weight: 720;
  }

  .dek {
    color: var(--choir-text-muted);
    line-height: 1.45;
  }

  .story-action-status {
    color: var(--choir-text-accent);
    font-size: 0.86rem;
  }

  .style-switcher,
  .claims {
    display: grid;
    gap: 0.7rem;
    padding: 0.75rem;
    border: 1px solid var(--choir-border);
    border-radius: 8px;
    background: var(--choir-surface-pane);
  }

  .style-tabs {
    display: flex;
    flex-wrap: wrap;
    gap: 0.45rem;
  }

  .style-actions {
    display: flex;
    flex-wrap: wrap;
    gap: 0.45rem;
    justify-content: flex-end;
  }

  .projection {
    display: grid;
    gap: 0.55rem;
    padding: 0.75rem;
    border-left: 3px solid var(--choir-border-strong);
    background: var(--choir-surface-card);
  }

  .projection p,
  .claims li {
    line-height: 1.45;
  }

  .style-source-status {
    color: var(--choir-text-accent);
    font-size: 0.82rem;
  }

  .claims ul {
    display: grid;
    gap: 0.45rem;
    margin: 0;
    padding-left: 1.1rem;
  }

  .source-tier {
    display: grid;
    gap: 0.4rem;
    margin-bottom: 0.7rem;
  }

  .source-item {
    display: grid;
    gap: 0.18rem;
    padding: 0.55rem;
    border: 1px solid var(--choir-border);
    border-radius: 8px;
    background: var(--choir-surface-card);
  }

  .graph-canvas {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 0.55rem;
    align-items: stretch;
  }

  .graph-node {
    display: grid;
    place-items: center;
    min-height: var(--node-size);
    padding: 0.45rem;
    background: var(--choir-state-hover);
    overflow: hidden;
  }

  .graph-node span {
    display: -webkit-box;
    -webkit-line-clamp: 4;
    -webkit-box-orient: vertical;
    overflow: hidden;
    font-size: 0.78rem;
    line-height: 1.18;
  }

  .contribution {
    display: grid;
    gap: 0.65rem;
  }

  label {
    display: grid;
    gap: 0.3rem;
  }

  label span {
    color: var(--choir-text-muted);
    font-size: 0.78rem;
    font-weight: 720;
  }

  input,
  select,
  textarea {
    width: 100%;
    box-sizing: border-box;
    border: 1px solid var(--choir-border);
    border-radius: 8px;
    background: var(--choir-surface-input);
    color: var(--choir-text-primary);
    font: inherit;
  }

  input,
  select {
    min-height: 2.35rem;
    padding: 0.35rem 0.5rem;
  }

  textarea {
    resize: vertical;
    min-height: 6rem;
    padding: 0.55rem;
    line-height: 1.35;
  }

  .submit-contribution,
  .source-search-button {
    background: var(--choir-accent);
    color: var(--choir-on-accent);
  }

  .source-search {
    display: grid;
    gap: 0.5rem;
    padding: 0.55rem;
    border: 1px solid var(--choir-border);
    border-radius: 8px;
    background: var(--choir-surface-pane);
  }

  .queue-toggle {
    grid-template-columns: auto minmax(0, 1fr);
    align-items: center;
  }

  .queue-toggle input {
    width: auto;
    min-height: 0;
  }

  .source-search-status,
  .contribution-status {
    color: var(--choir-text-accent);
    font-size: 0.85rem;
  }

  .source-search-results {
    display: grid;
    gap: 0.35rem;
    max-height: 8rem;
    overflow: auto;
  }

  .source-search-results article {
    display: grid;
    gap: 0.15rem;
    padding: 0.4rem;
    border: 1px solid var(--choir-border);
    border-radius: 8px;
    background: var(--choir-surface-card);
  }

  .source-search-results strong {
    overflow-wrap: anywhere;
  }

  .contribution-list {
    display: grid;
    gap: 0.4rem;
  }

  .contribution-card {
    display: grid;
    gap: 0.4rem;
    padding: 0.45rem;
    border: 1px solid var(--choir-border);
    border-radius: 8px;
    background: var(--choir-surface-card);
    font-size: 0.82rem;
    line-height: 1.3;
  }

  .contribution-card p {
    line-height: 1.3;
  }

  .reconciliation-source,
  .graph-candidate {
    display: grid;
    gap: 0.15rem;
    padding: 0.45rem;
    border: 1px solid var(--choir-border);
    border-radius: 8px;
    background: var(--choir-surface-pane);
  }

  .graph-candidate {
    border-left: 3px solid var(--choir-border-strong);
    position: relative;
  }

  .reconciliation-source strong,
  .graph-candidate strong,
  .graph-candidate span {
    overflow-wrap: anywhere;
  }

  .projection-review-list {
    display: grid;
    gap: 0.35rem;
  }

  .projection-review-list small {
    display: block;
  }

  .projection-review-list button {
    justify-self: start;
    position: relative;
    z-index: 1;
    min-height: 2.1rem;
    padding: 0.35rem 0.65rem;
  }

  .claim-research-list {
    display: grid;
    gap: 0.35rem;
  }

  .claim-research-list article {
    display: grid;
    gap: 0.18rem;
    padding: 0.4rem;
    border: 1px solid var(--choir-border);
    border-radius: 8px;
    background: var(--choir-surface-card);
  }

  .claim-research-list em {
    color: var(--choir-text-muted);
    font-style: normal;
    overflow-wrap: anywhere;
  }

  .reconciliation-actions {
    display: flex;
    flex-wrap: wrap;
    gap: 0.4rem;
  }

  .reconciliation-actions button {
    flex: 1 1 5.5rem;
  }

  @media (max-width: 1080px) {
    .wire-layout {
      grid-template-columns: minmax(220px, 0.8fr) minmax(340px, 1.2fr);
    }

    .right-rail {
      grid-column: 1 / -1;
      grid-template-columns: repeat(3, minmax(0, 1fr));
      grid-template-rows: auto;
    }
  }

  @media (max-width: 760px) {
    .global-wire {
      overflow: auto;
    }

    .wire-header,
    .story-reader-header,
    .wire-layout,
    .right-rail {
      display: grid;
      grid-template-columns: 1fr;
    }

    .wire-layout,
    .right-rail {
      overflow: visible;
    }

    .wire-state {
      justify-items: start;
    }

    .reader-actions {
      justify-content: stretch;
    }

    .reader-actions button,
    .section-title button {
      flex: 1 1 9rem;
    }
  }
</style>
