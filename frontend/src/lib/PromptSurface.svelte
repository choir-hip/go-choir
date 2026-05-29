<script lang="ts">
  import { createEventDispatcher, onDestroy, onMount, tick } from 'svelte';
  import TetraMark from './TetraMark.svelte';
  import DeskSheet from './DeskSheet.svelte';
  import {
    activeWindowId,
    openWindows,
    restoreWindow,
    focusWindow,
    APP_REGISTRY,
    liveStatus as desktopLiveStatus,
  } from './stores/desktop.js';
  import { addLiveEventListener, liveEventKind, liveEventPayload } from './live-events.js';

  export let currentUser: any = null;
  export let authenticated = false;
  export let promptDisabled = false;
  export let promptPlaceholder = 'Ask anything...';
  export let promptStatus = '';
  export let placement: 'top' | 'bottom' = 'bottom';

  const dispatch = createEventDispatcher();
  let promptValue = '';
  let promptInputEl: HTMLTextAreaElement | null = null;
  let surfaceEl: HTMLDivElement | null = null;
  let resizeObserver: ResizeObserver | null = null;
  let sheetOpen = false;
  let promptFocused = false;
  let chyronItems: Array<{ id: string; text: string }> = [];
  let publicTickerTimer: number | null = null;
  let removeLiveListener = () => {};

  const launcherAppIds = [
    'files', 'browser', 'email', 'compute-monitor', 'vtext', 'trace', 'podcast',
    'image', 'audio', 'video', 'pdf', 'epub', 'features', 'terminal', 'settings',
  ];
  const deskApps = launcherAppIds.map((id) => APP_REGISTRY.find((app) => app.id === id)).filter(Boolean);
  const publicTicker = [
    'Preview mode: local edits stay in this browser until you sign in.',
    'Trace is showing demo trajectories; private evidence loads after sign-in.',
    'Save, send, import, activate, and provider-spend actions request auth at the moment of action.',
    'Node A design lab: review the shell without touching choir.news.',
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
    const collapsedHeight = Math.max(44, Math.ceil(lineHeight + 18));
    const maxHeight = Math.min(128, Math.max(72, window.innerHeight * 0.22));
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
  }

  function launchApp(app: any) {
    dispatch('launchapp', {
      appId: app.id,
      appName: app.name,
      icon: app.icon,
      appContext: app.id === 'podcast' ? { appHint: 'podcast', windowTitle: 'Podcast' } : {},
    });
    sheetOpen = false;
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
    chyronItems = [
      ...chyronItems,
      { id: `${Date.now()}-${Math.random().toString(16).slice(2)}`, text },
    ].slice(-6);
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
    if (authenticated || publicTickerTimer) return;
    publicTicker.forEach((text, index) => {
      window.setTimeout(() => pushTicker(text), index * 450);
    });
    publicTickerTimer = window.setInterval(() => {
      const next = publicTicker[Math.floor(Date.now() / 6000) % publicTicker.length];
      pushTicker(next);
    }, 6000);
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

  $: if (authenticated && publicTickerTimer) {
    window.clearInterval(publicTickerTimer);
    publicTickerTimer = null;
  }

  onDestroy(() => {
    removeLiveListener();
    if (publicTickerTimer) window.clearInterval(publicTickerTimer);
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
  data-prompt-surface
  data-placement={normalizedPlacement}
  data-desk-sheet-open={sheetOpen ? 'true' : 'false'}
  bind:this={surfaceEl}
>
  <button
    class="desk-mark-button"
    data-desk-menu-button
    type="button"
    aria-label="Open Desk, {$openWindows.length} open windows"
    aria-expanded={sheetOpen ? 'true' : 'false'}
    on:click={() => (sheetOpen = !sheetOpen)}
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
    {#if chyronTickerItems.length > 0 && !sheetOpen}
      <div class:focused={promptFocused} class="agent-chyron" data-agent-chyron data-prompt-chyron aria-live="polite">
        <div>{#each chyronTickerItems as item, index (`${item.id}-${index}`)}<span>{item.text}</span>{/each}</div>
      </div>
    {/if}
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
    grid-template-columns: auto minmax(0, max-content) minmax(12rem, 1fr) auto auto;
    align-items: end;
    gap: 0.65rem;
    padding: 0.55rem 0.75rem;
    border: 1px solid var(--choir-border-strong);
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
    width: 2.75rem;
    height: 2.75rem;
    border: 1px solid var(--choir-border-strong);
    border-radius: var(--choir-radius-control);
    background: var(--choir-control-bg);
    color: var(--choir-tetramark-color);
    box-shadow: var(--choir-control-shadow);
    cursor: pointer;
  }

  .desk-mark-button:hover,
  .desk-mark-button:focus-visible {
    border-color: var(--choir-accent);
    box-shadow: var(--choir-shadow-glow);
  }

  .window-count {
    position: absolute;
    top: -0.35rem;
    right: -0.35rem;
    min-width: 1.15rem;
    height: 1.15rem;
    display: grid;
    place-items: center;
    border-radius: 999px;
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
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
    border: 1px solid var(--choir-border);
    border-radius: var(--choir-radius-control-sm);
    background: var(--choir-panel-soft);
    color: var(--choir-muted);
    padding: 0.42rem 0.55rem;
    cursor: pointer;
  }

  .window-tray-item.active {
    color: var(--choir-fg);
    border-color: var(--choir-accent);
    background: var(--choir-selected);
  }

  .window-tray-item small {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .command-field {
    position: relative;
    min-width: 0;
    display: grid;
    align-items: end;
    overflow: hidden;
    border: 1px solid var(--choir-border);
    border-radius: var(--choir-radius-pill);
    background: var(--choir-input-bg);
  }

  .command-field textarea {
    position: relative;
    z-index: 2;
    width: 100%;
    min-height: 2.75rem;
    border: 0;
    outline: 0;
    resize: none;
    color: var(--choir-fg);
    background: transparent;
    font: inherit;
    font-size: 1rem;
    line-height: 1.35;
    padding: 0.72rem 1rem;
  }

  .command-field textarea::placeholder {
    color: var(--choir-muted);
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
    opacity: 0.3;
    pointer-events: none;
    mask-image: linear-gradient(90deg, transparent, black 8%, black 92%, transparent);
  }

  .agent-chyron.focused {
    opacity: 0.08;
  }

  .agent-chyron > div {
    display: inline-flex;
    gap: 1.6rem;
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
    border-radius: 999px;
    background: var(--choir-muted);
    box-shadow: 0 0 0 4px color-mix(in srgb, var(--choir-muted) 16%, transparent);
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
      padding-inline: 0.55rem;
    }

    .window-tray {
      display: none;
    }

    .voice-button {
      display: none;
    }
  }
</style>
