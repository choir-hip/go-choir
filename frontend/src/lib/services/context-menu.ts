// The context menu on the slot (slice S4; docs/design/modular-architecture-2026-11-04.md
// sections 2.2, 2.4 and 5; plan section 11). Three seams, all in the shell kernel:
//
//  - `contextMenu.renderer` is a single-slot service. The base provider
//    (`menu-default`, ContextMenu.svelte) is always there; a package may provide
//    another (tier T2: it sees every menu, file names included). The ranking is
//    the kernel's. A renderer that fails its contract is refused at
//    registration; one that fails while rendering is reported to the watchdog
//    and the base menu opens in its place, so a right-click always answers.
//  - `contextMenu.build` is a pipeline over the menu's items. Handlers get a
//    frozen list of plain descriptors (no callbacks) led by the public target
//    { kind, ext, name }, and return a list. An item names a `command`; it
//    never carries a callback. `essential` items cannot be removed or edited.
//  - `contributes.menus` ({ target, command, group?, order?, when? }) adds a
//    package's commands to a target's menu through that pipeline. A host-side
//    contributor (Open with on files) adds items that carry callbacks, before
//    the pipeline runs.
//
// Failure modes:
//  X1 a renderer with no render function, or an unknown service ABI: refused,
//     the base keeps serving;
//  X2 a renderer that throws or returns false: reported against it (three in
//     the window un-rank it) and the base renders this menu;
//  X3 a handler that throws, runs over budget, or returns junk: skipped by the
//     kernel (P2), the menu opens with the previous items;
//  X4 a handler removes or relabels an essential item: the original returns;
//  X5 a handler item with no command, an unregistered command, or an id that
//     collides with a host callback it is not relabeling: dropped, never run;
//  X6 a contributed menu entry for a command that is unregistered, not
//     applicable to the target, or whose `when` is false: not shown;
//  X7 a package leaves: its handler, contributions and renderer leave with its
//     scope (the leak differential);
//  X8 a menu with no handler for its target opens synchronously, exactly as
//     before the slot existed; only a target something intercepts goes async;
//  X9 more than MAX_ITEMS items or deeper than one submenu: truncated.
import { registerContributionPoint } from '../contributions';
import { shellKernel } from '../shell-kernel';
import type { Scope } from '../kernel';
import { commands, evaluateWhen } from './commands';
import type { ContextMenuEntry, ContextMenuItem, ContextMenuRequest, ContextMenuTarget } from '../context-menu-state';

export const RENDERER_SLOT = 'contextMenu.renderer';
export const RENDERER_ABI = 'choir.service/contextMenu.renderer/v1';
export const BUILD_PIPELINE = 'contextMenu.build';
export const BASE_RENDERER_ID = 'menu-default';
export const USE_DEFAULT_ID = 'use-default-menu';
export const MAX_ITEMS = 60;
const MAX_LABEL = 120;

export type RenderRequest = ContextMenuRequest & { seq: number };
export interface ContextMenuRenderer {
  /** Show the menu. Return false (or throw) to say it could not; the base renders instead. */
  render(request: RenderRequest): boolean | void;
}
export interface RendererSpec {
  id: string;
  impl: ContextMenuRenderer;
  tier?: 'policy' | 'user' | 'app' | 'core';
  priority?: number;
  grants?: string[];
}
type Descriptor = {
  id: string;
  label?: string;
  icon?: string;
  shortcut?: string;
  disabled?: boolean;
  danger?: boolean;
  separator?: boolean;
  essential?: boolean;
  command?: string;
  group?: string;
  order?: number;
  items?: Descriptor[];
  origin?: string;
};
export type MenuEntry = { target: string; command: string; group?: string; order?: number; when?: string };
export type MenuContributor = {
  id: string;
  kinds: string[];
  /** Entries (with callbacks) to add for this target, and the item id to follow (default: the end). */
  contribute(target: ContextMenuTarget): { after?: string; entries: ContextMenuEntry[] } | null;
};

const root = shellKernel.root;
shellKernel.services.define(root, RENDERER_SLOT, RENDERER_ABI);
shellKernel.pipelines.define(root, BUILD_PIPELINE, { budgetMs: 150 });

const SLOT_ID = /^[a-z][a-z0-9-]{0,63}$/;

function contractError(spec: RendererSpec): string | null {
  if (!spec || typeof spec !== 'object') return 'a renderer is { id, impl }';
  if (typeof spec.id !== 'string' || !SLOT_ID.test(spec.id)) return `renderer id "${String(spec.id)}" must be a lowercase word`;
  if (!spec.impl || typeof spec.impl.render !== 'function') return `renderer ${spec.id}: impl.render must be a function`;
  return null;
}

