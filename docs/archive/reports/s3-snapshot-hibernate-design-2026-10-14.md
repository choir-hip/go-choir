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
   fence closes work admission at the actor handler boundary
   (`actorHandler.HandleUpdate`, every update kind, including source polls
   that call models without a run) only when no occurrence is inside, then
   checks running runs and mid-transaction self-development operations;
   busy answers 409 and reopens admission; vmmanager then uses the kill path
   (today's behavior).
2. **Race: new work admitted between the check and the pause.** Answer: the
   gate stays closed from the check until `resumed` (or a 2-minute
   monotonic TTL, which a paused guest does not advance). There is no abort
   call: every host failure after the fence is a kill. A run
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
   `credential.img`) and of the boot artifacts and read-only store image
   (kernel, initrd, `storedisk.erofs`, rootfs) are taken after Firecracker
   stops and checked before load; any cold boot also discards the snapshot
   first. The artifacts are bound because Node B deploys them to stable
   paths (`/var/lib/go-choir/guest/...`): an unchanged config hash does not
   prove unchanged bytes, and a resumed guest's page cache describes the old
   store image. A deploy that rewrites those files therefore invalidates
   every snapshot (conservative).
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

## Implementation map

- Host, vmmanager: `internal/vmmanager/fcapi.go` (Firecracker API client),
  `snapshot.go` (modes, metadata, validation, budget, guest client),
  `snapshot_lifecycle.go` (`hibernateVMSnapshot`, `ResumeVMFromSnapshot`,
  `HibernateTimelineForVM`). Env: `VM_HIBERNATE_MODE` (default `kill`),
  `VM_SNAPSHOT_BUDGET_GIB` (48), `VM_SNAPSHOT_MAX_AGE` (12h).
- Host, vmctl: hibernate calls the manager outside the registry lock under
  the per-computer in-progress guard; `startExistingVM` offers a hibernated,
  unheld computer to `ResumeVMFromSnapshot` before RecoverVM/BootVM, passing
  its current shape, hold, computer id and epoch; a snapshot belongs to one
  epoch. Operator surface: `POST /internal/vmctl/hibernate`
  (`{"computer_id"}` or `{"user_id","desktop_id"}`, stop reason
  `operator`, response carries the hibernate receipt) and
  `GET /internal/vmctl/boot-timeline?computer_id=...&kind=hibernate`.
- Guest: `internal/agentcore/hibernate.go`.
  `POST /internal/runtime/hibernate/prepare` answers 200
  `{"status":"prepared","capability_expires_at","guest_build","credential_resume"}`
  or 409 `{"outcome":"busy",...}`; `POST /internal/runtime/hibernate/resumed`
  `{"host_unix_nano","credential_envelope"?}` answers 200
  `{"status":"resumed","clock_step_ms","credential":"installed|renewed"}`.
  Both are host-peer only. `GuestCredentials.RenewNow` forces a renewal;
  `GuestCredentials.InstallResumeEnvelope` installs a delivered envelope
  (see "Re-credential on resume").
- `nix/node-b.nix`: `VM_HIBERNATE_MODE=snapshot`, `VM_SNAPSHOT_BUDGET_GIB=48`.

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

- **R1** credential window (item 11): lifted by re-credential on resume
  (section below); it still bounds snapshots of guests without
  `credential_resume` and wakes whose issuance fails.
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
   the API socket (read-only: `ls -la <vm-state>/<vm>/api/` shows `fc.sock`;
   a computer launched before the switch hibernates by kill with reason
   `no_api_socket`).
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

## Re-credential on resume (R1 lift, 2026-10-14)

Mutation class: **red** (guest credential delivery and installation,
platform capability lifecycle, VM lifecycle wake path). Before this slice a
snapshot was resumable only until the frozen capability's expiry + 15 min
renewal grace − 3 min margin (about 42 minutes after hibernate); later wakes
cold booted (crash boot). Most hibernations last hours, so real hibernation
rarely applied.

