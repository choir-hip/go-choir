<script>
  // Icons as inline SVG: no symbol or emoji glyphs in buttons (they render as
  // boxes in the capsule and on some devices). The manifest's display.icon is
  // the one exception.
  export let name = 'search';
  export let size = 16;
</script>

<svg
  viewBox="0 0 24 24"
  width={size}
  height={size}
  fill="none"
  stroke="currentColor"
  stroke-width="2"
  stroke-linecap="round"
  stroke-linejoin="round"
  aria-hidden="true"
  focusable="false"
>
  {#if name === 'search'}
    <circle cx="11" cy="11" r="7" />
    <path d="M20 20l-4.2-4.2" />
  {:else if name === 'close'}
    <path d="M6 6l12 12M18 6L6 18" />
  {:else if name === 'left'}
    <path d="M14 6l-6 6 6 6" />
  {:else if name === 'right'}
    <path d="M10 6l6 6-6 6" />
  {:else if name === 'up'}
    <path d="M12 19V5M6 11l6-6 6 6" />
  {:else if name === 'download'}
    <path d="M12 4v11M7 10l5 5 5-5M5 20h14" />
  {:else if name === 'play'}
    <path d="M8 5l11 7-11 7z" />
  {:else if name === 'image'}
    <rect x="3.5" y="5.5" width="17" height="13" rx="2" />
    <circle cx="9" cy="10" r="1.4" />
    <path d="M4.5 17l5-5 3.5 3.5 3-3 3.5 3.5" />
  {:else if name === 'package'}
    <path d="M12 3l8 4.5v9L12 21l-8-4.5v-9z" />
    <path d="M4 7.5l8 4.5 8-4.5M12 12v9" />
  {:else if name === 'check'}
    <path d="M5 13l4.5 4.5L19 7" />
  {:else if name === 'warning'}
    <path d="M12 4l9 16H3z" />
    <path d="M12 10v4M12 17.2v.1" />
  {:else if name === 'refresh'}
    <path d="M20 12a8 8 0 1 1-2.6-5.9" />
    <path d="M20 4v4h-4" />
  {:else}
    <circle cx="12" cy="12" r="8" />
  {/if}
</svg>
