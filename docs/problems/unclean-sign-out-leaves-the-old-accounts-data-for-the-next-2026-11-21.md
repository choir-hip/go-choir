# A sign-out that does not finish cleanly leaves the old account's data for the next account

Recorded 2026-11-21T04:04Z by the lead, from seat 2's sixth review of the takes lane. On
main today (`mobile/AutoradioLab/account.ts`). Not fixed; the fix is its own
red change (lane `account-settle-memo`), not part of the takes move.

## Problem

Shared code settles the account once at launch and remembers the answer
(`settled`, read before any page's source runs, `app.ts:135`). A sign-out
that ends other than cleanly leaves that remembered answer in place. If a
second account then signs in without a relaunch, the settle step answers
"nothing to do", the first account's shared files and stamp remain, and the
second account's pages read them.

## Evidence

The reviewer ran the real modules with the test phone's own failure switches:

- sign-out times out after the key is deleted (`account.ts:244`): result
  `done:false, failed:''`, marker and stamp still the first account's;
- native throws or reports not done (`:246-251`);
- the late continuation (`:240`) returns on any present key without checking
  whose, so a second sign-in in the window suppresses the late wipe;
- a failed re-settle is itself remembered (`settleAccount`, `:266`).

In each: key = B, `settleAccount` returns the launch answer, action
"nothing", and `app:/data/takes.json` and `podcasts-library.json` of A are
still present.

## Belief

Reachable only when the native half of a sign-out hangs or throws after the
key delete, and the app is not relaunched before the next sign-in. Rare, but
it is private data crossing accounts, which the product forbids outright.
The app is not launched and has one user, so nothing has leaked.

## Remaining error

Whether the account page offers sign-in while a sign-out is still pending
was read, not run. The same review notes `Conversation.begin` has no
generation guard (a late question could be added after a sign-out); not
reproduced.

## Next

Tests for the four exits first, then: forget the remembered answer on every
unclean result, forget it when a settle reports a failure, and make the late
continuation check whose key it sees. Red: two seats, proof on the simulator
drill.

## Two builds, both sent back by both seats; the lane is stopped for a design (2026-11-21T05:32Z)

Lane `account-settle-memo`. Nothing of it is on main.

1. `eb0604d12` (three guards that reset the remembered answer at exits).
   Seat 2: one guard's retry wipes the current account's data on every page
   answer. Seat 1: three more paths where the answer stays remembered.
2. `579ceb2db` (the answer remembered only for the observed state; a list of
   stores still to wipe kept in the stamp). Seat 2: `app.ts:135-139` discards
   the settle result, so when a wipe or the stamp write fails the page source
   answers anyway and a listed store is readable by the next account. Seat 1:
   the same fail-open, plus a failed marker removal remembered as clean, two
   settles racing through an account change, and an unknown name in the list
   looping forever.

Belief. Both builds worked on what settle remembers. Neither changed what a
page source does when settle has not finished cleanly, and that is where the
data crosses: the gate fails open. The questions a design has to answer
before any code: (a) what a page source is allowed to read while the account
state is not known clean, and what the person sees then; (b) who is the one
writer of the account state, so two settles cannot race; (c) what is durable
and in what order, so a crash at any point restarts into a state that
refuses rather than reads; (d) how native stores (`HeldPages`, `TakeMirror`
and the rest of the wipe list) take the same boundary. The reviews are in
the session scratchpad (`review-acct/r1`, `seat1`, `r2`, `seat1b`).

Remaining error: the defect is still on main. It needs a sign-out whose
native half hangs or throws, then a second account without a relaunch; the
app has one user and is not launched.

Next: a written design (the account boundary in shared code), reviewed
before code, then tests from its table.

## 2026-11-21: HeldPages needs the wipe's generation as its boundary (reader review 4)

The reader's native `HeldPages` (the pages the shared reader hands the voice) is emptied by AccountWipe's registration but has no generation boundary: a hand-over in flight across an account change can still land after the wipe. A floor kept from the shared generation was tried and removed (it never healed after a failed sign-out); the boundary arrives with the wipe's own generation in the account change.

## 2026-11-21: station 1 of the partition design is built in lane `account-partition-s1` (not landed)

The design "Accounts are partitions" (`docs/design/account-boundary-2026-11-21.md`) replaces the wipe machinery. Station 1 is code on a lane from the cutover train, with its bun tests (`account.test.ts`, `account-world.test.ts`, `app-gate.test.ts`) and six simulator drills in `mobile/scripts/proof-real.sh`.

What now holds, each with the test that fails if it stops: every account path is re-rooted under the account's folder (I1: `account-world.test.ts` R1 to R2b, and the scan that only `api.ts` and `account.ts` hold raw storage); no request is signed except with its world's key (I2: R5, and the scan of raw `fetch` and the Authorization header); no module-level mutable state outside a world's bag (I3: the M3 scan, with a reason for each of the 40 listed names); sources that draw before sign-in get the device handle (I4: G3); at most one account world per process, and none after a sign-out (I5: L5, S1, R3); every root is named by a live key or is deleted at launch, never when a key source is unreadable (I6: L1 to L3, S2); every gated resource takes the handle or is `transient` (I7: the contract scan); no key in a path (I8: L6).

Remaining error, stated: (a) the six drills have not run on the simulator; (b) `sqlite.open app:/data/accounts` as a prefix grant and the first frame keyed by the handle are read from Exact's docs and source, not seen on a device; (c) the native stores and the native client still serve the one Keychain item (station 2); (d) a module that keeps a `gen` guard around its own writes still has it: with the one-world rule it is redundant and was left, not repaired; (e) the old tests of the per-module guards that relied on `writeCurrent` removing a late file were deleted (the late write now lands in the closed world's own root, which no key names and launch deletes); (f) the sign-out waits for native's run-sheet acknowledgement, which shared code now sends itself every 100 ms, so it no longer stands still for five seconds.

