// Selection records and "Who provides this?" (slice L2;
// docs/design/modular-architecture-2026-11-04.md section 2.4, step 2 of the
// resolution order, and decisions 4 and 11).
//
// The owner's choice of provider for a single-slot shell service is a record
// on the owner's tape: the preference packages.selection,
// {"<slot>": {package, release, at}} (internal/apppackage/selection.go). The
// shell reads it at sign-in and hands each choice to the kernel
// (services.select), which ranks the chosen provider first while it is loaded
// and healthy. "Use default" selects the slot's base-image package: nothing is
// uninstalled. The Settings app shows each slot's providers over the kernel's
// snapshot (who draws it, who was chosen, who was set aside and why).
//
// Failure modes:
//  P1 the record names a package that is not installed or not healthy: the
//     kernel passes it over (the next candidate or the base serves); the
//     record stays, so the choice returns with the package;
//  P2 a signed-out shell, a failed read, a value this shell cannot read: no
//     selection (the default order), never a throw;
//  P3 a write the server refuses: { ok: false, error }, and the kernel keeps
//     the choice in force before it (nothing is applied before the record);
//  P4 another window's write (live event setting.updated): this shell
//     applies the new set, its own echo ignored;
//  P5 sign-out: every applied choice is withdrawn from the kernel, so the
//     next owner starts from the default order;
//  P6 two writes race (Settings in two windows, a choice and a yank): a write
//     carries only the slots it changes and the computer merges it into the
//     stored set (internal/mediastate handleSelections), so one slot's write
//     never clobbers another's; the shell applies the merged set it returns;
//  P7 the platform yanks (or force-replaces) a release that draws or was
//     chosen for a slot (slice L4): at the next sync the provider is set aside
//     in the kernel with the platform's reason, and the slot's base-image
//     package is recorded as the choice, so the slot falls back to the base,
//     never to another third party's provider by priority.
import { get } from 'svelte/store';
import type { Disposer, Scope } from '../kernel';
import { shellKernel } from '../shell-kernel';
import { RECOVERY_SLOTS, inTreeSurfaces } from '../shell-slots';
import { installedPackages, onPlatformActs, type PlatformAct } from '../apps/packages';
import { announceFailover } from './failover';
import { fetchWithRenewal } from '../auth.js';
import { addLiveEventListener, currentDeviceId, isOwnLiveEvent, liveEventKind, liveEventPayload } from '../live-events.js';

const KEY = 'packages.selection';
const URL = `/api/preferences/${KEY}`;
const NAME = /^[a-z][a-z0-9-]{1,31}$/;
const SHA256 = /^[0-9a-f]{64}$/;
/** The host's last-resort renderer (services/context-menu.ts BASE_RENDERER_ID): never a choice. */
const LAST_RESORT = 'last-resort';

type SelectionRecord = { package: string; release?: string; at: string };
type Records = Record<string, SelectionRecord>;

export type ProviderRow = { id: string; healthy: boolean; reason: string | null; isDefault: boolean };
export type ProviderSlot = {
  slot: string;
  /** Who draws the slot now (null: the host's last resort). */
  active: string | null;
  /** The owner's recorded choice, or null. */
  selected: string | null;
  defaultPackage: string;
  lastResort: boolean;
  providers: ProviderRow[];
};
export type SelectionResult = { ok: boolean; error?: string };

// Read at call time: shell-slots.ts may still be evaluating when this module is (an import cycle through the loader).
const slotList = (): readonly string[] => RECOVERY_SLOTS;
let records: Records = {};
let scope: Scope | null = null;
const applied = new Map<string, Disposer>();

function readRecords(value: unknown): Records {
  const out: Records = {};
  if (!value || typeof value !== 'object' || Array.isArray(value)) return out;
  for (const [slot, rec] of Object.entries(value as Record<string, any>)) {
    if (!slotList().includes(slot) || !rec || typeof rec !== 'object') continue;
    if (!NAME.test(String(rec.package || ''))) continue;
    const release = typeof rec.release === 'string' && SHA256.test(rec.release) ? rec.release : undefined;
    out[slot] = { package: rec.package, ...(release ? { release } : {}), at: String(rec.at || '') };
  }
  return out;
}

function selectionScope(): Scope {
  if (!scope || scope.disposed) scope = shellKernel.scope('selection');
  return scope;
}

/** Hand the records to the kernel: one select per slot, replacing the one before. */
async function apply(next: Records): Promise<void> {
  records = next;
  for (const slot of slotList()) {
    const want = next[slot]?.package;
    const prev = applied.get(slot);
    if (prev) {
      applied.delete(slot);
      await prev();
    }
    if (want) applied.set(slot, shellKernel.services.select(selectionScope(), slot, want));
  }
  await shellKernel.services.settled();
}

let listening = false;
function listen(): void {
  if (listening) return;
  listening = true;
  addLiveEventListener((message: any) => {
    if (liveEventKind(message) !== 'setting.updated' || isOwnLiveEvent(message)) return;
    const { key, value } = (liveEventPayload(message) || {}) as { key?: string; value?: unknown };
    if (key === KEY) void apply(readRecords(value));
  });
}

let loading: Promise<void> | null = null;

/** Sign-in: read the owner's records and apply them (P2: any failure leaves the default order). */
export function loadProviderSelections(): Promise<void> {
  loading = readSelections();
  return loading;
}

