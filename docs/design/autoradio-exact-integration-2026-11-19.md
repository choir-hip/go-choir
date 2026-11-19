# Autoradio on Exact: the integration branch, 2026-11-19

Branch `lane/exact-integ` (base `lane/exact-base`). The integrator's record: what is merged, how the shared code and the native side
reach each other, what was wired and deleted, what is left. Read with `docs/design/autoradio-in-exact-2026-11-18.md` and
`docs/reference/exact/native-modules.md`.

## Stage 1: one branch that builds

Merged in order, each clean except for the generated files and `app.ts` (both sides kept; REGEN redone, never taken):
settings 8688c3f54, runsheet 6597f90a2, content 1bf7b7420, podcasts ef3a43078, deck f853ecaf3, files 0a93ea1cf, mail b24091308.
Deck, files and mail were merged after their builders pushed.

Checks on the merged branch (last lines):
- contract build: `[]`
- `bun exact.mjs test web`: paper 6 passed, 0 failed; podcast 3 passed, 0 failed (others 1 and 2 passed)
- `bun test`: 542 pass, 0 fail, 20 files (after sign-out step; 534 at the merge)
- locked `swift build` of `mobile/AutoradioKit`: Build complete
- locked `swift test`: Executed 1146 tests, with 1 test skipped and 0 failures (the settings lane's unconfirmed run is confirmed)
- native boundary gate: ceiling at 101 files, 22,036 lines after the merges (it only fell)
- native build through the queue (mode `build`) on the first three-lane merge: exit 0.
- the silent proof (mode `proof`) at 21b62ad31 (all seven lanes, the sign-out step, the Paper fix): `PROOF OK`. Earlier runs failed at the stale
  drive and then at the empty Paper page (below).

Fixed while merging: `podcast.test.ts` expected the grant text to end the file (`net.fetch https:` + backtick); the grants are now one
list, the test reads the line. `takes.ts` had a type error (`Uint8Array` to `BufferSource`) that nothing found until `app.ts` reached it.

### The device's 100 ms turn (a regression of the Paper move, found by the proof)

On the simulator `paperFront` took 317 to 389 ms of CPU in one turn against Exact's 100 ms budget (`reference.md`, Timers row: "A turn gets
100 ms of its own CPU time; `await scheduler.yield()` starts another"), so the resource failed and the Paper page drew nothing. Bun runs the same
code in under a millisecond; Hermes does not. Cause read from the proof's log (`resource pfront failed: ... over the 100 ms budget`): a
`/[\p{L}\p{N}]/u` literal tested once per character in `key()` and `padded()`, which `sections()` and `belongs()` call per story and per
section. Fix: the classes are compiled once, `key()` is one replace and both remember their answers (`paper-model.ts`; the same hoist in
`edition-model.ts`, `notebook.ts`, `programme-model.ts`). The proof then passed the Paper steps. LESSON FOR EVERY TWIN STILL TO BE WIRED
(`search.ts` BM25 and fusion, `content.ts`, `runsheet-model.ts`, `mail-model.ts`): a source that is fast under Bun can fail on the device. Each
needs a timed run on the simulator (the proof's log prints the failure) and, where it loops, `await scheduler.yield()` (guard `typeof scheduler`;
Bun has none).

### The proof was stale (a), and then Paper was broken (b)

The silent proof (`mobile/scripts/proof-sim.sh`) failed on `exact-base` at `tap paper-story-demo-grid-storage`. It is (a): the script
had not been run since the 2026-11-18 landing. The app opens on the Autoradio page (`radio:` is the first fixed page, `members-stack.contract`),
and the bar was redesigned (the mark opens the launcher; `bar-pages`, `bar-voice`, `pile-places`, `card-swipe-*`, `bar-pages-count` do not
exist). Paper in demo mode is fine: the Paper source takes `demo`. The script was rewritten for today's bar and no longer starts audio (it
used to tap the voice button; that step is gone and `bar-play` is only looked for).

## Stage 2: how shared code and native code reach each other

Read from `docs/reference/exact/native-modules.md` ("Calling convention", "Out (Swift to the UI)") and the Exact checkout
(`docs/recipes/after-a-write.md`, `docs/contract-for-agents.md` "root task ... `change(action)`").

What exists, and nothing else:
1. Shared to native: `app.ts` calls the APP MODULE with `native.call` / `native.later` (here `ModelsModule`, `modules/apple/ModelsEntry.swift`).
   It cannot call a PACKAGE's names (`@choir/autoradio`); those are called only from Contract (`send`, `resource`).
