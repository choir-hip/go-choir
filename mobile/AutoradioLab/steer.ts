// The listener's more / less / mute taps, sent to their computer as steering (a port of the Swift SteerTap, SteerClient and SteerQueue, Choir/Learning.swift, which are gone).
//   POST /api/autoradio/steer  {entry_id, direction, stories?, title?, handle?}   direction is `more`, `less` or `mute`
// A server that refuses the extra fields (400 or 422) gets the tap once more with only `direction`, `entry_id` and `handle`. A missing route (404, 501) or no network keeps the tap
// queued; a refusal for good (another 4xx but 429) drops it. The queue is a file (`app:/data/steer-queue.json`), at most 200 taps, the oldest go first; sign-out removes it (account.ts).
// The request goes through the one signed-in client (api.ts). Logs carry counts only, never a title or a story.
//
// Failure list (steer.test.ts): S1 a tap with no entry on air (and no handle) is not made. S2 the 201st tap drops the oldest. S3 404/501 keeps the tap and stops the flush.
// S4 400 resends bare; a second 400 drops it. S5 a 5xx or a transport error keeps it. S6 signed out keeps it. S7 a sign-out during a flush leaves no file.
// S8 two flushes at once send each tap once. S9 the story routes of a tap lose their version and are unique, at most 12.
import type { Storage } from './app.contract.d.ts';
import { currentWorld, accountGeneration, authedJSON, HTTPError, SignedOutError, putFile, scoped } from './api.ts';
import { onAirEntry, type Entry, type Sheet } from './runsheet-model.ts';

// Module state belongs to the world (api.ts `scoped`): a closed world's is unreachable and no later world joins it.
const S = scoped(() => ({ counter: 0, taps: [] as SteerTap[], loaded: false, loading: null as Promise<void> | null, flushing: null as Promise<void> | null, epoch: 0, state: "idle", saving: Promise.resolve() as Promise<unknown> }));

export const STEER_FILE = 'app:/data/steer-queue.json';
export const STEER_PATH = '/api/autoradio/steer';
export const MAX_TAPS = 200;

export type SteerTap = { id: string; entryID: string; direction: 'more' | 'less' | 'mute'; stories: string[]; title: string; handle?: string; at: string };
export type SteerSend = 'sent' | 'notYet' | 'offline' | 'refused';
export type Post = (body: Record<string, unknown>) => Promise<void>;

/** The route a `pub:<route>[@v<n>]` ref names, without the version. */
export function storyRoute(ref: string): string {
  const r = ref.startsWith('pub:') ? ref.slice(4) : ref;
  return r.split('@')[0];
}

const cut = (s: string, bytes: number): string => {
  const enc = new TextEncoder();
  if (enc.encode(s).length <= bytes) return s;
  let out = s;
  while (out.length > 0 && enc.encode(out).length > bytes) out = out.slice(0, -1);
  return out;
};

const stamp = (now: number): string => (now > 0 ? new Date(now * 1000).toISOString().replace(/\.\d+Z$/, 'Z') : '');
const newID = (): string => 's-' + Array.from(crypto.getRandomValues(new Uint8Array(6)), (x) => x.toString(16).padStart(2, '0')).join('') + (S().counter++).toString(36);

/** A tap on the entry on air: the entry's id, the stories it cites and the one the listener tapped (when it is another). Null with nothing on air (S1). */
export function tapOn(entry: Entry | undefined, direction: 'more' | 'less', tappedStory: string | undefined, now = 0): SteerTap | null {
  if (!entry) return null;
  const stories: string[] = [];
  const add = (r: string): void => { if (r !== '' && !stories.includes(r)) stories.push(r); };
  for (const c of entry.cites ?? []) if (c.ref.startsWith('pub:')) add(storyRoute(c.ref));
  if (tappedStory && tappedStory.startsWith('pub:')) add(storyRoute(tappedStory));
  return { id: newID(), entryID: entry.id, direction, stories, title: entry.title, at: stamp(now) };
}
export function mute(handle: string, now = 0): SteerTap {
  return { id: newID(), entryID: '', direction: 'mute', stories: [], title: '', handle, at: stamp(now) };
}
/** The entry on air in a run sheet the native side handed over as JSON (the radio page's facts). */
export function onAir(sheet: Sheet | undefined): Entry | undefined { return sheet ? onAirEntry(sheet) : undefined; }

