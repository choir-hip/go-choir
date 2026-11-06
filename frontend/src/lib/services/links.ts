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
// without at least one of the named params. Two params may fill one context
// key (an older spelling of a parameter): the first one present wins. A param
// whose context is `windowTitle` names the window. A link no rule accepts is
// cleared and ignored. Mail's older special case (`draft`/`approval`) stays
// in App.svelte until Mail's links move into its manifest.
//
// A public route (slice T1) is a rule with `route` instead of parameters: the
// page path under the package's own public prefix opens its app, with the
// path in one context key:
//
//   { "app": "texture", "params": [],
//     "route": { "prefix": "/pub/texture/", "context": "publishedRoutePath" },
//     "launch": { "windowTitle": "Published Texture", "context": { "allowMultiple": true } } }
//
// `/pub/texture/a-story` opens Texture with { publishedRoutePath:
// '/pub/texture/a-story', allowMultiple: true }. A package has at most one
// parameter rule and one route rule.
//
// Failure modes:
//  L1 a package claims another app's link: refused, `app` is the package's own;
//  L2 a param named `app`, `title`, `doc` or `doc_id` (the shell's): refused;
//  L3 a param value that is too long or holds control characters: the link is
//     refused (a URL is untrusted input);
//  L4 two packages declare the same `app`: the second is refused;
//  L5 uninstall: the rule leaves with its package scope, and its params stop
//     being cleared (the leak differential);
//  L7 more than one rule claiming the signed-out home: the second is refused;
//  L6 a context value that is not a literal or `param:<name>` of this rule:
//     the rule is refused (no expressions in data);
//  L8 a route outside the package's own prefix (/pub/<package>/): refused, a
//     package cannot answer another app's public pages; a route path that is
//     too long or holds control characters opens nothing (L3 for paths).
import { getAppDefinition } from '../apps/registry';
import { registerContributionPoint } from '../contributions';

export type LinkParam = { name: string; required?: boolean; context?: string };
export type LinkLaunch = {
  /** The signed-out home opens this app's window (the preview seed): at most one rule is the home. */
  home?: boolean;
  windowTitle?: string;
  requireAny?: string[];
  context?: Record<string, string | number | boolean>;
};
export type LinkRoute = { prefix: string; context: string };
export type LinkRule = { app: string; params: LinkParam[]; route?: LinkRoute; launch?: LinkLaunch };
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
const RESERVED_PARAMS = new Set(['app', 'draft', 'approval', 'intake', 'intake_id']);
const MAX_VALUE = 512;

