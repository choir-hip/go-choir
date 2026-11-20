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
//  - Sign-out or an account change: the sessions and the pending file go (account.ts WIPES `texture-edit`); a save in flight finds the account generation changed and keeps nothing.
//  - Demo: nothing is posted (a demo has no computer); the edit is kept in memory and the page says Saved.
//
// What stays native, and why: after a save, ONE call, `{op: 'textureSaved'}` on the app module, makes the Swift reader forget its copy of the document and puts the new text in the Swift
// content store ("kept and searchable at once"). Both stores are Swift until the reader and the content store move (content.ts is not yet the live store).
//
// Ways this can fail (each has a test in texture-edit.test.ts):
//  1. A keystroke per revision: the burst is one post.  2. An edit lost on a bad network, or posted twice after a relaunch.  3. A document nobody may edit (a revision, a story) opens a session.
//  4. A save that changes nothing is posted.  5. Typing while a save is in flight is dropped: it saves right after.  6. Another account's text shows or posts after a switch.
import type { Storage } from './app.contract.d.ts';
import { authedJSON, accountGeneration, segment, writeCurrent, SignedOutError, HTTPError, type Reply } from './api.ts';
import { WORDS } from './words.ts';
import * as Body from './texture-body.ts';

export const PENDING_FILE = 'app:/data/texture-pending.json';
export type Info = { status: string; label: string; address: string; editable: boolean };
export type Net = { json(path: string, opts?: { method?: 'GET' | 'POST'; body?: unknown }): Promise<Reply> };
export type Deps = { net?: Net; storage?: Storage; demo?: boolean; native?: (request: Record<string, unknown>) => Promise<unknown> };
type Head = { content: string; body: Body.Node | null; revisionId: string; version: number };
type Session = { base: Head; draft: string | null; status: string; label: string; saving: boolean; gen: number };

const real: Net = { json: (path, opts) => authedJSON(path, opts) };
const obj = (v: unknown): v is Body.Node => typeof v === 'object' && v !== null && !Array.isArray(v);
const str = (v: unknown): string => (typeof v === 'string' ? v : '');
const idle = (address = '', editable = false): Info => ({ status: 'idle', label: '', address, editable });

const sessions = new Map<string, Session>();
let pending = new Map<string, string>();
let pendingLoaded = false;
let retried = false;
/** The documents whose text an EARLIER launch kept (read from the file): only these are posted by the first answer; text typed in this session is its own session's to post. */
let earlier = new Set<string>();
let epoch = 0;

/** Sign-out, an account change, tests: nothing of the old account stays. */
export async function wipeTextureEdit(storage: Storage | undefined): Promise<void> {
  epoch += 1; sessions.clear(); pending = new Map(); earlier = new Set(); pendingLoaded = false; retried = false;
  if (storage) { try { await storage.fs.rm(PENDING_FILE); } catch { /* absent */ } }
}
export function resetTextureEdit(): void { epoch += 1; sessions.clear(); pending = new Map(); earlier = new Set(); pendingLoaded = false; retried = false; }

/** The document id of an address the listener may edit (the head of a Texture document), else null. */
export function editableDoc(address: string): string | null {
  if (!address.startsWith('texture:')) return null;
  const rest = address.slice('texture:'.length);
  const at = rest.indexOf('@');
  const doc = at < 0 ? rest : rest.slice(0, at);
  const rev = at < 0 ? '' : rest.slice(at + 1);
  return doc !== '' && rev === '' ? doc : null;
}

async function loadPending(d: Deps): Promise<void> {
  if (pendingLoaded) return;
  pendingLoaded = true;
  const mine = epoch;
  if (!d.storage) return;
  try {
    const o = JSON.parse(new TextDecoder().decode(await d.storage.fs.readFile(PENDING_FILE)));
    if (mine === epoch && obj(o)) for (const [k, v] of Object.entries(o)) if (typeof v === 'string' && !pending.has(k)) { pending.set(k, v); earlier.add(k); }
  } catch { /* none kept, or damaged: none */ }
}
async function savePending(d: Deps): Promise<void> {
  if (!d.storage) return;
  const gen = accountGeneration();
  if (pending.size === 0) { try { await d.storage.fs.rm(PENDING_FILE); } catch { /* absent */ } return; }
  await writeCurrent(gen, d.storage, PENDING_FILE, new TextEncoder().encode(JSON.stringify(Object.fromEntries(pending)))).catch(() => false);
}

// MARK: the server's routes (as the web uses them)

async function fetchHead(net: Net, doc: string): Promise<Head> {
  let d = (await net.json(`/api/texture/documents/${segment(doc)}`)).body;
  if (obj(d) && obj(d.document)) d = d.document;
  const rid = obj(d) ? str(d.current_revision_id) : '';
  if (rid === '') return { content: '', body: null, revisionId: '', version: 0 };
  let rev = (await net.json(`/api/texture/revisions/${segment(rid)}`)).body;
  if (obj(rev) && obj(rev.revision)) rev = rev.revision;
  const r = obj(rev) ? rev : {};
  const n = typeof r.version_number === 'number' ? Math.trunc(r.version_number) : -1;
  return { content: str(r.content), body: obj(r.body_doc) ? r.body_doc : null, revisionId: rid, version: n + 1 };
}

