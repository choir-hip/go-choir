---
definition_version: 4

# Tape segment store, stage A2 (the production writer behind a flag) and
# the copy-only repack of the legacy tape files. Run as
# `/goal docs/definitions/choir-tape-segments-a2-repack-2026-10-21.md`.
# Owner 2026-10-21 ~03:20 UTC (docs/reports/owner-notes-2026-10-14.md
# section 33): "we should shard the flat directories... we need a durable
# fix for the backup verify". The durable fix already designed for these
# directories is the segment store, not prefix sharding
# (docs/reports/tape-volume-forward-fix-and-surgery-2026-10-19.md, sections
# 4, 6.1, 9.6 and appendix A); A1 (the reader, dual-read) landed in
# 4333900a9. Prefix sharding would touch the same sites (tape sweep, GC,
# archive verification, vmctl's artifact verifier) and then be superseded.
readiness: drafted

review:
  reviewer: none yet
  frozen_ref: none
  verdict: none
  evidence_ref: none

start:
  captured_at: 2026-10-21T03:35:00Z
  source:
    canonical_ref: main 8d027370b
    deploy_identity: staging https://choir.news at 7bb7b3e65 (8d027370b in CI)
  worktrees:
    - path: session scratchpad tape-wt (branch feat/tape-segments-a2, from origin/main 8d027370b)
      status: A2 writer, built test-first by a subagent, not pushed
      class: goal_candidate
      owner: this session
      touch: goal_owned
      recovery: branch
  observed:
    - >-
      Node B's artifact store holds 9.7 million files in about 1,440
      directories (2026-10-21 01:45 UTC): sha256/pin-receipts 4.58 million,
      sha256/computer-event 2.29 million, sha256/computer-event-payload
      2.29 million. restic stores each directory as one tree blob; the
      daily restic check held the backup unit at its memory line for hours
      on runs 3 and 4 and was moved to the weekly verify
      (docs/problems/no-off-host-backup-2026-10-20.md section 13 items 18
      and 19). The weekly verify still walks the same trees.
    - >-
      Every tape event costs about four small files; nothing deletes a tape
      file; 87% are under 2 KiB and live inline in btrfs metadata
      (docs/problems/platform-artifact-small-file-growth-2026-10-19.md).
    - >-
      internal/tapestore (A1) reads segments first and the legacy flat file
      second in every production read site; no production code writes a
      segment; a guard test keeps the fixture writer out of production.

finish:
  deliver: >-
    New projection-batch tape objects land in per-computer segments when
    the flag is on, and the existing 9 million legacy tape files have
    byte-identical copies in sealed segments, verified digest by digest,
    so the owner can approve the legacy delete that makes the backup
    verify and the file system cope.
  artifact: >-
    (1) A2: the production segment writer (per-computer lock, torn-tail
    recovery, rotation, one fsync per event, collision check across both
    stores) behind PLATFORM_TAPE_SEGMENTS=off|write, default off; counters
    for files created per 1,000 writes and reader hits on the diagnostics
    endpoint. (2) A staging E2E report with the flag on for one test
    computer's host path (section 4.5). (3) The repack tool, dry run and
    copy only (section 6.1 steps 2 and 3), census mode with the legacy
    fallback off for the census (step 4). No delete in this mission.
  acceptance:
    - action: Deploy A2 with the flag off; run the tape and replay acceptance suite on staging.
      proves: Flag off is byte-identical to today (no segment files appear; legacy file counts grow as before).
      evidence_class: deployed proof
    - action: Turn the flag to write on staging (owner approval, morning of 2026-10-21); drive N guest writes on a test computer; read the counters.
      proves: Files per event in the tape namespaces drop from about 4 to below 0.01; replay after a planned and a crash restart reads segment-resident payloads; checkpointd publishes a base whose tail came from segments.
      evidence_class: deployed proof (E2E JSON report)
    - action: Repack dry run, then copy for test computers only, then census.
      proves: Every manifest digest resolves from a segment with equal bytes with the fallback disabled.
      evidence_class: deployed proof (operator read, counts only)
  rollback: >-
    The flag to off; the read path stays. No binary rollback below A1 once
    any segment holds data (tape-explode first). The repack writes copies
    only; dropping the segments restores today.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Minimize the files the tape adds per event and the files it already
    holds, while preserving every event, digest, receipt, head and
    sequence byte for byte, replay and checkpoint from any segment, and the
    owner's ruling before anything is deleted.
  goodharting_would_be: >-
    A file count that falls because writes failed or went elsewhere; a
    census run with the legacy fallback on; a repack counted as done on
    the owner's computer when only test computers were copied; deleting
    legacy files before the census, the off-node archive and the soak.

