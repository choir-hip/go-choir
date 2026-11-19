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
DEVIATION from the review: a phone with NO key wipes nothing at launch, because a locked Keychain looks the same as no key and must not cost
anyone their data; the stamp catches the next account. Native `done` is now truthful (the key is re-read after the Swift sign-out). The live
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

## Left, in order (for a third integrator)

1. Queue `proof` and `real` at the tip (the tip below the mail commit has only been through `proof`), read each result.
2. Step E remainder: the Files, Mail and podcasts tours and the sign-out drill in `proof-real.sh` (reads only; never open a PDF, Save draft, the
   approval link or Subscribe). Not written.
3. Step C, files over 48 MB: Exact supports it (`exactBodyTo`, `exactBodyFrom`, reference.md fetch row: to 1 GiB, bytes never enter JavaScript; the
   body goes to a staging file and replaces the target only when complete; a non-2xx reply leaves the file untouched). Not wired in `files.ts`.
   Needs a signed-in `authedToFile`/`authedFromFile` beside `authedBytes` in api.ts (same demo, generation and redirect rules; the file under
   `app:/cache/files`, already granted and wiped) and the Swift limits (800 MB video, 300 MB audio, 60 MB images) as constants.
4. Step D: podcast listening positions (one native read-and-delete of the old Swift library file), the paper page passing the story title with the
   route to `choirPlay`, audio from Files into the queue through the deck's names.
5. Step F, the bridge (a Contract `task ... key=deckState.seq change(action)`): unproven. Step G: not started; the Swift player still decides.
6. Time every shared source that loops on the simulator against the 100 ms turn (`settleAccount` adds one `stat` and one `readFile` to the first
   answer after a launch: measure it).
7. The deck preconditions above; Android's Kotlin for `mailSeal`/`mailOpen` and `signOut`; the earlier list (steps 4 to 6 of the first record).

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
