<script lang="ts">
  import {
    materializeRichInlineLineRange,
    prepareRichInline,
    walkRichInlineLineRanges,
    type PreparedRichInline,
    type RichInlineItem,
  } from '@chenglou/pretext/rich-inline';
  import { onMount } from 'svelte';

  export let prefix = '';
  export let subject = '';
  export let collapsedDetail = '';
  export let disclosure = '';
  export let ariaLabel = 'More information';

  const LINE_HEIGHT = 26;
  const INFO_CHIP_EXTRA_WIDTH = 9;
  const MIN_LAYOUT_WIDTH = 160;

  type FragmentKind = 'text' | 'subject';

  type DisclosureFragment = {
    className: string;
    gapBefore: number;
    kind: FragmentKind;
    text: string;
  };

  type DisclosureLine = {
    fragments: DisclosureFragment[];
    width: number;
  };

  type DisclosureLayout = {
    lines: DisclosureLine[];
  };

  let rootNode: HTMLDivElement;
  let layoutWidth = 320;
  let preparedCache = new Map<string, PreparedRichInline>();
  let headingFont = '760 20px Inter, sans-serif';
  let bodyFont = '520 14px Inter, sans-serif';
  let pinned = false;
  let hovered = false;
  let focused = false;

  $: active = pinned || hovered || focused;
  $: collapsedLayout = layoutDisclosure(false);
  $: expandedLayout = layoutDisclosure(true);
  $: visibleLayout = active ? expandedLayout : collapsedLayout;
  $: reservedLineCount = Math.max(collapsedLayout.lines.length, expandedLayout.lines.length, 1);
  $: reservedHeight = reservedLineCount * LINE_HEIGHT;
  $: headingLabel = `${prefix}${subject}`.replace(/\s+/g, ' ').trim();

  function disclosureItems(expanded: boolean): RichInlineItem[] {
    const detail = expanded ? disclosure : collapsedDetail;
    const items: RichInlineItem[] = [
      { text: prefix, font: headingFont },
      {
        text: `${subject} ⓘ`,
        font: headingFont,
        break: 'never',
        extraWidth: INFO_CHIP_EXTRA_WIDTH,
      },
    ];

    if (detail.trim().length > 0) {
      items.push({ text: ` ${detail}`, font: bodyFont });
    }

    return items;
  }

  function preparedDisclosure(expanded: boolean): PreparedRichInline {
    const items = disclosureItems(expanded);
    const key = items.map(item => [
      item.text,
      item.font,
      item.break ?? 'normal',
      item.extraWidth ?? 0,
      item.letterSpacing ?? 0,
    ].join('\u0001')).join('\u0002');
    const cached = preparedCache.get(key);
    if (cached) return cached;
    const prepared = prepareRichInline(items);
    preparedCache.set(key, prepared);
    return prepared;
  }

  function layoutDisclosure(expanded: boolean): DisclosureLayout {
    const prepared = preparedDisclosure(expanded);
    const maxWidth = Math.max(MIN_LAYOUT_WIDTH, layoutWidth);
    const lines: DisclosureLine[] = [];

    walkRichInlineLineRanges(prepared, maxWidth, range => {
      const line = materializeRichInlineLineRange(prepared, range);
      lines.push({
        width: line.width,
        fragments: line.fragments.map(fragment => ({
          className: fragment.itemIndex === 2
            ? 'pretext-disclosure-fragment pretext-disclosure-fragment--body'
            : 'pretext-disclosure-fragment pretext-disclosure-fragment--heading',
          gapBefore: fragment.gapBefore,
          kind: fragment.itemIndex === 1 ? 'subject' : 'text',
          text: fragment.text,
        })),
      });
    });

    return { lines };
  }

  function fontFamilyFromVar(name: string, fallback: string): string {
    if (typeof document === 'undefined') return fallback;
    const value = getComputedStyle(document.documentElement).getPropertyValue(name).trim();
    return value || fallback;
  }

  function updateFonts() {
    const display = fontFamilyFromVar('--choir-font-display', 'Inter, sans-serif');
    const ui = fontFamilyFromVar('--choir-font-ui', 'Inter, sans-serif');
    headingFont = `760 20px ${display}`;
    bodyFont = `520 14px ${ui}`;
    preparedCache = new Map();
  }

  function updateLayoutWidth() {
    if (!rootNode) return;
    layoutWidth = Math.max(MIN_LAYOUT_WIDTH, Math.floor(rootNode.clientWidth));
  }

  function toggleDisclosure() {
    pinned = !pinned;
  }

  onMount(() => {
    updateFonts();
    updateLayoutWidth();

    const resizeObserver = new ResizeObserver(updateLayoutWidth);
    resizeObserver.observe(rootNode);

    const handleThemeChange = () => {
      updateFonts();
      requestAnimationFrame(updateLayoutWidth);
    };

    const fonts = (document as Document & { fonts?: { ready: Promise<unknown> } }).fonts;
    fonts?.ready.then(() => {
      updateFonts();
      updateLayoutWidth();
    });

    window.addEventListener('choir-theme-change', handleThemeChange);
    return () => {
      resizeObserver.disconnect();
      window.removeEventListener('choir-theme-change', handleThemeChange);
    };
  });
