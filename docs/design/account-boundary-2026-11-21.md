# The account boundary in shared code

For [the unclean sign-out defect](../problems/unclean-sign-out-leaves-the-old-accounts-data-for-the-next-2026-11-21.md),
2026-11-21. A design, not code; nothing here is built. Every landing is red.

## Summary

The phone must never show one account's private data to another. Today shared code
settles the account once, remembers the answer, and lets every page read its stores
whatever that answer was. Two attempts to fix it changed what is remembered. Both were
sent back by both review seats, because a page still read its store when the settle
had not finished cleanly: the gate fails open. On main and on `579ceb2db` alike,
`app.ts:135` awaits the settle and throws its result away.

This design stops remembering. The boundary is a fact that holds or does not hold, and
it is worked out again from two durable facts before every page answer that touches
account data:

- **the key**: the sign-in key in the Keychain, read through the app module (absent,
  present with its SHA-256, or unreadable);
- **the record**: one small file, `app:/data/account-stamp`, holding the SHA-256 of the
  key of the account whose data the phone's stores hold, or not there at all.

A page may read or write account data in exactly two cases: the record and the key name
the same account, or there is no record and the key is absent (the stores hold no
account's data). In every other case the page source refuses, the account card says why
and offers the way out on every page, and the one writer of the boundary works to put it
right: it wipes every store and then removes the record. Nothing about the boundary is
kept in memory between answers except the account generation, a number that only
drops work begun before a wipe.

What changes in practice: the marker file goes (a sign-out is "delete the key, then
settle"; an absent key under a record is the proof a wipe is owed, and late native
answers need no handling). Every page source is gated in `app.ts` except a short named
list. Any doubt refuses: a fact that cannot be read is never "nothing to protect".
Sign-in is offered only when the phone holds no account's data. Native stores take the
generation they are given with each wipe, so a late wipe never clears newer data.

The first landing is shared code and contracts only. It closes the cross-account read
on the shared side and makes more pages refuse while a wipe is owed. Native follows.

## 1. The frame

**What the boundary is.** It is not a procedure ("settle at launch") and not a memo.
It is a relation between two things with different writers: the key (written by the
Swift sign-in, deleted by the Swift sign-out) and the data (written by a dozen shared
and native stores). The relation holds when every byte of account data on the phone
belongs to the account whose key is stored, or to none. A remembered answer goes stale
the moment either writer acts, and the key's writer is native code that shared code
cannot see. So the relation is checked, not remembered. That includes the record: the
check reads the file each time and keeps no copy of it in memory.

**What can be derived need not be stored.** The second attempt wrote a marker and a
list of stores still to wipe. Each extra durable fact was a new way to be inconsistent
(an unknown name in the list looped forever; a failed marker removal was taken as
clean). The record plus the key already say everything: a record naming A with A's key
is open; a record naming A with no key or another key means A's data must go; no record
and no key means nothing to protect.

**The physical bounds.** Anything can fail: the Keychain delete (its status is ignored,
`ChoirClient.swift`), the native wipe (it hangs on the main actor, and its stores
swallow removal errors with `try?`, `AccountWipe.swift:71-72,98-101`), a shared file
removal, the record read or write, the key read (no deadline today, `api.ts:300-311`),
a crash at any instruction, a native answer after its deadline. The design does not
make these succeed. It makes each one leave a state that refuses, and makes every
refusing state heal by doing the same idempotent work again.

**One mechanism, several jobs.** The account generation (moved once by every sign-out
and every wipe attempt, and by nothing else) already drops replies that started under
an older account. It now also (a) ends a page answer that began before a wipe, (b) is
the floor every native store refuses below, carried to native code with the wipe, and
(c) lets native code ignore a wipe older than one it has already run.

## 2. The real states

**Reading the facts.** Exact's file store rejects with an `Error` whose `code` is the
same on every host: `'ENOENT'` for a missing file, otherwise `'EACCES'`, `'EBUSY'`,
`'EISDIR'`, `'denied'`, `'full'`, `'failed'` and the rest (Exact reference at the pin
`f53347503`, `docs/reference.md:2900-2910`: "Branch on the code, never the message"; the
cross-host conformance app `host/web-js/conformance/storage-background/app.ts:9-24`
reads `error.code` and expects `"ENOENT"`). So:

- the record is **none** only when the read rejects with `ENOENT`; **names d** when it is
  exactly 64 lowercase hex characters equal to the key's digest; **names another** when
  it is any other content (empty, a bare name, the second attempt's digest-plus-list);
  **unreadable** on any other rejection, on a read past the deadline, or when the answer
  has no storage at all;
- the key is **absent** only when the Keychain answered item-not-found; **present d**
  when it was read whole and hashed; **unreadable** when locked, on a bridge error, past
  the deadline, or when `keystate` says present but the session read then fails or finds
  nothing (`api.ts:307-309`);
- a host with **no native module** (Android, web) has no key at all.

The existing helpers that catch every error (`account.ts:99-105` `readText`, `exists`,
`removeFile`; `:143-155`) are replaced: a path exists unless it rejects `ENOENT`, and a
removal counts only when the stat afterwards rejects `ENOENT`.

| record \ key | absent | present, digest d | unreadable |
| --- | --- | --- | --- |
| none | EMPTY | ADOPT, then OPEN | LOCKED |
| names d | CLEARING | OPEN | LOCKED |
| names another | CLEARING | CLEARING | LOCKED |
| unreadable | LOCKED | LOCKED | LOCKED |

With no native module: record none is EMPTY; any record is LOCKED (no record is ever
written on such a host, so one there is not ours to judge). A native module that is
expected but missing at an answer is the key's "unreadable", never "absent": an absent
key leads to a wipe, so only the Keychain's own answer may say it.

- **OPEN**: the record names the stored key. Pages read and write.
- **EMPTY**: no record, no key. The stores hold no account's data, because the record is
  removed only after every wipe succeeded (section 4). Pages read and write; anything
  written now belongs to whoever holds the phone, not to an account.
- **ADOPT**: no record and a key. The writer writes the key's digest as the record.
  Until that write succeeds the state is closed.
- **CLEARING**: the record names an account whose key is not stored. Its data must go.
  Pages refuse. The writer wipes everything and then removes the record.
- **LOCKED**: a fact cannot be read. Pages refuse. Nothing is written or wiped.

One assumption carries the invariant "no record means no account's data": every
container that holds account data has a record. That is true for containers written by
main since the stamp was added (main adopts at the first settle) and by this design. A
container from before the stamp, or one where main's adopt write failed (main ignores
the result, `account.ts:173`), would be adopted with its data. The app is unlaunched
with one user whose phone has a stamp; a drill checks it (D1 reads the stamp first).

The second attempt's 36 rows were the wrong resolution: its marker and list were state
it had added and then had to keep consistent, and its "sign-out in flight" rows are not
states here (before the key delete the phone is OPEN, after it CLEARING).

## 3. What a page source may do, and what the person sees

**The gate**, in `app.ts` `answer`:

```
if (EXEMPT.has(source)) return sources[source](...);            // no boundary read, no wait
const b = await boundary.enter(storage, store, native);         // derives the state
if (!b.readable) throw new AccountClosedError(b.state);
const gen = accountGeneration();                                // after enter: a heal may have moved it
await clearDemoLeftovers(storage);                              // only behind an open boundary
const out = await sources[source](...);
if (accountGeneration() !== gen) throw new AccountClosedError('changed');
return out;
```

`enter` reads the two facts and decides. OPEN and EMPTY pass. ADOPT writes the record
and waits for that write (one file; the first answer after a sign-in then works). LOCKED
refuses. CLEARING refuses at once and, if no wipe is running, starts one in the
background; it never makes an answer wait for a wipe. Fifteen resources that arrive
together at launch therefore cost fifteen fact reads and one wipe, not fifteen waits.
The generation check ends an answer that began before a wipe.

**The mechanism: the world (2026-11-21, after five review rounds; replaces per-site guards).** Guarding each await
did not converge: every review round found another one (a late read that sets a cache, a sync that starts a fresh
send under the next key, a take-session load, a theme loader). So the guard is not at the await.
*What Exact offers* (read at `mobile/.exact-src`, `docs/reference.md`): no call from app code reloads the data
module or opens a fresh JavaScript context. Module reload exists only as a development client and over-the-air
update (`reference.md:2289-2296`; "Files and databases survive module reload", `:2886`); `exactRestart` (`:4199`)
keeps a child process alive, not the module; timers end with the answer that made them (EXACT.md, Timers). So
option 1 (a fresh runtime when the boundary moves) is not available, and the choice is option 2.
*What was chosen:* `api.ts` keeps one `World` per stretch of boundary with no account change in it. A sign-out,
a Clear or an Adopt closes it and opens the next. Every I/O choke point is inert for a closed world: the storage
the gate hands a source (`bindStorage`), `getJSON`, `authedJSON`/`authedBytes`/`authedToFile`, the podcast feed
fetch, and a chain's timers. A chain that waits on I/O in a closed world never resumes (its promise never
settles), so after the move nothing of it is inserted, written or sent, and no per-site guard is needed. The gate
races each answer against its world and ends it refused. A request is signed only with the key of its world
(`world.digest`, set by the gate from the facts); work held outside a chain, such as a queue of posts, carries
`opts.world` and is refused once that world is closed. The Clear drains every write the closed world started
before it wipes, and a Clear that times out is a failed Clear (the phone stays CLEARING, the card says to close
and reopen the app); nothing is abandoned and there is never a second writer. A queued Adopt or Clear whose
boundary moved before its turn is skipped; a queued sign-out refuses if the key it finds is not the one it saw at
the tap. Texture themes (the one account-bearing store the exempt theme sources reach) are not shown while the
new world is owed. *What it does not cover:* a chain resumed from a non-I/O await after the close (a microtask,
a promise another chain resolves) that then starts new I/O begins it in the new world; the inventory test lists
every module-level mutable name so a new lane cannot add one unseen, and keeps raw `fetch` and the Keychain read
in their choke points.

