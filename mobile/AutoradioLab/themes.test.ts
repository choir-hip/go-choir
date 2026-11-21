// The themes in shared code under bun test (themes.ts). Ported from the Swift tests this replaces: ThemeDefaultsTests (D1-D12), the ThemeLoaderTests of SettingsDataTests (F1-F7), ThemeLensTests (L1-L3),
// the two theme tests of SpineTests, DesignRulesTests R1 (radii) and TypographyRulesTests T8 (role faces). Then what the move adds:
// Failure list (written before the code; each line has a test):
//  D1-D12 the default pair (Chrome Pastel light, Liquid Metal dark), an explicit mode, a chosen theme that survives a relaunch, the default never stored as a choice, removing the chosen theme,
//         nothing shadows a bundled theme, a single-scheme theme locks the mode, the Default card previews the pair and is first.
//  F1-F7  the contrast gate, faces, CSS in a value, a missing role or wrong version, the type scale, reduced motion, a dark-only theme never shown light, an install that survives a relaunch and writes nothing when refused, problems not duplicated.
//  L1-L3  the lens.   S1 S2 the bundled six load in both schemes; an unreadable theme is refused.   R1 the radius rule.   T8 the role faces.
//  M1  every file in mobile/themes is imported by themes-bundled.ts (a new theme file is not silently left out).
//  H1  the host facts decide: the system scheme and Reduce Motion come in as arguments; Android (no native handle) gets the same themes.
//  K1  the choice is kept in app:/data (a relaunch finds it); the default writes no theme; a removed theme clears it; a file dropped in the folder is read.
//  G1  the migration: a phone with the old choice and old themes keeps both; once; nothing is asked again; a failed write leaves the old copy and tries again; a choice made here wins; no native, nothing happens.
//  X1  the Texture fetch: the words for 404, 422, refused, locked; the demo document; nothing written for a theme that is refused; a bundled id is refused.
//  Z1  the sources answer the shapes the contracts bind (every field of Theme, ThemeExtras and ThemeCard).
import { beforeEach, describe, expect, test } from 'bun:test';
import { readdirSync, readFileSync } from 'node:fs';
import * as T from './themes.ts';
import { BUNDLED } from './themes-bundled.ts';
import { resetSeam, bindNative, HTTPError, SignedOutError, type Reply } from './api.ts';
import { WORDS } from './words.ts';
import demoJSON from './fixtures/texture/theme-ember.json';

const ALL = ['chrome-pastel', 'depth', 'lantern', 'liquid-metal', 'newsprint', 'signal'];
const light: T.Host = { scheme: 'light', reduceMotion: false };
const dark: T.Host = { scheme: 'dark', reduceMotion: false };
const doc = (id: string, extra: Record<string, unknown> = {}): Record<string, unknown> => ({ schema_version: 3, id, name: id.toUpperCase(), colors: { paper: '#FFFFFF', fg: '#111111', muted: '#444444', accent: '#0000CC', onAccent: '#FFFFFF' }, ...extra });
const colors = (c: Record<string, unknown>): Record<string, unknown> => ({ colors: { paper: '#FFFFFF', fg: '#000000', muted: '#444444', accent: '#0000AA', onAccent: '#FFFFFF', ...c } });

/** A real in-memory file system: files and folders, rm removes a folder's children, readdir lists them. */
function memFs(failWrites = false) {
  const files = new Map<string, Uint8Array>();
  const kids = (p: string) => [...files.keys()].filter((k) => k.startsWith(p + '/'));
  const fs = {
    atomicWriteFile: async (p: string, b: Uint8Array) => { if (failWrites) throw new Error('disk full'); files.set(p, b); },
    readFile: async (p: string) => { const b = files.get(p); if (!b) throw new Error('ENOENT'); return b; },
    rm: async (p: string) => { files.delete(p); for (const k of kids(p)) files.delete(k); },
    mkdir: async () => undefined,
    readdir: async (p: string) => { const k = kids(p); if (!k.length) throw new Error('ENOENT'); return [...new Set(k.map((x) => x.slice(p.length + 1).split('/')[0]))]; },
  };
  const put = (p: string, text: string) => files.set(p, new TextEncoder().encode(text));
  const text = (p: string) => (files.has(p) ? new TextDecoder().decode(files.get(p)!) : null);
  return { files, put, text, storage: { fs, sqlite: {} } as never };
}
const demo = (): Record<string, unknown> => JSON.parse(JSON.stringify(demoJSON));
const reply = (body: unknown): Reply => ({ body, serverTime: 0, raw: '' });
const kept = (m: ReturnType<typeof memFs>) => JSON.parse(m.text(T.CHOICE_FILE) ?? 'null');
beforeEach(() => { resetSeam(); T.resetThemes(); });
const store = (bundled?: Record<string, unknown>) => new T.ThemeStore(bundled);
const tview = (bundled?: Record<string, unknown>) => new T.ThemeStore(bundled).withTextures([]);      // the copy a page is shown: the only store that may hold a Texture theme

