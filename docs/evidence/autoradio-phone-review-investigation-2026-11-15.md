# Autoradio phone review: what the code says, and what Mike's voice loop is (2026-11-15)

Read-only investigation. Nothing was built, installed or launched on a phone
or simulator, and nothing in `mobile/` or in Mike was changed. The checklist
is the eleven items of
[autoradio-owner-phone-review-2026-11-15.md](../problems/autoradio-owner-phone-review-2026-11-15.md).
Paths are in `mobile/` unless a root is named; `K` is
`mobile/AutoradioKit/Sources/AutoradioKit`, `L` is `mobile/AutoradioLab`,
`X` is `mobile/exact-package/apple`, and Mike is the owner's repository at
`/Users/wiz/Dev/mike` (read with `git show` and file reads only; its remote
was not fetched, so "Mike main" means the checkout on this machine, commit
66b5b7f05 of 2026-10-31).

Everything below is "read", not "seen on a device". A claim that needs a
device is marked **not verified** and names the observation that decides it.

## 0. Findings in short

1. The interview shows the question twice because the code records it twice
   (a defect read from the code, section A8). It hears nothing because the
   interview runs under the user's talk setting, whose default is "Tap to
   talk", and that addressing ignores speech without a button press, while
   the screen says the microphone is open in text that is not conditional on
   anything. Both are read from code, not seen on the phone.
2. Text to speech exists. The programme player speaks with Apple's system
   voice (or a per-phone downloaded model voice) through the app's own audio
   engine. What it lacks is a way in from a story's page, a reader, Textures
   or a file, and it has one proven-by-reading defect (it ends each spoken
   paragraph at the synthesizer's first empty buffer, the exact defect Mike's
   README records as cutting speech at about 13 seconds). Why Play makes no
   sound on the phone is **not verified**; the candidates and the
   deciding observation are in A4.
3. Mike's hands-free loop is a native Swift engine, 6,562 lines in 21 files,
   of which one file (`MikeSpeechPlugin.swift`) imports Capacitor. The rest
   is portable Swift. What does not port as it is: its audio-session calls,
   its Capacitor bridge, the TypeScript conversation layer, and two
   third-party packages that Exact can only take as static xcframeworks.
4. There are seven voice implementations in this repo and none of them is
   Mike's. The 2026-11-10 pass-1 definition ruled that lane V port Mike's
   plugin; the code that landed is a different pipeline (AutoradioKit's
   stage engine).

## A. The eleven items

### A1 and A3. Cards, the deck, article navigation

What exists. The main view is a deck of full-screen pages
(`L/pages.contract:93-130`, `L/members-stack.contract`). The deck's
order is a fixed list of five key pages,
`state order = ["paper:front", "textures:", "file:/", "mailbox:inbox",
"settings:root"]` (`members-stack.contract:18`), and an opened story is a page
put on top (`openPage`, `members-stack.contract:42-48`). A sideways swipe
flies the top page off and sends it to the bottom, and the next page of
`order` comes up (`pageRelease`, `cycleDeck`,
`members-stack.contract:62-85`). So a swipe goes from page to page of the
deck, never from article to article of the paper: after a story the next
card is Textures or Files, not the next story. That is the observed
behaviour of item 1.

Only the top page is mounted; the two beneath it are `Placard`s, a kind word
and the page title (`pages.contract:62-74`, `deckWin = order.slice(0, 3)`).
A placard for the Paper page therefore reads "Paper" and nothing else: this
is the likely source of the screenshot's card with only the word "Paper"
(the launcher's list "In the deck, top first", `L/launcher.contract:105`,
is the second candidate; **not verified** which screen the owner meant).

The story stack the owner liked was removed on purpose: `members-cards.contract:1-4`
says "the pile, the story stack, the card drags ... are gone (owner ruling
19, 2026-11-11: the page stack is the main view)". The Swift side is still
there and unused: `StackSession`, `storyStack`, `cardStories`, `stackDecide`
(`X/AutoradioModule+Cards.swift:109-118,197-218`).

Known deck defect: a sideways swipe never reaches the deck on the Textures
page (`docs/problems/autoradio-deck-swipe-stalls-on-textures-2026-11-11.md`,
cause unknown, simulator only).

What a card stack of all paper articles needs, from the code:

- `order` seeded from the paper's order instead of five fixed pages. The
  list exists: `choirStories` returns the front list in the listener's
  shaded order (`AutoradioModule+Cards.swift:99-105`), limited to 30 stories
  (`K/Choir/ChoirApp.swift:103`).
- The key pages (Paper, Textures, Files, Mail, Settings) out of the swipe
  cycle and reached from the launcher only (the launcher already lists
  them, `launcher.contract:99-104`).
- The next one or two cards must not be placards. Today a drag shows a
  placard under the top page and mounts the real page only after the Kit's
  focus has caught up (`pages.contract:60-62`, `members-stack.contract:7-11`).
  For a stack the next story's text has to be resolved ahead
  (`ChoirFront.load`, `K/Choir/ChoirProgramme.swift:77`).
- Item 3 (page turning, more animation): the deck animates with springs on
  `translate`, `scale` and `rotate` (`pages.contract:93-95`); a page-turn
  needs a different transform, and whether Contract can express a curl is
  **not verified** (`docs/reference/exact/rendering-and-motion.md` was not
  read for this).

