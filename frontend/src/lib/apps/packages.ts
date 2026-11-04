// Build-time discovery of first-party app packages
// (docs/design/app-packages-replan-2026-10-24.md §3.1).
//
// A package is a directory src/packages/<name>/ holding choir-package.json and
// App.svelte. Vite resolves both globs at build time, so adding a package
// needs no registry edit. scripts/check-packages.mjs (run by `npm run build`)
// is the gate that refuses a bad package; the defensive skips below only keep
// the shell alive if that gate was bypassed.
import type { ComponentType } from 'svelte';
import { writable } from 'svelte/store';
import { AuthRequiredError, fetchWithRenewal } from '../auth.js';
import type { ChoirAppDefinition } from './registry';

type PackageManifest = {
  schema: string;
  name: string;
  version: string;
  origin: string;
  display: { title: string; icon: string; description: string };
  app: {
    entry: string;
    abi: string;
    aliases?: string[];
    launcher: ChoirAppDefinition['launcher'];
    window: ChoirAppDefinition['window'];
    auth: ChoirAppDefinition['auth'];
    theme: ChoirAppDefinition['theme'];
  };
  opens?: { media_type: string; extensions: string[]; default?: boolean }[];
};

// `default: false` routes offer the package under Open with only; the
// extension keeps its default opener.
export type PackageMediaRoute = { appId: string; mediaType: string; extensions: string[]; default: boolean };

export type LoadedPackages = {
  apps: ChoirAppDefinition[];
  aliases: Record<string, string>;
  mediaRoutes: PackageMediaRoute[];
};

const manifestModules = import.meta.glob('../../packages/*/choir-package.json', {
  eager: true,
  import: 'default',
}) as Record<string, PackageManifest>;

const componentModules = import.meta.glob('../../packages/*/App.svelte') as Record<
  string,
  () => Promise<{ default: ComponentType }>
>;

const isObject = (v: unknown): v is Record<string, any> => v !== null && typeof v === 'object' && !Array.isArray(v);

export function loadPackageApps(reservedIds: ReadonlySet<string>): LoadedPackages {
  const apps: ChoirAppDefinition[] = [];
  const aliases: Record<string, string> = {};
  const mediaRoutes: PackageMediaRoute[] = [];
  const taken = new Set(reservedIds);

  for (const [manifestPath, manifest] of Object.entries(manifestModules).sort(([a], [b]) => a.localeCompare(b))) {
    try {
      const entryPath = manifestPath.replace(/choir-package\.json$/, 'App.svelte');
      const component = componentModules[entryPath];
      const name = manifest?.name;
      if (typeof name !== 'string' || !name || !isObject(manifest.app) || !isObject(manifest.display)) {
        console.warn(`[packages] skipped ${manifestPath}: malformed manifest`);
        continue;
      }
      if (!component) {
        console.warn(`[packages] skipped ${name}: no App.svelte`);
        continue;
      }
      const { launcher, window: windowPref, auth, theme } = manifest.app;
      if (![launcher, windowPref, auth, theme].every(isObject)) {
        console.warn(`[packages] skipped ${name}: launcher, window, auth and theme must be objects`);
        continue;
      }
      const packageAliases = manifest.app.aliases ?? [];
      const opensList = manifest.opens ?? [];
      if (!Array.isArray(packageAliases) || !packageAliases.every((a) => typeof a === 'string') || !Array.isArray(opensList)
        || !opensList.every((o) => isObject(o) && Array.isArray(o.extensions) && (o.default === undefined || typeof o.default === 'boolean'))) {
        console.warn(`[packages] skipped ${name}: aliases (strings), opens and opens[].extensions must be arrays`);
        continue;
      }
      const claimed = [name, ...packageAliases];
      if (claimed.some((id) => taken.has(id))) {
        console.warn(`[packages] skipped ${name}: name or alias already taken`);
        continue;
      }
      // Build everything before mutating shared state, so a skip leaves no trace.
      const app: ChoirAppDefinition = {
        id: name,
        name: manifest.display.title,
        icon: manifest.display.icon,
        description: manifest.display.description,
        component,
        launcher,
        window: windowPref,
        auth,
        theme,
      };
      const routes = opensList.map((o) => ({ appId: name, mediaType: o.media_type, extensions: o.extensions, default: o.default !== false }));
      claimed.forEach((id) => taken.add(id));
      packageAliases.forEach((alias) => {
        aliases[alias] = name;
      });
      apps.push(app);
      mediaRoutes.push(...routes);
    } catch (error) {
      console.warn(`[packages] skipped ${manifestPath}:`, error);
    }
  }
  return { apps, aliases, mediaRoutes };
}

// ---- Runtime install (S12; docs/design/s12-runtime-install-build-2026-11-04.md) ----
//
// An installed package is not built with the shell. The computer built it into
// one module/v1 ES module (mount(target, ctx), abi, requires_sdk, sdk_imports)
// in its module store, outside the source tree. GET /api/packages/installed
// lists the installed set; PackageAppFrame fetches a module by digest,
// refuses bytes whose SHA-256 is not the listed one, and imports the verified
// bytes. An installed package whose name an in-tree package also holds
// replaces it while installed (a track under review), unless the shell's SDK
// is out of its range: then the in-tree package wins.

export const MODULE_ABI = 'choir.app-frontend/module/v1';
const RUNTIME_NAME = /^[a-z][a-z0-9-]{1,39}$/;
const SHA256_HEX = /^[0-9a-f]{64}$/;
export const PACKAGES_CHANGED_EVENT = 'choir:packages-changed';