async function postRevision(net: Net, doc: string, content: string, body: Body.Node, parent: string): Promise<{ revisionId: string; version: number }> {
  const payload: Body.Node = { content, body_doc: body };
  if (parent !== '') payload.parent_revision_id = parent;
  // As the web does: a document with a trajectory is saved against its lifecycle (the version it read) with an idempotency key.
  let d = (await net.json(`/api/texture/documents/${segment(doc)}`)).body;
  if (obj(d) && obj(d.document)) d = d.document;
  const traj = obj(d) ? str(d.trajectory_id) : '';
  if (traj !== '') {
    const life = (await net.json(`/api/texture/lifecycle/${segment(traj)}`)).body;
    const l = obj(life) ? life : {};
    payload.idempotency_key = uuid();
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

/** Offline: the route did not answer (transport, timeout) or answered "try later"; anything else is a refusal the listener should read. */
export function isOffline(e: unknown): boolean {
  if (e instanceof SignedOutError) return false;
  if (e instanceof HTTPError) return e.status >= 500 || e.status === 404 || e.status === 429;
  return true;
}
/** One plain sentence for a failed save. */
export function saveSentence(e: unknown): string {
  if (e instanceof SignedOutError) return e.reason === 'locked' ? WORDS.locked : 'Sign in again to save.';
  if (e instanceof HTTPError) return e.status === 401 ? 'Sign in again to save.' : e.status === 403 ? 'This device is not allowed to save here.' : `Could not save. ${e.message}`;
  return 'Could not reach your computer. Check the connection and try again.';
}

// MARK: sessions

const demoHead = (): Head => ({ content: '', body: null, revisionId: '', version: 0 });
async function session(doc: string, d: Deps): Promise<Session | null> {
  const gen = accountGeneration();
  const have = sessions.get(doc);
  if (have && have.gen === gen) return have;
  sessions.delete(doc);
  try {
    const head = d.demo ? demoHead() : await fetchHead(d.net ?? real, doc);
    const again = sessions.get(doc);
    if (again && again.gen === gen) return again;
    if (gen !== accountGeneration()) return null;
    const s: Session = { base: head, draft: null, status: 'idle', label: '', saving: false, gen };
    sessions.set(doc, s);
    return s;
  } catch { return null; }
}

export async function textureEditInfo(address: string, d: Deps = {}, hidden = false): Promise<Info> {
  const doc = editableDoc(address);
  await loadPending(d);
  if (!retried) { retried = true; for (const k of [...earlier]) if (pending.has(k) && !sessions.has(k)) await retryOne(k, d); }          // what an earlier launch or a bad network kept
  if (hidden) await flushAll(d);                                                                                                     // the phone went to the background: what is waiting is posted now
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
  const s = await session(doc, d);
  if (!s) { pending.set(doc, markdown); await savePending(d); return info(doc, address); }       // no head yet (offline): the text is kept and posted when the route answers
  s.draft = markdown;
  if (markdown === s.base.content) { s.status = 'idle'; s.label = ''; pending.delete(doc); }
  else { s.status = 'dirty'; s.label = 'Editing'; pending.set(doc, markdown); }
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
  for (const doc of [...sessions.keys()]) await save(doc, d);
  await loadPending(d);
  for (const doc of [...pending.keys()]) if (!sessions.has(doc)) await retryOne(doc, d);
}

const headingOf = (text: string): string => text.split(/\r?\n|\r/).map((l) => l.replace(/^[#\s]+|\s+$/g, '')).find((l) => l !== '')?.slice(0, 120) ?? 'Document';

async function save(doc: string, d: Deps): Promise<void> {
  const s = sessions.get(doc);
  if (!s || s.saving || s.gen !== accountGeneration() || s.draft === null || s.draft === s.base.content) return;
  const draft = s.draft;
  s.saving = true; s.status = 'saving'; s.label = 'Saving';
  const gen = s.gen, mine = epoch;
  const merged = Body.merge(s.base.body, s.base.content, draft);
  try {
    const r = d.demo ? { revisionId: `demo-${s.base.version + 1}`, version: s.base.version + 1 } : await postRevision(d.net ?? real, doc, draft, merged.body, s.base.revisionId);
    if (gen !== accountGeneration() || mine !== epoch) { s.saving = false; return; }            // signed out meanwhile: nothing of it is kept
    s.base = { content: draft, body: merged.body, revisionId: r.revisionId, version: r.version };
    pending.delete(doc); await savePending(d);
    if (d.native) { try { await d.native({ op: 'textureSaved', doc, title: headingOf(draft), text: draft, version: r.version }); } catch { /* the Swift reader and search catch up at the next sync */ } }
    s.status = 'saved'; s.label = r.version > 0 ? `Saved · version ${r.version}` : 'Saved';
  } catch (e) {
    if (gen !== accountGeneration() || mine !== epoch) { s.saving = false; return; }
    pending.set(doc, draft); await savePending(d);
    if (isOffline(e)) { s.status = 'offline'; s.label = 'Offline: kept on this phone, posts when it can'; }
    else { s.status = 'failed'; s.label = `Not saved: ${saveSentence(e)}`; }
  }
  s.saving = false;
  if (s.draft !== null && s.draft !== s.base.content && s.status === 'saved') await save(doc, d);      // typed while it was posting
}

async function retryOne(doc: string, d: Deps): Promise<void> {
  const text = pending.get(doc);
  if (text === undefined) return;
  let s = sessions.get(doc);
  if (!s || s.gen !== accountGeneration()) {
    s = (await session(doc, d)) ?? undefined;
    if (!s) return;                                                  // the route did not answer: still kept
  }
  s.draft = text;
  await save(doc, d);
}
/** Post what an earlier launch or a bad network kept. The newest text wins over the head as it is now. */
export async function retryPending(d: Deps = {}): Promise<void> {
  await loadPending(d);
  for (const doc of [...pending.keys()]) await retryOne(doc, d);
}