homotopy:
  realism_axis: >-
    Flag off on staging -> flag write for the platform host with test
    computers driven -> repack dry run -> copy for test computers -> census
    -> (owner) the paper and the owner's computer -> (owner) archive, soak,
    legacy delete, balance.

boundaries:
  mutation_class: red
  protected_surfaces: [computer events, event payloads, pin receipts, replay, checkpointd, ledger witness]
  authority_sources:
    - owner notes sections 32 and 33
    - docs/reports/tape-volume-forward-fix-and-surgery-2026-10-19.md
    - docs/choir-doctrine.md, docs/agent-product-doctrine.md, CLAUDE.md
  must_preserve:
    - The tape is immutable; no event, digest, receipt, head or sequence changes.
    - Segments are storage, not authority; the platform head and append receipt stay the authority.
    - No legacy file is deleted in this mission.
    - The owner's computer is counts, states and timings only; its tape is not repacked without the owner.
    - No tape bytes in Dolt.
  excluded:
    - legacy delete, btrfs balance, the off-node archive of segments (owner rulings)
    - group commit (A3)
    - retention of destroyed computers' tapes (section 6.3, owner ruling)
  ceremony: >-
    Red: conjecture delta, protected surfaces, admissible evidence,
    rollback and heresy delta in each commit or this file; failure modes
    written before code.

heresy_delta:
  discovered: >-
    writeBlob and writeImmutable never fsync (2026-10-19 record); the
    backup verify cannot walk a 4.58 million entry directory tree under
    its memory cap.
  introduced: none
  repaired: none

