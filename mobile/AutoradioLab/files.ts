// The Files page's data: the listener's own computer's files (folders, search, recent), the open file the viewers read, the changes (new folder, delete,
// rename, add a file) and what opening a file that is another page's document does. Moved from Swift (Kit/ChoirFiles.swift and its neighbours); the
// pure parts are in files-model.ts. Routes (internal/autoputer/files.go, custodian ownerfiles), all through the signed-in seam in api.ts:
//   GET    /api/files[/{path}]   a folder's array of {name, type: "file"|"directory", size, modified}; a file's bytes (?disposition=inline)
//   POST   /api/files/{path}     make a folder         PUT /api/files/{path}   write a file (raw body)
//   DELETE /api/files/{path}     delete                PATCH /api/files/{path} {"move_to"}   rename a regular file only
//   POST   /api/texture/files/open {source_path, title, initial_content} -> {doc_id}: the computer opens the file as a Texture document. A PDF or an
//          e-book is read by the computer (decision D4: it extracts the text; the phone shows and speaks it), text, Markdown and code open the same way.
// Logs carry counts and kinds, never a file name. No source throws for a failure the page has words for: it answers its own status and message.
import type { Storage } from './app.contract.d.ts';
import { WORDS } from './words.ts';
import { authedJSON, authedBytes, authedToFile, SignedOutError, putFile, accountGeneration, scoped, currentWorld, appPath } from './api.ts';
import * as M from './files-model.ts';
import type { Entry, FileInfo, FileListing, FileResolved } from './files-model.ts';

export const RECENT_FILE = 'app:/data/recent-files.json';
export const CACHE_DIR = 'app:/cache/files';
export const RECENT_LIMIT = 30;
const SEARCH_FOLDERS = 80, SEARCH_DEPTH = 4, SEARCH_FOUND = 200, TEXTURE_CAP = 1_000_000;

type Opts = Omit<Parameters<typeof authedJSON>[1], 'world'>;
export type Net = { json: (path: string, opts?: Opts) => ReturnType<typeof authedJSON>; bytes: (path: string, opts?: Opts) => ReturnType<typeof authedBytes>; toFile: (path: string, dest: string, opts?: Opts, storage?: Storage) => Promise<{ bytes: number; contentType: string }> };
// Every request states its world: the one current when it is made (a chain that captured account data first passes the world it began in).
export const realNet: Net = { json: (path, opts) => authedJSON(path, { ...opts, world: currentWorld() }), bytes: (path, opts) => authedBytes(path, { ...opts, world: currentWorld() }), toFile: (path, dest, opts, storage) => authedToFile(path, dest, storage, { ...opts, world: currentWorld() }) };

/** What the page may say when a file cannot be had. */
export class FilesError extends Error {
  constructor(public kind: 'notFound' | 'exists' | 'tooLarge' | 'unsupported' | 'badName' | 'notADirectory' | 'server', message: string) { super(message); }
}
export const say = (e: unknown): string => {
  if (e instanceof FilesError) return e.message;
  if (e instanceof SignedOutError) return e.reason === 'locked' ? WORDS.locked : e.reason === 'unknown' ? WORDS.unknown : e.reason === 'refused' ? 'Sign in again to see your files.' : 'Sign in to see your files.';
  const status = (e as { status?: number })?.status;
  if (status === 401) return 'Sign in again to see your files.';
  if (status === 403) return 'This device is not allowed to read files.';
  if (status === 404) return 'That file is not there any more.';
  if (status === 409) return 'Something with that name is already there.';
  if (status === 405) return 'This computer does not allow that here yet.';
  if (typeof status === 'number') return 'Could not read your files.';
  return 'Could not reach your computer. Check the connection and try again.';
};
const BAD_NAME = 'That name cannot be used. Avoid slashes and empty names.';

// ---------------------------------------------------------------- backends

type Backend = {
  list(path: string): Promise<{ entries: Entry[]; now: number | null }>;
  fetchBytes(path: string): Promise<Uint8Array>;
  /** The file at `path` into the app file `dest`, the bytes never in JavaScript; the size written. */
  download(path: string, dest: string, storage: Storage): Promise<number>;
  put(path: string, bytes: Uint8Array): Promise<void>;
  mkdir(path: string): Promise<void>;
  remove(path: string): Promise<void>;
  move(from: string, to: string): Promise<void>;
};

