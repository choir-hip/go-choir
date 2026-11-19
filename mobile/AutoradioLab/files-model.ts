// The Files page's pure data: kinds, labels, paths and addresses, sorting, CSV and the shapes the contract reads
// (files-types.contract is the same shapes in Contract; app.contract.d.ts is generated from it, so a field added there is a type error here).
// Nothing in this file reads the clock or the network: the clock is the server's `Date` header, passed in as `now` (epoch milliseconds).
//
// Ways this can fail (written before the code; files.test.ts has a test for each):
//  1. A path escapes the root ("..", an encoded "..", a NUL): normalize refuses it before any request.
//  2. A name with "/" or "." or "" is offered for a new folder, a rename or an upload: refused, nothing sent.
//  3. A file name with spaces, "?", "#" or "%" reaches a URL: every component is percent-encoded; the cache name is sanitised.
//  4. Two files with the same name in different folders (or a file replaced on the computer) share a cache entry: the key hashes path, size and modified time.
//  5. Sorting: equal names with different case, numbers inside names ("2" before "10"), folders mixed with files, an empty modified date. Folders first except in
//     recents; ties fall back to the path so the order is total.

export type FileKind = 'folder' | 'image' | 'video' | 'audio' | 'pdf' | 'epub' | 'markdown' | 'text' | 'code' | 'csv' | 'other';

const IMAGE = new Set(['png', 'jpg', 'jpeg', 'gif', 'webp', 'heic', 'heif', 'bmp', 'tif', 'tiff']);
const VIDEO = new Set(['mp4', 'm4v', 'mov']);
const AUDIO = new Set(['mp3', 'm4a', 'aac', 'wav', 'aif', 'aiff', 'caf', 'flac']);
const MARKDOWN = new Set(['md', 'markdown']);
const TEXT = new Set(['txt', 'log', 'text']);
const CSV = new Set(['csv', 'tsv']);
// svg is code on purpose: SVG does not draw as an image on Apple.
const CODE = new Set(['swift', 'ts', 'tsx', 'js', 'jsx', 'mjs', 'json', 'py', 'go', 'rs', 'c', 'h', 'cpp', 'java', 'kt', 'sh', 'yml', 'yaml', 'toml', 'xml', 'html', 'css', 'sql', 'svg', 'contract', 'rb', 'lua']);

export function extOf(name: string): string {
  const i = name.lastIndexOf('.');
  return i <= 0 || i === name.length - 1 ? '' : name.slice(i + 1).toLowerCase();
}

export function kindOf(name: string, isDirectory: boolean): FileKind {
  if (isDirectory) return 'folder';
  const e = extOf(name);
  if (IMAGE.has(e)) return 'image';
  if (VIDEO.has(e)) return 'video';
  if (AUDIO.has(e)) return 'audio';
  if (e === 'pdf') return 'pdf';
  if (e === 'epub') return 'epub';
  if (MARKDOWN.has(e)) return 'markdown';
  if (CSV.has(e)) return 'csv';
  if (TEXT.has(e)) return 'text';
  if (CODE.has(e)) return 'code';
  return 'other';
}

const LABELS: Record<FileKind, string> = { folder: 'Folder', image: 'Image', video: 'Video', audio: 'Audio', pdf: 'PDF', epub: 'Book', markdown: 'Markdown', text: 'Text', code: 'Code', csv: 'Table', other: 'File' };
export const kindLabel = (k: FileKind): string => LABELS[k];

const SYMBOLS: Record<FileKind, string> = {
  folder: 'symbol:sf/folder', image: 'symbol:sf/photo', video: 'symbol:sf/film', audio: 'symbol:sf/waveform', pdf: 'symbol:sf/doc.richtext', epub: 'symbol:sf/book',
  markdown: 'symbol:sf/doc.text', text: 'symbol:sf/doc.plaintext', code: 'symbol:sf/chevron.left.forwardslash.chevron.right', csv: 'symbol:sf/tablecells', other: 'symbol:sf/doc',
};
export const symbolOf = (k: FileKind): string => SYMBOLS[k];

/** The kinds that open as another page: a PDF or an e-book is read by the listener's computer (decision D4), text, Markdown and code open in Texture. */
export const opensElsewhere = (k: FileKind): boolean => k === 'pdf' || k === 'epub' || k === 'markdown' || k === 'text' || k === 'code';

/** The most this phone brings into the app's memory to open a kind. A reply is held in memory (the shared code has no file stream through the signed-in
 * seam yet), so the caps are smaller than the Swift app's (which streamed to disk). Beyond a cap the page shows the facts and says why. */
