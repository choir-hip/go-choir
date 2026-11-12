---
definition_version: 4

readiness: executable

review:
  reviewer: none
  frozen_ref: none
  verdict: none
  evidence_ref: owner statement 2026-11-12 (authority is the owner's direction, not a file review)

start:
  captured_at: 2026-11-12T20:30:00Z
  source:
    canonical_ref: origin/main 2f1ed42cd
    deploy_identity: choir.news deployed_commit 4faab153b182
  worktrees:
    - path: /Users/wiz/Dev/go-choir
      status: dirty
      class: user_wip
      owner: owner
      touch: forbidden
      recovery: leave in place (one untracked env backup that holds secrets)
  observed:
    - Node B has 81 GiB free of 476, 21 GiB above the 60 GiB deploy floor, falling about 6.2 GiB an hour.
    - The drain is a retry loop of failed paper snapshots (docs/problems/paper-chunk-writes-drain-node-b-disk-2026-11-12.md).
    - The paper holds about 196 GiB on Node B for a 21 GiB store: image 49.2, tape 69.4, receipts about 24, snapshot chunks 53 (docs/design/storage-pressure-frame-2026-11-12.md).
    - The paper appended 129,868 events between 09:00Z and 15:00Z; 97% landed under 300 ms after the previous one. The tape appender is busy about 88% of wall time.
    - About 35 GiB belongs to computers that no longer exist. A dry run of the destroyed-computer sweep (ts-20261112T202150Z-a31549a9) is running.
    - None of the 2026-11-11 assessment's mechanisms B1 to B11 is built.
    - Lanes in progress, none landed, none reviewed: lane/appender-wedge, lane/snapshot-loop, lane/storage-guards.
    - Unknown: what fills 28 GiB of the paper's image beyond its store; the share of tape writes that are ephemeral; the store's compression ratio; whether capture bodies repeat.

finish:
  deliver: >
    Node B stops filling. The paper costs the host a small multiple of its
    live store, growth is bounded per surface by a mechanism, and nobody is
    asked to delete under pressure.
  artifact: >
    On main and deployed: snapshots that cost the change and keep two;
    group commit on the tape; receipts folded past a verified checkpoint; a
    collector that reaps destroyed computers and superseded snapshots by
    itself; a forecast alarm with a growth meter that covers every surface;
    and a per-surface bound table in docs/ACTIVE.md with the measured value
    beside each bound.
  acceptance:
    - action: read the growth ledger for seven consecutive days after the last landing, with no hand release in the window
      proves: free space on Node B falls by at most 3 GiB a day (it fell 14 to 17 a day in early November)
      evidence_class: deployed, the hourly growth ledger on Node B
    - action: sum the paper's image, tape, receipts and chunks on Node B
      proves: at most 90 GiB without tape tiering (196 at the start); at most 50 if tiering lands
      evidence_class: deployed, host metadata (du, pg_total_relation_size)
    - action: on d@e, make a snapshot attempt fail (deadline forced low), then let one succeed
      proves: a failed attempt uploads no chunk and backs off; a good snapshot of a store changed by under 1% writes under 5% of the store's bytes
      evidence_class: deployed proof on d@e, collector log and chunk counts
    - action: on d@e, destroy the guest disk and restore from the latest snapshot plus the tape tail, after each change to snapshots, tape or receipts
      proves: the restore address still works; the local head equals the platform head; no event is lost
      evidence_class: deployed drill on d@e (adversarial drills run on d@e only)
    - action: measure the paper's appended events and receipts rows per day before and after group commit, on a day of comparable work
      proves: at least 4 times fewer events (calibration claim C57) and the write pool's wait time per write falls
      evidence_class: deployed, host receipts table (metadata) and the guest's write-pool counters
    - action: replay the 2026-11-01 to 2026-11-12 ledger through the forecast alarm
      proves: it warns at least five days before 2026-11-11 and reports the 2026-11-12 fast drain within six hours
      evidence_class: scripted test in the repo
    - action: create and destroy a test computer on staging and wait for the collector
      proves: its chunks, roots, tape and rows are gone within the reaper's window with no operator act
      evidence_class: deployed, collector log and du
  rollback: >
    Each lane reverts by commit. Deletions do not come back: every delete
    runs from a dry-run manifest, and Node A's nightly backup holds the
    files for its retention. No tape segment of a registered computer is
    dropped locally until two verified off-host copies exist.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >
    Minimise the bytes Node B holds per byte of live computer state, and
    the bytes added per day, while the restore drill keeps passing and the
    tape stays the single canonical history.
  goodharting_would_be: >
    Free space raised by one-time deletes while the slope is unchanged; a
    lower deploy floor; shorter retention windows in place of mechanisms
    that cost the change; a footprint figure that omits Node A or the
    archive; dropping tape with one copy left; snapshots made rarer so that
    recovery replays past its bound.

homotopy:
  realism_axis: >
    Unit and fault-injection tests, then d@e on staging under its own load,
    then the paper under a full day's work, then seven unattended days on
    Node B.

boundaries:
  mutation_class: red
  authority_sources:
    - owner, 2026-11-12, "You have my go-ahead to make every change, deleting all dead stuff, everything that's not useful."
    - owner, 2026-11-12, "the point is to reduce the storage pressure going forward ... we'll be making software architecture improvements that are foundationally valuable."
    - owner, 2026-11-12, "Let's make a storage solutions mission (part 2) as our next priority. We can return to autoradio after we do that."
    - docs/computer-ontology.md (Store Taxonomy), docs/choir-doctrine.md, CLAUDE.md
  must_preserve:
    - the tape is canonical; nothing a restore needs is deleted
    - the paper keeps publishing; acts on the paper are planned refreshes through deploys
    - b@c is never deleted; the real user's computer and the owner's computer are untouched
    - no switches; no limits on our own model calls
    - every delete from a reviewed dry-run manifest
  excluded:
    - a new storage engine or a move off SQLite, PostgreSQL or btrfs
    - Node A's layout beyond the archive repository
    - Autoradio and the iOS app (paused behind this mission)
  protected_surfaces:
    - tape and evidence (append path, segments, receipts)
    - snapshot and restore
    - the artifact collector (deletes)
    - store schema
    - nix host configuration (swap, timers)

stations:
  - id: S0
    name: stop the drain, release the dead
    scope: lane/snapshot-loop (backoff, witness before upload, 90-minute deadline, 3-hour unrooted window); apply the destroyed sweep after reading its manifest
    class: red; the sweep apply is black, owner go given 2026-11-12
    review: two seats
  - id: S1
    name: guards
    scope: lane/storage-guards (metadata balance, forecast alarm, meter coverage, swap 16 to 4 GiB, dead config)
    class: orange
    review: one seat
  - id: S2
    name: the appender wedge fix
    scope: lane/appender-wedge (re-send on an unclear CAS, in-place catch-up)
    class: red
    review: two seats
  - id: S3
    name: measure
    scope: the writer census, the group-commit factor, a table census of the paper's store, the image's unexplained 28 GiB, capture-body repetition
    class: green and yellow
  - id: S4
    name: snapshots cost the change, keep two
    scope: stable chunk boundaries or a page-diff snapshot, a faster witness, replay-cost trigger, immediate collection of superseded snapshots
    class: red, design record first
    review: two seats
  - id: S5
    name: group commit, then ephemeral state off the tape
    scope: one event per set of queued store writes; then the owner-ratified list of state that is not taped
    class: red, design record first
    review: two seats
  - id: S6
    name: receipts fold
    scope: one signed root per sealed segment past a verified checkpoint
    class: red
    review: two seats
  - id: S7
    name: the collector reaps by itself
    scope: destroyed computers on the hourly collector with the sweep's proof; capture bodies archived past N days
    class: red
    review: two seats
  - id: S8
    name: cold tape and a smaller store
    scope: tiering design and the archive-and-verify half; column compression after the table census; the floor recomputed from measured peaks
    class: red, owner decisions first

decisions_for_the_owner:
  - id: D1
    question: Where does the second copy of cold tape live?
    why: >
      The tape has two copies today, Node B and Node A's nightly backup.
      That backup keeps two snapshot sets, so a segment dropped on Node B
      leaves the backup at the next prune. Tiering therefore needs a
      never-pruned archive, and if that archive sits only on Node A, one
      disk failure there loses canonical history that nothing can rebuild.
    options: an object storage bucket (encrypted client-side by restic; about 100 GiB cold); a second machine the owner controls; no tiering and a bigger disk
    default: build archive and verify; drop nothing locally until two verified off-host copies exist
  - id: D2
    question: How much tape stays local?
    default: everything since the last two verified snapshots plus seven days
  - id: D3
    question: Which state may stay off the tape?
    why: it changes what "the tape is canonical" covers; a restart already never resumes work
    default: nothing leaves the tape until the owner ratifies a list from the census; group commit goes first because it changes no meaning
  - id: D4
    question: May per-event receipts past a verified checkpoint be replaced by one signed root per segment?
    default: design and two seats first; the per-event receipts go to the archive, not to nothing
  - id: D5
    question: How long are raw capture bodies kept on the host?
    default: 30 days local, then archive; no delete without the owner
  - id: D6
    question: Is file history older than a day a product promise?
    default: yes, seven days as now; no thinning
  - id: D7
    question: A bigger disk or a second device?
    default: not needed if S4 to S6 meet the footprint; decide at the seven-day reading

now:
  status: working
  slice: S0, with S1 and S2 in build and S3 running
  source_ref: origin/main 2f1ed42cd
  deploy_identity: choir.news 4faab153b182
  candidate:
    id: none yet
    state: three lanes in build
  conjecture:
    id: SS2-C1
    claim: >
      The paper's footprint is copies and per-write overhead, not content.
      Snapshots that cost the change, group commit and folded receipts
      bring it from 196 GiB to at most 90 and the host's growth from about
      7.5 GiB a day to at most 3, with no tape leaving the host.
    test: the first two acceptance items, read from Node B after S4 to S6 land
    edge: missing_oracle
    delta_o: >
      per-writer batch counts and bytes in the guest's observability, and
      chunk, capture and update trees in the growth meter, so the share of
      each surface is seen and not inferred
    scope_if_supported: S8's tiering becomes an optimisation, not a rescue
    status: proposed
    evidence_refs: [docs/design/storage-pressure-frame-2026-11-12.md]
  decision:
    what: mechanisms that cost the change come before retention windows and before tiering
    kind: architecture
    status: proposed
    evidence_ref: docs/design/storage-pressure-frame-2026-11-12.md
    owner_ratification_ref: none yet
  belief:
    believed_state: >
      The drain is understood and its fix is in build. The dead state is
      identified and its sweep is in dry run. The durable gains are
      estimates from one day's measurements.
    main_uncertainty: >
      the group-commit factor and the ephemeral share (claims C57, C58);
      whether a page-diff snapshot can keep the witness sound; what else is
      in the paper's image
    next_observation: the sweep manifest; the three lane reports; the write census
  blocker_or_risk: >
    The deploy floor is about three hours away at the current drain. The
    snapshot fix must be reviewed and deployed, or the sweep applied, before
    then. Three red lanes land close together; each gets its own review and
    its own deploy proof.
  next_action: >
    Read the sweep manifest and apply it. Review lane/snapshot-loop with two
    seats, land it, verify on staging that the paper records a snapshot and
    the births stop. Then S1 and S2, then the S3 readings decide the order of
    S4 and S5.

receipts: []

weak_measures:
  - name: Node B free space
    kind: telemetry
    baseline: 81 GiB at 2026-11-12T20:30Z
    informs: whether a release or a deploy is urgent
    cannot_prove: that growth is bounded; a one-time delete raises it
  - name: nightly backup bytes added
    kind: telemetry
    baseline: 34 to 41 GB a night
    informs: whether snapshot and tape changes reduce churn
    cannot_prove: restore soundness
---

# Storage solutions, part 2

Part 1 was the lean-system work of 2026-10-30 and the releases of
2026-11-11. It freed space and bounded three surfaces. This mission bounds
the rest by changing what is written, in the order above. The frame, the
measurements and the mechanisms are in
[storage-pressure-frame-2026-11-12](../design/storage-pressure-frame-2026-11-12.md).
Move history goes to the
[ledger](choir-storage-solutions-2-2026-11-12.ledger.md).
