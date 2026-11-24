// The deck's open pages, in shared code (a port of the Swift OpenPages, Pages.swift): which pages are open, which is current, open / close / focus / pin / reopen, and the file that keeps
// them between launches. It is the one open-page set on every host (the Swift OpenPages is gone): the deck, the launcher (cards.ts) and the voice, which is handed the pages as facts (`mirror`). The set is addresses and
// titles only (never resumed work); the Autoradio page (`radio:`) is always open and cannot be closed; the pile keeps the 30 most recent unpinned pages; an address is open once.
// Sign-out: only the radio stays and the file goes (account.ts wipes it). Logs carry counts only.
import type { Storage } from './app.contract.d.ts';
import { accountGeneration, nativeLater, writeCurrent } from './api.ts';
import { note as noteRecent, noteNow as noteRecentNow, isLoaded as recentLoaded, events as recentEvents } from './recent-pages.ts';
import { lapOpen } from './lap.ts';

export const PAGES_FILE = 'app:/data/open-pages-shared.json';
export const RADIO = 'radio:';
export const EDITION = 'onboarding:edit';
export const LIMIT = 30;

export type Page = { address: string; kind: string; title: string; opener: string; pinned: boolean };
export type PageList = { rows: Page[]; top: string; topKind: string; topTitle: string; count: number };

export function canonical(address: string): string {
  const doc = /^doc:(.+)@\d+$/.exec(address);                         // a search hit's block is remembered by the Swift reader; here the document page opens
  if (doc) return `doc:${doc[1]}`;
  return address.trim().toLowerCase() === 'texture:my-edition' ? EDITION : address;
}
export function kindOf(address: string): string {
  const i = address.indexOf(':');
  switch (i < 0 ? address : address.slice(0, i)) {
    case 'radio': return 'radio'; case 'notebook': return 'notebook'; case 'paper': return 'paper'; case 'pub': case 'texture': return 'reader';
    case 'textures': return 'textures'; case 'file': return 'file'; case 'mail': return 'mail'; case 'mailbox': return 'mailbox'; case 'activity': return 'activity';
    case 'settings': return 'settings'; case 'lab': return 'lab'; case 'imported': return 'imported'; case 'doc': return 'doc'; case 'conversation': return 'conversation';
    case 'podcasts': case 'podcast': return 'podcast'; case 'onboarding': return 'onboarding'; default: return 'unknown';
  }
}
export function defaultTitle(address: string): string {
  const rest = address.includes(':') ? address.slice(address.indexOf(':') + 1) : '';
  switch (address) {
    case 'radio:': return 'Autoradio'; case 'notebook:': return 'My notebook'; case 'paper:front': return 'Paper'; case 'textures:': return 'Textures'; case 'activity:': return 'Activity';
    case 'settings:': case 'settings:root': return 'Settings'; case 'lab:components': return 'Components'; case 'lab:trace': return 'Trace'; case 'conversation:': return 'Conversation';
    case 'onboarding:setup': return 'Make it yours'; case 'onboarding:edit': return 'My edition'; case 'mailbox:inbox': return 'Mail'; case 'podcasts:': return 'Podcasts';
    case 'file:/': case 'file:': return 'Files'; case 'imported:': return 'Imported documents';
  }
  switch (kindOf(address)) {
    case 'paper': return 'Paper · ' + rest.replace('section/', '');
    case 'settings': return 'Settings · ' + rest;
    case 'file': return rest.slice(rest.lastIndexOf('/') + 1);
    case 'mailbox': return 'Mail · ' + rest;
    case 'mail': return 'Message';
    case 'doc': return 'Document';
    case 'podcast': return rest.startsWith('episode/') ? 'Episode' : 'Show';
    case 'reader': return rest === '' ? 'Story' : rest;
    default: return address;
  }
}

let pages: Page[] = [];
let recency: string[] = [];
let lastClosed: Page | null = null;
let loaded = false;
let loading: Promise<void> | null = null;
let epoch = 0;                       // bumped by a wipe: a read that began before it belongs to the old account and is dropped

const radioPage = (): Page => ({ address: RADIO, kind: 'radio', title: 'Autoradio', opener: '', pinned: false });
function focusLocked(address: string): void { recency = [address, ...recency.filter((a) => a !== address)]; }

function openLocked(p: Page): Page {
  const i = pages.findIndex((x) => x.address === p.address);
  if (i >= 0) {
    if (p.title !== '' && pages[i].title === defaultTitle(p.address)) pages[i].title = p.title;
    focusLocked(p.address); return pages[i];
  }
  pages.push(p); focusLocked(p.address);
  for (;;) {                                                               // the pile keeps the 30 most recent unpinned pages; the radio is never trimmed
    if (pages.filter((x) => !x.pinned && x.address !== RADIO).length <= LIMIT) break;
    const oldest = [...recency].reverse().find((a) => a !== RADIO && pages.find((x) => x.address === a)?.pinned === false);
    if (oldest === undefined) break;
    pages = pages.filter((x) => x.address !== oldest); recency = recency.filter((a) => a !== oldest);
  }
  return p;
}

