// Editing your own Texture documents on the phone, in shared code (it was Swift: TextureEditor.swift, deleted with this file's arrival; the body merge is texture-body.ts). The editor is Exact's
// `textarea markup="markdown"` over the document's Markdown (members-content.contract); this file is everything behind it.
//
// THE STATE MODEL (written before the code, because an earlier port of this editor was sent back three times for holding four texts in fields whose meaning changed by path:
// docs/problems/texture-editor-in-exact-three-reviews-no-landing-2026-11-20.md). There are exactly two texts and one revision id per document:
//   head   the revision the server holds, as last read: {id, content, body, version}. Memory only.
//   kept   the listener's newest unsent text, and the revision it was typed over: {text, base, key}. Kept on the phone (`app:/data/texture-pending.json`) from the first keystroke until the
//          server holds it. `base` is the head's id when the text first differed from the head; it does not move while the text is unsent. `key` is the idempotency key of this text.
// `seed` is the text the field showed before the first keystroke of `kept`, when that was not the head's text or the head could not be read (offline; null: the caller made no claim, '' is an empty text): `base` is then '' (unknown), and the text is posted
// only when the head, read later, still is the seed, or no claim was made (then base := head.id); any other head is a CONFLICT (never a guess about who wrote what).
// The field is seeded from `open` (the head read NOW: `Info.head`), not from the reader's older copy, so the first keystroke descends from what the listener saw.
// A document has no other state: the field is whatever the page shows, and on reopening it is `info.kept` when there is one, else the head's text (the contract seeds it).
// The unsent file belongs to an ACCOUNT (a digest of its computer id; of the key when it has none): another account's file is deleted, an unreadable or absent key keeps it unread and unwritten. Every keystroke is written
// to it by ONE writer before the call waits for the queue or the network. Transitions (each has a test in texture-edit.test.ts, E1 to E29):
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
// Never two calls on one document at once (ONE queue per document, `run`: a keystroke typed during a save waits behind it, then is kept on the new head). No timer lives here: a fetch from background work is refused (docs/reference/exact/answers-let-go.md), so the
// WAIT (2 s after the last keystroke, 30 s between retries) is the contract's `task after(...)`, and every call below is an answer.
import type { Storage } from './app.contract.d.ts';
import { authedJSON, accountGeneration, accountIdentity, segment, signOutBlocked, SignedOutError, HTTPError, type Reply } from './api.ts';
import * as Body from './texture-body.ts';

/** How long one network call of the editor may take before the text is simply kept for a later try. */
export const NET_TIMEOUT_MS = 20_000;
export const PENDING_FILE = 'app:/data/texture-pending.json';
export type Info = { status: string; label: string; address: string; editable: boolean; kept: string; hasKept: boolean; waiting: number; stamp: number; head: string; opened: boolean };
export type Net = { json(path: string, opts?: { method?: 'GET' | 'POST'; body?: unknown }): Promise<Reply> };
export type Deps = { net?: Net; storage?: Storage; timeoutMs?: number };
type Head = { id: string; content: string; body: Body.Body | null; version: number };
type Kept = { text: string; base: string; key: string; seed: string | null };
type Session = { head: Head | null; status: string; label: string; conflict: boolean; skipReads: number };

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
/** A reply this call was let go for: the same mutation was sent again, so this answer may not fetch any more (docs/reference/exact/answers-let-go.md). Not a failure and not offline: the newer answer does the work. */
export const isLetGo = (e: unknown): boolean => e instanceof Error && /let go/i.test(e.message);
export const isOffline = (e: unknown): boolean => (e instanceof HTTPError ? e.status >= 500 || e.status === 404 || e.status === 429 : !(e instanceof SignedOutError));

/** What a call began under: the account generation, the wipe epoch, and WHO is signed in (the account id from `accountIdentity`, api.ts: the one helper mail's `keyOwner` also comes from). Everything a call does
 * after an await is checked against it (`live`): a sign-out that has begun, the wipe, or ANOTHER ACCOUNT's key (a re-sign-in after a refusal moves no generation) makes the call a no-op. */
