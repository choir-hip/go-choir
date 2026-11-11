// The theme service (slice S5; docs/design/modular-architecture-2026-11-04.md
// sections 2.2 and 5). Three jobs:
//  - getTheme / setTheme / onThemeChange, the SDK's replacement for the window
//    `choir-theme-change` event and for reading the theme preference. The shell
//    (App.svelte) binds the host that applies and persists a theme and
//    publishes the applied one back here;
//  - contributes.themes: a pack's `{ id, file }` entries become selectable
//    themes (theme.ts registerTheme), data only, validated against every token
//    key, the safe-CSS-value gate and the contrast gate;
//  - a selected theme survives load order: if the owner's stored theme names a
//    contributed id that has not registered yet, it is applied the moment the
//    pack does, and when the pack goes the shell falls back to the default
//    (the owner's stored choice is untouched, so reinstalling brings it back).
//
// Failure modes:
//  T1 a contributed theme missing a token, with an unsafe value, or below the
//     contrast gate: refused at registration, the pack's other entries apply;
//  T2 a theme id equal to a built-in or another pack's: refused, never shadows;
//  T3 the selected contributed theme is uninstalled: the system default is applied
//     without persisting, and the choice returns if the pack does;
//  T4 a theme file the package does not carry: that entry is skipped;
//  T5 setTheme with an unknown id or an invalid config: { ok: false, errors },
//     nothing applied.
import { get, writable } from 'svelte/store';
import { registerContributionPoint } from '../contributions';
import {
  contrastErrors,
  isKnownThemeId,
  listThemes,
  normalizeThemeConfig,
  registerTheme,
  systemDefaultTheme,
  validateThemeConfig,
  type ChoirTheme,
} from '../theme';

export type ThemeHost = { apply(theme: unknown, persist: boolean): { ok: boolean; errors?: string[] } };

let host: ThemeHost | null = null;
const current = writable<ChoirTheme>(systemDefaultTheme());
let wanted: unknown = null; // the owner's stored theme when its id was not selectable yet (T3)

export function bindThemeHost(next: ThemeHost): () => void {
  host = next;
  return () => { if (host === next) host = null; };
}

/** The shell reports the theme it applied (App.svelte). */
export function publishTheme(theme: ChoirTheme): void {
  current.set(theme);
}

/** The shell reports a stored theme it could not select yet (a contributed id), to apply when it registers. */
export function rememberWantedTheme(theme: unknown): void {
  wanted = theme && typeof theme === 'object' && !isKnownThemeId((theme as any).id) ? theme : null;
}

export function getTheme(): ChoirTheme {
  return get(current);
}

/** Select a theme by id (built-in or contributed) or by a full config. */
export function setTheme(idOrConfig: string | Record<string, unknown>): { ok: boolean; errors: string[] } {
  const candidate = typeof idOrConfig === 'string' ? listThemes().find((t) => t.id === idOrConfig) : idOrConfig;
  if (!candidate) return { ok: false, errors: [`unknown theme "${String(idOrConfig)}"`] };
  const validation = validateThemeConfig(candidate);
  if (!validation.ok) return { ok: false, errors: validation.errors };
  // The normalized theme is what would apply: refuse it if text on its surfaces is unreadable (N8, the same gate a contributed theme passes).
  const normalized = normalizeThemeConfig(candidate);
  const unreadable = contrastErrors(normalized.colors);
  if (unreadable.length) return { ok: false, errors: unreadable };
  if (!host) return { ok: false, errors: ['no shell is mounted to apply the theme'] };
  const applied = host.apply(normalized, true);
  return { ok: applied.ok !== false, errors: applied.errors ?? [] };
}

/** Called after every applied theme change; returns the disposer. */
export function onThemeChange(fn: (theme: ChoirTheme) => void): () => void {
  let first = true;
  return current.subscribe((theme) => {
    if (first) { first = false; return; }
    fn(theme);
  });
}

// ---- contributes.themes ----

// In-tree packages carry their theme files beside the manifest.
const treeFiles = import.meta.glob('../../packages/**/*.json', { eager: true, import: 'default' }) as Record<string, unknown>;
export type ThemeFileResolver = (pkg: string, file: string) => unknown;
const installedFiles = new Map<string, unknown>();
const fileKey = (pkg: string, file: string) => `${pkg}/${file.replace(/^\.?\//, '')}`;
let resolveFile: ThemeFileResolver = (pkg, file) => installedFiles.get(fileKey(pkg, file)) ?? treeFiles[`../../packages/${pkg}/${file.replace(/^\.?\//, '')}`];
/**
 * The theme files of an installed package, fetched and digest-checked by the
 * loader (apps/packages.ts) before its contributions apply; null removes them.
 * They win over an in-tree twin's.
 */
export function setInstalledThemeFiles(pkg: string, files: Record<string, unknown> | null): void {
  for (const key of [...installedFiles.keys()]) if (key.startsWith(`${pkg}/`)) installedFiles.delete(key);
  for (const [file, theme] of Object.entries(files ?? {})) installedFiles.set(fileKey(pkg, file), theme);
}
/** Tests and the installed-package loader supply where a package's theme files come from. */
export function setThemeFileResolver(next: ThemeFileResolver): () => void {
  const previous = resolveFile;
  resolveFile = next;
  return () => { resolveFile = previous; };
}

registerContributionPoint('themes', (pkg, entries, scope) => {
  for (const entry of entries as Array<{ id?: string; file?: string }>) {
    try {
      if (!entry || typeof entry.id !== 'string' || typeof entry.file !== 'string' || entry.file.includes('..')) throw new Error('a theme is { id, file }');
      const theme = resolveFile(pkg, entry.file) as ChoirTheme | undefined;
      if (!theme) throw new Error(`theme file ${entry.file} is not in the package`);
      if (theme.id !== entry.id) throw new Error(`theme file ${entry.file} has id "${theme.id}", not "${entry.id}"`);
      const remove = registerTheme(theme);
      const id = theme.id;
      scope.effect(() => {
        remove();
        // The selected theme leaves with its pack: the default shows, the choice is kept (T3).
        if (get(current).id === id) {
          wanted = get(current);
          host?.apply(wanted, false); // a pending choice: the shell shows the scheme default and keeps the choice
        }
      });
      if (wanted && (wanted as any).id === id) {
        const pending = wanted;
        wanted = null;
        host?.apply(pending, false);
      }
    } catch (err) {
      console.warn(`[contributes] ${pkg}: theme not applied: ${(err as Error).message}`);
    }
  }
});