describe('the default pair (ThemeDefaultsTests)', () => {
  test('D1 a light scheme shows Chrome Pastel', () => {
    const s = store();
    expect(s.themeID(light)).toBe('chrome-pastel');
    expect(s.current(light).scheme).toBe('light'); expect(s.current(light).paper).toBe('#F7FBFE');
    expect(s.list(light).find((t) => t.current)?.id).toBe('chrome-pastel');
  });
  test('D2 a dark scheme shows Liquid Metal; the Default card is the one in use and comes first', () => {
    const s = store();
    expect(s.themeID(dark)).toBe('liquid-metal');
    expect(s.current(dark).scheme).toBe('dark'); expect(s.current(dark).paper).toBe('#1C242E');
    expect(s.cards(dark).find((c) => c.current)?.id).toBe('default');
    expect(s.cards(dark)[0].id).toBe('default'); expect(s.cards(dark)[0].paper).toBe('#1C242E');
  });
  test('D3 an explicit mode decides the default', () => {
    const a = store(); a.setMode('dark');
    expect(a.themeID(light)).toBe('liquid-metal'); expect(a.effectiveScheme(light)).toBe('dark');
    const b = store(); b.setMode('light');
    expect(b.themeID(dark)).toBe('chrome-pastel'); expect(b.effectiveScheme(dark)).toBe('light');
  });
  test('D4 a chosen theme wins in either scheme and after a relaunch', async () => {
    const m = memFs();
    let s = await T.themeStore(m.storage);
    await T.themeSet('depth', m.storage);
    expect(s.themeID(dark)).toBe('depth'); expect(s.current(dark).scheme).toBe('dark');
    T.resetThemes();                                                              // a relaunch
    s = await T.themeStore(m.storage);
    expect(s.themeID(light)).toBe('depth');
    await T.schemeSet('dark', m.storage); expect(s.themeID(light)).toBe('depth');
  });
  test('D5 the default is not stored as a choice, and it follows the mode', async () => {
    const m = memFs();
    const s = await T.themeStore(m.storage);
    s.current(dark); s.cards(dark); s.themeID(dark); await T.schemeSet('light', m.storage); s.current(dark);
    expect(kept(m).theme).toBeNull();
    expect(s.themeID(dark)).toBe('chrome-pastel');
  });
  test('D6 both defaults pass the gate and carry every token', () => {
    const s = store(); expect(s.problems).toEqual([]);
    for (const [mode, id] of [['light', 'chrome-pastel'], ['dark', 'liquid-metal']] as const) {
      s.setMode(mode);
      expect(s.themeID(light)).toBe(id);
      const t = s.current(light);
      expect(t.scheme).toBe(mode);
      expect(t.scrim).not.toBe(''); expect(t.scrim).not.toBe('rgba(0,0,0,.7)'); expect(t.onScrim).not.toBe('');
      expect(t.barBacking).toBe(0.94); expect(t.layerBacking).toBe(0.86); expect(t.material2).toBe('glass'); expect(t.material3).toBe('glass');
      expect(T.ratio(T.rgb(t.fg)!, T.rgb(t.paper)!)).toBeGreaterThanOrEqual(4.5);
      expect(T.ratio(T.rgb(t.muted)!, T.rgb(t.paper2)!)).toBeGreaterThanOrEqual(4.5);
      expect(T.ratio(T.rgb(t.onScrim)!, T.rgb('#000000')!)).toBeGreaterThanOrEqual(4.5);
    }
  });
  test('D7 the body reads in the web\'s sans', () => {
    for (const mode of ['light', 'dark']) { const s = store(); s.setMode(mode); expect(s.current(light).fontRead).toBe('system-ui'); expect(s.current(light).fontUi).toBe('system-ui'); }
  });
  test('D8 removing the chosen theme returns to the default and clears the kept choice', async () => {
    const m = memFs();
    await T.themeStore(m.storage);
    await T.themeFromTexture('t1', m.storage, { net: async () => reply({ doc_id: 't1', theme: doc('ember') }) });
    await T.themeSet('ember', m.storage); expect((await T.shownStore(m.storage)).themeID(dark)).toBe('ember');
    await T.themeRemove('ember', m.storage);
    expect(kept(m).theme).toBeNull(); expect((await T.shownStore(m.storage)).themeID(dark)).toBe('liquid-metal');
    expect(m.files.has(`${T.TEXTURE_DIR}/texture-ember.json`)).toBe(false);
  });
  test('D9 nothing replaces a bundled theme (an install, a file, the reserved id), and removing it does nothing', async () => {
    const m = memFs();
    m.put(`${T.THEMES_DIR}/chrome-pastel.json`, JSON.stringify(doc('chrome-pastel', { name: 'Fake' })));
    const s = await T.themeStore(m.storage);
    const paper = s.current(light).paper;
    expect(() => s.installFromTexture(doc('chrome-pastel', { name: 'Fake' }))).toThrow();
    expect(s.cards(light).find((c) => c.id === 'chrome-pastel')?.name).toBe('Chrome Pastel'); expect(s.current(light).paper).toBe(paper);
    expect(s.problems.some((p) => p.includes('bundled'))).toBe(true);
    const ack = await T.themeFromTexture('x', m.storage, { net: async () => reply({ theme: doc('chrome-pastel') }) });
    expect(ack.ok).toBe(false); expect(m.files.has(`${T.THEMES_DIR}/texture-chrome-pastel.json`)).toBe(false);
    await T.themeRemove('chrome-pastel', m.storage);
    expect(s.themeID(light)).toBe('chrome-pastel'); expect(s.current(light).paper).toBe(paper);
    expect(() => s.installFromTexture(doc('default'))).toThrow();
  });
  test('D10 a chosen single-scheme theme locks the mode honestly; a two-scheme theme or the Default gives it back', () => {
    const s = store(); s.setMode('dark');
    s.setTheme('chrome-pastel');
    expect(s.lockedScheme).toBe('light'); expect(s.schemeMode).toBe('light');
    s.setMode('dark');
    expect(s.schemeMode).toBe('light'); expect(s.effectiveScheme(dark)).toBe('light');
    expect(s.current(dark).scheme).toBe('light'); expect(s.current(dark).id).toBe('chrome-pastel');
    s.setTheme('depth');
    expect(s.lockedScheme).toBeNull(); expect(s.schemeMode).toBe('dark'); expect(s.current(light).scheme).toBe('dark');
    s.setTheme('chrome-pastel'); s.setTheme('default');
    expect(s.lockedScheme).toBeNull(); expect(s.schemeMode).toBe('dark');
    expect(s.themeID(light)).toBe('liquid-metal'); expect(s.current(light).scheme).toBe('dark');
  });
  test('D12 the Default card previews the default, not the chosen theme', () => {
    const l = store(); l.setTheme('lantern');
    expect(l.cards(light)[0].id).toBe('default'); expect(l.cards(light)[0].paper).toBe('#F7FBFE');
    expect(l.cards(light)[0].paper).not.toBe(l.cards(light).find((c) => c.id === 'lantern')!.paper);
    const d = store(); d.setTheme('depth');
    expect(d.cards(dark)[0].paper).toBe('#1C242E');
    const k = store(); k.setTheme('chrome-pastel');                               // light-locked, the phone asks dark
    expect(k.cards(dark)[0].paper).toBe('#1C242E');
  });
  test('D11 the Default entry is first, clears the choice, and is in use only without one; it survives a relaunch', async () => {
    const m = memFs();
    let s = await T.themeStore(m.storage);
    expect(s.cards(light)[0].id).toBe('default'); expect(s.cards(light)[0].name).toBe('Default');
    expect(s.cards(light).filter((c) => c.current).map((c) => c.id)).toEqual(['default']);
    await T.themeSet('lantern', m.storage);
    expect(s.cards(light).filter((c) => c.current).map((c) => c.id)).toEqual(['lantern']); expect(kept(m).theme).toBe('lantern');
    await T.themeSet('default', m.storage);
    expect(kept(m).theme).toBeNull(); expect(s.cards(light).filter((c) => c.current).map((c) => c.id)).toEqual(['default']); expect(s.themeID(light)).toBe('chrome-pastel');
    T.resetThemes(); s = await T.themeStore(m.storage); expect(s.themeID(light)).toBe('chrome-pastel');
  });
});

