<!--
  BottomBar — fixed Shelf for the ChoirOS desktop.

  Contains:
    - Left: Desk button + desktop/account menu + open window indicators
    - Center: prompt bar input with "Ask anything..." placeholder
    - Right: quiet connection status

  Data attributes for test targeting:
    data-bottom-bar         — root bar container (legacy selector)
    data-shelf              — root Shelf container
    data-show-desktop-btn   — Show Desktop toggle button
    data-minimized-indicator — minimized window indicator
    data-prompt-input       — prompt text input
    data-bottom-user        — user info area
    data-bottom-logout      — logout button
    data-connection-status  — connection status indicator
-->
<script>
  import { createEventDispatcher, onDestroy, onMount, tick } from 'svelte';
  import {
    activeWindowId,
    openWindows,
    restoreWindow,
    focusWindow,
    toggleShowDesktop,
    APP_REGISTRY,
    liveStatus as desktopLiveStatus,
  } from './stores/desktop.js';
  import { fetchWithRenewal } from './auth.js';
  import { addLiveEventListener, liveEventKind, liveEventPayload } from './live-events.js';

  export let currentUser = null;
  export let authenticated = false;
  export let promptDisabled = false;
  export let promptPlaceholder = 'Ask anything...';
  export let promptStatus = '';

  const dispatch = createEventDispatcher();

  let promptValue = '';
  let promptInputEl = null;
  let bottomBarEl = null;
  let bottomBarResizeObserver = null;
  let menuOpen = false;
  let promptFocused = false;
  let chyronItems = [];
  let featureTransitions = [];
  let featureActionStatus = '';
  let featureActionError = '';
  let dismissedFeatureID = '';
  let featureActing = '';
  let featureRefreshTimer = null;
  let removeLiveListener = () => {};

  const launcherAppIds = [
    'files',
    'browser',
    'compute-monitor',
    'vtext',
    'trace',
    'podcast',
    'image',
    'audio',
    'video',
    'pdf',
    'epub',
    'features',
    'terminal',
    'settings',
  ];
  const startApps = launcherAppIds
    .map((appId) => APP_REGISTRY.find((app) => app.id === appId))
    .filter(Boolean);

  function handleWindowSwitch(win) {
    if (win.mode === 'minimized') {
      restoreWindow(win.windowId);
    } else {
      focusWindow(win.windowId);
    }
    menuOpen = false;
  }

  function handleStartButton() {
    menuOpen = !menuOpen;
    if (menuOpen) {
      void refreshFeatureTransitions();
    }
  }

  function handleShowDesktop() {
    toggleShowDesktop();
    menuOpen = false;
  }

  function handleShowOverview() {
    dispatch('showoverview');
    menuOpen = false;
  }

  function handleLaunchApp(app) {
    dispatch('launchapp', {
      appId: app.id,
      appName: app.name,
      icon: app.icon,
      appContext: app.id === 'podcast'
        ? { appHint: 'podcast', windowTitle: 'Podcast' }
        : {},
    });
    menuOpen = false;
  }

  function latestFeatureTransition(transitions, dismissedID) {
    return (transitions || [])
      .filter((item) => item?.adoption_id && item.adoption_id !== dismissedID)
      .find((item) => ['verified', 'owner_approved', 'adopted', 'rolled_back', 'blocked'].includes(item.status)) || null;
  }

  function canRefreshFeatureTransitions() {
    return authenticated || !!currentUser?.id || !!currentUser?.email;
  }

  function hasRollbackRef(adoption) {
    if (!adoption?.rollback_profile_json) return false;
    try {
      const profile = typeof adoption.rollback_profile_json === 'string'
        ? JSON.parse(adoption.rollback_profile_json)
        : adoption.rollback_profile_json;
      return !!profile?.previous_active_source_ref;
    } catch (_err) {
      return false;
    }
  }

  function featureActionLabel(adoption) {
    if (!adoption) return '';
    if (['verified', 'owner_approved'].includes(adoption.status) && hasRollbackRef(adoption)) return 'Ready to activate';
    if (adoption.status === 'adopted' && hasRollbackRef(adoption)) return 'Active';
    if (adoption.status === 'rolled_back') return 'Rolled back';
    if (adoption.status === 'blocked') return 'Blocked';
    return String(adoption.status || '').replaceAll('_', ' ');
  }

  function featureTransitionRank(status) {
    const ranks = {
      adoption_proposed: 1,
      candidate_applied: 2,
      verifying: 3,
      built: 4,
      blocked: 5,
      verified: 6,
      owner_approved: 7,
      adopted: 8,
      rolled_back: 8,
    };
    return ranks[status] || 0;
  }

  function newerFeatureTransition(first, second) {
    if (!first) return second;
    if (!second) return first;
    const firstRank = featureTransitionRank(first.status);
    const secondRank = featureTransitionRank(second.status);
    if (secondRank > firstRank) return second;
    if (firstRank > secondRank) return first;
    const firstTime = new Date(first.updated_at || first.created_at || 0).getTime();
    const secondTime = new Date(second.updated_at || second.created_at || 0).getTime();
    return secondTime >= firstTime ? second : first;
  }

  function mergeFeatureTransitions(nextAdoptions = []) {
    const merged = new Map();
    for (const adoption of featureTransitions || []) {
      if (adoption?.adoption_id) merged.set(adoption.adoption_id, adoption);
    }
    for (const adoption of nextAdoptions || []) {
      if (!adoption?.adoption_id) continue;
      merged.set(adoption.adoption_id, newerFeatureTransition(merged.get(adoption.adoption_id), adoption));
    }
    featureTransitions = [...merged.values()]
      .sort((first, second) => new Date(second.updated_at || second.created_at || 0) - new Date(first.updated_at || first.created_at || 0));
  }

  async function refreshFeatureTransitions() {
    if (!canRefreshFeatureTransitions()) {
      featureTransitions = [];
      return;
    }
    try {
      const res = await fetchWithRenewal('/api/adoptions?limit=20', { method: 'GET' });
      if (!res.ok) return;
      const body = await res.json().catch(() => ({}));
      mergeFeatureTransitions(Array.isArray(body?.adoptions) ? body.adoptions : []);
    } catch (_err) {
      // Desk remains usable without the feature summary.
    }
  }

  async function runDeskFeatureAction(adoption, action) {
    if (!adoption?.adoption_id) return;
    featureActionStatus = '';
    featureActionError = '';
    featureActing = action;
    try {
      const res = await fetchWithRenewal(`/api/adoptions/${encodeURIComponent(adoption.adoption_id)}/${action}`, {
        method: 'POST',
        headers: action === 'rollback' ? {} : { 'Content-Type': 'application/json' },
        body: action === 'rollback' ? undefined : '{}',
      });
      const body = await res.json().catch(() => ({}));
      if (!res.ok) throw new Error(body?.error || `Feature action failed (${res.status})`);
      featureActionStatus = action === 'promote'
        ? 'Activated. Rollback is available here.'
        : action === 'rollback'
          ? 'Rolled back. You can roll forward from Desk.'
          : 'Rolled forward.';
      await refreshFeatureTransitions();
    } catch (err) {
      featureActionError = err.message || 'Feature action failed';
    } finally {
      featureActing = '';
    }
  }

  function openFeaturesFromDesk() {
    const app = APP_REGISTRY.find((item) => item.id === 'features');
    if (app) handleLaunchApp(app);
  }

  function dismissFeature(adoption) {
    dismissedFeatureID = adoption?.adoption_id || '';
    featureActionStatus = 'Saved for later.';
  }

  function handlePromptKeydown(event) {
    if (promptDisabled) {
      return;
    }

    if (event.key === 'Enter' && !event.shiftKey && promptValue.trim()) {
      event.preventDefault();
      dispatch('promptsubmit', { text: promptValue.trim() });
      promptValue = '';
      tick().then(resizePromptInput);
    } else if (event.key === 'Escape') {
      event.target.blur();
    }
  }

  function chyronSummary(message) {
    const kind = liveEventKind(message);
    const payload = liveEventPayload(message);
    const agent = String(message?.agent_id || payload.agent_id || payload.from || payload.role || 'agent')
      .split(':')
      .pop();
    if (kind === 'tool.invoked') {
      return `${agent} called ${payload.tool || 'tool'}`;
    }
    if (kind === 'tool.result') {
      return `${agent} ${payload.tool || 'tool'} ${payload.is_error ? 'errored' : 'returned'}`;
    }
    if (kind === 'channel.message') {
      const from = String(payload.from || agent || 'agent').split(':').pop();
      const to = String(payload.to_agent_id || '').split(':').pop();
      const content = String(payload.content || '').replace(/\s+/g, ' ').trim();
      return `${from}${to ? ` → ${to}` : ''}: ${content.slice(0, 120)}`;
    }
    if (kind === 'loop.submitted') return `${agent} queued`;
    if (kind === 'loop.started') return `${agent} started`;
    if (kind === 'loop.completed') return `${agent} completed`;
    if (kind === 'loop.failed') return `${agent} failed`;
    if (kind === 'vtext.agent_revision.started') return 'VText revision started';
    if (kind === 'vtext.agent_revision.completed') return 'VText revision completed';
    if (kind === 'vtext.document_revision.created') return `VText wrote ${String(payload.revision_id || 'a new version').slice(0, 8)}`;
    return '';
  }

  function shouldShowInChyron(message) {
    return [
      'tool.invoked',
      'tool.result',
      'channel.message',
      'loop.submitted',
      'loop.started',
      'loop.completed',
      'loop.failed',
      'vtext.agent_revision.started',
      'vtext.agent_revision.completed',
      'vtext.document_revision.created',
      'app_adoption.verified',
      'app_adoption.blocked',
      'app_adoption.promoted',
      'app_adoption.rolled_back',
    ].includes(liveEventKind(message));
  }

  function handleLiveEvent(message) {
    if (liveEventKind(message).startsWith('app_adoption.')) {
      void refreshFeatureTransitions();
    }
    if (!shouldShowInChyron(message)) return;
    const text = chyronSummary(message);
    if (!text) return;
    chyronItems = [
      ...chyronItems,
      {
        id: `${message.stream_seq || Date.now()}-${Math.random().toString(16).slice(2)}`,
        text,
      },
    ].slice(-10);
  }

  function mergeFeatureTransition(adoption) {
    if (!adoption?.adoption_id) return;
    mergeFeatureTransitions([adoption]);
  }

  function handleFeatureTransitionObserved(event) {
    mergeFeatureTransition(event?.detail?.adoption);
    if (canRefreshFeatureTransitions()) {
      void refreshFeatureTransitions();
    }
  }

  function resizePromptInput() {
    if (!promptInputEl) return;
    const lineHeight = Number.parseFloat(getComputedStyle(promptInputEl).lineHeight) || 22;
    const verticalPadding = 18;
    const collapsedHeight = Math.max(44, Math.ceil(lineHeight + verticalPadding));
    const maxHeight = Math.min(128, Math.max(72, window.innerHeight * 0.22));
    promptInputEl.style.height = `${collapsedHeight}px`;
    const desiredHeight = promptValue.trim() ? Math.min(promptInputEl.scrollHeight, maxHeight) : collapsedHeight;
    promptInputEl.style.height = `${Math.max(collapsedHeight, desiredHeight)}px`;
    promptInputEl.style.overflowY = promptInputEl.scrollHeight > maxHeight ? 'auto' : 'hidden';
    tick().then(publishBottomBarHeight);
  }

  function publishBottomBarHeight() {
    if (!bottomBarEl || typeof document === 'undefined') return;
    document.documentElement.style.setProperty('--choir-bottom-bar-height', `${bottomBarEl.offsetHeight}px`);
  }

  function handleLogout() {
    menuOpen = false;
    dispatch('logout');
  }

  function handleAuthRequest() {
    menuOpen = false;
    dispatch('authrequest');
  }

  $: statusColor = (() => {
    if ($desktopLiveStatus === 'connected') return '#4ade80';
    if ($desktopLiveStatus === 'connecting') return '#fbbf24';
    if ($desktopLiveStatus === 'error') return '#f87171';
    return '#444';
  })();

  $: statusText = (() => {
    if ($desktopLiveStatus === 'connected') return 'Connected';
    if ($desktopLiveStatus === 'connecting') return 'Connecting';
    if ($desktopLiveStatus === 'error') return 'Error';
    return 'Disconnected';
  })();

  $: chyronTickerItems = chyronItems.length > 0 ? [...chyronItems, ...chyronItems] : [];
  $: deskFeatureTransition = latestFeatureTransition(featureTransitions, dismissedFeatureID);
  $: deskHasFeatureAction = !!deskFeatureTransition && ['verified', 'owner_approved', 'adopted', 'rolled_back', 'blocked'].includes(deskFeatureTransition.status);

  onMount(() => {
    publishBottomBarHeight();
    resizePromptInput();
    void refreshFeatureTransitions();
    featureRefreshTimer = window.setInterval(() => {
      if (canRefreshFeatureTransitions()) void refreshFeatureTransitions();
    }, 15_000);
    removeLiveListener = addLiveEventListener(handleLiveEvent);
    window.addEventListener('choir-feature-transition-observed', handleFeatureTransitionObserved);
    if (typeof ResizeObserver !== 'undefined' && bottomBarEl) {
      bottomBarResizeObserver = new ResizeObserver(publishBottomBarHeight);
      bottomBarResizeObserver.observe(bottomBarEl);
    }
    window.addEventListener('resize', resizePromptInput);
  });

  onDestroy(() => {
    removeLiveListener();
    if (featureRefreshTimer) window.clearInterval(featureRefreshTimer);
    window.removeEventListener('choir-feature-transition-observed', handleFeatureTransitionObserved);
    bottomBarResizeObserver?.disconnect();
    window.removeEventListener('resize', resizePromptInput);
  });
