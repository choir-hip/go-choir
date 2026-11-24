// Editing your own Texture documents on the phone, in shared code. It was Swift (TextureEditor.swift); the body merge is texture-body.ts. The editor is Exact's `textarea markup="markdown"` over the
// document's Markdown (members-content.contract); this file is everything behind it.
//
// THIS COMMIT IS THE STATE MODEL ONLY. The code and the tests are derived from the table below and land after it (docs/problems/texture-editor-in-exact-three-reviews-no-landing-2026-11-20.md: an earlier
// port was sent back by review four times for patching a design that had no written model; the lane's rule 7 says change the approach, so the approach is written here first).
//
// =====================================================================================================================================================================================
// THREE DECISIONS (they replace the per-call tokens, the per-await identity checks and the assigned statuses of the earlier attempts)
// =====================================================================================================================================================================================
//
// (a) IDENTITY IS THE INSTANCE'S LIFETIME. One `Editor` exists per signed-in account and is constructed only when ONE stable account id is known: the computer id (api.ts `accountId`, a digest of it). There is no
//     fallback to a key digest: while the id is not known the module answers a read-only facade (`writable: false`, a sentence that says why) and nothing is kept. Sign-out or an account change KILLS the instance
//     (`kill()`: one `dead` flag, set synchronously when a sign-out BEGINS, by a hook api.ts `beginSignOut` runs, and when the id read at a public call differs from the instance's) and a new instance is built
//     later. The flag is read at the instance's three exits and nowhere else: the door (`run`), the network (`json`) and the file writer (`writeFile`). There is no token, no `live(t)`, no adopt/wipe on a changed
//     representation of the same account. An unreadable key is not another account: the instance goes on, and what it keeps is on disk under its own account's name.
//
// (b) EVERY MUTATION GOES THROUGH `run(doc)`, A KEYSTROKE INCLUDED. `kept`, `head`, `flags` of a document are touched only inside `run(doc, op)`: one at a time per document, in arrival order, `dead` checked at the door.
//     Durability of typing is a separate, tiny thing: the JOURNAL, a map document -> {text, from, key} (the latest typed text per document), with its own SINGLE WRITER to ONE file per account
//     (`app:/data/texture-pending/<account>.json`). A keystroke writes the journal and asks the writer to write (synchronously, before it waits for anything), then enters the queue to be applied. The journal is read
//     only at load. An entry is deleted only (1) when the server acknowledges THAT EXACT TEXT (compared by text, never by sequence: a newer keystroke typed while the post was out keeps its entry), or (2) when the
//     person discards and the journal's text is still the text that was discarded (no newer keystroke), or (3) when the automatic writer drops its own text and it is still its own.
//
// (c) STATUS IS DERIVED, NEVER ASSIGNED. A document's facts are: `journal` (is there unsent text), `inFlight` (a POST is out), `conflictAt` (the head id the server refused over, or none), `last` (the result of the last
//     attempt: none | offline | failed(sentence) | saved(version)). The status the page sees is computed from them and from nothing else:
//         conflict  if conflictAt is set
//         saving    else if inFlight
//         offline   else if journal and last is offline          failed    else if journal and last is failed
//         dirty     else if journal
//         saved     else if last is saved                         idle      otherwise
//     So "Saved" cannot be shown over text the server does not hold, and a keystroke during a save needs no status code at all: the journal is simply there when the post ends.
//
// =====================================================================================================================================================================================
// FACTS (per document; all in memory except the journal entry, which is also on disk)
// =====================================================================================================================================================================================
//   entry    {text, from, base, key} | none     THE JOURNAL ENTRY: stated here and nowhere else. `text` the latest typed text. `base` the id of the last revision the SERVER acknowledged underneath this
//                                              unsent chain ('' = not known). `from` what the field showed before the keystroke that made `text`, EXCEPT while `base` is '': then `from` is what the field showed before the
//                                              FIRST keystroke of the chain (null = the caller makes no claim; '' is an EMPTY text). `key` the idempotency key of `text`. A keystroke over unsent text INHERITS the base of the text
//                                              it replaces (and, once `base` is known, takes `from` = the text it replaced); a keystroke over nothing takes base = head.id when the field showed the head's text or no claim, else ''.
//   head     {id, content, body, version} | none   as last read or last acknowledged; id '' = a document with NO revision yet (that is a head, with empty content)
//   inFlight bool        conflictAt string | none      last none | offline | failed(sentence) | saved(version)
//   kept     the entry as run knows it: the same fields, held in memory inside run; the file holds the journal map {doc: entry}.
//
// THE FLUSH RULE (one place, used on every launch and every retry): read the head h; then
//     h.id == entry.base            -> post entry.text with parent h.id                       (the post that may or may not have landed did not: the base is still the head)
//     h.content == entry.text       -> acknowledged: delete the entry                           (a post that landed, or the same text written elsewhere)
//     h.content == entry.from or entry.from is null -> rebase: base := h.id, post              (the head holds exactly what this text was typed over: a landed earlier post of the person's own)
//     otherwise                     -> conflict: conflictAt = h.id, nothing is posted           (the head is somebody else's)
//
// =====================================================================================================================================================================================
//
// =====================================================================================================================================================================================
// THE TABLE: state x event -> transition, what is on disk, what the person sees.   S = the derived status. Every row is a test (texture-edit.test.ts, id in the left column).
// "disk" = the journal file after the transition. "sees" = Info (status / label / kept / hasKept / notice).
// =====================================================================================================================================================================================
//
// Events:  OPEN   KEY(t, f) (a keystroke: text t; the field showed f before it)   FLUSH (Done, the idle pause, leaving the page, the retry)   DISCARD ("Use theirs")   OVERWRITE ("Keep mine")
//          AUTO(t) (the notebook's writer)   NET-OK / NET-OFFLINE / NET-REFUSED / NET-409 / NET-LETGO (what the network answers to the step in progress)   SIGNOUT (begins)   ACCOUNT (another id read)
//          BACKGROUND (the app goes to the background)   KILL (the process dies)   LAUNCH
//
//  id    state (S)          event                          transition                                                                                         disk (entry)               the person sees
//  ----  -----------------  -----------------------------  -------------------------------------------------------------------------------------------------  -------------------------  ----------------------------------------------------------
//  T01   idle               OPEN                           read head (one door op); head = it; no entry                                                       none                       idle; opened=true; field seeded from the head
//  T02   idle               OPEN, NET-OFFLINE              last = offline; head none                                                                           none                       idle; opened=false; the field keeps what the page shows
//  T03   idle/saved         KEY(t,f), head known           entry {t, from f, base head.id if f==head.content or f null, else '' (then from = f), key}         entry t                    dirty; hasKept; kept=t
//  T04   idle               KEY(t,f), head not read yet    entry {t, f, base '', key} at once; the apply op reads the head once and settles base as T03       entry t (before the read)  dirty (before any network answer)
//  T05   idle               KEY(t,f), t == head.content    entry made, then apply finds t == head: entry deleted (text compared)                              none                       idle
//  T06   idle               KEY(t,f), head.id == ''        a head with NO revision is a head: base '' (== head.id), from = its content ''                      entry t                    dirty
//  T07   dirty              KEY(t2,f2)                     entry {t2, base INHERITED, from f2 (the text replaced) if base known else the chain's first from, key2}  entry t2          dirty; kept=t2
//  T08   dirty              FLUSH, NET-OK                  flush rule: post parent=base (+ lifecycle version); inFlight; head = the posted revision; entry deleted iff its text == posted  none   saving, then saved(version n)
//  T09   dirty              FLUSH, base '', head==from     rebase: base := head.id, then as T08                                                                entry t (base rewritten)   saving, then saved
//  T10   dirty              FLUSH, head is somebody else's flush rule "otherwise": conflictAt = h.id; nothing posted                                              entry t                    conflict ("changed somewhere else; your text is kept")
//  T11   dirty              FLUSH, NET-409                 read head again; the flush rule decides: acknowledged, or conflict                                  entry t / none             conflict, or saved when the head already holds t
//  T12   dirty              FLUSH, NET-OFFLINE             last = offline                                                                                      entry t                    offline; kept=t; "kept on this phone, posts when it can"
//  T13   dirty              FLUSH, NET-REFUSED (400/403)   last = failed(sentence)                                                                              entry t                    failed; sentence; kept=t
//  T14   dirty              FLUSH, NET-LETGO               nothing changes: a newer answer of the same mutation does the work                                  entry t                    dirty (never "offline")
//  T15   dirty              FLUSH, signed out (401)        last = failed("Sign in again to save")                                                              entry t                    failed; kept=t
//  T16   saving             KEY(t2,f2) while the POST is out  entry {t2, base inherited, from f2}: on disk NOW; the apply waits behind the post in the queue                                    entry t2                   saving; after NET-OK for t: entry t2 STAYS (T40) and the page is dirty, never saved
//  T17   saving             DISCARD queued behind the post runs after the post (door order); see T25                                                             see T25                    see T25
//  T18   saving             FLUSH again                    queued behind the post; nothing to send when the entry is gone                                      none                       saved
//  T19   offline/failed     FLUSH (retry)                  as T08..T13                                                                                          entry t / none             as T08..T13
//  T20   offline/failed     KEY(t2,f2)                     as T07; last unchanged until the next attempt                                                       entry t2                   offline/failed; kept=t2
//  T21   conflict           KEY(t2,f2)                     entry t2 (base/from inherited); conflictAt stays                                                    entry t2                   conflict (the listener has not chosen)
//  T22   conflict           FLUSH                          nothing is sent; no retry; `waiting` does not count it                                              entry t                    conflict
//  T23   conflict           OVERWRITE                      read head h; base := h.id, from := h.content; conflictAt none; then FLUSH as T08                    entry t, then none         saving, then saved
//  T24   conflict           DISCARD                        entry deleted iff its text == the kept text discarded; head := none (re-read at the next open); conflictAt none  none (if no newer key)  idle; the field is seeded from the head at the next open
//  T25   any with entry     DISCARD, a newer KEY(t2) typed after the discard was asked but before it ran    the entry (text t2) is NOT deleted; the discard drops the older kept text only; the apply of t2 follows in the queue   entry t2   dirty; kept=t2
//  T26   idle               AUTO(t), no entry              in ONE run op: entry = {t, from, base, key}; settle; save; saved: entry deleted iff still t          entry t, then none         saved
//  T27   dirty/any entry    AUTO(t)                        returns 'busy'; nothing replaced, nothing written                                                   unchanged                  unchanged
//  T28   idle               AUTO(t), save fails            its own entry deleted iff its text is still t; a listener's text typed behind it is untouched        none or the listener's t2  idle, or the listener's dirty
//  T29   any                SIGNOUT begins                 kill(): dead = true now; queued ops answer an empty Info; the network exit refuses; the writer writes nothing more. The account wipe (after the native sign-out) awaits the writer's last write, then DELETES THE WHOLE texture-pending FOLDER (every account's file, the .bad copy): nothing of one account is left for another (owner: nothing carried over)   folder empty   nothing of it (the page asks a new instance)
//  T30   any                ACCOUNT (another id read at a call, no sign-out)   cannot happen in this app; if it does it is treated as T29: kill the old instance, wipe the folder, build the new instance on an empty folder   folder empty, then the new account's   the new account's own state, empty
//  T31   (module)           no account id yet              read-only facade: writable=false, a sentence; KEY returns it and keeps nothing                      none                       "The editor needs your account: ..."; no Edit
//  T32   (module)           the id becomes known           an instance is built; it loads its file                                                             its file                   the kept texts of this account
//  T33   any                NET answers after kill()       the op finishes without effect: no write, no POST, no state read by anyone                          unchanged                  nothing
//  T34   (load)             LAUNCH, file of this account   journal = the entries (text, from, base, key as written); kept built lazily at the first run op of each document   the entries   "Edits kept on this phone" (offline) until an attempt; hasKept; waiting counts it
//  T35   (load)             LAUNCH, file unreadable (an I/O error, not "no file")  the file is left alone; the instance retries the read at the next call; nothing is written over it   untouched   nothing kept shown yet; no loss; the retry finds it
//  T36   (load)             LAUNCH, corrupt content        the file is renamed to texture-pending/<account>.bad (ONE .bad at most: an older .bad is replaced, never accumulated; the account wipe deletes it too); journal empty; Info.notice = "Unsent text could not be read and was set aside."   .bad   the notice sentence (never silence); no kept text
//  T37   two documents      KEY(a), KEY(b) together        one writer writes the journal map WHEN IT RUNS (never a stale snapshot): both entries land           both entries              both dirty
//  T38   any                the account id appears later / the key rotates, same account   the id is the computer id: the same instance goes on; the text is kept                          unchanged                  unchanged
//  T39   any                a hung network (no answer)     `json` ends in the editor's own timeout (20 s per call): last = offline; the entry is already on disk   entry t                  offline
//  T40   saving/any         NET-OK for text t while the entry holds t2 != t (typed meanwhile)    head := the posted revision; entry t2 REWRITTEN with base := the posted revision id, from := t; last = saved; status is dirty (derived: an entry exists)   entry t2 (new base)   dirty
//  T41   idle               OPEN, a head that moved since the last open    head := it (a re-read); an entry is untouched; a conflict is NOT assumed here                                       unchanged                  unchanged
//  T42   idle               KEY(t,f), f != head.content (the field showed an older reading)   entry {t, from f, base '', key}; FLUSH: the flush rule: head.content == f -> rebase; the head moved -> conflict; nothing is overwritten   entry t   dirty, then conflict if the head moved
//  T43   saving             BACKGROUND (iOS may suspend the request)   the request ends as NET-OFFLINE or NET-LETGO, never as a lost entry: the entry was on disk before the post began and stays until acknowledged   entry t   offline or dirty; never idle with the text gone
//
// KILL (the process dies) AT EACH POINT, and what a relaunch shows (each is a test: it builds a second instance on the same files, which is a LAUNCH):
//  K1  after KEY(t1) landed on disk and KEY(t2) is typed: the write of t2 had not landed   the file holds t1 (the previous journal): AT MOST THE LAST KEYSTROKE IS LOST, never an earlier one; the bound is one write long
//  K2  after the journal write landed, before the head was read    the file has t; relaunch: dirty, hasKept, kept = t; FLUSH posts it (base '' + from: the flush rule)
//  K3  during the POST of t (it may or may not have landed)        the file has {t, base h0}; relaunch FLUSH: the head is h0 -> post again with the SAME idempotency key (no duplicate); the head holds t -> acknowledged (entry deleted)
//  K4a t was posting, t2 typed over it, the POST LANDED, killed    the file has {t2, base h0, from t}; relaunch: head = r1 holding t: h.id != base, h.content == from -> REBASE (base := r1) and post t2: no conflict
//  K4b t was posting, t2 typed over it, the POST DID NOT LAND, killed   the file has {t2, base h0, from t}; relaunch: head = h0: h.id == base -> post t2 on h0: no conflict against the person's own unsent text
//  K5  after NET-OK, before the entry was deleted                  as K3: the head holds t -> acknowledged at the next flush
//  K6  with a conflict                                             the file has t; relaunch: dirty (the conflict is the server's fact and is found again at the next FLUSH by the flush rule)
//  K7  with the key unreadable at relaunch                         the file is untouched; the instance is built when the id can be read (T35, T31)
//  K8  with two documents                                          both entries come back
//
// WHAT IS NOT A STATE: "again", "pending save", "loaded", a token, a generation, an epoch, an owner string compared at an await.
//
// Sentences the page shows come from this file (`LABELS`), one per derived status; the contract shows them and decides nothing.

// ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------
import type { Storage } from './app.contract.d.ts';
import { authedJSON, accountId, onSignOutBegin, sealingBlocked, segment, SignedOutError, HTTPError, type Reply } from './api.ts';
import * as Body from './texture-body.ts';

export const PENDING_DIR = 'app:/data/texture-pending';
/** One network call of the editor ends in this: a link that answers nothing must not hold a document's queue. The unsent text is on disk before any call. */
export const NET_TIMEOUT_MS = 20_000;
export type Net = { json(path: string, opts?: { method?: 'GET' | 'POST'; body?: unknown }): Promise<Reply> };
export type Info = { status: string; label: string; notice: string; address: string; editable: boolean; writable: boolean; kept: string; hasKept: boolean; waiting: number; stamp: number; head: string; opened: boolean };
export type Deps = { net: Net; storage: Storage | undefined; account: string; timeoutMs?: number };
type Entry = { text: string; from: string | null; base: string; key: string };
type Head = { id: string; content: string; body: Body.Body | null; version: number };
type Last = { kind: 'none' } | { kind: 'offline'; label: string } | { kind: 'failed'; label: string } | { kind: 'saved'; version: number };

const obj = (v: unknown): v is Record<string, unknown> => typeof v === 'object' && v !== null && !Array.isArray(v);
const str = (v: unknown): string => (typeof v === 'string' ? v : '');
const uuid = (): string => { const b = crypto.getRandomValues(new Uint8Array(16)); b[6] = (b[6] & 0x0f) | 0x40; b[8] = (b[8] & 0x3f) | 0x80; const h = Array.from(b, (x) => x.toString(16).padStart(2, '0')).join(''); return `${h.slice(0, 8)}-${h.slice(8, 12)}-${h.slice(12, 16)}-${h.slice(16, 20)}-${h.slice(20)}`; };
/** The document id of an address the listener may edit (the head of a Texture document), else null. A revision (`texture:<doc>@<rev>`) and a story (`pub:`) are read-only. */
export function editableDoc(address: string): string | null {
  if (!address.startsWith('texture:')) return null;
  const rest = address.slice('texture:'.length);
  return rest === '' || rest.includes('@') ? null : rest;
}
const isLetGo = (e: unknown): boolean => e instanceof Error && /let go/i.test(e.message);
const isOffline = (e: unknown): boolean => (e instanceof HTTPError ? e.status >= 500 || e.status === 404 || e.status === 429 : !(e instanceof SignedOutError));
const sentence = (e: unknown): string => (e instanceof SignedOutError ? 'Sign in again to save.' : e instanceof HTTPError ? (e.status === 403 ? 'This device is not allowed to save here.' : `Could not save. ${e.message}`) : 'Something went wrong saving the document.');
/** An error that says "there is no such file" (the kept file is not there yet), against one that says it could not be read. */
const isAbsent = (e: unknown): boolean => { const c = (e as { code?: unknown } | null)?.code; return c === 'ENOENT' || /no such file|not found|does not exist|ENOENT/i.test(e instanceof Error ? e.message : ''); };

