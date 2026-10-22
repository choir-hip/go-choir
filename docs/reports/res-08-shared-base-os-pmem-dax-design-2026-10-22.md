# RES-08: Sharing the Base OS in Guest RAM (virtio-pmem + DAX)

Design, threat analysis and measurement plan for the DEC-05 security panel.
Date: 2026-10-22. Station:
[choir-launch-fp10-res-08-2026-10-21.md](../definitions/choir-launch-fp10-res-08-2026-10-21.md).
Problem: [base-os-copied-into-every-guest-ram-2026-10-22.md](../problems/base-os-copied-into-every-guest-ram-2026-10-22.md).
Lever L10 in [vm-density-limits-2026-10-14.md](vm-density-limits-2026-10-14.md).

Nothing here was run on Node B or staging. Code facts cite `d1c60ad87`.
Firecracker facts quote `docs/pmem.md` on Firecracker main (read
2026-10-22); each one marked **verify** must be checked on 1.16.1 before
the build slice. Everything else is reasoning, and is labelled as such.

## 0. Summary for the panel

- **What the change does.** Today every guest copies the base-OS pages it
  uses into its own RAM. With pmem + DAX, every guest would map **the same
  host physical pages** (the host page cache of one image file) straight
  into its address space and execute from them.
- **What it buys.** This is unmeasured. The hypothesis is 0.3 to 0.45 GiB
  per idle guest. The guest struct-page array for the pmem region costs
  about 1.6% of the uncompressed image per guest (about 96 MiB for a
  6 GiB image), and that comes off the saving. §5 gives the read that
  settles the number; it now exists (`/internal/debug/pprof/procmem?pagecache=1`,
  also served alone as `/internal/debug/pprof/pagecache`; owner notes §40).
- **What it opens.** The new channel is cache-line Flush+Reload on shared
  code and data across guests (§3, C1). It is not a privilege break. What
  it leaks is a victim's execution and data-access pattern inside shared
  binaries: activity reliably, and secrets only through non-constant-time
  code. It is also a high-bandwidth covert channel between colluding
  guests, which bypasses per-guest egress policy.
- **Recommendation** (for the panel to accept or reject):
  1. Measure first (§5). If the net saving is under 64 MiB per idle
     guest, close RES-08 and take L5 (guest page-cache reclaim) instead.
  2. If the saving is worth it, share **within a trust domain only**: one
     image inode per (owner, trust class) per release (§4, O2). Never use
     one inode across tenants (O1). Use the same domain key as the
     core-scheduling cookie (DEC-06).
  3. Ship the build behind a selector that defaults to off and starts with
     `class:test`. Its owner is FP8's vmmanager and image lane, after FP8
     lands.
- **DEC-05 scope.** The same reasoning binds ARCH-11 (shared snapshot
  templates). A `MAP_PRIVATE` restore from one memory file shares clean
  pages across the restored VMs, which gives the same C1 channel.

## 1. Today and the proposal

| | today (virtio-blk, lz4 EROFS) | proposal (virtio-pmem, uncompressed EROFS, `-o dax`) |
|---|---|---|
| host | one image file; Firecracker `pread`s blocks; one host page-cache copy (per inode; the jailer makes one inode per VM: [jailer problem](../problems/jailer-copies-store-image-per-vm-2026-10-21.md)) | the same file `mmap`ed `MAP_SHARED` by each Firecracker (pmem doc: "backed by a file with `MAP_SHARED` mapping type"); the host page cache pages *are* the guest's pmem |
| guest | a private copy of each block, decompressed into guest page cache; code runs from the copy | no guest page cache for the store; loads and instruction fetches go straight to the shared host pages through EPT |
| physical pages shared across guests | **none** (each guest has its own copy) | **every hot base-OS page** |
| guest write | the drive is `is_read_only` (`manager.go:1737`) | `read_only: true`: "can lead to VM shutting down on any attempt to write" (pmem doc) |
| image | 2.9 GB lz4 | uncompressed. DAX needs it, and so does a 4 KiB block size. The store's apparent size is about 2 to 3 times larger (the guest read reports it as `store.bytes`) |
| guest RAM cost | page cache, about 640 MiB idle (unsplit) | the struct-page array: 64 B per 4 KiB of image (**verify** whether virtio-pmem fsdax can put it on the device; virtio-pmem namespaces are raw, so assume RAM) |

## 2. Threat model

- **Attacker:** native code at guest root in computer A. This covers a
  compromised capsule, model-written code, a malicious stranger, or the
  public-facing Autopaper computer after it ingests hostile content.
  Assume the attacker has `clflush`, `rdtsc` (KVM does not trap it by
  default) and a counting thread for timing.