</script>

<div
  bind:this={rootNode}
  class="pretext-disclosure"
  data-pretext-disclosure
  data-state={active ? 'expanded' : 'collapsed'}
  role="heading"
  aria-level="2"
  aria-label={headingLabel}
  style={`--pretext-disclosure-height: ${reservedHeight}px; --pretext-disclosure-line-height: ${LINE_HEIGHT}px;`}
  on:pointerenter={() => hovered = true}
  on:pointerleave={() => hovered = false}
>
  <div class="pretext-disclosure-stage" aria-live="polite">
    {#each visibleLayout.lines as line, lineIndex}
      <div
        class="pretext-disclosure-line"
        data-pretext-line
        data-line-index={lineIndex}
        style={`top: ${lineIndex * LINE_HEIGHT}px;`}
      >
        {#each line.fragments as fragment, fragmentIndex}
          {#if fragment.kind === 'subject'}
            <span
              class="pretext-disclosure-subject"
              data-pretext-fragment
              data-fragment-kind="subject"
              style={fragment.gapBefore > 0 ? `margin-left: ${fragment.gapBefore}px;` : ''}
            >
              <span>{subject}</span>
              <button
                type="button"
                class="pretext-disclosure-info"
                data-passkey-info-button
                data-pretext-info-button
                aria-label={ariaLabel}
                aria-expanded={active}
                on:click={toggleDisclosure}
                on:focus={() => focused = true}
                on:blur={() => focused = false}
              >ⓘ</button>
            </span>
          {:else}
            <span
              class={fragment.className}
              data-pretext-fragment
              data-fragment-kind={fragmentIndex === 0 && lineIndex === 0 ? 'prefix' : 'body'}
              style={fragment.gapBefore > 0 ? `margin-left: ${fragment.gapBefore}px;` : ''}
            >{fragment.text}</span>
          {/if}
        {/each}
      </div>
    {/each}
  </div>
</div>

<style>
  .pretext-disclosure {
    position: relative;
    width: 100%;
    height: var(--pretext-disclosure-height);
    color: var(--choir-text-primary);
  }

  .pretext-disclosure-stage {
    position: relative;
    height: 100%;
  }

  .pretext-disclosure-line {
    position: absolute;
    left: 0;
    display: flex;
    align-items: center;
    min-height: var(--pretext-disclosure-line-height);
    line-height: var(--pretext-disclosure-line-height);
    white-space: pre;
  }

  .pretext-disclosure-fragment {
    display: inline-block;
    white-space: pre;
  }

  .pretext-disclosure-fragment--heading,
  .pretext-disclosure-subject {
    font-family: var(--choir-font-display, Inter, sans-serif);
    font-size: 1.28rem;
    font-weight: 760;
    line-height: var(--pretext-disclosure-line-height);
  }

  .pretext-disclosure-fragment--body {
    color: var(--choir-text-muted);
    font-family: var(--choir-font-ui, Inter, sans-serif);
    font-size: 0.88rem;
    font-weight: 520;
    line-height: var(--pretext-disclosure-line-height);
  }

  .pretext-disclosure-subject {
    display: inline-flex;
    align-items: center;
    gap: 0.25rem;
    white-space: nowrap;
  }

  .pretext-disclosure-info {
    display: inline-grid;
    place-items: center;
    width: 1.35rem;
    height: 1.35rem;
    border: 0;
    border-radius: 999px;
    background: var(--choir-surface-control);
    color: var(--choir-accent);
    cursor: pointer;
    font: inherit;
    font-size: 0.86rem;
    line-height: 1;
    box-shadow: var(--choir-control-shadow);
  }

  .pretext-disclosure-info:focus-visible {
    outline: 3px solid color-mix(in srgb, var(--choir-accent) 32%, transparent);
    outline-offset: 2px;
  }
</style>
