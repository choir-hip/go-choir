<!--
  LastResortMenu: the host's plain list menu, the base of the
  contextMenu.renderer slot (services/context-menu.ts). It renders only when no
  renderer package is healthy (menu-default removed, failing, or declined), so
  a right-click always answers. One level, no submenu panel: a submenu's items
  are listed under their parent. Escape, an outside press or a choice closes it.
-->
<script lang="ts">
  import { onDestroy, tick } from 'svelte';
  import { contextMenuState, type ContextMenuEntry, type ContextMenuItem } from './context-menu-state';

  export let request: any = null;

  let el: HTMLElement | null = null;
  const closeContextMenu = () => contextMenuState.set(null);
  const flat = (entries: ContextMenuEntry[] = []): ContextMenuItem[] =>
    entries.flatMap((e) => ((e as { separator?: boolean }).separator ? [] : (e as ContextMenuItem).items?.length ? (e as ContextMenuItem).items!.filter((c) => !(c as { separator?: boolean }).separator) as ContextMenuItem[] : [e as ContextMenuItem]));
  $: items = flat(request?.items);
  $: if (request) void tick().then(() => el?.querySelector<HTMLElement>('button:not([disabled])')?.focus({ preventScroll: true }));

  function choose(item: ContextMenuItem) {
    if (item.disabled) return;
    closeContextMenu();
    try { item.onSelect?.(); } catch (err) { console.error('[context-menu] action failed', err); }
  }

  function onPointer(event: PointerEvent) {
    if (request && el && !el.contains(event.target as Node)) closeContextMenu();
  }
  function onKey(event: KeyboardEvent) {
    if (request && event.key === 'Escape') { event.preventDefault(); closeContextMenu(); }
  }
  window.addEventListener('pointerdown', onPointer, true);
  window.addEventListener('keydown', onKey);
  onDestroy(() => {
    window.removeEventListener('pointerdown', onPointer, true);
    window.removeEventListener('keydown', onKey);
  });
</script>

{#if request && items.length}
  <ul
    class="last-resort-menu"
    bind:this={el}
    role="menu"
    aria-label={request.label || 'Actions'}
    data-last-resort="menu"
    data-context-menu
    style="left:{Math.max(8, Math.min(request.x, window.innerWidth - 248))}px; top:{Math.max(8, Math.min(request.y, window.innerHeight - 48))}px"
  >
    {#each items as item, i (item.id || `${item.label}-${i}`)}
      <li>
        <button type="button" role="menuitem" disabled={item.disabled} data-context-menu-item={item.id || item.label} on:click={() => choose(item)}>{item.label}</button>
      </li>
    {/each}
  </ul>
{/if}

<style>
  .last-resort-menu {
    position: fixed;
    z-index: 30000;
    width: 240px;
    max-height: 70vh;
    overflow: auto;
    margin: 0;
    padding: 4px;
    list-style: none;
    border: 1px solid var(--choir-border, currentColor);
    border-radius: 6px;
    color: var(--choir-fg, #111);
    background: var(--choir-bg, #fff);
  }

  button {
    width: 100%;
    min-height: 32px;
    text-align: left;
  }
</style>
