// Sign-out, orchestrated from shared code. The Account screen's "Sign out" sends `accountSignOut` (app.ts), which:
//   1. asks the native side to sign out: ONE native call, `{op: 'signOut'}` on the app module (modules/apple/ModelsEntry.swift). Only native
//      code can meet it: the sign-in key is a Keychain item the Swift sign-in wrote, and the Swift stores (AccountWipe) still hold the voice's
//      and the not-yet-moved pages' data. The native side stops playback, lets the Swift run sheet post what it holds with the OLD key,
//      deletes the key, runs its own wipe, and answers `{done: true}` when it has finished.
//   2. only then wipes every store the shared modules keep (WIPES below), each on its own: one that throws is named and does not stop the rest.
// Why this order and this direction: Exact has no call from native code into `app.ts` (the only ways back are `context.changed`, which asks a
// source again, and a declared event on a Contract node: docs/reference/exact/native-modules.md, "Out (Swift to the UI)"). Sign-out is
// started by a Contract action, so shared code can lead and call native once; no hook is needed, and no shared store can be missed by a
// native list. The drift guard in account.test.ts (S4) fails when a shared module keeps a path no wipe here covers.
// Sign-in stays Swift; the key is never read or moved here.
//
// ONE FUNCTION decides what a launch and the end of every sign-out do about the account's data (`reconcile`, below): three facts in, one action out.
//   key state (native, THREE answers: present with a digest / absent = the Keychain said item-not-found / unreadable = locked or anything else),
//   stamp (the SHA-256 of the key of the account whose data the shared stores hold: none / same / other), marker (`signout-pending`: none / pending).
// Rules (proved by the table in account.test.ts): an unreadable key never wipes and never adopts. A key present with a pending marker means the native half never
// deleted the key: the marker goes, nothing is wiped. A key absent with a stamp or a marker means a sign-out is unfinished: finish it (the native sign-out, which is
// idempotent and reruns the Swift wipe, THEN the shared wipe, THEN the stamp and the marker go). A key present whose digest is not the stamp's is another account: wipe
// both halves, THEN write the new stamp. A stamp or marker is advanced or removed ONLY when every wipe succeeded; otherwise it stays, the next launch tries again, and
// the page is told which stores failed. A native call that outlives its deadline keeps its continuation, and the marker stays until it answers or the next launch reconciles.
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
import { nextGeneration, resetSession, beginSignOut, abortSignOut, keyState, nativeLater, type KeyState } from './api.ts';
import { forgetOldCopy } from './mail-store.ts';
import { wipePicks, parseStored, PICKS_FILE } from './edition-picks.ts';
import { wipeOnboarding, PROGRESS_FILE as ONBOARDING_FILE } from './onboarding.ts';

/** Written (a one-word file) BEFORE the native sign-out and removed only after every wipe succeeded. A kill in between leaves it, and the next
 * launch (`settleAccount`) finishes the wipe before any source answers. (Review 2026-11-19, finding 2.) */
export const PENDING_FILE = 'app:/data/signout-pending';
/** The SHA-256 of the key of the account whose data the shared stores hold. A different account seen at launch wipes them first. */
export const STAMP_FILE = 'app:/data/account-stamp';

export type Wipe = { name: string; run(storage: Storage | undefined, store: Store | undefined): Promise<unknown> };

const rmAll = async (storage: Storage | undefined, paths: string[]): Promise<void> => {
  if (!storage) return;
  for (const p of paths) { try { await storage.fs.rm(p); } catch { /* absent */ } }
};

