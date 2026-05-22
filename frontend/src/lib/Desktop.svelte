<!--
  Desktop — ChoirOS desktop shell with floating desktop icons, floating windows, and bottom bar.

  Layout:
    - Floating desktop icons freely draggable on the desktop surface
    - Floating windows draggable/resizable on top of icons
    - Bottom bar fixed at viewport bottom

  Responsive layout across three breakpoints:
    - Desktop (>1024px): full floating icons with labels, floating draggable windows
    - Tablet (768-1024px): floating icons, windows with max-width constraint
    - Mobile (<768px): same floating desktop/window model with tighter geometry

  Data attributes for test targeting:
    data-desktop             — root desktop container
    data-desktop-windows     — window container area
    data-desktop-surface     — desktop surface with floating icons
    data-shell               — backward compat with existing tests
-->
<script>
  import { createEventDispatcher } from 'svelte';
  import { onMount } from 'svelte';
  import { onDestroy } from 'svelte';
  import { tick } from 'svelte';
  import { fetchWithRenewal, AuthRequiredError, renewSession } from './auth.js';
  import { submitConductorPrompt, waitForConductorDecision } from './conductor.js';
  import { fetchDesktopState, saveDesktopState } from './desktop.js';
  import { withDesktopSelector } from './desktop-selector.js';
  import {
    currentSessionId,
    dispatchLiveEvent,
    isDrivingSession,
    isOwnLiveEvent,
    liveEventKind,
    liveEventPayload,
    observeRemoteDriverSession,
    renewDriverLease,
  } from './live-events.js';
  import FloatingDesktopIcons from './FloatingDesktopIcons.svelte';
  import BottomBar from './BottomBar.svelte';
  import FloatingWindow from './FloatingWindow.svelte';
  import DesktopOverview from './DesktopOverview.svelte';
  import TraceApp from './TraceApp.svelte';
  import VTextEditor from './VTextEditor.svelte';
  import SettingsApp from './SettingsApp.svelte';
  import { openFileDocument } from './vtext.js';
  import FileBrowser from './FileBrowser.svelte';
  import BrowserApp from './BrowserApp.svelte';
  import AppsChangesApp from './AppsChangesApp.svelte';
  import TerminalApp from './TerminalApp.svelte';
  import ComputeMonitorApp from './ComputeMonitorApp.svelte';
  import PodcastApp from './PodcastApp.svelte';
  import ImageApp from './ImageApp.svelte';
  import AudioApp from './AudioApp.svelte';
  import VideoApp from './VideoApp.svelte';
  import PdfApp from './PdfApp.svelte';
  import EpubApp from './EpubApp.svelte';
  import {
    windows,
    activeWindowId,
    liveStatus,
    iconPositions,
    showDesktopMode,
    selectedIconId,
    openApp,
    closeWindow,
    focusWindow,
    minimizeWindow,
    maximizeWindow,
    restoreWindow,
    moveWindow,
    resizeWindow,
    clearWindowRestoreSuspension,
    suspendWindowBody,
    suspendBackgroundHeavyWindows,
    updateWindowAppContext,
    setWindows,
    setIconPositions,
    getDefaultIconPositions,
    getAppIcon,
    isHeavyAppId,
  } from './stores/desktop.js';
  import {
    createOverviewPreviewDecisions,
    getOverviewPreviewDecision,
  } from './desktop-overview-preview.js';

  export let currentUser = null;
  export let authenticated = false;
  export let promptReplay = null;
  export let appReplay = null;
  export let publicRoutePath = '';

  const dispatch = createEventDispatcher();

  // ---- Bootstrap data (preserved for session renewal, not displayed) ----
  let bootstrapData = null;
  let bootstrapError = '';
  let bootstrapStable = false;
  let desktopReady = false;
  let promptPlaceholder = 'Connecting to desktop...';
  let promptStatus = '';
  let mounted = false;
  let authenticatedStartupRunning = false;
  let lastAuthenticated = null;
  let lastPromptReplayId = null;
  let lastAppReplayId = null;
  let lastOpenedPublicRoutePath = '';

  // ---- WebSocket state ----
  let ws = null;
  let wsClosedByLogout = false;
  let wsReconnectAttempt = 0;
  let wsReconnecting = false;
  let lastLiveStreamSeq = 0;
  const MAX_WS_RECONNECT_ATTEMPTS = 5;
  const WS_RECONNECT_BASE_DELAY = 1000;
  let toasts = [];
  let toastCounter = 0;

  // ---- Desktop state persistence ----
  let stateLoaded = false;
  let applyingPersistedDesktopState = false;
  let saveTimer = null;
  const SAVE_DEBOUNCE_MS = 500;
  const BOOTSTRAP_STABILITY_DEADLINE_MS = 300_000;
  const BOOTSTRAP_STABILITY_DELAY_MS = 1_000;
  const BOOTSTRAP_PROBE_TIMEOUT_MS = 15_000;
  const MAX_BOOT_LINES = 9;
  let bootPromptPlaceholder = 'Booting user computer...';
  let bootLines = [];
  let bootLineCounter = 0;
  let bootStartedAt = 0;
  let restoreRecovery = null;
  let restoreRecoveryWindows = [];
  let restoreRecoveryActiveId = '';
  let restoreRecoverySaving = false;
  let restoreRecoveryStatus = '';
  let desktopOverviewOpen = false;
  let overviewViewportWidth = 1280;
  let overviewViewportHeight = 800;
  let overviewBottomBarHeight = 56;

  const RESTORE_RECOVERY_COMPACT_BREAKPOINT = 768;
  const RESTORE_RECOVERY_WINDOW_LIMIT = 12;
  const RESTORE_RECOVERY_HEAVY_WINDOW_LIMIT = 8;
  const OVERVIEW_STAGE_TOP_MOBILE = 76;
  const OVERVIEW_STAGE_TOP_DESKTOP = 96;
  const OVERVIEW_STAGE_BOTTOM_RAIL_MOBILE = 190;
  const OVERVIEW_STAGE_BOTTOM_RAIL_DESKTOP = 196;
  $: desktopReady = bootstrapStable && stateLoaded;
  $: promptPlaceholder = desktopReady ? 'Ask anything...' : bootPromptPlaceholder;
  $: if (mounted && authenticated !== lastAuthenticated) {
    const wasAuthenticated = lastAuthenticated === true;
    lastAuthenticated = authenticated;
    if (authenticated) {
      startAuthenticatedDesktop();
    } else {
      if (wasAuthenticated) {
        void flushDesktopState({ keepalive: true, allowSignedOutTransition: true });
      }
      enterPublicDesktop();
    }
  }
  $: if (
    mounted &&
    authenticated &&
    desktopReady &&
    promptReplay?.id &&
    promptReplay.id !== lastPromptReplayId
  ) {
    lastPromptReplayId = promptReplay.id;
    submitPromptText(promptReplay.text);
  }
  $: if (mounted && desktopReady && publicRoutePath && publicRoutePath !== lastOpenedPublicRoutePath) {
    lastOpenedPublicRoutePath = publicRoutePath;
    openPublishedVText(publicRoutePath, !authenticated);
  }
  $: if (
    mounted &&
    authenticated &&
    desktopReady &&
    appReplay?.id &&
    appReplay.id !== lastAppReplayId
  ) {
    lastAppReplayId = appReplay.id;
    openApp(appReplay.appId, appReplay.appName, appReplay.icon, appReplay.appContext || {});
  }
  $: overviewPreviewDecisions = desktopOverviewOpen
    ? createOverviewPreviewDecisions($windows, $activeWindowId, {
        viewportWidth: overviewViewportWidth,
      })
    : {};

  // ---- Desktop state persistence ----

  function closeLiveChannel() {
    wsClosedByLogout = true;
    if (ws) {
      ws.close();
      ws = null;
    }
    wsReconnectAttempt = 0;
    wsReconnecting = false;
    liveStatus.set('disconnected');
  }

  function enterPublicDesktop() {
    closeLiveChannel();
    bootstrapData = null;
    bootstrapError = '';
    bootstrapStable = true;
    stateLoaded = true;
    desktopReady = true;
    bootLines = [];
    bootPromptPlaceholder = 'Booting user computer...';
    promptPlaceholder = 'Ask anything...';
    promptStatus = '';
    authenticatedStartupRunning = false;
    restoreRecovery = null;
    restoreRecoveryWindows = [];
    restoreRecoveryActiveId = '';
    restoreRecoveryStatus = '';
    if (saveTimer) {
      clearTimeout(saveTimer);
      saveTimer = null;
    }
    setWindows([], '');
    setIconPositions(getDefaultIconPositions());
  }

  async function startAuthenticatedDesktop() {
    if (authenticatedStartupRunning) return;
    authenticatedStartupRunning = true;
    bootstrapStable = false;
    stateLoaded = false;
    desktopReady = false;
    bootLines = [];
    bootStartedAt = Date.now();
    bootPromptPlaceholder = 'Booting user computer...';
    bootstrapError = '';
    promptStatus = '';
    wsClosedByLogout = false;
    restoreRecovery = null;
    restoreRecoveryWindows = [];
    restoreRecoveryActiveId = '';
    restoreRecoveryStatus = '';
    liveStatus.set('connecting');
    appendBootLine('Powering user computer');

    try {
      const stable = await stabilizeBootstrap();
      if (!authenticated) return;
      if (!stable) {
        appendBootLine('Desktop route did not become ready in time', 'error');
        liveStatus.set('error');
        return;
      }
      appendBootLine('Opening live channel');
      connectLiveChannel();
      appendBootLine('Restoring desktop state');
      await loadDesktopState();
      desktopReady = bootstrapStable && stateLoaded;
      if (desktopReady) {
        promptStatus = '';
      }
    } catch (err) {
      if (err instanceof AuthRequiredError) {
        dispatch('authexpired');
        return;
      }
      bootstrapError = 'Bootstrap request failed';
      appendBootLine('Bootstrap request failed', 'error');
      liveStatus.set('error');
    } finally {
      authenticatedStartupRunning = false;
    }
  }

  function applyPersistedDesktopState(fn) {
    applyingPersistedDesktopState = true;
    try {
      fn();
    } finally {
      applyingPersistedDesktopState = false;
    }
  }

  function shouldApplyRemoteDesktopStateUpdate() {
    if (typeof document === 'undefined') return true;
    return document.visibilityState === 'hidden';
  }

  function desktopLiveEventAffectsSharedState(message) {
    const kind = liveEventKind(message);
    return kind === 'desktop.app_instances.updated' ||
      kind === 'desktop.window_placement.updated' ||
      kind === 'desktop.driver_lease.updated';
  }

  function currentActiveWindowIdSnapshot() {
    let current = '';
    activeWindowId.subscribe((id) => { current = id || ''; })();
    return current;
  }

  function mergeRemoteDesktopWindows(remoteWindows = []) {
    const localWindows = windowsSnapshot();
    const localById = new Map(localWindows.map((win) => [win.windowId, win]));
    const remoteIds = new Set(remoteWindows.map((win) => win.windowId));
    const activeId = currentActiveWindowIdSnapshot();
    const activeStillExists = activeId && remoteIds.has(activeId);
    const localMaxZ = localWindows.reduce((max, win) => Math.max(max, win.zIndex || 0), 0);

    let addedZ = Math.max(1, localMaxZ - remoteWindows.length - 1);
    const merged = remoteWindows.map((remoteWin) => {
      const localWin = localById.get(remoteWin.windowId);
      if (!localWin) {
        // New remote app instances should become visible, but passive sessions
        // must not let them cover the window the local user is touching.
        return {
          ...remoteWin,
          icon: getAppIcon(remoteWin.appId),
          zIndex: activeStillExists ? Math.max(1, addedZ++) : (remoteWin.zIndex || 1),
        };
      }
      return {
        ...localWin,
        title: remoteWin.title || localWin.title,
        icon: getAppIcon(remoteWin.appId || localWin.appId),
        appContext: remoteWin.appContext || localWin.appContext,
        restoreSuspended: localWin.restoreSuspended,
      };
    });

    if (activeStillExists) {
      const nextMax = merged.reduce((max, win) => Math.max(max, win.zIndex || 0), 0);
      return {
        windows: merged.map((win) =>
          win.windowId === activeId ? { ...win, zIndex: Math.max(nextMax + 1, win.zIndex || 1) } : win
        ),
        activeWindowId: activeId,
      };
    }

    const visible = merged.filter((win) => win.mode !== 'closed' && win.mode !== 'hidden' && win.mode !== 'minimized');
    const nextActive = visible.length > 0
      ? visible.reduce((best, win) => ((win.zIndex || 0) > (best.zIndex || 0) ? win : best)).windowId
      : '';
    return { windows: merged, activeWindowId: nextActive };
  }

  async function mergeRemoteDesktopSharedState() {
    try {
      const state = await fetchDesktopState();
      if (!state?.windows) return;
      applyPersistedDesktopState(() => {
        const merged = mergeRemoteDesktopWindows(state.windows.map((w) => ({
          windowId: w.window_id,
          appId: w.app_id,
          title: w.title,
          icon: getAppIcon(w.app_id),
          x: w.geometry?.x ?? 100,
          y: w.geometry?.y ?? 100,
          width: w.geometry?.width ?? 600,
          height: w.geometry?.height ?? 400,
          mode: w.mode ?? 'normal',
          zIndex: w.z_index ?? 1,
          restoredGeometry: w.restored_geometry
            ? { x: w.restored_geometry.x, y: w.restored_geometry.y, width: w.restored_geometry.width, height: w.restored_geometry.height }
            : null,
          appContext: w.app_context ?? {},
        })));
        setWindows(merged.windows, merged.activeWindowId);
      });
    } catch (err) {
      if (err instanceof AuthRequiredError) {
        dispatch('authexpired');
      }
    }
  }

  function handleRemoteDesktopStateUpdate(message = {}) {
    const payload = liveEventPayload(message);
    observeRemoteDriverSession(payload.source_session_id || '');
    if (!desktopLiveEventAffectsSharedState(message)) return;
    // Desktop layout is viewport- and interaction-sensitive. A visible tab may
    // merge shared app identity/order, but remote saves must not seize local
    // focus or geometry. Hidden tabs can safely reload their full snapshot.
    if (shouldApplyRemoteDesktopStateUpdate()) {
      void loadDesktopState();
    } else {
      void mergeRemoteDesktopSharedState();
    }
  }

  async function loadDesktopState() {
    if (!authenticated) {
      stateLoaded = true;
      desktopReady = bootstrapStable && stateLoaded;
      return;
    }
    try {
      const state = await fetchDesktopState();
      if (state) {
        applyPersistedDesktopState(() => {
          // Restore icon positions
          if (state.icon_positions && Object.keys(state.icon_positions).length > 0) {
            setIconPositions(state.icon_positions);
          }
          // Restore windows
          if (state.windows && state.windows.length > 0) {
            const restoredWindowsRaw = state.windows.map((w) => ({
              windowId: w.window_id,
              appId: w.app_id,
              title: w.title,
              icon: getAppIcon(w.app_id),
              x: w.geometry?.x ?? 100,
              y: w.geometry?.y ?? 100,
              width: w.geometry?.width ?? 600,
              height: w.geometry?.height ?? 400,
              mode: w.mode ?? 'normal',
              zIndex: w.z_index ?? 1,
              restoredGeometry: w.restored_geometry
                ? { x: w.restored_geometry.x, y: w.restored_geometry.y, width: w.restored_geometry.width, height: w.restored_geometry.height }
                : null,
              appContext: w.app_context ?? {},
            }));
            const hydrationActiveId = state.active_window_id || pickTopRestoredWindow(restoredWindowsRaw, '')?.windowId || '';
            const lazyHydration = shouldLazyHydrateRestoredWindows(restoredWindowsRaw);
            const restoredWindows = restoredWindowsRaw.map((win) => ({
              ...win,
              restoreSuspended: shouldSuspendRestoredWindow(win, hydrationActiveId, lazyHydration),
            }));
            const recovery = shouldEnterRestoreRecovery(restoredWindows, state.active_window_id || '');
            if (recovery) {
              restoreRecovery = recovery;
              restoreRecoveryWindows = restoredWindows;
              restoreRecoveryActiveId = state.active_window_id || '';
              restoreRecoveryStatus = '';
              setWindows([], '');
            } else {
              restoreRecovery = null;
              restoreRecoveryWindows = [];
              restoreRecoveryActiveId = '';
              restoreRecoveryStatus = '';
              setWindows(restoredWindows, state.active_window_id || '');
            }
          } else {
            restoreRecovery = null;
            restoreRecoveryWindows = [];
            restoreRecoveryActiveId = '';
            restoreRecoveryStatus = '';
            setWindows([], '');
          }
        });
      }
    } catch (err) {
      if (err instanceof AuthRequiredError) {
        dispatch('authexpired');
        return;
      }
    }
    stateLoaded = true;
    desktopReady = bootstrapStable && stateLoaded;
  }

  function visibleRestoredWindows(restoredWindows) {
    return (restoredWindows || []).filter((win) =>
      win.mode !== 'closed' && win.mode !== 'hidden' && win.mode !== 'minimized'
    );
  }

  function shouldLazyHydrateRestoredWindows(restoredWindows) {
    const visibleWindows = visibleRestoredWindows(restoredWindows);
    const heavyWindows = visibleWindows.filter((win) => isHeavyAppId(win.appId));
    return visibleWindows.length > RESTORE_RECOVERY_WINDOW_LIMIT ||
      heavyWindows.length >= RESTORE_RECOVERY_HEAVY_WINDOW_LIMIT;
  }

  function shouldSuspendRestoredWindow(win, activeId, lazyHydration) {
    if (!lazyHydration || !win || !isHeavyAppId(win.appId)) return false;
    if (win.windowId === activeId) return false;
    return win.mode !== 'closed' && win.mode !== 'hidden' && win.mode !== 'minimized';
  }

  function isWindowAppBodySuspended(win) {
    return Boolean(win?.restoreSuspended && isHeavyAppId(win.appId));
  }

  function readOverviewBottomBarHeight() {
    if (typeof document === 'undefined') return 56;
    const bottomBar = document.querySelector('[data-bottom-bar]');
    if (bottomBar?.offsetHeight) return bottomBar.offsetHeight;
    const fromTheme = window
      .getComputedStyle(document.documentElement)
      .getPropertyValue('--choir-bottom-bar-height');
    const parsed = Number.parseFloat(fromTheme);
    return Number.isFinite(parsed) ? parsed : 56;
  }

  function refreshOverviewViewport() {
    if (typeof window === 'undefined') return;
    overviewViewportWidth = window.innerWidth || 1280;
    overviewViewportHeight = window.innerHeight || 800;
    overviewBottomBarHeight = readOverviewBottomBarHeight();
  }

  function clampOverviewValue(value, min, max) {
    return Math.min(Math.max(value, min), Math.max(min, max));
  }

  function renderedWindowGeometryForOverview(win) {
    const mobile = overviewViewportWidth <= RESTORE_RECOVERY_COMPACT_BREAKPOINT;
    const tablet = overviewViewportWidth <= 1024;
    const margin = mobile ? 8 : tablet ? 16 : 12;
    const minWidth = 200;
    const minHeight = 120;
    const maxWidth = Math.max(minWidth, overviewViewportWidth - margin * 2);
    const maxHeight = Math.max(
      minHeight,
      overviewViewportHeight - overviewBottomBarHeight - margin * 2
    );
    const width = Math.min(Math.max(win.width || 600, minWidth), maxWidth);
    const height = Math.min(Math.max(win.height || 400, minHeight), maxHeight);
    const maxX = Math.max(margin, overviewViewportWidth - width - margin);
    const maxY = Math.max(
      margin,
      overviewViewportHeight - overviewBottomBarHeight - height - margin
    );

    if (win.mode === 'maximized') {
      return {
        x: 0,
        y: 0,
        width: overviewViewportWidth,
        height: Math.max(minHeight, overviewViewportHeight - overviewBottomBarHeight),
      };
    }

    return {
      x: clampOverviewValue(win.x || margin, margin, maxX),
      y: clampOverviewValue(win.y || margin, margin, maxY),
      width,
      height,
    };
  }

  function getOverviewPreviewState(win, decision = null) {
    return (decision || getOverviewPreviewDecision(overviewPreviewDecisions, win.windowId)).state;
  }

  function getOverviewPreviewStyle(win, decision = null) {
    if (!desktopOverviewOpen) return '';
    const previewDecision = decision || getOverviewPreviewDecision(overviewPreviewDecisions, win.windowId);
    if (previewDecision.state !== 'live') return '';

    const mobile = overviewViewportWidth < RESTORE_RECOVERY_COMPACT_BREAKPOINT;
    const stageX = mobile ? 20 : 56;
    const stageTop = mobile ? OVERVIEW_STAGE_TOP_MOBILE : OVERVIEW_STAGE_TOP_DESKTOP;
    const bottomRail = mobile
      ? OVERVIEW_STAGE_BOTTOM_RAIL_MOBILE
      : OVERVIEW_STAGE_BOTTOM_RAIL_DESKTOP;
    const stageWidth = Math.max(260, overviewViewportWidth - stageX * 2);
    const stageHeight = Math.max(
      220,
      overviewViewportHeight - overviewBottomBarHeight - stageTop - bottomRail
    );
    const source = renderedWindowGeometryForOverview(win);
    const preferredScale = mobile
      ? previewDecision.liveCount <= 2 ? 0.58 : 0.48
      : previewDecision.liveCount <= 3 ? 0.48 : 0.38;
    const scale = clampOverviewValue(
      Math.min(
        preferredScale,
        (stageWidth * 0.72) / Math.max(source.width, 1),
        (stageHeight * 0.74) / Math.max(source.height, 1)
      ),
      mobile ? 0.32 : 0.24,
      mobile ? 0.62 : 0.54
    );
    const targetWidth = source.width * scale;
    const targetHeight = source.height * scale;
    const sourceMaxX = Math.max(1, overviewViewportWidth - source.width);
    const sourceMaxY = Math.max(1, overviewViewportHeight - overviewBottomBarHeight - source.height);
    const normalizedX = clampOverviewValue(source.x / sourceMaxX, 0, 1);
    const normalizedY = clampOverviewValue(source.y / sourceMaxY, 0, 1);
    const liveIndex = Math.max(0, previewDecision.liveIndex);
    const slotWidth = Math.max(0, stageWidth - targetWidth);
    const slotHeight = Math.max(0, stageHeight - targetHeight);
    const mobileSlots = [
      { x: 0.02, y: 0.02 },
      { x: 0.58, y: 0.2 },
      { x: 0.25, y: 0.47 },
    ];
    let slotX = normalizedX;
    let slotY = normalizedY;

    if (mobile && previewDecision.liveCount <= mobileSlots.length) {
      slotX = mobileSlots[liveIndex]?.x ?? normalizedX;
      slotY = mobileSlots[liveIndex]?.y ?? normalizedY;
    } else if (!mobile) {
      const columns = Math.min(3, Math.max(1, previewDecision.liveCount));
      const rows = Math.max(1, Math.ceil(previewDecision.liveCount / columns));
      const col = liveIndex % columns;
      const row = Math.floor(liveIndex / columns);
      slotX = columns === 1 ? 0.5 : col / (columns - 1);
      slotY = rows === 1 ? 0.12 : row / (rows - 1);
    }

    const nudgeX = (normalizedX - 0.5) * (mobile ? 18 : 28);
    const nudgeY = (normalizedY - 0.5) * (mobile ? 14 : 22);
    const targetX = clampOverviewValue(
      stageX + slotX * slotWidth + nudgeX,
      stageX,
      stageX + slotWidth
    );
    const targetY = clampOverviewValue(
      stageTop + slotY * slotHeight + nudgeY,
      stageTop,
      stageTop + slotHeight
    );
    const translateX = Math.round(targetX - source.x);
    const translateY = Math.round(targetY - source.y);

    return [
      `--overview-translate-x:${translateX}px;`,
      `--overview-translate-y:${translateY}px;`,
      `--overview-scale:${scale.toFixed(4)};`,
    ].join(' ');
  }

  function restoreRecoveryRequestedByURL() {
    if (typeof window === 'undefined') return false;
    const params = new URLSearchParams(window.location.search || '');
    const recoveryValue = String(params.get('desktop_recovery') || '').toLowerCase();
    const safeValue = String(params.get('desktop_safe') || params.get('safe') || '').toLowerCase();
    return ['1', 'true', 'yes', 'on'].includes(recoveryValue) || ['1', 'true', 'yes', 'on'].includes(safeValue);
  }

  function shouldEnterRestoreRecovery(restoredWindows, activeId = '') {
    const visibleWindows = visibleRestoredWindows(restoredWindows);
    const heavyWindows = visibleWindows.filter((win) => isHeavyAppId(win.appId));
    const urlRequested = restoreRecoveryRequestedByURL();

    if (!urlRequested && visibleWindows.length <= RESTORE_RECOVERY_WINDOW_LIMIT && heavyWindows.length < RESTORE_RECOVERY_HEAVY_WINDOW_LIMIT) {
      return null;
    }

    const topWindow = pickTopRestoredWindow(restoredWindows, activeId);
    return {
      reason: urlRequested ? 'url' : 'heavy-restore',
      totalCount: restoredWindows.length,
      visibleCount: visibleWindows.length,
      heavyCount: heavyWindows.length,
      topWindowTitle: topWindow?.title || '',
      topWindowAppId: topWindow?.appId || '',
    };
  }

  function pickTopRestoredWindow(restoredWindows, activeId = '') {
    const candidates = (restoredWindows || []).filter((win) => win.mode !== 'closed' && win.mode !== 'hidden');
    const activeWindow = candidates.find((win) => win.windowId === activeId);
    if (activeWindow) return activeWindow;
    if (candidates.length === 0) return null;
    return candidates.reduce((best, win) => ((win.zIndex || 0) > (best.zIndex || 0) ? win : best));
  }

  function serializeWindowsForSave(nextWindows) {
    return nextWindows.map((w) => ({
      window_id: w.windowId,
      app_id: w.appId,
      title: w.title,
      geometry: { x: w.x, y: w.y, width: w.width, height: w.height },
      restored_geometry: w.restoredGeometry,
      mode: w.mode,
      z_index: w.zIndex,
      app_context: w.appContext,
    }));
  }

  function readDesktopStateForSave() {
    let currentWindows;
    let currentActiveId;
    let currentIconPositions;
    windows.subscribe((w) => { currentWindows = w; })();
    activeWindowId.subscribe((id) => { currentActiveId = id; })();
    iconPositions.subscribe((p) => { currentIconPositions = p; })();

    return {
      windows: serializeWindowsForSave((currentWindows || []).filter((w) => w.mode !== 'hidden')),
      active_window_id: currentActiveId || '',
      icon_positions: currentIconPositions,
    };
  }

  async function persistWindowSet(nextWindows, nextActiveId) {
    setWindows(nextWindows, nextActiveId || '');
    await tick();
    await saveDesktopState({
      windows: serializeWindowsForSave(nextWindows),
      active_window_id: nextActiveId || '',
    });
  }

  async function persistRecoveredWindows(nextWindows, nextActiveId) {
    restoreRecoverySaving = true;
    restoreRecoveryStatus = '';
    try {
      await persistWindowSet(nextWindows, nextActiveId || '');
      restoreRecovery = null;
      restoreRecoveryWindows = [];
      restoreRecoveryActiveId = '';
      restoreRecoveryStatus = '';
    } catch (err) {
      if (err instanceof AuthRequiredError) {
        dispatch('authexpired');
        return;
      }
      restoreRecoveryStatus = err?.message || 'Could not update saved desktop state';
      setWindows([], '');
    } finally {
      restoreRecoverySaving = false;
    }
  }

  async function handleClearRestoredWindows() {
    await persistRecoveredWindows([], '');
    showToast('Saved desktop windows cleared');
  }

  async function handleKeepTopRestoredWindow() {
    const topWindow = pickTopRestoredWindow(restoreRecoveryWindows, restoreRecoveryActiveId);
    if (!topWindow) {
      await handleClearRestoredWindows();
      return;
    }
    await persistRecoveredWindows([{ ...topWindow, mode: topWindow.mode === 'minimized' ? 'normal' : topWindow.mode }], topWindow.windowId);
    showToast('Saved desktop reduced to one window');
  }

  function handleRestoreAllWindows() {
    const restoredWindows = restoreRecoveryWindows;
    const activeId = restoreRecoveryActiveId;
    restoreRecovery = null;
    restoreRecoveryWindows = [];
    restoreRecoveryActiveId = '';
    restoreRecoveryStatus = '';
    setWindows(restoredWindows, activeId);
  }

  function delay(ms) {
    return new Promise((resolve) => setTimeout(resolve, ms));
  }

  function appendBootLine(message, kind = 'info') {
    const elapsedMs = bootStartedAt ? Date.now() - bootStartedAt : 0;
    const elapsed = `${Math.max(0, Math.floor(elapsedMs / 1000)).toString().padStart(2, '0')}s`;
    bootPromptPlaceholder = message;
    promptStatus = desktopReady ? '' : message;
    bootLines = [
      ...bootLines.slice(-(MAX_BOOT_LINES - 1)),
      { id: ++bootLineCounter, elapsed, message, kind },
    ];
  }

  function scheduleSave() {
    if (!authenticated || !stateLoaded) return;
    if (!isDrivingSession()) return;
    if (saveTimer) clearTimeout(saveTimer);
    saveTimer = setTimeout(persistDesktopState, SAVE_DEBOUNCE_MS);
  }

  async function flushDesktopState(options = {}) {
    if (saveTimer) {
      clearTimeout(saveTimer);
      saveTimer = null;
    }
    await persistDesktopState(options);
  }

  async function persistDesktopState(options = {}) {
    if ((!authenticated && !options.allowSignedOutTransition) || !stateLoaded) return;
    if (!isDrivingSession()) return;
    if (saveTimer) {
      clearTimeout(saveTimer);
      saveTimer = null;
    }
    try {
      await saveDesktopState(readDesktopStateForSave(), {
        keepalive: options.keepalive === true,
      });
    } catch (err) {
      if (err instanceof AuthRequiredError) {
        dispatch('authexpired');
      }
    }
  }

  // ---- Auto-save on store changes ----

  let unsubscribeWindows;
  let unsubscribeActive;
  let unsubscribeIconPositions;

  function handlePageHide() {
    void flushDesktopState({ keepalive: true });
  }

  function handleVisibilityChange() {
    if (document.visibilityState === 'hidden') {
      void flushDesktopState({ keepalive: true });
    }
  }

  function handleLocalDriverInput() {
    renewDriverLease();
  }

  const DRIVER_INPUT_EVENTS = ['pointerdown', 'keydown', 'wheel', 'touchstart'];

  function installDriverInputListeners() {
    for (const eventName of DRIVER_INPUT_EVENTS) {
      window.addEventListener(eventName, handleLocalDriverInput, { capture: true, passive: true });
    }
  }

  function removeDriverInputListeners() {
    for (const eventName of DRIVER_INPUT_EVENTS) {
      window.removeEventListener(eventName, handleLocalDriverInput, { capture: true });
    }
  }

  // ---- Bootstrap fetch (preserved for session renewal, not displayed) ----

  async function stabilizeBootstrap() {
    bootstrapStable = false;
    bootstrapError = '';
    let previousSandboxId = '';
    let attempt = 0;
    const deadline = Date.now() + BOOTSTRAP_STABILITY_DEADLINE_MS;
    appendBootLine('Resolving active computer');

    while (authenticated && Date.now() < deadline) {
      attempt++;
      let res;
      try {
        res = await fetchBootstrapProbe();
      } catch (err) {
        if (err instanceof AuthRequiredError) {
          throw err;
        }
        bootstrapError = err?.name === 'AbortError' ? 'Computer boot is still pending' : 'Bootstrap request failed';
        appendBootLine(
          err?.name === 'AbortError'
            ? `Bootstrap probe ${attempt} is still waiting; retrying`
            : `Bootstrap probe ${attempt} lost contact; retrying`,
          'warn'
        );
        await delay(BOOTSTRAP_STABILITY_DELAY_MS);
        continue;
      }
      if (!res.ok) {
        bootstrapError = `Bootstrap failed (${res.status})`;
        appendBootLine(`VM route returned ${res.status}; retrying`, 'warn');
        await delay(BOOTSTRAP_STABILITY_DELAY_MS);
        continue;
      }
      bootstrapData = await res.json();
      const sandboxId = (bootstrapData?.sandbox_id || '').trim();
      if (sandboxId !== '' && sandboxId === previousSandboxId) {
        bootstrapStable = true;
        appendBootLine('Stable computer route confirmed');
        return true;
      }
      if (sandboxId !== '') {
        appendBootLine(previousSandboxId ? 'Waiting for route to stabilize' : 'Candidate computer route found');
      } else {
        appendBootLine('Waiting for computer identity');
      }
      previousSandboxId = sandboxId;
      await delay(BOOTSTRAP_STABILITY_DELAY_MS);
    }
    bootstrapError = 'Desktop routing is still stabilizing';
    return false;
  }

  async function fetchBootstrapProbe() {
    const controller = new AbortController();
    const timeout = setTimeout(() => controller.abort(), BOOTSTRAP_PROBE_TIMEOUT_MS);
    try {
      return await fetchWithRenewal('/api/shell/bootstrap', {
        method: 'GET',
        signal: controller.signal,
      });
    } finally {
      clearTimeout(timeout);
    }
  }

  // ---- Live channel (WebSocket) ----

  function connectLiveChannel() {
    if (!authenticated) return;
    liveStatus.set('connecting');
    try {
      const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
      const baseUrl = `${protocol}//${window.location.host}/api/ws`;
      const wsUrl = withDesktopSelector(lastLiveStreamSeq > 0 ? `${baseUrl}?after_seq=${lastLiveStreamSeq}` : baseUrl);
      ws = new WebSocket(wsUrl);
      ws.onopen = () => {
        liveStatus.set('connected');
        wsReconnectAttempt = 0;
      };
      ws.onmessage = (event) => {
        handleLiveMessage(event.data);
      };
      ws.onerror = () => {
        liveStatus.set('error');
      };
      ws.onclose = () => {
        if (wsClosedByLogout) {
          liveStatus.set('disconnected');
          return;
        }
        liveStatus.update((s) => s === 'error' ? s : 'disconnected');
        attemptWsReconnection();
      };
    } catch (_err) {
      liveStatus.set('error');
    }
  }

  function handleLiveMessage(raw) {
    let message;
    try {
      message = JSON.parse(raw);
    } catch (_err) {
      return;
    }
    if (message?.type === 'connected') {
      liveStatus.set('connected');
      return;
    }
    if (message?.type === 'event') {
      if (Number.isFinite(Number(message.stream_seq))) {
        lastLiveStreamSeq = Math.max(lastLiveStreamSeq, Number(message.stream_seq));
      }
      dispatchLiveEvent(message);
      if (desktopLiveEventAffectsSharedState(message) && !isOwnLiveEvent(message)) {
        handleRemoteDesktopStateUpdate(message);
      }
      return;
    }
    if (message?.type === 'ping' && ws?.readyState === WebSocket.OPEN) {
      try {
        ws.send(JSON.stringify({ type: 'ack', stream_seq: lastLiveStreamSeq }));
      } catch (_err) {
        // The close handler owns reconnect decisions.
      }
    }
  }

  async function attemptWsReconnection() {
    if (!authenticated) return;
    if (wsReconnecting) return;
    if (wsClosedByLogout) return;
    if (wsReconnectAttempt >= MAX_WS_RECONNECT_ATTEMPTS) {
      liveStatus.set('error');
      return;
    }
    wsReconnecting = true;
    wsReconnectAttempt++;
    const delay = WS_RECONNECT_BASE_DELAY * wsReconnectAttempt;
    try {
      await new Promise((resolve) => setTimeout(resolve, delay));
      const { renewed } = await renewSession();
      if (!renewed) {
        dispatch('authexpired');
        return;
      }
      connectLiveChannel();
    } finally {
      wsReconnecting = false;
    }
  }

  // ---- Event handlers ----

  function requestAuth(detail = {}) {
    dispatch('authrequired', detail);
  }

  function normalizePublicRoutePath(routePath) {
    const normalized = `/${String(routePath || '').trim().replace(/^\/+/, '')}`;
    return normalized.startsWith('/pub/vtext/') ? normalized.replace(/\/+$/, '') : normalized;
  }

  function windowsSnapshot() {
    let current = [];
    windows.subscribe((items) => {
      current = items || [];
    })();
    return current;
  }

  function openPublishedVText(routePath, guest = false) {
    const normalizedRoutePath = normalizePublicRoutePath(routePath);
    const matchingWindows = windowsSnapshot().filter((win) =>
      win.appId === 'vtext' &&
      win.mode !== 'closed' &&
      win.mode !== 'hidden' &&
      normalizePublicRoutePath(win.appContext?.publishedRoutePath || '') === normalizedRoutePath
    );

    if (matchingWindows.length > 0) {
      const primary = matchingWindows.reduce((best, win) =>
        (win.zIndex || 0) > (best.zIndex || 0) ? win : best
      );
      for (const duplicate of matchingWindows) {
        if (duplicate.windowId !== primary.windowId) {
          closeWindow(duplicate.windowId);
        }
      }
      if (primary.mode === 'minimized') {
        restoreWindow(primary.windowId);
      } else {
        focusWindow(primary.windowId);
      }
      return;
    }

    openApp('vtext', 'VText', '📝', {
      windowTitle: 'Published VText',
      publishedRoutePath: normalizedRoutePath,
      publishedGuest: guest,
      allowMultiple: true,
    });
  }

  function handleLaunchApp(event) {
    if (!authenticated) {
      const publicApps = new Set(['podcast', 'image', 'audio', 'video', 'pdf', 'epub', 'trace', 'vtext', 'browser']);
      const appId = event.detail?.appId || '';
      if (publicApps.has(appId)) {
        openApp(appId, event.detail?.appName || appId, event.detail?.icon || '', {
          ...(event.detail?.appContext || {}),
          guestMode: true,
        });
        return;
      }
      requestAuth({
        kind: 'app_launch',
        appId,
        appName: event.detail?.appName || 'app',
        icon: event.detail?.icon || '',
        appContext: event.detail?.appContext || {},
      });
      return;
    }
    if (!desktopReady) {
      showToast('Desktop is still connecting');
      return;
    }
    openApp(event.detail.appId, event.detail.appName, event.detail.icon, {
      ...(event.detail.appContext || {}),
    });
  }

  function handleWindowClose(event) {
    closeWindow(event.detail.windowId);
    scheduleSave();
  }

  function handleWindowFocus(event) {
    clearWindowRestoreSuspension(event.detail.windowId);
    focusWindow(event.detail.windowId);
    desktopOverviewOpen = false;
    scheduleSave();
  }

  function handleWindowMinimize(event) {
    minimizeWindow(event.detail.windowId);
    scheduleSave();
  }

  function handleWindowMaximize(event) {
    maximizeWindow(event.detail.windowId);
    scheduleSave();
  }

  function handleWindowRestore(event) {
    restoreWindow(event.detail.windowId);
    scheduleSave();
  }

  function handleWindowMove(event) {
    moveWindow(event.detail.windowId, event.detail.x, event.detail.y);
    scheduleSave();
  }

  function handleWindowResize(event) {
    resizeWindow(
      event.detail.windowId,
      event.detail.x,
      event.detail.y,
      event.detail.width,
      event.detail.height
    );
    scheduleSave();
  }

  function handleWindowAppContextChange(event) {
    updateWindowAppContext(
      event.detail?.windowId || '',
      event.detail?.appContext || {},
      event.detail?.title || ''
    );
    scheduleSave();
  }

  function handleLogout() {
    void flushDesktopState({ keepalive: true });
    closeLiveChannel();
    dispatch('logout');
  }

  async function handlePromptSubmit(event) {
    const text = (event.detail?.text || '').trim();
    if (!text) return;
    if (!authenticated) {
      requestAuth({ kind: 'prompt', text });
      return;
    }
    submitPromptText(text);
  }

  async function submitPromptText(text) {
    if (!desktopReady) {
      showToast('Desktop is still connecting');
      return;
    }

    const fallbackWindowTitle = text.length > 28 ? `${text.slice(0, 28)}…` : text;

    try {
      promptStatus = 'Routing through conductor...';
      const submission = await submitConductorPrompt(text);
      const conductorSubmissionId = submission.submission_id || '';
      promptStatus = 'Waiting for conductor decision...';
      const decision = await waitForConductorDecision(conductorSubmissionId);

      if (decision.action === 'toast') {
        promptStatus = '';
        showToast(decision.message || 'Conductor acknowledged the request');
        return;
      }

      if (decision.action !== 'open_app') {
        promptStatus = '';
        showToast('Conductor returned an unsupported route');
        return;
      }

      if (decision.app === 'vtext') {
        promptStatus = `Opening ${decision.title || 'VText'}...`;
        openApp('vtext', 'VText', '📝', {
          windowTitle: decision.title || fallbackWindowTitle,
          docId: decision.doc_id || '',
          seedPrompt: decision.seed_prompt || text,
          initialContent: decision.initial_content || decision.seed_prompt || text,
          createInitialVersion: decision.create_initial_version !== false,
          conductorLoopId: conductorSubmissionId,
        });
        setTimeout(() => {
          if (promptStatus.startsWith('Opening ')) promptStatus = '';
        }, 1800);
        return;
      }

      promptStatus = `Opening ${decision.title || decision.app || 'app'}...`;
      openApp(decision.app || 'browser', decision.title || decision.app || fallbackWindowTitle, '', {
        windowTitle: decision.title || fallbackWindowTitle,
        sourceUrl: decision.source_url || text,
        mediaType: decision.media_type || '',
        appHint: decision.app_hint || decision.app || '',
        contentId: decision.content_id || '',
        conductorLoopId: conductorSubmissionId,
      });
      setTimeout(() => {
        if (promptStatus.startsWith('Opening ')) promptStatus = '';
      }, 1800);
    } catch (err) {
      if (err instanceof AuthRequiredError) {
        dispatch('authexpired');
        return;
      }
      promptStatus = '';
      showToast(err.message || 'Conductor submission failed', { kind: 'error' });
    }
  }

  async function handleOpenTextFile(event) {
    if (!authenticated) {
      requestAuth({ kind: 'file_open', fileName: event.detail?.fileName || 'document' });
      return;
    }
    const pathSegments = event.detail?.pathSegments || [];
    const fileName = event.detail?.fileName || pathSegments[pathSegments.length - 1] || 'Document';
    const path = '/api/files/' + pathSegments.map(encodeURIComponent).join('/');

    try {
      const res = await fetchWithRenewal(path, { method: 'GET' });
      if (!res.ok) {
        if (res.status === 401) {
          dispatch('authexpired');
          return;
        }
        showToast(`Could not open ${fileName}`);
        return;
      }
      const content = await res.text();
      const doc = await openFileDocument({
        sourcePath: pathSegments.join('/'),
        title: fileName,
        initialContent: content,
      });
      openApp('vtext', 'VText', '📝', {
        windowTitle: fileName,
        fileName,
        docId: doc.doc_id,
        sourcePath: pathSegments.join('/'),
      });
      showToast(`Opened ${fileName} in VText`);
    } catch (err) {
      if (err instanceof AuthRequiredError) {
        dispatch('authexpired');
        return;
      }
      showToast(`Could not open ${fileName}`);
    }
  }

  function handleOpenMediaFile(event) {
    if (!authenticated) {
      requestAuth({ kind: 'file_open', fileName: event.detail?.fileName || 'media file' });
      return;
    }
    if (!desktopReady) {
      showToast('Desktop is still connecting');
      return;
    }

    const detail = event.detail || {};
    const appId = detail.appId || '';
    if (!['image', 'audio', 'video', 'pdf', 'epub'].includes(appId)) {
      showToast(`Could not open ${detail.fileName || 'media file'}`);
      return;
    }

    openApp(appId, detail.fileName || appId, getAppIcon(appId), {
      windowTitle: detail.fileName || appId,
      fileName: detail.fileName || '',
      filePath: detail.filePath || (detail.pathSegments || []).join('/'),
      mediaType: detail.mediaType || '',
      appHint: appId,
      allowMultiple: true,
    });
    showToast(`Opened ${detail.fileName || appId}`);
  }

  function handleOpenVTextFromContent(event) {
    if (!authenticated) {
      requestAuth({ kind: 'open_vtext', title: event.detail?.title || 'document' });
      return;
    }
    if (!desktopReady) {
      showToast('Desktop is still connecting');
      return;
    }
    const detail = event.detail || {};
    const docId = detail.docId || '';
    openApp('vtext', 'VText', '📝', {
      windowTitle: detail.title || 'Radio Brief',
      docId,
      initialContent: detail.initialContent || '',
      seedPrompt: detail.seedPrompt || '',
      createInitialVersion: docId ? false : detail.createInitialVersion !== false,
      allowMultiple: true,
      sourceUrl: detail.sourceUrl || '',
      sourceContentId: detail.sourceContentId || '',
      appHint: detail.appHint || '',
      createdFrom: detail.createdFrom || 'content_viewer',
    });
    showToast(detail.toastMessage || 'Opened in VText');
  }

  function handleOpenTraceFromContent(event) {
    if (!authenticated) {
      requestAuth({ kind: 'open_trace', title: event.detail?.title || 'Trace evidence' });
      return;
    }
    if (!desktopReady) {
      showToast('Desktop is still connecting');
      return;
    }

    const detail = event.detail || {};
    const appContext = {
      windowTitle: detail.title || 'Trace',
      trajectoryId: detail.trajectoryId || detail.traceId || '',
      acceptanceId: detail.acceptanceId || '',
    };
    const existing = windowsSnapshot().find((win) =>
      win.appId === 'trace' &&
      win.mode !== 'closed' &&
      win.mode !== 'hidden'
    );
    if (existing) {
      updateWindowAppContext(existing.windowId, appContext, appContext.windowTitle);
      if (existing.mode === 'minimized') {
        restoreWindow(existing.windowId);
      } else {
        focusWindow(existing.windowId);
      }
    } else {
      openApp('trace', 'Trace', getAppIcon('trace'), appContext);
    }
    showToast(detail.toastMessage || 'Opened Trace evidence');
    scheduleSave();
  }

  function handleIconPositionsChanged() {
    scheduleSave();
  }

  function handleResetDesktop() {
    if (!authenticated) {
      requestAuth({ kind: 'reset_desktop' });
      return;
    }
    setWindows([], '');
    setIconPositions(getDefaultIconPositions());
    scheduleSave();
    showToast('Desktop layout reset');
  }

  function handleOpenComputeMonitor() {
    openApp('compute-monitor', 'Compute Monitor', getAppIcon('compute-monitor'), {
      windowTitle: 'Compute Monitor',
    });
  }

  function handleShowDesktopOverview() {
    refreshOverviewViewport();
    desktopOverviewOpen = true;
  }

  function handleCloseDesktopOverview() {
    desktopOverviewOpen = false;
  }

  function handleOverviewFocusWindow(event) {
    const windowId = event.detail?.windowId || '';
    if (!windowId) return;
    clearWindowRestoreSuspension(windowId);
    focusWindow(windowId);
    desktopOverviewOpen = false;
    scheduleSave();
  }

  function handleOverviewMinimizeWindow(event) {
    const windowId = event.detail?.windowId || '';
    if (!windowId) return;
    minimizeWindow(windowId);
    scheduleSave();
  }

  function handleOverviewCloseWindow(event) {
    const windowId = event.detail?.windowId || '';
    if (!windowId) return;
    closeWindow(windowId);
    scheduleSave();
  }

  function handleOverviewSuspendWindow(event) {
    const windowId = event.detail?.windowId || '';
    if (!windowId) return;
    const suspended = suspendWindowBody(windowId);
    if (suspended) {
      showToast('Window suspended');
      scheduleSave();
    }
  }

  function handleOverviewSuspendBackground() {
    const count = suspendBackgroundHeavyWindows($activeWindowId);
    showToast(count > 0 ? `Suspended ${count} background app${count === 1 ? '' : 's'}` : 'No background apps to suspend');
    if (count > 0) scheduleSave();
  }

  function handleOverviewOpenComputeMonitor() {
    handleOpenComputeMonitor();
    desktopOverviewOpen = false;
  }

  async function handleOverviewKeepActiveOnly(event) {
    desktopOverviewOpen = false;
    await handleKeepWindowOnly(event);
  }

  async function handleOverviewClearSavedWindows() {
    desktopOverviewOpen = false;
    await handleClearDesktopWindows();
  }

  async function handleClearDesktopWindows() {
    if (!authenticated) {
      requestAuth({ kind: 'reset_desktop' });
      return;
    }
    try {
      await persistWindowSet([], '');
      showToast('Saved desktop windows cleared');
    } catch (err) {
      if (err instanceof AuthRequiredError) {
        dispatch('authexpired');
        return;
      }
      showToast('Could not clear saved windows', { kind: 'error' });
    }
  }

  async function handleKeepWindowOnly(event) {
    if (!authenticated) {
      requestAuth({ kind: 'reset_desktop' });
      return;
    }
    const targetWindowId = event.detail?.windowId || '';
    const targetWindow = windowsSnapshot().find((win) => win.windowId === targetWindowId);
    if (!targetWindow) {
      showToast('Could not find the selected window', { kind: 'error' });
      return;
    }
    const keptWindow = {
      ...targetWindow,
      mode: targetWindow.mode === 'minimized' ? 'normal' : targetWindow.mode,
      restoreSuspended: false,
    };
    try {
      await persistWindowSet([keptWindow], keptWindow.windowId);
      showToast('Saved desktop reduced to one window');
    } catch (err) {
      if (err instanceof AuthRequiredError) {
        dispatch('authexpired');
        return;
      }
      showToast('Could not update saved windows', { kind: 'error' });
    }
  }

  function showToast(message, options = {}) {
    const id = ++toastCounter;
    const kind = options.kind || 'info';
    const durationMs = options.durationMs ?? (kind === 'error' ? 9000 : 2400);
    toasts = [...toasts, { id, message, kind }];
    setTimeout(() => {
      toasts = toasts.filter((toast) => toast.id !== id);
    }, durationMs);
  }

  // ---- Lifecycle ----

  onMount(() => {
    mounted = true;
    currentSessionId();
    installDriverInputListeners();
    refreshOverviewViewport();
    window.addEventListener('resize', refreshOverviewViewport);
    window.addEventListener('pagehide', handlePageHide);
    document.addEventListener('visibilitychange', handleVisibilityChange);
    lastAuthenticated = authenticated;
    if (authenticated) {
      startAuthenticatedDesktop();
    } else {
      enterPublicDesktop();
    }

    // Subscribe to store changes for auto-save
    unsubscribeWindows = windows.subscribe(() => {
      if (stateLoaded && !applyingPersistedDesktopState) scheduleSave();
    });
    unsubscribeActive = activeWindowId.subscribe(() => {
      if (stateLoaded && !applyingPersistedDesktopState) scheduleSave();
    });
    unsubscribeIconPositions = iconPositions.subscribe(() => {
      if (stateLoaded && !applyingPersistedDesktopState) scheduleSave();
    });
  });

  onDestroy(() => {
    mounted = false;
    void flushDesktopState({ keepalive: true });
    window.removeEventListener('resize', refreshOverviewViewport);
    window.removeEventListener('pagehide', handlePageHide);
    document.removeEventListener('visibilitychange', handleVisibilityChange);
    removeDriverInputListeners();
    closeLiveChannel();
    if (saveTimer) clearTimeout(saveTimer);
    if (unsubscribeWindows) unsubscribeWindows();
    if (unsubscribeActive) unsubscribeActive();
    if (unsubscribeIconPositions) unsubscribeIconPositions();
  });
