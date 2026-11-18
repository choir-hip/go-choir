// The layout engine: a pure function from ranked stories, a layout document, a width and a text measurer to placed bands.
// Ported from the Swift this replaced (Layout/LayoutEngine.swift). Rules walk the ranked pool band by band; each band's
// stories leave the pool. The document is mobile/layouts/default.json (mounted as ./layouts, see app.json).
//
// Ways this can fail (each has a case in paper.test.ts):
//  1 no stories, one, two: no crash, no empty band.   2 equal weights: the order is total (weight, newer, route).
//  3 a headline too long for every step: the smallest step, lines clamped.   4 a pair that does not fit degrades to a stack.
//  5 a route twice in the input: once.   6 a rule naming an unknown module or condition: skipped.
//  7 each story in at most one band.   8 width tiny or zero: floored.   9 missing importance, sources, date, dek: defaults.
// 10 opinion never leads.
//
// Text is measured by an estimate (greedy wrap, an average glyph width), not by the platform: the Swift engine asked Core Text,
// which TypeScript cannot. Line counts may differ by a line from the old phone layout on a headline near a step boundary.

import layoutJson from './layouts/default.json';
import { type Story, isOpinion, relative, slug, weight } from './paper-model.ts';

export type Step = { size: number; weight: number; lineHeight: number };
type Slot = { span: number; headline: string[]; maxLines: number; dek?: boolean; dekLines?: number; meta?: string };
type Module = {
  slots?: Slot[]; rule?: string; degrade?: string; stackHeadline?: string[]; stackMaxLines?: number;
  follow?: { max: number; headline: string[]; label: string }; list?: { max: number; headline: string[]; label?: string };
  bySection?: boolean; perSection?: number; maxSections?: number; headline?: string[];
};
type Rule = { group?: string; if?: string; use?: string; else?: string; then?: string };
export type Doc = {
  schema: string; name: string; grid: { units: number; gutter: number; margin: number }; steps: Record<string, Step>;
  freshHours?: number; modules: Record<string, Module>; rules: Rule[];
};

export const DEFAULT_DOC = layoutJson as unknown as Doc;

/** The document in `json`, or the default when it is malformed or not this schema. */
export function loadDoc(json: string): Doc {
  try {
    const d = JSON.parse(json) as Doc;
    if (d.schema === 'choir.layout.v1' && d.grid && d.grid.units > 0 && d.steps && d.modules && Array.isArray(d.rules)) return d;
  } catch { /* fall through */ }
  return DEFAULT_DOC;
}

export type Item = {
  route: string; title: string; dek: string; section: string; meta: string; byline: string; entityLabels: string; entityIds: string;
  size: number; weight: number; lineHeight: number; lines: number; showDek: boolean; dekSize: number; dekLines: number; indent: boolean; rank: number;
};
export type Band = { id: string; module: string; label: string; side: boolean; rule: boolean; items: Item[] };

export type Measurer = (text: string, size: number, weight: number, width: number) => number;

/** Greedy word wrap with an average glyph width and the 3 percent safety margin the Core Text measurer used. */
export const approxMeasurer: Measurer = (text, size, wt, width) => {
  if (text === '') return 0;
  const w = Math.max(width * 0.97, 1);
  const glyph = size * 0.5 * (wt >= 600 ? 1.07 : 1.0);
  let lines = 1, x = 0;
  for (const word of text.split(' ').filter((s) => s !== '')) {
    const len = Array.from(word).length * glyph;
    const space = x === 0 ? 0 : glyph;
    if (x + space + len <= w) { x += space + len; continue; }
    const own = Math.max(1, Math.ceil(len / w));
    lines += own - (x === 0 ? 1 : 0);
    x = own === 1 ? len : len - (own - 1) * w;
  }
  return lines;
};

const FALLBACK_STEP: Step = { size: 17, weight: 600, lineHeight: 1.25 };
const DISTANT_PAST = Number.NEGATIVE_INFINITY;

