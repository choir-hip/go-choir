// The one open context menu, shared by the host component and the API
// (context-menu.ts). Kept apart from both so the component and the API do not
// import each other.
import { writable } from 'svelte/store';

export type ContextMenuItem = {
  /** Stable id; rendered as data-context-menu-item for tests and automation. */
  id?: string;
  label: string;
  icon?: string;
  /** A hint shown at the right edge, e.g. "⌘C". */
  shortcut?: string;
  disabled?: boolean;
  /** Destructive actions (Delete) take the danger colour. */
  danger?: boolean;
  /** Runs after the menu has closed, inside the same user gesture. */
  onSelect?: () => unknown;
  /** One level of submenu ("Open with ▸"). */
  items?: ContextMenuEntry[];
  /**
   * A command to run on select (the contextMenu.build pipeline, and a package's
   * contributes.menus, name a command and never a callback). A package item
   * carries this, not onSelect.
   */
  command?: string;
  /** Window close, minimize, dismiss, "Use default menu": no handler removes or edits these. */
  essential?: boolean;
  separator?: false;
};

/**
 * What a menu is for. kind, ext and name are public: the contextMenu.build
 * pipeline and contributes.menus see them. data is the opener's own and never
 * leaves the host (a file row's open-with callback, for one).
 */
export type ContextMenuTarget = {
  kind: string;
  name?: string;
  ext?: string;
  data?: Record<string, unknown>;
};

export type ContextMenuSeparator = { separator: true };

export type ContextMenuEntry = ContextMenuItem | ContextMenuSeparator;

export type ContextMenuRequest = {
  /** Viewport coordinates of the press, or of the element for keyboard opens. */
  x: number;
  y: number;
  items: ContextMenuEntry[];
  /** Accessible name of the menu. */
  label?: string;
  /** Where focus returns when the menu closes (defaults to the focused element). */
  returnFocus?: HTMLElement | null;
  /** What the menu is for; absent means kind "other". */
  target?: ContextMenuTarget;
};

export const contextMenuState = writable<(ContextMenuRequest & { seq: number }) | null>(null);

export function isSeparator(entry: ContextMenuEntry): entry is ContextMenuSeparator {
  return Boolean(entry && (entry as ContextMenuSeparator).separator === true);
}

export const MENU_MARGIN = 8;

/**
 * Where a menu of `size` opened at `point` goes so it stays inside `viewport`:
 * it opens down and right of the point, flips left or up when it would spill
 * past the edge, and is clamped (and scrolls, via maxHeight) when it is larger
 * than the viewport.
 */
export function placeMenu(
  point: { x: number; y: number },
  size: { width: number; height: number },
  viewport: { width: number; height: number },
  margin = MENU_MARGIN,
): { x: number; y: number; maxHeight: number } {
  const maxHeight = Math.max(0, viewport.height - margin * 2);
  const height = Math.min(size.height, maxHeight);
  const axis = (start: number, extent: number, limit: number) => {
    if (start + extent <= limit - margin) return Math.max(margin, start);
    if (start - extent >= margin) return start - extent;
    return Math.max(margin, limit - margin - extent);
  };
  return {
    x: Math.round(axis(point.x, size.width, viewport.width)),
    y: Math.round(axis(point.y, height, viewport.height)),
    maxHeight,
  };
}

/** A submenu beside its parent item: right if it fits, else left, then clamped. */
export function placeSubmenu(
  parent: { left: number; right: number; top: number },
  size: { width: number; height: number },
  viewport: { width: number; height: number },
  margin = MENU_MARGIN,
): { x: number; y: number; maxHeight: number } {
  const maxHeight = Math.max(0, viewport.height - margin * 2);
  const height = Math.min(size.height, maxHeight);
  let x = parent.right - 2;
  if (x + size.width > viewport.width - margin) x = parent.left - size.width + 2;
  x = Math.min(Math.max(margin, x), Math.max(margin, viewport.width - margin - size.width));
  let y = parent.top - 4;
  y = Math.min(Math.max(margin, y), Math.max(margin, viewport.height - margin - height));
  return { x: Math.round(x), y: Math.round(y), maxHeight };
}
