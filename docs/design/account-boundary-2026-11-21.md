# The account boundary in shared code

Written 2026-11-21 for the defect in
[docs/problems/unclean-sign-out-leaves-the-old-accounts-data-for-the-next-2026-11-21.md](../problems/unclean-sign-out-leaves-the-old-accounts-data-for-the-next-2026-11-21.md).
A design, not code. Nothing here is built. Mutation class of every landing below: red
(auth and session).

## Summary

The phone must never show one account's private data to another. Today shared code
settles the account once, remembers the answer, and lets every page read its stores
whatever that answer was. Two attempts to fix it changed what is remembered. Both were
sent back by both review seats, because a page still read its store when the settle
had not finished cleanly: the gate fails open. The lead's belief is confirmed by the
code: on main and on `579ceb2db` alike, `app.ts:135` awaits the settle and throws its
result away.

This design stops remembering. The boundary is not a step that runs once. It is a
fact that holds or does not hold, and it is worked out again from two durable facts
before every page answer:

- **the key**: the sign-in key in the Keychain, read through the app module (absent,
  present with its SHA-256, or unreadable);
- **the record**: one small file, `app:/data/account-stamp`, holding the SHA-256 of the
  key of the account whose data the phone's stores hold, or not there at all.

A page may read or write account data in exactly two cases: the record and the key name
the same account, or there is no record (the stores hold no account's data). In every
other case the page source refuses, the account card says why on every page, and the
one writer of the boundary works to put it right: it wipes every store and then
removes the record. No state is remembered between answers except the record (on disk,
mirrored in memory by its only writer) and the account generation (memory only).

What changes in practice:

1. **The marker file goes.** A sign-out is "delete the key, then settle". A key that
   is gone while the record still names an account is itself the proof that a wipe is
   owed. Late answers from the native side need no handling: the next answer
   re-reads the facts.
2. **Every page source is gated** in `app.ts`, by a short list of sources that read no
   account data. A new source is gated unless someone adds it to that list.
3. **Sign-in is offered only when the phone holds no account's data.** A refused key
   offers "Clear this phone" first, not "Sign in".
4. **Native stores take the generation they are given** with each wipe, so no native
   guard has to guess it.

The first landing is shared code only. It closes the cross-account read, and it makes
more pages refuse while a wipe is owed. Native follows.

## 1. The frame

Three lenses changed the design. The rest of this document follows from them.

**What the boundary is.** It is not a procedure ("settle at launch") and not a memo.
It is a relation between two things with different writers: the key (written by the
Swift sign-in, deleted by the Swift sign-out) and the data (written by a dozen shared
and native stores). The relation holds when every byte of account data on the phone
belongs to the account whose key is stored, or to none. Both attempts tried to
remember whether the relation held. A remembered answer goes stale the moment either
writer acts, and the key's writer is native code that shared code cannot see. So the
relation is checked, not remembered.

**What can be derived need not be stored.** The second attempt wrote a marker and a
list of stores still to wipe. Each extra durable fact is a new way to be inconsistent
(an unknown name in the list looped forever; a failed marker removal was taken as
clean). The record plus the key already say everything needed: a record naming A with
A's key is open; a record naming A with no key or with another key means A's data must
go; no record means nothing to protect. The marker existed to say "a sign-out was
under way"; the absent key says that better, because it is true whatever crashed.

**The physical bounds.** Anything can fail: the Keychain delete (its status is
ignored, `ChoirClient.swift`), the native wipe (hangs on the main actor, or a store's
file removal is swallowed by `try?`), a shared file removal, the record write, the key
read itself (no deadline today), a crash at any instruction, a native answer arriving
after its deadline. The design does not try to make these succeed. It makes each one
leave a state that refuses, and makes every refusing state heal by doing the same
idempotent work again.

**One mechanism, several jobs.** The account generation (a number in memory, moved by
every sign-out and every wipe) already drops replies that started under an older
account. The same number now also (a) ends a page answer that started before a wipe,
(b) is the floor every native store refuses below, carried to native code with the
wipe, and (c) tells a late native answer which wipe it belongs to.

## 2. The real states

There are three values of the record and three of the key. Nine cells, five states.

| record \ key | absent | present, digest d | unreadable or no answer |
| --- | --- | --- | --- |
| none | EMPTY | ADOPT, then OPEN | EMPTY |
| names d | CLEARING | OPEN | LOCKED |
| names another, or not a digest | CLEARING | CLEARING | LOCKED |

- **OPEN**: the record names the stored key. Pages read and write.
- **EMPTY**: no record. The stores hold no account's data, by the rule that the record
  is removed only after every wipe succeeded (section 4). Pages read and write;
  anything written now belongs to whoever holds the phone, not to an account.
- **ADOPT**: no record and a key. The writer writes the key's digest as the record.
  Until that write succeeds the state is closed; once it succeeds it is OPEN.
- **CLEARING**: the record names an account whose key is not stored. Its data must go.
  Pages refuse. The writer wipes everything and then removes the record.
- **LOCKED**: a record exists but the key cannot be read (before the first unlock, a
  bridge error, a key read past its deadline). Pages refuse, because the owner cannot
  be confirmed. Nothing is wiped.

A record that is not exactly 64 lowercase hex characters, or a record file that exists
but cannot be read, counts as "names another". It is never read as "none": the second
attempt read an empty or garbled record as none and adopted over it (seat 2, round 2).

The 36-row table of the second attempt was the wrong resolution. Its marker and its
list were state the design had added, then had to keep consistent. Its sign-out "in
flight" rows are not states here: during a sign-out the facts are read like at any
other time, so before the key is deleted the phone is OPEN for the leaving account and
after it is deleted the phone is CLEARING.

## 3. What a page source may do, and what the person sees

`app.ts` `answer` asks the boundary before every source that is not on the exempt list.

```
const gen = accountGeneration();
const b = await boundary.check(storage);          // derives the state; may run ADOPT or start a wipe
if (!EXEMPT.has(source) && !b.readable) throw new AccountClosedError(b.state);
const out = await sources[source](...);
if (!EXEMPT.has(source) && accountGeneration() !== gen) throw new AccountClosedError('changed');
return out;
```

`readable` is true in OPEN and EMPTY only. The second check ends an answer that began
before a wipe (it may hold data read from a store that was wiped while it ran).

**The exempt list** names the sources that read no account data and no signed-in
route: `accountState`, `accountSignOut`, `hostCaps`, `aboutBuild`, the theme sources
(theme files are device settings today and are not in the wipe list), and the sources
that are pure functions of their arguments (`barInfo`, `radioPage`,
`conversationInfo`). The list is an allow-list in `app.ts`; a test fails when a source
on it imports a module that touches a wiped path or a signed-in request. Everything
else, the paper front included (its cache is in the wipe list), is gated.

A refused source throws. The page shows its existing failure state (`else empty()` or
its error view); no page gets new words. The words are in one place: the account card
(`accountState`, `account-state.ts`), which is exempt, reads the same boundary, and is
shown on every page by `bannerOn` and on Settings > Account.

| State | Pages | The card says | Action offered | How it heals without a relaunch |
| --- | --- | --- | --- | --- |
| OPEN | work | nothing (or "rejected" after a 401) | Sign out; after a 401, "Clear this phone" | — |
| EMPTY, key absent | work (public pages, signed-out words) | "You are signed out." | Sign in | — |
| EMPTY, key unreadable | work | "Unlock your phone to continue." | none | unlock; the card's 3 s re-read |
| ADOPT failing | refuse | "This phone could not record the account. Trying again." | Try again | next answer or tap retries the write |
| CLEARING | refuse | "Clearing the previous account's data from this phone." | Try again (after 5 failed tries) | each answer and each 3 s re-read starts a wipe if none is running |
| LOCKED | refuse | "Unlock your phone to continue." | none | unlock; the 3 s re-read |
| LOCKED by a bridge error | refuse | "This phone's sign-in cannot be read right now." | Try again | the 3 s re-read (today "unknown" shows nothing; it must say something now that pages refuse) |

The 3 s re-read already exists (`account.contract`, `rereadLocked`). It gains
`clearing`, bounded like `rereadRejected` to five tries, after which "Try again"
appears and restarts the count. The re-read is local file work and one key read, not a
model call; the bound is there so a store that can never be removed does not spin
while the person looks at a card that cannot change.

**If it cannot heal.** A store whose file the system refuses to remove keeps the phone
in CLEARING. The card then names the store ("could not clear: takes") and says: "Delete
and reinstall the app to finish clearing. Nothing on your computer is affected." A
reinstall removes the app's container; the Keychain survives it, so the phone comes
back in ADOPT with empty stores and the same account. Signing out is never blocked:
`accountSignOut` is exempt in every state.

**Sign-in is offered only in EMPTY with the key absent.** That closes the way in: no
second account can sign in while the first one's data is on the phone, whatever
failed. The refused-key state ("rejected") today offers "Sign in" over the old key
(`account-state.ts:21`), which replaces key A with key B and leaves A's data; it now
offers "Clear this phone" (the sign-out), and sign-in follows from EMPTY. If a key
appears anyway (the device flow started in EMPTY completes late, or anything outside
this design writes the Keychain), the table still holds: a record naming A with key B
is CLEARING.

**Cost.** One key read per gated answer. Today `keyState` is two native calls and a
SHA-256 in shared code (seat 2, round 2). The first landing gives `keyState` a deadline
(past it, the key is "unreadable" and the phone LOCKED, never open) and logs the count
of key reads so a spike is seen. Landing 2 has the app module answer `keystate` with
the digest in one call, so the key never crosses the bridge for a check.

## 4. One writer, and the order of durable writes

`account.ts` is the only writer of the record, the only caller of the shared wipes and
the native wipe, and the only place the generation moves. Its work runs one job at a
time on one promise chain: a job that arrives while another runs waits for it. There
are two jobs.

**Settle.** Read the facts, then:

- OPEN, EMPTY, LOCKED: nothing to write.
- ADOPT: write the record (the key's digest). On failure the state stays closed.
- CLEARING: one wipe attempt:
  1. move the generation to g (drops every reply and write begun before);
  2. ask native `{op: 'wipe', generation: g}` under the deadline. If a native wipe from
     an earlier attempt is still running, wait for that one instead of starting
     another (two wipes would queue on the main actor);
  3. run every shared wipe, each on its own, then look for survivors of every wiped
     path (as `leftovers` does today);
  4. only if native answered done and nothing survived: remove the record, and check
     that it is gone;
  5. read the key again: present means ADOPT, in the same job.

A check that finds CLEARING starts a settle if none is running and waits for it up to
the deadline; it never waits for a second one. So two answers that arrive together see
one wipe, and neither reads while it runs.

**Sign-out** (the person's tap):

1. move the generation and stop sealing (`beginSignOut`, as today);
2. ask native `{op: 'signOut', generation: g}` under the deadline: delete the key, run
   the native wipe at g;
3. run a settle.

The result follows from the facts after step 3, not from the native answer: key absent
and record removed is "signed out"; key absent and CLEARING is "signed out; clearing";
key still present (the delete failed, or the native call is still running) is "Sign-out
did not finish. Try again." with the phone OPEN for the same account, its native data
already wiped. A native answer that arrives after the deadline changes nothing: when
the key goes, the next answer finds CLEARING and wipes.

**A wipe never deletes a key.** Only the person's sign-out does, at step 2. A settle
that wipes after a relaunch, or with another account's key present, keeps the key.
Main today calls the idempotent native `signOut` from the launch settle, which deletes
whatever key is stored; seat 1 (round 2, finding 3) showed it could delete a key B that
arrived in the window.

**Ordering of the cases the reviews raised.**

- Two settles: one job; the second waits and then finds nothing to do.
- Two sign-outs: the second waits for the first, then deletes an already absent key and
  settles; the native sign-out is idempotent.
- A sign-in during a pending sign-out: not offered (CLEARING shows no sign-in). If a key
  B appears, the record still names A, so the settle wipes and then adopts B. B has
  written nothing in the meantime: its pages were refused.
- A late native answer: no continuation code at all. It may finish a native wipe that
  an attempt is waiting on (step 2 waits for the running one), which is the only use it
  has.

**Durable order.** The record is the only durable fact shared code writes.

- It is written (ADOPT) only when there is no record, so it never names an account
  whose data was not already the phone's or nobody's.
- It is removed only after every wipe succeeded and native answered done.
- It is never rewritten from one account to another. An account change is CLEARING
  followed by ADOPT.

**The second attempt wrote the new account's stamp before wiping. That is wrong; the
reverse is right.** Writing "B" first labels A's data as B's for as long as any wipe
fails, and every reader that trusts the label reads A's data as B's (seat 2, round 2,
B1). The label must claim ownership only when it is true: remove the old record after
the wipe, write the new one after that. The second attempt wrote first because it
needed somewhere durable to keep "wipes still owed". Here that fact needs no file: a
record naming A with a key that is not A is the debt.

**A crash at each point, and what the relaunch reads.**

| Crash after | Record | Key | Relaunch state | What happens |
| --- | --- | --- | --- | --- |
| sign-out tapped, before native | A | A | OPEN | nothing happened; still signed in |
| native deleted the key, before its wipe | A | absent | CLEARING | wipe, remove record, EMPTY |
| native wipe, before the shared wipes | A | absent | CLEARING | the same (wipes are idempotent) |
| any shared wipe | A | absent | CLEARING | the same |
| the wipes, before the record removal | A | absent | CLEARING | the same |
| the record removal | none | absent | EMPTY | — |
| a launch that found key B, mid-wipe | A | B | CLEARING | wipe, then ADOPT B |
| the record removal, before ADOPT B | none | B | ADOPT | write B |
| a native delete that silently failed | A | A | OPEN | the card said "did not finish"; the person signs out again |

There is no crash point at which a relaunch reads OPEN or EMPTY while another
account's data remains.

## 5. Native stores take the same boundary

Native code keeps its own counter today (`AccountWipe._generation`, bumped by `run()`),
and stores in other lanes guess the boundary from it: `HeldPages` set its floor to
"the last generation seen, plus one" (reader review r4) and `TakeMirror` to a floor
relative to the last push it accepted (takes reviews r6, r7). A floor that is a guess
can sit above the generation the still signed-in account uses, after a failed
sign-out, and then every push is refused for the rest of the process with no way to
heal.

The rule: **there is one generation, and shared code writes it.** Native code only
receives it.

- `{op: 'signOut', generation}` and `{op: 'wipe', generation}` carry the shared
  generation (`ModelsEntry.swift`). `signOutForShared(keepKey:generation:)` passes it to
  `AccountWipe.run(generation:)` (`AutoradioModule.swift:109`, `ChoirSignIn.signOut`).
- `run(generation: g)` sets the native generation to the larger of its current value and
  g, never anything else, before it calls the stores. `bumpGeneration()` is deleted.
  The native writers that already capture `AccountWipe.generation` (19 sites) keep
  working unchanged and now use the shared number.
- Each store's floor is the generation of the latest wipe it received. A push or write
  carrying a generation below the floor is refused; one at or above it is accepted.
  `HeldPages.lastSeen` and the seen-relative floor of `TakeMirror` go.
- So the native floor is always a generation shared code has already reached. After a
  failed sign-out the account keeps working at that same generation, and its next
  push is accepted. A late wipe at an older generation cannot raise the floor past a
  newer one, because the floor only takes the larger value.
- The native answer says done only after every registered wipe has returned, and
  carries the generation it ran at. A native wipe that never returns keeps the phone in
  CLEARING (section 4, step 4), which is fail-closed for the shared side.

What this does not yet close: native code reads its own stores without asking shared
code. If the native wipe has not finished and a new account's voice turn starts (a
launch with key B, before shared code's first answer), the voice could read the old
account's turn history or held page. Landing 3 closes it with one gate at the voice's
entry points (`Conversation.begin`, which the problem record notes has no generation
guard, and the held-page read): native code refuses those while its last wipe has not
returned, and at launch until shared code has answered once. The exact list of entry
points is the open part of landing 3 (section 9).

## 6. Files

Landing 1 (shared code only):

- `mobile/AutoradioLab/account.ts`: rewritten smaller. Keeps `WIPES`, `wipedPaths`,
  the leftovers check and the native deadline. Adds the record read (strict digest, any
  error counts as "another"), `check`, the one-at-a-time job chain, `settle`,
  `signOut`. Deletes `PENDING_FILE`, `decide`'s five actions, `reconcile`'s two rounds,
  the late continuations, the token, `settled` and `resetSettled`. The record is
  removed from `WIPES` (it is the boundary, not a store).
- `mobile/AutoradioLab/app.ts`: the gate and the generation check in `answer`; the
  exempt list; the `signout-pending` grant lines go.
- `mobile/AutoradioLab/api.ts`: a deadline on `keyState`'s native calls (past it:
  unreadable, with `error: true`); a count of key reads in the log.
- `mobile/AutoradioLab/account-state.ts`, `words.ts`: the card from the boundary
  state; sign-in only in EMPTY with the key absent; "rejected" offers "Clear this
  phone"; sentences for CLEARING, ADOPT failing and the bridge error.
- `mobile/AutoradioLab/account.contract`, `settings.contract`, `signedout.contract`:
  the re-read task includes `clearing`; "Try again" after five tries.
- `mobile/AutoradioLab/account.test.ts`: rewritten from section 7. A new
  `app-gate.test.ts` drives `answer` itself.
- `mobile/scripts/proof-real.sh`: the drills of section 8.
- `docs/current-architecture.md`: the account row, in the same change.

Landing 2 (native receives the generation): `ModelsEntry.swift`,
`AutoradioModule.swift` (one parameter passed through), `ChoirSignIn.swift`,
`ChoirApp.swift`, `AccountWipe.swift`; `HeldPages` and `TakeMirror` wherever they have
landed by then; `keystate` answers the digest.

Landing 3 (the native read gate): `Conversation.swift` and the held-page read, after
the entry-point list is settled.

## 7. Tests, from the model

Written before the code, each failing on main or on the second attempt where it names a
finding. "Drive" means through `app.ts` `answer` with a real source reading a seeded
store (takes, podcasts library), not through `account.ts` alone.

**The nine cells.** For each cell of section 2: the state `check` returns; whether a
gated source answers or throws (drive); whether the card offers sign-in; what one
settle leaves (record, stores, key untouched); that a second settle changes nothing.

**Transitions.**

- T1 ADOPT writes the record only when there is none.
- T2 CLEARING: native wipe, shared wipes, survivors checked, record removed, in that
  order; with key B present, ADOPT B follows in the same job.
- T3 the record is never rewritten from one digest to another (a spy on every write).
- T4 a wipe never deletes the key: a settle calls native `wipe`, never `signOut`.

**Crash points** (the process stops; a fresh module set reads the same files and
key). One test per row of the crash table in section 4, each asserting that no gated
source reads the old account's store before the relaunch's settle ends, and the end
state.

**Physical faults.**

- F1 native `signOut` past the deadline, the key deleted later: the result says "did
  not finish"; once the key is gone, the next answer is CLEARING and the drive refuses.
- F2 native throws synchronously, throws later, answers `done: false`: the same rule.
- F3 the Keychain delete fails (key still A): OPEN for A, no shared wipe, message says
  try again.
- F4 a shared wipe throws, and a wipe leaves its file silently: CLEARING stays, the
  card names the store, the drive refuses, the next attempt retries every store.
- F5 the record write at ADOPT fails: closed; the drive refuses; the next answer retries.
- F6 the record removal fails, or the file survives its removal: CLEARING stays.
- F7 the record holds an empty line, a bare name, a digest plus a list (the second
  attempt's format) or bytes that are not text: each is "another": CLEARING, never
  ADOPT.
- F8 the record file exists but its read throws: CLEARING.
- F9 `keyState` hangs: past the deadline, LOCKED; the drive refuses; nothing wiped.
- F10 a native wipe that never answers: CLEARING stays; a second attempt does not start
  a second native wipe; when the first answers done, the next attempt removes the record.

**Concurrency.**

- C1 two answers during a CLEARING with key B: one wipe runs; both refuse; after it,
  both answer B's (empty) data.
- C2 a sign-out tapped while a settle runs: it waits; the order is settle, sign-out.
- C3 two sign-outs: the second waits and ends signed out with one key delete that
  mattered.
- C4 key B appears while CLEARING: wipe, then ADOPT B; B's store write during CLEARING is
  refused.
- C5 a source that began in OPEN, with a wipe completing while it awaits: its answer is
  refused (the generation check), and its write is refused (`writeCurrent`).
- C6 a late native answer after its attempt ended: nothing is written by it.

**The gate itself.**

- G1 every source not on the exempt list throws in CLEARING, ADOPT failing and LOCKED
  (enumerated from `sources`, so a new source is covered without editing the test).
- G2 every exempt source imports no module that touches a wiped path or a signed-in
  request.
- G3 the card per state, including "Clear this phone" for a refused key and no sign-in
  outside EMPTY.

**Native (landing 2, Swift package tests through the lock).**

- N1 `run(generation: g)` sets the native generation to max(current, g); a late
  `run(generation: g-1)` lowers nothing.
- N2 a failed sign-out (the key remains) at g: the next push at g is accepted by
  `HeldPages` and `TakeMirror`.
- N3 a push begun before the wipe (generation below g) is refused; one at g is held.
- N4 the native answer is done only after every registered wipe returned.
- N5 no wipe test touches a developer's real files (the takes review's should-fix:
  temp instances for every store passed to `registerStandard`).

**Every review finding, mapped.**

| Finding | Test |
| --- | --- |
| Problem record: timeout, throw, not-done, late return on any key, failed re-settle remembered | F1, F2, C6, F4 (nothing is remembered) |
| Seat 2 r1 F1: a retry wipes the new account's data on every answer | C4, F4 (B cannot write while CLEARING, so a full retry loses nothing of B's) |
| Seat 2 r1 F2: tests did not check B's data survives | C1, C4 |
| Seat 2 r1 F3 / seat 1 r2 #5: a failed stamp or marker removal counted as clean | F6 |
| Seat 2 r1 F4 / seat 1 r1 #4: a throwing settle stuck for the process | F8, F9 (a throw is a refusal for this answer; nothing is kept) |
| Seat 1 r1 #1: a clean sign-out left a memo and no stamp; C adopted B's data | the nine cells; T1 (no memo exists) |
| Seat 1 r1 #2: late continuation after a failed shared wipe kept the memo | C6, F4 |
| Seat 1 r1 #3: B signs in inside the 30 s window and reads A's data | C4, G3 (sign-in not offered; if B appears, CLEARING) |
| Seat 2 r2 B1 / seat 1 r2 #1: `app.ts` discards the settle result | G1, every drive |
| Seat 2 r2 B2: unreadable key with a pending list read A's data | the LOCKED cells, F9 |
| Seat 2 r2: the native retry not scoped to the account | T4, C4 |
| Seat 2 r2 / seat 1 r2 #4: unknown list name loops; generation bumped each answer | F7 (no list exists); an attempt runs only in CLEARING, where no page reads |
| Seat 2 r2: a garbled stamp read as none | F7 |
| Seat 2 r2 / seat 1 r2 #5: native calls without a deadline; a sync throw escapes | F9, F2 |
| Seat 1 r2 #2: a failed marker removal fell through to "nothing" | the marker is deleted; F6 for the record |
| Seat 1 r2 #3: settles race; C's stamp overwritten back to B; finish deletes B's key | C1, T3, T4 |
| Reader r4: native floor above the account's generation, no heal | N1, N2 |
| Takes r6, r7: TakeMirror seen-relative floor; failed sign-out refuses for the process | N2, N3 |
| Takes r6: wipe tests delete real files | N5 |
| Problem record: `Conversation.begin` has no generation guard | landing 3 |

## 8. Landings

**Landing 1: the gate and the derived boundary, shared code only.** Red, two seats
(seat 1 Gemini 4 Hydrogen through `agy`, a Sonnet subagent on a rate limit; seat 2
DeepSeek v4.1 flash), each given the code and this document, not
the author's conclusion. It closes the cross-account read on the shared side and makes
more pages refuse while a wipe is owed. Rollback: revert the commit; the record format
is unchanged (a bare digest, the same file main writes today), so main's code reads
what this landing leaves, and a stray `signout-pending` file left by main is inert.
Precommit in `docs/calibration/claude-ledger.md` before landing: "the account drills
pass on staging's build with no gated page answering in CLEARING", settled by the
drill's output. Proof on the simulator (`mobile/scripts/proof-real.sh`, the lead's
build):