/** Stories in rank order: priority (a higher count first), weight, newer, then route; a route twice is kept once. */
export function rank(stories: Story[], now: number, priority: Record<string, number> = {}): Story[] {
  const seen = new Set<string>();
  const unique = stories.filter((s) => (seen.has(s.route) ? false : (seen.add(s.route), true)));
  return unique.sort((a, b) => {
    const pa = priority[a.route] ?? 0, pb = priority[b.route] ?? 0;
    if (pa !== pb) return pb - pa;
    const wa = weight(a, now), wb = weight(b, now);
    if (wa !== wb) return wb - wa;
    const ua = a.updated ?? DISTANT_PAST, ub = b.updated ?? DISTANT_PAST;
    if (ua !== ub) return ub - ua;
    return a.route < b.route ? -1 : a.route > b.route ? 1 : 0;
  });
}

export function compose(stories: Story[], width: number, now: number, priority: Record<string, number> = {},
  doc: Doc = DEFAULT_DOC, measure: Measurer = approxMeasurer): Band[] {
  const all = rank(stories, now, priority);
  let pool = all.filter((s) => !isOpinion(s));
  let opinions = all.filter((s) => isOpinion(s));
  if (pool.length === 0 && opinions.length === 0) return [];
  const content = Math.max(width - 2 * doc.grid.margin, 120);
  const freshCut = now - (doc.freshHours ?? 12) * 3600 * 1000;
  const fresh = pool.filter((s) => (s.updated ?? DISTANT_PAST) >= freshCut).length;
  let anchor: Story | null = null;
  const bands: Band[] = [];
  let rankN = 0;
  const resolved = new Set<string>();
  const nextRank = (): number => ++rankN;

  const slotWidth = (span: number): number => {
    const u = Math.max(doc.grid.units, 1), g = doc.grid.gutter;
    const col = (content - g * (u - 1)) / u;
    return Math.max(col * span + g * Math.max(span - 1, 0), 60);
  };
  const related = (): number => {
    if (!anchor) return 0;
    const ids = new Set(anchor.entities.map((e) => e.id));
    return pool.filter((s) => s.entities.some((e) => ids.has(e.id))).length;
  };
  const weightOf = (i: number): number => (i < pool.length ? weight(pool[i], now) : 0);
  const compare = (a: number, op: string, b: number): boolean =>
    op === '>=' ? a >= b : op === '<=' ? a <= b : op === '>' ? a > b : op === '<' ? a < b : op === '==' ? a === b : false;
  const term = (t: string): boolean => {
    let m = /^top\.weight\s*(>=|<=|>|<)\s*([0-9.]+)\s*\*\s*second\.weight$/.exec(t);
    if (m) { if (pool.length === 0) return false; return compare(weightOf(0), m[1], parseFloat(m[2]) * weightOf(1)); }
    m = /^(fresh|remaining|related\(lead\))\s*(>=|<=|>|<|==)\s*([0-9.]+)$/.exec(t);
    if (m) { const v = m[1] === 'fresh' ? fresh : m[1] === 'remaining' ? pool.length : related(); return compare(v, m[2], parseFloat(m[3])); }
    m = /^has\((\w+)\)$/.exec(t);
    if (m) { const name = m[1]; return bands.some((b) => b.module === name || (name === 'lead' && b.module === 'colead')); }
    return false;
  };
  const cond = (text: string): boolean => text.split(' and ').every((t) => term(t.trim()));
  const take = (n: number): Story[] => pool.splice(0, n);

  const fit = (title: string, steps: string[], maxLines: number, w: number): { step: Step; lines: number; fitted: boolean } => {
    const cands = steps.map((s) => doc.steps[s]).filter((s): s is Step => !!s);
    const smallest = Object.values(doc.steps).reduce<Step | null>((a, s) => (a === null || s.size < a.size ? s : a), null);
    const list = cands.length > 0 ? cands : [smallest ?? FALLBACK_STEP];
    for (const st of list) {
      const n = measure(title, st.size, st.weight, w);
      if (n <= maxLines) return { step: st, lines: n, fitted: true };
    }
    return { step: list[list.length - 1], lines: maxLines, fitted: false };
  };
  const itemFit = (s: Story, slot: Slot, w: number, rk: number, withEntities = false): { item: Item; fitted: boolean } => {
    const f = fit(s.title, slot.headline, slot.maxLines, w);
    const dekStep = doc.steps['dek'] ?? { size: 16, weight: 400, lineHeight: 1.4 };
    let show = false, dl = 0;
    if (slot.dek === true && s.dek !== '') {
      const n = measure(s.dek, dekStep.size, dekStep.weight, w);
      if (n <= (slot.dekLines ?? 3)) { show = true; dl = n; }
    }
    const parts: string[] = [];
    if (slot.meta === 'full') {
      if (s.sources > 0) parts.push(`${s.sources} sources`);
      if (s.versions > 1) parts.push(`${s.versions} versions`);
    } else if (slot.meta === 'short' && s.sources > 0) parts.push(`${s.sources} sources`);
    const rel = relative(s.updated, now);
    if (slot.meta !== undefined && rel !== '') parts.push(rel);
    const ents = s.entities.slice(0, 3);
    const item: Item = {
      route: s.route, title: s.title, dek: show ? s.dek : '', section: s.section === '' ? 'News' : s.section, meta: parts.join(' · '), byline: s.author,
      entityLabels: withEntities || slot.meta === 'full' ? ents.map((e) => e.label).join(' · ') : '', entityIds: s.entities.map((e) => e.id).join(','),
      size: f.step.size, weight: f.step.weight, lineHeight: f.step.lineHeight, lines: f.lines, showDek: show, dekSize: dekStep.size, dekLines: dl, indent: false, rank: rk,
    };
    return { item, fitted: f.fitted };
  };
  const item = (s: Story, slot: Slot, w: number, rk: number, withEntities: boolean): Item => itemFit(s, slot, w, rk, withEntities).item;
  const flat = (headline: string[], span = doc.grid.units): Slot => ({ span, headline, maxLines: 3, dek: false, dekLines: 0 });

  for (const r of doc.rules) {
    const name = r.use ?? r.else ?? r.then;
    const mod = name ? doc.modules[name] : undefined;
    if (!name || !mod) continue;
    if (r.group && resolved.has(r.group)) continue;
    if (r.if !== undefined && !cond(r.if)) continue;
    if (r.group) resolved.add(r.group);
    switch (name) {
      case 'lead': case 'colead': {
        const slots = mod.slots;
        if (!slots || slots.length === 0 || pool.length === 0) continue;
        const n = name === 'colead' && pool.length >= 2 ? 2 : 1;
        const ss = take(n);
        if (anchor === null) anchor = ss[0];
        const items = ss.map((s, i) => item(s, slots[Math.min(i, slots.length - 1)], slotWidth(6), nextRank(), i === 0 && name === 'lead'));
        bands.push({ id: `${name}-${bands.length}`, module: n === 2 ? 'colead' : 'lead', label: '', side: false, rule: n === 2 && mod.rule === 'between', items });
        break;
      }
      case 'pair': {
        const slots = mod.slots;
        if (!slots || slots.length < 2 || pool.length < 2) continue;
        const ss = take(2);
        const half = slotWidth(slots[0].span);
        const items = ss.map((s, i) => itemFit(s, slots[i], half, nextRank()));
        const fits = items.every((i) => i.fitted);
        if (fits || mod.degrade !== 'stack') {
          bands.push({ id: `pair-${bands.length}`, module: 'pair', label: '', side: true, rule: false, items: items.map((i) => i.item) });
        } else {
          const sl: Slot = { ...slots[0], headline: mod.stackHeadline ?? slots[0].headline, maxLines: mod.stackMaxLines ?? slots[0].maxLines, span: doc.grid.units };
          const stacked = ss.map((s, i) => item(s, sl, slotWidth(doc.grid.units), items[i].item.rank, false));
          bands.push({ id: `pair-${bands.length}`, module: 'pair', label: '', side: false, rule: true, items: stacked });
        }
        break;
      }
      case 'cluster': {
        const f = mod.follow;
        if (!f || anchor === null) continue;
        const ids = new Set(anchor.entities.map((e) => e.id));
        const rel = pool.filter((s) => s.entities.some((e) => ids.has(e.id))).slice(0, f.max);
        if (rel.length === 0) continue;
        const gone = new Set(rel.map((s) => s.route));
        pool = pool.filter((s) => !gone.has(s.route));
        const sl = flat(f.headline);
        const items = rel.map((s) => ({ ...item(s, sl, slotWidth(doc.grid.units) - 16, nextRank(), false), indent: true }));
        bands.push({ id: `cluster-${bands.length}`, module: 'cluster', label: f.label, side: false, rule: false, items });
        break;
      }
      case 'briefs': {
        const l = mod.list;
        if (!l || pool.length === 0) continue;
        const ss = take(l.max);
        const sl = flat(l.headline);
        bands.push({ id: `briefs-${bands.length}`, module: 'briefs', label: l.label ?? '', side: false, rule: true,
          items: ss.map((s) => item(s, sl, slotWidth(doc.grid.units), nextRank(), false)) });
        break;
      }
      case 'river': {
        if (mod.bySection !== true || pool.length === 0) continue;
        const order: string[] = [];
        for (const s of pool) { const k = s.section === '' ? 'News' : s.section; if (!order.includes(k)) order.push(k); }
        const sl = flat(mod.headline ?? ['s']);
        for (const sec of order.slice(0, mod.maxSections ?? 5)) {
          const ss = pool.filter((s) => (s.section === '' ? 'News' : s.section) === sec).slice(0, mod.perSection ?? 4);
          const gone = new Set(ss.map((s) => s.route));
          pool = pool.filter((s) => !gone.has(s.route));
          bands.push({ id: `river-${slug(sec)}`, module: 'river', label: sec, side: false, rule: false,
            items: ss.map((s) => item(s, sl, slotWidth(doc.grid.units), nextRank(), false)) });
        }
        break;
      }
      case 'opinion': {
        const l = mod.list;
        if (!l || opinions.length === 0) continue;
        const ss = opinions.slice(0, l.max);
        opinions = opinions.slice(ss.length);
        const sl = flat(l.headline);
        bands.push({ id: 'opinion', module: 'opinion', label: l.label ?? 'Opinion', side: false, rule: true,
          items: ss.map((s) => item(s, sl, slotWidth(doc.grid.units), nextRank(), false)) });
        break;
      }
      default: continue;
    }
  }
  return bands;
}

/** The modules in order, a stacked pair marked: "lead+cluster+pair(stack)+briefs+river". */
export function composition(bands: Band[]): string {
  const out: string[] = [];
  for (const b of bands) {
    const m = b.module === 'pair' && !b.side ? 'pair(stack)' : b.module;
    if (m === 'river' && out[out.length - 1] === 'river') continue;
    out.push(m);
  }
  return out.join('+');
}

/** Flatten bands into reading order; a route in two bands is listed once, at its first place. */
export function stack(bands: Band[]): { routes: string[]; titles: string[] } {
  const seen = new Set<string>(), routes: string[] = [], titles: string[] = [];
  for (const b of bands) for (const it of b.items) if (it.route !== '' && !seen.has(it.route)) { seen.add(it.route); routes.push(it.route); titles.push(it.title); }
  return { routes, titles };
}
