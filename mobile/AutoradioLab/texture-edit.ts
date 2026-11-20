// Editing your own Texture documents on the phone, in shared code (it was Swift: TextureEditor.swift, with TextureBody.swift now texture-body.ts). The editor is Exact's `textarea markup="markdown"`
// over the document's Markdown (reader.contract TextureEditArea); this file is everything behind it: the session for one document, the save, and the offline copy.
//
// The rules:
//  - Who can edit what: `texture:<doc>` (the head of your own document, including one opened from Files) is editable. A revision (`texture:<doc>@<rev>`) and a published story (`pub:`) are read-only.
//  - An edit becomes a revision through the web's own call, POST /api/texture/documents/{id}/revisions (frontend texture.js createRevision): `content` (the Markdown) and `body_doc` (texture-body.ts
//    `merge`), `parent_revision_id`, and for a document with a trajectory the lifecycle version and an idempotency key. No second write path.
//  - When it is posted: 2 seconds after the last keystroke, at once on Done, and when the phone goes to the background. The WAIT is the contract's (members-content.contract: a `task after(2000)`
//    keyed by the draft, so each keystroke restarts it; a source cannot sleep between answers); `textureFlush` is what it calls. Never two saves at once; a burst of typing is one revision.
//  - The newest text is kept on the phone from the first keystroke (`app:/data/texture-pending.json`) and removed when it is posted, so a kill, a bad network or a missed background flush loses
//    nothing. Each kept text records the revision it descends from (`base`). It is posted when the route answers: the contract asks again every 30 s while anything is waiting, on ANY page
//    (`textureFlush('')`), and the first answer after a launch retries what an earlier launch kept, whatever page is in front.
//  - The invariants of the owner's real documents (review round 2; each is tested in texture-edit.test.ts with texts the field can really produce):
//    I1 No keystroke after a merge removes another writer's change. The field is never told a merge, so EVERY save is a three-way merge: base = the field's text at the last successful save (`ref`),
//       ours = the field now, theirs = the head as last read (a 409 re-reads it). A field that lags the head can never revert it.
//    I2 A kept text with no recorded base (typed when the head could not be read; every entry of the old build's file) is never posted over a head it is not equal to: it becomes a kept conflict. Equal to
//       the head: cleared. Opening a document reads its head (`textureEditInfo`), so typing offline afterwards has a base.
//    I3 A kept conflict is readable and copyable (`info.kept`) after the editor closes and after a relaunch, is never overwritten (a second one is refused with a sentence; the new text stays in the field
//       and in the ordinary kept set, and cannot post until the conflict is copied or discarded), and goes only by `textureDiscard`.
//    I4 Sign-out can say how many unsent edits it removes (`info.unsent`).
//  - Sign-out or an account change: the sessions and the pending file go (account.ts WIPES `texture-edit`); a save in flight finds the account generation changed and keeps nothing. The kept
//    file names the account (the SHA-256 of the sign-in key, api.ts keyState): another account's file is not loaded and is removed; after the account generation moves, nothing of it shows or posts.
//  - A head that moved: the server refuses a stale parent with 409. The editor re-reads the head and merges by lines (texture-body.ts `merge3`); the same lines changed both ways post nothing, keep the text
//    as a conflict, and say so (status `conflict`, no retry). The app's own topic sync (Notebook.swift, still Swift) posts behind this session; the first save after it takes this road.
//
// What stays native, and why. Three calls on the app module, each with its reason:
//  1. `{op: 'textureSaved'}` after a save: the Swift reader forgets its copy and the Swift content store keeps the new text ("kept and searchable at once"). Both stores are Swift until the reader and the content store move.
//  2. `{op: 'textureOldPending'}` then `{op: 'textureOldPendingRemove'}` at the first answer after a launch: the build before this one kept an unsent edit at Application Support/Autoradio/texture-pending.json,
//     a platform location outside every Exact grant, which only native code can read. It is read once (the JSON `{"<doc>": "<markdown>"}`), merged into the kept set here, written durably in the new place
//     (app:/data/texture-pending.json), and only then removed from the old place; the normal save sends it. A crash between the two steps leaves both copies: the next launch merges again, the new place wins
//     and the text, once sent, equals the head, so it is not sent twice. Nothing is read where there is no native module.
//  3. `{op: 'textureResigned'}` when something is waiting: a counter the native side moves on willResignActive (the phone locks or the app is left). `visibilityState` is not established on an iPhone for
//     lock and unlock (docs/reference/exact/ui-model.md), so the flush rests on the platform's own notification too (AutoradioModule+TextureEdit.swift touches this source when it fires).
//
// Ways this can fail (each has a test in texture-edit.test.ts; the numbered lists are at its head):
//  1. A keystroke per revision: the burst is one post.  2. An edit lost on a bad network, an upgrade or a crash, or posted twice after a relaunch.  3. A document nobody may edit (a revision, a story) opens a session.
//  4. A save that changes nothing is posted.  5. Typing while a save is in flight is dropped: it saves right after.  6. Another account's text shows or posts after a switch.
//  7. A newer version of the document is silently overwritten, or the session base is stale for ever.  8. A long document exhausts the heap or the turn, or leaves `saving` set.
import type { Storage } from './app.contract.d.ts';
import { authedJSON, accountGeneration, keyState, segment, writeCurrent, SignedOutError, HTTPError, type Reply } from './api.ts';
import { WORDS } from './words.ts';
import * as Body from './texture-body.ts';