const route = (path: string): string => (path === '/' ? '/api/files' : '/api/files' + M.encodePath(path));

function choirBackend(net: Net): Backend {
  return {
    async list(path) {
      const r = await net.json(route(path));
      if (!Array.isArray(r.body)) throw new Error('not a file listing');
      const entries: Entry[] = [];
      for (const o of r.body as Record<string, unknown>[]) {
        const name = o?.name;
        if (typeof name !== 'string' || !M.validName(name)) continue;
        entries.push({ name, path: M.join(path, name), isDirectory: o.type === 'directory', size: typeof o.size === 'number' ? o.size : 0, modified: M.parseDate(o.modified) });
      }
      return { entries, now: r.serverTime };
    },
    async fetchBytes(path) { return (await net.bytes(route(path), { query: { disposition: 'inline' } })).bytes; },
    async download(path, dest, storage) { return (await net.toFile(route(path), dest, { query: { disposition: 'inline' } }, storage)).bytes; },
    async put(path, bytes) { await net.json(route(path), { method: 'PUT', body: bytes, headers: { 'content-type': M.mimeOf(M.nameOf(path)) } }); },
    async mkdir(path) { await net.json(route(path), { method: 'POST' }); },
    async remove(path) { await net.json(route(path), { method: 'DELETE' }); },
    async move(from, to) { await net.json(route(from), { method: 'PATCH', body: { move_to: to } }); },
  };
}

// ---------------------------------------------------------------- state between answers (cleared by forget())

type Recent = { path: string; size: number; modified: number | null; openedAt: number };
const S = scoped(() => ({ recents: null as Recent[] | null, lastNow: 0, resolved: new Map<string, Promise<FileResolved>>() }));      // module state belongs to the world (api.ts `scoped`)

export function forget(): void { S.reset(); }


async function loadRecents(storage: Storage | undefined): Promise<Recent[]> {
  const s = S();
  if (s.recents) return s.recents;
  const none: Recent[] = []; s.recents = none;
  if (!storage) return none;
  try {
    const v = JSON.parse(new TextDecoder().decode(await storage.fs.readFile(RECENT_FILE)));
    if (Array.isArray(v)) s.recents = v.filter((r) => r && typeof r.path === 'string' && typeof r.openedAt === 'number').slice(0, RECENT_LIMIT);
  } catch { /* none kept yet */ }
  return s.recents ?? none;
}
async function saveRecents(storage: Storage | undefined, gen: number = accountGeneration()): Promise<void> {
  if (!S().recents) return;
  await putFile(storage, RECENT_FILE, new TextEncoder().encode(JSON.stringify(S().recents)));      // a full disk loses only the recent list; a sign-out in flight writes nothing
}
async function touch(storage: Storage | undefined, e: Entry, at: number, gen: number = accountGeneration()): Promise<void> {
  const list = await loadRecents(storage);
  if (gen !== accountGeneration()) return;                  // signed out since the source started: neither memory nor the file takes it
  S().recents = [{ path: e.path, size: e.size, modified: e.modified, openedAt: at }, ...list.filter((r) => r.path !== e.path)].slice(0, RECENT_LIMIT);
  await saveRecents(storage, gen);
}
async function forgetRecent(storage: Storage | undefined, path: string, prefix = false): Promise<void> {
  const list = await loadRecents(storage);
  S().recents = list.filter((r) => r.path !== path && !(prefix && r.path.startsWith(path + '/')));
  await saveRecents(storage);
}
async function renameRecent(storage: Storage | undefined, from: string, to: string): Promise<void> {
  const list = await loadRecents(storage);
  S().recents = list.map((r) => (r.path === from ? { ...r, path: to } : r));
  await saveRecents(storage);
}

// ---------------------------------------------------------------- the listing

export type ListArgs = { path: string; mode: string; sort: string; desc: boolean; query: string };

