---
definition_version: 4

# FP10 of the launch first pass (choir-launch-first-pass-metamission-2026-10-21.md),
# item RES-14 (P1, red) of docs/security-density-architecture-checklist-2026-10-21.md:
# verify the snapshot budget counts real bytes. FP0's assignment: the
# problem record (what the code counts against what the disk holds), a
# read-only probe FP0 runs on Node B, and a fix only if the count is wrong
# and the fix is small.
readiness: drafted

review:
  reviewer: none; docs and a read-only probe script (green). A fix, if the readings call for one, is red and goes to FP0's panel.
  frozen_ref: none
  verdict: none
  evidence_ref: none

start:
  captured_at: 2026-10-22T06:00:00Z
  source:
    canonical_ref: main 51cb3a8cd (lane base)
    deploy_identity: not read by this lane
  worktrees:
    - path: /home/agent/lanes/fp10-res14-snapshot-bytes
      status: clean at start
      class: goal_candidate
      owner: FP10 worker
      touch: goal_owned
      recovery: branch lane/fp10-res14-snapshot-bytes

finish:
  deliver: >-
    The snapshot budget's figure for each committed snapshot is the bytes
    the host disk gives back when that snapshot is deleted, read from Node B
    rather than assumed.
  artifact: >-
    The audit section of
    docs/problems/snapshot-budget-falls-back-to-logical-bytes-2026-10-14.md,
    scripts/snapshot_budget_bytes_probe.py, and FP0's reading recorded in
    that problem record.
  acceptance:
    - action: FP0 reads the vmctl journal for hibernate lines and measure fallbacks since 10e9f18e1 deployed (step 1 below)
      proves: which measure the budget uses on Node B
      evidence_class: deployed reading
    - action: FP0 runs the probe on Node B while at least one computer is hibernated by snapshot (steps 2 to 4)
      proves: recorded disk_bytes against tree search and exclusive bytes per snapshot; gaps G1 and G2
      evidence_class: deployed reading
  rollback: docs and a script only (git revert).
  landing:
    required: false (no runtime change)
    environment: staging
    required_receipts: [pushed_commit]

value:
  better_means: >-
    The budget admits as many snapshots as the disk really holds, so fewer
    computers lose their snapshot to eviction and cold-boot, while the
    free-space floor still holds.
  goodharting_would_be: >-
    Raising VM_SNAPSHOT_BUDGET_GIB on the logged figure alone, without the
    exclusive bytes and the free-space reading; counting compressed bytes
    that a shared extent keeps on disk after deletion.

homotopy:
  realism_axis: code reading, then Node B readings, then (if needed) a unit-tested fix and a staging hibernate on a test computer.

boundaries:
  mutation_class: green this pass (docs, read-only probe); any fix red (vmmanager snapshot path)
  authority_sources:
    - FP0's assignment of RES-14 to this lane (2026-10-22)
    - docs/problems/snapshot-budget-falls-back-to-logical-bytes-2026-10-14.md
    - docs/reports/vm-density-limits-2026-10-14.md
  must_preserve:
    - The metamission's must_preserve list, in full.
    - The budget never counts a snapshot below the bytes its deletion frees; the 16 GiB free-space floor stays.
    - No edit to internal/vmmanager while FP8 holds it.
  excluded:
    - RES-13 (sparse or Diff snapshots, idle timeout, snapshots across deploys) and RES-16 (data images that shrink).
  protected_surfaces: [vmctl]

