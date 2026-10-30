# Memory Hotplug Controller: Design and Failure Modes

Date: 2026-10-14. Owner request (in session, "so let's do this!"): a Choir
computer's memory grows and shrinks at run time with Firecracker's
virtio-mem hotplug, so a capsule running Playwright/Chromium gets memory
while the browser runs and gives it back afterwards.

- Mutation class: **red** (vmctl, VM lifecycle, Firecracker launch config,
  host memory admission, a new guest-to-host call).
- Parent analysis: [Playwright in capsules](../../reports/playwright-in-capsules-memory-2026-10-15.md)
  section 1.4 (virtio-mem) and section 6 slice S4; density lever L11 in
  [VM density limits](../../reports/vm-density-limits-2026-10-14.md).
- Backend seam: [VM backend interface](../../reports/vm-hypervisor-seam-design-2026-10-21.md)
  (`MemoryController`, capability matrix row "virtio-mem").
- This commit is documentation only. It names the problem and the failure
  modes before any code (CLAUDE.md, "Problem Documentation First").

## 1. The problem

A browser run in a capsule peaks at 0.4 to 1.7 GiB (Playwright report,
section 2). Today a computer gets that memory in one of two ways:

- **Boot it bigger.** The ceiling costs memmap (about 1.6% of the ceiling,
  resident for the VM's life), lets guest page cache grow toward it, and
  makes every full snapshot larger.
- **Share what it booted with.** The browser competes with the build in
  the same capsule budget.

Neither returns memory to the host on demand:
- free page reporting returns only what the guest frees in 2 MiB blocks;
- nothing on the host can take memory back from a running guest;
- vmctl cannot see how much memory a computer currently has without SSH
  (the VM list shows only the measured peak RSS).

What is missing is a **host-side controller**. Firecracker 1.15.1 has the
device; vmmanager never configures it, and nothing calls its run-time API.

## 2. Firecracker 1.15.1 API (verified)

Verified on 2026-10-14 against the `v1.15.1` tag of
firecracker-microvm/firecracker:

- `docs/memory-hotplug.md` (whole file);
- `src/firecracker/swagger/firecracker.yaml`, path `/hotplug/memory`
  (PUT, PATCH, GET) and definitions `MemoryHotplugConfig`,
  `MemoryHotplugSizeUpdate`, `MemoryHotplugStatus`;
- `src/vmm/src/vmm_config/memory_hotplug.rs` (validation);
- `src/vmm/src/resources.rs` (`VmmConfig` is `rename_all = "kebab-case"`,
  field `memory_hotplug`, so the config-file key is `memory-hotplug`);
- `src/vmm/src/devices/virtio/mem/device.rs` (`update_requested_size`,
  `VirtioMemStatus`, `VirtioMemError`);
- `src/vmm/src/lib.rs` (`memory_hotplug_status`,
  `update_memory_hotplug_size`);
- `src/vmm/src/builder.rs` (`attach_virtio_mem_device` attaches it as an
  ordinary virtio device, so it rides `--enable-pci` like the others).

| what | exact shape | source |
|---|---|---|
| boot config (config file) | `"memory-hotplug": {"total_size_mib": N, "block_size_mib": B, "slot_size_mib": S}` | resources.rs `VmmConfig`; swagger `memory-hotplug` |
| boot config (API) | `PUT /hotplug/memory` with the same body; pre-boot only, never on a restored VM | memory-hotplug.md "API Configuration" |
| validation | `block_size_mib` >= 2 and a power of 2; `slot_size_mib` >= 128 and a multiple of the block; `total_size_mib` >= slot and a multiple of the slot. Unknown fields are denied | memory_hotplug.rs `validate`, `deny_unknown_fields` |
| defaults | block 2 MiB, slot 128 MiB | memory_hotplug.rs; swagger |
| run time | `PATCH /hotplug/memory` `{"requested_size_mib": R}`; 204 | swagger `patchMemoryHotplug` |
| run-time errors | R not a multiple of the block or larger than the total: `InvalidSize`; guest driver not bound yet: `DeviceNotActive` | device.rs `update_requested_size` |
| status | `GET /hotplug/memory` → `{total_size_mib, slot_size_mib, block_size_mib, plugged_size_mib, requested_size_mib}` | swagger `MemoryHotplugStatus`; device.rs `VirtioMemStatus` |
| at boot | the region is fully unplugged; requested starts at 0 | memory-hotplug.md |
| plug | asynchronous; done when `plugged_size_mib == requested_size_mib` | memory-hotplug.md "Hotplugging Memory" |
| unplug | the host frees a block (`MADV_DONTNEED`) as soon as the guest reports it; a fully unplugged slot is removed from KVM and `mprotect`ed. "Unplugging requires the guest to cooperate" | memory-hotplug.md "Hot-removing Memory", "Memory Protection" |
| snapshots | full and diff snapshots carry unplugged areas as sparse holes; a restored VM uses the saved hotplug config | memory-hotplug.md "Snapshots" |

The doc's prose once says `plugged_memory_mib`/`requested_memory_mib`; the
swagger and `VirtioMemStatus` both say `plugged_size_mib` and
`requested_size_mib`. The code uses the swagger names.

**UNVERIFIED** (no KVM locally; the staging probe measures them):
- plug and unplug latency on Node B;
- whether `PATCH` after a snapshot restore works. The source has no
  restore-specific guard (`update_memory_hotplug_size` checks only that the
  device is active and the size is valid), but it has not been exercised;
- how much of a 1 GiB lease the 6.18 guest can unplug after a Chromium run
  (it depends on movable-zone placement and page cache);
- whether the plugged region shows up in the firecracker process's RSS
  only when touched (expected: the region is a private anonymous mapping).

## 3. Guest requirements

From `docs/memory-hotplug.md` "Prerequisites" and "Configuring the guest
driver":

- `CONFIG_VIRTIO_MEM`, with kernel 5.16 or later on x86_64. The guest
  kernel (6.18.37 then; 6.18.55 on main, section 9.1) already builds `VIRTIO_MEM=m` and
  `MEMORY_HOTREMOVE=y` (density report, section 2). No kernel config
  change is needed.
- The driver must be bound before the host can `PATCH`
  (`DeviceNotActive`). udev loads `virtio_mem` by modalias; the guest NixOS
  config adds it to `boot.kernelModules` so the load is explicit, as it
  already does for `virtio_balloon`. That is one line in
  `nix/autoputer-vm.nix`, kept separate from the guest-kernel bump on
  branch `ops/kernel-6.18-security`. Inert without the device.
- `memhp_default_state=online_movable` on the kernel command line, so
  plugged memory is onlined automatically into `ZONE_MOVABLE` and can be
  unplugged again. vmmanager appends it only to computers launched with
  the device, so no other computer's command line changes.
- memmap for plugged memory comes from boot memory: 64 bytes per 4 KiB
  page, 16 MiB per plugged GiB. A 4 GiB region fully plugged needs 64 MiB
  of free boot memory. `memmap_on_memory` is not used: it does not work for
  memory onlined entirely as movable.

## 4. Design

### 4.1 Flags and selection (all off by default)

| variable | read by | default | meaning |
|---|---|---|---|
| `VM_MEMORY_HOTPLUG` | vmctl and vmmanager | off | master switch; both sides must be on |
| `VM_MEMORY_HOTPLUG_ONLY` | vmctl | empty (no computer) | comma list of VM ids, computer ids, `owner:<user_id>`, `class:test` (the test warmness class), or `*` |
| `VM_MEMORY_HOTPLUG_TOTAL_MIB` | vmctl | 4096 | the hotplug region a selected computer boots with |
| `VM_MEMORY_HOTPLUG_BLOCK_MIB`, `VM_MEMORY_HOTPLUG_SLOT_MIB` | vmmanager | 2, 128 | Firecracker's block and slot |
| `VMCTL_MEMORY_LEASE_MAX_MIB` | vmctl | 2048 | the most one computer may hold above its base |
| `VMCTL_MEMORY_LEASE_TTL`, `VMCTL_MEMORY_LEASE_MAX_TTL` | vmctl | 10m, 30m | lease length when not asked, and its cap |
| `VMCTL_MEMORY_GROW_MAX_PSI` | vmctl | 1.0 | a grow is refused while host memory PSI some avg10 is above this |

Selection is a boot-time machine-shape decision, like `MachineMemSizeMib`:
vmctl decides it per ownership and passes `MemoryHotplugMiB` in the VM
config. Platform computers are never selected. vmmanager adds the device
only when its own `VM_MEMORY_HOTPLUG` is also on and the sizes validate
(defense in depth; a bad size omits the device and logs, it never fails a
boot). A computer that booted before it was selected gets the device at its
next cold boot.

### 4.2 Launch (vmmanager)

- `fc-config.json` gains `"memory-hotplug": {total, block, slot}`.
- The command line gains `memhp_default_state=online_movable`.
- A selected computer always gets an API socket, even in kill hibernate
  mode (run-time `PATCH` needs it). Unselected computers launch exactly as
  today.
- The snapshot launch-config hash includes the block, so a snapshot taken
  with a different selection never restores; it cold boots (S3's existing
  rule).

### 4.3 Run-time API (vmmanager)

- `MemoryHotplugStatus(vmID)` → `GET /hotplug/memory` plus the base size.
- `SetMemoryHotplugRequestedMiB(vmID, mib)` → `PATCH /hotplug/memory`.
- Both take the VM's operation lock with try-lock: a VM mid-boot,
  mid-hibernate or mid-restore answers "busy" instead of blocking, and the
  controller retries on its next tick.
- Backend mapping: Cloud Hypervisor's `vm.resize` takes the desired total
  RAM, so the neutral contract is "total = base + requested hotplug". The
  vmctl route speaks totals (`target_mib`); only the Firecracker adapter
  subtracts the base.

### 4.4 The vmctl controller

Grows are **leases**. A lease has a target total, a holder (`host` or
`guest`), and an expiry. Per computer:

```text
base ──grow(admitted)──> growing ──plugged==requested──> leased
  ^                         |                               |
  |                         └──────── release / expiry / revoke ┘
  |                                         v
  └──────────── plugged==0 ──────────── shrinking ──(stall)──> shrink_stalled
                                            ^                     |
                                            └──── retry PATCH 0 ──┘
```

- **Grow admission** (only for an increase). Refused when:
  - the computer is not selected, or its Firecracker answers that no
    device exists;
  - the target exceeds base + `min(LEASE_MAX_MIB, region)`;
  - host memory PSI some avg10 > `VMCTL_MEMORY_GROW_MAX_PSI`;
  - the host sample is missing (a grow is optional, so unknown refuses;
    starts still admit on a missing sample);
  - `MemAvailable < floor + headroom + reserved + lease`, where `reserved`
    is the start ledger's outstanding reservations plus every other
    computer's lease (the same ledger as start admission, so starts and
    grows cannot both pass one reading).