**The exempt list** is the sources that read no account store, make no signed-in
request, and are handed no account data in their arguments: `accountState`,
`accountSignOut`, `hostCaps`, `aboutBuild`, and the theme sources except
`themeFromTexture` (which fetches a Texture document with the key, `themes.ts:498,652`).
`barInfo`, `radioPage` and `conversationInfo` are gated: their arguments are the native
conversation entries, bar facts and radio facts (`members-talk.contract:10-11`,
`spine.contract:38-44`, `members-radio.contract:10-11`), which hold the account's
words while the native side is unwiped. The list is named in `app.ts`.

A refused source throws, and the page shows its existing failure state. The words are
in one place: the account card (`accountState`, `account-state.ts`), which is exempt,
asks the boundary for the state without waiting (`boundary.state()`: the same fact read,
no writes; when the state is CLEARING and no wipe is running it starts one and answers
at once) (amended in landing 1: the card also runs the Adopt job when the state is ADOPT, and answers with its result. The write is the same one a gated answer would make, idempotent, only when there is no record and a key, and it is what heals a failed Adopt without a tap), and is shown on every page by the banner and on Settings > Account.

| State | Pages | The card says | Actions on the banner and in Settings | How it heals without a relaunch |
| --- | --- | --- | --- | --- |
| OPEN | work | nothing (after a 401: "no longer accepted") | Sign out; after a 401, Clear this phone | — |
| EMPTY | work (public pages, signed-out words) | "You are signed out." | Sign in | — |
| ADOPT failing | refuse | "This phone could not record the account." | Try again | the next gated answer or the tap retries the write |
| CLEARING | refuse | "Clearing the previous account's data from this phone." | Try again and Clear this phone, after five tries | each gated answer and each 3 s re-read start a wipe if none runs |
| LOCKED | refuse | "Unlock your phone to continue." | after five tries: Try again, Clear this phone | unlock; the 3 s re-read |

