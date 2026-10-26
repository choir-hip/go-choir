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
  // Fallback only: the prompt bar publishes its measured height into
  // --choir-prompt-surface-size / -bottom-offset; 56px is the bar's real height
  // before the first measurement.
  const DEFAULT_PROMPT_SURFACE_SIZE = 56;
  const MOBILE_BREAKPOINT = 768;
  const EDGE_RESIZE_ZONE = 8;
  const CORNER_RESIZE_ZONE = 16;
  const MOBILE_CORNER_RESIZE_ZONE = 28;
  const TITLEBAR_REACHABLE = 40;
  // How far past the top of the free area a title bar has to be dragged, and
  // held, before the snap-to-maximize preview shows.
  const SNAP_TOP_THRESHOLD = 1;
  // How far a press on a maximized title bar must travel before the window
  // restores under the pointer; below this a plain click keeps it maximized.
  const DRAG_RESTORE_THRESHOLD = 6;

  // ---- Free-area measurement ----

  let viewportWidth = DEFAULT_VIEWPORT_WIDTH;
  let viewportHeight = DEFAULT_VIEWPORT_HEIGHT;
  let promptSurfaceTopOffset = 0;
  let promptSurfaceBottomOffset = DEFAULT_PROMPT_SURFACE_SIZE;
  let safeAreaTop = 0;
  let safeAreaLeft = 0;
  let safeAreaRight = 0;
  let promptSurfaceObserver = null;
  let desktopAreaObserver = null;

  // ---- Drag state ----

  let dragging = false;
  let dragOffsetX = 0;
  let dragOffsetY = 0;
  let dragPointerId = null;
  let dragPointerTarget = null;
  let snapPreviewActive = false;
  // A press on a maximized title bar only restores the window once it has
  // travelled DRAG_RESTORE_THRESHOLD px; until then a plain click or a
  // double-click keeps the window maximized.
  let dragStartX = 0;
  let dragStartY = 0;
  let dragTraveled = false;

  // ---- Resize state ----

  let resizing = false;
  let resizeDirection = '';
  let resizeStartX = 0;
  let resizeStartY = 0;
  let resizeStartLeft = 0;
  let resizeStartTop = 0;
  let resizeStartWidth = 0;
  let resizeStartHeight = 0;
  let resizePointerId = null;
  let resizePointerTarget = null;
  // The window the title bar's pointerdown landed on. Move and resize handlers
  // listen on window, so each window ignores listeners for another window's
  // gesture.
  let gestureWindowId = '';

  function clamp(value, min, max) {
    return Math.min(Math.max(value, min), Math.max(min, max));
  }

  function parsePixelValue(value, fallback) {
    const parsed = Number.parseFloat(value);
    return Number.isFinite(parsed) ? parsed : fallback;
  }

  function readRootPixelValue(name, fallback) {
    if (typeof document === 'undefined') return fallback;
    return parsePixelValue(window.getComputedStyle(document.documentElement).getPropertyValue(name), fallback);
  }

  function readPromptSurfaceOffsets() {
    if (typeof document === 'undefined') {
      return { top: 0, bottom: DEFAULT_PROMPT_SURFACE_SIZE };
    }

    const themeTop = readRootPixelValue('--choir-prompt-surface-top-offset', 0);
    const themeBottom = readRootPixelValue('--choir-prompt-surface-bottom-offset', DEFAULT_PROMPT_SURFACE_SIZE);

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
    // The desktop area applies env(safe-area-inset-*) once (see app.css);
    // read those values so this window matches the pixels it lives in.
    const desktopArea = document.querySelector('[data-desktop-windows]');
    if (desktopArea) {
      const computed = window.getComputedStyle(desktopArea);
      safeAreaTop = parsePixelValue(computed.getPropertyValue('--choir-safe-area-inset-top'), 0);
      safeAreaLeft = parsePixelValue(computed.getPropertyValue('--choir-safe-area-inset-left'), 0);
      safeAreaRight = parsePixelValue(computed.getPropertyValue('--choir-safe-area-inset-right'), 0);
    }
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

    // A maximized window drags too: the first movement restores it under the
    // pointer (see handleDragMove).
    dragging = true;
    gestureWindowId = windowId;
    dragStartX = event.clientX;
    dragStartY = event.clientY;
    dragTraveled = false;
    dragOffsetX = event.clientX - renderedX;
    dragOffsetY = event.clientY - renderedY;
    dragPointerId = event.pointerId;
    dragPointerTarget = event.currentTarget;
    trySetPointerCapture(dragPointerTarget, dragPointerId);

    // A double-click on a maximized title bar still restores it: the press that
    // starts it never travels, so the window stays maximized until the
    // dblclick handler fires (see handleTitlebarDoubleClick).
    handleFocusWindow();
    event.preventDefault();
  }

  function handleDragMove(event) {
    if (!dragging || gestureWindowId !== windowId) return;
    if (dragPointerId != null && event.pointerId !== dragPointerId) return;

    if (mode === 'maximized') {
      // Dragging a maximized title bar restores the window, but only after the
      // press has travelled DRAG_RESTORE_THRESHOLD px: a plain click (or the
      // first click of a double-click) leaves it maximized.
      if (!dragTraveled) {
        const distance = Math.hypot(event.clientX - dragStartX, event.clientY - dragStartY);
        if (distance <= DRAG_RESTORE_THRESHOLD) return;
        dragTraveled = true;

        // Restore under the pointer, keeping the pointer at the same relative
        // position along the title bar it grabbed: the restored window is the
        // one it had before maximizing, shifted so the grab point stays put,
        // with its title bar under the pointer's height.
        //
        // The grab point is measured against the frame the pointer actually
        // pressed, not against the geometry the store holds. While a window is
        // maximized the frame is drawn at the free area (see windowStyle) and
        // renderedX / renderedWidth still describe the stored normal geometry:
        // a snap-to-top maximize keeps x where the drag released it, so
        // (dragStartX - renderedX) / renderedWidth read 200/700 instead of
        // 500/1280 and the window restored at the pointer (left edge 300)
        // rather than under the same fraction along it (left edge 273).
        // Measuring the pressed frame fixes both maximize paths with one rule.
        const normalWidth =
          restoredGeometry?.width ||
          (mode === 'maximized' && renderedWidth < freeArea.width
            ? renderedWidth
            : Math.round(freeArea.width * 0.7));
        const frameLeft = mode === 'maximized' ? freeArea.x : renderedX;
        const frameWidth = mode === 'maximized' ? freeArea.width : renderedWidth;
        const grabRatio = Math.min(
          Math.max((dragStartX - frameLeft) / Math.max(1, frameWidth), 0),
          1
        );
        dragOffsetX = Math.round(normalWidth * grabRatio);
        dragOffsetY = Math.round(Math.min(TITLEBAR_REACHABLE, 18));
        dispatch('restore', { windowId });
      }
    }

    const newX = event.clientX - dragOffsetX;
    const newY = event.clientY - dragOffsetY;
    dispatch('move', { windowId, x: newX, y: newY });

    // Snap: held at the top edge of the free area -> preview the maximized
    // area; release maximizes (see handleDragEnd).
    snapPreviewActive = event.clientY <= freeArea.y + SNAP_TOP_THRESHOLD;
  }

  function handleDragEnd(event) {
    if (!dragging || gestureWindowId !== windowId) return;
    if (dragPointerId != null && event?.pointerId != null && event.pointerId !== dragPointerId) return;
    tryReleasePointerCapture(dragPointerTarget, dragPointerId);
    const shouldSnap = snapPreviewActive && mode !== 'maximized';
    dragging = false;
    snapPreviewActive = false;
    dragTraveled = false;
    dragPointerId = null;
    dragPointerTarget = null;
    gestureWindowId = '';
    if (shouldSnap) {
      dispatch('maximize', { windowId });
    }
  }

  // ---- Resize handlers (four edges, four corners, normal mode) ----

  function handleResizeStart(event, direction) {
    if (overviewOpen) return;
    if (mode !== 'normal') return;
    if (event.pointerType === 'mouse' && event.button !== 0) return;

    resizing = true;
    resizeDirection = direction;
    gestureWindowId = windowId;
    resizeStartX = event.clientX;
    resizeStartY = event.clientY;
    resizeStartLeft = renderedX;
    resizeStartTop = renderedY;
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
    if (!resizing || gestureWindowId !== windowId) return;
    if (resizePointerId != null && event.pointerId !== resizePointerId) return;

    const dx = event.clientX - resizeStartX;
    const dy = event.clientY - resizeStartY;
    const dir = resizeDirection;

    let newX = resizeStartLeft;
    let newY = resizeStartTop;
    let newWidth = resizeStartWidth;
    let newHeight = resizeStartHeight;

    if (dir.includes('e')) {
      newWidth = resizeStartWidth + dx;
    }
    if (dir.includes('s')) {
      newHeight = resizeStartHeight + dy;
    }
    if (dir.includes('w')) {
      // The right edge stays put while the left edge follows the pointer.
      newWidth = resizeStartWidth - dx;
      newX = resizeStartLeft + (resizeStartWidth - newWidth);
    }
    if (dir.includes('n')) {
      // The bottom edge stays put while the top edge follows the pointer.
      newHeight = resizeStartHeight - dy;
      newY = resizeStartTop + (resizeStartHeight - newHeight);
    }

    // The minimum size always wins over the anchored edge, so the dragged edge
    // stops at a fixed position instead of drifting.
    if (newWidth < MIN_WIDTH) {
      if (dir.includes('w')) newX = resizeStartLeft + resizeStartWidth - MIN_WIDTH;
      newWidth = MIN_WIDTH;
    }
    if (newHeight < MIN_HEIGHT) {
      if (dir.includes('n')) newY = resizeStartTop + resizeStartHeight - MIN_HEIGHT;
      newHeight = MIN_HEIGHT;
    }

    // A resize stops at the free area's edges. Pulling the top edge past the
    // free area's top (or the left edge past its left) stops that edge exactly
    // there and keeps the opposite edge where it was, so the window shrinks
    // instead of sliding: the bottom edge stays at resizeStartTop +
    // resizeStartHeight and the right edge at resizeStartLeft + resizeStartWidth.
    if (dir.includes('n') && newY < freeArea.y) {
      newY = freeArea.y;
      newHeight = resizeStartTop + resizeStartHeight - newY;
    }
    if (dir.includes('w') && newX < freeArea.x) {
      newX = freeArea.x;
      newWidth = resizeStartLeft + resizeStartWidth - newX;
    }

    // The anchor names the edge that stays put when the store's own constraint
    // has to move an edge: a west drag keeps the right edge, a north drag keeps
    // the bottom edge, and a plain east/south drag keeps the left/top.
    const anchor = dir.includes('w') ? 'ne' : dir.includes('n') ? 'sw' : 'nw';
    dispatch('resize', { windowId, x: newX, y: newY, width: newWidth, height: newHeight, anchor });
  }

  function handleResizeEnd(event) {
    if (!resizing || gestureWindowId !== windowId) return;
    if (resizePointerId != null && event?.pointerId != null && event.pointerId !== resizePointerId) return;
    tryReleasePointerCapture(resizePointerTarget, resizePointerId);
    resizing = false;
    resizeDirection = '';
    resizePointerId = null;
    resizePointerTarget = null;
    gestureWindowId = '';
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

    if (typeof ResizeObserver !== 'undefined') {
      const promptSurface = document.querySelector('[data-prompt-surface]');
      if (promptSurface) {
        promptSurfaceObserver = new ResizeObserver(refreshViewportBounds);
        promptSurfaceObserver.observe(promptSurface);
      }
      // The desktop area is where the safe-area insets land, so watching it
      // keeps the free area right when a notch appears or the screen rotates.
      const desktopArea = document.querySelector('[data-desktop-windows]');
      if (desktopArea) {
        desktopAreaObserver = new ResizeObserver(refreshViewportBounds);
        desktopAreaObserver.observe(desktopArea);
      }
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
    desktopAreaObserver?.disconnect();
  });

  // ---- Free area and rendering ----

  // The one free area: the viewport inset once by the safe-area insets, from
  // the top edge to the top of the prompt bar. The store clamps geometry to
  // exactly this rectangle; nothing here adds a margin of its own.
  $: freeArea = {
    x: safeAreaLeft,
    y: safeAreaTop + promptSurfaceTopOffset,
    width: Math.max(MIN_WIDTH, viewportWidth - safeAreaLeft - safeAreaRight),
    height: Math.max(
      MIN_HEIGHT,
      viewportHeight - safeAreaTop - promptSurfaceTopOffset - promptSurfaceBottomOffset
    ),
  };

  // The title bar's height, read from its box: 36px, or 40px under 768px.
  let titlebarHeight = 36;
  let windowElement = null;

  $: if (windowElement) measureTitlebar(windowElement);

  function measureTitlebar(target) {
    const bar = target?.querySelector?.('[data-window-titlebar]');
    if (!bar) return;
    const height = bar.getBoundingClientRect().height;
    if (height > 0) titlebarHeight = Math.round(height);
  }

  // Render what the store holds. Only the title bar's 40px reachability is
  // enforced here, so a window can never be dragged fully off the free area.
  $: renderedWidth = clamp(width, MIN_WIDTH, freeArea.width);
  $: renderedHeight = clamp(height, MIN_HEIGHT, freeArea.height);
  $: renderedX = clamp(x, freeArea.x + TITLEBAR_REACHABLE - renderedWidth, freeArea.x + freeArea.width - TITLEBAR_REACHABLE);
  $: renderedY = clamp(y, freeArea.y, freeArea.y + freeArea.height - TITLEBAR_REACHABLE);

  $: windowStyle = mode === 'maximized'
    ? `left:${freeArea.x}px; top:${freeArea.y}px; width:${freeArea.width}px; height:${freeArea.height}px;`
    : mode === 'minimized'
    ? 'display:none;'
    : `left:${renderedX}px; top:${renderedY}px; width:${renderedWidth}px; height:${renderedHeight}px;`;

  $: snapPreviewStyle = `left:${freeArea.x}px; top:${freeArea.y}px; width:${freeArea.width}px; height:${freeArea.height}px;`;
  $: showSnapPreview = snapPreviewActive && dragging && mode !== 'maximized';

  $: maxRestoreTitle = mode === 'maximized' ? 'Restore' : 'Maximize';
  $: showResizeHandles = mode === 'normal' && !overviewOpen;
  // Eight zones: every edge is an 8px strip on every screen; only the four
  // corners grow to 28px under 768px for a finger. The top edge and the top
  // corners start below the title bar, so no zone ever covers the title bar's
  // minimize/maximize/close buttons (a pointerdown there must reach them).
  $: compactZones = viewportWidth < MOBILE_BREAKPOINT;
  $: cornerZone = compactZones ? MOBILE_CORNER_RESIZE_ZONE : CORNER_RESIZE_ZONE;
  $: edgeZone = EDGE_RESIZE_ZONE;
  // Keep every resize zone out from under the title bar and its buttons.
  $: topZoneInset = Math.max(cornerZone, titlebarHeight + edgeZone);
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
{#if showSnapPreview}
  <div class="snap-preview" style={snapPreviewStyle} data-snap-preview aria-hidden="true"></div>
{/if}

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
  bind:this={windowElement}
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
      >
        <svg class="ctrl-glyph" viewBox="0 0 12 12" aria-hidden="true" focusable="false">
          <line x1="2.5" y1="6" x2="9.5" y2="6" />
        </svg>
      </button>
      <button
        class="ctrl-btn maximize-btn"
        data-window-maximize
        on:click|stopPropagation={handleMaximizeRestore}
        title={maxRestoreTitle}
        aria-label={maxRestoreTitle}
      >
        {#if mode === 'maximized'}
          <svg class="ctrl-glyph" viewBox="0 0 12 12" aria-hidden="true" focusable="false">
            <rect x="2.4" y="3.6" width="6" height="6" rx="1.4" />
            <path d="M4.6 3.6V2.7A1.3 1.3 0 0 1 5.9 1.4h2.9A1.3 1.3 0 0 1 10.1 2.7v2.9A1.3 1.3 0 0 1 8.8 6.9h-0.9" />
          </svg>
        {:else}
          <svg class="ctrl-glyph" viewBox="0 0 12 12" aria-hidden="true" focusable="false">
            <rect x="2.5" y="2.5" width="7" height="7" rx="1.6" />
          </svg>
        {/if}
      </button>
      <button
        class="ctrl-btn close-btn"
        data-window-close
        on:click|stopPropagation={handleClose}
        title="Close"
        aria-label="Close"
      >
        <svg class="ctrl-glyph" viewBox="0 0 12 12" aria-hidden="true" focusable="false">
          <line x1="3" y1="3" x2="9" y2="9" />
          <line x1="9" y1="3" x2="3" y2="9" />
        </svg>
      </button>
    </div>
  </div>

  <!-- Content area -->
  <div class="window-content" data-window-content>
    <slot />
  </div>

  <!--
    Resize zones: four edges and four corners, normal mode only. Edges are
    thin strips of the window's outer 8px; corners own the outer corner square
    (16px on desktop, 28px under 768px). Each zone covers only its own strip,
    so a pointer aimed at an edge never lands in a corner zone. The top edge
    and the top corners start below the title bar, since a zone above them
    would intercept the pointer aimed at the title bar's buttons.
  -->
  {#if showResizeHandles}
    <div
      class="resize-zone resize-e"
      data-resize-handle
      style="top:{cornerZone}px; bottom:{cornerZone}px; width:{edgeZone}px;"
      on:pointerdown|stopPropagation={(event) => handleResizeStart(event, 'e')}
    ></div>
    <div
      class="resize-zone resize-w"
      data-resize-handle
      style="top:{cornerZone}px; bottom:{cornerZone}px; width:{edgeZone}px;"
      on:pointerdown|stopPropagation={(event) => handleResizeStart(event, 'w')}
    ></div>
    <div
      class="resize-zone resize-s"
      data-resize-handle
      style="left:{cornerZone}px; right:{cornerZone}px; height:{edgeZone}px;"
      on:pointerdown|stopPropagation={(event) => handleResizeStart(event, 's')}
    ></div>
    <div
      class="resize-zone resize-n"
      data-resize-handle
      style="left:{cornerZone}px; right:{cornerZone}px; top:{titlebarHeight}px; height:{edgeZone}px;"
      on:pointerdown|stopPropagation={(event) => handleResizeStart(event, 'n')}
    ></div>
    <div
      class="resize-zone resize-ne"
      data-resize-handle
      style="top:{topZoneInset}px; width:{cornerZone}px; height:{cornerZone}px;"
      on:pointerdown|stopPropagation={(event) => handleResizeStart(event, 'ne')}
    ></div>
    <div
      class="resize-zone resize-nw"
      data-resize-handle
      style="top:{topZoneInset}px; width:{cornerZone}px; height:{cornerZone}px;"
      on:pointerdown|stopPropagation={(event) => handleResizeStart(event, 'nw')}
    ></div>
    <div
      class="resize-zone resize-se"
      data-resize-handle
      style="width:{cornerZone}px; height:{cornerZone}px;"
      on:pointerdown|stopPropagation={(event) => handleResizeStart(event, 'se')}
    ><span class="resize-grip" aria-hidden="true"></span></div>
    <div
      class="resize-zone resize-sw"
      data-resize-handle
      style="width:{cornerZone}px; height:{cornerZone}px;"
      on:pointerdown|stopPropagation={(event) => handleResizeStart(event, 'sw')}
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
      0 10px 30px color-mix(in srgb, var(--choir-accent) 10%, transparent);
    transform-origin: top left;
    transition: box-shadow 0.15s, filter 0.15s, border-radius 0.12s;
    user-select: none;
    /* No max-width / max-height: the store already sized and clamped this
       window to the free area, and a cap here would hold a maximized window
       off the screen edge (the wasted strip this rewrite removes). */
  }

  /* Maximized means the whole free area, edge to edge: square corners, no
     shadow, no border. The style attribute from Svelte carries the rect. */
  .window[data-window-mode='maximized'] {
    border-radius: 0;
    box-shadow: none;
    border: 0;
    max-width: none;
    max-height: none;
  }

  .snap-preview {
    position: absolute;
    z-index: 0;
    pointer-events: none;
    border-radius: var(--choir-radius-panel, 26px);
    border: 2px dashed color-mix(in srgb, var(--choir-accent) 70%, transparent);
    background: color-mix(in srgb, var(--choir-accent) 12%, transparent);
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
      0 0 54px color-mix(in srgb, var(--choir-accent) 24%, transparent);
  }

  .window[data-window-mode='maximized'].window-active {
    box-shadow: none;
  }

  /* ---- Title bar ---- */
  .titlebar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0 0.5rem 0 0.75rem;
    height: 36px;
    min-height: 36px;
    background-color: var(--choir-surface-pane);
    background-image: linear-gradient(
      color-mix(in srgb, var(--choir-surface-pane) 86%, transparent),
      color-mix(in srgb, var(--choir-surface-pane) 86%, transparent)
    );
    box-shadow: 0 14px 30px color-mix(in srgb, var(--choir-shadow-color) 18%, transparent);
    cursor: grab;
    flex-shrink: 0;
    touch-action: none;
  }

  .titltexture {
    font-size: 0.8rem;
    font-weight: 600;
    color: var(--choir-text-accent);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    flex: 1;
  }

  .window-controls {
    display: flex;
    align-items: center;
    gap: 2px;
    flex-shrink: 0;
  }

  .ctrl-btn {
    width: 28px;
    height: 28px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: transparent;
    border: none;
    border-radius: var(--choir-radius-control-sm, 14px);
    cursor: pointer;
    color: var(--choir-text-muted);
    transition: background 0.15s, color 0.15s;
  }

  .ctrl-glyph {
    width: 13px;
    height: 13px;
    fill: none;
    stroke: currentColor;
    stroke-width: 1.5;
    stroke-linecap: round;
    stroke-linejoin: round;
  }

  .ctrl-btn:hover,
  .ctrl-btn:focus-visible {
    background: color-mix(in srgb, var(--choir-text-primary) 10%, transparent);
    color: var(--choir-text-primary);
    outline: none;
  }

  .close-btn:hover,
  .close-btn:focus-visible {
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
  .window[data-window-app-id='features'] .window-content {
    overflow: hidden;
  }

  /* ---- Resize zones: four edges, four corners ---- */
  .resize-zone {
    position: absolute;
    z-index: 10;
    touch-action: none;
  }

  .resize-e {
    top: 0;
    right: 0;
    bottom: 0;
    cursor: ew-resize;
  }

  .resize-w {
    top: 0;
    left: 0;
    bottom: 0;
    cursor: ew-resize;
  }

  .resize-s {
    left: 0;
    right: 0;
    bottom: 0;
    cursor: ns-resize;
  }

  .resize-n {
    left: 0;
    right: 0;
    top: 0;
    cursor: ns-resize;
  }

  .resize-ne {
    top: 0;
    right: 0;
    cursor: nesw-resize;
  }

  .resize-nw {
    top: 0;
    left: 0;
    cursor: nwse-resize;
  }

  .resize-se {
    bottom: 0;
    right: 0;
    cursor: nwse-resize;
  }

  .resize-sw {
    bottom: 0;
    left: 0;
    cursor: nesw-resize;
  }

  /* Subtle visual indicator on the bottom-right corner, as before. */
  .resize-grip {
    position: absolute;
    bottom: 3px;
    right: 3px;
    width: 8px;
    height: 8px;
    background: radial-gradient(circle at 100% 100%, var(--choir-surface-card), transparent 60%);
    border-radius: 999px;
    pointer-events: none;
  }

  @media (max-width: 768px) {
    .titlebar {
      height: 40px;
      min-height: 40px;
    }

    .ctrl-btn {
      width: 32px;
      height: 32px;
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .window,
    .window.overview-preview-live {
      transition: none;
    }
  }
</style>
