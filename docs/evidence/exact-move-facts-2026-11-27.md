# Facts for the mission "finish the move of Autoradio to Exact, on the latest Exact, with a signed-out turn that completes on the phone"

Standing: governing | kind: happened | settled-by: a read-only count on 2026-11-27 against origin/main 8e8c86f3d, for docs/definitions/choir-finish-the-move-to-exact-2026-11-27.md | note: nothing was built or run; the voice marking is the repository's own list, which the reader thought generous

Read-only investigation, 2026-11-27. Source: origin/main at 8e8c86f3d (after `git fetch`), a copy of `mobile/` extracted to `scratchpad/exact-facts/tree/` for counting, lane refs read with git, the untrusted upstream clone read with `git -C` only. "Inference" marks my reading. Nothing was built or run.

## 1. Where the app stands against the rule

Documents read: the problem record (`docs/problems/autoradio-app-logic-is-swift-not-exact-2026-11-18.md`, updates through 2026-11-24), `docs/design/autoradio-in-exact-2026-11-18.md`, the three definitions, `docs/definitions/choir-shipping-the-app-2026-11-23.md`, `choir-autoradio-on-android-2026-11-22.md`, `docs/current-architecture.md` section 6, `docs/system-map.md` section 7, `docs/evidence/exact-conversion-audit-2026-11-24.md`, and `docs/reference/exact/*` (14 pages: answers-let-go, audio, auth-session, build-and-ship, capabilities, deck, gaps, internals, markdown-editor-and-textflow, native-modules, passkeys, rendering-and-motion, testing-and-debugging, ui-model) plus `docs/reference/EXACT.md` (the main Exact page).

What the documents say, in short:
- Measured 2026-11-18: 50 contract files / 7,742 lines of pages over 157 Swift files / 38,149 lines of logic. Today (my count, `mobile/AutoradioLab`): 92 `.contract` files / 9,891 lines, 189 TypeScript files / 39,757 lines (107 of them or their contract twins are tests).
- The repo's own gate (`scripts/check-autoradio-native-boundary`, ceiling `scripts/autoradio-native-ceiling.txt`) counts non-voice native code: **34 files, 3,375 lines, 17 interface names** on main now (was 119 files / 28,093 lines / 145 names on 2026-11-19). I reproduced 34 files / 3,375 lines (35 / 3,894 with the one C++ file, which the gate does not count).
- The system map (section 7, updated 2026-11-25) says the app "ships one Exact app from `mobile/AutoradioLab`"; the build `473021` from `16f04df4d` is what Apple has.
- The cutover definition (`choir-exact-cutover-and-ontology-2026-11-20.md`) carries every station X0..X8 as `status: working` (stale: the card was last updated `as_of: 2026-11-24T02:21Z`). Its first station X0 "Exact moved to upstream" has not landed (pin is still f53347503, item 3).
- The audit of 2026-11-24 (read-only, estimates, not measurements) says that of 19,280 Swift lines then, about 55 percent could still move to Exact, naming the narrow native boundary that stays (mic open, finalise, speak, local model generate/embed, speaker embed, model fetch/load, deck, take recorder). Much of that 55 percent is files the repo's voice list classifies as voice (TurnPolicy, Configurator, ModelCandidates, KnownModels, VoiceSetup, VoiceChoice, ListenerPipeline). Whether those count as "voice" under the owner's rule is a judgment call the documents do not close. See item 2.
- The problem record's last update (2026-11-24) names what stays Swift on purpose: the Texture editor, notebook, mail seal. Inference: that note is stale for the editor, because main now has no `TextureEditor.swift` (only `texture-edit.ts`, `notebook-topics.ts` mention it) and `docs/changes.md:393` records the editor landing (lane `exact-editor`). `MailSeal.swift` (72 lines) and `Tests/.../MailSealTests.swift` are still on main; `mobile/AutoradioLab/mail-seal.ts` is also there. Owner ruled 2026-11-24 "get rid of mail seal; mail on the phone's disk" (`docs/definitions/choir-shipping-the-app-2026-11-23.md` owner_rulings).

### Table: each page or capability, where its logic lives today

Lines are my count on origin/main. "impl" = non-test files matching the page's name prefixes in `mobile/AutoradioLab`; tests are `*.test.ts` / `*.test.contract` of the same prefixes. Prefix grouping is approximate.