The 3 s re-read (`account.contract`, `rereadLocked`) gains `clearing` and `adopt`,
bounded to five tries like `rereadRejected`; then the buttons appear and "Try again"
restarts the count. It is file work and one key read, not a model call.

**The buttons reach every page.** Today the banner injects only `signIn` and draws a
button only for it (`signedout.contract:8-21`), and Settings draws "Clear this phone" only
`if au.signedIn` (`settings.contract:357-359`), which is false once the key is gone. The
banner injects `signOut` and `retryAccount` as well and draws the button that
`acs.action` names (`signIn`, `clear`, `retry`); Settings draws them from `acs.action`,
not from `au.signedIn`. "Clear this phone" is the sign-out: exempt, it never waits on
the boundary, and it is offered in every refusing state, so no state has no way out.

**Pages re-ask when the boundary heals.** A resource is asked again only when an input
changes (`docs/reference/exact/ui-model.md:76-80`), so a page that refused stays blank
after the heal unless something it reads moves. The card's answer carries the boundary
epoch (the wipe counter and the state); one derive, `acctTick`, replaces
`signedOutTick` (`account.contract:55`, which moves only on a sign-out result), and every
gated resource takes it as its last argument (files and mail already take an `_acct`
argument; the rest gain one their source ignores). A wipe started by the gate, at launch
or on a key change, therefore re-asks the bar, the open pages and the notebook too.

