# D2 design: memory snapshots that survive a platform deploy (2026-10-26)

Station D (density), item D2 of
[choir-density-2026-10-26.md](../definitions/choir-density-2026-10-26.md).
It supersedes the L20 section of
[fp10-snapshot-levers-and-host-slab-2026-10-22.md](fp10-snapshot-levers-and-host-slab-2026-10-22.md)
(see §4.1 for where and why it differs). Evidence:
[start-refused-for-memory-no-reclaim-2026-10-26.md](../problems/start-refused-for-memory-no-reclaim-2026-10-26.md) §6.3,
[hibernate-writes-whole-guest-and-deploys-void-snapshots-2026-10-22.md](../problems/hibernate-writes-whole-guest-and-deploys-void-snapshots-2026-10-22.md) R1.

- **Mutation class of this document:** green. Every build slice is red
  (VM lifecycle, snapshot validation, boot path, deploy activation)
  except S1, which is orange.
- **Review tier:** this design is reviewed like the change it designs:
  **tier 2** (Devin and DeepSeek v4.1 flash). A defect here can be silent
  and can damage data: loading guest memory against disk bytes it was not
  taken against corrupts a computer without an error. S3 is tier 2. S1, S2,
  S4 and S5 are additive and behind a switch, so they are tier 1.
- **Owner inputs:** "If we can restore fast we can hibernate idle computers
  at will" (2026-10-26). Security first, then availability, then
  customizations (2026-10-25). The floor is "all host-level and
  security-relevant code, everything engineering has no access to"
  (2026-10-25 Q2). The support window is one week (2026-10-25 Q1). Restart
  rules are in CLAUDE.md, "Restarts End Work (Crash) Or Resume It (Planned
  Update)".
- **Proof computers:** d@e and test accounts only. b@c is crash-looping
  at boot (X1) and may not be a starter, a victim or a proof computer
  until X1 lands.

## 0. The answer in brief

1. **What voids a snapshot today.** Every deploy that runs a NixOS
   switch builds a new guest image, even when no guest source changed,
   because the commit is stamped into it. The guest is launched from
   stable paths that the switch repoints to the new image. So the
   snapshot's launch hash fails (`kernel-params` names a new `init=`
   store path), and so does its disk identity (new inodes). The real
   incompatibility is the **store disk** (and the runtime binary inside
   it). The checks on the kernel, initrd, rootfs and kernel parameters are
   conservative. A deploy that changes host services only (no NixOS
   switch) already keeps snapshots: staging resumed one across 26 minutes
   on 2026-10-24.
2. **When it is safe.** A snapshot is safe to resume after a deploy if
   all of these hold: the bytes it was taken against still exist; the
   guest floor is unchanged; the security epoch is unchanged; the host
   still serves the guest's ABI; the release is inside the support window;
   and the Firecracker version, host kernel, confinement and host boot
   policy are unchanged. Then it resumes onto **its own** release. A
   computer that resumed behind the current release is updated at its next
   idle moment by an ordinary planned, marker-backed refresh ("resume,
   then update"). If any condition fails, the snapshot never resumes and
   the wake cold boots onto the current release, as every wake does today.
3. **Restart rules.** A resume is not a boot, so no marker is consumed or
   written. The update after it is the existing deploy refresh. The guest
   writes its marker only when it is not busy. The refreshed boot consumes
   the marker once. A crash in that boot closes work. Busy computers wait
   in the existing durable pending set.
4. **Security.** Floor and security epoch are fixed when the image is
   built and recorded in the image itself. They are compared against the
   **current** release at every wake. A snapshot from before a floor move or
   an epoch bump cannot load. A hibernated computer therefore never runs
   pre-fix code again; its next wake is a cold boot onto the fix. A
   computer running behind is active, so the deploy that carries the fix
   reaches it like any other active computer.
5. **Proof.** Hibernate d@e. Wait for a deploy that **changes the guest
   image**, which means any NixOS-switch deploy. Wake d@e and read the wake
   timeline: `outcome=snapshot`, `behind=true`, `wake_to_healthy_ms < 1000`.
   Then read the retry sweep's planned refresh onto current. A deploy that
   changes host services only is **not** a test of D2, because it already
   passes today; it is the baseline (§9 P0).

## 1. Why a deploy voids a snapshot today

### 1.1 The mechanism, with citations

- **How the guest is launched.** vmctl launches every guest from stable
  paths:
  - `VM_KERNEL_IMAGE=/var/lib/go-choir/guest/vmlinux`, `VM_ROOTFS_IMAGE`,
    `VM_INITRD_IMAGE`, `VM_STORE_DISK_IMAGE` and
    `VM_KERNEL_PARAMS_FILE=/var/lib/go-choir/guest/kernel-params`
    (nix/node-b.nix:1632-1636).
  - `/var/lib/go-choir/guest` is a symlink. The `go-choir-guest-image`
    activation script repoints it at the new guest image's Nix store path
    at every NixOS switch: `src="${guestImage}"`, then
    `mv -Tf "$next" "$target"` (nix/node-b.nix:2865-2979).
- **Every switch makes a new guest image.** The commit is stamped into it
  in three places:
  - `buildCommit = self.rev` (flake.nix:235) is written into the image's
    `build.json` (flake.nix:597-598).
  - It is also written into `/etc/choir-guest-image-manifest` inside the
    guest (`build_commit=`, nix/autoputer-vm.nix:50-53).
  - It is linked into every Go binary (`-X ...buildinfo.Commit`,
    flake.nix:289). All Go services are one `buildGoModule`, so any Go
    change gives every binary in the guest a new output path.

  The guest system closure therefore changes on every commit. So do the
  erofs store disk and the `init=` path in `kernel-params`
  (flake.nix:600-602). The 10-22 record already found this: "Even
  840e1a701, which changed only the Go vendorHash, has its own guest image
  path" (hibernate-writes-whole-guest R1).
- **Which deploys switch.** A NixOS switch happens whenever the deploy
  classifier marks the host OS:
  - `mark_guest_boot_contract` sets `deploy_host_os=true` for
    `internal/vmctl`, `internal/vmmanager`, `cmd/vmctl`, the frontend,
    `internal/projectionbase` and others
    (.github/scripts/deploy-impact-classify:89-95, 172-193).
  - Any `nix/` change switches too.
  - A change to host services only (proxy, auth, gateway, maild,
    corpusd) is a service-pointer deploy: "Skipping NixOS switch (host
    service pointer deploy)" (.github/scripts/node-b-ci-deploy:66-69,
    326-327). The guest symlink does not move.
