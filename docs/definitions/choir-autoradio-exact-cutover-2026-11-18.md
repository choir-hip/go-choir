---
definition_version: 7

readiness: executable

review:
  reviewer: none
  frozen_ref: none
  verdict: none
  evidence_ref: "none yet; the owner's words of 2026-11-18 are the authority to start (owner_rulings R1 to R4). Station X0 puts the design and this file in front of two seats before any red station starts."

start:
  captured_at: 2026-11-18T12:45Z
  source:
    canonical_ref: origin/main b3ed207de
    deploy_identity: staging deployed_commit ce719fe1a
  worktrees:
    - path: /Users/wiz/Dev/go-choir
      status: dirty
      class: user_wip
      owner: owner
      touch: forbidden
      recovery: leave in place (one untracked env backup that holds secrets)
    - path: /Users/wiz/Dev/go-choir/.lane/trees/qa-mac
      status: clean
      class: goal_candidate
      owner: lead
      touch: goal_owned
      recovery: "the one build tree, on local branch android-probe 572e6f203 (lane/autoradio-integ20 plus app.json naming Android), pushed as lane/android-probe; holds the one Exact checkout (mobile/.exact-src at de212e671 with our patches)"
    - path: lanes exact-loop, exact-paper
      status: unknown
      class: other_agent_wip
      owner: builders
      touch: read_only
      recovery: each lane is its own branch on origin, based on lane/autoradio-integ20
    - path: lanes audio-held, start-gap, learn-notebook
      status: unknown
      class: other_agent_wip
      owner: builders
      touch: read_only
      recovery: each lane is its own branch on origin; they carry work from the learning-loop mission (stations K1 and K2 here)
  known:
    - "The phone app's pages are Exact contract files (50 files, 7,742 lines). Its logic is Swift: 157 files and 38,149 lines in mobile/AutoradioKit/Sources, reached through 181 native calls bound by 22 Swift files (mobile/exact-package). About a quarter of the Swift is voice. The app's TypeScript is one file of 11 lines. The package has no Android side. Record: docs/problems/autoradio-app-logic-is-swift-not-exact-2026-11-18.md."
    - "None of the phone work is on main. It lives on lanes; the owner's iPhone runs lane/autoradio-integ20 8ff44115f, installed 2026-11-18 06:39 local. On that build, recording a take breaks the radio until the app is reopened."
    - "Exact's intended loop (bun exact.mjs web, a native client following it) has never been used on this app. Measured today in the build tree: a one-line page edit takes 57 to 90 s in that loop; the Contract compiler takes 64 to 66 s on our pages and 16 s with the Tetramark's drawing stubbed out; the mark is 4.6 MB of a 5.9 MB plan, compiled once for each of its 29 uses. Exact's figure for a fresh app is about 0.1 s."
    - "The Android toolchain works on this Mac. Exact's Shelf sample built (6 min 33 s), installed on an emulator and drew correctly. Autoradio built for Android unchanged (260 s) and draws its shell, bar, mark and tray; under the driver all 114 source reads are substituted and a send fails with NotSupportedError. No physical Android phone is reachable."
    - "Read from Exact's source, not run: shared code keeps running on a locked iPhone while our audio plays; on Android the page freezes when the app leaves the screen unless a foreground-service claim is held, and Exact has no media-playback service kind; whether Exact's iOS files carry the file protection our mail store sets is not stated."
    - "Our Exact reference pages were corrected against the source today (docs/reference/EXACT.md, 'How Exact means an app to be built'). The design for the move is docs/design/autoradio-in-exact-2026-11-18.md: proposed, read from source, reviewed by no one."
    - "The Go server has the run sheet reducer and shared fixtures (mobile/fixtures/runsheet, mobile/fixtures/takes) that two implementations already satisfy. Bun is in neither the flake nor CI."

