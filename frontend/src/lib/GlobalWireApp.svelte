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
  let sourceSchedulerRuns = [];
  let styleSourceStatus = '';
  let styleSourceBusy = false;
  let queueTopSourceResult = true;
  let storyActionStatus = '';
  let storyActionBusy = '';
  let contributions = [];
  let reconciliationSourceItems = {};
  let sourceDossiers = [];
  let reconciliationDecisions = [];
  let graphUpdateCandidates = [];
  let graphPromotionDecisions = [];
  let sourceRefreshes = [];
  let claimRecords = [];
  let researchTasks = [];
  let extractionArtifacts = [];
  let researchEvidence = [];
  let researchDecisions = [];
  let publicationUpdates = [];
  let publicationArtifacts = [];
  let publicationDeliveries = [];
  let autoradioScripts = [];
  let deliveryExports = [];
  let publicLinks = [];
  let newsletterSubscribers = [];
  let newsletterIssues = [];
  let newsletterDeliveries = [];
  let publicationFeedItems = [];
  let publicationFeedStatus = '';
  let publicationDeliveryDetail = null;
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
      sourceDossiers = [];
      reconciliationDecisions = [];
      graphUpdateCandidates = [];
      graphPromotionDecisions = [];
      sourceRefreshes = [];
      fetchCycles = [];
      sourceRegistryEntries = [];
      sourceSchedulerRuns = [];
      claimRecords = [];
      researchTasks = [];
      extractionArtifacts = [];
      publicationArtifacts = [];
      publicationDeliveries = [];
      autoradioScripts = [];
      deliveryExports = [];
      publicLinks = [];
      newsletterSubscribers = [];
      newsletterIssues = [];
      newsletterDeliveries = [];
      publicationFeedItems = [];
      publicationFeedStatus = '';
      publicationDeliveryDetail = null;
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
      sourceDossiers = Array.isArray(payload.source_dossiers) ? payload.source_dossiers : [];
      reconciliationDecisions = Array.isArray(payload.decisions) ? payload.decisions : [];
      graphUpdateCandidates = Array.isArray(payload.candidates) ? payload.candidates : [];
      graphPromotionDecisions = Array.isArray(payload.promotions) ? payload.promotions : [];
      sourceRefreshes = Array.isArray(payload.refreshes) ? payload.refreshes : [];
      claimRecords = Array.isArray(payload.claim_records) ? payload.claim_records : [];
      researchTasks = Array.isArray(payload.research_tasks) ? payload.research_tasks : [];
      extractionArtifacts = Array.isArray(payload.extraction_artifacts) ? payload.extraction_artifacts : [];
      researchEvidence = Array.isArray(payload.research_evidence) ? payload.research_evidence : [];
      researchDecisions = Array.isArray(payload.research_decisions) ? payload.research_decisions : [];
      publicationUpdates = Array.isArray(payload.publication_updates) ? payload.publication_updates : [];
      publicationArtifacts = Array.isArray(payload.publication_artifacts) ? payload.publication_artifacts : [];
      publicationDeliveries = Array.isArray(payload.publication_deliveries) ? payload.publication_deliveries : [];
      autoradioScripts = Array.isArray(payload.autoradio_scripts) ? payload.autoradio_scripts : [];
      deliveryExports = Array.isArray(payload.delivery_exports) ? payload.delivery_exports : [];
      publicLinks = Array.isArray(payload.public_links) ? payload.public_links : [];
      newsletterSubscribers = Array.isArray(payload.newsletter_subscribers) ? payload.newsletter_subscribers : [];
      newsletterIssues = Array.isArray(payload.newsletter_issues) ? payload.newsletter_issues : [];
      newsletterDeliveries = Array.isArray(payload.newsletter_deliveries) ? payload.newsletter_deliveries : [];
      projectionReviews = Array.isArray(payload.projection_reviews) ? payload.projection_reviews : [];
      await loadPublicationFeed(storyId);
      await loadFetchCycles(storyId);
    } catch {
      contributions = [];
      reconciliationSourceItems = {};
      sourceDossiers = [];
      reconciliationDecisions = [];
      graphUpdateCandidates = [];
      graphPromotionDecisions = [];
      sourceRefreshes = [];
      fetchCycles = [];
      sourceRegistryEntries = [];
      sourceSchedulerRuns = [];
      claimRecords = [];
      researchTasks = [];
      extractionArtifacts = [];
      researchEvidence = [];
      researchDecisions = [];
      publicationUpdates = [];
      publicationArtifacts = [];
      publicationDeliveries = [];
      autoradioScripts = [];
      deliveryExports = [];
      publicLinks = [];
      newsletterSubscribers = [];
      newsletterIssues = [];
      newsletterDeliveries = [];
      publicationFeedItems = [];
      publicationFeedStatus = '';
      publicationDeliveryDetail = null;
      projectionReviews = [];
    }
  }

  async function loadPublicationFeed(storyId = selectedStoryId) {
    if (!authenticated || !storyId) return;
    try {
      const response = await fetch(`/api/global-wire/publication-feed?story_id=${encodeURIComponent(storyId)}&channel=newsletter`, {
        credentials: 'include',
      });
      if (!response.ok) throw new Error(`Publication feed load failed: ${response.status}`);
      const payload = await response.json();
      publicationFeedItems = Array.isArray(payload.feed_items) ? payload.feed_items : [];
      publicationFeedStatus = payload.status || '';
    } catch {
      publicationFeedItems = [];
      publicationFeedStatus = '';
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
    const feedItem = mode === 'autoradio' ? selectedPublicationFeedItem() : null;
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
    if (feedItem?.artifact?.id) {
      return [
        'Create an Autoradio-ready spoken brief from the selected Global Wire publication artifact. Use the artifact body as the primary traversal object, keep uncertainty audible, cite the artifact and source neighborhood, and do not add facts beyond the artifact and source context.',
        '',
        `StoryGraph id: ${selectedStory.id}`,
        `Headline: ${selectedStory.headline}`,
        `State: ${selectedStory.changeState}; ${selectedStory.tension}`,
        `Style.vtext source: ${selectedStyle.title}`,
        '',
        'Publication Artifact:',
        `Artifact id: ${feedItem.artifact.id}`,
        `Status: ${feedItem.status || feedItem.artifact.status}`,
        `Channel: ${feedItem.artifact.channel}`,
        `Title: ${feedItem.artifact.title}`,
        `Citation count: ${feedItem.citation_count}`,
        `Rollback count: ${feedItem.rollback_count}`,
        '',
        'Artifact Body:',
        feedItem.artifact.body,
        '',
        'Source Context:',
        feedItem.source_item
          ? `${feedItem.source_item.title} (${feedItem.source_item.source_type}; ${feedItem.source_item.content_id})`
          : feedItem.artifact.source_content_id || 'Source manifest context only',
        '',
        'Citation Refs:',
        ...(feedItem.artifact.citation_refs || []).map((ref) => `- ${ref}`),
        '',
        'Rollback Refs:',
        ...(feedItem.artifact.rollback_refs || []).map((ref) => `- ${ref}`),
        '',
        'Related Story VTexts:',
        related,
        '',
        'Guardrail: speak from this citeable publication artifact, keep provenance audible, do not mutate the platform StoryGraph, and do not invent facts.',
      ].join('\n');
    }
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
      if (payload.extraction_artifact?.id) {
        extractionArtifacts = [payload.extraction_artifact, ...extractionArtifacts]
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

  async function runFetchCycle(schedulerMode = false) {
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
          trigger: schedulerMode ? 'global-wire-app-scheduled-standing-cycle' : 'global-wire-app-bounded-cycle',
          scheduler_mode: schedulerMode,
          cadence_seconds: schedulerMode ? 3600 : undefined,
        }),
      });
      const payload = await response.json();
      fetchCycleStatus = payload.message || payload.status || `Fetch cycle ${response.status}`;
      if (payload.fetch_cycle?.id) {
        fetchCycles = [payload.fetch_cycle, ...fetchCycles]
          .filter(Boolean)
          .slice(0, 20);
      }
      if (payload.scheduler_run?.id) {
        sourceSchedulerRuns = [payload.scheduler_run, ...sourceSchedulerRuns]
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
      if (Array.isArray(payload.extraction_artifacts) && payload.extraction_artifacts.length) {
        extractionArtifacts = [...payload.extraction_artifacts, ...extractionArtifacts]
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

  function selectedSourceDossier() {
    return sourceDossiers.find((dossier) => dossier.story_id === selectedStoryId) || sourceDossiers[0] || null;
  }

  function noteNewsletterIssueInDossiers(issue, deliveries = []) {
    const issueId = issue?.id || '';
    if (!issueId) return;
    const storyId = issue.story_id || selectedStoryId;
    sourceDossiers = sourceDossiers.map((dossier) => {
      if (dossier.story_id !== storyId) return dossier;
      const publicationRefs = dossier.publication_refs || {};
      const deliveryIds = deliveries
        .filter((delivery) => delivery?.issue_id === issueId || delivery?.story_id === storyId)
        .map((delivery) => delivery.id)
        .filter(Boolean);
      return {
        ...dossier,
        publication_refs: {
          ...publicationRefs,
          newsletter_issue_ids: Array.from(new Set([...(publicationRefs.newsletter_issue_ids || []), issueId])),
          newsletter_delivery_ids: Array.from(new Set([...(publicationRefs.newsletter_delivery_ids || []), ...deliveryIds])),
          citation_refs: Array.from(new Set([...(publicationRefs.citation_refs || []), ...(issue.citation_refs || [])])),
          rollback_refs: Array.from(new Set([...(publicationRefs.rollback_refs || []), ...(issue.rollback_refs || [])])),
        },
        missing_fields: (dossier.missing_fields || []).filter((field) => field !== 'newsletter_issues'),
      };
    });
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

  function claimExtractionArtifacts(claim) {
    const id = claim?.id || '';
    return extractionArtifacts.filter((artifact) => artifact.claim_id === id);
  }

  function taskEvidence(task) {
    const id = task?.id || '';
    return researchEvidence.filter((evidence) => evidence.task_id === id);
  }

  function evidenceDecision(evidence) {
    const id = evidence?.id || '';
    return researchDecisions.find((decision) => decision.evidence_id === id);
  }

  function publicationUpdateForDecision(decision) {
    const id = decision?.id || '';
    return publicationUpdates.find((update) => update.research_decision_id === id);
  }

  function publicationArtifactForUpdate(update) {
    const id = update?.id || '';
    return publicationArtifacts.find((artifact) => artifact.update_id === id);
  }

  function publicationDeliveryForArtifact(artifact) {
    const id = artifact?.id || '';
    return publicationDeliveries.find((delivery) => delivery.artifact_id === id);
  }

  function autoradioScriptForArtifact(artifact) {
    const id = artifact?.id || '';
    return autoradioScripts.find((script) => script.artifact_id === id);
  }

  function deliveryExportForDelivery(delivery) {
    const id = delivery?.id || '';
    return deliveryExports.find((item) => item.delivery_id === id);
  }

  function publicLinkForExport(deliveryExport) {
    const id = deliveryExport?.id || '';
    return publicLinks.find((item) => item.export_id === id);
  }

  function newsletterIssueForPublicLink(publicLink) {
    const id = publicLink?.id || '';
    return newsletterIssues.find((issue) => (issue.public_link_ids || []).includes(id));
  }

  function newsletterDeliveriesForIssue(issue) {
    const id = issue?.id || '';
    return newsletterDeliveries.filter((delivery) => delivery.issue_id === id);
  }

  function selectedPublicationFeedItem() {
    return publicationFeedItems.find((item) => item.story?.id === selectedStory.id || item.artifact?.story_id === selectedStory.id);
  }

  function researchTaskEvidenceSummary(task, action) {
    if (action === 'assign') {
      return `Research task assigned for ${task.task_kind || 'claim review'}; no platform story mutation applied.`;
    }
    if (action === 'block') {
      return `Research task blocked pending additional source evidence for ${task.claim_id || task.story_id}.`;
    }
    return `Research task completed for ${task.claim_id || task.story_id}; evidence is ready for reconciliation without mutating the platform StoryGraph.`;
  }

  async function updateResearchTask(task, action) {
    if (!authenticated || !task?.id) return;
    reconciliationBusyId = `${task.id}:${action}`;
    contributionStatus = '';
    try {
      const response = await fetch('/api/global-wire/research-tasks', {
        method: 'POST',
        credentials: 'include',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          task_id: task.id,
          action,
          evidence_level: 'reconciliation-level',
          evidence_summary: researchTaskEvidenceSummary(task, action),
          reviewer_note: `global-wire-app:${action}`,
        }),
      });
      if (!response.ok) throw new Error(`Research task ${action} failed: ${response.status}`);
      const payload = await response.json();
      researchTasks = [payload.task, ...researchTasks.filter((item) => item.id !== task.id)]
        .filter(Boolean)
        .slice(0, 30);
      researchEvidence = [payload.evidence, ...researchEvidence]
        .filter(Boolean)
        .slice(0, 50);
      contributionStatus = `Research task ${payload.task?.status || action}`;
      await loadContributions(selectedStory.id);
    } catch (error) {
      contributionStatus = error?.message || `Research task ${action} failed`;
    } finally {
      reconciliationBusyId = '';
    }
  }

  async function reviewResearchEvidence(evidence, decision) {
    if (!authenticated || !evidence?.id) return;
    reconciliationBusyId = `${evidence.id}:${decision}`;
    contributionStatus = '';
    try {
      const response = await fetch('/api/global-wire/research-evidence', {
        method: 'POST',
        credentials: 'include',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          evidence_id: evidence.id,
          decision,
          note: `global-wire-app:${decision}`,
        }),
      });
      if (!response.ok) throw new Error(`Research evidence ${decision} failed: ${response.status}`);
      const payload = await response.json();
      researchDecisions = [payload.decision, ...researchDecisions]
        .filter(Boolean)
        .slice(0, 50);
      if (payload.candidate?.id) {
        graphUpdateCandidates = [payload.candidate, ...graphUpdateCandidates.filter((item) => item.id !== payload.candidate.id)]
          .filter(Boolean)
          .slice(0, 20);
      }
      contributionStatus = `Research evidence ${payload.decision?.decision || decision}`;
      await loadContributions(selectedStory.id);
    } catch (error) {
      contributionStatus = error?.message || `Research evidence ${decision} failed`;
    } finally {
      reconciliationBusyId = '';
    }
  }

  async function packagePublicationUpdate(decision) {
    if (!authenticated || !decision?.id) return;
    reconciliationBusyId = `${decision.id}:publication-update`;
    contributionStatus = '';
    try {
      const response = await fetch('/api/global-wire/publication-updates', {
        method: 'POST',
        credentials: 'include',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          research_decision_id: decision.id,
        }),
      });
      if (!response.ok) throw new Error(`Publication update package failed: ${response.status}`);
      const payload = await response.json();
      publicationUpdates = [payload.update, ...publicationUpdates]
        .filter(Boolean)
        .slice(0, 30);
      contributionStatus = `Publication update ${payload.update?.status || 'packaged'}`;
      await loadContributions(selectedStory.id);
    } catch (error) {
      contributionStatus = error?.message || 'Publication update package failed';
    } finally {
      reconciliationBusyId = '';
    }
  }

  async function createPublicationArtifact(update) {
    if (!authenticated || !update?.id) return;
    reconciliationBusyId = `${update.id}:publication-artifact`;
    contributionStatus = '';
    try {
      const response = await fetch('/api/global-wire/publication-artifacts', {
        method: 'POST',
        credentials: 'include',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          update_id: update.id,
          channel: 'newsletter',
        }),
      });
      if (!response.ok) throw new Error(`Publication artifact failed: ${response.status}`);
      const payload = await response.json();
      publicationArtifacts = [payload.artifact, ...publicationArtifacts]
        .filter(Boolean)
        .slice(0, 30);
      contributionStatus = `Publication artifact ${payload.artifact?.status || 'created'}`;
      await loadContributions(selectedStory.id);
    } catch (error) {
      contributionStatus = error?.message || 'Publication artifact failed';
    } finally {
      reconciliationBusyId = '';
    }
  }

  async function reviewPublicationArtifact(item, decision) {
    const artifact = item?.artifact || item;
    if (!authenticated || !artifact?.id) return;
    reconciliationBusyId = `${artifact.id}:publication-${decision}`;
    contributionStatus = '';
    try {
      const response = await fetch('/api/global-wire/publication-artifact-reviews', {
        method: 'POST',
        credentials: 'include',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          artifact_id: artifact.id,
          decision,
          note: `global-wire-app:${decision}`,
        }),
      });
      if (!response.ok) throw new Error(`Publication artifact ${decision} failed: ${response.status}`);
      const payload = await response.json();
      publicationArtifacts = [payload.artifact, ...publicationArtifacts.filter((entry) => entry.id !== payload.artifact?.id)]
        .filter(Boolean)
        .slice(0, 30);
      publicationFeedItems = publicationFeedItems.map((feedItem) => {
        if (feedItem.artifact?.id !== payload.artifact?.id) return feedItem;
        return {
          ...feedItem,
          artifact: payload.artifact,
          status: payload.artifact.status,
        };
      });
      contributionStatus = `Publication artifact ${payload.artifact?.status || decision}`;
      await loadPublicationFeed(selectedStory.id);
    } catch (error) {
      contributionStatus = error?.message || `Publication artifact ${decision} failed`;
    } finally {
      reconciliationBusyId = '';
    }
  }

  async function createPublicationDelivery(item) {
    const artifact = item?.artifact || item;
    if (!authenticated || !artifact?.id) return;
    reconciliationBusyId = `${artifact.id}:publication-delivery`;
    contributionStatus = '';
    try {
      const response = await fetch('/api/global-wire/publication-deliveries', {
        method: 'POST',
        credentials: 'include',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          artifact_id: artifact.id,
          channel: artifact.channel || 'newsletter',
        }),
      });
      if (!response.ok) throw new Error(`Publication delivery failed: ${response.status}`);
      const payload = await response.json();
      publicationDeliveries = [payload.delivery, ...publicationDeliveries.filter((delivery) => delivery.id !== payload.delivery?.id)]
        .filter(Boolean)
        .slice(0, 30);
      contributionStatus = `Publication delivery ${payload.delivery?.status || 'created'}`;
      await loadContributions(selectedStory.id);
    } catch (error) {
      contributionStatus = error?.message || 'Publication delivery failed';
    } finally {
      reconciliationBusyId = '';
    }
  }

  async function openPublicationDeliveryDetail(delivery) {
    if (!authenticated || !delivery?.id) return;
    reconciliationBusyId = `${delivery.id}:publication-delivery-detail`;
    try {
      const response = await fetch(`/api/global-wire/publication-deliveries/${encodeURIComponent(delivery.id)}`, {
        credentials: 'include',
      });
      if (!response.ok) throw new Error(`Publication delivery detail failed: ${response.status}`);
      publicationDeliveryDetail = await response.json();
    } catch (error) {
      contributionStatus = error?.message || 'Publication delivery detail failed';
    } finally {
      reconciliationBusyId = '';
    }
  }

  async function createAutoradioScript(item) {
    const artifact = item?.artifact || item;
    if (!authenticated || !artifact?.id) return;
    reconciliationBusyId = `${artifact.id}:autoradio-script`;
    contributionStatus = '';
    try {
      const response = await fetch('/api/global-wire/autoradio-scripts', {
        method: 'POST',
        credentials: 'include',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          artifact_id: artifact.id,
        }),
      });
      if (!response.ok) throw new Error(`Autoradio script failed: ${response.status}`);
      const payload = await response.json();
      autoradioScripts = [payload.script, ...autoradioScripts.filter((script) => script.id !== payload.script?.id)]
        .filter(Boolean)
        .slice(0, 30);
      contributionStatus = `Autoradio script ${payload.script?.status || 'created'}`;
      await loadContributions(selectedStory.id);
    } catch (error) {
      contributionStatus = error?.message || 'Autoradio script failed';
    } finally {
      reconciliationBusyId = '';
    }
  }

  async function createDeliveryExport(delivery) {
    if (!authenticated || !delivery?.id) return;
    reconciliationBusyId = `${delivery.id}:delivery-export`;
    contributionStatus = '';
    try {
      const response = await fetch('/api/global-wire/publication-delivery-exports', {
        method: 'POST',
        credentials: 'include',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          delivery_id: delivery.id,
          format: 'md',
        }),
      });
      if (!response.ok) throw new Error(`Delivery export failed: ${response.status}`);
      const payload = await response.json();
      deliveryExports = [payload.export, ...deliveryExports.filter((item) => item.id !== payload.export?.id)]
        .filter(Boolean)
        .slice(0, 30);
      contributionStatus = `Delivery export ${payload.export?.status || 'created'}`;
      await loadContributions(selectedStory.id);
    } catch (error) {
      contributionStatus = error?.message || 'Delivery export failed';
    } finally {
      reconciliationBusyId = '';
    }
  }

  async function createPublicLink(deliveryExport) {
    if (!authenticated || !deliveryExport?.id) return;
    reconciliationBusyId = `${deliveryExport.id}:public-link`;
    contributionStatus = '';
    try {
      const response = await fetch('/api/global-wire/publication-public-links', {
        method: 'POST',
        credentials: 'include',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          export_id: deliveryExport.id,
        }),
      });
      if (!response.ok) throw new Error(`Public link failed: ${response.status}`);
      const payload = await response.json();
      publicLinks = [payload.public_link, ...publicLinks.filter((item) => item.id !== payload.public_link?.id)]
        .filter(Boolean)
        .slice(0, 30);
      contributionStatus = `Public link ${payload.public_link?.status || 'created'}`;
      await loadContributions(selectedStory.id);
    } catch (error) {
      contributionStatus = error?.message || 'Public link failed';
    } finally {
      reconciliationBusyId = '';
    }
  }

  async function createNewsletterIssue(publicLink) {
    if (!authenticated || !publicLink?.id) return;
    reconciliationBusyId = `${publicLink.id}:newsletter-issue`;
    contributionStatus = '';
    try {
      let subscribers = newsletterSubscribers.filter((subscriber) => subscriber.status === 'active');
      if (!subscribers.length) {
        const subscriberResponse = await fetch('/api/global-wire/newsletter-subscribers', {
          method: 'POST',
          credentials: 'include',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            email: 'global-wire-subscriber@example.com',
            label: 'Global Wire staging subscriber',
          }),
        });
        if (!subscriberResponse.ok) throw new Error(`Newsletter subscriber failed: ${subscriberResponse.status}`);
        const subscriberPayload = await subscriberResponse.json();
        subscribers = [subscriberPayload.subscriber].filter(Boolean);
        newsletterSubscribers = [subscriberPayload.subscriber, ...newsletterSubscribers.filter((item) => item.id !== subscriberPayload.subscriber?.id)]
          .filter(Boolean)
          .slice(0, 30);
      }
      const response = await fetch('/api/global-wire/newsletter-issues', {
        method: 'POST',
        credentials: 'include',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          public_link_ids: [publicLink.id],
          story_id: publicLink.story_id,
        }),
      });
      if (!response.ok) throw new Error(`Newsletter issue failed: ${response.status}`);
      const payload = await response.json();
      newsletterIssues = [payload.issue, ...newsletterIssues.filter((item) => item.id !== payload.issue?.id)]
        .filter(Boolean)
        .slice(0, 30);
      newsletterDeliveries = [...(payload.deliveries || []), ...newsletterDeliveries]
        .filter(Boolean)
        .slice(0, 50);
      noteNewsletterIssueInDossiers(payload.issue, payload.deliveries || []);
      newsletterSubscribers = [...(payload.subscribers || subscribers), ...newsletterSubscribers]
        .filter(Boolean)
        .filter((item, index, list) => list.findIndex((candidate) => candidate.id === item.id) === index)
        .slice(0, 30);
      contributionStatus = `Newsletter issue ${payload.issue?.status || 'created'}`;
      await loadContributions(selectedStory.id);
    } catch (error) {
      contributionStatus = error?.message || 'Newsletter issue failed';
    } finally {
      reconciliationBusyId = '';
    }
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
            on:click={() => runFetchCycle(false)}
            disabled={fetchCycleBusy}
            data-global-wire-fetch-cycle
          >
            {fetchCycleBusy ? 'Running...' : 'Run fetch cycle'}
          </button>
          <button
            type="button"
            class="source-search-button"
            on:click={() => runFetchCycle(true)}
            disabled={fetchCycleBusy}
            data-global-wire-scheduler-cycle
          >
            {fetchCycleBusy ? 'Running...' : 'Run source schedule'}
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
          {#if fetchCycles.length || sourceRegistryEntries.length || sourceSchedulerRuns.length}
            <div class="source-search-results" data-global-wire-fetch-cycle-runs>
              {#each sourceSchedulerRuns.slice(0, 2) as run}
                <article data-global-wire-source-scheduler-run>
                  <strong>{run.status}</strong>
                  <small>{run.trigger} · {(run.standing_policies || []).join(', ')}</small>
                  <span>{run.message}</span>
                </article>
              {/each}
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
                  <small>{entry.status} · {entry.story_id} · {entry.source_standing_policy || 'standing review'}</small>
                  <span>{entry.query}</span>
                  {#if entry.source_standing_rationale}
                    <small data-global-wire-source-standing-policy>{entry.source_standing_rationale}</small>
                  {/if}
                  {#if entry.cadence_seconds}
                    <small data-global-wire-source-schedule-cadence>
                      cadence {entry.cadence_seconds}s · next due {entry.next_due_at || 'pending'}
                    </small>
                  {/if}
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
        {#if publicationFeedItems.length}
          <div class="publication-feed" data-global-wire-publication-feed>
            <div class="section-title">
              <h4>Publication Feed</h4>
              <span>{publicationFeedStatus || 'ready'} · newsletter</span>
            </div>
            {#each publicationFeedItems.slice(0, 3) as item}
              {@const delivery = publicationDeliveryForArtifact(item.artifact)}
              {@const autoradioScript = autoradioScriptForArtifact(item.artifact)}
              {@const deliveryExport = deliveryExportForDelivery(delivery)}
              {@const publicLink = publicLinkForExport(deliveryExport)}
              {@const newsletterIssue = newsletterIssueForPublicLink(publicLink)}
              {@const issueDeliveries = newsletterDeliveriesForIssue(newsletterIssue)}
              <article
                data-global-wire-publication-feed-item
                data-global-wire-publication-feed-artifact-id={item.artifact.id}
              >
                <strong>{item.artifact.title}</strong>
                <small>{item.status} · {item.artifact.channel} · {item.story.headline}</small>
                <span>{item.artifact.body}</span>
                <small data-global-wire-publication-feed-provenance>
                  citations: {item.citation_count} · rollback refs: {item.rollback_count} · source {item.source_item?.title || item.artifact.source_content_id || 'story manifest'}
                </small>
                {#if delivery}
                  <small
                    data-global-wire-publication-delivery
                    data-global-wire-publication-delivery-id={delivery.id}
                  >
                    {delivery.status}: {delivery.channel} · {delivery.delivery_ref}
                  </small>
                  <small data-global-wire-publication-delivery-provenance>
                    delivery citations: {delivery.citation_count} · rollback refs: {delivery.rollback_count}
                  </small>
                  {#if autoradioScript}
                    <div
                      class="autoradio-script"
                      data-global-wire-autoradio-script
                      data-global-wire-autoradio-script-id={autoradioScript.id}
                    >
                      <strong>{autoradioScript.status}: {autoradioScript.title}</strong>
                      <span>{autoradioScript.script_body}</span>
                      <small data-global-wire-autoradio-script-provenance>
                        script citations: {autoradioScript.citation_count} · rollback refs: {autoradioScript.rollback_count}
                      </small>
                    </div>
                  {/if}
                  {#if deliveryExport}
                    <div
                      class="delivery-export"
                      data-global-wire-delivery-export
                      data-global-wire-delivery-export-id={deliveryExport.id}
                    >
                      <strong>{deliveryExport.status}: {deliveryExport.title}</strong>
                      <span>{deliveryExport.export_body}</span>
                      <small data-global-wire-delivery-export-provenance>
                        export format: {deliveryExport.format} · citations: {deliveryExport.citation_count} · rollback refs: {deliveryExport.rollback_count}
                      </small>
                    </div>
                    {#if publicLink}
                      <small
                        data-global-wire-public-link
                        data-global-wire-public-link-id={publicLink.id}
                      >
                        {publicLink.status}: {publicLink.route_path}
                      </small>
                      {#if newsletterIssue}
                        <div
                          class="delivery-export"
                          data-global-wire-newsletter-issue
                          data-global-wire-newsletter-issue-id={newsletterIssue.id}
                        >
                          <strong>{newsletterIssue.status}: {newsletterIssue.subject}</strong>
                          <span>{newsletterIssue.issue_body}</span>
                          <small data-global-wire-newsletter-issue-provenance>
                            subscribers: {newsletterIssue.subscriber_count} · citations: {newsletterIssue.citation_count} · rollback refs: {newsletterIssue.rollback_count}
                          </small>
                          {#each issueDeliveries.slice(0, 3) as issueDelivery}
                            <small
                              data-global-wire-newsletter-delivery
                              data-global-wire-newsletter-delivery-id={issueDelivery.id}
                            >
                              {issueDelivery.status}: {issueDelivery.delivery_ref}
                            </small>
                          {/each}
                        </div>
                      {/if}
                    {/if}
                  {/if}
                  <div class="publication-feed-actions">
                    <button
                      type="button"
                      on:click={() => openPublicationDeliveryDetail(delivery)}
                      disabled={reconciliationBusyId === `${delivery.id}:publication-delivery-detail`}
                      data-global-wire-open-publication-delivery
                    >
                      Inspect
                    </button>
                    {#if !autoradioScript}
                      <button
                        type="button"
                        on:click={() => createAutoradioScript(item)}
                        disabled={reconciliationBusyId === `${item.artifact.id}:autoradio-script`}
                        data-global-wire-create-autoradio-script
                      >
                        Script
                      </button>
                    {/if}
                    {#if !deliveryExport}
                      <button
                        type="button"
                        on:click={() => createDeliveryExport(delivery)}
                        disabled={reconciliationBusyId === `${delivery.id}:delivery-export`}
                        data-global-wire-create-delivery-export
                      >
                        Export
                      </button>
                    {/if}
                    {#if deliveryExport && !publicLink}
                      <button
                        type="button"
                        on:click={() => createPublicLink(deliveryExport)}
                        disabled={reconciliationBusyId === `${deliveryExport.id}:public-link`}
                        data-global-wire-create-public-link
                      >
                        Publish
                      </button>
                    {/if}
                    {#if publicLink && !newsletterIssue}
                      <button
                        type="button"
                        on:click={() => createNewsletterIssue(publicLink)}
                        disabled={reconciliationBusyId === `${publicLink.id}:newsletter-issue`}
                        data-global-wire-create-newsletter-issue
                      >
                        Issue
                      </button>
                    {/if}
                  </div>
                {:else if authenticated && item.status === 'publication-approved'}
                  <div class="publication-feed-actions">
                    <button
                      type="button"
                      on:click={() => createPublicationDelivery(item)}
                      disabled={reconciliationBusyId === `${item.artifact.id}:publication-delivery`}
                      data-global-wire-create-publication-delivery
                    >
                      Deliver
                    </button>
                    {#if autoradioScript}
                      <small data-global-wire-autoradio-script>
                        {autoradioScript.status}: {autoradioScript.title}
                      </small>
                    {:else}
                      <button
                        type="button"
                        on:click={() => createAutoradioScript(item)}
                        disabled={reconciliationBusyId === `${item.artifact.id}:autoradio-script`}
                        data-global-wire-create-autoradio-script
                      >
                        Script
                      </button>
                    {/if}
                  </div>
                {:else if authenticated && item.status === 'publication-review-ready'}
                  <div class="publication-feed-actions">
                    <button
                      type="button"
                      on:click={() => reviewPublicationArtifact(item, 'approve')}
                      disabled={reconciliationBusyId === `${item.artifact.id}:publication-approve`}
                      data-global-wire-approve-publication-artifact
                    >
                      Approve
                    </button>
                    <button
                      type="button"
                      on:click={() => reviewPublicationArtifact(item, 'reject')}
                      disabled={reconciliationBusyId === `${item.artifact.id}:publication-reject`}
                      data-global-wire-reject-publication-artifact
                    >
                      Reject
                    </button>
                  </div>
                {/if}
              </article>
            {/each}
          </div>
        {/if}
        {#if publicationDeliveryDetail}
          <div class="publication-delivery-detail" data-global-wire-publication-delivery-detail>
            <div class="section-title">
              <h4>{publicationDeliveryDetail.artifact.title}</h4>
              <span>{publicationDeliveryDetail.delivery.status} · {publicationDeliveryDetail.delivery.channel}</span>
            </div>
            <small data-global-wire-publication-delivery-detail-ref>
              {publicationDeliveryDetail.delivery.delivery_ref}
            </small>
            <strong>{publicationDeliveryDetail.story.headline}</strong>
            <p>{publicationDeliveryDetail.artifact.body}</p>
            <small data-global-wire-publication-delivery-detail-source>
              source {publicationDeliveryDetail.source_item?.title || publicationDeliveryDetail.artifact.source_content_id || 'story manifest'}
            </small>
            <small data-global-wire-publication-delivery-detail-citations>
              citations: {(publicationDeliveryDetail.delivery.citation_refs || []).slice(0, 4).join(' · ')}
            </small>
            <small data-global-wire-publication-delivery-detail-rollback>
              rollback refs: {(publicationDeliveryDetail.delivery.rollback_refs || []).slice(0, 4).join(' · ')}
            </small>
          </div>
        {/if}
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
                            {@const extractions = claimExtractionArtifacts(claim)}
                            <article data-global-wire-claim-record data-global-wire-claim-id={claim.id}>
                              <strong>{claim.claim_kind}</strong>
                              <small>{claim.uncertainty_state} · {claim.dispute_state} · {claim.status}</small>
                              <span>{claim.claim_text}</span>
                              <em>{claim.evidence_gap}</em>
                              {#each extractions.slice(0, 2) as extraction}
                                <small
                                  data-global-wire-extraction-artifact
                                  data-global-wire-extraction-id={extraction.id}
                                >
                                  {extraction.status}: entities {(extraction.entities || []).length} · events {(extraction.events || []).length}
                                </small>
                                {#if (extraction.timeline || []).length}
                                  <small data-global-wire-extraction-timeline>
                                    {(extraction.timeline || [])[0]}
                                  </small>
                                {/if}
                              {/each}
                              {#each tasks.slice(0, 2) as task}
                                {@const evidencePackets = taskEvidence(task)}
                                <div
                                  class="research-task"
                                  data-global-wire-research-task
                                  data-global-wire-research-task-id={task.id}
                                >
                                  <small>{task.task_kind}: {task.status} · {task.priority}</small>
                                  <div class="research-task-actions">
                                    <button
                                      type="button"
                                      on:click={() => updateResearchTask(task, 'assign')}
                                      disabled={reconciliationBusyId === `${task.id}:assign`}
                                      data-global-wire-assign-research-task
                                    >
                                      Assign
                                    </button>
                                    <button
                                      type="button"
                                      on:click={() => updateResearchTask(task, 'complete')}
                                      disabled={reconciliationBusyId === `${task.id}:complete`}
                                      data-global-wire-complete-research-task
                                    >
                                      Complete
                                    </button>
                                    <button
                                      type="button"
                                      on:click={() => updateResearchTask(task, 'block')}
                                      disabled={reconciliationBusyId === `${task.id}:block`}
                                      data-global-wire-block-research-task
                                    >
                                      Block
                                    </button>
                                  </div>
                                  {#each evidencePackets.slice(0, 2) as evidence}
                                    {@const handoff = evidenceDecision(evidence)}
                                    {@const publicationUpdate = publicationUpdateForDecision(handoff)}
                                    <small
                                      data-global-wire-research-task-evidence
                                      data-global-wire-research-evidence-id={evidence.id}
                                    >
                                      {evidence.status}: {evidence.summary}
                                    </small>
                                    {#if handoff}
                                      <small data-global-wire-research-evidence-decision>
                                        {handoff.decision}: {handoff.result_state}
                                      </small>
                                      {#if publicationUpdate}
                                        {@const publicationArtifact = publicationArtifactForUpdate(publicationUpdate)}
                                        <small
                                          data-global-wire-publication-update
                                          data-global-wire-publication-update-id={publicationUpdate.id}
                                        >
                                          {publicationUpdate.status}: {publicationUpdate.summary}
                                        </small>
                                        <small data-global-wire-publication-rollback>
                                          rollback refs: {(publicationUpdate.rollback_refs || []).length}
                                        </small>
                                        <small data-global-wire-publication-extraction-refs>
                                          extraction refs: {(publicationUpdate.extraction_ids || []).length}
                                        </small>
                                        {#if publicationArtifact}
                                          <small
                                            data-global-wire-publication-artifact
                                            data-global-wire-publication-artifact-id={publicationArtifact.id}
                                          >
                                            {publicationArtifact.status}: {publicationArtifact.title}
                                          </small>
                                          <small data-global-wire-publication-artifact-citations>
                                            citations: {(publicationArtifact.citation_refs || []).length} / scheduler refs: {(publicationArtifact.scheduler_run_ids || []).length}
                                          </small>
                                        {:else}
                                          <div class="research-task-actions">
                                            <button
                                              type="button"
                                              on:click={() => createPublicationArtifact(publicationUpdate)}
                                              disabled={reconciliationBusyId === `${publicationUpdate.id}:publication-artifact`}
                                              data-global-wire-create-publication-artifact
                                            >
                                              Build publication artifact
                                            </button>
                                          </div>
                                        {/if}
                                      {:else if handoff.result_state === 'ready-for-platform-review'}
                                        <div class="research-task-actions">
                                          <button
                                            type="button"
                                            on:click={() => packagePublicationUpdate(handoff)}
                                            disabled={reconciliationBusyId === `${handoff.id}:publication-update`}
                                            data-global-wire-package-publication-update
                                          >
                                            Package update
                                          </button>
                                        </div>
                                      {/if}
                                    {:else}
                                      <div class="research-task-actions">
                                        <button
                                          type="button"
                                          on:click={() => reviewResearchEvidence(evidence, 'accept')}
                                          disabled={reconciliationBusyId === `${evidence.id}:accept`}
                                          data-global-wire-accept-research-evidence
                                        >
                                          Accept
                                        </button>
                                        <button
                                          type="button"
                                          on:click={() => reviewResearchEvidence(evidence, 'block')}
                                          disabled={reconciliationBusyId === `${evidence.id}:block`}
                                          data-global-wire-block-research-evidence
                                        >
                                          Block
                                        </button>
                                      </div>
                                    {/if}
                                  {/each}
                                </div>
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
          {@const dossier = selectedSourceDossier()}
          {#if dossier}
            <article
              class="source-dossier"
              data-global-wire-source-dossier
              data-global-wire-source-dossier-id={dossier.id}
            >
              <div>
                <strong>{dossier.review_state}</strong>
                <small>{dossier.headline}</small>
              </div>
              <div class="dossier-grid">
                {#each dossier.manifest_tiers || [] as tier}
                  <small data-global-wire-source-dossier-tier data-global-wire-source-tier={tier.tier}>
                    {tier.tier}: {tier.count}
                  </small>
                {/each}
              </div>
              <small data-global-wire-source-dossier-claims>
                claims: {(dossier.claim_dossiers || []).length} · extractions: {(dossier.extraction_ids || []).length} · tasks: {(dossier.research_task_ids || []).length}
              </small>
              <small data-global-wire-source-dossier-publication>
                publications: {(dossier.publication_refs?.artifact_ids || []).length} · deliveries: {(dossier.publication_refs?.delivery_ids || []).length} · newsletter issues: {(dossier.publication_refs?.newsletter_issue_ids || []).length}
              </small>
              <small data-global-wire-source-dossier-provenance>
                citations: {(dossier.publication_refs?.citation_refs || []).length} · rollback refs: {(dossier.publication_refs?.rollback_refs || []).length} · missing: {(dossier.missing_fields || []).join(', ') || 'none'}
              </small>
              {#if (dossier.entity_terms || []).length || (dossier.event_terms || []).length}
                <small data-global-wire-source-dossier-overlay>
                  entities: {(dossier.entity_terms || []).slice(0, 3).join(', ')} · events: {(dossier.event_terms || []).slice(0, 2).join(', ')}
                </small>
              {/if}
              {#if (dossier.timeline || []).length}
                <small data-global-wire-source-dossier-timeline>
                  {(dossier.timeline || [])[0]}
                </small>
              {/if}
            </article>
          {/if}
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

  .publication-feed {
    display: grid;
    gap: 0.4rem;
    padding: 0.55rem;
    border: 1px solid var(--choir-border);
    border-radius: 8px;
    background: var(--choir-surface-pane);
  }

  .publication-feed article {
    display: grid;
    gap: 0.2rem;
    padding: 0.45rem;
    border: 1px solid var(--choir-border);
    border-left: 3px solid var(--choir-border-strong);
    border-radius: 8px;
    background: var(--choir-surface-card);
    font-size: 0.82rem;
    line-height: 1.3;
  }

  .publication-feed strong,
  .publication-feed span {
    overflow-wrap: anywhere;
  }

  .publication-feed span {
    display: -webkit-box;
    -webkit-line-clamp: 5;
    -webkit-box-orient: vertical;
    overflow: hidden;
  }

  .publication-feed-actions {
    display: flex;
    flex-wrap: wrap;
    gap: 0.35rem;
  }

  .publication-feed-actions button {
    flex: 1 1 5.25rem;
    min-height: 2rem;
    padding: 0.32rem 0.5rem;
    border: 1px solid var(--choir-border);
    border-radius: 8px;
    background: var(--choir-surface-control);
    color: var(--choir-text-primary);
    font: inherit;
    font-size: 0.72rem;
    font-weight: 720;
    cursor: pointer;
  }

  .autoradio-script {
    display: grid;
    gap: 0.22rem;
    padding: 0.38rem;
    border: 1px solid var(--choir-border);
    border-radius: 8px;
    background: var(--choir-surface-pane);
  }

  .autoradio-script span {
    -webkit-line-clamp: 6;
  }

  .delivery-export {
    display: grid;
    gap: 0.22rem;
    padding: 0.38rem;
    border: 1px solid var(--choir-border);
    border-radius: 8px;
    background: var(--choir-surface-pane);
  }

  .delivery-export span {
    -webkit-line-clamp: 7;
  }

  .publication-delivery-detail {
    display: grid;
    gap: 0.4rem;
    padding: 0.55rem;
    border: 1px solid var(--choir-border);
    border-left: 3px solid var(--choir-border-strong);
    border-radius: 8px;
    background: var(--choir-surface-pane);
    font-size: 0.82rem;
    line-height: 1.32;
  }

  .publication-delivery-detail p {
    display: -webkit-box;
    -webkit-line-clamp: 7;
    -webkit-box-orient: vertical;
    overflow: hidden;
    line-height: 1.35;
  }

  .publication-delivery-detail strong,
  .publication-delivery-detail p,
  .publication-delivery-detail small {
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
  .graph-candidate,
  .source-dossier {
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

  .source-dossier {
    gap: 0.35rem;
    border-left: 3px solid var(--choir-border-strong);
    font-size: 0.82rem;
    line-height: 1.3;
  }

  .dossier-grid {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 0.25rem;
  }

  .source-dossier small {
    overflow-wrap: anywhere;
  }

  .reconciliation-source strong,
  .graph-candidate strong,
  .graph-candidate span,
  .source-dossier strong {
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

  .research-task {
    display: grid;
    gap: 0.3rem;
    min-width: 0;
  }

  .research-task-actions {
    display: flex;
    flex-wrap: wrap;
    gap: 0.35rem;
  }

  .research-task-actions button {
    flex: 1 1 4.75rem;
    min-height: 2rem;
    padding: 0.32rem 0.5rem;
    font-size: 0.72rem;
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
