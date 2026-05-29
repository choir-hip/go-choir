<!--
  FloatingDesktopIcons — freely-draggable desktop icons on the desktop surface.

  Displays 4 app icons (Files, Browser, Terminal, Settings) as emoji + label,
  positioned in a grid by default. Users can drag icons to any position.
  Icon positions persist via the desktop state API.

  Interactions:
    - Single-click: selects the icon (visual highlight)
    - Double-click: opens/focuses the app window (single-instance)
    - Drag: moves the icon freely on the desktop surface

  Icons sit at z-index below windows so windows cover them.

  Data attributes for test targeting:
    data-desktop-surface   — the desktop surface container holding icons
    data-desktop-icon      — individual icon entry
    data-desktop-icon-id   — app identifier on the icon
    data-desktop-icon-emoji — emoji span
    data-desktop-icon-label — text label span
-->
<script>
  import { createEventDispatcher } from 'svelte';
  import { onMount, onDestroy } from 'svelte';
  import {
    DESKTOP_ICON_APPS,
    windows,
    activeWindowId,
    iconPositions,
    selectedIconId,
    moveIcon,
  } from './stores/desktop.js';

  const dispatch = createEventDispatcher();
  const DESKTOP_ICON_WIDTH = 80;
  const DESKTOP_ICON_HEIGHT = 76;
  const MOBILE_ICON_WIDTH = 72;
  const MOBILE_ICON_HEIGHT = 66;
  const ICON_GRID_X = 24;
  const ICON_GRID_Y = 24;
  const ICON_GRID_COL_WIDTH = 96;
  const ICON_GRID_ROW_HEIGHT = 86;

  // ---- Drag state ----
  let dragging = false;
  let dragAppId = null;
  let dragOffsetX = 0;
  let dragOffsetY = 0;
  let surfaceEl;
  let resizeObserver;
  let normalizeFrame = 0;

  function iconMetrics() {
    const iconEl = surfaceEl?.querySelector('[data-desktop-icon]');
    if (iconEl) {
      const rect = iconEl.getBoundingClientRect();
      if (rect.width > 0 && rect.height > 0) {
        return { width: Math.ceil(rect.width), height: Math.ceil(rect.height) };
      }
    }
    const compact = typeof window !== 'undefined' && window.matchMedia('(max-width: 768px)').matches;
    return compact
      ? { width: MOBILE_ICON_WIDTH, height: MOBILE_ICON_HEIGHT }
      : { width: DESKTOP_ICON_WIDTH, height: DESKTOP_ICON_HEIGHT };
  }

  function safeBounds() {
    const rect = surfaceEl?.getBoundingClientRect();
    const metrics = iconMetrics();
    const width = Math.max(metrics.width, rect?.width || 0);
    const height = Math.max(metrics.height, rect?.height || 0);
    return {
      width,
      height,
      iconWidth: metrics.width,
      iconHeight: metrics.height,
      maxX: Math.max(0, width - metrics.width),
      maxY: Math.max(0, height - metrics.height),
    };
  }

  function clampPosition(pos, bounds = safeBounds()) {
    return {
      x: Math.max(0, Math.min(Number(pos?.x) || 0, bounds.maxX)),
      y: Math.max(0, Math.min(Number(pos?.y) || 0, bounds.maxY)),
    };
  }

  function gridPosition(index, bounds) {
    const usableHeight = Math.max(bounds.iconHeight, bounds.height - ICON_GRID_Y);
    const rows = Math.max(1, Math.floor((usableHeight + ICON_GRID_ROW_HEIGHT - bounds.iconHeight) / ICON_GRID_ROW_HEIGHT));
    const row = index % rows;
    const col = Math.floor(index / rows);
    return clampPosition(
      {
        x: ICON_GRID_X + col * ICON_GRID_COL_WIDTH,
        y: ICON_GRID_Y + row * ICON_GRID_ROW_HEIGHT,
      },
      bounds
    );
  }

  function positionIsVisible(pos, bounds) {
    return (
      pos &&
      Number.isFinite(pos.x) &&
      Number.isFinite(pos.y) &&
      pos.x >= 0 &&
      pos.y >= 0 &&
      pos.x <= bounds.maxX &&
      pos.y <= bounds.maxY
    );
  }

  function normalizeIconPositions() {
    if (!surfaceEl) return;
    const bounds = safeBounds();
    const positions = $iconPositions || {};
    const needsReflow = DESKTOP_ICON_APPS.some((app) => !positionIsVisible(positions[app.id], bounds));
    if (!needsReflow) return;

    const nextPositions = {};
    DESKTOP_ICON_APPS.forEach((app, index) => {
      nextPositions[app.id] = gridPosition(index, bounds);
    });
    iconPositions.set(nextPositions);
    dispatch('iconpositionschanged');
  }

  function scheduleNormalizeIconPositions() {
    if (normalizeFrame) cancelAnimationFrame(normalizeFrame);
    normalizeFrame = requestAnimationFrame(() => {
      normalizeFrame = 0;
      normalizeIconPositions();
    });
  }

  /** Check if an app has an open (non-closed) window */
  function isAppOpen($windows, appId) {
    return $windows.some((w) => w.appId === appId && w.mode !== 'closed');
  }

  /** Check if an app's window is the active one */
  function isAppActive($windows, $activeWindowId, appId) {
    const activeWin = $windows.find((w) => w.windowId === $activeWindowId);
    return activeWin && activeWin.appId === appId;
  }

  /** Handle single click — select icon */
  function handleClick(app) {
    selectedIconId.set(app.id);
  }

  /** Handle double click — open/focus app window */
  function handleDblClick(app) {
    dispatch('launchapp', { appId: app.id, appName: app.name, icon: app.icon });
  }

  /** Handle drag start on icon */
  function handleDragStart(event, app) {
    if (event.button !== 0) return;
    event.preventDefault();
    event.stopPropagation();

    dragging = true;
    dragAppId = app.id;

    const surfaceRect = surfaceEl?.getBoundingClientRect();
    const pos = $iconPositions[app.id] || { x: 0, y: 0 };
    dragOffsetX = event.clientX - (surfaceRect?.left || 0) - pos.x;
    dragOffsetY = event.clientY - (surfaceRect?.top || 0) - pos.y;

    selectedIconId.set(app.id);
  }

  /** Handle mouse move during drag */
  function handleMouseMove(event) {
    if (!dragging || !dragAppId) return;

    if (!surfaceEl) return;

    const rect = surfaceEl.getBoundingClientRect();
    const bounds = safeBounds();
    const newX = Math.max(0, Math.min(event.clientX - rect.left - dragOffsetX, bounds.maxX));
    const newY = Math.max(0, Math.min(event.clientY - rect.top - dragOffsetY, bounds.maxY));

    moveIcon(dragAppId, newX, newY);
  }

  /** Handle mouse up — end drag */
  function handleMouseUp() {
    if (dragging) {
      dragging = false;
      dragAppId = null;
      dispatch('iconpositionschanged');
    }
  }

  // Touch support for mobile drag
  let touchDragging = false;
  let touchAppId = null;
  let touchOffsetX = 0;
  let touchOffsetY = 0;

  function handleTouchStart(event, app) {
    if (event.touches.length !== 1) return;
    const touch = event.touches[0];

    touchDragging = true;
    touchAppId = app.id;

    const surfaceRect = surfaceEl?.getBoundingClientRect();
    const pos = $iconPositions[app.id] || { x: 0, y: 0 };
    touchOffsetX = touch.clientX - (surfaceRect?.left || 0) - pos.x;
    touchOffsetY = touch.clientY - (surfaceRect?.top || 0) - pos.y;

    selectedIconId.set(app.id);
  }

  function handleTouchMove(event) {
    if (!touchDragging || !touchAppId) return;
    event.preventDefault();

    const touch = event.touches[0];
    if (!surfaceEl) return;

    const rect = surfaceEl.getBoundingClientRect();
    const bounds = safeBounds();
    const newX = Math.max(0, Math.min(touch.clientX - rect.left - touchOffsetX, bounds.maxX));
    const newY = Math.max(0, Math.min(touch.clientY - rect.top - touchOffsetY, bounds.maxY));

    moveIcon(touchAppId, newX, newY);
  }

  function handleTouchEnd() {
    if (touchDragging) {
      touchDragging = false;
      touchAppId = null;
      dispatch('iconpositionschanged');
    }
  }

  onMount(() => {
    window.addEventListener('mousemove', handleMouseMove);
    window.addEventListener('mouseup', handleMouseUp);
    window.addEventListener('touchmove', handleTouchMove, { passive: false });
    window.addEventListener('touchend', handleTouchEnd);
    window.addEventListener('resize', scheduleNormalizeIconPositions);
    window.visualViewport?.addEventListener('resize', scheduleNormalizeIconPositions);
    resizeObserver = new ResizeObserver(scheduleNormalizeIconPositions);
    if (surfaceEl) resizeObserver.observe(surfaceEl);
    scheduleNormalizeIconPositions();
  });

  onDestroy(() => {
    if (normalizeFrame) cancelAnimationFrame(normalizeFrame);
    window.removeEventListener('mousemove', handleMouseMove);
    window.removeEventListener('mouseup', handleMouseUp);
    window.removeEventListener('touchmove', handleTouchMove);
    window.removeEventListener('touchend', handleTouchEnd);
    window.removeEventListener('resize', scheduleNormalizeIconPositions);
    window.visualViewport?.removeEventListener('resize', scheduleNormalizeIconPositions);
    resizeObserver?.disconnect();
  });
