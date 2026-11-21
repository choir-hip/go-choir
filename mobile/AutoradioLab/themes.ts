// Themes as data (schema version 3), in shared code. It was Swift (AutoradioKit ThemeLoader.swift: ThemeStore, answered by the native sources theme, themeList, themeExtras and themeCards, with the
// choice in UserDefaults and the user's themes in Application Support); Android had no themes at all. A theme is JSON: colours (one value, or {light, dark}), type stacks and a named scale, radii,
// elevation (shadow, material, behind), motion (with reduced-motion values), the lens and the mark. This module reads the bundled themes (themes-bundled.ts), the user's themes under
// app:/data/themes (a file dropped in, or a theme installed from a Texture, saved as texture-<id>.json), validates each, gates contrast, and resolves one theme for one scheme into `Theme`
// (flat strings and numbers the contracts bind) and `ThemeExtras` (the type scale and the rest of the mark and motion tokens). No component names a colour, a font or a radius.
//
// The two facts only the system knows, the colour scheme and Reduce Motion, are Exact's own: `exactViewport().prefersColorScheme` and `.prefersReducedMotion` (the contracts pass them in). Nothing
// here is native except ONE read of the old Swift storage, once, for a phone that already has a choice and themes (the migration below).
//
// Ways it can fail (each has a test in themes.test.ts):
//  F1 A theme whose text is unreadable on its paper loads (the gate: fg and muted on paper and paper2 at 4.5, onAccent on accent at 3, accent on paper at 3, on every scheme the theme claims; a user
//     theme with a bad pair is refused with the reason, the others still load).
//  F2 A theme names a face the phone does not have and the text falls back silently: stacks are reduced to the first entry Exact resolves (ui-serif, system-ui, ui-rounded, ui-monospace; serif,
//     sans-serif, monospace map to those); a stack with none is refused.
//  F3 A value carries CSS that is not a value (url(, ;, braces, var(, calc(, @import, javascript:, expression): refused, so a forked Texture cannot paint outside its roles.
//  F4 A scheme the theme does not define is shown (a dark-only theme in a light phone): the effective scheme is limited to the sides the theme defines.
//  F5 A re-load of the user directory duplicates problems or the theme list: problems are kept per source.
//  F6 Reduced motion leaves springs and entrances running: with it on, enter is the theme's reduced value, the spring is critically damped and quick, and the mark's drift is zero when its reduced value is "static".
//  F7 A theme installed from a Texture replaces a bundled theme by accident, or an invalid one is written to disk: the install validates before it writes, the file name is texture-<id>.json, the origin is shown on the card.
//  F8 A new colour role is missing in a partial theme: roles fall back to the base (paper2 to paper, accent2 to accent).
//  F9 A Texture or a file installs a theme under a bundled id (or the reserved id "default") and replaces the bundled one: refused.
//  F10 A chosen theme that has only one scheme meets a mode control that asks for the other: the control shows the one scheme and refuses the other; the "default" id clears the choice.
//  F11 The lens is a style (A native glass, B no material with a wash, C ultra-thin) and a wash of the page colour: a theme without a "lens" key shows the native glass; an unknown style or a wash outside 0...1 is refused.
import type { Storage } from './app.contract.d.ts';
import { authedJSON, HTTPError, SignedOutError, nativeLater, currentWorld, unbound, type Reply, type World } from './api.ts';
import { WORDS } from './words.ts';
import { BUNDLED } from './themes-bundled.ts';

export type Theme = {
  id: string; name: string; scheme: string; schemes: string;
  paper: string; paper2: string; fg: string; muted: string; accent: string; accent2: string; success: string; danger: string; onAccent: string; rule: string; scrim: string; onScrim: string;
  fontRead: string; fontUi: string; fontMono: string; fontHead: string; fontText: string; fontLabel: string; fontAction: string; scale: string;
  radiusCard: number; radiusControl: number; radiusPill: number; radiusRules: number;
  shadow1: string; shadow2: string; shadow3: string; edge1: string; edge2: string; edge3: string; material1: string; material2: string; material3: string;
  press1: number; behindScale3: number; behindDim3: number; barBacking: number; layerBacking: number; lensStyle: string; lensWash: number;
  motionFastMs: number; motionEnterMs: number; motionSheetMs: number; staggerMs: number; springStiffness: number; springDamping: number; springMass: number;
  reducedMotion: boolean; markDriftS: number; markListenScale: number; markTumble: string;
};
export type ThemeExtras = {
  sizeNameplate: number; sizeHeadline: number; sizeH2: number; sizeBody: number; sizeLabel: number; sizeMeta: number; lineBody: number; lineHeadline: number; weightHeadline: number; weightUi: number;
  flipStiffness: number; flipDamping: number; markTumbleAS: number; markTumbleBS: number; markStatic: boolean; pressTint: boolean; reduceMotion: boolean; contrast: string;
};
export type ThemeInfo = { id: string; name: string; current: boolean };
export type ThemeCard = {
  id: string; name: string; origin: string; current: boolean; schemes: string; contrast: string; scale: string;
  paper: string; paper2: string; fg: string; muted: string; accent: string; accent2: string; onAccent: string; rule: string; fontRead: string; fontUi: string; radiusCard: number; radiusControl: number; shadow: string; material: string;
};
export type ThemeList = { rows: ThemeInfo[]; mode: string; themeID: string; scheme: string; problems: string[]; locked: string };
export type ThemeCards = { rows: ThemeCard[]; mode: string; scheme: string; themeID: string; problems: string[]; status: string };
export type Ack = { ok: boolean; message: string };
/** The two host facts, from `exactViewport()`: the system's colour scheme ("light" or "dark"; anything else reads as light) and Reduce Motion. */
export type Host = { scheme: string; reduceMotion: boolean };

export const DEFAULT_LIGHT_ID = 'chrome-pastel';
export const DEFAULT_DARK_ID = 'liquid-metal';
/** The id Settings sends to go back to the default pair (it clears the choice). No theme may take it. */
export const DEFAULT_CHOICE = 'default';

export const defaultTheme = (): Theme => ({
  id: '', name: '', scheme: 'light', schemes: 'light,dark',
  paper: '#FFFFFF', paper2: '#FFFFFF', fg: '#000000', muted: '#555555', accent: '#0066CC', accent2: '#004499', success: '#0C6644', danger: '#A3142F', onAccent: '#FFFFFF', rule: '#999999', scrim: 'rgba(0,0,0,.7)', onScrim: '#FFFFFF',
  fontRead: 'ui-serif', fontUi: 'system-ui', fontMono: 'ui-monospace', fontHead: 'ui-serif', fontText: 'ui-serif', fontLabel: 'system-ui', fontAction: 'system-ui', scale: 'editorial',
  radiusCard: 12, radiusControl: 6, radiusPill: 6, radiusRules: 0,
  shadow1: 'none', shadow2: 'none', shadow3: 'none', edge1: 'none', edge2: 'none', edge3: 'none', material1: 'solid', material2: 'solid', material3: 'solid',
  press1: 1, behindScale3: 1, behindDim3: 0, barBacking: 0.94, layerBacking: 0.86, lensStyle: 'A', lensWash: 0,
  motionFastMs: 120, motionEnterMs: 320, motionSheetMs: 260, staggerMs: 40, springStiffness: 240, springDamping: 26, springMass: 1,
  reducedMotion: false, markDriftS: 60, markListenScale: 1.15, markTumble: '3.3s,5.9s',
});
export const defaultExtras = (): ThemeExtras => ({
  sizeNameplate: 36, sizeHeadline: 28, sizeH2: 19, sizeBody: 18, sizeLabel: 15, sizeMeta: 13, lineBody: 1.6, lineHeadline: 1.12, weightHeadline: 700, weightUi: 600,
  flipStiffness: 300, flipDamping: 22, markTumbleAS: 3.3, markTumbleBS: 5.9, markStatic: false, pressTint: false, reduceMotion: false, contrast: 'AA',
});

