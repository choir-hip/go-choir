# Choir data system build: settle the frame, then build it and cut over once (2026-10-30)

Run with `/goal docs/definitions/choir-data-system-build-2026-10-30.md`.

This is a metamission whose stations are slices of this file. Phase 1 (D0,
D1, C0 to C5) gets the new system running for the whole fleet, about 85% of
the frame. Phase 2 (I1 to I7) iterates the last mile on the new system. The frame it builds is
[`docs/design/choir-data-system-frame-2026-10-30.md`](../design/choir-data-system-frame-2026-10-30.md).
Read the frame first; this file does not repeat it.

```yaml
definition_version: throughline/1
readiness: executable
readiness_authority: >-
  Owner, 2026-10-30 ~06:00 UTC: "I'd like you to start building this. I
  mean, iron out the kinks first ofc. All these reviews and design have been
  very fruitful. So keep going and then get to development." Executable on
  the owner's word (precedent: choir-selfdev-readiness-metamission-2026-10-24).
  Owner, 2026-10-30 ~06:30 UTC: "Remember, we are working at agentic speed. We can't let code review slow us down. We reviewed the plan a bunch of times for a reason. So we can do the migration in bulk, and let runtime checks be the verifier, not code review."
  So no file review is run on this goal.
  Owner, 2026-10-30 ~06:40 UTC: "Skip tonight's backup. Accelerate the migration cutover. Yes, do the fleet cutover, and delete old data. Downtime is acceptable. Basically, with our current (old) data architecture, we are hopeless. So getting the new system up and running is essential. Better to rapidly get it 85% there, and iterate for the last mile, than move slowly thinking that will improve safety. Thats false. We do our best with planning then execute with pace and overwhelming force."
  The cutover and the deletion of the old data are authorized. Downtime is
  acceptable.
review:
  reviewer: none yet
  frozen_ref: none
  verdict: none
  evidence_ref: none

metamission:
  phase_1_cutover: >-
    The new system runs the fleet as soon as possible, about 85% of the
    frame. Everything not needed to run on the new system and to delete the
    old data moves to phase 2.
  stations:
    - id: D0
      title: fold review round 1 into the frame, once
      status: complete
      depends_on: []
      exit: >-
        Frame revision 2 adopts round 1's findings: Devin, the Sonnet seat,
        and DeepSeek if it has landed. Two calls are already decided:
        - one PostgreSQL instance with two databases (control and the
          object graph), split later if load demands it;
        - the cutover rollback is fix-forward, with the old data deleted
          once checks are green, not kept a week.

        Then the frame is frozen and ledger round 0m is filed. This takes
        one pass, with no panel.
    - id: D1
      title: the build sheet
      status: complete
      depends_on: [D0]
      exit: >-
        One short document, docs/design/data-system-build-sheet-2026-10-30.md,
        giving each builder only the decisions it needs:
        - the SQLite schema per guest table;
        - the PostgreSQL schemas;
        - the tape v2 segment, envelope and batch-receipt formats;
        - the writer-epoch rule;
        - the migration mapping for each store;
        - the keep-set checks;
        - the package boundaries, so builders run in parallel without
          conflicts.

        The two detailed designs are reference, not rewritten.
    - id: C0
      title: the cutover drills
      status: pending
      depends_on: [D1]
      note: >-
        Built first and run against every landing:
        - kill -9 at random points loses no acknowledged write;
        - the differential: per computer, the migrated SQLite state answers
          like its Dolt state, and PostgreSQL answers like Store A and
          Store B;
        - keep-set counts and hashes before and after;
        - a computer boots, wakes and serves;
        - the paper publishes;
        - an owner computer opens Texture;
        - mail arrives;
        - bounded peak memory on migration and boot;
        - a stale-epoch append is refused.
    - id: C1
      title: guest store on SQLite, state only
      status: pending
      depends_on: [D1]
      note: >-
        Dolt to SQLite inside the computers. No event log is kept in the
        guest. Dolt-only calls are replaced: commits, AS OF, DOLT_HASHOF_TABLE
        in witness v2, and the Texture history checkpoint.
    - id: C2
      title: PostgreSQL for control and the object graph
      status: pending
      depends_on: [D1]
      note: >-
        The Store A and Store B code paths move to PostgreSQL, with the
        MySQL dialect ported. Per-event receipt rows go away; a segment
        index replaces them.
    - id: C3
      title: tape v2
      status: pending
      depends_on: [D1]
      note: >-
        - batched appends with one receipt per batch, kept in the segment;
        - writer epochs carried in the capability and checked inside the
          append;
        - slim envelopes for projection batches; semantic and gate-bearing
          kinds stay synchronous;
        - payloads sealed by the guest under the computer key from the
          first new event;
        - the outbox with backpressure.
    - id: C4
      title: the migration
      status: pending
      depends_on: [C0, C1, C2, C3]
      note: >-
        A migration release in which each computer converts its own Dolt
        state into SQLite at boot. The host migrates Store A and Store B
        into PostgreSQL. Each computer then:
        - seals a first snapshot (the SQLite file and the file root) into
          the blob store;
        - archives the pre-cutover tape, sealed and compressed;
        - appends the first new-tape event, naming the old head.

        It runs on d@e first and is drilled there.
    - id: C5
      title: the fleet cutover and deletion of the old data
      status: pending
      depends_on: [C4]
      note: >-
        Authorized by the owner, 2026-10-30 ~06:40 UTC:
        1. drain;
        2. migrate the fleet;
        3. switch;
        4. run C0's drills on the fleet;
        5. once the differential and keep-set checks are green, delete the
           old data.

        The old data is Store A and Store B (Dolt), the guest Dolt
        directories, projection bases, legacy pins, checkpoint JSON and the
        backup's Store A copy. The sealed pre-cutover tape archive is
        history, not old data, and stays. Downtime is acceptable.
  phase_2_iterate:
    - id: I1
      title: restore to any point, rewind and broken-computer rebuild (frame §6), with which release a rewind boots
    - id: I2
      title: the effect barrier at the host mediators (round 1's closed effect set)
    - id: I3
      title: one blob store with tiers, packs, deletion by act, pending uploads, refcount and quarantine
    - id: I4
      title: telemetry off the tape and log-kind retirement in the state
    - id: I5
      title: the audit log, anchors, per-tenant export and legal hold
    - id: I6
      title: the DEK off the guest disk, key delivery at every boot, per-object keys with key commitment
    - id: I7
      title: the off-host backup rebuilt for PostgreSQL WAL archiving and blob packs, then the backup timer re-enabled
start:
  captured_at: 2026-10-30T05:57Z
  source:
    canonical_ref: main 3f4dafb7e (docs-only commits since the deploy)
    deploy_identity: >-
      staging /health build.deployed_commit d346c6a84 (deploy receipt
      activated 2026-10-28T21:58Z, run 40774810198)
  worktrees:
    - path: /Users/wiz/Dev/go-choir
      status: clean
      class: goal_candidate
      owner: FP0
      touch: goal_owned
      recovery: none needed
  facts:
    - >-
      Node B has 99 GB free of 476 GB (79%), load 11.4 at 05:57 UTC. The
      paper's health (http://10.201.67.2:8085/health) answers 200 and
      flickers store_stalled at baseline.
    - >-
      The nightly off-host backup failed on 10-30 (restic OOM). Its next
      run, about 03:00 UTC on 10-31, rewrites the 52 GB Store A copy
      inside the data directory
      (docs/problems/offhost-backup-store-a-copy-and-restic-oom-2026-10-30.md).
    - >-
      Data stores today are Store A (platform Dolt, port 13306), Store B
      (corpus Dolt, port 13307), a Dolt store inside each computer,
      platform-artifacts CAS and tape-seg segments. Inventory in
      docs/evidence/node-b-storage-inventory-2026-10-29.md.
    - >-
      Evidence behind the frame, in docs/evidence/:
      tape-window-sizing-2026-10-30, paper-tape-composition-2026-10-30,
      new-engine-storage-measurement-2026-10-30 and
      keep-set-accounting-2026-10-29.
    - >-
      Problems open against the frame, in docs/problems/ and dated
      2026-10-30: host-holds-user-state-in-plaintext,
      restore-is-neither-forward-nor-durable,
      host-key-custody-and-audit-gaps,
      keep-set-missed-published-content-blobs and
      offhost-backup-store-a-copy-and-restic-oom.
    - >-
      Review round 1 of the frame was started at 05:5x UTC. Devin and
      DeepSeek ran through the runner (out-dir
      .agentic-consensus/frame-r1/out, log runner.log, prompt
      .agentic-consensus/frame-r1/prompt.md). The Sonnet seat ran as a
      background subagent of the session that wrote this file.
finish:
  deliver: >-
    Choir runs on the data system in the frame: one sealed, host-kept tape
    per computer with batched receipts; one blob store; PostgreSQL for
    control and for the Autopaper object graph; SQLite state in each
    computer; an anchored audit log. Every computer can be rewound to any
    point in its window and rebuilt when broken. Precommitments carry
    provable timing. Dolt and the old stores are gone.
  artifact: >-
    The landed code for phase 1, deployed. The fleet cut over once and the old data deleted.
    docs/evidence/data-system-cutover-<date>.md records the drills, the
    differential, the before-and-after resource numbers and the rollback
    window.
  acceptance:
    - action: C0's drills on the proof ring (the d@e test computer) after
        its migration
      proves: >-
        The hard gates below hold on the real system under real load
        shapes before the fleet moves.
      evidence_class: deployed proof on staging (choir.news)
    - action: the same drills after the cutover, on the fleet
      proves: the cutover kept every hard gate
      evidence_class: deployed proof
    - action: >-
        The keep-set check before and after the cutover: newspaper content,
        the Autopaper object graph, the owner's emails, Textures,
        precommitment records and the agent record.
      proves: nothing the owner keeps was lost
      evidence_class: differential counts and hashes, recorded
  hard_gates: >-
    Inherited from choir-data-architecture-selection-2026-10-28:
    - bounded peak memory: no operation's memory grows with data size;
    - kill -9 durability: no acknowledged write lost;
    - the crash and planned-restart rules;
    - a substrate-neutral witness;
    - Texture as sole writer, with a history path that has no Dolt
      coupling;
    - self-development isolation in capsules;
    - the differential;
    - security: sealing, per-owner keys, crypto-erase, separate tiers;
    - a separable data tier;
    - scale-out: shardable by computer, content-addressed, indexes
      rebuildable.

    Added by the frame:
    - rule 2's barrier;
    - restore never rewinds acts or the agent record;
    - every unwrap is audited.
  rollback: >-
    Before C5, each change rolls back by revert, and the fleet stays on the
    old system. At C5, the pre-cutover state stays on disk until the
    differential and keep-set checks are green. After the deletion, failures
    are fixed forward. The last copy of the old data off the host is the
    off-host restic snapshot of 2026-10-29 03:28 UTC; tonight's backup is
    skipped by owner direction.
  landing:
    required: true
    environment: staging (choir.news)
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Minimize the gap between the running data system and the frame. The
    measure is hard gates passed on the proof ring, then on the fleet. Every keep-set item and the restart rules must be
    preserved, and the system must get simpler (fewer homes, copies and
    mechanisms), not larger.
  goodharting_would_be: >-
    Any of these:
    - mechanisms that pass unit tests or local runs but were never drilled
      on the proof ring;
    - a cutover that "works" because it dropped kept data or history;
    - design rounds that add machinery instead of removing it;
    - two data systems left running after the cutover;
    - pieces left behind switches;
    - calling the build done on a deploy SHA without the drills.

homotopy:
  realism_axis: >-
    Where the new system runs, from least to most real:
    1. the E2E harness locally and in CI;
    2. the d@e test computer on staging, end to end;
    3. the fleet migration (C5), checked by the differential against each
       computer's pre-cutover state before that state is deleted;
    4. the fleet after the one cutover.

boundaries:
  mutation_class: >-
    D0 to D2 are green documents designing red surfaces, so they are
    reviewed as red. C0 is yellow. C1 to C5 and I1 to I7 are red: tape, keys, Texture
    canonical writes, the boot path, store schema, deletion and key
    delivery. Deleting the old data after the rollback week is black.
  authority_sources:
    - "owner rulings 2026-10-28..30, recorded in docs/definitions/choir-data-architecture-selection-2026-10-28.md (owner sections) and the frame"
    - CLAUDE.md (mutation classes, review tiers, landing loop, restart rules, No Blocking Asks)
    - docs/choir-doctrine.md (I26 desks, I27 one home per kind)
    - the frame, once frozen in D0
  settled_decisions:
    - one copy of the tape, host-canonical (owner 2026-10-30; as settled 2026-08-16)
    - host escrow of key wraps before release; passkeys on a phone TEE later (owner 2026-10-30)
    - one cutover, never two data systems (owner 2026-10-30)
    - PostgreSQL on the host, SQLite in the computers (owner leaning 2026-10-29, measured 10-30)
    - restore windows of 7, 14 or 30 days; any point in the window (owner 2026-10-30)
    - the agent record is kept like acts (owner 2026-10-30 ~06:00 UTC)
    - user computers may run at paper scale (owner 2026-10-30)
    - Qdrant removed; vectors later through pgvector (owner 2026-10-28)
    - "runtime checks are the verifier, not code review; the migration is built and landed in bulk (owner 2026-10-30 ~06:30 UTC). This supersedes, for this mission, the CLAUDE.md review tiers and the earlier design-panel order"
    - "review criteria in priority order: security, stability, simplicity, scalability, in the context of PSA and self-development (owner 2026-10-30)"
  defaults_in_force: >-
    These are the recommendations in frame §10. The owner has not objected
    ("I'm liking where we are going"), and they stand until the owner says
    otherwise:
    - the paper keeps a 7-day window;
    - the paper's agent record goes off-host cold until the analytical
      database exists;
    - history before the cutover becomes one sealed archive chained into
      the new tape's first event.
  proof_ring_and_cutover: >-
    d@e runs the migration first and is drilled. Then the fleet moves in
    one cutover (C5). Nothing widens piecemeal.
  owner_gates: >-
    None. The owner authorized the fleet cutover and the deletion of the
    old data (2026-10-30 ~06:40 UTC). Downtime is acceptable.
  backup: >-
    The nightly off-host backup is skipped by owner direction. The timer
    was stopped at 06:10 UTC on 10-30, and a runtime drop-in,
    /run/systemd/system/go-choir-offhost-backup.service.d/skip-during-cutover.conf,
    makes the service skip even if a deploy restarts the timer. I7
    rebuilds the backup for the new stores and re-enables it.
  must_preserve:
    - the keep set (newspaper content, the Autopaper object graph, the owner's emails, Textures, precommitment records, the agent record)
    - crash restarts close work, planned restarts with a marker resume; no new boot-time resumer
    - the paper and the owner's computers are never used for adversarial drills (d@e takes those); their migration in C5 is authorized
    - Texture is the sole agent writer of documents; one home and one writer per kind
    - the tape is never rewritten in place; restore is a forward event
    - "production measurements are read-only: reflink copies, MemoryMax/nice/ionice, load under ~6, paper health watched, metadata only for user data"
    - no secrets, credential values or the owner's email addresses in the repo
  protected_surfaces: [tape and evidence, Texture canonical writes, key escrow and delivery, the boot path, store schema, deletion, checkpoint and witness, self-development apply, deploy routing]
  excluded:
    - the analytical database (later)
    - passkeys and the mobile app (later)
    - multi-host distribution (interfaces must allow it; not built)
    - cross-owner dedup of private content
  verification: >-
    Runtime checks are the verifier; there is no code-review gate. In order
    of strength:
    - C0's drills on d@e and then on the fleet;
    - the hard gates;
    - the differential (migrated state answers like the Dolt state);
    - the keep-set counts and hashes;
    - deployed acceptance on staging after each landing;
    - CI;
    - the repo's static gates (check-v1-writer-purity, check-decode-roots,
      check-test-hang-guards, selftest-guest-token, check-switch-registry,
      and a nix eval when nix changed).

    A drill that has not run on the proof ring proves nothing. The
    irreversible steps are covered by runtime evidence, not review: the
    old data is deleted only after the differential and the keep-set
    checks are green on the migrated fleet (owner authorized the deletion,
    2026-10-30 ~06:40 UTC).
  build_in_bulk: >-
    C1 to C3 are built together. Parallel builders are Sonnet subagents,
    each in its own worktree off the integration branch
    `cutover/data-system`, working against C0's harness. Because new host
    code is PostgreSQL only, there is no dual path, and `main` receives the
    branch as the cutover itself (C5). CI runs on the branch's pull
    request. The rehearsal (C4) runs the migrators on reflink copies on
    Node B. See docs/design/data-system-build-sheet-2026-10-30.md,
    decisions 1 and 2.
  tempo: >-
    This mission runs under AGENTS.md "Operating Tempo". In practice:
    - Fold, freeze, build. D0 and D1 are one pass each and take hours,
      not days.
    - Fan out C0 to C3 at once: as many parallel Sonnet builders as there
      are disjoint package sets, each in its own worktree. Opus
      integrates.
    - Merge into the integration branch continuously. While CI runs, the
      next piece is being built.
    - When something fails, fix forward. A failing drill is the next
      task, not a reason to stop.
    - Take the most direct route the frame allows, and record any choice
      in the ledger in one line.
    - The goal stops only for the keep set, or a defect that would lose
      owner data.
    - Capacity: run builders and builds on this machine and on the agents
      VM, `ssh choir-agents` (8 cores, 9 GB of RAM, 27 GB free; about five
      agents, with compiles staggered). Its CI mirror is ~/ci-mirror.sh.
      Node B carries only deploys, the drills and the migration.
  agents: >-
    Opus for design and adjudication. Sonnet subagents for following a
    plan, optimizing a metric and investigations (owner 2026-10-22).
    Codex is used sparingly; the owner has few Codex tokens.

now:
  status: working
  slice: >-
    C1 to C3 build. D0 and D1 are closed. WP0 (the dbx seam) and the two
    seam contracts are on cutover/data-system at 48048f0e6. Six Sonnet
    builders run in parallel worktrees, branches cutover/wp-a to wp-f:
    A guest store, B guest object graph, C witness v3 and snapshot v1,
    D host control on PostgreSQL, E host graph on PostgreSQL, F
    migrators, nix and CI. FP0 merges them into cutover/data-system, then
    writes the C0 drills.
  source_ref: cutover/data-system 48048f0e6
  deploy_identity: d346c6a84
  candidate:
    id: frame-r1
    state: frozen
    ref: docs/design/choir-data-system-frame-2026-10-30.md
    base: 3f4dafb7e
    digest: git blob at 3f4dafb7e
    scope: [docs/design/choir-data-system-frame-2026-10-30.md]
  conjecture:
    id: DS-bridge
    claim: >-
      If the data system is rebuilt to the frame and cut over once, Choir
      gets a production-grade substrate:
      - bounded memory and disk per heavy computer (a 30-day window under
        10 GB);
      - any point restorable within the window;
      - user state sealed from the host;
      - precommitment timing provable;
      - fewer mechanisms than today.

      That advances PSA, self-development and enterprise auditability.
    test: >-
      The hard gates and C0's drills: on the proof ring, then on the
      fleet. The before-and-after resource
      numbers are set against the §12 precommitments.
    edge: missing_oracle (no drill harness exists yet; C0 builds it)
    delta_o: C0
    scope_if_supported: Node B at today's fleet and the paper's rate; the interfaces are argued for multi-host, not proved
    status: active
    evidence_refs: [docs/evidence/paper-tape-composition-2026-10-30.md, docs/evidence/tape-window-sizing-2026-10-30.md, docs/evidence/new-engine-storage-measurement-2026-10-30.md]
  decision:
    what: >-
      Build to the frame, in bulk. Runtime checks verify; there is no
      code-review gate and no further design panels (owner, 2026-10-30
      ~06:30 UTC).
    kind: architecture
    status: settled
    evidence_ref: owner 2026-10-30 messages quoted above
    owner_ratification_ref: readiness_authority
  belief:
    believed_state: >-
      The frame is coherent and matches the code on single writer. Its
      class shares rest on one measured day (replicated on a second).
    main_uncertainty: >-
      The size of the guest-store port from Dolt to SQLite (C1). The
      question is whether guest code depends on Dolt-only behavior:
      commits, AS OF, DOLT_HASHOF_TABLE in witness v2, the dirty-gated
      checkpoint in Texture history. This is the biggest schedule risk
      and decides whether C1 is days or weeks.
    next_observation: >-
      The frame round-1 verdicts. Then a Sonnet census of the Dolt-specific
      calls in internal/store, internal/computerversion and
      internal/textureowner, which can run during D1.
  blocker_or_risk: >-
    - Node B disk: 99 GB free. Tonight's backup is skipped, so the 52 GB
      Store A copy is not rewritten. Check `df` before the migration: old
      and new state coexist until C5's checks are green. The paper's
      data.img is 57 GB; the migration writes SQLite next to its Dolt
      state inside the guest volume, so check the guest volume's free
      space too.
    - The paper's health flickers.
    - Load was 11.4 at capture.
  next_action: >-
    1. D0: read round 1 in .agentic-consensus/frame-r1/out/ (devin.out,
       sonnet.out, and omp-deepseek-v41-flash.out if it has landed). Fold
       the findings into frame revision 2 in one pass, freeze it, and
       file ledger round 0m.
    2. D1: the build sheet. Start the Dolt-call census at the same time:
       a Sonnet subagent counts the Dolt-specific calls in internal/store,
       internal/computerevent, internal/computerversion,
       internal/textureowner and platform/corpusd.
    3. Fan out C0 to C3 as parallel Sonnet builders in worktrees, split by
       package boundary.
    4. C4 on d@e.
    5. C5 for the fleet.

receipts:
  - id: selection-superseded
    boundary: define
    identity: docs/definitions/choir-data-architecture-selection-2026-10-28.md (terminal receipt 2026-10-30)
    proof_refs: [docs/design/choir-data-system-frame-2026-10-30.md]
    rollback_ref: git revert of this file's commit
    disposition: the selection goal is superseded by this build; hard gates, the ledger and the differential carry forward

weak_measures:
  - name: lines in the two detailed designs
    kind: weak_signal
    baseline: object storage 2,100 lines, restore 970 lines
    desired: down in D1
    decision_use: flags machinery the frame should have removed
    cannot_prove: that the design is right
```

