# Autoradio: where each Swift test goes (2026-11-18)

Station XG of [the cutover](../definitions/choir-autoradio-exact-cutover-2026-11-18.md);
the design is [autoradio-in-exact-2026-11-18.md](autoradio-in-exact-2026-11-18.md).
Every Swift test file under `mobile/` has one line: it **stays** with the
voice, it **ports** with a station (its product rules are rewritten as Bun
tests or contract tests in `mobile/AutoradioLab` before the Swift they test is
deleted, on the same fixtures where the test reads one), or it **dies** with
its code (the rule is Swift's own, nothing in shared code can break it).

There is one Swift test target, `mobile/AutoradioKit/Tests/AutoradioKitTests`
(95 files), plus nothing on the Kotlin side. No TypeScript test exists in
`mobile/` yet. Read each file's subject from its header and its test names; the
dispositions were made from those, not from file names.

The rule for a station: the Swift tests this table gives it are ported in the
same change that deletes the Swift. A rule a Swift test carries that no ported
test carries is a defect of that change. A file marked `mixed` lists what each
part does; a station takes only its part, and the file is deleted when its
last part has moved. Where a row says `X4` the part is a rule of the deck.

Stations: X2 paper page; X3 sign-in; X4 deck; X5 run sheet and the radio's
decisions; X7 podcasts and clips; X8 files, reader, books; X9 mail; X10
content, search, takes, notebook; X11 settings, onboarding, themes, cards, the
bar; X12 the end (ChoirClient goes).

| Test file | Subject | Goes |
| --- | --- | --- |
| AccountWipeTests | sign-out leaves no marker in any store; the first request after the next sign-in carries none of it | ports with X3, and each store's station adds its folder to the wipe test |
| AckTimingTests | the immediate acknowledgement's timing against System 1 and the desk, with stub stages | stays with the voice |
| AimAndLensTests | a talk press names the aimed page; the lens is theme data | ports with X11 |
| AnswerKeepsPlaceTests | an answer over a programme entry puts a continuation back when answers end | ports with X5 |
| AudioHeldTests | engine restarts after a take; the session is not re-asserted on every try | stays with the voice |
| AudioSessionHoldTests | a session failure is not the entry's failure; a reclaim keeps its claim; the radio's own session change is not a pause | stays with the voice; the first rule (no verdict from a failure to make sound) is written again against the deck's journal in X4 |
| AutoradioDeskTests | the turn route client and its stream, against a fake server | ports with X5 |
| BarStateTests | the prompt bar's states | ports with X11 |
| CardsTests | what swipes teach the paper (preferences), the story stack, the launcher's search and recents, signals | ports with X11 |
| ChoirClientTests | the server client: System 1 judgments, computer discovery, auth headers | mixed: discovery and auth port with X3; System 1 with X5; the rest dies with ChoirClient at X12 |
| ClipLocatorTests | the clip fingerprint's conformance fixture, mp3 frame sync, finding a clip in a differently served file | stays with the voice (fingerprinting stays native, X7) |
| ConfiguratorTests | which on-device model is chosen per role, with faked measurements | stays with the voice |
| ContentFullScreenTests | reads the contract text: content gets the whole screen but the bar | ports with X8 as a contract test |
| ContentStoreTests | the content store and its sync, kill and relaunch, logs never show titles | ports with X10 |
| CorpusSearchTests | the corpus search ask and its answer shape | ports with X10 |
| DesignRulesTests | reads the contracts: no pills, no outlined bubbles | ports with X11 as a contract test |
| DJClipsTests | at most three clips per story sent to the DJ; one bulk clips request per stretch | ports with X7 |
| EditionSetupTests | first-run edition setup: always ends, the edition's address | ports with X11 |
| EmbedderTests | the embedding model's interface and vector store, with a fake model | stays with the voice (embeddings stay native with the models) |
| FileKindsOpenTests | opening each kind of file from the Files place; a PDF takes its first page with text | ports with X8; the PDF cases die with PDFKit and are rewritten against the listener's computer reading the text (decision D4) |
| FilesInReaderTests | a PDF or e-book opened from Files is a Reader document | ports with X8 |
| FilesInTextureTests | text, Markdown and code from Files open in Texture by the web's call | ports with X8 |
| FilesOpenTests | a Files address that resolves to another page leaves no `file:` page in the pile | ports with X8 |
| FilesTests | file paths, the files client, EPUB reading, the demo files | ports with X8 |
| GoldenDocumentsTests | the golden Texture documents through the reader's model, the one test of the web's own fixtures | ports with X8 (the fixture stays under internal/texturedoc/golden) |
| HandsFreeTests | hands-free listening: loud frames are speech, the turn policy opens and closes turns | stays with the voice |
| HandsFreeToggleTests | hands-free as one on/off state; when the listening state shows | stays with the voice; the pure "when the state shows" rule (T7) ports with X11 |
| HeldRestTests | the words still to read of a cut passage survive a forgotten process | ports with X5 |
| HybridRetrievalTests | BM25 plus vectors, rank fusion | ports with X10 (the vectors come from the native model) |
| JoinsImportTests | a typed .pdf or .epub in the bar imports | ports with X8 |
| JoinsTests | a clip opens its episode; a story page's clips; a show lists its clips; a book is passages for retrieval | mixed: J1, J3, J6 port with X7; J5 (books for retrieval) with X10 |
| LayoutTests | the layout engine's rules on one layout document | ports with X2 |
| LearnedFromPreferencesTests | what the paper learned from preferences, reset, order | ports with X10 |
| LearningLoopTests | the phone's side of the learning loop: segment events, claims, settlement client | ports with X5 |
| ListenTextTests | what Listen speaks for a document, and what never reaches speech | ports with X8 |
| LocalAckTests | the on-phone model's streamed acknowledgement, the local speaker, the turn timeline | stays with the voice |
| MailboxTabsTests | mailbox tab frames and counts | ports with X9 |
| MailTests | mail: threading, readable text, the client | ports with X9 |
| MarkupSyntaxHiding | (not a test) the pure rules of Exact patch 0001, linked into the test target | stays: it belongs to the Exact patch in `mobile/exact-patches`, outside this gate's roots |
| MarkupSyntaxHidingTests | the patch's rules for hiding Markdown syntax away from the caret | stays with the patch |
| MicAskTests | the microphone is asked for only with voice on | stays with the voice |
| MicCheckTests | the first-run microphone check's pass rule and timeout | stays with the voice |
| MicLevelTests | the bar's mic level: silence reads 0, one loud frame does not stick | ports with X11 (native hands over the raw level) |
| NavigationTests | the paper's stack order and neighbours; no contract scrolls sideways; mail tabs wrap | mixed: stack rules port with X2; the contract rules with X11 |
| NoWaitingWordsTests | runs `mobile/scripts/check-no-loading.py` so a "Loading" string fails the suite | dies with its Swift wrapper; the script stays and a Bun test runs it (X11); until then `proof-app.sh` still runs it |
| OnAirSoundTests | an entry on air hands samples to the output node; a silent voice is a failure | stays with the voice |
| OpeningCacheTests | the written-ahead opening: age, part of the day, its audio | ports with X5 |
| PaperViewsTests | the paper views from a real front answer | ports with X2 |
| PhoneRepairFixAccountTests | an unanswered notebook ask is not "no notebook"; hostile characters in a stranger's take title | ports with X10 |
| PlaybackContinuityTests | background audio, interruption, route loss, Play after the background | stays with the voice; the decisions it names (what resumes, what does not) are written against the deck in X4 and X6 |
| PlayerControlsTests | speed to 3x, next and previous story, autoplay, the DJ's candidates | ports with X5 |
| PlayerDJTests | the programme request carries the paper's stories; a stretch of breaks is not a programme | ports with X5 |
| PlayerRepairFixTests | a replayed clip plays again; Previous in a lead-in; a hold is not a system pause; a held player is not resumed by a rate change | mixed: the hold and pause rules port with X4 (journal), the rest with X5 |
| PlayheadTests | position from rendered audio at any rate; text queues behind the story; the desk turn's cut text | mixed: rendered position and rate stay with the voice; the queueing and cut-text rules port with X5 |
| PodcastTests | feed parsing under hostile input, subscriptions, positions | ports with X7 |
| ProgrammePlayerRepair2Tests | resumeAt outlives its document; sign-out clears `loading`; format-character stripping | ports with X5 |
| ProgrammePlayerRepairTests | a multi-block segment whose first block got no voice fails once and nothing after it | ports with X5 |
| ProgrammePlayerTests | the programme player and its stretch decoder, on `mobile/fixtures/learning` | ports with X5 (the fixtures are read by Swift and TypeScript, which the Bun-in-CI step will enforce) |
| ProofKeyTests | the simulator's proof-key sign-in | ports with X3 (the proof's way of signing in is rewritten with the key's new home) |
| ProvenanceTests | the stories an entry was built on survive every hop to the heard record | ports with X5 |
| QuickRelaunchTests | a relaunch five minutes after an opening plays without a second greeting | ports with X5 |
| RadioPageTests | the Autoradio page's view model and the ops its gestures send | ports with X5 |
| ReaderPrefetchTests | the stack's neighbours are fetched before the swipe | ports with X8 |
| ReaderSampleTests | the demo sample book | ports with X8 |
| ReaderShelfTests | the shelf, EPUB zip reading, PDFs, books kept out of backups | ports with X8; the PDF cases follow FileKindsOpenTests |
| RealShapeTests | trimmed copies of what choir.news really answers: related, front, textures, who-am-I, editions | mixed: front with X2, textures and editions with X8, who-am-I with X3, related with X10 |
| RepairTests | the 2026-11-11 app-review repairs | mixed: Once, TaskBag, withDeadline and Locked die with `Concurrency.swift`; the same-origin key tables port with X3; MailSanitizer cases with X9; UTF-8 trimming and read-text cuts with X8; bad sample rates stay with the voice |
| RetrievalTests | the retrieval index over stories and books, its size bounds | ports with X10 |
| RoundThreeRepairTests | cancelling a held turn, the desk hand-over waiting for the local voice, a desk failure said in words, model downloads resume and corrupt files drop | mixed: turns and hand-over stay with the voice; the failure words port with X5; downloads and the rejected-model deletion stay with the voice |
| RunSheetStoreTests | the phone on one run sheet: opening, placement, restore, outcomes (60 tests) | ports with X5 |
| RunSheetTests | the Swift reducer against `mobile/fixtures/runsheet/*.json`, the fixtures the Go reducer reads | ports with X5; the Swift copy dies and CI holds Go and TypeScript to the same fixtures |
| SettingsDataTests | theme loading and the contrast gate, accounts, settings rows, the learned list, the activity feed | mixed: themes, settings, learned and activity port with X11; the account view with X3 |
| SourceLinkTests | which links open inside the app, which leave it, which are refused | ports with X11 |
| SpeakerGateTests | the reader's voice: enrolment, decisions, storage, with a fake embedder | stays with the voice |
| SpeechFixesTests | the system voice's empty-buffer paragraph end and the other phone-review speech repairs | stays with the voice |
| SpineTests | the open-pages pile, persistence, theme loading, the demo serving fixtures and refusing the rest | ports with X11 |
| StartGapTests | the start gap: one extend and no open ask, a dropped entry does not block the next extend | ports with X5 |
| TakeSessionTests | the take recorder's state machine behind four protocols, and the on-air Take control | stays with the voice (the take recorder is native); the panel's states, not the recorder, port with X10 |
| TakesTests | the takes wire and client: publish, list, the stub server | ports with X10 |
| TalkTests | retrieval index, the conversation card and its answers, the holding line, every turn going to the desk, queue ops | mixed: the retrieval index with X10; the card, answers and the holding line with X11; turns to the desk and queue ops with X5 |
| TetraMarkMotionTests | the mark's motion arithmetic, and that the Swift data matches the contract | ports with X11 (pure arithmetic, no UI) |
| TextureBodyTests | the document stays Texture-format compatible through the block pipeline | ports with X10 |
| TextureDocTests | the reader's document model: paragraphs, headings, unknown nodes | ports with X8 |
| TextureEditorTests | the editor's save rules, relaunch, posted once | ports with X10 |
| ThemeDefaultsTests | the default themes by scheme; a chosen theme always wins | ports with X11 |
| TrayGridTests | the tray's glyph rows | ports with X11 |
| TurnContextTests | what the phone sends the desk with a turn: the page in front, retrieval | ports with X5 |
| TurnPolicyTests | replays event traces through the turn policy (54 tests) | stays with the voice |
| TypedTurnTests | a typed question runs the turn pipeline and leaves the engine idle | stays with the voice; the words reaching the desk port with X5 |
| TypographyRulesTests | reads the contracts: type roles name a role, never a face | ports with X11 as a contract test |
| VoiceChoiceTests | the voice catalog, the saved choice, the speaker that falls back, the voice cards | stays with the voice |
| VoiceCoexistTests | voice on must not stop the radio; the radio does not fight other apps for the microphone | stays with the voice |
| VoiceModelsTests | the model catalog and the voice setup backend, after the first phone walk | stays with the voice |
| VoiceSetupTests | voice setup's fallback ladder per stage | stays with the voice |
| WebParityTests | the six places the native reader departed from the web reader on the golden documents | ports with X8 |

Counts, from this table: 95 files; the ones marked **stays** (including the
two patch rows) are the voice's and the patch's; every other row names the
station that ports it, and a **mixed** row names each part's station.

## Product rules a Swift test carries that have no other home

These are written here so a deletion cannot lose them:

- A failure to make sound is not an entry's failure and gives no verdict
  (AudioSessionHoldTests): the deck's journal states it (X4).
- The wipe at sign-out covers every store the phone holds (AccountWipeTests):
  each station that moves a store adds it to the wipe test.
- The fixtures `mobile/fixtures/runsheet`, `learning`, `takes`, `podcast-clip-fingerprint-v1.json`,
  `texture` and `radio-page` are read by Swift tests today; each moves to a
  Bun test in its station, and the Swift reader goes with its Swift.
