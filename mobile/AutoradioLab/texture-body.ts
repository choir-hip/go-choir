// The phone's Texture document stays Texture-format compatible always (owner, 2026-11-16: "It still needs to be texture format compatible always"). In shared code now (it was Swift:
// TextureBody.swift `TextureMd` and `TextureBodyMerge`). A revision is the Markdown text (`content`) and the structured `body_doc` (`choir.texture_doc.v1`). The editor works on the Markdown;
// `merge` writes the `body_doc` that goes with it, as the web editor does (frontend/src/packages/texture/texture-structured-editor-doc.ts): paragraph, heading, bullet_list, ordered_list,
// list_item, blockquote, code_block, horizontal_rule, hard_break, text with the marks strong, emphasis and code. The web has no link mark: a link is its text in `body_doc` and `[t](u)` in `content`.
//
// The rule that keeps it compatible: a block the listener did not change is the ORIGINAL node, byte for byte (its ids, its source_ref citations, an embed, a table, a node type this phone has
// never heard of). Only a changed block is written fresh, and it keeps its old id and the citations that were on it. A node with no Markdown block of its own stays where it was.
//
// Ways this can fail (each has a test in texture-body.test.ts):
//  1. Load, edit nothing, save: the Markdown is the original string and the body_doc is the original, equal in every key.
//  2. A small edit changes other nodes: only the edited node differs; every other node is equal to its original.
//  3. A citation (source_ref), an embed, a table or an unknown node is dropped or reordered by an edit nearby.
//  4. An edited block whose original node this phone cannot write (a table, an embed) is flattened into a paragraph: it keeps its node and the edit lives in `content` (counted in `opaqueEdits`).
//  5. A new document (no body_doc) gets none, or one the web editor cannot open: it gets schema and doc with fresh ids.
// Ids come from `crypto.getRandomValues` (the call already used on the device: account.ts); a test sets its own source with `setIdSource`.

// eslint-disable-next-line @typescript-eslint/no-explicit-any
export type Node = { [k: string]: any };
export type Merged = { body: Node; opaqueEdits: number };
export type Kind = 'code' | 'heading' | 'rule' | 'quote' | 'list' | 'table' | 'paragraph';

let idSource: () => string = () => Array.from(crypto.getRandomValues(new Uint8Array(4)), (b) => b.toString(16).padStart(2, '0')).join('');
export function setIdSource(f: (() => string) | null): void { idSource = f ?? (() => Array.from(crypto.getRandomValues(new Uint8Array(4)), (b) => b.toString(16).padStart(2, '0')).join('')); }
export const newID = (kind: string): string => `${kind}-${idSource()}`;

// Swift `.whitespaces`: spaces and tabs, not newlines.
const trim = (s: string): string => s.replace(/^[\p{Zs}\t]+|[\p{Zs}\t]+$/gu, '');
const isFence = (t: string): boolean => t.startsWith('```') || t.startsWith('~~~');
const keepLetters = (s: string): string => s.replace(/[^\p{L}\p{M}\p{Nd}]/gu, '').toLowerCase();       // Swift `CharacterSet.letters` (L and M) and `decimalDigits` (Nd)

// MARK: Blocks

/** Top-level blocks of Markdown source: a fenced block is one block (blank lines inside it stay), otherwise blocks end at a blank line. */
export function blocks(src: string): string[] {
  const lines = src.replace(/\r\n/g, '\n').split('\n');
  const out: string[] = [];
  let cur: string[] = [];
  const flush = (): void => { if (cur.length > 0) { out.push(cur.join('\n')); cur = []; } };
  let i = 0;
  while (i < lines.length) {
    const l = lines[i];
    const t = trim(l);
    if (isFence(t)) {
      flush();
      const fence = t.slice(0, 3);
      const block = [l];
      i += 1;
      while (i < lines.length) { block.push(lines[i]); if (trim(lines[i]).startsWith(fence)) { i += 1; break; } i += 1; }
      out.push(block.join('\n'));
      continue;
    }
    if (t === '') flush(); else cur.push(l);
    i += 1;
  }
  flush();
  return out;
}

