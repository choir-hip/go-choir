# Playwright in Capsules: Memory, Placement and Visual Evidence for Self-Development

Date: 2026-10-15. Base: `main` at `e9d85a71d`.

Owner ask (2026-10-14, paraphrased): test computers should have Playwright
so they can take images and videos of their work, and humans or other
agents can review that visual evidence. Playwright is resource-intensive.
Is it workable on Node B's 32 GB while there are no human users yet? Could
it run in one real user VM? Can a VM's memory be ballooned up for
Playwright in a capsule, then shrunk when Playwright is off?

Mutation class of this report: **green**. It adds docs plus measurement
scripts and raw results under
[`docs/evidence/playwright-capsule-memory-2026-10-15/`](../evidence/playwright-capsule-memory-2026-10-15/).
It changes no runtime. Nothing was deployed, and no staging computer or host
setting was touched. Each proposed change below names its own class.

Sources:
- CLAUDE.md, including "Browser proof is specialized; do not assume every
  worker VM should carry Playwright/Chromium";
- [computer ontology](../computer-ontology.md);
- [VM density limits](vm-density-limits-2026-10-14.md);
- [computer CPU efficiency](../archive/reports/computer-cpu-efficiency-2026-10-14.md);
- [VM backend interface](vm-hypervisor-seam-design-2026-10-21.md);
- [isolation runtimes](../research/isolation-runtimes-2026-10-14.md). It is
  under `docs/research/`, not `docs/reports/`;
- [guest memory return](../archive/reports/lr-guest-memory-return-2026-10-14.md);
- the [backend browser learnings](../archive/backend-browser-substrate-learnings.md);
- the code at `e9d85a71d`:
  - `internal/vmmanager/balloon.go`;
  - `internal/vmctl/host_admission.go` and `pressure_sustained.go` (both
    landed in `b6f19a284`);
  - `internal/capsule` (seccomp, egress, cgroups, mounts, freeze);
  - `internal/autoputer/capsule_executor_linux.go`;
  - `internal/agentcore/engineering_assignment_runtime.go` (2 GiB, landed
    in `1a91176ac`);
  - `internal/browsercontrol`;
  - `internal/provider` (image blocks);
  - `internal/modelcatalog`;
  - `nix/node-b.nix`, `nix/autoputer-vm.nix` and `flake.nix`;
- Firecracker v1.15.1 at its tag: `docs/memory-hotplug.md`,
  `docs/ballooning.md` and `CHANGELOG.md`;
- Cloud Hypervisor `main`: `docs/memory.md` and `docs/hotplug.md`;
- cache.nixos.org narinfo for the pinned nixpkgs, and the Chrome for Testing
  bucket's object sizes;
- a local measurement, described in section 2.

Every number is marked **measured** (with where), **from docs**, or
**estimated**. Anything this pass could not confirm is marked
**UNVERIFIED**.

## The short answer

**It is feasible, and most of it already exists.** A browser can run inside a
test computer's engineering capsule, as a step after the build, within the
capsule's existing 2 GiB budget and one CPU. It needs no new memory
mechanism, no balloon and no host browser.

What a capture costs, measured in a cgroup shaped like that capsule:
- One viewport screenshot of Choir's own frontend peaks at about
  **0.4 GiB** (Playwright's headless shell) or **0.55 GiB** (full Chromium,
  which is what nixpkgs ships).
- It takes 2 to 3 CPU-seconds.
- It runs well under the capsule's 256-task limit.
- Video costs much more. Recording uses every CPU it is given (1.0 core at
  a one-CPU cap, 1.9 cores at two) and runs 2 to 3 times slower than real
  time.

**"Grow while used, shrink after" works, with one missing step.** The host
gives a guest memory as the guest touches it, so growth is automatic. Free
page reporting is already on for Node B
(`VM_BALLOON_FREE_PAGE_REPORTING=1`), and it returns memory the guest frees.
But after the browser exits, **0.19 to 0.43 GiB of its memory stays behind
as guest page cache**: the Chromium binary, fonts and the files it wrote.
The guest never frees that cache on its own, so reporting never returns it.
When the capsule's cgroup is removed, Linux moves that cache to the parent
cgroup instead of freeing it.

Measured: writing to the capsule cgroup's `memory.reclaim` before teardown
brings it back to baseline. So the slice needs one guest-side change:
**reclaim the capsule's cgroup before removing it**. Ballooning and
virtio-mem are the next step, for when the host needs a *guarantee* rather
than guest cooperation. Both work on Firecracker 1.15.1, and both need a
host-side controller that does not exist yet.

**On Node B's 32 GB:**
- **Two** browser-capable test computers can be busy at once safely, beside
  the paper and the owner's computer. **Three** is the edge.
- Run only **one video at a time**.
- Idle browser-capable computers cost nothing beyond their disk once they
  hibernate.
- One real user computer is workable for viewport captures once the
  test-computer slice has proved the return of memory. The owner's computer
  is 8 GiB with 4 vCPUs.

**Recommended placement: (a), a browser inside the computer's capsule.**
- It keeps a page exploit two isolation layers away from the host.
- It needs no egress, because the page under test is served on the
  capsule's own loopback.
- It binds the screenshot to the exact frozen bundle that Freeze names.

The other two placements:
- **(c)**, a dedicated browser-worker computer, is a later fleet option. It
  is the same placement on a computer that does nothing else.
- **(b)**, a host-side shared Chromium pool, is **rejected** while the
  pages are agent-written code. It would run untrusted JavaScript on the
  host.

**Minimal slice** (section 8):
- One test computer's engineering capsule serves its frozen candidate
  frontend on capsule loopback and takes viewport screenshots with
  Chromium.
- The capsule writes those screenshots, with a manifest, into the frozen
  bundle, and Freeze cites them.
- The capsule reclaims its cgroup on teardown.
- The acceptance probe reads the Firecracker process's host RSS through
  vmctl and requires that RSS to return within about 2 minutes of the
  browser exiting. "Return" means: to within 128 MiB of where it stood
  before the browser started.

## 1. Memory mechanics

### 1.1 What the host does with guest memory today

Firecracker backs guest RAM with a private anonymous mapping of
`mem_size_mib`. It is faulted in as the guest touches it.
- Test computers are interactive computers, so on Node B they boot at 8 GiB
  and 4 vCPUs (`VM_INTERACTIVE_MEM_MIB=8192`, `VM_INTERACTIVE_CPU_COUNT=4`
  in `nix/node-b.nix`; the code default is 2 GiB and 2 vCPUs,
  `interactiveVMMemSizeMib` in `internal/vmctl/ownership.go`). Platform
  computers boot at 4 GiB and 2 vCPUs (`platformVMMemSizeMib`).
  `VM_MEM_MIB=4096` and `VM_CPU_COUNT=2` are vmmanager's base config; vmctl's
  per-ownership shape (`machineShapeForOwnership`) overrides them.
  (Corrected 2026-10-22, RES-21: this section said test computers boot at
  4 GiB.)
- Both are in `nix/node-b.nix`.

