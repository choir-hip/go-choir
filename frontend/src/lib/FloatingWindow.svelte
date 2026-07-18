<!--
  FloatingWindow — simplified desktop window with bottom-right resize only.

  Rewrites Window.svelte with:
    - Title bar drag (no drag on buttons)
    - Single resize handle at bottom-right corner (no 8-handle system)
    - Minimum dimensions: width >= 200px, height >= 120px
    - Maximized fills desktop area excluding the prompt surface
    - Maximize button icon changes to restore icon when maximized
    - Restore returns to pre-maximize geometry
    - Minimize hides window, shows indicator in the prompt surface tray
    - Restore from minimized returns to pre-minimize geometry
    - Clicking window brings it to front (z-index management)
    - Active window uses elevated shadow and accent glow
    - Cascade positioning: 30px offset per window, wraps after 8
    - Window close transfers focus to next highest z-index window

  Data attributes for test targeting:
    data-window          — root container
    data-window-id       — window identifier
    data-window-titlebar — title bar for drag and window controls
    data-window-close    — close button
    data-window-minimize — minimize button
    data-window-maximize — maximize/restore button
    data-window-content  — content area hosting the app
    data-resize-handle   — bottom-right resize handle (se only)
-->
<script>
  import { createEventDispatcher } from 'svelte';
  import { onMount, onDestroy } from 'svelte';

  export let windowId = '';
  export let appId = '';
  export let title = 'Window';
  export let x = 100;
  export let y = 50;
  export let width = 600;
  export let height = 400;
  export let mode = 'normal'; // 'normal' | 'minimized' | 'maximized'
  export let zIndex = 1;
  export let active = false;
  export let restoredGeometry = null;
  export let overviewOpen = false;
  export let overviewPreviewState = 'normal';
  export let overviewPreviewStyle = '';

  // Suppress unused-export warnings — props used by parent for persistence
  $: _appId = appId;
  $: _restoredGeo = restoredGeometry;

  const dispatch = createEventDispatcher();

  // ---- Constants ----
  const MIN_WIDTH = 200;
  const MIN_HEIGHT = 120;
  const DEFAULT_VIEWPORT_WIDTH = 1280;
  const DEFAULT_VIEWPORT_HEIGHT = 800;
  const DEFAULT_PROMPT_SURFACE_SIZE = 64;
  const MOBILE_BREAKPOINT = 768;
  const TABLET_BREAKPOINT = 1024;

  let viewportWidth = DEFAULT_VIEWPORT_WIDTH;
  let viewportHeight = DEFAULT_VIEWPORT_HEIGHT;
  let promptSurfaceTopOffset = 0;
  let promptSurfaceBottomOffset = DEFAULT_PROMPT_SURFACE_SIZE;
  let promptSurfaceObserver = null;

  // ---- Drag state ----
  let dragging = false;
  let dragOffsetX = 0;
  let dragOffsetY = 0;
  let dragPointerId = null;
  let dragPointerTarget = null;

  // ---- Resize state ----
  let resizing = false;
  let resizeStartX = 0;
  let resizeStartY = 0;
  let resizeStartWidth = 0;
  let resizeStartHeight = 0;
  let resizePointerId = null;
  let resizePointerTarget = null;

  function clamp(value, min, max) {
    return Math.min(Math.max(value, min), Math.max(min, max));
  }

  function parsePixelValue(value, fallback) {
    const parsed = Number.parseFloat(value);
    return Number.isFinite(parsed) ? parsed : fallback;
  }

  function readPromptSurfaceOffsets() {
    if (typeof document === 'undefined') {
      return { top: 0, bottom: DEFAULT_PROMPT_SURFACE_SIZE };
    }

    const root = window.getComputedStyle(document.documentElement);
    const themeTop = parsePixelValue(
      root.getPropertyValue('--choir-prompt-surface-top-offset'),
      0
    );
    const themeBottom = parsePixelValue(
      root.getPropertyValue('--choir-prompt-surface-bottom-offset'),
      DEFAULT_PROMPT_SURFACE_SIZE
    );

    const promptSurface = document.querySelector('[data-prompt-surface]');
    if (!promptSurface) return { top: themeTop, bottom: themeBottom };

    const placement = promptSurface.getAttribute('data-placement') || document.documentElement.dataset.promptSurfacePlacement;
    const rect = promptSurface.getBoundingClientRect();
    if (placement === 'top') {
      return { top: Math.max(themeTop, rect.bottom), bottom: 0 };
    }
    if (placement === 'bottom') {
      return { top: 0, bottom: Math.max(themeBottom, viewportHeight - rect.top) };
    }
    return { top: themeTop, bottom: themeBottom };
  }

  function refreshViewportBounds() {
    if (typeof window === 'undefined') return;

    viewportWidth = window.innerWidth || DEFAULT_VIEWPORT_WIDTH;
    viewportHeight = window.innerHeight || DEFAULT_VIEWPORT_HEIGHT;
    const promptOffsets = readPromptSurfaceOffsets();
    promptSurfaceTopOffset = promptOffsets.top;
    promptSurfaceBottomOffset = promptOffsets.bottom;
  }

  function refreshViewportBoundsAfterThemeChange() {
    window.requestAnimationFrame(refreshViewportBounds);
  }

  function trySetPointerCapture(target, pointerId) {
    if (!target?.setPointerCapture || pointerId == null) return;
    try {
      target.setPointerCapture(pointerId);
    } catch {
      // Some browsers reject capture for synthetic or already-lost pointers.
    }
  }

  function tryReleasePointerCapture(target, pointerId) {
    if (!target?.releasePointerCapture || pointerId == null) return;
    try {
      if (!target.hasPointerCapture || target.hasPointerCapture(pointerId)) {
        target.releasePointerCapture(pointerId);
      }
    } catch {
      // Ignore capture-release errors during teardown.
    }
  }

  // ---- Window control handlers ----

  function handleClose() {
    dispatch('close', { windowId });
  }

  function handleMinimize() {
    dispatch('minimize', { windowId });
  }

  function handleMaximizeRestore() {
    if (mode === 'maximized') {
      dispatch('restore', { windowId });
    } else {
      dispatch('maximize', { windowId });
    }
  }

  function handleTitlebarDoubleClick(event) {
    if (event.target.closest('button')) return;
    handleMaximizeRestore();
  }

  // ---- Focus handler ----

  function handleFocusWindow() {
    if (!active) {
      dispatch('focus', { windowId });
    }
  }

  // ---- Drag handlers (title bar only) ----

  function handleDragStart(event) {
    if (overviewOpen) return;
    if (event.pointerType === 'mouse' && event.button !== 0) return;
    if (event.target.closest('button')) return;
    if (mode === 'maximized') return;

    dragging = true;
    dragOffsetX = event.clientX - renderedX;
    dragOffsetY = event.clientY - renderedY;
    dragPointerId = event.pointerId;
    dragPointerTarget = event.currentTarget;
    trySetPointerCapture(dragPointerTarget, dragPointerId);

    handleFocusWindow();
    event.preventDefault();
  }

  function handleDragMove(event) {
    if (!dragging) return;
    if (dragPointerId != null && event.pointerId !== dragPointerId) return;
    const newX = event.clientX - dragOffsetX;
    const newY = event.clientY - dragOffsetY;
    dispatch('move', { windowId, x: newX, y: newY });
  }

  function handleDragEnd(event) {
    if (!dragging) return;
    if (dragPointerId != null && event?.pointerId != null && event.pointerId !== dragPointerId) return;
    tryReleasePointerCapture(dragPointerTarget, dragPointerId);
    dragging = false;
    dragPointerId = null;
    dragPointerTarget = null;
  }

  // ---- Resize handler (bottom-right handle only) ----

  function handleResizeStart(event) {
    if (overviewOpen) return;
    if (mode !== 'normal') return;
    if (event.pointerType === 'mouse' && event.button !== 0) return;

    resizing = true;
    resizeStartX = event.clientX;
    resizeStartY = event.clientY;
    resizeStartWidth = renderedWidth;
    resizeStartHeight = renderedHeight;
    resizePointerId = event.pointerId;
    resizePointerTarget = event.currentTarget;
    trySetPointerCapture(resizePointerTarget, resizePointerId);

    handleFocusWindow();
    event.preventDefault();
    event.stopPropagation();
  }

  function handleResizeMove(event) {
    if (!resizing) return;
    if (resizePointerId != null && event.pointerId !== resizePointerId) return;

    const dx = event.clientX - resizeStartX;
    const dy = event.clientY - resizeStartY;

    const newWidth = Math.max(MIN_WIDTH, resizeStartWidth + dx);
    const newHeight = Math.max(MIN_HEIGHT, resizeStartHeight + dy);

    dispatch('resize', { windowId, x: renderedX, y: renderedY, width: newWidth, height: newHeight });
  }

  function handleResizeEnd(event) {
    if (!resizing) return;
    if (resizePointerId != null && event?.pointerId != null && event.pointerId !== resizePointerId) return;
    tryReleasePointerCapture(resizePointerTarget, resizePointerId);
    resizing = false;
    resizePointerId = null;
    resizePointerTarget = null;
  }

  // ---- Global pointer event wiring ----

  onMount(() => {
    refreshViewportBounds();
    window.addEventListener('pointermove', handleDragMove);
    window.addEventListener('pointerup', handleDragEnd);
    window.addEventListener('pointermove', handleResizeMove);
    window.addEventListener('pointerup', handleResizeEnd);
    window.addEventListener('pointercancel', handleDragEnd);
    window.addEventListener('pointercancel', handleResizeEnd);
    window.addEventListener('resize', refreshViewportBounds);
    window.addEventListener('choir-theme-change', refreshViewportBoundsAfterThemeChange);

    const promptSurface = document.querySelector('[data-prompt-surface]');
    if (typeof ResizeObserver !== 'undefined' && promptSurface) {
      promptSurfaceObserver = new ResizeObserver(refreshViewportBounds);
      promptSurfaceObserver.observe(promptSurface);
    }
  });

  onDestroy(() => {
    window.removeEventListener('pointermove', handleDragMove);
    window.removeEventListener('pointerup', handleDragEnd);
    window.removeEventListener('pointermove', handleResizeMove);
    window.removeEventListener('pointerup', handleResizeEnd);
    window.removeEventListener('pointercancel', handleDragEnd);
    window.removeEventListener('pointercancel', handleResizeEnd);
    window.removeEventListener('resize', refreshViewportBounds);
    window.removeEventListener('choir-theme-change', refreshViewportBoundsAfterThemeChange);
    promptSurfaceObserver?.disconnect();
  });

  // ---- Computed styles ----

  $: viewportMargin = viewportWidth <= MOBILE_BREAKPOINT
    ? 8
    : viewportWidth <= TABLET_BREAKPOINT
    ? 16
    : 12;
  $: maxNormalWidth = Math.max(MIN_WIDTH, viewportWidth - viewportMargin * 2);
  $: maxNormalHeight = Math.max(
    MIN_HEIGHT,
    viewportHeight - promptSurfaceTopOffset - promptSurfaceBottomOffset - viewportMargin * 2
  );
  $: minRenderedY = viewportMargin + promptSurfaceTopOffset;
  $: renderedWidth = Math.min(Math.max(width, MIN_WIDTH), maxNormalWidth);
  $: renderedHeight = Math.min(Math.max(height, MIN_HEIGHT), maxNormalHeight);
  $: maxRenderedX = Math.max(viewportMargin, viewportWidth - renderedWidth - viewportMargin);
  $: maxRenderedY = Math.max(
    minRenderedY,
    viewportHeight - promptSurfaceBottomOffset - renderedHeight - viewportMargin
  );
  $: renderedX = clamp(x, viewportMargin, maxRenderedX);
  $: renderedY = clamp(y, minRenderedY, maxRenderedY);

  $: windowStyle = mode === 'maximized'
    ? `left:0; top:${promptSurfaceTopOffset}px; width:100%; height:calc(100dvh - ${promptSurfaceTopOffset + promptSurfaceBottomOffset}px);`
    : mode === 'minimized'
    ? 'display:none;'
    : `left:${renderedX}px; top:${renderedY}px; width:${renderedWidth}px; height:${renderedHeight}px;`;

  $: maxRestoreIcon = mode === 'maximized' ? '⤢' : '▢';
  $: maxRestoreTitle = mode === 'maximized' ? 'Restore' : 'Maximize';
  $: showResizeHandle = mode === 'normal' && !overviewOpen;
  $: overviewClass = overviewOpen ? `overview-preview overview-preview-${overviewPreviewState}` : '';
  $: effectiveZIndex = overviewOpen
    ? overviewPreviewState === 'live'
      ? 11000 + (zIndex || 0)
      : 9000 + (zIndex || 0)
    : zIndex;
  $: combinedStyle = `${windowStyle} z-index: ${effectiveZIndex}; ${overviewOpen ? overviewPreviewStyle : ''}`;
