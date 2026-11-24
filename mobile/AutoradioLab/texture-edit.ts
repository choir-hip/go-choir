// Editing your own Texture documents on the phone, in shared code (it was Swift: TextureEditor.swift, deleted with this file's arrival; the body merge is texture-body.ts). The editor is Exact's
// `textarea markup="markdown"` over the document's Markdown (members-content.contract); this file is everything behind it.
//
// THE STATE MODEL (written before the code, because an earlier port of this editor was sent back three times for holding four texts in fields whose meaning changed by path:
// docs/problems/texture-editor-in-exact-three-reviews-no-landing-2026-11-20.md). There are exactly two texts and one revision id per document:
//   head   the revision the server holds, as last read: {id, content, body, version}. Memory only.
//   kept   the listener's newest unsent text, and the revision it was typed over: {text, base, key}. Kept on the phone (`app:/data/texture-pending.json`) from the first keystroke until the
//          server holds it. `base` is the head's id when the text first differed from the head; it does not move while the text is unsent. `key` is the idempotency key of this text.
// `seed` is the text the field was seeded from when `kept` was typed over a head this editor could not read (offline) or that was not the field's text: `base` is then '' (unknown), and the text is posted
// only when the head, read later, still is the seed (then base := head.id); any other head is a CONFLICT (never a guess about who wrote what).
// The field is seeded from `open` (the head read NOW: `Info.head`), not from the reader's older copy, so the first keystroke descends from what the listener saw.
// A document has no other state: the field is whatever the page shows, and on reopening it is `info.kept` when there is one, else the head's text (the contract seeds it).
// Transitions (each has a test in texture-edit.test.ts, E1 to E14):
//   open           reads the head now. Answers `head` (its text), `opened`, and `kept`.
//   edit(text)     text equals head.content: kept is removed (idle). Otherwise kept = {text, base: kept.base ?? head.id, key: new} and is written to the file at once (dirty). With no head readable (offline)
//                  kept = {text, base: '', seed}. All calls on one document run one after another, in the order they came.
//   save           (flush, the contract's idle timer, Done, the phone leaving the front, a retry) reads the head's id, then
//                  - head.id == kept.base:  POST the revision with parent_revision_id = base (the server refuses a stale parent with 409 and never rebases: `allow_rebase` is not sent).
//                    Accepted: head = the new revision. kept is removed, unless the text changed while the post was out: then kept keeps the NEWER text with base = the new head's id, and it is saved next.
//                  - head.id != kept.base (another writer, or the kept text is from an earlier launch): the head is read. Same text as kept: kept is removed (a reply that was lost). Otherwise
//                    CONFLICT: nothing is posted, nothing is merged, kept stays, `info.kept` shows it, no retry. Only the listener's act ends it: `discard` (the head wins) or `overwrite`
//                    (kept.base := head.id, then save).
//                  - offline (no route, 5xx, 404, 429): kept stays, status offline, retried by the contract every 30 s and at launch. Any other refusal: failed with a sentence, kept stays.
//   wipe           sign-out or an account change: memory and file go; a save in flight finds the account generation moved and keeps nothing.
// Never two saves of one document at once (a typed edit during a save is `again`). No timer lives here: a fetch from background work is refused (docs/reference/exact/answers-let-go.md), so the
// WAIT (2 s after the last keystroke, 30 s between retries) is the contract's `task after(...)`, and every call below is an answer.
import type { Storage } from './app.contract.d.ts';
import { authedJSON, accountGeneration, segment, writeCurrent, signOutBlocked, SignedOutError, HTTPError, type Reply } from './api.ts';
import * as Body from './texture-body.ts';

export const PENDING_FILE = 'app:/data/texture-pending.json';
export type Info = { status: string; label: string; address: string; editable: boolean; kept: string; waiting: number; stamp: number; head: string; opened: boolean };
export type Net = { json(path: string, opts?: { method?: 'GET' | 'POST'; body?: unknown }): Promise<Reply> };
export type Deps = { net?: Net; storage?: Storage };
type Head = { id: string; content: string; body: Body.Body | null; version: number };
type Kept = { text: string; base: string; key: string; seed: string };
type Session = { head: Head | null; status: string; label: string; saving: boolean; again: boolean; conflict: boolean };

