// The phone's copy of the listener's mail. The listener's computer (the Mail agent) is the custodian; this is a cache of it, so it is never migrated
// and never trusted over the server: a list or a message the server just sent replaces what is held. It keeps what the Swift ContentStore kept for
// mail (rows with their preview, the text of a message and who it went to; not the HTML and not attachments) and what a mailbox needs to open with
// no network. Every file is sealed (mail-seal.ts, the platform's AES-GCM through one native pair; the key is native and never seen here); with no cipher nothing is written, and the mailbox works from memory.
//
// Where it lives: app:/data/mail/ (kept by the app, not a cache the system may drop), files `rows-<folder>.bin`, `msg-<hash of the id>.bin`,
// `state.bin`. The Swift store's own folder (Application Support/Autoradio/content/items/mail) is not reachable from here; ContentStore.swift deletes
// it at open (the one-time loss the cutover file's D3 accepts: mail is fetched again). Leftovers this module can reach, the old
// `app:/cache/mail-*.html` originals, are removed by `forgetOldCopy`.
//
// Ways this can fail (tests in mail.test.ts):
//  1. No key (a drive that keeps no secrets, a secret store that refuses): nothing on disk, everything else works.   2. A file that does not open (another
//  key, a changed byte, a half write): it is ignored and removed, the server's answer replaces it.   3. A full disk or a missing grant: the write is lost, the
//  page is unaffected.   4. Too many messages: the oldest files go first (MAX_FILES).   5. Sign-out: `wipeMail` removes every file and forgets the native key.
//  6. Logs: counts only, never an id's content.

import type { Storage } from './app.contract.d.ts';
import { open, seal, canSeal, forgetKey, sha256, hex } from './mail-seal.ts';
import { accountGeneration } from './api.ts';
import type { AttachmentMeta, Summary } from './mail-model.ts';

export const DIR = 'app:/data/mail';
export const CACHE = 'app:/cache';
/** The most message files kept; past it the oldest go (the Swift store capped mail at 200 MB, a year at 100 messages a day). */
export const MAX_FILES = 6000;

/** `gen`: the account generation (api.ts) the source started under. A write checks it at the last moment before it seals and again before and after the file
 * is written: a sign-out that began in between (the generation moved) writes nothing, makes no key and leaves no file. */
export type Env = { storage?: Storage; gen?: number };
const stale = (env: Env): boolean => env.gen !== undefined && env.gen !== accountGeneration();
export type FolderCopy = { folder: string; rows: Summary[]; total: number; unread: number; savedAt: number };
export type MessageCopy = { id: string; folder: string; summary: Summary; text: string; to: string[]; cc: string[]; replyTo: string; attachments: AttachmentMeta[]; savedAt: number };
export type StateCopy = { done: Record<string, boolean>; syncedAt: Record<string, number>; cursor: Record<string, string> };

const enc = (v: unknown): Uint8Array => new TextEncoder().encode(JSON.stringify(v));

/** Is there a cipher (the native pair, mail-seal.ts)? Without one nothing is written to disk. */
export const sealed = (env: Env): Promise<boolean> => (stale(env) ? Promise.resolve(false) : canSeal());

export async function fileFor(id: string): Promise<string> { return `${DIR}/msg-${hex((await sha256(new TextEncoder().encode(id))).subarray(0, 16))}.bin`; }
export const rowsFile = (folder: string): string => `${DIR}/rows-${folder.replace(/[^a-z0-9_-]/g, '_')}.bin`;
export const STATE_FILE = `${DIR}/state.bin`;

async function write(env: Env, path: string, value: unknown, aad: string): Promise<boolean> {
  // Two checks, each with a test that fails without it: before anything is sealed (no key work for a stale source) and after the file is written (a sign-out that began
  // while it was being sealed or written removes the file again). The checks between them were redundant and are gone.
  if (!env.storage || stale(env) || !(await canSeal())) return false;
  try {
    const box = await seal(enc(value), aad);
    if (!box) return false;                                // the cipher refused (no key could be made): nothing is written
    await env.storage.fs.mkdir(DIR).catch(() => undefined);
    await env.storage.fs.atomicWriteFile(path, box);
    if (stale(env)) { try { await env.storage.fs.rm(path); } catch { /* the wipe will */ } return false; }
    return true;
  } catch { return false; }
}

async function read<T>(env: Env, path: string, aad: string): Promise<T | null> {
  if (!env.storage || !(await canSeal())) return null;
  let bytes: Uint8Array;
  try { bytes = new Uint8Array(await env.storage.fs.readFile(path)); } catch { return null; }
  const o = await open(bytes, aad);
  if ('why' in o) {
    // Removed only when the cipher said the data is bad (changed, another record's, an earlier format): a locked Keychain or a bridge error keeps the file.
    if (o.why === 'bad') { try { await env.storage.fs.rm(path); } catch { /* already gone */ } }
    return null;
  }
  try { return JSON.parse(new TextDecoder().decode(o.plain)) as T; } catch { return null; }
}