</script>

<!-- svelte-ignore a11y-click-events-have-key-events -->
<!-- svelte-ignore a11y-no-static-element-interactions -->
<div
  class="window {active ? 'window-active' : ''} {overviewClass}"
  style={combinedStyle}
  data-window
  data-window-id={windowId}
  data-window-app-id={appId}
  data-window-mode={mode}
  data-window-active={active ? 'true' : 'false'}
  data-overview-preview-state={overviewOpen ? overviewPreviewState : 'normal'}
  on:pointerdown={handleFocusWindow}
>
  <!-- Title bar -->
  <div
    class="titlebar"
    data-window-titlebar
    on:pointerdown={handleDragStart}
    on:dblclick={handleTitlebarDoubleClick}
  >
    <span class="titltexture">{title}</span>
    <div class="window-controls">
      <button
        class="ctrl-btn minimize-btn"
        data-window-minimize
        on:click|stopPropagation={handleMinimize}
        title="Minimize"
        aria-label="Minimize"
      >−</button>
      <button
        class="ctrl-btn maximize-btn"
        data-window-maximize
        on:click|stopPropagation={handleMaximizeRestore}
        title={maxRestoreTitle}
        aria-label={maxRestoreTitle}
      >{maxRestoreIcon}</button>
      <button
        class="ctrl-btn close-btn"
        data-window-close
        on:click|stopPropagation={handleClose}
        title="Close"
        aria-label="Close"
      >✕</button>
    </div>
  </div>

  <!-- Content area -->
  <div class="window-content" data-window-content>
    <slot />
  </div>

  <!-- Resize handle: bottom-right corner only (normal mode, not mobile) -->
  {#if showResizeHandle}
    <div
      class="resize-handle resize-se"
      data-resize-handle
      on:pointerdown|stopPropagation={handleResizeStart}
    ></div>
  {/if}
</div>

<style>
  .window {
    position: absolute;
    display: flex;
    flex-direction: column;
    background: var(--choir-surface-app);
    background-clip: padding-box;
    border: 0;
    border-radius: var(--choir-radius-panel, 26px);
    overflow: hidden;
    isolation: isolate;
    contain: paint;
    box-shadow:
      0 28px 80px color-mix(in srgb, var(--choir-shadow-color) 48%, transparent),
      0 10px 30px color-mix(in srgb, var(--choir-accent) 10%, transparent),
      inset 0 0 0 1px color-mix(in srgb, var(--choir-border-strong) 42%, transparent),
      inset 0 1px 0 color-mix(in srgb, var(--choir-text-primary) 7%, transparent);
    transform-origin: top left;
    transition: box-shadow 0.18s ease, filter 0.15s ease;
    user-select: none;
    max-width: calc(100vw - 24px);
    max-height: calc(100dvh - var(--choir-prompt-surface-top-offset, 0px) - var(--choir-prompt-surface-bottom-offset, 64px) - 16px);
  }

  .window.overview-preview {
    user-select: none;
    will-change: transform, opacity;
  }

  .window.overview-preview-live {
    cursor: pointer;
    transform:
      translate(
        var(--overview-translate-x, 0px),
        var(--overview-translate-y, 0px)
      )
      scale(var(--overview-scale, 1));
    transition:
      transform 0.36s cubic-bezier(0.2, 0.8, 0.2, 1),
      box-shadow 0.2s ease,
      opacity 0.2s ease;
    box-shadow:
      0 24px 70px color-mix(in srgb, var(--choir-shadow-color) 52%, transparent),
      0 12px 42px color-mix(in srgb, var(--choir-accent) 14%, transparent);
  }

  .window.overview-preview-live.window-active {
    box-shadow:
      0 28px 86px var(--choir-state-active-glow),
      0 0 44px var(--choir-state-active-glow);
  }

  .window.overview-preview-card,
  .window.overview-preview-redacted,
  .window.overview-preview-suspended {
    opacity: 0;
    pointer-events: none;
    transform: scale(0.92);
  }

  .window.overview-preview-live .titlebar {
    cursor: pointer;
  }

  .window-active {
    box-shadow:
      0 30px 88px color-mix(in srgb, var(--choir-shadow-color) 52%, transparent),
      0 0 54px color-mix(in srgb, var(--choir-accent) 24%, transparent),
      inset 0 0 0 1px color-mix(in srgb, var(--choir-accent) 38%, transparent),
      inset 0 1px 0 color-mix(in srgb, var(--choir-text-primary) 9%, transparent);
  }

  /* ---- Title bar ---- */
  .titlebar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0 0.45rem 0 0.85rem;
    height: 38px;
    min-height: 38px;
    background:
      linear-gradient(
        180deg,
        color-mix(in srgb, var(--choir-text-primary) 5%, transparent),
        transparent 58%
      ),
      color-mix(in srgb, var(--choir-surface-pane) 92%, transparent);
    box-shadow: 0 1px 0 color-mix(in srgb, var(--choir-border) 55%, transparent);
    cursor: grab;
    flex-shrink: 0;
    touch-action: none;
  }

  .titltexture {
    font-size: 0.8rem;
    font-weight: 650;
    letter-spacing: 0.01em;
    color: var(--choir-text-accent);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    flex: 1;
  }

  .window-controls {
    display: flex;
    align-items: center;
    gap: 4px;
    flex-shrink: 0;
  }

  .ctrl-btn {
    width: 28px;
    height: 28px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: color-mix(in srgb, var(--choir-surface-control) 70%, transparent);
    border: none;
    border-radius: 999px;
    font-size: 0.68rem;
    cursor: pointer;
    color: var(--choir-text-muted);
    transition: background 0.15s, color 0.15s, transform 0.15s;
  }

  .ctrl-btn:hover {
    background: color-mix(in srgb, var(--choir-text-primary) 12%, transparent);
    color: var(--choir-text-primary);
    transform: translateY(-0.5px);
  }

  .close-btn:hover {
    background: var(--choir-status-danger-soft);
    color: var(--choir-status-danger);
  }

  /* ---- Content area ---- */
  .window-content {
    flex: 1;
    overflow: auto;
    position: relative;
    min-height: 0;
    background-color: var(--choir-surface-app);
    background-clip: padding-box;
    isolation: isolate;
    user-select: text;
  }

  .window[data-window-app-id='podcast'] .window-content,
  .window[data-window-app-id='texture'] .window-content,
  .window[data-window-app-id='image'] .window-content,
  .window[data-window-app-id='audio'] .window-content,
  .window[data-window-app-id='video'] .window-content,
  .window[data-window-app-id='pdf'] .window-content,
  .window[data-window-app-id='epub'] .window-content,
  .window[data-window-app-id='features'] .window-content,
  .window[data-window-app-id='super-console'] .window-content {
    overflow: hidden;
  }

  /* ---- Resize handle: bottom-right corner only ---- */
  .resize-handle {
    position: absolute;
    z-index: 10;
  }

  .resize-se {
    bottom: 0;
    right: 0;
    width: 16px;
    height: 16px;
    cursor: se-resize;
    touch-action: none;
  }

  /* Subtle visual indicator for the resize handle */
  .resize-se::after {
    content: '';
    position: absolute;
    bottom: 3px;
    right: 3px;
    width: 8px;
    height: 8px;
    background: radial-gradient(circle at 100% 100%, var(--choir-surface-card), transparent 60%);
    border-radius: 999px;
  }

  @media (max-width: 1024px) and (min-width: 769px) {
    .window {
      max-width: calc(100vw - 32px);
    }
  }

  @media (max-width: 768px) {
    .window {
      max-width: calc(100vw - 16px);
      max-height: calc(100dvh - var(--choir-prompt-surface-top-offset, 0px) - var(--choir-prompt-surface-bottom-offset, 64px) - 8px);
    }

    .titlebar {
      height: 42px;
      min-height: 42px;
    }

    .ctrl-btn {
      width: 32px;
      height: 32px;
    }

    .resize-se {
      width: 28px;
      height: 28px;
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .window.overview-preview-live {
      transition: none;
    }

    .ctrl-btn {
      transition: none;
    }
  }
</style>
