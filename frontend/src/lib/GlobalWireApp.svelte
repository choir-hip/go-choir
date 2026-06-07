<script>
  import { createEventDispatcher } from 'svelte';

  export let currentUser = null;
  export let authenticated = false;

  const dispatch = createEventDispatcher();

  const styleSources = [
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

  const stories = [
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

  let selectedStoryId = stories[0].id;
  let selectedStyleId = styleSources[0].id;
  let contributionKind = 'source';
  let contributionText = '';
  let contributionStatus = '';
  let contributions = [];

  $: selectedStory = stories.find((story) => story.id === selectedStoryId) || stories[0];
  $: selectedStyle = styleSources.find((style) => style.id === selectedStyleId) || styleSources[0];
  $: projectionText = selectedStory.projections[selectedStyle.id] || selectedStory.projections['wire-style'];
  $: allSources = [
    ...selectedStory.manifest.lead,
    ...selectedStory.manifest.supporting,
    ...selectedStory.manifest.contrary,
    ...selectedStory.manifest.context,
  ];

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

  function launchVText({ title, content, createdFrom, sourcePath = '' }) {
    dispatch('launchapp', {
      appId: 'vtext',
      appName: 'VText',
      icon: '📝',
      appContext: {
        windowTitle: title,
        initialContent: content,
        createInitialVersion: true,
        createdFrom,
        sourcePath,
        appHint: 'global-wire',
        allowMultiple: true,
      },
    });
  }

  function openStoryVText() {
    launchVText({
      title: selectedStory.headline,
      content: storyVTextContent(),
      createdFrom: 'global_wire_story_projection',
      sourcePath: `global-wire/${selectedStory.id}.story.vtext`,
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
    });
  }

  function submitContribution() {
    const text = contributionText.trim();
    const record = {
      id: `contribution-${Date.now()}`,
      kind: contributionKind,
      storyId: selectedStory.id,
      headline: selectedStory.headline,
      text: text || 'Draft contribution awaiting detail.',
      owner: currentUser?.email || 'public-preview',
    };
    contributions = [record, ...contributions].slice(0, 6);
    contributionStatus = authenticated
      ? 'Contribution queued as a user-owned VText draft'
      : 'Local contribution preview - sign in to save';
    launchVText({
      title: `Contribution: ${selectedStory.headline}`,
      content: contributionContent(),
      createdFrom: 'global_wire_user_contribution',
      sourcePath: `contributions/${selectedStory.id}-${record.kind}.vtext`,
    });
    contributionText = '';
  }
</script>

<section class="global-wire" data-global-wire-app>
  <header class="wire-header">
    <div>
      <p class="eyebrow">Global Wire</p>
      <h2>StoryGraph desk</h2>
    </div>
    <div class="wire-state" data-global-wire-state>
      <span>{authenticated ? 'owner computer' : 'public preview'}</span>
      <strong>{stories.length} story nodes</strong>
    </div>
  </header>

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
          <button type="button" on:click={openStoryVText} data-global-wire-open-vtext>Open VText</button>
          <button type="button" on:click={forkStory} data-global-wire-fork-story>Fork/Edit</button>
        </div>
      </div>

      <p class="dek">{selectedStory.dek}</p>

      <div class="style-switcher" data-global-wire-style-switcher>
        <div class="section-title">
          <h3>Style.vtext Projection</h3>
          <button type="button" on:click={openStyleVText} data-global-wire-open-style>Open style source</button>
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
              <p><strong>{item.kind.replaceAll('-', ' ')}</strong> · {item.text}</p>
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
  .submit-contribution {
    padding: 0.45rem 0.7rem;
    font-weight: 720;
  }

  .dek {
    color: var(--choir-text-muted);
    line-height: 1.45;
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

  .submit-contribution {
    background: var(--choir-accent);
    color: var(--choir-on-accent);
  }

  .contribution-status {
    color: var(--choir-text-accent);
    font-size: 0.85rem;
  }

  .contribution-list {
    display: grid;
    gap: 0.4rem;
    max-height: 7rem;
    overflow: auto;
  }

  .contribution-list p {
    padding: 0.45rem;
    border: 1px solid var(--choir-border);
    border-radius: 8px;
    background: var(--choir-surface-card);
    font-size: 0.82rem;
    line-height: 1.3;
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