### A2. Horizontal scrolling

Every sideways scroller in the app is a `scroll overflow-x="scroll"` row:

| Where | Anchor |
| --- | --- |
| Paper section chips | `L/paper.contract:197` |
| Launcher key pages | `L/launcher.contract:99` |
| Mail folder tabs | `L/mail.contract:81` |
| Files sort row and breadcrumbs | `L/files.contract:153,162` |
| Reader tables and code blocks | `L/reader.contract:181,186` |
| CSV viewer | `L/viewers.contract:253` |

The Mail page drawn shifted left with its title and first tab cut off: the
Mail row at `mail.contract:81` is the only one of these that has neither
`width="100%"` nor `flex-shrink=0` on its buttons (the Paper row, line 197,
has both; the Launcher row, line 99, has `width="100%"`). Without a width a
row inside a column can size to its content and make the page wider than the
screen. This is a code-supported candidate, **not verified**: the deciding
observation is to give the Mail row `width="100%"` in the simulator and see
the page stop shifting. The page's vertical `PageScroll` is `scroll`
without an axis (`ui.contract:126`), so whether it also scrolls sideways
when its content is wider is **not verified** either.

Why horizontal scrolling breaks the swipe: the deck listens for a horizontal
`pan` with `touch-action="pan-y"` (`pages.contract:109`), and a horizontal
scroller on the page competes for the same gesture. The deck-swipe problem
record found the same family of failure on Textures.

### A4. Audio, text to speech, what Play does

There is a TTS path. The chain a story's Play button takes:

1. `L/paper.contract:37,64,83` (per-story Play) calls `playStory(route)`,
   `L/lab.contract:52`, which sends `choirPlay`.
2. `X/AutoradioModule+Spine.swift:67-75` starts the engine if it is not
   running (`runtime.start`) and calls `ChoirApp.play(route:)`
   (`K/Choir/ChoirApp.swift:147`).
3. That loads the story as a programme of speech blocks
   (`ChoirFront.load`, `ChoirProgramme.swift:77-92`), then
   `ProgrammePlayer.start(at:)` and `player.play()`.
4. `ProgrammePlayer.playBlock` (`K/LiveAudio.swift:~580-600`) makes the
   configured `voice` stage, collects its samples and plays them through
   `LiveAudio.playProgramme` (`LiveAudio.swift:322`), the app's own
   `AVAudioEngine` with a programme node and a voice node.
5. The shipped listener configuration names the voice `avspeech`
   (`ChoirApp.swift:18`): `AVSpeechVoice`, which calls
   `AVSpeechSynthesizer.write` and turns the buffers into samples
   (`K/AppleStages.swift:174-225`).

The bar's Play button sends `choirPlayPause` (`lab.contract:46-47`), the
player layer's Play is the same (`player.contract:132`). Mail's "Read it to
me" speaks through `speakText` (`mail.contract:164`,
`X/AutoradioModule+Mail.swift:70-75`, `K/LabRuntime.swift:124,177`).

What does not exist: any way to speak a story from the reader page, a
Texture, a PDF or EPUB, a file, or captured text. Grepping every contract
for read-aloud finds only the paper's Play buttons and mail's one. Item 10
is therefore a missing entry point, not a missing engine. `speakText`
(`LiveAudio.swift:551`) already turns any text into a programme, capped at
200 paragraphs (`prefix(200)`).

Defects read in the path (none proves why the phone is silent):

- `AVSpeechVoice` resolves an utterance at the first empty buffer
  (`AppleStages.swift:214`, `if pcm.frameLength == 0 { once.resume(()); return }`).
  Mike's README (`mobile/plugins/speech/README.md`, opening paragraph) records
  that `AVSpeechSynthesizer.write` sends empty buffers between phrases and
  that stopping at the first one "cut every chunk off after about 13
  seconds"; Mike ends at `didFinish`. The app's paragraphs are up to 600
  characters, so a paragraph would be cut at its first phrase pause. Read,
  not heard.
- `playBlock` swallows a failure to make the voice: `guard let voice = try?
  registry.make(.voice, ...) else { return false }` (`LiveAudio.swift:585`),
  which logs nothing. A block that makes no sound is recorded as failed, not
  as an error, and the player moves on, so a silent programme looks like
  success on the screen.
- `LabRuntime.start` catches a failure (a refused session claim, a mic or
  engine error) into `snap.message` and carries on with `started` false
  (`K/LabRuntime.swift:144-170`). Nothing then plays and nothing says so on
  the Play button.
- Voice setup may have replaced the voice. When first-run setup finishes it
  sets each stage to what the per-phone configurator chose, if the kit
  registered it (`X/AutoradioModule+Onboarding.swift:146-151`); the voice
  can then be Kokoro or Pocket TTS (a model download of tens to hundreds of
  MB, `mobile/README.md` "On-device model stages"). A model voice that fails
  to load returns no frames and is silent.

The programme the owner heard was only idents and bridges because the phone
sends no candidates (overnight report, 2026-11-15; `RadioProgramme.swift:362`
builds a request body of `request_id`, `cause`, `local_time` and `tz` only).
That is why "a single story cannot be played" is two separate things: the
programme has no story in it, and the single-story Play path has an
unverified fault.