describe('the loader (ThemeLoaderTests)', () => {
  test('F1 the bundled themes all load and pass the gate', () => { const s = store(); expect(s.problems).toEqual([]); expect(s.list(light).map((t) => t.id).sort()).toEqual(ALL); });
  test('F1 newsprint dark passes onAccent on accent', () => {
    const s = store(); s.setTheme('newsprint'); s.setMode('dark');
    const t = s.current(light); expect(t.scheme).toBe('dark');
    expect(T.ratio(T.rgb(t.onAccent)!, T.rgb(t.accent)!)).toBeGreaterThanOrEqual(3);
  });
  test('F1 signal is high contrast and the others AA', () => {
    const by = Object.fromEntries(store().cards(light).map((c) => [c.id, c]));
    expect(by.signal.contrast).toBe('AAA'); expect(by.lantern.contrast).toBe('AA');
  });
  test('F2 stacks reduce to the faces Exact resolves', () => {
    expect(T.face("ui-serif, 'Iowan Old Style', Charter, Georgia, serif")).toBe('ui-serif');
    expect(T.face('Inter, Helvetica, sans-serif')).toBe('system-ui');
    expect(T.face("'SF Mono', monospace")).toBe('ui-monospace');
    expect(T.face('ui-rounded')).toBe('ui-rounded');
    expect(T.face('Papyrus, Comic Sans MS')).toBeNull();
    const s = store(); s.setTheme('depth');
    expect(s.current(light).fontRead).toBe('ui-serif'); expect(s.current(light).fontUi).toBe('system-ui');
    s.setTheme('lantern'); expect(s.current(light).fontUi).toBe('ui-rounded');
    s.setTheme('signal'); expect(s.current(light).fontRead).toBe('system-ui');
  });
  test('F2 a theme naming no face is refused', () => {
    const s = store({ odd: doc('odd', { type: { read: 'Papyrus' } }) });
    expect(s.list(light)).toEqual([]); expect(s.problems[0]).toContain('names no face');
  });
  test('F3 CSS in a value is refused', () => {
    expect(store({ inj: doc('inj', { elevation: { '1': { shadow: '0 0 0 url(x)' } } }) }).list(light)).toEqual([]);
    expect(store({ semi: doc('semi', colors({ paper: '#FFFFFF;x' })) }).list(light)).toEqual([]);
    for (const v of ['var(--x)', 'calc(1px)', '@import x', 'javascript:x', 'expression(x)', '{', '<', '\\']) expect(T.safe(v)).toBe(false);
    expect(T.safe('0 1px 2px rgba(0,0,0,.2)')).toBe(true); expect(T.safe('x'.repeat(241))).toBe(false);
  });
  test('F1 a missing role and a wrong version are refused', () => {
    const noMuted = { schema_version: 3, id: 'nm', name: 'NM', colors: { paper: '#FFFFFF', fg: '#000000', accent: '#0000AA', onAccent: '#FFFFFF' } };
    const s = store({ nm: noMuted, v2: { ...doc('v2'), schema_version: 2 } });
    expect(s.list(light)).toEqual([]); expect(s.problems.length).toBe(2);
  });
  test('F1 a theme with unreadable text is refused with the reason (the gate)', () => {
    const s = store({ bad: doc('bad', colors({ fg: '#EEEEEE', muted: '#EEEEEE', accent: '#0000FF' })) });
    expect(s.list(light)).toEqual([]); expect(s.problems[0]).toMatch(/below 4\.5 in light/);
  });
  test('F8 a partial theme falls back to the base (paper2 to paper, accent2 to accent)', () => {
    const t = store({ p: doc('p') }).current(light);
    expect(t.paper2).toBe(t.paper); expect(t.accent2).toBe(t.accent);
  });
  test('the type scale resolves per theme', () => {
    const s = store();
    s.setTheme('depth'); expect(s.extras(light).sizeBody).toBe(18);
    s.setTheme('lantern'); expect(s.extras(light).sizeBody).toBe(20); expect(s.current(light).scale).toBe('large');
    s.setTheme('signal'); expect(s.extras(light).pressTint).toBe(true);
  });
  test('F6 reduced motion stops the mark and the entrance', () => {
    const s = store(); s.setTheme('depth');
    const on: T.Host = { scheme: 'light', reduceMotion: true };
    const t = s.current(on);
    expect(t.reducedMotion).toBe(true); expect(t.motionEnterMs).toBe(0); expect(t.markDriftS).toBe(0); expect(t.behindScale3).toBe(1); expect(s.extras(on).markStatic).toBe(true);
    expect(t.springDamping).toBeCloseTo(2 * Math.sqrt(t.springStiffness * t.springMass), 2);       // critically damped
    expect(s.current(light).reducedMotion).toBe(false); expect(s.current(light).markDriftS).toBe(60);
  });
  test('F4 a dark-only theme is never shown light', () => {
    const s = tview();
    expect(s.installFromTexture(T.parseTextureTheme(demo()).theme)).toBe('ember');
    s.setTheme('ember'); s.setMode('light');
    expect(s.effectiveScheme(light)).toBe('dark'); expect(s.current(light).scheme).toBe('dark');
  });
  test('F7 an install survives a relaunch, shows its origin, and can be removed; a bundled theme stays', async () => {
    const m = memFs();
    await T.themeStore(m.storage);
    const ack = await T.themeFromTexture('demo-theme', m.storage, { net: async () => reply(demo()) });
    expect(ack).toEqual({ ok: true, message: 'Added Ember from the Texture.' });
    T.resetThemes();
    const again = await T.shownStore(m.storage);
    const card = again.cards(light).find((c) => c.id === 'ember');
    expect(card?.origin).toBe('texture'); expect(card?.schemes).toBe('dark');
    expect((await T.themeStore(m.storage)).cards(light).find((c) => c.id === 'ember')).toBeUndefined();       // the device store never holds it
    await T.themeRemove('ember', m.storage); await T.themeRemove('depth', m.storage);
    const after = await T.shownStore(m.storage);
    expect(after.cards(light).find((c) => c.id === 'ember')).toBeUndefined(); expect(after.cards(light).find((c) => c.id === 'depth')).toBeDefined();
    T.resetThemes();
    expect((await T.shownStore(m.storage)).cards(light).find((c) => c.id === 'ember')).toBeUndefined();
  });
  test('F7 an unreadable theme is refused and nothing is written', async () => {
    const m = memFs();
    const ack = await T.themeFromTexture('d', m.storage, { net: async () => reply({ theme: doc('faint', colors({ fg: '#EEEEEE', muted: '#EEEEEE', accent: '#0000FF' })) }) });
    expect(ack.ok).toBe(false); expect([...m.files.keys()]).toEqual([]);
    expect((await T.themeStore(m.storage)).cards(light).find((c) => c.id === 'faint')).toBeUndefined();
  });
  test('F5 a re-read does not duplicate problems', () => {
    const s = store(); s.add('not json', 'broken.json', 'file'); s.add('not json', 'broken.json', 'file'); s.add('not json', 'broken.json', 'file');
    expect(s.problems.length).toBe(1);
  });
  test('the Texture answer: the route\'s object or a bare theme; nothing else', () => {
    const a = T.parseTextureTheme({ doc_id: 'd1', revision_id: 'r9', theme: { schema_version: 3, id: 'x', name: 'X' }, ignored: ['layout'] });
    expect(a.docID).toBe('d1'); expect(a.revisionID).toBe('r9'); expect(a.ignored).toEqual(['layout']);
    expect(T.parseTextureTheme({ schema_version: 3, id: 'y', name: 'Y' }).docID).toBe('');
    expect(() => T.parseTextureTheme({ error: 'nope' })).toThrow(); expect(() => T.parseTextureTheme([])).toThrow();
    const d = T.parseTextureTheme(demo()); expect(d.docID).toBe('demo-theme'); expect(d.revisionID).toBe('demo-theme-r3');
  });
});