- **Leases are reservations** in the start admission ledger, at their full
  size, until the unplug completes. This is conservative (memory the guest
  has touched is counted both in `MemAvailable` and in the lease) and makes
  start admission see grows.
- **Renewal** is the same request again: same or smaller target extends the
  expiry without admission; a larger one is a new grow.
- **Expiry and release** set the request to 0. A crashed browser or a
  guest that never releases cannot pin memory past the TTL.
- **Revocation under pressure**: when host MemAvailable falls below the
  floor, every lease is ended (shrink) before the sustained-pressure
  reclaim hibernates a bystander.
- **Shrink cooperation**: the controller polls `GET` every tick. Partial
  unplug is normal; the controller re-sends `PATCH 0` every 15 s (re-raising
  the config interrupt so the guest driver retries), records progress, and
  marks `shrink_stalled` after 2 minutes without reaching 0. A stalled
  shrink keeps its ledger reservation (the host still backs that memory)
  and keeps retrying.
- **vmctl restart**: leases live in vmctl memory. A restart is a crash
  under the restart rule, so leases end: the controller's periodic scan
  finds a selected computer with `requested > 0` and no lease, and shrinks
  it. A guest that still wants memory asks again and is admitted afresh.
- **VM gone**: a stopped, hibernated or failed computer drops its state and
  its reservation.