**If it cannot heal.** A store the system refuses to remove keeps the phone in CLEARING;
a key that stays unreadable keeps it LOCKED. After five tries the card names what failed
and says: "If this does not clear, delete and reinstall the app. Nothing on your
computer is affected." A reinstall removes the app's container; the Keychain survives it,
so the phone comes back in ADOPT with empty stores and the same account.

**Sign-in is offered only in EMPTY.** No second account can sign in while the first
one's data is on the phone. The refused-key state today offers "Sign in" over the old key
(`account-state.ts:21`), which replaces key A with key B and leaves A's data; it offers
"Clear this phone" instead. If a key appears anyway, the table still holds: a record
naming A with key B is CLEARING.

**Cost.** One record read and one key read per gated answer. Today `keyState` is two
native calls and a SHA-256 in shared code. The first landing gives it a deadline and logs
the count of key reads so a spike is seen; landing 2 has `keystate` answer the digest in
one native call, so the key never crosses the bridge for a check.

## 4. One writer, and the order of durable writes

`account.ts` is the only writer of the record, the only caller of the shared wipes and
the native wipe, and the only place the generation moves. Its work runs one job at a
time on one promise chain; the fact reads that decide a job are made inside the job, so
two answers cannot interleave between a read and the write it leads to. A job that
finds the state already changed does nothing. There are three jobs.

**Adopt.** Read the facts; if still no record and a key, write the digest. A failed
write leaves the state closed.

**Clear** (one wipe attempt). Read the facts; if still CLEARING:

1. move the generation once, to g (`wipeAll` stops moving it itself; a check that wipes
   nothing never moves it);
2. ask native `{op: 'wipe', generation: g}` under the deadline. If a native wipe from an
   earlier attempt is still running and within its deadline, wait for it; past its
   deadline, ask again (a lost reply must not starve the phone; a repeat is idempotent,
   and from landing 2 an older one no longer clears anything);
3. run every shared wipe, each on its own; then look for survivors of every wiped path,
   including the `app:/cache/mail-*` copies `wipeMail` removes by name, with a removal
   counted only on `ENOENT`;
4. only if native answered done with no failures and nothing survived: remove the record,
   and confirm it reads `ENOENT`;
5. read the key again: present means Adopt, in the same job.