export const LABELS = {
  conflict: 'Not saved: this document changed somewhere else, and your changes could not be combined with it. Your text is kept on this phone.',
  saving: 'Saving', dirty: 'Editing', offline: 'Offline: kept on this phone, posts when it can', launch: 'Edits kept on this phone',
  readOnly: 'The editor needs your account to be known before it keeps your text. Try again in a moment.',
  bad: 'Unsent text could not be read and was set aside.',
} as const;

export class Editor {
  dead = false;
  private net: Net; private storage: Storage | undefined; private account: string; private timeoutMs: number;
  private journal: Record<string, Entry> = {};               // the entries: the file's content, one per document (a keystroke writes it; a run op only rewrites base/from and deletes by text)
  private heads = new Map<string, Head>(); private inFlight = new Set<string>(); private conflictAt = new Map<string, string>(); private last = new Map<string, Last>();
  private chain = new Map<string, Promise<unknown>>(); private stamp = 0; private notice = '';
  private loaded = false; private loading: Promise<void> | undefined; private diskPending = false; private pDirty = false; private pRun: Promise<void> | undefined;
  constructor(d: Deps) { this.net = d.net; this.storage = d.storage; this.account = d.account; this.timeoutMs = d.timeoutMs ?? NET_TIMEOUT_MS; }
  private file(): string { return `${PENDING_DIR}/${this.account}.json`; }
  private bump(): void { this.stamp += 1; }
  /** Sign-out began, or another account: this instance does nothing more. One flag, read at the door (`run`), the network (`json`) and the file writer (`writeFile`). */
  kill(): void { this.dead = true; }
  /** For the wipe: waits for a write that is in flight to end (the writer starts no new one once dead). */
  async settleWrites(): Promise<void> { while (this.pRun) await this.pRun; }

