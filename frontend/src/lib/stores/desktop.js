/**
 * Desktop stores for window management in the go-choir desktop shell.
 *
 * Provides Svelte writable stores for:
 *   - windows: array of open window objects
 *   - activeWindowId: currently focused window ID
 *   - nextZIndex: next z-index counter for window stacking
 *   - liveStatus: WebSocket connection status
 *   - iconPositions: positions of floating desktop icons
 *   - showDesktopMode: whether all windows are minimized (show desktop)
 *   - selectedIconId: currently selected desktop icon (set on icon press)
 *
 * Each window object has:
 *   { windowId, appId, title, icon, x, y, width, height, mode, zIndex, restoredGeometry, appContext }
 *
 * App registry defines the hardcoded apps with their icons.
 */

import { writable, derived } from 'svelte/store';
import {
  APP_REGISTRY,
  LAUNCHER_APPS,
  DESKTOP_ICON_APPS,
  HEAVY_APP_IDS,
  getAppDefinition,
  getAppIcon,
  getAppWindowPreference,
  isHeavyAppId,
  normalizeAppId,
} from '../apps/registry';

// ---- App registry ----

export { APP_REGISTRY, LAUNCHER_APPS, DESKTOP_ICON_APPS, HEAVY_APP_IDS, getAppDefinition, getAppIcon, getAppWindowPreference, isHeavyAppId, normalizeAppId };

// ---- Window counter ----

let windowCounter = 0;

const MIN_WINDOW_WIDTH = 200;
const MIN_WINDOW_HEIGHT = 120;
// Fallbacks only: the prompt bar's real height is published into
// --choir-prompt-surface-size / -bottom-offset by PromptSurface on mount and
// on every resize. Before that first measurement the bar's real 56px is used.
const PROMPT_SURFACE_SIZE = 56;
const COMPACT_BREAKPOINT = 768;
const DEFAULT_VIEWPORT_WIDTH = 1280;
const DEFAULT_VIEWPORT_HEIGHT = 800;
// Share of the free desktop a new window asks for on desktop; the larger of
// this and the app's registry default, capped at the free area.
const NEW_WINDOW_FREE_AREA_RATIO = 0.7;
// Desktop cascade step, and how many steps before the cascade wraps.
const CASCADE_STEP = 24;
const CASCADE_WRAP = 6;
// A restored (non-maximized) window on a phone is at least this much of the
// screen, so restoring from maximized stays usable.
const COMPACT_RESTORE_WIDTH_RATIO = 0.92;
const COMPACT_RESTORE_HEIGHT_RATIO = 0.85;
// The title bar must keep at least this much on screen so a window dragged or
// resized past an edge can always be dragged back.
const TITLEBAR_REACHABLE = 40;
// How far past the top of the free area a title bar must be dragged (and held)
// for the snap-to-maximize preview to appear.
const SNAP_TOP_THRESHOLD = 1;
const WINDOW_Z_INDEX_COMPACT_AT = 80;
function generateWindowId() {
  windowCounter++;
  return `win-${Date.now()}-${windowCounter}`;
}

function clamp(value, min, max) {
  return Math.min(Math.max(value, min), max);
}

function parsePixelValue(value, fallback = 0) {
  const parsed = Number.parseFloat(value);
  return Number.isFinite(parsed) ? parsed : fallback;
}

// The shell applies env(safe-area-inset-*) once, on the desktop area (see
// Desktop.svelte and app.css); the store reads those computed values so window
// geometry agrees with the pixels the windows are positioned in. 0 on every
// screen without a notch.
function readSafeAreaInset(side = 'top') {
  if (typeof window === 'undefined' || typeof document === 'undefined') return 0;
  const target = document.querySelector('[data-desktop-windows]');
  if (!target) return 0;
  return parsePixelValue(
    window.getComputedStyle(target).getPropertyValue('--choir-safe-area-inset-' + side),
    0
  );
}