### 4.5 Routes

- `POST /internal/vmctl/memory` — host only (`isInternalCaller`: loopback or
  UDS transport **and** the hostauth credential). Body
  `{vm_id | computer_id, target_mib, ttl_seconds?, wait_seconds?}`.
  `target_mib` is the total guest memory; base (or 0) releases.
- `GET /internal/vmctl/memory[?vm_id=]` — host only; one computer or all,
  with counters.
- `POST /internal/vmctl/memory/guest` — guest only. Body
  `{target_mib, ttl_seconds?, wait_seconds?}`. The computer is the one the
  caller's tap source address binds to (`GuestOwnershipForAddress`, the
  rule the self-development route already uses); a body that names another
  `vm_id` or `computer_id` is refused with 403. The guest can only grow or
  shrink itself.
- `GET /internal/vmctl/list` gains `memory_base_mib`, `memory_current_mib`,
  `memory_requested_mib`, `memory_plugged_mib`, `memory_lease_expires_at`,
  `memory_hotplug_phase` and `memory_rss_bytes` (the latest host RSS
  sample). The list reads the controller's cache; it never calls
  Firecracker.

### 4.6 The guest path, and why it is least privilege

Guests already call vmctl directly: the kernel command line carries
`choir.vmctl_url=http://<host_ip>:8083`, the guest runtime reads it as
`RUNTIME_VMCTL_URL`, and `internal/autoputer/run.go` builds a
`vmctl.NewClient` from it for the self-development route. Port 8083 is in
`tapReachableHostServicePorts`, and the tap INPUT rule admits only the
guest's own source address (S1a anti-spoofing).

