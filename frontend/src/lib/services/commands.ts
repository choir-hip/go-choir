// Commands and shortcuts (slice S2;
// docs/design/modular-architecture-2026-11-04.md sections 2.2 and 5).
//
// A command is { id, title, scope, run, when? }. `run` is a declarative action
// (openApp, openFile, copyText, toast: data, safe from the Library)
// or { handler: true }, meaning code registered the function (registerCommand
// with a handler). A keybinding is { key, command, when? }: a package binds a
// key and never reads keys; the shell captures keydown once (installKeyCapture)
// and resolves it here, so a broken package cannot swallow the owner's input.
//
// Failure modes:
//  M1 a package takes a locked key (Mod+K): refused at registration;
//  M2 two commands with one id: the second is refused, never shadowed;
//  M3 an id outside the owner's namespace (`<package>.<name>`; `core.` is the
//     shell's): refused, so one package cannot overwrite another's command;
//  M4 a binding to a command nobody registered: it stays inert and resolves
//     once the command exists, and goes with its owner at uninstall;
//  M5 a key pressed while typing in an input: only bindings that say so
//     (`allowInInput`, or a Mod chord) fire, never a bare letter;
//  M6 a handler that throws: reported, the keypress is not swallowed twice;
//  M7 uninstall leaks nothing: every registration is an effect in the owner's
//     scope (the leak differential in tests/commands.spec.js).
import { registerContributionPoint } from '../contributions';
import { packageScope } from '../shell-kernel';
import type { Scope } from '../kernel';

export type CommandScope = 'global' | 'app' | `target:${string}`;
export type DeclarativeAction =
  | { action: 'openApp'; app: string }
  | { action: 'openFile'; path: string }
  | { action: 'copyText'; text: string; label?: string }
  | { action: 'toast'; message: string };
export type CommandRun = DeclarativeAction | { handler: true };
export interface CommandSpec {
  id: string;
  title: string;
  scope: CommandScope;
  run: CommandRun;
  when?: string;
}
export type CommandHandler = (context: Record<string, unknown>) => void | Promise<void>;
export interface KeybindingSpec {
  key: string;
  command: string;
  when?: string;
  /** Fire while an input has focus. Mod chords always do. */
  allowInInput?: boolean;
}
type Tier = 'user' | 'app' | 'core';
const TIER_RANK: Record<Tier, number> = { user: 0, app: 1, core: 2 };

/** Keys no package may bind (the shell's own: the prompt bar). */
export const LOCKED_KEYS: ReadonlySet<string> = new Set(['Mod+K']);

const ID = /^[a-z][a-z0-9-]*\.[A-Za-z][A-Za-z0-9._-]*$/;
const ACTIONS = new Set(['openApp', 'openFile', 'copyText', 'toast']);
const SCOPE = /^(global|app|target:[a-z][a-z0-9_-]*)$/;

export function normalizeKey(key: string): string {
  const parts = String(key || '').split('+').map((p) => p.trim()).filter(Boolean);
  const mods = new Set<string>();
  let main = '';
  for (const part of parts) {
    const lower = part.toLowerCase();
    if (lower === 'mod' || lower === 'cmd' || lower === 'ctrl' || lower === 'meta') mods.add('Mod');
    else if (lower === 'alt' || lower === 'option') mods.add('Alt');
    else if (lower === 'shift') mods.add('Shift');
    else main = part.length === 1 ? part.toUpperCase() : part[0].toUpperCase() + part.slice(1);
  }
  if (!main) throw new Error(`key "${key}" names no key`);
  return [...['Mod', 'Alt', 'Shift'].filter((m) => mods.has(m)), main].join('+');
}

export function keyOfEvent(event: { key: string; metaKey?: boolean; ctrlKey?: boolean; altKey?: boolean; shiftKey?: boolean }): string {
  const main = event.key.length === 1 ? event.key.toUpperCase() : event.key;
  const mods = [event.metaKey || event.ctrlKey ? 'Mod' : '', event.altKey ? 'Alt' : '', event.shiftKey && event.key.length > 1 ? 'Shift' : '']
    .filter(Boolean);
  return [...mods, main].join('+');
}

/** A `when` clause: `key`, `!key`, `key == value`, `key != value`, joined by `&&`. */
export function evaluateWhen(when: string | undefined, context: Record<string, unknown>): boolean {
  if (!when || !when.trim()) return true;
  return when.split('&&').every((raw) => {
    const term = raw.trim();
    const eq = /^([\w.]+)\s*(==|!=)\s*(\S+)$/.exec(term);
    if (eq) return (String(context[eq[1]] ?? '') === eq[3]) === (eq[2] === '==');
    const neg = term.startsWith('!');
    const value = Boolean(context[neg ? term.slice(1).trim() : term]);
    return neg ? !value : value;
  });
}

