// The phone's copy of the listener's mail. The listener's computer (the Mail agent) is the custodian; this is a cache of it, so it is never migrated
// and never trusted over the server: a list or a message the server just sent replaces what is held. It keeps what the Swift ContentStore kept for
// mail (rows with their preview, the text of a message and who it went to; not the HTML and not attachments) and what a mailbox needs to open with
// no network. Every file is sealed (mail-seal.ts) with a key in the secret store; with no key nothing is written, and the mailbox works from memory.
//
// Where it lives: app:/data/mail/ (kept by the app, not a cache the system may drop), files `rows-<folder>.bin`, `msg-<hash of the id>.bin`,
// `state.bin`. The Swift store's own folder (Application Support/Autoradio/content/items/mail) is not reachable from here; ContentStore.swift deletes
// it at open (the one-time loss the cutover file's D3 accepts: mail is fetched again). Leftovers this module can reach, the old
// `app:/cache/mail-*.html` originals, are removed by `forgetOldCopy`.
//
// Ways this can fail (tests in mail.test.ts):
//  1. No key (a drive that keeps no secrets, a secret store that refuses): nothing on disk, everything else works.   2. A file that does not open (another
//  key, a changed byte, a half write): it is ignored and removed, the server's answer replaces it.   3. A full disk or a missing grant: the write is lost, the
//  page is unaffected.   4. Too many messages: the oldest files go first (MAX_FILES).   5. Sign-out: `wipeMail` removes every file and forgets the key.
//  6. Logs: counts only, never an id's content.

import type { Storage, Store } from './app.contract.d.ts';
import { open, seal, newKey, validKey, sha256, hex } from './mail-seal.ts';
import type { AttachmentMeta, Summary } from './mail-model.ts';

export const DIR = 'app:/data/mail';
export const CACHE = 'app:/cache';
/** The secret's name; the grant in app.ts is `secret.keep mail-key`. */
export const KEY_NAME = 'mail-key';
/** The most message files kept; past it the oldest go (the Swift store capped mail at 200 MB, a year at 100 messages a day). */
export const MAX_FILES = 6000;

export type Env = { storage?: Storage; store?: Store };
export type FolderCopy = { folder: string; rows: Summary[]; total: number; unread: number; savedAt: number };
export type MessageCopy = { id: string; folder: string; summary: Summary; text: string; to: string[]; cc: string[]; replyTo: string; attachments: AttachmentMeta[]; savedAt: number };
export type StateCopy = { done: Record<string, boolean>; syncedAt: Record<string, number>; cursor: Record<string, string> };

const enc = (v: unknown): Uint8Array => new TextEncoder().encode(JSON.stringify(v));

/** The key, made on first use. Null when the secret store keeps nothing (then nothing is written to disk). */
export function keyOf(env: Env): string | null {
  const store = env.store;
  if (!store) return null;
  try {
    const kept = store.get(KEY_NAME);
    if (validKey(kept)) return kept;
    const k = newKey();
    store.set(KEY_NAME, k);
    return store.get(KEY_NAME) === k ? k : null;      // a drive that does not keep secrets answers null: no key, no copy
  } catch { return null; }
}

export async function fileFor(id: string): Promise<string> { return `${DIR}/msg-${hex((await sha256(new TextEncoder().encode(id))).subarray(0, 16))}.bin`; }
export const rowsFile = (folder: string): string => `${DIR}/rows-${folder.replace(/[^a-z0-9_-]/g, '_')}.bin`;
export const STATE_FILE = `${DIR}/state.bin`;

async function write(env: Env, path: string, value: unknown): Promise<boolean> {
  const k = keyOf(env);
  if (!k || !env.storage) return false;
  try {
    await env.storage.fs.mkdir(DIR).catch(() => undefined);
    await env.storage.fs.atomicWriteFile(path, await seal(enc(value), k));
    return true;
  } catch { return false; }
}

async function read<T>(env: Env, path: string): Promise<T | null> {
  const k = keyOf(env);
  if (!k || !env.storage) return null;
  let bytes: Uint8Array;
  try { bytes = new Uint8Array(await env.storage.fs.readFile(path)); } catch { return null; }
  const plain = await open(bytes, k);
  if (plain === null) { try { await env.storage.fs.rm(path); } catch { /* already gone */ } return null; }
  try { return JSON.parse(new TextDecoder().decode(plain)) as T; } catch { return null; }
}

export const saveFolder = (env: Env, c: FolderCopy): Promise<boolean> => write(env, rowsFile(c.folder), c);
export const loadFolder = (env: Env, folder: string): Promise<FolderCopy | null> => read<FolderCopy>(env, rowsFile(folder));
export async function saveMessage(env: Env, c: MessageCopy): Promise<boolean> { return write(env, await fileFor(c.id), c); }
export async function loadMessage(env: Env, id: string): Promise<MessageCopy | null> { const c = await read<MessageCopy>(env, await fileFor(id)); return c && c.id === id ? c : null; }
export const saveState = (env: Env, s: StateCopy): Promise<boolean> => write(env, STATE_FILE, s);
export async function loadState(env: Env): Promise<StateCopy> {
  const s = await read<StateCopy>(env, STATE_FILE);
  return s && typeof s === 'object' && s.done && s.syncedAt && s.cursor ? s : { done: {}, syncedAt: {}, cursor: {} };
}