- **Victim:** computer B on the same host, running the same release, so
  the same image.
- **Assets:** B's activity (what runs, and when); B's secrets that touch
  shared code (guest-core and verifier signer keys, gateway and session
  tokens, TLS keys, and content B processes through shared libraries such
  as ICU, parsers and Dolt); and the integrity of B's base OS.
- **Out of scope here:** VMM escape. The pmem device is a small additional
  Firecracker surface, already counted in the isolation review.

## 3. Channels: baseline compared with the proposal

| # | channel | today | with global DAX (O1) | with per-domain DAX (O2) |
|---|---|---|---|---|
| C1 | **Flush+Reload** on shared lines: flush a line of a shared binary, wait, time a reload. A fast reload means the victim touched that 64 B line | absent: no shared physical pages between guests | **present, cross-tenant**: 64 B granularity, sub-µs; works cross-core through the coherent cache hierarchy (Intel inclusive LLC; assume it works on Zen too) | within a domain only |
| C2 | page-residency timing (Evict+Reload at page granularity; the "page cache attacks" class) | **present, coarse**: `O_DIRECT` reads of the store device time a host page-cache hit against NVMe. That reveals whether any guest touched a 4 KiB block since the host last evicted it. A guest cannot evict the host cache cheaply | the same, plus timing of EPT faults on first touch; about the same as today | within a domain; host-cache residency stays per inode, so it is per domain too |
| C3 | covert channel between colluding guests (one compromised guest with egress, one without) | present at low bandwidth: LLC Prime+Probe, contention, and SMT channels while SMT is on | high bandwidth (Flush+Reload covert channels run at hundreds of KB/s or more) | within a domain |
| C4 | integrity of the shared image | the host file is root-owned and read-only; guests get copies | the guest cannot write it (`read_only`; **verify** that Firecracker opens `O_RDONLY` with `PROT_READ` and a read-only KVM memslot, so a VMM bug cannot write either) | same |
| C5 | Rowhammer against shared pages (the Flip Feng Shui class) | a flip in a host page-cache page reaches guests on their next read; the attacker cannot see the page | the attacker can read and execute the shared page, so it can check a flip; one flip corrupts every guest's code until the page is evicted. ECC narrows this (FP0 reads EDAC) | within a domain |
| C6 | block-layer verification | none today (no dm-verity on the store) | DAX bypasses the guest block layer, so guest-side dm-verity is impossible on this path. That forecloses a future guest-side check; it does not regress anything that exists | same |
| C7 | image swapped under running VMs | a refresh resets the path for new launches (`manager.go:1692-1705`) | an in-place write or truncate of a mapped file would SIGBUS or corrupt every running guest. The image must be an immutable inode, which a nix store path is. Restore reopens the same path (pmem doc), so the path must outlive snapshots (L20) | same |

**What C1 leaks in practice** (reasoning, not measured):
- Activity is near certain. Which handler, desk worker or tool runs, and
  when, shows from which lines of the Go daemon (statically linked, in the
  store), the Dolt binary, node or ICU fire.
- Secrets leak only where shared code is not constant-time. Go's
  `crypto/ed25519`, `crypto/aes` (AES-NI) and x/crypto are written to be
  constant-time. Text processing is not: ICU and parser table lookups are
  input-dependent, so content B processes can leak in part. Nobody has
  audited every library on the path, so the panel should assume some
  leakage of B's content and of B's activity.
- **Ordering with SMT.** On Node B, SMT is on, without core scheduling,
  and the kernel reports "SMT vulnerable" for L1TF and MDS (isolation
  review §3.7). Those channels read *data*, not just access patterns. So
  until DEC-06 is closed, C1 is dominated by a worse channel that already
  exists. Once DEC-06 closes that channel for strangers, C1 would be the
  open one. This is why sharing has to follow the same domain as the
  core-scheduling cookie.

**No in-place mitigation for O1.** KVM has no VM-exit for `CLFLUSH`.
Eviction sets reproduce the flush without it, timing comes from a counting
thread, and cache partitioning (Intel CAT) does not separate *shared*
lines. The only real control is not sharing pages across trust domains.

## 4. Options

