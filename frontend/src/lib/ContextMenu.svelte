<!--
  ContextMenu — the shell's one context menu (owner, 2026-11-03: "everything
  needs context menu right click"). It renders whatever context-menu.ts put in
  contextMenuState; nothing mounts it by hand (context-menu.ts mounts one host
  on first use).

  WAI-ARIA menu pattern: role=menu / menuitem, roving focus, ArrowUp/Down
  (wrapping, skipping disabled items), Home/End, Enter/Space, ArrowRight and
  ArrowLeft for the submenu, Escape closes one level, Tab closes. It stays on
  screen (placeMenu / placeSubmenu) and takes the theme's tokens.

  Data attributes:
    data-context-menu        — a menu panel (the root and the submenu)
    data-context-submenu     — the submenu panel
    data-context-menu-item   — an item, valued by its id (or its label)
-->
<script lang="ts">
  import { onDestroy, tick } from 'svelte';
  import {
    contextMenuState,
    isSeparator,
    placeMenu,
    placeSubmenu,
    type ContextMenuEntry,
    type ContextMenuItem,
  } from './context-menu-state';

  let rootEl: HTMLDivElement | null = null;
  let subEl: HTMLDivElement | null = null;
  let rootPos = { x: 0, y: 0, maxHeight: 0 };
  let subPos = { x: 0, y: 0, maxHeight: 0 };
  let rootPlaced = false;
  let subPlaced = false;
  let subIndex = -1;
  let current: any = null;
  let returnFocus: HTMLElement | null = null;

  const unsubscribe = contextMenuState.subscribe((value) => {
    if (value && (!current || value.seq !== current.seq)) {
      void openMenu(value);
    } else if (!value && current) {
      teardown(true);
    }
  });

  onDestroy(() => {
    unsubscribe();
    removeGlobalListeners();
  });

  $: items = (current?.items || []) as ContextMenuEntry[];
  $: subItems = (subIndex >= 0 ? (items[subIndex] as ContextMenuItem)?.items || [] : []) as ContextMenuEntry[];

  const itemKey = (item: ContextMenuItem, i: number) => item.id || item.label || String(i);

  function viewport() {
    return { width: window.innerWidth, height: window.innerHeight };
  }

  async function openMenu(value: any) {
    const wasOpen = Boolean(current);
    current = value;
    subIndex = -1;
    subPlaced = false;
    rootPlaced = false;
    if (!wasOpen) {
      const active = document.activeElement as HTMLElement | null;
      returnFocus = value.returnFocus || (active && active !== document.body ? active : null);
      addGlobalListeners();
    } else if (value.returnFocus) {
      returnFocus = value.returnFocus;
    }
    await tick();
    if (!rootEl) return;
    const rect = rootEl.getBoundingClientRect();
    rootPos = placeMenu({ x: value.x, y: value.y }, { width: rect.width, height: rect.height }, viewport());
    rootPlaced = true;
    await tick();
    focusFirst(rootEl);
  }

  function teardown(restoreFocus: boolean) {
    current = null;
    subIndex = -1;
    rootPlaced = false;
    subPlaced = false;
    removeGlobalListeners();
    const target = returnFocus;
    returnFocus = null;
    if (restoreFocus && target && target.isConnected && typeof target.focus === 'function') {
      target.focus({ preventScroll: true });
    }
  }

  function close(restoreFocus = true) {
    if (!current) return;
    teardown(restoreFocus);
    contextMenuState.set(null);
  }

  function enabledButtons(panel: HTMLElement | null): HTMLElement[] {
    if (!panel) return [];
    return Array.from(panel.querySelectorAll<HTMLElement>('[role="menuitem"]:not([aria-disabled="true"])'));
  }

  function focusFirst(panel: HTMLElement | null) {
    enabledButtons(panel)[0]?.focus({ preventScroll: true });
  }

  async function openSub(index: number, focus: boolean) {
    const item = items[index] as ContextMenuItem;
    if (!item?.items?.length || item.disabled) return;
    if (subIndex !== index) {
      subIndex = index;
      subPlaced = false;
      await tick();
      const parent = rootEl?.querySelector<HTMLElement>(`[data-context-menu-index="${index}"]`);
      if (!subEl || !parent) return;
      const prect = parent.getBoundingClientRect();
      const srect = subEl.getBoundingClientRect();
      subPos = placeSubmenu(prect, { width: srect.width, height: srect.height }, viewport());
      subPlaced = true;
      await tick();
    }
    if (focus) focusFirst(subEl);
  }

  function closeSub(focusParent: boolean) {
    const index = subIndex;
    subIndex = -1;
    subPlaced = false;
    if (focusParent) {
      rootEl?.querySelector<HTMLElement>(`[data-context-menu-index="${index}"]`)?.focus({ preventScroll: true });
    }
  }

  function activate(item: ContextMenuItem, index: number, inSub: boolean) {
    if (!item || item.disabled) return;
    if (!inSub && item.items?.length) {
      void openSub(index, true);
      return;
    }
    close(true);
    // Synchronous, so a file picker or clipboard write keeps the user gesture.
    try {
      item.onSelect?.();
    } catch (err) {
      console.error('[context-menu] action failed', err);
    }
  }

  function move(panel: HTMLElement | null, delta: number | 'first' | 'last') {
    const buttons = enabledButtons(panel);
    if (!buttons.length) return;
    const at = buttons.indexOf(document.activeElement as HTMLElement);
    let next = 0;
    if (delta === 'first') next = 0;
    else if (delta === 'last') next = buttons.length - 1;
    else next = at < 0 ? (delta > 0 ? 0 : buttons.length - 1) : (at + delta + buttons.length) % buttons.length;
    buttons[next].focus({ preventScroll: true });
  }

  function handleKeydown(event: KeyboardEvent, inSub: boolean) {
    const panel = inSub ? subEl : rootEl;
    const focused = document.activeElement as HTMLElement | null;
    const index = Number(focused?.dataset?.contextMenuIndex ?? -1);
    switch (event.key) {
      case 'ArrowDown':
        move(panel, 1);
        break;
      case 'ArrowUp':
        move(panel, -1);
        break;
      case 'Home':
        move(panel, 'first');
        break;
      case 'End':
        move(panel, 'last');
        break;
      case 'ArrowRight':
        if (!inSub && index >= 0) void openSub(index, true);
        break;
      case 'ArrowLeft':
        if (inSub) closeSub(true);
        break;
      case 'Escape':
        if (inSub) closeSub(true);
        else close(true);
        break;
      case 'Tab':
        close(false);
        return; // let Tab move focus on
      case 'Enter':
      case ' ': {
        const list = inSub ? subItems : items;
        const item = list[index] as ContextMenuItem;
        if (item && !isSeparator(item)) activate(item, index, inSub);
        break;
      }
      default:
        return;
    }
    event.preventDefault();
    event.stopPropagation();
  }

  function handleItemPointerEnter(index: number, inSub: boolean) {
    const list = inSub ? subItems : items;
    const item = list[index] as ContextMenuItem;
    if (inSub) return;
    if (item?.items?.length && !item.disabled) void openSub(index, false);
    else if (subIndex >= 0) closeSub(false);
  }

  function inside(target: EventTarget | null) {
    const node = target as Node | null;
    return Boolean(node && ((rootEl && rootEl.contains(node)) || (subEl && subEl.contains(node))));
  }

  function onOutsidePointer(event: PointerEvent) {
    if (!inside(event.target)) close(false);
  }

  function onOutsideContextMenu(event: MouseEvent) {
    // A right-click inside the menu does nothing; outside, a new menu (if the
    // target has one) replaces this one in the same tick.
    if (inside(event.target)) event.preventDefault();
  }

  // Only a user's wheel closes the menu: programmatic scrolls (focus moving a
  // row into view scrolls the desktop area) must not.
  function onWheel(event: WheelEvent) {
    if (!inside(event.target)) close(false);
  }

  function onResize() {
    close(false);
  }

  function onBlur() {
    close(false);
  }

  function onKeydown(event: KeyboardEvent) {
    if (event.key === 'Escape') {
      event.preventDefault();
      close(true);
    }
  }

  function addGlobalListeners() {
    window.addEventListener('pointerdown', onOutsidePointer, true);
    window.addEventListener('contextmenu', onOutsideContextMenu, true);
    window.addEventListener('wheel', onWheel, { capture: true, passive: true });
    window.addEventListener('resize', onResize);
    window.addEventListener('blur', onBlur);
    window.addEventListener('keydown', onKeydown);
  }

  function removeGlobalListeners() {
    window.removeEventListener('pointerdown', onOutsidePointer, true);
    window.removeEventListener('contextmenu', onOutsideContextMenu, true);
    window.removeEventListener('wheel', onWheel, { capture: true } as any);
    window.removeEventListener('resize', onResize);
    window.removeEventListener('blur', onBlur);
    window.removeEventListener('keydown', onKeydown);
  }