  // ----------------------------------------------------------------------------------------------------------------- the door
  private run<T>(doc: string, fn: () => Promise<T>, empty: () => Promise<T>): Promise<T> {
    const go = async (): Promise<T> => (this.dead ? empty() : fn());
    const p = (this.chain.get(doc) ?? Promise.resolve()).then(go, go);
    this.chain.set(doc, p.then(() => undefined, () => undefined));
    return p;
  }
  // ----------------------------------------------------------------------------------------------------------------- the network exit
  private async json(path: string, opts?: { method?: 'GET' | 'POST'; body?: unknown }): Promise<Reply> {
    if (this.dead) throw new Error('the editor was closed');
    let timer: ReturnType<typeof setTimeout> | undefined;
    const late = new Promise<never>((_, rej) => { timer = setTimeout(() => rej(new Error('timeout: no answer from the computer')), this.timeoutMs); });
    try { const r = await Promise.race([this.net.json(path, opts), late]); if (this.dead) throw new Error('the editor was closed'); return r; } finally { clearTimeout(timer); }
  }
  // ----------------------------------------------------------------------------------------------------------------- the file: read once, one writer
  private ready(): Promise<void> {
    if (this.loaded) return Promise.resolve();
    return (this.loading ??= this.load().finally(() => { this.loading = undefined; }));
  }
  private async load(): Promise<void> {
    if (!this.storage) { this.loaded = true; return; }
    let bytes: Uint8Array;
    try { bytes = new Uint8Array(await this.storage.fs.readFile(this.file())); }
    catch (e) { if (isAbsent(e)) { this.loaded = true; return; } return; }                    // an I/O error is not "no file": left alone, asked again at the next call, nothing written over it
    let docs: Record<string, Entry> = {};
    try {
      const o = JSON.parse(new TextDecoder().decode(bytes));
      if (!obj(o) || !obj(o.docs)) throw new Error('shape');
      for (const [d, v] of Object.entries(o.docs)) if (obj(v) && typeof v.text === 'string' && typeof v.base === 'string') docs[d] = { text: v.text, from: typeof v.from === 'string' ? v.from : null, base: v.base, key: str(v.key) || uuid() };
    } catch {
      docs = {};                                                                              // corrupt: set aside as the ONE .bad (replaced), and say so
      try { await this.storage.fs.atomicWriteFile(`${PENDING_DIR}/${this.account}.bad`, bytes); await this.storage.fs.rm(this.file()); } catch { /* the notice is still given */ }
      this.notice = LABELS.bad;
    }
    for (const [d, e] of Object.entries(docs)) if (!this.journal[d]) { this.journal[d] = e; this.last.set(d, { kind: 'offline', label: LABELS.launch }); }
    this.loaded = true; this.bump();
    if (this.diskPending) await this.persist();
  }
  private persist(): Promise<void> {
    this.pDirty = true;
    if (!this.pRun) this.pRun = (async () => { try { while (this.pDirty) { this.pDirty = false; await this.writeFile(); } } finally { this.pRun = undefined; } })();
    return this.pRun;
  }
  private async writeFile(): Promise<void> {
    if (this.dead || !this.storage) return;
    if (!this.loaded) { this.diskPending = true; return; }                                    // the file that is there has not been read: not written over
    this.diskPending = false;
    try {
      if (Object.keys(this.journal).length === 0) { try { await this.storage.fs.rm(this.file()); } catch { /* none */ } return; }
      try { await this.storage.fs.mkdir(PENDING_DIR); } catch { /* there */ }
      await this.storage.fs.atomicWriteFile(this.file(), new TextEncoder().encode(JSON.stringify({ account: this.account, docs: this.journal })));
    } catch { this.diskPending = true; }
  }
  // ----------------------------------------------------------------------------------------------------------------- what the page sees: derived, never assigned
  private statusOf(doc: string): { status: string; label: string } {
    const e = this.journal[doc], l = this.last.get(doc) ?? { kind: 'none' as const };
    if (this.conflictAt.has(doc)) return { status: 'conflict', label: LABELS.conflict };
    if (this.inFlight.has(doc)) return { status: 'saving', label: LABELS.saving };
    if (e) {
      if (l.kind === 'offline') return { status: 'offline', label: l.label };
      if (l.kind === 'failed') return { status: 'failed', label: l.label };
      return { status: 'dirty', label: this.diskPending ? 'Editing (not yet saved on this phone)' : LABELS.dirty };
    }
    if (l.kind === 'saved') return { status: 'saved', label: l.version > 0 ? `Saved · version ${l.version}` : 'Saved' };
    return { status: 'idle', label: '' };
  }
  async info(address: string): Promise<Info> {
    await this.ready();
    const doc = editableDoc(address);
    const waiting = Object.keys(this.journal).filter((d) => !this.conflictAt.has(d)).length;      // a conflict waits for the listener, not for a retry
    const base = { notice: this.notice, address, writable: true, waiting, stamp: this.stamp };
    if (doc === null) return { ...base, status: 'idle', label: '', editable: false, kept: '', hasKept: false, head: '', opened: false };
    const e = this.journal[doc], h = this.heads.get(doc), st = this.statusOf(doc);
    return { ...base, ...st, editable: true, kept: e?.text ?? '', hasKept: !!e, head: h?.content ?? '', opened: !!h };
  }
  // ----------------------------------------------------------------------------------------------------------------- reads (inside run)
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
  /** Reads the head into the facts. False when it could not (last says why, unless the answer was let go). */
  private async readHead(doc: string): Promise<{ row: { id: string; trajectory: string }; head: Head } | null> {
    try { const row = await this.docRow(doc); const head = await this.revision(row.id); this.heads.set(doc, head); return { row, head }; }
    catch (e) { this.fail(doc, e); return null; }
  }
  private fail(doc: string, e: unknown): void {
    if (this.dead || isLetGo(e)) return;                                                         // a newer answer of the same mutation does the work
    this.last.set(doc, isOffline(e) ? { kind: 'offline', label: LABELS.offline } : { kind: 'failed', label: `Not saved: ${sentence(e)}` }); this.bump();
  }
  /** The entry's text is on the server: the entry goes (compared by text: a newer keystroke keeps its own). */
  private acknowledged(doc: string, text: string, version: number): void {
    const e = this.journal[doc];
    if (e && e.text === text) delete this.journal[doc];
    this.last.set(doc, { kind: 'saved', version }); this.conflictAt.delete(doc); this.bump();
  }

