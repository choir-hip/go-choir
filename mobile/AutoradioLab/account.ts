// THE ACCOUNT BOUNDARY (docs/design/account-boundary-2026-11-21.md). The phone must never show one account's private data to another.
//
// The boundary is NOT remembered. It is a fact worked out again, before every page answer that touches account data, from two durable facts:
//   the KEY     the sign-in key in the Keychain, read through the app module: absent (the Keychain said item-not-found), present (with its SHA-256), unreadable, or no module at all;
//   the RECORD  `app:/data/account-stamp`: the SHA-256 of the key of the account whose data the phone's stores hold; absent only when the read rejects ENOENT.
// A page may read or write in exactly two cases (the table below): the record and the key name the same account (OPEN), or there is no record and no key (EMPTY).
// Every other case refuses (app.ts `answer` throws AccountClosedError) and the one writer, here, works to put it right: it wipes every store and THEN removes the record.
// Nothing about the boundary is kept in memory between answers except the generation (api.ts: a number that only drops work begun before a wipe), the counter the card
// reads as its epoch, and the names of what the last wipe could not clear (for the card's words).
//
//   record \ key   absent     present, same   present, another   unreadable      no module
//   none           EMPTY      ADOPT           ADOPT              LOCKED          EMPTY
//   names the key  CLEARING   OPEN            CLEARING           LOCKED          LOCKED
//   another/torn   CLEARING   CLEARING        CLEARING           LOCKED          LOCKED
//   unreadable     LOCKED     LOCKED          LOCKED             LOCKED          LOCKED
//
// ONE WRITER: every job (Adopt, Clear, Sign-out) runs one at a time on one promise chain, and reads its facts INSIDE the job, so two answers cannot interleave between a read and
// the write it leads to. A fact that cannot be read refuses; it is never "nothing to protect". A wipe never deletes a key: only the person's sign-out does.
// Durable order: the record is written only when there is none, removed only after every wipe succeeded and nothing survived, never rewritten from one account to another.
// A native answer after its deadline has no continuation: it changes nothing, and the next answer finds the facts as they are.
import type { Storage, Store } from './app.contract.d.ts';
import * as podcast from './podcast.ts';
import { wipeMail } from './mail.ts';
import { wipeFiles } from './files.ts';
import { wipePaper, CACHE_FILE as PAPER_FRONT } from './paper.ts';
import { wipeKept, KEPT_FILE } from './takes.ts';
import { wipeNotebook, CACHE_FILE as NOTEBOOK_CACHE } from './notebook.ts';
import { VECTORS_DIR, wipeLiveVectors } from './search.ts';
import { CONTENT_DB, wipeLiveContent } from './content.ts';
import { forget as forgetClips } from './clips.ts';
import { wipePages, PAGES_FILE } from './pages.ts';
import { DEMO_MARKER } from './demo-leftovers.ts';
import { type World, nextGeneration, closeWorld, drainClosed, bindStorage, currentWorld, resetSession, beginSignOut, abortSignOut, keyFact, nativeLater, withDeadline, accountGeneration, type KeyFact } from './api.ts';
import { originalsLeft } from './mail-store.ts';
import { wipeTextureThemes, textureThemesLeft } from './themes.ts';

/** The record: the SHA-256 (64 lowercase hex) of the key of the account whose data the phone's stores hold. The boundary's own file: not in WIPES, removed last. */
export const STAMP_FILE = 'app:/data/account-stamp';

/** A page answer refused because the boundary is not open (or moved while the answer ran). The page shows its existing failure state; the card says why. */
export class AccountClosedError extends Error {
  state: string;
  constructor(state: string) { super(`the account boundary is closed (${state})`); this.name = 'AccountClosedError'; this.state = state; }
}

export type Wipe = { name: string; run(storage: Storage | undefined, store: Store | undefined): Promise<unknown> };

const rmAll = async (storage: Storage | undefined, paths: string[]): Promise<void> => {
  if (!storage) return;
  for (const p of paths) { try { await storage.fs.rm(p); } catch { /* absent, or kept: the survivor check below looks */ } }
};

