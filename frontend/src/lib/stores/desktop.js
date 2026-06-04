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
 *   - selectedIconId: currently selected desktop icon (single-click)
 *
 * Each window object has:
 *   { windowId, appId, title, icon, x, y, width, height, mode, zIndex, restoredGeometry, appContext }
 *
 * App registry defines the hardcoded apps with their icons.
 */

import { writable, derived } from 'svelte/store';
import {
  APP_REGISTRY,
  DESK_APPS,
  DESKTOP_ICON_APPS,
  HEAVY_APP_IDS,
  getAppDefinition,
  getAppIcon,
  getAppWindowPreference,
  isHeavyAppId,
} from '../apps/registry';

// ---- App registry ----

export { APP_REGISTRY, DESK_APPS, DESKTOP_ICON_APPS, HEAVY_APP_IDS, getAppDefinition, getAppIcon, getAppWindowPreference, isHeavyAppId };

// ---- Window counter ----

let windowCounter = 0;

const MIN_WINDOW_WIDTH = 200;
const MIN_WINDOW_HEIGHT = 120;
const PROMPT_SURFACE_SIZE = 64;
const COMPACT_BREAKPOINT = 768;
const DEFAULT_VIEWPORT_WIDTH = 1280;
const DEFAULT_VIEWPORT_HEIGHT = 800;
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
  const margin = compact ? 10 : 24;
  const workspaceStartY = margin + promptSurfaceOffsets.top;
  const preferredWorkspaceStartX = compact ? margin + 8 : margin + 124;
  const workspaceStartX = Math.min(
    preferredWorkspaceStartX,
    Math.max(margin, viewportWidth - MIN_WINDOW_WIDTH - margin)
  );
  const workspaceWidth = Math.max(MIN_WINDOW_WIDTH, viewportWidth - workspaceStartX - margin);
  const maxWidth = Math.max(MIN_WINDOW_WIDTH, viewportWidth - margin * 2);
  const maxHeight = Math.max(
    MIN_WINDOW_HEIGHT,
    viewportHeight - promptSurfaceOffsets.top - promptSurfaceOffsets.bottom - margin * 2
  );
  const compactWindowWidth = Math.max(
    MIN_WINDOW_WIDTH,
    Math.min(Math.round(viewportWidth * 0.88), workspaceWidth)
  );
  const compactWindowHeight = Math.max(
    MIN_WINDOW_HEIGHT,
    Math.min(Math.round(maxHeight * 0.78), maxHeight)
  );
  const baseWidth = Math.min(compact ? compactWindowWidth : 650, workspaceWidth);
  const baseHeight = Math.min(compact ? compactWindowHeight : 450, maxHeight);
  return {
    compact,
    margin,
    viewportWidth,
    viewportHeight,
    promptSurfaceOffsets,
    workspaceStartX,
    workspaceStartY,
    workspaceWidth,
    maxWidth,
    maxHeight,
    baseWidth,
    baseHeight,
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

function constrainWindowGeometry({ x, y, width, height, appId = '' }) {
  const metrics = getViewportMetrics();
  const minimums = appMinimums(appId, metrics);
  const clampedWidth = clamp(width, Math.min(minimums.width, metrics.maxWidth), metrics.maxWidth);
  const clampedHeight = clamp(height, Math.min(minimums.height, metrics.maxHeight), metrics.maxHeight);
  const maxX = Math.max(metrics.margin, metrics.viewportWidth - clampedWidth - metrics.margin);
  const maxY = Math.max(
    metrics.workspaceStartY,
    metrics.viewportHeight - metrics.promptSurfaceOffsets.bottom - clampedHeight - metrics.margin
  );

  return {
    x: Math.round(clamp(x, metrics.margin, maxX)),
    y: Math.round(clamp(y, metrics.workspaceStartY, maxY)),
    width: Math.round(clampedWidth),
    height: Math.round(clampedHeight),
  };
}

function getNewWindowGeometry(openCount, appId = '') {
  const metrics = getViewportMetrics();
  const offsetStep = metrics.compact ? 16 : 30;
  const offset = (openCount % 6) * offsetStep;
  const preference = getAppWindowPreference(appId);
  const desktopPref = preference.desktop || {};

  if (metrics.compact) {
    const compactPref = preference.compact || {};
    return constrainWindowGeometry({
      x: metrics.workspaceStartX + offset,
      y: metrics.workspaceStartY + offset,
      width: Math.min(compactPref.width || metrics.baseWidth, metrics.maxWidth),
      height: Math.min(compactPref.height || metrics.baseHeight, metrics.maxHeight),
      appId,
    });
  }

  return constrainWindowGeometry({
    x: metrics.workspaceStartX + offset,
    y: metrics.workspaceStartY + offset,
    width: Math.min(desktopPref.width || metrics.baseWidth, metrics.workspaceWidth),
    height: desktopPref.height || metrics.baseHeight,
    appId,
  });
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
  const geometry = constrainWindowGeometry({ ...windowState, appId: windowState.appId });
  const restoredGeometry = windowState.restoredGeometry
    ? constrainWindowGeometry({ ...windowState.restoredGeometry, appId: windowState.appId })
    : null;

  return {
    ...windowState,
    ...geometry,
    restoredGeometry,
  };
}

function withoutShowDesktopMarkers(windowState) {
  const { _showDesktopMinimized, _showDesktopPrevMode, ...rest } = windowState;
  return rest;
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

/** Minimized windows (shown in the prompt surface tray) */
export const minimizedWindows = derived(windows, ($windows) =>
  $windows.filter((w) => w.mode === 'minimized')
);

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
 * Most apps are single-instance per appId. VText is multi-instance so
 * prompt/file opens create fresh windows.
 */
export function openApp(appId, appName, icon, appContext = {}) {
  windows.update(($windows) => {
    const definition = getAppDefinition(appId);
    const allowMultiple = appContext.allowMultiple === true || definition?.window?.singleton === false;
    const existing = !allowMultiple ? $windows.find((w) => w.appId === appId && w.mode !== 'closed') : null;
    if (existing) {
      // Focus existing window and apply any launch context such as a deep link.
      activeWindowId.set(existing.windowId);
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
            }
          : w
      );
      // If it was minimized, restore its geometry
      if (existing.mode === 'minimized' && existing.restoredGeometry) {
        const geo = constrainWindowGeometry({ ...existing.restoredGeometry, appId });
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
    const geometry = applyLaunchWindowGeometry(getNewWindowGeometry(openCount, appId), appContext, appId);
    const preferredMode = appContext.windowMode === 'maximized' ? 'maximized' : 'normal';
    const newWindow = {
      windowId,
      appId,
      title: appContext.windowTitle || appName || appId,
      icon: icon || getAppIcon(appId),
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
          restoredGeometry: { x: w.x, y: w.y, width: w.width, height: w.height },
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
          const geo = constrainWindowGeometry({ ...w.restoredGeometry, appId: w.appId });
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
export function resizeWindow(windowId, x, y, width, height) {
  windows.update(($windows) =>
    $windows.map((w) => {
      if (w.windowId !== windowId) return w;
      const geometry = constrainWindowGeometry({ x, y, width, height, appId: w.appId });
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
  iconPositions.update((positions) => ({
    ...positions,
    [appId]: { x, y },
  }));
}

/** Set icon positions (used for loading from server) */
export function setIconPositions(positions) {
  if (positions && Object.keys(positions).length > 0) {
    iconPositions.set(positions);
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
