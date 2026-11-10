<!--
  The prompt bar: the desk mark, the window buttons, the command field with
  its target pill, the voice button and the submit dot. A shell surface of the
  prompt slot (lib/shell-slots.ts; plan section 11.1, slice H3, R31).

  Slot ABI, props: authenticated, placement ('top' | 'bottom'), promptDisabled,
  promptPlaceholder, promptStatus, promptRouting, liveStatus, launcherOpen,
  launcherRestoreFocus, target (the focused window's prompt target:
  { windowId, appId, appName, label, focus, aim? } where focus is
  { app_agent_id, object_ref, selection? } or null and aim, when the window
  states what the bar is aimed at, is { state: 'owned' | 'foreign', verb, chip,
  revisionLabel, baseRevisionId, source }) and route (where the last text went:
  { state: 'routing' | 'routed', label, class?, alternatives? }; a class is
  edit, comment or request, and each alternative is one tap to send the same
  text under it). Events: promptsubmit ({ text, focus, windowId, aim,
  baseRevisionId } or { resendClass }; focus null sends the text to the agent),
  windowaction ({ windowId, action }), launcher ({ open, restoreFocus }).
  Everything else is an SDK call. Cmd+K is the host's (core.toggleDesk).

  One grid: every control is 44px tall and sits on the bar's bottom row, so
  with single-line text they share one centre line. The window buttons focus,
  minimise and restore (a click on the focused window minimises it); on a
  phone one windows button opens the same window list popover that the
  desktop +N button opens. The target pill names only the destination
  ("To Texture", "To Agent"; with an aim, an icon and a verb placeholder); one tap sends to the agent instead and a second restores it. Enter is a newline; the submit button and Cmd/Ctrl+Enter
  send (owner, 2026-11-09). A pasted link is text like any other: it goes to
  the focused object (owner, 2026-11-06).
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

  type PromptAim = {
    state: 'owned' | 'foreign';
    title: string;
    verb: string;
    chip: string;
    revisionLabel: string;
    baseRevisionId: string;
    source: { publication_version_id: string; manifest_hash: string } | null;
  };
  type PromptTarget = { windowId: string; appId: string; appName: string; label: string; focus: PromptFocus | null; aim?: PromptAim | null };
  type PromptRoute = { state: 'routing' | 'routed'; label: string; class?: string; alternatives?: string[] };

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

  const PHONE_QUERY = '(max-width: 768px)';
  // Grid constants shared with the CSS (section 3.1 of the design).
  const ROW = 44;
  const PAD = 6;
  const GAP = 8;
  const FIELD_MIN = 320;
  const BUTTON_MIN = 112;
  const BUTTON_GAP = 4;

  const dispatch = createEventDispatcher();
  let promptValue = '';
  let promptInputEl: HTMLTextAreaElement | null = null;
  let surfaceEl: HTMLDivElement | null = null;
  let deskButtonEl: HTMLButtonElement | null = null;
  let trayEl: HTMLDivElement | null = null;
  let switcherEl: HTMLDivElement | null = null;
  let switcherOpenerEl: HTMLElement | null = null;
  let resizeObserver: ResizeObserver | null = null;
  $: sheetOpen = launcherOpen;
  let isPhone = typeof window !== 'undefined' && window.matchMedia(PHONE_QUERY).matches;
  let surfaceWidth = typeof window !== 'undefined' ? Math.max(0, window.innerWidth - 24) : 1256;
  let switcherOpen = false;
  let switcherStyle = '';
  let rovingOverride = '';
  let promptFocused = false;
  let chyronItems: Array<{ id: string; text: string }> = [];
  let removeLiveListener = () => {};
  let removeResize = () => {};
  let removeMedia = () => {};
  let removeOutside = () => {};
  let promptWasDisabled = promptDisabled;
  // The owner's one-tap choice ('conductor' clears the focus, 'focus' keeps
  // it; '' is the default). It belongs to the target in view and to one
  // send: a different target, or the send settling, clears it.
  let choice = '';
  let choiceKey = '';
  let choiceRouting = false;

  const publicTicker: string[] = [];
  const isMac = typeof navigator !== 'undefined' && /Mac|iPhone|iPad/.test(navigator.platform || navigator.userAgent || '');

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
  // What the text goes to: 'focus' (the focused app agent, or the document
  // the window states as the aim), 'hint' (the focused app has no desk to take
  // it: the agent routes) or 'conductor' (the agent; the value is a hook).
  $: mode = !target
    ? 'conductor'
    : choice === 'conductor'
      ? 'conductor'
      : target.focus || target.aim
        ? 'focus'
        : 'hint';
  $: aim = mode === 'focus' && target?.aim ? target.aim : null;
  $: aimKind = aim ? (aim.state === 'owned' ? 'edit' : 'reply') : mode === 'conductor' ? 'agent' : '';
  // The pill names the destination desk and nothing else. Only a focus the
  // owner can clear to the conductor, or restore, is interactive.
  $: chipName = mode === 'focus' ? target?.appName || '' : 'Agent';
  $: chipText = isPhone ? chipName : `To ${chipName}`;
  $: chipLabel = target?.label || '';
  $: chipInteractive = !!target && (mode === 'focus' || mode === 'conductor');
  $: chipAria = mode === 'focus'
    ? `${aim ? `${aim.verb.replace(/…$/, '')}${aim.revisionLabel ? `, responding to ${aim.revisionLabel}` : ''}` : `To ${target?.appName}${chipLabel ? `: ${chipLabel}` : ''}`}. Activate to send to the agent instead.`
    : mode === 'hint'
      ? `To the agent, which routes the text. ${target?.appName} has no desk for it.`
      : target
        ? `To the agent. Activate to send to ${target.appName}${chipLabel ? `: ${chipLabel}` : ''}.`
        : 'To the agent, which routes the text.';

  // ── Window buttons ────────────────────────────────────────────────────
  function windowState(win: any): 'focused' | 'open' | 'minimized' {
    if (win.mode === 'minimized') return 'minimized';
    return win.focused ? 'focused' : 'open';
  }
  function stateWord(win: any) {
    const state = windowState(win);
    return state === 'minimized' ? 'minimised' : state;
  }
  function windowTitle(win: any) {
    return String(win.title || '').trim() || String(win.appId || '').trim() || 'Window';
  }

  // The deterministic fit (design 4.4): how many buttons the strip holds
  // before the +N button takes the rest. The focused window is always shown.
  $: available = surfaceWidth - 2 * PAD - 3 * ROW - 4 * GAP - FIELD_MIN;
  $: maxSlots = Math.max(1, Math.floor((available + BUTTON_GAP) / (BUTTON_MIN + BUTTON_GAP)));
  $: shownWindows = fitWindows(openWindows, maxSlots, activeWindowId);
  $: hiddenCount = openWindows.length - shownWindows.length;
  $: stopIds = [...shownWindows.map((win) => win.windowId), ...(hiddenCount > 0 ? ['__overflow'] : [])];
  $: rovingId = stopIds.includes(rovingOverride)
    ? rovingOverride
    : stopIds.includes(activeWindowId)
      ? activeWindowId
      : stopIds[0] || '';

  function fitWindows(list: any[], slots: number, activeId: string) {
    if (list.length <= slots) return list;
    const keep = Math.max(1, slots - 1);
    const shown = list.slice(0, keep);
    const focused = list.find((win) => win.windowId === activeId);
    if (focused && !shown.includes(focused)) shown[shown.length - 1] = focused;
    return shown;
  }

  function activateWindow(win: any) {
    const state = windowState(win);
    if (state === 'minimized') restoreWindow(win.windowId);
    else if (state === 'focused') dispatch('windowaction', { windowId: win.windowId, action: 'minimize' });
    else focusWindow(win.windowId);
    dispatch('launcher', { open: false });
    switcherOpen = false;
  }

  // The list never minimises: tapping the current window means go there.
  function chooseWindowFromList(win: any) {
    if (win.mode === 'minimized') restoreWindow(win.windowId);
    else focusWindow(win.windowId);
    dispatch('launcher', { open: false });
    closeSwitcher();
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

  // Closing a window must not be one stray click away.
  function swallowMiddleClick(event: MouseEvent) {
    if (event.button === 1) event.preventDefault();
  }

  function handleTrayKeydown(event: KeyboardEvent) {
    const keys = ['ArrowLeft', 'ArrowRight', 'Home', 'End'];
    if (!keys.includes(event.key) || !trayEl) return;
    const stops = Array.from(trayEl.querySelectorAll<HTMLElement>('[data-stop-id]'));
    if (stops.length === 0) return;
    const current = stops.findIndex((el) => el === document.activeElement || el.contains(document.activeElement));
    let next = current < 0 ? 0 : current;
    if (event.key === 'ArrowRight') next = (current + 1) % stops.length;
    else if (event.key === 'ArrowLeft') next = (current - 1 + stops.length) % stops.length;
    else if (event.key === 'Home') next = 0;
    else next = stops.length - 1;
    event.preventDefault();
    rovingOverride = stops[next].dataset.stopId || '';
    stops[next].focus();
  }

  // ── The window list popover (desktop +N, phone windows button) ────────
  function placeSwitcher() {
    if (!surfaceEl || typeof window === 'undefined') return;
    const bar = surfaceEl.getBoundingClientRect();
    const vertical = normalizedPlacement === 'top'
      ? `top:${Math.round(bar.bottom + 8)}px;`
      : `bottom:${Math.round(window.innerHeight - bar.top + 8)}px;`;
    if (isPhone) {
      switcherStyle = `${vertical}left:${Math.round(bar.left)}px;width:${Math.round(bar.width)}px;`;
    } else {
      const width = Math.min(320, window.innerWidth - 16);
      const anchor = switcherOpenerEl?.getBoundingClientRect().left ?? bar.left;
      const left = Math.max(8, Math.min(anchor, window.innerWidth - width - 8));
      switcherStyle = `${vertical}left:${Math.round(left)}px;width:${width}px;`;
    }
  }

  function switcherItems() {
    return switcherEl ? Array.from(switcherEl.querySelectorAll<HTMLElement>('[data-window-switcher-item]')) : [];
  }

  async function openSwitcher(anchor: HTMLElement | null) {
    switcherOpenerEl = anchor;
    dispatch('launcher', { open: false });
    placeSwitcher();
    switcherOpen = true;
    await tick();
    placeSwitcher();
    const items = switcherItems();
    (items.find((el) => el.dataset.windowState === 'focused') || items[0])?.focus();
  }

  function closeSwitcher(restoreFocus = false) {
    if (!switcherOpen) return;
    switcherOpen = false;
    if (restoreFocus) {
      tick().then(() => {
        const back = switcherOpenerEl && switcherOpenerEl.isConnected ? switcherOpenerEl : deskButtonEl;
        back?.focus();
      });
    }
  }

  function toggleSwitcher(event: MouseEvent) {
    const anchor = event.currentTarget as HTMLElement;
    if (switcherOpen) closeSwitcher();
    else openSwitcher(anchor);
  }

  function handleSwitcherKeydown(event: KeyboardEvent) {
    if (event.key === 'Escape') {
      event.preventDefault();
      event.stopPropagation();
      closeSwitcher(true);
      return;
    }
    const items = switcherItems();
    const index = items.findIndex((el) => el === document.activeElement || el.parentElement?.contains(document.activeElement));
    if (event.key === 'ArrowDown' || event.key === 'ArrowUp' || event.key === 'Home' || event.key === 'End') {
      event.preventDefault();
      if (items.length === 0) return;
      let next = index < 0 ? 0 : index;
      if (event.key === 'ArrowDown') next = Math.min(items.length - 1, index + 1);
      else if (event.key === 'ArrowUp') next = Math.max(0, index - 1);
      else if (event.key === 'Home') next = 0;
      else next = items.length - 1;
      items[next].focus();
    } else if (event.key === 'Tab' && switcherEl) {
      const stops = Array.from(switcherEl.querySelectorAll<HTMLElement>('button'));
      if (stops.length === 0) return;
      const at = stops.indexOf(document.activeElement as HTMLElement);
      const next = event.shiftKey ? (at <= 0 ? stops.length - 1 : at - 1) : (at < 0 || at === stops.length - 1 ? 0 : at + 1);
      event.preventDefault();
      stops[next].focus();
    }
  }

  async function closeWindowFromList(win: any) {
    const items = switcherItems();
    const at = items.findIndex((el) => el.dataset.windowId === win.windowId);
    dispatch('windowaction', { windowId: win.windowId, action: 'close' });
    await tick();
    const after = switcherItems();
    if (after.length > 0) after[Math.max(0, Math.min(at, after.length - 1))].focus();
  }

  function handleOutsidePointer(event: PointerEvent) {
    if (!switcherOpen) return;
    const node = event.target as Node | null;
    if (node && (switcherEl?.contains(node) || switcherOpenerEl?.contains(node))) return;
    closeSwitcher();
  }

  $: if (launcherOpen && switcherOpen) switcherOpen = false;
  $: if (openWindows.length === 0 && switcherOpen) switcherOpen = false;

  // ── Submit, target and the field ──────────────────────────────────────
  function publishSurfaceMetrics() {
    if (!surfaceEl || typeof document === 'undefined') return;
    const size = surfaceEl.offsetHeight || 56;
    const root = document.documentElement;
    root.style.setProperty('--choir-prompt-surface-size', `${size}px`);
    root.style.setProperty('--choir-prompt-surface-top-offset', normalizedPlacement === 'top' ? `${size}px` : '0px');
    root.style.setProperty('--choir-prompt-surface-bottom-offset', normalizedPlacement === 'bottom' ? `${size}px` : '0px');
    root.dataset.promptSurfacePlacement = normalizedPlacement;
    if (switcherOpen) placeSwitcher();
  }

  // The one submit path: the send button (and Ctrl/Cmd+Enter from a
  // keyboard) comes here. Enter is a newline (owner, 2026-11-09: "the enter
  // button on keyboard should be a newline, not submit"). An empty
  // field only takes the focus. With a target in view, one send at a time: a
  // second send while the first routes would read a target the first is
  // still changing, and the text stays in the field.
  function submitPrompt() {
    if (promptDisabled) return;
    if (!promptValue.trim()) {
      promptInputEl?.focus();
      return;
    }
    if (promptRouting && target) return;
    const focus = mode === 'focus' && target?.focus ? target.focus : null;
    // An aim is the document in view: Jev classifies the text and the shell
    // sends it as an edit, a comment (a fork on a document the owner does not
    // own) or a request; the revision it answers rides along.
    dispatch('promptsubmit', {
      text: promptValue.trim(), focus, windowId: focus || aim ? target?.windowId || '' : '',
      aim, baseRevisionId: aim?.baseRevisionId || '',
    });
    promptValue = '';
    tick().then(resizePromptInput);
  }

  function handlePromptKeydown(event: KeyboardEvent) {
    if (promptDisabled) return;
    if (event.key === 'Enter' && (event.metaKey || event.ctrlKey) && !event.isComposing && promptValue.trim()) {
      event.preventDefault();
      submitPrompt();
    } else if (event.key === 'Escape') {
      (event.currentTarget as HTMLTextAreaElement)?.blur();
    }
  }

  // One tap clears the focus to the agent, a second restores it; the
  // focus goes back to the prompt, so the owner keeps typing.
  function toggleTarget() {
    if (!target || !chipInteractive) return;
    choice = mode === 'conductor' ? 'focus' : 'conductor';
    promptInputEl?.focus();
  }

  // The field grows with its text, up to a cap, then scrolls. The bar is
  // anchored to its edge, so it grows away from it (upward at the bottom)
  // and the surface metrics follow.
  function resizePromptInput() {
    if (!promptInputEl) return;
    const compact = window.innerWidth <= 768;
    const lineHeight = Number.parseFloat(getComputedStyle(promptInputEl).lineHeight) || 22;
    const minHeight = ROW;
    const maxHeight = Math.max(minHeight + lineHeight, Math.min(compact ? 168 : 200, window.innerHeight * 0.3));
    promptInputEl.style.height = 'auto';
    const wanted = promptValue ? promptInputEl.scrollHeight : minHeight;
    promptInputEl.style.height = `${Math.max(minHeight, Math.min(wanted, maxHeight))}px`;
    promptInputEl.style.overflowY = wanted > maxHeight ? 'auto' : 'hidden';
    tick().then(publishSurfaceMetrics);
  }

  function handleDeskButtonClick() {
    dispatch('launcher', { open: !launcherOpen });
  }

  // The launcher closed itself (Escape, its Close button): the focus goes
  // back to the button that opened it.
  let launcherWasOpen = false;
  $: if (launcherOpen && !launcherWasOpen) {
    launcherWasOpen = true;
    tick().then(publishSurfaceMetrics);
  } else if (!launcherOpen && launcherWasOpen) {
    launcherWasOpen = false;
    if (launcherRestoreFocus) tick().then(() => deskButtonEl?.focus());
    tick().then(publishSurfaceMetrics);
  }

  function focusPromptWhenReady() {
    if (promptDisabled || isPhone) return;
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

  // The chyron shows only on an idle, empty field with nothing else open.
  $: chyronVisible = chyronTickerItems.length > 0
    && !promptValue
    && !promptFocused
    && !sheetOpen
    && !switcherOpen
    && !promptStatus
    && !route;
  $: placeholder = chyronVisible
    ? ''
    : promptPlaceholder
      || (aim?.verb ? aim.verb : '')
      || (isPhone ? 'Message' : `Enter for a new line · ${isMac ? '⌘Enter' : 'Ctrl+Enter'} to send`);

  // One tap sends the last text again under another class.
  function resendAs(cls: string) {
    if (promptDisabled || promptRouting) return;
    dispatch('promptsubmit', { resendClass: cls });
  }

  onMount(() => {
    if (surfaceEl) surfaceWidth = surfaceEl.clientWidth;
    publishSurfaceMetrics();
    resizePromptInput();
    startPublicTicker();
    removeLiveListener = addLiveEventListener(handleLiveEvent);
    if (typeof ResizeObserver !== 'undefined' && surfaceEl) {
      resizeObserver = new ResizeObserver(() => {
        if (surfaceEl) surfaceWidth = surfaceEl.clientWidth;
        publishSurfaceMetrics();
      });
      resizeObserver.observe(surfaceEl);
    }
    removeResize = onWindowEvent('resize', resizePromptInput);
    const media = window.matchMedia(PHONE_QUERY);
    const onMedia = () => {
      isPhone = media.matches;
      switcherOpen = false;
      tick().then(() => {
        if (surfaceEl) surfaceWidth = surfaceEl.clientWidth;
        resizePromptInput();
      });
    };
    media.addEventListener('change', onMedia);
    removeMedia = () => media.removeEventListener('change', onMedia);
    removeOutside = onWindowEvent('pointerdown', handleOutsidePointer as EventListener, true);
    focusPromptWhenReady();
  });

  $: if (authenticated && chyronItems.some((item) => item.id.startsWith('public-'))) chyronItems = [];
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
    removeMedia();
    removeOutside();
  });