## 2026-11-21: seat 1's four blockers on `95065ed1c` (station 1), and what is still open

Fixed in lane `account-partition-s1`: (1) the launch swept the signed-in account's root when there was no native handle yet (`keyFact` answered `nomodule`, `launch()` refused only `unreadable`): it deleted every root and latched. The sweep now runs only on a key that was read or the Keychain's own `absent` (`account.ts` `launch`, `sweepOnce`); no handle, no storage, a timeout or `unreadable` is LOCKED, not latched, and the sweep runs when the fact arrives (tests L7 to L9). A host that has decided it has no native module at all opens the anonymous world and sweeps nothing. (2) The card `acs` and `au` are `transient`: a kept `acs` would have handed the last launch's handle to every gated resource at the first frame. (3) The I7 scan lists every resource of every contract, native ones included, and parses the arguments. (4) Drill 6 now dumps Files with nothing settled after the tap and has a control drive that must show kept rows, or says "not driven".

Still open, stated: native voice and audio keep running from `endWorld` until the native sign-out answers (up to 30 s, `timing.nativeMs`); `sqlite.open app:/data/accounts` as a prefix grant is read from Exact's grants crate, not opened on a device; what Exact's secret snapshot answers before first unlock is unknown (the sweep is guarded by the unreadable key, not by it); Exact's kept answers of any native resource that is NOT on the transient list stay at rest after a sign-out until station 2 (none remain on the list in `account-world.test.ts`, but the native wipe still does not clear Exact's kept answers of the resources that are keyed by the handle).

## 2026-11-21: seat 2's review of `95065ed1c` (P0 to P10), fixed in the same push

P2 (a kill between the native key delete and the Swift wipe left native stores that the shared sources relay): a durable ending mark in device storage (`app:/data/ending`), written before the key delete, removed only when native reports a clean wipe (`AccountWipe.run()` returns the failed names; it used to swallow them), refusing every gated source while it exists and wiping again at launch (tests E1 to E6). P4: a locked state has Try again and Sign out after five re-reads, with the reinstall words. P5: `openAccountStorage` unwrapped a bound handle (`themeFromTexture` failed through the real gate with a message carrying the root); every source of `app.ts` is now driven through `answer` in `app-gate.test.ts`, and no message carries a path. P6: `imported.ts` hands the host and the page `appPath`. P8: the device handle denies `sqlite`. P9: step one of a sign-out (`accountEnding`) ends the session and writes the mark at once, so the card draws before the native half; `rmTree` has a deadline; the ended card takes taps (`cardTap`). P10: stale comments, dead code, `app:/tmp/picked` removed at sign-out. Not fixed, stated: native stores are not partitioned (station 2); a failed native key delete still runs `MailSeal().forget()`; the launch sweep runs inside the first answer and can exceed one 100 ms turn on a very large orphan root; the drills (including the new native-store plant) have not run on the simulator.

## 2026-11-21: the lead's five points on `302d25878`, and the residuals to record

