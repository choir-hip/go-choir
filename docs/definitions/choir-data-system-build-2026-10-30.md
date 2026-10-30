# Choir data system build: settle the frame, then build it and cut over once (2026-10-30)

Run with `/goal docs/definitions/choir-data-system-build-2026-10-30.md`.

This is a metamission. Stations D0 to D2 are slices of this file. Stations B0
to B8 get their own goal files, authored in D2. The frame it builds is
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
  The file review is pending: one Devin seat reviews this file together with
  frame round 2 (D0).
review:
  reviewer: none yet
  frozen_ref: none
  verdict: none
  evidence_ref: none

metamission:
  stations:
    - id: D0
      title: the frame settled
      path: this file (slice)
      status: working
      depends_on: []
      exit: >-
        Devin, DeepSeek and the Sonnet seat all accept the frame (accept or
        accept_with_notes); then Codex reviews it once; its findings are
        adjudicated; the §12 numbers are filed as precommitment round 0m;
        the frame is frozen.
    - id: D1
      title: the detailed designs cut to the frame
      path: this file (slice)
      status: pending
      depends_on: [D0]
      exit: >-
        object-storage revision 14 and history-restore-audit revision 2
        fit the frame:
        - rule 4 replaces object storage §4;
        - tape segments and snapshots are blob classes;
        - the restore, broken-computer and key flows are as in frame §6
          and §7;
        - one cutover plan, with rehearsal, drain, switch and rollback.

        Panel loop as D0 until all three accept, then Codex once.
    - id: D2
      title: the build plan
      path: this file (slice)
      status: pending
      depends_on: [D1]
      exit: >-
        A throughline goal file for each of B0 to B8. Each names its tier,
        its proof-ring acceptance and its rollback, and has passed one
        Devin seat. This file's metamission block points at them.
    - id: B0
      title: the acceptance harness first
      path: authored in D2
      status: pending
      depends_on: [D2]
      note: >-
        E2E drills before any mechanism (repo testing rule):
        - kill -9 at random points with no acknowledged write lost;
        - restore to an arbitrary point, with acts and the agent record
          surviving;
        - a broken computer rebuilt from snapshot plus tape;
        - the barrier: no effect before its receipt;
        - the PSA timing proof (receipt plus anchor);
        - per-tenant export with inclusion proofs;
        - the differential: migrated state answers like the Dolt state;
        - the bounded-peak-memory gate.

        Each drill produces a repeatable artifact.
    - id: B1
      title: the guest store on SQLite, state only
      path: authored in D2
      status: pending
      depends_on: [B0]
      note: >-
        Dolt to SQLite in the computers. The guest keeps no event log, and
        log kinds retire from the state (frame §5).
    - id: B2
      title: the write path
      path: authored in D2
      status: pending
      depends_on: [B1]
      note: >-
        - the outbox and batched group commit;
        - one receipt per batch;
        - writer epochs;
        - the commit barrier;
        - the slim envelope;
        - the segment index;
        - each kind declares its class;
        - telemetry off the tape.
    - id: B3
      title: sealing and keys
      path: authored in D2
      status: pending
      depends_on: [B2]
      note: >-
        Payloads and snapshots are sealed by the guest under the computer
        key. The key is delivered at boot from host escrow before release,
        and every unwrap is audited. Closes
        docs/problems/host-holds-user-state-in-plaintext-2026-10-30.md.
    - id: B4
      title: snapshots, restore and the broken computer
      path: authored in D2
      status: pending
      depends_on: [B1, B3, B5]
    - id: B5
      title: one blob store
      path: authored in D2
      status: pending
      depends_on: [B0]
      note: >-
        Tiers, packs, deletion by act, rule-based expiry for derived tiers,
        verified backups.
    - id: B6
      title: PostgreSQL for control and for the Autopaper object graph
      path: authored in D2
      status: pending
      depends_on: [B0]
    - id: B7
      title: the audit log, anchors, export and legal hold
      path: authored in D2
      status: pending
      depends_on: [B6]
    - id: B8
      title: the cutover
      path: authored in D2
      status: pending
      depends_on: [B1, B2, B3, B4, B5, B6, B7]
      note: >-
        - a migration release;
        - a rehearsal on reflink copies of the paper and the owner's
          computers;
        - the owner's go;
        - drain, migrate, switch;
        - the differential gate;
        - rollback for a week (the old directories read-only);
        - then the old system deleted on the owner's word.

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
    The landed code for B0 to B8, deployed. The fleet cut over once.
    docs/evidence/data-system-cutover-<date>.md records the drills, the
    differential, the before-and-after resource numbers and the rollback
    window.
  acceptance:
    - action: B0's drills on the proof ring (the d@e test computer) and in
        the rehearsal on copies of the paper and the owner's computers
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
    Before the cutover, each station's rollback is its own revert, and the
    fleet stays on the old system. After it, the rollback is the previous
    release, with the old data directories kept read-only for a week.
  landing:
    required: true
    environment: staging (choir.news)
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Minimize the gap between the running data system and the frame. The
    measure is hard gates passed on the proof ring, then in the rehearsal,
    then on the fleet. Every keep-set item and the restart rules must be
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
    3. the rehearsal on reflink copies of the paper and the owner's
       computers at their real sizes (never the live ones);
    4. the fleet after the one cutover.