/** Every store the shared modules keep. A module that moves in adds one line here (and its path to WIPED_PATHS). */
export const WIPES: Wipe[] = [
  { name: 'podcasts', run: (s) => podcast.wipe(s) },
  { name: 'mail', run: (s) => wipeMail(s) },
  { name: 'files', run: (s) => wipeFiles(s) },
  { name: 'paper-front', run: (s) => wipePaper(s) },
  { name: 'takes', run: (s) => wipeKept(s) },
  { name: 'notebook', run: (s) => wipeNotebook(s) },
  { name: 'edition-picks', run: (s) => wipePicks(s) },                                                            // the listener's topics (edition-picks.ts): memory, the file, and the native copy
  { name: 'onboarding', run: (s) => wipeOnboarding(s) },                                                           // the first run's progress (onboarding.ts): memory, the file, and the voice side
  { name: 'vectors', run: async (s) => { await wipeLiveVectors(s); await rmAll(s, [VECTORS_DIR]); } },       // the live in-memory store first, then the files
  { name: 'content', run: async (s) => { await wipeLiveContent(); await rmAll(s, [CONTENT_DB, `${CONTENT_DB}-wal`, `${CONTENT_DB}-shm`]); } },  // stops a running sync (generation), empties the open db
  { name: 'pages', run: (s) => wipePages(s) },                                                                  // the deck's open set (pages.ts): only the radio stays
  { name: 'clips', run: async () => forgetClips() },                                                              // in memory only (public clips, but another account's page asked for them)
  { name: 'demo-leftovers', run: (s) => rmAll(s, [DEMO_MARKER]) },                                                  // a marker, not data: it only lets the one-time pass run again
  { name: 'account-stamp', run: (s) => rmAll(s, [STAMP_FILE]) },
];

/** (A function, not a constant: api.ts imports app.ts, which imports this, so a constant read at load can meet a module not yet initialised.)
 * The paths the wipes above cover (account.test.ts S4 compares them with every `app:/` literal in the shared modules). `app:/tmp/picked` is files'. */
export const wipedPaths = (): string[] => [
  podcast.LIBRARY_FILE, podcast.FEED_DIR, 'app:/data/mail', 'app:/data/recent-files.json', 'app:/cache/files', 'app:/tmp/picked', PAPER_FRONT, KEPT_FILE, NOTEBOOK_CACHE, PICKS_FILE, ONBOARDING_FILE,
  VECTORS_DIR, CONTENT_DB, PAGES_FILE, DEMO_MARKER, PENDING_FILE, STAMP_FILE,
];

/** Run each wipe; the names of the ones that threw. Logs ids, never content. The signed-in seam is reset first: a new account generation (a reply
 * still in flight is dropped as signed out) and no remembered computer. */
/** Counts every completed wipe, the late continuation's too: the shared sources that keep state in memory are asked again when it changes (signedOutTick). */
let wipes = 0;
export const wipeEpoch = (): number => wipes;
export async function wipeAll(storage: Storage | undefined, store: Store | undefined, wipes_: Wipe[] = WIPES): Promise<string[]> {
  nextGeneration(); resetSession(); wipes += 1;
  const failed: string[] = [];
  for (const w of wipes_) {
    try { await w.run(storage, store); } catch { failed.push(w.name); console.log(`account: wipe ${w.name} failed`); }
  }
  return failed;
}

export type SignOutNative = { available?: boolean; later(request: { op: 'signOut' | 'wipe' }): Promise<unknown> };
export type SignOutResult = { done: boolean; failed: string; message: string; epoch: number };
/** A deadline, not a limit: a native sign-out that never answers leaves the page retryable. */
export const NATIVE_DEADLINE_MS = 30000;

const timeout = (ms: number): Promise<'timeout'> => new Promise((r) => setTimeout(() => r('timeout'), ms));
/** A token for one sign-out, written into its marker: a late answer from another sign-out is told apart by it. */
const signoutToken = (): string => Array.from(crypto.getRandomValues(new Uint8Array(16)), (b) => b.toString(16).padStart(2, '0')).join('');      // 16 random bytes, hex (getRandomValues is the call already used on the device, runsheet-entries.ts)
/** Fails closed: no storage, or a write that fails, is not a marker. */
const mark = async (storage: Storage | undefined, path: string, text: string): Promise<boolean> => {
  if (!storage) return false;
  try { await storage.fs.atomicWriteFile(path, new TextEncoder().encode(text)); return true; } catch { return false; }
};
const readText = async (storage: Storage, path: string): Promise<string | null> => { try { return new TextDecoder().decode(await storage.fs.readFile(path)); } catch { return null; } };
const exists = async (storage: Storage, path: string): Promise<boolean> => { try { await storage.fs.stat(path); return true; } catch { return false; } };
const removeFile = async (storage: Storage | undefined, path: string): Promise<boolean> => {
  if (!storage) return true;
  try { await storage.fs.rm(path); } catch { /* absent */ }
  return !(await exists(storage, path));
};

