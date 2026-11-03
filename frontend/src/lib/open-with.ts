// Open with: which apps can open a file (owner, 2026-11-03: new apps "only
// load [slides] from files app, which opens slides in the old slides app").
//
// The default opener stays what mediaRouteForFileName answers (static routes
// and packages' default routes), or Texture for text. The other entries come
// from packages' `opens` (including `"default": false` entries, which exist
// only to be listed here) and the built-in apps that read the format too.
import { PACKAGE_MEDIA_ROUTES, getAppDefinition, type ChoirAppDefinition } from './apps/registry';
import type { PackageMediaRoute } from './apps/packages';
import { MEDIA_FILE_ROUTES, mediaRouteForFileName } from './media-utils.js';

export type FileOpener = {
  appId: string;
  name: string;
  icon: string;
  mediaType: string;
  isDefault: boolean;
  /** 'media' opens through the shell's openmediafile path; 'text' through Texture. */
  via: 'media' | 'text';
};

export const TEXT_OPENER_APP_ID = 'texture';

// Built-in apps that read a format some other app opens by default.
const STATIC_ALSO_OPENS: { appId: string; mediaType: string; extensions: string[] }[] = [
  { appId: 'pdf', mediaType: 'application/pdf', extensions: ['pdf'] },
  { appId: 'epub', mediaType: 'application/epub+zip', extensions: ['epub'] },
  { appId: 'slides', mediaType: 'application/pdf', extensions: ['pdf'] },
];

const TEXT_EXTENSIONS = new Set([
  'txt', 'md', 'markdown', 'rst', 'org', 'texture',
  'json', 'yaml', 'yml', 'toml', 'ini', 'cfg', 'conf',
  'csv', 'tsv', 'log',
  'js', 'jsx', 'ts', 'tsx', 'svelte',
  'go', 'rs', 'py', 'sh', 'bash', 'zsh',
  'css', 'scss', 'html', 'htm', 'xml', 'svg',
  'c', 'h', 'cpp', 'hpp', 'java', 'kt', 'swift', 'rb', 'php', 'pl', 'lua', 'sql',
]);

export function fileExtension(name: string): string {
  const lower = String(name || '').toLowerCase();
  const dot = lower.lastIndexOf('.');
  return dot > 0 && dot < lower.length - 1 ? lower.slice(dot + 1) : '';
}

/** Files that open in Texture as text when no media app claims them. */
export function isTextFileName(name: string): boolean {
  const lower = String(name || '').toLowerCase();
  if (lower === 'makefile' || lower === 'dockerfile') return true;
  const parts = lower.split('.');
  const ext = parts.length > 1 ? parts.pop() || '' : '';
  if (!ext) return true;
  return TEXT_EXTENSIONS.has(ext);
}

function describe(appId: string, fallback: Partial<ChoirAppDefinition> = {}) {
  const def = getAppDefinition(appId);
  return { name: def?.name || fallback.name || appId, icon: def?.icon || fallback.icon || '📱' };
}

/**
 * Every app that opens `fileName`, the default first, then the rest by name.
 * `packageRoutes` defaults to the installed packages' routes (tests pass their
 * own); `mediaType` also matches a package route declared by media type.
 */
export function openersForFile(
  fileName: string,
  options: { packageRoutes?: PackageMediaRoute[]; mediaType?: string } = {},
): FileOpener[] {
  const ext = fileExtension(fileName);
  const packageRoutes = options.packageRoutes || PACKAGE_MEDIA_ROUTES;
  const mediaType = String(options.mediaType || '').toLowerCase();
  const matches = (route: { extensions: string[]; mediaType: string }) =>
    (ext && route.extensions.includes(ext)) || (mediaType && route.mediaType.toLowerCase() === mediaType);

  const out: FileOpener[] = [];
  const seen = new Set<string>();
  const add = (appId: string, type: string, isDefault: boolean, via: FileOpener['via']) => {
    if (!appId || seen.has(appId)) return;
    seen.add(appId);
    out.push({ appId, ...describe(appId), mediaType: type, isDefault, via });
  };

  // The default. Tests may pass their own package routes, so the default is
  // computed against those too.
  const defaultRoute = options.packageRoutes
    ? [...packageRoutes, ...MEDIA_FILE_ROUTES].find((r: any) => r.default !== false && ext && r.extensions.includes(ext))
    : null;
  const route = options.packageRoutes
    ? (defaultRoute ? { appId: defaultRoute.appId, mediaType: defaultRoute.mediaType } : null)
    : mediaRouteForFileName(fileName);
  if (route) add(route.appId, route.mediaType, true, 'media');
  else if (isTextFileName(fileName)) add(TEXT_OPENER_APP_ID, 'text/plain', true, 'text');

  const rest: FileOpener[] = [];
  const restSeen = new Set(seen);
  const addRest = (appId: string, type: string) => {
    if (restSeen.has(appId)) return;
    restSeen.add(appId);
    rest.push({ appId, ...describe(appId), mediaType: type, isDefault: false, via: 'media' });
  };
  for (const r of [...packageRoutes, ...MEDIA_FILE_ROUTES, ...STATIC_ALSO_OPENS]) {
    if (matches(r) && getAppDefinitionOrPackage(r.appId, packageRoutes)) addRest(r.appId, r.mediaType);
  }
  rest.sort((a, b) => a.name.localeCompare(b.name));
  return [...out, ...rest];
}

// A route names an app the registry knows, or (in tests) a package route.
function getAppDefinitionOrPackage(appId: string, packageRoutes: PackageMediaRoute[]) {
  return Boolean(getAppDefinition(appId)) || packageRoutes.some((r) => r.appId === appId);
}

/** The extensions an app opens (for a picker's accept list). Empty: it opens no files. */
export function extensionsOpenedBy(appId: string, packageRoutes: PackageMediaRoute[] = PACKAGE_MEDIA_ROUTES): string[] {
  const exts = new Set<string>();
  for (const r of [...packageRoutes, ...MEDIA_FILE_ROUTES, ...STATIC_ALSO_OPENS]) {
    if (r.appId === appId) r.extensions.forEach((e) => exts.add(e));
  }
  return [...exts];
}

/** True when `appId` is listed for `fileName` (the shell checks an Open with choice). */
export function canOpenWith(appId: string, fileName: string): boolean {
  return openersForFile(fileName).some((o) => o.appId === appId);
}
