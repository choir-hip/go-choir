---
definition_version: 4

readiness: drafted

review:
  reviewer: none
  frozen_ref: none
  verdict: none
  evidence_ref: none

start:
  captured_at: 2026-11-10T18:10:25Z
  source:
    canonical_ref: origin/main 87f0d495cd1a479a40aa62c2abbc719e1a8a6cce
    deploy_identity: choir.news deployed_commit d68d6f1a1c3a
  worktrees:
    - path: /Users/wiz/Dev/go-choir
      status: dirty
      class: user_wip
      owner: owner
      touch: forbidden
      recovery: leave in place (one untracked env backup that holds secrets)
    - path: /Users/wiz/Dev/mike
      status: dirty
      class: user_wip
      owner: owner
      touch: read_only
      recovery: leave in place (head 66b5b7f0, 2026-10-31; two dirty paths)
  observed:
    - No native app tree in the repo. No `mobile/`, no Xcode project.
    - Xcode.app is installed, but `xcodebuild` is not reachable from the
      repo's Nix dev shell (DEVELOPER_DIR points at a Nix SDK). iOS builds run
      outside that shell.
    - No paid Apple developer account. Device builds use free provisioning.
    - Exact and nsh are not checked out on this machine.
    - Mike's voice stack was tested thoroughly on the owner's iPhone (owner,
      2026-11-10); Mike's docs are older than that and say otherwise.
    - Autopaper pass 1 status per lane is not reconciled here; see
      `now.next_action`.

finish:
  deliver: >
    A reader holds an iPhone, hears their edition read aloud with podcast
    clips between stories, interrupts by voice to ask or comment, and sees
    the same Texture pages the web shows, drawn natively.
  artifact: >
    An iOS app in `mobile/` on Exact at a pinned commit, installed on a real
    iPhone from Xcode, signed in as a staging identity that is not the
    owner's, working against choir.news; plus the server pieces it needs on
    main and deployed.
  acceptance:
    - action: a fresh staging account approves the device with its passkey in a browser; the phone appears by name in that account's sessions; revoke it there
      proves: S1, a named revocable device with a refreshable token, without a passkey entitlement in the app
      evidence_class: deployed proof, real iPhone, screen recording and session list API output
    - action: start the edition and do not touch the phone
      proves: a story in the AI voice is followed by a podcast clip with no tap between
      evidence_class: deployed proof, real iPhone, audio recording and the run sheet record
    - action: lock the screen for 30 minutes during playback and use the lock-screen controls
      proves: background playback, Now Playing and remote commands
      evidence_class: real iPhone, device log with timestamps
    - action: the owner runs the 30-minute voice QA script in each mode (button, wake word, open, hybrid) on the loudspeaker and on AirPods
      proves: at least one mode is above the owner's usability threshold; this gates every later lane
      evidence_class: real iPhone, the per-mode numbers of docs/design/autoradio-voice-modes-2026-11-10.md and the owner's verdict
    - action: interrupt by voice during playback and ask about the story
      proves: barge-in over podcast audio, an answer is heard, and the request is a prompt bar submission on the tape
      evidence_class: deployed proof, tape record id
    - action: skip three clips from one show early, then start the next session
      proves: each run sheet choice is a precommitment record with a stated probability, settled by what the listener did, and the next run sheet reads those scores
      evidence_class: deployed proof, the commitment record ids, their settlements, and the two run sheets
    - action: comment by voice
      proves: the reader gets their own fork, shown natively
      evidence_class: deployed proof, fork document id
    - action: fire a changelog event for the account
      proves: the relay builds the right notification and the app opens the right native screen from it
      evidence_class: relay record on staging; on the phone, a local notification carrying the same payload (remote delivery waits for a paid account)
    - action: run the golden documents through both renderers
      proves: web and native draw the same page format
      evidence_class: scripted comparison, tracked in the repo
    - action: delete the account from the app
      proves: account deletion in the app
      evidence_class: deployed proof, account lookup after
    - action: capture the phone's network traffic for one voice session
      proves: no recognition or speech audio leaves the phone
      evidence_class: capture file and its summary
    - action: write the App Review notes, privacy labels and reviewer account plan
      proves: the submission is ready except for the paid account
      evidence_class: document in the repo
  rollback: >
    Server lanes revert by commit. The app is not distributed; removing it
    from the phone is the rollback. Each server lane names its own rollback
    in its commit.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >
    Minimise the count of the ten acceptance items not yet shown on a real
    iPhone against staging, while every server change extends an existing
    mechanism and the web product keeps working.
  goodharting_would_be: >
    Items shown in the simulator or against a local server; a voice demo
    that works only in silence and not over podcast audio; a native renderer
    that passes golden documents chosen because they pass; a second sign-in
    or input path built beside the existing one.