/** The stores of the wipe list WITHOUT the stamp: the stamp is advanced or removed only after every one of these succeeded. */
export const DATA_WIPES: Wipe[] = WIPES.filter((w) => w.name !== 'account-stamp');

// ---- the decision (pure)

export type StampFact = 'none' | 'same' | 'other';
export type MarkerFact = 'none' | 'pending';
export type Facts = { key: KeyState; stamp: StampFact; marker: MarkerFact };
export type Action = 'nothing' | 'adopt' | 'clear-marker' | 'finish' | 'account-change';

export function decide(f: Facts): Action {
  if (f.key === 'unreadable') return 'nothing';                       // never wipes, never adopts
  if (f.key === 'present') {
    if (f.marker === 'pending') return 'clear-marker';                // the native half did not delete the key: nothing was signed out
    return f.stamp === 'none' ? 'adopt' : f.stamp === 'same' ? 'nothing' : 'account-change';
  }
  return f.marker === 'pending' || f.stamp !== 'none' ? 'finish' : 'nothing';   // absent
}

// ---- the native half, under a deadline

export type Native = { signOut(onLate?: () => void): Promise<boolean>; wipe(onLate?: () => void): Promise<boolean> };
/** The app module's two ops as booleans. A call that outlives `ms` answers false NOW, but keeps its continuation: `onLate` runs if it answers done:true afterwards. */
export function nativeOf(later: ((r: { op: string }) => Promise<unknown>) | null, ms = NATIVE_DEADLINE_MS): Native {
  const call = (op: 'signOut' | 'wipe', onLate?: () => void): Promise<boolean> => {
    if (!later) return Promise.resolve(false);
    let timedOut = false;
    const p: Promise<boolean> = later({ op }).then((r) => !!r && (r as { done?: unknown }).done === true, () => false);
    void p.then((ok) => { if (timedOut && ok) onLate?.(); });
    return Promise.race([p, timeout(ms).then(() => { timedOut = true; return false; })]);
  };
  return { signOut: (l) => call('signOut', l), wipe: (l) => call('wipe', l) };
}

export type Reconciled = { action: Action; failed: string[] };

async function stampFact(storage: Storage, digest: string): Promise<StampFact> {
  let stamp = '';
  try { stamp = new TextDecoder().decode(await storage.fs.readFile(STAMP_FILE)); } catch { /* none */ }
  return stamp === '' ? 'none' : stamp === digest ? 'same' : 'other';
}

/** Most wipes swallow "absent", so a wipe that did nothing looks like one that worked. After the wipes, every path they cover is looked at: a survivor is a failure. */
export async function leftovers(storage: Storage | undefined): Promise<string[]> {
  if (!storage) return [];
  const out: string[] = [];
  for (const p of wipedPaths()) {
    if (p === STAMP_FILE || p === PENDING_FILE || !(await exists(storage, p))) continue;
    if (p === PICKS_FILE && parseStored((await readText(storage, p)) ?? '').length === 0) continue;       // rm failed and the file was emptied (wipePicks): cleared, not left over
    out.push(p.slice(p.lastIndexOf('/') + 1) || p);
  }
  return out;
}

/** Wipe every shared store; then, ONLY if every one succeeded and nothing is left, run `after`. Returns the names that failed. */
async function sharedWipeThen(storage: Storage | undefined, store: Store | undefined, after: () => Promise<void>): Promise<string[]> {
  const failed = [...(await wipeAll(storage, store, DATA_WIPES)), ...(await leftovers(storage))];
  if (failed.length === 0) await after();
  return failed;
}