export const PENDING_FILE = 'app:/data/texture-pending.json';
export type Info = { status: string; label: string; address: string; editable: boolean; kept: string; unsent: number; waiting: number };
export type Net = { json(path: string, opts?: { method?: 'GET' | 'POST'; body?: unknown }): Promise<Reply> };
export type Deps = { net?: Net; storage?: Storage; native?: (request: Record<string, unknown>) => Promise<unknown> };
type Head = { content: string; body: Body.Node | null; revisionId: string; version: number };
/** The field's lineage: `text` is what the field held at the last successful save (or the head's text when it opened), `id` the revision that lineage descends from. `text` is null until read (a kept text
 * written over an older revision names only the id). */
type Ref = { id: string; text: string | null };
type Session = { base: Head; ref: Ref; unbased: boolean; draft: string | null; status: string; label: string; saving: boolean; gen: number; key: { text: string; id: string } | null };
/** What is kept for a document: the newest text, and the revision it descends from ('' when that is not known). */
type Kept = { text: string; base: string };

const real: Net = { json: (path, opts) => authedJSON(path, opts) };
const obj = (v: unknown): v is Body.Node => typeof v === 'object' && v !== null && !Array.isArray(v);
const str = (v: unknown): string => (typeof v === 'string' ? v : '');
/** The words when the same lines were changed here and elsewhere: plain, and they say the text is safe. */
export const CONFLICT_WORDS = 'Not saved: this document changed somewhere else, and your changes could not be combined with it. Your text is kept on this phone.';
/** A conflict is already kept for this document: nothing new is posted until it is copied or discarded. */
export const BLOCKED_WORDS = 'Not saved: text kept on this phone from an earlier conflict is waiting for this document. Copy or discard it first.';
/** The draft could not be turned into a body (a damaged `body_doc`): a bug or bad data, never "offline". */
class MergeError extends Error { constructor(e: unknown) { super(e instanceof Error ? e.message : 'merge failed'); this.name = 'MergeError'; } }

const sessions = new Map<string, Session>();
let pending = new Map<string, Kept>();
/** Kept text that could not be combined with a newer head (or had no base): shown, copyable, removed only by `textureDiscard` or sign-out. One per document. */
let conflicts = new Map<string, Kept>();
let loading: Promise<void> | null = null;
let retried = false;
/** The documents whose text an EARLIER launch kept (read from the file): only these are posted by the first answer; text typed in this session is its own session's to post. */
let earlier = new Set<string>();
let epoch = 0;
/** The account generation the kept set belongs to (null: not yet seen). When it moves (a sign-out, an account change) the in-memory set is dropped and the file is not re-read in this process. */
let scopeGen: number | null = null;
/** How many times the native side has said the app went to the background, as last seen here. */
let resignSeen = 0;
let digestMemo: { gen: number; digest: string } | null = null;

function clear(): void { epoch += 1; sessions.clear(); pending = new Map(); conflicts = new Map(); earlier = new Set(); loading = null; retried = false; scopeGen = null; resignSeen = 0; digestMemo = null; }
/** Sign-out, an account change, tests: nothing of the old account stays. */
export async function wipeTextureEdit(storage: Storage | undefined): Promise<void> {
  clear();
  if (storage) { try { await storage.fs.rm(PENDING_FILE); } catch { /* absent */ } }
}
export function resetTextureEdit(): void { clear(); }

