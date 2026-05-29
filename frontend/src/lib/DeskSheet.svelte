<script lang="ts">
  import { createEventDispatcher } from 'svelte';

  export let normalizedPlacement: 'top' | 'bottom' = 'bottom';
  export let deskApps: any[] = [];
  export let openWindows: any[] = [];
  export let authenticated = false;
  export let currentUser: any = null;

  const dispatch = createEventDispatcher();
</script>

<button class="desk-sheet-backdrop" data-desk-sheet-backdrop type="button" aria-label="Close Desk" on:click={() => dispatch('close')}></button>
<section class="desk-sheet placement-{normalizedPlacement}" data-desk-sheet role="dialog" aria-label="Desk">
  <div class="sheet-handle" aria-hidden="true"></div>
  <header>
    <div>
      <p>Desk</p>
      <h2>{openWindows.length} open</h2>
    </div>
    <button type="button" data-desk-sheet-close on:click={() => dispatch('close')}>Close</button>
  </header>

  <button class="overview-card" data-desk-overview type="button" on:click={() => dispatch('showoverview')}>
    <span>▦</span>
    <strong>Desktop Overview</strong>
    <small>Switch, suspend, focus, and clean up windows</small>
  </button>

  <section class="desk-section">
    <h3>Apps</h3>
    <div class="app-grid" data-desk-sheet-apps>
      {#each deskApps as app}
        <button data-desk-sheet-app data-desk-app-id={app.id} type="button" on:click={() => dispatch('launchapp', { app })}>
          <span>{app.icon}</span>
          <strong>{app.name}</strong>
          <small>{app.description}</small>
        </button>
      {/each}
    </div>
  </section>

  <button class="plain-row" data-desk-show-desktop type="button" on:click={() => dispatch('showdesktop')}>Show desktop</button>

  <footer>
    {#if authenticated}
      <span data-prompt-surface-user>{currentUser?.email || 'signed in'}</span>
      <button type="button" data-prompt-surface-logout on:click={() => dispatch('logout')}>Sign out</button>
    {:else}
      <span data-prompt-surface-user>Public preview</span>
      <button type="button" data-prompt-surface-login on:click={() => dispatch('authrequest')}>Sign in</button>
    {/if}
  </footer>
</section>

<style>
  .desk-sheet-backdrop {
    position: fixed;
    inset: 0;
    z-index: 9998;
    border: 0;
    background: rgba(0, 0, 0, 0.18);
    cursor: default;
  }

  .desk-sheet {
    position: fixed;
    left: max(12px, env(safe-area-inset-left));
    right: max(12px, env(safe-area-inset-right));
    z-index: 9999;
    display: grid;
    grid-template-rows: auto auto minmax(0, 1fr) auto auto;
    gap: 0.85rem;
    height: min(var(--choir-desk-sheet-height, 56dvh), calc(100dvh - var(--choir-prompt-surface-size, 64px) - 28px));
    overflow: auto;
    padding: 1rem;
    background: var(--choir-sheet-bg);
    color: var(--choir-fg);
    box-shadow: var(--choir-shadow-floating), 0 -18px 70px color-mix(in srgb, var(--choir-accent) 12%, transparent);
    backdrop-filter: blur(var(--choir-blur));
  }

  .desk-sheet.placement-bottom {
    bottom: calc(var(--choir-prompt-surface-size, 64px) + max(18px, env(safe-area-inset-bottom)));
    border-radius: var(--choir-radius-sheet) var(--choir-radius-sheet) var(--choir-radius-control) var(--choir-radius-control);
  }

  .desk-sheet.placement-top {
    top: calc(var(--choir-prompt-surface-size, 64px) + max(18px, env(safe-area-inset-top)));
    border-radius: var(--choir-radius-control) var(--choir-radius-control) var(--choir-radius-sheet) var(--choir-radius-sheet);
  }

  .sheet-handle {
    justify-self: center;
    width: 3rem;
    height: 0.25rem;
    border-radius: 999px;
    background: color-mix(in srgb, var(--choir-accent) 42%, transparent);
    filter: blur(0.1px);
  }

  header,
  footer {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 1rem;
  }

  p,
  h2,
  h3 {
    margin: 0;
  }

  header p,
  h3,
  footer span {
    color: var(--choir-muted);
    font-size: 0.78rem;
    font-weight: 800;
    text-transform: uppercase;
  }

  header h2 {
    font-family: var(--choir-font-display);
    font-size: 1.8rem;
    letter-spacing: 0;
  }

  button {
    border: 0;
    border-radius: var(--choir-radius-control-sm);
    background: var(--choir-control-bg);
    color: var(--choir-fg);
    cursor: pointer;
    box-shadow: var(--choir-control-shadow);
  }

  header button,
  footer button,
  .plain-row {
    padding: 0.65rem 0.85rem;
  }

  .overview-card {
    display: grid;
    grid-template-columns: auto minmax(0, 1fr);
    gap: 0.5rem 0.8rem;
    align-self: start;
    align-items: center;
    min-height: 4.5rem;
    height: auto;
    padding: 0.9rem;
    text-align: left;
    border-radius: var(--choir-radius-control);
  }

  .overview-card span {
    grid-row: span 2;
    font-size: 1.4rem;
  }

  .overview-card small,
  .app-grid small {
    color: var(--choir-muted);
  }

  .app-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(12rem, 1fr));
    gap: 0.65rem;
  }

  .app-grid button {
    display: grid;
    grid-template-columns: auto minmax(0, 1fr);
    gap: 0.25rem 0.65rem;
    padding: 0.75rem;
    text-align: left;
  }

  .app-grid button span {
    grid-row: span 2;
    font-size: 1.35rem;
  }

  .app-grid strong,
  .app-grid small {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .plain-row {
    justify-self: start;
  }

  @media (max-width: 768px) {
    .desk-sheet {
      left: 8px;
      right: 8px;
      height: min(62dvh, calc(100dvh - var(--choir-prompt-surface-size, 64px) - 20px));
    }

    .app-grid {
      grid-template-columns: 1fr;
    }
  }
</style>
