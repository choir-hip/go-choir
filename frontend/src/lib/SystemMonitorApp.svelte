<script>
  import { createEventDispatcher, onDestroy, onMount } from 'svelte';
  import { activeWindowId, focusWindow, restoreWindow, suspendBackgroundHeavyWindows, windows } from './stores/desktop.js';
  import { AuthRequiredError } from './auth.js';
  import { fetchSystemStatus, wakeCurrentComputer } from './system-monitor.js';

  export let windowId = '';
  export let authenticated = false;

  const dispatch = createEventDispatcher();
  const HEAVY_APPS = new Set(['browser', 'candidate-desktop', 'terminal', 'vtext', 'trace', 'podcast', 'image', 'audio', 'video', 'pdf', 'epub']);

  let status = null;
  let loading = false;
  let error = '';
  let actionStatus = '';
  let refreshTimer = null;

  $: currentComputer = status?.current_computer || {};
  $: vmctl = status?.vmctl || {};
  $: pressure = vmctl?.reclaim?.pressure || {};
  $: reclaim = vmctl?.reclaim || {};
  $: runtime = status?.runtime || {};
  $: currentWindows = ($windows || []).filter((win) => win.mode !== 'closed' && win.mode !== 'hidden');
  $: visibleWindows = currentWindows.filter((win) => win.mode !== 'minimized');
  $: heavyWindows = visibleWindows.filter((win) => HEAVY_APPS.has(win.appId));
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
  $: memoryPercent = Number(pressure?.memory_available_percent || 0);
  $: cpuPsi = Number(pressure?.cpu_some_avg10 || 0);
  $: ioPsi = Number(pressure?.io_some_avg10 || 0);

  onMount(() => {
    refresh();
    refreshTimer = setInterval(refresh, 15000);
  });

  onDestroy(() => {
    if (refreshTimer) clearInterval(refreshTimer);
  });

  async function refresh() {
    if (!authenticated) return;
    loading = true;
    error = '';
    try {
      status = await fetchSystemStatus();
    } catch (err) {
      if (err instanceof AuthRequiredError) {
        dispatch('authexpired');
        return;
      }
      error = err?.message || 'Could not load system status';
    } finally {
      loading = false;
    }
  }

  async function handleWakeComputer() {
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
    if (typeof window !== 'undefined' && !window.confirm('Keep only System Monitor and close other saved windows?')) return;
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

  function formatBytes(value) {
    const bytes = Number(value || 0);
    if (!bytes) return 'n/a';
    const units = ['B', 'KB', 'MB', 'GB', 'TB'];
    let scaled = bytes;
    let unit = 0;
    while (scaled >= 1024 && unit < units.length - 1) {
      scaled /= 1024;
      unit += 1;
    }
    return `${scaled >= 10 ? scaled.toFixed(0) : scaled.toFixed(1)} ${units[unit]}`;
  }

  function pct(value) {
    const num = Number(value);
    if (!Number.isFinite(num) || num <= 0) return 'n/a';
    return `${num.toFixed(num >= 10 ? 0 : 1)}%`;
  }

  function psi(value) {
    const num = Number(value);
    if (!Number.isFinite(num)) return 'n/a';
    return `${num.toFixed(num >= 10 ? 0 : 2)}`;
  }

  function levelClass() {
    if (status?.status !== 'ok') return 'warn';
    if (pressure?.pressure) return 'warn';
    if (runtime?.reachable === false) return 'warn';
    return 'ok';
  }

  function barWidth(value, max = 100) {
    const num = Number(value);
    if (!Number.isFinite(num) || num <= 0) return '0%';
    return `${Math.max(2, Math.min(100, (num / max) * 100))}%`;
  }
</script>

<div class="system-monitor" data-system-monitor-app>
  <header class="monitor-top" data-system-monitor-summary>
    <div>
      <p class="eyebrow">System Monitor</p>
      <h1>Computer health and recovery</h1>
      <p class="summary-line">
        {currentComputer.protection || 'Status will appear after the current computer reports in.'}
      </p>
    </div>
    <div class="status-cluster">
      <span class="health-pill {levelClass()}" data-system-monitor-health>{healthState}</span>
      <button type="button" class="icon-button" on:click={refresh} disabled={loading} title="Refresh status" aria-label="Refresh status">
        ↻
      </button>
    </div>
  </header>

  {#if error}
    <div class="notice error" role="alert">{error}</div>
  {/if}
  {#if actionStatus}
    <div class="notice" aria-live="polite" data-system-monitor-action-status>{actionStatus}</div>
  {/if}

  <section class="metric-strip" data-system-monitor-metrics>
    <article class="metric-card">
      <span class="metric-label">Memory available</span>
      <strong>{pct(memoryPercent)}</strong>
      <div class="meter"><span style="width:{barWidth(memoryPercent)}"></span></div>
      <small>{formatBytes(pressure?.memory_available_bytes)} free</small>
    </article>
    <article class="metric-card">
      <span class="metric-label">CPU pressure</span>
      <strong>{psi(cpuPsi)}</strong>
      <div class="meter cpu"><span style="width:{barWidth(cpuPsi, 100)}"></span></div>
      <small>avg10 PSI</small>
    </article>
    <article class="metric-card">
      <span class="metric-label">I/O pressure</span>
      <strong>{psi(ioPsi)}</strong>
      <div class="meter io"><span style="width:{barWidth(ioPsi, 20)}"></span></div>
      <small>avg10 PSI</small>
    </article>
    <article class="metric-card">
      <span class="metric-label">App restore weight</span>
      <strong>{heavyWindows.length}/{visibleWindows.length}</strong>
      <div class="meter apps"><span style="width:{barWidth(heavyWindows.length, Math.max(visibleWindows.length, 1))}"></span></div>
      <small>{suspendedWindows.length} suspended</small>
    </article>
  </section>

  <div class="monitor-grid">
    <section class="panel" data-system-monitor-computer>
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

    <section class="panel" data-system-monitor-vmctl>
      <div class="panel-heading">
        <h2>Lifecycle Pressure</h2>
        <span class="chip">{reclaim.mode || 'unknown'}</span>
      </div>
      <dl class="facts">
        <div><dt>Active computers</dt><dd>{vmctl.active_vms ?? 'n/a'}</dd></div>
        <div><dt>Total records</dt><dd>{vmctl.total_ownerships ?? 'n/a'}</dd></div>
        <div><dt>Idle eligible</dt><dd>{vmctl.idle_eligible ?? 'n/a'}</dd></div>
        <div><dt>Decision</dt><dd>{reclaim.decision || 'n/a'}</dd></div>
      </dl>
      <p class="compact-copy">{reclaim.reason || 'No lifecycle decision has been reported yet.'}</p>
    </section>
  </div>

  <section class="panel recovery-panel" data-system-monitor-recovery>
    <div class="panel-heading">
      <h2>Safe Recovery</h2>
      <span class="chip">state preserving</span>
    </div>
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

  <section class="panel windows-panel" data-system-monitor-windows>
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
            data-system-monitor-window-row
          >
            <span class="window-icon">{win.icon || '□'}</span>
            <span class="window-copy">
              <strong>{win.title}</strong>
              <small>{win.appId} · {win.mode}{HEAVY_APPS.has(win.appId) ? ' · heavy' : ''}{win.restoreSuspended ? ' · suspended' : ''}</small>
            </span>
          </button>
        {/each}
      </div>
    {/if}
  </section>

  <section class="panel events-panel" data-system-monitor-events>
    <div class="panel-heading">
      <h2>Recent Evidence</h2>
      <span class="chip">{status?.generated_at ? 'live' : 'pending'}</span>
    </div>
    <ul class="event-list">
      <li>status api: {status?.generated_at || 'waiting'}</li>
      <li>build: {status?.build?.commit ? status.build.commit.slice(0, 12) : 'unknown'}</li>
      <li>runtime provider: {runtime.active_provider || 'unknown'}</li>
      <li>pressure sample: {pressure.sampled_at || 'unavailable'}</li>
    </ul>
  </section>
</div>

<style>
  .system-monitor {
    height: 100%;
    min-height: 0;
    overflow: auto;
    padding: 1rem;
    background:
      linear-gradient(135deg, rgba(8, 13, 24, 0.98), rgba(11, 18, 32, 0.98) 46%, rgba(6, 16, 18, 0.98));
    color: #e5edf9;
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
    color: #7dd3fc;
    font-size: 0.72rem;
    font-weight: 850;
    letter-spacing: 0.12em;
    text-transform: uppercase;
  }

  h1 {
    margin-top: 0.2rem;
    color: #f8fbff;
    font-size: 1.72rem;
    letter-spacing: 0;
    line-height: 1.05;
  }

  h2 {
    color: #f1f5f9;
    font-size: 0.96rem;
    letter-spacing: 0;
  }

  .summary-line,
  .compact-copy,
  .policy-copy span,
  .event-list {
    color: #9fb0c6;
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
    border: 1px solid rgba(148, 163, 184, 0.22);
    border-radius: 999px;
    background: rgba(15, 23, 42, 0.75);
    color: #dbeafe;
    padding: 0.22rem 0.58rem;
    font-size: 0.7rem;
    font-weight: 820;
    text-transform: uppercase;
  }

  .health-pill.ok {
    border-color: rgba(74, 222, 128, 0.34);
    color: #bbf7d0;
  }

  .health-pill.warn {
    border-color: rgba(251, 191, 36, 0.38);
    color: #fde68a;
  }

  .icon-button {
    width: 2.2rem;
    height: 2.2rem;
    border: 1px solid rgba(148, 163, 184, 0.22);
    border-radius: 999px;
    background: rgba(15, 23, 42, 0.82);
    color: #e0f2fe;
    cursor: pointer;
    font-size: 1.05rem;
  }

  .icon-button:disabled {
    cursor: wait;
    opacity: 0.6;
  }

  .notice {
    margin-bottom: 0.75rem;
    border: 1px solid rgba(125, 211, 252, 0.25);
    border-radius: 8px;
    background: rgba(8, 47, 73, 0.38);
    color: #dff7ff;
    padding: 0.62rem 0.72rem;
    font-size: 0.82rem;
  }

  .notice.error {
    border-color: rgba(248, 113, 113, 0.34);
    background: rgba(69, 10, 10, 0.38);
    color: #fee2e2;
  }

  .metric-strip {
    display: grid;
    grid-template-columns: repeat(4, minmax(0, 1fr));
    gap: 0.65rem;
    margin-bottom: 0.75rem;
  }

  .metric-card,
  .panel {
    border: 1px solid rgba(148, 163, 184, 0.16);
    border-radius: 8px;
    background: rgba(15, 23, 42, 0.72);
    box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.035);
  }

  .metric-card {
    display: grid;
    gap: 0.34rem;
    padding: 0.72rem;
  }

  .metric-label {
    color: #93a4ba;
    font-size: 0.68rem;
    font-weight: 840;
    letter-spacing: 0.08em;
    text-transform: uppercase;
  }

  .metric-card strong {
    color: #f8fafc;
    font-size: 1.2rem;
    line-height: 1;
  }

  .metric-card small {
    color: #8da0b8;
    font-size: 0.72rem;
  }

  .meter {
    overflow: hidden;
    height: 0.45rem;
    border-radius: 999px;
    background: rgba(30, 41, 59, 0.92);
  }

  .meter span {
    display: block;
    height: 100%;
    border-radius: inherit;
    background: linear-gradient(90deg, #22c55e, #38bdf8);
  }

  .meter.cpu span {
    background: linear-gradient(90deg, #38bdf8, #f59e0b);
  }

  .meter.io span {
    background: linear-gradient(90deg, #2dd4bf, #f97316);
  }

  .meter.apps span {
    background: linear-gradient(90deg, #60a5fa, #f87171);
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
    border: 1px solid rgba(148, 163, 184, 0.12);
    border-radius: 6px;
    background: rgba(2, 6, 23, 0.32);
    padding: 0.5rem;
  }

  dt {
    color: #94a3b8;
    font-size: 0.66rem;
    font-weight: 820;
    letter-spacing: 0.08em;
    text-transform: uppercase;
  }

  dd {
    margin: 0.16rem 0 0;
    overflow: hidden;
    color: #e2e8f0;
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
    color: #d9f99d;
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
    border: 1px solid rgba(96, 165, 250, 0.28);
    border-radius: 7px;
    background: rgba(30, 64, 175, 0.34);
    color: #eff6ff;
    cursor: pointer;
    font: inherit;
    font-size: 0.78rem;
    font-weight: 800;
    padding: 0.5rem 0.58rem;
  }

  .action-grid button:hover:not(:disabled) {
    border-color: rgba(125, 211, 252, 0.55);
    background: rgba(37, 99, 235, 0.42);
  }

  .action-grid button.danger {
    border-color: rgba(248, 113, 113, 0.32);
    background: rgba(127, 29, 29, 0.28);
    color: #fee2e2;
  }

  .action-grid button:disabled {
    cursor: not-allowed;
    opacity: 0.48;
  }

  .window-list {
    display: grid;
    gap: 0.42rem;
  }

  .window-row {
    display: grid;
    grid-template-columns: 2rem minmax(0, 1fr);
    gap: 0.58rem;
    align-items: center;
    width: 100%;
    border: 1px solid rgba(148, 163, 184, 0.12);
    border-radius: 7px;
    background: rgba(2, 6, 23, 0.32);
    color: #e5edf9;
    cursor: pointer;
    padding: 0.52rem;
    text-align: left;
  }

  .window-row.active {
    border-color: rgba(96, 165, 250, 0.52);
    background: rgba(30, 64, 175, 0.25);
  }

  .window-row.suspended {
    border-color: rgba(251, 191, 36, 0.3);
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
    color: #f8fafc;
    font-size: 0.84rem;
  }

  .window-copy small {
    color: #94a3b8;
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
    .system-monitor {
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
