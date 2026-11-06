<!--
  LastResort: what the host shows in a shell slot that has no healthy provider
  (docs/design/modular-architecture-2026-11-04.md section 1, item 7). Plain,
  unthemed beyond the tokens, and small: it exists so a broken or absent
  surface package never locks the owner out. Each `kind` is one slot's
  fallback; `apps`, `windows` and the handlers are what that slot needs. The
  prompt's fallback is a plain field at the bottom: Enter sends the text to the
  conductor (no focus), and Apps opens the launcher. The notifications'
  fallback is a plain stack. `recovery` is the recovery surface (H4): every
  slot, who draws it, the providers set aside, and "Use default" to register
  the slot's in-tree package again.
-->
<script lang="ts">
  import { createEventDispatcher } from 'svelte';

  export let kind: 'launcher' | 'overview' | 'prompt' | 'notifications' | 'recovery';
  export let apps: Array<{ id: string; name: string; icon: string }> = [];
  export let windows: Array<{ windowId: string; title: string; mode: string }> = [];
  export let disabled = false;
  export let notifications: Array<{ id: number; title: string; body: string; kind: string }> = [];
  export let rows: Array<{ slot: string; active: string | null; lastResort: boolean; defaultPackage: string; failed: string[] }> = [];

  const dispatch = createEventDispatcher();
  let text = '';

  function onPromptKey(event: KeyboardEvent) {
    if (event.key !== 'Enter' || event.shiftKey || disabled || !text.trim()) return;
    event.preventDefault();
    dispatch('submit', { text: text.trim() });
    text = '';
  }
</script>

{#if kind === 'notifications'}
<ul class="last-resort-notes" data-last-resort="notifications">
  {#each notifications as note (note.id)}
    <li role={note.kind === 'error' ? 'alert' : 'status'} aria-atomic="true">
      <span>{#if note.title}{note.title}: {/if}{note.body}</span>
      <button type="button" aria-label="Dismiss notification" on:click={() => dispatch('dismiss', { id: note.id })}>×</button>
    </li>
  {/each}
</ul>
{:else if kind === 'recovery'}
<section class="last-resort" data-last-resort="recovery" role="dialog" aria-label="Shell recovery">
  <header>
    <strong>Shell recovery</strong>
    <button type="button" data-last-resort-close on:click={() => dispatch('close')}>Close</button>
  </header>
  <table>
    <thead><tr><th>Slot</th><th>Drawn by</th><th></th></tr></thead>
    <tbody>
      {#each rows as row (row.slot)}
        <tr data-recovery-slot={row.slot} data-recovery-last-resort={row.lastResort ? 'true' : 'false'}>
          <td>{row.slot}</td>
          <td>{row.lastResort ? 'last resort' : row.active}{#if row.failed.length} (set aside: {row.failed.join(', ')}){/if}</td>
          <td>
            {#if row.defaultPackage && row.active !== row.defaultPackage}
              <button type="button" data-recovery-restore={row.defaultPackage} on:click={() => dispatch('restore', { name: row.defaultPackage })}>Use default ({row.defaultPackage})</button>
            {/if}
          </td>
        </tr>
      {/each}
    </tbody>
  </table>
</section>
{:else if kind === 'prompt'}
<form class="last-resort-prompt" data-last-resort="prompt" on:submit|preventDefault>
  <button type="button" data-last-resort-apps on:click={() => dispatch('apps')}>Apps</button>
  <textarea rows="1" bind:value={text} {disabled} aria-label="Command prompt" data-last-resort-prompt-input on:keydown={onPromptKey} />
</form>
{:else}
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
      <li><button type="button" data-last-resort-recovery on:click={() => dispatch('recovery')}>Shell recovery</button></li>
    {:else}
      {#each windows.filter((w) => w.mode !== 'closed' && w.mode !== 'hidden') as win (win.windowId)}
        <li><button type="button" data-last-resort-window={win.windowId} on:click={() => dispatch('focus', { windowId: win.windowId })}>{win.title}</button></li>
      {/each}
    {/if}
  </ul>
</section>
{/if}

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

  .last-resort-notes {
    position: fixed;
    left: 12px;
    right: 12px;
    bottom: 80px;
    z-index: 1200;
    margin: 0;
    padding: 0;
    list-style: none;
  }

  .last-resort-notes li {
    display: flex;
    justify-content: space-between;
    gap: 8px;
    margin-top: 4px;
    padding: 8px;
    border: 1px solid var(--choir-border, currentColor);
    border-radius: 6px;
    color: var(--choir-fg, #111);
    background: var(--choir-bg, #fff);
  }

  table {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.85rem;
  }

  td, th {
    padding: 4px;
    text-align: left;
  }

  .last-resort-prompt {
    position: fixed;
    left: 12px;
    right: 12px;
    bottom: 12px;
    z-index: 10000;
    display: flex;
    gap: 8px;
    padding: 8px;
    border: 1px solid var(--choir-border, currentColor);
    border-radius: 8px;
    color: var(--choir-fg, #111);
    background: var(--choir-bg, #fff);
  }

  .last-resort-prompt textarea {
    flex: 1;
    min-height: 40px;
    font: inherit;
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