| Page or capability | Where the logic lives today | Evidence | What the documents say remains |
| --- | --- | --- | --- |
| Paper (front page, stories) | Exact: `paper.ts`, `paper-model.ts`, `paper-layout.ts`, `paper.contract` | 1,123 impl / 378 test | Nothing named. Signed-out Android baseline proven 2026-11-26. |
| Radio page, run sheet, queue, what plays next | Exact: `radio-runtime.ts`, `radio-director.ts`, `radio-page.ts`, `radio-writer.ts`, `runsheet-store.ts`, `radio-bridge.ts` and others | 4,742 impl / 4,178 test (21 files each) | "No Swift copy of the run sheet" (problem record 2026-11-23/24). Swift left: `RadioBridge.swift` (83, mailbox), `RunSheetTypes.swift` (25), `RadioProgramme.swift` (288, programme types, counted non-voice), `BarState.swift` (235), `SegmentTracker.swift` (168, no production reader, kept for ~30 tests), `QueueVoice.swift` (19). |
| The deck (audio of what plays) | Native by design: `Deck.swift` (601), `AutoradioModule+Deck.swift`, Kotlin `Deck*.kt`, `DeckMediaEngine.kt`; Exact side `deck.ts`, `deck-ops.ts`, `deck-native.ts` | voice list (audio) | Owner rule: audio is native. Locked-phone behaviour not measured (item 7). |
| The bar, conversation, answer card | Exact: `bar.ts`, `conversation.ts`, `cards.ts`, `pages.ts` | 1,367 impl / 818 test | Swift left: `Conversation.swift` (124, the turn's record), `SavedAnswerHolder.swift` (148, the phone's first answer from what it holds), `AutoradioModule+Bar.swift` (130), `+Talk.swift` (44). |
| Mail | Exact: `mail.ts`, `mail-model.ts`, `mail-store.ts`, `mail-sanitize.ts`, `mail-seal.ts` | 2,018 impl / 828 test | `MailSeal.swift` (72, CryptoKit AES-GCM, key in Keychain; Android owes a Kotlin copy) and `AutoradioModule+Mail.swift` (24, in-app web view). Owner ruled the seal is to go; lane `exact-rest` holds the deletion (unlanded, item 5). |
| Files | Exact: `files.ts`, `files-model.ts` | 1,163 impl / 359 test | Nothing named. |
| The reader (stories, Texture documents) | Exact: `reader.ts`, `reader-parse.ts`, `story-text.ts` | 1,637 impl / 745 test | Swift left: `HeldPages.swift` (82), `ReaderFollow.swift` (83) (the voice's side of the reader), `AutoradioModule+Reader.swift` (117). Documents the phone cannot write exactly open read-only (lane `exr-r`, unlanded). |
| Imported PDFs and e-books | Exact: `imported.ts`, `imported-model.ts`, `imported-search.ts` (file goes to the person's computer) | in the search row below | Native is audio only (`importedHand`, `importedFollow`). Swift `ReaderShelf`, `Epub` deleted. |
| Podcasts and clips | Exact: `podcast.ts`, `podcast-feed.ts`, `podcast-now.ts`, `clip-play.ts`, `clips.ts` | 1,838 impl / 962 test | Fingerprinting and locating a clip stays native (`ClipFingerprint.swift`, `ClipLocator.swift` 349, on the voice list). |
| Takes (record, publish) | Exact: `takes.ts`, `take-session.ts` | 756 impl / 843 test | The recorder is native (`TakeSession.swift` 240, `AutoradioModule+Takes.swift` 87). `Choir/Takes.swift` (22) left. |
| Notebook, learning route | Exact: `notebook.ts`, `notebook-run.ts`, `notebook-topics.ts`, `learn*.contract` | 680 impl / 418 test | Landed 2026-11-21; editor landed 2026-11-24 (inference from main's tree). `AutoradioModule+Notebook.swift` (15). |
| Texture editor and Textures list | Exact: `texture-edit.ts`, `texture-body.ts`, `textures.ts` | 1,302 impl / 937 test | Landed on main (`docs/changes.md:393`). The problem record still says "stay Swift"; stale. |
| Search and retrieval (launcher, voice's passages) | Exact: `search.ts`, `retrieval-live.ts`, `retrieval-run.ts`, `imported-search.ts` | 2,394 impl / 1,638 test | Quality loss named 2026-11-21: documents and book passages not in the voice turn's first answer. Native asks via `RetrievalFacts.swift` (voice list). |
| Sign-in, account state, consent | Exact: `app-signin.ts`, `account.ts`, `account-state.ts`, `api.ts`, `consent.ts`, `passkey-ceremony.ts`, `refused.ts`, `reviewer-signin` test | 2,004 impl / 2,256 test | Swift left: `ChoirApp.swift` (252, the engine's controller: sign-out order), `ChoirClient.swift` (439, key stores and the voice turn's HTTP client), `ChoirSignIn.swift` (32, sign-out revoke), `AccountWipe.swift` (103), `ProofKey.swift` (35). The native server client exists only for the voice turn's calls. |
| Sync, the content store | Exact: `content.ts`, `content-run.ts`, `heartbeat.ts` (`app:/data/content.db`) | 1,059 impl / 338 test | Named residual: the store is in the device backup; Swift protected it and excluded it from backup. |
| Settings (Computer, Activity, About, Themes, Voice and models, Downloads) | Exact: `settings.ts`, `computer.ts`, `activity.ts`, `about.ts`, `themes.ts`, `models.ts`, `offers.ts`, `native-models.ts` | 2,258 impl / 1,168 test | Model chooser and downloader stay native on the voice list (`Configurator`, `LiveConfigBackend`, `VoiceSetup`, `DownloadDoor`, `ModelStore`, `ModelPins`). The audit says the config/choice logic could move. Settings > Downloads (lane `voice-more`) is not on main. |
| Onboarding / first run | Exact: `onboarding.ts`, `edition-writer.ts`, `edition-picks.ts`, `micask.ts`, `voicelearn.ts`, `voice-facts.ts` | 1,998 impl / 1,328 test | Native: `AutoradioModule+Onboarding.swift` (435, voice side: models setup, mic check, hearing sentence), `EditionSetup.swift` (114). The "onboarding interview" belongs to 1.1 per big-picture. |
| Account deletion | Exact: `account-delete.ts` (144 lines, 230 test). Server side landed with the App Review train (`ed89268fa`). | `docs/ship/data-inventory.md` F14: "built and not deployed; the 5.1.1(v) requirement is unproved until the deployed proof on the drill account" | Deployed proof still owed (campaign: "account deletion deployed" is a 1.1 item). |
| Account wipe at sign-out | Both: `AccountWipe.swift` (103) native, shared wipe lists | per problem record | Account boundary design (12 review rounds, no convergence): `docs/problems/account-boundary-three-approaches-no-convergence-2026-11-23.md`; lane `account-boundary-l1` unlanded. |
| Voice (mic, speech, turn engine, models, download) | Native on purpose: ~20,000 lines Swift/Kotlin on the voice list | item 2 | Owner rule. Audit: part movable. |
| Android | Kotlin package: `exact-package/android` (16 files, 2,444 lines) with deck (Media3), model runtime (JNI), WebView seam; no Android sign-in proof yet | `docs/ACTIVE.md` station state 2026-11-27; `docs/reference/AUTORADIO-ANDROID-PARITY.md` | A1 passkey sheet needs owner's biometric; hands-free voice, A4 mail secrecy, A7 parity matrix owed. |

## 2. Measurements under `mobile/` (origin/main 8e8c86f3d)

Top-level directories (files / lines):

| Directory | Swift | Kotlin | TS (non-contract) | Contract | JSON | Shell / Py | Other |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `AutoradioKit` | 205 files / 33,189 lines (Sources 112 / 20,032; Tests 92 / 13,101; +Package.swift) | 0 | 0 | 0 | 0 | 0 | |
| `AutoradioLab` | 1 / 119 (`modules/apple/ModelsEntry.swift`) | 0 | 189 / 39,757 | 92 / 9,891 | 4 / 71 | 0 | 300 files total incl. assets |
| `exact-package` | 16 / 1,590 (`apple/`) | 16 / 2,444 (`android/`) + 1 C++ (519) | 0 | 0 | 20 / 2,297 (`exact.json`, `interface/*.json`) | 0 | |
| `exact-patches` | 1 / 51 (`sources/MarkupSyntaxHiding.swift`) | 0 | 0 | 0 | 0 | 0 | 3 patches + README |
| `exact-spike` | 1 / 96 | 0 | 1 / 6 | 1 / 32 | 6 / 86 | 0 | historical example, no CI uses it (`build-and-ship.md`) |
| `fixtures` | 0 | 0 | 0 | 0 | 114 / 19,735 | 0 | |
| `scripts` | 0 | 0 | 1 / 95 | 0 | 2 / 222 | 35 sh / 2,653; 17 py / 2,087 | `tiny-voca/`, `voice-pack/` subtrees |
| `themes`, `layouts`, `docs` | 0 | 0 | 0 | 0 | 7 / 255 | 0 | 1 md |

Native non-voice vs voice (gate's own classification, `scripts/autoradio-native-voice-list.txt`; Tests excluded; Swift + Kotlin, plus the C++ file in my count):

| Mark | Files | Lines |
| --- | --- | --- |
| NOT VOICE (gate counts, Swift+Kotlin) | 34 | 3,375 |
| NOT VOICE incl. `native_models.cpp` (519, the Android model runtime JNI; the list does not name it) | 35 | 3,894 |
| VOICE (on the list: audio session, mic, speech, on-device models, deck, take recorder, the voice turn's server stages, host binding) | 112 | 20,866 |

The VOICE mark is the repository's list, not my reading. My own reading (inference): the list is generous. It includes files whose imports are network and file code, not audio: `ChoirDesk.swift` 173 and `AutoradioDesk.swift` 221 (the voice turn's POST stages), `DeskContext.swift`, `ClipLocator.swift` 349 (fingerprint search over decoded PCM; audio analysis, defensible), `Configurator.swift` 425, `ModelCandidates`, `KnownModels` 371, `VoiceSetup.swift` 665, `VoiceChoice.swift` 368, `LabRuntime.swift` 523 (the voice lab), `TurnPolicy.swift` 520 (pure policy; the audit says it could move since every threshold is 300 ms or longer), `AutoradioModule+Onboarding.swift` 435 (first-run voice side), Android `AutoradioModule.kt` 287 (the list itself says it is a WebView and external-URL seam, not voice). I did not read each file's body; marks come from the list's reason text and the head of each file.

Files over 150 lines (Swift/Kotlin/C++; Tests excluded):

NOT VOICE (all non-voice files over 150 lines):
| Lines | File | What it does |
| --- | --- | --- |
| 519 | `exact-package/android/native/native_models.cpp` | JNI model runtime (llama.cpp/ONNX glue); not counted by the gate, not on the list |
| 439 | `AutoradioKit/.../Choir/ChoirClient.swift` | key stores (Keychain, file, memory) and the phone's HTTP client to the computer |
| 288 | `.../Choir/RadioProgramme.swift` | programme segment types and wire for the desk's writer |
| 252 | `.../Choir/ChoirApp.swift` | the engine's controller: sign-in state, sign-out order, programme and desk wiring |
| 235 | `.../BarState.swift` | radio facts for the bar (most fields now read only by tests) |
| 168 | `.../SegmentTracker.swift` | playback events, no production reader (kept for ~30 tests) |
| 157 | `exact-package/apple/AutoradioModule+Platform.swift` | platform seam: audio session owner, console log, background time |
| 148 | `.../SavedAnswerHolder.swift` | the first spoken answer from what the phone holds, while the desk answers |
Total of these 8: 2,206 lines. The other 27 non-voice files are under 150 lines each (1,688 lines).

VOICE (over 150 lines; 53 files; by the list's reasons):
| Lines | File | What it does |
| --- | --- | --- |
| 1,222 | LiveAudio.swift | the one AVAudioEngine: mic tap, player nodes, mixer, reference tap |
| 798 | TurnEngine.swift | the voice turn engine |
| 665 | VoiceSetup.swift | voice setup: model checks and downloads for this phone |
| 601 | Deck.swift | the deck: plays what shared code hands it, journal |
| 523 | LabRuntime.swift | the voice lab runtime |
| 520 | Voice/TurnPolicy.swift | turn policy (yield, barge-in) |
| 479 | AutoradioModels/HubModelProvider.swift | model download provider |
| 435 | exact-package/apple/AutoradioModule+Onboarding.swift | first run's voice side |
| 430 | android/DeckMediaEngine.kt | Media3 playback, on-device speech to files, audio focus |
| 425 | Configurator.swift | picks one on-device model per voice role |
| 410 | AutoradioModels/LlamaSystem1.swift | llama.cpp System 1 stage |
| 397 | LocalAck.swift | the on-phone model's immediate spoken response |
| 388 | android/DeckController.kt | the deck's named rules |
| 381 | SessionCoordinator.swift | the one owner of the audio session |
| 371 | KnownModels.swift | the catalog of on-device models |
| 368 | VoiceChoice.swift | catalog of voices |
| 355 | HandsFree.swift | hands-free listening |
| 349 | ClipLocator.swift; 349 ModelStore.swift | clip finding by fingerprint; model weights store |
| 320 | ModelPins.swift; 319 android/DeckJournal.kt | pinned model files; durable deck journal |
| 316 | Stages.swift | stage protocols |
| 297 | Voice/SpeakerProfile.swift | speaker identity |
| 287 | android/AutoradioModule.kt | Android WebView and external-URL seam (not voice by its own text) |
| 274 | RemoteAudioPlayer.swift | AVPlayer for remote audio |
| 273 | BasicStages.swift | model-free voice stages |
| 258 | DownloadDoor.swift | the one door for model downloads |
| 246 | Core.swift | stage descriptors |
| 244 / 227 / 221 | TinyVocaRunner / TinyVocaHost / TinyVocaVoice | Tiny Voca TTS |
| 240 | TakeSession.swift | take recorder |
| 236 | Events.swift | engine clock and event log |
| 226 | exact-package/apple/AutoradioModule.swift | module class registering names with Exact |
| 221 | Voice/ListenerPipeline, Voice/VoiceContract, Choir/AutoradioDesk | pipeline chooser; voice contract; the Autoradio desk POST stage |
| 210 | PlaybackContinuity.swift | audio through background, lock screen, interruptions |
| 200 | AutoradioModels/VoiceStages.swift | on-device model stages |
| 193 | Voice/VoicePreparation.swift | voice preparation |
| 185 | autoradio-models/main.swift | Mac command proving models |
| 180 / 180 / 179 / 177 / 174 / 173 / 169 / 156 | KeptTurns / android/NativeModels.kt / android/ModelDownloadManager.kt / AppleStages / Audio / Choir/ChoirDesk / ModelCandidates / android/NativeModelSelfTest.kt | kept mic turns; one llama model at a time; manifest-pinned download; Apple speech stages; PCM buses; Texture desk stage; candidate table; model self-test |

Small files (<=150 lines) in each mark are in my counts above but not itemised. Android Kotlin files all sit on the voice list (deck, models) except `AutoradioModule.kt`, which the list also includes with a non-voice reason; no Kotlin sign-in, mail or page logic exists (shared TS owns it).

## 3. The Exact pin

- Pin: `mobile/exact.pin` = `f5334750386192245ee76add13a67ab51fb4e2fd` (upstream `main`, committed 2026-11-21T08:13:59+09:00; moved 2026-11-20 from `de212e671`). Also recorded in prose in `docs/reference/EXACT.md:3-4`, `docs/reference/exact/build-and-ship.md:68,425`, `exact-patches/README.md`. Read by `mobile/scripts/setup-exact.sh` (line 6), `check-exact-pin.sh` (refuses a build when `.exact-src` is not at the pin or any patch is not applied).
- CI installs a stub `@exact/passkeys` and does not check out Exact (`ci.yml` lines 798-803), so no CI step depends on the pin.
- Upstream clone (untrusted, read with `git -C`): HEAD `1c11c9bd89ac5216101473f28dfee93df2e9dd3f`, 2026-11-26T11:57:01-08:00, "media-capture: restart AVAudioEngine in place on configuration change". The pin is an ancestor. **1,266 commits** from the pin to that HEAD (4,444 files changed, +312,596/-48,193).
- The clone's object store also holds `8552d20f9b50a013fc19d5fc9298ff149d8b2dc5` (2026-11-26T23:02:26-08:00, "The web finds class-held readers and scrollers, and seeds each input by name (#2015)"), which `docs/big-picture.md:371` names as read on 2026-11-27. It is a descendant of the clone's HEAD (100 commits past it) and **1,366 commits past the pin**. Which is "latest" upstream today I cannot know without the network; 8552d20f9 is the newest I can see.
- Patches under `mobile/exact-patches/` (applied by `setup-exact.sh` in filename order; no fork):
  - `0001-markdown-editor-hides-all-syntax.patch` (166 lines; touches `host/apple/Sources/ExactMarkdown/MarkupEditor.swift`, `MarkupHidden.swift`, adds `MarkupSyntaxHiding.swift`): the iOS Markdown editor hides every syntax marker instead of dimming it (owner, 2026-11-16). README: "not compiled or run" by us.
  - `0002-android-package-shared-libraries.patch` (73 lines; `scripts/agent-android.mjs`, `host/android/app/build.gradle.kts`): a bound package's `jniLibs/<abi>/*.so` is checked (ELF magic, 16 KiB alignment) and packaged into the APK. Verified on the Motorola per README.
  - `0003-media-player-for-background-playback.patch` (17 lines; `scripts/agent-android.mjs`): link Media3 and the playback service when `audio_session` is `playback`. Verified on the Motorola.
- Do the patches apply upstream? Tested with `git apply --cached` against a scratch index read from the tree (no clone file touched): 0001 applies at both `1c11c9bd8` and `8552d20f9`. **0002 and 0003 fail at both** (`scripts/agent-android.mjs` changed: 22 commits to that file since the pin). Inference: both need re-basing by hand on the new pin; upstream has its own GPU `jniLibs` staging now (`scripts/agent-android.mjs:1032`), which may overlap 0002. Patch 0001 is also unchanged in the files it depends on (`markup.rs` differs by one line between pin and HEAD).
- Already planned: station X0 of the cutover definition ("the pin moves... read the grammar and package-interface diff against our contracts and mobile/exact-package first"), and the how-to is `docs/reference/exact/build-and-ship.md` lines 442-460 (pin move recipe). Previous move (2026-11-20/21): "803 shared tests passed" first run on the new pin (`build-and-ship.md:436`).

## 4. Signed out, today

- Public calls (no session): `getJSON`/`getText` in `mobile/AutoradioLab/api.ts:27-37` (header: "the routes the shared pages call are public: no sign-in, no token"). Callers: `/api/platform/publications/front`, `/publications/resolve` (`content.ts`, `radio-bridge.ts`, `paper.ts`), `/api/platform/podcast-popular` (`podcast.ts`), `/health` (`about.ts`, `content-run.ts`).
- Session calls: `authedJSON/Text/Bytes/ToFile` (`api.ts:365-402`), used by `podcast.ts` (feeds, transcript, subscriptions), `textures.ts`, `takes.ts`, `radio-bridge.ts`, `content.ts`, `themes.ts`, `texture-edit.ts`, `steer.ts`, `settings.ts`, `runsheet-store.ts`, `reader.ts`, `notebook*.ts`, `mail.ts`, `files.ts`, `computer.ts`, `activity.ts`, `account.ts`. Without a key they throw `SignedOutError` with a reason `none | locked | refused | unknown` (`api.ts:65-69`), and the sentence on the page comes from `account-state.ts` (one list of words in `words.ts`: "This is the public paper. Sign in to hear your own programme."). The slim banner is `signedout.contract`.
- Works signed out (documents): the paper reads, a story opens, and the radio reads the public front page aloud (`current-architecture.md` section 6 "Signed out"; `radio-director.ts:92,265` places "the paper's front page" when nothing is ahead). Android: signed-out Paper fetched 100 stories and opened a story; the Kotlin deck played a signed-out station with on-device speech (`system-map.md`, `current-architecture.md`).
- **No turn completes signed out.** `TurnGate.swift` (`allowed` is false with no key) is read at `HandsFree.swift:304-305` (cancels the session) and `LabRuntime.swift:299`; `AutoradioModule+Onboarding.swift:69` sets `keyPresent` from the stored API key. Owner ruling 2026-11-23: "logged out should be listening only. if you ask it should prompt you to log in / sign up" (`choir-shipping-the-app-2026-11-23.md`). So the mission's goal ("a signed-out turn that completes on the phone") reverses that ruling; I found no ruling that does so. Inference: the 2026-11-27 campaign line and `docs/big-picture.md:141` ("a turn that completes on the phone, signed out") is the new intent; its text is only a list item, with no design.
- Pieces that exist for a local turn: `TurnEngine.swift` already has a "local stream" (lines 541-616: the local model and the server desk are shown the same retrieved passages; the local model speaks while the desk is awaited; `LocalAck.swift` 397 lines: the on-phone model's immediate spoken response; `SavedAnswerHolder.swift` 148: a first answer from what the phone holds with the small downloaded model). With a desk (`desk != nil`) it posts `POST /api/autoradio/turns` (`Choir/AutoradioDesk.swift:168`). Inference: with the gate lifted and no desk, the engine's `desk: nil` path ("local") would speak from the local model only; I did not trace whether it ends the turn cleanly, and no test I read covers it. Whether the on-device model answer is good enough, and whether the models are downloaded for a signed-out first run, is not documented ("On-device models: download state on the owner's phone is unknown", `system-map.md`).

## 5. Unlanded work, by lane

Measured against origin/main 8e8c86f3d. "ahead" is `git rev-list`; "patch-ids not on main" is `git cherry` (a commit with an equal patch is on main by another route). "Added files on main" counts files the lane added (non-docs) that exist on main; "deleted files still on main" counts files the lane deleted that main still has. All last commits are 2026-11-24 or earlier. Landed by another route: the 2026-11-24/25 App Review train (`ed89268fa`) and `877b8b17d`, `5afd187e9`.

| Lane (branch) | Ahead / patch-ids not on main | Last commit | Uncommitted in tree | Files vs merge-base (top dirs) | Added on main / deleted still on main | Verdict |
| --- | --- | --- | --- | --- | --- | --- |
| `move-a` (also origin/lane/move-a) | 48 / 35 | 2026-11-24 | 0 | 242 (`mobile/AutoradioKit`, `AutoradioLab`, `exact-package`) | 54 of 55 added are on main; 0 of 30 deletions still on main | looks landed by another route (only `AuthShapeTests.swift` missing) |
| `move-b` (also origin/lane/move-b) | 48 / 35 | 2026-11-24 | 0 | 276 | 51 of 54 added; 4 of 34 deletions still on main: `AutoradioModule+Imported.swift`, `+Mail.swift`, `interface/imported.json`, `interface/mail.json` | mostly landed; residue: native Imported and Mail module files and 3 new tests (`exact-facts.test.ts`, `key-once.test.ts`, `round-trips.test.ts`) not on main. Looks partly unlanded |
| `unified-voice-download` | 51 / 42 | 2026-11-24 | **22** | 183 (Kit voice/model files) | 44 of 47; 0 deletions pending | mostly landed; unlanded: `Voice/VoiceModelPacks.swift`, `VoiceTransferRoutingTests.swift`, `AuthShapeTests.swift`. Looks partly unlanded, with 22 uncommitted paths (not read) |
| `voice-models` | 21 / 21 | 2026-11-24 | 0 | 95 | 25 of 29; missing: `Voice/TrialGate.swift`, `VoiceSamplePlayer.swift`, `SamplePlayTests.swift`, `TrialAudioGateTests.swift` | looks unlanded (the Test-on-Voice-and-models gate; problem record + structural assessment in its docs) |
| `voice-more` (= `rev-voicemore`) | 29 / 4 | 2026-11-24 | 0 | 121 | 44 of 48; missing `downloads.ts`, `downloads.contract`, `downloads-types.contract`, `downloads.test.ts` | looks unlanded for 4 commits: Settings > Downloads page, 53 English voices + LFM2.5-350M, Phonon in the download group, "first run: listening-only signed out, model downloads offered with sizes before any byte" (`e207859d6`) |
| `voice-default` | 5 / 4 | 2026-11-23 | 0 | 32 | 7 of 7 | looks landed by another route |
| `voice-pro`, `voice-pro-rb` | 7 / 6 and 4 / 4 | 2026-11-23 | 0 | 33, 17 | 9/9 and 2/2 | looks landed by another route (Fluid Speech Pro recogniser row; main's ship docs name it) |
| `exact-rest` (also origin/lane/exact-rest) | 8 / 8 | 2026-11-23 | 0 | 132 | 15 of 17 added; 3 deletions still on main: `MailSeal.swift`, `MailSealTests.swift`, `mail-seal.ts` | partly unlanded: mail seal removal; `VoiceFrontTests.swift` and `voice-front.test.ts` missing |
| `exact-rest-a` | 2 / 2 | 2026-11-24 | 0 | 81 | 9/9; 0 pending | looks landed (commit `877b8b17d` carries its subject) |
| `exr-fix2` | 4 / 3 | 2026-11-24 | 0 | 83 | 9/9 | looks landed (`5afd187e9`, `5fb2dc2b8` on main) |
| `exr-stack` tree, branch `exr-r` (also `exact-editor`, `exr-b`, `exr-c`) | `exr-r` 13/13; `exact-editor` 6/6; `exr-b` 14/10; `exr-c` 9/5 | 2026-11-24 | `exr-stack`: **12** | 76 (exr-r) | `exr-r`: 11 of 13 added; missing `internal/textureowner/phone_stabilize_test.go`, `AuthShapeTests.swift`. `exact-editor`: 15/15, 0 pending | editor landed; residue in `exr-r` (newest: "a document the phone cannot write exactly opens read-only", "editor saves cited and embedded documents", server projection tests). Looks partly unlanded; 12 uncommitted paths not read |
| `exact-editor` tree | 6 / 6 | 2026-11-24 | 0 | 111 | 15 of 15; 26 deletions all gone from main | looks landed by another route (changes.md:393) |
| `exact-edition-picks` | 4 / 4 | 2026-11-21 | 0 | 30 | 3 of 3 | looks landed (old, superseded) |
| `rebased` (tree `account-delete`; also origin/lane/account-delete 36 ahead) | 38 / 38 | 2026-11-24 | 0 | 101 | 38 of 38 added files are on main | server, frontend and `account-delete.ts` appear on main; the last commits ("round 3": backup runs copy the deletion ledger off-host; state not clock is the cancel boundary; recovery link not mailed while deletion pending) may be unlanded. Cannot tell; `docs/ship/data-inventory.md` F14 says built, not deployed |
| `origin/lane/account-boundary-l1` | 15 / 15 | 2026-11-21 | (no tree) | 65 | 0 of 4 added on main (`account-phone.ts`, `account-world.test.ts`, `app-gate.test.ts`, `module-scan.ts`) | looks unlanded; the owner's choice of account boundary is open per `docs/changes.md:355` and the structural assessment of 2026-11-23 |
| `origin/lane/offline-slice-r0` | 26 / 26 | 2026-11-22 | (no tree) | 40 (cmd/offlineslice, docs) | 0 of 39 | unlanded; not a phone-app change (offline experiments in Go); likely out of mission |
| `offline-experiments` | 8 / 8 | 2026-11-24 | 0 | 38 (`scripts/offline-experiments`, docs) | 0 of 14 | unlanded; not phone-app; out of mission |
| `origin/lane/phone-int` | 45 / 32 | 2026-11-24 | (no tree) | 227 | not computed | integration branch; last commits "share the launch settle and the host decision as values, not a kept promise with a timer", "problem record: one let-go answer fails every shared-code request on the phone". 164 of 211 non-doc files identical to main. Cannot tell |
| older `origin/lane/exact-*`, `phone-*`, `autoradio-integ*` (11-18 to 11-21) | 1 to 223 ahead, mostly not by patch-id | 2026-11-18..21 | | | | superseded by the train (inference: their content is the lanes the train merged); not checked file by file |

Do not rely on the "files identical to main" percentage in `lanes-out.txt`; it is confounded by main moving on. The "added on main / deleted still on main" columns are the more reliable test.

## 6. How the phone app is built and tested, as documented

- Allowed in a lane without a native build (CLAUDE.md "Build output lives in one place"): the Exact loop against the lead's one checkout: `EXACT2=<build tree>/mobile/.exact-src`, then `scripts/sync-exact-package.sh`, `bun install`, and `bun exact.mjs contract build`, `test web`, `agent web`, and `bun test` in `mobile/AutoradioLab`. Also Swift package tests, one at a time, through the lock: `/usr/bin/lockf -k -t 7200 ~/.cache/autoradio-mac-build.lock ...`.
- Lead only: `mobile/scripts/setup-exact.sh` and `build-lab.sh` (refuse outside the lead's build tree), every Exact native build, device builds, simulators and emulators (a lane may get a slim simulator only via `AR_SIM_NAME=<name> mobile/scripts/sim-device.sh`, shut down at the end, one build lock for native builds), `scripts/gc-build-output`, App Store archive/upload (`mobile/scripts/upload-store.sh`, `store-negative-control.sh`), the owner's phone installs.
- Shared-code tests: `mobile/AutoradioLab/*.test.ts` and `*.test.contract` (107 files in my copy), fixtures `mobile/fixtures` and `internal/texturedoc/golden`. Swift tests: `mobile/AutoradioKit/Tests` (92 files, 881 `func test` by my grep) run on the Mac only.
- CI job "Autoradio Shared-Code Tests" (`.github/workflows/ci.yml:776-811`): `ubuntu-latest`, 20 minute timeout, runs when `needs.plan.outputs.mobile == 'true'`; steps: Node 22, `npm install --global bun@1.4.2`, a stub `mobile/.exact-src/packages/passkeys/package.json`, `cd mobile/AutoradioLab && bun install --frozen-lockfile`, `bun test` (about 20 s), then `cd mobile && sh scripts/test-app-identity.sh`. Triggered by `.github/scripts/ci-impact-classify` line 127: any change under `mobile/AutoradioLab/*`, `mobile/fixtures/*`, `mobile/layouts/*`, `mobile/themes/*`, `mobile/exact-package/exact.json`, `mobile/exact-package/interface/*`, `mobile/scripts/gen-exact-interface.py`, `mobile/scripts/*.sh`, `mobile/docs/typography.md`, `internal/autoradioqueue/queue.go`, `internal/texturedoc/golden/*`, the workflow or the classifier. No Exact checkout, no Xcode. Swift under `AutoradioKit` and `exact-package/apple` is not tested in CI (classifier comment line 125).
- The native-boundary gate `scripts/check-autoradio-native-boundary` runs from `scripts/gates` (stated in its header). Contract-compiler loop cost (problem record 2026-11-18): about 57 to 90 s for a one-line page edit before the pages were slimmed; not re-measured today.

## 7. The locked-phone test that is owed

- `docs/big-picture.md:141` lists "the locked-phone test" as a prerequisite of 1.1; `:371-377` ("Not known"): "Whether shared code keeps running on a locked phone. Read at Exact `8552d20f9` (2026-11-27): Exact does not freeze the page while the app holds a playback or speech claim... Whether iOS keeps the process running is not measured by us or by Exact. A ten-minute locked-phone log settles it; the native deck's fallback rules stay until it is run."
- `docs/definitions/choir-exact-cutover-and-ontology-2026-11-20.md:318-326` station X9 "The device probe", `status: pending`: native fact to shared code to native command round trip, and whether shared code keeps running on a locked phone while a speech-playback claim is held; note `docs/evidence/exact-conversion-audit-2026-11-24.md` slice 1 (it "deletes nothing; only a device can prove it").
- `docs/definitions/choir-autoradio-exact-cutover-2026-11-18.md:60-61` (an acceptance row: a locked hour, app killed once, every entry heard locked reached the server with its time heard), `:117`, `:361` (conjecture delta_o: "a locked phone is the oracle we lack").
- `docs/changes.md:356`: "Autoradio 1... the owner's tests are owed: a locked phone, a first-launch microphone, the sign-in sheet".
- `docs/reference/exact/audio.md:133`: at `f53347503` "on iOS and tvOS the page is `frozen` in the background unless `backgroundExecutionEligible()` is true" (the `backgroundExecutionEligible` text; read this page for the rule). Also `docs/reference/exact/answers-let-go.md` (an answer let go after 100 ms).
- No record of a run exists in docs (I found none). Inference: it is not done.

## 8. App Store state, from documents

- Autoradio 1.0: build `473021` from `16f04df4d`, `WAITING_FOR_REVIEW` at the 2026-11-25 readback; the owner submitted it (`docs/ship/submission-checklist.md:3-13`). Approved 2026-11-25, "on the App Store and has no users yet" (`docs/definitions/choir-campaign-2026-11-27.md`, `what_is_true_now`, captured 2026-11-27T08:56Z). The owner on 2026-11-27: "Apple approved it!" (`docs/inbox.md:243`). The approval email itself is not in the repo; the date comes from the campaign file. Bundle `news.choir.autoradio`, name "Autoradio 1", version 1.0 (`docs/changes.md:529`).
- Version 1.1 (`docs/big-picture.md:126-141`, `docs/inbox.md:238-240`, campaign rows at `choir-campaign-2026-11-27.md:157-159`): "the app made nice with the features it has, deeper use of on-device models, and the onboarding voice interview. The move to Exact finishes first, on the latest Exact. The docs transform is its prerequisite." Needed first: the docs transform; the batched lander; the Exact pin updated and the move finished; the locked-phone test; a turn that completes on the phone, signed out; account deletion deployed. Version 1.2: takes and social. Unit of planning: one submission about a week, each a real feature step (owner, 2026-11-27).

## What I could not determine

- Which upstream commit is newest today (no network): I can see 1c11c9bd8 (clone HEAD) and 8552d20f9 (in the clone's objects).
- Whether patches 0002 and 0003 can be re-made against upstream without losing function (they fail to apply; I did not read the new upstream code in detail).
- The contents of the uncommitted paths in `unified-voice-download` (22) and `exr-stack` (12) (counted only; not opened).
- Exact content of the "phone-int" integration branch beyond the subjects, and the other 11-18 to 11-21 lanes.
- Whether the signed-out local-only turn works if the gate is lifted (not traced or run).
- Whether the on-device models are on the owner's phone (the documents say unknown).
- Whether the later rounds of `rebased`/`account-delete` (round 3) landed.
- Whether each file marked VOICE by the list would be VOICE by the owner's rule (a judgment; the audit disagrees for some).
- Status of CI/deploy on main today and the deployed commit.

Scratch files (counting scripts, copy of `mobile/`, `lanes-out.txt`) are in `/private/tmp/claude-501/-Users-wiz-Dev-go-choir/7912d8c6-1450-40ed-b61a-78e7dab6ea69/scratchpad/exact-facts/`.