export function headingLevel(t: string): [number, string] | null {
  let n = 0;
  while (n < t.length && t[n] === '#') n += 1;
  if (n < 1 || n > 6 || t[n] !== ' ') return null;
  return [n, trim(t.slice(n + 1))];
}
export function isRule(t: string): boolean {
  const s = t.replace(/ /g, '');
  return s.length >= 3 && '-*_'.includes(s[0]) && [...s].every((c) => c === s[0]);
}
export type Marker = { indent: number; ordered: boolean; number: number; rest: string };
/** (indent, ordered?, number, rest) of a list line. */
export function listMarker(line: string): Marker | null {
  let indent = 0, k = 0;
  while (k < line.length && (line[k] === ' ' || line[k] === '\t')) { indent += line[k] === '\t' ? 4 : 1; k += 1; }
  const chars = line.slice(k);
  if (chars.length >= 2 && '-*+'.includes(chars[0]) && chars[1] === ' ') return { indent, ordered: false, number: 0, rest: chars.slice(2) };
  const m = /^(\p{Nd}+)([.)]) /u.exec(chars);
  if (m) { const n = parseInt(m[1], 10); return { indent, ordered: true, number: Number.isFinite(n) ? n : 1, rest: chars.slice(m[0].length) }; }
  return null;
}

export function kindOf(block: string): Kind {
  const lines = block.split('\n');
  const first = lines[0] ?? '';
  const t = trim(first);
  if (isFence(t)) return 'code';
  if (headingLevel(t) !== null) return 'heading';
  if (isRule(t) && !block.includes('\n')) return 'rule';
  if (t.startsWith('>')) return 'quote';
  if (listMarker(first) !== null) return 'list';
  if (t.startsWith('|') && block.includes('|') && lines.length >= 2) return 'table';
  return 'paragraph';
}

// MARK: Matching keys

/** What a block says, without its syntax: letters and digits, lower case, link destinations and markers gone. */
export function key(block: string): string {
  let s = block;
  if (isFence(trim(s))) s = s.split('\n').slice(1).join('\n');                                              // the fence's language word
  s = s.split('\n').map((l) => l.replace(/^\s*(>\s*)*([-*+]|\d+[.)])\s+/, '')).join('\n');                  // list markers
  s = s.replace(/\]\([^)]*\)/g, ']');
  return keepLetters(s);
}

export function plainText(node: Node): string {
  if (node.type === 'text') return typeof node.text === 'string' ? node.text : '';
  return (Array.isArray(node.content) ? (node.content as Node[]) : []).map(plainText).join('');
}
const nodeKey = (node: Node): string => keepLetters(plainText(node));

function compatible(k: Kind, type: string): boolean {
  switch (k) {
    case 'code': return type === 'code_block';
    case 'heading': return type === 'heading';
    case 'rule': return type === 'horizontal_rule';
    case 'quote': return type === 'blockquote';
    case 'list': return type === 'bullet_list' || type === 'ordered_list';
    case 'table': return type === 'table';
    case 'paragraph': return type === 'paragraph';
  }
}
/** The node types this phone can write from Markdown. */
const WRITABLE = new Set(['paragraph', 'heading', 'bullet_list', 'ordered_list', 'blockquote', 'code_block', 'horizontal_rule']);

// MARK: Markdown to nodes (what the web editor writes)

export function paragraph(text: string): Node {
  return { type: 'paragraph', attrs: { id: newID('paragraph') }, content: inline(text) };
}
function paragraphLines(lines: string[]): Node {
  return { type: 'paragraph', attrs: { id: newID('paragraph') }, content: inline(lines.join('\n')) };
}