/** What a stored file holds, entry by entry: a malformed entry is dropped (or its field takes the default), never half-applied. */
function validated(raw: unknown): { pages: Page[]; recency: string[] } {
  const o = typeof raw === 'object' && raw !== null ? (raw as { pages?: unknown; recency?: unknown }) : {};
  const out: Page[] = []; const seen = new Set<string>();
  for (const q of Array.isArray(o.pages) ? o.pages : []) {
    if (typeof q !== 'object' || q === null || typeof (q as { address?: unknown }).address !== 'string') continue;
    const e = q as { address: string; title?: unknown; opener?: unknown; pinned?: unknown };
    const address = canonical(e.address);
    if (address === '' || seen.has(address)) continue; seen.add(address);
    out.push({ address, kind: kindOf(address), title: typeof e.title === 'string' && e.title !== '' ? e.title : defaultTitle(address), opener: typeof e.opener === 'string' ? e.opener : '', pinned: e.pinned === true });
  }
  const inRec = new Set<string>(); const rec: string[] = [];
  for (const a of Array.isArray(o.recency) ? o.recency : []) {
    if (typeof a !== 'string') continue;
    const c = canonical(a);
    if (out.some((x) => x.address === c) && !inRec.has(c)) { inRec.add(c); rec.push(c); }
  }
  for (const q of out) if (!inRec.has(q.address)) rec.push(q.address);
  return { pages: out, recency: rec };
}

/** One read at a time (single flight): a second caller during the read waits for it. A wipe during the read drops its result and the caller reads again. */
async function load(storage: Storage | undefined): Promise<void> {
  for (let tries = 0; !loaded && tries < 3; tries++) {
    if (!loading) {
      const mine = epoch;
      const p: Promise<void> = (async () => {
        let stored: unknown = null;
        if (storage) { try { stored = JSON.parse(new TextDecoder().decode(await storage.fs.readFile(PAGES_FILE))); } catch { /* none kept yet, or damaged: the radio alone */ } }
        if (mine !== epoch) return;                                        // wiped while reading: this is the old account's file
        const v = validated(stored);
        pages = v.pages; recency = v.recency;
        openLocked(radioPage());                                           // every launch lands on the radio
        loaded = true;
      })().finally(() => { if (loading === p) loading = null; });
      loading = p;
    }
    await loading;
  }
}

let saving: Promise<unknown> = Promise.resolve();
/** The write is issued NOW, from the answer that called, and carries the set as it is now. Exact runs a module's storage in one queue in the order issued, so the last state wins the file; and a storage or native
 * call made after the answer ended is refused (docs/reference.md "refused in background work"), which a write chained on an earlier one (`.then`) would be when the earlier one is still out. A failed write is logged. */
function save(storage: Storage | undefined, gen: number): Promise<void> {
  const run = writeCurrent(gen, storage, PAGES_FILE, new TextEncoder().encode(JSON.stringify({ pages, recency })), false).then((ok) => {
    if (!ok && storage && gen === accountGeneration()) console.log('pages: open set not saved');
  });
  saving = Promise.all([saving, run]);
  return run;
}
/** For tests: every write issued so far has finished. */
export async function pagesSettled(): Promise<void> { await saving; }

function view(): PageList {
  const rows = recency.flatMap((a) => pages.filter((x) => x.address === a));
  const top = rows[0];
  return { rows, top: top?.address ?? '', topKind: top?.kind ?? '', topTitle: top?.title ?? '', count: pages.length };
}

/** The voice's copy: native keeps no set, only the pages as FACTS (OpenPageFacts, LocalAck.swift): which page is in front (the acknowledgement names it, the local model warms on it, a page left
 * posts what was typed on it) and the list the retrieval ask carries. Pushed through the one narrow app-module op `pagesMirror` after every change, with the account generation the change began under:
 * native refuses a push older than one it has seen. Best effort: no module (Android, the web) or a failed call loses nothing, the set is this file's. */
async function mirror(gen: number): Promise<void> {
  const raw = nativeLater();
  if (!raw) return;
  const later = raw as unknown as (r: Record<string, unknown>) => Promise<unknown>;
  const v = view();
  try { await later({ op: 'pagesMirror', gen, pages: v.rows.map((r) => ({ address: r.address, title: r.title, kind: r.kind })) }); } catch { /* the voice keeps the last facts */ }
}
let mirrored = false;

