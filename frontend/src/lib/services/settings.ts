// Package settings (slice S6; docs/design/modular-architecture-2026-11-04.md
// sections 2.2 and 2.6). A package declares `contributes.settings`: sections of
// fields { key, type, default, label, enum? }. The shell stores the owner's
// values as preferences named pkg.<package>.<key> through
// /api/preferences/{key} (the shell custodian: internal/mediastate), and the
// base Settings app renders every section from this schema. A package reads and
// writes its own values with getSetting / setSetting / onSetting.
//
// Failure modes:
//  S1 a section or field the schema refuses (bad key, type, default, enum):
//     that section is skipped with a console note, the package's other
//     contributions still apply;
//  S2 two packages declaring one key: they cannot collide, the preference name
//     carries the package; one package declaring a key twice: the second is
//     refused;
//  S3 a read before the schema is known, a failed request, a signed-out owner:
//     the declared default (or undefined for an unknown key), never a throw;
//  S4 a write the field refuses (type, enum, size): { ok: false, error },
//     nothing is sent, nothing cached, nobody notified;
//  S5 a write the server refuses or cannot take: { ok: false, error }, the
//     cache keeps the last good value;
//  S6 another window's write (live event setting.updated): the cache follows
//     and onSetting listeners hear it; this window's own echo is ignored;
//  S7 uninstall: the package's sections leave the Settings app and its
//     listeners are dropped with its scope; the stored values stay on the
//     computer (data, not code) and the package's key is refused by the server
//     until it is installed again.
import { writable, get } from 'svelte/store';
import { registerContributionPoint } from '../contributions';
import { fetchWithRenewal } from '../auth.js';
import { addLiveEventListener, currentDeviceId, isOwnLiveEvent, liveEventKind, liveEventPayload } from '../live-events.js';

export type SettingType = 'boolean' | 'string' | 'number';
export type SettingField = { key: string; type: SettingType; default: unknown; label: string; enum?: string[] };
export type SettingsSection = { section: string; fields: SettingField[] };
export type PackageSettingsSection = { pkg: string; section: string; fields: SettingField[] };
export type SettingResult = { ok: boolean; error?: string };

const KEY = /^[a-z][a-z0-9_-]{0,39}$/;
const MAX_STRING = 1024;
const MAX_FIELDS = 32;

/** The sections packages contribute, in install order; the Settings app renders them. */
export const settingSections = writable<PackageSettingsSection[]>([]);

const cache = new Map<string, unknown>();
const listeners = new Map<string, Set<(value: unknown) => void>>();
const prefKey = (pkg: string, key: string) => `pkg.${pkg}.${key}`;

/** Why a value does not fit the field, or null (the same rules as apppackage.SettingField.Check). */
export function settingValueError(field: SettingField, value: unknown): string | null {
  switch (field.type) {
    case 'boolean':
      return typeof value === 'boolean' ? null : 'must be a boolean';
    case 'number':
      return typeof value === 'number' && Number.isFinite(value) ? null : 'must be a finite number';
    case 'string':
      if (typeof value !== 'string') return 'must be a string';
      if ([...value].length > MAX_STRING) return `is over ${MAX_STRING} characters`;
      if (field.enum?.length && !field.enum.includes(value)) return 'must be one of the declared options';
      return null;
    default:
      return `type ${String((field as SettingField).type)} is unknown`;
  }
}

/** Why a section is refused, or null. */
export function validateSection(section: SettingsSection, taken: Set<string>): string | null {
  if (!section || typeof section !== 'object' || typeof section.section !== 'string' || !section.section.trim()) return 'a section has a name';
  if (!Array.isArray(section.fields) || section.fields.length === 0 || section.fields.length > MAX_FIELDS) return `section ${section.section}: one to ${MAX_FIELDS} fields`;
  const here = new Set<string>();
  for (const field of section.fields) {
    if (!field || typeof field.key !== 'string' || !KEY.test(field.key)) return `section ${section.section}: field keys are lowercase words`;
    if (taken.has(field.key) || here.has(field.key)) return `section ${section.section}: key "${field.key}" is declared twice`;
    if (typeof field.label !== 'string' || !field.label.trim()) return `section ${section.section}: field ${field.key} needs a label`;
    if (!['boolean', 'string', 'number'].includes(field.type)) return `section ${section.section}: field ${field.key} has type "${String(field.type)}"`;
    if (field.enum !== undefined && (field.type !== 'string' || !Array.isArray(field.enum) || field.enum.some((o) => typeof o !== 'string' || !o))) {
      return `section ${section.section}: field ${field.key} enum is for strings`;
    }
    const bad = settingValueError(field, field.default);
    if (bad) return `section ${section.section}: field ${field.key} default ${bad}`;
    here.add(field.key);
  }
  return null;
}

