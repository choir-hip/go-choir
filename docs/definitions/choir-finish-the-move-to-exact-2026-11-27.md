---
definition_version: 7
readiness: executable
authority: 'the owner, 2026-11-27: "docs transform precedes into sdlc improvements, as that is the next code thing, then exact cutover finishing and android launching"; "Finishing the exact cutover is 1 day at agentic speed"; "we should plan to update to upstream before the cutover finishes"; "we should expand what can be done signed out. All the local on device stuff doesn''t care about you having an account". The rule itself is CLAUDE.md, Product: "The phone app is Exact; only voice is native" (owner, 2026-11-18 and 2026-11-20).'
review:
  reviewer: none (the owner's statements are the authority; each red station is reviewed on its own)
  frozen_ref: none
  verdict: none
  evidence_ref: none
start:
  captured_at: 2026-11-27T13:50Z
  source:
    canonical_ref: origin/main 5844447d4
    deploy_identity: staging on 323df6f3b, read from the x-choir-build-commit header of https://choir.news at 2026-11-27T13:40Z
  what_is_true_now:
  - 'Measured on main by scripts/check-autoradio-native-boundary on 2026-11-27: non-voice native code is 34 files and 3,375 lines, and the gate holds that as its ceiling. The largest are ChoirClient.swift (439 lines), RadioProgramme.swift (288), ChoirApp.swift (252), BarState.swift (235).'
  - 'What is not in Exact, from docs/evidence/exact-move-facts-2026-11-27.md: sign-out and the voice turn''s server client (ChoirApp, ChoirClient, ChoirSignIn, AccountWipe); the mail seal (MailSeal.swift, which the owner ruled on 2026-11-24 goes); bar and run-sheet remnants (BarState, SegmentTracker, RadioProgramme, SavedAnswerHolder, Conversation); reader, notebook and take helpers. Sign-in itself is already Exact.'
  - 'The pin is mobile/exact.pin f5334750 of 2026-11-21. Upstream read on 2026-11-27 was 1,266 to 1,366 commits ahead. Of the three patches we apply, 0001 (the editor hides Markdown syntax) still applies; 0002 (Android shared libraries in the APK) and 0003 (Media3 for background playback) do not, because scripts/agent-android.mjs changed upstream.'
  - 'Signed out today the paper, stories and the radio reading the public front page work; no turn completes. TurnGate.swift refuses a turn with no key (read at HandsFree.swift:304 and LabRuntime.swift:299). The owner ruled on 2026-11-23 that signed out is listening only, and on 2026-11-27 that everything on the device should work signed out; the newer ruling governs.'
  - 'Lanes that look unlanded and belong here (read-only on 2026-11-27, not verified by building): voice-models (21 commits, 95 files), voice-more (4 commits, 121 files: Downloads, 53 voices, a signed-out first run), unified-voice-download (183 files and 22 uncommitted paths), exact-rest (the mail seal deletion), exr-r (editor read-only and save fixes, 12 uncommitted paths), account-boundary-l1 (65 files), and what is left of move-b.'
  - 'Autoradio 1.0 was approved on 2026-11-25 and has no users yet. No Swift runs in CI; the job Autoradio Shared-Code Tests runs bun test when the classifier sets mobile. Native builds, simulators and device builds are the lead''s, one at a time through the build lock.'
  - No record exists of the locked-phone test ever being run.
  - 'Correction, 2026-11-27T14:40Z: the line above that the owner ruled on 2026-11-24 that the mail seal goes has no source. No owner words about it are in docs/inbox.md, the problem record or the change log; the only record is the subject of an unlanded commit (2a971bbd8, "mail stored plain by shared code"), and docs/changes.md:406 says the seal stays until the mail files'' identity is redesigned, after four review rounds. Station E4 waits on decision D4.'
  worktrees:
  - path: .lane/trees/inbox-notes
    status: clean
    class: goal_candidate
    owner: the lead of this session
    touch: goal_owned
    recovery: lane rm inbox-notes after it lands
  - path: the lanes named above and every other lane under .lane/trees
    status: unknown
    class: other_agent_wip
    owner: other agents and the lead of other sessions
    touch: forbidden
    recovery: leave in place; a lane is taken up only by a station of this file that names it, after reading its uncommitted paths and never discarding them
finish:
  deliver: Every part of the phone app that is not voice is written once in Exact, on a current Exact, and a person with no account can ask the phone a question and hear an answer from the model on the phone.
  artifact: on main, the pin at a current upstream commit with the patches that still apply; the non-voice native code removed or moved to shared code; the turn gate replaced by a rule that lets an on-device turn finish signed out; a build the owner can install
  acceptance:
  - action: run scripts/check-autoradio-native-boundary on origin/main
    proves: non-voice native code is gone or is a short list each line of which states the platform reason beside it
    evidence_class: the gate's count on main
  - action: read mobile/exact.pin and run the shared-code tests in CI on that commit
    proves: the app is on a current Exact and its shared tests pass there
    evidence_class: a CI run on main
  - action: on the owner's phone, signed out, ask a question aloud and hear the on-device model answer, with the network off
    proves: a turn completes on the phone with no account and no server
    evidence_class: the owner's verdict on a device build
  - action: with the phone locked for ten minutes while the radio plays, read the app's own log for the shared code's ticks
    proves: whether shared code keeps running on a locked phone while a playback claim is held
    evidence_class: a device log from the lead's build
  - action: for each lane named in the start receipt, read main for its work
    proves: each is landed, folded into a station here, or dropped with a stated reason, and none is left ambiguous
    evidence_class: this file's stations and the commits they name
  rollback: each station is its own commit and reverts alone; the pin move reverts by restoring mobile/exact.pin and the patch files; version 1.0 stays on the App Store whatever happens here
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci]
value:
  better_means: the lines of app logic that exist twice or only in Swift go to zero, and what a person can do with no account goes up
  goodharting_would_be: 'the gate''s count lowered by adding files to the voice list; Swift deleted with its behaviour lost and no shared code in its place; a signed-out turn that quietly calls the server; a pin moved with the Android patches dropped; a lane called landed without reading its uncommitted paths; stations marked done at a builder''s commit and not on main with green CI'
