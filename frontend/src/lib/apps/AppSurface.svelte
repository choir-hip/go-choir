<script lang="ts">
  import type { ChoirAppDefinition } from './registry';

  export let app: ChoirAppDefinition | null = null;
  export let win: any = null;
  export let attrs: Record<string, string> = {};

  $: className = `app-content ${app?.theme.contentClass || 'standard-content'}`;
</script>

<div
  class={className}
  data-app-host
  data-app-id={app?.id || win?.appId || ''}
  data-app-surface={app?.theme.surface || 'standard'}
  data-app-preview-policy={app?.auth.preview || 'private'}
  data-app-window-id={win?.windowId || ''}
  {...attrs}
>
  <slot />
</div>

<style>
  .app-content {
    display: flex;
    flex-direction: column;
    height: 100%;
    padding: 1rem;
    background:
      linear-gradient(var(--choir-panel, #0d1628), var(--choir-panel, #0d1628)),
      #0d1628;
    background-color: #0d1628 !important;
    background-clip: padding-box;
    isolation: isolate;
    color: var(--choir-fg, #f7faff);
  }

  .app-content[data-app-surface='document'],
  .app-content[data-app-surface='media'],
  .app-content[data-app-surface='terminal'],
  .app-content[data-app-id='trace'],
  .app-content[data-app-id='settings'],
  .app-content[data-app-id='compute-monitor'],
  .app-content[data-app-id='features'] {
    padding: 0;
  }
</style>