export function validateLink(rule: LinkRule, owner: string): string | null {
  if (!rule || typeof rule !== 'object' || typeof rule.app !== 'string') return 'a link is { app, params, launch }';
  if (rule.app !== owner) return `link for app "${rule.app}" is not the package's own ("${owner}")`;
  if (rule.launch?.home !== undefined && typeof rule.launch.home !== 'boolean') return `link ${rule.app}: home must be a boolean`;
  if (rule.route !== undefined) {
    const r = rule.route;
    if (!r || typeof r !== 'object' || r.prefix !== `/pub/${owner}/`) return `link ${rule.app}: a route's prefix is the package's own "/pub/${owner}/"`;
    if (typeof r.context !== 'string' || !CONTEXT_KEY.test(r.context)) return `link ${rule.app}: route context "${r.context}" is not a word`;
    if (!Array.isArray(rule.params) || rule.params.length !== 0) return `link ${rule.app}: a route rule takes no parameters`;
    if (rule.launch?.home !== undefined || rule.launch?.requireAny !== undefined) return `link ${rule.app}: a route rule has no home or requireAny`;
  }
  if (!Array.isArray(rule.params) || (rule.params.length === 0 && rule.launch?.home !== true && rule.route === undefined)) return `link ${rule.app}: params must list at least one parameter`;
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
  const routes = new Map<string, LinkRule>();

  function add(rule: LinkRule, owner: string): () => void {
    const problem = validateLink(rule, owner);
    if (problem) throw new Error(problem);
    const table = rule.route ? routes : rules;
    if (table.has(rule.app)) throw new Error(`a ${rule.route ? 'route' : 'link'} for app "${rule.app}" is already declared`);
    if (rule.launch?.home && [...rules.values()].some((r) => r.launch?.home)) throw new Error(`link ${rule.app}: another app is already the signed-out home`);
    const frozen = JSON.parse(JSON.stringify(rule)) as LinkRule;
    table.set(rule.app, frozen);
    return () => { if (table.get(rule.app) === frozen) table.delete(rule.app); };
  }

  /** The launch a page path asks for (a public route), or null (no rule, an unsafe path). */
  function resolveRoute(pathname: string): LinkIntent | null {
    const raw = String(pathname || '');
    if (raw.length > MAX_VALUE || /[\u0000-\u001f]/.test(raw)) return null;
    for (const rule of routes.values()) {
      const prefix = rule.route!.prefix;
      if (!raw.startsWith(prefix)) continue;
      const path = raw.replace(/\/+$/, '');
      if (path.length <= prefix.length - 1) return null;
      const launch = rule.launch ?? {};
      const appContext: Record<string, unknown> = { ...(launch.context ?? {}) };
      appContext[rule.route!.context] = path;
      const app = describeApp(rule.app);
      appContext.windowTitle = launch.windowTitle || app?.name || rule.app;
      return { kind: 'app_launch', source: 'url', appId: rule.app, appName: app?.name || rule.app, icon: app?.icon || '', appContext };
    }
    return null;
  }

  /** The context key an app's public route fills (one window per value), or '' when it has none. */
  function routeContextFor(appId: string): string {
    return routes.get(appId)?.route?.context || '';
  }

  /** The parameters packages' links consume, to clear them from the address bar with the shell's own. */
  function consumedParams(): string[] {
    return [...new Set([...rules.values()].flatMap((r) => r.params.map((p) => p.name)))].sort();
  }

  /** The window the signed-out home opens (the preview seed), or null when no rule is the home. */
  function homeIntent(): LinkIntent | null {
    const rule = [...rules.values()].find((r) => r.launch?.home);
    if (!rule) return null;
    const app = describeApp(rule.app);
    const appContext: Record<string, unknown> = { ...(rule.launch?.context ?? {}) };
    appContext.windowTitle = rule.launch?.windowTitle || app?.name || rule.app;
    return { kind: 'app_launch', source: 'url', appId: rule.app, appName: app?.name || rule.app, icon: app?.icon || '', appContext };
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
    // Two params may fill one key (an older spelling): the first present wins.
    for (const p of rule.params) if (p.context && (!(p.context in appContext) || (!appContext[p.context] && values[p.name]))) appContext[p.context] = values[p.name] ?? '';
    for (const [key, value] of Object.entries(launch.context ?? {})) {
      if (typeof value === 'string' && value.startsWith('param:')) appContext[key] = values[value.slice(6)] ?? '';
      else appContext[key] = value;
    }
    const app = describeApp(rule.app);
    appContext.windowTitle = (typeof appContext.windowTitle === 'string' && appContext.windowTitle) || launch.windowTitle || app?.name || rule.app;
    return { kind: 'app_launch', source: 'url', appId: rule.app, appName: app?.name || rule.app, icon: app?.icon || '', appContext };
  }

  return { add, resolve, resolveRoute, routeContextFor, homeIntent, consumedParams, has: (app: string) => rules.has(app) || routes.has(app) };
}

// ---- the shell's service ----

export const links = createLinks((appId) => {
  const app = getAppDefinition(appId);
  return app ? { name: app.name, icon: app.icon } : null;
});

// The signed-out home: the front list of living stories. Autopaper is a
// built-in app until its row moves into a manifest (C8); then this rule moves
// with it as contributes.links[].launch.home.
links.add({ app: 'autopaper', params: [], launch: { home: true, windowTitle: 'Autopaper', context: { preview: true } } }, 'autopaper');

registerContributionPoint('links', (pkg, entries, scope) => {
  for (const entry of entries) {
    try {
      scope.effect(links.add(entry as LinkRule, pkg));
    } catch (err) {
      console.warn(`[contributes] ${pkg}: ${(err as Error).message}`);
    }
  }
});
