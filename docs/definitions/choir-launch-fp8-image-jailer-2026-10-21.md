---
definition_version: 4

# FP8 of the launch first pass (choir-launch-first-pass-metamission-2026-10-21.md):
# batch B6, the VM image, the jailer and density. Items from
# docs/security-density-architecture-checklist-2026-10-21.md in this order:
# RES-05 then SEC-04, NET-08 then RES-16, RES-06, RES-09, RES-14; launch
# checklist L4-03. Every behavior change ships off and is selected per
# computer, so the deploy that carries it changes no running computer until
# FP0 names a test computer (or class:test) in vmctl's environment.
readiness: drafted

review:
  reviewer: none yet; FP0 sends the frozen candidate to a panel (red)
  frozen_ref: none
  verdict: none
  evidence_ref: none

start:
  captured_at: 2026-10-21T23:30:00Z
  source:
    canonical_ref: >-
      lane base 122eb9623 (the FP3 group 1 round-3 candidate, still in
      review; this lane does not change its commits)
    deploy_identity: unchanged by this lane (nothing here is deployed)
  worktrees:
    - path: lanes/fp8-image-jailer
      status: clean at start
      class: goal_candidate
      owner: FP8 worker
      touch: goal_owned
      recovery: branch lane/fp8-image-jailer

finish:
  deliver: >-
    Jailed computers share one host page-cache copy of the store image;
    the jailer, Firecracker 1.17, data-image discard and guest page-cache
    reclaim can each be turned on for one test computer (or class:test)
    without changing any other computer; the hotplug refusal and the
    snapshot budget method are diagnosed with exact staging steps; and
    sign-ups have a first cap.
  artifact: >-
    Commits on lane/fp8-image-jailer: this file with the problem records,
    then per item a failing test, the code, and the FP0 proof steps in
    "FP0 proof steps" below. Nix changes are written and unevaluated.
  acceptance:
    - action: go test ./internal/vmmanager (focused -run per item) and the touched vmctl tests
      proves: argument building, jail layout, per-computer selection and gates
      evidence_class: unit
    - action: FP0 follows "FP0 proof steps" per item on a minted test computer
      proves: cold boot, snapshot wake, vmctl-restart reattach and burst reclaim work with each feature on, and admission refuses safely when full
      evidence_class: deployed proof
  rollback: >-
    Every feature is off unless vmctl's environment names it; removing the
    environment line returns new launches to today's behavior. Code: git
    revert of the lane commits.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    More computers per Node B GiB and a confined VMM, without a single
    deploy that changes every computer at once.
  goodharting_would_be: >-
    A jailer that is "on" but copies the store per VM; a Firecracker bump
    that every computer takes at once; reclaim that evicts the hot store
    and turns RAM savings into disk reads; a cap that refuses the owner.

homotopy:
  realism_axis: >-
    unit tests with fakes, then a frozen reviewed candidate, then one test
    computer on staging per feature, then class:test, then everyone.

boundaries:
  mutation_class: red (vmctl, VM launch config, guest boot args, sign-up admission)
  authority_sources:
    - owner 2026-10-21 (metamission quotes); owner notes §27 item 3 (jailer rollout authorized)
    - docs/security-density-architecture-checklist-2026-10-21.md RES-05, SEC-04, NET-08, RES-16, RES-06, RES-09, RES-14
    - docs/launch-checklist-2026-10-21.md L4-03
  must_preserve:
    - A deploy of this lane changes no running computer; each feature is selected per computer.
    - Snapshot validity: a snapshot taken under another binary, confinement or boot args is refused and cold boots, never loaded.
    - The store image the guest maps is the one its snapshot was taken against (snapshotBoundPaths).
    - No boot-time resumer; no boot-path scan of og_objects.
    - Lane rules (no push of main, no Node B, no secrets, no nix builds).
  excluded:
    - Running anything on staging (FP0); the per-VM network namespace (jailer slice 2); RES-08 pmem+DAX.
  protected_surfaces: [vmctl, VM launch config, snapshot validity, guest boot args, sign-up admission]

