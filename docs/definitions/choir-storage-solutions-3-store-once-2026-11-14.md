---
definition_version: 7

readiness: executable

review:
  reviewer: omp-gemini4-hydrogen and omp-deepseek-v41-flash, on the design this file follows
  frozen_ref: 8203f53fd
  verdict: send_back
  evidence_ref: both seats REPAIR at 2026-11-14T01:25Z; the design is at revision 2 and this file's stations follow it. T0 and T7 run on the owner's word; no red station builds until a seat accepts the re-ranked design (station T0)

start:
  captured_at: 2026-11-14T01:15Z
  source:
    canonical_ref: origin/main eecafd3f1
    deploy_identity: choir.news deployed_commit eecafd3f132a
  worktrees:
    - path: /Users/wiz/Dev/go-choir
      status: dirty
      class: user_wip
      owner: owner
      touch: forbidden
      recovery: leave in place (one untracked env backup that holds secrets)
  observed:
    - The paper holds about 168 GiB on Node B and grows about 7 GB a day (tape 72.9 GiB at 3.6 GB a day; guest store 43.8 GiB at about 2 GiB a day; snapshot chunks 37.5 GiB; receipts about 14 GB). Its published content is a few hundred megabytes.
    - Measured over 2.9 hours on the paper (2026-11-13T21:05Z to 23:58Z), 76,832 tape events, 438 MB. choir.run is 17% of tape bytes at 16 KB an op, about 19 writes per run; choir.texture_document is 18% at 170 KB an event. 6.03 corpusd calls per event. 88% of events carry one op.
    - Retirement exists and is almost unused (7 retire ops against 110,000 writes in the window).
    - d@e, an ordinary user computer, has 20.5 GiB of its 31.2 GiB data volume in the updater directory and 0.8 GiB free. Nothing there has ever been deleted.
    - Three mechanisms are built and off behind settings (store.event_table, store.telemetry_partitions, store.tape_group_commit), against the no-switches rule.
    - In flight under storage solutions part 2, none deployed yet at capture: coalescing (lane/s5-coalesce-land, both seats accepted, landing), the updater prune (lane/updater-prune, round 2 of review), the per-table store reading (lane/store-size, one seat accepted).
    - Unknown: what fills the paper's store by table; why a Texture document op is 170 KB; the compression factor of small payloads with a shared dictionary; how many readers open history older than a day.