/** Read the set. The first read of a launch also hands the voice its facts. */
export async function pagesView(storage: Storage | undefined): Promise<PageList> {
  const gen = accountGeneration();
  await load(storage);
  if (!mirrored && gen === accountGeneration()) { mirrored = true; await mirror(gen); }
  return view();
}

/** A page a native module asked to open (`pageRequest`: an episode opened from the playing bar): opened once per request id, then the new view. An id of 0 asks nothing. */
let lastRequest = 0;
export async function pageRequested(storage: Storage | undefined, id: number, address: string, title: string): Promise<PageList> {
  if (id > 0 && id !== lastRequest && address !== '') { lastRequest = id; return pagesOpen(storage, address, title); }
  return pagesView(storage);
}

export async function pagesOpen(storage: Storage | undefined, rawAddress: string, title: string): Promise<PageList> {
  const gen = accountGeneration();
  const address = canonical(rawAddress);
  lapOpen('pagesOpen entry', address);
  await load(storage);
  lapOpen('pagesOpen loaded', address);
  if (gen !== accountGeneration()) return view();                          // a sign-out began while the set loaded: nothing of this open is kept
  // The recent ring is read at the first page of a launch (the one wait kept: its first note needs it); every later open finds it held.
  if (!recentLoaded()) await recentEvents(storage);
  if (gen !== accountGeneration()) return view();
  openLocked({ address, kind: kindOf(address), title: title !== '' ? title : defaultTitle(address), opener: recency[0] ?? '', pinned: false });
  // The page mounts now. Its three records are STARTED here, in order, and not waited for: the open-set file, the launcher's recent note and the voice's copy. Exact refuses a storage or native call made after the
  // answer has ended, so they must be issued inside it; it runs the storage in the order issued, and the voice's call goes after both. What is later than the view: the files on disk (a kill in those few milliseconds
  // loses one open, and a launch lands on the radio anyway) and the voice's facts (they arrive when the call lands; the acknowledgement names the page in front from them, and it is asked after the person has the page).
  // A failure is logged by each step (a write: `pages: open set not saved`), and the open stands in memory.
  const v = view();
  lapOpen('pagesOpen view ready', address);
  void save(storage, gen).then(() => lapOpen('pagesOpen saved', address));
  void noteRecentNow(storage, 'open', address, gen).then(() => lapOpen('pagesOpen recent noted', address));
  void mirror(gen).then(() => lapOpen('pagesOpen mirrored', address));
  return v;
}
export async function pagesFocus(storage: Storage | undefined, rawAddress: string): Promise<PageList> {
  const gen = accountGeneration(); await load(storage);
  const address = canonical(rawAddress);
  if (pages.some((x) => x.address === address)) focusLocked(address);
  await save(storage, gen); await mirror(gen); return view();
}
/** Close a page. When it was in front, its opener (if still open) comes forward, else the next most recent. */
export async function pagesClose(storage: Storage | undefined, rawAddress: string): Promise<PageList> {
  const gen = accountGeneration(); await load(storage);
  const address = canonical(rawAddress);
  const p = pages.find((x) => x.address === address);
  if (address !== RADIO && p) {
    const wasFront = recency[0] === address;
    pages = pages.filter((x) => x.address !== address); recency = recency.filter((a) => a !== address);
    lastClosed = p;
    await noteRecent(storage, 'close', address);
    if (wasFront && p.opener !== '' && pages.some((x) => x.address === p.opener)) focusLocked(p.opener);
  }
  await save(storage, gen); await mirror(gen); return view();
}
export async function pagesReopen(storage: Storage | undefined): Promise<PageList> {
  await load(storage);
  const p = lastClosed; lastClosed = null;
  if (!p) return view();
  const gen = accountGeneration();
  openLocked({ ...p });                                                    // the opener it had is kept (the Swift reopen does the same)
  await save(storage, gen);
  await mirror(gen);
  return view();
}
export async function pagesPin(storage: Storage | undefined, rawAddress: string): Promise<PageList> {
  const gen = accountGeneration(); await load(storage);
  const address = canonical(rawAddress);
  const p = pages.find((x) => x.address === address); if (p) p.pinned = !p.pinned;
  await save(storage, gen); await mirror(gen); return view();
}

/** Sign-out and tests: only the radio stays; the file is removed by the account wipe. */
export function forget(): void { pages = []; recency = []; lastClosed = null; loaded = false; loading = null; epoch += 1; saving = Promise.resolve(); mirrored = false; lastRequest = 0; }
export async function wipePages(storage: Storage | undefined): Promise<void> { forget(); if (!storage) return; try { await storage.fs.rm(PAGES_FILE); } catch { /* absent */ } }
