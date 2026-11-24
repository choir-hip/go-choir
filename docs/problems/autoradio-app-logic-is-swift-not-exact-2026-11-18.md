# Autoradio: the app's logic is a Swift package, not Exact, so nothing is shared with Android (2026-11-18)

Status: open. The rule is now in the contract (CLAUDE.md, Product: "The phone
app is Exact; only voice is native"). The move has not started; a design is
commissioned. Class of the move: orange per page, red where it carries
sign-in, the mail store or the run sheet's posted ops.

## Problem

The owner's intent is one codebase in Exact for iPhone and Android, with only
voice native. What was built is Exact pages over a Swift application. An
Android build today would draw the pages with nothing behind them.

## Evidence (measured on `lane/autoradio-integ20`, 2026-11-18)

| Part | Size | Language |
| --- | --- | --- |
| Pages | 50 contract files, 7,742 lines | Exact contract |
| App glue | 1 file, 11 lines (`app.ts`) | TypeScript in the Exact app |
| Logic behind the pages (`mobile/AutoradioKit/Sources`) | 157 files, 38,149 lines | Swift |
| Binding (`mobile/exact-package/apple`) | 22 files, 3,136 lines | Swift |
| Interface (`mobile/exact-package/exact.json`) | 181 names, 62 with a declared substitute | JSON |

The Swift in `AutoradioKit` by what it does (by file name, so approximate):

| Lines | Share | What |
| --- | --- | --- |
| 10,086 | 28% | page models, onboarding, settings, the bar, demo data, the rest |
| 8,964 | 25% | voice, audio, speech, models |
| 4,652 | 13% | server client, sign-in, sync, content |
| 3,220 | 9% | run sheet, queue, player logic |
| 2,959 | 8% | files, reader, books |
| 2,061 | 6% | podcasts, clips |
| 1,646 | 5% | search, embedding, retrieval |
| 1,337 | 4% | takes, notebook, learning |
| 1,167 | 3% | mail |

The package has an Apple implementation only. On Android an Exact package is
a Kotlin class (`host/android/README.md:796-806` in the pinned source); ours
has none.

Correction, the same day: this table first said "8 files, 4,884 lines" of
TypeScript. That count included generated type files in the build output.
The app has one TypeScript file of 11 lines. There is no shared logic layer
to build on; the move writes one.

## Belief

The native package is the only way Exact gives an app the audio session
(`docs/reference/exact/native-modules.md`, "Two ways in; only one reaches the
audio session"). Voice went there for that reason. Each later feature was
added where the code already was, with builders briefed in Swift and tested
by a Swift suite, and no rule said otherwise. The lead did not flag it.

What it has cost already: a second copy of the run sheet reducer (Go and
Swift) kept in step by fixtures; an interface of 181 calls in which a field
the declaration does not know empties a page; app-target Swift that no suite
compiles; long builds.

## A second cause of slow builds, found the same day

The owner expected that using Exact as intended gives fast builds. Exact's
loop is `bun exact.mjs web`: a Contract or `app.ts` edit is in the page in
about a tenth of a second on a fresh app, and a phone or emulator can follow
that loop with state kept. We never used it, because the pages' data was
Swift. Measured on our app, the loop takes 57 to 90 s for a one-line page
edit, and the Swift has no part in that: the Contract compiler takes 64 s on
our pages and 16 s with the Tetramark's drawing stubbed out. The mark is 4.6
of the plan's 5.9 MB, compiled once for each of its 29 uses. Numbers and
commands: `docs/reference/exact/build-and-ship.md`, "Our own loop is slow".
So the move to shared logic is necessary for fast builds and not sufficient;
the pages must also get lighter.

## Not known

- What Exact's shared layer can carry: network requests with our headers, a
  server-sent stream, durable storage, a store encrypted at rest (the mail
  rule), background work, crypto for the sign-in key. The move depends on
  each; none has been read from the source for this purpose.
- What Android does with an interface name that has no implementation and no
  substitute (119 of the 181): refuse the build, refuse the call, or answer
  empty.
- Which of the retrieval and embedding code is "voice" (it feeds the
  on-device model) and which is app logic.

## Next

1. The design for the move is
   [docs/design/autoradio-in-exact-2026-11-18.md](../design/autoradio-in-exact-2026-11-18.md)
   (proposed; read from source, not run).
2. The Android toolchain proved on Exact's own sample app, then the app with
   Android enabled, to see what the pages do with no native side.
3. Until a page is moved, repairs to its Swift are allowed; new features in
   Swift are not.


## Update 2026-11-19 (late): what an Android build now shows

With `"android": {}` in the host, an Android build draws the paper page with its real story rows and opens a story's text with no native code. Moved to shared code: the deck's open-page set
(`pages.ts`), the story reader (`reader.ts`, text only) and the notebook page. Still Swift (and owed in Kotlin where it is voice or audio): the run sheet, the radio page, the queue, the voice, the
books shelf, the Textures place, takes, themes. The full list with reasons is in `docs/design/autoradio-exact-integration-2026-11-19.md`, "The deck with no native code".

## Update 2026-11-20: wording in Swift that says "sign in" when the key is only locked

The account sentence is now one Exact source (`account-state.ts`: signed in, signed out, locked, refused, no sign-in). These Swift pages still decide their own words and say sign-in when the phone is only locked; the wording goes when each page moves to Exact (no Swift was repaired in the signed-out lane): `Notebook.swift:270`, `Learning.swift:111`, `BarState.swift:173`, `ChoirApp.swift:237`, `RunSheetFeed.swift:294` and `TextureReaderSource.swift:111` (`:286` went with the Textures place, below).

## Update 2026-11-20 (later): the Textures place's list moved

The list (rows, the pinned edition, the states and the words, including "Unlock your phone" for a locked key and "Sign in again" for a refused one) is shared code: `mobile/AutoradioLab/textures.ts`, with its tests ported from `TextureDocTests.testTextureList`, `RealShapeTests.testTextureListRealShape` and `testTheServersEditionIsPinnedFirst` (`textures.test.ts`). The Swift `TextureList`, `TextureRow`, the `texturesList` handler and those three Swift tests are deleted; the source and its shapes are no longer in the package interface. Left in Swift for now: `EditionInfo` and `EditionLookup` (the notebook and the onboarding ask it), and opening a document (`texture:<id>`, the Swift reader). Nothing in the Textures list is native: the signed-in client is the whole platform need. Proven on the simulator signed in (documents listed) and signed out (the sentence).

## Update 2026-11-20 (later still): the themes moved

The themes (validate, contrast gate, resolve per scheme, list, the choice, themes from a Texture) are shared code: `mobile/AutoradioLab/themes.ts`, with `themes-bundled.ts` importing `mobile/themes/*.json` (the Swift generator is gone: `scripts/embed-assets.py` was deleted, its demo fixtures having gone with demo mode). Tests were ported from `ThemeDefaultsTests`, the `ThemeLoaderTests` of `SettingsDataTests`, `ThemeLensTests`, the theme tests of `SpineTests`, `DesignRulesTests` R1 and `TypographyRulesTests` T8 (`themes.test.ts`, `themes.test.contract`). Deleted: `ThemeLoader.swift`, `BundledThemes.generated.swift`, the theme handlers of the Spine and Settings lanes, the native sources `theme`, `themeList`, `themeExtras`, `themeCards` and the sends `themeSet`, `schemeSet`, `themeFromTexture`, `themeRemove`. Android now has the themes (it had only the default). The system scheme and Reduce Motion are read from `exactViewport()`; the shapes (`Theme`, `ThemeList`, `ThemeCards`, `ThemeExtras`) still sit in `interface/spine.json` and `settings.json` and can move to a contract file later (a mechanical edit of 29 `use` lines). One native call is left, for the move only: `themeLegacy` reads (and removes nothing of) the old choice (UserDefaults `AutoradioThemeID`, `AutoradioThemeMode`) and old user themes (Application Support/Autoradio/themes) once; `ModelsEntry.swift`, with the reason beside it, and `migrate` in `themes.ts` go, with the old copy, when phones have moved.

## Update 2026-11-20 (the bar): the bar's page model moved

The bar's state, words, timed windows (error 4 s, revision 6 s), progress, answer card and the run sheet overlay (held for the audio session, nothing on air, remote audio shown as the radio) are shared code: `mobile/AutoradioLab/bar.ts`, tests `bar.test.ts` (B1 to B11, written first and run red). Deleted: `BarInfo`, `BarTracker.barInfo/baseInfo/decorate/plain`, `BarState`, `Conversation.visible`, `RunSheetPlayer.overlay(BarInfo)`, the native source `barState`. Native now answers one source, `barFacts` (`interface/bar.json`): the engine's phase and words, its events kept raw with the device's time, the player's state and the audio's place in the block, hands-free, and the run sheet player's three flags (on air, held for the session, remote audio). It decides nothing; each is native because only the audio code knows it. `conversationEntries` also hands over the id the card was closed on. A data source may not read the clock, so the facts carry `at`, and while a timed state shows the contract reads the facts again every quarter second. The only memory in shared code is the player's message ("new once"), keyed on the account generation. Nothing is kept in a file, so there is nothing to carry over and nothing the listener loses at upgrade (the owner, 2026-11-20: the app has not launched; nothing is to be saved). Behaviour changed on purpose: a message that goes and comes back is new again (the Swift kept the last words forever). (The Autoradio page briefly read the engine's phase itself; slice 2 below put it back on the bar's state.) The card's Close now carries the id of the entry the card shows (`answerId`), so it closes that entry even when a later one was cancelled. Left in Swift and not moved: the run sheet queue and posting (`RunSheetPlayer`, `RunSheetStore`, `RunSheetFeed`: posted ops are red), the radio view (`BarTracker.radioInfo`), the speed and the microphone level.

## Update 2026-11-20 (the Autoradio page): the run sheet page model moved

The Autoradio page (status line, the card on air, Up next, Heard, the greeting, "Built on", "Why this", remote audio's clock, the take on air) is shared code: `mobile/AutoradioLab/radio-page.ts` over `radio-model.ts` (which was a port with no caller), tests `radio-page.test.ts` (R1 to R9, run red first) and the existing `radio.test.ts`. Native answers one source, `radioPageFacts` (`interface/radio.json`): the run sheet as the store holds it (wire JSON, applied op ids left out), the player's sentence and the audio's place, remote audio's seconds, the desk's flags, when the story on air was published, the story versions the phone holds, the take on air. The words ("2 h ago", the clock labels, the day-part) and every decision are shared. Whether a turn is open is the bar's state (`bar.ts`), so an error held on the bar beats "Listening" again, as it did before slice 1. Deleted: `RadioPage.make`, `RadioPageInput`, `RadioPageInfo` and its row shapes, the Kit's greeting, date and day-part words, `PublishedTimes.words`, `radioPageInfo()`, and the Swift tests of all that (ported first). Left native, on purpose: the gestures (`radioPageAct`: reorder, remove, play, replay change the sheet and so post ops; red, with the queue), and `RadioPage.length`, `openAddress` and `builtOn`, which the takes pages still call (they go with the takes page). Nothing is stored; nothing is carried over.

## Update 2026-11-20 (the Autoradio page, simulator proof): an on-air story with no title

The simulator proof of the page showed the card titled "Story" and Built on a raw route. Cause, read from the run sheet file the drive left on the simulator: the on-air entry was `place now` with title "" and label "". The Spine's native handler for `choirPlay` passed only the route to `ChoirApp.play`, which fell back to the radio's own story list for the headline; when the list lacked the story the entry had no name. The old Swift page showed the same "Story" in that case (its `displayTitle` is the same), so the bug is older than the move and was hidden when the list held the story. Fixed at both ends: the handler now passes the title it is given, and the page (`radio-page.ts`, test R10) names an on-air page or story entry that carries no title by the programme on air (`airTitle`), for its title, small line and Built on row. The small line is the entry's source label, as before: with a title and a label that are the same words the headline shows in the title and the small line, as on main.

## Update 2026-11-20 (the takes page moved)

The Takes section is shared code: `mobile/AutoradioLab/take-session.ts` (the session: publish, refresh, delete, mute, every word, the listener's own ids and muted voices) over `takes.ts` (the wire and the page model, already shared). Tests: `take-session.test.ts` (S1 to S8, U1 to U3), `takes.test.contract`; `TakesPage.swift`, the publish, list-of-mine, delete and record half of `TakesClient`, `MyTake`, `Mp4Header`, `TakeBounds`, the multipart body and the Swift `TakeSession` publish half are deleted, and the native sources `takesPage` and `takesAct` with them. What stays native, with the reason: the RECORDER (`TakeSession.swift`, on the voice list: microphone, the radio's gate, the file, the recogniser, the preview, the ceiling stop; it answers one snapshot, `takesRecorder`, and codes for why a take ended, the words being shared); `takeAudio` (the file is native's), `takeSteerMute` (the steer queue is native), the take audio fetch for the run sheet. Still Swift and not moved: the programme request (`RadioProgramme.candidates`) lists the public takes and filters them by the listener's own ids and muted voices, so `TakeOwn` and `TakeMutes` stay as READ-ONLY copies that shared code pushes (`takeMirror`) after every change, and `TakeStore`, `TakesClient.list` and `audio`, `TakeCandidates` remain until the programme request moves. Data on an upgraded phone (changed after four reviews and the owner's ruling, `docs/problems/takes-in-exact-three-reviews-no-landing-2026-11-20.md`): nothing is carried from the pre-release build. Its two files (`takes-own.json`, `takes-muted.json`) are never read; sign-out removes them. Muted voices start empty; the own ids and handle are re-learned from the server's list of the account's live takes. The programme's copy is other files, pushed by shared code. The table is in `docs/reference/AUTORADIO-TAKES.md`, "The phone's own state". An unpublished take was never stored across launches (the file lives in the temporary folder, the state in memory), and still is not: a take in review belongs to the recorder, and a restart of the shared code finds it. The floor is now measured by the recorder (the file's length through `AVAudioFile`, the recorder's clock if unreadable), not by the MP4 header; the claim at publish still comes from the header (`mp4Seconds`). Ceiling 95/18366/93 to 94/18144/92.

## Update 2026-11-20 (reader, slice 1): the document read moved

The reader page's document (blocks, citations, versions, related, tables, embeds, "Hear it", stop, restore, reload, prefetch) is shared code: `mobile/AutoradioLab/reader.ts` and `reader-parse.ts`, with `reader-parse.test.ts` (the old `TextureDocTests` intents, and the eleven golden documents of `internal/texturedoc/golden` projected and compared with the committed `expected/` files) and `reader.test.ts`. The five Swift handlers, `ReaderStore.restore`, `cancelRevision` and `prefetch`, `HearIt.rows(address:)` and their tests are deleted. Not deleted, with the reason: `ReaderStore`, `TextureDocParser`, `Citations` and `Markdown` still serve the voice's page text (`pageSources`, `ReaderPageSource`), the retrieval corpus (`SavedCorpus`), the content sync, the notebook and the Texture editor; they go when the voice's page text is handed over by the shared page and those four move. Next slices: 2, the follow-along and aim (the radio sets them; today Swift matches the spoken words against its own copy of the document, so a sentence highlight can differ from the shared page's sentences); 3, the voice's page text and the removal of the Swift parser. No old native file is read: the reader kept nothing on disk. At upgrade the listener loses the offline copy of stories not opened that session.

## Update 2026-11-20 (reader, slice 3): native stops parsing the page for the voice

The shared page hands the page in front to the voice (`reader.ts` `hand`, the app module's `pageText` op, `HeldPages.swift`), so the voice turn's page text, the saved corpus's "the stories the reader showed" and the follow-along read what the page drew, with the page's own block ids. Deleted: `ReaderPageSource`, `PublicationTextSource`, the `ReaderStore` cache, in-flight sharing, offline hook, `pub:` fetch and related list (it is now a bare fetch of a `texture:` document), `ContentStore.readerDoc`, `readerFollowSet` (handler, name, mutation) and their tests. Callers left, and why each cannot go yet:
- `TextureDocParser` (+ `Citations`, `Markdown`): `ReaderStore.fetch` for `Notebook.swift` (`defaultLoadDoc`, `writeTopicsNow`: the edition's Markdown) and `ContentSync.swift` (`documentBody`); `StoryBodies.resolveFetcher` for the sync's story text. `Citations.normalised` is also the follow-along's text match. `TextureAddress` is used by `TextureEditor` and `Learning.swift`.
- `ReaderStore`: the same three callers (Notebook twice, ContentSync once).
- `ReaderPageSource`: gone. `HeldPageSource` (not a parser) answers the voice's page text.
They go with the notebook and the content store, which are other pages' moves. Offline with nothing held the reader says "Could not open this page. Check the connection and try again."; nothing is read from the old content store.

## Update 2026-11-21 (first run, Computer, Activity and About moved; stacked on the edition picks lane)

First run is shared code: `mobile/AutoradioLab/onboarding.ts` holds the steps (welcome, setup, mic, hearing, voice, ready, topics, writing, done, skipped), their order, what each asks and what is kept (`app:/data/onboarding.json`, on the account wipe list); `edition-writer.ts` is the edition's writer and its deadline (ported from EditionSetup.swift with C1 to C4). Settings > Computer is `computer.ts` (the Swift ComputerModel and its 40-look poll, now the page's own timer); Activity is `activity.ts` over the native facts; About's version row is `settings.ts aboutPage`. Deleted: `ActivityFeed.swift`, `Accounts.swift` (it held only the computer model), `SettingsDataTests.swift`, and the controller's step machine (`AutoradioModule+Onboarding.swift` 489 lines is now a voice shim: facts `voiceSetupFacts` and `ob*` calls, each with its reason in the file header). Tests: `onboarding.test.ts` (O1 to O14), `computer.test.ts`, `edition-writer.test.ts`, `activity.test.ts`, `settings.test.ts`. Nothing is carried from the Swift build's `onboarding.json`.

Not moved, and why: `Configurator.swift`, `LiveConfigBackend.swift`, `VoiceSetup.swift` are the on-device model chooser and downloader (already on the voice list). `Preferences.swift` and `LearnedFromPreferences.swift` are read by the cards (`AutoradioModule+Cards.swift`: the story shading `choirStories`, `launcherData`, `storyMore`, the told-paper page); moving them is moving the cards, which no lane has. Settings > "What the paper has learned" keeps reading them natively until then. The edition's first lines in the notebook are still written by `NotebookStore` (Notebook.swift, the edition picks lane's), through the `obNotebookTopics` call.

## Update 2026-11-21 (the notebook, learning and the content store; lane exact-notebook-content)

The notebook page, the scorecard (the learning route), the selected line and Refresh, and the phone's content store with its sync are shared code on every host: `mobile/AutoradioLab/notebook.ts` (model), `notebook-run.ts` (page state, the one native call), `content.ts` (store and sync, `app:/data/content.db`) and `content-run.ts` (opened once, synced once per launch and per return to the front, the narrow reads `contentDocs`, `contentAt`, `localStories`). Deleted in Swift: `ContentStore.swift`, `ContentSync.swift`, `StoryBodies.swift`, the page, store and learning-route client of `Notebook.swift` and `Learning.swift`, the notebook interface names, `LocalStories`, and their tests (ported to `notebook-run.test.ts`, `content-run.test.ts`; `notebook.test.ts` N4 to N6 and `content.test.ts` already held the rest). Native stays, each with its reason in `AutoradioModule+Notebook.swift`: the topics write into the notebook (a Texture canonical write through the Swift TextureEditor), the steer queue (taps are made by the Swift cards), the DJ's notebook text (`EditionText`, `NotebookCache`: the request is built in `RadioProgramme.swift`), `OpeningCache` and `OpeningWriteAhead` (they hold audio the on-device voice made and drive the Swift run sheet feed), `SavedAnswerHolder` (the on-device model's holder) and `KeptTurns` (microphone turns). Lost until the shared run sheet and retrieval land: the offline story fill when the desk cannot be reached (`localStories` is ready for the writer), the phone's offline reader copy, and the "heard" history in the store (`recordRunSheet` waits for a shared run sheet to read).

## Update 2026-11-21 (content store: review fixes and what was lost)

Fixed on the train: the content sync starts once on each return to the front (the gate is "visible"; each start has its own run number) and not on the way to the background; the native "is audio on air" ask has a 2 s deadline (a wedged answer no longer holds the sync or its one-run slot); the launch sync is stories, documents and the small kinds only, as the Swift was (the file walk is not a launch job, and each kind logs its count and bytes); `localStories` keeps the run sheet's bounds (8 KiB of text, a 300-byte one-line title); the notebook page answers without waiting for native (the steer count is waited for at most 4 s; the text for the DJ is handed over in the background and counted as handed over only after the call succeeded); `forgetNotebook` is on the account wipe list; the pre-release Swift content folder (`Application Support/Autoradio/content`) is removed (never read) by the native sign-out wipe.

Named residuals. (1) Data protection and backup: the Swift store wrote each file with complete-until-first-user-authentication protection and excluded its folder from backup; the shared store is `app:/data/content.db`, which Exact puts in a device backup (`docs/reference/exact/native-modules.md`, mail section) and offers no option to protect or exclude. Stories are public; the documents and files held there are the listener's own, so the copy is in the owner's own backup, accepted pre-launch, to be revisited if Exact grows a storage option. (2) Native retrieval no longer sees stored story bodies or documents from the Swift side; the train wires the store into the shared index (`live.setDocSource('store', ...)`, retrieval-live.ts), which is the only reader now. (3) `EmbeddingService` has no vectors folder of its own any more; vectors are the shared store's (`app:/data/vectors`). (4) The offline story fill (`localStories`) has its helper and tests but no caller on the run sheet.

## Update 2026-11-21 (imported documents moved)

The Reader's PDFs and e-books are shared code: `mobile/AutoradioLab/imported.ts` over `imported-model.ts`, tests `imported.test.ts` (I1 to I15) and `imported-model.test.ts`. An imported document is a file on the person's computer (owner: uploaded files go to the computer and are not deleted); the computer reads its text. Deleted: `ReaderShelf.swift` (507), `Epub.swift` (461), `LibraryCorpus.swift` (49), `AutoradioModule+Books.swift` (212) and their Swift tests; the native ceiling fell from 17,664 to 16,420 lines (94 to 90 files, 92 to 82 names). Native stays only for audio: `AutoradioModule+Imported.swift` (`importedHand`, `importedFollow`). Not moved, named: a .pdf or .epub URL typed in the bar is no longer imported (it opens as a web page; `bar.ts` owes the import); the launcher and ask lane no longer search document bodies; a queued document has no words after a relaunch.

## Update 2026-11-21 (retrieval review residuals, fixed and accepted)

Seat 2's review of the retrieval package sent it back; the train fixed: the R7 wall-clock asserts (logged, not asserted), the index bound (at most 1,200 passages of 4,000 characters, `search.ts` MAX_INDEX_ITEMS and MAX_ITEM_CHARS, for the 64 MiB heap of one Exact module), the first ask after a launch indexing the stores in slices and waiting 1.5 s at most, the stale launcher answer carrying `query: ""`, `RetrievalBridge.clear()` on the account wipe and `last` cleared at each ask, the embedding pass kicked after a refresh that changed the index and run again after a sign-out, the embed progress log line, the deleted `mobile/EmbedTrial`. Accepted residuals, named:
- Documents in the content store and book passages are not searchable by the voice turn's first answer (the index holds the front's headlines and deks, the run sheet, open pages, topics, the page in front, and what the bound lets in); offline it cannot answer from stories the phone holds in full. The largest quality loss of the move.
- The vectors folder is no longer excluded from backup (derived data; a native op to set the flag is the smallest fix).
- A kill loses about 40 slices of embedding work (derived, made again).
- `saveVectors` writes vectors then rows; a kill between them after a removal can pair ids with other items' vectors. The Swift store had the same order.
- `QueryEmbedding` (`Embedder.swift`) has no caller but the loader's type and `LlamaEmbedder`; deleted with the on-device embedder's next change.
- `RadioDemand`'s two-second main-actor loop runs on every host (it was gated on the content store).

## Update 2026-11-21 (reader, the rest): the Swift document parser is deleted

`TextureDoc.swift` (the model and `TextureDocParser`), `TextureMarkdown.swift` and `Citations.swift`, with `TextureDocTests`, `GoldenDocumentsTests`, `WebParityTests` and `proof-golden.sh`, are gone; `reader-parity.test.ts` carries the six web-parity rules and `reader-parse.test.ts` P13 the golden documents. `ReaderStore` became `TextureText`, a fetch of a document's Markdown with no parsing, for `Notebook.swift` (two calls) and `ContentSync.swift` (a document's text); `StoryBodies.plain` gives the sync a story's text from the bundle's Markdown. Left in Swift with reasons: `TextureAddress` (a 20-line address splitter: `TextureEditor`, `Learning.swift`, `StoryBodies`), `HeldPages` (the voice's page), `ListenText` (what the voice speaks for Listen), `Pages.swift`/`OpenPagesUI.swift` (the native open-page set the voice's front page, the cards and the other native page sources still use).
## Update 2026-11-21 (the run sheet and queue package): the shared runtime exists; the native flip is the open part

Built and tested, not yet called: `mobile/AutoradioLab/radio-runtime.ts` (tests `radio-runtime.test.ts`, 23, run red first) is what `RunSheetPlayer.swift` decides, over the deck: the store and the director (`runsheet-store.ts`, `radio-director.ts`, both older ports) hand the deck one item per entry (words fetched first: a paper story, a page, a book, a take, a file), hold the next three entries whose sound needs no fetch, read the deck's numbered journal (an end is the entry's end; an entry the deck played alone while shared code was frozen is aired on the sheet; the lock screen's next and previous), acknowledge what they read, and give the screens one `view()` (what `QueueInfo` was). Also ported and the Swift deleted: the stream rules (`runsheet-stream.ts`, `RunSheetStream.swift`, 62 lines).

Why the Swift player and store are still there: nothing in the player can move alone. It is called by `ChoirApp.swift` (43 places: `play`, `openRadio`, `ensureOnAir`, `moveStory`, the opening, the lock screen), by `Station`, `RadioProgramme` and `OpeningWriteAhead` (the desk's writing and the phone's opening), by the voice turn engine through `QueueVoice` and `TurnQueueWire` (the queue sent with every turn, the server's `queue_ops` applied), by the podcast, takes, books, mail and joins handlers, and `Deck.swift` builds a `RunSheetEntry` to speak. The flip is one change: (1) a native op `deck` on the app module (`ModelsEntry.swift`) that reaches `Deck.shared`, so shared code can command the deck (`DeckPort` over `nativeLater`); (2) the sources `queueView`, `radioPageFacts`, the bar's run sheet facts and the sends `queueAdd`, `queueAct`, `radioPageAct` answered by shared code from `RadioRuntime`, the native ones deleted; (3) the desk writer and the opening (`RadioProgramme`, `OpeningWriteAhead`, `OpeningCache`, `Station`) written in shared code on `programme-model.ts` (the decoder and the writer's rules are ported), with the programme route read through the signed-in client (a POSTed event stream is not shown in Exact's reference: read the whole answer, or a long poll); (4) the voice queue bridge: shared code sends the turn's queue snapshot to native as text and reads the turn's `queue_ops` from a native inbox; (5) `Deck.swift`'s engine call takes a title and paragraphs, not a `RunSheetEntry`; (6) `RunSheet*.swift`, `RunSheetPlayer.swift`, `TurnQueueWire.swift`, `QueueView.swift` and the gesture half of `RadioPage.swift` are deleted (about 3,000 lines). The posted operations are the store's own (`runsheet-store.ts` `flush`, with `routePoster`): wiring the poster is the red part and is its own commit.

### The flip, as decided on 2026-11-21 and what stops it in one change

Decisions (lead, 2026-11-21): a native `deck` op on the app module (the deck is audio and stays native; the interface is item-shaped, so Exact's own `audio` element could replace the remote-file half later); the programme route is read as a whole answer (`gaps.md` 36: a POSTed event stream is not readable by shared code as it arrives; the entries of one write arrive together). What the one-change flip needs, measured on the lane tip: 45 Swift files name the player, the store, the entry type, the feed or the deck (54 places in `ChoirApp.swift`, 28 in the player's own file, 18 in `AutoradioModule+Queue.swift`, 16 in `TurnQueueWire.swift`, 13 each in `OpeningWriteAhead.swift` and `Deck.swift`, then the podcast, takes, books, mail, joins, spine, reader and cards handlers); the desk writer and the phone's opening (`RunSheetWriter`, `RouteStretchSource.candidates`, `StretchCandidates`, `PhoneOpening`, `OpeningCache`) have no shared twin yet (`programme-model.ts` has the decoder and the writer's rules; `takes.ts` the take candidates); the voice turn engine reads the queue through `QueueVoice` and applies the server's `queue_ops` through `TurnQueueWire`. None of it compiles or runs until every one of those is cut together, and nothing of it can be proven without the lead's train. The order that keeps the product working at every commit, so each can be pushed and proven alone: (1) the native `deck` op and `Deck.swift` taking a title and paragraphs, not a `RunSheetEntry`; (2) a native inbox: native callers send the radio verbs (`playNow`, `queue`, `skip`, desk ops with their cause) as text to shared code and read a mirror of the sheet shared code sends back, so `RunSheetPlayer`, `RunSheetStore` and the `RunSheet` reducer (about 1,600 lines) go while the writer, the opening and the voice bridge stay native for one more commit; (3) the writer and the phone's opening in shared code (`radio-writer.ts` over `programme-model.ts`, the opening from the notebook lane's shared functions), then `RunSheetFeed`, `RadioProgramme`'s writer half, `Station`, `OpeningWriteAhead`; (4) the voice bridge on the retrieval lane's pattern, then `QueueVoice`, `TurnQueueWire`; (5) the sources `queueView`, `radioPageFacts` and the bar's run sheet facts, and the sends, answered from `RadioRuntime` in `app.ts`, with the native ones deleted; (6) the poster (red). Steps 1 and 2 are what the podcasts, notebook and retrieval lanes wait for.

## Update 2026-11-22 (dead non-voice Swift deleted; the checker now counts the app's own module)

Owner, 2026-11-22: `RemoteSystem1` and `RemoteProgramme` "are both to be deprecated in favor of the autoradio desk working as designed". Deleted, each after a grep of the kit, `exact-package/apple`, `AutoradioLab/modules`, the Mac commands, the scripts and every `.contract`/`.ts` that names a native op, and a full `swift build --build-tests` and `swift test`:
`RemoteSystem1`, `RemoteProgramme`, the `Keychain` helper (nothing wrote its two items), the interface-only `ChoirDesk` ("choir") and `StageRegistry.defaultConfiguration` (moved to `Tests/.../TestConfiguration.swift`) from `BasicStages.swift`; `RouteStretchSource`, `StretchSource` and `StretchEvent` from `RadioProgramme.swift` (`StretchDecoder` and `StretchEvent` now live in `Tests/.../StretchDecoder.swift`, because the player and run sheet tests feed segments through them; shared code has its own in `programme-model.ts`); `StretchCandidate`, `fromFront`, `body`, `resolveText` and the bounds; `PaperModel.swift`; `Moment.swift`; `ListenText.swift`; `Learning.swift` (`SteerTap`); `PageText` and `HTMLText`; `EditionText`; `PodcastClips.fetch`/`ensure` and `CandidateClip`; and from `Takes.swift` everything but `TakeResponds` and the three read-only copies `M/+Takes` still writes (`TakeOwn`, `TakeMutes`, `TakeMirror`, now written and wiped but read by nothing). `RemoteHost.allows` moved next to `SameOrigin` (the mail review link uses it).
Kept, with the reason: `FixedSystem1`, `StubEchoDesk` and `FixtureProgramme` (the Mac replay proofs, `scripts/proof-replay.sh` and `proof-models.sh`, name `fixed`, `stub-echo` and `fixture` in `mobile/fixtures/configs/*.json`; and `LabRuntime` loads the fixture edition when not told `emptyProgramme`); `ChoirDeskStage` (Texture desk, `choir-desk`: registered by `registerChoirStages`, in no listener configuration; reached only by `proof-choir-client.sh` and `fixtures/configs/choir.json` and its tests: an owner decision); `SegmentTracker` (no reader outside tests, and it logs nothing, but the player's tests assert behaviour through its events, so it goes only with a rewrite of those tests); `NotebookCache` and `EditionLookup` (`AutoradioModule+Notebook` writes the cache and `NotebookStore` looks the edition up; the cache is now read by nothing but the sign-out wipe).
The checker's roots gained `mobile/AutoradioLab/modules/` (`ModelsEntry.swift`, 133 lines, was outside every root). Ceiling: 46 files, 6,224 lines, 30 names before; 43 files, 5,463 lines, 30 names after (the deletion alone is 42 files, 5,330 lines, minus 894 lines).

Second pass, same day (review seat 1, accepted with fixes):
- `mobile/fixtures/configs/choir.json` names the programme stage `choir-front`, which no Swift file registers (the programme stage is `radio`; `rg choir-front mobile/AutoradioKit` finds nothing, on `origin/lane/exact-train1` as well as here). `StageRegistry.make` throws `unknownStage` for it (`Core.swift`, `make`), so `mobile/scripts/proof-choir-client.sh`, which replays with that config, already failed before the deletions above. The config and the script stay, because `ChoirDeskStage` is the owner's to decide; they will keep failing until that decision.
- Written-but-never-read stores deleted: `NotebookCache` and the `notebookText` op (native handler and the shared-code sender in `notebook-run.ts`, with its tests R5 and R5b); `TakeOwn`, `TakeMutes`, `TakeMirror` and the `takeMirror` op (native handler and `Device.mirror` in `take-session.ts`, with the tests that asserted the pushes); `ClipStore.isFresh`, `markStale` (and its one call in `ChoirApp`) and the `failed:` and `at:` parameters of `ClipStore.replace`. Sign-out still removes the files earlier builds left (`notebook-cache.md`, `takes-own.json`, `takes-muted.json`, `takes-copy-own.json`, `takes-copy-muted.json`). `TextureText.fetch` stays: `NotebookStore.writeTopicsNow` calls it.
- `SegmentTracker` stays. `LiveAudio.swift:634` (`public let playback = SegmentTracker()`) holds it and the player calls `unitBegan`, `unitEnded`, `paused`, `cut` and `documentReplaced` on it; nothing outside the tests registers an observer, so its output reaches no production code, but the player tests read the player's behaviour through its events.
- Ceiling after this pass: 43 files, 5,339 lines, 30 names.

## Update 2026-11-23 (night, lane `exact-flip-player`): the flip is already in; what was left, and what still is

Read from the tree at `baf0dd778`, not from the 2026-11-21 plan above: the flip it describes has landed with the train. `RunSheetPlayer`, `RunSheetStore`, `RunSheetFeed`, `Station`, `OpeningWriteAhead`, `TurnQueueWire`, `QueueView` and `RadioProgramme`'s writer are gone; the run sheet, the queue, what plays next (`radio-director.ts`), the opening (`phoneOpening`), the desk's writer (`radio-writer.ts`), the bar's run-sheet facts (`barRun`), the radio page (`radioPageFacts`, `radioPageAct`), the queue sent with a voice turn and the server's `queue_ops` applied (`voice-queue.ts`) are shared code over the one `deck` op. Native keeps the deck, the voice and a mailbox (`RadioBridge`: `radioFacts`, `radioSet`).

Fixed tonight in shared code (two defects measured on 2026-11-23): the saved queue was discarded whenever an opening was placed after half an hour (`Director.open` removed every desk and phone entry ahead); nothing put the paper's front stories after the opening when the desk was slow, dry or down. Now the saved queue is kept, and with nothing ahead the director places the front stories the phone holds (`frontFill`). Tests: `radio-never-waits.test.ts`, `radio-front-fill.test.ts`. Deleted Swift: the story list in `ChoirApp` and `ChoirFront.stories` (the paper's list is pushed by shared code with the queue snapshot), the radio view's item window, `radioJump`, `RadioPage.length`.

Still Swift, each judged from the code:
- `RunSheet.swift` (wire types), `RunSheetEntries.swift`, `RadioBridge.swift`, `RadioPage.swift` (`openAddress`, `builtOn`), `PodcastClips.swift`, `AutoradioModule+Joins/+Podcasts`: the podcast and clip handlers and the take recorder's "responds to" still build and read entries natively. They belong to the podcasts, clips and takes moves (the `exact-rest` lane); they go when `clipPlay` and `podcastPlay` send plain words to `radioPlayAudio` and the recorder is handed its story.
- `SegmentTracker.swift`: no production reader. It stays because three test files read the player's behaviour through its events (`SegmentTrackerTests`, `RepairTrackerTests`, `Repair2TrackerTests` and the player tests of `ProgrammePlayerTests`, `ProgrammePlayerRepairTests`, `ProgrammePlayerRepair2Tests`: about 30 tests); deleting it is a rewrite of those against the player's own state, not done blind.
- `BarState.swift`: the radio view is now the native player facts (the mic level, the speed, the sentences of the block being read, the level the voice yields at); the contracts read only `title`, `playing`, `level`, `speedValue` and `micLevel` of it. The other fields are read by the Swift tests and `radioFacts` and can go with `SegmentTracker`.
- `PlaybackContinuity.swift`, `Playhead.swift`, `PlayerControls.swift` (`PlaybackSpeed`, `SpeedStore`, `StoryCursor`), `ChoirPlayback.swift`, `Deck.swift`: the audio session, the lock screen and the position of the audio that was rendered. On the voice list or the deck's, not run-sheet logic.
- `ChoirApp.swift` is the engine's controller (sign-in, the deck's engine, the sign-out order); its play paths are gone. `ChoirClient.swift` is the `exact-rest` lane's.
- `ChoirEnvironment.playing(...)` now answers nothing in production (its one writer, `ChoirFront.load`, is a test helper); the radio view's `route` is empty, so the radio's sentence follow-along in the reader is not lit. It was already empty on the deck path; the shared reader's own follow (`importedFollow`, the reader's aim) is the replacement.


## Update 2026-11-24 (lane `exact-rest-a`): the run sheet's Swift copy, podcast clips and the dead Swift

Moved to shared code and deleted in Swift: the run sheet's Swift types (shared code now pushes the voice's retrieval the entries on air and ahead, the take recorder the story on air and the reader's follow-along the page on air: `voice-facts.ts`), podcast clips and what is playing (`clip-play.ts`, `podcast-now.ts`), mail's "Read this to me" (the shared radio's `radioListen`), the themes' carry-over read, and 20-odd dead non-voice Swift files. The gate's count is 40 files, 4,657 lines, 22 names (from 49 / 5,741 / 26). Still Swift on purpose, and the next two changes: the Texture editor (`TextureEditor`, `TextureBody`, `TextureReaderSource`, `Notebook`, with `Pages.swift` and the page-model source they use) and the mail seal (`MailSeal`). They wait for a redesigned editor state model and mail identity (four review rounds found new defects in each fix: the contract's rule 7). Not proved: a native Exact build and a device.
