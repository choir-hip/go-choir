<script>
  import { createEventDispatcher, onDestroy, onMount } from 'svelte';
  import { isHeavyAppId } from './stores/desktop.js';
  import {
    getOverviewPreviewDecision,
    summarizeOverviewPreviewDecisions,
  } from './desktop-overview-preview.js';

  export let windows = [];
  export let activeWindowId = '';
  export let authenticated = false;
  export let previewDecisions = {};

  const dispatch = createEventDispatcher();

  let viewportWidth = 1280;
  let viewportHeight = 800;

  // Preserve the shared semantic window order supplied by the desktop store.
  // zIndex is session-local presentation state, so sorting the Overview by
  // zIndex makes two devices disagree whenever they have different active
  // windows. Spatial overlap still uses local zIndex in mapStyle().
  $: openWindows = (windows || [])
    .filter((win) => win.mode !== 'closed' && win.mode !== 'hidden');
  $: layeredWindows = openWindows;
  $: visibleCount = openWindows.filter((win) => win.mode !== 'minimized').length;
  $: suspendedCount = openWindows.filter((win) => win.restoreSuspended).length;
  $: heavyCount = openWindows.filter((win) => isHeavyAppId(win.appId)).length;
  $: mountedHeavyCount = openWindows.filter((win) =>
    isHeavyAppId(win.appId) && !win.restoreSuspended && win.mode !== 'minimized'
  ).length;
  $: minimizedCount = openWindows.filter((win) => win.mode === 'minimized').length;
  $: activeWindow = openWindows.find((win) => win.windowId === activeWindowId) || null;
  $: previewSummary = summarizeOverviewPreviewDecisions(previewDecisions);
  $: pressureLevel = mountedHeavyCount >= 6 || openWindows.length >= 14
    ? 'high'
    : mountedHeavyCount >= 3 || openWindows.length >= 10
    ? 'elevated'
    : 'steady';
  $: unavailablePreviewCount = previewSummary.suspended + previewSummary.redacted;

  function refreshViewport() {
    if (typeof window === 'undefined') return;
    viewportWidth = window.innerWidth || 1280;
    viewportHeight = window.innerHeight || 800;
  }

  function closeOverview() {
    dispatch('close');
  }

  function focusWindow(windowId) {
    dispatch('focuswindow', { windowId });
  }

  function minimizeWindow(windowId) {
    dispatch('minimizewindow', { windowId });
  }

  function closeWindow(windowId) {
    dispatch('closewindow', { windowId });
  }

  function suspendWindow(windowId) {
    dispatch('suspendwindow', { windowId });
  }

  function keepActiveOnly() {
    if (!activeWindowId) return;
    dispatch('keepactiveonly', { windowId: activeWindowId });
  }

  function handleKeydown(event) {
    if (event.key === 'Escape') closeOverview();
  }

  function modeLabel(win) {
    if (win.windowId === activeWindowId) return 'active';
    if (win.restoreSuspended) return 'suspended';
    if (win.mode === 'minimized') return 'minimized';
    if (win.mode === 'maximized') return 'maximized';
    return 'open';
  }

  function previewLabel(win) {
    const state = previewState(win);
    if (state === 'live') return 'live preview';
    if (state === 'redacted') return 'private';
    if (state === 'suspended') return 'parked';
    return 'card preview';
  }

  function resourceLabel(win) {
    if (!isHeavyAppId(win.appId)) return 'light';
    if (win.restoreSuspended || win.mode === 'minimized') return 'heavy, parked';
    return 'heavy, mounted';
  }

  function previewState(win) {
    return getOverviewPreviewDecision(previewDecisions, win.windowId).state;
  }

  function mapStyle(win) {
    const zValues = openWindows.map((item) => item.zIndex || 0);
    const minZ = Math.min(...zValues, 0);
    const maxZ = Math.max(...zValues, 1);
    const zRange = Math.max(1, maxZ - minZ);
    const left = Math.max(0, Math.min(92, ((win.x || 0) / viewportWidth) * 100));
    const top = Math.max(0, Math.min(88, ((win.y || 0) / viewportHeight) * 100));
    const dense = openWindows.length >= 10;
    const width = Math.max(dense ? 10 : 18, Math.min(92, ((win.width || 260) / viewportWidth) * 100));
    const height = Math.max(dense ? 9 : 18, Math.min(82, ((win.height || 180) / viewportHeight) * 100));
    const layer = 10 + Math.round(((win.zIndex || 0) - minZ) / zRange * 80);
    return `left:${left}%; top:${top}%; width:${width}%; height:${height}%; z-index:${layer};`;
  }

  onMount(() => {
    refreshViewport();
    window.addEventListener('resize', refreshViewport);
  });

  onDestroy(() => {
    window.removeEventListener('resize', refreshViewport);
  });
