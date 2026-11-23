// Search over what the phone holds: the lexical side (BM25 over one index), the fusion with vectors, the vector store and the embedding
// job's work list. Shared code: it replaces the Swift Retrieval.swift, RetrievalRank.swift, HybridRetrieval.swift, the vector store and
// the job of Embedder.swift, and the answer-side of CorpusSearch.swift. Only the model stays native (see `Embed`).
//
// Nothing here reads a clock, a file or the network except through what the caller passes in: time arrives as `now` (epoch ms), the
// model as an `Embed` function, the persisted store as a `Storage`.
//
// RANKING RULE (ported unchanged, tested in search.test.ts):
//   1. BM25 (k1 1.2, b 0.75); a title word counts three times a body word; the title on air adds half-weight terms when the question is
//      short (two or fewer content words) or leans on a pronoun.
//   2. Candidates clear the floor: at least min(2, n) of the n content words, at least half of them, and a fifth of the best score.
//   3. The best by score are taken until the budget (3,584 characters) is spent; at most two passages of one parent; one item per address.
//   4. They are presented newest first (undated last).
//   5. Nothing clearing the floor is one plain sentence telling the model to say it has nothing.
//
// The vectors: EmbeddingGemma 2, 768 dimensions, unit length (docs/reference/EMBEDDINGGEMMA.md section 3). The prompt strings are the
// model card's own and are built here, so the one native call is "embed these finished strings, return vectors".

import type { Storage } from './app.contract.d.ts';

// ---------------------------------------------------------------------------------------------------------------- documents

export type Part = { label: string; text: string; block?: number };
export type Doc = {
  id: string;
  /** story | headline | page | book | podcast | transcript | runsheet | file | mail | note | document */
  kind: string;
  title: string;
  text: string;
  /** Epoch milliseconds, or null. */
  date: number | null;
  source: string;
  address: string;
  parts?: Part[];
  /** Changes when the content changes; an unchanged key is not indexed again. */
  key?: string;
};

export type Item = {
  id: string; address: string; kind: string; title: string; source: string; date: number | null; place: string; score: number; text: string;
};

export const BUDGET = { windowTokens: 2048, share: 0.5, charsPerToken: 3.5, candidates: 20, perParent: 2, maxItemChars: 800, minItemChars: 240 };
/**
 * THE INDEX'S BOUND. The shared runtime has one Hermes module heap of 64 MiB for all of app.ts (docs/reference/exact/internals.md, "heap"); measured under Bun
 * (2026-11-21, review of the retrieval lane): 1,900 passages of the R7 fixture hold about 58 MB. So the index holds at most MAX_INDEX_ITEMS passages in all
 * (at the bound the oldest unpinned documents are evicted for a newer one, and the pinned groups, the page in front, the run sheet, the open pages and the edition topics, always fit and are never evicted; a document that could not be held is not marked indexed, never an error) and cuts one passage's text at MAX_ITEM_CHARS. 1,200 passages is
 * about 35 MB measured the same way: it leaves room for the app's other state. Raising it needs a heap measurement under the real 64 MiB limit first.
 */
export const MAX_INDEX_ITEMS = 1200;
export const MAX_ITEM_CHARS = 4000;
export const BUDGET_CHARS = Math.floor(BUDGET.windowTokens * BUDGET.share * BUDGET.charsPerToken); // 3,584

// ---------------------------------------------------------------------------------------------------------------- tokens

const STOP = new Set(['a', 'an', 'the', 'and', 'or', 'but', 'of', 'to', 'in', 'on', 'at', 'for', 'with', 'by', 'from', 'about', 'as', 'is', 'are', 'was', 'were',
  'be', 'been', 'it', 'its', 'this', 'that', 'these', 'those', 'what', 'which', 'who', 'whom', 'whose', 'when', 'where', 'why', 'how', 'do', 'does', 'did', 'i', 'me',
  'my', 'you', 'your', 'we', 'our', 'they', 'their', 'he', 'she', 'his', 'her', 'him', 'them', 'tell', 'say', 'said', 'says', 'please', 'can', 'could', 'would', 'will',
  'any', 'there', 'has', 'have', 'had', 'not', 'no', 'so', 'if', 'than', 'then', 'just', 'more', 'some', 'much', 'many', 'know', 'show', 'give', 'like', 'want', 'now',
  'latest', 'news', 'anything', 'everything', 'all', 'into', 'out', 'up', 'over']);
/** Words that point at something already on air instead of naming it. */
const PRONOUNS = new Set(['that', 'this', 'it', 'its', 'them', 'they', 'he', 'she', 'him', 'her', 'those', 'these', 'there', 'one', 'more', 'again', 'same']);

const WORD = /[\p{L}\p{N}\p{M}]+/gu;
const scalars = (s: string): number => { let n = 0; for (const _ of s) n++; return n; };
const utf8Length = (s: string): number => new TextEncoder().encode(s).length;

export function stem(w: string): string {
  const n = utf8Length(w);
  if (n <= 3) return w;
  if (w.endsWith('ies') && n > 4) return w.slice(0, -3) + 'y';
  if (w.endsWith('sses')) return w.slice(0, -2);
  if (w.endsWith('ss') || w.endsWith('us') || w.endsWith('is')) return w;
  if (w.endsWith('s')) return w.slice(0, -1);
  return w;
}

/** Lowercased words of letters and digits (any script), a small stop list, light stemming; a stem never changes the first letters. */
export function tokens(s: string, keepStops = false): string[] {
  const out: string[] = [];
  for (const m of s.toLowerCase().matchAll(WORD)) {
    const w = m[0];
    const t = stem(w);
    if (!keepStops && (STOP.has(w) || STOP.has(t))) continue;
    if (scalars(t) > 1 || /^\p{N}$/u.test(t)) out.push(t);
  }
  return out;
}

/** Paragraph-ish units: lines, long ones cut at sentence ends (or a space) to at most `max` characters. */
export function paragraphs(text: string, max = 600): string[] {
  const out: string[] = [];
  for (const raw of text.split('\n')) {
    let rest = raw.trim();
    while (rest.length > max) {
      let cut = max;
      const head = rest.slice(0, cut);
      const stop = Math.max(head.lastIndexOf('.'), head.lastIndexOf('!'), head.lastIndexOf('?'));
      if (stop > max / 3) cut = stop + 1;
      else { const sp = head.lastIndexOf(' '); if (sp > 0) cut = sp; }
      out.push(rest.slice(0, cut).trim());
      rest = rest.slice(cut).trim();
    }
    if (rest !== '') out.push(rest);
  }
  return out;
}