</script>

<div
  class:menu-open={menuOpen}
  class="bottom-bar"
  data-bottom-bar
  data-shelf
  data-desk-menu-open={menuOpen ? 'true' : 'false'}
  bind:this={bottomBarEl}
>
  <!-- Left section: Desk menu + open windows -->
  <div class="bar-left">
    <button
      class:feature-ready={deskHasFeatureAction}
      class="show-desktop-btn"
      data-show-desktop-btn
      data-start-button
      data-desk-button
      data-desk-feature-ready={deskHasFeatureAction ? 'true' : 'false'}
      on:click={handleStartButton}
      aria-label="Open Desk menu, {$openWindows.length} open windows"
      title="Open Desk menu"
    >
      <span class="show-desktop-icon">⊞</span>
      {#if $openWindows.length > 0}
        <span class="open-window-count" data-shelf-window-count>{$openWindows.length}</span>
      {/if}
    </button>

    {#if menuOpen}
      <div class="desktop-menu" data-desktop-menu data-start-menu data-desk-menu>
        <div class="menu-heading">
          <span>Desk</span>
          <small>{$openWindows.length} open</small>
        </div>
        <button
          class="menu-overview-btn"
          data-desk-overview
          on:click={handleShowOverview}
          aria-label="Open Desktop Overview"
        >
          <span class="overview-icon">▦</span>
          <span>
            <strong>Desktop Overview</strong>
            <small>See and manage open windows</small>
          </span>
        </button>
        {#if deskFeatureTransition}
          <section class="desk-feature" data-desk-feature-controls>
            <div>
              <span class="menu-label">Feature</span>
              <strong>{deskFeatureTransition.app_id || deskFeatureTransition.package_id || 'Feature import'}</strong>
              <small>{featureActionLabel(deskFeatureTransition)}</small>
            </div>
            {#if featureActionStatus}
              <p class="desk-feature-status">{featureActionStatus}</p>
            {/if}
            {#if featureActionError}
              <p class="desk-feature-error" role="alert">{featureActionError}</p>
            {/if}
            <div class="desk-feature-actions">
              <button data-desk-feature-watch on:click={openFeaturesFromDesk}>Watch demo</button>
              <button data-desk-feature-details on:click={openFeaturesFromDesk}>View details</button>
              {#if ['verified', 'owner_approved'].includes(deskFeatureTransition.status) && hasRollbackRef(deskFeatureTransition)}
                <button
                  class="primary"
                  data-desk-feature-activate
                  on:click={() => runDeskFeatureAction(deskFeatureTransition, 'promote')}
                  disabled={!!featureActing}
                >
                  {featureActing === 'promote' ? 'Activating...' : 'Activate'}
                </button>
              {/if}
              {#if deskFeatureTransition.status === 'adopted' && hasRollbackRef(deskFeatureTransition)}
                <button
                  class="danger"
                  data-desk-feature-rollback
                  on:click={() => runDeskFeatureAction(deskFeatureTransition, 'rollback')}
                  disabled={!!featureActing}
                >
                  {featureActing === 'rollback' ? 'Rolling back...' : 'Roll back'}
                </button>
              {/if}
              {#if deskFeatureTransition.status === 'rolled_back'}
                <button
                  class="primary"
                  data-desk-feature-roll-forward
                  on:click={() => runDeskFeatureAction(deskFeatureTransition, 'roll-forward')}
                  disabled={!!featureActing}
                >
                  {featureActing === 'roll-forward' ? 'Rolling forward...' : 'Roll forward'}
                </button>
              {/if}
              <button data-desk-feature-later on:click={() => dismissFeature(deskFeatureTransition)}>Later</button>
            </div>
          </section>
        {/if}
        <div class="menu-label menu-group-label">Apps</div>
        <div class="start-apps" data-start-apps>
          {#each startApps as app}
            <button
              class="start-app"
              data-start-app
              data-start-app-id={app.id}
              data-desk-app-id={app.id}
              on:click={() => handleLaunchApp(app)}
              aria-label={app.name}
            >
              <span class="start-app-icon">{app.icon}</span>
              <span class="start-app-copy">
                <span class="start-app-name">{app.name}</span>
                <span class="start-app-desc">{app.description}</span>
              </span>
            </button>
          {/each}
        </div>
        <button
          class="menu-show-desktop-btn"
          data-start-show-desktop
          on:click={handleShowDesktop}
          aria-label="Show desktop"
        >
          Show desktop
        </button>
        {#if authenticated}
          <div class="menu-section" data-bottom-user data-desktop-user data-shell-user>
            <span class="menu-label">Signed in</span>
            <span class="menu-email">{currentUser?.email || 'unknown'}</span>
          </div>
          <button
            class="menu-logout-btn"
            data-bottom-logout
            data-desktop-logout
            data-shell-logout
            on:click={handleLogout}
            aria-label="Sign out"
          >
            Sign out
          </button>
        {:else}
          <div class="menu-section" data-bottom-user data-desktop-user data-shell-user>
            <span class="menu-label">Public desktop</span>
            <span class="menu-email">Viewing only</span>
          </div>
          <button
            class="menu-login-btn"
            data-bottom-login
            data-shell-login
            on:click={handleAuthRequest}
            aria-label="Sign in"
          >
            Sign in
          </button>
        {/if}
      </div>
    {/if}

    <!-- Window switcher -->
    <div class="window-switcher" data-window-switcher>
      {#each $openWindows as win (win.windowId)}
        <button
          class:active={win.windowId === $activeWindowId}
          class:minimized={win.mode === 'minimized'}
          class="window-indicator"
          data-window-indicator
          data-minimized-indicator={win.mode === 'minimized' ? 'true' : undefined}
          data-window-indicator-active={win.windowId === $activeWindowId ? 'true' : 'false'}
          data-window-id={win.windowId}
          on:click={() => handleWindowSwitch(win)}
          title={win.title}
          aria-label="{win.mode === 'minimized' ? 'Restore' : 'Focus'} {win.title}"
        >
          <span class="indicator-icon">{win.icon || '📱'}</span>
          <span class="indicator-name">{win.title}</span>
        </button>
      {/each}
    </div>
  </div>

  <!-- Center section: prompt bar -->
  <div class="bar-center">
    <div class="prompt-bar">
      {#if chyronTickerItems.length > 0 && !menuOpen}
        <div
          class:prompt-chyron-focused={promptFocused}
          class="prompt-chyron"
          data-shelf-chyron
          data-prompt-chyron
          aria-live="polite"
        >
          <div class="prompt-chyron-track">
            {#each chyronTickerItems as item, index (`${item.id}-${index}`)}
              <span class="prompt-chyron-item">{item.text}</span>
            {/each}
          </div>
        </div>
      {/if}
      {#if promptStatus}
        <div class="prompt-status" data-prompt-status aria-live="polite">{promptStatus}</div>
      {/if}
      <textarea
        class="prompt-input"
        data-prompt-input
        rows="1"
        bind:this={promptInputEl}
        bind:value={promptValue}
        on:keydown={handlePromptKeydown}
        on:input={resizePromptInput}
        on:focus={() => (promptFocused = true)}
        on:blur={() => (promptFocused = false)}
        placeholder={promptPlaceholder}
        aria-label="Prompt input"
        disabled={promptDisabled}
      ></textarea>
    </div>
  </div>

  <!-- Right section: quiet connection status -->
  <div class="bar-right">
    <!-- Connection status dot -->
    <div
      class="connection-status"
      data-connection-status
      data-desktop-live-status
      data-live-status={$desktopLiveStatus}
      data-shell-live-status
      aria-live="polite"
      aria-label="Connection status: {statusText}"
    >
      <span
        class="status-dot"
        style="background: {statusColor}; {$desktopLiveStatus === 'connecting' ? 'animation: pulse 1.5s infinite;' : ''}"
      ></span>
      <span class="status-text">{statusText}</span>
    </div>
  </div>
</div>

<style>
  .bottom-bar {
    position: fixed;
    bottom: 0;
    left: 0;
    right: 0;
    min-height: 56px;
    background: var(--choir-panel-strong, #11111b);
    border-top: 1px solid var(--choir-border, #2a2a3a);
    display: flex;
    align-items: flex-end;
    padding: 6px 12px calc(6px + env(safe-area-inset-bottom, 0px));
    z-index: 100;
    gap: 12px;
  }

  .bottom-bar.menu-open {
    z-index: 10000;
  }

  .bar-left {
    position: relative;
    display: flex;
    align-items: center;
    gap: 4px;
    flex: 0 1 auto;
    min-width: 0;
    min-height: 40px;
  }

  .show-desktop-btn {
    position: relative;
    width: 36px;
    height: 36px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: transparent;
    border: 1px solid #333;
    border-radius: 6px;
    cursor: pointer;
    color: var(--choir-muted, #c0c0d0);
    font-size: 1.1rem;
    flex-shrink: 0;
    transition: background var(--choir-motion-fast, 0.15s), border-color var(--choir-motion-fast, 0.15s);
  }

  .show-desktop-btn:hover {
    background: rgba(255, 255, 255, 0.06);
    border-color: #444;
  }

  .show-desktop-btn.feature-ready {
    border-color: rgba(74, 222, 128, 0.78);
    color: #dcfce7;
    box-shadow: 0 0 0 1px rgba(74, 222, 128, 0.2), 0 0 22px rgba(34, 197, 94, 0.32);
  }

  .show-desktop-btn:focus-visible {
    outline: 2px solid var(--choir-accent, #3b82f6);
    outline-offset: 2px;
  }

  .open-window-count {
    position: absolute;
    right: -5px;
    top: -6px;
    display: grid;
    min-width: 1.1rem;
    height: 1.1rem;
    align-items: center;
    border: 1px solid rgba(15, 23, 42, 0.92);
    border-radius: 999px;
    background: #3b82f6;
    color: white;
    font-size: 0.62rem;
    font-weight: 850;
    line-height: 1;
    padding: 0 0.25rem;
  }

  .desktop-menu {
    position: fixed;
    left: max(12px, env(safe-area-inset-left, 0px));
    bottom: calc(var(--choir-bottom-bar-height, 56px) + 10px);
    min-width: min(21rem, calc(100vw - 24px));
    border: 1px solid rgba(148, 163, 184, 0.18);
    border-radius: var(--choir-radius-lg, 18px);
    background:
      radial-gradient(circle at top left, rgba(59, 130, 246, 0.14), transparent 34%),
      rgba(15, 23, 42, 0.96);
    box-shadow: var(--choir-shadow-soft, 0 18px 48px rgba(0, 0, 0, 0.4));
    padding: 0.8rem;
    z-index: 10001;
    max-height: calc(100dvh - var(--choir-bottom-bar-height, 56px) - 24px - env(safe-area-inset-top, 0px));
    overflow-y: auto;
    backdrop-filter: blur(18px);
  }

  .menu-heading {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    gap: 1rem;
    margin-bottom: 0.55rem;
  }

  .menu-heading span {
    color: #f8fafc;
    font-size: 1rem;
    font-weight: 850;
  }

  .menu-heading small {
    color: #93c5fd;
    font-size: 0.72rem;
    font-weight: 750;
  }

  .menu-overview-btn {
    display: grid;
    grid-template-columns: 2rem minmax(0, 1fr);
    gap: 0.65rem;
    align-items: center;
    width: 100%;
    margin-bottom: 0.65rem;
    border: 1px solid rgba(96, 165, 250, 0.34);
    border-radius: var(--choir-radius-md, 12px);
    background: rgba(37, 99, 235, 0.2);
    color: #dbeafe;
    cursor: pointer;
    padding: 0.62rem;
    text-align: left;
  }

  .menu-overview-btn:hover,
  .menu-overview-btn:focus-visible {
    border-color: rgba(125, 211, 252, 0.52);
    background: rgba(37, 99, 235, 0.32);
  }

  .overview-icon {
    display: grid;
    align-items: center;
    justify-content: center;
    width: 2rem;
    height: 2rem;
    border-radius: 10px;
    background: rgba(125, 211, 252, 0.12);
    color: #bfdbfe;
    font-weight: 850;
  }

  .menu-overview-btn span:last-child {
    display: grid;
    gap: 0.14rem;
    min-width: 0;
  }

  .menu-overview-btn strong,
  .menu-overview-btn small {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .menu-overview-btn strong {
    color: #f8fafc;
    font-size: 0.88rem;
  }

  .menu-overview-btn small {
    color: #bfdbfe;
    font-size: 0.7rem;
  }

  .desk-feature {
    display: grid;
    gap: 0.55rem;
    margin-bottom: 0.72rem;
    border: 1px solid rgba(74, 222, 128, 0.28);
    border-radius: 12px;
    background: rgba(22, 101, 52, 0.18);
    padding: 0.72rem;
  }

  .desk-feature strong,
  .desk-feature small,
  .desk-feature p {
    display: block;
    min-width: 0;
    overflow-wrap: anywhere;
  }

  .desk-feature strong {
    color: #f8fafc;
    font-size: 0.88rem;
  }

  .desk-feature small,
  .desk-feature-status {
    color: #bbf7d0;
    font-size: 0.74rem;
  }

  .desk-feature-error {
    color: #fecaca;
    font-size: 0.74rem;
  }

  .desk-feature-actions {
    display: flex;
    flex-wrap: wrap;
    gap: 0.38rem;
  }

  .desk-feature-actions button {
    border: 1px solid rgba(148, 163, 184, 0.2);
    border-radius: 8px;
    background: rgba(15, 23, 42, 0.76);
    color: #e2e8f0;
    cursor: pointer;
    padding: 0.42rem 0.58rem;
    font-size: 0.72rem;
    font-weight: 820;
  }

  .desk-feature-actions button.primary {
    border-color: rgba(96, 165, 250, 0.48);
    background: rgba(37, 99, 235, 0.48);
    color: white;
  }

  .desk-feature-actions button.danger {
    border-color: rgba(248, 113, 113, 0.42);
    background: rgba(127, 29, 29, 0.32);
    color: #fecaca;
  }

  .menu-group-label {
    margin: 0 0 0.45rem;
  }

  .menu-section {
    display: grid;
    gap: 0.2rem;
    margin: 0.7rem 0 0.65rem;
    min-width: 0;
  }

  .start-apps {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 0.45rem;
  }

  .start-app {
    display: grid;
    grid-template-columns: 2rem minmax(0, 1fr);
    gap: 0.55rem;
    align-items: center;
    min-height: 3.1rem;
    border: 1px solid rgba(148, 163, 184, 0.14);
    border-radius: var(--choir-radius-md, 12px);
    background: rgba(255, 255, 255, 0.045);
    color: var(--choir-fg, #e5eefc);
    cursor: pointer;
    padding: 0.5rem;
    text-align: left;
  }

  .start-app:hover,
  .start-app:focus-visible {
    border-color: rgba(96, 165, 250, 0.45);
    background: rgba(96, 165, 250, 0.12);
  }

  .start-app-icon {
    font-size: 1.25rem;
    text-align: center;
  }

  .start-app-copy {
    display: grid;
    gap: 0.1rem;
    min-width: 0;
  }

  .start-app-name {
    overflow: hidden;
    color: #e2e8f0;
    font-size: 0.84rem;
    font-weight: 800;
    line-height: 1.14;
    overflow-wrap: anywhere;
  }

  .start-app-desc {
    overflow: hidden;
    color: #94a3b8;
    font-size: 0.68rem;
    line-height: 1.18;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .menu-show-desktop-btn {
    width: 100%;
    margin-top: 0.55rem;
    border: 1px solid rgba(148, 163, 184, 0.18);
    border-radius: var(--choir-radius-md, 12px);
    background: rgba(15, 23, 42, 0.68);
    color: #dbeafe;
    cursor: pointer;
    padding: 0.55rem 0.75rem;
    text-align: left;
    font-weight: 760;
  }

  .menu-show-desktop-btn:hover {
    background: rgba(30, 41, 59, 0.9);
  }

  .menu-label {
    color: #94a3b8;
    font-size: 0.72rem;
    font-weight: 750;
    letter-spacing: 0.08em;
    text-transform: uppercase;
  }

  .menu-email {
    color: #e2e8f0;
    font-size: 0.86rem;
    overflow-wrap: anywhere;
  }

  .menu-logout-btn {
    width: 100%;
    border: 1px solid rgba(248, 113, 113, 0.26);
    border-radius: 12px;
    background: rgba(127, 29, 29, 0.22);
    color: #fecaca;
    cursor: pointer;
    padding: 0.62rem 0.75rem;
    text-align: left;
    font-weight: 760;
  }

  .menu-logout-btn:hover {
    background: rgba(127, 29, 29, 0.34);
  }

  .menu-login-btn {
    width: 100%;
    border: 1px solid rgba(96, 165, 250, 0.34);
    border-radius: 12px;
    background: rgba(37, 99, 235, 0.2);
    color: #dbeafe;
    cursor: pointer;
    padding: 0.62rem 0.75rem;
    text-align: left;
    font-weight: 760;
  }

  .menu-login-btn:hover {
    background: rgba(37, 99, 235, 0.32);
  }

  .window-switcher {
    display: flex;
    align-items: center;
    gap: 4px;
    overflow-x: auto;
    flex: 0 1 auto;
    max-width: 100%;
    min-width: 0;
    scrollbar-width: none;
  }

  .window-switcher::-webkit-scrollbar {
    display: none;
  }

  .window-indicator {
    display: flex;
    align-items: center;
    gap: 4px;
    justify-content: flex-start;
    min-width: 0;
    padding: 4px 8px;
    background: rgba(255, 255, 255, 0.05);
    border: 1px solid #333;
    border-radius: 4px;
    cursor: pointer;
    color: #c0c0d0;
    transition: background 0.15s;
    white-space: nowrap;
    flex: 0 1 9rem;
    max-width: 9rem;
  }

  .window-indicator:hover,
  .window-indicator.active {
    background: rgba(59, 130, 246, 0.15);
    border-color: rgba(59, 130, 246, 0.3);
  }

  .window-indicator.minimized {
    opacity: 0.72;
  }

  .window-indicator:focus-visible {
    outline: 2px solid #3b82f6;
    outline-offset: 2px;
  }

  .indicator-icon {
    font-size: 0.85rem;
  }

  .indicator-name {
    font-size: 0.7rem;
    min-width: 0;
    max-width: 6.5rem;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .bar-center {
    flex: 1 1 24rem;
    min-width: 0;
    display: flex;
    justify-content: stretch;
    align-items: flex-end;
  }

  .prompt-bar {
    position: relative;
    width: 100%;
    max-width: none;
    display: grid;
    gap: 0.35rem;
    overflow: hidden;
    border-radius: 20px;
  }

  .prompt-chyron {
    position: absolute;
    inset: 0;
    display: flex;
    align-items: center;
    overflow: hidden;
    pointer-events: none;
    z-index: 1;
    opacity: 0.34;
    mask-image: linear-gradient(90deg, transparent, black 8%, black 92%, transparent);
    transition: opacity 0.16s ease;
  }

  .prompt-chyron.prompt-chyron-focused {
    opacity: 0.085;
  }

  .prompt-chyron-track {
    display: inline-flex;
    min-width: max-content;
    gap: 1.65rem;
    align-items: center;
    white-space: nowrap;
    color: rgba(219, 234, 254, 0.92);
    font-size: 0.78rem;
    font-weight: 720;
    text-transform: none;
    text-shadow: 0 1px 12px rgba(2, 6, 23, 0.9);
    animation: prompt-chyron-flow 38s linear infinite;
    will-change: transform;
  }

  .prompt-chyron-item {
    opacity: 0.96;
  }

  @keyframes prompt-chyron-flow {
    from { transform: translateX(-50%); }
    to { transform: translateX(0); }
  }

  .prompt-status {
    position: relative;
    z-index: 3;
    min-height: 1rem;
    color: #93c5fd;
    font-size: 0.74rem;
    font-weight: 650;
    line-height: 1.2;
    text-align: center;
  }

  .prompt-input {
    position: relative;
    z-index: 2;
    width: 100%;
    height: 44px;
    min-height: 44px;
    max-height: min(8rem, 22dvh);
    padding: 9px 12px;
    background: rgba(255, 255, 255, 0.045);
    border: 1px solid #333;
    border-radius: 20px;
    color: #e0e0e0;
    font: inherit;
    font-size: 16px;
    line-height: 1.35;
    outline: none;
    resize: none;
    overflow-y: hidden;
    transition: border-color 0.15s;
  }

  .prompt-input::placeholder {
    color: #666;
  }

  .prompt-input:focus {
    border-color: #3b82f6;
    background: rgba(15, 23, 42, 0.82);
  }

  .bar-right {
    display: flex;
    align-items: center;
    gap: 10px;
    flex-shrink: 0;
    min-height: 40px;
  }

  .connection-status {
    display: flex;
    align-items: center;
    gap: 6px;
  }

  .status-dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    flex-shrink: 0;
  }

  .status-text {
    position: absolute;
    width: 1px;
    height: 1px;
    padding: 0;
    margin: -1px;
    overflow: hidden;
    clip: rect(0, 0, 0, 0);
    white-space: nowrap;
    border: 0;
  }

  :global(.status-dot-connected) {
    background: #4ade80;
    box-shadow: 0 0 4px rgba(74, 222, 128, 0.5);
  }

  @keyframes pulse {
    0%, 100% { opacity: 1; }
    50% { opacity: 0.4; }
  }

  /* Responsive: Tablet */
  @media (max-width: 1024px) {
    .desktop-menu {
      left: max(8px, env(safe-area-inset-left, 0px));
      min-width: min(21rem, calc(100vw - 16px));
    }
  }

  /* Responsive: Mobile */
  @media (max-width: 768px) {
    .bottom-bar {
      min-height: 52px;
      padding: 5px 8px calc(5px + env(safe-area-inset-bottom, 0px));
      gap: 7px;
    }

    .prompt-input {
      height: 44px;
      min-height: 44px;
      padding: 8px 11px;
    }

    .bar-center {
      flex: 1;
    }

    .bar-left {
      flex: 0 0 auto;
    }

    .prompt-bar {
      max-width: none;
    }

    .bar-right {
      gap: 4px;
    }

    .window-switcher {
      display: none;
    }

    .start-app {
      grid-template-columns: 1.8rem minmax(0, 1fr);
      min-height: 3.35rem;
      gap: 0.45rem;
      padding: 0.46rem;
    }

    .start-app-name {
      font-size: 0.8rem;
    }

    .start-app-desc {
      font-size: 0.64rem;
    }

    .indicator-name {
      max-width: 54px;
    }
  }
</style>
