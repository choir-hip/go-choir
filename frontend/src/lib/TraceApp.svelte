<script lang="ts">
  import { createEventDispatcher, onDestroy, onMount } from 'svelte';
  import { AuthRequiredError } from './auth.js';
  import {
    getTrajectoryMomentDetail,
    getTrajectoryLogText,
    getTrajectorySnapshot,
    listTrajectories,
    openTrajectoryEventStream,
    startContinuation,
    synthesizeContinuation,
  } from './trace.js';
  import { addLiveEventListener, liveEventKind } from './live-events.js';
  import { previewTraceSnapshot, previewTraceTrajectories } from './public-preview-data';

  const dispatch = createEventDispatcher();

  export let authenticated = false;
  export let appContext = {};

  let loadingIndex = true;
  let snapshotLoading = false;
  let detailLoading = false;
  let error = '';
  let trajectories = [];
  let snapshot = null;
  let selectedTrajectoryId = appContext?.trajectoryId || appContext?.traceId || '';
  let selectedAgentId = '';
  let selectedMomentId = '';
  let momentDetails = {};
  let stream = null;
  let streamStatus = 'idle';
  let lastStreamSeq = 0;
  let refreshTimer = null;
  let indexRefreshTimer = null;
  let removeLiveListener = null;
  let continuationBusy = false;
  let continuationError = '';
  let selectedContinuation = null;
  let selectedAcceptanceId = appContext?.acceptanceId || '';
  let mobilePanel = 'timeline';
  let lastAppContextTarget = '';
  let copyLogsBusy = false;
  let copyLogsStatus = '';
  let userPinnedTrajectory = Boolean(selectedTrajectoryId);

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

  function liveEventTrajectoryId(message) {
    const direct = String(message?.trajectory_id || '').trim();
    if (direct) return direct;
    const payload = message?.payload && typeof message.payload === 'object' ? message.payload : {};
    const payloadTrajectory = String(payload.trajectory_id || '').trim();
    if (payloadTrajectory) return payloadTrajectory;
    return String(message?.loop_id || '').trim();
  }

  function stateTone(state) {
    if (state === 'completed' || state === 'passed' || state === 'accepted') return 'success';
    if (state === 'running' || state === 'pending' || state === 'blocked' || state === 'synthesized') return 'active';
    if (state === 'failed' || state === 'cancelled' || state === 'rejected') return 'error';
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

  function agentModelCaption(agent) {
    const provider = String(agent?.llm_provider || '').trim();
    const model = String(agent?.llm_model || '').trim().split('/').pop();
    const reasoning = String(agent?.llm_reasoning_effort || '').trim();
    const policy = String(agent?.model_policy || '').trim();
    const main = [provider, model].filter(Boolean).join(' / ');
    const suffix = [reasoning, policy].filter(Boolean).join(' · ');
    if (main && suffix) return `${main} · ${suffix}`;
    return main || suffix || 'model config pending';
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
    const stats = { compactions: 0, continuations: 0, retries: 0, appMoves: 0 };
    for (const moment of items || []) {
      const kind = String(moment?.kind || '');
      if (kind.startsWith('loop.compaction')) stats.compactions += 1;
      if (kind.startsWith('loop.continuation')) stats.continuations += 1;
      if (kind === 'loop.retry') stats.retries += 1;
      if (kind.startsWith('app_change_package') || kind.startsWith('app_adoption')) stats.appMoves += 1;
    }
    return { ...stats, total: stats.compactions + stats.continuations + stats.retries + stats.appMoves };
  }

  function runGeometryMetrics(stats) {
    return [
      { label: 'compactions', value: stats.compactions },
      { label: 'continuations', value: stats.continuations },
      { label: 'retries', value: stats.retries },
      { label: 'app moves', value: stats.appMoves },
    ].filter((metric) => metric.value > 0);
  }

  function hasArtifacts(artifacts) {
    return !!(artifacts?.run_memory || artifacts?.continuation || artifacts?.app_change_package || artifacts?.app_adoption);
  }

  function momentTime(moment) {
    return parseDate(moment?.timestamp || moment?.created_at || moment?.started_at || moment?.updated_at);
  }

  function laneTone(moment) {
    if (moment?.tone) return moment.tone;
    return stateTone(moment?.state);
  }

  function buildSwimlanes(agents, items) {
    const laneMap = new Map();
    for (const agent of agents || []) {
      laneMap.set(agent.agent_id, {
        id: agent.agent_id,
        label: agent.label || excerpt(agent.agent_id, 18),
        role: agent.role || '',
        moments: [],
      });
    }
    for (const moment of items || []) {
      const agentId = moment.agent_id || moment.agent_label || 'system';
      if (!laneMap.has(agentId)) {
        laneMap.set(agentId, {
          id: agentId,
          label: moment.agent_label || excerpt(agentId, 18),
          role: 'moment source',
          moments: [],
        });
      }
      laneMap.get(agentId).moments.push(moment);
    }

    const times = (items || []).map(momentTime).filter((time) => time > 0);
    const start = times.length ? Math.min(...times) : Date.now();
    const end = times.length ? Math.max(...times) : start + 60_000;
    const range = Math.max(60_000, end - start);

    return [...laneMap.values()].map((lane) => {
      const sorted = [...lane.moments].sort((a, b) => momentTime(a) - momentTime(b));
      return {
        ...lane,
        moments: sorted.map((moment, index) => {
          const time = momentTime(moment) || start + index * 10_000;
          const nextTime = momentTime(sorted[index + 1]) || Math.min(start + range, time + Math.max(18_000, range / Math.max(4, sorted.length + 1)));
          const left = Math.max(0, Math.min(96, ((time - start) / range) * 100));
          const width = Math.max(4.5, Math.min(36, ((nextTime - time) / range) * 100));
          return {
            ...moment,
            laneLeft: left,
            laneWidth: Math.min(width, 100 - left),
            laneTone: laneTone(moment),
          };
        }),
      };
    });
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

  function closeTraceStream() {
    if (refreshTimer) {
      clearTimeout(refreshTimer);
      refreshTimer = null;
    }
    if (indexRefreshTimer) {
      clearTimeout(indexRefreshTimer);
      indexRefreshTimer = null;
    }
    if (stream) {
      stream.close();
      stream = null;
    }
  }

  function enterGuestTrace() {
    closeTraceStream();
    loadingIndex = false;
    snapshotLoading = false;
    detailLoading = false;
    error = '';
    trajectories = previewTraceTrajectories;
    snapshot = previewTraceSnapshot;
    selectedTrajectoryId = previewTraceSnapshot.trajectory.trajectory_id;
    selectedAgentId = '';
    selectedMomentId = previewTraceSnapshot.moments[previewTraceSnapshot.moments.length - 1]?.moment_id || '';
    momentDetails = {
      [selectedMomentId]: {
        moment: previewTraceSnapshot.moments[previewTraceSnapshot.moments.length - 1],
        payload: {
          preview: true,
          note: 'Local Trace preview. Private run evidence loads after sign-in.',
        },
      },
    };
    selectedContinuation = null;
    selectedAcceptanceId = '';
    continuationError = '';
    streamStatus = 'guest';
  }

  function requestTraceAuth() {
    dispatch('authrequired', {
      kind: 'app_launch',
      appId: 'trace',
      appName: 'Trace',
      icon: '🔎',
    });
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

  async function loadTrajectoryIndex({ silent = false } = {}) {
    if (!authenticated) {
      enterGuestTrace();
      return;
    }
    if (!silent) {
      loadingIndex = true;
    }
    error = '';
    try {
      const previousSelected = selectedTrajectoryId;
      const previousLatest = trajectories[0]?.trajectory_id || '';
      const response = await listTrajectories(200);
      trajectories = response.trajectories || [];
      const nextLatest = trajectories[0]?.trajectory_id || '';
      if (!selectedTrajectoryId && nextLatest) {
        selectedTrajectoryId = nextLatest;
      } else if (
        !userPinnedTrajectory &&
        previousSelected &&
        previousSelected === previousLatest &&
        nextLatest &&
        nextLatest !== previousSelected
      ) {
        selectedTrajectoryId = nextLatest;
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
      if (!silent) {
        loadingIndex = false;
      }
    }
  }

  async function loadTrajectorySnapshot(trajectoryId, { silent = false } = {}) {
    if (!authenticated) {
      enterGuestTrace();
      return;
    }
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
      const nextAcceptances = response?.acceptances || [];
      if (!nextAcceptances.some((item) => item.acceptance_id === selectedAcceptanceId)) {
        selectedAcceptanceId = nextAcceptances[0]?.acceptance_id || '';
      }

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
    if (!authenticated) return;
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
    if (!authenticated) return;
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

  function scheduleTrajectoryIndexRefresh() {
    if (!authenticated || indexRefreshTimer) return;
    indexRefreshTimer = setTimeout(async () => {
      indexRefreshTimer = null;
      try {
        await loadTrajectoryIndex({ silent: true });
      } catch (err) {
        if (err instanceof AuthRequiredError) {
          dispatch('authexpired');
          return;
        }
        error = err.message || 'Failed to refresh Trace runs';
      }
    }, 250);
  }

  async function copyTrajectoryLogs() {
    if (!authenticated) {
      requestTraceAuth();
      return;
    }
    if (!selectedTrajectoryId || copyLogsBusy) return;
    copyLogsBusy = true;
    copyLogsStatus = '';
    try {
      const text = await getTrajectoryLogText(selectedTrajectoryId);
      await navigator.clipboard.writeText(text);
      copyLogsStatus = 'copied';
    } catch (err) {
      if (err instanceof AuthRequiredError) {
        dispatch('authexpired');
        return;
      }
      copyLogsStatus = err.message || 'copy failed';
    } finally {
      copyLogsBusy = false;
      setTimeout(() => {
        copyLogsStatus = '';
      }, 2200);
    }
  }

  function connectStream(trajectoryId) {
    closeTraceStream();
    if (!authenticated) {
      streamStatus = 'guest';
      return;
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
    if (!authenticated) {
      const selected = previewTraceTrajectories.find((item) => item.trajectory_id === trajectoryId);
      if (!selected) return;
      selectedTrajectoryId = trajectoryId;
      snapshot = { ...previewTraceSnapshot, trajectory: selected };
      mobilePanel = 'timeline';
      return;
    }
    if (!trajectoryId || trajectoryId === selectedTrajectoryId) return;
    selectedTrajectoryId = trajectoryId;
    selectedAgentId = '';
    selectedMomentId = '';
    momentDetails = {};
    selectedContinuation = null;
    selectedAcceptanceId = '';
    continuationError = '';
    mobilePanel = 'timeline';
    userPinnedTrajectory = true;
    await loadTrajectorySnapshot(trajectoryId);
  }

  async function selectAppContextTrajectory() {
    if (!authenticated) return;
    const trajectoryId = appContext?.trajectoryId || appContext?.traceId || '';
    const acceptanceId = appContext?.acceptanceId || '';
    const targetKey = `${trajectoryId}:${acceptanceId}`;
    if (!trajectoryId || targetKey === lastAppContextTarget) return;
    lastAppContextTarget = targetKey;
    selectedTrajectoryId = trajectoryId;
    selectedAcceptanceId = acceptanceId;
    selectedAgentId = '';
    userPinnedTrajectory = true;
    selectedMomentId = '';
    momentDetails = {};
    selectedContinuation = null;
    continuationError = '';
    mobilePanel = 'timeline';
    await loadTrajectorySnapshot(trajectoryId);
  }

  async function selectMoment(momentId) {
    if (!authenticated) {
      selectedMomentId = momentId;
      mobilePanel = 'inspector';
      momentDetails = {
        ...momentDetails,
        [momentId]: {
          moment: previewTraceSnapshot.moments.find((item) => item.moment_id === momentId),
          payload: { preview: true, note: 'Local inspector detail; not backend proof.' },
        },
      };
      return;
    }
    if (!momentId) return;
    selectedMomentId = momentId;
    mobilePanel = 'inspector';
    await ensureMomentDetail(momentId);
  }

  function toggleAgent(agentId) {
    selectedAgentId = selectedAgentId === agentId ? '' : agentId;
  }

  async function selectNextContinuation() {
    if (!authenticated) {
      requestTraceAuth();
      return;
    }
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
    if (!authenticated) {
      requestTraceAuth();
      return;
    }
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
  $: mobileSummary = snapshot?.mobile_summary || null;
  $: showMobileSummary =
    !!mobileSummary?.acceptance_state ||
    !!mobileSummary?.readable_evidence?.length ||
    !!mobileSummary?.rollback_refs?.length;
  $: acceptances = snapshot?.acceptances || [];
  $: activeAcceptance =
    acceptances.find((item) => item.acceptance_id === selectedAcceptanceId) || acceptances[0] || null;
  $: graphLayout = buildGraphLayout(graphAgents, graphEdges);
  $: swimlanes = buildSwimlanes(graphAgents, moments);
  $: activeMoment = moments.find((moment) => moment.moment_id === selectedMomentId) || moments[moments.length - 1] || null;
  $: activeDetail = selectedMomentId ? momentDetails[selectedMomentId] : null;
  $: geometry = runGeometryStats(moments);
  $: continuableRunId = latestRunId(moments) || (trajectory?.state ? trajectory?.trajectory_id : '');
  $: canContinueTrajectory = canSelectContinuation(trajectory, continuableRunId);

  onMount(() => {
    removeLiveListener = addLiveEventListener((message) => {
      if (!authenticated) return;
      const kind = liveEventKind(message);
      if (!kind) return;
      const eventTrajectoryId = liveEventTrajectoryId(message);
      if (
        !userPinnedTrajectory &&
        eventTrajectoryId &&
        eventTrajectoryId !== selectedTrajectoryId &&
        (kind === 'loop.submitted' || kind === 'loop.started')
      ) {
        selectedTrajectoryId = eventTrajectoryId;
        selectedAgentId = '';
        selectedMomentId = '';
        momentDetails = {};
        selectedContinuation = null;
        selectedAcceptanceId = '';
        continuationError = '';
        mobilePanel = 'timeline';
        scheduleSnapshotRefresh();
      }
      scheduleTrajectoryIndexRefresh();
      if (eventTrajectoryId && eventTrajectoryId === selectedTrajectoryId) {
        scheduleSnapshotRefresh();
      }
    });
    if (authenticated) {
      loadTrajectoryIndex();
    } else {
      enterGuestTrace();
    }
  });

  $: if (authenticated && streamStatus === 'guest') {
    loadTrajectoryIndex();
  }

  $: if (authenticated && (appContext?.trajectoryId || appContext?.traceId)) {
    selectAppContextTrajectory();
  }

  $: if (!authenticated && streamStatus !== 'guest') {
    enterGuestTrace();
  }

  onDestroy(() => {
    removeLiveListener?.();
    closeTraceStream();
  });
</script>

<div class="trace-frame" data-trace-app>
  <div class="trace-app" data-mobile-panel={mobilePanel} data-trace-scroll-owner>
    <nav class="mobile-trace-tabs" aria-label="Trace sections" data-trace-mobile-tabs>
      <button type="button" class:selected={mobilePanel === 'runs'} on:click={() => (mobilePanel = 'runs')}>Runs</button>
      <button type="button" class:selected={mobilePanel === 'summary'} on:click={() => (mobilePanel = 'summary')}>Summary</button>
      <button type="button" class:selected={mobilePanel === 'timeline'} on:click={() => (mobilePanel = 'timeline')}>Timeline</button>
      <button type="button" class:selected={mobilePanel === 'inspector'} on:click={() => (mobilePanel = 'inspector')}>Inspector</button>
    </nav>
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
      {#if !authenticated}
        <div class="preview-banner" data-trace-guest>
          <span>Local preview</span>
          <button class="ghost-btn" data-trace-guest-sign-in on:click={requestTraceAuth}>Sign in for private Trace</button>
        </div>
      {/if}
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
          <button
            class="ghost-btn"
            data-trace-copy-logs
            on:click={copyTrajectoryLogs}
            disabled={copyLogsBusy}
            title="Copy this trajectory's debug log"
          >
            {copyLogsBusy ? 'Copying...' : copyLogsStatus || 'Copy logs'}
          </button>
          <span class={`status-pill ${stateTone(trajectory.state)}`}>{trajectory.live ? 'live' : trajectory.state || 'idle'}</span>
          <span class="status-pill neutral">{formatTime(trajectory.latest_activity_at)}</span>
        </div>
      </header>

      {#if continuationError}
        <div class="error-banner" data-trace-continuation-error>{continuationError}</div>
      {/if}

      <div class="trace-summary-stack" data-trace-summary-panel>
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

      {#if showMobileSummary}
        <section class="panel mobile-summary-panel" data-trace-mobile-summary>
          <div class="panel-header">
            <div>
              <h4>Provenance summary</h4>
              <p>{mobileSummary.headline}</p>
            </div>
            {#if mobileSummary.acceptance_state}
              <span class={`status-pill ${stateTone(mobileSummary.acceptance_state)}`}>
                {mobileSummary.acceptance_level || mobileSummary.acceptance_state}
              </span>
            {/if}
          </div>

          <div class="mobile-summary-strip">
            <span>{mobileSummary.agent_count || 0} agents</span>
            <span>{mobileSummary.delegation_count || 0} delegations</span>
            <span>{mobileSummary.evidence_ref_count || 0} evidence refs</span>
            <span>{mobileSummary.rollback_ref_count || 0} rollback refs</span>
          </div>

          {#if mobileSummary.readable_evidence?.length || mobileSummary.rollback_refs?.length}
            <div class="mobile-summary-detail">
              {#if mobileSummary.readable_evidence?.length}
                <div>
                  <h5>Evidence</h5>
                  {#each mobileSummary.readable_evidence as evidence}
                    <p>{evidence}</p>
                  {/each}
                </div>
              {/if}
              {#if mobileSummary.rollback_refs?.length}
                <div>
                  <h5>Rollback</h5>
                  {#each mobileSummary.rollback_refs as rollback}
                    <p>{rollback}</p>
                  {/each}
                </div>
              {/if}
            </div>
          {/if}
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

      {#if acceptances.length > 0}
        <section class="panel acceptance-panel" data-trace-run-acceptance>
          <div class="panel-header">
            <div>
              <h4>Run acceptance</h4>
              <p>{activeAcceptance?.target_mission_id || activeAcceptance?.acceptance_id}</p>
            </div>
            {#if activeAcceptance}
              <span class={`status-pill ${stateTone(activeAcceptance.state)}`}>{activeAcceptance.acceptance_level}</span>
            {/if}
          </div>

          {#if acceptances.length > 1}
            <div class="acceptance-tabs" role="tablist" aria-label="Run acceptances">
              {#each acceptances as acceptance (acceptance.acceptance_id)}
                <button
                  class:selected={activeAcceptance?.acceptance_id === acceptance.acceptance_id}
                  class="acceptance-tab"
                  type="button"
                  on:click={() => (selectedAcceptanceId = acceptance.acceptance_id)}
                >
                  <span>{acceptance.acceptance_level}</span>
                  <small>{excerpt(acceptance.acceptance_id, 18)}</small>
                </button>
              {/each}
            </div>
          {/if}

          {#if activeAcceptance}
            <div class="acceptance-grid">
              <div class="acceptance-card">
                <span class="detail-meta">State</span>
                <strong>{activeAcceptance.state}</strong>
                <span>{activeAcceptance.authority_profile || 'authority unknown'}</span>
              </div>
              <div class="acceptance-card">
                <span class="detail-meta">Build</span>
                <strong>{excerpt(activeAcceptance.health_commit || activeAcceptance.deployment_commit || 'unknown', 18)}</strong>
                <span>{activeAcceptance.staging_url || 'staging'}</span>
              </div>
              <div class="acceptance-card">
                <span class="detail-meta">Run</span>
                <strong>{excerpt(activeAcceptance.loop_id || 'loop unknown', 18)}</strong>
                <span>{excerpt(activeAcceptance.trajectory_id || 'trajectory unknown', 24)}</span>
              </div>
              <div class="acceptance-card">
                <span class="detail-meta">Evidence</span>
                <strong>{activeAcceptance.evidence_refs?.length || 0}</strong>
                <span>{activeAcceptance.rollback_refs?.length || 0} rollback refs</span>
              </div>
            </div>

            <div class="acceptance-detail-grid">
              <section class="acceptance-block">
                <h5>Checkpoints</h5>
                <div class="acceptance-list">
                  {#each activeAcceptance.checkpoints || [] as checkpoint}
                    <div class="acceptance-row" data-trace-acceptance-checkpoint>
                      <div class="detail-card-top">
                        <strong>{checkpoint.kind}</strong>
                        <span class={`status-pill ${stateTone(checkpoint.state)}`}>{checkpoint.state}</span>
                      </div>
                      {#if checkpoint.evidence_ref_ids?.length}
                        <div class="reference-row">
                          {#each checkpoint.evidence_ref_ids as refId}
                            <span class="ref-chip">{excerpt(refId, 28)}</span>
                          {/each}
                        </div>
                      {/if}
                      {#if checkpoint.details}
                        <pre class="payload-block compact">{formatPayload(checkpoint.details)}</pre>
                      {/if}
                    </div>
                  {/each}
                </div>
              </section>

              <section class="acceptance-block">
                <h5>Evidence refs</h5>
                <div class="acceptance-list">
                  {#each activeAcceptance.evidence_refs || [] as ref (ref.ref_id)}
                    <details class="evidence-detail" data-trace-acceptance-evidence>
                      <summary>
                        <span>{excerpt(ref.ref_id, 30)}</span>
                        <small>{ref.kind || ref.summary}</small>
                      </summary>
                      <div class="detail-meta">{ref.summary}</div>
                      {#if ref.url}
                        <div class="detail-meta">{ref.url}</div>
                      {/if}
                      {#if ref.details}
                        <pre class="payload-block compact">{formatPayload(ref.details)}</pre>
                      {/if}
                    </details>
                  {/each}
                </div>
              </section>
            </div>

            {#if activeAcceptance.rollback_refs?.length || activeAcceptance.verifier_contracts?.length}
              <div class="acceptance-detail-grid">
                {#if activeAcceptance.verifier_contracts?.length}
                  <section class="acceptance-block">
                    <h5>Verifier contracts</h5>
                    <div class="acceptance-list">
                      {#each activeAcceptance.verifier_contracts as contract (contract.name)}
                        <div class="acceptance-row">
                          <div class="detail-card-top">
                            <strong>{contract.name}</strong>
                            <span class={`status-pill ${stateTone(contract.state)}`}>{contract.state}</span>
                          </div>
                          <div class="detail-meta">{contract.purpose}</div>
                        </div>
                      {/each}
                    </div>
                  </section>
                {/if}

                {#if activeAcceptance.rollback_refs?.length}
                  <section class="acceptance-block">
                    <h5>Rollback refs</h5>
                    <div class="acceptance-list">
                      {#each activeAcceptance.rollback_refs as rollback (`${rollback.kind}-${rollback.ref}`)}
                        <div class="acceptance-row">
                          <div class="detail-card-top">
                            <strong>{rollback.kind}</strong>
                            <span>{excerpt(rollback.ref, 24)}</span>
                          </div>
                          <div class="detail-meta">{rollback.summary}</div>
                        </div>
                      {/each}
                    </div>
                  </section>
                {/if}
              </div>
            {/if}
          {/if}
        </section>
      {/if}

      {#if geometry.total > 0}
        <section class="panel geometry-panel" data-trace-run-geometry>
          <div class="panel-header">
            <div>
              <h4>Run geometry</h4>
              <p>Memory, continuation, retry, and app package/adoption control points in this trajectory.</p>
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
      </div>

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
                    style={`left: clamp(0px, calc(${agent.x}% - 84px), calc(100% - 168px)); top: clamp(0px, calc(${agent.y}% - 34px), calc(100% - 68px));`}
                    data-trace-agent-node
                    data-trace-agent-id={agent.agent_id}
                    data-trace-agent-model={agentModelCaption(agent)}
                    on:click={() => toggleAgent(agent.agent_id)}
                  >
                    <span class="agent-node-title">{agent.label}</span>
                    <span class="agent-node-meta">{agentCaption(agent)}</span>
                    <span class="agent-node-model">{agentModelCaption(agent)}</span>
                    <span class="agent-node-footer">
                      <span>{agent.run_count} runs</span>
                      <span>{agent.entry ? 'entry' : 'delegate'}</span>
                    </span>
                  </button>
                {/each}
              </div>
            {/if}
          </section>

          <section class="panel strip-panel" data-trace-swimlanes>
            <div class="panel-header">
              <div>
                <h4>Swimlanes</h4>
                <p>Agent and tool duration bars with failure ticks and selectable moments.</p>
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
              <div class="swimlane-chart" data-trace-swimlane-chart>
                {#each swimlanes as lane (lane.id)}
                  <div class:selected={selectedAgentId === lane.id} class:muted={selectedAgentId && selectedAgentId !== lane.id} class="swimlane-row" data-trace-swimlane data-trace-swimlane-id={lane.id}>
                    <button class="swimlane-label" type="button" on:click={() => toggleAgent(lane.id)}>
                      <strong>{lane.label}</strong>
                      <small>{lane.role || 'agent'}</small>
                    </button>
                    <div class="swimlane-rail">
                      {#each lane.moments as moment (moment.moment_id)}
                        <button
                          class:selected={selectedMomentId === moment.moment_id}
                          class:failed={moment.laneTone === 'error'}
                          class={`swimlane-moment tone-${moment.laneTone}`}
                          style={`left: ${moment.laneLeft}%; width: ${moment.laneWidth}%;`}
                          type="button"
                          data-trace-moment
                          data-trace-moment-id={moment.moment_id}
                          title={`${moment.agent_label || lane.label}: ${moment.summary || moment.title || moment.kind}`}
                          on:click={() => selectMoment(moment.moment_id)}
                        >
                          <span class="swimlane-bar"></span>
                          <span class="swimlane-dot"></span>
                          {#if moment.laneTone === 'error'}<span class="failure-tick" aria-label="failed"></span>{/if}
                        </button>
                      {/each}
                    </div>
                  </div>
                {/each}
                <div class="swimlane-legend">
                  <span><i class="legend-dot success"></i>Completed</span>
                  <span><i class="legend-dot active"></i>In progress</span>
                  <span><i class="legend-dot error"></i>Failed</span>
                </div>
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

                    {#if activeDetail.artifacts.app_change_package}
                      <div class="detail-card" data-trace-artifact-card data-trace-artifact-kind="app-change-package">
                        <div class="detail-card-top">
                          <strong>AppChangePackage</strong>
                          <span>{activeDetail.artifacts.app_change_package.status}</span>
                        </div>
                        <div class="detail-meta">
                          {activeDetail.artifacts.app_change_package.source_computer_id || 'source computer'} · {activeDetail.artifacts.app_change_package.visibility || 'visibility'}
                        </div>
                        <pre class="payload-block compact">{activeDetail.artifacts.app_change_package.app_id || activeDetail.artifacts.app_change_package.package_id}</pre>
                      </div>
                    {/if}

                    {#if activeDetail.artifacts.app_adoption}
                      <div class="detail-card" data-trace-artifact-card data-trace-artifact-kind="app-adoption">
                        <div class="detail-card-top">
                          <strong>App adoption</strong>
                          <span>{activeDetail.artifacts.app_adoption.status}</span>
                        </div>
                        <div class="detail-meta">
                          {activeDetail.artifacts.app_adoption.target_computer_id || 'target computer'} · {activeDetail.artifacts.app_adoption.package_id}
                        </div>
                        {#if activeDetail.artifacts.app_adoption.runtime_artifact_digest || activeDetail.artifacts.app_adoption.ui_artifact_digest}
                          <pre class="payload-block compact">{formatPayload({
                            runtime_artifact_digest: activeDetail.artifacts.app_adoption.runtime_artifact_digest,
                            ui_artifact_digest: activeDetail.artifacts.app_adoption.ui_artifact_digest,
                            rollback_profile: activeDetail.artifacts.app_adoption.rollback_profile_json
                          })}</pre>
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
    background: var(--choir-panel, #0a0d14);
    color: var(--choir-fg, #e2e8f0);
    overscroll-behavior: contain;
    touch-action: pan-y;
  }

  .mobile-trace-tabs {
    display: none;
  }

  .trace-summary-stack {
    display: grid;
    gap: 0.9rem;
  }

  .trace-sidebar,
  .trace-main {
    min-height: 0;
  }

  .trace-sidebar {
    padding: 0.9rem;
    display: flex;
    flex-direction: column;
    gap: 0.9rem;
    min-height: 0;
    overflow: hidden;
    background:
      linear-gradient(90deg, color-mix(in srgb, var(--choir-accent, #60a5fa) 10%, transparent), transparent 96%),
      var(--choir-panel-soft, rgba(9, 12, 19, 0.92));
    box-shadow: var(--choir-card-shadow, 18px 0 36px rgba(0, 0, 0, 0.14));
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

  .detail-card-top {
    min-width: 0;
    flex-wrap: wrap;
  }

  .detail-card-top > * {
    min-width: 0;
    overflow-wrap: anywhere;
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
    border-radius: var(--choir-radius-control-sm, 14px);
    border: 0;
    background: rgba(15, 23, 42, 0.65);
    box-shadow: 0 8px 20px rgba(0, 0, 0, 0.16);
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
    min-height: 0;
    overflow: auto;
  }

  .trajectory-item,
  .panel,
  .metric-card,
  .detail-card,
  .empty-state,
  .error-banner {
    border: 0;
    background: rgba(15, 23, 42, 0.55);
    border-radius: var(--choir-radius-control, 20px);
    box-shadow: 0 16px 38px rgba(0, 0, 0, 0.22);
  }

  .trajectory-item {
    display: block;
    width: 100%;
    padding: 0.75rem;
    text-align: left;
    color: inherit;
    cursor: pointer;
  }

  .trajectory-item.selected {
    box-shadow:
      0 16px 38px rgba(0, 0, 0, 0.22),
      0 0 34px rgba(96, 165, 250, 0.2);
  }

  .trajectory-title {
    min-width: 0;
    font-size: 0.86rem;
    font-weight: 600;
    line-height: 1.35;
    overflow-wrap: anywhere;
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

  .mobile-summary-panel {
    display: grid;
    gap: 0.8rem;
  }

  .mobile-summary-strip {
    display: flex;
    flex-wrap: wrap;
    gap: 0.45rem;
    min-width: 0;
  }

  .mobile-summary-strip span {
    border: 0;
    border-radius: var(--choir-radius-control-sm, 14px);
    background: rgba(2, 6, 23, 0.34);
    color: #cbd5e1;
    font-size: 0.74rem;
    padding: 0.25rem 0.55rem;
    overflow-wrap: anywhere;
  }

  .mobile-summary-detail {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 0.75rem;
    min-width: 0;
  }

  .mobile-summary-detail div {
    display: grid;
    gap: 0.35rem;
    min-width: 0;
  }

  .mobile-summary-detail h5 {
    margin: 0;
    color: #94a3b8;
    font-size: 0.74rem;
    text-transform: uppercase;
    letter-spacing: 0.04em;
  }

  .mobile-summary-detail p {
    margin: 0;
    color: #e2e8f0;
    font-size: 0.8rem;
    line-height: 1.45;
    overflow-wrap: anywhere;
  }

  .acceptance-panel {
    display: grid;
    gap: 0.85rem;
  }

  .acceptance-tabs,
  .acceptance-grid,
  .acceptance-detail-grid,
  .acceptance-list {
    min-width: 0;
  }

  .acceptance-tabs {
    display: flex;
    flex-wrap: wrap;
    gap: 0.5rem;
  }

  .acceptance-tab {
    min-width: 0;
    padding: 0.55rem 0.65rem;
    border-radius: var(--choir-radius-control-sm, 14px);
    border: 0;
    background: rgba(2, 6, 23, 0.42);
    color: #cbd5e1;
    cursor: pointer;
    display: grid;
    gap: 0.18rem;
    text-align: left;
  }

  .acceptance-tab.selected {
    background: rgba(15, 23, 42, 0.74);
    box-shadow: 0 0 30px rgba(96, 165, 250, 0.18);
  }

  .acceptance-tab small {
    color: #94a3b8;
    font-size: 0.72rem;
  }

  .acceptance-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
    gap: 0.65rem;
  }

  .acceptance-card,
  .acceptance-row,
  .evidence-detail {
    min-width: 0;
    max-width: 100%;
    border: 0;
    border-radius: var(--choir-radius-control-sm, 14px);
    background: rgba(2, 6, 23, 0.34);
    box-shadow: 0 12px 30px rgba(0, 0, 0, 0.18);
  }

  .acceptance-card {
    padding: 0.75rem;
    display: grid;
    gap: 0.25rem;
  }

  .acceptance-card strong {
    color: #e2e8f0;
    font-size: 0.9rem;
    overflow-wrap: anywhere;
  }

  .acceptance-card span:last-child {
    color: #94a3b8;
    font-size: 0.75rem;
    overflow-wrap: anywhere;
  }

  .acceptance-detail-grid {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 0.75rem;
  }

  .acceptance-block {
    min-width: 0;
    max-width: 100%;
    display: grid;
    gap: 0.55rem;
  }

  .acceptance-block h5 {
    margin: 0;
    color: #94a3b8;
    font-size: 0.78rem;
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }

  .acceptance-list {
    display: grid;
    gap: 0.55rem;
  }

  .acceptance-row {
    padding: 0.7rem;
    display: grid;
    gap: 0.45rem;
    overflow: hidden;
  }

  .evidence-detail {
    padding: 0.7rem;
    display: grid;
    gap: 0.45rem;
    overflow: hidden;
  }

  .evidence-detail summary {
    cursor: pointer;
    display: flex;
    justify-content: space-between;
    gap: 0.75rem;
    color: #e2e8f0;
    font-size: 0.8rem;
    overflow-wrap: anywhere;
  }

  .evidence-detail summary small {
    color: #94a3b8;
    text-align: right;
  }

  .acceptance-panel .detail-card-top {
    align-items: flex-start;
    flex-wrap: wrap;
  }

  .acceptance-panel .detail-card-top strong,
  .acceptance-panel .detail-meta,
  .acceptance-panel .status-pill,
  .acceptance-panel .ref-chip {
    min-width: 0;
    max-width: 100%;
    overflow-wrap: anywhere;
    white-space: normal;
  }

  .geometry-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
    gap: 0.65rem;
  }

  .geometry-chip {
    border: 0;
    border-radius: var(--choir-radius-control-sm, 14px);
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
    border: 0;
    border-radius: var(--choir-radius-control, 20px);
    padding: 0.75rem;
    background: rgba(2, 6, 23, 0.34);
    display: grid;
    gap: 0.45rem;
  }

  .search-card.success {
    box-shadow: 0 0 28px rgba(134, 239, 172, 0.12);
  }

  .search-card.error {
    box-shadow: 0 0 28px rgba(252, 165, 165, 0.14);
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
    border-radius: var(--choir-radius-control, 20px);
    border: 0;
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
    border-radius: var(--choir-radius-control, 20px);
    text-align: left;
    color: inherit;
    background: rgba(9, 14, 23, 0.96);
    box-shadow: 0 10px 24px rgba(2, 6, 23, 0.22);
    cursor: pointer;
    display: grid;
    gap: 0.22rem;
  }

  .agent-node.selected {
    box-shadow: 0 12px 30px rgba(30, 41, 59, 0.32), 0 0 34px rgba(96, 165, 250, 0.18);
  }

  .agent-node.dimmed {
    opacity: 0.55;
  }

  .agent-node-title {
    font-weight: 600;
    font-size: 0.85rem;
  }

  .agent-node-meta,
  .agent-node-model,
  .agent-node-footer {
    color: #94a3b8;
    font-size: 0.73rem;
    display: flex;
    justify-content: space-between;
    gap: 0.45rem;
  }

  .agent-node-model {
    color: #bfdbfe;
    display: block;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .ghost-btn {
    padding: 0.35rem 0.65rem;
    border-radius: var(--choir-radius-control-sm, 14px);
    border: 0;
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

  .guest-trace-panel {
    align-self: start;
    display: grid;
    gap: 0.9rem;
    max-width: 720px;
  }

  .guest-trace-actions {
    display: flex;
    justify-content: flex-start;
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
    border-radius: var(--choir-radius-control, 20px);
    border: 0;
    background: rgba(2, 6, 23, 0.4);
    text-align: left;
    color: inherit;
    cursor: pointer;
  }

  .moment-chip.selected {
    background: rgba(15, 23, 42, 0.72);
    box-shadow: 0 0 30px rgba(96, 165, 250, 0.16);
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

  .swimlane-chart {
    margin-top: 0.9rem;
    display: grid;
    gap: 0.58rem;
    min-width: 0;
  }

  .swimlane-row {
    display: grid;
    grid-template-columns: minmax(7.5rem, 0.24fr) minmax(0, 1fr);
    gap: 0.7rem;
    align-items: center;
    min-width: 0;
    opacity: 1;
  }

  .swimlane-row.muted {
    opacity: 0.42;
  }

  .swimlane-row.selected .swimlane-label {
    color: var(--choir-accent, #60a5fa);
  }

  .swimlane-label {
    min-width: 0;
    border: 0;
    border-radius: var(--choir-radius-control-sm, 14px);
    background: rgba(15, 23, 42, 0.42);
    color: #cbd5e1;
    padding: 0.42rem 0.5rem;
    text-align: left;
    box-shadow: 0 10px 24px rgba(0, 0, 0, 0.14);
    cursor: pointer;
  }

  .swimlane-label strong,
  .swimlane-label small {
    display: block;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .swimlane-label strong {
    font-size: 0.78rem;
  }

  .swimlane-label small {
    color: #94a3b8;
    font-size: 0.68rem;
  }

  .swimlane-rail {
    position: relative;
    min-width: 0;
    height: 2.1rem;
    border-radius: var(--choir-radius-control-sm, 14px);
    background:
      linear-gradient(90deg, rgba(148, 163, 184, 0.06) 1px, transparent 1px) 0 0 / 12.5% 100%,
      rgba(2, 6, 23, 0.34);
    box-shadow: inset 0 10px 20px rgba(255, 255, 255, 0.018), 0 12px 28px rgba(0, 0, 0, 0.16);
  }

  .swimlane-moment {
    position: absolute;
    top: 50%;
    display: block;
    min-width: 1.4rem;
    height: 1.1rem;
    border: 0;
    border-radius: var(--choir-radius-control-sm, 14px);
    background: transparent;
    padding: 0;
    transform: translateY(-50%);
    cursor: pointer;
  }

  .swimlane-bar {
    position: absolute;
    inset: 0.3rem 0;
    border-radius: 999px;
    background: currentColor;
    opacity: 0.34;
    box-shadow: 0 0 18px currentColor;
  }

  .swimlane-dot {
    position: absolute;
    left: 0;
    top: 50%;
    width: 0.68rem;
    height: 0.68rem;
    border-radius: 999px;
    background: currentColor;
    transform: translate(-10%, -50%);
    box-shadow: 0 0 14px currentColor;
  }

  .swimlane-moment.selected .swimlane-bar {
    opacity: 0.54;
  }

  .swimlane-moment.selected .swimlane-dot {
    width: 0.82rem;
    height: 0.82rem;
  }

  .failure-tick {
    position: absolute;
    right: -0.1rem;
    top: 50%;
    width: 0.82rem;
    height: 0.82rem;
    border-radius: 999px;
    background: #ef4444;
    transform: translate(50%, -50%);
    box-shadow: 0 0 16px rgba(239, 68, 68, 0.8);
  }

  .swimlane-legend {
    display: flex;
    flex-wrap: wrap;
    gap: 0.75rem;
    color: #94a3b8;
    font-size: 0.72rem;
  }

  .swimlane-legend span {
    display: inline-flex;
    align-items: center;
    gap: 0.32rem;
  }

  .legend-dot {
    width: 0.48rem;
    height: 0.48rem;
    border-radius: 999px;
    background: currentColor;
  }

  .legend-dot.success,
  .tone-success {
    color: #86efac;
  }

  .legend-dot.active,
  .tone-active {
    color: #60a5fa;
  }

  .legend-dot.error,
  .tone-error {
    color: #f87171;
  }

  .tone-tool {
    color: #38bdf8;
  }

  .tone-message {
    color: #a78bfa;
  }

  .inspector-panel {
    min-height: 0;
    display: flex;
    flex-direction: column;
    gap: 0.95rem;
  }

  .inspector-summary {
    padding: 0.85rem;
    border-radius: var(--choir-radius-control, 20px);
    background: rgba(2, 6, 23, 0.42);
    border: 0;
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
    min-width: 0;
    overflow-wrap: anywhere;
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
    min-width: 0;
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

  .payload-block.compact {
    max-height: 240px;
    overflow: auto;
    overflow-wrap: anywhere;
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
    border-radius: var(--choir-radius-control, 20px);
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
    box-shadow: 0 0 30px rgba(248, 113, 113, 0.18);
  }

  @media (max-width: 1100px) {
    .metric-row {
      grid-template-columns: repeat(3, minmax(0, 1fr));
    }

    .main-grid {
      grid-template-columns: 1fr;
    }

    .acceptance-detail-grid {
      grid-template-columns: 1fr;
    }

    .mobile-summary-detail {
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
      grid-template-rows: minmax(0, auto) minmax(0, 1fr);
    }

    .trace-sidebar {
      border-right: none;
      box-shadow: 0 14px 30px rgba(0, 0, 0, 0.16);
      max-height: min(44vh, 18rem);
    }

    .sidebar-header,
    .trace-header,
    .panel-header,
    .trajectory-item-top,
    .trace-header-right {
      align-items: flex-start;
      flex-wrap: wrap;
    }

    .trace-header {
      flex-direction: column;
    }

    .trace-header-right {
      justify-content: flex-start;
    }

    .status-pill {
      flex-shrink: 0;
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

    .evidence-detail summary {
      display: grid;
      gap: 0.2rem;
    }

    .evidence-detail summary small {
      text-align: left;
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

    .acceptance-detail-grid {
      grid-template-columns: 1fr;
    }

    .mobile-summary-detail {
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
      grid-template-rows: auto auto minmax(0, 1fr);
      overflow-x: hidden;
      overflow-y: auto;
      -webkit-overflow-scrolling: touch;
    }

    .mobile-trace-tabs {
      grid-column: 1;
      grid-row: 1;
    }

    .trace-sidebar {
      grid-column: 1;
      grid-row: 2;
      max-height: none;
      overflow: visible;
    }

    .trace-main {
      grid-column: 1;
      grid-row: 3;
      overflow: visible;
    }

    .mobile-trace-tabs {
      display: flex;
      gap: 0.4rem;
      overflow-x: auto;
      padding: 0.55rem 0.65rem;
      background: var(--choir-panel-soft, rgba(9, 12, 19, 0.96));
      box-shadow: var(--choir-card-shadow, 0 16px 32px rgba(0, 0, 0, 0.18));
      scrollbar-width: none;
    }

    .mobile-trace-tabs::-webkit-scrollbar {
      display: none;
    }

    .mobile-trace-tabs button {
      flex: 1 0 auto;
      min-height: 34px;
      border: 0;
      border-radius: var(--choir-radius-control-sm, 14px);
      background: var(--choir-control-bg, rgba(15, 23, 42, 0.74));
      color: var(--choir-fg, #cbd5e1);
      cursor: pointer;
      font-weight: 820;
    }

    .mobile-trace-tabs button.selected {
      background: var(--choir-selected, rgba(37, 99, 235, 0.26));
      box-shadow: var(--choir-control-shadow, 0 0 30px rgba(96, 165, 250, 0.18));
      color: var(--choir-fg, #eff6ff);
    }

    .trace-app[data-mobile-panel='timeline'] .trace-sidebar,
    .trace-app[data-mobile-panel='inspector'] .trace-sidebar,
    .trace-app[data-mobile-panel='runs'] .trace-main {
      display: none;
    }

    .trace-app[data-mobile-panel='runs'] .trace-sidebar {
      max-height: none;
    }

    .trajectory-list,
    .moment-strip,
    .detail-stack {
      max-height: none;
      overflow: visible;
    }

    .payload-block.compact {
      max-height: none;
      overflow: visible;
    }

    .trace-app[data-mobile-panel='summary'] .main-grid,
    .trace-app[data-mobile-panel='timeline'] .trace-summary-stack,
    .trace-app[data-mobile-panel='timeline'] [data-trace-inspector],
    .trace-app[data-mobile-panel='inspector'] .trace-summary-stack,
    .trace-app[data-mobile-panel='inspector'] .main-left {
      display: none;
    }

    .trace-sidebar {
      border-right: none;
      box-shadow: 0 16px 32px rgba(0, 0, 0, 0.18);
    }

    .sidebar-header,
    .trace-header,
    .panel-header,
    .trajectory-item-top,
    .trace-header-right {
      align-items: flex-start;
      flex-wrap: wrap;
    }

    .trace-header {
      flex-direction: column;
    }

    .trace-header-right {
      justify-content: flex-start;
    }

    .status-pill {
      flex-shrink: 0;
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

    .evidence-detail summary {
      display: grid;
      gap: 0.2rem;
    }

    .evidence-detail summary small {
      text-align: left;
    }

    .moment-summary,
    .moment-meta {
      grid-column: 2 / -1;
    }
  }

  @media (max-width: 720px) {
    .trace-frame,
    .trace-main,
    .trace-body,
    .acceptance-grid,
    .acceptance-detail-grid {
      grid-template-columns: minmax(0, 1fr) !important;
      max-width: 100%;
      overflow-x: hidden;
    }

    .trace-frame {
      padding: 10px !important;
    }

    .panel,
    .acceptance-card,
    .acceptance-row,
    .detail-card,
    .inspector-panel {
      min-width: 0 !important;
      overflow-wrap: anywhere;
    }

    .acceptance-tabs,
    .trajectory-list {
      overflow-x: auto;
      max-width: 100%;
    }

    [data-trace-inspector],
    [data-trace-run-acceptance] {
      max-height: none;
    }

    .trace-app {
      overflow-x: hidden;
      overflow-y: auto;
    }

    .trace-sidebar,
    .trace-main,
    .trajectory-list,
    .moment-strip,
    .detail-stack,
    .payload-block.compact {
      max-height: none;
      overflow: visible;
    }
  }

</style>
