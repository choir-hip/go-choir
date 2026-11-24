// The phone's Texture document stays Texture-format compatible always (owner, 2026-11-16: "It still needs to be texture format compatible always").
// A revision is the Markdown text (`content`) and the structured `body_doc` (`choir.texture_doc.v1`). The editor works on the Markdown; `merge` writes the
// `body_doc` that goes with it, as the web editor does (frontend/src/packages/texture/texture-structured-editor-doc.ts): paragraph, heading, bullet_list,
// ordered_list, list_item, blockquote, code_block, horizontal_rule, hard_break, text with the marks strong, emphasis and code. The web has no link mark: a
// link is its text in `body_doc` and `[t](u)` in `content`. Ported from the Swift TextureBody.swift (deleted with it, 2026-11-23), tests texture-body.test.ts.
//
// The rule that keeps it compatible: a block the listener did not change is the ORIGINAL node, byte for byte (its ids, its source_ref citations, an embed, a
// table, a node type this phone has never heard of). Only a changed block is written fresh, and it keeps its old id and the citations that were on it. A node
// with no Markdown block of its own (an embed, a source flow) stays where it was.
//
// Ways this can fail (each has a test):
//  1. Load, edit nothing, save: the Markdown is the original string and the body_doc is the original, equal in every key.
//  2. A small edit changes other nodes: only the edited node differs; every other node is equal to its original.
//  3. A citation (source_ref), an embed, a table or an unknown node is dropped or reordered by an edit nearby.
//  4. An edited block whose original node this phone cannot write (a table, an embed) is flattened into a paragraph: it keeps its node and the edit lives in
//     `content` (counted in `opaqueEdits`).
//  5. A new document (no body_doc) gets none, or one the web editor cannot open: it gets schema and doc with fresh ids.
//  6. The `content` sent with a body_doc is not the text the server derives from it: the server refuses the revision (400 "content must match derived body_doc projection"). `projectBody` is the server's
//     projection (internal/texturedoc/projection.go), held to it by the shared fixture mobile/fixtures/texture-body/projection.json (written by a Go test; texture-body-projection.test.ts reads it). `canon` is the
//     one rule that says two Markdown texts are the same text: their projections are equal.

export type Node = Record<string, unknown>;
export type Body = Record<string, unknown>;
export type Kind = 'code' | 'heading' | 'rule' | 'quote' | 'list' | 'table' | 'paragraph';

const isObj = (v: unknown): v is Record<string, unknown> => v !== null && typeof v === 'object' && !Array.isArray(v);
const str = (v: unknown): string => (typeof v === 'string' ? v : '');
/** Swift's `.whitespaces`: spaces and tabs (the Zs category), never a newline. */
const trim = (s: string): string => s.replace(/^[\p{Zs}\t]+|[\p{Zs}\t]+$/gu, '');
const LD = /[\p{L}\p{M}\p{Nd}]/u;
const lettersDigits = (s: string): string => [...s].filter((c) => LD.test(c)).join('').toLowerCase();

// ---------------------------------------------------------------------------------------------------------------- blocks