## Verification, not review

Owner, 2026-10-30 ~06:30 UTC: "Remember, we are working at agentic speed. We can't let code review slow us down. We reviewed the plan a bunch of times for a reason. So we can do the migration in bulk, and let runtime checks be the verifier, not code review."

- The design was reviewed through 13 object-storage revisions, a restore
  design round and the frame's round 1. Round 1's findings are folded in
  once (D0). After that, nothing waits on a reviewer.
- Every claim a station makes is settled by a run: a drill, a hard gate,
  the differential, or deployed acceptance. Round 1 found classes of
  defect that only a run settles. C0 drills the ones the cutover needs, and phase 2 drills the rest:
  - fencing, so a stale computer cannot keep writing;
  - effects held until their receipt (the barrier);
  - restore composition: acts and the agent record survive a rewind;
  - deletion safety, against chunks still used by a retained snapshot;
  - which release a rewind boots.
- If a drill fails, fix the cause and rerun the drill. If the same
  mechanism fails three times, look at the substrate (CLAUDE.md).

## Ledger

Moves go to [`choir-data-system-build-2026-10-30.ledger.md`](choir-data-system-build-2026-10-30.ledger.md).
Estimates go to the precommitment ledger,
[`choir-data-architecture-selection-2026-10-28.precommit.md`](choir-data-architecture-selection-2026-10-28.precommit.md),
as new rounds. Earlier rounds are never edited.
