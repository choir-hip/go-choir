<script>
  import { createEventDispatcher, onDestroy, onMount } from 'svelte';
  import { AuthRequiredError } from './auth.js';
  import {
    getTrajectoryMomentDetail,
    getTrajectorySnapshot,
    listTrajectories,
    openTrajectoryEventStream,
    startContinuation,
    synthesizeContinuation,
  } from './trace.js';

  const dispatch = createEventDispatcher();

  let loadingIndex = true;
  let snapshotLoading = false;
  let detailLoading = false;
  let error = '';
  let trajectories = [];
  let snapshot = null;
  let selectedTrajectoryId = '';
  let selectedAgentId = '';
  let selectedMomentId = '';
  let momentDetails = {};
  let stream = null;
  let streamStatus = 'idle';
  let lastStreamSeq = 0;
  let refreshTimer = null;
  let continuationBusy = false;
  let continuationError = '';
  let selectedContinuation = null;

  function parseDate(value) {
    const time = value ? new Date(value).getTime() : 0;
    return Number.isFinite(time) ? time : 0;
  }

  function excerpt(text, max = 88) {
    const normalized = (text || '').replace(/\s+/g, ' ').trim();
    if (!normalized) return 'Untitled trajectory';
    if (normalized.length <= max) return normalized;
    return `${normalized.slice(0, max - 1)}…`;
  }

  function formatTime(value) {
    if (!value) return '';
    return new Date(value).toLocaleTimeString([], {
      hour: 'numeric',
      minute: '2-digit',
      second: '2-digit',
    });
  }

  function formatPayload(payload) {
    if (!payload) return '';
    if (typeof payload === 'string') return payload;
    try {
      return JSON.stringify(payload, null, 2);
    } catch {
      return String(payload);
    }
  }

  function stateTone(state) {
    if (state === 'completed') return 'success';
    if (state === 'running' || state === 'pending' || state === 'blocked') return 'active';
    if (state === 'failed' || state === 'cancelled') return 'error';
    return 'neutral';
  }

  function streamTone(status) {
    if (status === 'live') return 'success';
    if (status === 'connecting' || status === 'catching-up' || status === 'reconnecting') return 'active';
    return 'neutral';
  }

  function agentCaption(agent) {
    const parts = [agent.role, agent.profile].filter(Boolean);
    return parts.length > 0 ? parts.join(' · ') : excerpt(agent.agent_id, 20);
  }

  function traceMetrics(trajectory) {
    return [
      { label: 'agents', value: trajectory?.agent_count || 0 },
      { label: 'delegations', value: trajectory?.delegation_count || 0 },
      { label: 'moments', value: trajectory?.moment_count || 0 },
      { label: 'messages', value: trajectory?.message_count || 0 },
      { label: 'findings', value: trajectory?.finding_count || 0 },
      { label: 'searches', value: trajectory?.search_attempt_count || 0 },
    ];
  }

  function runGeometryStats(items) {
    const stats = { compactions: 0, continuations: 0, retries: 0, promotions: 0 };
    for (const moment of items || []) {
      const kind = String(moment?.kind || '');
      if (kind.startsWith('loop.compaction')) stats.compactions += 1;
      if (kind.startsWith('loop.continuation')) stats.continuations += 1;
      if (kind === 'loop.retry') stats.retries += 1;
      if (kind.startsWith('promotion.candidate')) stats.promotions += 1;
    }
    return { ...stats, total: stats.compactions + stats.continuations + stats.retries + stats.promotions };
  }

  function runGeometryMetrics(stats) {
    return [
      { label: 'compactions', value: stats.compactions },
      { label: 'continuations', value: stats.continuations },
      { label: 'retries', value: stats.retries },
      { label: 'promotions', value: stats.promotions },
    ].filter((metric) => metric.value > 0);
  }

  function hasArtifacts(artifacts) {
    return !!(artifacts?.run_memory || artifacts?.continuation || artifacts?.promotion_candidate);
  }

  function latestRunId(items) {
    for (let index = (items || []).length - 1; index >= 0; index -= 1) {
      const runId = (items[index]?.loop_id || '').trim();
      if (runId) return runId;
    }
    return '';
  }

  function canSelectContinuation(item, runId) {
    return !!runId && (item?.state === 'completed' || item?.state === 'blocked');
  }

  function buildGraphLayout(agents, edges) {
    if (!agents || agents.length === 0) {
      return { nodes: [], edges: [] };
    }

    const incoming = new Map();
    const outgoing = new Map();
    for (const agent of agents) {
      incoming.set(agent.agent_id, 0);
      outgoing.set(agent.agent_id, []);
    }
    for (const edge of edges || []) {
      incoming.set(edge.to_agent_id, (incoming.get(edge.to_agent_id) || 0) + 1);
      outgoing.set(edge.from_agent_id, [...(outgoing.get(edge.from_agent_id) || []), edge.to_agent_id]);
    }

    const depth = new Map();
    const queue = [];
    const roots = agents.filter((agent) => (incoming.get(agent.agent_id) || 0) === 0);
    for (const root of (roots.length > 0 ? roots : agents)) {
      if (!depth.has(root.agent_id)) {
        depth.set(root.agent_id, 0);
        queue.push(root.agent_id);
      }
    }
    while (queue.length > 0) {
      const current = queue.shift();
      const currentDepth = depth.get(current) || 0;
      for (const next of outgoing.get(current) || []) {
        if (!depth.has(next)) {
          depth.set(next, currentDepth + 1);
          queue.push(next);
        }
      }
    }

    for (const agent of agents) {
      if (!depth.has(agent.agent_id)) {
        depth.set(agent.agent_id, 0);
      }
    }

    const maxDepth = Math.max(...agents.map((agent) => depth.get(agent.agent_id) || 0), 0);
    const layers = new Map();
    for (const agent of agents) {
      const layer = depth.get(agent.agent_id) || 0;
      layers.set(layer, [...(layers.get(layer) || []), agent]);
    }

    const nodes = [];
    const positions = new Map();
    for (const [layer, members] of [...layers.entries()].sort((a, b) => a[0] - b[0])) {
      const sortedMembers = [...members].sort((left, right) => left.label.localeCompare(right.label));
      sortedMembers.forEach((agent, index) => {
        const x = maxDepth === 0 ? 50 : 24 + (layer * 52) / maxDepth;
        const y = ((index + 1) * 100) / (sortedMembers.length + 1);
        const node = { ...agent, x, y };
        positions.set(agent.agent_id, node);
        nodes.push(node);
      });
    }

    return {
      nodes,
      edges: (edges || [])
        .map((edge) => {
          const from = positions.get(edge.from_agent_id);
          const to = positions.get(edge.to_agent_id);
          if (!from || !to) return null;
          return { ...edge, from, to };
        })
        .filter(Boolean),
    };
  }

  async function loadTrajectoryIndex() {
    loadingIndex = true;
    error = '';
    try {
      const response = await listTrajectories(200);
      trajectories = response.trajectories || [];
      if (!selectedTrajectoryId && trajectories.length > 0) {
        selectedTrajectoryId = trajectories[0].trajectory_id;
      }
      if (selectedTrajectoryId) {
        await loadTrajectorySnapshot(selectedTrajectoryId);
      }
    } catch (err) {
      if (err instanceof AuthRequiredError) {
        dispatch('authexpired');
        return;
      }
      error = err.message || 'Failed to load Trace';
    } finally {
      loadingIndex = false;
    }
  }

  async function loadTrajectorySnapshot(trajectoryId, { silent = false } = {}) {
    if (!trajectoryId) {
      snapshot = null;
      return;
    }
    if (!silent) {
      snapshotLoading = true;
    }
    error = '';
    try {
      const response = await getTrajectorySnapshot(trajectoryId);
      snapshot = response;
      lastStreamSeq = response?.trajectory?.latest_stream_seq || 0;

      const selectedStillExists = response.moments?.some((moment) => moment.moment_id === selectedMomentId);
      if (!selectedStillExists) {
        selectedMomentId = response.moments?.[response.moments.length - 1]?.moment_id || '';
      }
      if (selectedAgentId && !(response.agents || []).some((agent) => agent.agent_id === selectedAgentId)) {
        selectedAgentId = '';
      }
      if (selectedMomentId) {
        await ensureMomentDetail(selectedMomentId);
      }
      connectStream(trajectoryId);
      streamStatus = 'live';
    } catch (err) {
      if (err instanceof AuthRequiredError) {
        dispatch('authexpired');
        return;
      }
      error = err.message || 'Failed to load trajectory';
    } finally {
      snapshotLoading = false;
    }
  }

  async function ensureMomentDetail(momentId, { force = false } = {}) {
    if (!selectedTrajectoryId || !momentId) return;
    if (!force && momentDetails[momentId]) return;
    detailLoading = true;
    try {
      const detail = await getTrajectoryMomentDetail(selectedTrajectoryId, momentId);
      momentDetails = { ...momentDetails, [momentId]: detail };
    } catch (err) {
      if (err instanceof AuthRequiredError) {
        dispatch('authexpired');
        return;
      }
      error = err.message || 'Failed to load moment detail';
    } finally {
      detailLoading = false;
    }
  }

  function scheduleSnapshotRefresh() {
    if (!selectedTrajectoryId || refreshTimer) return;
    refreshTimer = setTimeout(async () => {
      refreshTimer = null;
      try {
        await loadTrajectorySnapshot(selectedTrajectoryId, { silent: true });
      } catch (err) {
        if (err instanceof AuthRequiredError) {
          dispatch('authexpired');
          return;
        }
        error = err.message || 'Failed to refresh Trace';
      }
    }, 250);
  }

  function connectStream(trajectoryId) {
    if (stream) {
      stream.close();
      stream = null;
    }
    if (!trajectoryId) {
      streamStatus = 'idle';
      return;
    }
    streamStatus = 'connecting';
    stream = openTrajectoryEventStream(trajectoryId, {
      afterSeq: lastStreamSeq,
      onEvent: (eventRecord) => {
        lastStreamSeq = Math.max(lastStreamSeq, eventRecord.stream_seq || 0);
        streamStatus = 'catching-up';
        scheduleSnapshotRefresh();
      },
      onError: () => {
        streamStatus = 'reconnecting';
      },
    });
  }

  async function selectTrajectory(trajectoryId) {
    if (!trajectoryId || trajectoryId === selectedTrajectoryId) return;
    selectedTrajectoryId = trajectoryId;
    selectedAgentId = '';
    selectedMomentId = '';
    momentDetails = {};
    selectedContinuation = null;
    continuationError = '';
    await loadTrajectorySnapshot(trajectoryId);
  }

  async function selectMoment(momentId) {
    if (!momentId) return;
    selectedMomentId = momentId;
    await ensureMomentDetail(momentId);
  }

  function toggleAgent(agentId) {
    selectedAgentId = selectedAgentId === agentId ? '' : agentId;
  }

  async function selectNextContinuation() {
    if (!continuableRunId || continuationBusy) return;
    continuationBusy = true;
    continuationError = '';
    try {
      selectedContinuation = await synthesizeContinuation(continuableRunId);
      await loadTrajectorySnapshot(selectedTrajectoryId, { silent: true });
    } catch (err) {
      if (err instanceof AuthRequiredError) {
        dispatch('authexpired');
        return;
      }
      continuationError = err.message || 'Failed to select continuation';
    } finally {
      continuationBusy = false;
    }
  }

  async function startSelectedContinuation() {
    if (!selectedContinuation?.continuation_id || continuationBusy) return;
    continuationBusy = true;
    continuationError = '';
    try {
      selectedContinuation = await startContinuation(selectedContinuation.continuation_id);
      await loadTrajectorySnapshot(selectedTrajectoryId, { silent: true });
      await loadTrajectoryIndex();
    } catch (err) {
      if (err instanceof AuthRequiredError) {
        dispatch('authexpired');
        return;
      }
      continuationError = err.message || 'Failed to start continuation';
    } finally {
      continuationBusy = false;
    }
  }

  $: trajectory = snapshot?.trajectory || trajectories.find((item) => item.trajectory_id === selectedTrajectoryId) || null;
  $: graphAgents = snapshot?.agents || [];
  $: graphEdges = snapshot?.edges || [];
  $: moments = snapshot?.moments || [];
  $: searchSummary = snapshot?.search || { providers: [] };
  $: graphLayout = buildGraphLayout(graphAgents, graphEdges);
  $: activeMoment = moments.find((moment) => moment.moment_id === selectedMomentId) || moments[moments.length - 1] || null;
  $: activeDetail = selectedMomentId ? momentDetails[selectedMomentId] : null;
  $: geometry = runGeometryStats(moments);
  $: continuableRunId = latestRunId(moments) || (trajectory?.state ? trajectory?.trajectory_id : '');
  $: canContinueTrajectory = canSelectContinuation(trajectory, continuableRunId);

  onMount(() => {
    loadTrajectoryIndex();
  });

  onDestroy(() => {
    if (refreshTimer) clearTimeout(refreshTimer);
    if (stream) stream.close();
  });
