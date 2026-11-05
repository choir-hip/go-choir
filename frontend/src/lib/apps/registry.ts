import type { ComponentType } from 'svelte';
import { writable } from 'svelte/store';
import { fetchInstalledFiles, installedPackages, isPack, loadPackageApps, runtimeAppDefinition, type InstalledPackageRecord } from './packages';
import { setInstalledThemeFiles } from '../services/theme';
import { sdkSatisfies } from '../sdk-version';
import { setPackageContributions } from '../contributions';

export type AppPreviewPolicy = 'public-preview' | 'public-readonly' | 'private';
export type AppSurfaceKind = 'standard' | 'document' | 'media';

export type WindowGeometry = {
  width?: number;
  height?: number;
  minWidth?: number;
  minHeight?: number;
};

export type ChoirAppDefinition = {
  id: string;
  name: string;
  icon: string;
  description: string;
  component: () => Promise<{ default: ComponentType }>;
  launcher: {
    menu: boolean;
    desktopIcon: boolean;
    order: number;
  };
  window: {
    singleton: boolean;
    heavy: boolean;
    desktop?: WindowGeometry;
    compact?: WindowGeometry;
  };
  auth: {
    preview: AppPreviewPolicy;
  };
  theme: {
    surface: AppSurfaceKind;
    shellDataAttr: string;
    contentClass: string;
  };
};

const windowDefaults = {
  singleton: true,
  heavy: false,
};

const compactDefault = { minWidth: 280, minHeight: 420 };

export const TEXTURE_APP_ID = 'texture';

// Static apps: shell surfaces and apps not yet converted to a package.
// First-party packages (src/packages/<name>/) join them below.
const STATIC_APPS = [
  {
    id: 'browser',
    name: 'Web Lens',
    icon: '🌐',
    description: 'Open a page or read a snapshot',
    component: () => import('../BrowserApp.svelte'),
    launcher: { menu: true, desktopIcon: true, order: 20 },
    window: { ...windowDefaults },
    auth: { preview: 'public-preview' },
    theme: { surface: 'standard', shellDataAttr: 'data-browser-app-container', contentClass: 'browser-content' },
  },
  {
    id: 'email',
    name: 'Mail',
    icon: '📬',
    description: 'Mail for your automatic computer',
    component: () => import('../MailApp.svelte'),
    launcher: { menu: true, desktopIcon: true, order: 30 },
    window: {
      singleton: true,
      heavy: false,
      // A list column plus a reading column, and a message body that fits
      // under the list on shorter screens: Mail needs at least 1180x760.
      desktop: { width: 1180, height: 760, minWidth: 720, minHeight: 500 },
      compact: { width: 360, height: 560, minWidth: 340, minHeight: 520 },
    },
    auth: { preview: 'public-preview' },
    theme: { surface: 'standard', shellDataAttr: 'data-mail-window', contentClass: 'mail-content' },
  },
  {
    id: TEXTURE_APP_ID,
    name: 'Texture',
    icon: '📝',
    description: 'Versioned artifact editor',
    component: () => import('../TextureEditor.svelte'),
    launcher: { menu: true, desktopIcon: true, order: 50 },
    window: {
      singleton: false,
      heavy: true,
      desktop: { width: 960, height: 720, minWidth: 680, minHeight: 520 },
      compact: compactDefault,
    },
    auth: { preview: 'public-preview' },
    theme: { surface: 'document', shellDataAttr: 'data-texture-app', contentClass: 'texture-content' },
  },
  {
    id: 'content',
    name: 'Source',
    icon: '📎',
    description: 'Source substrate viewer',
    component: () => import('../ContentViewer.svelte'),
    launcher: { menu: false, desktopIcon: false, order: 135 },
    window: { singleton: false, heavy: false, desktop: { width: 820, height: 620, minWidth: 560, minHeight: 420 }, compact: compactDefault },
    auth: { preview: 'public-readonly' },
    theme: { surface: 'standard', shellDataAttr: 'data-content-window', contentClass: 'content-viewer-content' },
  },
  {
    id: 'settings',
    name: 'Settings',
    icon: '⚙️',
    description: 'Desktop settings',
    component: () => import('../SettingsApp.svelte'),
    launcher: { menu: true, desktopIcon: true, order: 150 },
    window: { singleton: true, heavy: false, desktop: { width: 940, height: 720 } },
    auth: { preview: 'public-readonly' },
    theme: { surface: 'standard', shellDataAttr: 'data-settings-window', contentClass: 'settings-content' },
  },
] satisfies ChoirAppDefinition[];

const PACKAGES = loadPackageApps(new Set(STATIC_APPS.map((app) => app.id)));

// What a package contributes beside its window (commands, keybindings, ...):
// handed to the owning services in the package's scope; an installed release
// replaces the in-tree twin's set and uninstall restores it.
const TREE_CONTRIBUTIONS = new Map(PACKAGES.contributions.map((c) => [c.name, c.contributes]));
for (const [name, contributes] of TREE_CONTRIBUTIONS) void setPackageContributions(name, contributes, `tree:${name}`);
let contributedByInstall = new Set<string>();
const contributedFingerprints = new Map<string, string>();

export const APP_REGISTRY: ChoirAppDefinition[] = [...STATIC_APPS, ...PACKAGES.apps];