boundaries:
  mutation_class: >-
    D0 to D2 are green documents designing red surfaces, so they are
    reviewed as red. B0 is yellow. B1 to B8 are red: tape, keys, Texture
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
    - "the review order is Devin, DeepSeek and a Sonnet subagent; Codex (gpt-6.1-sol high) only once all three accept (owner 2026-10-30)"
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
    The proof ring and the single cutover are reconciled this way.
    "On, never off everywhere; widen by ring" (owner 2026-10-28) is
    satisfied by running the new system whole on the d@e test computer
    and in the rehearsal. "Never two data systems" (owner 2026-10-30) is
    satisfied because no mechanism widens to the fleet piecemeal: the
    fleet moves once, in B8. The host-side PostgreSQL and blob store that
    the proof ring needs run beside the old stores only until B8.
  owner_gates: >-
    These are the only blocking asks, under No Blocking Asks:
    - executing the fleet cutover in B8;
    - deleting the old data directories after the rollback week;
    - any write to the paper's or the owner's live computers beyond what
      a normal deploy does.

    Everything else proceeds on continuous authority.
  must_preserve:
    - the keep set (newspaper content, the Autopaper object graph, the owner's emails, Textures, precommitment records, the agent record)
    - crash restarts close work, planned restarts with a marker resume; no new boot-time resumer
    - the paper and the owner's computers are never used for adversarial or proof steps
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
  agents: >-
    Opus for design and adjudication. Sonnet subagents for following a
    plan, optimizing a metric and investigations (owner 2026-10-22).
    Codex is used sparingly; the owner has few Codex tokens.

now:
  status: working
  slice: >-
    D0, frame review round 1. Devin returned accept_with_notes (F1 to F5)
    and the Sonnet seat returned send_back (B1 to B7); both outputs are in
    .agentic-consensus/frame-r1/out/ (devin.out, sonnet.out). DeepSeek was
    still running at authoring (omp-deepseek-v41-flash.out). The two
    reviews overlap on:
    - the PSA proof bound;
    - restore and replay semantics;
    - a closed effect set and the barrier;
    - fencing and writer epochs;
    - deletion safety;
    - the release a rewind boots;
    - custody honesty.
  source_ref: main 3f4dafb7e + this file's commit
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
      The hard gates and B0's drills: on the proof ring, in the rehearsal
      at real sizes, then on the fleet. The before-and-after resource
      numbers are set against the §12 precommitments.
    edge: missing_oracle (no drill harness exists yet; B0 builds it)
    delta_o: B0
    scope_if_supported: Node B at today's fleet and the paper's rate; the interfaces are argued for multi-host, not proved
    status: active
    evidence_refs: [docs/evidence/paper-tape-composition-2026-10-30.md, docs/evidence/tape-window-sizing-2026-10-30.md, docs/evidence/new-engine-storage-measurement-2026-10-30.md]
  decision:
    what: >-
      Build to the frame. The design panel is Devin, DeepSeek and Sonnet,
      then Codex once. Build review follows the CLAUDE.md tiers (red
      stations at tier 2).
    kind: architecture
    status: settled
    evidence_ref: owner 2026-10-30 messages quoted above
    owner_ratification_ref: readiness_authority
  belief:
    believed_state: >-
      The frame is coherent and matches the code on single writer. Its
      class shares rest on one measured day (replicated on a second).
    main_uncertainty: >-
      The size of the guest-store port from Dolt to SQLite (B1). The
      question is whether guest code depends on Dolt-only behavior:
      commits, AS OF, DOLT_HASHOF_TABLE in witness v2, the dirty-gated
      checkpoint in Texture history. This is the biggest schedule risk
      and decides whether B1 is days or weeks.
    next_observation: >-
      The frame round-1 verdicts. Then a Sonnet census of the Dolt-specific
      calls in internal/store, internal/computerversion and
      internal/textureowner, which can run during D1.
  blocker_or_risk: >-
    - Node B disk: 99 GB free. The 10-31 ~03:00 UTC backup rewrites a
      52 GB copy, so check `df` before heavy steps, and do not run the
      rehearsal until there is room for reflink copies plus PostgreSQL.
    - The paper's health flickers.
    - Load was 11.4 at capture.
  next_action: >-
    1. Collect round 1: .agentic-consensus/frame-r1/out/{devin.out,
       omp-deepseek-v41-flash.out} (manifest.tsv shows the status) and
       the Sonnet seat's result.
    2. Adjudicate each finding by the owner's criteria order.
    3. Revise the frame (commit as green; append a revision log to the
       frame).
    4. Run round 2 with the same three seats, plus one Devin seat on this
       goal file.
    5. On three accepts, run Codex once:
       `--include codex --codex-model gpt-6.1-sol --cwd /Users/wiz/Dev/go-choir`.
    6. Freeze the frame, file ledger round 0m, and move to D1.

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

## How to run the review loops (D0, D1)

- **Devin and DeepSeek.** Run
  `~/.claude/skills/agentic-consensus/agentic-consensus-runner.sh --include devin,omp-deepseek-v41-flash --keep-going --timeout-seconds 2400 --prompt-file <prompt> --out-dir <dir>/out --cwd "$(mktemp -d)"`.
  Reuse `.agentic-consensus/frame-r1/prompt.md` as the template, and put
  each round in its own directory (`frame-r2`, `design-r1`, ...).
- **Sonnet.** Use the Agent tool with model `sonnet`: read the prompt file,
  stay read-only, return the review.
- **Stopping.** After three rounds on one mechanism without convergence,
  look at the substrate (CLAUDE.md, "Escalate on dead ends"). Do not add a
  fourth round.
- **Adjudication.** Each finding is adopted, deferred to a named station,
  or rejected with a reason. Rank by security, stability, simplicity,
  scalability, judged against PSA and self-development. Prefer removing
  machinery to adding it.

## Ledger

Moves go to [`choir-data-system-build-2026-10-30.ledger.md`](choir-data-system-build-2026-10-30.ledger.md).
Estimates go to the precommitment ledger,
[`choir-data-architecture-selection-2026-10-28.precommit.md`](choir-data-architecture-selection-2026-10-28.precommit.md),
as new rounds. Earlier rounds are never edited.
