// The imported documents' pure half: PDFs and e-books the person imports to read and hear. An imported document is a FILE ON THE PERSON'S COMPUTER (in the
// folder FOLDER, put there through the Files client, files.ts); the computer reads its text (internal/content/extract.go: pdftotext, pandoc) and this file turns
// that text into sections and blocks the reader page draws, searches and hands to the player. Nothing here reads the clock, the network or a file.
// It replaces the Swift ReaderShelf.swift and Epub.swift (a zip and XML parser, PDFKit): the computer does the parsing now.
import * as F from './files-model.ts';

/** The folder of the person's computer the imported documents live in. */
export const FOLDER = '/Imported documents';

export type DocKind = 'pdf' | 'epub';
export type Block = { kind: 'heading' | 'paragraph' | 'quote' | 'item' | 'code'; level: number; text: string; marker: string; section: number };
export type Section = { title: string; start: number; count: number };
export type DocText = { sections: Section[]; blocks: Block[] };

// ---------------------------------------------------------------- names and addresses

export function kindOfName(name: string): DocKind | null {
  const e = F.extOf(name);
  return e === 'pdf' || e === 'epub' ? e : null;
}
/** The id of a document is its file name, percent-encoded: no "@" (an address's block mark) and no "/" in it. */
export const idOf = (name: string): string => encodeURIComponent(name);
export function nameOfId(id: string): string | null {
  if (id === '') return null;
  try { const n = decodeURIComponent(id); return n === '' ? null : n; } catch { return null; }
}
export const addressOf = (id: string, block?: number): string => 'doc:' + id + (block === undefined ? '' : '@' + Math.trunc(block));
/** "doc:<id>" or "doc:<id>@<block>". */
export function parseAddress(address: string): { id: string; block: number | null } | null {
  if (!address.startsWith('doc:')) return null;
  const rest = address.slice(4), at = rest.indexOf('@');
  const id = at < 0 ? rest : rest.slice(0, at);
  if (id === '') return null;
  const tail = at < 0 ? '' : rest.slice(at + 1);
  return { id, block: /^\d+$/.test(tail) ? parseInt(tail, 10) : null };
}
/** `name` unless taken; else "stem 2.ext", "stem 3.ext". */
export function uniqueName(name: string, taken: Set<string>): string {
  if (!taken.has(name)) return name;
  const dot = name.lastIndexOf('.');
  const stem = dot > 0 ? name.slice(0, dot) : name, ext = dot > 0 ? name.slice(dot) : '';
  for (let n = 2; ; n++) { const c = `${stem} ${n}${ext}`; if (!taken.has(c)) return c; }
}
export function titleOf(name: string): string {
  const dot = name.lastIndexOf('.');
  return (dot > 0 ? name.slice(0, dot) : name).replace(/_/g, ' ').trim();
}

// ---------------------------------------------------------------- text to blocks

