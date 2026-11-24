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
// THE STUB (first commit of the tests: every method fails, so the tests are seen failing before the code exists)
import type { Storage } from './app.contract.d.ts';
export const PENDING_DIR = 'app:/data/texture-pending';
export type Reply = { body: unknown; serverTime: number | null; raw: string };
export type Net = { json(path: string, opts?: { method?: 'GET' | 'POST'; body?: unknown }): Promise<Reply> };
export type Info = { status: string; label: string; notice: string; address: string; editable: boolean; writable: boolean; kept: string; hasKept: boolean; waiting: number; stamp: number; head: string; opened: boolean };
export type Deps = { net: Net; storage: Storage | undefined; account: string; timeoutMs?: number };
const nope = (): never => { throw new Error('not built'); };
export class Editor {
  dead = false;
  constructor(_deps: Deps) { void _deps; }
  kill(): void { nope(); }
  settleWrites(): Promise<void> { return nope(); }
  info(_a: string): Promise<Info> { return nope(); }
  open(_a: string): Promise<Info> { return nope(); }
  edit(_a: string, _t: string, _from: string | null): Promise<Info> { return nope(); }
  flush(_a: string): Promise<Info> { return nope(); }
  discard(_a: string): Promise<Info> { return nope(); }
  overwrite(_a: string): Promise<Info> { return nope(); }
  writeAutomatic(_a: string, _t: string, _from: string | null): Promise<'saved' | 'busy' | 'failed'> { return nope(); }
}
export const editableDoc = (a: string): string | null => { void a; return nope(); };
export const textureEditInfo = (_a: string, _s: Storage | undefined): Promise<Info> => nope();
export const textureEditOpen = (_a: string, _s: Storage | undefined): Promise<Info> => nope();
export const textureEdit = (_a: string, _t: string, _f: string, _s: Storage | undefined): Promise<Info> => nope();
export const textureFlush = (_a: string, _s: Storage | undefined): Promise<Info> => nope();
export const textureDiscard = (_a: string, _s: Storage | undefined): Promise<Info> => nope();
export const textureOverwrite = (_a: string, _s: Storage | undefined): Promise<Info> => nope();
export const wipeTextureEdit = (_s: Storage | undefined): Promise<void> => nope();
export const forgetTextureEdit = (): void => nope();
