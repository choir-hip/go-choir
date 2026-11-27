// The imported documents' data: PDFs and e-books the person imports to read and hear. An imported document is a FILE ON THE PERSON'S COMPUTER, in the folder
// FOLDER (imported-model.ts); it is put there through the Files client (files.ts: fileUpload), listed by it (fileEntries), and its text is what the computer read
// (files.ts fileTexture: POST /api/texture/files/open, then GET /api/texture/revisions/{id}: the server's pdftotext and pandoc, internal/content/extract.go).
// So the list is the same on every device signed in to the same computer, and an imported document is never deleted from here: it is the person's file; the Files page deletes.
// The phone keeps only what it can fetch again (under app:/cache/files/<key>/: the text as blocks, and the original for the page view and the share sheet) and the
// reading place of each document (app:/data/imported-places.json: small, per device).
//
// `importedReadAloud` and `importedQueue` answer the words (one line per block from a block to the end) as data; the page hands them to the shared radio (radioHandWords).
//
// Ways this can fail (each is a test in imported.test.ts):
//  1. Nothing chosen, a name that is not a PDF or an e-book, a file over what the phone can send: refused with a sentence, nothing sent.
//  2. The same file imported twice (the same name and size): one document, nothing uploaded again. Another file with a taken name gets a numbered name; nothing is overwritten.
//  3. The folder does not exist yet: it is made, once.
//  4. An upload that failed says why in plain words; one that did not land is not "added".
//  5. A file with no text (a scan) is kept on the computer, and the person is told it has no text to read.
//  6. A document that is gone from the computer says so; a document opened while signed out says to sign in; neither shows another document.
//  7. A position beyond the end or below zero is clamped; a place is kept across relaunches and wiped with the account.
//  8. The words handed to the player start at the chosen block when it is in the section on screen.
//  9. The text is fetched once per file version; a changed file (size or time) is fetched again.
// Logs carry counts and kinds, never a file name.
import type { Storage } from './app.contract.d.ts';
import { accountGeneration, authedJSON, writeCurrent } from './api.ts';
import * as F from './files.ts';
import * as FM from './files-model.ts';
import { textureSource } from './content.ts';
import * as M from './imported-model.ts';
import { lapYield } from './lap.ts';
/** Ends the turn between slices of a long read (Exact's scheduler.yield; a no-op under Bun). */
const lapTick = (): Promise<void> => lapYield('imported');
import type { Result } from './app.contract.d.ts';

export type ImportedList = Result<'importedList'>;
export type ImportedDocInfo = Result<'importedDoc'>;
export type ImportedSection = Result<'importedSection'>;
export type ImportedFind = Result<'importedFind'>;
export type ImportedAck = Result<'importedPlace'>;

export const PLACES_FILE = 'app:/data/imported-places.json';
/** The most the phone uploads: the file is read into JavaScript to be sent (files.ts fileUpload), and the engine's heap is 64 MiB. */
export const UPLOAD_MAX = FM.MEMORY_MAX;
/** The most the phone keeps a copy of, for the page view and the share sheet. A bigger document is still read (its text is the computer's). */
export const COPY_MAX = 100_000_000;
const OPEN_DOCUMENTS = 3;
/**
 * THE CACHE RULE (app:/cache/files, where the Files page keeps its own copies too). For imported documents: the original is kept for the COPY_KEEP documents opened last (it can be tens of megabytes),
 * the small text of every opened document stays (it is what the search reads), and a changed file's old version is removed whole. The record is `imported-lru.json` in the cache folder itself,
 * so the Files wipe removes it with the rest.
 */
export const COPY_KEEP = 6;
export const LRU_FILE = `${F.CACHE_DIR}/imported-lru.json`;
const LRU_MAX = 200;

