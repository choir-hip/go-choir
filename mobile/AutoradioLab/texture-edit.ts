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
//    nothing. It is posted when the route answers (the contract asks again every 30 s while the page says offline or failed, and the first answer after a launch posts what an earlier launch kept).
//    The newest text wins; the parent is the head at that moment.
//  - Sign-out or an account change: the sessions and the pending file go (account.ts WIPES `texture-edit`); a save in flight finds the account generation changed and keeps nothing. The kept
//    file names the account (the SHA-256 of the sign-in key, api.ts keyState): another account's file is not loaded and is removed; after the account generation moves, nothing of it shows or posts.
//  - A head that moved (review round 1): the server refuses a stale parent with 409. The editor then re-reads the head and merges the draft onto it by lines (texture-body.ts `merge3`, three-way, from the
//    revision the draft was written over); when the two changed the same lines it posts nothing, keeps the text with its base revision, and says so (status `conflict`, no retry). A kept text is written
//    with the revision it was written over, so a relaunch merges the same way and never posts over a newer version. The app's own topic sync (Notebook.swift, still Swift) posts behind this session; the
//    first save after it meets a 409 and takes this road, so a stale base cannot last.
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
export type Info = { status: string; label: string; address: string; editable: boolean };
export type Net = { json(path: string, opts?: { method?: 'GET' | 'POST'; body?: unknown }): Promise<Reply> };
export type Deps = { net?: Net; storage?: Storage; native?: (request: Record<string, unknown>) => Promise<unknown> };
type Head = { content: string; body: Body.Node | null; revisionId: string; version: number };
/** The revision a draft was written over, when that is not the head the next post goes against (`content` null until it is read). */
type Origin = { id: string; content: string | null };
type Session = { base: Head; draft: string | null; status: string; label: string; saving: boolean; gen: number; origin: Origin | null; conflictDraft: string | null; key: { text: string; id: string } | null };
/** What is kept for a document: the newest text, and the revision it was written over ('' when that is not known: text from the build before this one, or typed with no route). */
type Kept = { text: string; base: string };

const real: Net = { json: (path, opts) => authedJSON(path, opts) };
const obj = (v: unknown): v is Body.Node => typeof v === 'object' && v !== null && !Array.isArray(v);
const str = (v: unknown): string => (typeof v === 'string' ? v : '');
const idle = (address = '', editable = false): Info => ({ status: 'idle', label: '', address, editable });
/** The words when the same lines were changed here and elsewhere: plain, and they say the text is safe. */
export const CONFLICT_WORDS = 'Not saved: this document changed somewhere else, and your changes could not be combined with it. Your text is kept on this phone.';
/** The draft could not be turned into a body (a damaged `body_doc`): a bug or bad data, never "offline". */
class MergeError extends Error { constructor(e: unknown) { super(e instanceof Error ? e.message : 'merge failed'); this.name = 'MergeError'; } }

const sessions = new Map<string, Session>();
let pending = new Map<string, Kept>();
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

function clear(): void { epoch += 1; sessions.clear(); pending = new Map(); earlier = new Set(); loading = null; retried = false; scopeGen = null; resignSeen = 0; digestMemo = null; }
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
  if (scopeGen !== g) { scopeGen = g; pending = new Map(); earlier = new Set(); loading = Promise.resolve(); retried = true; }
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
async function doLoad(d: Deps): Promise<void> {
  const mine = epoch, gen = accountGeneration();
  if (!d.storage) return;
  try {
    const o = JSON.parse(new TextDecoder().decode(await d.storage.fs.readFile(PENDING_FILE)));
    if (obj(o) && o.v === 2 && obj(o.docs)) {
      const k = await keyState();
      const foreign = k.state === 'absent' || (k.state === 'present' && str(o.account) !== '' && str(o.account) !== k.digest);       // signed out, or another account's text
      if (mine !== epoch || gen !== accountGeneration()) return;
      if (foreign) { try { await d.storage.fs.rm(PENDING_FILE); } catch { /* absent */ } }
      else for (const [doc, v] of Object.entries(o.docs)) if (obj(v) && typeof v.text === 'string' && !pending.has(doc)) { pending.set(doc, { text: v.text, base: str(v.base) }); earlier.add(doc); }
    }
  } catch { /* none kept, or damaged: none */ }
  await migrateOldFile(d, mine, gen);
}
/** The build before this one kept an unsent edit outside app:/ (see the header, native call 2). Read, merged, written durably in the new place, THEN removed from the old: a crash in between loses
 * nothing and sends nothing twice. Text with no known base ('') is posted as that build did: the newest text wins, the parent is the head at that moment. */
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
  for (const [doc, text] of Object.entries(old)) if (typeof text === 'string' && !pending.has(doc)) { pending.set(doc, { text, base: '' }); earlier.add(doc); }
  if (!(await savePending(d))) return;                                             // not durable in the new place: the old file stays
  if (mine !== epoch || gen !== accountGeneration()) return;
  await remove();
}
/** True when the file is as `pending` says (or there is nowhere to write it). */
async function savePending(d: Deps): Promise<boolean> {
  if (!d.storage) return true;
  const gen = accountGeneration();
  if (pending.size === 0) { try { await d.storage.fs.rm(PENDING_FILE); } catch { /* absent */ } return true; }
  const account = await ownerDigest();
  const bytes = new TextEncoder().encode(JSON.stringify({ v: 2, account, docs: Object.fromEntries(pending) }));
  return writeCurrent(gen, d.storage, PENDING_FILE, bytes).catch(() => false);
}
const keep = (doc: string, text: string, base: string): void => { pending.set(doc, { text, base }); };

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
    const s: Session = { base: head, draft: null, status: 'idle', label: '', saving: false, gen, origin: null, conflictDraft: null, key: null };
    sessions.set(doc, s);
    return s;
  } catch { return null; }
}
/** What the native side says about the app going to the background (see the header, native call 3). */
async function resignCount(d: Deps): Promise<number> {
  try { const r = (await d.native!({ op: 'textureResigned' })) as { count?: unknown } | null; return r && typeof r.count === 'number' ? r.count : 0; } catch { return 0; }
}