/** The document id of an address the listener may edit (the head of a Texture document), else null. */
export function editableDoc(address: string): string | null {
  if (!address.startsWith('texture:')) return null;
  const rest = address.slice('texture:'.length);
  const at = rest.indexOf('@');
  const doc = at < 0 ? rest : rest.slice(0, at);
  const rev = at < 0 ? '' : rest.slice(at + 1);
  return doc !== '' && rev === '' ? doc : null;
}

// MARK: the kept text

/** The kept set is the account's: when the generation has moved since it was loaded, it is dropped (the sign-out wipe removes the file; another account's file is refused by its digest at the next load). */
function scope(): void {
  const g = accountGeneration();
  if (scopeGen === null) { scopeGen = g; return; }
  if (scopeGen !== g) { scopeGen = g; pending = new Map(); conflicts = new Map(); earlier = new Set(); loading = Promise.resolve(); retried = true; }
}
/** The SHA-256 of the sign-in key ('' when it cannot be read: locked, no native module). Remembered while the account generation stands. */
async function ownerDigest(): Promise<string> {
  const g = accountGeneration();
  if (digestMemo && digestMemo.gen === g) return digestMemo.digest;
  const k = await keyState();
  const digest = k.state === 'present' ? k.digest : '';
  if (digest !== '') digestMemo = { gen: g, digest };
  return digest;
}

function loadPending(d: Deps): Promise<void> {
  scope();
  if (!loading) loading = doLoad(d);
  return loading;
}
const readKept = (v: unknown): Map<string, Kept> => {
  const m = new Map<string, Kept>();
  if (obj(v)) for (const [doc, k] of Object.entries(v)) if (obj(k) && typeof k.text === 'string') m.set(doc, { text: k.text, base: str(k.base) });
  return m;
};
async function doLoad(d: Deps): Promise<void> {
  const mine = epoch, gen = accountGeneration();
  if (!d.storage) return;
  try {
    const o = JSON.parse(new TextDecoder().decode(await d.storage.fs.readFile(PENDING_FILE)));
    if (obj(o) && o.v === 2) {
      const k = await keyState();
      const foreign = k.state === 'absent' || (k.state === 'present' && str(o.account) !== '' && str(o.account) !== k.digest);       // signed out, or another account's text
      if (mine !== epoch || gen !== accountGeneration()) return;
      if (foreign) { try { await d.storage.fs.rm(PENDING_FILE); } catch { /* absent */ } }
      else {
        for (const [doc, v] of readKept(o.docs)) if (!pending.has(doc)) { pending.set(doc, v); earlier.add(doc); }
        for (const [doc, v] of readKept(o.conflicts)) if (!conflicts.has(doc)) conflicts.set(doc, v);
      }
    }
  } catch { /* none kept, or damaged: none */ }
  await migrateOldFile(d, mine, gen);
}
/** The build before this one kept an unsent edit outside app:/ (see the header, native call 2). Read, merged, written durably in the new place, THEN removed from the old: a crash in between loses
 * nothing and sends nothing twice. That build recorded no base, so none of its text is ever posted over a head it is not equal to: the first retry makes it a kept conflict (I2). */
