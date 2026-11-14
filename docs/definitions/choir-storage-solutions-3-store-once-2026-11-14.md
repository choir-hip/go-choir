---
definition_version: 7

readiness: executable

review:
  reviewer: none
  frozen_ref: none
  verdict: none
  evidence_ref: owner statement 2026-11-14 (authority is the owner's direction; the design this file follows is reviewed at station T0, before any format change is built)

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
    - action: read the paper's tape counters over a day of ordinary work after R1 and R4 are deployed
      proves: tape growth is under 1.5 GB a day (3.6 at the start) and no object kind writes more than twice its changed bytes
      evidence_class: deployed, the guest's observability counters and Node B du
    - action: read the paper's store size and its per-table reading after R2 is deployed, seven days apart
      proves: the store is under 8 GiB and the second reading is within 10% of the first (43.8 GiB and rising at the start)
      evidence_class: deployed, store_tables detail
    - action: time the paper's snapshot after R2
      proves: under 10 minutes (37 at the start)
      evidence_class: deployed, snapshot_recorded events
    - action: count the paper's receipt rows and bytes after R3
      proves: under 0.1 GB (about 14 GB at the start)
      evidence_class: deployed, host metadata (pg_total_relation_size)
    - action: on d@e, after every format or retention change, destroy the guest disk and restore from the latest snapshot plus the tape tail; then replay the whole tape into an empty store
      proves: the restore address works, the local head equals the platform head, and replay of old and new op formats yields the same objects byte for byte
      evidence_class: deployed drill on d@e
    - action: on d@e, open a run trace older than its retirement moment
      proves: the product answers from the tape with no operator act
      evidence_class: deployed, product path
    - action: read d@e's data volume after twelve updates with no hand cleanup
      proves: the updater directory holds two releases and is under 2 GiB (20.5 at the start), and the volume's use does not grow per update
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
    name: review the design, count the rewrites
    scope: two seats on docs/design/store-once-2026-11-14.md; per-kind mean op size and writes per object in the tape counters; land the per-table store reading and read d@e and the paper
    class: green and yellow (the counters), orange (the reading)
    review: two seats on the design; none on the counters; one seat on the reading (done)
    depends_on: []
    carrier: lane/store-size (one seat accepted)
    status: working
  - id: T1
    name: a run is written once
    scope: R1 for choir.run. Prompt, result and metadata by digest or by changed-field op; an upcast for old events; replay completeness on d@e
    class: red
    review: two seats
    depends_on: [T0]
    carrier: none yet
    status: pending
  - id: T2
    name: the other kinds that rewrite themselves
    scope: R1 for choir.texture_document, choir.commitment_record, choir.lifecycle_command, choir.capture_label, choir.source_schedule, in order of measured bytes; a gate that fails a new kind whose ops embed a growing list
    class: red
    review: two seats
    depends_on: [T1]
    carrier: none yet
    status: pending
  - id: T3
    name: the guest store is a working set
    scope: R2. A read path from the product to retired history on the tape; then retirement moments for run memory, telemetry rows and superseded revisions; freed pages leave the file. Resolves store.event_table and store.telemetry_partitions (ship on or delete)
    class: red
    review: two seats
    depends_on: [T0]
    carrier: design notes "bounding the guest store" on lane/store-size, not on main yet
    status: pending
  - id: T4
    name: receipts by segment
    scope: R3. One receipt per sealed segment; per-event rows only for the unsealed tail. Takes over S6 of part 2
    class: red
    review: two seats
    depends_on: [coalescing deployed]
    carrier: none yet
    status: pending
  - id: T5
    name: amortise the toll
    scope: R4. Coalescing (lands under part 2 S5); then a per-kind dictionary or per-segment compression of sealed payloads, with a version byte. Takes over the compression half of S8 of part 2
    class: red
    review: two seats
    depends_on: [T1]
    carrier: lane/s5-coalesce-land for coalescing
    status: working
  - id: T6
    name: one home for conversation content
    scope: R5. A tool-result telemetry event names the run-memory entry instead of carrying the output again
    class: red
    review: one seat
    depends_on: [T1]
    carrier: none yet
    status: pending
  - id: T7
    name: a user's data volume is bounded
    scope: the updater prune to current and predecessor; the host re-supplies a pruned release on demand so a restore to it works; a budget line per directory of the data volume, each with its mechanism
    class: red
    review: two seats
    depends_on: []
    carrier: lane/updater-prune (round 2 of review)
    status: working
  - id: T8
    name: find the next one
    scope: a standing read. After each landing, read the per-table store reading, the tape counters by kind and the data volume on d@e and the paper; anything that grows without a named mechanism gets a problem record and a station here
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
  When T1 to T7 have landed and passed their deployed proofs, the mission
  moves to observing for seven days. Readings: the paper's tape bytes a day,
  store size, snapshot time, receipt bytes; d@e's data volume. Reader: the
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
    calibration_claim: C72
    claim: >
      Most of what a computer stores is unchanged content written again and
      history that no running computer reads. If ops carry only what
      changed and the guest store keeps a working set, the paper's daily
      growth falls from about 7 GB to under 2.5 and its store from 43.8 GiB
      to under 8, with the tape still whole and replay complete.
    test: the first two acceptance items
    edge: missing_oracle
    delta_o: writes per object and mean op size by kind in the tape counters; the per-table store reading (both in T0)
    scope_if_supported: storage stops being a station of its own; a new kind is checked by a gate
    status: active
    evidence_refs: [docs/design/store-once-2026-11-14.md, docs/problems/the-papers-store-doubles-in-eleven-days-2026-11-13.md]
  decision:
    what: write less before keeping less; nothing is retired before its read path exists; the tape is the one archive and stays whole
    kind: architecture
    status: proposed
    evidence_ref: docs/design/store-once-2026-11-14.md
    owner_ratification_ref: none yet (the design is the lead's; T0 reviews it)
  belief:
    believed_state: >
      About half of tape bytes are objects rewritten whole on each state
      change, and the guest store keeps every row it has ever projected.
      Both are inferred from one 2.9-hour window and an unmeasured store.
    main_uncertainty: what fills the paper's store by table; whether a run's replay can be made byte-identical across the format change without touching every reader of RunRecord
    next_observation: store_tables detail on d@e and the paper (claim C73); writes per object by kind
  blocker_or_risk: >
    The design has had no review. T1 changes the tape format, the most
    protected surface in the system; a mistake there is silent until a
    replay. The read path in T3 does not exist and is the largest piece of
    new work.
  next_action: land the three lanes in flight, read the store by table, send the design to two seats.

receipts: []

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
changes what is written in the first place. The frame, the measurements and
the five refactors are in
[store-once-2026-11-14](../design/store-once-2026-11-14.md). Part 2 keeps
its stations S4 (snapshots), S5 (coalescing) and S7 (the collector) and its
seven-day reading; its S6 and the compression half of S8 continue here as
T4 and T5. Move history goes to the
[ledger](choir-storage-solutions-3-store-once-2026-11-14.ledger.md).