</script>

<!-- svelte-ignore a11y-click-events-have-key-events -->
<!-- svelte-ignore a11y-no-static-element-interactions -->
<div
  class="desktop-surface"
  bind:this={surfaceEl}
  data-desktop-surface
  on:click={() => selectedIconId.set('')}
>
  {#each DESKTOP_ICON_APPS as app (app.id)}
    {@const pos = $iconPositions[app.id] || { x: 32, y: 32 }}
    {@const isOpen = isAppOpen($windows, app.id)}
    {@const isActive = isAppActive($windows, $activeWindowId, app.id)}
    {@const isSelected = $selectedIconId === app.id}
    <!-- svelte-ignore a11y-click-events-have-key-events -->
    <!-- svelte-ignore a11y-no-static-element-interactions -->
    <div
      class="desktop-icon {isActive ? 'icon-active' : ''} {isSelected ? 'icon-selected' : ''} {isOpen ? 'icon-open' : ''}"
      style="left: {pos.x}px; top: {pos.y}px;"
      data-desktop-icon
      data-desktop-icon-id={app.id}
      on:click|stopPropagation={() => handleClick(app)}
      on:dblclick|stopPropagation={() => handleDblClick(app)}
      on:mousedown|stopPropagation={(e) => handleDragStart(e, app)}
      on:touchstart|stopPropagation={(e) => handleTouchStart(e, app)}
      role="button"
      tabindex="0"
      aria-label={app.name}
    >
      <span class="icon-emoji" data-desktop-icon-emoji>{app.icon}</span>
      <span class="icon-label" data-desktop-icon-label>{app.name}</span>
      {#if isOpen}
        <span class="open-indicator"></span>
      {/if}
    </div>
  {/each}
</div>

<style>
  .desktop-surface {
    position: absolute;
    top: var(--choir-prompt-surface-top-offset, 0px);
    left: 0;
    right: 0;
    bottom: var(--choir-prompt-surface-bottom-offset, 64px);
    z-index: 1; /* Below windows (which start at z-index 2+) */
    overflow: hidden;
  }

  .desktop-icon {
    position: absolute;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 4px;
    width: 80px;
    padding: 8px 4px;
    border-radius: 8px;
    cursor: pointer;
    user-select: none;
    transition: background 0.15s, border-color 0.15s;
    border: 2px solid transparent;
    background: transparent;
  }

  .desktop-icon:hover {
    background: rgba(255, 255, 255, 0.06);
  }

  .desktop-icon:focus-visible {
    outline: 2px solid #3b82f6;
    outline-offset: 2px;
  }

  .desktop-icon.icon-selected {
    background: rgba(255, 255, 255, 0.08);
    border-color: rgba(255, 255, 255, 0.15);
  }

  .desktop-icon.icon-active {
    background: rgba(59, 130, 246, 0.12);
    border-color: rgba(59, 130, 246, 0.3);
  }

  .desktop-icon.icon-active .icon-label {
    color: #e0e0e0;
  }

  .icon-emoji {
    font-size: 2rem;
    line-height: 1;
    pointer-events: none;
  }

  .icon-label {
    font-size: 0.7rem;
    font-weight: 500;
    color: #999;
    text-align: center;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    max-width: 72px;
    pointer-events: none;
  }

  .open-indicator {
    position: absolute;
    bottom: 2px;
    left: 50%;
    transform: translateX(-50%);
    width: 4px;
    height: 4px;
    border-radius: 50%;
    background: #3b82f6;
    pointer-events: none;
  }

  /* Mobile: icons remain the same size and freely positionable */
  @media (max-width: 768px) {
    .desktop-icon {
      width: 72px;
      padding: 6px 2px;
    }

    .icon-emoji {
      font-size: 1.6rem;
    }

    .icon-label {
      font-size: 0.65rem;
    }
  }
</style>