// File routes contributed by packages' `opens`; media-utils.js consults them first.
export const PACKAGE_MEDIA_ROUTES = PACKAGES.mediaRoutes;

export function normalizeAppId(appId: string): string {
  const id = String(appId || '').trim();
  return Object.prototype.hasOwnProperty.call(PACKAGES.aliases, id) ? PACKAGES.aliases[id] : id;
}

export const LAUNCHER_APPS = APP_REGISTRY
  .filter((app) => app.launcher.menu)
  .sort((a, b) => a.launcher.order - b.launcher.order);

export const DESKTOP_ICON_APPS = APP_REGISTRY
  .filter((app) => app.launcher.desktopIcon)
  .sort((a, b) => a.launcher.order - b.launcher.order);

export const HEAVY_APP_IDS = new Set(APP_REGISTRY.filter((app) => app.window.heavy).map((app) => app.id));

// Runtime install (S12): the installed packages join the registry when the
// computer's installed set is read, and leave it on uninstall. The arrays
// above are updated in place, and registryRevision counts the changes, so a
// component that lists apps reads `$registryRevision` to follow them.
// DESKTOP_ICON_APPS stays the boot set (installed packages are launcher-only).
export const registryRevision = writable(0);
const BUILT_IN: ChoirAppDefinition[] = [...APP_REGISTRY];
const STATIC_IDS = new Set(STATIC_APPS.map((app) => app.id));
// One definition per install and module: a refresh that leaves a package's
// install unchanged keeps its definition object, so AppHost (which reloads on
// a new definition) never remounts an open window for it.
let runtimeDefinitions = new Map<string, ChoirAppDefinition>();

export function applyInstalledPackages(records: InstalledPackageRecord[]): void {
  const runtime = new Map<string, ChoirAppDefinition>();
  const kept = new Map<string, ChoirAppDefinition>();
  for (const record of records) {
    // A static app is never replaced; an in-tree package is, unless the
    // shell's SDK is out of the installed package's range (the base wins).
    if (STATIC_IDS.has(record.name) || runtime.has(record.name)) continue;
    // The in-tree twin wins when the installed module cannot run here: the
    // SDK is out of its range, or its module is not in the store.
    const base = BUILT_IN.some((app) => app.id === record.name);
    if (base && (!sdkSatisfies(record.requires_sdk) || !record.available)) continue;
    const key = `${record.install_id}|${record.module.sha256}`;
    const app = runtimeDefinitions.get(key) || runtimeAppDefinition(record);
    if (app) {
      runtime.set(record.name, app);
      kept.set(key, app);
    }
  }
  runtimeDefinitions = kept;
  const nowInstalled = new Set<string>();
  // Apps and packs both contribute (a pack has no window): the files are
  // fetched and digest-checked first so a theme resolves when it registers.
  for (const record of records) {
    if (STATIC_IDS.has(record.name)) continue;
    if (!isPack(record) && !runtime.has(record.name)) continue;
    if (isPack(record) && !record.available) continue;
    nowInstalled.add(record.name);
    const fingerprint = `${record.install_id}|${record.module.sha256}`;
    if (contributedFingerprints.get(record.name) === fingerprint) continue;
    contributedFingerprints.set(record.name, fingerprint);
    void fetchInstalledFiles(record).then((files) => {
      setInstalledThemeFiles(record.name, files);
      return setPackageContributions(record.name, (record.manifest as any)?.contributes, fingerprint);
    });
  }
  for (const name of contributedByInstall) {
    if (nowInstalled.has(name)) continue;
    setInstalledThemeFiles(name, null);
    contributedFingerprints.delete(name);
    void setPackageContributions(name, TREE_CONTRIBUTIONS.get(name) ?? null, `tree:${name}`);
  }
  contributedByInstall = nowInstalled;
  const next = [...BUILT_IN.filter((app) => !runtime.has(app.id)), ...runtime.values()];
  APP_REGISTRY.splice(0, APP_REGISTRY.length, ...next);
  LAUNCHER_APPS.splice(0, LAUNCHER_APPS.length, ...next.filter((app) => app.launcher.menu).sort((a, b) => a.launcher.order - b.launcher.order));
  HEAVY_APP_IDS.clear();
  next.filter((app) => app.window.heavy).forEach((app) => HEAVY_APP_IDS.add(app.id));
  registryRevision.update((n) => n + 1);
}

installedPackages.subscribe((records) => {
  // The first call (the empty initial set) changes nothing.
  if (records.length === 0 && APP_REGISTRY.length === BUILT_IN.length && APP_REGISTRY.every((app, i) => app === BUILT_IN[i])) return;
  applyInstalledPackages(records);
});

export function getAppDefinition(appId: string): ChoirAppDefinition | null {
  const canonicalAppId = normalizeAppId(appId);
  return APP_REGISTRY.find((app) => app.id === canonicalAppId) || null;
}

export function getAppIcon(appId: string): string {
  return getAppDefinition(appId)?.icon || '📱';
}

export function getAppWindowPreference(appId: string): ChoirAppDefinition['window'] {
  return getAppDefinition(appId)?.window || { ...windowDefaults };
}

export function isHeavyAppId(appId: string): boolean {
  return Boolean(getAppDefinition(appId)?.window.heavy);
}