/** The one function: read the three facts, decide, act. Run at launch (`settleAccount`) and at the end of every sign-out attempt. */
export async function reconcile(storage: Storage | undefined, store: Store | undefined, native: Native): Promise<Reconciled> {
  if (!storage) return { action: 'nothing', failed: [] };
  for (let round = 0; round < 2; round++) {
    const k = await keyState();
    const marker: MarkerFact = (await exists(storage, PENDING_FILE)) ? 'pending' : 'none';
    const stamp: StampFact = k.state === 'present' ? await stampFact(storage, k.digest) : (await exists(storage, STAMP_FILE)) ? 'other' : 'none';
    const action = decide({ key: k.state, stamp, marker });
    if (action === 'clear-marker') { await removeFile(storage, PENDING_FILE); continue; }     // then the stamp rules apply to the same key
    if (action === 'adopt') { await mark(storage, STAMP_FILE, k.digest); return { action, failed: [] }; }
    if (action === 'finish') {
      // BOUND to this sign-out: its marker's content (a token written by accountSignOut) and its stamp are captured now; the bookkeeping clears them only if they are still those.
      // A later sign-out (sign in, sign out again) writes its own marker and stamp, and a stale answer must not delete them.
      const was = { marker: await readText(storage, PENDING_FILE), stamp: await readText(storage, STAMP_FILE) };
      const done = async (): Promise<void> => {
        if ((await readText(storage, PENDING_FILE)) !== was.marker || (await readText(storage, STAMP_FILE)) !== was.stamp) return;
        await removeFile(storage, STAMP_FILE); await removeFile(storage, PENDING_FILE);
      };
      // A late native answer never wipes the shared stores again (they are wiped below, once): it only finishes the bookkeeping that waited on it, and only if the key is still absent.
      let shared: Promise<string[]> | null = null;
      const nativeOk = await native.signOut(() => { void (async () => { if ((await keyState()).state === 'absent' && shared && (await shared).length === 0) await done(); })(); });
      // The shared wipe does not DEPEND on the native answer: the key is gone, so the previous account's shared stores must not stay readable because the native half failed or hung.
      // (It starts after the native call has answered or passed its deadline, at most NATIVE_DEADLINE_MS later; only the stamp and marker wait for both results.)
      shared = sharedWipeThen(storage, store, nativeOk ? done : async () => undefined);
      const failed = [...(await shared)];       // a copy: the late continuation reads the wipe's own result
      if (!nativeOk) failed.push('native');
      console.log(`account: finished an interrupted sign-out, ${failed.length} failed`);
      return { action, failed };
    }
    if (action === 'account-change') {
      const stamp2 = async (): Promise<void> => { await mark(storage, STAMP_FILE, k.digest); };
      // The late answer finishes only the stamp (the new account IS signed in: re-wiping would delete its fresh stores), and only if the same key is still stored and the wipe below was clean.
      let shared: Promise<string[]> | null = null;
      const nativeOk = await native.wipe(() => { void (async () => { const n = await keyState(); if (n.state === 'present' && n.digest === k.digest && shared && (await shared).length === 0) await stamp2(); })(); });
      shared = sharedWipeThen(storage, store, nativeOk ? stamp2 : async () => undefined);       // shared stores go regardless; the old stamp stays unless both succeeded
      const failed = [...(await shared)];       // a copy: the late continuation reads the wipe's own result
      if (!nativeOk) failed.push('native');
      console.log(`account: another account signed in, ${failed.length} failed`);
      return { action, failed };
    }
    return { action, failed: [] };
  }
  return { action: 'nothing', failed: [] };
}

// ---- sign-out

/** The sign-out: the result carries the wipe counter (a monotonic value the contract passes on to the sources that keep state in memory). */
export async function accountSignOut(
  storage: Storage | undefined, store: Store | undefined, native: SignOutNative | null | undefined,
  opts: { wipes?: Wipe[]; deadlineMs?: number; crash?: (step: string) => void } = {},
): Promise<SignOutResult> {
  const r = await signOutInner(storage, store, native, opts);
  return { ...r, epoch: wipes };
}

