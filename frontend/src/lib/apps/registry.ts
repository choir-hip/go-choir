import type { ComponentType } from 'svelte';

export type AppPreviewPolicy = 'public-preview' | 'public-readonly' | 'private';
export type AppSurfaceKind = 'standard' | 'document' | 'media' | 'terminal';

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
    desk: boolean;
    desktopIcon: boolean;
    mobileSwitcher: boolean;
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
    requiresAuthFor: string[];
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

export const APP_REGISTRY = [
  {
    id: 'files',
    name: 'Files',
    icon: '📁',
    description: 'File Browser',
    component: () => import('../FileBrowser.svelte'),
    launcher: { desk: true, desktopIcon: true, mobileSwitcher: true, order: 10 },
    window: { ...windowDefaults },
    auth: { preview: 'public-preview', requiresAuthFor: ['file_mutation', 'file_upload'] },
    theme: { surface: 'standard', shellDataAttr: 'data-files-app', contentClass: 'files-content' },
  },
  {
    id: 'browser',
    name: 'Web Lens',
    icon: '🌐',
    description: 'Web snapshots and imports',
    component: () => import('../BrowserApp.svelte'),
    launcher: { desk: true, desktopIcon: true, mobileSwitcher: true, order: 20 },
    window: { ...windowDefaults },
    auth: { preview: 'public-preview', requiresAuthFor: ['web_import', 'provider_spend'] },
    theme: { surface: 'standard', shellDataAttr: 'data-browser-app-container', contentClass: 'browser-content' },
  },
  {
    id: 'email',
    name: 'Email',
    icon: '✉️',
    description: 'Mail for your automatic computer',
    component: () => import('../EmailApp.svelte'),
    launcher: { desk: true, desktopIcon: true, mobileSwitcher: true, order: 30 },
    window: {
      singleton: true,
      heavy: false,
      desktop: { width: 1120, height: 720, minWidth: 760, minHeight: 520 },
      compact: { width: 360, height: 560, minWidth: 340, minHeight: 520 },
    },
    auth: { preview: 'public-preview', requiresAuthFor: ['email_reply', 'email_compose', 'email_send'] },
    theme: { surface: 'standard', shellDataAttr: 'data-email-window', contentClass: 'email-content' },
  },
  {
    id: 'compute-monitor',
    name: 'Compute Monitor',
    icon: '📊',
    description: 'User computer health and recovery',
    component: () => import('../ComputeMonitorApp.svelte'),
    launcher: { desk: true, desktopIcon: true, mobileSwitcher: true, order: 40 },
    window: { singleton: true, heavy: false, desktop: { width: 980, height: 700, minWidth: 700, minHeight: 520 }, compact: compactDefault },
    auth: { preview: 'public-preview', requiresAuthFor: ['wake_computer', 'suspend_background', 'reset_desktop'] },
    theme: { surface: 'standard', shellDataAttr: 'data-compute-monitor-window', contentClass: 'compute-monitor-content' },
  },
  {
    id: 'vtext',
    name: 'VText',
    icon: '📝',
    description: 'Versioned document editor',
    component: () => import('../VTextEditor.svelte'),
    launcher: { desk: true, desktopIcon: true, mobileSwitcher: true, order: 50 },
    window: {
      singleton: false,
      heavy: true,
      desktop: { width: 960, height: 720, minWidth: 680, minHeight: 520 },
      compact: compactDefault,
    },
    auth: { preview: 'public-preview', requiresAuthFor: ['save_vtext', 'revise_vtext', 'publish_vtext'] },
    theme: { surface: 'document', shellDataAttr: 'data-vtext-app', contentClass: 'vtext-content' },
  },
  {
    id: 'trace',
    name: 'Trace',
    icon: '🔎',
    description: 'Multiagent trace viewer',
    component: () => import('../TraceApp.svelte'),
    launcher: { desk: true, desktopIcon: true, mobileSwitcher: true, order: 60 },
    window: { singleton: true, heavy: true, desktop: { width: 1040, height: 680 } },
    auth: { preview: 'public-preview', requiresAuthFor: ['private_trace'] },
    theme: { surface: 'standard', shellDataAttr: 'data-trace-window', contentClass: 'trace-content' },
  },
  {
    id: 'podcast',
    name: 'Podcast',
    icon: '📡',
    description: 'Podcast feed player',
    component: () => import('../PodcastApp.svelte'),
    launcher: { desk: true, desktopIcon: true, mobileSwitcher: true, order: 70 },
    window: { singleton: false, heavy: true, desktop: { width: 900, height: 660 }, compact: compactDefault },
    auth: { preview: 'public-preview', requiresAuthFor: ['podcast_import', 'provider_spend'] },
    theme: { surface: 'media', shellDataAttr: 'data-podcast-window', contentClass: 'podcast-content' },
  },
  {
    id: 'image',
    name: 'Image',
    icon: '🖼️',
    description: 'Image viewer',
    component: () => import('../ImageApp.svelte'),
    launcher: { desk: true, desktopIcon: false, mobileSwitcher: true, order: 80 },
    window: { singleton: false, heavy: true, desktop: { width: 900, height: 680 }, compact: compactDefault },
    auth: { preview: 'public-preview', requiresAuthFor: ['file_upload'] },
    theme: { surface: 'media', shellDataAttr: 'data-image-window', contentClass: 'image-content' },
  },
  {
    id: 'audio',
    name: 'Audio',
    icon: '🎧',
    description: 'Audio player',
    component: () => import('../AudioApp.svelte'),
    launcher: { desk: true, desktopIcon: false, mobileSwitcher: true, order: 90 },
    window: { singleton: false, heavy: true, desktop: { width: 760, height: 420 }, compact: { minWidth: 280, minHeight: 320 } },
    auth: { preview: 'public-preview', requiresAuthFor: ['file_upload'] },
    theme: { surface: 'media', shellDataAttr: 'data-audio-window', contentClass: 'audio-content' },
  },
  {
    id: 'video',
    name: 'Video',
    icon: '🎬',
    description: 'Video and YouTube player',
    component: () => import('../VideoApp.svelte'),
    launcher: { desk: true, desktopIcon: false, mobileSwitcher: true, order: 100 },
    window: { singleton: false, heavy: true, desktop: { width: 980, height: 720 }, compact: compactDefault },
    auth: { preview: 'public-preview', requiresAuthFor: ['file_upload', 'provider_spend'] },
    theme: { surface: 'media', shellDataAttr: 'data-video-window', contentClass: 'video-content' },
  },
  {
    id: 'pdf',
    name: 'PDF',
    icon: '📄',
    description: 'PDF reader',
    component: () => import('../PdfApp.svelte'),
    launcher: { desk: true, desktopIcon: false, mobileSwitcher: true, order: 110 },
    window: { singleton: false, heavy: true, desktop: { width: 940, height: 720 }, compact: compactDefault },
    auth: { preview: 'public-preview', requiresAuthFor: ['file_upload'] },
    theme: { surface: 'document', shellDataAttr: 'data-pdf-window', contentClass: 'pdf-content' },
  },
  {
    id: 'epub',
    name: 'EPUB',
    icon: '📚',
    description: 'EPUB reader',
    component: () => import('../EpubApp.svelte'),
    launcher: { desk: true, desktopIcon: false, mobileSwitcher: true, order: 120 },
    window: { singleton: false, heavy: true, desktop: { width: 900, height: 700 }, compact: compactDefault },
    auth: { preview: 'public-preview', requiresAuthFor: ['file_upload'] },
    theme: { surface: 'document', shellDataAttr: 'data-epub-window', contentClass: 'epub-content' },
  },
  {
    id: 'features',
    name: 'Features',
    icon: '🎬',
    description: 'Watch demos and import features',
    component: () => import('../FeaturesApp.svelte'),
    launcher: { desk: true, desktopIcon: false, mobileSwitcher: true, order: 130 },
    window: { singleton: true, heavy: true, desktop: { width: 1100, height: 760, minWidth: 760, minHeight: 540 }, compact: compactDefault },
    auth: { preview: 'public-preview', requiresAuthFor: ['feature_import', 'feature_activate'] },
    theme: { surface: 'standard', shellDataAttr: 'data-features-window', contentClass: 'features-content' },
  },
  {
    id: 'terminal',
    name: 'Terminal',
    icon: '💻',
    description: 'Terminal',
    component: () => import('../TerminalApp.svelte'),
    launcher: { desk: true, desktopIcon: true, mobileSwitcher: true, order: 140 },
    window: { singleton: true, heavy: true },
    auth: { preview: 'public-preview', requiresAuthFor: ['terminal_session'] },
    theme: { surface: 'terminal', shellDataAttr: 'data-terminal-app', contentClass: 'terminal-content' },
  },
  {
    id: 'settings',
    name: 'Settings',
    icon: '⚙️',
    description: 'Desktop settings',
    component: () => import('../SettingsApp.svelte'),
    launcher: { desk: true, desktopIcon: true, mobileSwitcher: true, order: 150 },
    window: { singleton: true, heavy: false, desktop: { width: 940, height: 720 } },
    auth: { preview: 'public-readonly', requiresAuthFor: ['reset_desktop', 'account_settings'] },
    theme: { surface: 'standard', shellDataAttr: 'data-settings-window', contentClass: 'settings-content' },
  },
] satisfies ChoirAppDefinition[];

export const DESK_APPS = APP_REGISTRY
  .filter((app) => app.launcher.desk)
  .sort((a, b) => a.launcher.order - b.launcher.order);

export const DESKTOP_ICON_APPS = APP_REGISTRY
  .filter((app) => app.launcher.desktopIcon)
  .sort((a, b) => a.launcher.order - b.launcher.order);

export const MOBILE_SWITCHER_APPS = APP_REGISTRY
  .filter((app) => app.launcher.mobileSwitcher)
  .sort((a, b) => a.launcher.order - b.launcher.order);

export const HEAVY_APP_IDS = new Set(APP_REGISTRY.filter((app) => app.window.heavy).map((app) => app.id));

export function getAppDefinition(appId: string): ChoirAppDefinition | null {
  return APP_REGISTRY.find((app) => app.id === appId) || null;
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