export function nodeFor(block: string): Node {
  switch (kindOf(block)) {
    case 'code': {
      const lines = block.split('\n');
      lines.shift();
      const last = lines[lines.length - 1];
      if (last !== undefined && isFence(trim(last))) lines.pop();
      return { type: 'code_block', attrs: { id: newID('code-block') }, content: [{ type: 'text', text: lines.join('\n') }] };
    }
    case 'heading': {
      const h = headingLevel(trim(block)) ?? [1, block];
      return { type: 'heading', attrs: { id: newID('heading'), level: h[0] }, content: inline(h[1]) };
    }
    case 'rule': return { type: 'horizontal_rule', attrs: { id: newID('hr') } };
    case 'quote': {
      const inner = block.split('\n').map((l) => { let t = trim(l); if (t.startsWith('>')) { t = t.slice(1); if (t.startsWith(' ')) t = t.slice(1); } return t; }).join('\n');
      const content = blocks(inner).map(nodeFor);
      return { type: 'blockquote', attrs: { id: newID('blockquote') }, content: content.length === 0 ? [paragraph('')] : content };
    }
    case 'list': return list(block.split('\n'));
    case 'table': case 'paragraph': return paragraph(block);
  }
}

function list(lines: string[]): Node {
  const base = Math.min(...lines.map((l) => listMarker(l)?.indent).filter((n): n is number => n !== undefined), Infinity);
  const baseIndent = Number.isFinite(base) ? base : 0;
  const first = listMarker(lines[0]);
  const items: string[][] = [];
  for (const l of lines) {
    const m = listMarker(l);
    if (m !== null && m.indent <= baseIndent) items.push([l]);
    else if (items.length > 0) items[items.length - 1].push(l);
    else items.push([l]);
  }
  const ordered = first?.ordered ?? false;
  const nodes: Node[] = items.map((item) => {
    const head = listMarker(item[0])?.rest ?? item[0];
    const content: Node[] = [];
    const para = [head];
    const rest = item.slice(1);
    const k = rest.findIndex((x) => listMarker(x) !== null);
    if (k >= 0) {
      para.push(...rest.slice(0, k).map(trim));
      content.push(paragraphLines(para));
      content.push(list(rest.slice(k)));
    } else {
      para.push(...rest.map(trim));
      content.push(paragraphLines(para));
    }
    return { type: 'list_item', attrs: { id: newID('list-item') }, content };
  });
  const attrs: Node = { id: newID('list') };
  if (ordered) attrs.start = Math.max(first?.number ?? 1, 1);
  return { type: ordered ? 'ordered_list' : 'bullet_list', attrs, content: nodes };
}

// MARK: Inline