**Growth is automatic:** host RSS rises as the browser touches memory. The
ceiling is the boot size. There is no swap: the vmctl unit has
`MemorySwapMax=0`, and the capsule cgroup sets swap to 0.

**Shrinking is not automatic.** The host gets a page back in only two
cases:
- the guest frees the page and free page reporting reports it;
- the VM stops or hibernates.

Free page reporting (`internal/vmmanager/balloon.go`; the balloon is
configured with `amount_mib: 0` and `free_page_reporting: true`) works like
this (from docs: Firecracker `docs/ballooning.md` and
[guest memory return](../archive/reports/lr-guest-memory-return-2026-10-14.md)):
- The guest kernel reports free blocks of at least `page_reporting_order`.
  That is the pageblock order, 2 MiB on x86_64.
- It reports about 2 seconds after the blocks become free.
- Firecracker then drops them with `MADV_DONTNEED`.
- Reporting must be enabled before boot. It runs continuously and cannot be
  stopped.

So "free page reporting plus a larger boot ceiling" gives "grow while used,
shrink after" **only for memory the guest actually frees**. For a browser
that memory falls into three kinds:

1. **Anonymous memory of the browser processes**: heaps, V8, compositor
   tiles and the screenshot bitmap. It is freed when the processes exit.
   - It returns to the host only if the freed pages coalesce into 2 MiB free
     blocks.
   - On a guest with mostly free memory, the buddy allocator usually
     re-forms large blocks.
   - Page-cache pages interleaved with them stop coalescing.
   - How much returns on a real guest is **UNVERIFIED**; the staging probe
     in section 8 measures it.
   - Lowering `page_reporting_order` (density lever L5) raises the share
     returned.
2. **Page cache**: the Chromium binary and libraries read from the store
   disk, fonts, the profile directory, and the screenshots and video
   written to disk.
   - The guest kernel keeps clean page cache until *guest* memory pressure
     evicts it.
   - That never happens on a lightly used guest. This is finding 2 of the
     density report: the paper computer filled 8 GiB with cache while
     reporting was on.
   - Reporting never sees these pages, because they are not free.
3. **Memory charged to a removed cgroup.** When a cgroup v2 memory cgroup
   is removed, its remaining page cache is reparented, not freed. Deleting
   the capsule's cgroup therefore does not release what the browser cached.

**Measured** (section 2): after `browser.close()`, the capsule-shaped
cgroup still held:
- 233 to 539 MiB in total;
- of which 187 to 432 MiB was file pages;
- of which only 41 to 80 MiB was anonymous, the measuring Node process
  itself.

One write of `4G` to the cgroup's `memory.reclaim` brought every run back to
its baseline of 43 to 95 MiB, with file pages at 0 to 2 MiB.

The pages reclaimed in the guest become free pages, and reporting then
returns them to the host. That host-side half is **UNVERIFIED** until the
staging probe. Locally there is no KVM, so only the guest-side half was
measured.

So the slice needs one guest change: the capsule executor writes the
capsule's `memory.reclaim`, then removes the cgroup.
- Class: **orange** (capsule lifecycle).
- Rollback: revert the call.
- No code writes `memory.reclaim` or `drop_caches` today. A grep over
  `internal` and `nix` finds none.

The guest kernel is 6.18. `memory.reclaim` exists from 5.19.

### 1.2 Raising the boot ceiling

A bigger `mem_size_mib` costs four things even while unused:

- **memmap.** The guest's `struct page` array is 64 bytes per 4 KiB page,
  about 1.6% of the ceiling. It is touched at boot and resident for the
  VM's whole life. That is about 64 MiB more per 4 GiB of ceiling (from
  docs: the Firecracker hotplug doc gives the same 64 bytes per page).
  - The density report measured 248 MiB of an 8 GiB guest that never shows
    in `MemTotal`.
- **An unbounded page cache.** The guest's cache grows toward the ceiling,
  so a bigger ceiling means a bigger idle footprint until lever L5 (guest
  cache reclaim) lands.
- **Snapshot writes and the snapshot budget.**
  - Full snapshots write the whole `mem_size_mib`.
  - Hibernate measured 7.6 to 8.1 s for 8 GiB, with PSI up to 2.5. 12 GiB
    would take about 11 to 12 s (estimated, linear).
  - The snapshot budget still counts logical size, because of the
    `st_blocks` fallback (density finding 1).
  - Sparse snapshots (L3) remove both costs: the write becomes proportional
    to resident pages.
- **No restore-time cost by itself.** The File memory backend maps the
  snapshot and faults pages lazily: load was 6 to 7 ms, and the hot set is
  0.2 to 0.4 GiB in the first 30 s. The ceiling affects restore only
  through what was resident when the snapshot was taken.
  - A browser's leftover page cache would be written into every later
    snapshot and read back on demand. That is another reason to reclaim at
    teardown.

**Conclusion:** the browser does not need a bigger ceiling for viewport
captures.
- A one-shot capture peaks at 0.4 to 0.55 GiB.
- It fits inside the 2 GiB engineering capsule after the build finishes.
  The build's link peaks at 1.50 GiB stripped, and the two run one after
  the other.
- A bigger ceiling becomes useful only for tall full-page captures
  (measured 1.5 to 1.7 GiB), parallel contexts (about 1 GiB) or video
  beside a build. All three should be refused by default (section 8).

### 1.3 Balloon: inflate and deflate

From docs (Firecracker v1.15.1 `docs/ballooning.md`):

- **The target.**
  - `amount_mib` is a target. Below it, the guest driver allocates pages
    into the balloon and hands them to the host, retrying every 0.2 s.
  - Above it, the guest frees balloon pages back.
  - `PATCH /balloon` changes the target at run time. Options other than the
    target are fixed once the device runs.
  - Firecracker does not control the speed: "this is dictated by the guest
    kernel driver."
- **`deflate_on_oom`** lets the guest take pages back instead of
  OOM-killing. It does not cover kernel allocations, requests larger than
  the balloon, or allocations refused under `vm.overcommit_memory=0`.
  Choir sets `deflate_on_oom: true`.
- **Cooperation.** The balloon is best effort and guest cooperative. With a
  compromised driver, the guarantees about target size "are **no longer
  guaranteed**". The host must be ready for the process to use every byte
  it was booted with.
- **Isolation** still holds: page frame numbers are validated, and memory is
  `MAP_PRIVATE|MAP_ANONYMOUS`.
- **Configuration.** The balloon must be configured before boot. Choir
  already does this. Stats must also be enabled before boot.

What "balloon up for Playwright" would mean:
- A balloon can only **take memory away** from a guest, then give it back,
  up to the boot ceiling. It cannot raise a guest above `mem_size_mib`.
- So "up for Playwright" means: boot at the larger ceiling, keep the
  balloon inflated by default (a test computer at 8 GiB with 4 GiB
  ballooned, so it effectively has 4 GiB), deflate it for a browser lease,
  and inflate it again afterwards.

The re-inflation has one real virtue. It forces the guest to drop page cache
to fill the balloon, so it is a host-driven squeeze that also returns the
browser's cache.

