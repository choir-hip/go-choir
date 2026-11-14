---
definition_version: 7

readiness: executable

review:
  reviewer: none
  frozen_ref: none
  verdict: none
  evidence_ref: none; the owner's statement of 2026-11-14 is the authority to start. Station L0 puts the design before two seats, and no red station builds until both accept it

start:
  captured_at: 2026-11-14T19:07Z
  source:
    canonical_ref: origin/main 21e7860bd
    deploy_identity: unknown (choir.news health answered ok at capture; the deployed commit was not read)
  worktrees:
    - path: /Users/wiz/Dev/go-choir
      status: dirty
      class: user_wip
      owner: owner
      touch: forbidden
      recovery: leave in place (one untracked env backup that holds secrets)
  observed:
    - Autoradio pass 1 (docs/definitions/choir-autoradio-pass1-2026-11-10.md) is blocked_incomplete on owner acts. Its install candidate is origin/lane/golden-docs da1e678ad. An iOS app is on main under mobile/.
    - The fast-turn route POST /api/autoradio/turns is on main and deployed. On staging it answered from the Bedrock fallback, because River is not registered on the gateway (docs/reference/RIVER.md; the gateway credential file on Node B had no River entry on 2026-11-14, checked by variable name).
    - Not built. The programme route, the run sheet writer (the kind is reserved with no writer), the role autoradio_programme (named "future" in internal/modelpolicy/model_policy.go), the air log, POST /api/autoradio/heard, any Autoradio settlement rule, any lesson record.
    - The phone keeps gestures (more, less) in a local file and sends none of them. It has a "What the paper has learned" screen fed by those gestures. Its edition writer is a stub that throws.
    - A generic claims route exists (/api/precommit/claims, settlements, GET /api/precommit/calibration) and a device key holds precommit:write. It lets the caller settle its own claim.
    - Autopaper reads the listener's edition nowhere. Importance is one global score.
    - Storage solutions parts 2 and 3 are the live stations. Node B refused a deploy over disk headroom on 2026-11-11.
    - Unknown. Whether a device key's claim and settlement calls work end to end. Whether the commitment record can hold a per-question settlement written by a route handler. Free disk on Node B at capture. Whether DeepSeek V4.1 Flash writes useful lessons.

finish:
  deliver: >
    A listener hears a programme the DJ wrote for them, and the DJ visibly
    learns them: it says what it expects before each segment, the listener's
    own listening marks it, it writes what it learned in a notebook the
    listener can read and correct, and the next programme is different
    because of it.
  artifact: >
    On main and deployed: the programme call with a forecast and a stated
    cause on every content segment; the air log and its settlement rule; the
    settled outcomes and the DJ's record in the next call's input; cited
    lessons returned by the call; the notebook route; and on the phone, the
    player, the playback reports and the notebook screen. Design:
    docs/design/autoradio-learning-engine-2026-11-14.md.
  acceptance:
    - action: open a programme for d@e from a host off Node B and read the stretch
      proves: every story, update, headlines, mail and podcast_note segment carries p_heard and caused_by, and the stretch is one forecast record on the tape with one question per segment
      evidence_class: deployed proof, the stretch and the record id
    - action: finish two segments, skip one, cut one and do not resume; read the air log and the record
      proves: four air log events; the questions settle yes, yes, no and open; the settlement names the heard route, not the desk, as its writer
      evidence_class: deployed proof, the event ids and the settlements
    - action: run the test that drives the programme call with a provider that tries to emit an air log event and a settlement
      proves: the programme call has no path that writes an outcome (rule 1b)
      evidence_class: local test in the repo, run in CI
    - action: open the next stretch for d@e and read the request trace and the response
      proves: the call's input held the settled outcomes per segment, the notebook and the record by band, with no sentence telling the DJ which way to move; the response held at least one lesson citing those air log events
      evidence_class: deployed proof, the trace and the lesson ids
    - action: across one session skip three segments on one subject, then open the next two stretches
      proves: a lesson names the subject and cites the three skips; later segments on it are fewer, and those that remain carry a lower p_heard or another cause
      evidence_class: deployed proof, the two stretches and the notebook
    - action: read GET /api/autoradio/notebook, strike one lesson, open the next stretch
      proves: the notebook returns every lesson with cites that resolve to air log events, each lesson's tally and the DJ's record; the struck lesson is absent from the next call's input and is not written again from the same events
      evidence_class: deployed proof, the two notebook reads and the trace
    - action: the owner listens for thirty minutes on the phone, then reads the notebook
      proves: the phone's notebook shows what the route returns, and the owner says of each lesson whether it is true
      evidence_class: real iPhone, the owner's verdict per lesson, recorded in the ledger file
    - action: compute the Brier score of p_heard and of the listener's base rate over the first 100 settled segments of one real listener
      proves: whether the forecasts separate what is heard from what is skipped; this settles ARL-C1 either way and is not a pass line
      evidence_class: deployed, the calibration summary and the air log
  rollback: >
    Each station reverts by commit. Stretches, air log events and lessons
    already written stay on the tape as history. With the programme route
    reverted the app falls back to the cached front page.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >
    The gap between what the DJ plays and what this listener wanted gets
    smaller, shown by fewer corrections per listening hour and by p_heard
    separating heard from skipped better than the listener's base rate,
    while everything Autoradio holds about the listener is readable by them.
  goodharting_would_be: >
    A finish rate that rises because the programme got shorter, safer or
    more like a feed. Forecasts all at the base rate, calibrated and useless.
    Lessons that restate the edition, or whose cites do not support them. A
    notebook that is a summary while more is kept elsewhere. A DJ shown its
    finish rate and told to raise it.