**Sign-out** (the person's tap, never behind the gate):

1. move the generation and stop sealing (`beginSignOut`; its key read gets the same
   deadline, or it sets the block without naming a digest);
2. ask native `{op: 'signOut', generation: g}` under the deadline: delete the key, run
   the native wipe at g;
3. run Clear if the facts say CLEARING.

The result follows from the facts afterwards, not from the native answer: key absent and
record removed is "signed out"; key absent and CLEARING is "signed out; clearing"; key
still present is "Sign-out did not finish. Try again.", the phone stays OPEN for the same
account, and `abortSignOut` lifts the sealing block (today it stays blocked for that
account for the process, `api.ts:130-143`). A native answer after the deadline changes
nothing: when the key goes, the next answer finds CLEARING.

**A wipe never deletes a key.** Only the person's sign-out does, at step 2. A wipe after
a relaunch, or with another account's key present, keeps the key. Main today calls the
native `signOut` from the launch settle, which deletes whatever key is stored (seat 1,
round 2, finding 3: it could delete a key B that arrived in the window).

**Ordering.** Two answers in CLEARING: one Clear, both refuse, both re-asked when the
epoch moves. Two sign-outs, or a sign-out during Clear: the later waits on the chain. An
answer during a sign-out: OPEN before the key delete (its writes dropped by the
generation), CLEARING after. A sign-in during a pending sign-out is not offered; if a key
B appears anyway, Clear then Adopt B, and B wrote nothing meanwhile. A late native answer
has no continuation code; at most it ends a native wipe Clear is waiting on.

**Durable order.** The record is the only durable fact shared code writes. It is written
only when there is none; removed only after every wipe succeeded and native reported none
failed; never rewritten from one account to another (an account change is Clear, then
Adopt).

**The second attempt wrote the new account's stamp before wiping. That is wrong; the
reverse is right.** Writing "B" first labels A's data as B's for as long as any wipe
fails, and every reader that trusts the label reads A's data as B's (seat 2, round 2,
B1). The label claims ownership only when it is true: remove the old record after the
wipe, write the new one after that. The second attempt wrote first because it needed a
durable place for "wipes still owed"; here a record naming A with a key that is not A is
that debt.

**A crash at each point, and what the relaunch reads.**

| Crash after | Record | Key | Relaunch state | What happens |
| --- | --- | --- | --- | --- |
| sign-out tapped, before native | A | A | OPEN | nothing happened; still signed in |
| native deleted the key, before its wipe | A | absent | CLEARING | wipe, remove record, EMPTY |
| native wipe, before the shared wipes | A | absent | CLEARING | the same (wipes are idempotent) |
| any shared wipe | A | absent | CLEARING | the same |
| the wipes, before the record removal | A | absent | CLEARING | the same |
| the record removal | none | absent | EMPTY | — |
| a launch that found key B, mid-wipe | A | B | CLEARING | wipe, then Adopt B |
| the record removal, before Adopt B | none | B | ADOPT | write B |
| a native delete that silently failed | A | A | OPEN | the card said "did not finish" |
| a torn record write (host not atomic) | another | any | CLEARING or LOCKED | wipe; never adopt |

No crash point leads a relaunch to OPEN or EMPTY while another account's data remains.

## 5. Native stores take the same boundary

Native code keeps its own counter (`AccountWipe._generation`, bumped by `run()`,
`AccountWipe.swift:17-36`), and stores in other lanes guess the boundary from it:
`HeldPages` set its floor to "the last generation seen, plus one" (reader review r4) and
`TakeMirror` one relative to the last push it accepted (takes reviews r6, r7). After a
failed sign-out such a floor sits above the generation the still signed-in account uses,
and every push is refused for the rest of the process.

The rule: **there is one generation, and shared code writes it.** Native code receives
it, compares it, and never makes one.

- `{op: 'signOut', generation}` and `{op: 'wipe', generation}` carry it
  (`ModelsEntry.swift:53-63`), through `signOutForShared(keepKey:generation:)`
  (`AutoradioModule.swift:109`) to `AccountWipe.run(generation:)`. `bumpGeneration()` is
  deleted; the 19 native writers that capture `AccountWipe.generation` keep working and
  now use the shared number.
- `run(generation: g)` with g below the native generation it has already reached clears
  nothing and answers done for nothing: a late wipe for A can never clear what B wrote at
  a newer generation. With g at or above it, it sets the native generation to g, then
  clears every store.
- Each store's floor is the generation of the latest wipe it ran. A push carrying a
  generation below it is refused; one at or above it is accepted. `HeldPages.lastSeen`
  and the seen-relative floor of `TakeMirror` go. The floor is always a generation shared
  code has already reached, so after a failed sign-out the account's next push is
  accepted.
- Each registered wipe reports whether it removed what it owns (a removal that throws, a
  file still there). The native answer carries `failed` and the generation, and says
  done only after every wipe returned (today `ModelsEntry.swift:62` answers done for
  every `wipe`). Clear keeps the record while `failed` is not empty.
- When the key delete fails, the native sign-out runs no wipe and answers not done
  (today `ChoirSignIn.swift:128-129` deletes, then always wipes, and the mail key goes
  too, `ModelsEntry.swift:59`). An account that stays signed in keeps its data.

What landing 2 does not close: native code reads its own stores without asking shared
code. If the native wipe has not finished and a new account's voice turn starts (a
launch with key B, before shared code's first answer; `AutoradioModule.swift:192` starts
syncs at init), native code could read or write the old account's turn history or held
page. The native package sources (`conversationEntries`, `barFacts`, `radioPageFacts`,
`takesPage`, `openPages`, `deckState`, `readerDoc` and the rest the native interface
lists) stay ungated until their pages move to shared code. Landing 3 closes the voice's
part with one gate at its entry points (`Conversation.begin`, which the problem record
notes has no generation guard, the held-page read and the launch syncs): native code
refuses those while its last wipe has not returned, and at launch until shared code has
answered once. The problem record stays open until landing 3 lands.

## 6. Files

Landing 1:

- `mobile/AutoradioLab/account.ts`: rewritten smaller. Keeps `WIPES` (without the record)
  and `wipedPaths`. Adds the code-aware fact reads, `state`, `enter`, the one job chain
  and its three jobs. Deletes `PENDING_FILE`, `decide`, `reconcile`, the late
  continuations, the token, `settled`, `resetSettled`, and the generation bump inside
  `wipeAll`.
