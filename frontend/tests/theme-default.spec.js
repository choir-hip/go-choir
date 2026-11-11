// The default theme follows the system colour scheme for an owner who has not
// chosen one: light is Chrome Pastel, dark is Liquid Metal. Pure theme logic,
// loaded through vite like theme-packs.spec.js; no browser.
//
// Failure list, written before the code:
//  D1 the dark scheme still shows the light default (a white desktop at night);
//  D2 Chrome Pastel differs from the published release's values (a look-alike);
//  D3 the default is read where matchMedia is absent (Node, a worker) and
//     throws or answers dark by accident: it must answer light, always;
//  D4 a saved choice is overridden by the default (aero-chrome-heart in dark);
//  D5 a theme that cannot be applied (unknown id) falls to the light default
//     in dark mode instead of the scheme's default;
//  D6 the favicon colours stay the old default's when the scheme is dark;
//  D7 either default fails the contrast gate the shell enforces on selection;
//  D8 Aero Chrome Heart stops being selectable once it is no longer default.
import { test, expect } from '@playwright/test';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { createServer } from 'vite';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
let vite; let T; let M;
test.beforeAll(async () => {
  vite = await createServer({ root, server: { middlewareMode: true, hmr: false, watch: null }, appType: 'custom', logLevel: 'silent' });
  T = await vite.ssrLoadModule('/src/lib/theme.ts');
  M = await vite.ssrLoadModule('/src/lib/tetramark.ts');
});
test.afterAll(async () => { await vite?.close(); });

const scheme = (dark) => ({ matchMedia: (q) => ({ matches: dark && q === '(prefers-color-scheme: dark)' }) });
const published = JSON.parse(fs.readFileSync(path.join(root, 'tests/fixtures/chrome-pastel.theme.json'), 'utf8'));

test('D1: light shows Chrome Pastel, dark shows Liquid Metal', () => {
  expect(T.systemDefaultTheme(scheme(false)).id).toBe('chrome-pastel.hybrid');
  expect(T.systemDefaultTheme(scheme(false)).name).toBe('Chrome Pastel');
  expect(T.systemDefaultTheme(scheme(true)).id).toBe('liquid-metal');
  expect(T.defaultThemeForScheme('dark')).toBe(T.LIQUID_METAL);
  expect(T.defaultThemeForScheme('light')).toBe(T.CHROME_PASTEL);
});

test('D2: Chrome Pastel carries the published release values verbatim', () => {
  for (const group of ['colors', 'radii', 'motion', 'fonts', 'effects', 'layout']) {
    expect(T.CHROME_PASTEL[group], group).toEqual(published[group]);
  }
  expect(T.CHROME_PASTEL.schema_version).toBe(published.schema_version);
});

test('D3: where the scheme cannot be read the default is light, deterministically', () => {
  expect(T.systemDefaultTheme({})).toBe(T.CHROME_PASTEL);
  expect(T.systemDefaultTheme({ matchMedia: () => { throw new Error('no'); } })).toBe(T.CHROME_PASTEL);
  expect(T.DEFAULT_THEME).toBe(T.CHROME_PASTEL);
});

test('D4: a saved choice wins in either scheme', () => {
  const saved = JSON.parse(JSON.stringify(T.AERO_CHROME_HEART));
  const was = globalThis.matchMedia;
  try {
    for (const dark of [false, true]) {
      globalThis.matchMedia = scheme(dark).matchMedia;
      expect(T.normalizeThemeConfig(saved).id, `dark=${dark}`).toBe('aero-chrome-heart');
      expect(T.normalizeThemeConfig(T.LIQUID_METAL).id, `dark=${dark}`).toBe('liquid-metal');
      expect(T.normalizeThemeConfig(T.CHROME_PASTEL).id, `dark=${dark}`).toBe('chrome-pastel.hybrid');
    }
  } finally { globalThis.matchMedia = was; }
});

test('D5: an unusable theme falls to the scheme default', () => {
  const was = globalThis.matchMedia;
  try {
    globalThis.matchMedia = scheme(true).matchMedia;
    expect(T.normalizeThemeConfig({ schema_version: 2, id: 'no-such-theme' }).id).toBe('liquid-metal');
    expect(T.normalizeThemeConfig(null).id).toBe('liquid-metal');
    globalThis.matchMedia = scheme(false).matchMedia;
    expect(T.normalizeThemeConfig({ schema_version: 2, id: 'no-such-theme' }).id).toBe('chrome-pastel.hybrid');
  } finally { globalThis.matchMedia = was; }
});

test('D6: the favicon takes the scheme default colours', () => {
  const was = globalThis.matchMedia;
  try {
    globalThis.matchMedia = scheme(true).matchMedia;
    const dark = M.tetraMarkFaviconSvg();
    expect(dark).toContain(`fill="${T.LIQUID_METAL.colors.bg}"`);
    expect(dark).toContain(`fill="${T.LIQUID_METAL.colors.tetramarkColor}"`);
    globalThis.matchMedia = scheme(false).matchMedia;
    const light = M.tetraMarkFaviconSvg();
    expect(light).toContain(`fill="${T.CHROME_PASTEL.colors.bg}"`);
    expect(light).toContain(`fill="${T.CHROME_PASTEL.colors.tetramarkColor}"`);
  } finally { globalThis.matchMedia = was; }
});

test('D7: both defaults pass the contrast gate and the contributed-theme gate', () => {
  for (const theme of [T.CHROME_PASTEL, T.LIQUID_METAL]) {
    expect(T.contrastErrors(theme.colors), theme.id).toEqual([]);
    expect(T.contributedThemeErrors(theme), theme.id).toEqual([]);
    expect(T.validateThemeConfig(theme).ok, theme.id).toBe(true);
  }
});

test('D8: Aero Chrome Heart and the other presets stay selectable', () => {
  const ids = T.listThemes().map((t) => t.id);
  for (const id of ['aero-chrome-heart', 'liquid-metal', 'chrome-pastel.hybrid', 'frutiger-aero', 'vector-heart']) expect(ids).toContain(id);
  expect(new Set(ids).size).toBe(ids.length);
});

test('D9: an installed copy of the pack is refused by id, so Settings lists Chrome Pastel once', () => {
  const copy = JSON.parse(JSON.stringify(published));
  expect(() => T.registerTheme(copy)).toThrow(/already taken/);
  expect(T.listThemes().filter((t) => t.name === 'Chrome Pastel')).toHaveLength(1);
});
