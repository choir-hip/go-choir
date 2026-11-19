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
import type { Storage, Store } from './app.contract.d.ts';
import * as podcast from './podcast.ts';
import { wipeMail } from './mail.ts';
import { wipeFiles } from './files.ts';
import { wipePaper, CACHE_FILE as PAPER_FRONT } from './paper.ts';
import { wipeKept, KEPT_FILE } from './takes.ts';
import { wipeNotebook, CACHE_FILE as NOTEBOOK_CACHE } from './notebook.ts';
import { VECTORS_DIR, wipeLiveVectors } from './search.ts';
import { CONTENT_DB, wipeLiveContent } from './content.ts';
import { nextGeneration, resetSession, sessionDigest } from './api.ts';

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
  { name: 'mail', run: (s, k) => wipeMail(s, k) },
  { name: 'files', run: (s) => wipeFiles(s) },
  { name: 'paper-front', run: (s) => wipePaper(s) },
  { name: 'takes', run: (s) => wipeKept(s) },
  { name: 'notebook', run: (s) => wipeNotebook(s) },
  { name: 'vectors', run: async (s) => { await wipeLiveVectors(s); await rmAll(s, [VECTORS_DIR]); } },       // the live in-memory store first, then the files
  { name: 'content', run: async (s) => { await wipeLiveContent(); await rmAll(s, [CONTENT_DB, `${CONTENT_DB}-wal`, `${CONTENT_DB}-shm`]); } },  // stops a running sync (generation), empties the open db
  { name: 'account-stamp', run: (s) => rmAll(s, [STAMP_FILE]) },
];

/** (A function, not a constant: api.ts imports app.ts, which imports this, so a constant read at load can meet a module not yet initialised.)
 * The paths the wipes above cover (account.test.ts S4 compares them with every `app:/` literal in the shared modules). `app:/tmp/picked` is files'. */
export const wipedPaths = (): string[] => [
  podcast.LIBRARY_FILE, podcast.FEED_DIR, 'app:/data/mail', 'app:/data/recent-files.json', 'app:/cache/files', 'app:/tmp/picked', PAPER_FRONT, KEPT_FILE, NOTEBOOK_CACHE,
  VECTORS_DIR, CONTENT_DB, PENDING_FILE, STAMP_FILE,
];

/** Run each wipe; the names of the ones that threw. Logs ids, never content. The signed-in seam is reset first: a new account generation (a reply
 * still in flight is dropped as signed out) and no remembered computer. */
export async function wipeAll(storage: Storage | undefined, store: Store | undefined, wipes: Wipe[] = WIPES): Promise<string[]> {
  nextGeneration(); resetSession();
  const failed: string[] = [];
  for (const w of wipes) {
    try { await w.run(storage, store); } catch { failed.push(w.name); console.log(`account: wipe ${w.name} failed`); }
  }
  return failed;
}

export type SignOutNative = { available?: boolean; later(request: { op: 'signOut' }): Promise<unknown> };
export type SignOutResult = { done: boolean; failed: string; message: string };
/** A deadline, not a limit: a native sign-out that never answers leaves the page retryable. */
export const NATIVE_DEADLINE_MS = 30000;

const timeout = (ms: number): Promise<'timeout'> => new Promise((r) => setTimeout(() => r('timeout'), ms));
const mark = async (storage: Storage | undefined, path: string, text: string): Promise<boolean> => {
  if (!storage) return true;
  try { await storage.fs.atomicWriteFile(path, new TextEncoder().encode(text)); return true; } catch { return false; }
};

/** Sign-out. A native build refuses (done: false, nothing wiped) when its native module is absent: wiping the shared stores while the key stays
 * would report a sign-out that did not happen. The web build has no key to delete and no sign-in; it has nothing to sign out of. */
export async function accountSignOut(
  _a: { demo: boolean }, storage: Storage | undefined, store: Store | undefined, native: SignOutNative | null | undefined,
  opts: { wipes?: Wipe[]; deadlineMs?: number } = {},
): Promise<SignOutResult> {
  if (!native || native.available === false) return { done: false, failed: '', message: 'Sign-out needs the app’s own module, which is not here. Nothing was changed.' };
  // The marker first: if the app dies after the key is gone and before the wipes are finished, the next launch finishes them.
  if (!(await mark(storage, PENDING_FILE, '1'))) return { done: false, failed: '', message: 'Sign-out did not start. Try again.' };
  let ok = false;
  try {
    const r = await Promise.race([native.later({ op: 'signOut' }), timeout(opts.deadlineMs ?? NATIVE_DEADLINE_MS)]);
    ok = r !== 'timeout' && !!r && (r as { done?: unknown }).done === true;
  } catch { ok = false; }
  if (!ok) {
    // Not signed out (the key may still be there): nothing is wiped, so the marker goes, and a retry starts clean.
    try { await storage?.fs.rm(PENDING_FILE); } catch { /* absent */ }
    return { done: false, failed: '', message: 'Sign-out did not finish. Try again.' };
  }
  const failed = await wipeAll(storage, store, opts.wipes ?? WIPES);
  if (failed.length === 0) { try { await storage?.fs.rm(PENDING_FILE); } catch { /* absent */ } }
  return { done: true, failed: failed.join(','), message: '' };
}

let settled: Promise<void> | null = null;
/** The first answer after a launch waits for this. (1) A sign-out that was cut short (the marker is there): finish the wipes, then the answer
 * is a signed-out one. (2) The stored key belongs to another account than the one whose data the stores hold (the stamp): wipe, then keep the
 * new account's stamp. A phone with no key does nothing here: a locked Keychain looks the same as no key and must not cost anyone their data;
 * the stamp catches the next account instead. (A marker or stamp that cannot be read is treated as absent; no stamp means "adopt".) */
export function settleAccount(storage: Storage | undefined, store: Store | undefined): Promise<void> {
  if (!settled) settled = (async () => {
    if (!storage) return;
    let pending = false;
    try { await storage.fs.stat(PENDING_FILE); pending = true; } catch { /* none */ }
    if (pending) {
      const failed = await wipeAll(storage, store);
      if (failed.length === 0) { try { await storage.fs.rm(PENDING_FILE); } catch { /* absent */ } }
      console.log(`account: finished an interrupted sign-out, ${failed.length} failed`);
      return;
    }
    const now = await sessionDigest();
    if (!now) return;
    let stamp = '';
    try { stamp = new TextDecoder().decode(await storage.fs.readFile(STAMP_FILE)); } catch { /* none: adopt */ }
    if (stamp !== '' && stamp !== now) { await wipeAll(storage, store); console.log('account: another account signed in; the old one’s data was wiped'); }
    if (stamp !== now) await mark(storage, STAMP_FILE, now);
  })();
  return settled;
}
/** For tests. */
export function resetSettled(): void { settled = null; }
