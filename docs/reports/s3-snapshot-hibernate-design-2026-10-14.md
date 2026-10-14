# S3 Snapshot Hibernate: Design Note

Date: 2026-10-14. Owner request (2026-10-14, in session): real
hibernation, a Firecracker memory snapshot instead of a kill.
Mutation class: **red** (vmctl / VM lifecycle, Firecracker launch, guest
run admission, guest clock). Parent spec:
[S3 fast resume](../definitions/choir-appdev-s3-fast-resume-2026-10-01.md);
facts refreshed in the
[cost-track investigation](cost-track-sa-s3-sp-investigation-2026-10-14.md).
Why it matters: today a hibernate is a kill, so every wake is a crash boot
that closes the computer's work
([paper computer reclaimed](../problems/live-paper-computer-reclaimed-no-computer-tiers-2026-10-13.md):
14 schedules closed).

## What changes

- `VM_HIBERNATE_MODE=snapshot|kill` (code default `kill`; staging sets
  `snapshot` in `nix/node-b.nix`). In `kill` mode nothing changes, including
  the launch command line (`--no-api`).
- In `snapshot` mode a computer is launched with
  `--api-sock <state>/<vm>/api/fc.sock` (dir 0700) plus the same
  `--config-file`. Hibernate asks the guest to fence, pauses the VM, writes a
  full snapshot (`vmstate`, `mem`, 0600, dir 0700) and stops Firecracker.
- A wake first tries the snapshot: validate, mark consumed, load, tell the
  guest it resumed. Any refusal or failure falls back to today's cold boot.

## Compatibility finding (Firecracker 1.15.1)

Verified against the v1.15.1 tag of firecracker-microvm/firecracker:

- `--api-sock` with `--config-file` is a supported combination:
  `docs/getting-started.md` ("Configuring the microVM without sending API
  requests") shows `firecracker --api-sock /tmp/firecracker.socket
  --config-file <path>`; `src/firecracker/src/main.rs:377-433` runs
  `run_with_api(vmm_config_json, ...)` whenever `--no-api` is absent. The
  default socket is `/run/firecracker.socket` (`main.rs:42`), so every VM
  must pass its own path.
- Snapshots are supported with the PCI transport. Snapshots are GA since
  1.15.0 (CHANGELOG #5165); PCI arrived in 1.15.0 (#5364). The device
  state carries `pci_enabled` and every PCI virtio device, the balloon
  included: `src/vmm/src/device_manager/pci_mngr.rs:238-326` (save) and
  `:428-640` (restore re-attaches the PCI segment iff the saved state had
  it, then restores balloon, block, net, vsock, entropy, pmem, mem). The
  `--enable-pci` flag only drives the boot path (`builder.rs:205`), so
  passing it to the restoring process is harmless; we pass it anyway to keep
  the command lines identical. No device-model change is needed.
- The balloon device (being added concurrently) is saved and restored by
  the same code (`pci_mngr.rs:304-313`, `:440-460`).
- Other facts the design depends on (`docs/snapshotting/snapshot-support.md`):
  the memory file is mapped `MAP_PRIVATE` for the resumed VM's whole life;
  load requires the same TAP name and the same drive paths; a failed load
  ends the Firecracker process; the guest wall clock continues from the
  snapshot moment and must be stepped guest-side; resuming one snapshot
  more than once is insecure (identifiers, RNG state, tokens), resuming it
  exactly once is the documented secure usage; snapshots must be resumed on
  identical software (we pin Firecracker version and host kernel release).
- 1.15.1 #5809: on hosts >= 5.16 the guest monotonic clock no longer jumps
  by the elapsed wall time on restore (opt-in `clock_realtime`). We do not
  set it: guest wall time is stepped by the guest itself.
- Node B (read-only, 2026-10-14): Firecracker v1.15.1, host kernel 6.18.16,
  swap file 16 GiB present, THP never, root and state dir on btrfs
  `compress=zstd`. Guest kernel configs (6.18.37, 6.18.45): `KVM_GUEST=y`,
  `ACPI=y`, `PCI=y`, `VIRTIO_PCI=m`, `VMGENID=m`, `VIRTIO_BALLOON=m`.

## Failure list (written before the code)

Each item names how the design answers it.

1. **Work frozen mid-flight.** A run, a cell, or a model call is in progress
   when memory is captured; its outbound connections die. Answer: the guest
   fence closes run activation and checks running runs and mid-transaction
   self-development operations atomically under the same lock that admits
   runs; busy answers 409 and reopens admission; vmmanager then uses the kill
   path (today's behavior).
2. **Race: new work admitted between the check and the pause.** Answer: the
   gate stays closed from the check until `resumed` or `abort` (or a
   2-minute monotonic TTL, which a paused guest does not advance). A run
   that tries to start waits on the gate; it never starts and is never
   frozen mid-run.
3. **Old guest without the endpoint (404), unreachable guest, timeout.**
   Answer: kill path, reason recorded.
4. **Snapshot create fails** (disk full, I/O error, Firecracker error).
   Answer: kill path; partial files deleted; vmctl records hibernated as
   today (it already records hibernated whatever the manager returns).
5. **Capture interrupted** (vmctl or host dies mid-write). Answer: files are
   written under `snapshot.partial/`; only a complete set with metadata is
   renamed to `snapshot/` (fsync files, then rename, then fsync the parent).
   `snapshot.partial/` is never loaded and is deleted by the next boot or
   hibernate.
6. **Second load of the same snapshot** (resume twice, concurrent wakes, a
   crash mid-resume followed by another wake). Answer: per-VM operation lock
   serializes wakes; metadata `consumed=true` is written and fsynced before
   Firecracker starts; a consumed snapshot is discarded, never loaded.
7. **Disk changed after capture** (a cold boot, recover, refresh, cold
   recovery quarantine, manual repair). Answer: identities (path, size,
   mtime_ns, inode, dev) of every writable drive (`data.img`,
   `credential.img`) are taken after Firecracker stops and checked before
   load; any cold boot also discards the snapshot first.
8. **Platform changed after capture** (deploy: kernel, initrd, store disk,
   kernel params, machine shape, balloon config). Answer: the manager
   rebuilds the Firecracker config a cold boot of the same realization would
   use now (current deploy defaults, vmctl's current shape and hold) and
   compares its sha256 with the launch config's. Mismatch: cold boot onto
   the new platform.
9. **Hypervisor or host kernel changed.** Answer: Firecracker `--version`
   and `uname -r` recorded and compared.
10. **Guest wall clock stale after resume.** Breaks capability expiry
    checks, timestamps, TLS. Answer: `resumed` carries the host's wall time;
    the guest steps its clock (`settimeofday`; the runtime unit runs as root
    with no capability bounding set) and reports the step.
11. **Platform event capability expired while hibernated** (new finding).
    The guest's corpusd capability lives 30 min and is renewable only up to
    15 min past expiry (`internal/platform/event_capability.go:16-17`); the
    renewal needs the current token. A guest resumed later cannot append to
    its tape. Answer (this slice): prepare forces a renewal and reports the
    expiry; metadata carries `resume_deadline = expiry + 15 min - 3 min`;
    after it the snapshot is discarded and the computer cold boots. The
    `resumed` hook renews immediately and fails the resume (host kills,
    cold boots) if renewal is refused. Next slice: re-credential on resume
    (a fresh same-realization envelope delivered in `resumed`), which lifts
    the window. Named residual R1.
12. **Gateway token.** Not rotated by a snapshot resume (only a cold boot
    issues one; `GATEWAY_AUTOPUTER_TOKEN_TTL=720h` on staging). vmctl's
    existing post-resume `ensureExistingGatewayCredential` re-imports the
    token after a gateway restart. Snapshot resume must not issue a new
    gateway token or reserve a new epoch: it returns the snapshot's epoch.
13. **Stale TCP connections** in the guest's HTTP pools. Answer: `resumed`
    closes idle connections of the default transport (hostlink clients use
    it). Live-event websockets reconnect from the browser.
14. **RNG / unique state duplicated.** Answer: one-shot consumption (FC's
    "secure usage" example 1). VMGenID reseeds the kernel CRNG if the
    `vmgenid` module is loaded (it is `=m`): verify on staging (residual R3).
15. **Network identity.** The snapshot has the guest IP, MAC and TAP name in
    memory/device state. Answer: hibernate keeps the host port reserved; resume
    reuses the same port/subnet/TAP; if the subnet is held by a live
    interface, fall back to a cold boot.
16. **Host memory: a full snapshot faults in all guest memory.** Reading
    untouched anonymous pages maps the zero page, so RSS grows little, but
    Firecracker writes the full `mem_size_mib` file through the page cache.
    Answer: disk budget below; btrfs zstd compresses zero runs on disk.
17. **Host disk.** An 8 GiB guest writes an 8 GiB memory file. Answer:
    `VM_SNAPSHOT_BUDGET_GIB` (default 48) counts logical bytes of committed
    snapshots; least recently created snapshots are evicted first (their
    computers cold-wake later); a statfs floor (16 GiB free after the write)
    also applies; no room means the kill path.
18. **Memory file still mapped after resume.** Deleting it is safe (the
    mapping holds the inode) but its blocks stay allocated until that
    Firecracker exits. Answer: delete after a successful resume; the statfs
    floor covers the unlinked-but-held space.
19. **Busy hibernate holds vmctl's registry lock** for seconds of snapshot
    write, stalling every route. Answer: vmctl marks the computer with the
    existing in-progress guard and calls the manager outside the lock.
20. **Computer launched before the switch** (`--no-api`, no socket), or
    reattached after a vmctl restart. Answer: hibernate probes the socket;
    absent means the kill path. Reattached computers keep their socket.
21. **Snapshot of a held computer, or hold changed.** Answer: vmctl never
    tries a snapshot for a held computer; `MaintenanceHold` is part of the
    rebuilt config, so a hold flip is a mismatch.
22. **API socket path too long** (108 bytes). Answer: such a VM launches as
    today (`--no-api`) and hibernates by kill.
23. **Load fails or the guest never answers `resumed`/health.** Answer:
    kill Firecracker, discard, cold boot (a crash boot, see below).
24. **Rollback while snapshots exist.** Answer: kill mode never loads; the
    next cold boot discards any snapshot; the files are deleted by
    `DestroyVMState` with the state dir.
25. **Guest frozen with admission closed forever** (host dies after
    prepare, before snapshot). Answer: the host kills the guest in that case
    (kill path), and if the host process itself died the TTL reopens the gate.

## Restart rule

A snapshot resume is not a boot: the guest process never stopped, so no
restart-closing pass runs; the fence guaranteed nothing was in flight, so
nothing is lost. A fallback cold boot is a **crash boot**: hibernate writes
no planned-restart marker, so the boot closes open work exactly as today.
Standing intents (source schedules) survive a snapshot resume because the
guest's timers resume with it. Not done here: writing a marker during the
fence so that a fallback cold boot could resume — it would need a clear on
successful resume and owner confirmation (the cost-track note flags this as
an owner ruling when S3 opens).

## Decisions

- **D1 PCI**: keep `--enable-pci` for launch and restore (finding above).
- **D2 API socket only in snapshot mode**: the env switch is a full rollback.
- **D3 Full snapshots only**: Diff snapshots without dirty tracking use
  `mincore`, which misses swapped pages, and Node B has swap. Diff is
  developer preview.
- **D4 No fsfreeze**: memory and disk are captured as a pair while the VM is
  paused and the disk is not written until the paired load; `sync` in the
  fence protects the kill fallback. The S3 spec's reflink/fsfreeze pairing is
  deferred to cross-host or export work.
- **D5 Failure after the fence is a kill**, never a resume of the running VM,
  because vmctl records hibernated regardless.
- **D6 Metadata lives next to the snapshot**, not in vmctl's ownership file:
  the manager is the one loader, and the per-VM operation lock serializes it.
- **D7 Timeline**: hibernate writes `hibernate-timeline.json`
  (`boot_kind=hibernate_snapshot|hibernate_kill`, fence/pause/snapshot ms,
  bytes, reason). A wake records `boot_kind=snapshot_resume` with load ms,
  wake-to-healthy ms and outcome; a fallback cold boot records
  `snapshot_fallback` with the reason.

## Protected surfaces

Firecracker launch command (snapshot mode only), vmmanager hibernate and
wake, vmctl hibernate locking and wake path, guest run activation (gate),
guest wall clock (step), guest capability renewal timing (forced renew),
snapshot files under the VM state dir.

## Rollback

Set `VM_HIBERNATE_MODE=kill` (one line in `nix/node-b.nix`) and redeploy:
launches go back to `--no-api`, hibernate kills, wakes never load. Or revert
the commits. Existing snapshot files are discarded by the next cold boot of
their computer.

## Heresy delta

- discovered: the platform event capability (30 min + 15 min grace) bounds
  any memory-preserving hibernate; hibernate held vmctl's registry lock
  across the manager call.
- introduced: none intended; R1 (window) is a stated bound, not a silent one.
- repaired: hibernate is no longer always a destructive kill; the S3 spec's
  missing fence now exists for this path.

## Residuals

- **R1** credential window (item 11): re-credential on resume.
- **R2** snapshot space does not yet feed the pressure-reclaim decision;
  the budget bounds it instead.
- **R3** VMGenID module loaded in the guest: check on staging
  (`ls /sys/bus/acpi/drivers/vmgenid` in the guest console log or procmem).
- **R4** wake-to-healthy p50/p95 targets (S3 spec: < 1 s / < 3 s) need a run
  set on staging; a cold-path trim is out of scope.
- **R5** live-event websocket and model-stream reconnection after resume is
  observed, not engineered.

## Staging evidence plan (lead runs it)

1. Deploy; confirm `VM_HIBERNATE_MODE=snapshot` in the vmctl unit and the
   deploy SHA in health.
2. Mint a test computer; wait healthy; refresh it once so it launches with
   the API socket (boot timeline `firecracker_spawned` detail shows
   `api_sock=true`).
3. Create a file and open a Texture document; note the guest's
   `/api/runtime/obligations` and source schedules (if any).
4. `POST /internal/vmctl/hibernate {"computer_id": ...}`; read
   `GET /internal/vmctl/boot-timeline?computer_id=...&kind=hibernate`:
   expect `boot_kind=hibernate_snapshot`, fence and snapshot ms, bytes.
   Check the Firecracker process is gone and host `MemAvailable` rose by
   about the guest's RSS. Snapshot files `0600`, dir `0700`.
5. Wake with `POST /internal/vmctl/resolve {"user_id","desktop_id"}`; read
   the boot timeline: `boot_kind=snapshot_resume`, `outcome=healthy`,
   wake-to-healthy < 3 s, the same epoch as before hibernate, clock step
   reported. The file and document are served unchanged, schedules not
   closed, no `interrupted by a restart` fates.
6. Invalidation variants, each followed by a wake that must cold boot with
   `snapshot_fallback` naming the reason: touch `data.img` (mtime);
   hibernate, then refresh (config/consumed); wait past `resume_deadline`;
   hibernate a busy computer (expect `hibernate_kill` reason `guest_busy`);
   two concurrent resolves (one loader); kill vmctl mid-hibernate (no
   `snapshot/` dir, only `snapshot.partial/`).
7. Rollback drill: set `VM_HIBERNATE_MODE=kill`, redeploy, hibernate: kill.