type Tok = { gen: number; epoch: number; who: string | null };

// SIX INVARIANTS, each with a test that fails without it (texture-edit.test.ts):
//  1  Every call is made AS an account (the token's `who`, from the one identity helper in api.ts) and does nothing once that account is not the one signed in, whatever the generation: E30, E35.
//  2  The unsent file has ONE writer, is written BEFORE a keystroke waits for the queue or the network, is deleted only for ANOTHER account (never for a key that cannot be read), and a late write of a gone account never removes the new account's file: E26, E27, E28, E31.
//  3  `Info.hasKept` says whether there is a kept text, so an EMPTY kept text (everything deleted) is a text, not "none": E32.
//  4  A document with NO revision yet is a head: its seed is its empty text, so another writer's first revision is a conflict, never overwritten: E33.
//  5  A writer that is not the listener goes through `writeAutomatic`, one call inside the queue: it yields to the listener's text and only ever drops its own: E34.
//  6  Nothing is posted, and nothing fetched for a post, once a sign-out has begun: E35.
// Named, not a bug: a head that moved is a conflict even when the new revision has the SAME text, and so is a 409 that is only the lifecycle version: the editor never guesses who wrote what; "Keep mine" resolves it.
// THE INVARIANT (rewritten after two reviews found calls that bypassed the queue and writes that ignored a sign-out): there is exactly ONE way to touch a document, `run(doc, op)`. Open, edit,
// flush, discard and overwrite all enter it; it runs them one at a time per document in the order they arrived; and it hands each one a `Tok` that the operation, and `persist`, check before
// every write. A call whose `Tok` is stale (a sign-out began, or the wipe ran) does nothing and answers an empty `Info`. No operation touches `sessions` or `keptByDoc` outside it, and `wipe`
// is the only thing that replaces them (it moves the epoch first, so every queued call is stale). The "seed" is `string | null`: null is "the caller makes no claim" and '' is an EMPTY text.
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
  private timeoutMs: number;
  constructor(deps: Deps = {}) { this.net = deps.net ?? real; this.storage = deps.storage; this.timeoutMs = deps.timeoutMs ?? NET_TIMEOUT_MS; }
  /** Every network call of the editor ends in NET_TIMEOUT_MS: a link that answers nothing must not hold a document's queue for the signed-in client's 120 s. The unsent text is on disk before any of this. */
  private json(path: string, opts?: { method?: 'GET' | 'POST'; body?: unknown }): Promise<Reply> {
    let timer: ReturnType<typeof setTimeout> | undefined;
    const late = new Promise<never>((_, rej) => { timer = setTimeout(() => rej(new Error('timeout: no answer from the computer')), this.timeoutMs); });
    return Promise.race([this.net.json(path, opts), late]).finally(() => clearTimeout(timer));
  }

  private session(doc: string): Session {
    let s = this.sessions.get(doc);
    if (!s) { s = { head: null, status: 'idle', label: '', conflict: false, skipReads: 0 }; this.sessions.set(doc, s); }
    return s;
  }
  private bump(): void { this.stamp += 1; }
  private owner: string | null = null;
  /** Called first by every public entry. Another account's key than the one this editor's memory belongs to (a re-sign-in with no sign-out) empties the memory and its file before anything is read; the token it
   * returns names the account the call began as. An unreadable key is not another account: nothing is emptied, and the token names nobody (`who: null`) until it can be read. */
  private async adopt(): Promise<Tok> {
    const who = await accountIdentity();
    const id = who.kind === 'known' ? who.id : null;
    if (id !== null && this.owner !== null && this.owner !== id) await this.wipe();
    if (id !== null) this.owner = id;
    return { gen: accountGeneration(), epoch: this.epoch, who: id };
  }
  private async live(t: Tok): Promise<boolean> {
    if (t.gen !== accountGeneration() || t.epoch !== this.epoch || signOutBlocked()) return false;
    if (t.who === null) return true;
    const now = await accountIdentity();
    return now.kind !== 'known' || now.id === t.who;
  }

  /** The one door: `fn` runs after every earlier call on this document has finished, and only if the account has not moved since the call was made; else `stale()` answers. */
  private run<T>(doc: string, t: Tok, fn: (t: Tok) => Promise<T>, stale: () => Promise<T>): Promise<T> {
    const go = async (): Promise<T> => ((await this.live(t)) ? fn(t) : stale());
    const p = (this.chain.get(doc) ?? Promise.resolve()).then(go, go);
    this.chain.set(doc, p.then(() => undefined, () => undefined));
    return p;
  }

  // ------------------------------------------------------------------------------------------------------------ the kept file
  /** Read once; ONE read however many calls wait for it. The file is the unsent text of the account `owner` names. It is deleted ONLY when the key store says another account is signed in (`known` with
   * another id); when the key cannot be read yet (no native handle, a locked Keychain) or says there is none, the file is KEPT, not loaded, and nothing is written over it: the next call tries again. */
  private load(): Promise<void> {
    if (this.loaded) return Promise.resolve();
    if (this.loading) return this.loading;
    const ep = this.epoch;
    this.loading = (async () => {
      let found: Record<string, Kept> = {};
      let settled = true;
      try {
        const o = JSON.parse(new TextDecoder().decode(await this.storage!.fs.readFile(PENDING_FILE)));
        const who = await accountIdentity();
        if (who.kind !== 'known') settled = false;                                       // cannot say whose it is: keep it, load nothing, ask again later
        else if (obj(o) && o.owner === who.id && obj(o.docs)) {
          for (const [doc, v] of Object.entries(o.docs)) if (obj(v) && typeof v.text === 'string' && typeof v.base === 'string') found[doc] = { text: v.text, base: v.base, key: str(v.key) || uuid(), seed: typeof v.seed === 'string' ? v.seed : null };
        } else if (this.storage && ep === this.epoch) { try { await this.storage.fs.rm(PENDING_FILE); } catch { /* absent */ } }      // another account's text: not kept
      } catch { found = {}; /* no file, or not readable: nothing is kept */ }
      this.loading = undefined;
      if (ep !== this.epoch) return;
      for (const [doc, v] of Object.entries(found)) if (!this.keptByDoc[doc]) this.keptByDoc[doc] = v;
      if (settled) this.loaded = true;
    })();
    return this.loading;
  }
  /** ONE writer for the one file: every document's queue asks for a write, the writer coalesces them and always writes the map as it is WHEN IT RUNS, so two documents cannot land out of order. */
  private pDirty = false;
  private pRun: Promise<void> | undefined;
  private diskPending = false;
  private persist(): Promise<void> {
    this.pDirty = true;
    if (!this.pRun) this.pRun = (async () => { try { while (this.pDirty) { this.pDirty = false; await this.writeFile(); } } finally { this.pRun = undefined; } })();
    return this.pRun;
  }
  private async persistIf(t: Tok): Promise<void> { if (await this.live(t)) await this.persist(); }
  private async writeFile(): Promise<void> {
    if (!this.storage || signOutBlocked()) return;
    if (!this.loaded) { this.diskPending = true; return; }                                // the file that is there has not been read: not written over
    const gen = accountGeneration(), ep = this.epoch;
    const who = await accountIdentity();
    if (who.kind !== 'known' || gen !== accountGeneration() || ep !== this.epoch) { this.diskPending = true; return; }      // not writable now; written when the key can be read
    if (Object.keys(this.keptByDoc).length === 0) { await this.rmIfOwner(who.id); this.diskPending = false; return; }
    const owner = who.id;
    try { await this.storage.fs.atomicWriteFile(PENDING_FILE, new TextEncoder().encode(JSON.stringify({ owner, docs: this.keptByDoc }))); } catch { this.diskPending = true; return; }
    if (gen !== accountGeneration() || ep !== this.epoch) { await this.rmIfOwner(owner); return; }      // this write was for an account that has gone: removed, but only if the file still is ITS file
    this.diskPending = false;
  }
  /** Removes the pending file only when it names `owner`: the file of the account that signed in since is never touched by a late write of the one that left. */
  private async rmIfOwner(owner: string): Promise<void> {
    if (!this.storage) return;
    try { const o = JSON.parse(new TextDecoder().decode(await this.storage.fs.readFile(PENDING_FILE))); if (!obj(o) || o.owner === owner) await this.storage.fs.rm(PENDING_FILE); } catch { /* absent */ }
  }

  // ------------------------------------------------------------------------------------------------------------ reads
  private async docRow(doc: string): Promise<{ id: string; trajectory: string }> {
    const r = await this.json(`/api/texture/documents/${segment(doc)}`);
    let d = obj(r.body) ? r.body : {};
    if (obj(d.document)) d = d.document;
    return { id: str(d.current_revision_id), trajectory: str(d.trajectory_id) };
  }
  private async revision(rid: string): Promise<Head> {
    if (rid === '') return { id: '', content: '', body: null, version: 0 };
    const r = await this.json(`/api/texture/revisions/${segment(rid)}`);
    let v = obj(r.body) ? r.body : {};
    if (obj(v.revision)) v = v.revision;
    const n = typeof v.version_number === 'number' ? v.version_number : -1;
    return { id: rid, content: str(v.content), body: obj(v.body_doc) ? (v.body_doc as Body.Body) : null, version: n + 1 };
  }
  /** Reads the document's head into the session. False when it could not (the status says why). */
  private async readHead(doc: string, t: Tok): Promise<boolean> {
    const s = this.session(doc);
    try { const row = await this.docRow(doc); const h = await this.revision(row.id); if (!(await this.live(t))) return false; s.head = h; return true; }
    catch (e) { if (!(await this.live(t)) || isLetGo(e)) return false; s.skipReads = this.queuedEdits.get(doc) ?? 0; s.status = isOffline(e) ? 'offline' : 'failed'; s.label = isOffline(e) ? 'Offline: could not open the document to save' : `Not saved: ${saveSentence(e)}`; this.bump(); return false; }
  }

  // ------------------------------------------------------------------------------------------------------------ what the page sees
  async info(address: string): Promise<Info> {
    await this.adopt();
    await this.load();
    if (this.loaded && this.diskPending) await this.persist();                          // text typed while the key could not be read reaches the disk now
    const doc = editableDoc(address);
    // A sign-out under way (the generation moved, the wipe not yet run: it waits for the native half): the old account's text is not shown.
    if (signOutBlocked()) return { status: 'idle', label: '', address, editable: doc !== null, kept: '', hasKept: false, waiting: 0, stamp: this.stamp, head: '', opened: false };
    const waiting = Object.keys(this.keptByDoc).filter((d) => !this.sessions.get(d)?.conflict).length;      // a conflict waits for the listener, not for a retry
    if (doc === null) return { status: 'idle', label: '', address, editable: false, kept: '', hasKept: false, waiting, stamp: this.stamp, head: '', opened: false };
    const s = this.sessions.get(doc);
    const k = this.keptByDoc[doc];
    let status = s?.status ?? 'idle', label = s?.label ?? '';
    if (!s && k) { status = 'offline'; label = 'Edits kept on this phone'; }
    return { status, label, address, editable: true, kept: k?.text ?? '', hasKept: !!k, waiting, stamp: this.stamp, head: s?.head?.content ?? '', opened: !!s?.head };
  }

  /** The head is read, and the field is seeded from it (or from the kept text). Offline: `opened` is false and the field keeps whatever the page already shows. */
  async open(address: string): Promise<Info> {
    const doc = editableDoc(address);
    if (doc === null) return this.info(address);
    const t = await this.adopt();
    return this.run(doc, t, async (t1) => { await this.load(); if (await this.live(t1)) await this.readHead(doc, t1); return this.info(address); }, () => this.info(address));
  }

  // ------------------------------------------------------------------------------------------------------------ editing
  private queuedEdits = new Map<string, number>();
  /** Makes the kept record of the latest text and writes it to the file: NO NETWORK. The base comes from the head this session already read, else it is learned later (`settle`). A head with NO revision yet
   * is a head, never "no claim": its seed is its (empty) text, so another writer's first revision is a conflict. */
  private async keep(doc: string, markdown: string, from: string | null, t: Tok): Promise<boolean> {
    await this.load();
    if (!(await this.live(t))) return false;
    const s = this.session(doc);
    const k = this.keptByDoc[doc];
    let base = k ? k.base : '', seed: string | null = k ? k.seed : from;
    if (!k && s.head) {
      const fromHead = from === null || from === s.head.content;
      base = fromHead ? s.head.id : '';
      seed = fromHead ? (s.head.id === '' ? s.head.content : null) : from;
    }
    this.keptByDoc[doc] = { text: markdown, base, seed, key: uuid() };
    if (!s.conflict) { s.status = 'dirty'; s.label = this.diskPending ? 'Editing (not yet saved on this phone: the key could not be read)' : 'Editing'; }
    this.bump();
    return true;
  }
  /** The listener typed: the text is now `markdown`, and the field showed `from` before this keystroke (null: the caller makes no claim, an empty string is an empty text).
   * THE KEYSTROKE IS ON DISK BEFORE IT WAITS FOR ANYTHING (`keep`, then the one writer), then the call enters the document's queue to settle the base against the head: a save in flight, a hung link or a kill
   * leaves the latest text on the phone. */
  async edit(address: string, markdown: string, from: string | null = null): Promise<Info> {
    const doc = editableDoc(address);
    if (doc === null) return this.info(address);
    const t = await this.adopt();
    if (!(await this.keep(doc, markdown, from, t))) return this.info(address);
    const onDisk = this.persist();
    this.queuedEdits.set(doc, (this.queuedEdits.get(doc) ?? 0) + 1);
    const left = (): void => { this.queuedEdits.set(doc, Math.max(0, (this.queuedEdits.get(doc) ?? 1) - 1)); };
    const settled = this.run(doc, t, async (t2) => { left(); await this.settle(doc, t2); return this.info(address); }, () => { left(); return this.info(address); });
    await onDisk;
    return settled;
  }
  /** For a writer that is not the listener (the notebook's topics): ONE call inside the queue that does nothing when the listener has unsent text, else keeps, settles, saves, and on a failure drops its own text
   * and only its own: text typed behind it is the listener's and stays. 'saved', 'busy' (the listener has text here), or 'failed'. */
  async writeAutomatic(address: string, markdown: string, from: string | null): Promise<'saved' | 'busy' | 'failed'> {
    const doc = editableDoc(address);
    if (doc === null) return 'failed';
    const t = await this.adopt();
    return this.run<'saved' | 'busy' | 'failed'>(doc, t, async (t1) => {
      await this.load();
      if (!(await this.live(t1))) return 'failed';
      if (this.keptByDoc[doc]) return 'busy';
      if (!(await this.keep(doc, markdown, from, t1))) return 'failed';
      await this.persist();
      await this.settle(doc, t1);
      await this.save(doc, t1);
      const s = this.session(doc);
      const left = this.keptByDoc[doc] as Kept | undefined;
      if (s.status === 'saved' && !left) return 'saved';
      if (left && left.text === markdown) { delete this.keptByDoc[doc]; s.head = null; s.status = 'idle'; s.label = ''; s.conflict = false; this.bump(); await this.persistIf(t1); }      // its own text goes; text typed behind it is not touched
      return 'failed';
    }, async () => 'failed');
  }
  /** Inside the queue: the base of the LATEST kept text is settled against the head (read once while there is none: the calls queued behind a failed read do not read again). */
  private async settle(doc: string, t: Tok): Promise<void> {
    await this.load();
    if (!(await this.live(t))) return;
    const s = this.session(doc);
    if (!s.head) { if (s.skipReads > 0) s.skipReads -= 1; else await this.readHead(doc, t); }
    if (!(await this.live(t))) return;
    const k = this.keptByDoc[doc];
    if (!k || !s.head) return;
    if (k.text === s.head.content) { delete this.keptByDoc[doc]; s.status = 'idle'; s.label = ''; s.conflict = false; this.bump(); await this.persistIf(t); return; }
    if (k.base === '' && (k.seed === null || k.seed === s.head.content)) { k.base = s.head.id; k.seed = s.head.id === '' ? s.head.content : null; await this.persistIf(t); }      // typed over the head that was read (or over no claim): it has its base now
  }

  /** Post what is waiting for this document now (Done, leaving the page, the idle timer). An empty address is every document that has kept text and no conflict (the retry, on any page, and at launch). */
  async flush(address: string): Promise<Info> {
    const t = await this.adopt();
    await this.load();
    if (address === '') {
      for (const doc of Object.keys(this.keptByDoc)) await this.run(doc, t, async (t1) => { if (this.keptByDoc[doc] && !this.session(doc).conflict) await this.save(doc, t1); }, async () => undefined);
      return this.info('');
    }
    const doc = editableDoc(address);
    if (doc === null) return this.info(address);
    return this.run(doc, t, async (t1) => { await this.save(doc, t1); return this.info(address); }, () => this.info(address));
  }

  /** The head wins: the kept text is dropped (the field is seeded from the head the next time it opens). */
  async discard(address: string): Promise<Info> {
    const doc = editableDoc(address);
    if (doc === null) return this.info(address);
    const t = await this.adopt();
    return this.run(doc, t, async (t1) => {
      await this.load();
      delete this.keptByDoc[doc];
      const s = this.session(doc); s.head = null; s.status = 'idle'; s.label = ''; s.conflict = false;
      this.bump(); await this.persistIf(t1);
      return this.info(address);
    }, () => this.info(address));
  }
  /** The listener's own text wins: it is re-based on the head as it is now and saved. */
  async overwrite(address: string): Promise<Info> {
    const doc = editableDoc(address);
    if (doc === null) return this.info(address);
    const t = await this.adopt();
    return this.run(doc, t, async (t1) => {
      await this.load();
      const k = this.keptByDoc[doc];
      if (!k) return this.info(address);
      const s = this.session(doc);
      try {
        const row = await this.docRow(doc);
        const h = await this.revision(row.id);
        if (!(await this.live(t1))) return this.info(address);
        s.head = h;
        k.base = h.id; k.seed = h.id === '' ? h.content : null; k.key = uuid(); s.conflict = false;
        this.bump(); await this.persistIf(t1);
      } catch (e) { if (!(await this.live(t1))) return this.info(address); s.status = isOffline(e) ? 'offline' : 'failed'; s.label = isOffline(e) ? 'Offline: kept on this phone, posts when it can' : `Not saved: ${saveSentence(e)}`; this.bump(); return this.info(address); }
      await this.save(doc, t1);
      return this.info(address);
    }, () => this.info(address));
  }

  /** Runs only inside `run`: nothing else touches this document while it does, so there is no "again": a keystroke typed meanwhile waits its turn behind it. */
  private async save(doc: string, t: Tok): Promise<void> {
    const s = this.session(doc);
    const k = this.keptByDoc[doc];
    if (!k || s.conflict) return;
    s.status = 'saving'; s.label = 'Saving'; this.bump();
    try {
      const row = await this.docRow(doc);
      if (!(await this.live(t))) return;
      if (row.id !== k.base) {                                       // another writer, or a text kept from an earlier launch over an older head, or a base never known
        const h = await this.revision(row.id);
        if (!(await this.live(t))) return;
        s.head = h;
        if (h.content === k.text) { if (this.keptByDoc[doc]?.text === k.text) delete this.keptByDoc[doc]; s.status = 'saved'; s.label = h.version > 0 ? `Saved · version ${h.version}` : 'Saved'; this.bump(); await this.persistIf(t); return; }
        if (k.base === '' && (k.seed === null || h.content === k.seed)) { k.base = row.id; await this.persistIf(t); }      // typed over the text the head still is (or over no claim): now it has a base
        else { this.conflict(s); return; }
      }
      if (!s.head || s.head.id !== k.base) s.head = await this.revision(k.base);          // a relaunch: the head this text descends from
      if (!(await this.live(t))) return;
      const merged = Body.merge(s.head.body, s.head.content, k.text);
      const payload: Record<string, unknown> = { content: k.text, body_doc: merged.body, idempotency_key: k.key };
      if (k.base !== '') payload.parent_revision_id = k.base;
      if (!(await this.live(t))) return;
      if (row.trajectory !== '') {                                  // as the web does: a document with a trajectory is saved against its lifecycle version
        const life = await this.json(`/api/texture/lifecycle/${segment(row.trajectory)}`);
        const tr = obj(life.body) && obj(life.body.trajectory) ? life.body.trajectory : {};
        if (tr.lifecycle_version !== undefined) payload.expected_lifecycle_version = tr.lifecycle_version;
      }
      if (!(await this.live(t))) return;                            // nothing is posted once a sign-out has begun or the account is another
      let r: Reply;
      try { r = await this.json(`/api/texture/documents/${segment(doc)}/revisions`, { method: 'POST', body: payload }); }
      catch (e) {
        if (e instanceof HTTPError && e.status === 409) {           // the head moved between the read and the post
          if (!(await this.live(t))) return;
          const row2 = await this.docRow(doc);
          const h = await this.revision(row2.id);
          if (!(await this.live(t))) return;
          s.head = h;
          if (h.content === k.text) { if (this.keptByDoc[doc]?.text === k.text) delete this.keptByDoc[doc]; s.status = 'saved'; s.label = h.version > 0 ? `Saved · version ${h.version}` : 'Saved'; this.bump(); await this.persistIf(t); return; }
          this.conflict(s); return;
        }
        throw e;
      }
      if (!(await this.live(t))) return;                                    // signed out meanwhile: nothing of it is kept
      let o = obj(r.body) ? r.body : {};
      if (obj(o.revision)) o = o.revision;
      const n = typeof o.version_number === 'number' ? o.version_number : -1;
      const id = str(o.revision_id);
      s.head = { id, content: k.text, body: merged.body, version: n + 1 };
      const now = this.keptByDoc[doc];
      if (now && now.text !== k.text) { now.base = id; now.seed = null; }       // typed while it was posting: the newer text is kept over the new head and saved next
      else delete this.keptByDoc[doc];
      s.status = 'saved'; s.label = n >= 0 ? `Saved · version ${n + 1}` : 'Saved'; this.bump();
      await this.persistIf(t);
    } catch (e) {
      if (!(await this.live(t))) return;
      if (isLetGo(e)) { s.status = 'dirty'; s.label = 'Editing'; this.bump(); return; }      // a newer flush of the same mutation does this work: nothing is wrong
      s.status = isOffline(e) ? 'offline' : 'failed';
      s.label = isOffline(e) ? 'Offline: kept on this phone, posts when it can' : `Not saved: ${saveSentence(e)}`;
      this.bump();
    }
  }
  private conflict(s: Session): void { s.status = 'conflict'; s.label = CONFLICT_WORDS; s.conflict = true; this.bump(); }

  /** Sign-out or an account change: the sessions and the kept text, in memory and on disk. The epoch moves FIRST, so every call queued or in flight is stale and does nothing more. */
  async wipe(): Promise<void> {
    this.epoch += 1; this.loading = undefined; this.chain = new Map(); this.owner = null;
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
export const textureEdit = (address: string, markdown: string, from: string, storage: Storage | undefined): Promise<Info> => editor(storage).edit(address, markdown, from);
export const textureFlush = (address: string, storage: Storage | undefined): Promise<Info> => editor(storage).flush(address);
export const textureDiscard = (address: string, storage: Storage | undefined): Promise<Info> => editor(storage).discard(address);
export const textureOverwrite = (address: string, storage: Storage | undefined): Promise<Info> => editor(storage).overwrite(address);
/** Sign-out (account.ts WIPES): memory and the kept file. */
export async function wipeTextureEdit(storage: Storage | undefined): Promise<void> { const e = editor(storage); await e.wipe(); }
/** For tests: a fresh process. */
export function forgetTextureEdit(): void { live = undefined; }