- **What a wake checks.** `validateSnapshot`
  (internal/vmmanager/snapshot.go:538-597) runs these checks in order:
  1. Consumed, version and identity.
  2. Age: `VM_SNAPSHOT_MAX_AGE=36h` on Node B (nix/node-b.nix:1750; the
     code default is 12h, snapshot.go:67).
  3. Credential window.
  4. Firecracker version (:565).
  5. Host kernel (:568).
  6. Confinement (:572).
  7. The launch-config hash (:575-577).
  8. File presence and the Diff rules.
  9. The identity of every bound file: size, mtime, inode, device
     (:587-595).

  `expectedLaunchConfig` (snapshot.go:473-500) rebuilds the config
  against the **current** deploy. `refreshConfigForCurrentDeploy`
  blanks the kernel, initrd, rootfs, store disk and kernel parameters
  (internal/vmmanager/manager.go:1912-1925), and `applyLaunchDefaults`
  refills them from the manager config (manager.go:1232-1257). The manager
  config reads `kernel-params` **once, at vmctl start**
  (internal/vmmanager/config.go:47-49).
- **What staging shows** (vmctl journal, read 2026-10-26, last 7 days):
  - 13 snapshot hibernates and 2 resumes.
  - 5 fallbacks to a cold boot, all `launch_config_mismatch`. In each,
    the vmctl process changed between the hibernate and the wake (a
    deploy restart).
  - One resume, vm-ee2fb953, was hibernated at 2026-10-24 06:27:37 and
    resumed at 06:53:32 under the same vmctl process (pid 967842). No
    switch landed in between.
  - No snapshot hibernate has happened since 2026-10-24 08:23. See §11
    H4.

### 1.2 Real incompatibilities and conservative voiding