export async function textureEditInfo(address: string, d: Deps = {}, hidden = false): Promise<Info> {
  const doc = editableDoc(address);
  await loadPending(d);
  if (!retried) { retried = true; for (const k of [...earlier]) if (pending.has(k) && !sessions.has(k)) await retryOne(k, d); }          // what an earlier launch or a bad network kept
  let away = hidden;
  if (d.native && (sessions.size > 0 || pending.size > 0)) { const n = await resignCount(d); if (n > resignSeen) { resignSeen = n; away = true; } }
  if (away) await flushAll(d);                                                                                                       // the phone went to the background: what is waiting is posted now
  else for (const k of [...sessions.keys()]) if (k !== doc) await save(k, d);                                                        // the page in front is not that document's: leaving a page posts what was typed on it
  return doc === null ? idle() : info(doc, address);
}
function info(doc: string, address: string): Info {
  const s = sessions.get(doc);
  if (s && s.gen === accountGeneration()) return { status: s.status, label: s.label, address, editable: true };
  if (pending.has(doc)) return { status: 'offline', label: 'Edits kept on this phone', address, editable: true };
  return idle(address, true);
}

/** The listener typed: the text is now `markdown`. Kept on the phone at once; posted when `textureFlush` is called (the contract's pause in typing). */
export async function textureEdit(address: string, markdown: string, d: Deps = {}): Promise<Info> {
  const doc = editableDoc(address);
  if (doc === null) return idle();
  await loadPending(d);
  const had = sessions.has(doc);
  const s = await session(doc, d);
  if (!s) { keep(doc, markdown, pending.get(doc)?.base ?? ''); await savePending(d); return info(doc, address); }       // no head yet (offline): the text is kept and posted when the route answers
  if (!had && s.origin === null) { const b = pending.get(doc)?.base ?? ''; if (b !== '' && b !== s.base.revisionId) s.origin = { id: b, content: null }; }   // kept over an older revision: merged, not laid over
  s.draft = markdown;
  const over = s.origin?.id ?? s.base.revisionId;
  if (markdown === s.base.content && s.origin === null) { s.status = 'idle'; s.label = ''; pending.delete(doc); }
  else { s.status = 'dirty'; s.label = 'Editing'; keep(doc, markdown, over); }
  await savePending(d);
  return info(doc, address);
}

