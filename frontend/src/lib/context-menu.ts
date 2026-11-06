// The context-menu API (owner, 2026-11-03: "everything needs context menu
// right click"). One menu at a time, rendered by the active renderer of the
// contextMenu.renderer slot (the menu-default package, or the host's
// last-resort menu; menu-host.ts mounts them), once on document.body the
// first time each renders; the shell and every package (through app-sdk.ts)
// share it.
//
//   use:contextMenu={items | () => items | { items, label, longPress }}
//     right-click, a long press on touch (550 ms, cancelled by movement), and
//     the keyboard (Shift+F10 or the Menu key while the node or something in it
//     has focus). The innermost node with the action wins.
//   openContextMenu({ x, y, items, label })   open one at a point yourself
//   closeContextMenu()
import { get } from 'svelte/store';
import {
  contextMenuState,
  placeMenu,
  placeSubmenu,
  type ContextMenuEntry,
  type ContextMenuItem,
  type ContextMenuRequest,
  type ContextMenuSeparator,
  type ContextMenuTarget,
} from './context-menu-state';
import { presentMenu } from './services/context-menu';
import './services/open-with-menu';

export { contextMenuState, placeMenu, placeSubmenu };
export type { ContextMenuEntry, ContextMenuItem, ContextMenuRequest, ContextMenuSeparator, ContextMenuTarget };

export const LONG_PRESS_MS = 550;
const LONG_PRESS_SLOP_PX = 10;

/**
 * Opens a menu at viewport point (x, y): host contributions (Open with on a
 * file), the contextMenu.build pipeline for a target something intercepts, then
 * the active renderer. An empty item list opens nothing.
 */
export function openContextMenu(request: ContextMenuRequest): boolean {
  if (!(request?.items || []).some(Boolean) && !request?.target) return false;
  return presentMenu(request);
}

export function closeContextMenu() {
  if (get(contextMenuState)) contextMenuState.set(null);
}

export function isContextMenuOpen(): boolean {
  return Boolean(get(contextMenuState));
}

export type ContextMenuOptions =
  | ContextMenuEntry[]
  | (() => ContextMenuEntry[])
  | {
      items: ContextMenuEntry[] | (() => ContextMenuEntry[]);
      label?: string;
      /** Long press on touch opens the menu (default true). */
      longPress?: boolean;
      /** Off when false or when items resolve to an empty list. */
      enabled?: boolean;
      /** What this menu is for ({ kind: 'file', name }): contributions and the build pipeline key on it. */
      target?: ContextMenuTarget;
    };

function normalize(options: ContextMenuOptions) {
  if (Array.isArray(options) || typeof options === 'function') {
    return { items: options, label: undefined as string | undefined, longPress: true, enabled: true, target: undefined as ContextMenuTarget | undefined };
  }
  return {
    items: options?.items || [],
    label: options?.label,
    longPress: options?.longPress !== false,
    enabled: options?.enabled !== false,
    target: options?.target,
  };
}

function resolveItems(items: ContextMenuEntry[] | (() => ContextMenuEntry[])): ContextMenuEntry[] {
  try {
    return (typeof items === 'function' ? items() : items) || [];
  } catch (err) {
    console.error('[context-menu] items failed', err);
    return [];
  }
}

