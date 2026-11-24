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
import { accountGeneration, signOutBlocked, refreshSignOutBlock, keyOwner } from './api.ts';
import type { AttachmentMeta, Summary } from './mail-model.ts';

// THE INVARIANT (written after two reviews found the owner check missing from `holds` and cached across a key change): every file of the mail copy is reached through ONE accessor, `Files`, and a
// `Files` exists only for the account signed in RIGHT NOW (the key's digest is read fresh at `open`, never cached). Its paths carry that digest (`msg-<id>.<owner>.json`), so a file written under
// another key is not a path this accessor can name: `read`, `holds`, `count`, `trim` and every write see only their own account's files, and `open` removes the others once. There is no other way
// to read or write a mail file in this module: the path builders and the raw read and write are private to the class, and the exported functions below only open one and call it.

export const DIR = 'app:/data/mail';
export const CACHE = 'app:/cache';
/** The most message files kept; past it the oldest go (the Swift store capped mail at 200 MB, a year at 100 messages a day). */
export const MAX_FILES = 6000;

/** `gen`: the account generation (api.ts) the source started under. A write checks it again after the file is written: a sign-out that began in between (the generation moved) leaves no file. */
export type Env = { storage?: Storage; gen?: number };
const stale = (env: Env): boolean => env.gen !== undefined && env.gen !== accountGeneration();
export type FolderCopy = { folder: string; rows: Summary[]; total: number; unread: number; savedAt: number };
export type MessageCopy = { id: string; folder: string; summary: Summary; text: string; to: string[]; cc: string[]; replyTo: string; attachments: AttachmentMeta[]; savedAt: number };
export type StateCopy = { done: Record<string, boolean>; syncedAt: Record<string, number>; cursor: Record<string, string> };

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

/** The most bytes the message files may take (the Swift store was held to 200 MB for mail). */
export const MAX_BYTES = 200_000_000;
/** The text kept of a message is cut at this (the page shows at most MAX_CHARS, mail-model.ts). */
const KEEP_CHARS = 200_000;
let purgedFor = '';

export class Files {
  private constructor(private env: Env, private fs: NonNullable<Storage>['fs'], readonly owner: string) {}

  /** The copy of the account signed in now, or null: no storage, a sign-out under way, no key. Files of any other key are removed (once per account). */
  static async open(env: Env): Promise<Files | null> {
    if (!env.storage || !(await stores(env))) return null;
    const owner = await keyOwner();
    if (owner === null || stale(env)) return null;
    const f = new Files(env, env.storage.fs, owner);
    if (purgedFor !== owner) { await f.purgeForeign(); if (!stale(env)) purgedFor = owner; }
    return f;
  }

  private tail = (): string => `.${this.owner}.json`;
  private async msgPath(id: string): Promise<string> { return `${DIR}/msg-${hex((await sha256(new TextEncoder().encode(id))).subarray(0, 16))}${this.tail()}`; }
  private rowsPath(folder: string): string { return `${DIR}/rows-${folder.replace(/[^a-z0-9_-]/g, '_')}${this.tail()}`; }
  private statePath(): string { return `${DIR}/state${this.tail()}`; }
  /** For tests and the one caller that shows a path: the path a record has for this account. */
  pathOfMessage(id: string): Promise<string> { return this.msgPath(id); }
  pathOfRows(folder: string): string { return this.rowsPath(folder); }
  pathOfState(): string { return this.statePath(); }

  private async put(path: string, value: unknown): Promise<boolean> {
    if (stale(this.env)) return false;
    try {
      await this.fs.mkdir(DIR).catch(() => undefined);
      await this.fs.atomicWriteFile(path, enc(value));
      if (stale(this.env)) { try { await this.fs.rm(path); await this.fs.rm(DIR); } catch { /* the wipe will; the folder goes only when empty */ } return false; }
      return true;
    } catch { return false; }
  }
  private async get<T>(path: string): Promise<T | null> {
    let bytes: Uint8Array;
    try { bytes = new Uint8Array(await this.fs.readFile(path)); } catch { return null; }
    try { return JSON.parse(new TextDecoder().decode(bytes)) as T; }
    catch { try { await this.fs.rm(path); } catch { /* already gone */ } return null; }      // half written: the server's copy replaces it
  }

