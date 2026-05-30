<script lang="ts">
  import { createEventDispatcher, onDestroy, onMount } from 'svelte';
  import { activeWindowId, focusWindow, isHeavyAppId, restoreWindow, suspendBackgroundHeavyWindows, windows } from './stores/desktop.js';
  import { AuthRequiredError } from './auth.js';
  import { fetchComputeStatus, wakeCurrentComputer } from './compute-monitor.js';
  import { addLiveEventListener, liveEventKind } from './live-events.js';
  import { previewComputeStatus } from './public-preview-data';

  export let windowId = '';
  export let authenticated = false;

  const dispatch = createEventDispatcher();
  let status = null;
  let loading = false;
  let error = '';
  let actionStatus = '';
  let removeLiveListener = () => {};

  $: currentComputer = status?.current_computer || {};
  $: computers = status?.computers || [];
  $: candidateComputers = computers.filter((computer) => computer.role === 'candidate');
  $: runtime = status?.runtime || {};
  $: currentWindows = ($windows || []).filter((win) => win.mode !== 'closed' && win.mode !== 'hidden');
  $: visibleWindows = currentWindows.filter((win) => win.mode !== 'minimized');
  $: heavyWindows = visibleWindows.filter((win) => isHeavyAppId(win.appId));
  $: suspendedWindows = visibleWindows.filter((win) => win.restoreSuspended);
  $: healthState = !status
    ? 'loading'
    : status.status !== 'ok'
      ? status.status
      : runtime?.reachable === false
        ? 'degraded'
        : currentComputer.state === 'not_started'
          ? 'not started'
          : 'healthy';

  onMount(() => {
    refresh();
    removeLiveListener = addLiveEventListener((message) => {
      const kind = liveEventKind(message);
      if (kind === 'computer.status.updated' || kind === 'desktop.state.updated' || kind === 'runtime.health' || kind === 'runtime.degraded') {
        void refresh();
      }
    });
  });

  onDestroy(() => {
    removeLiveListener();
  });

  async function refresh() {
    if (!authenticated) {
      status = previewComputeStatus;
      loading = false;
      error = '';
      return;
    }
    loading = true;
    error = '';
    try {
      status = await fetchComputeStatus();
    } catch (err) {
      if (err instanceof AuthRequiredError) {
        dispatch('authexpired');
        return;
      }
      error = err?.message || 'Could not load compute status';
    } finally {
      loading = false;
    }
  }

  async function handleWakeComputer() {
    if (!authenticated) {
      actionStatus = 'Sign in to wake or mutate a durable computer.';
      return;
    }
    actionStatus = 'Waking current computer...';
    try {
      await wakeCurrentComputer();
      actionStatus = 'Wake request accepted';
      await refresh();
    } catch (err) {
      if (err instanceof AuthRequiredError) {
        dispatch('authexpired');
        return;
      }
      actionStatus = err?.message || 'Wake request failed';
    }
  }

  function handleSuspendBackgroundApps() {
    const count = suspendBackgroundHeavyWindows($activeWindowId || windowId);
    actionStatus = count > 0
      ? `Suspended ${count} background heavy app${count === 1 ? '' : 's'}`
      : 'No background heavy apps needed suspension';
  }

  function handleKeepMonitorOnly() {
    if (typeof window !== 'undefined' && !window.confirm('Keep only Compute Monitor and close other saved windows?')) return;
    dispatch('keepwindowonly', { windowId });
  }

  function handleClearDesktopWindows() {
    if (typeof window !== 'undefined' && !window.confirm('Clear all saved desktop windows? This does not delete files or documents.')) return;
    dispatch('clearsavedwindows');
  }

  function handleFocusWindow(win) {
    if (win.mode === 'minimized') {
      restoreWindow(win.windowId);
    } else {
      focusWindow(win.windowId);
    }
  }

  function levelClass() {
    if (status?.status !== 'ok') return 'warn';
    if (runtime?.reachable === false) return 'warn';
    return 'ok';
  }

  function barWidth(value, max = 100) {
    const num = Number(value);
    if (!Number.isFinite(num) || num <= 0) return '0%';
    return `${Math.max(2, Math.min(100, (num / max) * 100))}%`;
  }

  function computerLabel(computer) {
    if (computer.current) return 'Current';
    if (computer.role === 'candidate') return 'Candidate';
    return computer.role || 'Computer';
  }
