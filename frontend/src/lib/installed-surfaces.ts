// Installed shell surfaces (plan section 11, Phase 5 surface mount; 12.1).
//
// A package that provides a shell surface (manifest `provides`, tier T2: a
// context-menu renderer such as menu-default) installs like any other: the
// computer builds its module/v1 module (internal/pkgruntime, surface mode:
// the module exports `surface(service, ctx)` and `surfaces`). This file mounts
// it. An installed release of a package the base image ships replaces the
// in-tree package of that name while installed, in the same slot, under the
// same provider id: the in-tree registration is displaced, the installed
// component is registered through the slot's own registrar, and the kernel
// ranks and watches it like any provider.
//
// Failure modes:
//  I1 a release that is not on the owner's own track (isolation is not
//     "shell"), whose module is missing, whose SDK range this shell does not
//     satisfy, or in recovery boot (?recovery=core): not mounted, the in-tree
//     package keeps serving;
//  I2 the module fails to load or verify, lacks `surface`, or a provided
//     service has no component: refused before anything is displaced; the
//     in-tree package keeps serving;
//  I3 the installed provider throws or fails its contract while serving (the
//     kernel's watchdog sets it aside, or its registration is refused): the
//     in-tree package is registered again at once (L4 failover); the release
//     is not mounted again until it changes (same install, same bytes);
//  I4 uninstall (or a platform yank): the installed registration is disposed
//     and the in-tree package returns (package revert = uninstall);
//  I5 a newer release replaces an installed one: the old registration is
//     disposed and the new one mounted, without the in-tree package between
//     them when the new one mounts.
import type { ComponentType } from 'svelte';
import { installedPackages, isSurfacePackage, loadPackageModule, type InstalledPackageRecord, type PackageModule } from './apps/packages';
import { sdkSatisfies } from './sdk-version';
import { hasInTreeSurface, recoveryBoot, registerSurfaceOf, removeSurfacePackage, restoreSurfacePackage, surfaceScope } from './shell-slots';
import { shellKernel } from './shell-kernel';
import { PackageLoadError } from './apps/packages';

type SurfaceModule = PackageModule & { surface?: (service: string, ctx: { sdk: unknown; package: string }) => unknown; release?: () => void };
export type SurfaceLoader = (record: InstalledPackageRecord) => Promise<SurfaceModule>;
type Provide = { service?: string; requires_sdk?: string; priority?: number };

export type InstalledSurfaceState = {
  package: string;
  release_id: string;
  slots: string[];
  state: 'active' | 'set-aside' | 'refused';
  reason: string;
};

type Live = { fingerprint: string; record: InstalledPackageRecord; module: SurfaceModule };
const live = new Map<string, Live>();
const setAside = new Map<string, { fingerprint: string; record: InstalledPackageRecord; reason: string }>();
const refused = new Map<string, { fingerprint: string; record: InstalledPackageRecord; reason: string }>();

const fingerprintOf = (r: InstalledPackageRecord) => `${r.install_id}|${r.module.sha256}`;
const providesOf = (r: InstalledPackageRecord): Provide[] => ((r.manifest as any)?.provides ?? []) as Provide[];

/** Where the installed surfaces stand, for Settings and the deployed proof. */
export function describeInstalledSurfaces(): InstalledSurfaceState[] {
  const row = (record: InstalledPackageRecord, state: InstalledSurfaceState['state'], reason: string): InstalledSurfaceState => ({
    package: record.name, release_id: record.release_id, slots: providesOf(record).map((p) => String(p.service)), state, reason,
  });
  return [
    ...[...live.values()].map((l) => row(l.record, 'active', '')),
    ...[...setAside.values()].map((s) => row(s.record, 'set-aside', s.reason)),
    ...[...refused.values()].map((s) => row(s.record, 'refused', s.reason)),
  ].sort((a, b) => a.package.localeCompare(b.package));
}

function eligible(r: InstalledPackageRecord): boolean {
  // Only a package the base image ships is displaced (mount is "in place of"); other names wait for their own admission.
  return isSurfacePackage(r) && r.isolation === 'shell' && r.available !== false && hasInTreeSurface(r.name);
}

let defaultSdk: Promise<unknown> | null = null;
async function shellSdk(): Promise<unknown> {
  // The SDK reaches a module only as ctx.sdk (M5). Read lazily: the SDK is heavy and a signed-out shell never needs it.
  defaultSdk ||= import('./app-sdk').then((m) => Object.freeze({ ...m }));
  return defaultSdk;
}