const real: Net = { json: (path, opts) => authedJSON(path, opts) };
const obj = (v: unknown): v is Record<string, unknown> => typeof v === 'object' && v !== null && !Array.isArray(v);
const str = (v: unknown): string => (typeof v === 'string' ? v : '');
const uuid = (): string => { const b = crypto.getRandomValues(new Uint8Array(16)); b[6] = (b[6] & 0x0f) | 0x40; b[8] = (b[8] & 0x3f) | 0x80; const h = Array.from(b, (x) => x.toString(16).padStart(2, '0')).join(''); return `${h.slice(0, 8)}-${h.slice(8, 12)}-${h.slice(12, 16)}-${h.slice(16, 20)}-${h.slice(20)}`; };

/** The words when the head moved under an unsent text: plain, and they say the text is safe. */
export const CONFLICT_WORDS = 'Not saved: this document changed somewhere else, and your changes could not be combined with it. Your text is kept on this phone.';

/** The document id of an address the listener may edit (the head of a Texture document), else null. A revision (`texture:<doc>@<rev>`) and a story (`pub:`) are read-only. */
export function editableDoc(address: string): string | null {
  if (!address.startsWith('texture:')) return null;
  const rest = address.slice('texture:'.length);
  if (rest === '' || rest.includes('@')) return null;
  return rest;
}

/** One plain sentence for a failed save. */
export function saveSentence(e: unknown): string {
  if (e instanceof SignedOutError) return 'Sign in again to save.';
  if (e instanceof HTTPError) return e.status === 403 ? 'This device is not allowed to save here.' : `Could not save. ${e.message}`;
  return 'Something went wrong saving the document.';
}
/** No route to the computer (or it is not answering): the text stays and is posted when it can be. */
export const isOffline = (e: unknown): boolean => (e instanceof HTTPError ? e.status >= 500 || e.status === 404 || e.status === 429 : !(e instanceof SignedOutError));

export class Editor {
  private net: Net;
  private storage: Storage | undefined;
  private sessions = new Map<string, Session>();
  private keptByDoc: Record<string, Kept> = {};
  private loaded = false;
  private loading: Promise<void> | undefined;
  private epoch = 0;
  private chain = new Map<string, Promise<unknown>>();
  private stamp = 0;
  constructor(deps: Deps = {}) { this.net = deps.net ?? real; this.storage = deps.storage; }

  private session(doc: string): Session {
    let s = this.sessions.get(doc);
    if (!s) { s = { head: null, status: 'idle', label: '', saving: false, again: false, conflict: false }; this.sessions.set(doc, s); }
    return s;
  }
  private bump(): void { this.stamp += 1; }

  /** One call at a time per document, in arrival order: a burst of keystrokes before the head is read is applied in order, the head is read once, and the last text wins. */
  private serial<T>(doc: string, fn: () => Promise<T>): Promise<T> {
    const run = (this.chain.get(doc) ?? Promise.resolve()).then(fn, fn);
    this.chain.set(doc, run.catch(() => undefined));
    return run;
  }

  // ------------------------------------------------------------------------------------------------------------ the kept file
  /** Read once; ONE read however many calls wait for it. A sign-out while it is under way (the epoch moved) throws its answer away: it is the old account's. */
  private load(): Promise<void> {
    if (this.loaded) return Promise.resolve();
    if (this.loading) return this.loading;
    const ep = this.epoch;
    this.loading = (async () => {
      let found: Record<string, Kept> = {};
      try {
        const o = JSON.parse(new TextDecoder().decode(await this.storage!.fs.readFile(PENDING_FILE)));
        if (obj(o)) for (const [doc, v] of Object.entries(o)) if (obj(v) && typeof v.text === 'string' && typeof v.base === 'string') found[doc] = { text: v.text, base: v.base, key: str(v.key) || uuid(), seed: str(v.seed) };
      } catch { found = {}; /* none, or not readable: nothing is kept */ }
      if (ep !== this.epoch) return;
      for (const [doc, v] of Object.entries(found)) if (!this.keptByDoc[doc]) this.keptByDoc[doc] = v;
      this.loaded = true; this.loading = undefined;
    })();
    return this.loading;
  }
  /** `gen` is the account generation the CALLER began under: a write after a sign-out moved it is refused (api.ts writeCurrent), and so is the removal. */
  private async persist(gen: number): Promise<void> {
    if (!this.storage || gen !== accountGeneration()) return;
    if (Object.keys(this.keptByDoc).length === 0) { try { await this.storage.fs.rm(PENDING_FILE); } catch { /* absent */ } return; }
    await writeCurrent(gen, this.storage, PENDING_FILE, new TextEncoder().encode(JSON.stringify(this.keptByDoc)));
  }