export function maxBytes(k: FileKind): number {
  switch (k) {
    case 'markdown': case 'text': case 'code': case 'csv': return 5_000_000;
    case 'image': return 32_000_000;
    case 'audio': return 48_000_000;
    case 'video': return 48_000_000;
    default: return 0;
  }
}

const MIME: Record<string, string> = {
  png: 'image/png', jpg: 'image/jpeg', jpeg: 'image/jpeg', gif: 'image/gif', webp: 'image/webp', heic: 'image/heic', heif: 'image/heic', pdf: 'application/pdf',
  epub: 'application/epub+zip', mp4: 'video/mp4', m4v: 'video/mp4', mov: 'video/quicktime', mp3: 'audio/mpeg', m4a: 'audio/mp4', aac: 'audio/mp4', wav: 'audio/wav',
  md: 'text/markdown', markdown: 'text/markdown', csv: 'text/csv', txt: 'text/plain', log: 'text/plain', json: 'application/json',
};
export const mimeOf = (name: string): string => MIME[extOf(name)] ?? 'application/octet-stream';

// ---------------------------------------------------------------- paths and addresses

/** A canonical path: a leading "/", no empty or "." parts, no trailing "/" (but "/" itself). Null when it climbs out of the root with ".." or holds a NUL. */
export function normalize(p: string): string | null {
  if (p.includes('\0')) return null;
  const parts: string[] = [];
  for (const s of p.split('/')) {
    if (s === '' || s === '.') continue;
    if (s === '..') { if (parts.length === 0) return null; parts.pop(); continue; }
    parts.push(s);
  }
  return '/' + parts.join('/');
}
export const join = (dir: string, name: string): string => (dir === '/' || dir === '' ? '/' + name : dir + '/' + name);
export function parent(p: string): string { const i = p.lastIndexOf('/'); return i <= 0 ? '/' : p.slice(0, i); }
export function nameOf(p: string): string { const parts = p.split('/').filter((s) => s !== ''); return parts.length ? parts[parts.length - 1] : ''; }

/** A usable name for a new folder, a rename or an upload. */
export function validName(n: string): boolean {
  return n.length > 0 && n.trim() === n && n !== '.' && n !== '..' && !n.includes('/') && !n.includes('\0') && new TextEncoder().encode(n).length <= 255;
}

/** The page address of a path: a folder ends with "/". */
export function addressOf(path: string, isDirectory: boolean): string {
  if (path === '/') return 'file:/';
  return 'file:' + path + (isDirectory ? '/' : '');
}

/** The path and kind an address names, or null for a malformed one. */
export function fromAddress(address: string): { path: string; isDirectory: boolean } | null {
  if (!address.startsWith('file:')) return null;
  const rest = address.slice(5);
  const p = normalize(rest === '' ? '/' : rest);
  if (p === null) return null;
  return { path: p, isDirectory: rest === '' || rest.endsWith('/') };
}

export type Crumb = { name: string; path: string };
export function crumbsOf(path: string): Crumb[] {
  const out: Crumb[] = [{ name: 'Files', path: '/' }];
  let cur = '/';
  for (const part of path.split('/').filter((s) => s !== '')) { cur = join(cur, part); out.push({ name: part, path: cur }); }
  return out;
}

const UNRESERVED = /[A-Za-z0-9\-._~]/;
function pct(s: string): string {
  let out = '';
  for (const ch of s) {
    if (UNRESERVED.test(ch)) { out += ch; continue; }
    for (const b of new TextEncoder().encode(ch)) out += '%' + (b < 16 ? '0' : '') + b.toString(16).toUpperCase();
  }
  return out;
}
/** Percent-encode each component (so "?", "#", "%" and spaces stay in the name). */
export const encodePath = (path: string): string => path.split('/').filter((s) => s !== '').map((s) => '/' + pct(s)).join('');

/** A file name safe for the cache and for an `app:/` path used in a src: ASCII letters, digits and ".-_" only. */
export function cacheName(name: string): string {
  const e = extOf(name);
  let stem = e === '' ? name : name.slice(0, name.length - e.length - 1);
  stem = Array.from(stem).map((c) => (/[A-Za-z0-9_-]/.test(c) ? c : '-')).join('');
  if (stem === '' || /^-+$/.test(stem)) stem = 'file';
  const safeExt = e.replace(/[^a-z0-9]/g, '').slice(0, 8);
  return stem.slice(0, 60) + (safeExt === '' ? '' : '.' + safeExt);
}