It has four costs:
- **The API socket.** Run-time `PATCH /balloon` needs the API socket. The
  socket exists only in snapshot hibernate mode (`--api-sock`; kill mode
  runs `--no-api`). Node B runs `VM_HIBERNATE_MODE=snapshot`, so the socket
  is there.
- **No host controller.** vmmanager's `fcapi.go` has no balloon calls today,
  and the backend seam marks run-time balloon resize as "no"
  ([VM backend interface](vm-hypervisor-seam-design-2026-10-21.md) section 4).
  New controller: **red** (VM lifecycle).
- **`MemTotal` would mislead the guest.** With `deflate_on_oom` on, Linux's
  `virtio_balloon` does not lower the guest's `MemTotal` when it inflates.
  It adjusts managed pages only when deflate-on-OOM is off. This is from
  memory of the driver, **UNVERIFIED on 6.18**. If it holds, every guest
  limit derived from `MemTotal` would see the ceiling, not the ballooned
  size. Two such limits read it once at start:
  - the daemon's soft Go memory limit, half of `MemTotal`
    (`internal/gomemlimit`);
  - the capsule admission budget, three quarters of `MemTotal`
    (`internal/autoputer/capsule_executor_linux.go:56`).
- **No guarantee.** The host still has no guarantee: a guest can refuse to
  inflate.

### 1.4 virtio-mem hotplug

**Firecracker 1.15.1 supports virtio-mem. Verified from its CHANGELOG:**
- 1.14.0, #5534: "Added support for memory hot-plugging through the
  `virtio-mem` device".
- 1.15.0, #5705: fixed diff-snapshot corruption for VMs with several memory
  slots, including hotplugged ones.
- 1.15.1, #5793: fixed plug and unplug skipping KVM slot updates for blocks
  not aligned to a slot.