/** Passages of about `words` words, cut at paragraph ends. */
export function passages(text: string, words = 180): string[] {
  const out: string[] = [];
  let buf: string[] = [], count = 0;
  for (const p of paragraphs(text, 900)) {
    const n = p.split(/\s+/).filter((x) => x !== '').length;
    if (count > 0 && (count + n > (words * 3) / 2 || count >= words)) { out.push(buf.join('\n')); buf = []; count = 0; }
    buf.push(p); count += n;
  }
  if (buf.length > 0) out.push(buf.join('\n'));
  return out;
}

// ---------------------------------------------------------------------------------------------------------------- the index

export type Query = { words: string; onAirTitle?: string; pageID?: string | null; now: number };

type Slot = { doc: string; id: string; item: Item; terms: string[]; length: number; live: boolean };

const K1 = 1.2, B = 0.75, TITLE_WEIGHT = 3;

function docKey(d: Doc): string {
  if (d.key !== undefined) return d.key;
  const parts = d.parts ?? [];
  return `${utf8Length(d.text)}|${d.title}|${parts.length}|${parts.reduce((a, p) => a + utf8Length(p.text), 0)}|${d.date ?? 0}`;
}

export function unique<T>(a: T[]): T[] { const s = new Set<T>(); return a.filter((x) => (s.has(x) ? false : (s.add(x), true))); }

export function queryTerms(q: Query): { listener: string[]; onAir: string[]; usedOnAir: boolean } {
  const listener = unique(tokens(q.words));
  const raw = (q.words.toLowerCase().match(WORD) ?? []);
  const pointing = raw.some((w) => PRONOUNS.has(w));
  const air = unique(tokens(q.onAirTitle ?? '')).filter((t) => !listener.includes(t));
  const use = air.length > 0 && (listener.length <= 2 || pointing);
  return { listener, onAir: use ? air : [], usedOnAir: use };
}

export class Index {
  private slots: Slot[] = [];
  private free: number[] = [];
  private postings = new Map<string, { slot: number; tf: number }[]>();
  private children = new Map<string, number[]>();
  private keys = new Map<string, string>();
  private dates = new Map<string, number>();
  private pinned = new Set<string>();
  private liveCount = 0;
  private totalLength = 0;

  get count(): number { return this.liveCount; }
  get documentCount(): number { return this.keys.size; }
  contains(id: string): boolean { return this.keys.has(id); }
  keyOf(id: string): string | undefined { return this.keys.get(id); }

  /**
   * Index a document, or index it again when its key changed. Returns false when it was unchanged, or when it could not be held (then it is not marked indexed,
   * `contains` is false and the next upsert tries again). At the bound (MAX_INDEX_ITEMS) the OLDEST unpinned documents go to make room, never the newest: a
   * document evicts only older ones, and a `pin` document (the page in front, the entry on air, the edition topics: what "this" and "that" point at) evicts any
   * unpinned one and is never evicted.
   */
  upsert(d: Doc, pin = false): boolean {
    const k = docKey(d);
    if (this.keys.get(d.id) === k) return false;
    this.removeLocked(d.id);
    const titleTokens = tokens(d.title);
    const plan: { id: string; text: string; address: string; place: string }[] = [];
    const parts = d.parts ?? [];
    if (parts.length === 0) plan.push({ id: d.id, text: d.text, address: d.address, place: '' });
    else {
      let n = 0;
      for (const p of parts) {
        const base = d.address.split('@')[0];
        const address = p.block !== undefined ? `${base}@${p.block}` : d.address;
        for (const [j, text] of passages(p.text).entries()) {
          n++;
          plan.push({ id: `${d.id}#${n}`, text, address, place: j === 0 ? p.label : p.label === '' ? '' : `${p.label}, passage ${j + 1}` });
        }
      }
    }
    if (!this.makeRoom(d, plan.length, pin)) return false;
    this.keys.set(d.id, k);
    this.dates.set(d.id, d.date ?? -Infinity);
    if (pin) this.pinned.add(d.id); else this.pinned.delete(d.id);
    const mine: number[] = [];
    for (const { id, text: whole, address, place } of plan) {
      const text = whole.length > MAX_ITEM_CHARS ? whole.slice(0, MAX_ITEM_CHARS) : whole;
      const tf = new Map<string, number>();
      let n = 0;
      for (const t of tokens(text)) { tf.set(t, (tf.get(t) ?? 0) + 1); n++; }
      for (const t of titleTokens) tf.set(t, (tf.get(t) ?? 0) + TITLE_WEIGHT);
      const item: Item = { id, address, kind: d.kind, title: d.title, source: d.source, date: d.date, place, score: 0, text };
      const slot: Slot = { doc: d.id, id, item, terms: [...tf.keys()], length: n + titleTokens.length, live: true };
      let idx: number;
      const f = this.free.pop();
      if (f !== undefined) { this.slots[f] = slot; idx = f; } else { this.slots.push(slot); idx = this.slots.length - 1; }
      for (const [t, c] of tf) { const l = this.postings.get(t); const p = { slot: idx, tf: c }; if (l) l.push(p); else this.postings.set(t, [p]); }
      mine.push(idx); this.liveCount++; this.totalLength += slot.length;
    }
    this.children.set(d.id, mine);
    return true;
  }

  /** Make room for `need` passages of `d`: evict the oldest unpinned documents older than `d` (any unpinned one when `d` is pinned). False: it cannot fit, nothing was evicted. */
  private makeRoom(d: Doc, need: number, pin: boolean): boolean {
    const over = this.liveCount + need - MAX_INDEX_ITEMS;
    if (over <= 0) return true;
    const mine = d.date ?? -Infinity;
    const victims: { id: string; date: number; n: number }[] = [];
    for (const [id, idxs] of this.children) {
      if (id === d.id || this.pinned.has(id)) continue;
      const date = this.dates.get(id) ?? -Infinity;
      if (pin || date < mine) victims.push({ id, date, n: idxs.length });
    }
    victims.sort((a, b) => (a.date !== b.date ? a.date - b.date : a.id < b.id ? -1 : 1));
    let freed = 0, take = 0;
    while (take < victims.length && freed < over) freed += victims[take++].n;
    if (freed < over && !pin) return false;                     // not indexed, and nothing is evicted for a document that still would not fit
    for (let i = 0; i < take; i++) this.removeLocked(victims[i].id);
    return true;                                                // a pinned document that still does not fit is held over the bound: it is small, and it is what is in front
  }

  upsertAll(docs: Doc[]): void { for (const d of docs) this.upsert(d); }

  remove(id: string): void { this.removeLocked(id); }
  private removeLocked(id: string): void {
    this.keys.delete(id); this.dates.delete(id); this.pinned.delete(id);
    const mine = this.children.get(id);
    if (!mine) return;
    this.children.delete(id);
    for (const idx of mine) {
      const s = this.slots[idx];
      for (const t of s.terms) {
        const list = this.postings.get(t);
        if (!list) continue;
        const kept = list.filter((p) => p.slot !== idx);
        if (kept.length === 0) this.postings.delete(t); else this.postings.set(t, kept);
      }
      this.totalLength -= s.length; this.liveCount--;
      s.live = false; s.terms = []; s.item = { ...s.item, text: '' };
      this.free.push(idx);
    }
  }
  /** Drop every document not in `keep` (and, with `prefix`, only those whose id starts with it). */
  removeAll(keep: Set<string>, prefix?: string): void {
    for (const id of [...this.keys.keys()]) if (!keep.has(id) && (prefix === undefined || id.startsWith(prefix))) this.removeLocked(id);
  }

