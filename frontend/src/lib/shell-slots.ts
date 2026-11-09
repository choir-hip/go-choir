// The shell surfaces as service slots (plan section 11, slices H1 to H4;
// docs/design/modular-architecture-2026-11-04.md sections 1 and 4b).
//
// The launcher, the desktop icons, the overview, the prompt bar and the
// notifications are packages: each draws one slot of the shell, chosen by the
// kernel's ranking among the packages that provide it (manifest `provides`,
// internal/apppackage/provides.go). The host keeps the window manager, the key
// and context-menu capture, and, for a slot with no healthy provider, a
// last-resort surface (ShellSlot's default content), so a broken or absent
// surface package never locks the owner out.
//
// The slot ABI: a surface is a Svelte component. The host gives it what the
// host knows as props (who is signed in, the placement, the window list, the
// slot's own open state) and listens for the slot's own events (close, and the
// window-manager verbs of the overview). Everything else a surface asks of
// the shell goes through the SDK (openApp, windowAction, toast, ...), like any
// app. A surface imports only the SDK: the package boundary holds for it too.
//
// Failure modes:
//  S1 a provides entry names a slot the shell does not have, or its component
//     is missing: skipped with a console note; the slot keeps its other
//     providers or the last-resort surface;
//  S2 a surface whose requires_sdk this shell does not satisfy: skipped;
//  S3 a surface that throws while mounting: reported, taken out of the
//     ranking, and the next provider (or the last resort) renders at once;
//  S4 the package leaves (its scope is disposed): its surface unmounts and the
//     slot falls to the next provider or the last resort;
//  S5 no provider at all at boot: the last resort shows once registration has
//     settled, never as a flash before the packages have registered;
//  S6 a broken surface package hides the fix (slice L4): the boot recovery
//     entry, ?recovery=core, registers no surface package for that session,
//     so every slot shows the host's last resort and the recovery surface
//     opens; nothing about it is stored, and a reload without it is the
//     normal shell;
//  S7 a failover nobody sees: the provider set aside keeps its reason in the
//     kernel's snapshot (the recovery surface and Settings show it) and the
//     owner gets one toast (decision 4), not one per failure.
import type { ComponentType } from 'svelte';
import { readable, writable, type Readable } from 'svelte/store';
import type { Scope } from './kernel';
import { shellKernel } from './shell-kernel';
import { sdkSatisfies } from './sdk-version';
import { announceFailover } from './services/failover';

export const SURFACE_SLOTS = ['launcher', 'desktop.icons', 'overview', 'prompt', 'notifications.host'] as const;
export type SurfaceSlot = (typeof SURFACE_SLOTS)[number];
export type ActiveSurface = { id: string; component: ComponentType };
export type SurfaceSpec = { id: string; component: ComponentType; priority?: number; base?: boolean };

const abi = (slot: string) => `choir.service/${slot}/v1`;
const ID = /^[a-z][a-z0-9-]{0,63}$/;
const isSlot = (s: string): s is SurfaceSlot => (SURFACE_SLOTS as readonly string[]).includes(s);

/**
 * The boot recovery entry (S6): ?recovery=core loads only the host's
 * last-resort surfaces for this page load. Read once, from the URL, never
 * stored.
 */
export const recoveryBoot: boolean = (() => {
  try {
    return typeof location !== 'undefined' && new URLSearchParams(location.search).get('recovery') === 'core';
  } catch {
    return false;
  }
})();

const root = shellKernel.root;
for (const slot of SURFACE_SLOTS) shellKernel.services.define(root, slot, abi(slot));

const active = writable<Partial<Record<SurfaceSlot, ActiveSurface>>>({});
/** The surface each slot shows now (absent: the last resort). */
export const activeSurfaces: Readable<Partial<Record<SurfaceSlot, ActiveSurface>>> = { subscribe: active.subscribe };

function setActive(slot: SurfaceSlot, surface: ActiveSurface | null, onlyIf?: string): void {
  active.update((m) => {
    if (onlyIf !== undefined && m[slot]?.id !== onlyIf) return m;
    const next = { ...m };
    if (surface) next[slot] = surface;
    else delete next[slot];
    return next;
  });
}