now:
  status: working
  slice: >-
    built, not frozen: every item has its code, tests and FP0 proof steps;
    only FP0 steps (freeze, panel, land, deployed proof) and the owner's cap
    number remain
  source_ref: 122eb9623 (lane base; rebase onto main after FP3 group 1 lands)
  deploy_identity: unchanged (nothing of FP8 is deployed)
  candidate:
    id: FP8-B6
    state: built, unreviewed
    ref: lane/fp8-image-jailer
    base: 122eb9623
    digest: none
    scope: [internal/vmmanager, internal/vmctl/ownership.go, internal/auth, nix/node-b.nix, nix/firecracker-next.nix]
  conjecture:
    id: C-FP8-per-computer
    claim: >-
      Selecting each density and confinement feature per computer at launch
      (vm id, computer id or class:test) lets staging prove it on one test
      computer while every other computer launches exactly as before.
    test: >-
      Unit: with the selector empty the launch config, arguments and jail
      are byte-identical to today's; with it naming one computer only that
      computer's launch changes. Deployed: FP0 turns each on for one test
      computer and runs its proof steps.
    edge: missing_oracle (no staging run in this lane)
    delta_o: FP0's deployed proof per item
    scope_if_supported: Node B, test computers first
    status: active
---

# FP8: VM image, jailer and density (B6)

## Problem records

Each record names the evidence, from code at the lane base unless it says
otherwise. Nothing was run on staging by this lane.

### P1 (RES-05). The jailer copies the store image per VM

Recorded in
[jailer-copies-store-image-per-vm-2026-10-21.md](../problems/jailer-copies-store-image-per-vm-2026-10-21.md).
`prepareJail` places every read-only file (kernel, initrd, the about 2.9 GB
store image) with `placeReadOnlyCopy`: a reflink (a new inode) or a full
copy (`internal/vmmanager/confinement.go:600-664`). The page cache is per
inode, so N jailed VMs can hold N host copies of the hot store pages; today
direct VMMs share one. Not fixable with a hard link to the source: the image
is in `/nix/store`, another mount than the jails (`EXDEV`). Blocks SEC-04.

Added here: failure mode 5 of that record (snapshot wake maps another store
inode) is already closed for the *source* by `snapshotBoundPaths`, which
binds the host store image's inode and mtime into the snapshot
(`snapshot.go:423-432`, `validateSnapshot` "disk" reason). A fix must keep
the jail file derived from that same source identity.

### P2 (SEC-04). The jailer rollout needs a deploy per test computer

`VMCTL_JAILER_ONLY` names VM ids or computer ids only (`jailerSelects`,
`confinement.go:238-253`). A disposable test computer's id exists only after
it is minted, so step 2 of the rollout (one test computer) is a Nix change
and a deploy naming that id, and each further test computer another. Memory
hotplug and visual capture already select `class:test`
(`nix/node-b.nix` `VM_MEMORY_HOTPLUG_ONLY=class:test`,
`visual_capture.go`); the jailer cannot.

### P3 (NET-08, RES-16). One Firecracker binary for every computer