/** Every store the shared modules keep. A module that moves in adds one line here (and its path to wipedPaths). The record is NOT here. */
export const WIPES: Wipe[] = [
  { name: 'podcasts', run: (s) => podcast.wipe(s) },
  { name: 'mail', run: (s) => wipeMail(s) },
  { name: 'files', run: (s) => wipeFiles(s) },
  { name: 'paper-front', run: (s) => wipePaper(s) },
  { name: 'takes', run: (s) => wipeKept(s) },
  { name: 'notebook', run: (s) => wipeNotebook(s) },
  { name: 'vectors', run: async (s) => { await wipeLiveVectors(s); await rmAll(s, [VECTORS_DIR]); } },       // the live in-memory store first, then the files
  { name: 'content', run: async (s) => { await wipeLiveContent(); await rmAll(s, [CONTENT_DB, `${CONTENT_DB}-wal`, `${CONTENT_DB}-shm`]); } },  // stops a running sync (generation), empties the open db
  { name: 'pages', run: (s) => wipePages(s) },                                                                  // the deck's open set (pages.ts): only the radio stays
  { name: 'clips', run: async () => forgetClips() },                                                              // in memory only (public clips, but another account's page asked for them)
  { name: 'texture-themes', run: (s) => wipeTextureThemes(s) },                                                    // themes installed from a Texture document: the old account's document's name and colours
  { name: 'demo-leftovers', run: (s) => rmAll(s, [DEMO_MARKER]) },                                                  // a marker, not data: it only lets the one-time pass run again
];

/** (A function, not a constant: api.ts imports app.ts, which imports this, so a constant read at load can meet a module not yet initialised.)
 * The paths the wipes above cover (account.test.ts S4 compares them with every `app:/` literal in the shared modules); each is looked at after a wipe: a survivor is a failure.
 * `app:/tmp/picked` is files'. */
export const wipedPaths = (): string[] => [
  podcast.LIBRARY_FILE, podcast.FEED_DIR, 'app:/data/mail', 'app:/data/recent-files.json', 'app:/cache/files', 'app:/tmp/picked', PAPER_FRONT, KEPT_FILE, NOTEBOOK_CACHE,
  VECTORS_DIR, CONTENT_DB, PAGES_FILE, DEMO_MARKER,
];

/** Deadlines, not limits. `sharedMs`: the whole shared half of one Clear (a store that never lets go, or a live content sync that never ends, must not hold the one writer). `nativeMs`: one native sign-out or wipe. `recordMs`: one read of the record. (The key read's is api.ts `keyTiming`.) Tests shorten them. */
export const timing = { nativeMs: 30000, recordMs: 5000, sharedMs: 30000 };

/** Run each wipe on its own; the names of the ones that threw. Logs names, never content. It does not move the generation: a Clear attempt moves it once. */
export async function wipeAll(storage: Storage | undefined, store: Store | undefined, wipes_: Wipe[] = WIPES): Promise<string[]> {
  const failed: string[] = [];
  for (const w of wipes_) {
    try { await w.run(storage, store); } catch { failed.push(w.name); console.log(`account: wipe ${w.name} failed`); }
  }
  return failed;
}

// ---- the facts

const codeOf = (e: unknown): string => String((e as { code?: unknown } | null)?.code ?? '');
export type RecordFact = { k: 'none' } | { k: 'names'; digest: string } | { k: 'other' } | { k: 'unreadable' };
export type Facts = { record: RecordFact; key: KeyFact };
export type State = 'open' | 'empty' | 'adopt' | 'clearing' | 'locked';

/** The record. None ONLY when the read rejects ENOENT (Exact's storage code, the same on every host: reference.md, "storage refusal"); any other rejection, a read past its deadline, or no storage at all is unreadable. */
async function readRecord(storage: Storage | undefined): Promise<RecordFact> {
  if (!storage) return { k: 'unreadable' };
  type R = { ok: ArrayBuffer } | { err: unknown } | 'late';
  const r = await withDeadline<R>((async () => { try { return { ok: await storage.fs.readFile(STAMP_FILE) }; } catch (err) { return { err }; } })(), 'late', timing.recordMs);
  if (r === 'late') return { k: 'unreadable' };
  if ('err' in r) return codeOf(r.err) === 'ENOENT' ? { k: 'none' } : { k: 'unreadable' };
  const text = new TextDecoder().decode(r.ok);
  return /^[0-9a-f]{64}$/.test(text) ? { k: 'names', digest: text } : { k: 'other' };        // anything else (empty, short, upper case, a digest plus a list) names another
}