| What a deploy can change | Checked by | Real or conservative |
|---|---|---|
| **Guest store disk** (erofs: the guest's /nix/store, including the runtime binary `autoputer`, updater, capsule broker, frontend) | `kernel-params` `init=` path in the launch hash; bound-file identity | **Real.** The guest's page cache and the file-backed text of running processes describe old erofs blocks. Pages it evicts are read back from the disk, so loading against new bytes corrupts the guest. It is safe only against the **old** bytes. |
| **Runtime binary** | the same as the store disk (it lives inside it, nix/autoputer-vm.nix:239); no separate host-served runtime exists | **Real**, for the same reason. A self-development layered release lives on data.img, which is inside the snapshot. |
| **Guest kernel (vmlinux), initrd** | bound-file identity, launch hash | **Conservative.** A snapshot load does not read the boot source; kernel text is in guest memory. They matter only at the next cold boot. Kernel modules loaded later come from the store disk (covered above). |
| **rootfs.ext4** | bound-file identity only | **Conservative.** It is not attached when a store disk is configured (manager.go:1951-1968). |
| **kernel-params** | launch hash | **Conservative** in itself, because it is read only at boot. It is a proxy for "the store disk changed". |
| **Commit stamps** (build.json, manifest, buildinfo) | everything above | **Conservative.** A stamp-only change makes new bytes with no semantic difference, but the bytes still differ, so it still needs the old release. |
| **Host vmctl version** | not checked; a vmctl restart keeps hibernated snapshots (`SweepOrphanSnapshots` keep) | No incompatibility in the files. The host-guest **protocol** can drift; today it is guarded only by the fail-closed resume handshake (§6.4). |
| **Host boot policy** (guest DNS list, reclaim list, balloon, hotplug, discard, capsule tunnel identity: manager.go:2035-2047, 2114-2119) | launch hash | **Conservative but intended.** Policy is delivered at boot, and some of it is security policy (SEC-10 guest DNS), so a change must reach the computer at its next boot. **Keep.** |
| **Firecracker version** | exact `--version` string | Real across versions with different snapshot formats; conservative for patch versions. It changes rarely (pinned 1.17.0). **Keep.** |
| **Host kernel** | `uname -r` | KVM state compatibility; conservative. It changes only with a host reboot. **Keep.** |
| **CPU template** | **nothing**: vmmanager sets no `cpu_template` (no match in internal/vmmanager or nix/node-b.nix) | Same host CPU, so normally fine. A microcode or BIOS change without a kernel change can alter CPUID under a resumed guest: a **real risk that nothing guards**. S5 adds a CPU fingerprint. |
| **Confinement** (jailer or direct, VMM uid) | `confinement_mismatch` | **Real.** The vmstate names jail paths, and the files belong to the VMM uid. **Keep.** |

So today **every snapshot is voided by every NixOS-switch deploy**,
mostly for conservative reasons. The one real dependency, the store disk,
could be met by keeping the old bytes, which are still in the Nix store
for up to three system generations (scripts/node-b-nix-retention:45).
That is only about 2 to 3 hours at the current cadence.

## 2. When a snapshot is safe to resume after a deploy

| Case | Today | With D2 |
|---|---|---|
| **(a) Guest image unchanged, only host services changed** (service-pointer deploy, no switch) | Resumes already (vm-ee2fb953, §1.1). | Unchanged. |
| **(a') Host-only change that still needs a switch** (nix/node-b.nix, vmctl code) | Voided: the switch makes a new guest image (commit stamps). | Becomes case (b): the floor is unchanged, so it **resumes onto its own release**, then updates. |
| **(b) Guest changed, but not its floor and not its security epoch** (runtime, frontend, desks) | Voided, cold boot. | **Resume, then update.** Resume onto the snapshot's own release, sub-second. Mark the computer behind. It refreshes onto current at its next idle moment through the planned, marker-backed deploy refresh (§5). |
| **(c) Never** | n/a | Any of these: the floor digest changed; the security epoch changed; the guest ABI is outside the host's range; the release is older than the support window; the snapshot is older than `VM_SNAPSHOT_MAX_AGE`; the release files are missing or changed; the Firecracker version, host kernel, confinement or host boot policy changed; the CPU fingerprint changed (S5); or the snapshot was already consumed or its epoch moved. The wake cold boots onto current: a crash boot, which closes open work (none, since the fence guaranteed nothing ran). |

## 3. Data model

### 3.1 The guest release

A **release** is one guest image, identified by its Nix store hash, the
first 32 characters of the store path's basename (`id`, regex
`^[0-9a-df-np-sv-z]{32}$`). Its files live in a root-owned, read-only
directory:

```
/var/lib/go-choir/guest-releases/<id>/
  vmlinux  initrd  rootfs.ext4  storedisk.erofs  kernel-params
  build.json  guest-image-manifest  release.json          (reflink copies, 0444)
  installed.json   {"store_path": "/nix/store/<id>-go-choir-guest-image",
                    "first_current_at": "<RFC3339 UTC>"}  (written once, 0444)
```

`release.json` is built into the guest image by nix (S1), so it is
immutable and tied to the build:

```json
{"schema": 1,
 "floor_digest": "sha256:<hex>",
 "security_epoch": 1,
 "guest_abi": 1,
 "guest_abi_min": 1,
 "build_commit": "<commit>"}
```

- **floor_digest.** A sha256 computed at evaluation time over inputs that
  do not depend on the commit:
  - the guest kernel store path and its config file
    (`config.boot.kernelPackages.kernel`, `.configfile`);
  - the initrd store path (`config.system.build.initialRamdisk`);
  - the locked revisions of `nixpkgs` and `microvm` (flake.lock);
  - the Go toolchain version (`choirGo.version`);
  - the store path of a **floor source fileset**: `lib.fileset.toSource`
    over the paths listed in `nix/guest-floor-paths.txt`. Its hash
    depends only on file contents.

  The list holds:
  - every Choir package that the floor binaries import (the guest's Choir
    binaries other than `autoputer`: `cmd/choir-updater`,
    `cmd/capsule-broker`, `cmd/choir-receipt-signer`,
    `cmd/choir-delegate`, `cmd/choir-capture`);
  - `go.mod` and `go.sum`;
  - `nix/autoputer-vm.nix`;
  - the scripts it reads (`scripts/guest-signer-state-migrate`,
    `scripts/guest-signer-state-project`);
  - the definition of `choirKernelModule`.

  Today that is 35 Choir packages
  (`GOOS=linux go list -deps` of those commands, run 2026-10-26),
  including `internal/capsule`, `internal/updater`, `internal/types` and
  `internal/yaegikernel`. This is the owner's floor, "everything
  engineering has no access to". It over-approximates on purpose: a
  wrongly counted floor change only costs a cold boot.
- **security_epoch** comes from the tracked file
  `nix/guest-release-policy.json`. It is a monotone integer that the
  author of a security fix outside the floor (for example in the runtime
  or a desk) raises in the same commit. It works like Android's rollback
  index and the update design's `security_epoch`
  (docs/design/computer-update-architecture-rnd-2026-10-24.md §4.2).
- **guest_abi** and **guest_abi_min** come from the same policy file.
  `guest_abi` is the host-guest protocol version this build speaks.
  `guest_abi_min` is the oldest guest protocol this build's host services
  still serve. Raise `guest_abi_min` in the commit that makes a host
  service stop serving a request shape an older runtime sends.

There is **no separate mutable floor file**. The current floor is the
current release's own `release.json`. That gives one authority, and it
moves only with the deploy.

### 3.2 What vmctl records

- `VMConfig.GuestRelease string` (`json:"guest_release,omitempty"`): the
  release a realization was launched from. It is empty for a launch in
  legacy mode (switch off, or the current release has no directory).
- **Launch record** `<VM_STATE_DIR>/<vm>/guest-release.json`
  `{"release": "<id>", "epoch": N, "launched_at": "..."}`. It is written
  and fsynced before Firecracker is spawned, and rewritten at every cold
  boot. Reattach reads it (§4.3).
- `SnapshotMeta.GuestRelease string` (`json:"guest_release,omitempty"`):
  copied from the realization's config at hibernate. It is additive, with
  no meta version bump: an old meta reads as legacy.
- On `SnapshotTimeline`, `GuestRelease`, `CurrentRelease` and `Behind`
  are added to the wake receipt.
- On `vmctl.VMInstanceInfo`, `GuestRelease` and `Behind` are added
  (internal/vmctl/ownership.go:505, adapter cmd/vmctl/main.go:381).

## 4. Mechanism

### 4.1 Release directories (S2), and why not the fp10 L20 shape

- fp10 L20 proposed `/var/lib/go-choir/guest/releases/<digest>/` with a
  `current` symlink. Since then `/var/lib/go-choir/guest` has become an
  **atomic symlink into the Nix store** (nix/node-b.nix:2861-2865). A
  directory under it would be inside an immutable store path, and the
  cutover script refuses a non-symlink tree. Releases therefore live
  beside it, in `guest-releases/`.
- **Why not Nix GC roots on the old store paths?**
  - Rooting a guest image probably also roots its whole guest closure:
    the erofs carries store path names in plain text, which Nix's
    reference scanner may record (unmeasured; R7).
  - Every GC caller (`go-choir-nix-gc`, the disk retention sweep, the
    deploy preflight; nix/node-b.nix:250-255) would need to sync roots
    first.
  - vmctl runs under `ProtectSystem=strict` (nix/node-b.nix:430) and
    cannot write `/nix/var/nix/gcroots`.

  Reflink copies avoid all three.
- **Reflink is proven on Node B.** vmctl's jail code builds shared copies
  of `/var/lib/go-choir/guest/storedisk.erofs` "by reflink" (journal,
  2026-10-26 10:51 to 12:15, five times, once per new guest image). The
  source resolves into `/nix/store` on the same btrfs filesystem (`subvol=@`
  holds both, nix/disks.nix:6-10). A reflink costs metadata only until the
  store path is garbage collected. After that the release owns its
  extents.
- **The activation step** goes into the `go-choir-guest-image` activation
  script, before `mv -Tf "$next" "$target"`. It is root and never fails
  the switch:

  ```sh
  rel=/var/lib/go-choir/guest-releases
  id=$(basename "$src" | cut -c1-32)
  if [ -n "$retain" ] && [ ! -d "$rel/$id" ]; then      # retain: nix let-binding, see §7 switch
    tmp="$rel/.tmp-$id"; rm -rf "$tmp"; mkdir -p "$tmp"
    ok=1
    for f in vmlinux initrd rootfs.ext4 storedisk.erofs kernel-params build.json guest-image-manifest release.json; do
      cp --reflink=always "$src/$f" "$tmp/$f" || { ok=0; break; }
    done
    if [ "$ok" = 1 ]; then
      printf '{"store_path":"%s","first_current_at":"%s"}\n' "$src" "$(date -u +%FT%TZ)" > "$tmp/installed.json"
      chmod 0444 "$tmp"/*; chmod 0555 "$tmp"; sync -f "$tmp/installed.json"
      mv -T "$tmp" "$rel/$id"
    else
      rm -rf "$tmp"; echo "go-choir guest release $id not materialized (reflink failed); snapshots of it will cold boot" >&2
    fi
  fi
  ```

  Never use `--reflink=auto`: a silent full copy is GBs per deploy.
  Without a release directory, vmctl launches the current image in legacy
  mode, which is safe.
- **Access:** add `/var/lib/go-choir/guest-releases` to vmctl's
  `ReadOnlyPaths` (nix/node-b.nix:1550), so a VMM escape cannot rewrite a
  kernel a later boot runs, the same property `guest/` has. Add the
  tmpfiles rule `d /var/lib/go-choir/guest-releases 0755 root root -`.

### 4.2 Launch (S3)

- **Resolve once per launch.** New `internal/vmmanager/guest_release.go`
  provides:

  ```go
  // currentRelease resolves VM_GUEST_RELEASE_LINK once: readlink, take the
  // 32-char id, open <VM_GUEST_RELEASES_DIR>/<id>/installed.json, and
  // require installed.store_path == the link target. ok=false → legacy.
  func (m *Manager) currentRelease() (guestRelease, bool)
  type guestRelease struct {
      ID, Dir string
      Kernel, Initrd, Rootfs, StoreDisk string // Dir-joined paths
      KernelParams string                     // contents of Dir/kernel-params, trimmed
      Policy releasePolicy                    // Dir/release.json
      FirstCurrentAt time.Time                // Dir/installed.json
  }
  func (m *Manager) releaseByID(id string) (guestRelease, error)
  ```
- **Using it.** With `VM_SNAPSHOT_RELEASE_RETAIN=on`, every cold boot
  (`BootVM`, `RefreshVM`, recover) fills the kernel, initrd, rootfs, store
  disk and kernel parameters from `currentRelease()`. Today
  `applyLaunchDefaults` fills them from `m.cfg` (manager.go:1232-1257).
  The paths in the cold boot then name the release directory, so a direct
  VM's vmstate names release paths too. It sets `cfg.GuestRelease = id`
  and writes the launch record before the spawn, in `launchFirecracker`
  (manager.go:2283) after `fc-config.json` (:2315).
- **Legacy fallback.** If `currentRelease()` fails, the launch uses
  `m.cfg` exactly as today and `GuestRelease=""`.
- **The switch off.** With `off`, launch is **byte-identical to today**,
  which a golden test of `buildFirecrackerConfig` pins. This keeps the
  flip of the switch from voiding the snapshots that exist at that moment.
- **Jailed launches** need no special case. `prepareJail` links each
  source with `linkSharedReadOnly(src, ...)`, keyed by the source's path,
  device, inode, size and mtime (internal/vmmanager/jail_shared.go:14-33).
  A release path is stable across deploys, so jails on the same release
  still share one inode (RES-05). The vmstate names jail paths, as today.

### 4.3 Reattach (S3)

- **The problem.** `ReattachVMWithConfig` builds the reattached VM's
  config from `m.cfg` (manager.go:1517-1527). After a deploy, that is the
  **new** image's paths for a VM still running the old one. Today it is
  harmless only because the snapshot's launch hash then mismatches.
- **The fix.** With the switch on, reattach reads
  `<vm>/guest-release.json`. If its `epoch` equals the reattached epoch
  and the release directory exists, it sets `GuestRelease` and the release
  paths. Otherwise it sets `GuestRelease=""` and keeps today's paths, and
  that VM's next snapshot is legacy, so it cold boots after a deploy.

### 4.4 Hibernate (S3)

- **Meta.** `commitSnapshot` (internal/vmmanager/snapshot_lifecycle.go:341)
  records `meta.GuestRelease = cfg.GuestRelease`.
- **Bound paths.** `snapshotBoundPaths` (snapshot.go:455) already reads
  the realization's `cfg`, which now holds release paths. It needs no
  change, as long as §4.3 is right.

### 4.5 Wake (S3)

`expectedLaunchConfig` (snapshot.go:473) becomes:

```go
cfg := meta.Config; cfg.VMID = meta.VMID; cfg.credentialDiskPath = meta.CredentialDiskPath
cfg = refreshConfigForCurrentDeploy(cfg, m.cfg)          // blank artifacts as today
if meta.GuestRelease != "" && m.releaseRetainOn() {
    rel, err := m.releaseByID(meta.GuestRelease)         // err → caller refuses (release_missing)
    cfg.KernelImagePath, cfg.InitrdPath, cfg.RootfsPath, cfg.StoreDiskPath = rel.Kernel, rel.Initrd, rel.Rootfs, rel.StoreDisk
    cfg.KernelParams = rel.KernelParams
    cfg.GuestRelease = rel.ID
}
// ...today's shape, hold, hotplug, defaults, token, buildFirecrackerConfig, jail rewrite
```

Host boot policy still comes from today's vmctl, so a policy change still
mismatches the hash. That is wanted (§1.2).

`validateSnapshot` gains release rules, checked **after** consumed,
version, identity, age and credential window, and **before** the
Firecracker, kernel and hash checks:

| Order | Condition | Reason (new constant) |
|---|---|---|
| R0 | `meta.GuestRelease != ""` and the switch is off | `release_retain_off` |
| R1 | `meta.GuestRelease != ""`: the release directory is missing, `installed.json` or `release.json` is unreadable, or the id is malformed | `release_missing` |
| R2 | the current release is unresolvable (legacy current) and `meta.GuestRelease != ""` | `release_current_unknown` |
| R3 | `rel.floor_digest != cur.floor_digest` | `release_floor_moved` |
| R4 | `rel.security_epoch != cur.security_epoch` (lower: a fix landed; higher: the host rolled back; both refuse) | `release_security_epoch` |
| R5 | `rel.guest_abi < cur.guest_abi_min` or `rel.guest_abi > cur.guest_abi` | `release_abi_unsupported` |
| R6 | `now − rel.first_current_at > VM_GUEST_RELEASE_SUPPORT_WINDOW` (default 168h, owner Q1) | `release_out_of_support` |

- **Legacy metas** (`GuestRelease == ""`) skip R0 to R6 and validate
  exactly as today. A legacy snapshot of the current image therefore still
  resumes, and one taken before a deploy still cold boots.
- **On success.** `ResumeVMFromSnapshot` (snapshot_lifecycle.go:440) sets
  `snap.GuestRelease`, `snap.CurrentRelease` and
  `snap.Behind = meta.GuestRelease != cur.ID`, and keeps `GuestRelease` on
  the instance config.
- **Unchanged:**
  - the one-shot consume (snapshot.go:429);
  - the credential delivery (a fresh envelope minted by today's vmctl);
  - the fail-closed `resumed` handshake;
  - discarding the snapshot names after a healthy resume.

### 4.6 Releases in use, and retention (S2 script, S3 endpoint)

- **The endpoint.** `GET /internal/vmctl/guest-releases` (internal caller
  only) returns
  `{"retain": bool, "current": "<id>|", "in_use": [{"id", "running": [vm], "snapshots": [vm]}]}`.
  - `running`: manager instances in a live state with a `GuestRelease`.
  - `snapshots`: unconsumed metas from `listSnapshots()`, from every
    root including the vault, that name a release and are younger than
    `VM_SNAPSHOT_MAX_AGE`.
- **The script.** `scripts/node-b-guest-release-retention` runs as root
  from a new `go-choir-guest-release-retention` oneshot on a 30-minute
  timer, and from the disk retention sweep (beside `nixRetention`,
  nix/node-b.nix:257-275).
  - **Keep:** `current`; the newest other release by `first_current_at`
    ("previous"); any release whose `first_current_at` is under 2 h old
    (a launch that resolved just before a switch); and every id in
    `in_use`.
  - **Delete:** every other directory whose name matches the id regex
    and that is a real directory (not a symlink), with `rm -rf`.
  - **Delete anyway:** any non-current release older than the support
    window, which can never resume (R6).
  - **Cap:** `GO_CHOIR_GUEST_RELEASE_KEEP_MAX` (default 8). Above it,
    delete releases that only snapshots use, oldest first. Never delete
    current, previous, or a release a running VM uses.
  - **If vmctl does not answer** (connection error, 5xx), delete nothing
    except past the support window. If it answers 404 (an older vmctl),
    treat `in_use` as empty.
  - **Log** the count and `btrfs filesystem du -s` exclusive bytes.
- Retention affects **efficiency only, never correctness**. A deleted
  release makes its snapshots refuse (R1) and cold boot. A running VM
  keeps its bytes through its open file descriptors (direct) or its jail's
  shared copy (jailed).

## 5. Restart rules and deferred refresh

### 5.1 Resume, then update

1. **Resume.** It is not a boot (internal/agentcore/hibernate.go:29-33).
   No marker is written or consumed, and no closing pass runs. The
   hibernate fence guaranteed that nothing ran when the snapshot was taken
   (`prepareHibernate` refuses while runs or self-development operations
   are live, hibernate.go:281-326).
2. **Enroll.** If `Behind`, vmctl adds the computer to the **existing
   durable pending set** (internal/vmctl/deploy_refresh_retry.go:86-155)
   with:
   - `Target` = the current release's `build.json` commit;
   - a new field `Kind: "woke_behind"`;
   - `LastReason: "woke behind on release <id>"`.

   The hook goes into `OwnershipRegistry.resumeFromSnapshot`
   (ownership.go:1664-1689) as a callback set from `cmd/vmctl/main.go`
   next to `SetDeployRefreshPendingPath` (:264), so the registry does not
   import the handler. A resume that is not behind enrolls nothing.
3. **User-idle gate.** Before asking the guest, the sweep
   (`SweepDeployRefreshPending`, :256-288) skips a `woke_behind` entry
   until `now − own.LastActiveAt ≥ VMCTL_BEHIND_REFRESH_IDLE` (default
   2m). Without the gate, a person who has just woken their computer would
   get a 10 s cold boot within a minute. With D3's 10-minute idle
   hibernate, the 2-minute gate fires first in practice. Entries that are
   not `woke_behind` are unchanged.
4. **Update.** The sweep calls the unchanged `trackedDeployRefresh`.
   - The guest writes its planned-restart marker, or refuses while busy
     (deploy_refresh.go:424-447). `allow_busy` applies only to
     critical_protected and public_platform computers (:215-221), which
     never idle-hibernate.
   - vmctl then kills the computer and cold boots it onto current, and
     the boot consumes the marker once.
   - Busy: the entry stays and is asked again every minute. Refreshed or
     failed: the entry is forgotten (:134-155), so there is no kill loop.
   - "Already serves target": forgotten (:277-283).
5. **The next deploy.** A behind computer is active, so the next deploy's
   CI refresh (`selectDeployRefresh`, deploy_refresh.go:280-315) reaches
   it like any other active computer. `settle` then forgets the pending
   entry.

### 5.2 Why the update keeps the marker

The goal file's D2 note suggested that an idle computer needs no marker.
This design keeps the marker. The guest is the only authority on whether
it has open work (`plannedRestartBusy`). Asking it costs one request, and
the marker path is the existing, proven one, so a second refresh path is
not worth adding.

### 5.3 The marker rules hold

| Rule | How it holds |
|---|---|
| A crash restart never resumes work | A behind guest that crashes is recovered by a cold boot with no marker, onto current. Its work closes. |
| A planned restart resumes once | The refresh boot consumes the marker before resuming (planned_restart.go). If that boot crashes, the next boot has no marker and closes work. |
| No boot-time resumer without a marker | Nothing new runs at boot. The pending set is not a resumer: it only asks the guest for a planned refresh. After a vmctl restart, reattach reads the launch record (§4.3) and the durable pending set continues. **A vmctl restart resumes nothing.** |
| One marker, one resume | A snapshot resume neither writes nor consumes a marker. The remaining hazard is a marker **captured inside a snapshot**: mark, then a lifecycle race lets a hibernate win, `marker.clear` fails against the fenced guest (deploy_refresh.go:457-460), and the resumed guest crashes within the marker's 15 minutes (planned_restart.go:26). That crash would boot as planned. S5 closes this (§7). |

## 6. Security

1. **Forced fixes reach resumed computers.**
   - A floor fix moves `floor_digest` automatically (§3.1). A fix
     elsewhere must raise `security_epoch`.
   - Both are compared with the **current** release at every wake (R3,
     R4). So a snapshot from before the fix cannot load. The wake cold
     boots onto the fixed release, which is today's behaviour and today's
     latency.
   - A hibernated computer runs nothing, so it is not exposed while it
     waits.
   - A computer already running behind is active, so the deploy that
     carries the fix reaches it through the ordinary CI deploy refresh.
     D2 adds no new deferral. Forcing a fix on a **busy** computer (the
     owner's 10-25 ruling: "a forced fix is never deferred, and a
     computer is never contained or stopped for it") belongs to the
     update architecture (U-series), not to D2. D2 neither changes nor
     weakens it.
2. **A stale snapshot is never a bypass.**
   - Consumption is one-shot and durable before load.
   - Every refusal discards the snapshot files (`fallback`,
     snapshot_lifecycle.go:445-450).
   - The switch off refuses every release-bound snapshot (R0).
   - Release directories are read-only to vmctl.
   - Retention deletion only causes cold boots.
   - Epoch equality (R4) refuses in both directions, so a host rollback
     cannot load a snapshot newer than the host.
3. **Bounded staleness for an unclassified fix.** If an author ships a
   security fix outside the floor without raising the epoch, a woken
   computer runs the old runtime until its idle refresh. That is bounded
   by the support window (one week from `first_current_at`) and by
   `VM_SNAPSHOT_MAX_AGE` (36h). The exposure is the same as a busy
   computer deferred at deploy, which happens today. The guard is:
   - the commit rule (S1 adds it to `nix/guest-release-policy.json`'s
     header comment);
   - a deploy classifier line that labels any deploy touching that file
     "security epoch raised" (S1, explanation only).
4. **Host-guest protocol.**
   - **Today:** an old runtime that meets new host services is guarded
     only by the resume handshake. `resumed` must answer 200 and
     `/health` must pass, or the guest is killed and cold boots. The
     credential envelope is minted by the current vmctl, so a format the
     old guest cannot install fails closed (hibernate.go:367-385).
   - **D2 adds `guest_abi` and `guest_abi_min`** for skew that is
     *silent*. They are the same compatibility that deferred busy
     computers already rely on, now named.
5. **Credential delivery** is unchanged. **The snapshot vault** is
   unchanged; vault metas are counted in `in_use`.

## 7. Slices

Use the rules in the goal file for every item: failure modes first (green,
alone); then tests (yellow, red until the code); then the code; push
`lane/<name>` after each commit, never main; the gates before every push;
both nix evals when `nix/` changed. Order: S0, S1, S2, S3, S4; S5 may run
after S3; then S6.

### S0: problem documentation (green)

- Add a "D2" section to
  `docs/problems/density-station-d-failure-modes-2026-10-26.md` in the D1
  shape (Problem, Design, Failure modes M1..M19 from §8, Protected
  surfaces/evidence/rollback). Commit it alone.
- FP0 decides whether H1 and H2 (§11) get their own problem docs. They
  are real defects of today's code that S3 also repairs.

### S1: release metadata in the guest image (orange, tier 1)

**Files:**
- `nix/guest-release-policy.json` (new):
  `{"security_epoch":1,"guest_abi":1,"guest_abi_min":1}`, with the commit
  rule for raising each in a sibling `nix/guest-release-policy.md`
  comment, or in the flake comment, because JSON has no comments.
- `nix/guest-floor-paths.txt` (new): one repo-relative path per line.
- `flake.nix`: a `guestFloorDigest` let-binding (§3.1), and
  `mkGuestImage` (flake.nix:575-605) writes `$out/release.json` from it,
  the policy file and `buildCommit`.
- `scripts/check-guest-floor-paths` (new): runs
  `GOOS=linux CGO_ENABLED=0 go list -deps` over the five floor commands,
  keeps `github.com/yusefmosiah/go-choir/...` packages, and fails unless
  each package directory is listed in the floor paths file.
- `.github/workflows/ci.yml`: run that script in the check job.
- `.github/scripts/deploy-impact-classify`: an explanation line for
  `nix/guest-release-policy.json`.

**Tests, written first:**
1. The check script fails on a fixture list missing `internal/capsule`
   and passes on the real list.
2. A flake check, `checks.x86_64-linux.guest-floor-commit-free`. It
   evaluates the guest NixOS config with two different `buildCommit`
   specialArgs and asserts that the kernel, initrd and floor-source store
   paths are equal. This proves the floor digest does not move on a
   commit-only change. **If the initrd differs, take it out of the digest
   and record why.**

**Runtime effect:** none (one extra file in the image directory).
**Rollback:** git revert.

### S2: release directories and retention (red, inert, tier 1)

**Files:**
- `nix/node-b.nix`:
  - a let-binding `snapshotReleaseRetain = false;`, which drives both the
    activation step and the vmctl env in S3, so one edit flips both;
  - the activation step (§4.1);
  - the tmpfiles rule;
  - vmctl `ReadOnlyPaths`;
  - the retention oneshot and timer;
  - the call from `diskRetentionSweep`.
- `scripts/node-b-guest-release-retention` (new).
- `scripts/test-node-b-guest-release-retention.sh` (new; in the style of
  `scripts/test-node-b-nix-retention.sh`).

**Tests, written first:** a fixture tree with current, previous, a young
release, an in-use release, an old unused release, an out-of-support
release, a symlink with a release name, and an odd name. Check:
- with a stub vmctl answering 200: only the old unused and the
  out-of-support releases are deleted;
- connection refused: only out-of-support is deleted;
- 404: `in_use` is empty;
- the cap: the oldest snapshot-only releases go first, and current,
  previous and running are never deleted;
- a symlink or an odd name is never deleted.

**Proof before S3 switches on:** turn `snapshotReleaseRetain` on alone,
deploy, and read R7 (§10). Releases are made and pruned, and vmctl
ignores them.
**Rollback:** set the let-binding to `false`. Existing directories become
inert, and the next retention run prunes all but current and previous.

### S3: release-bound launch, hibernate and wake (red, tier 2)

**Files:**
- `internal/vmmanager/config.go`: env `VM_SNAPSHOT_RELEASE_RETAIN`
  (`off` by default), `VM_GUEST_RELEASE_LINK`
  (`/var/lib/go-choir/guest`), `VM_GUEST_RELEASES_DIR`
  (`/var/lib/go-choir/guest-releases`), `VM_GUEST_RELEASE_SUPPORT_WINDOW`
  (`168h`).
- `internal/vmmanager/guest_release.go` (new; §4.2).
- `manager.go`: the `VMConfig.GuestRelease` field; launch artifacts
  through `currentRelease()` when on; the launch record write in
  `launchFirecracker`; `ReattachVMWithConfig` (§4.3); `mergeVMConfigOverrides`
  must not carry `GuestRelease` into a refresh (blank it in
  `refreshConfigForCurrentDeploy`).
- `snapshot.go`: the `SnapshotMeta.GuestRelease` field;
  `expectedLaunchConfig` (§4.5); `validateSnapshot` R0 to R6; new reason
  constants; `SnapshotTimeline` fields.
- `snapshot_lifecycle.go`: the `commitSnapshot` meta field; the
  `ResumeVMFromSnapshot` timeline and instance fields.
- `internal/vmctl`: `VMInstanceInfo` fields (ownership.go:505); the
  `/internal/vmctl/guest-releases` handler (handlers.go route list near
  :1355); health: per active computer `guest_release` and `behind`, and
  top-level `guest_release_current`.
- `cmd/vmctl/main.go`: the adapter fields (:381).
- `nix/node-b.nix`: the four env lines, driven by `snapshotReleaseRetain`.

**Tests, written first** (`internal/vmmanager/guest_release_test.go`,
`snapshot_release_test.go`, `cmd/vmctl/env_config_test.go`). Use a temp
releases dir with two releases, R1 and R2, and a link pointing at R2:
1. **Off:** `buildFirecrackerConfig` output and the launch hash are
   byte-identical to today's for the same inputs (a golden test).
2. **On:** launch paths are under `releases/R2/`; `KernelParams` comes
   from `R2/kernel-params`, not from `m.cfg`; the launch record is written
   before the spawn hook runs; and a missing `installed.json` gives a
   legacy launch with `GuestRelease=""`.
3. A snapshot taken on R1 with the current release R2, the same floor,
   epoch and ABI, and a fresh release: `validateSnapshot` returns `""`,
   the expected hash equals the launch hash, and the resume timeline says
   `behind=true`.
4. Each of R0 to R6 returns its reason. A legacy meta under on behaves
   exactly as under off.
5. A changed byte or mtime in `R1/storedisk.erofs` gives
   `disk_identity_mismatch`.
6. A host policy change (for example the guest DNS list now selecting
   the computer) gives `launch_config_mismatch` even with R1 retained.
7. **Reattach** with a launch record for R1 and the link at R2: the
   instance config names R1. Hibernating it gives meta R1 and bound paths
   under R1. With no launch record, `GuestRelease=""`.
8. **Jailed:** `prepareJail` sources come from the release dir, and the
   shared-copy name is stable for the same release across two link flips.
9. **Env:** each of the four variables parses, and the defaults hold.
10. **The handler:** running plus unconsumed snapshots, max age honoured,
    vault root included.

Use `-race` for the reattach and resume tests.
**Rollback:** `snapshotReleaseRetain = false` (R0 then refuses every
release-bound snapshot, which cold boots once), or git revert. A reverted
vmctl ignores the unknown meta field. Its `expectedLaunchConfig` hashes
the stable paths, which differ from the release paths in the launch, so
the snapshot gets `launch_config_mismatch` and cold boots. No snapshot
loads against wrong bytes.

### S4: resume, then update (red, tier 1)

**Files:**
- `internal/vmctl/ownership.go`: the `resumeFromSnapshot` hook (§5.1).
- `deploy_refresh_retry.go`: the entry's `Kind`, `enrollBehind`, and the
  user-idle gate in `SweepDeployRefreshPending`.
- `cmd/vmctl/main.go`: wiring and env `VMCTL_BEHIND_REFRESH_IDLE`
  (default `2m`).
- `nix/node-b.nix`: the env line.

**Tests, written first** (`deploy_refresh_behind_test.go`, with the
fixed clock and the fake marker the existing tests use):
1. A behind resume enrolls one entry with target = the current commit. A
   non-behind resume enrolls nothing.
2. The sweep skips the entry while `LastActiveAt` is within the gate and
   refreshes it after (`planned_restart=true`).
3. A busy guest keeps the entry. Refreshed or failed forgets it.
4. "Already serves target" forgets it.
5. A vmctl restart reloads the entry from the pending file with `Kind`.
6. A CI deploy refresh of the same computer settles and forgets the entry.
7. A non-`woke_behind` entry is unaffected by the gate.

**Rollback:** the switch off stops behind resumes, so no new entries
appear. Existing entries are ordinary deferred refreshes.

### S5: hardening (red, tier 1; may follow S6)

1. **The marker is never captured in a snapshot.**
   - **Guest:** `prepareHibernate` (hibernate.go:281) answers 409 busy
     (`planned_restart_pending`) while a planned-restart marker younger
     than `plannedRestartMaxAge` exists.
   - **Host:** the idle sweep, make-room and operator hibernate skip a
     computer whose deploy refresh is in flight
     (`deployRefreshState().inFlight`).
   - **Tests** for both.
2. **CPU fingerprint.**
   - `SnapshotMeta.HostCPU` = sha256 of the first processor's `vendor_id`,
     `model name`, `flags` and `microcode` from `/proc/cpuinfo`.
   - Validation refuses `host_cpu_mismatch` when both are set and differ.
     An empty value (an old meta) is allowed, as today.
   - **Tests** with fake cpuinfo.

### S6: switch on and prove (red; FP0 runs the landing loop)

Set `snapshotReleaseRetain = true` in nix/node-b.nix. Land it through the
landing loop. Then run §9 P1 on the **next** NixOS-switch deploy after a
d@e hibernate taken under the switch. A snapshot taken before the switch
was on is legacy.

**Switch summary:**
- **Off:** `snapshotReleaseRetain = false`, the one nix let-binding that
  drives the activation step and `VM_SNAPSHOT_RELEASE_RETAIN`.
- **Behaviour when off:** byte-identical launches, no release-bound
  snapshots, and release directories pruned to current and previous.
- **Rollback:** one edit and one deploy. Every release-bound snapshot
  cold boots once (R0), as every snapshot does after a deploy today.

## 8. Failure modes (each pinned by a test or a reading)

| # | Failure | Answer |
|---|---|---|
| M1 | Resume onto a deleted or overwritten release | R1 `release_missing`; bound-file identity (S3 test 5). Retention never deletes a running or young release, and deleting a snapshot's release only cold boots it. |
| M2 | Resume below the floor or the security epoch | R3 and R4, compared at every wake with the current release. S3 test 4. |
| M3 | A behind computer is never refreshed | The durable pending set with a 1-minute sweep and a 2-minute user-idle gate. A computer behind and busy for longer than the support window is refused at its next wake (R6). The CI refresh at the next deploy also reaches it. Health lists `behind`. |
| M4 | Disk growth from retained releases | Kept: current, previous, young, in use; the cap (8); deletion past the support window. R7 sizes a release before S6. Logged per run. The snapshot budget (`VM_SNAPSHOT_BUDGET_GIB=48`) is separate and unchanged. |
| M5 | Jailer paths | The source is the release path, so the shared copy is stable per release (§4.2). `confinement_mismatch` and the uid checks are kept. S3 test 8. |
| M6 | Snapshot vault | No vault under the jailer (snapshot_lifecycle.go:131); vault metas are counted in `in_use` (S3 test 10). |
| M7 | Credential delivery on resume | Unchanged. An old guest that cannot install the new envelope fails the handshake and cold boots. |
| M8 | A vmctl restart resumes something | Nothing new runs at boot. Reattach reads the launch record. The pending set only asks guests for planned refreshes. |
| M9 | The switch races a launch | Launch resolves the link once and binds the resolved release paths. A release under 2 h old and the previous one are never pruned. |
| M10 | A reattached VM records the wrong release (today's `m.cfg` paths) | §4.3. With no launch record, the next snapshot is legacy, which is safe. S3 test 7. |
| M11 | `kernel-params` goes stale between the NixOS switch and the vmctl restart (H1) | With the switch on, every launch reads `kernel-params` from the resolved release. S3 test 2. |
| M12 | A planned-restart marker is captured in a snapshot | S5 item 1. |
| M13 | Silent host-guest protocol skew | R5 `guest_abi`, plus the fail-closed handshake. The same skew already exists for deferred computers. |
| M14 | Floor list drift: a floor dependency missing from the digest | `scripts/check-guest-floor-paths` in CI. The `guest-floor-commit-free` flake check. Over-approximation is safe. |
| M15 | CPU features change under a resumed guest | S5 item 2. |
| M16 | Rollback loads wrong bytes | Off: R0 refuses. Revert: the launch hash mismatches (§7 S3 rollback). Neither path loads. |
| M17 | A behind computer is hibernated again before its refresh (idle sweep, make-room) | Allowed: the snapshot names the old release, which stays in use. Each cycle stays bounded by R6 and the max age. |
| M18 | The update restarts a computer in front of its user | The user-idle gate (S4). The CI refresh at deploy time keeps today's semantics for every active computer, unchanged by D2. |
| M19 | A security fix outside the floor ships without an epoch bump | §6 item 3: bounded by the support window and the max age. Commit rule and classifier label. |

## 9. Proof on staging

Use d@e or a test account (example.com / example.test), never b@c, the
paper, public_platform or the owner's computers.

### P0: baseline, today's code (optional, read-only proof of §2 case a)

1. Hibernate d@e: `POST /internal/vmctl/hibernate {"computer_id": "<d@e>"}`.
   The response carries `hibernate_timeline` with `outcome=snapshot`.
2. Wait for a **service-pointer** deploy: its deploy log says "Skipping
   NixOS switch", and the vmctl pid is unchanged in the journal.
3. Wake through the product: a session request, or
   `POST /api/computers/{id}/lifecycle/start`. An API-key request does not
   wake a computer (internal/proxy/api_key_computer_authority.go:166-183).
4. Read
   `GET /internal/vmctl/boot-timeline?computer_id=<d@e>`:
   `kind=snapshot_resume`, `snapshot.outcome=snapshot`,
   `wake_to_healthy_ms < 1000`.

If this fails, it is a **new problem doc**, not D2.

### P1: the D2 acceptance (after S6)

1. **Read** vmctl health `guest_release_current` (call it R_a), and
   `ls /var/lib/go-choir/guest-releases`.
2. **Hibernate** d@e as in P0.1. Then
   `ls /var/lib/go-choir/vm-state/<vm>/`: `snapshot/` and
   `guest-release.json` exist. The meta's `guest_release` is R_a (read
   through the hibernate response or the boot timeline, not by catting
   files).
3. **Wait** for any **NixOS-switch** deploy whose floor digest is
   unchanged. Health `guest_release_current` is now R_b ≠ R_a. If the
   floor moved, the expected result is a refusal: record
   `release_floor_moved` and wait for the next deploy.
4. **Wake** d@e through the product. The wake timeline must show:
   - `snapshot.outcome=snapshot`;
   - `snapshot.guest_release=R_a`, `snapshot.current_release=R_b`,
     `snapshot.behind=true`;
   - `wake_to_healthy_ms < 1000`.

   The journal must show `resumed VM <vm> from snapshot`, followed by the
   behind enrollment. Record MemAvailable before and after.
5. **Leave d@e idle** for at least 2 minutes. The journal must show
   `deploy refresh caller=retry-sweep ... outcome=refreshed planned_restart=true`.
   The guest `/health` `build.commit` must equal R_b's commit. Health
   `behind=false`.
6. **Retention.** The next retention run keeps R_a only while in use, and
   prunes it after the refresh, once it is neither previous nor young.
   Record `btrfs filesystem du -s` of the releases directory (R7).

**Pass:**
- step 4 is sub-second, with `behind=true`;
- step 5 is a planned refresh onto current;
- no `release_*` refusal other than the expected floor case.

Record the evidence in
`docs/evidence/density-d2-snapshot-survives-deploy-<date>.md`.

## 10. Readings before S6 (FP0, Node B, read only)

- **R7: the size of a release.**
  - `ls -l` and `btrfs filesystem du -s` of one
    `/var/lib/go-choir/guest-releases/<id>` after its store path is
    collected.
  - `nix path-info -Sh $(readlink /var/lib/go-choir/guest)` for the
    closure-retention comparison in §4.1.
  - Gate: if exclusive bytes per release exceed 3 GiB, lower the cap
    before S6.
- **R8: the hibernation rate.** No snapshot hibernate since 2026-10-24
  08:23, per the journal. D2 pays off only once D3 makes idle hibernation
  reach computers again; P1 uses an operator hibernate, so it does not
  wait on D3.
- **R9: how often the floor moves.**
  - Data so far: in the 7 days to 2026-10-26, 159 of 1,135 non-doc
    commits on origin/main touched a floor Go package. The file changes
    are dominated by `internal/capsule` (132) and `internal/yaegikernel`
    (68). 19 touched `nix/autoputer-vm.nix` and 2 touched `flake.lock`.
  - So a sizeable share of NixOS-switch deploys will move the floor. For
    those, D2 resumes nothing, by design.
  - After S1, record the floor digest per deploy for a week and report
    the share of deploys that keep it. That share is D2's ceiling.
  - If it is low, the refinement is to compute the floor from **built**
    floor binaries without the commit stamp, rather than from source
    paths. Go builds are reproducible, so only real code changes would
    move it. That is a later slice.

## 11. Heresy delta and owner questions

**Discovered** (read from code, not yet observed on staging):

- **H1.** vmctl reads `kernel-params` once at start (config.go:47-49).
  The NixOS switch repoints `guest/` before the deploy restarts vmctl
  (node-b.nix:2965, then node-b-ci-deploy:508-516). Between the two, a
  cold boot would pair the new kernel and store disk with the old `init=`
  path, which is absent from the new erofs.
  - The code path is certain.
  - That it has caused failed boots is a **hypothesis**. To confirm, match
    boot failures in the vmctl journal against the switch-to-restart
    window of each deploy.
  - S3 repairs it when the switch is on.
- **H2.** `ReattachVMWithConfig` gives a VM running an older image the
  current image's paths (manager.go:1517-1527). Today it is masked by the
  launch hash. S3 repairs it.
- **H3.** Commit stamping makes every NixOS switch a guest change
  (§1.1). It is a substrate cause of snapshot voiding. Removing it would
  touch build identity, so it is out of scope and recorded only.
- **H4.** No snapshot hibernate on staging since 2026-10-24 08:23 (R8).
- **H5.** No CPU template and no CPU fingerprint check (§1.2). S5
  repairs it.

**Introduced:** none (a design). **Repaired:** none yet.

**Owner questions**, each with a default (none blocks the build):

- **Q-D2-1.** May a woken computer run the previous runtime for a few
  minutes, until its next idle moment, when the floor and the security
  epoch are unchanged? Default: yes, within the one-week support window.
- **Q-D2-2.** Who raises `security_epoch` for a fix outside the floor?
  Default: the fix's author, in the same commit, visible in the deploy
  explanation.
- **Q-D2-3.** The user-idle gate before the update restart. Default: 2
  minutes.
