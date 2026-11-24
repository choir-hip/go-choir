// The phone's copy of the listener's mail. The listener's computer (the Mail agent) is the custodian; this is a cache of it, so it is never migrated
// and never trusted over the server: a list or a message the server just sent replaces what is held. It keeps what the Swift ContentStore kept for
// mail (rows with their preview, the text of a message and who it went to; not the HTML and not attachments) and what a mailbox needs to open with
// no network. The files are plain JSON on the phone's disk, written by shared code (owner, 2026-11-23: "get rid of mail seal. i do want mail stored on
// phone's disk"): there is no native cipher and no key here; the platform's own file protection is the only seal. The old sealed copy is not read.
//
// Where it lives: app:/data/mail/ (kept by the app, not a cache the system may drop), files `rows-<folder>.json`, `msg-<hash of the id>.json`,
// `state.json`. A leftover file of an earlier build (`*.json`) is removed by the wipe and never read. The native wipe (AutoradioModule.swift, "app-files") removes the
// whole `content` folder at sign-out. Old `app:/cache/mail-*.html` originals are removed by `forgetOldCopy`.
//
// Ways this can fail (tests in mail.test.ts):
//  1. No storage (a drive that keeps none): nothing on disk, everything else works.   2. A file that does not parse (a half write, another shape): it is ignored and
//  removed, the server's answer replaces it.   3. A full disk or a missing grant: the write is lost, the page is unaffected.   4. Too many messages: the oldest files
//  go first (MAX_FILES).   5. Sign-out: `wipeMail` removes every file.   6. Logs: counts only, never an id's content.

import type { Storage } from './app.contract.d.ts';
import { accountGeneration, signOutBlocked, refreshSignOutBlock, keyState } from './api.ts';
import type { AttachmentMeta, Summary } from './mail-model.ts';

export const DIR = 'app:/data/mail';
export const CACHE = 'app:/cache';
/** The most message files kept; past it the oldest go (the Swift store capped mail at 200 MB, a year at 100 messages a day). */
export const MAX_FILES = 6000;

/** `gen`: the account generation (api.ts) the source started under. A write checks it at the last moment before it seals and again before and after the file
 * is written: a sign-out that began in between (the generation moved) writes nothing, makes no key and leaves no file. */
export type Env = { storage?: Storage; gen?: number };
const stale = (env: Env): boolean => env.gen !== undefined && env.gen !== accountGeneration();
/** `owner` is a digest of the sign-in key the file was written under (never the key): another account's file is not read, and is removed. */
export type FolderCopy = { owner?: string; folder: string; rows: Summary[]; total: number; unread: number; savedAt: number };
export type MessageCopy = { owner?: string; id: string; folder: string; summary: Summary; text: string; to: string[]; cc: string[]; replyTo: string; attachments: AttachmentMeta[]; savedAt: number };
export type StateCopy = { owner?: string; done: Record<string, boolean>; syncedAt: Record<string, number>; cursor: Record<string, string> };

export async function sha256(data: Uint8Array): Promise<Uint8Array> {
  return new Uint8Array(await crypto.subtle.digest('SHA-256', data as unknown as BufferSource));
}
export function hex(b: Uint8Array): string { return Array.from(b, (x) => x.toString(16).padStart(2, '0')).join(''); }

const enc = (v: unknown): Uint8Array => new TextEncoder().encode(JSON.stringify(v));

/** Can the phone keep mail now? Not while a sign-out is under way (api.ts) until another account signs in, and not with no storage. */
export const stores = async (env: Env): Promise<boolean> => {
  if (!env.storage || stale(env)) return false;
  await refreshSignOutBlock();
  return !signOutBlocked();
};

export async function fileFor(id: string): Promise<string> { return `${DIR}/msg-${hex((await sha256(new TextEncoder().encode(id))).subarray(0, 16))}.json`; }
export const rowsFile = (folder: string): string => `${DIR}/rows-${folder.replace(/[^a-z0-9_-]/g, '_')}.json`;
export const STATE_FILE = `${DIR}/state.json`;