const aadRows = (folder: string): string => `rows|${folder}`;
const aadMsg = (id: string): string => `msg|${id}`;
export const saveFolder = (env: Env, c: FolderCopy): Promise<boolean> => write(env, rowsFile(c.folder), c, aadRows(c.folder));
export async function loadFolder(env: Env, folder: string): Promise<FolderCopy | null> { const c = await read<FolderCopy>(env, rowsFile(folder), aadRows(folder)); return c && c.folder === folder ? c : null; }
/** The text kept of a message is cut at this (the page shows at most MAX_CHARS, mail-model.ts). */
const KEEP_CHARS = 200_000;
export async function saveMessage(env: Env, c: MessageCopy): Promise<boolean> { return write(env, await fileFor(c.id), c.text.length > KEEP_CHARS ? { ...c, text: c.text.slice(0, KEEP_CHARS) } : c, aadMsg(c.id)); }
export async function loadMessage(env: Env, id: string): Promise<MessageCopy | null> { const c = await read<MessageCopy>(env, await fileFor(id), aadMsg(id)); return c && c.id === id ? c : null; }
export const saveState = (env: Env, s: StateCopy): Promise<boolean> => write(env, STATE_FILE, s, 'state');
export async function loadState(env: Env): Promise<StateCopy> {
  const s = await read<StateCopy>(env, STATE_FILE, 'state');
  return s && typeof s === 'object' && s.done && s.syncedAt && s.cursor ? s : { done: {}, syncedAt: {}, cursor: {} };
}

/** Can the files that are here be opened with the key that is here? After a restore to another phone (the files came back, the key did not) every body would
 * be "held" forever by `holds` and never fetched again. One file is opened: 'ok' (or nothing there), 'purged' (it was bad: every message file and the state are
 * removed so the server's copy replaces them), 'cannot' (no verdict now: a locked Keychain; nothing is removed and the caller must not sync). */
export async function validate(env: Env): Promise<'ok' | 'purged' | 'cannot'> {
  const fs = env.storage?.fs;
  if (!fs || !(await canSeal())) return 'cannot';
  let names: string[] = [];
  try { names = await fs.readdir(DIR); } catch { return 'ok'; }
  const probe = names.includes('state.bin') ? 'state.bin' : names.find((n) => n.startsWith('rows-') || n.startsWith('msg-'));
  if (!probe) return 'ok';
  let bytes: Uint8Array;
  try { bytes = new Uint8Array(await fs.readFile(`${DIR}/${probe}`)); } catch { return 'ok'; }
  // The aad of the probe: a state file is 'state'; a rows file names its folder; for a message file only the key is tested (a wrong aad is also 'bad', so use the state/rows).
  const aad = probe === 'state.bin' ? 'state' : probe.startsWith('rows-') ? `rows|${probe.slice(5, -4)}` : '';
  if (aad === '') return 'ok';
  const o = await open(bytes, aad);
  if (!('why' in o)) return 'ok';
  if (o.why === 'cannot') return 'cannot';
  for (const n of names) { try { await fs.rm(`${DIR}/${n}`); } catch { /* kept */ } }
  return 'purged';
}

/** Is a message's body held (so a sync does not fetch it again)? Reads the file's name only: `validate` has said the key opens what is here. */
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
  try { names = (await fs.readdir(DIR)).filter((n) => n.startsWith('msg-')); } catch { return 0; }
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

// (There is no search over the phone's copy: the copy is sealed, and a search index of mail would hold its words unsealed. Mail stays out of local search
// until a search over sealed mail is designed; owner-visible regression, recorded in docs/design/autoradio-exact-integration-2026-11-19.md.)

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

/** The "Show original" copies still in the cache (names), or the cache folder itself when it cannot be read for a reason other than being absent: what a wipe is checked against (account.ts). */
export async function originalsLeft(storage: Storage | undefined): Promise<string[]> {
  const fs = storage?.fs;
  if (!fs) return [];
  let names: string[];
  try { names = await fs.readdir(CACHE); } catch (e) { return (e as { code?: unknown } | null)?.code === 'ENOENT' ? [] : [CACHE]; }
  return names.filter((n) => n.startsWith('mail-') && n.endsWith('.html')).map(() => 'mail copies');      // counted, never named: a message id is the account's
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
  await forgetKey();                                      // the native key goes too: whatever stays on a disk is then unreadable
  return { files, originals };
}