async function signOutInner(
  storage: Storage | undefined, store: Store | undefined, native: SignOutNative | null | undefined,
  opts: { wipes?: Wipe[]; deadlineMs?: number; crash?: (step: string) => void } = {},
): Promise<Omit<SignOutResult, 'epoch'>> {
  if (!native || native.available === false) return { done: false, failed: '', message: 'Sign-out needs the app’s own module, which is not here. Nothing was changed.' };
  const step = opts.crash ?? ((): void => undefined);
  // The marker first: if the app dies after the key is gone and before the wipes are finished, the next launch finishes them (and if the key is still there, it clears it).
  const token = signoutToken();
  if (!(await mark(storage, PENDING_FILE, token))) return { done: false, failed: '', message: 'Sign-out did not start. Try again.' };
  step('marker');
  // The generation moves and sealing stops BEFORE the native call: a write in flight that would land after the key is gone refuses itself.
  await beginSignOut();
  step('begun');
  let late = false;
  let nativeDone: Promise<unknown>;
  try { nativeDone = native.later({ op: 'signOut' }); } catch { nativeDone = Promise.resolve({ done: false }); }
  const wipes = opts.wipes ?? DATA_WIPES;
  // A native call that outlives the deadline keeps its continuation: if it answers done:true later, the shared stores are wiped then and the marker goes.
  nativeDone.then(async (r) => {
    if (!late || !r || (r as { done?: unknown }).done !== true) return;
    if ((await keyState()).state === 'present') return;                          // a sign-in finished inside the window: its stores are its own
    await sharedWipeThen(storage, store, async () => { if (storage && (await readText(storage, PENDING_FILE)) !== token) return; await removeFile(storage, STAMP_FILE); await removeFile(storage, PENDING_FILE); });
  }, () => undefined);
  const r = await Promise.race([nativeDone.then((x) => x, () => ({ done: false, threw: true })), timeout(opts.deadlineMs ?? NATIVE_DEADLINE_MS)]);
  if (r === 'timeout') { late = true; return { done: false, failed: '', message: 'Sign-out is taking long. It will finish by itself; nothing more to do.' }; }   // the marker stays; sealing stays off
  const ans = r as { done?: unknown; threw?: boolean };
  if (ans.done !== true) {
    // Not signed out (the key may still be there). An explicit "not done" means the key remains: the marker goes. A throw is unknown: the marker stays and the next
    // reconcile decides by the key (present: the marker is cleared and nothing is wiped).
    abortSignOut();
    if (!ans.threw) await removeFile(storage, PENDING_FILE);
    return { done: false, failed: '', message: 'Sign-out did not finish. Try again.' };
  }
  step('native');
  const run = wipes.map((w) => ({ ...w, run: async (s: Storage | undefined, st: Store | undefined) => { step('wipe:' + w.name); return w.run(s, st); } }));
  const failed = [...(await wipeAll(storage, store, run)), ...(opts.wipes ? [] : await leftovers(storage))];
  step('wiped');
  if (failed.length === 0) { await removeFile(storage, STAMP_FILE); await removeFile(storage, PENDING_FILE); return { done: true, failed: '', message: '' }; }
  // Signed out, but not everything could be cleared: the marker and the stamp stay, the next launch tries again, and the page says which stores.
  return { done: true, failed: failed.join(','), message: `Signed out, but these could not be cleared yet: ${failed.join(', ')}. They will be cleared the next time the app starts.` };
}

let settled: Promise<Reconciled> | null = null;
/** The first answer after a launch waits for this: it is `reconcile` with the app module's native ops. */
export function settleAccount(storage: Storage | undefined, store: Store | undefined, native: Native = nativeOf(nativeLater())): Promise<Reconciled> {
  if (!settled) settled = (async () => { await forgetOldCopy(storage); return reconcile(storage, store, native); })();
  return settled;
}
/** For tests. */
export function resetSettled(): void { settled = null; }