function build(path: string, mode: string, sort: M.SortKey, desc: boolean, query: string, entries: Entry[], now: number, places: Record<string, string> = {}, status?: string, message = ''): FileListing {
  const rows = entries.map((e) => M.rowOf(e, now, places[e.path] ?? ''));
  const title = path === '/' ? 'Files' : M.nameOf(path);
  const empty = rows.length === 0;
  return {
    path, title: mode === 'recent' ? 'Recent' : title, crumbs: M.crumbsOf(path), rows, count: rows.length, status: status ?? (empty ? 'empty' : 'ready'),
    message: message === '' && empty ? (mode === 'recent' ? 'Files you open appear here.' : mode === 'search' ? 'Nothing matches.' : 'This folder is empty.') : message,
    mode, sort, desc, query,
  };
}

const makeBackend = (net: Net): Backend => choirBackend(net);
const noteNow = (n: number | null): number => { if (n !== null) S().lastNow = n; return S().lastNow; };

export async function fileList(a: ListArgs, storage?: Storage, net: Net = realNet): Promise<FileListing> {
  const sort = M.sortKey(a.sort);
  const path = M.normalize(a.path) ?? '/';
  const q = a.query.trim();
  const backend = makeBackend(net);
  try {
    if (q !== '') return await search(backend, path, q, sort, a.desc);
    if (a.mode === 'recent') {
      const list = await loadRecents(storage);
      const now = S().lastNow || list.reduce((m, r) => Math.max(m, r.openedAt, r.modified ?? 0), 0);
      const entries = list.map((r): Entry => ({ name: M.nameOf(r.path), path: r.path, isDirectory: false, size: r.size, modified: r.modified }));
      const places: Record<string, string> = {};
      for (const e of entries) places[e.path] = M.parent(e.path) === '/' ? 'Files' : M.nameOf(M.parent(e.path));
      return build('/', 'recent', sort, a.desc, '', entries, now, places);
    }
    const r = await backend.list(path);
    const now = noteNow(r.now);
    return build(path, 'browse', sort, a.desc, '', M.sortEntries(r.entries, sort, a.desc), now);
  } catch (e) {
    return build(path, a.mode, sort, a.desc, q, [], S().lastNow, {}, 'error', say(e));
  }
}

/** A bounded walk under `path`: names containing the query (case and accents ignored). */
async function search(backend: Backend, path: string, query: string, sort: M.SortKey, desc: boolean): Promise<FileListing> {
  const found: Entry[] = [], places: Record<string, string> = {};
  const queue: [string, number][] = [[path, 0]];
  const needle = query.normalize('NFD').replace(/[̀-ͯ]/g, '').toLowerCase();
  let visited = 0;
  while (queue.length > 0 && visited < SEARCH_FOLDERS && found.length < SEARCH_FOUND) {
    const [dir, depth] = queue.shift()!;
    visited++;
    let listed;
    try { listed = await backend.list(dir); } catch (e) { if (dir === path) throw new FilesError('notFound', 'That file is not there any more.'); continue; }
    noteNow(listed.now);
    for (const e of listed.entries) {
      if (e.name.normalize('NFD').replace(/[̀-ͯ]/g, '').toLowerCase().includes(needle)) { found.push(e); places[e.path] = dir === '/' ? 'Files' : M.nameOf(dir); }
      if (e.isDirectory && depth < SEARCH_DEPTH) queue.push([e.path, depth + 1]);
    }
  }
  return build(path, 'search', sort, desc, query, M.sortEntries(found, sort, desc), S().lastNow, places);
}

// ---------------------------------------------------------------- opening

const cacheKey = (e: Entry): string => M.hash(`${e.path}|${e.size}|${e.modified ?? ''}`);
/** Where an entry's downloaded copy goes: `app:/cache/files/<key>/<name>`, which the account's root (api.ts) puts under `accounts/<d>/`. The host writes a download, not `storage`, so `authedToFile` re-roots its `dest` the same way. */
export const cacheDest = (e: Entry): string => `${CACHE_DIR}/${cacheKey(e)}/${M.cacheName(e.name)}`;

async function entryAt(backend: Backend, path: string): Promise<Entry | null> {
  const r = await backend.list(M.parent(path));
  noteNow(r.now);
  return r.entries.find((e) => e.path === path) ?? null;
}

function infoOf(e: Entry, status: string, message = ''): FileInfo {
  const k = M.kindOf(e.name, e.isDirectory);
  return {
    ...M.emptyInfo(), address: M.addressOf(e.path, e.isDirectory), path: e.path, folder: M.parent(e.path), name: e.name, kind: k, kindLabel: M.kindLabel(k), size: e.size,
    sizeLabel: M.sizeLabel(e.size), modifiedLabel: M.dateLabel(e.modified, S().lastNow), mime: M.mimeOf(e.name), status, message,
  };
}