- D1, the existing drill: data on the pages, sign out, the card says signed out, Files
  and Mail empty, no `account-stamp` and no `signout-pending` file.
- D2, a crash: sign out, terminate the app before the drill's wait ends, relaunch;
  before the first page answers, the old account's files are gone or every page refuses.
- D3, an account change without a sign-out: with the proof key signed in and data
  seeded, replace the Keychain item with a different string (the proof-key import path
  takes any file), relaunch; the stores are wiped before any page shows data, the record
  is the new digest, and the card shows the refused-key state with "Clear this phone"
  (the server refuses the fake key).
- D4, a refused key: the card offers "Clear this phone", not "Sign in".

Expedite: carries `Ring-Expedite: a sign-out that does not finish cleanly leaves the
old account's data for the next account`.

**Landing 2: native receives the generation.** Red, two seats. The generation passed
with each wipe; `bumpGeneration` deleted; `HeldPages` and `TakeMirror` floors from the
carried generation (or their lanes rebase onto this); `keystate` answers the digest.
Rollback: revert; landing 1 does not depend on it. Proof: N1 to N5 through the Swift
lock, D1 to D4 again, and the reader and takes drills.

**Landing 3: the native read gate.** Red, two seats. After the entry-point list is
written into the problem record. Rollback: revert. Proof: a Swift test that a voice
turn at launch with a changed key reads no old store, and D3 with a voice turn.

