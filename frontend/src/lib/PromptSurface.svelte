<script lang="ts">
  import { createEventDispatcher, onDestroy, onMount, tick } from 'svelte';
  import TetraMark from './TetraMark.svelte';
  import DeskSheet from './DeskSheet.svelte';
  import {
    activeWindowId,
    openWindows,
    restoreWindow,
    focusWindow,
    DESK_APPS,
    liveStatus as desktopLiveStatus,
  } from './stores/desktop.js';
  import { addLiveEventListener, liveEventKind, liveEventPayload } from './live-events.js';

  export let currentUser: any = null;
  export let authenticated = false;
  export let promptDisabled = false;
  export let promptPlaceholder = '';
  export let promptStatus = '';
  export let placement: 'top' | 'bottom' = 'bottom';

  const dispatch = createEventDispatcher();
  let promptValue = '';
  let promptInputEl: HTMLTextAreaElement | null = null;
  let surfaceEl: HTMLDivElement | null = null;
  let resizeObserver: ResizeObserver | null = null;
  let sheetOpen = false;
  let mobileSwitcherOpen = false;
  let promptFocused = false;
  let chyronItems: Array<{ id: string; text: string }> = [];
  let removeLiveListener = () => {};

  const deskApps = DESK_APPS;
  const publicTicker = [
  ];

  $: normalizedPlacement = placement === 'top' ? 'top' : 'bottom';
  $: online = $desktopLiveStatus === 'connected' || !authenticated;
  $: onlineLabel = authenticated
    ? ($desktopLiveStatus === 'connected' ? 'Online' : $desktopLiveStatus === 'connecting' ? 'Connecting' : $desktopLiveStatus === 'error' ? 'Connection error' : 'Offline')
    : 'Preview';
  $: chyronTickerItems = chyronItems.length > 0 ? [...chyronItems, ...chyronItems] : [];

  function publishSurfaceMetrics() {
    if (!surfaceEl || typeof document === 'undefined') return;
    const size = surfaceEl.offsetHeight || 64;
    const root = document.documentElement;
    root.style.setProperty('--choir-prompt-surface-size', `${size}px`);
    root.style.setProperty('--choir-prompt-surface-top-offset', normalizedPlacement === 'top' ? `${size}px` : '0px');
    root.style.setProperty('--choir-prompt-surface-bottom-offset', normalizedPlacement === 'bottom' ? `${size}px` : '0px');
    root.dataset.promptSurfacePlacement = normalizedPlacement;
  }

  function handlePromptKeydown(event: KeyboardEvent) {
    if (promptDisabled) return;
    if (event.key === 'Enter' && !event.shiftKey && promptValue.trim()) {
      event.preventDefault();
      dispatch('promptsubmit', { text: promptValue.trim() });
      promptValue = '';
      tick().then(resizePromptInput);
    } else if (event.key === 'Escape') {
      (event.currentTarget as HTMLTextAreaElement)?.blur();
    }
  }

  function resizePromptInput() {
    if (!promptInputEl) return;
    const lineHeight = Number.parseFloat(getComputedStyle(promptInputEl).lineHeight) || 22;
    const compact = window.innerWidth <= 768;
    const collapsedHeight = compact ? Math.max(54, Math.ceil(lineHeight + 24)) : Math.max(68, Math.ceil(lineHeight + 34));
    const maxHeight = compact ? Math.min(108, Math.max(64, window.innerHeight * 0.18)) : Math.min(128, Math.max(72, window.innerHeight * 0.22));
    promptInputEl.style.height = `${collapsedHeight}px`;
    const nextHeight = promptValue.trim() ? Math.min(promptInputEl.scrollHeight, maxHeight) : collapsedHeight;
    promptInputEl.style.height = `${Math.max(collapsedHeight, nextHeight)}px`;
    promptInputEl.style.overflowY = promptInputEl.scrollHeight > maxHeight ? 'auto' : 'hidden';
    tick().then(publishSurfaceMetrics);
  }

  function openWindowFromTray(win: any) {
    if (win.mode === 'minimized') restoreWindow(win.windowId);
    else focusWindow(win.windowId);
    sheetOpen = false;
    mobileSwitcherOpen = false;
  }

  function launchApp(app: any) {
    dispatch('launchapp', {
      appId: app.id,
      appName: app.name,
      icon: app.icon,
      appContext: app.id === 'podcast' ? { appHint: 'podcast', windowTitle: 'Podcast' } : {},
    });
    sheetOpen = false;
    mobileSwitcherOpen = false;
  }

  function isMobileViewport() {
    return typeof window !== 'undefined' && window.matchMedia('(max-width: 768px)').matches;
  }

  function handleDeskButtonClick() {
    const nextSheetOpen = !sheetOpen;
    sheetOpen = nextSheetOpen;
    mobileSwitcherOpen = nextSheetOpen && isMobileViewport() && $openWindows.length > 0;
    tick().then(publishSurfaceMetrics);
  }

  function summarizeLiveEvent(message: any) {
    const kind = liveEventKind(message);
    const payload = liveEventPayload(message);
    const agent = String(message?.agent_id || payload.agent_id || payload.from || payload.role || 'agent').split(':').pop();
    if (kind === 'tool.invoked') return `${agent} called ${payload.tool || 'tool'}`;
    if (kind === 'tool.result') return `${agent} ${payload.is_error ? 'hit a tool error' : 'received tool output'}`;
    if (kind === 'channel.message') return `${agent}: ${String(payload.content || '').replace(/\s+/g, ' ').trim().slice(0, 120)}`;
    if (kind === 'loop.completed') return `${agent} completed a run`;
    if (kind === 'loop.failed') return `${agent} reported a blocker`;
    if (kind === 'vtext.document_revision.created') return 'VText created a new revision';
    return '';
  }

  function pushTicker(text: string) {
    if (chyronItems.some((item) => item.text === text)) return;
    chyronItems = [
      ...chyronItems,
      { id: `${Date.now()}-${Math.random().toString(16).slice(2)}`, text },
    ].slice(-4);
  }

  function handleLiveEvent(message: any) {
    const kind = liveEventKind(message);
    if (![
      'tool.invoked',
      'tool.result',
      'channel.message',
      'loop.started',
      'loop.completed',
      'loop.failed',
      'vtext.document_revision.created',
    ].includes(kind)) return;
    const text = summarizeLiveEvent(message);
    if (text) pushTicker(text);
  }

  function startPublicTicker() {
    if (authenticated || chyronItems.length > 0) return;
    chyronItems = publicTicker.map((text, index) => ({ id: `public-${index}`, text }));
  }

  onMount(() => {
    publishSurfaceMetrics();
    resizePromptInput();
    startPublicTicker();
    removeLiveListener = addLiveEventListener(handleLiveEvent);
    if (typeof ResizeObserver !== 'undefined' && surfaceEl) {
      resizeObserver = new ResizeObserver(publishSurfaceMetrics);
      resizeObserver.observe(surfaceEl);
    }
    window.addEventListener('resize', resizePromptInput);
  });

  $: if (authenticated && chyronItems.some((item) => item.id.startsWith('public-'))) chyronItems = [];
  $: if ($openWindows.length === 0 && mobileSwitcherOpen) mobileSwitcherOpen = false;
  $: if (!sheetOpen && mobileSwitcherOpen) mobileSwitcherOpen = false;

  onDestroy(() => {
    removeLiveListener();
    resizeObserver?.disconnect();
    window.removeEventListener('resize', resizePromptInput);
  });