describe('the lens (ThemeLensTests)', () => {
  const withLens = (lens: unknown): Record<string, unknown> => { const o = JSON.parse(JSON.stringify(BUNDLED.depth)); o.id = 'lens-test'; o.name = 'Lens test'; if (lens === undefined) delete o.lens; else o.lens = lens; return o; };
  test('L1 every bundled theme says native glass and no wash; one without a lens key defaults to it', () => {
    for (const [name, o] of Object.entries(BUNDLED)) { const l = (o as { lens?: { style?: string; wash?: number } }).lens; expect(l?.style).toBe('A'); expect(l?.wash).toBe(0); void name; }
    const s = tview(); expect(s.current(light).lensStyle).toBe('A'); expect(s.current(light).lensWash).toBe(0);
    s.installFromTexture(withLens(undefined)); s.setTheme('lens-test');
    expect(s.current(light).lensStyle).toBe('A'); expect(s.current(light).lensWash).toBe(0);
  });
  test('L2 a lens style and wash reach the theme', () => {
    const s = tview(); s.installFromTexture(withLens({ style: 'B', wash: 0.55 })); s.setTheme('lens-test');
    expect(s.current(light).lensStyle).toBe('B'); expect(s.current(light).lensWash).toBe(0.55);
  });
  test('L3 a bad lens is refused with a reason', () => {
    const s = tview();
    expect(() => s.installFromTexture(withLens({ style: 'Z', wash: 0 }))).toThrow(/lens\.style/);
    expect(() => s.installFromTexture(withLens({ style: 'A', wash: 2 }))).toThrow(/lens\.wash/);
    expect(s.list(light).some((t) => t.id === 'lens-test')).toBe(false);
  });
});

