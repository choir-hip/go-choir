// The content store opened and synced in shared code. The store and the sync are content.ts; this file is the part Swift did at boot (ContentBoot, ContentSync.kick, its radio gate)
// and the narrow reads other lanes use. Tests: content-run.test.ts (S1 to S8).
//
// Narrow reads for other lanes (the Swift consumers of the store read nothing now): `contentDocs(storage)` every held item as search documents (retrieval), `contentAt(storage, address)` the item that
// opens at an address (the reader offline), `localStories(items, ...)` the stories the radio can play when the desk cannot be reached (the run sheet writer).
//
// The one native fact: `{op: 'contentBusy'}` on the app module (AutoradioModule+Notebook.swift `notebookOp`): whether remote audio is on air. The player is native; a sync does not take the network from it.
import type { Storage } from './app.contract.d.ts';
import { ContentStore, CONTENT_DB, syncAll, liveFetchers, toDoc, type Db, type Fetchers, type StoredItem } from './content.ts';
import { getJSON, nativeLater, SignedOutError } from './api.ts';
import { cut } from './takes.ts';
import type { Doc } from './search.ts';

type Closable = Db & { close?: () => Promise<void> };
let held: { store: ContentStore; db: Closable; gen: number } | null = null;
let opening: Promise<ContentStore | null> | null = null;
export function forgetContent(): void { held = null; opening = null; last = null; running = null; }

/** The one store of this process, made on first use. After a sign-out wipe (the generation moved; the file is gone) the old handle is closed and a fresh store opens. Null when it cannot open. */
export function contentStore(storage: Storage | undefined): Promise<ContentStore | null> {
  if (held && held.store.generation === held.gen) return Promise.resolve(held.store);
  if (opening) return opening;
  opening = (async () => {
    try {
      if (held) { held.store.dispose(); try { await held.db.close?.(); } catch { /* closed */ } held = null; }
      if (!storage) return null;
      const db = (await storage.sqlite.open(CONTENT_DB)) as Closable;
      const store = new ContentStore(db); await store.init();
      held = { store, db, gen: store.generation };
      return store;
    } catch { return null; } finally { opening = null; }
  })();
  return opening;
}

export type SyncInfo = { state: 'done' | 'signedout' | 'offline' | 'unavailable'; line: string };
export type Env = { fetchers?: Fetchers; now?: () => Promise<number | null>; busy?: () => Promise<boolean> };
const serverNow = async (): Promise<number | null> => { try { return (await getJSON('/health')).serverTime; } catch { return null; } };
const nativeBusy = async (): Promise<boolean> => {
  const l = nativeLater() as unknown as ((r: Record<string, unknown>) => Promise<unknown>) | null;
  if (!l) return false;
  try { const r = (await l({ op: 'contentBusy' })) as { busy?: unknown } | null; return r?.busy === true; } catch { return false; }
};

let last: { key: string; info: SyncInfo } | null = null;
let running: { key: string; p: Promise<SyncInfo> } | null = null;

/** Runs the sync once per key (the app's launch and each return to the front); the same key answers the last result. Never throws; logs hold counts, never a word. */
export function syncContent(key: string, storage: Storage | undefined, env: Env = {}): Promise<SyncInfo> {
  if (last?.key === key) return Promise.resolve(last.info);
  if (running?.key === key) return running.p;
  const p = run(storage, env).then((info) => { if (info.state !== 'unavailable' && info.state !== 'offline') last = { key, info }; return info; }).finally(() => { running = null; });
  running = { key, p };
  return p;
}

async function run(storage: Storage | undefined, env: Env): Promise<SyncInfo> {
  const store = await contentStore(storage);
  if (!store) return { state: 'unavailable', line: 'The phone\'s copy cannot be opened.' };
  const now = await (env.now ?? serverNow)();
  if (now === null) return { state: 'offline', line: 'No server time: nothing synced.' };
  const base = env.fetchers ?? liveFetchers();
  // syncAll turns a failing list into an "error" report; a refusal for want of a session is told apart here.
  let signedOut = false;
  const watched = Object.fromEntries(Object.entries(base).map(([k, fn]) => [k, (...a: unknown[]) => {
    try { const r = (fn as (...x: unknown[]) => unknown)(...a); return r instanceof Promise ? r.catch((e) => { if (e instanceof SignedOutError) signedOut = true; throw e; }) : r; }
    catch (e) { if (e instanceof SignedOutError) signedOut = true; throw e; }
  }])) as unknown as Fetchers;
  const f: Fetchers = { ...watched, audioBusy: env.busy ?? nativeBusy };
  try {
    const reports = await syncAll(store, f, { now, log: (l) => console.log(l) });
    if (signedOut) return { state: 'signedout', line: 'Signed out: nothing synced.' };
    const stopped = reports.find((r) => r.stoppedBy !== '');
    if (reports.length > 0 && reports.every((r) => r.stoppedBy === 'error') ) return { state: 'offline', line: 'The computer could not be reached.' };
    return { state: 'done', line: reports.map((r) => `${r.fetched} ${r.kind === 'story' ? 'stories' : r.kind + 's'}`).join(', ') + (stopped ? `; stopped: ${stopped.stoppedBy}` : '') };
  } catch (e) {
    if (e instanceof SignedOutError) return { state: 'signedout', line: 'Signed out: nothing synced.' };
    return { state: 'offline', line: 'The computer could not be reached.' };
  }
}

// ---------------------------------------------------------------------------------------------------------------- narrow reads

export async function contentDocs(storage: Storage | undefined): Promise<Doc[]> { const s = await contentStore(storage); return s ? s.docs() : []; }
export async function contentAt(storage: Storage | undefined, address: string): Promise<StoredItem | null> { const s = await contentStore(storage); return s ? s.itemAt(address) : null; }

export type LocalStory = { id: string; title: string; text: string; address: string };
/** The newest held stories not already on the sheet (`told`: their titles; `held`: ids on the sheet): what plays when the desk cannot be reached. Ported from the Swift LocalStories. */
export function localStories(items: StoredItem[], o: { told: string[]; held: Set<string>; limit?: number }): LocalStory[] {
  const told = new Set(o.told.map((t) => t.trim().toLowerCase())), out: LocalStory[] = [];
  for (const it of [...items].filter((x) => x.kind === 'story').sort((a, b) => (b.date ?? -Infinity) - (a.date ?? -Infinity))) {
    const title = it.title.replace(/\s+/g, ' ').trim(), body = it.text.trim();
    if (title === '' || body === '' || told.has(title.toLowerCase())) continue;
    let id = 's-';
    for (const ch of it.id) { if (new TextEncoder().encode(id).length >= 64) break; id += /[0-9A-Za-z_.-]/.test(ch) ? ch : '-'; }
    if (o.held.has(id)) continue;
    out.push({ id, title, text: cut(body, 16 * 1024), address: it.address });
    if (out.length >= (o.limit ?? 5)) break;
  }
  return out;
}
export { toDoc };