/** The digest of the key the phone is signed in with, learned once per account generation; null when no one is signed in (nothing is written or read then). */
let ownerCache: { gen: number; digest: string } | null = null;
async function ownerOf(env: Env): Promise<string | null> {
  const gen = accountGeneration();
  if (ownerCache && ownerCache.gen === gen && ownerCache.digest !== '') return ownerCache.digest;
  const k = await keyState();
  if (gen !== accountGeneration() || stale(env)) return null;
  if (k.state !== 'present' || k.digest === '') return null;
  ownerCache = { gen, digest: k.digest };
  return k.digest;
}

async function write(env: Env, path: string, value: unknown): Promise<boolean> {
  // Two checks, each with a test that fails without it: before anything is written (no work for a stale source) and after the file is written (a sign-out that began
  // while it was being written removes the file again).
  if (!env.storage || !(await stores(env))) return false;
  const owner = await ownerOf(env);
  if (owner === null || stale(env)) return false;
  try {
    await env.storage.fs.mkdir(DIR).catch(() => undefined);
    await env.storage.fs.atomicWriteFile(path, enc({ ...(value as object), owner }));
    if (stale(env)) { try { await env.storage.fs.rm(path); await env.storage.fs.rm(DIR); } catch { /* the wipe will; the folder goes only when empty */ } return false; }
    return true;
  } catch { return false; }
}

async function read<T>(env: Env, path: string): Promise<T | null> {
  if (!env.storage || !(await stores(env))) return null;
  let bytes: Uint8Array;
  try { bytes = new Uint8Array(await env.storage.fs.readFile(path)); } catch { return null; }
  const drop = async (): Promise<null> => { try { await env.storage!.fs.rm(path); } catch { /* already gone */ } return null; };      // not ours or half written: the server's copy replaces it
  let v: T & { owner?: string };
  try { v = JSON.parse(new TextDecoder().decode(bytes)) as T & { owner?: string }; } catch { return drop(); }
  const owner = await ownerOf(env);
  if (owner === null) return null;                                       // no one is signed in: nothing is served, nothing is judged
  if (v === null || typeof v !== 'object' || v.owner !== owner) return drop();      // another account's file (or an earlier format)
  return v;
}

export const saveFolder = (env: Env, c: FolderCopy): Promise<boolean> => write(env, rowsFile(c.folder), c);
export async function loadFolder(env: Env, folder: string): Promise<FolderCopy | null> { const c = await read<FolderCopy>(env, rowsFile(folder)); return c && c.folder === folder ? c : null; }
/** The text kept of a message is cut at this (the page shows at most MAX_CHARS, mail-model.ts). */
const KEEP_CHARS = 200_000;
export async function saveMessage(env: Env, c: MessageCopy): Promise<boolean> { return write(env, await fileFor(c.id), c.text.length > KEEP_CHARS ? { ...c, text: c.text.slice(0, KEEP_CHARS) } : c); }
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

/** The most bytes the message files may take (the Swift store was held to 200 MB for mail). */
export const MAX_BYTES = 200_000_000;
/** Past MAX_FILES or MAX_BYTES, the oldest message files go. Returns how many. */
export async function trim(env: Env, max = MAX_FILES, maxBytes = MAX_BYTES): Promise<number> {
  const fs = env.storage?.fs;
  if (!fs) return 0;
  let names: string[];
  try { names = (await fs.readdir(DIR)).filter((n) => n.startsWith('msg-') && n.endsWith('.json')); } catch { return 0; }
  const aged: { n: string; t: number; size: number }[] = [];
  let total = 0;
  for (const n of names) { try { const st = await fs.stat(`${DIR}/${n}`); aged.push({ n, t: st.modifiedMs, size: st.size ?? 0 }); total += st.size ?? 0; } catch { /* gone */ } }
  if (aged.length <= max && total <= maxBytes) return 0;
  aged.sort((a, b) => a.t - b.t);
  let gone = 0, left = aged.length;
  for (const a of aged) {
    if (left <= max && total <= maxBytes) break;
    try { await fs.rm(`${DIR}/${a.n}`); gone += 1; left -= 1; total -= a.size; } catch { /* kept */ }
  }
  return gone;
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

/** Remove every file of the copy and every "Show original" document. Counts only. */
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
  return { files, originals };
}
