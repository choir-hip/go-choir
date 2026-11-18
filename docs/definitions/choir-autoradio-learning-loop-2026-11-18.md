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
    - "mail and local-only files never leave the phone"
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
    scope: "one Texture document per listener within a one-page budget; the DJ proposes lesson lines in the stretch call from the settled rows it is shown; Texture writes the revision; a confirmation raises a count and adds no text; a new line displaces or merges older ones when the page is full; the listener's edits and strikes hold"
    class: red
    review: two seats
    depends_on: [L2]
    carrier: "lane/learn-design (design docs/design/autoradio-learned-document-2026-11-17.md), then a builder lane"
    status: working
  - id: L5
    name: The DJ reads the document
    scope: "the document goes into the programme input; caused_by points at its lines so a settled forecast credits or discredits the line that caused the pick; candidate order uses it"
    class: red
    review: two seats
    depends_on: [L4]
    carrier: not started
    status: pending
  - id: L6
    name: The phone
    scope: "a page for the document, the scorecard and the rows behind a line; spoken and typed steering and the more and less taps become the listener's own lines; the topic list moves into the document; the phone sends edition text"
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
    carrier: "specification first in lane/learn-design, then a server lane and a phone lane in parallel"
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
    scope: "the owner's paid team, an App Store Connect record, a distribution profile, usage strings, the privacy manifest, the encryption declaration, a sign-in a tester can complete; an archive that uploads"
    class: orange
    review: one seat
    depends_on: [L6, L7, L8]
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
    default: the notebook, replacing "My edition"
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
    default: a small, stated number of first offers, labelled as a trial by the DJ's cause

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

now:
  status: working
  slice: "L1 to L3 are with one builder on lane/learn-settle, whose first act is to confirm from the drill computer's records that outcomes are dropped today; the design for L4 to L7 is being written on lane/learn-design with the takes specification first; the phone candidate lane/autoradio-integ9 is in its simulator build"
  source_ref: "origin/main at the commit that carries this file"
  deploy_identity: "staging 90f258bb4"
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
    kind: architectural
    status: settled
    evidence_ref: this file, owner_rulings
    owner_ratification_ref: "the owner's messages of 2026-11-17"
  belief:
    believed_state: "one stroke of four runs; the pieces for the second and third exist unconnected; no document and no take exist"
    main_uncertainty: "whether outcomes are in fact dropped on the server today, and whether a one-page document can carry enough to change the DJ's picks"
    next_observation: "the learn-settle builder's read of the drill computer's sheet records"
  blocker_or_risk: "four red stations need two review seats each, which sets the pace; TestFlight needs the owner's paid team (D3)"
  next_action: "review and land L1 to L3 when lane/learn-settle is pushed; start the L7 server and phone builders when the takes specification is pushed; install lane/autoradio-integ9 when its tours pass"

receipts: []
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
`docs/design/autoradio-learned-document-2026-11-17.md`, on its way from
lane/learn-design.
