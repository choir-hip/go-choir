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
    canonical_ref: origin/main 955d4527f (revised after the L0 review; the first capture was 21e7860bd)
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
    - A generic claims route exists (/api/precommit/claims, settlements, GET /api/precommit/calibration) and a device key holds precommit:write. Any such key can create a claim and settle any claim with a rule or owner method, and the earliest rule settlement governs (internal/calibration/handler.go:187,261; math.go:49). The claims store takes one probability and one outcome per claim (records.go:196,334).
    - Autopaper reads the listener's edition nowhere. Importance is one global score.
    - Storage solutions parts 2 and 3 are the live stations. Node B refused a deploy over disk headroom on 2026-11-11.
    - Unknown. Whether a device key's claim and settlement calls work end to end (the design does not depend on them: station L1a closes the generic route to Autoradio). Free disk on Node B at capture. Whether DeepSeek V4.1 Flash writes useful lessons.

finish:
  deliver: >
    A listener hears a programme the DJ wrote for them, and the DJ visibly
    learns them: it says what it expects before each segment, the listener's
    own listening marks it, it writes what it learned in a notebook the
    listener can read and correct, and the next programme is different
    because of it.
  artifact: >
    On main and deployed: the programme call with a forecast claim and a
    stated cause on every content segment; the air log and its settlement
    rule; the marked cases in the next call's input; cited
    lessons returned by the call; the notebook route; and on the phone, the
    player, the playback reports and the notebook screen. Design:
    docs/design/autoradio-learning-engine-2026-11-14.md.
  acceptance:
    - action: L1a. A device key creates a claim as agent autoradio-dj, then settles a forecast the server wrote with method rule and name autoradio_heard, then with method owner; the generic list and summary are read with and without the agent filter
      proves: create, settle and get on a reserved claim answer 403; the generic list and summary hold no reserved claim and are not truncated by 10001 of them; a forged rule settlement cannot govern
      evidence_class: local test in CI, then the same calls on d@e; provenance is read from the route and the actor on the record, never from a scorer-name string
    - action: L1. Run the programme route with a fake provider that emits valid lines and one line each for p_heard 1.7, no p_heard, an unknown cause, an unresolved instruction, an edition paragraph out of range, a same-response lesson cite, an unknown cite, three invalid lines in a row, and a claim write that fails
      proves: only valid lines are streamed; each streamed segment has exactly one claim on the tape, committed before its segment event (order read from the tape and the event stream); a dropped line has no claim; a failed claim write ends the call with no segment; the stretch index lists its claims; a replay writes nothing new
      evidence_class: local test in CI; then deployed proof on d@e with the real route, the stretch, the claim ids and the index
    - action: L2. Drive the heard route from a script with the fixture batches, covering finished, skipped in the final block, skipped earlier, cut and never resumed, cut then resume then finished, paused, left, a duplicate, a conflicting event_id, an unknown segment, a hash mismatch, a late report after lapse, and every ordering of one batch
      proves: settlements yes, yes, no, open, yes, open, open as design 4.2 gives; the same event set settles the same under every ordering and under replay; unknown segment and hash mismatch answer 422 and the conflict 409; each settlement's evidence resolves to an air event written by the heard route; open and lapsed are counted in coverage and excluded from the Brier
      evidence_class: local test in CI; then deployed proof on d@e, the event ids and settlement ids read from the tape
    - action: L2. Run the test that drives the programme call with a provider that tries to emit an air event and a settlement, send a programme request carrying a heard field, and grep the call sites of the reserved settle method
      proves: the programme call has no path that writes an outcome (rule 1b); the heard field answers 400; the settle method has one caller, the heard handler
      evidence_class: local tests in CI
    - action: L3. Assemble the programme input for a fixture history and compare it with the golden prompt; drive the lesson validators with a fake provider (cite not in the input, fewer than two air events, replaces a struck or unknown lesson, every cite already on a struck lesson, a model-sent lesson id, a lesson line past 300 characters); put an instruction holding a fence look-alike and a candidate title that says to ignore the edition through the frame
      proves: the input holds marked cases only (no band table, count, rate, total or streak) in the exact shape of the golden file; each invalid lesson line is dropped; ids are minted by the handler; the hostile strings stay inside their fences as data
      evidence_class: local tests in CI. A change to the golden file needs a seat to read it
    - action: L4. Read GET /api/autoradio/notebook on d@e after a scripted session with turns, a gesture, a correction, a strike and a clear; compare each section with a listing of the tape
      proves: the notebook returns every lesson, forecast with outcome, air event, turn record with its words and instruction on the tape, cleared ones marked; the struck lesson is absent from the next call's input and is not accepted again from the same cites; clear removes events from the next input and deletes nothing; the operational traces are named under held_elsewhere
      evidence_class: deployed proof on d@e; the completeness comparison is a rerunnable script
    - action: P1, P2 and P3 against the fixtures, the simulator and a script against d@e (D3, no phone install until the owner says go)
      proves: the phone plays the programme, sends the batch, deletes acknowledged lines from its local signals file, and shows what the notebook route returns
      evidence_class: simulator run and script output; a real iPhone only after the owner's go (D3), recorded in the ledger file
    - action: observation, not acceptance. Run a scripted listener on d@e until 100 forecasts are settled, read the lessons the real model wrote, and compute the reading of design section 5
      proves: nothing is passed or failed. It records whether the forecasts separate heard from skipped better than the causal base rate, per segment kind, with coverage and the per-lesson tallies, and it settles ARL-C1 only. The owner's verdict on each lesson (true, false, unclear) goes in the ledger file, and a lesson the owner marks false is struck
      evidence_class: deployed, the calibration summary, the air log and the notebook
    - action: observation, not acceptance. Skip three segments on one subject, open the next two stretches, and replay the same input with the notebook empty
      proves: nothing is passed or failed. It records how many segments on the subject each stretch held and their causes and p_heard
      evidence_class: deployed, the stretches, the replays and the notebook
  rollback: >
    Each station reverts by commit. Stretches, air log events and lessons
    already written stay on the tape as history. With the programme route
    reverted the app falls back to the cached front page. L1a's reservation
    reverts only after the stations that write reserved claims.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >
    The gap between what the DJ plays and what this listener wanted gets
    smaller, shown by fewer corrections per listening hour and by p_heard
    separating heard from skipped better than the causal base rate, per
    segment kind, with coverage reported beside it, while everything Autoradio holds about the listener is readable by them.
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
    - rule 1; the DJ's model call never writes an air event, a settlement or a lesson tally; the heard route is the only writer of air events and the only caller of the reserved settle method; no generic claims call can create, settle or read an Autoradio claim
    - rule 2; the notebook route returns everything Autoradio holds about the listener (air log, forecasts with outcomes, turn records with their words, lessons with cites and tallies, instructions), and names the operational traces separately
    - the air log and the notebook never leave the computer and are never aggregated across listeners
    - a stated aim outranks a learned lesson; a lesson becomes a stated aim only by the listener's word
    - the DJ is never shown a finish rate, a count, a total, a streak or a band table, and is never told which way to move its probabilities; its record in its input is marked cases
    - one model on River, the 1017 snapshot; Chat Completions only (docs/reference/RIVER.md)
    - one custodian per data kind; the Autoradio desk does not write Texture
    - proofs and drills run on d@e, never on the paper or the owner's computers
  excluded:
    - registering River on the gateway (an owner act; the procedure is written, the act is not this mission's)
    - mail headers, podcasts, the station and write-ahead (programme design slices 3, 6 and 7)
    - the proposal to change the edition (design section 8); it waits for hand-offs (slice 5)
    - making Autopaper read the edition, and forecasts for source processing (the next mission)
    - programme design slice 4's System 1 resume question and redirect
    - latency claims on the Bedrock fallback, including the programme design's 6 s median first segment; they wait for River (D1)
    - erasure of the air log or the notebook (a black change and an owner act); clear is a visibility act
    - renaming precommit on the wire, in the CLI or in Go identifiers
    - any rule that widens the DJ's latitude from its record alone
  protected_surfaces:
    - gateway and provider calls (the new role autoradio_programme)
    - tape and evidence (first writers of the forecast claims, stretch index, air log and lesson records; settlements)
    - auth and session (device-key routes; the reservation in the generic claims routes)

stations:
  - id: L0
    name: design review
    scope: two seats read docs/design/autoradio-learning-engine-2026-11-14.md and this file; findings are adopted, deferred or rejected in the ledger file. Round 1 returned REPAIR from both seats (ledger); this revision answers it and goes back to the same two seats
    class: green
    review: two seats
    depends_on: []
    carrier: none yet
    status: working
  - id: L1a
    name: reserved namespace, forecast constructor, wire fixtures
    scope: the generic claims routes refuse create, settle and get on the Autoradio namespace (403) and exclude it from list and summary before the record limit; calibration.NewAutoradioForecast; the settle method with its closed rule registry (autoradio_heard); extend calibration.Summary with the causal base-rate Brier; the fixtures of design section 12. Design 3.3, 4.3, 5, 12
    class: red (auth and evidence on the generic route). Conjecture delta, protected surfaces, evidence class, rollback and heresy delta are written in the commit body. Ring-Expedite names the defect it unblocks, "a device key can settle a forecast about its own listening".
    review: two seats
    depends_on: [L0]
    carrier: none yet
    status: pending
  - id: L1
    name: programme call with forecasts
    scope: programme design slice 2, plus p_heard and caused_by, the validators of design 3.2, one claim per content segment appended before its segment is streamed, the stretch index, the candidates request field, and the role autoradio_programme registered in NormalizeRole with the "future" comment changed (modelpolicy). Design 3, 9
    class: red (provider role, first writer of forecast claims and the stretch index, device-key route). Red ceremony in the commit body; Ring-Expedite "the radio plays one article, not a programme".
    review: two seats
    depends_on: [L1a]
    carrier: none yet
    status: pending
  - id: L2
    name: air log, heard route, settlement
    scope: programme design slice 4's air log and dedupe (covered), POST /api/autoradio/heard with the rule, the result codes, lapse and ack of design 4.2, gesture and correction events, and the rule 1b tests of 4.1. Not the resume question or redirect
    class: red (settlements are evidence on the tape; first writer of air events)
    review: two seats
    depends_on: [L1]
    carrier: none yet
    status: pending
  - id: L3
    name: the paper back, and lessons
    scope: marked cases, instructions, edition, candidates and lessons in the call's input with the content-derived fence; lessons in its output with the validators of 3.2; lesson records minted by the handler; the golden prompt test (design 6.1, 6.2)
    class: red (first writer of lesson records; changes the input of a provider call)
    review: two seats
    depends_on: [L1, L2]
    carrier: none yet
    status: pending
  - id: L4
    name: notebook route
    scope: GET /api/autoradio/notebook with every section of design 6.3, strike, correct and clear as append-only visibility records, the calibration view from the extended summary, and the completeness script. Clear deletes nothing, so it is orange; any erasure would be black and is excluded
    class: orange
    review: one seat
    depends_on: [L3]
    carrier: none yet
    status: pending
  - id: P1
    name: phone, programme player
    scope: the engine and player of programme design section 6, against the fixtures of L1a first and the route when L1 is deployed; simulator only (D3)
    class: orange
    review: one seat
    depends_on: [L1a]
    carrier: none yet
    status: pending
  - id: P2
    name: phone, playback reports
    scope: finished, skipped, cut, resume, paused, left (synthesised at the next launch), more, less and corrected, buffered offline, sent to the heard route, and deleted from the local signals file once acknowledged
    class: orange
    review: one seat
    depends_on: [P1, L2]
    carrier: none yet
    status: pending
  - id: P3
    name: phone, the notebook screen
    scope: replaces "What the paper has learned"; shows lessons, cites, tallies and the record; strike, correct, clear
    class: orange
    review: one seat
    depends_on: [L4, P2]
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
    default: simulator and a script against d@e until the owner says go. Every phone acceptance item is stated for that default.
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
  - on: 2026-11-15
    decision: review seats, D4, and the first phone build
    said: "Sounds good. DeepSeek and sonnet for review. And build after pr 110. I want to build once tonight (I'll be up for 2-4 more hours) then let you work overnight."
    read_as: while the Gemini seat is out of quota the two seats for this mission are DeepSeek V4.1 Flash and Sonnet (a Sonnet seat never reviews a change a Sonnet builder of this session wrote alone; DeepSeek always sits); the mission runs overnight beside storage solutions (D4, beside it); the owner builds the phone app from main after PR 110 (0157fd919) tonight and again tomorrow; no install is made by the lead

now:
  status: working
  slice: L0 second round; P1 starts when L1a delivers the fixtures
  source_ref: origin/main 955d4527f
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
    calibration_claim: C90
    claim: >
      Showing the DJ each marked case with its stated cause, and having it
      write cited lessons, makes p_heard separate heard from skipped better
      than the causal base rate within the first 100 settled segments of one
      real listener.
    test: >
      Design section 5: the strict Brier of p_heard against the strict Brier
      of the causal base rate (the running rate over segments settled before
      each forecast), per segment kind, with coverage (settled over forecast)
      beside it, over the first 100 settled and the second 50 against the
      first 50, read from the extended calibration summary. It settles
      ARL-C1 only.
    edge: missing_oracle
    delta_o: >
      The per-lesson tally (segments whose forecast cited the lesson, heard
      and not heard), computable from the claim tags and read with the same
      window. It measures what the DJ said caused a segment, not what did,
      so it shows whether a gain came from the lessons or from the listener
      settling into a habit only as far as the DJ's own account goes.
    scope_if_supported: one listener, the content kinds played, the first hours; nothing about other listeners or about Autopaper
    status: proposed
    evidence_refs: []
  decision:
    what: each content segment is one claim in the claims store, appended before the segment is streamed; the heard route, the only writer of air events, settles it by a closed rule; lessons come back from the stretch call as desk-held records and live in a notebook the listener reads in full
    kind: architecture
    status: proposal
    evidence_ref: docs/design/autoradio-learning-engine-2026-11-14.md
    owner_ratification_ref: the owner's discussion of 2026-11-13 and 2026-11-14 set the frame and the rules; the design is in its second round of L0
  belief:
    believed_state: >
      Nothing of this mission is built. The route it extends (fast turns)
      works on staging from the Bedrock fallback. The phone has a player for
      one story and a local learned screen.
    main_uncertainty: >
      Whether a small fast model writes lessons worth reading from twenty
      outcomes. The claim path is decided (design 3.3, 12).
    next_observation: >
      L0's two verdicts on this revision; then L1a's failure list.
  blocker_or_risk: >
    None for L0 and P1. Risks: River unregistered (D1), no install go (D3),
    deploy headroom on Node B (D4). Review seats are the two the owner set
    (owner_rulings, 2026-11-15).
  next_action: >
    Run L0 again: put the revised design and this file before the same two
    seats, adjudicate every finding in the ledger file, then start L1a with
    its failure list.
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

L0 first. Then L1a, L1, L2, L3, L4 in that order on the server, each landing
and proved on d@e before the next starts. Every non-docs landing commit
carries `Ring-Expedite: <the defect it unblocks>` and is named in the next
report; red stations carry the red ceremony of CLAUDE.md (conjecture delta,
protected surfaces, evidence class, rollback, heresy delta) and two seats. P1
starts when L1a has delivered the wire fixtures; P2 waits for L2 and P3 for
L4 and P2. L1 and L2 give a DJ that forecasts and is marked. L3 is where it
starts to learn.

## What a builder must not do

- Give the programme call any way to write an outcome.
- Show the DJ a finish rate, a count, a band table or a streak, or write "be less confident" into its input.
- Give any generic route, key or request field a way to create, settle or read an Autoradio claim, or to write an air event.
- Keep anything about the listener that the notebook route does not return.
- Let a learned lesson override the edition.
- Build a second scoring path beside the claims store. Extend
  `calibration.Summary` (design 4.3, 5).

## After this

The backport to Autopaper: the paper reads the listener's edition, source
processing states forecasts with outcomes, and standing hypotheses carry
lessons. Then self-development, where the engine already half exists
([learning-engine.md](../learning-engine.md), section 9).