2. Native to shared: there is no call. A package can `context.changed("<pkg>.<source>")` (the Contract resource asks again) and
   `context.emit(event)` (a declared event on a Contract node). A Contract `task ... key=<expr>` with `change(action)` runs an action after each
   changed key, "for work that must follow an input change, such as ... issuing a host command" (`contract-for-agents.md` near line 4075), and
   an action can `send` a TypeScript source (a source is a send's target, `after-a-write.md`). So the only native-to-shared path is: native
   changes a source, Contract reacts, Contract asks shared code, and may then send the answer back down to a package name.
3. The app module can call package code (`import ExactPkg_autoradio`); a package cannot call the app module except by `NSClassFromString`.

Mechanisms chosen:
- **Sign-out (done, step 1).** Shared code leads. The Account screen sends the TypeScript source `accountSignOut`. It makes one app-module
  call `{op: 'signOut'}`; that runs the unchanged Swift `ChoirApp.signOut` (playback stops, run sheet posts under the old key, key deleted,
  Swift `AccountWipe`) through `AutoradioModule.current`, and answers `{done: true}`. Then every shared wipe runs, each on its own. A failed
  native sign-out wipes nothing. No hook into shared code is needed. The Swift sign-in and Keychain stay.
- **Demo mode.** A shared source takes the demo flag as an argument (`dm.on` in Contract) and answers from `mobile/fixtures/demo/*.json`,
  imported by the TypeScript (`./fixtures/demo/...`). Paper, settings, files and mail already do. The Swift `Demo.fixture` users die with their
  pages; the `runsheet-*.json`, `takes-*.json` fixtures are read by the Swift run sheet and takes until step 3. Podcasts have no demo (Swift
  had none either).
- **The voice turn's context** (retrieval, notebook text, takes). The turn starts natively and needs words about the question: query
  dependent, so it cannot be pushed down ahead of time as a finished context. Two real options: (i) the turn asks shared code at turn time
  through the path in item 2 (native raises a changed source with the question, a `change` task asks a shared source, Contract sends the
  context down to a package name); cost: two Contract commits and the shared search's CPU (a turn gets 100 ms of its own CPU, `await
  scheduler.yield()` for more) before the model call, and it fails while shared code is frozen (an iPhone lets shared code run only while audio
  sounds); (ii) shared code keeps pushing down a small corpus the native retrieval searches. Neither is proven. Decision for tonight, the
  conservative one: the on-device retrieval (`Retrieval.swift`, `SavedCorpus`, `RetrievalSources`) and the notebook/takes text the turn reads
  STAY Swift; step 4 is not done. A spike of (i) with a timer in the log is the next experiment; it needs the build queue and a signed-in
  simulator.
- **What plays next (the deck).** `radio-director.ts` is a TypeScript source; Contract has to carry its commands to the deck: a `change` task
  keyed on the deck's `seq` (a live package source, `deckState`) asks the director (a TypeScript source, an argument list built from the deck state
  and the sheet), and a `then` action sends `deckPlay`/`deckEnqueue` (package names). Nothing in shared code can call the deck directly. It is
  unproven, it is the red step, and the Swift player still decides everything. Not wired tonight (see "Left").

## Stage 3: what landed

1. Sign-out wipe hook (red; commit "signout: shared code leads ..."): `account.ts`, `account.test.ts` (S1 to S5, with a guard that fails when a
   shared module keeps an `app:/data` or `app:/cache` path no wipe covers), `account-types.contract`, `app.ts` source `accountSignOut`; Swift:
   `AutoradioModule.current` and `signOutForShared`, one op in `ModelsEntry.swift`; `choirSignOut` removed from the interface and the handler. New
   `wipeFiles`, `wipePaper`. Closes the files and podcasts gaps the lanes reported (recents, cached files, podcast library, mail key).
2. Demo fixtures for the moved sources: nothing to do beyond what the lanes did (see above). Recorded, not a commit.

## Dropped or changed on the way (from the lane reports, to carry)

- Podcasts: plain `http` feeds are no longer tried (grant `net.fetch https:`); the old Swift library file's listening positions are NOT
  imported (a one-time import is owed); local search no longer indexes podcasts (`podcastDocuments` is the hook for `search.ts`).
- Files: video and audio over 48 MB and images over 32 MB no longer open (replies are held in memory; the Swift caps were 800, 300, 60 MB). Open
  regression until a streaming download to a file exists (Exact: `exactBodyFrom` / `exactBodyTo` move files to and from disk "without entering
  JavaScript", to 1 GiB, `EXACT.md` "Where logic belongs", table row Server calls; not yet used by `files.ts`). Audio from Files into the queue and the run
  sheet's file resolver; file names in local search.
- Mail: the phone's copy is sealed by hand-written AES-CTR and HMAC in `mail-seal.ts`; the lead ruled that does not go to main. Established
  (the lead's questions): (a) NO platform AES in shared code. `docs/reference.md` (Exact checkout, data-module table, row `crypto.getRandomValues,
  crypto.randomUUID, crypto.subtle`) says `subtle` "digests (SHA-256/384/512) and ECDSA P-256 keys (LLP 1069.005), and refuses the rest by name":
  no AES-GCM. So the decision's second branch applies: one native pair `seal(bytes)` / `open(bytes)` with CryptoKit AES-GCM on the iPhone (owed
  in Kotlin), key still in the secret store (`mail-key`), and `mail-seal.ts`'s cipher deleted. NOT done tonight: it changes the red path of the
  owner's mail and needs its own commit and tests; (b) the file-protection class of `app:/data` on iOS and Android's location, and (c) whether a
  file can be excluded from backup: neither is stated in the Exact reference pages searched (`reference.md`, `recipes/files-and-grants.md`,
  `host/android/README.md`; the one hit is Android's update store under `noBackupFilesDir`). Unestablished. How `mail-key` is kept is `store` under
  `secret.keep`: Keychain on Apple (`EXACT.md`), accessibility class not read. Mail also no longer reaches the retrieval index; the catch-up fetches
  25 bodies per mailbox open.
- Stage 3 steps 3 to 6: not done (below).

## Left, in order, with what is known

1. Step 3: run sheet, director, programme decoder through the deck. RED. Needs the `change`-task bridge above proven on the simulator first.
   Swift to delete when whole: `RunSheet*.swift`, `RadioPage.swift`, the decoder in `Choir/RadioProgramme.swift`, `EditionSetup.swift`.
   One poster of `aired` ops at any commit; the deck's journal is the only source of heard.
2. Step 4: launcher search and the saved corpus on `search.ts`, behind the voice-turn spike.
3. Step 5: content store and sync on `content.ts`.
4. The files streaming regression; the podcast positions import; the mail cipher; the paper-title-with-route loss the deck builder named.
5. Step 6: themes (53 uses), `Preferences`, `Signals`, the stories list.

## Second integrator (2026-11-19, night): what was done, in order

Branch `lane/exact-integ` (local branch `exact-integ`). Check results are at the end of each item.

A. Repo gates on e60724280: `ALL GATES PASS` (`GOFLAGS=-mod=mod scripts/gates`, about ten minutes). No gate outside `mobile/` failed.

Review seat 1's findings on the sign-in seam and sign-out (commit 3ee5d7b06, one commit, mapped finding by finding in its message): demo enforced
at the seam (`setDemo`, `DEMO_AT` in app.ts) and natively (`CredentialReader` answers not-found in demo); the key and computer id in one native
read (`op: session`); only a digest of the key kept; the discovery keyed by it; an account generation bumped by `wipeAll` so a late reply is dropped
as `SignedOutError` (this is the guard for paper, files, notebook, takes and mail too: they write only after an `authedJSON`/`authedBytes` that
throws on an older generation); `redirect: 'manual'` (Exact's fetch supports it: reference.md, fetch row) with a 3xx an error; a durable
`app:/data/signout-pending` marker written before the native call and cleared after a clean wipe, finished at the next launch by `settleAccount`
before any source answers; an account stamp (`app:/data/account-stamp`, a SHA-256 of the key): another account's key at launch wipes the old data.
SUPERSEDED by the reconcile below (the earlier "no key wipes nothing" justification was wrong: with no key no stamp was written, so a later account was adopted without a wipe). Native `done` is now truthful (the key is re-read after the Swift sign-out). The live
`ContentStore` and `VectorStore` instances register themselves so the wipes empty them and stop a running sync. A 30 s deadline on the native
call; a missing native module refuses (nothing wiped). Not done: `wipeRunsheet` is not in the wipe list (the shared run sheet store is not wired;
the Swift `AccountWipe` still wipes it). Tests: api.test.ts D1, D2, G1, R1, O1, K1; account.test.ts S1 to S10 (S8 seeds an in-memory file
system at every `app:/` path the modules name and runs the real wipes; S9 is sign out A, sign in B).
`bun test`: 553 pass, 0 fail. Contract build `[]`. `test web`: 3 passed. Queue `proof` at 75e9bb1b0: `PROOF OK`.

Deck review items 1, 7, 8, 3 (commits b7a08ca9b, 75e9bb1b0): the journal never saves over a file that could not be read; the numbers have their
own file (`deck-journal.seq.json`); a failed save is counted and logged; the cap writes a `system` `dropped` line; `restore` takes an ended line only
after the item's last start; the doc and the `deck-ops.ts` header now say that the Swift player's `played` flag decides what is posted today.
Swift tests: 1154, 0 failures (full run). Preconditions of any shared poster, NOT done: a refused batch resent one op at a time (deck-ops.ts
160-161, 212, 217-218); `start_before_end` held or the late end posted first (deck-ops.ts:101, fixture 12); a journal epoch in `DeckState` and
`deckAck` changed by `wipe()`; a 200 acknowledged only when every op id has a result; the switch from the Swift poster to a shared one is ONE
commit that removes the Swift poster (the op ids differ). When the journal decides, an end reached by a seek is not heard.

B. Mail's cipher (this commit): `mail-seal.ts` is now a thin client of ONE native pair on the app module, `mailSeal` and `mailOpen` (plus
`mailForget`), in `AutoradioKit/Sources/AutoradioKit/Choir/MailSeal.swift`: CryptoKit AES-256-GCM, the key generated and kept in the Keychain as its
own item (service `news.choir.autoradio.mailseal`; `AfterFirstUnlockThisDeviceOnly` on iOS), never returned. The hand-made AES and HMAC are deleted
(the old tests, FIPS vector and Node cross-checks, with them). "No key, nothing written" holds: a refusing or missing native module seals nothing
and `mail-store` writes no file. Sign-out forgets the native key (in the native sign-out and in `wipeFiles`). The `secret.keep mail-key` grant is
gone. The gate has a way to record a non-voice exception, a `path` entry whose reason says so: `MailSeal.swift` is listed with the reason
"NOT VOICE: a platform cipher ...". That is a change to the rule and the owner has not ruled on it; the lead decided no hand-made cipher reaches main.
Android owes the same pair in Kotlin (Keystore AES-GCM): not written.
Evidence for the file questions (Exact checkout `mobile/.exact-src`, read, not run): `app:/data` on Apple is `$HOME/Library/Application
Support/exact/<app id>/data` (`host/apple/src/picker.rs:46-51`, or the app group root, `crate::store::group_root`). No code on the write path
(`data/host/src/native.rs:470-474`, `fs.atomicWriteFile`) sets a file-protection class, and a search of `host`, `data`, `js` and `vendor/ibex` for
`FileProtection`, `isExcludedFromBackup` and `NSURLIsExcluded` finds them only in the widget timeline cache
(`host/apple/Sources/ExactCore/WidgetTimelineCache.swift:63`, `completeUntilFirstUserAuthentication`) and the attestation package
(`packages/attestation/apple/Attestation.swift:226`, excluded from backup). So the files get the platform default for an app's own files (Apple's
rule, not Exact's: complete until first user authentication unless the app's entitlement says otherwise) and, being under Application Support with no
exclusion, they ARE in device backups. Exact's secret store (`secret.keep`) is `AfterFirstUnlockThisDeviceOnly`
(`vendor/ibex/crates/ibex2/src/secrets/darwin.rs:5`). That is why a native key (this device only) matters: a backup restored on another phone
carries sealed mail that cannot be opened. Android: `app.filesDir`; `noBackupFilesDir` is used only for Exact's update store
(`host/android/canvas/.../ExactActivity.kt:330`).
Checks: `bun test` 553 pass, 0 fail; contract build `[]`; `test web` 3 passed; locked `swift test --filter MailSealTests|CredentialReaderTests`:
11 tests, 0 failures. The fake cipher in mail.test.ts is Node's AES-256-GCM with the CryptoKit combined layout (nonce, ciphertext, tag).

proof-real.sh: the failure at `tap ob-skip` is the script's, not the app's. The app opens on the Autoradio page (`radio:` is the first fixed page);
a carried key shows no first-run page, and no lane touched `onboarding.contract` (last changed by the main landing). The script predates the
2026-11-18 landing and was never run against it; its five `ob-skip` taps are now `place paper` (the mark, then Paper).

Queue results: `proof` at 7fb0b3246 `PROOF OK`; `real` at a1e933fad `PROOF OK` (the signed-in tour: Paper, reader, a played story, a Texture document,
the theme, Settings account, Files root; 20 `api: session found=true` log lines, flags only). The first `real` run at 7fb0b3246 showed the Settings
account page, Files and Mail as signed out ("sign-in is no longer accepted") although the key answers 200 on staging; the next run at ef33b0adc
did not, and one run stalled at `settings-nav-account` (a carrier stall). Not explained: a first-launch race between the simulator proof key being
read into the Keychain and the first shared source's read is the suspect (the seam then sees no session and says signed out). A real device is not
affected by ProofKey, but the same read-before-written order should be checked. The textures "make your edition" check was dropped from the
script (the drill computer's state). Mail and Files rows were still not toured (the survey picks no row: dir= and mail= were empty).

## Third integrator (2026-11-19, night): what was done

Review tip for the re-review: the last commit named in the lead's `review-tip.txt` (auth, deck and mail together: 09057a63a, then ea63bff4f, then 1ea1351a7).

1. Seat 2 on the sign-in seam and the deck (commit 09057a63a). Already fixed before: auth 1, 3, 4, 6, 8, 11 (3ee5d7b06), deck 3 in part and 4 (b7a08ca9b). Fixed: a demo
   sign-out answers done false before any native call, marker or wipe (auth 2); a failed sign-out is written on the Account page (`signOutNote`, 5); the drift guard runs the
   real wipes against a recording storage (7); HTTPError scrubs before it cuts (9); `clips` forgotten at sign-out (10); `ContentStore.dispose()` (three `bun test` failures in a
   full run came from a store a test closed). Deck: the first unacknowledged line's gap is named, and so is the deck's own `dropped` system line (the cap still drops the oldest
   unacknowledged lines and SAYS so; stopping the deck for a full journal is the worse failure for a radio; chosen and recorded) (3, 5); `Deck.wipe()` clears the item on air,
   the held items and the reason, stops the engine, registered in the wipe (7); `level` and `rate` deleted from the interface (8). Deck 1 (remote audio recorded as heard
   without sound) untouched on purpose. Deck 2 (a whole-batch refusal acknowledged) and 6 (start_before_end holds one call) remain preconditions of step G.
2. Mail, two seats (commits ea63bff4f, 1ea1351a7). Linear scans with the input cut before any scan (htmlToText, trimH; `clean` capped at 120 000 with a plain statement over it);
   `beginSignOut` before the native call, a generation check at every store write, MailSeal makes no key when the sign-in key is gone; the seal authenticates the record
   (MS3); the native open answers bad or cannot, only bad removes; a first-use key race; `validate` before a catch-up (a restore re-fetches); no plain mail sync or
   mailDocs (mail stays out of local search until a search over sealed mail is designed: this REPLACES the mail part of step 3); compose text cleared at sign-out; draft size
   bound and no double save; `mailCan` probe; a byte budget; demo shows no real rows; links in the original are inert with their target written after them; the reference
   page `docs/reference/exact/native-modules.md` rewritten. Where `app:/cache` is: `$HOME/Library/Caches/exact/<app id>` (`host/apple/src/picker.rs:52`), outside a device
   backup by Apple's rule; `app:/data` is in backup. Residuals: the original is plain on disk while its page is open; the other moved stores (paper, files, notebook, takes,
   podcasts) have the reply-generation guard in `authedJSON`/`authedBytes` and the wipes, but no explicit check AT the write; timeout wording ("You are offline") unchanged;
   compose is cleared by the sign-out action, not by a task on a signed-out tick, and the cached answers of the last page are not cleared.
3. Step C, large files (commit 2a17d6ef4). `authedToFile` + `exactBodyTo`; caps 800/300/60 MB; audio from Files in the queue again through `queueAdd` kind `file` with a `local`
   option (no new native name). The native ceiling was kept by deleting the redundant Swift mail wipe.

NOT DONE (context ran out at a step boundary): step D (paper title with the route; the one-time podcast positions import; plain http feeds; file names and podcasts in
local search; mail cards' page text; mail catch-up beyond 25), step E (the signed-in tour: Files, Mail, podcasts, paper, notebook, takes, the local sign-out drill; the cause
of the signed-out screens in the first `real` run; the first-answer timing against the 100 ms turn), steps F and G (the deck bridge, the run sheet through the deck).

## Left, in order (for a fourth integrator): FIRST, the two reviews that came back after the third pass

A. AUTH RECONCILE (the only blocker left on the app; auth re-review of 09057a63a, SEND_BACK, text in the lead's `review-exact/auth-r2/`). Not started. Make the launch
   check and the end of every sign-out ONE function over three facts: the key as THREE native answers (present with digest / absent = item-not-found / cannot read now),
   the stamp (none/same/other) and the marker (none/pending). Rules: "cannot read now" never wipes and never adopts; key present with a pending marker means the native half
   did not delete the key, so remove the marker and wipe nothing; key absent with a stamp or marker means finish the sign-out (native op under a deadline, idempotent, reruns
   the Swift wipe, THEN the shared wipe, THEN remove stamp and marker); a stamp or marker is advanced or removed only when every wipe succeeded, else it stays and the page is
   told which stores failed; a native call past its deadline keeps its continuation and the marker stays. The four findings (each a failing table row): kill after the marker
   and before the key delete wipes a signed-in phone (account.ts:90, 116-120); kill between key delete and the Swift wipe leaves the Swift stores (ChoirSignIn.swift:128-129);
   a stamp written after a failed wipe (account.ts:126-127); the marker removed at the deadline while the native call still runs (account.ts:93-99). The CredentialReader read
   must be read-only (ChoirClient.swift:86 collapses not-found and locked into nil). Table test: key(3) x stamp(3) x marker(2), plus a kill at each sign-out step boundary then
   a relaunch, asserting (a) no wipe while a key is present, (b) no old-account store left after a new account is adopted (shared, and Swift through a recording stub).
   Notes: `wipeRunsheet` is not in the wipe list (add it or delete it); `mark` returns true with no storage (fail closed); whether `redirect: 'manual'` is honoured at this pin
   (read the Exact source; say what protects the key on a 302 if not); log `api: session` on change only. The first launch over a phone with a key, old Swift stores and no
   stamp must adopt and wipe nothing: say it here. Then the sign-out drill through the queue `real` (clean sign-out; a second account; first launch over a key with stale stores).
B. DECK (the deferred defect: `docs/problems/remote-audio-is-recorded-heard-without-sound-2026-11-19.md`, now in the lane through the merge of origin/main; re-review of 09057a63a: no blocking code finding; the problem record for remote audio is `docs/problems/remote-audio-is-recorded-heard-without-sound-2026-11-19.md` on
   origin/main, commit 7de5bd890; merge origin/main into the lane at a clean point, main's docs win except this file). New commits, tests first: re-run
   `python3 -I mobile/scripts/gen-exact-interface.py` (exact.json still declares DeckState.rate at :200, :1193; add exact.json to the D16 test); a corrupt journal is moved
   aside (`deck-journal.bad`) with a `system` line and journalling goes on from the numbers file; an unreadable launch must still close the item a kill left open once the merge
   succeeds (deck.md:50-52); `wipe()` stops the engine again after an in-flight start resolves; prune lines at or below the acknowledged number at init and merge; one fixture with a
   `dropped` system line read by all three languages, and delete the tautology at DeckTests.swift:294; deck.md:56 names the wipe by its old name; encode inside the `do`.
C. MAIL: no further round unless the seal or the sign-out order changes. Open notes: the original is plain on disk while its page is open; no explicit check AT the write for the
   other moved stores; the compose reset in account.contract is untested (the proof tour must open mail:new after a sign-out).

Then the earlier list:

1. Run `proof` and `real` at the tip again (the queue results of the third pass are in the lead's queue directory).
2. Step D as above. Mail in search: design a search over sealed mail first.
3. Step E: extend `mobile/scripts/proof-real.sh` (reads only: a text file, never a PDF; a message, never Save draft or the approval link; podcasts without Subscribe), then the
   local sign-out drill LAST with a tree assertion that the shared stores are empty and the wipe marker is gone. Measure the first shared answer after launch against 100 ms.
4. Steps F and G as in the first record; the deck preconditions above, plus deck 2 and 6.
5. Android: Kotlin for `mailSeal`/`mailOpen`/`mailCan`/`mailForget` and `signOut`; the file resolver for audio from Files after a relaunch.
6. The earlier list: steps 4 to 6 of the first record.

## Handoff (written at the end of the first pass)

Branch head and state: see the last commit of `lane/exact-integ`. The branch builds natively and the silent proof passes. A fresh integrator
needs only this file. What the proof does NOT cover: signing out (a write), the Files and Mail pages, podcasts, any sound.

For the owner or the lead to look at (simulator and Android):
- Sign-out on a signed-in simulator (not demo): the Account screen's Sign out must return to signed-out and leave no podcast library, mail copy, recents
  or cached files (`account.test.ts` proves the list, not the native call). The one native op `{op:'signOut'}` replies when the Swift sign-out has
  finished; if it never replies the Account screen stays as it was. NOT proven on a device. Android has no implementation of `signOut` (owed in Kotlin).
- Paper in demo and signed in; the launcher (the mark); Settings themes. Podcasts, Files and Mail pages were never driven by any proof: drive them
  once signed in (`real` mode, reads only; its tour has no Files or Mail steps yet: extend it, never tap Save draft or the approval link).
- Time every shared source that loops on the simulator: the 100 ms turn.

Next steps, in order:
1. Prove the native-to-shared bridge on the simulator before step 3: a Contract `task ... key=<deck seq> change(action)` whose action sends a
   TypeScript source and then a package send (`deckEnqueue`). One commit with a bun test of the source and a driver step; print the round trip.
2. Step 3 (run sheet, director, programme decoder through the deck), RED, in small commits; one poster of `aired` ops; delete the Swift run sheet,
   radio page, decoder, edition setup, their tests and interface names in the same commit as the wiring. The paper page must pass the story's title
   with the route to `choirPlay` (an empty title shows on the lock screen for a story not in the radio's list); today the Swift player decides, so
   the loss is not live.
3. The mail cipher (native `seal`/`open` with CryptoKit AES-GCM, see above), the files streaming regression (use `exactBodyTo` / `exactBodyFrom`),
   the podcast positions import, `proof-real.sh` tours for Files and Mail.
4. Voice-turn spike (stage 2), then steps 4, 5 and 6.


## The account reconcile (third integrator, auth round 2)

`account.ts` `reconcile` is the ONE function that decides, at launch and at the end of every sign-out attempt, from three facts: the key as three native answers
(`keystate` on the app module: present, absent = the Keychain said item-not-found, unreadable = locked or any other status; read only, nothing written or re-keyed; the
digest comes from the existing `session` read), the stamp (none, same, other) and the marker (none, pending). The table in `account.test.ts` (S14) runs all 18 combinations; S15
kills a sign-out at each step and relaunches. Rules: unreadable never wipes and never adopts; present + pending marker clears the marker and wipes nothing; absent + a
stamp or marker finishes the sign-out (the native `signOut`, idempotent, reruns the Swift wipe; then the shared wipe; then the stamp and the marker go); present + another
stamp wipes both halves (native `wipe`, which keeps the key) and then writes the new stamp. A stamp or marker moves only after every wipe succeeded AND no path a wipe covers
is left (most wipes swallow "absent", so survivors are looked for); otherwise it stays, the next launch tries again, and a sign-out names the stores in its message. A native
call past its 30 s deadline answers the page "taking long", keeps the marker and keeps its continuation (a late `done:true` runs the shared wipe). A sign-out does not start
without a storage to hold its marker.

What the first launch does on the owner's phone (a key, old Swift stores, no stamp): `keystate` present, stamp none, marker none -> adopt: it writes the stamp (a SHA-256
of the key) under `app:/data` and nothing else. Nothing is wiped; the Keychain item is only read. A locked Keychain (before the first unlock) is `unreadable`: nothing happens.

`redirect: 'manual'` (api.ts): the pin's reference (Exact checkout `docs/reference.md`, the `fetch` row, line 2084) says `redirect` is Fetch's: `follow`, `error`, or `manual`,
which "answers with the 3xx itself" natively. `typescriptRuntime` is NOT a key of this app's `app.json`: the build computes it from the module's own source and grants
(`exact_bake::compat::typescript_runtime_of`, `js/bake/src/lib.rs:1278`) and bakes it into the compatibility id, so a module that names `redirect` is built for a runtime that has it
(the reference says the field moves the number to 2). A 302 therefore reaches `check()` as a status in 300..399 and is refused; the key is not sent on. Not run against a
redirecting server on the simulator.

Residuals: a launch-time partial wipe is logged and retried, but the page is told only for a sign-out (the launch result is not bound to a screen); the table is a model of the phone
(memFs and a fake module), the real Keychain and Swift wipe were not driven; the signed-in drill through the queue `real` is not written yet (step E).


## Step 4 (the signed-in tour) and what the real runs showed

`mobile/scripts/proof-real.sh` now tours Settings account, Files (root, a folder), Mail (inbox, a message), podcasts (the list, a show), the notebook, the paper and a story, then
the LOCAL sign-out drill last (tree assertions: signed in before and signed out after, Files and Mail without rows after, the failure note absent, `signout-pending` and
`account-stamp` gone from the app's files). The proof key has no key id, so nothing is revoked on the server. `real` at 211a6fd28: PROOF OK (33 checks ok; read from the log).
Not toured: a file opened from Files (the drill computer's root holds no table or picture, and a text or Markdown file is opened by the computer as a Texture document, which
writes); a mail attachment; takes (no place in the launcher); a kill injected mid-sign-out on the simulator (the queue cannot inject one: the table in `account.test.ts` S14 and
S15 covers the steps, as a model of the phone, not the real Keychain).

The signed-out screens of the first run (Settings account, Files, Mail "sign-in is no longer accepted" for a whole first drive while the Swift pages said signed in): they recurred in the first
drive after an install in the third and seventh `real` runs and not in the others, so it is a race. What the logs showed: no `api: session` line in that drive (the seam never reached a
successful key read), and a resource's answer is kept until its page asks again. Cause (the best fit, not proven): the simulator's `proof-key` file is imported into the Keychain by the package
module's init, which can run after the first shared-code read of the key on a fresh install; the read finds no key and the answer is kept. A real device has no ProofKey (the sign-in writes the key
at a user act, long before any read), so it is not exposed. Fix (simulator only, compiled out of device builds): ModelsEntry.swift imports the proof key before any read. An earlier attempt,
`adoptNative` (a missing native handle no longer replaces a working one), is kept as harmless but was NOT the cause of the recurrence (the seventh run failed with it in place).

The 100 ms turn on the pages that moved, from the host's own refusals in the run logs: `podHome` was refused ("took 377.8 ms of its own (CPU) time") when the subscribed NPR feed (2.2 MB) came back,
and 103.5 ms after a first cap of 300 000 characters; with 150 000 the sixth run has no refusal. A feed over 150 000 characters is read to its newest whole items only (older episodes of a very large
feed are not shown: a regression; a yielding parser is the better fix). Every other moved page (paper, settings, files, mail, notebook, reader) raised no refusal in six runs. A millisecond figure for
the first answer after a launch was not obtained: `Date.now()` is unavailable in data sources (the analyzer refuses it, `analyze-ambient-api`), so the reconcile's added work
(a stat, a read, a native `keystate`, a SHA-256) is judged only by the absence of a refusal. A timing seam would need the host's own trace.

Left from step 3 (small regressions): the one-time import of the old Swift podcast positions (the Swift file is `Autoradio/podcasts-library.json`, outside `app:/`, so a native read-and-delete is
needed), plain http feeds (the grant is `net.fetch https:` and a test pins it), file names and podcasts in local search (`fileNameDocs`, `podcastDocuments`; mail stays out), mail cards' page text,
mail catch-up beyond 25 bodies. Done: the paper page passes the story title with the route.
Steps 5 and 6 (the deck bridge, the run sheet through the deck) are not started.

### Named residuals of the account reconcile (auth rounds 4 and 5, fail-closed, no exposure)

- `account-change`'s stamp write (`stamp2`) records B's key even if a third key landed between the first key read and the end of the wipe; the next launch then wipes for C. Fails closed.
- A synchronous throw from the native `later` inside `nativeOf` rejects `reconcile` before any shared wipe. The current bridge does not throw synchronously; if it ever does, the
  launch's first answer fails and the next launch retries.
- A late native answer on the launch paths only finishes bookkeeping bound to its own sign-out (the marker's token and the stamp are captured when the finish starts); if both
  changed in between, it does nothing and the next launch's reconcile decides.
- The wipe marker now holds a per-sign-out token (a UUID), not "1"; an older marker of "1" is still a marker.

## The deck with no native code (Android), 2026-11-19 late

The lead's Android build of the squashed lane (`"android": {}` in app.json's host, now committed) showed shared code working (the paper front fetched, 100 stories) but nothing on screen:
the deck's `order` was shared state, but WHICH page is on top came from the Swift OpenPages through `autoradio.openPages` / `pageOpen`, which Android answers NotSupported, so the
deck drew placards only. What moved, each proved with the queue's `android` mode (a fresh store, `tap place-paper`, then a story):

1. **The open pages** (`pages.ts`, a port of `OpenPages` in Pages.swift: open with the opener recorded, close with the opener coming forward, focus, pin, reopen, the 30-page pile, the radio
   always open and unclosable, kept in `app:/data/open-pages-shared.json`, wiped at sign-out). `spine.contract` keeps the native set as `pgn` and adds the shared `pgs`; `pg` is the native
   set when it has a top page (always on iOS) and the shared one otherwise; every open/close/pin/reopen is sent to both (`pms`/`pcs`). Result on Android: `deck-slot-paper:front` holds the
   paper page with 100 story rows with real titles (`paper-story-/pub/texture/ukraine-and-russia-trade-record-drone-strikes-...`). Tests: `pages.test.ts` P1 to P9.
2. **The reader for stories** (`reader.ts`): a `pub:` page resolves the public route and shows the story's text as h/p/li/quote blocks. `members-reader.contract` reads it when the Swift reader's
   answer is the empty "loading". Result on Android: tapping a story opens the reader page with the title and 55 text blocks. Owed (the Swift reader decodes more): citations, versions, related stories,
   tables, embeds, hear-it rows, `texture:` documents, editing. Tests: `reader.test.ts` R1 to R5.
3. **The notebook page**: `notebook.ts` already had the page source; the contract now reads it while the Swift one is empty/substituted (read only; selecting a line and steer taps stay native).

Still Swift, and why (the substituted sources of the Android log):
- Voice and audio, owed in Kotlin: `voiceProfile`, the mic, speech, on-device models, `barState`'s voice fields, `radioPage`/`radioView`, `queueView`/`queueAdd` (the run sheet player), `podcastNow`, the deck's
  audio names. Not stubbed: they answer NotSupported and the pages that need them draw their idle state.
- Plain server reads not yet moved (cheapest next): `takesPage` (takes.ts has `page`, but the Swift owns the kept ids and the audio), `shelfList/Doc/Chapter/Find/Follow` (the books/Reader shelf: Swift `ReaderShelf`),
  `texturesList` (the Textures place), `textureEditInfo`, `editionPicks`, `conversationInfo`, `themeList`/`theme` (themes are Swift data; the default theme is substituted), `signalList`.
- The Swift `OpenPages` is NOT removed: Swift-initiated opens (the bar's ask-about-this-page, the voice) go through it. Removing it needs those to tell shared code through a one-way call that Android may lack.
- Contracts that relied on `sf/...` symbols (drawn only on Apple) and `-exact-exit-animation` (refused on Android) have not been audited.

### Seat 1's answers on the Android delta (f8a74ddc9..ced9acc5c)

- The shared path is chosen by one host fact, `host.ts` `hostMode` (spine.contract `noNative`): "native" is decided at once by a usable native handle that answers a probe and is sticky; "none" only after a
  4 s wait for a handle and a failed probe; unknown is read as native. It is not the native open-pages set's content, so an iPhone cold launch (empty native set until the first native call) runs neither
  twin and keeps its native set; a failed or empty native answer keeps its last value. Test: `host.test.ts` H1 to H5 (H5: zero fetches with the flag false). On the web host the decision becomes "none" after
  about 4 s, so `agent web` with a longer wait mounts the shared set; the `*.test.contract` files read the first moments (their comments say so; `bun exact.mjs test web` passes).
- The feed cap cuts only at a whole item; if the first item alone is over 80 000 characters the show keeps its header and NO episode (tests: "the cap never shows a half item"). What the cap does to episodes:
  a queued episode keeps playing (the queue entry carries its audio url and title; `podcastQueue` puts both in the entry); but an episode older than the capped list (80 000 characters, about the newest 40 to
  60 episodes of a feed with long notes) is no longer on the show page, and its address, opened from a saved page or a "continue listening" row with no hint, resolves to "This episode has no audio" (`resolve`,
  podcast.ts: the feed's episode, else the address's hint). A clip's address carries a hint, so a clip still opens its episode. A yielding parser that reads the whole feed is the better fix; owed.
- `pages.ts` load is single-flight with an epoch (a wipe during the read drops it; the caller reads again), validates entry by entry, never trims the radio, and a reopen keeps the opener (as Swift does).
- Named residuals: the in-memory shared state is re-asked after a sign-out through `signedOutTick` (`pagesView`, `notebookPageShared`); the reader twin has no per-account state. A sign-out on Android is not
  implemented (no native sign-in), so this path has not been exercised there.