const NO_PLAY = { op: '', id: '', title: '', at: 0, text: '' };
const ack = (ok: boolean, message: string, play = NO_PLAY): ImportedAck => ({ ok, message, play });
const EMPTY_MESSAGE = 'Import a PDF or an EPUB to read it here, hear it read aloud and search it. It is kept on your computer.';
const NO_TEXT = { pdf: 'This PDF has no text to read.', epub: 'This EPUB has no readable chapters.' };
const GONE = 'This document is not on your computer any more.';

// ---------------------------------------------------------------- state between answers (cleared by forget())

type Loaded = { key: string; text: M.DocText; index: M.Index | null };
let places: Record<string, M.Place> | null = null;
let folderKnown = false;                              // the folder was listed, so it exists
const known = new Map<string, FM.Entry>();           // id -> the entry the last list or open saw
let open: Loaded[] = [];                              // the last few documents held in memory, newest last

let cacheGen = -1;
export function forget(): void { folderKnown = false; places = null; known.clear(); open = []; cacheGen = accountGeneration(); }
/** The caches belong to the account generation they were made in: the first touch after it moved empties them. A late answer is checked against its own captured generation before it inserts. */
const sync = (): void => { if (cacheGen !== accountGeneration()) forget(); };

/** Sign-out (account.ts): the places and what this session holds. The cache is under app:/cache/files, which the Files wipe removes. Never throws. */
export async function wipeImported(storage: Storage | undefined): Promise<void> {
  forget();
  if (!storage) return;
  try { await storage.fs.rm(PLACES_FILE); } catch { /* absent */ }
}

async function loadPlaces(storage: Storage | undefined): Promise<Record<string, M.Place>> {
  sync();
  if (places) return places;
  const gen = accountGeneration();
  const mine: Record<string, M.Place> = {};
  places = mine;
  if (!storage) return mine;
  try {
    const v = JSON.parse(new TextDecoder().decode(await storage.fs.readFile(PLACES_FILE)));
    if (v && typeof v === 'object' && !Array.isArray(v)) {
      for (const [k, p] of Object.entries(v as Record<string, { block?: unknown; length?: unknown }>)) {
        if (p && typeof p.block === 'number' && typeof p.length === 'number') mine[k] = { block: p.block, length: p.length };
      }
    }
  } catch { /* none kept yet */ }
  if (gen !== accountGeneration()) { if (places === mine) places = null; return {}; }            // the account moved while the file was read: nothing of it is kept or shown
  return mine;
}
async function setPlace(storage: Storage | undefined, id: string, block: number, length: number, gen: number): Promise<void> {
  const all = await loadPlaces(storage);
  if (gen !== accountGeneration()) return;
  const b = M.clampBlock(block, length);
  if (all[id] && all[id].block === b && all[id].length === length) return;
  all[id] = { block: b, length };
  await writeCurrent(gen, storage, PLACES_FILE, new TextEncoder().encode(JSON.stringify(all)));
}

// ---------------------------------------------------------------- the computer's folder

const say = F.say;
const isMissing = (e: unknown): boolean => (e as { status?: number })?.status === 404;
const cacheKey = (e: FM.Entry): string => FM.hash(`${e.path}|${e.size}|${e.modified ?? ''}`);

/** The documents in the folder (PDFs and e-books), newest first; a folder that does not exist yet holds none. */
async function entries(net: F.Net): Promise<FM.Entry[]> {
  sync();
  const gen = accountGeneration();
  let listed;
  try { listed = await F.fileEntries(M.FOLDER, net); if (gen === accountGeneration()) folderKnown = true; } catch (e) { if (isMissing(e)) { if (gen === accountGeneration()) folderKnown = false; return []; } throw e; }
  if (gen !== accountGeneration()) return [];                                                       // a late list of the previous account's folder is not kept
  const docs = listed.entries.filter((e) => !e.isDirectory && M.kindOfName(e.name) !== null)
    .sort((a, b) => (b.modified ?? 0) - (a.modified ?? 0) || (a.name < b.name ? -1 : a.name > b.name ? 1 : 0));
  for (const e of docs) known.set(M.idOf(e.name), e);
  return docs;
}
/** The documents of the folder (for the search's document source, imported-search.ts). */
export const entriesOf = entries;
async function entryOf(id: string, net: F.Net, refresh = false): Promise<FM.Entry | null> {
  const name = M.nameOfId(id);
  if (name === null || M.kindOfName(name) === null) return null;
  if (!refresh) { const k = known.get(id); if (k) return k; }
  known.delete(id);
  await entries(net);
  return known.get(id) ?? null;
}

