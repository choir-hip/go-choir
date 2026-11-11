// Theme service and contributes.themes, slice S5. A contributed theme is
// selected, validated and normalized exactly like a preset; the leak
// differential holds across install and uninstall.
// Failure modes T1 to T5 are listed in services/theme.ts.
import { test, expect } from '@playwright/test';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { createServer } from 'vite';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
let vite; let T; let S; let X; let K;
test.beforeAll(async () => {
  vite = await createServer({ root, server: { middlewareMode: true, hmr: false, watch: null }, appType: 'custom', logLevel: 'silent' });
  T = await vite.ssrLoadModule('/src/lib/theme.ts');
  S = await vite.ssrLoadModule('/src/lib/services/theme.ts');
  X = await vite.ssrLoadModule('/src/lib/contributions.ts');
  K = await vite.ssrLoadModule('/src/lib/shell-kernel.ts');
});
test.afterAll(async () => { await vite?.close(); });

const clone = (o) => JSON.parse(JSON.stringify(o));
const night = () => ({ ...clone(T.DEFAULT_THEME), id: 'paper-night.dark', name: 'Paper Night' });

test('every built-in preset passes the contributed-theme gate (the gate is not stricter than the shell)', () => {
  for (const preset of T.THEME_PRESETS) expect(T.contributedThemeErrors(preset), preset.id).toEqual([]);
});

test('T1: a missing token, an unsafe value, an extra key and low contrast are each refused', () => {
  const missing = night(); delete missing.colors.accent;
  expect(T.contributedThemeErrors(missing)).toContain('colors.accent is missing');
  const unsafe = night(); unsafe.effects.texture = 'url(https://example.com/x.png)';
  expect(T.contributedThemeErrors(unsafe)).toContain('effects.texture must be a safe CSS value');
  const extra = night(); extra.colors.sneaky = '#fff';
  expect(T.contributedThemeErrors(extra)).toContain('colors.sneaky is not supported');
  const grey = night(); grey.colors.fg = grey.colors.bg;
  expect(T.contributedThemeErrors(grey).join(' ')).toMatch(/colors\.fg on colors\.bg has contrast 1\.00, below 4\.5/);
  const odd = night(); odd.colors.bg = 'hsl(0 0% 0%)';
  expect(T.contributedThemeErrors(odd).join(' ')).toMatch(/hex or rgb/);
  expect(T.contrastRatio('#000000', '#ffffff')).toBeCloseTo(21, 0);
});

test('T2: a built-in id or a taken id is refused', () => {
  expect(() => T.registerTheme({ ...night(), id: 'futuristic-noir' })).toThrow(/already taken/);
  const off = T.registerTheme(night());
  expect(() => T.registerTheme(night())).toThrow(/already taken/);
  off();
});

test('a pack contributes a theme; selecting it normalizes to it; uninstall restores the baseline (T3, leak differential)', async () => {
  const before = JSON.stringify(K.shellKernel.snapshot());
  const idsBefore = T.listThemes().map((t) => t.id);
  const restore = S.setThemeFileResolver((pkg, file) => (pkg === 'paper-night' && file === 'theme.json' ? night() : undefined));
  const applied = [];
  const unbind = S.bindThemeHost({ apply: (theme, persist) => { const shown = theme ? T.normalizeThemeConfig(theme) : T.systemDefaultTheme(); applied.push([theme ? theme.id : null, persist]); S.publishTheme(shown); return { ok: true }; } });
  await X.setPackageContributions('paper-night', { themes: [{ id: 'paper-night.dark', file: 'theme.json' }] }, 'v1');
  expect(T.listThemes().map((t) => t.id)).toEqual([...idsBefore, 'paper-night.dark']);
  expect(T.normalizeThemeConfig(night()).id).toBe('paper-night.dark');
  expect(T.validateThemeConfig(night()).ok).toBe(true);
  expect(S.setTheme('paper-night.dark')).toEqual({ ok: true, errors: [] });
  expect(S.getTheme().id).toBe('paper-night.dark');
  const seen = [];
  const off = S.onThemeChange((t) => seen.push(t.id));
  // the pack goes while selected: its choice is applied as pending (the system default shows, the choice is kept), not persisted
  await X.setPackageContributions('paper-night', null, '');
  expect(applied.at(-1)).toEqual(['paper-night.dark', false]);
  expect(seen).toEqual([T.systemDefaultTheme().id]);
  expect(T.listThemes().map((t) => t.id)).toEqual(idsBefore);
  expect(S.setTheme('paper-night.dark').ok).toBe(false);
  // the choice returns with the pack
  await X.setPackageContributions('paper-night', { themes: [{ id: 'paper-night.dark', file: 'theme.json' }] }, 'v2');
  expect(applied.at(-1)).toEqual(['paper-night.dark', false]);
  await X.setPackageContributions('paper-night', null, '');
  off(); unbind(); restore();
  expect(JSON.stringify(K.shellKernel.snapshot())).toBe(before);
});

