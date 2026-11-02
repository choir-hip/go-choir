<script>
  export let name = '';
</script>

<svg
  class="glyph"
  viewBox="0 0 12 12"
  fill="none"
  stroke="currentColor"
  stroke-width="1.5"
  stroke-linecap="round"
  stroke-linejoin="round"
  aria-hidden="true"
  focusable="false"
>
  {#if name === 'chevron-left'}
    <path d="M7.5 2.5 4 6l3.5 3.5" />
  {:else if name === 'chevron-right'}
    <path d="M4.5 2.5 8 6 4.5 9.5" />
  {:else if name === 'expand'}
    <path d="M1.5 4.5v-3h3" />
    <path d="M7.5 1.5h3v3" />
    <path d="M10.5 7.5v3h-3" />
    <path d="M4.5 10.5h-3v-3" />
  {:else if name === 'shrink'}
    <path d="M4.5 1.5v3h-3" />
    <path d="M10.5 4.5h-3v-3" />
    <path d="M7.5 10.5v-3h3" />
    <path d="M1.5 7.5h3v3" />
  {:else if name === 'file'}
    <path d="M2.5 1.5H7l2.5 2.5v6.5h-7z" />
    <path d="M7 1.5V4h2.5" />
  {/if}
</svg>
