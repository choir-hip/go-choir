<!--
  BottomBar — fixed bottom bar for the ChoirOS desktop.

  Contains:
    - Left: Show Desktop button + desktop/account menu + minimized window indicators
    - Center: prompt bar input with "Ask anything..." placeholder
    - Right: quiet connection status

  Data attributes for test targeting:
    data-bottom-bar         — root bar container
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
    minimizedWindows,
    restoreWindow,
    focusWindow,
    toggleShowDesktop,
    APP_REGISTRY,
  } from './stores/desktop.js';

  export let currentUser = null;
  export let authenticated = false;
  export let liveStatus = 'disconnected';
  export let promptDisabled = false;
  export let promptPlaceholder = 'Ask anything...';
  export let promptStatus = '';

  const dispatch = createEventDispatcher();

  let promptValue = '';
  let promptInputEl = null;
  let bottomBarEl = null;
  let bottomBarResizeObserver = null;
  let menuOpen = false;

  const startApps = APP_REGISTRY.filter((app) =>
    ['files', 'browser', 'candidate-desktop', 'terminal', 'settings', 'vtext', 'trace', 'podcast'].includes(app.id)
  );

  function handleRestore(windowId) {
    restoreWindow(windowId);
  }

  function handleStartButton() {
    menuOpen = !menuOpen;
  }

  function handleShowDesktop() {
    toggleShowDesktop();
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

  function resizePromptInput() {
    if (!promptInputEl) return;
    promptInputEl.style.height = 'auto';
    const maxHeight = Math.min(128, Math.max(72, window.innerHeight * 0.22));
    promptInputEl.style.height = `${Math.min(promptInputEl.scrollHeight, maxHeight)}px`;
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

  function getStatusColor() {
    if (liveStatus === 'connected') return '#4ade80';
    if (liveStatus === 'connecting') return '#fbbf24';
    if (liveStatus === 'error') return '#f87171';
    return '#444';
  }

  function getStatusText() {
    if (liveStatus === 'connected') return 'Connected';
    if (liveStatus === 'connecting') return 'Connecting';
    if (liveStatus === 'error') return 'Error';
    return 'Disconnected';
  }

  onMount(() => {
    publishBottomBarHeight();
    resizePromptInput();
    if (typeof ResizeObserver !== 'undefined' && bottomBarEl) {
      bottomBarResizeObserver = new ResizeObserver(publishBottomBarHeight);
      bottomBarResizeObserver.observe(bottomBarEl);
    }
    window.addEventListener('resize', resizePromptInput);
  });

  onDestroy(() => {
    bottomBarResizeObserver?.disconnect();
    window.removeEventListener('resize', resizePromptInput);
  });
</script>

<div class="bottom-bar" data-bottom-bar bind:this={bottomBarEl}>
  <!-- Left section: Start menu + minimized windows -->
  <div class="bar-left">
    <button
      class="show-desktop-btn"
      data-show-desktop-btn
      data-start-button
      on:click={handleStartButton}
      aria-label="Open Start menu"
      title="Open Start menu"
    >
      <span class="show-desktop-icon">⊞</span>
    </button>

    {#if menuOpen}
      <div class="desktop-menu" data-desktop-menu data-start-menu>
        <div class="start-apps" data-start-apps>
          {#each startApps as app}
            <button
              class="start-app"
              data-start-app
              data-start-app-id={app.id}
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

    <!-- Minimized window indicators -->
    <div class="minimized-indicators">
      {#each $minimizedWindows as win (win.windowId)}
        <button
          class="minimized-indicator"
          data-minimized-indicator
          on:click={() => handleRestore(win.windowId)}
          title={win.title}
          aria-label="Restore {win.title}"
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
      data-shell-live-status
      aria-live="polite"
      aria-label="Connection status: {getStatusText()}"
    >
      <span
        class="status-dot"
        style="background: {getStatusColor()}; {liveStatus === 'connecting' ? 'animation: pulse 1.5s infinite;' : ''}"
      ></span>
      <span class="status-text">{getStatusText()}</span>
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

  .bar-left {
    position: relative;
    display: flex;
    align-items: center;
    gap: 4px;
    flex-shrink: 0;
    min-width: 0;
    min-height: 40px;
  }

  .show-desktop-btn {
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

  .show-desktop-btn:focus-visible {
    outline: 2px solid var(--choir-accent, #3b82f6);
    outline-offset: 2px;
  }

  .desktop-menu {
    position: absolute;
    left: 0;
    bottom: calc(100% + 10px);
    min-width: min(21rem, calc(100vw - 24px));
    border: 1px solid rgba(148, 163, 184, 0.18);
    border-radius: var(--choir-radius-lg, 18px);
    background:
      radial-gradient(circle at top left, rgba(59, 130, 246, 0.14), transparent 34%),
      rgba(15, 23, 42, 0.96);
    box-shadow: var(--choir-shadow-soft, 0 18px 48px rgba(0, 0, 0, 0.4));
    padding: 0.8rem;
    z-index: 300;
    backdrop-filter: blur(18px);
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
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .start-app-desc {
    overflow: hidden;
    color: #94a3b8;
    font-size: 0.68rem;
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

  .minimized-indicators {
    display: flex;
    align-items: center;
    gap: 4px;
    overflow-x: auto;
    flex-shrink: 0;
  }

  .minimized-indicator {
    display: flex;
    align-items: center;
    gap: 4px;
    padding: 4px 8px;
    background: rgba(255, 255, 255, 0.05);
    border: 1px solid #333;
    border-radius: 4px;
    cursor: pointer;
    color: #c0c0d0;
    transition: background 0.15s;
    white-space: nowrap;
    flex-shrink: 0;
  }

  .minimized-indicator:hover {
    background: rgba(59, 130, 246, 0.15);
    border-color: rgba(59, 130, 246, 0.3);
  }

  .minimized-indicator:focus-visible {
    outline: 2px solid #3b82f6;
    outline-offset: 2px;
  }

  .indicator-icon {
    font-size: 0.85rem;
  }

  .indicator-name {
    font-size: 0.7rem;
    max-width: 80px;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .bar-center {
    flex: 1;
    min-width: 0;
    display: flex;
    justify-content: center;
    align-items: flex-end;
  }

  .prompt-bar {
    width: 100%;
    max-width: 600px;
    display: grid;
    gap: 0.35rem;
  }

  .prompt-status {
    min-height: 1rem;
    color: #93c5fd;
    font-size: 0.74rem;
    font-weight: 650;
    line-height: 1.2;
    text-align: center;
  }

  .prompt-input {
    width: 100%;
    min-height: 40px;
    max-height: min(8rem, 22dvh);
    padding: 9px 12px;
    background: rgba(255, 255, 255, 0.05);
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
    background: rgba(255, 255, 255, 0.08);
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
      min-width: min(18rem, calc(100vw - 16px));
    }
  }

  /* Responsive: Mobile */
  @media (max-width: 768px) {
    .bottom-bar {
      padding: 6px 8px calc(6px + env(safe-area-inset-bottom, 0px));
      gap: 8px;
    }

    .bar-center {
      flex: 1;
    }

    .prompt-bar {
      max-width: none;
    }

    .bar-right {
      gap: 4px;
    }
  }
</style>