</script>

{#if sheetOpen}
  <DeskSheet
    {normalizedPlacement}
    {deskApps}
    openWindows={$openWindows}
    {authenticated}
    {currentUser}
    on:close={() => (sheetOpen = false)}
    on:showoverview={() => { sheetOpen = false; dispatch('showoverview'); }}
    on:showdesktop={() => { sheetOpen = false; dispatch('showdesktop'); }}
    on:launchapp={(event) => launchApp(event.detail.app)}
    on:authrequest={() => dispatch('authrequest')}
    on:logout={() => dispatch('logout')}
  />
{/if}

<div
  class="prompt-surface placement-{normalizedPlacement}"
  class:sheet-open={sheetOpen}
  class:mobile-switcher-open={mobileSwitcherOpen}
  data-prompt-surface
  data-placement={normalizedPlacement}
  data-desk-sheet-open={sheetOpen ? 'true' : 'false'}
  data-mobile-switcher-open={mobileSwitcherOpen ? 'true' : 'false'}
  bind:this={surfaceEl}
>
  <button
    class="desk-mark-button"
    data-desk-menu-button
    type="button"
    aria-label="Open app switcher or Desk, {$openWindows.length} open windows"
    aria-expanded={sheetOpen || mobileSwitcherOpen ? 'true' : 'false'}
    on:click={handleDeskButtonClick}
  >
    <TetraMark />
    {#if $openWindows.length > 0}<span class="window-count" data-window-count>{$openWindows.length}</span>{/if}
  </button>

  <div class="window-tray" data-window-tray>
    {#each $openWindows as win (win.windowId)}
      <button
        class:active={win.windowId === $activeWindowId}
        class:minimized={win.mode === 'minimized'}
        class="window-tray-item"
        data-window-tray-item
        data-window-mode={win.mode}
        data-window-tray-item-active={win.windowId === $activeWindowId ? 'true' : 'false'}
        data-window-id={win.windowId}
        title={win.title}
        on:click={() => openWindowFromTray(win)}
      >
        <span>{win.icon || '□'}</span><small>{win.title}</small>
      </button>
    {/each}
  </div>

  <div class="command-field" data-command-field>
    {#if mobileSwitcherOpen}
      <div class="mobile-app-switcher" data-mobile-app-switcher aria-label="Open apps">
        {#each $openWindows as win (win.windowId)}
          <button
            class:active={win.windowId === $activeWindowId}
            class="mobile-app-switcher-item"
            type="button"
            title={win.title}
            aria-label={`Switch to ${win.title}`}
            on:click={() => openWindowFromTray(win)}
          >
            <span>{win.icon || '□'}</span>
          </button>
        {/each}
      </div>
    {:else if chyronTickerItems.length > 0 && !sheetOpen}
      <div class:focused={promptFocused} class="agent-chyron" data-agent-chyron data-prompt-chyron aria-live="polite">
        <div>{#each chyronTickerItems as item, index (`${item.id}-${index}`)}<span>{item.text}</span>{/each}</div>
      </div>
    {/if}
    {#if !mobileSwitcherOpen}
      {#if promptStatus}<div class="prompt-status" data-prompt-status>{promptStatus}</div>{/if}
      <textarea
        bind:this={promptInputEl}
        bind:value={promptValue}
        data-prompt-input
        rows="1"
        placeholder={promptPlaceholder}
        disabled={promptDisabled}
        aria-label="Command prompt"
        on:keydown={handlePromptKeydown}
        on:input={resizePromptInput}
        on:focus={() => (promptFocused = true)}
        on:blur={() => (promptFocused = false)}
      />
    {/if}
  </div>

  <button class="voice-button" data-agent-audio-button type="button" aria-label="Agent audio input">
    <span aria-hidden="true">▥</span>
  </button>
  <span class:online class="online-indicator" data-online-indicator data-live-status={$desktopLiveStatus} aria-label={onlineLabel} title={onlineLabel}></span>
</div>

<style>
  .prompt-surface {
    position: fixed;
    left: max(12px, env(safe-area-inset-left));
    right: max(12px, env(safe-area-inset-right));
    z-index: 10000;
    display: grid;
    grid-template-columns: auto minmax(0, max-content) minmax(14rem, 1fr) auto auto;
    align-items: center;
    gap: 0.62rem;
    padding: 0.42rem 0.62rem;
    border-radius: var(--choir-radius-pill);
    color: var(--choir-fg);
    background: var(--choir-prompt-surface-bg);
    box-shadow: var(--choir-shadow-floating), var(--choir-shadow-glow);
    backdrop-filter: blur(var(--choir-blur));
  }

  .prompt-surface.placement-bottom {
    bottom: max(10px, env(safe-area-inset-bottom));
  }

  .prompt-surface.placement-top {
    top: max(10px, env(safe-area-inset-top));
  }

  .desk-mark-button,
  .voice-button {
    position: relative;
    display: grid;
    place-items: center;
    width: 2.45rem;
    height: 2.45rem;
    border: 0;
    border-radius: var(--choir-radius-control);
    background: var(--choir-control-bg);
    color: var(--choir-tetramark-color);
    box-shadow: var(--choir-control-shadow);
    cursor: pointer;
  }

  .desk-mark-button :global(svg),
  .voice-button span {
    display: block;
    width: 1.42rem;
    height: 1.42rem;
    line-height: 1;
  }

  .voice-button span {
    display: grid;
    place-items: center;
    font-size: 1.03rem;
  }

  .desk-mark-button:hover,
  .desk-mark-button:focus-visible {
    box-shadow: var(--choir-control-shadow), var(--choir-shadow-glow);
  }

  .window-count {
    position: absolute;
    top: -0.35rem;
    right: -0.35rem;
    min-width: 1.15rem;
    height: 1.15rem;
    display: grid;
    place-items: center;
    border-radius: var(--choir-radius-control-sm);
    background: var(--choir-accent);
    color: var(--choir-on-accent);
    font-size: 0.66rem;
    font-weight: 850;
  }

  .window-tray {
    display: flex;
    gap: 0.35rem;
    min-width: 0;
    max-width: min(30vw, 24rem);
    overflow-x: auto;
    scrollbar-width: none;
  }

  .window-tray::-webkit-scrollbar {
    display: none;
  }

  .window-tray-item {
    min-width: 0;
    max-width: 9rem;
    border: 0;
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
    border-radius: var(--choir-radius-control-sm);
    background: var(--choir-panel-soft);
    color: var(--choir-muted);
    padding: 0.26rem 0.5rem;
    min-height: 1.95rem;
    box-shadow: 0 8px 18px rgba(0, 0, 0, 0.18);
    cursor: pointer;
  }

  .window-tray-item > span {
    flex: 0 0 1.35rem;
    display: inline-grid;
    place-items: center;
    width: 1.35rem;
    height: 1.35rem;
    font-size: 1.08rem;
    line-height: 1;
  }

  .window-tray-item.active {
    color: var(--choir-fg);
    background: var(--choir-selected);
    box-shadow: 0 10px 24px color-mix(in srgb, var(--choir-accent) 18%, transparent);
  }

  .window-tray-item small {
    display: block;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    line-height: 1.05;
  }

  .command-field {
    position: relative;
    min-width: 0;
    display: grid;
    align-items: end;
    overflow: hidden;
    border-radius: var(--choir-radius-pill);
    background:
      linear-gradient(180deg, color-mix(in srgb, var(--choir-panel-soft) 50%, transparent), transparent 58%),
      var(--choir-input-bg);
    min-height: 3.05rem;
    height: 3.05rem;
    box-shadow:
      inset 0 14px 28px rgba(255, 255, 255, 0.018),
      0 12px 30px rgba(0, 0, 0, 0.22);
  }

  .command-field textarea {
    position: relative;
    z-index: 2;
    width: 100%;
    box-sizing: border-box;
    min-height: 3.05rem;
    height: 100%;
    border: 0;
    outline: 0;
    resize: none;
    color: var(--choir-fg);
    background: transparent;
    font: inherit;
    font-size: 1rem;
    line-height: 1.35;
    padding: 0.66rem 0.95rem;
  }

  .mobile-app-switcher {
    position: relative;
    z-index: 3;
    display: flex;
    align-items: center;
    gap: 0.45rem;
    box-sizing: border-box;
    height: 100%;
    min-height: 0;
    overflow-x: auto;
    padding: 0.335rem 0.45rem;
    scrollbar-width: none;
  }

  .mobile-app-switcher::-webkit-scrollbar {
    display: none;
  }

  .mobile-app-switcher-item {
    flex: 0 0 auto;
    display: grid;
    place-items: center;
    width: 2.38rem;
    height: 2.38rem;
    border: 0;
    border-radius: var(--choir-radius-control-sm);
    background: var(--choir-control-bg);
    color: var(--choir-fg);
    box-shadow: var(--choir-control-shadow);
  }

  .mobile-app-switcher-item.active {
    background: var(--choir-selected);
    box-shadow: 0 0 26px color-mix(in srgb, var(--choir-accent) 20%, transparent);
  }

  .mobile-app-switcher-item span {
    display: grid;
    place-items: center;
    width: 1.48rem;
    height: 1.48rem;
    font-size: 1.16rem;
    line-height: 1;
  }

  .mobile-app-switcher-item :global(svg) {
    width: 1.32rem;
    height: 1.32rem;
  }

  .prompt-status {
    position: absolute;
    right: 1rem;
    top: 0.2rem;
    z-index: 3;
    color: var(--choir-accent-2);
    font-size: 0.72rem;
    font-weight: 800;
  }

  .agent-chyron {
    position: absolute;
    inset: 0;
    z-index: 1;
    display: flex;
    align-items: center;
    overflow: hidden;
    opacity: 0.24;
    pointer-events: none;
    mask-image: linear-gradient(90deg, transparent, black 8%, black 92%, transparent);
  }

  .agent-chyron.focused {
    opacity: 0.08;
  }

  .agent-chyron > div {
    display: inline-flex;
    gap: 2rem;
    min-width: max-content;
    animation: choir-chyron 38s linear infinite;
  }

  .agent-chyron span {
    white-space: nowrap;
    color: var(--choir-muted);
  }

  @keyframes choir-chyron {
    from { transform: translateX(-50%); }
    to { transform: translateX(0); }
  }

  .online-indicator {
    align-self: center;
    width: 0.78rem;
    height: 0.78rem;
    border-radius: var(--choir-radius-control-sm);
    background: var(--choir-muted);
    box-shadow: 0 0 18px color-mix(in srgb, var(--choir-muted) 26%, transparent);
  }

  .online-indicator.online {
    background: var(--choir-success);
    box-shadow: 0 0 18px color-mix(in srgb, var(--choir-success) 45%, transparent);
  }

  @media (max-width: 768px) {
    .prompt-surface {
      left: 8px;
      right: 8px;
      grid-template-columns: auto minmax(0, 1fr) auto auto;
      gap: 0.45rem;
      padding: 0.4rem 0.48rem;
    }

    .window-tray {
      display: none;
    }

    .voice-button {
      display: none;
    }

    .command-field,
    .command-field textarea {
      min-height: 2.7rem;
    }

    .command-field {
      height: 2.7rem;
    }

    .mobile-app-switcher {
      min-height: 0;
      padding-block: 0.28rem;
    }

    .command-field textarea {
      padding-block: 0.46rem;
    }
  }
</style>