/** Top-level blocks of Markdown source: a fenced block is one block (blank lines inside it stay), otherwise blocks end at a blank line. */
export function blocks(src: string): string[] {
  const lines = src.replace(/\r\n/g, '\n').split('\n');
  const out: string[] = [];
  let cur: string[] = [];
  let i = 0;
  const flush = (): void => { if (cur.length > 0) { out.push(cur.join('\n')); cur = []; } };
  while (i < lines.length) {
    const l = lines[i];
    const t = trim(l);
    if (t.startsWith('```') || t.startsWith('~~~')) {
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

export function kindOf(block: string): Kind {
  const lines = block.split('\n');
  const first = lines[0] ?? '';
  const t = trim(first);
  if (t.startsWith('```') || t.startsWith('~~~')) return 'code';
  if (headingLevel(t) !== null) return 'heading';
  if (isRule(t) && !block.includes('\n')) return 'rule';
  if (t.startsWith('>')) return 'quote';
  if (listMarker(first) !== null) return 'list';
  if (t.startsWith('|') && block.includes('|') && lines.length >= 2) return 'table';
  return 'paragraph';
}

export function headingLevel(t: string): { level: number; text: string } | null {
  let n = 0;
  for (const c of t) { if (c === '#') n += 1; else break; }
  if (n < 1 || n > 6 || t[n] !== ' ') return null;
  return { level: n, text: trim(t.slice(n + 1)) };
}
function isRule(t: string): boolean {
  const s = [...t.replace(/ /g, '')];
  if (s.length < 3 || !'-*_'.includes(s[0])) return false;
  return s.every((c) => c === s[0]);
}
/** (indent, ordered?, number, rest) of a list line. */
export function listMarker(line: string): { indent: number; ordered: boolean; number: number; rest: string } | null {
  let indent = 0;
  const chars = [...line];
  let p = 0;
  while (p < chars.length && (chars[p] === ' ' || chars[p] === '\t')) { indent += chars[p] === '\t' ? 4 : 1; p += 1; }
  if (p < chars.length && '-*+'.includes(chars[p]) && chars[p + 1] === ' ') return { indent, ordered: false, number: 0, rest: chars.slice(p + 2).join('') };
  let digits = '';
  let q = p;
  while (q < chars.length && /\p{N}/u.test(chars[q])) { digits += chars[q]; q += 1; }
  if (digits !== '' && (chars[q] === '.' || chars[q] === ')') && chars[q + 1] === ' ') {
    const n = Number.parseInt(digits, 10);
    return { indent, ordered: true, number: Number.isFinite(n) ? n : 1, rest: chars.slice(q + 2).join('') };
  }
  return null;
}

// ---------------------------------------------------------------------------------------------------------------- matching keys

/** What a block says, without its syntax: letters and digits, lower case, link destinations and markers gone. */
export function key(block: string): string {
  let s = block;
  const t = trim(s);
  if (t.startsWith('```') || t.startsWith('~~~')) s = s.split('\n').slice(1).join('\n');     // the fence's language word
  s = s.split('\n').map((l) => l.replace(/^\s*(>\s*)*([-*+]|\d+[.)])\s+/u, '')).join('\n');   // list markers
  s = s.replace(/\]\([^)]*\)/g, ']');
  return lettersDigits(s);
}
export function plainText(node: Node): string {
  if (node.type === 'text') return str(node.text);
  return (Array.isArray(node.content) ? (node.content as Node[]) : []).map(plainText).join('');
}
const nodeKey = (node: Node): string => lettersDigits(plainText(node));

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

// ---------------------------------------------------------------------------------------------------------------- Markdown to nodes (what the web editor writes)

export const newID = (kind: string): string => `${kind}-` + Array.from(crypto.getRandomValues(new Uint8Array(4)), (b) => b.toString(16).padStart(2, '0')).join('');