async function migrateOldFile(d: Deps, mine: number, gen: number): Promise<void> {
  if (!d.native || !d.storage) return;
  let r: { found?: unknown; text?: unknown } | null;
  try { r = (await d.native({ op: 'textureOldPending' })) as { found?: unknown; text?: unknown } | null; } catch { return; }
  if (!r || r.found !== true) return;
  const remove = async (): Promise<void> => { try { await d.native!({ op: 'textureOldPendingRemove' }); } catch { /* the next launch merges again: the new place wins */ } };
  let old: unknown = null;
  try { old = JSON.parse(str(r.text)); } catch { /* damaged */ }
  if (!obj(old)) { await remove(); return; }                                       // nothing in it can be kept; do not read it again at every launch
  if (mine !== epoch || gen !== accountGeneration()) return;
  for (const [doc, text] of Object.entries(old)) if (typeof text === 'string' && !pending.has(doc) && !conflicts.has(doc)) { pending.set(doc, { text, base: '' }); earlier.add(doc); }
  if (!(await savePending(d))) return;                                             // not durable in the new place: the old file stays
  if (mine !== epoch || gen !== accountGeneration()) return;
  await remove();
}
/** True when the file is as the kept sets say (or there is nowhere to write it). */
async function savePending(d: Deps): Promise<boolean> {
  if (!d.storage) return true;
  const gen = accountGeneration();
  if (pending.size === 0 && conflicts.size === 0) { try { await d.storage.fs.rm(PENDING_FILE); } catch { /* absent */ } return true; }
  const account = await ownerDigest();
  const bytes = new TextEncoder().encode(JSON.stringify({ v: 2, account, docs: Object.fromEntries(pending), conflicts: Object.fromEntries(conflicts) }));
  return writeCurrent(gen, d.storage, PENDING_FILE, bytes).catch(() => false);
}
const keep = (doc: string, text: string, base: string): void => { pending.set(doc, { text, base }); };
/** Move a text that cannot be posted into the conflict shelf. False when the shelf already holds one for this document (it is never overwritten). */
function shelve(doc: string, k: Kept): boolean {
  if (conflicts.has(doc)) return false;
  conflicts.set(doc, k);
  return true;
}
const counts = (): { unsent: number; waiting: number } => {
  const docs = new Set([...pending.keys(), ...conflicts.keys()]);
  return { unsent: docs.size, waiting: [...pending.keys()].filter((k) => !conflicts.has(k)).length };
};
const idle = (address = '', editable = false): Info => ({ status: 'idle', label: '', address, editable, kept: '', ...counts() });

// MARK: the server's routes (as the web uses them)

async function fetchRevision(net: Net, rid: string): Promise<Head> {
  let rev = (await net.json(`/api/texture/revisions/${segment(rid)}`)).body;
  if (obj(rev) && obj(rev.revision)) rev = rev.revision;
  const r = obj(rev) ? rev : {};
  const n = typeof r.version_number === 'number' ? Math.trunc(r.version_number) : -1;
  return { content: str(r.content), body: obj(r.body_doc) ? r.body_doc : null, revisionId: rid, version: n + 1 };
}
async function fetchHead(net: Net, doc: string): Promise<Head> {
  let d = (await net.json(`/api/texture/documents/${segment(doc)}`)).body;
  if (obj(d) && obj(d.document)) d = d.document;
  const rid = obj(d) ? str(d.current_revision_id) : '';
  if (rid === '') return { content: '', body: null, revisionId: '', version: 0 };
  return fetchRevision(net, rid);
}

async function postRevision(net: Net, doc: string, content: string, body: Body.Node, parent: string, key: string): Promise<{ revisionId: string; version: number }> {
  const payload: Body.Node = { content, body_doc: body };
  if (parent !== '') payload.parent_revision_id = parent;
  // As the web does: a document with a trajectory is saved against its lifecycle (the version it read) with an idempotency key (one key per text, so a post whose answer was lost is a replay, not a second revision).
  let d = (await net.json(`/api/texture/documents/${segment(doc)}`)).body;
  if (obj(d) && obj(d.document)) d = d.document;
  const traj = obj(d) ? str(d.trajectory_id) : '';
  if (traj !== '') {
    const life = (await net.json(`/api/texture/lifecycle/${segment(traj)}`)).body;
    const l = obj(life) ? life : {};
    payload.idempotency_key = key;
    const v = obj(l.trajectory) ? l.trajectory.lifecycle_version : undefined;
    if (v !== undefined && v !== null) payload.expected_lifecycle_version = v;
    const head = (obj(l.current_document_head) ? str(l.current_document_head.revision_id) : '') || (obj(l.document) ? str(l.document.current_revision_id) : '') || (obj(l.head_revision) ? str(l.head_revision.revision_id) : '');
    if (head !== '') payload.parent_revision_id = head;
  }
  const r = (await net.json(`/api/texture/documents/${segment(doc)}/revisions`, { method: 'POST', body: payload })).body;
  let o = obj(r) ? r : {};
  if (obj(o.revision)) o = o.revision;
  const n = typeof o.version_number === 'number' ? Math.trunc(o.version_number) : -1;
  return { revisionId: str(o.revision_id), version: n + 1 };
}
const uuid = (): string => {
  const b = crypto.getRandomValues(new Uint8Array(16));
  b[6] = (b[6] & 0x0f) | 0x40; b[8] = (b[8] & 0x3f) | 0x80;
  const h = Array.from(b, (x) => x.toString(16).padStart(2, '0')).join('');
  return `${h.slice(0, 8)}-${h.slice(8, 12)}-${h.slice(12, 16)}-${h.slice(16, 20)}-${h.slice(20)}`;
};