export type OpenArgs = { address: string };

/** Open the file a `file:` address names: find it in its folder, bring what its viewer needs, and for a file another page reads, ask the computer to open it. */
export async function fileOpen(a: OpenArgs, storage?: Storage, net: Net = realNet): Promise<FileInfo> {
  const gen = accountGeneration();
  const parsed = M.fromAddress(a.address);
  if (!a.address.startsWith('file:') || a.address.endsWith('/') || !parsed) return { ...M.emptyInfo(), address: a.address };
  const { path } = parsed;
  const backend = makeBackend(net);
  let entry: Entry | null;
  try {
    entry = await entryAt(backend, path);
  } catch (e) {
    const notFound = e instanceof FilesError && e.kind === 'notFound' || (e as { status?: number })?.status === 404;
    if (notFound) await forgetRecent(storage, path);
    return { ...M.emptyInfo(), address: a.address, path, name: M.nameOf(path), status: notFound ? 'notfound' : 'error', message: notFound ? 'That file is not there any more.' : say(e) };
  }
  if (!entry) {
    await forgetRecent(storage, path);
    return { ...M.emptyInfo(), address: a.address, path, name: M.nameOf(path), status: 'notfound', message: 'That file is not there any more.' };
  }
  if (entry.isDirectory) return infoOf(entry, 'folder');
  const kind = M.kindOf(entry.name, false);
  const out = infoOf(entry, 'ready');
  if (kind === 'other') { await touch(storage, entry, S().lastNow, gen); return out; }
  if (M.opensElsewhere(kind)) {
    const r = await resolveEntry(entry, storage, net);
    out.target = r.target; out.status = r.status === 'ready' ? 'ready' : 'error'; out.message = r.message;
    return out;
  }
  const cap = M.maxBytes(kind);
  if (entry.size > cap) { out.status = 'toolarge'; out.message = `It is ${M.sizeLabel(entry.size)}, too large to open on this phone.`; return out; }
  try {
    const dest = cacheDest(entry), dir = dest.slice(0, dest.lastIndexOf('/'));
    let bytes: Uint8Array | null = null;
    let have = false;
    // Whole means the size the listing named: the host stages a download beside the file and puts it in place only when complete, so a file of another
    // size is a leftover (or a changed file) and is fetched again.
    if (storage) { try { have = (await storage.fs.stat(dest)).size === entry.size; } catch { have = false; } }
    if (!have) {
      if (storage) {
        // The host writes the body to the file; nothing enters JavaScript (a video of 800 MB costs the same as a note).
        await storage.fs.mkdir(dir).catch(() => {});
        try {
          const written = await backend.download(path, dest, storage);
          if (written > cap) throw new FilesError('tooLarge', `It is ${M.sizeLabel(written)}, too large to open on this phone.`);
        } catch (e) { try { await storage.fs.rm(dest); } catch { /* nothing there */ } throw e; }
      } else {
        // No file system (the web build): the reply is held in memory, so only what fits there.
        if (entry.size > M.MEMORY_MAX) throw new FilesError('tooLarge', `It is ${M.sizeLabel(entry.size)}, too large to open here.`);
        bytes = await backend.fetchBytes(path);
      }
    }
    if (storage) { out.local = appPath(dest); out.src = out.local; }      // what the page and the host see: the account's root is in the path
    if (kind === 'csv') {
      bytes = bytes ?? new Uint8Array(await storage!.fs.readFile(dest));
      const cut = bytes.length > 256_000;
      const text = new TextDecoder().decode(cut ? M.withoutPartialUTF8(bytes.slice(0, 256_000)) : bytes);
      out.rows = M.parseCsv(text, M.extOf(entry.name) === 'tsv' ? '\t' : ',', 300).map((cells) => ({ cells: cells.slice(0, 12) }));
      out.truncated = cut;
    } else if (kind === 'image' && entry.size <= M.MEMORY_MAX) {
      bytes = bytes ?? new Uint8Array(await storage!.fs.readFile(dest));
      const [w, h] = imageSize(bytes); out.width = w; out.height = h;
    }
    await touch(storage, entry, S().lastNow, gen);
  } catch (e) {
    out.status = e instanceof FilesError && e.kind === 'tooLarge' ? 'toolarge' : 'error';
    out.message = say(e);
  }
  return out;
}

