# What non-voice native code is left, and why

Standing: governing | kind: happened | settled-by: a reading of origin/main 46a18adac on 2026-11-28 | note: read from code and grep; nothing built or run, and "no caller" means no caller found by grep

`python3 -I scripts/check-autoradio-native-boundary` at 46a18adac: 30 files,
2,777 lines, 12 interface names, all Swift. No Kotlin file is outside the voice
list (`AutoradioModule.kt` is listed as the WebView and external-open seam).
The listed decisions are in
`docs/definitions/choir-finish-the-move-to-exact-2026-11-27.md`.

The finding: almost nothing is clean (a). Exact has no call from native code
into shared code (said in `RetrievalFacts`, on the voice list), so every
native mirror of voice-turn state stays native. What is left splits into the
account key, the mail seal and sign-out (owner decisions D6, D4, D7), and
voice-turn or deck state that is not on the list and should be.

Categories: (a) movable now, no owner decision; (b) blocked on a named
decision; (c) voice or a native-only report, belongs on
`scripts/autoradio-native-voice-list.txt` (a recommendation: adding is the
owner's act); (d) dead.

## Counts

| Category | Files | Lines |
| --- | --- | --- |
| (a) | 1 | 15 |
| (b) | 6 | 929 |
| (c) | 23 | 1,833 |
| (d), whole files | 0 | 0 |
| Total | 30 | 2,777 |

(b) is D6 for five files (ChoirClient, ChoirApp, ModelsEntry, ProofKey,
ChoirSignIn: 857 lines) and D4 for MailSeal (72). D7 touches ChoirSignIn and
the legacy-key part of ChoirClient but moves no whole file. Parts of files
are (d) or (a): the three unused notification constants in `+Platform`
(about 12 lines), `RemoteHost` in ChoirClient (about 7, test-only), the
old-build file deletions in `AccountWipe` and `Takes.swift` (about 33).

## The decisions, quoted

- D2: "Which files on the voice list are really voice (station E2)?"
- D4: "Does the mail seal go, so that the phone keeps its copy of a person's mail unencrypted by the app (the phone's own file protection still applies)?" Default: the seal stays.
- D6: "Where is the account key held on the phone?" Default: as today, in two hands.
- D7: "May the launch-time revoke of an old Swift build's key be deleted?" Default: it stays.

## The 30 files

Paths are under `mobile/AutoradioKit/Sources/AutoradioKit/` unless a prefix is shown.
`+X` is `mobile/exact-package/apple/AutoradioModule+X.swift`.

| File | Lines | What it decides | Callers (grep) | Cat | How or why |
| --- | --- | --- | --- | --- | --- |
| Choir/ChoirClient.swift | 442 | the voice turn's HTTP client, key store, origin check, computer id | the three desk stages, ChoirSignIn, TurnGate, ModelsEntry, replay tool, tests | b (D6, D7) | the turn's streaming POST is native by design (gaps.md row 36); the key store waits on D6, the legacy key on D7 |
| Choir/RadioProgramme.swift | 255 | splits a spoken entry into speech blocks and now-playing text | ChoirApp.speakSegment, LiveAudio | c | speech block preparation for the voice |
| Choir/ChoirApp.swift | 240 | stage registration, deck and audio wiring, native sign-out, error words | AutoradioModule, ChoirPlayback, +Bar, LabRuntime | b (D6) | about 160 lines are deck and audio wiring; sign-out and signed-in wait on D6 |
| +Platform | 157 | audio-session alias, console log file, `open`, `outputName` | AutoradioModule, +Onboarding, +Reader | c | a platform seam; 12 lines of unused constants are dead |
| SavedAnswerHolder.swift | 148 | the on-phone model's first answer from saved passages | ChoirApp, LabRuntime | c | a voice-turn stage |
| Conversation.swift | 124 | the turn's question and answer record | TurnEngine, LabRuntime, +Talk | c | written during a turn; `conversation.ts` only reads it |
| +Bar | 120 | engine events and mic frames into `barFacts`, `radioView`, `radioSpeed` | AutoradioModule, ModelsEntry | c | mic level and engine events are native; `radioSpeed` is the unanswered-on-Android op |
| BarState.swift | 115 | holding line, revision and error flags, mic level, speed | +Bar, LabRuntime, ChoirPlayback, AccountWipe | c | engine-event mirror |
| EditionSetup.swift | 114 | first-run spoken topic setup | +Onboarding, +Retrieval, ModelsEntry | c | voice stages; picks logic is `edition-picks.ts` |
| modules/apple/ModelsEntry.swift | 111 | routes native ops, sign-out and wipe, mail seal, proof key | the app's native module | b (D6, D4) | mostly voice pass-through; sign-out, seal and proof key wait |
| AccountWipe.swift | 103 | native stores wiped at sign-out, the generation counter | ChoirApp, +Onboarding, AutoradioModule | c | voice and deck stores stay native; about 25 lines of old-build deletion are (a), see move 2 |
| +Reader | 101 | WKWebView, link policy, `holdPage`, `readerOpenExternal` | AutoradioModule, ModelsEntry, +Spine | c | Exact has no web view (EXACT.md:342); the Kotlin file is already listed as this seam |
| +Takes | 87 | mic gate, take recorder host | AutoradioModule, ModelsEntry | c | microphone and speech; TakeSession is listed |
| RadioBridge.swift | 80 | mailbox between the voice turn and the shared run sheet | +Queue, +Talk, +Takes, +Bar, +Retrieval | c | the native-to-shared push, as RetrievalFacts |
| +Spine | 77 | `choirAsk`, hands-free toggle, `pageRequest` | AutoradioModule, ModelsEntry | c | drives the voice runtime |
| Choir/MailSeal.swift | 72 | AES-GCM seal of the mail copy, Keychain key | ModelsEntry, `mail-seal.ts` | b (D4) | Exact's `crypto.subtle` has no AES, so shared code cannot replace it |
| HeldPages.swift | 69 | native copy of pages for the local responder | +Spine, +Reader, +Retrieval, LocalAck, AccountWipe | c | the voice answers synchronously |
| PlayerControls.swift | 58 | speed clamp, `SpeedStore`, `StoryCursor` | LiveAudio, RemoteAudioPlayer, BarState, +Bar | c | used by the audio engine; `SpeedStore` (14 lines) is (a), see move 3 |
| Choir/ChoirProgramme.swift | 55 | sentence splitting | ChoirDesk, LiveAudio, RadioProgramme | c | only voice code calls it |
| +Talk | 44 | `conversationEntries`, `answerAction`, voice profile ops | AutoradioModule | c | voice |
| ProofKey.swift | 35 | simulator proof key import into the Keychain | AutoradioModule, ModelsEntry (simulator only) | b (D6) | a test seam for the key store |
| Choir/ChoirSignIn.swift | 29 | native sign-out and `revokeNow` | ChoirApp | b (D6, D7) | `revokeNow` exists only for the legacy key |
| +Queue | 28 | `radioNote`, `radioSet` into the bridge | AutoradioModule, ModelsEntry | c | the other half of the bridge |
| SourceLink.swift | 23 | in-app, external or refused routing of a URL | +Reader | c | the web view's decision is synchronous; Kotlin carries a copy |
| Choir/Takes.swift | 22 | `TakeResponds`, legacy take paths | RadioBridge, TakeSession, AccountWipe | c | about 8 lines of legacy paths are (a) |
| +Settings | 19 | `settingsFacts`: version, running state, turn history | AutoradioModule | c | a native-only report |
| QueueVoice.swift | 19 | a hook box | TurnEngine, +Talk | c | voice |
| +Notebook | 15 | `contentBusy`: is remote audio on air | content-run.ts:53, via ModelsEntry | a | move 1 |
| RunSheetTypes.swift | 8 | two enums | Deck.swift | c | deck types |
| PlaybackSupport.swift | 7 | `onMainThread` helper | ChoirApp, +Deck, +Takes | c | deck and voice helper |

## Kotlin

All 16 `.kt` files are under `mobile/exact-package/android/` and all are on the
list. Only `AutoradioModule.kt` is partly non-voice: about 170 lines are the
WebView, `route` and `openExternal`, a platform seam with its reason beside it
(`route` duplicates Swift `SourceLink`). `DeckController.kt` (389) and
`DeckJournal.kt` (320) are the deck's rules and journal: native by the deck
design, not because they are voice. `ModelDownloadManager.kt` (179) is the one
that might be non-voice: shared code has `net.toFile` and `crypto.subtle.digest`,
but nobody checked that llama.cpp can read the path Exact writes. `NativeTts.kt`
(16) has no caller found by grep.

## The three largest (a) moves, by lines removed per risk

1. `contentBusy` (+Notebook, 15 lines, 19 with its `ModelsEntry` branch): read
   `deckState` in shared code (`deck-native.ts` already does) instead of asking
   native. Low risk: no secret, no audio write. The silent failure would be a
   content sync running while remote audio plays. It also gives Android a real
   answer; today the app module answers nothing there, so busy is false.
   Unverified: that the item kind sits on the `item` object of `deckState`.
2. Old-build file deletions (about 33 lines: `AccountWipe.oldBuildFiles`,
   `removeOldBuildFiles`, `TakeFiles.legacy`, and the launch calls at
   `AutoradioModule.swift:145-150`). CLAUDE.md says a moved page carries nothing
   over until launch, so no decision blocks it. It is on the sign-out path, so
   red with two seats; one E6.1 reviewer once sent a smaller version back as
   cheap protection. Confirm that no owner device holds old files first.
3. `SpeedStore` (14 lines, plus its load and save in `+Bar`): keep the speed
   in a shared persisted setting. Medium risk: a late push at boot would reset
   the speed to 1.0 and touch audio rate. Only (a) if shared code pushes the
   speed to native before the first play (not verified).

Plain deletions beat all three on lines per risk but are (d): `NativeTts.kt`
(16), the unused notification constants in `+Platform` (about 12) and
`RemoteHost` (about 7).

## Recommendations for the owner's list

Add the 23 (c) files, or rule file by file under D2. Then the gate reads
15 lines before D4, D6 and D7.
