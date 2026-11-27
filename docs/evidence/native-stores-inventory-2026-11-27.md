# The native stores: what Swift code persists, and what sign-out does with it (2026-11-27)

Standing: governing | kind: exists | settled-by: a reading of origin/main f52809f7c on 2026-11-27 and scripts/check-native-stores | note: only the Swift under mobile/AutoradioKit/, mobile/exact-package/apple/ and mobile/AutoradioLab/modules/ was read; files shared code writes through the Exact runtime (`app:/data`, `app:/cache`) are not native Swift writes and are not here; the "account data" column is a reading of the types and comments, not a test; nothing was built or run on a device

Mutation class: yellow (a gate and a record). Serves station E6.4 and failure-list items G2 and D6 of
[phone-sign-out-and-server-client-in-exact-2026-11-27.md](../design/phone-sign-out-and-server-client-in-exact-2026-11-27.md),
item 4 of [the sign-out problem record](../problems/phone-sign-out-leaves-the-microphone-open-past-the-key-2026-11-27.md).
Nothing was fixed: this is the inventory and the gate that keeps it true.

## What was counted

`python3 -I scripts/check-native-stores` finds the lines that persist (file writes, `createDirectory`, `copyItem`/`moveItem`,
`FileHandle(forWriting`, `AVAudioFile(forWriting`, `AVAudioRecorder`, model downloads, `UserDefaults` sets, `SecItemAdd`/`SecItemUpdate`)
in the Swift sources, tests excluded. **86 lines in 31 files**, owned by 36 entries of `scripts/native-stores.txt`:
**24 wiped, 50 kept, 12 unruled** (counted in lines). The gate fails when a file has a persistence line the list does not own, when an entry has
no line left, or when a token is gone. A new site cannot be added without this list changing in the same diff.
The gate is in `scripts/gates`; CI has no step for it yet.

Kotlin: `mobile/exact-package/android/` holds Kotlin (for example `ModelDownloadManager.kt`, `NativeBridge.kt`); it was not read and the gate does not see it.

## The table (generated from the list; the reasons are the list's)

| File | Token | Sites | Ruling |
| --- | --- | --- | --- |
| `Kit/Choir/ChoirClient.swift` | `KeychainKeyStore` | 1 | wiped:ChoirSignIn.signOut (the Keychain item is deleted before the wipe; ModelsEntry.swift re-reads it) |
| `Kit/Choir/ChoirClient.swift` | `FileKeyStore` | 2 | kept:a key file for the Mac replay and lab tools only (readOnlyKeyFile in autoradio-replay); the phone app uses KeychainKeyStore |
| `Kit/Choir/MailSeal.swift` | `news.choir.autoradio.mailseal` | 1 | wiped:MailSeal().forget() in mobile/AutoradioLab/modules/apple/ModelsEntry.swift (outside the AccountWipe registry) |
| `apple/AutoradioModule+Onboarding.swift` | `voice-setup.json` | 2 | kept:models are on the phone, not in the account; the record says which model was chosen for which stage (stated beside the write, setupFinished) |
| `apple/AutoradioModule.swift` | `filesCacheDisk` | 1 | wiped:app-files (removeContents of the files cache) |
| `apple/AutoradioModule+Platform.swift` | `console.log` | 6 | unruled |
| `Kit/Deck.swift` | `deck-journal.json` | 5 | wiped:deck (Deck.wipe removes the journal and seq.json) |
| `Kit/Deck.swift` | `.bad` | 1 | unruled |
| `Kit/KeptTurns.swift` | `KeptTurnStore` | 6 | wiped:kept-turns (registered when LabRuntime.store is passed in ChoirApp.registerAccountWipe) |
| `Kit/Audio.swift` | `writeMono` | 1 | wiped:kept-turns (the only caller is KeptTurnStore.save) |
| `Kit/VoiceChoice.swift` | `VoiceChoiceStore` | 2 | unruled |
| `Kit/VoiceChoice.swift` | `FileVoiceReadiness` | 2 | kept:which voice model is ready on this phone (device data; voice-ready.json in the model folder) |
| `Kit/LabRuntime.swift` | `ConfigStore` | 2 | unruled |
| `Kit/Core.swift` | `LabConfiguration` | 1 | unruled |
| `Kit/Configurator.swift` | `voice-configuration.json` | 2 | kept:the device probe and the plan chosen for it (chip, memory, disk, model plan); no content of the person |
| `Kit/Choir/ClipLocator.swift` | `autoradio-clips` | 4 | wiped:clips (ClipLocator.wipe; the folder is also emptied at launch) |
| `Kit/TakeSession.swift` | `take-` | 2 | wiped:takes (TakeSession.discard, AccountWipe.removeTakeScratch) |
| `Kit/Voice/SpeakerProfile.swift` | `MikeVoice` | 3 | wiped:voice-profile (SpeakerGate.forget) |
| `Kit/PlayerControls.swift` | `autoradio.playbackSpeed` | 1 | kept:a playback-speed setting of the phone, not the person's content |
| `Kit/KnownModels.swift` | `AutoradioResponderModel` | 2 | kept:which model answers (device setting) |
| `Kit/KnownModels.swift` | `AutoradioDecisionModel` | 2 | kept:which model decides (device setting) |
| `Kit/LabRuntime.swift` | `ResponderChoice.key` | 2 | kept:which model answers or decides (device setting) |
| `Kit/Voice/ModelUse.swift` | `AutoradioModelUse` | 1 | kept:which model is used for a job (device setting) |
| `Kit/Voice/VoicePreparation.swift` | `mike.voice.preparedFor` | 1 | kept:the signature of the voice models last prepared (device data) |
| `Kit/ModelPins.swift` | `ModelManifests` | 4 | kept:model manifests and pending marks next to the models (device data) |
| `Kit/ModelStore.swift` | `KnownModels.storeRoot` | 9 | kept:the model downloads and their .part files (device data, survive sign-out by design D6) |
| `Kit/Voice/WeSpeakerEmbedder.swift` | `WeSpeaker` | 4 | kept:the voice-print model download (device data; the person's profile is a separate store) |
| `Kit/VoiceSetup.swift` | `backend.download` | 1 | kept:model download during voice setup (device data) |
| `Sources/AutoradioModels/ParakeetStages.swift` | `AsrModels.download` | 1 | kept:the speech model download into FluidAudio's own cache (device data) |
| `Sources/AutoradioModels/TinyVocaRunner.swift` | `compiled` | 1 | kept:a compiled speech model (device data) |
| `Kit/FixtureClip.swift` | `autoradio-fixture-clip` | 1 | kept:a synthetic test tone in tmp |
| `Kit/FixtureTurn.swift` | `autoradio-fixture-turn` | 4 | kept:a bundled synthetic turn written to tmp and, only in an EXACT_DEVELOPMENT build, into the kept-turn folder |
| `Sources/AutoradioModels/EmbedDeviceTrial.swift` | `embed-trial.json` | 2 | kept:a proof result in Documents/proof, behind the EmbedTrial launch default (dev proof) |
| `Sources/AutoradioModels/ModelProofLaunch.swift` | `results.json` | 1 | kept:a proof result in Documents/proof, behind the ModelProofStages launch default (dev proof) |
| `Sources/autoradio-models/main.swift` | `jsonOut` | 4 | kept:a Mac command-line tool, not in the app |
| `Sources/autoradio-replay/main.swift` | `fileURLWithPath` | 1 | kept:a Mac command-line tool, not in the app |

Reading the paths: Application Support is `Application Support/Autoradio/` for the app's own files (`deck-journal.json`,
`voice-choice.json`, `turns/`, `configs/`, `console.log`, `voice-setup.json`) and `Application Support/AutoradioLab/models/` for models.