- The CHANGELOG does not call virtio-mem a developer preview. It does call
  free page reporting one (1.14.0, #5491), although v1.15.1's
  `ballooning.md` no longer does.
- Node B runs Firecracker v1.15.1, read from the boot timeline's store path.

From docs (v1.15.1 `docs/memory-hotplug.md`):

- **Configuration.** It is configured before boot, by `PUT /hotplug/memory`
  or the config file's `memory-hotplug` block. The fields:
  - `total_size_mib`, the most that can ever be plugged;
  - `slot_size_mib`, default 128 MiB, the KVM slot and protection unit;
  - `block_size_mib`, default 2 MiB, the plug unit.
- **Run time.** `PATCH /hotplug/memory` sets `requested_size_mib`, and
  `GET` reports plugged against requested. Plugging is asynchronous: "the
  guest will incrementally plug memory until it reaches the target."
- **Unplug.** Once the guest reports a block unplugged, the host frees it
  immediately. But "unplugging requires the guest to cooperate and actually
  be able to find and report memory blocks." The host must poll `GET` and
  keep host limits such as cgroups.
- **Protection.** Memory that was never plugged stays out of KVM slots and
  is `mprotect`ed. A guest that touches it faults, which may crash
  Firecracker.
- **Guest requirements.** `CONFIG_VIRTIO_MEM`, kernel 5.16 or later on
  x86_64, and `memhp_default_state=online_movable` for reliable unplug. The
  guest kernel already builds `VIRTIO_MEM=m` and `MEMORY_HOTREMOVE=y`
  (density report).
- **Snapshots.** "Full and diff snapshots will include the unplugged areas
  as sparse holes." The hotplug configuration cannot be set on a
  snapshot-restored VM. Whether `PATCH` (changing the target) works after a
  restore is **UNVERIFIED**; the doc only forbids reconfiguring.
- **Not in the doc:**
  - whether virtio-mem needs `--enable-pci` or works over MMIO. Choir
    already launches with `--enable-pci` (`confinement.go:302-310`), so this
    does not block;
  - any interaction with the balloon;
  - plug or unplug timings. Plug and unplug latency is **UNVERIFIED**.

This is the clean version of the owner's idea, and it is lever L11 in the
density ledger:
- Boot a test computer at a base of 4 GiB plus a hotplug region of up to
  4 GiB.
- Plug 1 to 2 GiB for a browser lease.
- Reclaim the capsule.
- Unplug, and poll until `plugged == requested`.

Unlike the balloon:
- The guest's `MemTotal` really changes. The two limits that read it once at
  start still need to be told, by giving the lease its own capsule budget.
- memmap for the hotplug region is allocated only when it is plugged.
- The snapshot of an unplugged computer carries holes, not zeros.

What it needs:
- a hotplug controller in vmmanager, **red**;
- a guest boot parameter, **orange**;
- a lease protocol between guest and host, **red** (a new guest-to-host
  call).

It is the right second step, not the first.

### 1.5 Cloud Hypervisor

From docs (`docs/memory.md` and `docs/hotplug.md`, `main`; Choir pins
v52.0):
- **virtio-mem.** `--memory size=1024M,hotplug_size=8192M,hotplug_method=virtio-mem`.
  `hotplugged_size` plugs memory at boot.
- **Resize.** `ch-remote resize --memory <bytes>`, the `vm.resize` API, both
  expands and reduces a running VM. "Reducing RAM size might only partially
  work, as the guest might be using some of it."
- **ACPI hotplug**, the default method, cannot shrink until the next reboot.
- **Balloon.** Cloud Hypervisor also has a balloon with free page reporting
  and run-time resize.

Choir's Cloud Hypervisor backend wires cold boot, stop and metrics only. It
has no snapshot, no balloon resize and no virtio-mem
([VM backend interface](vm-hypervisor-seam-design-2026-10-21.md) section 4). It
is test-computer only behind `VM_BACKENDS_ENABLED`.

On Cloud Hypervisor, the lease design would be the same with `vm.resize` in
place of `PATCH /hotplug/memory`.

## 2. Chromium's real footprint

### 2.1 How it was measured

The measurement ran on the local machine. Docker Desktop on Apple silicon
runs a linux/arm64 VM with kernel 6.10.14 and 7.65 GiB.

The container:
- image `mcr.microsoft.com/playwright:v1.59.1-noble`, Chromium 147.0.7727.15;
- `playwright-core` 1.59.1, the same version as `frontend/` uses;
- `--memory=4g --cpus=2`, standing in for a 4 GiB, 2-vCPU computer (the size of a platform computer; Node B's test computers are 8 GiB, 4 vCPUs).

The setup:
- Each scenario ran in a **fresh cgroup v2 leaf shaped like an engineering
  capsule**:
  - `memory.max` 2 GiB and no swap;
  - `cpu.max` one CPU (`100000 100000`), as `engineeringAssignmentCPUQuota`
    sets;
  - `pids.max` 256.
- One rep of each scenario used a fresh cgroup with the page cache dropped
  first.
- The page served was:
  - Choir's built `frontend/dist`, from a local Node static server on
    container loopback, as a capsule would serve its candidate frontend;
  - a synthetic heavy page: 4,000 animated cards, six 1200×600 canvases,
    40 MB of JS arrays.
- **The sampler** read every 100 ms: the cgroup's `memory.current` and
  `memory.stat`, and the PSS and RSS of every Chromium and ffmpeg process.
- **The cgroup's `memory.peak`** is the headline.
- **After close**, the cgroup was read again, then written to
  `memory.reclaim` and read once more.

Each scenario ran 3 times (video at 2 CPUs: 2 times). Medians are given,
with the range in brackets. Raw JSONL, the scripts and the reproduce command
are in the evidence directory.

The figures below are **cgroup peak minus the 52 MiB baseline** of the
measuring Node process. That is the memory a capsule must hold for the
browser. All are measured, linux/arm64, at a one-CPU cap unless marked.

Viewport captures (1280×800):
- **One screenshot of the Choir frontend, Playwright's headless shell:**
  - 399 MiB (398 to 456);
  - anonymous 124 MiB, file pages 307 MiB;
  - Chromium PSS 348 MiB;
  - 6 processes, 71 tasks in the cgroup;
  - 1.8 CPU-seconds, 1.4 s to capture;
  - PNG 258 KiB.
- **The same capture with full Chromium in new-headless mode:**
  - 557 MiB (552 to 560);
  - anonymous 180 MiB;
  - 10 processes, 111 tasks;
  - 2.3 CPU-seconds.
  - This is the relevant figure for a Nix guest: nixpkgs has no
    `chromium-headless-shell` attribute, so the guest would run full
    Chromium headless. Whether nixpkgs' Playwright browser set maps the
    headless shell onto full Chromium is **UNVERIFIED**.
- **Five frontend routes in one page, in sequence:**
  - 431 MiB (427 to 446);
  - no growth over one capture;
  - 20.7 CPU-seconds over 21.6 s. The frontend's canvas animation keeps a
    renderer busy at about one core while the page is open.
- **The heavy page, viewport only:** 436 MiB (435 to 530), anonymous
  285 MiB.
- **Four contexts in parallel, two frontend and two heavy:** 952 MiB (950
  to 961), 117 tasks.

Full-page captures:
- **The heavy page, full page** (1280×34,888 px, read from the PNG):
  - 1,505 MiB (1,497 to 1,517);
  - anonymous 870 MiB;
  - a 6.2 MB PNG.
  - The full-page bitmap, not the DOM, is the cost.
- **Five pages, two of them heavy full-page:** 1,684 MiB (1,643 to 1,687),
  within 0.35 GiB of the 2 GiB cap.
- **Four contexts in parallel with two heavy full-page captures:** all 3
  reps **failed**. `page.screenshot` hit Playwright's 30 s timeout at one
  CPU. The cause (CPU starvation or memory reclaim at the cap) was not
  isolated.

Video (WebM VP8, 1280×720):
- **A 28 s scripted scroll of the frontend:**
  - 618 MiB (591 to 627) at one CPU, 663 MiB (603 to 723) at two;
  - 2.0 to 2.7 MB of video.
- **A 25 s scripted scroll of the heavy page:**
  - 815 MiB (815 to 844) at one CPU, 832 MiB at two;
  - 7.5 to 11.7 MB of video.

After `browser.close()`, every scenario left:
- 233 to 539 MiB charged to the cgroup;
- of which 169 to 432 MiB was file pages.

After one `memory.reclaim` write, every scenario was back at 43 to 95 MiB.
That is the Node baseline plus at most about 40 MiB, with file pages at 0
to 2 MiB.

### 2.2 What this says

- **A viewport screenshot is a 0.4 to 0.6 GiB event**, and a third of it or
  less is anonymous. The rest is page cache. That is why reclaim-on-teardown
  is the lever that matters.
- **Full-page screenshots of long pages and parallel contexts are the
  memory risks.** The slice should:
  - capture the viewport, or `clip` to at most four viewport heights;
  - use one context at a time.
- **Tasks:** 71 to 120 against a limit of 256. A browser and a running
  `go build` together would crowd it, which is one more reason to run them
  one after the other.
- **Arm64 numbers stand in for x86_64.** Node B is x86_64, a Xeon E-2236.
  Chromium's memory there is expected within about ±20% of these numbers
  (**estimated, UNVERIFIED**). The staging probe replaces them.
- **Choir's frontend should be captured with reduced motion.** It animates
  on load, and the first local screenshot caught the splash mid-transition.
  The capture should:
  - use `reducedMotion: 'reduce'`;
  - disable animations;
  - wait for a named ready element.

  This makes the image deterministic. It also cuts the about one core of
  CPU the animation burns.

### 2.3 Obscura is not a substitute (yet)

The guest already ships Obscura, a Rust and V8 headless browser with CDP
(`flake.nix` `obscuraPkg`, `nix/autoputer-vm.nix:787,836`).
`internal/browsercontrol` can ask it for `Page.captureScreenshot`. The live
test checks only that it returns a PNG of more than 1,000 bytes.

Whether Obscura lays out and paints real pixels is **UNVERIFIED**. If it is
a DOM and JS engine without a compositor, its screenshots are not visual
evidence of a Svelte UI. One local capture of the frontend with Obscura
would settle it. It was not run here.

### 2.4 Disk: Chromium in the Nix guest image, or fetched on demand

**On demand through the egress proxy does not work as is:**
- **The binary will not run.**
  - Playwright's x86_64 Linux builds are Chrome for Testing archives
    (measured object sizes):
    - `chrome-headless-shell-linux64.zip`, 117 MB;
    - `chrome-linux64.zip`, 179 MB.
  - Unpacked: 327 MiB and 612 MiB (measured, arm64 in the image).
  - They are linked against `/lib64/ld-linux-x86-64.so.2` and an FHS
    library set. They do not run on the NixOS guest without `nix-ld`,
    `patchelf` or an FHS wrapper (**UNVERIFIED** on the guest, but standard
    NixOS behaviour).
- **The hosts are not allowed.** The download hosts
  (`storage.googleapis.com`, `cdn.playwright.dev`) are not in the capsule's
  egress allowlist (`internal/capsule/egress.go` `ecosystemHosts`).
  Widening the list is a reviewed code change.
- **The download is paid again for every computer and every capsule, and it
  never shrinks.** Capsule state lives on `data.img`, and Firecracker 1.15
  virtio-blk has no discard, so the disk only grows (density finding 7).
  Keeping it outside the capsule, in the updater's private store, still
  lands on `data.img`.

**Nix in the guest store image is cheap per computer:**
- The pinned nixpkgs builds `chromium` 150.0.7871.46 (and Playwright driver
  1.60.0). From cache.nixos.org:
  - its closure is **1,774 MiB in 353 paths**;
  - `chromium-unwrapped` alone is 687 MiB as a NAR, a 205 MB zstd download.
- The guest already ships LibreOffice (`nix/autoputer-vm.nix:784`). Against
  LibreOffice's closure, Chromium adds **817 MiB in 59 paths**, of which
  `chromium-unwrapped` is 687 MiB.
  - Some of the rest, such as perl, gawk and procps, are probably in the
    guest already.
- The real delta against the whole guest closure is **UNVERIFIED** (not
  evaluated). Estimate: 0.7 to 0.8 GiB uncompressed, about 0.35 to 0.45 GB
  more in the lz4 EROFS store image (today 2.9 GB).

The store image is one read-only file on the host, shared by every
computer:
- **Disk:** the cost is that one file, plus deploy transfer and image build
  time.
- **RAM:** no computer pays anything until it launches the browser.
- **Host cache:** the pages it reads come through the host's shared page
  cache, so a second computer's launch is a host cache hit.

**Doctrine fit.** The CLAUDE.md rule is about capability, not bytes:
"do not assume every worker VM should carry Playwright/Chromium." The
binary can sit in the shared image while the *right to launch it* stays a
computer-level grant: a `browser` capsule capability given only to
browser-capable test computers. Three variants, in order:

- **N1 (recommended for the slice):** Chromium in the base store image,
  capability-gated. **Orange** (guest image). Rollback: drop it from
  `environment.systemPackages` or the capsule lower.
- **N2:** a separate read-only "browser" EROFS drive, attached only to
  browser-class computers. The base image stays lean. **Red**: a new VM
  device, and the drive order is contract (`vda` store, `vdb` data, then
  credential).
- **N3:** on demand from a host Nix substituter route, like the Go module
  proxy route. It lands in `data.img` per computer, permanently. Not
  recommended until virtio-blk discard (Firecracker 1.17, opt-in) or
  offline re-sparsifying (L8) exists.

**Driver versions must match.** Playwright's client and browser revision
must match. In the guest, use nixpkgs' `playwright-driver` (1.60.0) with
`PLAYWRIGHT_BROWSERS_PATH` pointed at `playwright-driver.browsers`, not an
npm-installed client. `frontend/` pins 1.59.1, so the guest and the CI
Playwright would differ by one minor version. The fallback for screenshots
alone needs no Node at all:
- `chromium --headless --screenshot=out.png --window-size=1280,800 URL`; or
- CDP from Go, which `internal/browsercontrol` already speaks.

### 2.5 CPU

All measured, one-CPU cap unless marked.
- **Screenshots are small.** 1.6 to 2.6 CPU-seconds for one viewport shot,
  launch included. 11 to 14 CPU-seconds for four in parallel.
- **The animated frontend costs while it is open.** About one core for as
  long as the page stays open, unless motion is reduced.
- **Video is CPU-bound.** Recording consumed 0.99 to 1.0 core at a one-CPU
  cap and 1.9 to 1.97 cores at two. Scripted sessions stretched:
  - at one CPU, the 28 s frontend session took 78 to 83 s, and the heavy
    session 61 to 98 s;
  - at two CPUs, 50 to 68 s and 70 to 75 s.
- So a 30 s video costs about 1.5 to 2.5 CPU-minutes, and it holds its
  capsule's whole CPU budget while it runs.
- **Node B will be slower.** The local CPU is Apple silicon. Node B's
  E-2236 cores are likely slower per core, so these are lower bounds
  (**estimated**).

## 3. Host budget on Node B's 32 GB

Counts only, from the [density report](vm-density-limits-2026-10-14.md)
(read 14:56 to 15:04 UTC on 2026-10-14) and the code at `e9d85a71d`. No
reading of Node B was taken for this report.

**The pool for test computers:**
- Node B has 31.1 GiB. The guest budget is about 22 GiB: 31.1, less 2.8 for
  services, about 1.6 for the kernel, and the 4.67 GiB floor
  (`VMCTL_PRESSURE_MIN_MEMORY_AVAILABLE_PERCENT=15` against
  `..._MIB=4096`).
- The paper computer holds about 7.85 GiB, its whole 8 GiB guest. The
  owner's computer holds 2.7 to 4.4 GiB.
- That leaves about **9.6 to 11.4 GiB** for everything else, less the
  measured-admission headroom of 1 GiB (`DefaultHostAdmissionConfig`).

**A browser-capable test computer during a change:**
- 1.21 GiB idle (cold boot);
- plus the engineering capsule, at most 2 GiB (the cgroup limit). The
  browser runs inside it after the build, not beside it;
- plus guest page cache, which today grows toward the 4 GiB ceiling.
- Typical about 3.2 GiB. Worst case about 4.2 GiB (the full guest plus
  memmap).

**Result:**
- **2 such computers busy at once safely** (9.6 / 4.2). **3 at the edge**
  (9.6 / 3.2).
- More if the paper is hibernated or after L5 bounds the guest cache.
- CPU agrees. Node B has 12 threads, and the platform fills to 70% (8.4):
  - The paper averages 1.2 cores plus about 0.18 of platform work, and the
    platform itself 1.5 to 2.
  - Each browser capsule is capped at one CPU.
  - Three busy test computers add 3 to 4.5 cores, which fits.
  - **Video is the exception:** a capsule given two CPUs uses both. Run one
    video at a time host-wide.

**One real user computer.** The owner's computer is protected and has an
8 GiB ceiling, of which 2.7 to 4.4 GiB is in use. Its capsule budget is
three quarters of `MemTotal`, about 5.8 GiB.
- A viewport capture adds 0.4 to 0.55 GiB to its RSS for a minute.
- **It is workable**, with three conditions:
  - only after the test-computer slice has shown memory returning;
  - only with reclaim-on-teardown landed. Without it, each run leaves about
    0.2 to 0.45 GiB of cache in a guest that never frees it;
  - only viewport captures.
- Running VMs grow outside admission, so a protected computer's spike is
  covered only by the floor and by hibernating idle others.

**How admission should treat a browser-active computer**
(`internal/vmctl/host_admission.go`):
- **Start need.** A start reserves the computer's last measured peak RSS
  (`MemoryPeakRSSBytes`), or 2 GiB when it is unmeasured.
- **The peak sticks.** The peak is a high-water mark for the VM's life. It
  resets only at the next start (`peakReset`). So a browser run raises the
  *next* admission's need for that computer. That is conservative and
  acceptable for test computers. Later, a lease could be recorded
  separately so it does not inflate wakes.
- **Correlated spikes.** Nothing reserves a running VM's growth. Until a
  lease exists, a browser run is a guest-local act the host does not see in
  advance, so the slice limits concurrency by policy:
  - one browser-enabled test computer running a browser at a time;
  - at most two browser-capable test computers awake.
- **Later, leases.** With leases (virtio-mem or balloon), a lease becomes a
  reservation in the same ledger: `need` is the lease size, and it is
  released when the unplug completes.

**How pressure reclaim should treat it**
(`internal/vmctl/pressure_sustained.go`, `pressure_reclaim.go`):
- **It is already exempt.** Reclaim hibernates only computers idle for at
  least `VMCTL_PRESSURE_RECLAIM_MIN_IDLE=30m`, one at a time, on a
  sustained signal: memory some avg60 > 5 and below the floor, 4 samples
  15 s apart. A computer running a browser is active, so it is not a
  candidate. Nothing needs to change for the slice.
- **The better act once leases exist:** under sustained pressure, ending a
  lease is better than hibernating a bystander. That means unplugging, or
  inflating, after asking the guest to stop the browser.
- **Hibernate** should never snapshot a computer mid-lease. Unplug or
  reclaim first, so the snapshot does not carry the browser's pages.

**Measurement stop rule:** every staging measurement in this plan stops at
host memory PSI some avg10 > 1.0. That is stricter than the reclaim
monitor's avg60 > 5 and matches the density benchmark's safety stops. The
other stops are the same: MemAvailable below floor + 2 GiB, or any reclaim
of a non-test computer.

## 4. Placement options

### (a) A browser capsule inside the computer (recommended)

**Security:**
- A page exploit in Chromium lands in the capsule:
  - its own mount, network, UTS, IPC and cgroup namespaces;
  - Landlock;
  - the default-deny seccomp filter (`internal/capsule/seccomp.go`);
  - cgroup limits.
- Escaping the capsule lands in the guest VM. That is two layers before the
  host.
- **Chromium's own sandbox is off.** Playwright launches Chromium with
  `--no-sandbox` unless `chromiumSandbox: true`. The capsule is the
  sandbox. Chromium's namespace sandbox would need user namespaces, and its
  seccomp layer would need the `seccomp` syscall, which the capsule filter
  does not admit.
- **No egress.** The page under test is the computer's own candidate
  frontend, served on the capsule's loopback, so the capsule needs **no
  egress**. A capsule without `Egress` set has only loopback.

**Measured seccomp fit** (local, Docker with a seccomp profile generated
from `capsuleAllowedSyscalls` plus the `socket` rule for `AF_UNIX`,
`AF_INET` and `AF_INET6`, default `EPERM`):
- Both a screenshot and a video recording **ran to completion** under the
  capsule's syscall allowlist, unchanged.
- An strace of the same runs found 8 syscalls used but not allowed:
  - `fallocate`;
  - `getpriority`, `setpriority`, `sched_getparam`, `sched_getscheduler`;
  - `io_uring_setup`, `io_uring_enter`;
  - `landlock_create_ruleset`.
- Every caller tolerated `EPERM`. The only trace in the logs was
  `pthread_getschedparam failed: 1`.
- The capsule's Landlock rules and mount layout were not reproduced:
  **UNVERIFIED**. The first staging run is the test.
- `/dev/shm` should not matter. Playwright 1.59.1 passes
  `--disable-dev-shm-usage` by default (`chromiumSwitches.js`), and
  `--no-sandbox` unless `chromiumSandbox: true` (`chromium.js:295`). So
  Chromium keeps its shared-memory files in `/tmp`, which in a capsule is a
  per-capsule directory on the state volume (`executor.go:2016-2030`).

**Memory** is charged to the computer's own VM, inside its existing ceiling
and capsule budget, and returns as section 1 describes.

**Evidence binding is strongest here.** The screenshot is taken in the same
capsule that produced the frozen release, of the frozen release's own
`frontend/` files. Freeze requires `frontend/` in every release
(`internal/capsule/executor.go:1612-1631`). The bundle digest that Freeze
cites covers both.

**Cost:**
- the Chromium closure in the image (N1);
- 0.4 to 0.6 GiB and a few CPU-seconds per capture.

**Limit.** A static serve of `release/frontend/` shows what renders without
the computer's backend, such as the landing and auth shell. Screens that
need live data need either:
- the candidate binary running in the capsule, which is heavy (Dolt store,
  boot replay); or
- Playwright route mocks: fixtures checked in with the frontend tests.

The slice starts with static and mocked routes.

### (b) A host-side shared browser service (rejected for now)

**Memory is the best of the three.**
- One Chromium serves every computer: one browser process plus about 50 to
  150 MiB per context (**estimated** from the four-context run: 952 MiB for
  4 contexts against 399 to 557 MiB for one).
- There is no per-guest page cache, and nothing to return.

**Security is the worst.**
- The pages would be agent-written JavaScript from a candidate change,
  executed in a Chromium on the host. Chromium's own sandbox (user
  namespaces) would be the only layer.
- The [isolation runtimes](../research/isolation-runtimes-2026-10-14.md)
  report names "fetchers, Chromium" as host-side tools that belong under
  gVisor and are red-team targets.
- The [security review](../archive/reports/computer-isolation-security-review-2026-10-14.md)
  already lists in-guest Obscura's missing address guard as a finding (F5).
  A host browser driven by guests is a larger version of the same SSRF
  surface.

**Reachability.**
- The host browser must reach a candidate frontend. Before apply, that
  frontend exists only inside a capsule. Exposing a capsule port outward is
  a new ingress path into the capsule.
- After apply, the served surface needs an owner session. The
  [backend browser learnings](../archive/backend-browser-substrate-learnings.md)
  already rule: "Do not claim Browser provides live VM view/control because
  a host CDP session can capture screenshots."

**Evidence binding:** a host capture is an independent witness of what a
*served* computer shows. That is valuable as post-apply verification, but it
is weak as evidence of the frozen candidate.

**Revisit (b) only** with Chromium under gVisor (`runsc`) and an owner-scoped
read credential, and then for post-apply witnesses, not for candidates.

### (c) A dedicated browser-worker test computer (later)

**Security:** VM isolation from the host, the same as (a). It needs no
egress either, if its input is a content-addressed bundle rather than a
URL.

**Memory:** about 1.2 GiB idle, plus the browser. It hibernates when idle
(0.34 to 0.52 s to wake), so it costs disk, not RAM, between uses. It
concentrates the browser cost on one computer, which would let every other
computer's image stay browser-free (N2's goal without a new drive).

