<script lang="ts">
  export let label = '';
</script>

<div class="toolbar app-toolbar" data-app-toolbar aria-label={label || undefined}>
  <slot />
</div>