- `mobile/AutoradioLab/app.ts`: the gate, the exempt list, `clearDemoLeftovers` behind
  the gate; the `signout-pending` grant lines go.
- `mobile/AutoradioLab/api.ts`: a deadline on `keyState`'s calls and on `beginSignOut`'s
  key read; a count of key reads in the log.
- `mail.ts`: `wipeMail` names the cache copies it removes. `account-state.ts`,
  `words.ts`: the card from the boundary state; actions `signIn`, `clear`, `retry`; epoch.
- `account.contract`, `signedout.contract`, `settings.contract`: the re-read task; the
  banner's three buttons; Settings from `acs.action`; `acctTick`.
- Every contract with a gated resource: `acctTick` as its last argument.
- `account.test.ts` rewritten from section 7 (its S4, S8 and S17 change with the record
  leaving `WIPES`); a new `app-gate.test.ts` drives `answer`.
- `mobile/scripts/proof-real.sh`: the drills of section 8.
- `docs/current-architecture.md` (the account row) and the Exact reference page (the
  error codes and how we branch on them), in the same change.

Landing 2 (native receives the generation): `ModelsEntry.swift`,
`AutoradioModule.swift` (one parameter passed through), `ChoirSignIn.swift`,
`ChoirApp.swift`, `AccountWipe.swift` and its registered wipes; `HeldPages` and
`TakeMirror` wherever they have landed; `keystate` answers the digest and the reason it
is unreadable (today every status but not-found is "unreadable", `ModelsEntry.swift:38-41`,
so the card cannot tell a lock from a broken item).

Landing 3 (the native read gate): `Conversation.swift`, the held-page read, the launch
syncs in `AutoradioModule.swift`, after the entry-point list is written.

## 7. Tests, from the model

Written before the code, each failing on main or on the second attempt where it names a
finding. "Drive" means through `app.ts` `answer` with a real source reading a seeded
store (takes, podcasts library), not through `account.ts` alone.

**The twelve cells.** For each cell of section 2 and for the host with no native module:
the state; whether a gated source answers or throws (drive); the card's action; what one
job leaves (record, stores, key untouched); that a second job changes nothing; that a
cell with no wipe leaves the generation unmoved.

**Fact reads.** R1 the record rejecting `ENOENT` is none; `EACCES`, `EBUSY`, `failed` are
unreadable (LOCKED, nothing wiped). R2 an empty, short, upper-case, or digest-plus-list
record is "another". R3 a record read past its deadline is LOCKED. R4 no storage is
LOCKED. R5 `keystate` present, then the session read hangs or finds nothing: LOCKED. R6
`keyState` hangs: LOCKED after the deadline. R7 a store path whose stat rejects anything
but `ENOENT` is a survivor.

**Transitions.** T1 Adopt writes only when there is no record. T2 Clear runs native, the
shared wipes, the survivor check, the record removal, in that order; with key B, Adopt B
follows. T3 the record is never rewritten from one digest to another (a spy on writes).
T4 Clear calls native `wipe`, never `signOut`. T5 one generation move per attempt.

**Crash points.** One per row of the crash table; a fresh module set reads the same files
and key; no gated source reads the old store before the relaunch's job ends.

**Physical faults.** F1 native `signOut` past the deadline, the key deleted later: "did
not finish"; once the key is gone the drive refuses. F2 native throws at once, throws
later, answers not done. F3 the key delete fails: OPEN for A, no shared wipe, sealing
unblocked. F4 a shared wipe throws, or leaves its file: CLEARING stays, the card names
the store, the next attempt retries every store. F5 Adopt's write fails: refused; retried.
F6 the record removal fails or the file survives it: CLEARING stays. F7 a mail cache copy
survives: CLEARING stays. F8 native answers done with `failed` not empty: the record
stays (landing 2). F9 a native wipe that never answers: a second attempt within the
deadline waits, past it asks again; when one answers done, the record goes.

**Concurrency.** C1 fifteen gated answers during CLEARING with key B: one Clear; all
refuse at once; after it, the epoch moves and each re-asked answer reads B's empty data.
C2 a sign-out while Clear runs waits for it. C3 two sign-outs. C4 key B appears while
CLEARING: Clear, then Adopt B; B's write during CLEARING is refused. C5 a source begun in
OPEN with a wipe completing while it awaits: answer refused, write refused. C6 a late
native answer after its attempt ended writes nothing. C7 a gated answer while the
sign-out awaits native: OPEN before the key delete, CLEARING after.