finish:
  deliver: "Autoradio is one Exact codebase that runs on iPhone and Android. Native code is the voice and nothing else. A change to a page or to logic is written once and seen in seconds."
  artifact: "mobile/AutoradioLab holds the pages (contract) and all logic (TypeScript modules with their tests); mobile/AutoradioKit and a new Kotlin package hold only the audio session, the microphone, speech, the on-device models, the take recorder and the deck (a queue of prepared audio that reports what started and ended); the same commit builds an iPhone app and an Android app; all of it is on main."
  acceptance:
    - action: "scripts/check-autoradio-native-boundary on main (a gate this mission builds): it lists every Swift and Kotlin source under mobile/ by area, and every native interface name"
      proves: "no native source or interface name exists outside the voice list (audio session, engine, microphone, speech in and out, models, take recorder, deck); the count outside it is zero and cannot rise"
      evidence_class: repo gate in CI
    - action: "on the Android emulator, signed in to the drill computer: open the app, press Play, hear nothing by rule (silent proof) but read the driver's tree and the server: a stretch from staging is placed, three entries air with the system voice, one is skipped; then read the scorecard route"
      proves: "the shared logic alone runs the radio on Android against the real server, and the listener's ops settle forecasts there as they do from the iPhone"
      evidence_class: deployed proof with a rerunnable script
    - action: "the same commit on the owner's iPhone for one ordinary day, including a locked hour, a take, a podcast clip, a file read aloud and mail; then the owner's verdict"
      proves: "the iPhone app is no worse than the Swift one for the person who uses it daily"
      evidence_class: the owner's verdict, and the phone's console log for that day
    - action: "in the build tree: time a one-line edit to a page and a one-line edit to a logic module in the web loop, three runs each; time the logic test suite"
      proves: "a page or logic edit is seen in 5 s or less and the logic tests run in 30 s or less, with no native build"
      evidence_class: command wall time read by the lead
    - action: "go test ./internal/runsheet/... and bun test in mobile/AutoradioLab, both reading mobile/fixtures"
      proves: "the run sheet has two implementations held to one set of fixtures (Go and TypeScript); the Swift copy is gone"
      evidence_class: CI
  rollback: "each station is one landing on main and reverts as one; the phone rolls back by installing the build before that station (lane/autoradio-integ20 until station K1 replaces it, then each station's predecessor). Stations marked irreversible below say what does not come back."
  landing:
    required: true
    environment: "main for the source; the owner's iPhone and the Android emulator for the apps; staging (choir.news) for the server the apps talk to"
    required_receipts: [pushed_commit, ci, device_install, deployed_acceptance]

value:
  better_means: "Fewer native lines and names outside the voice list, and a shorter time from an edit to seeing it, while everything the iPhone app does today keeps working for the owner: the tours pass, the product rules held by the Swift tests are held by shared tests, and a locked phone still plays and still reports what was heard."
  goodharting_would_be: "Calling logic 'voice' to keep it native, or one native call that decides everything behind a thin TypeScript wrapper. Deleting a feature to delete its Swift. TypeScript that only works on iPhone because it leans on native calls. Web tests passing on placeholder data while a phone is broken. A fast loop reached by removing the mark or the pages a listener sees. Android 'working' only under the driver with substituted data. Counting lines moved and not behaviour kept."

homotopy:
  realism_axis: "How much of the real app runs on shared logic, on how real a device: from one page (the paper) answered by TypeScript on the web host with a canned fetch, to every page and the radio's decisions on both phones against staging, screen locked for an hour, in the owner's daily use. Every point is the same app and the same server; a station moves one area fully and deletes its Swift, so no point is a second app or a switched path."