async function mount(record: InstalledPackageRecord, load: SurfaceLoader, sdk: () => Promise<unknown>, replaced: boolean): Promise<void> {
  let gone = replaced; // the in-tree package is not registered (displaced by this mount, or by the release it replaces)
  const name = record.name;
  const fingerprint = fingerprintOf(record);
  let module: SurfaceModule | null = null;
  // A refused mount leaves the slot to the in-tree package: restored when this
  // mount had already displaced it (a replace), and the module's CSS stops.
  const refuse = async (reason: string, transient = false) => {
    console.warn(`[shell] installed surface ${name} not mounted: ${reason}`);
    try { module?.release?.(); } catch { /* ignore */ }
    if (gone) {
      await removeSurfacePackage(name);
      if (!recoveryBoot) await restoreSurfacePackage(name);
    }
    if (!transient) refused.set(name, { fingerprint, record, reason });
  };
  if (!sdkSatisfies(record.requires_sdk)) return refuse(`needs SDK ${record.requires_sdk}`);
  const provides = providesOf(record);
  const components: { provide: Provide; component: ComponentType }[] = [];
  try {
    module = await load(record);
    if (typeof module.surface !== 'function') throw new Error('the module draws no surface');
    const ctx = { sdk: await sdk(), package: name };
    for (const provide of provides) {
      const component = module.surface(String(provide.service), ctx);
      if (!component || !['function', 'object'].includes(typeof component)) throw new Error(`no component for ${provide.service}`);
      components.push({ provide, component: component as ComponentType });
    }
  } catch (err) {
    // A fetch that failed (the network, a 5xx) is tried again at the next sync; a bad module is not.
    const transient = !(err instanceof PackageLoadError) || err.state === 'unavailable';
    return refuse((err as Error)?.message || String(err), transient);
  }

  // Displace the base image's package of the same name, then register the installed surfaces.
  await removeSurfacePackage(name);
  gone = true;
  const scope = surfaceScope(name);
  try {
    for (const { provide, component } of components) {
      registerSurfaceOf({
        name, slot: String(provide.service), component, requires: String(provide.requires_sdk || '^1.0'),
        priority: Number.isFinite(provide.priority) ? Number(provide.priority) : 0, installed: true,
      }, scope);
    }
  } catch (err) {
    return refuse((err as Error)?.message || String(err));
  }
  // The registrars swallow what they refuse (a slot the shell lacks, a missing
  // component, an SDK out of range): every provide must be in the kernel's list.
  const services = (shellKernel.services.describe().services || {}) as Record<string, { providers: { id: string }[] }>;
  const missing = components.filter(({ provide }) => !services[String(provide.service)]?.providers.some((p) => p.id === name));
  if (missing.length) return refuse(`${missing.map((m) => m.provide.service).join(', ')} did not register`);
  refused.delete(name);
  setAside.delete(name);
  live.set(name, { fingerprint, record, module });
}

async function unmount(name: string, restore: boolean): Promise<void> {
  const entry = live.get(name);
  live.delete(name);
  await removeSurfacePackage(name);
  try { entry?.module.release?.(); } catch { /* the module's style stays at worst */ }
  if (restore && hasInTreeSurface(name) && !recoveryBoot) await restoreSurfacePackage(name);
}

let queue: Promise<void> = Promise.resolve();

/**
 * Bring the mounted surfaces in line with the installed set. Serialized: each
 * call runs after the one before.
 */
export function syncInstalledSurfaces(
  records: InstalledPackageRecord[],
  options: { load?: SurfaceLoader; sdk?: () => Promise<unknown> } = {},
): Promise<void> {
  const load: SurfaceLoader = options.load ?? ((r) => loadPackageModule(r) as Promise<SurfaceModule>);
  const sdk = options.sdk ?? shellSdk;
  const run = async () => {
    const want = new Map(records.filter(eligible).map((r) => [r.name, r]));
    for (const map of [setAside, refused]) {
      for (const [name, entry] of [...map]) {
        const rec = want.get(name);
        if (!rec || fingerprintOf(rec) !== entry.fingerprint) map.delete(name);
      }
    }
    const replaced = new Set<string>();
    for (const [name, entry] of [...live]) {
      const rec = want.get(name);
      if (!rec) await unmount(name, true); // I4
      else if (fingerprintOf(rec) !== entry.fingerprint) { await unmount(name, false); replaced.add(name); } // I5
    }
    if (recoveryBoot) return; // I1
    for (const [name, rec] of want) {
      if (live.has(name)) continue;
      const fingerprint = fingerprintOf(rec);
      if (setAside.get(name)?.fingerprint === fingerprint || refused.get(name)?.fingerprint === fingerprint) continue; // I3
      await mount(rec, load, sdk, replaced.has(name));
    }
  };
  queue = queue.then(run, run).catch((err) => { console.warn('[shell] installed surfaces:', err); });
  return queue;
}

// I3: a displacing installed provider the kernel set aside (the watchdog, a
// refused activation, the owner's "Use default menu", a yank) gives the slot
// back to the in-tree package. Every set-aside path reaches this listener.
shellKernel.services.onTrip((slot, id, reason) => {
  const entry = live.get(id);
  if (!entry || !providesOf(entry.record).some((p) => p.service === slot)) return;
  live.delete(id);
  setAside.set(id, { fingerprint: entry.fingerprint, record: entry.record, reason });
  queue = queue.then(async () => {
    await removeSurfacePackage(id);
    try { entry.module.release?.(); } catch { /* ignore */ }
    if (hasInTreeSurface(id) && !recoveryBoot) await restoreSurfacePackage(id);
  }).catch((err) => { console.warn('[shell] installed surface failover:', err); });
});

installedPackages.subscribe((records) => {
  // The first call (the empty initial set) changes nothing.
  if (records.length === 0 && live.size === 0 && setAside.size === 0 && refused.size === 0) return;
  void syncInstalledSurfaces(records);
});