/** Offline: the route did not answer (transport, timeout) or answered "try later"; anything else is a refusal the listener should read. A draft that cannot be merged is a bug, not the network. */
export function isOffline(e: unknown): boolean {
  if (e instanceof SignedOutError || e instanceof MergeError) return false;
  if (e instanceof HTTPError) return e.status >= 500 || e.status === 404 || e.status === 429;
  return true;
}
/** One plain sentence for a failed save. */
export function saveSentence(e: unknown): string {
  if (e instanceof SignedOutError) return e.reason === 'locked' ? WORDS.locked : 'Sign in again to save.';
  if (e instanceof MergeError) return 'This document could not be prepared for saving. Your text is kept on this phone.';
  if (e instanceof HTTPError) return e.status === 401 ? 'Sign in again to save.' : e.status === 403 ? 'This device is not allowed to save here.' : `Could not save. ${e.message}`;
  return 'Could not reach your computer. Check the connection and try again.';
}
const isStale = (e: unknown): boolean => e instanceof HTTPError && e.status === 409;

// MARK: sessions

async function session(doc: string, d: Deps): Promise<Session | null> {
  const gen = accountGeneration();
  const have = sessions.get(doc);
  if (have && have.gen === gen) return have;
  sessions.delete(doc);
  try {
    const head = await fetchHead(d.net ?? real, doc);
    const again = sessions.get(doc);
    if (again && again.gen === gen) return again;
    if (gen !== accountGeneration()) return null;
    const s: Session = { base: head, ref: { id: head.revisionId, text: head.content }, unbased: false, draft: null, status: 'idle', label: '', saving: false, gen, key: null };
    sessions.set(doc, s);
    return s;
  } catch { return null; }
}
/** A kept text enters a session: written over the head's revision (nothing to do), over an older one (merged), or over none known (never posted: I2). */
function adopt(s: Session, k: Kept): void {
  s.draft = k.text;
  if (k.base === '') s.unbased = true;
  else if (k.base !== s.base.revisionId) s.ref = { id: k.base, text: null };
}
/** What the native side says about the app going to the background (see the header, native call 3). */
async function resignCount(d: Deps): Promise<number> {
  try { const r = (await d.native!({ op: 'textureResigned' })) as { count?: unknown } | null; return r && typeof r.count === 'number' ? r.count : 0; } catch { return 0; }
}

export async function textureEditInfo(address: string, d: Deps = {}, hidden = false): Promise<Info> {
  const doc = editableDoc(address);
  await loadPending(d);
  if (!retried) { retried = true; for (const k of [...earlier]) if (pending.has(k) && !sessions.has(k)) await retryOne(k, d); }          // what an earlier launch kept, from whatever page is in front
  let away = hidden;
  if (d.native && (sessions.size > 0 || pending.size > 0)) { const n = await resignCount(d); if (n > resignSeen) { resignSeen = n; away = true; } }
  if (away) await flushAll(d);                                                                                                       // the phone went to the background: what is waiting is posted now
  else for (const k of [...sessions.keys()]) if (k !== doc) await save(k, d);                                                        // the page in front is not that document's: leaving a page posts what was typed on it
  if (doc !== null && !sessions.has(doc)) await session(doc, d);                                                                     // the document is in front: its head is read now, so typing offline later has a base (I2)
  return doc === null ? idle() : info(doc, address);
}
function info(doc: string, address: string): Info {
  const s = sessions.get(doc);
  const kept = conflicts.get(doc)?.text ?? '';
  const c = counts();
  if (s && s.gen === accountGeneration() && !(kept !== '' && s.status === 'idle')) return { status: s.status, label: s.label, address, editable: true, kept, ...c };
  if (kept !== '') return { status: 'conflict', label: CONFLICT_WORDS, address, editable: true, kept, ...c };
  if (pending.has(doc)) return { status: 'offline', label: 'Edits kept on this phone', address, editable: true, kept, ...c };
  return idle(address, true);
}