Done: the launch finishes a sign-out whose mark it finds (tests E2, E3, S3; drill 2c); Sign out from LOCKED is two steps with Remove and Cancel and is withdrawn when a key read succeeds (P4b, the contract test); drill 6 is a FAILURE of the proof when it did not drive, and its control is Exact's own journal line "shows its kept answer from the last session"; a native handle that exists but is not usable keeps the launch locked, with no timer (L10); an unexpected error code on the mark does not lock a fresh install (E7; `docs/reference/exact/internals.md`).

Residuals, stated: (1) THE FIRST FRAME HAS NO KEPT DATA ANYWHERE. `acs` is transient, so the handle is empty at launch and every kept answer waits for it: for as long as the card's first answer takes (the key read and the sweep, usually well under a second, longer on a phone that has just been unlocked or with a large orphan root) the person sees the placeholders, not their last screens. If Exact shows no kept answer even after the handle is known, drill 6 has nothing to prove and its claim should be deleted. (2) A PERSISTENTLY FAILING NATIVE WIPE LEAVES REINSTALL AS THE ONLY EXIT: the ending mark stays, every gated source is refused, and the card offers Try again and a two-step Sign out; if the native store cannot be cleared, deleting and reinstalling the app is the way out, and the card says so. (3) Exact's data module has no platform fact, so an Android build with a module that never binds, or an iPhone whose module never answers a handle at all, is treated as having no module after the host's wait; a handle that was ever seen is never timed out. (4) The anonymous root can hold an onboarding flag written before a late sign-in; it is never swept.

## 2026-11-21: seat 2's second read (B1 to B9), and the claim corrected

B1: the ending mark trusted a native report that missed stores which swallowed removal failures with `try?` (nine of them). `AccountWipe.remove` now records every failed removal and `run()` names the store that was running, whatever it swallowed; an `AccountWipe` with no store registered names itself (`AccountWipeTests`: a store that swallows a failed removal is still named; once the removal works it is clear). Only native can see native files, so the check is native. The mail seal's forget runs on the `signOut` op only, not on a bare `wipe`. B3: the gate test drives the boundary sources last and counts a refusal as a failure. B4: two disabled assertions restored. B6: while the launch finishes a sign-out the card and the gate answer LOCKED ("finishing the sign-out") at once. B7, B8 and the first-frame sentence: docs and comments corrected. B5: drill 6 now FAILS the proof when it did not drive.