let baseRender: ((request: RenderRequest) => void) | null = null;

/** context-menu.ts installs the default menu (ContextMenu.svelte) as the base provider. */
export function installBaseRenderer(render: (request: RenderRequest) => void): void {
  baseRender = render;
  shellKernel.services.provide(root, RENDERER_SLOT, {
    id: BASE_RENDERER_ID,
    tier: 'core',
    priority: 0,
    base: true,
    impl: { render: (request: RenderRequest) => { render(request); return true; } } satisfies ContextMenuRenderer,
  });
}

/** A package provides a renderer. Throws (the base keeps serving) when the contract is not met. Returns the disposer. */
export function registerContextMenuRenderer(scope: Scope, spec: RendererSpec): () => void {
  const bad = contractError(spec);
  if (bad) throw new Error(bad);
  if (spec.id === BASE_RENDERER_ID) throw new Error(`renderer id "${BASE_RENDERER_ID}" is the shell's`);
  return shellKernel.services.provide(scope, RENDERER_SLOT, { id: spec.id, impl: spec.impl, tier: spec.tier ?? 'app', priority: spec.priority, grants: spec.grants });
}

function activeRenderer(): { id: string; impl: ContextMenuRenderer } | null {
  const info = (shellKernel.services.describe().services as any)[RENDERER_SLOT];
  const impl = shellKernel.services.resolve<ContextMenuRenderer>(RENDERER_SLOT);
  return impl && info?.active ? { id: info.active, impl } : null;
}

// ---- build handlers (the pipeline, with the targets they intercept) ----

type HandlerFn = (items: readonly Record<string, unknown>[]) => readonly Record<string, unknown>[] | void | Promise<readonly Record<string, unknown>[] | void>;
const interceptors = new Map<string, string[] | null>();

/** A handler on contextMenu.build for some targets (null: all). The first item it sees is { type: 'target', kind, ext, name }. */
export function useContextMenuBuild(scope: Scope, spec: { id: string; tier?: 'policy' | 'user' | 'app' | 'core'; priority?: number; targets?: string[] | null }, fn: HandlerFn): () => void {
  if (interceptors.has(spec.id)) throw new Error(`build handler ${spec.id} is already registered`);
  const targets = spec.targets ?? null;
  const dispose = shellKernel.pipelines.use(scope, BUILD_PIPELINE, { id: spec.id, tier: spec.tier ?? 'app', priority: spec.priority }, (items) => {
    const t = items[0] as { type?: string; kind?: string } | undefined;
    if (!t || t.type !== 'target') return undefined;
    if (targets && !targets.includes(String(t.kind))) return undefined;
    return fn(items);
  });
  interceptors.set(spec.id, targets);
  const off = scope.effect(() => { interceptors.delete(spec.id); });
  return () => { void off(); void dispose(); };
}

const contributors = new Map<string, MenuContributor>();
/** A host-side contributor (Open with): adds entries with callbacks before the pipeline runs. */
export function registerMenuContributor(scope: Scope, c: MenuContributor): () => void {
  if (contributors.has(c.id)) throw new Error(`menu contributor ${c.id} is already registered`);
  contributors.set(c.id, c);
  return scope.effect(() => { contributors.delete(c.id); });
}

// ---- descriptors <-> entries ----

function describeEntries(entries: ContextMenuEntry[], prefix: string, originals: Map<string, ContextMenuItem>, depth = 0): Descriptor[] {
  const out: Descriptor[] = [];
  entries.forEach((e, i) => {
    if (!e) return;
    if ((e as { separator?: boolean }).separator === true) {
      out.push({ id: `${prefix}sep${i}`, separator: true });
      return;
    }
    const item = e as ContextMenuItem;
    const id = item.id ?? `${prefix}i${i}`;
    originals.set(id, item);
    const d: Descriptor = { id, label: item.label };
    if (item.icon) d.icon = item.icon;
    if (item.shortcut) d.shortcut = item.shortcut;
    if (item.disabled) d.disabled = true;
    if (item.danger) d.danger = true;
    if (item.essential) d.essential = true;
    if (item.command) d.command = item.command;
    if (item.items?.length && depth < 1) d.items = describeEntries(item.items, `${id}/`, originals, depth + 1);
    out.push(d);
  });
  return out;
}

async function runCommand(command: string, target: ContextMenuTarget): Promise<void> {
  commands.setContext('target.kind', target.kind);
  commands.setContext('target.ext', target.ext);
  try {
    const ran = await commands.run(command, { target: { kind: target.kind, ext: target.ext, name: target.name } });
    if (!ran) console.warn(`[context-menu] command ${command} did not run for target ${target.kind}`);
  } catch (err) {
    console.warn(`[context-menu] command ${command} failed:`, err);
  } finally {
    commands.setContext('target.kind', undefined);
    commands.setContext('target.ext', undefined);
  }
}