/** The pixel size of a PNG, GIF or JPEG from its header (0, 0 when it is not one of those): the pinch-and-pan viewer fits it to its frame. */
export function imageSize(b: Uint8Array): [number, number] {
  const u32 = (i: number) => ((b[i] << 24) | (b[i + 1] << 16) | (b[i + 2] << 8) | b[i + 3]) >>> 0;
  if (b.length > 24 && b[0] === 0x89 && b[1] === 0x50 && b[2] === 0x4e && b[3] === 0x47) return [u32(16), u32(20)];
  if (b.length > 10 && b[0] === 0x47 && b[1] === 0x49 && b[2] === 0x46) return [b[6] | (b[7] << 8), b[8] | (b[9] << 8)];
  if (b.length > 4 && b[0] === 0xff && b[1] === 0xd8) {
    let i = 2;
    while (i + 9 < b.length) {
      if (b[i] !== 0xff) { i++; continue; }
      const m = b[i + 1];
      if (m >= 0xc0 && m <= 0xcf && m !== 0xc4 && m !== 0xc8 && m !== 0xcc) return [(b[i + 7] << 8) | b[i + 8], (b[i + 5] << 8) | b[i + 6]];
      i += 2 + ((b[i + 2] << 8) | b[i + 3]);
    }
  }
  return [0, 0];
}

// ---------------------------------------------------------------- a file that is another page's document

/** Ask the listener's computer to open the file as a Texture document (decision D4: it reads a PDF's or an e-book's text itself); the answer's `target` is the
 * `texture:` page to open. One ask per file version, shared by every caller; a failure is not kept, so a second open asks again. */
function resolveEntry(entry: Entry, storage: Storage | undefined, net: Net): Promise<FileResolved> {
  const key = `${entry.path}|${entry.size}|${entry.modified ?? ''}`;
  const held = S().resolved.get(key);
  if (held) return held;
  const p = doResolve(entry, storage, net).then((r) => { if (r.status !== 'ready') S().resolved.delete(key); return r; });
  S().resolved.set(key, p);
  return p;
}