// ---------------------------------------------------------------- the text

type Text = { status: 'ready' | 'empty' | 'error'; message: string; doc?: Loaded };
const remember = (l: Loaded, gen: number): Loaded => { if (gen !== accountGeneration()) return l; sync(); open = [...open.filter((o) => o.key !== l.key), l].slice(-OPEN_DOCUMENTS); return l; };

/** The text of a document: held in memory, else read from the phone's copy of what the computer read, else asked of the computer (and kept). */
async function textOf(e: FM.Entry, storage: Storage | undefined, net: F.Net): Promise<Text> {
  sync();
  const gen = accountGeneration();
  const key = cacheKey(e), kind = M.kindOfName(e.name)!;
  const held = open.find((o) => o.key === key);
  if (held) return { status: 'ready', message: '', doc: remember(held, gen) };
  const file = `${F.CACHE_DIR}/${key}/text.json`;
  if (storage) {
    try {
      const v = JSON.parse(new TextDecoder().decode(await storage.fs.readFile(file)));
      await lapTick();
      if (Array.isArray(v?.sections) && Array.isArray(v?.blocks) && v.blocks.length > 0) return { status: 'ready', message: '', doc: remember({ key, text: v as M.DocText, index: null }, gen) };
    } catch { /* not kept yet */ }
  }
  try {
    const r = await F.fileTexture(e, storage, net);
    if (r.status !== 'ready' || !r.target.startsWith('texture:')) return { status: 'error', message: r.message === '' ? 'Your computer could not read this document.' : r.message };
    const md = await textureSource(r.target.slice('texture:'.length), async (p) => (await net.json(p)).body);
    const text = kind === 'pdf' ? await M.fromPdfTextAsync(md ?? '', lapTick) : await M.fromMarkdownAsync(md ?? '', lapTick);   // a long book is read over several turns (each turn is about 100 ms of CPU)
    if (text.blocks.length === 0) return { status: 'empty', message: NO_TEXT[kind] };
    await lapTick();
    if (storage && gen === accountGeneration()) {
      try { await storage.fs.mkdir(`${F.CACHE_DIR}/${key}`).catch(() => {}); await writeCurrent(gen, storage, file, new TextEncoder().encode(JSON.stringify(text))); } catch { /* a full disk loses only the copy */ }
    }
    console.log(`imported: text kind=${kind} blocks=${text.blocks.length} sections=${text.sections.length}`);
    return { status: 'ready', message: '', doc: remember({ key, text, index: null }, gen) };
  } catch (err) {
    return { status: 'error', message: say(err) };
  }
}
const indexOf = async (d: Loaded): Promise<M.Index> => (d.index ??= await M.buildIndexAsync(d.text.blocks, lapTick));

/** The original, for the page view of a PDF and the share sheet: the host writes it under the cache, no bytes in JavaScript. '' when it cannot be had. */
async function copyOf(e: FM.Entry, storage: Storage | undefined, net: F.Net): Promise<string> {
  if (!storage || e.size > COPY_MAX) return '';
  const dest = `${F.CACHE_DIR}/${cacheKey(e)}/${FM.cacheName(e.name)}`;
  try { if ((await storage.fs.stat(dest)).size === e.size) return dest; } catch { /* not there */ }
  try {
    await storage.fs.mkdir(`${F.CACHE_DIR}/${cacheKey(e)}`).catch(() => {});
    const n = await F.fileDownload(e.path, dest, storage, net);
    if (n === e.size) return dest;
    try { await storage.fs.rm(dest); } catch { /* nothing there */ }
  } catch { try { await storage.fs.rm(dest); } catch { /* nothing there */ } }
  return '';
}