function realize(ds: readonly Descriptor[], originals: Map<string, ContextMenuItem>, target: ContextMenuTarget, depth = 0): ContextMenuEntry[] {
  const out: ContextMenuEntry[] = [];
  for (const d of ds) {
    if (!d || typeof d !== 'object') continue;
    if (d.separator === true) {
      if (out.length && !(out[out.length - 1] as { separator?: boolean }).separator) out.push({ separator: true });
      continue;
    }
    if (typeof d.id !== 'string' || typeof d.label !== 'string' || !d.label.trim()) continue;
    const orig = originals.get(d.id);
    const item: ContextMenuItem = { id: d.id, label: d.label.slice(0, MAX_LABEL) };
    if (typeof d.icon === 'string') item.icon = d.icon.slice(0, 16);
    if (typeof d.shortcut === 'string') item.shortcut = d.shortcut.slice(0, 24);
    if (d.disabled === true) item.disabled = true;
    if (d.danger === true) item.danger = true;
    if (d.essential === true && orig?.essential) item.essential = true;
    if (Array.isArray(d.items) && d.items.length && depth < 1) {
      const children = realize(d.items, originals, target, depth + 1);
      if (children.length) item.items = children;
    }
    if (typeof d.command === 'string' && d.command && commands.has(d.command)) {
      const command = d.command;
      item.command = command;
      item.onSelect = () => runCommand(command, target);
    } else if (orig?.onSelect) {
      item.onSelect = orig.onSelect;
    }
    if (!item.onSelect && !item.items) continue; // X5: nothing to do, nothing shown
    out.push(item);
  }
  while (out.length && (out[out.length - 1] as { separator?: boolean }).separator) out.pop();
  return out;
}

function restoreEssentials(entries: ContextMenuEntry[], before: ContextMenuEntry[]): ContextMenuEntry[] {
  const out = [...entries];
  for (const e of before) {
    const item = e as ContextMenuItem;
    if (!item.essential || !item.id) continue;
    const at = out.findIndex((x) => (x as ContextMenuItem).id === item.id);
    if (at >= 0) out[at] = item;
    else out.push(item);
  }
  return out;
}

function flatten(entries: ContextMenuEntry[]): number {
  let n = 0;
  for (const e of entries) n += 1 + ((e as ContextMenuItem).items ? flatten((e as ContextMenuItem).items!) : 0);
  return n;
}

function normalizeTarget(t?: ContextMenuTarget): ContextMenuTarget {
  const kind = typeof t?.kind === 'string' && /^[a-z][a-z0-9_-]{0,31}$/.test(t.kind) ? t.kind : 'other';
  const out: ContextMenuTarget = { kind };
  if (t?.name) out.name = String(t.name);
  const ext = t?.ext ?? (t?.name ? String(t.name).toLowerCase().split('.').slice(1).pop() : undefined);
  if (ext && /^[a-z0-9]{1,16}$/.test(ext)) out.ext = ext;
  if (t?.data) out.data = t.data;
  return out;
}

function withContributors(items: ContextMenuEntry[], target: ContextMenuTarget): ContextMenuEntry[] {
  let out = [...items];
  for (const c of contributors.values()) {
    if (!c.kinds.includes(target.kind)) continue;
    let add: ReturnType<MenuContributor['contribute']> = null;
    try { add = c.contribute(target); } catch (err) { console.warn(`[context-menu] contributor ${c.id} failed:`, err); }
    if (!add?.entries.length) continue;
    const at = add.after ? out.findIndex((e) => (e as ContextMenuItem).id === add!.after) : -1;
    if (at >= 0) out.splice(at + 1, 0, ...add.entries);
    else out.push(...add.entries);
  }
  return out;
}

function render(request: RenderRequest): void {
  const active = activeRenderer();
  if (active && active.id !== BASE_RENDERER_ID) {
    try {
      if (active.impl.render(request) !== false) return;
      shellKernel.services.reportFailure(RENDERER_SLOT, active.id, 'render returned false');
    } catch (err) {
      shellKernel.services.reportFailure(RENDERER_SLOT, active.id, `render threw: ${String(err)}`);
    }
  }
  baseRender?.(request); // X2: the base answers
}

let seq = 0;

/**
 * Show a menu: host contributors first, then (when something intercepts this
 * target) the build pipeline, then the active renderer. Returns whether a menu
 * will show. The base path with no interceptor is synchronous (X8).
 */
