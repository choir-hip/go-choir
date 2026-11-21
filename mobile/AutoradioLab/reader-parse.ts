// The reader's document model, in shared code (it was Swift: TextureDoc.swift, Citations.swift, TextureMarkdown.swift). The server's document shapes (a published bundle's `artifact.body_doc`, a private
// revision's `body_doc`, or plain Markdown text) become flat blocks of styled runs the page draws: no recursion in the model, because a Contract shape cannot nest itself. A list item is a block with a
// depth and a marker; a block quote is a block with a quote depth; a table is a block of rows of cells. Citations are runs that carry an anchor. Pure functions: no network, no storage.
// Rewritten from what the page must show; the committed golden documents (internal/texturedoc/golden) are the oracle for real shapes (reader-parse.test.ts P13).
//
// Ways this can fail (the tests name each):
//  1. body_doc missing, empty, not JSON or an unknown schema: the Markdown text (`content`, `payload.text`); with neither, a ready document with a visible "no text yet" block.
//  2. An unknown block or inline node (a newer schema): its text is kept; text is never dropped.   3. A source_ref with no entity or a duplicate id: it still draws a numbered mark, its card says
//  "Source unavailable"; ids are made unique.   4. Empty text nodes, whitespace-only paragraphs, a hard_break at either end: no empty runs, no empty blocks.   5. Nested lists, ordered lists with a start.
//  6. Quotes with several paragraphs or nested quotes.   7. Code keeps its newlines.   8. A very long document: one pass; a paragraph over 20 000 characters is not sentence-split.
//  9. Sentence indices come from the plain text without marks.   10. Tables with ragged rows; a Markdown table without a separator line is prose.   11. Unmatched `*`, snake_case, `5 * 3` are literal.
//  12. Versions: a revision above latest is clamped; "newer" only when latest > revision; a corrupt count is capped at 500 rows.   13. A first heading equal to the title is dropped.
import type { Result } from './app.contract.d.ts';
import { shortDate } from './textures.ts';

export type ReaderDoc = Result<'readerDocShared'>;
export type Block = ReaderDoc['blocks'][number];
export type Run = Block['runs'][number];
export type Cite = ReaderDoc['cites'][number];
type Obj = Record<string, unknown>;
const obj = (v: unknown): v is Obj => typeof v === 'object' && v !== null && !Array.isArray(v);
const str = (v: unknown): string => (typeof v === 'string' ? v : '');
const arr = (v: unknown): Obj[] => (Array.isArray(v) ? v.filter(obj) : []);
const trim = (s: string): string => s.replace(/^[\s ]+|[\s ]+$/g, '');
export function intOf(v: unknown): number | null {
  if (typeof v === 'number' && Number.isFinite(v)) return Math.trunc(v);
  if (typeof v === 'string' && /^[+-]?\d+$/.test(v.trim())) return parseInt(v, 10);
  return null;
}
/** Letters and digits, for the flanking rules. Hermes has no guaranteed \p classes: cased letters, digits and the CJK blocks. */
const wordCh = (c: string): boolean => c !== undefined && (c.toLowerCase() !== c.toUpperCase() || /[0-9぀-ヿ㐀-鿿가-힯]/.test(c));

export const MAX_VERSIONS = 500;
const MAX_SENTENCE_CHARS = 20000;
export const DISCLOSURE_STORY = 'Written by AI. Autopaper’s agent desks wrote this story from the sources it links; no human editor reviewed it before publication.';
export const NOTICE_OPINION = 'Opinion. Written and published by a reader; Autopaper’s agent desks did not write it and no editor reviewed it.';

// ---------------------------------------------------------------------------------------------------------------------------------------------------
// Shapes

export function blankDoc(address: string, status = 'loading', message = ''): ReaderDoc {
  return {
    address, status, message, title: '', kicker: '', dek: '', byline: '', meta: '', kind: 'document', disclosure: '', blocks: [], cites: [], sources: [], versions: [], related: [],
    revision: 0, latest: 0, newer: false, historical: false, revising: false, revisingNote: '', trajectory: '', permalink: '', plain: '', textBlocks: 0, markdown: '', editable: false, source: '', docId: '',
  };
}
export const run = (id: string, text: string, o: Partial<Run> = {}): Run => ({ id, text, kind: 't', weight: 400, slant: 'normal', anchor: '', label: '', sent: 0, href: '', ...o });
export const block = (id: string, kind: string, o: Partial<Block> = {}): Block => ({ id, kind, level: 0, depth: 0, marker: '', runs: [], rows: [], text: '', ordinal: -1, cites: [], sentences: 0, ...o });

// ---------------------------------------------------------------------------------------------------------------------------------------------------
// Addresses

export type Address = { kind: 'pub' | 'texture' | 'none'; route: string; doc: string; rev: string };
export function parseAddress(address: string): Address {
  const none: Address = { kind: 'none', route: '', doc: '', rev: '' };
  if (address.startsWith('pub:')) { const r = address.slice(4); return r === '' ? none : { kind: 'pub', route: r, doc: '', rev: '' }; }
  if (address.startsWith('texture:')) {
    const rest = address.slice(8), at = rest.indexOf('@');
    const doc = at < 0 ? rest : rest.slice(0, at);
    return doc === '' ? none : { kind: 'texture', route: '', doc, rev: at < 0 ? '' : rest.slice(at + 1) };
  }
  return none;
}
/** "slug/v3" -> {slug, 3}; a bare route -> {route, 0}. */
export function splitRoute(route: string): { bare: string; version: number } {
  const i = route.lastIndexOf('/v');
  if (i < 0) return { bare: route, version: 0 };
  const tail = route.slice(i + 2);
  return /^\d+$/.test(tail) && parseInt(tail, 10) > 0 ? { bare: route.slice(0, i), version: parseInt(tail, 10) } : { bare: route, version: 0 };
}

// ---------------------------------------------------------------------------------------------------------------------------------------------------
// Citations