  // ----------------------------------------------------------------------------------------------------------------- the events
  /** The head is read, and the field can be seeded from it. Offline: `opened` is false and the field keeps what the page shows. */
  async open(address: string): Promise<Info> {
    const doc = editableDoc(address);
    if (doc === null) return this.info(address);
    await this.ready();
    return this.run(doc, async () => { await this.readHead(doc); return this.info(address); }, () => this.info(address));
  }
  /** KEY: the entry is made and written FIRST (no network, no queue); then the apply enters the document's queue. `from` is what the field showed before this keystroke (null: no claim; '' is an empty text). */
  async edit(address: string, text: string, from: string | null): Promise<Info> {
    const doc = editableDoc(address);
    if (doc === null || this.dead) return this.info(address);
    await this.ready();
    if (this.dead) return this.info(address);
    const cur = this.journal[doc], head = this.heads.get(doc);
    let base = '', f = from;
    if (cur) { base = cur.base; f = cur.base !== '' ? from : cur.from; }                       // a keystroke over unsent text INHERITS the base of the text it replaces
    else if (head) { base = from === null || from === head.content ? head.id : ''; if (head.id === '') f = head.content; }
    this.journal[doc] = { text, from: f, base, key: uuid() };
    this.bump();
    const onDisk = this.persist();
    const applied = this.run(doc, async () => { await this.settle(doc); return this.info(address); }, () => this.info(address));
    await onDisk;
    return applied;
  }
  /** Inside the queue: the head of the document is known (read once while there is none), and the entry's base is settled against it. */
  private async settle(doc: string): Promise<void> {
    await this.ready();
    if (!this.heads.has(doc) && !(await this.readHead(doc))) return;
    const e = this.journal[doc], h = this.heads.get(doc);
    if (!e || !h) return;
    if (e.text === h.content) { delete this.journal[doc]; this.bump(); await this.persist(); return; }      // the same text as the head: nothing unsent
    if (e.base === '' && h.id !== '' && (e.from === null || e.from === h.content)) { e.base = h.id; await this.persist(); }
  }
  /** Post what is waiting now (Done, the idle pause, leaving the page, the retry: '' = every document with unsent text and no conflict). */
  async flush(address: string): Promise<Info> {
    await this.ready();
    if (address === '') {
      for (const doc of Object.keys(this.journal)) await this.run(doc, async () => { if (!this.conflictAt.has(doc)) await this.save(doc); }, async () => undefined);
      return this.info('');
    }
    const doc = editableDoc(address);
    if (doc === null) return this.info(address);
    return this.run(doc, async () => { await this.save(doc); return this.info(address); }, () => this.info(address));
  }
  /** "Use theirs": the head wins. The entry goes only if its text is still the text that was discarded (a newer keystroke is the listener's and stays). */
  async discard(address: string): Promise<Info> {
    const doc = editableDoc(address);
    if (doc === null) return this.info(address);
    await this.ready();
    const asked = this.journal[doc]?.text;
    return this.run(doc, async () => {
      if (asked !== undefined && this.journal[doc]?.text === asked) delete this.journal[doc];
      this.conflictAt.delete(doc); this.heads.delete(doc); this.last.delete(doc); this.bump(); await this.persist();
      return this.info(address);
    }, () => this.info(address));
  }
  /** "Keep mine": re-based on the head as it is now, then saved. */
  async overwrite(address: string): Promise<Info> {
    const doc = editableDoc(address);
    if (doc === null) return this.info(address);
    await this.ready();
    return this.run(doc, async () => {
      const e = this.journal[doc];
      if (!e) return this.info(address);
      const got = await this.readHead(doc);
      if (!got) return this.info(address);
      e.base = got.head.id; e.from = got.head.content; e.key = uuid(); this.conflictAt.delete(doc); this.bump(); await this.persist();
      await this.save(doc);
      return this.info(address);
    }, () => this.info(address));
  }
  /** For a writer that is not the listener (the notebook's topics): ONE op inside the queue, reading and writing there. 'busy' when the listener has text here; on a failure it drops its own text, if still its own. */
  async writeAutomatic(address: string, text: string, from: string | null): Promise<'saved' | 'busy' | 'failed'> {
    const doc = editableDoc(address);
    if (doc === null) return 'failed';
    await this.ready();
    return this.run<'saved' | 'busy' | 'failed'>(doc, async () => {
      if (this.journal[doc]) return 'busy';
      this.journal[doc] = { text, from, base: '', key: uuid() }; this.bump();
      await this.persist();
      await this.settle(doc);
      await this.save(doc);
      const l = this.last.get(doc);
      if (!this.journal[doc] && l && l.kind === 'saved') return 'saved';
      if (this.journal[doc]?.text === text) { delete this.journal[doc]; this.bump(); await this.persist(); }
      return 'failed';
    }, async () => 'failed');
  }