/** Inline Markdown as text nodes with the marks strong, emphasis and code, and hard breaks. A link keeps its text (the web has no link mark). */
export function inline(src: string): Node[] {
  const out: Node[] = [];
  let lastMarks: string[] | null = null;
  const same = (a: string[] | null, b: string[]): boolean => a !== null && a.length === b.length && a.every((x, i) => x === b[i]);
  const add = (text: string, marks: string[]): void => {
    if (text === '') return;
    const last = out[out.length - 1];
    if (last && last.type === 'text' && same(lastMarks, marks)) { last.text = (typeof last.text === 'string' ? last.text : '') + text; return; }
    const n: Node = { type: 'text', text };
    if (marks.length > 0) n.marks = marks.map((type) => ({ type }));
    out.push(n);
    lastMarks = marks;
  };
  const closer = (s: string[], from: number, pat: string[]): number | null => {
    let j = from;
    while (j + pat.length <= s.length) {
      if (j > from && pat.every((p, k) => s[j + k] === p)) return j;
      j += 1;
    }
    return null;
  };
  const withMark = (marks: string[], m: string): string[] => (marks.includes(m) ? marks : [...marks, m]);
  const scan = (s: string[], marks: string[]): void => {
    let i = 0, buf = '';
    const flush = (): void => { add(buf, marks); buf = ''; };
    while (i < s.length) {
      const c = s[i];
      if (c === '\\' && i + 1 < s.length) {
        if (s[i + 1] === '\n') { flush(); out.push({ type: 'hard_break' }); i += 2; continue; }
        buf += s[i + 1]; i += 2; continue;
      }
      if (c === ' ' && i + 2 < s.length && s[i + 1] === ' ' && s[i + 2] === '\n') { flush(); out.push({ type: 'hard_break' }); i += 3; continue; }
      if (c === '\n') { buf += ' '; i += 1; continue; }
      if (c === '`') {
        const j = s.indexOf('`', i + 1);
        if (j >= 0) { flush(); add(s.slice(i + 1, j).join(''), withMark(marks, 'code')); i = j + 1; continue; }
      }
      if ((c === '*' || c === '_') && i + 1 < s.length && s[i + 1] === c) {
        const j = closer(s, i + 2, [c, c]);
        if (j !== null) { flush(); scan(s.slice(i + 2, j), withMark(marks, 'strong')); i = j + 2; continue; }
      }
      if ((c === '*' || c === '_') && i + 1 < s.length && s[i + 1] !== ' ') {
        const j = closer(s, i + 1, [c]);
        if (j !== null) { flush(); scan(s.slice(i + 1, j), withMark(marks, 'emphasis')); i = j + 1; continue; }
      }
      if (c === '[') {
        const close = s.indexOf(']', i + 1);
        if (close >= 0 && close + 1 < s.length && s[close + 1] === '(') {
          const end = s.indexOf(')', close + 2);
          if (end >= 0) { flush(); scan(s.slice(i + 1, close), marks); i = end + 1; continue; }
        }
      }
      buf += c; i += 1;
    }
    flush();
  };
  scan(Array.from(src), []);
  return out;
}

// MARK: Merge

export const topNodes = (body: Node | null | undefined): Node[] => {
  const doc = body && typeof body.doc === 'object' && body.doc !== null ? (body.doc as Node) : null;
  return doc && Array.isArray(doc.content) ? (doc.content as Node[]) : [];
};

function root(content: Node[], like: Node | null): Node {
  const out: Node = { ...(like ?? {}) };
  out.schema = typeof like?.schema === 'string' ? like.schema : 'choir.texture_doc.v1';
  const doc: Node = like && typeof like.doc === 'object' && like.doc !== null ? { ...(like.doc as Node) } : { type: 'doc', attrs: { id: newID('doc') } };
  doc.content = content;
  out.doc = doc;
  return out;
}


// MARK: Diff (bounded)

/** Cells of the exact table `diffPairs` may allocate (about 4 MB as Int32): Exact gives an answer a 64 MiB heap and 100 ms of CPU (docs/reference/EXACT.md), and a book-length document is thousands of lines. */
export const EXACT_CELLS = 1_000_000;

/** The pairs [i, k] with a[i] === b[k], increasing in both: a longest common subsequence where it is affordable, else a good one found by anchors.
 * The common head and tail are paired first (an edit is local, so the middle is small). A middle of at most EXACT_CELLS cells is the exact table. A larger middle is cut at the lines that
 * occur once in each side (a patience diff), and each gap between anchors is done the same way; a gap with no anchor and too large for the table stays unpaired (its lines count as changed:
 * correct, only coarser). Memory is linear in the lines; time is near linear. */
export function diffPairs(a: string[], b: string[]): [number, number][] {
  const out: [number, number][] = [];
  diffMiddle(a, b, 0, a.length, 0, b.length, out);
  return out;
}