Alternatives considered:
- **guest → corpusd/gateway → vmctl**: adds a hop that would have to
  re-derive the same tap identity and would hold a host credential able to
  resize *any* computer. More privilege in the middle, not less.
- **the computer's own API through the proxy**: authenticates the owner's
  browser session, not the guest; it would let an owner session resize,
  and the guest would need an owner credential.
- **guest → vmctl over the tap, identity from the source address**: the
  guest names no target; vmctl derives it from an address the guest cannot
  forge. It cannot reach another computer's lease, and it holds no host
  credential. Chosen.

The guest side is a client helper (`vmctl.Client.GuestMemoryLease`,
`WithGuestMemoryLease`) the guest runtime calls around a browser capsule:
request, renew at half the TTL, release on return (release uses a fresh
context, so a cancelled run still releases; if the release is lost, the TTL
ends the lease). Wiring it into the capsule capture step is S1 work: that
step does not exist yet.

### 4.7 Snapshot and hibernate

The safe default is to shrink to base before a snapshot:

- after the guest fence (so no run holds browser memory) and before the
  pause, vmmanager sends `PATCH 0` and polls up to 10 s for `plugged == 0`;
- if the guest cannot unplug everything in time, the snapshot proceeds.
  Firecracker snapshots plugged blocks as data and unplugged ones as holes,
  and the restored VM keeps the device; the controller's scan then shrinks
  the orphan after restore. The hibernate timeline records
  `hotplug_unplug` with the plugged size before and after;
- the controller's lease for that computer ends when the VM leaves
  `running`.

A computer mid-lease is busy, so the guest fence refuses a hibernate while
a browser run holds admission, and pressure reclaim skips active computers.

## 5. Failure modes (written before the code)

Each row names the handling and the test that pins it.

