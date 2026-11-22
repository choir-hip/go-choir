# The account boundary in shared code

> **Superseded for landing (2026-11-21).** Sections 1 to 8 are the record of what was tried (the record, Clear, Adopt, the five-state table). The plan that lands is the section "Accounts are partitions (2026-11-21)" at the end of this file; where the two disagree, that section wins. Its station 1 is built in lane `account-partition-s1`; the claims below about the stamp, CLEARING, ADOPT and their rollback are not true of that code.

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
at once), and is shown on every page by the banner and on Settings > Account.

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
revert. (Corrected 2026-11-21: this holds only for the landing this section describes, which was not built. After the partition design's station 1, the record `account-stamp` is deleted at launch and nothing writes it, so a revert to main finds no record, no old top-level stores and a key: main ADOPTS the key onto empty stores and the phone starts signed in with empty stores, or signed out when the key is gone. That is safe with one user: nothing crosses accounts, and what the computer holds comes back. A leftover `ending` mark is inert to main, and the account roots are invisible to it.) Precommit in
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
4. **Themes stay device settings**, outside the boundary, except a theme fetched from a
   Texture document. Default: yes.

## Accounts are partitions (2026-11-21)

An amendment, not code. Nothing here is built. It replaces sections 1 to 8 as the
plan for landing; sections 1 to 8 stay as the record of what was tried. Every station
below is red (auth and session).

### What the owner asked, and the reading

Verbatim, 2026-11-21:

1. "hmm, is maybe the design a problem? should we really be going through the account
   gate so much?"
2. "we should allow users to be signed in to multiple accounts, as we do on choir.news.
   but that is a separate thing from signing in and out without disrupting service"
3. "i never asked for signout without disrupting service"

The reading. The design is the problem. Seven review rounds on lane
`account-boundary-l1` each found a narrower case of one kind: work or state of account A
lands where account B reads it. That is what a shared store must suffer when one
account's data and the next account's data live at the same paths, in the same module
state, in the same process. The gate was asked to hold back every late write, every
late read and every exempt source. It cannot be made tight one site at a time.

Two things were asked for, and they are separate. Several accounts signed in at once
is a feature. Keeping the app running through a sign-out was never asked for (quote 3).
The lead built for it unasked, and most of the machinery exists to serve it: Clear,
Adopt, the drains, the CLEARING and ADOPT states, and the world that opens a new
account in the same process. So a sign-out may stop playback and end the session. The
simplest sign-out that is certainly safe wins, and machinery is deleted rather than
kept alive. Only adding an account and switching between signed-in accounts still need
care inside the process.

### What choir.news does

From the code (confirmed in code, not taken from `docs/design/account-switcher-2026-11-09.md`):

- **The session.** The browser holds one access and refresh cookie pair, for the
  current account only (`internal/auth/handlers.go:30-33, 408-424`). A third cookie,
  `choir_accounts`, names a server-side set of accounts. Only its SHA-256 is stored
  (`internal/auth/accounts.go:21, 37-47, 82-93`). Each member is a row of (set, user,
  refresh session), and the set records the active user
  (`internal/auth/account_set_store.go:20-35`). Sign-in adds the account to the set and
  makes it active (`handlers.go:466-474`; `accounts.go:122-153`). A refresh token of a
  member that is not active is refused as `account_switched` (`accounts.go:197-221`).
- **List and switch.** `GET /auth/accounts` lists the set (`accounts.go:257-282`), and
  the Launcher shows it (`frontend/src/packages/launcher/Launcher.surface.svelte:22-34`).
  A switch (`frontend/src/lib/accounts.js:170-176`) flushes the desktop state, closes
  the live channel, posts the switch, tells the other tabs and then calls
  `window.location.reload()`. The comment at `accounts.js:164-169` gives the reason: the
  reload is "the simplest reset of every per-user cache, socket and store".
- **Computer.** The proxy routes by the user id in the access token, to that user's
  primary computer (`internal/proxy/handlers.go:1244, 1396-1408`). One account has one
  routable computer. A switch replaces the token and reloads, so the page then talks to
  the new account's computer.
- **Per account and per browser.** Browser keys are scoped as `<base>:<lastUserId>`
  (`frontend/src/lib/user-scope.js:4-32`). That covers the theme boot cache, reader
  preferences, Autopaper visits and a few more. Texture drafts are keyed by user id.
  Nothing deletes A's scoped keys at a switch or a sign-out (`accounts.js:178-204`). A's
  keys stay readable in the same origin, and only the key name picks which account the
  page reads. There is no IndexedDB and no service worker.
- **Sign-out.** Signing out of one account (`POST /auth/accounts/signout`,
  `accounts.go:417-455`) deletes that member and its refresh session. If it was the
  active account, the page switches to the next one and reloads (`accounts.js:178-196`).
  If not, only the list changes. Signing out of all (`accounts.go:459-499`) deletes the
  set and every cookie, then reloads. Browser storage is never cleared.
- **In flight.** Nothing is cancelled one by one. The reload ends it all
  (`frontend/src/lib/Desktop.svelte:1843-1847`).
- **Limit.** None on the number of accounts. Only per-IP rate limits apply
  (`internal/auth/rate_limit.go:24-43`).

**What the phone takes from it.**

- An account is the user, and its computer follows from it.
- The list of accounts belongs to the device.
- Storage is partitioned by account, and nothing is wiped at a switch.
- A switch is a reset of the process, not a careful handover inside it. On the web that
  reset is `location.reload()`.

**Where the phone differs.** The phone holds long-lived bearer keys, one per account,
not a cookie set; the server knows nothing of a set. The phone also has no reload:
nothing app code can call reloads the data module or opens a fresh JavaScript context
(Exact `docs/reference.md:2289-2296`; section 3 above). The phone must partition harder
than the web does. The web scopes its keys and leaves A's data in place. The phone puts
A's data under A's root, where B's handle cannot name it, and deletes the root at A's
sign-out.

### Two findings that shape the plan

Both were found while checking the lead's proposed direction. Neither appears in the
seven review rounds.

**Native code signs with one Keychain item.** `ChoirClient` reads the bearer key from
the one native item (`exact-package/apple/Kit/ChoirClient.swift:79, 268, 324`). That is
service `news.choir.autoradio.choir`, account `api-key`. Every native page source that
calls the server goes through it: the notebook, learning, the radio programme, podcast
clips, the Texture reader, the run sheet store, takes, the activity feed, corpus search,
story bodies and more (`exact-package/apple/Kit/*.swift`). The native mail seal makes a
key only while that item exists (`MailSeal.swift:18`). About nineteen native stores hold
account data and are wiped together, not partitioned (`AccountWipe.swift:69-97`). They
find their own folders, about fourteen sites, with no common base.

So an in-process switch to an account whose key is not in that item leaks. Native pages
keep showing A's stores and keep signing with A's key while B is current. No shared-code
design reaches that. Native needs one gate (station 2), and those pages read as signed
out for any other account until they move to Exact.

**Exact keeps resource answers across launches.** A native host shows a resource's last
answer at the next launch, before the data module is ready. That holds for a resource
whose source read device state (a file, a database, a secret), when it is asked with the
same identifying arguments, up to 256 KiB each and 512 KiB in all
(Exact `docs/contract-for-agents.md:767-787`). A resource ending in `transient` is never
kept (`:788-791`).

Our account resources take `acctTick` as an argument, and its value (for example
`open|0|1|c`) can be the same at two launches. So after a sign-out, a relaunch and
another account's sign-in, the first frame can show A's kept mail list or file list.
That holds on main, and on the lane too, because no wipe touches Exact's launch cache. A
partition alone does not fix it. The account must be one of the resource's identifying
arguments (invariant I7). I found no call app code can make to forget kept answers, so
it is an open question below.

### Recommendation

Partition on disk now, with one account per process. A sign-out ends the session behind
a full-screen card, and a switch, when it comes, is a restart the person makes by
closing and reopening the app, as choir.news's switch is a reload.

A seamless switch inside the process is a later, separate landing. It is taken only if
the owner wants it after using the restart (decision D2), and it needs two things first:
the world passed explicitly to every account read, and the native gate.

### The model

**An account** is one bearer key. It belongs to one user and, through the user, one
computer. Two keys for the same user are one account: the second is revoked at once
(Add, below).

**Where keys live.**

- *The native slot.* The one item native sign-in writes today, service
  `news.choir.autoradio.choir`, accounts `api-key`, `key-id` and `computer-id`. It holds
  at most one account. It is never written, moved or re-keyed by this design (the
  owner's credential). Shared code reads it as it does today (`api.ts` `keptSession`,
  `keyFact`). Only the person's sign-out of that account deletes it, through the
  existing native `signOut` op.
- *The account secret.* Every account added from station 3 on is kept in Exact's own
  secret store, under one grant: `secret.keep accounts`. Its value is a JSON list of
  `{digest, key, keyId, computerId, userId, email}`. One name, because grants are static
  and a name the grants do not keep "reads and writes as `denied: secret.keep`" (Exact
  `docs/reference.md:6418-6437`). Exact keeps it in the Keychain on Apple and in
  Keystore-wrapped files on Android (same lines). So Android gets accounts too, and no
  native code is written for it. A second grant, `secret.keep canary`, holds a constant.
  When it reads back, the secret store answered, so an absent account is truly absent
  (Launch, below).

**The root.** Each account's files live under three folders:

```
app:/data/accounts/<d>/    kept data: takes.json, mail/, content.db, vectors/,
                           theme-textures/, podcasts-library.json, podcast-feeds/,
                           recent-files.json, notebook-cache.md, open-pages-shared.json
app:/cache/accounts/<d>/   refetchable: files/, mail-*.html
app:/tmp/accounts/<d>/     scratch
```

`<d>` is the SHA-256, in 64 lowercase hex characters, of the bearer key string as
stored (UTF-8). It is the same digest the record of section 2 holds, so a reviewer
compares the same string everywhere. Why the key's digest:

- It names the thing every request is signed with. A root and the key that signs its
  requests cannot disagree.
- It is known offline, at launch, the instant the key exists. A digest of a server user
  id would need a request before the first file is written.
- When the key is deleted, the root is provably an orphan. No other fact has to agree.
- A digest, not the key: paths appear in logs and listings, and a key must never.
- A new key for the same user is a new account on the phone with a fresh root. That is
  right, because a new key comes only from a new sign-in.

A process that is signed out uses one more root, `app:/data/accounts/anonymous/` and
its cache twin. It holds public data only (the paper front read signed out). It is never
swept, and every sign-out of the last account deletes it.

**The account storage.** A world's storage is `bindStorage` (it exists, `api.ts`) with
one change: it re-roots paths. `app:/data/<rest>` becomes `app:/data/accounts/<d>/<rest>`,
and the same holds for `app:/cache` and `app:/tmp`. It refuses `..`, any path already
under `accounts/`, and the device paths. Modules keep writing `app:/data/takes.json`,
and the handle puts it in the root. A module cannot name another account's root,
because whatever it names gets the prefix.

The same rule covers `storage.sqlite` open paths (the content database). It also covers
the `dest` of `authedToFile` (`exactBodyTo`), because the host writes there and not
through `storage`. One exception: the picker's documents under `app:/tmp/picked` are the
host's input. They pass read-only, unprefixed, and are removed after the upload.

**Device data** is outside every root:

- `app:/data/theme-choice.json`
- `app:/data/themes/` (the person's own theme files; bundled themes are in the binary)
- `app:/data/current-account` (from station 3: the digest of the current account)
- onboarding and device facts, which are native today.

Sources that draw before sign-in (the theme sources, `hostCaps`, `aboutBuild`, the card)
get `deviceStorage` (it exists, `api.ts`). It reaches the device paths and nothing else.
The theme sources also read Texture themes from the open account world's storage,
read-only, and only while that world is open.

**Worlds.** The `World` of `api.ts` stays: an id, a closed flag, a bag for module state
(`scoped`), and inert I/O once closed. From station 1, a process opens **at most one
account world**. It may first open the anonymous world, which is public data and no
key, and then exactly one account world. After a sign-out (and, from station 3, a
switch) the process opens no world again until it is relaunched. A late chain of A can
then reach only A's bag, A's root and A's key. No B exists in the process to reach.

**The current account** is the account of the one open world. From station 3 it is
chosen at launch from `app:/data/current-account`. If that names no live key, the first
live key is used. If there is none, the process is signed out. Nothing else changes it.

**What "signed in to several" means for each part**, from station 3:

- **Radio and the programme.** The current account's. A signed-out process reads the
  public front.
- **Run sheet.** The current account's. Pending posts of A stay in A's root and are
  posted, signed with A's key, when A is current again. Today a sign-out drops them. A's
  sign-out still deletes them.
- **Mail.** The current account's. The native seal key is one device key
  (`MailSeal.swift:15`). Its guard changes from "the native slot exists" to "shared code
  named an account" (station 2). Each record's `aad` gains the account digest, so a copy
  sealed for A does not open as B's. The key is forgotten only when the last account
  signs out.
- **Files.** The current account's root, and the computer's files. Nothing on the
  computer is ever deleted by a sign-out or a switch, so uploaded files stay in the
  cloud. A switch is refused while an upload is in flight (decision D6).
- **Takes.** The kept list is in the root. The recorder is voice (native). A switch is
  refused while recording.
- **The notebook and podcasts.** The current account's. Subscriptions are account data.
  Feed caches stay in the root, which is simpler than splitting public from private.
- **Themes.** The device's: bundled themes, own themes and the choice. Texture themes
  are account data in the root. A choice that names a Texture theme the current root
  does not hold resolves to the default. The choice itself is kept, never cleared.
- **Voice.** Native, and it follows the current account. A live conversation is not
  carried across a switch: the switch is refused while a voice turn is live. The voice
  profile and kept turns are account data (decision D9).

**Audio playing from A when the person switches to B.** Recommended: it stops.

- The lock screen's now-playing card would show A's title while B is current. Native
  `LiveAudio` and `RemoteAudioPlayer` set it.
- A's playhead writes would land in A's root after A's world closed, which is harmless,
  but the person would be listening to an account the app no longer shows.
- choir.news's reload stops everything.
- A switch is a restart (below), and a restart ends playback anyway. Stopping first
  makes it deliberate and clears the now-playing card.

Decision D3 lets the owner choose otherwise.

### Sign-out ends the session

**What the person sees.** After the tap, the whole app becomes one full-screen card:
"Signed out of <email>. Close Autoradio and open it again." (from station 3: "...to
continue as <next email>.") It has no other controls. Playback has stopped. The root
contract shows it with one `if` over the card's state. Every gated page refuses
underneath, and nothing else is drawn.

**Why a card and not the app ending itself.** Exact gives app code no call that ends or
relaunches the process. Its only `exit(0)` calls are in the smoke run and the agent
drive (`host/apple/Sources/ExactIOS/main.swift:189`;
`host/apple/Sources/ExactKit/Agent.swift:199-200`, at pin `f53347503`). A native
`exit(0)` would be a new native call. Apple's guidance is not to quit an iOS app in code:
there is no API for a graceful exit, and the person sees an exit as a crash (Technical
Q&A QA1561 and the Human Interface Guidelines, read from memory, not re-read in this
session). So the app does not end itself. The person closes it, as after any other
finished task, and the next launch is a fresh process.

**Why this is certainly safe.** After the tap the process opens no world again. Every
chain of A is inert at its next I/O, and if one runs on from a microtask it finds only
A's bag and A's root. Those are being deleted, and nothing will read them. The
non-I/O-await gap that rounds five to seven left open (the problem record, fifth round)
has nothing to cross into.

### Switch: restart or seamless

| | Restart (close and reopen) | Seamless (in the process) |
| --- | --- | --- |
| Size | station 3 only: the pointer, the card, the list | station 3 plus about 30 files: `scoped(w)` everywhere, no ambient `currentWorld()` outside the gate, a native stop op used mid-process |
| Risk | the one-world rule holds; nothing new to review | the ambient-world gap returns unless every read carries its world; native pages need the station 2 gate in-process |

On what the person sees: with the restart they close and reopen the app, and the first
frame is fast because kept answers are keyed by account. Seamless is one tap. On the
model: the restart matches choir.news, whose switch is a reset. Seamless matches the
web's tabs, not its switch.

Recommended: the restart. It is what choir.news does in the only form iOS allows, and it
adds nothing to the one rule that makes a sign-out safe. Seamless is station 4, built
only on the owner's word.

### Each operation: what is written, in what order, and what a kill leaves

Durable facts: the native slot, the account secret, the roots, `current-account`, and
(station 2) native's served-account file. Every step is idempotent. Launch repairs every
state a kill can leave, and no kill leaves a state where one account reads another's
root.

**Add an account** (station 3; before station 3, the native device flow, offered only
while signed out):

1. Run the device flow in shared code. The key is held only in that job's memory.
2. Take its digest. If the digest is already in the set, drop the key.
3. Call `GET /auth/whoami` with the new key (`settings.ts:20`). If the user is already
   in the set, revoke the new key (`DELETE /auth/api-keys/{id}`, the route the Swift
   sign-out uses, `ChoirSignIn.swift:138`) and say "already signed in".
4. Write the account secret with the new entry, and write the canary if it is absent.
5. If the process is signed out (the anonymous world), close it and open the new
   account's world. That is the one allowed move. If an account world is open, ask
   "Switch to <email> now?" (Switch, below).

What a kill leaves:

- Before step 4: nothing on the phone. The server holds an unused key, which shows in
  the account's key list. That is accepted and named.
- After step 4: the next launch lists the new account. The current account is whatever
  the pointer names.

**Switch to B** (station 3):

1. Refuse if a recording, a voice turn or an upload is live, and say which.
2. Native `{op: 'account', digest: ''}`: stop playback, end the voice session, clear
   now-playing, touch no account store.
3. Close the world. The process is now ended, and the card shows the switch words.
4. Write `current-account` = B.

What a kill leaves: before step 4, A is still current at relaunch. After step 4, B is.
Both are valid accounts. Native stores still hold A's data, and native's served file
says A. B's launch finds the mismatch and native wipes before it reads (station 2).

**Sign out of one account X:**

1. If X is current: native `{op: 'account', digest: ''}`, then close the world. The
   process is ended.
2. Revoke X's key on the server, with X's key, under a deadline, best effort.
3. Delete X's key. For the native slot, that is native `{op: 'signOut'}`, which exists
   today: it deletes the item and runs the native wipe. For the account secret, rewrite
   the list without X.
4. Delete X's three root folders with `rmTree`, on the raw storage, from `account.ts`
   only.
5. If X was current: set `current-account` to the next account, or remove it. If X was
   not current, the list on Settings shrinks, and the open world, which is not X's, goes
   on.

The result follows from the facts afterwards. Is X's key gone? If it is still present
(the native delete failed), the card says "Sign-out did not finish. Close Autoradio, open
it again and try again." The process stays ended, and the relaunch is still signed in
as X. No `abortSignOut`, no sealing block.

What a kill leaves, step by step:

- After step 1: nothing durable changed. X is still signed in at relaunch.
- After step 2: X's key is revoked but still on the phone. The relaunch opens X, gets a
  401, and the card offers Sign out.
- After step 3: the key is gone and the root is an orphan. Launch deletes it.
- During step 4: the same.
- After step 4: the pointer may name X. Launch finds no key for X and picks the next
  account or signed out.

**Sign out of all.** Native `{op: 'account', digest: ''}` and close the world. Revoke
each key. Delete the account secret and the native slot (native `signOut`). `rmTree`
the three folders `app:/data/accounts`, `app:/cache/accounts` and `app:/tmp/accounts`,
which removes orphans and the anonymous root too. Remove `current-account`. Device
settings stay (decision D5). A kill at any point leaves keys that are still listed, or
roots that are orphans, and launch handles both.

**Launch:**

1. Read the key sources: native `keystate` (absent, present or unreadable, as today) and
   the account secret plus the canary.
2. If either source is unreadable, the state is LOCKED. Nothing is swept and nothing is
   opened, and the card says "Unlock your phone to continue". The secret store counts as
   answered only when the canary reads back, or when there are no roots at all.
3. Sweep. Delete every root whose digest is not a live key's, except `anonymous`. Delete
   the leftover paths (the old wipe list, below). Neither is ever read.
4. Choose the current account (The model, above).
5. Native `{op: 'account', digest}` before native touches an account store (station 2).
6. The first gated answer opens the world: the account's, or the anonymous one.

A key with no root is normal. The root is made by the first write, and the data comes
back from the computer. A root with no key is deleted at step 3, and only when both key
sources answered. Before first unlock nothing is deleted, which matters because Exact
reads secrets "into a snapshot before boot" (Exact `docs/reference.md:6432`). Whether
that snapshot is empty or refused before first unlock is unrun on a device. A kill
during launch leaves nothing that the next launch does not redo.

**A refused key** (a 401) offers "Sign out", which is the sign-out above. Sign-in is not
offered over a refused key.

### Invariants a reviewer can check mechanically

- **I1. No account path is reachable except through the open world's account storage.**
  `storage.fs` and `storage.sqlite` raw handles are used only in `api.ts` (the two
  handle builders) and `account.ts` (the sweep and the root deletion). Enforced by the
  existing module scan (`account-world.test.ts` M3 and `account.test.ts` S4), extended
  to list raw-storage users. A test proves the re-rooting: every path maps under
  `accounts/<d>/`, and `..`, `accounts/` and device paths are refused. A test proves
  `sqlite.open` and `authedToFile` dest are re-rooted too.
- **I2. No request is signed except with its world's key.** Raw `fetch` and the
  `Authorization` header appear only in `api.ts` `send`. `send` reads the key of
  `w.account` from its source (the native slot or the secret) and refuses when its
  digest is not the world's. Enforced by M2 (it exists) and the choke-point scan.
- **I3. No module-level mutable state outside a world's bag.** Enforced by the M3 scan
  (it exists). Its blind spots are stated in the problem record (fifth round): state
  inside functions or classes, and `const x = makeThing()`.
- **I4. Sources that draw before sign-in get a device-only handle.** Enforced by a test
  that calls every device source with a storage holding a root, and checks it reads
  nothing under `accounts/` and sends no signed request (the G2 shape).
- **I5. At most one account world per process.** `new World(` appears only in `api.ts`.
  Enforced by a test: after a sign-out or a switch, no answer opens a world, and the
  anonymous world moves to an account world at most once.
- **I6. Every root is named by a live key, or it is deleted at launch, and never when a
  key source is unreadable.** Enforced by launch tests: an orphan root; an orphan root
  with an unreadable `keystate`; an orphan root with the canary missing.
- **I7. Every resource that reads account data takes the account handle (the first 16
  hex of `<d>`) as an identifying argument, or is `transient`.** Enforced by a contract
  test that lists the gated sources and checks the resource lines, the same shape as
  section 3's `acctTick` test. The handle replaces `acctTick`.
- **I8. The key never appears in a path or a log.** Enforced by a test that every root
  name is 64 hex characters, and by the existing scrub tests.
- **I9 (station 2). Native signs requests and touches account stores only for the
  account shared code named.** Enforced by Swift package tests (N1 to N4, below).

### What of the built lane survives, what is deleted, what is new

**It survives** (from `.lane/trees/account-boundary-l1/mobile/AutoradioLab`):

- `api.ts`: `World` (id, closed, bag, `whenClosed`), `inert`, `inertTimeout`,
  `scoped`, `bindStorage` (it gains the re-rooting), `deviceStorage` (it loses
  `accountDirs` and gains the open world's read-only Texture themes), `withDeadline`,
  `rmTree`, `keyFact` and `keyState`, `send` with its world-digest check, and
  `authedJSON`, `authedBytes` and `authedToFile` with their required `world`.
  `digestOf`, `check` and `nativeInert` also stay. `computerId` and its cache move into
  the world's bag.
- `app.ts`: the gate's shape and `guarded`. The exempt list becomes the device-source
  list, with the same reasons.
- `account.ts`: `AccountClosedError`, the one job chain (sign-out, add and switch
  serialise on it), and `readFacts` for the key half.
- `themes.ts`: the split into a device store and a Texture store.
- `account-state.ts` and `words.ts`: the card, with fewer states.
- Tests: M1 (a closed world writes and sends nothing), M2, M3 and its scanner,
  C9.podcast-send and C9.take-session (each restated as "after sign-out"), and the
  device-source half of C9.texture-theme.

**It is deleted:**

- From `account.ts`: `STAMP_FILE` (the record), `wipeAll`, `survivors`,
  `removeRecord`, `nativeCall('wipe')`, `clearBody`, `adoptBody`, `startClear`,
  `enter`'s ADOPT and CLEARING branches, `peek`'s ADOPT and CLEARING branches, the
  five-state table and `stateOf`, `wipeEpoch` and `lastFailure`.
- From `api.ts`: `closedWorlds`, `drainClosed`, `World.pending`, `World.downloads`,
  `owed`, `confirmed`, `nextGeneration`, `beginSignOut`, `abortSignOut`,
  `refreshSealBlock`, `blockedDigest`, `sealingBlocked` and `writeCurrent`.
- Every module's wipe function: `podcast.wipe`, `wipeMail`, `wipeFiles`, `wipePaper`,
  `wipeKept`, `wipeNotebook`, `wipeLiveVectors`, `wipeLiveContent`, `wipePages`,
  `wipeTextureThemes` and clips' `forget`. Deleting the root replaces them all.
- The theme choice's cleared-by-origin logic in `themes.ts`, and `files.ts`
  `cacheDest`'s digest folder, which the root replaces.
- `demo-leftovers.ts`: its marker joins the leftover list.
- The CLEARING, ADOPT and "after five tries" cards, the 3 s re-read of those states, and
  "Clear this phone" as a separate action. Sign out is the one way out.
- Tests M4, C9b.queued-signout, C9b.gate-adopt, C9b.stale-wipe, and every Clear and
  Adopt case of section 7.

**What remains of the wipe list.** `wipedPaths()` becomes `LEFTOVERS`: the old
top-level paths (`app:/data/takes.json`, `app:/data/mail` and the rest, plus
`account-stamp`, `signout-pending` and the demo marker). Launch removes them and never
reads them. That is the owner's ruling that nothing is carried over from earlier builds.
On the owner's phone this drops the shared stores once (the podcast subscriptions among
them) and they come back from the computer where the computer holds them. Sign-out no
longer uses a wipe list at all; it deletes one root.

**What is new:**

- The re-rooting in `bindStorage`, including the `sqlite` and `exactBodyTo` paths.
- The launch sweep and the one-world rule in `account.ts`.
- The ended card in the root contract, `account.contract` and `signedout.contract`.
- `current-account` and the accounts list in Settings (station 3).
- `keys.ts` (station 3): the two key sources behind one reader.
- `signin.ts` (station 3): the device flow moved out of `ChoirSignIn.swift`, which the
  owner's rule wants anyway.
- The native `account` op (station 2).

**Size, estimated by reading, not measured.**

- Station 1: about 25 files touched (11 modules each lose a wipe, plus `api.ts`,
  `account.ts`, `app.ts`, `account-state.ts`, `words.ts`, `themes.ts`, `files.ts`,
  three contracts, three test files and `proof-real.sh`). Against the lane it is a net
  deletion of several hundred lines. Against main it is a net addition of the `World`
  pieces.
- Station 2: about 6 Swift files.
- Station 3: about 12 files, of which `signin.ts` (about 250 lines) and `keys.ts` are new.
- Station 4: about 30 files, mostly mechanical.

### Native code

Native code is unavoidable in three places. Each has a reason only native code can meet.

1. **The native slot.** Shared code cannot read another service's Keychain item through
   Exact's store (`api.ts:44-49`, `CredentialReader.swift:4-5`). It goes on being read
   through the existing `get`, `session` and `keystate` ops and deleted through
   `signOut`. These are not new.
2. **The `account` op** (station 2, new): `{op: 'account', digest}`, with `''` for "none
   now". Reason: native stores and the native client are native, and shared code cannot
   reach them. What native does on it:
   - It touches no account store and signs nothing until the op has answered once in
     this process. This is old landing 3's read gate, in one place.
   - With `''`, it stops playback, ends the voice session, clears now-playing and drops
     on-device model context.
   - With a digest that is not the one in its served file, it runs `AccountWipe.run()`.
     It writes the served file only after every registered wipe reported success, and it
     answers `failed` otherwise.
   - `ChoirClient`'s key read refuses unless the native slot's digest is the named
     digest. `MailSeal`'s guard becomes "an account is named".
3. **Several Keychain items need no native code.** New accounts go to Exact's secret
   store under `secret.keep accounts`. Putting them in native items, one per digest,
   would be a new native call with no reason only native can meet.

**The three planned native landings of the old design:**

- Landing 2 (the shared generation to native, floors in `HeldPages` and `TakeMirror`,
  `failed` reporting) is replaced. There is no generation. The served-file check does
  the job of the floors. `failed` reporting stays, inside the `account` op.
- Landing 3 (the native read gate) is folded into the same op.
- The native wipe at sign-out stays as it is.

The two landings become one, station 2. Until the native pages move to Exact, a switch
costs A's native caches, which come back from the computer. Voice stores (turn history,
the voice profile, kept turns, take recordings) are wiped with them at a switch, unless
station 5 roots them under the account. Station 5 is native and legitimately so, because
voice stays native.

### How the earlier rounds' findings fare

- **Late awaits** (a late read that fills a cache; a sync that sends under the next key).
  Gone. A's chain can reach only A's root, A's bag and A's key, and from station 1 no
  other account's world exists in that process.
- **Module single-flights and mutexes that outlive their world.** Gone. They are in A's
  bag, and no later world in the process joins them.
- **Exempt sources.** Changed. They become device sources with a handle that reaches
  device paths only. Texture themes are read only from the open world, which is the
  process's only account. The residual of the seventh round (a frame of A's colours
  after an account change) cannot happen without a relaunch.
- **Late downloads.** Gone. The host writes into A's root path. If the bytes land after
  A's root was deleted, they recreate a folder under A's digest. No key names it, so
  launch deletes it, and no account reads it meanwhile.
- **Themes from Textures.** Gone. They are account data in the root.
- **The choice file.** Changed. It is device data, never cleared. A choice that names a
  Texture theme the current root lacks resolves to the default.
- **Cold launch with a changed key.** Gone. A changed key is a new digest, so it gets a
  new empty root, and the old root is swept.
- **A failing Clear.** Gone, because there is no Clear. A root the system refuses to
  delete is disk space, not a leak: the key is gone, no account can name the root, and
  the next launch tries again.
- **Native stores and native signing.** Still present until station 2. It is the same
  gap old landings 2 and 3 named, now closed by one op.
- **The non-I/O-await gap.** Gone in stations 1 to 3, because there is no second world
  to cross into. It returns in station 4 unless every account read takes its world
  explicitly.
- **Kept answers** (new, above). Still present on main and the lane, and closed by I7.

### What partition does not solve

- **Exact's kept answers.** A's kept answers stay in Exact's launch cache after A signs
  out. They are keyed by A's handle, so they are never shown again, and they are bounded
  to 512 KiB, but they are at rest on the phone. Recommendation: I7 for all account
  resources, and `transient` on mail (the box and the message). Open: an Exact call to
  forget kept answers, if one exists.
- **Shared device caches.** I did not find whether Exact's `fetch` on Apple keeps an
  HTTP cache for authorised responses. Recommendation: verify in station 1. If it does,
  `send` sets no-store on every signed request, a one-line change.
- **The on-device search index and vectors.** Shared code's index is in the root
  (`search.ts` `VECTORS_DIR`), and the in-memory store is in the world's bag. Native
  corpus search and the content store (`CorpusSearch.swift`, `ContentStore.swift`) are
  covered by station 2's wipe on mismatch.
- **On-device models.** The weights are device data and public. Any conversation context
  a native model session holds is dropped by the `account` op with `''`.
- **Notifications.** None exist in the app today (no `UNUserNotificationCenter` or
  push grant found). The rule for when they come: each carries the account handle, and a
  sign-out removes that account's delivered notifications.
- **The native audio session and now-playing.** Stopped and cleared by the `account` op
  at a switch and a sign-out. Background audio keeps the page live on iOS
  (`docs/reference/EXACT.md` decision table, `page.frozen`), which is why playback must
  stop before the world closes, not after.
- **Anything keyed by URL rather than account.** Feed caches, file caches and mail
  originals are inside the root. Clips are in the bag. Exact's kept answers are covered
  by I7. The native Texture reader and the opening cache are covered by station 2.
- **The server.** An unused key from a sign-in killed before step 4 stays on the
  account. Revocation at sign-out is best effort, as today.

### Alternatives considered

1. **Keep wipe-on-change and finish patching.** Seven rounds have not converged. Each
   round narrowed the leak and added machinery (drains, deadlines, detached halves, an
   inventory scan). The owner's question 1 names the cause. CLAUDE.md rule 7 says to
   stop patching after three rounds. Rejected.
2. **Partition, and keep every signed-in account's world open in the process** (the
   lead's direction as written). It buys one thing: A's in-flight work finishes while B
   is current. The cost:
   - several live worlds;
   - background work for accounts nobody is looking at;
   - the ambient-world gap, unless every read is threaded;
   - native code that must know several accounts are live.

   Native pages would still sign with the native slot's key. Rejected. If seamless
   switching comes, A's world is closed at the switch (station 4).
3. **Partition with one account per process; a switch and a sign-out both end the
   session.** This is the smallest safe design, and it is recommended. It matches
   choir.news's reset-at-switch. Its cost is that the person closes and reopens the app
   to finish a switch or a sign-out.
4. **One account per process without partition** (wipe at sign-out, the session ends).
   It is smaller still, and it is safe for sign-out. It cannot hold several accounts,
   because a switch would have to wipe A, and A's local-only data would go. It is a
   fallback if station 1 stalls.
5. **A native `exit(0)` at switch or sign-out.** Rejected: Apple's guidance, and a new
   native call.

### Landing plan

All stations are red, with two seats each (seat 1 Gemini 4 Hydrogen through `agy`, a
Sonnet subagent on a rate limit; seat 2 DeepSeek v4.1 flash). Each seat is given the code
and this section, not the author's conclusion. Each station precommits its drill claim
in `docs/calibration/claude-ledger.md` and carries
`Ring-Expedite: a sign-out that does not finish cleanly leaves the old account's data for the next account`.
Each is built in a new lane from `origin/main` that takes the `World` pieces from
`account-boundary-l1`. The lane itself does not land.

**Station 0** (green): this section, the problem record pointing here, and
`docs/current-architecture.md` unchanged until station 1 lands.

**Station 1: partition and the ending sign-out.** The re-rooting, the device handle, the
one-world rule, the launch sweep, the leftovers, the ended card, and I7's account handle
on every gated resource. Every deletion listed above.

- Protected surfaces: the session seam (`api.ts` `send`) and the sign-out.
- Evidence class: bun tests plus a simulator drill.
- Rollback: revert. Main's code reads the leftover paths, which this station removed.
  Main then starts with empty stores and the same key, so nothing crosses accounts.
- Heresy delta: none. It deletes the "no disruption" machinery the owner did not ask
  for.
- Proof: the whole `bun test`, `contract build` and `test web` in the lane, then on the
  lead's build (`mobile/scripts/proof-real.sh`):
  - **sign out of one**: data on the pages, sign out, the ended card, the root gone, the
    relaunch signed out;
  - **kill mid sign-out**: kill after the key delete and before the root delete, then
    relaunch; the orphan is gone before any page draws;
  - **launch with an orphan root**: a planted `accounts/<other digest>/takes.json` is
    gone after launch and was never read;
  - **changed key** (old D3): a new root, and the old one swept;
  - **refused key** (old D4);
  - **first frame**: after sign-out, relaunch and the proof key, no kept answer of the
    old handle shows.

**Station 2: the native `account` op** (old landings 2 and 3 in one).

- Protected surfaces: the native client's key read and native stores.
- Rollback: revert. Station 1 does not depend on it.
- Proof: Swift package tests through the lock:
  - N1: no store is read before the op;
  - N2: on mismatch, wipe, then the served file;
  - N3: a failed wipe leaves the served file unwritten and answers `failed`;
  - N4: `ChoirClient` refuses a key whose digest is not the named one.

  The station 1 drills again, plus a voice turn at a changed-key launch that reads no
  old store.

**Station 3: several accounts, switch by restart.** `keys.ts`, `signin.ts`, the accounts
list in Settings, `current-account`, the switch card and the refusals.

- Protected surfaces: the key stores and sign-in.
- Rollback: revert. A revert leaves the account secret unread, and launch then sees only
  the native slot. Added accounts' roots become orphans and are swept. Accepted, because
  there is one user.
- Proof:
  - **two accounts signed in**: add the second, both listed, each one's Files showing
    only its own;
  - **switch during a download**: refused, or, when it is a page cache download, the
    bytes land under the old root and are never shown;
  - **switch during playback**: playback stops, now-playing is cleared, the card shows;
  - **sign out of one** that is not current: the list shrinks and the open account goes
    on;
  - **kill mid sign-out** and **orphan root** again with two accounts;
  - Android: sign-in works with no native module.

**Station 4** (only on the owner's word, decision D2): the seamless switch.
`scoped(w)`, the world passed explicitly from the gate, a scan that forbids
`currentWorld()` outside `app.ts` and `account.ts`, and the `account` op used
mid-process. Proof: the station 3 drills without a relaunch.

**Station 5** (native, voice): voice stores take their folder under the account root
from the `account` op, so a switch no longer wipes them.

### Decisions for the owner, each with the default the build proceeds on

- **D1. Sign-out ends the session behind a full-screen card; the app does not end
  itself.** Why it matters: it is what makes sign-out certainly safe and deletes most of
  the lane. Options: the card; a native `exit(0)`; staying alive in the process (the old
  design). Default: the card.
- **D2. A switch is a restart (close and reopen), not seamless.** Why it matters:
  seamless is about 30 more files and brings back the class of leak this amendment
  removes. Options: restart; seamless (station 4). Default: restart. Seamless is built
  only if the owner asks after using it.
- **D3. Audio stops at a switch.** Why it matters: the lock screen would show A's title
  under B. Options: stop; keep playing until the relaunch. Default: stop.
- **D4. No limit on accounts.** choir.news has none. Default: none.
- **D5. Sign-out of the last account keeps device settings** (themes, the choice,
  onboarding facts). Why it matters: the next person on the phone sees the previous
  person's theme. Options: keep; delete. Default: keep. A Texture theme is account data
  and goes with the root.
- **D6. A switch is refused while a recording, a live voice turn or an upload is in
  flight.** Why it matters: these are local work that would otherwise be cut off.
  Options: refuse; cut off. Default: refuse, with words saying which.
- **D7. Until native pages move to Exact, a switch drops A's native caches, which come
  back from the computer, and native pages read as signed out for an account not in the
  native slot.** Options: accept; partition native stores in Swift, which the Exact rule
  forbids outside voice. Default: accept.
- **D8. Mail resources are `transient`.** Why it matters: no fast first frame for mail,
  but no mail at rest in Exact's launch cache. Default: transient for mail; every other
  account resource is keyed by account (I7).
- **D9. The voice profile is per account.** Options: per account; per phone. Default:
  per account, which is the private choice.

### What I am unsure of

- What Exact's secret snapshot answers before first unlock (empty or refused). I6 and
  the canary are built for either, but a device run decides.
- Whether `GET /auth/whoami` and `DELETE /auth/api-keys/{id}` accept the phone's bearer
  key for an account added in shared code. The Swift sign-out uses the second today.
- Whether Exact's Apple `fetch` caches authorised responses.
- How large the native served-file change is in `AccountWipe.swift` and
  `AutoradioModule.swift` (its launch syncs, `:192`).

### Station 1 build notes (2026-11-21, lane `account-partition-s1`)

Where the design was silent or the build took the conservative reading:

- **The email in the ended card.** The card says "Signed out. Close Autoradio and open it again." without the address. The address is account data, and the card must not hold any in module state; naming it is a station 3 matter.
- **Onboarding.** `app:/data/onboarding.json` is account data in the root, not a device fact: the shared first run keeps the edition the account made, and the train wiped it at sign-out. The device list holds the theme choice, the themes folder and `current-account` only.
- **The sign-out's order.** The session ends first (`endWorld`), then the native `signOut` (audio, key, Swift stores, the background revoke, all existing), then the folders. The revoke is native's; shared code does not call `DELETE /auth/api-keys/{id}` itself in station 1. Native's five-second wait for the run sheet is answered by shared code every 100 ms until the native call returns (the radio's pump no longer runs).
- **The anonymous world.** A process whose key is absent opens the anonymous world and may move to an account world once (a sign-in), checked at each gated answer and by the card.
- **A path that leaves JavaScript** (an image's `src`, an iframe's, a url the deck plays) goes through `appPath`, the real rooted path; storage calls keep the module's own path.
- **Grants.** `fs.read` and `fs.write` on `app:/data`, `app:/cache` and `app:/tmp` (the sweep reaches the old top-level paths, and the handle does the confining) and `sqlite.open app:/data/accounts`.
- **Mail** resources are `transient`; the theme sources take the handle too (they show Texture themes of the open account).
- **`demo-leftovers.ts`** is deleted rather than kept: the sweep removes every old top-level path, and no root ever held demo data.
- **Retrieval** (`retrieval-live.ts`) and the radio (`radio-bridge.ts`) keep their state in the world's bag.

- **SUPERSEDED by "The launch wipe" below.** **The ending mark** (added after review 2026-11-21, P2). A kill between the native key delete and the Swift wipe, or a wipe that failed, leaves the old account's words in native stores that relay into the shared sources (the bar, the conversation, the takes page, the radio page). Station 1 therefore writes `app:/data/ending` (a device path) BEFORE asking native to delete the key, and removes it only when native answers that its wipe finished with no failure (`AccountWipe.run()` now returns the names of the stores it could not clear). While the mark exists, in this process or at a later launch, every gated source is refused and the launch asks native to wipe again (the `wipe` op); a wipe that keeps failing leaves the locked card with the reinstall words and the Try again and Sign out buttons.
- **The sweep needs a positive fact.** It runs only on a key that was read or the Keychain's own `absent`; no native handle yet, no storage, a timeout or `unreadable` is LOCKED and not latched. A host that decided it has no native module opens the anonymous world and sweeps nothing.
- **Exact's kept answers.** `acs` and `au` (and every native resource that holds account content) are `transient`; every other resource takes the handle. See `docs/reference/exact/internals.md`.

- **The mark is the person's intent (lead, 2026-11-21).** A sign-out once begun is finished. A launch that finds the ending mark asks native to delete the key and wipe, deletes the account folders, and only then removes the mark; the phone comes up signed out, even when the key was still there (a kill after the mark and before the key delete, or the 30 s native timeout). It never resumes signed in with half the data gone. This replaces the earlier note that the launch only wipes native again and keeps the key.
- **No "no module" by a timer where the module exists.** Once a native handle has existed in the process (even one not usable yet) the launch stays LOCKED until it answers. Only a host that never had a handle (the browser, an Android build without the module) opens the anonymous world after the host's own wait; Exact's data module has no platform fact to ask, so a seen handle is the signal.
- **Sign out from LOCKED is two steps** (the first tap asks; Remove signs out; Cancel withdraws) and the offer is withdrawn the moment a key read succeeds.

- **Corrections (2026-11-21, after review).** `app:/data/onboarding.json` is ACCOUNT data in the root (the first run keeps the edition the account made); this amends the device list and decision D5 above: the device keeps the theme choice, the themes folder, `current-account` and `ending`. `voiceSetupFacts` is `transient` (it carries the voice pick). Native stores are not partitioned until station 2, so after a CHANGED key the shared pages that relay native facts (the bar, the conversation, takes, the radio page) can still show the old account's native words until the native wipe runs: station 1 does not claim otherwise.

- **SUPERSEDED by "The launch wipe" below.** **The native wipe is a fact about the disk (lead, 2026-11-21).** Three rounds found a hole in the wipe each time because its truth was a tally of the stores' own reports. Now every native file-backed store of account content keeps its files under ONE folder, `Application Support/Autoradio/account/`, named by one function (`AccountWipe.url`; a Swift scan fails a store that builds its own support or caches path). `AccountWipe.run()` is serial (a second call waits for the first), tells the stores to drop memory, removes the folder and the other known paths (the old locations of the moved stores, once: no carry-over; and the app target's stores not moved yet, `extraPaths`), forgets the mail seal when the key is gone, then LISTS each and answers what is still there: no counter, no `lastFailed`, no per-store recorder. The speaker profile and the voice choice are moved once into the folder, for a signed-in launch only (see the dated note on the carry-over). This is most of station 2's and station 5's ground: a per-account native root later is that folder named by the key digest, as the shared side already does.
- **The launch finish is in the background (lead, 2026-11-21).** A launch that finds the mark starts the finish in the background and answers LOCKED "finishing the sign-out" at once; the launch, a tap and a retry share one finish in flight. A failed finish is retried only when the card asks.