| option | saving (if §5 confirms the hypothesis) | cross-domain channel | notes |
|---|---|---|---|
| **O1** one image inode for every VM | the largest: one host copy | C1, C3 and C5 across tenants | the pmem doc says this is "not recommended". Reject while strangers or public-facing computers share a host |
| **O2** one inode per trust domain per release: domain = (owner, trust class), with `public_platform` and `critical_protected` computers in their own domain apart from the owner's private computers | (VMs in the domain - 1) copies per domain. Node B today is almost all the owner's, so close to O1 | none across domains | the per-domain inode is a reflink copy (cheap disk on btrfs, separate page cache) plus root-owned read-only hard links into each jail of that domain. That resolves the [jailer problem](../problems/jailer-copies-store-image-per-vm-2026-10-21.md) for the store the same way. The domain key is the core-scheduling cookie (isolation review §4.7) |
| **O3** no DAX; guest page-cache reclaim (L5) | the store *cache* goes, but mapped code stays per guest (guest `Mapped`). Re-reads hit the host cache and pay lz4 decompression | none new | lands without a panel. The value DAX adds on top of L5 is roughly guest `Mapped` minus the memmap cost |
| O4 split image (shared data, private code) | partial | still leaks through input-dependent data tables (ICU) | rejected: more complex for a weaker guarantee |

**Recommended:** O3 first if the measurement says the store cache is
mostly unmapped. O2 if mapped store pages are large. O1 never without a
new panel.

## 5. Measurements FP0 takes (read-only)

Prerequisite: the guest page-cache read (lane/fp10-res08-pmem-dax) is deployed,
and the guest has rebooted onto it.

**Privilege check first.** The read reports a file as *unmeasured* when
the daemon can neither own it nor use `CAP_FOWNER`. Linux 5.2+ `mincore`
would otherwise report it as 100% resident. If `store.unmeasured_files` is
not 0, the daemon lacks that capability in the guest, and the store number
is a floor, not a measurement. Record that and stop. Do not quote the
number. Today the daemon unit sets no `User` and no
capability bound (`nix/autoputer-vm.nix:810` onwards), so it runs as guest
root and the count should be 0. A later move to a non-root runtime would
make the store unmeasured, and the read would say so.

**Overlay check second** (review bunch 13, finding 2). A layered release
runs the daemon in a private namespace where `/nix/store` is an overlay:
the base EROFS is the lower layer, and the updater's private store
(`$CHOIR_UPDATER_ROOT/store` on `/mnt/persistent`) is the upper
(`nix/autoputer-vm.nix:216-224`). Overlay hands `mmap` to the real file, so
residency is right for each file. But files from the upper are the
release delta, not the base image, and they are also visible under
`/mnt/persistent`. The read therefore parses the topmost `/nix/store`
mount, counts upper files as the root `store_upper`, excludes the upper
and work directories from `persistent`, and reports `store_layering`. The
probe refuses to quote a store number (`store_quotable: false`, with the
reasons in `refusals`) when `store_layering` is missing (a read older than
53c91bc1f), when `partitioned` is false, when store files are unmeasured,
or when the store walk is partial. The DAX memmap cost uses the base
store's bytes only.

**Floors.** `skipped` (unreadable directories and files) and
`skipped_mounts` (mounts below a root, such as the systemd
`InaccessiblePaths` the daemon unit sets on the signer and credential
directories) make a root's total a lower bound even when `partial` is
false. The probe lists such roots in `floors`. A store floor still counts
as a measurement, because it can only understate the saving; a persistent
floor is expected (the signer directories are hidden from the daemon).

Run from FP0's machine (the script uses `ssh node-b`; it boots, stops,
evicts and writes nothing):

1. **Idle test computer, 30 min or more after a cold boot:**
   `scripts/res08_pagecache_probe.py --vm-id <test-vm> --label idle-30m --out scratchpad/res08`
2. **The same computer within 2 min of a cold boot** (boot touches most of
   the closure): `--label cold-boot`.
3. **A busy test computer** during a prompt-bar run with a research or
   engineering job: `--label busy-run`.
4. **Optional: the paper** (`critical_protected`, busy all the time),
   outside owner demo windows. The walk costs the guest about 1 to 2 s of
   one core: `--label paper`.
5. **Host with N guests running.** Every run includes the host read:
   distinct store-image inodes, each one's host page-cache residency
   (`fincore` as root), Firecracker `RssFile`, KSM, SMT, CPU vulnerability
   strings and EDAC. If the jailer is on (FP8), N jailed VMs should show as
   N inodes.

Equivalent manual reads, if the script cannot run:
- guest, on Node B:
  `curl -s -H 'X-Internal-Caller: true' -H @/var/lib/go-choir/secrets/internal-service-token.header 'http://<guest-ip>:8085/internal/debug/pprof/procmem?pagecache=1'`
  The guest IP is `ip=` in `/var/lib/go-choir/vm-state/<vm>/fc-config.json`
  `boot_args`.
- host, on Node B:
  `fincore --bytes $(readlink -f /var/lib/go-choir/guest/storedisk.erofs)`
  and `grep -E 'VmRSS|RssAnon|RssFile' /proc/<firecracker-pid>/status`.