type Kept = { path: string; key: string; name: string; copy: boolean };
/** Note that this version was just used: drop the old versions of the same file, keep the original of the last COPY_KEEP, forget the oldest past LRU_MAX. Never throws. */
async function keepCache(e: FM.Entry, storage: Storage | undefined, gen: number): Promise<void> {
  if (!storage || gen !== accountGeneration()) return;
  const key = cacheKey(e);
  let list: Kept[] = [];
  try { const v = JSON.parse(new TextDecoder().decode(await storage.fs.readFile(LRU_FILE))); if (Array.isArray(v)) list = v.filter((k): k is Kept => k && typeof k.path === 'string' && typeof k.key === 'string' && typeof k.name === 'string'); } catch { /* none yet */ }
  const rm = async (p: string): Promise<void> => { try { await storage.fs.rm(p); } catch { /* absent */ } };
  for (const k of list.filter((k) => k.path === e.path && k.key !== key)) await rm(`${F.CACHE_DIR}/${k.key}`);
  list = list.filter((k) => k.key !== key && k.path !== e.path);
  list.push({ path: e.path, key, name: e.name, copy: true });
  while (list.length > LRU_MAX) { const old = list.shift()!; await rm(`${F.CACHE_DIR}/${old.key}`); }
  for (const k of list.slice(0, Math.max(0, list.length - COPY_KEEP))) if (k.copy) { await rm(`${F.CACHE_DIR}/${k.key}/${FM.cacheName(k.name)}`); k.copy = false; }
  if (gen === accountGeneration()) { try { await writeCurrent(gen, storage, LRU_FILE, new TextEncoder().encode(JSON.stringify(list))); } catch { /* the rule runs again next open */ } }
}

// ---------------------------------------------------------------- the list

const rowOf = (e: FM.Entry, p: Record<string, M.Place>): ImportedList['rows'][number] => {
  const id = M.idOf(e.name), kind = M.kindOfName(e.name)!;
  return { id, address: M.addressOf(id), title: M.titleOf(e.name), kindLabel: M.kindLabel(kind), detail: [M.kindLabel(kind), FM.sizeLabel(e.size), M.progressLabel(p[id])].join(' · '), progress: M.progressOf(p[id]) };
};
const listing = (rows: ImportedList['rows'], message = '', status?: string): ImportedList => ({
  status: status ?? (rows.length === 0 ? 'empty' : 'ready'), message: message === '' && rows.length === 0 && status !== 'error' ? EMPTY_MESSAGE : message, count: rows.length, rows,
});

export async function importedList(_at: string, storage?: Storage, net: F.Net = F.realNet, message = ''): Promise<ImportedList> {
  try {
    const docs = await entries(net);
    const p = await loadPlaces(storage);
    return listing(docs.map((e) => rowOf(e, p)), message);
  } catch (e) { return listing([], say(e), 'error'); }
}

// ---------------------------------------------------------------- a document

const blankInfo = (address: string): ImportedDocInfo => ({
  address, id: '', title: '', kind: '', kindLabel: '', status: 'loading', message: '', src: '', shareSrc: '', sections: [], blockCount: 0, position: 0, resumeSection: 0, resumeBlock: -1,
});