function getPromptSurfaceOffsets(viewportHeight) {
  if (typeof window === 'undefined' || typeof document === 'undefined') {
    return { top: 0, bottom: PROMPT_SURFACE_SIZE };
  }

  const rootStyle = window.getComputedStyle(document.documentElement);
  const themeTop = parsePixelValue(rootStyle.getPropertyValue('--choir-prompt-surface-top-offset'), 0);
  const themeBottom = parsePixelValue(rootStyle.getPropertyValue('--choir-prompt-surface-bottom-offset'), PROMPT_SURFACE_SIZE);
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

function getViewportMetrics() {
  const viewportWidth = typeof window !== 'undefined' ? window.innerWidth : DEFAULT_VIEWPORT_WIDTH;
  const viewportHeight = typeof window !== 'undefined' ? window.innerHeight : DEFAULT_VIEWPORT_HEIGHT;
  const promptSurfaceOffsets = getPromptSurfaceOffsets(viewportHeight);
  const compact = viewportWidth < COMPACT_BREAKPOINT;

  // The windows live inside the desktop area, which the shell insets once by
  // the safe-area insets (a notch or a rounded phone edge; 0 on a desktop).
  const insetTop = readSafeAreaInset('top');
  const insetLeft = readSafeAreaInset('left');
  const insetRight = readSafeAreaInset('right');

  // The one free area every window lives in: from the top of the desktop area
  // to the top of the prompt bar, edge to edge. No margin. A maximized window
  // is exactly this rectangle, and dragging/resizing may reach every edge of
  // it (the title bar stays reachable, see constrainWindowGeometry).
  const freeArea = {
    x: insetLeft,
    y: insetTop + promptSurfaceOffsets.top,
    width: Math.max(MIN_WINDOW_WIDTH, viewportWidth - insetLeft - insetRight),
    height: Math.max(
      MIN_WINDOW_HEIGHT,
      viewportHeight - insetTop - promptSurfaceOffsets.top - promptSurfaceOffsets.bottom
    ),
  };

  return {
    compact,
    viewportWidth,
    viewportHeight,
    promptSurfaceOffsets,
    freeArea,
    maxWidth: freeArea.width,
    maxHeight: freeArea.height,
  };
}

function appMinimums(appId, metrics) {
  const pref = getAppWindowPreference(appId);
  if (metrics.compact && pref.compact) {
    return {
      width: pref.compact.minWidth || MIN_WINDOW_WIDTH,
      height: pref.compact.minHeight || MIN_WINDOW_HEIGHT,
    };
  }
  const desktop = pref.desktop || {};
  return {
    width: desktop.minWidth || MIN_WINDOW_WIDTH,
    height: desktop.minHeight || MIN_WINDOW_HEIGHT,
  };
}

/**
 * The one constraint every position and size passes through (drag, resize,
 * launch, restore, saved state). Geometry may touch all four edges of the free
 * area; the only extra rule is that TITLEBAR_REACHABLE px of the title bar
 * always stays inside it, so a window can always be dragged back.
 *
 * The anchor says which edges stay put when a minimum size has to win:
 *   'nw' — the right/bottom edges move (dragging, or resizing from se/s/e)
 *   'ne' — the left/bottom edges move (resizing from sw/w)
 *   'sw' — the right/top edges move (resizing from ne/n)
 *
 * resizing: a resize stops with its edges exactly on the free area's left, right
 *   and top and the prompt bar's top and no further (a move keeps the title-bar
 *   reachability rule, so it may slide off screen as long as 40px stays).
 */
function constrainWindowGeometry({ x, y, width, height, appId = '', anchor = 'nw', resizing = false }) {
  const metrics = getViewportMetrics();
  const area = metrics.freeArea;
  const minimums = appMinimums(appId, metrics);
  const clampedWidth = Math.round(clamp(width, Math.min(minimums.width, area.width), area.width));
  const clampedHeight = Math.round(clamp(height, Math.min(minimums.height, area.height), area.height));

  const minX = area.x + TITLEBAR_REACHABLE - clampedWidth;
  const minY = area.y;
  const maxX = area.x + area.width - TITLEBAR_REACHABLE;
  let maxY = area.y + area.height - TITLEBAR_REACHABLE;
  if (anchor === 'sw') {
    // Resizing from the north edge: the bottom edge is fixed, so the top may
    // travel down to the prompt bar's top but never below the title bar rule.
    maxY = Math.max(minY, area.y + area.height - clampedHeight);
  }
  const clampedX = Math.round(clamp(x, Math.min(minX, maxX), maxX));

  if (resizing) {
    // A resize stops at the free area's edges: a window's left, right, and top
    // edges may land exactly on the viewport's left and right edges and the
    // prompt bar's top, and no further. Dragging the right edge or a right
    // corner past the screen therefore never grows the window off screen; the
    // anchor's fixed edge stays put and the app minimum still wins.
    const resizedWidth = Math.round(clamp(width, Math.min(minimums.width, area.width), area.width));
    const resizedHeight = Math.round(clamp(height, Math.min(minimums.height, area.height), area.height));
    const maxRight = area.x + area.width;
    const maxBottom = area.y + area.height;
    // A north or west drag stops the dragged edge on the free area's top/left
    // edge and keeps the opposite edge (the bottom for 'sw', the right for
    // 'ne') exactly where it was, so the window shrinks instead of sliding.
    let resizedWidthAfterEdge = resizedWidth;
    let fixedRightEdge = x + width;
    if (anchor === 'ne' && x < area.x) {
      resizedWidthAfterEdge = Math.max(
        Math.round(clamp(fixedRightEdge - area.x, Math.min(minimums.width, area.width), area.width)),
        resizedWidthAfterEdge
      );
      fixedRightEdge = area.x + resizedWidthAfterEdge;
    }
    let resizedHeightAfterEdge = resizedHeight;
    let fixedBottomEdge = y + height;
    if (anchor === 'sw' && y < area.y) {
      resizedHeightAfterEdge = Math.max(
        Math.round(clamp(fixedBottomEdge - area.y, Math.min(minimums.height, area.height), area.height)),
        resizedHeightAfterEdge
      );
      fixedBottomEdge = area.y + resizedHeightAfterEdge;
    }
    let resizedX;
    if (anchor === 'ne' || anchor === 'sw') {
      // The right edge is fixed: the left moves, and the width may not push it
      // past the free area's left edge.
      const fixedRight = Math.round(clamp(fixedRightEdge, area.x + resizedWidthAfterEdge, maxRight));
      resizedX = fixedRight - resizedWidthAfterEdge;
    } else {
      resizedX = clamp(x, area.x, maxRight - resizedWidthAfterEdge);
    }
    let resizedY;
    if (anchor === 'sw') {
      // The bottom edge is fixed: the top travels up to the free area's top
      // and no further, shrinking the window there.
      const fixedBottom = Math.round(clamp(fixedBottomEdge, area.y + resizedHeightAfterEdge, maxBottom));
      resizedY = fixedBottom - resizedHeightAfterEdge;
    } else {
      resizedY = clamp(y, area.y, maxBottom - resizedHeightAfterEdge);
    }
    return {
      x: Math.round(resizedX),
      y: Math.round(resizedY),
      width: resizedWidthAfterEdge,
      height: resizedHeightAfterEdge,
    };
  }
  const clampedY = Math.round(clamp(y, minY, Math.max(minY, maxY)));

  return {
    x: clampedX,
    y: clampedY,
    width: clampedWidth,
    height: clampedHeight,
  };
}

/**
 * Geometry for a new window.
 * Phone (<768px): windows always open maximized (see openApp); the geometry
 *   kept underneath is the free area.
 * Desktop: the larger of the app's registry default and 70% of the free area,
 *   capped at the free area. The first window is centered in the free area;
 *   later windows cascade by CASCADE_STEP from the top-left of it.
 */
function getNewWindowGeometry(openCount, appId = '') {
  const metrics = getViewportMetrics();
  const area = metrics.freeArea;
  const preference = getAppWindowPreference(appId);
  const desktopPref = preference.desktop || {};
  const compactPref = preference.compact || {};

  if (metrics.compact) {
    const compactWidth = Math.min(compactPref.width || area.width, area.width);
    const compactHeight = Math.min(compactPref.height || area.height, area.height);
    if (openCount === 0) {
      return constrainWindowGeometry({
        x: area.x + Math.round((area.width - compactWidth) / 2),
        y: area.y + Math.round((area.height - compactHeight) / 2),
        width: compactWidth,
        height: compactHeight,
        appId,
      });
    }
    const compactOffset = (openCount % CASCADE_WRAP) * CASCADE_STEP;
    return constrainWindowGeometry({
      x: area.x + compactOffset,
      y: area.y + compactOffset,
      width: compactWidth,
      height: compactHeight,
      appId,
    });
  }

  const defaultWidth = desktopPref.width || area.width;
  const defaultHeight = desktopPref.height || area.height;
  const width = Math.min(
    area.width,
    Math.max(defaultWidth, Math.round(area.width * NEW_WINDOW_FREE_AREA_RATIO))
  );
  const height = Math.min(
    area.height,
    Math.max(defaultHeight, Math.round(area.height * NEW_WINDOW_FREE_AREA_RATIO))
  );

  if (openCount === 0) {
    return constrainWindowGeometry({
      x: area.x + Math.round((area.width - width) / 2),
      y: area.y + Math.round((area.height - height) / 2),
      width,
      height,
      appId,
    });
  }

  const offset = (openCount % CASCADE_WRAP) * CASCADE_STEP;
  return constrainWindowGeometry({
    x: area.x + offset,
    y: area.y + offset,
    width,
    height,
    appId,
  });
}

/**
 * Geometry to settle on when a window leaves 'maximized'. On a phone the
 * restored window is at least 92% wide and 85% of the free height; on desktop
 * the last normal geometry is used as-is (constrained to the free area).
 */
function getRestoredGeometry(windowState) {
  const metrics = getViewportMetrics();
  const area = metrics.freeArea;
  const requested = windowState.restoredGeometry || {
    x: windowState.x,
    y: windowState.y,
    width: windowState.width,
    height: windowState.height,
  };

  if (!metrics.compact) {
    return constrainWindowGeometry({ ...requested, appId: windowState.appId });
  }

  const minimums = appMinimums(windowState.appId, metrics);
  const width = Math.min(
    area.width,
    Math.max(requested.width || 0, Math.round(area.width * COMPACT_RESTORE_WIDTH_RATIO), minimums.width)
  );
  const height = Math.min(
    area.height,
    Math.max(requested.height || 0, Math.round(area.height * COMPACT_RESTORE_HEIGHT_RATIO), minimums.height)
  );
  return constrainWindowGeometry({
    x: requested.x ?? area.x,
    y: requested.y ?? area.y,
    width,
    height,
    appId: windowState.appId,
  });
}

// Geometry a saved window settles on when the desktop is loaded. Under 768px
// every window shows maximized, so the saved normal geometry becomes the
// window's restoredGeometry (kept for the wider screen) and the window itself
// takes the free area. Nothing saved is rewritten.
function getCompactLoadGeometry(windowState, geometry, appId) {
  const metrics = getViewportMetrics();
  const area = metrics.freeArea;
  const saved = windowState.restoredGeometry || geometry;
  return {
    geometry: constrainWindowGeometry({
      x: area.x,
      y: area.y,
      width: area.width,
      height: area.height,
      appId,
    }),
    restoredGeometry: constrainWindowGeometry({
      x: saved.x ?? area.x,
      y: saved.y ?? area.y,
      width: saved.width || geometry.width,
      height: saved.height || geometry.height,
      appId,
    }),
  };
}

function applyLaunchWindowGeometry(baseGeometry, appContext = {}, appId = '') {
  const preferred = appContext?.windowGeometry || null;
  if (!preferred || typeof preferred !== 'object') return baseGeometry;

  return constrainWindowGeometry({
    x: preferred.x ?? baseGeometry.x,
    y: preferred.y ?? baseGeometry.y,
    width: preferred.width ?? baseGeometry.width,
    height: preferred.height ?? baseGeometry.height,
    appId,
  });
}

function normalizeWindowGeometry(windowState) {
  const appId = normalizeAppId(windowState.appId);
  const geometry = constrainWindowGeometry({ ...windowState, appId });
  // A maximized window keeps its stored geometry untouched at load: it is
  // drawn over the free area, but its normal geometry (and restoredGeometry)
  // must survive a reload exactly as saved, so restoring after a reload lands
  // where the window left off. A smaller screen is handled when the window is
  // actually restored (restoreWindow -> getRestoredGeometry).
  const desktopGeometry = windowState.mode === 'maximized'
    ? {
        x: windowState.x,
        y: windowState.y,
        width: windowState.width,
        height: windowState.height,
      }
    : geometry;
  const restoredGeometry = windowState.restoredGeometry
    ? constrainWindowGeometry({ ...windowState.restoredGeometry, appId })
    : null;

  // Under 768px a saved window shows maximized too, and its saved geometry
  // becomes restoredGeometry instead of being rewritten.
  if (windowWidthIsCompact() && windowState.mode !== 'minimized') {
    const compactLoad = getCompactLoadGeometry(windowState, geometry, appId);
    return {
      ...windowState,
      appId,
      ...compactLoad.geometry,
      mode: 'maximized',
      restoredGeometry: compactLoad.restoredGeometry,
    };
  }

  return {
    ...windowState,
    appId,
    ...desktopGeometry,
    restoredGeometry,
  };
}

function normalizeIconPositions(positions = {}) {
  return Object.fromEntries(
    Object.entries(positions || {}).map(([appId, position]) => [normalizeAppId(appId), position])
  );
}

// ---- Default icon grid positions ----

/** Default grid positions for floating desktop icons (column layout, left side) */
export function getDefaultIconPositions() {
  const positions = {};
  const startX = 32;
  const startY = 32;
  const colWidth = 100;
  const rowHeight = 90;
  DESKTOP_ICON_APPS.forEach((app, i) => {
    positions[app.id] = { x: startX, y: startY + i * rowHeight };
  });
  return positions;
}

// ---- Stores ----

/** @type {import('svelte/store').Writable<Array>} */
export const windows = writable([]);

/** @type {import('svelte/store').Writable<string>} */
export const activeWindowId = writable('');

/** @type {import('svelte/store').Writable<number>} */
export const nextZIndex = writable(1);

/** @type {import('svelte/store').Writable<string>} */
export const liveStatus = writable('disconnected');

/** @type {import('svelte/store').Writable<Object>} */
export const iconPositions = writable(getDefaultIconPositions());

/** @type {import('svelte/store').Writable<boolean>} */
export const showDesktopMode = writable(false);

/** @type {import('svelte/store').Writable<string>} */
export const selectedIconId = writable('');

// ---- Derived stores ----

/** Open windows (shown in the prompt surface tray) */
export const openWindows = derived(windows, ($windows) =>
  $windows.filter((w) => w.mode !== 'closed' && w.mode !== 'hidden')
);

/** Visible (non-closed, non-minimized, non-hidden) windows */
export const visibleWindows = derived(windows, ($windows) =>
  $windows.filter((w) => w.mode !== 'closed' && w.mode !== 'minimized' && w.mode !== 'hidden')
);

// ---- Store actions ----

/**
 * Open an app window.
 * Most apps are single-instance per appId. Texture is multi-instance so
 * prompt/file opens create fresh windows.
 */
// A phone shows one window at a time; every launch and every switch from the
// prompt bar's app switcher lands on the maximized window.
function isCompactViewport() {
  return typeof window !== 'undefined' && window.innerWidth < COMPACT_BREAKPOINT;
}

// Whether the screen being laid out is a phone. Same rule as
// isCompactViewport(), but safe while the desktop is loaded from a saved
// state during server-side rendering (no window): the saved widths stand in.
function windowWidthIsCompact() {
  if (typeof window !== 'undefined') return isCompactViewport();
  return getViewportMetrics().viewportWidth < COMPACT_BREAKPOINT;
}

export function openApp(appId, appName, icon, appContext = {}) {
  windows.update(($windows) => {
    const canonicalAppId = normalizeAppId(appId);
    const definition = getAppDefinition(canonicalAppId);
    const singletonKey = String(appContext.singletonKey || '').trim();
    const allowMultiple = !singletonKey && (appContext.allowMultiple === true || definition?.window?.singleton === false);
    const existing = singletonKey
      ? $windows.find((w) => normalizeAppId(w.appId) === canonicalAppId && w.mode !== 'closed' && w.appContext?.singletonKey === singletonKey)
      : (!allowMultiple ? $windows.find((w) => normalizeAppId(w.appId) === canonicalAppId && w.mode !== 'closed') : null);
    if (existing) {
      // Focus existing window and apply any launch context such as a deep link.
      activeWindowId.set(existing.windowId);
      const compactExisting = isCompactViewport();
      let updated = $windows.map((w) =>
        w.windowId === existing.windowId
          ? {
              ...withoutShowDesktopMarkers(w),
              title: appContext.windowTitle || appName || w.title,
              appContext: {
                ...(w.appContext || {}),
                ...(appContext || {}),
              },
              zIndex: getNextZIndex(),
              mode: w.mode === 'minimized' ? (w._showDesktopPrevMode || 'normal') : w.mode,
              ...(compactExisting ? {
                mode: 'maximized',
                restoredGeometry: w.restoredGeometry || { x: w.x, y: w.y, width: w.width, height: w.height },
              } : {}),
            }
          : w
      );
      // If it was minimized, restore its geometry
      if (existing.mode === 'minimized' && existing.restoredGeometry) {
        const geo = constrainWindowGeometry({ ...existing.restoredGeometry, appId: canonicalAppId });
        updated = updated.map((w) =>
          w.windowId === existing.windowId
            ? { ...w, x: geo.x, y: geo.y, width: geo.width, height: geo.height, restoredGeometry: null }
            : w
        );
      }
      showDesktopMode.set(false);
      return updated.map(withoutShowDesktopMarkers);
    }

    const windowId = generateWindowId();
    const openCount = $windows.filter((w) => w.mode !== 'closed').length;
    const compact = isCompactViewport();
    const geometry = applyLaunchWindowGeometry(getNewWindowGeometry(openCount, canonicalAppId), appContext, canonicalAppId);
    // A phone has one window on screen at a time; every window opens maximized
    // and the prompt bar's app switcher is how one moves between them.
    const preferredMode = appContext.windowMode === 'maximized' || compact ? 'maximized' : 'normal';
    const newWindow = {
      windowId,
      appId: canonicalAppId,
      title: appContext.windowTitle || appName || canonicalAppId,
      icon: icon || getAppIcon(canonicalAppId),
      x: geometry.x,
      y: geometry.y,
      width: geometry.width,
      height: geometry.height,
      mode: preferredMode,
      zIndex: getNextZIndex(),
      restoredGeometry: null,
      appContext: { ...appContext },
    };

    activeWindowId.set(windowId);
    return [...$windows, newWindow];
  });
}

/** Close a window by ID */
export function closeWindow(windowId) {
  windows.update(($windows) => {
    const remaining = $windows.filter((w) => w.windowId !== windowId);
    // Update active window
    activeWindowId.update(($activeId) => {
      if ($activeId === windowId) {
        const visible = remaining.filter((w) => w.mode !== 'closed');
        if (visible.length > 0) {
          return visible.reduce((a, b) => (a.zIndex > b.zIndex ? a : b)).windowId;
        }
        return '';
      }
      return $activeId;
    });
    return remaining;
  });
}

/** Focus a window (bring to top z-index) */
export function focusWindow(windowId) {
  activeWindowId.set(windowId);
  windows.update(($windows) => {
    const target = $windows.find((w) => w.windowId === windowId);
    const updated = $windows.map((w) => {
      if (w.windowId !== windowId) return w;
      return {
        ...withoutShowDesktopMarkers(w),
        mode: w.mode === 'minimized' ? (w._showDesktopPrevMode || 'normal') : w.mode,
        restoreSuspended: false,
        zIndex: getNextZIndex(),
      };
    });
    if (target?._showDesktopMinimized) {
      showDesktopMode.set(false);
      return updated.map(withoutShowDesktopMarkers);
    }
    return updated;
  });
}

/** Minimize a window */
export function minimizeWindow(windowId) {
  windows.update(($windows) => {
    const updated = $windows.map((w) =>
      w.windowId === windowId ? { ...w, mode: 'minimized' } : w
    );
    // Transfer focus to next visible window
    activeWindowId.update(($activeId) => {
      if ($activeId === windowId) {
        const visible = updated.filter((w) => w.mode === 'normal' || w.mode === 'maximized');
        if (visible.length > 0) {
          return visible.reduce((a, b) => (a.zIndex > b.zIndex ? a : b)).windowId;
        }
        return '';
      }
      return $activeId;
    });
    return updated;
  });
}

/** Maximize a window */
export function maximizeWindow(windowId) {
  windows.update(($windows) =>
    $windows.map((w) => {
      if (w.windowId === windowId) {
        return {
          ...w,
          mode: 'maximized',
          restoredGeometry: w.restoredGeometry || { x: w.x, y: w.y, width: w.width, height: w.height },
        };
      }
      return w;
    })
  );
  activeWindowId.set(windowId);
}

/** Restore a window from minimized or maximized */
export function restoreWindow(windowId) {
  const raisedZIndex = getNextZIndex();
  windows.update(($windows) => {
    const target = $windows.find((w) => w.windowId === windowId);
    const restored = $windows.map((w) => {
      if (w.windowId === windowId) {
        if (w.mode === 'minimized' && w.restoredGeometry) {
          const geo = constrainWindowGeometry({ ...w.restoredGeometry, appId: w.appId });
          return {
            ...withoutShowDesktopMarkers(w),
            mode: w._showDesktopPrevMode || 'normal',
            x: geo.x,
            y: geo.y,
            width: geo.width,
            height: geo.height,
            zIndex: raisedZIndex,
            restoredGeometry: null,
            restoreSuspended: false,
          };
        }
        if (w.mode === 'maximized' && w.restoredGeometry) {
          // On a phone the restored window is at least 92% wide and 85% of the
          // free height; on desktop it is the geometry it had before maximizing.
          const geo = getRestoredGeometry(w);
          return {
            ...withoutShowDesktopMarkers(w),
            mode: 'normal',
            x: geo.x,
            y: geo.y,
            width: geo.width,
            height: geo.height,
            zIndex: raisedZIndex,
            restoredGeometry: null,
            restoreSuspended: false,
          };
        }
        return {
          ...withoutShowDesktopMarkers(w),
          mode: w._showDesktopPrevMode || 'normal',
          zIndex: raisedZIndex,
          restoredGeometry: null,
          restoreSuspended: false,
        };
      }
      return w;
    });
    if (target?._showDesktopMinimized) {
      showDesktopMode.set(false);
      return restored.map(withoutShowDesktopMarkers);
    }
    return restored;
  });
  activeWindowId.set(windowId);
}

/** Move a window */
export function moveWindow(windowId, x, y) {
  windows.update(($windows) =>
    $windows.map((w) => {
      if (w.windowId !== windowId) return w;
      const geometry = constrainWindowGeometry({ x, y, width: w.width, height: w.height, appId: w.appId });
      return { ...w, ...geometry };
    })
  );
}

/** Resize a window */
export function resizeWindow(windowId, x, y, width, height, anchor = 'nw') {
  windows.update(($windows) =>
    $windows.map((w) => {
      if (w.windowId !== windowId) return w;
      const geometry = constrainWindowGeometry({ x, y, width, height, appId: w.appId, resizing: true, anchor });
      return { ...w, ...geometry };
    })
  );
}

/** Update durable per-window app context after an app creates or opens state. */
export function updateWindowAppContext(windowId, appContext = {}, title = '') {
  windows.update(($windows) =>
    $windows.map((w) => {
      if (w.windowId !== windowId) return w;
      const nextContext = {
        ...(w.appContext || {}),
        ...(appContext || {}),
      };
      return {
        ...w,
        title: title || nextContext.windowTitle || w.title,
        appContext: nextContext,
      };
    })
  );
}

export function clearWindowRestoreSuspension(windowId) {
  windows.update(($windows) =>
    $windows.map((w) =>
      w.windowId === windowId ? { ...w, restoreSuspended: false } : w
    )
  );
}

export function suspendBackgroundHeavyWindows(activeId = '') {
  let suspended = 0;
  windows.update(($windows) => {
    const topActiveId = activeId || $windows
      .filter((w) => w.mode !== 'closed' && w.mode !== 'hidden' && w.mode !== 'minimized')
      .sort((a, b) => (b.zIndex || 0) - (a.zIndex || 0))[0]?.windowId || '';
    return $windows.map((w) => {
      if (
        w.windowId !== topActiveId &&
        isHeavyAppId(w.appId) &&
        w.mode !== 'closed' &&
        w.mode !== 'hidden' &&
        w.mode !== 'minimized' &&
        !w.restoreSuspended
      ) {
        suspended += 1;
        return { ...w, restoreSuspended: true };
      }
      return w;
    });
  });
  return suspended;
}

export function suspendWindowBody(windowId) {
  let suspended = false;
  windows.update(($windows) =>
    $windows.map((w) => {
      if (
        w.windowId === windowId &&
        isHeavyAppId(w.appId) &&
        w.mode !== 'closed' &&
        w.mode !== 'hidden' &&
        !w.restoreSuspended
      ) {
        suspended = true;
        return { ...w, restoreSuspended: true };
      }
      return w;
    })
  );
  return suspended;
}

/** Set windows state (used for loading from server) */
export function setWindows(newWindows, newActiveId) {
  const normalizedWindows = normalizeWindowStackOrder(
    newWindows.map((windowState) => normalizeWindowGeometry(windowState))
  );
  windows.set(normalizedWindows);
  activeWindowId.set(newActiveId || '');
  if (normalizedWindows.length > 0) {
    const maxZ = Math.max(...normalizedWindows.map((w) => w.zIndex || 1));
    nextZIndex.set(maxZ + 1);
  }
}

// ---- Icon position actions ----

/** Move a desktop icon to a new position */
export function moveIcon(appId, x, y) {
  const canonicalAppId = normalizeAppId(appId);
  iconPositions.update((positions) => ({
    ...positions,
    [canonicalAppId]: { x, y },
  }));
}

/** Set icon positions (used for loading from server) */
export function setIconPositions(positions) {
  if (positions && Object.keys(positions).length > 0) {
    iconPositions.set(normalizeIconPositions(positions));
  } else {
    iconPositions.set(getDefaultIconPositions());
  }
}

// ---- Show Desktop actions ----

/** Toggle show desktop mode (minimize/restore all windows) */
export function toggleShowDesktop() {
  let currentShowDesktop;
  showDesktopMode.subscribe((v) => { currentShowDesktop = v; })();

  if (currentShowDesktop) {
    // Restore all windows that were minimized by show desktop
    windows.update(($windows) =>
      $windows.map((w) => {
        if (w._showDesktopMinimized) {
          const { _showDesktopMinimized, _showDesktopPrevMode, ...rest } = w;
          return { ...rest, mode: _showDesktopPrevMode || 'normal' };
        }
        return w;
      })
    );
    showDesktopMode.set(false);
  } else {
    // Minimize all visible windows and remember their previous mode
    windows.update(($windows) =>
      $windows.map((w) => {
        if (w.mode !== 'closed' && w.mode !== 'hidden' && w.mode !== 'minimized') {
          return { ...w, _showDesktopMinimized: true, _showDesktopPrevMode: w.mode, mode: 'minimized' };
        }
        return w;
      })
    );
    showDesktopMode.set(true);
  }
}

// ---- Internal helpers ----

// Show-desktop markers are transient. Focusing, restoring, re-opening, or a
// plain restore drops them, so an ordinary restore never resurrects a stale
// previous mode. This is the same removal toggleShowDesktop does inline.
function withoutShowDesktopMarkers(windowState) {
  if (!windowState) return windowState;
  if (!windowState._showDesktopMinimized && !windowState._showDesktopPrevMode) return windowState;
  const { _showDesktopMinimized, _showDesktopPrevMode, ...rest } = windowState;
  return rest;
}

function getNextZIndex() {
  let current;
  const unsubscribe = nextZIndex.subscribe((value) => {
    current = value;
  });
  unsubscribe();
  if (current > WINDOW_Z_INDEX_COMPACT_AT) {
    compactWindowStackOrder();
  }

  let next;
  nextZIndex.update((n) => {
    next = n;
    return n + 1;
  });
  return next;
}

function isStackedWindow(windowState) {
  return windowState?.mode !== 'closed' && windowState?.mode !== 'hidden';
}

function normalizeWindowStackOrder(windowStates = []) {
  const zById = new Map();
  [...windowStates]
    .filter(isStackedWindow)
    .sort((a, b) => {
      const zDiff = (a.zIndex || 0) - (b.zIndex || 0);
      if (zDiff !== 0) return zDiff;
      return String(a.windowId || '').localeCompare(String(b.windowId || ''));
    })
    .forEach((windowState, index) => {
      if (windowState.windowId) {
        zById.set(windowState.windowId, index + 1);
      }
    });

  return windowStates.map((windowState) =>
    zById.has(windowState.windowId)
      ? { ...windowState, zIndex: zById.get(windowState.windowId) }
      : windowState
  );
}

function compactWindowStackOrder() {
  let maxZ = 0;
  windows.update(($windows) => {
    const compacted = normalizeWindowStackOrder($windows);
    maxZ = compacted.reduce((max, windowState) => (
      isStackedWindow(windowState) ? Math.max(max, windowState.zIndex || 0) : max
    ), 0);
    return compacted;
  });
  nextZIndex.set(maxZ + 1);
}
