// The deck's open pages, in shared code (a port of the Swift OpenPages, Pages.swift): which pages are open, which is current, open / close / focus / pin / reopen, and the file that keeps
// them between launches. It is what lets the deck mount a page on a host with no native package (Android), and what iOS will read once the Swift set goes. The set is addresses and
// titles only (never resumed work); the Autoradio page (`radio:`) is always open and cannot be closed; the pile keeps the 30 most recent unpinned pages; an address is open once.
// Sign-out: only the radio stays and the file goes (account.ts wipes it). Logs carry counts only.
import type { Storage } from './app.contract.d.ts';
import { accountGeneration, writeCurrent, scoped } from './api.ts';

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
    case 'settings': return 'settings'; case 'lab': return 'lab'; case 'books': return 'books'; case 'doc': return 'doc'; case 'conversation': return 'conversation';
    case 'podcasts': case 'podcast': return 'podcast'; case 'onboarding': return 'onboarding'; default: return 'unknown';
  }
}
export function defaultTitle(address: string): string {
  const rest = address.includes(':') ? address.slice(address.indexOf(':') + 1) : '';
  switch (address) {
    case 'radio:': return 'Autoradio'; case 'notebook:': return 'My notebook'; case 'paper:front': return 'Paper'; case 'textures:': return 'Textures'; case 'activity:': return 'Activity';
    case 'settings:': case 'settings:root': return 'Settings'; case 'lab:components': return 'Components'; case 'lab:trace': return 'Trace'; case 'conversation:': return 'Conversation';
    case 'onboarding:setup': return 'Make it yours'; case 'onboarding:edit': return 'My edition'; case 'mailbox:inbox': return 'Mail'; case 'podcasts:': return 'Podcasts';
    case 'file:/': case 'file:': return 'Files'; case 'books:': return 'Reader';
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

const S = scoped(() => ({ pages: [] as Page[], recency: [] as string[], lastClosed: null as Page | null, loaded: false, loading: null as Promise<void> | null, saving: Promise.resolve() as Promise<unknown> }));      // module state belongs to the world (api.ts `scoped`)

const radioPage = (): Page => ({ address: RADIO, kind: 'radio', title: 'Autoradio', opener: '', pinned: false });
function focusLocked(address: string): void { S().recency = [address, ...S().recency.filter((a) => a !== address)]; }

function openLocked(p: Page): Page {
  const i = S().pages.findIndex((x) => x.address === p.address);
  if (i >= 0) {
    if (p.title !== '' && S().pages[i].title === defaultTitle(p.address)) S().pages[i].title = p.title;
    focusLocked(p.address); return S().pages[i];
  }
  S().pages.push(p); focusLocked(p.address);
  for (;;) {                                                               // the pile keeps the 30 most recent unpinned pages; the radio is never trimmed
    if (S().pages.filter((x) => !x.pinned && x.address !== RADIO).length <= LIMIT) break;
    const oldest = [...S().recency].reverse().find((a) => a !== RADIO && S().pages.find((x) => x.address === a)?.pinned === false);
    if (oldest === undefined) break;
    S().pages = S().pages.filter((x) => x.address !== oldest); S().recency = S().recency.filter((a) => a !== oldest);
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
  for (let tries = 0; !S().loaded && tries < 3; tries++) {
    if (!S().loading) {
      const s = S();                                                       // this load fills the state it began for: a wipe that resets the state mid-read leaves the old one to be dropped
      const p: Promise<void> = (async () => {
        let stored: unknown = null;
        if (storage) { try { stored = JSON.parse(new TextDecoder().decode(await storage.fs.readFile(PAGES_FILE))); } catch { /* none kept yet, or damaged: the radio alone */ } }
        const v = validated(stored);
        if (S() !== s) return;                                             // wiped while reading: this is the old account's file
        s.pages = v.pages; s.recency = v.recency;
        openLocked(radioPage());                                           // every launch lands on the radio
        s.loaded = true;
      })().finally(() => { if (s.loading === p) s.loading = null; });
      s.loading = p;
    }
    await S().loading;
  }
}

/** Saves run one after another and each writes the set as it is WHEN IT RUNS, so the last state wins the file whatever order the host finishes the writes in. */
function save(storage: Storage | undefined, gen: number): Promise<void> {
  const run = S().saving.then(() => writeCurrent(gen, storage, PAGES_FILE, new TextEncoder().encode(JSON.stringify({ pages: S().pages, recency: S().recency }))), () => undefined);   // a sign-out in flight writes nothing
  S().saving = run.catch(() => undefined);
  return run.then(() => undefined);
}

function view(): PageList {
  const rows = S().recency.flatMap((a) => S().pages.filter((x) => x.address === a));
  const top = rows[0];
  return { rows, top: top?.address ?? '', topKind: top?.kind ?? '', topTitle: top?.title ?? '', count: S().pages.length };
}

/** Read the set. */
export async function pagesView(storage: Storage | undefined): Promise<PageList> { await load(storage); return view(); }

export async function pagesOpen(storage: Storage | undefined, rawAddress: string, title: string): Promise<PageList> {
  const gen = accountGeneration();
  await load(storage);
  if (gen !== accountGeneration()) return view();                          // a sign-out began while the set loaded: nothing of this open is kept
  const address = canonical(rawAddress);
  openLocked({ address, kind: kindOf(address), title: title !== '' ? title : defaultTitle(address), opener: S().recency[0] ?? '', pinned: false });
  await save(storage, gen);
  return view();
}
export async function pagesFocus(storage: Storage | undefined, rawAddress: string): Promise<PageList> {
  const gen = accountGeneration(); await load(storage);
  const address = canonical(rawAddress);
  if (S().pages.some((x) => x.address === address)) focusLocked(address);
  await save(storage, gen); return view();
}
/** Close a page. When it was in front, its opener (if still open) comes forward, else the next most recent. */
export async function pagesClose(storage: Storage | undefined, rawAddress: string): Promise<PageList> {
  const gen = accountGeneration(); await load(storage);
  const address = canonical(rawAddress);
  const p = S().pages.find((x) => x.address === address);
  if (address !== RADIO && p) {
    const wasFront = S().recency[0] === address;
    S().pages = S().pages.filter((x) => x.address !== address); S().recency = S().recency.filter((a) => a !== address);
    S().lastClosed = p;
    if (wasFront && p.opener !== '' && S().pages.some((x) => x.address === p.opener)) focusLocked(p.opener);
  }
  await save(storage, gen); return view();
}
export async function pagesReopen(storage: Storage | undefined): Promise<PageList> {
  await load(storage);
  const p = S().lastClosed; S().lastClosed = null;
  if (!p) return view();
  const gen = accountGeneration();
  openLocked({ ...p });                                                    // the opener it had is kept (the Swift reopen does the same)
  await save(storage, gen);
  return view();
}
export async function pagesPin(storage: Storage | undefined, rawAddress: string): Promise<PageList> {
  const gen = accountGeneration(); await load(storage);
  const address = canonical(rawAddress);
  const p = S().pages.find((x) => x.address === address); if (p) p.pinned = !p.pinned;
  await save(storage, gen); return view();
}

/** Sign-out and tests: only the radio stays; the file is removed by the account wipe. */
export function forget(): void { S.reset(); }
export async function wipePages(storage: Storage | undefined): Promise<void> { forget(); if (!storage) return; try { await storage.fs.rm(PAGES_FILE); } catch { /* absent */ } }