</script>

<svelte:window on:keydown={handleKeydown} />

<section class="desktop-overview" data-desktop-overview aria-modal="true" role="dialog" aria-labelledby="desktop-overview-title">
  <button class="overview-backdrop" type="button" aria-label="Close Desktop Overview" on:click={closeOverview}></button>

  <div class="overview-panel">
    <header class="overview-header">
      <div>
        <p class="overview-kicker">Desktop Overview</p>
        <h2 id="desktop-overview-title">{openWindows.length} open window{openWindows.length === 1 ? '' : 's'}</h2>
        <p
          class="overview-summary"
          data-overview-summary
          data-overview-window-count={openWindows.length}
          data-overview-visible-count={visibleCount}
          data-overview-heavy-count={heavyCount}
          data-overview-mounted-heavy-count={mountedHeavyCount}
          data-overview-suspended-count={suspendedCount}
          data-overview-minimized-count={minimizedCount}
          data-overview-live-preview-count={previewSummary.live}
          data-overview-card-preview-count={previewSummary.card}
          data-overview-redacted-preview-count={previewSummary.redacted}
          data-overview-suspended-preview-count={previewSummary.suspended}
          data-overview-pressure={pressureLevel}
        >
          Active: {activeWindow?.title || 'none'} · {minimizedCount} minimized · {previewSummary.live} live preview{previewSummary.live === 1 ? '' : 's'}
        </p>
      </div>
      <button class="overview-close" type="button" on:click={closeOverview} data-overview-close aria-label="Close Desktop Overview">Close</button>
    </header>

    {#if openWindows.length === 0}
      <div class="overview-empty" data-overview-empty>
        <h3>No open windows</h3>
        <p>Open an app from the Desk menu or desktop icons.</p>
      </div>
    {:else}
      <div class="overview-pressure" class:high={pressureLevel === 'high'} class:elevated={pressureLevel === 'elevated'} data-overview-pressure-panel>
        <div>
          <span>Active</span>
          <strong>{activeWindow?.title || 'none'}</strong>
        </div>
        <div>
          <span>Minimized</span>
          <strong>{minimizedCount}</strong>
        </div>
        <div>
          <span>Heavy apps</span>
          <strong>{mountedHeavyCount}/{heavyCount} mounted</strong>
        </div>
        <div>
          <span>Previews</span>
          <strong>{previewSummary.live}/{openWindows.length}</strong>
        </div>
      </div>

      <div class="overview-live-hint" data-overview-live-hint>
        <span>{visibleCount} visible</span>
        <span>{previewSummary.card} card preview{previewSummary.card === 1 ? '' : 's'}</span>
        <span>{unavailablePreviewCount} private or parked</span>
      </div>

      <div class="overview-body">
        <section class="overview-stage" aria-labelledby="overview-stage-title">
          <div class="section-heading">
            <p>Window Map</p>
            <h3 id="overview-stage-title">Current layout</h3>
          </div>
          <div
            class="overview-map"
            class:dense={openWindows.length >= 10}
            data-overview-map
            aria-label="Spatial map of open windows"
          >
            {#each openWindows as win (win.windowId)}
              <button
                class:active={win.windowId === activeWindowId}
                class:minimized={win.mode === 'minimized'}
                class:suspended={win.restoreSuspended}
                class="map-window"
                type="button"
                style={mapStyle(win)}
                on:click={() => focusWindow(win.windowId)}
                data-overview-map-window
                data-overview-map-window-id={win.windowId}
                data-overview-map-window-app-id={win.appId}
                data-overview-map-window-state={modeLabel(win)}
                data-overview-map-window-preview-state={previewState(win)}
                aria-label="Focus {win.title}"
              >
                <span>{win.icon || '□'}</span>
                <strong>{win.title}</strong>
                <em>{modeLabel(win)}</em>
              </button>
            {/each}
          </div>
        </section>

        <section class="overview-window-list" aria-labelledby="overview-window-list-title">
          <div class="section-heading">
            <p>Windows</p>
            <h3 id="overview-window-list-title">Switch or clean up</h3>
          </div>
          <div class="overview-cards" data-overview-cards>
            {#each layeredWindows as win, index (win.windowId)}
              <article
                class:active={win.windowId === activeWindowId}
                class:heavy={isHeavyAppId(win.appId)}
                class:minimized={win.mode === 'minimized'}
                class:suspended={win.restoreSuspended}
                class="overview-card"
                data-overview-card
                data-overview-card-window-id={win.windowId}
                data-overview-card-app-id={win.appId}
                data-overview-card-state={modeLabel(win)}
                data-overview-card-preview-state={previewState(win)}
                data-overview-card-heavy={isHeavyAppId(win.appId) ? 'true' : 'false'}
                data-overview-card-suspended={win.restoreSuspended ? 'true' : 'false'}
              >
                <button class="card-main" type="button" on:click={() => focusWindow(win.windowId)} data-overview-focus-window>
                  <span class="card-icon">{win.icon || '□'}</span>
                  <span class="card-copy">
                    <strong>{win.title}</strong>
                    <small>{modeLabel(win)} · {previewLabel(win)} · layer {layeredWindows.length - index}</small>
                  </span>
                </button>
                <div class="card-badges" aria-label="Window state">
                  {#if win.windowId === activeWindowId}
                    <span class="badge active-badge">active</span>
                  {/if}
                  <span class="badge">{previewLabel(win)}</span>
                  <span class="badge">{resourceLabel(win)}</span>
                  {#if win.mode === 'minimized'}
                    <span class="badge">minimized</span>
                  {/if}
                </div>
                <div class="card-actions">
                  <button class="primary-card-action" type="button" on:click={() => focusWindow(win.windowId)} data-overview-card-focus>Focus</button>
                  <button type="button" on:click={() => minimizeWindow(win.windowId)} disabled={win.mode === 'minimized'} data-overview-card-minimize>Minimize</button>
                  <button
                    type="button"
                    on:click={() => suspendWindow(win.windowId)}
                    disabled={!isHeavyAppId(win.appId) || win.restoreSuspended || win.windowId === activeWindowId}
                    data-overview-card-suspend
                  >
                    Park
                  </button>
                  <button class="danger" type="button" on:click={() => closeWindow(win.windowId)} data-overview-card-close>Close</button>
                </div>
              </article>
            {/each}
          </div>
        </section>
      </div>

      <div class="overview-actions" data-overview-actions>
        <button type="button" on:click={() => dispatch('suspendbackground')} data-overview-suspend-background>
          Park background apps
        </button>
        {#if authenticated}
          <button type="button" on:click={() => dispatch('opencomputemonitor')} data-overview-open-compute-monitor>
            Open Compute Monitor
          </button>
          <button
            type="button"
            on:click={keepActiveOnly}
            disabled={!activeWindowId}
            data-overview-keep-active-only
          >
            Keep active only
          </button>
          <button type="button" on:click={() => dispatch('clearsavedwindows')} data-overview-clear-saved>
            Clear saved windows
          </button>
        {/if}
      </div>
    {/if}
  </div>
</section>

<style>
  .desktop-overview {
    position: fixed;
    inset: 0;
    z-index: 13000;
    color: var(--choir-fg, #e5eefc);
    pointer-events: none;
  }

  .overview-backdrop {
    position: absolute;
    inset: 0;
    border: 0;
    background:
      radial-gradient(circle at 50% 38%, color-mix(in srgb, var(--choir-accent, #6d8dff) 16%, transparent), transparent 44%),
      color-mix(in srgb, var(--choir-bg, #050912) 76%, transparent);
    cursor: default;
    pointer-events: auto;
  }

  .overview-panel {
    position: absolute;
    inset:
      clamp(10px, 2.5vw, 24px)
      clamp(10px, 2.5vw, 24px)
      calc(var(--choir-prompt-surface-bottom-offset, 64px) + clamp(10px, 2.5vw, 24px))
      clamp(10px, 2.5vw, 24px);
    display: grid;
    grid-template-rows: auto auto auto minmax(0, 1fr) auto;
    gap: 0.7rem;
    overflow: auto;
    border: 0;
    border-radius: 24px;
    background-color: var(--choir-panel, rgba(15, 23, 42, 0.94));
    background-image: var(--choir-sheet-bg, none);
    box-shadow: var(--choir-shadow-floating, 0 28px 90px rgba(0, 0, 0, 0.42));
    padding: clamp(0.8rem, 2vw, 1.1rem);
    pointer-events: auto;
    scrollbar-width: thin;
  }

  .overview-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 1rem;
    border-radius: 18px;
    background: color-mix(in srgb, var(--choir-panel, #0d1628) 90%, transparent);
    box-shadow: var(--choir-shadow-soft, 0 18px 54px rgba(0, 0, 0, 0.28));
    padding: 0.85rem 0.95rem;
  }

  .overview-kicker {
    margin: 0 0 0.2rem;
    color: var(--choir-accent-2, #7dd3fc);
    font-size: 0.74rem;
    font-weight: 850;
    letter-spacing: 0.16em;
    text-transform: uppercase;
  }

  .overview-header h2 {
    margin: 0;
    color: var(--choir-fg, #f8fafc);
    font-size: clamp(1.35rem, 4vw, 2rem);
    line-height: 1.05;
  }

  .overview-summary {
    margin: 0.28rem 0 0;
    color: var(--choir-muted, #aebbd3);
    font-size: 0.9rem;
  }

  .overview-pressure {
    display: grid;
    grid-template-columns: repeat(4, minmax(0, 1fr));
    gap: 0.55rem;
  }

  .overview-pressure > div {
    min-width: 0;
    border: 0;
    border-radius: 14px;
    background: color-mix(in srgb, var(--choir-panel-soft, rgba(18, 31, 55, 0.72)) 92%, transparent);
    box-shadow: var(--choir-control-shadow, 0 12px 30px rgba(0, 0, 0, 0.16));
    padding: 0.66rem 0.76rem;
  }

  .overview-live-hint {
    justify-self: start;
    display: flex;
    flex-wrap: wrap;
    gap: 0.4rem;
  }

  .overview-live-hint span {
    border: 0;
    border-radius: 999px;
    background: color-mix(in srgb, var(--choir-selected, rgba(91, 123, 255, 0.22)) 72%, transparent);
    color: var(--choir-fg, #cbd5e1);
    font-size: 0.74rem;
    font-weight: 760;
    padding: 0.35rem 0.55rem;
  }

  .overview-pressure span,
  .overview-pressure strong {
    display: block;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .overview-pressure span {
    color: var(--choir-muted, #94a3b8);
    font-size: 0.68rem;
    font-weight: 820;
    letter-spacing: 0.1em;
    text-transform: uppercase;
  }

  .overview-pressure strong {
    margin-top: 0.18rem;
    color: var(--choir-fg, #dbeafe);
    font-size: 0.94rem;
  }

  .overview-pressure.elevated > div:first-child {
    border-color: rgba(251, 191, 36, 0.36);
  }

  .overview-pressure.high > div:first-child {
    border-color: rgba(248, 113, 113, 0.44);
  }

  .overview-close,
  .overview-actions button,
  .card-actions button {
    border: 0;
    border-radius: 999px;
    background: var(--choir-control-bg, rgba(15, 23, 42, 0.74));
    color: var(--choir-fg, #dbeafe);
    box-shadow: var(--choir-control-shadow, 0 12px 32px rgba(0, 0, 0, 0.18));
    cursor: pointer;
    font: inherit;
    font-size: 0.82rem;
    font-weight: 780;
    min-height: 2.35rem;
    padding: 0.45rem 0.7rem;
  }

  .overview-close:hover,
  .overview-actions button:hover,
  .card-actions button:hover:not(:disabled) {
    background: var(--choir-selected, rgba(30, 64, 175, 0.38));
  }

  button:disabled {
    cursor: not-allowed;
    opacity: 0.44;
  }

  .overview-body {
    display: grid;
    grid-template-columns: minmax(18rem, 0.9fr) minmax(24rem, 1.25fr);
    gap: 0.75rem;
    min-height: 0;
    overflow: auto;
  }

  .overview-stage,
  .overview-window-list {
    min-width: 0;
    border-radius: 18px;
    background: color-mix(in srgb, var(--choir-panel, rgba(15, 23, 42, 0.74)) 88%, transparent);
    box-shadow: var(--choir-shadow-soft, 0 16px 42px rgba(0, 0, 0, 0.2));
    padding: 0.8rem;
  }

  .section-heading {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    gap: 0.75rem;
    margin-bottom: 0.62rem;
  }

  .section-heading p,
  .section-heading h3 {
    margin: 0;
  }

  .section-heading p {
    color: var(--choir-accent-2, #7dd3fc);
    font-size: 0.7rem;
    font-weight: 840;
    letter-spacing: 0.12em;
    text-transform: uppercase;
  }

  .section-heading h3 {
    color: var(--choir-fg, #f8fafc);
    font-size: 1rem;
  }

  .overview-map {
    position: relative;
    overflow: hidden;
    min-height: clamp(180px, 34vh, 320px);
    border: 0;
    border-radius: 14px;
    background:
      linear-gradient(color-mix(in srgb, var(--choir-border, rgba(148, 163, 184, 0.16)) 42%, transparent) 1px, transparent 1px),
      linear-gradient(90deg, color-mix(in srgb, var(--choir-border, rgba(148, 163, 184, 0.16)) 42%, transparent) 1px, transparent 1px),
      color-mix(in srgb, var(--choir-bg, #020617) 78%, var(--choir-panel, #0d1628));
    background-size: 32px 32px;
    pointer-events: auto;
  }

  .overview-map.dense {
    min-height: 150px;
  }

  .map-window {
    position: absolute;
    display: grid;
    grid-template-columns: 1.35rem minmax(0, 1fr);
    align-content: start;
    align-items: center;
    gap: 0.35rem;
    min-width: 4.8rem;
    min-height: 3.2rem;
    overflow: hidden;
    border: 0;
    border-radius: 9px;
    background: color-mix(in srgb, var(--choir-panel-strong, #13213b) 92%, transparent);
    box-shadow: 0 10px 26px color-mix(in srgb, var(--choir-bg, #020617) 36%, transparent);
    color: var(--choir-fg, #dbeafe);
    cursor: pointer;
    pointer-events: auto;
    padding: 0.45rem;
    text-align: left;
  }

  .map-window > span {
    display: inline-grid;
    place-items: center;
    width: 1.35rem;
    height: 1.35rem;
    font-size: 1rem;
    line-height: 1;
  }

  .map-window.active {
    background: var(--choir-selected, rgba(59, 130, 246, 0.24));
    box-shadow:
      0 14px 34px color-mix(in srgb, var(--choir-accent, #6d8dff) 18%, transparent),
      inset 0 0 28px color-mix(in srgb, var(--choir-accent, #6d8dff) 16%, transparent);
  }

  .map-window.minimized,
  .map-window.suspended {
    opacity: 0.64;
  }

  .map-window em {
    grid-column: 1 / -1;
    min-width: 0;
    overflow: hidden;
    color: var(--choir-muted, #94a3b8);
    font-size: 0.6rem;
    font-style: normal;
    font-weight: 760;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .map-window strong {
    min-width: 0;
    overflow: hidden;
    font-size: 0.72rem;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .overview-actions {
    display: flex;
    flex-wrap: wrap;
    gap: 0.45rem;
    position: relative;
    z-index: 2;
  }

  .overview-cards {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(min(18rem, 100%), 1fr));
    gap: 0.65rem;
    min-height: 0;
    overflow: visible;
    padding: 0;
  }

  .overview-card {
    display: grid;
    gap: 0.6rem;
    min-width: 0;
    border: 0;
    border-radius: 14px;
    background: color-mix(in srgb, var(--choir-panel-soft, rgba(15, 23, 42, 0.76)) 92%, transparent);
    box-shadow: var(--choir-control-shadow, 0 12px 30px rgba(0, 0, 0, 0.18));
    padding: 0.65rem;
  }

  .overview-card[data-overview-card-preview-state='live'] {
    background: color-mix(in srgb, var(--choir-panel-soft, rgba(8, 18, 32, 0.58)) 86%, var(--choir-selected, transparent));
  }

  .overview-card.active {
    background: var(--choir-selected, rgba(30, 64, 175, 0.22));
    box-shadow:
      var(--choir-shadow-soft, 0 16px 42px rgba(0, 0, 0, 0.2)),
      inset 0 0 34px color-mix(in srgb, var(--choir-accent, #6d8dff) 12%, transparent);
  }

  .overview-card.heavy {
    border-color: rgba(125, 211, 252, 0.22);
  }

  .overview-card.suspended {
    border-style: dashed;
  }

  .overview-card.minimized {
    opacity: 0.72;
  }

  .card-main {
    display: grid;
    grid-template-columns: 2rem minmax(0, 1fr);
    align-items: center;
    gap: 0.55rem;
    border: 0;
    background: transparent;
    color: inherit;
    cursor: pointer;
    padding: 0;
    text-align: left;
  }

  .card-icon {
    display: grid;
    place-items: center;
    width: 2rem;
    height: 2rem;
    border-radius: 10px;
    background: rgba(96, 165, 250, 0.13);
    background: color-mix(in srgb, var(--choir-accent, #6d8dff) 16%, transparent);
    font-size: 1.15rem;
    line-height: 1;
  }

  .card-copy {
    display: grid;
    gap: 0.12rem;
    min-width: 0;
  }

  .card-copy strong,
  .card-copy small {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .card-copy strong {
    color: var(--choir-fg, #f8fafc);
    font-size: 0.93rem;
  }

  .card-copy small {
    color: var(--choir-muted, #94a3b8);
    font-size: 0.74rem;
  }

  .card-actions {
    display: flex;
    flex-wrap: wrap;
    gap: 0.4rem;
  }

  .card-badges {
    display: flex;
    flex-wrap: wrap;
    gap: 0.3rem;
  }

  .badge {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    border: 0;
    border-radius: 999px;
    background: color-mix(in srgb, var(--choir-selected, rgba(91, 123, 255, 0.22)) 62%, transparent);
    color: var(--choir-fg, #aebbd3);
    font-size: 0.66rem;
    font-weight: 820;
    letter-spacing: 0.02em;
    line-height: 1;
    padding: 0.22rem 0.43rem;
    text-transform: uppercase;
  }

  .active-badge {
    background: color-mix(in srgb, var(--choir-accent, #6d8dff) 24%, transparent);
  }

  .card-actions .danger {
    color: var(--choir-danger, #fecaca);
  }

  .primary-card-action {
    background: var(--choir-selected, rgba(91, 123, 255, 0.22)) !important;
  }

  .overview-empty {
    display: grid;
    place-content: center;
    min-height: 16rem;
    border: 0;
    border-radius: 14px;
    background: color-mix(in srgb, var(--choir-panel-soft, rgba(15, 23, 42, 0.72)) 92%, transparent);
    color: var(--choir-muted, #94a3b8);
    text-align: center;
  }

  .overview-empty h3 {
    margin: 0 0 0.3rem;
    color: var(--choir-fg, #f8fafc);
  }

  .overview-empty p {
    margin: 0;
  }

  @media (max-width: 768px) {
    .overview-panel {
      inset:
        8px
        8px
        calc(var(--choir-prompt-surface-bottom-offset, 64px) + 8px)
        8px;
      grid-template-rows: auto auto auto minmax(0, 1fr) auto;
      gap: 0.65rem;
      border-radius: 20px;
      padding: 0.7rem;
    }

    .overview-header {
      align-items: center;
      max-width: calc(100vw - 1.4rem);
      padding: 0.62rem 0.68rem;
    }

    .overview-header h2 {
      font-size: 1.35rem;
    }

    .overview-summary {
      font-size: 0.78rem;
    }

    .overview-close {
      min-width: 4.5rem;
    }

    .overview-actions {
      display: grid;
      grid-template-columns: 1fr;
    }

    .overview-actions button {
      min-width: 0;
      padding-inline: 0.4rem;
    }

    .card-actions button {
      flex: 1 1 calc(50% - 0.4rem);
      min-width: 6rem;
    }

    .overview-pressure {
      grid-template-columns: repeat(2, minmax(0, 1fr));
      gap: 0.38rem;
    }

    .overview-pressure > div {
      padding: 0.48rem 0.6rem;
    }

    .overview-pressure strong {
      font-size: 0.86rem;
    }

    .overview-live-hint {
      gap: 0.3rem;
    }

    .overview-live-hint span {
      font-size: 0.66rem;
      padding: 0.28rem 0.44rem;
    }

    .overview-body {
      grid-template-columns: 1fr;
      padding-bottom: 0.25rem;
    }

    .overview-stage,
    .overview-window-list {
      padding: 0.65rem;
    }

    .overview-map {
      min-height: 128px;
    }

    .map-window {
      grid-template-columns: 1fr;
      align-content: center;
      justify-items: center;
      gap: 0;
      min-width: 2.1rem;
      min-height: 2.1rem;
      padding: 0.25rem;
    }

    .map-window strong,
    .map-window em {
      position: absolute;
      width: 1px;
      height: 1px;
      overflow: hidden;
      clip: rect(0 0 0 0);
      white-space: nowrap;
    }

    .overview-cards {
      grid-template-columns: 1fr;
    }
  }
</style>