  saveFolder(c: FolderCopy): Promise<boolean> { return this.put(this.rowsPath(c.folder), c); }
  async loadFolder(folder: string): Promise<FolderCopy | null> { const c = await this.get<FolderCopy>(this.rowsPath(folder)); return c && c.folder === folder ? c : null; }
  async saveMessage(c: MessageCopy): Promise<boolean> { return this.put(await this.msgPath(c.id), c.text.length > KEEP_CHARS ? { ...c, text: c.text.slice(0, KEEP_CHARS) } : c); }
  async loadMessage(id: string): Promise<MessageCopy | null> { const c = await this.get<MessageCopy>(await this.msgPath(id)); return c && c.id === id ? c : null; }
  saveState(s: StateCopy): Promise<boolean> { return this.put(this.statePath(), s); }
  async loadState(): Promise<StateCopy> {
    const s = await this.get<StateCopy>(this.statePath());
    return s && typeof s === 'object' && s.done && s.syncedAt && s.cursor ? s : { done: {}, syncedAt: {}, cursor: {} };
  }
  /** Is a message's body held (so a sync does not fetch it again)? The name carries the owner, so this is this account's file or nothing. */
  async holds(id: string): Promise<boolean> { try { await this.fs.stat(await this.msgPath(id)); return true; } catch { return false; } }
  /** How many message bodies this account's copy holds. */
  async count(): Promise<number> { return (await this.names()).filter((n) => n.startsWith('msg-')).length; }
  private async names(): Promise<string[]> { try { return (await this.fs.readdir(DIR)).filter((n) => n.endsWith(this.tail())); } catch { return []; } }

  /** Past `max` files or `maxBytes`, the oldest message files go. Returns how many. */
  async trim(max = MAX_FILES, maxBytes = MAX_BYTES): Promise<number> {
    const names = (await this.names()).filter((n) => n.startsWith('msg-'));
    const aged: { n: string; t: number; size: number }[] = [];
    let total = 0;
    for (const n of names) { try { const st = await this.fs.stat(`${DIR}/${n}`); aged.push({ n, t: st.modifiedMs, size: st.size ?? 0 }); total += st.size ?? 0; } catch { /* gone */ } }
    if (aged.length <= max && total <= maxBytes) return 0;
    aged.sort((a, b) => a.t - b.t);
    let gone = 0, left = aged.length;
    for (const a of aged) {
      if (left <= max && total <= maxBytes) break;
      try { await this.fs.rm(`${DIR}/${a.n}`); gone += 1; left -= 1; total -= a.size; } catch { /* kept */ }
    }
    return gone;
  }
  /** Every file of the folder that is not this account's (another key's, or an earlier format without an owner in its name) is removed. */
  async purgeForeign(): Promise<void> {
    let names: string[] = [];
    try { names = await this.fs.readdir(DIR); } catch { return; }
    for (const n of names) if (!n.endsWith(this.tail())) { try { await this.fs.rm(`${DIR}/${n}`); } catch { /* kept */ } }
  }
}

// The functions the pages and the tests call: each opens the accessor for the account signed in now (null: nothing is kept or served) and calls it.
export const saveFolder = async (env: Env, c: FolderCopy): Promise<boolean> => (await Files.open(env))?.saveFolder(c) ?? false;
export const loadFolder = async (env: Env, folder: string): Promise<FolderCopy | null> => (await Files.open(env))?.loadFolder(folder) ?? null;
export const saveMessage = async (env: Env, c: MessageCopy): Promise<boolean> => (await Files.open(env))?.saveMessage(c) ?? false;
export const loadMessage = async (env: Env, id: string): Promise<MessageCopy | null> => (await Files.open(env))?.loadMessage(id) ?? null;
export const saveState = async (env: Env, s: StateCopy): Promise<boolean> => (await Files.open(env))?.saveState(s) ?? false;
export const loadState = async (env: Env): Promise<StateCopy> => (await Files.open(env))?.loadState() ?? { done: {}, syncedAt: {}, cursor: {} };
export const holds = async (env: Env, id: string): Promise<boolean> => (await Files.open(env))?.holds(id) ?? false;
export const trim = async (env: Env, max = MAX_FILES, maxBytes = MAX_BYTES): Promise<number> => (await Files.open(env))?.trim(max, maxBytes) ?? 0;

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

/** Remove every file of the copy (every account's) and every "Show original" document. Counts only. */
export async function wipeFiles(env: Env): Promise<{ files: number; originals: number }> {
  let files = 0;
  purgedFor = '';
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