  // ------------------------------------------------------------------------------------------------------------ reads
  private async docRow(doc: string): Promise<{ id: string; trajectory: string }> {
    const r = await this.net.json(`/api/texture/documents/${segment(doc)}`);
    let d = obj(r.body) ? r.body : {};
    if (obj(d.document)) d = d.document;
    return { id: str(d.current_revision_id), trajectory: str(d.trajectory_id) };
  }
  private async revision(rid: string): Promise<Head> {
    if (rid === '') return { id: '', content: '', body: null, version: 0 };
    const r = await this.net.json(`/api/texture/revisions/${segment(rid)}`);
    let v = obj(r.body) ? r.body : {};
    if (obj(v.revision)) v = v.revision;
    const n = typeof v.version_number === 'number' ? v.version_number : -1;
    return { id: rid, content: str(v.content), body: obj(v.body_doc) ? (v.body_doc as Body.Body) : null, version: n + 1 };
  }

  // ------------------------------------------------------------------------------------------------------------ what the page sees
  async info(address: string): Promise<Info> {
    await this.load();
    const doc = editableDoc(address);
    // A sign-out under way (the generation moved, the wipe not yet run: it waits for the native half): the old account's text is not shown.
    if (signOutBlocked()) return { status: 'idle', label: '', address, editable: doc !== null, kept: '', waiting: 0, stamp: this.stamp, head: '', opened: false };
    const waiting = Object.keys(this.keptByDoc).filter((d) => !this.sessions.get(d)?.conflict).length;      // a conflict waits for the listener, not for a retry
    if (doc === null) return { status: 'idle', label: '', address, editable: false, kept: '', waiting, stamp: this.stamp, head: '', opened: false };
    const s = this.sessions.get(doc);
    const k = this.keptByDoc[doc];
    let status = s?.status ?? 'idle', label = s?.label ?? '';
    if (!s && k) { status = 'offline'; label = 'Edits kept on this phone'; }
    return { status, label, address, editable: true, kept: k?.text ?? '', waiting, stamp: this.stamp, head: s?.head?.content ?? '', opened: !!s?.head };
  }

  /** The head is read, and the field is seeded from it (or from the kept text). Offline: `opened` is false and the field keeps whatever the page already shows. */
  open(address: string): Promise<Info> {
    const doc = editableDoc(address);
    if (doc === null) return this.info(address);
    return this.serial(doc, async () => { await this.load(); await this.readHead(doc, accountGeneration()); return this.info(address); });
  }
  /** Reads the document's head into the session. False when it could not (the status says why). */
  private async readHead(doc: string, gen: number): Promise<boolean> {
    const s = this.session(doc);
    try { const row = await this.docRow(doc); const h = await this.revision(row.id); if (gen === accountGeneration()) s.head = h; return gen === accountGeneration(); }
    catch (e) { if (gen !== accountGeneration()) return false; s.status = isOffline(e) ? 'offline' : 'failed'; s.label = isOffline(e) ? 'Offline: could not open the document to save' : `Not saved: ${saveSentence(e)}`; this.bump(); return false; }
  }

  // ------------------------------------------------------------------------------------------------------------ editing
  /** The listener typed: the text is now `markdown`. It is kept on the phone at once and posted when `flush` is called. */
  edit(address: string, markdown: string, seed = ''): Promise<Info> {
    const doc = editableDoc(address);
    if (doc === null) return this.info(address);
    return this.serial(doc, async () => {
      await this.load();
      const s = this.session(doc);
      const gen = accountGeneration();
      if (!s.head) await this.readHead(doc, gen);
      if (gen !== accountGeneration()) return this.info(address);
      const k = this.keptByDoc[doc];
      if (!s.head) {                                                  // no head readable (offline): the text is kept, over the seed the field showed
        this.keptByDoc[doc] = { text: markdown, base: k ? k.base : '', seed: k ? k.seed : seed, key: uuid() };
        this.bump(); await this.persist(gen);
        return this.info(address);
      }
      if (markdown === s.head.content) {
        delete this.keptByDoc[doc];
        s.status = 'idle'; s.label = ''; s.conflict = false;
      } else {
        // The first keystroke of an unsent text: it descends from the head only when the field showed the head's text (or no seed was given). A field seeded from an older reading is over an UNKNOWN base.
        const fromHead = seed === '' || seed === s.head.content;
        this.keptByDoc[doc] = { text: markdown, base: k ? k.base : (fromHead ? s.head.id : ''), seed: k ? k.seed : (fromHead ? '' : seed), key: uuid() };
        if (!s.conflict) { s.status = 'dirty'; s.label = 'Editing'; }
      }
      this.bump();
      await this.persist(gen);
      return this.info(address);
    });
  }