/** A package provides a surface for a slot. Throws on a bad spec (S1). Returns the disposer. */
export function registerSurface(scope: Scope, slot: string, spec: SurfaceSpec): () => void {
  if (!isSlot(slot)) throw new Error(`"${slot}" is not a shell slot (${SURFACE_SLOTS.join(', ')})`);
  if (!spec || typeof spec.id !== 'string' || !ID.test(spec.id)) throw new Error(`surface id "${String(spec?.id)}" must be a package name`);
  // A component is a constructor in the browser (an object in a server render).
  if (!spec.component || !['function', 'object'].includes(typeof spec.component)) throw new Error(`surface ${spec.id} for ${slot}: the component is missing`);
  const surface: ActiveSurface = { id: spec.id, component: spec.component };
  const dispose = shellKernel.services.provide(scope, slot, {
    id: spec.id,
    impl: surface,
    tier: 'core',
    priority: spec.priority ?? 0,
    base: spec.base ?? false,
    activate: (act) => {
      setActive(slot, surface);
      act.effect(() => setActive(slot, null, spec.id));
    },
  });
  return () => { void dispose(); };
}

/** A surface failed to mount (S3): out of the ranking for this registration, with the reason. */
export function reportSurfaceFailure(slot: SurfaceSlot, id: string, reason: string): void {
  const why = `failed to mount: ${reason}`;
  if (shellKernel.services.trip(slot, id, why)) announceFailover(slot, id, why);
  setActive(slot, null, id);
}

/** Which package each slot's surface comes from (the recovery surface lists it). */
export function describeSlots(): { slot: SurfaceSlot; active: string | null; providers: string[] }[] {
  const services = (shellKernel.services.describe().services || {}) as Record<string, { active: string | null; providers: { id: string }[] }>;
  return SURFACE_SLOTS.map((slot) => ({
    slot,
    active: services[slot]?.active ?? null,
    providers: (services[slot]?.providers ?? []).map((p) => p.id),
  }));
}

// ---- the in-tree surface packages ----

type ProvidesEntry = { service?: string; entry?: string; requires_sdk?: string; priority?: number };
const manifests = import.meta.glob('../packages/*/choir-package.json', { eager: true, import: 'default' }) as Record<string, { name?: string; provides?: ProvidesEntry[] }>;
const components = import.meta.glob('../packages/*/*.surface.svelte', { eager: true, import: 'default' }) as Record<string, ComponentType>;

/** The in-tree surfaces, by slot: the base image's default set (S1, S2). */
export function inTreeSurfaces(): { name: string; slot: string; component: ComponentType | undefined; requires: string; priority: number }[] {
  const out = [];
  for (const [path, manifest] of Object.entries(manifests).sort(([a], [b]) => a.localeCompare(b))) {
    if (!Array.isArray(manifest?.provides) || typeof manifest.name !== 'string') continue;
    const dir = path.replace(/choir-package\.json$/, '');
    for (const p of manifest.provides) {
      out.push({
        name: manifest.name,
        slot: String(p?.service || ''),
        component: components[`${dir}${String(p?.entry || '')}`],
        requires: String(p?.requires_sdk || '^1.0'),
        priority: Number.isFinite(p?.priority) ? Number(p.priority) : 0,
      });
    }
  }
  return out;
}

// A package's surfaces live in their own scope beside its contributions' (the
// contributes front door disposes and re-applies the package scope when its
// set changes; a surface must not go with it).
const surfaceScopes = new Map<string, Scope>();
/** The scope a package's surfaces live in (context-menu.ts registers renderer packages in it). */
export function surfaceScope(name: string): Scope {
  const existing = surfaceScopes.get(name);
  if (existing && !existing.disposed) return existing;
  const scope = shellKernel.scope(`surface:${name}`);
  surfaceScopes.set(name, scope);
  return scope;
}

/** A surface package leaves (uninstall; the recovery drill): its surfaces go with it (S4). */
export async function removeSurfacePackage(name: string): Promise<void> {
  const scope = surfaceScopes.get(name);
  surfaceScopes.delete(name);
  if (scope) await scope.dispose();
}

/** A surface of a package: the in-tree set, or an installed release that displaces it (installed-surfaces.ts). */
export type InTreeSurface = ReturnType<typeof inTreeSurfaces>[number] & { installed?: boolean };
type Registrar = (surface: InTreeSurface, scope: Scope) => void;

