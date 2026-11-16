// The crash hold as reactive state (lib/crash-guard.js). `crashHeldMap` mirrors
// the guard's durable held map ({'w:<windowId>': count, 'a:<app>|<route>': count});
// `crashReopened` is page-local: windows the user Reopened in this page. A
// window's body may mount iff crashHoldFor(win, ...) is 0, so no code path has
// to remember to stamp a hold on a window object.
import { writable, get } from 'svelte/store';
import { crashHoldFor } from '../crash-guard.js';

export const crashHeldMap = writable({});
export const crashReopened = writable(new Set());

/** The hold count that keeps this window's body from mounting (0: it may mount). */
export function currentCrashHold(win) {
  return crashHoldFor(win, get(crashHeldMap), get(crashReopened));
}

export function markCrashReopened(windowId) {
  crashReopened.update((set) => new Set(set).add(windowId));
}

export function forgetCrashReopened(windowId) {
  crashReopened.update((set) => {
    if (!set.has(windowId)) return set;
    const next = new Set(set);
    next.delete(windowId);
    return next;
  });
}

export { crashHoldFor };
