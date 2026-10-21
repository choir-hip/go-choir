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
    A2 writer test-first in tape-wt (subagent), then review, land with the
    flag off, deploy, deployed acceptance with the flag off. The flag flip
    and the repack wait for the owner's morning review.
  source_ref: main 8d027370b
  deploy_identity: staging 7bb7b3e65
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
    repack are owner steps in the morning.
  belief:
    believed_state: A1 reader live in production; A2 writer being built.
    main_uncertainty: >-
      Lookup cost over many sealed segments (9.6 item 6) and the dead-weight
      bound for pins whose CAS never lands.
    next_observation: the subagent's tests and diff.
  blocker: none
  next_action: Review the A2 diff; land with the flag off.

receipts: []