homotopy:
  realism_axis: 'one page at a time, each moved whole: the shared code is written fresh (a moved page carries nothing over from the Swift build), the Swift is deleted in the same change, and the ratchet in the native-boundary gate is lowered in that change'
boundaries:
  mutation_class: orange for moved pages; red for sign-out, the server client, account wipe and the turn gate (auth and session)
  authority_sources:
  - CLAUDE.md, Product and Safety and hygiene
  - the owner's words of 2026-11-18, 20, 23, 24 and 27, the last in docs/inbox.md
  - docs/reference/EXACT.md and docs/reference/exact/, read before any Exact change
  - docs/evidence/exact-move-facts-2026-11-27.md, for the measurements only
  must_preserve:
  - only voice is native; no new logic in mobile/AutoradioKit or mobile/exact-package/apple outside voice, and a new native call carries its reason beside it
  - a page moved to Exact carries nothing over from the Swift build and never reads the old native file
  - one Mac build at a time, run by the lead through the build lock; a lane runs only the Exact loop that needs no native build
  - every simulator is slim and is shut down when the run ends
  - 'each red station gets its own review by two seats and its own deployed proof: it goes through scripts/land-queue --serial, never in a batch with another red station'
  - a lane's uncommitted paths are read and kept before the lane is taken up; no other agent's work is reverted or discarded
  - no secret, token or owner address in any file or output
  excluded:
  - Android beyond keeping its build working across the pin move (its mission is docs/definitions/choir-autoradio-on-android-2026-11-22.md)
  - the look of the app, the deeper on-device models and the onboarding interview, which are the next mission of version 1.1
  - takes, social and a way to pay (version 1.2)
  - any submission to the App Store, which is the owner's act
  protected_surfaces:
  - auth and session (sign-out, the stored key, account wipe)
  - the turn gate
stations:
- id: E0
  name: the facts on main
  scope: this file, docs/evidence/exact-move-facts-2026-11-27.md, the campaign and the registries
  class: green
  review: none
  depends_on: []
  carrier: '71aa6bee6'
  status: landed
- id: E1
  name: the pin moves
  scope: 'mobile/exact.pin to a current upstream commit by the recipe in docs/reference/exact/build-and-ship.md; patches 0002 and 0003 re-made against the new scripts/agent-android.mjs or shown to be unnecessary upstream; the grammar and package-interface diff read against our contracts first; docs/reference/EXACT.md updated in the same change'
  class: orange
  review: one seat
  depends_on: [E0]
  carrier: 'b32112d53 (the pin at 8552d20f9, patches 0002 and 0003 re-made, Android locks rewritten, the web toolchain in setup-exact), CI green; verified in the build tree on 2026-11-28: contract build, bun test 1,841 pass, test web with the two failures main has, the iOS compile, the Android lab-one build; nothing driven on a simulator or a phone'
  status: landed