  /** BM25 for weighted terms, best first, no floor. */
  scores(terms: [string, number][], matchTerms: Set<string>, boost: (doc: string) => number = () => 1): { slot: number; score: number; matched: number }[] {
    if (this.liveCount === 0) return [];
    const n = this.liveCount, avg = Math.max(1, this.totalLength / n);
    const acc = new Map<number, [number, number]>();
    for (const [t, w] of terms) {
      const list = this.postings.get(t);
      if (!list) continue;
      const d = list.length;
      const idf = Math.log(1 + (n - d + 0.5) / (d + 0.5));
      const isMatch = matchTerms.has(t);
      for (const p of list) {
        const len = this.slots[p.slot].length;
        const s = (w * idf * p.tf * (K1 + 1)) / (p.tf + K1 * (1 - B + (B * len) / avg));
        const e = acc.get(p.slot) ?? [0, 0];
        e[0] += s; if (isMatch) e[1]++;
        acc.set(p.slot, e);
      }
    }
    const out = [...acc].map(([slot, v]) => ({ slot, score: v[0] * boost(this.slots[slot].doc), matched: v[1] }));
    out.sort((a, b) => (a.score !== b.score ? b.score - a.score : a.slot - b.slot));
    return out;
  }

  itemAt(slot: number): Item { return this.slots[slot].item; }
  idf(t: string): number {
    const n = Math.max(this.liveCount, 1), d = this.postings.get(t)?.length ?? 0;
    return Math.log(1 + (n - d + 0.5) / (d + 0.5));
  }
  /** The names of the sources the index holds, with how many documents each. */
  sources(): Record<string, number> {
    const out: Record<string, number> = {};
    for (const idxs of this.children.values()) { const f = idxs[0]; if (f !== undefined && this.slots[f].live) { const s = this.slots[f].item.source; out[s] = (out[s] ?? 0) + 1; } }
    return out;
  }
  /** Every live item, for the embedding job. */
  snapshot(): Item[] { return this.slots.filter((s) => s.live).map((s) => s.item); }
  /** One item by id (a document's, or a passage's `<doc id>#<n>`). */
  item(id: string): Item | null {
    for (const i of this.children.get(id) ?? []) if (this.slots[i].live && this.slots[i].id === id) return this.slots[i].item;
    const parent = id.split('#')[0];
    for (const i of this.children.get(parent) ?? []) if (this.slots[i].live && this.slots[i].id === id) return this.slots[i].item;
    return null;
  }

  /** BM25 with the relevance floor: best first, full text, no budget. */
  candidates(q: Query, limit: number): Item[] {
    const { listener, onAir } = queryTerms(q);
    const matchTerms = new Set(listener.length === 0 ? onAir : listener);
    const weighted: [string, number][] = [...listener.map((t): [string, number] => [t, 1]), ...onAir.map((t): [string, number] => [t, listener.length === 0 ? 1 : 0.5])];
    const page = q.pageID ?? null;
    let ranked = this.scores(weighted, matchTerms, (doc) => (page !== null && doc === page ? 1.25 : 1));
    const need = Math.min(2, matchTerms.size);
    // The floor is measured among items that match the question's own words: one that matches only the title on air must not set the bar.
    ranked = ranked.filter((r) => r.matched >= need && r.matched / Math.max(1, matchTerms.size) >= 0.5);
    const best = ranked[0]?.score ?? 0;
    ranked = ranked.filter((r) => r.score >= best * 0.2);
    return ranked.slice(0, limit).map((r) => ({ ...this.itemAt(r.slot), score: r.score }));
  }

  /** The keyword list for hybrid ranking: BM25 for the listener's words, no floor, no budget. */
  bm25Ranking(words: string, limit = 50): { id: string; score: number }[] {
    const { listener } = queryTerms({ words, now: 0 });
    if (listener.length === 0) return [];
    return this.scores(listener.map((t): [string, number] => [t, 1]), new Set(listener)).slice(0, limit).map((r) => ({ id: this.itemAt(r.slot).id, score: r.score }));
  }

  /** The passages that hold a RARE word of the question (a word that fewer than max(2, 1% of the items) items contain), best BM25 first. */
  exactTermHits(q: Query, limit = 20): string[] {
    const { listener } = queryTerms(q);
    const maxDF = Math.max(2, Math.floor(Math.max(this.count, 1) / 100));
    const best = new Map<string, number>();
    for (const t of listener) {
      const r = this.scores([[t, 1]], new Set([t]));
      if (r.length === 0 || r.length > maxDF) continue;
      for (const h of r) { const id = this.itemAt(h.slot).id; best.set(id, Math.max(best.get(id) ?? 0, h.score)); }
    }
    return [...best].sort((a, b) => (a[1] !== b[1] ? b[1] - a[1] : a[0] < b[0] ? -1 : 1)).slice(0, limit).map((x) => x[0]);
  }
}

// ---------------------------------------------------------------------------------------------------------------- the context

export type Retriever = {
  candidates(q: Query, limit: number): Item[];
  count: number;
  sources(): Record<string, number>;
  idf(t: string): number;
};

export type Record_ = {
  at: number; query: string; onAir: string; terms: string[]; usedOnAir: boolean; considered: number; indexed: number;
  hits: { id: string; address: string; kind: string; score: number; date: number | null; chars: number }[];
  budgetChars: number; usedChars: number; sources: string[];
};
export type Context = { text: string; items: Item[]; record: Record_ };