async function readSelections(): Promise<void> {
  listen();
  try {
    const res = await fetchWithRenewal(URL, { method: 'GET', cache: 'no-store' });
    if (!res.ok) return;
    const body = await res.json();
    await apply(readRecords(body?.value));
  } catch {
    // P2: signed out or unreachable: the kernel's default order stands.
  }
}

/** Sign-out (P5): the previous owner's choices leave the kernel. */
export async function clearProviderSelections(): Promise<void> {
  await apply({});
}

function defaultPackageOf(slot: string): string {
  return inTreeSurfaces().find((s) => s.slot === slot)?.name || '';
}

function releaseOf(pkg: string): string | undefined {
  const rec = get(installedPackages).find((r) => r.name === pkg);
  return rec && SHA256.test(String(rec.release_id || '')) ? rec.release_id : undefined;
}

function recordFor(pkg: string): SelectionRecord {
  const release = releaseOf(pkg);
  return { package: pkg, ...(release ? { release } : {}), at: new Date().toISOString() };
}

/** Write only the slots that change (P6); the computer merges them and returns the whole set, which is applied. */
async function writeSlots(changes: Records): Promise<SelectionResult> {
  let merged: Records;
  try {
    const res = await fetchWithRenewal(URL, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json', 'X-Choir-Device': currentDeviceId() },
      body: JSON.stringify({ value: changes }),
    });
    const body = await res.json().catch(() => ({}));
    if (!res.ok) return { ok: false, error: String(body?.error || `the selection was not saved (${res.status})`) };
    merged = body && typeof body.value === 'object' && body.value ? readRecords(body.value) : { ...records, ...changes };
  } catch (err) {
    return { ok: false, error: (err as Error)?.message || 'the selection was not saved' };
  }
  await apply(merged);
  return { ok: true };
}

/** Record the owner's choice of provider for a slot, then rank it first (P3: nothing is applied unless the record is). */
export async function selectProvider(slot: string, pkg: string): Promise<SelectionResult> {
  if (!slotList().includes(slot)) return { ok: false, error: `"${slot}" is not a shell slot` };
  if (!NAME.test(String(pkg || '')) || pkg === LAST_RESORT) return { ok: false, error: `"${pkg}" is not a package` };
  return writeSlots({ [slot]: recordFor(pkg) });
}

/**
 * P7: the platform's acts from the update sync reach the kernel. Every slot
 * the acted-on package draws or was chosen for falls back to its base-image
 * package (recorded, so the fallback survives a reload), and the package's
 * registration is set aside with the platform's reason.
 */
export async function fallBackFromPlatformActs(acts: PlatformAct[]): Promise<SelectionResult> {
  if (loading) await loading.catch(() => undefined);
  await shellKernel.services.settled();
  const services = (shellKernel.services.describe().services || {}) as Record<string, { active: string | null; providers: { id: string }[] }>;
  const changes: Records = {};
  const tripped: [string, string, string][] = [];
  for (const act of acts) {
    const why = act.state === 'removed_by_platform'
      ? `yanked by the platform${act.detail ? `: ${act.detail}` : ''}`
      : `replaced by the platform${act.detail ? `: ${act.detail}` : ''}`;
    for (const slot of slotList()) {
      const base = defaultPackageOf(slot);
      if (!base || act.package === base) continue;
      const drew = services[slot]?.active === act.package;
      const chosen = records[slot]?.package === act.package;
      if (drew || chosen) changes[slot] = recordFor(base);
      if (services[slot]?.providers.some((p) => p.id === act.package)) tripped.push([slot, act.package, why]);
    }
  }
  for (const [slot, id, why] of tripped) {
    if (shellKernel.services.trip(slot, id, why)) announceFailover(slot, id, why);
  }
  if (Object.keys(changes).length === 0) {
    await shellKernel.services.settled();
    return { ok: true };
  }
  return writeSlots(changes);
}

onPlatformActs((acts) => { void fallBackFromPlatformActs(acts); });

/** "Use default": select the slot's base-image package; nothing is uninstalled. */
export async function useDefaultProvider(slot: string): Promise<SelectionResult> {
  const base = defaultPackageOf(slot);
  if (!base) return { ok: false, error: `the ${slot} slot has no default package` };
  return selectProvider(slot, base);
}

/** Who provides each slot, from the kernel's snapshot, once pending changes have settled. */
export async function providerSlots(): Promise<ProviderSlot[]> {
  await shellKernel.services.settled();
  const snap = shellKernel.snapshot() as any;
  const services = (snap.services || {}) as Record<string, { active: string | null; providers: { id: string; healthy: boolean; tripped: string | null }[] }>;
  const selection = (snap.selection || {}) as Record<string, string>;
  return slotList().map((slot) => {
    const defaultPackage = defaultPackageOf(slot);
    const raw = services[slot];
    const active = raw?.active && raw.active !== LAST_RESORT ? raw.active : null;
    return {
      slot,
      active,
      selected: records[slot]?.package ?? selection[slot] ?? null,
      defaultPackage,
      lastResort: active === null,
      providers: (raw?.providers ?? [])
        .filter((p) => p.id !== LAST_RESORT)
        .map((p) => ({ id: p.id, healthy: p.healthy, reason: p.healthy ? null : p.tripped || 'set aside after it failed', isDefault: p.id === defaultPackage })),
    };
  });
}
