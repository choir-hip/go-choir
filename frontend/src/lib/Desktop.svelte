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
  import { fetchWithRenewal, AuthRequiredError, renewSession } from './auth.js';
  import { submitConductorPrompt, waitForConductorDecision } from './conductor.js';
  import { fetchDesktopState, saveDesktopState } from './desktop.js';
  import { withDesktopSelector } from './desktop-selector.js';
  import FloatingDesktopIcons from './FloatingDesktopIcons.svelte';
  import BottomBar from './BottomBar.svelte';
  import FloatingWindow from './FloatingWindow.svelte';
  import TraceApp from './TraceApp.svelte';
  import VTextEditor from './VTextEditor.svelte';
  import SettingsApp from './SettingsApp.svelte';
  import { openFileDocument } from './vtext.js';
  import FileBrowser from './FileBrowser.svelte';
  import BrowserApp from './BrowserApp.svelte';
  import CandidateDesktopViewer from './CandidateDesktopViewer.svelte';
  import TerminalApp from './TerminalApp.svelte';
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
    updateWindowAppContext,
    setWindows,
    setIconPositions,
    getDefaultIconPositions,
    getAppIcon,
  } from './stores/desktop.js';

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
  let refreshing = false;
  let refreshStatus = '';
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
  const MAX_WS_RECONNECT_ATTEMPTS = 5;
  const WS_RECONNECT_BASE_DELAY = 1000;
  let toasts = [];
  let toastCounter = 0;

  // ---- Desktop state persistence ----
  let stateLoaded = false;
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

  $: desktopReady = bootstrapStable && stateLoaded;
  $: promptPlaceholder = desktopReady ? 'Ask anything...' : bootPromptPlaceholder;
  $: if (mounted && authenticated !== lastAuthenticated) {
    lastAuthenticated = authenticated;
    if (authenticated) {
      startAuthenticatedDesktop();
    } else {
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
    refreshing = false;
    refreshStatus = '';
    stateLoaded = true;
    desktopReady = true;
    bootLines = [];
    bootPromptPlaceholder = 'Booting user computer...';
    promptPlaceholder = 'Ask anything...';
    promptStatus = '';
    authenticatedStartupRunning = false;
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
    refreshStatus = '';
    promptStatus = '';
    wsClosedByLogout = false;
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

  async function loadDesktopState() {
    if (!authenticated) {
      stateLoaded = true;
      desktopReady = bootstrapStable && stateLoaded;
      return;
    }
    try {
      const state = await fetchDesktopState();
      if (state) {
        // Restore icon positions
        if (state.icon_positions && Object.keys(state.icon_positions).length > 0) {
          setIconPositions(state.icon_positions);
        }
        // Restore windows
        if (state.windows && state.windows.length > 0) {
          const restoredWindows = state.windows.map((w) => ({
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
          setWindows(restoredWindows, state.active_window_id || '');
        }
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
    if (saveTimer) clearTimeout(saveTimer);
    saveTimer = setTimeout(persistDesktopState, SAVE_DEBOUNCE_MS);
  }

  async function persistDesktopState() {
    if (!authenticated || !stateLoaded) return;
    try {
      let currentWindows;
      let currentActiveId;
      let currentIconPositions;
      windows.subscribe((w) => { currentWindows = w; })();
      activeWindowId.subscribe((id) => { currentActiveId = id; })();
      iconPositions.subscribe((p) => { currentIconPositions = p; })();

      const state = {
        windows: currentWindows
          .filter((w) => w.mode !== 'hidden')
          .map((w) => ({
            window_id: w.windowId,
            app_id: w.appId,
            title: w.title,
            geometry: { x: w.x, y: w.y, width: w.width, height: w.height },
            restored_geometry: w.restoredGeometry,
            mode: w.mode,
            z_index: w.zIndex,
            app_context: w.appContext,
          })),
        active_window_id: currentActiveId || '',
        icon_positions: currentIconPositions,
      };
      await saveDesktopState(state);
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

  async function handleRefresh() {
    if (!authenticated) {
      requestAuth({ kind: 'refresh' });
      return;
    }
    refreshing = true;
    refreshStatus = '';
    bootstrapError = '';
    try {
      const stable = await stabilizeBootstrap();
      if (!stable) {
        refreshStatus = 'Refresh failed';
        return;
      }
      refreshStatus = 'Session renewed';
    } catch (err) {
      if (err instanceof AuthRequiredError) {
        dispatch('authexpired');
        return;
      }
      refreshStatus = 'Refresh failed';
    } finally {
      refreshing = false;
    }
  }

  // ---- Live channel (WebSocket) ----

  function connectLiveChannel() {
    if (!authenticated) return;
    liveStatus.set('connecting');
    try {
      const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
      const wsUrl = withDesktopSelector(`${protocol}//${window.location.host}/api/ws`);
      ws = new WebSocket(wsUrl);
      ws.onopen = () => {
        liveStatus.set('connected');
        wsReconnectAttempt = 0;
      };
      ws.onmessage = () => {};
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
    focusWindow(event.detail.windowId);
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
    openApp('vtext', 'VText', '📝', {
      windowTitle: detail.title || 'Radio Brief',
      initialContent: detail.initialContent || '',
      seedPrompt: detail.seedPrompt || '',
      createInitialVersion: true,
      allowMultiple: true,
      sourceUrl: detail.sourceUrl || '',
      sourceContentId: detail.sourceContentId || '',
      appHint: detail.appHint || '',
      createdFrom: detail.createdFrom || 'content_viewer',
    });
    showToast(detail.toastMessage || 'Opened in VText');
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
    lastAuthenticated = authenticated;
    if (authenticated) {
      startAuthenticatedDesktop();
    } else {
      enterPublicDesktop();
    }

    // Subscribe to store changes for auto-save
    unsubscribeWindows = windows.subscribe(() => {
      if (stateLoaded) scheduleSave();
    });
    unsubscribeActive = activeWindowId.subscribe(() => {
      if (stateLoaded) scheduleSave();
    });
    unsubscribeIconPositions = iconPositions.subscribe(() => {
      if (stateLoaded) scheduleSave();
    });
  });

  onDestroy(() => {
    mounted = false;
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
            on:close={handleWindowClose}
            on:focus={handleWindowFocus}
            on:minimize={handleWindowMinimize}
            on:maximize={handleWindowMaximize}
            on:restore={handleWindowRestore}
            on:move={handleWindowMove}
            on:resize={handleWindowResize}
          >
            {#if win.appId === 'files'}
              <div class="app-content files-content" data-files-app>
                <FileBrowser on:authexpired={() => dispatch('authexpired')} on:opentextfile={handleOpenTextFile} />
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
            {:else if win.appId === 'candidate-desktop'}
              <div class="app-content candidate-desktop-content" data-candidate-desktop-window>
                <CandidateDesktopViewer appContext={win.appContext} on:authexpired={() => dispatch('authexpired')} />
              </div>
            {:else if win.appId === 'terminal'}
              <div class="app-content terminal-content" data-terminal-app>
                <TerminalApp windowId={win.windowId} />
              </div>
            {:else if win.appId === 'settings'}
              <div class="app-content settings-content" data-settings-window>
                <SettingsApp
                  {currentUser}
                  on:authexpired={() => dispatch('authexpired')}
                  on:resetdesktop={handleResetDesktop}
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
              <div class="app-content media-content" data-image-window>
                <ImageApp appContext={{ ...win.appContext, appId: win.appId }} on:authexpired={() => dispatch('authexpired')} />
              </div>
            {:else if win.appId === 'audio'}
              <div class="app-content media-content" data-audio-window>
                <AudioApp appContext={{ ...win.appContext, appId: win.appId }} on:authexpired={() => dispatch('authexpired')} />
              </div>
            {:else if win.appId === 'video'}
              <div class="app-content media-content" data-video-window>
                <VideoApp appContext={{ ...win.appContext, appId: win.appId }} on:authexpired={() => dispatch('authexpired')} />
              </div>
            {:else if win.appId === 'pdf'}
              <div class="app-content media-content" data-pdf-window>
                <PdfApp appContext={{ ...win.appContext, appId: win.appId }} on:authexpired={() => dispatch('authexpired')} />
              </div>
            {:else if win.appId === 'epub'}
              <div class="app-content media-content" data-epub-window>
                <EpubApp appContext={{ ...win.appContext, appId: win.appId }} on:authexpired={() => dispatch('authexpired')} />
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

  .candidate-desktop-content {
    padding: 0;
    background: #0d1117;
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
