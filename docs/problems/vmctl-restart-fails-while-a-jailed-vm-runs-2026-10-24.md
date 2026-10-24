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
  - The slice also needs a pre-existing, correctly-owned cgroup for the
    jailer's uid-based leaves (jailer runs as root; no change there).
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