export function body(t: SteerTap, bare: boolean): Record<string, unknown> {
  const b: Record<string, unknown> = { direction: t.direction };
  if (t.entryID !== '') b.entry_id = t.entryID;
  if (!bare) {
    if (t.stories.length > 0) b.stories = t.stories.slice(0, 12);
    if (t.title !== '') b.title = cut(t.title, 300);
  }
  if (t.handle) b.handle = t.handle;
  return b;
}

const realPost: Post = async (b) => { await authedJSON(STEER_PATH, { method: 'POST', body: b, world: currentWorld() }); };

export async function sendOne(t: SteerTap, post: Post = realPost): Promise<SteerSend> {
  const status = (e: unknown): number => (e instanceof HTTPError ? e.status : 0);
  try { await post(body(t, false)); return 'sent'; } catch (e) {
    if (e instanceof SignedOutError) return 'offline';
    const s = status(e);
    if (s === 404 || s === 501) return 'notYet';
    if (s === 400 || s === 422) {
      try { await post(body(t, true)); return 'sent'; } catch (e2) {
        const s2 = status(e2);
        if (s2 === 404 || s2 === 501) return 'notYet';
        if (s2 >= 400 && s2 < 500) return 'refused';
        return 'offline';
      }
    }
    if (s >= 400 && s < 500 && s !== 429) return 'refused';
    return 'offline';
  }
}


const valid = (t: unknown): t is SteerTap => {
  const o = t as Partial<SteerTap> | null;
  return !!o && typeof o.id === 'string' && o.id !== '' && typeof o.entryID === 'string' && (o.direction === 'more' || o.direction === 'less' || o.direction === 'mute') && Array.isArray(o.stories) && typeof o.title === 'string' && typeof o.at === 'string';
};

async function load(storage: Storage | undefined): Promise<void> {
  if (S().loaded) return;
  if (!S().loading) {
    const mine = S().epoch;
    const p: Promise<void> = (async () => {
      let stored: unknown = null;
      if (storage) { try { stored = JSON.parse(new TextDecoder().decode(await storage.fs.readFile(STEER_FILE))); } catch { /* none kept */ } }
      if (mine !== S().epoch) return;
      S().taps = (Array.isArray(stored) ? stored.filter(valid) : []).slice(-MAX_TAPS);
      S().loaded = true;
    })().finally(() => { if (S().loading === p) S().loading = null; });
    S().loading = p;
  }
  await S().loading;
}

function save(storage: Storage | undefined, gen: number): Promise<void> {
  const run = S().saving.then(() => putFile(storage, STEER_FILE, new TextEncoder().encode(JSON.stringify(S().taps))), () => undefined);
  S().saving = run.catch(() => undefined);
  return run.then(() => undefined);
}

export async function queued(storage: Storage | undefined): Promise<number> { await load(storage); return S().taps.length; }
export const steerState = (): string => S().state;

export async function enqueue(storage: Storage | undefined, t: SteerTap): Promise<void> {
  const gen = accountGeneration();
  await load(storage);
  if (gen !== accountGeneration()) return;
  S().taps.push(t);
  if (S().taps.length > MAX_TAPS) S().taps = S().taps.slice(S().taps.length - MAX_TAPS);
  await save(storage, gen);
}

/** Send what waits, in order, one flush at a time (S8). */
export function flush(storage: Storage | undefined, post: Post = realPost): Promise<void> {
  const cur = S().flushing; if (cur) return cur;
  const p: Promise<void> = (async () => {
    const gen = accountGeneration();
    await load(storage);
    let sent = 0;
    for (;;) {
      const tap = S().taps[0];
      if (!tap || gen !== accountGeneration()) break;
      const r = await sendOne(tap, post);
      if (gen !== accountGeneration()) break;                        // signed out meanwhile: the wipe owns the queue now
      if (r === 'sent') { S().taps = S().taps.filter((x) => x.id !== tap.id); S().state = 'sent'; sent += 1; }
      else if (r === 'refused') S().taps = S().taps.filter((x) => x.id !== tap.id);
      else S().state = r === 'notYet' ? 'notyet' : 'offline';
      await save(storage, gen);
      if (r === 'notYet' || r === 'offline') break;
    }
    console.log(`steer: sent=${sent} queued=${S().taps.length}`);
  })().finally(() => { if (S().flushing === p) S().flushing = null; });
  S().flushing = p;
  return p;
}

/** Sign-out and tests. */
export function forget(): void { S().taps = []; S().loaded = false; S().loading = null; S().flushing = null; S().state = 'idle'; S().epoch += 1; }