**Authority:** computers do not call each other. Results cross only through
publication and adoption (CLAUDE.md, Safety). So (c) would take a frozen
bundle digest as input, through the host or as a published object, render
it, and return digests. That is a new cross-computer job path: **red**, and
not needed while only a few test computers want screenshots.

**Evidence binding:** good, because the input is the bundle digest. But the
capture is separate from the capsule that froze it.

**Verdict:** (a) now. (c) when many small computers want visual evidence and
the per-computer closure or the per-capture memory starts to matter. (b)
only behind gVisor, and only as a post-apply witness.

## 5. How visual evidence enters review

### 5.1 The record

Visual evidence should be bytes plus a manifest, inside the frozen bundle,
cited by Freeze.

**Where it lives.** Freeze already materializes a bundle under
`/mnt/persistent/choir-updater/incoming/<digest>/`:
- `release/` files, each with its sha256;
- `source.patch` and its digest.

Add an `evidence/visual/` sibling of `release/`:
- the materializer ignores it and adopters never install it;
- the bundle digest covers it.

**Each capture** is a PNG (or a WebM) named by its sha256, with one
`visual-evidence.json` manifest:

```json
{
  "kind": "visual_capture",
  "subject": {"source_tree_sha256": "...", "release_files_sha256": "..."},
  "captures": [{
    "artifact": "artifact:sha256:<hex>",
    "media_type": "image/png", "bytes": 263744,
    "route": "/", "viewport": [1280, 800], "full_page": false,
    "ready": "[data-testid=desktop-ready]", "reduced_motion": true,
    "taken_at": "2026-10-15T..Z"
  }],
  "browser": {"name": "chromium", "version": "150.0.7871.46", "store_path": "/nix/store/...", "driver": "playwright-driver 1.60.0"},
  "capsule": {"id": "...", "memory_peak_bytes": 0, "cpu_usage_usec": 0}
}
```

