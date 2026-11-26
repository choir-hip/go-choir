# Autoradio Android parity matrix

Checked 2026-11-26 against the shipped Exact iOS source and the physical
Motorola evidence. This is the inventory required by station A0 and the scorecard
station A7 must fill. It is not a claim that Android has parity.

## Status words

- **proved** — the named Android journey has physical-device or staging evidence.
- **partial** — a named subset is proved; the row says exactly what remains.
- **blocked** — a required native/auth/model seam is absent or unproved.
- **unproved** — shared code exists, but no Android device result is cited.

A row passes only when the same shared TypeScript/Contract source and native
contract produce the observed result on `ZY22H5K8QR`. A substitute, a hidden
control, an emulator or a Kotlin copy of page logic does not pass.

## Page and journey inventory

| ID | Shipped iOS journey | Shared authority | Native dependency | Android status | Evidence or remaining gap | Owning station |
| --- | --- | --- | --- | --- | --- | --- |
| P01 | Launcher and signed-out Paper | `launcher.contract`, `paper.ts`, `pages.ts` | None for the public front | **proved** | A0 fetched 100 staging stories and drew the Paper; A3 repeated the launcher/Paper path. | A0 |
| P02 | Story reader | `reader.contract`, `reader.ts`, `reader-parse.ts` | `pageText` hand-off is optional for voice | **proved** for reading | A0 opened a story; A3 opened a current story and its source. Voice hand-off still refuses on Android. | A0/A6 |
| P03 | Source page inside the app | `reader.contract`, `content.contract` | Package view `web-page` | **proved** | A3 rendered `pravda.com.ua` in `NativeView [reader-web-view]`, closed it and cold-relaunched. | A3 |
| P04 | Open an HTTPS source in the browser | `members-reader.contract`, `members-content.contract` | `readerOpenExternal` | **proved** for one HTTPS source | A3 resumed Chrome through the package send. Other schemes/apps and malformed URL matrix remain code-only. | A3 |
| P05 | Reader follow and aim | `reader.contract`, `members-reader.contract` | `readerFollow`, `readerAim`, `readerAimSet` | **blocked** | A0 recorded both sources substituted; no Android implementation. | A3 |
| P06 | Share a document | `reader.contract`, `content.contract` | Platform share sheet / exported file | **unproved** | No Android share/export proof. | A3 |
| P07 | Files browse, upload, open, rename, delete, share | `files.contract`, `files.ts` | System picker, file provider, viewers, audio hand-off | **blocked** | No physical Android Files journey; picker/file-provider/preview paths are not proved. | A3 |
| P08 | Imported PDF/EPUB reading and read-aloud | `imported-docs.contract`, `imported.ts` | `importedFollow`, `radioHandWords`, picker, deck | **blocked** | A0 recorded `importedFollow` substituted; no import/open/read-aloud proof. | A2/A3 |
| P09 | Radio page, queue and run-sheet state | `radio.contract`, `radio-runtime.ts`, `radio-director.ts` | `radioNote`, `radioView`, `deckState` | **partial** | The deck's state, journal, held list and wipe are native and drive the station on the phone (`docs/evidence/android-a2-deck-2026-11-26/`); `radioNote`/`radioView` are still substituted, so the page's own facts are not all native-proved. | A2 |
| P10 | Radio speech playback | `radio-runtime.ts`, `deck-native.ts` | `deckPlay`, `deckEnqueue`, `deckAct`, `deckAck`, local TTS | **proved** | A signed-out station played on the phone through the package names (on-device speech to Media3), kept playing with the activity hidden, and answered pause/play/next; the APK and raw session states are in `docs/evidence/android-a2-deck-2026-11-26/`. | A2 |
| P11 | Podcast browse, search and subscribe | `podcasts.contract`, `podcast.ts` | Shared network/storage | **unproved** | No physical Android podcast browse/subscribe drive. | A3/A7 |
| P12 | Podcast playback, queue, resume and clips | `podcasts.contract`, `podcast.ts`, `clips.ts` | Deck, clip location, file/audio hand-off | **blocked** | No Android playback/background/clip-location proof. | A2/A3 |
| P13 | Lock-screen/headset media controls | `radio.contract`, `deck-native.ts` | Media3 `MediaSessionService`, audio focus, noisy-route receiver | **partial** | The service, the session and its pause/play/next were driven on the phone; a physical headset/route change, a phone call's transient loss and a locked screen are unproved. | A2 |
| P14 | Passkey sign-in and relaunch | `app-signin.ts`, `passkey-ceremony.ts` | `@exact/passkeys`, Android asset association, server RP origin | **blocked** | Android sign-in is deliberately refused; no association/origin. Device has zero enrolled biometrics, so the final prompt is an owner action. | A1 |
| P15 | Account state, sign-out and account wipe | `account.ts`, `account-state.ts`, `api.ts` | `choirAuth`, Exact secrets, `signOut`/`wipe`, deck/mail/model wipe | **blocked** | `choirAuth` substitutes signed out; Android has no app-module sign-out/wipe path. | A1/A2/A4/A6 |
| P16 | Account deletion | `account-delete.ts`, `settings.contract` | Passkey plus A1 account boundary | **blocked** | No Android deletion proof; must use d@e only. | A1/A7 |
| P17 | Mail list, read and sealed cache | `mail.contract`, `mail.ts` | `mailSeal`, `mailOpen`, `mailForget`, Android Keystore | **blocked** | No Android mail cipher or cache proof. | A4 |
| P18 | Mail compose, review and send | `mail.contract`, `mail.ts` | Mail cipher plus shared server client | **blocked** | No Android compose/review/send proof. | A4 |
| P19 | Textures list and editor | `textures.contract`, `texture-edit.ts` | Shared editor plus any native document/provider seam | **unproved** | No physical Android Texture list/edit/save proof. | A3/A7 |
| P20 | Notebook and edition topics | `notebook.contract`, `notebook-run.ts`, `edition-picks.ts` | Shared storage; native page list only where still present | **unproved** | No physical Android notebook/topics proof. | A3/A7 |
| P21 | Takes record, review, preview and publish | `radio.contract`, `take-session.ts` | `takesRecorder`, microphone, ASR, local playback/upload | **blocked** | A0 recorded `takesRecorder` substituted; no microphone/ASR proof. | A6 |
| P22 | Themes and appearance | `settings.contract`, `themes.ts` | Shared theme facts; `settingsFacts` for native build/radio facts | **unproved** | Shared theme code exists; no Android appearance/settings drive. | A3/A7 |
| P23 | First-run onboarding and voice setup | `onboarding.contract`, `onboarding.ts` | `voiceSetupFacts`, `ob*`, microphone, models, TTS | **blocked** | A0 recorded `voiceSetupFacts` substituted; no model download/setup proof. | A5/A6 |
| P24 | Voice and model settings | `models.contract`, `models.ts`, `retrieval-run.ts` | Model catalog/download/test/remove, `embed*` | **partial** | The APK's own JNI runtime executes Qwen3-0.6B and EmbeddingGemma 2, and its packaged ONNX Runtime runs Silero through the C API, all on the phone (`docs/evidence/android-a6-model-runtime-2026-11-26/`); the shared models page still has no Android `ob*`/`embed*` native seam, so the page itself, the download path and the Silero Kotlin class are unproved. | A6 |
| P25 | Hands-free local voice and streamed desk turn | `voice.contract`, `radio-bridge.ts`, `retrieval-live.ts` | Mic, VAD, live/final ASR, local Qwen, TTS, speaker, embedding | **blocked** | No real-microphone Android turn; no network/code-path proof that audio stays local. | A6 |
| P26 | Activity, Computer and About | `activity.contract`, `settings.contract`, `computer.ts` | `settingsFacts`, `choirAuth`, shared server reads | **blocked** | A0 recorded `settingsFacts`/`choirAuth` substituted; no authenticated Android proof. | A1/A3 |
| P27 | Launcher search over held pages/files/stories | `launcher.contract`, `retrieval-live.ts`, `search.ts` | `retrievalAsk`, optional embedding model | **unproved** | No Android search proof; lexical/vector behavior is not inferred from Paper. | A5/A6 |
| P28 | Full shipped-iOS parity and Autoradio 1 APK | All of the above | Every required row | **not complete** | P01–P04 and P10 have physical Android evidence; the rest are partial, blocked or unproved. | A7 |

