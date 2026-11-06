<!--
  The notification stack: a shell surface of the notifications.host slot
  (lib/shell-slots.ts; slice H4). The host posts notifications through the
  notification.post pipeline (lib/services/notifications.ts) and hands this
  surface what came out.

  Slot ABI, props: notifications ({ id, title, body, kind: 'info' | 'error' }[],
  oldest first). Event: dismiss ({ id }).
-->
<script lang="ts">
  import { createEventDispatcher } from 'svelte';

  export let notifications: Array<{ id: number; title: string; body: string; kind: string }> = [];

  const dispatch = createEventDispatcher();
</script>

{#if notifications.length > 0}
  <div class="toast-stack" data-notifications>
    {#each notifications as note (note.id)}
      <div class="toast" class:error={note.kind === 'error'} role={note.kind === 'error' ? 'alert' : 'status'} aria-atomic="true" data-notification={note.kind}>
        <span>{#if note.title}<strong>{note.title}</strong> {/if}{note.body}</span>
        <button class="toast-close" type="button" aria-label="Dismiss notification" on:click={() => dispatch('dismiss', { id: note.id })}>×</button>
      </div>
    {/each}
  </div>
{/if}

<style>
  .toast-stack {
    position: fixed;
    left: 50%;
    bottom: 72px;
    transform: translateX(-50%);
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
    z-index: 1200;
    pointer-events: none;
  }

  .toast {
    pointer-events: auto;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.65rem;
    width: max-content;
    max-width: min(32rem, calc(100vw - 2rem));
    background: var(--choir-state-selected);
    color: var(--choir-text-accent);
    border: 0;
    border-radius: var(--choir-radius-pill, 30px);
    padding: 0.6rem 0.95rem;
    font-size: 0.82rem;
    box-shadow: 0 12px 32px color-mix(in srgb, var(--choir-shadow-color) 25%, transparent);
    animation: toast-enter var(--choir-motion-fast);
  }

  .toast.error {
    background: var(--choir-status-danger);
    box-shadow: 0 12px 32px color-mix(in srgb, var(--choir-status-danger) 18%, transparent);
    color: var(--choir-text-on-danger);
  }

  .toast-close {
    display: inline-grid;
    flex: 0 0 auto;
    place-items: center;
    width: 2.25rem;
    height: 2.25rem;
    margin: -0.35rem -0.65rem -0.35rem 0;
    border: 0;
    border-radius: 50%;
    background: color-mix(in srgb, currentColor 14%, transparent);
    color: inherit;
    font: inherit;
    font-size: 1rem;
    cursor: pointer;
  }

  .toast-close:hover {
    background: color-mix(in srgb, currentColor 24%, transparent);
  }

  .toast-close:focus-visible {
    outline: 2px solid currentColor;
    outline-offset: 2px;
  }

  @keyframes toast-enter {
    from { opacity: 0; transform: translateY(0.4rem) scale(0.98); }
    to { opacity: 1; transform: translateY(0) scale(1); }
  }

  @media (prefers-reduced-motion: reduce) {
    .toast { animation: none; }
  }
</style>
