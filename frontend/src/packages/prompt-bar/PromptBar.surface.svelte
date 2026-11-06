<!--
  The prompt bar: the desk button, the window tray, the command field with
  its focus chip, the voice button and the online dot. A shell surface of the
  prompt slot (lib/shell-slots.ts; plan section 11.1, slice H3, R31).

  Slot ABI, props: authenticated, placement ('top' | 'bottom'), promptDisabled,
  promptPlaceholder, promptStatus, promptRouting, liveStatus, launcherOpen,
  launcherRestoreFocus, target (the focused window's prompt target:
  { windowId, appId, appName, label, focus } where focus is
  { app_agent_id, object_ref, selection? } or null) and route (where the last
  text went: { state: 'routing' | 'routed', label }). Events: promptsubmit
  ({ text, focus, windowId }; focus null sends the text to the conductor),
  windowaction ({ windowId, action }), launcher ({ open, restoreFocus }).
  Everything else is an SDK call. Cmd+K is the host's (core.toggleDesk).

  One chip names what the text goes to: the focused app agent and its object
  ("Texture · Tide notes"), the focused app as a hint when it has no desk to
  take it (the conductor routes), or the conductor. One tap clears a focus to
  the conductor and a second restores it. A pasted link is text like
  any other: it goes to the focused object (owner, 2026-11-06).