const flat = (s: string): string => s.split(/\s+/).filter((x) => x !== '').join(' ').replace(/"/g, "'");
export function day(ms: number | null): string {
  if (ms === null) return 'undated';
  // Civil date from days since the epoch (no Date object: the module never reads or builds a clock).
  const z = Math.floor(ms / 86400000) + 719468;
  const era = Math.floor(z / 146097), doe = z - era * 146097;
  const yoe = Math.floor((doe - Math.floor(doe / 1460) + Math.floor(doe / 36524) - Math.floor(doe / 146096)) / 365);
  const doy = doe - (365 * yoe + Math.floor(yoe / 4) - Math.floor(yoe / 100));
  const mp = Math.floor((5 * doy + 2) / 153);
  const dd = doy - Math.floor((153 * mp + 2) / 5) + 1, mm = mp < 10 ? mp + 3 : mp - 9, yy = yoe + era * 400 + (mm <= 2 ? 1 : 0);
  const p = (n: number, w: number): string => String(n).padStart(w, '0');
  return `${p(yy, 4)}-${p(mm, 2)}-${p(dd, 2)}`;
}
const header = (query: string, onAir: string, now: number): string =>
  `Today is ${day(now)}. Passages found on this phone for "${flat(query).slice(0, 120)}", newest first.\n` + (onAir !== '' ? `On air now: "${flat(onAir).slice(0, 120)}".\n` : '');
const nothing = (query: string, searched: number): string =>
  `Nothing on this phone matches "${flat(query).slice(0, 120)}" (searched ${searched} items). Say you have nothing on it; do not answer from memory or from what is playing.\n`;
const label = (i: Item): string => [i.kind, day(i.date), ...(i.source !== '' ? [i.source] : []), ...(i.place !== '' ? [i.place] : [])].join(' | ');
const headline = (n: number, i: Item): string => `Passage ${n} | ${label(i)} | id ${i.address} | "${flat(i.title).slice(0, 120)}"\n`;
const overhead = (i: Item): number => headline(9, i).length + 1;
const block = (n: number, i: Item): string => headline(n, i) + flat(i.text) + '\n';

/** The window of consecutive paragraphs that holds the most (idf-weighted) question words, cut to `limit` characters at a word. */
export function snippet(text: string, terms: Set<string>, idf: (t: string) => number, limit: number): string {
  const paras = paragraphs(text, 500);
  if (paras.length === 0) return '';
  if (text.length <= limit) return flat(text);
  const scored = paras.map((p) => {
    const seen = new Set<string>();
    let s = 0;
    for (const t of tokens(p)) if (terms.has(t)) { if (!seen.has(t)) { seen.add(t); s += idf(t); } else s += 0.1 * idf(t); }
    return s;
  });
  let bestStart = 0, bestScore = -1;
  for (let i = 0; i < paras.length; i++) {
    let len = 0, s = 0, j = i;
    while (j < paras.length && (len + paras[j].length <= limit || j === i)) { len += paras[j].length + 1; s += scored[j]; j++; }
    if (s > bestScore + 1e-9) { bestScore = s; bestStart = i; }
  }
  let out = '', j = bestStart;
  while (j < paras.length && (out.length + paras[j].length + 1 <= limit || out === '')) { out += (out === '' ? '' : ' ') + paras[j]; j++; }
  if (out.length > limit) {
    let cut = limit;
    const sp = out.slice(0, cut).lastIndexOf(' ');
    if (sp > 0) cut = sp;
    out = out.slice(0, cut) + '…';
  }
  return (bestStart > 0 ? '… ' : '') + flat(out);
}

/** The shared part of every retriever: the budget, the per-parent limit, one item per address, newest-first presentation, the record. */
export function retrieve(r: Retriever, q: Query): Context {
  const { listener, onAir, usedOnAir } = queryTerms(q);
  const matchTerms = new Set(listener.length === 0 ? onAir : listener);
  const top = r.candidates(q, BUDGET.candidates);
  const indexed = r.count;
  const chosen: Item[] = [];
  const perParent = new Map<string, number>();
  const seenAddress = new Set<string>();
  const hdr = header(q.words, q.onAirTitle ?? '', q.now);
  let used = hdr.length;
  for (const item of top) {
    const parent = item.id.split('#')[0];
    if ((perParent.get(parent) ?? 0) >= BUDGET.perParent) continue;
    if (item.kind !== 'book' && item.kind !== 'transcript') { if (seenAddress.has(item.address)) continue; seenAddress.add(item.address); }
    const room = BUDGET_CHARS - used - overhead(item);
    if (room < BUDGET.minItemChars) break;
    const it = { ...item, text: snippet(item.text, new Set([...matchTerms, ...onAir]), (t) => r.idf(t), Math.min(BUDGET.maxItemChars, room)) };
    used += overhead(it) + it.text.length;
    perParent.set(parent, (perParent.get(parent) ?? 0) + 1);
    chosen.push(it);
  }
  chosen.sort((a, b) => {
    if (a.date !== null && b.date !== null) return a.date !== b.date ? b.date - a.date : b.score - a.score;
    if (a.date !== null) return -1;
    if (b.date !== null) return 1;
    return b.score - a.score;
  });
  let text: string;
  if (chosen.length === 0) { used = 0; text = nothing(q.words, indexed); }
  else { text = hdr + chosen.map((it, i) => block(i + 1, it)).join(''); used = text.length; }
  const record: Record_ = {
    at: q.now, query: q.words, onAir: q.onAirTitle ?? '', terms: [...listener, ...onAir], usedOnAir, considered: top.length, indexed,
    hits: chosen.map((c) => ({ id: c.id, address: c.address, kind: c.kind, score: Math.round(c.score * 1000) / 1000, date: c.date, chars: c.text.length })),
    budgetChars: BUDGET_CHARS, usedChars: used, sources: Object.keys(r.sources()).sort(),
  };
  return { text, items: chosen, record };
}

export const bm25Retriever = (index: Index): Retriever => ({
  candidates: (q, limit) => index.candidates(q, limit), get count() { return index.count; }, sources: () => index.sources(), idf: (t) => index.idf(t),
});

// ---------------------------------------------------------------------------------------------------------------- vectors

/** The model card's own prompt strings (docs/reference/EMBEDDINGGEMMA.md section 3). */
export const embedPrompt = {
  document: (title: string | null | undefined, text: string): string => {
    const t = (title ?? '').trim();
    return `title: ${t === '' ? 'none' : t} | text: ${text}`;
  },
  query: (q: string): string => `task: search result | query: ${q}`,
};

/** THE ONE NATIVE CALL: finished prompt strings in, one vector each out (not necessarily unit length). The model is on the phone (llama.cpp). */
export type Embed = (prompts: string[]) => Promise<number[][]>;

export const ENGINE = 'embeddinggemma-2-q8_0@llama.cpp-b12259';
export const DIMENSION = 768;
/** Best cosine below this is not a match (set from the trial's unanswerable questions, reference section 0.6). */
export const COSINE_FLOOR = 0.67;

/** The first `dim` values, made unit length again (Matryoshka truncation). */
export function truncatedUnit(v: ArrayLike<number>, dim: number): Float32Array {
  const h = new Float32Array(Math.min(dim, v.length));
  let s = 0;
  for (let i = 0; i < h.length; i++) { h[i] = v[i]; s += h[i] * h[i]; }
  const n = Math.sqrt(s);
  if (n > 0) for (let i = 0; i < h.length; i++) h[i] /= n;
  return h;
}

// Half-precision storage (a row is dimension x 2 bytes), converted by hand: the JS engine has no Float16Array.
const f32 = new Float32Array(1), u32 = new Uint32Array(f32.buffer);
export function toHalf(x: number): number {
  f32[0] = x;
  const b = u32[0], sign = (b >>> 16) & 0x8000;
  let e = ((b >>> 23) & 0xff) - 127 + 15, m = b & 0x7fffff;
  if (((b >>> 23) & 0xff) === 0xff) return sign | 0x7c00 | (m ? 0x200 : 0);
  if (e >= 31) return sign | 0x7c00;
  if (e <= 0) {
    if (e < -10) return sign;
    m = (m | 0x800000) >>> (1 - e);
    return sign | ((m + 0x1000) >>> 13);
  }
  return sign | (e << 10) | ((m + 0x1000) >>> 13);
}
export function fromHalf(h: number): number {
  const s = h & 0x8000 ? -1 : 1, e = (h >>> 10) & 0x1f, m = h & 0x3ff;
  if (e === 0) return s * Math.pow(2, -14) * (m / 1024);
  if (e === 31) return m ? NaN : s * Infinity;
  return s * Math.pow(2, e - 15) * (1 + m / 1024);
}

/** The vector stores this session holds, so sign-out can empty them (the in-memory copy serves the old account otherwise). */
const liveVectors = new Set<VectorStore>();
/** What else sign-out must reach in memory (the retrieval runtime's corpus and its running embedding pass, retrieval-run.ts). Run before the vectors go. */
const liveResets = new Set<() => void>();
export function onSignOut(f: () => void): () => void { liveResets.add(f); return () => { liveResets.delete(f); }; }
export async function wipeLiveVectors(storage: Storage | undefined): Promise<void> {
  for (const f of [...liveResets]) f();
  for (const v of [...liveVectors]) await wipeVectors(v, storage);
}

export class VectorStore {
  private ids: string[] = [];
  private keys: string[] = [];
  private vec: Float32Array[] = [];
  private slot = new Map<string, number>();
  dirty = false;
  constructor(readonly engine: string = ENGINE, readonly dimension: number = DIMENSION) { liveVectors.add(this); }

  get count(): number { return this.ids.length; }
  /** The content key each stored id was made from (what the job compares to decide what to embed again). */
  contentKeys(): Map<string, string> { return new Map(this.ids.map((id, i) => [id, this.keys[i]])); }
  wipe(): void { this.ids = []; this.keys = []; this.vec = []; this.slot.clear(); this.dirty = true; }

  upsert(id: string, key: string, vector: ArrayLike<number>): void {
    if (vector.length < this.dimension) return;
    // Stored as half precision: round-trip now so what is searched equals what a reopened store searches.
    const u = truncatedUnit(vector, this.dimension);
    for (let i = 0; i < u.length; i++) u[i] = fromHalf(toHalf(u[i]));
    const i = this.slot.get(id);
    if (i !== undefined) { this.vec[i] = u; this.keys[i] = key; }
    else { this.slot.set(id, this.ids.length); this.ids.push(id); this.keys.push(key); this.vec.push(u); }
    this.dirty = true;
  }
  remove(id: string): void {
    const i = this.slot.get(id);
    if (i === undefined) return;
    this.slot.delete(id);
    const last = this.ids.length - 1;
    if (i !== last) { this.vec[i] = this.vec[last]; this.ids[i] = this.ids[last]; this.keys[i] = this.keys[last]; this.slot.set(this.ids[i], i); }
    this.vec.pop(); this.ids.pop(); this.keys.pop();
    this.dirty = true;
  }
  removeAll(keep: Set<string>): number {
    let n = 0;
    for (const id of [...this.ids]) if (!keep.has(id)) { this.remove(id); n++; }
    return n;
  }

  /** The best `k` rows by cosine (stored vectors and the query are unit length, so a dot product), best first. */
  search(query: ArrayLike<number>, k = 50): { id: string; score: number }[] {
    const q = truncatedUnit(query, this.dimension);
    if (this.ids.length === 0 || q.length !== this.dimension) return [];
    const out: { id: string; score: number }[] = [];
    for (let r = 0; r < this.vec.length; r++) {
      const v = this.vec[r];
      let s = 0;
      for (let i = 0; i < this.dimension; i++) s += v[i] * q[i];
      out.push({ id: this.ids[r], score: s });
    }
    out.sort((a, b) => (a.score !== b.score ? b.score - a.score : a.id < b.id ? -1 : 1));
    return out.slice(0, k);
  }

  /** The rows as the two files kept on disk: the half-precision vectors, and the ids and keys. */
  serialize(): { vectors: Uint8Array; rows: string } {
    const bytes = new Uint8Array(this.vec.length * this.dimension * 2);
    const dv = new DataView(bytes.buffer);
    this.vec.forEach((v, r) => { for (let i = 0; i < this.dimension; i++) dv.setUint16((r * this.dimension + i) * 2, toHalf(v[i]), true); });
    return { vectors: bytes, rows: JSON.stringify({ engine: this.engine, dimension: this.dimension, ids: this.ids, keys: this.keys }) };
  }
  /** A store read back. A file made by another engine or dimension, or a corrupt one, is an empty store; rows the vector file lacks are dropped. */
  static parse(vectors: Uint8Array | null, rows: string | null, engine: string = ENGINE, dimension: number = DIMENSION): VectorStore {
    const s = new VectorStore(engine, dimension);
    if (!vectors || rows === null) return s;
    try {
      const m = JSON.parse(rows) as { engine?: unknown; dimension?: unknown; ids?: unknown; keys?: unknown };
      if (m.engine !== engine || m.dimension !== dimension || !Array.isArray(m.ids) || !Array.isArray(m.keys) || m.ids.length !== m.keys.length) return s;
      const have = Math.min(m.ids.length, Math.floor(vectors.length / 2 / dimension));
      const dv = new DataView(vectors.buffer, vectors.byteOffset, vectors.byteLength);
      for (let r = 0; r < have; r++) {
        const v = new Float32Array(dimension);
        for (let i = 0; i < dimension; i++) v[i] = fromHalf(dv.getUint16((r * dimension + i) * 2, true));
        s.slot.set(String(m.ids[r]), s.ids.length); s.ids.push(String(m.ids[r])); s.keys.push(String(m.keys[r])); s.vec.push(v);
      }
    } catch { return new VectorStore(engine, dimension); }
    return s;
  }
}

export const VECTORS_DIR = 'app:/data/vectors';
export async function loadVectors(storage: Storage | undefined, engine: string = ENGINE, dimension: number = DIMENSION): Promise<VectorStore> {
  if (!storage) return new VectorStore(engine, dimension);
  try {
    const v = new Uint8Array(await storage.fs.readFile(`${VECTORS_DIR}/vectors.f16`));
    const r = new TextDecoder().decode(await storage.fs.readFile(`${VECTORS_DIR}/rows.json`));
    return VectorStore.parse(v, r, engine, dimension);
  } catch { return new VectorStore(engine, dimension); }
}
/** Vectors first, then the ids, so a kill leaves a store that opens. A store with nothing changed writes nothing. */
export async function saveVectors(store: VectorStore, storage: Storage | undefined): Promise<void> {
  if (!storage || !store.dirty) return;
  const { vectors, rows } = store.serialize();
  try {
    await storage.fs.mkdir(VECTORS_DIR).catch(() => undefined);
    await storage.fs.atomicWriteFile(`${VECTORS_DIR}/vectors.f16`, vectors);
    await storage.fs.atomicWriteFile(`${VECTORS_DIR}/rows.json`, new TextEncoder().encode(rows));
    store.dirty = false;
  } catch { /* a full disk loses only the derived vectors; they are made again */ }
}
/** Sign-out: forget every vector, in memory and on disk. */
export async function wipeVectors(store: VectorStore, storage: Storage | undefined): Promise<void> {
  store.wipe(); store.dirty = false;
  if (!storage) return;
  try { await storage.fs.rm(`${VECTORS_DIR}/vectors.f16`); } catch { /* absent */ }
  try { await storage.fs.rm(`${VECTORS_DIR}/rows.json`); } catch { /* absent */ }
}

// ---------------------------------------------------------------------------------------------------------------- fusion

/** Reciprocal rank fusion (Cormack, Clarke and Buettcher, SIGIR 2009), k = 60, over ranked id lists, best first. */
export function rrf(lists: string[][], k = 60): string[] {
  const s = new Map<string, number>();
  for (const l of lists) l.forEach((id, i) => s.set(id, (s.get(id) ?? 0) + 1 / (k + i + 1)));
  return [...s].sort((a, b) => (a[1] !== b[1] ? b[1] - a[1] : a[0] < b[0] ? -1 : 1)).map((x) => x[0]);
}

export type Fusion = 'vectorsOnly' | 'rrf' | 'vectorsWithExactTerms';
/** The strategy the app runs, chosen by the numbers in the reference page section 0.6. There is no setting. */
export const SHIPPED: Fusion = 'vectorsWithExactTerms';

/** Ids best first, at most `limit`. With no vectors, the keyword list as it is. */
export function fuse(s: Fusion, vectors: string[], bm25: string[], exact: string[], limit: number): string[] {
  if (vectors.length === 0) return bm25.slice(0, limit);
  if (s === 'vectorsOnly') return vectors.slice(0, limit);
  if (s === 'rrf') return rrf([bm25, vectors]).slice(0, limit);
  const inVectors = new Set(vectors.slice(0, limit));
  const missing = exact.filter((x) => !inVectors.has(x)).slice(0, Math.floor(limit / 2));
  const out = vectors.slice(0, Math.max(0, limit - missing.length));
  const have = new Set(out);
  out.push(...missing.filter((x) => !have.has(x)));
  return out.slice(0, limit);
}

/** The item id a vector's id stands for (a long item is embedded as windows `<id>~w<n>`). */
export function itemID(vectorID: string): string {
  const i = vectorID.lastIndexOf('~w');
  return i >= 0 && /^\d+$/.test(vectorID.slice(i + 2)) ? vectorID.slice(0, i) : vectorID;
}

export const POOL = 50;
/** The share of items that must hold a vector before the vectors are used at all (until then: BM25 alone). */
export const MIN_COVERAGE = 0.9;

/**
 * The retriever the model's context is built from once vectors exist. The query's vector is made before the call (the one native call is
 * asynchronous), so `candidates` stays synchronous: pass `queryVector` from `embedQuery`, or null to use BM25 alone.
 */
export function fusedRetriever(index: Index, vectors: VectorStore, o: { queryVector: number[] | null; coverage: number; cosineFloor?: number }): Retriever & { line: () => string } {
  let line = '';
  const floor = o.cosineFloor ?? COSINE_FLOOR;
  const uses = vectors.count > 0 && o.coverage >= MIN_COVERAGE && o.queryVector !== null;
  return {
    candidates(q, limit) {
      const bm = index.candidates(q, POOL);
      if (!uses) { const out = bm.slice(0, limit); line = `AUTORADIO retrieve retrieved=${out.length} vec=0 bm25=${bm.length}`; return out; }
      const vecIDs: string[] = [];
      const seen = new Set<string>();
      for (const h of vectors.search(o.queryVector!, POOL)) {
        if (h.score < floor) continue;
        const id = itemID(h.id);
        if (!seen.has(id)) { seen.add(id); vecIDs.push(id); }
      }
      const exact = index.exactTermHits(q, limit);
      const ids = fuse(SHIPPED, vecIDs, bm.map((b) => b.id), exact, limit);
      const byID = new Map(bm.map((b) => [b.id, b]));
      const out: Item[] = [];
      ids.forEach((id, rank) => {
        const it = byID.get(id) ?? index.item(id);
        if (it) out.push({ ...it, score: 1 / (rank + 1) });
      });
      line = `AUTORADIO retrieve retrieved=${out.length} vec=${vecIDs.length} bm25=${bm.length}`;
      return out;
    },
    get count() { return index.count; },
    sources: () => index.sources(),
    idf: (t) => index.idf(t),
    line: () => line,
  };
}

/** The question as the model reads it: the title on air joins when the question points at it. Embedded with the query prompt. */
export async function embedQuery(embed: Embed, q: Query): Promise<number[] | null> {
  const { usedOnAir } = queryTerms(q);
  const text = usedOnAir && (q.onAirTitle ?? '') !== '' ? `${q.words} ${q.onAirTitle}` : q.words;
  try { const v = await embed([embedPrompt.query(text)]); return v[0] ?? null; } catch { return null; }
}

// ---------------------------------------------------------------------------------------------------------------- the embedding work list and job

export type WorkItem = { id: string; key: string; title: string | null; text: string };

/** A stable content key (two 32-bit FNV-1a hashes of the title and text, in hex). */
export function contentKey(title: string, text: string): string {
  let a = 0x811c9dc5, b = 0x01000193 ^ 0x9e3779b9;
  const bytes = new TextEncoder().encode(title + '\u001f' + text);
  for (const x of bytes) { a = Math.imul(a ^ x, 0x01000193) >>> 0; b = Math.imul(b ^ (x + 31), 0x85ebca6b) >>> 0; }
  return a.toString(16).padStart(8, '0') + b.toString(16).padStart(8, '0');
}

/** Every live item of the index, a long one as windows of about 180 words, newest first, undated last. */
export function workItems(index: Index): WorkItem[] {
  const out: { date: number | null; item: WorkItem }[] = [];
  for (const it of index.snapshot()) {
    if (it.title === '' && it.text === '') continue;
    const wins = passages(it.text, 180);
    if (wins.length <= 1) out.push({ date: it.date, item: { id: it.id, key: contentKey(it.title, it.text), title: it.title, text: it.text } });
    else wins.forEach((w, i) => out.push({ date: it.date, item: { id: `${it.id}~w${i + 1}`, key: contentKey(it.title, w), title: it.title, text: w } }));
  }
  out.sort((a, b) => {
    if (a.date !== null && b.date !== null) return a.date !== b.date ? b.date - a.date : a.item.id < b.item.id ? -1 : 1;
    if (a.date !== null) return -1;
    if (b.date !== null) return 1;
    return a.item.id < b.item.id ? -1 : 1;
  });
  return out.map((x) => x.item);
}

/** What the store lacks: the items with no vector or a changed key (derived, nothing is kept), and the ids that left. */
export function todo(items: WorkItem[], store: VectorStore): { todo: WorkItem[]; removed: string[]; skipped: number } {
  const have = store.contentKeys();
  const keep = new Set(items.map((i) => i.id));
  const removed = [...have.keys()].filter((id) => !keep.has(id));
  const need = items.filter((i) => have.get(i.id) !== i.key);
  return { todo: need, removed, skipped: items.length - need.length };
}

export type JobBudget = { passagesPerSlice: number; savesEverySlices: number; maxChars: number };
export const JOB_BUDGET: JobBudget = { passagesPerSlice: 4, savesEverySlices: 40, maxChars: 1800 };
export type Progress = { embedded: number; skipped: number; removed: number; waitedForSpeech: number; failed: number; finished: boolean; gaveUp?: boolean };
/** While the device is busy the wait doubles from the rest delay up to this, and the pass ends after this many busy answers in a row (about 16 minutes): it is started again by the next ask or sync, not polled for. */
export const BUSY_WAIT_MAX_MS = 30000;
export const BUSY_GIVE_UP = 40;

/**
 * Bring a store in line with the items. Embeds only what is new or changed, in small slices, calls `save` every `savesEverySlices` slices
 * and at the end, and waits while `busy` says the radio needs the processor (speech always wins; `rest` is the wait). A model that
 * is not there stops the job with `failed` set; it never starts by itself.
 */
export async function runJob(store: VectorStore, embed: Embed, items: WorkItem[], o: {
  busy?: () => boolean | Promise<boolean>; save?: () => Promise<void>; rest?: () => Promise<void>; cancelled?: () => boolean; budget?: JobBudget;
  /** The wait while busy, in ms (it grows); without it `rest` is used. `restMs` is the first wait. */
  wait?: (ms: number) => Promise<void>; restMs?: number;
} = {}): Promise<Progress> {
  const budget = o.budget ?? JOB_BUDGET;
  const p: Progress = { embedded: 0, skipped: 0, removed: 0, waitedForSpeech: 0, failed: 0, finished: false, gaveUp: false };
  const t = todo(items, store);
  for (const id of t.removed) store.remove(id);
  p.removed = t.removed.length; p.skipped = t.skipped;
  let i = 0, since = 0, busyRun = 0;
  while (i < t.todo.length && !(o.cancelled?.() ?? false)) {
    if (o.busy && (await o.busy())) {
      p.waitedForSpeech++; busyRun++;
      if (busyRun >= BUSY_GIVE_UP) { p.gaveUp = true; break; }
      if (o.wait) await o.wait(Math.min(BUSY_WAIT_MAX_MS, (o.restMs ?? 150) * 2 ** (busyRun - 1))); else await o.rest?.();
      continue;
    }
    busyRun = 0;
    const slice = t.todo.slice(i, i + budget.passagesPerSlice);
    try {
      const vs = await embed(slice.map((x) => embedPrompt.document(x.title, x.text.slice(0, budget.maxChars))));
      if (vs.length !== slice.length) throw new Error('the model answered a different number of vectors');
      slice.forEach((it, j) => { store.upsert(it.id, it.key, vs[j]); p.embedded++; });
    } catch { p.failed = t.todo.length - i; break; }
    i += slice.length; since++;
    if (since >= budget.savesEverySlices) { await o.save?.(); since = 0; }
    await o.rest?.();
  }
  await o.save?.();
  p.finished = i >= t.todo.length;
  return p;
}

/** Whether the vectors are used at all: the share of items that hold one. */
export function coverage(store: VectorStore, total: number): number { return total === 0 ? (store.count > 0 ? 1 : 0) : Math.min(1, store.count / total); }

// ---------------------------------------------------------------------------------------------------------------- the saved corpus

/** The groups of what is in front of the listener: "this" and "that" point at them, so they are always indexed (Index.upsert's `pin`). */
export const PINNED_GROUPS = new Set(['page', 'runsheet', 'open-pages', 'edition-topics']);

/** The held documents by group (a store's own: "store", "paper-front", "runsheet", ...). A group replaces its earlier documents. */
export class Corpus {
  readonly index = new Index();
  private groups = new Map<string, Set<string>>();
  /** When each paper story was published, by address, so cached full text (which carries no date) is ordered by recency too. */
  readonly storyDates = new Map<string, number>();

  hold(group: string, docs: Doc[]): void {
    const ids = new Set(docs.map((d) => d.id));
    for (const old of this.groups.get(group) ?? []) if (!ids.has(old)) this.index.remove(old);
    this.groups.set(group, ids);
    const pin = PINNED_GROUPS.has(group);
    for (const d of docs) this.index.upsert(d, pin);
  }
  /** The same as `hold`, in slices that hand the processor back between them (an answer turn has a CPU budget of its own: a cold build of a real store must not run in one go).
   * `live` says the account is still the one the documents were read for; once it is not, nothing more is indexed. */
  async holdSliced(group: string, docs: Doc[], live: () => boolean, slice = 40): Promise<void> {
    const ids = new Set(docs.map((d) => d.id));
    for (const old of this.groups.get(group) ?? []) if (!ids.has(old)) this.index.remove(old);
    this.groups.set(group, ids);
    for (let i = 0; i < docs.length; i += slice) {
      if (!live()) return;
      this.index.upsertAll(docs.slice(i, i + slice));
      if (i + slice < docs.length) await new Promise<void>((r) => setTimeout(r, 0));
    }
  }
  drop(group: string): void { this.hold(group, []); this.groups.delete(group); }
  /** Sign-out: everything held goes. */
  reset(): void { for (const g of [...this.groups.keys()]) this.drop(g); this.storyDates.clear(); }
  retriever(): Retriever { return bm25Retriever(this.index); }
  retrieve(q: Query): Context { return retrieve(this.retriever(), q); }
}

// ---------------------------------------------------------------------------------------------------------------- sources of documents

/** The front list the phone last cached (`app:/data/paper-front.json`, written by paper.ts): title, dek and date of the stories. No full text. */
export function paperFrontDocs(body: unknown, dates?: Map<string, number>): Doc[] {
  const list = body !== null && typeof body === 'object' ? (body as { stories?: unknown }).stories : null;
  if (!Array.isArray(list)) return [];
  const out: Doc[] = [];
  for (const o of list) {
    if (o === null || typeof o !== 'object') continue;
    const r = o as { title?: unknown; dek?: unknown; route?: unknown; updated_at?: unknown };
    const route = typeof r.route === 'string' ? r.route : r.route !== null && typeof r.route === 'object' ? String((r.route as { bare?: unknown }).bare ?? '') : '';
    const title = typeof r.title === 'string' ? r.title : '';
    if (route === '' || title === '') continue;
    const date = parseIso(r.updated_at);
    if (date !== null && dates) dates.set(`pub:${route}`, date);
    out.push({ id: `front:${route}`, kind: 'story', title, text: typeof r.dek === 'string' ? r.dek : '', date, source: 'Autopaper', address: `pub:${route}` });
  }
  return out;
}

function parseIso(s: unknown): number | null {
  if (typeof s !== 'string') return null;
  const m = /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2}):(\d{2})(?:\.(\d+))?(Z|[+-]\d{2}:?\d{2})$/.exec(s.trim());
  if (!m) return null;
  let t = Date.UTC(+m[1], +m[2] - 1, +m[3], +m[4], +m[5], +m[6], m[7] ? parseInt((m[7] + '00').slice(0, 3), 10) : 0);
  if (m[8] !== 'Z') { const sign = m[8][0] === '-' ? -1 : 1; const d = m[8].slice(1).replace(':', ''); t -= sign * (parseInt(d.slice(0, 2), 10) * 60 + parseInt(d.slice(2), 10)) * 60000; }
  return Number.isFinite(t) ? t : null;
}