- **SUPERSEDED by "The launch wipe" below.** **The seal (2026-11-21).** The native wipe sets a process-wide seal before it removes anything and leaves a file where the account folder was; a store that writes late fails instead of recreating account data. Only a fresh process, which is what a sign-out already requires (close and reopen), clears it.

- **SUPERSEDED by "The launch wipe" below.** **The carry-over and the join (2026-11-21).** The voice move runs only when the key is positively PRESENT; with the key positively ABSENT the old voice files are removed (they belong to no one signed in); with the key unreadable they are left. The wipe's join has a deadline (25 s): a store that never returns is named (`<store>: timed out`, not clean) and a later run starts afresh. The mail seal is forgotten only on a positive `absent`.

- **SUPERSEDED by "The launch wipe" below.** **One account per process, stated as an invariant (2026-11-21).** A process that has sealed never opens a world again. EVERY finished sign-out ends the session, whichever entry finished it (the tap, the launch that found the ending mark, the card's retry): `endedWhy = 'signedout'`, `endWorld()`, the close-and-reopen card. The native sign-in entry refuses in a sealed process with the card's words, as a belt. The seal itself is replaced without a window: the folder is removed and a regular file put in its place, retried until a FILE stands there, else the answer names `seal`. A fresh process clears the seal file at its first `AccountWipe.url`, which is why the card says close and reopen.
- **SUPERSEDED by "The launch wipe" below.** **What the seal covers and what it does not (2026-11-21).** Sealed: every store that names its path through `AccountWipe.url`, now including the voice-setup record and the take recording scratch. Removed and listed but NOT sealed: shared-code caches on the app side (`recent-files.json`, `books`, `content`, the files cache) and `console.log`/`console.log.1` (a stdout capture that can hold session lines, so it joins the wipe). This is safe once the invariant holds: no world opens in a sealed process, so no shared-code writer runs after `endWorld`. Voice before the key can be read: a launch with an unreadable key and an old speaker or voice-choice file uses the OLD file (never a new empty one, which would strand the enrolment); the move happens when the key reads present. The native sign-out total (`choir.signOut` up to 8 s plus the wipe join, 15 s) fits inside the shared 30 s deadline; a run after a timeout has its own progress record.

- **No test touches the real Keychain (2026-11-21, owner of the build Mac's faults).** Every Keychain call in the Kit goes through one door, `SecDoor` (MailSeal.swift). Under XCTest its default answers "not available" and reaches nothing; a test that needs a Keychain derives from `KeychainFakeCase`, which installs an in-memory fake (with an `unreadable` switch) and restores the trap at tearDown. The three-state key read and the mail seal's status (success or item-not-found is clean, anything else names `mail-seal`) are tested against the fake. The Keychain service string is defined once (`KeychainKeyStore.defaultService`). A voice move that fails leaves the OLD file in use, never a new empty one.

- **SUPERSEDED in its shape by "THE LAUNCH VERDICT" below (what is built; the items under this heading that still hold are marked there).** **THE LAUNCH WIPE: the rule that replaces the in-process wipe, the seal and the join (lead, 2026-11-21, after six review rounds).** Every round found a hole beside the seal (a late writer after the listing, a cached path outside the root, a double run over the native deadline, a window between the removal and the seal file, a hung store holding every later run, a seal that outlived the launch that finished a sign-out). All of them came from wiping a process that was still running. The rule that removes the class:
  1. **At the tap** (`account.ts` `accountEnding`, `accountSignOut`): write the durable mark (Exact's `app:/data/ending`); end the session behind the full-screen card (no world opens again in this process); native stops the audio and the session and removes the Keychain key (`ChoirApp.signOut`); the answer is the Keychain's own status (`CredentialReader.keyState() == "absent"`). NOTHING is wiped in this process. Late writers in it may write what they like.
  2. **At the NEXT LAUNCH, first** (`AccountWipe.finishAtLaunch`, called at the top of `AutoradioModule.init`, before the console log, any store, the runtime, SpeakerGate, VoiceChoiceStore, LabRuntime or shared code exists): no mark: nothing is touched. A mark is the person's intent: the key goes if it is still there (named `key` if it cannot), the account folder, the old paths, the extra paths (`console.log`, the shared-code caches, the onboarding and recent-files records, books, content) go, the mail seal is forgotten (success or item-not-found is clean), everything is LISTED, and only a clean listing clears the mark. Nothing is writing those paths then, so the listing is true. A kill between any two steps leaves the mark, so the next launch finishes.
  3. **The mark: native decides, shared code clears (RULE B, one clearing point).** Shared code WRITES Exact's `app:/data/ending` before the key is touched. Native locates THIS app's mark from the module's `context.data` (no guessed or wildcard path), runs the launch wipe first thing at launch (off the main thread) and ANSWERS the outcome through `launchWipe`: `none` (no mark), `clean`, `stuck` with the fixed folder names left, `locked` (Keychain locked before first unlock), `fault` (the Keychain fails for another reason), or `notready` (not finished or not built). **Native NEVER clears the mark**, and while the mark exists every launch answers again (clean once its own work is done) until shared code removes it. On `clean` shared code runs its OWN sweep (the account roots, the old top-level LEFTOVERS, `picked`), LISTS what is left, and only if that listing is clean removes the mark, LAST. A kill anywhere before that last step leaves the mark and the next launch redoes everything; every step is idempotent (tested: native clean then killed before the sweep; the sweep done then killed before the mark is removed; a sweep that cannot finish keeps the mark and ends the session naming what is left). So the mark DOES depend on the shared sweep, by design, and native does not need to know shared code's leftovers list; native still removes the shared account roots (from Exact's own `context.data`, `context.cache`, `context.temporary`, through the one `AccountWipe.configure` the module and a test both call) as a first pass.
  4. **A launch whose outcome is `stuck`** (native could not finish) ENDS the session: no world, no sweep, the card says "close and reopen" and NAMES what is left (`endedWipe` plus the folder names native reported); the next launch tries again. **`locked`** (the Keychain cannot be read, as before the first unlock) touches NOTHING, keeps the mark, and the card says "unlock your phone, then close and reopen" (`endedUnlock`): chosen over treating it as a failed wipe, so a phone that is merely locked is never stranded and the wipe never acts on a guessed key state. A process that showed the card never opens a world again. No retry button, no background finish, no "finishing" state.
  5. **Close button.** None. The card has no control; an iPhone app that terminates itself reads as a crash and the platform guidance is to leave it to the person (the owner has said sign-out may stop the app; this is a one-line change if they want it: `exit(0)` behind the card's button).
  6. **Removed, and why:** the seal and `isSealed`, the seal file and its retry loop, `AccountWipe.url`'s sealed branch, the join deadline and the per-run progress records, the serial `run()` and `register`/`registerStandard`, the second wipe in `signOutForShared`, the in-process run from `signIn.onSignOut`, the background finish, `finishing`/`finishFailed`/`lockedWipe`/`lockedClearing` and the card's retry for a stuck wipe, the in-process `rmTree` of the account folders (the launch sweep removes them; a signed-out launch also removes `app:/tmp/picked`). Kept: the one account folder (`AccountWipe.url`), the positive-key voice move, the three-state key read, the ending mark, B1's tests (an ended process never opens a world), the Keychain door and the test isolation.
  7. **The revoke comes before the key goes (2026-11-21, round 7).** The tap revokes the key on the server (bounded, 8 s) while it is still in the Keychain, THEN removes it (`ChoirSignIn.signOut`); a kill leaves the key for the launch wipe, which revokes (bounded, 6 s, blocking: nothing else exists yet) before it removes (`AccountWipe.finishAtLaunch`). A revoke that cannot be made (offline, refused) never keeps the key on the phone (the person signed out): the key is removed anyway and the outcome is named: the tap answers `revoked: false` (logged by shared code and in the run log), the launch answers `unrevoked` (logged). An already-revoked key (401) or an unknown one (404) counts as revoked. Server side, read and not changed: device-login keys are created with `expires_at` NULL (`internal/auth/device_login_store.go:236`), so an unrevoked key has NO lifetime; it lives until revoked (the owner can revoke it from another signed-in device or `choir`, `DELETE /auth/api-keys/{id}`). Kill points: K0 after the mark, before the revoke: the launch revokes then removes. K1 after the revoke, before the key delete: the launch revokes again (401, counts as revoked) then removes. K2 after the key delete: no key, nothing to revoke; the tap's own outcome was logged. K3 before the mark: nothing began.
  8. **A wipe that is not clean builds nothing, and the half-built module still answers.** `AutoradioModule.init` returns right after `finishAtLaunch` answers `stuck` or `locked`: no console log, store, runtime, SpeakerGate or voice, and `AutoradioModule.current` stays nil so no lab op reaches a store. Exact still calls the package, so every entry answers a declared shape and the ended card draws: `later("choirAuth")` and `later("choirPlayer")` answer the Kit defaults (checked against the interface's declared substitutes by a test), `later` of any other name answers `NotSupportedError`, `view("web-page")` needs no store, `foreground` only schedules a change notice, and the lab module's `launchWipe` answers from a static, `notready` until the wipe has run. **What the wipe lists:** the account folder, the old paths, the extra paths and the shared account roots from Exact's own context (`context.data`, `context.cache`, `context.temporary` `accounts` folders and `picked`); the mark is cleared only when ALL of it is gone.
  9. **THE TRADE this rule makes, plainly:** between the tap and the next launch the account's files REMAIN on the phone (only the key is gone and the session ended); a person who signs out and never reopens the app leaves them on disk until they do. In exchange nothing is wiped in a running process, so no late writer, cached path or double run can leave account data that a "clean" answer missed. **Unproven:** the native read of the mark at `Application Support/exact/<app id>/data/ending` rests on native-modules.md's statement of Exact's `app:/data` mapping; it has been tested only against a temp folder and is unproven on a real install until drill 2c runs (its assertion: a planted mark with the key present is finished by the launch and the account's native stores are gone).
  10. **Dead code removed with it (round 7):** the stores' own `wipe()` methods (KeptTurnStore, OpenPageFacts, TakeOwn, TakeMutes, TakeStore, NotebookCache, NotebookStore, Deck and its journal, BarTracker, HeldPages, TextureEditor, MailReading, ClipLocator, HeldRest, RadioBridge), `ChoirEnvironment.wipePlaying`, `TakeMirror.invalidate`, `RetrievalBridge.clear`, `EditionPicksMirror`'s wipe use, the `AccountWipe.generation` guards (Notebook's topics write, HeldRest.put, the onboarding setup and topics calls) and the `floor` of OpenPageFacts and TakeMirror, with the tests that only exercised them. Kept because production or test fixtures still reach them: `EditionTalk.clear` (the onboarding page), `AckHistory.clear`, `TurnHistory.clear`, `Conversation.clear` and `EditionPicksMirror.reset` (reset hooks the test fixtures call on the shared singletons), `SpeakerGate.forget` ("Forget my voice"), `OnboardingController.reset` (the page's `obReset`).
  11. **What the tap itself still does, named (round 8):** `ChoirApp.signOut` drops the take in hand (`TakeSession.discard`, which deletes its scratch recording) before the pause; and `AccountWipe.url` removes the OLD voice files when the key is positively absent (they belong to no one signed in). Neither removes the account folder; both are deletions in a running process and are claimed here, not away.
  12. **Residuals, with their conditions.** (a) The Keychain changing under an open account ends the session with the `changed` card and writes NO mark: native files are not partitioned by account until station 2, so after a changed key the new account's native stores can show the old account's words until station 2 lands; the shared roots are partitioned and swept. (b) A key removed while offline and never revoked stays valid on the server (device-login keys never expire, `internal/auth/device_login_store.go:236`); exposure is an old device backup or an extracted Keychain item. The launch wipe cannot retry the revoke later from the key's ID alone: the server's `DELETE /auth/api-keys/{id}` is authenticated by a credential (the key itself, another key with `manage:keys`, or a passkey session, `internal/auth/handlers.go` `HandleRevokeAPIKey`), so retrying would mean keeping the key, which is what signing out removes; the owner revokes it from another signed-in device or `choir`. (c) The launch wipe blocks the main thread up to 6 s when it must revoke offline; to be measured in drill 2c. (d) The tap's revoke is stub-tested only; `AccountWipe.revoke`'s real network path is untested.
  13. **The bake and non-phone hosts (2026-11-21, after the first native build failed at the bake).** The build's bake evaluates constant data answers with no native module and no account; a gated answer that threw `AccountClosedError` there failed the bake. The gate now calls a bare `exactHost()` (`markNotBaked`), which THROWS at the bake and so leaves the answer unbaked (the first attempt caught the throw and failed the bake again), so no account-gated answer is evaluated at build time. The same fact separates the hosts: a phone (`native`) never reads a missing or unrecognised `launchWipe` answer as "no sign-out pending" (it asks until the deadline and stays LOCKED, not ended); a host that is not a phone (`web`, or no fact at all in a test) whose module does not know the op has no launch wipe (`nomodule`) and opens as before. Gated answers still refuse with a thrown `AccountClosedError` at runtime (the contracts' `else empty()` fallbacks and the card tell the person); that is not changed.
  14. **(SUPERSEDED by THE LAUNCH VERDICT below, which keeps its `exactHost()` and bake facts) RULE A, one gate (2026-11-21, round 9).** Shared code opens NO world, not even the anonymous one, until native has answered `launchWipe` with something other than `notready`. **Which Exact fact distinguishes a phone from a host with no native module:** `exactHost()` (`app.contract.d.ts`: `'web' | 'native'`, no grant), read through `api.ts` `hostKind` (tolerant). Separately, the gate calls `markNotBaked()` (a bare `exactHost()` that is allowed to throw): at the bake that call throws and the bake leaves the answer unbaked; it must never be wrapped in a try (a swallowed call failed the bake twice). On `native` a missing handle (the app module is loaded after the first frame), an unrecognised answer or no answer is NEVER "no module" and never "no sign-out pending": the launch stays LOCKED ("Starting Autoradio.") and asks again, with no timer deciding anything. Only a host that is not native by that fact (`web`, the web test host, a test with no fact at all) has no gate and opens as before. A real device cannot take the no-native branch because `exactHost()` is `native` there by construction of the host, not by a probe or a timeout; the bake evaluates no gated answer at all (not baked) and, were it to ask, is not native. A gated answer for a closed boundary still refuses with a thrown `AccountClosedError` at runtime; the card and the contracts' `else empty()` fallbacks tell the person.
  15. **(SUPERSEDED by THE LAUNCH VERDICT below, which keeps its `exactHost()` and bake facts) Keychain failures, named.** `errSecInteractionNotAllowed` (locked before the first unlock) is `locked`: the card says unlock your phone and reopen (the next launch finishes). Any other Keychain failure is `fault`: nothing is touched, the mark stays, and the card says plainly that signing out could not finish and that deleting and reinstalling Autoradio clears it (nothing on the computer is affected). **Residual:** a fault that never clears leaves reinstalling as the only way out; the card does not re-ask on foreground (reopen it).
  16. **(SUPERSEDED by THE LAUNCH VERDICT below, which keeps its `exactHost()` and bake facts) Native refuses a sign-in while the mark exists** (`ChoirSignIn.begin`), so no key can appear under the ended card for the launch wipe to delete (tested). The launch wipe and the revoke run OFF the main thread; the module builds nothing until they answer, so the first frame is answered with the declared substitutes and re-asked (`touch`) after the build. `later` itself lives in the app target (ExactPackage), which the Swift package cannot compile, so it is checked by reading and by the native build, not by a package test; the declared substitutes it answers are checked against the interface by a package test.
  17. **Every data function that takes `acctHandle`, and what a closed boundary does to it (2026-11-21).** All are account-gated through `answer`, and every one has `else empty()` (or a declared default) in its contract, which is the declared empty value at runtime: mailBox, mailFrame, mailSync, mailMessage, mailCompose (transient), launcherData, editionPicks, paperFront, fileList, fileOpen, importedList, importedDoc, importedSection, importedFind, importedLocate, showClips, storyClips, readerDocShared, texturesList, notebookPageShared, onboardingPage, editionTopics, takesPage, podcastPlayhead, podcastHome, podcastShow, podcastEpisode, queueView, radioPageFacts, radioPage, retrievalServe, paperSearch, themeCards, themeExtras, settingsAccount, settingsComputer, learnedInfo, activityPage, aboutPage, conversationInfo, pageRequested, pagesView, theme, themeList, radioPulse, barRun, barInfo. A closed boundary at RUN time throws `AccountClosedError`, the resource shows its `else` value and the card tells the person (a normal state, not a defect: that is the existing design and is unchanged). At the BAKE none of them is evaluated, because each is left unbaked by `markNotBaked`; the earlier failure was the swallowed call, not the throw. Tested by `app-gate.test.ts` B0 for every gated source.

- **SUPERSEDED BY "ROUND 12: THE KEY IS EXACT'S" at the end of this file (the verdict machine below is deleted from native; the shared side below stands).** **THE LAUNCH VERDICT (2026-11-22, round 10; `docs/problems/account-boundary-nine-review-rounds-structural-assessment-2026-11-22.md`).** A process belongs to ONE account for its whole life.
  1. **Native makes one verdict, once, and only when the phone's protected data is readable** (`AccountWipe.decide`, off the main thread, answered by the `verdict` op in `ModelsEntry`'s `call`). `anonymous`, `account(digest)` (the key's SHA-256: the root name), `blocked(why)`, and `ended` after a sign-out. Making it includes finishing a pending sign-out: the key is revoked (bounded) then removed, the account folder, the old paths, the extra paths and the shared account roots (Exact's own `context.data`, `.cache`, `.temporary` `accounts` folders, `picked`) are removed and LISTED, and native removes the mark only when that listing is clean. Anything left, a Keychain fault, or a mark that will not go is `blocked`, and the mark stays.
  2. **`notready` is answered at once and never cached** when any one of three facts fails, behind one seam (`AccountWipe.Readiness`): `UIApplication.isProtectedDataAvailable` (UIKit state, read on the main thread), a Keychain read that is not `errSecInteractionNotAllowed`, and the mark read by reading the FILE (before the first unlock metadata can be visible while contents are not, which is why `fileExists` is not used). Each ask that finds no verdict also starts the decision once; there is no timer and no loop. Exact fails a resource that overruns its turn and asks it again at 250 ms, then 1 s (`docs/reference/exact/internals.md`), so a native call cannot hold its reply: one call per ask, none once the verdict is known.
  3. **Shared code holds the verdict and never reads the key or the mark to decide anything** (`api.ts` `askVerdict`; `account.ts`). Launch: ask; `notready` is LOCKED ("Starting Autoradio."); `blocked` ends the session behind the ONE blocked card, which promises nothing unproven; otherwise the old pre-partition leftovers and orphan roots are swept FIRST, and only a sweep that finished lets any world open, the anonymous one included (a failed sweep is `blocked`). Those leftover files (`LEFTOVERS` in `account.ts`: `app:/data/takes.json`, `recent-files.json`, `paper-front.json`, `podcasts-library.json`, `podcast-feeds`, `notebook-cache.md`, `vectors`, `content.db*`, `mail`, `open-pages-shared.json`, `account-stamp`, `signout-pending`, `what-i-told-the-paper.json`, `recent-pages.json`, `steer-queue.json`, `runsheet.json`, `imported-places.json`, `edition-topics.json`, `onboarding.json`, `exports`, `app:/cache/files`, `app:/cache/mail-*`) are not the signed-out account's data: they are files of builds before the partition, no current store writes them, and the sweep removes them at every launch whether or not a sign-out was pending. So the mark depends only on what native removes and lists.
  4. **Two transitions, both one-way, both made by the code that performs them.** T1, `anonymous` to `account(digest)`: `AccountWipe.store` runs the key store under the lock that guards the verdict, checks `anonymous` immediately before it, leaves `anonymous` if the store throws, and makes the verdict `account` in the same critical section; the sign-in poll also checks cancellation before it, and `ChoirSignIn.begin` refuses unless the verdict is `anonymous` and no mark is pending (`AccountWipe.markPending`, also checked inside `store`). Shared code learns T1 from the card's tick (`au.signedIn`) and asks the verdict once more. T2, `account` to `ended`: `AccountWipe.end()` at the start of the native sign-out, so a poll already in flight cannot store a key; shared code ends the session and writes the mark first as before. Nothing else changes the state. A key changed by any other route is not looked for, because nothing in the app can cause it; **cost:** signing in while signed in to another account needs the app closed (owner, 2026-11-22), and a key removed by something outside the app is noticed at the next launch.
  5. **Deleted:** `keyFact`, `keyState`, the key check in `settle`, `peek`'s re-check, the `changed` ending, the fault/locked/unlock/wipe card split, the unbounded `wipeFact` poll, `finishMark`/`leftover`, the `nomodule` timer branches. The sentence in `account.contract` that said a page change re-asks the card is corrected (it never did).
  6. **What this does not cover, named.** Android and a native host without the app module: `exactHost()` says `native` there too, so such a host stays on the Starting card until the Android station gives it a module or a host fact. The theme sources read the open account world only (no changed-key window remains). The speaker profile and the voice choice are still moved into the current key's root by `AccountWipe.url`: native stores are not partitioned by account until station 2, which will key them on the verdict's digest. A persistent Keychain fault is the one `blocked` card ("Unlock the phone and open Autoradio again"), which may not clear it; reinstalling is NOT promised (iOS Keychain items commonly survive deletion). **Owner check:** only a real phone before first unlock shows the true protected-data case (drill 2d is a stub and fake-filesystem test here and an owner check there); `isProtectedDataAvailable` against Exact's file-protection class and the `context.data/ending` mapping (drill 2c) are unproven until run.
  7. **Round 10 point fixes (2026-11-22).** An unconfigured process (`AccountWipe.markURL` not set: the package init can run after the first `verdict` ask) is NOT READY, never "no mark", and `configure` sets `markURL` last. `decide` re-reads the Keychain after the readiness check: a lock appearing mid-decision is not ready (`blocked` is for a fault on a Keychain that can be read); a key that reads present but cannot be read whole in the mark branch is not ready, never a silent unrevoked delete. `kick` assigns a verdict only while none exists, and `end()` is final on the native side (a decision in flight when sign-out is tapped does not replace `ended`). A key is not stored while the mark exists (`markPending`, in `mayStoreKey` and `store`): an older or external build cannot write one under a pending sign-out. The simulator-only `ProofKey` import now runs in `AutoradioModule.init` BEFORE the first decision (it writes the Keychain outside `AccountWipe.store`; simulator builds only, compiled out of device builds), so drill results are not a race. Shared code: `settle` re-checks the session after the launch; the gate refuses a world that closed while it waited before calling the source; the host with no native module sweeps too before its anonymous world opens (the invariant is one: the sweep finishes before ANY world opens); the unknown re-read is bounded (twenty re-reads); `nativeInert` does not deliver a call for a closed world and the deck's default port is that call (a radio source in flight at sign-out cannot restart the deck); the stuck card no longer promises a reinstall. **Residual, named:** a native account folder left on the phone with no key and no mark (the key removed by something outside the app) is shown to nobody by shared code, but native stores are not partitioned by account until station 2, so native arguments of a gated source in the anonymous world can still carry those words.
  8. **Round 11 (2026-11-22).** The world-closed watcher leaves no reaction per answer: each gated answer registers one callback on its world (`World.watch`) and removes it when it settles, `close` drains the rest (tested: forty answers leave zero). A sign-out tapped on the locked or blocked card when no module was built now does what its words say: `AccountWipe.signOutWithoutModule` (called from `ModelsEntry`'s `signOut` when `AutoradioModule.current` is nil, off the main thread) revokes if it can, removes the three Keychain accounts and ends the verdict, so a key that reads present but never reads whole is removable. The gate's own pre-check of a world that closed between `enter` and the source call has no direct test: there is no await between them to hold open, so no seam exists short of changing the code; `settle`'s session check and the world-closed callback are tested. Raw native calls reachable after `endWorld`: `contentBusy` (content-run.ts) is listed in the I2 inventory as a device fact (whether remote audio is on air: no account data crosses), and the Keychain item read behind `keptComputerId` now goes through the inert call (not delivered for a closed world). **The proof-key import (simulator only) and the first verdict:** in the first drill run (83297a3ed) the import ran in `build`, after the verdict, so the proof key could never be there at decide time (confirmed: `launch verdict: anonymous` on every launch, Settings signed out). Moving it to `init` (1cc0983ec) did not change the drill; the cause of that is NOT established by the logs (they carry no import line). What is ruled out by them: a blocked/fault verdict (the log says anonymous, so the Keychain answered item-not-found, not "not available"), a stale mark (a mark would have been cleared by the first launch and later launches would show the account). The import now runs before `configure` and logs its outcome (`none`, `ignored`, `refused: <status>`, `stored`) and the Keychain state after it, and the verdict line carries the Keychain and mark state, so the next run names the cause.

### Round 12: the key is Exact's (2026-11-22; what is built)

The owner pointed out that Exact itself keeps secrets in the Keychain (`secret.keep`: `reference.md:6417-6436`; Apple `AfterFirstUnlockThisDeviceOnly`, `vendor/ibex/crates/ibex2/src/secrets/darwin_keychain.mm:7-8`; Android Keystore-wrapped files), so the sign-in key never needed to be native. "Yes, i want to move everything to exact that can be. signing in again is fine." Eleven rounds of review defects were all about native deciding what shared code needed to know; moving the key moves the decision.

1. **The verdict is shared code's** (`account.ts` `launch`, `settle`): finish a pending sign-out (the ending mark), then read the key from the answer's store (`api.ts` `adoptStore`), then the markers, then sweep, then open one world. There is no native verdict, no `notready` polling, no readiness seam, no protected-data check.
2. **Absent versus not readable yet.** `store.get` answers null for both (`host/apple/src/store.rs:818-827`: a failed read sets `complete = false` and leaves the name out). The host retries (`activation.rs:203-211`) and, when the snapshot changes, asks again every answer that read the store (`runner/src/runner/kept.rs:653-668` `store_changed`), so the retry is Exact's. The marker `app:/data/signed-in` ("an account was signed in on this install", written before the key by a sign-in, removed by the launch that finishes a sign-out) is the only thing shared code adds: marker set and secret null is STARTING (locked, nothing swept); an unreadable marker is STARTING; after the card's five re-reads (`peek(..., tries)`: a bound in time, not in answers) the launch is anonymous and Sign in is offered, and a key that reads later moves the anonymous world to the account's once (T1). Unverified: Android's behaviour, and the file-protection class of `app:/data` (before first unlock).
3. **Sign-in** is `signin.ts` (device flow with `fetch` through `api.ts unauthedPost`; the key with `store.set`; the approval page by the contract's `openURL` host command, `recipes/functions.md:36`, grammar "host commands"). **Sign-out**: the session ends, native is told to drop its in-memory key BEFORE the ending mark is written, the key is revoked then forgotten (`forgetSession`); nothing is wiped in that process; the next launch revokes (if the key is still there), forgets, has native forget its files, sweeps every root, and removes `signed-in` then `ending` (last). A step that fails is `blocked` and builds nothing (native is never told `ready`).
4. **What native keeps, each with the reason shared code cannot do it** (`ModelsEntry.swift` header): `hello` (is a native module here: a device fact); `ready` (build once the launch is clean); `forgetAccountFiles` (DELETION ONLY of the Swift stores' own files under Application Support, which Exact's `app:/` roots do not reach: `Library/Application Support/Autoradio` is native's, `reference.md` `app:/data` is `Library/Application Support/exact/<app id>/data`; shared code decides and writes the mark); `holdKey` (below); `retireOldKey` (the one-shot read of the old Keychain item under service `news.choir.autoradio.choir`, which Exact's store, keyed by the bundle id, cannot see: read once, revoked on the server, the three items deleted; a failed revoke still deletes and is logged as a state; DELETE when no phone that ran an earlier build is left); voice and the mail cipher as before.
5. **`holdKey` is a stated residual** (lead's ruling, 2026-11-22): the key in memory (`HeldKeys`, an `InMemoryKeyStore`) for the 14 Swift files that still call the server with `ChoirClient` (pages still in Swift: `BasicStages`, `AutoradioDesk`, `ChoirApp`, `ChoirDesk`, `ChoirProgramme`, `ChoirSystem1`, `PodcastClips`, `RadioProgramme`, `StretchCandidates`, `Takes`, `Notebook`, `TextureEditor`, `TextureReaderSource`, and `AccountWipe`'s revoke of the old key). Native never persists, reads back or returns it, and it is cleared at sign-out before the mark is written. A test (`testOnlyTheListedNativeFilesCallTheServerWithTheKey`) fails when a new file starts using `ChoirClient` or a listed one stops, so the list only shrinks.
6. **Station 2 (not in this commit): move those clients to shared code** (the list above, in this order of size: `Notebook`, `Takes`, `TextureEditor`, `TextureReaderSource`, `PodcastClips`, then the Choir programme and desks), then delete `holdKey`, `HeldKeys` and `AccountWipe.revoke`.
7. **The proof key** (simulator drills) is seeded as the secret file `api-key` of the scripted drive's scratch store (`reference.md:6432` "a scripted drive never keeps them"; `host/apple/src/picker.rs:70-76`, `store_secrets.rs:58-65`: `<container>/Library/Caches/exact/<app id>/agent/default/secrets/api-key`), by `proof-real.sh reset_state`; there is no simulator-only code in the app. Not verified: the secrets folder's permission check (0700 folder, 0600 file are set), and that a drive without `--storage` uses the scratch name `default`.
8. **Failure of the old import** (round 11): `SecItemAdd` returned -25300 from `AutoradioModule.init`; the cause (timing before the app finished launching, in the simulator) was never proven, and the import deleted the key file even when refused. The import is deleted with the key's move.
9. **Not covered**: Android native (`holdKey` and `forgetAccountFiles` have no Kotlin side yet; `exactHost()` native hosts without the module stay on the starting card), the Texture/other pages' native stores beyond `forgetAccountFiles`.
10. **First drill run on this design (acct7-real, 2026-11-22).** The account opened from the seeded secret; the sweep and sign-out checks failed. Evidence read: (a) the app log says `account: the secrets could not be forgotten` three times, each at a launch that found an ending mark; a launch whose forget is refused is `blocked`, its card covers the bar, and every later drive fails (`tap bar-mark: node #43 covers its middle`, `the account boundary is closed (ended)`) so drills 1, 2 and 5 failed for that one reason; the host's own refusal words were not logged, so `forgetSession` now tries every name and logs the refusal (names only) and the launch and sign-out log every step reached, and the sweep logs roots listed, removed and each failure's code; (b) the container holds TWO app:/data trees (`Application Support/exact/<id>/data` and the scratch store `Caches/exact/<id>/agent/default/data`): the orphan root of drill 3 sat in the first, which the app of that drive did not read, while the checks and `data_dir` looked at the first one that `find` met, so drills 3 and 4's "orphan not removed" is the script reading a stale tree; `proof-real.sh` now reads and plants in the tree touched most recently and removes the stale one at each reset. Not yet shown: why the host refuses the forget (the next run's log names it).
11. **One owner of the launch: the card.** A launch attempt's storage calls belong to the answer that made them and the host drops them with it. Gated answers only wait for the shared result; the card (`acs`) starts and restarts attempts, and the contract asks it again every three seconds while it has no answer (`task cardUnanswered`, tested), so a dead first attempt can never leave the gated answers waiting for a card that is never asked again.
12. **The cause of the blocked launches (second drill run, from Exact's source): the store exists only in the synchronous first turn of an answer.** `store.get`, `store.set` and `store.forget` call the host door with the answer's store; after any other await the continuation has none and the door answers `no store here, in background work or the continuation of an answer that was let go. Read the store before the answer's first await. A write that must stand goes after a request with effects (a POST), whose continuation keeps the store` (`js/src/door.rs:45-70` the three doors on `state.between_answers`, the message at `:366`; the store pointer is set only in `module_work.rs:182`, `lib.rs:1029` and `resume.rs:129,353` for the turn being run and cleared at `lib.rs:865,1080`, `resume.rs:196`). Forgetting an ABSENT name is NOT an error (`runner/src/store.rs:558-577`: it records the write and leaves the revision), so the unseeded `key-id` and `computer-id` were never the cause. The launch read the key (`adoptStore`) and forgot the secrets AFTER awaits (the marker listing, the revoke, native, the sweep), so on the real host `get` failed silently (the key came only from a later answer's first turn) and `forget` threw, the launch blocked, and every later launch found the same mark. The fake store answered from anywhere, which is why 1,166 tests passed. **Built now:** every answer calls `turn(store)` as its FIRST statement (`account.ts`: it reads the key and performs the forget a previous turn asked for); a launch or sign-out that has to forget sets `needForget` after the revoke that must come first, and the NEXT answer that has the store (the card is asked again at once by the sign-out's result, and every three seconds while locked) forgets and the launch goes on; a sign-in stores the key in the continuation of its POST before any other await. The test fake (`account-phone.ts fakeStore`) now behaves as the host does: it throws outside `turn(...)` and after a storage call, keeps the store only for the continuation of a POST/DELETE (`post(...)`), and treats forgetting an absent name as fine; K10 pins those rules, and reverting either fix fails S1-S3 and K5. Unverified: that a DELETE (the revoke) counts as "a request with effects" on the host (only the documented POST is relied on).
13. **A proof run starts clean.** `proof-real.sh` removes both app:/data layouts, the scratch store and the leases at the first reset of a run (an ending mark of the last run made every launch of the next one finish a sign-out), keeps the in-use tree between drives (the kill and relaunch drills depend on it), and no longer runs the old `rm -f .../exact/*` that failed on a directory every drive. `proof-common.sh run_drive` saves the app's `account:` console lines beside each drive (`drive-<name>.account.txt`, from the simulator's own log) and prints them when the drive fails.
14. **Third drill run (acct10-real): native was never built in the drives that opened the account.** Evidence: `AUTORADIO models registered` (native's build) is in the console of the drives that launched anonymous (`big`, `killplant`, `relaunch`, the first) and in none of `warm`, `survey`, `read`, `bar`, `drill`; the failing checks are exactly the ones that need native (the deck: `radio: hold: 4 ahead on the sheet, the deck holds 0, playing nothing` 34 times; the voice: `reader: the voice did not take the page`; the takes programme copy; the first-run page's voice facts); and the sweep line `kept the open account` shows the launch did open the world. Cause: native is told `ready` and handed `holdKey` by the launch attempt that OPENS the world, and an attempt whose answer the host dropped (item 11) dies after `openWorld` and before those calls; the attempt that restarts finds the world already open and returns without them. Now `ensureNative` (`account.ts`) sends `ready` and `holdKey` once each from whichever answer first sees the world open (K11), and retries while the native module is not usable. Stale checks fixed in `proof-real.sh`, each with its reason: drill 1 taps the ended card itself (the agent refuses a tap on a covered control, "node covers its middle", which ends the drive; the refusal is the proof the controls under it cannot be reached); drill 5 reads the roots after a relaunch (a sign-out wipes nothing in its own process; the roots go at the next launch); the edition-picks file is looked for in the account's root (`data/accounts/<digest>/`, where the partition put it). Not established from the log: the paper's `Offline` line in `survey` and the theme default under a dark system (both could follow from native not being built; the next run says).

