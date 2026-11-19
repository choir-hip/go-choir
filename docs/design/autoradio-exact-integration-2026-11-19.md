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

Fixed while merging: `podcast.test.ts` expected the grant text to end the file (`net.fetch https:` + backtick); the grants are now one
list, the test reads the line. `takes.ts` had a type error (`Uint8Array` to `BufferSource`) that nothing found until `app.ts` reached it.

### The proof was stale, not broken (a, not b)

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