/** The listener typed: the text is now `markdown`. Kept on the phone at once; posted when `textureFlush` is called (the contract's pause in typing). */
export async function textureEdit(address: string, markdown: string, d: Deps = {}): Promise<Info> {
  const doc = editableDoc(address);
  if (doc === null) return idle();
  await loadPending(d);
  // An earlier launch's kept text is not the field's (the field was reseeded from the head): the new typing never overwrites it. It goes to the conflict shelf; if the shelf is taken, the new text is refused.
  const old = pending.get(doc);
  if (old && earlier.has(doc) && old.text !== markdown && (sessions.get(doc)?.draft ?? null) === null) {
    if (!shelve(doc, old)) { const i = info(doc, address); return { ...i, status: 'conflict', label: BLOCKED_WORDS }; }
    pending.delete(doc); earlier.delete(doc);
  }
  const s = await session(doc, d);
  if (!s) { keep(doc, markdown, pending.get(doc)?.base ?? ''); await savePending(d); return info(doc, address); }       // no head yet (offline): kept, with no base unless it had one
  s.draft = markdown;
  const over = s.unbased ? '' : s.ref.id;
  if (markdown === s.base.content && !s.unbased) { s.status = 'idle'; s.label = ''; pending.delete(doc); s.ref = { id: s.base.revisionId, text: markdown }; }
  else { s.status = 'dirty'; s.label = 'Editing'; keep(doc, markdown, over); }
  await savePending(d);
  return info(doc, address);
}

/** Post now what is waiting (Done, the pause in typing). With no document (''), the global retry: every kept edit, whatever page is in front. */
export async function textureFlush(address: string, d: Deps = {}): Promise<Info> {
  const doc = editableDoc(address);
  await loadPending(d);
  if (doc === null) { if (address === '') await flushAll(d); return idle(); }
  if (sessions.has(doc)) await save(doc, d);
  else if (pending.has(doc)) await retryOne(doc, d);
  return info(doc, address);
}
export async function flushAll(d: Deps = {}): Promise<void> {
  scope();
  for (const doc of [...sessions.keys()]) await save(doc, d);
  await loadPending(d);
  for (const doc of [...pending.keys()]) if (!sessions.has(doc)) await retryOne(doc, d);
}
/** Throw away the conflict kept for this document (the owner asked; the contract asks once). The ordinary kept text, if any, is untouched. */
export async function textureDiscard(address: string, d: Deps = {}): Promise<Info> {
  const doc = editableDoc(address);
  if (doc === null) return idle();
  await loadPending(d);
  if (conflicts.delete(doc)) {
    await savePending(d);
    const s = sessions.get(doc);
    if (s && s.status === 'conflict') { s.status = s.draft !== null && s.draft !== s.base.content ? 'dirty' : 'idle'; s.label = s.status === 'dirty' ? 'Editing' : ''; }
  }
  return info(doc, address);
}