| # | failure | handling | test |
|---|---|---|---|
| F1 | flag off, or the computer is not selected | no device, no API change, launch identical to today; routes answer 404 (flag off) or 409 (not selected) | config: no `memory-hotplug` key; route: disabled and not-selected answers |
| F2 | bad sizes (total not a multiple of slot, block not a power of 2) | device omitted, logged, boot proceeds | config generation test |
| F3 | kill hibernate mode has no API socket | a selected computer gets `--api-sock` in both modes | launch args test |
| F4 | guest driver not bound (`DeviceNotActive`) or no device | grow answers 409 with Firecracker's message; nothing is reserved | controller: unavailable device |
| F5 | grow under host pressure: PSI high, MemAvailable short, sample missing | refused with the inputs; ledger decision recorded | controller: refused grows |
| F6 | concurrent grows and starts pass one reading | leases are ledger reservations taken under the ledger lock | controller: second grow sees the first lease |
| F7 | target above the per-computer max or the region; not block-aligned | refused (max) or rounded up to the block | controller: max refusal, rounding |
| F8 | crashed browser or guest never releases | lease expiry shrinks to base | controller: expiry |
| F9 | guest cannot unplug (memory in use) | retry `PATCH 0` every 15 s; `shrink_stalled` after 2 min; reservation kept while plugged | controller: partial unplug then completion; stall flag |
| F10 | guest asks to grow another computer | identity comes from the tap source; a body naming another id is 403; loopback without credential is 403 | route auth tests |
| F11 | host caller without hostauth credential | 403 | route auth test |
| F12 | vmctl restart loses leases | periodic scan shrinks `requested > 0` with no lease | controller: orphan reclaim |
| F13 | hibernate mid-lease snapshots browser pages | unplug before pause, bounded; proceed if partial | vmmanager snapshot unit test (fake API) |
| F14 | VM operation in flight (boot, hibernate, restore) | try-lock; "busy" retried next tick | vmmanager: busy answer |
| F15 | VM stops while leased | state and reservation dropped | controller: VM gone |
| F16 | host sustained pressure while leases exist | leases revoked (shrink) | controller: revoke under floor |
| F17 | malicious guest touches unplugged memory | Firecracker faults and may crash: only that computer's VMM dies (documented upstream trust model) | none (upstream guarantee) |
| F18 | guest's `MemTotal`-derived limits read once at start (`internal/gomemlimit`, capsule budget at `internal/autoputer/capsule_executor_linux.go:56`) | landing pass (section 9): the capsule budget is read live and a job's declared need raises its capsule's cgroup; `gomemlimit` stays a residual (J14) | section 9.4 |
| F19 | snapshot taken under a different selection | launch-config hash differs; the snapshot is discarded and the computer cold boots | existing S3 rule |
| F20 | the list endpoint blocks on a hung Firecracker | the list reads the controller cache only | list test |

## 6. Staging probe

`scripts/memory_hotplug_probe.py` runs only on a computer it mints (a test
computer, selected by `class:test`) or one named with `--computer-id` that
vmctl reports as selected. It:
1. records baseline host RSS of the firecracker process and guest
   `MemTotal` (guest health through the existing autoputer proxy route);
2. grows by 1 GiB through `POST /internal/vmctl/memory`, waits for
   `plugged == requested`, and checks guest `MemTotal` rose;
3. shrinks to base and checks the firecracker RSS returns within 128 MiB of
   baseline within 2 minutes;
4. stops at host memory PSI some avg10 > 1.0 at any step, and shrinks
   before exiting on any failure.

## 7. Rollback

- Unset `VM_MEMORY_HOTPLUG` (or empty `VM_MEMORY_HOTPLUG_ONLY`): new boots
  get no device; routes answer 404; running leases end at their TTL or at
  the next vmctl restart scan. Selected computers keep the device until
  their next cold boot (harmless at requested 0).
- Repo rollback: `git revert` the feature commits.

## 8. Heresy delta