export async function readFacts(storage: Storage | undefined): Promise<Facts> {
  const [record, key] = await Promise.all([readRecord(storage), keyFact()]);
  return { record, key };
}

/** The table above. Pure. */
export function stateOf(f: Facts): State {
  if (f.record.k === 'unreadable' || f.key.state === 'unreadable') return 'locked';
  if (f.key.state === 'nomodule') return f.record.k === 'none' ? 'empty' : 'locked';       // no record is ever written on such a host: one there is not ours to judge
  if (f.record.k === 'none') return f.key.state === 'absent' ? 'empty' : 'adopt';
  if (f.key.state === 'absent') return 'clearing';
  return f.record.k === 'names' && f.record.digest === f.key.digest ? 'open' : 'clearing';
}

/** After a wipe, every path it covers is looked at: a path exists unless its stat rejects ENOENT. Most wipes swallow errors, so one that did nothing looks like one that worked. */
async function survivors(storage: Storage | undefined): Promise<string[]> {
  if (!storage) return ['storage'];
  const out: string[] = [];
  for (const p of wipedPaths()) {
    try { await storage.fs.stat(p); out.push(p.slice(p.lastIndexOf('/') + 1) || p); } catch (e) { if (codeOf(e) !== 'ENOENT') out.push(p.slice(p.lastIndexOf('/') + 1) || p); }
  }
  for (const p of [`${CONTENT_DB}-wal`, `${CONTENT_DB}-shm`]) {
    try { await storage.fs.stat(p); out.push(p.slice(p.lastIndexOf('/') + 1)); } catch (e) { if (codeOf(e) !== 'ENOENT') out.push(p.slice(p.lastIndexOf('/') + 1)); }
  }
  out.push(...(await originalsLeft(storage)), ...(await textureThemesLeft(storage)));
  return out;
}

/** The record removed, and confirmed: it reads ENOENT afterwards. */
async function removeRecord(storage: Storage): Promise<boolean> {
  // On the storage of the world this attempt began, under a deadline: a hung removal is a failed attempt (the record stays), and one that lands late is waited for by the next Clear (drain), never a wedged writer.
  try { await withDeadline(bindStorage(storage, currentWorld()).fs.rm(STAMP_FILE).then(() => undefined, () => undefined), undefined, timing.recordMs); } catch { /* the check below decides */ }
  return (await readRecord(storage)).k === 'none';
}

// ---- the native half, under a deadline

/** One native op with the generation it is for (shared code writes the one generation; native code receives it). False when it threw, did not answer in time, answered not done, or
 * reports `failed` stores. The answer after the deadline is dropped: there is no continuation. */
async function nativeCall(op: 'signOut' | 'wipe', generation: number): Promise<boolean> {
  const later = nativeLater();
  if (!later) return false;
  const call = (async (): Promise<boolean> => {
    try {
      const r = (await later({ op, generation } as never)) as { done?: unknown; failed?: unknown } | null;
      if (!r || r.done !== true) return false;
      const failed = r.failed;
      return !(Array.isArray(failed) ? failed.length > 0 : typeof failed === 'string' && failed !== '');
    } catch { return false; }
  })();
  return withDeadline(call, false, timing.nativeMs);
}

// ---- the one writer

let chain: Promise<unknown> = Promise.resolve();
let clearing: Promise<void> | null = null;
let wipes = 0;                                   // completed Clear attempts: the card's epoch moves with it
let boundaryId = 0;                              // moved by resetBoundary: a job of an earlier process (a test) does not touch this one
let lastFailed: string[] = [];                   // what the last Clear could not clear, for the card's words
/** Counts every completed Clear attempt (monotonic): the sources that keep state in memory are asked again when it changes. */
export const wipeEpoch = (): number => wipes;
export const lastFailure = (): string[] => lastFailed;

let adopting: Promise<State | undefined> | null = null;
let active = 0;                                  // jobs queued or running
/** The one chain. `skipIfMoved`: a job that was queued for one boundary does not run if the boundary moved before its turn (an Adopt or a Clear re-reads the facts anyway; the next answer asks again). A
 * sign-out is the person's act and always runs. There is no way to abandon a running job: a hung one ends by its deadline (a failed Clear), and the person's way out is on the card (close and reopen). */