export async function importedDoc(address: string, storage?: Storage, net: F.Net = F.realNet): Promise<ImportedDocInfo> {
  const parsed = M.parseAddress(address);
  if (!parsed) return blankInfo(address);
  const gen = accountGeneration();
  const base = { ...blankInfo(address), id: parsed.id };
  const e = await entryOf(parsed.id, net, true).catch((err): Error => (err instanceof Error ? err : new Error('read')));
  if (e instanceof Error) return { ...base, status: 'error', message: say(e) };
  if (!e) return { ...base, status: 'notfound', message: GONE };
  const kind = M.kindOfName(e.name)!;
  const [t, copy] = await Promise.all([textOf(e, storage, net), copyOf(e, storage, net)]);
  await keepCache(e, storage, gen);
  const head = { ...base, title: M.titleOf(e.name), kind, kindLabel: M.kindLabel(kind) };
  if (t.status !== 'ready' || !t.doc) return { ...head, status: t.status === 'empty' ? 'empty' : 'error', message: t.message, shareSrc: copy };
  const text = t.doc.text;
  const p = await loadPlaces(storage);
  if (parsed.block !== null) await setPlace(storage, parsed.id, parsed.block, text.blocks.length, gen);
  else if (p[parsed.id] && p[parsed.id].length !== text.blocks.length) await setPlace(storage, parsed.id, p[parsed.id].block, text.blocks.length, gen);
  const position = M.clampBlock((await loadPlaces(storage))[parsed.id]?.block ?? 0, text.blocks.length);
  const src = kind === 'pdf' ? copy : '';
  return {
    ...head, status: 'ready', message: kind === 'pdf' && src === '' ? 'The original PDF could not be shown, so its text is here.' : '', src, shareSrc: copy,
    sections: text.sections.map((s, i) => ({ index: i, title: s.title })), blockCount: text.blocks.length, position, resumeSection: M.sectionOf(text, position), resumeBlock: position,
  };
}

async function held(address: string, storage: Storage | undefined, net: F.Net): Promise<{ id: string; e: FM.Entry; d: Loaded } | null> {
  const parsed = M.parseAddress(address);
  if (!parsed) return null;
  const e = await entryOf(parsed.id, net).catch(() => null);
  if (!e) return null;
  const t = await textOf(e, storage, net);
  return t.doc ? { id: parsed.id, e, d: t.doc } : null;
}

export async function importedSection(address: string, index: number, storage?: Storage, net: F.Net = F.realNet): Promise<ImportedSection> {
  const none: ImportedSection = { index: 0, count: 0, title: '', start: 0, page: 1, status: 'loading', blocks: [] };
  const h = await held(address, storage, net);
  if (!h || h.d.text.sections.length === 0) return none;
  const t = h.d.text;
  const i = Math.min(Math.max(Math.trunc(index) || 0, 0), t.sections.length - 1), s = t.sections[i];
  const blocks = t.blocks.slice(s.start, s.start + s.count).map((b, k) => ({ n: s.start + k, kind: b.kind, level: b.level, text: b.text, marker: b.marker }));
  // A PDF's section is titled "Page N" with N the real page (pages without text are not sections).
  const page = parseInt(s.title.replace('Page ', ''), 10);
  return { index: i, count: t.sections.length, title: s.title, start: s.start, page: Number.isFinite(page) ? page : i + 1, status: 'ready', blocks };
}

export async function importedFind(address: string, query: string, storage?: Storage, net: F.Net = F.realNet): Promise<ImportedFind> {
  const q = query.trim();
  const none: ImportedFind = { query: '', hits: [], message: '' };
  if (q === '') return none;
  const h = await held(address, storage, net);
  if (!h) return none;
  const t = h.d.text;
  const hits = M.search(await indexOf(h.d), q, 50).map((n) => ({ block: n, section: t.blocks[n].section, label: t.sections[t.blocks[n].section]?.title ?? '', snippet: M.snippet(t.blocks[n].text, q) }));
  return { query: q, hits, message: hits.length === 0 ? `Nothing in this document matches “${q}”.` : '' };
}