/** A 64-bit FNV-1a of the string as hex, in two 32-bit halves (no BigInt). The cache folder's name. */
export function hash(s: string): string {
  let a = 0x811c9dc5, b = 0xcbf29ce4;
  for (const byte of new TextEncoder().encode(s)) {
    a ^= byte; a = Math.imul(a, 0x01000193) >>> 0;
    b ^= byte; b = Math.imul(b, 0x01000193) >>> 0;
    b = (b + a) >>> 0;
  }
  return a.toString(16).padStart(8, '0') + b.toString(16).padStart(8, '0');
}

// ---------------------------------------------------------------- labels

/** "0 B", "812 B", "1.2 kB", "34 kB", "5.6 MB" (decimal units, one decimal under 10). */
export function sizeLabel(n: number): string {
  if (n < 1000) return `${Math.max(0, Math.trunc(n))} B`;
  const units = ['kB', 'MB', 'GB', 'TB'];
  let v = n / 1000, i = 0;
  while (v >= 1000 && i < units.length - 1) { v /= 1000; i++; }
  return (v < 10 ? v.toFixed(1) : v.toFixed(0)) + ' ' + units[i];
}

const MONTHS = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];
const DAY = 86_400_000;
/** "Today", "Yesterday", "3 Nov", "3 Nov 2025". Fixed English and UTC, so the same file reads the same everywhere. `now` is the server's clock. */
export function dateLabel(ms: number | null, now: number): string {
  if (ms === null) return '';
  const day = (t: number) => Math.floor(t / DAY);
  if (day(ms) === day(now)) return 'Today';
  if (day(ms) === day(now) - 1) return 'Yesterday';
  const d = new Date(ms), n = new Date(now);
  return `${d.getUTCDate()} ${MONTHS[d.getUTCMonth()]}` + (d.getUTCFullYear() === n.getUTCFullYear() ? '' : ` ${d.getUTCFullYear()}`);
}

/** An RFC 3339 time (fractions of any length) as epoch milliseconds; null when it is not one. */
export function parseDate(s: unknown): number | null {
  if (typeof s !== 'string') return null;
  const m = /^(\d{4})-(\d{2})-(\d{2})[Tt](\d{2}):(\d{2}):(\d{2})(?:\.(\d+))?([Zz]|[+-]\d{2}:\d{2})$/.exec(s);
  if (!m) return null;
  const frac = m[7] ? Math.floor(Number('0.' + m[7]) * 1000) : 0;
  let t = Date.UTC(+m[1], +m[2] - 1, +m[3], +m[4], +m[5], +m[6], frac);
  if (m[8].toUpperCase() !== 'Z') { const sign = m[8][0] === '-' ? -1 : 1; t -= sign * (+m[8].slice(1, 3) * 60 + +m[8].slice(4, 6)) * 60_000; }
  return Number.isFinite(t) ? t : null;
}

// ---------------------------------------------------------------- sorting

export type Entry = { name: string; path: string; isDirectory: boolean; size: number; modified: number | null };
export type SortKey = 'name' | 'modified' | 'size' | 'kind';
export const sortKey = (s: string): SortKey => (s === 'modified' || s === 'size' || s === 'kind' ? s : 'name');

const fold = (s: string): string => s.normalize('NFD').replace(/[̀-ͯ]/g, '').toLowerCase();
/** Finder-like order: case and accents ignored, runs of digits compared as numbers ("2" before "10"). */
export function standardCompare(a: string, b: string): number {
  const ta = fold(a).match(/\d+|\D+/g) ?? [], tb = fold(b).match(/\d+|\D+/g) ?? [];
  for (let i = 0; i < Math.min(ta.length, tb.length); i++) {
    const x = ta[i], y = tb[i];
    if (/^\d/.test(x) && /^\d/.test(y)) {
      const d = Number(x) - Number(y);
      if (d !== 0) return d < 0 ? -1 : 1;
    } else if (x !== y) return x < y ? -1 : 1;
  }
  return ta.length === tb.length ? 0 : ta.length < tb.length ? -1 : 1;
}
const cmp = (a: number | string, b: number | string): number => (a === b ? 0 : a < b ? -1 : 1);

/** Folders first, then by the key (ascending: names A-Z, oldest first, smallest first, kind labels A-Z). `desc` flips the key, not the folders-first rule.
 * Every comparison falls back to the path, so the order is total. */