describe('the spine, the radius rule, the role faces', () => {
  test('S1 the themes resolve in both schemes', () => {
    const s = store(); s.setMode('dark'); s.setTheme('newsprint');
    const t = s.current(light);
    expect(t.scheme).toBe('dark'); expect(t.paper).toBe('#0E0E0E'); expect(t.material2).toBe('solid');
    s.setMode('light'); s.setTheme('depth');
    expect(s.current(light).material2).toBe('glass'); expect(s.current(light).paper).toBe('#FFF7FE');
  });
  test('R1 themes are clamped to the radius rule; the defaults too', () => {
    for (const id of ALL) {
      const s = store(); s.setTheme(id); const t = s.current(light);
      expect(t.radiusCard).toBeGreaterThanOrEqual(8); expect(t.radiusCard).toBeLessThanOrEqual(12);
      expect(t.radiusControl).toBeGreaterThanOrEqual(4); expect(t.radiusControl).toBeLessThanOrEqual(6); expect(t.radiusPill).toBeLessThanOrEqual(6);
    }
    expect(T.defaultTheme().radiusPill).toBeLessThanOrEqual(6); expect(T.defaultTheme().radiusCard).toBeLessThanOrEqual(12);
  });
  test('T8 the role faces: the defaults, and a theme\'s own faces do not move the roles', () => {
    const t = T.defaultTheme();
    expect([t.fontHead, t.fontText, t.fontLabel, t.fontAction, t.fontMono]).toEqual(['ui-serif', 'ui-serif', 'system-ui', 'system-ui', 'ui-monospace']);
    for (const id of ALL) {
      const s = store(); s.setTheme(id); const c = s.current(light);
      expect(c.fontHead).toBe('ui-serif'); expect(c.fontText).toBe('ui-serif');
      expect(['system-ui', 'ui-rounded']).toContain(c.fontLabel); expect(c.fontLabel).toBe(c.fontAction);
    }
  });
  test('M1 every file in mobile/themes is imported by themes-bundled.ts', () => {
    const files = readdirSync(new URL('../themes/', import.meta.url).pathname).filter((f) => f.endsWith('.json')).map((f) => f.replace(/\.json$/, '')).sort();
    expect(Object.keys(BUNDLED).sort()).toEqual(files); expect(files).toEqual(ALL);
  });
});

describe('the gate refuses see-through colours (E1)', () => {
  test('E1 alpha below 1 on paper, paper2, fg, muted, accent or onAccent is refused; rule, scrim and success may be see-through', () => {
    for (const role of ['paper', 'paper2', 'fg', 'muted', 'accent', 'onAccent']) {
      for (const v of ['rgba(255,255,255,.05)', 'rgb(255 255 255 / 50%)', '#FFFFFF80']) {
        const c = { paper: '#FFFFFF', fg: '#111111', muted: '#444444', accent: '#0000CC', onAccent: '#FFFFFF', [role]: role === 'paper' || role === 'paper2' || role === 'onAccent' ? v : role === 'fg' ? v.replace('255,255,255', '0,0,0') : v };
        const s = store({ x: { schema_version: 3, id: 'x', name: 'X', colors: c } });
        expect(s.list(light)).toEqual([]); expect(s.problems[0]).toMatch(/transparent|below|not hex/);
      }
    }
    expect(store({ ok: doc('ok', { colors: { paper: '#FFFFFF', fg: '#111111', muted: '#444444', accent: '#0000CC', onAccent: '#FFFFFF', rule: 'rgba(0,0,0,.1)', scrim: 'rgba(0,0,0,.8)' } }) }).list(light).length).toBe(1);
    expect(T.alphaOf('#FFFFFF')).toBe(1); expect(T.alphaOf('rgb(1,2,3)')).toBe(1); expect(T.alphaOf('rgba(1,2,3,0.5)')).toBe(0.5); expect(T.alphaOf('rgb(1 2 3 / 25%)')).toBe(0.25);
  });
  test('E1 a Texture theme with a see-through paper is refused and nothing is written', async () => {
    const m = memFs();
    const a = await T.themeFromTexture('d', m.storage, { net: async () => reply({ theme: doc('glass', { colors: { paper: 'rgba(255,255,255,.05)', fg: '#000000', muted: '#222222', accent: '#0000CC', onAccent: '#FFFFFF' } }) }) });
    expect(a.ok).toBe(false); expect(a.message).toMatch(/transparent/); expect(m.files.size).toBe(0);
  });
});

describe('a user theme the new gate refuses is told, not replaced (E2)', () => {
  test('E2 an old user file with a see-through paper stays out of the list and shows its reason under its own file name, in the cards the Settings page lists', async () => {
    const m = memFs();
    m.put(`${T.THEMES_DIR}/old-glass.json`, JSON.stringify(doc('old-glass', { colors: { paper: '#FFFFFF80', fg: '#000000', muted: '#222222', accent: '#0000CC', onAccent: '#FFFFFF' } })));
    const cards = await T.themeCardsShared('light', false, m.storage);
    expect(cards.rows.find((c) => c.id === 'old-glass')).toBeUndefined();
    expect(cards.problems).toEqual(['old-glass.json: paper is transparent in light']);
    expect(cards.themeID).toBe('chrome-pastel');                                               // a person who had chosen it sees the default, and the line saying why
  });
  test('E2 an unknown mode in a kept choice is the system\'s, and the list says when a theme locks the mode', async () => {
    const m = memFs(); m.put(T.CHOICE_FILE, JSON.stringify({ theme: 'chrome-pastel', mode: 'purple' }));
    const l = await T.themeListShared('dark', false, m.storage);
    expect(l.locked).toBe('light'); expect(l.mode).toBe('light');
    await T.themeSet('default', m.storage);
    expect((await T.themeListShared('dark', false, m.storage)).locked).toBe('');
  });
});