test('a stored contributed id that was not selectable at boot is applied when the pack registers (load order)', async () => {
  const restore = S.setThemeFileResolver(() => night());
  const applied = [];
  const unbind = S.bindThemeHost({ apply: (theme, persist) => { applied.push([theme.id, persist]); return { ok: true }; } });
  S.rememberWantedTheme(night());
  await X.setPackageContributions('paper-night', { themes: [{ id: 'paper-night.dark', file: 'theme.json' }] }, 'v1');
  expect(applied).toEqual([['paper-night.dark', false]]);
  await X.setPackageContributions('paper-night', null, '');
  unbind(); restore();
});

test('T4, T5: a missing file is skipped with the rest applied; unknown ids and invalid configs are not applied', async () => {
  const restore = S.setThemeFileResolver((pkg, file) => (file === 'ok.json' ? { ...night(), id: 'px.ok' } : undefined));
  const warn = console.warn; console.warn = () => {};
  await X.setPackageContributions('px', { themes: [{ id: 'px.gone', file: 'gone.json' }, { id: 'px.ok', file: 'ok.json' }] }, 'v1');
  console.warn = warn;
  expect(T.listThemes().map((t) => t.id)).toContain('px.ok');
  expect(T.listThemes().map((t) => t.id)).not.toContain('px.gone');
  expect(S.setTheme('nope').ok).toBe(false);
  expect(S.setTheme({ schema_version: 2, id: 'futuristic-noir', colors: { fg: 'url(x)' } }).ok).toBe(false);
  await X.setPackageContributions('px', null, '');
  restore();
});

// Review r1 B4: a safe value never fetches. image-set(), image(), cross-fade(),
// src() and url() take URLs as bare strings; the one list is
// scripts/css-fetch-functions.json (the package-CSS gate reads it too).
const FETCHING = [
  'image-set("https://example.com/p.png" 1x)',
  '-webkit-image-set("https://example.com/p.png" 1x)',
  'IMAGE-SET("https://example.com/p.png" 1x)',
  'image("https://example.com/p.png")',
  'cross-fade(url-less, "https://example.com/p.png")',
  'src("https://example.com/p.png")',
  'url(https://example.com/p.png)',
  'url ( "https://example.com/p.png" )',
  'imag\\65-set("https://example.com/p.png" 1x)',
];
test('B4: no fetching function passes the safe-value gate, in a contributed theme or a config', () => {
  for (const value of FETCHING) {
    const t = night(); t.colors.selected = value;
    expect(T.contributedThemeErrors(t), value).toContain('colors.selected must be a safe CSS value');
    const v = T.validateThemeConfig({ ...clone(T.DEFAULT_THEME), colors: { selected: value } });
    expect(v.ok, value).toBe(false);
    expect(T.normalizeThemeConfig({ ...clone(T.DEFAULT_THEME), colors: { selected: value } }).colors.selected).toBe(T.DEFAULT_THEME.colors.selected);
  }
  const fine = night(); fine.colors.selected = 'linear-gradient(#123456, #654321)';
  expect(T.contributedThemeErrors(fine)).toEqual([]);
});

test('N8: setTheme runs the contrast gate; a config that makes the shell unreadable is refused and nothing applies', () => {
  const applied = [];
  const unbind = S.bindThemeHost({ apply: (theme) => { applied.push(theme.id); return { ok: true }; } });
  const grey = clone(T.DEFAULT_THEME); grey.colors.fg = grey.colors.bg;
  const r = S.setTheme(grey);
  expect(r.ok).toBe(false);
  expect(r.errors.join(' ')).toMatch(/contrast 1\.00/);
  expect(applied).toEqual([]);
  expect(S.setTheme(clone(T.DEFAULT_THEME)).ok).toBe(true);
  unbind();
});