  /** Post what is waiting for this document now (Done, leaving the page, the idle timer). An empty address is every document that has kept text (the retry, on any page, and at launch). */
  async flush(address: string): Promise<Info> {
    await this.load();
    if (address === '') { for (const doc of Object.keys(this.keptByDoc)) { const s = this.session(doc); if (!s.conflict) await this.save(doc); } return this.info(''); }
    const doc = editableDoc(address);
    if (doc !== null) await this.save(doc);
    return this.info(address);
  }

  /** The head wins: the kept text is dropped (the field is seeded from the head the next time it opens). */
  discard(address: string): Promise<Info> {
    const doc = editableDoc(address);
    if (doc === null) return this.info(address);
    return this.serial(doc, async () => {
      await this.load();
      const gen = accountGeneration();
      delete this.keptByDoc[doc];
      const s = this.session(doc); s.head = null; s.status = 'idle'; s.label = ''; s.conflict = false;
      this.bump(); await this.persist(gen);
      return this.info(address);
    });
  }
  /** The listener's own text wins: it is re-based on the head as it is now and saved. */
  overwrite(address: string): Promise<Info> {
    const doc = editableDoc(address);
    if (doc === null) return this.info(address);
    return this.serial(doc, async () => {
      await this.load();
      const k = this.keptByDoc[doc];
      if (!k) return this.info(address);
      const s = this.session(doc);
      const gen = accountGeneration();
      try {
        const row = await this.docRow(doc);
        const h = await this.revision(row.id);
        if (gen !== accountGeneration()) return this.info(address);
        s.head = h;
        k.base = h.id; k.seed = ''; k.key = uuid(); s.conflict = false;
        this.bump(); await this.persist(gen);
      } catch (e) { if (gen !== accountGeneration()) return this.info(address); s.status = isOffline(e) ? 'offline' : 'failed'; s.label = isOffline(e) ? 'Offline: kept on this phone, posts when it can' : `Not saved: ${saveSentence(e)}`; this.bump(); return this.info(address); }
      await this.save(doc);
      return this.info(address);
    });
  }