export class ThemeError extends Error { constructor(message: string) { super(message); this.name = 'ThemeError'; } }

// ---- The document (what the Swift Decodable accepted: a present key of the wrong type is an error, null is absent, other keys are ignored)

type Obj = Record<string, unknown>;
type SchemeColor = { light: string; dark: string };
type Level = { shadow?: string; edge?: string; material?: string; press?: number; pressTint?: boolean; backing?: number; behind?: { scale?: number; dim?: number } };
export type ThemeDoc = {
  schema_version: number; id: string; name: string; schemes?: string[]; colors: Record<string, SchemeColor>;
  type?: { read?: string; ui?: string; mono?: string; scale?: string; sizes?: Record<string, number> };
  radii?: Record<string, string>; elevation?: Record<string, Level>;
  motion?: { fast?: string; enter?: string; sheet?: string; stagger?: string; spring?: { stiffness?: number; damping?: number; mass?: number }; reduced?: { enter?: string; spring?: string } };
  mark?: { drift?: string; tumble?: string[]; listenScale?: number; flip?: { stiffness?: number; damping?: number }; reduced?: string };
  lens?: { style?: string; wash?: number };
};

const isObj = (v: unknown): v is Obj => typeof v === 'object' && v !== null && !Array.isArray(v);
const bad = (path: string, want: string): never => { throw new ThemeError(`${path} is not ${want}`); };
const absent = (v: unknown): boolean => v === undefined || v === null;
const str = (o: Obj, k: string, path = ''): string | undefined => { const v = o[k]; if (absent(v)) return undefined; return typeof v === 'string' ? v : bad(path + k, 'text'); };
const num = (o: Obj, k: string, path = ''): number | undefined => { const v = o[k]; if (absent(v)) return undefined; return typeof v === 'number' && Number.isFinite(v) ? v : bad(path + k, 'a number'); };
const bool = (o: Obj, k: string, path = ''): boolean | undefined => { const v = o[k]; if (absent(v)) return undefined; return typeof v === 'boolean' ? v : bad(path + k, 'true or false'); };
const sub = (o: Obj, k: string, path = ''): Obj | undefined => { const v = o[k]; if (absent(v)) return undefined; return isObj(v) ? v : bad(path + k, 'an object'); };
const strList = (o: Obj, k: string, path = ''): string[] | undefined => {
  const v = o[k]; if (absent(v)) return undefined;
  return Array.isArray(v) && v.every((x) => typeof x === 'string') ? (v as string[]) : bad(path + k, 'a list of text');
};
const map = <T>(o: Obj, k: string, one: (v: unknown, path: string) => T, path = ''): Record<string, T> | undefined => {
  const m = sub(o, k, path); if (!m) return undefined;
  const out: Record<string, T> = {};
  for (const [key, v] of Object.entries(m)) out[key] = one(v, `${path}${k}.${key}`);
  return out;
};

function schemeColor(v: unknown, path: string): SchemeColor {
  if (typeof v === 'string') return { light: v, dark: v };
  if (!isObj(v)) return bad(path, 'a colour');
  for (const x of Object.values(v)) if (typeof x !== 'string') return bad(path, 'a colour');
  const l = (v.light ?? v.dark) as string | undefined, d = (v.dark ?? v.light) as string | undefined;
  if (l === undefined || d === undefined) throw new ThemeError('colour needs light or dark');
  return { light: l, dark: d };
}
function level(v: unknown, path: string): Level {
  if (!isObj(v)) return bad(path, 'an object');
  const b = sub(v, 'behind', path + '.');
  return { shadow: str(v, 'shadow', path + '.'), edge: str(v, 'edge', path + '.'), material: str(v, 'material', path + '.'), press: num(v, 'press', path + '.'), pressTint: bool(v, 'pressTint', path + '.'),
    backing: num(v, 'backing', path + '.'), behind: b ? { scale: num(b, 'scale'), dim: num(b, 'dim') } : undefined };
}
const text = (v: unknown, path: string): string => (typeof v === 'string' ? v : bad(path, 'text'));
const number = (v: unknown, path: string): number => (typeof v === 'number' && Number.isFinite(v) ? v : bad(path, 'a number'));

/** The document from parsed JSON. Throws ThemeError (the reason) for a shape it cannot read. */
export function decode(raw: unknown): ThemeDoc {
  if (!isObj(raw)) throw new ThemeError('a theme is a JSON object');
  const sv = raw.schema_version;
  if (typeof sv !== 'number' || !Number.isInteger(sv)) throw new ThemeError('schema_version is missing');
  const id = str(raw, 'id'), name = str(raw, 'name');
  if (id === undefined) throw new ThemeError('id is missing');
  if (name === undefined) throw new ThemeError('name is missing');
  const colors = map(raw, 'colors', schemeColor);
  if (!colors) throw new ThemeError('colors is missing');
  const doc: ThemeDoc = { schema_version: sv, id, name, schemes: strList(raw, 'schemes'), colors };
  const ty = sub(raw, 'type'); if (ty) doc.type = { read: str(ty, 'read', 'type.'), ui: str(ty, 'ui', 'type.'), mono: str(ty, 'mono', 'type.'), scale: str(ty, 'scale', 'type.'), sizes: map(ty, 'sizes', number, 'type.') };
  doc.radii = map(raw, 'radii', text);
  doc.elevation = map(raw, 'elevation', level);
  const mo = sub(raw, 'motion');
  if (mo) {
    const sp = sub(mo, 'spring', 'motion.'), re = sub(mo, 'reduced', 'motion.');
    doc.motion = { fast: str(mo, 'fast', 'motion.'), enter: str(mo, 'enter', 'motion.'), sheet: str(mo, 'sheet', 'motion.'), stagger: str(mo, 'stagger', 'motion.'),
      spring: sp ? { stiffness: num(sp, 'stiffness'), damping: num(sp, 'damping'), mass: num(sp, 'mass') } : undefined, reduced: re ? { enter: str(re, 'enter'), spring: str(re, 'spring') } : undefined };
  }
  const mk = sub(raw, 'mark');
  if (mk) { const fl = sub(mk, 'flip', 'mark.'); doc.mark = { drift: str(mk, 'drift', 'mark.'), tumble: strList(mk, 'tumble', 'mark.'), listenScale: num(mk, 'listenScale', 'mark.'), flip: fl ? { stiffness: num(fl, 'stiffness'), damping: num(fl, 'damping') } : undefined, reduced: str(mk, 'reduced', 'mark.') }; }
  const ln = sub(raw, 'lens'); if (ln) doc.lens = { style: str(ln, 'style', 'lens.'), wash: num(ln, 'wash', 'lens.') };
  return doc;
}

