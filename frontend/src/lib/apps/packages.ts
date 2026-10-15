// App packages (S7-0a, docs/design/s7-0-app-packages-2026-10-14.md §9).
//
// The static APP_REGISTRY stays the shell's own apps. Packages sit beside it:
// the computer's installed set is the fold of its package events on the tape,
// served by GET /api/app-packages, and each enabled package becomes an app
// with id "package:<name>". Its frontend is one ES module (ABI
// choir.app-frontend/v1: mount(target, ctx) -> { destroy() }) that the shell
// fetches, checks against the SHA-256 bound on the tape, and imports from the
// verified bytes. It never shares the shell's Svelte runtime.
import { derived, get, writable } from 'svelte/store';
import type { ChoirAppDefinition } from './registry';

export const PACKAGE_APP_PREFIX = 'package:';
export const PACKAGE_FRONTEND_ABI = 'choir.app-frontend/v1';

const PACKAGE_NAME = /^[a-z][a-z0-9-]{1,39}$/;
const DIGEST = /^sha256:[0-9a-f]{64}$/;

export type PackageRecord = {
  name: string;
  version: string;
  status: 'installed' | 'enabled' | 'disabled';
  title: string;
  icon?: string;
  description?: string;
  manifest_digest: string;
  build_digest: string;
  data_store: string;
  available: boolean;
  window?: { singleton?: boolean; width?: number; height?: number };
  frontend: { entry: string; abi: string; url: string; sha256: string };
};

/** Every installed package on this computer, any status. */
export const installedPackages = writable<PackageRecord[]>([]);
/** True once the inventory has been read at least once this session. */
export const packagesLoaded = writable(false);

/** Desk-launchable apps: enabled packages only. */
export const packageApps = derived(installedPackages, ($packages) =>
  $packages.filter((record) => record.status === 'enabled').map((record) => packageDefinition(record.name, record)),
);

export function packageAppId(name: string): string {
  return `${PACKAGE_APP_PREFIX}${name}`;
}

export function packageNameFromAppId(appId: string): string | null {
  const id = String(appId || '').trim();
  if (!id.startsWith(PACKAGE_APP_PREFIX)) return null;
  const name = id.slice(PACKAGE_APP_PREFIX.length);
  return PACKAGE_NAME.test(name) ? name : null;
}

export function getPackageRecord(name: string): PackageRecord | null {
  return get(installedPackages).find((record) => record.name === name) || null;
}

/**
 * The app definition for a "package:<name>" id, whether or not that package
 * is installed or enabled: the frame it renders decides what to show, so a
 * window restored before the inventory loads (or after a disable) never
 * reads as "Unknown app" and never fetches code it should not.
 */
export function getPackageAppDefinition(appId: string): ChoirAppDefinition | null {
  const name = packageNameFromAppId(appId);
  if (!name) return null;
  return packageDefinition(name, getPackageRecord(name));
}

function packageDefinition(name: string, record: PackageRecord | null): ChoirAppDefinition {
  const width = clampSize(record?.window?.width, 640);
  const height = clampSize(record?.window?.height, 480);
  return {
    id: packageAppId(name),
    name: record?.title || name,
    icon: record?.icon || '📦',
    description: record?.description || `App package ${name}`,
    component: () => import('./PackageAppFrame.svelte'),
    launcher: { desk: true, desktopIcon: false, order: 1000 },
    window: {
      singleton: record?.window?.singleton !== false,
      heavy: false,
      desktop: { width, height, minWidth: 320, minHeight: 240 },
      compact: { minWidth: 280, minHeight: 320 },
    },
    auth: { preview: 'private' },
    theme: { surface: 'standard', shellDataAttr: 'data-package-app', contentClass: 'package-content' },
  };
}

function clampSize(value: unknown, fallback: number): number {
  const n = Number(value);
  if (!Number.isFinite(n) || n <= 0) return fallback;
  return Math.min(Math.max(Math.round(n), 240), 4096);
}

function validRecord(record: any): record is PackageRecord {
  return Boolean(
    record &&
      PACKAGE_NAME.test(String(record.name || '')) &&
      ['installed', 'enabled', 'disabled'].includes(record.status) &&
      record.frontend &&
      typeof record.frontend.url === 'string' &&
      record.frontend.url.startsWith('/api/app-packages/builds/') &&
      DIGEST.test(String(record.frontend.sha256 || '')),
  );
}