now:
  status: blocked
  slice: >-
    Audit and probe written; no code changed. The count is safe except
    gaps G1 (resumed VMs' mapped memory files, covered by the free-space
    floor) and G2 (need may omit the 4 GiB hotplug region, unproven). Waits
    on FP0's Node B readings.
  source_ref: 51cb3a8cd
  deploy_identity: unchanged
  candidate: {id: none, state: none, ref: lane/fp10-res14-snapshot-bytes, base: 51cb3a8cd, digest: none, scope: [docs/problems/snapshot-budget-falls-back-to-logical-bytes-2026-10-14.md, docs/definitions/choir-launch-fp10-res-14-2026-10-22.md, scripts/snapshot_budget_bytes_probe.py]}
  conjecture:
    id: C-FP10-res14-exact
    claim: >-
      Since 10e9f18e1 vmctl holds CAP_SYS_ADMIN, every hibernate records
      disk_bytes by btrfs_extents, and that figure equals the snapshot's
      exclusive bytes.
    test: >-
      Journal hibernate lines read disk_bytes_method=btrfs_extents; the probe
      shows recorded disk_bytes within 1% of tree_search and shared extents
      zero for every committed snapshot.
    edge: missing_oracle (no Node B reading since the unit changed)
    delta_o: steps 1 and 3 below
    scope_if_supported: every snapshot hibernate on Node B
    status: active
    evidence_refs: [docs/problems/snapshot-budget-falls-back-to-logical-bytes-2026-10-14.md]
  decision:
    what: no code fix this pass; readings first; G2's one-line fix and the stale comments go to the vmmanager holder if the readings call for them
    kind: operational
    status: settled
    evidence_ref: the audit section of the problem record
    owner_ratification_ref: not_applicable
  belief:
    believed_state: >-
      The budget counts referenced compressed bytes (exact) if measure 1
      runs, else an upper bound; a new snapshot reserves its logical size.
      With about 0.5 GiB per snapshot the budget holds about 80, so the
      free-space floor and the 36 h age are the real bounds.
    main_uncertainty: which measure Node B runs now; whether a hotplug computer's memory file is larger than mem_size_mib.
    next_observation: FP0's steps 1 to 4.
  blocker_or_risk: Node B readings are FP0's; vmmanager is FP8's.
  next_action: FP0 runs steps 1 to 4 and records the readings in the problem record; then decide G2 and the 48 GiB and 36 h settings.

receipts: []
---

# FP10: RES-14, the snapshot budget's real bytes

The audit (what each measure counts, file:line, gaps G1 to G4) is the
2026-10-22 section of
[snapshot-budget-falls-back-to-logical-bytes-2026-10-14.md](../problems/snapshot-budget-falls-back-to-logical-bytes-2026-10-14.md).

## FP0's steps on Node B (read-only, no secrets)

1. Which measure runs. Since the deploy that carried `10e9f18e1`:

   ```sh
   journalctl -u go-choir-vmctl --since '<that deploy>' --no-pager \
     | grep -E 'hibernated VM .* by snapshot|snapshot disk measure' | tail -n 40
   ```

   Expect `disk_bytes_method=btrfs_extents` and no
   `snapshot disk measure btrfs_extents unavailable` line. A
   `fiemap_bound` or `st_blocks` method, or an unavailable line, is the
   finding: record its errno.

2. Make sure at least one computer is hibernated by snapshot (a test
   computer left idle past its hibernate timeout, or one already
   hibernated: `ls -d /var/lib/go-choir/vm-state/*/snapshot`). For G2, one
   of them should be a test computer with memory hotplug.

3. Run the probe as root (root has `CAP_SYS_ADMIN`, so the tree search
   column is exact). The script is read from stdin, nothing is copied to
   the host:

   ```sh
   ssh node-b 'python3 -I - /var/lib/go-choir/vm-state' \
     < scripts/snapshot_budget_bytes_probe.py
   ```

   Add `--json` after the state dir for one JSON object instead of the
   table.

4. Record in the problem record: per snapshot, `recorded` against
   `tree_search` and `exclusive`; the `shared_ext` column; the totals line
   (budget's sum against the disk's sum); the `mapped deleted mem` lines
   (G1); and for a hotplug computer, `apparent` against `mem_size_mib`
   (G2: `apparent_vs_mem` above 1.01 means the memory file carries the
   hotplug region and `need` under-reserves).

Reading the result:

- recorded ≈ tree_search ≈ exclusive, shared 0: the count is real bytes;
  close RES-14 and revisit 48 GiB and 36 h with the free-space line.
- recorded > tree_search: an older or fallback measure; safe; check step 1.
- exclusive < tree_search (shared extents): deletion frees less than the
  budget subtracts; open a problem record before any budget change.
- G2 confirmed: a red one-line fix in vmmanager (`need` adds
  `cfg.MemoryHotplugMiB<<20`), test first.
