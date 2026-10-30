---
definition_version: 4

# FP10 of the launch first pass (choir-launch-first-pass-metamission-2026-10-21.md),
# items RES-13 (snapshot levers L3, L4, L20) and RES-22 (host slab and
# btrfs metadata, L19) of docs/security-density-architecture-checklist-2026-10-21.md.
# Both are P2 and red. FP0's assignment: both need host or guest readings
# this lane cannot take (no Node B), and vmctl and vmmanager are FP8's
# (lane/fp8-image-jailer), so this station writes the design and FP0's
# read-only measurement commands, and stops.
readiness: drafted

review:
  reviewer: none; docs only (green). Each build slice is red and goes to FP0's panel on its frozen diff.
  frozen_ref: none
  verdict: none
  evidence_ref: none

start:
  captured_at: 2026-10-22T00:30:00Z
  source:
    canonical_ref: main d1c60ad87 (lane base)
    deploy_identity: not read by this lane (FP0's ledger names 53a691be6 serving, FP3 group 1 deploying)
  worktrees:
    - path: /home/agent/lanes/fp10-res13-res22-density
      status: clean at start
      class: goal_candidate
      owner: FP10 worker
      touch: goal_owned
      recovery: branch lane/fp10-res13-res22-density

finish:
  deliver: >-
    A hibernate writes only the guest's resident pages; an idle computer
    hibernates after minutes, not half an hour; a guest-artifact deploy no
    longer turns every wake into a cold boot; host slab and btrfs metadata
    stop growing with the tape's file count.
  artifact: >-
    This pass: two problem records, the design
    (docs/design/fp10-snapshot-levers-and-host-slab-2026-10-22.md) and FP0's
    read-only measurement commands. Later passes: one red commit set per
    slice in the design's order, each off by default, with a failing test
    first and FP0 proof steps.
  acceptance:
    - action: FP0 runs the design's section 4 readings on Node B and records them in the two problem records
      proves: the numbers each slice's gate needs (resident fraction, swap guarantee, fallback reasons, slab and file growth)
      evidence_class: deployed reading
    - action: per build slice, its focused unit tests, then FP0's staging proof on a test computer
      proves: the slice's claim on the deployed product
      evidence_class: deployed proof
  rollback: >-
    This pass is docs only (git revert). Every build slice is a flag whose
    off value is today's behaviour.
  landing:
    required: false (docs only this pass)
    environment: staging
    required_receipts: [pushed_commit]

value:
  better_means: >-
    Fewer bytes written per hibernate, less RAM held by idle computers and
    by host slab, fewer cold boots after a deploy, while preserving guest
    memory integrity across hibernate and wake, the snapshot validity
    rules, and the restart rules.
  goodharting_would_be: >-
    A Diff snapshot that loses pages a restored guest needed; a shorter
    idle timeout that hibernates working computers; resuming a snapshot
    onto a guest release a security deploy meant to retire; raising
    vfs_cache_pressure so the paper's Dolt pages are evicted instead.

homotopy:
  realism_axis: >-
    Readings, then unit tests with a fake Firecracker API, then a test
    computer on staging with guest memory checksums, then every class.

boundaries:
  mutation_class: green this pass; each build slice red (vmctl/vmmanager, deployment routing for L20)
  authority_sources:
    - owner 2026-10-21 (the metamission quotes)
    - docs/reports/vm-density-limits-2026-10-14.md (levers L3, L4, L19, L20; order section 4)
    - docs/archive/reports/s3-snapshot-hibernate-design-2026-10-14.md (validity rules, failure modes 7, 8, 17)
  must_preserve:
    - The metamission's must_preserve list, in full.
    - A snapshot loads only if every validity rule in validateSnapshot holds; no slice weakens a rule, only adds the facts it compares.
    - Guest memory never swaps while a Diff snapshot may be taken from it.
    - No edit to internal/vmctl, internal/vmmanager or the VM image while FP8 holds them.
  excluded:
    - RES-14 (the budget's byte count) and every FP8 item.
    - The tape repack (choir-tape-segments-a2-repack-2026-10-21.md owns it).
  protected_surfaces: [vmctl, deployment routing]

now:
  status: blocked
  slice: >-
    L3a built on lane/fp10-res13-l3a (base 7448bf899), tests first
    (b0072c20e, ff96a7bf2), the inert red fix c657c0220 (default full, no
    computer selected), this green commit (record, proof steps, nix lines
    VM_SNAPSHOT_TYPE=full and VM_SNAPSHOT_DIFF_ONLY empty). Only FP0 steps
    remain: panel on the frozen diff, land, then the proof below on a test
    computer FP0 chooses. Owner notes §40: L4 is 10 minutes only after L3
    is on everywhere; L20 ships off.
  gates:
    L3a: >-
      Readings pass, with two design corrections. R2: guest memory cannot
      swap today (every Firecracker VmSwap 0; their leaf cgroup
      go-choir-vmctl.service/supervisor has no memory controller, so the
      service's memory.swap.max 0 bounds them), but the design's check reads
      the leaf's memory.swap.max, which does not exist; it must walk up to
      the nearest ancestor that sets 0. Repeat R2 under FP8's jailer. R4:
      Firecracker 1.16.1 (above the 1.15.0 floor), snapshot-editor
      installed, KSM off; host THP is madvise, not never, but every guest
      mapping reads THPeligible 0, so the condition should test the guest
      mapping. R5: a cold guest holds 14 to 17% of its 8 GiB (1.15 to
      1.37 GiB, all anonymous), so a Diff writes about 1.4 GiB where Full
      writes 8 GiB. Remaining gate: FP8 lands, the two corrections, then
      the build and its proof.
    L3b: >-
      snapshot-editor is installed. New fact: a healthy resume already
      unlinks the snapshot (the restored mapping reads "(deleted)"), so L3b
      must link the memory file to snapshot.base/mem in place of that
      discard; the design's paragraph says the directory survives until the
      next hibernate, which is wrong on main. A restored guest's touched
      memory is mostly clean file pages (201,872 of 237,240 kB), so the
      ColdOrigin guard is needed. Gate unchanged: L3a's proof plus a week.
    L4: >-
      Needs L3, plus a new prerequisite: API-key traffic neither wakes a
      hibernated computer (502 "api key computer realization unavailable",
      by design in internal/proxy/api_key_computer_authority.go) nor, by
      the trace, moves the idle clock (hibernated 32 min after the boot,
      25.5 min after the last key request). At 10 minutes, key-driven
      computers would hibernate between calls and answer 502 until a
      lifecycle start. PSI: a quiet-host Full hibernate stalled 22 to 36
      ms/s for 6 s (avg10 peak 1.03); on the pressured host the two
      reclaim hibernates added no visible step. R6 has only two wakes,
      too few to set a timeout.
    L20: >-
      R1's timing: in 58 h (the journal's span, not 7 days) 8 hibernates,
      0 resumes until FP0's test, 2 wakes both launch_config_mismatch, each
      found at the wake 11 to 13 minutes after the last of 6 and 12
      deploys. 76 deploy targets in 58 h; every commit is a new guest image
      (kernel-params init= and build_commit change), so every deploy voids
      every snapshot. FP0's own snapshot was voided 31 s after it was taken
      (deploy 8fabec36a), and the only resume came from waking within 3 s.
      At this cadence L3 saves the write but the snapshot is almost never
      loaded; L20 is what lets snapshots pay off. R7: a release is 3.9 GiB
      (storedisk.erofs 3.5 GiB), /var/lib/go-choir/guest is a symlink to
      its Nix store path, and the store already holds 11 releases with
      little sharing, so a GC root on the store path may replace the
      reflinked releases directory. Owner question Q2 still open.
    RES-22: >-
      S1: slab 1.21 GiB, btrfs_inode 294 MiB (9.75 and 5.9 GiB on 10-14),
      not near the guests' working set; no vfs_cache_pressure change is
      called for. S3: 47.0 GiB unallocated, growing about 3 GiB a day;
      metadata 82.3% used, 2.7 points under the alert slice's 85% line, so
      the alert slice is worth landing. S4: the tape's flat directories
      grow far slower than on 10-16 (numbers in the slab record). S2 is the
      remaining reading, read against S5's walker windows.
  source_ref: 9cbed1aa2
  deploy_identity: not changed by this lane (read only; staging served 8fabec36a during the readings)
  candidate: {id: FP10-L3a, state: built, not frozen, ref: lane/fp10-res13-l3a, base: 7448bf899, digest: none, scope: [internal/vmmanager, nix/node-b.nix, docs]}
  conjecture:
    id: C-FP10-sparse
    claim: >-
      A Diff snapshot without dirty tracking of a cold-booted guest whose
      memory cannot swap holds every page the guest can read, so it
      resumes identically while writing only resident pages.
    test: >-
      On a test computer: write a known pattern into guest memory, hibernate
      by Diff, wake, check the pattern and the guest's health; compare
      st_blocks, write time and PSI with a Full hibernate of the same guest.
    edge: >-
      missing_oracle, narrowed: R2 and R5 are read (no swap today, 14 to
      17% resident). Still unread: the jailer cgroup's swap bound, and
      whether Firecracker's mincore path writes a complete image of a
      virtio-mem guest.
    delta_o: R2 under FP8's jailer; L3a's proof step
    scope_if_supported: cold-booted computers on Node B, class:test first
    status: active
    evidence_refs: [docs/problems/hibernate-writes-whole-guest-and-deploys-void-snapshots-2026-10-22.md]
  decision:
    what: design only; slice order L3, then L4, then L20; RES-22 is readings, then an alert, then (only if readings show it) vfs_cache_pressure
    kind: operational
    status: settled
    evidence_ref: FP0's assignment to this lane (2026-10-21)
    owner_ratification_ref: not_applicable
  belief:
    believed_state: >-
      L3a is ready to build once FP8 lands, with the swap check walking up
      the cgroup tree and the THP condition testing the guest mapping. L4
      needs API-key activity to count before its timeout shrinks. L20
      matters more than its place in the order suggests: at tonight's
      deploy cadence no snapshot lives to be loaded without it. RES-22
      needs no slab knob today; the alert is the useful slice.
    main_uncertainty: whether S2 shows btrfs_inode rising at the backup or GC windows; whether the jailer keeps the swap bound.
    next_observation: S2's log at 2026-10-23 06:45Z; R2 under FP8's jailer.
  blocker_or_risk: >-
    FP8 holds vmmanager. Residual for FP0: the slice order (L20 before
    L4) is an open choice the readings raise; it is not changed here.
  next_action: >-
    FP0 freezes lane/fp10-res13-l3a, sends c657c0220 to the bunch panel,
    lands it (it shares internal/vmmanager with deploy M: the files are
    listed in "L3a: files and proof"), then runs the proof with one test
    computer named in VM_SNAPSHOT_DIFF_ONLY.
receipts:
  - id: fp0-res13-res22-readings-2026-10-22
    ref: docs/problems/hibernate-writes-whole-guest-and-deploys-void-snapshots-2026-10-22.md, docs/problems/node-b-host-slab-btrfs-inode-cache-2026-10-22.md
    result: partial (every section 4 reading taken but S2, which runs to 2026-10-23 06:45Z; R5 cold and restored on FP0's own test computer; one snapshot resume)
---

# FP10: snapshot levers and host slab

The design, the owner questions and FP0's read-only commands are in
[docs/design/fp10-snapshot-levers-and-host-slab-2026-10-22.md](../design/fp10-snapshot-levers-and-host-slab-2026-10-22.md).
The problems are
[hibernate-writes-whole-guest-and-deploys-void-snapshots-2026-10-22.md](../problems/hibernate-writes-whole-guest-and-deploys-void-snapshots-2026-10-22.md)
and
[node-b-host-slab-btrfs-inode-cache-2026-10-22.md](../problems/node-b-host-slab-btrfs-inode-cache-2026-10-22.md);
the btrfs metadata record is
[node-b-btrfs-metadata-exhausted-small-artifact-files-2026-10-16.md](../problems/node-b-btrfs-metadata-exhausted-small-artifact-files-2026-10-16.md).

## L3a: files and proof (FP0)

Files touched by the lane: `internal/vmmanager/snapshot_diff.go` (new),
`internal/vmmanager/snapshot_diff_test.go` (new), small hunks in
`internal/vmmanager/{config.go,fcapi.go,manager.go,snapshot.go,snapshot_lifecycle.go,snapshot_test.go}`
(the last: the fake Firecracker API's Diff mode), `nix/node-b.nix` (two
lines after `VM_SNAPSHOT_MAX_AGE`), and docs. Not touched:
`snapshot_vault_bind_test.go`, the host-network start, `cmd/vmctl`,
`internal/vmctl`.

The proof, on one test computer T that FP0 chooses (never the paper
`computer-1355b83b…`, the owner's `computer-ad1b46d4…` or the delegate):

1. If T runs under the jailer, repeat reading R2 for T's Firecracker pid
   first; every ancestor walk must reach a `memory.swap.max` of 0.
2. Deploy with `VM_SNAPSHOT_TYPE=diff` and
   `VM_SNAPSHOT_DIFF_ONLY=<T's computer id>`. A vmctl restart reattaches
   running guests with cold origin false, so T must cold boot after this
   deploy: stop it, then `POST /api/computers/{T}/lifecycle/start`.
   No deploy between this boot and step 4.
3. In T (its terminal or engineering capsule; no block device): write
   1 GiB of random bytes to `/dev/shm/l3a.bin`, record its `sha256sum`
   and `/proc/meminfo`.
4. Hibernate T (`POST /internal/vmctl/hibernate` with its computer id, or
   the idle sweep). Sample `/proc/pressure/memory` each second across it.
   Expect the journal line `hibernated VM … by snapshot (… snapshot_type=Diff diff_refused=)`;
   `stat -c '%s %b %B' …/snapshot/mem` with size `12884901888` (8 + 4 GiB
   on a hotplugged test computer) and `blocks × 512` near 2.5 GiB (about
   1.4 GiB resident plus the 1 GiB file), against Full's 8 GiB; `snapshot=`
   ms against R6's Full baseline (about 6 s, PSI some avg10 1.03).
   `meta.json` reads `"snapshot_type":"Diff","cold_origin":true`.
5. Within minutes, before any deploy, wake T with
   `POST /api/computers/{T}/lifecycle/start` (an API-key read cannot wake
   it). Expect `resumed VM … from snapshot`, not `cold boot … after
   snapshot fallback`. In T: `sha256sum /dev/shm/l3a.bin` matches step 3,
   `/health` is ok, and one prompt-bar call completes.
6. Hibernate T again: the journal must say `snapshot_type=Full
   diff_refused=not_cold_origin` (failure mode 1 on staging).
7. Rollback either way: `VM_SNAPSHOT_DIFF_ONLY=` (empty) and deploy.

Pass: steps 4 to 6 as stated. A `diff_refused` other than empty at step
4 is not a failure of the guard but a refusal to read: record it (it
names the condition) and stop.