- **discovered:** vmctl's VM view had no current-memory field; `fcapi.go`
  could not read a response body (only status), so no run-time GET existed.
- **introduced:** a new guest-to-host call (bounded by tap identity, TTL,
  per-computer max and admission).
- **repaired:** none yet; the list field repairs the first discovery when
  the code lands.

## 9. Landing: capsule jobs take leases (second pass, 2026-10-14)

The branch was rebased onto main after main gained capsule jobs
(`internal/capsulejob`, with a `SetJobAdmission` hook left for this
controller), the Claude delegate, the hostauth boundary, the always-on
planned refresh and Dolt GC. This section names what the landing adds and
its failure modes before the code (CLAUDE.md, "Problem Documentation
First"). Mutation class: **red** (vmctl selection turned on for test
computers, a guest-to-host call on the job path, capsule cgroup limits).

### 9.1 Guest kernel (verified on the built config)

Main's guest kernel now comes from the `nixpkgs-kernel` input
(nixos-unstable, rev `7fd4004d0511`), so section 3's "6.18.37" is stale.
Evaluated from this tree on 2026-10-14:

- `boot.kernelPackages.kernel.version` = **6.18.55**;
- configfile `/nix/store/63z3i6xz2dw1v1gvbfqj0vkjjwyjdrp8-linux-config-6.18.55`
  (fetched from cache.nixos.org): `CONFIG_VIRTIO_MEM=m`,
  `CONFIG_MEMORY_HOTPLUG=y`, `CONFIG_MEMORY_HOTREMOVE=y`,
  `CONFIG_MIGRATION=y`, `CONFIG_CONTIG_ALLOC=y`, `CONFIG_SPARSEMEM_VMEMMAP=y`,
  `CONFIG_MHP_DEFAULT_ONLINE_TYPE_ONLINE_AUTO=y` (the
  `memhp_default_state=online_movable` argument vmmanager adds to selected
  computers overrides that default);
- the module is shipped: `virtio_mem.ko.xz` is in
  `/nix/store/14ph847b27s9cvmrz544mav0vbx9v1k6-linux-6.18.55-modules/lib/modules/6.18.55/kernel/drivers/virtio/`,
  and `boot.kernelModules` lists `virtio_mem`.

That the module **loads** on a live guest is not provable locally (no
KVM). The guest now reports it: `GET /api/runtime/observability` →
`memory.virtio_mem_loaded` (`/sys/module/virtio_mem` exists) and
`memory.virtio_mem_bound` (`/sys/bus/virtio/drivers/virtio_mem` lists a
device). The staging probe requires both on the test computer.

### 9.2 Selection

`nix/node-b.nix` sets `VM_MEMORY_HOTPLUG=on` and
`VM_MEMORY_HOTPLUG_ONLY=class:test`. vmctl reads both; vmmanager reads the
first. A test computer gets the device at its **next cold boot** (a
snapshot restore keeps the launch it was taken with; the launch-config
hash then differs, so the snapshot is discarded and the computer cold
boots, F19). No other computer's launch changes.

### 9.3 The job path