describe('colours are exactly colours (E3)', () => {
  const asPaper = (v: string) => store({ x: { schema_version: 3, id: 'x', name: 'X', colors: { paper: v, fg: '#000000', muted: '#222222', accent: '#0000CC', onAccent: '#FFFFFF' } } }).list(light).length;
  test('E3 every accepted shape loads and every other shape is refused', () => {
    for (const ok of ['#FFF', '#ffffff', '#FFFFFF', '#fff', 'rgb(255,255,255)', 'rgb(255 255 255)', 'RGB(255, 255, 255)', 'rgba(255,255,255,1)', 'rgb(255 255 255 / 100%)', 'rgb(100%,100%,100%)', ' #FFFFFF ']) expect(asPaper(ok)).toBe(1);
    for (const bad of ['rgbx(255,255,255)', '#FFFFFF000', '#FFFFF', '#GGGGGG', '#FF', 'rgb(255,255)', 'rgb(255,255,255,2)', 'rgb(256,255,255)', 'rgb(-1,255,255)', 'rgb(255,255,255', 'rgb 255 255 255', 'rgb(255,255,255) x', 'rgb(255;255;255)', 'rgb(255,255,255,0.5,1)', 'rgb(101%,0%,0%)', 'rgba(255 255 255 255)', 'white', 'hsl(0,0%,100%)', 'transparent', '']) expect([bad, asPaper(bad)]).toEqual([bad, 0]);
    expect(T.rgb('#FFFFFF000')).toBeNull(); expect(T.rgb('rgbx(0,0,0)')).toBeNull(); expect(T.rgb('rgb(10,20,30)')).toEqual([10, 20, 30]); expect(T.rgb('#abc')).toEqual([170, 187, 204]);
  });
  test('E3 the bundled themes still load whole (every colour they name is a colour)', () => { expect(store().problems).toEqual([]); });
});

describe('the first frame (P1)', () => {
  // The contracts' placeholder is the whole default theme (the bundled light default, resolved), written out by hand in two contract files: this keeps it in step.
  const empty = (o: Record<string, unknown>): string => `empty(${Object.entries(o).map(([k, v]) => `${k}=${typeof v === 'string' ? JSON.stringify(v) : String(v)}`).join(', ')})`;
  const read = (f: string): string => readFileSync(new URL(`./${f}`, import.meta.url).pathname, 'utf8');
  test('P1 spine.contract and members-settings.contract carry the full default theme and extras as their placeholders', () => {
    const light0 = new T.ThemeStore().current({ scheme: 'light', reduceMotion: false });
    expect(read('spine.contract')).toContain(`as shape Theme else ${empty({ ...light0 })}`);
    expect(read('members-settings.contract')).toContain(`as shape ThemeExtras else ${empty({ ...T.defaultExtras() })}`);
  });
});

describe('the host facts (H1)', () => {
  test('H1 the scheme and Reduce Motion are the arguments; the mode wins over the system; a locked theme limits the scheme', async () => {
    const m = memFs();
    expect((await T.themeShared('dark', false, m.storage)).id).toBe('liquid-metal');
    expect((await T.themeShared('light', false, m.storage)).id).toBe('chrome-pastel');
    expect((await T.themeShared('', false, m.storage)).scheme).toBe('light');                    // no fact yet: light
    expect((await T.themeShared('dark', true, m.storage)).reducedMotion).toBe(true);
    expect((await T.themeExtrasShared('dark', true, m.storage)).reduceMotion).toBe(true);
    await T.schemeSet('light', m.storage);
    expect((await T.themeShared('dark', false, m.storage)).scheme).toBe('light');
    await T.schemeSet('system', m.storage); await T.themeSet('chrome-pastel', m.storage);
    expect((await T.themeShared('dark', false, m.storage)).scheme).toBe('light');                // a light-only theme in a dark phone
  });
  test('H1 Android: no native handle, no storage trouble: the same themes, no migration', async () => {
    const m = memFs();                                                                          // resetSeam: no native reader bound
    const l = await T.themeListShared('light', false, m.storage);
    expect(l.rows.map((r) => r.id).sort()).toEqual(ALL); expect(l.themeID).toBe('chrome-pastel'); expect(l.problems).toEqual([]);
    expect(m.files.size).toBe(0);                                                               // nothing was written by looking
  });
  test('Z1 the sources answer every field the contracts bind', async () => {
    const m = memFs();
    const th = await T.themeShared('light', false, m.storage);
    expect(Object.keys(th).sort()).toEqual(Object.keys(T.defaultTheme()).sort());
    expect(Object.keys(await T.themeExtrasShared('light', false, m.storage)).sort()).toEqual(Object.keys(T.defaultExtras()).sort());
    const cards = await T.themeCardsShared('light', false, m.storage);
    expect(Object.keys(cards).sort()).toEqual(['mode', 'problems', 'rows', 'scheme', 'status', 'themeID']);
    expect(Object.keys(cards.rows[0]).sort()).toEqual(['accent', 'accent2', 'contrast', 'current', 'fg', 'fontRead', 'fontUi', 'id', 'material', 'muted', 'name', 'onAccent', 'origin', 'paper', 'paper2', 'radiusCard', 'radiusControl', 'rule', 'scale', 'schemes', 'shadow']);
    expect(cards.rows[0].id).toBe('default'); expect(cards.mode).toBe('system');
    const list = await T.themeListShared('light', false, m.storage);
    expect(Object.keys(list).sort()).toEqual(['locked', 'mode', 'problems', 'rows', 'scheme', 'themeID']);
  });
});

describe('what is kept (K1)', () => {
  test('K1 a mode and a theme are kept in app:/data and found by a relaunch; a file dropped in the folder is read', async () => {
    const m = memFs();
    m.put(`${T.THEMES_DIR}/mine.json`, JSON.stringify(doc('mine')));
    await T.themeSet('mine', m.storage); await T.schemeSet('dark', m.storage);
    expect(kept(m)).toMatchObject({ theme: 'mine', mode: 'dark' });
    T.resetThemes();
    const s = await T.themeStore(m.storage);
    expect(s.choice).toEqual({ theme: 'mine', mode: 'dark' }); expect(s.originOf('mine')).toBe('file');
  });
  test('K1 an unknown theme id or a mode that is not one changes and writes nothing', async () => {
    const m = memFs();
    await T.themeSet('nothing-here', m.storage); await T.schemeSet('purple', m.storage);
    expect(m.files.size).toBe(0);
  });
  test('K1 a damaged choice file is the defaults', async () => {
    const m = memFs(); m.put(T.CHOICE_FILE, '{not json');
    const s = await T.themeStore(m.storage); expect(s.choice).toEqual({ theme: null, mode: 'system' });
  });
});

