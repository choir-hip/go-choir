---
definition_version: 7

readiness: executable

review:
  reviewer: none
  frozen_ref: none
  verdict: none
  evidence_ref: "none; the owner's words of 2026-11-17 (about 22:30 to 23:10 local) are the authority to start: 'the key thing is for the overnight work to be all about the learning engine', 'let's just build everything'. Each server candidate is reviewed at its tier before landing; phone builds are judged by the owner on the device."

start:
  captured_at: 2026-11-18T07:15Z
  source:
    canonical_ref: origin/main 3c887d1b3
    deploy_identity: staging deployed_commit 90f258bb4
  worktrees:
    - path: /Users/wiz/Dev/go-choir
      status: dirty
      class: user_wip
      owner: owner
      touch: forbidden
      recovery: leave in place (one untracked env backup that holds secrets)
    - path: /Users/wiz/Dev/go-choir/.lane/trees/qa-mac
      status: dirty
      class: generated_temp
      owner: lead
      touch: goal_owned
      recovery: the one build tree, detached at lane/autoradio-integ9 49a097859; generated files are checked out again before a switch
    - path: lanes learn-settle, learn-design, clip-fingerprint, clip-relocate, voice-coexist, radio-page2, files-open, autoradio-integ9
      status: unknown
      class: other_agent_wip
      owner: builders
      touch: read_only
      recovery: each lane is its own branch on origin
  known:
    - "Forecast is built and deployed: the DJ writes p_heard and caused_by on each story, update and headlines segment, and the server writes one claim (record kind autoradio.forecast) before sending the segment (internal/agentcore/api_autoradio_programme.go, internal/calibration/autoradio.go)."
    - "Outcome is half built: the phone posts aired and skip ops with an outcome and queues them offline, but DJ entries are never placed on the server run sheet, so the server most likely drops those ops as unknown_id. Read from code on 2026-11-17, not yet seen on a tape."
    - "Settlement is written and tested with no caller (calibration.SettleAutoradio). Nothing reads outcomes. Nothing writes a lesson. The programme input reads autoradio.lesson records that have no writer."
    - "The listener's edition is a topic list in a file on the phone. It never reaches the server; the programme request accepts edition_text and the phone sends none. Candidate order is the public front page."
    - "No take exists: no recording, no published voice object, no publisher track record."
    - "The phone runs lane/build-one-place dc6824fd8. The next candidate lane/autoradio-integ9 49a097859 (voice and microphone, clip playback from fetched bytes, the page changes, the file fixes) is building in the simulator and is not on the phone. No phone or Mac code is on main."
    - "The app is signed with a free personal team whose profiles last seven days. TestFlight needs the owner's paid developer programme team."