/** "https://www.Example.com:8080/a?b" -> "example.com"; "" when there is no host. */
export function hostOf(raw: string): string {
  let s = trim(raw);
  if (s === '') return '';
  if (!s.includes('://')) s = `https://${s}`;
  const m = /^[A-Za-z][A-Za-z0-9+.-]*:\/\/([^/?#]*)/.exec(s);
  if (!m) return '';
  let auth = m[1];
  const at = auth.lastIndexOf('@'); if (at >= 0) auth = auth.slice(at + 1);
  auth = auth.replace(/:\d*$/, '');
  let h = auth.toLowerCase();
  if (h === '' || /[\s<>"{}|\\^`[\]]/.test(h)) return '';
  if (h.startsWith('www.')) h = h.slice(4);
  return h.includes('.') || h === 'localhost' ? h : '';
}
export function checkLabel(state: string): { state: string; label: string } {
  switch (trim(state).toLowerCase()) {
    case 'exact': return { state: 'exact', label: 'Exact quote' };
    case 'tolerant': return { state: 'tolerant', label: 'Close match' };
    case 'failed': return { state: 'failed', label: 'Quote not found' };
    case 'unread': return { state: 'unread', label: 'Source not read' };
    default: return { state: 'pending', label: 'Not yet checked' };
  }
}
const isTextureKind = (kind: string, uri: string): boolean => ['texture_span', 'publication_span', 'texture', 'texture_doc'].includes(kind) || uri.startsWith('texture:');
/** A ref given as an object or as JSON text (the web's parseJSON of attrs.ref). */
function refObject(v: unknown): Obj | null {
  if (obj(v)) return v;
  if (typeof v === 'string') { try { const j = JSON.parse(v); return obj(j) ? j : null; } catch { return null; } }
  return null;
}
/** The server's check carries `quote` = the outcome word, not the quoted words. */
const OUTCOMES = new Set(['exact', 'tolerant', 'failed', 'unread', 'pending']);
const firstNonEmpty = (vals: unknown[], fallback: string): string => { for (const v of vals) if (typeof v === 'string' && trim(v) !== '') return trim(v); return fallback; };

/** Lower-cased, curly quotes straightened, runs of white space collapsed. `map[u]` is the original character index of the UTF-16 unit u of `text`. */
function normalised(s: string): { text: string; map: number[] } {
  let out = ''; const map: number[] = []; let lastSpace = true;
  Array.from(s).forEach((ch, i) => {
    let c = ch;
    if (c === '‘' || c === '’') c = "'"; else if (c === '“' || c === '”') c = '"'; else if (c === '–' || c === '—') c = '-';
    if (/^[\s ]$/.test(c)) { if (lastSpace) return; out += ' '; map.push(i); lastSpace = true; }
    else { const l = c.toLowerCase(); out += l; for (let k = 0; k < l.length; k++) map.push(i); lastSpace = false; }
  });
  while (out.endsWith(' ')) { out = out.slice(0, -1); map.pop(); }
  return { text: out, map };
}
/** Split `block` into before, hit and after around `quote`, matching through a normalisation and cutting from the original text. An ellipsis quote highlights from its first segment to its last. */
export function contextOf(quote: string, block: string): { before: string; hit: string; after: string } {
  const miss = { before: block, hit: '', after: '' };
  const q = trim(quote);
  if (q === '' || block === '') return miss;
  const segs = q.split('…').join('...').split('...').map(trim).filter((x) => x !== '');
  if (segs.length === 0) return miss;
  const { text: norm, map } = normalised(block);
  const nFirst = normalised(segs[0]).text, nLast = normalised(segs[segs.length - 1]).text;
  if (nFirst === '') return miss;
  const startN = norm.indexOf(nFirst);
  if (startN < 0) return miss;
  let endN = startN + nFirst.length;
  if (segs.length > 1 && nLast !== '') { const j = norm.indexOf(nLast, endN); if (j >= 0) endN = j + nLast.length; }
  if (!(startN < map.length && endN > startN && endN <= map.length)) return miss;
  const chars = Array.from(block);
  const s = map[startN], e = map[endN - 1] + 1;
  if (!(s < e && e <= chars.length)) return miss;
  return { before: chars.slice(0, s).join(''), hit: chars.slice(s, e).join(''), after: chars.slice(e).join('') };
}

type Ctx = { entities: Map<string, Obj>; transclusions: Map<string, Obj>; resolved: Map<string, Obj>; checks: Map<string, Obj>; manifestAnchors: Map<string, Obj> };
const newCtx = (): Ctx => ({ entities: new Map(), transclusions: new Map(), resolved: new Map(), checks: new Map(), manifestAnchors: new Map() });

function makeCite(anchor: string, number: number, blockId: string, entityId: string, ctx: Ctx, nodeRef: Obj | null): Cite {
  const entity = ctx.entities.get(entityId);
  const target = obj(entity?.target) ? entity!.target as Obj : {}; const display = obj(entity?.display) ? entity!.display as Obj : {};
  const kindRaw = str(target.kind), uri = str(target.uri), tid = str(target.id);
  const c: Cite = { anchor, number, blockId, entityId, title: '', host: '', url: '', kind: 'unknown', target: '', quote: '', before: '', hit: '', after: '', check: 'pending', checkLabel: 'Not yet checked' };
  if (!entity) { c.title = 'Source unavailable'; c.kind = 'unknown'; }
  else if (isTextureKind(kindRaw, uri)) {
    c.kind = 'texture';
    const raw = uri.startsWith('texture:') ? uri.slice(8) : tid;
    c.target = raw === '' ? '' : `texture:${raw}`;
    c.title = firstNonEmpty([display.title, display.label], 'A Texture');
  } else {
    c.kind = 'web'; c.url = uri; c.host = hostOf(uri);
    c.title = firstNonEmpty([display.title, display.label], c.host === '' ? 'Source' : c.host);
  }
  const tr = ctx.transclusions.get(anchor), ck = ctx.checks.get(anchor);
  const manifestRef = refObject(ctx.manifestAnchors.get(anchor)?.ref);
  const ref = (obj(tr?.ref) ? tr!.ref as Obj : null) ?? manifestRef ?? nodeRef ?? (obj(ck?.ref) ? ck!.ref as Obj : null) ?? {};
  const ckq = typeof ck?.quote === 'string' && !OUTCOMES.has(trim(ck.quote).toLowerCase()) ? ck.quote : null;
  c.quote = firstNonEmpty([tr?.snapshot_text, ref.quote, manifestRef?.quote, nodeRef?.quote, ckq], '');
  const cl = checkLabel(str(ck?.state)); c.check = cl.state; c.checkLabel = cl.label;
  const cid = str(ref.cid);
  const blk = cid === '' ? undefined : ctx.resolved.get(cid);
  if (blk) {
    const text = str(blk.text);
    if (c.url === '' && typeof blk.url === 'string' && c.kind === 'web') { c.url = blk.url; c.host = hostOf(blk.url); }
    if ((c.title === c.host || c.title === 'Source') && typeof blk.title === 'string' && trim(blk.title) !== '') c.title = trim(blk.title);
    const parts = contextOf(c.quote, text); c.before = parts.before; c.hit = parts.hit; c.after = parts.after;
  }
  return c;
}
/** One row per distinct source, numbered in reading order. */
export function sourcesOf(cites: Cite[]): ReaderDoc['sources'] {
  const order: string[] = []; const by = new Map<string, ReaderDoc['sources'][number]>();
  for (const c of cites) {
    const key = c.entityId === '' ? `anon-${c.anchor}` : c.entityId;
    const s = by.get(key);
    if (s) { if (c.quote !== '' && !s.quotes.includes(c.quote)) s.quotes.push(c.quote); }
    else { order.push(key); by.set(key, { number: c.number, entityId: c.entityId, title: c.title, host: c.host, url: c.url, kind: c.kind, target: c.target, quotes: c.quote === '' ? [] : [c.quote], anchor: c.anchor }); }
  }
  return order.map((k) => by.get(k)!).sort((a, b) => a.number - b.number);
}

// ---------------------------------------------------------------------------------------------------------------------------------------------------
// Sentences. Hermes has no Intl.Segmenter, so a sentence ends at . ! ? or an ellipsis (with closing quotes or brackets) followed by white space and a capital, digit, quote or end; a few
// abbreviations ("Dr.", "U.S.") do not end one. A wrong split only moves a highlight.

const ABBREV = new Set(['dr', 'mr', 'mrs', 'ms', 'prof', 'sr', 'jr', 'st', 'vs', 'etc', 'inc', 'ltd', 'no', 'u.s', 'u.k', 'e.g', 'i.e']);
/** UTF-16 end offsets of each sentence, the white space after it included. */
export function sentenceEnds(plain: string): number[] {
  const ends: number[] = []; const n = plain.length;
  let i = 0;
  while (i < n) {
    const c = plain[i];
    if (c === '\n' ) { let j = i; while (j < n && /\s/.test(plain[j])) j++; ends.push(j); i = j; continue; }
    if (c === '.' || c === '!' || c === '?' || c === '…') {
      let j = i + 1;
      while (j < n && '.!?…'.includes(plain[j])) j++;
      while (j < n && '"\'”’)]'.includes(plain[j])) j++;
      if (j >= n) { i = j; continue; }
      if (!/\s/.test(plain[j])) { i = j; continue; }
      let k = j; while (k < n && /\s/.test(plain[k]) && plain[k] !== '\n') k++;
      const next = plain[k];
      const word = /([A-Za-z.]+)$/.exec(plain.slice(0, i))?.[1]?.toLowerCase() ?? '';
      if (c === '.' && j === i + 1 && (ABBREV.has(word) || (word.length === 1 && /[a-z]/.test(word)))) { i = j; continue; }
      if (next !== undefined && next !== '\n' && /[a-z]/.test(next) && c === '.') { i = j; continue; }
      ends.push(k); i = k; continue;
    }
    i++;
  }
  if (ends.length === 0 || ends[ends.length - 1] < n) ends.push(n);
  return ends;
}
/** Splits runs at sentence boundaries of the plain text and numbers each piece with its sentence. Returns the count. */
export function assignSentences(runs: Run[], plain: string): number {
  if (plain === '' || plain.length > MAX_SENTENCE_CHARS) { runs.forEach((r, i) => { r.sent = 0; r.id = `x${i}`; }); return plain === '' ? 0 : 1; }
  const ends = sentenceEnds(plain);
  const out: Run[] = []; let offset = 0, sent = 0;
  for (const r of runs) {
    if (r.kind === 'cite' || r.kind === 'ref') { out.push({ ...r, sent: Math.min(sent, ends.length - 1) }); continue; }
    const len = r.text.length;
    if (len === 0) continue;
    let consumed = 0;
    while (consumed < len) {
      while (sent < ends.length - 1 && offset >= ends[sent]) sent++;
      const remain = ends[sent] - offset;
      const take = remain <= 0 || sent === ends.length - 1 ? len - consumed : Math.min(len - consumed, remain);
      out.push({ ...r, text: r.text.slice(consumed, consumed + take), sent });
      consumed += take; offset += take;
    }
  }
  for (let i = 0; i < out.length; i++) {
    if (out[i].kind === 'cite' || out[i].kind === 'ref') {
      for (let j = i - 1; j >= 0; j--) if (out[j].kind !== 'cite' && out[j].kind !== 'ref') { out[i].sent = out[j].sent; break; }
    }
  }
  out.forEach((r, i) => { r.id = `x${i}`; });
  runs.length = 0; runs.push(...out);
  return ends.length;
}
/** The text of each sentence of a block (what the radio's words are matched against). */
export function sentenceTexts(b: Block): string[] {
  const out: string[] = [];
  for (const r of b.runs) if (r.kind !== 'cite' && r.kind !== 'ref') { while (out.length <= r.sent) out.push(''); out[r.sent] += r.kind === 'br' ? '\n' : r.text; }
  return out.map(trim);      // index i is the sentence the runs numbered i say: the one splitter's numbering, which native matches against and hands back
}

// ---------------------------------------------------------------------------------------------------------------------------------------------------
// Markdown (a document with no body_doc: a legacy revision, a story's text). Conservative: anything it does not recognise is literal text.

export type MdBlock = { kind: string; text: string; level: number; depth: number; marker: string; rows: string[][]; header: boolean };
const mdb = (kind: string, o: Partial<MdBlock> = {}): MdBlock => ({ kind, text: '', level: 0, depth: 0, marker: '', rows: [], header: false, ...o });
const trimST = (s: string): string => s.replace(/^[ \t]+|[ \t]+$/g, '');
function isRule(l: string): boolean { const t = l.split(' ').join(''); return t.length >= 3 && (t[0] === '-' || t[0] === '*' || t[0] === '_') && [...t].every((c) => c === t[0]); }
function heading(l: string): { level: number; text: string } | null {
  let n = 0; while (n < l.length && l[n] === '#') n++;
  if (n < 1 || n > 6 || l.length <= n || l[n] !== ' ') return null;
  return { level: n, text: trim(trim(l.slice(n)).replace(/^#+|#+$/g, '')) };
}
const isSeparator = (l: string): boolean => { const t = trim(l); return t.includes('-') && (t.includes('|') || t.startsWith(':')) && /^[|\-: ]+$/.test(t); };
function cells(l: string): string[] { let t = trim(l); if (t.startsWith('|')) t = t.slice(1); if (t.endsWith('|')) t = t.slice(0, -1); return t.split('|').map(trim); }
function listItem(raw: string): { text: string; depth: number; marker: string } | null {
  let indent = 0; for (const c of raw) { if (c === ' ') indent += 1; else if (c === '\t') indent += 4; else break; }
  const t = trimST(raw), depth = 1 + Math.floor(indent / 2);
  if (t.length >= 2 && '-*+'.includes(t[0]) && t[1] === ' ' && !isRule(t)) return { text: trim(t.slice(2)), depth, marker: depth % 2 === 1 ? '•' : '–' };
  const m = /^(\d{1,4})\. /.exec(t);
  return m ? { text: trim(t.slice(m[0].length)), depth, marker: `${m[1]}.` } : null;
}
export function mdBlocks(src: string): MdBlock[] {
  const lines = src.split('\r\n').join('\n').split('\n');
  const out: MdBlock[] = []; let para: string[] = [];
  const flush = (): void => { if (para.length > 0) { out.push(mdb('p', { text: para.join(' ') })); para = []; } };
  let i = 0;
  while (i < lines.length) {
    const raw = lines[i], line = trimST(raw);
    if (line.startsWith('```')) {
      flush(); const code: string[] = []; i++;
      while (i < lines.length && !trimST(lines[i]).startsWith('```')) { code.push(lines[i]); i++; }
      out.push(mdb('code', { text: code.join('\n') })); i++; continue;
    }
    if (line === '') { flush(); i++; continue; }
    if (isRule(line)) { flush(); out.push(mdb('rule')); i++; continue; }
    const h = heading(line); if (h) { flush(); out.push(mdb('h', { text: h.text, level: h.level })); i++; continue; }
    if (line.includes('|') && i + 1 < lines.length && isSeparator(lines[i + 1]) && cells(line).length > 1) {
      flush(); const rows = [cells(line)]; i += 2;
      while (i < lines.length && lines[i].includes('|') && trim(lines[i]) !== '') { rows.push(cells(lines[i])); i++; }
      out.push(mdb('table', { rows, header: true })); continue;
    }
    if (line.startsWith('>')) {
      flush(); let depth = 0, rest = line;
      while (rest.startsWith('>')) { depth++; rest = rest.slice(1).replace(/^ +/, ''); }
      if (rest !== '') out.push(mdb('quote', { text: rest, depth }));
      i++; continue;
    }
    const li = listItem(raw); if (li) { flush(); out.push(mdb('li', li)); i++; continue; }
    para.push(line); i++;
  }
  flush();
  const merged: MdBlock[] = [];
  for (const b of out) { const l = merged[merged.length - 1]; if (b.kind === 'quote' && l && l.kind === 'quote' && l.depth === b.depth) l.text += ` ${b.text}`; else merged.push(b); }
  return merged;
}
/** Strong, emphasis, code and links; every unmatched marker is literal. */
export function mdInline(s: string, idPrefix: string): Run[] {
  const runs: Run[] = [];
  const add = (text: string, weight: number, slant: string, kind = 't', href = ''): void => { if (text !== '') runs.push(run(`${idPrefix}-${runs.length}`, text, { kind, weight, slant, href })); };
  const find = (a: string[], c: string, from: number): number => { for (let i = from; i < a.length; i++) if (a[i] === c) return i; return -1; };
  const findPair = (a: string[], p: string, from: number): number => { for (let i = from; i + p.length <= a.length; i++) if (a.slice(i, i + p.length).join('') === p) return i; return -1; };
  const parse = (text: string, weight: number, slant: string): void => {
    const chars = Array.from(text); let buf = ''; let i = 0;
    const flush = (): void => { add(buf, weight, slant); buf = ''; };
    const before = (idx: number, c: string): boolean => idx === 0 ? true : c === '*' ? true : !wordCh(chars[idx - 1]);
    const after = (j: number, c: string): boolean => j + 1 >= chars.length ? true : c === '*' ? true : !wordCh(chars[j + 1]);
    while (i < chars.length) {
      const c = chars[i];
      if (c === '`') { const j = find(chars, '`', i + 1); if (j > i + 1) { flush(); add(chars.slice(i + 1, j).join(''), weight, slant, 'code'); i = j + 1; continue; } }
      if (c === '*' && chars[i + 1] === '*') { const j = findPair(chars, '**', i + 2); if (j > i + 2) { flush(); parse(chars.slice(i + 2, j).join(''), 700, slant); i = j + 2; continue; } }
      if ((c === '*' || c === '_') && i + 1 < chars.length && !/\s/.test(chars[i + 1]) && before(i, c)) {
        const j = find(chars, c, i + 1);
        if (j > i + 1 && !/\s/.test(chars[j - 1]) && after(j, c)) { flush(); parse(chars.slice(i + 1, j).join(''), weight, 'italic'); i = j + 1; continue; }
      }
      if (c === '[') {
        const close = find(chars, ']', i + 1);
        if (close >= 0 && close + 1 < chars.length && chars[close + 1] === '(') {
          const end = find(chars, ')', close + 2);
          if (end >= 0) {
            const label = chars.slice(i + 1, close).join(''), href = chars.slice(close + 2, end).join('');
            if (label !== '' && href !== '' && !href.includes(' ')) { flush(); add(label, weight, slant, 'link', href); i = end + 1; continue; }
          }
        }
      }
      buf += c; i++;
    }
    flush();
  };
  parse(s, 400, 'normal');
  return runs;
}

// ---------------------------------------------------------------------------------------------------------------------------------------------------
// The builder: body_doc nodes to blocks.

class Builder {
  blocks: Block[] = []; cites: Cite[] = []; counter = 0; currentBlock = '';
  private entityNumber = new Map<string, number>(); private used = new Set<string>();
  constructor(readonly ctx: Ctx) {}
  nextId(p: string): string { this.counter += 1; return `${p}${this.counter}`; }

  block(n: Obj, listDepth: number, quoteDepth: number, marker = ''): void {
    const type = str(n.type), content = arr(n.content), attrs = obj(n.attrs) ? n.attrs : {};
    switch (type) {
      case 'paragraph': case 'heading': {
        const level = type === 'heading' ? Math.max(1, Math.min(6, intOf(attrs.level) ?? 2)) : 0;
        const kind = type === 'heading' ? 'h' : quoteDepth > 0 ? 'quote' : listDepth > 0 ? 'li' : 'p';
        this.emit(kind, level, kind === 'li' ? listDepth : quoteDepth, marker, content);
        return;
      }
      case 'bullet_list': case 'ordered_list': {
        let number = Math.max(1, intOf(attrs.start) ?? 1);
        for (const item of content) {
          const ordered = type === 'ordered_list';
          const m = ordered ? `${number}.` : listDepth % 2 === 0 ? '•' : '–';
          number += 1; let first = true;
          for (const child of arr(item.content)) {
            const ct = str(child.type);
            if (ct === 'bullet_list' || ct === 'ordered_list') this.block(child, listDepth + 1, quoteDepth);
            else if (ct === 'paragraph') { this.emit('li', 0, listDepth + 1, first ? m : '', arr(child.content)); first = false; }
            else { this.block(child, listDepth + 1, quoteDepth, first ? m : ''); first = false; }
          }
        }
        return;
      }
      case 'blockquote':
        for (const child of content) {
          const ct = str(child.type);
          if (ct === 'paragraph' || ct === 'heading') this.emit(ct === 'heading' ? 'h' : 'quote', ct === 'heading' ? Math.max(1, Math.min(6, intOf((obj(child.attrs) ? child.attrs : {}).level) ?? 3)) : 0, quoteDepth + 1, '', arr(child.content));
          else this.block(child, listDepth, quoteDepth + 1);
        }
        return;
      case 'code_block': {
        const text = content.map((c) => c.text).filter((t): t is string => typeof t === 'string').join('\n');
        if (text !== '') this.blocks.push(block(this.nextId('b'), 'code', { text }));
        return;
      }
      case 'horizontal_rule': this.blocks.push(block(this.nextId('b'), 'rule')); return;
      case 'table': {
        const rows: Block['rows'] = [];
        content.forEach((r, ri) => {
          const cells: Block['rows'][number]['cells'] = [];
          for (const c of arr(r.content)) {
            let inl = arr(c.content);
            if (str(inl[0]?.type) === 'paragraph') inl = inl.flatMap((x) => arr(x.content));
            cells.push({ runs: this.inline(inl, true).runs, header: str(c.type) === 'table_header' || str(c.type) === 'th' });
          }
          if (cells.length > 0) rows.push({ id: `r${ri}`, cells });
        });
        if (rows.length > 0) this.blocks.push(block(this.nextId('b'), 'table', { rows }));
        return;
      }
      case 'embed': {
        // The label the web draws: the caption, else the title (file, audio), else the alt text. The block carries no alt field, so the alt is the label only when there is no caption or title.
        const ne = (k: string): string | null => { const v = trim(str(attrs[k])); return v === '' ? null : v; };
        const label = ne('caption') ?? ne('title') ?? ne('alt') ?? ne('kind') ?? 'Embedded item';
        this.blocks.push(block(this.nextId('b'), 'embed', { text: label, marker: str(attrs.kind) }));
        return;
      }
      default:
        // An unknown block type (a newer schema): keep its text, never drop it.
        if (content.length > 0) {
          if (content.some((c) => str(c.type) === 'text' || str(c.type) === 'source_ref')) this.emit('p', 0, 0, '', content);
          else for (const c of content) this.block(c, listDepth, quoteDepth);
        } else if (typeof n.text === 'string' && trim(n.text) !== '') {
          this.blocks.push(block(this.nextId('b'), 'p', { runs: [run(`u${this.counter}`, n.text)], text: trim(n.text) }));
        }
    }
  }

  emit(kind: string, level: number, depth: number, marker: string, inl: Obj[]): void {
    const id = this.nextId('b'); this.currentBlock = id;
    const { runs, plain } = this.inline(inl, false);
    while (runs.length > 0 && runs[0].kind === 'br') runs.shift();
    while (runs.length > 0 && runs[runs.length - 1].kind === 'br') runs.pop();
    if (runs.length === 0 || (trim(plain) === '' && !runs.some((r) => r.kind === 'cite' || r.kind === 'ref'))) return;
    const sentences = assignSentences(runs, plain);
    this.blocks.push(block(id, kind, { level, depth, marker, runs, text: trim(plain), cites: runs.filter((r) => r.kind === 'cite' || r.kind === 'ref').map((r) => r.anchor), sentences }));
  }

  private addCite(hint: string, entityId: string, blockId: string, nodeRef: Obj | null): Cite {
    let anchor = trim(hint);
    if (anchor === '') anchor = `c${this.cites.length + 1}`;
    if (this.used.has(anchor)) { let k = 2; while (this.used.has(`${anchor}-${k}`)) k++; anchor = `${anchor}-${k}`; }
    this.used.add(anchor);
    let number: number;
    const have = this.entityNumber.get(entityId);
    if (have !== undefined && entityId !== '') number = have;
    else { number = this.entityNumber.size + 1; this.entityNumber.set(entityId !== '' ? entityId : `anon-${anchor}`, number); }
    const c = makeCite(anchor, number, blockId, entityId, this.ctx, nodeRef);
    this.cites.push(c);
    return c;
  }
  finishCites(ids: Set<string>): Cite[] { return this.cites.filter((c) => ids.has(c.blockId) || c.blockId === ''); }

  addMarkdown(m: MdBlock): void {
    const id = this.nextId('b');
    switch (m.kind) {
      case 'rule': this.blocks.push(block(id, 'rule')); return;
      case 'code': this.blocks.push(block(id, 'code', { text: m.text })); return;
      case 'table': this.blocks.push(block(id, 'table', { rows: m.rows.map((r, i) => ({ id: `r${i}`, cells: r.map((c) => ({ runs: mdInline(c, `t${id}${i}`), header: i === 0 && m.header })) })) })); return;
      default: {
        const runs = mdInline(m.text, id); const raw = runs.map((r) => r.text).join('');
        if (runs.length === 0 || trim(raw) === '') return;
        const sentences = assignSentences(runs, raw);
        this.blocks.push(block(id, m.kind, { level: m.level, depth: m.depth, marker: m.marker, runs, text: trim(raw), sentences }));
      }
    }
  }

  /** Inline nodes to runs and the plain text. `bare` is a table cell: a source mark is a dagger with no card. The web collapses adjacent published citations of one source into one marker
   * (renderInlineContent), only when the bundle carries manifest anchors; white space between them does not break the run, other text does. */
  inline(nodes: Obj[], bare: boolean): { runs: Run[]; plain: string } {
    const runs: Run[] = []; let plain = ''; let previousPublished = 0;
    const walk = (list: Obj[]): void => {
      for (const n of list) {
        const type = str(n.type);
        if (type === 'text') {
          const t = str(n.text); if (t === '') continue;
          const marks = arr(n.marks); const types = new Set(marks.map((m) => str(m.type)));
          const link = marks.find((m) => str(m.type) === 'link'); const href = link && obj(link.attrs) ? str(link.attrs.href) : '';
          if (trim(t) !== '') previousPublished = 0;
          if (marks.length === 0 && t.includes('**')) { const parts = legacyStrong(t); for (const p of parts) { runs.push(run(`x${runs.length}`, p.text, p.bold ? { weight: 700 } : {})); plain += p.text; } continue; }
          runs.push(run(`x${runs.length}`, t, { kind: types.has('code') ? 'code' : href === '' ? 't' : 'link', weight: types.has('strong') ? 700 : 400, slant: types.has('emphasis') ? 'italic' : 'normal', href }));
          plain += t;
        } else if (type === 'hard_break') { previousPublished = 0; runs.push(run(`x${runs.length}`, '\n', { kind: 'br' })); plain += '\n'; }
        else if (type === 'source_ref') {
          const attrs = obj(n.attrs) ? n.attrs : {}; const entity = str(attrs.source_entity_id);
          if (bare) { runs.push(run(`x${runs.length}`, '†')); continue; }
          const c = this.addCite(str(attrs.id), entity, this.currentBlock, refObject(attrs.ref));
          const ma = str(attrs.id) === '' ? undefined : this.ctx.manifestAnchors.get(str(attrs.id));
          const published = ma ? refObject(ma.ref)?.doc !== undefined : false;
          if (published) { if (c.number === previousPublished) continue; previousPublished = c.number; } else previousPublished = 0;
          const isRef = c.kind === 'texture';
          runs.push(run(`x${runs.length}`, isRef ? ` ${c.title} ` : ` ${c.number} `, { kind: isRef ? 'ref' : 'cite', weight: 700, anchor: c.anchor, label: `${c.number}` }));
        } else if (typeof n.text === 'string' && n.text !== '') { runs.push(run(`x${runs.length}`, n.text)); plain += n.text; }
        else if (Array.isArray(n.content)) walk(arr(n.content));
      }
    };
    walk(nodes);
    return { runs, plain };
  }
}

/** The web's legacy bold (texture-structured-editor-doc.ts LEGACY_STRONG_RE): the opener is not preceded by a letter, digit or *, and is followed by a non-space; the closer is preceded by a
 * non-space and not followed by a letter, digit or *. Paired ** in an unmarked text node is bold written before Write parsed marks. */
function legacyStrong(t: string): { text: string; bold: boolean }[] {
  const out: { text: string; bold: boolean }[] = []; let last = 0; let i = 0;
  while (i < t.length - 1) {
    if (t[i] === '*' && t[i + 1] === '*' && (i === 0 || !(wordCh(t[i - 1]) || t[i - 1] === '*')) && i + 2 < t.length && !/[\s*]/.test(t[i + 2])) {
      const k = t.indexOf('*', i + 2);
      if (k > 0 && t[k + 1] === '*' && k > i + 2 && !/\s/.test(t[k - 1]) && !(k + 2 < t.length && (wordCh(t[k + 2]) || t[k + 2] === '*'))) {
        if (i > last) out.push({ text: t.slice(last, i), bold: false });
        out.push({ text: t.slice(i + 2, k), bold: true }); last = k + 2; i = last; continue;
      }
    }
    i++;
  }
  if (last < t.length) out.push({ text: t.slice(last), bold: false });
  return out;
}

// ---------------------------------------------------------------------------------------------------------------------------------------------------
// Documents

function decodeBodyDoc(v: unknown): Obj | null {
  let o: unknown = v;
  if (typeof v === 'string') { try { o = JSON.parse(v); } catch { return null; } }
  if (!obj(o)) return null;
  if (obj(o.doc)) return o.doc;
  return o.type === 'doc' ? o : null;
}

/** The blocks as Markdown, a citation as a `cite:` link, for the reader that draws citations as atom pills. */
export function markdownSource(blocks: Block[]): string {
  const md = (runs: Run[]): string => runs.map((r) => {
    switch (r.kind) {
      case 'cite': return `[${r.label}](cite:${r.anchor})`;
      case 'ref': return `[${trim(r.text)}](cite:${r.anchor})`;
      case 'br': return '  \n';
      case 'code': return `\`${r.text}\``;
      case 'link': return `[${r.text}](${r.href})`;
      default: {
        let t = r.text; if (trim(t) === '') return t;
        const lead = /^ */.exec(t)![0], trail = / *$/.exec(t)![0]; t = trim(t);
        if (r.weight >= 700) t = `**${t}**`; if (r.slant === 'italic') t = `*${t}*`;
        return lead + t + trail;
      }
    }
  }).join('');
  const out: string[] = [];
  for (const b of blocks) {
    switch (b.kind) {
      case 'h': out.push(`${'#'.repeat(Math.max(1, Math.min(6, b.level)))} ${md(b.runs)}`); break;
      case 'li': out.push(`${'  '.repeat(Math.max(0, b.depth - 1))}${b.marker === '' ? '  ' : b.marker.endsWith('.') ? b.marker : '-'} ${md(b.runs)}`); break;
      case 'quote': out.push(`${'> '.repeat(Math.max(1, b.depth))}${md(b.runs)}`); break;
      case 'code': out.push(`\`\`\`\n${b.text}\n\`\`\``); break;
      case 'rule': out.push('---'); break;
      case 'table': {
        const lines: string[] = [];
        b.rows.forEach((r, i) => { lines.push(`| ${r.cells.map((c) => md(c.runs)).join(' | ')} |`); if (i === 0) lines.push(`|${'---|'.repeat(r.cells.length)}`); });
        out.push(lines.join('\n')); break;
      }
      default: out.push(md(b.runs));
    }
  }
  return out.join('\n\n');
}

function build(address: string, title: string, bodyDoc: unknown, markdown: string, ctx: Ctx): ReaderDoc {
  let b = new Builder(ctx); let parsed = false;
  const root = decodeBodyDoc(bodyDoc);
  const content = root ? arr(root.content) : [];
  if (content.length > 0) { for (const n of content) b.block(n, 0, 0); parsed = b.blocks.length > 0; }
  if (!parsed) { b = new Builder(ctx); for (const m of mdBlocks(markdown)) b.addMarkdown(m); }
  let blocks = b.blocks;
  if (blocks.length > 0 && blocks[0].kind === 'h' && title !== '' && trim(blocks[0].text).toLowerCase() === title.toLowerCase()) blocks = blocks.slice(1);
  let ord = 0;
  for (const x of blocks) if (x.kind !== 'rule' && x.kind !== 'table' && x.kind !== 'embed' && x.text !== '') { x.ordinal = ord; ord += 1; }
  if (blocks.length === 0) blocks = [block('empty', 'p', { runs: [run('empty-0', 'This document has no text yet.', { slant: 'italic' })], text: 'This document has no text yet.' })];
  const doc = blankDoc(address, 'ready'); doc.title = title; doc.blocks = blocks;
  doc.cites = b.finishCites(new Set(blocks.map((x) => x.id)));
  doc.sources = sourcesOf(doc.cites);
  doc.plain = blocks.filter((x) => x.kind !== 'code').map((x) => x.text).filter((t) => t !== '').join('\n\n');
  doc.textBlocks = ord; doc.markdown = markdownSource(blocks);
  return doc;
}

/** A published bundle (GET /api/platform/publications/resolve). */
export function parseBundle(o: Obj, address: string): ReaderDoc {
  const pub = obj(o.publication) ? o.publication : {}, artifact = obj(o.artifact) ? o.artifact : {}, payload = obj(o.payload) ? o.payload : {};
  const nav = obj(o.nav) ? o.nav : {}, route = obj(o.route) ? o.route : {};
  const title = trim(str(pub.title)), kindRaw = str(pub.kind);
  const ctx = newCtx();
  for (const e of arr(o.source_entities)) if (typeof e.source_entity_id === 'string' && obj(e.entity)) ctx.entities.set(e.source_entity_id, e.entity);
  for (const e of arr(artifact.source_entities)) if (typeof e.source_entity_id === 'string' && !ctx.entities.has(e.source_entity_id)) ctx.entities.set(e.source_entity_id, e);
  for (const t of arr(o.transclusions)) if (str(t.anchor_id) !== '') ctx.transclusions.set(str(t.anchor_id), t);
  for (const r of arr(payload.resolved_blocks)) if (typeof r.cid === 'string') ctx.resolved.set(r.cid, r);
  for (const c of arr(obj(o.checks) ? o.checks.citations : [])) if (typeof c.anchor_id === 'string') ctx.checks.set(c.anchor_id, c);
  let manifest: unknown = payload.manifest;
  if (typeof manifest === 'string') { try { manifest = JSON.parse(manifest); } catch { manifest = null; } }
  for (const a of arr(obj(manifest) ? manifest.anchors : [])) if (str(a.anchor_id) !== '') ctx.manifestAnchors.set(str(a.anchor_id), a);

  const text = str(artifact.content) !== '' ? str(artifact.content) : str(payload.text);
  const doc = build(address, title, artifact.body_doc, text, ctx);
  doc.kind = kindRaw === 'opinion' ? 'opinion' : kindRaw === 'story' ? 'story' : 'document';
  const author = trim(str(pub.author_label));
  if (doc.kind === 'story') { doc.kicker = str(pub.section) !== '' ? str(pub.section) : 'Autopaper'; doc.disclosure = DISCLOSURE_STORY; }
  else if (doc.kind === 'opinion') { doc.kicker = 'Opinion'; doc.byline = author === '' ? '' : `by ${author}`; doc.disclosure = author === '' ? NOTICE_OPINION : `${NOTICE_OPINION} Author: ${author}.`; }
  else doc.kicker = 'Texture';
  doc.dek = trim(str(pub.dek));
  const version = obj(o.version) ? o.version : {};
  let rev = intOf(nav.revision) ?? intOf(version.revision) ?? intOf(payload.revision) ?? 0;
  const latest = Math.min(Math.max(0, intOf(nav.latest) ?? rev), MAX_VERSIONS);
  if (rev > latest) rev = latest;
  doc.revision = rev; doc.latest = latest; doc.newer = latest > rev && rev > 0; doc.historical = doc.newer;
  const bare = str(route.bare) !== '' ? str(route.bare) : splitRoute(address.startsWith('pub:') ? address.slice(4) : address).bare;
  doc.permalink = str(route.permalink);
  if (latest > 0 && bare !== '') {
    for (let n = latest; n >= 1; n--) doc.versions.push({ address: `pub:${bare}/v${n}`, number: n, label: n === latest ? `Version ${n} · Latest` : `Version ${n}`, when: '', author: '', current: n === rev });
  }
  const published = str(version.published_at);
  if (published !== '') {
    const when = shortDate(published);
    doc.meta = latest > 0 ? `Version ${rev} of ${latest} · ${when}` : when;
    const v = doc.versions.find((x) => x.number === rev); if (v) v.when = when;
  } else if (latest > 0) doc.meta = `Version ${rev} of ${latest}`;
  if (doc.title === '') doc.title = 'Untitled';
  return doc;
}

/** The listener's own document at a revision (GET /api/texture/revisions/{id}, its document, and the revision list). */
export function parseRevision(rev: Obj, document: Obj, address: string, revisions: Obj[]): ReaderDoc {
  const title = trim(str(document.title) !== '' ? str(document.title) : str(rev.title));
  const ctx = newCtx();
  for (const e of arr(rev.source_entities)) if (typeof e.source_entity_id === 'string') ctx.entities.set(e.source_entity_id, e);
  for (const c of arr(rev.citations)) { const a = typeof c.anchor_id === 'string' ? c.anchor_id : typeof c.id === 'string' ? c.id : null; if (a !== null) ctx.checks.set(a, c); }
  const doc = build(address, title, rev.body_doc, str(rev.content), ctx);
  doc.kind = 'document'; doc.kicker = 'Texture';
  const ta = parseAddress(address);
  const rows: { rid: string; n: number; author: string; when: string }[] = [];
  for (const r of revisions) {
    if (typeof r.revision_id !== 'string') continue;
    const n = (intOf(r.version_number) ?? rows.length) + 1;
    rows.push({ rid: r.revision_id, n, author: trim(str(r.author_label) !== '' ? str(r.author_label) : str(r.author_kind)), when: shortDate(str(r.created_at)) });
  }
  rows.sort((a, b) => b.n - a.n);
  const latest = rows.length > 0 ? rows[0].n : 0;
  const thisRev = str(rev.revision_id);
  const vn = intOf(rev.version_number);
  const thisNumber = rows.find((r) => r.rid === thisRev)?.n ?? (vn !== null ? vn + 1 : latest);
  doc.revision = Math.min(thisNumber, Math.max(latest, thisNumber));
  doc.latest = Math.max(latest, doc.revision);
  doc.newer = doc.latest > doc.revision && doc.revision > 0; doc.historical = doc.newer;
  const docId = ta.doc !== '' ? ta.doc : str(document.doc_id);
  doc.versions = rows.map((r) => ({ address: r.n === latest ? `texture:${docId}` : `texture:${docId}@${r.rid}`, number: r.n, label: r.n === latest ? `Version ${r.n} · Latest` : `Version ${r.n}`, when: r.when, author: r.author, current: r.n === doc.revision }));
  const cur = rows.find((r) => r.n === doc.revision);
  const when = cur ? cur.when : shortDate(str(rev.created_at));
  doc.byline = cur ? cur.author : '';
  doc.meta = [doc.latest > 0 ? `Version ${doc.revision} of ${doc.latest}` : '', when].filter((s) => s !== '').join(' · ');
  doc.revising = typeof document.agent_revision_pending === 'boolean' ? document.agent_revision_pending : typeof document.revising === 'boolean' ? document.revising : false;
  doc.trajectory = str(document.trajectory_id);
  doc.revisingNote = typeof document.revising_note === 'string' ? document.revising_note : doc.revising ? 'The desk is writing a new version.' : '';
  if (doc.title === '') doc.title = 'Untitled';
  return doc;
}

/** The related list. The real route answers `links` (checked against choir.news, 2026-11-11); the older guesses stay readable. */
export function parseRelated(o: unknown): ReaderDoc['related'] {
  if (!obj(o)) return [];
  const list = arr(o.links).length > 0 ? arr(o.links) : arr(o.related).length > 0 ? arr(o.related) : arr(o.stories).length > 0 ? arr(o.stories) : arr(o.items);
  const out: ReaderDoc['related'] = [];
  for (const s of list) {
    const title = str(s.title); if (title === '') continue;
    const rt = typeof s.route === 'string' ? s.route : obj(s.route) ? (typeof s.route.bare === 'string' ? s.route.bare : str(s.route.path)) : '';
    const clean = rt.startsWith('/') ? rt.slice(1) : rt;
    if (clean === '') continue;
    out.push({ address: `pub:${clean}`, title, dek: str(s.dek) });
  }
  return out;
}
