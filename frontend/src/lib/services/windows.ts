// Window management as a service (slice MA5.3; plan section 5, slot "windows").
//
// The contract a windows provider meets, and the API W1 put in the SDK as its
// verbs (focus, restore, windowAction, listWindows, showOverview, showDesktop,
// suspendBackgroundHeavyWindows, isHeavyAppId). The window host (geometry,
// z-order, saved windows, Desktop.svelte over stores/desktop.js) stays the
// shell's: its base provider, "shell", is the one implementation, always there,
// tier core. A package that provides "windows" is held to the same contract and
// ranks over the base by priority, and "Use default" returns to the base, like
// any slot (Settings, "Who provides this?").
//
// Failure modes:
//  W1 a provider whose impl misses a verb: refused when it registers, the base
//     keeps serving;
//  W2 a provider that throws on a call: that call is dropped with a console
//     note (the SDK verbs never throw into an app);
//  W3 a provider id that is not a lowercase word, or "shell" (the host's): refused.
import type { Scope } from '../kernel';
import { shellKernel } from '../shell-kernel';
import { focusWindow, isHeavyAppId, restoreWindow, suspendBackgroundHeavyWindows } from '../stores/desktop.js';
import { listWindows, showDesktop, showOverview, windowAction, type WindowAction, type WindowSummary } from '../shell-services';

export const WINDOWS_SLOT = 'windows';
export const WINDOWS_ABI = 'choir.service/windows/v1';
export const BASE_WINDOWS_ID = 'shell';

/** What a windows provider implements: W1's verbs. */
export interface WindowsService {
  list(): WindowSummary[];
  focus(windowId: string): void;
  restore(windowId: string): void;
  action(windowId: string, action: WindowAction): void;
  overview(): void;
  desktop(): void;
  suspendBackgroundHeavy(): void;
  isHeavy(appId: string): boolean;
}

export const WINDOWS_VERBS = ['list', 'focus', 'restore', 'action', 'overview', 'desktop', 'suspendBackgroundHeavy', 'isHeavy'] as const;

const ID = /^[a-z][a-z0-9-]{0,63}$/;
const root = shellKernel.root;
shellKernel.services.define(root, WINDOWS_SLOT, WINDOWS_ABI);

/** The first contract problem with a provider, or null. */
export function windowsContractError(spec: { id?: unknown; impl?: unknown }): string | null {
  if (!spec || typeof spec !== 'object') return 'a windows provider is { id, impl }';
  if (typeof spec.id !== 'string' || !ID.test(spec.id)) return `windows provider id "${String(spec.id)}" must be a lowercase word`;
  if (spec.id === BASE_WINDOWS_ID) return `windows provider id "${BASE_WINDOWS_ID}" is the shell's`;
  const impl = spec.impl as Record<string, unknown> | undefined;
  for (const verb of WINDOWS_VERBS) {
    if (!impl || typeof impl[verb] !== 'function') return `windows provider ${spec.id}: impl.${verb} must be a function`;
  }
  return null;
}

const baseImpl: WindowsService = {
  list: () => listWindows(),
  focus: (id) => focusWindow(id),
  restore: (id) => restoreWindow(id),
  action: (id, action) => windowAction(id, action),
  overview: () => showOverview(),
  desktop: () => showDesktop(),
  suspendBackgroundHeavy: () => suspendBackgroundHeavyWindows(),
  isHeavy: (appId) => Boolean(isHeavyAppId(appId)),
};

shellKernel.services.provide(root, WINDOWS_SLOT, { id: BASE_WINDOWS_ID, tier: 'core', priority: 0, base: true, impl: baseImpl });

/** A package provides windows. Throws (the base keeps serving) when the contract is not met. Returns the disposer. */
export function registerWindowsProvider(scope: Scope, spec: { id: string; impl: WindowsService; priority?: number; tier?: 'policy' | 'user' | 'app' | 'core' }): () => void {
  const bad = windowsContractError(spec);
  if (bad) throw new Error(bad);
  return shellKernel.services.provide(scope, WINDOWS_SLOT, { id: spec.id, impl: spec.impl, tier: spec.tier ?? 'app', priority: spec.priority });
}

/** The active windows provider (the base when nothing ranks over it). */
export function windowsService(): WindowsService {
  return shellKernel.services.resolve<WindowsService>(WINDOWS_SLOT) ?? baseImpl;
}

/** Calls one verb on the active provider; a throwing provider drops the call (W2). */
export function callWindows<K extends keyof WindowsService>(verb: K, ...args: Parameters<WindowsService[K]>): ReturnType<WindowsService[K]> | undefined {
  try {
    return (windowsService()[verb] as (...a: unknown[]) => ReturnType<WindowsService[K]>)(...args);
  } catch (err) {
    console.warn(`[windows] ${String(verb)} failed:`, err);
    return undefined;
  }
}
