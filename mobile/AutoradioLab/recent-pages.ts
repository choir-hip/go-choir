// The pages opened or closed lately, for the launcher's "Recent" rows (a port of the Swift RecentPages over the Swift signal log, Signals.swift, which is gone).
// pages.ts records an open and a close here; cards.ts reads the newest first, without the pages open now. Addresses only, never a title or a word the listener said.
// Sign-out: the ring and its file go (account.ts WIPES, `recent-pages`).
import type { Storage } from './app.contract.d.ts';
import { accountGeneration, writeCurrent } from './api.ts';
import { kindOf } from './pages.ts';

export const RECENT_FILE = 'app:/data/recent-pages.json';
/** What the file keeps: the newest of each address is all `addresses` can use, so the ring is short. */
export const KEEP = 60;

export type RecentEvent = { at: number; kind: 'open' | 'close'; address: string };

let ring: RecentEvent[] = [];
let loaded = false;
let loading: Promise<void> | null = null;
let epoch = 0;                                  // bumped by a wipe: a read that began before it belongs to the old account

async function load(storage: Storage | undefined): Promise<void> {
  if (loaded) return;
  if (!loading) {
    const mine = epoch;
    const p: Promise<void> = (async () => {
      let stored: unknown = null;
      if (storage) { try { stored = JSON.parse(new TextDecoder().decode(await storage.fs.readFile(RECENT_FILE))); } catch { /* none kept yet, or damaged: empty */ } }
      if (mine !== epoch) return;
      const events = Array.isArray((stored as { events?: unknown } | null)?.events) ? ((stored as { events: unknown[] }).events) : [];
      ring = events.filter((e): e is RecentEvent => {
        const o = e as Partial<RecentEvent> | null;
        return !!o && typeof o.at === 'number' && Number.isFinite(o.at) && (o.kind === 'open' || o.kind === 'close') && typeof o.address === 'string' && o.address !== '';
      }).slice(-KEEP);
      loaded = true;
    })().finally(() => { if (loading === p) loading = null; });
    loading = p;
  }
  await loading;
}

let saving: Promise<unknown> = Promise.resolve();
/** The write is issued NOW, from the answer that called: Exact's one storage queue runs the writes in the order they were issued, so the last state wins the file, and a call made after the answer ended
 * is refused (reference.md, "refused in background work"), which a write chained on an earlier one would be. A failed write is logged. */
function save(storage: Storage | undefined, gen: number): Promise<void> {
  const run = writeCurrent(gen, storage, RECENT_FILE, new TextEncoder().encode(JSON.stringify({ events: ring }))).then((ok) => {
    if (!ok && storage && gen === accountGeneration()) console.log('recent-pages: the recent pages were not saved');
  });
  saving = Promise.all([saving, run]);
  return run;
}
/** For tests: every write issued so far has finished. */
export async function recentSettled(): Promise<void> { await saving; }

/** A page was opened or closed. */
/** There is no clock in a data source: `at` is a sequence number, the order of the events (one more than the newest kept). */
export async function note(storage: Storage | undefined, kind: 'open' | 'close', address: string): Promise<void> {
  const gen = accountGeneration();
  await load(storage);
  if (gen !== accountGeneration()) return;                      // a sign-out began while the ring loaded: nothing of this is kept
  await noteNow(storage, kind, address, gen);
}

/** The ring is read (the first note of a launch waits for it; a page's open does not wait for the note itself, only for this). */
export const isLoaded = (): boolean => loaded;

/** The note and its write, started now and not waited for by the caller that does not await the result. The ring must be loaded. */
export function noteNow(storage: Storage | undefined, kind: 'open' | 'close', address: string, gen: number = accountGeneration()): Promise<void> {
  if (gen !== accountGeneration()) return Promise.resolve();
  ring.push({ at: ring.reduce((m, e) => Math.max(m, e.at), 0) + 1, kind, address });
  if (ring.length > KEEP) ring = ring.slice(-KEEP);
  return save(storage, gen);
}

/** Addresses opened or closed lately, newest first, without those open now, one entry each, none with an unknown scheme (it would open a "not built yet" page). */
export function addresses(events: RecentEvent[], excluding: Set<string>, limit = 8): string[] {
  const seen = new Set(excluding);
  const out: string[] = [];
  for (const e of [...events].sort((a, b) => b.at - a.at)) {
    if (seen.has(e.address) || kindOf(e.address) === 'unknown') continue;
    seen.add(e.address); out.push(e.address);
    if (out.length === limit) break;
  }
  return out;
}

export async function events(storage: Storage | undefined): Promise<RecentEvent[]> { await load(storage); return [...ring]; }

/** Sign-out and tests. */
export function forget(): void { ring = []; loaded = false; loading = null; epoch += 1; saving = Promise.resolve(); }
export async function wipeRecent(storage: Storage | undefined): Promise<void> { forget(); if (!storage) return; try { await storage.fs.rm(RECENT_FILE); } catch { /* absent */ } }
