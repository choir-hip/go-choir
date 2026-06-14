<script lang="ts">
  import { onMount } from 'svelte';
  import { fetchPulseSummary } from './pulse.js';

  let summary: any = null;
  let loading = false;
  let error = '';

  $: accounts = summary?.accounts || {};
  $: accountClasses = accounts?.by_class || {};
  $: activity = summary?.activity || {};
  $: computers = summary?.computers || {};
  $: storage = summary?.storage || {};
  $: reliability = summary?.reliability || {};
  $: privacy = summary?.privacy || {};
  $: freshness = summary?.freshness || {};
  $: classifier = summary?.classifier || {};
  $: realComputerStates = computers?.real_primary_by_state || {};
  $: realComputerTotal = Number(computers?.real_primary_total || 0);
  $: realComputerUsable = Number(computers?.real_primary_usable || 0);
  $: realComputerHealth = realComputerTotal > 0 ? Math.round((realComputerUsable / realComputerTotal) * 100) : 0;
  $: realAccountCount = Number(accountClasses?.real || 0);
  $: codexAccountCount = Number(accountClasses?.codex_agentic_test || 0);
  $: protectedTestAccountCount = Number(accountClasses?.protected_test || 0);
  $: internalAccountCount = Number(accountClasses?.internal || 0);
  $: unknownAccountCount = Number(accountClasses?.unknown || 0);

  onMount(() => {
    refresh();
  });

  async function refresh() {
    loading = true;
    error = '';
    try {
      summary = await fetchPulseSummary();
    } catch (err) {
      error = err instanceof Error ? err.message : 'Could not load Pulse';
    } finally {
      loading = false;
    }
  }

  function formatNumber(value: unknown) {
    const num = Number(value);
    if (!Number.isFinite(num)) return '0';
    return new Intl.NumberFormat().format(num);
  }

  function formatBytes(value: unknown) {
    const num = Number(value);
    if (!Number.isFinite(num) || num <= 0) return '0 B';
    const units = ['B', 'KiB', 'MiB', 'GiB', 'TiB'];
    let scaled = num;
    let unit = 0;
    while (scaled >= 1024 && unit < units.length - 1) {
      scaled /= 1024;
      unit += 1;
    }
    return `${scaled >= 10 || unit === 0 ? scaled.toFixed(0) : scaled.toFixed(1)} ${units[unit]}`;
  }

  function formatPercent(value: unknown) {
    const num = Number(value);
    if (!Number.isFinite(num) || num < 0) return '0%';
    return `${num.toFixed(1)}%`;
  }

  function barWidth(value: unknown, max = 100) {
    const num = Number(value);
    if (!Number.isFinite(num) || num <= 0) return '0%';
    return `${Math.max(2, Math.min(100, (num / max) * 100))}%`;
  }

  function timestamp(value: string) {
    if (!value) return 'pending';
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return value;
    return date.toLocaleString(undefined, { dateStyle: 'medium', timeStyle: 'short' });
  }

  function stateEntries(states: Record<string, number>) {
    return Object.entries(states || {}).sort(([a], [b]) => a.localeCompare(b));
  }
</script>

