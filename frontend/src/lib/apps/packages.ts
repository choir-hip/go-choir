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

export function loadPackageApps(reservedIds: ReadonlySet<string>): LoadedPackages {
  const apps: ChoirAppDefinition[] = [];
  const aliases: Record<string, string> = {};
  const mediaRoutes: PackageMediaRoute[] = [];
  const taken = new Set(reservedIds);

  for (const [manifestPath, manifest] of Object.entries(manifestModules).sort(([a], [b]) => a.localeCompare(b))) {
    const entryPath = manifestPath.replace(/choir-package\.json$/, 'App.svelte');
    const component = componentModules[entryPath];
    const name = manifest?.name;
    if (!name || !manifest.app || !manifest.display) {
      console.warn(`[packages] skipped ${manifestPath}: malformed manifest`);
      continue;
    }
    if (!component) {
      console.warn(`[packages] skipped ${name}: no App.svelte`);
      continue;
    }
    const packageAliases = manifest.app.aliases || [];
    const claimed = [name, ...packageAliases];
    if (claimed.some((id) => taken.has(id))) {
      console.warn(`[packages] skipped ${name}: name or alias already taken`);
      continue;
    }
    claimed.forEach((id) => taken.add(id));
    packageAliases.forEach((alias) => {
      aliases[alias] = name;
    });
    apps.push({
      id: name,
      name: manifest.display.title,
      icon: manifest.display.icon,
      description: manifest.display.description,
      component,
      launcher: manifest.app.launcher,
      window: manifest.app.window,
      auth: manifest.app.auth,
      theme: manifest.app.theme,
    });
    for (const opens of manifest.opens || []) {
      mediaRoutes.push({ appId: name, mediaType: opens.media_type, extensions: opens.extensions });
    }
  }
  return { apps, aliases, mediaRoutes };
}
