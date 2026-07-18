<script lang="ts">
  export let title = '';
</script>

<section class="app-empty-state" data-app-empty-state>
  {#if title}
    <h3>{title}</h3>
  {/if}
  <div class="empty-body">
    <slot />
  </div>
</section>

<style>
  .app-empty-state {
    display: grid;
    gap: 0.55rem;
    place-content: center;
    justify-items: start;
    min-height: 10rem;
    padding: 1.25rem 1.15rem;
    border-radius: var(--choir-radius-panel, 22px);
    background:
      linear-gradient(160deg, color-mix(in srgb, var(--choir-accent) 8%, transparent), transparent 42%),
      var(--choir-surface-card);
    box-shadow:
      inset 0 1px 0 color-mix(in srgb, var(--choir-text-primary) 6%, transparent),
      0 14px 36px color-mix(in srgb, var(--choir-shadow-color) 16%, transparent);
    color: var(--choir-text-primary);
  }

  h3 {
    margin: 0;
    font-family: var(--choir-font-display, inherit);
    font-size: 1.05rem;
    font-weight: 720;
    letter-spacing: -0.01em;
    color: var(--choir-text-primary);
  }

  .empty-body {
    color: var(--choir-text-muted);
    font-size: 0.92rem;
    line-height: 1.45;
    max-width: 28rem;
  }
</style>
