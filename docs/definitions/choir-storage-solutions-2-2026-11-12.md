---
definition_version: 7

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
    scope: backoff after a failed snapshot counted from the attempt's end, witness before upload, 90-minute deadline, a free-space guard during the capture; apply the destroyed-computer sweep from a read manifest
    class: red; the sweep apply is black, owner go given 2026-11-12
    review: two seats
    depends_on: []
    carrier: "main 0d2f0bd46, deployed 2026-11-12T22:31Z, on the paper since 22:53Z, snapshot recorded 23:28Z; the drain stopped (claim C62 true, free space rose 6.7 GiB in the six hours to 2026-11-13T05:35Z). The sweep apply moved to S7: no dry run has finished"
    status: proved
  - id: S1
    name: guards
    scope: metadata balance, hourly forecast with a fast-drain line, meter coverage of capture bodies and platform updates, swap 16 to 4 GiB
    class: orange
    review: one seat (Gemini 4 Hydrogen; REPAIR, two blocking findings fixed)
    depends_on: []
    carrier: main cca135073, deployed 2026-11-12T20:53Z; first forecast run 21:25Z exit 0
    status: proved
  - id: S2
    name: the appender wedge fix
    scope: re-send on an unclear CAS reply, in-place catch-up
    class: red
    review: two seats
    depends_on: []
    carrier: main d6b88cd63 (both seats accepted on round 3), CI 43373203542, deployed 2026-11-13T05:55Z; scripts/chaos/run s2-corpusd-crash on d@e passed, but no re-send or catch-up ran on staging, so the lost-reply path is proved by the fault tests only (claim C63 partial)
    status: landed
  - id: S3
    name: measure
    scope: the write census (done); waiters at the appender; a table census of the paper's store; the image's unexplained 28 GiB; capture-body repetition
    class: green and yellow
    review: none
    depends_on: []
    carrier: docs/evidence/tape-write-census-2026-11-12.md
    status: working
  - id: S4
    name: snapshots cost the change, keep two
    scope: stable chunk boundaries or a page-diff snapshot, a faster witness, a replay-cost trigger, immediate collection of superseded snapshots; root-time chunk verification and a capture that does not span hibernation, then the shorter unrooted-chunk window (docs/problems/snapshot-root-recorded-without-chunk-check-2026-11-12.md)
    class: red, design record first
    review: two seats
    depends_on: [S0]
    carrier: part 1 (the host refuses a root whose chunks it does not hold; the collector keeps a chunk a live root names) main afe50a926, both seats accepted on round 3, CI 43379521482, deployed 2026-11-13T06:54Z. Not started, the shorter chunk window and snapshots that cost the change
    status: working
  - id: S5
    name: fewer events for the same record
    scope: counters (ops and bytes per kind, waiters at the appender); delete tape.group_commit and both loss windows; run-memory writes batched at the call site; lossless coalescing of queued writers with no setting (ruling D8); then the four calls per event
    class: red, design record first
    review: two seats
    depends_on: [S2]
    carrier: "part 1 (always-on counters, the appender depth histogram, corpusd calls per event; design docs/evidence/tape-write-coalescing-design-2026-11-12.md) main e392ac824, one seat accepted on round 2, CI 43384729837 (third attempt), deployed 2026-11-13T07:54Z. The paper over 5.4 hours to 2026-11-13T16:40Z: 90% of 147,466 appends entered at depth 3 or more (53% at 5 to 8), mean call 793 ms, 6.03 corpusd calls per event (two heads and two pins per append); coalescing is being built"
    status: working
  - id: S6
    name: receipts fold
    scope: one signed root per sealed segment past a verified checkpoint; per-event receipts go to the archive
    class: red
    review: two seats
    depends_on: [S5]
    carrier: none yet
    status: pending
  - id: S7
    name: the collector reaps by itself
    scope: destroyed computers on the hourly collector with the sweep's proof; a sweep job that survives a service restart (the first dry run died at the 20:53Z deploy); capture bodies archived past 30 days
    class: red
    review: two seats
    depends_on: [S0]
    carrier: "sweep progress that survives a restart, main 421288af7, both seats accepted on round 2, CI 43376520250, deployed 2026-11-13T06:15Z. The walk is too slow to finish on the paper (docs/problems/tape-sweep-walk-probes-every-segment-per-event-2026-11-13.md); repaired by f4276a0ba and dd43ff6b1 (a digest lookup in the view, 12 times faster on the paper, claim C66). Applied 2026-11-13T12:12Z to 12:29Z in five capped rounds: all 251 destroyed computers removed, 467,913 files and 26.89 GB deleted, no errors (claim C67 true). No scheduler yet, so the collector does not reap by itself"
    status: working
  - id: S8
    name: a smaller store, and the archive half of cold tape
    scope: column compression after the table census; the floor recomputed from measured peaks; archive and verify cold tape off-host, dropping nothing locally (ruling D1)
    class: red
    review: two seats
    depends_on: [S3]
    carrier: none yet
    status: pending
  - id: S9
    name: status scripts
    scope: scripts/paper-usage-status and scripts/storage-status, each runnable and tested alone, and a third that composes them into the hourly report; the session scratchpad script retires
    class: yellow
    review: none
    depends_on: [S0, S1]
    carrier: main 29666dc47 (scripts/status-paper-usage, scripts/status-storage, scripts/status-hourly; tests in scripts/gates); used for every hourly tick since 2026-11-13T04:35Z
    status: proved

