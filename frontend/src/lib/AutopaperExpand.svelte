<!--
  AutopaperExpand — the "More research" control in a story's header.

  It shows nothing unless the proxy says this reader may ask
  (GET /api/platform/publications/expansion allowed:true). With the public
  door on that includes readers who are not signed in; with it off only the
  allowlisted owner sees it. A click asks the paper to expand the
  story; the line under it follows the request while it is open. The server's
  words are never rendered as HTML, and every failure leaves the story alone.
-->
<script lang="ts">
  import { onDestroy } from 'svelte';
  import {
    EXPAND_POLL_MS,
    askExpansion,
    expansionIsOpen,
    fateLine,
    fetchExpansion,
    latestRequest,
    type ExpansionRead,
  } from './autopaper-expand';

  /** The story's publication route (bare or /v<n>). */
  export let route = '';

  let read: ExpansionRead | null = null;
  let asking = false;
  let notice = '';
  let timer: ReturnType<typeof setTimeout> | null = null;
  // A late answer for a story the reader left, or for an older ask, is dropped.
  let token = 0;

  $: void load(route);
  $: request = latestRequest(read);
  $: line = notice || fateLine(request);
  $: disabled = asking || read?.asked === true || expansionIsOpen(read);
  $: label = asking ? 'Asking…' : (read?.asked ? 'Asked for more' : 'More research');

  function stop() {
    if (timer) clearTimeout(timer);
    timer = null;
  }

  function schedule() {
    stop();
    if (read?.allowed && expansionIsOpen(read)) timer = setTimeout(poll, EXPAND_POLL_MS);
  }

  async function load(next: string) {
    token += 1;
    const mine = token;
    stop();
    read = null;
    notice = '';
    asking = false;
    if (!next) return;
    const got = await fetchExpansion(next);
    if (mine !== token) return;
    read = got?.allowed === true ? got : null;
    schedule();
  }

  async function poll() {
    const mine = token;
    const got = await fetchExpansion(route);
    if (mine !== token) return;
    if (got?.allowed === true) read = got;
    schedule();
  }

  async function ask() {
    if (disabled || !route) return;
    const mine = token;
    asking = true;
    notice = '';
    const result = await askExpansion(route);
    if (mine !== token) return;
    asking = false;
    if (!result.ok) notice = result.message;
    const got = await fetchExpansion(route);
    if (mine !== token) return;
    if (got?.allowed === true) read = got;
    schedule();
  }

  onDestroy(() => {
    token += 1;
    stop();
  });
</script>

{#if read?.allowed}
  <div class="ap-expand" data-autopaper-expand>
    <button
      type="button"
      class="ap-expand-button"
      data-autopaper-expand-button
      {disabled}
      on:click={ask}
    >{label}</button>
    {#if line}
      <span class="ap-expand-line" data-autopaper-expand-line role="status" aria-live="polite">{line}</span>
    {/if}
  </div>
{/if}

<style>
  /* The button and the line are the header's own flex items: the line wraps
     to a row of its own so a phone's header never scrolls sideways. */
  .ap-expand {
    display: contents;
  }

  .ap-expand-button {
    flex: 0 0 auto;
    min-height: 2.75rem;
    padding: 0.35rem 0.8rem;
    border: 1px solid var(--choir-border);
    border-radius: 8px;
    background: var(--choir-surface-control);
    color: var(--choir-text-primary);
    font: inherit;
    font-size: 0.78rem;
    font-weight: 700;
    cursor: pointer;
  }

  .ap-expand-button:hover:not(:disabled),
  .ap-expand-button:focus-visible {
    background: var(--choir-state-hover);
    outline: 2px solid var(--choir-accent);
    outline-offset: 2px;
  }

  .ap-expand-button:disabled {
    cursor: default;
    opacity: 0.7;
  }

  .ap-expand-line {
    flex: 1 0 100%;
    min-width: 0;
    color: var(--choir-text-muted);
    font-size: 0.8rem;
    overflow-wrap: anywhere;
  }
</style>