export function validateCommand(spec: CommandSpec, owner: string): string | null {
  if (!spec || typeof spec !== 'object') return 'a command is an object';
  if (typeof spec.id !== 'string' || !ID.test(spec.id)) return `command id "${spec?.id}" must be <package>.<name>`;
  if (spec.id.split('.')[0] !== owner) return `command id "${spec.id}" is outside the namespace of "${owner}"`;
  if (typeof spec.title !== 'string' || !spec.title.trim()) return `command ${spec.id} needs a title`;
  if (typeof spec.scope !== 'string' || !SCOPE.test(spec.scope)) return `command ${spec.id}: scope must be global, app or target:<kind>`;
  const run = spec.run as any;
  if (!run || typeof run !== 'object') return `command ${spec.id}: run must be an action or { handler: true }`;
  if (run.handler !== undefined) {
    if (run.handler !== true || Object.keys(run).length !== 1) return `command ${spec.id}: run.handler must be exactly { handler: true }`;
  } else if (!ACTIONS.has(run.action)) {
    return `command ${spec.id}: run.action must be one of ${[...ACTIONS].join(', ')}`;
  }
  if (spec.when !== undefined && typeof spec.when !== 'string') return `command ${spec.id}: when must be a string`;
  return null;
}

export function validateKeybinding(binding: KeybindingSpec, owner: string): string | null {
  if (!binding || typeof binding !== 'object' || typeof binding.key !== 'string' || typeof binding.command !== 'string') return 'a keybinding is { key, command }';
  let key: string;
  try {
    key = normalizeKey(binding.key);
  } catch (err) {
    return (err as Error).message;
  }
  if (owner !== 'core' && LOCKED_KEYS.has(key)) return `key ${key} is locked to the shell`;
  if (owner !== 'core' && !binding.command.startsWith(`${owner}.`)) return `keybinding ${key} names command "${binding.command}" outside "${owner}"`;
  if (binding.when !== undefined && typeof binding.when !== 'string') return `keybinding ${key}: when must be a string`;
  return null;
}

export interface Actions {
  openApp(app: string): void;
  openFile(path: string): void;
  copyText(text: string, label?: string): void;
  toast(message: string): void;
}

interface CommandRec { spec: CommandSpec; owner: string; handler?: CommandHandler }
interface BindingRec { spec: KeybindingSpec; key: string; owner: string; tier: Tier; priority: number; seq: number }

