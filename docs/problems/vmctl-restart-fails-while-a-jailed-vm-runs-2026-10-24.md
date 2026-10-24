# vmctl Cannot Restart While a Jailed VM Runs

Date: 2026-10-24, 00:1x UTC. Found by FP0 on staging. The D27 deploy
(f84c8fe34, SEC-04 step 3) restarted vmctl while one jailed VM was alive,
and vmctl stayed down from 23:57:58 to 00:16:20 UTC, about 18 minutes.
Mutation class of any fix: red (vmctl, VM lifecycle, the host's cgroup
tree). Parent record:
[jailed-firecracker-cannot-exec-dynamic-binary-2026-10-23.md](jailed-firecracker-cannot-exec-dynamic-binary-2026-10-23.md).

## Evidence (Node B)

- 23:47:05: D26's static jailer launched the first test computer,
  fp8-jailer-1023 (vm-fa28a867…), jailed: pid 782793 (after the wake),
  cgroup `system.slice/go-choir-vmctl.service/vms/vm-fa28a867…`.
- 23:57:57: the D27 switch stopped vmctl cleanly ("managed VMs left running
  for reattach"; KillMode=process). Four Firecracker processes stayed: the
  jailed one in `vms/vm-fa28a867…`, and three direct ones (the paper
  vm-1669020a…, candidate-fleet-e15cb89f…, vm-36a51fad…) in `supervisor`.
- Each of the 10 starts that followed failed:
  `Failed to spawn executor: Device or resource busy`,
  `Failed to spawn 'start-pre' task: Device or resource busy`,
  `Failed with result 'resources'`. Then `start-limit-hit`. The deploy job
  failed ("Job for go-choir-vmctl.service failed because of unavailable
  resources", deploy evidence /var/lib/go-choir/deploy-failures/40041867841-1.json).
- The cgroup state while vmctl was down (read at 00:1x): the unit root had
  0 processes and `subtree_control` = `cpu pids`; `vms` had `cpu pids`; the
  jailed leaf held 1 process; `supervisor` held 3. The unit has
  `Delegate=yes`, `DelegateSubgroup=supervisor`, `KillMode=process`, an
  `ExecStartPre` list, and systemd 261.3.
- The 23:44:53 restart (the D26 deploy) worked with the jailer on. At that
  time no jailed VM was running (the first test computer was down after
  its failed 23:07 launch).
- While vmctl was down, running guests kept running, but nothing could be
  launched, woken, refreshed or resolved. The paper had failed two vmctl
  health checks at 23:54:48 and 23:57:18, before the outage; that is a
  separate observation. The paper answered /health 200 in 22 to 44 ms at
  00:17.

## Recovery (FP0, 00:16)

1. Stopped the jailed test VM (TERM, pid 782793). It is a disposable test
   computer.
2. Cleared the delegated controllers bottom up: `-cpu -pids` in
   `vms/cgroup.subtree_control`, then in the unit root's. The two `rmdir`
   calls on the now-empty leaf and on `vms` answered EBUSY, and both
   directories remain, empty.
3. `systemctl reset-failed go-choir-vmctl`, then `systemctl start`. vmctl
   came up at 00:16:20 with `jailer=true` and reattached all three direct
   VMs, the paper included (pid 780706, epoch 13269). After the start, the
   unit root's `subtree_control` is empty, and systemd created a `.control`
   subgroup.

## Hypothesis (not confirmed)

systemd spawns ExecStartPre, a control process, into a cgroup inside the
delegated unit. Spawning fails with EBUSY while the unit's cgroup has
domain controllers (`cpu pids`) enabled in `subtree_control`. This is the
cgroup v2 rule that a cgroup giving controllers to children holds no
processes itself. Either systemd tries the unit root, or it tries to change
that `subtree_control` while `vms` has `cpu pids` enabled below. With no
jailed VM alive, `vms` held no process, and the 23:44 restart did not hit
this. That suggests the populated `vms` subtree, or its enabled
controllers, is what matters. Confirm from systemd 261's
delegation/control-process code, and with a reproduction on a disposable
host or in the agents VM, before choosing a fix.

## State now and the conservative step

- No jailed VM is running. The second test computer (fp8-jailer-1023b) was
  never launched. The jailer is rolled back to off (D28) so that no resolve
  can start a jailed VM before the next vmctl restart. That deploy's own
  restart happens with no jailed VM alive, the same condition as the clean
  23:44 restart.
- SEC-04 stays proved for one jailed launch, a hibernate and wake, and the
  breach probe (ledger 23:47 and 23:48). It is not proved for a vmctl
  restart while a jailed VM runs: that check found this problem.

## Fix directions (to evaluate)

1. Put the VMs' cgroups outside the unit: a separate slice, such as
   `choir-vms.slice`, delegated to vmctl. vmctl's own unit then never
   carries `subtree_control` with live children it doesn't own.
2. Keep the VMs under the unit, but have systemd place control processes in
   a subgroup that can never be refused (check what systemd 261 does with
   `DelegateSubgroup` for ExecStartPre), and keep the unit root's
   `subtree_control` stable across restarts.
3. A pre-start step that clears the controllers. Rejected: it cannot run,
   because the spawn of the pre-start task is what fails.

A deploy's vmctl restart must also be proven with a jailed VM alive before
`class:test` widening (SEC-04 step 5).

## Investigation (2026-10-24)

Evidence classes: **source-read** (systemd v261.3, Linux v6.18 cgroup core,
Firecracker v1.14 jailer) and **reproduced** (systemd 260.2 user manager,
kernel 6.18.37, cgroup v2, on the agents VM; no Node B access; every unit,
cgroup and process created there was removed). The reproduction used a user
service with `Delegate=yes`, `DelegateSubgroup=supervisor`, `KillMode=process`
and `ExecStartPre`; systemd 260 has the same code paths as 261.3 for
everything cited here (`unit_cgroup_disable_all_controllers` is in both).

### Cause (confirmed)

1. **Who writes `subtree_control`.** Not vmctl: no Go file in the repo writes
   `cgroup.subtree_control`. vmctl only moves its own pids into `supervisor`
   and `MkdirAll`s `vms` (`prepareJailerCgroups`). The Firecracker **jailer**
   does the write: `jailer/src/cgroup.rs: CgroupV2::write_all_subtree_control`
   (called from `write_values`, from `Env::run` via `cgroup_conf.setup()`)
   writes `+cpu`, `+pids` into the parent of the VM leaf and recursively into
   every ancestor up to the cgroup root. So `vms/` and the unit root (and
   `system.slice`) end up with `cpu pids`. The write is idempotent and nothing
   ever turns it off.
2. **Where systemd spawns start-pre.** `exec_spawn()` (src/core/execute.c)
   clones with `CLONE_INTO_CGROUP` into `params->cgroup_path`, the **unit root**,
   unless `exec_params_needs_control_subcgroup()` (execute.c:395) is true. That
   needs `EXEC_CONTROL_CGROUP`, which `service_exec_flags()` (service.c:1971)
   sets for every control command **except** `ExecCondition=` and
   `ExecStartPre=`; the main process is never spawned into a subcgroup either
   (comment in exec_spawn: it "might need to unshare the cgroup namespace
   first"). `DelegateSubgroup=`/`.control` only moves the process afterwards,
   via `cg_attach` in exec-invoke.c (`exec_params_get_cgroup_path` path), after
   the clone. So start-pre and the main process both must be accepted by the
   unit root first.
3. **Kernel refusal.** `clone3(CLONE_INTO_CGROUP)` goes
   `cgroup_css_set_fork` -> `cgroup_attach_permissions` ->
   `cgroup_migrate_vet_dst` (kernel/cgroup/cgroup.c): "apply no-internal-process
   constraint: `if (dst_cgrp->subtree_control) return -EBUSY`". The unit root
   has `subtree_control = cpu pids`, so the spawn returns EBUSY, which systemd
   logs as "Failed to spawn executor: Device or resource busy" (execute.c:600)
   and "Failed to spawn 'start-pre' task" (service.c).
4. **Systemd's own mitigation, and why it fails here.** systemd 261 has
   `unit_cgroup_disable_all_controllers()` (cgroup.c:4050), called from
   `service_start()` (service.c:3463) right before the spawn. Its comment names
   this exact failure: payload-enabled controllers "turn the cgroup into an
   internal node, causing clone3(CLONE_INTO_CGROUP) to fail with EBUSY". It
   writes `-cpu -pids` to the unit root's `subtree_control` via `cg_enable`
   (cgroup-setup.c:379) and **ignores the error** ("if sub-cgroups still have
   live processes the write will fail, but so will the upcoming spawn"). The
   kernel refuses that disable with EBUSY when any *live child* of the unit root
   has the controller in **its own** `subtree_control`
   (`cgroup_subtree_control_write`, "a child has it enabled?"). `vms/` has
   `cpu pids` (the jailer wrote it), so the disable fails, the root keeps
   `cpu pids`, and every start fails the same way until `start-limit-hit`.
   This is the sequence in the Evidence section.

Correction to the hypothesis above: the populated leaf is **not** what blocks
the disable; `vms/`'s own `subtree_control` is. The leaf only keeps `vms/`
from being removed.

### Why the restart with no jailed VM worked

At stop, `unit_prune_cgroup()` (cgroup.c:2828) calls `cg_trim(path, true)`
(cgroup-setup.c:77), whose `trim_cb` `rmdir`s every **empty** inner cgroup
(post-order; the busy ones fail with ENOTEMPTY/EBUSY and are ignored), even
though the unit root itself is kept alive by the `supervisor` survivors. With
no jailed VM, `vms/` (and its `cpu pids`) was removed at stop, so at the next
start the live-child check passed, systemd's disable cleared the unit root, and
start-pre spawned. With one jailed leaf alive, `vms/` cannot be removed, its
`subtree_control` stays, the root cannot be cleared. vmctl does nothing on
shutdown (no cleanup, by design: `KillMode=process`).

### Reproduction (agents VM, reproduced)

User unit with the node-b shape. A script as the main process (in
`supervisor`) creates `vms/`, writes `+cpu +pids` to the unit root and to
`vms/`, and puts a `sleep` into `vms/x`, as the jailer does.

| State at `systemctl --user restart` | Result |
|---|---|
| `vms/x` populated, `vms/` and root have `cpu pids` | **fails**: `Failed to spawn executor: Device or resource busy`, `Failed to spawn 'start-pre' task: Device or resource busy`, `Failed with result 'resources'`; the log line "Found left-over process ... in control group" precedes it. Same text as Node B. |
| same, then `echo "-cpu -pids" > vms/cgroup.subtree_control` (leaf **still populated**) | restart **works**; unit root bits become empty, `.control` created. Proves `vms/`'s bits, not the leaf, are the blocker. |
| `vms/` has bits but no leaf | restart **works**; `vms/` is gone after stop (`cg_trim`). Matches the clean 23:44 restart. |
| VM leaf and bits under a separate `Delegate=yes` **slice**, populated leaf, two restarts of the unit | both restarts **work**; the unit root never gets bits; the leaf is untouched. |

### Fix directions

- **Direction 2 (a systemd setting) has none.** `DelegateSubgroup=` is already
  set and does not apply to the spawn target (point 2); `.control` is not used
  for `ExecStartPre=`; no unit option changes the spawn target, and the
  mitigation in point 4 cannot succeed while a descendant holds the bits. A
  vmctl-side "clear the bits before exit" would work mechanically (leaf
  controllers can be disabled with the leaf populated) but silently drops the
  running VMs' `pids.max` and `cpu.weight` until vmctl re-enables the bits and
  re-applies them at boot; rejected as fragile.
- **Direction 1 (separate delegated slice) is the recommended fix**, and it is
  the only option where the failure class cannot occur: the jailer then writes
  controller bits only under the slice, and nothing systemd spawns ever goes
  into that cgroup. Implementation notes:
  - vmctl already honors `VMCTL_JAILER_CGROUP_PARENT` (an explicit parent skips
    the derivation, `prepareJailerHost`); set it to `<slice>/vms`. Add
    `Wants=`/`After=` on the slice so it exists before vmctl starts.
  - **Slice name:** a dash in a slice name is hierarchy (`choir-vms.slice` is
    child of `choir.slice`). Use a name without a dash, or accept that parent.
  - `MemoryLow=14G` and `MemorySwapMax=0` on the unit currently cover the jailed
    VMs; move them (or split the budget with the direct VMs, which stay in
    `supervisor`) onto the slice. Memory protection must be re-measured.
  - Never stop or restart the slice in a deploy (stopping it kills the VMs).
    Whether NixOS `switch-to-configuration` leaves a changed `.slice` alone
    needs confirming before the deploy; this was not tested.
- **Existing jailed VMs when the parent changes:** a live process keeps its
  cgroup, so a VM launched under `.../go-choir-vmctl.service/vms/<vm>` stays
  there. `cgroupLeafRel` derives the leaf from the current process-wide parent,
  so after the switch vmctl would remove the wrong path and leak the old leaf,
  and the old `vms/` bits would still wedge the switching deploy's own restart.
  Switch while **no** jailed VM is alive (true now: the jailer is off since
  D28, and an empty old `vms/` is pruned at the next stop). The receipt's
  `Cgroup` field (confinement.go:143) is the path to use if a cleanup must
  handle old leaves.
- Proof before `class:test` widening (SEC-04 step 5): a vmctl restart with a
  jailed VM alive, on a deploy, in both the restart and the stop/start order.

## Design: the VM slice (2026-10-24, lane `lane/fp0-vm-slice`)

Written before code. Mutation class red (vmctl, VM lifecycle, the host's
cgroup tree, a unit vmctl's boot pulls in). Evidence classes: source-read
(nixpkgs `fe1324228`, systemd 261.3 as cited above), reproduced (agents VM,
systemd 260.2 user manager, kernel 6.18.37; run 2026-10-24, everything
removed after).

### Two findings that shape it

1. **`Delegate=` is not supported on slices.** systemd 260.2 logs
   `fp0vms.slice:5: Delegate= setting not supported for this unit type,
   ignoring.` and `systemctl show -p Delegate` reads `no`. The
   investigation's "separate `Delegate=yes` slice" row worked, but with
   `Delegate=` ignored. So the slice is a plain systemd slice, and `vms/`
   under it is a cgroup systemd did not create. That departs from
   systemd's delegation contract (only delegated subtrees are the
   payload's), and is a named residual below. What systemd does to such a
   subtree, reproduced: realizing the slice (start, daemon-reload with a
   changed property) leaves `vms/` and the leaf alone; it cannot take the
   jailer's `cpu pids` off the slice while `vms/` has them in its own
   `subtree_control` (the same kernel check as cause point 4); stopping the
   slice stops its member units (it has none) and does **not** kill the
   leaf's process; an empty slice cgroup is removed at stop, a populated
   one stays. The delegation-correct alternative, one transient
   `Delegate=yes` scope per VM (what machined and docker's systemd driver
   do), needs a pid at scope creation and puts the jailer itself in the
   scope root it then writes `subtree_control` for, so it does not fit the
   jailer's own cgroup setup. A holder service with `Delegate=yes` and a
   placeholder process carries the same wedge class on its own restart.
   Neither is worth it for slice 1.
2. **NixOS switch never restarts or stops a changed slice.**
   switch-to-configuration-ng (`pkgs/by-name/sw/switch-to-configuration-ng/src/main.rs`,
   nixpkgs pin above), `cannot_be_restarted_directly()`: `unit.ends_with(".slice")`
   is in the "Do nothing. These cannot be restarted directly" set, with the
   comment "Slices and Paths don't have to be restarted since properties
   (resource limits and inotify watches) seem to get applied on
   daemon-reload". `handle_modified_unit` returns before any stop, start or
   restart for them. A slice whose unit file is **removed** is stopped
   unless its `[Unit]` says `X-StopOnRemoval=false` (main.rs, the
   `unit_file_state(&new_base_unit_file)?.is_gone()` branch reads the
   *current* unit file's `X-StopOnRemoval`, default true). The slice sets
   it false, so a rollback that removes the slice does not stop it either.
   `restartIfChanged`/`stopIfChanged` are service options and do not apply.
   Reproduced: a changed `MemoryLow=` plus daemon-reload applied to the
   slice's `memory.low` with the leaf's pid unchanged; a removed slice file
   plus daemon-reload left the slice active and the leaf alive.

### The shape

- **Name:** `system-choirvms.slice`, cgroup `system.slice/system-choirvms.slice`.
  The dash is used on purpose: it makes the slice a child of
  `system.slice`, a sibling of `go-choir-vmctl.service`, so the jailed VMs
  keep the ancestors they had (minus the unit) and the same CPU and memory
  competition position as today. A top-level `choirvms.slice` would compete
  with all of `system.slice` at the root (weight 100 against 100), giving
  test VMs more CPU under contention than the paper. No `choir.slice` or
  `choir-*` prefix is created.
- **Properties** (`systemd.slices.system-choirvms` in nix/node-b.nix):
  `MemorySwapMax=0` (required: guests never swap, and `guestMemoryCannotSwap`
  in snapshot_diff.go walks the VM's cgroup up to the first
  `memory.swap.max` 0, so without it every jailed diff snapshot is refused
  `diffRefusedMaySwap`); `MemoryLow=4G` (one 4 GiB guest; the two jailed
  test computers are the only members until `class:test`). The vmctl unit
  keeps `MemoryLow=14G` and `MemorySwapMax=0` for the direct VMs, which stay
  in its `supervisor` subgroup. Total configured protection rises from 14G
  to 18G of 31 GiB, but `memory.low` protects usage only, so with no jailed
  VM it costs nothing. CPU: the slice default (`CPUWeight` 100), per-VM
  `cpu.weight` and `pids.max` from `VMCTL_JAILER_CPU_WEIGHT`/`_PIDS_MAX` as
  today. No `MemoryMax` (failure item 13 of the jailer record). No
  `Delegate=` (finding 1). `unitConfig.X-StopOnRemoval = false` (finding 2).
  No `wantedBy`: vmctl pulls it in.
- **Ordering:** vmctl gets `wants` and `after` on `system-choirvms.slice`,
  not `requires`/`bindsTo`: stopping the slice never stops vmctl, stopping
  vmctl never touches the slice. systemd creates the slice's (empty)
  cgroup when the slice starts and applies its memory properties
  (reproduced with a member-less slice).
- **Write access:** vmctl runs as root with `ProtectControlGroups=false`
  (unchanged); its mount namespace has `/sys/fs/cgroup` read-write, which
  is how the jailer already writes `system.slice`'s `subtree_control`
  today. Nothing new is granted. vmctl's own unit keeps `Delegate=yes` and
  `DelegateSubgroup=supervisor` (the direct VMs live in `supervisor`;
  changing that would move live processes). Narrowing that is a follow-up.
- **`VMCTL_JAILER_CGROUP_PARENT`:** `system.slice/go-choir-vmctl.service/vms`
  becomes `system.slice/system-choirvms.slice/vms`. Leaves are
  `<slice>/vms/<vm>`; the jailer writes `+cpu +pids` into `vms/`, the
  slice, `system.slice` and the root, never into vmctl's unit.
- **vmctl (internal/vmmanager/confinement.go):**
  - `deriveJailerCgroupParent` is **deleted**. It derived `<unit>/vms`,
    which is exactly the layout that wedges the unit. An unset
    `VMCTL_JAILER_CGROUP_PARENT` leaves the jailer host unprepared (JAILER
    UNAVAILABLE, every launch direct, receipts say why).
  - `prepareJailerCgroups` refuses a parent inside vmctl's own unit cgroup
    (the nearest `*.service`/`*.scope` ancestor of vmctl's cgroup) with a
    reason naming this record, so a rollback of the env line alone fails
    safe to direct launches instead of re-arming the wedge. For a parent
    outside: it never moves any process and never writes any
    `subtree_control`; it requires the parent's parent (the slice's
    cgroup, systemd's) to exist (a missing one means the slice is not
    active; vmctl must not create a systemd-owned cgroup behind systemd's
    back); it refuses when the slice's cgroup or `vms/` holds processes
    (the jailer's `subtree_control` write would fail EBUSY on every
    launch); it creates only `vms/` (`Mkdir`, not `MkdirAll`). The
    process-moving code for the old layout is deleted with it.
  - **Leaf cleanup** (`removeCgroupLeaf`, on a jailed exit and on destroy)
    removes the leaf the launch receipt recorded (`confinement.json`
    `cgroup`, validated: clean, relative, last element the VM id) as well
    as the leaf under the current parent. `rmdir` only removes an empty
    cgroup, so a stale or wrong path costs one logged error.
- **The leftover `go-choir-vmctl.service/vms/` and `vms/vm-fa28a867…` on
  Node B:** harmless. Their `subtree_control` was cleared at 00:16, so
  systemd's pre-spawn disable of the unit root succeeds (the D28 restart
  at 00:2x was clean with them present, if they were still present). The
  EBUSY `rmdir` at 00:16 most likely met a not-yet-released exiting task in
  the leaf (hypothesis, not checked); `cg_trim` at the next vmctl stop
  removes empty subcgroups of the unit. No code handles them. The staging
  proof reads them.
- **Switch-over rule:** deploy this only while no process lives under
  `go-choir-vmctl.service/vms` (true since D28: jailer off). Checked as
  proof step 0. The switching deploy's own vmctl restart then meets the
  old layout empty, which restarts cleanly (the 23:44 and D28 cases).
- **Rollback, in order:** (1) `VMCTL_JAILER=off` and deploy: new launches
  go direct, a jailed snapshot is refused and cold boots (today's
  rollback). The slice stays; it is harmless and holds no protection in
  use once its VMs are gone. (2) Only after no process lives under the
  slice, `git revert` of the lane. Reverting the Go and nix together while
  a jailed VM runs in the slice is the one unsafe order: the old vmctl
  would put new leaves back under its unit. Reverting the env line alone is
  safe (vmctl refuses the in-unit parent).

### Failure modes (before code)

1. vmctl restart with a jailed VM alive wedges the unit: the unit's cgroup
   never gets controller bits (no leaf under it). Reproduced twice plus a
   stop/start.
2. Deploy restarts or stops the slice, killing VMs: switch never restarts
   or stops a changed slice (finding 2); removal is covered by
   `X-StopOnRemoval=false`; a manual `systemctl stop` of the slice does not
   kill the leaf (reproduced). A manual `systemctl kill` of the slice
   would; never do that.
3. Slice not active at vmctl start: `wants`/`after` start it first; if it
   is still missing, vmctl refuses the jailer (direct launches), never
   creates the slice's cgroup itself.
4. The slice or `vms/` holds a process (someone ran something there): the
   jailer's write would fail every launch; vmctl refuses up front. vmctl
   never moves processes it does not own.
5. The parent configured inside vmctl's unit (old env, a typo, a partial
   rollback): refused with this record named.
6. Parent unset: refused (no derivation).
7. Old-layout leaves after the switch: none by the switch-over rule; if one
   existed, the receipt path removes it at exit.
8. systemd strips the jailer's controller bits from the slice: it cannot
   while `vms/` has them (kernel check); if `vms/` is empty and pruned at a
   slice stop, the jailer re-enables them at the next launch.
9. Jailed diff snapshots refused for swap: the slice carries
   `MemorySwapMax=0`.
10. Protection: `MemoryLow` on a child of `system.slice` behaves as the
    vmctl unit's does today. Open question, not acted on: the kernel bounds
    a cgroup's effective `memory.low` by every ancestor's, and the agents
    VM reads `system.slice/memory.low` 0 under `memory_recursiveprot`. If
    Node B's `system.slice` also reads 0, the unit's 14G (and the slice's
    4G) protect nothing today. Read-only check on Node B:
    `cat /sys/fs/cgroup/system.slice/memory.low` and the `low` counter of
    `go-choir-vmctl.service/memory.events`.
11. systemd's own contract: `vms/` is not delegated (finding 1). A future
    systemd that prunes or migrates foreign cgroups in a slice would break
    this; re-run the reproduction on each systemd bump.

### Staging proof plan (FP0 runs it; not run by the lane)

Node B, after the lane lands and deploys with `VMCTL_JAILER=off` first:

0. Pre-deploy, read only: `cat /sys/fs/cgroup/system.slice/go-choir-vmctl.service/vms/cgroup.procs`
   (and every leaf's) is empty; the unit root's `cgroup.subtree_control`
   is empty. If not, stop: the switch-over rule is not met.
1. After the deploy: `systemctl show -p ActiveState system-choirvms.slice`
   is active; `cat /sys/fs/cgroup/system.slice/system-choirvms.slice/memory.swap.max`
   reads 0; the vmctl journal shows `jailer=` with no JAILER UNAVAILABLE
   for the cgroup parent (it is off, so expect off); the old `vms/` dirs
   are gone or empty.
2. A deploy with `VMCTL_JAILER=on`, `VMCTL_JAILER_ONLY` = the two test
   computers (fp8-jailer-1023 `computer-476cd794…`, fp8-jailer-1023b
   `computer-51443441…`). Resolve both; each receipt reads `mode: jailer`
   and `cgroup: system.slice/system-choirvms.slice/vms/<vm>`;
   `scripts/vmm_confinement_verify.py` passes for both; the vmctl unit
   root's `cgroup.subtree_control` stays empty.
3. With both jailed VMs alive, a deploy that restarts vmctl (any vmctl
   change, or a second deploy). Expect: the vmctl restart succeeds first
   try (no `Failed to spawn`, no `start-limit-hit`), both jailed VMs are
   reattached with the same pids, both answer `/health`, the paper is
   reattached, and the leaves still hold their pids. Then a
   `systemctl stop` / `start` of vmctl (the other order), same result.
4. RES-05 script step 5 (one shared store image across the two jailed
   computers) on the restarted vmctl.
5. Hibernate and wake one jailed computer: its leaf is removed at exit and
   recreated at wake; a diff snapshot is not refused for swap.

Pass: steps 1 to 5. Then SEC-04 step 5 (`class:test`) is unblocked, with
the slice's `MemoryLow` rebalanced against the unit's as members move.
