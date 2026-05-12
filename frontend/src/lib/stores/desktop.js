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

// ---- App registry ----

export const APP_REGISTRY = [
  { id: 'files', name: 'Files', icon: '📁', description: 'File Browser', singleton: true },
  { id: 'browser', name: 'Browser', icon: '🌐', description: 'Web Browser', singleton: true },
  { id: 'terminal', name: 'Terminal', icon: '💻', description: 'Terminal', singleton: true },
  { id: 'settings', name: 'Settings', icon: '⚙️', description: 'Desktop settings', singleton: true, window: { desktop: { width: 940, height: 720 } } },
  { id: 'pdf', name: 'PDF', icon: '📄', description: 'PDF reader', singleton: false, window: { desktop: { width: 940, height: 720 }, compact: { fullBleed: true, minWidth: 280, minHeight: 420 } } },
  { id: 'epub', name: 'EPUB', icon: '📚', description: 'EPUB reader', singleton: false, window: { desktop: { width: 900, height: 700 }, compact: { fullBleed: true, minWidth: 280, minHeight: 420 } } },
  { id: 'image', name: 'Image', icon: '🖼️', description: 'Image viewer', singleton: false, window: { desktop: { width: 900, height: 680 }, compact: { fullBleed: true, minWidth: 280, minHeight: 420 } } },
  { id: 'video', name: 'Video', icon: '🎬', description: 'Video and YouTube player', singleton: false, window: { desktop: { width: 980, height: 720 }, compact: { fullBleed: true, minWidth: 280, minHeight: 420 } } },
  { id: 'audio', name: 'Audio', icon: '🎧', description: 'Audio player', singleton: false, window: { desktop: { width: 760, height: 420 }, compact: { fullBleed: true, minWidth: 280, minHeight: 320 } } },
  { id: 'podcast', name: 'Podcast', icon: '📡', description: 'Podcast feed player', singleton: false, window: { desktop: { width: 900, height: 660 }, compact: { fullBleed: true, minWidth: 280, minHeight: 420 } } },
  {
    id: 'vtext',
    name: 'VText',
    icon: '📝',
    description: 'Versioned document editor',
    singleton: false,
    window: {
      desktop: { width: 960, height: 720, minWidth: 680, minHeight: 520 },
      compact: { fullBleed: true, minWidth: 280, minHeight: 420 },
    },
  },
  { id: 'trace', name: 'Trace', icon: '🔎', description: 'Multiagent trace viewer', singleton: true, window: { desktop: { width: 1040, height: 680 } } },
];

/** The main apps shown as floating desktop icons */
export const DESKTOP_ICON_APPS = APP_REGISTRY.filter((app) =>
  ['files', 'browser', 'terminal', 'settings', 'vtext', 'trace'].includes(app.id)
);

// ---- Window counter ----

let windowCounter = 0;

const MIN_WINDOW_WIDTH = 200;
const MIN_WINDOW_HEIGHT = 120;
const BOTTOM_BAR_HEIGHT = 56;
const COMPACT_BREAKPOINT = 768;
const DEFAULT_VIEWPORT_WIDTH = 1280;
const DEFAULT_VIEWPORT_HEIGHT = 800;
export function getAppDefinition(appId) {
  return APP_REGISTRY.find((app) => app.id === appId) || null;
}

export function getAppIcon(appId) {
  return getAppDefinition(appId)?.icon || '📱';
}

export function getAppWindowPreference(appId) {
  return getAppDefinition(appId)?.window || {};
}

function generateWindowId() {
  windowCounter++;
  return `win-${Date.now()}-${windowCounter}`;
}

function clamp(value, min, max) {
  return Math.min(Math.max(value, min), max);
}

function getViewportMetrics() {
  const viewportWidth = typeof window !== 'undefined' ? window.innerWidth : DEFAULT_VIEWPORT_WIDTH;
  const viewportHeight = typeof window !== 'undefined' ? window.innerHeight : DEFAULT_VIEWPORT_HEIGHT;
  const compact = viewportWidth < COMPACT_BREAKPOINT;
  const margin = compact ? 12 : 24;
  const preferredWorkspaceStartX = margin + (compact ? 96 : 124);
  const workspaceStartX = Math.min(
    preferredWorkspaceStartX,
    Math.max(margin, viewportWidth - MIN_WINDOW_WIDTH - margin)
  );
  const workspaceWidth = Math.max(MIN_WINDOW_WIDTH, viewportWidth - workspaceStartX - margin);
  const maxWidth = Math.max(MIN_WINDOW_WIDTH, viewportWidth - margin * 2);
  const maxHeight = Math.max(
    MIN_WINDOW_HEIGHT,
    viewportHeight - BOTTOM_BAR_HEIGHT - margin * 2
  );
  const compactWindowWidth = Math.max(
    MIN_WINDOW_WIDTH,
    Math.min(320, workspaceWidth - 36)
  );
  const baseWidth = Math.min(compact ? compactWindowWidth : 650, workspaceWidth);
  const baseHeight = Math.min(compact ? 420 : 450, maxHeight);
  return {
    compact,
    margin,
    viewportWidth,
    viewportHeight,
    workspaceStartX,
    workspaceWidth,
    maxWidth,
    maxHeight,
    baseWidth,
    baseHeight,
  };
}