/** The place left off. block < 0: the start of `section` (a chapter or page just opened). */
export async function importedPlace(a: { address: string; section: number; block: number }, storage?: Storage, net: F.Net = F.realNet): Promise<ImportedAck> {
  const gen = accountGeneration();
  const h = await held(a.address, storage, net);
  if (!h) return ack(false, GONE);
  const t = h.d.text;
  const b = a.block >= 0 ? a.block : (t.sections.length === 0 ? 0 : t.sections[Math.min(Math.max(Math.trunc(a.section), 0), t.sections.length - 1)].start);
  await setPlace(storage, h.id, b, t.blocks.length, gen);
  return ack(true, '');
}

async function hand(op: 'play' | 'queue', a: { address: string; section: number; block: number }, storage: Storage | undefined, net: F.Net): Promise<ImportedAck> {
  const h = await held(a.address, storage, net);
  const at = h ? M.startBlock(h.d.text, a.section, a.block) : null;
  if (!h || at === null) return ack(false, 'This document has nothing to read.');
  const words = M.spokenFrom(h.d.text, at);
  return ack(true, op === 'queue' ? 'Added to the queue.' : '', { op, id: h.id, title: M.titleOf(h.e.name), at, text: words.join('\n') });
}
export const importedReadAloud = (a: { address: string; section: number; block: number }, storage?: Storage, net: F.Net = F.realNet): Promise<ImportedAck> => hand('play', a, storage, net);
export const importedQueue = (a: { address: string; section: number; block: number }, storage?: Storage, net: F.Net = F.realNet): Promise<ImportedAck> => hand('queue', a, storage, net);

// ---------------------------------------------------------------- importing

/** Put a picked file on the person's computer, in the Imported documents folder, through the Files client. `from` is the system picker's copy under app:/tmp/picked. Answers the list. */
export async function importedAdd(a: { from: string; name: string }, storage?: Storage, net: F.Net = F.realNet): Promise<ImportedList> {
  const again = (message: string): Promise<ImportedList> => importedList('', storage, net, message);
  if (a.from === '' || !storage) return again('Nothing was chosen to add.');
  const kind = M.kindOfName(a.name);
  if (!kind) return again('Only PDF and EPUB files can be imported.');
  let size: number;
  try { size = (await storage.fs.stat(a.from)).size; } catch { return again('Nothing was chosen to add.'); }
  if (size > UPLOAD_MAX) return again(`This file is ${FM.sizeLabel(size)}, too large to import from this phone (the limit is ${FM.sizeLabel(UPLOAD_MAX)}).`);
  let existing: FM.Entry[];
  try { existing = await entries(net); } catch (e) { return again(say(e)); }
  const same = existing.find((e) => e.name === a.name && e.size === size);
  if (same) return again(`“${M.titleOf(same.name)}” is already imported.`);
  const name = M.uniqueName(a.name, new Set(existing.map((e) => e.name)));
  if (!FM.validName(name)) return again('That name cannot be used. Avoid slashes and empty names.');
  if (!folderKnown) { try { await F.fileMkdir({ dir: '/', name: M.FOLDER.slice(1) }, storage, net); } catch { /* the upload says why not */ } }
  const up = await F.fileUpload({ from: a.from, name, dir: M.FOLDER }, storage, net);
  let landed: FM.Entry | undefined;
  try { landed = (await entries(net)).find((e) => e.name === name && e.size === size); } catch { /* the list below says */ }
  if (!landed) return again(up.message.startsWith('Added') ? 'The file could not be imported.' : up.message);
  try { await storage.fs.rm(a.from); } catch { /* the picker's copy goes with sign-out at the latest */ }          // the file is on the computer now: the phone keeps no second copy of it
  const t = await textOf(landed, storage, net);
  console.log(`imported: added kind=${kind} text=${t.status}`);
  const title = M.titleOf(name);
  return again(t.status === 'ready' ? `Imported “${title}”.` : `“${title}” is on your computer. ${t.message}`);
}