/**
 * Reads the computer's package inventory. An unauthenticated or absent API
 * empties the set; a transient failure keeps the last known set.
 */
export async function refreshInstalledPackages(): Promise<void> {
  try {
    const res = await fetch('/api/app-packages', { credentials: 'same-origin', cache: 'no-store' });
    if (res.status === 401 || res.status === 404) {
      installedPackages.set([]);
      packagesLoaded.set(true);
      return;
    }
    if (!res.ok) return;
    const body = await res.json();
    const records = Array.isArray(body?.packages) ? body.packages.filter(validRecord) : [];
    installedPackages.set(records);
    packagesLoaded.set(true);
  } catch {
    // Keep the last known set; the next refresh retries.
  }
}

export function clearInstalledPackages(): void {
  installedPackages.set([]);
  packagesLoaded.set(false);
}

export type PackageLoadState = 'refused' | 'failed' | 'unavailable';

export class PackageLoadError extends Error {
  state: PackageLoadState;
  constructor(state: PackageLoadState, message: string) {
    super(message);
    this.state = state;
  }
}

async function sha256(bytes: ArrayBuffer): Promise<string> {
  if (!globalThis.crypto?.subtle) {
    throw new PackageLoadError('refused', 'this browser cannot verify package digests (no WebCrypto)');
  }
  const digest = new Uint8Array(await crypto.subtle.digest('SHA-256', bytes));
  return `sha256:${Array.from(digest, (b) => b.toString(16).padStart(2, '0')).join('')}`;
}

export type PackageModule = {
  mount?: (target: HTMLElement, ctx: PackageContext) => { destroy?: () => void } | void;
};

/**
 * Fetches a package's frontend module, refuses bytes whose SHA-256 is not the
 * digest bound on the tape, and imports the verified bytes (never the URL),
 * so what runs is exactly what was checked.
 */
export async function loadPackageModule(record: PackageRecord): Promise<PackageModule> {
  if (record.frontend.abi !== PACKAGE_FRONTEND_ABI) {
    throw new PackageLoadError('refused', `unsupported frontend ABI ${record.frontend.abi}`);
  }
  const res = await fetch(record.frontend.url, { credentials: 'same-origin' });
  if (!res.ok) {
    throw new PackageLoadError('unavailable', `package bytes are unavailable (${res.status})`);
  }
  const bytes = await res.arrayBuffer();
  const actual = await sha256(bytes);
  if (actual !== record.frontend.sha256) {
    throw new PackageLoadError('refused', `module digest ${actual} does not match the installed ${record.frontend.sha256}`);
  }
  const url = URL.createObjectURL(new Blob([bytes], { type: 'text/javascript' }));
  try {
    return (await import(/* @vite-ignore */ url)) as PackageModule;
  } catch (err) {
    throw new PackageLoadError('failed', `module failed to evaluate: ${err instanceof Error ? err.message : String(err)}`);
  } finally {
    URL.revokeObjectURL(url);
  }
}

export type PackageContext = {
  package: string;
  version: string;
  windowId: string;
  data: {
    get(key: string): Promise<unknown>;
    put(key: string, value: unknown): Promise<void>;
  };
};

/** The package_kv/v1 client a package receives, bound to its own name. */
export function packageContext(record: PackageRecord, windowId: string): PackageContext {
  const base = `/api/app-packages/${encodeURIComponent(record.name)}/data/`;
  return Object.freeze({
    package: record.name,
    version: record.version,
    windowId,
    data: Object.freeze({
      async get(key: string) {
        const res = await fetch(base + encodeURIComponent(key), { credentials: 'same-origin', cache: 'no-store' });
        if (res.status === 404) return undefined;
        if (!res.ok) throw new Error(`package data read failed (${res.status})`);
        const body = await res.json();
        return body?.value;
      },
      async put(key: string, value: unknown) {
        const res = await fetch(base + encodeURIComponent(key), {
          method: 'PUT',
          credentials: 'same-origin',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ value }),
        });
        if (!res.ok) throw new Error(`package data write failed (${res.status})`);
      },
    }),
  });
}