now:
  status: working
  slice: >-
    A2 writer built test-first on feat/tape-segments-a2 (tape-wt) and
    reviewed three times by gpt-6.1-sol. Not landed: each round found a new
    hole in the writer's recovery after a failed fsync. Escalated to the
    owner with the structural assessment below (CLAUDE.md, "Escalate on
    dead ends").
  source_ref: main (this card); branch feat/tape-segments-a2 at db96e07a1, base 8d027370b
  deploy_identity: staging cbef56155
  candidate: {id: none, state: none, ref: feat/tape-segments-a2, base: 8d027370b, digest: none, scope: [internal/tapestore, internal/platform]}
  conjecture:
    id: TS-C1
    claim: >-
      Routing projection-batch events, payloads and receipts into
      per-computer segments, behind a flag and with the A1 dual-read
      reader already in every read site, removes tape file growth without
      changing any digest or replay result.
    test: >-
      With the flag on for a staging window, files per event fall below
      0.01 and the completeness probe, both restart kinds and a checkpoint
      publish read correctly from segments.
    edge: missing_oracle
    delta_o: >-
      The counters and the E2E report; without them a falling file count
      cannot be told from failed writes.
  decision: >-
    Segment store over prefix sharding (owner asked to shard; the recorded
    durable design for these directories is the segment store, and
    sharding would be superseded). Flag default off; turning it on and the
    repack are owner steps. A2 is held, not landed, pending the owner's
    choice below.
  structural_assessment: >-
    Round 1 (five blockers: stale reader cache after rollback, concurrent
    pin acknowledging a removed artifact, ignored directory fsync errors,
    symlinks, retained open-scan state) was fixed. Round 2 found two P1s in
    the fsync-failure recovery (suspect bytes discarded before repair;
    unsynced knowledge lost on writer handoff) and two P2s; the fix replaced
    in-place repair with "abandon the unsynced tail, seal at the synced
    frontier, rewrite in a new segment". Round 3 (2026-10-21 05:10 UTC)
    reproduced a new P1: after two failed fsyncs, a writer keeps a pending
    retirement across lock releases; a second process appends and
    acknowledges a record; the first writer then truncates it away. It
    also showed offset reuse is still reachable between truncate and seal.
    All three rounds' holes live in one place: a writer that keeps going
    after an fsync error and later repairs the file. The flag-off path was
    found identical to legacy each time; the symlink race is accepted for
    flag-off under a trusted artifact root.
    Substrate-level option (not built): fail-stop on fsync error, the
    PostgreSQL rule since 2018. The first failed fsync poisons the
    appender; it acknowledges nothing more, never touches the segment
    again, and the call that failed returns an error so the caller falls
    back to the legacy file path for that pin. The next appender (a new
    process) treats an unsealed segment it did not write as read-only and
    seals it only after re-reading and fsyncing it, starting a new segment
    for its own writes. This deletes the retirement and suspect machinery
    rather than adding to it. Cost: an fsync error disables segment writes
    until restart, which is acceptable because the legacy path stays
    available.
  round_4: >-
    2026-10-21 05:50 UTC. Option (a), fail-stop, built test-first
    (4d3720c30 tests, 8864a1c5a fix on feat/tape-segments-a2-failstop;
    tapestore -race -count=2 and platform tests pass) and reviewed by
    gpt-6.1-sol: "land flag-off now". Round 3's two findings closed,
    re-checked with real second processes; flag-off path legacy-identical;
    reuse-lock fallback race-free. Two P1s must close before the flag is
    turned on: (1) a recovery refusal for a corrupt unsealed segment falls
    back to legacy and acknowledges a pin the segment-first reader then
    cannot read; (2) a sealed suspect tail can shadow its valid legacy copy.
    Both point to one fix: a digest-verified fallback to the legacy copy when
    a segment record fails verification, or treating recovery refusal as a
    hard error. P2: process-wide poison does not stop calls already inside
    build. Inherited: legacy writeBlob does not fsync (service.go).
    Decision under No Blocking Asks: land flag-off (reversible, dormant),
    after the SlidesApp drill; the flag stays off.
  round_5_design: >-
    2026-10-21 08:40 UTC, written before the code. Both enabled-mode P1s
    end in the same state: an acknowledged pin has a valid legacy copy,
    but the segment-first reader fails closed on a segment error and never
    reaches it. P1-1: a recovery refusal (corrupt unsealed segment) makes
    the pin fall back to legacy, and every later read for that computer
    probes the corrupt segment first. P1-2: a poisoned writer's
    unacknowledged tail, whole when the next writer reads it back, is
    sealed; if the kernel had dropped those pages, the sealed record later
    fails verification and shadows the legacy copy of the same pin.
    Change to design section 9.3.1 (fail closed): a segment error falls
    back to the legacy file only when that file's SHA-256 equals the
    requested digest. The bytes returned are then exactly the record
    meant, because the store is content-addressed and every returned byte
    is hashed. 9.3.1 refused the fallback because it "would answer from
    different storage than the record that was meant". With the digest
    check, the bytes cannot differ; the remaining risk is that corruption
    goes unnoticed. Each such read therefore counts in a new
    segment_error_fallbacks counter in health and logs once per segment,
    and the segment error is still returned when there is no legacy
    copy, or the legacy copy does not hash. ReadSegment, the census and
    the repack keep segment-only reads, so a corrupt segment can never be
    counted as a good copy or used as a repack source, and repack must
    not delete a legacy file whose segment copy fails verification.
    Failure modes: (1) a corrupt sealed record with an intact legacy copy
    serves legacy, counted; (2) the same without a legacy copy fails closed;
    (3) a legacy copy that does not hash fails with the segment error;
    (4) an unreadable segment directory or an index without its segment,
    with a legacy copy, serves legacy, counted; (5) the census and
    ReadSegment still report the segment error; (6) a healthy segment hit
    never reads legacy; (7) flag-off reads are unchanged.
  owner_choice: >-
    (a) build fail-stop and review it once more, then land flag-off;
    (b) land db96e07a1 flag-off now with round 3's P1 as a named residual
    (it needs two consecutive fsync failures and two writer processes;
    with the flag off nothing writes segments);
    (c) set A2 aside and use plain prefix sharding for the verify cost.
    Recommendation: (a).
  belief:
    believed_state: >-
      A1 reader live in production. A2 writer complete for the normal path,
      not landed. The weekly verify stays heavy until segments land and the
      legacy files are repacked.
    main_uncertainty: whether fail-stop closes the class in one more review round.
    next_observation: the owner's choice.
  blocker: none for flag-off landing; the two enabled-mode P1s (round_4) block turning the flag on
  next_action: Land land/tape-a2 (ten commits on main) with the flag off after the SlidesApp drill; deployed flag-off acceptance (no tape-seg files, legacy counts grow).

receipts: []