- id: E2
  name: what counts as voice
  scope: 'a list, for the owner to rule on, of the files the voice list covers that the audit of 2026-11-24 says could move (turn policy, model configuration and choice, setup, the server-turn stages ChoirDesk and AutoradioDesk); until the ruling the list stands as it is'
  class: green
  review: the owner
  depends_on: [E0]
  carrier: 'docs/evidence/what-counts-as-voice-2026-11-27.md, on main; waits on the owner (D2)'
  status: landed
- id: E3
  name: the lanes in flight
  scope: 'voice-models, voice-more, unified-voice-download, exact-rest, exr-r, account-boundary-l1 and the rest of move-b: each read, with its uncommitted paths, and given one verdict: land as it is, fold into a station here, or drop with the reason'
  class: green for the verdicts; each landing states its own
  review: one seat per landing
  depends_on: [E0]
  carrier: 'docs/evidence/phone-lanes-verdicts-2026-11-27.md: no lane lands as it is (every one conflicts with main); the mail seal deletion folds into E4, the Imported and Mail module deletions and the clock into E5, the one reader of signed in into E6, the first run into E7; the trial gate and the pinned voice packs go to the next mission; the editor read-only fix needs its own landing'
  status: landed
- id: E4
  name: the mail seal goes
  scope: MailSeal.swift deleted with what calls it, from lane exact-rest (owner, 2026-11-24)
  class: 'red: it ends encryption at rest of a person''s mail on the phone'
  review: the owner first (D4), then two seats
  depends_on: [E3]
  carrier: none yet
  status: pending
- id: E5
  name: the bar, the run sheet and the helpers
  scope: 'BarState, SegmentTracker, RadioProgramme, SavedAnswerHolder, Conversation, HeldPages, ReaderFollow, EditionSetup, Takes and the small AutoradioModule extensions: each moved to shared code or deleted where shared code already does the job, with the ratchet lowered in the same change'
  class: orange
  review: one seat
  depends_on: [E1, E3]
  carrier: '7abe41baf (SegmentTracker removed), 6d0ff7ed7 (the mail approval link and the Imported page ask Swift for nothing; Activity''s clock is Exact''s date) and 97dea4262 (test-only programme types removed; CI run 46025608432) and f0509f6a7 (16 unread RadioInfo fields deleted; CI run 46042223854): 32 files, 3,082 lines; staging serves it; seat 1 accepted each at its last round. Parked after four review rounds: origin/lane/bar-facts (the radio''s sentence follow in shared code, to 2,951 lines), see docs/evidence/bar-facts-lane-parked-2026-11-27.md. Owed on a device: the mail approval link opens, Activity shows ages, a spoken entry speaks and the lock screen shows its title. The app version is still read from Swift.'
  status: working
- id: E6
  name: sign-out and the server client
  scope: ChoirApp, ChoirClient, ChoirSignIn and AccountWipe moved to shared code or deleted; the voice turn reaches the server through the shared client
  class: red
  review: two seats
  depends_on: [E1, E3]
  carrier: 'E6.3 (the order of sign-out) landed as 4f5e66020, CI run 46055581016, staging serves it; accepted by two seats on the fourth review, after the assessment in docs/evidence/sign-out-gate-three-rounds-2026-11-27.md; not run on a device. E6.4''s gate (scripts/check-native-stores, every native persistence site named with its sign-out ruling) landed as dfe978cb7, CI run 46059825133; its inventory is docs/evidence/native-stores-inventory-2026-11-27.md. E6.2 (one native reader of signed in) landed as 48dde9e32 after the app was compiled with it and both seats accepted. The set-aside deck journal now goes at sign-out on iPhone (824cc0011) and Android (5a33b90b9). E6.1 waits on D7; E6.5 to E6.7 wait on D6'
  status: working
- id: E7
  name: a turn with no account
  scope: 'the turn gate replaced: signed out, a question is answered by the model on the phone and nothing is sent to the server; signed in, as today'
  class: red
  review: two seats
  depends_on: [E6]
  carrier: none yet
  status: pending