boundaries:
  mutation_class: red
  authority_sources:
    - "the owner's words of 2026-11-18 (owner_rulings)"
    - "CLAUDE.md, Product: the phone app is Exact; only voice is native"
    - "docs/choir-doctrine.md and docs/learning-engine.md"
    - "Exact's source at the pinned commit, over any of our pages about it"
    - "docs/design/autoradio-in-exact-2026-11-18.md, a proposal until station X0 reviews it"
  must_preserve:
    - "no switches: a station replaces the Swift for its area and deletes it in the same change; nothing selects the old or the new path"
    - "the owner's iPhone app does not get worse; Swift repairs for daily use are allowed until that page moves"
    - "only the listener's phone op settles a forecast; an entry that made no sound is never reported heard; a late or lost report is no verdict"
    - "mail and local-only files never go to the server desk; the mail store on the phone is encrypted at rest and wiped at sign-out; sign-out leaves nothing of the account"
    - "logs carry ids and counts, never words spoken, titles or mail"
    - "one voice at a time; Autoradio does not take the microphone or the audio session from another app without the listener's press"
    - "no limit on our own model calls; the phone asks the desk as rarely as the job allows"
    - "one fixture set per wire, written by the server side and read by every implementation"
    - "build output in one place: native builds, simulators and emulators run only in the lead's build tree; a lane runs the Exact loop that needs no native build"
  excluded:
    - "new product features while a page is being moved, including takes from voice mode (the owner's idea of 2026-11-18, held until the owner says more)"
    - "on-device models on Android; the first Android voice is the system's"
    - "TestFlight and the Play Store (TestFlight stays with the learning-loop file's station L9 and the owner's paid team)"
    - "changes to the server's routes beyond what a shared client needs to read them the same way"
    - "an upstream pull request to Exact without the owner's word"
  protected_surfaces:
    - "auth and session: sign-in, the key and where it is kept, sign-out wipe (station X3)"
    - "run acceptance and evidence: the ops the phone posts that settle forecasts (stations X4, X5)"
    - "the mail store and its encryption (station X8)"