function declared(pkg: string, key: string): SettingField | undefined {
  for (const s of get(settingSections)) {
    if (s.pkg !== pkg) continue;
    const field = s.fields.find((f) => f.key === key);
    if (field) return field;
  }
  return undefined;
}

registerContributionPoint('settings', (pkg, entries, scope) => {
  const taken = new Set<string>();
  const mine: PackageSettingsSection[] = [];
  for (const entry of entries as SettingsSection[]) {
    const bad = validateSection(entry, taken);
    if (bad) {
      console.warn(`[contributes] ${pkg}: settings not applied: ${bad}`);
      continue;
    }
    entry.fields.forEach((f) => taken.add(f.key));
    mine.push({ pkg, section: entry.section, fields: entry.fields });
  }
  if (mine.length === 0) return;
  settingSections.update((all) => [...all.filter((s) => s.pkg !== pkg), ...mine]);
  scope.effect(() => {
    settingSections.update((all) => all.filter((s) => s.pkg !== pkg));
    for (const k of [...cache.keys()]) if (k.startsWith(`pkg.${pkg}.`)) cache.delete(k);
    for (const k of [...listeners.keys()]) if (k.startsWith(`pkg.${pkg}.`)) listeners.delete(k);
  });
});

function notify(pref: string, value: unknown): void {
  for (const fn of [...(listeners.get(pref) ?? [])]) {
    try {
      fn(value);
    } catch (err) {
      console.warn(`[settings] ${pref}: listener failed:`, err);
    }
  }
}

/** The owner's value for a package's setting (S3: the declared default when it cannot be read). */
export async function getSetting(key: string, pkg?: string): Promise<unknown> {
  if (typeof pkg !== 'string' || !pkg) return undefined;
  const field = declared(pkg, key);
  const pref = prefKey(pkg, key);
  if (cache.has(pref)) return cache.get(pref);
  if (!field) return undefined;
  try {
    const res = await fetchWithRenewal(`/api/preferences/${pref}`, { method: 'GET' });
    if (!res.ok) return field.default;
    const body = await res.json();
    const value = settingValueError(field, body?.value) === null ? body.value : field.default;
    cache.set(pref, value);
    return value;
  } catch {
    return field.default;
  }
}

/** Set the owner's value for a package's setting. */
export async function setSetting(key: string, value: unknown, pkg?: string): Promise<SettingResult> {
  if (typeof pkg !== 'string' || !pkg) return { ok: false, error: 'no package named' };
  const field = declared(pkg, key);
  if (!field) return { ok: false, error: `${pkg} declares no setting "${key}"` };
  const bad = settingValueError(field, value);
  if (bad) return { ok: false, error: `${field.label} ${bad}` };
  const pref = prefKey(pkg, key);
  try {
    const res = await fetchWithRenewal(`/api/preferences/${pref}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json', 'X-Choir-Device': currentDeviceId() },
      body: JSON.stringify({ value }),
    });
    if (!res.ok) {
      const body = await res.json().catch(() => ({}));
      return { ok: false, error: String(body?.error || `save failed (${res.status})`) };
    }
  } catch (err) {
    return { ok: false, error: (err as Error).message || 'save failed' };
  }
  cache.set(pref, value);
  notify(pref, value);
  return { ok: true };
}

let liveBound = false;
function followOtherWindows(): void {
  if (liveBound) return;
  liveBound = true;
  addLiveEventListener((message: any) => {
    if (liveEventKind(message) !== 'setting.updated' || isOwnLiveEvent(message)) return;
    const { key, value } = liveEventPayload(message) as { key?: string; value?: unknown };
    if (typeof key !== 'string' || !key.startsWith('pkg.')) return;
    const [, pkg, field] = key.split('.');
    const decl = pkg && field ? declared(pkg, field) : undefined;
    if (!decl || settingValueError(decl, value) !== null) return;
    cache.set(key, value);
    notify(key, value);
  });
}

/** Follow a setting: called with each new value (this window's writes and other windows'). Returns the disposer. */
export function onSetting(key: string, fn: (value: unknown) => void, pkg?: string): () => void {
  if (typeof pkg !== 'string' || !pkg || typeof fn !== 'function') return () => {};
  followOtherWindows();
  const pref = prefKey(pkg, key);
  let set = listeners.get(pref);
  if (!set) listeners.set(pref, (set = new Set()));
  set.add(fn);
  return () => {
    listeners.get(pref)?.delete(fn);
  };
}