// ---- Validation

const FACES = new Set(['ui-serif', 'system-ui', 'ui-rounded', 'ui-monospace']);
const GENERIC: Record<string, string> = { serif: 'ui-serif', 'sans-serif': 'system-ui', monospace: 'ui-monospace', 'ui-sans-serif': 'system-ui' };
const TYPE_SCALES: Record<string, { nameplate: number; headline: number; h2: number; body: number; label: number; meta: number; lineBody: number }> = {
  editorial: { nameplate: 36, headline: 28, h2: 19, body: 18, label: 15, meta: 13, lineBody: 1.6 },
  large: { nameplate: 38, headline: 30, h2: 21, body: 20, label: 16, meta: 14, lineBody: 1.62 },
  compact: { nameplate: 32, headline: 25, h2: 17, body: 16, label: 14, meta: 12, lineBody: 1.5 },
};
export const typeScaleNames = (): string[] => Object.keys(TYPE_SCALES).sort();
const UNSAFE = ['url(', 'expression', '@import', 'javascript:', 'var(', 'calc(', ';', '{', '}', '<', '>', '\\'];
export const safe = (s: string): boolean => { const l = s.toLowerCase(); return !UNSAFE.some((u) => l.includes(u)) && [...s].length <= 240; };
const NUMBER = /^[+-]?(\d+\.?\d*|\.\d+)([eE][+-]?\d+)?$/;
const double = (s: string): number | null => (NUMBER.test(s) ? Number(s) : null);