stations:
  - id: X0
    name: Baseline and the four tests
    scope: "the Android toolchain and the app's baseline on the emulator (done); the design and this file read by two seats; the tests the design names: shared code and events on a locked iPhone (needs the owner's phone), file protection of Exact's files on iPhone, a server stream with our headers from the emulator, whether a stream can be a POST"
    class: yellow
    review: two seats on the design
    depends_on: []
    carrier: lane/android-probe; docs/design/autoradio-in-exact-2026-11-18.md
    status: working
  - id: X1
    name: The loop is fast
    scope: "the Contract compile of our pages falls from 64 s to 15 s or less with the mark looking the same; what the rest of the time is spent on is known; the Android refusals and Apple-only symbols are fixed or listed; a proposal for the lab pages leaving the product app"
    class: orange
    review: one seat
    depends_on: []
    carrier: lane/exact-loop
    status: working
  - id: X2
    name: The paper page, as the pattern
    scope: "the paper page's data is answered by TypeScript (public routes, no sign-in); its Swift and interface names are deleted in the same change; a shared fixture, bun tests and contract tests; the pattern written into docs/reference/exact/native-modules.md; real stories on the Android emulator"
    class: orange
    review: one seat
    depends_on: []
    carrier: lane/exact-paper
    status: working
  - id: XG
    name: The boundary gate and Bun in CI
    scope: "scripts/check-autoradio-native-boundary: native lines and interface names by area, with a recorded ceiling that only falls; pinned Bun 1.4.2 in the flake and a CI step that runs the logic tests and the shared fixtures; mobile/ changes land on main through scripts/land like everything else"
    class: yellow
    review: none
    depends_on: [K3]
    carrier: none yet
    status: pending
  - id: X3
    name: The server client and sign-in
    scope: "one TypeScript client (headers, back-off, deadlines); device-flow sign-in; the key in Exact's secret store; computer discovery; sign-out wipe of everything shared code holds; account and computer settings; native code is handed the identity by one call and keeps it in memory only. Deletes ChoirClient, ChoirSignIn, Accounts, ProofKey, most of AccountWipe. The owner signs in once more."
    class: red
    review: two seats
    depends_on: [X0, X2]
    carrier: none yet
    status: pending
  - id: X4
    name: The run sheet and the radio's decisions
    scope: "the reducer in TypeScript on the shared fixtures, the store, the stream, the writer that asks the desk, the outbox of ops, the on-air decisions; the deck interface with the existing Swift engine behind it on iPhone and a Kotlin deck (system voice, Media3) on Android. Deletes the Swift run sheet, the decision half of the player, the radio, bar and queue page models. Android plays the radio on screen."
    class: red
    review: two seats
    depends_on: [X3]
    carrier: none yet
    status: pending
  - id: X5
    name: Background play
    scope: "the deck plays on with the screen locked and journals what ended; shared code posts the ops when it next runs; on Android a media-playback service kind carried as a patch to Exact (mobile/exact-patches), with a media session. A locked hour on both, ops on staging matching what was heard."
    class: red
    review: two seats
    depends_on: [X4]
    carrier: none yet
    status: pending
  - id: X6
    name: Podcasts and clips
    scope: "feeds, subscriptions, downloads and the clip store in TypeScript; play goes through the deck"
    class: orange
    review: one seat
    depends_on: [X4]
    carrier: none yet
    status: pending
  - id: X7
    name: Files, the reader and books
    scope: "the files page, viewers, the shelf, EPUB in TypeScript; PDF text stays a native call until a shared way exists that keeps local files on the phone (decision D4)"
    class: orange
    review: one seat
    depends_on: [X3]
    carrier: none yet
    status: pending
  - id: X8
    name: Mail
    scope: "the mail pages and store in shared code, encrypted at rest by whatever station X0's file-protection test allows (Exact's files if protected, else a small native seal with the key in the Keychain or Keystore); wiped at sign-out. Irreversible on the phone: the old store is deleted and mail is fetched again."
    class: red
    review: two seats
    depends_on: [X0, X3]
    carrier: none yet
    status: pending
  - id: X9
    name: Content, search, takes, the notebook pages
    scope: "the content store and ranking in SQLite; the takes list and publish (multipart built as bytes); the notebook and learning pages; embeddings stay native with the models"
    class: orange
    review: one seat
    depends_on: [X3]
    carrier: none yet
    status: pending
  - id: X10
    name: Settings, onboarding, themes, cards, the bar
    scope: "the remaining page models and demo data as shared code and JSON; the lab pages leave the product app"
    class: orange
    review: one seat
    depends_on: [X4]
    carrier: none yet
    status: pending
  - id: X11
    name: Only voice is native
    scope: "what remains in mobile/AutoradioKit is the voice list; the gate's ceiling is zero outside it; the Swift tests that carried product rules exist as shared tests; current-architecture.md says what is built"
    class: orange
    review: one seat
    depends_on: [X5, X6, X7, X8, X9, X10]
    carrier: none yet
    status: pending
  - id: K1
    name: The owner's iPhone keeps working (carried from the learning loop)
    scope: "the reviewed repairs for the first listening session's faults: audio lost after a take (lane/audio-held) and the slow start and silence after the opening (lane/start-gap), built as one candidate on top of integ20 and installed when the owner's phone is back"
    class: red
    review: two seats, then seat 2 again after each blocking finding
    depends_on: []
    carrier: lane/audio-held 9d768912c; lane/start-gap c1a373fa6
    status: working
  - id: K2
    name: The notebook lands (carried from the learning loop, stations L4 and L5)
    scope: "lane/learn-notebook: the server writes and reads the listener's notebook; server code only, untouched by this cutover"
    class: red
    review: two seats
    depends_on: []
    carrier: lane/learn-notebook 4279998bd
    status: working
  - id: K3
    name: The phone work is on main
    scope: "the phone lanes (about 150 commits, through the K1 candidate) merged into main in one reviewed landing, so every later station lands on main and is reviewed as a landing"
    class: orange
    review: one seat on the merge; the parts were reviewed in lanes on 2026-11-18
    depends_on: [K1]
    carrier: none yet
    status: pending

