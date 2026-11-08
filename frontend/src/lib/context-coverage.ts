// Context-menu coverage (slice MA5.2; owner, 2026-11-03: "everything needs
// context menu right click"). A node that binds use:contextMenu owns its own
// menu and stops the event. This is the fallback for everything else: one
// listener the desktop installs on the document, which turns an unclaimed
// right-click (or the Menu key; Shift+F10 keeps the browser menu) into a menu with a target, so the
// contextMenu.build pipeline and contributes.menus reach it like any other:
//
//   app      a launcher tile            Open
//   window   an overview card or tab    Focus, Minimize, Close
//   notice   a toast                    Dismiss
//   text     a text field or selection  Cut, Copy, Paste, Select all
//   desktop  anything else in the shell Show desktop, Window overview, Launcher
//
// Failure modes:
//  V1 a node that already opened its own menu is not opened twice: this
//     listener runs on the bubble and skips a defaultPrevented event;
//  V2 Shift+right-click keeps the browser's own menu (spelling, inspect);
//  V3 a right-click outside the shell root (a package sandbox's iframe, the
//     renderer's own menu) is left alone;
//  V4 Paste without clipboard permission does nothing, never throws.
import { openContextMenu, type ContextMenuEntry, type ContextMenuTarget } from './context-menu';
import { toggleLauncher } from './shell-slots';
import { openApp, showDesktop, showOverview, windowAction } from './shell-services';

const FIELD = 'input:not([type]), input[type="text"], input[type="search"], input[type="url"], input[type="tel"], input[type="password"], textarea, [contenteditable=""], [contenteditable="true"]';

type Built = { items: ContextMenuEntry[]; label: string; target: ContextMenuTarget };

function textMenu(el: Element, field: HTMLElement | null): Built | null {
  const selection = String(window.getSelection?.()?.toString() || '');
  const input = field as (HTMLInputElement | HTMLTextAreaElement) | null;
  const selected = input && 'selectionStart' in input && input.selectionStart !== input.selectionEnd
    ? String(input.value || '').slice(input.selectionStart || 0, input.selectionEnd || 0)
    : selection;
  if (!field && !selected) return null;
  const editable = Boolean(field) && !(input as HTMLInputElement)?.readOnly && !(input as HTMLInputElement)?.disabled;
  const run = (cmd: string) => () => { field?.focus?.(); try { document.execCommand(cmd); } catch { /* V4 */ } };
  const items: ContextMenuEntry[] = [];
  if (editable) items.push({ id: 'cut', label: 'Cut', disabled: !selected, onSelect: run('cut') });
  items.push({ id: 'copy', label: 'Copy', disabled: !selected, onSelect: run('copy') });
  if (editable) {
    items.push({
      id: 'paste',
      label: 'Paste',
      onSelect: async () => {
        try {
          const text = await navigator.clipboard.readText();
          field?.focus?.();
          document.execCommand('insertText', false, text);
        } catch { /* V4 */ }
      },
    });
  }
  items.push({
    id: 'select-all',
    label: 'Select all',
    onSelect: () => {
      if (input && typeof input.select === 'function') { input.focus(); input.select(); return; }
      document.execCommand('selectAll');
    },
  });
  return { items, label: 'Text actions', target: { kind: 'text' } };
}

function windowMenu(windowId: string, appId: string, title: string): Built {
  return {
    label: `${title || appId || 'Window'} window`,
    target: { kind: 'window', name: appId },
    items: [
      { id: 'focus', label: 'Focus', onSelect: () => windowAction(windowId, 'focus') },
      { id: 'minimize', label: 'Minimize', essential: true, onSelect: () => windowAction(windowId, 'minimize') },
      { id: 'close', label: 'Close', essential: true, onSelect: () => windowAction(windowId, 'close') },
    ],
  };
}

/** What an unclaimed right-click on `el` should offer, or null outside the shell (V3). */
export function buildCoverageMenu(el: Element | null): Built | null {
  if (!el || !el.closest) return null;
  if (!el.closest('.desktop, [data-desktop-overview], [data-desk-sheet], [data-notifications]')) return null;
  const tile = el.closest('[data-desk-app-id]') as HTMLElement | null;
  if (tile) {
    const id = String(tile.getAttribute('data-desk-app-id') || '');
    const name = (tile.textContent || id).trim();
    return { label: `${name} actions`, target: { kind: 'app', name: id }, items: [{ id: 'open', label: 'Open', onSelect: () => openApp(id) }] };
  }
  const card = el.closest('[data-overview-card-window-id], [data-overview-map-window-id]') as HTMLElement | null;
  if (card) {
    const id = card.getAttribute('data-overview-card-window-id') || card.getAttribute('data-overview-map-window-id') || '';
    const app = card.getAttribute('data-overview-card-app-id') || card.getAttribute('data-overview-map-window-app-id') || '';
    return windowMenu(id, app, '');
  }
  const note = el.closest('[data-notification]') as HTMLElement | null;
  if (note) {
    return { label: 'Notification', target: { kind: 'notice' }, items: [{ id: 'dismiss', label: 'Dismiss', essential: true, onSelect: () => (note.querySelector('.toast-close') as HTMLElement | null)?.click() }] };
  }
  const text = textMenu(el, el.closest(FIELD) as HTMLElement | null);
  if (text) return text;
  return {
    label: 'Desktop actions',
    target: { kind: 'desktop' },
    items: [
      { id: 'show-desktop', label: 'Show desktop', onSelect: () => showDesktop() },
      { id: 'overview', label: 'Window overview', onSelect: () => showOverview() },
      { id: 'launcher', label: 'Launcher', onSelect: () => toggleLauncher() },
    ],
  };
}

/** Installs the fallback; returns the disposer. */
export function installContextCoverage(): () => void {
  if (typeof document === 'undefined') return () => {};
  const open = (event: MouseEvent | KeyboardEvent, x: number, y: number, el: Element | null) => {
    if (event.defaultPrevented) return; // V1
    const built = buildCoverageMenu(el);
    if (!built) return;
    if (openContextMenu({ x, y, items: built.items, label: built.label, target: built.target, returnFocus: document.activeElement as HTMLElement | null })) event.preventDefault();
  };
  const onContextMenu = (event: MouseEvent) => {
    if (event.shiftKey) return; // V2
    const fromKeyboard = event.button === -1 || (event.clientX === 0 && event.clientY === 0);
    const el = event.target as Element | null;
    if (fromKeyboard) {
      const r = (document.activeElement as HTMLElement | null)?.getBoundingClientRect?.();
      open(event, (r?.left ?? 16) + 8, (r?.bottom ?? 16), document.activeElement);
      return;
    }
    open(event, event.clientX, event.clientY, el);
  };
  document.addEventListener('contextmenu', onContextMenu);
  return () => document.removeEventListener('contextmenu', onContextMenu);
}