export function nodeFor(block: string): Node {
  switch (kindOf(block)) {
    case 'code': {
      const lines = block.split('\n');
      const language = trim(trim(lines.shift() ?? '').replace(/^(```|~~~)/, ''));
      const last = lines[lines.length - 1];
      if (last !== undefined && (trim(last).startsWith('```') || trim(last).startsWith('~~~'))) lines.pop();
      const attrs: Record<string, unknown> = { id: newID('code-block') };
      if (language !== '') attrs.language = language;                                       // the projection writes it after the fence
      return { type: 'code_block', attrs, content: lines.length === 0 ? [] : [{ type: 'text', text: lines.join('\n') }] };
    }
    case 'heading': {
      const h = headingLevel(trim(block)) ?? { level: 1, text: block };
      return { type: 'heading', attrs: { id: newID('heading'), level: h.level }, content: inline(h.text) };
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

export const paragraph = (text: string): Node => ({ type: 'paragraph', attrs: { id: newID('paragraph') }, content: inline(text) });

function list(lines: string[]): Node {
  const base = Math.min(...lines.map((l) => listMarker(l)?.indent).filter((x): x is number => x !== undefined), Infinity);
  const first = listMarker(lines[0]);
  const items: string[][] = [];
  for (const l of lines) {
    const m = listMarker(l);
    if (m !== null && m.indent <= (Number.isFinite(base) ? base : 0)) items.push([l]);
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
  const attrs: Record<string, unknown> = { id: newID('list') };
  if (ordered) attrs.start = Math.max(first?.number ?? 1, 1);
  return { type: ordered ? 'ordered_list' : 'bullet_list', attrs, content: nodes };
}
const paragraphLines = (lines: string[]): Node => ({ type: 'paragraph', attrs: { id: newID('paragraph') }, content: inline(lines.join('\n')) });

// ---------------------------------------------------------------------------------------------------------------- inline

/** Inline Markdown as text nodes with the marks strong, emphasis and code, and hard breaks. A link keeps its text (the web has no link mark). */
export function inline(src: string): Node[] {
  const out: Node[] = [];
  let lastMarks: string[] | null = null;
  const same = (a: string[] | null, b: string[]): boolean => a !== null && a.length === b.length && a.every((x, i) => x === b[i]);
  const add = (text: string, marks: string[]): void => {
    if (text === '') return;
    const last = out[out.length - 1];
    if (last !== undefined && last.type === 'text' && same(lastMarks, marks)) { last.text = str(last.text) + text; return; }
    const n: Node = { type: 'text', text };
    if (marks.length > 0) n.marks = marks.map((m) => ({ type: m }));
    out.push(n);
    lastMarks = marks;
  };
  const closer = (s: string[], from: number, pat: string[]): number | null => {
    for (let j = from; j + pat.length <= s.length; j += 1) {
      if (j > from && pat.every((c, k) => s[j + k] === c)) return j;
    }
    return null;
  };
  const indexFrom = (s: string[], c: string, from: number): number => { for (let j = from; j < s.length; j += 1) if (s[j] === c) return j; return -1; };
  const scan = (s: string[], marks: string[]): void => {
    let i = 0;
    let buf = '';
    const flush = (): void => { add(buf, marks); buf = ''; };
    while (i < s.length) {
      const c = s[i];
      if (c === '\\' && i + 1 < s.length) {
        if (s[i + 1] === '\n') { flush(); out.push({ type: 'hard_break' }); i += 2; continue; }
        buf += s[i + 1]; i += 2; continue;
      }
      if (c === ' ' && i + 2 < s.length && s[i + 1] === ' ' && s[i + 2] === '\n') { flush(); out.push({ type: 'hard_break' }); i += 3; continue; }
      if (c === '\n') { flush(); out.push({ type: 'hard_break' }); i += 1; continue; }      // ONE newline is a line break, as the server reads plain text (plainTextInlineNodes) and the web's Shift+Enter writes `<br>`; a blank line is a new paragraph (blocks())
      if (c === '`') {
        const j = indexFrom(s, '`', i + 1);
        if (j >= 0) { flush(); add(s.slice(i + 1, j).join(''), marks.includes('code') ? marks : [...marks, 'code']); i = j + 1; continue; }
      }
      if ((c === '*' || c === '_') && i + 1 < s.length && s[i + 1] === c) {
        const j = closer(s, i + 2, [c, c]);
        if (j !== null) { flush(); scan(s.slice(i + 2, j), marks.includes('strong') ? marks : [...marks, 'strong']); i = j + 2; continue; }
      }
      if ((c === '*' || c === '_') && i + 1 < s.length && s[i + 1] !== ' ') {
        const j = closer(s, i + 1, [c]);
        if (j !== null) { flush(); scan(s.slice(i + 1, j), marks.includes('emphasis') ? marks : [...marks, 'emphasis']); i = j + 1; continue; }
      }
      if (c === '[') {
        const close = indexFrom(s, ']', i + 1);
        if (close >= 0 && close + 1 < s.length && s[close + 1] === '(') {
          const end = indexFrom(s, ')', close + 2);
          if (end >= 0) { flush(); scan(s.slice(i + 1, close), marks); i = end + 1; continue; }
        }
      }
      buf += c; i += 1;
    }
    flush();
  };
  scan([...src], []);
  return out;
}

// ---------------------------------------------------------------------------------------------------------------- the merge

export type Merged = { body: Body; opaqueEdits: number };

const root = (content: Node[], like: Body | null): Body => {
  const out: Body = { ...(like ?? {}) };
  out.schema = typeof like?.schema === 'string' ? like.schema : 'choir.texture_doc.v1';
  const doc: Record<string, unknown> = isObj(like?.doc) ? { ...(like!.doc as Record<string, unknown>) } : { type: 'doc', attrs: { id: newID('doc') } };
  doc.content = content;
  out.doc = doc;
  return out;
};
export const topNodes = (body: Body | null): Node[] => (isObj(body?.doc) && Array.isArray((body!.doc as Record<string, unknown>).content) ? ((body!.doc as Record<string, unknown>).content as Node[]) : []);

/** The `body_doc` for `newMarkdown`, given the revision it was loaded from (`original`, `originalMarkdown`). */
export function merge(original: Body | null, originalMarkdown: string, newMarkdown: string, entities: Entity[] = []): Merged {
  if (original !== null && newMarkdown === originalMarkdown) return { body: original, opaqueEdits: 0 };
  const s = topNodes(original);
  const b = blocks(originalMarkdown);
  const n = blocks(newMarkdown);
  const blank = (nodes: Node[]): boolean => nodes.every((x) => x.type === 'paragraph' && plainText(x) === '');            // no text at all: nothing to keep, nothing to match
  if (original === null || (blank(s) && b.length === 0)) {
    const nodes = n.map(nodeFor);
    return { body: root(nodes.length === 0 ? [paragraph('')] : nodes, original), opaqueEdits: 0 };
  }
  // Which original node each original block is (forward, by what it says and what it is).
  const map: (number | null)[] = b.map(() => null);
  const texts = projectNodes(s, entities);                                                       // what the server projected for each node: an unchanged block IS its node's text (an embed's line, a cited paragraph's markers)
  let cursor = 0;
  for (let i = 0; i < b.length; i += 1) {
    const k = kindOf(b[i]);
    const kk = key(b[i]);
    for (let j = cursor; j < s.length; j += 1) {
      if (texts[j] === b[i] || (compatible(k, str(s[j].type)) && nodeKey(s[j]) === kk)) { map[i] = j; cursor = j + 1; break; }
    }
  }
  // Nodes no block claims (an embed, a source flow, an unknown node) stay after the block that came before them.
  const lead: Node[] = [];
  const attach = new Map<number, Node[]>();
  const blockOf = new Map<number, number>();
  map.forEach((j, i) => { if (j !== null) blockOf.set(j, i); });
  let lastClaimed: number | null = null;
  for (let j = 0; j < s.length; j += 1) {
    const i = blockOf.get(j);
    if (i !== undefined) lastClaimed = i;
    else if (lastClaimed !== null) { const a = attach.get(lastClaimed) ?? []; a.push(s[j]); attach.set(lastClaimed, a); }
    else lead.push(s[j]);
  }
  // Block diff (longest common subsequence on the exact source).
  const m = b.length;
  const p = n.length;
  const lcs: number[][] = Array.from({ length: m + 1 }, () => new Array<number>(p + 1).fill(0));
  for (let i = m - 1; i >= 0; i -= 1) for (let k = p - 1; k >= 0; k -= 1) lcs[i][k] = b[i] === n[k] ? lcs[i + 1][k + 1] + 1 : Math.max(lcs[i + 1][k], lcs[i][k + 1]);
  const pairs: [number, number][] = [];
  let i = 0;
  let k = 0;
  while (i < m && k < p) { if (b[i] === n[k]) { pairs.push([i, k]); i += 1; k += 1; } else if (lcs[i + 1][k] >= lcs[i][k + 1]) i += 1; else k += 1; }
  pairs.push([m, p]);
  let out: Node[] = [...lead];
  let opaque = 0;
  let pi = 0;
  let pk = 0;
  for (const [mi, mk] of pairs) {
    const deleted: number[] = [];
    for (let x = pi; x < mi; x += 1) deleted.push(x);
    const inserted: number[] = [];
    for (let x = pk; x < mk; x += 1) inserted.push(x);
    inserted.forEach((kk, t) => {
      const di = t < deleted.length ? map[deleted[t]] : null;
      const old: Node | null = di !== null && di !== undefined ? s[di] : null;
      if (old !== null && !WRITABLE.has(str(old.type))) { out.push(old); opaque += 1; return; }      // a table, an embed, an unknown node: kept; the edit lives in `content`
      const cited = old !== null && old.type === 'paragraph' && (Array.isArray(old.content) ? (old.content as Node[]) : []).some((x) => x.type === 'source_ref');
      const fresh = nodeFor(cited ? n[kk].replace(/\[source \d+: [^\]]*\]|\[\d+\]/g, '') : n[kk]);      // the markers are the refs' own: they are not typed text (the server refuses a literal "[1]"); the refs stay with the paragraph
      if (old !== null) carry(old, fresh);
      out.push(fresh);
    });
    for (const d of deleted) { const a = attach.get(d); if (a) out = out.concat(a); }           // what sat after a removed block stays
    if (mi < m) {
      const j = map[mi];
      if (j !== null) out.push(s[j]); else out.push(nodeFor(n[mk]));
      const a = attach.get(mi);
      if (a) out = out.concat(a);
    }
    pi = mi + 1; pk = mk + 1;
  }
  return { body: root(out.length === 0 ? [paragraph('')] : out, original), opaqueEdits: opaque };
}

/** An edited node keeps the id it had, and a paragraph keeps its citations (they stay at the end, where the web editor puts them). */
function carry(old: Node, fresh: Node): void {
  const id = isObj(old.attrs) ? (old.attrs as Record<string, unknown>).id : undefined;
  if (id !== undefined) fresh.attrs = { ...(isObj(fresh.attrs) ? (fresh.attrs as Record<string, unknown>) : {}), id };
  if (old.type !== 'paragraph' || fresh.type !== 'paragraph') return;
  const refs = (Array.isArray(old.content) ? (old.content as Node[]) : []).filter((x) => x.type === 'source_ref');
  if (refs.length > 0) fresh.content = [...(Array.isArray(fresh.content) ? (fresh.content as Node[]) : []), ...refs];
}

// ---------------------------------------------------------------------------------------------------------------- the server's projection

export type Entity = Record<string, unknown>;
/** The server's `stringAttr`: a string attribute, trimmed. */
const sattr = (n: Node, k: string): string => (isObj(n.attrs) && typeof (n.attrs as Record<string, unknown>)[k] === 'string' ? ((n.attrs as Record<string, unknown>)[k] as string).trim() : '');
/** The server's `intAttr`: a whole number (a JSON number with no fraction), else none. */
const iattr = (n: Node, k: string): number | null => { const v = isObj(n.attrs) ? (n.attrs as Record<string, unknown>)[k] : undefined; return typeof v === 'number' && Number.isInteger(v) ? v : null; };
/** What the server prints for an expanded source (`sourceTitle`): the entity's display title, else label, else its target id, else uri, else its own id. */
const entityTitle = (e: Entity | undefined, id: string): string => {
  const d = isObj(e?.display) ? (e!.display as Record<string, unknown>) : {};
  const g = isObj(e?.target) ? (e!.target as Record<string, unknown>) : {};
  for (const v of [d.title, d.label, g.id, g.uri]) if (typeof v === 'string' && v !== '') return v;
  return id;
};

/** The text of each top-level node as the server's projection writes it (numbers of sources by first appearance across the whole document). */
export function projectNodes(nodes: Node[], entities: Entity[] = []): string[] {
  const numbers = new Map<string, number>();
  const byId = new Map<string, Entity>(entities.map((e) => [str(e.source_entity_id).trim(), e]));
  const kids = (n: Node): Node[] => (Array.isArray(n.content) ? (n.content as Node[]) : []);
  const inlineOf = (list: Node[], out: { s: string }): void => {
    for (const n of list) {
      if (n.type === 'text') out.s += str(n.text);
      else if (n.type === 'hard_break') out.s += '\n';
      else if (n.type === 'source_ref') {
        const id = sattr(n, 'source_entity_id');
        let num = numbers.get(id);
        if (num === undefined) { num = numbers.size + 1; numbers.set(id, num); }
        if (sattr(n, 'display_mode') === 'expanded_ref') { out.s += `[source ${num}: ${entityTitle(byId.get(id), id)}]`; continue; }
        const marker = `[${num}]`;
        if (!out.s.replace(/ +$/, '').endsWith(marker)) out.s += marker;     // adjacent citations of one source read as one marker
      }
    }
  };
  const item = (n: Node, depth: number, out: { s: string }): void => {
    kids(n).forEach((c, i) => { if (i > 0) out.s += '\n' + '  '.repeat(depth + 1); block(c, depth + 1, out); });
  };
  const sequence = (list: Node[], depth: number, out: { s: string }): void => {
    list.forEach((n, i) => { block(n, depth, out); if (i < list.length - 1) out.s += '\n\n'; });
  };
  const block = (n: Node, depth: number, out: { s: string }): void => {
    switch (n.type) {
      case 'paragraph': inlineOf(kids(n), out); break;
      case 'heading': { const lv = iattr(n, 'level') ?? 0; out.s += '#'.repeat(Math.max(lv, 0)) + ' '; inlineOf(kids(n), out); break; }
      case 'bullet_list': kids(n).forEach((it, i, all) => { out.s += '  '.repeat(depth) + '- '; item(it, depth, out); if (i < all.length - 1) out.s += '\n'; }); break;
      case 'ordered_list': {
        const start = iattr(n, 'start') ?? 1;
        kids(n).forEach((it, i, all) => { out.s += '  '.repeat(depth) + `${start + i}. `; item(it, depth, out); if (i < all.length - 1) out.s += '\n'; });
        break;
      }
      case 'list_item': item(n, depth, out); break;
      case 'blockquote': {
        const inner = { s: '' };
        sequence(kids(n), depth, inner);
        out.s += inner.s.replace(/\n+$/, '').split('\n').map((l) => '> ' + l).join('\n');
        break;
      }
      case 'code_block': out.s += '```' + sattr(n, 'language') + '\n' + kids(n).map((c) => str(c.text)).join('\n') + '\n```'; break;
      case 'horizontal_rule': out.s += '---'; break;
      case 'embed': out.s += embedText(n); break;
    }
  };
  const embedText = (n: Node): string => {
    const a = (k: string): string => sattr(n, k);
    switch (a('kind')) {
      case 'capsule_evidence': return `[evidence: ${a('caption')}]`;
      case 'file': return `[file: ${a('title')}]`;
      case 'video': return a('caption') !== '' ? `[video: ${a('caption')}]` : `[video: ${a('alt')}]`;
      case 'audio': return `[audio: ${a('title')}]`;
      case 'image': return a('caption') !== '' ? `[image: ${a('caption')}]` : `[image: ${a('alt')}]`;
    }
    return '[embed]';
  };
  return nodes.map((n) => { const out = { s: '' }; block(n, 0, out); return out.s; });
}

/** The text the server derives from a body_doc (internal/texturedoc/projection.go `Project`): blocks a blank line apart, no inline marks, hard breaks as newlines, the end trimmed of newlines. The phone has no
 * source entities of its own; those of the revision it read come in `entities` (an expanded source reads its title there). */
export function projectBody(body: Body | null, entities: Entity[] = []): string {
  return projectNodes(topNodes(body), entities).join('\n\n').replace(/\n+$/, '');
}

/** The ids of the source entities a body cites. */
export function citedIds(body: Body | null): Set<string> {
  const out = new Set<string>();
  const walk = (n: Node): void => { if (n.type === 'source_ref') out.add(sattr(n, 'source_entity_id')); (Array.isArray(n.content) ? (n.content as Node[]) : []).forEach(walk); };
  topNodes(body).forEach(walk);
  return out;
}

/** A Markdown text as the server would hold it: the projection of the document the phone writes for it. Two texts are the same text when their `canon` is equal (a trailing newline, an Enter typed at the end, marks,
 * a continuation line's indent do not make a different text). */
export function canon(markdown: string): string {
  const nodes = blocks(markdown).map(nodeFor);
  return projectBody(root(nodes.length === 0 ? [paragraph('')] : nodes, null));
}