export function job<T>(fn: () => Promise<T>, o: { skipIfMoved?: boolean } = {}): Promise<T | undefined> {
  active += 1;
  const at = accountGeneration();
  const run = async (): Promise<T | undefined> => (o.skipIfMoved && accountGeneration() !== at ? undefined : fn());
  const p = chain.then(run, run).finally(() => { active -= 1; });
  chain = p.then(() => undefined, () => undefined);
  return p;
}
/** Resolves when no job is queued or running (a Clear started by an answer is awaited here: tests, and nothing else). */
export async function idle(): Promise<void> { for (;;) { const c = chain; await c; if (c === chain) return; } }
/** For tests: a process that has just started. */
export function resetBoundary(): void { boundaryId += 1; chain = Promise.resolve(); clearing = null; adopting = null; active = 0; wipes = 0; lastFailed = []; timing.nativeMs = 30000; timing.recordMs = 5000; timing.sharedMs = 30000; }

/** ADOPT: no record and a key. Write the key's digest; the state after is read again. A failed write leaves it closed. */
async function adoptBody(storage: Storage | undefined): Promise<State> {
  const f = await readFacts(storage);
  if (stateOf(f) === 'adopt' && storage && f.key.state === 'present') {
    // The record write is on the storage of the current world and under a deadline: a hung write leaves the state ADOPT (refusing); a late landing is waited for by the next boundary move (drain).
    const wrote = await withDeadline<boolean>(bindStorage(storage, currentWorld()).fs.atomicWriteFile(STAMP_FILE, new TextEncoder().encode(f.key.digest)).then(() => true, () => false), false, timing.recordMs);
    if (wrote) closeWorld(); else console.log('account: could not record the account');      // the new account's world begins here (its requests are signed with this key only)
  }
  return stateOf(await readFacts(storage));
}

/** CLEAR, one attempt: the record names an account whose key is not stored. Wipe native, wipe every shared store, look for survivors; only then remove the record. */
async function clearBody(storage: Storage | undefined, store: Store | undefined): Promise<void> {
  const id = boundaryId;
  if (stateOf(await readFacts(storage)) !== 'clearing' || !storage) return;       // a job that finds the state changed does nothing
  const g = nextGeneration(); resetSession();                                      // the world ends here: moved once per attempt, by nothing else
  const drained = await drainClosed(timing.sharedMs);                              // every write the ended world STARTED lands (or fails) before anything is wiped: none lands after the wipe
  const nativeOk = await nativeCall('wipe', g);                                    // a wipe, never a sign-out: it keeps whatever key is stored
  // A shared half that outlives its deadline is a failed Clear: nothing is marked clean, the record stays, the next attempt runs every store again.
  // The shared half runs on the storage of the world this attempt began: when the deadline ends the attempt, the half that is still running is DETACHED, not cancelled; it can do nothing later, because the next
  // boundary move (the next attempt, or an Adopt) closes that world and every call of its storage then never arrives. A timed-out Clear leaves the phone in CLEARING with the record kept.
  const mine = bindStorage(storage, currentWorld());
  const shared = await withDeadline<string[]>((async () => [...(await wipeAll(mine, store, WIPES)), ...(await survivors(mine))])(), ['shared (timed out)'], timing.sharedMs);
  const failed = [...shared];
  if (!nativeOk) failed.push('native');
  if (!drained) failed.push('writes in flight');
  let removed = false;
  if (failed.length === 0) { removed = await removeRecord(storage); if (!removed) failed.push('account-stamp'); }
  if (id !== boundaryId) return;
  lastFailed = failed; wipes += 1;
  console.log(`account: cleared another account's data, ${failed.length} failed`);
  if (removed && stateOf(await readFacts(storage)) === 'adopt') await adoptBody(storage);    // a key is stored: it is the new account's, on empty stores
}

/** Start a Clear unless one is queued or running. Never waited on by an answer. */
function startClear(storage: Storage | undefined, store: Store | undefined): void {
  if (clearing) return;
  const p: Promise<void> = job(() => clearBody(storage, store), { skipIfMoved: true }).then(() => undefined, () => undefined).finally(() => { if (clearing === p) clearing = null; });
  clearing = p;
}

/** The gate's one question, asked before every page answer that touches account data. OPEN and EMPTY pass; ADOPT writes the record and waits for that one write; CLEARING refuses at once
 * and starts a wipe if none runs; LOCKED refuses. */