  /** Inside the queue. THE FLUSH RULE (header): read the head, then post over it, acknowledge, rebase, or conflict. */
  private async save(doc: string): Promise<void> {
    const e0 = this.journal[doc];
    if (!e0 || this.conflictAt.has(doc)) return;
    try {
      const got = await this.readHead(doc);
      if (!got) return;
      const { row, head: h } = got;
      const e = this.journal[doc];
      if (!e) return;
      if (h.id !== e.base) {
        if (h.content === e.text) { this.acknowledged(doc, e.text, h.version); await this.persist(); return; }
        if ((e.base === '' && e.from === null) || h.content === e.from) { e.base = h.id; await this.persist(); }
        else { this.conflictAt.set(doc, h.id); this.bump(); return; }
      }
      const posted = e.text;
      const merged = Body.merge(h.body, h.content, posted);
      const payload: Record<string, unknown> = { content: posted, body_doc: merged.body, idempotency_key: e.key };
      if (h.id !== '') payload.parent_revision_id = h.id;
      if (row.trajectory !== '') {                                                            // as the web does: a document with a trajectory is saved against its lifecycle version
        const life = await this.json(`/api/texture/lifecycle/${segment(row.trajectory)}`);
        const t = obj(life.body) && obj(life.body.trajectory) ? life.body.trajectory : {};
        if (t.lifecycle_version !== undefined) payload.expected_lifecycle_version = t.lifecycle_version;
      }
      this.inFlight.add(doc); this.last.set(doc, { kind: 'none' }); this.bump();
      let r: Reply;
      try { r = await this.json(`/api/texture/documents/${segment(doc)}/revisions`, { method: 'POST', body: payload }); }
      catch (err) {
        this.inFlight.delete(doc);
        if (err instanceof HTTPError && err.status === 409) {                                 // the head moved between the read and the post
          const again = await this.readHead(doc);
          if (!again) return;
          if (again.head.content === posted) { this.acknowledged(doc, posted, again.head.version); await this.persist(); return; }
          this.conflictAt.set(doc, again.head.id); this.bump(); return;
        }
        throw err;
      }
      this.inFlight.delete(doc);
      let o = obj(r.body) ? r.body : {};
      if (obj(o.revision)) o = o.revision;
      const n = typeof o.version_number === 'number' ? o.version_number : -1;
      const id = str(o.revision_id);
      this.heads.set(doc, { id, content: posted, body: merged.body, version: n + 1 });
      const cur = this.journal[doc];
      if (cur && cur.text !== posted) { cur.base = id; cur.from = posted; }                   // typed while it was posting: its base is the revision just made
      this.acknowledged(doc, posted, n + 1);
      await this.persist();
    } catch (err) {
      this.inFlight.delete(doc);
      this.fail(doc, err);
    }
  }
}

// ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------
// THE MODULE: one instance per signed-in account, built only when the account's id is known.
const real: Net = { json: (path, opts) => authedJSON(path, opts) };
let live: { account: string; ed: Editor; storage: Storage | undefined } | null = null;
let hooked = false;
/** Registered at the first use, not at import (api.ts imports app.ts, which imports this: a call at load meets an api.ts not yet initialised). */
function hook(): void { if (!hooked) { hooked = true; onSignOutBegin(() => { if (live) { live.ed.kill(); live = null; } }); } }
const readOnly = (address: string): Info => ({ status: 'idle', label: LABELS.readOnly, notice: '', address, editable: editableDoc(address) !== null, writable: false, kept: '', hasKept: false, waiting: 0, stamp: 0, head: '', opened: false });
async function wipeFolder(storage: Storage | undefined): Promise<void> { if (storage) { try { await storage.fs.rm(PENDING_DIR); } catch { /* none */ } } }
/** The editor for the account signed in now, or null (read-only). The id is read once per call: another id than the instance's is treated as a sign-out (kill, wipe the folder, build anew); an id that cannot be read is
 * not another account, and the instance goes on. While a sign-out is under way no instance is built. */
async function current(storage: Storage | undefined): Promise<Editor | null> {
  hook();
  if (sealingBlocked()) return null;
  const id = await accountId();
  if (id === null) return live?.ed ?? null;
  if (live && live.account !== id) { const old = live.ed; old.kill(); live = null; await old.settleWrites(); await wipeFolder(storage); }
  if (!live) live = { account: id, ed: new Editor({ net: real, storage, account: id }), storage };
  return live.ed;
}
export const textureEditInfo = async (a: string, s: Storage | undefined): Promise<Info> => { const e = await current(s); return e ? e.info(a) : readOnly(a); };
export const textureEditOpen = async (a: string, s: Storage | undefined): Promise<Info> => { const e = await current(s); return e ? e.open(a) : readOnly(a); };
export const textureEdit = async (a: string, t: string, from: string, s: Storage | undefined): Promise<Info> => { const e = await current(s); return e ? e.edit(a, t, from) : readOnly(a); };
export const textureFlush = async (a: string, s: Storage | undefined): Promise<Info> => { const e = await current(s); return e ? e.flush(a) : readOnly(a); };
export const textureDiscard = async (a: string, s: Storage | undefined): Promise<Info> => { const e = await current(s); return e ? e.discard(a) : readOnly(a); };
export const textureOverwrite = async (a: string, s: Storage | undefined): Promise<Info> => { const e = await current(s); return e ? e.overwrite(a) : readOnly(a); };
export const textureWriteAutomatic = async (a: string, t: string, from: string | null, s: Storage | undefined): Promise<'saved' | 'busy' | 'failed'> => { const e = await current(s); return e ? e.writeAutomatic(a, t, from) : 'failed'; };
/** Sign-out (account.ts): the instance dies, its last write ends, and the whole folder goes (every account's file and the .bad). */
export async function wipeTextureEdit(storage: Storage | undefined): Promise<void> {
  const old = live; live = null;
  if (old) { old.ed.kill(); await old.ed.settleWrites(); }
  await wipeFolder(storage);
}
/** For tests: a fresh process. */
export function forgetTextureEdit(): void { if (live) live.ed.kill(); live = null; }