/** Is a message's body held (so a sync does not fetch it again)? Reads the file's name only. */
export async function holds(env: Env, id: string): Promise<boolean> {
  if (!env.storage) return false;
  try { await env.storage.fs.stat(await fileFor(id)); return true; } catch { return false; }
}

/** Past MAX_FILES, the oldest message files go. Returns how many. */
export async function trim(env: Env, max = MAX_FILES): Promise<number> {
  const fs = env.storage?.fs;
  if (!fs) return 0;
  let names: string[];
  try { names = (await fs.readdir(DIR)).filter((n) => n.startsWith('msg-')); } catch { return 0; }
  if (names.length <= max) return 0;
  const aged: { n: string; t: number }[] = [];
  for (const n of names) { try { aged.push({ n, t: (await fs.stat(`${DIR}/${n}`)).modifiedMs }); } catch { /* gone */ } }
  aged.sort((a, b) => a.t - b.t);
  let gone = 0;
  for (const a of aged.slice(0, Math.max(0, aged.length - max))) { try { await fs.rm(`${DIR}/${a.n}`); gone += 1; } catch { /* kept */ } }
  return gone;
}

/** The held messages, newest first by the date they carry, read and opened one by one (a few hundred at most: the caller says how many). */
export async function heldMessages(env: Env, limit: number): Promise<MessageCopy[]> {
  const fs = env.storage?.fs;
  if (!fs) return [];
  let names: string[];
  try { names = (await fs.readdir(DIR)).filter((n) => n.startsWith('msg-')); } catch { return []; }
  const aged: { n: string; t: number }[] = [];
  for (const n of names) { try { aged.push({ n, t: (await fs.stat(`${DIR}/${n}`)).modifiedMs }); } catch { /* gone */ } }
  aged.sort((a, b) => b.t - a.t);
  const out: MessageCopy[] = [];
  for (const a of aged.slice(0, limit)) { const m = await read<MessageCopy>(env, `${DIR}/${a.n}`); if (m) out.push(m); }
  return out.sort((a, b) => (a.summary.at < b.summary.at ? 1 : a.summary.at > b.summary.at ? -1 : 0));
}

/** Search the phone's copy: the words must all appear (any case) in the sender, subject or text of a held message, or in a held row's preview.
 * Returns message ids, newest first. The global search (X10) will read this; no mail page asks it yet. */
export async function search(env: Env, query: string, limit = 20, scan = 500): Promise<string[]> {
  const words = query.toLowerCase().split(/\s+/).filter((w) => w !== '');
  if (words.length === 0) return [];
  const seen = new Set<string>();
  const hits: { id: string; at: string }[] = [];
  const test = (id: string, at: string, hay: string): void => {
    if (seen.has(id)) return;
    const h = hay.toLowerCase();
    if (words.every((w) => h.includes(w))) { seen.add(id); hits.push({ id, at }); }
  };
  for (const m of await heldMessages(env, scan)) test(m.id, m.summary.at, `${m.summary.fromDisplay} ${m.summary.fromAddress} ${m.summary.subject} ${m.text}`);
  for (const f of ['inbox', 'sent', 'quarantine']) {
    const c = await loadFolder(env, f);
    for (const r of c?.rows ?? []) test(r.id, r.at, `${r.fromDisplay} ${r.fromAddress} ${r.subject} ${r.snippet}`);
  }
  return hits.sort((a, b) => (a.at < b.at ? 1 : a.at > b.at ? -1 : 0)).slice(0, limit).map((h) => h.id);
}

/** What the old Swift version left where shared code can reach: the "Show original" documents in the cache. Returns how many were removed. */
export async function forgetOldCopy(storage: Storage | undefined, keep = ''): Promise<number> {
  const fs = storage?.fs;
  if (!fs) return 0;
  let names: string[];
  try { names = await fs.readdir(CACHE); } catch { return 0; }
  let gone = 0;
  for (const n of names) {
    if (!(n.startsWith('mail-') && n.endsWith('.html')) || n === keep) continue;
    try { await fs.rm(`${CACHE}/${n}`); gone += 1; } catch { /* kept */ }
  }
  return gone;
}

/** Remove every file of the copy, every "Show original" document, and the key. Counts only. */
export async function wipeFiles(env: Env): Promise<{ files: number; originals: number }> {
  let files = 0;
  const fs = env.storage?.fs;
  if (fs) {
    let names: string[] = [];
    try { names = await fs.readdir(DIR); } catch { /* no folder */ }
    for (const n of names) { try { await fs.rm(`${DIR}/${n}`); files += 1; } catch { /* kept */ } }
    try { await fs.rm(DIR); } catch { /* the folder was never made, or is not empty */ }
  }
  const originals = await forgetOldCopy(env.storage);
  try { env.store?.forget(KEY_NAME); } catch { /* no secret store */ }
  return { files, originals };
}