export function createCommands(deps: { scopeFor: (owner: string) => Scope; actions: Actions }) {
  const commands = new Map<string, CommandRec>();
  const bindings: BindingRec[] = [];
  const context: Record<string, unknown> = {};
  let seq = 0;

  function registerCommand(spec: CommandSpec, owner: string, handler?: CommandHandler): () => void | Promise<void> {
    const problem = validateCommand(spec, owner);
    if (problem) throw new Error(problem);
    if (spec.run && (spec.run as any).handler === true && typeof handler !== 'function') throw new Error(`command ${spec.id} declares a handler and registered none`);
    if (commands.has(spec.id)) throw new Error(`command ${spec.id} is already registered`);
    commands.set(spec.id, { spec: Object.freeze({ ...spec }), owner, handler });
    return deps.scopeFor(owner).effect(() => { commands.delete(spec.id); });
  }

  function registerKeybinding(spec: KeybindingSpec, owner: string, tier: Tier = owner === 'core' ? 'core' : 'app', priority = 0) {
    const problem = validateKeybinding(spec, owner);
    if (problem) throw new Error(problem);
    const rec: BindingRec = { spec: Object.freeze({ ...spec }), key: normalizeKey(spec.key), owner, tier, priority, seq: ++seq };
    bindings.push(rec);
    return deps.scopeFor(owner).effect(() => {
      const at = bindings.indexOf(rec);
      if (at >= 0) bindings.splice(at, 1);
    });
  }

  function applies(rec: CommandRec): boolean {
    const scope = rec.spec.scope;
    if (scope === 'app') return context['window.app'] === rec.owner;
    if (scope.startsWith('target:')) return context['target.kind'] === scope.slice('target:'.length);
    return true;
  }

  async function run(id: string, extra: Record<string, unknown> = {}): Promise<boolean> {
    const rec = commands.get(id);
    if (!rec || !applies(rec) || !evaluateWhen(rec.spec.when, context)) return false;
    const run = rec.spec.run as any;
    if (run.handler === true) {
      await rec.handler!({ ...context, ...extra });
      return true;
    }
    switch (run.action) {
      case 'openApp': deps.actions.openApp(String(run.app || '')); break;
      case 'openFile': deps.actions.openFile(String(run.path || '')); break;
      case 'copyText': deps.actions.copyText(String(run.text ?? ''), run.label); break;
      case 'toast': deps.actions.toast(String(run.message || '')); break;
    }
    return true;
  }

  /** The binding that wins for a key, or null: locked and core first by tier, then priority, then oldest. */
  function resolveKey(key: string): BindingRec | null {
    const live = bindings
      .filter((b) => b.key === key && evaluateWhen(b.spec.when, context))
      .filter((b) => {
        const cmd = commands.get(b.spec.command);
        return Boolean(cmd && applies(cmd) && evaluateWhen(cmd.spec.when, context));
      });
    if (!live.length) return null;
    const locked = LOCKED_KEYS.has(key);
    live.sort((a, b) => {
      // A locked key belongs to the shell's own binding; an owner's own binding is on top elsewhere.
      if (locked && (a.tier === 'core') !== (b.tier === 'core')) return a.tier === 'core' ? -1 : 1;
      return TIER_RANK[a.tier] - TIER_RANK[b.tier] || b.priority - a.priority || a.seq - b.seq;
    });
    return live[0];
  }

  function handleKeydown(event: KeyboardEvent): boolean {
    if (event.defaultPrevented || event.isComposing) return false;
    const key = keyOfEvent(event);
    const hit = resolveKey(key);
    if (!hit) return false;
    const target = event.target as HTMLElement | null;
    const typing = Boolean(target && (target.isContentEditable || /^(INPUT|TEXTAREA|SELECT)$/.test(target.tagName)));
    if (typing && !hit.spec.allowInInput && !key.startsWith('Mod+')) return false;
    event.preventDefault();
    void run(hit.spec.command).catch((err) => console.warn(`[commands] ${hit.spec.command} failed:`, err));
    return true;
  }

  return {
    registerCommand,
    registerKeybinding,
    run,
    resolveKey,
    handleKeydown,
    setContext(key: string, value: unknown) { if (value === undefined || value === null || value === false) delete context[key]; else context[key] = value; },
    has: (id: string) => commands.has(id),
    list: () => [...commands.values()].map((c) => ({ ...c.spec, owner: c.owner })),
    bindings: () => bindings.map((b) => ({ key: b.key, command: b.spec.command, owner: b.owner, tier: b.tier })),
  };
}

// ---- the shell's service ----

let shellActions: Actions | null = null;
/** The desktop wires what a declarative action does; before that, actions are inert. */
export function bindCommandActions(actions: Actions): () => void {
  shellActions = actions;
  return () => { if (shellActions === actions) shellActions = null; };
}
const act = <K extends keyof Actions>(name: K) => ((...args: Parameters<Actions[K]>) => (shellActions?.[name] as any)?.(...args)) as Actions[K];

export const commands = createCommands({
  scopeFor: packageScope,
  actions: { openApp: act('openApp'), openFile: act('openFile'), copyText: act('copyText'), toast: act('toast') },
});

/** The shell's single keydown capture. Returns the disposer. */
export function installKeyCapture(target: Pick<Window, 'addEventListener' | 'removeEventListener'> = window): () => void {
  const onKey = (event: Event) => { commands.handleKeydown(event as KeyboardEvent); };
  target.addEventListener('keydown', onKey);
  return () => target.removeEventListener('keydown', onKey);
}

/** SDK: a package registers a command in code (a disposer comes back). The handler runs for { handler: true }. */
export function registerCommand(spec: CommandSpec, handler?: CommandHandler): () => void | Promise<void> {
  const owner = String(spec?.id || '').split('.')[0];
  return commands.registerCommand(spec, owner, handler);
}

// contributes.commands and contributes.keybindings (data, T0).
registerContributionPoint('commands', (pkg, entries) => {
  for (const entry of entries) {
    const spec = entry as CommandSpec;
    // A declarative command registers now; { handler: true } registers when code calls registerCommand.
    if ((spec?.run as any)?.handler === true) continue;
    try { commands.registerCommand(spec, pkg); } catch (err) { console.warn(`[contributes] ${pkg}: ${(err as Error).message}`); }
  }
});
registerContributionPoint('keybindings', (pkg, entries) => {
  for (const entry of entries) {
    try { commands.registerKeybinding(entry as KeybindingSpec, pkg, 'app'); } catch (err) { console.warn(`[contributes] ${pkg}: ${(err as Error).message}`); }
  }
});

