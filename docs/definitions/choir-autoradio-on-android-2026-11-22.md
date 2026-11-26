---
definition_version: 7
readiness: executable
authority: 'the owner, 2026-11-25: "we have a choir platform... it should stay as is. we have web. same, stay. ios with exact (research exact). it should stay. just got to app store. what we need is android... need to get feature parity with exact ios for exact android... goal is to build autoradio 1 for the connected android."'
review:
  reviewer: none (the owner's statement is the authority; each behavior station names its own review tier)
  frozen_ref: none
  verdict: none
  evidence_ref: none
start:
  captured_at: 2026-11-26T04:33Z
  source:
    canonical_ref: origin/main c1c5f4c40e2e3c789615597ccce65cbb1f0896c1
    deploy_identity: staging proxy and platform on 16f04df4d1dcda2b4a2777968c27926dfed485ba, read from https://choir.news/health at 2026-11-26T04:33Z
  worktrees:
    - path: /Users/wiz/Dev/go-choir
      status: clean
      class: other_agent_wip
      owner: lead
      touch: forbidden
      recovery: leave in place; make every change with lane new from origin/main
    - path: /Users/wiz/Dev/go-choir/.lane/trees/android-1-goal
      status: clean
      class: goal_candidate
      owner: goal author
      touch: goal_owned
      recovery: branch android-1-goal; this file is its only intended change until execution
    - path: /Users/wiz/Dev/go-choir/.lane/trees/qa-mac
      status: clean
      class: goal_candidate
      owner: lead
      touch: goal_owned
      recovery: detached at 16f04df4d1dcda2b4a2777968c27926dfed485ba; preserve as the one native build tree
    - path: /Users/wiz/Dev/go-choir/.lane/trees/qa-mac/mobile/.exact-src
      status: dirty
      class: generated_temp
      owner: lead
      touch: read_only
      recovery: preserve the tracked Markdown patch and untracked MarkupSyntaxHiding.swift; only the lead may run setup-exact.sh under the build lock
    - path: all other pre-existing .lane/trees and .claude/worktrees entries
      status: unknown
      class: other_agent_wip
      owner: unknown
      touch: forbidden
      recovery: leave in place; reconcile with lane ls, git worktree list and the 2026-11-25 lane audit before any nearby mutation
  start_correction:
    on: 2026-11-26
    evidence: 'calibration C198; docs/evidence/model-bench-2026-11-22/README.md; pinned Exact host/android PlaybackService.kt; read-only Android parity audit'
    what_changed: 'An older signed-out shell did run on the Motorola (100 stories fetched, 26 rows drawn, 22 native reads substituted), and a CPU ASR benchmark is already landed. Pinned Exact already has Android Media3 background playback. Neither fact proves current-main parity, the Autoradio Kotlin deck, voice or auth.'
  what_is_true_now:
    - 'The connected Motorola is serial ZY22H5K8QR, moto g stylus 5G - 2023, Android 14, arm64-v8a, MemTotal 3,589,868 kB, about 70 GiB free. adb is authorised and shell input works; the device is awake. adb is not on PATH: use /Users/wiz/Library/Android/sdk/platform-tools/adb or export that directory.'
    - 'news.choir.autoradio.lab versionName 0.1.0 versionCode 1 is installed from 2026-11-22. Its APK is signed by the standard Android debug certificate, SHA-256 dfe9767c642a2f864b297049bde97c0836cb58d5ce516ffe7a7da11f6fec9ea7. This certificate fingerprint is public; no private key belongs in the repository or output.'
    - 'Exact builds and draws Autoradio shared Contract/TypeScript on Android. mobile/exact-package exports only the interface and apple implementation; there is no Android implementation. Native calls therefore answer substitutes or NotSupportedError. The Kotlin deck does not exist.'
    - 'The shared app owns pages, server client, sign-in state, sync, run sheet, mail, files, reader, podcasts, takes and notebook. Native code is for the deck/audio surface, microphone, speech, models and genuine platform seams. Do not port page logic to Kotlin.'
    - 'Android passkeys exist in Exact, but Autoradio deliberately refuses Android sign-in: the server accepts no android:apk-key-hash origin and no Autoradio assetlinks.json is wired. Shared sign-in code otherwise already exists.'
    - 'The installed APK predates the current Exact train. Which current shared pages work on this physical phone, whether Exact audio survives Activity stop, and which models fit are unknown until A0 and A5 measure them.'
    - 'The old origin/lane/android-probe is a pre-train probe, not a base for this mission. Reuse its observations only; build from current main.'
    - 'The lead build tree is clean at the staging commit. Exact is pinned at f5334750386192245ee76add13a67ab51fb4e2fd with the expected local Markdown patch. Bun 1.4.2, Rust aarch64-linux-android, Android SDK/NDK and Homebrew JDK 21 are present. JAVA_HOME is not registered globally; set it explicitly for Android tools.'
  branches_in_flight:
    - 'All pre-existing lanes are forbidden unless the lead explicitly assigns one. In particular do not alter move-a, move-b, unified-voice-download, the voice lanes, the shipping lanes or qa-mac history.'
finish:
  deliver: 'Autoradio 1 runs on the connected Motorola from the same Exact source as iOS, with feature parity for the shipped user journeys: signed-out and signed-in pages, passkey account access, radio and podcasts in the background, reader/files/mail/textures/notebook/takes/settings, and a complete on-device voice configuration. The result is an installable version 1.0 APK, a rerunnable physical-device proof, and a sourced model-portability and benchmark record.'
  artifact: 'on main: the package-first Android ExactExtension implementation of @choir/autoradio, its native web view and genuine platform seams; Android passkey association and server-origin support; a version 1.0 lab-one APK with its SHA-256; scripts/proofs and docs/evidence from serial ZY22H5K8QR; updated Exact and on-device-model reference pages; and this existing Android goal plus ACTIVE/mission-graph reconciled to the proved state'
  acceptance:
    - action: 'from a frozen candidate on current main, build and install news.choir.autoradio.lab.one on ZY22H5K8QR under the Mac build lock, then drive the signed-out paper and a story with Exact agent/android and adb'
      proves: 'the current shared Exact app runs on the physical phone without hand edits or an emulator'
      evidence_class: 'physical-device drive with APK hash, Exact tree/state/screenshots and command log'
    - action: 'publish a page-by-page and native-call inventory for every shipped iOS journey, then rerun the same journeys on Android after the native seams land'
      proves: 'feature parity is measured against the shipped iOS app rather than inferred from one working page'
      evidence_class: 'checked parity matrix linked to device evidence; every required row must pass'
    - action: 'on staging and the Motorola, complete Android passkey sign-in as d@e through the real package/signing certificate, relaunch, read authenticated data, sign out, and prove the account copy and key are gone'
      proves: 'Android account access and the account boundary work with the same shared sign-in code as iOS'
      evidence_class: 'staging auth proof plus physical-device drive; if the system biometric sheet cannot be driven by adb, this remains a named owner action and the mission cannot be complete'
    - action: 'play a radio story and a podcast episode for at least 60 seconds each with the Activity stopped and screen off, read the advancing position, exercise pause/resume/next/previous or headset controls, and reconcile the deck journal with run-sheet ops'
      proves: 'Android audio, background execution, media controls and exactly-once heard accounting work'
      evidence_class: 'physical-device audio-position and journal proof against staging'
    - action: 'on the Motorola, exercise the reader, external links and in-app web view, file import/open/share and audio hand-off, podcast clip location, mail, Texture editor, notebook, takes, themes, onboarding, model settings and account controls'
      proves: 'the remaining platform seams do not silently reduce Android to a reader-only or audio-only app'
      evidence_class: 'physical-device drives using testIds, with adb only where the system UI leaves no Exact driver path and the reason recorded'
    - action: 'seal and reopen mail at rest, reject a changed record or wrong-account read, remove plaintext review files when closed, and wipe the mail key and files at sign-out'
      proves: 'Android mail confidentiality and lifecycle match the shared mail contract'
      evidence_class: 'Kotlin tests plus physical-device and staging proof; no plaintext or key in logs'
    - action: 'benchmark each credible speech, language, voice, speaker and embedding candidate on fixed fixtures on ZY22H5K8QR, recording model/build hashes, runtime, load time, first-token/first-audio latency, real-time factor, word error where applicable, peak memory, headroom and thermal state'
      proves: 'the selected Android configuration is a measured choice for this phone, not a CoreML assumption or a vendor benchmark'
      evidence_class: 'rerunnable benchmark artifact and table in docs/evidence; synthetic speech alone cannot support a quality claim'
    - action: 'run one complete hands-free turn through the real microphone using a fixed spoken fixture played near the phone: local VAD, live/final transcription, local grounded first answer, on-device speech, a plain queue command, and one streamed desk turn to staging d@e; exercise local embedding/search and takes recording; then download, use, test and remove the selected models'
      proves: 'the Android voice path is complete and on-device, with the desk still supplying the authoritative revision'
      evidence_class: 'physical-device turn record, model trace, audio/journal evidence and network/code-path evidence that no audio is uploaded'
    - action: 'land each behavior station through scripts/land, record the pushed SHA, CI, staging deploy where server code changed, deployed identity and deployed proof; update docs/changes.md, current-architecture.md, system-map.md and the reference pages in the same changes'
      proves: 'the artifact is on main and proved in the acceptance environment, not merely built locally'
      evidence_class: 'landing receipts, CI/deploy records and rerunnable deployed proofs'
    - action: 'read back the final APK package id, versionName 1.0, versionCode, signing-certificate SHA-256 and APK SHA-256, and preserve the exact command that reproduces it'
      proves: 'the thing installed on the connected phone is the identified Autoradio 1 candidate'
      evidence_class: 'apksigner/package-manager readback and artifact digest'
  rollback: 'revert each station commit; remove only the Android origin/assetlinks addition; reinstall the prior known-good lab APK or uninstall only news.choir.autoradio.lab.one. Never wipe, root or reflash the phone. No iOS, web or platform redesign is part of any rollback.'
  landing:
    required: true
    environment: staging plus physical Motorola ZY22H5K8QR
    required_receipts: [pushed_commit, ci, deploy_or_docs_only_classification, environment_identity, deployed_acceptance, apk_sha256]
value:
  better_means: 'the share of shipped iOS user journeys that pass on a low-end physical Android with the same Exact source rises to all required rows, and the fully on-device voice configuration is chosen from measurements on that phone'
  goodharting_would_be: 'an APK that installs and shows one page; Kotlin copies of page logic; emulator proof; hidden Android features; fixture injection presented as a real microphone turn; vendor speed claims; cloud speech fallback; a local test called deployed proof'
homotopy:
  realism_axis: 'signed-out shared pages -> signed-in shared pages -> background deck audio -> platform seams -> measured on-device models -> real-microphone voice and full iOS-parity journeys, with the same interfaces and authority at every rung'
boundaries:
  mutation_class: orange overall; red for Android auth/account boundary, mail secrecy and voice/key/session seams
  authority_sources:
    - 'the owner statement above'
    - 'CLAUDE.md Product: the phone app is Exact; only voice and genuine platform work are native'
    - 'the owner rulings quoted below, including device-only voice and old-device inclusion'
    - 'docs/reference/EXACT.md, exact/native-modules.md, exact/deck.md, exact/build-and-ship.md, exact/passkeys.md and ON-DEVICE-MODELS.md, rechecked against primary sources before use'
    - 'observed staging and physical-device evidence, never a design document alone'
  must_preserve:
    - 'Choir platform and web behavior stay as they are; no redesign or unrelated cleanup'
    - 'the shipped iOS Exact app stays as it is; additive shared changes must pass the existing shared suites and an iOS proof when they alter shared behavior'
    - 'one shared TypeScript/Contract authority; no Kotlin page logic, no second run-sheet reducer, no Android-only feature switch'
    - 'no audio leaves the phone for recognition or synthesis; the desk receives text and context as it does on iOS'
    - 'only d@e is used for account drills; do not purge it; no owner account, email, token, private key or secret in the repo or output'
    - 'one Mac native build at a time under /usr/bin/lockf -k -t 7200 ~/.cache/autoradio-mac-build.lock; builders do not run Exact/native/device builds'
    - 'build only in qa-mac or its lead-approved successor; no Exact copy or compiled output in a builder lane; run scripts/gc-build-output at session start and before the device build'
    - 'drive by testId through Exact; use adb for install, system UI and readback, never coordinate tapping as the normal proof'
    - 'every behavior landing carries Ring-Expedite naming the Android parity defect it unblocks, and every landing is represented in docs/changes.md'
  excluded:
    - 'Play Store/TestFlight-equivalent publication, production keystore custody, store screenshots or outward beta distribution'
    - 'the 32-bit Samsung, tablets, wear, car surfaces, desktop or Linux'
    - 'root, unlock, reflash, factory reset, wiping the phone or changing unrelated apps/data'
    - 'moving the Exact pin merely for convenience; if required, isolate it in its own reviewed commit and re-prove iOS'
    - 'cloud ASR/TTS, server-side voice, or a hidden degraded Android mode'
    - 'rewriting the iOS app, web frontend, Choir desks, Autopaper pipeline or host architecture'
  protected_surfaces:
    - 'internal/auth app ceremony, allowed app ids and relying-party origins'
    - 'the Android assetlinks/passkey association contract and signing-certificate fingerprint'
    - 'Exact secret storage, account wipe and mail key/file lifecycle'
    - 'the deck journal, run-sheet posting and heard/settlement semantics'
    - 'microphone permission, audio focus/session, foreground/media service and local-model execution'
    - 'staging deploy and any Node B service or configuration touched for Android auth'
  evidence_and_heresy:
    - 'admissible evidence: frozen diffs and review receipts, whole touched-package suites, CI, staging auth/deploy proof, and rerunnable physical-device artifacts'
    - 'heresy delta: no new authority source, no cloud voice, no Android-only state authority, no feature hidden behind a switch, no production account drill, no claim that CoreML artifacts run on Android'
stations:
  - id: A0
    name: Physical baseline and parity inventory
    scope: 'reconcile the goal/registries and pre-existing worktrees; build current main as lab-one; install on the Motorola; drive signed-out paper/story and every reachable page; record each native call that substituted or refused; establish the versioned APK/proof harness'
    class: yellow
    review: none for the harness; one seat if shared behavior must change
    depends_on: []
    carrier: none yet (android-1-goal holds this goal only)
    status: working
  - id: A1
    name: Android passkey and account boundary
    scope: 'wire the existing Exact Android passkey package, assetlinks.json and accepted android:apk-key-hash origin for the lab-one signing certificate; preserve the fixed app-id contract; prove d@e sign-in, relaunch and sign-out on staging and the phone'
    class: red
    review: two seats on a frozen candidate
    depends_on: [A0]
    carrier: .lane/trees/android-auth
    status: working
  - id: A2
    name: Kotlin deck and background audio
    scope: 'implement the existing deckState/deckPlay/deckEnqueue/deckAct/deckAck contract with Media3, speech playback, media session, audio focus, interruption/route handling, durable journal and background playback; patch Exact only if the pinned Android host truly lacks the required media-service path'
    class: orange
    review: one seat on a frozen candidate
    depends_on: [A0]
    carrier: none yet
    status: pending
  - id: A3
    name: Android platform shell
    scope: 'create mobile/exact-package/android as an ExactExtension and implement only genuine platform seams: the declared web-page view, external links, file/audio hand-off, reader follow, podcast clip location and any remaining shipped native source/send found by A0; keep all decisions and parsing in shared Exact code'
    class: orange
    review: one seat on a frozen candidate
    depends_on: [A0]
    carrier: android-package (web-page and external links); android-web-proof (physical evidence)
    status: working
  - id: A4
    name: Android mail secrecy
    scope: 'implement the shared mailSeal/mailOpen contract with Android Keystore AES-GCM, authenticated record identity, tamper/wrong-account refusal, cache cleanup and sign-out wipe; never log plaintext or key material'
    class: red
    review: two seats on a frozen candidate
    depends_on: [A1, A3]
    carrier: none yet
    status: pending
  - id: A5
    name: Model portability matrix and old-phone benchmark
    scope: 'verify each model/runtime from primary sources; distinguish identical portable weights from Android substitutes for CoreML-only builds; extend the landed scripts/model-bench ASR harness rather than rewrite it; benchmark Silero/EOU, Fluid Speech Mini GGUF through parakeet.cpp, Qwen3 and EmbeddingGemma through llama.cpp, Tiny Voca/Phonon speech and WeSpeaker on fixed fixtures; select one complete configuration that fits and meets the existing speed/headroom thresholds'
    class: yellow
    review: none
    depends_on: [A0]
    carrier: none yet
    status: pending
  - id: A6
    name: Kotlin hands-free voice and model execution
    scope: 'implement microphone capture, audio session coordination, VAD, live/final ASR, local responder/decisions, TTS, speaker enrolment/check, embedding/search, takes recording and model lifecycle behind the existing shared contract; use the A5 configuration, with an honest on-device floor rather than cloud speech; prove a real-microphone turn and a staging desk turn'
    class: red
    review: two seats on a frozen candidate
    depends_on: [A2, A3, A5]
    carrier: none yet
    status: pending
  - id: A7
    name: Full parity proof and Autoradio 1 APK
    scope: 'rerun the complete iOS-parity matrix on the Motorola, repair failures in their owning stations, read back versionName 1.0 and artifact digests, and close the mission only when every acceptance row has named evidence'
    class: yellow
    review: none for proof; any behavior repair returns to its owning station and review tier
    depends_on: [A1, A2, A3, A4, A5, A6]
    carrier: none yet
    status: pending
decisions_for_the_owner:
  - id: D1
    question: 'Which Android distribution identity and signing key should eventually ship to the Play Store?'
    why: 'it changes the package certificate that assetlinks and passkeys trust'
    options: ['a dedicated release keystore under the owner/CI control', 'another managed signing arrangement']
    default: 'use news.choir.autoradio.lab.one with the existing debug certificate for this device mission; no store upload and no private key in the repo'
  - id: D2
    question: 'Where should the model benchmark be published beyond this public repository?'
    why: 'posting to a model hub or another site is an outward act'
    options: ['repository only', 'a model-hub dataset', 'a public Autoradio page', 'more than one']
    default: 'repository only under docs/evidence with a rerunnable harness; no outward post'
  - id: D3
    question: 'May the Motorola use the owner account rather than the drill account?'
    why: 'the phone is an ADB-driven test device and the safe default is isolation'
    options: ['d@e only', 'the owner account too']
    default: 'd@e only'
owner_rulings:
  - on: 2026-11-25
    decision: scope
    said: 'we have a choir platform... it should stay as is. we have web. same, stay. ios with exact (research exact). it should stay. just got to app store. what we need is android... need to get feature parity with exact ios for exact android.'
    read_as: 'reactivate and expand the existing Android mission to shipped-iOS parity; do not create a parallel Android mission or alter the other products'
  - on: 2026-11-22
    decision: privacy
    said: 'my budget and privacy and taste doesnt want me to send voice to other services. and i dont yet want the infra demand to process voice server side... client side voice is such an unlock, as it dramatically improves the value of old devices'
    read_as: 'no cloud or server speech fallback at any rung'
  - on: 2026-11-22
    decision: baseline
    said: 'we should think about the up-spec... the point now is to get the baseline, the framework, and to get it working for beta users, ios and android... what are the minimum system requirements and recommended settings'
    read_as: 'the old Motorola is the baseline; larger-phone models are a later ladder, not a reason to skip the baseline'
now:
  status: working
  slice: 'A1: Android passkey association, accepted origin and shared account boundary; local candidate only, staging and owner biometric action pending'
  source_ref: origin/main c7b1ed39e0e3ec688921ab56f48cba7cff8c0a78
  deploy_identity: staging proxy/platform 16f04df4d1dcda2b4a2777968c27926dfed485ba
  candidate:
    id: android-auth
    state: rehearsing
    ref: .lane/trees/android-auth
    base: origin/main c7b1ed39e0e3ec688921ab56f48cba7cff8c0a78
    digest: none
    scope: [internal/appids, internal/auth, internal/proxy, cmd/auth, nix/node-b.nix, mobile/AutoradioLab, mobile/AutoradioKit, mobile/exact-package, mobile/scripts/gen-exact-interface.py, scripts/autoradio-native-ceiling.txt, scripts/proofs, docs]
  conjecture:
    id: Android-parity-bridge
    calibration_claim: C254
    claim: 'The current shared Exact app plus a narrow Kotlin native layer can pass every required shipped-iOS journey on the Motorola without porting page logic, and one fully on-device voice configuration can fit this 3.4 GiB phone while meeting the existing responder, first-audio and recogniser speed thresholds.'
    test: 'A0 inventory, A1-A6 frozen candidates and reviews, A5 benchmark, and A7 same-fixture physical-device parity matrix; settle true only if every finish.acceptance row has its named evidence'
    edge: resource
    delta_o: 'per-stage memory/thermal traces on the real phone, exact model/runtime hashes, and a journey matrix that records substitutes rather than inferring parity'
    scope_if_supported: 'this Motorola/Android 14/arm64 low-end class and the named Exact commit; not all Android devices or model qualities'
    status: proposed
    evidence_refs: []
  decision:
    what: 'reactivate the existing Android mission as an Exact-iOS parity mission on the connected Motorola'
    kind: purpose
    status: settled
    evidence_ref: owner statement of 2026-11-25
    owner_ratification_ref: owner statement of 2026-11-25
  belief:
    believed_state: 'Exact already supplies the shared Android host, Media3 background service and passkey package; the missing work is the package-first Kotlin ExactExtension, Android auth association, and verified model runtimes'
    main_uncertainty: 'how much of the current train is alive on the physical phone, whether background media needs an Exact patch, and which local model configuration fits and performs acceptably'
    next_observation: 'A1 two-seat frozen review, staging assetlinks/origin proof, then d@e passkey sign-in, relaunch and sign-out on ZY22H5K8QR with the owner completing the system biometric sheet'
  blocker_or_risk: 'the system passkey/biometric prompt may not be drivable by adb and the Motorola had no enrolled fingerprint/face at the A0 read; A1 is locally built but not deployed or device-proved; CoreML model bundles are not Android artifacts; Exact Android background media and model fitness are unproved; only one native build may run at a time'
  next_action: 'freeze and review the android-auth candidate with two seats; after the lead lands and deploys it, run scripts/proofs/android-a1-account-boundary.py in qa-mac with the owner present for the biometric action'
weak_measures:
  - name: required native calls answering a substitute
    kind: gate
    baseline: unknown until A0
    desired: zero on every required parity journey
    decision_use: 'orders A2/A3/A6 and proves no page is being silently hidden'
    cannot_prove: 'audio quality, background survival or account correctness'
  - name: selected voice configuration memory and speed
    kind: telemetry
    baseline: unknown until A5
    desired: 'at least the existing 500 MB post-load headroom; recogniser RTF <= 0.5; responder >= 15 tokens/s; first audio <= 1500 ms on fixed fixtures'
    decision_use: 'selects the model/runtime ladder for this phone'
    cannot_prove: 'every Android device, noisy-microphone accuracy or long-run thermal behavior'
  - name: parity rows with physical-device evidence
    kind: gate
    baseline: zero
    desired: every required row
    decision_use: 'A7 completion decision'
    cannot_prove: 'anything beyond the named fixture and journey'
receipts: []
---

# Autoradio on Android — parity with the shipped Exact iOS app

This is the existing Android mission, reactivated and expanded by the owner on
2026-11-25. Update this file in place; do not author a second Android goal.
The 2026-11-23 deferral is lifted. The old A0-to-A8 probe scope is superseded by
the stations above.

## What parity means

Parity is the shipped iOS user journey, not visual similarity. A0 writes the
matrix from the current iOS build and A7 fills its Android column with evidence.
The checked inventory lives in
[`docs/reference/AUTORADIO-ANDROID-PARITY.md`](../reference/AUTORADIO-ANDROID-PARITY.md).
At minimum it covers signed-out paper/radio, passkey sign-in and account wipe,
paper/story/reader, queue/run sheet, podcasts and clips, files and imported
documents, mail, Textures/editor, notebook, takes, themes, onboarding, Voice and
models, activity/about, account deletion, background radio, lock/headset
controls and hands-free voice.

A row passes only when the same shared source and contract produce the observed
result on the phone. A Kotlin implementation of page logic, a substitute value,
a hidden control or an emulator does not pass. If a system sheet requires the
owner's biometric/device credential, do everything else and record the exact
smallest owner action; the mission remains incomplete until that row is proved.

## Model rule: same weights where possible, measured substitutes where necessary

Core ML, Core AI and Neural Engine artifacts do not run on Android merely by
recompiling the app. For every job, A5 records one of:

1. **same weights, Android build** — the original model is portable (for example
   a GGUF/ONNX artifact) and only the runtime/JNI differs;
2. **same model family, different conversion** — the Android artifact is a new
   ONNX/GGUF/quantised build with its own hash and measured quality; or
3. **substitute** — no credible Android build of the iOS model exists, so an
   Android-native model fills the same job and is named honestly in the UI and
   benchmark.

The researched starting stack is Silero v6.2.1 ONNX, Parakeet EOU 120M INT8
ONNX, Fluid Speech Mini GGUF through `parakeet.cpp`, Qwen3 GGUF and
EmbeddingGemma 2 text GGUF through llama.cpp, Tiny Voca's GGUF+mmproj (or ONNX)
for speech, and WeSpeaker ResNet34-LM ONNX through sherpa-onnx. Qwen3 Q4_K_M is
the first low-memory rung; Q8_0 remains the exact-iOS-parity option. Phonon
31M Q4K is the temporary speech substitute if Tiny Voca fails its device spike;
Parakeet TDT v3 INT8 is the final-ASR substitute if Mini fails. These are
research leads, not device results. A5 must pin artifacts and verify current
primary sources, licenses, ABI/API requirements and real Android support before
choosing. A vendor Mac/iPhone number never enters the device result column.

## Build and proof discipline

- Use `ANDROID_SERIAL=ZY22H5K8QR` and the explicit adb path. Build
  `news.choir.autoradio.lab.one`; leave the older `.lab` install alone.
- Set `JAVA_HOME=/opt/homebrew/opt/openjdk@21/libexec/openjdk.jdk/Contents/Home`,
  `ANDROID_HOME=/Users/wiz/Library/Android/sdk`, Bun 1.4.2 and the Android
  platform-tools on `PATH` for Android commands.
- The lead alone runs `scripts/gc-build-output`, checks the build queue/lock and
  performs native/device builds in `qa-mac`. Builders work in their own lanes
  and may run only the cheap shared checks allowed by CLAUDE.md.
- Preserve the Exact pin unless a measured Android blocker requires a move. A
  pin move is a separate reviewed commit with an iOS rebuild/proof.
- Prefer Exact's Android agent and testIds. Use adb for package/install/readback,
  screen power and system UI; record every coordinate-based fallback and why
  Exact could not address it.
- A fixed spoken fixture played from the Mac near the phone is acceptable for
  the overnight real-microphone proof. PCM injected after the microphone is not.

## Landing and stop rules

Each behavior station lands separately with its class, review, rollback and
`Ring-Expedite`. Red stations get their own frozen review and deployed proof;
a send-back is repaired and re-reviewed, never re-run unchanged. Every landing
updates `docs/changes.md`; a status change updates `current-architecture.md` and
`system-map.md` in the same change. Update this goal, `docs/ACTIVE.md` and
`docs/mission-graph.yaml` when a station changes state.

Do not stop at a checkpoint, a passing local suite, an APK or a polished
benchmark. Continue until every acceptance row passes or a named blocker remains
after real probes. Never bypass auth, use the owner's account, send audio to a
cloud service, wipe the phone or publish to a store to make the mission look
complete.

## Required final report

Name the pushed SHA(s), CI and deploy status, staging commit, APK/package and
signing/artifact digests, every proof command and result, accepted run ids, each
station's mutation class and protected surfaces, every review receipt and
expedite, the selected model configuration and measured numbers, what was
introduced/discovered/repaired, every residual risk, the rollback, and the next
owner action if any. If incomplete, say exactly which acceptance row remains and
the smallest safe probe that can settle it.