async function doResolve(entry: Entry, storage: Storage | undefined, net: Net): Promise<FileResolved> {
  const gen = accountGeneration();
  const address = M.addressOf(entry.path, false);
  const fail = (message: string): FileResolved => ({ address, target: '', title: entry.name, status: 'error', message: `${entry.name} could not be opened. ${message} Tap the file to try again.` });
  const kind = M.kindOf(entry.name, false);
  try {
    let text = '';
    if (kind === 'markdown' || kind === 'text' || kind === 'code') {
      if (entry.size > TEXTURE_CAP) return fail(`It is ${M.sizeLabel(entry.size)}, too large to open on this phone.`);
      const bytes = await choirBackend(net).fetchBytes(entry.path);
      try { text = new TextDecoder('utf-8', { fatal: true }).decode(bytes); } catch { return fail('This file is not text.'); }
    }
    const r = await net.json('/api/texture/files/open', { method: 'POST', body: { source_path: entry.path.replace(/^\//, ''), title: entry.name, initial_content: text } });
    const id = (r.body as { doc_id?: unknown } | null)?.doc_id;
    if (typeof id !== 'string' || id === '') return fail('Texture did not open the document.');
    await touch(storage, entry, S().lastNow, gen);
    return { address, target: 'texture:' + id, title: entry.name, status: 'ready', message: '' };
  } catch (e) {
    return fail(say(e));
  }
}

export type ResolveArgs = { address: string; title: string };
export async function fileResolve(a: ResolveArgs, storage?: Storage, net: Net = realNet): Promise<FileResolved> {
  const parsed = M.fromAddress(a.address);
  if (!parsed || parsed.isDirectory) return { ...M.emptyResolved(), address: a.address, status: 'error', message: 'That is not a file address.' };
  const backend = makeBackend(net);
  const name = M.nameOf(parsed.path);
  try {
    const entry = await entryAt(backend, parsed.path);
    if (!entry) return { address: a.address, target: '', title: name, status: 'error', message: `${name} could not be opened. That file is not there any more. Tap the file to try again.` };
    if (!M.opensElsewhere(M.kindOf(entry.name, false))) return { address: a.address, target: a.address, title: a.title || name, status: 'ready', message: '' };
    return await resolveEntry(entry, storage, net);
  } catch (e) {
    return { address: a.address, target: '', title: name, status: 'error', message: `${name} could not be opened. ${say(e)} Tap the file to try again.` };
  }
}

// ---------------------------------------------------------------- changes (each answers the refreshed listing of the folder it changed)

async function after(net: Net, storage: Storage | undefined, dir: string, message: string): Promise<FileListing> {
  const l = await fileList({ path: dir, mode: 'browse', sort: 'name', desc: false, query: '' }, storage, net);
  l.message = message;
  return l;
}

export async function fileMkdir(a: { dir: string; name: string }, storage?: Storage, net: Net = realNet): Promise<FileListing> {
  const d = M.normalize(a.dir) ?? '/';
  if (!M.validName(a.name)) return after(net, storage, d, BAD_NAME);
  try { await makeBackend(net).mkdir(M.join(d, a.name)); return after(net, storage, d, `Made the folder “${a.name}”.`); }
  catch (e) { return after(net, storage, d, say(e)); }
}

export async function fileDelete(a: { path: string }, storage?: Storage, net: Net = realNet): Promise<FileListing> {
  const p = M.normalize(a.path);
  if (p === null || p === '/') return after(net, storage, '/', 'That cannot be deleted.');
  const d = M.parent(p);
  try { await makeBackend(net).remove(p); await forgetRecent(storage, p, true); return after(net, storage, d, `Deleted “${M.nameOf(p)}”.`); }
  catch (e) { return after(net, storage, d, say(e)); }
}

export async function fileRename(a: { path: string; name: string }, storage?: Storage, net: Net = realNet): Promise<FileListing> {
  const p = M.normalize(a.path);
  if (p === null || p === '/') return after(net, storage, '/', 'That cannot be renamed.');
  const d = M.parent(p);
  if (!M.validName(a.name)) return after(net, storage, d, BAD_NAME);
  const dest = M.join(d, a.name);
  try { await makeBackend(net).move(p, dest); await renameRecent(storage, p, dest); return after(net, storage, d, `Renamed to “${a.name}”.`); }
  catch (e) { return after(net, storage, d, say(e)); }
}

/** Add a copy of a local file into `dir`. `from` is the system picker's copy under app:/tmp/picked (a local file may go to the listener's own computer).
 * An empty `from` uploads nothing and says so. */
export async function fileUpload(a: { from: string; name: string; dir: string }, storage?: Storage, net: Net = realNet): Promise<FileListing> {
  const d = M.normalize(a.dir) ?? '/';
  const name = a.name;
  if (a.from === '') return after(net, storage, d, 'Nothing was chosen to add.');
  let bytes: Uint8Array;
  try { bytes = new Uint8Array(await storage!.fs.readFile(a.from)); } catch { return after(net, storage, d, 'Nothing was chosen to add.'); }
  if (!M.validName(name)) return after(net, storage, d, BAD_NAME);
  try { await makeBackend(net).put(M.join(d, name), bytes); return after(net, storage, d, `Added “${name}”.`); }
  catch (e) { return after(net, storage, d, say(e)); }
}

// ---------------------------------------------------------------- for another page that lives in a folder of the computer (imported documents, imported.ts)

/** A folder's entries as the computer lists them, raw (size and modified time): another page that keeps its own cache of what a folder holds reads this, not a second client. */
export async function fileEntries(path: string, net: Net = realNet): Promise<{ entries: Entry[]; now: number | null }> {
  return makeBackend(net).list(M.normalize(path) ?? '/');
}
/** A file of the computer written by the host to the app file `dest` (under app:/cache), the bytes never in JavaScript; the size written. */
export async function fileDownload(path: string, dest: string, storage: Storage, net: Net = realNet): Promise<number> {
  return makeBackend(net).download(M.normalize(path) ?? '/', dest, storage);
}
/** The `texture:` page the computer made of a PDF or an e-book (it read the text itself, decision D4), or why not. */
export async function fileTexture(entry: Entry, storage: Storage | undefined, net: Net = realNet): Promise<FileResolved> {
  return resolveEntry(entry, storage, net);
}