- id: E8
  name: the locked phone
  scope: ten minutes locked with the radio playing on the lead's device build, the shared code's ticks read from the app's log; recorded whichever way it comes out
  class: green
  review: none
  depends_on: [E1]
  carrier: none yet
  status: pending
- id: E9
  name: a build for the owner
  scope: a device build of main after E4 to E7, handed to the owner with what to try; the submission is the owner's
  class: green
  review: the owner
  depends_on: [E4, E5, E6, E7, E8]
  carrier: none yet
  status: pending
decisions_for_the_owner:
- id: D1
  question: Does this file take over the phone stations of docs/definitions/choir-exact-cutover-and-ontology-2026-11-20.md (X0, the pin; X9, the device probe; and what remains of X1 to X8)?
  why: Two files would otherwise both claim the same work, which is how missions have been left open.
  options: this file takes them and the older file keeps only the ontology work; the older file stays the working one and this is withdrawn; both stay
  default: this file is the working one for the phone; the older file is not edited until the owner rules
- id: D2
  question: Which files on the voice list are really voice (station E2)?
  why: The gate counts only what is off that list, so the list decides what "finished" means.
  options: the list as it is; the narrower list from the audit of 2026-11-24; a file-by-file ruling
  default: the list as it is
- id: D3
  question: Does the signed-out turn replace the ruling of 2026-11-23 that signed out is listening only?
  why: E7 reverses that ruling on the strength of the words of 2026-11-27.
  options: yes, any on-device turn works signed out; yes, but only once the on-device model is downloaded by the person's choice; no
  default: 'yes, on the 2026-11-27 words, and nothing is sent to the server signed out; E7 is not built until E6 has landed, which leaves time for the owner to say otherwise'
- id: D4
  question: Does the mail seal go, so that the phone keeps its copy of a person's mail unencrypted by the app (the phone's own file protection still applies)?
  why: 'The seal is 72 lines of Swift and Android owes a Kotlin copy, so removing it serves the rule. It is also the only encryption the app adds to mail at rest. An unlanded commit removed it and I can find no words of the owner asking for that.'
  options: the seal goes and mail is stored plain under the platform's file protection; the seal stays as the one native cipher, with its reason beside it, and Android gets its copy; mail is not kept on the phone at all
  default: the seal stays; E4 is not built
- id: D5
  question: May the build tree (lane qa-mac) be moved to the new Exact pin while it holds the Android session's uncommitted work?
  why: The move resets the one Exact checkout and re-applies the patches; that session edited patch 0003 itself, and the re-made 0003 differs from its edit.
  options: the Android session commits its work first and then the tree moves; move it now and keep its 38 paths as they are; name a second build tree for the pin move
  default: the tree is not moved; the pin waits on a branch
- id: D6
  question: Where is the account key held on the phone?
  why: 'The key never expires and carries every scope. Today it is in the platform''s secure store and both native code and shared code can use it. The design (docs/design/phone-sign-out-and-server-client-in-exact-2026-11-27.md, section 2) finds that keeping it native-only does not hold up, because sign-in already receives it in shared code and Android''s native side cannot read Exact''s secret store.'
  options: 'shared code''s secret store is the only place it rests, and it is pushed once into native memory for the live voice turn (the design''s recommendation: one path on both platforms); it stays as today, in two hands; the server issues short-lived keys first, which is server work'
  default: 'as today; only the parts of E6 that do not depend on the answer are built (the order of sign-out, one reader of signed in, the inventory of native stores, deleting old-build code)'
- id: D7
  question: May the launch-time revoke of an old Swift build's key be deleted?
  why: 'E6.1 was to delete LegacyKey, its launch task and ChoirSignIn.revokeNow (about 30 lines of non-voice Swift). A builder found on 2026-11-27 that nothing else revokes the full-access key an old Swift build left in the Keychain: shared code never reads that Keychain service and its pending-revoke list holds only its own sign-outs. The only installs of an old build are the owner''s own devices. If the owner''s phone has launched a current build once with the network up, the old key is already revoked and the code is dead; that cannot be read from here.'
  options: 'the owner confirms their devices have run a current build (or revokes old keys in the account page), and the code is deleted; it stays'
  default: 'it stays. The rest of E6.1 came to two lines (a wipe step also done at every launch); one of two reviewers sent that back as cheap protection worth keeping, so it was dropped and nothing of E6.1 landed'