`artifact:sha256:` is the existing `computerevent.ArtifactRef` form
(`internal/computerevent/artifact_ref.go`).

**Settlement.** Engineering's rule-settled precommitments
(`internal/types/commitment_rules.go`: `tests_pass`, `freeze_within`,
`paths_confined`, `verify_pass`, `completes`) get one more kind,
`visual_captured`:
- It settles true at Freeze when the bundle carries a manifest whose
  `subject` equals the frozen source and release digests, with at least the
  routes the precommit named.
- Engineering precommits before the evidence, as the existing rules require.
- The runtime settles from receipts it holds: the capture is a receipt, and
  the judgement of *what the image shows* is not. That keeps the say–do
  record honest: a screenshot existing is not the screenshot being right.
  That is the archive's rule: "Do not treat ... screenshot presence as
  proof."
- Class: **orange** (precommitment rules and the freeze bundle).

**Video** is the same manifest with `media_type: video/webm`. It is opt-in
per change, capped at one 30 s clip, for the CPU and size reasons in
section 2.

**Retention.** The platform artifact store already grows 12 GB a day
([problem](../problems/artifact-store-grows-12gb-a-day-2026-10-14.md)). A
viewport PNG is about 250 KiB, so 20 captures per change is about 5 MB.
Visual evidence should:
- follow the bundle's own retention;
- not be copied into the platform store.