function diffMiddle(a: string[], b: string[], a0: number, a1: number, b0: number, b1: number, out: [number, number][]): void {
  while (a0 < a1 && b0 < b1 && a[a0] === b[b0]) { out.push([a0, b0]); a0 += 1; b0 += 1; }          // the common head, then the common tail (an edit is local)
  const tail: [number, number][] = [];
  while (a1 > a0 && b1 > b0 && a[a1 - 1] === b[b1 - 1]) { a1 -= 1; b1 -= 1; tail.push([a1, b1]); }
  const m = a1 - a0, p = b1 - b0;
  if (m > 0 && p > 0) {
    if ((m + 1) * (p + 1) <= EXACT_CELLS) exactTable(a, b, a0, a1, b0, b1, out);
    else {
      let pa = a0, pb = b0;
      for (const [i, k] of uniqueAnchors(a, b, a0, a1, b0, b1)) { diffMiddle(a, b, pa, i, pb, k, out); out.push([i, k]); pa = i + 1; pb = k + 1; }
      if (pa > a0 || pb > b0) diffMiddle(a, b, pa, a1, pb, b1, out);                                  // no anchor at all: the gap stays unpaired
    }
  }
  for (let t = tail.length - 1; t >= 0; t--) out.push(tail[t]);
}

function exactTable(a: string[], b: string[], a0: number, a1: number, b0: number, b1: number, out: [number, number][]): void {
  const m = a1 - a0, p = b1 - b0, w = p + 1;
  const t = new Int32Array((m + 1) * w);
  for (let i = m - 1; i >= 0; i--) for (let k = p - 1; k >= 0; k--) t[i * w + k] = a[a0 + i] === b[b0 + k] ? t[(i + 1) * w + k + 1] + 1 : Math.max(t[(i + 1) * w + k], t[i * w + k + 1]);
  let i = 0, k = 0;
  while (i < m && k < p) {
    if (a[a0 + i] === b[b0 + k]) { out.push([a0 + i, b0 + k]); i += 1; k += 1; }
    else if (t[(i + 1) * w + k] >= t[i * w + k + 1]) i += 1; else k += 1;
  }
}

/** Lines that occur exactly once in a[a0,a1) and once in b[b0,b1), as the longest increasing run of their positions (patience sorting). */
function uniqueAnchors(a: string[], b: string[], a0: number, a1: number, b0: number, b1: number): [number, number][] {
  const countA = new Map<string, number>(), countB = new Map<string, number>(), posB = new Map<string, number>();
  for (let i = a0; i < a1; i++) countA.set(a[i], (countA.get(a[i]) ?? 0) + 1);
  for (let k = b0; k < b1; k++) { countB.set(b[k], (countB.get(b[k]) ?? 0) + 1); posB.set(b[k], k); }
  const cand: [number, number][] = [];
  for (let i = a0; i < a1; i++) if (countA.get(a[i]) === 1 && countB.get(a[i]) === 1) cand.push([i, posB.get(a[i])!]);
  const tails: number[] = [], prev: number[] = new Array<number>(cand.length).fill(-1);
  for (let x = 0; x < cand.length; x++) {
    let lo = 0, hi = tails.length;
    while (lo < hi) { const mid = (lo + hi) >> 1; if (cand[tails[mid]][1] < cand[x][1]) lo = mid + 1; else hi = mid; }
    if (lo > 0) prev[x] = tails[lo - 1];
    tails[lo] = x;
  }
  const res: [number, number][] = [];
  for (let x = tails.length > 0 ? tails[tails.length - 1] : -1; x >= 0; x = prev[x]) res.push(cand[x]);
  return res.reverse();
}

/** A three-way merge of text by lines: what `mine` changed from `base` and what `theirs` changed from `base`, together. Null when both changed the same lines differently (a conflict).
 * Lines are kept as they are, so what neither side touched is byte for byte what it was. */
