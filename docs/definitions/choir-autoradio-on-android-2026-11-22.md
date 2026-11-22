---
definition_version: 7
readiness: executable
authority: 'the owner, 2026-11-22: "the android phone is quite old so it would be limited in the local models it can run, but its a good benchmark for that reason. lets try to get autoradio running on it... thats a new mission to add to the metamission".'
review:
  reviewer: none (the owner's statement is the authority; each station names its own review tier)
  frozen_ref: none
  verdict: none
start:
  captured_at: 2026-11-22T22:24Z
  source:
    canonical_ref: origin/main 5a0ce4ebe
    deploy_identity: 'staging and the paper on 5a0ce4ebe; the phone app''s Exact train (lane/exact-train1, 50ceb7041) is built and not landed'
  what_is_true_now:
    - 'Two Android phones are plugged into the lead''s Mac and answer adb with debugging authorised. A Motorola moto g stylus 5G (2023), serial ZY22H5K8QR: Android 14, Snapdragon 6 Gen 1, arm64-v8a, about 3.4 GiB of memory, 70 GiB free; its touch screen does not register touches. A Samsung Galaxy A13 (SM-A135U1), serial R58T33LPWWH: Android 14, Exynos 850, a 32-bit system (zygote32, no 64-bit ABI list), about 2.6 GiB of memory, 4.7 GiB free, bootloader locked.'
    - 'Exact builds an Android app for aarch64 and x86_64 only (host/android/README.md in the pinned checkout: "arm64-v8a by default"). So the Samsung cannot load the app as Exact stands.'
    - 'The app has been built for Android once, on an emulator, and driven to the paper page (the build queue''s android mode, 2026-11-20, the themes work). It has never run on a physical Android phone.'
    - 'Everything native is Swift: the module, the voice, the on-device models, the audio session. mobile/exact-package has an apple implementation and no android one; mobile/AutoradioLab/modules has apple only. On Android a native call answers its substitute or is not supported.'
    - 'About 5,300 lines of non-voice Swift remain on the train plus its dead-code lane; each page still in Swift is a page Android does not have.'
    - 'unknown: which shared pages work on Android today without the native module; whether Exact''s audio plays our radio and podcasts on Android; whether Exact''s secret store (Keystore files) carries the sign-in there; which speech and language models fit in 3.4 GiB and how fast they run.'
  branches_in_flight:
    - 'lane/exact-train1 and lane/native-dead-code (M1): the base an Android build should be made from.'
    - 'lane/account-partition-s1 (M1): sign-in in shared code on Exact''s secret store; Android sign-in depends on it.'
finish:
  deliver: 'Autoradio runs on the Motorola: it installs from the shared codebase, signs in, shows the paper, plays the radio and podcasts, and a measured table says which on-device models an old phone can run and how fast.'
  artifact: 'on main: an android implementation of the package holding only what Android needs natively, the build queue''s android mode aimed at a physical phone, a rerunnable drive on the Motorola with its trees and screenshots, and docs/evidence with the model benchmark on that phone.'
  acceptance:
    - action: build from main and install on the Motorola, then drive it by Exact's agent to the paper page and a story
      proves: the shared pages run on a physical Android phone with no hand edits
      evidence_class: device drive, rerunnable
    - action: sign in on the Motorola as d@e through the device flow, relaunch, sign out
      proves: the account boundary holds on Android (the key in Exact's store, the account's data removed at sign-out)
      evidence_class: device drive
    - action: play a radio story and a podcast episode for a minute each with the screen off
      proves: audio plays and keeps playing in the background on Android
      evidence_class: device drive with the audio position read back
    - action: load each candidate speech and language model on the Motorola and time it on fixed inputs
      proves: which models load in 3.4 GiB, their real-time factor and their peak memory
      evidence_class: measured table, rerunnable
  rollback: each station is a commit that reverts; nothing on Android ships to anyone but the two named phones.
  landing:
    required: true
    environment: the Motorola, against staging
    required_receipts: [pushed_commit, ci, device_drive]
value:
  better_means: 'the share of Autoradio that runs on Android with no Android-only code rises, and the cost of the voice on a cheap phone is known from measurement'
  goodharting_would_be: 'a build that installs and shows one page; an Android copy of page logic in Kotlin; a benchmark of models nobody would ship; a demo on the emulator called a phone; features hidden on Android so the drive passes'
homotopy:
  realism_axis: 'from signed-out pages, to signed-in pages, to audio, to voice; the same shared code and contracts as the iPhone at each step, and a native part only where voice or the platform leaves no other way'
boundaries:
  mutation_class: orange for the shared code and scripts; red where sign-in and the account boundary are touched
  authority_sources:
    - 'the owner''s words above; CLAUDE.md, "The phone app is Exact; only voice is native"'
  owner_rulings:
    - '"only the voice stuff should be swift. the rest should be exact" (2026-11-18). Reading: on Android only the voice is Kotlin; a page that needs Kotlin to work is a page not yet moved to Exact, and the fix is in M1.'
  must_preserve:
    - 'no page logic in Kotlin; a native call needs a reason only native code can meet, written beside it'
    - 'one native build at a time through the Mac build lock; builders never run native builds, emulators or a phone'
    - 'the phones are the owner''s: nothing is wiped, rooted or reflashed; only our own app is installed and removed'
    - 'sign-in drills use d@e; no secrets in the repo or output'
    - 'no switches: Android gets the same features or an honest "not on this phone yet", never a hidden one'
  excluded:
    - 'a release to a store or to testers'
    - 'a 32-bit build (the Samsung), unless the owner rules otherwise (Q1)'
    - 'tablets, wear and car surfaces'
stations:
  - id: A0
    name: The app on the Motorola, signed out
    scope: 'the android mode of the build queue aimed at a physical phone by serial; build from the train, install, drive to the paper page and one story; every page listed as works, substitute or broken'
    class: yellow (scripts) and orange where shared code changes
    review: one seat for shared code
    depends_on: []
    status: working
  - id: A1
    name: What Android lacks, page by page
    scope: 'from A0''s list: each page that fails for want of the native module is either already queued in M1 (a Swift page to move to Exact) or a new station there; this mission adds no Kotlin for it'
    class: green
    review: none
    depends_on: [A0]
    status: pending
  - id: A2
    name: Sign-in and the account boundary on Android
    scope: 'the device flow, the key in Exact''s store, sign-out and the launch sweep, drilled on the Motorola as on the simulator'
    class: red
    review: two seats
    depends_on: [A0, 'M1: the account boundary landed']
    status: pending
  - id: A3
    name: Audio on Android
    scope: 'the radio and podcasts through Exact''s audio, in the background, with the lock-screen controls; what needs a native audio session says why'
    class: orange
    review: one seat
    depends_on: [A0]
    status: pending
  - id: A4
    name: The model benchmark on an old phone
    scope: 'a harness that loads each candidate speech-to-text, text-to-speech and language model on the Motorola and records load time, real-time factor, peak memory and heat on fixed inputs; a table in docs/evidence; read docs/reference for each model first'
    class: yellow
    review: none
    depends_on: []
    status: pending
  - id: A5
    name: The voice in Kotlin
    scope: 'design first, from A4''s table: the audio session, the microphone, speech and the models behind the same contract names the Swift voice answers; then its stations'
    class: red
    review: a panel on the design; two seats on the code
    depends_on: [A3, A4]
    status: pending
decisions_for_the_owner:
  - id: Q1
    question: Should the 32-bit Samsung be supported?
    why: 'it needs a 32-bit ARM target added to Exact itself, and its 2.6 GiB of memory and 4.7 GiB of free storage leave little room for models'
    options: [not pursued, add a 32-bit target to Exact]
    default: not pursued
  - id: Q2
    question: May the Motorola be signed in with the owner's own account, or only d@e?
    why: the phone's touch is broken, so it is driven by adb and is a test device in practice
    options: [d@e only, the owner's account too]
    default: d@e only
now:
  status: working
  as_of: 2026-11-22T22:24Z
  slice: 'A0: a first build from the train onto the Motorola'
  source_ref: origin/main 5a0ce4ebe
  deploy_identity: staging on 5a0ce4ebe
  candidate: none frozen
  conjecture:
    claim: 'the Exact train builds for Android and, installed on the Motorola, is driven signed out to the paper page showing at least one story, with no change to shared code'
    test: the queue's android build and Exact's agent tree and screenshot from the phone
    edge: missing_oracle
    edge_note: 'the emulator drive of 2026-11-20 is the only earlier evidence and predates most of the train; a page can render and still be dead where it calls the native module'
    observer_upgrade: the page-by-page list of A0, which names each native call that answered its substitute
    scope_if_supported: A3 and A4 may start on the same build
    ledger_id: C198
    status: untested
  believed_state: 'the phones are reachable; the app has never run on a physical Android phone'
  main_uncertainty: 'how much of the app is alive on Android without a native module'
  next_observation: 'the first build and drive on the Motorola'
  blocker: none
  next_action: 'aim the android mode at the Motorola''s serial and run it on the train'
operating_notes:
  - 'The Motorola''s touch is broken: everything is driven by adb and Exact''s agent.'
  - 'The Mac build lock covers Android builds too; the simulator is shut down first.'
receipts:
  - 'docs/reference/EXACT.md; the pinned Exact checkout''s host/android/README.md'
---

# Autoradio on Android

The owner plugged in two old Android phones on 2026-11-22. The Motorola can
take the Exact build and is the low-end benchmark. The Samsung runs a 32-bit
system, which Exact does not build for, and is left out unless the owner
rules otherwise. The rule from the iPhone holds: only the voice is native.
What Android lacks beyond the voice is a measure of what is still Swift.