/** A block's text has Markdown marks, links, images and attribute groups; the reader keeps plain words. A backslash escapes the next character. */
export function plain(s: string): string {
  let t = s;
  t = t.replace(/(?<!\\)!\[[^\]]*\]\([^)]*\)(\{[^}]*\})?/g, '');          // an image has no words
  t = t.replace(/(?<!\\)\[\^[^\]]*\]/g, '');                               // a footnote mark
  t = t.replace(/(?<!\\)\[([^\]]*)\]\([^)]*\)/g, '$1');                   // [text](url "title")
  t = t.replace(/(?<!\\)\[([^\]]*)\]\[[^\]]*\]/g, '$1');                  // [text][ref]
  t = t.replace(/(?<!\\)\[([^\]]*)\]\{[^}]*\}/g, '$1');                   // [text]{.class}
  t = t.replace(/(?<![\\\w])\{[#.][^}]*\}/g, '');                          // {#id .class}
  t = t.replace(/<\/?[A-Za-z][^>]*>/g, '');                                // html tags
  let out = '';
  for (let i = 0; i < t.length; i++) {
    const c = t[i];
    if (c === '\\') { if (i + 1 < t.length) { out += t[i + 1]; i++; } else out += ' '; }
    else if (c === '*' || c === '`') continue;
    else out += c;
  }
  return out.replace(/\s+/g, ' ').trim();
}

function cut(s: string, about: number): string[] {
  const out: string[] = [];
  let cur = '', sentence = '';
  for (const ch of s) {
    sentence += ch;
    if ('.!?'.includes(ch)) { cur += sentence; sentence = ''; if (cur.length >= about) { out.push(cur.trim()); cur = ''; } }
  }
  cur += sentence;
  if (cur.trim() !== '') out.push(cur.trim());
  return out;
}

/** One page's text as paragraphs: a blank line (spaces allowed) ends a paragraph; otherwise lines are joined (a hyphen at the end of a line joins the word
 * when the next starts in lower case); a page with no blank lines is cut at sentence ends into pieces of about 500 characters. */
export function paragraphs(page: string): string[] {
  const text = page.replace(/\r\n?/g, '\n');
  const join = (chunk: string): string => {
    let out = '';
    for (const line of chunk.split('\n')) {
      const l = line.trim();
      if (l === '') continue;
      if (out.endsWith('-') && /^\p{Ll}/u.test(l)) out = out.slice(0, -1) + l;
      else if (out === '') out = l;
      else out += ' ' + l;
    }
    return out;
  };
  let parts = text.split(/\n[ \t]*\n/).map(join).filter((p) => p !== '');
  if (parts.length === 1 && parts[0].length > 900) parts = cut(parts[0], 500);
  return parts;
}

/** The blocks per section when a document has no better sections of its own. */
export const PART_BLOCKS = 40;

/** Sections of PART_BLOCKS blocks each, titled "Part N". */
function parts(blocks: Block[]): DocText {
  const sections: Section[] = [];
  for (let i = 0; i < blocks.length; i += PART_BLOCKS) {
    const count = Math.min(PART_BLOCKS, blocks.length - i);
    sections.push({ title: `Part ${sections.length + 1}`, start: i, count });
    for (let k = i; k < i + count; k++) blocks[k].section = sections.length - 1;
  }
  return { sections, blocks };
}

/** A PDF's text (the computer's pdftotext: a form feed ends a page): a section per page that has words, titled "Page N" with N the real page number. */
export function fromPdfText(text: string): DocText {
  const sections: Section[] = [], blocks: Block[] = [];
  const pages = text.split('\f');
  if (pages.length === 1) {
    for (const p of paragraphs(text)) blocks.push({ kind: 'paragraph', level: 0, text: p, marker: '', section: 0 });
    return parts(blocks);
  }
  pages.forEach((page, i) => {
    const ps = paragraphs(page);
    if (ps.length === 0) return;
    const start = blocks.length;
    for (const p of ps) blocks.push({ kind: 'paragraph', level: 0, text: p, marker: '', section: sections.length });
    sections.push({ title: `Page ${i + 1}`, start, count: ps.length });
  });
  return { sections, blocks };
}

/** An e-book's text as the computer's pandoc writes it (Markdown). Chapters are the level of heading that repeats. */
export function fromMarkdown(md: string): DocText {
  const blocks: Block[] = [];
  const push = (kind: Block['kind'], level: number, raw: string, marker = ''): void => {
    const text = kind === 'code' ? raw : plain(raw);
    if (text !== '') blocks.push({ kind, level, text, marker, section: 0 });
  };
  let para: string[] = [], quote: string[] = [], item: { marker: string; text: string } | null = null, code: string[] | null = null;
  const flushItem = (): void => { if (item) { push('item', 0, item.text, item.marker); item = null; } };
  const flush = (): void => {
    if (para.length > 0) push('paragraph', 0, para.join(' '));
    if (quote.length > 0) push('quote', 0, quote.join(' '));
    para = []; quote = []; flushItem();
  };
  for (const raw of md.replace(/\r\n?/g, '\n').split('\n')) {
    const t = raw.trim();
    if (code) {
      if (/^(```|~~~)/.test(t)) { push('code', 0, code.join('\n')); code = null; } else code.push(raw);
      continue;
    }
    if (/^(```|~~~)/.test(t)) { flush(); code = []; continue; }
    if (t === '') { flush(); continue; }
    if (/^:{3,}/.test(t)) { flush(); continue; }                                     // a pandoc div
    if (/^([-*_]\s*){3,}$/.test(t)) { flush(); continue; }                           // a rule
    if (/^=+$/.test(t) && para.length === 1) { const h = para[0]; para = []; push('heading', 1, h); continue; }
    const h = /^(#{1,6})\s+(.*?)(\s+#+)?$/.exec(t);
    if (h) { flush(); push('heading', h[1].length, h[2]); continue; }
    if (/^!\[[^\]]*\]\([^)]*\)(\{[^}]*\})?$/.test(t)) { flush(); continue; }         // a picture on a line of its own
    if (/^[+|][-=+| :]*$/.test(t)) continue;                                         // a table's rule
    const li = /^(\s*)([-*+•]|\d+[.)])\s+(.*)$/.exec(raw);
    if (li) { flush(); item = { marker: /^\d/.test(li[2]) ? li[2].replace(')', '.') + ' ' : '• ', text: li[3] }; continue; }
    if (item && /^\s+\S/.test(raw)) { item.text += ' ' + t; continue; }
    if (t.startsWith('>')) { if (para.length > 0) { push('paragraph', 0, para.join(' ')); para = []; } flushItem(); quote.push(t.replace(/^>\s?/, '')); continue; }
    if (quote.length > 0) { push('quote', 0, quote.join(' ')); quote = []; }
    flushItem();
    para.push(t);
  }
  flush();
  if (code) push('code', 0, (code as string[]).join('\n'));

  const headings = blocks.map((b, i) => ({ b, i })).filter((x) => x.b.kind === 'heading');
  let level = 0;
  for (let l = 1; l <= 4 && level === 0; l++) if (headings.filter((x) => x.b.level === l).length >= 2) level = l;
  if (level === 0) return parts(blocks);
  const sections: Section[] = [];
  let cur: Section | null = null;
  blocks.forEach((b, i) => {
    const chapter = b.kind === 'heading' && b.level === level;
    if (!cur || chapter) {
      // Text before the first chapter is the front matter: titled by the heading that opens it (a book's title), else "Beginning".
      cur = { title: chapter || b.kind === 'heading' ? b.text : 'Beginning', start: i, count: 0 };
      sections.push(cur);
    }
    cur.count++; b.section = sections.length - 1;
  });
  return { sections, blocks };
}