export function merge3(base: string, mine: string, theirs: string): string | null {
  if (mine === theirs) return mine;
  if (mine === base) return theirs;
  if (theirs === base) return mine;
  const o = base.split('\n'), a = mine.split('\n'), b = theirs.split('\n');
  const ma = new Int32Array(o.length).fill(-1), mb = new Int32Array(o.length).fill(-1);
  for (const [i, k] of diffPairs(o, a)) ma[i] = k;
  for (const [i, k] of diffPairs(o, b)) mb[i] = k;
  const out: string[] = [];
  let io = 0, ia = 0, ib = 0;
  const same = (x: string[], y: string[]): boolean => x.length === y.length && x.every((l, i) => l === y[i]);
  for (let i = 0; i <= o.length; i++) {
    const end = i === o.length;
    if (!end && !(ma[i] >= 0 && mb[i] >= 0)) continue;                                              // a line only one side kept is not a place the two agree on
    const ea = end ? a.length : ma[i], eb = end ? b.length : mb[i];
    const co = o.slice(io, i), ca = a.slice(ia, ea), cb = b.slice(ib, eb);
    if (same(ca, co)) out.push(...cb); else if (same(cb, co) || same(ca, cb)) out.push(...ca); else return null;
    if (!end) out.push(o[i]);
    io = i + 1; ia = ea + 1; ib = eb + 1;
  }
  return out.join('\n');
}

/** An edited node keeps the id it had, and a paragraph keeps its citations (they stay at the end, where the web editor puts them). */
function carry(old: Node, fresh: Node): void {
  const id = old.attrs && typeof old.attrs === 'object' ? (old.attrs as Node).id : undefined;
  if (id !== undefined) fresh.attrs = { ...(fresh.attrs ?? {}), id };
  if (old.type !== 'paragraph' || fresh.type !== 'paragraph') return;
  const refs = (Array.isArray(old.content) ? (old.content as Node[]) : []).filter((n) => n.type === 'source_ref');
  if (refs.length > 0) fresh.content = [...(Array.isArray(fresh.content) ? (fresh.content as Node[]) : []), ...refs];
}

/** The `body_doc` for `newMarkdown`, given the revision it was loaded from (`original`, `originalMarkdown`). */
export function merge(original: Node | null, originalMarkdown: string, newMarkdown: string): Merged {
  if (original && newMarkdown === originalMarkdown) return { body: original, opaqueEdits: 0 };
  const s = topNodes(original);
  const b = blocks(originalMarkdown), n = blocks(newMarkdown);
  if (!original || (s.length === 0 && b.length === 0)) {
    const nodes = n.map(nodeFor);
    return { body: root(nodes.length === 0 ? [paragraph('')] : nodes, original), opaqueEdits: 0 };
  }
  // Which original node each original block is (forward, by what it says and what it is). The nodes' keys are read once: a document of thousands of blocks must not pay for them per block.
  const map: (number | null)[] = b.map(() => null);
  const sKeys = s.map(nodeKey), sTypes = s.map((x) => (typeof x.type === 'string' ? x.type : ''));
  let cursor = 0;
  for (let i = 0; i < b.length; i++) {
    const k = kindOf(b[i]), kk = key(b[i]);
    for (let j = cursor; j < s.length; j++) {
      if (sKeys[j] === kk && compatible(k, sTypes[j])) { map[i] = j; cursor = j + 1; break; }
    }
  }
  // Nodes no block claims (an embed, a source flow, an unknown node) stay after the block that came before them.
  const lead: Node[] = [];
  const attach = new Map<number, Node[]>();
  let lastClaimed: number | null = null;
  const blockOf = new Map<number, number>();
  map.forEach((j, i) => { if (j !== null) blockOf.set(j, i); });
  for (let j = 0; j < s.length; j++) {
    const i = blockOf.get(j);
    if (i !== undefined) lastClaimed = i;
    else if (lastClaimed !== null) attach.set(lastClaimed, [...(attach.get(lastClaimed) ?? []), s[j]]);
    else lead.push(s[j]);
  }
  // Block diff: the pairs of equal blocks, in order (diffPairs: bounded in time and memory for a document of any length).
  const m = b.length, p = n.length;
  const pairs = diffPairs(b, n);
  pairs.push([m, p]);
  const out: Node[] = [...lead];
  let opaque = 0, pi = 0, pk = 0;
  for (const [mi, mk] of pairs) {
    const deleted: number[] = [], inserted: number[] = [];
    for (let x = pi; x < mi; x++) deleted.push(x);
    for (let x = pk; x < mk; x++) inserted.push(x);
    inserted.forEach((kk, t) => {
      const di = t < deleted.length ? map[deleted[t]] : null;
      const old: Node | null = di !== null && di !== undefined ? s[di] : null;
      if (old && !WRITABLE.has(typeof old.type === 'string' ? old.type : '')) { out.push(old); opaque += 1; return; }          // a table, an embed, an unknown node: kept; the edit lives in `content`
      const fresh = nodeFor(n[kk]);
      if (old) carry(old, fresh);
      out.push(fresh);
    });
    for (const d of deleted) { const a = attach.get(d); if (a) out.push(...a); }                                           // what sat after a removed block stays
    if (mi < m) {
      const j = map[mi];
      if (j !== null) out.push(s[j]); else out.push(nodeFor(n[mk]));
      const a = attach.get(mi);
      if (a) out.push(...a);
    }
    pi = mi + 1; pk = mk + 1;
  }
  return { body: root(out.length === 0 ? [paragraph('')] : out, original), opaqueEdits: opaque };
}

