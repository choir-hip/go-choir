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

## Update 2026-11-20 (the takes page moved)

The Takes section is shared code: `mobile/AutoradioLab/take-session.ts` (the session: publish, refresh, delete, mute, every word, the listener's own ids and muted voices) over `takes.ts` (the wire and the page model, already shared). Tests: `take-session.test.ts` (S1 to S8, U1 to U3), `takes.test.contract`; `TakesPage.swift`, the publish, list-of-mine, delete and record half of `TakesClient`, `MyTake`, `Mp4Header`, `TakeBounds`, the multipart body and the Swift `TakeSession` publish half are deleted, and the native sources `takesPage` and `takesAct` with them. What stays native, with the reason: the RECORDER (`TakeSession.swift`, on the voice list: microphone, the radio's gate, the file, the recogniser, the preview, the ceiling stop; it answers one snapshot, `takesRecorder`, and codes for why a take ended, the words being shared); `takeAudio` (the file is native's), `takeSteerMute` (the steer queue is native), the take audio fetch for the run sheet. Still Swift and not moved: the programme request (`RadioProgramme.candidates`) lists the public takes and filters them by the listener's own ids and muted voices, so `TakeOwn` and `TakeMutes` stay as READ-ONLY copies that shared code pushes (`takeMirror`) after every change, and `TakeStore`, `TakesClient.list` and `audio`, `TakeCandidates` remain until the programme request moves. Data on an upgraded phone (changed after three reviews, `docs/problems/takes-in-exact-three-reviews-no-landing-2026-11-20.md`): the Swift files (`Application Support/Autoradio/takes-own.json`, `takes-muted.json`) are adopted into `app:/data/takes.json` only when one of their take ids is a take the server lists as the signed-in account's live take, and are then deleted; with no such id they are deleted unadopted (an account with no live take loses its old mutes); a server failure leaves them. The programme's copy is now other files, pushed by shared code. The proof and the table are in `docs/reference/AUTORADIO-TAKES.md`, "The phone's own state". An unpublished take was never stored across launches (the file lives in the temporary folder, the state in memory), and still is not: a take in review belongs to the recorder, and a restart of the shared code finds it. The floor is now measured by the recorder (the file's length through `AVAudioFile`, the recorder's clock if unreadable), not by the MP4 header; the claim at publish still comes from the header (`mp4Seconds`). Ceiling 95/18366/93 to 94/18144/92.