homotopy:
  realism_axis: >
    A script against d@e on staging with a scripted listener; then the
    simulator playing the real route; then the owner's iPhone for thirty
    minutes; then a week of the owner's ordinary listening.

boundaries:
  mutation_class: red
  authority_sources:
    - owner statements of 2026-11-13 and 2026-11-14 (owner_rulings below)
    - docs/learning-engine.md
    - docs/design/autoradio-learning-engine-2026-11-14.md
    - docs/design/autoradio-dj-programme-2026-11-11.md, as changed by section 9 of the design above
    - docs/choir-doctrine.md
    - CLAUDE.md
  must_preserve:
    - rule 1; the DJ's model call never writes an air log event, a settlement or a lesson tally
    - rule 2; the notebook route returns everything Autoradio holds about the listener
    - the air log and the notebook never leave the computer and are never aggregated across listeners
    - a stated aim outranks a learned lesson; a lesson becomes a stated aim only by the listener's word
    - the DJ is never shown a finish rate or told which way to move its probabilities
    - one model on River, the 1017 snapshot; Chat Completions only (docs/reference/RIVER.md)
    - one custodian per data kind; the Autoradio desk does not write Texture
    - proofs and drills run on d@e, never on the paper or the owner's computers
  excluded:
    - registering River on the gateway (an owner act; the procedure is written, the act is not this mission's)
    - mail headers, podcasts, the station and write-ahead (programme design slices 3, 6 and 7)
    - the proposal to change the edition (design section 8); it waits for hand-offs (slice 5)
    - making Autopaper read the edition, and forecasts for source processing (the next mission)
    - renaming precommit on the wire, in the CLI or in Go identifiers
    - any rule that widens the DJ's latitude from its record alone
  protected_surfaces:
    - gateway and provider calls (the new role autoradio_programme)
    - tape and evidence (first writers of the run sheet, air log and lesson records; settlements)
    - auth and session (device-key routes)

stations:
  - id: L0
    name: design review
    scope: two seats read docs/design/autoradio-learning-engine-2026-11-14.md and this file; findings are adopted, deferred or rejected in the ledger file
    class: green
    review: two seats
    depends_on: []
    carrier: none yet
    status: pending
  - id: L1
    name: programme call with forecasts
    scope: programme design slice 2, plus p_heard and caused_by on every content segment, line validation, and one forecast record per stretch (design sections 3 and 9)
    class: red (provider role, first writer of a kind, device-key route)
    review: two seats
    depends_on: [L0]
    carrier: none yet
    status: pending
  - id: L2
    name: air log and settlement
    scope: programme design slice 4's air log and POST /api/autoradio/heard, plus the settlement rule, gesture and correction events, and the rule 1b test (design section 4)
    class: red (settlements are evidence on the tape)
    review: two seats
    depends_on: [L1]
    carrier: none yet
    status: pending
  - id: L3
    name: the paper back, and lessons
    scope: settled outcomes, notebook and record in the call's input; lessons in its output with cite validation and computed tallies; a test that the assembled prompt carries no directional sentence (design 6.1, 6.2)
    class: orange
    review: one seat
    depends_on: [L1, L2]
    carrier: none yet
    status: pending
  - id: L4
    name: notebook route
    scope: GET /api/autoradio/notebook, strike, correct and clear, with the DJ's record (design 6.3 and 7)
    class: orange
    review: one seat
    depends_on: [L3]
    carrier: none yet
    status: pending
  - id: P1
    name: phone, programme player
    scope: the engine and player of programme design section 6, against fixtures first and the route when L1 is deployed
    class: orange
    review: one seat
    depends_on: []
    carrier: none yet
    status: pending
  - id: P2
    name: phone, playback reports
    scope: finished, skipped, cut, paused, left, more, less and corrected, buffered offline and sent to the heard route
    class: orange
    review: one seat
    depends_on: [P1]
    carrier: none yet
    status: pending
  - id: P3
    name: phone, the notebook screen
    scope: replaces "What the paper has learned"; shows lessons, cites, tallies and the record; strike, correct, clear
    class: orange
    review: one seat
    depends_on: [L4]
    carrier: none yet
    status: pending

decisions_for_the_owner:
  - id: D1
    question: Register River on the gateway now?
    why: Without it the programme answers from Bedrock, which does not stream, so the first segment waits for the whole stretch. It spends money and rewrites the gateway credential file.
    options: run docs/reference/RIVER.md section 5 now; or wait
    default: wait. Build and prove on the Bedrock fallback and make no latency claim.
  - id: D2
    question: Where does the notebook live?
    why: It decides the custodian of the lessons and what the listener opens to read them.
    options: a record held by the Autoradio desk and shown in the app; or a section of the listener's edition page written by Texture
    default: the desk-held record, because the desk may not write Texture and the phone's edition writer is a stub. Move it to the edition page when hand-offs exist.
  - id: D3
    question: A go for a phone install carrying P1 to P3?
    why: Phone proof needs an install. The free provisioning profile lasts seven days.
    options: go for a named candidate; or simulator only
    default: simulator and a script against d@e until the owner says go.
  - id: D4
    question: Does this run beside storage solutions, or after it?
    why: Every server station needs a deploy, and Node B refused one over disk on 2026-11-11.
    options: beside it, each deploy taken when headroom allows; or after part 3
    default: beside it. Phone stations and local work never wait; a server landing waits only on a refused deploy.
owner_rulings:
  - on: 2026-11-13
    decision: order of application
    said: "we also, just as more importantly, want to immediately apply this so that we can improve or really build to a level of strength the auto radio personalization learning loop. And then from there, we can then backport the learning engine into auto paper"
  - on: 2026-11-13
    decision: rule 2
    said: "everything that we learn about the listener is going to be visible to the listener. No hidden dossiers."
  - on: 2026-11-14
    decision: the frame
    said: "Yes, the outcome is the teacher."
  - on: 2026-11-14
    decision: River's model
    said: "They have other models but I only want this one: \"deepseek-ai/DeepSeek-V4.1-Flash-1017\""
  - on: 2026-11-14
    decision: D1, in part
    said: "you shouldn't actually configure this yet, but just write a doc for how to configure it"
    read_as: River stays unregistered until the owner says otherwise; the default of D1 follows this
  - on: 2026-11-14
    decision: start
    said: "I want you to actually make this into a PR, and we'll merge that PR, and then we'll get to work on it from our main worker thread."

now:
  status: working
  slice: L0, with P1 free to start on fixtures
  source_ref: origin/main 21e7860bd
  deploy_identity: unknown
  candidate:
    id: none
    state: none
    ref: none
    base: none
    digest: none
    scope: []
  conjecture:
    id: ARL-C1
    calibration_claim: C87
    claim: >
      Showing the DJ each settled segment with its stated cause, and having
      it write cited lessons, makes p_heard separate heard from skipped
      better than the listener's base rate within the first 100 settled
      segments of one real listener.
    test: >
      Brier score of p_heard against the Brier score of the base rate over
      the same 100 segments, and the second 50 against the first 50, read
      from the calibration summary and the air log.
    edge: missing_oracle
    delta_o: >
      The per-lesson tally (segments a lesson caused, for and against),
      which shows whether a gain came from the lessons or from the listener
      settling into a habit.
    scope_if_supported: one listener, the content kinds played, the first hours; nothing about other listeners or about Autopaper
    status: proposed
    evidence_refs: []
  decision:
    what: forecasts ride in the stretch; the heard route settles them by rule; lessons come back from the stretch call and live in a notebook the listener reads
    kind: architecture
    status: proposal
    evidence_ref: docs/design/autoradio-learning-engine-2026-11-14.md
    owner_ratification_ref: the owner's discussion of 2026-11-13 and 2026-11-14 set the frame and the rules; the design itself is unreviewed until L0
  belief:
    believed_state: >
      Nothing of this mission is built. The route it extends (fast turns)
      works on staging from the Bedrock fallback. The phone has a player for
      one story and a local learned screen.
    main_uncertainty: >
      Whether a small fast model writes lessons worth reading from twenty
      outcomes, and whether the commitment record takes a per-question
      settlement from a route handler without new rule kinds.
    next_observation: >
      L0's two verdicts; then the first hour of L2, which settles the
      settlement path (design section 12).
  blocker_or_risk: >
    None for L0 and P1. Risks: River unregistered (D1), no install go (D3),
    deploy headroom on Node B (D4). The programme design's slice 2 names old
    review seats; CLAUDE.md's seats apply.
  next_action: >
    Run L0: put the design and this file before the two seats named in
    CLAUDE.md, adjudicate every finding in the ledger file, then start L1
    with its failure list.
---

# Autoradio learns its listener: the four-stroke engine, first build

Run with `/goal docs/definitions/choir-autoradio-learning-engine-2026-11-14.md`.
History goes in
`docs/definitions/choir-autoradio-learning-engine-2026-11-14.ledger.md`,
created with the first state change.

## What this is

The first whole build of the
[four-stroke learning engine](../learning-engine.md), on the site with the
best teacher we have: a listener who finishes, skips or cuts in within
seconds. The design is
[autoradio-learning-engine-2026-11-14.md](../design/autoradio-learning-engine-2026-11-14.md).

It does not replace Autoradio pass 1
([choir-autoradio-pass1-2026-11-10.md](choir-autoradio-pass1-2026-11-10.md)),
which stays the root for the app, its voice modes and its install. It takes
over one of pass 1's acceptance items, the one about run sheet choices being
forecasts settled by what the listener did, and widens it to all four
strokes.

## Order

L0 first. Then L1, L2, L3, L4 in that order on the server, each landing and
proved on d@e before the next starts. P1 starts at once on fixtures; P2 and
P3 follow their server stations. L1 and L2 give a DJ that forecasts and is
marked. L3 is where it starts to learn.

## What a builder must not do

- Give the programme call any way to write an outcome.
- Show the DJ a finish rate, or write "be less confident" into its input.
- Keep anything about the listener that the notebook route does not return.
- Let a learned lesson override the edition.
- Build a second scoring path beside the commitment record and the claims
  store. Extend one of them (design section 12).

## After this

The backport to Autopaper: the paper reads the listener's edition, source
processing states forecasts with outcomes, and standing hypotheses carry
lessons. Then self-development, where the engine already half exists
([learning-engine.md](../learning-engine.md), section 9).