### 5.2 Viewing in Texture

Texture renders the commitment ledger into the owner's supervision surface,
and is the only writer of canonical document revisions. So the Freeze or
precommit row Texture renders shows the cited captures as thumbnails, with
the full image or video on click. Engineering never writes the document; it
only cites digests.

What is missing is a serving route. There is no digest-addressed media route
in the guest today; the Files app serves paths (`/api/files/{path}`). A new
owner-authenticated `GET` route would serve an evidence digest only when a
ledger record on this computer cites it. That keeps it from becoming a
general file read. **Orange.**

### 5.3 Agents scoring from images

**The plumbing exists in part:**
- `internal/provider` already carries image content blocks: base64, URL, or
  `artifact_ref`.
- But the gateway artifact resolver for `artifact_ref` is not built:
  `validateMediaRequest` refuses it.
- **No catalog model may receive an image.** The catalog marks its listed
  models text-only. The three Bedrock Claude entries (Haiku 4.5, Sonnet 4.6,
  Opus 4.6) carry no `Modalities` field, so `modelSupportsImage` returns
  false for them. The only image-capable match is a hard-coded `kimi-k2p6`
  name outside the catalog.

**The cheapest multimodal route in the catalog is Claude Haiku 4.5 on
Bedrock.** Its image input is a real upstream capability, but Choir's
catalog does not declare it. Enabling it needs two changes:
- `Modalities: ["text","image"]` on that entry (**orange**, model routing);
- the `artifact_ref` resolver at the gateway (**red**, gateway and provider
  calls), or base64 inline from the guest for the slice.

**Cost per image is UNVERIFIED here.** Anthropic counts image tokens by
size, which puts a 1280×800 capture at about 1,400 input tokens. That is
from memory, so it is UNVERIFIED, and price figures should come from the
pricing source, not from this report.

**How a reviewer should score:**
- **A rubric, not a vibe.** The reviewer gets the precommit's stated
  expectation, the before and after captures of the same route, and a fixed
  rubric. Does the named element appear? Is the layout broken? Does any
  text overflow? Is the change confined to the named area?
- **A recorded answer.** It answers with a probability and a short reason,
  recorded as a score record with scorer identity. Precommitment records:
  settlement is separate from scoring, and a committing desk never settles
  its own claims.
- **No video.** Models do not take video. For agent review, take a
  *filmstrip*: N screenshots at named steps, not a video. Keep video for
  humans.
- **Pixel diffs too.** Add a cheap deterministic check beside the model: a
  pixel diff against the parent release's capture of the same route, with a
  threshold. Report it as a number the scorer may cite.

## 6. What to build, in order

1. **S0 (green, this report):** the problem and design record.
2. **S1, the slice (section 8):**
   - Chromium in the guest image (N1), capability-gated;
   - a capture step in the engineering capsule;
   - reclaim on capsule teardown;
   - the evidence manifest in the freeze bundle;
   - current RSS in vmctl's VM view.

   Classes: orange throughout; the guest image change carries a red-adjacent
   deploy.
3. **S2:** Freeze cites the evidence, and the `visual_captured` rule. Texture
   thumbnails through the evidence route. Orange.
4. **S3:** a multimodal scorer: the Haiku 4.5 catalog modality and inline
   base64 first. Before and after diffs. Orange, then red for the resolver.
5. **S4:** browser leases by virtio-mem (L11) on test computers, with
   admission reservations and lease-ending under pressure. Red.
6. **Later:** placement (c), or N2, if many computers need captures.
   Placement (b) only behind gVisor.

## 7. Mutation classes and protected surfaces

- **This report and its evidence directory:** green.
- **Chromium in the guest image:** orange. It is the guest image, and it
  rides a platform deploy that rewrites guest artifacts, so it invalidates
  snapshots (S3 failure modes 7 and 8). Schedule it outside the owner's
  windows. Rollback: remove the package and redeploy.
- **The capsule capture step, teardown reclaim, and capsule receipts of
  `memory.peak`:** orange (capsule lifecycle). Rollback: revert.
- **Current RSS in vmctl's VM view:** orange (a read-only field on an
  existing API). Rollback: revert.
- **The `visual_captured` rule and the evidence route:** orange.
- **Model catalog modality:** orange (provider and model routing).
- **The gateway `artifact_ref` resolver:** red.
- **The balloon or virtio-mem controller and leases:** red (VM lifecycle,
  vmctl).

Heresy delta for this report:
- **discovered:**
  - capsule cgroups are removed with their page cache still charged, and it
    is reparented, not freed: a capsule's file pages outlive it in the guest;
  - every model in the catalog is text-only to Choir, so no agent can see an
    image today;
  - vmctl's VM view exposes only peak RSS, so "memory returned" cannot be
    read without SSH;
  - Obscura's screenshot is untested as pixels.
- **introduced:** none.
- **repaired:** none.

## 8. The minimal slice and its acceptance probe

**Scope:** one minted test computer (`WarmnessClassTest`), cold-booted with
reporting on. Never the paper, never the owner's computer.

### 8.1 The change

1. **Image (N1).** Add nixpkgs `chromium` and `playwright-driver` to the
   capsule's lower or the guest system. Export
   `PLAYWRIGHT_BROWSERS_PATH=${playwright-driver.browsers}`. No egress
   widening.