`VM_FIRECRACKER_BIN` (and `VM_JAILER_BIN`) is one path for the whole host
(`config.go:22`, `nix/node-b.nix:1282, 1289`). Moving to 1.17 (needed for
data-drive discard,
[guest-data-image-never-trims-2026-10-20.md](../problems/guest-data-image-never-trims-2026-10-20.md))
therefore changes every computer's VMM in one deploy and refuses every
hibernate snapshot at once (`firecracker_version_mismatch`), with no canary.
`VM_DATA_DISCARD` is likewise host-wide. Found while checking NET-08: the
`--enable-pci` half is already done; every launch and restore passes it in
both modes (`snapshot.go:319-329`, `confinement.go:300-311`, since
f240251b6). nixpkgs at the lock still packages 1.16.1.
Hotplug blocker found 2026-10-22: Firecracker 1.17 places the virtio-mem
hotplug region at 512 GiB, and Node B's Xeon E-2236 has 39-bit physical
addresses (2^39 = 512 GiB), so a 1.17 computer cannot hotplug on this host
(firecracker issue #6255, open). The host moves to 1.16.2 instead
([firecracker-1-16-1-virtio-mem-restore-bugs-2026-10-22.md](../problems/firecracker-1-16-1-virtio-mem-restore-bugs-2026-10-22.md));
NET-08 stays off hotplug computers until #6255 is fixed.

### P4 (RES-06). Guest page cache is never reclaimed while the guest is busy

Free page reporting is on, but it returns only pages the guest frees; a busy
guest keeps cold page cache until memory pressure. The paper computer held
7,970 MiB host RSS against about 2.8 GB used in the guest (checklist RES-06,
read 2026-10-21 18:50 UTC). Only capsule teardown reclaims
(5d228ecfd, 1f71940f8). The guest kernel has `CONFIG_DAMON_RECLAIM=y`
(`reports/vm-density-limits-2026-10-14.md:346`), unused: no boot argument and
no unit enables it. The after-burst measurement of free page reporting
(`evidence/guest-memory-split-balloon-2026-10-14.md:38-46`) is still owed and
comes first in FP0's steps.

### P5 (RES-09). The hotplug probe refusal is undiagnosed

The one recorded refusal, `refused ("computer is not selected for memory
hotplug")` on build 7b8944f03 (`reports/overnight-landing-2026-10-15.md:136`),
is the probe's own message (`scripts/memory_hotplug_probe.py:339-340`), raised
when vmctl's list omits or falses `memory_hotplug_selected` for the computer.
That build carried `VM_MEMORY_HOTPLUG=on` and `_ONLY=class:test`
(c9621a721), so the report's "flag-off by design" is wrong.

Code trace (hypothesis until FP0's read below confirms it): `class:test`
means the owner is in vmctl's test tier, which `refreshTestAccounts` fills.
A freshly minted test account is not in it until a refresh. Both boot paths
read the hotplug selection *before* the call that refreshes the tier:

- `reserveFreshVMConfig`: `cfg.MemoryHotplugMiB = memoryHotplugMiBWith(...)`
  (`internal/vmctl/ownership.go:1413`), then
  `cfg.WarmnessClass = bootWarmnessClass(own)` (`:1417`, which refreshes);
- the new-computer `BootVM` literal: `MemoryHotplugMiB:
  r.memoryHotplugMiBFor(own)` (`:2267`) is evaluated before `WarmnessClass:
  bootWarmnessClass(own)` (`:2273`).

So a minted test computer's first cold boot has no virtio-mem device even
though it boots as class test; afterwards vmctl selects it, the controller
scan probes it once a minute and Firecracker answers `400 Device not found`
(the tracer's log, `problems/go-served-1143-first-full-selfdev-loop-test-job-not-done-2026-10-15.md:62-66`). Any feature that
selects `class:test` at boot inherits this ordering, including the selectors
this lane adds. The list omits the field for a non-active VM
(`memory_hotplug_routes.go:198-204`), and a 403 on the list reads as "not
selected" in the probe: two other possible causes FP0's read separates.

### P6 (RES-14). Nobody has read which method the snapshot budget uses

Recorded in
[snapshot-budget-falls-back-to-logical-bytes-2026-10-14.md](../problems/snapshot-budget-falls-back-to-logical-bytes-2026-10-14.md).
The FIEMAP bound landed (9001e9842); vmctl gained `CAP_SYS_ADMIN` with the
jailer unit change (10e9f18e1), so the exact `btrfs_extents` method may now
succeed. No staging hibernate line has been read since. A reading only;
FP0's step.

### P7 (L4-03). No cap on sign-ups

Nothing limits registered users. `HandleRegisterBegin` creates a user for
any new email (`internal/auth/handlers.go:487-550`, `CreateUser` at
`store.go:441`); there is no invite, allowlist, waitlist or count. The
computer is minted lazily at first use through vmctl's resolve, where host
memory admission (`admitHostMemoryLocked`, `internal/vmctl/ownership.go`
new-computer branch; `host_admission.go:362-443`; active on Node B with
`VMCTL_PRESSURE_RECLAIM_MODE=active`, `VMCTL_ADMISSION_MODE=measured`)
refuses a start when the host is full. So the only limit is on computers
*running*, and a user past it has already signed up and then sees "Your
computer cannot start right now" with a retry
(`internal/proxy/computer_surface_page.go:15-48`) for as long as the host
stays full. Registered users cost disk (about 560 MB per hibernated
computer, `reports/vm-density-limits-2026-10-14.md:62-76`); the checklist
puts today's headroom at about 100 more users.

## As built (lane/fp8-image-jailer)

| Item | Commit | What | Off until |
|---|---|---|---|
| records | 4f12b86d6 | this file, P1 to P7 | n/a |
| RES-05 | e01d5423e | jails hard-link one root-owned 0444 copy per source identity (`jail_shared.go`) | the jailer is on |
| SEC-04 | 8e373bdd0 | `selectsComputer` (ids, `class:<class>`, `*`); `VMCTL_JAILER_ONLY=class:test` works | `VMCTL_JAILER=on` |
| NET-08, RES-16 | b4293aaec, 859cfe6cc | `VM_FIRECRACKER_NEXT_BIN`/`_ONLY`, per-VM `vmm-build.json`; discard per launched build, `VM_DATA_DISCARD_ONLY`; `nix/firecracker-next.nix` (UNEVALUATED, fakeHash x2, imported nowhere) | NEXT env set |
| RES-06 | f6d31783d | `damon_reclaim.*` boot args for `VM_GUEST_RECLAIM_ONLY` computers | ONLY env set |
| RES-09 | db7587bcc | vmctl reads the boot class (refreshing the test tier) before the hotplug selection | live on deploy (class:test only) |
| RES-14 | c46df4522 | stale comment only; the item is a reading | n/a |
| L4-03 | 85f8ce067 | `AUTH_REGISTRATION_CAP` + `registration_waitlist` table | cap set |

Tests (local, Linux, this lane): `go test ./internal/vmmanager` passes except
`TestLaunchThreadPolicyReachesOnlyItsChild`, which needs `/bin/cat` (absent
in this sandbox; untouched code); `go test ./internal/vmctl`,
`./internal/auth`, `./cmd/auth` pass; `go build ./cmd/vmctl` passes.

Only RES-09 changes live behavior on deploy, and only for test accounts'
computers (a freshly minted test computer's first boot gets the hotplug
region it was already meant to get). Everything else is inert until an
environment line names a computer.

## FP0 proof steps

Mint a disposable test computer with `node scripts/qa_test_computer.mjs
--label fp8-<item>`; read its VM id from `GET /internal/vmctl/list` on Node
B (the same curl `scripts/memory_hotplug_probe.py:53,111` uses). "Turn on"
below is a reviewed `nix/node-b.nix` change and a deploy; each is one red
activation under the metamission's lease. Record outputs under
`docs/evidence/fp8-<item>-<date>.*`.

### Landing (all items)

1. Rebase onto main after FP3 group 1 lands (`internal/vmctl/ownership.go`
   is touched by both; the FP8 hunks are in `reserveFreshVMConfig` and the
   new-computer `BootVM` literal). Freeze, panel, land as one red batch.
2. Deployed identity, then confirm no computer changed: a non-test
   computer's `vm-state/<vm>/fc-config.json` sha256 is unchanged across the
   deploy at its next launch (the boot timeline's launch hash), and vmctl's
   log has no `next Firecracker build` line.

### RES-09 (live on deploy)

1. `scripts/memory_hotplug_probe.py --out docs/evidence/fp8-res09-<date>.json`
   (mints its own test computer). Pass: no "not selected" refusal; the
   computer's `fc-config.json` has `memory-hotplug` and its boot args end in
   `memhp_default_state=online_movable`; the grow, job and shrink checks run.
2. If it still refuses, read the list entry for that computer (`state`,
   `warmness_class`, `memory_hotplug_selected`): 403 means the probe's
   token header, non-active means timing, class primary means the auth DB
   read; record which, as the P5 hypothesis's verdict.

### RES-05 and SEC-04 (jailer, one test computer, then class:test)

1. Turn on: `"VMCTL_JAILER=on"` and `"VMCTL_JAILER_ONLY=<test computer id>"`.
   vmctl log: `jailer ready`, not `JAILER UNAVAILABLE`.
2. Stop/start the test computer. `scripts/vmm_confinement_verify.py --ssh
   node-b --vm <vm> --expect jailer --out docs/evidence/vmm-confinement-<date>.json`
   passes; its boot timeline's `confinement.jail_files` shows
   `store.img=shared(reflink)` (first launch) or `store.img=shared`.
3. Mint a second test computer, add its id to `VMCTL_JAILER_ONLY`, start it.
   On Node B: `stat -c '%d:%i %U %a %h' /var/lib/go-choir/vm-state/.jail/firecracker/<vm1>/root/store.img
   /var/lib/go-choir/vm-state/.jail/firecracker/<vm2>/root/store.img`: the
   same `dev:inode`, owner root, mode 444, link count >= 3. And
   `ls -la /var/lib/go-choir/vm-state/.jail/.shared/` holds one store copy.
4. Lifecycle on the first: hibernate by snapshot and wake
   (`scripts/s3_snapshot_probe.py` pattern; expect `snapshot_resume`),
   stop/start, `systemctl restart go-choir-vmctl` and confirm reattach,
   then the assumed-breach probe (problem record step 3).
5. Then `VMCTL_JAILER_ONLY=class:test`; a fresh `qa_test_computer.mjs`
   computer comes up jailed with no id in the list. Owner decision before
   dropping the list (step 4 of the rollout cold boots every hibernated
   computer once).

### NET-08 and RES-16 (Firecracker 1.17 and discard, one test computer)

0. Precondition (2026-10-22): the NET-08 test computer must not be in
   `VM_MEMORY_HOTPLUG_ONLY`. Firecracker 1.17 puts the hotplug region at
   512 GiB, past the E-2236's 39-bit physical address limit
   (`grep 'address sizes' /proc/cpuinfo`; firecracker #6255, open). Do not
   move a hotplug computer to 1.17 until #6255 is fixed. Re-check the
   latest 1.17.x release notes for it before filling the hashes.
1. Host step: fill both hashes in `nix/firecracker-next.nix` (build it on
   Node B or the Mac; each fakeHash failure prints the real hash; check the
   latest 1.17.x tag first). Then in `nix/node-b.nix`:
   `firecrackerNext = import ./firecracker-next.nix { inherit pkgs; };` and
   the four lines in the vmctl comment block, naming one test computer id
   in both `_ONLY` lists.
2. vmctl log: `next Firecracker build ... (Firecracker v1.17.x) for <id>;
   data discard allowed=true`. Stop/start the test computer. Its boot
   timeline names the next binary; `fc-config.json`'s `data` drive has
   `"discard": true, "io_engine": "Sync"`; another computer's has neither.
3. In the guest: `/sys/block/vdb/queue/discard_max_bytes` non-zero and
   `choir-data-fstrim.timer` active (the guest needs the current image).
4. Hibernate and wake it: expect `snapshot_resume` (same build both sides).
   Then remove the test id from `VM_FIRECRACKER_NEXT_ONLY` and wake again:
   expect `firecracker_version_mismatch` and a cold boot.
5. RES-16 measurement (guest-data-image-never-trims record, "How to prove
   it"): hourly for 6 hours, `du -k data.img` against the live store; pass
   within about 1.5x and no hourly latency regression.

### RES-06 (guest reclaim, one test computer)

1. Owed first (free page reporting, LR report steps 3 to 5): on a test
   computer without reclaim, host `VmRSS` of its Firecracker
   (`/proc/<pid>/status`) and guest procmem at settle, at a burst peak, and
   after 10 idle minutes.
2. Turn on `"VM_GUEST_RECLAIM_ONLY=<second test computer id>"`; cold boot
   it. In the guest: `cat /sys/module/damon_reclaim/parameters/enabled` is
   `Y` and `kdamond_pid` is non-negative; boot args contain the six
   `damon_reclaim.*` entries.
3. Repeat step 1's three reads on it. Pass: idle host RSS materially below
   the control at equal guest usage; guest `Cached` falls after 5 idle
   minutes; kdamond CPU under about 1% of one core; desk turn latency and
   store reads not regressed (DAMON evicting hot store pages would show as
   disk reads in the guest's `/proc/vmstat` `pgmajfault`).
4. If `enabled` reads `N`, the kernel did not take boot-time enabling;
   record it, and the next slice is a guest unit that writes the
   parameters (needs an image change).

### RES-14 (a reading)

1. Hibernate one test computer by snapshot. `journalctl -u go-choir-vmctl
   | grep 'hibernated VM <vm>'`: expected `disk_bytes_method=btrfs_extents`
   (vmctl holds CAP_SYS_ADMIN since 10e9f18e1); record `bytes` and
   `disk_bytes`.
2. `scripts/snapshot_disk_measure_compare.py` on that `snapshot/mem` (as
   in the problem record) and compare. Then the owner revisits
   `VM_SNAPSHOT_BUDGET_GIB=48` and the 36 h max age with the real per-snapshot
   size.

### L4-03 (sign-up cap)

1. Read the number of users holding a passkey (`SELECT COUNT(DISTINCT
   user_id) FROM credentials` on a copy of `auth.db`, or the auth log).
   Recommended first cap: that count plus 75 (the checklist's ~100 users
   of headroom today, less a margin for the paper and test computers).
   Owner confirms the number.
2. Set `"AUTH_REGISTRATION_CAP=<n>"`. Probe with a cap one above the
   current count on a test deploy, or by reading: a sign-up begun with a
   fresh `@example.test` address at the cap gets 403 with `code
   registration_waitlist`, no `users` row appears, and
   `registration_waitlist` holds the email; the owner can still log in;
   an existing user adding nothing sees no change.
3. Host admission stays the second layer (`VMCTL_ADMISSION_MODE=measured`,
   pressure reclaim active).

## Residuals

- Snapshot resume validates against the class recorded at hibernate time:
  a computer whose class changed while hibernated keeps its old selection
  until its next cold boot.
- `nix/firecracker-next.nix` is unevaluated; `fetchCargoVendor`'s argument
  names follow current nixpkgs and may need adjusting to the locked rev.
- Boot-time enabling of DAMON_RECLAIM and `skip_anon` on the guest kernel
  (6.18) is from kernel documentation, not observed.
- The waitlist has no owner view or invite action; QA sign-ups at the cap
  are refused like anyone's (an `@example.test` exemption would be a bypass:
  those addresses are unverifiable).
- Lane base is the FP3 round-3 candidate; FP8 must be rebased after it
  lands (one shared file, `internal/vmctl/ownership.go`).