// MARK: Nodes to Markdown

function inlineMarkdown(nodes: Node[]): string {
  let out = '';
  for (const n of nodes) {
    if (n.type === 'text') {
      let t = typeof n.text === 'string' ? n.text : '';
      const marks = (Array.isArray(n.marks) ? (n.marks as Node[]) : []).map((x) => x.type);
      if (marks.includes('code')) t = `\`${t}\``;
      if (marks.includes('emphasis')) t = `*${t}*`;
      if (marks.includes('strong')) t = `**${t}**`;
      out += t;
    } else if (n.type === 'hard_break') out += '  \n';
  }
  return out;
}

function blockMarkdown(n: Node, indent: string): string | null {
  const content: Node[] = Array.isArray(n.content) ? (n.content as Node[]) : [];
  switch (n.type) {
    case 'paragraph': return indent + inlineMarkdown(content);
    case 'heading': return '#'.repeat(typeof n.attrs?.level === 'number' ? n.attrs.level : 1) + ' ' + inlineMarkdown(content);
    case 'horizontal_rule': return '---';
    case 'code_block': return '```\n' + (typeof content[0]?.text === 'string' ? content[0].text : '') + '\n```';
    case 'blockquote': return content.map((c) => blockMarkdown(c, '')).filter((x): x is string => x !== null).join('\n\n').split('\n').map((l) => '> ' + l).join('\n');
    case 'bullet_list': case 'ordered_list': {
      const ordered = n.type === 'ordered_list';
      let num = typeof n.attrs?.start === 'number' ? n.attrs.start : 1;
      const lines: string[] = [];
      for (const item of content) {
        const kids: Node[] = Array.isArray(item.content) ? (item.content as Node[]) : [];
        const marker = ordered ? `${num}.` : '-';
        num += 1;
        kids.forEach((kid, idx) => {
          if (idx === 0 && kid.type === 'paragraph') lines.push(indent + marker + ' ' + inlineMarkdown(Array.isArray(kid.content) ? (kid.content as Node[]) : []));
          else { const b = blockMarkdown(kid, indent + '  '); if (b !== null) lines.push(b); }
        });
      }
      return lines.join('\n');
    }
    default: return null;
  }
}

/** The Markdown for a `body_doc` (the server's `content` is this, or near it). Nodes with no Markdown form (an embed, a table, a node this phone does not know) have no block: they are kept by the merge, not written here. */
export function markdownFrom(body: Node | null | undefined): string {
  return topNodes(body).map((n) => blockMarkdown(n, '')).filter((x): x is string => x !== null).join('\n\n');
}