export async function enter(storage: Storage | undefined, store: Store | undefined): Promise<{ readable: boolean; state: State; digest: string; world: World }> {
  let w0 = currentWorld();                                  // the world these facts are read for: if the boundary moves while they are read, the answer is not given for another world
  const f0 = await readFacts(storage);
  let state = stateOf(f0);
  if (state === 'adopt') {
    // One Adopt at a time, awaited by every answer that finds it (a single write). Another job running (a hung sign-out) is never waited for: the page is refused and asked again.
    if (!adopting && active === 0) { const a: Promise<State | undefined> = job(() => adoptBody(storage), { skipIfMoved: true }).finally(() => { if (adopting === a) adopting = null; }); adopting = a; }
    state = adopting ? (await adopting) ?? 'adopt' : 'adopt';
    if (state === 'open') w0 = currentWorld();               // the Adopt began the new account's world itself
  }
  if (state === 'clearing') startClear(storage, store);
  const moved = currentWorld() !== w0;
  return { readable: !moved && (state === 'open' || state === 'empty'), state: moved ? 'locked' : state, digest: f0.key.state === 'present' ? f0.key.digest : '', world: w0 };
}

export type Peek = { state: State; epoch: string; failed: string[]; key: KeyFact };
/** The card's question: the same facts, never waiting on a wipe. CLEARING starts a wipe if none runs; ADOPT retries its one write and answers with the result. The epoch moves when a
 * wipe completes or the boundary closes, so a page that refused is asked again (the contracts' acctTick). */
export async function peek(storage: Storage | undefined, store: Store | undefined): Promise<Peek> {
  const f = await readFacts(storage);
  let state = stateOf(f);
  // The card never waits on a running job (a hung sign-out or Clear would leave the banner blank): while one runs, ADOPT is reported as the wipe it is part of; with none, the one write is awaited.
  if (state === 'adopt') state = active > 0 ? 'clearing' : ((await job(() => adoptBody(storage), { skipIfMoved: true })) ?? 'adopt');
  if (state === 'clearing') startClear(storage, store);
  return { state, epoch: `${state}|${wipes}|${accountGeneration()}`, failed: lastFailed, key: f.key };
}

// ---- sign-out

export type SignOutResult = { done: boolean; failed: string; message: string; epoch: number };

/** The person's tap, never behind the gate: "delete the key, then settle". The result follows from the facts afterwards, not from the native answer. */
export async function accountSignOut(storage: Storage | undefined, store: Store | undefined): Promise<SignOutResult> {
  if (!nativeLater()) return { done: false, failed: '', message: 'Sign-out needs the app’s own module, which is not here. Nothing was changed.', epoch: wipes };
  // The tap is for the account on the phone NOW. A tap that waits behind another job must not delete the key of an account that arrived meanwhile: the key it sees at its turn must be the one it saw here.
  const tap = (await readFacts(storage)).key;
  const at = tap.state === 'present' ? tap.digest : tap.state === 'absent' ? '' : undefined;
  const r = (await job(() => signOutBody(storage, store, at))) ?? { done: false, failed: '', message: 'Nothing was signed out.' };
  return { ...r, epoch: wipes };
}

async function signOutBody(storage: Storage | undefined, store: Store | undefined, at: string | undefined): Promise<Omit<SignOutResult, 'epoch'>> {
  const now = (await readFacts(storage)).key;
  if (at !== undefined && now.state === 'present' && now.digest !== at) return { done: false, failed: '', message: 'The account on this phone changed while this waited. Nothing was signed out. Try again.' };
  await beginSignOut();                                                            // the generation moves and sealing stops BEFORE the native call
  await nativeCall('signOut', accountGeneration());                                // the key delete, then the native wipe: its answer is not trusted, the facts are read
  let f = await readFacts(storage);
  if (stateOf(f) === 'clearing') { await clearBody(storage, store); f = await readFacts(storage); }
  const s = stateOf(f);
  if (f.key.state === 'absent' && s === 'empty') return { done: true, failed: '', message: '' };
  if (f.key.state === 'absent' && s === 'clearing') {
    return { done: true, failed: lastFailed.join(','), message: `Signed out, but these could not be cleared yet: ${lastFailed.join(', ')}. They will be cleared again by itself.` };
  }
  abortSignOut();                                                                  // still signed in (or not known): sealing is no longer blocked
  return { done: false, failed: '', message: f.key.state === 'present' ? 'Sign-out did not finish. Try again.' : 'Sign-out could not be checked just now. Try again.' };
}