</script>

<div class="compute-monitor" data-compute-monitor-app>
  <header class="monitor-top" data-compute-monitor-summary>
    <div>
      <p class="eyebrow">Compute Monitor</p>
      <h1>Computer health and recovery</h1>
      <p class="summary-line">
        {currentComputer.protection || 'Status will appear after the current computer reports in.'}
      </p>
    </div>
    <div class="status-cluster">
      <span class="health-pill {levelClass()}" data-compute-monitor-health>{healthState}</span>
    </div>
  </header>

  {#if error}
    <div class="notice error" role="alert">{error}</div>
  {/if}
  {#if actionStatus}
    <div class="notice" aria-live="polite" data-compute-monitor-action-status>{actionStatus}</div>
  {/if}

  <section class="metric-strip" data-compute-monitor-metrics>
    <article class="metric-card">
      <span class="metric-label">Current computer</span>
      <strong>{currentComputer.state || 'unknown'}</strong>
      <div class="meter"><span style="width:{currentComputer.state === 'active' ? '100%' : '35%'}"></span></div>
      <small>{currentComputer.desktop_id || 'primary'} · {currentComputer.warmness_class || 'unreported'}</small>
    </article>
    <article class="metric-card">
      <span class="metric-label">User computers</span>
      <strong>{computers.length || 1}</strong>
      <div class="meter cpu"><span style="width:{barWidth(computers.length || 1, Math.max(computers.length || 1, 1))}"></span></div>
      <small>{candidateComputers.length} background candidate{candidateComputers.length === 1 ? '' : 's'}</small>
    </article>
    <article class="metric-card">
      <span class="metric-label">Runtime health</span>
      <strong>{runtime.runtime_health || runtime.status || (runtime.reachable === false ? 'offline' : 'unknown')}</strong>
      <div class="meter io"><span style="width:{runtime.reachable === false ? '25%' : '100%'}"></span></div>
      <small>{runtime.running_runs ?? 0} running run{runtime.running_runs === 1 ? '' : 's'}</small>
    </article>
    <article class="metric-card">
      <span class="metric-label">App restore weight</span>
      <strong>{heavyWindows.length}/{visibleWindows.length}</strong>
      <div class="meter apps"><span style="width:{barWidth(heavyWindows.length, Math.max(visibleWindows.length, 1))}"></span></div>
      <small>{suspendedWindows.length} suspended</small>
    </article>
  </section>

  <div class="monitor-grid">
    <section class="panel" data-compute-monitor-computer>
      <div class="panel-heading">
        <h2>Current Computer</h2>
        <span class="chip">{currentComputer.warmness_class || 'unknown'}</span>
      </div>
      <dl class="facts">
        <div><dt>State</dt><dd>{currentComputer.state || 'unknown'}</dd></div>
        <div><dt>Desktop</dt><dd>{currentComputer.desktop_id || 'primary'}</dd></div>
        <div><dt>Runtime</dt><dd>{runtime.runtime_health || runtime.status || (runtime.reachable === false ? 'unreachable' : 'unknown')}</dd></div>
        <div><dt>Running runs</dt><dd>{runtime.running_runs ?? 'n/a'}</dd></div>
      </dl>
      <div class="policy-copy">
        <strong>{currentComputer.reclaimable ? 'Reclaimable' : 'Protected first'}</strong>
        <span>{currentComputer.protection || 'Priority policy not reported yet.'}</span>
      </div>
    </section>

    <section class="panel" data-compute-monitor-computers>
      <div class="panel-heading">
        <h2>Your Computers</h2>
        <span class="chip">{computers.length || 1} visible</span>
      </div>
      <div class="computer-list">
        {#each (computers.length ? computers : [currentComputer]) as computer (computer.desktop_id || computer.role || 'current')}
          <article class:current={computer.current} class="computer-row">
            <span>
              <strong>{computerLabel(computer)}</strong>
              <small>{computer.desktop_id || 'primary'}</small>
            </span>
            <span>
              <strong>{computer.state || 'unknown'}</strong>
              <small>{computer.protection || 'priority unavailable'}</small>
            </span>
          </article>
        {/each}
      </div>
    </section>
  </div>

  <section class="panel recovery-panel" data-compute-monitor-recovery>
      <div class="panel-heading">
        <h2>Safe Recovery</h2>
        <span class="chip">state preserving</span>
      </div>
    {#if !authenticated}
      <p class="compact-copy">Preview telemetry is local UI data. Recovery actions require sign-in because they mutate a durable computer.</p>
    {/if}
    <div class="action-grid">
      <button type="button" on:click={handleSuspendBackgroundApps}>Suspend background apps</button>
      <button type="button" on:click={handleWakeComputer} disabled={!status?.capabilities?.wake_current_computer}>Wake current computer</button>
      <button type="button" on:click={handleKeepMonitorOnly}>Keep monitor only</button>
      <button type="button" class="danger" on:click={handleClearDesktopWindows}>Clear saved windows</button>
      <button type="button" disabled title="Not exposed for active primary computers">Kill process</button>
      <button type="button" disabled title="Requires stronger rollback semantics">Force reset computer</button>
    </div>
    <p class="compact-copy">
      Process kill and force reset stay unavailable until app-owned process boundaries and rollback semantics are provable.
    </p>
  </section>

  <section class="panel windows-panel" data-compute-monitor-windows>
    <div class="panel-heading">
      <h2>Apps And Windows</h2>
      <span class="chip">{currentWindows.length} open</span>
    </div>
    {#if currentWindows.length === 0}
      <p class="compact-copy">No restored windows are open.</p>
    {:else}
      <div class="window-list">
        {#each currentWindows as win (win.windowId)}
          <button
            type="button"
            class:active={win.windowId === $activeWindowId}
            class:suspended={win.restoreSuspended}
            class="window-row"
            on:click={() => handleFocusWindow(win)}
            data-compute-monitor-window-row
          >
            <span class="window-icon">{win.icon || '□'}</span>
            <span class="window-copy">
              <strong>{win.title}</strong>
              <small>{win.appId} · {win.mode}{isHeavyAppId(win.appId) ? ' · heavy' : ''}{win.restoreSuspended ? ' · suspended' : ''}</small>
            </span>
          </button>
        {/each}
      </div>
    {/if}
  </section>

  <section class="panel events-panel" data-compute-monitor-events>
    <div class="panel-heading">
      <h2>Recent Compute Evidence</h2>
      <span class="chip">{status?.generated_at ? 'live' : 'pending'}</span>
    </div>
    <ul class="event-list">
      <li>compute status api: {status?.generated_at || 'waiting'}</li>
      <li>current desktop: {currentComputer.desktop_id || 'primary'}</li>
      <li>runtime health: {runtime.runtime_health || runtime.status || 'unknown'}</li>
      <li>warnings: {status?.warnings?.length || 0}</li>
    </ul>
  </section>
</div>

<style>
  .compute-monitor {
    height: 100%;
    min-height: 0;
    overflow: auto;
    padding: 1rem;
    background:
      linear-gradient(135deg, var(--choir-state-selected), var(--choir-state-selected) 46%, var(--choir-state-selected));
    color: var(--choir-text-accent);
  }

  .monitor-top {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 1rem;
    margin-bottom: 0.85rem;
  }

  .eyebrow,
  h1,
  h2,
  p {
    margin: 0;
  }

  .eyebrow {
    color: var(--choir-text-accent);
    font-size: 0.72rem;
    font-weight: 850;
    letter-spacing: 0.12em;
    text-transform: uppercase;
  }

  h1 {
    margin-top: 0.2rem;
    color: var(--choir-text-accent);
    font-size: 1.72rem;
    letter-spacing: 0;
    line-height: 1.05;
  }

  h2 {
    color: var(--choir-text-accent);
    font-size: 0.96rem;
    letter-spacing: 0;
  }

  .summary-line,
  .compact-copy,
  .policy-copy span,
  .event-list {
    color: var(--choir-text-accent);
    font-size: 0.84rem;
    line-height: 1.4;
  }

  .summary-line {
    margin-top: 0.35rem;
    max-width: 44rem;
  }

  .status-cluster {
    display: flex;
    align-items: center;
    gap: 0.45rem;
    flex-shrink: 0;
  }

  .health-pill,
  .chip {
    display: inline-flex;
    align-items: center;
    min-height: 1.55rem;
    border: 1px solid var(--choir-border-strong);
    border-radius: 999px;
    background: var(--choir-state-selected);
    color: var(--choir-text-accent);
    padding: 0.22rem 0.58rem;
    font-size: 0.7rem;
    font-weight: 820;
    text-transform: uppercase;
  }

  .health-pill.ok {
    border-color: var(--choir-status-success);
    color: var(--choir-status-success);
  }

  .health-pill.warn {
    border-color: var(--choir-status-warning);
    color: var(--choir-status-warning);
  }

  .icon-button {
    width: 2.2rem;
    height: 2.2rem;
    border: 1px solid var(--choir-border-strong);
    border-radius: 999px;
    background: var(--choir-state-selected);
    color: var(--choir-text-accent);
    cursor: pointer;
    font-size: 1.05rem;
  }

  .icon-button:disabled {
    cursor: wait;
    opacity: 0.6;
  }

  .notice {
    margin-bottom: 0.75rem;
    border: 1px solid var(--choir-border-strong);
    border-radius: 8px;
    background: var(--choir-state-selected);
    color: var(--choir-text-accent);
    padding: 0.62rem 0.72rem;
    font-size: 0.82rem;
  }

  .notice.error {
    border-color: var(--choir-status-danger);
    background: var(--choir-status-danger-soft);
    color: var(--choir-status-danger);
  }

  .metric-strip {
    display: grid;
    grid-template-columns: repeat(4, minmax(0, 1fr));
    gap: 0.65rem;
    margin-bottom: 0.75rem;
  }

  .metric-card,
  .panel {
    border: 1px solid var(--choir-border-strong);
    border-radius: 8px;
    background: var(--choir-state-selected);
    box-shadow: inset 0 1px 0 color-mix(in srgb, var(--choir-shadow-color) 4%, transparent);
  }

  .metric-card {
    display: grid;
    gap: 0.34rem;
    padding: 0.72rem;
  }

  .metric-label {
    color: var(--choir-text-accent);
    font-size: 0.68rem;
    font-weight: 840;
    letter-spacing: 0.08em;
    text-transform: uppercase;
  }

  .metric-card strong {
    color: var(--choir-text-accent);
    font-size: 1.2rem;
    line-height: 1;
  }

  .metric-card small {
    color: var(--choir-text-accent);
    font-size: 0.72rem;
  }

  .meter {
    overflow: hidden;
    height: 0.45rem;
    border-radius: 999px;
    background: var(--choir-state-selected);
  }

  .meter span {
    display: block;
    height: 100%;
    border-radius: inherit;
    background: linear-gradient(90deg, var(--choir-status-success), var(--choir-state-selected));
  }

  .meter.cpu span {
    background: linear-gradient(90deg, var(--choir-state-selected), var(--choir-status-warning));
  }

  .meter.io span {
    background: linear-gradient(90deg, var(--choir-state-selected), var(--choir-status-warning));
  }

  .meter.apps span {
    background: linear-gradient(90deg, var(--choir-state-selected), var(--choir-status-danger));
  }

  .monitor-grid {
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
    gap: 0.75rem;
    margin-bottom: 0.75rem;
  }

  .panel {
    padding: 0.85rem;
  }

  .panel-heading {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.75rem;
    margin-bottom: 0.7rem;
  }

  .facts {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 0.55rem;
    margin: 0 0 0.75rem;
  }

  .facts div {
    min-width: 0;
    border: 1px solid var(--choir-border-strong);
    border-radius: 6px;
    background: var(--choir-state-selected);
    padding: 0.5rem;
  }

  dt {
    color: var(--choir-text-accent);
    font-size: 0.66rem;
    font-weight: 820;
    letter-spacing: 0.08em;
    text-transform: uppercase;
  }

  dd {
    margin: 0.16rem 0 0;
    overflow: hidden;
    color: var(--choir-text-accent);
    font-size: 0.86rem;
    font-weight: 760;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .policy-copy {
    display: grid;
    gap: 0.18rem;
  }

  .policy-copy strong {
    color: var(--choir-status-success);
    font-size: 0.8rem;
  }

  .recovery-panel,
  .windows-panel,
  .events-panel {
    margin-bottom: 0.75rem;
  }

  .action-grid {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 0.5rem;
    margin-bottom: 0.65rem;
  }

  .action-grid button {
    min-height: 2.45rem;
    border: 1px solid var(--choir-border-strong);
    border-radius: 7px;
    background: var(--choir-state-selected);
    color: var(--choir-text-accent);
    cursor: pointer;
    font: inherit;
    font-size: 0.78rem;
    font-weight: 800;
    padding: 0.5rem 0.58rem;
  }

  .action-grid button:hover:not(:disabled) {
    border-color: var(--choir-border-strong);
    background: var(--choir-state-selected);
  }

  .action-grid button.danger {
    border-color: var(--choir-status-danger);
    background: var(--choir-status-danger-soft);
    color: var(--choir-status-danger);
  }

  .action-grid button:disabled {
    cursor: not-allowed;
    opacity: 0.48;
  }

  .window-list {
    display: grid;
    gap: 0.42rem;
  }

  .computer-list {
    display: grid;
    gap: 0.55rem;
  }

  .computer-row {
    display: grid;
    grid-template-columns: minmax(0, 0.8fr) minmax(0, 1.2fr);
    gap: 0.75rem;
    padding: 0.75rem;
    border: 1px solid var(--choir-border-strong);
    border-radius: 0.75rem;
    background: var(--choir-state-selected);
  }

  .computer-row.current {
    border-color: var(--choir-border-strong);
    background: var(--choir-state-hover);
  }

  .computer-row span {
    min-width: 0;
  }

  .computer-row strong,
  .computer-row small {
    display: block;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .computer-row small {
    margin-top: 0.18rem;
    color: var(--choir-text-accent);
    font-size: 0.72rem;
  }

  .window-row {
    display: grid;
    grid-template-columns: 2rem minmax(0, 1fr);
    gap: 0.58rem;
    align-items: center;
    width: 100%;
    border: 1px solid var(--choir-border-strong);
    border-radius: 7px;
    background: var(--choir-state-selected);
    color: var(--choir-text-accent);
    cursor: pointer;
    padding: 0.52rem;
    text-align: left;
  }

  .window-row.active {
    border-color: var(--choir-border-strong);
    background: var(--choir-state-selected);
  }

  .window-row.suspended {
    border-color: var(--choir-status-warning);
  }

  .window-icon {
    font-size: 1.1rem;
    text-align: center;
  }

  .window-copy {
    display: grid;
    gap: 0.12rem;
    min-width: 0;
  }

  .window-copy strong,
  .window-copy small {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .window-copy strong {
    color: var(--choir-text-accent);
    font-size: 0.84rem;
  }

  .window-copy small {
    color: var(--choir-text-accent);
    font-size: 0.72rem;
  }

  .event-list {
    display: grid;
    gap: 0.32rem;
    margin: 0;
    padding-left: 1.05rem;
    font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
    overflow-wrap: anywhere;
  }

  @media (max-width: 760px) {
    .compute-monitor {
      padding: 0.75rem;
    }

    h1 {
      font-size: 1.42rem;
    }

    .monitor-top {
      align-items: flex-start;
    }

    .metric-strip,
    .monitor-grid,
    .action-grid {
      grid-template-columns: 1fr;
    }

    .facts {
      grid-template-columns: 1fr 1fr;
    }
  }
</style>