2. **Capture step.** The engineering overlay gains one named act,
   `choir.Capture` (or a documented script the desk runs), after the build
   and before Freeze. In the same engineering capsule (2 GiB, one CPU, no
   egress needed) it:
   - serves `release/frontend/` on `127.0.0.1:<port>` with a static server,
     with an SPA fallback to `index.html`;
   - launches Chromium headless at 1280×800 with `reducedMotion: 'reduce'`
     and one context;
   - captures a fixed route list, by default `/`, viewport only. It refuses
     full-page captures taller than four viewports;
   - writes the PNGs and `visual-evidence.json` into `evidence/visual/`;
   - closes the browser;
   - has a hard deadline (60 s) that kills the browser's process group on
     expiry.
3. **Teardown reclaim.** When a capsule ends, the executor:
   - writes a large value (the capsule's `MemoryMax`) to the capsule
     cgroup's `memory.reclaim`, ignoring `EAGAIN`;
   - records `memory.peak`, `memory.stat` file and anon, and
     `cpu.stat usage_usec` in the capsule receipt;
   - only then removes the cgroup.

   This applies to every capsule, not only browser ones, because builds
   leave cache too.
4. **Freeze cites it.** The bundle digest covers `evidence/visual/`, and
   the Freeze receipt lists the capture artifacts. The `visual_captured`
   rule is S2, so in S1 the citation is enough.
5. **Readable RSS.** vmctl's VM view gains `memory_rss_bytes` and
   `memory_rss_at`, from the pressure monitor's existing 15 s sample
   (`led.rss`, read through `VMResidentBytes`, which is VmRSS of the
   Firecracker process). Without it, the probe needs SSH, and the standing
   questions ask for no-SSH operability.

### 8.2 The acceptance probe

The probe is a script under `scripts/`, run against `https://choir.news`
after the landing loop.

1. **Preconditions.** Read Node B's host memory PSI and MemAvailable. Start
   only if some avg10 ≤ 1.0, MemAvailable ≥ floor + 2 GiB + 4 GiB, and no
   other browser-enabled computer is busy.
2. **Mint and settle.** Mint a test computer (cold boot). Wait until its
   `memory_rss_bytes` is stable within 32 MiB over 2 minutes: R_idle.
3. **Change.** Ask for one frontend-visible change: a label text in the
   desktop shell. Let engineering build, run the capture step, and Freeze.
4. **Sample.** From the browser step's start to 5 minutes after it ends,
   sample `memory_rss_bytes` every 15 s. Take the capsule receipt's
   `memory.peak`, and the PNG and manifest from the frozen bundle by digest.
5. **Stop rule.** Abort (and stop the test computer) on PSI some avg10
   > 1.0, MemAvailable < floor + 2 GiB, any reclaim of a non-test computer,
   or host CPU > 80% for 60 s.
6. **Pass if all of these hold:**
   1. **The image is the change.** The Freeze receipt cites at least one
      capture whose sha256 matches the PNG in the bundle, and whose
      manifest subject equals the frozen source and release digests. The
      PNG decodes at 1280×800 and shows the changed label. A human or the
      S3 scorer checks the label; in S1, a text check through the
      accessibility tree suffices.
   2. **Growth is real.** The capsule receipt's `memory.peak` minus the
      pre-browser cgroup level is at least 300 MiB. Host RSS rose during
      the step, so the measurement saw the browser.
   3. **Memory returns.** Within **N = 2 minutes** of the browser exiting,
      `memory_rss_bytes ≤ R_pre + 128 MiB`, where R_pre is the RSS sampled
      just before the browser step started (after the build). It also
      holds for the next 3 minutes. Two minutes leaves room for:
      - the 2 s reporting delay;
      - the 15 s RSS sample;
      - reclaim of up to 0.5 GiB.

      Whether 128 MiB of slack suffices is the probe's own question. Record
      the actual return. If it fails only on fragmentation, the next lever
      is `page_reporting_order` (L5).
   4. **No pressure or kills.** No PSI stop fired, and no `oom_kill`
      appears in the capsule's `memory.events`.
7. **Artifact.** One JSONL row per sample plus a summary, under
   `docs/evidence/playwright-capsule-slice-<date>/`. It records:
   - the deployed commit, computer and VM ids;
   - R_idle, R_pre, the peak and the return time;
   - the capture digests;
   - the capsule receipt.

   Stop the test computer at the end and record its id.

**Control.** Repeat once with teardown reclaim switched off by an
environment knob. The expectation is that host RSS stays 0.2 to 0.45 GiB
above R_pre. That shows the reclaim, not luck, made the return.

## 9. Residuals and UNVERIFIED items

- **Host-side return of reclaimed guest pages.** That reclaimed pages reach
  the host through free page reporting, and how fully after fragmentation,
  is UNVERIFIED until the staging probe. Locally there is no KVM.
- **Architecture and runtime.** All Chromium figures are linux/arm64 in
  Docker Desktop's kernel 6.10, with Chromium 147 from Playwright. The
  guest is x86_64, kernel 6.18, nixpkgs Chromium 150 in full headless mode.
  Expect the new-headless numbers (about 0.55 GiB), give or take 20%:
  UNVERIFIED.
- **Capsule fit beyond seccomp.** The capsule's Landlock rules and mount
  layout were not reproduced. Only the seccomp allowlist was.
- **The four-parallel full-page failure.** Its cause (a 30 s timeout at one
  CPU) was not isolated.
- **Image size.** The Chromium closure delta against the whole guest system
  closure was not evaluated. Against LibreOffice's closure it is 817 MiB.
- **Firecracker virtio-mem.** Plug and unplug latency, `PATCH` after
  snapshot restore, and any PCI or MMIO requirement are UNVERIFIED.
- **The balloon and `MemTotal`.** That `virtio_balloon` with
  deflate-on-OOM leaves `MemTotal` unchanged is from memory: UNVERIFIED on
  6.18.
- **Obscura.** Whether its CDP screenshot is a real raster is UNVERIFIED.
- **Model cost.** Per-image token counts and price for the multimodal
  scorer are UNVERIFIED.
- **Next realism axis.** The same probe with a screen that needs live data
  (mocked routes, then a candidate binary in the capsule). Then a second
  browser-capable computer at once, to measure correlated spikes on Node B.

## Reproduce the local measurement

Directory:
[`docs/evidence/playwright-capsule-memory-2026-10-15/`](../evidence/playwright-capsule-memory-2026-10-15/).

1. Into a scratch directory, copy that directory's files. Copy a built
   `frontend/dist` as `site/`, and put `heavy.html` into `site/`.
2. In the Playwright image, run `npm i playwright-core@1.59.1`.
3. Run the `docker run` line at the top of `run.sh`. Use
   `SCENARIOS="..."` to pick scenarios, and `CPU_MAX="200000 100000"` for
   two CPUs.
4. `python3 summarize.py results.jsonl` prints the medians used above.

The seccomp fit:
- build the profile with `python3 mkprofile.py <repo>/internal/capsule/seccomp.go profile.json ""`;
- run `seccomp-test.sh` in the same image with
  `--security-opt seccomp=profile.json`;
- `strace.sh` lists the syscalls behind `syscalls-*.txt`.