</script>

<div class="trace-frame" data-trace-app>
  <div class="trace-app">
    <aside class="trace-sidebar">
    <div class="sidebar-header">
      <div>
        <h2>Trace</h2>
        <p>One trajectory at a time.</p>
      </div>
      <span class={`status-pill ${streamTone(streamStatus)}`}>{streamStatus}</span>
    </div>

    <div class="trajectory-list" data-trace-trajectory-list>
      {#if loadingIndex}
        <div class="empty-state">Loading trajectories…</div>
      {:else if trajectories.length === 0}
        <div class="empty-state">No trajectories yet. Start with the prompt bar or open VText.</div>
      {:else}
        {#each trajectories as item (item.trajectory_id)}
          <button
            class:selected={item.trajectory_id === selectedTrajectoryId}
            class={`trajectory-item ${stateTone(item.state)}`}
            data-trace-trajectory
            data-trace-trajectory-id={item.trajectory_id}
            on:click={() => selectTrajectory(item.trajectory_id)}
          >
            <div class="trajectory-item-top">
              <span class="trajectory-title">{item.title}</span>
              <span class={`status-pill ${stateTone(item.state)}`}>{item.live ? 'live' : item.state || 'idle'}</span>
            </div>
            <div class="trajectory-subtitle">{item.subtitle || item.trajectory_id}</div>
            <div class="trajectory-meta">
              <span>{item.agent_count} agents</span>
              <span>{item.moment_count} moments</span>
              <span>{formatTime(item.latest_activity_at)}</span>
            </div>
          </button>
        {/each}
      {/if}
    </div>
    </aside>

    <section class="trace-main">
    {#if error}
      <div class="error-banner">{error}</div>
    {/if}

    {#if trajectory}
      <header class="trace-header" data-trace-summary>
        <div>
          <h3>{trajectory.title}</h3>
          <p>{trajectory.subtitle || trajectory.trajectory_id}</p>
        </div>
        <div class="trace-header-right">
          {#if canContinueTrajectory}
            <button
              class="ghost-btn"
              data-trace-select-continuation
              on:click={selectNextContinuation}
              disabled={continuationBusy}
            >
              {continuationBusy ? 'Working...' : 'Next Objective'}
            </button>
          {/if}
          <span class={`status-pill ${stateTone(trajectory.state)}`}>{trajectory.live ? 'live' : trajectory.state || 'idle'}</span>
          <span class="status-pill neutral">{formatTime(trajectory.latest_activity_at)}</span>
        </div>
      </header>

      {#if continuationError}
        <div class="error-banner" data-trace-continuation-error>{continuationError}</div>
      {/if}

      {#if selectedContinuation}
        <section class="panel continuation-panel" data-trace-continuation-proposal>
          <div class="panel-header">
            <div>
              <h4>Next objective</h4>
              <p>{selectedContinuation.reason || selectedContinuation.status}</p>
            </div>
            <span class="status-pill active">{selectedContinuation.status}</span>
          </div>
          <pre class="payload-block compact">{selectedContinuation.objective}</pre>
          <div class="continuation-actions">
            <span>{selectedContinuation.authority_profile || 'bounded'} · lease {selectedContinuation.lease_seconds || 0}s</span>
            {#if selectedContinuation.status === 'selected'}
              <button
                class="ghost-btn"
                data-trace-start-continuation
                on:click={startSelectedContinuation}
                disabled={continuationBusy}
              >
                Start
              </button>
            {/if}
          </div>
        </section>
      {/if}

      <div class="metric-row">
        {#each traceMetrics(trajectory) as metric}
          <div class="metric-card">
            <strong>{metric.value}</strong>
            <span>{metric.label}</span>
          </div>
        {/each}
      </div>

      {#if geometry.total > 0}
        <section class="panel geometry-panel" data-trace-run-geometry>
          <div class="panel-header">
            <div>
              <h4>Run geometry</h4>
              <p>Memory, continuation, retry, and promotion control points in this trajectory.</p>
            </div>
            <span class="status-pill active">{geometry.total} control moments</span>
          </div>
          <div class="geometry-grid">
            {#each runGeometryMetrics(geometry) as metric}
              <div class="geometry-chip" data-trace-run-geometry-metric={metric.label}>
                <strong>{metric.value}</strong>
                <span>{metric.label}</span>
              </div>
            {/each}
          </div>
        </section>
      {/if}

      {#if searchSummary.attempts > 0}
        <section class="panel search-panel" data-trace-search-stats>
          <div class="panel-header">
            <div>
              <h4>Search endpoints</h4>
              <p>Provider attempts, health, rate limits, and result volume for this trajectory.</p>
            </div>
            <span class="status-pill neutral">
              {searchSummary.successes || 0}/{searchSummary.attempts || 0} succeeded
            </span>
          </div>
          <div class="search-grid">
            {#each searchSummary.providers || [] as provider (provider.provider)}
              <div class={`search-card ${provider.rate_limits > 0 ? 'error' : provider.successes > 0 ? 'success' : 'neutral'}`}>
                <div class="search-card-top">
                  <strong>{provider.provider}</strong>
                  <span>{provider.successes}/{provider.attempts}</span>
                </div>
                <div class="detail-meta">{provider.endpoint || 'endpoint unavailable'}</div>
                <div class="search-card-metrics">
                  <span>{provider.result_count || 0} results</span>
                  <span>{provider.rate_limits || 0} rate limits</span>
                  <span>{provider.errors || 0} errors</span>
                  {#if provider.avg_latency_ms}
                    <span>{provider.avg_latency_ms}ms avg</span>
                  {/if}
                </div>
                {#if provider.last_error}
                  <pre class="payload-block compact">{provider.last_error}</pre>
                {/if}
              </div>
            {/each}
          </div>
        </section>
      {/if}

      <div class="main-grid">
        <div class="main-left">
          <section class="panel graph-panel" data-trace-graph>
            <div class="panel-header">
              <div>
                <h4>Agent graph</h4>
                <p>Who exists in this trajectory, and who delegated to whom.</p>
              </div>
              {#if selectedAgentId}
                <button class="ghost-btn" on:click={() => (selectedAgentId = '')}>Clear focus</button>
              {/if}
            </div>

            {#if snapshotLoading && graphAgents.length === 0}
              <div class="empty-state">Loading graph…</div>
            {:else if graphAgents.length === 0}
              <div class="empty-state">No agent graph yet for this trajectory.</div>
            {:else}
              <div class="graph-stage">
                <svg class="graph-svg" viewBox="0 0 100 100" preserveAspectRatio="none" aria-hidden="true">
                  {#each graphLayout.edges as edge (`${edge.from_agent_id}-${edge.to_agent_id}`)}
                    <line
                      class="graph-edge"
                      class:graph-edge-focused={selectedAgentId && (edge.from_agent_id === selectedAgentId || edge.to_agent_id === selectedAgentId)}
                      x1={edge.from.x}
                      y1={edge.from.y}
                      x2={edge.to.x}
                      y2={edge.to.y}
                    />
                  {/each}
                </svg>

                {#each graphLayout.nodes as agent (agent.agent_id)}
                  <button
                    class:selected={selectedAgentId === agent.agent_id}
                    class:dimmed={selectedAgentId && selectedAgentId !== agent.agent_id}
                    class={`agent-node ${stateTone(agent.state)}`}
                    style={`left: calc(${agent.x}% - 84px); top: calc(${agent.y}% - 34px);`}
                    data-trace-agent-node
                    data-trace-agent-id={agent.agent_id}
                    on:click={() => toggleAgent(agent.agent_id)}
                  >
                    <span class="agent-node-title">{agent.label}</span>
                    <span class="agent-node-meta">{agentCaption(agent)}</span>
                    <span class="agent-node-footer">
                      <span>{agent.run_count} runs</span>
                      <span>{agent.entry ? 'entry' : 'delegate'}</span>
                    </span>
                  </button>
                {/each}
              </div>
            {/if}
          </section>

          <section class="panel strip-panel" data-trace-moment-strip>
            <div class="panel-header">
              <div>
                <h4>Trajectory moments</h4>
                <p>Each dot is a durable causal moment, not a raw log line.</p>
              </div>
              {#if selectedAgentId}
                <span class="status-pill neutral">
                  focused on {graphAgents.find((agent) => agent.agent_id === selectedAgentId)?.label || excerpt(selectedAgentId, 16)}
                </span>
              {/if}
            </div>

            {#if moments.length === 0}
              <div class="empty-state">No moments captured yet for this trajectory.</div>
            {:else}
              <div class="moment-strip">
                {#each moments as moment (moment.moment_id)}
                  <button
                    class:selected={selectedMomentId === moment.moment_id}
                    class:muted={selectedAgentId && selectedAgentId !== moment.agent_id}
                    class={`moment-chip tone-${moment.tone}`}
                    data-trace-moment
                    data-trace-moment-id={moment.moment_id}
                    on:click={() => selectMoment(moment.moment_id)}
                  >
                    <span class="moment-dot" aria-hidden="true"></span>
                    <span class="moment-agent">{moment.agent_label || 'agent'}</span>
                    <span class="moment-summary">{excerpt(moment.summary, 72)}</span>
                    <span class="moment-meta">{moment.kind} · {formatTime(moment.timestamp)}</span>
                  </button>
                {/each}
              </div>
            {/if}
          </section>
        </div>

        <aside class="panel inspector-panel" data-trace-inspector>
          <div class="panel-header">
            <div>
              <h4>Inspector</h4>
              <p>{activeMoment ? 'Selected moment detail.' : 'Select a moment to inspect.'}</p>
            </div>
            {#if activeMoment}
              <span class={`status-pill tone-${activeMoment.tone}`}>{activeMoment.kind}</span>
            {/if}
          </div>

          {#if !activeMoment}
            <div class="empty-state">Choose a trajectory moment to inspect messages, tool calls, and revision references.</div>
          {:else}
            <div class="inspector-summary" data-trace-moment-detail>
              <div class="inspector-kicker">
                <span>{activeMoment.agent_label || 'agent'}</span>
                <span>#{activeMoment.stream_seq}</span>
                <span>{formatTime(activeMoment.timestamp)}</span>
              </div>
              <h5>{activeMoment.summary}</h5>

              {#if activeDetail?.references}
                <div class="reference-row">
                  {#if activeDetail.references.doc_id}
                    <span class="ref-chip">doc {excerpt(activeDetail.references.doc_id, 18)}</span>
                  {/if}
                  {#if activeDetail.references.revision_id}
                    <span class="ref-chip">rev {excerpt(activeDetail.references.revision_id, 18)}</span>
                  {/if}
                  {#if activeDetail.references.finding_id}
                    <span class="ref-chip">finding {excerpt(activeDetail.references.finding_id, 18)}</span>
                  {/if}
                  {#each activeDetail.references.evidence_ids || [] as evidenceId}
                    <span class="ref-chip">evidence {excerpt(evidenceId, 18)}</span>
                  {/each}
                </div>
              {/if}
            </div>

            {#if detailLoading && !activeDetail}
              <div class="empty-state">Loading selected moment…</div>
            {:else}
              <div class="detail-stack">
                {#if hasArtifacts(activeDetail?.artifacts)}
                  <section class="detail-section" data-trace-artifacts>
                    <h5>Artifacts</h5>
                    {#if activeDetail.artifacts.run_memory}
                      <div class="detail-card" data-trace-artifact-card data-trace-artifact-kind="run_memory">
                        <div class="detail-card-top">
                          <strong>Run memory checkpoint</strong>
                          <span>seq {activeDetail.artifacts.run_memory.seq}</span>
                        </div>
                        <div class="detail-meta">
                          {activeDetail.artifacts.run_memory.reason || 'compaction'} · entry {excerpt(activeDetail.artifacts.run_memory.entry_id, 18)}
                        </div>
                        {#if activeDetail.artifacts.run_memory.summary}
                          <pre class="payload-block compact">{activeDetail.artifacts.run_memory.summary}</pre>
                        {/if}
                        {#if activeDetail.artifacts.run_memory.details}
                          <pre class="payload-block compact">{formatPayload(activeDetail.artifacts.run_memory.details)}</pre>
                        {/if}
                      </div>
                    {/if}

                    {#if activeDetail.artifacts.continuation}
                      <div class="detail-card" data-trace-artifact-card data-trace-artifact-kind="continuation">
                        <div class="detail-card-top">
                          <strong>Continuation</strong>
                          <span>{activeDetail.artifacts.continuation.status}</span>
                        </div>
                        <div class="detail-meta">
                          {activeDetail.artifacts.continuation.authority_profile || 'bounded'} · lease {activeDetail.artifacts.continuation.lease_seconds || 0}s
                        </div>
                        <pre class="payload-block compact">{activeDetail.artifacts.continuation.objective}</pre>
                        {#if activeDetail.artifacts.continuation.details}
                          <pre class="payload-block compact">{formatPayload(activeDetail.artifacts.continuation.details)}</pre>
                        {/if}
                      </div>
                    {/if}

                    {#if activeDetail.artifacts.promotion_candidate}
                      <div class="detail-card" data-trace-artifact-card data-trace-artifact-kind="promotion">
                        <div class="detail-card-top">
                          <strong>Promotion candidate</strong>
                          <span>{activeDetail.artifacts.promotion_candidate.status}</span>
                        </div>
                        <div class="detail-meta">
                          {activeDetail.artifacts.promotion_candidate.vm_id || 'vm'} · {activeDetail.artifacts.promotion_candidate.destination_branch || 'main'}
                        </div>
                        <pre class="payload-block compact">{activeDetail.artifacts.promotion_candidate.summary || activeDetail.artifacts.promotion_candidate.candidate_id}</pre>
                        {#if activeDetail.artifacts.promotion_candidate.report_json?.rollback}
                          <pre class="payload-block compact">{formatPayload(activeDetail.artifacts.promotion_candidate.report_json.rollback)}</pre>
                        {/if}
                      </div>
                    {/if}
                  </section>
                {/if}

                <section class="detail-section">
                  <h5>Events</h5>
                  {#each activeDetail?.events || [] as eventRecord (`${eventRecord.event_id}`)}
                    <div class="detail-card">
                      <div class="detail-card-top">
                        <strong>{eventRecord.kind}</strong>
                        <span>{formatTime(eventRecord.ts)}</span>
                      </div>
                      {#if formatPayload(eventRecord.payload)}
                        <pre class="payload-block">{formatPayload(eventRecord.payload)}</pre>
                      {/if}
                    </div>
                  {/each}
                </section>

                <section class="detail-section">
                  <h5>Messages</h5>
                  {#if (activeDetail?.messages || []).length === 0}
                    <div class="empty-inline">No direct channel message attached to this moment.</div>
                  {:else}
                    {#each activeDetail.messages as message (`${message.channel_id}-${message.seq}`)}
                      <div class="detail-card" data-trace-message-card>
                        <div class="detail-card-top">
                          <strong>{message.from || message.role || 'agent'}</strong>
                          <span>seq {message.seq}</span>
                        </div>
                        <div class="detail-meta">{message.role || 'message'} · {formatTime(message.timestamp)}</div>
                        <pre class="payload-block">{message.content}</pre>
                      </div>
                    {/each}
                  {/if}
                </section>

                <section class="detail-section">
                  <h5>Findings</h5>
                  {#if (activeDetail?.findings || []).length === 0}
                    <div class="empty-inline">No research bundle linked to this moment.</div>
                  {:else}
                    {#each activeDetail.findings as finding (`${finding.finding_id}`)}
                      <div class="detail-card">
                        <div class="detail-card-top">
                          <strong>{finding.finding_id}</strong>
                          <span>{formatTime(finding.created_at)}</span>
                        </div>
                        {#if finding.findings?.length}
                          <div class="detail-meta">Findings</div>
                          <ul class="finding-list">
                            {#each finding.findings as item}
                              <li>{item}</li>
                            {/each}
                          </ul>
                        {/if}
                        {#if finding.questions?.length}
                          <div class="detail-meta">Questions</div>
                          <ul class="finding-list">
                            {#each finding.questions as item}
                              <li>{item}</li>
                            {/each}
                          </ul>
                        {/if}
                      </div>
                    {/each}
                  {/if}
                </section>
              </div>
            {/if}
          {/if}
        </aside>
      </div>
    {:else if !loadingIndex}
      <div class="empty-state">Select a trajectory to inspect its graph, moments, and message flow.</div>
    {/if}
    </section>
  </div>
</div>

<style>
  .trace-frame,
  .trace-app {
    height: 100%;
    min-height: 0;
  }

  .trace-frame {
    container-type: inline-size;
  }

  .trace-app {
    display: grid;
    grid-template-columns: 292px minmax(0, 1fr);
    background: #0a0d14;
    color: #e2e8f0;
  }

  .trace-sidebar,
  .trace-main {
    min-height: 0;
  }

  .trace-sidebar {
    border-right: 1px solid rgba(148, 163, 184, 0.12);
    padding: 0.9rem;
    display: flex;
    flex-direction: column;
    gap: 0.9rem;
    background: rgba(9, 12, 19, 0.92);
  }

  .trace-main {
    padding: 1rem;
    display: flex;
    flex-direction: column;
    gap: 0.9rem;
    overflow: auto;
  }

  .sidebar-header,
  .trace-header,
  .panel-header,
  .trajectory-item-top,
  .detail-card-top,
  .inspector-kicker,
  .trace-header-right {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.75rem;
  }

  .sidebar-header h2,
  .trace-header h3,
  .panel-header h4,
  .inspector-summary h5,
  .detail-section h5 {
    margin: 0;
  }

  .sidebar-header p,
  .trace-header p,
  .panel-header p,
  .trajectory-subtitle,
  .trajectory-meta,
  .detail-meta,
  .inspector-kicker {
    margin: 0;
    color: #94a3b8;
    font-size: 0.78rem;
  }

  .status-pill,
  .ref-chip {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 0.3rem;
    padding: 0.18rem 0.5rem;
    border-radius: 999px;
    border: 1px solid rgba(148, 163, 184, 0.18);
    background: rgba(15, 23, 42, 0.65);
    font-size: 0.72rem;
    color: #cbd5e1;
  }

  .status-pill.success,
  .status-pill.tone-success,
  .tone-success .moment-dot,
  .metric-card strong {
    color: #86efac;
  }

  .status-pill.active,
  .status-pill.tone-tool,
  .tone-tool .moment-dot {
    color: #93c5fd;
  }

  .status-pill.error,
  .status-pill.tone-error,
  .tone-error .moment-dot {
    color: #fca5a5;
  }

  .status-pill.tone-message,
  .tone-message .moment-dot {
    color: #fcd34d;
  }

  .trajectory-list {
    display: flex;
    flex-direction: column;
    gap: 0.6rem;
    overflow: auto;
  }

  .trajectory-item,
  .panel,
  .metric-card,
  .detail-card,
  .empty-state,
  .error-banner {
    border: 1px solid rgba(148, 163, 184, 0.14);
    background: rgba(15, 23, 42, 0.55);
    border-radius: 14px;
  }

  .trajectory-item {
    padding: 0.75rem;
    text-align: left;
    color: inherit;
    cursor: pointer;
  }

  .trajectory-item.selected {
    border-color: rgba(96, 165, 250, 0.38);
    box-shadow: inset 0 0 0 1px rgba(96, 165, 250, 0.28);
  }

  .trajectory-title {
    font-size: 0.86rem;
    font-weight: 600;
    line-height: 1.35;
  }

  .trajectory-subtitle {
    margin-top: 0.45rem;
    line-height: 1.45;
  }

  .trajectory-meta {
    margin-top: 0.55rem;
    display: flex;
    flex-wrap: wrap;
    gap: 0.45rem;
  }

  .metric-row {
    display: grid;
    grid-template-columns: repeat(6, minmax(0, 1fr));
    gap: 0.75rem;
  }

  .metric-card {
    padding: 0.8rem;
    display: grid;
    gap: 0.24rem;
  }

  .metric-card strong {
    font-size: 1rem;
  }

  .metric-card span {
    color: #94a3b8;
    font-size: 0.76rem;
    text-transform: uppercase;
    letter-spacing: 0.04em;
  }

  .geometry-panel {
    display: grid;
    gap: 0.8rem;
  }

  .geometry-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
    gap: 0.65rem;
  }

  .geometry-chip {
    border: 1px solid rgba(96, 165, 250, 0.2);
    border-radius: 12px;
    background: rgba(15, 23, 42, 0.58);
    padding: 0.72rem 0.78rem;
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    gap: 0.75rem;
    min-width: 0;
  }

  .geometry-chip strong {
    color: #dbeafe;
    font-size: 0.95rem;
  }

  .geometry-chip span {
    color: #93c5fd;
    font-size: 0.75rem;
    text-transform: uppercase;
    letter-spacing: 0.04em;
  }

  .main-grid {
    min-height: 0;
    display: grid;
    grid-template-columns: minmax(0, 1.55fr) minmax(320px, 0.95fr);
    gap: 0.9rem;
  }

  .main-left {
    min-height: 0;
    display: flex;
    flex-direction: column;
    gap: 0.9rem;
  }

  .panel {
    padding: 0.95rem;
  }

  .search-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(190px, 1fr));
    gap: 0.7rem;
    margin-top: 0.85rem;
  }

  .search-card {
    border: 1px solid rgba(148, 163, 184, 0.14);
    border-radius: 14px;
    padding: 0.75rem;
    background: rgba(2, 6, 23, 0.34);
    display: grid;
    gap: 0.45rem;
  }

  .search-card.success {
    border-color: rgba(134, 239, 172, 0.24);
  }

  .search-card.error {
    border-color: rgba(252, 165, 165, 0.34);
  }

  .search-card-top,
  .search-card-metrics {
    display: flex;
    flex-wrap: wrap;
    justify-content: space-between;
    gap: 0.45rem;
  }

  .search-card-top strong {
    color: #e2e8f0;
  }

  .search-card-top span,
  .search-card-metrics span {
    color: #94a3b8;
    font-size: 0.75rem;
  }

  .graph-stage {
    position: relative;
    min-height: 360px;
    margin-top: 1rem;
    background: rgba(2, 6, 23, 0.45);
    border-radius: 16px;
    border: 1px solid rgba(148, 163, 184, 0.08);
    overflow: hidden;
  }

  .graph-svg {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
  }

  .graph-edge {
    stroke: rgba(148, 163, 184, 0.22);
    stroke-width: 1.1;
  }

  .graph-edge-focused {
    stroke: rgba(96, 165, 250, 0.55);
  }

  .agent-node {
    position: absolute;
    width: 168px;
    min-height: 68px;
    padding: 0.7rem;
    border-radius: 14px;
    text-align: left;
    color: inherit;
    background: rgba(9, 14, 23, 0.96);
    border: 1px solid rgba(148, 163, 184, 0.16);
    box-shadow: 0 10px 24px rgba(2, 6, 23, 0.22);
    cursor: pointer;
    display: grid;
    gap: 0.22rem;
  }

  .agent-node.selected {
    border-color: rgba(96, 165, 250, 0.48);
    box-shadow: 0 12px 30px rgba(30, 41, 59, 0.32), inset 0 0 0 1px rgba(96, 165, 250, 0.35);
  }

  .agent-node.dimmed {
    opacity: 0.55;
  }

  .agent-node-title {
    font-weight: 600;
    font-size: 0.85rem;
  }

  .agent-node-meta,
  .agent-node-footer {
    color: #94a3b8;
    font-size: 0.73rem;
    display: flex;
    justify-content: space-between;
    gap: 0.45rem;
  }

  .ghost-btn {
    padding: 0.35rem 0.65rem;
    border-radius: 999px;
    border: 1px solid rgba(148, 163, 184, 0.16);
    background: rgba(15, 23, 42, 0.42);
    color: #cbd5e1;
    cursor: pointer;
  }

  .ghost-btn:disabled {
    opacity: 0.55;
    cursor: not-allowed;
  }

  .continuation-actions {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.75rem;
    color: #94a3b8;
    font-size: 0.78rem;
  }

  .moment-strip {
    margin-top: 1rem;
    display: grid;
    gap: 0.65rem;
    max-height: 340px;
    overflow: auto;
  }

  .moment-chip {
    display: grid;
    grid-template-columns: auto minmax(0, auto) minmax(0, 1fr);
    gap: 0.35rem 0.75rem;
    align-items: center;
    padding: 0.7rem 0.8rem;
    border-radius: 14px;
    border: 1px solid rgba(148, 163, 184, 0.14);
    background: rgba(2, 6, 23, 0.4);
    text-align: left;
    color: inherit;
    cursor: pointer;
  }

  .moment-chip.selected {
    border-color: rgba(96, 165, 250, 0.38);
    background: rgba(15, 23, 42, 0.72);
  }

  .moment-chip.muted {
    opacity: 0.48;
  }

  .moment-dot {
    width: 10px;
    height: 10px;
    border-radius: 999px;
    background: currentColor;
  }

  .moment-agent {
    font-size: 0.74rem;
    color: #bfdbfe;
  }

  .moment-summary {
    font-size: 0.82rem;
    line-height: 1.4;
  }

  .moment-meta {
    grid-column: 2 / -1;
    color: #94a3b8;
    font-size: 0.74rem;
  }

  .inspector-panel {
    min-height: 0;
    display: flex;
    flex-direction: column;
    gap: 0.95rem;
  }

  .inspector-summary {
    padding: 0.85rem;
    border-radius: 14px;
    background: rgba(2, 6, 23, 0.42);
    border: 1px solid rgba(148, 163, 184, 0.08);
    display: grid;
    gap: 0.55rem;
  }

  .reference-row {
    display: flex;
    flex-wrap: wrap;
    gap: 0.45rem;
  }

  .detail-stack {
    min-height: 0;
    overflow: auto;
    display: grid;
    gap: 0.9rem;
  }

  .detail-section {
    display: grid;
    gap: 0.6rem;
  }

  .detail-section h5 {
    font-size: 0.82rem;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: #94a3b8;
  }

  .detail-card {
    padding: 0.75rem;
    display: grid;
    gap: 0.45rem;
  }

  .payload-block {
    margin: 0;
    padding: 0.7rem;
    border-radius: 12px;
    background: rgba(2, 6, 23, 0.58);
    white-space: pre-wrap;
    word-break: break-word;
    font-size: 0.77rem;
    line-height: 1.45;
    font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', monospace;
  }

  .finding-list {
    margin: 0;
    padding-left: 1rem;
    color: #cbd5e1;
    font-size: 0.8rem;
    line-height: 1.45;
  }

  .empty-inline,
  .empty-state,
  .error-banner {
    padding: 0.85rem;
    border-radius: 14px;
    font-size: 0.82rem;
  }

  .empty-inline,
  .empty-state {
    color: #94a3b8;
    background: rgba(15, 23, 42, 0.34);
  }

  .error-banner {
    color: #fecaca;
    background: rgba(127, 29, 29, 0.82);
    border: 1px solid rgba(248, 113, 113, 0.26);
  }

  @media (max-width: 1100px) {
    .metric-row {
      grid-template-columns: repeat(3, minmax(0, 1fr));
    }

    .main-grid {
      grid-template-columns: 1fr;
    }

    .inspector-panel {
      position: static;
      max-height: 52vh;
      border-top-left-radius: 18px;
      border-top-right-radius: 18px;
      box-shadow: 0 -14px 30px rgba(2, 6, 23, 0.28);
    }
  }

  @media (max-width: 860px) {
    .trace-app {
      grid-template-columns: 1fr;
    }

    .trace-sidebar {
      border-right: none;
      border-bottom: 1px solid rgba(148, 163, 184, 0.12);
      max-height: 36%;
    }

    .metric-row {
      grid-template-columns: repeat(2, minmax(0, 1fr));
    }

    .graph-stage {
      min-height: 420px;
    }

    .moment-chip {
      grid-template-columns: auto minmax(0, 1fr);
    }

    .moment-summary,
    .moment-meta {
      grid-column: 2 / -1;
    }
  }

  @container (max-width: 980px) {
    .metric-row {
      grid-template-columns: repeat(3, minmax(0, 1fr));
    }

    .main-grid {
      grid-template-columns: 1fr;
    }

    .inspector-panel {
      position: static;
      max-height: none;
      border-top-left-radius: 18px;
      border-top-right-radius: 18px;
      box-shadow: 0 -14px 30px rgba(2, 6, 23, 0.28);
    }
  }

  @container (max-width: 860px) {
    .trace-app {
      grid-template-columns: 1fr;
      grid-template-rows: auto minmax(0, 1fr);
    }

    .trace-sidebar {
      border-right: none;
      border-bottom: 1px solid rgba(148, 163, 184, 0.12);
      max-height: 36%;
    }

    .metric-row {
      grid-template-columns: repeat(2, minmax(0, 1fr));
    }

    .graph-stage {
      min-height: 320px;
    }

    .moment-chip {
      grid-template-columns: auto minmax(0, 1fr);
    }

    .moment-summary,
    .moment-meta {
      grid-column: 2 / -1;
    }
  }
</style>