export type InstalledPackageRecord = {
  name: string;
  release_id: string;
  install_id: string;
  installed_at: string;
  requires_sdk: string;
  manifest: PackageManifest;
  module: { abi: string; sha256: string; url: string };
  available: boolean;
};

/** The computer's installed packages; empty until read, or signed out. */
export const installedPackages = writable<InstalledPackageRecord[]>([]);

function validInstalled(record: any): record is InstalledPackageRecord {
  return Boolean(
    record
      && RUNTIME_NAME.test(String(record.name || ''))
      && isObject(record.manifest)
      && record.manifest.name === record.name
      && isObject(record.manifest.app)
      && isObject(record.manifest.display)
      && isObject(record.module)
      && record.module.abi === MODULE_ABI
      && SHA256_HEX.test(String(record.module.sha256 || ''))
      && record.module.url === `/api/packages/modules/${record.module.sha256}.js`,
  );
}

/**
 * Reads the installed set. Signed out or no route: empty. A transient failure
 * keeps the last known set.
 */
let refreshSeq = 0;

export async function refreshInstalledPackages(): Promise<void> {
  // Only the newest read lands: an older answer never overwrites a newer set.
  const mine = ++refreshSeq;
  const land = (records: InstalledPackageRecord[]) => {
    if (mine === refreshSeq) installedPackages.set(records);
  };
  try {
    const res = await fetchWithRenewal('/api/packages/installed', { cache: 'no-store' });
    if (res.status === 401 || res.status === 403 || res.status === 404) {
      land([]);
      return;
    }
    if (!res.ok) return;
    const body = await res.json();
    land(Array.isArray(body?.packages) ? body.packages.filter(validInstalled) : []);
  } catch (err) {
    // Signed out: nothing is installed for this window. Otherwise keep the
    // last known set; the next change or sign-in reads it again.
    if (err instanceof AuthRequiredError) land([]);
  }
}

/** Sign-out: the previous owner's installed apps leave the launcher at once. */
export function clearInstalledPackages(): void {
  refreshSeq += 1;
  installedPackages.set([]);
}

/** Tells the shell the installed set changed (the Library calls it after an install or uninstall). */
export function notifyPackagesChanged(): void {
  if (typeof window !== 'undefined') window.dispatchEvent(new CustomEvent(PACKAGES_CHANGED_EVENT));
}

if (typeof window !== 'undefined') {
  window.addEventListener(PACKAGES_CHANGED_EVENT, () => {
    void refreshInstalledPackages();
  });
}

/** The app definition of an installed package; the window renders PackageAppFrame. */
export function runtimeAppDefinition(record: InstalledPackageRecord): ChoirAppDefinition | null {
  const m = record.manifest;
  const { launcher, window: windowPref, auth, theme } = m.app;
  if (![launcher, windowPref, auth, theme].every(isObject)) return null;
  return {
    id: record.name,
    name: String(m.display.title || record.name),
    icon: String(m.display.icon || '📦'),
    description: String(m.display.description || ''),
    component: () => import('./PackageAppFrame.svelte'),
    // The desktop icon set is fixed at boot; an installed package is in the launcher.
    launcher: { menu: launcher.menu !== false, desktopIcon: false, order: Number.isFinite(launcher.order) ? launcher.order : 1000 },
    window: windowPref,
    auth,
    theme,
  };
}

export type PackageModule = {
  abi?: string;
  requires_sdk?: string;
  sdk_imports?: string[];
  mount?: (target: HTMLElement, ctx: any) => { update?: (props: any) => void; destroy?: () => void } | void;
};

export type PackageLoadState = 'refused' | 'failed' | 'unavailable';

export class PackageLoadError extends Error {
  state: PackageLoadState;
  constructor(state: PackageLoadState, message: string) {
    super(message);
    this.state = state;
  }
}

async function sha256Hex(bytes: ArrayBuffer): Promise<string> {
  if (!globalThis.crypto?.subtle) throw new PackageLoadError('refused', 'this browser cannot verify package digests (no WebCrypto)');
  const digest = new Uint8Array(await crypto.subtle.digest('SHA-256', bytes));
  return Array.from(digest, (b) => b.toString(16).padStart(2, '0')).join('');
}

/**
 * Fetches an installed package's module, refuses bytes whose SHA-256 is not
 * the listed digest, and imports the verified bytes (never the URL), so what
 * runs is exactly what was checked.
 */
export async function loadPackageModule(record: InstalledPackageRecord): Promise<PackageModule> {
  if (record.module.abi !== MODULE_ABI) throw new PackageLoadError('refused', `unsupported module ABI ${record.module.abi}`);
  const res = await fetchWithRenewal(record.module.url, {});
  if (!res.ok) throw new PackageLoadError('unavailable', `the package's module is unavailable (${res.status})`);
  const bytes = await res.arrayBuffer();
  const actual = await sha256Hex(bytes);
  if (actual !== record.module.sha256) {
    throw new PackageLoadError('refused', `module digest ${actual} is not the installed ${record.module.sha256}`);
  }
  const url = URL.createObjectURL(new Blob([bytes], { type: 'text/javascript' }));
  try {
    return (await import(/* @vite-ignore */ url)) as PackageModule;
  } catch (err) {
    throw new PackageLoadError('failed', `the module failed to load: ${err instanceof Error ? err.message : String(err)}`);
  } finally {
    URL.revokeObjectURL(url);
  }
}