### How boot delivery works today (read from the code)

- vmctl (`reserveFreshVMConfig`) asks corpusd
  `POST /internal/computers/credentials/issue` (trusted internal caller only)
  for a signed bootstrap envelope bound to `(computer_id, realization_id)`
  with idempotency key `guest-credential:<realization_id>:<epoch>`; TTL
  10 min. vmmanager writes it to `credential.img` (never logged, stripped
  from snapshot metadata).
- The guest (`autoputer/run.go`) verifies the envelope against its own
  computer and realization (`VerifyBootstrap`), exchanges it
  (`/credentials/exchange`, no bearer: the envelope is the proof) for a
  capability at the head's current **credential revocation epoch** E plus a
  post-revocation capability at E+1, consumes it (`/credentials/consume`,
  authorized by the new capability, one-shot per envelope nonce), then
  appends the consumption receipt as `key_revoked`, which advances the head
  to E+1 and revokes every older capability; the guest switches to the E+1
  token (`CompletePostRevocation`).

### What the platform allows (checked, no platform change needed)

Two different epochs exist and must not be confused:

- **Realization epoch** (vmctl `ReserveBootEpoch`, in `realization_id`
  `<vm>-epoch-<n>`): a snapshot resume keeps it. corpusd puts no uniqueness
  rule on envelopes per realization: a second envelope for the same
  realization is issued as long as the idempotency key differs. The boot key
  cannot be reused: a same-key request replays the stored issuance with an
  expiry derived from its recorded issue time, which is long past, and is
  refused (409). The resume therefore uses a fresh key
  `guest-credential-resume:<realization_id>:<epoch>:<random>`.
- **Credential revocation epoch** (event head, carried in every capability):
  the exchanged capability is minted at the head's current epoch, which
  `SignedCapabilityVerifier` requires; so minting a fresh capability for the
  same epoch is allowed. Consuming the envelope and appending `key_revoked`
  then advances this epoch by one — exactly what every cold boot does. The
  resume follows that boot protocol unchanged rather than skipping
  consumption (skipping it would leave the envelope re-exchangeable for its
  10-minute life and keep the frozen capability valid). Consequence: a
  resume advances the credential revocation epoch by one and revokes the
  snapshot's frozen capability; it does not advance the realization epoch.
  Pinned by `TestResumeEnvelopeForSameRealizationIsIssuedExchangedAndConsumed`
  (`internal/platform`).

### Design

- Prepare advertises `credential_resume: true` when the guest can install a
  delivered envelope; metadata records it.
- Wake: vmctl hands the manager an issuer (a closure, not a value: an
  envelope is minted only for a snapshot that validated, never for every
  wake of a hibernated computer). The manager validates, calls the issuer,
  then consumes the snapshot, loads, and posts
  `{"host_unix_nano", "credential_envelope"}` to
  `/internal/runtime/hibernate/resumed`.
- Guest: step the clock (envelope expiry checks need the right time), drop
  idle connections, verify the envelope against its own computer,
  realization and platform signing key, exchange, consume, append
  `key_revoked`, switch to the post-revocation token, then reopen admission.
  Without an envelope the guest renews as before.
- The `resume_deadline` is no longer enforced when the snapshot's guest
  accepts delivered credentials and vmctl can issue them. It is still
  recorded and still bounds the renew-only path (older guests, no corpusd
  URL, issuance failure). `VM_SNAPSHOT_MAX_AGE` (12 h) and the disk budget
  are unchanged.

### Failure modes (written before the code)

26. **Envelope minted but the snapshot is invalid.** Wasted issuance on
    every wake. Answer: the issuer runs only after validation passed.
27. **Issuance fails (corpusd down, refusal).** Answer: within the old
    renewal window the resume proceeds without an envelope (guest renews);
    past it, fall back to a cold boot (`credential_issue_failed`); that boot
    needs corpusd too and fails the same way it does today.