finish:
  deliver: Autoradio learns from the owner's listening. The DJ forecasts whether the owner will hear each segment through; the owner's listening settles it; what is learned is one short document the owner reads and edits and the DJ reads before the next stretch. The owner can record a take, publish it, and have it offered by another listener's DJ under the same forecast and settlement. The build is ready to submit to TestFlight.
  artifact: "One Texture document per listener (the notebook), at most about one page, holding what the listener asked for, a short list of lessons with counts, and what the DJ is trying; a readable ledger of forecasts, outcomes and settlements beside it; the programme reading that document; takes as published objects with a public track record; a phone build carrying the document page, steering, take recording and the evening's fixes."
  acceptance:
    - action: "scripts/prove-autoradio-learning on the drill computer: open a programme, extend once, post ops as a phone would (finish three entries, skip two, leave one), read the scorecard"
      proves: five forecasts settle by rule (three true, two false), one event has no verdict, and a repeat of the same ops settles nothing twice
      evidence_class: deployed proof
    - action: after that run, read the listener's document revisions and the next stretch's programme input on the drill computer
      proves: a new revision of the document exists that rests on those settled rows, it is within the page budget, and the next stretch's model input contains it
      evidence_class: deployed proof
    - action: on the drill computer, send a steering turn (more of this, less of that) while an entry is on air, then read the document
      proves: the listener's own words are a line in the document, quoted, with its referent, and outrank inferred lessons
      evidence_class: deployed proof
    - action: the listener's read routes return every record kind held about the listener (forecasts, ops, settlements, lessons, the document)
      proves: nothing is held about the listener that the listener cannot read (learning engine rule 2)
      evidence_class: deployed proof plus code read
    - action: a take recorded on the owner's phone is published, appears in the public takes list, is offered as a candidate to the drill computer's programme, is played with a forecast, and its settlement appears in the publisher's track record marked as drill data
      proves: the take path works end to end and drill acts never count toward a real record
      evidence_class: device plus deployed proof
    - action: the owner's pass on the phone (the document page, a steering phrase, record and publish a take, the evening's fixes)
      proves: the learning output is visible and editable by the listener, by the owner's eye and ear
      evidence_class: human inspection
    - action: an archive build signed with the owner's paid team uploads to App Store Connect and is accepted for TestFlight processing
      proves: the app can reach testers
      evidence_class: vendor receipt
  rollback: every server change reverts by commit; settlement records are append-only and a wrong rule is corrected by a new rule version, never by rewriting; the listener's document is a Texture revision chain and any revision can be restored; the phone keeps the last build the owner accepted.
  landing:
    required: true
    environment: staging (https://choir.news) for every server station; phone work lands on main after review and is proven on the owner's phone
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: "the DJ's forecasts separate what this listener hears through from what they skip better than the base rate does, and the reason is written in a page the listener can read, correct and recognise as true of them."
  goodharting_would_be: "a document that grows a line per segment; lessons that restate the base rate or tell the DJ which way to move a number; forecasts settled by anything the DJ or the programme route can write; a good Brier score from always picking safe stories; drill listening counted in a publisher's record; a scorecard the listener cannot open."

homotopy:
  realism_axis: "from the drill computer posting ops as a phone would, through the owner's own phone and listening, to invited TestFlight listeners hearing each other's takes."

boundaries:
  mutation_class: "red for stations 1, 2, 4 and 5 (tape, settlement, Texture canonical writes, model-call input) and for the server side of 7; orange for 3 and 6 and the phone side of 7"
  authority_sources: ["the owner's messages of 2026-11-17", CLAUDE.md, docs/learning-engine.md, docs/choir-doctrine.md]
  must_preserve:
    - "rule 1: the DJ never settles its own forecast and the programme route never writes an outcome; only the listener's device-keyed act settles"
    - "rule 2: every record held about a listener or a publisher is readable by them"
    - "the document stays within its page budget; per-segment rows live only in the ledger"
    - "one mechanism for evidence: the run sheet op log is the air log; no second heard route"
    - "three outcomes: heard through is true; skipped or cut in on is false; closing the app, sign-out, a crash or a failed entry is no verdict, recorded with its position"
    - "Texture is the only writer of the document; the Autoradio desk owns the ledger and asks Texture for each revision"
    - "no extra model call where the stretch call can carry the question; no limits on our own model calls"
    - "(struck 2026-11-18: 'mail and local-only files never leave the phone' was the lead's boundary, never the owner's; the owner: 'there is no rule that local files can't go to server')"
    - "drills on the drill computer only; never the paper or the owner's computer"
    - "no switches: each station ships on"
  excluded: [importance and coverage scoring, a server vector store, the master stream, opening takes to anyone beyond invited testers, a semantic index by category]
  protected_surfaces: [the tape, autoradio settlement records, Texture canonical writes, the programme model call input, published objects, auth and session]

stations:
  - id: L1
    name: Outcomes reach the server
    scope: "the programme route places each DJ entry on the listener's server run sheet through the existing reducer, carrying its forecast's claim id; the segment event carries the same ids; idempotent on replay"
    class: red
    review: two seats
    depends_on: []
    carrier: lane/learn-settle
    status: working
  - id: L2
    name: Settlement by rule
    scope: "an applied aired or skip op on an entry with a claim id settles that forecast once, by rule, using the existing settler; the three outcomes; late ops recorded and not settled; a crash is not recorded as the listener leaving"
    class: red
    review: two seats
    depends_on: [L1]
    carrier: lane/learn-settle
    status: working
  - id: L3
    name: Scorecard and ledger
    scope: "a read route for the listener: counts, heard-through rate and Brier by segment kind and by cause, and the ledger rows; scripts/prove-autoradio-learning"
    class: orange
    review: one seat
    depends_on: [L2]
    carrier: lane/learn-settle
    status: working
  - id: L4
    name: The document and its writer
    scope: "design sections 2 and 3: one Texture document per listener titled My notebook (editiondoc.Title changes; My edition is adopted, not duplicated); four sections (what you asked for, learned at most 12 lines, trying at most 3, recent steering at most 10 taps); the line grammar with [L<n> · heard h of m] tags, counts computed from the ledger by rule; target about 400 words, hard cap 1,000 words for desk sections; the DJ proposes notebook lines in the extend call, cites only settled rows it was shown, must name what it displaces when a section is full; the Autoradio handler appends an autoradio.notebook_patch record and calls textureowner.ApplyNotebookPatch, which writes one AuthorAppAgent revision labelled app:autoradio with no model call; listener edits detected by revision author and recorded as strikes; a struck line is never rewritten from the same cites; first writer of autoradio.lesson"
    class: red
    review: two seats
    depends_on: [L2]
    carrier: "design on lane/learn-design (docs/design/autoradio-learned-document-2026-11-17.md), then a builder lane"
    status: pending
  - id: L5
    name: The DJ reads the document
    scope: "design section 4: the notebook replaces the lessons and edition sections of the stable prefix, each line shown with its ref; marked cases (the last 40 settled forecasts) in the volatile part; caused_by accepts lesson:L<n>, try:T<n>, mine:<8 hex>, tap:<entry id> resolved against that call's notebook, and claims carry a line:<ref> tag, so a settlement moves that line's count at the next revision; edition_text is deleted from the request; candidate order stays the phone's front-list order (re-ranking by the notebook is deferred: it needs another call or an index)"
    class: red
    review: two seats
    depends_on: [L4]
    carrier: not started
    status: pending
  - id: L6
    name: The phone
    scope: "design sections 6 to 8: one page with the notebook (editable as a Texture), the scorecard and the ledger rows behind a tapped line; more and less taps posted to POST /api/autoradio/steer and shown under recent steering; spoken steering quoted verbatim into what you asked for by the next extend call; the onboarding topic list written once as the first lines of what you asked for (an AuthorUser revision); a crash is never synthesised as left; consolidation of the listener's lines only on their Yes; promotion of a lesson with at least 10 settled rows to what you asked for on their Yes"
    class: orange
    review: one seat
    depends_on: [L3, L4]
    carrier: not started
    status: pending
  - id: L7
    name: Takes
    scope: "record on the phone, review, publish as an object in the Autopaper object graph with a transcript and the story it responds to; a public takes list read into candidates; a take segment that plays the publisher's audio with a forecast and settles through L2; the publisher's public track record across listeners, with drill acts marked and never counted; mute a publisher, delete a take, bounds on what strangers can cause"
    class: red
    review: two seats on the server side, one on the phone side
    depends_on: [L2]
    carrier: "specification: design section 11, pushed first at origin/lane/learn-design 4be932ad5; then a server lane and a phone lane in parallel"
    status: working
  - id: L8
    name: The evening's fixes reach the phone and main
    scope: "voice on must hear and must not pause; yield the microphone to other apps; podcast clips located by fingerprint and played from fetched bytes (server fingerprint lands first); the opening's time computed at open; story references tappable; tray glyphs; files open; review and land the mobile lanes on main"
    class: orange
    review: one seat (two for the clip server change)
    depends_on: []
    carrier: lane/autoradio-integ9 49a097859; lane/clip-fingerprint
    status: working
  - id: L9
    name: TestFlight
    scope: "the list under 'TestFlight this week' below: paid team, App Store Connect record, distribution profile and owner acts; privacy manifest, encryption declaration, version and build number, a sign-in a new tester can complete, account deletion; usage strings, background audio and icon are present; an archive that uploads"
    class: orange
    review: one seat
    depends_on: [L6, L7, L8]
    carrier: not started
    status: pending
  - id: L10
    name: Android experiment
    scope: "after the owner unplugs the iPhone (about 05:45 local on 2026-11-18): see whether Exact builds the app for Android and runs on the owner's old Android phone (broken screen, mirrored on the Mac); an experiment in how far Exact for Android goes, not a product build; on-device models are not expected to fit; read docs/reference/exact first and record what works in a reference page"
    class: orange
    review: none until something is kept
    depends_on: []
    carrier: not started
    status: pending

decisions_for_the_owner:
  - id: D1
    question: Does closing the app during a segment count against it?
    why: it changes what the DJ learns from the most common way a session ends
    options: no verdict; counts as not heard; counts only when it happens early in a long segment
    default: no verdict, recorded with the position, revisited when there is data
  - id: D2
    question: What is the document called for the listener?
    why: the theory calls it the notebook; the phone needs one name
    options: the notebook; My edition; another name
    default: "the notebook; the Texture title is My notebook, and an existing My edition document is adopted under the new title"
  - id: D3
    question: Which Apple developer team and bundle id go to TestFlight?
    why: only the owner can enrol, create the app record and accept Apple's terms
    options: the owner's paid team with news.choir.autoradio; another id
    default: none; this is an owner act and blocks only L9
  - id: D4
    question: Who may hear a take in the first TestFlight build?
    why: there is no moderation beyond mute and delete
    options: invited testers only; anyone signed in
    default: invited testers only
  - id: D5
    question: What does a publisher with no record get?
    why: a new publisher cannot earn distribution without being offered once
    options: a small number of first offers; none until someone follows them
    default: "a trial: a take with fewer than 3 counted outcomes is always among the phone's 3 take candidates while it is listed; the DJ forecasts it on the transcript, cause explore when nothing else decided it (design 11.6)"

  - id: D6
    question: Who is the second review seat while seat 1 is out?
    why: "CLAUDE.md blocks tier-2 review until the owner names a replacement; L1, L2, L4, L5 and the L7 server side are red and need two seats"
    options: a named replacement model; land red stations on one seat with the owner's word; wait
    default: "wait for the named seat; meanwhile red candidates are built, reviewed by seat 2, frozen and proved on the drill computer from the lane, and do not land on main"

owner_rulings:
  - on: 2026-11-17
    decision: the mission
    said: "the key thing is for the overnight work to be all about the learning engine... what do we need to do to really make it so that Autoradio learns from my usage of it? ... The output's supposed to be a document, a prompt... for users, we want to have a canonical representation as a document. And we need to be aligned with the four-stroke learning engine theory documentation that we have."
  - on: 2026-11-17
    decision: D1, L2
    said: "the DJ is predicting whether you will listen to a segment in full, the alternative being you could skip it, or you could close the app entirely. And, well, maybe closing the app counts as a skip or not. We'll have to think about that."
  - on: 2026-11-17
    decision: L6
    said: "there's also driving the auto radio with voice, saying more of this, less of that, that kind of thing. Yeah, we should definitely be learning from those."
  - on: 2026-11-17
    decision: L4
    said: "we have to be careful with adding a line in the document for every predicted segment... We need to keep the size of the human readable document to less than 5 pages, ideally 1 page."
  - on: 2026-11-17
    decision: L7, L9
    said: "you as a user could earn distribution on your takes if another user's AI DJ predicts that user will listen to your take in full... you'll build up a track record which carries over across users. ... I do want the takes, because basically my goal is to get this on test flight this week. And so yeah, let's just build everything."
  - on: 2026-11-18
    decision: L7
    said: "The social nature of takes is intentional... We are just going to use an honest reporting system... we are going to cross that bridge of, like, what happens when someone tries to make 1,000 accounts to all listen to their takes to get a higher reputation... we'll deal with that later. Like, that's going to be a good problem to have... as soon as the reputation on choir is a valuable currency, then we're de facto in the realm of auto capital."
    read_as: "takes use honest reporting: each listener's computer reports its own outcome and the platform trusts it. No anti-fraud or anti-sybil mechanism is built in this mission; the ordinary bounds on what one account can cause stay. Rigging a record, up or down, is named as a known open edge that belongs to Autocapital."
  - on: 2026-11-18
    decision: D6
    said: "for the second review seat, use Sonnet subagents."
    read_as: "tier-2 review is DeepSeek v4.1 flash plus a Claude Sonnet subagent that did not write the change; red stations land when both accept"
  - on: 2026-11-18
    decision: L10
    said: "i want to see if we can build for android... exact for android is still a bit early compared to ios. thats really what i want to test out... the android stuff should wait to the morning, once i check in around 5:45 am and unplug my iphone."
    read_as: "no Android work before the owner's 05:45 check-in; until then the iPhone build comes first; Android building happens while the owner is out"

now:
  status: checkpoint_incomplete
  slice: "L1 to L3 (settlement) are landed, deployed and proven on d@e, with a follow-up; L7's server (takes) is landed, deployed and proven on d@e (ce719fe1a); L4 and L5 (the notebook) are on lane/learn-notebook in a repair round after both seats returned REPAIR; the phone build lane/autoradio-integ19 (014d71f7f) is installed on the owner's phone, unlaunched, and none of the phone lanes are on main"
  source_ref: "origin/main at the commit that carries this file"
  deploy_identity: "staging ce719fe1a at 2026-11-18T10:13Z"
  candidate:
    id: none
    state: none
    ref: none
    base: none
    digest: none
    scope: []
  conjecture:
    id: K1
    calibration_claim: C120
    claim: "When settled outcomes are written back as a short document of lessons that the DJ reads, the DJ's forecasts for this listener discriminate better than before: the lesson stroke works (learning engine, open question 8)."
    test: "on one listener, Brier and the gap between mean p_heard on heard and on skipped segments, for stretches written with the document against stretches before it, at least 60 settled forecasts on each side"
    edge: resource
    delta_o: "more listeners; until TestFlight there is one real listener and one drill computer, so the first reading is small and confounded with time of day and the day's news"
    scope_if_supported: one listener, the segment kinds that carry forecasts
    status: active
    evidence_refs: []
  decision:
    what: "the document is the canonical representation and Texture writes it; the DJ proposes lines inside the stretch call; the run sheet op log is the only evidence; build every station now and revise together"
    kind: architecture
    status: settled
    evidence_ref: this file, owner_rulings
    owner_ratification_ref: "the owner's messages of 2026-11-17"
  belief:
    believed_state: "two strokes of four run and are proven on the drill computer (forecast, outcome); the lesson stroke is built and not yet accepted by review; takes exist on both sides and have never met; trust is not built; no real listener has settled a forecast yet"
    main_uncertainty: "whether the phone's posted ops settle forecasts for a real listener as they do for the proof script, and whether a recording made by the phone passes the server's audio check"
    next_observation: "the owner's first listening session on integ19: the scorecard route on the owner's computer before and after (read only)"
  blocker_or_risk: "the phone lanes (about 150 commits) are not on main and were first reviewed on 2026-11-18 (docs/problems/autoradio-phone-code-unreviewed-for-three-days-2026-11-18.md); TestFlight needs the owner's paid team (D3); nothing about sound has been checked on the device"
  next_action: "none here. Mission incomplete; checkpoint landed. The owner cleared this goal on 2026-11-18 and opened docs/definitions/choir-autoradio-exact-cutover-2026-11-18.md, which carries the open work: K1 (the iPhone repairs), K2 (the notebook, stations L4 and L5), K3 (the phone work onto main). L8 (trust), L9 (TestFlight) and L10 (Android, now the cutover itself) are not done. Conjecture K1 (ledger C120) stays open."

receipts:
  - station: L1-L3
    what: "settlement: only the listener's phone op settles a forecast"
    landed: [f710dda6c, 53537c7ca, f54e92f48, de53cb778, 843ee5d80]
    ci: ["44257019990", "44265751276"]
    deployed: "f54e92f48 at 2026-11-18T07:26Z; 843ee5d80 at 2026-11-18T08:35Z"
    proof: "scripts/prove-autoradio-learning PASS on d@e, first run, twice (ledger C122, C123)"
    review: "seat 2 REPAIR then ACCEPT, seat 1 ACCEPT; follow-up seat 2 ACCEPT in round 2"
  - station: L7 (server)
    what: "takes: publish, list, audio, record, delete, outcome; a take as a programme candidate and a take entry on the run sheet"
    landed: [3f3cfdb7b, 9edf286b8, a1c3ac93a, ce719fe1a]
    ci: ["44280975897"]
    deployed: "ce719fe1a at 2026-11-18T10:13Z"
    proof: "scripts/prove-autoradio-takes PASS on d@e, first run, eight checks (ledger C124)"
    review: "five rounds: seat 1 ACCEPT in round 1; seat 2 REPAIR, REPAIR, ACCEPT, REPAIR (after the rebase onto settlement), ACCEPT"
---

# The Autoradio learning loop

The four strokes are in [docs/learning-engine.md](../learning-engine.md):
forecast, outcome, lesson, trust. Autoradio has the first. This mission builds
the other three for one listener, and then points the same engine outward at
takes.

**The loop in one line.** The DJ forecasts that the listener will hear a
segment through. The listener's act settles it. Settled cases become a short
document of lessons. The DJ reads the document before the next stretch.

**The document is the output.** It is one page the listener can read and edit,
and it is the text the DJ is given. It never holds a line per segment; those
rows are the ledger, which the listener can also read. A confirmation raises a
count. A new lesson has to displace or merge older ones.

**Who writes.** The student writes the lesson: the DJ, from the marked cases
it is shown, in its own words. The teacher is the listener's act, through a
rule. Texture holds the pen on the document, because Texture owns canonical
writes.

**Takes are the same engine.** Another listener's DJ forecasts whether that
listener will hear a take through. Their act settles it. The sum of those
settlements is the publisher's track record, readable by the publisher, and it
is what earns distribution.

**Everything is built at once and revised together** (owner, 2026-11-17). The
stations run in parallel where their dependencies allow. The earlier design,
[autoradio-learning-engine-2026-11-14.md](../design/autoradio-learning-engine-2026-11-14.md),
stands where the rulings above do not change it; the revision is
[autoradio-learned-document-2026-11-17.md](../design/autoradio-learned-document-2026-11-17.md).

## TestFlight this week

From the repo on 2026-11-18 (`mobile/README.md`, `mobile/AutoradioLab/app.json`,
`mobile/scripts/make-profile.sh`, `docs/reference/exact/build-and-ship.md`,
`docs/reference/exact/gaps.md`). App Store rules are not restated here.

| Item | State | Evidence or act |
|---|---|---|
| Paid developer programme team | owner act | signed today by a free personal team whose profiles last seven days (`make-profile.sh:2-3,7`; `build-and-ship.md:100,132,182-184`) |
| Bundle id | present, owner decision | `news.choir.autoradio.lab` (`app.json:5,9`); a public build may want another id (D3) |
| App Store Connect app record | owner act | nothing in the repo |
| Distribution certificate and profile | owner act, then wiring | Exact makes no profiles (`build-and-ship.md:102`); the release path takes `EXACT_IDENTITY` and `EXACT_PROFILE` (`:196-198`); upload "not exercised" (`:205`) |
| Microphone and speech usage strings | present | `mobile/AutoradioLab/strings/en.json:2,4`; `app.ts:4,6` |
| Background audio | present | `app.json:20`; locked-screen playback not yet proved on a phone (`README.md:460`) |
| Privacy manifest (`PrivacyInfo.xcprivacy`) | missing | not in the repo; the app stores preferences, Keychain data and voice, and takes publish a voice |
| Encryption declaration (`ITSAppUsesNonExemptEncryption`) | missing | not in the repo |
| Version and build number | missing | none in `app.json`; `mobile/EXACT_PIN` not yet created (`build-and-ship.md:226-227`) |
| A sign-in a new tester can complete | missing | device code approved by passkey for an existing account (`README.md:136-138`); no sign-up path and no review account; a new account's computer appears only after the first status call (`README.md:211`) |
| Account deletion | missing | sign-out only (`README.md:144,159,214`); needed once a tester can create an account in the app |
| App icon | present | `app.json:12-14` and `appicon/` |