decisions_for_the_owner:
  - id: D1
    question: "Is the order right: the paper page, then sign-in and the client, then the run sheet and the radio, then background play, then the other pages in parallel?"
    why: it decides when Android first shows real data (after X2), when it first plays (after X4) and how long the owner's iPhone carries two kinds of code
    options: as stated; the radio first with the system voice and sign-in after; sign-in first
    default: as stated
  - id: D2
    question: May we carry a patch to Exact that adds a media-playback service on Android?
    why: without it the radio stops when the Android app leaves the screen; with it we hold a fork until Exact takes it
    options: carry it in mobile/exact-patches and tell Exact's authors; carry it silently; no background play on Android for now
    default: carry it in mobile/exact-patches; nothing is sent upstream without the owner
  - id: D3
    question: The owner signs in again once when sign-in moves, and mail is fetched again once when mail moves. Acceptable?
    why: the key and the mail store change where and how they are kept; the old copies are deleted
    options: yes; migrate in place at the cost of a native reader for the old format
    default: yes, each announced the day before
  - id: D4
    question: Where is the text of a local PDF read?
    why: Exact has no PDF text reader; local files must not go to the server desk
    options: keep one native call per platform for PDF text; a JavaScript PDF library in shared code; published documents only, by the server
    default: keep the native call until a shared reader is measured on a phone
  - id: D5
    question: Do the lab pages (the voice lab, the motion gallery) leave the product app?
    why: they are part of what makes the pages slow to compile and they ship to listeners
    options: a second small app in the repo that imports the same components; delete them; keep them
    default: a second app; the product app has no lab pages
  - id: D6
    question: If the Tetramark cannot be made cheap to compile as pure page math, may its form change?
    why: it is 50 of the compiler's 66 seconds today
    options: any form that looks the same on iPhone, Android and web; keep pure page math and accept the cost
    default: any form that looks the same; the look is held fixed by screenshots
  - id: D7
    question: What is the first Android voice?
    why: the old test phone cannot run on-device models and may not be reachable at all
    options: the system voice; nothing until models run
    default: the system voice
  - id: D8
    question: Does TestFlight wait for the cutover?
    why: a TestFlight build this week would be the Swift app
    options: ship the Swift app to testers now and replace it; wait for station X5
    default: wait; the owner's own phone gets every station as it lands

owner_rulings:
  - on: 2026-11-18
    decision: R1
    said: "what do you mean all of the apps logic is a swift package? only the voice stuff should be swift. the rest should be exact..."
  - on: 2026-11-18
    decision: R2
    said: "omg, we need to rewrite it lol. i didnt realize we were skipping exact. now i know why the build times are so slow lol. wow, this is annoying. well, all the more reason to work on the android version. IN EXACT. the whole point is the codebase is shared, using EXACT"
  - on: 2026-11-18
    decision: R3
    said: "Ok I guess that old phone won't work...not sure why, it did last week. It's plugged in with usb debugging. Anyway, we can work in android simulator. And much more importantly we need to rewrite the app in exact. That should make it actually cross platform and improve build times a lot"
    read_as: "the emulator is the Android device for now; the rewrite outranks the other phone work"
  - on: 2026-11-18
    decision: R4
    said: "Make sure we're using exact correctly, because using it with TypeScript should lead to fast builds. That's one of the things that they advertise. Make sure you're reviewing the exact documentation. Make sure that our own exact.md is correct. It must not have been correct because we've been misusing it or not using it the whole time."
    read_as: "Exact's own documents are the authority on how to build with it; our pages are corrected to them; fast edits are part of the finish line, not a hoped-for side effect"
  - on: 2026-11-18
    decision: R5
    said: "Let's make a new goal file for this cutover from swift to exact"
    read_as: "this file; the learning-loop file is a checkpoint, with its open stations carried here as K1 to K3"