**The gate and the card.** G1 every source not exempt throws in CLEARING, ADOPT failing
and LOCKED (enumerated from `sources`). G2 every exempt source, called in CLEARING with
the old account's data seeded in the stores and in its arguments, returns none of it and
makes no signed-in request (a fetch spy). G3 the card's action per state; no sign-in
outside EMPTY; "Clear this phone" for a refused key and after five tries. G4 the gated
resources re-ask when the epoch moves (a contract test: `acctTick` reaches every gated
resource). G5 an exempt source never waits on a running Clear.

**Native (landing 2, Swift package tests through the lock).** N1 `run(generation: g)`
sets the native generation to g; a late `run(g - 1)` clears nothing and leaves a store
written at g intact. N2 a failed key delete: no wipe, answer not done, the next push
accepted. N3 a push below the floor is refused; one at it is held. N4 a wipe that throws
or leaves a file is reported in `failed`. N5 no wipe test touches a developer's real
files (temp instances for every store passed to `registerStandard`).

**Every review finding, mapped.**

| Finding | Test |
| --- | --- |
| Problem record: timeout, throw, not-done, late return on any key, failed re-settle remembered | F1, F2, C6, F4 |
| Round 1 seat 2 F1: a retry wipes the new account's data on every answer | C4, F4 |
| Round 1 seat 2 F3 / round 2 seat 1 #5: a failed removal counted as clean | F6, R7 |
| Round 1 seat 2 F4 / seat 1 #4: a throwing settle stuck for the process | R3, R6 |
| Round 1 seat 1 #1 to #3: memo and no stamp; late continuation; B inside the window | the cells, C6, C4, G3 |
| Round 2 seat 2 B1 / seat 1 #1: `app.ts` discards the settle result | G1, every drive |
| Round 2 seat 2 B2: unreadable key with a pending list | the LOCKED cells |
| Round 2 seat 2: the native retry not scoped to the account | T4, N1 |
| Round 2: unknown list name loops; generation bumped each answer | R2, T5 |
| Round 2: a garbled stamp read as none; native calls with no deadline; a sync throw | R2, R6, F2 |
| Round 2 seat 1 #2, #3: marker removal fell through; settles race; B's key deleted | F6, C1, T3, T4 |
| Reader r4 / takes r6, r7: native floor above the account's generation | N1, N2, N3 |
| Takes r6: wipe tests delete real files | N5 |
| Design review: exempt sources handed native data; `themeFromTexture` | G2 |
| Design review: fail-open fact reads; no storage | R1 to R7 |
| Design review: late older native wipe; swallowed native failures; failed delete still wipes | N1, N4, F8, N2 |
| Design review: gen read before the heal; the card and sign-out wait 30 s | C1, G5 |
| Design review: no buttons off Settings; pages never re-ask | G3, G4 |
| Design review: lost native reply; sealing stuck after a failed sign-out | F9, F3 |
| Problem record: `Conversation.begin` has no generation guard | landing 3 |

## 8. Landings