</script>

<div
  class="prompt-surface placement-{normalizedPlacement}"
  class:phone={isPhone}
  class:has-windows={openWindows.length > 0}
  class:sheet-open={sheetOpen}
  data-prompt-surface
  data-placement={normalizedPlacement}
  data-routing={promptRouting ? 'true' : 'false'}
  data-desk-sheet-open={sheetOpen ? 'true' : 'false'}
  bind:this={surfaceEl}
>
  <button
    bind:this={deskButtonEl}
    class="desk-mark-button"
    data-desk-menu-button
    type="button"
    aria-label="Desk"
    aria-haspopup="dialog"
    aria-expanded={sheetOpen ? 'true' : 'false'}
    aria-keyshortcuts="Meta+K Control+K"
    title={isMac ? 'Desk (⌘K)' : 'Desk (Ctrl+K)'}
    on:click={handleDeskButtonClick}
  >
    <svg class="tetra-mark" viewBox={TETRA_MARK_VIEWBOX} aria-hidden="true" focusable="false">
      {#each TETRA_MARK_PATHS as path}<path d={path} />{/each}
    </svg>
  </button>

  {#if openWindows.length > 0 && !isPhone}
    <!-- svelte-ignore a11y-no-noninteractive-element-to-interactive-role -->
    <div
      class="window-tray"
      data-window-tray
      role="toolbar"
      aria-label="Open windows"
      aria-orientation="horizontal"
      bind:this={trayEl}
      on:keydown={handleTrayKeydown}
    >
      {#each shownWindows as win (win.windowId)}
        {@const state = windowState(win)}
        {@const title = windowTitle(win)}
        <button
          type="button"
          class="window-tray-item"
          class:active={state === 'focused'}
          data-window-tray-item
          data-window-id={win.windowId}
          data-window-mode={win.mode}
          data-window-state={state}
          data-window-tray-item-active={state === 'focused' ? 'true' : 'false'}
          data-stop-id={win.windowId}
          aria-pressed={state === 'focused' ? 'true' : 'false'}
          aria-label={`${title}, ${stateWord(win)}`}
          title={title}
          tabindex={win.windowId === rovingId ? 0 : -1}
          on:click={() => activateWindow(win)}
          on:focus={() => (rovingOverride = win.windowId)}
          on:mousedown={swallowMiddleClick}
          on:auxclick={swallowMiddleClick}
          use:contextMenu={{ label: `${title} window`, items: () => trayMenuItems(win) }}
        >
          <span class="window-tray-icon" aria-hidden="true">{win.icon || '□'}</span>
          <span class="window-tray-title">{title}</span>
        </button>
      {/each}
      {#if hiddenCount > 0}
        <button
          type="button"
          class="window-tray-item window-overflow"
          data-window-overflow
          data-stop-id="__overflow"
          aria-haspopup="dialog"
          aria-expanded={switcherOpen ? 'true' : 'false'}
          aria-label={`${hiddenCount} more windows`}
          tabindex={rovingId === '__overflow' ? 0 : -1}
          on:click={toggleSwitcher}
          on:focus={() => (rovingOverride = '__overflow')}
        >+{hiddenCount}</button>
      {/if}
    </div>
  {/if}

  {#if openWindows.length > 0 && isPhone}
    <button
      type="button"
      class="windows-button"
      data-window-switcher-button
      aria-haspopup="dialog"
      aria-expanded={switcherOpen ? 'true' : 'false'}
      aria-label={`Windows, ${openWindows.length} open`}
      on:click={toggleSwitcher}
    >
      <span class="windows-tile" data-window-count-label aria-hidden="true">{openWindows.length}</span>
    </button>
  {/if}

  <div class="command-field" data-command-field>
    {#if promptStatus}<div class="prompt-status" data-prompt-status>{promptStatus}</div>{/if}
    {#if route}
      <div class="prompt-status" data-prompt-route data-prompt-route-state={route.state} aria-live="polite">
        {route.state === 'routing' ? `${route.label}…` : `→ ${route.label}`}
        {#if route.state === 'routed' && route.class}
          <span data-prompt-class={route.class}>Sent as {route.class}.</span>
          {#each route.alternatives || [] as alt (alt)}
            <button type="button" class="prompt-resend" data-prompt-resend={alt} on:click={() => resendAs(alt)}>Send as {alt}</button>
          {/each}
        {/if}
      </div>
    {/if}

    <button
      class="prompt-target"
      class:is-focus={mode === 'focus'}
      class:is-override={mode === 'conductor' && !!target}
      class:is-passive={!chipInteractive}
      type="button"
      data-prompt-target
      data-prompt-target-mode={mode}
      data-prompt-target-agent={mode === 'focus' ? target?.focus?.app_agent_id : ''}
      aria-pressed={chipInteractive ? (mode === 'focus' ? 'true' : 'false') : undefined}
      aria-disabled={chipInteractive ? undefined : 'true'}
      aria-label={chipAria}
      title={chipAria}
      disabled={!chipInteractive}
      on:click={toggleTarget}
    >
      <span class="prompt-target-pill">
        {#if aimKind}
          <svg class="prompt-aim-icon" viewBox="0 0 24 24" aria-hidden="true" focusable="false" data-prompt-aim-icon={aimKind}>
            {#if aimKind === 'edit'}
              <path d="M4 20h4L19 9l-4-4L4 16z" /><path d="M13.5 6.5l4 4" />
            {:else if aimKind === 'reply'}
              <path d="M10 6 4 12l6 6" /><path d="M4 12h10a6 6 0 0 1 6 6" />
            {:else}
              <path d="M12 3v4M12 17v4M3 12h4M17 12h4M6 6l2.5 2.5M15.5 15.5 18 18M18 6l-2.5 2.5M8.5 15.5 6 18" />
            {/if}
          </svg>
        {/if}
        <span class="prompt-target-label">{chipText}</span>
        {#if aim?.revisionLabel}
          <span class="prompt-aim-revision" data-prompt-aim-revision data-prompt-aim-state={aim.state} title={`Responding to ${aim.revisionLabel}${aim.title ? ` · ${aim.title}` : ''}`}>{aim.revisionLabel}</span>
        {/if}
        {#if chipInteractive && !isPhone}<span class="prompt-target-swap" aria-hidden="true">⇄</span>{/if}
      </span>
    </button>
    <div class="prompt-input-cell">
      {#if chyronVisible}
        <div class="agent-chyron" data-agent-chyron data-prompt-chyron aria-live="polite">
          <div>{#each chyronTickerItems as item, index (`${item.id}-${index}`)}<span>{item.text}</span>{/each}</div>
        </div>
      {/if}
      <textarea
        bind:this={promptInputEl}
        bind:value={promptValue}
        data-prompt-input
        rows="1"
        {placeholder}
        disabled={promptDisabled}
        aria-label="Command prompt"
        on:keydown={handlePromptKeydown}
        on:input={resizePromptInput}
        enterkeyhint="enter"
        on:focus={() => (promptFocused = true)}
        on:blur={() => (promptFocused = false)}
      />
    </div>
  </div>

  {#if !isPhone}
    <button class="voice-button" data-agent-audio-button type="button" aria-label="Agent audio input">
      <svg class="voice-glyph" viewBox="0 0 24 24" aria-hidden="true" focusable="false">
        <rect x="9" y="2.5" width="6" height="11" rx="3" />
        <path d="M5.5 11a6.5 6.5 0 0 0 13 0" />
        <line x1="12" y1="17.5" x2="12" y2="21" />
        <line x1="8.5" y1="21" x2="15.5" y2="21" />
      </svg>
    </button>
  {/if}
  <button
    class:online
    class="prompt-submit"
    type="button"
    data-prompt-submit
    data-online-indicator
    data-live-status={liveStatus}
    aria-label="Send prompt ({onlineLabel})"
    title="Send ({onlineLabel})"
    disabled={promptDisabled}
    on:click={submitPrompt}
  >
    <svg class="submit-glyph" viewBox="0 0 24 24" aria-hidden="true" focusable="false">
      <path d="M12 19V6" />
      <path d="M6.5 11.5 12 6l5.5 5.5" />
    </svg>
  </button>
</div>

{#if switcherOpen}
  <!-- Outside the surface: its backdrop-filter would make it the containing
       block of a fixed child. -->
  <div
    class="window-switcher"
    data-window-switcher
    role="dialog"
    aria-label="Open windows"
    style={switcherStyle}
    bind:this={switcherEl}
    on:keydown={handleSwitcherKeydown}
  >
    {#each openWindows as win (win.windowId)}
      {@const state = windowState(win)}
      {@const title = windowTitle(win)}
      <div class="switcher-row" use:contextMenu={{ label: `${title} window`, items: () => trayMenuItems(win) }}>
        <button
          type="button"
          class="switcher-item"
          data-window-switcher-item
          data-window-id={win.windowId}
          data-window-state={state}
          aria-label={`${title}, ${stateWord(win)}`}
          on:click={() => chooseWindowFromList(win)}
        >
          <span class="window-tray-icon" aria-hidden="true">{win.icon || '□'}</span>
          <span class="switcher-title">{title}</span>
          <span class="switcher-state">{state === 'focused' ? 'Focused' : state === 'minimized' ? 'Minimised' : ''}</span>
        </button>
        <button
          type="button"
          class="switcher-close"
          data-window-switcher-close
          aria-label={`Close ${title}`}
          on:click={() => closeWindowFromList(win)}
        ><span aria-hidden="true">×</span></button>
      </div>
    {/each}
  </div>
{/if}

<style>
  .prompt-surface {
    --pb-row: 44px;
    --pb-pad: 6px;
    --pb-gap: 8px;
    --pb-icon: 20px;
    position: fixed;
    left: max(12px, env(safe-area-inset-left));
    right: max(12px, env(safe-area-inset-right));
    z-index: 10000;
    display: grid;
    grid-template-columns: var(--pb-row) minmax(320px, 1fr) var(--pb-row) var(--pb-row);
    align-items: end;
    column-gap: var(--pb-gap);
    padding: var(--pb-pad);
    box-sizing: border-box;
    border-radius: min(var(--choir-radius-pill), 1.75rem);
    color: var(--choir-fg);
    background: var(--choir-prompt-surface-bg);
    box-shadow: var(--choir-shadow-floating), var(--choir-shadow-glow);
    backdrop-filter: blur(var(--choir-blur));
  }

  .prompt-surface.has-windows:not(.phone) {
    grid-template-columns: var(--pb-row) auto minmax(320px, 1fr) var(--pb-row) var(--pb-row);
  }

  .prompt-surface.phone {
    --pb-gap: 6px;
    left: max(8px, env(safe-area-inset-left));
    right: max(8px, env(safe-area-inset-right));
    grid-template-columns: var(--pb-row) minmax(0, 1fr) var(--pb-row);
  }

  .prompt-surface.phone.has-windows {
    grid-template-columns: var(--pb-row) var(--pb-row) minmax(0, 1fr) var(--pb-row);
  }

  .prompt-surface.placement-bottom {
    bottom: max(10px, env(safe-area-inset-bottom));
  }

  .prompt-surface.placement-top {
    top: max(10px, env(safe-area-inset-top));
    align-items: start;
  }

  /* One focus ring for every control on the bar and in the list. Outlines
     survive the themes' box-shadow overrides. */
  .prompt-surface :is(button):focus-visible,
  .window-switcher :is(button):focus-visible {
    outline: 2px solid var(--choir-accent);
    outline-offset: 2px;
  }

  .desk-mark-button,
  .voice-button,
  .windows-button {
    position: relative;
    display: grid;
    place-items: center;
    width: var(--pb-row);
    height: var(--pb-row);
    padding: 0;
    border: 0;
    border-radius: var(--choir-radius-control);
    background: var(--choir-control-bg);
    color: var(--choir-tetramark-color);
    box-shadow: var(--choir-control-shadow);
    cursor: pointer;
  }

  .windows-button {
    color: var(--choir-fg);
  }

  .desk-mark-button :global(svg),
  .voice-glyph {
    display: block;
    width: var(--pb-icon);
    height: var(--pb-icon);
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
  .desk-mark-button:hover,
  .windows-button:hover {
    box-shadow: var(--choir-control-shadow), var(--choir-shadow-glow);
  }

  .windows-tile {
    display: grid;
    place-items: center;
    box-sizing: border-box;
    width: 22px;
    height: 22px;
    border: 2px solid currentColor;
    border-radius: 6px;
    font-size: 12px;
    font-weight: 700;
    line-height: 1;
  }

  .window-tray {
    display: flex;
    gap: 4px;
    min-width: 0;
    height: var(--pb-row);
    align-items: center;
  }

  .window-tray-item {
    position: relative;
    flex: 0 1 152px;
    min-width: 112px;
    max-width: 152px;
    height: var(--pb-row);
    box-sizing: border-box;
    display: inline-flex;
    align-items: center;
    gap: 8px;
    padding: 0 12px 0 10px;
    border: 1px solid transparent;
    border-radius: var(--choir-radius-control-sm);
    background: transparent;
    color: var(--choir-fg);
    font: inherit;
    cursor: pointer;
  }

  .window-tray-item:hover,
  .switcher-item:hover {
    background: var(--choir-state-hover);
  }

  .window-tray-item.active,
  .window-tray-item.active:hover {
    background: var(--choir-selected);
  }

  .window-overflow {
    flex: 0 0 var(--pb-row);
    min-width: var(--pb-row);
    max-width: var(--pb-row);
    justify-content: center;
    padding: 0;
    font-size: 13px;
    font-weight: 700;
  }

  .window-tray-icon {
    flex: 0 0 var(--pb-icon);
    display: grid;
    place-items: center;
    width: var(--pb-icon);
    height: var(--pb-icon);
    font-size: 17px;
    line-height: 20px;
  }

  .window-tray-title {
    flex: 1 1 auto;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    text-align: left;
    font-size: 13px;
    line-height: 16px;
    font-weight: 600;
  }

  /* The state marks are pseudo-elements and child opacity, so they survive
     the themes that force background, border and shadow to nothing. */
  .window-tray-item[data-window-state]::after {
    content: '';
    position: absolute;
    bottom: 3px;
    left: 50%;
    transform: translateX(-50%);
    pointer-events: none;
  }

  .window-tray-item[data-window-state='focused']::after {
    width: 16px;
    height: 3px;
    border-radius: 2px;
    background: var(--choir-accent);
  }

  .window-tray-item[data-window-state='open']::after {
    width: 4px;
    height: 4px;
    border-radius: 50%;
    background: var(--choir-muted);
  }

  .window-tray-item[data-window-state='minimized']::after {
    content: none;
  }

  .window-tray-item[data-window-state='minimized'] {
    color: var(--choir-muted);
  }

  .window-tray-item[data-window-state='minimized'] .window-tray-icon,
  .switcher-item[data-window-state='minimized'] .window-tray-icon {
    opacity: 0.5;
  }

  /* The command field */
  .command-field {
    position: relative;
    min-width: 0;
    min-height: var(--pb-row);
    display: grid;
    grid-template-columns: max-content minmax(0, 1fr);
    align-items: end;
    border-radius: min(var(--choir-radius-pill), 22px);
    background:
      linear-gradient(180deg, color-mix(in srgb, var(--choir-panel-soft) 50%, transparent), transparent 58%),
      var(--choir-input-bg);
    box-shadow:
      inset 0 14px 28px color-mix(in srgb, var(--choir-shadow-color) 2%, transparent),
      0 12px 30px color-mix(in srgb, var(--choir-shadow-color) 22%, transparent);
  }

  .command-field:focus-within {
    box-shadow:
      inset 0 14px 28px color-mix(in srgb, var(--choir-shadow-color) 2%, transparent),
      0 12px 30px color-mix(in srgb, var(--choir-shadow-color) 22%, transparent),
      0 0 0 2px color-mix(in srgb, var(--choir-accent) 55%, transparent);
  }

  .prompt-input-cell {
    position: relative;
    min-width: 0;
    align-self: end;
  }

  .prompt-input-cell textarea {
    position: relative;
    z-index: 2;
    width: 100%;
    box-sizing: border-box;
    min-width: 0;
    min-height: var(--pb-row);
    display: block;
    border: 0;
    outline: 0;
    resize: none;
    color: var(--choir-fg);
    background: transparent;
    font: inherit;
    font-size: 16px;
    line-height: 22px;
    padding: 11px 12px 11px 8px;
  }

  /* A long hint is clipped on one line, never wrapped under the first row. */
  .prompt-input-cell textarea::placeholder {
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  /* The target pill: the button is the whole row height (the tap target);
     the pill inside is what shows. It never collapses in multiline text. */
  .prompt-target {
    position: relative;
    z-index: 3;
    align-self: end;
    height: var(--pb-row);
    width: max-content;
    max-width: 11rem;
    display: flex;
    align-items: center;
    margin: 0 0 0 8px;
    padding: 0;
    border: 0;
    background: transparent;
    color: var(--choir-fg);
    font: inherit;
    cursor: pointer;
  }

  .prompt-target:disabled {
    cursor: default;
    opacity: 1;
  }

  .prompt-target-pill {
    display: inline-flex;
    align-items: center;
    box-sizing: border-box;
    min-width: 0;
    max-width: 100%;
    height: 28px;
    padding: 0 10px;
    border-radius: var(--choir-radius-control-sm);
    background: var(--choir-selected);
    color: var(--choir-fg);
    font-size: 13px;
    line-height: 28px;
    font-weight: 600;
    white-space: nowrap;
  }

  .prompt-target.is-override .prompt-target-pill {
    background: transparent;
    outline: 1px dashed var(--choir-border-strong);
    outline-offset: -1px;
  }

  .prompt-target.is-passive .prompt-target-pill {
    background: var(--choir-panel-soft);
    color: var(--choir-muted);
  }

  .prompt-target:not(:disabled):hover .prompt-target-pill {
    box-shadow: var(--choir-control-shadow), var(--choir-shadow-glow);
  }

  .prompt-aim-icon {
    flex: 0 0 auto;
    width: 14px;
    height: 14px;
    margin-right: 6px;
    fill: none;
    stroke: currentColor;
    stroke-width: 2;
    stroke-linecap: round;
    stroke-linejoin: round;
  }

  .prompt-aim-revision {
    flex: 0 0 auto;
    margin-left: 6px;
    color: var(--choir-muted);
    font-size: 12px;
    font-weight: 600;
  }

  .prompt-resend {
    margin-left: 8px;
    padding: 0 8px;
    border: 1px solid var(--choir-border-strong);
    border-radius: var(--choir-radius-control-sm);
    background: transparent;
    color: var(--choir-fg);
    font: inherit;
    font-size: 12px;
    line-height: 18px;
    cursor: pointer;
  }

  .prompt-resend:hover {
    background: var(--choir-selected);
  }

  .prompt-target-label {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .prompt-target-swap {
    flex: 0 0 auto;
    margin-left: 6px;
    font-size: 12px;
    opacity: 0.7;
  }

  .tetra-mark {
    display: block;
    flex-shrink: 0;
    overflow: visible;
  }

  .tetra-mark path {
    fill: currentColor;
  }

  .prompt-status {
    grid-column: 1 / -1;
    position: relative;
    z-index: 3;
    box-sizing: border-box;
    padding: 6px 14px 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--choir-accent-2);
    font-size: 12px;
    line-height: 18px;
    font-weight: 800;
  }

  /* The chyron lives in the textarea's cell only, never under the pill. */
  .agent-chyron {
    position: absolute;
    inset: 0;
    z-index: 1;
    display: flex;
    align-items: center;
    overflow: hidden;
    opacity: 0.32;
    pointer-events: none;
    mask-image: linear-gradient(90deg, transparent, black 8%, black 92%, transparent);
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

  @media (prefers-reduced-motion: reduce) {
    .agent-chyron > div {
      animation: none;
      transform: translateX(0);
    }
  }

  .prompt-submit {
    display: grid;
    place-items: center;
    width: var(--pb-row);
    height: var(--pb-row);
    padding: 0;
    border: 0;
    border-radius: 50%;
    background: var(--choir-muted);
    color: var(--choir-text-on-accent);
    box-shadow: 0 0 18px color-mix(in srgb, var(--choir-muted) 26%, transparent);
    cursor: pointer;
  }

  .prompt-submit.online {
    background: var(--choir-success);
    box-shadow: 0 0 18px color-mix(in srgb, var(--choir-success) 45%, transparent);
  }

  .prompt-submit:disabled {
    opacity: 0.55;
    cursor: default;
  }

  .submit-glyph {
    display: block;
    width: var(--pb-icon);
    height: var(--pb-icon);
    fill: none;
    stroke: currentColor;
    stroke-width: 2.2;
    stroke-linecap: round;
    stroke-linejoin: round;
  }

  /* The window list popover */
  .window-switcher {
    position: fixed;
    z-index: 10001;
    box-sizing: border-box;
    max-height: min(60vh, 432px);
    overflow-y: auto;
    padding: 8px;
    color: var(--choir-fg);
    background: var(--choir-sheet-bg, var(--choir-panel));
    border: 1px solid var(--choir-border);
    border-radius: var(--choir-radius-panel, 16px);
    box-shadow: var(--choir-shadow-floating);
  }

  .switcher-row {
    display: flex;
    align-items: center;
    gap: 4px;
  }

  .switcher-item {
    position: relative;
    flex: 1 1 auto;
    min-width: 0;
    height: 48px;
    box-sizing: border-box;
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 0 12px 0 16px;
    border: 1px solid transparent;
    border-radius: var(--choir-radius-control-sm);
    background: transparent;
    color: var(--choir-fg);
    font: inherit;
    text-align: left;
    cursor: pointer;
  }

  .switcher-item[data-window-state='focused'] {
    background: var(--choir-selected);
  }

  .switcher-item[data-window-state='minimized'] {
    color: var(--choir-muted);
  }

  .switcher-item::after {
    content: '';
    position: absolute;
    left: 5px;
    top: 50%;
    transform: translateY(-50%);
  }

  .switcher-item[data-window-state='focused']::after {
    width: 3px;
    height: 16px;
    border-radius: 2px;
    background: var(--choir-accent);
  }

  .switcher-item[data-window-state='open']::after {
    width: 4px;
    height: 4px;
    border-radius: 50%;
    background: var(--choir-muted);
  }

  .switcher-item[data-window-state='minimized']::after {
    content: none;
  }

  .switcher-title {
    flex: 1 1 auto;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: 14px;
    font-weight: 600;
  }

  .switcher-state {
    flex: 0 0 auto;
    color: var(--choir-muted);
    font-size: 12px;
  }

  .switcher-close {
    flex: 0 0 44px;
    display: grid;
    place-items: center;
    width: 44px;
    height: 44px;
    padding: 0;
    border: 0;
    border-radius: var(--choir-radius-control-sm);
    background: transparent;
    color: var(--choir-muted);
    font-size: 16px;
    line-height: 1;
    cursor: pointer;
  }

  .switcher-close:hover {
    background: var(--choir-state-hover);
    color: var(--choir-fg);
  }

  .prompt-surface.phone .prompt-target {
    max-width: 88px;
  }
</style>