now:
  status: working
  slice: "X1 (the loop is fast) and X2 (the paper page) are being built in lanes; X0's device tests wait for the owner's phone; K1 and K2 are in review rounds"
  source_ref: "origin/main at the commit that carries this file"
  deploy_identity: "staging ce719fe1a; the owner's iPhone on lane/autoradio-integ20 8ff44115f"
  candidate:
    id: none
    state: none
    ref: none
    base: none
    digest: none
    scope: []
  conjecture:
    id: E1
    calibration_claim: C127
    claim: "If every page's data and decisions are shared TypeScript and native code is the voice and a deck, then one change runs on both phones and is seen in seconds, and the iPhone app is no worse for its daily listener."
    test: "the boundary gate at zero; the Android acceptance script against staging; the measured loop time; the owner's verdict after a day on the cut-over build"
    edge: missing_oracle
    delta_o: "a locked phone is the oracle we lack: whether shared code hears the deck's events and posts ops with the screen off is read from Exact's source and has not been run. The smallest upgrade is the thirty-minute test on the owner's iPhone (station X0), before station X4 is designed in detail. The dangerous edge is frame lock: 'shared code keeps running' may be true and still too coarse (a turn budget of 100 ms, no timers outside an answer), so the deck must be able to play on alone."
    scope_if_supported: "this app, iPhone and Android, at the pinned Exact commit"
    status: active
    evidence_refs: []
  decision:
    what: "native code is the voice and a deck; everything that decides is shared; stations replace and delete, in the order of decision D1; the phone work lands on main first (K3) so stations are landings"
    kind: architecture
    status: proposal
    evidence_ref: docs/design/autoradio-in-exact-2026-11-18.md
    owner_ratification_ref: "the rule is the owner's (R1, R2); the deck and the order are the lead's proposal until the owner rules on D1 and station X0's review returns"
  belief:
    believed_state: "the pages already run on Android and the web with nothing behind them; the first page is being moved; the app's own Exact loop is a minute per edit and most of that is one component; the iPhone app on the owner's phone still loses audio after a take"
    main_uncertainty: "what shared code may do while the phone is locked, on each platform, and so how much the deck must do alone"
    next_observation: "the paper lane's report: whether a page can read a TypeScript source while the package answers the rest; then the locked-phone test"
  blocker_or_risk: "the owner's iPhone is away, so nothing about sound or a locked screen can be checked today; two builders share one Contract compiler, which takes a minute a run until X1 lands; the phone work is not on main, so until K3 a station cannot land as a landing"
  next_action: "read the reports of lanes exact-loop and exact-paper, build each in the build tree, and settle C125 and C126; put the design before two seats (X0)"

receipts:
  - station: X0
    what: "the Android toolchain proven; the app's baseline on Android; our own loop measured; our Exact pages corrected"
    landed: [fba6b1210, a40d00a81, cbff38664, 9b0b6e5a0, b3ed207de]
    proof: "Shelf sample built and driven on emulator-5554 (tree and screenshot correct); Autoradio built for Android in 260 s, 114 sources substituted, pageOpen NotSupportedError; contract build 64 to 66 s, 16 s without the mark's drawing; web loop edit 57 to 90 s"
    review: "none; docs only"
---

# Autoradio: the cutover from Swift to Exact

The owner asked for an Android build on 2026-11-18 and found that the app
had been built against Exact's grain: Exact pages over a Swift application.
This file is the mission that moves the application into Exact. The rule is
in CLAUDE.md. The measured starting point is
[the problem record](../problems/autoradio-app-logic-is-swift-not-exact-2026-11-18.md).
The design is
[docs/design/autoradio-in-exact-2026-11-18.md](../design/autoradio-in-exact-2026-11-18.md).
How Exact means an app to be built is in
[docs/reference/EXACT.md](../reference/EXACT.md).

## What native code is, when this is done

Two things. The voice: the audio session, the engine, the microphone, speech
in and out, the on-device models, the take recorder. And the deck: a queue
of prepared audio that plays back to back, reports what started and ended,
and keeps playing if shared code is not running. The deck decides nothing.
Swift on iPhone, Kotlin on Android, one interface.

## Why two causes of slow builds are both in scope

Moving logic out of Swift removes the native build from most edits. It does
not make the pages compile quickly: that is a second, separate cause, found
when Exact's loop was run on this app for the first time. Station X1 owns
it. The finish line names a time, so neither cause can be left standing.

## How a station is done

Tests first, from the fixtures and the Swift tests that carry product rules.
The shared code is written; the page reads it; the Swift for that area and
its interface names are deleted in the same change. The lead builds the
iPhone app and the Android app from that commit in the build tree, runs the
tours, and installs it on the owner's phone. A red station has two seats
before it is installed. Nothing selects between the old path and the new.

## What is carried from the learning loop

The learning-loop file
([choir-autoradio-learning-loop-2026-11-18.md](choir-autoradio-learning-loop-2026-11-18.md))
is a checkpoint, not complete. Settlement and the takes server are deployed
and proven. Its open work continues here as K1 (the iPhone repairs), K2 (the
notebook's landing) and K3 (the phone work onto main). Its conjecture K1
(ledger C120) stays open and is settled by the owner's listening, which this
cutover must not interrupt.