export function sectionOf(t: DocText, block: number): number {
  return t.blocks.length === 0 ? 0 : t.blocks[Math.min(Math.max(block, 0), t.blocks.length - 1)].section;
}
export const clampBlock = (block: number, length: number): number => Math.min(Math.max(Math.trunc(block), 0), Math.max(0, length - 1));

/** The words to read aloud from `block` to the end, one line per block. */
export function spokenFrom(t: DocText, block: number): string[] {
  if (t.blocks.length === 0) return [];
  return t.blocks.slice(clampBlock(block, t.blocks.length)).map((b) => b.text.replace(/\s*\n\s*/g, ' '));
}
/** Where reading aloud starts: the chosen block when it lies in the section on screen, else that section's first block; null when there is nothing to read. */
export function startBlock(t: DocText, section: number, block: number): number | null {
  if (t.sections.length === 0) return null;
  const s = t.sections[Math.min(Math.max(Math.trunc(section), 0), t.sections.length - 1)];
  return block >= s.start && block < s.start + s.count ? block : s.start;
}

// ---------------------------------------------------------------- search

const fold = (s: string): string => s.normalize('NFKD').replace(/\p{M}/gu, '').toLowerCase();
/** Lower case, accents and widths folded; anything that is not a letter or digit separates. */
export const tokens = (s: string): string[] => fold(s).split(/[^\p{L}\p{N}]+/u).filter((t) => t !== '');

export type Index = { sets: Set<string>[]; lists: string[][]; norms: string[] };
export function buildIndex(blocks: Block[]): Index {
  const lists = blocks.map((b) => tokens(b.text));
  return { lists, sets: lists.map((l) => new Set(l)), norms: lists.map((l) => l.join(' ')) };
}
/** Blocks holding every word of the query (the last word may be the start of a word), phrase matches first, then in order. */
export function search(ix: Index, query: string, limit: number): number[] {
  const q = tokens(query);
  if (q.length === 0 || limit <= 0) return [];
  const phrase = q.join(' ');
  const scored: [number, number][] = [];
  for (let i = 0; i < ix.sets.length; i++) {
    const ok = q.every((t, n) => (n === q.length - 1 ? ix.lists[i].some((x) => x.startsWith(t)) : ix.sets[i].has(t)));
    if (!ok) continue;
    scored.push([i, q.length === 1 || ix.norms[i].includes(phrase) ? 2 : 1]);
  }
  scored.sort((a, b) => (a[1] !== b[1] ? b[1] - a[1] : a[0] - b[0]));
  return scored.slice(0, limit).map((s) => s[0]);
}

/** The words around the first match (60 before, 80 after), with an ellipsis where it is cut; no match is the start of the text. */
export function snippet(text: string, query: string): string {
  const one = text.split(/\s+/).filter((w) => w !== '').join(' ');
  const word = tokens(query)[0];
  const head = (): string => (one.length > 140 ? one.slice(0, 139) + '…' : one);
  if (!word) return head();
  let folded = '';
  const at: number[] = [];                                   // folded offset -> offset in `one`
  for (let i = 0; i < one.length; i++) { const f = fold(one[i]); for (let k = 0; k < f.length; k++) at.push(i); folded += f; }
  const hit = folded.indexOf(word);
  if (hit < 0) return head();
  const pos = at[hit];
  const from = Math.max(0, pos - 60), to = Math.min(one.length, pos + 80);
  return (from > 0 ? '…' : '') + one.slice(from, to) + (to < one.length ? '…' : '');
}

// ---------------------------------------------------------------- places and labels

export type Place = { block: number; length: number };
export const progressOf = (p: Place | undefined): number => (p && p.length > 1 ? Math.min(1, Math.max(0, p.block / (p.length - 1))) : 0);
export function progressLabel(p: Place | undefined): string {
  const x = progressOf(p);
  return x > 0 ? `${Math.round(x * 100)}% read` : 'Not started';
}
export const kindLabel = (k: DocKind): string => (k === 'pdf' ? 'PDF' : 'EPUB');