<div class="pulse-app" data-pulse-app data-public-readonly="true">
  <header class="pulse-top" data-pulse-summary>
    <div>
      <p class="eyebrow">Choir Pulse</p>
      <h1>Public aggregate health</h1>
      <p class="summary-line">
        {privacy?.data_mode || 'aggregate-only'} / {privacy?.surface || 'public-readonly'} / {freshness?.generated_by || 'vmctl'}
      </p>
    </div>
    <button type="button" class="refresh-button" on:click={refresh} disabled={loading} aria-label="Refresh Pulse">
      {loading ? '...' : 'Refresh'}
    </button>
  </header>

  {#if error}
    <div class="notice error" role="alert">{error}</div>
  {/if}

  <section class="metric-strip" data-pulse-metrics>
    <article class="metric-card">
      <span class="metric-label">Real users</span>
      <strong>{formatNumber(realAccountCount)}</strong>
      <div class="meter real"><span style="width:{barWidth(realAccountCount, Math.max(accounts?.total || 1, 1))}"></span></div>
      <small>+{formatNumber(accounts?.new_real_last_24h)} 24h / +{formatNumber(accounts?.new_real_last_7d)} 7d / +{formatNumber(accounts?.new_real_last_30d)} 30d</small>
    </article>
    <article class="metric-card">
      <span class="metric-label">Active real users</span>
      <strong>{formatNumber(activity?.real_active_last_24h)}</strong>
      <div class="meter activity"><span style="width:{barWidth(activity?.real_active_last_24h, Math.max(realAccountCount, 1))}"></span></div>
      <small>{formatNumber(activity?.real_active_last_7d)} 7d / {formatNumber(activity?.real_active_last_30d)} 30d</small>
    </article>
    <article class="metric-card">
      <span class="metric-label">Primary computers</span>
      <strong>{formatNumber(realComputerUsable)} / {formatNumber(realComputerTotal)}</strong>
      <div class="meter compute"><span style="width:{barWidth(realComputerHealth)}"></span></div>
      <small>{realComputerHealth}% usable</small>
    </article>
    <article class="metric-card">
      <span class="metric-label">VM state</span>
      <strong>{formatBytes(storage?.vm_state_bytes_total)}</strong>
      <div class="meter storage"><span style="width:{barWidth(storage?.vm_state_filesystem?.used_percent)}"></span></div>
      <small>{formatPercent(storage?.vm_state_filesystem?.used_percent)} filesystem used</small>
    </article>
    <article class="metric-card" class:warn-card={reliability?.real_primary_failed || reliability?.real_primary_booting || reliability?.real_primary_inaccessible}>
      <span class="metric-label">Reliability</span>
      <strong>{formatNumber((reliability?.real_primary_failed || 0) + (reliability?.real_primary_booting || 0) + (reliability?.real_primary_inaccessible || 0))}</strong>
      <div class="meter reliability"><span style="width:{barWidth((reliability?.real_primary_failed || 0) + (reliability?.real_primary_booting || 0) + (reliability?.real_primary_inaccessible || 0), Math.max(realComputerTotal, 1))}"></span></div>
      <small>failed / booting / inaccessible</small>
    </article>
  </section>

  <div class="pulse-grid">
    <section class="panel" data-pulse-accounts>
      <div class="panel-heading">
        <h2>Accounts</h2>
        <span class="chip">{accounts?.auth_data_available ? 'live auth db' : 'partial'}</span>
      </div>
      <dl class="facts">
        <div><dt>Real</dt><dd>{formatNumber(realAccountCount)}</dd></div>
        <div><dt>Codex test</dt><dd>{formatNumber(codexAccountCount)}</dd></div>
        <div><dt>Protected test</dt><dd>{formatNumber(protectedTestAccountCount)}</dd></div>
        <div><dt>Internal</dt><dd>{formatNumber(internalAccountCount)}</dd></div>
        <div><dt>Unknown</dt><dd>{formatNumber(unknownAccountCount)}</dd></div>
        <div><dt>Total</dt><dd>{formatNumber(accounts?.total)}</dd></div>
      </dl>
      <p class="compact-copy">{classifier?.version || 'classifier pending'} / {classifier?.unknown_policy || 'unknowns excluded from real counts'}</p>
    </section>

    <section class="panel" data-pulse-computers>
      <div class="panel-heading">
        <h2>Real Computers</h2>
        <span class="chip">{formatNumber(computers?.total_ownerships)} ownerships</span>
      </div>
      <div class="row-list">
        {#each stateEntries(realComputerStates) as [state, value]}
          <div class="data-row">
            <span>{state}</span>
            <strong>{formatNumber(value)}</strong>
          </div>
        {:else}
          <div class="data-row muted">
            <span>state</span>
            <strong>pending</strong>
          </div>
        {/each}
      </div>
    </section>

    <section class="panel wide" data-pulse-storage>
      <div class="panel-heading">
        <h2>Storage</h2>
        <span class="chip">sampled {timestamp(freshness?.storage_sampled_at)}</span>
      </div>
      <div class="storage-grid">
        <div>
          <h3>Filesystems</h3>
          <div class="row-list">
            <div class="data-row">
              <span>VM state</span>
              <strong>{formatBytes(storage?.vm_state_filesystem?.used_bytes)} / {formatBytes(storage?.vm_state_filesystem?.total_bytes)}</strong>
            </div>
            <div class="data-row">
              <span>Nix store mount</span>
              <strong>{formatBytes(storage?.nix_store_filesystem?.used_bytes)} / {formatBytes(storage?.nix_store_filesystem?.total_bytes)}</strong>
            </div>
            <div class="data-row">
              <span>Manual recovery snapshots</span>
              <strong>{formatNumber(storage?.manual_recovery_snapshot_count)} / {formatBytes(storage?.manual_recovery_snapshot_bytes)}</strong>
            </div>
          </div>
        </div>
        <div>
          <h3>VM state by account class</h3>
          <div class="row-list">
            {#each Object.entries(storage?.vm_state_bytes_by_class || {}) as [klass, bytes]}
              <div class="data-row">
                <span>{klass}</span>
                <strong>{formatBytes(bytes)}</strong>
              </div>
            {/each}
          </div>
        </div>
      </div>
    </section>

    <section class="panel" data-pulse-privacy>
      <div class="panel-heading">
        <h2>Public Boundary</h2>
        <span class="chip">{privacy?.no_private_superset ? 'no superset' : 'review'}</span>
      </div>
      <div class="row-list">
        <div class="data-row"><span>Row analytics</span><strong>{privacy?.no_row_level_analytics ? 'absent' : 'review'}</strong></div>
        <div class="data-row"><span>User identity output</span><strong>{privacy?.no_user_identity_output ? 'absent' : 'review'}</strong></div>
        <div class="data-row"><span>Protected test count</span><strong>{formatNumber(classifier?.protected_test_count)}</strong></div>
      </div>
    </section>

    <section class="panel" data-pulse-not-collected>
      <div class="panel-heading">
        <h2>Not Collected</h2>
        <span class="chip">{formatNumber((reliability?.not_collected || privacy?.excluded_data || []).length)} classes</span>
      </div>
      <ul class="plain-list">
        {#each (reliability?.not_collected || privacy?.excluded_data || []) as item}
          <li>{item}</li>
        {/each}
      </ul>
    </section>
  </div>

  <section class="panel event-panel" data-pulse-evidence>
    <div class="panel-heading">
      <h2>Freshness</h2>
      <span class="chip">{summary?.status || 'loading'}</span>
    </div>
    <ul class="event-list">
      <li>generated: {timestamp(summary?.generated_at)}</li>
      <li>ownerships: {timestamp(freshness?.ownerships_read_at)}</li>
      <li>auth data: {accounts?.auth_data_available ? timestamp(freshness?.auth_db_read_at) : 'unavailable'}</li>
      <li>warnings: {formatNumber(summary?.warnings?.length || 0)}</li>
    </ul>
  </section>
</div>

<style>
  .pulse-app {
    height: 100%;
    min-height: 0;
    overflow: auto;
    padding: 1rem;
    background: var(--choir-bg);
    color: var(--choir-text-primary);
  }

  .pulse-top {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 1rem;
    margin-bottom: 0.85rem;
  }

  .eyebrow,
  h1,
  h2,
  h3,
  p {
    margin: 0;
  }

  .eyebrow {
    color: var(--choir-text-muted);
    font-size: 0.72rem;
    font-weight: 850;
    letter-spacing: 0.12em;
    text-transform: uppercase;
  }

  h1 {
    margin-top: 0.2rem;
    color: var(--choir-text-primary);
    font-size: 1.64rem;
    letter-spacing: 0;
    line-height: 1.08;
  }

  h2 {
    color: var(--choir-text-primary);
    font-size: 0.98rem;
    letter-spacing: 0;
  }

  h3 {
    margin-bottom: 0.5rem;
    color: var(--choir-text-primary);
    font-size: 0.82rem;
    letter-spacing: 0;
  }

  .summary-line,
  .compact-copy,
  .event-list,
  .plain-list {
    color: var(--choir-text-muted);
    font-size: 0.84rem;
    line-height: 1.42;
  }

  .summary-line {
    margin-top: 0.35rem;
  }

  .refresh-button {
    min-height: 2.25rem;
    border: 1px solid var(--choir-border-strong);
    border-radius: 8px;
    background: var(--choir-surface-control);
    color: var(--choir-text-primary);
    padding: 0 0.75rem;
    font-weight: 780;
    cursor: pointer;
  }

  .refresh-button:disabled {
    cursor: wait;
    opacity: 0.68;
  }

  .notice {
    margin-bottom: 0.75rem;
    border: 1px solid var(--choir-border-strong);
    border-radius: 8px;
    background: var(--choir-surface-card);
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
    grid-template-columns: repeat(auto-fit, minmax(10.75rem, 1fr));
    gap: 0.65rem;
    margin-bottom: 0.75rem;
  }

  .metric-card,
  .panel {
    border: 1px solid var(--choir-border-strong);
    border-radius: 8px;
    background: var(--choir-surface-card);
  }

  .metric-card {
    display: grid;
    gap: 0.34rem;
    padding: 0.72rem;
  }

  .metric-label {
    color: var(--choir-text-muted);
    font-size: 0.68rem;
    font-weight: 840;
    letter-spacing: 0.08em;
    text-transform: uppercase;
  }

  .metric-card strong {
    color: var(--choir-text-primary);
    font-size: 1.2rem;
    line-height: 1;
  }

  .metric-card small {
    color: var(--choir-text-muted);
    font-size: 0.72rem;
  }

  .meter {
    overflow: hidden;
    height: 0.45rem;
    border-radius: 999px;
    background: var(--choir-border);
  }

  .meter span {
    display: block;
    height: 100%;
    border-radius: inherit;
    background: var(--choir-status-success);
  }

  .meter.activity span {
    background: var(--choir-chart-2);
  }

  .meter.compute span {
    background: var(--choir-status-success);
  }

  .meter.storage span {
    background: var(--choir-status-warning);
  }

  .meter.reliability span {
    background: var(--choir-status-danger);
  }

  .metric-card.warn-card {
    outline: 1px solid color-mix(in srgb, var(--choir-status-warning) 55%, transparent);
  }

  .pulse-grid {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 0.75rem;
    margin-bottom: 0.75rem;
  }

  .panel {
    padding: 0.85rem;
  }

  .panel.wide,
  .event-panel {
    grid-column: 1 / -1;
  }

  .panel-heading {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.75rem;
    margin-bottom: 0.7rem;
  }

  .chip {
    display: inline-flex;
    align-items: center;
    min-height: 1.55rem;
    border: 1px solid var(--choir-border-strong);
    border-radius: 999px;
    background: var(--choir-surface-control);
    color: var(--choir-text-muted);
    padding: 0.22rem 0.58rem;
    font-size: 0.7rem;
    font-weight: 820;
    text-transform: uppercase;
    white-space: nowrap;
  }

  .facts {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 0.5rem 0.75rem;
    margin: 0 0 0.75rem;
  }

  dt {
    color: var(--choir-text-muted);
    font-size: 0.66rem;
    font-weight: 820;
    letter-spacing: 0.08em;
    text-transform: uppercase;
  }

  dd {
    margin: 0.16rem 0 0;
    overflow: hidden;
    color: var(--choir-text-primary);
    font-size: 0.9rem;
    font-weight: 760;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .row-list,
  .storage-grid {
    display: grid;
    gap: 0.5rem;
  }

  .storage-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 0.9rem;
  }

  .data-row {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    gap: 0.75rem;
    min-height: 1.85rem;
    border-bottom: 1px solid var(--choir-border);
    color: var(--choir-text-muted);
    font-size: 0.84rem;
  }

  .data-row strong {
    overflow: hidden;
    color: var(--choir-text-primary);
    font-size: 0.86rem;
    text-align: right;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .data-row.muted strong {
    color: var(--choir-text-muted);
  }

  .plain-list,
  .event-list {
    display: grid;
    gap: 0.38rem;
    margin: 0;
    padding-left: 1rem;
  }

  @media (max-width: 720px) {
    .pulse-top {
      display: grid;
    }

    .refresh-button {
      width: 100%;
    }

    .pulse-grid,
    .storage-grid {
      grid-template-columns: 1fr;
    }

    .facts {
      grid-template-columns: 1fr;
    }
  }
</style>