/** The first face in a stack that Exact resolves; generics map to ui-serif / system-ui / ui-monospace. null: none. */
export function face(stack: string): string | null {
  for (const part of stack.split(',')) {
    const n = part.replace(/^[ '"]+|[ '"]+$/g, '').toLowerCase();
    if (FACES.has(n)) return n;
    if (GENERIC[n] !== undefined) return GENERIC[n];
  }
  return null;
}
/** A CSS duration in milliseconds ("120ms ease", "0.3s", "none"), or null. */
export function ms(s: string | undefined): number | null {
  if (s === undefined) return null;
  const first = s.split(' ').filter((p) => p !== '')[0] ?? s;
  if (first === 'none') return 0;
  if (first.endsWith('ms')) return double(first.slice(0, -2));
  if (first.endsWith('s')) { const v = double(first.slice(0, -1)); return v === null ? null : v * 1000; }
  return null;
}
// A colour is exactly one of: #RGB, #RGBA, #RRGGBB, #RRGGBBAA, or rgb()/rgba() with three numbers 0-255 (or percentages 0-100%) and an optional alpha 0-1 (or 0-100%), written with commas or
// with spaces and a "/". Anything else (a longer hex, an unknown function name, a missing or extra part, a number out of range) is not a colour and the theme is refused: the browser drops an
// invalid colour, and a theme must not make text or the sign-in banner vanish that way.
const NUM = '[+-]?(?:\\d+\\.?\\d*|\\.\\d+)';
const CHAN = `(${NUM}%?)`;
const RGB_COMMA = new RegExp(`^rgba?\\(\\s*${CHAN}\\s*,\\s*${CHAN}\\s*,\\s*${CHAN}\\s*(?:,\\s*(${NUM}%?)\\s*)?\\)$`, 'i');
const RGB_SPACE = new RegExp(`^rgba?\\(\\s*${CHAN}\\s+${CHAN}\\s+${CHAN}\\s*(?:/\\s*(${NUM}%?)\\s*)?\\)$`, 'i');
function parseColor(input: string): { rgb: [number, number, number]; a: number } | null {
  const v = input.trim();
  const hex = /^#([0-9a-fA-F]{3,4}|[0-9a-fA-F]{6}|[0-9a-fA-F]{8})$/.exec(v);
  if (hex) {
    let h = hex[1];
    if (h.length <= 4) h = [...h].map((c) => c + c).join('');
    const n = parseInt(h.slice(0, 6), 16);
    return { rgb: [(n >> 16) & 255, (n >> 8) & 255, n & 255], a: h.length === 8 ? parseInt(h.slice(6), 16) / 255 : 1 };
  }
  const m = RGB_COMMA.exec(v) ?? RGB_SPACE.exec(v);
  if (!m) return null;
  const chan = (t: string): number | null => { const pct = t.endsWith('%'); const n = double(pct ? t.slice(0, -1) : t); return n === null ? null : pct ? (n < 0 || n > 100 ? null : n * 2.55) : n < 0 || n > 255 ? null : n; };
  const c = [chan(m[1]), chan(m[2]), chan(m[3])];
  if (c.some((x) => x === null)) return null;
  let a = 1;
  if (m[4] !== undefined) { const pct = m[4].endsWith('%'); const n = double(pct ? m[4].slice(0, -1) : m[4]); if (n === null) return null; a = pct ? n / 100 : n; if (a < 0 || a > 1) return null; }
  return { rgb: [c[0] as number, c[1] as number, c[2] as number], a };
}
export function rgb(s: string): [number, number, number] | null { return parseColor(s)?.rgb ?? null; }
/** The opacity a colour asks for (1 when it names none); 1 too for a string that is not a colour (rgb() refuses it before the gate looks at alpha). */
export function alphaOf(s: string): number { return parseColor(s)?.a ?? 1; }
const lum = (c: [number, number, number]): number => {
  const f = (x: number): number => { const s = x / 255; return s <= 0.03928 ? s / 12.92 : Math.pow((s + 0.055) / 1.055, 2.4); };
  return 0.2126 * f(c[0]) + 0.7152 * f(c[1]) + 0.0722 * f(c[2]);
};
export const ratio = (a: [number, number, number], b: [number, number, number]): number => { const x = lum(a), y = lum(b); return (Math.max(x, y) + 0.05) / (Math.min(x, y) + 0.05); };
const sidesOf = (d: ThemeDoc): string[] => d.schemes ?? ['light', 'dark'];

export function contrastProblem(t: Theme): string | null {
  const paper = rgb(t.paper), paper2 = rgb(t.paper2), fg = rgb(t.fg), muted = rgb(t.muted), onA = rgb(t.onAccent), acc = rgb(t.accent);
  if (!paper || !paper2 || !fg || !muted || !onA || !acc) return 'a colour is not hex or rgb()';
  // The ratios are computed on the opaque colour; a see-through paper or text would be unreadable over whatever lies behind it, so these six must be opaque.
  for (const [name, v] of [['paper', t.paper], ['paper2', t.paper2], ['fg', t.fg], ['muted', t.muted], ['accent', t.accent], ['onAccent', t.onAccent]] as const) if (alphaOf(v) < 1) return `${name} is transparent`;
  if (ratio(fg, paper) < 4.5) return 'fg on paper below 4.5';
  if (ratio(muted, paper) < 4.5) return 'muted on paper below 4.5';
  if (ratio(fg, paper2) < 4.5) return 'fg on paper2 below 4.5';
  if (ratio(muted, paper2) < 4.5) return 'muted on paper2 below 4.5';
  if (ratio(onA, acc) < 3) return 'onAccent on accent below 3';
  if (ratio(acc, paper) < 3) return 'accent on paper below 3';
  return null;
}
/** "AAA" when every pair clears the enhanced level (7 for text, 4.5 for accent pairs) on every scheme, else "AA". */
export function contrastLevel(d: ThemeDoc): string {
  for (const scheme of sidesOf(d)) {
    const t = resolve(d, scheme, false);
    const paper = rgb(t.paper), paper2 = rgb(t.paper2), fg = rgb(t.fg), muted = rgb(t.muted), onA = rgb(t.onAccent), acc = rgb(t.accent);
    if (!paper || !paper2 || !fg || !muted || !onA || !acc) return 'AA';
    if (Math.min(ratio(fg, paper), ratio(muted, paper), ratio(fg, paper2), ratio(muted, paper2)) < 7) return 'AA';
    if (Math.min(ratio(onA, acc), ratio(acc, paper)) < 4.5) return 'AA';
  }
  return 'AAA';
}

export function validate(d: ThemeDoc): void {
  if (d.schema_version !== 3) throw new ThemeError(`schema_version ${d.schema_version} is not 3`);
  if (!/^[a-z0-9][a-z0-9-]{0,39}$/.test(d.id)) throw new ThemeError('id must be lower case letters, digits and dashes');
  if (d.id === DEFAULT_CHOICE) throw new ThemeError('the id default is reserved for the default pair');
  const n = [...d.name].length;
  if (n === 0 || n > 40) throw new ThemeError('name must be 1 to 40 characters');
  const sides = sidesOf(d);
  if (sides.length === 0 || !sides.every((s) => s === 'light' || s === 'dark')) throw new ThemeError('schemes must be light and/or dark');
  for (const [k, c] of Object.entries(d.colors)) for (const s of [c.light, c.dark]) if (!safe(s) || rgb(s) === null) throw new ThemeError(`colour ${k} is not hex or rgb()`);
  for (const role of ['paper', 'fg', 'muted', 'accent', 'onAccent']) if (d.colors[role] === undefined) throw new ThemeError(`colour ${role} is missing`);
  if (d.type) {
    for (const [k, v] of [['read', d.type.read], ['ui', d.type.ui], ['mono', d.type.mono]] as const) {
      if (v === undefined) continue;
      if (!safe(v) || face(v) === null) throw new ThemeError(`type.${k} names no face the phone has (ui-serif, system-ui, ui-rounded, ui-monospace)`);
    }
    if (d.type.scale !== undefined && TYPE_SCALES[d.type.scale] === undefined) throw new ThemeError(`type.scale ${d.type.scale} is not one of ${typeScaleNames().join(', ')}`);
  }
  for (const [k, v] of Object.entries(d.radii ?? {})) if (!(safe(v) && double(v.replaceAll('px', '')) !== null)) throw new ThemeError(`radii.${k} is not a px length`);
  for (const [k, l] of Object.entries(d.elevation ?? {})) {
    for (const s of [l.shadow, l.edge]) if (s !== undefined && !safe(s)) throw new ThemeError(`elevation.${k} has a value that is not a shadow`);
    if (l.material !== undefined && l.material !== 'glass' && l.material !== 'solid') throw new ThemeError(`elevation.${k}.material is glass or solid`);
  }
  if (d.motion) {
    for (const s of [d.motion.fast, d.motion.enter, d.motion.sheet, d.motion.stagger]) if (s !== undefined && (!safe(s) || ms(s) === null)) throw new ThemeError(`motion value ${[...s].slice(0, 20).join('')} is not a duration`);
    const sp = d.motion.spring;
    if (sp) {
      if (sp.stiffness !== undefined && !(sp.stiffness >= 1 && sp.stiffness <= 3000)) throw new ThemeError('spring stiffness out of range');
      if (sp.damping !== undefined && !(sp.damping >= 1 && sp.damping <= 300)) throw new ThemeError('spring damping out of range');
      if (sp.mass !== undefined && !(sp.mass >= 0.1 && sp.mass <= 10)) throw new ThemeError('spring mass out of range');
    }
  }
  if (d.lens) {
    if (d.lens.style !== undefined && !['A', 'B', 'C'].includes(d.lens.style)) throw new ThemeError('lens.style is A, B or C');
    if (d.lens.wash !== undefined && !(d.lens.wash >= 0 && d.lens.wash <= 1)) throw new ThemeError('lens.wash is between 0 and 1');
  }
  const tu = d.mark?.tumble;
  if (tu !== undefined && (tu.length !== 2 || tu.some((s) => ms(s) === null))) throw new ThemeError('mark.tumble is two durations');
  for (const scheme of sides) {
    const bad = contrastProblem(resolve(d, scheme, false));
    if (bad !== null) throw new ThemeError(`${bad} in ${scheme}`);
  }
}

// ---- Resolution

/** The one scheme a theme shows: the wanted one, limited to the sides the theme defines. */
export function limited(wanted: string, d: ThemeDoc | undefined): string {
  const sides = d ? sidesOf(d) : ['light', 'dark'];
  return sides.includes(wanted) ? wanted : sides[0] ?? 'light';
}

export function resolve(d: ThemeDoc, scheme: string, reduced: boolean): Theme {
  const t = defaultTheme();
  t.id = d.id; t.name = d.name; t.scheme = scheme; t.schemes = sidesOf(d).join(',');
  const c = (k: string, fallback: string): string => { const v = d.colors[k]; return v ? (scheme === 'dark' ? v.dark : v.light) : fallback; };
  t.paper = c('paper', t.paper); t.paper2 = c('paper2', t.paper); t.fg = c('fg', t.fg); t.muted = c('muted', t.muted);
  t.accent = c('accent', t.accent); t.accent2 = c('accent2', t.accent); t.success = c('success', t.success);
  t.danger = c('danger', t.danger); t.onAccent = c('onAccent', t.onAccent); t.rule = c('rule', t.rule); t.scrim = c('scrim', t.scrim); t.onScrim = c('onScrim', t.onScrim);
  if (d.type) {
    const f = (s: string | undefined, fb: string): string => (s === undefined ? fb : face(s) ?? fb);
    t.fontRead = f(d.type.read, t.fontRead); t.fontUi = f(d.type.ui, t.fontUi); t.fontMono = f(d.type.mono, t.fontMono); t.scale = d.type.scale ?? t.scale;
    if (t.fontUi === 'ui-rounded') { t.fontLabel = 'ui-rounded'; t.fontAction = 'ui-rounded'; }
  }
  const px = (s: string | undefined): number | undefined => (s === undefined ? undefined : double(s.replaceAll('px', '')) ?? undefined);
  if (d.radii) { t.radiusCard = px(d.radii.card) ?? t.radiusCard; t.radiusControl = px(d.radii.control) ?? t.radiusControl; t.radiusPill = px(d.radii.pill) ?? t.radiusPill; t.radiusRules = px(d.radii.rules) ?? t.radiusRules; }
  // Owner ruling, 2026-11-15: "no pills". Cards 8 to 12, blocks 4 to 6, never half the height, whatever a theme asks for; the "pill" radius is a block's.
  t.radiusCard = Math.min(Math.max(t.radiusCard, 8), 12); t.radiusControl = Math.min(Math.max(t.radiusControl, 4), 6); t.radiusPill = t.radiusControl;
  const edge = (s: string | undefined): string => (s ?? 'none').replaceAll(' rule', ' ' + t.rule);   // "0 1px 0 rule" names a colour token: the resolved rule colour
  const e = d.elevation;
  if (e) {
    const l1 = e['1'], l2 = e['2'], l3 = e['3'];
    if (l1) { t.shadow1 = l1.shadow ?? 'none'; t.edge1 = edge(l1.edge); t.material1 = l1.material ?? 'solid'; t.press1 = l1.press ?? 1; }
    if (l2) { t.shadow2 = l2.shadow ?? 'none'; t.edge2 = edge(l2.edge); t.material2 = l2.material ?? 'solid'; t.barBacking = l2.backing ?? t.barBacking; }
    if (l3) { t.shadow3 = l3.shadow ?? 'none'; t.edge3 = edge(l3.edge); t.material3 = l3.material ?? 'solid'; t.behindScale3 = l3.behind?.scale ?? 1; t.behindDim3 = l3.behind?.dim ?? 0; t.layerBacking = l3.backing ?? t.layerBacking; }
  }
  const joined = (shadow: string, ed: string): string => (shadow === 'none' ? ed : ed === 'none' ? shadow : shadow + ', ' + ed);
  t.shadow1 = joined(t.shadow1, t.edge1); if (t.material2 !== 'glass') t.shadow2 = joined(t.shadow2, t.edge2); t.shadow3 = joined(t.shadow3, t.edge3);
  const m = d.motion;
  if (m) {
    t.motionFastMs = ms(m.fast) ?? t.motionFastMs; t.motionEnterMs = ms(m.enter) ?? t.motionEnterMs; t.motionSheetMs = ms(m.sheet) ?? t.motionSheetMs; t.staggerMs = ms(m.stagger) ?? t.staggerMs;
    t.springStiffness = m.spring?.stiffness ?? t.springStiffness; t.springDamping = m.spring?.damping ?? t.springDamping; t.springMass = m.spring?.mass ?? t.springMass;
  }
  if (d.lens) { t.lensStyle = d.lens.style ?? t.lensStyle; t.lensWash = d.lens.wash ?? t.lensWash; }
  if (d.mark) { t.markDriftS = (ms(d.mark.drift) ?? 0) / 1000; t.markListenScale = d.mark.listenScale ?? 1; t.markTumble = (d.mark.tumble ?? []).join(','); }
  if (reduced) {
    // F6: the system's Reduce Motion wins. Entrances use the theme's reduced value (default instant), the spring settles at once (critically damped and stiff), the mark stops drifting when its reduced form is static.
    t.reducedMotion = true;
    t.motionEnterMs = ms(d.motion?.reduced?.enter) ?? 0;
    t.motionSheetMs = Math.min(t.motionSheetMs, 120); t.staggerMs = 0;
    if ((d.motion?.reduced?.spring ?? 'none') === 'none') { t.springStiffness = 900; t.springDamping = 60; t.springMass = 1; }
    if ((d.mark?.reduced ?? 'static') === 'static') { t.markDriftS = 0; t.markListenScale = 1; }
    t.behindScale3 = 1;
  }
  return t;
}

export function resolveExtras(d: ThemeDoc, _scheme: string, reduced: boolean): ThemeExtras {
  const x = defaultExtras();
  const s = TYPE_SCALES[d.type?.scale ?? 'editorial'];
  if (s) { x.sizeNameplate = s.nameplate; x.sizeHeadline = s.headline; x.sizeH2 = s.h2; x.sizeBody = s.body; x.sizeLabel = s.label; x.sizeMeta = s.meta; x.lineBody = s.lineBody; }
  const o = d.type?.sizes;
  if (o) { x.sizeNameplate = o.nameplate ?? x.sizeNameplate; x.sizeHeadline = o.headline ?? x.sizeHeadline; x.sizeH2 = o.h2 ?? x.sizeH2; x.sizeBody = o.body ?? x.sizeBody; x.sizeLabel = o.label ?? x.sizeLabel; x.sizeMeta = o.meta ?? x.sizeMeta; }
  x.flipStiffness = d.mark?.flip?.stiffness ?? x.flipStiffness; x.flipDamping = d.mark?.flip?.damping ?? x.flipDamping;
  const tum = (d.mark?.tumble ?? []).map((v) => ms(v)).filter((v): v is number => v !== null).map((v) => v / 1000);
  if (tum.length === 2) { x.markTumbleAS = tum[0]; x.markTumbleBS = tum[1]; }
  x.markStatic = reduced && (d.mark?.reduced ?? 'static') === 'static';
  x.pressTint = d.elevation?.['1']?.pressTint ?? false;
  x.reduceMotion = reduced;
  x.contrast = contrastLevel(d);
  return x;
}

// ---- The store (the Swift ThemeStore, without its files: the choice and the user's themes are kept by the functions below)

const MODES = ['light', 'dark', 'system'];

export class ThemeStore {
  private docs = new Map<string, ThemeDoc>();
  private origins = new Map<string, string>();       // id -> bundled | file | texture
  private order: string[] = [];
  private sourceProblems = new Map<string, string>(); // source name -> reason (F5)
  private chosenID: string | null = null;            // null: no choice made, the default for the scheme shows
  private mode = 'system';                           // light | dark | system

  /** `bundled`: source name -> theme document (text or parsed). */
  constructor(bundled: Record<string, unknown> = BUNDLED) { for (const k of Object.keys(bundled).sort()) this.add(bundled[k], k, 'bundled'); }

  get problems(): string[] { return [...this.sourceProblems.keys()].sort().map((k) => `${k}: ${this.sourceProblems.get(k)}`); }

  /** A problem against a source that is not a theme the store holds (a file that could not be read at all). */
  noteProblem(source: string, reason: string): void { this.sourceProblems.set(source, reason); }

  /** The choice as it is kept: the chosen id (null: the default pair) and the mode. */
  get choice(): { theme: string | null; mode: string } { return { theme: this.chosenID, mode: this.mode }; }
  /** Restore a kept choice. An unknown id shows the default (it is kept, as the Swift did: the theme may load later); an unknown mode is "system". */
  restore(theme: unknown, mode: unknown): void { this.chosenID = typeof theme === 'string' && theme !== '' ? theme : null; this.mode = typeof mode === 'string' && MODES.includes(mode) ? mode : 'system'; }

  /** Validate and add. Returns the id, or null (the reason is recorded against `source`). `raw` is a document or its JSON text. */
  add(raw: unknown, source: string, origin: string): string | null {
    try {
      const doc = this.prepare(raw);
      if (origin !== 'bundled' && this.origins.get(doc.id) === 'bundled') { this.sourceProblems.set(source, `id ${doc.id} is a bundled theme and cannot be replaced`); return null; }
      this.put(doc, origin); this.sourceProblems.delete(source);
      return doc.id;
    } catch (e) { this.sourceProblems.set(source, e instanceof Error ? e.message : String(e)); return null; }
  }
  private prepare(raw: unknown): ThemeDoc {
    let v = raw;
    if (typeof raw === 'string') { try { v = JSON.parse(raw); } catch { throw new ThemeError('not JSON'); } }
    const doc = decode(v); validate(doc); return doc;
  }
  private put(doc: ThemeDoc, origin: string): void { if (!this.docs.has(doc.id)) this.order.push(doc.id); this.docs.set(doc.id, doc); this.origins.set(doc.id, origin); }

  /** Check a theme served by `GET /api/texture/theme` before anything is written (F7, F9): the document, or the reason it throws. */
  checkInstall(theme: unknown): ThemeDoc {
    const doc = this.prepare(theme);
    if (this.origins.get(doc.id) === 'bundled') throw new ThemeError(`id ${doc.id} is a bundled theme and cannot be replaced`);   // F9
    return doc;
  }
  /** The checked theme is listed, with origin "texture". */
  commitInstall(doc: ThemeDoc): void { this.put(doc, 'texture'); this.sourceProblems.delete(`texture-${doc.id}.json`); }
  /** Install in memory only (the tests, and anything that writes nothing). Throws the reason. */
  installFromTexture(theme: unknown): string { const doc = this.checkInstall(theme); this.commitInstall(doc); return doc.id; }

  /** Remove a theme that came from a file or a Texture (a bundled theme stays). True when something was removed. */
  remove(id: string): boolean {
    const o = this.origins.get(id);
    if (o === undefined || o === 'bundled') return false;
    this.docs.delete(id); this.origins.delete(id); this.order = this.order.filter((x) => x !== id);
    if (this.chosenID === id) this.chosenID = null;
    return true;
  }
  /** The same store without what came from a Texture document (a copy: the real store keeps them for when the boundary is open again). */
  withoutTexture(): ThemeStore {
    const t = new ThemeStore();
    for (const id of this.order) { const o = this.origins.get(id); const d = this.docs.get(id); if (d && o && o !== 'bundled' && o !== 'texture') t.put(d, o); }
    t.chosenID = this.chosenID; t.mode = this.mode;
    return t;
  }
  originOf(id: string): string | undefined { return this.origins.get(id); }
  has(id: string): boolean { return this.docs.has(id); }

  list(h: Host): ThemeInfo[] { const cur = this.themeID(h); return this.order.map((id) => ({ id, name: this.docs.get(id)?.name ?? id, current: id === cur })); }

  /** The scheme asked for: the mode, or the system's. */
  private wanted(h: Host): string { return this.mode === 'system' ? (h.scheme === 'dark' ? 'dark' : 'light') : this.mode; }

  /** The theme in force: the chosen one, else the default for the scheme wanted (the mode, or the system's). */
  themeID(h: Host): string {
    if (this.chosenID !== null && this.docs.has(this.chosenID)) return this.chosenID;
    const d = this.wanted(h) === 'dark' ? DEFAULT_DARK_ID : DEFAULT_LIGHT_ID;
    return this.docs.has(d) ? d : this.order[0] ?? DEFAULT_LIGHT_ID;
  }
  private activeDoc(h: Host): ThemeDoc | undefined { return this.docs.get(this.themeID(h)) ?? (this.order[0] !== undefined ? this.docs.get(this.order[0]) : undefined); }

  /** The one scheme the chosen theme can show (a theme that defines only light or only dark), else null. While it is set the mode control shows that scheme and refuses the other (F10). */
  get lockedScheme(): string | null {
    const sides = this.chosenID !== null ? this.docs.get(this.chosenID)?.schemes : undefined;
    return sides !== undefined && sides.length === 1 ? sides[0] : null;
  }
  get schemeMode(): string { return this.lockedScheme ?? this.mode; }

  /** A theme id, or `DEFAULT_CHOICE` to clear the choice and show the default pair. An unknown id changes nothing. True when the choice changed. */
  setTheme(id: string): boolean {
    if (id === DEFAULT_CHOICE) { const was = this.chosenID !== null; this.chosenID = null; return was; }
    if (this.docs.has(id)) { const was = this.chosenID !== id; this.chosenID = id; return was; }
    return false;
  }
  /** light, dark or system; the other scheme of a locked theme is refused. True when the mode changed. */
  setMode(m: string): boolean {
    if (!MODES.includes(m)) return false;
    const l = this.lockedScheme;
    if (l !== null && m !== l) return false;
    const was = this.mode !== m; this.mode = m; return was;
  }

  /** The scheme in force: the mode's, else the system's, limited to the sides the theme defines. */
  effectiveScheme(h: Host): string { return limited(this.wanted(h), this.activeDoc(h)); }
  current(h: Host): Theme { const d = this.activeDoc(h); return d ? resolve(d, this.effectiveScheme(h), h.reduceMotion) : defaultTheme(); }
  extras(h: Host): ThemeExtras { const d = this.activeDoc(h); return d ? resolveExtras(d, this.effectiveScheme(h), h.reduceMotion) : defaultExtras(); }

  /** Every theme as a preview card, in the scheme in force (or the theme's own side when it lacks that one). The Default entry comes first. */
  cards(h: Host): ThemeCard[] {
    const cur = this.themeID(h);
    const hasChoice = this.chosenID !== null && this.docs.has(this.chosenID);
    const wanted = this.effectiveScheme(h);       // every card in the scheme in force (a locked theme leaves no mixed screen)
    const card = (id: string, d: ThemeDoc): ThemeCard => {
      const t = resolve(d, limited(wanted, d), false);
      return { id, name: d.name, origin: this.origins.get(d.id) ?? 'bundled', current: id === cur && hasChoice, schemes: t.schemes, contrast: contrastLevel(d), scale: t.scale,
        paper: t.paper, paper2: t.paper2, fg: t.fg, muted: t.muted, accent: t.accent, accent2: t.accent2, onAccent: t.onAccent, rule: t.rule, fontRead: t.fontRead, fontUi: t.fontUi,
        radiusCard: t.radiusCard, radiusControl: t.radiusControl, shadow: t.shadow1, material: t.material2 };
    };
    const out: ThemeCard[] = [];
    // The pair member the scheme asked for (the mode, or the system's), never the chosen theme.
    const d = this.docs.get(this.wanted(h) === 'dark' ? DEFAULT_DARK_ID : DEFAULT_LIGHT_ID) ?? this.docs.get(cur);
    if (d) out.push({ ...card(DEFAULT_CHOICE, d), id: DEFAULT_CHOICE, name: 'Default', origin: 'bundled', current: !hasChoice, schemes: 'light,dark' });
    for (const id of this.order) { const doc = this.docs.get(id); if (doc) out.push(card(id, doc)); }
    return out;
  }
}

// ---- What `GET /api/texture/theme?doc=` answers: {doc_id, revision_id, theme, ignored?}

export type TextureTheme = { docID: string; revisionID: string; theme: unknown; ignored: string[] };
/** Parse the route's answer. Also accepts a bare theme object (a file), so a fixture can be either. */
export function parseTextureTheme(body: unknown): TextureTheme {
  if (!isObj(body)) throw new ThemeError('the answer is not an object');
  const theme = isObj(body.theme) ? body.theme : body;
  if (theme.schema_version === undefined) throw new ThemeError('the answer holds no theme');
  return { docID: typeof body.doc_id === 'string' ? body.doc_id : '', revisionID: typeof body.revision_id === 'string' ? body.revision_id : '', theme,
    ignored: Array.isArray(body.ignored) ? body.ignored.filter((x): x is string => typeof x === 'string') : [] };
}

// ---- Kept state: the choice (app:/data/theme-choice.json) and the user's themes (app:/data/themes/*.json)

export const CHOICE_FILE = 'app:/data/theme-choice.json';
export const THEMES_DIR = 'app:/data/themes';
const enc = (o: unknown): Uint8Array => new TextEncoder().encode(JSON.stringify(o));
const fileName = (id: string): string => `texture-${id}.json`;

type Seam = { net: (path: string, query: Record<string, string>, world?: World) => Promise<Reply> };
const realNet: Seam = { net: (path, query, world) => authedJSON(path, { query, world: world ?? currentWorld() }) };

let loading: Promise<ThemeStore> | null = null;
let current: ThemeStore | null = null;
let status = '';
let saving: Promise<unknown> = Promise.resolve();
let dirty = false;      // a save that did not land: the next tap on the same theme tries again
/** A Clear (account.ts): the themes installed from a Texture document (`texture-<id>.json`) carry the old account's document (its name and colours), so they go with it, files and memory. The phone's own
 * choice and its hand-installed themes stay (owner decision 4, narrowed by seat 2's review of fe027e0fc). Throws when a removal fails, so the Clear is not marked clean. */
export async function wipeTextureThemes(storage: Storage | undefined): Promise<void> {
  loading = null; current = null;                           // the store is read again from what is left
  if (!storage) return;
  let names: string[] = [];
  try { names = await storage.fs.readdir(THEMES_DIR); } catch { return; }
  for (const n of names) if (n.startsWith('texture-')) await storage.fs.rm(`${THEMES_DIR}/${n}`);
}
/** The texture theme files still there (names), for the survivor check; the folder itself when it cannot be read for a reason other than being absent. */
export async function textureThemesLeft(storage: Storage | undefined): Promise<string[]> {
  if (!storage) return [];
  try { return (await storage.fs.readdir(THEMES_DIR)).filter((n) => n.startsWith('texture-')); } catch (e) { return (e as { code?: unknown } | null)?.code === 'ENOENT' ? [] : [THEMES_DIR]; }
}
/** Forget everything held (the tests; an account change does NOT call this: the theme is the phone's, not the account's). */
export function resetThemes(): void { loading = null; current = null; status = ''; saving = Promise.resolve(); migrated = false; dirty = false; hadChoiceFile = false; choiceTouched = false; migrating = null; migrateTries = 0; rev = 0; }

async function readJSON(storage: Storage | undefined, path: string): Promise<unknown | undefined> {
  if (!storage) return undefined;
  try { return JSON.parse(new TextDecoder().decode(await storage.fs.readFile(path))); } catch { return undefined; }
}

async function ensureDir(storage: Storage): Promise<void> { try { await storage.fs.mkdir(THEMES_DIR); } catch { /* it is there */ } }

const LEGACY_WAIT_MS = 1200;     // a first launch after the update waits at most this long for the old copy; a late answer is read at the next launch
const LEGACY_FILES = 64;         // the native read sends at most this many files, each at most 64 KiB (ModelsEntry.swift); the rest are named in `skipped`
/** Whether this phone's old copy has been carried over. ONLY `migrate` sets it (a theme tap in a launch where the read timed out must not mark it done). */
let migrated = false;
let hadChoiceFile = false;
let choiceTouched = false;      // the person chose something in this launch: the old choice must not override it
let migrating: Promise<void> | null = null;
let migrateTries = 0;
let rev = 0;
const HANDLE_WAIT_MS = 1500;     // as host.ts: a native module may bind after the first sources answer
/** The ONE native read, once. A phone that has the Swift app's choice (UserDefaults AutoradioThemeID, AutoradioThemeMode) and its user themes (Application Support/Autoradio/themes) keeps them: shared code
 * cannot see those places (Exact's `app:/data` is a different folder), so the app module answers `{op: "themeLegacy"}` with `{found, theme, mode, files: {name: text}, skipped: [name]}`. The old copy is NOT
 * removed (a reverted build still reads it, so "revert" stays a true rollback); a later change removes it when phones have moved. A file the read could not carry (over 64 KiB, or past the 64th)
 * is shown as a problem under its own name, never dropped silently. Android answers nothing (no handle, nothing to carry). It is complete when the choice file says `migrated`, written last by this
 * function alone, so a failure part-way or a read that timed out is tried again at the next launch. Delete this function, the native op (AutoradioLab/modules/apple/ModelsEntry.swift) and the `migrated` field
 * once phones have moved. */
async function migrate(storage: Storage | undefined, s: ThemeStore, fresh: () => boolean): Promise<boolean> {
  const later = nativeLater();
  if (!storage || !later) return false;
  const w = currentWorld();                       // a Texture theme carries an account's document: none is written once the boundary has moved (this runs on the raw storage, outside the gate)
  let r: unknown;
  try { r = await Promise.race([later({ op: 'themeLegacy' }), new Promise<null>((res) => setTimeout(() => res(null), LEGACY_WAIT_MS))]); } catch { return false; }
  if (!isObj(r) || typeof r.found !== 'boolean') return false;
  const plain = (n: string): boolean => /^[^/\\]+\.json$/.test(n) && !n.startsWith('.');
  const all = isObj(r.files) ? Object.entries(r.files).filter((e): e is [string, string] => typeof e[1] === 'string' && plain(e[0])).sort((a, b) => (a[0] < b[0] ? -1 : 1)) : [];
  const files = all.slice(0, LEGACY_FILES);
  const skipped = [...(Array.isArray(r.skipped) ? r.skipped.filter((x): x is string => typeof x === 'string' && plain(x)) : []), ...all.slice(LEGACY_FILES).map((e) => e[0])];
  try {
    if (files.length > 0) await ensureDir(storage);
    for (const [name, body] of files) {
      if (name.startsWith('texture-') && (w.closed || w.owed || currentWorld() !== w)) { s.noteProblem(name, 'not carried over: the account changed while it was read'); continue; }
      // A file already in the new place is the newer one (a theme installed by this build, or an earlier try of this carry-over): it is kept, never written over.
      let there = true; try { await storage.fs.readFile(`${THEMES_DIR}/${name}`); } catch { there = false; }
      if (!there) { await storage.fs.atomicWriteFile(`${THEMES_DIR}/${name}`, new TextEncoder().encode(body)); s.add(body, name, name.startsWith('texture-') ? 'texture' : 'file'); }
    }
    for (const name of skipped) s.noteProblem(name, 'could not be carried over from the old place (too large, or too many files); it is still there');
    // The choice is the person's from the old place only when this phone has made none here (a choice file already there wins).
    if (fresh()) s.restore(r.theme, r.mode);       // asked NOW: a choice made while the read was out wins
    await storage.fs.atomicWriteFile(CHOICE_FILE, enc({ ...s.choice, migrated: true }));
  } catch { return false; }
  migrated = true;
  return true;
}

/** The carry-over, apart from the first paint. `host` is hostCaps.mode ("" not decided, "native", "none"): with no handle yet it waits a bounded time for one (the module may bind after the first sources
 * answer), and with "none" (Android) or no handle by then it does nothing and tries again when `host` changes. It answers the revision the theme sources take as an argument, which moves once a carry-over
 * has changed what they show, so they are asked again. At most three tries a launch. */
export async function themeMigrate(host: string, storage: Storage | undefined): Promise<{ rev: number }> {
  const s = await themeStore(storage);
  if (migrated || !storage) return { rev };
  if (!migrating) {
    migrating = (async () => {
      let later = nativeLater();
      for (let waited = 0; !later && host !== 'none' && waited < HANDLE_WAIT_MS; waited += 50) { await new Promise((r) => setTimeout(r, 50)); later = nativeLater(); }     // (no clock in a data source: counted in steps)
      if (!later || migrateTries >= 3) { console.log(`themes: carry-over not run (handle=${later ? 'yes' : 'no'} host=${host} tries=${migrateTries})`); return; }
      migrateTries++;
      const done = await migrate(storage, s, () => !hadChoiceFile && !choiceTouched);
      if (done) rev++;
      console.log(`themes: carry-over ${done ? 'done' : 'did not finish'} (rev=${rev} choice=${s.choice.theme ?? 'default'}/${s.choice.mode} themes=${s.list({ scheme: 'light', reduceMotion: false }).length})`);
    })().finally(() => { migrating = null; });
  }
  await migrating;
  return { rev };
}

/** The store, loaded once: bundled themes, the user's files, the kept choice (and the migration on a phone that has the old one). */
export function themeStore(bound: Storage | undefined): Promise<ThemeStore> {
  const storage = unbound(bound);                           // the device's own settings: a world closing under a gated caller must not cut the store's load (only a Texture-derived write stays in its world)
  if (current) return Promise.resolve(current);
  if (!loading) {
    loading = (async () => {
      const s = new ThemeStore(); const w = currentWorld();
      const kept = await readJSON(storage, CHOICE_FILE);
      const choice = isObj(kept) ? kept : null;
      let names: string[] = [];
      try { names = storage ? (await storage.fs.readdir(THEMES_DIR)).filter((n) => n.endsWith('.json')).sort() : []; } catch { /* no folder yet */ }
      for (const n of names) {
        try { const body = new TextDecoder().decode(await storage!.fs.readFile(`${THEMES_DIR}/${n}`)); if (n.startsWith('texture-') && w.closed) continue; s.add(body, n, n.startsWith('texture-') ? 'texture' : 'file'); }      // a Texture theme read across a boundary move carries the old account's document: not restored
        catch { /* a file that cannot be read is skipped */ }
      }
      s.restore(choice?.theme, choice?.mode);
      migrated = choice?.migrated === true; hadChoiceFile = choice !== null;      // the carry-over is NOT done here: the first paint does not wait for the native handle (themeMigrate)
      current = s;
      return s;
    })().catch((e) => { loading = null; throw e; });
  }
  return loading;
}

/** Writes run one after another; each writes the choice as it is WHEN IT RUNS, so the last choice wins the file. */
function saveChoice(bound: Storage | undefined, s: ThemeStore): Promise<void> {
  const storage = unbound(bound);
  const run = saving.then(async () => { if (storage) await storage.fs.atomicWriteFile(CHOICE_FILE, enc({ ...s.choice, ...(migrated ? { migrated: true } : {}) })); dirty = false; }, () => undefined);
  saving = run.catch(() => undefined);
  return run.catch(() => undefined);     // the choice holds in memory for this launch; a failed write is not the person's error
}

// ---- The sources (app.ts). `host` is [prefersColorScheme, prefersReducedMotion] from `exactViewport()`.

const hostOf = (scheme: string, reduce: boolean): Host => ({ scheme, reduceMotion: reduce === true });
/** What the exempt theme sources show: a Texture theme carries an account's document, so while the boundary is owed (a sign-out or a Clear began and no gate has confirmed it open) it is not shown. */
async function shownStore(storage: Storage | undefined): Promise<ThemeStore> { const s = await themeStore(storage); return currentWorld().owed ? s.withoutTexture() : s; }
export async function themeShared(scheme: string, reduce: boolean, storage: Storage | undefined): Promise<Theme> { return (await shownStore(storage)).current(hostOf(scheme, reduce)); }
export async function themeExtrasShared(scheme: string, reduce: boolean, storage: Storage | undefined): Promise<ThemeExtras> { return (await shownStore(storage)).extras(hostOf(scheme, reduce)); }
export async function themeListShared(scheme: string, reduce: boolean, storage: Storage | undefined): Promise<ThemeList> {
  const s = await shownStore(storage), h = hostOf(scheme, reduce);
  return { rows: s.list(h), mode: s.schemeMode, themeID: s.themeID(h), scheme: s.effectiveScheme(h), problems: s.problems, locked: s.lockedScheme ?? '' };
}
export async function themeCardsShared(scheme: string, reduce: boolean, storage: Storage | undefined): Promise<ThemeCards> {
  const s = await shownStore(storage), h = hostOf(scheme, reduce);
  return { rows: s.cards(h), mode: s.schemeMode, scheme: s.effectiveScheme(h), themeID: s.themeID(h), problems: s.problems, status };
}

export async function themeSet(id: string, storage: Storage | undefined): Promise<Ack> {
  const s = await themeStore(storage);
  if (s.setTheme(id)) { dirty = true; choiceTouched = true; }
  if (dirty) await saveChoice(storage, s);
  return { ok: true, message: '' };
}
export async function schemeSet(mode: string, storage: Storage | undefined): Promise<Ack> {
  const s = await themeStore(storage);
  if (s.setMode(mode)) { dirty = true; choiceTouched = true; }
  if (dirty) await saveChoice(storage, s);
  return { ok: true, message: '' };
}
export async function themeRemove(id: string, storage: Storage | undefined): Promise<Ack> {
  const s = await themeStore(storage);
  const was = s.choice.theme;
  if (s.remove(id)) {
    if (storage) for (const n of [fileName(id), `${id}.json`]) { try { await storage.fs.rm(`${THEMES_DIR}/${n}`); } catch { /* not there */ } }
    if (was === id) await saveChoice(storage, s);
  }
  status = '';
  return { ok: true, message: '' };
}

/** Install a theme from a Texture (a theme document with one choir-theme block). */
export async function themeFromTexture(docId: string, storage: Storage | undefined, seam: Seam = realNet): Promise<Ack> {
  const say = (ok: boolean, message: string): Ack => { status = message; return { ok, message }; };
  const doc = docId.trim();
  if (doc === '') return say(false, 'Enter the id of a theme Texture.');
  const w = currentWorld();                                                // the document id and the request belong to the world this began in, however long the store takes to load
  try {
    const s = await themeStore(storage);
    const body: unknown = (await seam.net('/api/texture/theme', { doc }, w)).body;
    const t = parseTextureTheme(body);
    const checked = s.checkInstall(t.theme);                              // validated before anything is written (F7)
    if (storage) { await ensureDir(storage); await storage.fs.atomicWriteFile(`${THEMES_DIR}/${fileName(checked.id)}`, enc(t.theme)); }
    s.commitInstall(checked);
    return say(true, `Added ${checked.name} from the Texture` + (t.ignored.length === 0 ? '.' : ` (the phone ignored: ${t.ignored.join(', ')}).`));
  } catch (e) {
    if (e instanceof HTTPError) return say(false, e.status === 404 ? 'No theme Texture with that id.' : e.status === 422 ? `That Texture is not a usable theme: ${e.message.replace(/^HTTP 422: ?/, '').slice(0, 120)}` : `The server refused (${e.status}).`);
    if (e instanceof SignedOutError) return say(false, e.reason === 'refused' ? WORDS.rejected : e.reason === 'locked' ? WORDS.locked : e.reason === 'unknown' ? WORDS.unknown : 'Sign in to add a theme from a Texture.');
    return say(false, (e instanceof Error ? e.message : String(e)).slice(0, 160));
  }
}