/** The cached front, read from the phone: the corpus source "paper-front". A missing or corrupt file is no documents, never a throw. */
export const PAPER_FRONT_FILE = 'app:/data/paper-front.json';
export async function loadPaperFront(storage: Storage | undefined, dates?: Map<string, number>): Promise<Doc[]> {
  if (!storage) return [];
  try { return paperFrontDocs(JSON.parse(new TextDecoder().decode(await storage.fs.readFile(PAPER_FRONT_FILE))), dates); } catch { return []; }
}

// (There is no mail document source: mail text in the vector store would sit unsealed in app:/data/vectors. Mail stays out of local search until a search over sealed mail is designed.)
/** A folder's file names (never their contents): searchable by name, openable by path. */
export const fileNameDocs = (entries: { path: string; name: string; isDirectory: boolean; modified: number | null }[]): Doc[] =>
  entries.filter((e) => !e.isDirectory).map((e) => ({ id: `file:${e.path}`, kind: 'file', title: e.name, text: e.path, date: e.modified, source: 'Files', address: `file:${e.path}` }));
export const editionTopicDocs = (topics: string[]): Doc[] =>
  topics.length === 0 ? [] : [{ id: 'edition-topics', kind: 'note', title: 'My edition topics', text: `The reader follows these topics: ${topics.join(', ')}`, date: null, source: 'Autopaper', address: 'paper:mine' }];