## Account data that sign-out does NOT remove

1. `Application Support/Autoradio/deck-journal.bad` (`Deck.swift`, `setAside`): a journal that failed to decode is moved aside, and `Deck.wipe()`
   removes `deck-journal.json` and `deck-journal.seq.json` only. The journal holds the deck's lines (items, commands, reasons). It is an account store the wipe misses.
2. `Application Support/Autoradio/console.log` and `console.log.1` (`AutoradioModule+Platform.swift`, `ConsoleFile`): every `AUTORADIO ...` line the Kit prints, 256 KB, rotated.
   The comment says lines carry counts and ids, never content; ids of turns and entries are the old account's. No registration removes it. Unruled.
3. Nothing else that holds the person's words, audio or key was found outside a registration: kept turns, takes, clips, the deck journal, the voice profile,
   the sign-in key and the mail key are all wiped (the mail key by `MailSeal().forget()` in `ModelsEntry.swift`, outside the registry; the design moves it in).

## The unruled ones, each with its question

- `console.log` (6 lines, two `ConsoleFile` copies, Mac and iOS): is an id of the old account's turns and entries account data? If yes, wipe it at sign-out (it is open for append, so close and truncate); if no, say so beside the write.
- `deck-journal.bad` (1 line): remove it in `Deck.wipe()`? Almost certainly yes; the question is only whether the corrupt file may be kept for diagnosis once the account has left.
- `voice-choice.json` (`VoiceChoiceStore`, 2 lines): holds a `StageSetting` (a stage id and a free-form `params` map). Does any stage put the person's text in `params`? If not it is device data and becomes kept.
- `configs/*.json` (`ConfigStore.save`, 2 lines) and `LabConfiguration.write(to:)` (1 line): the same `StageSetting` shape. No caller of `ConfigStore.save` was found in the app, so the writer may be dead code (delete it) or lab-only (kept).

## Rulings that were a judgement

- `voice-configuration.json`: kept. Its type is a device probe and a model plan (chip, memory, disk, thermal state, plan, one line per role). Nothing of the person was found in it.
- `voice-setup.json`: kept, on the word of the comment beside the write ("the models are on the phone, not in the account").
- The model downloads, manifests, `.part` files, the FluidAudio cache and the voice-print model: kept (design D6). Nothing asserts they survive a wipe; that is a test, not this gate.
- UserDefaults keys (`autoradio.playbackSpeed`, `AutoradioResponderModel`, `AutoradioDecisionModel`, `AutoradioModelUse`, `mike.voice.preparedFor`): kept as device settings.
- `FileKeyStore` (2 lines): a key file used only by the Mac replay tool with a read-only key file; the app uses the Keychain.

## Not checked

The gate counts lines, so a rewrite that moves a write without changing the count in its file passes; the token check is the guard against that and is weak.
A store written by a library the Kit calls (FluidAudio's cache, `AVAudioRecorder`'s own file beyond its URL, WebKit's data store in the Reader) is seen only where the Kit names the call.
Whether a "wiped" store is really emptied is the sign-out tests' question (`AccountWipeTests`), which this record did not run.
