# Choir lean system: shrink the paper, clean the nodes, finish phase 2, make the docs true (2026-10-30)

Run with `/goal docs/definitions/choir-lean-system-2026-10-30.md`.

This metamission supersedes
[`choir-data-system-build-2026-10-30.md`](choir-data-system-build-2026-10-30.md),
whose phase 1 (the fleet cutover to PostgreSQL on the host and SQLite in the
computers) landed on 2026-10-30. Its phase 2 (I1 to I7) carries over here
unchanged in design; the frame
[`docs/design/choir-data-system-frame-2026-10-30.md`](../design/choir-data-system-frame-2026-10-30.md)
stays the design source. This file adds what the cutover left: the paper is
still 31 GB, the nodes hold dead data, a crashed computer stays down, and the
docs describe a system that no longer exists.

```yaml
definition_version: throughline/1
readiness: executable
readiness_authority: >-
  Owner, 2026-10-30 ~22:55 UTC: "let's have another aggressive night. What I
  mean by that is all the paper stuff you just mentioned, I want to do that.
  I want to do what you call the overnight priority stuff, touching D at E
  and the paper as well. And I want to do phase two. And I want to make sure
  we free up all the disk space. and I want to tie up as many loose ends as
  possible."
  Owner, ~23:00: the docs revision is a major loose end: triage docs/ with
  Haiku subagents; archive procedural docs older than two weeks, delete older
  than four (git history keeps them); consolidate; the docs reflect the
  current architecture.
  Owner, ~23:05: "all we need on Node A is to be able to run [Mike, read for "mic"] safely
  and have our [Choir] agents VM for spare work capacity ... a lot of the
  backed up state is something we can get rid of. As long as we have our
  latest backup generation."
  Owner, ~23:08: backups do not block the night; streamline the backup and
  run it after the night's work; the risk of no backup tonight is accepted
  ("we don't have any real valuable data that needs to be backed up ...
  It's pre-release").
  Owner, ~23:12: chose a new superseding mission over extending the build.
  The data system build's Operating Tempo (CLAUDE.md) still governs: runtime
  checks are the verifier, no file review.
review:
  reviewer: none (Operating Tempo, owner 2026-10-30)
  frozen_ref: none
  verdict: none
  evidence_ref: none
metamission:
  stations:
    - id: L0
      title: free the disks on Node A and Node B
      status: working
      depends_on: []
      exit: >-
        Node A holds only what it needs: Mike's staging stack, the agents VM,
        the workstations and their latest snapshot, the latest backup
        generation. Node B holds no dead old-system data (projection bases,
        Qdrant state, the rehearsal bench, interim dumps once a backup holds
        them), and its Nix store and guest releases are at retention. Each
        deletion is recorded with bytes freed.
    - id: L1
      title: the paper an order of magnitude smaller
      status: working  # target relaxed by the owner, 2026-10-31: "if 3gb isnt realistic, then we we settle for what is"; the reviews estimate 8-10 GB live
      depends_on: []
      exit: >-
        The paper's state.db is at most 3 GB (from 31 GB), measured on the
        live computer after a planned VACUUM, with replay-completeness green,
        the paper publishing, and every reader of old events either served
        from the host tape or refused visibly. Levers, in order of size: the
        guest event index windowed to the restore window, telemetry
        retirement on for every computer (I4's state half), class retention,
        VACUUM at a planned restart. d@e first, then the paper.
    - id: L2
      title: a crashed computer comes back, a crash loop does not
      status: complete  # 2026-10-31 02:26-02:39, d@e drill PASS (ledger)
      depends_on: []
      exit: >-
        A crashed computer restarts by itself with its work closed (the
        owner's rule), and after 3 crashes in 30 minutes it holds with a
        visible cause until its owner starts it. Drilled on d@e by killing
        its VM process: one kill comes back with work closed; three kills
        hold.
    - id: L3
      title: the docs describe the running system
      status: working
      depends_on: []
      exit: >-
        docs/ triaged (Haiku seats, one verdict per file), applied: procedural
        docs modified before 2026-10-16 archived, before 2026-10-02 deleted,
        overlapping records merged, the archive pruned of records nobody
        needs; every citer of a moved or deleted doc fixed; the normative
        docs (doctrine, vision, ontology, product doctrine, standing
        questions, the computer ontology) updated where they name a deleted
        mechanism. Docs-only: the docs truth checker is the gate.
    - id: L4
      title: phase 2 of the data system (I1, I2, I3, I5, I6; I4's tape half)
      status: pending
      depends_on: []
      exit: >-
        Each item lands to staging with its drill on d@e, as designed in the
        frame: I1 restore to any point (checkpoint as a snapshot reference),
        I2 the effect barrier, I3 one blob store, I4 telemetry off the tape,
        I5 the audit log, I6 the key at every boot. Order by what the night
        reaches; durability first.
    - id: L5
      title: the off-host backup, streamlined (I7)
      status: pending
      depends_on: [L0, L1]
      exit: >-
        A new restic repository on Node A takes what restores the system and
        nothing rebuildable: PostgreSQL (dump now, WAL archiving next), the
        tape, the file CAS, secrets minus the excluded keys. No guest images
        (the tape rebuilds them; drills G and H). A run finishes, a restore
        drill reads it back, and only then is the old repository deleted.
    - id: L6
      title: loose ends and the handoff
      status: pending
      depends_on: [L0, L1, L2, L3]
      exit: >-
        The digest retry inside the hour, mail arrival proof, R0n-09 and
        R0k-06 settled on the paper's snapshot, VM_BOOT_READY_TIMEOUT,
        R0m-01 and R0m-04; the closing report; docs/ACTIVE.md, the mission
        graph and the next goal file.

start:
  captured_at: 2026-10-30T23:15Z
  source:
    main: 3b1930690
    deployed: 485d3726c (deployed 2026-10-30 22:23:25)
  worktrees:
    - path: .claude/worktrees/og-redesign
      disposition: this mission's integration worktree, clean at start
    - path: the crash-restart builder's agent worktree (branch crash-restart)
      disposition: L2 candidate, integrated by the orchestrator
    - path: /Users/wiz/Dev/go-choir (branch offhost-pg)
      disposition: main checkout, clean; not touched
    - path: other .claude/worktrees (about 320 in total)
      disposition: other sessions' work; not in scope, not touched
  facts:
    - The paper's state.db is 31 GB (og_objects about 9.5 GB, of which choir.event rows about 4.9 GB; the rest is believed to be computer_event_index, 5.1M rows, and indexes; not yet measured exactly).
    - Node B 476 GB, 90 GB free. platform-artifacts 210 GB (file-cas-chunks 82, projection-base 40, tape-seg 27, pin-receipts 20, computer-event-payload 19, computer-event 9.5), vm-state 79, Nix store 66, PostgreSQL 20, guest-releases 15.
    - Node A at 23:00 was full (145 MB free); tonight's backup run failed with 507 Insufficient Storage after uploading 32 GB. By 23:10 the orphaned packs, 144 old workstation snapshots and 389 GiB of Nix store garbage were gone; Node A has 359 GB free. The old restic repository keeps its 5 latest snapshots (7 forgotten); its prune was killed for memory and is not needed (L5 replaces the repository).
    - A crashed computer becomes degraded and stays down until its owner starts it: the proxy wakes only hibernated or stopped computers (58311f268, an agent's change on 2026-10-26, not an owner rule).
    - docs/ holds 1,298 files (problems 516, evidence 276, reports 140, archive 117, definitions 104, design 46, top level 45).

finish:
  deliver: >-
    Choir runs lean: the paper fits in a few gigabytes, the nodes carry no
    dead data, a crash does not need the owner, phase 2 makes every computer
    restorable to any point with keys delivered at boot, a streamlined backup
    leaves the building, and a newcomer reading docs/ learns the system that
    runs.
  artifact: >-
    Pushed main with L1, L2 and L4 deployed to staging; the paper's measured
    store size; the disk ledger for both nodes; the applied docs triage; one
    off-host snapshot of the new repository with a restore drill; the closing
    report.
  acceptance:
    - L1: the paper's state.db size before and after, replay-completeness on the paper, the paper's /health and a published edition after the planned restart.
    - L2: the d@e kill drill (one kill comes back, three kills hold).
    - L4: each item's drill on d@e on the deployed release.
    - L5: restic snapshot id, restore drill result.
    - L3: the docs truth checker report on the docs commit.
  rollback: >-
    Source: git revert per commit. The paper: VACUUM and windowing delete only
    what the host tape holds; the tape rebuilds the store (drills G and H).
    Disk deletions are of dead or rebuildable data only; Node A's old restic
    repository stays until the new one has a verified snapshot.
  landing: commit -> push origin main -> CI -> staging deploy identity -> deployed acceptance (L3 is docs-only and takes the docs truth checker instead).

value:
  better_means: >-
    Minimize stored bytes per unit of live user state (the paper's store,
    Node A, Node B) and the distance between docs/ and the running system,
    while every computer stays recoverable from the host and the tape stays
    sealed and whole.
  goodharting_would_be: >-
    Shrinking the paper by deleting history the host tape does not hold;
    freeing disk by deleting something a restore needs; "revising" the docs
    by archiving everything so nothing is wrong because nothing is said.

homotopy:
  realism_axis: >-
    A read-only copy of the paper's store on Node B -> d@e live -> the paper
    live at a planned restart -> the fleet. The same code at each point; only
    the computer and its size change.

boundaries:
  mutation_class: >-
    L0 black-adjacent (deletion) limited to dead or rebuildable data, under
    the owner's grants above; L1 and L4 red (store schema, tape, keys, boot
    path); L2 red (VM lifecycle); L3 green; L5 orange.
  authority_sources: [the owner statements in readiness_authority, CLAUDE.md Operating Tempo]
  must_preserve:
    - the keep set (newspaper content, the Autopaper object graph, the owner's emails, Textures, precommitment records, the agent record)
    - the sealed tape, immutable; agent traces are the users' data, sealed and preserved
    - crash restarts close work; planned restarts with a marker resume; no crash-restart loop
    - no secrets or the owner's email addresses in the repo
  excluded:
    - adversarial drills on the paper or the owner's live computers (d@e only)
    - reading document, capture or mail content (sizes and counts only)
    - sending email on the owner's behalf
    - the privacy policy text (legal; flag, the owner decides)
  verification: >-
    Runtime checks, as in the build: repo gates, CI, the deploy, drills on
    d@e, replay-completeness, measured sizes. A check that has not run proves
    nothing.

now:
  status: working
  slice: >-
    2026-10-31 ~06:30 UTC. L2 complete (d@e drill). L1 in flight: telemetry
    retirement now really on fleet-wide (the settings-default fix), the
    paper's first backlog clearing in kind-group passes (about 528,000
    choir.event rows by 05:18; resumed at 05:47 after the 05:26 restart);
    boot never compacts; the event index catch-up runs in the background,
    paced. One-tape durability (docs/design/one-tape-durability-2026-10-31.md)
    built to: R7 trace log deleted, telemetry batching on d@e, bundle export,
    verify, import, format 2 packs, delta bundles. CI: test suite 2,109 s to
    849 s measured on six packages; both shard sets weighted.
  source_ref: main 435136164
  deploy_identity: 435136164 (deployed 2026-10-31 06:18)
  conjecture: >-
    If the guest store keeps only the restore window's events plus live object
    state, and the host tape holds the rest sealed, then the paper's store
    drops by about a factor of three to four (32 GB to 8-10 GB live) with no
    reader losing an answer it needs. TEST: state.db measured after the first
    full retirement pass and a compact snapshot with a planned rebase (design
    6b); replay-completeness green; the paper publishing. EDGE
    (missing_oracle): acceptance re-synthesis reading tool.result older than
    seven days (an owner decision, below). Status: testing.
  belief:
    believed_state: >-
      Retirement removes the bulk of choir.event, but SQLite keeps the freed
      pages, so the file shrinks only through a compact snapshot and a
      planned rebase; boot must stay light.
    main_uncertainty: >-
      How much the og_meta secondary indexes hold after retirement, and the
      rebase's restart cost on the paper.
    next_observation: >-
      Measured 06:20: 32.4 GB file, 8.7 GB free pages, about 23.6 GB live,
      1.96M choir.event rows. Next: the same counts after S5's compact
      snapshot and planned rebase on d@e, then the paper.
  blocker_or_risk: >-
    Owner decisions settled 2026-10-31: (1) the tape is the tape, kept for
    the computer's life; no per-item deletion promise; binary blobs may be
    deleted; whole-computer deletion (destroy the computer key) comes later,
    behind several confirmations, after sync to the owner's machine. The
    privacy policy now says so. (2) Tool results leave the working store
    after seven days except those accepted evidence cites (to build);
    restoration is for failed self-development, about a week back; long-term
    history goes to the user's synced machine, on by default (Mac pull, S5a).
    S5 landed on the paper 12:53 (planned restart of e7c099e6a): 32.4 GB to
    20.4 GB, 0 free pages, incremental auto_vacuum, replay of 9,673 events,
    publishing. No off-host copy until the Mac pull or L5.
  next_action: >-
    Keep tool.results cited by accepted evidence in
    retirement; the Mac pull over delta bundles as the default long-term
    history (S5a); L5; L6's handoff. Small follow-ups in the ledger.
receipts:
  - id: build-superseded
    boundary: supersede
    identity: docs/definitions/choir-data-system-build-2026-10-30.md (phase 1 landed 2026-10-30; terminal receipt in its now card)
    proof_refs: [docs/definitions/choir-data-system-build-2026-10-30.ledger.md, docs/reports/lost-disk-comes-back-whole-2026-10-30.md]
    rollback_ref: git revert of this file's commit
    disposition: phase 2 (I1 to I7) carries over as L4 and L5; the frame stays the design source; the build's ledger stays closed

weak_measures:
  - name: docs file count
    kind: weak_signal
    baseline: 1,298 files under docs/
    desired: down sharply in L3
    decision_use: shows the triage was applied
    cannot_prove: that the remaining docs are true
```

## Ledger

Append-only, in
[`choir-lean-system-2026-10-30.ledger.md`](choir-lean-system-2026-10-30.ledger.md).