finish:
  deliver: >
    A computer costs storage in proportion to what it changed, not to how
    long it has lived. An ordinary user's computer fits its data volume
    without anyone pruning it, and a computer as busy as the paper costs
    the host a few gigabytes a day at most.
  artifact: >
    On main and deployed: tape ops that carry only what changed, with large
    content written once by digest; a guest store that retires what a
    running computer no longer needs and reads older history from the tape;
    receipts held by segment; sealed payloads compressed with shared
    context; an updater directory bounded to the current release and its
    predecessor; and a per-computer byte budget in docs/ACTIVE.md with the
    measured value beside each line.
  acceptance:
    - action: read the paper's per-op tape counters over a day of ordinary work after T1 and T2 are deployed
      proves: carried and duplicated bytes are under 5% of payload bytes, no kind writes more than twice its changed bytes, and tape growth is under 1 GB a day (3.6 at the start)
      evidence_class: deployed, the guest's observability counters and Node B du
    - action: read the paper's store by table after T3 is deployed, seven days apart
      proves: the store is under 10 GiB, the second reading is within 10% of the first, and every table over 1 GiB has a named retirement rule or a named reason to stay (43.8 GiB and rising at the start)
      evidence_class: deployed, store_tables detail
    - action: count the paper's receipt rows and bytes after T4
      proves: under 2 GB (about 14 GB at the start; the restore window alone is about 1.7 GB at today's row width, so this needs the slim row)
      evidence_class: deployed, host metadata (pg_total_relation_size)
    - action: sum the paper's daily growth on Node B across tape, store image, chunks and receipts over seven days
      proves: under 1.5 GB a day (about 7 at the start); and everything but the tape (store image, chunks, receipts) is under 25 GiB (about 95 at the start). The 72.9 GiB of tape already written stays, neither rewritten nor dropped
      evidence_class: deployed, the hourly growth ledger on Node B
    - action: time the paper's snapshot after T3
      proves: under 10 minutes (37 at the start)
      evidence_class: deployed, snapshot_recorded events
    - action: on d@e, after every change to what an op carries or to retention, destroy the guest disk and restore from the latest snapshot plus the tape tail; then replay the whole tape into an empty store
      proves: the restore address works, the local head equals the platform head, and replay of old and new ops yields the same objects and content hashes
      evidence_class: deployed drill on d@e
    - action: on d@e, start a run that carries a conversation by reference and compare the messages the model is sent with the copying path
      proves: byte-equal messages
      evidence_class: test in the repo and a deployed run on d@e
    - action: read d@e's data volume after twelve updates with no hand cleanup
      proves: the updater directory holds at most three releases and is under 2 GiB (20.5 at the start), and the volume's use does not grow per update
      evidence_class: deployed, data_volume detail
  rollback: >
    Each station reverts by commit. A tape format change keeps the old
    reader forever, so a revert leaves both formats readable; no existing
    tape is rewritten. Retirement is a tape event and reverting its code
    stops new retirement; rows already retired are read from the tape.
    Updater pruning deletes releases that the host can supply again.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >
    Minimise bytes written per byte of state that actually changed, summed
    over the tape, the guest store, snapshots, receipts and the data
    volume, while replay stays complete and the tape stays whole on Node B.
  goodharting_would_be: >
    A smaller footprint from deleting history instead of writing less;
    retirement with no read path to what was retired; a format that old
    tape cannot be replayed through; sampling or dropping telemetry;
    snapshots made rarer; a retention setting in place of a rule that
    belongs to the kind; one-time cleanups reported as a bound.

homotopy:
  realism_axis: >
    Per-kind counters and replay tests, then one kind (choir.run) on d@e
    with a full replay, then every kind on d@e, then the paper under a
    day's work, then seven unattended days.

boundaries:
  mutation_class: red
  authority_sources:
    - owner, 2026-11-13, "we are storing stuff redundantly, and it's not strategic ... what are some strategic refactors that would make the system much more efficient. And I want you, Fable, to figure out this refactoring, not a Sonnet sub-agent."
    - owner, 2026-11-13, "the general user vm will look like d@e, so we cant have any of this runaway growth. we cant afford to be conservative here"
    - owner, 2026-11-14, "all these refactors and maybe more if you discover them ... should be key top priorities, because we really need to get our infrastructure strong before we can ship auto radio anyway."
    - owner rulings D1 and D8 of storage solutions part 2 (no tape segment leaves Node B or is dropped; coalescing is lossless and has no setting)
    - docs/design/store-once-2026-11-14.md, docs/computer-ontology.md, docs/choir-doctrine.md, CLAUDE.md
  must_preserve:
    - the tape is canonical and whole; replay of every existing event keeps working
    - nothing is retired from the guest store before a product read path to it exists
    - no telemetry is sampled or lost
    - no switches; a kind's retirement moment is a property of the kind
    - the paper keeps publishing; format changes are proved on d@e first
    - b@c, the real user's computer and the owner's computer are untouched
  excluded:
    - moving or truncating tape (ruling D1 stands)
    - a new storage engine
    - Autoradio and the iOS app (behind this mission by the owner's word)
  protected_surfaces:
    - tape op format, append path, segments, receipts
    - store schema and projection
    - snapshot and restore
    - the updater's apply, rollback and restore inputs

stations:
  - id: T0
    name: count honestly, then re-rank
    scope: per-op bytes by kind for every op in a batch (the current counter gives a batch to its first op), writes per object, run-memory bytes split carried and new; land the per-table store reading and read d@e and the paper; read the paper's retirement log and watermarks; then re-rank the design and get one seat's accept
    class: yellow (counters), orange (the reading), green (the re-rank)
    review: two seats on the design (done, both REPAIR, revision 2 written); one seat on the re-ranked revision
    depends_on: []
    carrier: lane/store-size-land ceb5288c7 (one seat accepted, rounds 1 and 2); lane/rewrite-counters (building)
    status: working
  - id: T1
    name: stop copying forward
    scope: a new run carries its conversation by naming the source run and entry range, with the retirement rule keeping entries a live run names; a lifecycle command stops embedding the document, revision and events its own batch writes; a tool result names its run-memory entry
    class: red
    review: two seats
    depends_on: [T0, coalescing deployed]
    carrier: none yet
    status: pending
  - id: T2
    name: stop writing mutable objects whole
    scope: a patch op for run updates applied inside the store's transaction, live and on replay, with a witness that the content hash matches a whole write (or fewer intermediate run writes, decided from the T0 numbers); head objects that embed a growing list (choir.story_library_head) split into an append-only kind; a gate that fails a new kind that embeds a growing list
    class: red
    review: two seats
    depends_on: [T0, T1]
    carrier: none yet
    status: pending
  - id: T3
    name: the guest store, from the reading
    scope: no design until T0's reading; then retirement rules for whatever the reading shows uncovered; an index from run and trajectory to tape positions before any widening of retirement; store.event_table and store.telemetry_partitions shipped on or deleted
    class: red
    review: two seats
    depends_on: [T0]
    carrier: design notes "bounding the guest store" on lane/store-size-land
    status: pending
  - id: T4
    name: receipts, with the retained set named
    scope: keep per-event rows for the latest two snapshots to the head, checkpoint head receipts, every non-batch event and live idempotency keys; replace the wide row and four indexes of old batch events with a slim sequence-to-digest row. Takes over S6 of part 2
    class: red
    review: two seats
    depends_on: [coalescing deployed]
    carrier: none yet
    status: pending
  - id: T5
    name: compression, measured first
    scope: payloads are already compressed singly before sealing; measure the share of payload bytes in small plaintexts after coalescing and build a shared dictionary only if it is large
    class: yellow (the measurement)
    review: none for the measurement
    depends_on: [coalescing deployed]
    carrier: none yet
    status: pending
  - id: T7
    name: a user's data volume is bounded
    scope: the updater prune to current and its predecessors (at most three releases); the host re-supplies a pruned release on demand so a restore to it works; a budget line per directory of the data volume, each with its mechanism
    class: red
    review: two seats
    depends_on: []
    carrier: lane/updater-prune (review round 3 pending; both seats REPAIR on rounds 1 and 2)
    status: working
  - id: T8
    name: find the next one
    scope: a standing read. After each landing, read the per-table store reading, the per-op tape counters and the data volume on d@e and the paper; anything that grows without a named mechanism gets a problem record and a station here
    class: green
    review: none
    depends_on: [T0]
    carrier: none yet
    status: pending

decisions_for_the_owner:
  - id: E1
    question: May old run traces open more slowly once they are read from the tape?
    default: yes; a running computer's recent work stays local
  - id: E2
    question: Is existing tape ever rewritten into the new format?
    default: no; old events are read through an upcast forever
  - id: E3
    question: A restore to a release that is neither current nor its predecessor
    default: it refuses cleanly until the host can re-supply the release (T7); no cushion of old releases is kept

observation_plan: >
  When T1 to T4 and T7 have landed and passed their deployed proofs, the mission
  moves to observing for seven days. Readings: the paper's tape bytes a day,
  store size by table, receipt bytes; d@e's data volume. Reader: the
  hourly status tick. Tripwire: a failed replay or restore drill, a kind
  whose writes per object exceed two, or a day over the budget returns the
  mission to working.

now:
  status: working
  slice: "T0 and T7. Land coalescing and the updater prune (carried from part 2), land the store reading and read both computers, send the design to two seats, then build the per-kind rewrite counters"
  source_ref: origin/main eecafd3f1
  deploy_identity: choir.news eecafd3f132a
  candidate:
    id: none
    state: none
  conjecture:
    id: SS3-C1
    calibration_claim: C74
    claim: >
      Most repeated bytes on the tape are content that already has a home:
      conversation tails copied from run to run, results embedded beside
      the objects they describe, and a few mutable objects written whole.
      Naming instead of copying, with coalescing, brings the paper's tape
      growth to well under half of 3.6 GB a day with the tape whole and
      replay complete. What fills the guest store is not yet known.
    test: the first acceptance item; C74 for the carried share
    edge: missing_oracle
    delta_o: per-op bytes by kind and the carried and new split (T0); the per-table store reading (T0)
    scope_if_supported: storage stops being a station of its own; a new kind is checked by a gate
    status: active
    evidence_refs: [docs/design/store-once-2026-11-14.md, docs/evidence/tape-write-census-2026-11-12.md]
  decision:
    what: name content instead of copying it; nothing is retired before its read path exists; the tape is the one archive and stays whole; no content store outside the tape
    kind: architecture
    status: proposed
    evidence_ref: docs/design/store-once-2026-11-14.md
    owner_ratification_ref: none yet (the design is the lead's; T0 reviews it)
  belief:
    believed_state: >
      The first ranking was wrong: it came from a counter that gives a
      batch's bytes to its first op, and three of the five kinds it named
      are written once (claim C72 false). Retirement already runs daily on
      the guest store. The duplicates are read from code, not measured.
    main_uncertainty: the measured share of each duplicate; what fills the paper's store after a retirement that runs; whether retirement is running on the paper at all
    next_observation: store_tables detail on d@e and the paper (claim C73); the per-op counters; the paper's retirement log
  blocker_or_risk: >
    No red station may build until the re-ranked design has a seat's
    accept. Carry by reference changes what an agent's conversation is
    built from; a mistake there changes model input silently.
  next_action: land coalescing and the store reading, read both computers by table, land the per-op counters, re-rank.

receipts: []

targets_note: >
  The size numbers in acceptance are targets set by the owner's instruction
  to be safely aggressive (2026-11-14), not forecasts. They say what a
  computer should cost; the mechanisms to reach them are ranked after T0's
  counters. Each has a calibration claim with a probability (C75 to C78).
  A target that the measured numbers show to be unreachable is reported with
  the number that is reachable, not quietly lowered.

weak_measures:
  - name: Node B free space
    kind: telemetry
    baseline: about 151 GiB at 2026-11-13T23:50Z
    informs: urgency
    cannot_prove: that anything is bounded
  - name: bytes per tape event
    kind: telemetry
    baseline: 5.7 KB
    informs: whether R1 and R4 bite
    cannot_prove: replay completeness
---

# Storage solutions, part 3: store once

Part 2 stopped the drain and bounds what the host keeps. This mission
changes what is written in the first place. The frame, what the review
corrected and the refactors are in
[store-once-2026-11-14](../design/store-once-2026-11-14.md). Part 2 keeps
its stations S4 (snapshots), S5 (coalescing) and S7 (the collector) and its
seven-day reading; its S6 continues here as T4. Move history goes to the
[ledger](choir-storage-solutions-3-store-once-2026-11-14.ledger.md).