## 9. What I am unsure of, and what would falsify this

- **Telling "no record" from "cannot read the record".** The design needs the file
  store to distinguish a missing file from a failed read. I did not find Exact's error
  kinds in `docs/reference/exact`. If they cannot be told apart, a read failure counts
  as "none" and the phone could ADOPT over another account's data. The fallback is to
  stat first and treat any error on a file that stats as "another"; landing 1 must read
  the Exact host source and write the answer into the reference page.
- **Whether the key changes only through our sign-in.** The gate trusts that the key
  does not change between the check and the source's reads in OPEN, because sign-in is
  offered only in EMPTY. A Swift path that writes the key while OPEN (an onboarding page
  not yet moved, the simulator's proof-key import) would break that. The proof-key
  import runs only in simulator builds, before any key read. A search for every
  `keys.set(` in the Kit belongs in landing 1's review brief.
- **The cost of one key read per answer.** Not measured. If it is too much, the fix is
  the one-call digest of landing 2, not a remembered answer.
- **Native reads at launch.** Section 5's residual. How many native readers there are
  and whether one gate at the voice entry covers them is not settled.
- **What a refused page looks like.** Some pages render an empty placeholder on failure
  ("no mail") rather than an error. The card above it says why, but a page that reads as
  "you have nothing" during CLEARING would be misleading. D1 to D4 screenshots will show
  it.

What would falsify the design: a drive test or a drill in which a gated source returns
a store's contents while the record names an account other than the stored key's; a
relaunch after any crash point that reads OPEN or EMPTY with the old account's files
present; a native floor above the shared generation; a refusing state that does not
heal after the fault is removed and the card's re-read or "Try again" runs.

## 10. Owner decisions, each with the default taken

1. **A refused key offers "Clear this phone", not "Sign in".** One more tap for the
   person whose key was revoked. Default: yes. The alternative (sign in over the old
   key) is the one path that changes the key with the old data in place.
2. **Public pages refuse during CLEARING.** The paper front and the podcasts library
   are in the wipe list, so they refuse while a wipe is owed. Default: refuse; the state
   lasts seconds unless a store cannot be removed.
3. **A crash before the native key delete leaves the person signed in.** The wipe is
   not run, and the person taps Sign out again. Default: accept. Keeping the sign-out
   intent across a crash would need a durable marker again and a retry that may delete
   a key; the design keeps key deletion to the person's own tap.
4. **Themes stay device settings**, outside the boundary, as today. Default: yes.