</script>

<div
  class="desktop {desktopReady ? 'desktop-ready' : 'desktop-loading'}"
  data-desktop
  data-shell
  data-authenticated={authenticated}
  data-desktop-ready={desktopReady}
>
  <!-- Desktop surface (floating icons + windows, full viewport width) -->
  <div class="desktop-area {desktopReady ? 'state-loaded' : 'state-loading'}" data-desktop-windows>
    <!-- Floating desktop icons (z-index below windows) -->
    <FloatingDesktopIcons on:launchapp={handleLaunchApp} on:iconpositionschanged={handleIconPositionsChanged} />

    <!-- Floating windows (rendered on top of icons) -->
    {#if desktopReady}
      {#if restoreRecovery}
        <section class="desktop-recovery" data-desktop-recovery role="status" aria-live="polite">
          <div>
            <p class="recovery-kicker">Desktop recovery</p>
            <h2>Saved windows are paused</h2>
            <p>
              {restoreRecovery.visibleCount} visible windows, including {restoreRecovery.heavyCount} heavy app windows, were saved for this computer.
              Loading them all at once can crash mobile Safari.
            </p>
            {#if restoreRecovery.topWindowTitle}
              <p class="recovery-top-window">
                Top saved window: <strong>{restoreRecovery.topWindowTitle}</strong>
              </p>
            {/if}
            {#if restoreRecoveryStatus}
              <p class="recovery-status" role="alert">{restoreRecoveryStatus}</p>
            {/if}
          </div>
          <div class="recovery-actions">
            <button
              type="button"
              class="recovery-primary"
              data-desktop-recovery-clear
              disabled={restoreRecoverySaving}
              on:click={handleClearRestoredWindows}
            >
              Clear saved windows
            </button>
            <button
              type="button"
              data-desktop-recovery-keep-top
              disabled={restoreRecoverySaving}
              on:click={handleKeepTopRestoredWindow}
            >
              Keep top window only
            </button>
            <button
              type="button"
              data-desktop-recovery-restore-all
              disabled={restoreRecoverySaving}
              on:click={handleRestoreAllWindows}
            >
              Restore all anyway
            </button>
          </div>
        </section>
      {/if}
      {#each $windows as win (win.windowId)}
        {#if win.mode !== 'closed' && win.mode !== 'hidden'}
          <FloatingWindow
            windowId={win.windowId}
            appId={win.appId}
            title={win.title}
            x={win.x}
            y={win.y}
            width={win.width}
            height={win.height}
            mode={win.mode}
            zIndex={win.zIndex}
            active={win.windowId === $activeWindowId}
            restoredGeometry={win.restoredGeometry}
            overviewOpen={desktopOverviewOpen}
            overviewPreviewState={getOverviewPreviewState(win, overviewPreviewDecisions[win.windowId])}
            overviewPreviewStyle={getOverviewPreviewStyle(win, overviewPreviewDecisions[win.windowId])}
            on:close={handleWindowClose}
            on:focus={handleWindowFocus}
            on:minimize={handleWindowMinimize}
            on:maximize={handleWindowMaximize}
            on:restore={handleWindowRestore}
            on:move={handleWindowMove}
            on:resize={handleWindowResize}
          >
            {#if isWindowAppBodySuspended(win)}
              <div class="app-content suspended-app-content" data-suspended-app data-suspended-app-id={win.appId}>
                <div class="suspended-card">
                  <p class="suspended-kicker">Paused restore</p>
                  <h2>{win.title}</h2>
                  <p>This heavy app window is suspended until it is raised, so the desktop can recover without mounting every saved app at once.</p>
                  <button type="button" on:click={() => handleWindowFocus({ detail: { windowId: win.windowId } })}>Resume app</button>
                </div>
              </div>
            {:else if win.appId === 'files'}
              <div class="app-content files-content" data-files-app>
                <FileBrowser
                  on:authexpired={() => dispatch('authexpired')}
                  on:opentextfile={handleOpenTextFile}
                  on:openmediafile={handleOpenMediaFile}
                />
              </div>
            {:else if win.appId === 'browser'}
              <div class="app-content browser-content" data-browser-app-container>
                <BrowserApp
                  appContext={win.appContext}
                  {authenticated}
                  on:authexpired={() => dispatch('authexpired')}
                  on:openvtext={handleOpenVTextFromContent}
                />
              </div>
            {:else if win.appId === 'apps-changes'}
              <div class="app-content apps-changes-content" data-apps-changes-window>
                <AppsChangesApp
                  appContext={win.appContext}
                  on:authexpired={() => dispatch('authexpired')}
                  on:openvtext={handleOpenVTextFromContent}
                  on:opentrace={handleOpenTraceFromContent}
                />
              </div>
            {:else if win.appId === 'terminal'}
              <div class="app-content terminal-content" data-terminal-app>
                <TerminalApp windowId={win.windowId} />
              </div>
            {:else if win.appId === 'compute-monitor'}
              <div class="app-content compute-monitor-content" data-compute-monitor-window>
                <ComputeMonitorApp
                  windowId={win.windowId}
                  {authenticated}
                  on:authexpired={() => dispatch('authexpired')}
                  on:clearsavedwindows={handleClearDesktopWindows}
                  on:keepwindowonly={handleKeepWindowOnly}
                />
              </div>
            {:else if win.appId === 'settings'}
              <div class="app-content settings-content" data-settings-window>
                <SettingsApp
                  {currentUser}
                  on:authexpired={() => dispatch('authexpired')}
                  on:resetdesktop={handleResetDesktop}
                  on:opencomputemonitor={handleOpenComputeMonitor}
                />
              </div>
            {:else if win.appId === 'vtext'}
              <div class="app-content vtext-content" data-vtext-app>
                <VTextEditor
                  windowId={win.windowId}
                  {currentUser}
                  {authenticated}
                  appContext={win.appContext}
                  on:authexpired={() => dispatch('authexpired')}
                  on:authrequired={(event) => requestAuth(event.detail || {})}
                  on:contextchange={handleWindowAppContextChange}
                />
              </div>
            {:else if win.appId === 'trace'}
              <div class="app-content trace-content" data-trace-window>
                <TraceApp
                  {authenticated}
                  appContext={win.appContext}
                  on:authexpired={() => dispatch('authexpired')}
                  on:authrequired={(event) => requestAuth(event.detail || {})}
                />
              </div>
            {:else if win.appId === 'podcast'}
              <div class="app-content podcast-content" data-podcast-window>
                <PodcastApp
                  appContext={{ ...win.appContext, appId: win.appId }}
                  {authenticated}
                  on:authexpired={() => dispatch('authexpired')}
                  on:authrequired={(event) => requestAuth(event.detail || {})}
                  on:openvtext={handleOpenVTextFromContent}
                />
              </div>
            {:else if win.appId === 'image'}
              <div class="app-content image-content" data-image-window>
                <ImageApp
                  windowId={win.windowId}
                  appContext={{ ...win.appContext, appId: win.appId }}
                  on:authexpired={() => dispatch('authexpired')}
                  on:contextchange={handleWindowAppContextChange}
                />
              </div>
            {:else if win.appId === 'audio'}
              <div class="app-content audio-content" data-audio-window>
                <AudioApp
                  windowId={win.windowId}
                  appContext={{ ...win.appContext, appId: win.appId }}
                  on:authexpired={() => dispatch('authexpired')}
                  on:contextchange={handleWindowAppContextChange}
                />
              </div>
            {:else if win.appId === 'video'}
              <div class="app-content video-content" data-video-window>
                <VideoApp
                  windowId={win.windowId}
                  appContext={{ ...win.appContext, appId: win.appId }}
                  on:authexpired={() => dispatch('authexpired')}
                  on:contextchange={handleWindowAppContextChange}
                />
              </div>
            {:else if win.appId === 'pdf'}
              <div class="app-content pdf-content" data-pdf-window>
                <PdfApp
                  windowId={win.windowId}
                  appContext={{ ...win.appContext, appId: win.appId }}
                  on:authexpired={() => dispatch('authexpired')}
                  on:contextchange={handleWindowAppContextChange}
                />
              </div>
            {:else if win.appId === 'epub'}
              <div class="app-content epub-content" data-epub-window>
                <EpubApp
                  windowId={win.windowId}
                  appContext={{ ...win.appContext, appId: win.appId }}
                  on:authexpired={() => dispatch('authexpired')}
                  on:contextchange={handleWindowAppContextChange}
                />
              </div>
            {:else}
              <div class="app-content">
                <div class="app-header">
                  <span class="app-label">{win.title}</span>
                </div>
              </div>
            {/if}
          </FloatingWindow>
        {/if}
      {/each}
    {/if}

    {#if desktopReady && desktopOverviewOpen}
      <DesktopOverview
        windows={$windows}
        activeWindowId={$activeWindowId}
        {authenticated}
        previewDecisions={overviewPreviewDecisions}
        on:close={handleCloseDesktopOverview}
        on:focuswindow={handleOverviewFocusWindow}
        on:minimizewindow={handleOverviewMinimizeWindow}
        on:closewindow={handleOverviewCloseWindow}
        on:suspendwindow={handleOverviewSuspendWindow}
        on:suspendbackground={handleOverviewSuspendBackground}
        on:opencomputemonitor={handleOverviewOpenComputeMonitor}
        on:keepactiveonly={handleOverviewKeepActiveOnly}
        on:clearsavedwindows={handleOverviewClearSavedWindows}
      />
    {/if}
  </div>

  {#if authenticated && !desktopReady}
    <div class="boot-console" data-boot-console aria-live="polite" role="status">
      <div class="boot-console-header">
        <span>CHOIR BIOS</span>
        <span>{bootstrapError || 'VM bootstrap'}</span>
      </div>
      <div class="boot-lines" data-boot-lines>
        {#each bootLines as line (line.id)}
          <div class="boot-line" class:warn={line.kind === 'warn'} class:error={line.kind === 'error'} data-boot-line>
            <span class="boot-time">{line.elapsed}</span>
            <span class="boot-message">{line.message}</span>
          </div>
        {/each}
        <div class="boot-line boot-cursor" data-boot-cursor>
          <span class="boot-time">..</span>
          <span class="boot-message">_</span>
        </div>
      </div>
    </div>
  {/if}

  {#if toasts.length > 0}
    <div class="toast-stack" aria-live="polite" aria-atomic="true">
      {#each toasts as toast (toast.id)}
        <div class="toast" class:error={toast.kind === 'error'} role={toast.kind === 'error' ? 'alert' : undefined}>{toast.message}</div>
      {/each}
    </div>
  {/if}

  <!-- Bottom bar -->
  <BottomBar
    {currentUser}
    {authenticated}
    liveStatus={$liveStatus}
    promptDisabled={!desktopReady}
    {promptPlaceholder}
    {promptStatus}
    on:logout={handleLogout}
    on:authrequest={() => requestAuth({ kind: 'sign_in' })}
    on:promptsubmit={handlePromptSubmit}
    on:launchapp={handleLaunchApp}
    on:showoverview={handleShowDesktopOverview}
  />
</div>

<style>
  .desktop {
    display: flex;
    flex-direction: column;
    height: 100dvh;
    min-height: 100dvh;
    background: var(--choir-bg, #0f0f0f);
    overflow: hidden;
  }

  .desktop.desktop-loading {
    visibility: hidden;
  }

  .desktop.desktop-loading :global(.bottom-bar),
  .desktop.desktop-loading :global(.desktop-menu),
  .desktop.desktop-loading .boot-console {
    visibility: visible;
  }

  .desktop.desktop-ready {
    visibility: visible;
  }

  /* Desktop area (window container) — full viewport width, no left rail */
  .desktop-area {
    flex: 1;
    position: relative;
    overflow: hidden;
    height: calc(100dvh - var(--choir-bottom-bar-height, 56px));
    padding-bottom: env(safe-area-inset-bottom, 0px);
  }

  /* Prevent flash of empty desktop while state loads (VAL-SHELL-022) */
  .desktop-area.state-loading {
    visibility: hidden;
  }

  .desktop-area.state-loaded {
    visibility: visible;
  }

  .boot-console {
    position: fixed;
    left: clamp(16px, 6vw, 72px);
    right: clamp(16px, 6vw, 72px);
    bottom: calc(var(--choir-bottom-bar-height, 56px) + 24px);
    max-width: 760px;
    border: 1px solid rgba(148, 163, 184, 0.24);
    border-radius: 8px;
    background: rgba(5, 8, 14, 0.92);
    box-shadow: 0 20px 60px rgba(0, 0, 0, 0.38);
    color: #d1fae5;
    font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
    z-index: 90;
  }

  .boot-console-header {
    display: flex;
    justify-content: space-between;
    gap: 1rem;
    border-bottom: 1px solid rgba(148, 163, 184, 0.18);
    padding: 0.65rem 0.8rem;
    color: #bfdbfe;
    font-size: 0.72rem;
    font-weight: 800;
    text-transform: uppercase;
  }

  .boot-lines {
    display: grid;
    gap: 0.35rem;
    padding: 0.75rem 0.8rem 0.85rem;
  }

  .boot-line {
    display: grid;
    grid-template-columns: 3rem minmax(0, 1fr);
    gap: 0.6rem;
    align-items: baseline;
    min-width: 0;
    color: #bbf7d0;
    font-size: 0.8rem;
    line-height: 1.35;
  }

  .boot-line.warn {
    color: #fde68a;
  }

  .boot-line.error {
    color: #fecaca;
  }

  .boot-time {
    color: #7dd3fc;
    font-size: 0.72rem;
  }

  .boot-message {
    overflow-wrap: anywhere;
  }

  .boot-cursor .boot-message {
    animation: boot-cursor-blink 1s steps(2, start) infinite;
  }

  @keyframes boot-cursor-blink {
    0%, 45% { opacity: 1; }
    46%, 100% { opacity: 0; }
  }

  .desktop-recovery {
    position: absolute;
    left: clamp(14px, 5vw, 56px);
    top: clamp(14px, 5vw, 56px);
    width: min(520px, calc(100vw - 28px));
    display: grid;
    gap: 1rem;
    padding: 1.1rem;
    border: 1px solid rgba(96, 165, 250, 0.34);
    border-radius: 12px;
    background: rgba(8, 13, 24, 0.94);
    box-shadow: 0 28px 70px rgba(0, 0, 0, 0.42), 0 0 0 1px rgba(15, 23, 42, 0.82);
    color: #e5edf9;
    z-index: 85;
  }

  .desktop-recovery h2,
  .desktop-recovery p {
    margin: 0;
  }

  .desktop-recovery h2 {
    margin-top: 0.2rem;
    font-size: clamp(1.2rem, 4vw, 1.55rem);
    letter-spacing: 0;
  }

  .desktop-recovery p {
    color: #aebbd0;
    line-height: 1.45;
  }

  .recovery-kicker {
    color: #93c5fd !important;
    font-size: 0.72rem;
    font-weight: 800;
    letter-spacing: 0.12em;
    text-transform: uppercase;
  }

  .recovery-top-window {
    margin-top: 0.7rem !important;
    color: #dbeafe !important;
  }

  .recovery-status {
    margin-top: 0.7rem !important;
    color: #fecaca !important;
  }

  .recovery-actions {
    display: flex;
    flex-wrap: wrap;
    gap: 0.55rem;
  }

  .recovery-actions button {
    min-height: 40px;
    border: 1px solid rgba(148, 163, 184, 0.28);
    border-radius: 8px;
    background: rgba(15, 23, 42, 0.86);
    color: #dbeafe;
    padding: 0.55rem 0.78rem;
    font: inherit;
    font-size: 0.82rem;
    font-weight: 750;
    cursor: pointer;
  }

  .recovery-actions button:hover {
    border-color: rgba(147, 197, 253, 0.62);
    background: rgba(30, 41, 59, 0.94);
  }

  .recovery-actions button:disabled {
    cursor: wait;
    opacity: 0.58;
  }

  .recovery-actions .recovery-primary {
    border-color: rgba(96, 165, 250, 0.62);
    background: rgba(30, 64, 175, 0.72);
    color: #f8fbff;
  }

  /* App content inside windows */
  .app-content {
    padding: 1rem;
    height: 100%;
    display: flex;
    flex-direction: column;
  }

  .vtext-content {
    padding: 0;
    background: #12131c;
  }

  .terminal-content {
    padding: 0;
    background: #1a1b26;
  }

  .trace-content {
    padding: 0;
    background: #0a0d14;
  }

  .podcast-content {
    padding: 0;
    background: #080d18;
  }

  .settings-content {
    padding: 0;
    background: #171827;
  }

  .compute-monitor-content {
    padding: 0;
    background: #080d18;
  }

  .apps-changes-content {
    padding: 0;
    background: #07111e;
  }

  .suspended-app-content {
    align-items: center;
    justify-content: center;
    background:
      linear-gradient(135deg, rgba(8, 13, 24, 0.96), rgba(15, 23, 42, 0.96));
  }

  .suspended-card {
    max-width: 28rem;
    display: grid;
    gap: 0.65rem;
    border: 1px solid rgba(251, 191, 36, 0.28);
    border-radius: 8px;
    background: rgba(2, 6, 23, 0.68);
    padding: 1rem;
    color: #e5edf9;
  }

  .suspended-card h2,
  .suspended-card p {
    margin: 0;
  }

  .suspended-card h2 {
    font-size: 1.1rem;
    letter-spacing: 0;
  }

  .suspended-card p {
    color: #aebbd0;
    line-height: 1.45;
  }

  .suspended-kicker {
    color: #fde68a !important;
    font-size: 0.7rem;
    font-weight: 850;
    letter-spacing: 0.12em;
    text-transform: uppercase;
  }

  .suspended-card button {
    justify-self: start;
    min-height: 2.35rem;
    border: 1px solid rgba(96, 165, 250, 0.38);
    border-radius: 7px;
    background: rgba(30, 64, 175, 0.46);
    color: #eff6ff;
    cursor: pointer;
    font: inherit;
    font-size: 0.82rem;
    font-weight: 800;
    padding: 0.5rem 0.72rem;
  }

  .toast-stack {
    position: fixed;
    left: 50%;
    bottom: 72px;
    transform: translateX(-50%);
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
    z-index: 1200;
    pointer-events: none;
  }

  .toast {
    background: rgba(17, 24, 39, 0.95);
    color: #edf2ff;
    border: 1px solid rgba(255, 255, 255, 0.12);
    border-radius: 999px;
    padding: 0.6rem 0.95rem;
    font-size: 0.82rem;
    box-shadow: 0 12px 32px rgba(0, 0, 0, 0.25);
  }

  .toast.error {
    background: rgba(69, 10, 10, 0.94);
    border-color: rgba(248, 113, 113, 0.42);
    color: #fee2e2;
  }

  .app-header {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    flex-wrap: wrap;
  }

  .app-label {
    font-size: 0.95rem;
    font-weight: 600;
    color: #c0c0d0;
  }

  @media (max-width: 768px) {
    .boot-console {
      left: 12px;
      right: 12px;
      bottom: calc(var(--choir-bottom-bar-height, 56px) + 12px);
    }

    .boot-console-header {
      font-size: 0.66rem;
      padding: 0.55rem 0.65rem;
    }

    .boot-lines {
      padding: 0.65rem;
    }

    .boot-line {
      grid-template-columns: 2.65rem minmax(0, 1fr);
      font-size: 0.72rem;
    }
  }
</style>