28. **Boot idempotency key reused.** corpusd replays an expired issuance and
    refuses. Answer: fresh random key per resume attempt; vmctl's retry
    inside one attempt reuses it, so a timed-out issuance is replayed, not
    duplicated.
29. **Envelope for another computer or realization delivered to this
    guest.** Answer: `VerifyBootstrap` binds computer and realization; any
    mismatch refuses, the host kills and cold boots.
30. **Platform signing key rotated while hibernated.** The guest's receipt
    resolver holds the old key. Answer: an envelope signed by a different
    key is refused (cold boot rebuilds everything from the new key).
31. **Old capability already expired past the grace.** Renewal is refused.
    Answer: the exchange needs no old bearer (the envelope is the proof);
    consume is authorized by the newly exchanged token.
32. **Guest mid-transition** (an unconsumed exchange or unreconciled
    lifecycle receipt left in memory). Answer: install refuses; cold boot.
33. **Exchange succeeds, consume or `key_revoked` append fails.** Answer:
    resumed answers non-200; admission stays closed; host kills, discards
    the (already consumed) snapshot and cold boots. The cold boot's own
    envelope and `key_revoked` revoke whatever the failed install held. An
    unjoined consumption receipt is left, the same as a boot that crashed at
    that step.
34. **A background append races the epoch switch** (between the
    `key_revoked` append and `CompletePostRevocation`). Admission is closed,
    so no actor work runs; a stray background request may be refused once.
    Answer: accepted, named here; the window is one in-process call.
35. **Credential material leaks.** Answer: the envelope travels only in the
    host-only resumed call (host-peer check on the guest, the host's TAP
    link, like every guest↔corpusd bearer); it is never written to snapshot
    metadata, the timeline, `inst.Config`, or logs; error strings carry no
    token. The snapshot's frozen token is revoked by the resume's
    `key_revoked`.
36. **Old guest, new host** (snapshot taken by a guest without the field).
    Answer: no `credential_resume` in metadata, so no envelope is sent and
    the deadline applies (today's behavior). A guest that does not know the
    field would refuse the request (unknown field) and cold boot, which is
    safe.
37. **Resume twice / envelope replay.** Answer: snapshot one-shot is
    unchanged (consumed before load); the envelope is consumed once by the
    platform; a replayed exchange after consumption is refused.
38. **Resumed call now does three corpusd calls and an append.** Answer:
    the host's resumed timeout rises from 30 s to 60 s.

### Red ceremony

- **Conjecture delta:** before, a hibernated computer older than ~42 min
  woke by crash boot. After: a snapshot of a credential-resume guest wakes
  by memory resume up to `VM_SNAPSHOT_MAX_AGE` (12 h), with a fresh
  capability from the same issuance path a boot uses. Falsified if staging
  shows `snapshot_fallback` with `credential_window_passed` or
  `guest_resumed_failed` after hibernations longer than 45 minutes.
- **Protected surfaces:** corpusd credential issuance (called with a new key
  shape, no code change), guest credential install
  (`selfdev.GuestCredentials.InstallResumeEnvelope`), guest `key_revoked`
  append at runtime (the boot reconcile function moved, unchanged, to
  `agentcore.ReconcilePendingLifecycleReceipts`), vmmanager resume
  validation and resumed call, vmctl wake path.
- **Admissible evidence:** unit tests with fakes (below) for shape;
  staging-only for the claim (real corpusd, real guest, real clock): a
  hibernation held > 45 min must wake with `boot_kind=snapshot_resume`,
  `credential=delivered`, the same realization epoch, the event head's
  credential revocation epoch +1, a `key_revoked` event on the tape, and no
  `interrupted by a restart` fates.
- **Rollback:** revert the commits (the host then sends no envelope and the
  deadline applies again), or `VM_HIBERNATE_MODE=kill` for the whole
  feature.
- **Heresy delta:** discovered — "same epoch" is ambiguous in the S3 note:
  the realization epoch is kept, but every envelope consumption (boot or
  resume) advances the credential revocation epoch; the boot key cannot be
  replayed past its window. Introduced — the guest appends `key_revoked`
  while running (only boot did before), with the race in item 34. Repaired —
  R1: snapshot resume no longer bounded by capability lifetime.

### Residuals of this slice

- **R1′** the renewal window still bounds snapshots taken by guests built
  before this slice (no `credential_resume`) and wakes whose issuance fails
  past the window; both cold boot as before.
- **R6** `VM_SNAPSHOT_MAX_AGE` (12 h) is now the binding age bound; an owner
  ruling may lengthen it (overnight hibernation) once staging shows resumes
  are clean.
- **R7** item 34's window (a background request refused once between the
  `key_revoked` append and the token switch) is accepted, not engineered.
