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
import { VECTORS_DIR } from './search.ts';
import { CONTENT_DB } from './content.ts';

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
  { name: 'vectors', run: (s) => rmAll(s, [VECTORS_DIR]) },
  { name: 'content', run: (s) => rmAll(s, [CONTENT_DB, `${CONTENT_DB}-wal`, `${CONTENT_DB}-shm`]) },
];

/** The paths the wipes above cover (account.test.ts S4 compares them with the constants the modules declare). Mail's `app:/cache` is handled by wipeMail. */
export const WIPED_PATHS: string[] = [
  podcast.LIBRARY_FILE, podcast.FEED_DIR, 'app:/data/mail', 'app:/data/recent-files.json', 'app:/cache/files', PAPER_FRONT, KEPT_FILE, NOTEBOOK_CACHE, VECTORS_DIR, CONTENT_DB,
];

/** Run each wipe; the names of the ones that threw. Logs ids, never content. */
export async function wipeAll(storage: Storage | undefined, store: Store | undefined, wipes: Wipe[] = WIPES): Promise<string[]> {
  const failed: string[] = [];
  for (const w of wipes) {
    try { await w.run(storage, store); } catch { failed.push(w.name); console.log(`account: wipe ${w.name} failed`); }
  }
  return failed;
}

export type SignOutNative = { available?: boolean; later(request: { op: 'signOut' }): Promise<unknown> };
export type SignOutResult = { done: boolean; failed: string; message: string };

export async function accountSignOut(
  _a: { demo: boolean }, storage: Storage | undefined, store: Store | undefined, native: SignOutNative | null | undefined, opts: { wipes?: Wipe[] } = {},
): Promise<SignOutResult> {
  if (native && native.available !== false) {
    let ok = false;
    try { const r = (await native.later({ op: 'signOut' })) as { done?: unknown } | null; ok = !!r && r.done === true; } catch { ok = false; }
    if (!ok) return { done: false, failed: '', message: 'Sign-out did not finish. Try again.' };
  }
  const failed = await wipeAll(storage, store, opts.wipes ?? WIPES);
  return { done: true, failed: failed.join(','), message: '' };
}
