<script lang="ts">
  export let type: 'button' | 'submit' | 'reset' = 'button';
  export let disabled = false;
</script>

<button class="app-button" {type} {disabled} on:click>
  <slot />
</button>

<style>
  .app-button {
    min-height: 2.35rem;
    padding: 0.55rem 0.9rem;
    border: 0;
    border-radius: var(--choir-radius-control-sm, 14px);
    background: var(--choir-surface-control);
    color: var(--choir-text-primary);
    font: inherit;
    font-weight: 700;
    cursor: pointer;
    box-shadow: var(--choir-control-shadow);
    transition:
      transform var(--choir-motion-fast, 120ms ease),
      filter var(--choir-motion-fast, 120ms ease),
      box-shadow var(--choir-motion-fast, 120ms ease);
  }

  .app-button:hover:enabled {
    filter: brightness(1.06);
    transform: translateY(-1px);
    box-shadow: var(--choir-control-shadow), 0 0 22px color-mix(in srgb, var(--choir-accent) 16%, transparent);
  }

  .app-button:focus-visible {
    outline: 2px solid var(--choir-accent);
    outline-offset: 2px;
  }

  .app-button:disabled {
    opacity: 0.55;
    cursor: not-allowed;
    transform: none;
  }

  @media (prefers-reduced-motion: reduce) {
    .app-button {
      transition: none;
    }
  }
</style>