export function presentMenu(request: ContextMenuRequest): boolean {
  const target = normalizeTarget(request.target);
  let items = withContributors((request.items || []).filter(Boolean), target);
  if (!items.length) return false;
  const active = activeRenderer();
  if (active && active.id !== BASE_RENDERER_ID) {
    const id = active.id;
    items = [...items, { separator: true }, { id: USE_DEFAULT_ID, label: 'Use default menu', essential: true, onSelect: () => { void shellKernel.services.decline(root, RENDERER_SLOT, id); } }];
  }
  const intercepted = [...interceptors.values()].some((t) => t === null || t.includes(target.kind));
  const finish = (entries: ContextMenuEntry[]) => {
    const clipped = flatten(entries) > MAX_ITEMS ? entries.slice(0, MAX_ITEMS) : entries;
    if (!clipped.length) return;
    seq += 1;
    render({ ...request, items: clipped, target: { kind: target.kind, ext: target.ext, name: target.name }, seq });
  };
  if (!intercepted) {
    finish(items);
    return true;
  }
  const originals = new Map<string, ContextMenuItem>();
  const described = describeEntries(items, '', originals);
  const payload: Record<string, unknown>[] = [{ type: 'target', kind: target.kind, ext: target.ext, name: target.name }, ...described];
  void shellKernel.pipelines.run(BUILD_PIPELINE, payload)
    .then((result) => {
      for (const s of result.skipped) console.warn(`[context-menu] ${s.handler} skipped: ${s.reason}`);
      const built = result.items.filter((it) => (it as { type?: string }).type !== 'target') as unknown as Descriptor[];
      finish(restoreEssentials(realize(built, originals, target), items));
    })
    .catch((err) => {
      console.warn('[context-menu] build failed, opening the unmodified menu:', err);
      finish(items);
    });
  return true;
}

// ---- contributes.menus ----

const TARGET = /^[a-z][a-z0-9_-]{0,31}$/;
const GROUP = /^[a-z][a-z0-9_-]{0,31}$/;

export function validateMenu(entry: MenuEntry, owner: string): string | null {
  if (!entry || typeof entry !== 'object') return 'a menu entry is { target, command }';
  if (typeof entry.target !== 'string' || !TARGET.test(entry.target)) return `menu target "${String(entry?.target)}" must be a lowercase word`;
  if (typeof entry.command !== 'string' || !entry.command.startsWith(`${owner}.`)) return `menu entry for ${entry.target} names command "${String(entry.command)}" outside "${owner}"`;
  if (entry.group !== undefined && (typeof entry.group !== 'string' || !GROUP.test(entry.group))) return `menu entry ${entry.command}: group must be a lowercase word`;
  if (entry.order !== undefined && !(typeof entry.order === 'number' && Number.isFinite(entry.order))) return `menu entry ${entry.command}: order must be a number`;
  if (entry.when !== undefined && typeof entry.when !== 'string') return `menu entry ${entry.command}: when must be a string`;
  return null;
}

registerContributionPoint('menus', (pkg, entries, scope) => {
  const mine: MenuEntry[] = [];
  for (const raw of entries as MenuEntry[]) {
    const bad = validateMenu(raw, pkg);
    if (bad) { console.warn(`[contributes] ${pkg}: menu entry not applied: ${bad}`); continue; }
    mine.push(raw);
  }
  if (!mine.length) return;
  const kinds = [...new Set(mine.map((m) => m.target))];
  useContextMenuBuild(scope, { id: `${pkg}:menus`, tier: 'app', targets: kinds }, (items) => {
    const t = items[0] as { kind: string; ext?: string; name?: string };
    const when = { 'target.kind': t.kind, 'target.ext': t.ext };
    const shown = mine
      .filter((m) => m.target === t.kind && evaluateWhen(m.when, when))
      .map((m) => ({ m, cmd: commands.list().find((c) => c.id === m.command) }))
      .filter(({ cmd }) => cmd && (cmd.scope === 'global' || cmd.scope === `target:${t.kind}`) && evaluateWhen(cmd.when, when))
      .sort((a, b) => String(a.m.group ?? '').localeCompare(String(b.m.group ?? '')) || (a.m.order ?? 0) - (b.m.order ?? 0));
    if (!shown.length) return undefined;
    const added = shown.map(({ m, cmd }) => ({ id: `${pkg}:${m.command}`, label: cmd!.title, command: m.command }));
    const last = items[items.length - 1] as { separator?: boolean };
    return [...items, ...(items.length > 1 && !last.separator ? [{ id: `${pkg}:sep`, separator: true }] : []), ...added];
  });
});