</script>

{#if current}
  <!-- svelte-ignore a11y-no-noninteractive-element-interactions -->
  <div
    class="choir-context-menu"
    class:placed={rootPlaced}
    bind:this={rootEl}
    role="menu"
    tabindex="-1"
    aria-label={current.label || 'Actions'}
    aria-orientation="vertical"
    data-context-menu
    style="left:{rootPos.x}px; top:{rootPos.y}px; {rootPos.maxHeight ? `max-height:${rootPos.maxHeight}px;` : ''}"
    on:keydown={(event) => handleKeydown(event, false)}
    on:contextmenu|preventDefault
  >
    {#each items as entry, i}
      {#if isSeparator(entry)}
        <div class="separator" role="separator"></div>
      {:else}
        <button
          type="button"
          class="item"
          class:danger={entry.danger}
          class:expanded={subIndex === i}
          role="menuitem"
          tabindex="-1"
          aria-disabled={entry.disabled ? 'true' : undefined}
          aria-haspopup={entry.items?.length ? 'menu' : undefined}
          aria-expanded={entry.items?.length ? (subIndex === i ? 'true' : 'false') : undefined}
          data-context-menu-item={itemKey(entry, i)}
          data-context-menu-index={i}
          on:click={() => activate(entry, i, false)}
          on:pointerenter={() => handleItemPointerEnter(i, false)}
        >
          <span class="icon" aria-hidden="true">{entry.icon || ''}</span>
          <span class="label">{entry.label}</span>
          {#if entry.items?.length}
            <span class="chevron" aria-hidden="true">▸</span>
          {:else if entry.shortcut}
            <span class="shortcut" aria-hidden="true">{entry.shortcut}</span>
          {/if}
        </button>
      {/if}
    {/each}
  </div>

  {#if subIndex >= 0 && subItems.length}
    <!-- svelte-ignore a11y-no-noninteractive-element-interactions -->
    <div
      class="choir-context-menu"
      class:placed={subPlaced}
      bind:this={subEl}
      role="menu"
      tabindex="-1"
      aria-label={(items[subIndex] && !isSeparator(items[subIndex]) && items[subIndex].label) || 'More'}
      aria-orientation="vertical"
      data-context-menu
      data-context-submenu
      style="left:{subPos.x}px; top:{subPos.y}px; {subPos.maxHeight ? `max-height:${subPos.maxHeight}px;` : ''}"
      on:keydown={(event) => handleKeydown(event, true)}
      on:contextmenu|preventDefault
    >
      {#each subItems as entry, j}
        {#if isSeparator(entry)}
          <div class="separator" role="separator"></div>
        {:else}
          <button
            type="button"
            class="item"
            class:danger={entry.danger}
            role="menuitem"
            tabindex="-1"
            aria-disabled={entry.disabled ? 'true' : undefined}
            data-context-menu-item={itemKey(entry, j)}
            data-context-menu-index={j}
            on:click={() => activate(entry, j, true)}
          >
            <span class="icon" aria-hidden="true">{entry.icon || ''}</span>
            <span class="label">{entry.label}</span>
            {#if entry.shortcut}<span class="shortcut" aria-hidden="true">{entry.shortcut}</span>{/if}
          </button>
        {/if}
      {/each}
    </div>
  {/if}
{/if}

<style>
  .choir-context-menu {
    position: fixed;
    z-index: 2147483000;
    min-width: 184px;
    max-width: min(320px, calc(100vw - 16px));
    overflow-y: auto;
    overscroll-behavior: contain;
    padding: 4px;
    border: 1px solid var(--choir-chrome-border, var(--choir-border-strong));
    border-radius: var(--choir-radius-control-sm, 12px);
    background: var(--choir-panel-strong, var(--choir-panel));
    color: var(--choir-text-primary, var(--choir-fg));
    box-shadow: var(--choir-shadow-floating);
    font-family: var(--choir-font-ui, inherit);
    font-size: 0.86rem;
    visibility: hidden;
    outline: none;
  }

  .choir-context-menu.placed {
    visibility: visible;
  }

  .item {
    display: grid;
    grid-template-columns: 1.3em minmax(0, 1fr) auto;
    align-items: center;
    gap: 8px;
    width: 100%;
    min-height: 32px;
    padding: 5px 10px 5px 8px;
    border: 0;
    border-radius: calc(var(--choir-radius-control-sm, 12px) - 4px);
    background: transparent;
    color: inherit;
    font: inherit;
    text-align: left;
    cursor: pointer;
  }

  .item:hover,
  .item:focus-visible,
  .item:focus,
  .item.expanded {
    background: var(--choir-state-hover);
    outline: none;
  }

  .item[aria-disabled='true'] {
    color: var(--choir-text-muted, var(--choir-muted));
    cursor: default;
    background: transparent;
  }

  .item.danger:not([aria-disabled='true']) {
    color: var(--choir-danger);
  }

  .label {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .shortcut,
  .chevron {
    color: var(--choir-text-muted, var(--choir-muted));
    font-size: 0.78rem;
  }

  .separator {
    height: 1px;
    margin: 4px 6px;
    background: var(--choir-border, var(--choir-border-strong));
  }

  @media (pointer: coarse) {
    .item {
      min-height: 44px;
    }
  }
</style>
