// The deck's open pages, in shared code (a port of the Swift OpenPages, Pages.swift): which pages are open, which is current, open / close / focus / pin / reopen, and the file that keeps
// them between launches. It is what lets the deck mount a page on a host with no native package (Android), and what iOS will read once the Swift set goes. The set is addresses and
// titles only (never resumed work); the Autoradio page (`radio:`) is always open and cannot be closed; the pile keeps the 30 most recent unpinned pages; an address is open once.
// Sign-out: only the radio stays and the file goes (account.ts wipes it). Logs carry counts only.
import type { Storage } from './app.contract.d.ts';
import { accountGeneration, writeCurrent } from './api.ts';

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

let pages: Page[] = [];
let recency: string[] = [];
let lastClosed: Page | null = null;
let loaded = false;

const radioPage = (): Page => ({ address: RADIO, kind: 'radio', title: 'Autoradio', opener: '', pinned: false });
function focusLocked(address: string): void { recency = [address, ...recency.filter((a) => a !== address)]; }

function openLocked(p: Page): Page {
  const i = pages.findIndex((x) => x.address === p.address);
  if (i >= 0) {
    if (p.title !== '' && pages[i].title === defaultTitle(p.address)) pages[i].title = p.title;
    focusLocked(p.address); return pages[i];
  }
  pages.push(p); focusLocked(p.address);
  for (;;) {                                                               // the pile keeps the 30 most recent unpinned pages
    if (pages.filter((x) => !x.pinned).length <= LIMIT) break;
    const oldest = [...recency].reverse().find((a) => pages.find((x) => x.address === a)?.pinned === false);
    if (oldest === undefined) break;
    pages = pages.filter((x) => x.address !== oldest); recency = recency.filter((a) => a !== oldest);
  }
  return p;
}

async function load(storage: Storage | undefined): Promise<void> {
  if (loaded) return;
  loaded = true;
  pages = []; recency = [];
  if (storage) {
    try {
      const s = JSON.parse(new TextDecoder().decode(await storage.fs.readFile(PAGES_FILE))) as { pages?: Page[]; recency?: string[] };
      const seen = new Set<string>();
      for (const q of Array.isArray(s.pages) ? s.pages : []) {
        if (!q || typeof q.address !== 'string') continue;
        const address = canonical(q.address);
        if (seen.has(address)) continue; seen.add(address);
        pages.push({ address, kind: kindOf(address), title: typeof q.title === 'string' ? q.title : defaultTitle(address), opener: typeof q.opener === 'string' ? q.opener : '', pinned: q.pinned === true });
      }
      const inRec = new Set<string>();
      recency = (Array.isArray(s.recency) ? s.recency : []).map(canonical).filter((a) => pages.some((x) => x.address === a) && !inRec.has(a) && (inRec.add(a), true));
      for (const q of pages) if (!recency.includes(q.address)) recency.push(q.address);
    } catch { /* none kept yet, or damaged: the radio alone */ }
  }
  openLocked(radioPage());                                                 // every launch lands on the radio
}

async function save(storage: Storage | undefined, gen: number): Promise<void> {
  await writeCurrent(gen, storage, PAGES_FILE, new TextEncoder().encode(JSON.stringify({ pages, recency })));   // a sign-out in flight writes nothing
}

function view(): PageList {
  const rows = recency.flatMap((a) => pages.filter((x) => x.address === a));
  const top = rows[0];
  return { rows, top: top?.address ?? '', topKind: top?.kind ?? '', topTitle: top?.title ?? '', count: pages.length };
}

/** Read the set. */
export async function pagesView(storage: Storage | undefined): Promise<PageList> { await load(storage); return view(); }

export async function pagesOpen(storage: Storage | undefined, rawAddress: string, title: string): Promise<PageList> {
  const gen = accountGeneration();
  await load(storage);
  const address = canonical(rawAddress);
  openLocked({ address, kind: kindOf(address), title: title !== '' ? title : defaultTitle(address), opener: recency[0] ?? '', pinned: false });
  await save(storage, gen);
  return view();
}
export async function pagesFocus(storage: Storage | undefined, address: string): Promise<PageList> {
  const gen = accountGeneration(); await load(storage);
  if (pages.some((x) => x.address === address)) focusLocked(address);
  await save(storage, gen); return view();
}
/** Close a page. When it was in front, its opener (if still open) comes forward, else the next most recent. */
export async function pagesClose(storage: Storage | undefined, address: string): Promise<PageList> {
  const gen = accountGeneration(); await load(storage);
  const p = pages.find((x) => x.address === address);
  if (address !== RADIO && p) {
    const wasFront = recency[0] === address;
    pages = pages.filter((x) => x.address !== address); recency = recency.filter((a) => a !== address);
    lastClosed = p;
    if (wasFront && p.opener !== '' && pages.some((x) => x.address === p.opener)) focusLocked(p.opener);
  }
  await save(storage, gen); return view();
}
export async function pagesReopen(storage: Storage | undefined): Promise<PageList> {
  await load(storage);
  const p = lastClosed; lastClosed = null;
  return p ? pagesOpen(storage, p.address, p.title) : view();
}
export async function pagesPin(storage: Storage | undefined, address: string): Promise<PageList> {
  const gen = accountGeneration(); await load(storage);
  const p = pages.find((x) => x.address === address); if (p) p.pinned = !p.pinned;
  await save(storage, gen); return view();
}

/** Sign-out and tests: only the radio stays; the file is removed by the account wipe. */
export function forget(): void { pages = []; recency = []; lastClosed = null; loaded = false; }
export async function wipePages(storage: Storage | undefined): Promise<void> { forget(); if (!storage) return; try { await storage.fs.rm(PAGES_FILE); } catch { /* absent */ } }