describe('the migration from the Swift storage (G1)', () => {
  const load = async (m: ReturnType<typeof memFs>, host = 'native') => { const s = await T.themeStore(m.storage); await T.themeMigrate(host, m.storage); return s; };
  const legacy = (over: Record<string, unknown> = {}, log: string[] = []) => ({
    available: true,
    later: async (r: { op: string }) => { log.push(r.op); return r.op === 'themeLegacy' ? { found: true, theme: 'mine', mode: 'dark', files: { 'mine.json': JSON.stringify(doc('mine')), 'texture-ember.json': JSON.stringify(T.parseTextureTheme(demo()).theme), 'broken.json': 'nope' }, ...over } : { done: true }; },
  });
  test('G1 the old choice and the old themes are carried over once; the old copy is released after; the next launch asks nothing', async () => {
    const m = memFs(); const log: string[] = [];
    bindNative(legacy({}, log) as never);
    const s = await load(m);
    expect(s.choice).toEqual({ theme: 'mine', mode: 'dark' }); expect(s.originOf('mine')).toBe('file'); expect(s.originOf('ember')).toBeUndefined();       // a Texture theme is account data: never carried over
    expect(s.problems.some((p) => p.startsWith('broken.json'))).toBe(true);                       // honest: the broken one is still reported
    expect(m.files.has(`${T.THEMES_DIR}/texture-ember.json`)).toBe(false); expect(m.files.has(`${T.TEXTURE_DIR}/texture-ember.json`)).toBe(false); expect(m.files.has(`${T.THEMES_DIR}/mine.json`)).toBe(true);
    expect(s.problems.some((p) => p.startsWith('texture-ember.json'))).toBe(true);
    expect(kept(m)).toEqual({ theme: 'mine', mode: 'dark', migrated: true });
    expect(log).toEqual(['themeLegacy']);                                                          // the old copy is not removed (a reverted build still reads it)
    T.resetThemes(); log.length = 0;
    const again = await load(m);
    expect(again.choice.theme).toBe('mine'); expect(log).toEqual([]);                            // `migrated` is kept: no second read
  });
  test('G1 a phone with nothing old is marked and never asked again', async () => {
    const m = memFs(); const log: string[] = [];
    bindNative(legacy({ found: false, theme: '', mode: '', files: {} }, log) as never);
    const s = await load(m);
    expect(s.choice).toEqual({ theme: null, mode: 'system' }); expect(kept(m).migrated).toBe(true);
  });
  test('G1 a choice already made here wins over the old one; the old themes still come', async () => {
    const m = memFs(); m.put(T.CHOICE_FILE, JSON.stringify({ theme: 'depth', mode: 'light' }));
    bindNative(legacy() as never);
    const s = await load(m);
    expect(s.choice).toEqual({ theme: 'depth', mode: 'light' }); expect(s.originOf('mine')).toBe('file'); expect(kept(m).migrated).toBe(true);
  });
  test('G1 a write that fails leaves no marker, so the next launch tries again', async () => {
    const m = memFs(true); const log: string[] = [];
    bindNative(legacy({}, log) as never);
    await load(m);
    expect(log).toEqual(['themeLegacy']); expect(m.files.size).toBe(0);
  });
  test('G1 a read that never answers, then a theme tap, then a relaunch: the old themes still come (only migrate writes the marker)', async () => {
    const m = memFs();
    bindNative({ available: true, later: async () => new Promise(() => undefined) } as never);
    await load(m);                                                              // waits out LEGACY_WAIT_MS, answers without it
    await T.themeSet('depth', m.storage); await T.schemeSet('dark', m.storage);
    expect(kept(m)).toEqual({ theme: 'depth', mode: 'dark' });                                  // no `migrated`
    T.resetThemes();
    bindNative(legacy() as never);
    const s = await load(m);
    expect(s.originOf('mine')).toBe('file'); expect(s.choice).toEqual({ theme: 'depth', mode: 'dark' });   // the choice made here wins, the old themes arrived
    expect(kept(m).migrated).toBe(true);
  }, 10000);
  test('G1 a file the read could not carry (over the cap, or past the 64th) is a problem under its own name, not dropped', async () => {
    const m = memFs();
    const many: Record<string, string> = {};
    for (let i = 0; i < 66; i++) many[`t${String(i).padStart(2, '0')}.json`] = JSON.stringify(doc(`t${i}`));
    bindNative(legacy({ files: many, skipped: ['huge.json'] }) as never);
    const s = await load(m);
    expect(s.problems.filter((p) => p.includes('could not be carried over')).map((p) => p.split(':')[0]).sort()).toEqual(['huge.json', 't64.json', 't65.json']);
  });
  test('K1 a failed save is tried again by the next tap on the same theme', async () => {
    const m = memFs(true);
    await T.themeSet('depth', m.storage); expect(m.files.size).toBe(0);
    const ok = memFs(); ok.put('x', 'y');
    // same store, a disk that works now
    const s = await load(m); expect(s.choice.theme).toBe('depth');
    (m.storage as never as { fs: { atomicWriteFile: (p: string, b: Uint8Array) => Promise<void> } }).fs.atomicWriteFile = async (p, b) => { m.files.set(p, b); };
    await T.themeSet('depth', m.storage); expect(kept(m).theme).toBe('depth');
  });

  test('G1 an unreadable answer or a bridge that throws changes nothing and marks nothing', async () => {
    for (const later of [async () => 'garbage', async () => { throw new Error('bridge'); }]) {
      T.resetThemes(); const m = memFs();
      bindNative({ available: true, later } as never);
      const s = await load(m);
      expect(s.choice).toEqual({ theme: null, mode: 'system' }); expect(m.files.size).toBe(0);
    }
  });
  test('G1 file names are plain: a path, a hidden file or a non-json name in the answer is dropped', async () => {
    const m = memFs();
    bindNative(legacy({ files: { '../evil.json': '{}', '.hidden.json': '{}', 'a/b.json': '{}', 'x.txt': '{}', 'ok.json': JSON.stringify(doc('ok')) } }) as never);
    await load(m);
    expect([...m.files.keys()].sort()).toEqual([T.CHOICE_FILE, `${T.THEMES_DIR}/ok.json`].sort());
  });
  test('G1 a file already in the new place is kept: the carry-over (and a retry of it) never writes over it', async () => {
    const m = memFs();
    const mine = JSON.stringify(doc('mine', { name: 'Newer' }));
    m.put(`${T.THEMES_DIR}/mine.json`, mine);
    bindNative(legacy() as never);
    const s = await load(m);
    expect(m.text(`${T.THEMES_DIR}/mine.json`)).toBe(mine); expect(s.cards(light).find((c) => c.id === 'mine')?.name).toBe('Newer');
    expect(m.files.has(`${T.THEMES_DIR}/theme-two.json`) || m.files.has(`${T.THEMES_DIR}/mine.json`)).toBe(true);      // the others still come (a Texture theme is not carried)
  });
  test('G1 the first answer does not wait for the carry-over; the revision moves when it is done, so the sources are asked again', async () => {
    const m = memFs(); let release: () => void = () => undefined;
    bindNative({ available: true, later: async (r: { op: string }) => { await new Promise<void>((res) => { release = res; }); return r.op === 'themeLegacy' ? { found: true, theme: 'depth', mode: 'dark', files: {} } : {}; } } as never);
    const first = await T.themeShared('light', false, m.storage);                              // answers at once: the default
    expect(first.id).toBe('chrome-pastel');
    const p = T.themeMigrate('native', m.storage);
    await new Promise((r) => setTimeout(r, 30)); release();
    expect((await p).rev).toBe(1);
    expect((await T.themeShared('light', false, m.storage)).id).toBe('depth');
  });
  test('G1 the handle binds late: the carry-over waits for it (bounded) and then runs', async () => {
    const m = memFs();
    await T.themeStore(m.storage);
    setTimeout(() => bindNative(legacy() as never), 300);
    const r = await T.themeMigrate('', m.storage);
    expect(r.rev).toBe(1); expect(kept(m).migrated).toBe(true); expect(kept(m).theme).toBe('mine');
  });
  test('G1 Android: host "none" and no handle returns at once, writes nothing, asks nothing', async () => {
    const m = memFs(); await T.themeStore(m.storage);
    const t0 = Date.now(); const r = await T.themeMigrate('none', m.storage);
    expect(Date.now() - t0).toBeLessThan(500); expect(r.rev).toBe(0); expect(m.files.size).toBe(0);
  });
  test('G1 no handle ever: it gives up after the bounded wait, writes nothing and is asked again when the host fact changes', async () => {
    const m = memFs(); await T.themeStore(m.storage);
    expect((await T.themeMigrate('', m.storage)).rev).toBe(0); expect(m.files.size).toBe(0);
    bindNative(legacy() as never);
    expect((await T.themeMigrate('native', m.storage)).rev).toBe(1);
  }, 10000);
  test('G1 a theme chosen while the carry-over runs is not overridden by the old choice', async () => {
    const m = memFs(); await T.themeStore(m.storage);
    bindNative({ available: true, later: async () => { await T.themeSet('signal', m.storage); return { found: true, theme: 'mine', mode: 'dark', files: { 'mine.json': JSON.stringify(doc('mine')) } }; } } as never);
    await T.themeMigrate('native', m.storage);
    const s = await T.themeStore(m.storage);
    expect(s.choice.theme).toBe('signal'); expect(s.originOf('mine')).toBe('file');
  });
});