owner_rulings:
- on: 2026-11-27
  decision: order
  said: "docs transform precedes into sdlc improvements, as that is the next code thing, then exact cutover finishing and android launching."
- on: 2026-11-27
  decision: effort
  said: "Finishing the exact cutover is 1 day at agentic speed. 8 hours really. Faster if I'm focused but I don't need to allocate much human attention to it. Just decision and instruction framing."
- on: 2026-11-27
  decision: upstream
  said: "we should plan to update to upstream before the cutover finishes."
- on: 2026-11-27
  decision: signed out
  said: "we should expand what can be done signed out. All the local on device stuff doesn't care about you having an account. Like, the account is for the choir computer, which free users don't have."
  read_as: an on-device turn completes with no account; anything that needs the person's computer still needs sign-in.
now:
  status: working
  slice: E1 landed (b32112d53); E5 and E6 continue; the next moves are ranked in the iOS and Android difference audit
  source_ref: origin/main b32112d53
  deploy_identity: staging on b32112d53
  candidate:
    id: none
    state: none
    ref: none
    base: none
    digest: none
    scope: []
  conjecture:
    id: EX-C1
    calibration_claim: 20261127T1350Z-non-voice-native-under-a-thousand-lines
    claim: 'the move finishes fast once it is framed: non-voice native code on main, by the gate''s own count, falls from 3,375 lines to 1,000 or fewer within three days'
    test: scripts/check-autoradio-native-boundary on origin/main at the settle-by time
    edge: resource
    delta_o: 'the count per landed station, which shows whether the time goes to moving code or to the lead''s one native build at a time'
    scope_if_supported: the non-voice native code only; nothing about the signed-out turn or the locked phone
    status: active
    evidence_refs: []
  decision:
    what: one page at a time, moved whole, Swift deleted in the same change
    kind: operational
    status: settled
    evidence_ref: CLAUDE.md, Product
    owner_ratification_ref: not_applicable
  belief:
    believed_state: 'three slices of E5 are on main and deployed: 3,375 lines became 3,127. A builder who read Conversation, SavedAnswerHolder, EditionSetup and Takes found them to be the voice turn''s own state, written while a turn runs, with their page logic already in shared code; they are not on the voice list, which is the owner''s to change (D2). Counting those, RadioProgramme''s speech blocking and what stays of BarState as voice, the non-voice code that can still move without the red stations is small; most of the rest is ChoirClient, ChoirApp and AccountWipe (E6). Nothing has run on a device and the pin has not moved.'
    main_uncertainty: 'whether the owner counts the voice turn''s state as voice (it decides whether the gate can reach zero without rewriting the turn), and whether the build tree may move'
    next_observation: 'the contract build and bun test on the new pin, once the build tree may move'
  blocker_or_risk: 'The app-side Swift has no compile check that runs without a person: main did not compile as an iPhone app from ce79438a8 until e25afe05f, 31 hours (docs/problems/main-broke-the-iphone-app-build-for-a-day-and-no-check-saw-it-2026-11-28.md); the owner is asked which check closes it. The build tree was moved to main on 2026-11-28 on the owner''s word to land the Android work (its old state is on branch deck-build 4b46ca522); D5 is answered by that. Nothing of K3 has run on a device or a simulator.'
  next_action: 'the next E5 slices and the moves ranked in the iOS and Android difference audit (model catalog and selection to shared code wait on D2); device checks are owed for everything landed'
receipts: []
---

# Finishing the move to Exact

The phone app is Exact and only voice is native. This mission finishes
that on a current Exact, and lets a person with no account ask the phone a
question and hear the phone's own model answer.

Run it with `/goal docs/definitions/choir-finish-the-move-to-exact-2026-11-27.md`.

The measurements it starts from are in
[the facts file](../evidence/exact-move-facts-2026-11-27.md). They were
read, not built or run.

## Why a new file

The move began on 2026-11-18 and the file that carried it
([the cutover and the ontology](choir-exact-cutover-and-ontology-2026-11-20.md))
also carries the ontology work, and its station list has not kept up with
what shipped. This file holds only what is left for the phone. Decision D1
asks the owner whether it takes those stations over.

## A station is done when

Its commit is on main and the CI run on it is green. A builder's commit in
a lane is not done. Red stations go one at a time with their own deployed
proof; the others may land as a batch.
