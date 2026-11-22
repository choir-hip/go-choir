// The pages opened or closed lately, for the launcher's "Recent" rows (a port of the Swift RecentPages over the Swift signal log, Signals.swift, which is gone).
// pages.ts records an open and a close here; cards.ts reads the newest first, without the pages open now. Addresses only, never a title or a word the listener said.
// Sign-out: the ring and its file go (account.ts WIPES, `recent-pages`).
import type { Storage } from './app.contract.d.ts';
import { accountGeneration, putFile, scoped } from './api.ts';
import { kindOf } from './pages.ts';

// Module state belongs to the world (api.ts `scoped`): a closed world's is unreachable and no later world joins it.
const S = scoped(() => ({ ring: [] as RecentEvent[], loaded: false, loading: null as Promise<void> | null, epoch: 0, saving: Promise.resolve() as Promise<unknown> }));

export const RECENT_FILE = 'app:/data/recent-pages.json';
/** What the file keeps: the newest of each address is all `addresses` can use, so the ring is short. */
export const KEEP = 60;

export type RecentEvent = { at: number; kind: 'open' | 'close'; address: string };


async function load(storage: Storage | undefined): Promise<void> {
  if (S().loaded) return;
  if (!S().loading) {
    const mine = S().epoch;
    const p: Promise<void> = (async () => {
      let stored: unknown = null;
      if (storage) { try { stored = JSON.parse(new TextDecoder().decode(await storage.fs.readFile(RECENT_FILE))); } catch { /* none kept yet, or damaged: empty */ } }
      if (mine !== S().epoch) return;
      const events = Array.isArray((stored as { events?: unknown } | null)?.events) ? ((stored as { events: unknown[] }).events) : [];
      S().ring = events.filter((e): e is RecentEvent => {
        const o = e as Partial<RecentEvent> | null;
        return !!o && typeof o.at === 'number' && Number.isFinite(o.at) && (o.kind === 'open' || o.kind === 'close') && typeof o.address === 'string' && o.address !== '';
      }).slice(-KEEP);
      S().loaded = true;
    })().finally(() => { if (S().loading === p) S().loading = null; });
    S().loading = p;
  }
  await S().loading;
}

function save(storage: Storage | undefined, gen: number): Promise<void> {
  const run = S().saving.then(() => putFile(storage, RECENT_FILE, new TextEncoder().encode(JSON.stringify({ events: S().ring }))), () => undefined);
  S().saving = run.catch(() => undefined);
  return run.then(() => undefined);
}

/** A page was opened or closed. */
/** There is no clock in a data source: `at` is a sequence number, the order of the events (one more than the newest kept). */
export async function note(storage: Storage | undefined, kind: 'open' | 'close', address: string): Promise<void> {
  const gen = accountGeneration();
  await load(storage);
  if (gen !== accountGeneration()) return;                      // a sign-out began while the ring loaded: nothing of this is kept
  S().ring.push({ at: S().ring.reduce((m, e) => Math.max(m, e.at), 0) + 1, kind, address });
  if (S().ring.length > KEEP) S().ring = S().ring.slice(-KEEP);
  await save(storage, gen);
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

export async function events(storage: Storage | undefined): Promise<RecentEvent[]> { await load(storage); return [...S().ring]; }

/** Sign-out and tests. */
export function forget(): void { S().ring = []; S().loaded = false; S().loading = null; S().epoch += 1; }