const headingOf = (text: string): string => text.split(/\r?\n|\r/).map((l) => l.replace(/^[#\s]+|\s+$/g, '')).find((l) => l !== '')?.slice(0, 120) ?? 'Document';

/** The text cannot be posted: it goes to the shelf and the owner is told. A shelf already taken keeps it as the ordinary kept text and says it is waiting. */
async function conflict(doc: string, s: Session, text: string, d: Deps): Promise<void> {
  if (shelve(doc, { text, base: s.unbased ? '' : s.ref.id })) { pending.delete(doc); s.status = 'conflict'; s.label = CONFLICT_WORDS; }
  else { keep(doc, text, s.unbased ? '' : s.ref.id); s.status = 'conflict'; s.label = BLOCKED_WORDS; }
  s.unbased = false;
  await savePending(d);
}

async function save(doc: string, d: Deps): Promise<void> {
  const s = sessions.get(doc);
  if (!s || s.saving || s.gen !== accountGeneration()) return;
  if (s.draft === null) { const k = pending.get(doc); if (k === undefined) return; adopt(s, k); }                            // kept text meets a session (a launch, or the head was unreadable when it was typed)
  if (s.draft === null) return;
  if (s.draft === s.base.content) {                                                                                           // the head already says it: nothing to post, nothing to keep
    s.unbased = false; s.ref = { id: s.base.revisionId, text: s.draft };
    if (pending.delete(doc)) await savePending(d);
    return;
  }
  if (conflicts.has(doc)) { s.status = 'conflict'; s.label = BLOCKED_WORDS; return; }                                       // a kept conflict is never overwritten, and nothing posts behind it
  if (s.unbased) { await conflict(doc, s, s.draft, d); return; }                                                              // no recorded base: never posted over the head (I2)
  const net = d.net ?? real;
  const gen = s.gen, mine = epoch;
  const snapshot = s.draft;
  let posted = false, shelved = false, merged: Body.Merged | null = null, text = snapshot;
  let r: { revisionId: string; version: number } | null = null;
  s.saving = true; s.status = 'saving'; s.label = 'Saving';
  try {
    let refused: unknown = null;
    for (let attempt = 0; r === null; attempt++) {
      // I1: the field's text at the last save is the common ground; the head is theirs. A head that is not what the field was is merged, not overwritten.
      const refText = s.ref.text ?? (await fetchRevision(net, s.ref.id)).content;
      if (s.ref.text === null) s.ref = { id: s.ref.id, text: refText };
      if (gen !== accountGeneration() || mine !== epoch) { s.saving = false; return; }
      const theirs = s.base.content;
      if (theirs === refText) text = snapshot;
      else { const m = Body.merge3(refText, snapshot, theirs); if (m === null) { shelved = true; break; } text = m; }
      if (text === s.base.content) { r = { revisionId: s.base.revisionId, version: s.base.version }; break; }                // the head already says it (a post whose answer was lost)
      try { merged = Body.merge(s.base.body, s.base.content, text); } catch (e) { throw new MergeError(e); }
      if (!s.key || s.key.text !== text) s.key = { text, id: uuid() };
      try { r = await postRevision(net, doc, text, merged.body, s.base.revisionId, s.key.id); posted = true; }
      catch (e) {
        if (!isStale(e) || attempt >= 3) throw e;
        refused = e;                                                                           // the head moved under us: read it again, merge, post with the fresh parent
        const head = await fetchHead(net, doc);
        if (gen !== accountGeneration() || mine !== epoch) { s.saving = false; return; }
        if (head.revisionId === s.base.revisionId) throw refused;                               // not a moved head: the server's own refusal
        s.base = head;
      }
    }
    if (gen !== accountGeneration() || mine !== epoch) { s.saving = false; return; }            // signed out meanwhile: nothing of it is kept
    if (shelved) { await conflict(doc, s, s.draft ?? snapshot, d); s.saving = false; return; }
    if (posted) s.base = { content: text, body: merged!.body, revisionId: r!.revisionId, version: r!.version };
    s.ref = text === snapshot ? { id: s.base.revisionId, text: snapshot } : { id: s.ref.id, text: snapshot };      // the field still holds `snapshot`; the head holds `text`
    if (s.draft === snapshot) pending.delete(doc); else keep(doc, s.draft!, s.ref.id);                              // typed meanwhile: kept, and merged over what this post changed
    await savePending(d);
    if (d.native) { try { await d.native({ op: 'textureSaved', doc, title: headingOf(text), text, version: s.base.version }); } catch { /* the Swift reader and search catch up at the next sync */ } }
    s.status = 'saved'; s.label = s.base.version > 0 ? `Saved · version ${s.base.version}` : 'Saved';
  } catch (e) {
    if (gen !== accountGeneration() || mine !== epoch) { s.saving = false; return; }
    const newest = s.draft !== null && s.draft !== snapshot ? s.draft : snapshot;               // typing during a failing save is not undone by the older text
    keep(doc, newest, s.ref.id); await savePending(d);
    if (isOffline(e)) { s.status = 'offline'; s.label = 'Offline: kept on this phone, posts when it can'; }
    else { s.status = 'failed'; s.label = `Not saved: ${saveSentence(e)}`; }
  }
  s.saving = false;
  if (s.status === 'saved' && s.draft !== null && s.draft !== snapshot) await save(doc, d);      // typed while it was posting
}

async function retryOne(doc: string, d: Deps): Promise<void> {
  if (!pending.has(doc) || conflicts.has(doc)) return;                                           // a kept conflict blocks the retry: nothing posts behind it
  let s = sessions.get(doc);
  if (!s || s.gen !== accountGeneration()) {
    s = (await session(doc, d)) ?? undefined;
    if (!s) return;                                                  // the route did not answer: still kept
  }
  await save(doc, d);
}