export const openPageDocs = (pages: { address: string; title: string; kind: string }[]): Doc[] =>
  pages.filter((p) => p.title !== '').map((p) => ({ id: `openpage:${p.address}`, kind: 'page', title: p.title, text: `An open ${p.kind} page.`, date: null, source: 'Open pages', address: p.address }));

// ---------------------------------------------------------------------------------------------------------------- the paper-wide search route

export const CORPUS_PATH = '/api/platform/retrieval/search';
/** One space between words, no edges ("nvidia " and "nvidia" are the same query). */
export const normalizeQuery = (q: string): string => q.split(/\s+/).filter((x) => x !== '').join(' ');
const oneLine = (s: string): string => s.split(/\s+/).filter((x) => x !== '').join(' ');

export type CorpusHit = { route: string; title: string; snippet: string; address: string };

/** The answer of the route: hits in the server's order, without a hit that has no route or title or repeats a route. */
export function decodeCorpus(body: unknown): CorpusHit[] {
  if (body === null || typeof body !== 'object' || Array.isArray(body)) throw new Error('not a JSON object');
  const results = (body as { results?: unknown }).results;
  if (results === undefined || results === null) return [];
  if (!Array.isArray(results)) throw new Error('results is not a list');
  const seen = new Set<string>(), out: CorpusHit[] = [];
  for (const r of results) {
    if (r === null || typeof r !== 'object') continue;
    const o = r as { route_path?: unknown; title?: unknown; snippet?: unknown };
    const route = typeof o.route_path === 'string' ? o.route_path : '';
    const title = oneLine(typeof o.title === 'string' ? o.title : '');
    if (!route.startsWith('/') || title === '' || seen.has(route)) continue;
    seen.add(route);
    out.push({ route, title, snippet: corpusSnippet(typeof o.snippet === 'string' ? o.snippet : '', title), address: `pub:${route}` });
  }
  return out;
}

