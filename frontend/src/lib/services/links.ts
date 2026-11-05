// Deep links (slice S7; docs/design/modular-architecture-2026-11-04.md
// section 2.2). A package declares `contributes.links`: which URL parameters
// open its app and what window context they make, as data:
//
//   { "app": "notes", "params": [ { "name": "note", "context": "noteId" },
//                                  { "name": "tag" } ],
//     "launch": { "windowTitle": "Notes", "requireAny": ["note"],
//                 "context": { "noteId": "param:note", "pinned": true } } }
//
// `?app=notes&note=7` opens Notes with { noteId: '7', pinned: true }. A param
// with `context` fills that key; a param without it is consumed (cleared from
// the address bar, as the old `intake` parameters were) and nothing more.
// `required: true` refuses the link without it; `requireAny` refuses it
// without at least one of the named params. A link no rule accepts is cleared
// and ignored. The shell's two older special cases (Texture's `doc`, Mail's
// `draft`/`approval`) stay in App.svelte until those apps move into packages.
//
// Failure modes:
//  L1 a package claims another app's link: refused, `app` is the package's own;
//  L2 a param named `app`, `title`, `doc` or `doc_id` (the shell's): refused;
//  L3 a param value that is too long or holds control characters: the link is
//     refused (a URL is untrusted input);
//  L4 two packages declare the same `app`: the second is refused;
//  L5 uninstall: the rule leaves with its package scope, and its params stop
//     being cleared (the leak differential);
//  L6 a context value that is not a literal or `param:<name>` of this rule:
//     the rule is refused (no expressions in data).
import { getAppDefinition } from '../apps/registry';
import { registerContributionPoint } from '../contributions';

export type LinkParam = { name: string; required?: boolean; context?: string };
export type LinkLaunch = {
  windowTitle?: string;
  requireAny?: string[];
  context?: Record<string, string | number | boolean>;
};
export type LinkRule = { app: string; params: LinkParam[]; launch?: LinkLaunch };
export type LinkIntent = {
  kind: 'app_launch';
  source: 'url';
  appId: string;
  appName: string;
  icon: string;
  appContext: Record<string, unknown>;
};

const NAME = /^[a-z][a-z0-9_]{0,31}$/;
const CONTEXT_KEY = /^[a-z][A-Za-z0-9_]{0,31}$/;
const RESERVED_PARAMS = new Set(['app', 'title', 'doc', 'doc_id']);
const MAX_VALUE = 512;

export function validateLink(rule: LinkRule, owner: string): string | null {
  if (!rule || typeof rule !== 'object' || typeof rule.app !== 'string') return 'a link is { app, params, launch }';
  if (rule.app !== owner) return `link for app "${rule.app}" is not the package's own ("${owner}")`;
  if (!Array.isArray(rule.params) || rule.params.length === 0) return `link ${rule.app}: params must list at least one parameter`;
  const names = new Set<string>();
  for (const p of rule.params) {
    if (!p || typeof p.name !== 'string' || !NAME.test(p.name)) return `link ${rule.app}: parameter names are lowercase words`;
    if (RESERVED_PARAMS.has(p.name)) return `link ${rule.app}: parameter "${p.name}" is the shell's`;
    if (names.has(p.name)) return `link ${rule.app}: parameter "${p.name}" is listed twice`;
    if (p.required !== undefined && typeof p.required !== 'boolean') return `link ${rule.app}: required must be a boolean`;
    if (p.context !== undefined && (typeof p.context !== 'string' || !CONTEXT_KEY.test(p.context))) {
      return `link ${rule.app}: context key "${p.context}" is not a word`;
    }
    names.add(p.name);
  }
  const launch = rule.launch ?? {};
  for (const n of launch.requireAny ?? []) if (!names.has(n)) return `link ${rule.app}: requireAny names "${n}", which is not a parameter`;
  for (const [key, value] of Object.entries(launch.context ?? {})) {
    if (typeof value === 'string' && value.startsWith('param:')) {
      if (!names.has(value.slice(6))) return `link ${rule.app}: context ${key} reads parameter "${value.slice(6)}", which is not declared`;
    } else if (!['string', 'number', 'boolean'].includes(typeof value)) {
      return `link ${rule.app}: context ${key} must be a literal or "param:<name>"`;
    }
  }
  return null;
}

export function createLinks(describeApp: (appId: string) => { name: string; icon: string } | null) {
  const rules = new Map<string, LinkRule>();

  function add(rule: LinkRule, owner: string): () => void {
    const problem = validateLink(rule, owner);
    if (problem) throw new Error(problem);
    if (rules.has(rule.app)) throw new Error(`a link for app "${rule.app}" is already declared`);
    const frozen = JSON.parse(JSON.stringify(rule)) as LinkRule;
    rules.set(rule.app, frozen);
    return () => { if (rules.get(rule.app) === frozen) rules.delete(rule.app); };
  }

  /** The parameters packages' links consume, to clear them from the address bar with the shell's own. */
  function consumedParams(): string[] {
    return [...new Set([...rules.values()].flatMap((r) => r.params.map((p) => p.name)))].sort();
  }

  /** The launch a URL asks for, or null (no rule, a missing required parameter, an unsafe value). */
  function resolve(search: string): LinkIntent | null {
    const params = new URLSearchParams(search || '');
    const appId = (params.get('app') || '').trim().toLowerCase();
    const rule = rules.get(appId);
    if (!rule) return null;
    const values: Record<string, string> = {};
    for (const p of rule.params) {
      const raw = params.get(p.name);
      const value = (raw ?? '').trim();
      if (value.length > MAX_VALUE || /[\u0000-\u001f]/.test(value)) return null;
      if (!value) {
        if (p.required) return null;
        continue;
      }
      values[p.name] = value;
    }
    const launch = rule.launch ?? {};
    if (launch.requireAny?.length && !launch.requireAny.some((n) => values[n])) return null;
    const appContext: Record<string, unknown> = {};
    for (const p of rule.params) if (p.context) appContext[p.context] = values[p.name] ?? '';
    for (const [key, value] of Object.entries(launch.context ?? {})) {
      if (typeof value === 'string' && value.startsWith('param:')) appContext[key] = values[value.slice(6)] ?? '';
      else appContext[key] = value;
    }
    const app = describeApp(rule.app);
    appContext.windowTitle = launch.windowTitle || app?.name || rule.app;
    return { kind: 'app_launch', source: 'url', appId: rule.app, appName: app?.name || rule.app, icon: app?.icon || '', appContext };
  }

  return { add, resolve, consumedParams, has: (app: string) => rules.has(app) };
}

// ---- the shell's service ----

export const links = createLinks((appId) => {
  const app = getAppDefinition(appId);
  return app ? { name: app.name, icon: app.icon } : null;
});

registerContributionPoint('links', (pkg, entries, scope) => {
  for (const entry of entries) {
    try {
      scope.effect(links.add(entry as LinkRule, pkg));
    } catch (err) {
      console.warn(`[contributes] ${pkg}: ${(err as Error).message}`);
    }
  }
});
