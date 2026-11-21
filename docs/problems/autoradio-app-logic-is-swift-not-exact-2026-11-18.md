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