- **R8** each snapshot resume appends one `key_revoked` event and two
  corpusd lifecycle receipts, as a boot does.
- **R9** not proven on staging: everything here is unit evidence with fakes
  plus the platform rules exercised on a real corpusd store in-process.

### Staging evidence plan addition

8. Hibernate a computer, wait 50 minutes, wake: expect
   `boot_kind=snapshot_resume`, `snapshot.credential=delivered`, epoch
   unchanged, corpusd lifecycle receipts `credential_envelope_issued` and
   `credential_envelope_consumed` under a `guest-credential-resume:` key,
   one new `key_revoked` event on the computer's tape, and a subsequent
   event append succeeding.

## Real-disk budget and dead snapshots (follow-up, 2026-10-14)

Mutation class: **red** (VM lifecycle: snapshot budget, stop paths, vmctl
start). Evidence (Node B, 2026-10-14, `compsize`; recorded in
[artifact store problem](../../problems/artifact-store-grows-12gb-a-day-2026-10-14.md)
"Also found"): the root filesystem is btrfs `compress=zstd:3`; an 8 GiB
snapshot memory file uses about 560 MB of disk (6%). Item 17's budget counts
logical bytes, so `VM_SNAPSHOT_BUDGET_GIB=48` admits about 6 snapshots where
the disk allows about 85. Separately, snapshots of computers that were
later stopped stayed on disk (`vm-0f24d3b8...`, `vm-db4bb41c...`, both
stopped at 08:30): a stopped computer cold boots, so those files are dead
weight, and they keep their host-port reservation.

### Failure modes (written before the code)

39. **Budget counts logical bytes** (observed). Answer: count each committed
    snapshot at its on-disk bytes, recorded in metadata at commit.
40. **The on-disk measure is wrong.** `st_blocks` on btrfs counts the
    uncompressed size of allocated extents, so it is not compression-aware.
    FIEMAP reports logical extent lengths and has no compressed-length
    field. The free-space delta of `statfs` around the write is noisy (other
    VMs and the artifact store write concurrently) and lags the transaction
    commit. Answer: on btrfs, read the file's extent items with
    `BTRFS_IOC_TREE_SEARCH` (what `compsize` does) and sum each distinct
    extent's `disk_num_bytes` (inline extents: their inline length); holes
    count zero. Measured after the files are fsynced, when the compressed
    extents exist. Any error (not btrfs, no `CAP_SYS_ADMIN`, no extents found
    for a non-empty file) falls back to `st_blocks*512`, which on btrfs
    overcounts: the safe direction. The method is recorded with the bytes.
41. **The next snapshot's on-disk size is unknown before it is written.**
    Answer: reserve the worst case (logical memory size plus state
    overhead: incompressible guest memory) for the new snapshot; committed
    snapshots count at their measured bytes. With 48 GiB and 560 MB
    snapshots that admits about 72, not 85; the remainder is the worst-case
    reservation, not waste.
42. **Snapshots committed before this change carry no on-disk bytes.**
    Answer: measured live by the same method when the budget lists them.
