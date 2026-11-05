// The open windows as a store (SDK 1.4.0): what the Compute Monitor lists and
// follows. listWindows answers once; this follows changes. Read-only: acting on
// a window goes through focus, restore, windowAction and the other W1 verbs.
import { derived, type Readable } from 'svelte/store';
import { activeWindowId, windows } from './stores/desktop.js';

export type WindowEntry = {
  windowId: string;
  appId: string;
  title: string;
  icon: string;
  mode: string;
  focused: boolean;
  restoreSuspended: boolean;
};

export const windowList: Readable<WindowEntry[]> = derived([windows, activeWindowId], ([$windows, $active]) =>
  (($windows as any[]) || [])
    .filter((win) => win.mode !== 'closed' && win.mode !== 'hidden')
    .map((win) => ({
      windowId: String(win.windowId),
      appId: String(win.appId),
      title: String(win.title || ''),
      icon: String(win.icon || ''),
      mode: String(win.mode),
      focused: win.windowId === $active,
      restoreSuspended: Boolean(win.restoreSuspended),
    })),
);