**What to record per read** (the script's `summary`): `store_quotable`
and `refusals` first; then `cached_mib`, `mapped_mib`,
`store_resident_mib`, `store_upper_resident_mib`, `persistent_resident_mib`,
the per-root `examined_files`, `skipped`, `skipped_mounts`,
`unmeasured_files` and `partial`, and `floors`;
`store_apparent_mib` (approximately the uncompressed image),
`dax_memmap_cost_mib` and `net_saving_mib`; on the host, the resident bytes
of each image inode.

**Decision thresholds** (from the median of the idle reads):

| net saving per idle guest (`store_resident - memmap`) | and `mapped_mib` | decision |
|---|---|---|
| < 64 MiB | any | close RES-08 as measured, not worth a channel; take L5 |
| 64 to 256 MiB | < 64 MiB | L5 first (it removes most of it without sharing), then re-measure |
| ≥ 256 MiB, or `mapped_mib` ≥ 128 MiB | | take it to the panel with O2 as the proposal |

The busy and cold-boot reads bound the burst. They do not move the idle
decision.

## 6. Build plan after a panel verdict (not this lane: FP8's surfaces)

1. **Firecracker 1.16.1 checks (verify):** the `PUT /pmem/{id}` fields
   (`path_on_host`, `root_device: false`, `read_only: true`); the
   read-only mapping, and that a guest write ends only that VM; snapshot
   and restore with a pmem device (the same path at restore); coexistence
   with the virtio-mem hotplug region; the 2 MiB padding being private
   anonymous memory.
2. **Image (nix):** a second store image built uncompressed (empty
   `storeDiskErofsFlags`, 4 KiB blocks), next to the lz4 one during
   rollout. Cost: disk once per release per domain (reflinks), plus build
   time.
3. **Guest:** the initrd loads `virtio_pmem` and `nd_pmem`, then mounts
   `/dev/pmem0` read-only with `-o dax=always`. If there is no pmem
   device, it falls back to the block device by label, so one image serves
   both kinds of launch. Kernel config already has `VIRTIO_PMEM=m` and
   `FS_DAX=y` (density report §2).
4. **vmmanager:** with the selector on, a `pmem` entry replaces the
   `store` drive. The backing path is the domain's inode (O2). The launch
   config hash changes, so snapshots are invalidated once: use a
   cold-boot window or a planned-restart marker, never a boot-time
   resumer. Selector `VM_STORE_PMEM_DAX=off|class:test|domain`, default
   off.
5. **Proof:** the §5 read on a pmem test computer (`store_resident_mib`
   about 0, `MemTotal` minus the memmap), host `RssFile` shared across two
   test computers in one domain, and two computers in different domains
   holding different inodes.
6. **Rollback:** selector off. The next boot uses virtio-blk; no data
   migration (the store is read-only).

## 7. Questions for the panel

1. Is C1 an acceptable cost **within** one owner's computers, given that
   the owner's public-facing computers ingest hostile content? (The O2
   domain split proposes no.)
2. Is (owner, trust class) the right domain key, and must it equal the
   core-scheduling cookie?
3. Does DEC-05 cover ARCH-11's shared snapshot templates with the same
   rule (sharing only within a domain)?
4. Is the C5 Rowhammer delta acceptable on Node B? It depends on whether
   EDAC reports ECC (FP0's host read).
5. Should the build wait for DEC-06 (SMT and core scheduling)? Until DEC-06
   is closed, C1 is dominated by the L1TF and MDS channels, so the order
   does not change today's risk.

## 8. Residuals

- **The guard is literal only in enforce mode** (review bunch 13,
  finding 5). The read uses procmem's guard: `HostSourcedCaller`
  (`internal/server/host_caller.go`) accepts loopback and TEST-NET-1
  callers as well as the host peer, and `hostauth.Authorize` returns true
  in `ModeOff` (`internal/hostauth/hostauth.go`). So with hostauth off, an
  in-guest loopback caller can run the walk without the token. What it
  reveals is no more than a guest process can already read with its own
  `mincore`. The cost is one bounded walk (at most 30 s; about 1 to 2 s on
  today's store), so this is a small denial-of-service surface. It is
  pre-existing and the same for procmem and every pprof profile. It is not
  changed here: the fix is hostauth enforce mode, or `HostPeerCaller` for
  the whole pprof prefix, and it belongs to the hostauth owner.
- The mount-crossing count (`skipped_mounts`) has no unit test, because a
  test cannot mount without privileges. The guest read shows it.
- mountinfo option values are split on commas. An upper or work path that
  contains an escaped comma would be misparsed. Today's paths have none.