function appMinimums(appId, metrics) {
  const pref = getAppWindowPreference(appId);
  if (metrics.compact && pref.compact?.fullBleed) {
    return {
      width: Math.min(metrics.maxWidth, Math.max(pref.compact.minWidth || MIN_WINDOW_WIDTH, metrics.viewportWidth - metrics.margin * 2)),
      height: Math.min(metrics.maxHeight, Math.max(pref.compact.minHeight || MIN_WINDOW_HEIGHT, metrics.maxHeight)),
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
    metrics.margin,
    metrics.viewportHeight - BOTTOM_BAR_HEIGHT - clampedHeight - metrics.margin
  );

  return {
    x: clamp(x, metrics.margin, maxX),
    y: clamp(y, metrics.margin, maxY),
    width: clampedWidth,
    height: clampedHeight,
  };
}

function getNewWindowGeometry(openCount, appId = '') {
  const metrics = getViewportMetrics();
  const offsetStep = metrics.compact ? 18 : 30;
  const offset = (openCount % 6) * offsetStep;
  const preference = getAppWindowPreference(appId);
  const desktopPref = preference.desktop || {};

  if (metrics.compact && preference.compact?.fullBleed) {
    return constrainWindowGeometry({
      x: metrics.margin,
      y: metrics.margin,
      width: metrics.maxWidth,
      height: metrics.maxHeight,
      appId,
    });
  }

  return constrainWindowGeometry({
    x: metrics.workspaceStartX + offset,
    y: metrics.margin + offset,
    width: Math.min(desktopPref.width || metrics.baseWidth, metrics.workspaceWidth),
    height: desktopPref.height || metrics.baseHeight,
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

/** Minimized windows (shown in bottom bar) */
export const minimizedWindows = derived(windows, ($windows) =>
  $windows.filter((w) => w.mode === 'minimized')
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
    const allowMultiple = appContext.allowMultiple === true || definition?.singleton === false;
    const existing = !allowMultiple ? $windows.find((w) => w.appId === appId && w.mode !== 'closed') : null;
    if (existing) {
      // Focus existing window
      activeWindowId.set(existing.windowId);
      let updated = $windows.map((w) =>
        w.windowId === existing.windowId
          ? { ...w, zIndex: getNextZIndex(), mode: w.mode === 'minimized' ? 'normal' : w.mode }
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
      return updated;
    }

    const windowId = generateWindowId();
    const openCount = $windows.filter((w) => w.mode !== 'closed').length;
    const geometry = getNewWindowGeometry(openCount, appId);
    const newWindow = {
      windowId,
      appId,
      title: appContext.windowTitle || appName || appId,
      icon: icon || getAppIcon(appId),
      x: geometry.x,
      y: geometry.y,
      width: geometry.width,
      height: geometry.height,
      mode: 'normal',
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
  windows.update(($windows) =>
    $windows.map((w) =>
      w.windowId === windowId ? { ...w, zIndex: getNextZIndex() } : w
    )
  );
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
  windows.update(($windows) =>
    $windows.map((w) => {
      if (w.windowId === windowId) {
        if (w.mode === 'minimized' && w.restoredGeometry) {
          const geo = constrainWindowGeometry({ ...w.restoredGeometry, appId: w.appId });
          return {
            ...w,
            mode: 'normal',
            x: geo.x,
            y: geo.y,
            width: geo.width,
            height: geo.height,
            restoredGeometry: null,
          };
        }
        if (w.mode === 'maximized' && w.restoredGeometry) {
          const geo = constrainWindowGeometry({ ...w.restoredGeometry, appId: w.appId });
          return {
            ...w,
            mode: 'normal',
            x: geo.x,
            y: geo.y,
            width: geo.width,
            height: geo.height,
            restoredGeometry: null,
          };
        }
        return { ...w, mode: 'normal', restoredGeometry: null };
      }
      return w;
    })
  );
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

/** Set windows state (used for loading from server) */
export function setWindows(newWindows, newActiveId) {
  windows.set(newWindows.map((windowState) => normalizeWindowGeometry(windowState)));
  activeWindowId.set(newActiveId || '');
  if (newWindows.length > 0) {
    const maxZ = Math.max(...newWindows.map((w) => w.zIndex || 1));
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
  let next;
  nextZIndex.update((n) => {
    next = n;
    return n + 1;
  });
  return next;
}
