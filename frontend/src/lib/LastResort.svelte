<!--
  LastResort: what the host shows in a shell slot that has no healthy provider
  (docs/design/modular-architecture-2026-11-04.md section 1, item 7). Plain,
  unthemed beyond the tokens, and small: it exists so a broken or absent
  surface package never locks the owner out. Each `kind` is one slot's
  fallback; `apps`, `windows` and the handlers are what that slot needs.
-->
<script lang="ts">
  import { createEventDispatcher } from 'svelte';

  export let kind: 'launcher' | 'overview';
  export let apps: Array<{ id: string; name: string; icon: string }> = [];
  export let windows: Array<{ windowId: string; title: string; mode: string }> = [];

  const dispatch = createEventDispatcher();
</script>

<section class="last-resort" data-last-resort={kind} role="dialog" aria-label={kind === 'launcher' ? 'Apps' : 'Open windows'}>
  <header>
    <strong>{kind === 'launcher' ? 'Apps' : 'Open windows'}</strong>
    <button type="button" data-last-resort-close on:click={() => dispatch('close')}>Close</button>
  </header>
  <ul>
    {#if kind === 'launcher'}
      {#each apps as app (app.id)}
        <li><button type="button" data-last-resort-app={app.id} on:click={() => dispatch('launch', { app })}>{app.icon} {app.name}</button></li>
      {/each}
    {:else}
      {#each windows.filter((w) => w.mode !== 'closed' && w.mode !== 'hidden') as win (win.windowId)}
        <li><button type="button" data-last-resort-window={win.windowId} on:click={() => dispatch('focus', { windowId: win.windowId })}>{win.title}</button></li>
      {/each}
    {/if}
  </ul>
</section>

<style>
  .last-resort {
    position: fixed;
    left: 50%;
    top: 50%;
    z-index: 9999;
    width: min(360px, calc(100vw - 32px));
    max-height: 70vh;
    overflow: auto;
    transform: translate(-50%, -50%);
    padding: 12px;
    border: 1px solid var(--choir-border, currentColor);
    border-radius: 8px;
    color: var(--choir-fg, #111);
    background: var(--choir-bg, #fff);
  }

  header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 8px;
  }

  ul {
    margin: 0;
    padding: 0;
    list-style: none;
  }

  li button {
    width: 100%;
    min-height: 40px;
    text-align: left;
  }
</style>