/** Svelte action: gives `node` a context menu. */
export function contextMenu(node: HTMLElement, options: ContextMenuOptions) {
  let opts = normalize(options);
  let pressTimer: ReturnType<typeof setTimeout> | null = null;
  let pressStart: { x: number; y: number; id: number } | null = null;
  let suppressClickUntil = 0;
  // A long press already opened it; the contextmenu event the platform may
  // send after it (Android does on long press) is swallowed.
  let guardUntil = 0;

  const open = (x: number, y: number, returnFocus: HTMLElement | null) => {
    if (!opts.enabled) return false;
    const items = resolveItems(opts.items);
    if (!items.length && !opts.target) return false;
    return openContextMenu({ x, y, items, label: opts.label, returnFocus, target: opts.target });
  };

  const anchorPoint = () => {
    const rect = node.getBoundingClientRect();
    return { x: rect.left + Math.min(16, rect.width / 2), y: rect.top + Math.min(rect.height, 40) };
  };

  const cancelPress = () => {
    if (pressTimer) clearTimeout(pressTimer);
    pressTimer = null;
    pressStart = null;
  };

  function onContextMenu(event: MouseEvent) {
    // Keyboard-generated contextmenu events carry no pointer position.
    const fromKeyboard = event.button === -1 || (event.clientX === 0 && event.clientY === 0);
    if (inTextField(event.target)) {
      event.stopPropagation();
      return;
    }
    if (Date.now() < guardUntil || (fromKeyboard && isContextMenuOpen())) {
      event.preventDefault();
      event.stopPropagation();
      return;
    }
    cancelPress();
    const point = fromKeyboard ? anchorPoint() : { x: event.clientX, y: event.clientY };
    const focused = document.activeElement as HTMLElement | null;
    const returnFocus = node.contains(focused) ? focused : node;
    if (open(point.x, point.y, returnFocus)) {
      event.preventDefault();
      event.stopPropagation();
    }
  }

  function onKeydown(event: KeyboardEvent) {
    const isMenuKey = event.key === 'ContextMenu' || (event.shiftKey && event.key === 'F10');
    if (!isMenuKey || inTextField(event.target)) return;
    const point = anchorPoint();
    const focused = document.activeElement as HTMLElement | null;
    if (open(point.x, point.y, node.contains(focused) ? focused : node)) {
      event.preventDefault();
      event.stopPropagation();
    }
  }

  // The innermost node with the action owns a press, as stopPropagation makes
  // it own a right-click: an ancestor's timer must not replace the row's menu.
  const ownsTarget = (target: EventTarget | null) =>
    (target as Element | null)?.closest?.('[data-context-menu-bound]') === node;

  // Text fields keep the browser's own menu (paste, spelling).
  const inTextField = (target: EventTarget | null) => {
    const field = (target as Element | null)?.closest?.('input, textarea, select, [contenteditable=""], [contenteditable="true"]');
    return Boolean(field && field !== node);
  };

  function onPointerDown(event: PointerEvent) {
    if (!opts.longPress || event.pointerType === 'mouse' || !event.isPrimary) return;
    if (!ownsTarget(event.target) || inTextField(event.target)) return;
    cancelPress();
    pressStart = { x: event.clientX, y: event.clientY, id: event.pointerId };
    pressTimer = setTimeout(() => {
      const start = pressStart;
      pressTimer = null;
      pressStart = null;
      if (!start) return;
      if (open(start.x, start.y, node)) {
        suppressClickUntil = Date.now() + 1000;
        guardUntil = Date.now() + 800;
      }
    }, LONG_PRESS_MS);
  }

  function onPointerMove(event: PointerEvent) {
    if (!pressStart || event.pointerId !== pressStart.id) return;
    if (Math.hypot(event.clientX - pressStart.x, event.clientY - pressStart.y) > LONG_PRESS_SLOP_PX) cancelPress();
  }

  function onPointerEnd(event: PointerEvent) {
    if (pressStart && event.pointerId === pressStart.id) cancelPress();
  }

  // The click a long press ends with must not also activate the target.
  function onClickCapture(event: MouseEvent) {
    if (Date.now() < suppressClickUntil) {
      suppressClickUntil = 0;
      event.preventDefault();
      event.stopImmediatePropagation();
    }
  }

  node.setAttribute('data-context-menu-bound', '');
  node.addEventListener('contextmenu', onContextMenu);
  node.addEventListener('keydown', onKeydown);
  node.addEventListener('pointerdown', onPointerDown);
  node.addEventListener('pointermove', onPointerMove);
  node.addEventListener('pointerup', onPointerEnd);
  node.addEventListener('pointercancel', onPointerEnd);
  node.addEventListener('click', onClickCapture, true);

  return {
    update(next: ContextMenuOptions) {
      opts = normalize(next);
    },
    destroy() {
      cancelPress();
      node.removeAttribute('data-context-menu-bound');
      node.removeEventListener('contextmenu', onContextMenu);
      node.removeEventListener('keydown', onKeydown);
      node.removeEventListener('pointerdown', onPointerDown);
      node.removeEventListener('pointermove', onPointerMove);
      node.removeEventListener('pointerup', onPointerEnd);
      node.removeEventListener('pointercancel', onPointerEnd);
      node.removeEventListener('click', onClickCapture, true);
    },
  };
}