homotopy:
  realism_axis: >
    Simulator with Exact's speech packages as doubles, then a real iPhone on
    free provisioning with Mike's voice module, then the same build over
    podcast audio with the screen locked for 30 minutes, then a TestFlight
    build under a paid account.

boundaries:
  mutation_class: red
  authority_sources:
    - owner statements of 2026-11-09 and 2026-11-10
    - docs/design/autoradio-mission-handoff-2026-11-09.md
    - docs/design/autopaper-next-mission-handoff-2026-11-09.md section 7
    - docs/choir-doctrine.md
    - CLAUDE.md
  must_preserve:
    - one sign-in system (the device authorization flow), one input route (the prompt bar), one custodian per data kind
    - no recognition or speech audio leaves the phone
    - no over-the-air code or UI updates in the shipped app
    - only Exact's audio session owner configures the audio session
    - the web product and the paper keep working
  excluded:
    - Android, CarPlay, the watch
    - YouTube, Sign in with Google, e-books
    - block editing on the phone, Mail
    - a server-side voice
    - TestFlight and App Store submission (waits for the paid account)
    - the rename of retired Autopaper names in code
  protected_surfaces:
    - auth and session (lane S1)
    - Texture canonical writes and the publish gate (lane S2)
    - tape and evidence (lanes D and U)

now:
  status: working
  slice: step 0 (reconcile, page format, frozen API, runtime spike)
  source_ref: origin/main 87f0d495cd1a479a40aa62c2abbc719e1a8a6cce
  deploy_identity: choir.news d68d6f1a1c3a
  candidate:
    id: none
    state: none
    ref: none
    base: none
    digest: none
    scope: []
  conjecture:
    id: AR-C1
    claim: >
      Two audio states serve the product: a listening state with full
      quality playback and the phone's own microphone feeding a wake word
      detector, and a talking state with playback paused and voice
      processing on; the switch between them is fast enough not to be
      noticed, with the screen locked, on the loudspeaker and on AirPods.
    test: >
      The five questions of docs/design/autoradio-voice-modes-2026-11-10.md
      ("What the spike must answer") on the owner's iPhone, with Mike's
      `setCategory` and `setActive` calls replaced by Exact's session owner.
    edge: missing_oracle
    delta_o: >
      Exact's test driver on the physical phone plus a device log of every
      audio session category change, so a second caller is visible.
    scope_if_supported: lanes V and P may be built on Exact as designed
    status: proposed
    evidence_refs: []
  decision:
    what: Exact leads; nsh is the fallback; no third stack without a written finding
    kind: architecture
    status: settled
    evidence_ref: docs/design/autoradio-mission-handoff-2026-11-09.md section 4, step 0.3
    owner_ratification_ref: owner, 2026-11-09 and 2026-11-10 ("built on Exact")
  belief:
    believed_state: >
      The server side is mostly extension of running mechanisms. The risk is
      on the phone, in audio session ownership and background listening,
      which Mike does not do today.
    main_uncertainty: >
      whether AR-C1 holds on the pinned Exact commit, and whether any voice
      mode clears the owner's threshold (owner, 2026-11-10: pure push-to-talk
      does not)
    next_observation: the step 0.3 spike on the owner's iPhone
  blocker_or_risk: >
    The spike needs the owner's iPhone connected and unlocked, and a free
    provisioning profile, which expires after seven days.
  next_action: >
    Reconcile: for each Autopaper item in the handoff's section 5, record
    whether it is on staging. Then start step 0.1 and 0.2 on the build VM and
    0.3 on the Mac.

receipts: []

