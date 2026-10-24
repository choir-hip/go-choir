import type { ComponentType } from 'svelte';
import { loadPackageApps } from './packages';

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
    id: 'files',
    name: 'Files',
    icon: '📁',
    description: 'File Browser',
    component: () => import('../FileBrowser.svelte'),
    launcher: { menu: true, desktopIcon: true, order: 10 },
    window: { ...windowDefaults },
    auth: { preview: 'public-preview' },
    theme: { surface: 'standard', shellDataAttr: 'data-files-app', contentClass: 'files-content' },
  },
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
      desktop: { width: 1180, height: 740, minWidth: 720, minHeight: 500 },
      compact: { width: 360, height: 560, minWidth: 340, minHeight: 520 },
    },
    auth: { preview: 'public-preview' },
    theme: { surface: 'standard', shellDataAttr: 'data-mail-window', contentClass: 'mail-content' },
  },
  {
    id: 'compute-monitor',
    name: 'Compute Monitor',
    icon: '📊',
    description: 'User computer health and recovery',
    component: () => import('../ComputeMonitorApp.svelte'),
    launcher: { menu: true, desktopIcon: true, order: 40 },
    window: { singleton: true, heavy: false, desktop: { width: 980, height: 700, minWidth: 700, minHeight: 520 }, compact: compactDefault },
    auth: { preview: 'public-preview' },
    theme: { surface: 'standard', shellDataAttr: 'data-compute-monitor-window', contentClass: 'compute-monitor-content' },
  },
  {
    id: 'pulse',
    name: 'Pulse',
    icon: '📈',
    description: 'Public aggregate Choir health',
    component: () => import('../PulseApp.svelte'),
    launcher: { menu: true, desktopIcon: true, order: 45 },
    window: { singleton: true, heavy: false, desktop: { width: 980, height: 720, minWidth: 700, minHeight: 520 }, compact: compactDefault },
    auth: { preview: 'public-readonly' },
    theme: { surface: 'standard', shellDataAttr: 'data-pulse-window', contentClass: 'pulse-content' },
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
    id: 'autopaper',
    name: 'Autopaper',
    icon: '📰',
    description: 'Living stories with checked sources',
    component: () => import('../AutopaperApp.svelte'),
    launcher: { menu: true, desktopIcon: true, order: 65 },
    window: {
      singleton: true,
      heavy: false,
      desktop: { width: 880, height: 760, minWidth: 360, minHeight: 480 },
      compact: compactDefault,
    },
    auth: { preview: 'public-readonly' },
    theme: { surface: 'document', shellDataAttr: 'data-autopaper-window', contentClass: 'autopaper-content' },
  },
  {
    id: 'podcast',
    name: 'Podcast',
    icon: '📡',
    description: 'Podcast feed player',
    component: () => import('../PodcastApp.svelte'),
    launcher: { menu: true, desktopIcon: true, order: 70 },
    window: { singleton: false, heavy: true, desktop: { width: 900, height: 660 }, compact: compactDefault },
    auth: { preview: 'public-preview' },
    theme: { surface: 'media', shellDataAttr: 'data-podcast-window', contentClass: 'podcast-content' },
  },
  {
    id: 'audio',
    name: 'Audio',
    icon: '🎧',
    description: 'Audio player',
    component: () => import('../AudioApp.svelte'),
    launcher: { menu: true, desktopIcon: false, order: 90 },
    window: { singleton: false, heavy: true, desktop: { width: 760, height: 420 }, compact: { minWidth: 280, minHeight: 320 } },
    auth: { preview: 'public-preview' },
    theme: { surface: 'media', shellDataAttr: 'data-audio-window', contentClass: 'audio-content' },
  },
  {
    id: 'video',
    name: 'Video',
    icon: '🎬',
    description: 'Video and YouTube player',
    component: () => import('../VideoApp.svelte'),
    launcher: { menu: true, desktopIcon: false, order: 100 },
    window: { singleton: false, heavy: true, desktop: { width: 980, height: 720 }, compact: compactDefault },
    auth: { preview: 'public-preview' },
    theme: { surface: 'media', shellDataAttr: 'data-video-window', contentClass: 'video-content' },
  },
  {
    id: 'pdf',
    name: 'PDF',
    icon: '📄',
    description: 'PDF reader',
    component: () => import('../PdfApp.svelte'),
    launcher: { menu: true, desktopIcon: false, order: 110 },
    window: { singleton: false, heavy: true, desktop: { width: 940, height: 720 }, compact: compactDefault },
    auth: { preview: 'public-preview' },
    theme: { surface: 'document', shellDataAttr: 'data-pdf-window', contentClass: 'pdf-content' },
  },
  {
    id: 'epub',
    name: 'EPUB',
    icon: '📚',
    description: 'EPUB reader',
    component: () => import('../EpubApp.svelte'),
    launcher: { menu: true, desktopIcon: false, order: 120 },
    window: { singleton: false, heavy: true, desktop: { width: 900, height: 700 }, compact: compactDefault },
    auth: { preview: 'public-preview' },
    theme: { surface: 'document', shellDataAttr: 'data-epub-window', contentClass: 'epub-content' },
  },
  {
    id: 'slides',
    name: 'Slides',
    icon: '🖥️',
    description: 'Slide player for PPTX, PDF, and HTML',
    component: () => import('../SlidesApp.svelte'),
    launcher: { menu: true, desktopIcon: false, order: 125 },
    window: { singleton: false, heavy: true, desktop: { width: 1024, height: 768, minWidth: 640, minHeight: 480 }, compact: compactDefault },
    auth: { preview: 'public-preview' },
    theme: { surface: 'media', shellDataAttr: 'data-slides-window', contentClass: 'slides-content' },
  },
  {
    id: 'calendar',
    name: 'Calendar',
    icon: '📅',
    description: 'Calendar with ICS import',
    component: () => import('../CalendarApp.svelte'),
    launcher: { menu: true, desktopIcon: true, order: 115 },
    window: { singleton: true, heavy: false, desktop: { width: 960, height: 720, minWidth: 640, minHeight: 480 }, compact: compactDefault },
    auth: { preview: 'public-preview' },
    theme: { surface: 'standard', shellDataAttr: 'data-calendar-window', contentClass: 'calendar-content' },
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