// A slot surface of the in-tree set, registered as the slot's base-image default.
function registerSlotSurface(s: InTreeSurface, scope: Scope): void {
  if (!isSlot(s.slot)) return; // contextMenu.renderer registers through its own service (menu-host.ts)
  if (!sdkSatisfies(s.requires)) {
    console.warn(`[shell] ${s.name}: needs SDK ${s.requires}; its ${s.slot} surface is skipped`);
    return;
  }
  try {
    registerSurface(scope, s.slot, { id: s.name, component: s.component as ComponentType, priority: s.priority, base: !s.installed });
  } catch (err) {
    console.warn(`[shell] ${s.name}: ${(err as Error).message}`);
  }
}

const registrars: Registrar[] = [registerSlotSurface];
/** Another slot's registration of the in-tree surfaces (menu-host.ts, for the renderer), run again on a restore. */
export function addSurfaceRegistrar(fn: Registrar): void {
  registrars.push(fn);
}

// S6: the recovery boot registers none; the recovery surface's "Use default" still can.
if (!recoveryBoot) for (const s of inTreeSurfaces()) registerSlotSurface(s, surfaceScope(s.name));

/** An installed release's surface, registered through every slot's registrar like an in-tree one (installed-surfaces.ts). */
export function registerSurfaceOf(s: InTreeSurface, scope: Scope): void {
  for (const register of registrars) register(s, scope);
}

/** Whether the base image ships a surface package of this name (an installed release of it displaces it). */
export function hasInTreeSurface(name: string): boolean {
  return inTreeSurfaces().some((s) => s.name === name);
}

/**
 * The recovery surface's "use default": the in-tree package's surfaces are
 * registered again, healthy, in a fresh scope (a failed provider stays tripped
 * until its registration is disposed and provided again; kernel V3).
 */
export async function restoreSurfacePackage(name: string): Promise<void> {
  const surfaces = inTreeSurfaces().filter((s) => s.name === name);
  if (surfaces.length === 0) throw new Error(`${name} is not a surface package of this shell`);
  await removeSurfacePackage(name);
  const scope = surfaceScope(name);
  for (const s of surfaces) for (const register of registrars) register(s, scope);
  await shellKernel.services.settled();
}

/** The slots the recovery surface lists: the surface slots and the context-menu renderer. */
export const RECOVERY_SLOTS = [...SURFACE_SLOTS, 'contextMenu.renderer'] as const;
export type RecoveryRow = { slot: string; active: string | null; lastResort: boolean; defaultPackage: string; failed: { id: string; reason: string }[] };

/** For each slot: who draws it, whether the host's last resort does, its default package and the providers set aside. */
export function describeRecovery(): RecoveryRow[] {
  const services = (shellKernel.services.describe().services || {}) as Record<string, { active: string | null; providers: { id: string; healthy: boolean; tripped: string | null }[] }>;
  const tree = inTreeSurfaces();
  return RECOVERY_SLOTS.map((slot) => {
    const active = services[slot]?.active ?? null;
    return {
      slot,
      active,
      lastResort: active === null || active === 'last-resort',
      defaultPackage: tree.find((s) => s.slot === slot)?.name || '',
      failed: (services[slot]?.providers ?? []).filter((p) => !p.healthy).map((p) => ({ id: p.id, reason: p.tripped || 'set aside' })),
    };
  });
}

// ---- the launcher's open state (the host's: the prompt bar opens it, Cmd+K toggles it) ----

export type LauncherState = { open: boolean; restoreFocus: boolean };
const launcher = writable<LauncherState>({ open: false, restoreFocus: false });
/** Whether the launcher is open, and whether closing it hands the focus back to the button that opened it. */
export const launcherState: Readable<LauncherState> = { subscribe: launcher.subscribe };

export function setLauncherOpen(open: boolean, { restoreFocus = false }: { restoreFocus?: boolean } = {}): void {
  launcher.set({ open: Boolean(open), restoreFocus: !open && restoreFocus });
}

export function toggleLauncher(): void {
  let open = false;
  launcher.update((s) => { open = !s.open; return { open, restoreFocus: false }; });
}

/** True once the boot registrations have settled (S5: the last resort never flashes before them). */
export const surfacesSettled: Readable<boolean> = readable(false, (set) => {
  let live = true;
  void shellKernel.services.settled().then(() => { if (live) set(true); });
  return () => { live = false; };
});
