// Build-time discovery of first-party app packages
// (docs/design/app-packages-replan-2026-10-24.md §3.1).
//
// A package is a directory src/packages/<name>/ holding choir-package.json and
// App.svelte. Vite resolves both globs at build time, so adding a package
// needs no registry edit. scripts/check-packages.mjs (run by `npm run build`)
// is the gate that refuses a bad package; the defensive skips below only keep
// the shell alive if that gate was bypassed.
import type { ComponentType } from 'svelte';
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
  opens?: { media_type: string; extensions: string[] }[];
};

export type PackageMediaRoute = { appId: string; mediaType: string; extensions: string[] };

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
        || !opensList.every((o) => isObject(o) && Array.isArray(o.extensions))) {
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
      const routes = opensList.map((o) => ({ appId: name, mediaType: o.media_type, extensions: o.extensions }));
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
