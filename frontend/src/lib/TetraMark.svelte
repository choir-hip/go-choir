<!--
  TetraMark — the Choir mark.

  `size` is a prop rather than something a caller reaches in and overrides
  with CSS. Svelte compiles a component's own class selector with the
  scoping class attached twice — `.tetra-mark.svelte-x.svelte-x` — so it
  outranks any `.wrapper :global(svg)` a parent can write. A caller trying
  to size this from outside therefore loses the specificity fight silently:
  the rule lands, the mark stays 1.35rem, and the failure looks like a
  layout bug rather than a cascade bug. (It did. The landing hero mark was
  rendering at 22px, left-aligned inside a 116px box.)
-->
<script lang="ts">
  import { TETRA_MARK_PATHS, TETRA_MARK_VIEWBOX } from './tetramark';

  export let label = 'Desk';
  /** Any CSS length. Inline so it always wins, which is the point. */
  export let size = '1.35rem';
</script>

<svg
  class="tetra-mark"
  viewBox={TETRA_MARK_VIEWBOX}
  role="img"
  aria-label={label}
  aria-hidden={label ? undefined : 'true'}
  focusable="false"
  style:width={size}
  style:height={size}
>
  {#each TETRA_MARK_PATHS as path}
    <path d={path} />
  {/each}
</svg>

<style>
  .tetra-mark {
    display: block;
    flex-shrink: 0;
    color: currentColor;
    overflow: visible;
  }
  .tetra-mark path {
    fill: currentColor;
  }
</style>