1. A cell declares a job's memory need: `choir.Start(argv,
   {"memory_mib": 768})`. The need is part of the job spec
   (`capsulejob.Spec.MemoryMiB`, 1 to 4096) and is validated again at the
   reducer.
2. Before the job starts, the executor calls the admission hook. The guest
   runtime installs one (`autoputer/job_memory.go`) that adds the need to
   the computer's lease through `vmctl.GuestMemoryLeases`: one lease per
   computer, target = base + the sum of live needs, renewed at half its
   TTL while any need is live, waiting up to 60 s for the plug and then up
   to 10 s for the guest's `MemTotal` to show it.
3. The executor then admits the need against the **live** capsule budget
   and raises that capsule's cgroup `memory.max` by the need. The budget is
   3/4 of the guest's current `MemTotal`, read at every admission instead
   of once at start (F18); `CHOIR_CAPSULE_MEMORY_TOTAL_BYTES` still pins it.
4. When the job ends (done, failed, timed out, stopped, or its capsule
   ended), the executor lowers the cgroup first, then the need leaves the
   lease: the target steps down to base + the remaining needs, and at none
   the lease is released.

The lease is a raise of the budget, not a precondition: on a computer
without hotplug the need is admitted against the budget it already has.

### 9.4 Failure modes (written before the code)

| # | failure | handling | test |
|---|---|---|---|
| J1 | hotplug off, computer not selected, no device, driver unbound, or no vmctl URL | "unavailable" (404, 409, no client): the job is admitted against the budget the guest already has, and refused only if it does not fit; the reason is logged | autoputer: unavailable lease falls back; capsule: budget refusal |
| J2 | grow refused (host PSI, MemAvailable short, sample missing, over the per-computer max) | the job does not start; the refusal names vmctl's reason; nothing is boosted | autoputer: refusal refuses the job |
| J3 | plug incomplete in the wait (slow or partial guest) | the lease stays; the live budget decides; a job that does not fit is refused and its need leaves the lease | capsule: partial plug refuses; lease released |
| J4 | `MemTotal` lags `plugged` (onlining after plug) | the hook polls `MemTotal` up to 10 s for the plugged growth | autoputer: waits for MemTotal |
| J5 | two memory jobs at once (two capsules) | one lease per computer with the summed target; one job's end steps the target down, it does not release the other's memory | vmctl: summed target, step down, release at zero |
| J6 | the lease is released while the job's pages are still charged to the capsule | the cgroup is lowered before the need leaves the lease | capsule: lower before release (order test) |
| J7 | lowering `memory.max` below usage would OOM-kill inside the capsule (its broker) | first `memory.reclaim` the excess; if usage is still above the base, the boost is **retained** (accounted, cgroup kept, need kept in the lease) until the capsule is destroyed | capsule: retained boost returned at destroy |
| J8 | the capsule ends while its job is boosted | the destroy path returns every boost to the budget and releases its needs; the job watcher's later end is a no-op | capsule: destroy returns boosts once |
| J9 | the release is lost (vmctl unreachable) | logged; the lease ends at its TTL because nothing renews a need that left | vmctl: no renewal after the last release |
| J10 | the guest process restarts mid-job | jobs close as interrupted (restart rule); the new process releases any lease at start, so the budget never counts memory from before the restart for long | autoputer: release at start |
| J11 | vmctl restarts mid-job (its leases end, the orphan scan shrinks) | the next renewal is a fresh grow, admitted again; between them the guest may be asked to unplug memory the job uses and cannot (it keeps it: unplug needs free movable memory). Residual | residual |
| J12 | job memory and capsule spawns race on the budget | both are admitted under the executor's lock against one live reading | capsule: spawn sees the boost |
| J13 | declared need out of range | spec validation refuses < 0 or > 4096 MiB at the cell and the reducer | capsulejob, yaegikernel: bounds |
| J14 | `GOMEMLIMIT` and other `MemTotal`-derived limits in the guest daemon (`internal/gomemlimit`) stay at their boot value | the daemon does not run the job; leases are for capsule jobs. Residual | residual |

### 9.5 The probe uses the memory

Plugged memory the guest never touches never enters the firecracker
process's RSS, so "RSS returns within 128 MiB" after a host-only grow and
shrink proves nothing. The probe keeps the host grow and shrink as a
device check (plug, guest sees it, unplug to 0), then runs the real check:
a prompt-bar run asks engineering to start a capsule job with
`memory_mib` 1024 that allocates and touches 768 MiB and holds it for two
minutes. The probe samples while the job runs and passes only if:

- vmctl shows a **guest** lease and plugged memory ≥ 768 MiB;
- the firecracker RSS rose by at least 600 MiB over the last sample before
  the lease (the guest used the memory);
- after the job, plugged returns to 0 and the RSS returns within 128 MiB
  of that pre-lease sample within 2 minutes.