-->
<script lang="ts">
  import { createEventDispatcher, onDestroy, onMount, tick } from 'svelte';
  import {
    TETRA_MARK_PATHS,
    TETRA_MARK_VIEWBOX,
    addLiveEventListener,
    contextMenu,
    focus as focusWindow,
    liveEventKind,
    liveEventPayload,
    onWindowEvent,
    restore as restoreWindow,
    windowList,
    type ContextMenuEntry,
    type PromptFocus,
  } from '../../lib/app-sdk';

  type PromptTarget = { windowId: string; appId: string; appName: string; label: string; focus: PromptFocus | null };
  type PromptRoute = { state: 'routing' | 'routed'; label: string };

  export let authenticated = false;
  export let promptDisabled = false;
  export let promptPlaceholder = '';
  export let promptStatus = '';
  export let promptRouting = false;
  export let placement: 'top' | 'bottom' = 'bottom';
  export let liveStatus = 'disconnected';
  export let launcherOpen = false;
  export let launcherRestoreFocus = false;
  export let target: PromptTarget | null = null;
  export let route: PromptRoute | null = null;

  const dispatch = createEventDispatcher();
  let promptValue = '';
  let promptInputEl: HTMLTextAreaElement | null = null;
  let surfaceEl: HTMLDivElement | null = null;
  let deskButtonEl: HTMLButtonElement | null = null;
  let resizeObserver: ResizeObserver | null = null;
  $: sheetOpen = launcherOpen;
  let mobileSwitcherOpen = false;
  let promptFocused = false;
  let chyronItems: Array<{ id: string; text: string }> = [];
  let removeLiveListener = () => {};
  let removeResize = () => {};
  let promptWasDisabled = promptDisabled;
  // The owner's one-tap choice ('conductor' clears the focus, 'focus' keeps
  // it; '' is the default). It belongs to the target in view and to one
  // send: a different target, or the send settling, clears it.
  let choice = '';
  let choiceKey = '';
  let choiceRouting = false;

  const publicTicker: string[] = [];

  $: openWindows = $windowList;
  $: activeWindowId = openWindows.find((win) => win.focused)?.windowId || '';
  $: normalizedPlacement = placement === 'top' ? 'top' : 'bottom';
  $: online = liveStatus === 'connected' || !authenticated;
  $: onlineLabel = authenticated
    ? (liveStatus === 'connected' ? 'Online' : liveStatus === 'connecting' ? 'Connecting' : liveStatus === 'error' ? 'Connection error' : 'Offline')
    : 'Preview';
  $: chyronTickerItems = chyronItems.length > 0 ? [...chyronItems, ...chyronItems] : [];
  $: targetKey = target ? `${target.windowId}|${target.focus?.app_agent_id || ''}|${target.focus?.object_ref || ''}` : '';
  $: if (targetKey !== choiceKey) {
    choiceKey = targetKey;
    choice = '';
  }
  $: if (promptRouting) {
    choiceRouting = true;
  } else if (choiceRouting) {
    choiceRouting = false;
    choice = '';
  }
  // What the text goes to: 'focus' (the focused app agent), 'hint' (the
  // focused app has no desk to take it: the conductor routes, the chip names
  // the app) or 'conductor'.
  $: mode = !target
    ? 'conductor'
    : choice === 'conductor'
      ? 'conductor'
      : target.focus
        ? 'focus'
        : 'hint';
  $: chipName = mode === 'conductor' ? 'Conductor' : target?.appName || '';
  $: chipLabel = mode === 'conductor' ? '' : target?.label || '';
  $: chipAria = mode === 'focus'
    ? `To ${target?.appName}${chipLabel ? `: ${chipLabel}` : ''}. Tap to send to the conductor.`
    : mode === 'hint'
      ? `${target?.appName} is focused; the conductor routes. Tap to clear.`
      : target
        ? `To the conductor. Tap to send to ${target.appName}.`
        : 'To the conductor, which routes the text.';

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
      // With a target in view, one send at a time: a second send while the
      // first routes would read a target the first is still changing. The
      // text stays in the field.
      if (promptRouting && target) return;
      const focus = mode === 'focus' && target?.focus ? target.focus : null;
      dispatch('promptsubmit', { text: promptValue.trim(), focus, windowId: focus ? target?.windowId || '' : '' });
      promptValue = '';
      tick().then(resizePromptInput);
    } else if (event.key === 'Escape') {
      (event.currentTarget as HTMLTextAreaElement)?.blur();
    }
  }

  // One tap clears the focus to the conductor, a second restores it; the
  // focus goes back to the prompt, so the owner keeps typing.
  function toggleTarget() {
    if (!target) return;
    choice = mode === 'conductor' ? 'focus' : 'conductor';
    promptInputEl?.focus();
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
    dispatch('launcher', { open: false });
    mobileSwitcherOpen = false;
  }

  function trayMenuItems(win: any): ContextMenuEntry[] {
    const act = (action: string) => () => dispatch('windowaction', { windowId: win.windowId, action });
    const minimized = win.mode === 'minimized';
    const maximized = win.mode === 'maximized';
    return [
      { id: 'show', label: minimized ? 'Restore' : 'Bring to front', onSelect: act(minimized ? 'restore' : 'focus') },
      { id: 'minimize', label: 'Minimize', disabled: minimized, onSelect: act('minimize') },
      maximized
        ? { id: 'restore', label: 'Restore size', onSelect: act('restore') }
        : { id: 'maximize', label: 'Maximize', onSelect: act('maximize') },
      { separator: true },
      { id: 'close', label: 'Close', danger: true, onSelect: act('close') },
    ];
  }

  function isMobileViewport() {
    return typeof window !== 'undefined' && window.matchMedia('(max-width: 768px)').matches;
  }

  function handleDeskButtonClick() {
    dispatch('launcher', { open: !launcherOpen });
  }

  // The launcher opened (this button or the host's Cmd+K): on a phone with
  // windows open, the field shows the app switcher. It closed itself (Escape,
  // its Close button): the focus goes back to the button that opened it.
  let launcherWasOpen = false;
  $: if (launcherOpen && !launcherWasOpen) {
    launcherWasOpen = true;
    mobileSwitcherOpen = isMobileViewport() && openWindows.length > 0;
    tick().then(publishSurfaceMetrics);
  } else if (!launcherOpen && launcherWasOpen) {
    launcherWasOpen = false;
    mobileSwitcherOpen = false;
    if (launcherRestoreFocus) tick().then(() => deskButtonEl?.focus());
    tick().then(publishSurfaceMetrics);
  }

  function focusPromptWhenReady() {
    if (promptDisabled || isMobileViewport()) return;
    tick().then(() => {
      const active = document.activeElement;
      if (!active || active === document.body || active === document.documentElement) {
        promptInputEl?.focus();
      }
    });
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
    if (kind === 'texture.document_revision.created') return 'Texture created a new revision';
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
      'texture.document_revision.created',
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
    removeResize = onWindowEvent('resize', resizePromptInput);
    focusPromptWhenReady();
  });

  $: if (authenticated && chyronItems.some((item) => item.id.startsWith('public-'))) chyronItems = [];
  $: if (openWindows.length === 0 && mobileSwitcherOpen) mobileSwitcherOpen = false;
  $: if (!sheetOpen && mobileSwitcherOpen) mobileSwitcherOpen = false;
  $: if (promptDisabled) {
    promptWasDisabled = true;
  } else if (promptWasDisabled) {
    promptWasDisabled = false;
    focusPromptWhenReady();
  }

  onDestroy(() => {
    removeLiveListener();
    resizeObserver?.disconnect();
    removeResize();
  });
