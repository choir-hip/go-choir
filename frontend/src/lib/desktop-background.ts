// The desktop background (owner, 2026-11-09: "themes must be separate from the
// desktop background"). The background is an image the owner picks from Files
// or uploads; a theme never sets it.
//
// Stored per computer through the package-settings path, next to the theme
// choice (preferences pkg.settings.background / pkg.settings.background_fit,
// declared in packages/settings/choir-package.json). The value is a Files path.
// Seam for later: a package asset ref (e.g. "pkg:<name>/<asset>") would resolve
// here to a URL; nothing builds packages as backgrounds yet, so such a value is
// ignored (cleanBackgroundPath returns '').
//
// Rules:
//  - '' means "Theme colour": the desktop shows the active theme's flat bg, as
//    before this setting existed;
//  - an image is painted over the theme's bg, which stays the fallback and the
//    letterbox colour. While an image is set, the theme's desktop-only
//    wallpaper (the decorative layers painted on .app-root and .desktop in
//    app.css, and --choir-desk-texture) is covered by the image layer, so a
//    theme never shows through or alters the owner's background. The theme's
//    surface tokens (bg, panel...) for windows are untouched;
//  - bytes come from the owner-authenticated Files read route, never a data URI;
//  - mail/ is Mail-agent custody: never listed, selected or painted.
import { getSetting, setSetting, type SettingResult } from './services/settings';
import { pickFile } from './file-picker';

export const BACKGROUND_SETTING_PKG = 'settings';
export const BACKGROUND_KEY = 'background';
export const BACKGROUND_FIT_KEY = 'background_fit';
export const BACKGROUND_FITS = ['cover', 'contain', 'center', 'tile'] as const;
export type BackgroundFit = (typeof BACKGROUND_FITS)[number];
export const BACKGROUND_EXTENSIONS = ['png', 'jpg', 'jpeg', 'webp', 'gif'];

/** The Files path if it is an allowed background image, else ''. */
export function cleanBackgroundPath(raw: unknown): string {
  if (typeof raw !== 'string') return '';
  const path = raw.trim().replace(/^\/+/, '');
  if (!path || path.length > 1000) return '';
  const parts = path.split('/');
  if (parts.some((p) => !p || p === '.' || p === '..' || p.includes('\\'))) return '';
  if (parts[0].toLowerCase() === 'mail') return '';
  const ext = parts[parts.length - 1].split('.').pop()?.toLowerCase() ?? '';
  return BACKGROUND_EXTENSIONS.includes(ext) ? path : '';
}

export function cleanBackgroundFit(raw: unknown): BackgroundFit {
  return (BACKGROUND_FITS as readonly string[]).includes(raw as string) ? (raw as BackgroundFit) : 'cover';
}

/** The CSS for the image layer, or '' when no image shows. */
export function backgroundLayerStyle(rawPath: unknown, rawFit: unknown): string {
  const path = cleanBackgroundPath(rawPath);
  if (!path) return '';
  const url = '/api/files/' + path.split('/').map(encodeURIComponent).join('/') + '?disposition=inline';
  const fit = cleanBackgroundFit(rawFit);
  const size = fit === 'cover' ? 'cover' : fit === 'contain' ? 'contain' : 'auto';
  const repeat = fit === 'tile' ? 'repeat' : 'no-repeat';
  const position = fit === 'tile' ? '0 0' : 'center';
  return `background-image:url("${url}");background-size:${size};background-repeat:${repeat};background-position:${position}`;
}

/**
 * Make a Files image the desktop background: the one write the Settings
 * control, the picker and Files' "Set as desktop wallpaper" all use. A path
 * the rules refuse (not an image, under mail/) is not saved.
 */
export async function setDesktopBackground(path: string): Promise<SettingResult> {
  const clean = cleanBackgroundPath(path);
  if (!clean) return { ok: false, error: 'Choose a png, jpg, webp or gif image outside mail/' };
  return setSetting(BACKGROUND_KEY, clean, BACKGROUND_SETTING_PKG);
}

/**
 * Open the Files picker on images and, if the owner chooses one, make it the
 * background. Resolves to the path, or null when cancelled (nothing changes).
 */
export async function chooseDesktopBackground(): Promise<{ path: string | null; result?: SettingResult }> {
  const current = cleanBackgroundPath(await getSetting(BACKGROUND_KEY, BACKGROUND_SETTING_PKG));
  const path = await pickFile({
    title: 'Choose a desktop background',
    confirmLabel: 'Use as background',
    extensions: BACKGROUND_EXTENSIONS,
    filterLabel: 'Images',
    hideAtRoot: ['mail'],
    startPath: current,
  });
  if (!path) return { path: null };
  return { path, result: await setDesktopBackground(path) };
}
