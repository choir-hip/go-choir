<script>
  import { createEventDispatcher, onDestroy, onMount } from 'svelte';
  import { isHeavyAppId } from './stores/desktop.js';

  export let windows = [];
  export let activeWindowId = '';
  export let authenticated = false;

  const dispatch = createEventDispatcher();

  let viewportWidth = 1280;
  let viewportHeight = 800;

  $: openWindows = (windows || [])
    .filter((win) => win.mode !== 'closed' && win.mode !== 'hidden')
    .sort((a, b) => (a.zIndex || 0) - (b.zIndex || 0));
  $: visibleCount = openWindows.filter((win) => win.mode !== 'minimized').length;
  $: suspendedCount = openWindows.filter((win) => win.restoreSuspended).length;
  $: heavyCount = openWindows.filter((win) => isHeavyAppId(win.appId)).length;

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

  function handleKeydown(event) {
    if (event.key === 'Escape') closeOverview();
  }

  function modeLabel(win) {
    if (win.restoreSuspended) return 'suspended';
    if (win.mode === 'minimized') return 'minimized';
    if (win.mode === 'maximized') return 'maximized';
    return win.windowId === activeWindowId ? 'active' : 'open';
  }

  function mapStyle(win) {
    const zValues = openWindows.map((item) => item.zIndex || 0);
    const minZ = Math.min(...zValues, 0);
    const maxZ = Math.max(...zValues, 1);
    const zRange = Math.max(1, maxZ - minZ);
    const left = Math.max(0, Math.min(92, ((win.x || 0) / viewportWidth) * 100));
    const top = Math.max(0, Math.min(88, ((win.y || 0) / viewportHeight) * 100));
    const width = Math.max(18, Math.min(92, ((win.width || 260) / viewportWidth) * 100));
    const height = Math.max(18, Math.min(82, ((win.height || 180) / viewportHeight) * 100));
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
        <h2 id="desktop-overview-title">Open windows</h2>
        <p class="overview-summary" data-overview-summary>
          {openWindows.length} open, {visibleCount} visible, {heavyCount} heavy, {suspendedCount} suspended
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
      <div class="overview-map" data-overview-map aria-label="Spatial map of open windows">
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
            aria-label="Focus {win.title}"
          >
            <span>{win.icon || '□'}</span>
            <strong>{win.title}</strong>
          </button>
        {/each}
      </div>

      <div class="overview-actions" data-overview-actions>
        <button type="button" on:click={() => dispatch('suspendbackground')} data-overview-suspend-background>
          Suspend background apps
        </button>
        {#if authenticated}
          <button type="button" on:click={() => dispatch('opencomputemonitor')} data-overview-open-compute-monitor>
            Open Compute Monitor
          </button>
          <button type="button" on:click={() => dispatch('clearsavedwindows')} data-overview-clear-saved>
            Clear saved windows
          </button>
        {/if}
      </div>

      <div class="overview-cards" data-overview-cards>
        {#each [...openWindows].reverse() as win (win.windowId)}
          <article
            class:active={win.windowId === activeWindowId}
            class:suspended={win.restoreSuspended}
            class="overview-card"
            data-overview-card
            data-overview-card-window-id={win.windowId}
            data-overview-card-app-id={win.appId}
          >
            <button class="card-main" type="button" on:click={() => focusWindow(win.windowId)} data-overview-focus-window>
              <span class="card-icon">{win.icon || '□'}</span>
              <span class="card-copy">
                <strong>{win.title}</strong>
                <small>{win.appId} · {modeLabel(win)}</small>
              </span>
            </button>
            <div class="card-actions">
              <button type="button" on:click={() => focusWindow(win.windowId)} data-overview-card-focus>Focus</button>
              <button type="button" on:click={() => minimizeWindow(win.windowId)} disabled={win.mode === 'minimized'} data-overview-card-minimize>Minimize</button>
              <button type="button" on:click={() => suspendWindow(win.windowId)} disabled={!isHeavyAppId(win.appId) || win.restoreSuspended} data-overview-card-suspend>Suspend</button>
              <button class="danger" type="button" on:click={() => closeWindow(win.windowId)} data-overview-card-close>Close</button>
            </div>
          </article>
        {/each}
      </div>
    {/if}
  </div>
</section>

<style>
  .desktop-overview {
    position: fixed;
    inset: 0;
    z-index: 10000;
    color: #e5eefc;
  }

  .overview-backdrop {
    position: absolute;
    inset: 0;
    border: 0;
    background: rgba(3, 7, 18, 0.72);
    cursor: default;
  }

  .overview-panel {
    position: absolute;
    inset: clamp(12px, 3vw, 28px);
    display: grid;
    grid-template-rows: auto minmax(160px, 1fr) auto minmax(0, 1fr);
    gap: 0.85rem;
    overflow: hidden;
    border: 1px solid rgba(96, 165, 250, 0.35);
    border-radius: 18px;
    background:
      linear-gradient(145deg, rgba(15, 23, 42, 0.96), rgba(2, 6, 23, 0.98)),
      rgba(15, 23, 42, 0.98);
    box-shadow: 0 28px 80px rgba(0, 0, 0, 0.56), 0 0 0 1px rgba(15, 23, 42, 0.8);
    padding: clamp(0.8rem, 2vw, 1.1rem);
    backdrop-filter: blur(18px);
  }

  .overview-header {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 1rem;
  }

  .overview-kicker {
    margin: 0 0 0.2rem;
    color: #7dd3fc;
    font-size: 0.74rem;
    font-weight: 850;
    letter-spacing: 0.16em;
    text-transform: uppercase;
  }

  .overview-header h2 {
    margin: 0;
    color: #f8fafc;
    font-size: clamp(1.35rem, 4vw, 2rem);
    line-height: 1.05;
  }

  .overview-summary {
    margin: 0.28rem 0 0;
    color: #aebbd3;
    font-size: 0.9rem;
  }

  .overview-close,
  .overview-actions button,
  .card-actions button {
    border: 1px solid rgba(148, 163, 184, 0.24);
    border-radius: 10px;
    background: rgba(15, 23, 42, 0.74);
    color: #dbeafe;
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
    border-color: rgba(125, 211, 252, 0.55);
    background: rgba(30, 64, 175, 0.38);
  }

  button:disabled {
    cursor: not-allowed;
    opacity: 0.44;
  }

  .overview-map {
    position: relative;
    overflow: hidden;
    min-height: 180px;
    border: 1px solid rgba(148, 163, 184, 0.16);
    border-radius: 14px;
    background:
      linear-gradient(rgba(148, 163, 184, 0.05) 1px, transparent 1px),
      linear-gradient(90deg, rgba(148, 163, 184, 0.05) 1px, transparent 1px),
      rgba(2, 6, 23, 0.8);
    background-size: 32px 32px;
  }

  .map-window {
    position: absolute;
    display: grid;
    grid-template-columns: 1.2rem minmax(0, 1fr);
    align-content: start;
    align-items: center;
    gap: 0.35rem;
    min-width: 4.8rem;
    min-height: 3.2rem;
    overflow: hidden;
    border: 1px solid rgba(148, 163, 184, 0.28);
    border-radius: 9px;
    background: rgba(15, 23, 42, 0.88);
    box-shadow: 0 10px 28px rgba(0, 0, 0, 0.3);
    color: #dbeafe;
    cursor: pointer;
    padding: 0.45rem;
    text-align: left;
  }

  .map-window.active {
    border-color: rgba(59, 130, 246, 0.9);
    box-shadow: 0 0 0 2px rgba(59, 130, 246, 0.28), 0 14px 34px rgba(37, 99, 235, 0.28);
  }

  .map-window.minimized,
  .map-window.suspended {
    opacity: 0.64;
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
  }

  .overview-cards {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(min(18rem, 100%), 1fr));
    gap: 0.65rem;
    min-height: 0;
    overflow: auto;
    padding-right: 0.15rem;
  }

  .overview-card {
    display: grid;
    gap: 0.6rem;
    min-width: 0;
    border: 1px solid rgba(148, 163, 184, 0.16);
    border-radius: 14px;
    background: rgba(15, 23, 42, 0.72);
    padding: 0.65rem;
  }

  .overview-card.active {
    border-color: rgba(96, 165, 250, 0.6);
    background: rgba(30, 64, 175, 0.22);
  }

  .overview-card.suspended {
    border-style: dashed;
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
    align-items: center;
    justify-content: center;
    width: 2rem;
    height: 2rem;
    border-radius: 10px;
    background: rgba(96, 165, 250, 0.13);
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
    color: #f8fafc;
    font-size: 0.93rem;
  }

  .card-copy small {
    color: #94a3b8;
    font-size: 0.74rem;
  }

  .card-actions {
    display: flex;
    flex-wrap: wrap;
    gap: 0.4rem;
  }

  .card-actions .danger {
    border-color: rgba(248, 113, 113, 0.32);
    color: #fecaca;
  }

  .overview-empty {
    display: grid;
    place-content: center;
    min-height: 16rem;
    border: 1px solid rgba(148, 163, 184, 0.16);
    border-radius: 14px;
    color: #94a3b8;
    text-align: center;
  }

  .overview-empty h3 {
    margin: 0 0 0.3rem;
    color: #f8fafc;
  }

  .overview-empty p {
    margin: 0;
  }

  @media (max-width: 768px) {
    .overview-panel {
      inset: 8px;
      grid-template-rows: auto minmax(180px, 0.72fr) auto minmax(0, 1fr);
      gap: 0.65rem;
      border-radius: 14px;
      padding: 0.7rem;
    }

    .overview-header {
      align-items: center;
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
      grid-template-columns: repeat(3, minmax(0, 1fr));
    }

    .overview-actions button {
      min-width: 0;
      padding-inline: 0.4rem;
    }

    .card-actions button {
      flex: 1 1 calc(50% - 0.4rem);
      min-width: 6rem;
    }
  }
</style>