</script>

<div
  class="prompt-surface placement-{normalizedPlacement}"
  class:sheet-open={sheetOpen}
  class:mobile-switcher-open={mobileSwitcherOpen}
  data-prompt-surface
  data-placement={normalizedPlacement}
  data-routing={promptRouting ? 'true' : 'false'}
  data-desk-sheet-open={sheetOpen ? 'true' : 'false'}
  data-mobile-switcher-open={mobileSwitcherOpen ? 'true' : 'false'}
  bind:this={surfaceEl}
>
  <button
    bind:this={deskButtonEl}
    class="desk-mark-button"
    data-desk-menu-button
    type="button"
    aria-label="Open app switcher or Desk, {openWindows.length} open windows"
    aria-keyshortcuts="Meta+K Control+K"
    aria-expanded={sheetOpen || mobileSwitcherOpen ? 'true' : 'false'}
    title="Desk (⌘K)"
    on:click={handleDeskButtonClick}
  >
    <svg class="tetra-mark" viewBox={TETRA_MARK_VIEWBOX} role="img" aria-label="Desk" focusable="false">
      {#each TETRA_MARK_PATHS as path}<path d={path} />{/each}
    </svg>
    {#if openWindows.length > 0}<span class="window-count" data-window-count>{openWindows.length}</span>{/if}
  </button>

  <div class="window-tray" data-window-tray>
    {#each openWindows as win (win.windowId)}
      <button
        class:active={win.windowId === activeWindowId}
        class:minimized={win.mode === 'minimized'}
        class="window-tray-item"
        data-window-tray-item
        data-window-mode={win.mode}
        data-window-tray-item-active={win.windowId === activeWindowId ? 'true' : 'false'}
        data-window-id={win.windowId}
        title={win.title}
        on:click={() => openWindowFromTray(win)}
        use:contextMenu={{ label: `${win.title} window`, items: () => trayMenuItems(win) }}
      >
        <span>{win.icon || '□'}</span><small>{win.title}</small>
      </button>
    {/each}
  </div>

  <div class="command-field" class:has-target={!mobileSwitcherOpen} data-command-field>
    {#if mobileSwitcherOpen}
      <div class="mobile-app-switcher" data-mobile-app-switcher aria-label="Open apps">
        {#each openWindows as win (win.windowId)}
          <button
            class:active={win.windowId === activeWindowId}
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
      {#if route}
        <div class="prompt-status" data-prompt-route data-prompt-route-state={route.state} aria-live="polite">
          {route.state === 'routing' ? `${route.label}…` : `→ ${route.label}`}
        </div>
      {/if}
      <button
        class="prompt-target"
        class:new-document={mode !== 'focus'}
        type="button"
        data-prompt-target
        data-prompt-target-mode={mode}
        data-prompt-target-agent={mode === 'focus' ? target?.focus?.app_agent_id : ''}
        aria-pressed={mode === 'focus' ? 'true' : 'false'}
        aria-label={chipAria}
        title={chipAria}
        on:click={toggleTarget}
      >
        <span class="prompt-target-pill">
          <span class="prompt-target-verb">{chipName}</span>
          {#if chipLabel}<span class="prompt-target-sep" aria-hidden="true">·</span><span class="prompt-target-title">{chipLabel}</span>{/if}
        </span>
      </button>
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
    <svg class="voice-glyph" viewBox="0 0 24 24" aria-hidden="true" focusable="false">
      <rect x="9" y="2.5" width="6" height="11" rx="3" />
      <path d="M5.5 11a6.5 6.5 0 0 0 13 0" />
      <line x1="12" y1="17.5" x2="12" y2="21" />
      <line x1="8.5" y1="21" x2="15.5" y2="21" />
    </svg>
  </button>
  <span class:online class="online-indicator" data-online-indicator data-live-status={liveStatus} aria-label={onlineLabel} title={onlineLabel}></span>
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
  .voice-glyph {
    display: block;
    width: 1.42rem;
    height: 1.42rem;
    line-height: 1;
  }

  .voice-glyph {
    fill: none;
    stroke: currentColor;
    stroke-width: 1.7;
    stroke-linecap: round;
    stroke-linejoin: round;
  }

  .voice-button:hover,
  .voice-button:focus-visible {
    box-shadow: var(--choir-control-shadow), var(--choir-shadow-glow);
    outline: none;
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
    color: var(--choir-text-on-accent);
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
    box-shadow: 0 8px 18px color-mix(in srgb, var(--choir-shadow-color) 18%, transparent);
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
      inset 0 14px 28px color-mix(in srgb, var(--choir-shadow-color) 2%, transparent),
      0 12px 30px color-mix(in srgb, var(--choir-shadow-color) 22%, transparent);
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

  .command-field.has-target {
    grid-template-columns: auto minmax(0, 1fr);
    grid-template-rows: 100%;
  }

  .command-field.has-target textarea {
    padding-left: 0.5rem;
  }

  /* The prompt's target, inside the field: no band of its own. The button
     fills the field's height (a phone tap target of at least 40px); the
     pill inside it is what shows. */
  .prompt-target {
    position: relative;
    z-index: 3;
    align-self: stretch;
    display: inline-flex;
    align-items: center;
    min-width: 0;
    max-width: min(14rem, 34vw);
    margin: 0 0 0 0.42rem;
    padding: 0;
    border: 0;
    background: transparent;
    color: var(--choir-fg);
    font: inherit;
    font-size: 0.78rem;
    line-height: 1.2;
    white-space: nowrap;
    cursor: pointer;
  }

  .prompt-target-pill {
    display: inline-flex;
    align-items: baseline;
    gap: 0.3rem;
    min-width: 0;
    padding: 0.3rem 0.62rem;
    border-radius: var(--choir-radius-control-sm);
    background: var(--choir-selected);
  }

  .prompt-target.new-document {
    color: var(--choir-muted);
  }

  .prompt-target.new-document .prompt-target-pill {
    background: var(--choir-panel-soft);
  }

  .prompt-target:focus-visible {
    outline: none;
  }

  .prompt-target:hover .prompt-target-pill,
  .prompt-target:focus-visible .prompt-target-pill {
    box-shadow: var(--choir-control-shadow), var(--choir-shadow-glow);
  }

  .prompt-target-verb {
    flex: 0 0 auto;
    font-weight: 700;
  }

  .prompt-target-sep {
    flex: 0 0 auto;
    opacity: 0.6;
  }

  .tetra-mark {
    display: block;
    flex-shrink: 0;
    overflow: visible;
  }

  .tetra-mark path {
    fill: currentColor;
  }

  .prompt-target-title {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
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