43. **The free-space floor is computed from compressed numbers.** Answer:
    the 16 GiB floor stays an absolute guard against the worst-case
    reservation (`statfs` free space on btrfs is real free space).
44. **A stopped computer keeps its snapshot** (observed). Answer: every
    vmctl stop path (explicit stop and wedge stop, logout, ownership
    removal, a hold turning a hibernated computer stopped) discards the
    snapshot and releases its port. It must not depend on the manager's
    `StopVM`, which is not called for a hibernated computer the manager no
    longer tracks (after a vmctl restart) and returns early for a
    hibernated one it does track. Refresh and recover already cold boot,
    and every cold boot discards first (item 7); purge
    (`DestroyVMState`) removes the whole state dir and releases the port.
45. **A discard races a manager operation on the same VM** (resume loading
    it, a hibernate writing it, a boot). Blocking would hold vmctl's
    registry lock behind a minutes-long boot. Answer: the discard takes the
    VM's operation lock only if it is free; if busy it skips and logs: a
    resume removes the files itself (after success or fallback), a cold
    boot discards first, and the start sweep catches anything left.
46. **The start sweep deletes a hibernated computer's snapshot.** The
    computer then crash-boots instead of resuming. Answer: the sweep keeps
    every snapshot whose ownership is `hibernated` (held or not), and runs
    only when the ownership registry was actually read from its durable
    file: an empty in-memory registry would make every snapshot look
    orphaned. Ownerships the restart turned `stopped` (they were running)
    have no committed snapshot to keep.
47. **The sweep deletes outside the state dir or races a capture.** Answer:
    it only removes `<state>/<vm>/snapshot` and `<state>/<vm>/snapshot.partial`
    of real directories (symlinks skipped), and uses the same non-blocking
    operation lock. `snapshot.partial/` is an interrupted capture and is
    removed for every VM.
48. **A hibernate whose ownership changed during capture** leaves a
    committed snapshot vmctl does not record as hibernated. Answer: the
    next start sweep removes it (named residual R10: until then it counts
    against the budget and may be evicted first by age).

### Red ceremony

- **Conjecture delta:** the budget admits snapshots by real disk use, so
  ~70 computers can hibernate warm on Node B instead of ~6; a stopped
  computer never holds snapshot disk. Falsified if staging shows
  `disk_budget` kill-path hibernates with fewer than ~60 committed
  snapshots, or a `snapshot/` dir under a computer whose ownership is not
  `hibernated` after a stop or a vmctl restart.
- **Protected surfaces:** vmmanager budget (`ensureSnapshotRoom`), snapshot
  metadata and hibernate timeline fields (`disk_bytes`,
  `disk_bytes_method`), vmctl stop paths, vmctl start.
- **Admissible evidence:** unit tests with a faked disk measure for the
  budget, the stop discard and the sweep; the btrfs measure itself only on
  staging: `disk_bytes` in a hibernate timeline within a few percent of
  `compsize` for the same `mem` file.
- **Rollback:** revert the commits; metadata gains only optional fields, so
  the reverted code reads new snapshots (it counts them at logical bytes
  again).
- **Heresy delta:** discovered: items 39 and 44 (staging); the manager's
  `StopVM` is a no-op for hibernated computers. Introduced: none intended;
  R10 named. Repaired: item 17's budget unit; dead snapshots of stopped
  computers.

### Staging evidence plan addition

9. Hibernate a computer; the hibernate timeline shows `bytes` (logical) and
   `disk_bytes` with `disk_bytes_method=btrfs_extents`; `compsize
   <state>/<vm>/snapshot/mem` agrees within a few percent. Stop it
   (`POST /internal/vmctl/stop`): its `snapshot/` dir is gone. Restart
   vmctl with one hibernated and one orphaned snapshot dir: the log names
   the orphan as removed, the hibernated one survives and resumes.