/** A few words of the story around the match: one line, markdown marks dropped, 140 characters; the story's own heading is not repeated. */
export function corpusSnippet(s: string, title: string): string {
  let t = oneLine(s.replace(/#/g, '').replace(/\*/g, ''));
  while (t.startsWith('.')) t = t.slice(1);
  t = t.trim();
  if (t.toLowerCase().startsWith(title.toLowerCase())) t = t.slice(title.length).trim();
  return t.length > 140 ? t.slice(0, 139) + '…' : t;
}

export type Row = { address: string; title: string; kind: string; detail: string };
/** The hits as launcher rows: a story each, after `local` rows, none repeated by address. */
export function corpusRows(hits: CorpusHit[], local: Row[]): Row[] {
  const seen = new Set(local.map((r) => r.address));
  const out = [...local];
  for (const h of hits) if (!seen.has(h.address)) { seen.add(h.address); out.push({ address: h.address, title: h.title, kind: 'story', detail: h.snippet }); }
  return out;
}

export type CorpusStatus = { state: 'idle' | 'done' | 'failed'; hits: CorpusHit[] };
/** The paper-wide search: one public GET (no sign-in), never throws; a failure is `failed`, and the local hits stay. */
export async function searchPaper(query: string, get: (path: string, query: Record<string, string>) => Promise<unknown>): Promise<CorpusStatus> {
  const q = normalizeQuery(query);
  if (q === '') return { state: 'idle', hits: [] };
  try { return { state: 'done', hits: decodeCorpus(await get(CORPUS_PATH, { q })) }; } catch { return { state: 'failed', hits: [] }; }
}
/** The note under the hits: empty when there is nothing to say. */
export function corpusNote(s: CorpusStatus, hasHits: boolean): string {
  if (s.state === 'failed') return 'Could not search the paper right now. Check your connection and try again.';
  if (s.state === 'done') return s.hits.length === 0 && !hasHits ? 'Nothing found in the paper.' : '';
  return '';
}