**Landing 1: the gate and the derived boundary.** Red, two seats (seat 1 Gemini 4
Hydrogen through `agy`, a Sonnet subagent on a rate limit; seat 2 DeepSeek v4.1 flash),
each given the code and this document, not the author's conclusion. It closes the
cross-account read on the shared side and makes more pages refuse while a wipe is owed.
The reviews added three things to it, none of which can wait: the code-aware fact reads
(without them the gate opens on a failed read, the same fail-open it exists to close);
the banner's buttons and `acctTick` (without them a refusing page does not heal and an
off-Settings phone has no way out, which the design forbids); and gating the three bar
and radio sources (without it the old account's words show in CLEARING). Rollback:
revert; the record is still a bare digest in the same file main writes, so main reads
what this landing leaves, and a stray `signout-pending` file is inert. Precommit in
`docs/calibration/claude-ledger.md` before landing: "D1 to D4 pass on the lead's build
with no gated page answering in CLEARING or LOCKED", settled by the drill output. Proof
on the simulator (`mobile/scripts/proof-real.sh`, the lead's build):

- D1, the existing drill: the stamp present before; data on the pages; sign out; the
  card says signed out; Files and Mail empty; no `account-stamp`, no `signout-pending`.
- D2, a crash: sign out, terminate the app before the drill's wait ends, relaunch;
  before any page shows data, the old files are gone or every gated page refuses.
- D3, an account change without a sign-out: data seeded under the proof key; replace the
  Keychain item with another string (the proof-key import takes any file, simulator
  builds only, `ProofKey.swift:29`); relaunch; the stores are wiped before any page shows
  data, the record is the new digest, and the card offers "Clear this phone" (the server
  refuses the fake key).
- D4, a refused key: the banner offers "Clear this phone", not "Sign in"; after it, the
  pages re-ask and show signed-out words.

Expedite: `Ring-Expedite: a sign-out that does not finish cleanly leaves the old
account's data for the next account`.

**Landing 2: native receives the generation.** Red, two seats. Section 5's rules, the
one-call digest and the unreadable reason. Rollback: revert; landing 1 does not depend on
it. Proof: N1 to N5 through the Swift lock, D1 to D4 again, the reader and takes drills.

**Landing 3: the native read gate.** Red, two seats, after the entry-point list is
written into the problem record. Rollback: revert. Proof: a Swift test that a voice turn
at launch with a changed key reads no old store, and D3 with a voice turn. Then the
problem record closes.

## 9. Reviewed and kept

- **The key is not read a second time after each source** (seat 2, finding 4). Only two
  code paths write the sign-in key: the device flow (`ChoirSignIn.swift:95`) and the
  simulator-only proof key (`ProofKey.swift:29`, run before any key read,
  `ModelsEntry.swift:43-48`). The device flow is offered only in EMPTY, a second flow
  cancels the first (`ChoirSignIn.swift:50`), and sign-out cancels it
  (`ChoirSignIn.swift:125`); so the key cannot change during an OPEN answer from a path
  that exists, and a change during an EMPTY answer exposes nothing. A second native read
  per answer would buy safety against a writer that does not exist; if one is added, its
  review must add the read.
- **The sealing block is not deleted in landing 1** (seat 1, should-change 1). Lifting it
  when the key remains (section 4) ends the stuck state; deleting `blockedDigest`,
  `sealingBlocked` and `refreshSealBlock` (`api.ts:130-143`) is a simplification that
  touches mail sealing, and goes in its own change after landing 1, not inside the red
  one.
- **Native init syncs at a changed-key launch** (seat 1, should-change 3) are landing 3,
  not landing 1: the shared gate cannot reach them (`AutoradioModule.swift:192`), and
  landing 2 already stops a late wipe clearing newer data.

## 10. What I am unsure of, and what would falsify this

- **Device error codes.** The pin's reference and conformance say `ENOENT` on every host;
  I did not see an iPhone answer for a data-protected file before the first unlock. If it
  is not `EACCES`, `EBUSY` or `failed` (all LOCKED), R1 fails on the device.
- **The bar in CLEARING.** Gating `barInfo` may hide the bar the card sits above; the D
  screenshots must show the card and its buttons.
- **The cost of the reads per answer.** Not measured; if too high, the one-call digest.
- **Failure placeholders** (`files.contract:192-193`, `mail.contract:103-104` draw a
  skeleton); D1 to D4 show whether a refusing page misleads.

What would falsify the design: a drive or drill in which a gated source returns a
store's contents while the record names an account other than the stored key's; a
relaunch after any crash point that reads OPEN or EMPTY with the old files present; an
exempt source returning the old account's data; a native floor above the shared
generation; a refusing state that does not heal once the fault is gone.

## 11. Owner decisions, each with the default taken

1. **A refused key offers "Clear this phone", not "Sign in".** Default: yes. Signing in
   over the old key is the one path that changes the key with the old data in place.
2. **Public pages refuse during CLEARING and LOCKED.** The paper front and podcasts
   library are in the wipe list. Default: refuse; CLEARING lasts seconds unless a store
   cannot be removed, and LOCKED lasts until the unlock.
3. **A crash before the native key delete leaves the person signed in.** Default:
   accept. Keeping the intent across a crash needs a durable marker again and a retry
   that may delete a key; key deletion stays the person's own tap.
4. **Themes stay device settings**, outside the boundary, except a theme installed from a
   Texture document: its file `texture-<id>.json` and the copy in memory are removed by every Clear
   (they carry account A's document's name and colours; landing 1, after seat 2's review). Bundled and
   hand-installed themes and the choice stay. Default: yes.