owner_rulings:
  - on: 2026-11-12
    decision: D1
    said: "let's go with the default; don't drop the old tape segments from node b yet. We will revisit this before we ship autoradio to TestFlight."
  - on: 2026-11-12
    decision: D2 to D7
    said: "I go with your defaults"
  - on: 2026-11-12
    decision: D8
    said: "group commit, I decided it off but it should be on, huh?"
    read_as: yes to coalescing with no loss window and no setting; the two loss windows stay deleted; counters land first

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
  - id: D8
    question: May the lossless half of tape group commit ship, with no setting, after your 2026-11-03 ruling to delete tape.group_commit?
    why: >
      The census found group commit already built behind the default-off
      setting tape.group_commit, which you ruled deleted ("i dont like the
      sound of losing events just before a crash"). The loss came from two
      windows (telemetry stretched to 10 s, run-memory messages joining it).
      Leader and follower coalescing of writers already waiting loses
      nothing: every caller still returns only after its event is on the
      tape. Estimate 519k events a day to about 130k.
    default: delete the setting and both windows now, as ruled; add the waiters-at-enter counter; ship coalescing only on the owner's word
  - id: D7
    question: A bigger disk or a second device?
    default: not needed if S4 to S6 meet the footprint; decide at the seven-day reading

observation_plan: >
  When every station has landed and passed its deployed proof, this mission
  moves to `observing` (skills/throughline, Settlement). Readings: the hourly
  growth ledger and the go-choir-disk-forecast line on Node B; the paper's
  footprint by surface, daily; snapshot_recorded spacing. Reader: the forecast
  unit on Node B, and the combined hourly status tick once it is in the repo.
  Window: seven days from the last landing. Pass: the first two acceptance
  items. Tripwire: a forecast warning, a failed paper snapshot, or a day with
  more than 3 GiB lost returns the mission to `working`. Autoradio resumes
  when this mission enters `observing`, not when it completes.

now:
  status: working
  slice: "the live station again (owner, 2026-11-13: take them to the point of diminishing marginal returns). Lanes: S4 design (snapshots that cost the change, the chunk window), S5 build (coalescing, two calls fewer per event), S7 build (the sweep on a schedule), small guards"
  source_ref: origin/main dd43ff6b1
  deploy_identity: choir.news dd43ff6b11fe
  candidate:
    id: none
    state: none
  conjecture:
    id: SS2-C1
    calibration_claim: C61
    claim: >
      The paper's footprint is copies and per-write overhead, not content.
      Snapshots that cost the change, fewer events for the same record and
      folded receipts bring it from 196 GiB to at most 90 and the host's
      growth from about 7.5 GiB a day to at most 3, with no tape leaving the
      host.
    test: the first two acceptance items, read from Node B after S4 to S6 land
    edge: missing_oracle
    delta_o: >
      per-writer batch counts and the waiters-at-enter histogram in the
      guest's observability (landed with e392ac824); chunk, capture and update
      trees in the growth meter (landed with S1)
    scope_if_supported: cold-tape tiering becomes an optimisation, not a rescue
    status: active
    evidence_refs: [docs/design/storage-pressure-frame-2026-11-12.md, docs/evidence/tape-write-census-2026-11-12.md]
  decision:
    what: mechanisms that cost the change come before retention windows and before tiering; no tape leaves Node B
    kind: architecture
    status: settled
    evidence_ref: docs/design/storage-pressure-frame-2026-11-12.md
    owner_ratification_ref: owner_rulings, 2026-11-12, D1 to D8
  belief:
    believed_state: >
      The drain has stopped: the paper's chunks are flat at 49.3 GiB and its
      last snapshot (23:28Z) stands. What moves Node B's free space
      now is deploys (about 5 GiB transient each, and 3.8 GiB guest
      releases held for a while). The wedge fix, the root-time chunk check,
      restart-surviving sweep progress, the counters and the status scripts
      are deployed. The footprint itself has not shrunk: about 45 GiB of
      unrooted paper chunks age out from 2026-11-13T08:40Z, and the destroyed
      computers are gone: the sweep applied on 2026-11-13 (251 computers,
      26.89 GB), and Node B has 117.9 GiB free.
    main_uncertainty: >
      whether the paper's appender depth in steady state matches its first
      reading (89% of appends enter at depth 3 or more, most at 5 to 8, about
      a second each; six corpusd calls per tape event); how many of the
      paper's events are in no segment; how many guest releases the host
      keeps
    next_observation: the paper's depth histogram after a day; the collector's passes as the unrooted chunks age out (about 3 GiB a pass since 09:00Z)
  blocker_or_risk: >
    The sweep runs only when someone calls it; no scheduler exists, so newly
    destroyed computers wait. Each apply writes a small orphan archive on Node B
    even with no_archive set (0.6 MB so far).
    lane/restart-cancels-capture is parked after five review rounds (a deploy
    still waits out a capture, now rare). The lost-reply path of the wedge fix
    has not run on staging. The failed-attempt path of the snapshot fix has
    not been seen on the paper.
  next_action: >
    Read the paper's appender
    depth after a day, then build coalescing and remove the second head read
    and pin per append. Design S4's shorter chunk window. The
    hourly status script is the tripwire while Autoradio is the live station.

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