The claim, corrected (B2): after a CHANGED key, native stores are not partitioned (station 2), so the shared pages that relay native facts can still show the old account's native words until the native wipe runs. Station 1 shows no old account's SHARED data and no kept answer of another handle; it does not close the native stores. Residuals: a loading first frame (the placeholders, for as long as the card's first answer takes); a slow native module on an iPhone with no handle ever seen is judged "no module" after 1.5 s; the ledger precommit is the lead's.

## 2026-11-21: round three (F1, F2), and the structure changed (contract rule 7)

F1: two `AccountWipe.run()` calls could interleave, a drained global counter lost a failed removal and both finishes cleared the mark. F2: the first answer of a launch that found the mark waited the whole native deadline. The cause was the same each round: the wipe's truth was assembled from the stores' own reports. It is now a fact about the disk (design note, dated): one native account folder, a serial `run()` that removes it and LISTS it, an answer that is what is still on disk; the per-store recorder is deleted; a scan fails a store that builds its own path; the finish runs in the background behind an immediate LOCKED "finishing" answer and is single-flight. Residuals, recorded: `voice-choice.json` and the speaker profile moved into the folder (an enrolled voice is lost once; no carry-over); `deck-journal.json.bad` and the other old paths are removed by the wipe, not by the stores; the finish deletes the whole `accounts` tree including the anonymous cache, so it is cold after a sign-out; "a wipe with no store registered is not clean" depends on registration discipline; the app target's not-yet-moved stores (onboarding, books, files cache) are verified by `extraPaths`, not by the folder; the proof drills are written, not run, and drill 2c's seed rides on the simulator-only key import, which can race.

The one carry-over (lead, 2026-11-21): the speaker profile and the voice choice are MOVED once from the old path into the account folder before the store first reads (`AccountWipe.url`, tested); an enrolment costs minutes of speaking and is not saved in the cloud. If the move fails the old file stays, is tried again at the next call and is never read from the old place; the old paths stay in the wipe's verified list. Everything else is no-carry-over. The earlier note that the voice is lost once is withdrawn. Seat 2, F5: the move runs only when a key is PRESENT (the old files are removed on a positive `absent`, left when unreadable), so a leftover profile is never handed to whoever signs in next. F7: the join has a 25 s deadline (`<store>: timed out`). F8: the mail seal is forgotten only on a positive `absent`. F3: a turn in flight (voice learn and kept transcript) landing after `run()` writes nothing (tested with a gated embedder).

## 2026-11-21: the seal (seat 1, B1) and the non-blocking items

B1: a store that writes asynchronously could recreate a file after the wipe's listing (a speaker enrolment finishing, a kept turn, a take copy: they cache their path and create their directory on a save). Structural fix, not per store: `AccountWipe.run()` first sets a process-wide seal and, after removing the folder, leaves a regular FILE where it was, so every late write fails (ENOTDIR) instead of recreating account data; `url()` creates and moves nothing once sealed; a fresh process clears the seal file; the listing is authoritative for the rest of the process. Sign-out also stops the runtime's voice and turn pipeline (`runtime.stop()`), as it stops playback. `url()` still creates EMPTY parent folders when it is asked for a path (stores rely on it); that is not account data, and the listing treats a tree of empty folders as gone. M1: the take recording scratch (`tmp/takes`) is in the verified list; `take-play-*` files are still removed but not listed. M5: the scan covers the Kit, `exact-package/apple` and the native modules, nine tokens, exceptions per line with a reason, and checks the callers of stores that take a root. M7: a folder removal that failed keeps the ending mark. The dead `autoradio.account.` UserDefaults wipe code is deleted (nothing wrote such keys). M3, recorded: the voice move is not tied to an account (a profile enrolled by one person moves into the folder whoever signed in); it happens once, at the first launch of this build, and is narrow.

## 2026-11-21: round 5 (seat 1 on 403e7cc9b)

B1: the seal outlived the launch that finished a sign-out, so a later sign-in worked in a sealed process and every native store failed silently. The design's own rule is one account per process and every finished sign-out ends the session: a finish from any entry now ends it (`account.ts` launch), and the native sign-in entry refuses when sealed. F2: the seal's window (a late createDirectory recreating the folder) is closed by retrying until a regular file stands there; the answer names `seal` otherwise. F1: the voice-setup record and the take scratch go through `AccountWipe.url`; the shared-code caches are removed and listed, not sealed (safe under the invariant). F3: an unreadable key keeps the voice on its OLD file until it can move; the proof key is imported before the runtime reads the voice choice. F4: the native total fits the shared deadline; `console.log` joins the wipe. The legacy-file parameters of `registerStandard` are deleted (the wipe's `oldPaths` cover them).

Seat 2, S2: the Swift tests reached the host's real login Keychain (the voice-move test read the real key state; the no-key test left `absent` installed so later runs executed the real mail-seal delete). The build Mac has had repeated Keychain faults (hangs in SecItemCopyMatching, refused writes). Fixed at the root: one door (`SecDoor`) whose default under XCTest reaches nothing, an in-memory fake base class, every Keychain-touching test class moved onto it (MailSealTests and CredentialReaderTests used made-up services on the REAL Keychain; they now use the fake). S3: `MailSeal.forget()` returns the status and the wipe's answer rests on it. S4: one service literal; a failed voice move keeps the old file in use.

## 2026-11-21: round 6, the approach changed

Seat 1 and seat 2 each found, on the seal, the same remaining class: wiping a process that is still running (a cached path outside the root, a double run over the native deadline, a stuck seal, a hung store). After six rounds the approach changed rather than the seal being patched again: nothing is wiped in the process that signs out; the next launch wipes first, before any store exists, from the mark. See the design note "The launch wipe". Test isolation: under XCTest the account base is a temp folder even with no override, `KeychainFakeCase` gives every wipe/store test its own base and an in-memory Keychain, and a scan fails any test that names a real folder; earlier the Deck, HeldRest and wipe tests ran the wipe against the real Application Support and deleted the enrolled voice on the build Mac.

Round 7 (lead review of the launch wipe): the key is revoked BEFORE it is removed (the tap), and again at the launch if a kill left it; an unrevokable key leaves the phone anyway and is named. A not-clean wipe builds nothing (`AutoradioModule.init` returns). The dead per-store wipes and generation guards are deleted. Residuals: files remain between the tap and the next launch (the trade); the mark path read is unproven on a real install until drill 2c runs; the offline unrevoked key has no server-side lifetime (device-login keys have `expires_at` NULL).