Observation that decides the cause of the silence, no build needed (a build
is already on the phone): press Play on one story, then open the Lab page's
Trace (`lab:trace`). Look for `error` events with stage `start`, `mic`,
`voice` or `player`, and for a `programmeAudioStart` event. No
`programmeAudioStart` with a `voice` error means the voice stage; a `start`
error means the session claim; neither means the engine never reached
`playBlock`.

### A5 and A6. The prompt bar and search

The prompt bar. Its placeholder is the constant "Prompt Autoradio..."
(`L/bar.contract:94`). The field's Send action does two things
(`L/spine.contract:46-49`): it records a signal and sends
`choirAsk(text, aim=<the page in front>)`. `choirAsk`
(`X/AutoradioModule+Spine.swift:80-93`) calls `LabRuntime.ask` ->
`TurnEngine.typedTurn` (`K/TurnEngine.swift:197`): one turn through the
System 1 stage, the holding voice and the configured desk, which in the
shipped configuration is `AutoradioDeskStage`
(`ChoirApp.swift:19`), a stream from `POST /api/autoradio/turns`
(`K/Choir/AutoradioDesk.swift:156`, contract
`docs/reference/AUTORADIO-TURNS.md`). The answer is spoken sentence by
sentence. So the bar is a conversation turn aimed at the page in front; it
is not search and not a command line. Nothing on the screen says so, which
is item 5. Its own comment calls it a place where "people instruct as well
as ask: play, skip, open mail, rewrite this" (`bar.contract:92`); whether the
turn route acts on such instructions is **not verified** (the route's
reference page was not read for that).

The search field is the launcher's, "Search pages, files, mail"
(`launcher.contract:85`). It calls `launcherData(q)`
(`X/AutoradioModule+Cards.swift:145-157`), which runs `LauncherSearch.hits`
(`K/OpenPagesUI.swift:55-89`): a local substring and prefix match over
(a) the open pages' titles, (b) five fixed places, (c) recently opened or
closed pages (from the signal log, `RecentPages`), and (d) the stories of
the current front list, matched on title, dek and entities. Three facts
explain "only what was already read":

- (a) and (c) are pages the listener already opened;
- (d) is at most 30 stories (`ChoirApp.swift:103`), the current front list;
- the entity map that would let a search find "Nvidia" in a story that does
  not say it in its title is empty outside demo mode
  (`AutoradioModule+Cards.swift:30`, `Demo.isOn ? ... : [:]`).