## Native package sources and views

A0 recorded 17 package sources substituted. The Android package now implements
the entries marked proved (the `web-page` view, `readerOpenExternal`, `deckState`
and the deck's sends); every other entry still answers `NotSupportedError` until
its station lands.

| Package name | Purpose | Android state | Station |
| --- | --- | --- | --- |
| `readerOpenExternal` | External URL hand-off | **proved** for HTTPS | A3 |
| `web-page` | In-app source view | **proved** open/close/relaunch | A3 |
| `pageRequest` | Native page request | missing | A3 |
| `readerFollow` | Current reader block | missing | A3 |
| `readerAim` | Aimed reader block | missing | A3 |
| `importedFollow` | Current imported-document block | missing | A3 |
| `choirAuth` | Signed-in fact | missing; A1 moves it shared | A1 |
| `choirPlayer` | Native player fact | missing; the deck now plays, the page's fact is still substituted | A2/A6 |
| `barFacts` | Voice/player bar fact | missing | A2/A6 |
| `radioNote` | Radio pulse | missing | A2 |
| `radioView` | Radio native view | missing | A2 |
| `deckState` | Durable deck state/journal | **proved** (native source; the station plays) | A2 |
| `deckPlay`, `deckEnqueue`, `deckAct`, `deckAck`, `deckWipe` | The deck's commands | **proved** for play and the media session's pause/play/next, which act on the deck; enqueue/ack/wipe are code- and test-proved | A2 |
| `settingsFacts` | Build/clock/radio facts | missing | A3/A6 |
| `conversationEntries` | Native conversation entries | missing | A6 |
| `retrievalAsk` | Native ask for shared retrieval | missing | A6 |
| `takesRecorder` | Microphone/ASR recorder | missing | A6 |
| `voiceCards` | Voice picker facts | missing | A6 |
| `voiceProfile` | Speaker profile facts | missing | A6 |
| `voiceSetupFacts` | Model setup/download facts | missing | A5/A6 |

## Native app-module operations

The current Android host has no equivalent of the Apple app-module op bus. A0
recorded 48 `No value for extension` failures. Package sends are the preferred
replacement where the operation is a declared package contract; a genuine app
module seam needs an explicit Android implementation.

| Operation family | Purpose | Android state | Station |
| --- | --- | --- | --- |
| `generation` | Account-generation fence | missing | A1 |
| `keystate`, `session`, `get` | Exact secret reads | missing app-module path; A1 can use shared secret store | A1 |
| `signOut`, `wipe` | Native account wipe | missing | A1 |
| `deck` | Five deck commands | **replaced**: the package names `deckPlay`/`deckEnqueue`/`deckAct`/`deckAck`/`deckWipe` are implemented and physically proved | A2 |
| `openPage` | Native page request | missing | A3 |
| `radioFacts`, `radioSet` | Voice/audio mailbox | missing | A2/A6 |
| `ob*` | Onboarding/model setup/mic/TTS | missing | A6 |
| `embedState`, `embedEnsure`, `embed` | Embedding model lifecycle | missing | A5/A6 |
| `retrievalAnswer` | Shared answer to native retrieval ask | missing | A6 |
| `contentBusy` | Content-sync native fact | missing | A3 |
| `take*` | Take recorder/player | missing | A6 |
| `pagesMirror`, `pageText` | Native voice page context | missing | A6 |
| `edition*` | Voice edition picks | missing | A6 |
| `mailSeal`, `mailOpen`, `mailForget` | Mail confidentiality | missing | A4 |

## Evidence anchors

- A0 signed-out Paper/story and native-call inventory:
  `docs/evidence/android-a0-2026-11-26/`.
- A3 in-app WebView, external hand-off, close and cold relaunch:
  `docs/evidence/android-a3-web-view-2026-11-26/`.
- Package review receipts:
  `docs/evidence/android-package-seat1-*-2026-11-26.txt`.
- Android model benchmarks (outside the app):
  `docs/evidence/model-bench-android-2026-11-26/`.
- The APK's compiled JNI/ONNX Runtime through its own `NativeModels`, on the phone:
  `docs/evidence/android-a6-model-runtime-2026-11-26/`.
- The deck's playback, background survival and media-session controls on the phone:
  `docs/evidence/android-a2-deck-2026-11-26/`.

The matrix is updated in the same change that lands or proves a row. A7 may
close the mission only when every required row has named evidence.