export function sortEntries(entries: Entry[], key: SortKey, desc: boolean, foldersFirst = true): Entry[] {
  const byName = (a: Entry, b: Entry) => standardCompare(a.name, b.name);
  const keyed = (a: Entry, b: Entry): number => {
    switch (key) {
      case 'name': return byName(a, b);
      case 'modified': return cmp(a.modified ?? -Infinity, b.modified ?? -Infinity);
      case 'size': return cmp(a.size, b.size);
      case 'kind': { const r = cmp(kindLabel(kindOf(a.name, a.isDirectory)), kindLabel(kindOf(b.name, b.isDirectory))); return r === 0 ? byName(a, b) : r; }
    }
  };
  return entries.slice().sort((a, b) => {
    if (foldersFirst && a.isDirectory !== b.isDirectory) return a.isDirectory ? -1 : 1;
    let r = keyed(a, b);
    if (r === 0) r = cmp(a.path, b.path); else if (desc) r = -r;
    return r;
  });
}

// ---------------------------------------------------------------- CSV

/** RFC 4180: quoted fields, doubled quotes, the separator inside quotes, CRLF or LF, a last line with no newline, a ragged row (kept as is). A stray quote
 * inside an unquoted field stays literal. At most `maxRows` rows. */
export function parseCsv(text: string, separator = ',', maxRows = 500): string[][] {
  const rows: string[][] = [];
  let row: string[] = [], field = '', inQuotes = false;
  const chars = Array.from(text);
  for (let i = 0; i < chars.length; i++) {
    const c = chars[i];
    if (inQuotes) {
      if (c === '"') { if (chars[i + 1] === '"') { field += '"'; i++; } else inQuotes = false; }
      else field += c;
    } else if (c === '"' && field === '') inQuotes = true;
    else if (c === separator) { row.push(field); field = ''; }
    else if (c === '\n' || c === '\r') {
      if (c === '\r' && chars[i + 1] === '\n') i++;
      row.push(field); field = '';
      if (!(row.length === 1 && row[0] === '')) rows.push(row);
      row = [];
      if (rows.length >= maxRows) return rows;
    } else field += c;
  }
  if (field !== '' || row.length > 0) { row.push(field); rows.push(row); }
  return rows;
}

/** Drop a UTF-8 sequence a cut left unfinished (and only that: a character that ends exactly at the cut stays). */
export function withoutPartialUTF8(b: Uint8Array): Uint8Array {
  let i = b.length - 1, back = 0;
  while (i >= 0 && back < 3 && (b[i] & 0xc0) === 0x80) { i--; back++; }
  if (i < 0) return b;
  const lead = b[i];
  const need = lead >= 0xf0 ? 4 : lead >= 0xe0 ? 3 : lead >= 0xc0 ? 2 : 1;
  return b.length - i < need ? b.slice(0, i) : b;
}

// ---------------------------------------------------------------- shapes (the same as files-types.contract)

export type FileRow = { name: string; path: string; address: string; kind: string; kindLabel: string; symbol: string; isDir: boolean; size: number; sizeLabel: string; modifiedLabel: string; detail: string; place: string };
export type FileListing = { path: string; title: string; crumbs: Crumb[]; rows: FileRow[]; count: number; status: string; message: string; mode: string; sort: string; desc: boolean; query: string };
export type FileInfo = {
  address: string; path: string; folder: string; name: string; kind: string; kindLabel: string; size: number; sizeLabel: string; modifiedLabel: string; mime: string;
  status: string; message: string; src: string; local: string; truncated: boolean; rows: { cells: string[] }[]; width: number; height: number; target: string;
};
/** What opening a file that opens as another page answered: where to go, or why not. */
export type FileResolved = { address: string; target: string; title: string; status: string; message: string };

export const emptyInfo = (): FileInfo => ({
  address: '', path: '', folder: '/', name: '', kind: 'other', kindLabel: 'File', size: 0, sizeLabel: '', modifiedLabel: '', mime: '', status: 'empty', message: '', src: '', local: '',
  truncated: false, rows: [], width: 0, height: 0, target: '',
});
export const emptyResolved = (): FileResolved => ({ address: '', target: '', title: '', status: 'empty', message: '' });

export function rowOf(e: Entry, now: number, place = ''): FileRow {
  const k = kindOf(e.name, e.isDirectory);
  const date = dateLabel(e.modified, now);
  return {
    name: e.name, path: e.path, address: addressOf(e.path, e.isDirectory), kind: k, kindLabel: kindLabel(k), symbol: symbolOf(k), isDir: e.isDirectory, size: e.size,
    sizeLabel: e.isDirectory ? '' : sizeLabel(e.size), modifiedLabel: date, detail: e.isDirectory ? date : [sizeLabel(e.size), date].filter((s) => s !== '').join(' · '), place,
  };
}