The placeholder promises files and mail; the function searches neither (its
own comment says those rows "join when those lanes expose their own
listings", `OpenPagesUI.swift:56-57`).

Corpus search that exists on the server:

- `GET /api/platform/retrieval/search?q=` is public
  (`internal/proxy/handlers.go:990`, `internal/proxy/platform_public.go:178-203`)
  and reaches `Service.SearchPublished`
  (`internal/platform/service_publication_read.go:639-791`). It walks up to
  `publishedPieceScanMax` (2,000) publications, reads each one's blob, counts
  the lower-cased query as a substring in the text, adds 3 for a title hit
  and returns at most 20 results with snippets. No ranking beyond the count.
  The phone does not call it.
- `SearchPublishedPieces` (`internal/platform/publication_pieces.go:123-`) is
  a BM25 ranking over the whole archive of published stories and opinion
  pieces by their latest version, default 10, at most 25 results. It is
  served only at `/internal/platform/published/search`
  (`internal/platform/handlers.go:341,673`) and the proxy admits it only for
  a guest bound to a live computer
  (`internal/proxy/guest_authority.go:94`). It has no phone-reachable route.
  It is the one research uses (`internal/researchtools/published_search.go`).

### A7. Search and edition as one feature

Facts for the design question. The edition today is local and thin: the
paper's order is shaded on the phone by gestures
(`Preferences`, `AutoradioModule+Cards.swift:44,99-105`); the interview writes
an edition document through `UnbuiltEditionWriter`, which throws in a live
build (`K/Interview.swift`, "Not built here ... fails honestly"; demo uses a
fixture); `EditionLookup` asks the computer for an existing one
(`AutoradioModule+Onboarding.swift:420`). The front-list route takes only
`limit` and `section` (`ChoirProgramme.swift:67-68`), so there is no
filter-the-paper request today. Options are in section C, slice 4.

### A8. The interview screen (step 4 of 4)

Screen: `L/onboarding.contract:255-300`. Controller:
`X/AutoradioModule+Onboarding.swift`. State machine: `K/Interview.swift`.

**Why the radio's question shows twice. Cause read from the code.**
`startInterview` calls `s.start()` (`Onboarding.swift:232`). `Interview.start`
sets the status to running and `askCurrent` records the question as a radio
turn (`Interview.swift:307-316`, `record`, line 303). Then `startOnEngine`
sets the programme to `InterviewProgramme` and loads it
(`Onboarding.swift:238-249`). `InterviewProgramme.load` decides with
`s.snapshot.status == .notStarted ? s.start() : s.resume()`
(`Interview.swift:746`). The status is already `.running`, so it calls
`resume()`. `resume` proceeds when the status is running and turns exist
(`Interview.swift:286`), says "Welcome back." (not recorded) and calls
`askCurrent(first: false)` again, which records the same question a second
time. The screen lists `ob.talk`, the last seven turns
(`Onboarding.swift:340`), so it shows the question twice. It also means the
phone speaks "Welcome back." then the question to a person who has just
begun. The demo path never calls `startOnEngine`, which is why the
simulator proof did not show it.

**Why no owner turn appears while the screen says the microphone is open.**
Two things are separate in the code:

1. The sentence "The microphone is open. Talk any time, even over me." is
   static text shown whenever the interview is not paused
   (`onboarding.contract:285`). It reads no engine state.
2. The interview does not set the addressing; it applies the user's talk
   setting (`Onboarding.swift:242-243`, "The talk mode has one owner:
   Settings. The interview reads it and does not set it."). The setting's
   default is `button` ("Tap to talk"; `K/SettingsData.swift:39-45`), which
   maps to `ButtonAddressing` (`SettingsData.swift:86-88`,
   `K/BasicStages.swift:45-53`), described in its own descriptor as "Speech
   without the button is ignored". In `TurnEngine.ingest` a speech start
   goes to `route(.speechStarted)`, `ButtonAddressing.decide` answers
   `.none` for it, no turn begins, nothing is transcribed and the desk
   (`InterviewDesk`, which is the only caller of `InterviewSession.hear`,
   `Interview.swift:765-770`) is never called. So no owner turn is ever
   appended. The same applies to the microphone check step, which starts the
   engine and waits 20 seconds for a transcript (`Onboarding.swift:186-195`);
   under button addressing that transcript cannot arrive, which would send
   that step to its "I could not hear anything" branch (**not verified**
   that the owner saw it; the owner reached step 4).

The cause is shown by the code only on the condition that the owner's
Settings still says "Tap to talk", which I could not read. The candidates
and the observation that decides between them:

| Candidate | Observation on the phone |
| --- | --- |
| A. Talk setting is "Tap to talk" (the code's cause) | Settings shows "Tap to talk"; tap the bar's mic during the interview and speak: an owner turn then appears under "So far". Change the setting to "Always listening" and speak with no tap: it appears. |
| B. Setting is open but speech recognition is not authorised or unavailable (`SFSpeechTranscriber.begin` throws "speech recognition not authorized", `K/AppleStages.swift:37-41`) | Trace shows an `error` with stage `apple-speech` or `turn`; iOS Settings shows Speech Recognition off for the app. The grant list does include `device.speech-recognition` (`L/app.ts`). |
| C. Setting is open but the microphone was refused or the engine did not start | Trace shows `error` stage `mic` or `start`; the orange microphone indicator is absent. |
| D. The per-phone setup replaced the transcriber with a model stage that did not load | Trace shows the transcriber id; a model-load error. |
| E. The voice processing or session mode drops the mic (`alwaysTalking` policy, `Onboarding.swift:244`) | Trace shows the `audioSession` claims and any mode mismatch note (`AutoradioModule.swift:36`). |

A is the only one the code proves can happen with default settings.

### A9. Transcription anywhere there is a text block

Today a spoken turn is the only transcription, started from the bar's mic
(`spine.contract:56-62`) and sent as a turn. The text fields (bar field,
launcher search, mail compose) are plain `input` elements, so they depend on
the keyboard's own dictation (**not verified**). Mike's live dictation is a
separate, portable piece (B, `LiveDictation.swift`).

### A10. Text to speech for any long text

Covered in A4: `speakText` (`LiveAudio.swift:551`) is the entry point and is
wired for mail only. Reader pages (`L/reader.contract`), Textures, files,
PDF and EPUB viewers (`L/viewers.contract`, `K/Epub.swift`) and captured
text have no Listen control. A reader has the text already
(`TextureReaderSource.swift`, `PublicationTextSource`).

### A11. How sources open

The source sheet's "Open original" button (`L/reader.contract:259-262`)
calls `openLink(c.url)` (`L/members-reader.contract:68-69`), which is the
Contract builtin `openURL`. The module's own `UIApplication.shared.open` is
used for the sign-in approval page and a mail approval link
(`AutoradioModule.swift:96,229`, `AutoradioModule+Mail.swift:60`). All of
these leave the app for Safari.

There is no web view in the stack. Exact has none by design
(`docs/reference/EXACT.md:45`, "No webview"). The one embedded-web
mechanism in the app is `iframe` on an app-owned local file, for sanitised
mail HTML and PDFs (`docs/reference/exact/ui-model.md:179-190`;
`mail.contract:176`). The module's package view list is empty
(`X/exact.json:11`, `"views": []`); `mobile/README.md` names `PDFPage.swift`
and `QuickLookPage.swift` as package views but neither file is in this tree
(**not verified** where they are). So an in-app web view is a native view to
be added, not a Contract change (slice 3).

## B. Mike's hands-free voice loop

### Where it is and its state

- Repository `/Users/wiz/Dev/mike`, a Capacitor app. The loop is the iPhone
  speech plugin `mobile/plugins/speech/` (`@mike/speech`), native Swift in
  `ios/Sources/MikeSpeechPlugin/`, and a conversation layer in the web app:
  `frontend/src/app/lib/voice/` (voiceEngine.ts, speakable.ts, clauses.ts,
  spokenPrefix.ts, turnPolicy mirror in `web/`) and
  `frontend/src/app/hooks/useVoiceChat.ts` (885 lines).
- Docs: `docs/voice-chat.md` (design, 953 lines; its "As built" section
  still says "has run in the iOS simulator only"), `docs/voice-chat-contract.md`
  (the plugin's wire contract), `mobile/plugins/speech/README.md`,
  `docs/phonon-read-aloud.md`, `docs/fluid-speech-mini-iphone.md`.
- State. The owner ruled twice that the docs are behind the code and that
  all voice modes are in daily use on the phone
  (`docs/definitions/choir-autoradio-pass1-2026-11-10.md:37-38`;
  `docs/design/resource-fabric-2026-10-31.md:846`). I could not verify that
  by running it. The newest commit to the plugin is 2026-10-25, to the
  frontend voice code 2026-10-28; the six voice branches in Mike
  (`docs/voice-turn-taking`, `perf/voice-*`, `fix/voice-load-once-and-campplus-cpu`,
  `feat/keep-screen-on-while-listening`) are all merged into Mike main (0
  commits ahead). `feat/web-voice-mode` and `feat/phone-web-voice-modal` were
  not read. There is no Mike branch in this repo; `git branch -a` here shows
  none, and the only Mike material in this repo is design prose.
  The plugin has 13 Swift test files (`TurnPolicyTests`, `ClauseQueueTests`,
  `VoiceContractTests`, `SpeakerProfileTests` and others); I did not run them.

### Architecture, from `docs/voice-chat.md` and the source

One `AVAudioEngine` with Apple's voice processing, so the echo canceller
hears Mike's own speech (`VoiceAudioEngine.swift`). The microphone is
converted to 16 kHz mono and fanned out, all running all the time:

| Job | Piece | Source file |
| --- | --- | --- |
| Speech or silence | Silero VAD, 256 ms hops, own hysteresis | `VoiceModelCache.swift`, `TurnPolicy.swift` |
| Live words, candidate end | Parakeet realtime EOU 120M at 320 ms, driven chunk by chunk and re-armed on every `<EOU>` | `EouStream.swift` |
| Confirm the end of a turn | Smart Turn v3.2 (Core ML), asked after a 0.5 s pause, again every 750 ms, hard stop at 3 s | `SmartTurn.swift` |
| Decide turn end, barge-in, false start | `TurnPolicy`, a pure value driven by timestamped events (replayable traces) | `TurnPolicy.swift` (520 lines) |
| Final transcript of the turn | Fluid Speech Mini, warm, one pass over the VAD-bounded audio | `FluidSpeechMini.swift` |
| Another voice does not barge in or get sent | WeSpeaker and CAM++ embeddings, a learned voice after three agreeing turns, no enrolment | `SpeakerEmbedder.swift`, `SpeakerProfile.swift`, `SpeakerFusion.swift` |
| Speak | Phonon 31m (Core ML via `xn-ptts`), clause by clause, one clause prepared ahead, played into the engine's own player node; played-sample clock records what was heard | `PhononVoice.swift`, `ClauseQueue.swift`, `VoiceEngine.swift:1036-1093` |
| Models | Download from pinned Hugging Face commits, every file SHA-256 checked, prepare and prewarm, 5 GB minimum memory | `VoiceModels.swift`, `VoicePreparation.swift`, `VoiceEngine.swift` |
| The engine itself | actor `VoiceEngine.shared`, `start(config:voice:emit:)` (line 582), `speak(turnId:clauseId:text:)` (1036), `endAnswer` (1076), `cancelAnswer` (1083) | `VoiceEngine.swift` (1,223 lines) |

Turn thresholds are one config object the web layer passes on start
(`VoiceContract.swift`: interruption needs 300 ms of speech and two words not
in the clause being played; playback holds at 700 ms and resumes after 1.5 s
without such words; a turn is cut at 60 s). Events out: `state`, `partial`,
`turn`, `bargeIn`, `clauseStarted`, `clausePlayed`, `interrupted(fraction)`,
`answerFinished`, `ended`, `ignored`, `metrics`.

The split is by time. Anything that must react inside an audio frame is
native. The conversation (sending the turn, making the answer speakable,
cutting clauses, remembering what was heard) is web code that talks to
Mike's chat backend.

### What is portable and what is not

Portable as Swift, by file: every file in `MikeSpeechPlugin/` except
`MikeSpeechPlugin.swift` (523 lines, the only one that imports Capacitor,
checked with grep). That is `VoiceEngine`, `VoiceAudioEngine`, `TurnPolicy`,
`ClauseQueue`, `EouStream`, `SmartTurn`, `SpeakerEmbedder`, `SpeakerProfile`,
`SpeakerFusion` (plus `SpeakerFusion/speaker-fusion.bin`), `VoiceModels`,
`VoiceModelCache`, `VoicePreparation`, `VoiceSupport`, `VoiceContract`,
`PhononVoice`, `FluidSpeechMini`, `LiveDictation`, `OnDeviceSpeech`,
`OnDeviceVoice`, `ScreenAwake`. Licence: the owner's own code ("Mike speech
code is mine", pass1:256-258); the third-party models keep their licences.

What does not carry over, each with the reason:

1. **The audio session.** `VoiceAudioEngine.start` calls
   `setCategory(.playAndRecord, mode: .voiceChat, ...)` and `setActive`
   itself and restores the old category on stop (`VoiceAudioEngine.swift:96-107,
   398-407`). In the Exact app only Exact's owner may touch the session; the
   app's adapter claims `voiceChat` through `context.audioSession.claim`
   (`X/AutoradioModule.swift:21-50`), and Exact's behaviour on a switch
   (release, then claim) is documented in `docs/reference/exact/audio.md`.
   Pass 1 already ruled "those calls come out" (pass1:248).
2. **The bridge.** `MikeSpeechPlugin.swift` is Capacitor; the Exact module
   (`X/AutoradioModule.swift`, `handle(name)` handlers) replaces it.
3. **The conversation layer is TypeScript.** `useVoiceChat.ts`,
   `voiceEngine.ts`, `speakable.ts`, `clauses.ts`, `spokenPrefix.ts` are not
   portable as code. What the app needs from them is small: send the
   committed turn text to the desk, take the streamed answer sentence by
   sentence into `speak`, call `endAnswer` and `cancelAnswer`, and keep the
   spoken prefix. The app already has this half in Swift:
   `AutoradioDeskStage` streams `/api/autoradio/turns` and speaks per
   sentence (`K/Choir/AutoradioDesk.swift`, `LocalSpeaker.swift`).
4. **Programme playback.** Mike plays an answer, not a programme. Its one
   player node is the AEC reference for what Mike says. A programme of
   minutes needs the same node (or a second one in the same engine) to be
   the echo reference, plus Now Playing, seeking and one timeline: pass 1
   names this as lane P (pass1:252-253). The app's `LiveAudio` already has
   programme and voice nodes in one engine feeding a reference tap
   (`LiveAudio.swift:35-46`); which engine survives is the central design
   choice of slice 2.
5. **Dependencies.** Mike's plugin pins FluidAudio 0.17.7 and `xn-ptts`
   0.4.0 through SwiftPM (`Package.swift`). The app pins FluidAudio 0.18.3
   (`mobile/AutoradioKit/Package.swift`). An Exact package cannot use SwiftPM
   libraries; third-party Swift reaches the app as a static xcframework in
   `modules/apple/` (`mobile/README.md`, "Third-party dependencies in the
   Exact app"). FluidAudio already arrives this way. `xn-ptts` is not built
   as an xcframework in this repo; whether it can be (it ships a C library
   and, per Mike's README, needs a separate Catalyst slice) is **not
   verified**. `EouStream.swift` drives FluidAudio's public pieces (mel
   spectrogram, loopback encoder, `RnntDecoder`, `Tokenizer`) as of 0.17.7;
   that they are unchanged in 0.18.3 is **not verified**.
6. **The model catalogue.** Mike bundles Fluid Speech Mini in the app
   (`Package.swift` copies `FluidSpeechMini/`); the owner's ruling is that
   models are not in the binary and download at onboarding
   (`autoradio-mission-handoff-2026-11-09.md:233`). The rest of Mike's models
   already download (VAD, EOU, Smart Turn, speaker models, Phonon,
   `VoiceModels.swift`). About 700 MB of models are resident at once
   (`docs/voice-chat.md`, "Models in memory").
7. **Background.** Mike's voice mode ends when the app leaves the foreground
   (pass1:250-251). The review's goal of hands-free listening with the screen
   locked is new work on top; Exact grants the background claim
   (`app.ts`, `device.microphone-background`).
8. **Reference pages.** `docs/reference/ON-DEVICE-MODELS.md` lists Phonon,
   Smart Turn, Silero, the EOU model and the speaker models, each "untested
   by us". Per CLAUDE.md, a dependency newer than my training data needs a
   reference page before it is touched; `xn-ptts`/Phonon and Smart Turn have
   no page of their own in `docs/reference/`. Slice 2 starts by writing them
   from Mike's source and the vendors' pages.

Mike's server dependence: none for the loop itself. Speech, VAD, turn end,
barge-in, speaker check and TTS all run on the phone; models come from
Hugging Face. Mike's backend is needed only for the chat turn the web layer
sends. The radio's equivalent is `POST /api/autoradio/turns`, which already
exists.

## Voice implementations found in this repo

1. **AutoradioKit stage pipeline**, the main one: `K/TurnEngine.swift`
   (713 lines), `K/LiveAudio.swift` (676), `K/BasicStages.swift` (energy
   VAD, button, open and hybrid addressing), `K/AppleStages.swift`
   (`SFSpeechRecognizer` on-device, iOS 26 `SpeechAnalyzer`,
   `AVSpeechSynthesizer.write` voice), `K/LocalAck.swift` and
   `K/LocalSpeaker.swift` (a local Qwen3 model "thinking aloud" as holder).
   Default configuration: button addressing, energy VAD, `apple-speech`,
   `avspeech` (`ChoirApp.swift:12-22`).
2. **AutoradioModels**: a second set of the same jobs on FluidAudio and
   llama.cpp, selectable but not default (`mobile/AutoradioKit/Sources/AutoradioModels/`):
   `silero-vad`, `parakeet-eou`, `parakeet-tdt`, `kokoro`, `pocket-tts`,
   `lfm3-audio-live`, `lfm3-audio-tts`, and `LlamaSystem1`.
3. **Voice setup and the configurator**: `K/VoiceSetup.swift` (678),
   `K/Configurator.swift`, `K/ModelCandidates.swift`, `K/KnownModels.swift`,
   `K/LiveConfigBackend.swift`, `K/ModelStore.swift`: a per-phone model
   picker that measures and chooses stages, run in onboarding.
4. **The interview's voice**: `K/Interview.swift` (937) with its own stages
   (`InterviewProgramme`, `InterviewDesk`) and `X/AutoradioModule+Onboarding.swift`
   (439), which also owns a private `AVSpeechSynthesizer` ("Can you hear
   this?", line 30,203) and its own engine start for the mic check.
5. **Programme player and read-aloud**: `ProgrammePlayer.speakText`
   (`LiveAudio.swift:551`), `MailReading` (`K/ChoirMail.swift:726`).
6. **Spikes and tools**: `mobile/exact-spike/package/apple/Spike.swift` (a
   standalone engine and player-node audio-session spike), the
   `autoradio-replay` Mac command and `autoradio-models` command
   (`K/../autoradio-replay/main.swift`), and the fixture turns.
7. **Unmerged branches**: `lane/ar3-voice` (6 commits ahead of main, last
   2026-11-11: "responder ladder", "no timers around model calls",
   Qwen3-1.7B), `backup/ar3-voice-prerebase`, `docs/on-device-voice-research`.
   Not read in detail.

Designs that describe voice work not in code: `docs/design/autoradio-voice-modes-2026-11-10.md`
(three modes and a hybrid, two audio states, wake word) and
`docs/design/autoradio-mission-handoff-2026-11-09.md` lane V ("port Mike's
recognition, turn-taking, echo cancellation and AI voice as a Swift module").
Lane V as written was not built; a grep of `mobile/` for Mike's types
(`TurnPolicy`, `ClauseQueue`, `SmartTurn`, `EouStream`, `VoiceAudioEngine`,
`MikeSpeech`) finds no code, only README mentions of the "Mike-plugin shape".

## C. Proposed order of work

The owner's order stands, with one addition in front and two corrections
from the code. Each slice is one mechanism.

### Slice 0. Read the phone, fix the one-line defect (no device build to read)

- Observations first, by the owner, on the build already installed: open
  Lab, Trace after pressing Play on one story (A4), and read Settings for the
  talk mode (A8). This decides the silence and the interview-hears-nothing
  candidates with no code.
- Code: the duplicate question. Make the interview's first beat happen once
  (either `startInterview` does not call `s.start()` because
  `InterviewProgramme.load` does, or load does not resume a running
  interview). Files: `X/AutoradioModule+Onboarding.swift`, `K/Interview.swift`,
  `K/../Tests/AutoradioKitTests/InterviewTests.swift`. Class orange
  (runtime behaviour), rollback revert. Deletes nothing. Proof: a test that
  drives `startInterview` then `InterviewProgramme.load` and counts radio
  turns, then the phone. Server: none.
- Correction to the proposed order: this slice is first because the visible
  failures of items 4 and 8 are partly known, and it costs a read, not a
  rebuild of voice. Slice 2 will make the talk-mode part of item 8 moot
  (Mike's loop is open by design), so do not repair interview addressing
  separately.

### Slice 1. Text to speech for one story and any long text

One mechanism: a text becomes audio through `speakText`.

- Files: `K/AppleStages.swift` (end an utterance at the synthesizer's
  `didFinish`, not at the first empty buffer; the fix Mike's README
  documents); `K/LiveAudio.swift` (log a failed `registry.make(.voice)` as an
  error event, `playBlock` ~585; show a plain line on the Play button when
  `LabRuntime.start` failed); a "Listen" control on the reader
  (`L/reader.contract`, `L/members-reader.contract`), on Textures, and on
  the file viewers (`L/viewers.contract`) calling `speakText` through a new
  `readerSpeak`-style send in `X/AutoradioModule+Reader.swift`; lift the
  200-paragraph cap or chunk it.
- Deletes: the private synthesizer and engine start in
  `X/AutoradioModule+Onboarding.swift` (lines 30, 186-195, 203), replaced by
  `speakText`.
- Correction to the proposed order: this slice is the system voice only. It
  proves the whole route (entry point, player, Now Playing) and gives the
  owner one story he can hear, but its voice stage is thrown away by slice 2,
  so keep the voice choice behind `speakText` and do not build on
  `AVSpeechVoice` further. If the Trace in slice 0 shows the silence is the
  session claim and not the voice, fix that here.
- Device proof: Play on one story; the Trace shows `programmeAudioStart` for
  each paragraph and the audio runs to the end of a 600-character paragraph
  (the earlier cut); "Listen" on a reader page, a Texture and an EPUB chapter.
- Server: none (`/api/platform/publications/resolve` exists).

### Slice 2. Mike's loop adopted whole; the other voice versions deleted

One mechanism: Mike's `VoiceEngine` is the only microphone, turn-taking and
speech engine in the app.

- Files added: Mike's `MikeSpeechPlugin/` Swift files except the Capacitor
  bridge, as a native module under `mobile/AutoradioLab/modules/apple/` (a
  third-party-style module, as `README` "Third-party dependencies" describes),
  with: its `AVAudioSession` calls replaced by `ExactSessionOwner` claims
  (`AutoradioModule.swift:21-50`); a new Exact handler set replacing
  `voiceStart`, `voiceSpeak`, `voiceEndAnswer`, `voiceCancelAnswer`,
  `voiceStatus`, `voiceInstall`; the engine's events (`turn`, `partial`,
  `bargeIn`, `interrupted`) routed to the desk stage and the bar; a second
  player node (programme) in Mike's engine so the programme is the echo
  reference; `FluidAudio` and `xn-ptts` as static xcframeworks; reference
  pages `docs/reference/` for Phonon/`xn-ptts` and Smart Turn first.
- Files deleted, to be confirmed by the owner before the change: from `K/`,
  the transcriber and voice stages in `AppleStages.swift`, the energy VAD and
  the open and hybrid addressing in `BasicStages.swift`, `VoiceSetup.swift`,
  `Configurator.swift`, `ModelCandidates.swift`, `KnownModels.swift`,
  `LiveConfigBackend.swift`, the speaker-check stub; from `AutoradioModels/`,
  `SileroVAD`, `ParakeetStages`, `VoiceStages`, `Lfm3Audio`; the interview's
  mic and hearing steps and its stages, replaced by the loop; the
  `autoradio-models` command; the `ar3-voice` and backup branches after the
  owner agrees. About 30 test files cover the kit; a list of which go
  with their code is part of this slice's failure list.
  `LocalAck` (a Qwen3 holder that "thinks aloud") is a different job from the
  loop and is not in this list; the owner decides.
- Also in this slice: dictation anywhere there is a text block (item 9), from
  Mike's `LiveDictation.swift` and `FluidSpeechMini.swift`, because it uses
  the same models and engine.
- Device proof: the loop answers a spoken turn with no tap; barge-in cuts the
  radio; speaking over the interview's question produces an owner turn with
  the `interrupted` flag (`Interview.swift` already takes `interrupted` and
  `heard`); a recorded trace replay of `TurnPolicy` matches Mike's tests.
  Mike's own thresholds are untuned on speaker output over a radio (pass1
  notes it); the first device run measures them.
- Server: none for the loop. The phone must still send candidates to the
  programme route so a programme has stories in it (overnight report); that
  is a separate app change, not a server one.
- Risk to state: both new dependencies must build as static xcframeworks
  and `EouStream` must hold on FluidAudio 0.18.3; if either fails the slice
  stops at that point and says which.

### Slice 3. Navigation

One mechanism: the deck is the paper's articles.

- Files: `L/members-stack.contract` (`order` seeded from `choirStories`; key
  pages leave the cycle), `L/pages.contract` (real next cards instead of
  placards, the animation of item 3), `X/AutoradioModule+Cards.swift`
  (re-use `StackSession` or delete it), the horizontal scrollers listed in
  A2 (wrap or use a vertical menu; give the Mail row a width), and an
  in-app source view: `SFSafariViewController` presented from the module for
  "Open original" (`members-reader.contract:68`, new send in
  `AutoradioModule+Reader.swift`), or a native `WKWebView` page kind. The
  first is the smaller change; the second keeps the web page in the deck.
- Deletes: the five-page `order` default, the unused card handlers
  (`cardStories`, `cardFling` and the rest of the old pile) if the stack
  does not reuse them.
- Device proof: swipe through ten stories without returning to the front
  page; no page scrolls sideways on Mail, Paper, Files, the launcher or a
  reader with a table; "Open original" stays in the app and a swipe back
  returns to the story. The deck-swipe problem on Textures is read first
  (`autoradio-deck-swipe-stalls-on-textures-2026-11-11.md`), since it hits
  every swipe design.
- Server: none.
- Order note: items 1 to 3 and 11 do not depend on slice 2, so this slice can
  run in parallel with it in its own worktree if there are two builders.

### Slice 4. Search, the prompt bar and the edition: options only

Needs the owner. The facts: the bar is a conversation turn, the search is a
local filter over about 30 stories and the pages already opened, the server
has a public substring search, a BM25 search that no phone can reach, and no
filter on the front list.

- Option 1: two controls with plain labels. The bar says "Ask or tell the
  radio" and the launcher's field says "Search the paper". Search calls the
  existing public route `GET /api/platform/retrieval/search` (slow, 20
  results, substring, whole archive up to 2,000 publications). No server
  change. Edition stays local shading.
- Option 2: as Option 1 but search calls the BM25 index. Needs a server
  change: a route reachable with a device key (`internal/proxy/handlers.go`,
  new public path to `SearchPublishedPieces`; auth surface, red under the
  contract). Gives ranked results for "Nvidia" or "Israel".
- Option 3: one mechanism for filter and edition. A query is a temporary
  edition; keeping it saves it, and the front list and the programme route
  take it as a parameter (a new `q` or edition id on
  `/api/platform/publications/front` and `/api/autoradio/programme`). This is
  the item 7 design. It changes what the edition is (the interview's
  document, the phone's local shading and a saved query would have to be one
  object), so it needs the owner's design, not a guess here.
- Option 4: the bar does both: a typed line that the radio decides is either
  a question, a command or a filter. Smallest change on screen, largest
  change in the turn route.

## What was not done or not verified

- Nothing ran. All causes are from reading; the device observations that
  decide them are named at each item.
- Mike's remote was not fetched. `feat/web-voice-mode` and
  `feat/phone-web-voice-modal` were not read. Mike's tests were not run.
- `lane/ar3-voice` and `backup/ar3-voice-prerebase` were counted, not read.
- `docs/reference/AUTORADIO-TURNS.md` and `docs/reference/exact/rendering-and-motion.md`
  were not read for item 5's instruction behaviour and item 3's page turn.
- Whether the owner changed the talk setting, and whether the Mail row
  alone causes the sideways scroll, are open until read on the phone.