/** Post now what is waiting (Done, the pause in typing, the retry). */
export async function textureFlush(address: string, d: Deps = {}): Promise<Info> {
  const doc = editableDoc(address);
  if (doc === null) return idle();
  await loadPending(d);
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

const headingOf = (text: string): string => text.split(/\r?\n|\r/).map((l) => l.replace(/^[#\s]+|\s+$/g, '')).find((l) => l !== '')?.slice(0, 120) ?? 'Document';

type Rebased = { kind: 'ok' | 'same' | 'conflict' | 'unchanged'; text: string; from: Origin | null };
/** The head moved since `draft` was written (a 409, or a text kept over an older revision): read the head and lay the draft over it, three-way, from the revision it was written over.
 * `session.base` becomes the head. 'same': the head already says it (a post whose answer was lost). 'conflict': the same lines changed both ways; the session keeps its origin. 'unchanged': the head is the origin. */
async function rebase(net: Net, doc: string, s: Session, draft: string): Promise<Rebased> {
  const o = s.origin!;
  const head = await fetchHead(net, doc);
  if (head.revisionId === o.id) return { kind: 'unchanged', text: draft, from: null };
  const from: Origin = { id: o.id, content: o.content ?? (await fetchRevision(net, o.id)).content };
  s.base = head;
  if (head.content === draft) { s.origin = null; return { kind: 'same', text: draft, from }; }
  if (head.content === from.content) { s.origin = null; return { kind: 'ok', text: draft, from }; }
  const m = Body.merge3(from.content!, draft, head.content);
  if (m === null) { s.origin = from; return { kind: 'conflict', text: draft, from }; }
  s.origin = null;
  return { kind: 'ok', text: m, from };
}

async function save(doc: string, d: Deps): Promise<void> {
  const s = sessions.get(doc);
  if (!s || s.saving || s.gen !== accountGeneration() || s.draft === null) return;
  if (s.draft === s.base.content && s.origin === null) { if (pending.delete(doc)) await savePending(d); return; }      // the head already says it: nothing to post, nothing to keep
  if (s.status === 'conflict' && s.conflictDraft === s.draft) return;                                                     // told; waiting for the owner's next edit
  const net = d.net ?? real;
  const gen = s.gen, mine = epoch;
  const snapshot = s.draft;
  let draft = snapshot, posted = false, conflict = false, merged: Body.Merged | null = null, from: Origin | null = null;
  let r: { revisionId: string; version: number } | null = null;
  s.saving = true; s.status = 'saving'; s.label = 'Saving';
  try {
    let refused: unknown = null;
    for (let attempt = 0; r === null; attempt++) {
      if (s.origin) {
        const rb = await rebase(net, doc, s, draft);
        if (gen !== accountGeneration() || mine !== epoch) { s.saving = false; return; }
        if (rb.kind === 'unchanged') throw refused ?? new HTTPError(409, 'document head changed; reload the latest version before saving');
        if (rb.from) from = rb.from;
        if (rb.kind === 'conflict') { conflict = true; break; }
        if (rb.kind === 'same') { r = { revisionId: s.base.revisionId, version: s.base.version }; break; }
        draft = rb.text;
      }
      if (draft === s.base.content) { r = { revisionId: s.base.revisionId, version: s.base.version }; break; }
      try { merged = Body.merge(s.base.body, s.base.content, draft); } catch (e) { throw new MergeError(e); }
      if (!s.key || s.key.text !== draft) s.key = { text: draft, id: uuid() };
      try {
        r = await postRevision(net, doc, draft, merged.body, s.base.revisionId, s.key.id);
        posted = true;
      } catch (e) {
        if (!isStale(e) || attempt >= 3) throw e;
        refused = e;                                                                           // the head moved under us: read it, merge, post again with the fresh parent
        s.origin = { id: s.base.revisionId, content: s.base.content };
      }
    }
    if (gen !== accountGeneration() || mine !== epoch) { s.saving = false; return; }            // signed out meanwhile: nothing of it is kept
    const typed = s.draft !== snapshot;                                                           // the owner typed while it was posting
    if (conflict) {
      s.status = 'conflict'; s.label = CONFLICT_WORDS; s.conflictDraft = s.draft;
      keep(doc, s.draft ?? draft, s.origin?.id ?? s.base.revisionId); await savePending(d);
      s.saving = false;
      return;
    }
    if (posted) s.base = { content: draft, body: merged!.body, revisionId: r!.revisionId, version: r!.version };
    s.origin = null;
    if (!typed) { s.draft = draft; pending.delete(doc); }
    else { if (draft !== snapshot && from) s.origin = from; keep(doc, s.draft!, s.origin?.id ?? s.base.revisionId); }       // newer text than what was posted: kept, and merged over what this post changed
    await savePending(d);
    if (d.native) { try { await d.native({ op: 'textureSaved', doc, title: headingOf(draft), text: draft, version: s.base.version }); } catch { /* the Swift reader and search catch up at the next sync */ } }
    s.status = 'saved'; s.label = s.base.version > 0 ? `Saved · version ${s.base.version}` : 'Saved';
  } catch (e) {
    if (gen !== accountGeneration() || mine !== epoch) { s.saving = false; return; }
    const newest = s.draft !== snapshot && s.draft !== null ? s.draft : draft;                  // typing during a failing save is not undone by the older text
    keep(doc, newest, s.origin?.id ?? s.base.revisionId); await savePending(d);
    if (isOffline(e)) { s.status = 'offline'; s.label = 'Offline: kept on this phone, posts when it can'; }
    else { s.status = 'failed'; s.label = `Not saved: ${saveSentence(e)}`; }
  }
  s.saving = false;
  if (s.draft !== null && (s.draft !== s.base.content || s.origin !== null) && s.status === 'saved') await save(doc, d);      // typed while it was posting
}

async function retryOne(doc: string, d: Deps): Promise<void> {
  const kept = pending.get(doc);
  if (kept === undefined) return;
  let s = sessions.get(doc);
  if (!s || s.gen !== accountGeneration()) {
    s = (await session(doc, d)) ?? undefined;
    if (!s) return;                                                  // the route did not answer: still kept
  }
  s.draft = kept.text;
  if (s.origin === null && kept.base !== '' && kept.base !== s.base.revisionId) s.origin = { id: kept.base, content: null };      // written over an older revision than the head: merged, never laid over
  await save(doc, d);
}
