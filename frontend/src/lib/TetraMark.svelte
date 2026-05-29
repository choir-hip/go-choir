<script lang="ts">
  import { TETRA_MARK_PATHS, TETRA_MARK_VIEWBOX } from './tetramark';

  export let label = 'Desk';
</script>

<svg class="tetra-mark" viewBox={TETRA_MARK_VIEWBOX} role="img" aria-label={label} focusable="false">
  {#each TETRA_MARK_PATHS as path}
    <path d={path} />
  {/each}
</svg>

<style>
  .tetra-mark {
    display: block;
    width: 1.35rem;
    height: 1.35rem;
    color: currentColor;
    overflow: visible;
  }

  .tetra-mark path {
    fill: currentColor;
  }
</style>