  private async save(doc: string): Promise<void> {
    const s = this.session(doc);
    if (s.saving) { s.again = true; return; }
    s.saving = true;
    const gen = accountGeneration();
    try {
      for (;;) {
        s.again = false;
        const k = this.keptByDoc[doc];
        if (!k || s.conflict) break;
        s.status = 'saving'; s.label = 'Saving'; this.bump();
        try {
          const row = await this.docRow(doc);
          if (gen !== accountGeneration()) return;
          if (row.id !== k.base) {                                       // another writer, or a text kept from an earlier launch over an older head
            const h = await this.revision(row.id);
            if (gen !== accountGeneration()) return;
            s.head = h;
            if (h.content === k.text) { delete this.keptByDoc[doc]; s.status = 'saved'; s.label = h.version > 0 ? `Saved · version ${h.version}` : 'Saved'; this.bump(); await this.persist(gen); break; }
            if (k.base === '' && k.seed !== '' && h.content === k.seed) { k.base = row.id; await this.persist(gen); }      // typed over the text the head still is: now it has a base
            else { this.conflict(s); break; }
          }
          if (!s.head || s.head.id !== k.base) s.head = await this.revision(k.base);          // a relaunch: the head this text descends from
          if (gen !== accountGeneration()) return;
          const merged = Body.merge(s.head.body, s.head.content, k.text);
          const payload: Record<string, unknown> = { content: k.text, body_doc: merged.body, idempotency_key: k.key };
          if (k.base !== '') payload.parent_revision_id = k.base;
          if (row.trajectory !== '') {                                  // as the web does: a document with a trajectory is saved against its lifecycle version
            const life = await this.net.json(`/api/texture/lifecycle/${segment(row.trajectory)}`);
            const t = obj(life.body) && obj(life.body.trajectory) ? life.body.trajectory : {};
            if (t.lifecycle_version !== undefined) payload.expected_lifecycle_version = t.lifecycle_version;
          }
          let r: Reply;
          try { r = await this.net.json(`/api/texture/documents/${segment(doc)}/revisions`, { method: 'POST', body: payload }); }
          catch (e) {
            if (e instanceof HTTPError && e.status === 409) {           // the head moved between the read and the post
              if (gen !== accountGeneration()) return;
              const row2 = await this.docRow(doc);
              const h = await this.revision(row2.id);
              if (gen !== accountGeneration()) return;
              s.head = h;
              if (h.content === k.text) { delete this.keptByDoc[doc]; s.status = 'saved'; s.label = h.version > 0 ? `Saved · version ${h.version}` : 'Saved'; this.bump(); await this.persist(gen); break; }
              this.conflict(s); break;
            }
            throw e;
          }
          if (gen !== accountGeneration()) return;                      // signed out meanwhile: nothing of it is kept
          let o = obj(r.body) ? r.body : {};
          if (obj(o.revision)) o = o.revision;
          const n = typeof o.version_number === 'number' ? o.version_number : -1;
          const id = str(o.revision_id);
          s.head = { id, content: k.text, body: merged.body, version: n + 1 };
          const now = this.keptByDoc[doc];
          if (now && now.text !== k.text) { now.base = id; }             // typed while it was posting: the newer text is kept over the new head and saved next
          else delete this.keptByDoc[doc];
          s.status = 'saved'; s.label = n >= 0 ? `Saved · version ${n + 1}` : 'Saved'; this.bump();
          await this.persist(gen);
          if (!this.keptByDoc[doc]) break;
        } catch (e) {
          if (gen !== accountGeneration()) return;
          s.status = isOffline(e) ? 'offline' : 'failed';
          s.label = isOffline(e) ? 'Offline: kept on this phone, posts when it can' : `Not saved: ${saveSentence(e)}`;
          this.bump();
          break;
        }
      }
    } finally { s.saving = false; }
  }
  private conflict(s: Session): void { s.status = 'conflict'; s.label = CONFLICT_WORDS; s.conflict = true; this.bump(); }

  /** Sign-out or an account change: the sessions and the kept text, in memory and on disk. A save in flight finds the generation moved and keeps nothing. */
  async wipe(): Promise<void> {
    this.epoch += 1; this.loading = undefined; this.chain = new Map();
    this.sessions = new Map(); this.keptByDoc = {}; this.loaded = true; this.bump();
    if (this.storage) { try { await this.storage.fs.rm(PENDING_FILE); } catch { /* absent */ } }
  }
}

// ---------------------------------------------------------------------------------------------------------------- the process's one editor (app.ts sources)
let live: Editor | undefined;
const editor = (storage: Storage | undefined): Editor => { if (!live) live = new Editor({ storage }); else if (storage) (live as unknown as { storage: Storage }).storage = storage; return live; };
/** The process's one editor, for the callers inside shared code (the notebook's topics write). */
export const liveEditor = (storage: Storage | undefined): Editor => editor(storage);
export const textureEditInfo = (address: string, storage: Storage | undefined): Promise<Info> => editor(storage).info(address);
export const textureEditOpen = (address: string, storage: Storage | undefined): Promise<Info> => editor(storage).open(address);
export const textureEdit = (address: string, markdown: string, seed: string, storage: Storage | undefined): Promise<Info> => editor(storage).edit(address, markdown, seed);
export const textureFlush = (address: string, storage: Storage | undefined): Promise<Info> => editor(storage).flush(address);
export const textureDiscard = (address: string, storage: Storage | undefined): Promise<Info> => editor(storage).discard(address);
export const textureOverwrite = (address: string, storage: Storage | undefined): Promise<Info> => editor(storage).overwrite(address);
/** Sign-out (account.ts WIPES): memory and the kept file. */
export async function wipeTextureEdit(storage: Storage | undefined): Promise<void> { const e = editor(storage); await e.wipe(); }
/** For tests: a fresh process. */
export function forgetTextureEdit(): void { live = undefined; }
