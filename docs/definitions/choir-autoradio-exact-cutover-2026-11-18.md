---
definition_version: 7
readiness: executable
review:
  reviewer: seat 1 (a Claude Sonnet reviewer, with Exact's source) and seat 2 (DeepSeek v4.1 flash through OMP), on the design and this file
  frozen_ref: origin/main fc6f89fb0
  verdict: send_back
  evidence_ref: 'both returned REPAIR with the frame upheld; this revision answers them (design, section "After review"). Seat 2 output: session scratchpad rev-cutover; seat 1: the lead''s session. The revised file has not been re-reviewed; red stations X3 to X6 and X9 each get their own two seats.'
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
    recovery: the one build tree, on local branch android-probe 572e6f203 (lane/autoradio-integ20 plus app.json naming Android), pushed as lane/android-probe; holds the one Exact checkout (mobile/.exact-src at de212e671 with our patches)
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
  - 'The phone app''s pages are Exact contract files (50 files, 7,742 lines). Its logic is Swift: 157 files and 38,149 lines in mobile/AutoradioKit/Sources, reached through 181 native calls bound by 22 Swift files (mobile/exact-package). About a quarter of the Swift is voice. The app''s TypeScript is one file of 11 lines. The package has no Android side. Record: docs/problems/autoradio-app-logic-is-swift-not-exact-2026-11-18.md.'
  - None of the phone work is on main. It lives on lanes; the owner's iPhone runs lane/autoradio-integ20 8ff44115f, installed 2026-11-18 06:39 local. On that build, recording a take breaks the radio until the app is reopened.
  - 'Exact''s intended loop (bun exact.mjs web, a native client following it) has never been used on this app. Measured today in the build tree: a one-line page edit takes 57 to 90 s in that loop; the Contract compiler takes 64 to 66 s on our pages and 16 s with the Tetramark''s drawing stubbed out; the mark is 4.6 MB of a 5.9 MB plan, compiled once for each of its 29 uses. Exact''s figure for a fresh app is about 0.1 s.'
  - The Android toolchain works on this Mac. Exact's Shelf sample built (6 min 33 s), installed on an emulator and drew correctly. Autoradio built for Android unchanged (260 s) and draws its shell, bar, mark and tray; under the driver all 114 source reads are substituted and a send fails with NotSupportedError. No physical Android phone is reachable.
  - 'Read from Exact''s source, not run: shared code keeps running on a locked iPhone while our audio plays; on Android the page freezes when the app leaves the screen unless a foreground-service claim is held, and Exact has no media-playback service kind; whether Exact''s iOS files carry the file protection our mail store sets is not stated.'
  - 'Our Exact reference pages were corrected against the source today (docs/reference/EXACT.md, ''How Exact means an app to be built''). The design for the move is docs/design/autoradio-in-exact-2026-11-18.md: proposed, read from source, reviewed by no one.'
  - The Go server has the run sheet reducer and shared fixtures (mobile/fixtures/runsheet, mobile/fixtures/takes) that two implementations already satisfy. Bun is in neither the flake nor CI.
finish:
  deliver: Autoradio is one Exact codebase that runs on iPhone and Android. Native code is the voice and nothing else. A change to a page or to logic is written once and seen in seconds.
  artifact: 'mobile/AutoradioLab holds the pages (contract) and all logic (TypeScript modules with their tests); mobile/AutoradioKit and a new Kotlin package hold only the voice list: the audio session, the engine, the microphone, speech in and out, the on-device models, the take recorder, clip fingerprinting, and the deck (a queue of prepared audio that applies a short list of named rules, keeps a journal of what started and ended, and plays on if shared code is not running); the same commit builds an iPhone app, an Android app and the Mac app; all of it is on main.'
  acceptance:
  - action: 'scripts/check-autoradio-native-boundary on main (a gate this mission builds): it runs the pages on the web host with a fake native side that answers only the voice list, scans every shipped native source including the app target, refuses an interface entry that takes an opaque payload, and carries a planted violation it must catch'
    proves: no page needs anything native outside the voice list, no native source or interface name exists outside it, and the gate can fail
    evidence_class: repo gate in CI
  - action: 'on the Android emulator, signed in to the drill computer, sound muted: press Play; a stretch from staging is placed and three items start and end on a deck that rendered no audio; then read the server''s ledger for those entries'
    proves: the shared logic alone runs the radio's transport and reducer on Android against the real server, and a deck that made no sound produces no verdict, never heard
    evidence_class: deployed proof with a rerunnable script
  - action: a person listens to three entries on Android (the emulator's sound or a phone) and skips one; then the scorecard route
    proves: an entry a person heard on Android settles as heard and a skipped one as not heard, from the deck's journal of rendered audio
    evidence_class: a person's ears, and the server's ledger
  - action: the same commit on the owner's iPhone for one ordinary day, including a locked hour (the app killed once in it), a phone call, a take, a podcast clip, a file read aloud and mail; then the owner's verdict, and the server's ledger for that day
    proves: the iPhone app is no worse than the Swift one for the person who uses it daily, and every entry heard with the screen locked reached the server with the time it was heard
    evidence_class: the owner's verdict, and the phone's console log for that day
  - action: 'in the build tree: time a one-line edit to a logic module and a one-line edit to a page in the web loop, three runs each; time the logic test suite'
    proves: a logic edit is seen in 5 s or less, a page edit in 15 s or less, and the logic tests run in 30 s or less, with no native build
    evidence_class: command wall time read by the lead
  - action: go test ./internal/runsheet/... and bun test in mobile/AutoradioLab, both reading mobile/fixtures
    proves: the run sheet has two implementations held to one set of fixtures (Go and TypeScript); the Swift copy is gone
    evidence_class: CI
  rollback: each station is one landing on main and reverts as one; the phone rolls back by installing the build before that station (lane/autoradio-integ20 until station K1 replaces it, then each station's predecessor). Stations marked irreversible below say what does not come back.
  landing:
    required: true
    environment: main for the source; the owner's iPhone and the Android emulator for the apps; staging (choir.news) for the server the apps talk to
    required_receipts:
    - pushed_commit
    - ci
    - device_install
    - deployed_acceptance
value:
  better_means: 'Fewer native lines and names outside the voice list, and a shorter time from an edit to seeing it, while everything the iPhone app does today keeps working for the owner: the tours pass, the product rules held by the Swift tests are held by shared tests, and a locked phone still plays and still reports what was heard.'
  goodharting_would_be: Calling logic 'voice' to keep it native, or one native call that decides everything behind a thin TypeScript wrapper. Deleting a feature to delete its Swift. TypeScript that only works on iPhone because it leans on native calls. Web tests passing on placeholder data while a phone is broken. A fast loop reached by removing the mark or the pages a listener sees. Android 'working' only under the driver with substituted data. Counting lines moved and not behaviour kept.
homotopy:
  realism_axis: 'How much of the real app runs on shared logic, on how real a device: from one page (the paper) answered by TypeScript on the web host with a canned fetch, to every page and the radio''s decisions on both phones against staging, screen locked for an hour, in the owner''s daily use. Every point is the same app and the same server; a station moves one area fully and deletes its Swift, so no point is a second app or a switched path.'
boundaries:
  mutation_class: red
  authority_sources:
  - the owner's words of 2026-11-18 (owner_rulings)
  - 'CLAUDE.md, Product: the phone app is Exact; only voice is native'
  - docs/choir-doctrine.md and docs/learning-engine.md
  - Exact's source at the pinned commit, over any of our pages about it
  - docs/design/autoradio-in-exact-2026-11-18.md, a proposal until station X0 reviews it
  must_preserve:
  - 'no switches: a station replaces the Swift for its area and deletes it in the same change; nothing selects the old or the new path'
  - the owner's iPhone app does not get worse; Swift repairs for daily use are allowed until that page moves
  - only the listener's phone op settles a forecast; an entry that made no sound is never reported heard; a late or lost report is no verdict
  - 'Autoradio is an interface into the listener''s own Choir computer (owner, 2026-11-18): mail, files and anything else the phone holds may go to that computer and its desks; the phone keeps copies so it works offline. Nothing is withheld from the listener''s own computer. (Lead''s defaults, not owner rulings, which the owner may strike: the mail copy on the phone is encrypted at rest; sign-out removes the account''s data from the phone.)'
  - logs carry ids and counts, never words spoken, titles or mail (lead's default, not an owner ruling)
  - one voice at a time; Autoradio does not take the microphone or the audio session from another app without the listener's press
  - no limit on our own model calls; the phone asks the desk as rarely as the job allows
  - one fixture set per wire, written by the server side and read by every implementation
  - 'build output in one place: native builds, simulators and emulators run only in the lead''s build tree; a lane runs the Exact loop that needs no native build'
  - 'the deck is the evidence: an op is posted with the time the deck recorded, and an item with no rendered audio is never posted as heard'
  - shared code is not updated over the air while it holds the sign-in key (deploy.store stays "0")
  - a station says what local state it moves and what is fetched again; the owner loses nothing without being told the day before
  excluded:
  - new product features while a page is being moved, including takes from voice mode (the owner's idea of 2026-11-18, held until the owner says more)
  - on-device models on Android; the first Android voice is the system's
  - TestFlight and the Play Store (TestFlight stays with the learning-loop file's station L9 and the owner's paid team)
  - changes to the server's routes beyond what a shared client needs to read them the same way
  - an upstream pull request to Exact without the owner's word
  protected_surfaces:
  - 'auth and session: sign-in, the key and where it is kept, sign-out wipe (station X3)'
  - 'run acceptance and evidence: the ops the phone posts that settle forecasts (stations X4, X5)'
  - the mail store and its encryption (station X8)
stations:
- id: X0
  name: Baseline, review and the device tests
  scope: 'done: the Android toolchain, the app''s baseline on the emulator, the design and this file before two seats. Open, on the owner''s iPhone: after first unlock and locked, play three entries, kill the app, relaunch (does shared code hear a deck event and post with no touch; is the key readable; what protection class do Exact''s files and SQLite get); a call, a headphone unplug, speaking over the radio. Open, on the emulator: a stream that is a POST with our headers and the real grant, a dropped stream, a long poll.'
  class: yellow
  review: 'two seats on the design (done: REPAIR, answered)'
  depends_on: []
  carrier: lane/android-probe; docs/design/autoradio-in-exact-2026-11-18.md
  status: working
- id: X1
  name: The loop is fast
  scope: 'the Contract compile of our pages falls from 64 s to 15 s or less with the mark looking the same, and this station owns the rest of the time too: the 16 s that remains without the mark is explained and cut, not only reported; the Android refusals and Apple-only symbols are fixed or listed; a proposal for the lab pages leaving the product app'
  class: orange
  review: one seat
  depends_on: []
  carrier: lane/exact-loop
  status: working
- id: X2
  name: The paper page, as the pattern
  scope: the paper page's data is answered by TypeScript (public routes, no sign-in); its Swift and interface names are deleted in the same change, including the native text measuring its layout uses today if the page can lay out with Exact's own layout; a shared fixture, bun tests and contract tests; the pattern written into docs/reference/exact/native-modules.md; real stories on the Android emulator
  class: orange
  review: one seat
  depends_on: []
  carrier: lane/exact-paper
  status: working
- id: XG
  name: The boundary gate, the test map and Bun in CI
  scope: 'scripts/check-autoradio-native-boundary as the finish line describes it, with a ceiling that only falls; a file-by-file map of the Swift tests (stay with the voice, port with which station, die); pinned Bun 1.4.2 in the flake and a CI job that runs the logic tests and fails when a shared fixture is read by only one implementation. Built (2026-11-19, lane/exact-xg): the gate, its self-test, the voice-and-deck list (scripts/autoradio-native-voice-list.txt), the ceiling (scripts/autoradio-native-ceiling.txt), both run by scripts/gates, and the test map (docs/design/autoradio-swift-test-map-2026-11-18.md). Remaining: a CI step for the gate and its self-test (a workflow change, its own review), Bun 1.4.2 pinned in the flake, the CI job that runs bun test in mobile/AutoradioLab and fails when a fixture under mobile/fixtures is read by one implementation only, and the gate''s later rules (it runs the pages on the web host against a fake native side that answers only the voice list, refuses an interface entry with an opaque payload, requires the interface to equal the voice list at the end, carries a planted violation it must catch; the ceiling then goes to zero outside the list).'
  class: yellow
  review: none
  depends_on: []
  carrier: lane/exact-xg
  status: working
- id: X3
  name: Sign-in in shared code
  scope: 'device-flow sign-in, accounts, the proof key and computer discovery in TypeScript; the key in Exact''s secret store; one sign-out wipe across both languages with the generation passed to shared code; account and computer settings. ChoirClient stays, fed the identity by one call, until the last station. The old Keychain item is deleted at the move. Decided before it starts: the key is readable on a locked phone (X0), deploy.store stays "0", the wipe order. The owner signs in once more.'
  class: red
  review: two seats
  depends_on:
  - X0
  - X2
  carrier: none yet
  status: pending
- id: X4
  name: The deck, under the Swift player
  scope: 'the deck interface in Swift in front of the existing engine: the queue, the named rules of the design, one state for why it is silent, the journal of started and ended with device time, position and rendered audio; the Swift player drives it and reads the journal before it restores the run sheet. No decision moves yet. Passes a locked hour with a kill on the owner''s phone.'
  class: red
  review: two seats
  depends_on:
  - K1
  carrier: none yet
  status: pending
- id: X5
  name: The run sheet and the radio's decisions in shared code
  scope: the reducer in TypeScript on the shared fixtures, the store in SQLite, the feed (a long poll first), the writer that asks the desk, the outbox of ops posted with the journal's times, the on-air decisions; a Kotlin deck with the same interface (system voice, Media3). Deletes the Swift run sheet, the decision half of the player, the radio, bar and queue page models. Pending ops are posted before the move. Android plays the radio on screen.
  class: red
  review: two seats
  depends_on:
  - X0
  - X3
  - X4
  carrier: none yet
  status: pending
- id: X6
  name: Background play on Android
  scope: a media-playback service kind carried as a patch to Exact (mobile/exact-patches), a media session, the Kotlin deck playing on with the app off screen and its journal posted afterwards
  class: red
  review: two seats
  depends_on:
  - X5
  carrier: none yet
  status: pending
- id: X7
  name: Podcasts and clips
  scope: feeds, subscriptions, positions, downloads and the clip store in TypeScript; play goes through the deck; fingerprinting stays native; subscriptions and positions are moved, not lost
  class: orange
  review: one seat
  depends_on:
  - X5
  carrier: none yet
  status: pending
- id: X8
  name: Files, the reader and books
  scope: the files page, viewers, the shelf, EPUB in TypeScript; read-aloud goes through the deck; a PDF's text is read by the listener's computer (decision D4) and PDFKit is deleted; book positions are moved
  class: orange
  review: one seat
  depends_on:
  - X5
  carrier: none yet
  status: pending
- id: X9
  name: Mail
  scope: 'the mail pages and store in shared code, encrypted at rest by what X0''s protection test allows (Exact''s files if protected, else a small native seal with the key in the Keychain or Keystore); wiped at sign-out. Irreversible on the phone: the old store is deleted and mail is fetched again.'
  class: red
  review: two seats
  depends_on:
  - X0
  - X3
  carrier: none yet
  status: pending
- id: X10
  name: Content, search, takes, the notebook pages
  scope: the content store and ranking in SQLite; the takes list and publish (multipart built as bytes); the notebook and learning pages; embeddings stay native with the models
  class: orange
  review: one seat
  depends_on:
  - X3
  carrier: none yet
  status: pending
- id: X11
  name: Settings, onboarding, themes, cards, the bar
  scope: the remaining page models and demo data as shared code and JSON; settings, voice, speed and theme are moved; the lab pages leave the product app
  class: orange
  review: one seat
  depends_on:
  - X5
  carrier: none yet
  status: pending
- id: X12
  name: Only voice is native
  scope: ChoirClient goes; what remains in mobile/AutoradioKit is the voice list; the gate's ceiling is zero outside it and the interface equals the voice list; the Mac app builds and runs from the same commit; current-architecture.md says what is built
  class: orange
  review: one seat
  depends_on:
  - X6
  - X7
  - X8
  - X9
  - X10
  - X11
  carrier: none yet
  status: pending
- id: K1
  name: The owner's iPhone keeps working (carried from the learning loop)
  scope: 'the reviewed repairs for the first listening session''s faults: audio lost after a take (lane/audio-held, in its third review round) and the slow start and silence after the opening (lane/start-gap, accepted), built as one candidate (lane/autoradio-integ21) and installed when the owner''s phone is back'
  class: red
  review: two seats, then seat 2 again after each blocking finding
  depends_on: []
  carrier: lane/autoradio-integ21; lane/audio-held; lane/start-gap 1ef916bd3
  status: working
- id: K2
  name: The notebook lands (carried from the learning loop, stations L4 and L5)
  scope: 'lane/learn-notebook: the server writes and reads the listener''s notebook; server code only, untouched by this cutover'
  class: red
  review: two seats
  depends_on: []
  carrier: lane/learn-notebook
  status: landed
  landed: d21930b76 on 2026-11-18T17:45Z (second attempt; the first failed on a missing trust impact row); CI run 44362158500 green; staging reports the commit (deployed 18:07Z); scripts/prove-autoradio-learning passed on the drill computer at 18:10Z
- id: K3
  name: The phone work is on main
  scope: the installed build's branch (lane/autoradio-integ20, 162 commits, with the Exact pin and our patches) merged into main in one landing, its thirteen documentation conflicts with today's audit resolved by hand; K1's candidate lands on top when accepted. After it every station lands on main as a landing.
  class: orange
  review: one seat on the merge; the parts were reviewed in lanes on 2026-11-18
  depends_on: []
  carrier: lane/phone-main
  status: landed
  landed: 33cf20826 on 2026-11-18T13:11Z; CI run 44311266775 green; staging reports the commit (deployed 13:20Z); the paper was publishing at 13:43Z
decisions_for_the_owner:
- id: D1
  question: 'Is the order right: the paper page, then sign-in, then the deck under the Swift player, then the radio''s decisions, then the other pages in parallel?'
  why: it decides when Android first shows real data (after X2), when it first plays (after X5), and it keeps the owner's phone from running new decisions over an unproven deck
  options: as stated; the radio first with the system voice and sign-in after; sign-in first
  default: as stated
- id: D2
  question: May we carry a patch to Exact that adds a media-playback service on Android?
  why: without it the radio stops when the Android app leaves the screen; with it we hold a fork until Exact takes it
  options: carry it in mobile/exact-patches and tell Exact's authors; carry it silently; no background play on Android for now
  default: carry it in mobile/exact-patches; nothing is sent upstream without the owner
- id: D3
  question: The owner signs in again once when sign-in moves, and mail is fetched again once when mail moves. Everything else the owner would miss (podcast subscriptions and positions, book positions, settings, voice, speed, theme) is carried across by a one-time read that is deleted afterwards. Acceptable?
  why: each Swift store dies with its Swift; what is not carried is lost on the owner's phone
  options: as stated; carry nothing and set the phone up again; migrate the key and mail in place too
  default: as stated, each loss announced the day before
- id: D4
  question: Where is the text of a PDF read?
  why: Exact has no PDF text reader, and the phone is an interface into the listener's computer, which can read any file
  options: the listener's computer reads it and the phone asks for the text; a JavaScript PDF library in shared code; keep one native call per platform
  default: the listener's computer reads it; the native PDF code is deleted. A PDF opened with no connection waits for one
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
- id: D9
  question: Does the Mac app follow the phone at each station, or at the end?
  why: it builds from the same pages and the same Swift voice; checking it at every station costs a Mac build each time
  options: at the end (station X12); at every station
  default: at the end; a station that breaks the Mac build says so
- id: D10
  question: Who listens for the first heard on Android?
  why: an emulator with no sound cannot honestly report that an entry was heard, and the lead cannot hear
  options: the owner, on the emulator's sound or a phone; any tester with an Android phone
  default: the owner, when convenient; until then Android's proof stops at no verdict
owner_rulings:
- true: 2026-11-18
  decision: R1
  said: what do you mean all of the apps logic is a swift package? only the voice stuff should be swift. the rest should be exact...
- true: 2026-11-18
  decision: R2
  said: omg, we need to rewrite it lol. i didnt realize we were skipping exact. now i know why the build times are so slow lol. wow, this is annoying. well, all the more reason to work on the android version. IN EXACT. the whole point is the codebase is shared, using EXACT
- true: 2026-11-18
  decision: R3
  said: Ok I guess that old phone won't work...not sure why, it did last week. It's plugged in with usb debugging. Anyway, we can work in android simulator. And much more importantly we need to rewrite the app in exact. That should make it actually cross platform and improve build times a lot
  read_as: the emulator is the Android device for now; the rewrite outranks the other phone work
- true: 2026-11-18
  decision: R4
  said: Make sure we're using exact correctly, because using it with TypeScript should lead to fast builds. That's one of the things that they advertise. Make sure you're reviewing the exact documentation. Make sure that our own exact.md is correct. It must not have been correct because we've been misusing it or not using it the whole time.
  read_as: Exact's own documents are the authority on how to build with it; our pages are corrected to them; fast edits are part of the finish line, not a hoped-for side effect
- true: 2026-11-18
  decision: R5
  said: Let's make a new goal file for this cutover from swift to exact
  read_as: this file; the learning-loop file is a checkpoint, with its open stations carried here as K1 to K3
- 'on': '2026-11-18'
  decision: R6
  said: Just to be clear, there is no rule that local files can't go to server. And that's unrelated to native code vs exact anyway. Not sure where these ideas came from. The whole point is that autoradio is an interface into user's own choir vm
  read_as: nothing the phone holds is withheld from the listener's own computer; the withholding built on 2026-11-17 and 2026-11-18 is removed; where logic runs (native or shared) is a separate question from where data may go
now:
  status: working
  slice: X1 (the loop is fast) and X2 (the paper page) are being built in lanes; K3 (the phone work onto main) is being merged; K1 is in its third review round and K2 in its last fixes; X0's device tests wait for the owner's phone
  source_ref: origin/main at the commit that carries this file
  deploy_identity: staging ce719fe1a; the owner's iPhone on lane/autoradio-integ20 8ff44115f
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
    claim: If every page's data and decisions are shared TypeScript and native code is the voice and a deck, then one change runs on both phones and is seen in seconds, and the iPhone app is no worse for its daily listener.
    test: the boundary gate at zero; the Android acceptance script against staging; the measured loop time; the owner's verdict after a day on the cut-over build
    edge: missing_oracle
    delta_o: 'a locked phone is the oracle we lack. Read from Exact''s source and not run: shared code stays live on a locked iPhone only while audio is sounding, and nothing shows how a deck event becomes a posted op with no touch. The smallest upgrade is station X0''s test on the owner''s iPhone, before X4 is built. The dangerous edge is frame lock: thinking in terms of ''shared code runs or it does not'', when it runs in turns of 100 ms with no timers outside an answer; so the deck applies its rules and keeps its journal alone, and the design does not depend on the answer.'
    scope_if_supported: this app, iPhone and Android, at the pinned Exact commit
    status: active
    evidence_refs: []
  decision:
    what: native code is the voice and a deck that applies named rules and keeps the journal; everything else is shared; the deck lands under the Swift player before the decisions move; stations replace and delete; the phone work lands on main first
    kind: architecture
    status: proposal
    evidence_ref: docs/design/autoradio-in-exact-2026-11-18.md
    owner_ratification_ref: the rule is the owner's (R1, R2); the deck, its rules and the order are the lead's proposal, revised after two seats, until the owner rules on D1
  belief:
    believed_state: the pages already run on Android and the web with nothing behind them; the first page is being moved; the app's own Exact loop is a minute per edit and most of that is one component; the iPhone app on the owner's phone still loses audio after a take
    main_uncertainty: whether a deck event can cause an op to be posted on a locked phone, and what Exact's secret store and files allow while locked; the design is built to survive either answer, at the cost of late reports
    next_observation: the paper lane's report (whether a page can read a TypeScript source while the package answers the rest), then the locked-phone test when the owner's iPhone is back
  blocker_or_risk: the owner's iPhone is away, so nothing about sound or a locked screen can be checked today; two builders share one Contract compiler, which takes a minute a run until X1 lands; the phone work is not on main, so until K3 a station cannot land as a landing
  next_action: read the reports of lanes exact-loop, exact-paper and phone-main; build each in the build tree; settle C125 and C126; land K3
receipts:
- station: X0
  what: the Android toolchain proven; the app's baseline on Android; our own loop measured; our Exact pages corrected
  landed:
  - fba6b1210
  - a40d00a81
  - cbff38664
  - 9b0b6e5a0
  - b3ed207de
  proof: Shelf sample built and driven on emulator-5554 (tree and screenshot correct); Autoradio built for Android in 260 s, 114 sources substituted, pageOpen NotSupportedError; contract build 64 to 66 s, 16 s without the mark's drawing; web loop edit 57 to 90 s
  review: none; docs only
- station: X0 (review)
  what: the design and this file before two seats
  proof: both REPAIR, frame upheld; eight blocking findings answered in the design section "After review" and in this revision (the deck's rules and journal, the order, sign-in, honest Android outcomes, the gate, the loop target, the landing path, migration)
  review: seat 1 Sonnet with Exact's source; seat 2 DeepSeek
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
in and out, the on-device models, the take recorder, clip fingerprinting.
And the deck: a queue of prepared audio that plays back to back, applies a
short list of named rules that cannot wait for shared code (the design lists
them), keeps a journal of what started and ended and whether sound was
made, and plays on if shared code is not running. Swift on iPhone, Kotlin on
Android, one interface.

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