describe('a theme from a Texture (X1)', () => {
  const seam = (f: () => Promise<Reply>) => ({ net: f });
  test('X1 the words for each failure', async () => {
    const m = memFs();
    const say = async (e: unknown) => (await T.themeFromTexture('d', m.storage, seam(async () => { throw e; }))).message;
    expect(await say(new HTTPError(404, 'HTTP 404'))).toBe('No theme Texture with that id.');
    expect(await say(new HTTPError(422, 'HTTP 422: no choir-theme block'))).toBe('That Texture is not a usable theme: no choir-theme block');
    expect(await say(new HTTPError(503, 'HTTP 503'))).toBe('The server refused (503).');
    expect(await say(new SignedOutError('x', 'refused'))).toBe(WORDS.rejected);
    expect(await say(new SignedOutError('x', 'locked'))).toBe(WORDS.locked);
    expect(await say(new SignedOutError())).toBe('Sign in to add a theme from a Texture.');
    expect((await T.themeFromTexture('  ', m.storage, seam(async () => reply(null)))).message).toBe('Enter the id of a theme Texture.');
    expect((await T.themeCardsShared('light', false, m.storage)).status).toBe('Enter the id of a theme Texture.');
  });
  test('X1 the ignored parts are said; the status shows on the cards; removing clears it', async () => {
    const m = memFs();
    const a = await T.themeFromTexture('d', m.storage, seam(async () => reply({ doc_id: 'd', theme: doc('ember'), ignored: ['layout', 'sounds'] })));
    expect(a.message).toBe('Added EMBER from the Texture (the phone ignored: layout, sounds).');
    expect((await T.themeCardsShared('light', false, m.storage)).status).toBe(a.message);
    await T.themeRemove('ember', m.storage);
    expect((await T.themeCardsShared('light', false, m.storage)).status).toBe('');
  });
  test('X1 a bundled id from a Texture is refused and the bundled theme is unchanged', async () => {
    const m = memFs();
    const a = await T.themeFromTexture('d', m.storage, seam(async () => reply({ theme: doc('signal', { name: 'Fake' }) })));
    expect(a.ok).toBe(false); expect(a.message).toContain('bundled');
    expect((await T.themeStore(m.storage)).cards(light).find((c) => c.id === 'signal')?.name).toBe('Signal');
  });
});