weak_measures:
  - name: time from a spoken interruption to the first word of the answer
    kind: telemetry
    baseline: unknown
    desired: lower
    decision_use: which voice mode and window length to ship as the default
    cannot_prove: that the answer is right
  - name: model download size at onboarding
    kind: telemetry
    baseline: several hundred MB (Mike's docs)
    desired: stated to the reader before the download
    decision_use: which models ship in pass 1
    cannot_prove: recognition quality
---

# Autoradio, pass 1

The inputs are the
[handoff](../design/autoradio-mission-handoff-2026-11-09.md) and the review
of Mike on 2026-11-10. This file changes the handoff in the places below and
otherwise follows it.

## What changed from the handoff

1. **No TestFlight in pass 1.** The owner has no paid Apple account and will
   get one when the app is worth sharing. Every acceptance item is shown on a
   real iPhone installed from Xcode. Submission becomes "ready except for the
   account".
2. **Push is proved in two halves.** Remote notifications need the paid
   account. Pass 1 proves the relay on staging and proves on the phone that a
   notification with that payload opens the right screen.
3. **Sign-in does not use a passkey inside the app.** The existing device
   authorization flow has the reader approve in a browser. The app needs no
   associated-domains entitlement, which a free account cannot have. This is
   from general knowledge of Apple's rules and is checked in lane S1's first
   hour.
4. **Mike changes lanes V and P.**
   - Mike's plugin configures the audio session itself. Those calls come out;
     the session comes from Exact's owner.
   - Mike's voice mode ends when the app leaves the foreground. Listening in
     the background is new work.
   - Mike has no Now Playing or remote commands and plays separate WAV
     chunks. Lane P builds a native player with one seekable timeline.
   - The voice stack needs iOS 18 or later. The target stays iOS 27.
   - Most models download after install. Onboarding says so.
5. **Mike's licence.** The repo is AGPL-3.0, but the speech plugin is the
   owner's own code (owner, 2026-11-10: "Mike speech code is mine"). Lane V
   ports it. The third-party models it loads keep their own licences, which
   lane V lists.

## Voice comes first (owner, 2026-11-10)

Pure push-to-talk is below the usability threshold, so the voice modes are
built and judged before the shell, the renderer and the desk. The design
is [autoradio-voice-modes-2026-11-10.md](../design/autoradio-voice-modes-2026-11-10.md).
Step 0.3 becomes that note's spike, then a voice-only test app with all
three modes and the hybrid, then the owner's QA session. Lanes N, R, D and
X start after the owner's verdict. S1, S2, 0.1 and 0.2 do not depend on it
and may run alongside.

Sign-in stays the existing device authorization flow
(`internal/auth/device_login.go`): the app holds no passkey, the reader
approves in a browser, and tests approve with the virtual authenticator
the deployed specs already use.

The learning loop is framed as precommitment records: a run sheet choice is
a request with a stated probability, the listener's behaviour settles it,
and the desk reads its own scored record when it builds the next run
sheet. New settlement rule kinds (heard through, skipped early) and
trusted playback events from the phone are design work for lane D.

## Order of work

**Step 0** (three agents, in parallel):

| Item | Output | Where |
| --- | --- | --- |
| Reconcile | which Autopaper items of handoff section 5 are on staging | read-only |
| 0.1 Page format | a written spec beside `internal/texturedoc`, with golden documents and the audio timeline | build VM |
| 0.2 Frozen API | the list of endpoints the app calls, versioned, with the rule for a newer server | build VM |
| 0.3 Runtime spike | the five checks of the handoff on a real iPhone at a pinned Exact commit; decides AR-C1 | the Mac |

**Wave 1** (parallel after step 0): N native shell, V voice, P playback on
the Mac; S1 sign-in and S2 audio in Texture on the build VM.

**Wave 2:** R native renderer, D the Autoradio desk, U voice as input, X the
push relay. M (subscription) waits for the paid account, because store
purchases need it.

**Wave 3:** the review notes and privacy labels.

## How the work runs

- Server builders compile on the build VM. The Mac is for iOS.
- iOS builds run outside the repo's Nix dev shell.
- Landings go through `scripts/land` once it is on main.
- Review: Gemini 3.8 flash through `agy`, Sonnet as the second seat for the
  red lanes (S1, S2, D, U).
- Each acceptance item gets a tracked proof under `scripts/proofs/` or
  `frontend/tests/`, with ids taken from earlier output.
- The staging identity is a fresh passkey account made by the deployed spec
  fixtures.

## Residuals carried in from Autopaper

Not blocking this mission; each is recorded in the overnight ledger.

- Label quality after intake batching is not measurable from the host.
- The overflow split sends none to River.
- The Devin pacer has not run a full paced hour.
- The markerless restart path is shown only in tests.
- The rename of retired names in code.

## Decisions for the owner

Each runs on its default until answered.

| Question | Default |
| --- | --- |
| Which voice mode is the default | decided by the owner's QA session; button, wake word, open and the hybrid are all built (owner, 2026-11-10) |
| Wake phrase | "Hey Choir", with "Okay Choir" and bare "Choir" in the test |
| Hybrid window length | five minutes once the voice is learned, 30 seconds before |
| Is subscription (lane M) in pass 1? | no; it needs the paid account |
| A branch build on d@e before main | built alongside wave 1, for the server lanes |
